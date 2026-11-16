package main

import (
	"testing"
)

// The hardware NextZXOS needs after the firmware has handed over: the DivMMC's programmable
// entry points, the alternate ROM and ROM lock, and a screen cache that follows RAM.

func newEntryTestDivMMC() (*DivMMC, *bool, *uint16) {
	d := NewDivMMC(make([]byte, 8192), nil)
	d.automapEnabled = true
	rom3 := false
	pc := uint16(0xFFFF)
	d.rom3Fn = func() bool { return rom3 }
	d.pcFn = func() uint16 { return pc }
	return d, &rom3, &pc
}

func TestDivMMCEntryPointsDelayedAndROM3(t *testing.T) {
	d, rom3, _ := newEntryTestDivMMC()

	// Reset values: 0x0000 maps whatever the ROM, and only after its own opcode.
	d.OnM1(0x0000, 0xF3, "normal")
	if d.paged {
		t.Fatal("0x0000 is a delayed entry point: the DivMMC must not be mapped during the fetch")
	}
	d.MemRead(0x0001)
	if !d.paged {
		t.Fatal("0x0000 must map the DivMMC from the next memory access on")
	}

	// 0x1FF8-0x1FFF unmaps, also after the opcode there.
	d.OnM1(0x1FF9, 0xC9, "normal")
	if !d.paged {
		t.Fatal("the opcode at 0x1FF9 is the DivMMC's own: still mapped during its fetch")
	}
	d.MemRead(0xFF00)
	if d.paged {
		t.Fatal("0x1FF9 must unmap the DivMMC after its opcode")
	}

	// 0x0038 counts only with ROM 3.
	d.OnM1(0x0038, 0xF5, "normal")
	d.MemRead(0x0039)
	if d.paged {
		t.Fatal("0x0038 mapped the DivMMC without ROM 3")
	}
	*rom3 = true
	d.OnM1(0x0038, 0xF5, "normal")
	d.MemRead(0x0039)
	if !d.paged {
		t.Fatal("0x0038 must map the DivMMC when ROM 3 is present")
	}
}

func TestDivMMCIgnoresInterruptAcknowledge(t *testing.T) {
	d, rom3, _ := newEntryTestDivMMC()
	*rom3 = true
	// zen80 reports the acknowledge of an IM 1 interrupt as an M1 at 0x0038. The stack
	// write that follows must not find the DivMMC mapped: the opcode at 0x0038 has not
	// been fetched yet, and it has to come from the ROM.
	d.OnM1(0x0038, 0xFF, "IM1")
	d.MemWrite(0xFF40, 0x12)
	if d.paged {
		t.Fatal("an interrupt acknowledge at 0x0038 mapped the DivMMC before the opcode there was fetched")
	}
	d.OnM1(0x0066, 0x00, "NMI")
	d.MemRead(0x0067)
	if d.paged {
		t.Fatal("an NMI acknowledge mapped the DivMMC; 0x0066 needs the button")
	}
}

func TestDivMMCInstantEntryAt3Dxx(t *testing.T) {
	d, rom3, pc := newEntryTestDivMMC()
	*rom3 = true
	d.sram[0][0x1D00] = 0xC9

	// A data read of 0x3D00 (the 48K ROM's character set) from code elsewhere is no entry.
	*pc = 0x0B65
	if _, ok := d.MemRead(0x3D00); ok || d.paged {
		t.Fatal("a data read at 0x3D00 mapped the DivMMC")
	}
	// The opcode fetch at 0x3D00 is: the opcode itself already comes from the DivMMC.
	*pc = 0x3D00
	v, ok := d.MemRead(0x3D00)
	if !ok || v != 0xC9 || !d.paged {
		t.Fatalf("opcode fetch at 0x3D00: ok=%v value=0x%02X paged=%v, want the DivMMC's 0xC9", ok, v, d.paged)
	}
	// Not without ROM 3.
	d.paged = false
	*rom3 = false
	if _, ok := d.MemRead(0x3D00); ok {
		t.Fatal("0x3D00 mapped the DivMMC without ROM 3")
	}
}

func TestDivMMCEntryRegistersAndGate(t *testing.T) {
	d, _, _ := newEntryTestDivMMC()
	d.setEntryReg(0xB8, 0x82) // NextZXOS's value: 0x0038 and 0x0008, no longer 0x0000
	d.OnM1(0x0000, 0xF3, "normal")
	d.MemRead(0x0001)
	if d.paged {
		t.Fatal("0x0000 mapped the DivMMC after NextREG 0xB8 turned it off")
	}
	d.resetEntryPoints()
	d.automapEnabled = false // NextREG 0x0A bit 4
	d.OnM1(0x0000, 0xF3, "normal")
	d.MemRead(0x0001)
	if d.paged {
		t.Fatal("an entry point mapped the DivMMC with automap disabled")
	}
	d.setEntryReg(0xBB, 0x8D) // bit 6 clear: 0x1FF8 no longer unmaps
	d.paged = true
	d.OnM1(0x1FF8, 0x00, "normal")
	d.MemRead(0x1FF9)
	if !d.paged {
		t.Fatal("0x1FF8 unmapped the DivMMC after NextREG 0xBB bit 6 was cleared")
	}

	d.ctrl = 0x41
	d.clearMapRAM()
	if d.ctrl != 0x01 {
		t.Fatalf("clearMapRAM left control at 0x%02X, want 0x01", d.ctrl)
	}
	if d.ReadControl() != 0xFF {
		t.Fatal("port 0xE3 must read 0xFF on the original interface")
	}
	d.controlReadable = true
	if d.ReadControl() != 0x01 {
		t.Fatal("port 0xE3 must read back on the Next")
	}
}

func TestNextAlternateROMAndLock(t *testing.T) {
	mem, io := newNextBootTestMachine(t)
	io.nextRegWriteDirect(0x03, 0xB3) // out of config mode
	for b := 0; b < 4; b++ {
		mem.rom[b][0x0100] = uint8(0xA0 + b)
	}

	io.nextRegWriteDirect(0x8C, 0xC0) // enabled, for writes only
	mem.Write(0x0100, 0x77)
	if got := mem.Read(0x0100); got != 0xA0 {
		t.Fatalf("with the alternate ROM write-only, reads must still see the ROM: 0x%02X", got)
	}
	io.nextRegWriteDirect(0x8C, 0x80) // replaces the ROM for reads
	if got := mem.Read(0x0100); got != 0x77 {
		t.Fatalf("alternate ROM 0 reads 0x%02X, want the 0x77 written through the ROM's address", got)
	}
	mem.Write(0x0100, 0x00)
	if got := mem.Read(0x0100); got != 0x77 {
		t.Fatal("the alternate ROM took a write while enabled for reads")
	}
	// Port 0x7FFD bit 4 chooses alternate ROM 1, which is still empty.
	mem.SetPaging(0x10)
	if got := mem.Read(0x0100); got != 0x00 {
		t.Fatalf("alternate ROM 1 reads 0x%02X, want 0x00", got)
	}
	mem.SetPaging(0x00)

	// The lock bits hold a ROM in place whatever the ports say.
	io.nextRegWriteDirect(0x8C, 0x30)
	if got := mem.Read(0x0100); got != 0xA3 {
		t.Fatalf("with ROM 3 locked the read is 0x%02X, want 0xA3", got)
	}
	if !io.divMMC.rom3() {
		t.Fatal("a locked ROM 3 must count as ROM 3 for the DivMMC's entry points")
	}
	io.nextRegWriteDirect(0x8C, 0x00)
	if io.divMMC.rom3() {
		t.Fatal("ROM 0 counted as ROM 3")
	}
}

func TestNextSoftResetCarriesAlternateROMBits(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	zx.memory.Enable128K()
	zx.memory.EnablePlus3()
	zx.memory.EnableNext()
	zx.memory.nextAltROM = 0xC5
	zx.io.nextResetRequest = nextResetSoft
	zx.serviceNextReset()
	if zx.memory.nextAltROM != 0x55 {
		t.Fatalf("after a soft reset NextREG 0x8C is 0x%02X, want bits 3-0 copied up: 0x55", zx.memory.nextAltROM)
	}
	if got := zx.io.nextRegs.regs[0x02]; got != nextResetSoft {
		t.Fatalf("NextREG 0x02 reads 0x%02X after a soft reset, want 0x01", got)
	}
	zx.io.nextResetRequest = nextResetHard
	zx.serviceNextReset()
	if zx.memory.nextAltROM != 0 || zx.io.nextRegs.regs[0x02] != nextResetHard {
		t.Fatalf("after a hard reset: NR 0x8C=0x%02X NR 0x02=0x%02X, want 0x00 and 0x02", zx.memory.nextAltROM, zx.io.nextRegs.regs[0x02])
	}
}

func TestNextScreenCacheFollowsRAM(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	mem.SetNextSlot(2, 10) // bank 5 at 0x4000
	mem.SetNextSlot(6, 14) // bank 7 at 0xC000

	mem.Write(0x4000, 0x11) // the displayed bank
	mem.Write(0xC000, 0x22) // the other one
	if mem.screen.bitmap[0] != 0x11 {
		t.Fatalf("cache byte 0 = 0x%02X, want the write to bank 5 (0x11)", mem.screen.bitmap[0])
	}

	mem.SetPaging(0x08) // display bank 7
	if mem.screen.bitmap[0] != 0x22 {
		t.Fatalf("after switching to bank 7 the cache shows 0x%02X, want 0x22", mem.screen.bitmap[0])
	}
	if mem.ram[5][0] != 0x11 || mem.ram[7][0] != 0x22 {
		t.Fatalf("switching the displayed bank changed RAM: bank 5 0x%02X, bank 7 0x%02X", mem.ram[5][0], mem.ram[7][0])
	}
	mem.Write(0x4000, 0x33) // bank 5, no longer displayed
	if mem.screen.bitmap[0] != 0x22 {
		t.Fatal("a write to the bank that is not displayed reached the cache")
	}
	mem.SetPaging(0x00)
	if mem.screen.bitmap[0] != 0x33 || mem.ram[7][0] != 0x22 {
		t.Fatalf("back on bank 5: cache 0x%02X (want 0x33), bank 7 0x%02X (want 0x22)", mem.screen.bitmap[0], mem.ram[7][0])
	}
}
