package main

import (
	"bytes"
	"testing"
)

// newNextBootTestMachine builds the memory and IO of a Next the way -model=next with
// -bootrom, -divmmc-rom and -sdcard does: +3 base, Next MMU, a boot ROM (so the machine
// powers on in config mode) and a DivMMC with an empty card.
func newNextBootTestMachine(t *testing.T) (*SpectrumMemory, *SpectrumIO) {
	t.Helper()
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	mem.SetNextSlot(0, 0xFF)
	mem.SetNextSlot(1, 0xFF)
	io := NewSpectrumIO(mem, nil)
	boot := make([]byte, 8192)
	for i := range boot {
		boot[i] = 0xB0
	}
	mem.SetBootROM(boot)
	io.EnableDivMMC(bytes.NewReader(make([]byte, 512)), 512, make([]byte, 8192))
	return mem, io
}

func TestNextConfigModeLoadsROMThroughTheWindow(t *testing.T) {
	mem, io := newNextBootTestMachine(t)
	if !mem.nextCfg.active {
		t.Fatal("a Next with a boot ROM must power on in config mode")
	}
	if got := mem.Read(0x0123); got != 0xB0 {
		t.Fatalf("boot ROM must cover the window while enabled, read 0x%02X", got)
	}

	// The boot ROM's own write: machine-type bits 000 turn the boot ROM off and change
	// nothing else.
	io.nextRegWriteDirect(0x03, 0x00)
	if mem.bootROMEnabled || !mem.nextCfg.active {
		t.Fatalf("after NR 0x03 <- 0x00: bootROMEnabled=%v configMode=%v, want false and true", mem.bootROMEnabled, mem.nextCfg.active)
	}

	// Each kind of SRAM bank, written at address 0x1234 or 0x2234 through the window.
	io.nextRegWriteDirect(0x04, 0x02) // ROM bank 2
	mem.Write(0x1234, 0xA2)
	io.nextRegWriteDirect(0x04, 0x04) // DivMMC ROM
	mem.Write(0x1234, 0xA4)
	io.nextRegWriteDirect(0x04, 0x06) // alternate ROM 0
	mem.Write(0x1234, 0xA6)
	io.nextRegWriteDirect(0x04, 0x09) // DivMMC RAM pages 2 and 3
	mem.Write(0x2234, 0xA9)
	io.nextRegWriteDirect(0x04, 0x15) // RAM bank 5: pages 10 and 11, the screen
	mem.Write(0x0000, 0x81)
	mem.Write(0x2234, 0xB5)
	io.nextRegWriteDirect(0x04, 0x20) // RAM bank 16: pages 32 and 33
	mem.Write(0x2234, 0xC0)

	if got := mem.rom[2][0x1234]; got != 0xA2 {
		t.Errorf("ROM bank 2 = 0x%02X, want 0xA2", got)
	}
	if got := io.divMMC.bootROM[0x1234]; got != 0xA4 {
		t.Errorf("DivMMC ROM = 0x%02X, want 0xA4", got)
	}
	if got := io.divMMC.sram[3][0x0234]; got != 0xA9 {
		t.Errorf("DivMMC RAM page 3 = 0x%02X, want 0xA9", got)
	}
	if got := mem.nextPageRead(11, 0x0234); got != 0xB5 {
		t.Errorf("page 11 = 0x%02X, want 0xB5", got)
	}
	if got := mem.screen.bitmap[0]; got != 0x81 {
		t.Errorf("a write to page 10 through the window must reach the renderer's screen, got 0x%02X", got)
	}
	if got := mem.nextPageRead(33, 0x0234); got != 0xC0 {
		t.Errorf("page 33 = 0x%02X, want 0xC0", got)
	}
	io.nextRegWriteDirect(0x04, 0x06)
	if got := mem.Read(0x1234); got != 0xA6 {
		t.Errorf("alternate ROM bank reads back 0x%02X, want 0xA6", got)
	}

	// Choosing the machine leaves config mode; the window closes and the loaded ROM answers.
	io.nextRegWriteDirect(0x04, 0x02)
	io.nextRegWriteDirect(0x03, 0xB3)
	if mem.nextCfg.active || mem.nextCfg.machineType != 3 {
		t.Fatalf("after NR 0x03 <- 0xB3: configMode=%v machineType=%d, want false and 3", mem.nextCfg.active, mem.nextCfg.machineType)
	}
	mem.romBank = 2
	if got := mem.Read(0x1234); got != 0xA2 {
		t.Errorf("ROM 2 after config mode = 0x%02X, want the byte the firmware loaded (0xA2)", got)
	}
	mem.Write(0x1234, 0x00)
	if got := mem.Read(0x1234); got != 0xA2 {
		t.Errorf("ROM must be read-only outside config mode, got 0x%02X", got)
	}

	// The machine type is only accepted in config mode; 111 enters it again.
	io.nextRegWriteDirect(0x03, 0x01)
	if mem.nextCfg.machineType != 3 {
		t.Errorf("machine type changed outside config mode to %d", mem.nextCfg.machineType)
	}
	io.nextRegWriteDirect(0x03, 0x07)
	if !mem.nextCfg.active {
		t.Error("NR 0x03 bits 2-0 = 111 must enter config mode")
	}
}

func TestNextResetRequests(t *testing.T) {
	_, io := newNextBootTestMachine(t)
	io.nextRegWriteDirect(0x02, 0x00)
	if io.nextResetRequest != 0 {
		t.Errorf("NR 0x02 <- 0 asked for reset %d", io.nextResetRequest)
	}
	io.nextRegWriteDirect(0x02, 0x01)
	if io.nextResetRequest != nextResetSoft {
		t.Errorf("NR 0x02 <- 1: request %d, want soft", io.nextResetRequest)
	}
	io.nextRegWriteDirect(0x02, 0x03)
	if io.nextResetRequest != nextResetHard {
		t.Errorf("NR 0x02 <- 3: request %d, want hard", io.nextResetRequest)
	}
}

func TestNextReg8ESwitchesROMAtOnce(t *testing.T) {
	mem, io := newNextBootTestMachine(t)
	io.nextRegWriteDirect(0x03, 0xB3) // out of config mode, boot ROM off
	for b := 0; b < 4; b++ {
		mem.rom[b][0x3E17] = uint8(0xC0 + b)
	}
	io.nextRegWriteDirect(0x57, 0x21) // a slot the register must leave alone when bit 3 is 0

	for rom := uint8(0); rom < 4; rom++ {
		io.nextRegWriteDirect(0x8E, rom) // bit 3 clear: ROM only
		if got := mem.Read(0x3E17); got != 0xC0+rom {
			t.Errorf("NR 0x8E <- 0x%02X: ROM read 0x%02X, want 0x%02X", rom, got, 0xC0+rom)
		}
		if mem.nextSlots[7] != 0x21 {
			t.Errorf("NR 0x8E <- 0x%02X with bit 3 clear changed slot 7 to %d", rom, mem.nextSlots[7])
		}
		if got := io.nextReg8ERead() & 0x03; got != rom {
			t.Errorf("NR 0x8E reads back ROM %d, want %d", got, rom)
		}
	}

	io.nextRegWriteDirect(0x8E, 0x38) // bit 3 set: RAM bank 3 at the top
	if mem.nextSlots[6] != 6 || mem.nextSlots[7] != 7 {
		t.Errorf("NR 0x8E <- 0x38: slots 6,7 = %d,%d, want 6,7", mem.nextSlots[6], mem.nextSlots[7])
	}
}

func TestNextMMURegistersReadBackTheLiveMapping(t *testing.T) {
	mem, io := newNextBootTestMachine(t)
	io.nextRegWriteDirect(0x57, 223)
	mem.SetNextLegacyBank7FFD(0x00) // what a write to port 0x7FFD does to slots 6 and 7
	io.nextRegs.selected = 0x57
	if got := io.nextRegReadSelected(); got != 1 {
		t.Errorf("NR 0x57 reads %d after port 0x7FFD mapped bank 0, want 1", got)
	}
	io.nextRegs.selected = 0x56
	if got := io.nextRegReadSelected(); got != 0 {
		t.Errorf("NR 0x56 reads %d, want 0", got)
	}
}

func TestNextVideoLineRegisters(t *testing.T) {
	_, io := newNextBootTestMachine(t)
	line := 0
	io.nextVideoLineFn = func() int { return line }
	for _, c := range []struct{ line, hi, lo int }{{0, 0, 0}, {191, 0, 191}, {255, 0, 255}, {256, 1, 0}, {310, 1, 54}} {
		line = c.line
		io.nextRegs.selected = 0x1E
		hi := io.nextRegReadSelected()
		io.nextRegs.selected = 0x1F
		lo := io.nextRegReadSelected()
		if int(hi) != c.hi || int(lo) != c.lo {
			t.Errorf("line %d: NR 0x1E,0x1F = %d,%d, want %d,%d", c.line, hi, lo, c.hi, c.lo)
		}
	}
}
