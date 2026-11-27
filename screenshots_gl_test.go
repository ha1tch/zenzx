//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// stripes is a picture that has many colours across it, w by h.
func stripes(w, h int, shift int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*img.Stride + x*4
			img.Pix[i] = uint8((x + shift) * 255 / w)
			img.Pix[i+1] = uint8(y * 255 / h)
			img.Pix[i+2] = uint8(((x+y)/8%2)*200 + 20)
			img.Pix[i+3] = 0xFF
		}
	}
	return img
}

// TestGLScreenshotsWindowDrawsThePictures draws the Screenshots window in a real window, as the
// grid and as rows, and the preview of a GIF and of a SCR: each thumbnail has the colours of its
// picture, the preview has the picture and its caption. Dormant guard G-34 (docs/KNOWN_ISSUES.md);
// it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_SHOTS_OUT=shots go test -run TestGLScreenshotsWindow -v .
func TestGLScreenshotsWindowDrawsThePictures(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-34)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	out := os.Getenv("ZENZX_SHOTS_OUT")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	rl.SetTraceLogLevel(rl.LogNone)
	const W, H = 1280, 800
	rl.InitWindow(W, H, "ZenZX screenshots")
	t.Cleanup(rl.CloseWindow) // registered first, so it runs after the bar's text is unloaded

	dir := filepath.Join(t.TempDir(), "a-folder-with-a-very-long-name-that-goes-on-and-on-and-on-and-on", "screenshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, stripes(352, 296, 0))
	gifData, err := encodeGIF([]*image.RGBA{stripes(256, 192, 0), stripes(256, 192, 90)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	scrData := make([]byte, 6912)
	for i := 0; i < 6144; i++ {
		scrData[i] = byte(i * 37)
	}
	for i := 0; i < 768; i++ {
		scrData[6144+i] = byte(i%7+1) | byte(i/32%8)<<3
	}
	nxiData := make([]byte, 49152)
	for i := range nxiData {
		nxiData[i] = byte(i%256) ^ byte(i/256)
	}
	timexData := make([]byte, 12288)
	for i := range timexData {
		timexData[i] = byte(i * 13)
	}
	hiColData := append([]byte{}, timexData...) // a bitmap, then a few attributes
	for i := 6144; i < len(hiColData); i++ {
		hiColData[i] = byte(0x38 + i%5)
	}
	for name, data := range map[string][]byte{
		"screenshot_Manic-Miner_20201127_093000.png":   pngBuf.Bytes(),
		"screenshot_Jet-Set-Willy_20201127_093100.gif": gifData,
		"screenshot_Horace_20201127_093200.scr":        scrData,
		"screenshot_Timex_20201127_093600.hicol.scr":   hiColData,
		"screenshot_Next_20201127_093300.nxi":          nxiData,
		"screenshot_Timex_20201127_093500.hires.scr":   timexData,
		"broken.png": []byte("not a png"),
		"screenshot_A-machine-with-an-extremely-long-name-indeed_20201127_093400.png": pngBuf.Bytes(),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b, _ := newTestBar(t, nil, "")
	rt := rl.LoadRenderTexture(W, H)
	defer rl.UnloadRenderTexture(rt)
	w := newScreenshotsWindow(dir)
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
	frames := func(n int) {
		for i := 0; i < n; i++ {
			w.Update(zenui.Input{DeltaTime: 0.02}, b.renderer, W, H)
			draw("scratch")
		}
	}
	// distinct counts the colours in a rectangle of the picture just drawn.
	distinct := func(px []color.RGBA, r zenui.Rect) int {
		seen := map[color.RGBA]bool{}
		for y := max(r.Y, 0); y < min(r.Y+r.H, H); y++ {
			for x := max(r.X, 0); x < min(r.X+r.W, W); x++ {
				seen[px[y*W+x]] = true
			}
		}
		return len(seen)
	}
	// inkBeside counts the pixels to the right of the panel that are not what the screen is
	// there: text that ran out of the dialog.
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

	// visible says whether a thumbnail is inside the list as it is drawn.
	visible := func(r zenui.Rect) bool {
		a := w.listArea
		return r.X >= a.X && r.Y >= a.Y && r.X+r.W <= a.X+a.W && r.Y+r.H <= a.Y+a.H
	}

	// The grid. Two thumbnails are made a frame: a few frames and they are all there.
	frames(6)
	px := draw("screenshots-grid")
	for i, f := range w.all {
		if !visible(thumbRect(i)) { // a third row of the grid is below the list until it scrolls
			continue
		}
		n := distinct(px, thumbRect(i))
		want := 8 // a picture has many colours; a broken file's "?" has the black and the grey of its text
		if f.Name == "broken.png" {
			want = 1
		}
		if n < want {
			t.Errorf("grid: %s has %d colours in its thumbnail, want at least %d", f.Name, n, want)
		}
		if f.Name == "broken.png" && n > 4 {
			t.Errorf("grid: the broken file's thumbnail has %d colours: it should be black with a mark", n)
		}
	}

	if n := inkBeside(px, w.panel); n > 0 {
		t.Errorf("grid: %d pixels of ink are beyond the right edge of the dialog", n)
	}

	// Rows.
	w.do(shotsBtnLayout)
	frames(2)
	px = draw("screenshots-rows")
	for i, f := range w.all {
		if f.Name == "broken.png" || !visible(thumbRect(i)) {
			continue
		}
		if n := distinct(px, thumbRect(i)); n < 4 {
			t.Errorf("rows: %s has %d colours in its thumbnail, want at least 4", f.Name, n)
		}
	}
	if n := inkBeside(px, w.panel); n > 0 {
		t.Errorf("rows: %d pixels of ink are beyond the right edge of the dialog", n)
	}

	// The preview of the GIF, in each of its two frames, and of the SCR.
	for _, c := range []struct{ name, shot string }{{"Jet-Set-Willy_20201127_093100.gif", "preview-gif"}, {"Horace_20201127_093200.scr", "preview-scr"}} {
		w.closePreview()
		for i, f := range w.all {
			if filepath.Base(f.Path) == "screenshot_"+c.name {
				w.openPreview(i)
			}
		}
		if w.view == nil || w.view.err != nil {
			t.Fatalf("%s: the preview is %+v", c.name, w.view)
		}
		px = draw(c.shot + "-0")
		for y := 0; y < H; y++ { // the caption, however long, stays inside the margin
			for x := W - 30; x < W; x++ {
				if px[y*W+x] != px[y*W+W-1] {
					t.Fatalf("%s: ink at %d,%d, beyond the preview's right margin", c.name, x, y)
				}
			}
		}
		mid := zenui.Rect{X: W/2 - 40, Y: H/2 - 40, W: 80, H: 80}
		if n := distinct(px, mid); n < 4 {
			t.Errorf("%s: the middle of the preview has %d colours", c.name, n)
		}
		if w.view.pic.Animated() {
			before := distinct(px, mid)
			w.Update(zenui.Input{DeltaTime: 0.05}, b.renderer, W, H)
			px2 := draw(c.shot + "-1")
			if w.view.frame != 1 {
				t.Errorf("%s: frame %d after 0.05 s", c.name, w.view.frame)
			}
			if px2[(H/2)*W+W/2] == px[(H/2)*W+W/2] && before > 0 && distinct(px2, mid) == before {
				t.Logf("%s: the middle pixel did not change between the frames (the pictures may agree there)", c.name)
			}
		}
	}
	w.closePreview()
}
