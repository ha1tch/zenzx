#!/bin/sh
# Launch the connected-world example across nine Spectrums in a 3x3 grid.
#
#     sh examples/world/run.sh
#
# It launches the zenzx GUI with the parameters this example needs -- nine
# 48K machines tiled in a grid, each with a link and each auto-loading the
# game tape -- and then starts the host that ties them into one world.
#
# The binaries are not built here. It needs the zenzx GUI and the zenas
# assembler; set ZENZX and ZENAS to point at them, or leave them and the
# script looks, in order, at: the environment variable, the repository root
# (for zenzx) / a sibling zenas checkout, then the PATH. Works on Linux and
# macOS; no GNU-only tools.
#
# Controls once it is up: click a machine, or press Alt+1..9, to give it the
# keyboard; walk with Q/A/O/P. Walk off an edge and the character crosses to
# the neighbouring screen; the balls drift and bounce between screens.
set -e

here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root=$(CDPATH= cd -- "$here/../.." && pwd)
examples_dir=$(CDPATH= cd -- "$here/.." && pwd)

# Python runs the host: find it, or offer to fetch it, before a window opens.
. "$examples_dir/tools/findpy.sh"
[ -n "$PY" ] || exit 1

# --- locate the binaries ----------------------------------------------------
# find_bin NAME [candidate ...]: the first candidate path that is executable,
# else NAME on the PATH. Prints the path and returns 0, or returns 1.
find_bin() {
    name=$1
    shift
    for cand in "$@"; do
        if [ -n "$cand" ] && [ -x "$cand" ]; then printf '%s\n' "$cand"; return 0; fi
    done
    command -v "$name" 2>/dev/null && return 0
    return 1
}

# zenzx: $ZENZX, then dist/ (where `make build` puts it), the repo root.
zenzx=$(find_bin zenzx "$ZENZX" "$root/dist/zenzx" "$root/zenzx") || {
    # none here: launch.py offers to download it
    exec "$PY" "$examples_dir/launch.py" world "$@"
}
# zenas is optional: a prebuilt world.tap ships with the example, so it runs
# without an assembler. zenas is only needed to rebuild the tape. Look in a
# sibling zenas checkout's dist/ and root too.
zenas=$(find_bin zenas "$ZENAS" "$root/../zenas/dist/zenas" "$root/../zenas/zenas") || zenas=""

tap="$here/world.tap"

# --- (re)assemble the tape only when needed, and only if zenas is present ----
need_build=no
if [ ! -f "$tap" ]; then
    need_build=yes
else
    for src in "$here/world.asm" "$here/assets.asm" "$here/rooms.asm"; do
        if [ "$src" -nt "$tap" ]; then need_build=yes; break; fi
    done
fi
if [ "$need_build" = yes ]; then
    if [ -n "$zenas" ]; then
        echo "Assembling world.asm -> world.tap"
        "$zenas" build "$here/world.asm" --tap --loader --start Main -o "$here/world"
    elif [ -f "$tap" ]; then
        echo "note: zenas not found; the sources are newer than world.tap," >&2
        echo "      running the prebuilt tape as shipped." >&2
    else
        echo "run.sh: no world.tap and no zenas to build one." >&2
        echo "  Put zenas on the PATH or set ZENAS=/path/to/zenas." >&2
        exit 1
    fi
fi



# --- the nine link ports, one per machine, read row by row ------------------
ports=16384,16385,16386,16387,16388,16389,16390,16391,16392
taps=$(printf '%s,' "$tap" "$tap" "$tap" "$tap" "$tap" "$tap" "$tap" "$tap" "$tap")
taps=${taps%,}

# --- launch the GUI with the parameters this example needs ------------------
"$zenzx" -scale 2 -layout grid -focus-follow \
    -multi 9:48k \
    -listen "$ports" \
    -autorun "$taps" &
gui=$!

# --- a moment to boot and load, then the host that routes the crossings -----
sleep 10
"$PY" "$here/host.py" --rows 3 --cols 3 \
    16384 16385 16386 16387 16388 16389 16390 16391 16392 &
host=$!

trap 'kill $host $gui 2>/dev/null' INT TERM
wait $gui
kill $host 2>/dev/null || true
