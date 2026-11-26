//go:build !headless

package zenzx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// The Run Diagnostics item of the Snapshot menu.
//
// It used to print a developer's dump (a hex dump of a test buffer, and ten instructions run
// on the live CPU to watch R) to the terminal. It now opens a window with a report on the
// machine as it is: what it is, what its CPU and memory are doing, what is in the tape deck,
// the disc drive and the card slots, and a few checks that say plainly whether something is
// wrong. Nothing in it changes the machine. The text is in the reader's own markup, as the
// info windows' are (info_gui.go): lines under 70 columns, "## " headings, and a line that
// begins with two spaces drawn dimmer.

// diagRegs writes registers four to a line: "AF 1234  BC 0000  ...".
func diagRegs(pairs ...string) string { return "  " + strings.Join(pairs, "   ") }

// diagCheck is one line of the checks: ok, or the reason it is not.
func diagCheck(label string, ok bool, why string) string {
	if ok {
		return infoLine(label, "ok")
	}
	return infoLine(label, "CHECK: "+why)
}

// snapshotByteOrderOK saves a known register set the way the snapshot files do and reads it
// back: a snapshot written on this build is read in the same byte order it was written in.
func snapshotByteOrderOK() bool {
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, CPUState{AF: 0x1234, BC: 0x5678, PC: 0xABCD}); err != nil {
		return false
	}
	data := buf.Bytes()
	if len(data) < 6 || data[0] != 0x34 || data[1] != 0x12 || data[2] != 0x78 || data[3] != 0x56 {
		return false
	}
	var back CPUState
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &back); err != nil {
		return false
	}
	return back.AF == 0x1234 && back.BC == 0x5678 && back.PC == 0xABCD
}

// diagWhere says what an address is: the ROM, the screen, the attributes, the system
// variables or the rest of RAM.
func diagWhere(addr uint16) string {
	switch {
	case addr < 0x4000:
		return "ROM"
	case addr < 0x5800:
		return "screen"
	case addr < 0x5B00:
		return "attributes"
	case addr < 0x5C00:
		return "printer buffer"
	case addr < 0x6000:
		return "system variables"
	}
	return "RAM"
}

// snapshotFormatWord is the format Quick Save writes, as the report says it.
func snapshotFormatWord(zx *ZenZX) string {
	if zx.snapshotFormat == "" || zx.snapshotFormat == "auto" {
		return "ZXS (the -format option is auto)"
	}
	return strings.ToUpper(zx.snapshotFormat) + " (the -format option)"
}

// showDiagnostics opens the report in a window: Snapshot > Run Diagnostics, and F3.
func (b *appMenuBar) showDiagnostics(zx *ZenZX) {
	b.showInfo("DIAGNOSTICS", diagnosticsText(zx, b.currentModel()))
}

// diagnosticsText is the report. model is the model the menu has ticked ("" when it is none
// of the menu's).
func diagnosticsText(zx *ZenZX, model string) string {
	cpu, mem := zx.cpu, zx.memory
	state := "running"
	if zx.paused {
		state = "paused"
	}
	name := "unknown"
	if model != "" {
		name = modelLabel(model)
	}
	lines := []string{
		"## Machine",
		"",
		infoLine("Model", name),
		infoLine("State", state),
		infoLine("Frames run", fmt.Sprintf("%d", zx.frameCount)),
		infoLine("Frame", fmt.Sprintf("%d cycles, %d of them in the interrupt", zx.cyclesPerFrame, zx.interruptLength)),
		infoLine("Cycles run", fmt.Sprintf("%d", cpu.Cycles)),
		infoLine("Snapshots in", snapshotFormatWord(zx)),
		"",
		"## CPU",
		"",
		diagRegs(fmt.Sprintf("AF %04X", cpu.AF()), fmt.Sprintf("BC %04X", cpu.BC()),
			fmt.Sprintf("DE %04X", cpu.DE()), fmt.Sprintf("HL %04X", cpu.HL())),
		diagRegs(fmt.Sprintf("AF' %04X", uint16(cpu.A_)<<8|uint16(cpu.F_)),
			fmt.Sprintf("BC' %04X", uint16(cpu.B_)<<8|uint16(cpu.C_)),
			fmt.Sprintf("DE' %04X", uint16(cpu.D_)<<8|uint16(cpu.E_)),
			fmt.Sprintf("HL' %04X", uint16(cpu.H_)<<8|uint16(cpu.L_))),
		diagRegs(fmt.Sprintf("IX %04X", cpu.IX()), fmt.Sprintf("IY %04X", cpu.IY()),
			fmt.Sprintf("SP %04X", cpu.SP), fmt.Sprintf("PC %04X", cpu.PC)),
		diagRegs(fmt.Sprintf("I %02X", cpu.I), fmt.Sprintf("R %02X", cpu.R),
			fmt.Sprintf("IM %d", cpu.IM)),
		"",
		infoLine("Flags", fmt.Sprintf("S%d Z%d H%d P/V%d N%d C%d",
			btoi(cpu.F&0x80 != 0), btoi(cpu.F&0x40 != 0), btoi(cpu.F&0x10 != 0),
			btoi(cpu.F&0x04 != 0), btoi(cpu.F&0x02 != 0), btoi(cpu.F&0x01 != 0))),
		infoLine("Interrupts", fmt.Sprintf("%s, mode %d, halted %s",
			map[bool]string{true: "enabled", false: "disabled"}[cpu.IFF1], cpu.IM, yesNo(cpu.Halted))),
		infoLine("PC is in", diagWhere(cpu.PC)),
	}
	if cpu.PC < 0x4000 {
		if addr, routine := nearestROMRoutine(cpu.PC); routine != "" {
			lines = append(lines, infoLine("Near", fmt.Sprintf("%04X %s, +%d", addr, routine, cpu.PC-addr)))
		}
	}

	lines = append(lines, "", "## Memory", "")
	switch {
	case mem.isNext:
		lines = append(lines, infoLine("Layout", "Next: ROMs in SRAM, 8K MMU slots, 128K paging"))
	case mem.isPlus3:
		lines = append(lines, infoLine("Layout", "+3: four ROMs, eight RAM banks, special paging"))
	case mem.is128K:
		lines = append(lines, infoLine("Layout", "128K: two ROMs, eight RAM banks"))
	default:
		lines = append(lines, infoLine("Layout", "48K: one ROM, 48K of RAM, no banking"))
	}
	if what, ok := zx.nextROMWhat(); ok {
		// which ROM the CPU reads, and the registers that say why: a patch to the wrong bank looks
		// like a patch that does nothing, and this is how to tell
		lines = append(lines,
			infoLine("ROM live", what),
			infoLine("ROM lock", fmt.Sprintf("%d (NextREG 0x8C is %02X)", (mem.nextAltROM>>4)&0x03, mem.nextAltROM)),
			infoLine("ROM by ports", fmt.Sprintf("%d", mem.romBank)))
	}
	if mem.is128K || mem.isPlus3 {
		lines = append(lines,
			infoLine("ROM", fmt.Sprintf("bank %d", mem.romBank)),
			infoLine("RAM at 4000", fmt.Sprintf("bank %d", mem.ramBankLow)),
			infoLine("RAM at 8000", fmt.Sprintf("bank %d", mem.ramBankHigh)),
			infoLine("RAM at C000", fmt.Sprintf("bank %d", mem.ramBankTop)),
			infoLine("Screen", fmt.Sprintf("bank %d", mem.screenBank)),
			infoLine("Paging", fmt.Sprintf("%s (port 7FFD %02X)", map[bool]string{true: "locked", false: "free"}[mem.pagingLocked], mem.port7FFD)))
		if mem.isPlus3 {
			lines = append(lines, infoLine("Port 1FFD", fmt.Sprintf("%02X, special mode %d", mem.port1FFD, mem.specialMode)))
		}
	}

	lines = append(lines, nextDiagnosticsLines(zx)...) // none unless this is a Next (diagnostics_next_gui.go)

	lines = append(lines, "", "## Stack", "",
		infoLine("SP", fmt.Sprintf("%04X, in %s", cpu.SP, diagWhere(cpu.SP))), "")
	sp := cpu.SP
	for i := uint16(0); i < 6 && uint32(sp)+uint32(i)*2+1 <= 0xFFFF; i++ {
		addr := sp + i*2
		word := uint16(mem.Read(addr+1))<<8 | uint16(mem.Read(addr))
		lines = append(lines, fmt.Sprintf("  %04X  %04X  %s", addr, word, diagWhere(word)))
	}

	lines = append(lines, "", "## Tape", "")
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		lines = append(lines, "  No tape is loaded.")
	} else {
		st := zx.tape.st
		mode := map[TapeMode]string{TapeFast: "fast", TapeAccurate: "accurate", TapeTurbo: "turbo"}[st.Mode]
		play := "stopped"
		switch {
		case st.Playing && st.Paused:
			play = "paused"
		case st.Playing:
			play = "playing"
		}
		lines = append(lines,
			infoLine("File", infoPath(st.Filename)),
			infoLine("Mode", mode),
			infoLine("State", fmt.Sprintf("%s, block %d of %d", play, min(st.Position+1, len(st.Blocks)), len(st.Blocks))))
	}

	if zx.io.hasFDC && zx.io.fdc != nil {
		lines = append(lines, "", "## Floppy disk", "")
		fdc := zx.io.fdc
		switch {
		case !fdc.diskPresent:
			lines = append(lines, "  Drive A: is empty.")
		default:
			file := "a blank disc, not saved yet"
			if fdc.diskFilename != "" {
				file = infoPath(fdc.diskFilename)
			}
			changes := "none"
			if fdc.diskModified {
				changes = "unsaved"
			}
			lines = append(lines, infoLine("Drive A:", file), infoLine("Changes", changes))
		}
	}

	if usesSDCards(zx) {
		lines = append(lines, "", "## SD cards", "")
		slot := func(n int, path string, protected bool) string {
			if path == "" {
				return infoLine(fmt.Sprintf("Slot %d", n), "empty")
			}
			w := "writable"
			if protected {
				w = "write-protected"
			}
			return infoLine(fmt.Sprintf("Slot %d", n), w+", "+infoPath(path))
		}
		lines = append(lines, slot(1, zx.next.files.Card, zx.next.protected))
		if zx.card2 == nil {
			lines = append(lines, slot(2, "", false))
		} else {
			lines = append(lines, slot(2, zx.card2.path, zx.card2.protected))
		}
	}

	lines = append(lines, "", "## Checks", "",
		diagCheck("Byte order", snapshotByteOrderOK(), "a saved register set did not read back (snapshot files)"),
		diagCheck("Stack pointer", cpu.SP >= 0x4000 || cpu.SP == 0, "SP points into the ROM, where nothing can be pushed"))
	if !mem.is128K && !mem.isPlus3 {
		ok := mem.ramBankLow == 5 && mem.ramBankHigh == 2 && mem.ramBankTop == 0
		lines = append(lines, diagCheck("48K memory map", ok,
			fmt.Sprintf("banks %d/%d/%d, not 5/2/0", mem.ramBankLow, mem.ramBankHigh, mem.ramBankTop)))
	} else {
		ok := mem.ramBankLow <= 7 && mem.ramBankHigh <= 7 && mem.ramBankTop <= 7 && mem.screenBank <= 7
		lines = append(lines, diagCheck("Banks in range", ok, "a bank number is above 7"))
	}
	if st, err := os.Stat(snapshotPath(quickSaveName("zxs"))); err == nil && st.Size() == 0 {
		lines = append(lines, infoLine("Quick save file", "CHECK: "+snapshotPath(quickSaveName("zxs"))+" is empty"))
	}
	return strings.Join(lines, "\n")
}

// yesNo is a bool as a word.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
