#!/bin/bash
# build_headless.sh - Build the CGO-free headless ZenZX binary.
#
# The headless build has no window and no audio device: it renders the
# Spectrum framebuffer to PNG screenshots. It needs no system libraries
# (no GL, X11, Wayland, or ALSA) and cross-compiles trivially.
set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || cat VERSION 2>/dev/null || echo dev)
OUT="${1:-zenzx-headless}"

echo "Building headless ZenZX ${VERSION} -> ${OUT}"
CGO_ENABLED=0 go build -tags headless \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "${OUT}" .

echo "Done:"
ls -lh "${OUT}"
