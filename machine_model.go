package zenzx

import "fmt"

// loadModelROMs gives a machine the ROMs and the hardware of a model, as
// -model names it: the ROM set (from romDir, or the ones built into the
// program), and what goes with it, the floppy controller of a +3 or the
// processor and memory of a Next. It does not reset the machine, and it
// does not give a Next its boot files (nextchain.go).
//
// It returns the model's name for messages, whether the model is one it
// knows, and whether its ROMs were found and loaded.
//
// The GUI's live model switch and every machine after the first use it;
// the first machine of a run is still set up by the start-up code of each
// program, which has the command line's own options to apply as it goes.
func loadModelROMs(zx *ZenZX, model, romDir string) (label string, known, loaded bool) {
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

	case "next":
		if data, err := loadPlus3Bytes(romDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			loaded = zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil
			if loaded {
				zx.EnableZ80N()
				zx.memory.EnableNext()
				zx.memory.SetNextSlot(0, 0xFF) // ROM low half -- defers to legacy ROM (readNext)
				zx.memory.SetNextSlot(1, 0xFF) // ROM high half -- ditto
				zx.memory.SetNextSlot(2, 10)   // bank 5 low half (screen) -- classic reset default
				zx.memory.SetNextSlot(3, 11)   // bank 5 high half
				zx.memory.SetNextSlot(4, 4)    // bank 2 low half
				zx.memory.SetNextSlot(5, 5)    // bank 2 high half
				zx.memory.SetNextSlot(6, 0)    // bank 0 low half -- reset default before any 7FFD write
				zx.memory.SetNextSlot(7, 1)    // bank 0 high half
			}
		}
		label = "ZX Spectrum Next (+3 ROM base)"

	case "ts2068":
		if home, err := resolveROMBytes("ts2068-0.rom", romDir); err == nil {
			if ext, err := resolveROMBytes("ts2068-1.rom", romDir); err == nil {
				loaded = zx.LoadTS2068ROMBytes(home, ext) == nil
			}
		}
		label = "TS2068 (Home + Extension)"

	default:
		return "", false, false
	}
	return label, true, loaded
}

// newMachine makes a machine of the given model, reset and ready to run,
// with no sound device and no display of its own.
func newMachine(model, romDir string) (*ZenZX, error) {
	zx := NewZenZX(AudioBackendOto)
	label, known, loaded := loadModelROMs(zx, model, romDir)
	if !known {
		return nil, fmt.Errorf("no such model %q", model)
	}
	if !loaded {
		return nil, fmt.Errorf("the ROMs of the %s were not found in %s or in the program", label, romDir)
	}
	zx.Reset()
	zx.io.SetJoystickMode(defaultJoystickModeForModel(model))
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	return zx, nil
}
