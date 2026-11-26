#!/bin/sh
# build_windows.sh - Cross-compile the GUI ZenZX binary for Windows (amd64).
# Requires the mingw-w64 toolchain. Uses the Go package build (no file list).
set -e

export CGO_ENABLED=1
export GOOS=windows
export GOARCH=amd64
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++

VERSION=$(git describe --tags --always --dirty 2>/dev/null || cat VERSION 2>/dev/null || echo dev)

echo "Cross-compiling ZenZX ${VERSION} for Windows amd64..."
go build -v -ldflags "-s -w -H=windowsgui -X github.com/ha1tch/zenzx.version=${VERSION}" -o zenzx.exe ./cmd/zenzx

echo "Done:"
ls -lh zenzx.exe
