package main

import (
	"image"
	"image/color"
)

// ============================================================================
// ZX Spectrum Next: Layer 2 graphics (T-29, wave ti0.r3)
//
// Scope, per docs/TRACKING.md T-29 and NEXT_SUPPORT_DEVELOPMENT_PLAN.md's
// ti0.r3 "done when": a 256x192, 8-bit-colour linear framebuffer, base bank
// configurable via NextREG, composited against the ULA layer. The wiki's two
// wider/narrower-colour modes (320x256x8bpp, 640x256x4bpp -- NR 0x70 bits
// 5-4 = %01/%10) are explicitly deferred until this base mode renders
// correctly; DecodeLayer2 below only ever produces the %00 layout. Of NR
// 0x15's six possible three-layer priority orderings, only the two
// NEXT_SUPPORT_DEVELOPMENT_PLAN.md calls out are implemented here: the
// default "S L U" ordering (Layer 2 sits between sprites and ULA -- since no
// sprite layer exists yet in this emulator, that collapses to plain
// Layer2-over-ULA) and the explicit "L U S"/"L S U" family, which is the
// same Layer2-over-ULA result from this compositor's point of view once
// sprites are absent. Every ordering that puts ULA on top of Layer 2 (U S L,
// U L S) is also handled, since distinguishing "not yet implemented" from
// "correctly renders ULA on top" would be strictly worse than just decoding
// the real bit pattern. The two core-3.1.1 blending modes (%110/%111) are
// explicitly out of scope -- see layer2Priority's own doc comment.
//
// Register map used here (confirmed against wiki.specnext.dev's Layer_2,
// Layer_2_Control_Register, and Sprite_and_Layers_System_Register pages,
// not the older, imprecise "0x12/0x13/0x14 clip window" gloss in
// docs/TRACKING.md's own T-29 prose -- that gloss conflates the base-bank
// registers with the clip window, which is genuinely 0x18 and already
// implemented by nextreg.go/T-27):
//
//	NR 0x12  Layer 2 visible screen base bank (this file: layer2BaseBank)
//	NR 0x13  Layer 2 shadow screen base bank (not consumed here -- no
//	         mechanism yet selects shadow over visible; see nextreg.go's
//	         own comment on why port 0x123B's shadow bit is out of scope)
//	NR 0x18  Layer 2 clip window: X1,X2,Y1,Y2 (nextreg.go, T-27)
//	NR 0x70  Layer 2 control: bits 5-4 resolution, bits 3-0 palette offset
//	NR 0x15  sprite/layer priority, bits 4-2 (this file: layer2Priority)
//
// Framebuffer addressing: the visible base bank is an 8K-page-pair index
// (the same NextREG page-number convention nextreg.go's mmuBootDefaults and
// memory.go's nextPageRead/nextPageWrite already use for the MMU slots) --
// NOT a CPU address-space slot. Layer 2's 256x192x8bpp framebuffer is
// 49152 bytes = exactly 6 consecutive 8K pages, read directly via
// SpectrumMemory.nextPageRead so it works whether those pages fall in the
// aliased-to-`ram` low range or genuine nextExtraRAM -- Layer 2 memory is
// never mapped through nextSlots/readNext, matching how real Next software
// accesses Layer 2 memory via its own paging windows (port 0x123B, out of
// scope here) rather than the general MMU slots T-28 wired up.
// ============================================================================

const (
	// layer2Width/Height name the fixed 256x192 output canvas every layer
	// (ULA, Layer 2, Sprites) composites onto -- this is the real screen
	// resolution, unrelated to which of Layer 2's three OWN framebuffer
	// modes (layer2Mode*, below) is currently selected. Kept under their
	// original names since sprite.go and this file's own CompositeLayer2/
	// CompositeNextLayers already reference them as "the screen size";
	// renaming would touch call sites that have nothing to do with this
	// wave's actual change (Layer 2's framebuffer, not the screen).
	layer2Width  = 256
	layer2Height = 192

	// layer2PageCount is how many consecutive 8K pages a 256x192x8bpp
	// framebuffer spans: 256*192 = 49152 bytes = 49152/8192 = 6 pages.
	// Still correct for mode %00 specifically; layer2FramebufferPages
	// (below) generalizes this per-mode for the two wider modes.
	layer2PageCount = (layer2Width * layer2Height) / 8192
)

// layer2Mode identifies one of the three real NR 0x70 bits-5-4 resolution
// modes. Named distinctly from a bare uint8 so call sites read as intent
// ("is this the wide mode") rather than a magic 0/1/2.
type layer2Mode uint8

const (
	layer2Mode256x192x8bpp layer2Mode = iota // NR 0x70 bits 5-4 = %00
	layer2Mode320x256x8bpp                   // NR 0x70 bits 5-4 = %01
	layer2Mode640x256x4bpp                   // NR 0x70 bits 5-4 = %10
)

// layer2CurrentMode decodes NR 0x70 bits 5-4 into a layer2Mode. %11 is
// documented as reserved/undefined (Layer_2_Control_Register) -- treated
// as the base mode, the same "safe fallback to a real, implemented case"
// choice layer2ULAOnTop already makes for NR 0x15's undefined bit
// patterns, rather than inventing behaviour for a pattern real hardware
// itself leaves unspecified.
func (io *SpectrumIO) layer2CurrentMode() layer2Mode {
	switch (io.GetNextRegister(0x70) >> 4) & 0x03 {
	case 0b01:
		return layer2Mode320x256x8bpp
	case 0b10:
		return layer2Mode640x256x4bpp
	default: // 0b00, and the reserved 0b11
		return layer2Mode256x192x8bpp
	}
}

// layer2FramebufferDimensions returns the actual pixel dimensions of
// Layer 2's OWN framebuffer for a given mode -- NOT the 256x192 output
// canvas (layer2Width/Height above). Confirmed against wiki.specnext.dev/
// Layer_2: %00 is 256x192, %01 is 320x256, %10 is 640x256 (packed 2
// pixels/byte -- see layer2FramebufferPages and layer2Pixel's own mode
// switch for the packing).
func layer2FramebufferDimensions(mode layer2Mode) (width, height int) {
	switch mode {
	case layer2Mode320x256x8bpp:
		return 320, 256
	case layer2Mode640x256x4bpp:
		return 640, 256
	default:
		return 256, 192
	}
}

// layer2PixelWidthScale (2026-09-15, T-29 GUI-wiring pass) returns how
// many SCREEN pixels wide one framebuffer pixel of mode draws at window
// multiplier mult -- NOT simply float32(mult) for every mode. Confirmed
// against an exact quote from wiki.specnext.dev/Layer_2_Control_Register:
// "The 640x256x4bpp mode is more like 320x256x8bpp mode, but every byte
// is displayed as two half-width paired pixels" -- i.e. 640-mode's 640
// native columns occupy the SAME physical picture width as 320-mode's own
// 320 columns; the mode is double horizontal DENSITY, not a physically
// wider picture. This corrects an earlier draft of this GUI-wiring pass
// that assumed every mode's pixels draw at uniform full width and tried
// to solve 640-mode's apparent "overflow" by widening the window instead
// -- wrong, since it contradicted this exact hardware behaviour (caught
// before shipping, not after: the wiki fetch was made specifically
// because the arithmetic in that draft didn't reconcile with Horacio's
// own stated design, prompting a recheck rather than proceeding on an
// unverified assumption). Vertical density is unaffected -- 640x256's
// height (256) is identical to 320x256's, both drawn at full float32(mult)
// per row -- only this function's horizontal return value differs by mode.
func layer2PixelWidthScale(mode layer2Mode, mult int) float32 {
	if mode == layer2Mode640x256x4bpp {
		return float32(mult) / 2
	}
	return float32(mult)
}

// layer2DisplayedSize (2026-09-15, T-29 GUI-wiring pass) returns the
// active Layer 2 mode's actual on-screen footprint in window pixels at
// multiplier mult -- width via layer2PixelWidthScale (so 640-mode's
// half-width pixels are reflected here too, not just in the per-pixel
// draw calls), height via the framebuffer's own real row count at full
// density. Used to scissor renderLayer2GPU's draw calls to the picture's
// own true displayed bounds (see that function's own comment on why the
// existing oversized-splat-per-pixel technique needs a tight scissor to
// avoid bleeding into the border/margin around it), and by
// DisplayManager.nextLayersViewportOffset (display.go) to centre that
// same footprint within the fixed reference canvas.
func layer2DisplayedSize(mode layer2Mode, mult int) (width, height float32) {
	vw, vh := layer2FramebufferDimensions(mode)
	return float32(vw) * layer2PixelWidthScale(mode, mult), float32(vh) * float32(mult)
}

// layer2FramebufferPages returns how many consecutive 8K pages mode's
// framebuffer spans, per the wiki's own stated sizes: 48KiB (6 pages) for
// %00, 80KiB (10 pages) for both %01 and %10 -- %10's 640x256x4bpp packs
// two pixels per byte, so its BYTE count matches %01's despite having
// twice the pixel count.
func layer2FramebufferPages(mode layer2Mode) uint8 {
	switch mode {
	case layer2Mode320x256x8bpp, layer2Mode640x256x4bpp:
		return 10
	default:
		return layer2PageCount // 6
	}
}

// layer2Enabled reports whether Layer 2 should be composited at all this
// frame. There is no dedicated Layer 2 enable bit documented anywhere in
// the sources consulted (Sprite_and_Layers_System_Register's own page notes
// "the documentation does not specify a dedicated bit to disable Layer 2
// visibility independent of the priority modes") -- so, matching that
// documented absence rather than inventing a gate the real hardware
// doesn't have, this is gated purely on Next mode being active at all
// (io.memory.isNext). A Next machine that has never touched NR 0x12/0x70
// still composites an all-zero (palette index 0, black) Layer 2 image --
// harmless, and correctly matches "Layer 2 shows whatever is in its
// framebuffer" hardware behaviour rather than silently hiding a layer real
// hardware has no way to hide.
func (io *SpectrumIO) layer2Enabled() bool {
	return io.memory != nil && io.memory.isNext
}

// layer2BaseBank returns NR 0x12's current value: the first of the six
// consecutive 8K pages backing the visible Layer 2 framebuffer.
func (io *SpectrumIO) layer2BaseBank() uint8 {
	return io.GetNextRegister(0x12)
}

// layer2PaletteOffset returns NR 0x70 bits 3-0: added to the top nibble of
// every Layer 2 pixel value before palette lookup, per resolveNextPixelIndex
// (T-31's real 256-entry Next palette; the offset<<4 + raw arithmetic is
// unchanged from T-29/T-30's own derivation).
//
// For the two wider modes (layer2Mode320x256x8bpp is unaffected -- it is
// still a genuine 8-bit pixel, same as the base mode): layer2Mode640x256x4bpp
// packs a native 4-bit pixel per nibble, and the wiki's own wording
// ("added to top four bits of each pixel") is written assuming an 8-bit
// pixel where the offset lands in bits 7-4 above a separate bits-3-0
// pixel value -- it does not say what "top four bits" means for a pixel
// that IS only 4 bits wide, with no separate low nibble to speak of. No
// source consulted resolves this. This function still returns the same
// bits-3-0 value regardless of mode; layer2Pixel's 4bpp branch is where
// the inferred (not confirmed) choice actually gets made -- see that
// function's own comment.
func (io *SpectrumIO) layer2PaletteOffset() uint8 {
	return io.GetNextRegister(0x70) & 0x0F
}

// layer2ClipWindow returns the current X1,X2,Y1,Y2 clip window from NR
// 0x18 (nextreg.go/T-27) as (x1, x2, y1, y2), all inclusive, matching the
// wiki's own "0,255,0,191" full-screen reset default.
func (io *SpectrumIO) layer2ClipWindow() (x1, x2, y1, y2 uint8) {
	v := io.nextRegClipWin.values
	return v[0], v[1], v[2], v[3]
}

// layer2ULAOnTop decodes NR 0x15 bits 4-2 into whether the ULA layer should
// be drawn on top of Layer 2 for this frame's composite. Table per
// wiki.specnext.dev/Sprite_and_Layers_System_Register (confirmed against
// two independent fetches of that page, since nextreg.go's own existing
// 0x15 default-value comment describes 0x08 as "S L U" when the actual bit
// pattern for 0x08 decodes to %010 = "S U L" -- a pre-existing T-27
// discrepancy this function does not silently correct, since nextreg.go's
// stored default value is unrelated production behaviour already shipped
// in v0.6.x; flagged instead in docs/TRACKING.md's T-29 entry for someone
// to reconcile against real hardware separately). This function decodes
// whatever value is actually in the register correctly, independent of
// whether that pre-existing default itself is right:
//
//	%000 S L U   %001 L S U   %010 S U L
//	%011 L U S   %100 U S L   %101 U L S
//	%110/%111    core-3.1.1 blending modes -- NOT implemented; treated the
//	             same as %100 (ULA on top) as the closest safe fallback,
//	             since a wrong-but-plausible ordering beats silently
//	             ignoring the register's high bit pattern.
//
// With no sprite layer implemented anywhere in this emulator, every
// ordering collapses to a two-layer question: is ULA on top of Layer 2, or
// is Layer 2 on top of ULA? Per the wiki's own letter notation (leftmost
// letter is topmost -- confirmed directly against the raw page text, not
// a paraphrase, since an earlier fetch's own prose summary self-
// contradicted on this exact point), ULA is on top whenever U appears
// before L in the three-letter string: %010 (S U L) and %100/%101 (U S L,
// U L S) all put U ahead of L. Every other ordering (%000 S L U, %001
// L S U, %011 L U S) has L ahead of U, so Layer 2 is on top.
//
// Correction (2026-09-15, found while building T-30's genuine three-layer
// decode, nextLayerOrder below): this function's switch statement
// previously grouped %010 into the Layer2-on-top case, contradicting its
// OWN stated rule in the paragraph above (which correctly says "U before
// L means ULA-on-top", and S U L has U before L) -- a real, shipped bug
// since T-27/T-29, not a new discrepancy nextLayerOrder introduced. Its
// practical impact: NR 0x15's power-on default (0x08, bits 4-2 = %010)
// was being composited as Layer2-over-ULA when it should be ULA-over-
// Layer2, on every Next-mode session that never explicitly wrote NR 0x15.
// Fixed by moving %010 into the true case below; nextLayerOrder's own
// bottom-to-top table was cross-checked against this corrected version,
// not the original buggy one, before either was trusted.
func (io *SpectrumIO) layer2ULAOnTop() bool {
	bits := (io.GetNextRegister(0x15) >> 2) & 0x07
	switch bits {
	case 0b010, 0b100, 0b101: // S U L, U S L, U L S -- U appears before L
		return true
	case 0b110, 0b111: // blending modes, not implemented -- fallback
		return true
	default: // 000 S L U, 001 L S U, 011 L U S -- L appears before U
		return false
	}
}

// nextColourIndex is a real hardware Next palette index (0-255) -- an
// 8-bit-per-pixel value into whichever 256-entry 9-bit-RGB palette a
// caller resolves it through (T-31, nextpalette.go). Renamed from this
// file's own pre-T-31 "color2bpp" (a name that was never accurate even
// then -- 16 entries is 4 bits, not 2 -- kept only because this is the
// first change to touch the type since it was introduced).
type nextColourIndex uint8

// resolveNextPixelIndex is the shared colour-INDEX-resolution seam
// proposed in docs/proposals/next-gpu-compositing-seam.md: given a raw
// Next-mode pixel byte and the palette offset that applies to it (NR
// 0x70 bits 3-0 for Layer 2, a per-sprite attribute-table offset for
// T-30), returns the final 8-bit palette index real hardware would look
// up -- confirmed against wiki.specnext.dev's Layer_2_Control_Register
// and Sprites pages: both describe the identical arithmetic, "palette
// offset added to the top four bits of the pixel value" for Layer 2's
// 8bpp mode and "PPPP0000 + IIIIIIII" for sprites, which reduce to the
// same expression this function already computed pre-T-31 (raw +
// paletteOffset<<4, plain uint8 wraparound) -- T-31 did not need to
// change this arithmetic, only what happens to its result afterward.
//
// This function does NOT look the index up in a palette -- that needs
// to know which of the two palettes per layer is currently displayed
// (nextPaletteSystem.displayTarget), which this pure function has no
// access to and shouldn't: nextColourFor (below) is the seam's other
// half, doing that lookup. Both Layer 2 (layer2Pixel) and T-30's sprite
// pixel path (spritePixel, sprite.go) call this rather than each
// independently inventing the same raw-byte arithmetic.
func resolveNextPixelIndex(raw uint8, paletteOffset uint8) nextColourIndex {
	return nextColourIndex(raw + paletteOffset<<4)
}

// nextColourFor looks up an already-resolved index (resolveNextPixelIndex's
// result) in whichever of layer's two palettes NR 0x43's display-select
// bits currently show (T-31). Kept separate from resolveNextPixelIndex
// so a caller that needs the raw index itself -- to compare against a
// transparency index, as sprite.go's spritePixel does, before ever
// touching a palette -- doesn't have to look a colour up just to throw
// it away.
func (io *SpectrumIO) nextColourFor(layer nextLayer, idx nextColourIndex) color.NRGBA {
	return io.nextPalette.lookup(layer, uint8(idx)).Colour()
}

// layer2Pixel reads Layer 2's raw framebuffer byte at (x, y) -- in SCREEN
// coordinates (0..layer2Width-1, 0..layer2Height-1), i.e. the fixed
// 256x192 output canvas, never Layer 2's own wider framebuffer coordinate
// space -- and resolves it to a final palette index via
// resolveNextPixelIndex, using NR 0x70's palette offset
// (layer2PaletteOffset). Returns the index, not a colour -- DecodeLayer2
// (CPU path) and renderLayer2GPU (GPU path, videorender_gpu.go) each do
// their own colour lookup/caching from this index, the same split
// resolveNextPixelIndex/nextColourFor keep.
//
// Addressing per mode (wiki.specnext.dev/Layer_2, quoted directly since
// the wording matters -- "the pixels are stored in memory going from top
// to bottom and left to right (second byte is first pixel on second
// line, 256th byte is second pixel on first line)"):
//
//   - layer2Mode256x192x8bpp: row-major, address = y*256+x -- the
//     original, unchanged formula.
//   - layer2Mode320x256x8bpp: COLUMN-major -- address = x*256+y.
//     Cross-checked against BOTH of the wiki quote's own illustrative
//     examples under 1-indexed ordinal numbering ("second byte"=address
//     1, "256th byte"=address 255): "second byte is first pixel on
//     second line" matches this formula exactly (x=0,y=1 -> address 1);
//     "256th byte is second pixel on first line" is off by exactly one
//     under this formula (x=1,y=0 -> address 256, not 255) -- a
//     one-sided discrepancy read as the wiki using "the 256th byte"
//     colloquially (meaning "at offset 256") rather than strict ordinal
//     phrasing, not evidence the formula is wrong; an earlier version of
//     this very comment (and this pass's own first test for this
//     addressing) made the OPPOSITE misreading (treating x=0,y=1 as
//     landing at address 256), caught only once the test that encoded it
//     was actually run and failed -- see layer2_test.go's own
//     TestLayer2Pixel320x256UsesColumnMajorAddressing doc comment for the
//     full numeric working. This emulator has no X/Y scroll register
//     support (NR 0x16/0x17/0x71 are not in
//     nextRegDefaults -- see this file's header on the same gap for the
//     shadow bank), so a screen coordinate (x,y) samples the wider
//     framebuffer's own (x,y) directly with no scroll offset applied --
//     the visible window is always the framebuffer's own top-left
//     256x192/256x256 corner. Genuinely undocumented behaviour beyond
//     that point (what real hardware shows with no scroll register ever
//     written) is not invented here.
//   - layer2Mode640x256x4bpp: "stored identically to 320x256 mode, but
//     every byte contains two pixels" (wiki, direct quote) -- same
//     column-major addressing as the 320-wide mode, but the BYTE address
//     is for a PAIR of horizontally-adjacent pixels (x, x+1), with x even
//     addressing the top nibble ("left" pixel) and x odd the bottom
//     nibble ("right" pixel) of that same byte, per the wiki's explicit
//     "top nibble forms left pixel, bottom nibble forms right pixel".
//     The palette-offset ambiguity this raises is handled entirely in the
//     4bpp branch below, not in layer2PaletteOffset (see that function's
//     own comment).
func (m *SpectrumMemory) layer2Pixel(io *SpectrumIO, x, y int) nextColourIndex {
	base := io.layer2BaseBank()
	mode := io.layer2CurrentMode()
	switch mode {
	case layer2Mode320x256x8bpp:
		offset := x*256 + y
		raw := m.nextPageRead(base+uint8(offset/8192), uint16(offset%8192))
		return resolveNextPixelIndex(raw, io.layer2PaletteOffset())
	case layer2Mode640x256x4bpp:
		// Two screen pixels per byte: byte index is for the (x/2)'th pair
		// in column-major order, using the SAME x*256+y formula as the
		// 320-wide mode but addressed by pixel-pair rather than by pixel.
		pairX := x / 2
		byteOffset := pairX*256 + y
		raw := m.nextPageRead(base+uint8(byteOffset/8192), uint16(byteOffset%8192))
		var nibble uint8
		if x%2 == 0 {
			nibble = raw >> 4 // top nibble = "left" pixel
		} else {
			nibble = raw & 0x0F // bottom nibble = "right" pixel
		}
		// Inferred, not confirmed (see layer2PaletteOffset's own comment):
		// a native 4-bit pixel has no separate "top four bits" for the
		// offset to land above, so the offset is added directly to the
		// pixel's own 4-bit value here, matching resolveNextPixelIndex's
		// existing offset<<4+raw arithmetic would produce if raw itself
		// already occupied only the low nibble with a zero high nibble --
		// i.e. this reuses the exact same seam rather than inventing a
		// second one, just fed a nibble instead of a full byte.
		return resolveNextPixelIndex(nibble, io.layer2PaletteOffset())
	default: // layer2Mode256x192x8bpp
		offset := y*layer2Width + x
		raw := m.nextPageRead(base+uint8(offset/8192), uint16(offset%8192))
		return resolveNextPixelIndex(raw, io.layer2PaletteOffset())
	}
}

// DecodeLayer2 renders Layer 2's current framebuffer as a 256x192
// image.RGBA, honouring the NR 0x18 clip window: pixels outside the clip
// rectangle are transparent (alpha 0) rather than black, so a caller
// compositing this over the ULA image sees the ULA layer show through
// unclipped regions instead of a black rectangle. Returns nil if Layer 2
// is not enabled (see layer2Enabled) -- callers must check for nil rather
// than compositing an always-present image, since "Layer 2 not active" and
// "Layer 2 active showing an all-black frame" are genuinely different
// states worth keeping distinguishable at this layer.
func (io *SpectrumIO) DecodeLayer2() *image.RGBA {
	if !io.layer2Enabled() {
		return nil
	}
	x1, x2, y1, y2 := io.layer2ClipWindow()
	img := image.NewRGBA(image.Rect(0, 0, layer2Width, layer2Height))
	for y := 0; y < layer2Height; y++ {
		inYClip := y >= int(y1) && y <= int(y2)
		for x := 0; x < layer2Width; x++ {
			if !inYClip || x < int(x1) || x > int(x2) {
				continue // leaves img.Pix at zero value: alpha 0, transparent
			}
			idx := io.memory.layer2Pixel(io, x, y)
			c := io.nextColourFor(nextLayerLayer2, idx)
			img.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A})
		}
	}
	return img
}

// CompositeLayer2 blends a Layer 2 image over (or under) a ULA image per
// layer2ULAOnTop, producing the final on-screen picture. Both images must
// already be layer2Width x layer2Height (== ScreenWidth x ScreenHeight --
// the two constants are numerically identical but named separately
// per-layer since Layer 2's wider modes will one day diverge from the
// ULA's fixed 256x192). ula is never mutated -- a fresh image is returned
// -- so a caller holding onto the ULA-only image for another purpose
// (screenshot diffing, etc.) is unaffected.
//
// Transparent Layer 2 pixels (outside the clip window, alpha 0 -- see
// DecodeLayer2) always show the ULA pixel beneath them regardless of
// ulaOnTop, matching real hardware: the clip window controls where Layer 2
// draws at all, independent of the separate priority-ordering question of
// which layer wins where both would otherwise draw.
func CompositeLayer2(ula, l2 *image.RGBA, ulaOnTop bool) *image.RGBA {
	out := image.NewRGBA(ula.Rect)
	for y := ula.Rect.Min.Y; y < ula.Rect.Max.Y; y++ {
		for x := ula.Rect.Min.X; x < ula.Rect.Max.X; x++ {
			uc := ula.RGBAAt(x, y)
			lc := l2.RGBAAt(x, y)
			if lc.A == 0 || ulaOnTop {
				out.SetRGBA(x, y, uc)
				continue
			}
			out.SetRGBA(x, y, lc)
		}
	}
	return out
}

// nextLayer identifies one of the three layers NR 0x15 orders: sprites,
// ULA, and Layer 2. Named distinctly from any existing type so a caller
// can't confuse a layer identifier with a colour index or register value.
type nextLayer uint8

const (
	nextLayerSprites nextLayer = iota
	nextLayerULA
	nextLayerLayer2
)

// nextLayerOrder decodes NR 0x15 bits 4-2 into the genuine three-layer
// back-to-front drawing order: index 0 is drawn first (bottommost, loses
// every overlap), index 2 is drawn last (topmost, wins every overlap).
// layer2ULAOnTop's own doc comment already flagged that its boolean
// collapse "was only ever correct in the sprite-absent world T-29 shipped
// into" -- this function is the real decode docs/proposals/
// next-gpu-compositing-seam.md said T-30 would need.
//
// Table per wiki.specnext.dev/Sprite_and_Layers_System_Register, whose
// own wording is explicit about direction: "S L U (Sprites are at top,
// Layer 2 under, Enhanced_ULA at bottom)" -- i.e. the LEFTMOST letter in
// the wiki's notation is the TOPMOST layer, the opposite of this
// function's own [3]nextLayer array order (bottom-to-top), so each case
// below is the wiki's top-to-bottom triple written out back-to-front:
//
//	%000 S L U (top->bottom)  -> bottom->top: U, L, S
//	%001 L S U (top->bottom)  -> bottom->top: U, S, L
//	%010 S U L (top->bottom)  -> bottom->top: L, U, S
//	%011 L U S (top->bottom)  -> bottom->top: S, U, L
//	%100 U S L (top->bottom)  -> bottom->top: L, S, U
//	%101 U L S (top->bottom)  -> bottom->top: S, L, U
//	%110/%111 core-3.1.1 blending modes -- NOT implemented; treated the
//	          same as %100 (U S L) as the closest safe fallback, matching
//	          layer2ULAOnTop's own established fallback choice.
//
// Cross-checked against layer2ULAOnTop's own already-shipped, already-
// tested two-layer table for every one of the 6 real cases (that
// function's ULA-on-top/Layer2-on-top answer for each bit pattern agrees
// with this function's relative U/L ordering here) before this function
// was trusted -- both decode the same three bits from the same source, so
// they cannot be allowed to silently disagree.
func nextLayerOrder(nr15 uint8) [3]nextLayer {
	bits := (nr15 >> 2) & 0x07
	switch bits {
	case 0b000: // S L U -> bottom->top: U, L, S
		return [3]nextLayer{nextLayerULA, nextLayerLayer2, nextLayerSprites}
	case 0b001: // L S U -> bottom->top: U, S, L
		return [3]nextLayer{nextLayerULA, nextLayerSprites, nextLayerLayer2}
	case 0b010: // S U L -> bottom->top: L, U, S
		return [3]nextLayer{nextLayerLayer2, nextLayerULA, nextLayerSprites}
	case 0b011: // L U S -> bottom->top: S, U, L
		return [3]nextLayer{nextLayerSprites, nextLayerULA, nextLayerLayer2}
	case 0b100: // U S L -> bottom->top: L, S, U
		return [3]nextLayer{nextLayerLayer2, nextLayerSprites, nextLayerULA}
	case 0b101: // U L S -> bottom->top: S, L, U
		return [3]nextLayer{nextLayerSprites, nextLayerLayer2, nextLayerULA}
	default: // 110/111 blending modes -- fallback to %100 U S L
		return [3]nextLayer{nextLayerLayer2, nextLayerSprites, nextLayerULA}
	}
}

// CompositeNextLayers blends ULA, Layer 2, and sprite images together per
// nextLayerOrder, producing the final on-screen picture -- the sprite-
// aware successor to CompositeLayer2 (kept as-is for its own existing
// callers/tests; this function is DecodeDisplay's new entry point once
// sprites exist). l2 and sprites may each be nil (Layer 2 disabled,
// sprite layer disabled) -- a nil layer is treated as fully transparent
// everywhere, exactly like an all-transparent image would be, so passing
// nil rather than constructing a throwaway blank image is both cheaper
// and matches DecodeLayer2/DecodeSprites' own "return nil when disabled"
// convention. ula must never be nil -- there is always a base ULA image.
//
// Composites bottom-to-top per pixel: each layer in nextLayerOrder either
// overwrites the running result with its own opaque pixel, or leaves the
// result untouched where it's transparent -- so the last opaque layer in
// the order wins, matching "topmost wins every overlap". ULA itself is
// always treated as fully opaque (it is the base image, never a
// transparent one), so it always sets the result when its turn comes.
func CompositeNextLayers(ula, l2, sprites *image.RGBA, nr15 uint8) *image.RGBA {
	order := nextLayerOrder(nr15)
	out := image.NewRGBA(ula.Rect)
	for y := ula.Rect.Min.Y; y < ula.Rect.Max.Y; y++ {
		for x := ula.Rect.Min.X; x < ula.Rect.Max.X; x++ {
			chosen := ula.RGBAAt(x, y)
			for _, layer := range order {
				switch layer {
				case nextLayerULA:
					chosen = ula.RGBAAt(x, y)
				case nextLayerLayer2:
					if l2 != nil {
						if c := l2.RGBAAt(x, y); c.A != 0 {
							chosen = c
						}
					}
				case nextLayerSprites:
					if sprites != nil {
						if c := sprites.RGBAAt(x, y); c.A != 0 {
							chosen = c
						}
					}
				}
			}
			out.SetRGBA(x, y, chosen)
		}
	}
	return out
}

// layersAboveULA returns, in back-to-front draw order, which of Layer 2
// and Sprites should be drawn as an additional GPU pass on top of an
// already-drawn ULA image. Built on nextLayerOrder (this file, above):
// since the ULA layer is always drawn onto the render target first (the
// base texture blit in DisplayManager.Render, before renderLayer2GPU or
// a future renderSpritesGPU ever runs), any layer positioned BELOW ULA in
// the real three-layer order is already correctly hidden -- ULA's own
// draw call already painted over it -- so only layers positioned ABOVE
// ULA need a real draw call at all. This is what makes it safe for the
// GPU fast path to skip a layer entirely rather than draw it and then
// have ULA draw over it again: this function only ever returns layers
// that must draw AFTER ULA, in the order they must draw, so a caller
// iterating the result and drawing each one in turn reproduces
// CompositeNextLayers' own per-pixel "last opaque layer wins" semantics
// without needing to touch pixels ULA already got right.
//
// Only ever returns nextLayerLayer2 and/or nextLayerSprites -- ULA itself
// is never included, since this function's whole purpose is "what to draw
// on top of the ULA that's already there."
func layersAboveULA(nr15 uint8) []nextLayer {
	order := nextLayerOrder(nr15)
	ulaIndex := 0
	for i, l := range order {
		if l == nextLayerULA {
			ulaIndex = i
			break
		}
	}
	var above []nextLayer
	for _, l := range order[ulaIndex+1:] {
		above = append(above, l)
	}
	return above
}

// layerIsAboveULA reports whether layer l is drawn above ULA in the
// current NR 0x15 ordering -- a single-layer convenience over
// layersAboveULA (above) for a caller (renderLayer2GPU today, a future
// renderSpritesGPU) that only needs a yes/no gate for one specific layer
// rather than the full ordered slice.
func layerIsAboveULA(l nextLayer, nr15 uint8) bool {
	for _, above := range layersAboveULA(nr15) {
		if above == l {
			return true
		}
	}
	return false
}
