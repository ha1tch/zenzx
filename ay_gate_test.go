package zenzx

import "testing"

// newAYGateIO builds an IO over a memory configured by enable (nil leaves it
// a 48K), with no audio attached: the AY register cache is what is observed.
func newAYGateIO(enable func(*SpectrumMemory)) *SpectrumIO {
	mem, _ := newTestMemoryAndScreen()
	if enable != nil {
		enable(mem)
	}
	return NewSpectrumIO(mem, nil)
}

func ayRegistersTouched(io *SpectrumIO) bool {
	for _, v := range io.ayRegisters {
		if v != 0 {
			return true
		}
	}
	return io.ayRegister != 0
}

// TestAYPortsAbsentOn48K is T-17: a 48K has no AY, so writes to 0xFFFD and
// 0xBFFD must not select or load an AY register, and a read of 0xFFFD must
// fall through to the floating bus like any other unattached port.
func TestAYPortsAbsentOn48K(t *testing.T) {
	io := newAYGateIO(nil)
	io.floatingBusFn = func() uint8 { return 0xA5 }

	io.WritePort(0xFFFD, 7)
	io.WritePort(0xBFFD, 0x55)
	if ayRegistersTouched(io) {
		t.Fatalf("a 48K must ignore 0xFFFD/0xBFFD writes; ayRegister=%d ayRegisters=%v", io.ayRegister, io.ayRegisters)
	}
	if got := io.ReadPort(0xFFFD); got != 0xA5 {
		t.Errorf("ReadPort(0xFFFD) on a 48K = 0x%02X, want the floating bus 0xA5", got)
	}
}

// TestAYPortsPresentOn128KFamily confirms the gate leaves every machine that
// really has an AY exactly as it was.
func TestAYPortsPresentOn128KFamily(t *testing.T) {
	cases := []struct {
		name   string
		enable func(*SpectrumMemory)
	}{
		{"128K", func(m *SpectrumMemory) { m.Enable128K() }},
		{"+3", func(m *SpectrumMemory) { m.EnablePlus3() }},
		{"Next", func(m *SpectrumMemory) { m.Enable128K(); m.EnableNext() }},
	}
	for _, c := range cases {
		io := newAYGateIO(c.enable)
		io.floatingBusFn = func() uint8 { return 0xA5 }
		io.WritePort(0xFFFD, 7)
		io.WritePort(0xBFFD, 0x55)
		if io.ayRegister != 7 || io.ayRegisters[7] != 0x55 {
			t.Errorf("%s: AY write via 0xFFFD/0xBFFD lost; ayRegister=%d ayRegisters[7]=0x%02X", c.name, io.ayRegister, io.ayRegisters[7])
		}
		if got := io.ReadPort(0xFFFD); got != 0xFF {
			t.Errorf("%s: ReadPort(0xFFFD) = 0x%02X, want the AY read path's 0xFF (no audio attached), not the floating bus", c.name, got)
		}
	}
}
