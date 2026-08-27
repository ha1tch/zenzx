package main

import "testing"

// TestIs48KROMActive checks the ROM-context guard against real,
// verified per-model bank assignments (memory.go's own doc comment
// cites the exact check that produced these: rom/48.rom, rom/128-1.rom,
// rom/plus3-3.rom all byte-for-byte match at the LD-BYTES/SA-BYTES
// addresses; rom/128-0.rom, rom/plus3-0/1/2.rom do not).
func TestIs48KROMActive(t *testing.T) {
	cases := []struct {
		name            string
		is128K, isPlus3 bool
		isTS2068        bool
		romBank         uint8
		want            bool
	}{
		{"48K (only bank)", false, false, false, 0, true},
		{"128K bank 1 (48K-compatible)", true, false, false, 1, true},
		{"128K bank 0 (128K editor, not compatible)", true, false, false, 0, false},
		{"+3 bank 3 (48K-compatible)", true, true, false, 3, true},
		{"+3 bank 0 (not compatible)", true, true, false, 0, false},
		{"+3 bank 1 (not compatible)", true, true, false, 1, false},
		{"TS2068 always excluded regardless of other flags", false, false, true, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			screen := NewSpectrumScreen()
			m := NewSpectrumMemory(screen)
			m.is128K = c.is128K
			m.isPlus3 = c.isPlus3
			m.isTS2068 = c.isTS2068
			m.romBank = c.romBank
			if got := m.is48KROMActive(); got != c.want {
				t.Errorf("is48KROMActive() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestFastLoaderTrapPoints guards the two trap constants against silent
// drift -- both were verified directly against rom/48.rom's actual
// bytes (not copied from documentation or another emulator's own
// internal timing offsets), and a change here should be a deliberate,
// re-verified decision, not an accident.
func TestFastLoaderTrapPoints(t *testing.T) {
	if ldBytesTrapPC != 0x056C {
		t.Errorf("ldBytesTrapPC = 0x%04X, want 0x056C", ldBytesTrapPC)
	}
	if saBytesTrapPC != 0x04C2 {
		t.Errorf("saBytesTrapPC = 0x%04X, want 0x04C2", saBytesTrapPC)
	}
}

// newFastLoaderTestSetup builds a minimal ZenZX (no ROM file needed --
// these are Go-level register/memory assertions on trapLoad itself, not
// tests of real Z80 execution) with a single tape block loaded, ready
// for a trapLoad call.
func newFastLoaderTestSetup(t *testing.T, blockData []byte) (*ZenZX, *Tape) {
	t.Helper()
	zx := NewZenZX(AudioBackendOto)
	tp := &Tape{
		zx: zx,
		st: &TapeState{
			Loaded: true,
			Mode:   TapeFast,
			Blocks: []TapeBlock{{Data: blockData}},
		},
	}
	zx.tape = tp
	return zx, tp
}

// setUpLoadCall configures CPU state exactly matching what the real
// LD-BYTES routine has already established by ldBytesTrapPC (verified
// against rom/48.rom): EX AF,AF' has run, so the caller's original flag
// byte and LOAD/VERIFY carry are in the shadow registers; IX/DE hold
// the destination and length; SP points at a return address.
func setUpLoadCall(zx *ZenZX, flag uint8, verify bool, dest, length uint16, returnAddr uint16) {
	zx.cpu.A_ = flag
	if verify {
		zx.cpu.F_ &^= 0x01
	} else {
		zx.cpu.F_ |= 0x01
	}
	zx.cpu.SetIX(dest)
	zx.cpu.D = uint8(length >> 8)
	zx.cpu.E = uint8(length)
	zx.cpu.SP = 0xFF00
	zx.memory.Write(zx.cpu.SP, uint8(returnAddr))
	zx.memory.Write(zx.cpu.SP+1, uint8(returnAddr>>8))
	zx.cpu.PC = ldBytesTrapPC
}

// buildTapeBlock constructs a real, correctly-checksummed tape block:
// flag byte, payload, then the running XOR checksum -- matching the
// real Spectrum tape block format trapLoad verifies against.
func buildTapeBlock(flag uint8, payload []byte) []byte {
	block := make([]byte, 0, len(payload)+2)
	block = append(block, flag)
	checksum := flag
	for _, b := range payload {
		block = append(block, b)
		checksum ^= b
	}
	block = append(block, checksum)
	return block
}

func TestFastLoaderLoadSuccess(t *testing.T) {
	payload := []byte{0x11, 0x22, 0x33, 0x44}
	block := buildTapeBlock(0xFF, payload)
	zx, tp := newFastLoaderTestSetup(t, block)
	fl := &FastLoader{Enabled: true}

	const dest = 0x8000
	setUpLoadCall(zx, 0xFF, false, dest, uint16(len(payload)), 0x9000)

	if !fl.TryIntercept(zx, tp) {
		t.Fatal("TryIntercept did not fire at the trap point")
	}

	for i, want := range payload {
		if got := zx.memory.Read(dest + uint16(i)); got != want {
			t.Errorf("byte %d = 0x%02X, want 0x%02X", i, got, want)
		}
	}
	if zx.cpu.F&0x01 == 0 {
		t.Error("Carry not set -- want success")
	}
	if zx.cpu.PC != 0x9000 {
		t.Errorf("PC = 0x%04X, want 0x9000 (return address)", zx.cpu.PC)
	}
	if zx.cpu.SP != 0xFF02 {
		t.Errorf("SP = 0x%04X, want 0xFF02 (popped by 2)", zx.cpu.SP)
	}
	if tp.st.Position != 1 {
		t.Errorf("Position = %d, want 1 (advanced past the loaded block)", tp.st.Position)
	}
}

func TestFastLoaderVerifySuccess(t *testing.T) {
	payload := []byte{0xAA, 0xBB, 0xCC}
	block := buildTapeBlock(0x00, payload)
	zx, tp := newFastLoaderTestSetup(t, block)
	fl := &FastLoader{Enabled: true}

	const dest = 0x8000
	for i, b := range payload {
		zx.memory.Write(dest+uint16(i), b) // memory already matches -- VERIFY should succeed
	}
	setUpLoadCall(zx, 0x00, true, dest, uint16(len(payload)), 0x9000)

	if !fl.TryIntercept(zx, tp) {
		t.Fatal("TryIntercept did not fire")
	}
	if zx.cpu.F&0x01 == 0 {
		t.Error("Carry not set -- VERIFY should have succeeded against matching memory")
	}
}

func TestFastLoaderVerifyMismatch(t *testing.T) {
	payload := []byte{0xAA, 0xBB, 0xCC}
	block := buildTapeBlock(0x00, payload)
	zx, tp := newFastLoaderTestSetup(t, block)
	fl := &FastLoader{Enabled: true}

	const dest = 0x8000
	zx.memory.Write(dest, 0xAA)
	zx.memory.Write(dest+1, 0x00) // deliberately wrong -- want 0xBB
	zx.memory.Write(dest+2, 0xCC)
	setUpLoadCall(zx, 0x00, true, dest, uint16(len(payload)), 0x9000)

	if !fl.TryIntercept(zx, tp) {
		t.Fatal("TryIntercept did not fire")
	}
	if zx.cpu.F&0x01 != 0 {
		t.Error("Carry set -- want clear, VERIFY should have failed on the mismatch")
	}
	// Real hardware aborts on the first mismatch (confirmed against
	// rom/48.rom's own VERIFY loop: "XOR L; RET NZ" breaks immediately)
	// -- byte 2 (0xCC), past the mismatch, must be untouched.
	if got := zx.memory.Read(dest + 2); got != 0xCC {
		t.Errorf("byte 2 = 0x%02X, want unchanged 0xCC (loop should abort at the first mismatch)", got)
	}
}

func TestFastLoaderWrongFlagByte(t *testing.T) {
	payload := []byte{0x01, 0x02}
	block := buildTapeBlock(0x00, payload) // block is a header (flag 0x00)
	zx, tp := newFastLoaderTestSetup(t, block)
	fl := &FastLoader{Enabled: true}

	const dest = 0x8000
	zx.memory.Write(dest, 0x99)                                        // sentinel -- must not get overwritten
	setUpLoadCall(zx, 0xFF, false, dest, uint16(len(payload)), 0x9000) // caller wants a DATA block (0xFF), not this header

	if !fl.TryIntercept(zx, tp) {
		t.Fatal("TryIntercept did not fire")
	}
	if zx.cpu.F&0x01 != 0 {
		t.Error("Carry set -- want clear, flag byte did not match what the caller asked for")
	}
	if got := zx.memory.Read(dest); got != 0x99 {
		t.Errorf("memory at dest = 0x%02X, want untouched sentinel 0x99 (should not write on a flag mismatch)", got)
	}
	if tp.st.Position != 1 {
		t.Errorf("Position = %d, want 1 (still advances past a non-matching block, matching real hardware retry behaviour)", tp.st.Position)
	}
}

func TestFastLoaderWrongROMContext(t *testing.T) {
	payload := []byte{0x01}
	block := buildTapeBlock(0xFF, payload)
	zx, tp := newFastLoaderTestSetup(t, block)
	fl := &FastLoader{Enabled: true}
	zx.memory.is128K = true
	zx.memory.romBank = 0 // 128K editor ROM -- NOT the 48K-compatible bank

	setUpLoadCall(zx, 0xFF, false, 0x8000, 1, 0x9000)

	if fl.TryIntercept(zx, tp) {
		t.Error("TryIntercept fired with the wrong ROM bank paged in -- should decline")
	}
}

func TestFastLoaderDisabled(t *testing.T) {
	payload := []byte{0x01}
	block := buildTapeBlock(0xFF, payload)
	zx, tp := newFastLoaderTestSetup(t, block)
	fl := &FastLoader{Enabled: false}

	setUpLoadCall(zx, 0xFF, false, 0x8000, 1, 0x9000)

	if fl.TryIntercept(zx, tp) {
		t.Error("TryIntercept fired while disabled")
	}
}

// TestFastLoaderRealROMIntegration is the definitive proof: not a
// synthetic Go-level call into trapLoad, but a hand-assembled program
// executed via real zx.cpu.Step() calls that CALLs the genuine
// rom/48.rom LD-BYTES entry point (0x0556) directly. Real ROM code
// runs the EX AF,AF' register swap, the border flash, and the BREAK-key
// check for real -- exactly the housekeeping a naive entry-point trap
// (the old code's approach) would have skipped -- before naturally
// reaching 0x056C, where the Go-level trap takes over. Confirms the
// whole path end to end: real ROM prelude -> trap fires -> real
// CALL-pushed return address gets used correctly.
func TestFastLoaderRealROMIntegration(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	block := buildTapeBlock(0xFF, payload)
	tp := &Tape{
		zx: zx,
		st: &TapeState{
			Loaded:  true,
			Mode:    TapeFast,
			Playing: true,
			Blocks:  []TapeBlock{{Data: block}},
		},
	}
	zx.tape = tp
	zx.tape.fl = &FastLoader{Enabled: true}

	dest := uint16(0x9000)
	// LD SP,0xFF00 / LD A,0xFF / SCF / LD IX,dest / LD DE,len(payload) /
	// CALL 0x0556 (real LD-BYTES entry) / JR $
	prog := []byte{
		0x31, 0x00, 0xFF, // LD SP,0xFF00
		0x3E, 0xFF, // LD A,0xFF (flag byte -- data block)
		0x37,                                    // SCF (Carry set -- LOAD, not VERIFY)
		0xDD, 0x21, byte(dest), byte(dest >> 8), // LD IX,dest
		0x11, byte(len(payload)), 0x00, // LD DE,len(payload)
		0xCD, 0x56, 0x05, // CALL 0x0556 (real LD-BYTES)
		0x18, 0xFE, // JR $
	}
	zx.memory.Load(0x8000, prog)
	zx.cpu.PC = 0x8000

	const doneAddr = 0x8010
	reached := false
	for i := 0; i < 2_000_000; i++ {
		cycles := zx.cpu.Step()
		zx.tape.Tick(cycles)
		if zx.cpu.PC == doneAddr {
			reached = true
			break
		}
	}
	if !reached {
		t.Fatalf("program did not reach the done loop (PC=0x%04X) -- real ROM code never reached the trap, or the trap did not return correctly", zx.cpu.PC)
	}

	for i, want := range payload {
		if got := zx.memory.Read(dest + uint16(i)); got != want {
			t.Errorf("byte %d = 0x%02X, want 0x%02X", i, got, want)
		}
	}
	if zx.cpu.F&0x01 == 0 {
		t.Error("Carry not set after real-ROM-driven load -- want success")
	}
}

// TestTS2068ExtensionROMActive checks the chunk-0-aware context guard:
// TS2068's tape trap addresses only mean anything when chunk 0 is
// genuinely switched to Extension ROM, not merely because the model is
// TS2068 -- ordinary Home Bank execution (the vast majority of the
// time) must not risk misreading 0x0112/0x0068 as tape routines just
// because they happen to be low addresses.
func TestTS2068ExtensionROMActive(t *testing.T) {
	cases := []struct {
		name                          string
		isTS2068, chunk0, exromSelect bool
		want                          bool
	}{
		{"not TS2068 at all", false, true, true, false},
		{"TS2068, Home Bank (normal case)", true, false, false, false},
		{"TS2068, chunk0 selected but Dock not Extension", true, true, false, false},
		{"TS2068, Extension selected but chunk0 not switched out", true, false, true, false},
		{"TS2068, genuinely in Extension ROM", true, true, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			screen := NewSpectrumScreen()
			m := NewSpectrumMemory(screen)
			m.isTS2068 = c.isTS2068
			m.ts2068HSRChunk0 = c.chunk0
			m.ts2068ExRomSelect = c.exromSelect
			if got := m.isTS2068ExtensionROMActive(); got != c.want {
				t.Errorf("isTS2068ExtensionROMActive() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTS2068TapeTrapPoints(t *testing.T) {
	if ts2068RTapeTrapPC != 0x0112 {
		t.Errorf("ts2068RTapeTrapPC = 0x%04X, want 0x0112", ts2068RTapeTrapPC)
	}
	if ts2068WTapeTrapPC != 0x0068 {
		t.Errorf("ts2068WTapeTrapPC = 0x%04X, want 0x0068", ts2068WTapeTrapPC)
	}
}

// TestTS2068FastLoaderWrongContext confirms the trap declines outside
// genuine Extension-ROM execution, even for a TS2068 ZenZX -- PC
// happening to equal 0x0112 during ordinary Home ROM code (which can
// legitimately happen; it's just a low address) must not fire.
func TestTS2068FastLoaderWrongContext(t *testing.T) {
	payload := []byte{0x01}
	block := buildTapeBlock(0xFF, payload)
	zx, tp := newFastLoaderTestSetup(t, block)
	zx.memory.isTS2068 = true
	zx.memory.ts2068HSRChunk0 = false // Home Bank, not Extension
	fl := &FastLoader{Enabled: true}

	zx.cpu.PC = ts2068RTapeTrapPC
	if fl.TryIntercept(zx, tp) {
		t.Error("TryIntercept fired while chunk 0 was Home Bank, not Extension ROM")
	}
}

// TestTS2068FastLoaderRealROMIntegration is the definitive proof for
// TS2068, mirroring TestFastLoaderRealROMIntegration: a hand-assembled
// program executed via real zx.cpu.Step() calls that engages chunk-0
// Extension ROM banking itself (the same IFRTN-style port F4H/FFH
// sequence real Extension ROM Interface Routine calls use, confirmed
// working since T-14/AMX mouse and Stage 3's CHNG_VID work) and CALLs
// the genuine rom/ts2068-1.rom R_TAPE entry point (0x00FC) directly.
// Real ROM code runs its own EX AF,AF', border flash, and BREAK check
// before naturally reaching 0x0112, where the Go-level trap -- the
// exact same trapLoad used for the standard 48K path -- takes over.
func TestTS2068FastLoaderRealROMIntegration(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadTS2068ROM("./rom/ts2068-0.rom", "./rom/ts2068-1.rom"); err != nil {
		t.Fatalf("LoadTS2068ROM: %v", err)
	}
	payload := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	block := buildTapeBlock(0xFF, payload)
	tp := &Tape{
		zx: zx,
		st: &TapeState{
			Loaded:  true,
			Mode:    TapeFast,
			Playing: true,
			Blocks:  []TapeBlock{{Data: block}},
		},
	}
	zx.tape = tp
	zx.tape.fl = &FastLoader{Enabled: true}

	dest := uint16(0x8800)
	// LD SP,0xFF00 / switch chunk 0 to Extension (real IFRTN-style
	// sequence: SET 7 on port FF, then 1 to port F4) / LD A,0xFF /
	// SCF / LD IX,dest / LD DE,len(payload) / CALL 0x00FC (real
	// R_TAPE entry) / JR $
	prog := []byte{
		0x31, 0x00, 0xFF, // LD SP,0xFF00
		0xDB, 0xFF, // IN A,(0xFF)
		0xCB, 0xFF, // SET 7,A
		0xD3, 0xFF, // OUT (0xFF),A
		0x3E, 0x01, // LD A,1
		0xD3, 0xF4, // OUT (0xF4),A  -- chunk 0 now Extension ROM
		0x3E, 0xFF, // LD A,0xFF (flag byte -- data block)
		0x37,                                    // SCF (Carry set -- LOAD, not VERIFY)
		0xDD, 0x21, byte(dest), byte(dest >> 8), // LD IX,dest
		0x11, byte(len(payload)), 0x00, // LD DE,len(payload)
		0xCD, 0xFC, 0x00, // CALL 0x00FC (real R_TAPE)
		0x18, 0xFE, // JR $
	}
	zx.memory.Load(0x8000, prog)
	zx.cpu.PC = 0x8000

	doneAddr := uint16(0x8000 + len(prog) - 2)
	reached := false
	for i := 0; i < 2_000_000; i++ {
		cycles := zx.cpu.Step()
		zx.tape.Tick(cycles)
		if zx.cpu.PC == doneAddr {
			reached = true
			break
		}
	}
	if !reached {
		t.Fatalf("program did not reach the done loop (PC=0x%04X) -- real R_TAPE never reached the trap, or the trap did not return correctly", zx.cpu.PC)
	}

	for i, want := range payload {
		if got := zx.memory.Read(dest + uint16(i)); got != want {
			t.Errorf("byte %d = 0x%02X, want 0x%02X", i, got, want)
		}
	}
	if zx.cpu.F&0x01 == 0 {
		t.Error("Carry not set after real-R_TAPE-driven load -- want success")
	}
	// After the trap fires, chunk 0 is still Extension-ROM-selected --
	// trapLoad doesn't touch F4H/FFH itself (real ROM code did that
	// before the trap, and the JR $ parking loop, being in RAM at
	// 0x8000+, doesn't need Home ROM back). Confirms the trap doesn't
	// silently corrupt banking state it isn't responsible for.
	if !zx.memory.ts2068HSRChunk0 || !zx.memory.ts2068ExRomSelect {
		t.Error("chunk-0 Extension ROM state changed unexpectedly by trapLoad")
	}
}

// TestTS2068AccurateModeRealROMIntegration is Stage 5's other half: not
// the fast-trap short-circuit, but genuine pulse-by-pulse accurate-mode
// loading through the real R_TAPE routine, exactly as the plan
// predicted -- "should already work once Stage 1 is solid" -- now
// actually verified rather than assumed. No TS2068-specific code exists
// for this path at all: port 0xFE is reused unchanged (Stage 1), and
// Tick()'s pulse-advancement logic has never been model-aware. Uses the
// same genPulses the real .tap loading pipeline uses (not a synthetic
// pulse train), and the fast loader is deliberately not attached, so
// the full, slow, real pilot/sync/data polling loop runs to completion
// -- confirmed practical (809,629 steps, ~30ms) rather than assumed
// impractically slow.
func TestTS2068AccurateModeRealROMIntegration(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadTS2068ROM("./rom/ts2068-0.rom", "./rom/ts2068-1.rom"); err != nil {
		t.Fatalf("LoadTS2068ROM: %v", err)
	}

	payload := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	block := buildTapeBlock(0xFF, payload)
	tp := &Tape{zx: zx, st: &TapeState{}}
	pulses := tp.genPulses(block)
	tp.st.Loaded = true
	tp.st.Mode = TapeAccurate
	tp.st.Playing = true
	tp.st.Pulses = pulses
	zx.tape = tp // fl deliberately left nil -- no fast-load shortcut available

	dest := uint16(0x8800)
	// Same chunk-0-Extension-ROM engagement and R_TAPE call as the fast-
	// mode integration test above, but with real pulses to poll instead
	// of a fast loader to intercept them.
	prog := []byte{
		0x31, 0x00, 0xFF, // LD SP,0xFF00
		0xDB, 0xFF, 0xCB, 0xFF, 0xD3, 0xFF, // engage chunk-0 Extension ROM
		0x3E, 0x01, 0xD3, 0xF4,
		0x3E, 0xFF, 0x37, // LD A,0xFF / SCF
		0xDD, 0x21, byte(dest), byte(dest >> 8), // LD IX,dest
		0x11, byte(len(payload)), 0x00, // LD DE,len(payload)
		0xCD, 0xFC, 0x00, // CALL 0x00FC (real R_TAPE, full pulse polling)
		0x18, 0xFE, // JR $
	}
	zx.memory.Load(0x8000, prog)
	zx.cpu.PC = 0x8000
	doneAddr := uint16(0x8000 + len(prog) - 2)

	reached := false
	for i := 0; i < 5_000_000; i++ {
		cycles := zx.cpu.Step()
		zx.tape.Tick(cycles)
		if zx.cpu.PC == doneAddr {
			reached = true
			break
		}
	}
	if !reached {
		t.Fatalf("program did not reach the done loop (PC=0x%04X) -- accurate-mode pulse polling did not complete", zx.cpu.PC)
	}

	for i, want := range payload {
		if got := zx.memory.Read(dest + uint16(i)); got != want {
			t.Errorf("byte %d = 0x%02X, want 0x%02X", i, got, want)
		}
	}
	if zx.cpu.F&0x01 == 0 {
		t.Error("Carry not set after accurate-mode load -- want success")
	}
}

func TestFastLoaderEndOfTape(t *testing.T) {
	payload := []byte{0x01, 0x02}
	block := buildTapeBlock(0xFF, payload)
	zx, tp := newFastLoaderTestSetup(t, block) // only one block
	fl := &FastLoader{Enabled: true}
	tp.st.Playing = true

	setUpLoadCall(zx, 0xFF, false, 0x8000, uint16(len(payload)), 0x9000)
	if !fl.TryIntercept(zx, tp) {
		t.Fatal("the only block should load")
	}
	if tp.st.Playing {
		t.Error("Playing still true after loading the only block on the tape -- want it cleared at genuine end of tape")
	}
}

// TestFastLoaderMultiBlockContinues guards against the bug this
// hardening pass fixed: the old TryIntercept set Playing=false after
// *every* successful trap, which broke ordinary multi-block loading
// (header block, then data block, each requiring its own separate call
// into LD-BYTES) -- the second block's LD-BYTES call would find
// Playing already false and Tick would return immediately without ever
// checking the trap.
func TestFastLoaderMultiBlockContinues(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	tp := &Tape{
		zx: zx,
		st: &TapeState{
			Loaded:  true,
			Mode:    TapeFast,
			Playing: true,
			Blocks: []TapeBlock{
				{Data: buildTapeBlock(0x00, []byte{0xAA}), IsHeader: true},
				{Data: buildTapeBlock(0xFF, []byte{0xBB, 0xCC})},
			},
		},
	}
	zx.tape = tp
	fl := &FastLoader{Enabled: true}

	setUpLoadCall(zx, 0x00, false, 0x8000, 1, 0x9000)
	if !fl.TryIntercept(zx, tp) {
		t.Fatal("first (header) block should load")
	}
	if !tp.st.Playing {
		t.Fatal("Playing cleared after the first of two blocks -- second block would never get a chance to load")
	}
	if tp.st.Position != 1 {
		t.Fatalf("Position = %d, want 1 after the first block", tp.st.Position)
	}

	setUpLoadCall(zx, 0xFF, false, 0x8100, 2, 0x9100)
	if !fl.TryIntercept(zx, tp) {
		t.Fatal("second (data) block should also load")
	}
	if tp.st.Playing {
		t.Error("Playing still true after the second and last block -- want it cleared now")
	}
	if got := zx.memory.Read(0x8100); got != 0xBB {
		t.Errorf("second block byte 0 = 0x%02X, want 0xBB", got)
	}
}
