#!/bin/sh
# Release gate: CHANGELOG.md must carry an entry for the version in VERSION.
# Run after the sync step (which writes VERSION), so the version is authoritative.
set -e
cd "$(dirname "$0")"
V=$(tr -d '[:space:]' < VERSION)
[ -n "$V" ] || { echo "VERSION is empty"; exit 1; }
if grep -q "^## \[$V\]" CHANGELOG.md; then
    echo "CHANGELOG has [$V]"
else
    echo "CHANGELOG.md has no entry for [$V] -- write it before releasing"
    exit 1
fi
