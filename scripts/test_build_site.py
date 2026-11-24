#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests of tools/build_site.py: python3 scripts/test_build_site.py"""

import contextlib
import hashlib
import importlib.util
import io
import json
import os
import re
import tarfile
import tempfile
import unittest
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("build_site", os.path.join(HERE, "..", "tools", "build_site.py"))
build_site = importlib.util.module_from_spec(spec)
spec.loader.exec_module(build_site)

VERSION = "9.9.9"
HEADS = {
    "linux": b"\x7fELF",
    "windows": b"MZ",
    "darwin": b"\xcf\xfa\xed\xfe",
}


def program(os_, arch):
    """A stand-in for a program: the right first bytes, then bytes that differ by platform."""
    body = f"{os_}/{arch}\n".encode() * 100_000
    return HEADS[os_] + body


def make_archives(folder, skip=(), override=None):
    """The release's GUI archives, and a headless one that must be ignored."""
    override = override or {}
    for os_, arch in build_site.PLATFORMS:
        if (os_, arch) in skip:
            continue
        data = override.get((os_, arch), program(os_, arch))
        top = f"zenzx-{os_}-{arch}"
        exe = "zenzx.exe" if os_ == "windows" else "zenzx"
        base = os.path.join(folder, f"zenzx-{VERSION}-{os_}-{arch}")
        if os_ == "windows":
            with zipfile.ZipFile(base + ".zip", "w") as z:
                z.writestr(f"{top}/{exe}", data)
                z.writestr(f"{top}/README.md", b"readme")
        else:
            with tarfile.open(base + ".tar.gz", "w:gz") as t:
                for name, content in ((f"{top}/{exe}", data), (f"{top}/README.md", b"readme")):
                    info = tarfile.TarInfo(name)
                    info.size = len(content)
                    t.addfile(info, io.BytesIO(content))
    # The programs without a window: not on the page.
    with tarfile.open(os.path.join(folder, f"zenzx-{VERSION}-linux-amd64-headless.tar.gz"), "w:gz") as t:
        info = tarfile.TarInfo("zenzx-linux-amd64-headless/zenzx-headless")
        info.size = 4
        t.addfile(info, io.BytesIO(b"nope"))


def slurp(path, mode="rb", **kw):
    with open(path, mode, **kw) as f:
        return f.read()


def run(arch_dir, out, version=VERSION):
    err = io.StringIO()
    with contextlib.redirect_stderr(err), contextlib.redirect_stdout(io.StringIO()):
        code = build_site.main(["--version", version, "--archives", arch_dir, "--out", out])
    return code, err.getvalue()


class BuildSite(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.arch = os.path.join(self.tmp.name, "dist")
        self.out = os.path.join(self.tmp.name, "site")
        os.mkdir(self.arch)

    def test_the_page_has_every_program_with_its_checksum(self):
        make_archives(self.arch)
        code, err = run(self.arch, self.out)
        self.assertEqual(code, 0, err)

        manifest = json.loads(slurp(os.path.join(self.out, "runners.json"), "r", encoding="utf-8"))
        self.assertEqual(manifest["schema"], build_site.SCHEMA)
        self.assertEqual(manifest["version"], VERSION)
        self.assertEqual([(r["os"], r["arch"]) for r in manifest["runners"]], build_site.PLATFORMS)

        sums = {}
        for line in slurp(os.path.join(self.out, "bin", "checksums.txt"), "r", encoding="ascii").splitlines(keepends=True):
            h, name = re.fullmatch(r"([0-9a-f]{64})  (\S+)\n", line).groups()
            sums[name] = h
        self.assertEqual(len(sums), len(build_site.PLATFORMS))

        for r in manifest["runners"]:
            path = os.path.join(self.out, r["file"])
            data = slurp(path)
            self.assertEqual(data, program(r["os"], r["arch"]), r["file"])
            self.assertEqual(r["size"], len(data))
            self.assertEqual(r["sha256"], hashlib.sha256(data).hexdigest())
            self.assertEqual(sums[os.path.basename(r["file"])], r["sha256"])
            self.assertEqual(r["file"], "bin/" + build_site.exe_name(r["os"], r["arch"]))
        # Windows has the .exe, the others have none.
        self.assertTrue(any(r["file"].endswith("-windows-amd64.exe") for r in manifest["runners"]))
        self.assertFalse(any(r["file"].endswith(".exe") for r in manifest["runners"] if r["os"] != "windows"))

        index = slurp(os.path.join(self.out, "index.html"), "r", encoding="utf-8")
        self.assertIn(f"ZenZX {VERSION}", index)
        for r in manifest["runners"]:
            self.assertIn(f'href="{r["file"]}"', index)
            self.assertIn(r["sha256"], index)
        self.assertIn(f"releases/tag/v{VERSION}", index)
        self.assertNotIn("headless-nope", index)
        # Nothing else is in the folder: no headless program, no archive.
        listing = sorted(os.listdir(os.path.join(self.out, "bin")))
        self.assertEqual(len(listing), len(build_site.PLATFORMS) + 1)

    def test_a_missing_platform_stops_the_build(self):
        make_archives(self.arch, skip=[("darwin", "arm64")])
        code, err = run(self.arch, self.out)
        self.assertEqual(code, 1)
        self.assertIn("darwin/arm64", err)

    def test_an_archive_of_another_version_does_not_count(self):
        make_archives(self.arch)
        code, err = run(self.arch, self.out, version="1.0.0")
        self.assertEqual(code, 1)
        self.assertIn("no archive for version 1.0.0", err)

    def test_a_program_that_is_not_one_for_its_platform_stops_the_build(self):
        wrong = HEADS["linux"] + b"x" * 2_000_000  # an ELF where a Windows program should be
        make_archives(self.arch, override={("windows", "amd64"): wrong})
        code, err = run(self.arch, self.out)
        self.assertEqual(code, 1)
        self.assertIn("zenzx-windows-amd64.exe", err)

    def test_a_program_far_too_small_stops_the_build(self):
        make_archives(self.arch, override={("linux", "arm64"): b"\x7fELF" + b"x" * 100})
        code, err = run(self.arch, self.out)
        self.assertEqual(code, 1)
        self.assertIn("too small", err)

    def test_the_page_is_built_afresh(self):
        make_archives(self.arch)
        os.makedirs(os.path.join(self.out, "bin"))
        stale = os.path.join(self.out, "bin", "zenzx-old")
        with open(stale, "w") as f:
            f.write("old")
        code, err = run(self.arch, self.out)
        self.assertEqual(code, 0, err)
        self.assertFalse(os.path.exists(stale))

    def test_a_version_is_escaped_in_the_page(self):
        archives = os.path.join(self.tmp.name, "x")
        os.mkdir(archives)
        v = "1.0<b>"
        # The archives' names carry the version, which a tag could make odd.
        for os_, arch in build_site.PLATFORMS:
            data = program(os_, arch)
            top = f"zenzx-{os_}-{arch}"
            exe = "zenzx.exe" if os_ == "windows" else "zenzx"
            base = os.path.join(archives, f"zenzx-{v}-{os_}-{arch}")
            if os_ == "windows":
                with zipfile.ZipFile(base + ".zip", "w") as z:
                    z.writestr(f"{top}/{exe}", data)
            else:
                with tarfile.open(base + ".tar.gz", "w:gz") as t:
                    info = tarfile.TarInfo(f"{top}/{exe}")
                    info.size = len(data)
                    t.addfile(info, io.BytesIO(data))
        code, err = run(archives, self.out, version=v)
        self.assertEqual(code, 0, err)
        index = slurp(os.path.join(self.out, "index.html"), "r", encoding="utf-8")
        self.assertNotIn("<b>", index)
        self.assertIn("1.0&lt;b&gt;", index)


if __name__ == "__main__":
    unittest.main()
