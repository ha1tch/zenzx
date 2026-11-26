#!/bin/sh
# manifest.sh - write or check MANIFEST.sha256: one "sha256  path" line for every file git
# tracks, in byte order of path, MANIFEST.sha256 itself left out.
#
# Usage: sh manifest.sh          write MANIFEST.sha256 from the working tree (needs git)
#        sh manifest.sh check    exit non-zero if MANIFEST.sha256 differs from the working tree
#
# In a git checkout, check compares the manifest with the tracked files, so an added or
# removed file is caught too. In an unpacked release archive there is no git, and check
# verifies the files the manifest lists.
#
# Run it after the last edit of a release and before the commit. Works on Linux (sha256sum)
# and macOS (shasum).
#
# Copyright (c) 2026 haitch - h@ual.li - https://oldbytes.space/@haitchfive
# Licensed under the Apache License, Version 2.0: https://www.apache.org/licenses/LICENSE-2.0

cd "$(dirname "$0")" || exit 1

if command -v sha256sum > /dev/null 2>&1; then
    sum() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum > /dev/null 2>&1; then
    sum() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
    echo "manifest.sh: neither sha256sum nor shasum found" >&2
    exit 1
fi

stale() {
    printf '\033[31mMANIFEST.sha256 is stale\033[0m: %s\n' "$1"
    exit 1
}

if ! git rev-parse --is-inside-work-tree > /dev/null 2>&1; then
    if [ "$1" != check ]; then
        echo "manifest.sh: not a git checkout; only 'check' works here" >&2
        exit 1
    fi
    n=0
    while IFS= read -r line; do
        want=${line%%  *}
        f=${line#*  }
        [ -f "$f" ] || stale "$f is missing"
        [ "$(sum "$f")" = "$want" ] || stale "$f differs"
        n=$((n + 1))
    done < MANIFEST.sha256
    echo "MANIFEST.sha256 matches the $n files it lists"
    exit 0
fi

tmp=$(mktemp) || exit 1
trap 'rm -f "$tmp"' EXIT

git ls-files | grep -v '^MANIFEST\.sha256$' | LC_ALL=C sort | while IFS= read -r f; do
    [ -f "$f" ] || continue
    printf '%s  %s\n' "$(sum "$f")" "$f"
done > "$tmp"

if [ "$1" = check ]; then
    cmp -s "$tmp" MANIFEST.sha256 || stale "run sh manifest.sh"
    echo "MANIFEST.sha256 matches the working tree ($(wc -l < "$tmp" | tr -d ' ') files)"
else
    cp "$tmp" MANIFEST.sha256 && echo "MANIFEST.sha256 written ($(wc -l < "$tmp" | tr -d ' ') files)"
fi
