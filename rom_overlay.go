// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ha1tch/zenzx/pkg/romimage"
)

// -rom-bin: a ROM made on the fly from a binary file of any size.
//
// The binary goes at an address in a ROM bank (0x0000 unless -rom-at says
// another), and what is around it is zeroes or the bank's own ROM, so that the
// binary acts as an overlay. The result is exactly the size of the bank, as any
// ROM image is; with -rom-save it is also written to a file, to be loaded later
// with -rom like any other ROM. scripts/mkrom.py makes the same image outside
// ZenZX, and the two agree byte for byte.
//
// It is applied after -rom and -rom0 .. -rom3, so that -rom-fill rom overlays
// the ROM those left in the bank, and before -custom-roms-menu, which stays the
// last word.

// romBinFlags are the flags of -rom-bin, registered on a flag set by
// addROMBinFlags and read by apply.
type romBinFlags struct {
	bin  *string
	at   *string
	fill *string
	bank *int
	save *string
}

// addROMBinFlags registers -rom-bin and its companions on fs.
func addROMBinFlags(fs *flag.FlagSet) *romBinFlags {
	return &romBinFlags{
		bin:  fs.String("rom-bin", "", "Make a ROM on the fly from this binary file, of any size, and use it as a ROM bank; see -rom-at, -rom-fill, -rom-bank, -rom-save"),
		at:   fs.String("rom-at", "", "Where in the ROM the -rom-bin file goes: 0x1234, $1234 or decimal (default 0x0000). On a Next it counts from the start of -rom-bank, and a file may run on into the next banks, up to the 256 KiB"),
		fill: fs.String("rom-fill", "zero", "What is around the -rom-bin file: zero, or rom (the bank's own ROM, so the file is an overlay on it; on a Next booted from its card, what the firmware loaded)"),
		bank: fs.Int("rom-bank", 0, "The ROM bank the -rom-bin ROM goes in (default 0); 0-7 on a Next, whose banks 4-7 are the DivMMC ROM, the Multiface and the alternate ROMs"),
		save: fs.String("rom-save", "", "Also write the -rom-bin ROM to this file, to load later with -rom"),
	}
}

// active reports whether -rom-bin was given.
func (f *romBinFlags) active() bool { return *f.bin != "" }

// check reports a companion flag given without -rom-bin, or a value that is no
// good, before any machine is made.
func (f *romBinFlags) check() error {
	if !f.active() {
		for name, set := range map[string]bool{
			"-rom-at": *f.at != "", "-rom-fill": *f.fill != "zero", "-rom-bank": *f.bank != 0, "-rom-save": *f.save != "",
		} {
			if set {
				return fmt.Errorf("%s needs -rom-bin, the file to build the ROM from", name)
			}
		}
		return nil
	}
	if *f.fill != "zero" && *f.fill != "rom" {
		return fmt.Errorf("-rom-fill %q: it is zero or rom", *f.fill)
	}
	if *f.at != "" {
		if _, err := romimage.ParseAddress(*f.at); err != nil {
			return fmt.Errorf("-rom-at: %v", err)
		}
	}
	return nil
}

// ROMBankSize is the size of a ROM bank of the loaded model: 16 KiB, except the
// TS2068's extension ROM, which is 8 KiB.
func (zx *ZenZX) ROMBankSize(bank int) int {
	if zx.memory.isTS2068 && bank == 1 {
		return 8192
	}
	return romimage.Size
}

// ROMBankBytes is a copy of the bytes of a ROM bank as it is now.
func (zx *ZenZX) ROMBankBytes(bank int) ([]byte, error) {
	if bank < 0 || bank > zx.maxROMBank() {
		return nil, fmt.Errorf("rom bank %d not valid for this model (valid range: 0-%d)", bank, zx.maxROMBank())
	}
	if zx.memory.isTS2068 && bank == 1 {
		return append([]byte(nil), zx.memory.ts2068ExtROM[:]...), nil
	}
	return append([]byte(nil), zx.memory.rom[bank][:]...), nil
}

// apply builds the ROM and puts it in its bank. It returns a line for the log
// saying what was done, and warnings (a binary cut at the end of the ROM, an
// empty one) to be shown as warnings. The machine must have its standard ROM
// set loaded already.
//
// On a Next the ROM is the 256 KiB of SRAM below the RAM, and the address is a
// physical one: -rom-bank starts it at that bank (0-7) and -rom-at counts from
// there, so a file can run across banks. What is built, saved and written is
// the banks the file touches, whole (T-76).
func (f *romBinFlags) apply(zx *ZenZX) (summary string, warnings []string, err error) {
	if err := f.check(); err != nil {
		return "", nil, err
	}
	next := zx.memory.isNext
	bank := *f.bank
	limit := zx.maxROMBank()
	if next {
		limit = nextROMBinBanks - 1
	}
	if bank < 0 || bank > limit {
		return "", nil, fmt.Errorf("-rom-bank %d not valid for this model (valid range: 0-%d)", bank, limit)
	}
	address := 0
	if *f.at != "" {
		address, _ = romimage.ParseAddress(*f.at)
	}
	payload, err := os.ReadFile(*f.bin)
	if err != nil {
		return "", nil, fmt.Errorf("-rom-bin: cannot read %s: %v", *f.bin, err)
	}

	size := zx.ROMBankSize(bank)
	regionStart := 0 // where the banks built start, in the Next's SRAM
	phys := 0        // where the file starts, in the Next's SRAM
	if next {
		phys = bank*romimage.Size + address
		if phys >= nextROMSpace {
			return "", nil, fmt.Errorf("-rom-at: 0x%X is past the Next's 256 KiB of ROM (0x%X)", phys, nextROMSpace-1)
		}
		last := phys
		if len(payload) > 0 {
			last = phys + len(payload) - 1
		}
		if last >= nextROMSpace {
			last = nextROMSpace - 1
		}
		first := phys / romimage.Size
		regionStart = first * romimage.Size
		size = (last/romimage.Size - first + 1) * romimage.Size
		address = phys - regionStart
	}
	base := romimage.Zeroes(size)
	around := "zeroes"
	if *f.fill == "rom" {
		if next {
			base = zx.nextROMRegion(regionStart, size)
			around = fmt.Sprintf("what the ROM held at 0x%06X-0x%06X", regionStart, regionStart+size-1)
		} else {
			if base, err = zx.ROMBankBytes(bank); err != nil {
				return "", nil, err
			}
			around = fmt.Sprintf("what ROM bank %d held", bank)
		}
	}

	image, cut, err := romimage.Build(payload, address, base)
	if err != nil {
		return "", nil, fmt.Errorf("-rom-at: %v", err)
	}
	if len(payload) == 0 {
		warnings = append(warnings, "-rom-bin: the file is empty; the ROM is only what is around it")
	}
	if cut > 0 {
		if next {
			warnings = append(warnings, "-rom-bin: "+romimage.CutWarning(len(payload), phys, nextROMSpace, cut))
		} else {
			warnings = append(warnings, "-rom-bin: "+romimage.CutWarning(len(payload), address, size, cut))
		}
	}

	saved := ""
	if *f.save != "" {
		if same, _ := sameFile(*f.save, *f.bin); same {
			return "", warnings, fmt.Errorf("-rom-save: %s is the -rom-bin file; give another name", *f.save)
		}
		if err := os.WriteFile(*f.save, image, 0o644); err != nil {
			return "", warnings, fmt.Errorf("-rom-save: cannot write %s: %v", *f.save, err)
		}
		saved = ", saved as " + *f.save
	}

	if next {
		zx.writeNextROMRegion(regionStart, image)
	} else if err := zx.OverrideROMBankBytes(bank, image); err != nil {
		return "", warnings, err
	}

	placed := len(payload) - cut
	where := "nothing placed"
	if next {
		if placed > 0 {
			where = fmt.Sprintf("%s at 0x%06X-0x%06X (%d bytes)", baseName(*f.bin), phys, phys+placed-1, placed)
		}
		return fmt.Sprintf("Next ROM built at 0x%06X-0x%06X: %s, around it %s%s", regionStart, regionStart+size-1, where, around, saved), warnings, nil
	}
	if placed > 0 {
		where = fmt.Sprintf("%s at 0x%04X-0x%04X (%d bytes)", baseName(*f.bin), address, address+placed-1, placed)
	}
	return fmt.Sprintf("ROM bank %d built: %s, around it %s%s", bank, where, around, saved), warnings, nil
}

// sameFile reports whether two paths are the same existing file.
func sameFile(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ia, ib), nil
}

// baseName is the last element of a path, with either slash.
func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}
