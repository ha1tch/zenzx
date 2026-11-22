package zenzx

import (
	"image"
	"image/color"
)

// The Timex screen modes on the Next (T-44).
//
// The Next's ULA has the Timex SCLD's modes, chosen through port 0xFF (bits 5-0 are also
// NextREG 0x69 bits 5-0). NextZXOS uses the high resolution one for the editor's 64 and
// 85 column modes. From the Next's ports.txt:
//
//	bits 2-0  000 screen 0: the standard screen at 0x4000
//	          001 screen 1: a standard screen at 0x6000
//	          010 hi-colour: pixels at 0x4000, an attribute for every pixel row at 0x6000
//	          110 hi-res: 512x192 in two colours, even columns at 0x4000, odd at 0x6000
//	bits 5-3  hi-res only: the ink colour, shown bright; the paper is its complement
//
// Both screens are in bank 5. This file draws the picture straight from that bank through
// the Next's ULA palette; it is used by DecodeDisplay, so by the headless build. The GUI's
// own drawing path does not use it yet.

const (
	nextTimexScreen1  = 0x01
	nextTimexHiColour = 0x02
	nextTimexHiRes    = 0x06
)

// nextTimexMode is the ULA screen mode a Next is in: 0 for the standard screen.
func (io *SpectrumIO) nextTimexMode() uint8 {
	if io.memory == nil || !io.memory.isNext {
		return 0
	}
	return io.nextTimexPort & 0x07
}

// nextTimexPortWrite takes a write to port 0xFF on a Next, or to NextREG 0x69's low bits.
func (io *SpectrumIO) nextTimexPortWrite(value uint8) {
	io.nextTimexPort = value
	io.nextRegs.regs[0x69] = io.nextRegs.regs[0x69]&0xC0 | value&0x3F
}

// bank5 reads a byte of bank 5, where both Timex screens live.
func (io *SpectrumIO) bank5(offset int) uint8 {
	return io.memory.nextPageRead(uint8(10+offset>>13), uint16(offset&0x1FFF))
}

func (io *SpectrumIO) ulaColour(index uint8) color.RGBA {
	return nrgbaToRGBA(io.nextPalette.lookup(nextLayerULA, index).Colour())
}

// DecodeNextTimex draws the Timex mode a Next is in, or returns nil in the standard mode,
// which the ordinary renderer draws. The picture is 256x192, or 512x192 in hi-res.
func (io *SpectrumIO) DecodeNextTimex(screen *SpectrumScreen) *image.RGBA {
	return io.decodeNextTimexLines(screen, 0, ScreenHeight-1, nil)
}

// decodeNextTimexLines is DecodeNextTimex for lines y0 to y1 of the picture only. With
// scratch it draws into the pictures kept there (one of each width), made when first
// needed, and leaves their other lines as they were; without, into a new picture.
func (io *SpectrumIO) decodeNextTimexLines(screen *SpectrumScreen, y0, y1 int, scratch *[2]*image.RGBA) *image.RGBA {
	if y0 < 0 {
		y0 = 0
	}
	if y1 > ScreenHeight-1 {
		y1 = ScreenHeight - 1
	}
	picture := func(wide int) *image.RGBA {
		if scratch == nil {
			return image.NewRGBA(image.Rect(0, 0, (1+wide)*ScreenWidth, ScreenHeight))
		}
		if scratch[wide] == nil {
			scratch[wide] = image.NewRGBA(image.Rect(0, 0, (1+wide)*ScreenWidth, ScreenHeight))
		}
		return scratch[wide]
	}
	mode := io.nextTimexMode()
	flashOn := screen != nil && screen.flashEnabled && screen.flashTickTock
	switch mode {
	case nextTimexHiRes:
		img := picture(1)
		ink := (io.nextTimexPort >> 3) & 0x07
		inkC, paperC := io.ulaColour(8+ink), io.ulaColour(24+(7-ink))
		for y := y0; y <= y1; y++ {
			row := (y&0xC0)<<5 | (y&0x07)<<8 | (y&0x38)<<2
			for col := 0; col < 64; col++ {
				b := io.bank5((col&1)*0x2000 + row + col>>1)
				for bit := 0; bit < 8; bit++ {
					c := paperC
					if b&(0x80>>uint(bit)) != 0 {
						c = inkC
					}
					img.SetRGBA(col*8+bit, y, c)
				}
			}
		}
		return img
	case nextTimexScreen1, nextTimexHiColour:
		img := picture(0)
		for y := y0; y <= y1; y++ {
			row := (y&0xC0)<<5 | (y&0x07)<<8 | (y&0x38)<<2
			for col := 0; col < 32; col++ {
				var b, attr uint8
				if mode == nextTimexScreen1 {
					b, attr = io.bank5(0x2000+row+col), io.bank5(0x2000+0x1800+(y/8)*32+col)
				} else {
					b, attr = io.bank5(row+col), io.bank5(0x2000+row+col)
				}
				if attr&0x80 != 0 && flashOn {
					b = ^b
				}
				bright := (attr >> 3) & 0x08
				inkC, paperC := io.ulaColour(bright+attr&0x07), io.ulaColour(16+bright+(attr>>3)&0x07)
				for bit := 0; bit < 8; bit++ {
					c := paperC
					if b&(0x80>>uint(bit)) != 0 {
						c = inkC
					}
					img.SetRGBA(col*8+bit, y, c)
				}
			}
		}
		return img
	}
	return nil
}
