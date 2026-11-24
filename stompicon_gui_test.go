//go:build !headless

package zenzx

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The icon the stop animation stamps on is the Spectrum it was given: a red badge, four rows of
// keys, and a green stripe of single pixels running down to the left. Nothing else is on it.
func TestStompIconIsTheSpectrumItWasGiven(t *testing.T) {
	img := drawnStompIcon()
	if b := img.Bounds(); b.Dx() != 23 || b.Dy() != 15 {
		t.Fatalf("the icon is %v, want 23 by 15", b)
	}
	at := func(x, y int) color.RGBA { return img.RGBAAt(x, y) }
	// The badge: three red pixels in the corner.
	for x := 0; x < 23; x++ {
		want := stompBody
		if x >= 1 && x <= 3 {
			want = stompBadge
		}
		if at(x, 1) != want {
			t.Errorf("badge row, column %d is %v, want %v", x, at(x, 1), want)
		}
	}
	// The keys: rows 5, 7, 9 and 11, each starting a pixel further along, keys of two pixels with
	// one between, seven, seven, six and four of them.
	for r, n := range []int{7, 7, 6, 4} {
		y, first := 5+r*2, 1+r
		for x := 0; x < 23; x++ {
			key := false
			for k := 0; k < n; k++ {
				if x == first+k*3 || x == first+k*3+1 {
					key = true
				}
			}
			if key != (at(x, y) == stompKey) {
				t.Errorf("key row %d, column %d is %v (a key: %v)", y, x, at(x, y), key)
			}
		}
	}
	// The stripe: one green pixel on each row from 10 to 14, a pixel further left each row, and
	// no other green anywhere; nothing but the sprite's own colours.
	green := 0
	for y := 0; y < 15; y++ {
		for x := 0; x < 23; x++ {
			c := at(x, y)
			if c == stompGreen {
				green++
				if y < 10 || x != 22-(y-10) {
					t.Errorf("green at %d,%d, off the stripe", x, y)
				}
			} else if c != stompBody && c != stompKey && c != stompBadge {
				t.Fatalf("pixel %d,%d is %v, a colour the sprite does not have", x, y, c)
			}
		}
	}
	if green != 5 {
		t.Errorf("%d green pixels, want 5", green)
	}
	if dir := os.Getenv("ZENZX_STOMP_SHOTS"); dir != "" { // a look at it, enlarged without smoothing
		big := image.NewRGBA(image.Rect(0, 0, 23*16, 15*16))
		for y := 0; y < 15*16; y++ {
			for x := 0; x < 23*16; x++ {
				big.SetRGBA(x, y, img.RGBAAt(x/16, y/16))
			}
		}
		if f, err := os.Create(filepath.Join(dir, "icon.png")); err == nil {
			png.Encode(f, big)
			f.Close()
		}
	}
}

// The bin and the ground are zenplay's: the rubbish icon, and a ground made of a surface and bricks.
func TestTheBinAndTheGround(t *testing.T) {
	bin := drawnStompBin()
	if b := bin.Bounds(); b.Dx() != 16 || b.Dy() != 16 {
		t.Fatalf("the bin is %v, want 16 by 16", b)
	}
	// Its lid's knob at the top, a rim, ribs in its body, and nothing in the corners.
	if bin.RGBAAt(7, 0) != stompBody || bin.RGBAAt(6, 0) != stompKey || bin.RGBAAt(0, 0).A != 0 || bin.RGBAAt(15, 15).A != 0 {
		t.Error("the bin's lid is not where it should be")
	}
	if bin.RGBAAt(4, 8) != stompBody || bin.RGBAAt(5, 8) != stompKey {
		t.Errorf("the bin's ribs are %v and %v at 4,8 and 5,8", bin.RGBAAt(4, 8), bin.RGBAAt(5, 8))
	}
	top, under := drawnStompGroundTop(), drawnStompGround()
	if r := firstRowWithAPixel(top); r != 9 {
		t.Errorf("the ground's surface is on row %d, want 9", r)
	}
	if top.RGBAAt(3, 4).A != 0 || top.RGBAAt(3, 9) != stompKey || top.RGBAAt(3, 10) != stompKey {
		t.Error("the ground is not a line of two pixels with nothing above it")
	}
	// The dither under the line goes on as a checkerboard (neighbours differ, in a row and a column).
	for y := 11; y < 15; y++ {
		for x := 0; x < 15; x++ {
			if top.RGBAAt(x, y) == top.RGBAAt(x+1, y) || top.RGBAAt(x, y) == top.RGBAAt(x, y+1) {
				t.Fatalf("the dither at %d,%d is not a checkerboard", x, y)
			}
		}
	}
	if under.RGBAAt(5, 0) != stompKey || under.RGBAAt(3, 1) != stompKey || under.RGBAAt(4, 1) != stompBody {
		t.Error("the bricks are not bricks")
	}
	if under.RGBAAt(0, 0).A == 0 {
		t.Error("the ground under the surface has holes in it")
	}
}

// The foot that ships in sprites/stomp is the one it was drawn as: 25 by 24, a leg of eight
// pixels coming into its top at the left, and a leg piece of eight by eight that fits into that.
func TestTheShippedFootAndLeg(t *testing.T) {
	got := embeddedStompImages()
	if b := got.boot.Bounds(); b.Dx() != 25 || b.Dy() != 24 {
		t.Errorf("the boot is %v, want 25 by 24", b)
	}
	if b := got.leg.Bounds(); b.Dx() != 8 || b.Dy() != 8 {
		t.Errorf("the leg is %v, want 8 by 8", b)
	}
	// The leg joins the boot where the boot's first row has its pixels: columns 2 to 9, middle 6.
	if m := joinMid2(got.boot); m != 12 {
		t.Errorf("the leg joins at twice %d.0, want twice 6", m/2)
	}
	// A leg piece's top and bottom rows are alike, so that a stack of them has no seam
	// (the inside of the leg may differ; its edges are what show a seam).
	for _, x := range []int{0, 7} {
		if got.leg.RGBAAt(x, 0) != got.leg.RGBAAt(x, 7) {
			t.Errorf("the leg's top and bottom rows differ at column %d: a seam", x)
		}
	}
	// A symmetrical boot joins in the middle, as the drawn one does.
	if m := joinMid2(drawnStompBoot()); m != 24 {
		t.Errorf("the drawn boot joins at twice %d.0, want twice 12", m/2)
	}
	if m := joinMid2(image.NewRGBA(image.Rect(0, 0, 9, 3))); m != 9 {
		t.Errorf("an empty picture joins at %d, want its own width, 9", m)
	}
}
