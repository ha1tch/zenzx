// Next-mode (ZX Spectrum Next) colour decoding -- additive to this
// package's existing classic 16-colour palette, per zxclassicpalette.go's
// own anticipation (in the zenui package) that "the ZX Spectrum Next's
// palette works on entirely different principles -- 9-bit RGB, not eight
// named colours plus a bright flag."
//
// This file holds only pure, portable colour math: decoding the wire
// format real NextREG palette-value registers use into 3-bit-per-channel
// RGB, and scaling that down to 8-bit-per-channel for display. It
// deliberately does NOT hold any live, mutable, per-machine palette state
// (256 entries x 8 palettes, selected and written via NextREG) -- that is
// runtime device state specific to one running machine, not a portable
// colour-space fact, so it belongs in the emulator itself (zenzx's own
// nextpalette.go), the same separation this package already keeps between
// its own static classic-palette constants and any given consumer's UI
// state.
//
// Register map this decodes (confirmed against wiki.specnext.dev's
// Palette_Value_Register and ULA_Palette_Control_Register pages, cross-
// checked against specnext.com/tbblue-io-port-system's own register
// summary):
//
//	NextREG $41  Palette Value, 8-bit write: one byte, RRRGGGBB. The
//	             missing third blue bit is synthesised as bit1 OR bit0 of
//	             the 2-bit blue field ("Read does not auto-increment the
//	             index, and reads always the top 8 bits of colour from
//	             palette" -- the wiki's own wording for the read side of
//	             this same 8-bit view).
//	NextREG $44  Palette Value, 9-bit write: two consecutive writes. The
//	             first byte is the same RRRGGGBB layout $41 uses; the
//	             second byte's bit 0 is the true third blue bit (replacing
//	             the $41 path's OR-synthesised approximation) and bit 7 is
//	             a Layer-2-only priority flag (not decoded here -- colour
//	             only; a caller wanting the priority bit reads it directly
//	             off the second byte itself).
package zxpalette

import "image/color"

// RGB9 is one Next palette entry's colour, 3 bits per channel (0-7 each)
// -- the real hardware's native storage width, kept separate from the
// scaled-to-8-bit color.NRGBA a renderer actually wants, so a caller that
// only needs to compare or store entries never pays for the scaling.
type RGB9 struct {
	R, G, B uint8 // each 0-7
}

// DecodeRGB9From8Bit decodes a single-byte ($41-style) RRRGGGBB write
// into 3-bit channels, synthesising the missing third blue bit as bit1
// OR bit0 of the 2-bit blue field, per the wiki's own documented
// behaviour for this narrower write path.
func DecodeRGB9From8Bit(b uint8) RGB9 {
	r := (b >> 5) & 0x07
	g := (b >> 2) & 0x07
	b2 := b & 0x03
	orBit := (b2 >> 1) | (b2 & 0x01) // bit1 OR bit0, the synthesised third blue bit
	return RGB9{R: r, G: g, B: (b2 << 1) | orBit}
}

// DecodeRGB9From9Bit decodes the two-byte ($44-style) write into 3-bit
// channels: byte1 is the same RRRGGGBB layout as the 8-bit path, byte2's
// bit 0 supplies the true third blue bit (no synthesis needed).
func DecodeRGB9From9Bit(byte1, byte2 uint8) RGB9 {
	r := (byte1 >> 5) & 0x07
	g := (byte1 >> 2) & 0x07
	b2 := byte1 & 0x03
	thirdBlueBit := byte2 & 0x01
	return RGB9{R: r, G: g, B: (b2 << 1) | thirdBlueBit}
}

// EncodeRGB9To8Bit packs a 3-bit-channel colour back into the $41/$44
// first-byte RRRGGGBB wire format (the third blue bit, if any, is
// dropped -- this is the narrower 8-bit view of a 9-bit entry, matching
// the wiki's own statement that reading $41 "reads always the top 8 bits
// of colour from palette").
func EncodeRGB9To8Bit(c RGB9) uint8 {
	return (c.R&0x07)<<5 | (c.G&0x07)<<2 | (c.B>>1)&0x03
}

// Colour scales a 3-bit-per-channel Next colour to 8-bit-per-channel for
// display, using plain linear scaling (v*255/7, ~36.4 per step) rather
// than a hardware DAC lookup table -- no citation for a non-linear Next
// DAC curve was found against the sources this package's own header
// points at (wiki.specnext.dev), so this is a documented, deliberately
// simple choice, not a claim of measured hardware accuracy.
func (c RGB9) Colour() color.NRGBA {
	scale := func(v uint8) uint8 { return uint8(uint16(v) * 255 / 7) }
	return color.NRGBA{R: scale(c.R), G: scale(c.G), B: scale(c.B), A: 0xff}
}
