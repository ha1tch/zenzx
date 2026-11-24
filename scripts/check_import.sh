#!/bin/sh
# check_import.sh - show that another Go module can import ZenZX as a library.
#
# Makes a throwaway module in a temporary directory, points it at this checkout, and
# builds and runs a program that makes a machine, runs it and reads its
# picture back, with the tags an embedder uses: headless (no window of
# ZenZX's own) and purego (no cgo). Run it from the repository's root:
#
#   sh scripts/check_import.sh
#
# It needs Go and, the first time, the network or a warm module cache.
set -e

root=$(pwd)
[ -f "$root/go.mod" ] && grep -q '^module github.com/ha1tch/zenzx$' "$root/go.mod" || {
    echo "check_import: run this from the ZenZX repository's root" >&2
    exit 1
}

work=$(mktemp -d "${TMPDIR:-/tmp}/zenzx-import.XXXXXX")
trap 'rm -rf "$work"' EXIT

cat > "$work/go.mod" <<EOF
module example.invalid/embedder

go 1.25

require github.com/ha1tch/zenzx v0.0.0

replace github.com/ha1tch/zenzx => $root
EOF

cat > "$work/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"

	"github.com/ha1tch/zenzx"
)

func main() {
	rom, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	zx := zenzx.NewZenZX(zenzx.AudioBackendOto)
	if err := zx.LoadROMBytes(rom); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	zx.Reset()
	for i := 0; i < 150; i++ {
		zx.RunFrame()
	}
	pic := zx.DecodeDisplay()
	lit := 0
	for i := 0; i < len(pic.Pix); i += 4 {
		if pic.Pix[i] > 0x80 {
			lit++
		}
	}
	if lit == 0 {
		fmt.Fprintln(os.Stderr, "the machine ran but its picture is black")
		os.Exit(1)
	}
	fmt.Printf("embedded machine: %dx%d picture, %d lit pixels\n", pic.Bounds().Dx(), pic.Bounds().Dy(), lit)
}
EOF

cd "$work"
GOWORK=off GOFLAGS=-mod=mod CGO_ENABLED=0 go build -tags headless,purego -o embedder .
out=$(./embedder "$root/rom/48.rom")
echo "$out"
lines=$(printf '%s\n' "$out" | wc -l | tr -d ' ')
[ "$lines" = 1 ] || {
    echo "check_import: the library printed something of its own; it must be silent" >&2
    exit 1
}
echo "check_import: ok"
