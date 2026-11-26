#!/bin/sh
# check_release_binaries.sh - Release gate: build the programs the Release workflow builds
# without a C compiler (the headless program for every system, the window program for macOS
# and Windows) and look at each with tools/check_binaries.py, as that workflow does once a tag
# is pushed. A program that would fail there fails here, before the tag.
# Needs the Go toolchain and LIEF (pip install lief; on a Mac, pip3 install lief).
set -e
cd "$(dirname "$0")"
python3 -c 'import lief' 2>/dev/null || { echo "LIEF is missing: pip install lief (on a Mac, pip3 install lief)"; exit 1; }
VERSION=$(cat VERSION 2>/dev/null || echo dev)
DIST=$(mktemp -d)
trap 'rm -rf "$DIST"' EXIT

build() {
    # build OS ARCH KIND: KIND is headless or gui
    os="$1"; arch="$2"; kind="$3"
    ext=""
    flags="-s -w"
    [ "$os" = windows ] && ext=".exe"
    if [ "$kind" = headless ]; then
        out="$DIST/zenzx-$os-$arch-headless"
        mkdir -p "$out"
        CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -tags headless \
            -ldflags "$flags -X github.com/ha1tch/zenzx.version=$VERSION" \
            -o "$out/zenzx-headless$ext" ./cmd/zenzx-headless
    else
        [ "$os" = windows ] && flags="$flags -H=windowsgui"
        out="$DIST/zenzx-$os-$arch"
        mkdir -p "$out"
        CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
            -ldflags "$flags -X github.com/ha1tch/zenzx.version=$VERSION" \
            -o "$out/zenzx$ext" ./cmd/zenzx
    fi
}

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64; do
    build "${target%/*}" "${target#*/}" headless
done
for target in darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
    build "${target%/*}" "${target#*/}" gui
done
python3 tools/check_binaries.py -q "$DIST"
echo "Built and checked 7 headless programs and 4 window programs"
