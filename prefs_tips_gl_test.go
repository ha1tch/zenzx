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

// TestGLPreferencesTips draws the Preferences window with the pointer resting on a row's
// caption, then on a group's title, and with it resting on nothing, and checks what is on the
// screen: with nothing under the pointer no help box is there (the pixel where it would be is
// the panel's), and with the pointer at rest the box is there, filled in the window's text
// colour with the text in the panel's. It writes the pictures to the folder named by
// ZENZX_PREFS_SHOTS (or a temporary one).
//
// Part of dormant guard G-14 (docs/KNOWN_ISSUES.md): it opens a window. Run it by itself,
// since a process opens one window.
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_PREFS_SHOTS=shots go test -run TestGLPreferencesTips -v .
func TestGLPreferencesTips(t *testing.T) {
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
	rl.InitWindow(960, 768, "ZenZX preferences tips")
	t.Cleanup(rl.CloseWindow)

	b, zx := newTestBar(t, nil, "")
	for _, th := range []zenui.ThemeName{zenui.Themes[0], zenui.Themes[len(zenui.Themes)-1]} {
		b.ApplyTheme(th)
		for _, sz := range [][2]int{{960, 768}, {640, 480}} {
			w, h := sz[0], sz[1]
			rt := rl.LoadRenderTexture(int32(w), int32(h))
			win := newPrefsWindow(b.prefSections(zx), "/some/where/settings.json", func() {})
			win.tabs.Selected = win.tabIndex("Machines")
			win.layout(b.renderer, w, h)

			shot := func(name string) func(x, y int) rl.Color {
				rl.BeginTextureMode(rt)
				rl.ClearBackground(rl.Gray)
				win.Draw(b.renderer, w, h, b.theme)
				rl.EndTextureMode()
				img := rl.LoadImageFromTexture(rt.Texture)
				rl.ImageFlipVertical(img)
				px := rl.LoadImageColors(img)
				rl.ExportImage(*img, filepath.Join(out, "tip-"+string(th)+"-"+itoa(w)+"-"+name+".png"))
				rl.UnloadImage(img)
				return func(x, y int) rl.Color { return px[y*w+x] }
			}
			rest := func(x, y int) {
				for i := 0; i < 5; i++ {
					win.Update(zenui.Input{MouseX: x, MouseY: y, DeltaTime: 0.1}, b.renderer, w, h)
				}
			}
			same := func(c rl.Color, f zenui.Colour) bool { return c.R == f.R && c.G == f.G && c.B == f.B }

			// Nothing under the pointer: no box.
			rest(0, 0)
			shot("none")
			if win.tipShowing() {
				t.Fatalf("%s %dx%d: a box shows with the pointer on nothing", th, w, h)
			}

			find := func(label string) (int, int) {
				for si, s := range win.sections {
					for ri, row := range s.rows {
						if row.label == label {
							l := win.labels[si][ri]
							return l.X + l.W/2, l.Y + l.H/2
						}
					}
				}
				t.Fatalf("no row %q", label)
				return 0, 0
			}

			// On a caption, and then on a title.
			x, y := find("Tape deck")
			targets := map[string][2]int{"row": {x, y}}
			for si, s := range win.sections {
				if s.title == "Tape" {
					hd := win.heads[si]
					targets["title"] = [2]int{hd.X + 4, hd.Y + hd.H/2}
				}
			}
			for name, p := range targets {
				rest(p[0], p[1])
				atTip := shot(name)
				if !win.tipShowing() {
					t.Fatalf("%s %dx%d %s: no box after the pointer rested", th, w, h, name)
				}
				box, lines, pad := win.tipBox(b.renderer, w, h)
				if box.X < 0 || box.Y < 0 || box.X+box.W > w || box.Y+box.H > h {
					t.Errorf("%s %dx%d %s: the box %+v is not on the screen", th, w, h, name, box)
				}
				if len(lines) == 0 {
					t.Errorf("%s %dx%d %s: the box says nothing", th, w, h, name)
				}
				inBox := atTip(box.X+pad/2+1, box.Y+pad/2+1)
				if !same(inBox, b.theme.Text) {
					t.Errorf("%s %dx%d %s: the box is %v at its corner, want the text colour %v", th, w, h, name, inBox, b.theme.Text)
				}
				// Text is drawn in the panel's colour inside the box.
				seen := false
				for yy := box.Y + pad; yy < box.Y+box.H-pad && !seen; yy++ {
					for xx := box.X + pad; xx < box.X+box.W-pad; xx++ {
						if same(atTip(xx, yy), b.theme.Panel) {
							seen = true
							break
						}
					}
				}
				if !seen {
					t.Errorf("%s %dx%d %s: no text in the box", th, w, h, name)
				}
			}
			rl.UnloadRenderTexture(rt)
		}
	}
}
