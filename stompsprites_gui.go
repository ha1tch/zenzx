//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"embed"
	"image"
	"image/color"
	"image/png"
	"io/fs"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The pictures of the stop animation (stomp_gui.go): the machine, the boot that comes down on it,
// the leg the boot is on the end of, and what is left of the machine afterwards.
//
// ZenZX draws its own, in code, at the smallest size they are drawn at (each is enlarged by a
// whole number, without smoothing). They are meant to be replaced: a PNG of the right name in
// sprites/stomp is used in place of ZenZX's own for that picture (README.txt there says which
// names, and how the pictures fit together).

//go:embed sprites/stomp
var stompFiles embed.FS

// stompImages are the pictures, before they are textures.
type stompImages struct {
	icon, boot, leg *image.RGBA
	flat            *image.RGBA // what is left of the icon; nil: the icon, squashed
	// bin is what the machine stands on, groundTop the ground the bin stands on (its first
	// row with anything in it is the ground's surface) and ground what is under that, tiled.
	bin, groundTop, ground *image.RGBA
}

// loadStompImages reads sprites/stomp's pictures from fsys (the embedded folder, or a test's),
// and draws ZenZX's own for each that is not there.
func loadStompImages(fsys fs.FS) stompImages {
	read := func(name string) *image.RGBA { return readSpritePNG(fsys, name) }
	s := stompImages{icon: read("icon.png"), boot: read("boot.png"), leg: read("leg.png"), flat: read("flat.png"),
		bin: read("bin.png"), groundTop: read("ground-top.png"), ground: read("ground.png")}
	if s.icon == nil {
		s.icon = drawnStompIcon()
	}
	if s.boot == nil {
		s.boot = drawnStompBoot()
	}
	if s.leg == nil {
		s.leg = drawnStompLeg()
	}
	if s.bin == nil {
		s.bin = drawnStompBin()
	}
	if s.groundTop == nil {
		s.groundTop = drawnStompGroundTop()
	}
	if s.ground == nil {
		s.ground = drawnStompGround()
	}
	return s
}

// embeddedStompImages are the pictures this build has.
func embeddedStompImages() stompImages {
	sub, err := fs.Sub(stompFiles, "sprites/stomp")
	if err != nil {
		return loadStompImages(emptyFS{})
	}
	return loadStompImages(sub)
}

// readSpritePNG reads the PNG called name from fsys as an image at the origin; nil when there is
// none, or it is not a picture.
func readSpritePNG(fsys fs.FS, name string) *image.RGBA {
	f, err := fsys.Open(name)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil || img.Bounds().Empty() {
		return nil
	}
	out := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			out.Set(x, y, img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y))
		}
	}
	return out
}

// emptyFS is a folder with nothing in it.
type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

// paint fills the pixels x0 to x1 and y0 to y1, the ends included.
func paint(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if image.Pt(x, y).In(img.Bounds()) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

var (
	stompBody  = color.RGBA{0x00, 0x00, 0x00, 0xff}
	stompEdge  = color.RGBA{0x55, 0x55, 0x66, 0xff} // not drawn by the icon now; kept for the colour test
	stompKey   = color.RGBA{0xc8, 0xc8, 0xc8, 0xff}
	stompBadge = color.RGBA{0xc8, 0x00, 0x00, 0xff}
	stompGreen = color.RGBA{0x00, 0xff, 0x00, 0xff} // the stripe of the Spectrum's badge, in one colour
)

// drawnStompIcon is a small ZX Spectrum, a black slab: a red badge in its corner, four rows of
// light keys each one pixel further along than the one above, and a green stripe running down to
// the left at its lower right, a staircase of single pixels. It is the picture ZenZX was given,
// in the part of it that was seen: nothing is added to it. 23 by 15.
func drawnStompIcon() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 23, 15))
	paint(img, 0, 0, 22, 14, stompBody)
	paint(img, 1, 1, 3, 1, stompBadge)
	for r := 0; r < 4; r++ { // the keys: two pixels wide, a gap of one
		n := 7
		switch r {
		case 2:
			n = 6
		case 3:
			n = 4
		}
		for k := 0; k < n; k++ {
			x := 1 + r + k*3
			paint(img, x, 5+r*2, x+1, 5+r*2, stompKey)
		}
	}
	for y := 10; y <= 14; y++ {
		paint(img, 22-(y-10), y, 22-(y-10), y, stompGreen) // a pixel further left every row
	}
	return img
}

var (
	stompLeather = color.RGBA{0x1c, 0x1c, 0x1c, 0xff}
	stompShine   = color.RGBA{0x6a, 0x6a, 0x6a, 0xff}
	stompSole    = color.RGBA{0x58, 0x36, 0x18, 0xff}
	stompLegLo   = color.RGBA{0x9a, 0x9a, 0xa8, 0xff}
	stompLegHi   = color.RGBA{0xd2, 0xd2, 0xdc, 0xff}
	stompLegEdge = color.RGBA{0x4a, 0x4a, 0x58, 0xff}
)

// drawnStompBoot is a boot seen from the side, toe to the right, its leg joining at the top
// middle: a shaft, a foot, and a sole. 24 by 16.
func drawnStompBoot() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 24, 16))
	paint(img, 7, 0, 16, 8, stompLeather)   // the shaft, under the leg
	paint(img, 2, 8, 21, 13, stompLeather)  // the foot
	paint(img, 21, 9, 22, 13, stompLeather) // the toe, rounded
	paint(img, 8, 1, 9, 7, stompShine)
	paint(img, 4, 9, 18, 9, stompShine)
	paint(img, 1, 14, 23, 15, stompSole) // the sole
	for _, p := range [][2]int{{2, 8}, {2, 9}, {1, 14}, {23, 14}, {23, 15}} {
		img.SetRGBA(p[0], p[1], color.RGBA{}) // corners knocked off
	}
	return img
}

// drawnStompLeg is a piece of leg: a column with a lighter middle and darker sides. 10 by 8; the
// top and bottom rows are alike, so that a stack of them has no seam.
func drawnStompLeg() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 10, 8))
	paint(img, 0, 0, 9, 7, stompLegLo)
	paint(img, 3, 0, 6, 7, stompLegHi)
	paint(img, 0, 0, 0, 7, stompLegEdge)
	paint(img, 9, 0, 9, 7, stompLegEdge)
	return img
}

// The bin the machine stands on, and the ground the bin stands on, are zenplay's own: the sprite
// "icon-rubbish" and two of the tile patterns of its package gui-elements.zplay, in the notation
// of dockicon_gui.go (sixteen rows of four hexadecimal digits of ink, a slash, four of mask) for
// the bin and as a word of sixteen bits a row for the patterns. The machine'"'"'s own colours are
// used for them: the keys'"'"' grey for ink and paper on black.
var stompBinRows = [16]string{
	"0180/03c0", "0240/ffff", "7ffe/ffff", "4002/ffff", "7ffe/ffff", "2004/ffff", "2924/7ffe", "2924/7ffe",
	"2924/7ffe", "2924/7ffe", "2924/7ffe", "2924/7ffe", "2494/7ffe", "2004/7ffe", "1ff8/7ffe", "0000/3ffc",
}

// The ground'"'"'s surface: a line two pixels thick and dither under it, with nothing above it where
// the bin stands.
var stompGroundTopRows = [16]uint16{
	0, 0, 0, 0, 0, 0, 0, 0, 0,
	0xffff, 0xffff, 0xaaaa, 0x5555, 0xaaaa, 0x5555, 0xaaaa,
}

// And what is under it: bricks.
var stompGroundRows = [16]uint16{
	0xffff, 0x1020, 0x1020, 0x1020, 0xffff, 0x0204, 0x0204, 0x0204,
	0xffff, 0x1020, 0x1020, 0x1020, 0xffff, 0x0204, 0x0204, 0x0204,
}

// drawnStompBin is the rubbish bin, 16 by 16: black ribs and outline, grey where it is solid,
// nothing round it.
func drawnStompBin() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	icon, err := parseIcon(stompBinRows)
	if err != nil {
		return img // the rows are literals: not reached
	}
	for y, row := range icon {
		for x := 0; x < 16; x++ {
			bit := uint16(0x8000) >> x
			switch {
			case row.ink&bit != 0:
				img.SetRGBA(x, y, stompBody)
			case row.mask&bit != 0:
				img.SetRGBA(x, y, stompKey)
			}
		}
	}
	return img
}

// drawnStompTile is a 16 by 16 tile from rows of bits: the keys'"'"' grey where a bit is set, black
// where it is not, and nothing above the first row that has a bit set when clear is true.
func drawnStompTile(rows [16]uint16, clear bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	seen := !clear
	for y, row := range rows {
		if row != 0 {
			seen = true
		}
		if !seen {
			continue
		}
		for x := 0; x < 16; x++ {
			c := stompBody
			if row&(uint16(0x8000)>>x) != 0 {
				c = stompKey
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func drawnStompGroundTop() *image.RGBA { return drawnStompTile(stompGroundTopRows, true) }
func drawnStompGround() *image.RGBA    { return drawnStompTile(stompGroundRows, false) }

// firstRowWithAPixel is the first row of img, counting from its top, that has a pixel in it (0
// when it has none).
func firstRowWithAPixel(img *image.RGBA) int32 {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y).A != 0 {
				return int32(y - b.Min.Y)
			}
		}
	}
	return 0
}

// stompTextures are the pictures on the graphics card.
type stompTextures struct {
	icon, boot, leg, flat  rl.Texture2D
	hasFlat                bool
	bin, groundTop, ground rl.Texture2D
	// surface is the row of the ground's top tile on which the bin stands.
	surface int32
	// legMid2 is twice the position, in the boot's own pixels, of the middle of where the leg
	// joins it: the middle of the first row of the boot that has anything in it. The leg is drawn
	// with its middle there, so a boot need not be symmetrical.
	legMid2 int32
}

// joinMid2 is twice the middle of the first row of img that has a pixel in it (the middle of its
// width when it has none).
func joinMid2(img *image.RGBA) int32 {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		first, last := -1, -1
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y).A != 0 {
				if first < 0 {
					first = x - b.Min.X
				}
				last = x - b.Min.X
			}
		}
		if first >= 0 {
			return int32(first + last + 1)
		}
	}
	return int32(b.Dx())
}

func newStompTextures(s stompImages) *stompTextures {
	load := func(img *image.RGBA) rl.Texture2D {
		t := rl.LoadTextureFromImage(rl.NewImageFromImage(img))
		rl.SetTextureFilter(t, rl.FilterPoint)
		return t
	}
	t := &stompTextures{icon: load(s.icon), boot: load(s.boot), leg: load(s.leg), legMid2: joinMid2(s.boot),
		bin: load(s.bin), groundTop: load(s.groundTop), ground: load(s.ground), surface: firstRowWithAPixel(s.groundTop)}
	if s.flat != nil {
		t.flat, t.hasFlat = load(s.flat), true
	}
	return t
}

func (t *stompTextures) unload() {
	if t == nil {
		return
	}
	rl.UnloadTexture(t.icon)
	rl.UnloadTexture(t.boot)
	rl.UnloadTexture(t.leg)
	rl.UnloadTexture(t.bin)
	rl.UnloadTexture(t.groundTop)
	rl.UnloadTexture(t.ground)
	if t.hasFlat {
		rl.UnloadTexture(t.flat)
	}
}
