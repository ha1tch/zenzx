package zenzx

import "image"

// The Next's picture, drawn as the beam goes down it.
//
// ComposeNextFrame (nextframe.go) draws a frame from the registers as they are at one
// moment. A program can change them part of the way down the screen, with the copper
// (nextcopper.go), from a line interrupt, or by timing its own writes, and what is above
// the change then keeps the old value on the display: that is how a Next shows more than
// 256 colours of a palette at once, a sky that fades, a status panel that does not scroll.
//
// So the frame is drawn in pieces. Just before anything that the picture depends on is
// changed, the part of the frame the beam has already passed is composed, from the state
// as it still is, and kept. What the front ends show (NextFrame) is those kept pieces and,
// for the rest of the frame, the state as it is when they ask. A frame in which nothing
// changes is composed once, when it is shown, exactly as before; a frame that is never
// shown and never changed is not composed at all.
//
// The beam's position comes from the T-state count in the frame, with the origin and line
// length the ULA interrupt and NextREG 0x1E/0x1F use (nextvideoline.go). The 320x256 frame
// is the pixel area with 32 pixels of border on each side; in time, a line of the frame is
// its left border (the last 16 T-states of the raster line before), then the pixel area
// and the right border (the first 144 T-states of its own).
//
// The pieces are whole lines, as CSpect draws them, and CSpect is the reference for what a
// Next's picture looks like: a line is drawn from the state at the end of its right
// border, so a change made anywhere on a line before that shows on all of the line, its
// left border included, and a change made after it shows from the next line. A copper
// list that waits for horizontal 0 of each line therefore gives clean bars the full width
// of the frame. The machine itself changes at the pixel; rasterIndex is where that would
// be done.
//
// A write by the CPU is placed at the start of the instruction that makes it, which is as
// much as the CPU core reports: up to 20 T-states early, which matters only near the end
// of a line.
//
// What is kept is pixels, so the pieces are from the memory as it was when each was
// composed, not as it was when the beam drew each line: a program that redraws Layer 2
// while the beam is on it is not shown tearing as it would on the machine.

// nextRaster is the part of the current frame already composed.
type nextRaster struct {
	on      bool        // a Next with timing and a screen to draw from
	skip    bool        // this frame is not being kept (the lean tape loading frame)
	done    int         // pixels composed, in beam order: line*320 + column
	scale   int         // horizontal resolution of img, 1 or 2
	img     *image.RGBA // 320*scale by 256; lines and pixels below done are valid
	sprites *image.RGBA // scratch for the sprites' lines, 320x256
	timex   [2]*image.RGBA
	ended   bool // the frame's end has been seen (EndFrame)
}

const nextFramePixels = nextFrameWidth * nextFrameHeight

// nextRasterTiming is the beam's place in the frame and the frame's shape, in T-states.
type nextRasterTiming struct {
	t              int // T-states since the frame's origin
	tPerLine       int
	firstPixelLine int // the raster line of the pixel area's first line
	lines          int
}

// rasterIndex turns a time in the frame, in 28 MHz clocks (8 to a T-state), into the
// number of frame pixels that are settled by then: whole lines, each settled when the beam
// leaves its right border.
func rasterIndex(clock int, tm nextRasterTiming) int {
	lineClocks := tm.tPerLine * 8
	r, px := clock/lineClocks, (clock%lineClocks)/4 // 4 clocks to a pixel
	y := r - tm.firstPixelLine + nextFrameInset     // the frame line whose pixel area this raster line draws
	linePixels := tm.tPerLine * 2
	var idx int
	switch {
	case px < nextFrameWidth-nextFrameInset: // pixel area and right border
		idx = y*nextFrameWidth + nextFrameInset + px
	case px < linePixels-nextFrameInset: // off the frame: the line is finished
		idx = (y + 1) * nextFrameWidth
	default: // the next line's left border
		idx = (y+1)*nextFrameWidth + px - (linePixels - nextFrameInset)
	}
	if idx < 0 {
		return 0
	}
	if idx > nextFramePixels {
		return nextFramePixels
	}
	return idx - idx%nextFrameWidth // whole lines only
}

// rasterClock is the time in the frame now, in 28 MHz clocks.
func (io *SpectrumIO) rasterClock() (int, nextRasterTiming) {
	tm := io.rasterTimingFn()
	if tm.t < 0 {
		tm.t = 0
	}
	return tm.t * 8, tm
}

// rasterEnable turns drawing by pieces on: the front end has given the timing and the
// screen. Without them (a SpectrumIO made alone, as most tests make it) every frame is
// composed whole.
func (io *SpectrumIO) rasterEnable(timing func() nextRasterTiming, screen *SpectrumScreen) {
	io.rasterTimingFn, io.rasterScreen = timing, screen
}

func (io *SpectrumIO) rasterActive() bool {
	return io.rasterTimingFn != nil && io.rasterScreen != nil && io.memory != nil && io.memory.isNext
}

// rasterFrameStart begins a frame: nothing of it is composed yet.
func (io *SpectrumIO) rasterFrameStart() {
	if !io.rasterActive() {
		io.raster.on = false
		return
	}
	if io.raster.on && !io.raster.ended {
		io.rasterFrameEnd() // a frame that was left without its end: finish the copper's part
	}
	io.raster.on, io.raster.skip, io.raster.ended, io.raster.done = true, false, false, 0
}

// rasterFrameEnd is the end of the frame's T-states: the copper runs to the end of it.
// What the copper changes after the last line of the frame composes the rest of the
// frame first, like any other change.
func (io *SpectrumIO) rasterFrameEnd() {
	if !io.raster.on || io.raster.ended {
		return
	}
	tm := io.rasterTimingFn()
	io.copperFrameEnd(tm.tPerLine * tm.lines * 8)
	io.raster.ended = true
}

// rasterSync is called before the CPU changes something the picture depends on: the
// copper catches up with the CPU, and the frame is composed up to the beam.
func (io *SpectrumIO) rasterSync() {
	if !io.raster.on || io.raster.ended {
		return
	}
	if io.copper.busy { // the change is the copper's own: it is at its own place in the frame
		io.rasterFlush(io.copper.clock)
		return
	}
	clock, _ := io.rasterClock()
	io.copperAdvance(clock)
	io.rasterFlush(clock)
}

// rasterCopperSync lets the copper catch up with the CPU, without composing anything: for
// what the CPU reads, and for what it writes that the picture does not depend on.
func (io *SpectrumIO) rasterCopperSync() {
	if !io.raster.on || io.raster.ended || io.copper.busy || io.copper.mode == 0 {
		return
	}
	clock, _ := io.rasterClock()
	io.copperAdvance(clock)
}

// rasterFlush composes the frame up to the beam's place at a time in the frame.
func (io *SpectrumIO) rasterFlush(clock int) {
	r := &io.raster
	if r.skip {
		return
	}
	to := rasterIndex(clock, io.rasterTimingFn())
	if to <= r.done {
		return
	}
	need := io.nextFrameScale()
	switch {
	case r.img == nil || r.done == 0 && r.scale != need:
		if r.img == nil || r.scale != need {
			r.img = image.NewRGBA(image.Rect(0, 0, nextFrameWidth*need, nextFrameHeight))
		}
		r.scale = need
	case need > r.scale: // a double resolution layer came on part of the way down
		r.img, r.scale = widenFrame(r.img, r.done), need
	}
	io.composeNextRange(r.img, r.scale, r.done, to, nil, io.rasterScreen, io.borderColor)
	r.done = to
}

// widenFrame returns a frame of twice the horizontal resolution with the first done
// pixels (counted at the frame's 320 across) of img in it.
func widenFrame(img *image.RGBA, done int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, 2*nextFrameWidth, nextFrameHeight))
	for y := 0; y*nextFrameWidth < done; y++ {
		n := done - y*nextFrameWidth
		if n > nextFrameWidth {
			n = nextFrameWidth
		}
		src, dst := img.Pix[y*img.Stride:], out.Pix[y*out.Stride:]
		for x := 0; x < n; x++ {
			copy(dst[x*8:x*8+4], src[x*4:x*4+4])
			copy(dst[x*8+4:x*8+8], src[x*4:x*4+4])
		}
	}
	return out
}

// NextFrame is the Next's picture for the front ends: the pieces of the current frame
// composed so far, and the rest of it from the state as it is now. standard, screen and
// border are as for ComposeNextFrame.
func (io *SpectrumIO) NextFrame(standard func() *image.RGBA, screen *SpectrumScreen, border uint8) *image.RGBA {
	r := &io.raster
	if !r.on || r.skip || r.done == 0 || r.img == nil {
		return io.ComposeNextFrame(standard, screen, border)
	}
	if !r.ended { // asked for in the middle of a frame: the copper is brought up to the CPU
		io.rasterCopperSync()
	}
	scale := io.nextFrameScale()
	if r.scale > scale {
		scale = r.scale
	}
	var out *image.RGBA
	if scale > r.scale {
		out = widenFrame(r.img, r.done)
	} else {
		out = image.NewRGBA(r.img.Rect)
		lines := (r.done + nextFrameWidth - 1) / nextFrameWidth
		copy(out.Pix[:lines*out.Stride], r.img.Pix[:lines*r.img.Stride])
	}
	io.composeNextRange(out, scale, r.done, nextFramePixels, standard, screen, border)
	return out
}

// nextDisplayReg marks the registers the picture depends on: a write to one of them
// composes the frame up to the beam first. The rest (paging, the MMU, sound, the registers
// that only choose which entry or which sprite the next write is for) do not.
var nextDisplayReg = func() (m [256]bool) {
	for _, r := range []uint8{
		0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, // Layer 2, layers, clip windows
		0x26, 0x27, 0x2F, 0x30, 0x31, 0x32, 0x33, // scrolling
		0x35, 0x36, 0x37, 0x38, 0x39, 0x75, 0x76, 0x77, 0x78, 0x79, // sprite attributes
		0x41, 0x42, 0x43, 0x44, 0x4A, 0x4B, 0x4C, // palettes, fallback and transparency colours
		0x68, 0x69, 0x6A, 0x6B, 0x6C, 0x6E, 0x6F, 0x70, 0x71, // ULA, LoRes, Layer 2 and tilemap control
	} {
		m[r] = true
	}
	return
}()

// rasterBeforeRegWrite is called at the start of every NextREG write.
func (io *SpectrumIO) rasterBeforeRegWrite(reg uint8) {
	if !io.raster.on {
		return
	}
	if nextDisplayReg[reg] {
		io.rasterSync()
	} else {
		io.rasterCopperSync()
	}
}

// rasterBeforePortWrite is called at the start of a port write on a Next, for the ports
// the picture depends on: the border, the Timex screen modes, Layer 2's port and the
// sprite ports.
func (io *SpectrumIO) rasterBeforePortWrite(port uint16, value uint8) {
	if !io.raster.on {
		return
	}
	switch {
	case port&0xFF == 0xFF, port == 0x303B, port&0xFF == 0x57, port&0xFF == 0x5B, port == layer2AccessPortAddr:
	case port&0x01 == 0 && value&0x07 != io.borderColor:
	default:
		return
	}
	io.rasterSync()
}
