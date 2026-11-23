//go:build !headless

package zenzx

import (
	"image"
	"image/color"
	"os"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
	"github.com/ha1tch/zenzx/pkg/fonts"
)

// TestGLAudioDebugOverlay draws the audio debug overlay in a window and checks that it is made of
// the interface's own font and zenui's rectangles: the level bar is the health's colour as far as
// the level reaches, the three lines of figures and the health word at the foot are drawn in
// glyphs of the BDF face (their ink is the exact colour asked for, a whole number of face pixels
// high), and a line too wide for the window is drawn one notch smaller. Part of dormant guard G-14
// (docs/KNOWN_ISSUES.md); it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLAudioDebugOverlay -v .
func TestGLAudioDebugOverlay(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(960, 768, "ZenZX audio overlay")
	defer rl.CloseWindow()
	face, err := fonts.Sinclair()
	if err != nil {
		t.Fatal(err)
	}
	txt := zenuiraylib.NewBDFText(face)
	defer txt.Unload()
	defer func(f func() *zenuiraylib.BDFText) { uiText = f }(uiText)
	uiText = func() *zenuiraylib.BDFText { return txt }

	px := func(c zenui.Colour) color.RGBA { return color.RGBA{R: c.R, G: c.G, B: c.B, A: 255} }
	draw := func(w, h int, level float32) *image.RGBA {
		rt := rl.LoadRenderTexture(int32(w), int32(h))
		defer rl.UnloadRenderTexture(rt)
		rl.BeginDrawing()
		rl.BeginTextureMode(rt)
		rl.ClearBackground(rl.NewColor(128, 128, 128, 255))
		drawAudioDebug(w, h, level, "Audio Buffer: 50.0% (2048/4096 samples)", "Speaker changes: 12", "CPU Cycle: 70000")
		rl.DrawRenderBatchActive()
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(rt.Texture)
		rl.ImageFlipVertical(img)
		colors := rl.LoadImageColors(img)
		rl.UnloadImage(img)
		rl.EndDrawing()
		if len(colors) != w*h {
			t.Fatalf("read back %d pixels, want %d", len(colors), w*h)
		}
		out := image.NewRGBA(image.Rect(0, 0, w, h))
		for i, p := range colors {
			out.Pix[i*4], out.Pix[i*4+1], out.Pix[i*4+2], out.Pix[i*4+3] = p.R, p.G, p.B, 255
		}
		return out
	}
	// inkRows is the first and last row, and the rightmost column, in the given rectangle that
	// hold a pixel of exactly colour c.
	inkBox := func(img *image.RGBA, r image.Rectangle, c color.RGBA) (n, minY, maxY, maxX int) {
		minY, maxY, maxX = 1<<30, -1, -1
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if img.RGBAAt(x, y) == c {
					n++
					minY, maxY, maxX = min(minY, y), max(maxY, y), max(maxX, x)
				}
			}
		}
		return
	}

	for _, c := range []struct {
		level  float32
		colour zenui.Colour
	}{{10, overlayRed}, {50, overlayYellow}, {90, overlayGreen}} {
		const w, h = 960, 768
		img := draw(w, h, c.level)
		fill := int(float32(w) * c.level / 100)
		// The bar: the health's colour under the line of text (row barY+19 is below its glyphs), out
		// to the level, and the translucent black beyond it.
		if got := img.RGBAAt(5, 40+19); got != px(c.colour) {
			t.Errorf("level %v: the bar is %v, want %v", c.level, got, px(c.colour))
		}
		if got := img.RGBAAt(fill-2, 40+19); got != px(c.colour) {
			t.Errorf("level %v: the bar does not reach its level (%v at %d)", c.level, got, fill-2)
		}
		if got := img.RGBAAt(fill+3, 40+19); got == px(c.colour) || got.R >= 128 {
			t.Errorf("level %v: beyond the level the bar is %v, want it dark", c.level, got)
		}
		// The figures, in the face's glyphs: white on the bar's line, sky blue below.
		n, y0, y1, x1 := inkBox(img, image.Rect(0, 40, w, 60), px(overlayWhite))
		if n == 0 || y0 < 42 || y1 > 42+15 {
			t.Errorf("level %v: the buffer line has %d white pixels in rows %d..%d; want its glyphs in the 16 rows from 42", c.level, n, y0, y1)
		}
		if x1 > 5+len("Audio Buffer: 50.0% (2048/4096 samples)")*16 {
			t.Errorf("level %v: the buffer line reaches column %d: not drawn at the interface's size", c.level, x1)
		}
		if n, _, _, _ := inkBox(img, image.Rect(0, 65, w, 115), px(overlaySky)); n == 0 {
			t.Errorf("level %v: no speaker or cycle line", c.level)
		}
		// The health word at the foot is in the bar's colour.
		if n, _, _, _ := inkBox(img, image.Rect(0, h-45, w, h-20), px(c.colour)); n == 0 {
			t.Errorf("level %v: no health line at the foot", c.level)
		}
	}

	// A window too narrow for the line at the interface's size draws it one notch smaller.
	img := draw(480, 384, 50)
	if _, _, _, x1 := inkBox(img, image.Rect(0, 40, 480, 60), px(overlayWhite)); x1 < 0 || x1 > 480-5 {
		t.Errorf("in a 480 pixel window the buffer line ends at column %d; want it inside", x1)
	}
}
