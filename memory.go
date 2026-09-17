package main

import "fmt"

// ============================================================================
// Memory Constants
// ============================================================================

const (
	ROMSize = 16384 // 16KB
	RAMSize = 49152 // 48KB
)

// Next MMU page numbering (T-28, wave ti0.r2, item #19; RAM-capacity
// correction 2026-09-15). Logical 8K page numbers are the NextREG
// 0x50-0x57 register-visible convention (already used by nextreg.go's
// mmuBootDefaults): classic 16K bank N is pages 2N/2N+1. Pages below
// nextExtraPageBase deliberately alias the existing `ram` array (see
// nextPageRead/nextPageWrite) rather than duplicating storage, so
// Next-mode and classic-128K/+3 addressing of the same bank see the
// same bytes.
//
// Capacity correction: an earlier version of this code capped
// nextTotalPages at 96 (768K), wrongly treating T-28's own "at least
// the 768K-1MB unexpanded-machine tier" wording as a ceiling. It is a
// floor, not a ceiling -- jnext's own source (src/memory/ram.h,
// FPGA-REPO-ANALYSIS.md) states the real hardware supports 768K
// (unexpanded board) OR 2048K (expanded board), and jnext's emulator
// always allocates for the 2048K ceiling regardless of which physical
// board is modelled. zenzx now does the same: nextTotalPages targets
// the full expanded-board capacity.
//
// The real hardware ceiling is NOT a flat 2048K/8K=256 logical pages,
// though. Confirmed directly against jnext's Mmu::rebuild_ptr
// (src/memory/mmu.cpp): register values 0xE0-0xFF (32 of the 256
// possible 8-bit page numbers) are a reserved sentinel range, gated
// BEFORE any RAM lookup -- for slots 2-7 the slot goes inactive
// (floating-bus read, dropped write); for slots 0/1 it defers to
// legacy ROM paging instead of addressing RAM at all. Only register
// values 0x00-0xDF (224 values) ever reach ordinary RAM on real
// hardware. jnext's own to_sram_page()'s "+0x20 wrap" shift on top of
// that is purely so jnext's ROM-in-SRAM images (its own physical pages
// 0-7) don't collide with page 0 RAM -- an implementation detail of
// how jnext's Ram backing store is laid out, and irrelevant here since
// zenzx keeps ROM in its own separate `rom` array, never in the Next
// RAM backing store.
//
// So zenzx's honest ceiling is 224 ordinary logical pages (0x00-0xDF)
// -- 1792K -- not 2048K flatly. nextTotalPages=224 * 8K = 1792K:
// existing 128K (16 pages, aliased) + 1664K/208 pages of new backing
// store. Pages 0xE0-0xFF are explicitly gated as reserved in
// nextPageRead/nextPageWrite (see nextPageIsReserved), matching real
// hardware, rather than merely falling outside nextTotalPages by
// accident of array bounds.
const (
	nextExtraPageBase = 16
	nextTotalPages    = 224

	// nextReservedPageBase..255 mirrors real hardware's NR 0x50-0x57
	// sentinel range (jnext Mmu::rebuild_ptr's `page >= 0xE0` gate):
	// never ordinary RAM regardless of nextTotalPages. Checked
	// explicitly by nextPageIsReserved so the behaviour doesn't depend
	// on nextTotalPages happening to be small enough for array bounds
	// to catch it as a side effect.
	nextReservedPageBase = 0xE0
)

// ============================================================================
// Memory Implementation for Z80
// ============================================================================

type SpectrumMemory struct {
	// +3 model has:
	// - 4 ROM banks (16KB each)
	// - 8 RAM banks (16KB each)
	rom    [4][16384]byte // ROM banks 0-3
	ram    [8][16384]byte // RAM banks 0-7
	screen *SpectrumScreen

	// Memory configuration
	romBank     uint8 // Currently selected ROM bank (0-3)
	ramBankLow  uint8 // RAM bank at 0x4000-0x7FFF
	ramBankHigh uint8 // RAM bank at 0x8000-0xBFFF
	ramBankTop  uint8 // RAM bank at 0xC000-0xFFFF

	// Paging control
	pagingLocked  bool  // When true, no more paging allowed
	screenBank    uint8 // Which RAM bank is displayed (5 or 7)
	is128K        bool  // True for 128K mode, false for 48K mode
	isPlus3       bool  // True for +2A/+3 mode with 4 ROMs
	specialMode   uint8 // +3 special paging config index (0-3); valid only when specialPaging is true
	specialPaging bool  // +3 special (all-RAM) paging active (port 0x1FFD bit 0)
	port7FFD      uint8 // Last value written to 0x7FFD
	port1FFD      uint8 // Last value written to 0x1FFD (+3 only)

	// TS2068 model support (ts2068.go). Kept orthogonal to is128K/isPlus3:
	// TS2068 sets is128K=false deliberately, to reuse the existing 48K-style
	// RAM addressing path unchanged (structurally identical: 16K Home Bank
	// RAM + 32K upper RAM, screen mirroring at 0x4000-0x5AFF) -- only the
	// ROM region (0000H-3FFFH) needs TS2068-specific handling.
	isTS2068          bool
	ts2068ExtROM      [8192]byte // Extension ROM, 8K, chunk 0 only
	ts2068HSRChunk0   bool       // Port F4H (HSR) bit 0: chunk 0 selected out of Home Bank
	ts2068ExRomSelect bool       // Port FFH bit 7: selected-chunks source is Extension ROM (true) vs Dock (false)

	// Next MMU (T-28, wave ti0.r2, item #19): 8 independent 8K-granularity
	// slots covering the full 64K address space, additive to -- never a
	// replacement for -- ramBankLow/ramBankHigh/ramBankTop above. Every
	// existing 48K/128K/+3/TS2068 Read/Write branch is completely
	// untouched by this; isNext defaults false (zero value) and is the
	// only gate that reaches this new path at all. See docs/TRACKING.md
	// T-28 for the jnext-source-comparison basis of the design (real
	// hardware/VHDL, not the wiki alone): register-visible page numbers
	// are 8K logical indices where classic 16K bank N is pages 2N/2N+1 --
	// already the convention nextreg.go's mmuBootDefaults uses.
	isNext bool

	// nextSlots holds the logical 8K page number currently mapped into
	// each of the 8 slots (address>>13). Driven by NextREG 0x50-0x57
	// writes (T-28 item #26, nextreg.go's nextRegWriteDirect/
	// SetNextRegister via SetNextSlot) and, for slots 6/7 specifically,
	// also by the legacy 0x7FFD/0xDFFD ports on a genuine Next machine
	// (T-28 item #27, recomposeNextLegacySlots below) -- both paths
	// converge on the same SetNextSlot, so nothing else needs to know
	// which register or port actually drove a given slot.
	nextSlots [8]uint8

	// layer2AccessReadFn/WriteFn are readNext/writeNext's hook into IO
	// port 0x123B's CPU-memory-paging state (layer2accessport.go, T-29
	// completion pass): that state genuinely lives on SpectrumIO (it
	// reads NR 0x12/0x13, exactly like layer2Pixel's own io parameter
	// already does), but Read/Write/readNext/writeNext are plain
	// SpectrumMemory methods with no SpectrumIO reference threaded
	// through their many call sites -- adding one everywhere would be a
	// far bigger change than this one port justifies. Wired by
	// NewSpectrumIO the same closure-injection pattern SpectrumIO itself
	// already uses in the opposite direction (tapePlayingFn,
	// floatingBusFn, onTS2068VideoModeChange, all set by NewZenZX) --
	// just applied here to cross the boundary the other way. nil on any
	// SpectrumMemory not constructed alongside a SpectrumIO (e.g. a bare
	// memory-only test), in which case readNext/writeNext fall straight
	// through to the ordinary MMU slot path, matching real hardware with
	// this port never written (its power-on state maps nothing).
	layer2AccessReadFn  func(address uint16) (value uint8, ok bool)
	layer2AccessWriteFn func(address uint16, value uint8) (handled bool)

	// nextExtraRAM backs logical pages nextExtraPageBase..nextTotalPages-1
	// (0x10-0xDF): 208 pages, 1664K -- pages 0..nextExtraPageBase-1
	// deliberately alias the EXISTING `ram` array instead of duplicating
	// storage (see nextPageRead/nextPageWrite) so Next-mode and
	// classic-128K/+3 addressing of the same physical bank see the same
	// bytes -- required for compatibility, not optional. Together with
	// the 16 aliased pages this reaches the full 1792K (224-page) real
	// hardware ceiling; pages nextReservedPageBase (0xE0) and above are
	// never backed by storage at all -- see nextPageIsReserved.
	nextExtraRAM [nextTotalPages - nextExtraPageBase][8192]byte

	// nextLegacyBank7FFD/nextLegacyPortDFFD (T-28 item #27): the raw
	// values most recently written to ports 0x7FFD/0xDFFD, tracked for
	// the Next-mode legacy-paging path ONLY -- deliberately separate
	// storage from the classic port7FFD/port1FFD fields above and from
	// the classic is128K/pagingLocked gates, never read or written by
	// SetPaging/SetPlus3Paging and never read by them either. This is
	// intentional, not an oversight: a genuine Next machine is not
	// required to also be is128K (EnableNext does not set it -- see its
	// own doc comment), so SetPaging's own `if !m.is128K { return }`
	// gate cannot be relied on to have stored anything by the time a
	// Next-mode machine needs port7FFD's value; and 0xDFFD itself has no
	// defined role on any non-Next machine this emulator models
	// (48K/128K/+3/TS2068) -- real hardware assigns it only via the
	// Next's own Profi-heritage extended paging. Both fields are inert
	// (0x00 default) and harmless on a non-Next machine; only
	// recomposeNextLegacySlots (isNext-gated) ever reads them. See
	// SetNextLegacyBank7FFD/SetNextLegacyPortDFFD.
	nextLegacyBank7FFD uint8
	nextLegacyPortDFFD uint8
}

func NewSpectrumMemory(screen *SpectrumScreen) *SpectrumMemory {
	m := &SpectrumMemory{
		screen:      screen,
		romBank:     0, // Start with ROM 0
		ramBankLow:  5, // Default config
		ramBankHigh: 2,
		ramBankTop:  0,
		screenBank:  5, // Normal screen in bank 5
		is128K:      false,
		isPlus3:     false,
		specialMode: 0, // Normal mode
	}
	return m
}

func (m *SpectrumMemory) Enable128K() {
	m.is128K = true
}

func (m *SpectrumMemory) EnablePlus3() {
	m.is128K = true
	m.isPlus3 = true
}

// EnableNext turns on the Next-mode 8-slot/8K-granularity MMU (T-28, wave
// ti0.r2, item #19). This is a NEW, additive addressing path -- it does
// NOT set is128K/isPlus3 and does not disturb ramBankLow/High/Top or any
// existing 48K/128K/+3/TS2068 behaviour; Read/Write dispatch to the new
// path only when isNext is true, checked before every other branch.
//
// nextSlots starts zeroed (all 8 slots pointing at logical page 0, the
// low 8K half of RAM bank 0) rather than mirroring nextreg.go's
// mmuBootDefaults -- wiring NextREG 0x50-0x57 to actually drive nextSlots
// is T-28 item #26, a separate, later change; until then this is a
// deliberately inert default a caller/test can override directly.
func (m *SpectrumMemory) EnableNext() {
	m.isNext = true
}

// SetNextSlot sets logical 8K slot `slot` (0-7, address>>13) to page
// `page` -- the memory-side half of T-28 item #26 (wiring NextREG
// 0x50-0x57 to actual paging). nextreg.go's nextRegWriteDirect and
// SetNextRegister call this whenever a write lands on 0x50-0x57, so a
// NextREG write to those registers now genuinely changes what address
// space that slot serves, not merely the mmuSlots mirror nextreg.go
// keeps for readback. Silently ignores an out-of-range slot rather
// than panicking -- callers pass reg-0x50 (always 0-7 by construction
// of the 0x50-0x57 range check), but this stays defensive rather than
// assuming that invariant forever.
func (m *SpectrumMemory) SetNextSlot(slot int, page uint8) {
	if slot < 0 || slot > 7 {
		return
	}
	m.nextSlots[slot] = page
}

// SetNextLegacyBank7FFD is the Next-mode counterpart to SetPaging for
// port 0x7FFD (T-28 item #27): on a genuine Next machine the SAME
// physical port that drives classic 128K/+3 paging also recomposes the
// Next MMU's own slots 6/7 (the 0xC000-0xFFFF 16K-bank-select role) --
// confirmed against jnext's own developer documentation ("the legacy
// ports maintain no map of their own: they recompose the same eight
// slots", src/doc/developer-guide/03-subsystems/02-memory.md) and its
// compose_bank_/apply_legacy_ram_slots_ implementation
// (src/memory/mmu.cpp). This is a wholly separate, additive path from
// SetPaging: it does not call SetPaging, does not read or write
// port7FFD/ramBankTop/romBank/pagingLocked/screenBank or any other
// classic field, and SetPaging itself is completely unmodified by T-28
// -- both are called side by side from io.go's port-0x7FFD dispatch,
// each blind to the other. A no-op when isNext is false, so this has
// zero effect on any classic 48K/128K/+3/TS2068 machine.
//
// Does NOT model the classic paging-lock bit (value bit 5) at all --
// that is a distinct mechanism on real Next hardware (jnext's own
// port_7ffd_locked_/effective_paging_locked gating) that would need its
// own dedicated design; recognised here as an explicit open limitation
// rather than silently ignored or half-implemented.
func (m *SpectrumMemory) SetNextLegacyBank7FFD(value uint8) {
	if !m.isNext {
		return
	}
	m.nextLegacyBank7FFD = value
	m.recomposeNextLegacySlots()
}

// SetNextLegacyPortDFFD is port 0xDFFD's write handler (T-28 item #27):
// the ZX Next's extended-bank-bits port, supplying the bits of the
// composed 16K bank number that port 0x7FFD's 3 bits alone (banks 0-7)
// cannot reach -- confirmed against jnext's Mmu::write_port_dffd /
// compose_bank_ (src/memory/mmu.cpp) and FPGA-REPO-ANALYSIS.md
// ("Extended bank bits (bits 3:0 high bank)"). 0xDFFD has no defined
// role on any non-Next machine this emulator models (48K/128K/+3/
// TS2068) -- it is a Next/Profi-heritage extension, not a standard
// Sinclair port -- so this is a no-op when isNext is false, matching
// that real hardware scope rather than a shortcut.
func (m *SpectrumMemory) SetNextLegacyPortDFFD(value uint8) {
	if !m.isNext {
		return
	}
	m.nextLegacyPortDFFD = value
	m.recomposeNextLegacySlots()
}

// recomposeNextLegacySlots implements jnext's compose_bank_ +
// apply_legacy_ram_slots_ non-Pentagon formula (zenzx models no
// Pentagon timing/machine type, so the Pentagon branch does not apply)
// purely on the Next-mode side: combines the low 3 bits of the last
// 0x7FFD write with 0xDFFD's extension bits into a 7-bit classic-style
// 16K bank number (0-127), then writes that bank's two constituent 8K
// pages into Next MMU slots 6/7 via the existing SetNextSlot. Bit
// mapping (confirmed against jnext's compose_bank_ literally, not
// re-derived from a written description): bank(2:0)=7FFD(2:0),
// bank(4:3)=DFFD(1:0), bank(5)=DFFD(2), bank(6)=DFFD(3).
//
// A composed bank at or beyond zenzx's honest 224-page/1792K ordinary-
// RAM ceiling (bank 112 and above -- see the RAM-capacity correction in
// docs/TRACKING.md T-28) needs no separate bounds check here: its pages
// land at or past nextReservedPageBase and nextPageRead/nextPageWrite's
// existing nextPageIsReserved gate already gives the correct real-
// hardware behaviour (floating-bus read, dropped write) for slots 6/7
// automatically. Slots 0-5 and every classic field (ram, ramBankLow/
// High/Top, romBank, screenBank, port7FFD, port1FFD, pagingLocked) are
// completely untouched -- this function's only side effects are the
// two SetNextSlot calls.
func (m *SpectrumMemory) recomposeNextLegacySlots() {
	bank := m.nextLegacyBank7FFD & 0x07
	bank |= (m.nextLegacyPortDFFD & 0x03) << 3 // DFFD(1:0) -> bank(4:3)
	bank |= (m.nextLegacyPortDFFD & 0x04) << 3 // DFFD(2)   -> bank(5)
	bank |= (m.nextLegacyPortDFFD & 0x08) << 3 // DFFD(3)   -> bank(6)
	page := bank * 2
	m.SetNextSlot(6, page)
	m.SetNextSlot(7, page+1)
}

// nextPageIsReserved reports whether page is in the real-hardware
// reserved sentinel range (T-28, RAM-capacity correction 2026-09-15).
// Confirmed directly against jnext's Mmu::rebuild_ptr (src/memory/mmu.cpp,
// `if (page >= 0xE0)`): NR 0x50-0x57 values 0xE0-0xFF are gated BEFORE
// any RAM lookup on real hardware -- never ordinary addressable RAM
// regardless of how much physical RAM is fitted. This includes 0xFF,
// the "unmapped/legacy-ROM-defer" sentinel docs/TRACKING.md T-28
// describes. The check is explicit and independent of nextTotalPages so
// the reserved range stays correct even if nextTotalPages changes again
// -- it must never depend on array-bounds happening to exclude it by
// accident.
func nextPageIsReserved(page uint8) bool {
	return page >= nextReservedPageBase
}

// nextPageRead returns the byte at offset (0-0x1FFF) within logical 8K
// page `page`. Pages below nextExtraPageBase alias the existing `ram`
// array: page 2*b is the low 8K half of bank b, page 2*b+1 the high half
// -- so a byte written via the classic ramBankLow/High/Top path and one
// read via the Next-mode slot path referring to the same bank see the
// same storage, not a copy. A reserved page (nextPageIsReserved, i.e.
// nextReservedPageBase/0xE0 and above -- this deliberately includes the
// 0xFF "unmapped/legacy-ROM-defer" sentinel) reads back 0xFF, matching
// the floating-bus behaviour an inactive Next MMU slot has on real
// hardware. Redirecting slots 0/1's reserved-range values specifically
// into the existing legacy ROM-paging logic is T-28 item #27, not this
// change -- treating it as a plain reserved page for now is safe (no
// crash, well-defined 0xFF read / dropped write) and does not preclude
// that later, real behavioural upgrade.
func (m *SpectrumMemory) nextPageRead(page uint8, offset uint16) uint8 {
	if page < nextExtraPageBase {
		bank := page / 2
		half := page % 2
		return m.ram[bank][uint16(half)*0x2000+offset]
	}
	if nextPageIsReserved(page) {
		return 0xFF
	}
	idx := int(page) - nextExtraPageBase
	if idx >= len(m.nextExtraRAM) {
		return 0xFF
	}
	return m.nextExtraRAM[idx][offset]
}

// nextPageWrite is nextPageRead's write counterpart -- same aliasing onto
// `ram` for pages below nextExtraPageBase, same drop-on-reserved-page
// behaviour (matching an inactive Next MMU slot's write being silently
// discarded on real hardware, nextPageIsReserved / 0xE0-0xFF, 0xFF
// included) as nextPageRead.
func (m *SpectrumMemory) nextPageWrite(page uint8, offset uint16, value uint8) {
	if page < nextExtraPageBase {
		bank := page / 2
		half := page % 2
		m.ram[bank][uint16(half)*0x2000+offset] = value
		return
	}
	if nextPageIsReserved(page) {
		return
	}
	idx := int(page) - nextExtraPageBase
	if idx >= len(m.nextExtraRAM) {
		return
	}
	m.nextExtraRAM[idx][offset] = value
}

// nextSlotDefersToLegacyROM reports whether slot/page combination must
// defer to the existing classic ROM-paging machinery instead of being
// treated as a plain reserved (inactive/floating-bus) Next MMU page --
// T-28 item #27, the special case the jnext-comparison addendum in
// docs/TRACKING.md T-28 describes. Confirmed against jnext's
// Mmu::rebuild_ptr (src/memory/mmu.cpp): for slots 0/1 specifically
// (cpu_a(15:14) = "00" on real hardware), a reserved-range NR 0x50/0x51
// value does NOT make the slot inactive the way it does for slots 2-7
// -- it falls through to the machine's current ROM selection instead.
// Slots 2-7 are deliberately excluded here; their reserved-range
// behaviour (floating-bus read, dropped write) is already correct via
// nextPageIsReserved in nextPageRead/nextPageWrite and unchanged by
// this function.
func nextSlotDefersToLegacyROM(slot uint16, page uint8) bool {
	return slot < 2 && nextPageIsReserved(page)
}

// readNext is the Next-mode Read dispatch: slot = address>>13 (0-7, each
// 8K), page = whatever nextSlots currently maps that slot to.
//
// T-28 item #27: slots 0/1 holding a reserved-range page (0xE0-0xFF,
// nextPageIsReserved) defer to the existing classic ROM-paging
// machinery -- m.rom[m.romBank], already driven by the classic
// SetPaging/SetPlus3Paging port handlers -- rather than the plain
// inactive/floating-bus treatment nextPageRead gives every other
// reserved-page case. Slot 0 covers ROM offset 0x0000-0x1FFF (the low
// half of the 16K ROM bank), slot 1 covers 0x2000-0x3FFF (the high
// half) -- the same low/high-half split nextPageRead already uses for
// classic-bank-aliased RAM pages, just applied to `rom` instead of
// `ram`. This reuses zenzx's EXISTING ROM selection and storage
// entirely -- no new ROM state, no jnext-style +0x20 SRAM shift (zenzx
// never stores ROM images inside the Next RAM backing store the way
// jnext's rom_in_sram_ does), matching the "additive, never a
// duplicate" principle the rest of T-28 has held to. Slots 2-7 with a
// reserved page are unaffected -- still plain floating-bus/dropped-write
// via nextPageRead/nextPageWrite, exactly as before this change.
func (m *SpectrumMemory) readNext(address uint16) uint8 {
	// Layer 2 Access Port (T-29 completion pass, layer2accessport.go):
	// checked first, before the ordinary MMU slot path -- port 0x123B's
	// mapping, when active, overrides the CPU's lower 16K window
	// entirely, the same "checked first, unconditionally, so nothing
	// below can shadow it" precedence this file's own doc comments use
	// for other exact-match special cases. ok=false (port not currently
	// mapping this address) falls through unchanged.
	if m.layer2AccessReadFn != nil {
		if v, ok := m.layer2AccessReadFn(address); ok {
			return v
		}
	}
	slot := address >> 13
	page := m.nextSlots[slot]
	offset := address & 0x1FFF
	if nextSlotDefersToLegacyROM(slot, page) {
		return m.rom[m.romBank][uint16(slot)*0x2000+offset]
	}
	return m.nextPageRead(page, offset)
}

// writeNext is readNext's write counterpart. The legacy-ROM-deferred
// case (T-28 item #27, see readNext) is read-only, matching every other
// ROM-area write in this file ("ROM area - read only") -- the write is
// simply dropped.
func (m *SpectrumMemory) writeNext(address uint16, value uint8) {
	// Layer 2 Access Port -- see readNext's own comment above; same
	// checked-first precedence, same fall-through-on-ok=false contract.
	if m.layer2AccessWriteFn != nil {
		if m.layer2AccessWriteFn(address, value) {
			return
		}
	}
	slot := address >> 13
	page := m.nextSlots[slot]
	offset := address & 0x1FFF
	if nextSlotDefersToLegacyROM(slot, page) {
		return
	}
	m.nextPageWrite(page, offset, value)
}

// is48KROMActive reports whether the currently-paged ROM bank is
// genuinely the 48K-BASIC-compatible one containing LD-BYTES/SA-BYTES
// at the addresses the fast loader (tape.go) traps -- verified directly
// against the actual shipped ROM bytes, not assumed from documentation:
// 48K's only bank; 128K/+2's bank 1; +3/+2A's bank 3 (English and
// Spanish alike -- confirmed the same bank-index convention holds
// across localisations). TS2068 has its own, completely different tape
// ROM routines (in the Extension ROM, reached only via chunk-0 banking)
// and is explicitly excluded here rather than accidentally matching.
func (m *SpectrumMemory) is48KROMActive() bool {
	if m.isTS2068 {
		return false
	}
	if !m.is128K {
		return true
	}
	if m.isPlus3 {
		return m.romBank == 3
	}
	return m.romBank == 1
}

func (m *SpectrumMemory) Read(address uint16) uint8 {
	// Next MMU (T-28, item #19): checked first, before every existing
	// branch below. isNext defaults false, so this is a no-op for every
	// existing 48K/128K/+3/TS2068 model -- zero behaviour change unless
	// EnableNext() has been called.
	if m.isNext {
		return m.readNext(address)
	}
	if !m.is128K {
		// 48K mode - simple memory map (no banking)
		// The 48K Spectrum has continuous RAM from 0x4000-0xFFFF
		// But this emulator stores it in banks 5, 2, 0 for compatibility
		if address < ROMSize {
			if m.isTS2068 && address < 0x2000 && m.ts2068HSRChunk0 && m.ts2068ExRomSelect {
				// Chunk 0 (0000H-1FFFH) switched out of the Home Bank to
				// the Extension ROM -- Technical Manual 2.1.8.1/5.3.1,
				// confirmed against the disassembled Extension ROM
				// Interface Routine (IFRTN). Chunk 1 (2000H-3FFFH) is
				// never affected -- the Extension ROM is only 8K and the
				// HSR has no bit for chunk 1.
				return m.ts2068ExtROM[address]
			}
			return m.rom[0][address]
		}

		// Map addresses to the correct RAM banks
		// 0x4000-0x7FFF -> bank 5 (screen area)
		// 0x8000-0xBFFF -> bank 2
		// 0xC000-0xFFFF -> bank 0
		if address < 0x8000 {
			// 0x4000-0x7FFF: RAM bank 5 (contains screen memory)
			offset := address - 0x4000
			val := m.ram[5][offset]

			// Handle screen memory reading
			if address >= 0x4000 && address < 0x5800 {
				return m.screen.bitmap[address-0x4000]
			} else if address >= 0x5800 && address < 0x5B00 {
				return m.screen.attributes[address-0x5800]
			}
			return val
		} else if address < 0xC000 {
			// 0x8000-0xBFFF: RAM bank 2
			offset := address - 0x8000
			return m.ram[2][offset]
		} else {
			// 0xC000-0xFFFF: RAM bank 0
			offset := address - 0xC000
			return m.ram[0][offset]
		}
	}

	// 128K/+3 mode with banking
	// Check for special paging modes (+3 only)
	if m.isPlus3 && m.specialPaging {
		return m.readSpecialMode(address)
	}

	// Normal paging mode
	switch {
	case address < 0x4000:
		// ROM area (0x0000-0x3FFF)
		return m.rom[m.romBank][address]

	case address < 0x8000:
		// RAM bank at 0x4000-0x7FFF
		offset := address - 0x4000
		val := m.ram[m.ramBankLow][offset]

		// Handle screen memory reading
		if m.ramBankLow == 5 && m.screenBank == 5 {
			if address >= 0x4000 && address < 0x5800 {
				return m.screen.bitmap[address-0x4000]
			} else if address >= 0x5800 && address < 0x5B00 {
				return m.screen.attributes[address-0x5800]
			}
		} else if m.ramBankLow == 7 && m.screenBank == 7 {
			if address >= 0x4000 && address < 0x5800 {
				return m.screen.bitmap[address-0x4000]
			} else if address >= 0x5800 && address < 0x5B00 {
				return m.screen.attributes[address-0x5800]
			}
		}
		return val

	case address < 0xC000:
		// RAM bank at 0x8000-0xBFFF
		offset := address - 0x8000
		return m.ram[m.ramBankHigh][offset]

	default:
		// RAM bank at 0xC000-0xFFFF
		offset := address - 0xC000
		val := m.ram[m.ramBankTop][offset]

		// Handle screen memory if bank 7 is paged in at top
		if m.ramBankTop == 7 && m.screenBank == 7 {
			if address >= 0xC000 && address < 0xD800 {
				return m.screen.bitmap[address-0xC000]
			} else if address >= 0xD800 && address < 0xDB00 {
				return m.screen.attributes[address-0xD800]
			}
		} else if m.ramBankTop == 5 && m.screenBank == 5 {
			if address >= 0xC000 && address < 0xD800 {
				return m.screen.bitmap[address-0xC000]
			} else if address >= 0xD800 && address < 0xDB00 {
				return m.screen.attributes[address-0xD800]
			}
		}
		return val
	}
}

func (m *SpectrumMemory) readSpecialMode(address uint16) uint8 {
	// +3 special paging: four all-RAM configurations, indexed by specialMode.
	config := [4][4]uint8{
		{0, 1, 2, 3}, // Config 0: RAM 0,1,2,3
		{4, 5, 6, 7}, // Config 1: RAM 4,5,6,7
		{4, 5, 6, 3}, // Config 2: RAM 4,5,6,3
		{4, 7, 6, 3}, // Config 3: RAM 4,7,6,3
	}

	bank := address / 0x4000
	offset := address % 0x4000

	ramBank := config[m.specialMode][bank]
	val := m.ram[ramBank][offset]

	// Handle screen memory in special modes
	if ramBank == m.screenBank {
		relAddr := bank*0x4000 + offset
		if relAddr >= 0x0000 && relAddr < 0x1800 {
			return m.screen.bitmap[relAddr]
		} else if relAddr >= 0x1800 && relAddr < 0x1B00 {
			return m.screen.attributes[relAddr-0x1800]
		}
	}
	return val
}

func (m *SpectrumMemory) Write(address uint16, value uint8) {
	// Next MMU (T-28, item #19): same gate as Read -- see its comment.
	if m.isNext {
		m.writeNext(address, value)
		return
	}
	if !m.is128K {
		// 48K mode - simple memory map (no banking)
		if address < ROMSize {
			return // ROM is read-only
		}

		// Map addresses to the correct RAM banks
		// 0x4000-0x7FFF -> bank 5 (screen area)
		// 0x8000-0xBFFF -> bank 2
		// 0xC000-0xFFFF -> bank 0
		if address < 0x8000 {
			// 0x4000-0x7FFF: RAM bank 5 (contains screen memory)
			offset := address - 0x4000
			m.ram[5][offset] = value

			// Update screen if writing to display memory
			if address >= 0x4000 && address < 0x5800 {
				m.screen.bitmap[address-0x4000] = value
			} else if address >= 0x5800 && address < 0x5B00 {
				m.screen.attributes[address-0x5800] = value
			}
		} else if address < 0xC000 {
			// 0x8000-0xBFFF: RAM bank 2
			offset := address - 0x8000
			m.ram[2][offset] = value
		} else {
			// 0xC000-0xFFFF: RAM bank 0
			offset := address - 0xC000
			m.ram[0][offset] = value
		}
		return
	}

	// 128K/+3 mode
	if m.isPlus3 && m.specialPaging {
		m.writeSpecialMode(address, value)
		return
	}

	// Normal mode writes
	switch {
	case address < 0x4000:
		// ROM area - read only
		return

	case address < 0x8000:
		// RAM bank at 0x4000-0x7FFF
		offset := address - 0x4000
		m.ram[m.ramBankLow][offset] = value

		// Update screen if appropriate bank
		if m.ramBankLow == m.screenBank {
			if address >= 0x4000 && address < 0x5800 {
				m.screen.bitmap[address-0x4000] = value
			} else if address >= 0x5800 && address < 0x5B00 {
				m.screen.attributes[address-0x5800] = value
			}
		}

	case address < 0xC000:
		// RAM bank at 0x8000-0xBFFF
		offset := address - 0x8000
		m.ram[m.ramBankHigh][offset] = value

	default:
		// RAM bank at 0xC000-0xFFFF
		offset := address - 0xC000
		m.ram[m.ramBankTop][offset] = value

		// Update screen if appropriate bank
		if m.ramBankTop == m.screenBank {
			if address >= 0xC000 && address < 0xD800 {
				m.screen.bitmap[address-0xC000] = value
			} else if address >= 0xD800 && address < 0xDB00 {
				m.screen.attributes[address-0xD800] = value
			}
		}
	}
}

// Load writes a block of bytes into memory starting at the given address,
// going through the normal Write path so RAM banking and the screen mirror
// are honoured and ROM regions are protected. Bytes that would extend past
// the top of the 64K address space are silently dropped; callers that need to
// detect overflow should check len(data)+int(address) <= 0x10000 first.
//
// This mirrors the Load convention used by zen80's RAM/MappedMemory types
// (address is uint16, matching the Z80's 16-bit address bus).
func (m *SpectrumMemory) Load(address uint16, data []byte) {
	for i, b := range data {
		addr := int(address) + i
		if addr > 0xFFFF {
			break
		}
		m.Write(uint16(addr), b)
	}
}

func (m *SpectrumMemory) writeSpecialMode(address uint16, value uint8) {
	// +3 special paging: four all-RAM configurations; all are writable.
	config := [4][4]uint8{
		{0, 1, 2, 3}, // Config 0: RAM 0,1,2,3
		{4, 5, 6, 7}, // Config 1: RAM 4,5,6,7
		{4, 5, 6, 3}, // Config 2: RAM 4,5,6,3
		{4, 7, 6, 3}, // Config 3: RAM 4,7,6,3
	}

	bank := address / 0x4000
	offset := address % 0x4000
	ramBank := config[m.specialMode][bank]

	m.ram[ramBank][offset] = value

	// Update screen if writing to screen bank
	if ramBank == m.screenBank {
		relAddr := bank*0x4000 + offset
		if relAddr >= 0x0000 && relAddr < 0x1800 {
			m.screen.bitmap[relAddr] = value
		} else if relAddr >= 0x1800 && relAddr < 0x1B00 {
			m.screen.attributes[relAddr-0x1800] = value
		}
	}
}

// SetROMBank overwrites a single ROM bank (0-3) in place, without
// touching the others or re-inferring is128K/isPlus3 -- the memory-level
// primitive behind -rom0 through -rom3 (zenzx.go's OverrideROMBank),
// which replace one bank of an already-loaded model's ROM set (e.g.
// only +3DOS via -rom3 on an otherwise-standard +3) rather than
// requiring every bank to be respecified.
func (m *SpectrumMemory) SetROMBank(bank int, data []byte) error {
	if bank < 0 || bank > 3 {
		return fmt.Errorf("ROM bank must be 0-3, got %d", bank)
	}
	if len(data) != 16384 {
		return fmt.Errorf("ROM bank must be 16384 bytes, got %d", len(data))
	}
	copy(m.rom[bank][:], data)
	return nil
}

func (m *SpectrumMemory) LoadROM(data []byte) {
	if len(data) == 16384 {
		// 48K ROM
		copy(m.rom[0][:], data)
		m.is128K = false
		m.isPlus3 = false
	} else if len(data) == 32768 {
		// 128K ROM (2 banks)
		copy(m.rom[0][:], data[0:16384])
		copy(m.rom[1][:], data[16384:32768])
		m.Enable128K()
		m.isPlus3 = false
	} else if len(data) == 65536 {
		// +3 ROM (4 banks)
		copy(m.rom[0][:], data[0:16384])
		copy(m.rom[1][:], data[16384:32768])
		copy(m.rom[2][:], data[32768:49152])
		copy(m.rom[3][:], data[49152:65536])
		m.EnablePlus3()
	}
}

// SetPaging handles the 128K memory paging via port 0x7FFD
func (m *SpectrumMemory) SetPaging(value uint8) {
	if !m.is128K || m.pagingLocked {
		return
	}

	m.port7FFD = value

	// In special mode, only screen select works
	if m.isPlus3 && m.specialPaging {
		// Bit 3: Screen select still works in special modes
		newScreenBank := uint8(5)
		if value&0x08 != 0 {
			newScreenBank = 7
		}
		m.updateScreenBank(newScreenBank)
		return
	}

	// Normal mode paging
	// Bit 0-2: RAM page at 0xC000
	m.ramBankTop = value & 0x07

	// Bit 3: Screen select (0=normal in bank 5, 1=shadow in bank 7)
	newScreenBank := uint8(5)
	if value&0x08 != 0 {
		newScreenBank = 7
	}
	m.updateScreenBank(newScreenBank)

	// Bit 4: ROM select
	if m.isPlus3 {
		// +3: bit 4 selects low bit of ROM number
		m.romBank = (m.romBank & 0x02) | ((value >> 4) & 0x01)
	} else {
		// 128K: bit 4 selects between ROM 0 and 1
		if value&0x10 != 0 {
			m.romBank = 1
		} else {
			m.romBank = 0
		}
	}

	// Bit 5: Lock paging
	if value&0x20 != 0 {
		m.pagingLocked = true
	}
}

// SetPlus3Paging handles +3 specific paging via port 0x1FFD
func (m *SpectrumMemory) SetPlus3Paging(value uint8) {
	if !m.isPlus3 || m.pagingLocked {
		return
	}

	m.port1FFD = value

	// Bit 0: paging mode (0 = normal, 1 = special all-RAM paging).
	if value&0x01 != 0 {
		// Special paging mode. Bits 1-2 select one of four all-RAM
		// configurations (there is no "all ROM" special config):
		//   0: banks 0,1,2,3   1: banks 4,5,6,7
		//   2: banks 4,5,6,3   3: banks 4,7,6,3
		m.specialPaging = true
		m.specialMode = (value >> 1) & 0x03
		switch m.specialMode {
		case 0:
			m.ramBankLow, m.ramBankHigh, m.ramBankTop = 1, 2, 3
		case 1:
			m.ramBankLow, m.ramBankHigh, m.ramBankTop = 5, 6, 7
		case 2:
			m.ramBankLow, m.ramBankHigh, m.ramBankTop = 5, 6, 3
		case 3:
			m.ramBankLow, m.ramBankHigh, m.ramBankTop = 7, 6, 3
		}
	} else {
		// Normal paging mode.
		m.specialPaging = false
		m.specialMode = 0
		m.ramBankLow = 5
		m.ramBankHigh = 2
		// ramBankTop is controlled by port 0x7FFD.

		// Bit 2 of 0x1FFD is the high bit of ROM selection (combined with
		// bit 4 of 0x7FFD). Only meaningful in normal paging mode.
		m.romBank = ((value >> 1) & 0x02) | (m.romBank & 0x01)
	}

	// Bit 3: Disk motor control (not implemented)
	// Bit 4: Printer strobe (not implemented)
}

func (m *SpectrumMemory) updateScreenBank(newBank uint8) {
	if newBank == m.screenBank {
		return
	}

	// Copy current screen to its RAM bank
	if m.screenBank == 5 {
		copy(m.ram[5][0:6144], m.screen.bitmap[:])
		copy(m.ram[5][6144:6912], m.screen.attributes[:])
	} else {
		copy(m.ram[7][0:6144], m.screen.bitmap[:])
		copy(m.ram[7][6144:6912], m.screen.attributes[:])
	}

	// Load new screen from RAM bank
	if newBank == 5 {
		copy(m.screen.bitmap[:], m.ram[5][0:6144])
		copy(m.screen.attributes[:], m.ram[5][6144:6912])
	} else {
		copy(m.screen.bitmap[:], m.ram[7][0:6144])
		copy(m.screen.attributes[:], m.ram[7][6144:6912])
	}

	m.screenBank = newBank
}
