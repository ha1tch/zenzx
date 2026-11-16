package main

// NextREG 0x1E and 0x1F: the line the display is drawing (T-36).
//
// The pair reads back a 9-bit line number, bit 8 in 0x1E bit 0 and the low byte in 0x1F.
// Line 0 is the first line of the pixel area; the count runs through the bottom border,
// the vertical blank and the top border and wraps at the frame's line count. The firmware
// paces its splash screen by waiting for bit 8 to change, so a register that always reads
// 0 stops the boot there.
//
// The number is worked out from the T-state position in the frame, with the same frame
// origin the ULA interrupt and the contention model use.

// nextVideoLine returns the 9-bit active video line for the current CPU position.
func (zx *ZenZX) nextVideoLine() int {
	tPerLine, firstPixelLine := 224, 64
	if zx.is128K || zx.isPlus3 {
		tPerLine, firstPixelLine = 228, 63
	}
	lines := zx.cyclesPerFrame / tPerLine
	if lines <= 0 {
		return 0
	}
	pos := int(zx.cpu.Cycles - zx.frameOrigin)
	if pos < 0 {
		pos = 0
	}
	raster := (pos / tPerLine) % lines
	return (raster - firstPixelLine + lines) % lines
}
