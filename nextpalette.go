package main

import "github.com/ha1tch/zenzx/pkg/zxpalette"

// ============================================================================
// ZX Spectrum Next: the real 9-bit/512-colour palette (T-31, wave ti0.r5)
//
// Scope, per docs/TRACKING.md T-31 and NEXT_SUPPORT_DEVELOPMENT_PLAN.md's
// ti0.r5 "done when": a next-mode test program can write a known palette
// entry and the rendered pixel for that index matches the expected 9-bit
// RGB value, for both Layer 2 and sprites. This file implements all 8
// palettes' storage and the full NextREG read/write protocol (real
// software pokes these registers regardless of which palettes this
// emulator actually renders through), but only Layer 2 and Sprites are
// wired into resolveNextPixel/the GPU renderers -- the ULA and Tilemap
// palettes are stored and readable back correctly, matching this
// project's established store-and-ignore convention for scope this wave's
// own "done when" bar does not require (ULA's own enhanced "ULANext" mode
// and the Tilemap hardware layer are both separate, larger features, not
// part of this wave or even this plan's Tier 0 -- see
// NEXT_SUPPORT_DEVELOPMENT_PLAN.md's "Deferred within Tier 0"/Tier 1-2
// lists).
//
// Register map (confirmed against wiki.specnext.dev's Palette_Index_Register,
// Palette_Value_Register, and ULA_Palette_Control_Register pages, and
// cross-checked against specnext.com/tbblue-io-port-system's own register
// summary):
//
//	NR 0x40  Palette Index: selects index 0-255 within whichever palette
//	         NR 0x43 bits 6-4 currently target. Writing this register
//	         also resets NR 0x44's own two-write sequence, so the next
//	         write there is treated as the first byte of a new colour.
//	NR 0x41  Palette Value, 8-bit: one write, RRRGGGBB, the missing third
//	         blue bit synthesised as (bit1 OR bit0) of the 2-bit blue
//	         field. Auto-increments NR 0x40's index afterward unless
//	         disabled (NR 0x43 bit 7). A read returns the CURRENT entry's
//	         colour re-encoded to this 8-bit view (not the last byte
//	         written) and does not advance the index -- "reads always the
//	         top 8 bits of colour from palette", per the wiki.
//	NR 0x43  Palette Control: bit 7 disables index auto-increment; bits
//	         6-4 select which of the 8 palettes 0x40/0x41/0x44 read and
//	         write (000=ULA1st, 001=Layer2 1st, 010=Sprites 1st,
//	         011=Tilemap 1st, 100=ULA2nd, 101=Layer2 2nd, 110=Sprites
//	         2nd, 111=Tilemap 2nd); bits 3/2/1 independently select which
//	         of first/second is the currently DISPLAYED palette for
//	         Sprites/Layer2/ULA respectively (orthogonal to bits 6-4,
//	         which only pick the read/write target) -- this emulator only
//	         consumes the Layer2/Sprites display bits, per this file's
//	         own scope note above; bit 0 enables ULANext mode, stored but
//	         not consumed (a separate ULA feature, out of scope). Writing
//	         this register also resets NR 0x44's two-write sequence --
//	         not explicitly documented by the sources consulted, but a
//	         reasonable, defensive assumption (switching the write target
//	         mid-sequence leaving a stale pending first byte from a
//	         different palette would be a plausible latent-bug source);
//	         flagged here as inferred, not confirmed, matching this
//	         file's own established honesty convention for the same
//	         situation elsewhere in this codebase.
//	NR 0x44  Palette Value, 9-bit: two consecutive writes. The first byte
//	         is the same RRRGGGBB layout as 0x41; the second byte's bit 0
//	         is the true third blue bit (replacing 0x41's OR-synthesised
//	         approximation) and bit 7 is a Layer-2-only priority flag,
//	         stored but not consumed (per-pixel Layer 2 priority
//	         compositing against sprites is Tier 1+, not part of this
//	         wave). After the second write, the index auto-increments
//	         (unless disabled) and the sequence resets to expect a first
//	         byte again. This emulator does not implement a distinct read
//	         path for 0x44 -- no source consulted documents one, so reads
//	         fall through to this file's general stored-value convention
//	         (whatever was last written), not a claim of confirmed
//	         hardware read behaviour.
//
// Power-on/construction default: every entry starts at RGB9{0,0,0}
// (black). Real hardware almost certainly boots with a palette
// approximating the classic 16 ZX Spectrum colours (so unmodified 48K/
// 128K software displays correctly on first boot), but no source
// consulted for this wave documents the actual default table, and
// inventing one would be a colour-accuracy claim this session has no
// basis for -- an honest gap, not a silent omission; see this file's own
// entry in docs/TRACKING.md T-31 for why it stays open rather than
// closing.
// ============================================================================

// nextPaletteTarget identifies one of the 8 real hardware palettes, in
// the same 3-bit encoding NR 0x43 bits 6-4 use -- see this file's header
// for the full table.
type nextPaletteTarget uint8

const (
	nextPaletteULA1st nextPaletteTarget = iota
	nextPaletteLayer2_1st
	nextPaletteSprites1st
	nextPaletteTilemap1st
	nextPaletteULA2nd
	nextPaletteLayer2_2nd
	nextPaletteSprites2nd
	nextPaletteTilemap2nd
)

// nextPaletteSystem is the palette-adjacent state living on SpectrumIO,
// mirroring how nextRegs/nextRegClipWin/sprites are owned there. It holds
// all 8 palettes' full 256-entry storage plus the NR 0x40/0x43/0x44
// write-cursor state; it does NOT own nextPaletteVersion (io.go) --
// nextreg.go's write handlers call into this type for the mechanics and
// bump that counter themselves, the same separation sprite.go's
// writeAttribute already has from nextPaletteVersion.
type nextPaletteSystem struct {
	palettes [8][256]zxpalette.RGB9

	writeIndex  uint8             // NR 0x40
	writeTarget nextPaletteTarget // NR 0x43 bits 6-4

	autoIncrementDisabled bool // NR 0x43 bit 7
	displaySecondULA      bool // NR 0x43 bit 1
	displaySecondLayer2   bool // NR 0x43 bit 2
	displaySecondSprites  bool // NR 0x43 bit 3
	ulaNextEnabled        bool // NR 0x43 bit 0, stored not consumed

	pendingFirstByte   uint8
	awaitingSecondByte bool // NR 0x44's own two-write sequence state
}

func newNextPaletteSystem() *nextPaletteSystem {
	return &nextPaletteSystem{}
}

// resetSequence clears NR 0x44's pending-first-byte state -- called by
// both the NR 0x40 and NR 0x43 write handlers, per this file's header
// comment on why 0x43 resets it too.
func (p *nextPaletteSystem) resetSequence() {
	p.awaitingSecondByte = false
	p.pendingFirstByte = 0
}

// writeIndexReg implements a write to NR 0x40.
func (p *nextPaletteSystem) writeIndexReg(value uint8) {
	p.writeIndex = value
	p.resetSequence()
}

// writeControlReg implements a write to NR 0x43.
func (p *nextPaletteSystem) writeControlReg(value uint8) {
	p.autoIncrementDisabled = value&0x80 != 0
	p.writeTarget = nextPaletteTarget((value >> 4) & 0x07)
	p.displaySecondSprites = value&0x08 != 0
	p.displaySecondLayer2 = value&0x04 != 0
	p.displaySecondULA = value&0x02 != 0
	p.ulaNextEnabled = value&0x01 != 0
	p.resetSequence()
}

// advanceIndex implements the auto-increment NR 0x41/0x44 both trigger
// after a complete colour write, honouring NR 0x43 bit 7. Wraps 255->0
// via plain uint8 overflow -- no source consulted states the wrap
// behaviour explicitly, so this is the natural hardware-register
// assumption, not a confirmed citation.
func (p *nextPaletteSystem) advanceIndex() {
	if p.autoIncrementDisabled {
		return
	}
	p.writeIndex++
}

// writeValue8 implements a write to NR 0x41: one byte, RRRGGGBB,
// completing a colour write on its own (no two-write sequence), and
// resetting any NR 0x44 sequence already in progress.
func (p *nextPaletteSystem) writeValue8(value uint8) {
	p.palettes[p.writeTarget][p.writeIndex] = zxpalette.DecodeRGB9From8Bit(value)
	p.resetSequence()
	p.advanceIndex()
}

// readValue8 implements a read from NR 0x41: the CURRENT entry's colour
// re-encoded to the 8-bit view, not the last byte written -- "reads
// always the top 8 bits of colour from palette", per the wiki. Does not
// advance the index.
func (p *nextPaletteSystem) readValue8() uint8 {
	return zxpalette.EncodeRGB9To8Bit(p.palettes[p.writeTarget][p.writeIndex])
}

// writeValue9 implements a write to NR 0x44: the first of a pair is
// buffered; the second completes the colour, advances the index (if
// enabled), and resets the sequence for the next pair. priority (bit 7
// of the second byte) is accepted but not stored -- see file header.
func (p *nextPaletteSystem) writeValue9(value uint8) {
	if !p.awaitingSecondByte {
		p.pendingFirstByte = value
		p.awaitingSecondByte = true
		return
	}
	p.palettes[p.writeTarget][p.writeIndex] = zxpalette.DecodeRGB9From9Bit(p.pendingFirstByte, value)
	p.awaitingSecondByte = false
	p.pendingFirstByte = 0
	p.advanceIndex()
}

// displayTarget returns which of the 8 palettes is currently shown for
// layer -- the display-select bits (NR 0x43 bits 3-1), independent of
// writeTarget (bits 6-4), which only pick 0x40/0x41/0x44's own read/
// write target. Only Layer 2 and Sprites are meaningful arguments here;
// see this file's header for why ULA/Tilemap are stored but not
// rendered through yet.
func (p *nextPaletteSystem) displayTarget(layer nextLayer) nextPaletteTarget {
	switch layer {
	case nextLayerLayer2:
		if p.displaySecondLayer2 {
			return nextPaletteLayer2_2nd
		}
		return nextPaletteLayer2_1st
	case nextLayerSprites:
		if p.displaySecondSprites {
			return nextPaletteSprites2nd
		}
		return nextPaletteSprites1st
	default:
		// ULA has no consumer yet (see file header); returning its 1st
		// palette rather than panicking keeps this total, matching this
		// codebase's general preference for a documented fallback over
		// a reachable panic.
		if p.displaySecondULA {
			return nextPaletteULA2nd
		}
		return nextPaletteULA1st
	}
}

// lookup resolves a final 8-bit palette index (see resolveNextPixelIndex,
// layer2.go) to a colour, through whichever of layer's two palettes is
// currently displayed.
func (p *nextPaletteSystem) lookup(layer nextLayer, index uint8) zxpalette.RGB9 {
	return p.palettes[p.displayTarget(layer)][index]
}
