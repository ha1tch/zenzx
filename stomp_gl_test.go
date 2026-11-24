//go:build !headless

package zenzx

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestGLStomp draws the stop animation over a stand-in for a machine's picture at several moments
// and checks what each should show: the ground coming up from below, the leg coming down from the
// top edge, the icon squashed under the boot, the boot gone, the ground going back down with the
// remains, and nothing at the end. It writes a contact sheet
// to the folder named by ZENZX_STOMP_SHOTS (or a temporary one).
//
// A dormant guard, G-14 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	Xvfb :99 -screen 0 1280x1024x24 &
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 ZENZX_STOMP_SHOTS=shots go test -run TestGLStomp -v .
func TestGLStomp(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-14)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	out := os.Getenv("ZENZX_STOMP_SHOTS")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	rl.SetTraceLogLevel(rl.LogNone)
	const w, h = 768, 576
	rl.InitWindow(w, h, "ZenZX stomp")
	defer rl.CloseWindow()

	imgs := embeddedStompImages()
	tex := newStompTextures(imgs)
	defer tex.unload()
	rt := rl.LoadRenderTexture(w, h)
	backdrop := rl.NewColor(0, 0, 160, 255)
	target := rl.Rectangle{Width: w, Height: h}

	frame := func(at float64) *image.RGBA {
		now := time.Now()
		a := &stompAnim{start: now.Add(-time.Duration(at * float64(time.Second))), target: target}
		rl.BeginDrawing()
		rl.BeginTextureMode(rt)
		rl.ClearBackground(backdrop)
		a.draw(tex, now)
		rl.DrawRenderBatchActive()
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(rt.Texture)
		rl.ImageFlipVertical(img)
		colors := rl.LoadImageColors(img)
		rl.UnloadImage(img)
		rl.EndDrawing()
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		for i, p := range colors {
			rgba.Pix[i*4], rgba.Pix[i*4+1], rgba.Pix[i*4+2], rgba.Pix[i*4+3] = p.R, p.G, p.B, 255
		}
		return rgba
	}
	// rows of the picture where something other than the backdrop (dimmed or not) is drawn, in
	// the column of the leg, and the icon's own rows (its body and keys).
	// The leg is drawn with its middle where the boot's top is (stomp_gui.go), at the same whole-number
	// scale as the rest.
	sc := max(2, min(8, min(w, h)/4/int(tex.icon.Width)))
	legCol := w/2 - int(tex.boot.Width)*sc/2 + int(tex.legMid2)*sc/2
	// What is the icon's: its badge and its green stripe (its keys are the bin's grey, and its
	// black body is the same black as the boot's fill and the ground's).
	isIcon := func(c color.RGBA) bool { return c == stompBadge || c == stompGreen }
	// Something other than the backdrop, which is blue, dimmed or not.
	notBackdrop := func(c color.RGBA) bool { return c.R > 0 || c.B == 0 }
	iconRows := func(img *image.RGBA) (n int) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if isIcon(img.RGBAAt(x, y)) {
					n++
					break
				}
			}
		}
		return n
	}
	legAt := func(img *image.RGBA, y int) bool {
		return notBackdrop(img.RGBAAt(legCol, y))
	}

	// fromTop: the leg, or the boot it is joined to, is at the top edge in the middle column (early
	// in the drop the boot is itself still entering, with only its shaft in view).
	fromTop := func(img *image.RGBA) bool {
		return notBackdrop(img.RGBAAt(legCol, 0))
	}

	// The moments looked at, in seconds: nothing yet, the ground coming up, up, the boot falling,
	// landed, lifting, the remains at rest, the ground going, and the end.
	times := []float64{0.01, 0.25, 0.50, 1.00, 1.15, 1.35, 1.55, 1.85, 2.15, 2.40, 2.65, 2.85}
	shots := map[float64]*image.RGBA{}
	for _, b := range times {
		shots[b] = frame(b)
	}
	shot := func(at float64) *image.RGBA { return shots[at] }

	// The top of the ground, in a column clear of the bin: the first row that is not the backdrop,
	// or h when there is none.
	groundTop := func(img *image.RGBA) int {
		for y := 0; y < h; y++ {
			if notBackdrop(img.RGBAAt(w/8, y)) {
				return y
			}
		}
		return h
	}
	reach := func(img *image.RGBA) int {
		y := 0
		for y < h && legAt(img, y) {
			y++
		}
		return y
	}

	// Nothing is there at the start: the ground is below the window, and so are the bin and the machine.
	if g := groundTop(shot(0.01)); g < h-sc*4 {
		t.Errorf("at the start the ground is already up to row %d", g)
	}
	if n := iconRows(shot(0.01)); n != 0 {
		t.Errorf("at the start %d rows of the icon are in view: it should be below the window", n)
	}
	// It comes up: higher at each of these moments than at the one before, and then in its place.
	// (Past its place a little at 0.50, where the ground overshoots.)
	rest := groundTop(shot(1.00))
	if g := groundTop(shot(0.25)); g <= rest+20 {
		t.Errorf("at 0.25s the ground is at row %d, and at rest at %d: it should be well below its place", g, rest)
	}
	if g := groundTop(shot(0.50)); g >= groundTop(shot(0.25)) || g > rest+sc {
		t.Errorf("at 0.45s the ground is at row %d (at 0.25s %d, at rest %d): it should be nearly up", g, groundTop(shot(0.25)), rest)
	}
	// The ground runs across the bottom of the window once it is up, with the bin on it.
	if !notBackdrop(shot(1.00).RGBAAt(w/8, h-4)) || !notBackdrop(shot(1.00).RGBAAt(w*7/8, h-4)) {
		t.Error("there is no ground at the bottom of the window when it is up")
	}
	full := iconRows(shot(1.00))
	if full < 14 {
		t.Fatalf("the icon is %d rows tall when the ground is up: it is not drawn", full)
	}
	// While it is whole it has a thin grey line round it, which is gone once it is flattened.
	outlined := func(img *image.RGBA) (n int) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if c := img.RGBAAt(x, y); c.R == stompOutline.R && c.G == stompOutline.G && c.B == stompOutline.B {
					n++
				}
			}
		}
		return n
	}
	// (The bin and the ground have greys of their own; what the line adds is what differs.)
	bare := outlined(shot(2.15))
	if n := outlined(shot(1.00)) - bare; n < 2*(int(tex.icon.Width)+int(tex.icon.Height))*sc/3 {
		t.Errorf("the whole machine has %d pixels of outline more than the flattened one: it should be outlined", n)
	}
	// The drop: the leg is there from the top edge, and reaches further down as time goes on.
	if !fromTop(shot(1.15)) || !fromTop(shot(1.35)) {
		t.Error("the leg does not come from the top edge of the window")
	}
	if a, b := reach(shot(1.15)), reach(shot(1.35)); a >= b {
		t.Errorf("the leg reaches to %d at 0.95s and %d at 1.15s: it is not coming down", a, b)
	}
	// Landed: the icon is squashed to a fraction of its height, with the boot on it.
	if landed := iconRows(shot(1.55)); landed*3 > full || landed == 0 {
		t.Errorf("landed, the icon is %d rows tall (it was %d): it should be a fraction", landed, full)
	}
	// Lifted: the boot is off, the remains stay in their place.
	if legAt(shot(2.15), 40) {
		t.Error("the leg is still down when the boot has gone back up")
	}
	if iconRows(shot(2.15)) == 0 {
		t.Error("the remains are gone before the ground goes")
	}
	if g := groundTop(shot(2.15)); g != rest {
		t.Errorf("the ground is at row %d after the stomp, and was at %d before it", g, rest)
	}
	// The retiring: the ground, the bin and the remains go down together, and are gone.
	if g := groundTop(shot(2.65)); g <= rest+sc*4 {
		t.Errorf("at 2.45s the ground is at row %d (at rest %d): it should be going down", g, rest)
	}
	if got := iconRows(shot(2.85)); got != 0 {
		t.Errorf("%d rows of the icon are left after the animation is over", got)
	}
	if notBackdrop(shot(2.85).RGBAAt(w/8, h-4)) {
		t.Error("the ground is still there when the animation is over")
	}
	over := shot(2.85)
	for _, p := range [][2]int{{10, 10}, {w / 2, h / 2}, {w - 10, h - 10}} {
		if c := over.RGBAAt(p[0], p[1]); c != (color.RGBA{0, 0, 160, 255}) {
			t.Errorf("after the end (%d,%d) is %v: the picture was left dimmed", p[0], p[1], c)
		}
	}

	cw, ch := w/4, h/4
	sheet := image.NewRGBA(image.Rect(0, 0, 4*cw, 3*ch))
	for i, b := range times {
		small := image.NewRGBA(image.Rect(0, 0, cw, ch))
		for y := 0; y < ch; y++ {
			for x := 0; x < cw; x++ {
				small.Set(x, y, shots[b].At(x*4, y*4))
			}
		}
		draw.Draw(sheet, image.Rect((i%4)*cw, (i/4)*ch, (i%4+1)*cw, (i/4+1)*ch), small, image.Point{}, draw.Src)
	}
	f, err := os.Create(filepath.Join(out, "stomp.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, sheet); err != nil {
		t.Fatal(err)
	}
	fmt.Println("stomp contact sheet:", f.Name())
}
