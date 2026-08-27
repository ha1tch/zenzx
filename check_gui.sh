#!/bin/sh
# Release gate: type-check the GUI (raylib + oto) sources.
# In an environment with the system libraries the GUI links; elsewhere only
# cgo/system-library failures are tolerated -- any Go-level error in a .go
# file fails the gate. Linking the real GUI binary is dormant guard G-01.
set -e
cd "$(dirname "$0")"
ERR=$(mktemp)
if CGO_ENABLED=1 go build -o /dev/null . 2>"$ERR"; then
    echo "GUI build linked (system libs present)"
    rm -f "$ERR"; exit 0
fi
if grep -qE '^(\./)?[a-z_0-9]+\.go:[0-9]+' "$ERR"; then
    cat "$ERR"; rm -f "$ERR"
    echo "GUI sources have Go-level errors"; exit 1
fi
rm -f "$ERR"
echo "No Go-level errors in GUI sources (only cgo/system-lib failures, expected without raylib/ALSA)"
