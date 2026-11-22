package zenzx

import (
	"image"
	"image/color"
	"math/bits"

	"github.com/ha1tch/zenui/zxpalette"
)

// The Next's tilemap (T-44).
//
// A character display of 40x32 or 80x32 cells of 8x8 pixels, on a 320x256 surface that
// reaches 32 pixels into the border on every side (in 80 column mode the same surface has
// 640 pixels of half width). It shares the ULA's layer: the two are combined first, tile
// by tile above or below the ULA picture, and the result is the "U" that NextREG 0x15
// orders against Layer 2 and the sprites. NextZXOS uses it for the editor's 64 and 85
// column modes, and a great many programs use it.
//
// From tilemap.txt and registers.txt in the distribution (/docs/extra-hw):
//
//	0x6B  bit 7 enable, bit 6 80 columns, bit 5 no attribute byte in the map (NextREG 0x6C
//	      is used for every tile), bit 4 second palette, bit 3 text mode, bit 1 512 tiles,
//	      bit 0 tilemap always above the ULA
//	0x6C  the attribute when the map has none
//	0x6E  where the map is: bits 5-0 the high byte of its offset in bank 5; bit 7 selects
//	      bank 7, whose 8K wrap
//	0x6F  the same for the tile definitions
//	0x4C  the colour index that is transparent, bits 3-0, 0xF after a reset
//	0x1B  clip window, four writes: X1, X2, Y1, Y2; 0, 159, 0, 255 after a reset; X is
//	      doubled, and both ends are inclusive
//	0x2F, 0x30, 0x31  scroll offsets, X (10 bits) and Y; the surface wraps
//	0x68  bit 7 ULA off, bit 0 stencil mode
//
// A map entry is the tile number and then the attribute: bits 7-4 palette offset, bit 3 X
// mirror, bit 2 Y mirror, bit 1 rotate, bit 0 "ULA over this tile" (or, with 512 tiles,
// bit 8 of the tile number). A tile is 32 bytes, 4 bits a pixel, high nibble first; its
// colour index is the palette offset in the top four bits and the pixel below.
//
// In text mode a tile is 8 bytes, one bit a pixel, there is no mirroring or rotation, and
// the attribute's bits 7-1 are a 7-bit palette offset: the colour index is the offset with
// the pixel as its lowest bit. Transparency is then a matter of colour, not of index: a
// pixel whose colour is the global transparency colour (NextREG 0x14) is transparent.
// (Text mode is in registers.txt only as a bit; its rules here are as Ochenta's and the
// Next's own programmers describe them.)
//
// This file draws the part of the surface that lies over the 256x192 picture, which is
// what the headless build and DecodeDisplay show. The 32 pixels in the border are not
// drawn, and the GUI's own drawing path does not use this yet.

const (
	tilemapSurfaceHeight = 256
	tilemapBorder        = 32 // pixels of the surface outside the picture, each side
)

// tilemapState is what the tilemap needs beyond the register file.
type tilemapState struct {
	clip *nextRegClip
}

func newTilemapState() tilemapState {
	return tilemapState{clip: &nextRegClip{values: [4]uint8{0, 159, 0, 255}}}
}

func (io *SpectrumIO) tilemapControl() uint8 { return io.nextRegs.regs[0x6B] }

// tilemapEnabled reports whether the tilemap is displayed.
func (io *SpectrumIO) tilemapEnabled() bool {
	return io.memory != nil && io.memory.isNext && io.tilemapControl()&0x80 != 0
}

// tilemapWide reports the 80 column mode, whose pixels are half as wide.
func (io *SpectrumIO) tilemapWide() bool { return io.tilemapControl()&0x40 != 0 }

// tilemapByte reads the map or the tile definitions: base is NextREG 0x6E or 0x6F.
func (io *SpectrumIO) tilemapByte(base uint8, offset int) uint8 {
	addr := int(base&0x3F)<<8 + offset
	if base&0x80 != 0 { // bank 7: only its lower 8K, wrapping
		return io.memory.nextPageRead(14, uint16(addr&0x1FFF))
	}
	addr &= 0x3FFF
	return io.memory.nextPageRead(uint8(10+addr>>13), uint16(addr&0x1FFF))
}

// tilemapPixel returns the tilemap's pixel at (x, y) of its surface: x from 0 to 319, or to
// 639 in 80 column mode, y from 0 to 255. below says the ULA goes over it.
func (io *SpectrumIO) tilemapPixel(x, y int) (c color.RGBA, opaque, below bool) {
	ctrl := io.tilemapControl()
	cols, scale := 40, 1
	if ctrl&0x40 != 0 {
		cols, scale = 80, 2
	}
	clip := io.tilemap.clip.values
	if cx := x / scale; cx < int(clip[0])*2 || cx > int(clip[1])*2+1 || y < int(clip[2]) || y > int(clip[3]) {
		return color.RGBA{}, false, false
	}
	width := cols * 8
	sx := (x + int(io.nextRegs.regs[0x2F]&0x03)<<8 + int(io.nextRegs.regs[0x30])) % width
	sy := (y + int(io.nextRegs.regs[0x31])) % tilemapSurfaceHeight

	cell := (sy/8)*cols + sx/8
	var tile int
	var attr uint8
	if ctrl&0x20 != 0 {
		tile = int(io.tilemapByte(io.nextRegs.regs[0x6E], cell))
		attr = io.nextRegs.regs[0x6C]
	} else {
		tile = int(io.tilemapByte(io.nextRegs.regs[0x6E], cell*2))
		attr = io.tilemapByte(io.nextRegs.regs[0x6E], cell*2+1)
	}
	if ctrl&0x02 != 0 { // 512 tiles: the attribute's bit 0 is bit 8 of the tile number
		tile |= int(attr&0x01) << 8
	} else {
		below = attr&0x01 != 0 && ctrl&0x01 == 0
	}

	palette := nextPaletteTilemap1st
	if ctrl&0x10 != 0 {
		palette = nextPaletteTilemap2nd
	}
	px, py := sx&7, sy&7
	if ctrl&0x08 != 0 { // text mode
		bit := io.tilemapByte(io.nextRegs.regs[0x6F], tile*8+py) >> (7 - uint(px)) & 0x01
		rgb := io.nextPalette.palettes[palette][attr&0xFE|bit]
		if zxpalette.EncodeRGB9To8Bit(rgb) == io.nextRegs.regs[0x14] {
			return color.RGBA{}, false, below
		}
		return nrgbaToRGBA(rgb.Colour()), true, below
	}

	// Mirroring and rotation as for the sprites: rotation swaps the axes and takes the X
	// mirror with it. The direction of the rotation has not been checked against hardware.
	if (attr&0x08 != 0) != (attr&0x02 != 0) {
		px = 7 - px
	}
	if attr&0x04 != 0 {
		py = 7 - py
	}
	if attr&0x02 != 0 {
		px, py = py, px
	}
	b := io.tilemapByte(io.nextRegs.regs[0x6F], tile*32+py*4+px/2)
	nibble := b >> 4
	if px&1 != 0 {
		nibble = b & 0x0F
	}
	if nibble == io.nextRegs.regs[0x4C]&0x0F {
		return color.RGBA{}, false, below
	}
	return nrgbaToRGBA(io.nextPalette.palettes[palette][attr&0xF0|nibble].Colour()), true, below
}

func nrgbaToRGBA(c color.NRGBA) color.RGBA {
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xFF}
}

// CombineULAAndTilemap makes the "U" layer from the ULA picture (256x192) and the tilemap.
// With the tilemap off it returns the ULA picture itself. Otherwise the result is 256
// pixels wide, or 512 in 80 column mode, where each ULA pixel covers two.
//
// In the ordinary mode the result is the topmost pixel that is not transparent: the tile's
// where the tile is above the ULA, the ULA's where the tile says the ULA goes over it. A
// ULA pixel is transparent when its colour in the Next's ULA palette is the global
// transparency colour (NextREG 0x14); programs make a paper colour transparent that way
// and print over the tilemap. In stencil mode, NextREG 0x68 bit 0 with both enabled, a
// pixel is the AND of the two colours, and transparent where either is. Where nothing is
// left the fallback colour (NextREG 0x4A) shows.
func (io *SpectrumIO) CombineULAAndTilemap(ula *image.RGBA, screen *SpectrumScreen) *image.RGBA {
	if !io.tilemapEnabled() {
		return ula
	}
	// With the standard 256x192 picture the ULA's pixels are taken from the display file
	// through the Next's ULA palette, which is what makes one of them transparent.
	fromScreen := screen != nil && io.nextTimexMode() == 0 && ula.Rect.Dx() == ScreenWidth && ula.Rect.Dy() == ScreenHeight
	// The result is as wide as the wider of the two: 512 when the tilemap is in 80 column
	// mode or the ULA picture is a hi-res one.
	tmScale := 1
	if io.tilemapWide() {
		tmScale = 2
	}
	w, h := ScreenWidth*tmScale, ula.Rect.Dy()
	if ula.Rect.Dx() > w {
		w = ula.Rect.Dx()
	}
	ulaStep := w / ula.Rect.Dx()          // output pixels to a ULA pixel
	tmStep := w / (ScreenWidth * tmScale) // output pixels to a tilemap pixel
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	ulaOff := io.nextRegs.regs[0x68]&0x80 != 0
	stencil := io.nextRegs.regs[0x68]&0x01 != 0 && !ulaOff
	fallback := nrgbaToRGBA(zxpalette.DecodeRGB9From8Bit(io.nextRegs.regs[0x4A]).Colour())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u, uOpaque := ula.RGBAAt(ula.Rect.Min.X+x/ulaStep, ula.Rect.Min.Y+y), !ulaOff
			if uOpaque && fromScreen {
				u, uOpaque = io.ulaPalettePixel(screen, x/ulaStep, y)
			}
			t, opaque, below := io.tilemapPixel(x/tmStep+tilemapBorder*tmScale, y+tilemapBorder)
			var c color.RGBA
			switch {
			case stencil && opaque && uOpaque:
				c = color.RGBA{R: u.R & t.R, G: u.G & t.G, B: u.B & t.B, A: 0xFF}
			case stencil:
				c = fallback
			case opaque && (!below || !uOpaque):
				c = t
			case uOpaque:
				c = u
			default:
				c = fallback
			}
			out.SetRGBA(x, y, c)
		}
	}
	return out
}

// widenToMatch doubles a layer's pixels horizontally when the U layer is 512 wide, so that
// Layer 2 and the sprites line up with it.
func widenToMatch(layer *image.RGBA, width int) *image.RGBA {
	if layer == nil || layer.Rect.Dx() == width || layer.Rect.Dx()*2 != width {
		return layer
	}
	out := image.NewRGBA(image.Rect(0, 0, width, layer.Rect.Dy()))
	for y := 0; y < layer.Rect.Dy(); y++ {
		for x := 0; x < width; x++ {
			out.SetRGBA(x, y, layer.RGBAAt(layer.Rect.Min.X+x/2, layer.Rect.Min.Y+y))
		}
	}
	return out
}

// ulaPalettePixel returns the ULA's pixel at (x, y) of the 256x192 picture as the Next shows
// it: the display file's ink or paper looked up in the ULA palette on display (inks at
// entries 0-7, bright inks 8-15, papers 16-23, bright papers 24-31), and whether it is
// opaque, which it is unless its colour is the global transparency colour.
func (io *SpectrumIO) ulaPalettePixel(screen *SpectrumScreen, x, y int) (color.RGBA, bool) {
	b := screen.bitmap[screen.calcByteOffset(x, y)]
	attr := screen.attributes[(y/8)*32+x/8]
	on := (b>>(7-uint(x%8)))&1 == 1
	if screen.flashEnabled && attr&0x80 != 0 && screen.flashTickTock {
		on = !on
	}
	if io.nextPalette.ulaNextEnabled {
		// ULANext (NextREG 0x43 bit 0): NextREG 0x42 is a mask of the attribute bits that
		// are ink, 1 to 255 in solid runs from the right; the rest are paper. Inks are
		// palette entries from 0, papers from 128. There is no FLASH or BRIGHT. With all
		// eight bits ink there is no paper: it is the fallback colour.
		mask := io.nextRegs.regs[0x42]
		if (b>>(7-uint(x%8)))&1 == 1 {
			rgb := io.nextPalette.lookup(nextLayerULA, attr&mask)
			return nrgbaToRGBA(rgb.Colour()), zxpalette.EncodeRGB9To8Bit(rgb) != io.nextRegs.regs[0x14]
		}
		if mask == 0xFF {
			return color.RGBA{}, false
		}
		rgb := io.nextPalette.lookup(nextLayerULA, 128+(attr&^mask)>>uint(bits.OnesCount8(mask)))
		return nrgbaToRGBA(rgb.Colour()), zxpalette.EncodeRGB9To8Bit(rgb) != io.nextRegs.regs[0x14]
	}
	index := attr & 0x07
	if !on {
		index = 16 + (attr>>3)&0x07
	}
	if attr&0x40 != 0 {
		index += 8
	}
	rgb := io.nextPalette.lookup(nextLayerULA, index)
	return nrgbaToRGBA(rgb.Colour()), zxpalette.EncodeRGB9To8Bit(rgb) != io.nextRegs.regs[0x14]
}
