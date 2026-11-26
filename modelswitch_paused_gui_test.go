//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"testing"
)

// A machine that is switched to another model is a new machine: nothing of the old one is left to be
// seen, whether it was running or paused. A paused machine does not run its frames, so what the
// switch has to do is done there and then, and not at the first frame after Play (a Next's pending
// hard reset was, and until then the picture was the old machine's screen drawn as the new model's).

// dirty puts something of its own on the machine's screen, in both of the 128K's screens when it has two,
// and on a Next in Layer 2 as well.
func dirty(t *testing.T, zx *ZenZX) {
	t.Helper()
	m := zx.memory
	for i := uint16(0); i < 6912; i++ {
		m.Write(0x4000+i, 0xA5^byte(i))
	}
	if zx.memory.is128K && !m.isNext {
		m.SetPaging(0x07) // RAM bank 7 at the top, the normal screen
		for i := uint16(0); i < 6912; i++ {
			m.Write(0xC000+i, 0x5A^byte(i))
		}
		m.SetPaging(0x08 | 0x07) // and the shadow screen shown: "screen file 1"
	}
	if m.isNext {
		zx.io.SetNextRegister(0x69, 0x80) // Layer 2 on
		for p := uint8(18); p < 21; p++ {
			zx.io.SetNextRegister(0x12, p) // its bank
		}
	}
	zx.DecodeDisplay() // the first picture, which fills what the display keeps of it
}

// sameAsNew says the machine draws what a new machine of its model draws, paused as it is.
func sameAsNew(t *testing.T, label, model string, zx *ZenZX) {
	t.Helper()
	fresh := machineOf(t, model)
	fresh.paused = true
	fresh.Reset()
	want, got := fresh.DecodeDisplay(), zx.DecodeDisplay()
	if want.Bounds() != got.Bounds() {
		t.Fatalf("%s: the picture is %v, a new machine's is %v", label, got.Bounds(), want.Bounds())
	}
	for i := 0; i+4 <= len(want.Pix); i += 4 {
		if want.Pix[i] != got.Pix[i] || want.Pix[i+1] != got.Pix[i+1] || want.Pix[i+2] != got.Pix[i+2] {
			t.Fatalf("%s: pixel %d is %v, a new machine's is %v: the old machine's screen is still shown", label, i/4, got.Pix[i:i+4], want.Pix[i:i+4])
		}
	}
}

// ramEmpty says no RAM byte of the machine is anything but zero.
func ramEmpty(t *testing.T, label string, zx *ZenZX) {
	t.Helper()
	for b := range zx.memory.ram {
		for i, v := range zx.memory.ram[b] {
			if v != 0 {
				t.Fatalf("%s: RAM bank %d holds %#02x at %#04x: the old machine's memory", label, b, v, i)
			}
		}
	}
	if x := zx.memory.nextExtraRAM; x != nil {
		for p := range x {
			for i, v := range x[p] {
				if v != 0 {
					t.Fatalf("%s: Next page %d holds %#02x at %#04x", label, p+nextExtraPageBase, v, i)
				}
			}
		}
	}
}

func TestSwitchingModelWhilePausedLeavesNothingOfTheOldMachine(t *testing.T) {
	models := []string{"48k", "128k", "plus3", "ts2068", "next"}
	for _, from := range models {
		for _, to := range models {
			if from == to {
				continue
			}
			zx := machineOf(t, from)
			for i := 0; i < 3; i++ {
				zx.RunFrame()
			}
			dirty(t, zx)
			zx.paused = true
			switchModelLive(zx, to, "./rom")
			label := from + " to " + to
			if !zx.paused {
				t.Errorf("%s: the switch took the machine out of its pause", label)
			}
			if zx.io.nextResetRequest != 0 {
				t.Errorf("%s: a reset is still waiting for the first frame after Play", label)
			}
			if zx.memory.screenBank != 5 {
				t.Errorf("%s: the screen shown is bank %d", label, zx.memory.screenBank)
			}
			ramEmpty(t, label, zx)
			sameAsNew(t, label, to, zx)
			if zx.io.layer2Enabled() {
				t.Errorf("%s: Layer 2 is on", label)
			}
		}
	}
}
