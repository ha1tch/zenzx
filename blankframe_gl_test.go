//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestGLBlankBackground is the window of a deck with no machine in it: the background that is
// around a picture, the gradient and its five lines under the bar's band, the same all the way
// across because there is no picture and so no shadow. Part of dormant guard G-14
// (docs/KNOWN_ISSUES.md); it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLBlankBackground -v .
func TestGLBlankBackground(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const top = 24
	const mult = 2
	asked := [2]int{RefWidth * mult, RefHeight*mult + top}
	winW, winH := 700, 560+top
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(int32(asked[0]), int32(asked[1]), "ZenZX blank background")
	defer rl.CloseWindow()

	mem, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	screen.multiplier = mult
	dm := NewDisplayManager(screen)
	dm.SetVideoRenderer(standardVideoRenderer{})
	dm.SetSpectrumIO(io)
	dm.reservedTopHeight = top
	dm.currentWidth, dm.currentHeight = asked[0], asked[1]
	dm.targetWidth, dm.targetHeight = asked[0], asked[1]
	dm.InitializeAfterWindow()
	defer dm.CleanupTextures()

	var px []rl.Color
	var w, h int
	rl.SetWindowSize(winW, winH)
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
						rl.ExportImage(*img, filepath.Join(dir, "blank-background.png"))
					}
				}
				rl.UnloadImage(img)
			})
		}
		dm.RenderBlank()
	}
	dm.SetPreEndDrawHook(nil)
	if w != winW || h != winH {
		t.Fatalf("setting up: the picture read is %dx%d, want %dx%d", w, h, winW, winH)
	}
	grey := func(x, y int) int {
		c := px[y*w+x]
		if c.R != c.G || c.G != c.B {
			t.Errorf("(%d, %d) is %v: the background is not grey", x, y, c)
		}
		return int(c.R)
	}
	// The bar's band is black, and the gradient starts under it and goes darker down the window.
	if c := px[(top-1)*w+5]; c.R != 0 || c.G != 0 || c.B != 0 {
		t.Errorf("the bar's band is %v, want black", c)
	}
	if g := grey(5, top); g < frameTopGrey-4 || g > frameTopGrey {
		t.Errorf("under the bar the background is %d, want %d", g, frameTopGrey)
	}
	if g := grey(5, winH-1); g < frameBottomGrey-2 || g > frameBottomGrey+2 {
		t.Errorf("at the bottom the background is %d, want %d", g, frameBottomGrey)
	}
	// Nothing darkens it: there is no picture to cast a shadow, so a column a quarter of the way
	// across (clear of the figure in the middle) is the column at the edge, line for line.
	for y := top; y < winH; y++ {
		if a, b := grey(5, y), grey(winW/4, y); a-b > 1 || b-a > 1 {
			t.Fatalf("at y=%d the background is %d at the edge and %d a quarter across: a shadow", y, a, b)
		}
	}
	// The waiting figure stands in the middle: something light there (the figure is grey, and the
	// background nowhere near as light), and none of it far out at the sides, a third of the window's height tall at most.
	light := func(x0, x1 int) (n int) {
		for y := top; y < winH; y++ {
			for x := x0; x < x1; x++ {
				if c := px[y*w+x]; c.R > 150 {
					n++
				}
			}
		}
		return n
	}
	if n := light(winW/2-60, winW/2+60); n < 100 {
		t.Errorf("%d light pixels where the waiting figure stands: it is not drawn", n)
	}
	if n := light(0, winW/2-120) + light(winW/2+120, winW); n != 0 {
		t.Errorf("%d light pixels away from the middle: the figure is not alone", n)
	}
	// The five lines are there: a dark row, and a light one under it.
	ly := frameLineYs(top, winH)
	if len(ly) != 5 {
		t.Fatalf("%d lines, want 5", len(ly))
	}
	for i, y := range ly {
		base := grey(5, y-3)
		if dark, light := grey(5, y), grey(5, y+1); dark > base-2 || light < base+15 {
			t.Errorf("line %d: %d over %d over a gradient of %d is not a groove", i+1, dark, light, base)
		}
	}
}
