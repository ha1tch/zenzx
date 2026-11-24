//go:build !headless

package zenzx

import (
	"strings"
	"testing"
)

// The Diagnostics report says what the machine is and how it stands, in text the reader's font
// can draw, and reading it changes nothing in the machine.
func TestDiagnosticsText(t *testing.T) {
	zx := testZXWithDrive()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatal(err)
	}
	zx.cpu.PC, zx.cpu.SP = 0x1219, 0xFF00
	pc, sp, cycles, r, frames := zx.cpu.PC, zx.cpu.SP, zx.cpu.Cycles, zx.cpu.R, zx.frameCount

	text := diagnosticsText(zx, "48k")
	fitsThePanel(t, "diagnostics", text)
	for _, want := range []string{
		"## Machine", "Model           48K", "## CPU", "PC 1219", "SP FF00", "## Memory", "48K: one ROM",
		"RAM-CHECK", "## Stack", "## Tape", "No tape is loaded.", "## Checks", "Byte order      ok",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the report lacks %q:\n%s", want, text)
		}
	}
	for i, r := range text {
		if r > 0x7E || (r < 0x20 && r != '\n') {
			t.Fatalf("the report has a character the font cannot draw (U+%04X at %d)", r, i)
		}
	}
	if zx.cpu.PC != pc || zx.cpu.SP != sp || zx.cpu.Cycles != cycles || zx.cpu.R != r || zx.frameCount != frames {
		t.Error("the report changed the machine")
	}

	// A fault shows in the checks.
	zx.cpu.SP = 0x0100
	if text := diagnosticsText(zx, "48k"); !strings.Contains(text, "CHECK: SP points into the ROM") {
		t.Errorf("a stack pointer in the ROM is not reported:\n%s", text)
	}
	// A drive and a card slot have their sections where the machine has them.
	if text := diagnosticsText(zx, "plus3"); !strings.Contains(text, "## Floppy disk") {
		t.Errorf("a machine with a drive has no section for it:\n%s", text)
	}
}

// The nearest routine is the greatest entry at or below the address, the cold start included.
func TestNearestROMRoutine(t *testing.T) {
	for _, c := range []struct {
		pc   uint16
		addr uint16
		want string
	}{
		{0x0000, 0x0000, "RESET"}, {0x0005, 0x0000, "RESET"}, {0x0038, 0x0038, "MASK-INT"},
		{0x0100, 0x0066, "NMI"}, {0x3FFF, 0x1219, "RAM-CHECK"},
	} {
		addr, name := nearestROMRoutine(c.pc)
		if addr != c.addr || !strings.HasPrefix(name, c.want) {
			t.Errorf("%04X: %04X %q, want %04X %s", c.pc, addr, name, c.addr, c.want)
		}
	}
}
