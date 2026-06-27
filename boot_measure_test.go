//go:build headless

package main

import (
	"fmt"
	"testing"
)

type bootModel struct {
	name    string
	roms    []string
	rainbow bool
}

var bootModels = []bootModel{
	{"48k", []string{"rom/48.rom"}, false},
	{"128k", []string{"rom/128-0.rom", "rom/128-1.rom"}, true},
	{"plus2", []string{"rom/plus2-0.rom", "rom/plus2-1.rom"}, true},
	{"plus3", []string{"rom/plus3-0.rom", "rom/plus3-1.rom", "rom/plus3-2.rom", "rom/plus3-3.rom"}, true},
	{"spanish48k", []string{"rom/48-spanish.rom"}, false},
	{"spanish128k", []string{"rom/128-spanish-0.rom", "rom/128-spanish-1.rom"}, false},
}

func loadBootModel(m bootModel) *ZenZX {
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	zx.cpu.Reset()
	var err error
	switch len(m.roms) {
	case 1:
		err = zx.LoadROM(m.roms[0])
	case 2:
		err = zx.Load128KROM(m.roms[0], m.roms[1])
	case 4:
		err = zx.LoadPlus3ROM(m.roms[0], m.roms[1], m.roms[2], m.roms[3])
		if err == nil {
			zx.io.EnableFDC()
		}
	}
	if err != nil {
		return nil
	}
	zx.cpu.Reset()
	return zx
}

// TestBootBaselines is a regression guard: it measures each model's live boot
// frame and fails if it has drifted outside the recorded baseline +-5% band.
// Unlike TestMeasureBoot (a print-only harness), this asserts. A model whose
// ROM set is absent is skipped, not failed.
func TestBootBaselines(t *testing.T) {
	const maxFrames = 800
	for _, m := range bootModels {
		base, ok := bootBaselines[m.name]
		if !ok {
			t.Errorf("%s: no baseline recorded", m.name)
			continue
		}
		zx := loadBootModel(m)
		if zx == nil {
			t.Logf("%s: ROM set unavailable, skipping", m.name)
			continue
		}
		d := NewBootDetector(base.rainbow)
		got := -1
		for f := 1; f <= maxFrames; f++ {
			zx.RunFrame()
			d.Update(zx)
			if d.Ready() {
				got = d.ReadyFrame()
				break
			}
		}
		if got < 0 {
			t.Errorf("%s: never reached boot-ready within %d frames", m.name, maxFrames)
			continue
		}
		lo := float64(base.frame) * (1 - BootTolerance)
		hi := float64(base.frame) * (1 + BootTolerance)
		if float64(got) < lo || float64(got) > hi {
			t.Errorf("%s: boot frame %d outside baseline %d +-%.0f%% (%.1f..%.1f)",
				m.name, got, base.frame, BootTolerance*100, lo, hi)
		}
	}
}

// TestMeasureBoot measures the boot-ready frame for each model via
// BootDetector and prints a table with a +-5% tolerance band. Measurement
// harness, not an assertion (see TestBootBaselines for the regression guard).
func TestMeasureBoot(t *testing.T) {
	const fps = 50.0
	const maxFrames = 800
	const tol = 0.05

	fmt.Printf("\n%-14s %6s %6s %9s %16s\n", "model", "ready", "rbow", "seconds", "+-5% band (s)")
	fmt.Println("-------------- ------ ------ --------- ----------------")
	for _, m := range bootModels {
		zx := loadBootModel(m)
		if zx == nil {
			fmt.Printf("%-14s %6s\n", m.name, "ROM N/A")
			continue
		}
		d := NewBootDetector(m.rainbow)
		for f := 1; f <= maxFrames; f++ {
			zx.RunFrame()
			d.Update(zx)
			if d.Ready() {
				break
			}
		}
		if !d.Ready() {
			fmt.Printf("%-14s %6s (not ready within %d frames)\n", m.name, "-", maxFrames)
			continue
		}
		rf := d.ReadyFrame()
		sec := float64(rf) / fps
		lo := sec * (1 - tol)
		hi := sec * (1 + tol)
		fmt.Printf("%-14s %6d %6t %8.3fs  %6.3f-%6.3f\n", m.name, rf, d.seenRainbow, sec, lo, hi)
	}
	fmt.Println()
}
