#!/bin/sh
# Start the ZX Spectrum Next with the examples' card: gallop, fuji and orbit.
#
#     sh examples/next/run.sh
#
# launch.py does the work (it makes the card and starts zenzx); this finds a Python to run it,
# or offers to fetch one, and then hands over. The same on Linux and macOS.
set -e
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
examples_dir=$(CDPATH= cd -- "$here/.." && pwd)
. "$examples_dir/tools/findpy.sh"
[ -n "$PY" ] || exit 1
exec "$PY" "$examples_dir/launch.py" next "$@"
