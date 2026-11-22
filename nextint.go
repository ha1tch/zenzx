package zenzx

// The Next's own interrupt mode, and its CTC.
//
// A Next has more things that interrupt than the frame: the line interrupt, CTC timers
// and the UARTs. In the ordinary ("pulse") mode each one pulls the interrupt line for a
// short time and puts 0xFF on the bus, as the ULA always did. NextREG 0xC0 bit 0 turns on
// the Next's own mode, "hardware IM 2": an interrupt then waits until the CPU takes it,
// and each source gives its own vector, so a program can have a handler for each without
// working out what happened. From the Next's registers.txt:
//
//	0xC0  bit 0     hardware IM 2 mode
//	      bits 7-5  the top three bits of every vector; bits 4-1 are the source:
//	                0 line, 1-2 UART receive, 3-10 CTC channels 0-7, 11 ULA (the
//	                frame), 12-13 UART transmit; bit 0 is 0
//	0xC4  bit 0     ULA interrupt enabled      bit 1  line interrupt enabled
//	                (the same two as NextREG 0x22 bit 2, inverted, and bit 1)
//	0xC5            CTC channels' interrupts enabled, a bit each
//	0xC8, 0xC9      status, the same bits: set when the source has interrupted or is
//	                waiting; writing a 1 clears the bit
//
// When several are waiting the one with the lowest source number is taken first. The CPU
// taking an interrupt (it asks for the vector) is what ends that source's wait.
//
// The CTC is the Zilog part, four channels on ports 0x183B, 0x193B, 0x1A3B and 0x1B3B. A
// write with bit 0 set is a control word: bit 7 interrupt, bit 6 counter (1) or timer (0),
// bit 5 prescaler 256 (1) or 16 (0), bit 3 wait for a trigger to start, bit 2 a time
// constant follows, bit 1 reset. A timer counts the 28 MHz clock through its prescaler
// down from the time constant (0 is 256) and at zero starts again and gives a pulse; a
// counter counts the pulses of the channel before it. Reading the port gives the count.
// A channel that was reset does nothing until it is given a time constant.
//
// Left out: the UARTs; the CTC's interrupts in the ordinary mode (a channel then only sets
// its status bit); the chain that holds a lower source back while a higher one's handler
// runs; NextREG 0xCC to 0xCE (which interrupts may break into a DMA transfer); and a CPU
// that is not in IM 2 while the hardware mode is on, which takes no vector and so never
// ends a source's wait.

const (
	nextIntLine = 0
	nextIntCTC0 = 3
	nextIntULA  = 11
)

type nextInterrupts struct {
	hw      bool
	pending uint16 // a bit for each source waiting, by source number
	ctc     [4]ctcChannel
	ctcOn   bool // some channel is counting time
}

type ctcChannel struct {
	control uint8
	tc      int  // time constant, 1 to 256
	count   int  // the down counter
	pre     int  // clocks not yet a prescaler step
	running bool // it has a time constant and is counting
	wantTC  bool // the next write is the time constant
}

func (io *SpectrumIO) nextIsNext() bool { return io.memory != nil && io.memory.isNext }

// nextIntRegWrite is a write to NextREG 0xC0, 0xC4, 0xC5, 0xC8 or 0xC9, or to 0x22, whose
// two enable bits are 0xC4's. It returns the value to keep in the register.
func (io *SpectrumIO) nextIntRegWrite(reg, v uint8) uint8 {
	n := &io.nextInt
	r := io.nextRegs.regs[:]
	switch reg {
	case 0xC0:
		n.hw = v&0x01 != 0
		if !n.hw {
			n.pending = 0
		}
	case 0xC4: // the enables live in NextREG 0x22
		r[0x22] = r[0x22]&^0x06 | v&0x02
		if v&0x01 == 0 {
			r[0x22] |= 0x04
		}
		io.nextLineIntSet(0, 0)
		return v & 0x80
	case 0xC8, 0xC9:
		return r[reg] &^ v // a 1 clears
	}
	return v
}

// nextIntRegRead is a read of NextREG 0xC4, 0xC8 or 0xC9.
func (io *SpectrumIO) nextIntRegRead(reg uint8) uint8 {
	r := io.nextRegs.regs[:]
	switch reg {
	case 0xC4:
		v := r[0xC4]&0x80 | r[0x22]&0x02
		if r[0x22]&0x04 == 0 {
			v |= 0x01
		}
		return v
	case 0xC8:
		v := r[0xC8]
		if io.nextInt.pending&(1<<nextIntLine) != 0 {
			v |= 0x02
		}
		if io.nextInt.pending&(1<<nextIntULA) != 0 {
			v |= 0x01
		}
		return v
	}
	return r[0xC9] | uint8(io.nextInt.pending>>nextIntCTC0)
}

// nextIntRaise is a source interrupting: its status bit is set and, in the hardware mode
// with the source enabled, it waits for the CPU. It reports whether the hardware mode took
// it; if not, the caller gives the ordinary pulse where the source has one.
func (io *SpectrumIO) nextIntRaise(source int) bool {
	if !io.nextIsNext() {
		return false
	}
	r := io.nextRegs.regs[:]
	enabled := false
	switch {
	case source == nextIntLine:
		r[0xC8] |= 0x02
		enabled = r[0x22]&0x02 != 0
	case source == nextIntULA:
		r[0xC8] |= 0x01
		enabled = r[0x22]&0x04 == 0
	default:
		ch := uint(source - nextIntCTC0)
		r[0xC9] |= 1 << ch
		enabled = r[0xC5]&(1<<ch) != 0 || io.nextInt.ctc[ch].control&0x80 != 0
	}
	if !io.nextInt.hw {
		return false
	}
	if enabled {
		io.nextInt.pending |= 1 << uint(source)
	}
	return true
}

// nextIntVector is the CPU taking an interrupt in the hardware mode: the waiting source
// with the lowest number gives its vector and stops waiting.
func (io *SpectrumIO) nextIntVector() uint8 {
	n := &io.nextInt
	for source := 0; source < 16; source++ {
		if n.pending&(1<<uint(source)) != 0 {
			n.pending &^= 1 << uint(source)
			return io.nextRegs.regs[0xC0]&0xE0 | uint8(source)<<1
		}
	}
	return io.nextRegs.regs[0xC0]&0xE0 | nextIntULA<<1 // nothing waiting: taken as the frame's
}

// ctcPortWrite is a write to a CTC channel's port.
func (io *SpectrumIO) ctcPortWrite(ch int, v uint8) {
	c := &io.nextInt.ctc[ch]
	switch {
	case c.wantTC:
		c.wantTC = false
		c.tc = int(v)
		if c.tc == 0 {
			c.tc = 256
		}
		c.count, c.pre, c.running = c.tc, 0, true
	case v&0x01 != 0: // control word
		c.control = v
		if v&0x02 != 0 { // reset: stops until a time constant comes
			c.running = false
		}
		c.wantTC = v&0x04 != 0
	}
	// a write with bit 0 clear is the Zilog part's vector, which the Next does not use
	io.nextInt.ctcOn = false
	for i := range io.nextInt.ctc {
		if io.nextInt.ctc[i].running && io.nextInt.ctc[i].control&0x40 == 0 {
			io.nextInt.ctcOn = true
		}
	}
}

// ctcPortRead is a read of a CTC channel's port: its count.
func (io *SpectrumIO) ctcPortRead(ch int) uint8 {
	return uint8(io.nextInt.ctc[ch].count)
}

// ctcTick moves the timers on by a number of 28 MHz clocks.
func (io *SpectrumIO) ctcTick(clocks int) {
	for ch := range io.nextInt.ctc {
		c := &io.nextInt.ctc[ch]
		if !c.running || c.control&0x40 != 0 {
			continue
		}
		prescaler := 16
		if c.control&0x20 != 0 {
			prescaler = 256
		}
		c.pre += clocks
		for ; c.pre >= prescaler; c.pre -= prescaler {
			io.ctcStep(ch, 0)
		}
	}
}

// ctcStep takes one off a channel's count; at zero it starts again and gives its pulse,
// which interrupts and is counted by the next channel if that one is a counter.
func (io *SpectrumIO) ctcStep(ch, depth int) {
	c := &io.nextInt.ctc[ch]
	c.count--
	if c.count > 0 {
		return
	}
	c.count = c.tc
	io.nextIntRaise(nextIntCTC0 + ch)
	next := (ch + 1) % len(io.nextInt.ctc)
	if n := &io.nextInt.ctc[next]; depth < len(io.nextInt.ctc) && n.running && n.control&0x40 != 0 {
		io.ctcStep(next, depth+1)
	}
}

// ctcChannelOfPort gives the channel a port is, or -1.
func ctcChannelOfPort(port uint16) int {
	if port&0xFF == 0x3B && port>>8 >= 0x18 && port>>8 <= 0x1B {
		return int(port>>8) - 0x18
	}
	return -1
}
