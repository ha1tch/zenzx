#!/usr/bin/env bash
# release.sh - ZenZX release hygiene automation
#
# ZenZX has two build variants:
#   - GUI      (default tags): raylib + oto, needs CGO and system libraries
#                              (GL, X11/Wayland, ALSA). Built via the platform
#                              build_*.sh scripts; cannot be built in a headless
#                              CI sandbox.
#   - headless (-tags headless): CGO-free, no window, no audio device. Renders
#                              the Spectrum framebuffer to PNG. This is the
#                              variant the release gate builds and smoke-tests.
#
# Single-pass release preparation:
#   1. Validates version string and CHANGELOG entry
#   2. Syncs VERSION + version.go
#   3. Type-checks the GUI sources (without linking system libs)
#   4. Builds the headless binary (CGO-free)
#   5. Runs tests ONCE (pkg/version)
#   6. Runs a headless smoke test (boot 48K, capture a screenshot)
#   7. Verifies version strings are consistent
#   8. Cuts a checkpoint zip
#
# Usage:
#   ./release.sh <version>            e.g. ./release.sh 0.1.0
#   ./release.sh <version> --no-zip   dry run, no checkpoint
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

CUT_ZIP=true
VERSION=""
for arg in "$@"; do
    case "$arg" in
        --no-zip)  CUT_ZIP=false ;;
        --help|-h) sed -n '3,25p' "$0" | sed 's/^# \?//'; exit 0 ;;
        --*)       echo "Unknown option: $arg" >&2; exit 1 ;;
        *)         if [ -z "$VERSION" ]; then VERSION="$arg"; else echo "Unexpected argument: $arg" >&2; exit 1; fi ;;
    esac
done
[ -z "$VERSION" ] && { echo "Usage: $0 <version> [--no-zip]" >&2; exit 1; }

step() { echo ""; echo "-- $1"; }
ok()   { echo "   OK $1"; }
warn() { echo "   !! $1"; }
fail() { echo "   FAIL $1" >&2; exit 1; }

# hexhead <file> <nbytes> -- first N bytes as lowercase hex, no whitespace.
# Uses od (POSIX, always present); avoids the xxd dependency.
hexhead() {
    head -c "$2" "$1" | od -An -tx1 | tr -d ' \n'
}

# 1. Validate version
step "Version: $VERSION"
echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$' \
    || fail "Invalid version format. Expected X.Y.Z or X.Y.Z-suffix"
ok "Format valid"

GIT_VERSION="$VERSION"
if git rev-parse --git-dir > /dev/null 2>&1; then
    if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
        warn "Working tree has uncommitted changes -- checkpoint will not correspond to a clean commit"
    else
        ok "Git tree clean at $(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
    fi
    GIT_VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo "$VERSION")"
else
    warn "Not a git repository -- skipping tree check"
fi

# 2. CHANGELOG
step "Checking CHANGELOG"
if grep -q "^## \[${VERSION}\]" CHANGELOG.md; then
    ok "Entry exists: [${VERSION}]"
else
    warn "No CHANGELOG entry for [${VERSION}] -- continuing anyway"
fi

# 3. Sync version files
step "Syncing version files"
./syncver.sh set "$VERSION"
ok "VERSION and version.go = $VERSION"

# 4. Type-check GUI sources
# The GUI build links raylib + oto (CGO + system libs) and cannot complete in a
# headless sandbox. We still want to catch Go-level errors in the GUI sources.
# `go vet` triggers cgo; instead we compile and accept ONLY third-party cgo /
# system-lib failures, failing the gate if any error references a zenzx source.
step "Type-checking GUI sources"
GUI_ERR="$(mktemp)"
if CGO_ENABLED=1 go build -o /dev/null . 2>"$GUI_ERR"; then
    ok "GUI build linked (system libs present)"
else
    if grep -qE '^\./[a-z_]+\.go:[0-9]+|^[a-z_]+\.go:[0-9]+' "$GUI_ERR"; then
        cat "$GUI_ERR" >&2
        rm -f "$GUI_ERR"
        fail "GUI sources have Go-level errors"
    fi
    ok "No Go-level errors in GUI sources (only cgo/system-lib failures, expected in CI)"
fi
rm -f "$GUI_ERR"

# 5. Build headless (CGO-free)
step "Building headless binary"
CGO_ENABLED=0 go build -tags headless \
    -ldflags "-s -w -X main.version=${GIT_VERSION}" \
    -o zenzx-headless . 2>&1 | tail -3
ok "Headless build clean (CGO-free)"

# 6. Tests
step "Running tests (single pass)"
COVER_OUT="cover.out"
set +e
# pkg/version (build-tag agnostic) plus the root package tests, which exercise
# .bin loading and memory.Load and are headless-tagged (NewZenZX pulls raylib
# in a GUI build, so the unit tests run under -tags headless).
go test -count=1 -coverprofile="$COVER_OUT" ./pkg/... > test-errors.txt 2>&1
TEST_EXIT=$?
go test -tags headless -count=1 . >> test-errors.txt 2>&1
TEST_EXIT2=$?
set -e
if [ $TEST_EXIT -ne 0 ] || [ $TEST_EXIT2 -ne 0 ]; then
    cat test-errors.txt >&2
    fail "Tests failed -- aborting"
fi
ok "All tests passed (pkg + headless root package)"

# 7. Headless smoke test
step "Headless smoke test"
SHOT_DIR="$(mktemp -d)"
if ./zenzx-headless -model=48k -romdir=./rom -frames=100 \
       -shot-dir="$SHOT_DIR" -shot-prefix=smoke -quiet > /dev/null 2>&1; then
    SHOT="$SHOT_DIR/smoke-frame000100.png"
    if [ -f "$SHOT" ]; then
        # PNG magic check: first 8 bytes = 89 50 4E 47 0D 0A 1A 0A
        MAGIC=$(hexhead "$SHOT" 8)
        if [ "$MAGIC" = "89504e470d0a1a0a" ]; then
            ok "Booted 48K and produced a valid PNG screenshot"
        else
            rm -rf "$SHOT_DIR"; fail "Screenshot is not a valid PNG"
        fi
    else
        rm -rf "$SHOT_DIR"; fail "Smoke test produced no screenshot"
    fi
else
    rm -rf "$SHOT_DIR"; fail "Headless smoke test failed to run"
fi
rm -rf "$SHOT_DIR"

# 8. Consistency
step "Consistency check"
./syncver.sh check
ok "All version strings consistent: $VERSION"

# 9. Checkpoint
if $CUT_ZIP; then
    step "Cutting checkpoint"
    ZIPNAME="zenzx-v${VERSION}-checkpoint.zip"
    rm -f "$ZIPNAME"

    # Explicit source list -- never zip '.' or a parent dir. rom/ holds ROM
    # images and ZEX test binaries (data files, not host executables), included
    # deliberately; the magic-byte scan below skips them.
    ZIP_SOURCES=(
        VERSION CHANGELOG.md
        go.mod
        syncver.sh release.sh build.sh build_linux.sh build_windows.sh
        build_bsd.sh build_example_bsd.sh _bsd.sh build_headless.sh Dockerfile
        zxspectrum.txt
        pkg/ rom/ docs/
    )
    [ -f go.sum ] && ZIP_SOURCES+=(go.sum)
    [ -f README.md ] && ZIP_SOURCES+=(README.md)
    [ -f LICENSE ] && ZIP_SOURCES+=(LICENSE)
    [ -f NOTICE ] && ZIP_SOURCES+=(NOTICE)
    # All Go sources at the package root.
    for f in *.go; do ZIP_SOURCES+=("$f"); done
    # Bundled formatted DSK image for disk-write experimentation.
    [ -f zenzx-formatted.dsk ] && ZIP_SOURCES+=(zenzx-formatted.dsk)

    zip -X -r "$ZIPNAME" "${ZIP_SOURCES[@]}" \
        -x "*.out" -x "test-errors.txt" -x "cover.out" \
        -x "*.tmp" -x "*.test" \
        -x "zenzx-headless" -x "zenzx" -x "zenzx_linux" -x "zenzx_windows*" \
        -x "*.so" -x "*.dylib" -x "*.dll" -x "*.a" -x "*.o" \
        > /dev/null 2>&1

    ART_COUNT=$(unzip -l "$ZIPNAME" | grep -cE '\.test$|cover\.out$|test-errors\.txt$' || true)
    [ "$ART_COUNT" -gt 0 ] && fail "Checkpoint contains $ART_COUNT test artifact file(s)"

    BINARY_FILES=""
    while IFS= read -r entry; do
        magic=$(unzip -p "$ZIPNAME" "$entry" 2>/dev/null | head -c 4 | od -An -tx1 | tr -d ' \n' || true)
        case "$magic" in
            7f454c46) BINARY_FILES="$BINARY_FILES\n  ELF:    $entry" ;;
            cafebabe|feedface|feedfacf) BINARY_FILES="$BINARY_FILES\n  Mach-O: $entry" ;;
            4d5a*)     BINARY_FILES="$BINARY_FILES\n  PE/MZ:  $entry" ;;
        esac
    done < <(unzip -l "$ZIPNAME" | awk 'NR>3 && /[^/]$/ {print $NF}' | grep -v '^rom/' | head -200)
    if [ -n "$BINARY_FILES" ]; then
        printf "   Detected binary files in checkpoint:%b\n" "$BINARY_FILES" >&2
        fail "Checkpoint contains binary files -- aborting"
    fi

    ZIP_SIZE=$(du -sh "$ZIPNAME" | cut -f1)
    ok "Created: ${ZIPNAME} (${ZIP_SIZE})"
else
    warn "Skipping zip (--no-zip)"
fi

echo ""
echo "======================================"
echo "  Release v${VERSION} prepared"
echo "  Build tag (headless): ${GIT_VERSION}"
$CUT_ZIP && echo "  Zip: zenzx-v${VERSION}-checkpoint.zip"
echo "======================================"
echo ""
echo "Note: the GUI binary (raylib + oto) is built separately on a machine"
echo "with the required system libraries, via build.sh / build_linux.sh etc."
