package zenzx

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

// NextREG 0x64 is added to the count: with 32 in it, line 0 is the first line of the
// 320x256 frame, 32 lines above the pixel area.

// NextFrameCycles is a Next's frame: 311 lines of 228 T-states, the 128K machines' timing
// that the Next has at 50 Hz. The line counter, the copper and the line interrupt count
// those 311 lines.
const NextFrameCycles = 311 * 228

// nextFrameLength gives a Next its own frame length, and takes it away again from a
// machine that has stopped being one. The other machines, the 128K ones included, run a
// frame of 69888 T-states (T-55).
func (zx *ZenZX) nextFrameLength() {
	switch next := zx.memory != nil && zx.memory.isNext; {
	case next && zx.cyclesPerFrame == ULAFrameCycles:
		zx.cyclesPerFrame = NextFrameCycles
	case !next && zx.cyclesPerFrame == NextFrameCycles:
		zx.cyclesPerFrame = ULAFrameCycles
	}
}

// rasterTiming is the CPU's place in the frame and the frame's shape (nextraster.go).
func (zx *ZenZX) rasterTiming() nextRasterTiming {
	tm := nextRasterTiming{tPerLine: 224, firstPixelLine: 64}
	if zx.is128K || zx.isPlus3 {
		tm.tPerLine, tm.firstPixelLine = 228, 63
	}
	tm.lines = zx.cyclesPerFrame / tm.tPerLine
	if tm.lines <= 0 {
		tm.lines = 1
	}
	if tm.t = int(zx.cpu.Cycles - zx.frameOrigin); tm.t < 0 {
		tm.t = 0
	}
	return tm
}

// nextVideoLine returns the 9-bit active video line for the current CPU position.
func (zx *ZenZX) nextVideoLine() int {
	tm := zx.rasterTiming()
	raster := (tm.t / tm.tPerLine) % tm.lines
	offset := int(zx.io.nextRegs.regs[0x64])
	return ((raster-tm.firstPixelLine+offset)%tm.lines + tm.lines) % tm.lines
}
