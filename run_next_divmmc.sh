#!/bin/sh
# run_next_divmmc.sh - start zenzx as a Next with the whole boot chain attached (T-36):
# boot ROM, DivMMC ROM and SD card.
#
# Usage: sh run_next_divmmc.sh [extra zenzx flags]
#        NEXT_DEBUG=1 sh run_next_divmmc.sh     with DivMMC and SD card logging (slow)
#
# It expects three files in ./next (or the folder named by NEXT_FILES):
#
#   nextboot.rom    the 8K boot ROM, the first stage. It initialises the card, loads
#                   TBBLUE.FW from it and turns itself off. Not part of the distribution.
#   enNxtmmc.rom    the DivMMC ROM, from the same distribution as the card
#   next-card.img   the SD card
#
# Make the last two with `make next-card` (or `sh scripts/get_next_card.sh`), which
# downloads the official emulator distribution (about 70 MB, from specnext.com) and runs
# scripts/next_card.py on it; that script explains what it changes on the card and why.
# The ROM and the card must come from the same distribution. The card is opened for
# writing: what NextZXOS saves stays in next-card.img.
#
# The binary is built again whenever a source file is newer than it, so that a ./zenzx
# left from an earlier version is never the one that runs.

cd "$(dirname "$0")" || exit 1

DIR=${NEXT_FILES:-./next}
[ -f "$DIR/next-card.img" ] || [ ! -f ./thirdparty/next-card.img ] || DIR=./thirdparty # where the files went before
BOOT_ROM="$DIR/nextboot.rom"
DIVMMC_ROM="$DIR/enNxtmmc.rom"
SDCARD="$DIR/next-card.img"
BIN=./zenzx

for f in "$BOOT_ROM" "$DIVMMC_ROM" "$SDCARD"; do
    if [ ! -f "$f" ]; then
        printf '\033[31mError\033[0m: missing %s (see the comments at the top of this script)\n' "$f" >&2
        exit 1
    fi
done

if [ ! -x "$BIN" ] || [ -n "$(find . -maxdepth 1 \( -name "*.go" -o -name go.mod \) -newer "$BIN" | head -n 1)" ]; then
    echo "Building $BIN..."
    sh ./build.sh "$BIN" || exit 1
fi
if [ ! -w "$SDCARD" ]; then
    echo "Note: $SDCARD is not writable, so the card will be write-protected and nothing can be saved."
fi

# Logging of the DivMMC and of every card command is off unless NEXT_DEBUG=1: it writes a
# line for each byte exchanged with the card, which slows the machine down a great deal.
DEBUG_FLAG=
if [ "${NEXT_DEBUG:-0}" = "1" ]; then
    DEBUG_FLAG=-divmmc-debug
fi

echo "Starting zenzx with the Next boot chain:"
echo "  -bootrom=$BOOT_ROM"
echo "  -divmmc-rom=$DIVMMC_ROM"
echo "  -sdcard=$SDCARD"
if [ -n "$DEBUG_FLAG" ]; then
    echo "  $DEBUG_FLAG (NEXT_DEBUG=1)"
fi
exec "$BIN" -model=next -bootrom="$BOOT_ROM" -divmmc-rom="$DIVMMC_ROM" -sdcard="$SDCARD" $DEBUG_FLAG "$@"
