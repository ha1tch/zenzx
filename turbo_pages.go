package main

// ============================================================================
// Turbo mode: page-based physical memory model
//
// zen80's FastMem is a flat 64K array -- correct for a machine with no
// banking, but incapable of representing one that pages physical memory in
// and out of the logical address space, since a flat array has no notion
// of "the byte that used to be here belongs to a different physical bank
// now". Confirmed necessary, not speculative: Cybernoid 2 (128K) actively
// switches RAM and ROM banks via port 0x7FFD *during ongoing code
// execution*, not just between tape blocks -- the per-block sync a flat
// snapshot could support was never going to be enough for this class of
// game.
//
// This models physical memory the way the real hardware actually has it:
// a fixed pool of MemoryPageCount independently-addressable 16K pages,
// each tagged with what it is, and a separate, small "current mapping"
// saying which physical page is presently visible at each of the four 16K
// logical slots (0x0000, 0x4000, 0x8000, 0xC000). zen80's FastMem still
// gets the flat, fast array it needs -- this layer's job is keeping that
// array's content correct as the mapping changes underneath it, swapping
// pages in and out the moment a paging port write actually happens
// (FastPortWriteOut, zen80 0.4.0), not just at tape-block boundaries.
//
// MemoryPageCount (12) is not an arbitrary round number: it is exactly
// SpectrumMemory's own real layout (memory.go) -- 4 ROM banks (+3's own
// count, a superset of 128K/+2's 2) plus 8 RAM banks. Pages 0-3 are the
// ROM banks; pages 4-11 are RAM banks 0-7 (RAM bank N lives at page 4+N).
// Scope for now: standard 128K/+2/+3 paging via port 0x7FFD, which is
// what every case investigated this session actually needed. +3's
// "special" all-RAM paging mode (port 0x1FFD bit 0) and TS2068's entirely
// different Extension-ROM paging scheme are deliberately not handled yet
// -- both are rare for tape-based titles specifically, and are a
// reasonable place to extend this later rather than a reason to block on
// now.
// ============================================================================

const MemoryPageCount = 12

// MemoryFlags tags what a physical page actually is. The low 4 bits are a
// page-type enum (room for up to 16 without a layout change); bits 4-6 are
// independent boolean properties.
type MemoryFlags uint64

const (
	PageTypeSpectrum128 MemoryFlags = iota // standard 128K/+2 ROM or RAM bank
	PageTypePlus3                          // +3-specific ROM bank (editor/syntax/48-BASIC/DOS)
	PageTypeTimex                          // reserved for future TS2068 Extension ROM modelling
	PageTypeNext                           // reserved for future Next compatibility-mode paging
)

const pageTypeMask MemoryFlags = 0xF

const (
	FlagROM       MemoryFlags = 1 << 4 // set: ROM (read-only); clear: RAM
	FlagSwappable MemoryFlags = 1 << 5 // set: can be paged out; clear: always mapped regardless of paging state
	FlagUser      MemoryFlags = 1 << 6 // set: user/game RAM; clear: system-reserved
)

// MemoryPage16k is one physical, independently-addressable 16K page --
// exactly the real hardware's own paging granularity.
type MemoryPage16k [16384]byte

// turboPageFlags and turboPages are the physical page pool, held on ZenZX
// (never package-level globals -- keeping this per-instance is what makes
// it safe to run more than one ZenZX, e.g. under test, without one
// instance's turbo-mode memory bleeding into another's). turboPageFlags
// holds metadata only; turboPages holds the actual 16K of data for each
// of the same MemoryPageCount slots.
//
// turboSlotPage tracks the CURRENT mapping: turboSlotPage[i] is the page
// index (0-11) presently visible at logical slot i (0=0x0000, 1=0x4000,
// 2=0x8000, 3=0xC000). turboPort7FFD is the fast path's own tracked copy
// of the paging register -- distinct from the real SpectrumMemory's
// port7FFD, since that only updates via the real (non-fast-path) I/O
// route, which the fast path deliberately bypasses.

// initTurboPageFlags sets each page's metadata once, matching
// SpectrumMemory's own rom[4]/ram[8] layout exactly: pages 0-3 are ROM
// banks 0-3 (fixed, system, read-only), pages 4-11 are RAM banks 0-7
// (swappable, user, read-write).
func initTurboPageFlags() [MemoryPageCount]MemoryFlags {
	var flags [MemoryPageCount]MemoryFlags
	for i := 0; i < 4; i++ {
		flags[i] = PageTypePlus3 | FlagROM
	}
	for i := 4; i < MemoryPageCount; i++ {
		flags[i] = PageTypeSpectrum128 | FlagSwappable | FlagUser
	}
	return flags
}

// turboSlotForAddr returns which of the four 16K logical slots a given
// 16-bit address falls in (0-3).
func turboSlotForAddr(addr uint16) int {
	return int(addr >> 14)
}

// turboPageIndexFor128K computes the desired physical page index (0-11)
// for a given logical slot, from the current (fast-path-tracked) port
// 0x7FFD value -- mirroring SetPaging's own decoding in memory.go
// exactly, for the standard (non-+3-special) paging case.
func turboPageIndexFor128K(slot int, port7FFD uint8, isPlus3 bool, plus3RomBit bool) int {
	switch slot {
	case 0: // 0x0000-0x3FFF: ROM bank
		if isPlus3 {
			// +3: bit 4 of 0x7FFD is the low bit of a 2-bit ROM number;
			// the high bit comes from port 0x1FFD bit 2, not tracked in
			// this initial, standard-paging-only scope -- default to 0.
			lo := 0
			if port7FFD&0x10 != 0 {
				lo = 1
			}
			hi := 0
			if plus3RomBit {
				hi = 2
			}
			return hi | lo
		}
		if port7FFD&0x10 != 0 {
			return 1
		}
		return 0
	case 1: // 0x4000-0x7FFF: always RAM bank 5 in standard (non-special) paging
		return 4 + 5
	case 2: // 0x8000-0xBFFF: always RAM bank 2 in standard (non-special) paging
		return 4 + 2
	case 3: // 0xC000-0xFFFF: RAM bank selected by bits 0-2 of 0x7FFD
		return 4 + int(port7FFD&0x07)
	}
	return 0
}
