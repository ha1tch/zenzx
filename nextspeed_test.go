package zenzx

import "testing"

func TestNextSpeedRegister(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.nextRegs.selected = 0x07
	if got := io.nextRegReadSelected(); got != 0x00 {
		t.Errorf("NextREG 0x07 reads 0x%02X at the start, want 0x00", got)
	}
	for speed, want := range map[uint8]uint8{0: 0x00, 1: 0x11, 2: 0x22, 3: 0x33} {
		io.nextRegWriteDirect(0x07, speed)
		if got := io.nextRegReadSelected(); got != want {
			t.Errorf("NextREG 0x07 reads 0x%02X after %d was written, want 0x%02X: the speed in bits 5-4 and 1-0", got, speed, want)
		}
	}

	// A machine that is not a Next has no such register to act on.
	mem48, _ := newTestMemoryAndScreen()
	io48 := NewSpectrumIO(mem48, nil)
	io48.nextRegWriteDirect(0x07, 3)
	if io48.nextSpeed.shift != 0 {
		t.Error("a write to NextREG 0x07 changed the speed of a machine that is not a Next")
	}
}

func TestStepCPUCountsTStatesAtTheSpeedAsked(t *testing.T) {
	// The CPU steps through NOPs, four cycles each to the core.
	for _, c := range []struct {
		shift uint8
		steps int
		want  int // 3.5 MHz T-states for those steps
	}{
		{0, 8, 32},
		{1, 8, 16},
		{2, 8, 8},
		{3, 8, 4},
		{3, 7, 3}, // the half T-state left over is carried, not rounded up
	} {
		zx := NewZenZX(AudioBackendOto)
		zx.io.nextSpeed.shift = c.shift
		before, total := zx.cpu.Cycles, 0
		for i := 0; i < c.steps; i++ {
			total += zx.stepCPU()
		}
		if total != c.want || int(zx.cpu.Cycles-before) != c.want {
			t.Errorf("speed %d, %d NOPs: stepCPU returned %d T-states and the core's count moved by %d, want %d for both",
				c.shift, c.steps, total, zx.cpu.Cycles-before, c.want)
		}
	}
}

func TestNextSpeedChangeDropsTheFraction(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.nextRegWriteDirect(0x07, 3)
	io.nextSpeed.frac = 5
	io.nextRegWriteDirect(0x07, 3) // the same speed again: nothing changes
	if io.nextSpeed.frac != 5 {
		t.Error("writing the same speed again dropped the fraction of a T-state carried")
	}
	io.nextRegWriteDirect(0x07, 1)
	if io.nextSpeed.frac != 0 || io.nextSpeed.shift != 1 {
		t.Errorf("after a change of speed: shift %d, fraction %d; want 1 and 0", io.nextSpeed.shift, io.nextSpeed.frac)
	}
}
