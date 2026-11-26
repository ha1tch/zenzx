#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Tests for tools/build_manual.py: the page the PDF is printed from, and, where a Chrome is
installed, the PDF itself.

    python3 scripts/test_build_manual.py
"""
import importlib.util
import os
import re
import shutil
import tempfile
import unittest

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
spec = importlib.util.spec_from_file_location("build_manual", os.path.join(REPO, "tools", "build_manual.py"))
bm = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bm)

try:
    import markdown  # noqa: F401
    HAVE_MARKDOWN = True
except ImportError:
    HAVE_MARKDOWN = False


@unittest.skipUnless(HAVE_MARKDOWN, "the markdown package is not installed")
class PageTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.page = bm.manual_html("9.9.9", REPO)

    def test_the_cover_has_the_title_and_the_version(self):
        self.assertIn('<div class="cover"><h1>ZenZX Manual</h1><p>Version 9.9.9</p>', self.page)
        self.assertEqual(self.page.count("<h1"), 1)

    def test_every_chapter_of_the_manual_is_in_the_contents_once(self):
        with open(os.path.join(REPO, "docs", "MANUAL.md"), encoding="utf-8") as f:
            chapters = re.findall(r"^## (.+)$", f.read(), re.M)
        chapters = [c for c in chapters if c != "Contents"]
        contents = self.page.split('<div class="contents">')[1].split("</div>")[0]
        for c in chapters:
            self.assertEqual(contents.count(">%s</a>" % c.replace("&", "&amp;")), 1, c)
        self.assertNotIn(">Contents</a>", contents)
        # and the manual's own list of contents, with its links that go nowhere in a file, is gone
        self.assertEqual(self.page.count("<h2 id=\"contents\">"), 0)

    def test_every_chapter_has_its_heading_in_the_body(self):
        with open(os.path.join(REPO, "docs", "MANUAL.md"), encoding="utf-8") as f:
            chapters = [c for c in re.findall(r"^## (.+)$", f.read(), re.M) if c != "Contents"]
        for c in chapters:
            self.assertRegex(self.page, r"<h2 id=\"[^\"]+\">%s</h2>" % re.escape(c.replace("&", "&amp;")), c)

    def test_pictures_are_files_and_documents_are_the_tag_on_github(self):
        imgs = re.findall(r'<img\b[^>]*\bsrc="([^"]+)"', self.page)
        self.assertGreater(len(imgs), 0)
        for src in imgs:
            self.assertTrue(src.startswith("file://"), src)
            self.assertTrue(os.path.isfile(src[len("file://"):]), src)
        hrefs = re.findall(r'<a\b[^>]*\bhref="([^"]+)"', self.page)
        for h in hrefs:
            self.assertTrue(h.startswith(("#", "http://", "https://")), "a relative link is left: " + h)
        self.assertIn("https://github.com/ha1tch/zenzx/blob/v9.9.9/docs/gui.md", self.page)
        self.assertIn("https://github.com/ha1tch/zenzx/blob/v9.9.9/README.md", self.page)
        self.assertIn("https://github.com/ha1tch/zenzx/blob/v9.9.9/docs/gui.md#top-menu-bar", self.page)

    def test_a_line_that_starts_with_a_hash_is_not_a_heading(self):
        # python-markdown reads "#B6B6B6" at the start of a line as a heading; GitHub does not.
        # The manual is written so that neither does.
        with open(os.path.join(REPO, "docs", "MANUAL.md"), encoding="utf-8") as f:
            for n, line in enumerate(f, 1):
                self.assertFalse(re.match(r"^#[^# ]", line), "MANUAL.md line %d starts with #: %s" % (n, line.strip()))

    def test_a_link_to_a_file_that_is_not_there_stops_the_build(self):
        with tempfile.TemporaryDirectory() as d:
            os.makedirs(os.path.join(d, "docs"))
            with open(os.path.join(d, "docs", "MANUAL.md"), "w", encoding="utf-8") as f:
                f.write("# T\n\nintro\n\n## One\n\nsee [x](nothere.md)\n")
            with self.assertRaises(bm.BuildError) as cm:
                bm.manual_html("1.0.0", d)
            self.assertIn("nothere.md", str(cm.exception))

    def test_a_picture_that_is_not_there_stops_the_build(self):
        with tempfile.TemporaryDirectory() as d:
            os.makedirs(os.path.join(d, "docs"))
            with open(os.path.join(d, "docs", "MANUAL.md"), "w", encoding="utf-8") as f:
                f.write("# T\n\nintro\n\n## One\n\n![x](img/nothere.png)\n")
            with self.assertRaises(bm.BuildError) as cm:
                bm.manual_html("1.0.0", d)
            self.assertIn("nothere.png", str(cm.exception))


@unittest.skipUnless(HAVE_MARKDOWN and bm.find_chrome(), "no markdown package, or no Chrome or Chromium")
class PdfTests(unittest.TestCase):
    def test_the_pdf_is_a_pdf_of_the_manual_with_its_pictures(self):
        with tempfile.TemporaryDirectory() as d:
            path, pages = bm.build("9.9.9", REPO, d)
            self.assertEqual(os.path.basename(path), "zenzx-9.9.9-manual.pdf")
            self.assertGreaterEqual(pages, bm.MIN_PAGES)
            with open(path, "rb") as f:
                data = f.read()
            self.assertTrue(data.startswith(b"%PDF-"))
            self.assertGreaterEqual(len(re.findall(rb"/Subtype\s*/Image", data)), 8)

    def test_no_chrome_stops_the_build_and_writes_nothing(self):
        with tempfile.TemporaryDirectory() as d:
            old = os.environ.pop("CHROME", None)
            oldpath = os.environ["PATH"]
            os.environ["PATH"] = d
            try:
                with self.assertRaises(bm.BuildError):
                    bm.build("9.9.9", REPO, os.path.join(d, "out"), chrome=os.path.join(d, "nochrome"))
            finally:
                os.environ["PATH"] = oldpath
                if old:
                    os.environ["CHROME"] = old
            self.assertFalse(os.path.exists(os.path.join(d, "out")))


if __name__ == "__main__":
    unittest.main()
