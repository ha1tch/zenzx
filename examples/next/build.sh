#!/bin/sh
# Build the Next examples: pictures to the Next's formats with zx (zentools), code with
# zenas, and the two together into a .nex with zx.
#
#   sh build.sh              all of them
#   sh build.sh gallop       one
#
# zx 0.14.4 or later and zenas 0.11.2 or later must be on the PATH, or named in ZX and ZENAS.
set -u
ZX=${ZX:-zx}
ZENAS=${ZENAS:-zenas}
cd "$(dirname "$0")" || exit 1

need() {
	command -v "$1" >/dev/null 2>&1 || { echo "build.sh: $1 not found ($2)" >&2; exit 1; }
}
need "$ZX" "zentools: https://github.com/ha1tch/zentools"
need "$ZENAS" "https://github.com/ha1tch/zenas"

gallop() {
	mkdir -p build &&
	"$ZX" next bitmap encode -mode 256x192 -palette build/sierra.nxp -o build/sierra.nxi art/sierra.png &&
	"$ZX" next sprite encode -depth 4 -transparent 3 -palette build/horse.nxp -o build/horse.spr art/horse.png &&
	"$ZENAS" assemble gallop.asm build/gallop.bin --next --no-hex &&
	"$ZX" next nex build -screen l2=build/sierra.nxi -palette build/sierra.nxp \
		-bin 0x8000=build/gallop.bin -pc 0x8000 -sp 0xFF00 -border 0 -o gallop.nex
}

fuji() {
	mkdir -p build &&
	"$ZX" next bitmap encode -mode 320x256 -palette build/sky.nxp -o build/sky.nxi art/sky.png &&
	"$ZX" next tiles encode -depth 4 -colors 16 -transparent 15 -dedup mirror -map16 \
		-palette build/mountain.nxp -o build/mountain art/mountain.png &&
	"$ZX" next sprite encode -std -transparent 0xE3 -o build/title.spr art/title.png &&
	"$ZENAS" assemble fuji.asm build/fuji.bin --next --no-hex || return 1
	# The tiles follow the map in bank 5 and must end before 0x8000.
	tiles=$(wc -c <build/mountain.nxt)
	[ "$tiles" -le 13824 ] || { echo "the mountain's tiles are $tiles bytes; 13824 fit" >&2; return 1; }
	# The sky goes into the file as the five banks Layer 2 shows at 320x256. (As a loading
	# screen it would need NEX V1.3, which the .nexload on the distribution's card refuses.)
	for i in 0 1 2 3 4; do
		dd if=build/sky.nxi of=build/sky.$i bs=16384 skip=$i count=1 2>/dev/null || return 1
	done
	"$ZX" next nex build -bank 9=build/sky.0 -bank 10=build/sky.1 -bank 11=build/sky.2 \
		-bank 12=build/sky.3 -bank 13=build/sky.4 \
		-bin 0x4000=build/mountain.nxm -bin 0x4A00=build/mountain.nxt \
		-bin 0x8000=build/fuji.bin -pc 0x8000 -sp 0xFF00 -border 0 -o fuji.nex
}

orbit() {
	mkdir -p build &&
	"$ZX" next bitmap encode -mode 256x192 -transparent 0xE3 -palette build/earth.nxp -o build/earth.nxi art/earth.png &&
	"$ZX" next sprite encode -transparent 0xE3 -palette build/bodies.nxp -o build/bodies.spr art/bodies.png &&
	"$ZENAS" assemble orbit.asm build/orbit.bin --next --no-hex &&
	"$ZX" next nex build -screen l2=build/earth.nxi -palette build/earth.nxp \
		-bin 0x8000=build/orbit.bin -pc 0x8000 -sp 0xFF00 -border 0 -o orbit.nex
}

[ $# -eq 0 ] && set -- gallop fuji orbit
for name in "$@"; do
	case "$name" in
	gallop | fuji | orbit) ;;
	*) echo "build.sh: no example called $name" >&2; exit 1 ;;
	esac
	echo "== $name"
	(cd "$name" && "$name" >build.log 2>&1) || { cat "$name/build.log" >&2; echo "build.sh: $name failed" >&2; exit 1; }
	tail -n 1 "$name/build.log"
done
