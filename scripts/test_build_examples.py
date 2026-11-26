#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests for tools/build_examples.py.

    python3 scripts/test_build_examples.py
"""
import hashlib
import importlib.util
import os
import re
import shutil
import tempfile
import unittest
import zipfile

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
spec = importlib.util.spec_from_file_location("build_examples", os.path.join(REPO, "tools", "build_examples.py"))
be = importlib.util.module_from_spec(spec)
spec.loader.exec_module(be)


class BuildExamplesTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.path, cls.n = be.build("9.9.9", REPO, cls.tmp.name)
        cls.zip = zipfile.ZipFile(cls.path)
        cls.names = set(cls.zip.namelist())
        cls.top = "zenzx-9.9.9-examples/"

    @classmethod
    def tearDownClass(cls):
        cls.zip.close()
        cls.tmp.cleanup()

    def test_one_folder_named_for_the_version(self):
        self.assertTrue(os.path.basename(self.path) == "zenzx-9.9.9-examples.zip")
        self.assertTrue(all(n.startswith(self.top) for n in self.names))

    def test_what_the_examples_need_is_there(self):
        for rel in be.REQUIRED + ["LICENSE", "NOTICE"]:
            self.assertIn(self.top + rel, self.names)

    def test_what_building_leaves_behind_is_not(self):
        for n in self.names:
            self.assertNotRegex(n, r"/build/|build\.log|\.gitignore|__pycache__|\.pyc$|/python/|/bin/")

    def test_every_picture_a_readme_shows_is_in_the_archive(self):
        shown = 0
        for n in sorted(self.names):
            if not n.endswith(".md"):
                continue
            text = self.zip.read(n).decode("utf-8")
            for m in re.finditer(r"\((\.\./[^)\s]*\.png)\)", text):
                target = os.path.normpath(os.path.join(os.path.dirname(n), m.group(1))).replace(os.sep, "/")
                self.assertIn(target, self.names, "%s shows %s" % (n, m.group(1)))
                shown += 1
        self.assertGreater(shown, 0)

    def test_programs_are_executable_and_the_windows_ones_keep_their_line_ends(self):
        for n in self.names:
            mode = (self.zip.getinfo(n).external_attr >> 16) & 0o777
            # a program (it starts with #!) is executable, and what it reads is not
            expected = 0o755 if self.zip.read(n).startswith(b"#!") else 0o644
            self.assertEqual(mode, expected, n)
            if n.endswith(".cmd"):
                data = self.zip.read(n)
                self.assertNotRegex(data, rb"(?<!\r)\n", n + " has a bare LF")

    def test_the_same_tree_gives_the_same_bytes(self):
        with tempfile.TemporaryDirectory() as d:
            again, _ = be.build("9.9.9", REPO, d)
            with open(again, "rb") as a, open(self.path, "rb") as b:
                self.assertEqual(hashlib.sha256(a.read()).hexdigest(), hashlib.sha256(b.read()).hexdigest())

    def test_a_missing_launcher_stops_the_build_and_writes_nothing(self):
        with tempfile.TemporaryDirectory() as d:
            copy = os.path.join(d, "repo")
            shutil.copytree(os.path.join(REPO, "examples"), os.path.join(copy, "examples"))
            for name in ("LICENSE", "NOTICE"):
                shutil.copy(os.path.join(REPO, name), copy)
            os.remove(os.path.join(copy, "examples", "world", "run.cmd"))
            out = os.path.join(d, "out")
            with self.assertRaises(be.BuildError) as cm:
                be.build("9.9.9", copy, out)
            self.assertIn("run.cmd", str(cm.exception))
            self.assertFalse(os.path.exists(os.path.join(out, "zenzx-9.9.9-examples.zip")))


if __name__ == "__main__":
    unittest.main()
