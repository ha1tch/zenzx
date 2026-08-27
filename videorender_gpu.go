//go:build !headless

package main

import (
	"image"
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Compile-time checks that both fast-path renderers genuinely satisfy
// FastGUIRenderer -- catches an accidental signature drift immediately,
// rather than only discovering it via the runtime type assertion in
// Render silently falling back to Decode().
var (
	_ FastGUIRenderer = standardVideoRenderer{}
	_ FastGUIRenderer = hicolourVideoRenderer{}
)

// FastGUIRenderer is an optional interface a VideoRenderer can also
// implement for GPU-accelerated live rendering via raylib texture
// blitting, used by DisplayManager.Render instead of Decode()+texture-
// upload when available. A renderer without this interface -- or the
// headless build, which has no GL context to blit into -- uses Decode()
// exactly as before; nothing about that portable path changes.
//
// RenderGPU draws directly to whatever render target is currently bound
// (the main window, already cleared and with the border texture drawn if
// applicable by the time Render calls this) -- no intermediate texture,
// matching how this renderer worked before the VideoRenderer abstraction
// existed. offsetX/offsetY is where the screen area begins (0,0 if
// borderless, or just past the border if not); multiplier is the fixed
// integer pixel scale (dm.screen.multiplier) -- both pkg-textures
// (bitPatternGPU, paperColourGPU) are baked once and never re-uploaded;
// only draw calls happen per frame.
type FastGUIRenderer interface {
	RenderGPU(dm *DisplayManager, mem *SpectrumMemory, screen *SpectrumScreen, offsetX, offsetY float32, multiplier int)
}

// generateGPUTextures bakes the two shared texture caches every
// FastGUIRenderer draws from: bitPatternGPU (256 textures, one per
// possible bitmap byte value -- an 8x1 mask of which pixels are "on",
// tinted per-draw to whatever colour a given cell/row needs) and
// paperColourGPU (16 textures, one solid-colour block per palette entry).
// Both are universal concepts, not renderer-specific, so every
// FastGUIRenderer implementation shares this one bake rather than each
// generating its own. Call once, after the window exists (raylib texture
// upload needs a live GL context) -- InitializeAfterWindow does this.
func (dm *DisplayManager) generateGPUTextures() {
	for bitPattern := 0; bitPattern < 256; bitPattern++ {
		img := image.NewRGBA(image.Rect(0, 0, 8, 1))
		for bit := 0; bit < 8; bit++ {
			on := (bitPattern>>(7-bit))&1 == 1
			var c color.RGBA
			if on {
				c = color.RGBA{0xff, 0xff, 0xff, 0xff} // opaque: tinted by draw colour
			} else {
				c = color.RGBA{0, 0, 0, 0} // transparent: paper shows through
			}
			img.Set(bit, 0, c)
		}
		dm.bitPatternGPU[bitPattern] = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
	}

	for colorIndex := 0; colorIndex < 16; colorIndex++ {
		c := zxPalette[colorIndex]
		img := image.NewRGBA(image.Rect(0, 0, 8, 1))
		for x := 0; x < 8; x++ {
			img.Set(x, 0, color.RGBA{c.R, c.G, c.B, c.A})
		}
		dm.paperColourGPU[colorIndex] = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
	}

	dm.gpuTexturesReady = true
}

// RenderGPU is standardVideoRenderer's fast path: two passes over the
// 24x32 attribute grid, one texture-blit per cell per pass, all pixel
// compositing done by the GPU via tinted texture draws rather than a
// CPU-side image.RGBA built pixel by pixel. Restores the technique this
// renderer used before the VideoRenderer abstraction (needed for
// hi-colour and headless screenshot support) replaced it with the
// portable Decode() path -- Decode() is unchanged and still used by
// headless and by any renderer without a fast path; this is purely an
// additional, faster path for the GUI's live rendering of the specific
// case this renderer covers.
func (standardVideoRenderer) RenderGPU(dm *DisplayManager, mem *SpectrumMemory, screen *SpectrumScreen, offsetX, offsetY float32, multiplier int) {
	mult := float32(multiplier)

	// Paper pass: one 8x8 solid-colour block per attribute cell.
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			attr := screen.attributes[row*32+col]
			paper := (attr >> 3) & 0x07
			bright := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			colourIndex := paper | (bright << 3)
			if screen.flashEnabled && flash == 1 && screen.flashTickTock {
				ink := attr & 0x07
				colourIndex = ink | (bright << 3)
			}

			pos := rl.NewVector2(offsetX+float32(col*8)*mult, offsetY+float32(row*8)*mult)
			rl.DrawTextureEx(dm.paperColourGPU[colourIndex], pos, 0, mult*8, rl.White)
		}
	}

	// Ink pass: one 8x1 bit-pattern blit per bitmap byte (8 per cell),
	// tinted to that cell's ink (or paper, mid-flash) colour.
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			attr := screen.attributes[row*32+col]
			ink := attr & 0x07
			bright := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			var tint rl.Color
			if screen.flashEnabled && flash == 1 && screen.flashTickTock {
				paper := (attr >> 3) & 0x07
				tint = zxPalette[paper|(bright<<3)]
			} else {
				tint = zxPalette[ink|(bright<<3)]
			}

			for y := 0; y < 8; y++ {
				off := screen.calcByteOffset(col*8, row*8+y)
				byteValue := screen.bitmap[off]
				pos := rl.NewVector2(offsetX+float32(col*8)*mult, offsetY+float32(row*8+y)*mult)
				rl.DrawTextureEx(dm.bitPatternGPU[byteValue], pos, 0, mult, tint)
			}
		}
	}
}

// RenderGPU is hicolourVideoRenderer's fast path -- same shared textures
// and the same two-pass shape as the standard renderer, but at the
// per-byte (8x1) attribute granularity hi-colour actually has, rather
// than standard's per-cell (8x8) granularity. FLASH is deliberately
// unread here too, matching Decode()'s own documented design (see
// videorender_hicolour.go's package doc) -- this is a faster path to the
// exact same pixels Decode() already produces, not a behaviour change.
func (hicolourVideoRenderer) RenderGPU(dm *DisplayManager, mem *SpectrumMemory, screen *SpectrumScreen, offsetX, offsetY float32, multiplier int) {
	mult := float32(multiplier)

	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			for y := 0; y < 8; y++ {
				off := screen.calcByteOffset(col*8, row*8+y)
				attr := mem.Read(uint16(hicolourAttrBase + off))
				ink := attr & 0x07
				paper := (attr >> 3) & 0x07
				bright := (attr >> 6) & 0x01
				flash := (attr >> 7) & 0x01

				if screen.flashEnabled && flash == 1 && screen.flashTickTock {
					ink, paper = paper, ink
				}

				px := offsetX + float32(col*8)*mult
				py := offsetY + float32(row*8+y)*mult

				rl.DrawTextureEx(dm.paperColourGPU[paper|(bright<<3)],
					rl.NewVector2(px, py), 0, mult*8, rl.White)

				byteValue := screen.bitmap[off]
				tint := zxPalette[ink|(bright<<3)]
				rl.DrawTextureEx(dm.bitPatternGPU[byteValue],
					rl.NewVector2(px, py), 0, mult, tint)
			}
		}
	}
}
