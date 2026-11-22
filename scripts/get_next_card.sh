#!/bin/sh
# get_next_card.sh - download the official ZX Spectrum Next emulator distribution and make
# the SD card and DivMMC ROM that -model=next boots from.
#
# Usage: sh scripts/get_next_card.sh [OUTPUT_FOLDER] [VERSION]
#        make next-card
#
#   OUTPUT_FOLDER   where the files go; default ./next (whose contents git ignores)
#   VERSION         the distribution to fetch; default 26.11
#
# What it does:
#
#   1. Downloads sn-emulator-VERSION.zip (about 70 MB) from specnext.com into the output
#      folder, unless it is already there. For a version this script knows, the file's
#      SHA-256 is checked.
#   2. Runs scripts/next_card.py on it, which writes next-card.img (1 GB), enNxtmmc.rom and
#      enNextZX.rom into the output folder. next_card.py's own header explains the four
#      changes it makes to the card and why.
#   3. Says what is still missing. The boot ROM (nextboot.rom) is not part of the
#      distribution and is not downloaded: put your copy in the output folder.
#
# An existing next-card.img is not overwritten unless FORCE=1 is set, because it may hold
# files saved from the emulator.
#
# Needs: curl or wget, python3 with pyfatfs, and about 1.2 GB of disk.
#   Linux:  sudo apt install curl python3 python3-pip && pip install pyfatfs
#   macOS:  brew install python && pip3 install pyfatfs      (curl is already there)
#
# Copyright (c) 2026 haitch - h@ual.li - https://oldbytes.space/@haitchfive
# Licensed under the Apache License, Version 2.0: https://www.apache.org/licenses/LICENSE-2.0

cd "$(dirname "$0")/.." || exit 1

OUT=${1:-./next}
VERSION=${2:-26.11}
ZIP="$OUT/sn-emulator-$VERSION.zip"
URL="https://www.specnext.com/distro/$VERSION/sn-emulator-$VERSION.zip"
CARD="$OUT/next-card.img"

fail() {
    printf '\033[31mError\033[0m: %s\n' "$1" >&2
    exit 1
}

# The SHA-256 of the distributions this script was tested with.
known_sum() {
    case "$1" in
        26.11) echo 81fe97d37ef4e373643b4d399c22ab2acca631c02a778e8d01e136e0c7c5d2d6 ;;
        *) echo "" ;;
    esac
}

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | cut -d' ' -f1
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | cut -d' ' -f1
    else
        echo ""
    fi
}

command -v python3 >/dev/null 2>&1 || fail "python3 is not installed"
python3 -c 'import pyfatfs' >/dev/null 2>&1 || fail "the Python package pyfatfs is not installed: pip install pyfatfs"

mkdir -p "$OUT" || fail "cannot create $OUT"

if [ -f "$CARD" ] && [ "${FORCE:-0}" != "1" ]; then
    echo "$CARD is already there and was left alone (it may hold files saved from the emulator)."
    echo "To make it again: FORCE=1 sh scripts/get_next_card.sh $OUT $VERSION"
else
    if [ -f "$ZIP" ]; then
        echo "Using $ZIP, already downloaded."
    else
        echo "Downloading $URL (about 70 MB)..."
        if command -v curl >/dev/null 2>&1; then
            curl -fL --retry 3 -o "$ZIP.part" "$URL" || { rm -f "$ZIP.part"; fail "the download failed"; }
        elif command -v wget >/dev/null 2>&1; then
            wget -O "$ZIP.part" "$URL" || { rm -f "$ZIP.part"; fail "the download failed"; }
        else
            fail "neither curl nor wget is installed"
        fi
        mv "$ZIP.part" "$ZIP" || fail "cannot write $ZIP"
    fi

    want=$(known_sum "$VERSION")
    got=$(sha256_of "$ZIP")
    if [ -z "$want" ]; then
        echo "No checksum is recorded for version $VERSION; the file was not checked."
    elif [ -z "$got" ]; then
        echo "Neither sha256sum nor shasum is installed; the file was not checked."
    elif [ "$want" != "$got" ]; then
        fail "$ZIP has SHA-256 $got, expected $want. Delete it and run this again."
    else
        echo "Checksum of $ZIP is as expected."
    fi

    python3 scripts/next_card.py "$ZIP" "$OUT" || fail "scripts/next_card.py failed"
fi

echo
missing=0
for f in next-card.img enNxtmmc.rom nextboot.rom; do
    if [ -f "$OUT/$f" ]; then
        echo "  present  $OUT/$f"
    else
        echo "  missing  $OUT/$f"
        missing=$((missing + 1))
    fi
done
echo
if [ ! -f "$OUT/nextboot.rom" ]; then
    echo "nextboot.rom, the 8K boot ROM, is not in the distribution. Copy yours to $OUT/nextboot.rom."
fi
if [ "$missing" -eq 0 ]; then
    echo "Everything is there. Start the Next with: sh run_next_divmmc.sh"
fi
