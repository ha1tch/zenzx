package zenzx

import (
	"image/color"
	"testing"
)

func TestFourBitSpritePatterns(t *testing.T) {
	io := newNextIO()
	io.nextRegWriteDirect(0x15, 0x03) // sprites on, over the border
	io.nextRegWriteDirect(0x4B, 0xE3) // transparent: nibble 3 for a 4-bit pattern
	// Pattern slot 5 holds two 4-bit patterns. The first: left pixel colour 1, next
	// colour 2, then a transparent one (3), on its first line. The second: all colour 4.
	p := &io.sprites.patterns[5]
	p[0], p[1] = 0x12, 0x35
	for i := 128; i < 256; i++ {
		p[i] = 0x44
	}
	colour := func(index uint8) color.RGBA {
		return nrgbaToRGBA(io.nextPalette.lookup(nextLayerSprites, index).Colour())
	}
	set := func(sprite, x, y, attr2, attr3, attr4 uint8) {
		io.nextRegWriteDirect(0x34, sprite)
		io.nextRegWriteDirect(0x35, x)
		io.nextRegWriteDirect(0x36, y)
		io.nextRegWriteDirect(0x37, attr2)
		io.nextRegWriteDirect(0x38, attr3)
		io.nextRegWriteDirect(0x39, attr4)
	}
	// Sprite 0: pattern 5, 4-bit, first half, palette offset 2.
	set(0, 40, 40, 0x20, 0xC5, 0x80)
	img := io.DecodeSpritesFrame()
	for x, want := range map[int]uint8{40: 0x21, 41: 0x22, 43: 0x25} {
		if got := img.RGBAAt(x, 40); got != colour(want) {
			t.Errorf("4-bit sprite pixel at x=%d is %v, want palette entry 0x%02X %v", x, got, want, colour(want))
		}
	}
	if got := img.RGBAAt(42, 40); got.A != 0 {
		t.Errorf("the pixel whose nibble is the low four bits of NextREG 0x4B is %v, want transparent", got)
	}
	// N6 chooses the second half of the slot.
	set(0, 40, 40, 0x00, 0xC5, 0xC0)
	if got := io.DecodeSpritesFrame().RGBAAt(40, 40); got != colour(0x04) {
		t.Errorf("with N6 set the first pixel is %v, want the second pattern's colour 4 %v", got, colour(0x04))
	}
	// A relative sprite has its anchor's kind of pattern; its N6 is attribute 4 bit 5.
	set(0, 40, 40, 0x00, 0xC5, 0x80)
	set(1, 16, 0, 0x00, 0xC5, 0x60) // relative, 16 to the right, N6 set
	img = io.DecodeSpritesFrame()
	if got := img.RGBAAt(56, 40); got != colour(0x04) {
		t.Errorf("the relative sprite's first pixel is %v, want the second 4-bit pattern's colour 4 %v", got, colour(0x04))
	}
	// An 8-bit sprite of the same slot still reads a byte a pixel.
	set(0, 40, 40, 0x00, 0x85, 0x00)
	set(1, 0, 0, 0, 0, 0)
	if got := io.DecodeSpritesFrame().RGBAAt(40, 40); got != colour(0x12) {
		t.Errorf("an 8-bit sprite's first pixel is %v, want palette entry 0x12 %v", got, colour(0x12))
	}
}
