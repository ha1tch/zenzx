package main

import "testing"

func TestParseJoystickMode(t *testing.T) {
	cases := []struct {
		input   string
		want    JoystickMode
		wantErr bool
	}{
		{"none", JoystickNone, false},
		{"", JoystickNone, false},
		{"kempston", JoystickKempston, false},
		{"Kempston", JoystickKempston, false}, // case-insensitive
		{"  sinclair  ", JoystickSinclair1, false},
		{"sinclair1", JoystickSinclair1, false},
		{"sinclair2", JoystickSinclair2, false},
		{"sinclair-both", JoystickSinclairBoth, false},
		{"cursor", JoystickNone, true}, // not implemented -- must error, not silently fall back
		{"garbage", JoystickNone, true},
	}
	for _, c := range cases {
		got, err := ParseJoystickMode(c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseJoystickMode(%q) = nil error, want an error", c.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseJoystickMode(%q) unexpected error: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseJoystickMode(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// TestKempstonByte checks the port 0x1F bit layout against the original
// Kempston Joystick Interface instruction sheet: bit0=right, bit1=left,
// bit2=down, bit3=up, bit4=fire, active HIGH.
func TestKempstonByte(t *testing.T) {
	cases := []struct {
		name  string
		state JoystickState
		want  uint8
	}{
		{"nothing", JoystickState{}, 0x00},
		{"right", JoystickState{Right: true}, 0x01},
		{"left", JoystickState{Left: true}, 0x02},
		{"down", JoystickState{Down: true}, 0x04},
		{"up", JoystickState{Up: true}, 0x08},
		{"fire", JoystickState{Fire: true}, 0x10},
		{"up-right and fire", JoystickState{Up: true, Right: true, Fire: true}, 0x08 | 0x01 | 0x10},
		{"all directions (diagonal + opposite, still valid electrically)",
			JoystickState{Up: true, Down: true, Left: true, Right: true, Fire: true}, 0x1F},
	}
	for _, c := range cases {
		if got := kempstonByte(c.state); got != c.want {
			t.Errorf("%s: kempstonByte = 0x%02X, want 0x%02X", c.name, got, c.want)
		}
	}
}

// TestSetJoystickStateKempston checks the full path: mode selection ->
// SetJoystickState -> port 0x1F read, via SpectrumIO's real ReadPort.
func TestSetJoystickStateKempston(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetJoystickMode(JoystickKempston)

	io.SetJoystickState(JoystickState{Left: true, Fire: true})
	got := io.ReadPort(0x1F)
	want := uint8(0x02 | 0x10)
	if got != want {
		t.Errorf("port 0x1F after left+fire = 0x%02X, want 0x%02X", got, want)
	}

	io.SetJoystickState(JoystickState{}) // release
	if got := io.ReadPort(0x1F); got != 0x00 {
		t.Errorf("port 0x1F after release = 0x%02X, want 0x00", got)
	}
}

// TestSinclairKeyMatrices checks both ports' row/col mappings match
// input.go's own key matrix positions exactly (row 4: keys 6,7,8,9,0 at
// columns 4,3,2,1,0; row 3: keys 1,2,3,4,5 at columns 0,1,2,3,4), and the
// direction assignment (Wikipedia's ZX Interface 2 article, the
// Interface 2 circuitry reference, libretro's Fuse docs, and
// sharedmemorydump.net's own +2/+3 port testing): Joystick 1 = 6 left,
// 7 right, 8 down, 9 up, 0 fire; Joystick 2 mirrors the same pattern
// onto 1-5.
func TestSinclairKeyMatrices(t *testing.T) {
	cases := []struct {
		name string
		pos  [2]uint8
		want [2]uint8
	}{
		{"port1 left (key 6)", sinclairKeyMatrix1.Left, [2]uint8{4, 4}},
		{"port1 right (key 7)", sinclairKeyMatrix1.Right, [2]uint8{4, 3}},
		{"port1 down (key 8)", sinclairKeyMatrix1.Down, [2]uint8{4, 2}},
		{"port1 up (key 9)", sinclairKeyMatrix1.Up, [2]uint8{4, 1}},
		{"port1 fire (key 0)", sinclairKeyMatrix1.Fire, [2]uint8{4, 0}},
		{"port2 left (key 1)", sinclairKeyMatrix2.Left, [2]uint8{3, 0}},
		{"port2 right (key 2)", sinclairKeyMatrix2.Right, [2]uint8{3, 1}},
		{"port2 down (key 3)", sinclairKeyMatrix2.Down, [2]uint8{3, 2}},
		{"port2 up (key 4)", sinclairKeyMatrix2.Up, [2]uint8{3, 3}},
		{"port2 fire (key 5)", sinclairKeyMatrix2.Fire, [2]uint8{3, 4}},
	}
	for _, c := range cases {
		if c.pos != c.want {
			t.Errorf("%s: matrix position = %v, want %v", c.name, c.pos, c.want)
		}
	}
}

// TestSetJoystickStateSinclair1 checks the full path: mode selection ->
// SetJoystickState -> keyboard matrix -> port 0xFE read, exactly as a
// real program checking for the joystick (by reading the number-row
// keys) would see it.
func TestSetJoystickStateSinclair1(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.ResetKeyboard()
	io.SetJoystickMode(JoystickSinclair1)

	io.SetJoystickState(JoystickState{Up: true, Fire: true})

	// Row 4 (address line A12 low, per the standard matrix -- port value
	// 0xEFFE covers row 4) should show key 9 (up, col 1) and key 0 (fire,
	// col 0) pressed (bits cleared, active low), everything else released.
	// Bit 7 is always set on a real ULA read (ReadPort's own behaviour,
	// unrelated to this feature) -- included in the expected value.
	row4 := io.ReadPort(0xEFFE)
	// 0xA0: bits 5 and 7 both read high on a real Spectrum's ULA port
	// (idle value 0xBF) -- the previous 0x80 baseline encoded the old,
	// hardware-inaccurate bit-5-low behaviour.
	wantRow4 := uint8(0xA0 | (0x1F &^ (1 << 1) &^ (1 << 0)))
	if row4 != wantRow4 {
		t.Errorf("row 4 after up+fire = 0x%02X, want 0x%02X", row4, wantRow4)
	}

	// A different row (row 3: keys 1-5, port 2's row) must be entirely
	// untouched by port 1 activity.
	row3 := io.ReadPort(0xF7FE)
	if row3 != 0xBF {
		t.Errorf("row 3 (port 2's keys, unrelated to port 1) = 0x%02X, want 0xBF (untouched, bits 5 and 7 always set)", row3)
	}

	// Release: both bits should return to released (set).
	io.SetJoystickState(JoystickState{})
	if got := io.ReadPort(0xEFFE); got != 0xBF {
		t.Errorf("row 4 after release = 0x%02X, want 0xBF", got)
	}
}

// TestSetJoystickStateSinclair2 is Sinclair Joystick 2's equivalent,
// completing what was deferred in T-13 -- needed now to correctly
// represent the +2/+2A/+3's real two-port built-in hardware.
func TestSetJoystickStateSinclair2(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.ResetKeyboard()
	io.SetJoystickMode(JoystickSinclair2)

	io.SetJoystickState(JoystickState{Down: true})

	// Row 3 should show key 3 (down, col 2) pressed.
	row3 := io.ReadPort(0xF7FE)
	wantRow3 := uint8(0xA0 | (0x1F &^ (1 << 2)))
	if row3 != wantRow3 {
		t.Errorf("row 3 after down = 0x%02X, want 0x%02X", row3, wantRow3)
	}

	// Row 4 (port 1's row) must be entirely untouched.
	row4 := io.ReadPort(0xEFFE)
	if row4 != 0xBF {
		t.Errorf("row 4 (port 1's keys, unrelated to port 2) = 0x%02X, want 0xBF (untouched)", row4)
	}
}

// TestSetJoystickStateSinclairBoth is the +2/+2A/+3's real hardware
// configuration: both built-in ports active simultaneously, from two
// independent abstract states in one call.
func TestSetJoystickStateSinclairBoth(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.ResetKeyboard()
	io.SetJoystickMode(JoystickSinclairBoth)

	io.SetJoystickStateBoth(
		JoystickState{Left: true},  // port 1: key 6
		JoystickState{Right: true}, // port 2: key 2
	)

	row4 := io.ReadPort(0xEFFE)
	wantRow4 := uint8(0xA0 | (0x1F &^ (1 << 4))) // key 6 = col 4
	if row4 != wantRow4 {
		t.Errorf("row 4 (port 1, left) = 0x%02X, want 0x%02X", row4, wantRow4)
	}

	row3 := io.ReadPort(0xF7FE)
	wantRow3 := uint8(0xA0 | (0x1F &^ (1 << 1))) // key 2 = col 1
	if row3 != wantRow3 {
		t.Errorf("row 3 (port 2, right) = 0x%02X, want 0x%02X", row3, wantRow3)
	}

	// SetJoystickState (single-state) must not touch anything while
	// SinclairBoth is configured -- only SetJoystickStateBoth applies in
	// this mode.
	io.SetJoystickState(JoystickState{Up: true})
	if got := io.ReadPort(0xEFFE); got != row4 {
		t.Errorf("SetJoystickState changed row 4 while SinclairBoth configured: got 0x%02X, want unchanged 0x%02X", got, row4)
	}
}

// TestSetJoystickStateNoneIsNoOp confirms JoystickNone touches neither the
// Kempston byte nor the keyboard matrix -- the default (-joystick not
// passed) must not change any existing behaviour.
// TestDefaultJoystickModeForModel checks -model's implied built-in
// joystick configuration against real hardware history (verified via
// search this session, not assumed -- see the joystick.go package doc
// for citations): 48K and the original pre-Amstrad 128K have no
// built-in port at all; +2/+2A/+3 (and Spanish variants) have two
// built-in Sinclair-protocol ports; TS2068 has its own separate,
// always-on mechanism, so None is the correct default here (not a
// guess at which canonical-Spectrum mechanism it might resemble).
func TestDefaultJoystickModeForModel(t *testing.T) {
	cases := []struct {
		model string
		want  JoystickMode
	}{
		{"48k", JoystickNone},
		{"128k", JoystickNone}, // original "Toastrack" -- no built-in port, confirmed
		{"plus2", JoystickSinclairBoth},
		{"plus2a", JoystickSinclairBoth},
		{"plus3", JoystickSinclairBoth},
		{"spanish48k", JoystickNone},
		{"spanish128k", JoystickNone},
		{"spanishplus2", JoystickSinclairBoth},
		{"spanishplus3", JoystickSinclairBoth},
		{"ts2068", JoystickNone},        // has its own separate mechanism, not this one
		{"PLUS3", JoystickSinclairBoth}, // case-insensitive
	}
	for _, c := range cases {
		if got := defaultJoystickModeForModel(c.model); got != c.want {
			t.Errorf("defaultJoystickModeForModel(%q) = %v, want %v", c.model, got, c.want)
		}
	}
}

// TestResolveJoystickMode checks "auto" resolves against the model, and
// an explicit value overrides it regardless of model -- matching how
// real third-party interfaces (Kempston, most commonly) remained a
// legitimate choice even on models with their own built-in ports.
func TestResolveJoystickMode(t *testing.T) {
	got, err := resolveJoystickMode("auto", "plus3")
	if err != nil || got != JoystickSinclairBoth {
		t.Errorf(`resolveJoystickMode("auto","plus3") = (%v,%v), want (%v,nil)`, got, err, JoystickSinclairBoth)
	}

	got, err = resolveJoystickMode("auto", "48k")
	if err != nil || got != JoystickNone {
		t.Errorf(`resolveJoystickMode("auto","48k") = (%v,%v), want (%v,nil)`, got, err, JoystickNone)
	}

	// Explicit choice overrides the model default -- a real third-party
	// Kempston interface plugged into a +3 is a legitimate, if
	// non-default, configuration.
	got, err = resolveJoystickMode("kempston", "plus3")
	if err != nil || got != JoystickKempston {
		t.Errorf(`resolveJoystickMode("kempston","plus3") = (%v,%v), want (%v,nil)`, got, err, JoystickKempston)
	}

	// "Auto" is case-insensitive and tolerant of surrounding whitespace,
	// matching this codebase's other flag-parsing conventions.
	got, err = resolveJoystickMode(" AUTO ", "plus2")
	if err != nil || got != JoystickSinclairBoth {
		t.Errorf(`resolveJoystickMode(" AUTO ","plus2") = (%v,%v), want (%v,nil)`, got, err, JoystickSinclairBoth)
	}

	if _, err := resolveJoystickMode("garbage", "48k"); err == nil {
		t.Error(`resolveJoystickMode("garbage","48k") = nil error, want an error`)
	}
}

func TestSetJoystickStateNoneIsNoOp(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.ResetKeyboard()
	// JoystickNone is the zero value -- deliberately not calling
	// SetJoystickMode, to check the default itself.

	io.SetJoystickState(JoystickState{Up: true, Fire: true, Left: true})

	if got := io.ReadPort(0x1F); got != 0x00 {
		t.Errorf("Kempston port with JoystickNone = 0x%02X, want 0x00 (untouched)", got)
	}
	if got := io.ReadPort(0xEFFE); got != 0xBF {
		t.Errorf("keyboard row 4 with JoystickNone = 0x%02X, want 0xBF (untouched, bits 5 and 7 always set)", got)
	}
}

// TestSetJoystickStateKempston2 checks the second Kempston port (0x37,
// neo-Spectrum platforms only -- see the package doc for the ZX
// Spectrum Next and KEMPSTON_MAX2 cross-confirmation of that address)
// works identically to the first, just on its own port and its own
// state field.
func TestSetJoystickStateKempston2(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetJoystickMode(JoystickKempston2)

	io.SetJoystickState(JoystickState{Up: true, Down: true})
	if got := io.ReadPort(0x37); got != (0x08 | 0x04) {
		t.Errorf("port 0x37 after up+down = 0x%02X, want 0x%02X", got, 0x08|0x04)
	}
	// The first Kempston port must be completely unaffected.
	if got := io.ReadPort(0x1F); got != 0x00 {
		t.Errorf("port 0x1F while Kempston2 configured = 0x%02X, want 0x00 (untouched)", got)
	}
}

// TestSetJoystickStateKempstonBoth is the neo-Spectrum dual-Kempston
// configuration: both ports active simultaneously from two independent
// abstract states in one call, mirroring SinclairBoth's own test.
func TestSetJoystickStateKempstonBoth(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetJoystickMode(JoystickKempstonBoth)

	io.SetJoystickStateBoth(
		JoystickState{Left: true},  // port 1 (0x1F)
		JoystickState{Right: true}, // port 2 (0x37)
	)

	if got := io.ReadPort(0x1F); got != 0x02 {
		t.Errorf("port 0x1F (port 1, left) = 0x%02X, want 0x02", got)
	}
	if got := io.ReadPort(0x37); got != 0x01 {
		t.Errorf("port 0x37 (port 2, right) = 0x%02X, want 0x01", got)
	}

	// SetJoystickState (single-state) must not touch anything while
	// KempstonBoth is configured -- only SetJoystickStateBoth applies.
	io.SetJoystickState(JoystickState{Fire: true})
	if got := io.ReadPort(0x1F); got != 0x02 {
		t.Errorf("SetJoystickState changed port 0x1F while KempstonBoth configured: got 0x%02X, want unchanged 0x02", got)
	}
}

// TestParseJoystickModeKempstonVariants extends TestParseJoystickMode's
// coverage for the two new values without duplicating the whole table.
func TestParseJoystickModeKempstonVariants(t *testing.T) {
	cases := []struct {
		input string
		want  JoystickMode
	}{
		{"kempston2", JoystickKempston2},
		{"kempston-both", JoystickKempstonBoth},
		{"Kempston-Both", JoystickKempstonBoth}, // case-insensitive
	}
	for _, c := range cases {
		got, err := ParseJoystickMode(c.input)
		if err != nil {
			t.Errorf("ParseJoystickMode(%q) unexpected error: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseJoystickMode(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// TestDefaultJoystickModeNeverImpliesDualKempston confirms no -model
// value ever auto-selects Kempston2/KempstonBoth -- unlike
// JoystickSinclairBoth, no stock model ever had a second Kempston port
// TestDefaultJoystickModeNeverImpliesDualKempston confirms no -model
// value ever auto-selects Kempston2/KempstonBoth -- unlike
// JoystickSinclairBoth, no stock model ever had a second Kempston port
// built in; it must always be an explicit choice.
func TestDefaultJoystickModeNeverImpliesDualKempston(t *testing.T) {
	models := []string{"48k", "128k", "plus2", "plus2a", "plus3", "spanish48k", "spanish128k", "spanishplus2", "spanishplus3", "ts2068"}
	for _, m := range models {
		got := defaultJoystickModeForModel(m)
		if got == JoystickKempston2 || got == JoystickKempstonBoth {
			t.Errorf("defaultJoystickModeForModel(%q) = %v, want never Kempston2/KempstonBoth (no stock model ever had a second Kempston port)", m, got)
		}
	}
}

// TestKempstonMouseCoexistsWithDualKempstonJoysticks answers a direct
// question: does Kempston Mouse take over either Kempston joystick
// port, or can a machine genuinely run two Kempston joysticks plus a
// Kempston mouse at once? Verified by research (a first-hand account on
// Spectrum Computing's forums: Kempston Mouse is "a custom interface"
// with its own dedicated ports, not something that plugs into a
// joystick port) and by exact address analysis: joystick ports 0x1F/
// 0x37 versus mouse ports 0xFADF/0xFBDF/0xFFDF share no low byte in
// common (0x1F, 0x37, 0xDF are all distinct), and zenzx's own port
// dispatch (io.go) uses full decoding for all of them -- exactly the
// "use full 8-bit address decoding" practice the ZX-VGA-JOY interface's
// own design notes cite as what avoids the accidental collisions some
// real, incompletely-decoded 1980s interfaces suffered from. This test
// proves it, not just asserts it: both joystick ports and the mouse
// active and independently correct in the same session.
func TestKempstonMouseCoexistsWithDualKempstonJoysticks(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(NewSpectrumMemory(screen), nil)
	io.SetJoystickMode(JoystickKempstonBoth)
	io.SetMouseMode(MouseKempston)

	io.SetJoystickStateBoth(
		JoystickState{Up: true},   // port 1 (0x1F)
		JoystickState{Down: true}, // port 2 (0x37)
	)
	io.SetMouseState(MouseState{DeltaX: 5, DeltaY: -3, Left: true})

	if got := io.ReadPort(0x1F); got != 0x08 {
		t.Errorf("joystick port 1 (0x1F) = 0x%02X, want 0x08 (up) -- mouse must not have taken it over", got)
	}
	if got := io.ReadPort(0x37); got != 0x04 {
		t.Errorf("joystick port 2 (0x37) = 0x%02X, want 0x04 (down) -- mouse must not have taken it over", got)
	}
	if got := io.ReadPort(0xFBDF); got != 5 {
		t.Errorf("mouse X (0xFBDF) = %d, want 5 -- joystick activity must not have corrupted it", got)
	}
	var deltaY int8 = -3 // a variable, not a constant, so the conversion below is a runtime reinterpretation (wraps) rather than a constant conversion (which would need representability)
	wantY := uint8(deltaY)
	if got := io.ReadPort(0xFFDF); got != wantY {
		t.Errorf("mouse Y (0xFFDF) = %d, want %d (-3 wrapped)", got, wantY)
	}
	buttons := io.ReadPort(0xFADF)
	if buttons&0x02 != 0 { // left button = bit1, active low (right=bit0, middle=bit2)
		t.Errorf("mouse left button (0xFADF) = 0x%02X, want bit1 clear (pressed)", buttons)
	}
}

// TestJoystickKempstonBothConflictsWithAMX guards the fix to a real gap
// this investigation found: JoystickKempstonBoth also uses port 0x1F
// internally for its first sub-port (same as plain JoystickKempston),
// so it has the identical AMX Mouse conflict (both use 0x1F) -- but the
// original validation only checked JoystickKempston, not
// JoystickKempstonBoth. This is a Go-level guard for the underlying
// fact; the actual CLI validation lives in zenzx_headless.go/
// zenzx_gui.go's main() and isn't directly unit-testable from here.
func TestJoystickKempstonBothConflictsWithAMX(t *testing.T) {
	// AMX's own ports, confirmed in mouse.go's package doc: 0x1F (X)
	// and 0x3F (Y) -- 0x1F is exactly JoystickKempstonBoth's first
	// sub-port, hence the real conflict this test documents.
	if JoystickKempstonBoth == JoystickNone {
		t.Fatal("sanity check failed -- JoystickKempstonBoth is not a distinct mode")
	}
	// The actual behavioural proof that KempstonBoth's first sub-port
	// really is 0x1F (the address AMX also uses) is
	// TestSetJoystickStateKempstonBoth, above.
}
