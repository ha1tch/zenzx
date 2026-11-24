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

// TestGLTapeDeckMorph draws the deck at five moments of its move from a full deck to the chip, and
// of one from a small cassette to a big one, in the way the window draws a deck that is moving
// (the parts of a full deck, at the places between), and checks what is on the screen: the
// deck's panel is there at every moment, and nothing of the deck is drawn outside it in the band
// it is in. It writes the pictures to the folder named by ZENZX_DECK_SHOTS (or a temporary one).
//
// Part of dormant guard G-17 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_DECK_SHOTS=shots go test -run TestGLTapeDeckMorph -v .
func TestGLTapeDeckMorph(t *testing.T) {
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
	rl.InitWindow(960, 768, "ZenZX tape deck morph")
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
	const w, h, band = 960, 768, 72
	backdrop := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	area := deckRect{0, 2, w, band - 4}
	scale := 2
	lineH := txt.CellH() * scale
	full := layoutTapeDeckSized(area, lineH, txt.CellH(), cassetteSmall, false)
	big := layoutTapeDeckSized(area, lineH, txt.CellH(), cassetteBig, false)
	chip := chipAsLayout(layoutDeckChip(area, lineH, txt.Measure("00:00 / 00:00", scale), false))
	if full.Panel.empty() || chip.Panel.empty() || big.Panel.empty() {
		t.Fatalf("a layout is empty: %+v %+v %+v", full.Panel, chip.Panel, big.Panel)
	}
	rt := rl.LoadRenderTexture(w, h)
	defer rl.UnloadRenderTexture(rt)

	render := func(geo tapeDeckLayout) *image.RGBA {
		u := newTapeDeckUI()
		u.scale, u.size = scale, cassetteSmall
		u.layout, u.shown = geo, geo.Panel
		u.view = tapeDeckView{Visible: true, Mode: deckFull, Form: deckFull, Playing: true, Fraction: 0.4,
			Elapsed: 38, Length: 95, Label: "Manic Miner", Spin: 0.4}
		rl.BeginDrawing()
		rl.BeginTextureMode(rt)
		rl.ClearBackground(rl.NewColor(backdrop.R, backdrop.G, backdrop.B, 255))
		u.Draw(theme)
		rl.DrawRenderBatchActive()
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(rt.Texture)
		rl.ImageFlipVertical(img)
		colors := rl.LoadImageColors(img)
		rl.UnloadImage(img)
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		for i, p := range colors {
			rgba.Pix[i*4], rgba.Pix[i*4+1], rgba.Pix[i*4+2], rgba.Pix[i*4+3] = p.R, p.G, p.B, 255
		}
		rl.EndDrawing()
		return rgba
	}
	for name, pair := range map[string][2]tapeDeckLayout{"full-to-chip": {full, chip}, "small-to-big": {full, big}} {
		for _, step := range []float64{0, 0.25, 0.5, 0.75, 1} {
			geo := lerpLayout(pair[0], pair[1], deckEase(step))
			img := render(geo)
			f, err := os.Create(filepath.Join(out, "morph-"+name+"-"+itoa(int(step*100))+".png"))
			if err != nil {
				t.Fatal(err)
			}
			_ = png.Encode(f, img)
			f.Close()
			p := geo.Panel
			if c := img.RGBAAt(p.X+p.W/2, p.Y+3); c == backdrop {
				t.Errorf("%s at %.2f: the panel is not drawn (pixel %v at its top)", name, step, c)
			}
			// Nothing but the panel is drawn in the band, but for the stroke's edge.
			stray := 0
			for y := 0; y < band; y++ {
				for x := 0; x < w; x++ {
					if x >= p.X-2 && x < p.X+p.W+2 && y >= p.Y-2 && y < p.Y+p.H+2 {
						continue
					}
					if img.RGBAAt(x, y) != backdrop {
						stray++
					}
				}
			}
			if stray > 0 {
				t.Errorf("%s at %.2f: %d pixels are drawn outside the panel %+v", name, step, stray, p)
			}
		}
	}
}
