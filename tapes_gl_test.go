//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zenui"
)

// variedScreen is a standard screen that has many colours across it.
func variedScreen() []byte {
	scr := make([]byte, tapeScreenLen)
	for i := 0; i < 6144; i++ {
		scr[i] = byte(i * 37)
	}
	for i := 0; i < 768; i++ {
		scr[6144+i] = byte(i%7+1) | byte(i/32%8)<<3
	}
	return scr
}

// TestGLTapesWindowDrawsTheScreensAndTheBlocks draws the Tapes window in a real window, as the grid
// and as rows: each tape that has a screen shows it in its thumbnail, a tape without one and a file
// that cannot be read have a mark on black, the chosen tape's blocks are written in the box under the
// list and scroll with Tab and the arrow keys, and no ink lands beyond the dialog. Dormant guard G-35
// (docs/KNOWN_ISSUES.md); it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_TAPES_OUT=tapes go test -run TestGLTapesWindow -v .
func TestGLTapesWindowDrawsTheScreensAndTheBlocks(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-35)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	out := os.Getenv("ZENZX_TAPES_OUT")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	rl.SetTraceLogLevel(rl.LogNone)
	const W, H = 1280, 800
	rl.InitWindow(W, H, "ZenZX tapes")
	t.Cleanup(rl.CloseWindow) // registered first, so it runs after the bar's text is unloaded

	dir := filepath.Join(t.TempDir(), "a-folder-with-a-very-long-name-that-goes-on-and-on-and-on-and-on", "tapes")
	loader := tap.EncodeProgram("LOADER", []byte{1, 2, 3, 4}, 10)
	screen := tap.EncodeCode("SCREEN$", variedScreen(), 16384)
	files := map[string][]byte{
		"manic-miner.tap":   concat(loader, screen),
		"jet-set-willy.tap": concat(loader, tap.EncodeCode("title.scr", variedScreen(), 32768)),
		"no-screen.tap":     aLongTape(3),
		"long-tape.tap":     concat(aLongTape(12), screen),
		"broken.tap":        []byte("not a tape"),
		"a-tape-with-an-extremely-long-name-indeed-that-must-be-cut-short.tap": concat(loader, screen),
	}
	for name, data := range files {
		writeTape(t, dir, name, data)
	}

	b, _ := newTestBar(t, nil, "")
	rt := rl.LoadRenderTexture(W, H)
	defer rl.UnloadRenderTexture(rt)
	w := newTapesWindow(dir, nil, nil)
	defer w.close()

	draw := func(name string) []color.RGBA {
		rl.BeginTextureMode(rt)
		rl.ClearBackground(rl.Gray)
		w.Draw(b.renderer, W, H, b.theme)
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(rt.Texture)
		rl.ImageFlipVertical(img)
		px := rl.LoadImageColors(img)
		rl.ExportImage(*img, filepath.Join(out, name+".png"))
		rl.UnloadImage(img)
		return px
	}
	frames := func(n int, keys ...zenui.Key) {
		for i := 0; i < n; i++ {
			w.Update(zenui.Input{DeltaTime: 0.02, Keys: keys}, b.renderer, W, H)
			keys = nil
			draw("scratch")
		}
	}
	distinct := func(px []color.RGBA, r zenui.Rect) int {
		seen := map[color.RGBA]bool{}
		for y := max(r.Y, 0); y < min(r.Y+r.H, H); y++ {
			for x := max(r.X, 0); x < min(r.X+r.W, W); x++ {
				seen[px[y*W+x]] = true
			}
		}
		return len(seen)
	}
	inkBeside := func(px []color.RGBA, p zenui.Rect) (n int) {
		for y := max(p.Y, 0); y < min(p.Y+p.H, H); y++ {
			for x := p.X + p.W + 2; x < min(p.X+p.W+60, W); x++ {
				if px[y*W+x] != px[y*W+W-1] {
					n++
				}
			}
		}
		return n
	}
	thumbRect := func(i int) zenui.Rect {
		c := w.list.ItemRect(i, w.listArea, w.lineH)
		pad := max(2, w.lineH/4)
		side := c.W - 2*pad
		if w.list.Columns == 1 {
			side = w.lineH * 2
		}
		return zenui.Rect{X: c.X + pad + 2, Y: c.Y + pad + 2, W: side - 4, H: side - 4}
	}
	visible := func(r zenui.Rect) bool {
		a := w.listArea
		return r.X >= a.X && r.Y >= a.Y && r.X+r.W <= a.X+a.W && r.Y+r.H <= a.Y+a.H
	}
	hasScreen := func(name string) bool {
		return name == "manic-miner.tap" || name == "jet-set-willy.tap" || name == "long-tape.tap" ||
			name == "a-tape-with-an-extremely-long-name-indeed-that-must-be-cut-short.tap"
	}
	check := func(layout string, px []color.RGBA, floor int) {
		for i, f := range w.all {
			if !visible(thumbRect(i)) { // a later row of the grid is below the list until it scrolls
				continue
			}
			n := distinct(px, thumbRect(i))
			switch {
			case hasScreen(f.Name):
				if n < floor {
					t.Errorf("%s: %s has %d colours in its thumbnail, want at least %d: its screen", layout, f.Name, n, floor)
				}
			default:
				if n > 8 {
					t.Errorf("%s: %s has %d colours in its thumbnail: it should be black with a mark", layout, f.Name, n)
				}
			}
		}
		if n := inkBeside(px, w.panel); n > 0 {
			t.Errorf("%s: %d pixels of ink are beyond the right edge of the dialog", layout, n)
		}
	}

	// The grid. Two tapes are read a frame: a few frames and they are all there.
	frames(8)
	px := draw("tapes-grid")
	check("grid", px, 16)
	if n := distinct(px, w.blockArea); n < 2 {
		t.Errorf("grid: the blocks' box has %d colours: nothing is written in it", n)
	}

	// Rows.
	w.do(tapesBtnLayout)
	frames(3)
	px = draw("tapes-rows")
	check("rows", px, 4)

	// The long tape's blocks scroll: Tab gives them the keys, and the box is not the same after Down.
	for i, f := range w.all {
		if f.Name == "long-tape.tap" {
			w.list.Selected = i
		}
	}
	frames(2)
	before := append([]color.RGBA(nil), draw("blocks-top")...)
	if n := len(w.blocksOf()); n != 26 || n <= w.blockLines {
		t.Fatalf("long-tape.tap shows %d blocks of %d on show: want 26 and more than fit", n, w.blockLines)
	}
	frames(1, zenui.KeyTab)
	frames(1, zenui.KeyDown)
	frames(1, zenui.KeyDown)
	after := draw("blocks-scrolled")
	if w.blockTop != 2 {
		t.Fatalf("two Downs in the blocks: first block %d, want 3rd (index 2)", w.blockTop)
	}
	same := true
	for y := w.blockArea.Y + 2; y < w.blockArea.Y+w.blockArea.H-2 && same; y++ {
		for x := w.blockArea.X + 2; x < w.blockArea.X+w.blockArea.W-2; x++ {
			if before[y*W+x] != after[y*W+x] {
				same = false
				break
			}
		}
	}
	if same {
		t.Error("the blocks' box is the same after it scrolled")
	}
}
