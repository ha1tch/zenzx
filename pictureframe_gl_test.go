//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestGLPictureFrame is the frame around a centred picture in a window: under the bar it is a
// grey that is 80% (80% of the way from white to black) at the top and goes to 90% at the
// bottom, and the picture's shadow darkens it
// beside the picture. Part of dormant guard G-14 (docs/KNOWN_ISSUES.md); it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_FRAME_SHOTS=shots go test -run TestGLPictureFrame -v .
//
// ZENZX_FRAME_SHOTS names a folder to write the picture it reads to, to be looked at.
func TestGLPictureFrame(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const top = 24
	const mult = 2
	asked := [2]int{RefWidth * mult, RefHeight*mult + top}
	winW, winH := 700, 560+top // the picture, 640 by 512, has 30 each side and 24 above and below
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(int32(asked[0]), int32(asked[1]), "ZenZX picture frame")
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
	dm.borderColor = 2
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
				if dir := os.Getenv("ZENZX_FRAME_SHOTS"); dir != "" {
					if err := os.MkdirAll(dir, 0o755); err == nil {
						rl.ExportImage(*img, filepath.Join(dir, "picture-frame.png"))
					}
				}
				rl.UnloadImage(img)
			})
		}
		dm.Render(false, mem, screen, io)
	}
	dm.SetPreEndDrawHook(nil)
	if dw, dh := drawableSize(); dw != winW || dh != winH || w != winW || h != winH {
		t.Fatalf("setting up: the window is %dx%d (picture read %dx%d), want %dx%d", dw, dh, w, h, winW, winH)
	}
	if !dm.fit.shadow.ready {
		t.Error("the shadow's texture was not made")
	}
	at := func(x, y int) int { // the grey, after checking that it is one
		c := px[y*w+x]
		if c.R != c.G || c.G != c.B {
			t.Errorf("(%d, %d) is %v: the frame is not grey", x, y, c)
		}
		return int(c.R)
	}

	// The gradient, down a column at the left edge, far from the picture and its shadow.
	const col = 2
	ys := []int{top, top + 100, top + 250, top + 400, winH - 1}
	last := 256
	for _, y := range ys {
		g := at(col, y)
		want := frameBottomGrey + (frameTopGrey-frameBottomGrey)*(winH-1-y)/(winH-1-top)
		if g > last {
			t.Errorf("at y=%d the frame is %d, lighter than %d above it: it should darken downwards", y, g, last)
		}
		if g < want-4 || g > want+4 {
			t.Errorf("at y=%d the frame is %d, want about %d on the way from %d to %d", y, g, want, frameTopGrey, frameBottomGrey)
		}
		last = g
	}
	if top := at(col, top); top < frameTopGrey-4 || top > frameTopGrey {
		t.Errorf("under the bar the frame is %d, want %d (80%% grey, from white to black)", top, frameTopGrey)
	}
	if bottom := at(col, winH-1); bottom < frameBottomGrey-2 || bottom > frameBottomGrey+2 {
		t.Errorf("at the bottom the frame is %d, want %d (90%% grey, from white to black)", bottom, frameBottomGrey)
	}

	// The five lines: each a dark row and, under it, a light one, 28% of black and 18%
	// of white laid on the gradient, which is read three rows above for the reference.
	ly := frameLineYs(top, winH)
	if len(ly) != 5 {
		t.Fatalf("%d lines, want 5", len(ly))
	}
	for i, y := range ly {
		base := at(col, y-3)
		dark, light := at(col, y), at(col, y+1)
		if want := base * 72 / 100; dark < want-2 || dark > want+1 || dark > base-2 {
			t.Errorf("line %d: the dark row (y=%d) is %d over a gradient of %d, want about %d", i+1, y, dark, base, want)
		}
		if want := base + (255-base)*18/100; light < want-2 || light > want+2 || light < base+15 {
			t.Errorf("line %d: the light row (y=%d) is %d over a gradient of %d, want about %d", i+1, y+1, light, base, want)
		}
		// Across the whole width, not only at the edge: just left of the picture's shadow.
		if a, b := at(col, y), at(winW-3, y); a-b > 2 || b-a > 2 {
			t.Errorf("line %d is %d at the left and %d at the right", i+1, a, b)
		}
	}

	// The shadow: beside the picture, at half its height, the frame is darker next to it than
	// away from it, and is the gradient again out at the reach of the blur.
	mid := top + (winH-top)/2 - 20 // not on a line: those are checked below
	far, next := at(1, mid), at(30-3, mid)
	if next > far-4 {
		t.Errorf("3 pixels from the picture the frame is %d, and %d at the window's edge: no shadow", next, far)
	}
	// Above the picture the shadow is weaker than below it, for it is dropped.
	above := at(winW/2, top+24-3)
	if ref := at(1, top+24-3); above > ref-4 {
		t.Errorf("above the picture the frame is %d against %d away from it: no shadow", above, ref)
	}
	// The picture itself is not touched by it: its red border is there at the corner.
	if c := px[(top+24)*w+30]; c.R < 150 || c.G > 60 || c.B > 60 {
		t.Errorf("the picture's first pixel is %v, want the red border", c)
	}
}
