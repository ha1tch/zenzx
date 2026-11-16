#!/bin/sh
# run_next_divmmc.sh - start zenzx as a Next with the whole boot chain attached (T-36):
# boot ROM, DivMMC ROM and SD card.
#
# Usage: sh run_next_divmmc.sh [extra zenzx flags]
#
# It expects three files in ./thirdparty (or the folder named by NEXT_FILES):
#
#   nextboot.rom    the 8K boot ROM, the first stage. It initialises the card, loads
#                   TBBLUE.FW from it and turns itself off. Not part of the distribution.
#   enNxtmmc.rom    the DivMMC ROM, from the same distribution as the card
#   next-card.img   the SD card
#
# Make the last two from the official emulator distribution (about 70 MB, from
# https://www.specnext.com/latestdistro/):
#
#   pip install pyfatfs
#   python3 scripts/next_card.py sn-emulator-26.11.zip thirdparty/
#
# scripts/next_card.py explains what it changes on the card and why. The ROM and the card
# must come from the same distribution.

cd "$(dirname "$0")" || exit 1

DIR=${NEXT_FILES:-./thirdparty}
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

if [ ! -x "$BIN" ]; then
    echo "Building $BIN..."
    sh ./build.sh "$BIN" || exit 1
fi

echo "Starting zenzx with the Next boot chain (debug logging on):"
echo "  -bootrom=$BOOT_ROM"
echo "  -divmmc-rom=$DIVMMC_ROM"
echo "  -sdcard=$SDCARD"
exec "$BIN" -model=next -bootrom="$BOOT_ROM" -divmmc-rom="$DIVMMC_ROM" -sdcard="$SDCARD" -divmmc-debug "$@"
