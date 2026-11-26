#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests of tools/check_binaries.py: python3 scripts/test_check_binaries.py

They build a small Go program for each system (so a real ELF, Mach-O and PE of each kind is
read, not a hand-made header) and need LIEF and a Go toolchain; without either they are
skipped.
"""

import contextlib
import importlib.util
import io
import os
import shutil
import subprocess
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("check_binaries", os.path.join(HERE, "..", "tools", "check_binaries.py"))
cb = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cb)

PROGRAM = "package main\n\nfunc main() {}\n"
GO_MOD = "module probe\n\ngo 1.21\n"


def run(*argv):
    """check_binaries.main(argv): (exit status, what it printed)."""
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        try:
            rc = cb.main(list(argv))
        except SystemExit as e:  # argparse
            rc = e.code
    return rc, out.getvalue() + err.getvalue()


@unittest.skipIf(cb.lief is None, "LIEF is not installed (pip install lief)")
@unittest.skipIf(shutil.which("go") is None, "no Go toolchain")
class CheckBinaries(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.dir = tempfile.mkdtemp(prefix="cbtest")
        src = os.path.join(cls.dir, "src")
        os.mkdir(src)
        with open(os.path.join(src, "main.go"), "w") as f:
            f.write(PROGRAM)
        with open(os.path.join(src, "go.mod"), "w") as f:
            f.write(GO_MOD)
        cls.src = src
        cls.bin = os.path.join(cls.dir, "bin")
        os.mkdir(cls.bin)

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.dir, ignore_errors=True)

    @classmethod
    def build(cls, goos, goarch, name, ldflags="-s -w"):
        """A tiny program for goos/goarch, written as bin/name."""
        out = os.path.join(cls.bin, name)
        env = dict(os.environ, CGO_ENABLED="0", GOOS=goos, GOARCH=goarch, GOTOOLCHAIN="local", GOFLAGS="")
        r = subprocess.run(["go", "build", "-trimpath", "-ldflags", ldflags, "-o", out, "."],
                           cwd=cls.src, env=env, capture_output=True, text=True)
        if r.returncode != 0:
            raise RuntimeError("go build %s/%s: %s" % (goos, goarch, r.stderr))
        return out

    def path(self, name):
        return os.path.join(self.bin, name)

    def need(self, goos, goarch, name, ldflags="-s -w"):
        if not os.path.exists(self.path(name)):
            self.build(goos, goarch, name, ldflags)
        return self.path(name)

    # The programs, as a release builds them, pass.

    def test_every_system_passes(self):
        for goos, goarch, name, ld in [
            ("linux", "amd64", "p-linux-amd64", "-s -w"),
            ("linux", "arm64", "p-linux-arm64", "-s -w"),
            ("freebsd", "amd64", "p-freebsd-amd64", "-s -w"),
            ("darwin", "amd64", "p-darwin-amd64", "-s -w"),
            ("darwin", "arm64", "p-darwin-arm64", "-s -w"),
            ("windows", "amd64", "p-windows-amd64.exe", "-s -w -H=windowsgui"),
            ("windows", "arm64", "p-windows-arm64.exe", "-s -w -H=windowsgui"),
        ]:
            with self.subTest(name):
                rc, text = run(self.need(goos, goarch, name, ld))
                self.assertEqual(rc, 0, text)
                self.assertIn("0 failed", text)

    def test_the_darwin_arm64_program_is_signed_and_names_its_minimum(self):
        rc, text = run(self.need("darwin", "arm64", "p-darwin-arm64"))
        self.assertEqual(rc, 0, text)
        self.assertRegex(text, r"signature\s+present")
        self.assertRegex(text, r"minos\s+macOS \d+\.\d+ or later")

    # Each check can fail.

    def test_the_name_says_another_processor(self):
        wrong = os.path.join(self.bin, "p-windows-arm64-wrong.exe")
        shutil.copy(self.need("windows", "amd64", "p-windows-amd64.exe", "-s -w -H=windowsgui"), wrong)
        rc, text = run(wrong)
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+arch\s+amd64, but the name says arm64")

    def test_the_name_says_another_system(self):
        wrong = os.path.join(self.bin, "p-darwin-arm64-wrong")
        shutil.copy(self.need("linux", "arm64", "p-linux-arm64"), wrong)
        rc, text = run(wrong)
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+format\s+ELF, but a darwin program is Mach-O")

    def test_a_windows_program_with_a_console_fails_as_a_gui_program(self):
        console = self.need("windows", "amd64", "p-windows-amd64-console.exe", "-s -w")
        rc, text = run(console)
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+subsystem\s+windows_cui, but windows_gui is wanted")
        # The headless program is the one that is meant to have a console.
        rc, text = run(console, "--kind", "headless")
        self.assertEqual(rc, 0, text)
        rc, text = run(console, "--subsystem", "console")
        self.assertEqual(rc, 0, text)
        rc, text = run(console, "--subsystem", "any")
        self.assertEqual(rc, 0, text)

    def test_a_headless_windows_program_with_a_window_subsystem_fails(self):
        gui = self.need("windows", "amd64", "p-windows-amd64.exe", "-s -w -H=windowsgui")
        named = os.path.join(self.bin, "p-headless-windows-amd64.exe")
        shutil.copy(gui, named)
        rc, text = run(named)
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+subsystem\s+windows_gui, but windows_cui is wanted")

    def test_an_import_that_is_not_allowed(self):
        exe = self.need("windows", "amd64", "p-windows-amd64.exe", "-s -w -H=windowsgui")
        rc, text = run(exe, "--allow-import", "user32.dll")
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+imports\s+imports kernel32.dll, which is not allowed")
        rc, text = run(exe, "--allow-import", "user32.dll", "--allow-import", "kernel32.dll")
        self.assertEqual(rc, 0, text)

    def test_the_macos_minimum_is_above_the_limit(self):
        exe = self.need("darwin", "arm64", "p-darwin-arm64")
        rc, text = run(exe, "--max-min-macos", "10.9")
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+minos\s+macOS .* above the limit 10.9")

    def test_a_macos_library_outside_the_system(self):
        exe = self.need("darwin", "arm64", "p-darwin-arm64")
        rc, text = run(exe, "--allow-lib", "/opt/elsewhere/")
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+libs\s+outside the system")

    def test_a_build_path_inside_the_program(self):
        dirty = os.path.join(self.bin, "p-linux-amd64-dirty")
        shutil.copy(self.need("linux", "amd64", "p-linux-amd64"), dirty)
        with open(dirty, "ab") as f:
            f.write(b"\0/home/builder/work/probe/main.go\0")
        rc, text = run(dirty)
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+strings\s+'/home/' at byte \d+")
        rc, text = run(dirty, "--no-default-forbid")
        self.assertEqual(rc, 0, text)
        rc, text = run(self.need("linux", "amd64", "p-linux-amd64"), "--forbid", "main.main")
        self.assertEqual(rc, 1, text)

    def test_homebrews_path_on_linux_is_not_a_build_path(self):
        # pkg/fetch looks for brew in Homebrew's prefix on Linux, so the program names it.
        brew = os.path.join(self.bin, "p-linux-amd64-brew")
        shutil.copy(self.need("linux", "amd64", "p-linux-amd64"), brew)
        with open(brew, "ab") as f:
            f.write(b"\0/home/linuxbrew/.linuxbrew/bin\0")
        rc, text = run(brew)
        self.assertEqual(rc, 0, text)
        # a build path after it is still found, and is the one that is said
        with open(brew, "ab") as f:
            f.write(b"\0/home/builder/work/probe/main.go\0")
        rc, text = run(brew)
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+strings\s+'/home/' at byte \d+ \(.*builder")
        # and --allow lets another path through
        rc, text = run(brew, "--allow", "/home/builder/")
        self.assertEqual(rc, 0, text)

    def test_a_static_program_needs_no_glibc(self):
        exe = self.need("linux", "amd64", "p-linux-amd64")
        rc, text = run(exe)  # a static Go program needs no glibc at all
        self.assertRegex(text, r"glibc\s+needs no glibc")
        self.assertEqual(rc, 0, text)

    def test_a_file_that_is_not_a_program(self):
        txt = os.path.join(self.dir, "notes.txt")
        with open(txt, "w") as f:
            f.write("not a program\n")
        rc, text = run(txt)
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+format\s+not an ELF, Mach-O or PE file")

    # The options that change what the tool does.

    def test_skip_and_warn_only(self):
        console = self.need("windows", "amd64", "p-windows-amd64-console.exe", "-s -w")
        rc, text = run(console, "--skip", "subsystem")
        self.assertEqual(rc, 0, text)
        self.assertNotIn("subsystem", text)
        rc, text = run(console, "--warn-only")
        self.assertEqual(rc, 0, text)
        self.assertIn("FAIL", text)
        self.assertIn("--warn-only", text)

    def test_a_directory_is_searched_and_other_files_are_left_alone(self):
        d = os.path.join(self.dir, "dist")
        os.makedirs(os.path.join(d, "zenzx-linux-amd64"))
        shutil.copy(self.need("linux", "amd64", "p-linux-amd64"), os.path.join(d, "zenzx-linux-amd64", "zenzx"))
        with open(os.path.join(d, "README.md"), "w") as f:
            f.write("text\n")
        rc, text = run(d)
        self.assertEqual(rc, 0, text)
        self.assertIn("1 programs", text)
        self.assertNotIn("README", text)

    def test_the_directory_names_the_system_and_the_processor(self):
        d = os.path.join(self.dir, "dist2")
        os.makedirs(os.path.join(d, "zenzx-darwin-arm64"))
        shutil.copy(self.need("linux", "amd64", "p-linux-amd64"), os.path.join(d, "zenzx-darwin-arm64", "zenzx"))
        rc, text = run(d)
        self.assertEqual(rc, 1, text)
        self.assertRegex(text, r"FAIL\s+format")

    def test_quiet_prints_only_what_failed(self):
        good = self.need("linux", "amd64", "p-linux-amd64")
        rc, text = run(good, "--quiet")
        self.assertEqual(rc, 0, text)
        self.assertNotIn("  ok  ", text)

    def test_usage_errors_exit_2(self):
        for argv in (["--skip", "nonsense", "x"], [], ["/no/such/file"]):
            rc, text = run(*argv)
            self.assertEqual(rc, 2, (argv, text))

    def test_list_names_every_check(self):
        rc, text = run("--list")
        self.assertEqual(rc, 0)
        for name in cb.CHECKS:
            self.assertIn(name, text)


class LinuxGUILibraries(unittest.TestCase):
    def test_the_glibc_loader_of_each_processor_is_allowed(self):
        # The arm64 build lists ld-linux-aarch64.so.1 among its needed libraries (the amd64 one
        # does not list its own): a GUI program that needs only glibc, ALSA and X11 must pass.
        for name in ("ld-linux-aarch64.so.1", "ld-linux-x86-64.so.2", "libc.so.6", "libasound.so.2", "libX11.so.6"):
            self.assertIn(name, cb.LINUX_GUI_LIBS)
        self.assertNotIn("libGL.so.1", cb.LINUX_GUI_LIBS)  # raylib loads GL at run time: linking it is a mistake


class VersionHelpers(unittest.TestCase):
    def test_version_tuple(self):
        self.assertEqual(cb.version_tuple("GLIBC_2.38"), (2, 38))
        self.assertEqual(cb.version_tuple("12.0"), (12, 0))
        self.assertIsNone(cb.version_tuple("none"))

    def test_expectations_from_the_path(self):
        class Opts:
            os = arch = kind = None

        self.assertEqual(cb.expectations("dist/zenzx-windows-arm64.exe", Opts), ("windows", "arm64", "gui"))
        self.assertEqual(cb.expectations("dist/zenzx-headless-linux-amd64", Opts), ("linux", "amd64", "headless"))
        self.assertEqual(cb.expectations("dist/zenzx-darwin-amd64/zenzx", Opts), ("darwin", "amd64", "gui"))
        self.assertEqual(cb.expectations("dist/zenzx", Opts), (None, None, "gui"))


if __name__ == "__main__":
    unittest.main()
