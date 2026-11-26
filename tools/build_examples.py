#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""Pack the examples into the archive a release carries beside its programs.

    build_examples.py --version 0.50.0 --out dist [--repo .]

writes dist/zenzx-0.50.0-examples.zip, which unpacks to one folder, zenzx-0.50.0-examples:

    examples/        what examples/ holds (world, link, next, launch.py, tools), without what
                     building them leaves behind (build/, build.log, .gitignore, caches)
    docs/img/        the pictures the examples' READMEs show, at the paths they name
    LICENSE NOTICE

The archive is the same bytes for the same tree: the files are in name order with a fixed
date, and the scripts (what starts with #!) are marked executable. It stops, and writes
nothing, when something the examples need is missing (a launcher, the prebuilt tape, a
.nex, a picture a README shows).

Python's standard library only.
"""
import argparse
import os
import re
import sys
import zipfile

SKIP_DIRS = {"build", "__pycache__", "python", "bin"}
SKIP_FILES = {"build.log", ".gitignore", ".DS_Store"}
SKIP_SUFFIXES = (".pyc", ".pyo", ".part")

DATE = (2026, 1, 1, 0, 0, 0)
# What the examples cannot be run without.
REQUIRED = [
    "examples/README.md",
    "examples/launch.py",
    "examples/tools/findpy.cmd",
    "examples/tools/findpy.sh",
    "examples/world/run.sh",
    "examples/world/run.cmd",
    "examples/world/run.command",
    "examples/world/host.py",
    "examples/world/world.tap",
    "examples/link/chat.sh",
    "examples/link/chat.cmd",
    "examples/link/chat.command",
    "examples/link/chatroom.py",
    "examples/next/run.cmd",
    "examples/next/run.command",
    "examples/next/run.sh",
    "examples/next/gallop/gallop.nex",
    "examples/next/fuji/fuji.nex",
    "examples/next/orbit/orbit.nex",
]
IMG_RE = re.compile(r"\(((?:\.\./)+)(docs/img/[^)\s]+)\)")


class BuildError(Exception):
    pass


def collect(repo):
    """[(archive path, file path)] for everything the archive holds, in name order."""
    out = {}
    base = os.path.join(repo, "examples")
    if not os.path.isdir(base):
        raise BuildError("no examples folder in " + repo)
    for dirpath, dirs, files in os.walk(base):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS)
        for name in sorted(files):
            if name in SKIP_FILES or name.endswith(SKIP_SUFFIXES):
                continue
            full = os.path.join(dirpath, name)
            rel = os.path.relpath(full, repo).replace(os.sep, "/")
            if os.path.getsize(full) == 0:
                raise BuildError(rel + ": is empty")
            out[rel] = full
    for rel in REQUIRED:
        if rel not in out:
            raise BuildError(rel + ": is missing")
    # The pictures the READMEs show, at the paths they name from where the README is.
    for rel, full in sorted(out.items()):
        if not rel.endswith(".md"):
            continue
        with open(full, encoding="utf-8") as f:
            text = f.read()
        for m in IMG_RE.finditer(text):
            target = os.path.normpath(os.path.join(os.path.dirname(rel), m.group(1) + m.group(2))).replace(os.sep, "/")
            src = os.path.join(repo, target)
            if not os.path.isfile(src):
                raise BuildError("%s shows %s, which is not in the repository" % (rel, target))
            out[target] = src
    for name in ("LICENSE", "NOTICE"):
        src = os.path.join(repo, name)
        if not os.path.isfile(src):
            raise BuildError(name + ": is missing")
        out[name] = src
    return sorted(out.items())


def build(version, repo, outdir):
    """Write the archive; its path."""
    files = collect(repo)
    top = "zenzx-%s-examples" % version
    path = os.path.join(outdir, top + ".zip")
    os.makedirs(outdir, exist_ok=True)
    tmp = path + ".part"
    try:
        with zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
            for rel, full in files:
                with open(full, "rb") as f:
                    data = f.read()
                info = zipfile.ZipInfo(top + "/" + rel, DATE)
                info.compress_type = zipfile.ZIP_DEFLATED
                # a program (it starts with #!) is marked executable; the files it reads are not
                mode = 0o755 if data.startswith(b"#!") else 0o644
                info.external_attr = (0o100000 | mode) << 16
                z.writestr(info, data)
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.remove(tmp)
    return path, len(files)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--version", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--repo", default=".")
    args = ap.parse_args()
    try:
        path, n = build(args.version, args.repo, args.out)
    except BuildError as e:
        print("build_examples.py: " + str(e), file=sys.stderr)
        return 1
    print("%s: %d files, %.1f MB" % (path, n, os.path.getsize(path) / 1e6))
    return 0


if __name__ == "__main__":
    sys.exit(main())
