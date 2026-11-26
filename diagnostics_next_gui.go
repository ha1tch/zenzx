//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"strings"
)

// The Next's part of the Diagnostics report (diagnostics_gui.go): what a person who is chasing a
// program that does not do what it should needs to see at once, and has otherwise to read out of
// NextREGs one at a time. Everything here is read from the machine as it is and changes nothing:
// the NextREG file is read directly (GetNextRegister has special cases that are not reads), and a
// register is decoded only where the decoding is the one the emulation itself uses.
//
// Three sections: the CPU and what reaches it (speed, timing, the eight MMU slots, the interrupts),
// the picture (layer order, sprites, Layer 2, the tilemap, LoRes, the Timex mode) and the devices
// that work on their own (the DMA, the copper, the DivMMC).

// nextCPUSpeedWord is the CPU's speed as NextREG 0x07 sets it.
func nextCPUSpeedWord(shift uint8) string {
	return [...]string{"3.5 MHz", "7 MHz", "14 MHz", "28 MHz"}[shift&3]
}

// nextTimingWord is the machine timing NextREG 0x03 bits 2-0 give.
func nextTimingWord(nr03 uint8) string {
	switch nr03 & 0x07 {
	case 1:
		return "48K"
	case 2:
		return "128K"
	case 3:
		return "+3"
	case 4:
		return "Pentagon"
	}
	return fmt.Sprintf("%d (not a timing)", nr03&0x07)
}

// nextLayerOrderWord is the layer order NextREG 0x15 bits 4-2 give, top layer first, in the Next's own
// letters: S the sprites, L Layer 2 and U the ULA (and the tilemap, which is part of it).
func nextLayerOrderWord(nr15 uint8) string {
	bits := (nr15 >> 2) & 0x07
	if bits >= 6 {
		return fmt.Sprintf("%03b (a blending mode)", bits)
	}
	return [...]string{"S L U", "L S U", "S U L", "L U S", "U S L", "U L S"}[bits] + ", top first"
}

// nextCopperWord is the copper's start control (NextREG 0x62 bits 7-6).
func nextCopperWord(mode uint8) string {
	return [...]string{"stopped", "running from the start", "running from where it was",
		"running from the start, again every frame"}[mode&3]
}

// nextTimexWord is the ULA's screen mode (port 0xFF bits 2-0).
func nextTimexWord(mode uint8) string {
	switch mode & 0x07 {
	case 0:
		return "standard"
	case 1:
		return "second screen"
	case 2:
		return "hi-colour"
	case 6:
		return "hi-res"
	}
	return fmt.Sprintf("%d (not a mode)", mode&0x07)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// nextDiagnosticsLines are the Next's sections of the report, none for a machine that is no Next.
func nextDiagnosticsLines(zx *ZenZX) []string {
	io, mem := zx.io, zx.memory
	if mem == nil || !mem.isNext || io == nil || io.nextRegs == nil {
		return nil
	}
	reg := func(r uint8) uint8 { return io.nextRegs.regs[r] }

	var lines []string
	lines = append(lines, "", "## Next: CPU and memory", "",
		infoLine("Speed", nextCPUSpeedWord(io.nextSpeed.shift)),
		infoLine("Timing", nextTimingWord(reg(0x03))),
		infoLine("MMU 0 to 3", fmt.Sprintf("%02X %02X %02X %02X  (FF is ROM)", mem.nextSlots[0], mem.nextSlots[1], mem.nextSlots[2], mem.nextSlots[3])),
		infoLine("MMU 4 to 7", fmt.Sprintf("%02X %02X %02X %02X", mem.nextSlots[4], mem.nextSlots[5], mem.nextSlots[6], mem.nextSlots[7])))

	im := "as a Spectrum's"
	if io.nextInt.hw {
		im = "hardware, IM 2 mode"
	}
	line := "off"
	if io.nextLineInt.on {
		line = fmt.Sprintf("on, at line %d", int(reg(0x22)&0x01)<<8|int(reg(0x23)))
	}
	running := 0
	for _, c := range io.nextInt.ctc {
		if c.running {
			running++
		}
	}
	lines = append(lines,
		infoLine("Interrupts", im),
		infoLine("Pending", fmt.Sprintf("%03X", io.nextInt.pending)),
		infoLine("Line interrupt", line),
		infoLine("CTC", fmt.Sprintf("%d of 4 channels counting", running)))

	nr15 := reg(0x15)
	l2 := "off"
	if io.layer2Enabled() {
		w, h := layer2FramebufferDimensions(io.layer2CurrentMode())
		l2 = fmt.Sprintf("on, %dx%d, from bank %d", w, h, reg(0x12)&0x7F)
	}
	tm := "off"
	if io.tilemapEnabled() {
		tm = "on, 40x32"
		if io.tilemapWide() {
			tm = "on, 80x32"
		}
	}
	sprites := onOff(nr15&0x01 != 0)
	if nr15&0x01 != 0 && nr15&0x02 != 0 {
		sprites = "on, over the border too"
	}
	lines = append(lines, "", "## Next: picture", "",
		infoLine("Layer order", nextLayerOrderWord(nr15)),
		infoLine("ULA mode", nextTimexWord(io.nextTimexMode())),
		infoLine("LoRes", onOff(io.loresEnabled())),
		infoLine("Layer 2", l2),
		infoLine("Tilemap", tm),
		infoLine("Sprites", sprites))

	d := &io.dma
	dma := "disabled"
	if d.enabled {
		dma = "enabled"
	}
	mode := "zxn"
	if d.z80Mode {
		mode = "Z80"
	}
	dir := "B to A"
	if d.aToB {
		dir = "A to B"
	}
	lines = append(lines, "", "## Next: devices", "",
		infoLine("DMA", fmt.Sprintf("%s, %s mode, %s", dma, mode, dir)),
		infoLine("DMA A, B", fmt.Sprintf("%04X, %04X, length %d", d.aStart, d.bStart, d.length)),
		infoLine("Copper", nextCopperWord(io.copper.mode)),
		infoLine("Copper at", fmt.Sprintf("instruction %d of %d", io.copper.pc, copperInstructions)))

	dm := "absent"
	if dv := io.divMMC; dv != nil {
		parts := []string{"not paged"}
		if dv.active() {
			parts = []string{"paged"}
			if dv.conmem() {
				parts = append(parts, "by CONMEM")
			} else {
				parts = append(parts, "by the automap")
			}
		}
		if dv.mapram() {
			parts = append(parts, "MAPRAM")
		}
		parts = append(parts, fmt.Sprintf("SRAM page %d", dv.sramPage()))
		dm = strings.Join(parts, ", ")
	}
	lines = append(lines, infoLine("DivMMC", dm))
	return lines
}
