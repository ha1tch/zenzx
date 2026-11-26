//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"strings"
	"testing"
)

// The decoding of the registers is the emulation's own, word by word.
func TestNextDiagnosticsWords(t *testing.T) {
	for i, want := range []string{"3.5 MHz", "7 MHz", "14 MHz", "28 MHz"} {
		if got := nextCPUSpeedWord(uint8(i)); got != want {
			t.Errorf("speed %d: %q, want %q", i, got, want)
		}
	}
	for nr, want := range map[uint8]string{1: "48K", 2: "128K", 3: "+3", 4: "Pentagon", 0: "0 (not a timing)", 7: "7 (not a timing)"} {
		if got := nextTimingWord(nr | 0xA8); got != want {
			t.Errorf("timing %d: %q, want %q", nr, got, want)
		}
	}
	for bits, want := range []string{"S L U", "L S U", "S U L", "L U S", "U S L", "U L S"} {
		if got := nextLayerOrderWord(uint8(bits) << 2); got != want+", top first" {
			t.Errorf("layer order %d: %q", bits, got)
		}
	}
	if got := nextLayerOrderWord(6<<2 | 0x03); !strings.Contains(got, "blending") {
		t.Errorf("layer order 6: %q", got)
	}
	for mode, want := range map[uint8]string{0: "standard", 1: "second screen", 2: "hi-colour", 6: "hi-res", 3: "3 (not a mode)"} {
		if got := nextTimexWord(mode | 0x38); got != want {
			t.Errorf("timex %d: %q, want %q", mode, got, want)
		}
	}
	if !strings.Contains(nextCopperWord(3), "every frame") || nextCopperWord(0) != "stopped" {
		t.Error("the copper words are wrong")
	}
}

// A Next's report has its three sections, they show what the machine has been set to, they fit
// the panel and the font, and reading them changes nothing.
func TestDiagnosticsOfANextHasItsHardware(t *testing.T) {
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Skip(err)
	}
	m, io := zx.memory, zx.io
	m.SetNextSlot(4, 0x0A)
	m.SetNextSlot(5, 0x0B)
	io.nextSpeed.shift = 3
	io.nextRegs.regs[0x03] = 0x03 | 0x80
	io.nextRegs.regs[0x15] = 3<<2 | 0x03
	io.nextRegs.regs[0x22] = 0x01
	io.nextRegs.regs[0x23] = 0x20
	io.nextLineInt.on = true
	io.nextInt.hw = true
	io.nextInt.ctc[1].running = true
	io.nextInt.ctc[3].running = true
	io.dma.aStart, io.dma.bStart, io.dma.length = 0x4000, 0x8000, 6912
	io.dma.enabled, io.dma.aToB = true, true
	io.copper.mode, io.copper.pc = 3, 17
	pc, cycles, frames := zx.cpu.PC, zx.cpu.Cycles, zx.frameCount

	text := diagnosticsText(zx, "next")
	fitsThePanel(t, "diagnostics of a Next", text)
	for _, want := range []string{
		"## Next: CPU and memory", "Speed           28 MHz", "Timing          +3",
		"MMU 4 to 7      0A 0B ", "Interrupts      hardware, IM 2 mode", "Line interrupt  on, at line 288",
		"CTC             2 of 4 channels counting",
		"## Next: picture", "Layer order     L U S, top first", "Sprites         on, over the border too",
		"## Next: devices", "DMA             enabled, zxn mode, A to B", "DMA A, B        4000, 8000, length 6912",
		"Copper          running from the start, again every frame", "Copper at       instruction 17 of 1024",
		"DivMMC",
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
	if zx.cpu.PC != pc || zx.cpu.Cycles != cycles || zx.frameCount != frames || io.copper.pc != 17 {
		t.Error("the report changed the machine")
	}

	// Things off say they are off.
	io.nextLineInt.on, io.nextInt.hw = false, false
	io.nextRegs.regs[0x15] = 0
	text = diagnosticsText(zx, "next")
	for _, want := range []string{"Line interrupt  off", "Interrupts      as a Spectrum's", "Sprites         off"} {
		if !strings.Contains(text, want) {
			t.Errorf("the report lacks %q:\n%s", want, text)
		}
	}
}

// A machine that is no Next has none of it.
func TestDiagnosticsOfASpectrumHasNoNextHardware(t *testing.T) {
	text := diagnosticsText(testZXWithDrive(), "48k")
	for _, bad := range []string{"## Next", "MMU 0 to 3", "Copper"} {
		if strings.Contains(text, bad) {
			t.Errorf("a 48K has %q:\n%s", bad, text)
		}
	}
	if lines := nextDiagnosticsLines(testZXWithDrive()); lines != nil {
		t.Errorf("a 48K has Next lines: %v", lines)
	}
}
