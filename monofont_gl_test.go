//go:build !headless

package zenzx

import (
	"os"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"

	"github.com/ha1tch/zenzx/pkg/fonts"
)

// TestGLHexDumpFontFollowsTheChosenFont opens a window and, for each face the program has, asks for
// the renderer of the hex dumps: the face itself when it is monospaced, and when it is not (Tom
// Thumb) the monospaced face whose cell is the closest in size. The cell is measured through the
// real glyph renderer. Then it draws the Hot Memory tab with each face as the program's font, and
// checks that the dump has ink in it.
//
// Part of dormant guard G-29 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLHexDump -v .
func TestGLHexDumpFontFollowsTheChosenFont(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-29)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(1000, 700, "ZenZX hex dump font")
	t.Cleanup(rl.CloseWindow)
	w, h := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())

	b, zx := newTestBar(t, nil, "")
	t.Cleanup(func() {
		if b.monoText != nil {
			b.monoText.Unload() // before the window closes
		}
	})
	theme := b.theme

	cell := func(name fonts.Name) (int, int) {
		f, err := fonts.Load(name)
		if err != nil {
			t.Fatal(err)
		}
		return f.CellWidth, f.CellHeight
	}
	for _, name := range fonts.All {
		if err := b.ApplyFont(name); err != nil { // as the Font menu does: the face and its name together
			t.Fatalf("%s: %v", name, err)
		}
		r := b.monoRenderer()
		want := fonts.NearestMonospaced(name)
		wantW, wantH := cell(want)
		if got := r.TextWidth("0", 1); got != wantW {
			t.Errorf("%s: the dump's characters are %d wide, want %d (%s)", name, got, wantW, want)
		}
		if got := r.LineHeight(1); got < wantH {
			t.Errorf("%s: the dump's lines are %d high, want at least %d (%s)", name, got, wantH, want)
		}
		if fonts.Monospaced(name) && want != name {
			t.Errorf("%s is monospaced and was not kept: %s", name, want)
		}
		if name == fonts.NameTomThumb && want == name {
			t.Error("Tom Thumb is not monospaced, and was used for the dump")
		}

		// the window drawn with it: the dump has ink in it
		win := newBinaryWindow(zx, binTabHot)
		win.mono = r
		win.Update(zenui.Input{}, b.renderer, w, h)
		win.hot.goTo(0x4000)
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		win.Draw(b.renderer, w, h, theme)
		rl.DrawRenderBatchActive()
		img := rl.LoadImageFromScreen()
		rl.EndDrawing()
		cols := rl.LoadImageColors(img)
		rl.UnloadImage(img)
		g := win.hot.gridRect
		distinct := map[[3]uint8]bool{}
		for y := g.Y + 2; y < g.Y+g.H-2; y++ {
			for x := g.X + 2; x < g.X+g.W-2; x++ {
				c := cols[y*w+x]
				distinct[[3]uint8{c.R, c.G, c.B}] = true
			}
		}
		if len(distinct) < 3 {
			t.Errorf("%s: the dump drew %d colours: nothing is in it", name, len(distinct))
		}
	}
}
