package zenzx

// The Next's Multiface.
//
// A Multiface is a ROM and 8K of RAM of its own that take the place of the bottom 16K when
// the NMI button is pressed, so that a program can be stopped wherever it is. The Next has
// one built in. Its ROM (enNextMf.rom on the card) is put in SRAM bank 0x05 by the firmware
// (nextconfig.go), with the Multiface's RAM in the upper half of the same bank: the ROM is
// at 0x0000-0x1FFF and the RAM at 0x2000-0x3FFF.
//
// NextZXOS uses it for more than the NMI menu. A snapshot is started through it: NextZXOS
// loads the snapshot, then asks for a Multiface NMI itself (NextREG 0x02 bit 3) and the
// Multiface ROM puts the registers in place and leaves with RETN into the program. Without
// a Multiface that NMI goes to address 0x66 of whatever ROM is there, and the snapshot
// never starts.
//
// What is modelled, from the Next's registers.txt and ports.txt and from what the ROM does:
//
//	NextREG 0x02 bit 3   write 1: ask for a Multiface NMI. Ignored while one is being served.
//	NextREG 0x06 bit 3   the M1 button asks for one too (the front ends' NMI key).
//	NextREG 0x0A bits 7-6  the kind of Multiface, which decides its two ports:
//	                       00  +3            enable 0x3F, disable 0xBF
//	                       01  128 v87.2     enable 0xBF, disable 0x3F
//	                       10  128 v87.12    enable 0x9F, disable 0x1F
//	                       11  1             enable 0x9F, disable 0x1F
//	NextREG 0xC0 bit 3   stackless NMI: the return address is put in NextREG 0xC2/0xC3 and
//	                     not in memory (SP still goes down by two), and the RETN that ends
//	                     the NMI takes it from there (SP goes up by two). NextZXOS turns
//	                     this on: a program's stack may be in ROM, or nowhere.
//
// The NMI is taken at the next instruction. The ROM and RAM are paged in as the CPU goes to
// 0x66. Reading the disable port pages them out and reading the enable port pages them in
// again, while the NMI is being served and the Multiface has not been hidden. RETN pages
// them out and ends the NMI. Over the bottom 16K the Multiface comes before DivMMC, Layer
// 2's mapping and the MMU, and after the boot ROM.
//
// On the +3 kind, the enable port read with a high byte also gives back what the ports a
// snapshot has to save were last set to (by a write to them, or through NextREG 0x8E): 0x7F3F port 0x7FFD, 0x1F3F port 0x1FFD,
// 0xDF3F port 0xDFFD, 0xEF3F port 0xEFF7 and 0xFE3F the border colour. The first two are
// the +3 Multiface's own; the others are what the Next's ROM reads there. On the 128 kinds
// bit 7 of the enable port is the screen bit of 0x7FFD.
//
// A write to either port hides the Multiface until the next NMI: the enable port then
// neither pages it in nor gives the saved values. On the machine it is the write to one
// particular port of the two that hides it; the Next's ROM writes only as it leaves, where
// both readings give the same.

type nextMultiface struct {
	request   bool // an NMI has been asked for and not yet taken
	active    bool // an NMI is being served: from 0x66 to RETN
	visible   bool // the enable port works: from the NMI to a write to one of the ports
	stackless bool // the return address of this NMI is in NextREG 0xC2/0xC3
	retn      bool // the instruction being executed is a RETN

	lastEFF7 uint8 // the last write to port 0xEFF7; the other ports' are the memory's
}

// mfPorts gives the Multiface's enable and disable ports (the low byte) for the kind
// chosen in NextREG 0x0A.
func (io *SpectrumIO) mfPorts() (enable, disable uint8) {
	switch io.nextRegs.regs[0x0A] >> 6 {
	case 0:
		return 0x3F, 0xBF
	case 1:
		return 0xBF, 0x3F
	}
	return 0x9F, 0x1F
}

// mfRequestNMI asks for a Multiface NMI: NextREG 0x02 bit 3, or the M1 button.
func (io *SpectrumIO) mfRequestNMI() {
	m := io.memory
	if m == nil || !m.isNext || m.nextCfg.active || m.bootROMEnabled || m.nextCfg.scratch == nil {
		return // not a Next that has loaded its Multiface ROM
	}
	if io.mf.active {
		return
	}
	io.mf.request = true
}

// MultifaceButton is the M1 button, if NextREG 0x06 lets it make an NMI.
func (io *SpectrumIO) MultifaceButton() {
	if io.nextRegs.regs[0x06]&0x08 != 0 {
		io.mfRequestNMI()
	}
}

// mfReset is a reset of the machine.
func (io *SpectrumIO) mfReset() {
	io.mf = nextMultiface{}
	if io.memory != nil {
		io.memory.mfPaged = false
	}
}

// mfPortRead is a read of a port on a Next: the Multiface's two ports page it in and out.
// ok is false where the Multiface gives no value and the read is some other device's.
func (io *SpectrumIO) mfPortRead(port uint16) (value uint8, ok bool) {
	mf := &io.mf
	if !mf.active {
		return 0, false
	}
	enable, disable := io.mfPorts()
	switch uint8(port) {
	case disable:
		io.memory.mfPaged = false
	case enable:
		if !mf.visible {
			return 0, false
		}
		io.memory.mfPaged = true
		switch io.nextRegs.regs[0x0A] >> 6 {
		case 0:
			switch uint8(port >> 8) {
			case 0x7F:
				return io.memory.port7FFD, true
			case 0x1F:
				return io.memory.port1FFD, true
			case 0xDF:
				return io.memory.nextLegacyPortDFFD, true
			case 0xEF:
				return mf.lastEFF7, true
			case 0xFE:
				return io.borderColor & 0x07, true
			}
		case 1, 2:
			return io.memory.port7FFD&0x08<<4 | 0x7F, true
		}
	}
	return 0, false
}

// mfPortWrite sees every port write on a Next: it keeps what was last written to port
// 0xEFF7, and a write to one of the Multiface's own ports hides it.
func (io *SpectrumIO) mfPortWrite(port uint16, value uint8) {
	mf := &io.mf
	if port == 0xEFF7 {
		mf.lastEFF7 = value
	}
	if mf.active {
		if enable, disable := io.mfPorts(); uint8(port) == enable || uint8(port) == disable {
			mf.visible = false
		}
	}
}

// multifaceAtRETN reports whether the instruction the CPU is about to execute is a RETN,
// which ends the NMI. (ED 4D is RETI, which does not.) It is asked only while an NMI is
// being served.
func (zx *ZenZX) multifaceAtRETN() bool {
	peek := func(a uint16) uint8 {
		if zx.memory.mfPaged {
			return zx.memory.nextCfg.scratch[1][a&0x3FFF]
		}
		return zx.memory.Read(a)
	}
	pc := zx.cpu.PC
	if zx.memory.mfPaged && pc >= 0x4000 || peek(pc) != 0xED {
		return false
	}
	op := peek(pc + 1)
	return op&0xC7 == 0x45 && op != 0x4D
}

// multifaceNMI takes a Multiface NMI that has been asked for, in place of the CPU's next
// instruction, and returns its length in T-states.
func (zx *ZenZX) multifaceNMI() int {
	io, cpu := zx.io, zx.cpu
	mf := &io.mf
	mf.request, mf.active, mf.visible = false, true, true
	cpu.Halted = false
	cpu.IFF1 = false
	cpu.SP -= 2
	if mf.stackless = io.nextRegs.regs[0xC0]&0x08 != 0; mf.stackless {
		io.nextRegs.regs[0xC2], io.nextRegs.regs[0xC3] = uint8(cpu.PC), uint8(cpu.PC>>8)
	} else {
		zx.memory.Write(cpu.SP+1, uint8(cpu.PC>>8))
		zx.memory.Write(cpu.SP, uint8(cpu.PC))
	}
	cpu.PC = 0x0066
	cpu.WZ = cpu.PC
	cpu.R = cpu.R&0x80 | (cpu.R+1)&0x7F
	zx.memory.mfPaged = true
	cpu.Cycles += 11
	return 11
}

// multifaceRETN is the end of a RETN executed while a Multiface NMI was being served.
func (zx *ZenZX) multifaceRETN() {
	io := zx.io
	mf := &io.mf
	if mf.stackless { // the CPU has taken two bytes from where SP was; they are not the address
		zx.cpu.PC = uint16(io.nextRegs.regs[0xC3])<<8 | uint16(io.nextRegs.regs[0xC2])
		zx.cpu.WZ = zx.cpu.PC
	}
	*mf = nextMultiface{lastEFF7: mf.lastEFF7}
	zx.memory.mfPaged = false
}
