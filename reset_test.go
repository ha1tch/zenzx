package zenzx

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// What a reset has to do, model by model, is what a switched-on machine is: after Reset, a
// machine that has run a program is compared field by field with one that has never run
// anything (docs/TRACKING.md T-74). The comparison is the machine-state codec's own dump
// (pkg/machinestate, nextStatePolicy), so a field that is added to a model later is compared
// without anyone remembering to add it here.
//
// Only what a reset really leaves is left out of the comparison, and each rule says why.

// resetDump is the machine's state as one "path = value" line for each field.
func resetDump(t *testing.T, zx *ZenZX) map[string]string {
	t.Helper()
	var b bytes.Buffer
	if err := nextStatePolicy().Dump(&b, zx); err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, l := range strings.Split(b.String(), "\n") {
		if i := strings.Index(l, " = "); i > 0 {
			m[l[:i]] = l[i+3:]
		}
	}
	return m
}

// resetLeaves says whether the field named k is one a reset does not put back, or one that has
// no meaning to compare, and why.
func resetLeaves(k, v string, afterRunning bool) (why string, leaves bool) {
	switch {
	case strings.HasPrefix(k, "root0.memory.ram["), strings.HasPrefix(k, "root0.memory.screen."),
		strings.HasPrefix(k, "root0.memory.nextExtraRAM"), strings.HasPrefix(k, "root0.memory.nextCfg.scratch"):
		return "memory: a reset keeps its contents", true
	case k == "root0.io.borderColor", k == "root0.io.speaker", k == "root0.io.ulaLastOut":
		return "the ULA has no reset line; the ROM writes its latch again", true
	case k == "root0.frameOrigin", strings.Contains(k, "borderHistory"):
		return "where the frame timeline stands is the host's", true
	case k == "root0.io.intVector", strings.HasSuffix(k, "divMMC.fetchPC"):
		return "what was last on the bus, which nothing reads", true
	case strings.Contains(k, "patternGeneration"), k == "root0.io.nextPaletteVersion":
		return "a counter that only goes up, so that a renderer's cache is never taken for current", true
	case strings.HasPrefix(v, "#"):
		return "an object's identity in the dump, not its contents", true
	case strings.HasPrefix(k, "root0.cpu."):
		// Straight after a reset the CPU's flags and mode are compared (a reset clears them);
		// its working registers never are. After the machines have run on, the CPU is a program's.
		if !afterRunning && (strings.HasPrefix(k, "root0.cpu.IFF") || strings.HasPrefix(k, "root0.cpu.IM") || k == "root0.cpu.Halted") {
			return "", false
		}
		return "the CPU's working registers are the program's", true
	}
	return "", false
}

// resetDiff lists the fields in which a differs from b that a reset must have put back.
func resetDiff(a, b map[string]string, afterRunning bool) []string {
	var out []string
	for k, av := range a {
		bv := b[k]
		if bv == av {
			continue
		}
		if _, leaves := resetLeaves(k, av, afterRunning); leaves {
			continue
		}
		if _, leaves := resetLeaves(k, bv, afterRunning); leaves {
			continue
		}
		out = append(out, fmt.Sprintf("%s: switched on %s, after reset %s", k, clip(av), clip(bv)))
	}
	sort.Strings(out)
	return out
}

func clip(s string) string {
	if len(s) > 48 {
		return s[:48] + "..."
	}
	return s
}

// snapshotForModel is the snapshot a test loads into a machine of model: the file, when it is of
// the kind of machine the model is, and otherwise (a +3 has no snapshot among the files, a
// snapshot being of the machine it was taken on) one that a +3 of that model saves of itself after
// it has run for a while.
func snapshotForModel(t *testing.T, model, file, format string) (string, string) {
	t.Helper()
	sm, err := readSnapshotMachine(file, format)
	if err != nil {
		t.Fatal(err)
	}
	if c := classOfModel(model); c == classOther || c == sm.Class {
		return file, format
	}
	src, err := newMachine(model, "./rom")
	if err != nil {
		t.Skipf("%s: %v", model, err)
	}
	resetRun(src, 100)
	path := filepath.Join(t.TempDir(), model+".zxs")
	if err := src.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	return path, "zxs"
}

func resetRun(zx *ZenZX, frames int) {
	for i := 0; i < frames; i++ {
		zx.RunFrame()
	}
}

// A machine that has been running a real program, once reset, is a machine that has just been
// switched on: the same hardware state at once, and the same machine again after it has run
// on, which for a 128K is the 128 menu and not the 48K BASIC a leftover ROM latch gave it.
func TestResetIsASwitchOn(t *testing.T) {
	const snaps = "testdata/snapshots/"
	cases := []struct{ model, file, format string }{
		{"48k", "manicminer_v2_48k.z80", "z80"},
		{"48k", "jetsetwilly_v1_48k.z80", "z80"},
		{"48k", "z88dk_zx48.sna", "sna"},
		{"128k", "z80attack_v3_128k.z80", "z80"},
		{"128k", "z88dk_zx128.sna", "sna"},
		{"plus2", "z80attack_v3_128k.z80", "z80"},
		{"plus3", "z80attack_v3_128k.z80", "z80"},
		{"ts2068", "manicminer_v2_48k.z80", "z80"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.model+"/"+c.file, func(t *testing.T) {
			cold, err := newMachine(c.model, "./rom")
			if err != nil {
				t.Skipf("%s: %v", c.model, err)
			}
			zx, err := newMachine(c.model, "./rom")
			if err != nil {
				t.Fatal(err)
			}
			snapFile, snapFormat := snapshotForModel(t, c.model, snaps+c.file, c.format)
			if err := zx.LoadSnapshotFormat(snapFile, snapFormat); err != nil {
				t.Fatalf("loading %s: %v", c.file, err)
			}
			zx.paused = false
			resetRun(zx, 300) // far enough for the program to be well into what it does

			zx.Reset()
			if d := resetDiff(resetDump(t, cold), resetDump(t, zx), false); len(d) > 0 {
				t.Errorf("right after Reset, %d fields are not as on a machine just switched on:\n  %s", len(d), strings.Join(d, "\n  "))
			}

			resetRun(cold, 400)
			resetRun(zx, 400)
			if d := resetDiff(resetDump(t, cold), resetDump(t, zx), true); len(d) > 0 {
				t.Errorf("400 frames after Reset, the machine is not where a switched-on one is:\n  %s", strings.Join(d, "\n  "))
			}
		})
	}
}

// The memory latches, one at a time and by name, since they are what made a 128K start in the
// wrong ROM: after Reset the ROM, the RAM at the top, the screen and the lock are as on
// switch-on, and the paging that the lock had shut works again.
func TestResetPutsBackThePaging(t *testing.T) {
	type setup struct {
		name    string
		model   string
		writes  [][2]uint16 // port, value
		locked  bool
		special bool
	}
	cases := []setup{
		{"128k locked", "128k", [][2]uint16{{0x7FFD, 0x37}}, true, false}, // RAM 7 at the top, ROM 1, the lock
		{"plus2 locked", "plus2", [][2]uint16{{0x7FFD, 0x37}}, true, false},
		{"plus3 locked", "plus3", [][2]uint16{{0x7FFD, 0x37}}, true, false},
		{"plus3 special paging", "plus3", [][2]uint16{{0x1FFD, 0x05}, {0x7FFD, 0x17}}, false, true}, // configuration 2: banks 4, 5, 6, 3
		{"plus3 ROM 3", "plus3", [][2]uint16{{0x1FFD, 0x04}, {0x7FFD, 0x10}}, false, false},         // the +3's fourth ROM
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			zx, err := newMachine(c.model, "./rom")
			if err != nil {
				t.Skipf("%s: %v", c.model, err)
			}
			m := zx.memory
			for _, w := range c.writes {
				zx.io.WritePort(w[0], uint8(w[1]))
			}
			if m.pagingLocked != c.locked || m.specialPaging != c.special || m.port7FFD == 0 {
				t.Fatalf("setting up: ports 0x%02X/0x%02X, locked %v, special %v", m.port7FFD, m.port1FFD, m.pagingLocked, m.specialPaging)
			}

			zx.Reset()
			if m.romBank != 0 || m.ramBankTop != 0 || m.port7FFD != 0 || m.port1FFD != 0 || m.pagingLocked || m.specialPaging || m.screenBank != 5 {
				t.Errorf("after Reset: ROM %d, RAM at the top %d, ports 0x%02X/0x%02X, locked %v, special %v, screen %d; want ROM 0, RAM 0, ports 0, unlocked, normal, screen 5",
					m.romBank, m.ramBankTop, m.port7FFD, m.port1FFD, m.pagingLocked, m.specialPaging, m.screenBank)
			}
			if m.ramBankLow != 5 || m.ramBankHigh != 2 {
				t.Errorf("after Reset the RAM at 0x4000 and 0x8000 is banks %d and %d, want 5 and 2", m.ramBankLow, m.ramBankHigh)
			}
			zx.io.WritePort(0x7FFD, 0x07) // a lock would have shut this
			if m.ramBankTop != 7 {
				t.Errorf("after Reset, paging is still locked: RAM at the top is %d after a write of RAM 7", m.ramBankTop)
			}
		})
	}
}

// The shadow screen is switched back to the normal one without losing what was on it: it is
// RAM, and a reset keeps RAM.
func TestResetKeepsTheShadowScreenInItsBank(t *testing.T) {
	zx, err := newMachine("128k", "./rom")
	if err != nil {
		t.Skip(err)
	}
	zx.io.WritePort(0x7FFD, 0x08) // the shadow screen, bank 7, on show
	zx.screen.bitmap[10] = 0xA5   // drawn on it
	zx.Reset()
	if zx.memory.screenBank != 5 {
		t.Fatalf("screen bank %d after Reset, want 5", zx.memory.screenBank)
	}
	if zx.memory.ram[7][10] != 0xA5 {
		t.Errorf("what was on the shadow screen is not in bank 7: 0x%02X", zx.memory.ram[7][10])
	}
}

// The AY's /RESET is the machine's: its registers and its counters go back, and so does the
// copy of them that a snapshot takes.
func TestResetSilencesTheAY(t *testing.T) {
	for _, model := range []string{"128k", "plus2", "plus3", "ts2068", "next"} {
		model := model
		t.Run(model, func(t *testing.T) {
			zx, err := newMachine(model, "./rom")
			if err != nil {
				t.Skipf("%s: %v", model, err)
			}
			ayWrite := func(reg, value uint8) {
				if model == "ts2068" { // the TS2068 has its own ports for the chip
					zx.io.WritePort(0xF5, uint8(reg))
					zx.io.WritePort(0xF6, value)
					return
				}
				zx.io.WritePort(0xFFFD, uint8(reg))
				zx.io.WritePort(0xBFFD, value)
			}
			ayWrite(0, 0x55)
			ayWrite(8, 0x0F)
			if got := zx.audio.ReadAYRegister(0); got != 0x55 {
				t.Skipf("setting up: the chip holds 0x%02X in register 0, not 0x55 (no chip on this build?)", got)
			}

			zx.Reset()
			if got := zx.audio.ReadAYRegister(0); got != 0 {
				t.Errorf("the chip's register 0 is 0x%02X after Reset", got)
			}
			if got := zx.audio.ReadAYRegister(8); got != 0 {
				t.Errorf("the chip's register 8 (channel A's volume) is 0x%02X after Reset", got)
			}
			if regs := zx.io.GetAYRegisters(); regs != [16]uint8{} || zx.io.ayRegister != 0 {
				t.Errorf("the registers' copy is %v, register %d selected", regs, zx.io.ayRegister)
			}
		})
	}
}

// The Timex ports are the SCLD's, which a reset clears with it: the screen mode and the chunk
// of ROM that a program chose are not still chosen.
func TestResetPutsBackTheTimexPorts(t *testing.T) {
	zx, err := newMachine("ts2068", "./rom")
	if err != nil {
		t.Skip(err)
	}
	zx.io.WritePort(0xF4, 0x01) // chunk 0 out of the Home bank
	zx.io.WritePort(0xFF, 0x82) // hi-colour, and the extension ROM in the chunks
	if _, std := zx.videoRenderer.(standardVideoRenderer); std {
		t.Fatal("setting up: hi-colour mode did not change the renderer")
	}

	zx.Reset()
	if zx.io.ts2068HSR != 0 || zx.io.ts2068Port0xFF != 0 || zx.memory.ts2068HSRChunk0 || zx.memory.ts2068ExRomSelect {
		t.Errorf("ports 0xF4/0xFF are 0x%02X/0x%02X after Reset, chunk 0 out %v, extension ROM %v",
			zx.io.ts2068HSR, zx.io.ts2068Port0xFF, zx.memory.ts2068HSRChunk0, zx.memory.ts2068ExRomSelect)
	}
	if _, std := zx.videoRenderer.(standardVideoRenderer); !std {
		t.Errorf("the renderer is %T after Reset, want the standard one", zx.videoRenderer)
	}
}

// The +3's disk controller starts again, with the disk still in the drive.
func TestResetStartsTheDiscControllerAgain(t *testing.T) {
	zx, err := newMachine("plus3", "./rom")
	if err != nil {
		t.Skip(err)
	}
	fdc := zx.io.fdc
	if fdc == nil {
		t.Fatal("a +3 with no disk controller")
	}
	fdc.diskPresent, fdc.diskFilename, fdc.diskModified = true, "x.dsk", true
	fdc.motorOn, fdc.commandPhase, fdc.commandLen, fdc.mainStatus = true, 2, 9, 0xD0
	fdc.commandBuf, fdc.resultBuf = []uint8{1, 2, 3}, []uint8{4, 5}

	zx.Reset()
	if fdc.motorOn || fdc.commandPhase != 0 || fdc.commandLen != 0 || fdc.mainStatus != 0x80 || fdc.commandBuf != nil || fdc.resultBuf != nil {
		t.Errorf("the controller is mid-command after Reset: motor %v, phase %d, %d command bytes, status 0x%02X",
			fdc.motorOn, fdc.commandPhase, fdc.commandLen, fdc.mainStatus)
	}
	if !fdc.diskPresent || fdc.diskFilename != "x.dsk" || !fdc.diskModified {
		t.Error("Reset took the disk out of the drive, or forgot what was not yet written to it")
	}
}

// What a reset keeps it keeps: the RAM and the ULA's latch, on every model.
func TestResetKeepsMemoryAndTheULALatch(t *testing.T) {
	for _, model := range []string{"48k", "128k", "plus3", "ts2068", "next"} {
		model := model
		t.Run(model, func(t *testing.T) {
			zx, err := newMachine(model, "./rom")
			if err != nil {
				t.Skipf("%s: %v", model, err)
			}
			zx.memory.ram[0][123], zx.memory.ram[2][456] = 0x5A, 0xA5
			zx.io.WritePort(0xFE, 0x0A) // border 2, the speaker
			zx.Reset()
			if zx.memory.ram[0][123] != 0x5A || zx.memory.ram[2][456] != 0xA5 {
				t.Error("Reset cleared RAM")
			}
			if zx.io.borderColor != 2 || zx.io.ulaLastOut != 0x0A {
				t.Errorf("Reset changed the ULA's latch: border %d, last byte 0x%02X", zx.io.borderColor, zx.io.ulaLastOut)
			}
		})
	}
}

// A Next resets as a Next does. With no boot chain attached (the firmware's own tests are the
// hardware guard G-15, reset_next_hw_test.go), a reset is a hard reset of the registers and
// of everything they drive, so that a program's Layer 2, tilemap, sprites, copper and palettes
// are not over the next thing that runs; and a Next with no boot ROM has no config mode to
// start in.
func TestResetNextIsAHardReset(t *testing.T) {
	cold, err := newMachine("next", "./rom")
	if err != nil {
		t.Skip(err)
	}
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	reg := func(r, v uint8) {
		zx.io.WritePort(0x243B, r)
		zx.io.WritePort(0x253B, v)
	}
	resetRun(zx, 3)
	reg(0x69, 0x80) // Layer 2 on
	reg(0x15, 0x01) // sprites on
	reg(0x6B, 0xA9) // the tilemap on
	reg(0x70, 0x10) // Layer 2's wide mode
	reg(0x12, 0x09) // its bank
	reg(0x0A, 0x11) // DivMMC automap
	reg(0x07, 0x03) // 28 MHz
	reg(0x56, 0x20) // a page in slot 6
	reg(0x40, 0x03) // a palette entry
	reg(0x41, 0xAA)
	reg(0x34, 0x05) // a sprite
	reg(0x35, 0x42)
	reg(0x38, 0x80)
	reg(0x62, 0xC0) // the copper, running
	zx.io.WritePort(0x123B, 0x03)
	zx.io.WritePort(0xFF, 0x2E)
	zx.io.WritePort(0x7FFD, 0x17)
	zx.io.WritePort(0xFFFD, 0)
	zx.io.WritePort(0xBFFD, 0x55)
	resetRun(zx, 2)
	if zx.io.nextRegs.regs[0x69]&0x80 == 0 || zx.io.nextRegs.regs[0x15] != 0x01 { // port 0xFF owns bits 5-0 of 0x69
		t.Fatal("setting up: the registers were not written")
	}

	zx.Reset()
	if zx.io.nextResetRequest != nextResetHard {
		t.Fatalf("Reset on a Next asks for reset %d, want the hard one (%d)", zx.io.nextResetRequest, nextResetHard)
	}
	resetRun(zx, 1)
	resetRun(cold, 1)
	if zx.memory.nextCfg.active || zx.memory.bootROMEnabled {
		t.Error("a Next with no boot ROM went into config mode")
	}
	if d := resetDiff(resetDump(t, cold), resetDump(t, zx), false); len(d) > 0 {
		t.Errorf("a Next that has been reset is not as one just switched on, in %d fields:\n  %s", len(d), strings.Join(d, "\n  "))
	}
	// Said outright, apart from the comparison, for the registers the pictures depend on.
	for _, r := range []struct{ reg, want uint8 }{{0x69, 0}, {0x15, 0}, {0x6B, 0}, {0x70, 0}, {0x12, 0x08}, {0x0A, 0x01}, {0x07, 0}, {0x62, 0}} {
		if got := zx.io.nextRegs.regs[r.reg]; got != r.want {
			t.Errorf("NextREG 0x%02X is 0x%02X after Reset, want 0x%02X", r.reg, got, r.want)
		}
	}
	if zx.io.copper.mode != 0 || zx.io.nextTimexPort != 0 || zx.io.sprites.attrs[5] != (spriteAttributes{}) {
		t.Error("the copper, the Timex port or a sprite was left as the program set it")
	}
}
