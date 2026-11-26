#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests for examples/launch.py: where it looks for zenzx, the arguments it gives it, and the
download of the program from a page of programs (a local one, through file://).

    python3 scripts/test_launch.py
"""
import hashlib
import importlib.util
import json
import os
import stat
import subprocess
import sys
import tempfile
import unittest

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
LAUNCH = os.path.join(REPO, "examples", "launch.py")


def load():
    spec = importlib.util.spec_from_file_location("launch_under_test", LAUNCH)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def fake_zenzx(path, log):
    """A zenzx that records its arguments, one line per start, and leaves."""
    with open(path, "w", newline="\n") as f:
        f.write('#!/bin/sh\necho "$@" >> "%s"\nexit 0\n' % log)
    os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR)


@unittest.skipIf(os.name == "nt", "the fake program is a shell script")
class LaunchTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = self.tmp.name
        self.mod = load()
        self.mod.ROOT = self.root
        self.log = os.path.join(self.root, "args.log")
        self.env = dict(os.environ)
        for k in ("ZENZX", "ZENZX_SITE"):
            self.env.pop(k, None)

    def test_the_environment_variable_comes_first(self):
        a = os.path.join(self.root, "a")
        b = os.path.join(self.root, "bin")
        os.makedirs(b)
        fake_zenzx(a, self.log)
        fake_zenzx(os.path.join(b, "zenzx"), self.log)
        os.environ["ZENZX"] = a
        try:
            self.assertEqual(self.mod.find_zenzx(), a)
        finally:
            del os.environ["ZENZX"]

    def test_bin_then_the_folder_then_an_unpacked_archive(self):
        os.environ.pop("ZENZX", None)
        arch = os.path.join(self.root, "zenzx-linux-amd64")
        os.makedirs(arch)
        fake_zenzx(os.path.join(arch, "zenzx"), self.log)
        self.assertEqual(self.mod.find_zenzx(), os.path.join(arch, "zenzx"))
        fake_zenzx(os.path.join(self.root, "zenzx"), self.log)
        self.assertEqual(self.mod.find_zenzx(), os.path.join(self.root, "zenzx"))
        os.makedirs(os.path.join(self.root, "bin"))
        fake_zenzx(os.path.join(self.root, "bin", "zenzx"), self.log)
        self.assertEqual(self.mod.find_zenzx(), os.path.join(self.root, "bin", "zenzx"))

    def run_launcher(self, *args):
        env = dict(self.env, ZENZX_EXAMPLES_DELAY="0")
        return subprocess.run([sys.executable, LAUNCH] + list(args), env=env, capture_output=True, text=True, timeout=60)

    def test_world_gives_zenzx_what_run_sh_gives_it(self):
        z = os.path.join(self.root, "fake")
        fake_zenzx(z, self.log)
        self.env["ZENZX"] = z
        r = self.run_launcher("world")
        with open(self.log) as f:
            line = f.read().strip()
        tap = os.path.join(REPO, "examples", "world", "world.tap")
        ports = ",".join(str(16384 + i) for i in range(9))
        want = "-scale 2 -layout grid -focus-follow -multi 9:48k -listen %s -autorun %s" % (ports, ",".join([tap] * 9))
        self.assertEqual(line, want, r.stderr)

    def test_chat_gives_zenzx_what_chat_sh_gives_it(self):
        z = os.path.join(self.root, "fake")
        fake_zenzx(z, self.log)
        self.env["ZENZX"] = z
        self.run_launcher("chat")
        with open(self.log) as f:
            line = f.read().strip()
        here = os.path.join(REPO, "examples", "link")
        clients = ",".join(os.path.join(here, n) for n in ("chat.bas", "chat-ada.bas", "chat-clive.bas", "chat-grace.bas"))
        self.assertEqual(line, "-scale 3 -layout grid -multi 4:48k,48k,128k,plus3 -listen 16384,16385,16386,16387 -autorun " + clients)

    def test_next_puts_the_examples_on_the_card_and_starts_the_next(self):
        z = os.path.join(self.root, "fake")
        fake_zenzx(z, self.log)
        self.env["ZENZX"] = z
        self.run_launcher("next", "--", "-scale", "1")
        with open(self.log) as f:
            lines = f.read().splitlines()
        self.assertEqual(lines, ["-next-examples", "-model=next -scale 1"])

    def test_missing_zenzx_is_said_with_where_to_get_it_and_exits_2(self):
        empty = os.path.join(self.root, "empty")
        os.makedirs(empty)
        env = dict(self.env, PATH=empty, ZENZX_SITE="file:///nonexistent/")
        r = subprocess.run([sys.executable, LAUNCH, "chat"], env=env, capture_output=True, text=True, timeout=60, stdin=subprocess.DEVNULL)
        self.assertEqual(r.returncode, 2, r.stdout + r.stderr)
        self.assertIn("cannot find the zenzx program", r.stderr)
        self.assertIn("file:///nonexistent/", r.stderr)

    def make_site(self, payload, sha=None):
        site = os.path.join(self.root, "site")
        os.makedirs(os.path.join(site, "bin"), exist_ok=True)
        system, arch = self.mod.host_key()
        name = "bin/zenzx-%s-%s" % (system, arch)
        with open(os.path.join(site, name), "wb") as f:
            f.write(payload)
        manifest = {"schema": 1, "program": "zenzx", "version": "9.9.9", "runners": [
            {"os": system, "arch": arch, "file": name, "size": len(payload), "sha256": sha or hashlib.sha256(payload).hexdigest()}]}
        with open(os.path.join(site, "runners.json"), "w") as f:
            json.dump(manifest, f)
        return "file://" + site + "/"

    def test_download_checks_the_sha256_and_keeps_what_matches(self):
        system, arch = self.mod.host_key()
        if not system or not arch:
            self.skipTest("this system is not one the page has a program for")
        self.mod.SITE = self.make_site(b"#!/bin/sh\nexit 0\n")
        got = self.mod.download_zenzx(True)
        self.assertEqual(got, os.path.join(self.root, "bin", "zenzx"))
        self.assertTrue(os.access(got, os.X_OK))
        self.assertFalse(os.path.exists(got + ".part"))

    def test_download_refuses_what_does_not_match_and_leaves_nothing(self):
        system, arch = self.mod.host_key()
        if not system or not arch:
            self.skipTest("this system is not one the page has a program for")
        self.mod.SITE = self.make_site(b"not the program", sha="0" * 64)
        self.assertIsNone(self.mod.download_zenzx(True))
        self.assertFalse(os.path.exists(os.path.join(self.root, "bin", "zenzx")))
        self.assertFalse(os.path.exists(os.path.join(self.root, "bin", "zenzx.part")))

    def test_download_without_consent_in_a_pipe_does_not_download(self):
        system, arch = self.mod.host_key()
        if not system or not arch:
            self.skipTest("this system is not one the page has a program for")
        self.mod.SITE = self.make_site(b"x")
        self.assertIsNone(self.mod.download_zenzx(False))
        self.assertFalse(os.path.exists(os.path.join(self.root, "bin")))


if __name__ == "__main__":
    unittest.main()
