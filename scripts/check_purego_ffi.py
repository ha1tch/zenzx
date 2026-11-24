#!/usr/bin/env python3
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
"""check_purego_ffi.py -- do raylib-go's cgo-free calls pass the arguments they were prepared for?

    python3 scripts/check_purego_ffi.py            # the raylib-go version go.mod selects
    python3 scripts/check_purego_ffi.py FILE...    # a raylib_purego.go, or a copy of one

On macOS and Windows the GUI reaches raylib through raylib-go's purego backend
(raylib_purego.go), which prepares each C function once, with its result type and the
types of its arguments, and calls it later with the values. The library checks at run
time that the two have the same number of arguments, and panics at the call if they do
not ("ffi: calling with too many arguments"). Nothing checks it when the program is
built, so a function prepared wrongly builds, passes every test on Linux (which does not
use the backend), and stops the program at the first call on a Mac or a Windows machine.
raylib-go v0.60.0 had two (SetWindowSize and DrawCircleLinesV); the first ended every
start of the GUI on macOS.

This reads the file and compares, for every prepared function, the number of argument
types it was prepared with against every call made through it. It does not know the
types themselves, only the count; it is a guard against that mistake, not a proof that
the backend is right.

Exit status: 0 no mismatch, 1 a mismatch, 2 the file could not be found or read.
"""

import os
import re
import subprocess
import sys

PREP_RE = re.compile(r'(\w+)\s*=\s*\w+\.(?:Must)?Prep\("(\w+)",\s*([^\n]*)\)\s*$', re.M)
CALL_RE = re.compile(r"\b(\w+)\.Call\(")


def split_args(s):
    """s split at the commas that are not inside brackets."""
    out, depth, cur = [], 0, ""
    for ch in s:
        if ch in "([{":
            depth += 1
        elif ch in ")]}":
            depth -= 1
        if ch == "," and depth == 0:
            out.append(cur.strip())
            cur = ""
        else:
            cur += ch
    if cur.strip():
        out.append(cur.strip())
    return out


def audit(src):
    """(number of prepared functions, [(line, C function, prepared with, called with)])."""
    preps = {}
    for m in PREP_RE.finditer(src):
        var, cname, rest = m.group(1), m.group(2), m.group(3)
        # The first type is the result's; the rest are the arguments'.
        preps[var] = (cname, max(len(split_args(rest)) - 1, 0))
    bad = []
    for m in CALL_RE.finditer(src):
        var = m.group(1)
        if var not in preps:
            continue
        i, depth = m.end(), 1
        while depth and i < len(src):
            if src[i] == "(":
                depth += 1
            elif src[i] == ")":
                depth -= 1
            i += 1
        # The first value is where the result goes (nil for none); the rest are the arguments.
        called = max(len(split_args(src[m.end():i - 1])) - 1, 0)
        cname, prepared = preps[var]
        if called != prepared:
            bad.append((src.count("\n", 0, m.start()) + 1, cname, prepared, called))
    return len(preps), bad


def selected_file():
    """raylib_purego.go of the raylib-go version this module selects, or None."""
    here = os.path.dirname(os.path.abspath(__file__))
    mod = "github.com/gen2brain/raylib-go/raylib"
    root = os.path.join(here, "..")

    def listed():
        r = subprocess.run(["go", "list", "-m", "-f", "{{.Dir}}", mod], cwd=root, capture_output=True, text=True)
        return r.stdout.strip() if r.returncode == 0 else ""

    d = listed()
    if not d:
        # Not in the module cache: the cgo-free core build (headless) never needs raylib-go, so on
        # a fresh machine (CI) nothing has fetched it. Fetch just this module, then ask again.
        subprocess.run(["go", "mod", "download", mod], cwd=root, capture_output=True, text=True)
        d = listed()
    if not d:
        return None
    return os.path.join(d, "raylib_purego.go")


def main(argv):
    paths = argv or [selected_file()]
    status = 0
    for p in paths:
        if not p or not os.path.isfile(p):
            print("check_purego_ffi: no raylib_purego.go to read (%s); is the module downloaded? go mod download" % p,
                  file=sys.stderr)
            return 2
        with open(p, encoding="utf-8") as f:
            n, bad = audit(f.read())
        if n == 0:
            print("check_purego_ffi: %s has no prepared functions; the library's layout has changed" % p, file=sys.stderr)
            return 2
        print("%s: %d prepared functions, %d called with the wrong number of arguments" % (p, n, len(bad)))
        for line, cname, prepared, called in bad:
            print("  line %d  %s: prepared with %d arguments, called with %d" % (line, cname, prepared, called))
        if bad:
            status = 1
    return status


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
