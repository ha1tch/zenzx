package zenzx

import (
	"image"
	"testing"
)

// A sprite's position is nine bits and goes round: at X 504 its first eight columns are off
// the right of everything and its last eight are the first eight of the frame.
func TestSpritePositionsGoRound(t *testing.T) {
	io := newNextIO()
	io.nextRegWriteDirect(0x15, 0x03) // sprites on, over the border
	io.WritePort(0x303B, 0)
	for i := 0; i < 256; i++ {
		io.WritePort(0x5B, 0x1C) // pattern 0: all green
	}
	io.WritePort(0x303B, 0)
	io.WritePort(0x57, 504&0xFF) // X low
	io.WritePort(0x57, 40)       // Y
	io.WritePort(0x57, 0x01)     // X bit 8
	io.WritePort(0x57, 0x80)     // visible, pattern 0
	img := image.NewRGBA(image.Rect(0, 0, nextFrameWidth, nextFrameHeight))
	io.decodeSpriteLines(img, 0, nextFrameHeight-1)
	at := func(x, y int) bool { return img.RGBAAt(x, y).A != 0 }
	if !at(0, 40) || !at(7, 55) {
		t.Error("the part of the sprite past 511 is not at the left of the frame")
	}
	if at(8, 40) || at(319, 40) {
		t.Error("the sprite is drawn where it should not be")
	}
}
