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

	"github.com/ha1tch/zenui/zenuiraylib"
	"github.com/ha1tch/zenzx/pkg/fonts"
)

// TestGLToastOverTheDemos draws the toast over pictures of the Next's demos running in ZenZX
// (docs/img) and checks what was asked of it: the face of the text is white inside a black
// stroke, and the toast, text and stroke, is never wider or taller than half the window: a long
// message is wrapped in lines at the usual size, and drawn one notch smaller only when a word is too wide for a line. It writes the pictures it
// draws, with the sparks, to the folder named by ZENZX_TOAST_SHOTS (or a temporary one).
//
// A dormant guard, G-14 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	Xvfb :99 -screen 0 1280x1024x24 &
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_TOAST_SHOTS=shots go test -run TestGLToast -v .
func TestGLToastOverTheDemos(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	out := os.Getenv("ZENZX_TOAST_SHOTS")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(960, 768, "ZenZX toast")
	defer rl.CloseWindow()
	face, err := fonts.Sinclair()
	if err != nil {
		t.Fatal(err)
	}
	txt := zenuiraylib.NewBDFText(face)
	defer txt.Unload()

	cases := []struct {
		name         string
		w, h         int
		message      string
		scale, lines int
	}{
		{"short", 960, 768, "Loaded fuji.nex", 2, 1},
		{"wrapped-usual", 960, 768, "The snapshot manic-miner-v2.z80 has been restored", 2, 3},
		{"wrapped-usual-narrow", 480, 384, "The snapshot has been restored", 2, 3},
		{"wrapped", 480, 384, "The tape a-rather-long-tape-name.tap is loaded and rewound (3 blocks)", 1, 0},
		{"wrapped-narrow", 320, 256, "The tape a-rather-long-tape-name.tap is loaded and rewound (3 blocks)", 1, 0},
	}
	demos := []string{"next-example-fuji", "next-example-gallop", "next-example-orbit", "next-scrollnutter", "next-spectron-2084", "next-night-knight"}
	for _, demo := range demos {
		bg := rl.LoadTexture(filepath.Join("docs", "img", demo+".png"))
		if bg.ID == 0 {
			t.Fatalf("no picture %s", demo)
		}
		for _, c := range cases {
			tt := newToast(txt)
			tt.Start(c.message)
			tt.fit(c.w, c.h)
			tt.travel = 60 // risen far enough to be wholly in the window, and not yet fading
			if tt.scale != c.scale || (c.lines > 0 && len(tt.lines) != c.lines) || len(tt.lines) == 0 {
				t.Fatalf("%s: %q laid out at scale %d in %d lines", c.name, c.message, tt.scale, len(tt.lines))
			}
			// Drawn into a texture of the window's size, so that a window of any size can be had
			// from the one that is open.
			rt := rl.LoadRenderTexture(int32(c.w), int32(c.h))
			defer rl.UnloadRenderTexture(rt)
			frame := func(withToast, sparks, onGrey bool) *image.RGBA {
				rl.BeginDrawing()
				rl.BeginTextureMode(rt)
				if onGrey {
					// For what is measured: a stroke on black would change no pixel.
					rl.ClearBackground(rl.NewColor(128, 128, 128, 255))
				} else {
					rl.ClearBackground(rl.Black)
					rl.DrawTexturePro(bg, rl.NewRectangle(0, 0, float32(bg.Width), float32(bg.Height)),
						rl.NewRectangle(0, 0, float32(c.w), float32(c.h)), rl.NewVector2(0, 0), 0, rl.White)
				}
				if withToast {
					if sparks {
						tt.scatter()
					} else {
						tt.sparks = nil
					}
					tt.Draw(c.w, c.h)
				}
				rl.DrawRenderBatchActive()
				rl.EndTextureMode()
				img := rl.LoadImageFromTexture(rt.Texture)
				rl.ImageFlipVertical(img)
				colors := rl.LoadImageColors(img)
				rl.UnloadImage(img)
				rl.EndDrawing()
				if len(colors) != c.w*c.h {
					t.Fatalf("read back %d pixels, want %d", len(colors), c.w*c.h)
				}
				rgba := image.NewRGBA(image.Rect(0, 0, c.w, c.h))
				for i, p := range colors {
					rgba.Pix[i*4], rgba.Pix[i*4+1], rgba.Pix[i*4+2], rgba.Pix[i*4+3] = p.R, p.G, p.B, 255
				}
				return rgba
			}
			plain := frame(false, false, true)
			marked := frame(true, false, true)

			// What the toast changed: its text and stroke.
			minX, maxX, minY, maxY := c.w, -1, c.h, -1
			white, black, orange := 0, 0, 0
			for y := 0; y < c.h; y++ {
				for x := 0; x < c.w; x++ {
					a, b := plain.RGBAAt(x, y), marked.RGBAAt(x, y)
					if a == b {
						continue
					}
					minX, maxX = min(minX, x), max(maxX, x)
					minY, maxY = min(minY, y), max(maxY, y)
					switch b {
					case color.RGBA{255, 255, 255, 255}:
						white++
					case color.RGBA{0, 0, 0, 255}:
						black++
					case color.RGBA{0xff, 0x9a, 0x10, 255}:
						orange++
					}
				}
			}
			if maxX < 0 {
				t.Fatalf("%s/%s: the toast drew nothing", demo, c.name)
			}
			if width := maxX - minX + 1; width > c.w/2 {
				t.Errorf("%s/%s: the toast is %d wide in a window %d wide", demo, c.name, width, c.w)
			}
			if white == 0 || black == 0 || orange != 0 {
				t.Errorf("%s/%s: %d white, %d black and %d orange pixels in the toast; it is white inside black", demo, c.name, white, black, orange)
			}
			// The box behind the text: darkened, its corners round, and more see-through at its
			// top edge than at its bottom one. Sampled in the padding, clear of the text.
			midX, midY := (minX+maxX)/2, (minY+maxY)/2
			top, bottom, side := marked.RGBAAt(midX, minY+2), marked.RGBAAt(midX, maxY-2), marked.RGBAAt(minX+toastPadX/2, midY)
			if !(top.R < 128 && bottom.R < top.R && side.R < 128) || bottom.R > 60 {
				t.Errorf("%s/%s: the box over grey is %d at its top, %d at its bottom, %d at its side; want darker than the grey and darker towards the bottom", demo, c.name, top.R, bottom.R, side.R)
			}
			if plain.RGBAAt(minX, minY) != marked.RGBAAt(minX, minY) || plain.RGBAAt(maxX, maxY) != marked.RGBAAt(maxX, maxY) {
				t.Errorf("%s/%s: the box has square corners", demo, c.name)
			}
			if maxX > c.w-toastMargin+1 || maxY >= c.h {
				t.Errorf("%s/%s: the toast reaches (%d,%d) of a %dx%d window", demo, c.name, maxX, maxY, c.w, c.h)
			}

			shot := frame(true, true, false)
			path := filepath.Join(out, fmt.Sprintf("toast-%s-%s.png", demo, c.name))
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, shot); err != nil {
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s: %s", path, fmt.Sprintf("scale %d, %d line(s), %d wide of %d", tt.scale, len(tt.lines), maxX-minX+1, c.w))
		}
		rl.UnloadTexture(bg)
	}
}
