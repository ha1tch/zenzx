package zenzx

import (
	"image"
	"testing"
)

func TestNextTimexPortAndRegisterAreOneLatch(t *testing.T) {
	io, _, _ := newTilemapTestIO(t)
	if io.nextTimexMode() != 0 || io.DecodeNextTimex(nil) != nil {
		t.Fatal("a Next starts outside the standard screen mode")
	}
	io.WritePort(0x00FF, 0x0E) // hi-res, ink 1
	if io.nextTimexMode() != nextTimexHiRes {
		t.Fatalf("port 0xFF <- 0x0E: mode %d, want hi-res", io.nextTimexMode())
	}
	if got := io.GetNextRegister(0x69) & 0x3F; got != 0x0E {
		t.Fatalf("NextREG 0x69 bits 5-0 read 0x%02X after the port write, want 0x0E", got)
	}
	io.nextRegWriteDirect(0x69, 0x82) // Layer 2 on, hi-colour
	if io.nextTimexMode() != nextTimexHiColour || !io.layer2Enabled() {
		t.Fatalf("NextREG 0x69 <- 0x82: mode %d, Layer 2 %v", io.nextTimexMode(), io.layer2Enabled())
	}
}

func TestNextTimexHiResInterleavesTheTwoScreens(t *testing.T) {
	io, _, _ := newTilemapTestIO(t)
	mem := io.memory
	mem.nextPageWrite(10, 0, 0x80) // screen 0, first byte: column 0
	mem.nextPageWrite(11, 0, 0x01) // screen 1, first byte: column 1
	io.WritePort(0x00FF, 0x06)     // hi-res, ink 0: black on white
	img := io.DecodeNextTimex(nil)
	if img == nil || img.Rect.Dx() != 512 || img.Rect.Dy() != 192 {
		t.Fatalf("hi-res picture is %v, want 512x192", img)
	}
	ink, paper := io.ulaColour(8), io.ulaColour(24+7)
	for x, want := range map[int]bool{0: true, 1: false, 7: false, 8: false, 15: true, 16: false} {
		got := img.RGBAAt(x, 0) == ink
		if got != want || (!got && img.RGBAAt(x, 0) != paper) {
			t.Errorf("hi-res pixel %d: ink=%v, want %v", x, got, want)
		}
	}
	// The colour bits: ink 2 (red) on its complement (cyan), both bright.
	io.WritePort(0x00FF, 0x16)
	img = io.DecodeNextTimex(nil)
	if img.RGBAAt(0, 0) != io.ulaColour(8+2) || img.RGBAAt(1, 0) != io.ulaColour(24+5) {
		t.Errorf("hi-res colours: %v on %v, want bright red on bright cyan", img.RGBAAt(0, 0), img.RGBAAt(1, 0))
	}
	// The display decode hands the wide picture on, and the tilemap combines with it.
	io.nextRegWriteDirect(0x6B, 0x80) // tilemap on, 40 columns: half as fine as the ULA here
	out := io.CombineULAAndTilemap(img, nil)
	if out.Rect.Dx() != 512 {
		t.Errorf("hi-res ULA with a 40 column tilemap gives %d pixels, want 512", out.Rect.Dx())
	}
}

func TestNextTimexScreen1AndHiColour(t *testing.T) {
	io, _, _ := newTilemapTestIO(t)
	mem := io.memory
	// Screen 1: a standard screen in the upper half of bank 5.
	mem.nextPageWrite(11, 0, 0x80)
	mem.nextPageWrite(11, 0x1800, 0x56) // bright, paper 2, ink 6
	io.WritePort(0x00FF, 0x01)
	img := io.DecodeNextTimex(nil)
	if img.Rect != image.Rect(0, 0, 256, 192) {
		t.Fatalf("screen 1 picture is %v", img.Rect)
	}
	if img.RGBAAt(0, 0) != io.ulaColour(8+6) || img.RGBAAt(1, 0) != io.ulaColour(24+2) {
		t.Errorf("screen 1: %v and %v, want bright yellow ink on bright red paper", img.RGBAAt(0, 0), img.RGBAAt(1, 0))
	}
	// Hi-colour: pixels from screen 0, an attribute for each pixel row from screen 1.
	mem.nextPageWrite(10, 0, 0xFF)      // row 0, column 0
	mem.nextPageWrite(10, 0x0100, 0xFF) // row 1, column 0
	mem.nextPageWrite(11, 0, 0x01)      // row 0: blue ink
	mem.nextPageWrite(11, 0x0100, 0x04) // row 1: green ink
	io.WritePort(0x00FF, 0x02)
	img = io.DecodeNextTimex(nil)
	if img.RGBAAt(0, 0) != io.ulaColour(1) || img.RGBAAt(0, 1) != io.ulaColour(4) {
		t.Errorf("hi-colour: rows 0 and 1 are %v and %v, want blue and green", img.RGBAAt(0, 0), img.RGBAAt(0, 1))
	}
}
