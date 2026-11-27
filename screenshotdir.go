// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ha1tch/zentools/pkg/scr"
	"github.com/ha1tch/zentools/pkg/zxn"
	"github.com/ha1tch/zenui/zxpalette"
)

// What is in the screenshots folder, and how a file of it becomes a picture: the Manage
// Screenshots window (screenshots_gui.go) lists the files and draws their pictures from here.

// screenshotFile is one file of the screenshots folder.
type screenshotFile struct {
	Path    string
	Name    string
	Ext     string    // "png", "scr", "hires.scr", "hicol.scr", "nxi" or "gif"
	Size    int64     //
	When    time.Time // when it was made: from its name when it is one ZenZX gave, otherwise the file's
	Machine string    // the machine's name in its name, "" for a file that was renamed
}

// listScreenshots reads the screenshots in dir, the newest first. Anything that is not of one of
// the formats is left out, and so is a folder.
func listScreenshots(dir string) []screenshotFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []screenshotFile
	for _, e := range entries {
		name := e.Name()
		ext := shotExtOf(name)
		if e.IsDir() || !isScreenshotExt(ext) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		f := screenshotFile{Path: filepath.Join(dir, name), Name: name, Ext: ext, Size: info.Size(), When: info.ModTime()}
		if machine, when, _, ok := parseScreenshotName(name); ok {
			f.Machine, f.When = machine, when
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].When.Equal(out[j].When) {
			return out[i].When.After(out[j].When)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// shotPicture is a screenshot's picture: one frame, or the frames of a GIF with how long each
// is shown, in hundredths of a second.
type shotPicture struct {
	Frames []*image.RGBA
	Delays []int
}

// Size is the size of the picture.
func (p *shotPicture) Size() (w, h int) {
	if len(p.Frames) == 0 {
		return 0, 0
	}
	b := p.Frames[0].Bounds()
	return b.Dx(), b.Dy()
}

// Animated says whether the picture has more than one frame.
func (p *shotPicture) Animated() bool { return len(p.Frames) > 1 }

// FrameAt is the frame to show after t hundredths of a second, going round and round.
func (p *shotPicture) FrameAt(t int) int {
	total := 0
	for _, d := range p.Delays {
		total += max(d, 1)
	}
	if len(p.Frames) < 2 || total == 0 {
		return 0
	}
	t %= total
	for i, d := range p.Delays {
		if t -= max(d, 1); t < 0 {
			return i
		}
	}
	return len(p.Frames) - 1
}

func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

// loadShotPicture reads the file at path as a picture. For a GIF, first says that the first
// frame is all that is wanted, which is quick: a thumbnail needs no more.
func loadShotPicture(path string, first bool) (*shotPicture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch shotExtOf(path) {
	case "png":
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return &shotPicture{Frames: []*image.RGBA{toRGBA(img)}, Delays: []int{0}}, nil
	case "gif":
		return decodeShotGIF(data, first)
	case "scr":
		img, err := decodeShotSCR(data)
		if err != nil {
			return nil, err
		}
		return &shotPicture{Frames: []*image.RGBA{img}, Delays: []int{0}}, nil
	case hiResExt:
		img, err := decodeShotHiRes(data)
		if err != nil {
			return nil, err
		}
		return &shotPicture{Frames: []*image.RGBA{img}, Delays: []int{0}}, nil
	case hiColExt:
		img, err := decodeShotHiColour(data)
		if err != nil {
			return nil, err
		}
		return &shotPicture{Frames: []*image.RGBA{img}, Delays: []int{0}}, nil
	case "nxi":
		img, err := decodeShotNXI(data)
		if err != nil {
			return nil, err
		}
		return &shotPicture{Frames: []*image.RGBA{img}, Delays: []int{0}}, nil
	}
	return nil, errors.New("not a picture ZenZX knows")
}

// decodeShotGIF reads a GIF: each frame is drawn over the one before, as a viewer does.
func decodeShotGIF(data []byte, first bool) (*shotPicture, error) {
	if first {
		img, err := gif.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return &shotPicture{Frames: []*image.RGBA{toRGBA(img)}, Delays: []int{0}}, nil
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(g.Image) == 0 {
		return nil, errors.New("the GIF has no frames")
	}
	w, h := g.Config.Width, g.Config.Height
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	p := &shotPicture{}
	for i, fr := range g.Image {
		draw.Draw(canvas, fr.Bounds(), fr, fr.Bounds().Min, draw.Over)
		cp := image.NewRGBA(canvas.Bounds())
		copy(cp.Pix, canvas.Pix)
		p.Frames = append(p.Frames, cp)
		p.Delays = append(p.Delays, g.Delay[i])
	}
	return p, nil
}

// decodeShotSCR reads a ZX Spectrum screen: 6912 bytes, or the Timex's 12288 in a plain .scr.
func decodeShotSCR(data []byte) (*image.RGBA, error) {
	switch len(data) {
	case scr.FileLen:
		s, err := scr.Decode(data)
		if err != nil {
			return nil, err
		}
		return scr.ToImage(s), nil
	case zxn.TimexFileLen, zxn.TimexFileLen + 1: // a plain .scr of the Timex's size, from other tools
		if hiRes, _, _ := timexScreenKind("", data); hiRes {
			return decodeShotHiRes(data)
		}
		return decodeShotHiColour(data)
	}
	return nil, fmt.Errorf("a screen is %d or %d bytes; this is %d", scr.FileLen, zxn.TimexFileLen, len(data))
}

// checkTimexLen refuses what is not a Timex screen: 12288 bytes, and one more that some tools add.
func checkTimexLen(data []byte) error {
	if len(data) != zxn.TimexFileLen && len(data) != zxn.TimexFileLen+1 {
		return fmt.Errorf("a Timex screen is %d bytes; this is %d", zxn.TimexFileLen, len(data))
	}
	return nil
}

// decodeShotHiRes reads a Timex hi-res screen, a .hires.scr: 12288 bytes, and after them the byte
// of port 0xFF when there is one, whose bits 5-3 are the ink; without it the ink is black. It is
// drawn as the machine draws it, ink and paper both bright.
func decodeShotHiRes(data []byte) (*image.RGBA, error) {
	if err := checkTimexLen(data); err != nil {
		return nil, err
	}
	ink := uint8(0)
	if len(data) > zxn.TimexFileLen {
		ink = (data[zxn.TimexFileLen] >> 3) & 7
	}
	h, err := zxn.DecodeHiRes(data[:zxn.TimexFileLen], ink)
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, 512, 192))
	inkC, paperC := scr.ULAColor(h.Ink, true), scr.ULAColor(7-h.Ink, true)
	for y := 0; y < 192; y++ {
		for x := 0; x < 512; x++ {
			c := paperC
			if h.Pixels[y][x] {
				c = inkC
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img, nil
}

// decodeShotHiColour reads a Timex hi-colour screen, a .hicol.scr: the bitmap, then an attribute
// for each of its bytes, 12288 bytes. A trailing byte, which some tools add, is not read.
func decodeShotHiColour(data []byte) (*image.RGBA, error) {
	if err := checkTimexLen(data); err != nil {
		return nil, err
	}
	h, err := zxn.DecodeHiColour(data[:zxn.TimexFileLen])
	if err != nil {
		return nil, err
	}
	return h.Image(), nil
}

// timexScreenKind says whether a screen file is a Timex screen, and which: hi-res or hi-colour.
// A .hires.scr and a .hicol.scr say so; a plain .scr goes by its size, as the screen files of
// the other emulators do: 12289 bytes is hi-res (the 12288 and the byte of port 0xFF), and 12288
// is hi-colour by that convention, but other tools save hi-res without the byte, so a bitmap in
// its second half decides (timexIsHiRes). ok is false for anything else, a standard screen.
func timexScreenKind(name string, data []byte) (hiRes, ok bool, err error) {
	switch shotExtOf(name) {
	case hiResExt:
		return true, true, checkTimexLen(data)
	case hiColExt:
		return false, true, checkTimexLen(data)
	}
	switch len(data) {
	case zxn.TimexFileLen + 1:
		return true, true, nil
	case zxn.TimexFileLen:
		return timexIsHiRes(data), true, nil
	}
	return false, false, nil
}

// timexIsHiRes guesses whether a 12288-byte Timex screen is hi-res or hi-colour, which a plain .scr
// of that size does not say (a .hires.scr and a .hicol.scr do): the second half of a hi-colour screen is attributes, which have a few values
// between them, and the second half of a hi-res one is half of the bitmap, which has many.
func timexIsHiRes(data []byte) bool {
	var seen [256]bool
	n := 0
	for _, b := range data[zxn.TimexBitmapLen:] {
		if !seen[b] {
			seen[b] = true
			n++
		}
	}
	return n > 48
}

// nxiLayout is a Next bitmap file as a size says it is: its picture, how many bytes of palette
// come before the pixels, and how they are stored.
type nxiLayout struct {
	w, h, depth int
	order       zxn.PixelOrder
	palette     int
}

// nxiLayouts are the sizes of the .nxi files ZenZX writes (screenshot.go), with their palette,
// and the bare Layer 2 screens other tools make.
var nxiLayouts = map[int]nxiLayout{
	512 + 49152: {256, 192, 8, zxn.RowMajor, 512},
	512 + 81920: {320, 256, 8, zxn.ColumnMajor, 512},
	32 + 81920:  {640, 256, 4, zxn.ColumnMajor, 32}, // a 16-colour palette: the 4-bit mode
	512 + 12288: {128, 96, 8, zxn.RowMajor, 512},
	32 + 6144:   {128, 96, 4, zxn.RowMajor, 32},
	49152:       {256, 192, 8, zxn.RowMajor, 0},
	81920:       {320, 256, 8, zxn.ColumnMajor, 0},
}

// decodeShotNXI reads a Next bitmap: the palette that is in the file, or the Next's own colours
// when there is none.
func decodeShotNXI(data []byte) (*image.RGBA, error) {
	l, ok := nxiLayouts[len(data)]
	if !ok {
		return nil, fmt.Errorf("a Next bitmap this size (%d bytes) is not one ZenZX knows", len(data))
	}
	var pal *zxn.Palette
	if l.palette > 0 {
		p, err := zxn.DecodeNXP(data[:l.palette])
		if err != nil {
			return nil, err
		}
		pal = p
	} else {
		pal = &zxn.Palette{Colors: make([]zxn.Color333, 256)}
		for i := range pal.Colors {
			c := zxpalette.DecodeRGB9From8Bit(uint8(i))
			pal.Colors[i] = zxn.Color333{R: c.R, G: c.G, B: c.B}
		}
	}
	bm, err := zxn.DecodeNXI(data[l.palette:], l.w, l.h, l.depth, l.order)
	if err != nil {
		return nil, err
	}
	return zxn.BitmapImage(bm, pal, -1), nil
}
