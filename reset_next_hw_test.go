//go:build headless

package zenzx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextResetRestartsTheFirmware resets a Next that is in the middle of one of the examples,
// the way the Machine menu, F1 and the debugger's reset do (Reset), against the real firmware.
// The Next starts again from its boot ROM in config mode, with every register and what it
// drives as at power on (the program's Layer 2, tilemap, sprites, copper, interrupts and
// palettes are gone), and NextZXOS comes up again to its menu.
//
// Dormant guard G-15 (docs/KNOWN_ISSUES.md): gated on ZENZX_NEXT_FILES, as G-04 and G-12 are.
// The boot card is opened read-only. It is the hardware half of T-74's test; the part that
// needs no firmware is TestResetNextIsAHardReset (reset_test.go).
func TestNextResetRestartsTheFirmware(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-15)")
	}
	examples, err := nextExamples()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	nex := map[string]string{}
	for _, ex := range examples {
		p := filepath.Join(tmp, ex.Name)
		if err := os.WriteFile(p, ex.Data, 0o644); err != nil {
			t.Fatal(err)
		}
		nex[ex.Name] = p
	}
	// What each example leaves on the Next, apart from its picture: the registers it set, so
	// that a reset that did not put them back is seen.
	examplesSet := []struct {
		name    string
		running func(zx *ZenZX) bool
	}{
		{"gallop.nex", func(zx *ZenZX) bool {
			return zx.io.copper.mode == 3 && zx.io.nextRegs.regs[0x6B] == 0xA9 && zx.io.nextRegs.regs[0x15] == 0x01
		}},
		{"fuji.nex", func(zx *ZenZX) bool {
			return zx.io.copper.mode == 3 && zx.io.nextRegs.regs[0x70] == 0x10 && zx.io.nextRegs.regs[0x6B] == 0x83
		}},
		{"orbit.nex", func(zx *ZenZX) bool { return zx.io.nextInt.hw && zx.cpu.IM == 2 }},
	}
	for _, ex := range examplesSet {
		ex := ex
		t.Run(ex.name, func(t *testing.T) {
			if nex[ex.name] == "" {
				t.Skipf("%s is not among the examples", ex.name)
			}
			f := nextcard.In(dir)
			zx, err := newMachine("next", "./rom")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := bootNextFrom(zx, f, f.Card, true, false); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { zx.dropNEXScratch(); zx.detachNextBootChain() })
			zx.running = true
			resetRun(zx, 600)
			if msg, reset := zx.RunNEX(nex[ex.name], false); reset {
				t.Fatalf("a Next at its welcome screen asked for a reset: %s", msg)
			}
			for n := 0; n < nexRunGiveUp+500 && zx.NEXRunning(); n++ {
				zx.RunFrame()
			}
			resetRun(zx, 120)
			if !ex.running(zx) {
				t.Fatalf("setting up: %s is not running", ex.name)
			}

			zx.Reset()
			resetRun(zx, 1)

			m, io := zx.memory, zx.io
			if !m.nextCfg.active || !m.bootROMEnabled || m.nextCfg.machineType != 0 {
				t.Errorf("the firmware did not start again in config mode: config %v, boot ROM %v, machine type %d",
					m.nextCfg.active, m.bootROMEnabled, m.nextCfg.machineType)
			}
			for _, r := range []struct{ reg, want uint8 }{
				{0x15, 0x00}, {0x69, 0x00}, {0x6B, 0x00}, {0x70, 0x00}, {0x12, 0x08}, {0x13, 0x0B}, {0x14, 0xE3},
				{0x4A, 0xE3}, {0x07, 0x00}, {0x62, 0x00}, {0x0A, 0x01}, {0x6E, 0x6C}, {0x6F, 0x4C},
			} {
				if got := io.nextRegs.regs[r.reg]; got != r.want {
					t.Errorf("NextREG 0x%02X is 0x%02X after Reset, want 0x%02X", r.reg, got, r.want)
				}
			}
			if io.copper.mode != 0 || io.nextInt.hw || io.nextInt.ctcOn || io.dma.enabled || zx.cpu.IM == 2 {
				t.Errorf("the program's copper (mode %d), interrupts (hardware %v, CTC %v), DMA (%v) or interrupt mode (%d) is still there",
					io.copper.mode, io.nextInt.hw, io.nextInt.ctcOn, io.dma.enabled, zx.cpu.IM)
			}
			if io.divMMC == nil || io.divMMC.automapEnabled {
				t.Error("the DivMMC's automap is still on from before the reset")
			}
			if io.ayRegisters != [16]uint8{} {
				t.Errorf("the AY still holds %v", io.ayRegisters)
			}
			// And NextZXOS comes up again: the firmware chooses the machine and leaves config
			// mode, and the menu is drawn.
			cyan := func() int {
				n := 0
				for cell := 4 * 32; cell < 16*32; cell++ {
					if zx.memory.screen.attributes[cell]&0x38 == 0x28 {
						n++
					}
				}
				return n
			}
			handedOver := 0
			for frames := 1; frames <= 6000; frames += 50 {
				resetRun(zx, 50)
				if handedOver == 0 {
					if m.nextCfg.machineType == 3 && !m.nextCfg.active && !m.bootROMEnabled {
						handedOver = frames
					}
					continue
				}
				if cyan() >= 8 {
					t.Logf("%s: the firmware handed over after %d frames and the NextZXOS menu was up after %d", ex.name, handedOver, frames)
					return
				}
				if (frames-handedOver)%300 == 250 { // the welcome screen waits for a key, as it does after a switch-on
					io.PressKey(7, 0)
					resetRun(zx, 10)
					io.ReleaseKey(7, 0)
				}
			}
			t.Fatalf("NextZXOS did not come up again within 6000 frames of the reset (the firmware handed over at frame %d; 0 is never)", handedOver)
		})
	}
}
