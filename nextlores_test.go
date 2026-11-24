package zenzx

import (
	"image/color"
	"testing"
)

func TestLoRes(t *testing.T) {
	f := newFrameTest(t)
	bank5 := func(offset int, v uint8) { f.mem.nextPageWrite(uint8(10+offset>>13), uint16(offset&0x1FFF), v) }
	ula := func(index uint8) color.RGBA {
		return nrgbaToRGBA(f.io.nextPalette.lookup(nextLayerULA, index).Colour())
	}
	// ULA palette entries the test can tell apart.
	set := func(index, colour uint8) {
		f.io.nextRegWriteDirect(0x43, 0x00)
		f.io.nextRegWriteDirect(0x40, index)
		f.io.nextRegWriteDirect(0x41, colour)
	}
	set(0x21, 0xE0)
	set(0x22, 0x1C)
	set(0x35, 0x03)
	set(0x40, 0xFC)
	bank5(0, 0x21)          // LoRes pixel (0,0)
	bank5(1, 0x22)          // (1,0)
	bank5(0x2000, 0x35)     // (0,48): the bottom half starts at 0x6000
	bank5(47*128+127, 0x22) // (127,47)
	f.io.nextRegWriteDirect(0x15, 0x80)
	img := f.frame()
	at := func(x, y int) color.RGBA { return img.RGBAAt(nextFrameInset+x, nextFrameInset+y) }
	for _, c := range []struct {
		x, y int
		want uint8
		what string
	}{
		{0, 0, 0x21, "pixel (0,0), top left of its 2x2"}, {1, 1, 0x21, "pixel (0,0), bottom right of its 2x2"},
		{2, 0, 0x22, "pixel (1,0)"}, {0, 96, 0x35, "pixel (0,48), from the second block"},
		{255, 95, 0x22, "pixel (127,47)"},
	} {
		if got := at(c.x, c.y); got != ula(c.want) {
			t.Errorf("%s: %v, want ULA palette entry 0x%02X %v", c.what, got, c.want, ula(c.want))
		}
	}
	// Scrolling is in frame pixels.
	f.io.nextRegWriteDirect(0x33, 96)
	if got := f.frame().RGBAAt(nextFrameInset, nextFrameInset); got != ula(0x35) {
		t.Errorf("scrolled 96 down, the corner is %v, want pixel (0,48) %v", got, ula(0x35))
	}
	f.io.nextRegWriteDirect(0x33, 0)
	f.io.nextRegWriteDirect(0x32, 2)
	if got := f.frame().RGBAAt(nextFrameInset, nextFrameInset); got != ula(0x22) {
		t.Errorf("scrolled 2 across, the corner is %v, want pixel (1,0) %v", got, ula(0x22))
	}
	f.io.nextRegWriteDirect(0x32, 1)
	if got := f.frame().RGBAAt(nextFrameInset, nextFrameInset); got != ula(0x21) {
		t.Errorf("scrolled 1 across, half a LoRes pixel, the corner is %v, want pixel (0,0) still %v", got, ula(0x21))
	}
	f.io.nextRegWriteDirect(0x32, 0)
	f.io.nextRegWriteDirect(0x33, 0)

	// Radastan: a nibble a pixel, the palette offset its high four bits.
	bank5(0, 0x15)
	f.io.nextRegWriteDirect(0x6A, 0x22) // Radastan, offset 2
	img = f.frame()
	if a, b := at(0, 0), at(2, 0); a != ula(0x21) || b != ula(0x25) {
		t.Errorf("Radastan pixels (0,0) and (1,0) are %v and %v, want entries 0x21 and 0x25", a, b)
	}
	// The other display file.
	bank5(0x2000, 0x20)
	f.io.nextRegWriteDirect(0x6A, 0x32)
	if got := f.frame().RGBAAt(nextFrameInset, nextFrameInset); got != ula(0x22) {
		t.Errorf("Radastan from the second display file: %v, want entry 0x22", got)
	}
	// Off again: the ULA's picture is back.
	f.io.nextRegWriteDirect(0x15, 0x00)
	if got := f.frame().RGBAAt(nextFrameInset, nextFrameInset); got == ula(0x22) {
		t.Error("LoRes is still drawn with NextREG 0x15 bit 7 clear")
	}
}
