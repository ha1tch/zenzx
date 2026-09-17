package main

import "testing"

// TestNextRegRawPort exercises the register file directly through
// SpectrumIO's port methods, bypassing the CPU entirely -- the "raw
// port OUT" half of ti0.r1's own done condition ("both a raw port OUT
// and the NEXTREG instruction produce identical, correct register-file
// state").
func TestNextRegRawPort(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)

	// Select register 0x07 (CPU speed) and write 0x01 (7MHz).
	io.WritePort(nextRegSelectPort, 0x07)
	io.WritePort(nextRegDataPort, 0x01)

	if got := io.GetNextRegister(0x07); got != 0x01 {
		t.Fatalf("register 0x07 = 0x%02X, want 0x01", got)
	}

	// Reading the data port back (not just GetNextRegister) must see
	// the same value, still selected.
	if got := io.ReadPort(nextRegDataPort); got != 0x01 {
		t.Fatalf("ReadPort(data) = 0x%02X, want 0x01", got)
	}

	// Reading the select port back must return the register number
	// currently selected.
	if got := io.ReadPort(nextRegSelectPort); got != 0x07 {
		t.Fatalf("ReadPort(select) = 0x%02X, want 0x07 (selected register readback)", got)
	}
}

// TestNextRegViaZ80NInstruction is the regression the file-level
// comment in nextreg.go promises: confirming zen80's NEXTREG
// instruction (ED 91, "NEXTREG n,n") actually reaches this handler,
// with no further CPU-side change required, rather than assuming it
// from reading z80n.go alone. Goes through EnableZ80N (not a bare
// zx.cpu.Z80N = true) so it exercises the same NextregWrite wiring
// production code uses -- see TestNextRegOpcodeDoesNotTouchSelectedReg
// below for the regression specific to that wiring's whole reason for
// existing.
func TestNextRegViaZ80NInstruction(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	zx.cpu.Reset()
	zx.EnableZ80N()

	// NEXTREG $15,$08 (ED 91 15 08): select register 0x15 (sprite/layer
	// system) and write 0x08 ("L U" ordering is the default already,
	// so pick a different, checkable value: 0x01).
	const testReg = 0x15
	const testVal = 0x01
	zx.memory.Write(0x8000, 0xED)
	zx.memory.Write(0x8001, 0x91)
	zx.memory.Write(0x8002, testReg)
	zx.memory.Write(0x8003, testVal)
	zx.cpu.PC = 0x8000

	zx.cpu.Step()

	if got := zx.io.GetNextRegister(testReg); got != testVal {
		t.Fatalf("after NEXTREG $%02X,$%02X: register = 0x%02X, want 0x%02X",
			testReg, testVal, got, testVal)
	}

	// NEXTREG $07,A (ED 92): select register 0x07, write the A register.
	zx.cpu.A = 0x02
	zx.memory.Write(0x8004, 0xED)
	zx.memory.Write(0x8005, 0x92)
	zx.memory.Write(0x8006, 0x07)
	zx.cpu.PC = 0x8004

	zx.cpu.Step()

	if got := zx.io.GetNextRegister(0x07); got != 0x02 {
		t.Fatalf("after NEXTREG $07,A (A=0x02): register = 0x%02X, want 0x02", got)
	}
}

// TestNextRegRawPortAndInstructionAgreeOnValue writes the same register
// via both paths and confirms they agree on the STORED VALUE -- but see
// TestNextRegOpcodeDoesNotTouchSelectedReg immediately below for the
// one respect in which they must NOT agree.
//
// T-33 CORRECTION: this test used to be
// TestNextRegRawPortAndInstructionAgree and additionally asserted the
// two paths produce fully "identical" state, which was wrong -- ti0.r1's
// own "done when" criterion (both a raw port OUT and the NEXTREG
// instruction produce identical, correct register-file state) encoded
// a real hardware misunderstanding. Per jnext (github.com/jorgegv/jnext,
// GH #54/RevivalSurvival.nex) and the VHDL it cites, real Next hardware's
// NEXTREG opcode writes directly into the register file and never
// touches the port-0x243B select latch, while a raw port OUT through
// 0x243B/0x253B does select-then-write and so DOES move that latch.
// The two paths therefore agree on the written register's value but
// must disagree on `selected` state whenever a register was already
// selected beforehand. This test keeps the value-agreement half, which
// is still true and still worth asserting; the divergence half moved to
// its own test below where it can be asserted precisely.
func TestNextRegRawPortAndInstructionAgreeOnValue(t *testing.T) {
	const testReg = 0x08
	const testVal = 0x55

	// Path 1: raw port OUT.
	rawMem, _ := newTestMemoryAndScreen()
	rawIO := NewSpectrumIO(rawMem, nil)
	rawIO.WritePort(nextRegSelectPort, testReg)
	rawIO.WritePort(nextRegDataPort, testVal)
	rawResult := rawIO.GetNextRegister(testReg)

	// Path 2: NEXTREG instruction via the real CPU, wired the way
	// production code wires it (EnableZ80N sets NextregWrite).
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	zx.cpu.Reset()
	zx.EnableZ80N()
	zx.memory.Write(0x8000, 0xED)
	zx.memory.Write(0x8001, 0x91)
	zx.memory.Write(0x8002, testReg)
	zx.memory.Write(0x8003, testVal)
	zx.cpu.PC = 0x8000
	zx.cpu.Step()
	instrResult := zx.io.GetNextRegister(testReg)

	if rawResult != instrResult {
		t.Fatalf("raw-port result 0x%02X != NEXTREG-instruction result 0x%02X for register 0x%02X",
			rawResult, instrResult, testReg)
	}
	if rawResult != testVal {
		t.Fatalf("both paths agree but neither matches the written value: got 0x%02X, want 0x%02X", rawResult, testVal)
	}
}

// TestNextRegOpcodeDoesNotTouchSelectedReg is the T-33 regression: the
// Z80N NEXTREG opcode, wired via EnableZ80N (zx.cpu.NextregWrite =
// zx.io.nextRegOpcodeWrite), must leave whatever register was
// previously selected via port 0x243B completely untouched -- unlike a
// raw port OUT through 0x243B/0x253B, which DOES move that selection.
// This is the exact real, previously-shipped jnext bug (GH #54): a
// caller that selects a register once and then polls it via repeated
// IN A,(0x253B) -- a raster-wait idiom -- had that read permanently
// redirected the moment any NEXTREG executed, e.g. from an interrupt
// handler (RevivalSurvival.nex hung this way until jnext fixed it).
func TestNextRegOpcodeDoesNotTouchSelectedReg(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	zx.cpu.Reset()
	zx.EnableZ80N()

	// Select register 0x1F via the port pair first, as a raster-wait
	// loop would before polling it in a tight IN A,(0x253B) loop.
	const preSelected = 0x1F
	zx.io.WritePort(nextRegSelectPort, preSelected)

	// Now execute NEXTREG $07,$99 -- selecting and writing a DIFFERENT
	// register via the opcode, as an interrupt handler might.
	const opcodeReg = 0x07
	const opcodeVal = 0x99
	zx.memory.Write(0x8000, 0xED)
	zx.memory.Write(0x8001, 0x91)
	zx.memory.Write(0x8002, opcodeReg)
	zx.memory.Write(0x8003, opcodeVal)
	zx.cpu.PC = 0x8000
	zx.cpu.Step()

	// The opcode must have written its target register...
	if got := zx.io.GetNextRegister(opcodeReg); got != opcodeVal {
		t.Fatalf("after NEXTREG $%02X,$%02X: register = 0x%02X, want 0x%02X",
			opcodeReg, opcodeVal, got, opcodeVal)
	}
	// ...but the port-0x243B selected register must be UNCHANGED --
	// reading the select port back must still report 0x1F, not 0x07.
	if got := zx.io.ReadPort(nextRegSelectPort); got != preSelected {
		t.Fatalf("ReadPort(select) = 0x%02X, want 0x%02X (NEXTREG opcode must not move the port-0x243B selected register)",
			got, preSelected)
	}
}

// TestNextRegReadOnly confirms register 0x00 (machine ID) and 0x01
// (core version), documented read-only, silently ignore writes rather
// than corrupting their identification value -- matches real hardware,
// where there is no documented write-triggers-an-error behaviour for
// these registers.
func TestNextRegReadOnly(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)

	before := io.GetNextRegister(0x00)
	io.WritePort(nextRegSelectPort, 0x00)
	io.WritePort(nextRegDataPort, 0xFF)
	if got := io.GetNextRegister(0x00); got != before {
		t.Fatalf("register 0x00 (read-only) changed from 0x%02X to 0x%02X after a write", before, got)
	}
}

// TestNextRegDefaults spot-checks power-on defaults documented by the
// SpecNext wiki: register 0x15's default sprite/layer priority
// ("S L U", value 0x08) and the Layer 2 clip window's full-screen
// default (0,255,0,191).
func TestNextRegDefaults(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)

	if got := io.GetNextRegister(0x15); got != 0x08 {
		t.Fatalf("register 0x15 power-on default = 0x%02X, want 0x08 (S L U)", got)
	}

	io.WritePort(nextRegSelectPort, 0x18)
	want := [4]uint8{0, 255, 0, 191}
	for i, w := range want {
		got := io.ReadPort(nextRegDataPort)
		if got != w {
			t.Fatalf("clip window byte %d = %d, want %d (power-on full-screen default)", i, got, w)
		}
		// A read must not advance the write cursor -- reading the same
		// index again should return the same byte, not the next one.
		if got2 := io.ReadPort(nextRegDataPort); got2 != w {
			t.Fatalf("clip window byte %d changed on a second read (%d then %d) -- read must not advance the cursor", i, got, got2)
		}
		// Advance by performing the write a real sequence would do,
		// confirming write does advance.
		io.WritePort(nextRegDataPort, w)
	}
}

// TestNextRegClipWindowSequence confirms sequential writes to 0x18 set
// X1, X2, Y1, Y2 (indices 0-3) in turn and wrap back to X1 (index 0) on
// a fifth write -- after writing 10,20,30,40,50, the cursor has
// advanced 0->1->2->3->0->1, so index 0 (X1) now holds the wrapped
// write (50) and index 1 (X2) still holds its own write (20), not yet
// overwritten by a sixth value.
func TestNextRegClipWindowSequence(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	io.WritePort(nextRegSelectPort, 0x18)

	seq := []uint8{10, 20, 30, 40, 50} // fifth value wraps around to X1
	for _, v := range seq {
		io.WritePort(nextRegDataPort, v)
	}

	io.WritePort(nextRegSelectPort, 0x18) // re-select; selection itself must not reset the cursor
	got := io.ReadPort(nextRegDataPort)
	if got != 20 {
		t.Fatalf("after writes (10,20,30,40,50), cursor should sit at index 1 (X2, still holding 20 from write #2): got %d", got)
	}
}

// TestNextRegMMUSlotsDefaultAndReadback confirms the eight MMU slot
// registers (0x50-0x57) boot to the documented default mapping and can
// be independently written and read back at the register-file level.
// T-28 item #26 wires these values to the actual paging model too
// (SpectrumMemory.nextSlots via SetNextSlot) -- see
// TestNextRegMMUWriteDrivesPaging and TestNextRegMMUBootDefaultsReachMemory
// below for that behavioural half; this test stays scoped to the
// register file itself.
func TestNextRegMMUSlotsDefaultAndReadback(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)

	wantDefaults := [8]uint8{0xFF, 0xFF, 10, 11, 4, 5, 0, 1}
	if got := io.GetMMUSlots(); got != wantDefaults {
		t.Fatalf("MMU slot defaults = %v, want %v", got, wantDefaults)
	}

	for i := 0; i < 8; i++ {
		reg := uint8(0x50 + i)
		val := uint8(100 + i)
		io.WritePort(nextRegSelectPort, reg)
		io.WritePort(nextRegDataPort, val)
	}

	slots := io.GetMMUSlots()
	for i := 0; i < 8; i++ {
		if slots[i] != uint8(100+i) {
			t.Fatalf("MMU slot %d = %d, want %d", i, slots[i], 100+i)
		}
	}
}

// TestNextRegUnimplementedRegisterStoreAndIgnore confirms a register
// number outside the implemented set stores whatever is written and
// reads it back, rather than erroring or silently discarding it --
// documented file behaviour, distinguishing "not yet covered" from
// "broken."
func TestNextRegUnimplementedRegisterStoreAndIgnore(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	const reg = 0x42 // not in nextRegDefaults

	if got := io.GetNextRegister(reg); got != 0x00 {
		t.Fatalf("unimplemented register 0x%02X default = 0x%02X, want 0x00", reg, got)
	}
	io.WritePort(nextRegSelectPort, reg)
	io.WritePort(nextRegDataPort, 0x77)
	if got := io.ReadPort(nextRegDataPort); got != 0x77 {
		t.Fatalf("unimplemented register 0x%02X after write = 0x%02X, want 0x77 (store-and-ignore)", reg, got)
	}
}

// TestNextRegMMUBootDefaultsReachMemory confirms T-28 item #26's other
// half: newNextRegisterFile's mmuBootDefaults must reach
// SpectrumMemory.nextSlots at NewSpectrumIO construction time, not just
// io.nextRegs.mmuSlots -- otherwise the paging model would silently
// disagree with the register file's own documented boot state (ROM
// sentinel in slots 0/1, classic 128K-layout mirror in slots 2-7) until
// the first NR 0x50-0x57 write happened to overwrite it.
func TestNextRegMMUBootDefaultsReachMemory(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	NewSpectrumIO(mem, nil)

	wantDefaults := [8]uint8{0xFF, 0xFF, 10, 11, 4, 5, 0, 1}
	if mem.nextSlots != wantDefaults {
		t.Fatalf("memory.nextSlots after construction = %v, want %v (must match newNextRegisterFile's mmuBootDefaults)", mem.nextSlots, wantDefaults)
	}
}

// TestNextRegMMUWriteDrivesPaging confirms T-28 item #26's core claim:
// a NextREG 0x50-0x57 write must change what SpectrumMemory.Read/Write
// actually see once Next mode is enabled, not merely update the
// mmuSlots readback mirror TestNextRegMMUSlotsDefaultAndReadback
// exercises. Uses the raw-port write path; TestNextRegMMUOpcodeWriteDrivesPaging
// below is the same proof via the NEXTREG opcode path.
func TestNextRegMMUWriteDrivesPaging(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	mem.EnableNext()

	// Slot 0 (0x0000-0x1FFF) currently defaults to the ROM sentinel
	// 0xFF (reserved -- reads 0xFF, drops writes per nextPageIsReserved).
	// Point it at extra-store page nextExtraPageBase instead via a raw
	// NR 0x50 write, then prove the memory-level Read/Write actually
	// follows that new mapping.
	io.WritePort(nextRegSelectPort, 0x50)
	io.WritePort(nextRegDataPort, nextExtraPageBase)

	if got := io.GetMMUSlots()[0]; got != nextExtraPageBase {
		t.Fatalf("mmuSlots[0] readback = %d, want %d (register file itself should still be correct)", got, nextExtraPageBase)
	}
	if got := mem.nextSlots[0]; got != nextExtraPageBase {
		t.Fatalf("memory.nextSlots[0] = %d, want %d -- NR 0x50 write must drive the actual paging model (T-28 item #26)", got, nextExtraPageBase)
	}

	mem.Write(0x0000, 0xAB)
	if got := mem.Read(0x0000); got != 0xAB {
		t.Fatalf("Read(0x0000) after NR 0x50 write + Write = 0x%02X, want 0xAB -- slot 0 must genuinely address the new page, not the stale ROM-sentinel default", got)
	}
	if got := mem.nextExtraRAM[0][0]; got != 0xAB {
		t.Fatalf("nextExtraRAM[0][0] = 0x%02X, want 0xAB -- confirms the write landed in the page the NR write named, not by coincidence", got)
	}
}

// TestNextRegMMUOpcodeWriteDrivesPaging is
// TestNextRegMMUWriteDrivesPaging's counterpart via the Z80N NEXTREG
// opcode path (nextRegOpcodeWrite) instead of the raw port sequence --
// confirming item #26's memory wiring is shared by nextRegWriteDirect
// regardless of which of the two callers (nextRegWriteSelected or
// nextRegOpcodeWrite) reached it, matching this file's existing
// raw-port/opcode-parity convention for other registers.
func TestNextRegMMUOpcodeWriteDrivesPaging(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	mem.EnableNext()

	io.nextRegOpcodeWrite(0x51, nextExtraPageBase+1)

	if got := mem.nextSlots[1]; got != nextExtraPageBase+1 {
		t.Fatalf("memory.nextSlots[1] after opcode-path write = %d, want %d", got, nextExtraPageBase+1)
	}

	mem.Write(0x2000, 0xCD)
	if got := mem.Read(0x2000); got != 0xCD {
		t.Fatalf("Read(0x2000) after opcode-path NR 0x51 write + Write = 0x%02X, want 0xCD", got)
	}
}

// TestPort7FFDDrivesNextLegacySlots67 confirms T-28 item #27's port-level
// wiring: on a genuine Next machine, a raw OUT to 0x7FFD both drives the
// classic SetPaging path (unchanged) AND recomposes Next MMU slots 6/7
// via SetNextLegacyBank7FFD, side by side, neither aware of the other.
func TestPort7FFDDrivesNextLegacySlots67(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.WritePort(0x7FFD, 0x03) // bank bits(2:0) = 3, also classic ramBankTop = 3

	if mem.ramBankTop != 3 {
		t.Fatalf("ramBankTop after WritePort(0x7FFD, 0x03) = %d, want 3 (classic SetPaging path must still work)", mem.ramBankTop)
	}
	if mem.nextSlots[6] != 6 || mem.nextSlots[7] != 7 {
		t.Fatalf("nextSlots[6:8] = [%d %d], want [6 7] (bank 3 composed with DFFD=0)", mem.nextSlots[6], mem.nextSlots[7])
	}
}

// TestPortDFFDDrivesNextLegacySlots67 confirms a raw OUT to 0xDFFD on a
// genuine Next machine supplies the extension bits recomposeNextLegacySlots
// combines with the last 0x7FFD write.
func TestPortDFFDDrivesNextLegacySlots67(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.WritePort(0x7FFD, 0x01) // bank bits(2:0) = 1
	io.WritePort(0xDFFD, 0x01) // DFFD bits(1:0) = 1 -> bank(4:3) = 01 -> bank = 1|8 = 9

	if mem.nextSlots[6] != 18 || mem.nextSlots[7] != 19 {
		t.Fatalf("nextSlots[6:8] = [%d %d], want [18 19] (bank 9 via 0x7FFD+0xDFFD)", mem.nextSlots[6], mem.nextSlots[7])
	}
}

// TestPortDFFDNoOpOnClassicMachine confirms 0xDFFD has zero paging effect
// when isNext is false -- SetNextLegacyPortDFFD's own no-op contract,
// proven this time through the actual port-dispatch entry point rather
// than calling the memory-layer setter directly.
func TestPortDFFDNoOpOnClassicMachine(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	// isNext left false.
	io := NewSpectrumIO(mem, nil)

	before := mem.nextSlots // mmuBootDefaults, pushed by NewSpectrumIO's
	// construction-time sync regardless of isNext (T-28 item #26) --
	// harmless since Read/Write never consult nextSlots unless isNext is
	// true, but this test must compare against THAT baseline, not zero.

	io.WritePort(0xDFFD, 0x0F)

	if mem.nextSlots != before {
		t.Fatalf("nextSlots changed from %v to %v after 0xDFFD write on a classic machine, want unchanged (no-op)", before, mem.nextSlots)
	}
}

// TestPortDFFDOnClassicMachinePreservesPreexistingAYCollision is a
// deliberate compatibility proof, not an endorsement: 0xDFFD has no
// defined role on any non-Next machine this emulator models, so it falls
// through to whatever zenzx's own coarse 0xC002==0xC000 AY register-select
// decode already did with that address BEFORE T-28 item #27 existed. The
// AY-collision fix added alongside the new 0xDFFD feature deliberately
// excludes ONLY the isNext=true case (see WritePort's AY register-select
// comment in io.go) -- this test pins the isNext=false side so a future
// change cannot silently alter classic-machine behaviour while "fixing"
// the collision further.
func TestPortDFFDOnClassicMachinePreservesPreexistingAYCollision(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	// isNext left false.
	io := NewSpectrumIO(mem, nil)

	io.WritePort(0xDFFD, 0x0A)

	if io.ayRegister != 0x0A {
		t.Fatalf("ayRegister after 0xDFFD write on a classic machine = 0x%02X, want 0x0A (pre-existing AY register-select collision must be UNCHANGED on non-Next machines)", io.ayRegister)
	}
}

// TestPortDFFDOnNextMachineDoesNotCorruptAYRegister is the actual
// regression proof for the AY-collision fix discovered while implementing
// T-28 item #27: on a genuine Next machine, a write to the literal address
// 0xDFFD must drive the new Next-mode legacy paging WITHOUT also being
// misread as an AY-3-8912 register-select write (0xC002==0xC000 also
// matches 0xDFFD). Without the fix, this test's io.ayRegister assertion
// would fail because the 0xDFFD write would silently overwrite it.
func TestPortDFFDOnNextMachineDoesNotCorruptAYRegister(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.WritePort(0xFFFD, 0x05) // genuine AY register select -> ayRegister = 5
	if io.ayRegister != 0x05 {
		t.Fatalf("ayRegister after WritePort(0xFFFD, 0x05) = 0x%02X, want 0x05 (sanity check before the real assertion)", io.ayRegister)
	}

	io.WritePort(0x7FFD, 0x07) // bank bits(2:0) = 7 -- sets up the max-bank case below

	io.WritePort(0xDFFD, 0x0F) // Next-mode legacy paging write -- must NOT touch ayRegister

	if io.ayRegister != 0x05 {
		t.Fatalf("ayRegister after Next-mode WritePort(0xDFFD, 0x0F) = 0x%02X, want unchanged 0x05 -- 0xDFFD must not be misread as an AY register-select write on a Next machine", io.ayRegister)
	}
	if mem.nextSlots[6] != 254 || mem.nextSlots[7] != 255 {
		t.Fatalf("nextSlots[6:8] = [%d %d], want [254 255] -- the 0xDFFD write must still genuinely drive Next-mode paging", mem.nextSlots[6], mem.nextSlots[7])
	}
}

// TestPortFFFDStillSelectsAYRegisterOnNextMachine confirms the AY-collision
// fix's exclusion is address-EXACT (port == 0xDFFD), not a blanket
// isNext-gated disabling of AY register select: the real AY register-select
// port 0xFFFD must keep working normally on a Next machine, exactly as on
// any classic machine.
func TestPortFFFDStillSelectsAYRegisterOnNextMachine(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.WritePort(0xFFFD, 0x0C)

	if io.ayRegister != 0x0C {
		t.Fatalf("ayRegister after WritePort(0xFFFD, 0x0C) on a Next machine = 0x%02X, want 0x0C (0xFFFD must be unaffected by the 0xDFFD-specific exclusion)", io.ayRegister)
	}
}

// TestT28CompatibilityRegressionClassicModels is T-28 item #28's
// "compatibility regression" half. Every earlier T-28 unit test (parts
// 1-3, items #19/#26/#27) proves non-interference by poking memory.go
// fields directly on a bare newTestMemoryAndScreen() instance. This test
// instead builds each classic machine through the SAME production entry
// point real -model=48k/128k/plus3 selection uses (NewZenZX +
// LoadROMBytes, which is what decides is128K/isPlus3 from ROM size) and
// drives it end-to-end through the real Z80 CPU executing actual paging
// (OUT) and memory (LD) machine code -- not Go-level Read/Write calls.
// EnableZ80N's own doc comment states no -model option enables Next
// mode; this test is the regression proof that claim rests on.
func TestT28CompatibilityRegressionClassicModels(t *testing.T) {
	cases := []struct {
		name    string
		rom     []byte
		pageOut bool // exercise the 0x7FFD paging port (128K/+3 only)
	}{
		{"48K", make([]byte, 16384), false},
		{"128K", make([]byte, 32768), true},
		{"+3", make([]byte, 65536), true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			zx := NewZenZX(AudioBackendOto)
			if zx.audio != nil {
				zx.audio.SetEnabled(false)
			}
			if err := zx.LoadROMBytes(c.rom); err != nil {
				t.Fatalf("LoadROMBytes: %v", err)
			}
			zx.cpu.Reset()

			if zx.memory.isNext {
				t.Fatalf("isNext = true immediately after LoadROMBytes(%s), want false", c.name)
			}
			bootSlots := zx.memory.nextSlots // whatever construction-time seeding left behind

			run := func(addr uint16, prog []uint8) {
				for i, b := range prog {
					zx.memory.Write(addr+uint16(i), b)
				}
				zx.cpu.PC = addr
				zx.cpu.Step()
			}

			if c.pageOut {
				run(0x8000, []uint8{0x01, 0xFD, 0x7F}) // LD BC,0x7FFD
				run(0x8010, []uint8{0x3E, 0x05})       // LD A,0x05
				run(0x8020, []uint8{0xED, 0x79})       // OUT (C),A -- page RAM bank 5 at 0xC000
			}
			run(0x8030, []uint8{0x3E, 0x99})       // LD A,0x99
			run(0x8040, []uint8{0x32, 0x00, 0xC0}) // LD (0xC000),A
			run(0x8050, []uint8{0x3E, 0x00})       // LD A,0x00 (clobber, so the next load proves something)
			run(0x8060, []uint8{0x3A, 0x00, 0xC0}) // LD A,(0xC000)

			if zx.cpu.A != 0x99 {
				t.Fatalf("%s: A after CPU-executed paging+LD round-trip = 0x%02X, want 0x99", c.name, zx.cpu.A)
			}
			if zx.memory.isNext {
				t.Fatalf("%s: isNext became true after running a classic-mode program, want still false", c.name)
			}
			if zx.memory.nextSlots != bootSlots {
				t.Fatalf("%s: nextSlots changed from %v to %v while running a classic-mode program, want unchanged", c.name, bootSlots, zx.memory.nextSlots)
			}
		})
	}
}

// TestT28NextModeMultiSlotProgram is T-28 item #28's "Next-mode
// multi-slot test program" half: a single real Z80N program, executed
// entirely through the CPU (no direct Go-level SetNextSlot/Read/Write
// calls standing in for what an actual program would do), that exercises
// items #19 (extra-store paging), #26 (NEXTREG-opcode-driven slots) and
// #27 (both the 0x50 legacy-ROM-defer sentinel AND the 0x7FFD/0xDFFD
// legacy-port slot 6/7 recomposition) TOGETHER against several different
// slots in one continuous scenario -- every earlier T-28 test exercises
// one mechanism at a time.
func TestT28NextModeMultiSlotProgram(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	zx.cpu.Reset()
	zx.memory.EnableNext()
	zx.EnableZ80N()

	// Seed a distinguishable byte in the classic ROM bank the item #27
	// legacy-ROM-defer path (slot 0) must read through -- there is no
	// real ROM loaded in this synthetic scenario, matching the existing
	// convention (e.g. TestNextReadWriteDefersToLegacyROMSlots0And1) of
	// direct field pokes for setup while the actual proof runs for real.
	zx.memory.rom[0][0x0000] = 0xAB

	run := func(addr uint16, prog []uint8) {
		for i, b := range prog {
			zx.memory.Write(addr+uint16(i), b)
		}
		zx.cpu.PC = addr
		zx.cpu.Step()
	}

	// NEXTREG $50,$FF -- slot 0 to the reserved ROM-defer sentinel
	// (item #26 opcode path driving item #27's special case).
	run(0x8000, []uint8{0xED, 0x91, 0x50, 0xFF})
	// NEXTREG $52,$10 -- slot 2 (register 0x50+2) to extra-store page 16
	// (item #26, #19).
	run(0x8010, []uint8{0xED, 0x91, 0x52, 0x10})
	// NEXTREG $53,$11 -- slot 3 (register 0x50+3) to extra-store page 17.
	run(0x8020, []uint8{0xED, 0x91, 0x53, 0x11})

	// Legacy port recomposition (item #27): 0x7FFD bits(2:0)=1,
	// 0xDFFD bits(1:0)=1 -> composed bank = 1|8 = 9 -> slots 6/7 = pages
	// 18/19.
	run(0x8030, []uint8{0x01, 0xFD, 0x7F}) // LD BC,0x7FFD
	run(0x8040, []uint8{0x3E, 0x01})       // LD A,0x01
	run(0x8050, []uint8{0xED, 0x79})       // OUT (C),A
	run(0x8060, []uint8{0x01, 0xFD, 0xDF}) // LD BC,0xDFFD
	run(0x8070, []uint8{0x3E, 0x01})       // LD A,0x01
	run(0x8080, []uint8{0xED, 0x79})       // OUT (C),A

	if got := zx.memory.nextSlots; got[0] != 0xFF || got[2] != 16 || got[3] != 17 || got[6] != 18 || got[7] != 19 {
		t.Fatalf("nextSlots after the setup program = %v, want [0xFF _ 16 17 _ _ 18 19]", got)
	}

	// Slot 0 (0x0000-0x1FFF): must defer to the classic ROM, not read
	// as an inactive/floating-bus reserved page.
	run(0x8090, []uint8{0x3A, 0x00, 0x00}) // LD A,(0x0000)
	if zx.cpu.A != 0xAB {
		t.Fatalf("A after LD A,(0x0000) with slot 0 = ROM-defer sentinel = 0x%02X, want 0xAB", zx.cpu.A)
	}

	// Slot 2 (0x4000-0x5FFF, extra-store page 16): real round-trip.
	run(0x80A0, []uint8{0x3E, 0x22})       // LD A,0x22
	run(0x80B0, []uint8{0x32, 0x00, 0x40}) // LD (0x4000),A
	run(0x80C0, []uint8{0x3E, 0x00})       // LD A,0x00 (clobber)
	run(0x80D0, []uint8{0x3A, 0x00, 0x40}) // LD A,(0x4000)
	if zx.cpu.A != 0x22 {
		t.Fatalf("A after slot-2 (page 16) round-trip = 0x%02X, want 0x22", zx.cpu.A)
	}
	if zx.memory.nextExtraRAM[16-nextExtraPageBase][0] != 0x22 {
		t.Fatalf("nextExtraRAM[%d][0] = 0x%02X, want 0x22 -- write must land in the page NEXTREG $02 named", 16-nextExtraPageBase, zx.memory.nextExtraRAM[16-nextExtraPageBase][0])
	}

	// Slot 3 (0x6000-0x7FFF, extra-store page 17): real round-trip.
	run(0x80E0, []uint8{0x3E, 0x33})       // LD A,0x33
	run(0x80F0, []uint8{0x32, 0x00, 0x60}) // LD (0x6000),A
	run(0x8100, []uint8{0x3E, 0x00})       // LD A,0x00 (clobber)
	run(0x8110, []uint8{0x3A, 0x00, 0x60}) // LD A,(0x6000)
	if zx.cpu.A != 0x33 {
		t.Fatalf("A after slot-3 (page 17) round-trip = 0x%02X, want 0x33", zx.cpu.A)
	}

	// Slot 6 (0xC000-0xDFFF, page 18 via the 0x7FFD/0xDFFD legacy-port
	// recomposition): real round-trip -- the item #27 proof that
	// matters most, since it is driven purely by real port I/O.
	run(0x8120, []uint8{0x3E, 0x44})       // LD A,0x44
	run(0x8130, []uint8{0x32, 0x00, 0xC0}) // LD (0xC000),A
	run(0x8140, []uint8{0x3E, 0x00})       // LD A,0x00 (clobber)
	run(0x8150, []uint8{0x3A, 0x00, 0xC0}) // LD A,(0xC000)
	if zx.cpu.A != 0x44 {
		t.Fatalf("A after slot-6 (legacy-port-composed page 18) round-trip = 0x%02X, want 0x44", zx.cpu.A)
	}
	if zx.memory.nextExtraRAM[18-nextExtraPageBase][0] != 0x44 {
		t.Fatalf("nextExtraRAM[%d][0] = 0x%02X, want 0x44 -- write must land in the page the 0x7FFD/0xDFFD ports composed", 18-nextExtraPageBase, zx.memory.nextExtraRAM[18-nextExtraPageBase][0])
	}
}
