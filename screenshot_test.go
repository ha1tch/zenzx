// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ha1tch/zentools/pkg/scr"
	"github.com/ha1tch/zentools/pkg/zxn"
)

func TestShotMachineField(t *testing.T) {
	for in, want := range map[string]string{
		"MANIC":              "MANIC",
		"Jet Set Willy":      "Jet-Set-Willy",
		"  two  spaces  ":    "two-spaces",
		"under_score":        "under-score",
		"a/b\\c:d*e?f":       "a-b-c-d-e-f",
		"Spectrum 128K":      "Spectrum-128K",
		"--lead and trail--": "lead-and-trail",
		"Pokémon":            "Pokémon",
		"":                   "",
		"!!!":                "",
		"...":                "",
		"a_b_c":              "a-b-c",
		"0123456789012345678901234567890123456789": "01234567890123456789012345678901",
	} {
		if got := shotMachineField(in); got != want {
			t.Errorf("shotMachineField(%q) = %q, want %q", in, got, want)
		}
	}
	// No underscore may survive: it is the field separator.
	for _, in := range []string{"a_b", "_", "x _ y"} {
		for _, r := range shotMachineField(in) {
			if r == '_' {
				t.Errorf("shotMachineField(%q) kept an underscore", in)
			}
		}
	}
	// A name cut at the limit does not end in a hyphen.
	if got := shotMachineField("0123456789012345678901234567890 x"); got[len(got)-1] == '-' {
		t.Errorf("shotMachineField cut at the limit left a trailing hyphen: %q", got)
	}
}

func TestScreenshotNameRoundTrip(t *testing.T) {
	when := time.Date(2026, 11, 27, 9, 5, 7, 0, time.Local)
	name := screenshotName("Jet Set Willy", when, "png")
	if name != "screenshot_Jet-Set-Willy_20261127_090507.png" {
		t.Fatalf("screenshotName = %q", name)
	}
	machine, got, ext, ok := parseScreenshotName(name)
	if !ok || machine != "Jet-Set-Willy" || !got.Equal(when) || ext != "png" {
		t.Errorf("parseScreenshotName(%q) = %q, %v, %q, %v", name, machine, got, ext, ok)
	}
	for _, bad := range []string{
		"screenshot_20261127_090507.png",     // no machine
		"screenshot_a_b_20261127_090507.png", // an underscore in the machine
		"shot_a_20261127_090507.png",         // another prefix
		"screenshot_a_20261127_090507.txt",   // not an image
		"screenshot_a_20261340_090507.png",   // not a date
		"screenshot_a_20261127_090507",       // no extension
		"screenshot__20261127_090507.png",    // an empty machine
		"my-favourite-screenshot.png",
	} {
		if _, _, _, ok := parseScreenshotName(bad); ok {
			t.Errorf("parseScreenshotName(%q) read a name that is not one", bad)
		}
	}
	if _, _, ext, ok := parseScreenshotName("screenshot_a_20261127_090507.GIF"); !ok || ext != "gif" {
		t.Errorf("the extension is not read in any case: %q, %v", ext, ok)
	}
}

func TestWriteScreenshotFileDoesNotWriteOver(t *testing.T) {
	old := screenshotDir
	screenshotDir = filepath.Join(t.TempDir(), "shots")
	defer func() { screenshotDir = old }()
	when := time.Date(2026, 11, 27, 9, 0, 0, 0, time.Local)
	a, err := writeScreenshotFile("m", when, "png", []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := writeScreenshotFile("m", when, "png", []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("the second screenshot went over the first: %s", a)
	}
	if got, _ := os.ReadFile(a); string(got) != "first" {
		t.Errorf("the first screenshot was changed: %q", got)
	}
	if _, when2, _, ok := parseScreenshotName(filepath.Base(b)); !ok || !when2.Equal(when.Add(time.Second)) {
		t.Errorf("the second name %q is not the next second's: %v %v", b, when2, ok)
	}
}

// nextShotMachine is a bare machine with a Next's memory, for the Next's formats.
func nextShotMachine() *ZenZX {
	mem, screen := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	return &ZenZX{io: io, memory: mem, screen: screen}
}

func shotSet(zx *ZenZX) (set map[ShotFormat]bool) {
	set = map[ShotFormat]bool{}
	for _, f := range ShotFormats {
		set[f] = zx.ShotApplicable(f)
	}
	return set
}

func TestShotApplicableByMode(t *testing.T) {
	// A classic machine in its standard mode: PNG, SCR and GIF.
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skipf("48k: %v", err)
	}
	want := map[ShotFormat]bool{ShotPNG: true, ShotSCR: true, ShotGIF: true}
	for f, got := range shotSet(zx) {
		if got != want[f] {
			t.Errorf("48K: ShotApplicable(%d) = %v, want %v", f, got, want[f])
		}
	}

	// A Next that is showing only its ULA: the same.
	n := nextShotMachine()
	for f, got := range shotSet(n) {
		if got != want[f] {
			t.Errorf("Next, ULA only: ShotApplicable(%d) = %v, want %v", f, got, want[f])
		}
	}

	// Layer 2 on: its .nxi as well.
	n.io.layer2Access.raw = 0x02
	if !n.ShotApplicable(ShotLayer2) || n.ShotApplicable(ShotLoRes) || n.ShotApplicable(ShotHiRes) || n.ShotApplicable(ShotHiColour) {
		t.Errorf("Next, Layer 2 on: %v", shotSet(n))
	}
	n.io.layer2Access.raw = 0

	// LoRes takes the ULA's place: its .nxi, and no SCR.
	n.io.SetNextRegister(0x15, 0x80)
	if !n.ShotApplicable(ShotLoRes) || n.ShotApplicable(ShotSCR) {
		t.Errorf("Next, LoRes on: %v", shotSet(n))
	}
	n.io.SetNextRegister(0x15, 0)

	// The ULA off: no SCR.
	n.io.SetNextRegister(0x68, 0x80)
	if n.ShotApplicable(ShotSCR) {
		t.Errorf("Next, ULA off: SCR is applicable")
	}
	n.io.SetNextRegister(0x68, 0)

	// Timex hi-res and hi-colour: a file of each; screen 1 is a standard screen.
	for port, want := range map[uint8]struct{ res, col bool }{0x06: {res: true}, 0x02: {col: true}, 0x01: {}, 0x00: {}} {
		n.io.nextTimexPortWrite(port)
		wantTimex := want.res || want.col
		if got := n.ShotApplicable(ShotHiRes); got != want.res {
			t.Errorf("Next, port 0xFF = %02X: hi-res = %v, want %v", port, got, want.res)
		}
		if got := n.ShotApplicable(ShotHiColour); got != want.col {
			t.Errorf("Next, port 0xFF = %02X: hi-colour = %v, want %v", port, got, want.col)
		}
		if got := n.ShotApplicable(ShotSCR); got == wantTimex {
			t.Errorf("Next, port 0xFF = %02X: SCR = %v alongside Timex = %v", port, got, wantTimex)
		}
	}

	// A format that does not apply is not made.
	if _, err := zx.ShotBytes(ShotLayer2, true); err == nil {
		t.Errorf("a 48K made a Layer 2 file")
	}
	if _, err := zx.ShotBytes(ShotGIF, true); err == nil {
		t.Errorf("ShotBytes made a GIF, which takes two seconds")
	}
}

func TestShotSCRIsTheDisplayFile(t *testing.T) {
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skipf("48k: %v", err)
	}
	zx.screen.bitmap[0], zx.screen.bitmap[6143] = 0xAA, 0x55
	zx.screen.attributes[0], zx.screen.attributes[767] = 0x47, 0x38
	data, err := zx.ShotBytes(ShotSCR, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != scr.FileLen {
		t.Fatalf("a SCR is %d bytes, want %d", len(data), scr.FileLen)
	}
	if data[0] != 0xAA || data[6143] != 0x55 || data[6144] != 0x47 || data[6911] != 0x38 {
		t.Errorf("the SCR is not the bitmap and the attributes in order: % X ... % X", data[:2], data[6142:6146])
	}
	if _, err := scr.Decode(data); err != nil {
		t.Errorf("zentools does not read what was saved: %v", err)
	}
}

func TestShotPNGHasTheBorder(t *testing.T) {
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skipf("48k: %v", err)
	}
	zx.io.borderColor = 2 // red
	with, err := zx.ShotBytes(ShotPNG, true)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(with))
	if err != nil {
		t.Fatal(err)
	}
	l, r, tp, b := zx.videoRenderer.BorderMargins()
	if cfg.Width != ScreenWidth+l+r || cfg.Height != ScreenHeight+tp+b {
		t.Errorf("a PNG with the border is %dx%d, want %dx%d", cfg.Width, cfg.Height, ScreenWidth+l+r, ScreenHeight+tp+b)
	}
	img, _ := png.Decode(bytes.NewReader(with))
	if got := color.RGBAModel.Convert(img.At(0, 0)); got != zxPalette[2] {
		t.Errorf("the corner is %v, want the border's red %v", got, zxPalette[2])
	}
	without, _ := zx.ShotBytes(ShotPNG, false)
	cfg, _ = png.DecodeConfig(bytes.NewReader(without))
	if cfg.Width != ScreenWidth || cfg.Height != ScreenHeight {
		t.Errorf("a PNG without the border is %dx%d, want %dx%d", cfg.Width, cfg.Height, ScreenWidth, ScreenHeight)
	}
}

func TestShotLayer2IsThePalettePlusTheMemory(t *testing.T) {
	n := nextShotMachine()
	io := n.io
	io.layer2Access.raw = 0x02
	for _, c := range []struct {
		reg    uint8
		bytes  int
		depth  int
		palLen int
		name   string
	}{
		{0x00, 49152, 8, 512, "256x192"},
		{0x10, 81920, 8, 512, "320x256"},
		{0x20, 81920, 4, 32, "640x256"},
	} {
		io.SetNextRegister(0x70, c.reg)
		base := io.layer2BaseBank()
		n.memory.nextPageWrite(base, 0, 0x5A)
		n.memory.nextPageWrite(base, 1, 0xC3)
		data, err := n.ShotBytes(ShotLayer2, true)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(data) != c.palLen+c.bytes {
			t.Fatalf("%s: %d bytes, want %d+%d", c.name, len(data), c.palLen, c.bytes)
		}
		if data[c.palLen] != 0x5A || data[c.palLen+1] != 0xC3 {
			t.Errorf("%s: the pixels do not start with the memory's: % X", c.name, data[c.palLen:c.palLen+2])
		}
		if _, err := zxn.DecodeNXP(data[:c.palLen]); err != nil {
			t.Errorf("%s: the palette is not one zentools reads: %v", c.name, err)
		}
		// zentools reads the pixels back as the layer holds them.
		mode, _ := zxn.ParseLayer2Mode(c.name)
		if _, err := zxn.DecodeNXI(data[c.palLen:], mode.Width, mode.Height, mode.Depth, mode.Order); err != nil {
			t.Errorf("%s: zentools does not read the pixels: %v", c.name, err)
		}
	}
}

func TestShotLayer2PaletteDrawsAsTheLayerDoes(t *testing.T) {
	n := nextShotMachine()
	io := n.io
	io.layer2Access.raw = 0x02
	io.SetNextRegister(0x70, 0x02) // palette offset 2: index 0 reads colour 32
	data, _ := n.ShotBytes(ShotLayer2, true)
	pal, err := zxn.DecodeNXP(data[:512])
	if err != nil {
		t.Fatal(err)
	}
	want := io.nextPalette.lookup(nextLayerLayer2, 32)
	if got := pal.Colors[0]; got.R != want.R || got.G != want.G || got.B != want.B {
		t.Errorf("colour 0 of the file is %+v, want the layer's colour 32: %+v", got, want)
	}
}

func TestShotLoResBothModes(t *testing.T) {
	n := nextShotMachine()
	io := n.io
	io.SetNextRegister(0x15, 0x80)
	n.memory.nextPageWrite(10, 0, 0x11) // bank 5 offset 0: the first pixel of the top half
	n.memory.nextPageWrite(11, 0, 0x22) // bank 5 offset 0x2000: the first pixel of the bottom half
	data, err := n.ShotBytes(ShotLoRes, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 512+12288 || data[512] != 0x11 || data[512+6144] != 0x22 {
		t.Errorf("standard LoRes: %d bytes, first pixels % X ... % X", len(data), data[512:513], data[512+6144:512+6145])
	}
	io.SetNextRegister(0x6A, 0x20) // Radastan: 4 bits a pixel, 64 bytes a line
	data, err = n.ShotBytes(ShotLoRes, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 32+6144 {
		t.Errorf("Radastan LoRes: %d bytes, want %d", len(data), 32+6144)
	}
}

func TestShotTimexIsTwoHalves(t *testing.T) {
	n := nextShotMachine()
	n.memory.nextPageWrite(10, 0, 0xA1)
	n.memory.nextPageWrite(11, 0, 0xB2)
	// Hi-res: the two halves and the byte of port 0xFF, whose bits 5-3 are the ink.
	n.io.nextTimexPortWrite(0x1E)
	data, err := n.ShotBytes(ShotHiRes, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != zxn.TimexFileLen+1 || data[0] != 0xA1 || data[zxn.TimexBitmapLen] != 0xB2 || data[zxn.TimexFileLen] != 0x1E {
		t.Errorf("hi-res: %d bytes, halves start % X and % X, last % X", len(data), data[:1], data[zxn.TimexBitmapLen:zxn.TimexBitmapLen+1], data[len(data)-1:])
	}
	if _, err := zxn.DecodeHiRes(data[:zxn.TimexFileLen], 3); err != nil {
		t.Errorf("zentools does not read the hi-res screen: %v", err)
	}
	// Hi-colour: the two halves and nothing else.
	n.io.nextTimexPortWrite(0x02)
	data, err = n.ShotBytes(ShotHiColour, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != zxn.TimexFileLen || data[0] != 0xA1 || data[zxn.TimexBitmapLen] != 0xB2 {
		t.Errorf("hi-colour: %d bytes, halves start % X and % X", len(data), data[:1], data[zxn.TimexBitmapLen:zxn.TimexBitmapLen+1])
	}
	if _, err := zxn.DecodeHiColour(data); err != nil {
		t.Errorf("zentools does not read the hi-colour screen: %v", err)
	}
	// A format the mode does not hold is not made.
	if _, err := n.ShotBytes(ShotHiRes, true); err == nil {
		t.Error("a hi-colour screen made a hi-res file")
	}
}

// --- the GIF ---------------------------------------------------------------------------------

func solidFrame(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 0xFF
	}
	return img
}

func TestEncodeGIFKeepsFewColoursExactly(t *testing.T) {
	red, blue := color.RGBA{0xD7, 0, 0, 0xFF}, color.RGBA{0, 0, 0xD7, 0xFF}
	data, err := encodeGIF([]*image.RGBA{solidFrame(8, 4, red), solidFrame(8, 4, blue)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) != 2 || g.Delay[0] != 4 || g.Delay[1] != 4 {
		t.Fatalf("%d frames, delays %v", len(g.Image), g.Delay)
	}
	for i, want := range []color.RGBA{red, blue} {
		if got := color.RGBAModel.Convert(g.Image[i].At(3, 2)); got != want {
			t.Errorf("frame %d pixel is %v, want exactly %v", i, got, want)
		}
	}
}

func TestEncodeGIFCutsManyColoursTo256(t *testing.T) {
	// A frame with 1000 different colours: too many for a palette, cut down by the median cut.
	img := image.NewRGBA(image.Rect(0, 0, 40, 25))
	for i := 0; i < 1000; i++ {
		img.SetRGBA(i%40, i/40, color.RGBA{uint8(i * 7), uint8(i * 13), uint8(i * 31), 0xFF})
	}
	data, err := encodeGIF([]*image.RGBA{img}, 4)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(g.Image[0].Palette); n > 256 {
		t.Errorf("a palette of %d colours", n)
	}
	// What it gives back is near what went in: the cut is not a scramble.
	var worst int
	for y := 0; y < 25; y++ {
		for x := 0; x < 40; x++ {
			a := img.RGBAAt(x, y)
			r, gg, b, _ := g.Image[0].At(x, y).RGBA()
			for _, d := range []int{int(a.R) - int(r>>8), int(a.G) - int(gg>>8), int(a.B) - int(b>>8)} {
				worst = max(worst, max(d, -d))
			}
		}
	}
	if worst > 80 {
		t.Errorf("the cut moved a channel by %d", worst)
	}
}

func TestEncodeGIFMakesFramesOfOneSize(t *testing.T) {
	a, b := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, color.RGBA{0, 0, 0xFF, 0xFF}
	data, err := encodeGIF([]*image.RGBA{solidFrame(4, 3, a), solidFrame(8, 3, b)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if g.Config.Width != 8 || g.Config.Height != 3 {
		t.Fatalf("the GIF is %dx%d, want 8x3", g.Config.Width, g.Config.Height)
	}
	// The narrow frame is made twice as wide by its own pixels, so no black is round it.
	if got := color.RGBAModel.Convert(g.Image[0].At(7, 2)); got != a {
		t.Errorf("the narrow frame's right edge is %v, want %v", got, a)
	}
}

func TestEncodeGIFWithNoFrames(t *testing.T) {
	if _, err := encodeGIF(nil, 4); err == nil {
		t.Errorf("a GIF of nothing was made")
	}
}

// A recording is two seconds of the machine's own frames: 100 of them give 50, 4 hundredths of a
// second each, and a machine that is paused gives none.
func TestRecordingTakesEveryOtherFrame(t *testing.T) {
	old := screenshotDir
	screenshotDir = filepath.Join(t.TempDir(), "shots")
	defer func() { screenshotDir = old }()
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skipf("48k: %v", err)
	}
	zx.paused = false
	when := time.Date(2026, 11, 27, 9, 0, 0, 0, time.Local)
	if !zx.StartGIF(true, when) {
		t.Fatal("a recording did not start")
	}
	if zx.StartGIF(true, when) {
		t.Fatal("a second recording started while the first was under way")
	}
	zx.paused = true
	for i := 0; i < 10; i++ {
		zx.RunFrame()
	}
	if have, on := zx.GIFRecording(); !on || have != 0 {
		t.Fatalf("a paused machine recorded %d frames (recording %v)", have, on)
	}
	zx.paused = false
	for i := 0; i < 97; i++ {
		zx.RunFrame()
	}
	// Frames 0, 2, ... 96 of the 97 run so far are 49 of the recording's.
	if have, on := zx.GIFRecording(); !on || have != GIFFrames-1 {
		t.Fatalf("after 97 frames: %d taken (recording %v), want %d", have, on, GIFFrames-1)
	}
	for i := 0; i < 3; i++ { // the 99th frame is the 50th taken, which hands the recording on
		zx.RunFrame()
	}
	var res []ShotResult
	for i := 0; i < 400 && len(res) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
		res = zx.TakeShotResults()
	}
	if len(res) != 1 || res[0].Err != nil || res[0].Format != ShotGIF {
		t.Fatalf("results: %+v", res)
	}
	if _, on := zx.GIFRecording(); on {
		t.Errorf("the recording is still under way after it was handed on")
	}
	f, err := os.Open(res[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) != GIFFrames {
		t.Errorf("%d frames in the GIF, want %d", len(g.Image), GIFFrames)
	}
	for i, d := range g.Delay {
		if d != gifDelay {
			t.Fatalf("frame %d has a delay of %d, want %d", i, d, gifDelay)
		}
	}
	if g.LoopCount != 0 {
		t.Errorf("loop count %d: it should repeat for ever", g.LoopCount)
	}
	if _, w, ext, ok := parseScreenshotName(filepath.Base(res[0].Path)); !ok || ext != "gif" || !w.Equal(when) {
		t.Errorf("the GIF's name %q is not a screenshot's: %v %q %v", res[0].Path, w, ext, ok)
	}
}
