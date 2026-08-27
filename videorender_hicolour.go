package main

import "image"

// ============================================================================
// mode-timex-001-hicolour: Timex Extended Colour Mode
//
// Per docs/timex-modes.md, corroborated by four independent sources
// including the T/S 2068 Technical Manual's own worked example (section
// 5.2.2): screen 0's bitmap supplies pixels exactly as standard mode.
// Screen 1, at the same non-linear byte offset plus 0x2000 (i.e. Address
// Bit 13 set -- $4000 pairs with $6000, $47FF with $67FF, $57FF with
// $77FF), supplies one FLASH/BRIGHT/PAPER/INK attribute byte per 8x1
// pixel row instead of the standard 8x8 block.
//
// Screen 1 is not special-cased storage the way screen 0's bitmap and
// attributes are (see screen.go) -- memory.go already treats
// $6000-$77FF as ordinary bank-5 RAM, exactly like real hardware outside
// a video mode that reads it. So this renderer reads it straight through
// mem.Read(), the same way any Z80 program's own memory access would.
//
// Scope, decided 2026-08-17 (see docs/TRACKING.md T-11): this is a
// static, startup-selected rendering mode. There is no I/O port
// emulation for $FF, so nothing in a running guest program can engage
// or disengage it -- selecting it is entirely a host-side choice via
// -ns-graphics, made once before boot. Real hardware's collision
// protection for this address range (the Extension ROM's CHNG_VID
// service, which relocates OS-resident code, the machine stack, and the
// UDG table out of $6000-$77FF) has no equivalent here, because zenzx
// does not emulate the T/S 2068's system ROM at all -- there is no
// service to call. This mode is therefore opt-in for content written
// for it (a zenscript that pokes both planes deliberately, a custom
// loader), not a promise that arbitrary Sinclair BASIC programs can
// safely enable it mid-run the way real T/S 2068 software could.
//
// FLASH is honoured here, matching the ZX-Uno manual's own attribute
// description for this mode ("paper/ink/bright/flash attribute per each
// 8x1 pixels block") and this project's own docs/timex-modes.md, both
// corroborating the attribute byte's ordinary Spectrum layout. An
// earlier version of this renderer left FLASH unread as a scope
// decision that was mistakenly described in this comment as if it
// reflected real hardware behaviour -- it didn't; nothing found
// suggests real hi-colour hardware disables or ignores this bit.
// ============================================================================

type hicolourVideoRenderer struct{}

func (hicolourVideoRenderer) Name() string { return NSGraphicsTimex001HiColour }

// Dimensions: hi-colour does not change pixel resolution, only
// attribute granularity -- still 256x192.
func (hicolourVideoRenderer) Dimensions() (int, int) { return ScreenWidth, ScreenHeight }

// BorderMargins: real hardware does not fix the border in this mode
// (unlike 64-column mode), so the standard border applies unchanged.
func (hicolourVideoRenderer) BorderMargins() (int, int, int, int) {
	return BorderLeft, BorderRight, BorderTop, BorderBottom
}

// hicolourAttrBase is screen 1's base address (Address Bit 13 set
// relative to screen 0's $4000), per the Technical Manual's worked
// example: pixel byte at $4000 pairs with attribute byte at $6000.
const hicolourAttrBase = 0x6000

// Decode reads screen 0's bitmap for pixels (via screen.bitmap, the
// same as the standard renderer) and screen 1 for one attribute byte
// per 8x1 pixel row (via mem.Read, since that range is not
// screen-mirrored storage). FLASH swaps ink/paper for the row exactly
// as standard mode does for a cell, driven by the same screen.flashEnabled/
// screen.flashTickTock timer (screen.go's updateFlash) -- see the design
// comment above standardVideoRenderer in videorender.go for why that
// timer lives on SpectrumScreen rather than being renderer-specific
// state.
func (hicolourVideoRenderer) Decode(mem *SpectrumMemory, screen *SpectrumScreen) *image.RGBA {
	const w, h = ScreenWidth, ScreenHeight
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			offset := screen.calcByteOffset(x, y)
			b := screen.bitmap[offset]
			pixelOn := (b>>(7-uint(x%8)))&1 == 1

			attr := mem.Read(uint16(hicolourAttrBase + offset))
			ink := attr & 0x07
			paper := (attr >> 3) & 0x07
			bright := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			if screen.flashEnabled && flash == 1 && screen.flashTickTock {
				ink, paper = paper, ink
			}

			var idx uint8
			if pixelOn {
				idx = ink
			} else {
				idx = paper
			}
			if bright == 1 {
				idx += 8
			}
			img.SetRGBA(x, y, zxPalette[idx])
		}
	}
	return img
}

func init() {
	RegisterVideoRenderer(hicolourVideoRenderer{})
}
