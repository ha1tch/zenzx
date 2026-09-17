package main

// ============================================================================
// ZX Spectrum Next: Layer 2 Access Port, IO port 0x123B (T-29 completion
// pass, wave ti0.r3)
//
// Confirmed against wiki.specnext.dev/Layer_2_Access_Port: this is a CPU
// memory-PAGING feature, mapping Layer 2 framebuffer memory into the Z80's
// own 16-bit address space for direct reads/writes -- it has NOTHING to do
// with which bank is DISPLAYED (that is always NR 0x12, unconditionally,
// per the wiki's own bit-1 description: "Layer 2 visible ... controls
// what appears on screen (always driven by NextReg:$12, regardless of
// bit 3)"). This corrects an earlier framing in docs/TRACKING.md's T-29
// entry, which described NR 0x13/port 0x123B loosely as a "shadow screen"
// switch -- the register genuinely only matters as a target for THIS
// port's CPU-paging window, never for display.
//
// Bit layout, standard mode (bit 4 = 0):
//
//	bits 7-6  video RAM bank select within the mapped bank: which 16K
//	          third of the target bank appears at the CPU's lower 16K
//	          address window (0x0000-0x3FFF, this file's mappedWindow):
//	          %00 = first 16K, %01 = second 16K, %10 = third 16K,
//	          %11 = all 48K (core 3.0+ -- not implemented here, see
//	          layer2AccessPortWindow's own comment)
//	bit 3     register select: 0 = target is NR 0x12 (visible bank),
//	          1 = target is NR 0x13 (shadow bank) -- this is the ONLY
//	          place NR 0x13 has any effect at all
//	bit 2     read enable: mapped reads return Layer 2 memory instead of
//	          whatever ROM/RAM would otherwise be there
//	bit 1     "Layer 2 visible" -- per the wiki, ignored for actual
//	          display purposes (always NR 0x12); stored for read-back
//	          completeness only
//	bit 0     write enable: mapped writes go to Layer 2 memory instead
//	          of whatever ROM/RAM would otherwise be there
//
// Extended mode (bit 4 = 1, core 3.0.7+): bits 2-0 become a bank offset
// (+0..+7) shifting the mapped window rather than the bits-7-6 selector
// above. Not implemented here -- no source consulted gives a documented
// consumer of this mode, and NEXT_SUPPORT_DEVELOPMENT_PLAN.md's own
// ti0.r3 "done when" bar does not require it; stored-and-ignored, the
// same convention this codebase already uses for every other
// documented-but-out-of-scope bit (e.g. sprite.go's mirror/rotate before
// this same pass, nextreg.go's NR 0x13 before this file existed).
// ============================================================================

// layer2AccessPortAddr is IO port 0x123B, fully 16-bit decoded on real
// hardware -- checked by exact match in io.go's ReadPort/WritePort,
// alongside nextRegSelectPort/nextRegDataPort, before any partial-decode
// legacy port branch runs.
const layer2AccessPortAddr = 0x123B

// layer2AccessPort holds the raw byte last written to port 0x123B, plus
// the decoded fields readNext/writeNext (memory.go) consult on every
// access to the CPU's lower 16K window. Owned by SpectrumIO the same way
// sprites/nextPalette are (io.go), constructed by NewSpectrumIO.
type layer2AccessPort struct {
	raw uint8
}

// bankSelect returns bits 7-6: which 16K third of the target bank is
// mapped into the CPU's lower window. Extended mode (bit 4 set) is not
// implemented -- see file header -- so this is only meaningful when
// extended() is false.
func (p *layer2AccessPort) bankSelect() uint8 {
	return (p.raw >> 6) & 0x03
}

// usesShadowBank reports bit 3: whether the mapped bank is NR 0x13
// (shadow) rather than NR 0x12 (visible) -- the only place this
// distinction exists anywhere in this emulator.
func (p *layer2AccessPort) usesShadowBank() bool {
	return p.raw&0x08 != 0
}

// readEnabled/writeEnabled decode bits 2 and 0 respectively.
func (p *layer2AccessPort) readEnabled() bool  { return p.raw&0x04 != 0 }
func (p *layer2AccessPort) writeEnabled() bool { return p.raw&0x01 != 0 }

// extended reports bit 4 -- see file header on why this emulator does
// not implement the resulting bank-offset addressing mode.
func (p *layer2AccessPort) extended() bool { return p.raw&0x10 != 0 }

// targetPage resolves this port's current settings to the actual 8K
// page (the same page numbering nextPageRead/nextPageWrite,
// memory.go, and layer2BaseBank, layer2.go, already use) that the CPU's
// lower 16K window should read/write through, or ok=false if the port's
// settings don't produce a valid mapping this emulator implements
// (extended mode, or neither read nor write enabled -- matching real
// hardware's behaviour of simply not intercepting the access at all in
// that case).
func (io *SpectrumIO) layer2AccessPortTargetPage(offset uint16) (page uint8, ok bool) {
	p := &io.layer2Access
	if p.extended() {
		return 0, false
	}
	if !p.readEnabled() && !p.writeEnabled() {
		return 0, false
	}
	var baseBank uint8
	if p.usesShadowBank() {
		baseBank = io.GetNextRegister(0x13)
	} else {
		baseBank = io.layer2BaseBank()
	}
	// bankSelect picks which 16K (two 8K pages) third of the mapped
	// bank's own 48K span this maps -- %11 (all 48K) is core 3.0+ and
	// not implemented (file header); treated as %00 (first 16K) as the
	// closest safe fallback, the same "fall back to a real implemented
	// case rather than invent behaviour" choice layer2CurrentMode makes
	// for NR 0x70's own reserved %11 pattern.
	third := p.bankSelect()
	if third > 2 {
		third = 0
	}
	pageOffsetWithinBank := uint8(third) * 2 // each third is 2 8K pages
	// offset is the CPU-side address within the 16K window (0x0000-
	// 0x3FFF, i.e. slots 0-1, 8K each) -- pick the right one of the
	// mapped third's own two 8K pages.
	pageOffsetWithinBank += uint8(offset / 0x2000)
	return baseBank + pageOffsetWithinBank, true
}

// layer2AccessPortRead/Write are readNext/writeNext's (memory.go) hook
// into this port: called first, before the normal MMU slot lookup, for
// any access landing in the CPU's lower 16K (address < 0x4000) -- the
// only range this port ever maps, per the wiki ("lower memory window").
// ok=false means the port isn't currently mapping this access at all
// (extended mode, or neither enable bit set) -- the caller should fall
// through to the ordinary MMU slot path in that case, matching real
// hardware leaving the normal memory map untouched when this port isn't
// actively intercepting.
func (io *SpectrumIO) layer2AccessPortRead(address uint16) (value uint8, ok bool) {
	if address >= 0x4000 || !io.layer2Access.readEnabled() {
		return 0, false
	}
	page, mapped := io.layer2AccessPortTargetPage(address)
	if !mapped {
		return 0, false
	}
	return io.memory.nextPageRead(page, address&0x1FFF), true
}

func (io *SpectrumIO) layer2AccessPortWrite(address uint16, value uint8) (handled bool) {
	if address >= 0x4000 || !io.layer2Access.writeEnabled() {
		return false
	}
	page, mapped := io.layer2AccessPortTargetPage(address)
	if !mapped {
		return false
	}
	io.memory.nextPageWrite(page, address&0x1FFF, value)
	return true
}
