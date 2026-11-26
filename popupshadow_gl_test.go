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

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
)

// TestGLPopupMenuHasADropShadowAndTheBarHasNone opens the File menu in a window of the program's
// own and draws the bar twice, with no shadow and with the picture frame's: the strip is the same
// in both (the bar casts none, and the popup's shadow does not fall on it), the panel is the same,
// and the ground beside and below the panel is darker with the shadow.
//
// Part of dormant guard G-27 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLPopupMenu -v .
func TestGLPopupMenuHasADropShadowAndTheBarHasNone(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-27)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(800, 600, "ZenZX popup shadow")
	t.Cleanup(rl.CloseWindow)
	w, h := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())

	b, zx := newTestBar(t, nil, "")
	dm := &DisplayManager{}
	t.Cleanup(dm.unloadFrameShadow) // registered last, so it runs before the window closes
	ground := color.RGBA{210, 140, 60, 255}
	b.state, b.progress = barShown, 1

	frame := func() []color.RGBA {
		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(ground.R, ground.G, ground.B, ground.A))
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
	lum := func(c color.RGBA) int { return int(c.R) + int(c.G) + int(c.B) }

	b.Update(zx) // the rows of the menus
	frame()      // lays the labels out
	// hovering a label opens its menu
	b.widget.Update(zenui.Input{MouseX: 20, MouseY: b.stripH() / 2})
	if !b.widget.Active() {
		t.Logf("state %v progress %v strip %d barY %d items %d", b.state, b.progress, b.stripH(), b.barY(), len(b.widget.ItemsFor(0)))
		t.Fatal("the File menu did not open")
	}
	plain := frame()

	// the panel, from the row a little under the strip: the first and last pixels that are not the ground
	strip := b.stripH()
	row := strip + 24
	left, right := -1, -1
	for x := 0; x < w; x++ {
		if at(plain, x, row) != ground {
			if left < 0 {
				left = x
			}
			right = x
		}
	}
	if left < 0 || right-left < 40 {
		t.Fatalf("no panel on row %d (%d to %d)", row, left, right)
	}
	bottom := -1
	for y := strip; y < h; y++ {
		if at(plain, left+8, y) != ground {
			bottom = y
		}
	}
	if bottom < row {
		t.Fatalf("no bottom to the panel (%d)", bottom)
	}

	b.popupShadow = func(box rl.Rectangle) { drawBoxShadow(dm.shadowTexture(), box) }
	shadowed := frame()

	// the strip and the panel are as they were
	for y := 0; y < strip; y++ {
		for x := 0; x < w; x++ {
			if at(plain, x, y) != at(shadowed, x, y) {
				t.Fatalf("the strip changed at (%d,%d): %v, was %v", x, y, at(shadowed, x, y), at(plain, x, y))
			}
		}
	}
	for _, p := range [][2]int{{left + 3, row}, {(left + right) / 2, row}, {right - 3, row}, {left + 8, bottom - 2}} {
		if at(plain, p[0], p[1]) != at(shadowed, p[0], p[1]) {
			t.Errorf("the panel changed at %v: %v, was %v", p, at(shadowed, p[0], p[1]), at(plain, p[0], p[1]))
		}
	}
	// the ground beside the panel (its right edge, the middle of its height) and under it is darker
	for name, p := range map[string][2]int{
		"beside": {right + 6, row},
		"below":  {(left + right) / 2, bottom + 8},
	} {
		if at(plain, p[0], p[1]) != ground {
			t.Fatalf("%s: (%d,%d) is %v with no shadow, want the ground", name, p[0], p[1], at(plain, p[0], p[1]))
		}
		if got := at(shadowed, p[0], p[1]); lum(got) > lum(ground)-60 {
			t.Errorf("%s: (%d,%d) is %v with the shadow, want it much darker than the ground %v", name, p[0], p[1], got, ground)
		}
	}
	// and far from it, nothing
	if got := at(shadowed, w-4, h-4); got != ground {
		t.Errorf("the far corner is %v, want the ground", got)
	}

	if dir := os.Getenv("ZENZX_FADE_SHOTS"); dir != "" { // the picture, to be looked at
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for i, c := range shadowed {
			img.SetRGBA(i%w, i/w, c)
		}
		if f, err := os.Create(filepath.Join(dir, "popupshadow.png")); err == nil {
			png.Encode(f, img)
			f.Close()
		}
	}
}
