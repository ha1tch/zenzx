#!/bin/sh
# bump_build.sh - add one to the build number (pkg/version/build.go).
#
# Run by the release (the "build-number" step of .repoman.json), once for each release built, and not
# by hand. The number is never reset and never goes down: a re-release of a version has the next
# number, and so has the first release of the next. Binaries built from the tree afterwards carry it.
# It is edited through repoman, so the journal and the provenance record are as they were.
set -e

FILE=pkg/version/build.go
CUR=$(sed -n 's/^const Build uint64 = \([0-9][0-9]*\)$/\1/p' "$FILE")
if [ -z "$CUR" ]; then
	echo "bump_build.sh: no 'const Build uint64 = N' in $FILE" >&2
	exit 1
fi
NEXT=$((CUR + 1))
repoman ed sub "const Build uint64 = $CUR" "const Build uint64 = $NEXT" "$FILE" --expect 1
echo "build number $CUR -> $NEXT"
