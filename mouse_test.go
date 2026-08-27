package main

import "testing"

func TestParseMouseMode(t *testing.T) {
	cases := []struct {
		input   string
		want    MouseMode
		wantErr bool
	}{
		{"none", MouseNone, false},
		{"", MouseNone, false},
		{"kempston", MouseKempston, false},
		{"Kempston", MouseKempston, false}, // case-insensitive
		{"amx", MouseAMX, false},
		{"AMX", MouseAMX, false},
		{"garbage", MouseNone, true},
	}
	for _, c := range cases {
		got, err := ParseMouseMode(c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseMouseMode(%q) = nil error, want an error", c.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseMouseMode(%q) unexpected error: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseMouseMode(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestKempstonMouseButtonByte(t *testing.T) {
	cases := []struct {
		name  string
		state MouseState
		want  uint8
	}{
		{"nothing pressed", MouseState{}, 0xFF},
		{"right", MouseState{Right: true}, 0xFF &^ 0x01},
		{"left", MouseState{Left: true}, 0xFF &^ 0x02},
		{"middle", MouseState{Middle: true}, 0xFF &^ 0x04},
		{"left+right", MouseState{Left: true, Right: true}, 0xFF &^ 0x01 &^ 0x02},
		{"all three", MouseState{Left: true, Right: true, Middle: true}, 0xFF &^ 0x07},
	}
	for _, c := range cases {
		if got := kempstonMouseButtonByte(c.state); got != c.want {
			t.Errorf("%s: kempstonMouseButtonByte = 0x%02X, want 0x%02X", c.name, got, c.want)
		}
	}
}

// TestKempstonMouseDefaultButtons checks the constructor initialises the
// button byte to 0xFF (nothing pressed under active-low semantics), not
// Go's zero value 0x00 -- which would mean "everything pressed", a real
// bug this test exists specifically to catch.
func TestKempstonMouseDefaultButtons(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseKempston)

	got := io.ReadPort(0xFADF)
	if got != 0xFF {
		t.Errorf("default mouse button port = 0x%02X, want 0xFF (nothing pressed, active low)", got)
	}
}

// TestKempstonMousePortDecode checks the full path through ReadPort for
// all three ports, including the partial-decode mask (upper 4 address
// bits don't-care on real hardware) by probing an aliased address.
func TestKempstonMousePortDecode(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseKempston)

	io.SetMouseState(MouseState{DeltaX: 5, DeltaY: -3, Left: true})

	if got := io.ReadPort(0xFBDF); got != 5 {
		t.Errorf("X port = %d, want 5", got)
	}
	var negThree int32 = -3
	wantY := uint8(negThree) // wraps to 253 at runtime -- not expressible as a Go constant conversion
	if got := io.ReadPort(0xFFDF); got != wantY {
		t.Errorf("Y port = %d, want %d (wrapped -3)", got, wantY)
	}
	if got := io.ReadPort(0xFADF); got != 0xFF&^0x02 {
		t.Errorf("buttons port = 0x%02X, want 0x%02X (left pressed)", got, 0xFF&^0x02)
	}

	// Aliased address (upper 4 bits differ) must decode identically --
	// real hardware ignores those bits per the documented partial decode.
	if got := io.ReadPort(0x0BDF); got != 5 {
		t.Errorf("aliased X port (0x0BDF) = %d, want 5 (upper address bits are don't-care)", got)
	}
}

// TestKempstonMouseWraps checks the 8-bit counter wraps at the byte
// boundary rather than saturating or overflowing into another field --
// exactly the real 74LS191 counter's behaviour (k1.spdns.de's Kempston
// Mouse Interface page).
func TestKempstonMouseWraps(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseKempston)

	io.SetMouseState(MouseState{DeltaX: 250})
	io.SetMouseState(MouseState{DeltaX: 10}) // 250+10 = 260, wraps to 4
	if got := io.ReadPort(0xFBDF); got != 4 {
		t.Errorf("X after wraparound = %d, want 4", got)
	}
}

// TestKempstonMouseFractionalAccumulation checks sub-1.0 per-frame deltas
// (the common case at high magnification, once divided by the display
// multiplier) are not silently discarded -- they must accumulate across
// frames and eventually register, not truncate to zero forever.
func TestKempstonMouseFractionalAccumulation(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseKempston)

	// 0.3 per frame: truncates to 0 every single frame if not accumulated,
	// but 4 frames of 0.3 sums to 1.2, which should register as +1.
	for i := 0; i < 4; i++ {
		io.SetMouseState(MouseState{DeltaX: 0.3})
	}
	if got := io.ReadPort(0xFBDF); got != 1 {
		t.Errorf("X after 4x0.3 delta = %d, want 1 (fractional accumulation)", got)
	}
}

// TestMouseNoneIsNoOp confirms the default mode touches none of the
// mouse ports -- matches joystick.go's equivalent guarantee.
func TestMouseNoneIsNoOp(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	// MouseNone is the zero value -- deliberately not calling SetMouseMode.

	io.SetMouseState(MouseState{DeltaX: 10, DeltaY: 10, Left: true})

	if got := io.ReadPort(0xFBDF); got != 0 {
		t.Errorf("X port with MouseNone = %d, want 0 (untouched)", got)
	}
	if got := io.ReadPort(0xFADF); got != 0xFF {
		t.Errorf("buttons port with MouseNone = 0x%02X, want 0xFF (untouched, default)", got)
	}
}

// TestAMXMouseButtonByte checks the bit assignment (5/6/7, active high)
// against the disassembly (docs/TRACKING.md T-14): AMX Art polls each
// bit waiting for it to become 1, a different layout and polarity from
// Kempston Mouse's bits 0-2 active-low on a different port.
func TestAMXMouseButtonByte(t *testing.T) {
	cases := []struct {
		name  string
		state MouseState
		want  uint8
	}{
		{"nothing pressed", MouseState{}, 0x00},
		{"left", MouseState{Left: true}, 0x80},
		{"right", MouseState{Right: true}, 0x40},
		{"middle", MouseState{Middle: true}, 0x20},
		{"all three", MouseState{Left: true, Right: true, Middle: true}, 0xE0},
	}
	for _, c := range cases {
		if got := amxMouseButtonByte(c.state); got != c.want {
			t.Errorf("%s: amxMouseButtonByte = 0x%02X, want 0x%02X", c.name, got, c.want)
		}
	}
}

// TestAMXButtonsPolledNoInterrupt confirms button state is available
// immediately via port 0xDF with no interrupt needed -- the disassembly
// showed buttons are polled, not interrupt-driven, unlike X/Y.
func TestAMXButtonsPolledNoInterrupt(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseAMX)

	io.SetMouseState(MouseState{Left: true})

	if got := io.ReadPort(0xDF); got != 0x80 {
		t.Errorf("port 0xDF = 0x%02X, want 0x80 (left button, no interrupt needed)", got)
	}
	if len(io.amxIntQueue) != 0 {
		t.Errorf("button-only state change queued %d interrupts, want 0", len(io.amxIntQueue))
	}
}

// TestAMXQueuesOneStepPerWholeDelta checks whole-number deltas queue
// exactly the right number of steps, alternating axes as expected, and
// that the port value carries the correct direction bit.
func TestAMXQueuesOneStepPerWholeDelta(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseAMX)

	io.SetMouseState(MouseState{DeltaX: 3}) // three whole steps, positive direction
	if len(io.amxIntQueue) != 3 {
		t.Fatalf("queued %d requests for DeltaX=3, want 3", len(io.amxIntQueue))
	}
	for i, req := range io.amxIntQueue {
		if req.vector != amxVectorX {
			t.Errorf("request %d vector = 0x%02X, want X vector 0x%02X", i, req.vector, amxVectorX)
		}
		if req.portValue&1 != 0 {
			t.Errorf("request %d portValue bit0 = 1, want 0 (positive direction)", i)
		}
	}
}

// TestAMXFractionalAccumulation mirrors Kempston's equivalent test: a
// sub-1.0 per-frame delta (the common case at high magnification) must
// accumulate across calls, not silently vanish to truncation.
func TestAMXFractionalAccumulation(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseAMX)

	for i := 0; i < 4; i++ {
		io.SetMouseState(MouseState{DeltaX: 0.3})
	}
	// 4 x 0.3 = 1.2 -- exactly one step should have queued by now.
	if len(io.amxIntQueue) != 1 {
		t.Errorf("queued %d requests after 4x0.3 delta, want 1", len(io.amxIntQueue))
	}
}

// TestAMXNegativeDirectionBit checks the opposite-direction case sets
// bit0, matching the disassembled handler's `AND 1 / JR NZ` branch.
func TestAMXNegativeDirectionBit(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseAMX)

	io.SetMouseState(MouseState{DeltaY: -1})
	if len(io.amxIntQueue) != 1 {
		t.Fatalf("queued %d requests for DeltaY=-1, want 1", len(io.amxIntQueue))
	}
	req := io.amxIntQueue[0]
	if req.vector != amxVectorY {
		t.Errorf("vector = 0x%02X, want Y vector 0x%02X", req.vector, amxVectorY)
	}
	if req.portValue&1 != 1 {
		t.Errorf("portValue bit0 = %d, want 1 (negative direction)", req.portValue&1)
	}
}

// TestAMXQueueCap checks the queue drops the oldest entry rather than
// growing without bound when a guest never services interrupts.
func TestAMXQueueCap(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseAMX)

	for i := 0; i < amxQueueCap+10; i++ {
		io.SetMouseState(MouseState{DeltaX: 1})
	}
	if len(io.amxIntQueue) != amxQueueCap {
		t.Errorf("queue length = %d, want capped at %d", len(io.amxIntQueue), amxQueueCap)
	}
}

// TestGetInterruptVectorDefault checks the zen80 InterruptController
// interface's default (0xFF, matching zen80's own built-in fallback --
// see z80.go) when nothing AMX-related is pending, so implementing this
// interface at all changes nothing for non-AMX software.
func TestGetInterruptVectorDefault(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)

	if got := io.GetInterruptVector(); got != 0xFF {
		t.Errorf("GetInterruptVector() with nothing pending = 0x%02X, want 0xFF", got)
	}
}

// TestPopAMXInterruptSetsVectorAndClearsOnRead checks PopAMXInterrupt
// stages the vector for GetInterruptVector, and that GetInterruptVector
// consumes (clears) it -- a second call without another pop must fall
// back to the default, not repeat the same vector forever.
func TestPopAMXInterruptSetsVectorAndClearsOnRead(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetMouseMode(MouseAMX)
	io.SetMouseState(MouseState{DeltaX: 1})

	v, ok := io.PopAMXInterrupt()
	if !ok || v != amxVectorX {
		t.Fatalf("PopAMXInterrupt() = (0x%02X, %v), want (0x%02X, true)", v, ok, amxVectorX)
	}
	if got := io.GetInterruptVector(); got != amxVectorX {
		t.Errorf("GetInterruptVector() after pop = 0x%02X, want 0x%02X", got, amxVectorX)
	}
	if got := io.GetInterruptVector(); got != 0xFF {
		t.Errorf("GetInterruptVector() second call = 0x%02X, want default 0xFF (consumed, not repeated)", got)
	}
}

// TestAMXEndToEndInterrupt is the real proof: a hand-assembled Z80
// program, installed at the exact AMX vector table addresses the
// disassembly found (I=0xE9, X at table entry 0xE9D2, Y at 0xE9D0),
// running on a real ZenZX (not a bare SpectrumIO), driven the same way
// -mouse amx's real pipeline works (SetMouseState -> queued interrupt ->
// checkAMXInterrupt asserts cpu.INT -> zen80 accepts and calls
// GetInterruptVector -> jumps to the installed handler -> handler reads
// port 0x1F/0x3F itself, exactly as AMX Art's own code does). Confirms
// the whole path end to end, not just each piece in isolation.
func TestAMXEndToEndInterrupt(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	zx.io.SetMouseMode(MouseAMX)

	// Program: DI; LD SP,0xFF00; LD A,0xE9; LD I,A; IM 2;
	// install X handler (0x8030) at table entry 0xE9D2;
	// install Y handler (0x8040) at table entry 0xE9D0;
	// clear two "did the handler run" flags at 0x9000/0x9001; EI; JR $.
	prog := map[uint16][]byte{
		0x8000: {0xF3},             // DI
		0x8001: {0x31, 0x00, 0xFF}, // LD SP,0xFF00
		0x8004: {0x3E, 0xE9},       // LD A,0xE9
		0x8006: {0xED, 0x47},       // LD I,A
		0x8008: {0xED, 0x5E},       // IM 2
		0x800A: {0x21, 0x30, 0x80}, // LD HL,0x8030 (X handler addr)
		0x800D: {0x22, 0xD2, 0xE9}, // LD (0xE9D2),HL
		0x8010: {0x21, 0x40, 0x80}, // LD HL,0x8040 (Y handler addr)
		0x8013: {0x22, 0xD0, 0xE9}, // LD (0xE9D0),HL
		0x8016: {0xAF},             // XOR A
		0x8017: {0x32, 0x00, 0x90}, // LD (0x9000),A  ; X-ran flag = 0
		0x801A: {0x32, 0x01, 0x90}, // LD (0x9001),A  ; Y-ran flag = 0
		0x801D: {0xFB},             // EI
		0x801E: {0x18, 0xFE},       // JR $  (loop forever, waiting)
		// X handler
		0x8030: {0xF5},             // PUSH AF
		0x8031: {0xDB, 0x1F},       // IN A,(0x1F)
		0x8033: {0x32, 0x02, 0x90}, // LD (0x9002),A  ; store what we read
		0x8036: {0x3E, 0x01},       // LD A,1
		0x8038: {0x32, 0x00, 0x90}, // LD (0x9000),A  ; X-ran flag = 1
		0x803B: {0xF1},             // POP AF
		0x803C: {0xFB},             // EI
		0x803D: {0xED, 0x4D},       // RETI
		// Y handler
		0x8040: {0xF5},             // PUSH AF
		0x8041: {0xDB, 0x3F},       // IN A,(0x3F)
		0x8043: {0x32, 0x03, 0x90}, // LD (0x9003),A
		0x8046: {0x3E, 0x01},       // LD A,1
		0x8048: {0x32, 0x01, 0x90}, // LD (0x9001),A  ; Y-ran flag = 1
		0x804B: {0xF1},             // POP AF
		0x804C: {0xFB},             // EI
		0x804D: {0xED, 0x4D},       // RETI
	}
	for addr, bytes := range prog {
		zx.memory.Load(addr, bytes)
	}
	zx.cpu.PC = 0x8000

	// Run enough instructions to execute the setup code and reach the
	// JR $ wait loop, before any mouse movement is queued.
	for i := 0; i < 200; i++ {
		zx.cpu.Step()
	}
	if zx.cpu.PC != 0x801E {
		t.Fatalf("setup did not reach the wait loop: PC=0x%04X, want 0x801E", zx.cpu.PC)
	}
	if zx.memory.Read(0x9000) != 0 || zx.memory.Read(0x9001) != 0 {
		t.Fatal("ran flags not zero before any mouse movement -- test setup is wrong")
	}

	// Drive it exactly the way -mouse amx's real pipeline does: queue a
	// step via SetMouseState, then let RunFrame's checkAMXInterrupt pick
	// it up and deliver it.
	zx.io.SetMouseState(MouseState{DeltaX: 1}) // one X step, positive direction
	zx.RunFrame()

	if zx.memory.Read(0x9000) != 1 {
		t.Errorf("X handler did not run (flag=%d) -- interrupt was not delivered/vectored correctly", zx.memory.Read(0x9000))
	}
	if got := zx.memory.Read(0x9002); got&1 != 0 {
		t.Errorf("X handler read port value 0x%02X, bit0=1, want bit0=0 (positive direction)", got)
	}
	if zx.memory.Read(0x9001) != 0 {
		t.Error("Y handler ran, but only an X step was queued -- wrong vector delivered")
	}

	// Now a Y step, negative direction, on top of the same running
	// program (confirms the mechanism handles a second, different-axis
	// interrupt correctly, not just a one-shot).
	zx.io.SetMouseState(MouseState{DeltaY: -1})
	zx.RunFrame()

	if zx.memory.Read(0x9001) != 1 {
		t.Errorf("Y handler did not run (flag=%d)", zx.memory.Read(0x9001))
	}
	if got := zx.memory.Read(0x9003); got&1 != 1 {
		t.Errorf("Y handler read port value 0x%02X, bit0=%d, want bit0=1 (negative direction)", got, got&1)
	}
}
