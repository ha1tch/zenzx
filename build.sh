#!/bin/bash
# build.sh - Build the native GUI ZenZX binary (raylib + oto).
#
# Uses the Go package build so the file set is determined by build tags and
# can never drift out of sync (the old hand-maintained file list did).
# Requires CGO and the system libraries raylib/oto need: OpenGL, X11 or
# Wayland, and ALSA. For a CGO-free build with no window, use build_headless.sh.
set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || cat VERSION 2>/dev/null || echo dev)
OUT="${1:-zenzx}"

echo "Building GUI ZenZX ${VERSION} -> ${OUT}"
CGO_ENABLED=1 go build -ldflags "-s -w -X main.version=${VERSION}" -o "${OUT}" .

echo "Done:"
ls -lh "${OUT}"
