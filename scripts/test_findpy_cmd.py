#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests for examples/tools/findpy.cmd, the Windows launcher's search for Python, under Wine.

There is no Windows here, so the real batch file runs in Wine's cmd, with the real ZenZX built
for Windows (the headless build, which needs no C compiler) fetching "Python" from a web server
of this test's own: a zip with a tiny python.exe that exits 0. Skipped without Wine or Go.

    python3 scripts/test_findpy_cmd.py
"""
import hashlib
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import test_fetch_cli as fc  # noqa: E402

REPO = os.path.dirname(HERE)
FINDPY = os.path.join(REPO, "examples", "tools", "findpy.cmd")
WINE = next((p for p in ("/usr/lib/wine/wine64", shutil.which("wine64") or "", shutil.which("wine") or "") if p and os.path.exists(p)), None)
GO = shutil.which("go")
WRAPPER = '@echo off\r\ncall "%~dp0..\\tools\\findpy.cmd"\r\nif defined PY (echo PY=%PY%) else (echo PY=NONE)\r\n'


def win(path):
    return "Z:" + path.replace("/", "\\")


@unittest.skipUnless(WINE and GO, "needs Wine and Go")
class FindPyCmd(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build = tempfile.mkdtemp(prefix="zenzx-cmd-test-")
        env = dict(os.environ, CGO_ENABLED="0", GOOS="windows", GOARCH="amd64")
        stub = os.path.join(cls.build, "stub")
        os.makedirs(stub)
        with open(os.path.join(stub, "main.go"), "w") as f:
            f.write("package main\nfunc main() {}\n")
        with open(os.path.join(stub, "go.mod"), "w") as f:
            f.write("module stub\ngo 1.21\n")
        cls.pyexe = os.path.join(cls.build, "python.exe")
        cls.run_go(["build", "-o", cls.pyexe, "."], stub, env)
        cls.console = os.path.join(cls.build, "zenzx-headless.exe")
        cls.run_go(["build", "-tags", "headless", "-o", cls.console, "./cmd/zenzx-headless"], REPO, env)
        cls.windowed = os.path.join(cls.build, "zenzx.exe")  # the Windows subsystem, as the program for people is
        cls.run_go(["build", "-tags", "headless", "-ldflags=-H=windowsgui", "-o", cls.windowed, "./cmd/zenzx-headless"], REPO, env)
        cls.zip_path = os.path.join(cls.build, "py.zip")
        with zipfile.ZipFile(cls.zip_path, "w") as z:
            z.write(cls.pyexe, "python.exe")
        cls.zip_bytes = fc.slurp(cls.zip_path, "rb")
        cls.prefixes = {}  # the Wine prefix, once made, and the Windows version it is set to

    @classmethod
    def run_go(cls, args, cwd, env):
        r = subprocess.run([GO] + args, cwd=cwd, env=env, capture_output=True, text=True)
        if r.returncode != 0:
            raise RuntimeError("go %s failed:\n%s" % (" ".join(args), r.stderr))

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.build, True)

    def prefix(self, version="win10"):
        """The Wine prefix, made once (it is some 600 MB, so there is one) and set to Windows `version`."""
        path = os.path.join(self.build, "prefix")
        env = self.wine_env(path)
        if not self.prefixes:
            subprocess.run([WINE, "cmd", "/c", "echo ready"], env=env, capture_output=True, timeout=240)
        if self.prefixes.get("version") != version:
            subprocess.run([WINE, "winecfg", "/v", version], env=env, capture_output=True, timeout=240)
            self.prefixes["version"] = version
        return path

    def wine_env(self, prefix, extra=None):
        env = {"HOME": os.path.join(self.build, "home"), "PATH": "/usr/bin:/bin", "WINEPREFIX": prefix,
               "WINEDEBUG": "-all", "WINEARCH": "win64"}
        env.update(extra or {})
        return env

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="zenzx-cmd-")
        self.addCleanup(shutil.rmtree, self.tmp, True)
        self.pkg = os.path.join(self.tmp, "pkg")
        self.ex = os.path.join(self.pkg, "examples")
        os.makedirs(os.path.join(self.ex, "tools"))
        os.makedirs(os.path.join(self.ex, "probe"))
        shutil.copy(FINDPY, os.path.join(self.ex, "tools", "findpy.cmd"))
        with open(os.path.join(self.ex, "probe", "t.cmd"), "w", newline="") as f:
            f.write(WRAPPER)

    def put_zenzx(self, exe, as_name="zenzx.exe"):
        os.makedirs(os.path.join(self.pkg, "bin"), exist_ok=True)
        shutil.copy(exe, os.path.join(self.pkg, "bin", as_name))

    def probe(self, answer=b"\r\n", extra=None, version="win10"):
        env = self.wine_env(self.prefix(version), extra)
        r = subprocess.run([WINE, "cmd", "/c", "t.cmd"], cwd=os.path.join(self.ex, "probe"), env=env, input=answer,
                           capture_output=True, timeout=240)
        return (r.stdout + r.stderr).decode("utf-8", "replace").replace("\r", "")

    def download_env(self, base, sha=None):
        return {"ZENZX_PYTHON_URL": base + "/py.zip", "ZENZX_PYTHON_SHA256": sha or hashlib.sha256(self.zip_bytes).hexdigest()}

    def test_python_is_fetched_by_zenzx_checked_and_found(self):
        self.put_zenzx(self.console)
        with fc.Server({"/py.zip": self.zip_bytes}) as base:
            out = self.probe(extra=self.download_env(base))
        self.assertIn("Press Enter to download it now", out)
        self.assertIn("Downloading Python 3.12.10", out, out)
        self.assertRegex(out, r'PY="Z:[^"]*\\examples\\python\\python\.exe"')
        self.assertTrue(os.path.exists(os.path.join(self.ex, "python", "python.exe")))
        self.assertEqual([n for n in os.listdir(self.ex) if n.startswith(".python")], [])

    def test_windows_7_gets_python_3_8(self):
        self.put_zenzx(self.console)
        with fc.Server({"/py.zip": self.zip_bytes}) as base:
            out = self.probe(extra=self.download_env(base), version="win7")
        self.assertIn("Downloading Python 3.8.10", out, out)
        self.assertRegex(out, r'PY="Z:[^"]*python\.exe"')

    def test_the_program_for_people_is_waited_for_and_its_failure_comes_back(self):
        # the Windows program is built for the windowed subsystem, which a batch file does not wait for by itself
        self.put_zenzx(self.windowed)
        with fc.Server({"/py.zip": self.zip_bytes}) as base:
            out = self.probe(extra=self.download_env(base))
            self.assertRegex(out, r'PY="Z:[^"]*python\.exe"', out)
            shutil.rmtree(os.path.join(self.ex, "python"))
            out = self.probe(extra=self.download_env(base, "0" * 64))
        self.assertIn("PY=NONE", out)
        self.assertIn("Python could not be fetched", out)
        self.assertFalse(os.path.exists(os.path.join(self.ex, "python")))

    def test_a_download_that_does_not_match_is_refused(self):
        self.put_zenzx(self.console)
        with fc.Server({"/py.zip": self.zip_bytes}) as base:
            out = self.probe(extra=self.download_env(base, "0" * 64))
        self.assertIn("PY=NONE", out)
        self.assertIn("not the expected file", out)
        self.assertFalse(os.path.exists(os.path.join(self.ex, "python")))
        self.assertEqual([n for n in os.listdir(self.ex) if n.startswith(".python")], [])

    def test_q_quits_without_fetching(self):
        self.put_zenzx(self.console)
        out = self.probe(answer=b"q\r\n")
        self.assertIn("PY=NONE", out)
        self.assertFalse(os.path.exists(os.path.join(self.ex, "python")))

    def test_without_zenzx_it_says_where_zenzx_goes(self):
        out = self.probe()
        self.assertIn("PY=NONE", out)
        self.assertIn("The program ZenZX, which fetches it, is not where it can be found", out)
        self.assertIn("https://www.python.org/downloads/windows/", out)

    def test_zenzx_variable_names_the_program(self):
        os.makedirs(os.path.join(self.tmp, "elsewhere"))
        shutil.copy(self.console, os.path.join(self.tmp, "elsewhere", "mine.exe"))
        with fc.Server({"/py.zip": self.zip_bytes}) as base:
            out = self.probe(extra=dict(self.download_env(base), ZENZX=win(os.path.join(self.tmp, "elsewhere", "mine.exe"))))
        self.assertRegex(out, r'PY="Z:[^"]*python\.exe"', out)

    def test_an_environment_made_by_a_tool_is_found(self):
        os.makedirs(os.path.join(self.ex, "python", "Scripts"))
        shutil.copy(self.pyexe, os.path.join(self.ex, "python", "Scripts", "python.exe"))
        out = self.probe()
        self.assertRegex(out, r'PY="Z:[^"]*\\python\\Scripts\\python\.exe"', out)
        self.assertNotIn("not on this computer yet", out)


if __name__ == "__main__":
    unittest.main(verbosity=2)
