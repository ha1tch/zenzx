// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ha1tch/zentools/pkg/zxn"
)

func writeShotFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListScreenshotsNewestFirstAndOnlyScreenshots(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shots")
	if got := listScreenshots(dir); len(got) != 0 {
		t.Fatalf("a folder that is not there lists %d files", len(got))
	}
	a := screenshotName("Manic-Miner", time.Date(2020, 11, 20, 10, 0, 0, 0, time.Local), "png")
	b := screenshotName("Manic-Miner", time.Date(2020, 11, 27, 9, 30, 0, 0, time.Local), "gif")
	c := screenshotName("Next", time.Date(2020, 11, 27, 9, 30, 0, 0, time.Local), "nxi")
	for _, n := range []string{a, b, c, "notes.txt", "mine.PNG"} {
		writeShotFile(t, dir, n, []byte("x"))
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A file that was renamed is dated by the file, which is as new as it gets.
	got := listScreenshots(dir)
	var names []string
	for _, f := range got {
		names = append(names, f.Name)
	}
	if len(got) != 4 {
		t.Fatalf("listed %q, want the four pictures and neither the text file nor the folder", names)
	}
	if got[0].Name != "mine.PNG" || got[0].Machine != "" || got[0].Ext != "png" {
		t.Errorf("first is %+v, want the renamed file (dated by the file: now), extension in lower case", got[0])
	}
	// The same minute: the name breaks the tie, so the order never wobbles.
	if got[1].Name != b || got[2].Name != c || got[3].Name != a {
		t.Errorf("order is %q, want %q, %q, %q after the renamed one", names, b, c, a)
	}
	if got[1].Machine != "Manic-Miner" || got[1].When.Year() != 2020 || got[1].When.Day() != 27 || got[1].Size != 1 {
		t.Errorf("the GIF is %+v", got[1])
	}
}

func TestShotPictureFrameAt(t *testing.T) {
	p := &shotPicture{Frames: make([]*image.RGBA, 3), Delays: []int{4, 4, 8}}
	for at, want := range map[int]int{0: 0, 3: 0, 4: 1, 7: 1, 8: 2, 15: 2, 16: 0, 20: 1, 1600: 0} {
		if got := p.FrameAt(at); got != want {
			t.Errorf("FrameAt(%d) = %d, want %d", at, got, want)
		}
	}
	one := &shotPicture{Frames: make([]*image.RGBA, 1), Delays: []int{0}}
	if one.Animated() || one.FrameAt(99) != 0 {
		t.Error("a still picture does not move")
	}
	// A frame with a delay of 0 still is shown for a moment, or a loop of them would not end.
	zero := &shotPicture{Frames: make([]*image.RGBA, 2), Delays: []int{0, 0}}
	if got := zero.FrameAt(1); got != 1 {
		t.Errorf("FrameAt(1) with delays 0 = %d, want 1", got)
	}
}

func TestLoadPicturePNGAndGIF(t *testing.T) {
	dir := t.TempDir()
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skipf("48k: %v", err)
	}
	pngPath, err := withShotDir(dir, func() (string, error) { return zx.SaveScreenshot(ShotPNG, true, time.Now()) })
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadShotPicture(pngPath, true)
	if err != nil {
		t.Fatal(err)
	}
	img := zx.ShotImage(true)
	if w, h := p.Size(); w != img.Bounds().Dx() || h != img.Bounds().Dy() || p.Animated() {
		t.Errorf("the PNG is %dx%d (animated %v), want %v", w, h, p.Animated(), img.Bounds())
	}

	red, blue := color.RGBA{0xD7, 0, 0, 0xFF}, color.RGBA{0, 0, 0xD7, 0xFF}
	data, err := encodeGIF([]*image.RGBA{solidFrame(8, 4, red), solidFrame(8, 4, blue), solidFrame(8, 4, red)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	gp := writeShotFile(t, dir, "a.gif", data)
	all, err := loadShotPicture(gp, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Frames) != 3 || all.Delays[1] != 4 || !all.Animated() {
		t.Fatalf("the GIF has %d frames, delays %v", len(all.Frames), all.Delays)
	}
	if got := all.Frames[1].RGBAAt(3, 2); got != blue {
		t.Errorf("the second frame's pixel is %v, want %v", got, blue)
	}
	if got := all.Frames[2].RGBAAt(3, 2); got != red {
		t.Errorf("the third frame's pixel is %v, want %v", got, red)
	}
	first, err := loadShotPicture(gp, true)
	if err != nil || len(first.Frames) != 1 || first.Frames[0].RGBAAt(0, 0) != red {
		t.Errorf("the first frame of the GIF: %v, %v", first, err)
	}
}

// withShotDir runs f with the screenshots folder in dir.
func withShotDir(dir string, f func() (string, error)) (string, error) {
	old := screenshotDir
	screenshotDir = dir
	defer func() { screenshotDir = old }()
	return f()
}

func TestLoadPictureSCR(t *testing.T) {
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skipf("48k: %v", err)
	}
	zx.screen.bitmap[0] = 0x80     // the top left pixel is ink
	zx.screen.attributes[0] = 0x02 // red ink on black
	data, err := zx.ShotBytes(ShotSCR, false)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadShotPicture(writeShotFile(t, t.TempDir(), "a.scr", data), true)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 256 || h != 192 {
		t.Fatalf("a SCR is %dx%d, want 256x192", w, h)
	}
	if p.Frames[0].RGBAAt(0, 0) == p.Frames[0].RGBAAt(1, 0) {
		t.Error("the ink pixel and the paper next to it are the same colour")
	}
	if _, err := loadShotPicture(writeShotFile(t, t.TempDir(), "b.scr", make([]byte, 100)), true); err == nil {
		t.Error("a SCR of 100 bytes was read")
	}
}

func TestLoadPictureTimexGuessesTheMode(t *testing.T) {
	dir := t.TempDir()
	hiColour := make([]byte, zxn.TimexFileLen)
	for i := range hiColour[:zxn.TimexBitmapLen] {
		hiColour[i] = byte(i * 7)
	}
	for i := zxn.TimexBitmapLen; i < zxn.TimexFileLen; i++ {
		hiColour[i] = byte(0x38 + i%3) // a handful of attributes
	}
	hiRes := make([]byte, zxn.TimexFileLen)
	for i := range hiRes {
		hiRes[i] = byte(i * 13)
	}
	for name, c := range map[string]struct {
		data []byte
		w, h int
	}{
		"colour.scr": {hiColour, 256, 192},
		"res.scr":    {hiRes, 512, 192},
		"plus.scr":   {append(append([]byte{}, hiRes...), 0x06), 512, 192}, // a mode byte after the screen
	} {
		p, err := loadShotPicture(writeShotFile(t, dir, name, c.data), true)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if w, h := p.Size(); w != c.w || h != c.h {
			t.Errorf("%s is %dx%d, want %dx%d", name, w, h, c.w, c.h)
		}
	}
}

func TestLoadPictureNextBitmaps(t *testing.T) {
	n := nextShotMachine()
	io := n.io
	io.layer2Access.raw = 0x02
	// Two colours for the two bytes that are written: red and green, in Layer 2's first palette.
	for _, p := range [][2]uint8{{0x5A, 0xE0}, {0xC3, 0x1C}} {
		io.nextRegWriteDirect(0x43, 0x10)
		io.nextRegWriteDirect(0x40, p[0])
		io.nextRegWriteDirect(0x41, p[1])
	}
	dir := t.TempDir()
	for _, c := range []struct {
		reg  uint8
		w, h int
		name string
	}{
		{0x00, 256, 192, "l2"},
		{0x10, 320, 256, "wide"},
		{0x20, 640, 256, "nibble"},
	} {
		io.SetNextRegister(0x70, c.reg)
		base := io.layer2BaseBank()
		n.memory.nextPageWrite(base, 0, 0x5A)
		n.memory.nextPageWrite(base, 1, 0xC3)
		data, err := n.ShotBytes(ShotLayer2, true)
		if err != nil {
			t.Fatal(err)
		}
		p, err := loadShotPicture(writeShotFile(t, dir, c.name+".nxi", data), true)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if w, h := p.Size(); w != c.w || h != c.h {
			t.Errorf("%s is %dx%d, want %dx%d", c.name, w, h, c.w, c.h)
		}
		img := p.Frames[0]
		switch c.reg {
		case 0x00: // row by row: the first two bytes are side by side
			if img.RGBAAt(0, 0) == img.RGBAAt(1, 0) {
				t.Errorf("%s: two different colours came out the same", c.name)
			}
		case 0x10: // column by column: they are one above the other
			if img.RGBAAt(0, 0) == img.RGBAAt(0, 1) {
				t.Errorf("%s: two different colours came out the same", c.name)
			}
		}
	}

	// LoRes: both modes.
	io.SetNextRegister(0x15, 0x80)
	data, _ := n.ShotBytes(ShotLoRes, true)
	p, err := loadShotPicture(writeShotFile(t, dir, "lores.nxi", data), true)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 128 || h != 96 {
		t.Errorf("LoRes is %dx%d, want 128x96", w, h)
	}
	io.SetNextRegister(0x6A, 0x20)
	data, _ = n.ShotBytes(ShotLoRes, true)
	if p, err = loadShotPicture(writeShotFile(t, dir, "radastan.nxi", data), true); err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 128 || h != 96 {
		t.Errorf("Radastan is %dx%d, want 128x96", w, h)
	}

	// A bare Layer 2 screen, as other tools write one, has no palette: the Next's own colours.
	p, err = loadShotPicture(writeShotFile(t, dir, "bare.nxi", make([]byte, 49152)), true)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 256 || h != 192 {
		t.Errorf("a bare Layer 2 screen is %dx%d, want 256x192", w, h)
	}
	if _, err := loadShotPicture(writeShotFile(t, dir, "odd.nxi", make([]byte, 1000)), true); err == nil {
		t.Error("a Next bitmap of 1000 bytes was read")
	}
}

func TestLoadPictureRefusesWhatIsNotOne(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string][]byte{
		"bad.png":  []byte("not a png"),
		"bad.gif":  []byte("GIF89a but not really"),
		"zero.png": nil,
	} {
		if _, err := loadShotPicture(writeShotFile(t, dir, name, data), false); err == nil {
			t.Errorf("%s was read", name)
		}
	}
	if _, err := loadShotPicture(filepath.Join(dir, "missing.png"), false); err == nil {
		t.Error("a file that is not there was read")
	}
}
