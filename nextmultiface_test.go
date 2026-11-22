//go:build headless

package zenzx

import "testing"

// mfMachine is a Next out of config mode with a small Multiface ROM: at 0x66 it writes a
// mark into the Multiface's RAM, pages itself out and in again around a write to the
// same address, reads the saved 0x7FFD, hides itself and returns.
func mfMachine(t *testing.T) *ZenZX {
	t.Helper()
	zx := lineIntMachine(t)
	rom := zx.memory.nextConfigScratch(nextCfgBankScratch)
	copy(rom[0x66:], []uint8{
		0x3E, 0x55, // ld a,0x55
		0x32, 0x00, 0x20, // ld (0x2000),a    the Multiface's RAM
		0xDB, 0xBF, // in a,(0xBF)      paged out; the next opcode comes from the copy below
	})
	copy(rom[0x70:], []uint8{
		0x01, 0x3F, 0x7F, // ld bc,0x7F3F
		0xED, 0x78, // in a,(c)         the last 0x7FFD
		0x32, 0x01, 0x20, // ld (0x2001),a
		0xD3, 0x3F, // out (0x3F),a     hidden
		0xDB, 0x3F, // in a,(0x3F)      does nothing now
		0xED, 0x45, // retn
	})
	return zx
}

func mfPoke(zx *ZenZX, addr uint16, bytes ...uint8) {
	for i, b := range bytes {
		zx.memory.Write(addr+uint16(i), b)
	}
}

func TestMultifaceStacklessNMI(t *testing.T) {
	zx := mfMachine(t)
	// Under the Multiface, in RAM mapped at the bottom: the code that runs while it is paged out.
	zx.io.nextRegWriteDirect(0x50, 0x20)
	zx.io.nextRegWriteDirect(0x51, 0x21)
	mfPoke(zx, 0x006D,
		0x3E, 0xAA, // ld a,0xAA
		0xDB, 0x3F, // in a,(0x3F)      paged in again; execution goes on in the ROM at 0x71
	)
	mfPoke(zx, 0x2000, 0x11, 0x22)
	mfPoke(zx, 0x8000, 0x00, 0x00, 0x00)
	mfPoke(zx, 0x7FFE, 0xEE, 0xEE)
	zx.io.WritePort(0x7FFD, 0x03)
	zx.cpu.PC, zx.cpu.SP = 0x8000, 0x8000
	zx.cpu.IFF1, zx.cpu.IFF2 = true, true
	zx.io.nextRegWriteDirect(0xC0, 0x08) // stackless
	zx.io.nextRegWriteDirect(0x02, 0x08)
	if !zx.io.mf.request {
		t.Fatal("NextREG 0x02 bit 3 asked for no NMI")
	}
	zx.stepCPU()
	if zx.cpu.PC != 0x66 || zx.cpu.SP != 0x7FFE || zx.cpu.IFF1 || !zx.cpu.IFF2 || !zx.memory.mfPaged {
		t.Fatalf("after the NMI: PC %04X SP %04X IFF1 %v IFF2 %v paged %v", zx.cpu.PC, zx.cpu.SP, zx.cpu.IFF1, zx.cpu.IFF2, zx.memory.mfPaged)
	}
	if lo, hi := zx.io.nextRegs.regs[0xC2], zx.io.nextRegs.regs[0xC3]; lo != 0x00 || hi != 0x80 {
		t.Fatalf("return address in NextREG 0xC2/0xC3: %02X%02X, want 8000", hi, lo)
	}
	zx.stepCPU() // ld a
	zx.stepCPU() // ld (0x2000),a
	zx.stepCPU() // in a,(0xBF)
	if zx.memory.mfPaged {
		t.Fatal("reading the disable port left the Multiface paged in")
	}
	if got := zx.memory.Read(0x2000); got != 0x11 {
		t.Fatalf("0x2000 with the Multiface paged out: %02X, want the RAM under it (11)", got)
	}
	zx.stepCPU() // ld a,0xAA, from the RAM under the Multiface
	zx.stepCPU() // in a,(0x3F)
	if !zx.memory.mfPaged {
		t.Fatal("reading the enable port did not page the Multiface in")
	}
	if got := zx.memory.Read(0x2000); got != 0x55 {
		t.Fatalf("the Multiface's RAM at 0x2000: %02X, want 55", got)
	}
	zx.cpu.PC = 0x70
	for i := 0; i < 5; i++ { // ld bc; in a,(c); ld (0x2001),a; out (0x3F),a; in a,(0x3F)
		zx.stepCPU()
	}
	if got := zx.memory.Read(0x2001); got != 0x03 {
		t.Fatalf("port 0x7F3F gave %02X, want the last 0x7FFD (03)", got)
	}
	if zx.io.mf.visible {
		t.Fatal("a write to the Multiface's port did not hide it")
	}
	zx.stepCPU() // retn
	if zx.cpu.PC != 0x8000 || zx.cpu.SP != 0x8000 || !zx.cpu.IFF1 || zx.memory.mfPaged || zx.io.mf.active {
		t.Fatalf("after RETN: PC %04X SP %04X IFF1 %v paged %v active %v", zx.cpu.PC, zx.cpu.SP, zx.cpu.IFF1, zx.memory.mfPaged, zx.io.mf.active)
	}
	if a, b := zx.memory.Read(0x7FFE), zx.memory.Read(0x7FFF); a != 0xEE || b != 0xEE {
		t.Fatalf("a stackless NMI wrote to the stack: %02X %02X", a, b)
	}
	if _, ok := zx.io.mfPortRead(0x7F3F); ok || zx.memory.mfPaged {
		t.Fatal("the enable port works outside an NMI")
	}
}

func TestMultifaceNMIOnTheStack(t *testing.T) {
	zx := mfMachine(t)
	copy(zx.memory.nextConfigScratch(nextCfgBankScratch)[0x66:], []uint8{0xED, 0x45}) // retn
	zx.cpu.PC, zx.cpu.SP = 0x8123, 0x9000
	zx.io.nextRegWriteDirect(0xC0, 0x00)
	zx.io.nextRegWriteDirect(0x02, 0x08)
	zx.stepCPU()
	if lo, hi := zx.memory.Read(0x8FFE), zx.memory.Read(0x8FFF); lo != 0x23 || hi != 0x81 || zx.cpu.SP != 0x8FFE {
		t.Fatalf("return address on the stack: %02X%02X at SP %04X", hi, lo, zx.cpu.SP)
	}
	zx.io.nextRegWriteDirect(0x02, 0x08) // a second one while the first is served: ignored
	if zx.io.mf.request {
		t.Fatal("an NMI was accepted while one was being served")
	}
	zx.stepCPU()
	if zx.cpu.PC != 0x8123 || zx.cpu.SP != 0x9000 || zx.io.mf.active {
		t.Fatalf("after RETN: PC %04X SP %04X active %v", zx.cpu.PC, zx.cpu.SP, zx.io.mf.active)
	}
}

// The M1 button makes an NMI only if NextREG 0x06 bit 3 allows it, and a Next that has not
// loaded a Multiface ROM makes none.
func TestMultifaceButtonAndNoROM(t *testing.T) {
	zx := mfMachine(t)
	zx.io.nextRegWriteDirect(0x06, 0x00)
	zx.io.MultifaceButton()
	if zx.io.mf.request {
		t.Fatal("the button made an NMI with NextREG 0x06 bit 3 clear")
	}
	zx.io.nextRegWriteDirect(0x06, 0x08)
	zx.io.MultifaceButton()
	if !zx.io.mf.request {
		t.Fatal("the button made no NMI with NextREG 0x06 bit 3 set")
	}
	bare := lineIntMachine(t)
	bare.io.nextRegWriteDirect(0x02, 0x08)
	if bare.io.mf.request {
		t.Fatal("an NMI was accepted with no Multiface ROM loaded")
	}
}

// The handler's address in IM 2 comes from I*256 and the whole of the vector byte: 0xFF
// from a Spectrum's ULA, so its two bytes are the last of one page and the first of the next.
func TestIM2UsesTheWholeVector(t *testing.T) {
	zx := lineIntMachine(t)
	for a := 0xFE00; a <= 0xFF01; a++ {
		zx.memory.Write(uint16(a), 0x00)
	}
	mfPoke(zx, 0xFEFF, 0x34, 0x92) // the handler is at 0x9234
	mfPoke(zx, 0x8000, 0x00, 0x00)
	zx.cpu.PC, zx.cpu.SP, zx.cpu.I, zx.cpu.IM = 0x8000, 0x9000, 0xFE, 2
	zx.cpu.IFF1, zx.cpu.IFF2 = true, true
	zx.cpu.INT = true
	zx.stepCPU()
	if zx.cpu.PC != 0x9234 {
		t.Fatalf("IM 2 with vector 0xFF and I 0xFE went to %04X, want 9234", zx.cpu.PC)
	}
}
