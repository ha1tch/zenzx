package main

import (
	"image"
	"image/color"
)

// ============================================================================
// ZX Spectrum Next: hardware sprites (T-30, wave ti0.r4; scaling, mirror,
// rotation, and composite/unified relative sprites added in the T-29/T-30
// completion pass alongside T-29's own remaining Layer 2 work)
//
// Scope, per docs/TRACKING.md T-30 and NEXT_SUPPORT_DEVELOPMENT_PLAN.md's
// ti0.r4 "done when": 128 sprites, 16x16 pixels, 8-bit colour, composited
// per NR 0x15's layer-priority register, reachable via NextREG (nextreg.go's
// 0x34 sprite-select + 0x35-0x39 attribute registers -- see that file's own
// comments on why the NextREG-mirror path was chosen over the real
// hardware's alternative port-0x303B/0x57/0x5B path: the plan only requires
// "reachable via ti0.r1's NextREG mechanism", not a specific one of the
// two). 4-bit colour patterns remain explicitly deferred (no source
// consulted gives this emulator a second pattern-memory format, and
// nothing in this pass's own scope required it); everything else
// NEXT_SUPPORT_DEVELOPMENT_PLAN.md's "Deferred within Tier 0" list named
// -- scaling, rotation, mirroring, composite sprites -- is implemented as
// of this pass.
//
// Attribute format (confirmed against wiki.specnext.dev/Sprites and
// wiki.specnext.dev/Sprite_port-mirror_Attribute_{2,4}_Register, each
// fetched independently and cross-checked against the other's references
// to the same bits before being trusted -- the byte-4/attribute-2
// "rotation is applied before mirroring" ordering, and attribute 4's
// dual meaning depending on the sprite's own bits 7-6, are both stated
// directly on the source pages, not inferred):
//
//	attr0 (X LSB)    bits 7-0: X position bits 7-0 (anchor); signed 8-bit
//	                 X offset from the anchor's position (relative)
//	attr1 (Y LSB)    bits 7-0: Y position bits 7-0 (anchor, no MSB here --
//	                 that lives in attr4 bit 0 for anchors); signed 8-bit
//	                 Y offset from the anchor (relative)
//	attr2            bits 7-4: palette offset (added to the pixel's raw
//	                 index, same convention as Layer 2's NR 0x70); bit 3
//	                 X mirror; bit 2 Y mirror; bit 1 rotate (90-degree
//	                 clockwise, applied before mirroring per the wiki's
//	                 own explicit ordering); bit 0: X position MSB, 9th
//	                 bit (anchor) / "PR" enable-relative-palette-offset
//	                 (relative -- 1 = anchor's palette offset is added to
//	                 this sprite's own)
//	attr3            bit 7: visible; bit 6: "E", enable attribute 4 (if
//	                 0, attribute 4 is never written/read for this
//	                 sprite and it behaves as an unscaled, unrotated,
//	                 non-relative anchor -- exactly this emulator's
//	                 pre-this-pass behaviour); bits 5-0: pattern index
//	                 (0-63)
//	attr4 (NR 0x39, only meaningful when attr3 bit 6 is set) -- dual
//	format, discriminated by bits 7-6 alone (confirmed directly: "%01
//	signals relative sprite mode" regardless of any other bit):
//	  bits 7-6 = %01: RELATIVE sprite.
//	    bit 5    relative TYPE: 0 = composite (independent), 1 = unified/
//	             "big sprite" (inherits the anchor's rotate/mirror/scale,
//	             hardware repositions+retransforms this sprite around it)
//	    bits 4-3 X scale (this sprite's own, used only in composite mode
//	             -- unified mode uses the ANCHOR's scale for everything,
//	             per this sprite's own inherited transform)
//	    bits 2-1 Y scale (composite mode only, same reasoning)
//	    bit 0    "PO": enable relative pattern-index offset (1 = this
//	             sprite's own pattern index, attr3 bits 5-0, is ADDED to
//	             the anchor's pattern index rather than used standalone)
//	  bits 7-6 = %00 or %10/%11: ANCHOR sprite (extended).
//	    bit 5    relative TYPE this anchor sets for ITS OWN following
//	             relatives (0 = composite, 1 = unified) -- confirmed
//	             against the wiki's own "type for following relative
//	             sprites" wording: this bit lives on the ANCHOR, not on
//	             each relative individually, so every relative following
//	             one anchor shares that anchor's type choice
//	    bits 4-3 X scale: %00=1x %01=2x %10=4x %11=8x
//	    bits 2-1 Y scale: same encoding
//	    bit 0    Y position MSB (9th bit, combined with attr1 for 0-511)
//	  H (bit 7 as %1x)/N6 (bit 6) flag 4-bit sprite patterns -- not
//	  implemented (this file's header); an anchor with H set is decoded
//	  as if H were clear (falls into the %10/%11 anchor case above,
//	  which this emulator treats identically to %00 since it has no
//	  4-bit pattern format to switch to), rather than silently
//	  misreading it as a relative sprite the way a naive "bits 7-6 != 0"
//	  check would.
//
// Anchor tracking: real hardware processes sprites 0-127 in order,
// remembering the most recent ANCHOR (a sprite whose OWN attr4 bits 7-6
// != %01, or one with attr3 bit 6 clear, which behaves as if it had no
// attribute 4 at all and is therefore always its own anchor) as "the
// current anchor" for every following sprite until the next anchor.
// spriteSystem.effectiveAttrs (below) computes this once per frame in
// index order, exactly mirroring that real hardware sequencing, rather
// than each sprite independently trying to find "its" anchor.
//
// Pattern memory: 64 pattern slots (matching attribute 3's 6-bit pattern
// index range), each a 16x16 grid of raw 8-bit Next colour bytes -- the
// same raw-byte-to-colour resolution Layer 2 uses (resolveNextPixelIndex,
// layer2.go), per docs/proposals/next-gpu-compositing-seam.md's Phase 1:
// sprites are the seam's second consumer, not a second independent colour
// mapping. Real hardware addresses pattern memory via a separate selector
// and a sequential-write port (0x303B pattern mode + 0x5B); this emulator
// exposes it as a direct Go-level write method (WriteSpritePatternByte)
// since no NextREG-mirror path for pattern memory is documented anywhere
// the wiki page consulted describes -- test code and any future CPU-side
// port wiring both go through this one method.
//
// Rendering: scale/rotate/mirror are applied as GEOMETRY, not as separate
// baked pixel variants -- see videorender_gpu.go's renderSpritesGPU for
// the reasoning (raylib's DrawTexturePro applies source-rect flip, a
// rotation angle, and a scaled dest rect all from ONE unscaled/
// unrotated/unmirrored baked texture per (pattern, paletteOffset), so the
// GPU cache key does NOT need a transform axis at all -- a cheaper and
// more precise design than the transform-inclusive cache key floated
// before this pass started, once DrawTexturePro's actual capabilities
// were confirmed against the vendored raylib-go source rather than
// assumed). The portable, non-GPU path (spritePixel/DecodeSprites below)
// applies the same transforms in plain Go arithmetic, since it has no
// hardware rasterizer to hand the work to.
// ============================================================================

const (
	spriteCount        = 128
	spritePatternSlots = 64 // matches attribute 3's 6-bit pattern-index range
	spriteWidth        = 16
	spriteHeight       = 16

	// spriteTransparentIndexDefault is NR 0x4B's documented power-on/
	// soft-reset default (0xE3) -- see nextRegDefaults' own 0x4B entry
	// (nextreg.go) for the live, configurable value; this constant is
	// now only the seed nextRegDefaults' table uses, not a fixed
	// stand-in (T-29/T-30 completion pass makes NR 0x4B real).
	spriteTransparentIndexDefault = uint8(0xE3)
)

// spriteRelativeType distinguishes the two real relative-sprite modes an
// anchor's own attribute 4 bit 5 selects for its following relatives.
type spriteRelativeType uint8

const (
	spriteRelativeComposite spriteRelativeType = iota // independent rendering
	spriteRelativeUnified                             // inherits anchor's transform
)

// spriteScale is one axis's real hardware scale factor (attr4 bits 4-3 or
// 2-1): %00=1x %01=2x %10=4x %11=8x. A distinct type from a bare int so
// call sites can't confuse a raw 2-bit field with an already-multiplied
// factor.
type spriteScale uint8

const (
	spriteScale1x spriteScale = iota
	spriteScale2x
	spriteScale4x
	spriteScale8x
)

// factor returns the real integer multiplier this scale value represents.
func (s spriteScale) factor() int {
	switch s {
	case spriteScale2x:
		return 2
	case spriteScale4x:
		return 4
	case spriteScale8x:
		return 8
	default:
		return 1
	}
}

// decodeSpriteScale decodes a raw 2-bit field (already shifted to bits
// 1-0) into a spriteScale.
func decodeSpriteScale(bits uint8) spriteScale {
	return spriteScale(bits & 0x03)
}

// spriteAttributes holds one sprite's RAW attribute bytes, not
// pre-decoded fields -- unlike this struct's pre-this-pass shape, attr2
// and attr4's MEANING depends on bits that can be written in either
// order (attr4's own bits 7-6 discriminator, and attr3 bit 6's "E" gate,
// can each be written before or after the other attribute bytes; the
// wiki does not document a mandatory write order), so decoding eagerly
// on every individual byte write risks caching a decode made from an
// incomplete or since-superseded picture. effectiveAttrs (below)
// decodes the full, current picture for all 128 sprites together, once
// per frame, after every attribute write for that frame has already
// landed.
type spriteAttributes struct {
	attr0 uint8 // X LSB (anchor) / signed X offset (relative)
	attr1 uint8 // Y LSB (anchor) / signed Y offset (relative)
	attr2 uint8 // palette offset, mirror, rotate, X-MSB/PR
	attr3 uint8 // visible, E (attr4 enable), pattern index
	attr4 uint8 // scale/relative-type/Y-MSB or PO, only if attr3 bit6 set
}

// spriteResolved is one sprite's fully-decoded, anchor-resolved render
// state for the current frame -- what DecodeSprites/renderSpritesGPU
// actually consume, computed by spriteSystem.effectiveAttrs. Kept
// distinct from spriteAttributes (the raw written bytes) so the anchor-
// walk and transform inheritance happen in exactly one place.
type spriteResolved struct {
	visible       bool
	x, y          int // final screen position, top-left of the 16x16 cell (post-offset for relatives, pre-scale)
	pattern       uint8
	paletteOffset uint8
	xMirror       bool
	yMirror       bool
	rotate        bool
	scaleX        spriteScale
	scaleY        spriteScale
}

// spriteSystem is the sprite-adjacent state living on SpectrumIO,
// mirroring how nextRegs/nextRegClipWin are owned there: the 128-entry
// attribute table, the currently-selected sprite (NR 0x34), and pattern
// memory (spritePatternSlots slots of spriteWidth*spriteHeight raw bytes
// each).
type spriteSystem struct {
	selected uint8
	attrs    [spriteCount]spriteAttributes
	patterns [spritePatternSlots][spriteWidth * spriteHeight]uint8

	// patternGeneration[slot] increments every time that pattern slot's
	// pixel data is written (WriteSpritePatternByte) -- the dirty-flag
	// mechanism docs/proposals/next-gpu-compositing-seam.md's own sprite
	// cache design calls for: renderSpritesGPU (videorender_gpu.go)
	// compares its own baked-against generation for a slot against this
	// live value the same way a GPU cache compares nextPaletteVersion, and
	// rebakes only when they diverge. A plain counter rather than a bool
	// so a slot rebaked once, then written again before the next frame,
	// is still correctly detected as dirty a second time.
	patternGeneration [spritePatternSlots]uint32
}

func newSpriteSystem() *spriteSystem {
	return &spriteSystem{}
}

// writeAttribute applies a write to attribute register attrIndex (0-4,
// corresponding to NextREG 0x35-0x39) for whichever sprite is currently
// selected (NR 0x34) -- called from nextreg.go's nextRegWriteDirect the
// same way that function calls nextRegClipWin.write for register 0x18.
// Stores the raw byte only -- see spriteAttributes' own doc comment on
// why decoding is deferred to effectiveAttrs.
func (s *spriteSystem) writeAttribute(attrIndex uint8, value uint8) {
	a := &s.attrs[s.selected]
	switch attrIndex {
	case 0:
		a.attr0 = value
	case 1:
		a.attr1 = value
	case 2:
		a.attr2 = value
	case 3:
		a.attr3 = value
	case 4:
		a.attr4 = value
	}
}

// isAnchor reports whether this sprite's OWN attributes make it an
// anchor (true) or a relative (false) -- attr3 bit 6 clear means
// attribute 4 is never consulted at all (this emulator's pre-scale/
// rotate/relative behaviour: always its own, unscaled, unrotated
// anchor); attr3 bit 6 set defers to attr4 bits 7-6, where %01 alone
// means relative (H/N6 4-bit-pattern bits on an anchor, %10/%11, are
// deliberately not confused with the relative discriminator -- see this
// file's header on why a naive "!= 0" check would be wrong here).
func (a spriteAttributes) isAnchor() bool {
	if a.attr3&0x40 == 0 {
		return true
	}
	return (a.attr4>>6)&0x03 != 0b01
}

// decodeAnchor decodes this sprite's attributes assuming isAnchor() is
// true. hasExt reports whether attr4 is actually in effect (attr3 bit 6
// set) -- when false, scale is 1x/1x, mirror/rotate come from attr2
// (always meaningful regardless of attr4), and there is no Y MSB.
func (a spriteAttributes) decodeAnchor() (x uint16, y uint16, paletteOffset uint8, xMirror, yMirror, rotate bool, scaleX, scaleY spriteScale, relType spriteRelativeType, hasExt bool) {
	x = uint16(a.attr0)
	if a.attr2&0x01 != 0 {
		x |= 0x100
	}
	y = uint16(a.attr1)
	paletteOffset = (a.attr2 >> 4) & 0x0F
	xMirror = a.attr2&0x08 != 0
	yMirror = a.attr2&0x04 != 0
	rotate = a.attr2&0x02 != 0
	scaleX, scaleY = spriteScale1x, spriteScale1x
	relType = spriteRelativeComposite
	hasExt = a.attr3&0x40 != 0
	if hasExt {
		if a.attr4&0x01 != 0 {
			y |= 0x100
		}
		scaleX = decodeSpriteScale((a.attr4 >> 3) & 0x03)
		scaleY = decodeSpriteScale((a.attr4 >> 1) & 0x03)
		if a.attr4&0x20 != 0 {
			relType = spriteRelativeUnified
		}
	}
	return
}

// decodeRelative decodes this sprite's attributes assuming isAnchor() is
// false (attr4 bits 7-6 == %01, which requires attr3 bit 6 to have been
// set -- isAnchor already checked that). dx/dy are SIGNED 8-bit offsets
// from the anchor. prEnable/poEnable are the "add the anchor's own
// palette-offset/pattern-index to mine" flags (attr2 bit 0 / attr4 bit 0
// respectively) -- named to match the wiki's own PR/PO abbreviations.
func (a spriteAttributes) decodeRelative() (dx, dy int8, paletteOffset uint8, xMirror, yMirror, rotate bool, prEnable bool, scaleX, scaleY spriteScale, poEnable bool, unified bool) {
	dx = int8(a.attr0)
	dy = int8(a.attr1)
	paletteOffset = (a.attr2 >> 4) & 0x0F
	xMirror = a.attr2&0x08 != 0
	yMirror = a.attr2&0x04 != 0
	rotate = a.attr2&0x02 != 0
	prEnable = a.attr2&0x01 != 0
	scaleX = decodeSpriteScale((a.attr4 >> 3) & 0x03)
	scaleY = decodeSpriteScale((a.attr4 >> 1) & 0x03)
	poEnable = a.attr4&0x01 != 0
	unified = a.attr4&0x20 != 0
	return
}

// rotatePoint90CW rotates an integer offset (dx, dy) by 90 degrees
// clockwise around the origin -- screen Y grows downward, so clockwise
// on screen is (dx, dy) -> (-dy, dx), the standard screen-space rotation
// matrix (unlike math convention, which assumes Y grows upward and
// would give the opposite sign). Used by unified-mode relative-sprite
// offset transformation (effectiveAttrs) to match "rotating the anchor
// causes all the relatives to rotate around the anchor."
func rotatePoint90CW(dx, dy int) (int, int) {
	return -dy, dx
}

// effectiveAttrs computes every sprite's fully-resolved render state for
// the current frame, walking sprites 0-127 in order and tracking the
// most recent anchor exactly as real hardware does (this file's header
// comment). A relative sprite with no anchor yet seen (index 0 itself is
// relative, or -- degenerate, but not disallowed by any source consulted
// -- attr3 bit6 is clear so it can never actually decode as relative in
// the first place) falls back to behaving as its own anchor at (0,0),
// since there is no real anchor state to inherit from and inventing one
// would be worse than a clearly-wrong, clearly-visible fallback position.
func (s *spriteSystem) effectiveAttrs() [spriteCount]spriteResolved {
	var out [spriteCount]spriteResolved
	var anchor spriteResolved
	var anchorRelType spriteRelativeType
	haveAnchor := false
	for i := 0; i < spriteCount; i++ {
		a := s.attrs[i]
		visible := a.attr3&0x80 != 0
		pattern := a.attr3 & 0x3F
		if a.isAnchor() {
			x, y, po, xm, ym, rot, sx, sy, relType, _ := a.decodeAnchor()
			r := spriteResolved{
				visible: visible, x: int(x), y: int(y), pattern: pattern,
				paletteOffset: po, xMirror: xm, yMirror: ym, rotate: rot,
				scaleX: sx, scaleY: sy,
			}
			out[i] = r
			anchor = r
			anchorRelType = relType
			haveAnchor = true
			continue
		}
		// Relative sprite.
		dx, dy, po, xm, ym, rot, pr, sx, sy, po2, unifiedSelf := a.decodeRelative()
		if !haveAnchor {
			// No real anchor to inherit from -- see doc comment.
			anchor = spriteResolved{}
			anchorRelType = spriteRelativeComposite
			haveAnchor = true
		}
		effPalette := po
		if pr {
			effPalette = anchor.paletteOffset + po
		}
		effPattern := pattern
		if po2 {
			effPattern = anchor.pattern + pattern
		}
		_ = unifiedSelf // the relative's OWN bit 5 selects nothing per the wiki -- type is the ANCHOR's choice (anchorRelType); kept decoded for documentation/testing symmetry with decodeAnchor's relType, not consumed here.
		if anchorRelType == spriteRelativeUnified {
			// Unified/"big sprite": the anchor's rotate/mirror/scale
			// apply to the WHOLE group, and the offset itself is
			// rotated/mirrored/scaled around the anchor before being
			// added to the anchor's own position -- "the hardware will
			// automatically adjust X,Y coords ... according to settings
			// in the anchor" (wiki, direct quote, this file's header).
			ddx, ddy := int(dx), int(dy)
			if anchor.rotate {
				ddx, ddy = rotatePoint90CW(ddx, ddy)
			}
			if anchor.xMirror {
				ddx = -ddx
			}
			if anchor.yMirror {
				ddy = -ddy
			}
			ddx *= anchor.scaleX.factor()
			ddy *= anchor.scaleY.factor()
			out[i] = spriteResolved{
				visible: visible && anchor.visible, // "visibility ANDed together" (wiki)
				x:       anchor.x + ddx, y: anchor.y + ddy,
				pattern: effPattern, paletteOffset: effPalette,
				xMirror: anchor.xMirror, yMirror: anchor.yMirror, rotate: anchor.rotate,
				scaleX: anchor.scaleX, scaleY: anchor.scaleY,
			}
			continue
		}
		// Composite: independent rendering, own transform, offset added
		// to the anchor's raw position with no rotation/scale applied
		// to the offset itself.
		out[i] = spriteResolved{
			visible: visible && anchor.visible,
			x:       anchor.x + int(dx), y: anchor.y + int(dy),
			pattern: effPattern, paletteOffset: effPalette,
			xMirror: xm, yMirror: ym, rotate: rot,
			scaleX: sx, scaleY: sy,
		}
	}
	return out
}

// WriteSpritePatternByte writes one raw Next-colour byte into pattern
// slot `slot`'s pixel data at (x, y) -- see this file's header comment on
// why this is a direct method rather than a NextREG-mirrored port path.
func (s *spriteSystem) WriteSpritePatternByte(slot uint8, x, y int, value uint8) {
	if int(slot) >= spritePatternSlots || x < 0 || x >= spriteWidth || y < 0 || y >= spriteHeight {
		return
	}
	s.patterns[slot][y*spriteWidth+x] = value
	s.patternGeneration[slot]++
}

// spriteEnabled reports whether the sprite layer should be composited at
// all this frame. Matches layer2Enabled's own reasoning (layer2.go): NR
// 0x15 bit 0 is documented as "sprite visibility enable" per the wiki, so
// unlike Layer 2 (which has no such bit and is gated purely on Next mode)
// sprites DO have a real, documented enable bit -- honour it rather than
// gating on isNext alone.
func (io *SpectrumIO) spriteEnabled() bool {
	return io.memory != nil && io.memory.isNext && io.GetNextRegister(0x15)&0x01 != 0
}

// spriteTransparentIndex returns NR 0x4B's current value -- the raw
// pattern-byte value treated as transparent for 8-bit sprites (see
// spritePixelIndexAt's own comment on why this emulator uses the full
// byte rather than the wiki's 4-bit-sprite-specific low-nibble rule).
func (io *SpectrumIO) spriteTransparentIndex() uint8 {
	return io.GetNextRegister(0x4B)
}

// spritePixelIndexAt resolves resolved sprite r's pattern pixel at its
// own LOCAL, PRE-TRANSFORM (lx, ly) coordinate (0-15 each -- i.e. the
// coordinate within the pattern's own 16x16 grid, before any
// mirror/rotate this sprite applies) to a final palette index, or
// reports transparent. Returns (index, opaque).
//
// Transparency is checked against the RAW pattern byte, before
// resolveNextPixelIndex applies the palette offset -- confirmed against
// wiki.specnext.dev/Sprite_Transparency_Index_Register's own explicit
// wording, "the pixel index is compared before palette-offset is applied
// to it." This corrects a bug this pass found in the pre-existing code:
// the previous version compared the OFFSET-resolved index against the
// transparent constant, so any sprite with a non-zero palette offset got
// silently wrong transparency. Fixed here as part of making NR 0x4B
// itself real (nextreg.go) rather than left as a fixed stand-in with the
// comparison-order bug still underneath it.
func (io *SpectrumIO) spritePixelIndexAt(r spriteResolved, lx, ly int) (nextColourIndex, bool) {
	raw := io.sprites.patterns[r.pattern][ly*spriteWidth+lx]
	if raw == io.spriteTransparentIndex() {
		return 0, false
	}
	return resolveNextPixelIndex(raw, r.paletteOffset), true
}

// spriteLocalCoord maps a point WITHIN the sprite's own final on-screen
// footprint (ox, oy, each 0..15*scale-1) back to the corresponding
// PRE-TRANSFORM pattern coordinate (lx, ly, each 0-15) -- the inverse of
// "scale then mirror then rotate", applied in reverse order (unrotate,
// unmirror, unscale) since transforms compose that way. Used by the
// portable (non-GPU) path, DecodeSprites, which walks screen pixels
// forward rather than letting a hardware rasterizer do the geometry the
// way renderSpritesGPU's DrawTexturePro call does. ok=false if (ox, oy)
// falls outside the sprite's actual scaled footprint (can happen after
// rotation swaps which scale axis governs width vs height).
// The forward pipeline this function inverts, step by step, matching
// "rotation is applied before mirroring" (wiki, quoted in this file's
// header): pattern (16x16, coordinate (lx,ly)) -> SCALE (each pattern
// pixel becomes a scaleX x scaleY block, giving a (16*scaleX) x
// (16*scaleY) image) -> ROTATE 90 clockwise (swaps width/height) ->
// MIRROR (X and/or Y, within the now-rotated footprint) -> final screen
// footprint, coordinate (ox,oy). Inverting therefore undoes mirror
// first, then rotate, then scale -- the exact reverse order.
func spriteLocalCoord(r spriteResolved, ox, oy int) (lx, ly int, ok bool) {
	scaledW := spriteWidth * r.scaleX.factor()  // width after scale, before rotate
	scaledH := spriteHeight * r.scaleY.factor() // height after scale, before rotate
	footW, footH := scaledW, scaledH            // final footprint, after rotate
	if r.rotate {
		footW, footH = scaledH, scaledW
	}
	if ox < 0 || ox >= footW || oy < 0 || oy >= footH {
		return 0, 0, false
	}

	// Step 1 (inverse of the LAST forward step): undo mirroring, against
	// the rotated footprint's own extent (footW/footH) -- mirroring is
	// applied after rotation, so it operates on the already-rotated
	// image and its own inverse must too.
	x, y := ox, oy
	if r.xMirror {
		x = footW - 1 - x
	}
	if r.yMirror {
		y = footH - 1 - y
	}

	// Step 2: undo the 90-degree-clockwise rotation, mapping back from
	// the footW x footH rotated frame to the scaledW x scaledH
	// pre-rotation frame.
	//
	// Forward rotation of a point (px,py) in a W x H box to its position
	// (rx,ry) in the resulting H x W box, 90 degrees clockwise in
	// screen-space (Y-down) coordinates, is (rx,ry) = (H-1-py, px) --
	// verified numerically (not just derived by eye) against a brute-
	// force rotation of a labelled 3x2 test grid before being trusted;
	// an earlier version of this function's inverse was ALSO derived by
	// eye, shipped with a wrong formula, and only caught by that same
	// numerical check, which is why this comment insists on it.
	//
	// Solving (rx,ry) = (H-1-py, px) for (px,py) given (rx,ry):
	// px = ry, py = H-1-rx. So the inverse is (x,y) -> (y, H-1-x),
	// where H is the PRE-rotation height (scaledH here) -- NOT
	// scaledW; that was the earlier version's actual mistake.
	if r.rotate {
		x, y = y, scaledH-1-x
	}

	// Step 3: undo scaling, back to the original 16x16 pattern grid.
	lx = x / r.scaleX.factor()
	ly = y / r.scaleY.factor()
	if lx < 0 || lx >= spriteWidth || ly < 0 || ly >= spriteHeight {
		return 0, 0, false
	}
	return lx, ly, true
}

// spritePixel resolves resolved sprite r's on-screen offset (ox, oy)
// (within its own scaled/rotated footprint) to a real displayable colour
// via spriteLocalCoord + spritePixelIndexAt plus a Sprites-palette
// lookup (nextColourFor, T-31), or reports transparent (including "off
// the sprite's own footprint entirely"). Returns (colour, opaque).
func (io *SpectrumIO) spritePixel(r spriteResolved, ox, oy int) (color.NRGBA, bool) {
	lx, ly, ok := spriteLocalCoord(r, ox, oy)
	if !ok {
		return color.NRGBA{}, false
	}
	idx, opaque := io.spritePixelIndexAt(r, lx, ly)
	if !opaque {
		return color.NRGBA{}, false
	}
	return io.nextColourFor(nextLayerSprites, idx), true
}

// spriteFootprint returns resolved sprite r's final on-screen width and
// height, accounting for scale and the width/height swap a 90-degree
// rotation causes.
func spriteFootprint(r spriteResolved) (w, h int) {
	w = spriteWidth * r.scaleX.factor()
	h = spriteHeight * r.scaleY.factor()
	if r.rotate {
		w, h = h, w
	}
	return
}

// DecodeSprites renders every visible sprite into a 256x192 image.RGBA,
// sprite 0 drawn first and sprite 127 last so a higher-numbered sprite
// wins where two overlap -- matching the wiki's documented default
// priority (real hardware also offers the reverse via NR 0x15 bit 6,
// explicitly deferred here along with the other NR 0x15 bits this scope
// doesn't consume; see nextLayerOrder's own comment). Pixels with no
// visible, opaque sprite covering them are left at the image's zero
// value (alpha 0, transparent), exactly matching DecodeLayer2's own
// convention so both layers composite the same way. Returns nil if the
// sprite layer is not enabled (spriteEnabled).
func (io *SpectrumIO) DecodeSprites() *image.RGBA {
	if !io.spriteEnabled() {
		return nil
	}
	resolved := io.sprites.effectiveAttrs()
	img := image.NewRGBA(image.Rect(0, 0, layer2Width, layer2Height))
	for i := 0; i < spriteCount; i++ {
		r := resolved[i]
		if !r.visible {
			continue
		}
		w, h := spriteFootprint(r)
		for oy := 0; oy < h; oy++ {
			py := r.y + oy
			if py < 0 || py >= layer2Height {
				continue
			}
			for ox := 0; ox < w; ox++ {
				px := r.x + ox
				if px < 0 || px >= layer2Width {
					continue
				}
				c, opaque := io.spritePixel(r, ox, oy)
				if !opaque {
					continue
				}
				img.SetRGBA(px, py, color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A})
			}
		}
	}
	return img
}
