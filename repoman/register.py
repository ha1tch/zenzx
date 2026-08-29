#!/usr/bin/env python3
"""Thin shim -- forwards to the gorepoman binary's `register` subcommand.

The vendored Python implementation previously in this file has been
retired in favour of github.com/ha1tch/gorepoman (a static Go binary).
This shim exists only so every pre-existing invocation of
`python3 repoman/register.py ...` (Makefile targets, .repoman.json
release steps, README/MANUAL examples) keeps working unchanged.

Binary is located, in order:
  1. $REPOMAN_BIN environment variable
  2. `repoman` on $PATH

If neither resolves, install the binary -- see
https://github.com/ha1tch/gorepoman (mirror:
https://ha1tch.github.io/gorepoman/) -- then either export REPOMAN_BIN
to its path or put it on PATH.
"""
import os
import shutil
import subprocess
import sys

SUBCOMMAND = "register"


def find_binary():
    env = os.environ.get("REPOMAN_BIN")
    if env and os.path.isfile(env) and os.access(env, os.X_OK):
        return env
    return shutil.which("repoman")


def main():
    binary = find_binary()
    if binary is None:
        sys.stderr.write(
            "error: gorepoman binary not found for repoman/register.py shim.\n"
            "Set REPOMAN_BIN to its path, or put `repoman` on PATH.\n"
            "Install: https://github.com/ha1tch/gorepoman "
            "(mirror: https://ha1tch.github.io/gorepoman/)\n"
        )
        return 127
    return subprocess.call([binary, SUBCOMMAND] + sys.argv[1:])


if __name__ == "__main__":
    sys.exit(main())
