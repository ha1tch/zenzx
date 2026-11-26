# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0
#
# findpy.sh -- finds the Python that runs the examples' helper programs, or gets one.
# It is sourced by a launcher, which sets examples_dir (the folder that holds launch.py) first:
#
#     examples_dir=$(CDPATH= cd -- "$here/.." && pwd)
#     . "$examples_dir/tools/findpy.sh"
#     [ -n "$PY" ] || exit 1
#
# PY is then a python that runs 3.6 or later, or empty after the person has been told, in plain
# words, what Python is for, and how to get it. Looked for, in order: the private copy or
# environment in examples/python, python3 and python on the PATH, and in the folders of
# ZENZX_PYTHON_DIRS (default: where Homebrew puts them, /opt/homebrew/bin and /usr/local/bin;
# empty means none, for the tests).
# A command counts only when it runs. On macOS the python3 that Apple ships is a stand-in that
# opens a dialog offering to install developer tools: it is not run unless those tools are there.
#
# When there is none, and someone is at the keyboard, it says what Python is, and runs
# "zenzx -fetch python", which asks how to get one and does it: a private copy that ZenZX
# downloads itself (about 35 MB, checked against a SHA-256; nothing is installed, and deleting
# the folder "python" removes it), Homebrew where there is brew, a separate environment where
# there is uv or conda, or by hand from python.org. The download is ZenZX's own (pkg/fetch), so
# it needs no curl or wget. ZENZX_EXAMPLES_YES=1 takes the first way, the private copy, without
# asking. ZENZX_PYTHON_URL and ZENZX_PYTHON_SHA256 replace the download and its checksum, for
# the tests. ZenZX is looked for where launch.py looks (ZENZX, bin or dist beside the examples
# folder, the PATH); the headless build, which needs no graphics library, comes first.
#
# Plain POSIX sh: it works with the sh of macOS and of Linux, and with no GNU-only tool.

PY_PAGE="https://www.python.org/downloads/"

PY=""

py_works() {
    [ -n "$1" ] && [ -x "$1" ] || return 1
    "$1" -c 'import sys; sys.exit(0 if sys.version_info >= (3, 6) else 1)' >/dev/null 2>&1
}

# py_find: set PY to the first python that works; return 1 when none does.
py_find() {
    PY=""
    # Where Homebrew puts python3, which a launcher started from a double-click may not have on its PATH.
    py_extra=""
    for d in ${ZENZX_PYTHON_DIRS-/opt/homebrew/bin /usr/local/bin}; do py_extra="$py_extra $d/python3"; done
    for cand in "$examples_dir/python/bin/python3" "$examples_dir/python/bin/python" "$(command -v python3 2>/dev/null)" \
        "$(command -v python 2>/dev/null)" $py_extra; do
        [ -n "$cand" ] || continue
        if [ "$cand" = /usr/bin/python3 ] && [ "$(uname -s)" = Darwin ] && ! xcode-select -p >/dev/null 2>&1; then
            continue
        fi
        if py_works "$cand"; then
            PY=$cand
            return 0
        fi
    done
    return 1
}

# py_zenzx_find: set zz to the ZenZX program that fetches Python; return 1 when there is none.
# The same places launch.py looks: $ZENZX, then bin, the folder itself and dist beside the
# examples folder and one folder above it, then the PATH.
py_zenzx_find() {
    zz=""
    if [ -n "${ZENZX:-}" ] && [ -x "$ZENZX" ]; then
        zz=$ZENZX
        return 0
    fi
    zroot=$(CDPATH= cd -- "$examples_dir/.." && pwd)
    zup=$(CDPATH= cd -- "$zroot/.." && pwd)
    for zname in zenzx-headless zenzx; do
        for zbase in "$zroot" "$zup"; do
            for zdir in "$zbase/bin" "$zbase" "$zbase/dist"; do
                if [ -f "$zdir/$zname" ] && [ -x "$zdir/$zname" ]; then
                    zz=$zdir/$zname
                    return 0
                fi
            done
        done
    done
    for zname in zenzx-headless zenzx; do
        zz=$(command -v "$zname" 2>/dev/null) && [ -n "$zz" ] && return 0
    done
    zz=""
    return 1
}

py_explain() {
    cat <<EOF

  These examples need a free helper program called Python.
  (The programming language, not the snake: a great many programs use it quietly.)
  It is not on this computer yet.

EOF
}

py_how_to_install() {
    cat <<EOF
  To install Python yourself instead, go to $PY_PAGE
  (macOS with Homebrew: brew install python    Debian, Ubuntu: sudo apt install python3
   Fedora, Red Hat: sudo dnf install python3)
  and run this again afterwards.
EOF
}

# py_offer: say what Python is, and get one with "zenzx -fetch python", which asks how.
py_offer() {
    py_explain
    if ! py_zenzx_find; then
        echo "  The program ZenZX, which fetches it, is not where it can be found: put it in a folder"
        echo "  called bin next to the folder of examples (https://ha1tch.github.io/zenzx/ has it)."
        echo
        py_how_to_install
        return 1
    fi
    if [ "${ZENZX_EXAMPLES_YES:-}" != 1 ] && [ ! -t 0 ]; then
        echo "  Run this again in a terminal window to be asked how, or set ZENZX_EXAMPLES_YES=1 to"
        echo "  have a private copy downloaded without being asked."
        py_how_to_install
        return 1
    fi
    # No "set --" here: this file is sourced, and the launcher's own arguments are still wanted.
    if [ "${ZENZX_EXAMPLES_YES:-}" = 1 ]; then zyes=true; else zyes=false; fi
    if "$zz" -fetch python -fetch-to "$examples_dir/python" -fetch-yes="$zyes" \
        -fetch-url="${ZENZX_PYTHON_URL:-}" -fetch-sha256="${ZENZX_PYTHON_SHA256:-}" && py_find; then
        echo "  Python is ready."
        return 0
    fi
    echo
    py_how_to_install
    return 1
}

py_find || py_offer || PY=""
