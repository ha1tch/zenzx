//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestGLTiledBackground is a tiled window (several machines) with a part of it that no tile covers:
// that part is the background that is around a picture, the gradient and its five lines, as it is
// for a window with no machine, and not black. Part of dormant guard G-14 (docs/KNOWN_ISSUES.md);
// it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLTiledBackground -v .
func TestGLTiledBackground(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const top = 24
	const mult = 2
	winW, winH := 700, 560+top
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(int32(winW), int32(winH), "ZenZX tiled background")
	defer rl.CloseWindow()

	mem, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	screen.multiplier = mult
	dm := NewDisplayManager(screen)
	dm.SetVideoRenderer(standardVideoRenderer{})
	dm.SetSpectrumIO(io)
	dm.reservedTopHeight = top
	dm.currentWidth, dm.currentHeight = winW, winH
	dm.targetWidth, dm.targetHeight = winW, winH
	dm.InitializeAfterWindow()
	defer dm.CleanupTextures()

	// One tile in the top left corner of the window: 200 by 150, in a colour the background is not.
	tile := rl.Rectangle{X: 0, Y: top, Width: 200, Height: 150}
	dm.setTiles(winW, winH-top, func() { rl.DrawRectangleRec(tile, rl.Magenta) })

	var px []rl.Color
	var w, h int
	for frame := 0; frame < 80; frame++ {
		dm.UpdateWindowSize()
		rl.SetWindowSize(winW, winH)
		if frame == 79 {
			dm.SetPreEndDrawHook(func() {
				rl.DrawRenderBatchActive()
				img := rl.LoadImageFromScreen()
				px = rl.LoadImageColors(img)
				w, h = int(img.Width), int(img.Height)
				if dir := os.Getenv("ZENZX_FRAME_SHOTS"); dir != "" { // to be looked at
					if err := os.MkdirAll(dir, 0o755); err == nil {
						rl.ExportImage(*img, filepath.Join(dir, "tiled-background.png"))
					}
				}
				rl.UnloadImage(img)
			})
		}
		dm.Render(false, mem, screen, io)
	}
	dm.SetPreEndDrawHook(nil)
	if w != winW || h != winH {
		t.Fatalf("setting up: the picture read is %dx%d, want %dx%d", w, h, winW, winH)
	}
	if c := px[(top+50)*w+100]; c.R != 255 || c.G != 0 || c.B != 255 {
		t.Fatalf("the tile is %v: it is not drawn over the background", c)
	}
	grey := func(x, y int) int {
		c := px[y*w+x]
		if c.R != c.G || c.G != c.B {
			t.Errorf("(%d, %d) is %v: the uncovered part is not grey", x, y, c)
		}
		return int(c.R)
	}
	if c := px[(top-1)*w+400]; c.R != 0 || c.G != 0 || c.B != 0 {
		t.Errorf("the bar's band is %v, want black", c)
	}
	if g := grey(600, top); g < frameTopGrey-4 || g > frameTopGrey {
		t.Errorf("under the bar the uncovered part is %d, want %d", g, frameTopGrey)
	}
	if g := grey(600, winH-1); g < frameBottomGrey-2 || g > frameBottomGrey+2 {
		t.Errorf("at the bottom the uncovered part is %d, want %d", g, frameBottomGrey)
	}
	// Right of the tile and under it the same gradient, line for line, and the five lines are there.
	for y := top; y < winH; y++ {
		if a, b := grey(400, y), grey(650, y); a-b > 1 || b-a > 1 {
			t.Fatalf("at y=%d the background is %d at one place and %d at another", y, a, b)
		}
	}
	ly := frameLineYs(top, winH)
	if len(ly) != 5 {
		t.Fatalf("%d lines, want 5", len(ly))
	}
	for i, y := range ly {
		base := grey(650, y-3)
		if dark, light := grey(650, y), grey(650, y+1); dark > base-2 || light < base+15 {
			t.Errorf("line %d: %d over %d over a gradient of %d is not a groove", i+1, dark, light, base)
		}
	}
}
