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
import shutil
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


def make_extras(folder, version=VERSION, only=None):
    """The release's installers and Windows 7 files, each with bytes of its own; {name: bytes}."""
    made = {}
    for _group, _system, _detail, pattern in build_site.EXTRAS:
        name = pattern.replace("{v}", version)
        if only is not None and name not in [p.replace("{v}", version) for p in only]:
            continue
        made[name] = f"the bytes of {name}\n".encode() * 1000
        with open(os.path.join(folder, name), "wb") as f:
            f.write(made[name])
    # A checksum file sits beside them on the release, and is not a file the page lists.
    with open(os.path.join(folder, f"zenzx-{version}-installers.sha256"), "w") as f:
        f.write("not listed\n")
    return made


def slurp(path, mode="rb", **kw):
    with open(path, mode, **kw) as f:
        return f.read()


def run(arch_dir, out, version=VERSION, extras=None):
    err = io.StringIO()
    args = ["--version", version, "--archives", arch_dir, "--out", out]
    if extras:
        args += ["--extras", extras]
    with contextlib.redirect_stderr(err), contextlib.redirect_stdout(io.StringIO()):
        code = build_site.main(args)
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

    def extras_folder(self):
        folder = os.path.join(self.tmp.name, "extras")
        os.mkdir(folder)
        return folder

    def test_the_installers_and_the_windows_7_program_are_listed(self):
        make_archives(self.arch)
        extras = self.extras_folder()
        made = make_extras(extras)
        code, err = run(self.arch, self.out, extras=extras)
        self.assertEqual(code, 0, err)
        index = slurp(os.path.join(self.out, "index.html"), "r", encoding="utf-8")
        base = f"https://github.com/ha1tch/zenzx/releases/download/v{VERSION}/"
        for name, data in made.items():
            self.assertIn(f'href="{base}{name}"', index, name)
            self.assertIn(hashlib.sha256(data).hexdigest(), index, name)
        self.assertEqual(len(made), len(build_site.EXTRAS))
        self.assertIn("<h2>Installers</h2>", index)
        self.assertIn("<h2>Windows 7</h2>", index)
        self.assertIn("<h2>Manual and examples</h2>", index)
        self.assertIn("Python 3", index)
        self.assertIn("not signed", index)
        self.assertNotIn("not listed", index)
        # What a client reads, and what the page itself holds, are the programs and nothing more.
        manifest = json.loads(slurp(os.path.join(self.out, "runners.json"), "r", encoding="utf-8"))
        self.assertEqual(len(manifest["runners"]), len(build_site.PLATFORMS))
        self.assertNotIn(".msi", slurp(os.path.join(self.out, "runners.json"), "r", encoding="utf-8"))
        self.assertEqual(len(os.listdir(os.path.join(self.out, "bin"))), len(build_site.PLATFORMS) + 1)

    def test_without_extras_the_page_has_neither_section(self):
        make_archives(self.arch)
        for extras in (None, os.path.join(self.tmp.name, "not-there")):
            code, err = run(self.arch, self.out, extras=extras)
            self.assertEqual(code, 0, err)
            index = slurp(os.path.join(self.out, "index.html"), "r", encoding="utf-8")
            self.assertNotIn("<h2>Installers</h2>", index)
            self.assertNotIn("<h2>Windows 7</h2>", index)
            self.assertNotIn("<h2>Manual and examples</h2>", index)
            self.assertNotIn("not signed", index)
            self.assertNotIn("{extras}", index)

    def test_a_partial_set_lists_what_there_is(self):
        make_archives(self.arch)
        extras = self.extras_folder()
        only = ["zenzx_{v}_arm64.deb", "zenzx-{v}-windows7-amd64.zip"]
        made = make_extras(extras, only=only)
        self.assertEqual(len(made), 2)
        code, err = run(self.arch, self.out, extras=extras)
        self.assertEqual(code, 0, err)
        index = slurp(os.path.join(self.out, "index.html"), "r", encoding="utf-8")
        self.assertIn(f"zenzx_{VERSION}_arm64.deb", index)
        self.assertIn(f"zenzx-{VERSION}-windows7-amd64.zip", index)
        # Nothing is said about an installer that is not there.
        for gone in (".dmg", "SmartScreen", "Gatekeeper", "Open Anyway", "_amd64.deb", "windows-amd64.msi",
                     "Manual and examples", "-manual.pdf", "-examples.zip", 'id="manual"'):
            self.assertNotIn(gone, index, gone)
        self.assertNotIn("not signed", index)

    def test_extras_of_another_version_are_not_listed(self):
        make_archives(self.arch)
        extras = self.extras_folder()
        make_extras(extras, version="1.0.0")
        code, err = run(self.arch, self.out, extras=extras)
        self.assertEqual(code, 0, err)
        index = slurp(os.path.join(self.out, "index.html"), "r", encoding="utf-8")
        self.assertNotIn("1.0.0", index)
        self.assertNotIn("<h2>Installers</h2>", index)

    def test_an_empty_extra_stops_the_build(self):
        make_archives(self.arch)
        extras = self.extras_folder()
        with open(os.path.join(extras, f"zenzx-{VERSION}-macos-universal.dmg"), "wb"):
            pass
        code, err = run(self.arch, self.out, extras=extras)
        self.assertEqual(code, 1)
        self.assertIn("is empty", err)


class PageContents(unittest.TestCase):
    """The page a person reads: it offers what the release has, finds its own files, and draws
    itself with the program's own pictures and face."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.arch = os.path.join(self.tmp.name, "dist")
        self.out = os.path.join(self.tmp.name, "site")
        self.extras = os.path.join(self.tmp.name, "extras")
        os.mkdir(self.arch)
        os.mkdir(self.extras)
        make_archives(self.arch)

    def build(self, with_extras=True):
        if with_extras:
            make_extras(self.extras)
        code, err = run(self.arch, self.out, extras=self.extras if with_extras else None)
        self.assertEqual(code, 0, err)
        return slurp(os.path.join(self.out, "index.html"), "r", encoding="utf-8")

    def test_every_system_has_a_place_and_every_program_a_plain_description(self):
        index = self.build()
        for os_, os_id in (("windows", "windows"), ("darwin", "macos"), ("linux", "linux")):
            self.assertIn(f'<section class="os" id="os-{os_id}" data-os="{os_}">', index)
        for os_, arch in build_site.PLATFORMS:
            m = re.search(r'<a class="dl" href="bin/%s"[^>]*data-os="%s" data-arch="%s" data-kind="program"' % (
                re.escape(build_site.exe_name(os_, arch)), os_, arch), index)
            self.assertTrue(m, f"{os_}/{arch} is not offered in its system's part of the page")
        # What each build is for is said without the word amd64 having to be understood.
        for words in ("Intel or AMD PC (64-bit)", "Windows on ARM", "Apple silicon", "A Mac with an Intel processor",
                      "uname -m", "System type", "About This Mac"):
            self.assertIn(words, index, words)

    def test_installers_are_offered_beside_the_programs_and_marked_as_installers(self):
        index = self.build()
        base = f"https://github.com/ha1tch/zenzx/releases/download/v{VERSION}/"
        for name, os_, arch in (
            (f"zenzx-{VERSION}-windows-amd64.msi", "windows", "amd64"),
            (f"zenzx-{VERSION}-windows-arm64.msi", "windows", "arm64"),
            (f"zenzx-{VERSION}-macos-universal.dmg", "darwin", "universal"),
            (f"zenzx_{VERSION}_amd64.deb", "linux", "amd64"),
            (f"zenzx_{VERSION}_arm64.deb", "linux", "arm64"),
            (f"zenzx-{VERSION}-windows7-amd64.msi", "windows7", "amd64"),
            (f"zenzx-{VERSION}-windows7-amd64.zip", "windows7", "amd64"),
        ):
            self.assertIn(f'<a class="dl" href="{base}{name}" data-os="{os_}" data-arch="{arch}" data-kind="installer"',
                          index, name)
        self.assertIn("The manual (PDF", index)
        self.assertIn("The examples (", index)
        # the manual has a panel of its own, and a link in the header that goes straight to the PDF
        self.assertIn('<section class="panel" id="manual">', index)
        self.assertIn(f'<a href="{base}zenzx-{VERSION}-manual.pdf">Manual</a>', index)
        self.assertIn("Help &gt; ZenZX Manual", index)

    def test_without_extras_the_page_offers_only_the_programs(self):
        index = self.build(with_extras=False)
        self.assertNotIn('data-kind="installer"', index)
        self.assertEqual(index.count('data-kind="program"'), len(build_site.PLATFORMS))
        self.assertNotIn("The manual (PDF", index)
        self.assertNotIn('id="manual"', index)
        self.assertNotIn(">Manual</a>", index)
        self.assertNotIn("On Windows 7", index)

    def test_every_file_the_page_names_is_in_the_site_and_nothing_is_fetched_from_elsewhere(self):
        index = self.build()
        refs = re.findall(r'(?:src|href)="([^"]+)"', index) + re.findall(r"url\((assets/[^)]+)\)", index)
        self.assertTrue(refs)
        for ref in refs:
            if ref.startswith("#") or ref.startswith("https://github.com/"):
                continue
            self.assertFalse(re.match(r"[a-z]+:", ref), f"{ref} leaves the site")
            self.assertTrue(os.path.isfile(os.path.join(self.out, ref)), f"{ref} is not in the site")
        self.assertNotIn("<script src", index)
        self.assertNotIn("@import", index)

    def test_the_page_is_drawn_with_the_programs_own_face_and_pictures(self):
        index = self.build()
        for name in ("spleen-5x8.woff", "spleen-5x8-proportional.woff"):
            self.assertEqual(slurp(os.path.join(self.out, "assets", name))[:4], b"wOFF", name)
        self.assertIn("Spleen", slurp(os.path.join(self.out, "assets", "LICENSE-Spleen.txt"), "r"))
        # The icon is the repository's, byte for byte, and so are the figure's pictures.
        root = os.path.join(HERE, "..")
        self.assertEqual(slurp(os.path.join(self.out, "assets", "speccy.png")),
                         slurp(os.path.join(root, "assets", "icon", "speccy.png")))
        frames = sorted(n for n in os.listdir(os.path.join(root, "sprites", "wait")) if n.startswith("frame"))
        self.assertEqual(sorted(os.listdir(os.path.join(self.out, "assets", "figure"))), frames)
        for n in frames:
            self.assertEqual(slurp(os.path.join(self.out, "assets", "figure", n)),
                             slurp(os.path.join(root, "sprites", "wait", n)))
        # Spleen has the characters of Latin-1 and no more: the text must not need another face.
        text = re.sub(r"<script.*?</script>|<style.*?</style>", "", index, flags=re.S)
        odd = sorted({c for c in text if ord(c) > 0xFF})
        self.assertEqual(odd, [], "characters the face does not have")

    def test_the_figure_does_what_its_sequence_file_says(self):
        index = self.build()
        found = re.search(r'<script type="application/json" id="figure-data">(.*?)</script>', index, re.S)
        data = json.loads(found.group(1))
        n = len(data["frames"])
        self.assertEqual(n, 10)
        self.assertEqual(data["intro"][0], [0, 0.25])
        self.assertTrue(data["loop"])
        for i, secs in data["intro"] + data["loop"]:
            self.assertTrue(0 <= i < n and secs > 0)
        self.assertIn(data["frames"][data["rest"]], index)      # the picture before, or without, a script
        self.assertEqual(data["rest"], 2)                       # the figure at rest

    def test_a_sequence_that_names_a_missing_picture_stops_the_build(self):
        saved = build_site.FIGURE_DIR
        bad = os.path.join(self.tmp.name, "wait")
        shutil.copytree(saved, bad)
        with open(os.path.join(bad, "sequence.txt"), "w") as f:
            f.write("99 1.0\n")
        build_site.FIGURE_DIR = bad
        try:
            with self.assertRaises(build_site.BuildError):
                build_site.read_figure()
        finally:
            build_site.FIGURE_DIR = saved


try:
    from playwright.sync_api import sync_playwright
except ImportError:          # the browser tests below need Playwright and its Chromium
    sync_playwright = None

AGENTS = {
    "windows": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36",
    "windows 7": "Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115.0",
    "windows 32-bit": "Mozilla/5.0 (Windows NT 10.0; rv:109.0) Gecko/20100101 Firefox/115.0",
    "mac": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
    "linux": "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0",
    "linux arm": "Mozilla/5.0 (X11; Linux aarch64; rv:120.0) Gecko/20100101 Firefox/120.0",
    "iphone": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1",
}
# What the page marks as the one for each, by file name ("" when it does not choose).
RECOMMENDED = {
    "windows": f"zenzx-{VERSION}-windows-amd64.msi",
    "windows 7": f"zenzx-{VERSION}-windows7-amd64.msi",
    "windows 32-bit": "",
    "mac": f"zenzx-{VERSION}-macos-universal.dmg",
    "linux": "zenzx-linux-amd64",
    "linux arm": "zenzx-linux-arm64",
    "iphone": "",
}


@unittest.skipUnless(sync_playwright, "Playwright is not installed")
class PageInABrowser(unittest.TestCase):
    """The script of the page, in Chromium, as each kind of computer says it is."""

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        arch, extras = os.path.join(cls.tmp.name, "dist"), os.path.join(cls.tmp.name, "extras")
        os.mkdir(arch)
        os.mkdir(extras)
        make_archives(arch)
        make_extras(extras)
        cls.out = os.path.join(cls.tmp.name, "site")
        code, err = run(arch, cls.out, extras=extras)
        assert code == 0, err
        cls.pw = sync_playwright().start()
        try:
            cls.browser = cls.pw.chromium.launch()
        except Exception as ex:      # no Chromium to launch: the tests are skipped, not failed
            cls.pw.stop()
            cls.tmp.cleanup()
            raise unittest.SkipTest(f"no Chromium: {ex}")

    @classmethod
    def tearDownClass(cls):
        cls.browser.close()
        cls.pw.stop()
        cls.tmp.cleanup()

    def open(self, agent, **kw):
        ctx = self.browser.new_context(user_agent=agent, **kw)
        # Chromium's client hints would answer for this computer, not for the one the agent names.
        ctx.add_init_script("Object.defineProperty(navigator, 'userAgentData', {get: () => undefined});")
        self.addCleanup(ctx.close)
        page = ctx.new_page()
        errors = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto("file://" + os.path.join(self.out, "index.html"))
        page.wait_for_timeout(200)
        self.assertEqual(errors, [])
        return page

    def test_each_kind_of_computer_is_shown_the_build_that_fits(self):
        for kind, agent in AGENTS.items():
            page = self.open(agent)
            got = page.locator(".choice.rec a.dl .fn").all_inner_texts()
            self.assertEqual(got, [RECOMMENDED[kind]] if RECOMMENDED[kind] else [], kind)
            if RECOMMENDED[kind]:
                big = page.locator("#pick a.big")
                self.assertTrue(big.is_visible(), kind)
                self.assertTrue(big.get_attribute("href").endswith(RECOMMENDED[kind]), kind)
                self.assertIn(f"ZenZX {VERSION}", big.inner_text(), kind)
            else:
                self.assertFalse(page.locator("#pick").is_visible(), kind)

    def test_the_tab_of_the_computers_system_is_the_one_open_and_the_others_can_be_chosen(self):
        page = self.open(AGENTS["mac"])
        self.assertTrue(page.locator("#os-macos").is_visible())
        self.assertFalse(page.locator("#os-windows").is_visible())
        page.get_by_role("tab", name="Linux").click()
        self.assertTrue(page.locator("#os-linux").is_visible())
        self.assertFalse(page.locator("#os-macos").is_visible())

    def test_a_32_bit_windows_is_told_there_is_no_build_for_it(self):
        text = self.open(AGENTS["windows 32-bit"]).locator("#you").inner_text()
        self.assertIn("32-bit", text)
        self.assertIn("no 32-bit build", text)

    def test_without_a_script_every_system_is_listed(self):
        page = self.open(AGENTS["windows"], java_script_enabled=False)
        for os_id in ("windows", "macos", "linux"):
            self.assertTrue(page.locator(f"#os-{os_id}").is_visible(), os_id)

    def test_the_page_fits_a_phone_without_scrolling_sideways(self):
        page = self.open(AGENTS["iphone"], viewport={"width": 390, "height": 800})
        self.assertLessEqual(page.evaluate("document.documentElement.scrollWidth"), 390)

    def test_the_figure_moves_unless_the_person_asked_for_none(self):
        page = self.open(AGENTS["windows"])
        seen = set()
        for _ in range(8):
            seen.add(page.locator("#figure").get_attribute("src"))
            page.wait_for_timeout(400)
        self.assertGreater(len(seen), 1)
        still = self.open(AGENTS["windows"], reduced_motion="reduce")
        first = still.locator("#figure").get_attribute("src")
        still.wait_for_timeout(1500)
        self.assertEqual(still.locator("#figure").get_attribute("src"), first)


if __name__ == "__main__":
    unittest.main()
