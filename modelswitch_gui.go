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

	label, known, loaded := loadModelROMs(zx, model, romDir) // machine_model.go
	if !known {
		fmt.Printf("Menu bar: unknown model %q, not switching\n", model)
		return
	}

	if !loaded {
		fmt.Printf("Menu bar: failed to load ROM set for %s, keeping the previous model running\n", model)
		return
	}

	zx.Reset()
	zx.io.SetJoystickMode(defaultJoystickModeForModel(model))
	// A Next has its own mouse port; the mouse chosen for another model stays as it was,
	// but for the AMX mouse, which a Next cannot have.
	if model == "next" && zx.io.mouseMode != MouseKempston {
		zx.io.SetMouseMode(MouseKempston)
	}
	fmt.Printf("Switched to %s\n", label)
}

// applyCustomROMLive overrides one ROM bank of the currently running model
// with name (resolved against dir) and resets, so the new ROM actually
// takes effect immediately rather than sitting loaded but unexecuted.
func applyCustomROMLive(zx *ZenZX, dir, name string, bank int) {
	path := filepath.Join(dir, name)
	if err := zx.OverrideROMBank(bank, path); err != nil {
		fmt.Printf("Menu bar: could not apply %s to bank %d: %v\n", name, bank, err)
		return
	}
	zx.Reset()
	fmt.Printf("Loaded %s into ROM bank %d\n", name, bank)
}
