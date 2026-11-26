//go:build !headless

package zenzx

import (
	"fmt"
	"path/filepath"
)

// switchModelLive reloads model's standard ROM set into the already-
// running zx and resets it, mirroring zenzx_gui.go's startup switch
// statement (the set-up itself is loadModelROMs, machine_model.go, which
// the headless program's extra machines share) -- same ROM files, same
// +3-style FDC handling -- but without
// consulting any CLI-only flags (-disk, -noFdc, -debugFdc), which don't
// have a live equivalent here. FDC is unconditionally disabled first, then
// re-enabled only if the new model is plus3/spanishplus3, so switching
// away from +3 to anything else doesn't leave it running against a ROM
// set that no longer expects it.
//
// Joystick mode is reset to model's own default (defaultJoystickModeForModel)
// unconditionally -- a deliberate simplification. There's no live record
// here of whether -joystick was explicitly set at startup, so "switching
// machines resets to that machine's natural default" is the simplest
// correct behaviour, even though it means an explicit startup override
// doesn't survive a live switch.
func switchModelLive(zx *ZenZX, model, romDir string) {
	zx.io.DisableFDC()
	wasTS2068 := zx.memory.isTS2068

	label, known, loaded := loadModelROMs(zx, model, romDir) // machine_model.go
	if !known {
		fmt.Printf("Menu bar: unknown model %q, not switching\n", model)
		zx.notice("There is no model called %q: the machine is as it was", model)
		return
	}

	if !loaded {
		fmt.Printf("Menu bar: failed to load ROM set for %s, keeping the previous model running\n", model)
		zx.notice("The ROMs of the %s could not be loaded: the machine is as it was", model)
		return
	}

	zx.wipeForNewModel(wasTS2068)
	zx.Reset()
	zx.io.SetJoystickMode(defaultJoystickModeForModel(model))
	// A Next has its own mouse port; the mouse chosen for another model stays as it was,
	// but for the AMX mouse, which a Next cannot have.
	if model == "next" && zx.io.mouseMode != MouseKempston {
		zx.io.SetMouseMode(MouseKempston)
	}
	fmt.Printf("Switched to %s\n", label)
	zx.notice("Switched to the %s", label)
}

// wipeForNewModel makes the machine as a new one of another model is: what the old model had in its
// RAM and on its screen is gone (a switch is not a reset, which keeps RAM), the screen shown is the first,
// and the video mode of a TS2068 is not left behind. Reset, which follows, does the rest, and on a Next
// does it at once: see Reset.
func (zx *ZenZX) wipeForNewModel(wasTS2068 bool) {
	m := zx.memory
	m.ram = [8][16384]byte{}
	m.nextExtraRAM = nil
	m.nextPhys = nextPhysical{}
	m.screenBank = 5 // not through updateScreenBank, which would save the old screen over the new RAM
	clear(zx.screen.bitmap)
	clear(zx.screen.attributes)
	// the border is black until the machine sets it, and the picture the display last drew is not this
	// machine's: a paused machine runs no frame to replace it
	zx.io.SetBorderColor(0)
	zx.io.borderHistory.Clear()
	zx.display.SetBorderChanges(nil, zx.frameOrigin, zx.cyclesPerFrame)
	if dm, ok := zx.display.(*DisplayManager); ok {
		dm.SetBorderColor(0)
	}
	if wasTS2068 {
		zx.SelectVideoRenderer("") // the TS2068's screen modes are its own
	}
}

// applyCustomROMLive overrides one ROM bank of the currently running model
// with name (resolved against dir) and resets, so the new ROM actually
// takes effect immediately rather than sitting loaded but unexecuted.
func applyCustomROMLive(zx *ZenZX, dir, name string, bank int) {
	path := filepath.Join(dir, name)
	if err := zx.OverrideROMBank(bank, path); err != nil {
		fmt.Printf("Menu bar: could not apply %s to bank %d: %v\n", name, bank, err)
		zx.notice("%s could not be used as ROM bank %d: %v", name, bank, err)
		return
	}
	zx.Reset()
	fmt.Printf("Loaded %s into ROM bank %d\n", name, bank)
	zx.notice("%s is ROM bank %d now, and the machine is reset", name, bank)
}
