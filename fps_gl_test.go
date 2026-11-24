//go:build !headless

package zenzx

import (
	"image/color"
	"os"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui/zenuiraylib"
	"github.com/ha1tch/zenzx/pkg/fonts"
)

// TestGLFPSCounter checks what the FPS counter is now: hidden until it is asked for, and drawn
// in the interface font (the BDF text renderer the front end names with uiText), at the
// bottom right of the window, and not in raylib's own font. Part of dormant guard G-14
// (docs/KNOWN_ISSUES.md), with TestGLToastOverTheDemos; it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLFPSCounter -v .
func TestGLFPSCounter(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const mult = 2
	const winW, winH = RefWidth * mult, RefHeight * mult
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(winW, winH, "ZenZX FPS")
	defer rl.CloseWindow()
	face, err := fonts.Sinclair()
	if err != nil {
		t.Fatal(err)
	}
	txt := zenuiraylib.NewBDFText(face)
	defer txt.Unload()
	asked := 0
	uiText = func() *zenuiraylib.BDFText { asked++; return txt }
	defer func() { uiText = nil }()

	mem, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	screen.multiplier = mult
	dm := NewDisplayManager(screen)
	dm.SetVideoRenderer(standardVideoRenderer{})
	dm.SetSpectrumIO(io)
	dm.currentWidth, dm.currentHeight = winW, winH
	dm.InitializeAfterWindow()
	defer dm.CleanupTextures()

	if dm.showFPS {
		t.Fatal("the FPS counter is on in a new display manager; it is a diagnostic and starts hidden")
	}
	// The red pixels of a frame in the bottom-right corner, where the counter goes.
	reds := func() int {
		n := 0
		dm.SetPreEndDrawHook(func() {
			rl.DrawRenderBatchActive()
			img := rl.LoadImageFromScreen()
			px := rl.LoadImageColors(img)
			rl.UnloadImage(img)
			for y := winH - 40; y < winH; y++ {
				for x := winW - 200; x < winW; x++ {
					if px[y*winW+x] == (color.RGBA{rl.Red.R, rl.Red.G, rl.Red.B, 255}) {
						n++
					}
				}
			}
		})
		dm.Render(false, mem, screen, io)
		dm.SetPreEndDrawHook(nil)
		return n
	}
	if n := reds(); n != 0 || asked != 0 {
		t.Fatalf("hidden, yet %d red pixels and the font asked for %d times", n, asked)
	}
	dm.showFPS = true
	if n := reds(); n == 0 || asked == 0 {
		t.Fatalf("shown, yet %d red pixels and the font asked for %d times", n, asked)
	}
}
