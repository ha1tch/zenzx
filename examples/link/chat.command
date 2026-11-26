#!/bin/sh
# Double-click this file in Finder to start the example: macOS opens it in a Terminal window.
# The first time, macOS may refuse because the file came from the internet: right-click it,
# choose Open, then Open again (or System Settings > Privacy & Security > Open Anyway).
cd "$(dirname "$0")" || exit 1
sh ./chat.sh "$@" || {
    echo
    printf 'It did not start. Press Return to close this window. '
    read -r _key
}
