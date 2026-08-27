//go:build !headless

package main

import (
	"fmt"
	"path/filepath"
)

// switchModelLive reloads model's standard ROM set into the already-
// running zx and resets it, mirroring zenzx_gui.go's startup switch
// statement -- same ROM files, same +3-style FDC handling -- but without
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

	var loaded bool
	var label string

	switch model {
	case "48k":
		if data, err := resolveROMBytes("48.rom", romDir); err == nil {
			loaded = zx.LoadROMBytes(data) == nil
		}
		label = "48K"

	case "128k":
		if data0, err := resolveROMBytes("128-0.rom", romDir); err == nil {
			if data1, err := resolveROMBytes("128-1.rom", romDir); err == nil {
				loaded = zx.Load128KROMBytes(data0, data1) == nil
			}
		}
		label = "128K (Sinclair)"

	case "plus2":
		if data0, err := resolveROMBytes("plus2-0.rom", romDir); err == nil {
			if data1, err := resolveROMBytes("plus2-1.rom", romDir); err == nil {
				loaded = zx.Load128KROMBytes(data0, data1) == nil
			}
		}
		label = "Spectrum +2 (grey)"

	case "plus2a":
		if data, err := loadPlus3Bytes(romDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			loaded = zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil
		}
		label = "Spectrum +2A (+3 ROM, no floppy controller)"

	case "plus3":
		if data, err := loadPlus3Bytes(romDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			loaded = zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil
			if loaded {
				zx.io.EnableFDC()
			}
		}
		label = "Spectrum +3"

	case "spanish48k":
		if data, err := resolveROMBytes("48-spanish.rom", romDir); err == nil {
			loaded = zx.LoadROMBytes(data) == nil
		}
		label = "Spanish 48K"

	case "spanish128k":
		if data0, err := resolveROMBytes("128-spanish-0.rom", romDir); err == nil {
			if data1, err := resolveROMBytes("128-spanish-1.rom", romDir); err == nil {
				loaded = zx.Load128KROMBytes(data0, data1) == nil
			}
		}
		label = "Spanish 128K"

	case "spanishplus2":
		if data0, err := resolveROMBytes("plus2-spanish-0.rom", romDir); err == nil {
			if data1, err := resolveROMBytes("plus2-spanish-1.rom", romDir); err == nil {
				loaded = zx.Load128KROMBytes(data0, data1) == nil
			}
		}
		label = "Spanish Spectrum +2"

	case "spanishplus3":
		if data, err := loadPlus3Bytes(romDir, "plus3-spanish-0.rom", "plus3-spanish-1.rom", "plus3-spanish-2.rom", "plus3-spanish-3.rom"); err == nil {
			loaded = zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil
			if loaded {
				zx.io.EnableFDC()
			}
		}
		label = "Spanish Spectrum +3"

	case "ts2068":
		if home, err := resolveROMBytes("ts2068-0.rom", romDir); err == nil {
			if ext, err := resolveROMBytes("ts2068-1.rom", romDir); err == nil {
				loaded = zx.LoadTS2068ROMBytes(home, ext) == nil
			}
		}
		label = "TS2068 (Home + Extension)"

	default:
		fmt.Printf("Menu bar: unknown model %q, not switching\n", model)
		return
	}

	if !loaded {
		fmt.Printf("Menu bar: failed to load ROM set for %s, keeping the previous model running\n", model)
		return
	}

	zx.Reset()
	zx.io.SetJoystickMode(defaultJoystickModeForModel(model))
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
