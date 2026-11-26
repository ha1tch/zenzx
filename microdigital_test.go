package zenzx

import "testing"

// withMicrodigital sets the language switch for one test and puts it back.
func withMicrodigital(t *testing.T, portuguese bool) {
	t.Helper()
	was := MicrodigitalPortuguese()
	SetMicrodigitalLanguage(portuguese)
	t.Cleanup(func() { SetMicrodigitalLanguage(was) })
}

func TestMicrodigitalSwitchIsSpanishUntilTurned(t *testing.T) {
	if MicrodigitalPortuguese() {
		t.Fatal("the switch starts at Portuguese: every machine's bit 7 would change under a program that never asked")
	}
}

// Spanish leaves bit 7 of the ULA port high, as it reads on every other Spectrum; Portuguese
// reads it low, whatever half rows are selected and whatever keys are down. Nothing else of
// the byte moves: bit 5 stays high, and the five key bits still follow the keyboard.
func TestMicrodigitalSwitchSetsBit7OfTheULAPort(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	io.ResetKeyboard()
	io.PressKey(1, 2) // a key on half row 1

	for _, port := range []uint16{0xFEFE, 0xFDFE, 0xFFFE, 0x00FE, 0xBFFE, 0x7FFE} {
		withMicrodigital(t, false)
		es := io.ReadPort(port)
		SetMicrodigitalLanguage(true)
		pt := io.ReadPort(port)
		if es&0x80 == 0 {
			t.Errorf("port %04X: Spanish reads bit 7 low (%02X)", port, es)
		}
		if pt&0x80 != 0 {
			t.Errorf("port %04X: Portuguese reads bit 7 high (%02X)", port, pt)
		}
		if es&^0x80 != pt&^0x80 {
			t.Errorf("port %04X: the switch moved more than bit 7: %02X against %02X", port, es, pt)
		}
		if pt&0x20 == 0 {
			t.Errorf("port %04X: Portuguese lost bit 5 (%02X)", port, pt)
		}
	}
	withMicrodigital(t, true)
	if got := io.ReadPort(0xFDFE) & 0x1F; got != 0x1F&^(1<<2) {
		t.Errorf("the key under the switch reads %02X, want %02X", got, 0x1F&^(1<<2))
	}
}

// Only a 48K machine reads the switch: the Amstrad models (128K, +2, +2A, +3), the Timex and
// the Next read the ULA port as they always did, whatever the position.
func TestMicrodigitalSwitchIsOnlyThe48Ks(t *testing.T) {
	withMicrodigital(t, true)
	for _, c := range []struct {
		name    string
		set     func(m *SpectrumMemory)
		low     bool // bit 7 reads low at Portuguese
		isPlain bool
	}{
		{"48K", func(m *SpectrumMemory) {}, true, true},
		{"128K", func(m *SpectrumMemory) { m.Enable128K() }, false, false},
		{"+3", func(m *SpectrumMemory) { m.Enable128K(); m.EnablePlus3() }, false, false},
		{"Timex", func(m *SpectrumMemory) { m.isTS2068 = true }, false, false},
		{"Next", func(m *SpectrumMemory) { m.EnableNext() }, false, false},
	} {
		mem, _ := newTestMemoryAndScreen()
		c.set(mem)
		io := NewSpectrumIO(mem, nil)
		io.ResetKeyboard()
		if got := mem.isPlain48K(); got != c.isPlain {
			t.Errorf("%s: isPlain48K = %v", c.name, got)
		}
		if low := io.ReadPort(0xFEFE)&0x80 == 0; low != c.low {
			t.Errorf("%s at Portuguese: bit 7 low = %v, want %v", c.name, low, c.low)
		}
	}
	var none *SpectrumMemory
	if !none.isPlain48K() {
		t.Error("no memory is not taken for a 48K")
	}
}

// The switch is the ULA port's: the Kempston port and an unattached one answer as they did.
func TestMicrodigitalSwitchLeavesOtherPortsAlone(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	for _, port := range []uint16{0x001F, 0x00BF, 0xBFBF} {
		withMicrodigital(t, false)
		es := io.ReadPort(port)
		SetMicrodigitalLanguage(true)
		if pt := io.ReadPort(port); pt != es {
			t.Errorf("port %04X reads %02X at Portuguese, %02X at Spanish", port, pt, es)
		}
	}
}

func TestParseMicrodigitalLanguage(t *testing.T) {
	for _, c := range []struct {
		in         string
		pt, wantOK bool
	}{
		{"portuguese", true, true}, {"PT", true, true}, {" pt ", true, true},
		{"spanish", false, true}, {"Spanish", false, true}, {"es", false, true},
		{"", false, false}, {"english", false, false}, {"portugues", false, false},
	} {
		pt, ok := parseMicrodigitalLanguage(c.in)
		if pt != c.pt || ok != c.wantOK {
			t.Errorf("parseMicrodigitalLanguage(%q) = %v, %v; want %v, %v", c.in, pt, ok, c.pt, c.wantOK)
		}
	}
	if microdigitalWord(true) != "portuguese" || microdigitalWord(false) != "spanish" {
		t.Error("microdigitalWord is not the settings.json spelling")
	}
}
