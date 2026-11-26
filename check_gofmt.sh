#!/bin/sh
# Release gate, and CI's first step: every Go file in the tree is formatted
# as gofmt would format it.
#
# It runs the gofmt of the Go the module builds with (go.mod's version, which
# `go` switches to by itself), not whichever gofmt happens to be first on
# PATH: an older one can call a tree clean that CI, on the right version,
# then rejects.
#
#   sh check_gofmt.sh        list the files gofmt would change; exit 1 if any
#   sh check_gofmt.sh -w     format them in place
set -e
cd "$(dirname "$0")"

fmt="$(go env GOROOT)/bin/gofmt"
[ -x "$fmt" ] || fmt=gofmt

if [ "$1" = "-w" ]; then
    "$fmt" -w .
    echo "gofmt: formatted ($(go env GOVERSION))"
    exit 0
fi

out=$("$fmt" -l .)
if [ -n "$out" ]; then
    printf '\033[31mgofmt would change:\033[0m\n%s\n' "$out"
    echo "run: sh check_gofmt.sh -w"
    exit 1
fi
echo "gofmt clean ($(go env GOVERSION), $(find . -name '*.go' -not -path './.git/*' | wc -l | tr -d ' ') files)"
