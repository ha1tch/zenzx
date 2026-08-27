package main

import (
	"fmt"
	"os"
)

// ============================================================================
// TS2068 model support -- Stage 1 (docs/TS2068_DEVELOPMENT_PLAN.md)
//
// Guiding principle from the plan: because zenzx runs the real ROM
// images, almost everything that looks like "implement TS2068 system-
// software feature X" is actually "get the memory/port/timing substrate
// right and let the real ROM code do it." This file is entirely
// substrate: ROM loading and chunk-0 Home/Extension banking. No BASIC,
// interrupt-fielding, or OS-RAM-routine behaviour is implemented here --
// none of it needs to be. It's already in the ROM bytes.
// ============================================================================

// LoadTS2068ROMBytes is LoadTS2068ROM's byte-data equivalent, used when
// the caller has already resolved the Home/Extension ROM bytes (from
// the embedded binary or the filesystem -- see resolveROMBytes in
// zenzx_headless.go/zenzx_gui.go) rather than holding bare paths.
func (zx *ZenZX) LoadTS2068ROMBytes(home, ext []byte) error {
	if len(home) != 16384 {
		return fmt.Errorf("TS2068 Home ROM must be 16384 bytes, got %d", len(home))
	}
	if len(ext) != 8192 {
		return fmt.Errorf("TS2068 Extension ROM must be 8192 bytes, got %d", len(ext))
	}

	copy(zx.memory.rom[0][:], home)
	copy(zx.memory.ts2068ExtROM[:], ext)
	zx.memory.isTS2068 = true
	zx.memory.is128K = false
	zx.memory.isPlus3 = false

	// Stage 2: genuine NTSC timing, not the PAL default. 3.528MHz
	// (14.112MHz/4) and 262 lines/frame at 60.1145Hz (Technical Manual
	// 2.1.8.2/2.1.8.3) -- 3528000/60.1145 = 58688 cycles/frame, computed
	// precisely rather than the plan's rough "roughly 58,696" estimate.
	// The /INT pulse length stays the constructor default
	// (InterruptLength): the interrupt is asserted at the top of each
	// frame for a fixed short pulse, so the old per-model end-of-frame
	// threshold no longer exists.
	zx.cyclesPerFrame = 58688

	return nil
}

// LoadTS2068ROM loads the Home ROM (16K) and Extension ROM (8K) and
// switches memory into TS2068 mode. Deliberately does NOT set is128K --
// TS2068's RAM organisation (16K Home Bank RAM + 32K upper RAM, screen
// mirroring at 0x4000-0x5AFF) is structurally identical to the existing
// 48K path, so is128K stays false and that addressing is reused
// unchanged. Only isTS2068 distinguishes it, gating the ROM-region
// special-casing in memory.go's Read.
func (zx *ZenZX) LoadTS2068ROM(homePath, extPath string) error {
	home, err := os.ReadFile(homePath)
	if err != nil {
		return fmt.Errorf("error loading TS2068 Home ROM: %v", err)
	}
	ext, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("error loading TS2068 Extension ROM: %v", err)
	}
	return zx.LoadTS2068ROMBytes(home, ext)
}

// ts2068WritePort handles the ports Stage 1 and Stage 4 need: F4H
// (Horizontal Select Register, chunk-bank-select -- only bit 0, chunk 0,
// is meaningful yet; see the plan for why the other 7 bits are
// deferred), FFH bit 7 (EXROM/Dock select) and bits 0-2 (video mode,
// Stage 3), and F5H/F6H (AY-3-8912 sound chip -- same underlying chip
// and ayRegister field the 128K-style ports already use, just a
// different port address, per the Technical Manual's port map). Called
// from SpectrumIO.WritePort, gated on io.memory.isTS2068 so it's a
// complete no-op for every other model.
//
// Bits 1-7 of the HSR are tracked (so a program enabling them doesn't
// silently appear to do nothing) but have no effect: no Dock/cartridge
// content exists to switch in, per the plan's explicit scope (full
// 8-chunk Dock banking stays unimplemented indefinitely).
func (io *SpectrumIO) ts2068WritePort(port uint16, value uint8) bool {
	if !io.memory.isTS2068 {
		return false
	}
	switch port & 0xFF {
	case 0xF4:
		io.ts2068HSR = value
		io.memory.ts2068HSRChunk0 = value&0x01 != 0
		return true
	case 0xFF:
		io.ts2068Port0xFF = value
		io.memory.ts2068ExRomSelect = value&0x80 != 0
		if io.onTS2068VideoModeChange != nil {
			io.onTS2068VideoModeChange(value & 0x07)
		}
		return true
	case 0xF5:
		// AY-3-8912 register select. Write-only on real hardware
		// (Technical Manual Table 2.1.13-1) -- reuses the same
		// ayRegister field the existing 128K-style ports already use.
		io.ayRegister = value & 0x0F
		return true
	case 0xF6:
		// AY-3-8912 data write. Register 14 is the joystick I/O port in
		// practice (input direction), but TS2068 software reading
		// joysticks never writes here -- writes pass through to the AY
		// chip like any other register, no special-casing needed.
		if io.ayRegister < 16 {
			if io.audio != nil {
				io.audio.WriteAYRegister(io.ayRegister, value)
			}
			io.ayRegisters[io.ayRegister] = value
		}
		return true
	}
	return false
}

// ts2068AYJoystickRegister is the AY-3-8912 register (I/O Port A Data
// Store) TS2068 repurposes for joystick reading, per Technical Manual
// §2.1.6.1.
const ts2068AYJoystickRegister = 14

// ts2068ReadPort handles reading back the same ports. Real hardware
// documents FFH and F4H as R/W (Technical Manual Table 2.1.13-1);
// software that reads its own last-written value (the Extension ROM
// Interface Routine's IFRTN does exactly this, saving and restoring the
// HSR around a call) needs this to work, not just writes. F6H read is
// where the joystick ports actually live, per §2.1.6.1/§2.1.7: selected
// by having written register 14 to F5H, then reading F6H with address
// bit 8 (port 1) or bit 9 (port 2) set -- confirmed against
// Table 2.4.4-1's *DIR1/*BUTTON signal naming, active low.
func (io *SpectrumIO) ts2068ReadPort(port uint16) (uint8, bool) {
	if !io.memory.isTS2068 {
		return 0, false
	}
	switch port & 0xFF {
	case 0xF4:
		return io.ts2068HSR, true
	case 0xFF:
		return io.ts2068Port0xFF, true
	case 0xF6:
		if io.ayRegister == ts2068AYJoystickRegister {
			switch {
			case port&0x0100 != 0: // address bit 8: joystick port 1
				return ts2068JoystickByte(io.ts2068Joy1), true
			case port&0x0200 != 0: // address bit 9: joystick port 2
				return ts2068JoystickByte(io.ts2068Joy2), true
			default: // neither selected -- nothing pressed, active low
				return 0xFF, true
			}
		}
		if io.audio != nil && io.ayRegister < 16 {
			return io.audio.ReadAYRegister(io.ayRegister), true
		}
		return 0xFF, true
	}
	return 0, false
}

// ts2068JoystickByte renders a JoystickState as the byte port F6H would
// return for one joystick port: bits 0-3 up/down/left/right, bit 7
// fire, active low (Table 2.4.4-1's *DIR1/*BUTTON naming -- a different
// polarity from Kempston Joystick's active-high convention on the
// canonical Spectrum, and a different bit layout again from AMX
// Mouse's buttons).
func ts2068JoystickByte(s JoystickState) uint8 {
	b := uint8(0xFF)
	if s.Up {
		b &^= 0x01
	}
	if s.Down {
		b &^= 0x02
	}
	if s.Left {
		b &^= 0x04
	}
	if s.Right {
		b &^= 0x08
	}
	if s.Fire {
		b &^= 0x80
	}
	return b
}

// SetTS2068JoystickState updates one of TS2068's two built-in joystick
// ports. player is 1 or 2; any other value is a no-op.
func (io *SpectrumIO) SetTS2068JoystickState(player int, s JoystickState) {
	switch player {
	case 1:
		io.ts2068Joy1 = s
	case 2:
		io.ts2068Joy2 = s
	}
}
