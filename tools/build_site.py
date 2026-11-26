#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Build the ZenZX download page: the GUI program for every platform as a raw
executable, with its checksums and a manifest a ZenZX client can read.

    build_site.py --version 0.50.0 --archives dist --out zenzx_site [--extras extras]

--archives is the folder the release workflow's GUI jobs leave their archives
in (zenzx-<version>-<os>-<arch>.tar.gz, and .zip for Windows). The headless
archives are not used. --extras is optional: a folder with what the release
carries beside those programs, the installers (.deb, .msi, .dmg) and the
Windows 7 program (.zip, .msi). Whatever of them is there is listed on the page,
with a link to the file on the release, its size and its SHA-256; whatever is
not there is left out, and the build does not stop. They are not in runners.json,
which is for the programs a client runs. The page is:

    index.html          for a person
    runners.json        for a ZenZX client: every program, its size and sha256
    bin/zenzx-<os>-<arch>[.exe]   the programs
    bin/checksums.txt   sha256sum's format, for `sha256sum -c`

The page is built from the same archives the release publishes, so a program on
the page is the program on the release, byte for byte. It is all or nothing:
a platform missing, or an executable that is not one for its platform, stops
the build, and nothing is deployed.

Python's standard library only.
"""
import argparse
import hashlib
import html
import json
import os
import re
import shutil
import sys
import tarfile
import zipfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import site_page  # noqa: E402  (the page itself, beside this file)

REPO = "ha1tch/zenzx"
SCHEMA = 1

# The programs the page has: GUI builds, one per platform.
PLATFORMS = [
    ("linux", "amd64"),
    ("linux", "arm64"),
    ("darwin", "amd64"),
    ("darwin", "arm64"),
    ("windows", "amd64"),
    ("windows", "arm64"),
]

# <program>-<version>-<os>-<arch>.<ext>; the headless archives, which end in
# -headless, do not match.
ARCHIVE = re.compile(
    r"^zenzx-(?P<version>.+)-(?P<os>linux|darwin|windows)-(?P<arch>amd64|arm64)\.(?P<ext>tar\.gz|zip)$"
)

# What the first bytes of an executable are, by operating system.
MAGIC = {
    "linux": [b"\x7fELF"],
    "windows": [b"MZ"],
    "darwin": [
        b"\xfe\xed\xfa\xce", b"\xfe\xed\xfa\xcf",  # Mach-O, big-endian
        b"\xce\xfa\xed\xfe", b"\xcf\xfa\xed\xfe",  # Mach-O, little-endian
        b"\xca\xfe\xba\xbe",                       # universal
    ],
}


class BuildError(Exception):
    pass


def exe_name(os_, arch):
    return f"zenzx-{os_}-{arch}" + (".exe" if os_ == "windows" else "")


def find_archives(folder, version):
    """The archives in folder for version, as {(os, arch): path}."""
    found = {}
    for name in sorted(os.listdir(folder)):
        m = ARCHIVE.match(name)
        if not m or m["version"] != version:
            continue
        key = (m["os"], m["arch"])
        if key in found:
            raise BuildError(f"two archives for {key[0]}/{key[1]}: {found[key]} and {name}")
        found[key] = os.path.join(folder, name)
    missing = [f"{o}/{a}" for o, a in PLATFORMS if (o, a) not in found]
    if missing:
        raise BuildError(
            f"no archive for version {version} for: {', '.join(missing)} (in {folder})"
        )
    return found


def read_program(path, os_):
    """The program's bytes from the archive at path: the file named zenzx, or
    zenzx.exe, in it."""
    want = "zenzx.exe" if os_ == "windows" else "zenzx"
    hits = []
    if path.endswith(".zip"):
        with zipfile.ZipFile(path) as z:
            for info in z.infolist():
                if not info.is_dir() and os.path.basename(info.filename) == want:
                    hits.append(z.read(info))
    else:
        with tarfile.open(path, "r:gz") as t:
            for member in t.getmembers():
                if member.isfile() and os.path.basename(member.name) == want:
                    hits.append(t.extractfile(member).read())
    if len(hits) != 1:
        raise BuildError(f"{os.path.basename(path)} has {len(hits)} files named {want}, want 1")
    return hits[0]


def check_program(data, os_, arch, name):
    if len(data) < 1_000_000:
        raise BuildError(f"{name}: {len(data)} bytes is too small for the program")
    if not any(data.startswith(m) for m in MAGIC[os_]):
        raise BuildError(f"{name}: does not begin like an executable for {os_}: {data[:4]!r}")


# What a release carries beside the programs: the installers, the Windows 7 program, the manual
# as a PDF and the examples.
# (group, system, for, file name); {v} is the version. The page lists the ones that are there.
EXTRAS = [
    ("installer", "Linux", "Debian and Ubuntu, amd64", "zenzx_{v}_amd64.deb"),
    ("installer", "Linux", "Debian and Ubuntu, arm64", "zenzx_{v}_arm64.deb"),
    ("installer", "Windows 10 and later", "amd64", "zenzx-{v}-windows-amd64.msi"),
    ("installer", "Windows 10 and later", "arm64, Windows on ARM", "zenzx-{v}-windows-arm64.msi"),
    ("installer", "macOS", "Intel and Apple silicon", "zenzx-{v}-macos-universal.dmg"),
    ("windows7", "Windows 7", "amd64, installer", "zenzx-{v}-windows7-amd64.msi"),
    ("windows7", "Windows 7", "amd64, a folder to unzip", "zenzx-{v}-windows7-amd64.zip"),
    ("documents", "Any system", "the manual, A4, with its pictures", "zenzx-{v}-manual.pdf"),
    ("documents", "Any system", "the examples, with launchers for Windows, macOS and Linux", "zenzx-{v}-examples.zip"),
]


def find_extras(folder, version):
    """The files of EXTRAS that are in folder, as rows for the page. A folder or a
    file that is not there is no error: the installers and the Windows 7 program are
    made after the release is published, and the page is worth having without them."""
    rows = []
    if not folder or not os.path.isdir(folder):
        return rows
    for group, system, detail, pattern in EXTRAS:
        name = pattern.replace("{v}", version)
        path = os.path.join(folder, name)
        if not os.path.isfile(path):
            continue
        with open(path, "rb") as f:
            data = f.read()
        if not data:
            raise BuildError(f"{name}: is empty")
        rows.append({
            "group": group, "system": system, "detail": detail, "name": name,
            "size": len(data), "sha256": hashlib.sha256(data).hexdigest(),
        })
    return rows


def human(n):
    return site_page.human(n)


# The figure that waits in a ZenZX window with no machine in it: the pictures, and what it does
# with them (sprites/wait/sequence.txt), which the page plays as the program does.
FIGURE_DIR = os.path.join(ROOT, "sprites", "wait")


def read_figure():
    """{"frames": [file names under assets/], "intro": [[frame, seconds]...], "loop": [...], "rest": n}:
    the steps before the line "loop" are played once, those after it go round. "rest" is the picture
    that is shown the longest, which is what the page shows before (or without) a script."""
    names = sorted(n for n in os.listdir(FIGURE_DIR) if re.fullmatch(r"frame\d+\.png", n))
    if not names:
        raise BuildError(f"no frame*.png in {FIGURE_DIR}")
    intro, loop, cur = [], [], None
    path = os.path.join(FIGURE_DIR, "sequence.txt")
    if os.path.isfile(path):
        cur = intro
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.split("#", 1)[0].strip()
                if not line:
                    continue
                if line == "loop":
                    cur = loop
                    continue
                try:
                    n, secs = line.split()
                    step = [int(n), float(secs)]
                except ValueError:
                    raise BuildError(f"sequence.txt: cannot read {line!r}")
                if not 0 <= step[0] < len(names):
                    raise BuildError(f"sequence.txt: there is no picture {step[0]}")
                cur.append(step)
    if not intro and not loop:
        loop = [[i, 0.2] for i in range(len(names))]
    held = {}
    for n, secs in loop or intro:
        held[n] = held.get(n, 0) + secs
    return {"frames": ["assets/figure/" + n for n in names], "intro": intro, "loop": loop,
            "rest": max(held, key=held.get)}


def copy_assets(out):
    """What the page draws itself with: the icon, the face, the figure. All of it is in the repository."""
    assets = os.path.join(out, "assets")
    os.makedirs(os.path.join(assets, "figure"))
    for src, dst in (
        (("assets", "icon", "speccy.png"), "speccy.png"),
        (("assets", "icon", "png", "zenzx-64.png"), "zenzx-64.png"),
        (("assets", "site", "spleen-5x8.woff"), "spleen-5x8.woff"),
        (("assets", "site", "spleen-5x8-proportional.woff"), "spleen-5x8-proportional.woff"),
        (("assets", "site", "LICENSE-Spleen.txt"), "LICENSE-Spleen.txt"),
    ):
        path = os.path.join(ROOT, *src)
        if not os.path.isfile(path):
            raise BuildError(f"the page needs {os.path.join(*src)}")
        shutil.copyfile(path, os.path.join(assets, dst))
    for n in sorted(os.listdir(FIGURE_DIR)):
        if re.fullmatch(r"frame\d+\.png", n):
            shutil.copyfile(os.path.join(FIGURE_DIR, n), os.path.join(assets, "figure", n))


def build(version, archives, out, repo=REPO, extras=None):
    found = find_archives(archives, version)
    extra_rows = find_extras(extras, version)

    if os.path.isdir(out):
        shutil.rmtree(out)
    bin_dir = os.path.join(out, "bin")
    os.makedirs(bin_dir)

    runners = []
    for os_, arch in PLATFORMS:
        name = exe_name(os_, arch)
        data = read_program(found[(os_, arch)], os_)
        check_program(data, os_, arch, name)
        with open(os.path.join(bin_dir, name), "wb") as f:
            f.write(data)
        runners.append({
            "os": os_,
            "arch": arch,
            "file": f"bin/{name}",
            "size": len(data),
            "sha256": hashlib.sha256(data).hexdigest(),
        })

    with open(os.path.join(bin_dir, "checksums.txt"), "w", encoding="ascii", newline="\n") as f:
        for r in runners:
            f.write(f"{r['sha256']}  {os.path.basename(r['file'])}\n")

    manifest = {"schema": SCHEMA, "program": "zenzx", "version": version, "runners": runners}
    with open(os.path.join(out, "runners.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(manifest, f, indent=2)
        f.write("\n")

    copy_assets(out)
    page = site_page.Page(version, repo, runners, extra_rows, read_figure())
    with open(os.path.join(out, "index.html"), "w", encoding="utf-8", newline="\n") as f:
        f.write(page.render())
    return manifest, extra_rows


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--version", required=True, help="the version, without the leading v")
    ap.add_argument("--archives", required=True, help="the folder of the release's GUI archives")
    ap.add_argument("--extras", default=None,
                    help="a folder with the release's installers and Windows 7 files, to list on the page (optional)")
    ap.add_argument("--out", required=True, help="the folder to build the page in (replaced)")
    ap.add_argument("--repo", default=REPO, help="owner/name on GitHub, for the page's links")
    args = ap.parse_args(argv)
    try:
        m, extra_rows = build(args.version, args.archives, args.out, args.repo, args.extras)
    except BuildError as e:
        print(f"build_site: {e}", file=sys.stderr)
        return 1
    total = sum(r["size"] for r in m["runners"])
    note = f", {len(extra_rows)} installer and Windows 7 files listed" if extra_rows else ""
    print(f"zenzx {m['version']} site built: {len(m['runners'])} programs, {human(total)}{note} -> {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
