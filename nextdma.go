package zenzx

// The Next's DMA.
//
// The zxnDMA moves blocks of bytes between memory and ports without the CPU: a program
// writes it a few bytes to say from where, to where and how many, and it does the rest.
// Next games use it to clear and copy screens, to send sprite patterns to port 0x5B and
// streams of values to other ports, and to play samples at a steady rate. It is a subset
// of the Zilog Z80 DMA, programmed the same way, on port 0x6B; port 0x0B is the same
// device in the Z80 DMA's own mode, which moves one byte more than the length it is
// given (the Next's ports.txt, and the specification it names, "The zxnDMA").
//
// A write to the port either starts a register or is a parameter of the one before. The
// register is told by its bit pattern, and bits in it say which parameters follow, lowest
// bit first:
//
//	WR0  0xxxxxAA, AA not 00   bit 2 direction (1 = A to B); then, for bits 3 to 6:
//	                           port A address low, high, block length low, high
//	WR1  0xxxx100              port A: bit 3 it is a port, not memory; bits 5-4
//	                           00 count down, 01 count up, 1x stay; bit 6 a timing
//	                           byte follows
//	WR2  0xxxx000              port B, as WR1; its timing byte's bit 5 says the
//	                           prescaler follows
//	WR3  1xxxxx00              bit 6 enable; bits 3 and 4 a mask and a match byte follow
//	WR4  1xxxxx01              bits 6-5 mode: 01 continuous, 10 burst; then, for bits
//	                           2 and 3: port B address low, high; bit 4 an interrupt
//	                           control byte follows (and what it says follows it)
//	WR5  10xxx010              bit 5 auto restart
//	WR6  1xxxxx11              a command: 0xC3 reset, 0xC7 and 0xCB reset a port's
//	                           timing, 0xCF load, 0xD3 continue, 0x87 enable, 0x83
//	                           disable, 0x8B clear the status, 0xA7 start the read
//	                           sequence, 0xBB the read mask follows, 0xBF read the
//	                           status next
//
// Load takes the two start addresses into the working addresses and sets the count to 0;
// continue sets the count to 0 and keeps the addresses; enable runs the transfer.
//
// Reading the port gives, in turn, what the read mask chooses of: the status byte
// (00E1101T: T set once a byte has gone, E clear once a block has finished), the count
// low and high, port A's address low and high, port B's low and high.
//
// Time. With no prescaler the block goes at once and the CPU waits for it: each byte takes
// the two ports' cycle lengths (4, 3 or 2 CPU cycles each, 3 until set) and those cycles
// are charged to the instruction that enabled the DMA, so a long block runs over the end
// of a frame and its interrupt is missed, as on the machine. With a prescaler p a byte
// goes every 4*p T-states of the 3.5 MHz clock (875 kHz / p), whatever the CPU's speed,
// and the CPU runs in between: that is how samples are played.
//
// Left out: the prescaler in continuous mode holds the CPU on the machine and does not
// here; auto restart is honoured only with a prescaler (without one it would never end);
// the Z80 DMA's search, interrupts and ready line are not there, as on the Next.
//
// DMA+, the zxnDDMA.
//
// From core 3.03.00 the same device is also a blitter, through more commands on port 0x6B
// (the guide on the distribution's card, /docs/zxnDDMA/zxnddma.gde, is the source for all
// of this, and for the corrections it brought to the above: a length of 0 moves one byte,
// reset keeps the addresses, a cycle length of 11 is 4):
//
//	0xD7 nn, 0xDB nn   bits 23-16 of port A's and port B's address
//	0xDF mm [X X Y]    mode flags, all set at once: bit 0 physical addresses, bit 1 2D,
//	                   bit 2 transparent, bit 3 fill, bit 4 a Layer 2 X,Y follows (three
//	                   bytes) and port B's address is worked out from it, bit 5 with bit
//	                   4 the Layer 2 that is not shown (NextREG 0x13) and not 0x12's
//	0xE3, 0xE7         row length and number of rows, 16 bits each
//	0xEB, 0xEF         port A's and port B's stride: from one row's start to the next's
//	0xF3 nn            the transparent colour: a byte equal to it is not written
//	0xF7 nn            the fill value: nothing is read, this is written
//
// On port 0x0B these command bytes are dropped and what follows them is read as ordinary
// register writes, which is how a program finds out whether DMA+ is there.
//
// A physical address is the SRAM's own, with no MMU: 22 bits, the extended byte's low six
// and the 16 of the start address (nextphys.go). It carries from the low 16 bits into the
// extended byte; an ordinary address goes round within 64K. In 2D mode the block is row
// length times number of rows, and after each row both addresses go to the start of the
// row plus their stride. A Layer 2 X,Y sets port B's start address, its extended byte and
// its stride (256): base + Y*256 + X at 256x192, base + X*256 + Y in the column modes,
// where base is 0x40000 + bank * 0x4000.

type nextDMA struct {
	// What the program has set.
	aStart, bStart uint16
	length         uint16
	aToB           bool
	aIO, bIO       bool
	aStep, bStep   int // -1, 0 or +1
	aCycles        int
	bCycles        int
	prescaler      uint8
	burst          bool
	autoRestart    bool
	z80Mode        bool // programmed through port 0x0B: length+1 bytes
	hung           bool // WR4 asked for an interrupt control byte alone: nothing more is taken

	// DMA+.
	aExt, bExt       uint8  // bits 23-16 of the start addresses
	flags            uint8  // 0xDF's
	rowLen, rows     uint16 // 2D
	aStride, bStride uint16
	transparent      uint8
	fill             uint8
	xy               [3]uint8 // a Layer 2 X,Y being received

	// The transfer. The working addresses are 24 bits: the extended byte and the 16.
	aAddr, bAddr uint32
	aRow, bRow   uint32 // where the current row started
	rowPos       int    // bytes of the current row that have gone
	count        int
	enabled      bool
	wait         int  // T-states until the next byte of a timed transfer
	anyByte      bool // status T
	endOfBlock   bool // status E, inverted

	// Programming.
	params   []dmaParam // what the next writes are
	readMask uint8
	readSeq  []uint8
	readPos  int

	stolen int  // CPU cycles a transfer took, not yet charged
	busy   bool // a transfer is moving bytes: its own writes to this port are not programs
}

type dmaParam int

const (
	dmaAStartLo dmaParam = iota
	dmaAStartHi
	dmaLenLo
	dmaLenHi
	dmaATiming
	dmaBTiming
	dmaPrescaler
	dmaBStartLo
	dmaBStartHi
	dmaIntControl
	dmaIgnored
	dmaReadMask
	dmaAExt
	dmaBExt
	dmaFlags
	dmaXLo
	dmaXHi
	dmaY
	dmaRowLenLo
	dmaRowLenHi
	dmaRowsLo
	dmaRowsHi
	dmaAStrideLo
	dmaAStrideHi
	dmaBStrideLo
	dmaBStrideHi
	dmaTransparent
	dmaFill
)

// The mode flags of 0xDF.
const (
	dmaPhysical    = 0x01
	dma2D          = 0x02
	dmaTransp      = 0x04
	dmaFillMode    = 0x08
	dmaLayer2XY    = 0x10
	dmaHiddenLayer = 0x20
)

// reset is the machine's reset: everything as at power on.
func (d *nextDMA) reset() {
	*d = nextDMA{aStep: 1, bStep: 1, aCycles: 3, bCycles: 3, readMask: 0x7F}
}

// commandReset is the command 0xC3. It stops the transfer and puts back the timings, the
// prescaler, auto restart and all of DMA+'s settings; the start addresses, the direction,
// what the ports are and how they count, the mode, the length and the read mask stay.
func (d *nextDMA) commandReset() {
	d.enabled, d.wait = false, 0
	d.anyByte, d.endOfBlock = false, false // the status starts again too, as it always has here
	d.aCycles, d.bCycles, d.prescaler = 3, 3, 0
	d.autoRestart = false
	d.aExt, d.bExt, d.flags = 0, 0, 0
	d.rowLen, d.rows, d.aStride, d.bStride = 0, 0, 0, 0
	d.transparent, d.fill = 0, 0
}

func dmaCycles(timing uint8) int {
	switch timing & 0x03 {
	case 0:
		return 4
	case 1:
		return 3
	case 2:
		return 2
	}
	return 4
}

func dmaStep(bits uint8) int {
	switch bits & 0x03 {
	case 0:
		return -1
	case 1:
		return 1
	}
	return 0
}

// dmaPortWrite is a write to port 0x6B or 0x0B.
func (io *SpectrumIO) dmaPortWrite(port uint16, v uint8) {
	d := &io.dma
	if d.busy || d.hung {
		return
	}
	if d.aCycles == 0 {
		d.reset() // never set up: the state as after a reset
	}
	d.z80Mode = port&0xFF == 0x0B
	if len(d.params) > 0 {
		p := d.params[0]
		d.params = d.params[1:]
		switch p {
		case dmaAStartLo:
			d.aStart = d.aStart&0xFF00 | uint16(v)
		case dmaAStartHi:
			d.aStart = d.aStart&0x00FF | uint16(v)<<8
		case dmaLenLo:
			d.length = d.length&0xFF00 | uint16(v)
		case dmaLenHi:
			d.length = d.length&0x00FF | uint16(v)<<8
		case dmaATiming:
			d.aCycles = dmaCycles(v)
			if v&0x20 != 0 { // a byte follows, as port B's prescaler does, and is not used
				d.params = append([]dmaParam{dmaIgnored}, d.params...)
			}
		case dmaBTiming:
			d.bCycles = dmaCycles(v)
			if v&0x20 != 0 {
				d.params = append([]dmaParam{dmaPrescaler}, d.params...)
			}
		case dmaPrescaler:
			d.prescaler = v
		case dmaBStartLo:
			d.bStart = d.bStart&0xFF00 | uint16(v)
		case dmaBStartHi:
			d.bStart = d.bStart&0x00FF | uint16(v)<<8
		case dmaIntControl: // not on the Next; what it says follows is taken and dropped
			var more []dmaParam
			if v&0x08 != 0 {
				more = append(more, dmaIgnored)
			}
			if v&0x10 != 0 {
				more = append(more, dmaIgnored)
			}
			d.params = append(more, d.params...)
		case dmaReadMask:
			d.readMask = v & 0x7F
			io.dmaReadSequence()
		case dmaAExt:
			d.aExt = v
		case dmaBExt:
			d.bExt = v
		case dmaFlags:
			d.flags = v & 0x3F
			if v&dmaLayer2XY != 0 {
				d.params = append([]dmaParam{dmaXLo, dmaXHi, dmaY}, d.params...)
			}
		case dmaXLo:
			d.xy[0] = v
		case dmaXHi:
			d.xy[1] = v
		case dmaY:
			d.xy[2] = v
			io.dmaLayer2XY()
		case dmaRowLenLo:
			d.rowLen = d.rowLen&0xFF00 | uint16(v)
		case dmaRowLenHi:
			d.rowLen = d.rowLen&0x00FF | uint16(v)<<8
		case dmaRowsLo:
			d.rows = d.rows&0xFF00 | uint16(v)
		case dmaRowsHi:
			d.rows = d.rows&0x00FF | uint16(v)<<8
		case dmaAStrideLo:
			d.aStride = d.aStride&0xFF00 | uint16(v)
		case dmaAStrideHi:
			d.aStride = d.aStride&0x00FF | uint16(v)<<8
		case dmaBStrideLo:
			d.bStride = d.bStride&0xFF00 | uint16(v)
		case dmaBStrideHi:
			d.bStride = d.bStride&0x00FF | uint16(v)<<8
		case dmaTransparent:
			d.transparent = v
		case dmaFill:
			d.fill = v
		}
		return
	}
	follow := func(bit uint8, p dmaParam) {
		if v&bit != 0 {
			d.params = append(d.params, p)
		}
	}
	switch {
	case v&0x80 == 0 && v&0x07 == 0x04: // WR1
		d.aIO, d.aStep = v&0x08 != 0, dmaStep(v>>4)
		follow(0x40, dmaATiming)
	case v&0x80 == 0 && v&0x07 == 0x00: // WR2
		d.bIO, d.bStep = v&0x08 != 0, dmaStep(v>>4)
		follow(0x40, dmaBTiming)
	case v&0x80 == 0: // WR0
		d.aToB = v&0x04 != 0
		follow(0x08, dmaAStartLo)
		follow(0x10, dmaAStartHi)
		follow(0x20, dmaLenLo)
		follow(0x40, dmaLenHi)
	case v&0x03 == 0x00: // WR3
		follow(0x08, dmaIgnored)
		follow(0x10, dmaIgnored)
		if v&0x40 != 0 {
			io.dmaEnable()
		}
	case v&0x03 == 0x01: // WR4
		d.burst = v>>5&0x03 == 0x02
		follow(0x04, dmaBStartLo)
		follow(0x08, dmaBStartHi)
		if v&0x1C == 0x10 {
			// An interrupt control byte and no address: the machine's decoder waits for a
			// byte it has no handler for, and takes nothing more until a reset.
			d.hung = true
		}
	case v&0x03 == 0x02: // WR5
		d.autoRestart = v&0x20 != 0
	default: // WR6
		switch v {
		case 0xC3:
			d.commandReset()
		case 0xC7:
			d.aCycles = 3
		case 0xCB:
			d.bCycles = 3
		case 0xCF:
			d.load()
		case 0xD3:
			d.count, d.rowPos = 0, 0
			d.aRow, d.bRow = d.aAddr, d.bAddr
		case 0x87:
			io.dmaEnable()
		case 0x83:
			d.enabled = false
		case 0x8B:
			d.anyByte, d.endOfBlock = false, false
		case 0xA7:
			io.dmaReadSequence()
		case 0xBB:
			d.params = append(d.params, dmaReadMask)
		case 0xBF:
			d.readSeq, d.readPos = []uint8{0}, 0
		default:
			if !d.z80Mode {
				io.dmaPlusCommand(v)
			}
		}
	}
}

// dmaPlusCommand starts one of DMA+'s commands, which are on port 0x6B only.
func (io *SpectrumIO) dmaPlusCommand(v uint8) {
	d := &io.dma
	switch v {
	case 0xD7:
		d.params = append(d.params, dmaAExt)
	case 0xDB:
		d.params = append(d.params, dmaBExt)
	case 0xDF:
		d.params = append(d.params, dmaFlags)
	case 0xE3:
		d.params = append(d.params, dmaRowLenLo, dmaRowLenHi)
	case 0xE7:
		d.params = append(d.params, dmaRowsLo, dmaRowsHi)
	case 0xEB:
		d.params = append(d.params, dmaAStrideLo, dmaAStrideHi)
	case 0xEF:
		d.params = append(d.params, dmaBStrideLo, dmaBStrideHi)
	case 0xF3:
		d.params = append(d.params, dmaTransparent)
	case 0xF7:
		d.params = append(d.params, dmaFill)
	}
}

// dmaLayer2XY works out port B's address from the Layer 2 position that came with 0xDF.
func (io *SpectrumIO) dmaLayer2XY() {
	d := &io.dma
	bank := io.nextRegs.regs[0x12]
	if d.flags&dmaHiddenLayer != 0 {
		bank = io.nextRegs.regs[0x13]
	}
	addr := nextPhysRAMBase + uint32(bank)*0x4000
	x, y := uint32(d.xy[0]), uint32(d.xy[2])
	if io.nextRegs.regs[0x70]>>4&0x03 == 0 {
		addr += y<<8 + x // row by row; the high byte of X is not used
	} else {
		addr += (uint32(d.xy[1]&0x01)<<8|x)<<8 + y // column by column
	}
	d.bStart, d.bExt = uint16(addr), uint8(addr>>16)
	d.bStride = 256
}

// load is the command 0xCF: the start addresses, with their extended bytes, become the
// working ones and the count starts again.
func (d *nextDMA) load() {
	d.aAddr = uint32(d.aExt)<<16 | uint32(d.aStart)
	d.bAddr = uint32(d.bExt)<<16 | uint32(d.bStart)
	d.aRow, d.bRow = d.aAddr, d.bAddr
	d.count, d.rowPos = 0, 0
}

// dmaAdvance moves a working address on by n. A physical address is 24 bits and carries;
// an ordinary one goes round within its 64K.
func (d *nextDMA) dmaAdvance(addr uint32, n int) uint32 {
	if d.flags&dmaPhysical != 0 {
		return (addr + uint32(n)) & 0xFFFFFF
	}
	return addr&0xFF0000 | uint32(uint16(addr)+uint16(n))
}

// dmaReadSequence starts the read sequence the mask chooses.
func (io *SpectrumIO) dmaReadSequence() {
	d := &io.dma
	d.readSeq, d.readPos = d.readSeq[:0], 0
	for i := uint8(0); i < 7; i++ {
		if d.readMask&(1<<i) != 0 {
			d.readSeq = append(d.readSeq, i)
		}
	}
}

// dmaPortRead is a read of port 0x6B or 0x0B.
func (io *SpectrumIO) dmaPortRead() uint8 {
	d := &io.dma
	if len(d.readSeq) == 0 {
		io.dmaReadSequence()
	}
	if len(d.readSeq) == 0 {
		return 0
	}
	which := d.readSeq[d.readPos%len(d.readSeq)]
	d.readPos = (d.readPos + 1) % len(d.readSeq)
	switch which {
	case 0:
		s := uint8(0x1A)
		if !d.endOfBlock {
			s |= 0x20
		}
		if d.anyByte {
			s |= 0x01
		}
		return s
	case 1:
		return uint8(d.count)
	case 2:
		return uint8(d.count >> 8)
	case 3:
		return uint8(d.aAddr)
	case 4:
		return uint8(d.aAddr >> 8)
	case 5:
		return uint8(d.bAddr)
	}
	return uint8(d.bAddr >> 8)
}

// dmaBlock is how many bytes a block is: the length, and one more in the Z80 DMA's mode; a
// length of 0 is one byte in both. In 2D mode it is the rows times their length, and a 0
// in either counts as 1.
func (d *nextDMA) dmaBlock() int {
	if d.flags&dma2D != 0 {
		return d.dmaRowLen() * max(1, int(d.rows))
	}
	n := int(d.length)
	if d.z80Mode {
		n++
	}
	return max(1, n)
}

func (d *nextDMA) dmaRowLen() int { return max(1, int(d.rowLen)) }

// dmaByteCycles is how long one byte takes: a read and a write, or with fill the write alone.
func (d *nextDMA) dmaByteCycles() int {
	if d.flags&dmaFillMode != 0 {
		if d.aToB {
			return d.bCycles
		}
		return d.aCycles
	}
	return d.aCycles + d.bCycles
}

// dmaEnable starts the transfer: all of it now, or a byte at a time with a prescaler.
func (io *SpectrumIO) dmaEnable() {
	d := &io.dma
	d.enabled = true
	if d.prescaler != 0 {
		d.wait = 0
		return
	}
	for d.enabled && d.count < d.dmaBlock() {
		io.dmaByte()
		d.stolen += d.dmaByteCycles()
	}
	d.enabled = false
}

// dmaByte moves one byte and ends the block when it was the last.
func (io *SpectrumIO) dmaByte() {
	d := &io.dma
	d.busy = true
	physical := d.flags&dmaPhysical != 0
	get := func(isIO bool, addr uint32) uint8 {
		switch {
		case isIO:
			return io.ReadPort(uint16(addr))
		case physical:
			return io.memory.nextPhysRead(addr)
		}
		return io.memory.Read(uint16(addr))
	}
	put := func(isIO bool, addr uint32, v uint8) {
		switch {
		case isIO:
			io.WritePort(uint16(addr), v)
		case physical:
			io.memory.nextPhysWrite(addr, v)
		default:
			io.memory.Write(uint16(addr), v)
		}
	}
	srcIO, src, dstIO, dst := d.aIO, d.aAddr, d.bIO, d.bAddr
	if !d.aToB {
		srcIO, src, dstIO, dst = d.bIO, d.bAddr, d.aIO, d.aAddr
	}
	v := d.fill
	if d.flags&dmaFillMode == 0 {
		v = get(srcIO, src)
	}
	if d.flags&dmaTransp == 0 || v != d.transparent {
		put(dstIO, dst, v)
	}
	d.busy = false
	d.aAddr = d.dmaAdvance(d.aAddr, d.aStep)
	d.bAddr = d.dmaAdvance(d.bAddr, d.bStep)
	d.count++
	d.anyByte = true
	if d.flags&dma2D != 0 {
		if d.rowPos++; d.rowPos >= d.dmaRowLen() { // the next row
			d.rowPos = 0
			d.aRow = d.dmaAdvance(d.aRow, int(d.aStride))
			d.bRow = d.dmaAdvance(d.bRow, int(d.bStride))
			d.aAddr, d.bAddr = d.aRow, d.bRow
		}
	}
	if d.count >= d.dmaBlock() {
		d.endOfBlock = true
		if d.autoRestart && d.prescaler != 0 {
			d.load()
		} else {
			d.enabled = false
		}
	}
}

// dmaTick moves the bytes of a timed transfer that have come due in tstates of the 3.5 MHz
// clock. It is called after each instruction while such a transfer is running.
func (io *SpectrumIO) dmaTick(tstates int) {
	d := &io.dma
	d.wait -= tstates
	for d.enabled && d.prescaler != 0 && d.wait <= 0 {
		io.dmaByte()
		d.wait += 4 * int(d.prescaler)
	}
}
