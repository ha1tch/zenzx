package zenzx

import (
	"image"
	"image/color"

	"github.com/ha1tch/zenui/zxpalette"
)

// The whole picture of a Next, composed by software.
//
// A Next's layers do not share the 256x192 picture of the classic machines. The display is
// 320x256: the classic picture sits 32 pixels in from the left and from the top, and what is
// round it is border. Layer 2 in its two wide modes, the tilemap, and the sprites (when
// they are allowed over the border) cover all of it. Some layers have twice the horizontal
// resolution: Timex hi-res, the 80 column tilemap, Layer 2 at 640x256.
//
// ComposeNextFrame draws all of that into one image, the frame, 320x256 or 640x256 when a
// double resolution layer is on. Both front ends show it for a Next, always
// (nextFrameActive): the headless build as its screenshot, the GUI as one texture over the
// whole picture area in place of its cell drawing and its border. One function,
// so the two cannot differ.
//
// The registers, from the Next's registers.txt:
//
//	0x12        Layer 2's first 16K bank
//	0x14        global transparency colour: a Layer 2 or ULA pixel of this colour (the top
//	            eight bits of its nine) lets what is under it show
//	0x15        bit 0 sprites on, bit 1 sprites over the border, bit 5 clip them there,
//	            bits 4-2 the order of sprites (S), Layer 2 (L) and ULA with tilemap (U)
//	0x16, 0x71  Layer 2 X scroll, low byte and bit 8      0x17  Layer 2 Y scroll
//	0x18        Layer 2 clip window    0x19  sprites'    0x1A  the ULA's    0x1B  tilemap's
//	            X1, X2, Y1, Y2 in four writes, inclusive. In the wide Layer 2 modes, for
//	            the tilemap, and for sprites clipped over the border, X counts in units
//	            of two pixels from the frame's left edge; otherwise the window is in the
//	            classic picture's coordinates.
//	0x4A        fallback colour, shown where every layer is transparent
//	0x68        bit 7 ULA off, bit 0 stencil mode (ULA AND tilemap)
//	0x69        bit 7 Layer 2 on (also port 0x123B bit 1)
//	0x6B        tilemap control (nexttilemap.go)
//	0x70        bits 5-4 Layer 2 mode: 256x192, 320x256, 640x256 with 16 colours
//
// LoRes (NextREG 0x15 bit 7) takes the ULA's place: nextlores.go.
//
// Not drawn: the two blending orders (NextREG 0x15 bits 4-2 =
// 110 and 111, taken as S L U without the blend), the ULA's own scrolling (0x26, 0x27),
// and the Layer 2 palette's priority bit.

const (
	nextFrameWidth  = 320
	nextFrameHeight = 256
	nextFrameInset  = 32 // the classic picture's left and top edge in the frame
)

// nextFrameActive reports whether the picture comes from ComposeNextFrame: on a Next,
// always. A Next's colours are its palettes', its picture includes the border, and any of
// its layers can be turned on at any moment, so the classic machines' drawing, with its
// fixed colours and 256x192 picture, is never the right one for it.
func (io *SpectrumIO) nextFrameActive() bool {
	return io.memory != nil && io.memory.isNext
}

// nextFrameScale is the frame's horizontal resolution: 2 when a layer with 640 (or 512)
// pixels across is on, 1 otherwise.
func (io *SpectrumIO) nextFrameScale() int {
	if io.layer2Enabled() && io.layer2CurrentMode() == layer2Mode640x256x4bpp {
		return 2
	}
	if io.tilemapEnabled() && io.tilemapWide() {
		return 2
	}
	if io.memory != nil && io.memory.isNext && io.nextTimexMode() == nextTimexHiRes {
		return 2
	}
	return 1
}

// layer2Scroll returns the Layer 2 scroll offsets.
func (io *SpectrumIO) layer2Scroll() (x, y int) {
	return int(io.nextRegs.regs[0x71]&0x01)<<8 | int(io.nextRegs.regs[0x16]), int(io.nextRegs.regs[0x17])
}

// layer2FramePixel returns Layer 2's pixel at a point of the frame: x in frame pixels at
// the frame's own horizontal resolution (scale 1 or 2), y from 0 to 255.
func (io *SpectrumIO) layer2FramePixel(x, y, scale int) (color.RGBA, bool) {
	x1, x2, y1, y2 := io.layer2ClipWindow()
	sx, sy := io.layer2Scroll()
	var px, py int
	switch io.layer2CurrentMode() {
	case layer2Mode320x256x8bpp:
		lx := x / scale
		if lx < int(x1)*2 || lx > int(x2)*2+1 || y < int(y1) || y > int(y2) {
			return color.RGBA{}, false
		}
		px, py = (lx+sx)%320, (y+sy)%256
	case layer2Mode640x256x4bpp:
		lx := x * 2 / scale // 640 across
		if lx/2 < int(x1)*2 || lx/2 > int(x2)*2+1 || y < int(y1) || y > int(y2) {
			return color.RGBA{}, false
		}
		px, py = ((lx/2+sx)%320)*2+lx%2, (y+sy)%256
	default:
		lx, ly := x/scale-nextFrameInset, y-nextFrameInset
		if lx < int(x1) || lx > int(x2) || ly < int(y1) || ly > int(y2) || lx > 255 || ly > 191 {
			return color.RGBA{}, false
		}
		px, py = (lx+sx)%256, (ly+sy%192)%192
	}
	rgb := io.nextPalette.lookup(nextLayerLayer2, uint8(io.memory.layer2Pixel(io, px, py)))
	if zxpalette.EncodeRGB9To8Bit(rgb) == io.nextRegs.regs[0x14] {
		return color.RGBA{}, false
	}
	return nrgbaToRGBA(rgb.Colour()), true
}

// DecodeSpritesFrame draws the sprites on the frame, 320x256, where a sprite at (32,32)
// has its corner on the classic picture's. nil when the sprites are off. Without "sprites
// over border" they show only on the classic picture, inside their clip window; with it
// they show on the whole frame, or inside the clip window when NextREG 0x15 bit 5 asks.
func (io *SpectrumIO) DecodeSpritesFrame() *image.RGBA {
	if !io.spriteEnabled() {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, nextFrameWidth, nextFrameHeight))
	io.decodeSpriteLines(img, 0, nextFrameHeight-1)
	return img
}

// decodeSpriteLines draws lines y0 to y1 of the sprites' frame into img, which is 320x256,
// clearing them first. The other lines are left as they were.
func (io *SpectrumIO) decodeSpriteLines(img *image.RGBA, y0, y1 int) {
	clear(img.Pix[y0*img.Stride : (y1+1)*img.Stride])
	nr15 := io.GetNextRegister(0x15)
	clip := io.nextClipSprites.values
	minX, maxX, minY, maxY := 0, nextFrameWidth-1, 0, nextFrameHeight-1
	switch {
	case nr15&0x02 == 0: // on the classic picture only
		minX, maxX = nextFrameInset+int(clip[0]), nextFrameInset+int(clip[1])
		minY, maxY = nextFrameInset+int(clip[2]), nextFrameInset+int(clip[3])
		if maxY > nextFrameInset+ScreenHeight-1 {
			maxY = nextFrameInset + ScreenHeight - 1
		}
	case nr15&0x20 != 0: // over the border, clipped: X doubled, from the frame's corner
		minX, maxX, minY, maxY = int(clip[0])*2, int(clip[1])*2+1, int(clip[2]), int(clip[3])
	}
	if maxX > nextFrameWidth-1 {
		maxX = nextFrameWidth - 1
	}
	if minY < y0 {
		minY = y0
	}
	if maxY > y1 {
		maxY = y1
	}
	for _, r := range io.sprites.effectiveAttrs() {
		if !r.visible {
			continue
		}
		w, h := spriteFootprint(r)
		for oy := 0; oy < h; oy++ {
			py := (r.y + oy) & 511 // the positions are nine bits and go round: a sprite at 500 shows its right at the left
			if py < minY || py > maxY {
				continue
			}
			for ox := 0; ox < w; ox++ {
				px := (r.x + ox) & 511
				if px < minX || px > maxX {
					continue
				}
				if c, opaque := io.spritePixel(r, ox, oy); opaque {
					img.SetRGBA(px, py, color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A})
				}
			}
		}
	}
}

// ComposeNextFrame draws the frame. standard draws the classic 256x192 picture, for when
// the screen is not given or a Timex mode is not in use; screen, when not nil, lets the
// classic picture's pixels go through the Next's ULA palette, which is what can make them
// transparent; border is the border colour, 0 to 7.
//
// It draws all of the frame from the state as it is now. A frame in which the state
// changes part of the way down is put together from pieces by NextFrame (nextraster.go).
func (io *SpectrumIO) ComposeNextFrame(standard func() *image.RGBA, screen *SpectrumScreen, border uint8) *image.RGBA {
	scale := io.nextFrameScale()
	out := image.NewRGBA(image.Rect(0, 0, nextFrameWidth*scale, nextFrameHeight))
	io.composeNextRange(out, scale, 0, nextFramePixels, standard, screen, border)
	return out
}

// composeNextRange draws part of the frame into out, which is 320*scale by 256: the
// pixels from and up to (not including) to, counted in the order the beam draws them at
// 320 to a line. scale is the frame's horizontal resolution and must be at least
// nextFrameScale().
func (io *SpectrumIO) composeNextRange(out *image.RGBA, scale, from, to int, standard func() *image.RGBA, screen *SpectrumScreen, border uint8) {
	if to <= from {
		return
	}
	w := nextFrameWidth * scale
	y0, y1 := from/nextFrameWidth, (to-1)/nextFrameWidth

	transparent := io.nextRegs.regs[0x14]
	fallback := nrgbaToRGBA(zxpalette.DecodeRGB9From8Bit(io.nextRegs.regs[0x4A]).Colour())
	transparentRGBA := nrgbaToRGBA(zxpalette.DecodeRGB9From8Bit(transparent).Colour())

	// The ULA: the classic picture or a Timex one, with the border round it.
	ulaOff := io.nextRegs.regs[0x68]&0x80 != 0
	stencil := io.nextRegs.regs[0x68]&0x01 != 0 && !ulaOff && io.tilemapEnabled()
	var ula *image.RGBA
	fromScreen := false
	if !ulaOff {
		if ula = io.decodeNextTimexLines(screen, y0-nextFrameInset, y1-nextFrameInset, &io.raster.timex); ula == nil {
			fromScreen = screen != nil
			if !fromScreen {
				ula = standard()
			}
		}
	}
	ulaStep := 1 // frame pixels to a ULA pixel
	if ula == nil || ula.Rect.Dx() == ScreenWidth {
		ulaStep = scale
	}
	borderIndex := 16 + border&0x07
	if io.nextPalette.ulaNextEnabled { // ULANext: the border is a paper, and papers start at 128
		borderIndex = 128 + border&0x07
	}
	borderRGB := io.nextPalette.lookup(nextLayerULA, borderIndex)
	borderColour, borderOpaque := nrgbaToRGBA(borderRGB.Colour()), zxpalette.EncodeRGB9To8Bit(borderRGB) != transparent
	uc := io.nextClipULA.values
	lores := io.loresEnabled()
	ulaPixel := func(x, y int) (color.RGBA, bool) {
		if ulaOff {
			return color.RGBA{}, false
		}
		ux, uy := x-nextFrameInset*scale, y-nextFrameInset
		if ux < 0 || ux >= ScreenWidth*scale || uy < 0 || uy >= ScreenHeight {
			return borderColour, borderOpaque
		}
		if cx := ux / scale; cx < int(uc[0]) || cx > int(uc[1]) || uy < int(uc[2]) || uy > int(uc[3]) {
			return color.RGBA{}, false
		}
		if lores {
			return io.loresPixel(ux/scale, uy) // nextlores.go
		}
		if fromScreen {
			return io.ulaPalettePixel(screen, ux/ulaStep, uy)
		}
		c := ula.RGBAAt(ula.Rect.Min.X+ux/ulaStep, ula.Rect.Min.Y+uy)
		return c, c != transparentRGBA
	}

	tilemapOn := io.tilemapEnabled()
	tmStep := scale // frame pixels to a tilemap pixel
	if io.tilemapWide() {
		tmStep = 1
	}
	// The U layer: ULA and tilemap together, as one place in the order.
	uPixel := func(x, y int) (color.RGBA, bool) {
		u, uOpaque := ulaPixel(x, y)
		if !tilemapOn {
			return u, uOpaque
		}
		t, tOpaque, below := io.tilemapPixel(x/tmStep, y)
		switch {
		case stencil && tOpaque && uOpaque:
			return color.RGBA{R: u.R & t.R, G: u.G & t.G, B: u.B & t.B, A: 0xFF}, true
		case stencil:
			return color.RGBA{}, false
		case tOpaque && (!below || !uOpaque):
			return t, true
		}
		return u, uOpaque
	}

	layer2On := io.layer2Enabled()
	var sprites *image.RGBA
	if io.spriteEnabled() {
		if io.raster.sprites == nil {
			io.raster.sprites = image.NewRGBA(image.Rect(0, 0, nextFrameWidth, nextFrameHeight))
		}
		sprites = io.raster.sprites
		io.decodeSpriteLines(sprites, y0, y1)
	}
	order := nextLayerOrder(io.GetNextRegister(0x15))
	for y := y0; y <= y1; y++ {
		xa, xb := 0, w
		if y == y0 {
			xa = (from % nextFrameWidth) * scale
		}
		if y == y1 {
			xb = ((to-1)%nextFrameWidth + 1) * scale
		}
		for x := xa; x < xb; x++ {
			chosen := fallback
			for _, layer := range order { // bottom to top: the last opaque one shows
				var c color.RGBA
				opaque := false
				switch layer {
				case nextLayerULA:
					c, opaque = uPixel(x, y)
				case nextLayerLayer2:
					if layer2On {
						c, opaque = io.layer2FramePixel(x, y, scale)
					}
				case nextLayerSprites:
					if sprites != nil {
						c = sprites.RGBAAt(x/scale, y)
						opaque = c.A != 0
					}
				}
				if opaque {
					chosen = c
				}
			}
			out.SetRGBA(x, y, chosen)
		}
	}
}
