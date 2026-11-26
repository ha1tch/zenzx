#!/usr/bin/env python3
"""Tests for tools/package.py: the .deb is a valid, repeatable Debian package, the app bundle and
the WiX source have what they must, and a bad version or architecture is refused.
Run: python3 -W error scripts/test_package.py

Copyright (c) 2026 haitch - h@ual.li - https://oldbytes.space/@haitchfive
Licensed under the Apache License, Version 2.0: https://www.apache.org/licenses/LICENSE-2.0
"""
import gzip
import hashlib
import io
import os
import plistlib
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
import xml.etree.ElementTree as ET

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "tools"))
import package as pk  # noqa: E402


def fake_program(os_, glibc=None):
    """Bytes that pass build_site.check_program: the right first bytes and over a megabyte."""
    magic = {"linux": b"\x7fELF", "windows": b"MZ", "darwin": b"\xcf\xfa\xed\xfe"}[os_]
    body = bytearray(os.urandom(0) + b"\0" * 1_100_000)
    if glibc:
        body[100:100 + len(glibc)] = glibc
    return magic + bytes(body)


def read_ar(data):
    assert data[:8] == b"!<arch>\n"
    pos, out = 8, []
    while pos < len(data):
        head = data[pos:pos + 60]
        assert head[58:60] == b"`\n", head
        name = head[:16].decode().strip()
        size = int(head[48:58].decode())
        out.append((name, data[pos + 60:pos + 60 + size]))
        pos += 60 + size + (size % 2)
    return out


def untar(gz_bytes):
    t = tarfile.open(fileobj=io.BytesIO(gzip.decompress(gz_bytes)))
    return {m.name: (m, t.extractfile(m).read() if m.isfile() else None) for m in t.getmembers()}


class Deb(unittest.TestCase):
    def build(self, program=None, version="0.50.0", arch="amd64"):
        program = program or fake_program("linux")
        with tempfile.TemporaryDirectory() as d:
            path = pk.build_deb(program, version, arch, d)
            self.assertEqual(os.path.basename(path), "zenzx_%s_%s.deb" % (version, arch))
            with open(path, "rb") as f:
                return f.read()

    def test_is_an_ar_archive_of_three_members_in_the_order_dpkg_wants(self):
        members = read_ar(self.build())
        self.assertEqual([n for n, _ in members], ["debian-binary", "control.tar.gz", "data.tar.gz"])
        self.assertEqual(members[0][1], b"2.0\n")

    def test_control_fields(self):
        control = untar(dict(read_ar(self.build()))["control.tar.gz"])
        fields = dict(line.split(": ", 1) for line in control["./control"][1].decode().splitlines()
                      if line and not line.startswith(" "))
        self.assertEqual(fields["Package"], "zenzx")
        self.assertEqual(fields["Version"], "0.50.0")
        self.assertEqual(fields["Architecture"], "amd64")
        self.assertIn("libx11-6", fields["Depends"])
        self.assertIn("libasound2", fields["Depends"])
        self.assertTrue(fields["Maintainer"].endswith("<h@ual.li>"))
        self.assertTrue(control["./control"][1].endswith(b"\n"))

    def test_depends_on_the_glibc_the_program_names(self):
        deb = self.build(fake_program("linux", b"GLIBC_2.38\0GLIBC_2.17\0GLIBC_2.9\0"))
        control = untar(dict(read_ar(deb))["control.tar.gz"])["./control"][1].decode()
        self.assertIn("libc6 (>= 2.38)", control)
        self.assertEqual(pk.newest_glibc(b"x GLIBC_2.4 y GLIBC_2.31 z"), (2, 31))
        self.assertEqual(pk.newest_glibc(b"no versions"), (2, 17))

    def test_files_owners_modes_and_md5sums(self):
        parts = dict(read_ar(self.build()))
        data, control = untar(parts["data.tar.gz"]), untar(parts["control.tar.gz"])
        for name, (m, _) in data.items():
            self.assertEqual((m.uid, m.gid, m.uname, m.gname), (0, 0, "root", "root"), name)
        self.assertEqual(data["./usr/bin/zenzx"][0].mode, 0o755)
        self.assertEqual(data["./usr/share/applications/zenzx.desktop"][0].mode, 0o644)
        for size in pk.HICOLOR:
            path = "./usr/share/icons/hicolor/%dx%d/apps/zenzx.png" % (size, size)
            with open(os.path.join(pk.ICON_DIR, "png", "zenzx-%d.png" % size), "rb") as f:
                self.assertEqual(data[path][1], f.read(), path)
        sums = dict(reversed(l.split("  ", 1)) for l in control["./md5sums"][1].decode().splitlines())
        for name, (m, body) in data.items():
            if m.isfile():
                self.assertEqual(sums[name[2:]], hashlib.md5(body).hexdigest(), name)
        self.assertEqual(len(sums), sum(1 for m, _ in data.values() if m.isfile()))

    def test_every_directory_is_listed_before_its_files(self):
        names = list(untar(dict(read_ar(self.build()))["data.tar.gz"]))   # tarfile drops the slash
        for i, n in enumerate(names):
            parent = n.rsplit("/", 1)[0]
            if n != ".":
                self.assertIn(parent, names[:i], n)

    def test_desktop_entry_names_the_program_and_the_icon(self):
        text = untar(dict(read_ar(self.build()))["data.tar.gz"])["./usr/share/applications/zenzx.desktop"][1].decode()
        for line in ("Type=Application", "Exec=zenzx", "Icon=zenzx", "Terminal=false"):
            self.assertIn(line, text)
        self.assertNotIn("MimeType", text)      # no associations: T-94

    def test_the_same_inputs_make_the_same_bytes(self):
        prog = fake_program("linux")
        old = os.environ.get("SOURCE_DATE_EPOCH")
        os.environ["SOURCE_DATE_EPOCH"] = "1700000000"
        try:
            self.assertEqual(self.build(prog), self.build(prog))
        finally:
            if old is None:
                del os.environ["SOURCE_DATE_EPOCH"]
            else:
                os.environ["SOURCE_DATE_EPOCH"] = old

    @unittest.skipUnless(shutil.which("dpkg-deb"), "dpkg-deb is not installed")
    def test_dpkg_deb_reads_it(self):
        with tempfile.TemporaryDirectory() as d:
            path = pk.build_deb(fake_program("linux"), "0.50.0", "arm64", d)
            info = subprocess.run(["dpkg-deb", "-I", path], capture_output=True, text=True, check=True).stdout
            self.assertIn("Architecture: arm64", info)
            listing = subprocess.run(["dpkg-deb", "-c", path], capture_output=True, text=True, check=True).stdout
            self.assertIn("./usr/bin/zenzx", listing)
            out = os.path.join(d, "x")
            subprocess.run(["dpkg-deb", "-x", path, out], check=True)
            self.assertTrue(os.access(os.path.join(out, "usr", "bin", "zenzx"), os.X_OK))

    def test_bad_version_or_architecture_is_refused(self):
        for v in ("0.50", "v0.50.0", "0.50.0-rc1", ""):
            with self.assertRaises(pk.PackageError, msg=v):
                pk.build_deb(fake_program("linux"), v, "amd64", tempfile.gettempdir())
        with self.assertRaises(pk.PackageError):
            pk.build_deb(fake_program("linux"), "0.50.0", "i386", tempfile.gettempdir())


class App(unittest.TestCase):
    def test_bundle(self):
        prog = fake_program("darwin")
        with tempfile.TemporaryDirectory() as d:
            app = pk.build_app(prog, "0.50.0", d)
            self.assertEqual(os.path.basename(app), "ZenZX.app")
            exe = os.path.join(app, "Contents", "MacOS", "zenzx")
            self.assertTrue(os.access(exe, os.X_OK))
            with open(exe, "rb") as f:
                self.assertEqual(f.read(), prog)
            with open(os.path.join(app, "Contents", "Info.plist"), "rb") as f:
                plist = plistlib.load(f)
            self.assertEqual(plist["CFBundleExecutable"], "zenzx")
            self.assertEqual(plist["CFBundleIdentifier"], "io.github.ha1tch.zenzx")
            self.assertEqual(plist["CFBundleShortVersionString"], "0.50.0")
            self.assertEqual(plist["CFBundleIconFile"], "zenzx")
            self.assertEqual(plist["LSMinimumSystemVersion"], "12.0")
            self.assertTrue(plist["NSHighResolutionCapable"])
            self.assertNotIn("CFBundleDocumentTypes", plist)    # no associations: T-94
            with open(os.path.join(app, "Contents", "Resources", "zenzx.icns"), "rb") as f, \
                    open(os.path.join(pk.ICON_DIR, "zenzx.icns"), "rb") as g:
                self.assertEqual(f.read(), g.read())

    def test_building_again_replaces_the_old_bundle(self):
        with tempfile.TemporaryDirectory() as d:
            pk.build_app(fake_program("darwin"), "0.50.0", d)
            stray = os.path.join(d, "ZenZX.app", "Contents", "stray")
            open(stray, "w").close()
            pk.build_app(fake_program("darwin"), "0.50.1", d)
            self.assertFalse(os.path.exists(stray))

    @unittest.skipIf(sys.platform == "darwin", "this machine can make a .dmg")
    def test_dmg_needs_macos(self):
        with self.assertRaises(pk.PackageError):
            pk.build_dmg("ZenZX.app", "ZenZX.dmg")


class Msi(unittest.TestCase):
    NS = {"w": "http://schemas.microsoft.com/wix/2006/wi"}

    def test_source(self):
        prog = fake_program("windows")
        for arch in ("x64", "arm64"):
            with tempfile.TemporaryDirectory() as d:
                path = pk.build_msi_source(prog, "0.50.0", arch, d)
                root = ET.parse(path).getroot()
                product = root.find("w:Product", self.NS)
                self.assertEqual(product.get("Version"), "0.50.0")
                self.assertEqual(product.get("UpgradeCode"), "6B1C8A40-3F6E-4C1E-9A53-2E1D7F0B7A11")
                package = product.find("w:Package", self.NS)
                self.assertEqual(package.get("Platform"), arch)
                self.assertEqual(package.get("InstallScope"), "perUser")
                upgrade = product.find("w:MajorUpgrade", self.NS)
                self.assertIsNotNone(upgrade)
                # The Windows 7 and the Windows 10 package have one version: either replaces the other.
                self.assertEqual(upgrade.get("AllowSameVersionUpgrades"), "yes")
                self.assertEqual(product.find(".//w:File", self.NS).get("Name"), "zenzx.exe")
                # Every component of a per-user install must be keyed to the user's own area.
                for comp in product.iter("{%s}Component" % self.NS["w"]):
                    self.assertTrue(comp.find(".//*[@KeyPath='yes']", self.NS) is not None, comp.get("Id"))
                with open(os.path.join(d, "zenzx.exe"), "rb") as f:
                    self.assertEqual(f.read(), prog)
                with open(os.path.join(d, "zenzx.ico"), "rb") as f, \
                        open(os.path.join(pk.ICON_DIR, "zenzx.ico"), "rb") as g:
                    self.assertEqual(f.read(), g.read())

    def test_the_upgrade_code_never_changes(self):
        # A new code would make a new version install beside the old one instead of replacing it.
        self.assertEqual(pk.UPGRADE_CODE, "6B1C8A40-3F6E-4C1E-9A53-2E1D7F0B7A11")

    def test_bad_arch_or_version_is_refused(self):
        with self.assertRaises(pk.PackageError):
            pk.build_msi_source(fake_program("windows"), "0.50.0", "amd64", tempfile.gettempdir())
        with self.assertRaises(pk.PackageError):
            pk.build_msi_source(fake_program("windows"), "0.50", "x64", tempfile.gettempdir())


class Program(unittest.TestCase):
    def test_a_file_that_is_not_a_program_is_refused(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "x")
            with open(p, "wb") as f:
                f.write(b"MZ" + b"\0" * 2_000_000)
            rc = pk.main(["deb", "--program", p, "--version", "0.50.0", "--arch", "amd64", "--out", d])
            self.assertEqual(rc, 1)             # a Windows file where a Linux one is wanted

    def test_a_small_file_is_refused(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "x")
            with open(p, "wb") as f:
                f.write(b"\x7fELF" + b"\0" * 100)
            self.assertEqual(pk.main(["deb", "--program", p, "--version", "0.50.0",
                                      "--arch", "amd64", "--out", d]), 1)


if __name__ == "__main__":
    unittest.main()
