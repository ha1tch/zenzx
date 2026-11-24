#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Build the ZenZX download page: the GUI program for every platform as a raw
executable, with its checksums and a manifest a ZenZX client can read.

    build_site.py --version 0.50.0 --archives dist --out zenzx_site

--archives is the folder the release workflow's GUI jobs leave their archives
in (zenzx-<version>-<os>-<arch>.tar.gz, and .zip for Windows). The headless
archives are not used. The page is:

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


def human(n):
    return f"{n / 1048576:.1f} MiB"


PAGE = """<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ZenZX {version}</title>
<style>
:root {{ --bg: #ffffff; --fg: #1a1a1a; --dim: #5c5c5c; --line: #d6d6d6; --link: #0b57d0; }}
@media (prefers-color-scheme: dark) {{
  :root {{ --bg: #1a1a1a; --fg: #e8e8e8; --dim: #a0a0a0; --line: #3a3a3a; --link: #8ab4f8; }}
}}
body {{ background: var(--bg); color: var(--fg); font: 16px/1.5 system-ui, sans-serif;
        margin: 0 auto; max-width: 52rem; padding: 2rem 16px; }}
h1, h2 {{ line-height: 1.2; }}
a {{ color: var(--link); }}
table {{ border-collapse: collapse; width: 100%; }}
th, td {{ text-align: left; padding: .4rem .6rem; border-bottom: 1px solid var(--line); }}
td.n {{ white-space: nowrap; }}
code {{ font-family: ui-monospace, monospace; font-size: .9em; }}
.dim {{ color: var(--dim); }}
.sum {{ font-size: .8em; word-break: break-all; }}
@media (max-width: 40rem) {{
  .sum {{ display: none; }}
  td.n {{ white-space: normal; overflow-wrap: anywhere; }}
  th, td {{ padding: .4rem .3rem; }}
}}
</style>
</head>
<body>
<h1>ZenZX {version}</h1>
<p>A ZX Spectrum emulator: 48K, 128K, +2, +2A, +3, the Spanish variants, the Timex Sinclair 2068
and the ZX Spectrum Next. One program, with its ROMs inside it.</p>

<h2>Programs</h2>
<table>
<tr><th>System</th><th>Arch</th><th>Download</th><th>Size</th><th class="sum">SHA-256</th></tr>
{rows}
</table>
<p>Check a download with <code>sha256sum -c checksums.txt</code> (<code>shasum -a 256 -c</code> on a Mac),
in the folder that has both. <a href="bin/checksums.txt">checksums.txt</a></p>
<p class="dim">On Linux and macOS, <code>chmod +x</code> the file. A browser marks a program it downloads,
and macOS then asks for permission to open it: <code>xattr -d com.apple.quarantine zenzx-darwin-arm64</code>
clears that.</p>

<h2>For a program</h2>
<p><a href="runners.json">runners.json</a> lists the same programs for a ZenZX client, with the
size and SHA-256 of each; <code>file</code> is relative to the manifest's own address.</p>

<h2>More</h2>
<ul>
<li><a href="https://github.com/{repo}/releases/tag/v{version}">Release {version}</a>:
the same programs in archives, with the ROMs, the manual and the Next demo card beside them,
and the programs without a window.</li>
<li><a href="https://github.com/{repo}">Source</a>, Apache License 2.0.</li>
</ul>
</body>
</html>
"""


def build(version, archives, out, repo=REPO):
    found = find_archives(archives, version)

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

    rows = "\n".join(
        '<tr><td>{os}</td><td>{arch}</td><td class="n"><a href="{file}">{name}</a></td>'
        '<td class="n">{size}</td><td class="sum"><code>{sha}</code></td></tr>'.format(
            os=html.escape(r["os"]), arch=html.escape(r["arch"]), file=html.escape(r["file"]),
            name=html.escape(os.path.basename(r["file"])), size=human(r["size"]), sha=r["sha256"])
        for r in runners
    )
    with open(os.path.join(out, "index.html"), "w", encoding="utf-8", newline="\n") as f:
        f.write(PAGE.format(version=html.escape(version), repo=html.escape(repo), rows=rows))
    return manifest


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--version", required=True, help="the version, without the leading v")
    ap.add_argument("--archives", required=True, help="the folder of the release's GUI archives")
    ap.add_argument("--out", required=True, help="the folder to build the page in (replaced)")
    ap.add_argument("--repo", default=REPO, help="owner/name on GitHub, for the page's links")
    args = ap.parse_args(argv)
    try:
        m = build(args.version, args.archives, args.out, args.repo)
    except BuildError as e:
        print(f"build_site: {e}", file=sys.stderr)
        return 1
    total = sum(r["size"] for r in m["runners"])
    print(f"zenzx {m['version']} site built: {len(m['runners'])} programs, {human(total)} -> {args.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
