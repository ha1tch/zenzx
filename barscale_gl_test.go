//go:build !headless

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// TestGLBarScale draws the menu bar at several window widths and in two themes, and looks at
// what comes out. The labels, with a blank space after each, may take 80% of the
// window, and the widths are the bar's own, at its face (Spleen, the default one: 285 pixels at
// X1, 570 at X2 and 855 at X3, so X2 from 713 wide and X3 from 1069). The text has room above and
// below it, half its height (so the strip is 24 high at X1, 32 at X2 and 48 at X3), and the strip
// may take an eighth of the window's height: X2 from 256 high and X3 from 384. The pictures are
// written to ZENZX_BAR_SHOTS, to be looked at.
// Part of dormant guard G-14 (docs/KNOWN_ISSUES.md); it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_BAR_SHOTS=shots go test -run TestGLBarScale -v .
func TestGLBarScale(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	out := os.Getenv("ZENZX_BAR_SHOTS")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(960, 768, "ZenZX bar scale")
	t.Cleanup(rl.CloseWindow) // registered first, so it runs after the bar's text is unloaded

	b, _ := newTestBar(t, nil, "")
	for _, th := range []zenui.ThemeName{zenui.Themes[0], zenui.Themes[len(zenui.Themes)-1]} {
		b.ApplyTheme(th)
		// The widths are the window's, either side of where X2 and X3 begin: the narrowest window
		// in which the labels at that scale take no more than 80% of it.
		x2, x3 := (b.barScaleWidth(2)*5+3)/4, (b.barScaleWidth(3)*5+3)/4
		if x2 >= x3 || x2 < 300 {
			t.Fatalf("setup: X2 begins at %d wide and X3 at %d", x2, x3)
		}
		for _, c := range []struct{ w, h, want int }{
			{x2 - 120, 480, 1}, {x2 - 1, 900, 1}, {x2, 900, 2}, {x3 - 1, 900, 2}, {x3, 900, 3}, {x3 + 600, 900, 3},
			{x2, 255, 1}, {x2, 256, 2}, {x3, 383, 2}, {x3, 384, 3}, // the window's height
		} {
			w, h := c.w, c.h
			rt := rl.LoadRenderTexture(int32(w), int32(h))
			b.state, b.progress = barShown, 1
			rl.BeginTextureMode(rt)
			rl.ClearBackground(rl.Gray)
			b.drawBar(w, h)
			rl.EndTextureMode()
			img := rl.LoadImageFromTexture(rt.Texture)
			rl.ImageFlipVertical(img)
			rl.ExportImage(*img, filepath.Join(out, fmt.Sprintf("bar-%s-%dx%d.png", th, w, h)))
			rl.UnloadImage(img)
			rl.UnloadRenderTexture(rt)

			if got := b.scaleOfBar(); got != c.want {
				t.Errorf("%s, a window %d wide: the bar's text is X%d, want X%d", th, w, got, c.want)
			}
			line := b.renderer.LineHeight(c.want)
			if got, want := b.stripH(), max(menuStripHeight, line+2*barPadFor(line)); got != want {
				t.Errorf("%s, %dx%d: the strip is %d high, want %d", th, w, h, got, want)
			}
			if got := b.stripH(); 2*line > got || got*barHeightDenom > h && c.want > 1 {
				t.Errorf("%s, %dx%d: the strip is %d high for text %d high", th, w, h, got, line)
			}
		}
	}
}
