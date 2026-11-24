//go:build !headless

package zenzx

import (
	"fmt"
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

// TestGLPlayButton draws the Play button over a plain backdrop and over pictures of the Next's
// demos (docs/img), in three window sizes and its three looks (at rest, under the pointer,
// pressed), and checks what was asked of it: the sprite is the debugger's Play, enlarged by
// whole pixels (every sprite pixel a solid square of the theme's text colour), it is centred
// in the picture, the face has round corners, hovering brightens the face, and pressing moves
// the sprite one pixel down and right. It writes the pictures to the folder named by
// ZENZX_PLAY_SHOTS (or a temporary one).
//
// A dormant guard, G-14 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	Xvfb :99 -screen 0 1280x1024x24 &
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_PLAY_SHOTS=shots go test -run TestGLPlayButton -v .
func TestGLPlayButton(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	out := os.Getenv("ZENZX_PLAY_SHOTS")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(960, 768, "ZenZX play button")
	defer rl.CloseWindow()

	theme := zenui.DarkTheme()
	ink := color.RGBA{R: theme.Text.R, G: theme.Text.G, B: theme.Text.B, A: 255}
	btn := newPlayButton()
	if !btn.iconOK {
		t.Fatal("the debugger's Play sprite was not found")
	}
	spriteInk := 0
	for _, run := range iconRuns(btn.icon) {
		if run.ink {
			spriteInk += run.n
		}
	}
	x0, y0, x1, y1 := iconBounds(btn.icon)
	grey := rl.NewColor(128, 128, 128, 255)

	type look struct {
		name        string
		hover, down bool
	}
	looks := []look{{"rest", false, false}, {"hover", true, false}, {"pressed", true, true}}
	sizes := [][2]int{{960, 768}, {480, 384}, {320, 256}}
	demos := []string{"", "next-example-fuji", "next-example-gallop", "next-spectron-2084"} // "" is the grey backdrop, for measuring

	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		rt := rl.LoadRenderTexture(int32(w), int32(h))
		area := rl.Rectangle{X: 0, Y: 0, Width: float32(w), Height: float32(h)}
		at := playButtonRect(area)
		s := playScale(area.Width, area.Height)
		btn.round = nil // made again for this size
		frame := func(bg rl.Texture2D, l look) *image.RGBA {
			btn.hover, btn.down = l.hover, l.down
			rl.BeginDrawing()
			rl.BeginTextureMode(rt)
			if bg.ID == 0 {
				rl.ClearBackground(grey)
			} else {
				rl.ClearBackground(rl.Black)
				rl.DrawTexturePro(bg, rl.NewRectangle(0, 0, float32(bg.Width), float32(bg.Height)),
					rl.NewRectangle(0, 0, float32(w), float32(h)), rl.NewVector2(0, 0), 0, rl.White)
			}
			btn.Draw(area, theme)
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
			rgba := image.NewRGBA(image.Rect(0, 0, w, h))
			for i, p := range colors {
				rgba.Pix[i*4], rgba.Pix[i*4+1], rgba.Pix[i*4+2], rgba.Pix[i*4+3] = p.R, p.G, p.B, 255
			}
			return rgba
		}

		// Where the sprite's ink is, in each look, on the grey backdrop.
		var inkBox [3]image.Rectangle
		var face [3]color.RGBA
		for i, l := range looks {
			img := frame(rl.Texture2D{}, l)
			box := image.Rectangle{Min: image.Pt(w, h), Max: image.Pt(-1, -1)}
			n := 0
			// The sprite's own ink: inside the face, clear of the edge, which under the pointer is
			// the same colour.
			in := image.Rect(int(at.X)+s, int(at.Y)+s, int(at.X+at.Width)-s, int(at.Y+at.Height)-s)
			for y := in.Min.Y; y < in.Max.Y; y++ {
				for x := in.Min.X; x < in.Max.X; x++ {
					if img.RGBAAt(x, y) == ink {
						n++
						box.Min.X, box.Min.Y = min(box.Min.X, x), min(box.Min.Y, y)
						box.Max.X, box.Max.Y = max(box.Max.X, x+1), max(box.Max.Y, y+1)
					}
				}
			}
			inkBox[i] = box
			// A pixel of the face high up, clear of the sprite and the corners, where its tint is
			// white at rest, whiter hovered and black pressed.
			face[i] = img.RGBAAt(int(at.X)+2*s, int(at.Y)+int(at.Height)/5)
			if n != spriteInk*s*s {
				t.Errorf("%dx%d %s: %d ink pixels, want %d (%d sprite pixels, each %dx%d)", w, h, l.name, n, spriteInk, spriteInk, s, s)
			}
			if box.Dx() != (x1-x0)*s || box.Dy() != (y1-y0)*s {
				t.Errorf("%dx%d %s: the sprite measures %dx%d, want %dx%d", w, h, l.name, box.Dx(), box.Dy(), (x1-x0)*s, (y1-y0)*s)
			}
			// Round corners: the face's own corner pixel is still the backdrop.
			if img.RGBAAt(int(at.X), int(at.Y)) != (color.RGBA{128, 128, 128, 255}) {
				t.Errorf("%dx%d %s: the face has a square corner", w, h, l.name)
			}
			// Centred, give or take the rounding of an odd-sized sprite and the pressed pixel.
			cx, cy := (box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2
			if d := cx - w/2; d < -s || d > s+1 {
				t.Errorf("%dx%d %s: the sprite is centred %d px off across", w, h, l.name, d)
			}
			if d := cy - h/2; d < -s || d > s+1 {
				t.Errorf("%dx%d %s: the sprite is centred %d px off down", w, h, l.name, d)
			}
		}
		if got := inkBox[2].Min.Sub(inkBox[0].Min); got != image.Pt(1, 1) {
			t.Errorf("%dx%d: pressing moved the sprite by %v, want (1,1)", w, h, got)
		}
		if face[1].R <= face[0].R || face[2].R >= face[0].R {
			t.Errorf("%dx%d: the face is %d at rest, %d hovered, %d pressed; want brighter hovered and darker pressed", w, h, face[0].R, face[1].R, face[2].R)
		}

		for _, demo := range demos {
			if demo == "" {
				continue
			}
			bg := rl.LoadTexture(filepath.Join("docs", "img", demo+".png"))
			if bg.ID == 0 {
				t.Fatalf("no picture %s", demo)
			}
			for _, l := range looks {
				path := filepath.Join(out, fmt.Sprintf("play-%s-%dx%d-%s.png", demo, w, h, l.name))
				f, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, frame(bg, l)); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
			rl.UnloadTexture(bg)
		}
		rl.UnloadRenderTexture(rt)
		t.Logf("%dx%d: scale %d, face %v, sprite %d ink pixels", w, h, s, at, spriteInk)
	}
	t.Logf("pictures in %s", out)
}
