package zenzx

import (
	"image/color"

	"github.com/ha1tch/zenui/zxpalette"
)

// LoRes: the Next's 128x96 picture, in place of the ULA's.
//
// NextREG 0x15 bit 7 turns it on. It takes the ULA's place in the frame: the same 256x192
// area, the same clip window (NextREG 0x1A), the same place in the layer order, the same
// border round it. Each of its pixels is two frame pixels wide and two lines high. From
// the Next's registers.txt:
//
//	0x15 bit 7   LoRes on
//	0x32, 0x33   X and Y scroll, 0-255 and 0-191, in frame pixels: half a LoRes pixel
//	0x6A bit 5   Radastan mode: 16 colours, a nibble a pixel, 6144 bytes
//	     bit 4   Radastan: use the other Timex display file (exclusive or with the one
//	             port 0xFF's bit 0 chooses)
//	     bits 3-0  palette offset: the high four bits of a Radastan pixel's colour, and
//	             added to the high four bits of an 8-bit pixel's
//
// The standard mode has a byte a pixel, a colour of the ULA palette: the top 48 lines are
// the 6144 bytes at 0x4000 and the bottom 48 the 6144 at 0x6000, both in bank 5. Radastan
// mode has all 96 lines in one of those two blocks, 64 bytes a line, the left pixel in the
// high nibble. A pixel whose colour is the global transparency colour lets what is under
// it show, as for the ULA.

func (io *SpectrumIO) loresEnabled() bool {
	return io.memory != nil && io.memory.isNext && io.nextRegs.regs[0x15]&0x80 != 0
}

// loresPixel is LoRes's pixel at a point of the 256x192 picture.
func (io *SpectrumIO) loresPixel(x, y int) (color.RGBA, bool) {
	control := io.nextRegs.regs[0x6A]
	lx := (x + int(io.nextRegs.regs[0x32])) % 256 / 2
	ly := (y + int(io.nextRegs.regs[0x33])) % 192 / 2
	var index uint8
	if control&0x20 != 0 { // Radastan
		file := int(io.nextTimexPort&0x01) ^ int(control>>4&0x01)
		b := io.bank5(file*0x2000 + ly*64 + lx/2)
		nibble := b >> 4
		if lx&1 == 1 {
			nibble = b & 0x0F
		}
		index = control&0x0F<<4 | nibble
	} else {
		offset := ly*128 + lx
		if ly >= 48 {
			offset = 0x2000 + (ly-48)*128 + lx
		}
		index = io.bank5(offset) + control&0x0F<<4
	}
	rgb := io.nextPalette.lookup(nextLayerULA, index)
	return nrgbaToRGBA(rgb.Colour()), zxpalette.EncodeRGB9To8Bit(rgb) != io.nextRegs.regs[0x14]
}
