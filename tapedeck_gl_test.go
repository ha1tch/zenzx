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
	"github.com/ha1tch/zenui/zenuiraylib"
	"github.com/ha1tch/zenzx/pkg/fonts"
)

// TestGLTapeDeck draws the tape deck in a band of the border (top and bottom) and floating over
// the picture, at three window sizes, in several colours of cassette, and checks what was asked
// of it: the panel is on the backdrop, the title is written on the label in the label's
// rectangle (or, when the cassette is too small for it, in the name row), the bar is filled to
// the fraction played and not beyond, the colours of cassette differ, and a button that is on
// (Play while playing) looks different from one that is not. It writes the pictures to the
// folder named by ZENZX_DECK_SHOTS (or a temporary one).
//
// A dormant guard, G-14 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	Xvfb :99 -screen 0 1280x1024x24 &
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_DECK_SHOTS=shots go test -run TestGLTapeDeck -v .
func TestGLTapeDeck(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	out := os.Getenv("ZENZX_DECK_SHOTS")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(960, 768, "ZenZX tape deck")
	defer rl.CloseWindow()
	face, err := fonts.Sinclair()
	if err != nil {
		t.Fatal(err)
	}
	txt := zenuiraylib.NewBDFText(face)
	defer txt.Unload()
	defer func(f func() *zenuiraylib.BDFText) { uiText = f }(uiText)
	uiText = func() *zenuiraylib.BDFText { return txt }

	theme := zenui.DarkTheme()
	border := rl.NewColor(180, 180, 180, 255)
	screen := rl.NewColor(0, 0, 0, 255)

	type scene struct {
		name  string
		w, h  int
		mult  int
		place string // "top", "bottom", "float-top" or "float-bottom"
		size  tapeCassetteSize
	}
	scenes := []scene{
		{"x3-top", 960, 768, 3, "top", cassetteSmall}, {"x3-bottom", 960, 768, 3, "bottom", cassetteSmall},
		{"x2-top", 640, 512, 2, "top", cassetteSmall}, {"x2-bottom", 640, 512, 2, "bottom", cassetteSmall},
		{"x1-float-top", 320, 256, 1, "float-top", cassetteSmall}, {"x1-float-bottom", 320, 256, 1, "float-bottom", cassetteSmall},
		{"x3-float-bottom", 960, 768, 3, "float-bottom", cassetteSmall},
		{"x3-big-float-top", 960, 768, 3, "float-top", cassetteBig},
		{"x3-big-float-bottom", 960, 768, 3, "float-bottom", cassetteBig},
		{"x2-big-float-bottom", 640, 512, 2, "float-bottom", cassetteBig},
		{"x1-big-float-top", 320, 256, 1, "float-top", cassetteBig},
		{"x3-hidden-top", 960, 768, 3, "top", cassetteHidden},
		{"x2-hidden-bottom", 640, 512, 2, "bottom", cassetteHidden},
		{"x1-hidden-float-top", 320, 256, 1, "float-top", cassetteHidden},
		{"x3-hidden-float-bottom", 960, 768, 3, "float-bottom", cassetteHidden},
	}

	grab := func(rt rl.RenderTexture2D, w, h int) *image.RGBA {
		img := rl.LoadImageFromTexture(rt.Texture)
		rl.ImageFlipVertical(img)
		colors := rl.LoadImageColors(img)
		rl.UnloadImage(img)
		if len(colors) != w*h {
			t.Fatalf("read back %d pixels, want %d", len(colors), w*h)
		}
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		for i, p := range colors {
			rgba.Pix[i*4], rgba.Pix[i*4+1], rgba.Pix[i*4+2], rgba.Pix[i*4+3] = p.R, p.G, p.B, 255
		}
		return rgba
	}
	save := func(name string, img *image.RGBA) {
		f, err := os.Create(filepath.Join(out, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	near := func(a, b color.RGBA, tol int) bool {
		d := func(x, y uint8) int {
			if x > y {
				return int(x - y)
			}
			return int(y - x)
		}
		return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
	}
	at := func(img *image.RGBA, x, y int) color.RGBA { return img.RGBAAt(x, y) }

	for _, sc := range scenes {
		rt := rl.LoadRenderTexture(int32(sc.w), int32(sc.h))
		band := 24 * sc.mult
		// The picture: a border band at the top and the bottom, a black screen between.
		var area deckRect
		floatH := clampInt(sc.h*24/100, 56, 128)
		switch sc.size {
		case cassetteBig:
			floatH = clampInt(sc.h*62/100, 120, 460)
		case cassetteHidden:
			floatH = clampInt(sc.h*16/100, 56, 96)
		}
		switch sc.place {
		case "top":
			area = deckRect{0, 2, sc.w, band - 4}
		case "bottom":
			area = deckRect{0, sc.h - band + 2, sc.w, band - 4}
		case "float-top":
			area = deckRect{0, 8, sc.w, floatH}
		default:
			area = deckRect{0, sc.h - floatH - 8, sc.w, floatH}
		}
		scale := 1
		if sc.mult >= 3 {
			scale = 2
		}
		lineH := txt.CellH() * scale

		render := func(variant int, playing bool, frac float64, label string) (*image.RGBA, tapeDeckLayout) {
			u := newTapeDeckUI()
			u.scale = scale
			u.size = sc.size
			u.layout = layoutTapeDeckSized(area, lineH, txt.CellH(), sc.size, sc.place == "bottom" || sc.place == "float-bottom")
			u.shown = u.layout.Panel
			u.view = tapeDeckView{Visible: true, Playing: playing, Fraction: frac, Elapsed: frac * 95, Length: 95,
				Label: label, Variant: variant, Spin: 0.4}
			rl.BeginDrawing()
			rl.BeginTextureMode(rt)
			rl.ClearBackground(border)
			rl.DrawRectangle(0, int32(band), int32(sc.w), int32(sc.h-2*band), screen)
			u.Draw(theme)
			rl.DrawRenderBatchActive()
			rl.EndTextureMode()
			img := grab(rt, sc.w, sc.h)
			rl.EndDrawing()
			return img, u.layout
		}

		img, l := render(0, true, 0.5, "MANIC MINER")
		if l.Panel.empty() || (l.Cassette.empty() && sc.size != cassetteHidden) || l.Bar.empty() {
			t.Errorf("%s: no deck (%+v)", sc.name, l)
			rl.UnloadRenderTexture(rt)
			continue
		}
		save("deck-"+sc.name, img)

		// The panel is on the backdrop: inside its corner, clear of the cassette, it is not the border.
		if p := at(img, l.Panel.X+l.Panel.W-4, l.Panel.Y+l.Panel.H/2); near(p, color.RGBA{R: 180, G: 180, B: 180, A: 255}, 6) {
			t.Errorf("%s: panel not drawn (pixel %v)", sc.name, p)
		}

		// The title: ink on the label's rectangle, or in the name row.
		ink := func(r deckRect, want color.RGBA, tol int) int {
			n := 0
			for y := r.Y; y < r.Y+r.H; y++ {
				for x := r.X; x < r.X+r.W; x++ {
					if near(at(img, x, y), want, tol) {
						n++
					}
				}
			}
			return n
		}
		if sc.size == cassetteBig && !l.LabelOnCassette {
			t.Errorf("%s: a big cassette's label cannot hold the title", sc.name)
		}
		if l.LabelOnCassette {
			if n := ink(l.Label, color.RGBA{R: 22, G: 24, B: 70, A: 255}, 40); n < 20 {
				t.Errorf("%s: %d ink pixels on the label, the title is not written there", sc.name, n)
			}
		} else if !l.Name.empty() {
			tc := theme.Text
			if n := ink(l.Name, color.RGBA{R: tc.R, G: tc.G, B: tc.B, A: 255}, 40); n < 20 {
				t.Errorf("%s: %d text pixels in the name row", sc.name, n)
			}
		}

		// The bar: at half, the quarter point is filled (bright) and the three-quarter point is
		// the dark track.
		y := l.Bar.Y + l.Bar.H/2
		left, right := at(img, l.Bar.X+l.Bar.W/4, y), at(img, l.Bar.X+l.Bar.W*3/4, y)
		if left.R < 150 || right.R > 120 {
			t.Errorf("%s: the bar at half reads %v | %v", sc.name, left, right)
		}
		// And a bar at nothing has no fill; at the end it is full.
		img0, _ := render(0, false, 0, "X")
		imgN, _ := render(0, false, 1, "X")
		if p := at(img0, l.Bar.X+l.Bar.W/4, y); p.R > 120 {
			t.Errorf("%s: an empty bar is filled (%v)", sc.name, p)
		}
		if p := at(imgN, l.Bar.X+l.Bar.W-3, y); p.R < 150 {
			t.Errorf("%s: a full bar is not filled at its end (%v)", sc.name, p)
		}

		// The colours of cassette are different, and the label is the same white on each.
		seen := map[color.RGBA]bool{}
		for v := 0; v < cassetteVariants; v++ {
			if sc.size == cassetteHidden {
				break
			}
			iv, lv := render(v, false, 0.3, "")
			bx := lv.Cassette.X + int(18*float64(lv.Cassette.W)/cassetteW)
			by := lv.Cassette.Y + lv.Cassette.H/2
			c := at(iv, bx, by)
			if seen[c] {
				t.Errorf("%s: variant %d has the body colour of another (%v)", sc.name, v, c)
			}
			seen[c] = true
			lc := at(iv, lv.Label.X+lv.Label.W/2, lv.Label.Y+lv.Label.H/2)
			if !near(lc, color.RGBA{R: 246, G: 246, B: 246, A: 255}, 10) {
				t.Errorf("%s: variant %d label is %v, not paper", sc.name, v, lc)
			}
			if sc.name == "x3-bottom" || sc.name == "x1-float-top" || sc.name == "x3-big-float-bottom" {
				save("deck-"+sc.name+"-"+cassetteNames[v], iv)
			}
		}

		// A button that is on looks different from the same button off.
		on, _ := render(0, true, 0.3, "")
		off, _ := render(0, false, 0.3, "")
		b := l.Buttons[tapePlay]
		diff := 0
		for y := b.Y; y < b.Y+b.H; y++ {
			for x := b.X; x < b.X+b.W; x++ {
				if at(on, x, y) != at(off, x, y) {
					diff++
				}
			}
		}
		if diff == 0 {
			t.Errorf("%s: Play looks the same playing and stopped", sc.name)
		}

		// The corner buttons, where there is room for them, have their dash and cross drawn in
		// the theme's text colour.
		for name, r := range map[string]deckRect{"Minimise": l.Minimise, "Close": l.Close} {
			if r.empty() {
				continue
			}
			tc := theme.Text
			n := 0
			for y := r.Y; y < r.Y+r.H; y++ {
				for x := r.X; x < r.X+r.W; x++ {
					if near(at(on, x, y), color.RGBA{R: tc.R, G: tc.G, B: tc.B, A: 255}, 60) {
						n++
					}
				}
			}
			if n < r.W/2 {
				t.Errorf("%s: %s has %d ink pixels in %+v", sc.name, name, n, r)
			}
		}
		rl.UnloadRenderTexture(rt)
	}
}
