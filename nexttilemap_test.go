package zenzx

import (
	"image"
	"image/color"
	"testing"

	"github.com/ha1tch/zenui/zxpalette"
)

// newTilemapTestIO builds a Next with the tilemap at its reset addresses (map at 0x6C00,
// tile definitions at 0x4C00, both in bank 5) and returns helpers to poke them.
func newTilemapTestIO(t *testing.T) (*SpectrumIO, func(offset int, v uint8), func(offset int, v uint8)) {
	t.Helper()
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	bank5 := func(addr int, v uint8) { mem.nextPageWrite(uint8(10+addr>>13), uint16(addr&0x1FFF), v) }
	return io, func(o int, v uint8) { bank5(0x2C00+o, v) }, func(o int, v uint8) { bank5(0x0C00+o, v) }
}

func tilemapColour(v uint8) color.RGBA {
	return nrgbaToRGBA(zxpalette.DecodeRGB9From8Bit(v).Colour())
}

func TestTilemapOffByDefaultAndResetRegisters(t *testing.T) {
	io, _, _ := newTilemapTestIO(t)
	if io.tilemapEnabled() {
		t.Fatal("the tilemap is on after a reset")
	}
	for reg, want := range map[uint8]uint8{0x6E: 0x6C, 0x6F: 0x4C, 0x4C: 0x0F, 0x14: 0xE3, 0x4A: 0xE3} {
		if got := io.GetNextRegister(reg); got != want {
			t.Errorf("NextREG 0x%02X resets to 0x%02X, want 0x%02X", reg, got, want)
		}
	}
	if io.tilemap.clip.values != [4]uint8{0, 159, 0, 255} {
		t.Errorf("clip window resets to %v, want [0 159 0 255]", io.tilemap.clip.values)
	}
	ula := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	if io.CombineULAAndTilemap(ula, nil) != ula {
		t.Error("with the tilemap off the ULA picture must be returned untouched")
	}
}

func TestTilemapGraphicTile(t *testing.T) {
	io, poke, def := newTilemapTestIO(t)
	io.nextRegWriteDirect(0x6B, 0x80) // on, 40 columns, two-byte entries

	// Tile 5: left half colour 1, right half colour 2, with the pixel at (0, 0) transparent
	// (index 0xF).
	for row := 0; row < 8; row++ {
		def(5*32+row*4+0, 0x11)
		def(5*32+row*4+1, 0x11)
		def(5*32+row*4+2, 0x22)
		def(5*32+row*4+3, 0x22)
	}
	def(5*32, 0xF1)
	// Cell (column 6, row 7): tile 5, palette offset 3.
	cell := 7*40 + 6
	poke(cell*2, 5)
	poke(cell*2+1, 0x30)

	x0, y0 := 6*8, 7*8
	if _, opaque, _ := io.tilemapPixel(x0, y0); opaque {
		t.Error("the pixel with the transparency index is opaque")
	}
	c, opaque, below := io.tilemapPixel(x0+1, y0)
	if !opaque || below || c != tilemapColour(0x31) {
		t.Errorf("pixel (1,0): %v opaque=%v below=%v, want palette entry 0x31", c, opaque, below)
	}
	if c, _, _ := io.tilemapPixel(x0+5, y0+7); c != tilemapColour(0x32) {
		t.Errorf("pixel (5,7): %v, want palette entry 0x32", c)
	}

	// X mirror puts colour 2 on the left.
	poke(cell*2+1, 0x38)
	if c, _, _ := io.tilemapPixel(x0+1, y0); c != tilemapColour(0x32) {
		t.Errorf("X-mirrored pixel (1,0): %v, want palette entry 0x32", c)
	}
	// Bit 0: the ULA goes over this tile, unless NextREG 0x6B bit 0 forces the tilemap up.
	poke(cell*2+1, 0x31)
	if _, _, below := io.tilemapPixel(x0+1, y0); !below {
		t.Error("attribute bit 0 did not put the tile below the ULA")
	}
	io.nextRegWriteDirect(0x6B, 0x81)
	if _, _, below := io.tilemapPixel(x0+1, y0); below {
		t.Error("NextREG 0x6B bit 0 did not force the tilemap above the ULA")
	}
	// 512 tiles: bit 0 is bit 8 of the tile number instead. Tile 261 is not defined.
	io.nextRegWriteDirect(0x6B, 0x82)
	if c, opaque, below := io.tilemapPixel(x0+1, y0); below || !opaque || c != tilemapColour(0x30) {
		t.Errorf("512-tile mode: %v opaque=%v below=%v, want tile 261 (all zero: palette entry 0x30)", c, opaque, below)
	}

	// The second palette.
	io.nextPalette.palettes[nextPaletteTilemap2nd][0x31] = zxpalette.DecodeRGB9From8Bit(0xE0)
	io.nextRegWriteDirect(0x6B, 0x90)
	poke(cell*2+1, 0x30)
	if c, _, _ := io.tilemapPixel(x0+1, y0); c != tilemapColour(0xE0) {
		t.Errorf("second palette: %v, want the red written to its entry 0x31", c)
	}
}

func TestTilemapOneByteEntriesAndBank7(t *testing.T) {
	io, poke, def := newTilemapTestIO(t)
	io.nextRegWriteDirect(0x6B, 0xA0) // on, no attribute byte in the map
	io.nextRegWriteDirect(0x6C, 0x40) // palette offset 4 for every tile
	def(9*32, 0x30)
	poke(3, 9) // cell 3 of row 0 is one byte into the map
	if c, _, _ := io.tilemapPixel(3*8, 0); c != tilemapColour(0x43) {
		t.Errorf("one-byte entry: %v, want palette entry 0x43", c)
	}
	// The map in bank 7 (bit 7 of NextREG 0x6E), which wraps at 8K.
	io.nextRegWriteDirect(0x6E, 0x80|0x1F) // offset 0x1F00 in bank 7
	io.memory.nextPageWrite(14, 0x1F00+2, 9)
	io.memory.nextPageWrite(14, 0x0004, 9) // cell 260 = 0x1F00 + 260 wraps to 0x0004
	if c, _, _ := io.tilemapPixel(2*8, 0); c != tilemapColour(0x43) {
		t.Errorf("map in bank 7: %v, want palette entry 0x43", c)
	}
	if c, _, _ := io.tilemapPixel((260%40)*8, (260/40)*8); c != tilemapColour(0x43) {
		t.Errorf("map in bank 7 past its 8K: %v, want the wrapped cell", c)
	}
}

func TestTilemapTextMode(t *testing.T) {
	io, poke, def := newTilemapTestIO(t)
	io.nextRegWriteDirect(0x6B, 0xC8) // on, 80 columns, text mode
	// Tile 0x41: top row 0b10100000.
	def(0x41*8, 0xA0)
	cell := 2*80 + 10
	poke(cell*2, 0x41)
	poke(cell*2+1, 0x24) // 7-bit palette offset 0x12: entries 0x24 (paper) and 0x25 (ink)
	io.nextPalette.palettes[nextPaletteTilemap1st][0x24] = zxpalette.DecodeRGB9From8Bit(0x03)
	io.nextPalette.palettes[nextPaletteTilemap1st][0x25] = zxpalette.DecodeRGB9From8Bit(0xFC)

	x0, y0 := 10*8, 2*8
	for px, want := range []uint8{0xFC, 0x03, 0xFC, 0x03} {
		if c, opaque, _ := io.tilemapPixel(x0+px, y0); !opaque || c != tilemapColour(want) {
			t.Errorf("text pixel %d: %v opaque=%v, want colour 0x%02X", px, c, opaque, want)
		}
	}
	// A colour equal to the global transparency colour is transparent.
	io.nextPalette.palettes[nextPaletteTilemap1st][0x24] = zxpalette.DecodeRGB9From8Bit(0xE3)
	if _, opaque, _ := io.tilemapPixel(x0+1, y0); opaque {
		t.Error("a text pixel in the global transparency colour is opaque")
	}
	if _, opaque, _ := io.tilemapPixel(x0, y0); !opaque {
		t.Error("the ink pixel next to it became transparent")
	}
}

func TestTilemapClipAndScroll(t *testing.T) {
	io, poke, def := newTilemapTestIO(t)
	io.nextRegWriteDirect(0x6B, 0x80)
	for i := 0; i < 32; i++ {
		def(1*32+i, 0x55)
	}
	poke(0, 1) // cell (0,0) is tile 1, solid colour 5
	if c, opaque, _ := io.tilemapPixel(3, 3); !opaque || c != tilemapColour(0x05) {
		t.Fatalf("unscrolled: %v opaque=%v", c, opaque)
	}
	// Scrolled 16 right and 8 down, the surface wraps: cell (0,0) is found at the far end.
	io.nextRegWriteDirect(0x30, 16)
	io.nextRegWriteDirect(0x31, 8)
	if c, _, _ := io.tilemapPixel(320-16+3, 256-8+3); c != tilemapColour(0x05) {
		t.Errorf("scrolled: %v, want the tile from cell (0,0)", c)
	}
	io.nextRegWriteDirect(0x30, 0)
	io.nextRegWriteDirect(0x31, 0)

	// Clip to X 1..2 (pixels 2..5) and Y 2..3.
	for _, v := range []uint8{1, 2, 2, 3} {
		io.nextRegWriteDirect(0x1B, v)
	}
	for _, c := range []struct {
		x, y int
		in   bool
	}{{1, 2, false}, {2, 2, true}, {5, 3, true}, {6, 3, false}, {3, 1, false}, {3, 4, false}} {
		if _, opaque, _ := io.tilemapPixel(c.x, c.y); opaque != c.in {
			t.Errorf("clip: pixel (%d,%d) opaque=%v, want %v", c.x, c.y, opaque, c.in)
		}
	}
	// NextREG 0x1C bit 3 restarts the four-write sequence.
	io.nextRegWriteDirect(0x1B, 9)
	io.nextRegWriteDirect(0x1C, 0x08)
	io.nextRegWriteDirect(0x1B, 0)
	if io.tilemap.clip.values[0] != 0 {
		t.Errorf("after the reset of the sequence the write went to index %d", io.tilemap.clip.writeIndex)
	}
}

func TestCombineULAAndTilemap(t *testing.T) {
	io, poke, def := newTilemapTestIO(t)
	screen := io.memory.screen
	io.nextRegWriteDirect(0x6B, 0x80)
	for i := 0; i < 32; i++ {
		def(1*32+i, 0x55) // tile 1: solid colour 5
	}
	// The picture's top left corner is cell (4, 4) of the surface.
	corner := (4*40 + 4) * 2
	poke(corner, 1)
	// The ULA there: white ink on blue paper, top-left pixel set.
	screen.bitmap[0] = 0x80
	screen.attributes[0] = 0x0F
	ula := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))

	tile, ink, paper := tilemapColour(0x05), tilemapColour(0xB6), tilemapColour(0x02)

	out := io.CombineULAAndTilemap(ula, screen)
	if out.Rect.Dx() != 256 || out.RGBAAt(0, 0) != tile || out.RGBAAt(1, 0) != tile {
		t.Fatalf("tile above the ULA: %dx, (0,0)=%v (1,0)=%v, want the tile", out.Rect.Dx(), out.RGBAAt(0, 0), out.RGBAAt(1, 0))
	}
	if got := out.RGBAAt(8, 0); got != tilemapColour(0x00) {
		// The next cell's tile is tile 0, all zero pixels: palette entry 0, black.
		t.Fatalf("the neighbouring cell shows %v, want tile 0's black", got)
	}

	// The ULA over the tile: ink and paper from the ULA palette.
	poke(corner+1, 0x01)
	out = io.CombineULAAndTilemap(ula, screen)
	if out.RGBAAt(0, 0) != ink || out.RGBAAt(1, 0) != paper {
		t.Fatalf("ULA over the tile: (0,0)=%v (1,0)=%v, want ink %v and paper %v", out.RGBAAt(0, 0), out.RGBAAt(1, 0), ink, paper)
	}
	// Make blue paper the transparent colour: the tile shows through it, the ink stays.
	io.nextPalette.palettes[nextPaletteULA1st][16+1] = zxpalette.DecodeRGB9From8Bit(0xE3)
	out = io.CombineULAAndTilemap(ula, screen)
	if out.RGBAAt(0, 0) != ink || out.RGBAAt(1, 0) != tile {
		t.Fatalf("transparent paper: (0,0)=%v (1,0)=%v, want ink and the tile", out.RGBAAt(0, 0), out.RGBAAt(1, 0))
	}

	// ULA off: only the tilemap, and the fallback colour where it is transparent.
	io.nextRegWriteDirect(0x68, 0x80)
	io.nextRegWriteDirect(0x4A, 0x1C)
	io.nextRegWriteDirect(0x4C, 0x05) // colour 5 is now the transparent index
	out = io.CombineULAAndTilemap(ula, screen)
	if got := out.RGBAAt(0, 0); got != tilemapColour(0x1C) {
		t.Fatalf("ULA off and the tile transparent: %v, want the fallback colour", got)
	}
	io.nextRegWriteDirect(0x68, 0x00)
	io.nextRegWriteDirect(0x4C, 0x0F)

	// Stencil mode: the AND of the two where both are opaque.
	io.nextPalette.palettes[nextPaletteULA1st][16+1] = zxpalette.DecodeRGB9From8Bit(0x02)
	io.nextRegWriteDirect(0x68, 0x01)
	out = io.CombineULAAndTilemap(ula, screen)
	want := color.RGBA{R: ink.R & tile.R, G: ink.G & tile.G, B: ink.B & tile.B, A: 0xFF}
	if got := out.RGBAAt(0, 0); got != want {
		t.Fatalf("stencil: %v, want %v", got, want)
	}
	io.nextRegWriteDirect(0x68, 0x00)

	// 80 columns: the result is 512 wide and each ULA pixel covers two.
	io.nextRegWriteDirect(0x6B, 0xC0)
	out = io.CombineULAAndTilemap(ula, screen)
	if out.Rect.Dx() != 512 || out.Rect.Dy() != 192 {
		t.Fatalf("80 column result is %dx%d, want 512x192", out.Rect.Dx(), out.Rect.Dy())
	}
	layer := image.NewRGBA(image.Rect(0, 0, 256, 192))
	layer.SetRGBA(3, 0, color.RGBA{R: 9, A: 0xFF})
	wide := widenToMatch(layer, 512)
	if wide.Rect.Dx() != 512 || wide.RGBAAt(6, 0).R != 9 || wide.RGBAAt(7, 0).R != 9 || wide.RGBAAt(8, 0).R != 0 {
		t.Fatal("widenToMatch did not double the layer's pixels")
	}
	if widenToMatch(layer, 256) != layer || widenToMatch(nil, 512) != nil {
		t.Fatal("widenToMatch changed a layer that already fits, or a missing one")
	}
}
