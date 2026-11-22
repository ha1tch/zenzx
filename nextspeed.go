package zenzx

// The Next's CPU speed: NextREG 0x07.
//
//	bits 1-0 (W)  00 = 3.5 MHz, 01 = 7 MHz, 10 = 14 MHz, 11 = 28 MHz (soft reset 00)
//	bits 5-4 (R)  the speed the CPU is running at; bits 1-0 the speed asked for
//
// The video does not change with the CPU's speed: a frame is as long, a line is as long,
// and the copper, the sound and the tape go on at their own rates. So everything outside
// the CPU stays in 3.5 MHz T-states, and a faster CPU is one whose instructions take a
// half, a quarter or an eighth of the T-states the core counts for them. stepCPU does that
// after each instruction, carrying the fraction, and puts the core's own count back in
// step, since the sound, the border and the beam's position are all worked out from it.
//
// NextZXOS turns the speed up for its own work (the Browser, the editor, loading) and back
// to what the user chose for the program that runs, and programs that time themselves
// against the beam ask for 28 MHz to get more done in a line.
//
// Left out: memory contention, which the machine has only at 3.5 MHz, is charged at any
// speed and divided with the rest.

// nextCPUSpeed is the speed asked for and the T-states owed.
type nextCPUSpeed struct {
	shift uint8 // 0 to 3: the CPU runs 1<<shift times 3.5 MHz
	frac  int   // core cycles not yet a whole T-state
}

// nextSpeedRegWrite is a write to NextREG 0x07.
func (io *SpectrumIO) nextSpeedRegWrite(value uint8) {
	if io.memory == nil || !io.memory.isNext {
		return
	}
	if shift := value & 0x03; shift != io.nextSpeed.shift {
		io.nextSpeed = nextCPUSpeed{shift: shift}
	}
}

// nextSpeedRegRead is a read of NextREG 0x07.
func (io *SpectrumIO) nextSpeedRegRead() uint8 {
	return io.nextSpeed.shift<<4 | io.nextSpeed.shift
}

// stepCPU runs one instruction and returns the 3.5 MHz T-states it took.
func (zx *ZenZX) stepCPU() int {
	if zx.io.mf.request {
		return zx.multifaceNMI() // nextmultiface.go
	}
	if zx.io.mf.active {
		zx.io.mf.retn = zx.multifaceAtRETN()
	}
	im2 := zx.cpu.IM == 2 && zx.cpu.INT && zx.cpu.IFF1
	cycles := zx.cpu.Step()
	if im2 && cycles == 19 && !zx.cpu.IFF1 {
		zx.im2Accepted() // im2vector.go
	}
	if zx.io.mf.retn && cycles == 14 { // 14: the RETN was executed, and not an interrupt taken in its place
		zx.multifaceRETN()
	}
	d := &zx.io.dma
	if d.stolen != 0 { // a DMA transfer held the CPU for these cycles: nextdma.go
		cycles += d.stolen
		zx.cpu.Cycles += uint64(d.stolen)
		d.stolen = 0
	}
	s := &zx.io.nextSpeed
	if s.shift != 0 {
		s.frac += cycles
		scaled := s.frac >> s.shift
		s.frac &= 1<<s.shift - 1
		zx.cpu.Cycles -= uint64(cycles - scaled)
		cycles = scaled
	}
	if d.enabled && d.prescaler != 0 {
		zx.io.dmaTick(cycles) // a timed transfer goes by the 3.5 MHz clock
	}
	if n := &zx.io.nextInt; n.ctcOn || n.hw { // the CTC and the Next's interrupt mode: nextint.go
		if n.ctcOn {
			zx.io.ctcTick(cycles * 8)
		}
		if n.hw {
			zx.cpu.INT = n.pending != 0
		}
	}
	return cycles
}
