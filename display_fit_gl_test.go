//go:build !headless

package zenzx

import (
	"os"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestGLPictureFitsAWindowOfAnotherSize is T-75 in a window: the window is held at a size other
// than the one the display asked for, every frame, as a system that will not resize it does, and
// the picture must be whole and in the middle of it with the same margin on both sides. The
// border is red so that the picture's outer edge can be found. Part of dormant guard G-14
// (docs/KNOWN_ISSUES.md); it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLPictureFits -v .
func TestGLPictureFitsAWindowOfAnotherSize(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const top = 24
	const mult = 2
	asked := [2]int{RefWidth * mult, RefHeight*mult + top} // 640 by 536
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(int32(asked[0]), int32(asked[1]), "ZenZX picture fit")
	defer rl.CloseWindow()

	// The textures go before the window does (deferred after CloseWindow's, so run before it).
	var managers []*DisplayManager
	defer func() {
		for _, m := range managers {
			m.CleanupTextures()
		}
	}()

	mem, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)

	// run holds the window at winW by winH for a while, drawing every frame as the program does,
	// and returns the bounding box of the red border in the last one.
	run := func(t *testing.T, winW, winH int) (box [4]int, dm *DisplayManager) {
		t.Helper()
		screen.multiplier = mult
		dm = NewDisplayManager(screen)
		dm.SetVideoRenderer(standardVideoRenderer{})
		dm.SetSpectrumIO(io)
		dm.reservedTopHeight = top
		dm.currentWidth, dm.currentHeight = asked[0], asked[1]
		dm.targetWidth, dm.targetHeight = asked[0], asked[1]
		dm.borderColor = 2
		dm.InitializeAfterWindow()
		managers = append(managers, dm)
		if !dm.borderView().Shown {
			t.Fatal("setting up: the border is not shown")
		}
		rl.SetWindowSize(winW, winH)
		for frame := 0; frame < 80; frame++ {
			dm.UpdateWindowSize()
			rl.SetWindowSize(winW, winH) // the system keeps the window at this size
			last := frame == 79
			if last {
				dm.SetPreEndDrawHook(func() {
					rl.DrawRenderBatchActive()
					img := rl.LoadImageFromScreen()
					px := rl.LoadImageColors(img)
					w, h := int(img.Width), int(img.Height)
					rl.UnloadImage(img)
					box = [4]int{w, h, -1, -1} // minX, minY, maxX, maxY as found
					for y := 0; y < h; y++ {
						for x := 0; x < w; x++ {
							if c := px[y*w+x]; c.R > 150 && c.G < 60 && c.B < 60 {
								if x < box[0] {
									box[0] = x
								}
								if y < box[1] {
									box[1] = y
								}
								if x > box[2] {
									box[2] = x
								}
								if y > box[3] {
									box[3] = y
								}
							}
						}
					}
				})
			}
			dm.Render(false, mem, screen, io)
		}
		dm.SetPreEndDrawHook(nil)
		if w, h := drawableSize(); w != winW || h != winH {
			t.Fatalf("setting up: the window is %dx%d, want it held at %dx%d", w, h, winW, winH)
		}
		return box, dm
	}
	near := func(a, b, tol int) bool { return a >= b-tol && a <= b+tol }
	// The cases run on this goroutine, not in t.Run's: the window's GL context belongs to the
	// thread this test locked, and another goroutine would draw nothing.
	sub := func(name string, f func(t *testing.T)) {
		t.Helper()
		t.Logf("case: %s", name)
		f(t)
	}

	sub("the window is the size asked for: nothing changes", func(t *testing.T) {
		box, dm := run(t, asked[0], asked[1])
		if dm.fit.canvasReady {
			t.Error("the picture went through the canvas though the window is the size the display asked for")
		}
		if box[0] != 0 || box[2] != asked[0]-1 || box[1] != top || box[3] != asked[1]-1 {
			t.Errorf("the picture is at %v, want it over the whole window under the bar (0, %d)-(%d, %d)", box, top, asked[0]-1, asked[1]-1)
		}
	})

	sub("a window a little larger: the picture stays and is in the middle", func(t *testing.T) {
		box, dm := run(t, 700, 560+top)
		if dm.screen.multiplier != 2 {
			t.Errorf("the zoom is %d, want 2 (it fits)", dm.screen.multiplier)
		}
		// 640 by 512 in 700 by 560 under the bar: 30 each side, 24 above and below.
		if box[0] != 30 || box[2] != 700-30-1 || box[1] != top+24 || box[3] != top+24+512-1 {
			t.Errorf("the picture is at %v, want (30, %d)-(669, %d)", box, top+24, top+24+511)
		}
	})

	sub("a window too small for the zoom: the zoom that fits, in the middle", func(t *testing.T) {
		box, dm := run(t, 400, 300+top)
		if dm.screen.multiplier != 1 {
			t.Errorf("the zoom is %d, want 1", dm.screen.multiplier)
		}
		// 320 by 256 in 400 by 300 under the bar: 40 each side, 22 above and below.
		if box[0] != 40 || box[2] != 400-40-1 || box[1] != top+22 || box[3] != top+22+256-1 {
			t.Errorf("the picture is at %v, want (40, %d)-(359, %d)", box, top+22, top+22+255)
		}
	})

	sub("a window smaller than the picture at its least: scaled down, whole", func(t *testing.T) {
		box, _ := run(t, 200, 150+top)
		w, h := box[2]-box[0]+1, box[3]-box[1]+1
		if box[0] < 0 || box[2] > 199 || box[1] < top || box[3] > 150+top-1 {
			t.Errorf("the picture is cropped: %v in a window 200 by %d under a bar of %d", box, 150+top, top)
		}
		// 150/256 is the smaller factor: 187 by 150.
		if !near(w, 187, 3) || !near(h, 150, 3) {
			t.Errorf("the picture is %d by %d, want about 187 by 150 (kept in proportion)", w, h)
		}
		if left, right := box[0], 199-box[2]; !near(left, right, 2) {
			t.Errorf("margins left %d, right %d", left, right)
		}
	})
}
