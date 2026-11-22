//go:build !headless

package zenzx

import (
	"image/color"
	"os"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Tests that need a live OpenGL context: they open a window. They are dormant guards G-03
// and G-05 (docs/KNOWN_ISSUES.md) and run only when ZENZX_GL_TESTS is set, on a host with
// a display. A virtual one with software rendering is enough:
//
//	Xvfb :99 -screen 0 1280x1024x24 &
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGL -v .
//
// Everything is in one test because raylib has one window and one context, tied to the
// thread that made them.

func TestGLNextPicture(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: these tests open a window (dormant guards G-03, G-05)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// The window as the GUI opens it: the picture with its border, at twice the size.
	const mult = 2
	const winW, winH = RefWidth * mult, RefHeight * mult
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(winW, winH, "ZenZX GL tests")
	defer rl.CloseWindow()

	mem, screen := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	screen.multiplier = mult
	dm := NewDisplayManager(screen)
	dm.SetVideoRenderer(standardVideoRenderer{})
	dm.SetSpectrumIO(io)
	dm.showFPS = false
	dm.currentWidth, dm.currentHeight = winW, winH // the 320x256 canvas of the standard renderer
	dm.InitializeAfterWindow()
	defer dm.CleanupTextures()

	// frame draws one frame and returns what is in the framebuffer at the end of it.
	frame := func() []color.RGBA {
		var pixels []color.RGBA
		dm.SetPreEndDrawHook(func() {
			rl.DrawRenderBatchActive()
			img := rl.LoadImageFromScreen()
			pixels = rl.LoadImageColors(img)
			rl.UnloadImage(img)
		})
		dm.Render(false, mem, screen, io)
		dm.SetPreEndDrawHook(nil)
		if len(pixels) != winW*winH {
			t.Fatalf("read back %d pixels from the window, want %d", len(pixels), winW*winH)
		}
		return pixels
	}
	// at reads a window pixel, counted from the top left of the 256x192 picture.
	ox, oy := dm.classicScreenOffset()
	at := func(pixels []color.RGBA, x, y int) color.RGBA { return pixels[(int(oy)+y)*winW+int(ox)+x] }
	sameRGB := func(a, b color.RGBA) bool { return a.R == b.R && a.G == b.G && a.B == b.B }
	// The sections run here and not as subtests: a subtest is another goroutine, on
	// another thread, where the context made above is not current.
	section := func(name string, fn func()) {
		t.Log(name)
		fn()
	}

	section("G-03 Layer 2 palette textures", func() {
		// A 9-bit colour in the first Layer 2 palette, written the way a program does.
		io.nextRegWriteDirect(0x43, 0x10)
		io.nextRegWriteDirect(0x40, 0x11)
		io.nextRegWriteDirect(0x44, 0b11100100)
		io.nextRegWriteDirect(0x44, 0x01)
		dm.bakeLayer2ColourGPU(io)
		for idx := 0; idx < 256; idx++ {
			img := rl.LoadImageFromTexture(dm.layer2ColourGPU[idx])
			got := rl.LoadImageColors(img)[0]
			rl.UnloadImage(img)
			want := io.nextColourFor(nextLayerLayer2, nextColourIndex(idx))
			if got.R != want.R || got.G != want.G || got.B != want.B || got.A != want.A {
				t.Fatalf("texture for Layer 2 colour 0x%02X holds %v, the palette says %v", idx, got, want)
			}
		}
		if dm.layer2ColourGPUVersion != io.nextPaletteVersion {
			t.Fatal("the textures are not marked as made from the current palette")
		}
	})

	section("G-05 Layer 2 from bank 8 reaches the window", func() {
		// T-42: with NextREG 0x12 at its reset value the picture is in banks 8-10, so its
		// first byte is the first byte of page 16.
		mem.nextPageWrite(16, 0, 0x11)
		mem.nextPageWrite(16, 1, 0x00)
		io.nextRegWriteDirect(0x69, 0x80) // Layer 2 on; NextREG 0x15 at its reset value puts it over the ULA
		pixels := frame()
		want := nrgbaToRGBA(io.nextColourFor(nextLayerLayer2, 0x11))
		if got := at(pixels, 0, 0); !sameRGB(got, want) {
			t.Errorf("Layer 2 pixel (0,0) in the window is %v, want palette entry 0x11: %v", got, want)
		}
		if sameRGB(at(pixels, mult, 0), want) {
			t.Error("the next Layer 2 pixel has the first one's colour")
		}
		// The frame's border is the machine's border colour, through the ULA palette.
		io.borderColor = 2
		red := nrgbaToRGBA(io.nextPalette.lookup(nextLayerULA, 16+2).Colour())
		if got := frame()[winW+1]; !sameRGB(got, red) {
			t.Errorf("the window's corner is %v with a red border, want %v", got, red)
		}
		io.borderColor = 0
		io.nextRegWriteDirect(0x69, 0x00)
		if sameRGB(at(frame(), 0, 0), want) {
			t.Error("Layer 2 is still drawn after NextREG 0x69 bit 7 was cleared")
		}
	})

	section("G-05 classic picture, in the frame", func() {
		screen.bitmap[0] = 0x80     // top left pixel
		screen.attributes[0] = 0x38 // black ink on white paper
		pixels := frame()
		ink, paper := at(pixels, 0, 0), at(pixels, mult, 0)
		if !sameRGB(ink, at(pixels, 1, 0)) || sameRGB(ink, paper) {
			t.Fatalf("standard screen: pixels %v %v then %v, want one doubled ink pixel then paper", ink, at(pixels, 1, 0), paper)
		}
	})

	section("G-05 Timex hi-res reaches the window", func() {
		mem.nextPageWrite(10, 0, 0x80) // screen 0, first byte: column 0
		mem.nextPageWrite(11, 0, 0x01) // screen 1, first byte: column 1
		io.WritePort(0x00FF, 0x06)     // hi-res, black on white
		pixels := frame()
		if !dm.screenTextureReady || dm.screenTexture.Width != 2*nextFrameWidth {
			t.Fatalf("hi-res: screen texture ready=%v width=%d, want the frame at double width, 640", dm.screenTextureReady, dm.screenTexture.Width)
		}
		// At twice the size a hi-res pixel is one window pixel.
		ink, paper := io.ulaColour(8), io.ulaColour(24+7)
		for x, want := range map[int]color.RGBA{0: ink, 1: paper, 14: paper, 15: ink, 16: paper} {
			if got := at(pixels, x, 0); !sameRGB(got, want) {
				t.Errorf("hi-res window pixel %d is %v, want %v", x, got, want)
			}
		}
		io.WritePort(0x00FF, 0x00)
	})

	section("G-05 tilemap reaches the window", func() {
		// One 4-bit tile, all of colour 5, at the first cell inside the 256 pixel area:
		// the tilemap is 320 wide, so that is column 4, row 4. Map at bank 5 offset
		// 0x2C00, definitions at 0x0C00 (the reset values of NextREG 0x6E and 0x6F).
		bank5 := func(addr int, v uint8) { mem.nextPageWrite(uint8(10+addr>>13), uint16(addr&0x1FFF), v) }
		for i := 0; i < 32; i++ {
			bank5(0x0C00+32+i, 0x55) // tile 1
		}
		bank5(0x2C00+(4*40+4)*2, 1)
		bank5(0x2C00+(4*40+4)*2+1, 0)
		bank5(0x2C00, 1) // and the tile at the frame's corner, in the border
		bank5(0x2C00+1, 0)
		io.nextRegWriteDirect(0x6B, 0x81) // on, 40 columns, over the ULA
		pixels := frame()
		if !dm.screenTextureReady || dm.screenTexture.Width != nextFrameWidth || dm.screenTexture.Height != nextFrameHeight {
			t.Fatalf("tilemap: screen texture ready=%v %dx%d, want the 320x256 frame", dm.screenTextureReady, dm.screenTexture.Width, dm.screenTexture.Height)
		}
		want := tilemapColour(5)
		if got := at(pixels, 3*mult, 3*mult); !sameRGB(got, want) {
			t.Fatalf("tilemap: window pixel is %v, the software picture has %v there", got, want)
		}
		if got := pixels[3*winW+3]; !sameRGB(got, want) {
			t.Fatalf("tilemap: the tile in the border's corner is %v in the window, want %v", got, want)
		}
		// With the border hidden the window shows the classic picture's part of the frame:
		// the tile at column 4, row 4 is then in the window's corner.
		dm.showBorder = false
		pixels = frame()
		dm.showBorder = true
		if got := pixels[3*winW+3]; !sameRGB(got, want) {
			t.Fatalf("tilemap, border hidden: the window's corner is %v, want the tile at the classic picture's corner %v", got, want)
		}
		if got := pixels[3*winW+winW/2]; sameRGB(got, want) {
			t.Fatal("tilemap, border hidden: the tile's colour is in the middle of the top line")
		}
		// With the tilemap off again the classic picture is back.
		io.nextRegWriteDirect(0x6B, 0x00)
		pixels = frame()
		if sameRGB(at(pixels, 3*mult, 3*mult), want) {
			t.Fatal("after the tilemap was turned off its tile is still in the window")
		}
	})
}
