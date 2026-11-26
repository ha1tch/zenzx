//go:build !headless

package zenzx

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestGLDialogFadesAndShrinksWhenClosed closes the Help window in a window of the program's own and
// draws the frames after it: the first is the window as it was, with nothing taken away; half way
// the backdrop is half as dark and the window is fainter and smaller (a pixel just inside its edge
// is no longer the panel's); and at the end nothing of it is left, the picture being what it is
// with no window at all.
//
// Part of dormant guard G-27 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLDialogFades -v .
func TestGLDialogFadesAndShrinksWhenClosed(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-27)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(800, 600, "ZenZX dialog fade")
	t.Cleanup(rl.CloseWindow) // registered first, so it runs after the bar's text is unloaded
	w, h := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())

	b, _ := newTestBar(t, nil, "")
	ground := rl.NewColor(210, 140, 60, 255)
	clock := time.Unix(1000, 0)
	b.fade.now = func() time.Time { return clock }

	// frame draws the bar, with the window or the fade there is, over the ground, and gives the picture
	frame := func() []color.RGBA {
		rl.BeginDrawing()
		rl.ClearBackground(ground)
		b.Draw(w, h)
		rl.DrawRenderBatchActive()
		img := rl.LoadImageFromScreen()
		rl.EndDrawing()
		cols := rl.LoadImageColors(img)
		rl.UnloadImage(img)
		out := make([]color.RGBA, len(cols))
		for i, c := range cols {
			out[i] = color.RGBA{c.R, c.G, c.B, c.A}
		}
		return out
	}
	at := func(p []color.RGBA, x, y int) color.RGBA { return p[y*w+x] }
	near := func(a, b color.RGBA, tol int) bool {
		d := func(x, y uint8) int {
			if x > y {
				return int(x - y)
			}
			return int(y - x)
		}
		return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
	}

	b.activeModal = newHelpModal()
	open := frame()
	b.activeModal = nil
	// the window is darker over the whole screen, and its panel is told from the backdrop by the row
	// through the middle: the panel's left edge is the first pixel that is not the backdrop's
	corner, mid := at(open, 3, h-3), h/2
	if near(corner, color.RGBA{210, 140, 60, 255}, 4) {
		t.Fatalf("the open window has no backdrop: the corner is %v", corner)
	}
	left := 0
	for x := 0; x < w/2; x++ {
		if !near(at(open, x, mid), corner, 2) {
			left = x
			break
		}
	}
	if left < 4 {
		t.Fatalf("no panel found on the middle row (left edge %d)", left)
	}
	inside := at(open, left+2, mid) // just inside the panel's left edge

	// the frame after it closed: nothing is taken away yet
	first := frame()
	if !near(at(first, 3, h-3), corner, 3) || !near(at(first, left+2, mid), inside, 3) {
		t.Errorf("at the start of the fade the corner is %v (was %v) and the panel's edge %v (was %v)",
			at(first, 3, h-3), corner, at(first, left+2, mid), inside)
	}

	// half way
	clock = clock.Add(dialogFadeTime / 2)
	half := frame()
	ch := at(half, 3, h-3)
	if !(ch.R > corner.R && ch.R < 210) {
		t.Errorf("half way the corner is %v: want it between the open window's %v and the ground", ch, corner)
	}
	// shrunk, the window's edge has moved in past the pixel that was just inside it: that pixel is
	// now the backdrop's, as the one at the screen's edge on the same row is
	if !near(at(half, left+2, mid), at(half, 3, mid), 3) {
		t.Errorf("half way the pixel just inside the panel's old edge is %v, want the backdrop's %v: the window has not shrunk",
			at(half, left+2, mid), at(half, 3, mid))
	}
	if got := at(half, w/2, h/2); near(got, at(open, w/2, h/2), 0) {
		t.Errorf("half way the middle of the window is as it was: %v", got)
	}

	if dir := os.Getenv("ZENZX_FADE_SHOTS"); dir != "" { // the pictures, to be looked at
		for name, p := range map[string][]color.RGBA{"open": open, "start": first, "half": half} {
			img := image.NewRGBA(image.Rect(0, 0, w, h))
			for i, c := range p {
				img.SetRGBA(i%w, i/w, c)
			}
			if f, err := os.Create(filepath.Join(dir, "dialogfade-"+name+".png")); err == nil {
				png.Encode(f, img)
				f.Close()
			}
		}
	}

	// the end: what a frame with no window has
	clock = clock.Add(dialogFadeTime)
	end := frame()
	if b.fade.have {
		t.Error("the fade has not ended")
	}
	for _, p := range [][2]int{{3, h - 3}, {left + 2, mid}, {w / 2, h / 2}, {w - 3, h - 3}} {
		if got := at(end, p[0], p[1]); !near(got, color.RGBA{210, 140, 60, 255}, 1) {
			t.Errorf("at the end of the fade (%d,%d) is %v, want the ground", p[0], p[1], got)
		}
	}

	// a window opened during the fade ends it
	b.activeModal = newHelpModal()
	frame()
	b.activeModal = nil
	frame()
	b.activeModal = newHelpModal()
	frame()
	if b.fade.have {
		t.Error("a window that opened during a fade left it going")
	}
}

// The drop-choice dialog is drawn by the program and not by the bar, and fades the same way: held
// whole while it is open (the bar captures nothing), then fainter, then gone.
//
// Part of dormant guard G-27.
func TestGLDropChoiceFadesToo(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-27)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(800, 600, "ZenZX drop-choice fade")
	t.Cleanup(rl.CloseWindow)
	w, h := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())

	b, _ := newTestBar(t, nil, "")
	dropC := &dropChoice{}
	clock := time.Unix(1000, 0)
	b.fade.now = func() time.Time { return clock }
	ground := color.RGBA{210, 140, 60, 255}

	// corner is the pixel at the bottom left of the window, which only the backdrop reaches
	corner := func() color.RGBA {
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(ground.R, ground.G, ground.B, 255))
		b.watchDropChoice(dropC, w, h) // as the program does it, in this order
		b.Draw(w, h)
		dropC.Draw(b.renderer, w, h, b.theme)
		rl.DrawRenderBatchActive()
		img := rl.LoadImageFromScreen()
		rl.EndDrawing()
		c := rl.GetImageColor(*img, 3, int32(h-3))
		rl.UnloadImage(img)
		return color.RGBA{c.R, c.G, c.B, c.A}
	}

	dropC.open([]string{"game.tap"}, 0)
	open := corner()
	if open == ground {
		t.Fatalf("the open dialog has no backdrop: the corner is %v", open)
	}
	if again := corner(); again != open || !dropC.Active() {
		t.Fatalf("an open dialog changed from one frame to the next: %v then %v", open, again)
	}
	if b.fade.have {
		t.Fatal("a fade started while the dialog was open")
	}
	dropC.close()
	if first := corner(); first != open {
		t.Errorf("the frame after it closed is %v, want the dialog as it was (%v)", first, open)
	}
	clock = clock.Add(dialogFadeTime / 2)
	if half := corner(); !(half.R > open.R && half.R < ground.R) {
		t.Errorf("half way the corner is %v: want it between the dialog's %v and the ground", half, open)
	}
	clock = clock.Add(dialogFadeTime)
	if end := corner(); end != ground || b.fade.have {
		t.Errorf("at the end the corner is %v (fade going %v), want the ground", end, b.fade.have)
	}
}
