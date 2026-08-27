package main

// ============================================================================
// Floating bus (48K / 128K)
// ============================================================================
//
// Reading a port no device claims does not return 0xFF on a 48K/128K
// Spectrum while the ULA is fetching display data: the data bus floats
// to the byte the ULA is reading at that T-state. Within each visible
// line's 128-T-state fetch window the ULA reads, per 8 T-states:
// bitmap byte n, attribute n, bitmap n+1, attribute n+1, then four idle
// cycles (0xFF). Ocean-era code (five of the nine corpus failures are
// Ocean titles) polls this for raster synchronisation; without it such
// loops wait forever on a bus that never changes.
//
// Reference: FUSE's spectrum_unattached_port (spectrum.c), which this
// mirrors against zenzx's own frame-origin timebase.
//
// Precision note: the position uses cpu.Cycles (instruction-start
// granularity), not the exact T-state of the IN's IO cycle -- up to
// ~11 T-states early. Sync-polling loops are indifferent to this;
// pixel-exact floating-bus effects would need the same offset the
// contention hooks receive. Scope notes: +2A/+3 genuinely has no
// floating bus (returns 0xFF); TS2068 is out of scope; the 128K
// shares the 48K model here, ignoring 128K-specific port-decode
// subtleties -- documented approximation, same spirit as
// setupContention128.
func (zx *ZenZX) floatingBusByte() uint8 {
	if zx.memory == nil || zx.memory.isTS2068 || zx.memory.isPlus3 {
		return 0xFF
	}
	pos := int(zx.cpu.Cycles - zx.frameOrigin)
	rel := pos - 14336 // first screen pixel, per the frame-origin timebase
	if rel < 0 {
		return 0xFF
	}
	line := rel / 224
	if line >= 192 {
		return 0xFF
	}
	tl := rel % 224
	if tl >= 128 {
		return 0xFF
	}
	col := (tl / 8) * 2
	scr := zx.memory.screen
	if scr == nil {
		return 0xFF
	}
	switch tl % 8 {
	case 0:
		return scr.bitmap[bitmapOffset(line, col)]
	case 1:
		return scr.attributes[(line/8)*32+col]
	case 2:
		return scr.bitmap[bitmapOffset(line, col+1)]
	case 3:
		return scr.attributes[(line/8)*32+col+1]
	}
	return 0xFF
}

// bitmapOffset maps (screen line 0-191, column 0-31) to the classic
// interleaved bitmap layout's offset within the 6144-byte pixel area.
func bitmapOffset(line, col int) int {
	return ((line & 0xC0) << 5) | ((line & 0x07) << 8) | ((line & 0x38) << 2) | col
}
