package zenzx

import (
	"image"
	"image/color"
	"testing"
)

// The Next's composed frame (nextframe.go): where each layer sits in the 320x256 picture,
// what its clip window and scroll do, and how the layers go over one another.

// frameTest is a Next with a blank classic picture and helpers to drive its layers.
type frameTest struct {
	t      *testing.T
	io     *SpectrumIO
	mem    *SpectrumMemory
	border uint8
}

func newFrameTest(t *testing.T) *frameTest {
	t.Helper()
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	return &frameTest{t: t, io: NewSpectrumIO(mem, nil), mem: mem}
}

// frame composes the picture over an all-black classic one.
func (f *frameTest) frame() *image.RGBA {
	standard := func() *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, ScreenWidth, ScreenHeight))
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 0xFF
		}
		return img
	}
	return f.io.ComposeNextFrame(standard, nil, f.border)
}

// palette sets one entry of a palette to an 8-bit colour. target is NextREG 0x43's bits
// 6-4: 1 for Layer 2's first palette, 2 for the sprites', 3 for the tilemap's.
func (f *frameTest) palette(target, index, colour uint8) {
	f.io.nextRegWriteDirect(0x43, target<<4)
	f.io.nextRegWriteDirect(0x40, index)
	f.io.nextRegWriteDirect(0x41, colour)
}

// layer2Byte writes a byte of the Layer 2 picture, which starts at bank 8: page 16.
func (f *frameTest) layer2Byte(offset int, v uint8) {
	f.mem.nextPageWrite(uint8(16+offset/8192), uint16(offset%8192), v)
}

func (f *frameTest) clip(reg uint8, x1, x2, y1, y2 uint8) {
	f.io.nextRegWriteDirect(0x1C, 0x0F) // start every window's sequence again
	for _, v := range []uint8{x1, x2, y1, y2} {
		f.io.nextRegWriteDirect(reg, v)
	}
}

func (f *frameTest) want(img *image.RGBA, x, y int, want color.RGBA, what string) {
	f.t.Helper()
	if got := img.RGBAAt(x, y); got != want {
		f.t.Errorf("%s: frame pixel (%d,%d) is %v, want %v", what, x, y, got, want)
	}
}

var (
	frameRed   = tilemapColour(0xE0)
	frameGreen = tilemapColour(0x1C)
	frameBlue  = tilemapColour(0x03)
	frameBlack = color.RGBA{A: 0xFF}
)

func TestNextFrameIsForEveryNext(t *testing.T) {
	f := newFrameTest(t)
	if !f.io.nextFrameActive() {
		t.Fatal("a Next does not ask for the frame")
	}
	if img := f.frame(); img.Rect.Dx() != 320 || img.Rect.Dy() != 256 {
		t.Fatalf("a Next with none of its layers on gives %v, want the 320x256 frame", img.Rect)
	}
	for name, turnOn := range map[string]func(){
		"Layer 2": func() { f.io.nextRegWriteDirect(0x69, 0x80) },
		"sprites": func() { f.io.nextRegWriteDirect(0x15, 0x01) },
		"tilemap": func() { f.io.nextRegWriteDirect(0x6B, 0x80) },
	} {
		f.io.nextRegWriteDirect(0x69, 0x00)
		f.io.nextRegWriteDirect(0x15, 0x00)
		f.io.nextRegWriteDirect(0x6B, 0x00)
		turnOn()
		if img := f.frame(); img.Rect.Dx() != 320 || img.Rect.Dy() != 256 {
			t.Errorf("with %s on the frame is %v, want 320x256", name, img.Rect)
		}
	}

	// A classic machine never does, whatever is in the registers.
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	io := NewSpectrumIO(mem, nil)
	io.nextRegs.regs[0x69], io.nextRegs.regs[0x15], io.nextRegs.regs[0x6B] = 0x80, 0x01, 0x80
	if io.nextFrameActive() {
		t.Fatal("a 128K asks for the Next's frame")
	}
}

func TestNextFrameClassicPictureAndBorder(t *testing.T) {
	f := newFrameTest(t)
	f.io.nextRegWriteDirect(0x69, 0x80) // Layer 2 on, its picture all transparent...
	f.palette(1, 0x00, 0xE3)            // ...because colour 0 is the transparency colour
	f.border = 2                        // red
	img := f.frame()
	red := nrgbaToRGBA(f.io.nextPalette.lookup(nextLayerULA, 16+2).Colour())
	for _, p := range [][2]int{{0, 0}, {31, 31}, {319, 255}, {288, 100}, {100, 224}, {100, 31}} {
		f.want(img, p[0], p[1], red, "border")
	}
	for _, p := range [][2]int{{32, 32}, {287, 223}, {160, 128}} {
		f.want(img, p[0], p[1], frameBlack, "classic picture")
	}
}

func TestNextFrameLayer2At256x192(t *testing.T) {
	f := newFrameTest(t)
	f.io.nextRegWriteDirect(0x69, 0x80)
	f.palette(1, 0x00, 0xE3) // the rest of the picture is transparent
	f.palette(1, 0x01, 0xE0)
	f.palette(1, 0x02, 0x1C)
	f.layer2Byte(0, 1)           // (0,0) red
	f.layer2Byte(191*256+255, 2) // (255,191) green
	f.layer2Byte(1, 2)           // (1,0) green
	img := f.frame()
	f.want(img, 32, 32, frameRed, "Layer 2 (0,0)")
	f.want(img, 287, 223, frameGreen, "Layer 2 (255,191)")
	f.want(img, 31, 32, nrgbaToRGBA(f.io.nextPalette.lookup(nextLayerULA, 16).Colour()), "left of Layer 2: the border")

	// Scrolling by one pixel brings (1,0) to the corner, and (0,0) round to the far end.
	f.io.nextRegWriteDirect(0x16, 1)
	img = f.frame()
	f.want(img, 32, 32, frameGreen, "scrolled X by 1")
	f.want(img, 287, 32, frameRed, "scrolled X by 1, wrapped")
	f.io.nextRegWriteDirect(0x16, 0)
	f.io.nextRegWriteDirect(0x17, 191)
	f.want(f.frame(), 32, 33, frameRed, "scrolled Y by 191: row 0 one line down")
	f.io.nextRegWriteDirect(0x17, 0)

	// The clip window is in the classic picture's coordinates.
	f.clip(0x18, 1, 255, 0, 191)
	img = f.frame()
	f.want(img, 32, 32, frameBlack, "clipped column 0: the ULA shows")
	f.want(img, 33, 32, frameGreen, "column 1, inside the clip window")
}

func TestNextFrameLayer2At320x256(t *testing.T) {
	f := newFrameTest(t)
	f.io.nextRegWriteDirect(0x69, 0x80)
	f.io.nextRegWriteDirect(0x70, 0x10)
	f.palette(1, 0x00, 0xE3)
	f.palette(1, 0x01, 0xE0)
	f.palette(1, 0x02, 0x1C)
	f.palette(1, 0x03, 0x03)
	// Columns come first in memory: the byte of (x,y) is x*256+y.
	f.layer2Byte(0, 1)           // (0,0)
	f.layer2Byte(319*256+255, 2) // (319,255)
	f.layer2Byte(160*256+10, 3)  // (160,10)
	f.clip(0x18, 0, 159, 0, 255) // what the firmware sets: the whole frame, X in units of two
	img := f.frame()
	if img.Rect.Dx() != 320 {
		t.Fatalf("frame is %d wide in the 320x256 mode", img.Rect.Dx())
	}
	f.want(img, 0, 0, frameRed, "320x256 (0,0): the frame's corner, in the border")
	f.want(img, 319, 255, frameGreen, "320x256 (319,255)")
	f.want(img, 160, 10, frameBlue, "320x256 (160,10): the right half")

	// The reset clip window, 0-255 by 0-191, is 512 pixels across and stops at line 191.
	f.clip(0x18, 0, 255, 0, 191)
	img = f.frame()
	f.want(img, 160, 10, frameBlue, "reset clip: the right half still shows")
	if img.RGBAAt(319, 255) == frameGreen {
		t.Error("reset clip: line 255 shows, though the window ends at 191")
	}
	// Half the width: X2 = 79 is pixel 159.
	f.clip(0x18, 0, 79, 0, 255)
	img = f.frame()
	f.want(img, 0, 0, frameRed, "clip X2=79: the left half shows")
	if img.RGBAAt(160, 10) == frameBlue {
		t.Error("clip X2=79: pixel 160 shows")
	}

	f.clip(0x18, 0, 159, 0, 255)
	f.io.nextRegWriteDirect(0x16, 0x3F) // scroll X = 319: bit 8 is in NextREG 0x71
	f.io.nextRegWriteDirect(0x71, 0x01)
	f.want(f.frame(), 1, 0, frameRed, "scrolled X by 319: column 0 at column 1")
}

func TestNextFrameLayer2At640x256(t *testing.T) {
	f := newFrameTest(t)
	f.io.nextRegWriteDirect(0x69, 0x80)
	f.io.nextRegWriteDirect(0x70, 0x20)
	f.palette(1, 0x00, 0xE3)
	f.palette(1, 0x01, 0xE0)
	f.palette(1, 0x02, 0x1C)
	f.layer2Byte(0, 0x12)           // (0,0) red, (1,0) green
	f.layer2Byte(319*256+255, 0x21) // (638,255) green, (639,255) red
	f.clip(0x18, 0, 159, 0, 255)
	img := f.frame()
	if img.Rect.Dx() != 640 || img.Rect.Dy() != 256 {
		t.Fatalf("frame is %v in the 640x256 mode, want 640x256", img.Rect)
	}
	f.want(img, 0, 0, frameRed, "640x256 (0,0)")
	f.want(img, 1, 0, frameGreen, "640x256 (1,0)")
	f.want(img, 638, 255, frameGreen, "640x256 (638,255)")
	f.want(img, 639, 255, frameRed, "640x256 (639,255)")
	// The classic picture is two frame pixels to one of its own, 64 in.
	f.want(img, 64, 32, frameBlack, "classic picture's corner at double width")
}

func TestNextFrameSpritesAndTheBorder(t *testing.T) {
	f := newFrameTest(t)
	f.palette(2, 0xF0, 0xE0) // red
	f.io.sprites.WriteSpritePatternByte(0, 0, 0, 0xF0)
	place := func(x, y uint8) {
		f.io.nextRegWriteDirect(0x34, 0)
		f.io.nextRegWriteDirect(0x35, x)
		f.io.nextRegWriteDirect(0x36, y)
		f.io.nextRegWriteDirect(0x38, 0x80)
	}
	// Sprite coordinates are the frame's: (32,32) is the classic picture's corner.
	place(32, 32)
	f.io.nextRegWriteDirect(0x15, 0x01)
	f.want(f.frame(), 32, 32, frameRed, "sprite at (32,32)")

	// In the border it shows only with "sprites over border".
	place(4, 4)
	if f.frame().RGBAAt(4, 4) == frameRed {
		t.Error("a sprite in the border shows without sprites over border")
	}
	f.io.nextRegWriteDirect(0x15, 0x03)
	f.want(f.frame(), 4, 4, frameRed, "sprite in the border, sprites over border on")

	// Clipped over the border, the window is from the frame's corner with X doubled.
	f.clip(0x19, 3, 255, 0, 255) // pixels 6 and up
	f.io.nextRegWriteDirect(0x15, 0x23)
	if f.frame().RGBAAt(4, 4) == frameRed {
		t.Error("the sprite at x=4 shows outside a clip window that starts at pixel 6")
	}
	place(6, 4)
	f.want(f.frame(), 6, 4, frameRed, "sprite at x=6, on the clip window's edge")

	// Not over the border, the window is in the classic picture's coordinates.
	f.io.nextRegWriteDirect(0x15, 0x01)
	f.clip(0x19, 10, 255, 0, 191)
	place(32+9, 32)
	if f.frame().RGBAAt(32+9, 32) == frameRed {
		t.Error("a sprite at column 9 of the picture shows with the clip window starting at 10")
	}
	place(32+10, 32)
	f.want(f.frame(), 32+10, 32, frameRed, "sprite at column 10 of the picture")
}

func TestNextFrameTilemapCoversTheBorder(t *testing.T) {
	f := newFrameTest(t)
	bank5 := func(addr int, v uint8) { f.mem.nextPageWrite(uint8(10+addr>>13), uint16(addr&0x1FFF), v) }
	for i := 0; i < 32; i++ {
		bank5(0x0C00+32+i, 0x55) // tile 1, all of colour 5
	}
	cell := func(col, row int) {
		bank5(0x2C00+(row*40+col)*2, 1)
		bank5(0x2C00+(row*40+col)*2+1, 0)
	}
	cell(0, 0)   // the frame's corner
	cell(39, 31) // the opposite corner
	cell(4, 4)   // the classic picture's corner
	f.io.nextRegWriteDirect(0x6B, 0x81)
	img := f.frame()
	five := tilemapColour(5)
	f.want(img, 0, 0, five, "tile at column 0, row 0")
	f.want(img, 319, 255, five, "tile at column 39, row 31")
	f.want(img, 32, 32, five, "tile at column 4, row 4")
	f.want(img, 8, 0, nrgbaToRGBA(f.io.nextPalette.lookup(nextLayerULA, 16).Colour()), "an empty tile in the border: the border shows")

	// 80 columns: the frame is 640 wide and a tile is still eight of its pixels.
	f.io.nextRegWriteDirect(0x6B, 0xC1)
	bank5(0x2C00+79*2, 1)
	bank5(0x2C00+79*2+1, 0)
	img = f.frame()
	if img.Rect.Dx() != 640 {
		t.Fatalf("frame is %d wide with the 80 column tilemap", img.Rect.Dx())
	}
	f.want(img, 639, 0, five, "tile at column 79 of 80")
	f.want(img, 7, 0, five, "tile at column 0 of 80")
}

func TestNextFrameOrderTransparencyAndFallback(t *testing.T) {
	f := newFrameTest(t)
	f.io.nextRegWriteDirect(0x69, 0x80)
	f.palette(1, 0x00, 0xE3)
	f.palette(1, 0x01, 0xE0) // Layer 2 red at the classic picture's corner
	f.layer2Byte(0, 1)
	f.palette(2, 0xF0, 0x1C) // a green sprite on the same pixel
	f.io.sprites.WriteSpritePatternByte(0, 0, 0, 0xF0)
	for reg, v := range map[uint8]uint8{0x34: 0, 0x35: 32, 0x36: 32, 0x38: 0x80} {
		f.io.nextRegWriteDirect(reg, v)
	}
	// Bits 4-2 of NextREG 0x15, the topmost layer first in each name.
	for _, c := range []struct {
		order uint8
		name  string
		want  color.RGBA
	}{
		{0, "S L U", frameGreen}, {1, "L S U", frameRed}, {2, "S U L", frameGreen},
		{3, "L U S", frameRed}, {4, "U S L", frameBlack}, {5, "U L S", frameBlack},
	} {
		f.io.nextRegWriteDirect(0x15, c.order<<2|0x01)
		f.want(f.frame(), 32, 32, c.want, c.name)
	}

	// With the ULA on top, a Layer 2 pixel shows through where the ULA is switched off.
	f.io.nextRegWriteDirect(0x15, 5<<2) // U L S, sprites off
	f.io.nextRegWriteDirect(0x68, 0x80)
	img := f.frame()
	f.want(img, 32, 32, frameRed, "ULA off: Layer 2 shows under it")
	// Where every layer is transparent the fallback colour shows, border included.
	f.io.nextRegWriteDirect(0x4A, 0x03)
	img = f.frame()
	f.want(img, 33, 32, frameBlue, "nothing opaque: the fallback colour")
	f.want(img, 0, 0, frameBlue, "ULA off: the border is the fallback colour too")
	f.io.nextRegWriteDirect(0x68, 0x00)

	// The ULA's clip window takes pixels out of the classic picture, not out of the border.
	f.clip(0x1A, 1, 255, 0, 191)
	img = f.frame()
	f.want(img, 32, 32, frameRed, "ULA clipped at column 0: Layer 2 shows")
	f.want(img, 33, 32, frameBlack, "ULA column 1")
	f.want(img, 0, 0, nrgbaToRGBA(f.io.nextPalette.lookup(nextLayerULA, 16).Colour()), "the border, outside the ULA's clip window")
}

func TestNextClipWindowControlRegister(t *testing.T) {
	f := newFrameTest(t)
	readReg := func(reg uint8) uint8 {
		f.io.nextRegSelect(reg)
		return f.io.nextRegReadSelected()
	}
	f.io.nextRegWriteDirect(0x18, 1) // Layer 2: one write
	f.io.nextRegWriteDirect(0x19, 1) // sprites: two
	f.io.nextRegWriteDirect(0x19, 2)
	f.io.nextRegWriteDirect(0x1A, 1) // ULA: three
	f.io.nextRegWriteDirect(0x1A, 2)
	f.io.nextRegWriteDirect(0x1A, 3)
	if got := readReg(0x1C); got != 0<<6|3<<4|2<<2|1 {
		t.Fatalf("NextREG 0x1C reads 0x%02X, want the four write positions 0x39", got)
	}
	f.io.nextRegWriteDirect(0x1C, 0x04) // restart the ULA's only
	if got := readReg(0x1C); got != 2<<2|1 {
		t.Fatalf("after restarting the ULA's window NextREG 0x1C reads 0x%02X, want 0x09", got)
	}
	if v := f.io.nextClipULA.values; v != [4]uint8{1, 2, 3, 191} {
		t.Fatalf("the ULA's clip window is %v after three writes", v)
	}
	if v := f.io.nextClipSprites.values; v != [4]uint8{1, 2, 0, 191} {
		t.Fatalf("the sprites' clip window is %v after two writes", v)
	}
}

func TestDecodeDisplayIsTheFrameOnANext(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	zx.memory.Enable128K()
	zx.memory.EnablePlus3()
	if r := zx.DecodeDisplay().Rect; r.Dx() != 256 || r.Dy() != 192 {
		t.Fatalf("a +3 gives %v, want the classic 256x192", r)
	}
	zx.memory.EnableNext()
	if r := zx.DecodeDisplay().Rect; r.Dx() != 320 || r.Dy() != 256 {
		t.Fatalf("a Next gives %v, want the 320x256 frame", r)
	}
	zx.io.WritePort(0x00FF, 0x06) // Timex hi-res
	if r := zx.DecodeDisplay().Rect; r.Dx() != 640 || r.Dy() != 256 {
		t.Fatalf("a Next in Timex hi-res gives %v, want 640x256", r)
	}
	zx.io.WritePort(0x00FF, 0x00)
	zx.io.nextRegWriteDirect(0x69, 0x80)
	zx.io.nextRegWriteDirect(0x70, 0x20)
	if r := zx.DecodeDisplay().Rect; r.Dx() != 640 || r.Dy() != 256 {
		t.Fatalf("with Layer 2 at 640x256 the picture is %v", r)
	}
}

func TestNextFrameClassicPictureGoesThroughThePalette(t *testing.T) {
	f := newFrameTest(t)
	screen := f.mem.screen
	for i := range screen.attributes {
		screen.attributes[i] = 0x38 // black ink on white paper
	}
	// The starting palette: white is five sevenths of full on each gun, 0xB6, and bright
	// white is full. These are the colours CSpect shows for NextZXOS.
	img := f.io.ComposeNextFrame(nil, screen, 7)
	f.want(img, 32, 32, color.RGBA{0xB6, 0xB6, 0xB6, 0xFF}, "white paper")
	f.want(img, 0, 0, color.RGBA{0xB6, 0xB6, 0xB6, 0xFF}, "white border")
	screen.attributes[0] = 0x78
	f.want(f.io.ComposeNextFrame(nil, screen, 7), 32, 32, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, "bright white paper")
	screen.attributes[0] = 0x38

	// A palette write shows at once, on the picture and on the border.
	f.palette(0, 16+7, 0x1C) // paper 7 green
	img = f.io.ComposeNextFrame(nil, screen, 7)
	f.want(img, 32, 32, frameGreen, "paper 7 through the palette")
	f.want(img, 0, 0, frameGreen, "border 7 through the palette")
	// The second ULA palette shows only when NextREG 0x43 puts it on display.
	f.palette(4, 16+7, 0xE0)
	f.want(f.io.ComposeNextFrame(nil, screen, 7), 32, 32, frameGreen, "second ULA palette changed, first on display")
	f.io.nextRegWriteDirect(0x43, 0x02)
	f.want(f.io.ComposeNextFrame(nil, screen, 7), 32, 32, frameRed, "second ULA palette on display")
}

func TestPaletteNineBitReadBack(t *testing.T) {
	// NextREG 0x44 reads back the second byte of the pair: the lowest bit of the blue.
	// NextZXOS reads its colours with 0x41 and 0x44 and writes them again with 0x44; when
	// the read gave 0, white lost that bit and came out as a warm grey.
	f := newFrameTest(t)
	read := func(reg uint8) uint8 {
		f.io.nextRegSelect(reg)
		return f.io.nextRegReadSelected()
	}
	f.io.nextRegWriteDirect(0x43, 0x80) // ULA first palette, the index stays put
	f.io.nextRegWriteDirect(0x40, 7)
	if hi, lo := read(0x41), read(0x44); hi != 0xB6 || lo != 0x01 {
		t.Fatalf("starting white reads back as %02X,%02X, want B6,01", hi, lo)
	}
	f.io.nextRegWriteDirect(0x44, 0xB6)
	f.io.nextRegWriteDirect(0x44, 0x00)
	if lo := read(0x44); lo != 0x00 {
		t.Fatalf("after writing B6,00 NextREG 0x44 reads %02X, want 00", lo)
	}
	f.io.nextRegWriteDirect(0x41, 0xB6) // an 8-bit write makes the bit from the other two
	if lo := read(0x44); lo != 0x01 {
		t.Fatalf("after an 8-bit write of B6 NextREG 0x44 reads %02X, want 01", lo)
	}
	// Copying an entry the way NextZXOS does leaves it as it was.
	hi, lo := read(0x41), read(0x44)
	f.io.nextRegWriteDirect(0x40, 23)
	f.io.nextRegWriteDirect(0x44, hi)
	f.io.nextRegWriteDirect(0x44, lo)
	if got, want := f.io.nextPalette.palettes[nextPaletteULA1st][23], f.io.nextPalette.palettes[nextPaletteULA1st][7]; got != want {
		t.Fatalf("an entry copied through 0x41 and 0x44 is %+v, the original %+v", got, want)
	}
}

func TestNextFrameULANext(t *testing.T) {
	f := newFrameTest(t)
	screen := f.mem.screen
	screen.bitmap[0] = 0x80 // (0,0) ink, the rest paper
	// Mask 0x0F: the low four attribute bits are the ink, the high four the paper.
	screen.attributes[0] = 0xA5 // ink 5, paper 10
	f.palette(0, 5, 0xE0)       // ink 5 red
	f.palette(0, 128+10, 0x03)  // paper 10 blue
	f.palette(0, 128+2, 0x1C)   // paper 2, the border's, green
	f.io.nextRegWriteDirect(0x42, 0x0F)
	f.io.nextRegWriteDirect(0x43, 0x01)
	img := f.io.ComposeNextFrame(nil, screen, 2)
	f.want(img, 32, 32, frameRed, "ULANext ink 5")
	f.want(img, 33, 32, frameBlue, "ULANext paper 10")
	f.want(img, 0, 0, frameGreen, "ULANext border: paper 2, from entry 130")
	// All eight bits ink: there is no paper, and the fallback colour shows.
	f.io.nextRegWriteDirect(0x42, 0xFF)
	f.io.nextRegWriteDirect(0x4A, 0x1C)
	f.palette(0, 0xA5, 0xE0)
	f.io.nextRegWriteDirect(0x43, 0x01)
	img = f.io.ComposeNextFrame(nil, screen, 2)
	f.want(img, 32, 32, frameRed, "ULANext, mask 0xFF: ink is entry 0xA5")
	f.want(img, 33, 32, frameGreen, "ULANext, mask 0xFF: paper is the fallback colour")
}

func TestNextRegLogSelection(t *testing.T) {
	f := newFrameTest(t)
	if f.io.nextRegLog.on {
		t.Fatal("the register log is on without being asked for")
	}
	if err := f.io.setNextRegLog("40, 0x44,6b"); err != nil {
		t.Fatal(err)
	}
	w := f.io.nextRegLog.watch
	if !f.io.nextRegLog.on || !w[0x40] || !w[0x44] || !w[0x6B] || w[0x41] {
		t.Fatal("the register log does not watch exactly 0x40, 0x44 and 0x6B")
	}
	if err := f.io.setNextRegLog("all"); err != nil || !f.io.nextRegLog.watch[0x00] || !f.io.nextRegLog.watch[0xFF] {
		t.Fatalf("\"all\" does not watch every register (err %v)", err)
	}
	if err := f.io.setNextRegLog("zz"); err == nil {
		t.Fatal("a register that is not hex was accepted")
	}
	if err := f.io.setNextRegLog(""); err != nil || f.io.nextRegLog.on {
		t.Fatal("an empty list does not turn the log off")
	}
}
