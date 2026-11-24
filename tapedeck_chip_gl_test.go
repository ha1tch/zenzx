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

// TestGLTapeDeckChip draws the tape deck as the chip at the top and the bottom of a window at
// three sizes and checks what was asked of it: the chip is on the backdrop at the right end, the
// mini cassette is drawn, the play button is a pause button while the tape plays and looks
// different from the stopped one, the counter is written, the close cross is drawn and the
// hairline of progress is filled to the fraction played. It writes the pictures to the folder
// named by ZENZX_DECK_SHOTS (or a temporary one).
//
// Part of dormant guard G-17 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_DECK_SHOTS=shots go test -run TestGLTapeDeckChip -v .
func TestGLTapeDeckChip(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-17)")
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
	rl.InitWindow(960, 768, "ZenZX tape deck chip")
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
	backdrop := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	type scene struct {
		name   string
		w, h   int
		mult   int
		bottom bool
		float  bool
	}
	scenes := []scene{
		{"x3-top", 960, 768, 3, false, false}, {"x3-bottom", 960, 768, 3, true, false},
		{"x2-top", 640, 512, 2, false, false}, {"x2-bottom", 640, 512, 2, true, false},
		{"x1-float-top", 320, 256, 1, false, true}, {"x1-float-bottom", 320, 256, 1, true, true},
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
	for _, sc := range scenes {
		rt := rl.LoadRenderTexture(int32(sc.w), int32(sc.h))
		band := 24 * sc.mult
		floatH := clampInt(sc.h*16/100, 56, 96)
		var area deckRect
		switch {
		case !sc.float && !sc.bottom:
			area = deckRect{0, 2, sc.w, band - 4}
		case !sc.float:
			area = deckRect{0, sc.h - band + 2, sc.w, band - 4}
		case !sc.bottom:
			area = deckRect{0, 8, sc.w, floatH}
		default:
			area = deckRect{0, sc.h - floatH - 8, sc.w, floatH}
		}
		scale := 1
		if sc.mult >= 3 {
			scale = 2
		}
		render := func(playing bool, frac float64) (*image.RGBA, deckChipLayout) {
			u := newTapeDeckUI()
			u.scale = scale
			u.chip = layoutDeckChip(area, txt.CellH()*scale, txt.Measure("00:00 / 00:00", scale), sc.bottom)
			u.shown, u.chipStyle = u.chip.Panel, true
			u.view = tapeDeckView{Visible: true, Mode: deckChip, Form: deckChip, Playing: playing, Fraction: frac,
				Elapsed: frac * 95, Length: 95, Variant: 0, Spin: 0.4}
			rl.BeginDrawing()
			rl.BeginTextureMode(rt)
			rl.ClearBackground(rl.NewColor(180, 180, 180, 255))
			rl.DrawRectangle(0, int32(band), int32(sc.w), int32(sc.h-2*band), rl.Black)
			u.Draw(theme)
			rl.DrawRenderBatchActive()
			rl.EndTextureMode()
			img := rl.LoadImageFromTexture(rt.Texture)
			rl.ImageFlipVertical(img)
			colors := rl.LoadImageColors(img)
			rl.UnloadImage(img)
			rgba := image.NewRGBA(image.Rect(0, 0, sc.w, sc.h))
			for i, p := range colors {
				rgba.Pix[i*4], rgba.Pix[i*4+1], rgba.Pix[i*4+2], rgba.Pix[i*4+3] = p.R, p.G, p.B, 255
			}
			rl.EndDrawing()
			return rgba, u.chip
		}
		img, l := render(false, 0.5)
		if l.Panel.empty() || l.Cassette.empty() || l.Play.empty() || l.Close.empty() || l.Bar.empty() {
			t.Errorf("%s: no chip (%+v)", sc.name, l)
			rl.UnloadRenderTexture(rt)
			continue
		}
		f, err := os.Create(filepath.Join(out, "chip-"+sc.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		_ = png.Encode(f, img)
		f.Close()

		if p := img.RGBAAt(l.Panel.X+l.Panel.W-2, l.Panel.Y+l.Panel.H/2); near(p, backdrop, 6) {
			t.Errorf("%s: the chip is not drawn (pixel %v)", sc.name, p)
		}
		ink := func(r deckRect, want color.RGBA, tol int) int {
			n := 0
			for y := r.Y; y < r.Y+r.H; y++ {
				for x := r.X; x < r.X+r.W; x++ {
					if near(img.RGBAAt(x, y), want, tol) {
						n++
					}
				}
			}
			return n
		}
		tc := theme.Text
		textCol := color.RGBA{R: tc.R, G: tc.G, B: tc.B, A: 255}
		if n := ink(l.Close, textCol, 60); n < l.Close.W/2 {
			t.Errorf("%s: %d ink pixels in the close button", sc.name, n)
		}
		if n := ink(l.Play, textCol, 60); n < 4 {
			t.Errorf("%s: %d ink pixels in the play button", sc.name, n)
		}
		if !l.Time.empty() {
			dc := theme.DimText
			if n := ink(l.Time, color.RGBA{R: dc.R, G: dc.G, B: dc.B, A: 255}, 40); n < 10 {
				t.Errorf("%s: %d text pixels in the counter", sc.name, n)
			}
		}
		// The hairline at half: filled at a quarter of its length, the dark track at three quarters.
		y := l.Bar.Y + l.Bar.H/2
		left, right := img.RGBAAt(l.Bar.X+l.Bar.W/4, y), img.RGBAAt(l.Bar.X+l.Bar.W*3/4, y)
		if left.R < 150 || right.R > 120 {
			t.Errorf("%s: the hairline at half reads %v | %v", sc.name, left, right)
		}
		// The play button is a pause button while the tape plays, and looks different.
		playing, _ := render(true, 0.5)
		diff := 0
		for y := l.Play.Y; y < l.Play.Y+l.Play.H; y++ {
			for x := l.Play.X; x < l.Play.X+l.Play.W; x++ {
				if playing.RGBAAt(x, y) != img.RGBAAt(x, y) {
					diff++
				}
			}
		}
		if diff == 0 {
			t.Errorf("%s: the chip's button looks the same playing and stopped", sc.name)
		}
		rl.UnloadRenderTexture(rt)
	}
}
