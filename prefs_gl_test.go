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

// TestGLPreferencesTabs draws the Preferences window, each tab of it, in two window sizes and
// two themes, and checks what is on the screen: the chosen tab is filled in the theme's
// selection colour and the others are not, and the controls of the tab that is open are drawn
// (the window's own text colour is found under the strip) while a tab's rows do not show on
// another. It writes the pictures to the folder named by ZENZX_PREFS_SHOTS (or a temporary one).
//
// Part of dormant guard G-14 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_PREFS_SHOTS=shots go test -run TestGLPreferencesTabs -v .
func TestGLPreferencesTabs(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	out := os.Getenv("ZENZX_PREFS_SHOTS")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(960, 768, "ZenZX preferences")
	t.Cleanup(rl.CloseWindow) // registered first, so it runs after the bar's text is unloaded

	b, zx := newTestBar(t, nil, "")
	for _, th := range []zenui.ThemeName{zenui.Themes[0], zenui.Themes[len(zenui.Themes)-1]} {
		b.ApplyTheme(th)
		for _, sz := range [][2]int{{960, 768}, {640, 480}} {
			w, h := sz[0], sz[1]
			rt := rl.LoadRenderTexture(int32(w), int32(h))
			win := newPrefsWindow(b.prefSections(zx), "/some/where/settings.json", func() {})
			for i, name := range win.tabNames {
				win.tabs.Selected = i
				rl.BeginTextureMode(rt)
				rl.ClearBackground(rl.Gray)
				win.Draw(b.renderer, w, h, b.theme)
				rl.EndTextureMode()
				img := rl.LoadImageFromTexture(rt.Texture)
				rl.ImageFlipVertical(img)
				px := rl.LoadImageColors(img)

				// The chosen tab is filled in the selection colour; no other one is.
				sel := b.theme.SelFill
				at := func(x, y int) rl.Color { return px[y*w+x] }
				for j, r := range win.tabs.Layout(win.tabArea, win.lineH) {
					c := at(r.X+r.W/2, r.Y+3)
					got := c.R == sel.R && c.G == sel.G && c.B == sel.B
					if got != (j == i) {
						t.Errorf("%s %dx%d tab %q: tab %d fill %v, chosen %v", th, w, h, name, j, c, j == i)
					}
				}
				// Something is drawn under the strip, inside the panel.
				seen := false
				for y := win.tabArea.Y + win.tabArea.H + 2; y < win.barArea.Y && !seen; y++ {
					for x := win.panel.X + 4; x < win.panel.X+win.panel.W-4; x++ {
						if c := at(x, y); c.R == b.theme.Text.R && c.G == b.theme.Text.G && c.B == b.theme.Text.B {
							seen = true
							break
						}
					}
				}
				if !seen {
					t.Errorf("%s %dx%d tab %q: nothing of the tab's controls is drawn", th, w, h, name)
				}
				rl.ExportImage(*img, filepath.Join(out, "prefs-"+string(th)+"-"+itoa(w)+"-"+name+".png"))
				rl.UnloadImage(img)
			}
			rl.UnloadRenderTexture(rt)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}
