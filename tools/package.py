#!/usr/bin/env python3
"""package.py - the installers: a .deb for Linux, a ZenZX.app and .dmg for macOS, and the WiX source
for a Windows .msi.

Each one is made from the GUI program of a release (the archives the release job published), so
that an installer holds the release's own bytes and not a second build.

  deb       a Debian package: zenzx_<version>_<amd64|arm64>.deb. Written here with the standard
            library (an ar archive of two tar files), so no dpkg is needed to make it.
  app       ZenZX.app: the program, its Info.plist and its icon.
  dmg       a disk image holding ZenZX.app and a link to /Applications (macOS only: hdiutil).
  msi-source  zenzx.wxs for WiX 3.14, with zenzx.exe and zenzx.ico beside it. The workflow runs
            candle and light on it.

The program is either a file (--program) or a release archive (--archive), such as
zenzx-0.50.0-linux-amd64.tar.gz.

There are no file associations: ZenZX ignores a file named on its command line (T-94), so an
association would start the emulator with nothing in it.

Usage: python3 tools/package.py deb --archive A --version V --arch amd64 --out DIR
       python3 tools/package.py app --program P --version V --out DIR
       python3 tools/package.py dmg --app DIR/ZenZX.app --out ZenZX.dmg
       python3 tools/package.py msi-source --archive A --version V --arch x64 --out DIR

Standard library only (Python 3.8 or later).

Copyright (c) 2026 haitch - h@ual.li - https://oldbytes.space/@haitchfive
Licensed under the Apache License, Version 2.0: https://www.apache.org/licenses/LICENSE-2.0
"""
import argparse
import gzip
import hashlib
import io
import os
import plistlib
import re
import shutil
import subprocess
import sys
import tarfile
from xml.sax.saxutils import escape, quoteattr

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.join(HERE, "..")
sys.path.insert(0, HERE)
import build_site  # noqa: E402  (the archive reading the page already uses)

NAME = "zenzx"
TITLE = "ZenZX"
SUMMARY = "ZX Spectrum emulator"
HOMEPAGE = "https://github.com/ha1tch/zenzx"
MAINTAINER = "haitch <h@ual.li>"
BUNDLE_ID = "io.github.ha1tch.zenzx"
# Windows Installer's identity for ZenZX: the same in every version, so that a newer MSI replaces
# an older one. It must never change.
UPGRADE_CODE = "6B1C8A40-3F6E-4C1E-9A53-2E1D7F0B7A11"
ICON_DIR = os.path.join(ROOT, "assets", "icon")
HICOLOR = (64, 128, 256, 512)          # the sizes of assets/icon/png that hicolor takes
MIN_MACOS = "12.0"                     # Go 1.25 needs macOS 12 or later


class PackageError(Exception):
    pass


def epoch():
    """The time put on every file: SOURCE_DATE_EPOCH, or 0, so that a package is repeatable."""
    return int(os.environ.get("SOURCE_DATE_EPOCH", "0"))


def program_bytes(args, os_):
    if args.program:
        with open(args.program, "rb") as f:
            data = f.read()
    elif args.archive:
        data = build_site.read_program(args.archive, os_)
    else:
        raise PackageError("give --program or --archive")
    build_site.check_program(data, os_, getattr(args, "arch", "") or "?", args.program or args.archive)
    return data


def read_icon(name):
    with open(os.path.join(ICON_DIR, name), "rb") as f:
        return f.read()


def check_version(v):
    if not re.fullmatch(r"\d+\.\d+\.\d+", v):
        raise PackageError("version %r is not major.minor.patch" % v)
    return v


# ---------------------------------------------------------------------------------- .deb

def newest_glibc(data):
    """The highest GLIBC_x.y[.z] a program names (the versions it was linked against)."""
    best = (2, 17)
    for m in re.finditer(rb"GLIBC_(\d+)\.(\d+)", data):
        best = max(best, (int(m.group(1)), int(m.group(2))))
    return best


def tar_bytes(entries):
    """A tar of (name, data or None for a directory, mode) with root ownership and one time."""
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w", format=tarfile.GNU_FORMAT) as t:
        for name, data, mode in entries:
            info = tarfile.TarInfo(name)
            info.mtime, info.uid, info.gid, info.uname, info.gname = epoch(), 0, 0, "root", "root"
            info.mode = mode
            if data is None:
                info.type = tarfile.DIRTYPE
                t.addfile(info)
            else:
                info.size = len(data)
                t.addfile(info, io.BytesIO(data))
    return buf.getvalue()


def gz(data):
    out = io.BytesIO()
    with gzip.GzipFile(fileobj=out, mode="wb", mtime=0, compresslevel=9) as g:
        g.write(data)
    return out.getvalue()


def ar_archive(members):
    """A Debian package is an ar archive: a name, a time, owner 0/0, mode, size, then the data."""
    out = b"!<arch>\n"
    for name, data in members:
        head = "%-16s%-12d%-6d%-6d%-8s%-10d`\n" % (name, epoch(), 0, 0, "100644", len(data))
        assert len(head) == 60
        out += head.encode("ascii") + data + (b"\n" if len(data) % 2 else b"")
    return out


DESKTOP = """[Desktop Entry]
Type=Application
Name=ZenZX
GenericName=ZX Spectrum emulator
Comment=Emulates the ZX Spectrum, Spectrum 128K, +2, +2A, +3 and Timex Sinclair 2068
Exec=zenzx
Icon=zenzx
Terminal=false
Categories=System;Emulator;
Keywords=Spectrum;ZX;Sinclair;emulator;
"""

COPYRIGHT = """Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: ZenZX
Source: https://github.com/ha1tch/zenzx

Files: *
Copyright: 2026 haitch
License: Apache-2.0

License: Apache-2.0
 Licensed under the Apache License, Version 2.0. On Debian systems the full text is in
 /usr/share/common-licenses/Apache-2.0, and at https://www.apache.org/licenses/LICENSE-2.0
"""


def build_deb(program, version, arch, out_dir):
    check_version(version)
    if arch not in ("amd64", "arm64"):
        raise PackageError("arch must be amd64 or arm64")
    major, minor = newest_glibc(program)
    files = [("usr/bin/zenzx", program, 0o755),
             ("usr/share/applications/zenzx.desktop", DESKTOP.encode(), 0o644),
             ("usr/share/doc/zenzx/copyright", COPYRIGHT.encode(), 0o644)]
    for size in HICOLOR:
        files.append(("usr/share/icons/hicolor/%dx%d/apps/zenzx.png" % (size, size),
                      read_icon("png/zenzx-%d.png" % size), 0o644))
    notice = os.path.join(ROOT, "NOTICE")
    if os.path.exists(notice):
        with open(notice, "rb") as f:
            files.append(("usr/share/doc/zenzx/NOTICE", f.read(), 0o644))
    files.sort()

    dirs = sorted({d for name, _, _ in files for d in parents(name)})
    data_entries = [("./", None, 0o755)] + [("./%s/" % d, None, 0o755) for d in dirs]
    data_entries += [("./" + name, data, mode) for name, data, mode in files]

    md5s = "".join("%s  %s\n" % (hashlib.md5(data).hexdigest(), name) for name, data, _ in files)
    installed_kib = (sum(len(d) for _, d, _ in files) + 1023) // 1024
    control = (
        "Package: zenzx\n"
        "Version: %s\n"
        "Architecture: %s\n"
        "Maintainer: %s\n"
        "Installed-Size: %d\n"
        "Depends: libc6 (>= %d.%d), libasound2t64 | libasound2, libx11-6\n"
        "Recommends: libgl1\n"
        "Section: otherosfs\n"
        "Priority: optional\n"
        "Homepage: %s\n"
        "Description: %s\n"
        " ZenZX emulates the 48K, 128K, +2, +2A and +3 Spectrums, their Spanish variants,\n"
        " the Timex Sinclair 2068 and the ZX Spectrum Next. It loads tapes, snapshots and\n"
        " disks, and can turn a machine into a program of its own.\n"
    ) % (version, arch, MAINTAINER, installed_kib, major, minor, HOMEPAGE, SUMMARY)
    control_tar = gz(tar_bytes([("./", None, 0o755),
                                ("./control", control.encode(), 0o644),
                                ("./md5sums", md5s.encode(), 0o644)]))
    data_tar = gz(tar_bytes(data_entries))
    deb = ar_archive([("debian-binary", b"2.0\n"),
                      ("control.tar.gz", control_tar),
                      ("data.tar.gz", data_tar)])
    os.makedirs(out_dir, exist_ok=True)
    path = os.path.join(out_dir, "zenzx_%s_%s.deb" % (version, arch))
    with open(path, "wb") as f:
        f.write(deb)
    return path


def parents(name):
    parts = name.split("/")[:-1]
    return ["/".join(parts[:i + 1]) for i in range(len(parts))]


# ------------------------------------------------------------------------- macOS app, .dmg

def build_app(program, version, out_dir):
    check_version(version)
    app = os.path.join(out_dir, "ZenZX.app")
    if os.path.exists(app):
        shutil.rmtree(app)
    macos = os.path.join(app, "Contents", "MacOS")
    res = os.path.join(app, "Contents", "Resources")
    os.makedirs(macos)
    os.makedirs(res)
    exe = os.path.join(macos, "zenzx")
    with open(exe, "wb") as f:
        f.write(program)
    os.chmod(exe, 0o755)
    with open(os.path.join(res, "zenzx.icns"), "wb") as f:
        f.write(read_icon("zenzx.icns"))
    plist = {
        "CFBundleName": TITLE,
        "CFBundleDisplayName": TITLE,
        "CFBundleIdentifier": BUNDLE_ID,
        "CFBundleVersion": version,
        "CFBundleShortVersionString": version,
        "CFBundleExecutable": "zenzx",
        "CFBundleIconFile": "zenzx",
        "CFBundlePackageType": "APPL",
        "CFBundleInfoDictionaryVersion": "6.0",
        "LSMinimumSystemVersion": MIN_MACOS,
        "LSApplicationCategoryType": "public.app-category.utilities",
        "NSHighResolutionCapable": True,
    }
    with open(os.path.join(app, "Contents", "Info.plist"), "wb") as f:
        plistlib.dump(plist, f)
    with open(os.path.join(app, "Contents", "PkgInfo"), "wb") as f:
        f.write(b"APPL????")
    return app


def build_dmg(app, out):
    """A compressed disk image: ZenZX.app and a link to /Applications. Needs macOS (hdiutil)."""
    if sys.platform != "darwin":
        raise PackageError("a .dmg is made with hdiutil, which exists only on macOS")
    stage = out + ".stage"
    if os.path.exists(stage):
        shutil.rmtree(stage)
    os.makedirs(stage)
    shutil.copytree(app, os.path.join(stage, "ZenZX.app"), symlinks=True)
    os.symlink("/Applications", os.path.join(stage, "Applications"))
    if os.path.exists(out):
        os.remove(out)
    subprocess.run(["hdiutil", "create", "-volname", TITLE, "-srcfolder", stage,
                    "-format", "UDZO", "-ov", out], check=True)
    shutil.rmtree(stage)
    return out


# -------------------------------------------------------------------------------- Windows

WXS = """<?xml version="1.0" encoding="UTF-8"?>
<!-- Made by tools/package.py: WiX 3.14 source for the ZenZX installer. Built with
     candle -arch {arch} and light. It installs for the person who runs it, so it needs no
     administrator, and puts ZenZX in the Start menu. -->
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" Name="ZenZX" Language="1033" Version="{version}"
           Manufacturer="ZenZX" UpgradeCode="{upgrade}">
    <Package InstallerVersion="500" Compressed="yes" InstallScope="perUser" Platform="{arch}"
             Description="ZenZX, a ZX Spectrum emulator" Manufacturer="ZenZX"/>
    <MajorUpgrade DowngradeErrorMessage="A newer version of ZenZX is already installed."
                  AllowSameVersionUpgrades="yes"/>
    <MediaTemplate EmbedCab="yes"/>

    <Icon Id="ZenZXIcon" SourceFile="zenzx.ico"/>
    <Property Id="ARPPRODUCTICON" Value="ZenZXIcon"/>
    <Property Id="ARPHELPLINK" Value="{home}"/>

    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="LocalAppDataFolder">
        <Directory Id="ProgramsFolder" Name="Programs">
          <Directory Id="INSTALLFOLDER" Name="ZenZX"/>
        </Directory>
      </Directory>
      <Directory Id="ProgramMenuFolder"/>
    </Directory>

    <DirectoryRef Id="INSTALLFOLDER">
      <Component Id="ProgramFile" Guid="*" Win64="{win64}">
        <File Id="ZenZXExe" Name="zenzx.exe" Source="zenzx.exe" KeyPath="yes"/>
        <RemoveFolder Id="RemoveInstallFolder" On="uninstall"/>
      </Component>
    </DirectoryRef>

    <DirectoryRef Id="ProgramMenuFolder">
      <Component Id="StartMenuShortcut" Guid="*" Win64="{win64}">
        <Shortcut Id="ZenZXShortcut" Name="ZenZX" Description="ZX Spectrum emulator"
                  Target="[INSTALLFOLDER]zenzx.exe" WorkingDirectory="INSTALLFOLDER"
                  Icon="ZenZXIcon"/>
        <RegistryValue Root="HKCU" Key="Software\\ZenZX" Name="installed" Type="integer"
                       Value="1" KeyPath="yes"/>
      </Component>
    </DirectoryRef>

    <Feature Id="Everything" Title="ZenZX" Level="1">
      <ComponentRef Id="ProgramFile"/>
      <ComponentRef Id="StartMenuShortcut"/>
    </Feature>
  </Product>
</Wix>
"""


def build_msi_source(program, version, arch, out_dir):
    check_version(version)
    if arch not in ("x64", "arm64"):
        raise PackageError("arch must be x64 or arm64")
    os.makedirs(out_dir, exist_ok=True)
    with open(os.path.join(out_dir, "zenzx.exe"), "wb") as f:
        f.write(program)
    with open(os.path.join(out_dir, "zenzx.ico"), "wb") as f:
        f.write(read_icon("zenzx.ico"))
    wxs = WXS.format(arch=arch, version=version, upgrade=UPGRADE_CODE,
                     home=escape(HOMEPAGE), win64="yes")
    path = os.path.join(out_dir, "zenzx.wxs")
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.write(wxs)
    return path


# ------------------------------------------------------------------------------------ main

def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    sub = ap.add_subparsers(dest="cmd", required=True)

    def source(p):
        p.add_argument("--program")
        p.add_argument("--archive")
        p.add_argument("--version", required=True)

    p = sub.add_parser("deb"); source(p)
    p.add_argument("--arch", required=True, choices=["amd64", "arm64"])
    p.add_argument("--out", required=True)
    p = sub.add_parser("app"); source(p)
    p.add_argument("--out", required=True)
    p = sub.add_parser("dmg")
    p.add_argument("--app", required=True)
    p.add_argument("--out", required=True)
    p = sub.add_parser("msi-source"); source(p)
    p.add_argument("--arch", required=True, choices=["x64", "arm64"])
    p.add_argument("--out", required=True)
    args = ap.parse_args(argv)
    try:
        if args.cmd == "deb":
            print(build_deb(program_bytes(args, "linux"), args.version, args.arch, args.out))
        elif args.cmd == "app":
            print(build_app(program_bytes(args, "darwin"), args.version, args.out))
        elif args.cmd == "dmg":
            print(build_dmg(args.app, args.out))
        else:
            print(build_msi_source(program_bytes(args, "windows"), args.version, args.arch, args.out))
    except (PackageError, build_site.BuildError, OSError, subprocess.CalledProcessError) as e:
        print("package: %s" % e, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
