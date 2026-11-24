// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// The ROM switches (-rom, -rom0 .. -rom3, -rom-bin) on a Next that boots from its card.
//
// A Next has no ROM chips: its ROMs are the first 256 KiB of the board's SRAM, which the
// firmware fills from the card about 450 frames into the run. What the switches put in the
// banks before then is replaced, so on such a Next they are kept as a list of patches on the
// machine and written again when the firmware leaves config mode (NextREG 0x03 choosing the
// machine, then the soft reset that starts it): over what the firmware loaded, which is also
// what -rom-fill rom reads and what -rom-save writes. A hard reset runs the firmware again, and
// the list is applied again after it. docs/proposals/NEXT_ROM_OVERLAY.md, T-76.
//
// Everywhere else, and on a Next with no boot files, the switches act at once, as they always
// did.

// romPatch is one switch, as something that can be done again.
type romPatch struct {
	name  string // for the log: "-rom bank 0", "-rom-bin"
	key   string // what makes a boot cache depend on it: the name, the file's hash, the options
	fatal bool   // a failure at start is an error and not a warning
	do    func(zx *ZenZX) (summary string, warnings []string, err error)
}

// romSink is where the patches say what they did.
type romSink struct {
	say  func(string)
	warn func(string)
}

// romPatchSet is the machine's list of patches, and who they report to.
type romPatchSet struct {
	list  []romPatch
	sink  romSink
	noted bool // the warning that the firmware replaces the ROMs has been given
}

// romSwitches is what -rom, -rom0 .. -rom3 and -rom-bin ask for.
type romSwitches struct {
	list string    // -rom: files for banks 0, 1, 2 ... in order
	bank [4]string // -rom0 .. -rom3
	bin  *romBinFlags
}

// nextChainWillBoot reports whether the machine is a Next that is about to be given its boot
// files, which is when the firmware will load its own ROMs over whatever the switches did.
func nextChainWillBoot(zx *ZenZX, flagDir, model string, classic bool, explicit ...string) bool {
	if zx.memory == nil || !zx.memory.isNext || !wantNextBootChain(model, classic, explicit...) {
		return false
	}
	_, ok, _ := locateNextFiles(flagDir)
	return ok
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// romFilePatch is -rom or -romN: the file goes in the bank.
func romFilePatch(name string, bank int, path string) (romPatch, error) {
	sum, err := hashFile(path)
	if err != nil {
		return romPatch{}, err
	}
	return romPatch{
		name: name,
		key:  fmt.Sprintf("rom bank %d sha256:%s", bank, sum),
		do: func(zx *ZenZX) (string, []string, error) {
			if err := zx.OverrideROMBank(bank, path); err != nil {
				return "", nil, err
			}
			return fmt.Sprintf("ROM bank %d overridden: %s", bank, path), nil, nil
		},
	}, nil
}

// romBinPatch is -rom-bin.
func romBinPatch(f *romBinFlags) (romPatch, error) {
	sum, err := hashFile(*f.bin)
	if err != nil {
		return romPatch{}, fmt.Errorf("-rom-bin: cannot read %s: %v", *f.bin, err)
	}
	return romPatch{
		name:  "-rom-bin",
		key:   fmt.Sprintf("rom-bin sha256:%s at %q fill %s bank %d", sum, *f.at, *f.fill, *f.bank),
		fatal: true,
		do:    func(zx *ZenZX) (string, []string, error) { return f.apply(zx) },
	}, nil
}

// applyROMSwitches does what the switches ask, in their order: -rom, then -rom0 .. -rom3, then
// -rom-bin, so that -rom-fill rom overlays what the others left. When firmware is set, the
// machine is a Next that is about to load its ROMs from its card: the switches are kept, with a
// warning, and applied when the firmware has done. On any Next they are kept for the hard resets
// that run the firmware again.
func (zx *ZenZX) applyROMSwitches(s romSwitches, firmware bool, sink romSink) error {
	var patches []romPatch
	add := func(name string, bank int, path string) {
		p, err := romFilePatch(name, bank, path)
		if err != nil {
			sink.warn(fmt.Sprintf("%s: error reading ROM bank %d override %s: %v", name, bank, path, err))
			return
		}
		patches = append(patches, p)
	}
	if s.list != "" {
		for i, p := range strings.Split(s.list, ",") {
			if p = strings.TrimSpace(p); p != "" {
				add(fmt.Sprintf("-rom bank %d", i), i, p)
			}
		}
	}
	for bank, p := range s.bank {
		if p != "" {
			add(fmt.Sprintf("-rom%d", bank), bank, p)
		}
	}
	if s.bin != nil && s.bin.active() {
		if err := s.bin.check(); err != nil {
			return err
		}
		p, err := romBinPatch(s.bin)
		if err != nil {
			return err
		}
		patches = append(patches, p)
	}
	if len(patches) == 0 {
		return nil
	}

	if zx.memory != nil && zx.memory.isNext {
		zx.romPatches.list = append(zx.romPatches.list, patches...)
		zx.romPatches.sink = sink
	}
	if firmware {
		zx.warnROMSwitchesWaitForFirmware()
		return nil
	}
	for _, p := range patches {
		if err := zx.runROMPatch(p, sink); err != nil {
			return err
		}
	}
	return nil
}

// runROMPatch does one patch and reports it. A failure is returned when the patch is one that
// must not fail, and is a warning otherwise.
func (zx *ZenZX) runROMPatch(p romPatch, sink romSink) error {
	summary, warns, err := p.do(zx)
	for _, w := range warns {
		sink.warn(w)
	}
	if err != nil {
		if p.fatal {
			return err
		}
		sink.warn(fmt.Sprintf("%s: %v", p.name, err))
		return nil
	}
	sink.say(summary)
	return nil
}

// warnROMSwitchesWaitForFirmware says, once, that the switches given on a Next that boots from
// its card are applied when the firmware has loaded its ROMs and not before.
func (zx *ZenZX) warnROMSwitchesWaitForFirmware() {
	if zx.romPatches.noted || len(zx.romPatches.list) == 0 || zx.romPatches.sink.warn == nil {
		return
	}
	zx.romPatches.noted = true
	zx.romPatches.sink.warn("a Next loads its ROMs from its card, about 450 frames in, and replaces the ROM banks: " +
		"the ROM switches (-rom, -romN, -rom-bin) are applied when it has done, and again after a hard reset; " +
		"-rom-fill rom, -rom-save and the warning for a file that does not fit use the ROMs it loaded")
}

// replayROMPatches is the event "the ROMs are in place": the firmware has left config mode and
// the machine is starting. Called by serviceNextReset.
func (zx *ZenZX) replayROMPatches() {
	set := &zx.romPatches
	if len(set.list) == 0 || set.sink.say == nil {
		return
	}
	for _, p := range set.list {
		if err := zx.runROMPatch(p, set.sink); err != nil {
			set.sink.warn(fmt.Sprintf("%s: %v", p.name, err))
		}
	}
}

// romPatchKeys are the patches as a boot cache must know them: a cache made with other ROMs
// laid over the firmware's is another cache.
func (zx *ZenZX) romPatchKeys() []string {
	var keys []string
	for _, p := range zx.romPatches.list {
		keys = append(keys, p.key)
	}
	return keys
}

// noteROMPatchesForFirmware is called when a boot chain is attached: switches given at start
// are kept on the machine, and from now on they wait for the firmware.
func (zx *ZenZX) noteROMPatchesForFirmware() {
	zx.warnROMSwitchesWaitForFirmware()
}

// NextROMLive says which ROM the CPU reads at 0x0000-0x3FFF of a Next, and why: the boot ROM,
// the config window, the DivMMC, RAM paged over the ROM, or one of the SRAM's ROM banks, with
// the lock bits, NextREG 0x8C, the ROM the ports choose and the two lowest MMU slots. A patch to
// the wrong bank looks like a patch that does nothing, and this is how to tell.
func (zx *ZenZX) NextROMLive() string {
	m := zx.memory
	if m == nil || !m.isNext {
		return ""
	}
	var what string
	switch {
	case m.bootROMEnabled:
		what = "the boot ROM (config mode before the firmware has chosen the machine)"
	case m.nextCfg.active:
		what = fmt.Sprintf("the config window on SRAM bank %d (config mode)", m.nextCfg.bank)
	case zx.io != nil && zx.io.divMMC != nil && zx.io.divMMC.paged:
		what = "the DivMMC's ROM, paged over the ROM by the automap, for 0x0000-0x1FFF"
	case m.nextSlots[0] != 0xFF || m.nextSlots[1] != 0xFF:
		what = fmt.Sprintf("RAM pages %d and %d, paged over the ROM by the MMU", m.nextSlots[0], m.nextSlots[1])
	case m.nextAltROM&0xC0 == 0x80:
		what = fmt.Sprintf("the alternate ROM, SRAM bank %d", 6+nextAltIndex(m))
	default:
		what = fmt.Sprintf("ROM %d, SRAM bank %d", m.nextROMIndex(), m.nextROMIndex())
	}
	return fmt.Sprintf("Next ROM live: %s. ROM lock %d, ports choose ROM %d, NextREG 0x8C=0x%02X, MMU slots 0/1 = 0x%02X/0x%02X",
		what, (m.nextAltROM>>4)&0x03, m.romBank, m.nextAltROM, m.nextSlots[0], m.nextSlots[1])
}

// nextAltIndex is which of the two alternate ROMs (SRAM banks 6 and 7) the alternate ROM
// reads and writes reach: the lock bits of NextREG 0x8C, or the ROM the 0x7FFD port chooses.
func nextAltIndex(m *SpectrumMemory) uint8 {
	switch {
	case m.nextAltROM&0x20 != 0:
		return 1
	case m.nextAltROM&0x10 != 0:
		return 0
	}
	return (m.port7FFD >> 4) & 0x01
}
