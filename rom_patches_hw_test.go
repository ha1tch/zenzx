//go:build headless

package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextROMPatchesOverTheFirmware lays the ROM switches over the ROMs the real firmware
// loads (T-76): -rom-bin on a Next that boots from its card is applied when the firmware leaves
// config mode and not before, and it goes over what the firmware put in bank 0.
//
// Dormant guard G-16 (docs/KNOWN_ISSUES.md): gated on ZENZX_NEXT_FILES, as G-04 and G-15 are.
// The boot card is opened read-only.
func TestNextROMPatchesOverTheFirmware(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-16)")
	}
	boot := func(t *testing.T) *ZenZX {
		t.Helper()
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
		return zx
	}
	handedOver := func(zx *ZenZX) bool {
		m := zx.memory
		// The firmware chooses the machine and asks for the reset; the reset, about ten frames
		// later, is what starts it, and where the patches go over the ROMs.
		return m.nextCfg.machineType == 3 && !m.nextCfg.active && !m.bootROMEnabled && !zx.io.nextConfigLeft
	}
	patchWith := func(t *testing.T, zx *ZenZX, payload []byte) *romLog {
		t.Helper()
		p := filepath.Join(t.TempDir(), "patch.bin")
		if err := os.WriteFile(p, payload, 0o644); err != nil {
			t.Fatal(err)
		}
		var log romLog
		if err := zx.applyROMSwitches(romSwitches{bin: binSwitch(t, "-rom-bin", p, "-rom-fill", "rom")}, true, log.sink()); err != nil {
			t.Fatal(err)
		}
		return &log
	}

	t.Run("a patch makes a boot cache of its own", func(t *testing.T) {
		zx := boot(t)
		files := nextcard.In(dir)
		before, err := zx.nextCacheKeyFor(files)
		if err != nil {
			t.Fatal(err)
		}
		patchWith(t, zx, []byte{0xF3, 0x18, 0xFE})
		after, err := zx.nextCacheKeyFor(files)
		if err != nil {
			t.Fatal(err)
		}
		diff := before.Diff(after)
		if len(diff) != 1 || diff[0] != "the ROM patches" {
			t.Errorf("a cache made without the patch would be used with it: the keys differ in %q, want the ROM patches", diff)
		}
	})

	// The firmware's own bank 0, as it is at the hand-over, for the second part.
	var firmwareBank0 [256]byte

	t.Run("a patch over bank 0 runs at the hand-over", func(t *testing.T) {
		zx := boot(t)
		// DI; LD A,2; OUT (0FEh),A; JR $ -- the border goes red and the CPU spins at 5.
		log := patchWith(t, zx, []byte{0xF3, 0x3E, 0x02, 0xD3, 0xFE, 0x18, 0xFE})
		if len(log.warned) != 1 {
			t.Errorf("want the one warning that the firmware replaces the ROMs, got %q", log.warned)
		}
		clean := zx.memory.rom[0]
		resetRun(zx, 1)
		if zx.memory.rom[0] != clean {
			t.Fatal("the patch was applied before the firmware had loaded the ROMs")
		}
		frames := 0
		for ; frames < 800 && !handedOver(zx); frames++ {
			zx.RunFrame()
		}
		if !handedOver(zx) {
			t.Fatal("the firmware did not hand over within 800 frames")
		}
		resetRun(zx, 5)
		if len(log.said) != 1 {
			t.Errorf("the patch should have reported once, at the hand-over: %q", log.said)
		}
		if zx.io.borderColor != 2 || zx.cpu.PC != 5 {
			t.Errorf("%d frames in the patch did not run: border %d, PC 0x%04X; want border 2 and PC 0x0005", frames, zx.io.borderColor, zx.cpu.PC)
		}
		t.Logf("the firmware handed over after %d frames and the patch ran", frames)

		// A reset runs the firmware again, and the patch goes over the ROMs it loads again.
		zx.Reset()
		resetRun(zx, 1)
		if !zx.memory.nextCfg.active {
			t.Fatal("Reset did not put the Next back in config mode")
		}
		for n := 0; n < 800 && !handedOver(zx); n++ {
			zx.RunFrame()
		}
		resetRun(zx, 5)
		if len(log.said) != 2 || zx.cpu.PC != 5 || zx.io.borderColor != 2 {
			t.Errorf("after a reset the patch was not applied again: reported %d times, border %d, PC 0x%04X", len(log.said), zx.io.borderColor, zx.cpu.PC)
		}
	})

	t.Run("a patch of the bytes the firmware loaded still reaches the menu", func(t *testing.T) {
		// What the firmware loads into bank 0, taken from a run with no patch.
		zx := boot(t)
		for n := 0; n < 800 && !handedOver(zx); n++ {
			zx.RunFrame()
		}
		if !handedOver(zx) {
			t.Fatal("the firmware did not hand over within 800 frames")
		}
		copy(firmwareBank0[:], zx.memory.rom[0][:256])

		zx = boot(t)
		log := patchWith(t, zx, firmwareBank0[:])
		over := 0
		for frames := 1; frames <= 6000; frames += 50 {
			resetRun(zx, 50)
			if over == 0 {
				if handedOver(zx) {
					over = frames
					if len(log.said) != 1 {
						t.Fatalf("the patch was not applied at the hand-over: %q", log.said)
					}
					if !bytes.Equal(zx.memory.rom[0][:256], firmwareBank0[:]) {
						t.Fatal("bank 0 is not what the firmware loaded after the same bytes were laid over it")
					}
				}
				continue
			}
			cyan := 0
			for cell := 4 * 32; cell < 16*32; cell++ {
				if zx.memory.screen.attributes[cell]&0x38 == 0x28 {
					cyan++
				}
			}
			if cyan >= 8 {
				t.Logf("handed over after %d frames, the NextZXOS menu was up after %d", over, frames)
				return
			}
			if (frames-over)%300 == 250 { // the welcome screen waits for a key
				zx.io.PressKey(7, 0)
				resetRun(zx, 10)
				zx.io.ReleaseKey(7, 0)
			}
		}
		t.Fatalf("NextZXOS did not come up with the firmware's own bytes laid over bank 0 (handed over at %d; 0 is never)", over)
	})
}
