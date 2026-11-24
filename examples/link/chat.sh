#!/bin/sh
# Launch the chat-room example: four Spectrums in one room, tiled in a grid,
# with a host that relays each line to the others.
#
#     sh examples/link/chat.sh
#
# The clients are BASIC text (chat.bas and its variants), autoloaded on each
# machine -- no assembler needed. It launches the zenzx GUI with the right
# parameters and then the Python host. Works on Linux and macOS; no GNU-only
# tools. The emulator is not built here: it looks for zenzx where `make build`
# puts it (dist/zenzx), then at the repository root and on the PATH; set ZENZX
# to point elsewhere.
#
# Each machine shows what it is sent and sends a line of its own a moment
# after it starts; type a line at the host to send it to all of them.
set -e

here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root=$(CDPATH= cd -- "$here/../.." && pwd)

# Look for zenzx: $ZENZX, then dist/ (the Makefile's output), the repo root,
# then the PATH.
zenzx=""
for cand in "$ZENZX" "$root/dist/zenzx" "$root/zenzx"; do
    if [ -n "$cand" ] && [ -x "$cand" ]; then zenzx="$cand"; break; fi
done
[ -n "$zenzx" ] || zenzx=$(command -v zenzx 2>/dev/null) || {
    echo "chat.sh: cannot find the zenzx GUI binary." >&2
    echo "  Build it with 'make build' (it lands in dist/zenzx)," >&2
    echo "  or set ZENZX=/path/to/zenzx." >&2
    exit 1
}

ports="16384,16385,16386,16387"
clients="$here/chat.bas,$here/chat-ada.bas,$here/chat-clive.bas,$here/chat-grace.bas"

"$zenzx" -scale 3 -layout grid \
    -multi 4:48k,48k,128k,plus3 \
    -listen "$ports" \
    -autorun "$clients" &
gui=$!

# Stop the GUI when the host leaves (or we are interrupted).
trap 'kill $gui 2>/dev/null' INT TERM EXIT

# A moment for the machines to boot and load, then the host -- in the
# FOREGROUND, so it has this terminal: type a line and press Enter to send it
# to every machine as "host: ...", Ctrl-D to leave. The machines' own lines
# and each other's are printed here too, in colour.
sleep 10
echo "Host ready. Type a line and press Enter to chat; Ctrl-D to leave."
python3 "$here/chatroom.py" 16384 16385 16386 16387
