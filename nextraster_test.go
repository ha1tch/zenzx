package zenzx

import (
	"image"
	"image/color"
	"testing"
)

// rasterTest is a Next whose place in the frame the test sets by hand.
type rasterTest struct {
	t      *testing.T
	io     *SpectrumIO
	mem    *SpectrumMemory
	screen *SpectrumScreen
	tm     nextRasterTiming
}

func newRasterTest(t *testing.T) *rasterTest {
	t.Helper()
	mem, screen := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	r := &rasterTest{t: t, io: NewSpectrumIO(mem, nil), mem: mem, screen: screen}
	r.tm = nextRasterTiming{tPerLine: 228, firstPixelLine: 63, lines: 311}
	r.io.rasterEnable(func() nextRasterTiming { return r.tm }, screen)
	r.io.nextRegWriteDirect(0x68, 0x80) // ULA off: the frame is the fallback colour
	r.io.StartFrame()
	return r
}

// at puts the CPU where the beam is at a pixel of the frame: line y, and x pixels from
// the left edge of the pixel area (so -32 is the frame's left edge).
func (r *rasterTest) at(y, x int) {
	r.tm.t = (r.tm.firstPixelLine-nextFrameInset+y)*r.tm.tPerLine + x/2
	if x < 0 {
		r.tm.t += x % 2 // keep the division rounding down
	}
}

// newFrame ends the frame and starts another.
func (r *rasterTest) newFrame() {
	r.io.EndFrame()
	r.tm.t = 0
	r.io.StartFrame()
}

func (r *rasterTest) frame() *image.RGBA {
	return r.io.NextFrame(nil, r.screen, r.io.borderColor)
}

// copper loads a program at instruction 0 and leaves the copper stopped.
func (r *rasterTest) copper(program ...uint16) {
	r.io.nextRegWriteDirect(0x62, 0x00)
	r.io.nextRegWriteDirect(0x61, 0x00)
	for _, ins := range program {
		r.io.nextRegWriteDirect(0x60, uint8(ins>>8))
		r.io.nextRegWriteDirect(0x60, uint8(ins))
	}
}

func copperWait(line, h int) uint16      { return 0x8000 | uint16(h)<<9 | uint16(line) }
func copperMove(reg, value uint8) uint16 { return uint16(reg)<<8 | uint16(value) }

const copperHalt = 0xFFFF

var (
	rasterRed   = color.RGBA{R: 0xFF, A: 0xFF} // fallback 0xE0
	rasterGreen = color.RGBA{G: 0xFF, A: 0xFF} // 0x1C
	rasterBlue  = color.RGBA{B: 0xFF, A: 0xFF} // 0x03
)

func (r *rasterTest) want(img *image.RGBA, x, y int, c color.RGBA, what string) {
	r.t.Helper()
	if got := img.RGBAAt(x, y); got != c {
		r.t.Errorf("%s: frame pixel (%d,%d) is %v, want %v", what, x, y, got, c)
	}
}

func TestRasterIndex(t *testing.T) {
	tm := nextRasterTiming{tPerLine: 228, firstPixelLine: 63, lines: 311}
	line0 := (63 - 32) * 228 // the raster line of frame line 0's pixel area
	for _, c := range []struct {
		t    int
		want int
		what string
	}{
		{0, 0, "the interrupt, above the frame"},
		{line0 - 16, 0, "the first pixel of the frame: line 0's left border"},
		{line0, 0, "the pixel area's left edge: line 0 is not settled yet"},
		{line0 + 143, 0, "the last T-state of line 0's right border"},
		{line0 + 144, 320, "the end of line 0: it is settled"},
		{line0 + 200, 320, "between the lines"},
		{line0 + 213, 320, "in line 1's left border"},
		{line0 + 228 + 143, 320, "the last T-state of line 1's right border"},
		{line0 + 228 + 144, 640, "the end of line 1"},
		{line0 + 255*228 + 143, 255 * 320, "the last T-state of the frame's last line"},
		{line0 + 255*228 + 144, 256 * 320, "past the frame"},
		{310 * 228, 256 * 320, "the last raster line"},
	} {
		if got := rasterIndex(c.t*8, tm); got != c.want {
			t.Errorf("%s (T-state %d): %d pixels passed, want %d", c.what, c.t, got, c.want)
		}
	}
}

func TestRasterRegisterChangedDownTheFrame(t *testing.T) {
	r := newRasterTest(t)
	r.io.nextRegWriteDirect(0x4A, 0xE0) // above the frame: red everywhere
	r.at(100, 0)
	r.io.nextRegWriteDirect(0x4A, 0x1C) // on line 100, at the pixel area's left edge: green from line 100
	r.at(150, 64)
	r.io.nextRegWriteDirect(0x4A, 0x03) // 64 pixels into line 150: blue from line 150
	r.at(200, 290)
	r.io.nextRegWriteDirect(0x4A, 0xE0) // past line 200's right border: red from line 201
	img := r.frame()
	r.want(img, 0, 0, rasterRed, "top")
	r.want(img, 319, 99, rasterRed, "the line before the first change")
	r.want(img, 0, 100, rasterGreen, "the left border of the line it changed on: lines change whole")
	r.want(img, 319, 100, rasterGreen, "the right border of that line")
	r.want(img, 0, 149, rasterGreen, "the line before the second change")
	r.want(img, 0, 150, rasterBlue, "the line it changed on, left of where the beam was")
	r.want(img, 319, 200, rasterBlue, "a line whose right border the beam had left")
	r.want(img, 0, 201, rasterRed, "the line after it")
	r.want(img, 319, 255, rasterRed, "bottom")

	// The same frame asked for twice is the same picture.
	if again := r.frame(); string(again.Pix) != string(img.Pix) {
		t.Error("asking for the frame a second time gave a different picture")
	}
	// The next frame starts from the state as it is: blue all the way down.
	r.newFrame()
	img = r.frame()
	r.want(img, 0, 0, rasterRed, "next frame, top")
	r.want(img, 32, 100, rasterRed, "next frame, middle")
}

func TestRasterWriteThatIsNotToThePictureComposesNothing(t *testing.T) {
	r := newRasterTest(t)
	r.at(100, 0)
	r.io.nextRegWriteDirect(0x40, 0x10) // palette index
	r.io.nextRegWriteDirect(0x56, 0x20) // MMU
	r.io.nextRegWriteDirect(0x34, 0x05) // sprite number
	r.io.WritePort(0xFFFD, 0x07)        // AY
	if r.io.raster.done != 0 {
		t.Errorf("%d pixels were composed for writes the picture does not depend on", r.io.raster.done)
	}
}

func TestRasterBorderStripes(t *testing.T) {
	r := newRasterTest(t)
	r.io.nextRegWriteDirect(0x68, 0x00) // ULA on: the border shows
	r.io.WritePort(0x00FE, 0x02)        // red
	r.at(10, -32)
	r.io.WritePort(0x00FE, 0x01) // blue from line 10
	r.at(10, -32)
	r.io.WritePort(0x00FE, 0x01) // the same colour again composes nothing more
	img := r.frame()
	red := nrgbaToRGBA(r.io.nextPalette.lookup(nextLayerULA, 16+2).Colour())
	blue := nrgbaToRGBA(r.io.nextPalette.lookup(nextLayerULA, 16+1).Colour())
	r.want(img, 0, 9, red, "border above the change")
	r.want(img, 319, 9, red, "right border above the change")
	r.want(img, 0, 10, blue, "border from the change")
}

func TestRasterPaletteChangedDownTheFrame(t *testing.T) {
	// What the distribution's show512 does: one Layer 2 picture, its palette written again
	// half way down.
	r := newRasterTest(t)
	r.mem.nextPageWrite(16, 0, 0x01)       // Layer 2 pixel (0,0)
	r.mem.nextPageWrite(16+3, 0x800, 0x01) // and (0,104): bank 8 offset 104*256
	r.io.nextRegWriteDirect(0x69, 0x80)
	setColour := func(c uint8) {
		r.io.nextRegWriteDirect(0x43, 0x10)
		r.io.nextRegWriteDirect(0x40, 0x01)
		r.io.nextRegWriteDirect(0x41, c)
	}
	setColour(0xE0)
	r.at(32+96, -32)
	setColour(0x1C)
	img := r.frame()
	r.want(img, 32, 32, rasterRed, "Layer 2 pixel above the palette change")
	r.want(img, 32, 32+104, rasterGreen, "Layer 2 pixel below it")
}

func TestRasterDoubleResolutionComesOnDownTheFrame(t *testing.T) {
	r := newRasterTest(t)
	r.io.nextRegWriteDirect(0x4A, 0xE0)
	r.io.nextRegWriteDirect(0x4C, 0x00) // the empty tilemap's colour 0 is transparent
	r.at(100, -32)
	r.io.nextRegWriteDirect(0x4A, 0x1C)
	r.at(120, -32)
	r.io.nextRegWriteDirect(0x6B, 0xC0) // the tilemap, 80 columns: 640 across
	r.at(140, -32)
	r.io.nextRegWriteDirect(0x4A, 0x03)
	img := r.frame()
	if img.Rect.Dx() != 2*nextFrameWidth {
		t.Fatalf("the frame is %d across, want %d", img.Rect.Dx(), 2*nextFrameWidth)
	}
	r.want(img, 0, 50, rasterRed, "left, composed before the frame was widened")
	r.want(img, 639, 50, rasterRed, "right, composed before the frame was widened")
	r.want(img, 639, 110, rasterGreen, "second piece")
	r.want(img, 639, 200, rasterBlue, "last piece")
}

func TestRasterSpritesMovedDownTheFrame(t *testing.T) {
	// One sprite shown twice in a frame: moved after the beam has drawn it.
	r := newRasterTest(t)
	r.io.nextRegWriteDirect(0x15, 0x03) // sprites on, over the border
	for i := 0; i < 256; i++ {
		r.io.sprites.patterns[0][i] = 0xE0
	}
	place := func(y uint8) {
		r.io.nextRegWriteDirect(0x34, 0)
		r.io.nextRegWriteDirect(0x35, 40)
		r.io.nextRegWriteDirect(0x36, y)
		r.io.nextRegWriteDirect(0x37, 0)
		r.io.nextRegWriteDirect(0x38, 0x80)
	}
	place(20)
	r.at(100, -32)
	place(150)
	img := r.frame()
	sprite := nrgbaToRGBA(r.io.nextPalette.lookup(nextLayerSprites, 0xE0).Colour())
	r.want(img, 44, 24, sprite, "the sprite where it was when the beam passed")
	r.want(img, 44, 154, sprite, "and where it was moved to")
}

func TestCopperWaitAndMove(t *testing.T) {
	r := newRasterTest(t)
	r.io.nextRegWriteDirect(0x4A, 0xE0)
	r.copper(
		copperWait(10, 0), copperMove(0x4A, 0x1C),
		copperWait(20, 16), copperMove(0x4A, 0x03),
		copperHalt,
	)
	r.io.nextRegWriteDirect(0x62, 0x40) // start from 0, go round
	r.io.EndFrame()
	img := r.frame()
	r.want(img, 319, 41, rasterRed, "above the first WAIT")
	r.want(img, 0, 42, rasterGreen, "line 10, from its left border: the bar is the full width")
	r.want(img, 319, 51, rasterGreen, "line 19")
	r.want(img, 0, 52, rasterBlue, "line 20, all of it, for a WAIT at horizontal 16")
	r.want(img, 319, 255, rasterBlue, "bottom")

	// Going round means the list is not started again each frame: it is at its HALT.
	r.tm.t = 0
	r.io.StartFrame()
	r.io.nextRegWriteDirect(0x4A, 0xE0)
	r.io.EndFrame()
	img = r.frame()
	r.want(img, 32, 42, rasterRed, "second frame, start control 01: the list has halted")
	r.want(img, 319, 255, rasterRed, "second frame, bottom")
}

func TestCopperRestartsEachFrame(t *testing.T) {
	r := newRasterTest(t)
	r.copper(copperWait(0, 0), copperMove(0x4A, 0xE0), copperWait(96, 0), copperMove(0x4A, 0x1C), copperHalt)
	r.io.nextRegWriteDirect(0x62, 0xC0) // start from 0 and again at (0,0)
	r.newFrame()                        // the frame it was started in
	for frame := 1; frame <= 2; frame++ {
		r.io.EndFrame()
		img := r.frame()
		r.want(img, 0, 0, rasterGreen, "top border: the last value of the frame before")
		r.want(img, 32, 32, rasterRed, "line 0")
		r.want(img, 32, 32+96, rasterGreen, "line 96")
		r.tm.t = 0
		r.io.StartFrame()
	}
}

func TestCopperLineOffset(t *testing.T) {
	r := newRasterTest(t)
	r.io.nextRegWriteDirect(0x4A, 0xE0)
	r.io.nextRegWriteDirect(0x64, 32) // line 0 is the frame's first line
	r.copper(copperWait(0, 0), copperMove(0x4A, 0x1C), copperWait(255, 0), copperMove(0x4A, 0x03), copperHalt)
	r.io.nextRegWriteDirect(0x62, 0x40)
	r.io.EndFrame()
	img := r.frame()
	r.want(img, 0, 0, rasterGreen, "frame line 0 with an offset of 32")
	r.want(img, 32, 255, rasterBlue, "frame line 255")
}

func TestCopperProgramMemory(t *testing.T) {
	r := newRasterTest(t)
	// 0x63 writes a pair together when the second byte arrives.
	r.io.nextRegWriteDirect(0x61, 0x10)
	r.io.nextRegWriteDirect(0x62, 0x02) // address 0x210, stopped
	r.io.nextRegWriteDirect(0x63, 0xAB)
	if r.io.copper.mem[0x210] != 0 {
		t.Error("NextREG 0x63 wrote the first byte of a pair before the second arrived")
	}
	r.io.nextRegWriteDirect(0x63, 0xCD)
	if r.io.copper.mem[0x210] != 0xAB || r.io.copper.mem[0x211] != 0xCD {
		t.Errorf("NextREG 0x63 pair: memory holds %02X %02X, want AB CD", r.io.copper.mem[0x210], r.io.copper.mem[0x211])
	}
	r.io.nextRegs.selected = 0x61
	if got := r.io.nextRegReadSelected(); got != 0x12 {
		t.Errorf("NextREG 0x61 reads 0x%02X after two writes from 0x210, want 0x12", got)
	}
	r.io.nextRegs.selected = 0x62
	if got := r.io.nextRegReadSelected(); got != 0x02 {
		t.Errorf("NextREG 0x62 reads 0x%02X, want 0x02", got)
	}
	// The address wraps at 2K.
	r.io.nextRegWriteDirect(0x61, 0xFF)
	r.io.nextRegWriteDirect(0x62, 0x07)
	r.io.nextRegWriteDirect(0x60, 0x77)
	if r.io.copper.mem[0x7FF] != 0x77 || r.io.copper.addr != 0 {
		t.Errorf("a write at 0x7FF: memory 0x%02X, address then 0x%03X; want 0x77 and 0", r.io.copper.mem[0x7FF], r.io.copper.addr)
	}
}

func TestCopperSameStartControlDoesNotStartAgain(t *testing.T) {
	r := newRasterTest(t)
	r.io.nextRegWriteDirect(0x4A, 0xE0)
	r.copper(copperWait(10, 0), copperMove(0x4A, 0x1C), copperHalt)
	r.io.nextRegWriteDirect(0x62, 0x40)
	r.at(32+50, 0)
	r.io.nextRegWriteDirect(0x4A, 0xE0) // the CPU: the copper has run to here first
	if r.io.copper.pc != 2 {
		t.Fatalf("the copper is at instruction %d by line 50, want 2 (its HALT)", r.io.copper.pc)
	}
	r.io.nextRegWriteDirect(0x62, 0x40)
	if r.io.copper.pc != 2 {
		t.Error("writing the same start control started the list again")
	}
	r.io.nextRegWriteDirect(0x62, 0x00)
	r.io.nextRegWriteDirect(0x62, 0x80) // start from where it was
	if r.io.copper.pc != 2 {
		t.Error("start control 10 moved the copper from where it stopped")
	}
	r.io.nextRegWriteDirect(0x62, 0x40)
	if r.io.copper.pc != 0 {
		t.Error("start control 01 after another did not start from instruction 0")
	}
}

func TestCopperWithoutWaitsEndsWithTheFrame(t *testing.T) {
	// A list with no WAIT goes round for the whole frame: about 140,000 MOVEs. It must
	// come to the frame's end, and its last write is what is left.
	r := newRasterTest(t)
	r.copper(copperMove(0x4A, 0xE0), copperMove(0x4A, 0x1C))
	for i := 2; i < copperInstructions; i++ {
		r.io.nextRegWriteDirect(0x60, 0x4A)
		r.io.nextRegWriteDirect(0x60, 0x03)
	}
	r.io.nextRegWriteDirect(0x62, 0x40)
	r.io.EndFrame()
	if r.io.copper.clock != 0 {
		t.Errorf("after the frame's end the copper's clock is %d, want 0", r.io.copper.clock)
	}
	img := r.frame()
	// Each MOVE is half a pixel: the first two colours are a pixel, then 511 pixels of blue.
	r.want(img, 100, 200, rasterBlue, "in the long run of the same value")
}

func TestCopperStopsOnReset(t *testing.T) {
	r := newRasterTest(t)
	r.copper(copperMove(0x4A, 0xE0))
	r.io.nextRegWriteDirect(0x62, 0xC0)
	r.io.copperStop()
	if r.io.copper.mode != 0 || r.io.copper.mem[0] != 0x4A {
		t.Errorf("after a reset: start control %d, first byte 0x%02X; want stopped with the program kept", r.io.copper.mode, r.io.copper.mem[0])
	}
}

func TestVideoLineRegisterTakesTheOffset(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	zx.frameOrigin = zx.cpu.Cycles
	if got := zx.nextVideoLine(); got != (zx.rasterTiming().lines - zx.rasterTiming().firstPixelLine) {
		t.Fatalf("line at the frame's origin is %d", got)
	}
	before := zx.nextVideoLine()
	zx.io.nextRegs.regs[0x64] = 32
	if got := zx.nextVideoLine(); got != (before+32)%zx.rasterTiming().lines {
		t.Errorf("with NextREG 0x64 = 32 the line is %d, want %d", got, (before+32)%zx.rasterTiming().lines)
	}
}
