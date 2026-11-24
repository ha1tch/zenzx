//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// TestGLExportForm draws the Export form in a window, in two themes, with and without a reason
// for a refused answer: the explanation, the box for the name, the folder and the three buttons
// are on the screen, and the reason is in the theme's accent colour. Part of dormant guard G-14
// (docs/KNOWN_ISSUES.md); it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_EXPORT_SHOTS=shots go test -run TestGLExportForm -v .
func TestGLExportForm(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	out := os.Getenv("ZENZX_EXPORT_SHOTS")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	rl.SetTraceLogLevel(rl.LogNone)
	const w, h = 960, 640
	rl.InitWindow(w, h, "ZenZX export")
	t.Cleanup(rl.CloseWindow) // registered first, so it runs after the bar's text is unloaded

	b, _ := newTestBar(t, nil, "")
	rt := rl.LoadRenderTexture(w, h)
	defer rl.UnloadRenderTexture(rt)
	for _, th := range []zenui.ThemeName{zenui.Themes[0], zenui.Themes[len(zenui.Themes)-1]} {
		b.ApplyTheme(th)
		for _, why := range []string{"", "A file called game is there already. Export again to replace it."} {
			b.exportAsk = nil
			b.openExportAsk("48k")
			b.exportAsk.dir = "/Users/somebody/Documents/a/rather/long/folder/name/for/a/program"
			b.exportAsk.why = why
			b.exportAsk.build()

			rl.BeginTextureMode(rt)
			rl.ClearBackground(rl.Gray)
			b.exportAsk.draw(b.renderer, w, h, b.theme)
			rl.EndTextureMode()
			img := rl.LoadImageFromTexture(rt.Texture)
			rl.ImageFlipVertical(img)
			px := rl.LoadImageColors(img)

			form := b.exportAsk.modal.Form()
			if bb := form.Bounds(); bb.W < 200 || bb.H < 150 || bb.X < 0 || bb.Y < 0 || bb.X+bb.W > w || bb.Y+bb.H > h {
				t.Errorf("%s: the form is at %+v in a window of %dx%d", th, bb, w, h)
			}
			count := func(r zenui.Rect, c zenui.Colour) (n int) {
				for y := max(r.Y, 0); y < min(r.Y+r.H, h); y++ {
					for x := max(r.X, 0); x < min(r.X+r.W, w); x++ {
						if p := px[y*w+x]; p.R == c.R && p.G == c.G && p.B == c.B {
							n++
						}
					}
				}
				return n
			}
			// The explanation is drawn in the theme's text colour, and the reason, when there is
			// one, in its accent colour: only then.
			if n := count(form.FieldRect(0), b.theme.Text); n == 0 {
				t.Errorf("%s, %q: nothing of the explanation is drawn", th, why)
			}
			for _, i := range b.exportAsk.whyAt {
				if n := count(form.FieldRect(i), b.theme.DirText); n == 0 {
					t.Errorf("%s: the reason (field %d) is not in the accent colour", th, i)
				}
			}
			if (why != "") != (len(b.exportAsk.whyAt) > 0) {
				t.Errorf("%s: reason %q, but %d lines of it", th, why, len(b.exportAsk.whyAt))
			}
			name := "export-" + string(th)
			if why != "" {
				name += "-refused"
			}
			rl.ExportImage(*img, filepath.Join(out, name+".png"))
			rl.UnloadImage(img)
		}
	}
}
