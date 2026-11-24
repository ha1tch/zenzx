package zenzx

// The copper.
//
// The Next's copper is a small processor that follows the beam. Its program is a list of
// up to 1024 instructions of two kinds: wait for a place on the screen, and write a value
// to a NextREG. It changes registers at exact places in the frame without the CPU, and is
// how programs get a new palette, scroll or layer order on each line.
//
// From the Next's registers.txt:
//
//	0x60 (W)    a byte for the program memory at the write address, which then goes up
//	0x61        the write address, low byte
//	0x62        bits 2-0 the write address, high bits (the memory is 2K: 0 to 0x7FF)
//	            bits 7-6 start control:
//	              00 stopped
//	              01 start from instruction 0, and go round to the start at the end
//	              10 start from where it stopped, and go round to the start at the end
//	              11 start from instruction 0, and go back to it each time the beam
//	                 is at (0,0)
//	            Writing the start control it already has does not start it again.
//	0x63 (W)    as 0x60, but the first byte of a pair, for an even address, is held and
//	            both are written together when the second arrives
//	0x64        added to the line counter the copper (and NextREG 0x1E/0x1F) sees: with
//	            0, line 0 is the first line of the pixel area; with 32, line 0 is the
//	            first line of the 320x256 frame
//	The copper cannot reach registers 0x80 and above.
//
// and the instructions, from the Next's documentation of the copper:
//
//	1hhhhhhv vvvvvvvv  WAIT   until the line is v and the beam is at or past h*8 pixels
//	                          across it; h 0 is the left edge of the pixel area
//	0rrrrrrr vvvvvvvv  MOVE   write v to NextREG r; register 0 writes nothing (NOOP)
//
// A WAIT for a line the frame does not have (HALT is WAIT 63,511) is never satisfied. A
// WAIT for a line the beam has left waits for it in the next frame, unless start control
// 11 takes the program back to its start first.
//
// The copper's clock is 28 MHz, 8 to a T-state and 4 to a pixel: a WAIT that is satisfied
// and a NOOP take one, a MOVE two. So about two MOVEs fit in a pixel. The copper is run to
// that precision; what it changes shows by whole lines (nextraster.go).
//
// It is run by time, not instruction by instruction with the CPU: whenever the CPU is
// about to read or write a NextREG or change the picture, and at the end of each frame,
// the copper is brought up to the CPU's time (copperAdvance). Nothing the copper does can
// be seen by the CPU any other way. A copper is run only where the picture is drawn in
// pieces (nextraster.go), which is where there is a beam to follow.

const (
	copperInstructions = 1024
	copperMemory       = 2 * copperInstructions
)

type nextCopper struct {
	mem   [copperMemory]uint8
	addr  uint16 // where NextREG 0x60 and 0x63 write
	latch uint8  // the first byte of a 0x63 pair
	mode  uint8  // start control
	pc    uint16 // the instruction it is at, 0 to 1023
	clock int    // how far through the frame it has run, in 28 MHz clocks
	reset bool   // start control 11: it has gone back to the start in this frame
	busy  bool   // it is running: its own writes do not run it again
}

// copperRegWrite is a write to NextREG 0x60-0x63.
func (io *SpectrumIO) copperRegWrite(reg, value uint8) {
	c := &io.copper
	switch reg {
	case 0x60:
		c.mem[c.addr] = value
		c.addr = (c.addr + 1) & (copperMemory - 1)
	case 0x63:
		if c.addr&1 == 0 {
			c.latch = value
		} else {
			c.mem[c.addr-1], c.mem[c.addr] = c.latch, value
		}
		c.addr = (c.addr + 1) & (copperMemory - 1)
	case 0x61:
		c.addr = c.addr&0x700 | uint16(value)
	case 0x62:
		c.addr = c.addr&0x0FF | uint16(value&0x07)<<8
		if mode := value >> 6; mode != c.mode {
			c.mode = mode
			if mode == 1 || mode == 3 {
				c.pc = 0
			}
			if !c.busy && io.raster.on { // it starts from now, not from where it was last run to
				c.clock, _ = io.rasterClock()
			}
		}
	}
}

// copperRegRead is a read of NextREG 0x61 or 0x62.
func (io *SpectrumIO) copperRegRead(reg uint8) uint8 {
	if reg == 0x61 {
		return uint8(io.copper.addr)
	}
	return io.copper.mode<<6 | uint8(io.copper.addr>>8)
}

// copperStop is a reset of the machine: the copper stops. Its program stays in memory.
func (io *SpectrumIO) copperStop() {
	c := &io.copper
	c.mode, c.pc, c.addr, c.reset = 0, 0, 0, false
	io.nextRegs.regs[0x62] = 0
}

// copperAdvance runs the copper up to a time in the frame, in 28 MHz clocks.
func (io *SpectrumIO) copperAdvance(to int) {
	c := &io.copper
	if c.busy || to <= c.clock {
		return
	}
	if c.mode == 0 {
		c.clock = to
		return
	}
	tm := io.rasterTimingFn()
	lineClocks := tm.tPerLine * 8
	offset := int(io.nextRegs.regs[0x64])
	// rasterLine is the raster line on which the copper's line counter reads v.
	rasterLine := func(v int) int { return ((v-offset+tm.firstPixelLine)%tm.lines + tm.lines) % tm.lines }
	restartAt := rasterLine(0) * lineClocks // the beam at (0,0)

	c.busy = true
	defer func() { c.busy = false }()
	for c.clock < to && c.mode != 0 {
		limit := to
		if c.mode == 3 && !c.reset {
			if c.clock >= restartAt {
				c.pc, c.reset = 0, true
			} else if restartAt < limit {
				limit = restartAt
			}
		}
		ins := uint16(c.mem[c.pc*2])<<8 | uint16(c.mem[c.pc*2+1])
		if ins&0x8000 != 0 { // WAIT
			v, h := int(ins&0x1FF), int(ins>>9)&0x3F
			if v >= tm.lines || h*8 >= tm.tPerLine*2 {
				c.clock = limit // not a place on the screen
				continue
			}
			start := rasterLine(v)*lineClocks + h*8*4
			end := (rasterLine(v) + 1) * lineClocks
			if c.clock >= end || start >= limit {
				c.clock = limit // not in what is left of this stretch
				continue
			}
			if c.clock < start {
				c.clock = start
			}
			c.clock++
		} else if reg := uint8(ins >> 8); reg == 0 { // NOOP
			c.clock++
		} else { // MOVE
			c.clock += 2
			io.nextRegWriteDirect(reg, uint8(ins))
		}
		c.pc = (c.pc + 1) & (copperInstructions - 1)
	}
}

// copperFrameEnd runs the copper to the end of a frame of the given length and puts its
// clock back to the start for the next one.
func (io *SpectrumIO) copperFrameEnd(frameClocks int) {
	io.copperAdvance(frameClocks)
	io.copper.clock, io.copper.reset = 0, false
}
