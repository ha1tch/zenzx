package zenzx

// The Next's line interrupt: NextREG 0x22 and 0x23.
//
//	0x22  bit 7 (R)  the ULA is asserting its interrupt, whether or not it is disabled
//	      bit 2      disable the ULA's interrupt, the one at the start of each frame
//	      bit 1      enable the line interrupt
//	      bit 0      bit 8 of the line
//	0x23             bits 7-0 of the line
//
// The line is counted as NextREG 0x1E/0x1F and the copper count it: 0 is the first line of
// the pixel area, and NextREG 0x64 is added. The interrupt for line y is raised on line
// y-1 as the beam leaves the pixel area, 256 pixels across, so that the program has the
// right border and the blank to get ready for line y. It is a pulse of the same length as
// the frame interrupt: if the CPU has interrupts disabled for all of it, it is lost, as on
// the machine in its ordinary interrupt mode. A handler that sets the line again for a
// place further down the frame gets another interrupt there in the same frame, which is
// how one interrupt splits a screen several times.
//
// Left out: the Next's own interrupt mode with its vectors, enables and status (NextREG
// 0xC0 to 0xCF). In the ordinary mode the line interrupt puts 0xFF on the bus, as the
// frame interrupt does.

// nextLineInt is where in the frame the line interrupt is raised, and how far it has got.
type nextLineInt struct {
	on     bool
	at     int  // T-states from the frame's origin
	raised bool // it has been raised in this frame, or its place had passed when it was set
	until  int  // when the pulse ends, in T-states of the frame; 0 when it is not up
}

// nextLineIntSet works the place out again from the registers. reg and value are a write
// that is about to be stored (0x22, 0x23 or 0x64), or 0, 0 for none.
func (io *SpectrumIO) nextLineIntSet(reg, value uint8) {
	li := &io.nextLineInt
	if io.memory == nil || !io.memory.isNext || io.rasterTimingFn == nil {
		li.on = false
		return
	}
	get := func(r uint8) uint8 {
		if r == reg {
			return value
		}
		return io.nextRegs.regs[r]
	}
	if get(0x22)&0x02 == 0 {
		li.on = false
		return
	}
	tm := io.rasterTimingFn()
	line := int(get(0x22)&0x01)<<8 | int(get(0x23))
	if line >= tm.lines {
		li.on = false // not a line of this frame
		return
	}
	// The raster line on which the counter reads line, and the one before it.
	raster := ((line-int(get(0x64))+tm.firstPixelLine)%tm.lines + tm.lines) % tm.lines
	before := (raster - 1 + tm.lines) % tm.lines
	li.on, li.at = true, before*tm.tPerLine+ScreenWidth/2
	li.raised = tm.t >= li.at
}

// nextLineIntFrameStart begins a frame: the interrupt has not been raised in it.
func (io *SpectrumIO) nextLineIntFrameStart() {
	if io.nextLineInt.on || (io.memory != nil && io.memory.isNext) {
		io.nextLineIntSet(0, 0)
		io.nextLineInt.raised, io.nextLineInt.until = false, 0
	}
}

// nextULAIntOff reports NextREG 0x22 bit 2: no frame interrupt.
func (io *SpectrumIO) nextULAIntOff() bool {
	return io.memory != nil && io.memory.isNext && io.nextRegs.regs[0x22]&0x04 != 0
}

// nextLineIntRegRead is a read of NextREG 0x22.
func (io *SpectrumIO) nextLineIntRegRead() uint8 {
	v := io.nextRegs.regs[0x22] & 0x07
	if io.rasterTimingFn != nil && io.rasterTimingFn().t < InterruptLength {
		v |= 0x80
	}
	return v
}

// lineInterrupt raises the line interrupt when the frame reaches its place and drops it
// when the pulse is over. It is called after each instruction while the interrupt is on,
// with the T-states of the frame so far.
func (zx *ZenZX) lineInterrupt(frameCycles int) {
	li := &zx.io.nextLineInt
	if !li.raised {
		if frameCycles >= li.at {
			li.raised = true
			if !zx.io.nextIntRaise(nextIntLine) { // the ordinary mode: a pulse
				zx.cpu.INT = true
				li.until = frameCycles + zx.interruptLength
			}
		}
		return
	}
	if li.until != 0 && frameCycles >= li.until {
		li.until = 0
		if zx.io.mouseMode != MouseAMX || (!zx.io.amxVectorPending && len(zx.io.amxIntQueue) == 0) {
			zx.cpu.INT = false
		}
	}
}
