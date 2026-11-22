package zenzx

import "testing"

// The sprite ports (spriteports.go): 0x303B, 0x57 and 0x5B, and the NextREG registers that
// go with them.

func TestSpritePatternPort(t *testing.T) {
	f := newFrameTest(t)
	f.io.WritePort(0x303B, 2) // pattern 2
	for i := 0; i < 256; i++ {
		f.io.WritePort(0x005B, uint8(i))
	}
	f.io.WritePort(0xFF5B, 0xAA) // the 257th byte is the first of pattern 3; the high byte is not decoded
	p := f.io.sprites.patterns
	if p[2][0] != 0 || p[2][1] != 1 || p[2][255] != 255 || p[3][0] != 0xAA {
		t.Fatalf("pattern 2 holds %d %d ... %d and pattern 3 starts %#x", p[2][0], p[2][1], p[2][255], p[3][0])
	}
	if p[1][255] != 0 || p[3][1] != 0 {
		t.Fatal("the pattern upload reached bytes it should not have")
	}
	// Bit 7 starts half way through the pattern, for the second of two 4-bit patterns.
	f.io.WritePort(0x303B, 0x80|5)
	f.io.WritePort(0x005B, 0x77)
	if p := f.io.sprites.patterns[5]; p[128] != 0x77 || p[0] != 0 {
		t.Fatalf("with bit 7 set the byte went to offset 0 (%#x) and not 128 (%#x)", p[0], p[128])
	}
	// The address wraps at the end of pattern memory.
	f.io.WritePort(0x303B, 63)
	for i := 0; i < 257; i++ {
		f.io.WritePort(0x005B, 0x11)
	}
	if f.io.sprites.patterns[0][0] != 0x11 {
		t.Fatal("the pattern address did not wrap from pattern 63 to pattern 0")
	}
}

func TestSpriteAttributePort(t *testing.T) {
	f := newFrameTest(t)
	f.io.WritePort(0x303B, 10)
	// Sprite 10: four bytes, because the fourth has bit 6 clear.
	for _, v := range []uint8{40, 50, 0x30, 0x80 | 7} {
		f.io.WritePort(0x0057, v)
	}
	// Sprite 11 follows without another select: five bytes, bit 6 set.
	for _, v := range []uint8{60, 70, 0x00, 0x80 | 0x40 | 9, 0x0A} {
		f.io.WritePort(0x0057, v)
	}
	// And sprite 12.
	f.io.WritePort(0x0057, 99)
	a := f.io.sprites.attrs
	if a[10].attr0 != 40 || a[10].attr1 != 50 || a[10].attr2 != 0x30 || a[10].attr3 != 0x87 || a[10].attr4 != 0 {
		t.Fatalf("sprite 10 is %+v", a[10])
	}
	if a[11].attr0 != 60 || a[11].attr3 != 0xC9 || a[11].attr4 != 0x0A {
		t.Fatalf("sprite 11 is %+v", a[11])
	}
	if a[12].attr0 != 99 {
		t.Fatalf("the byte after sprite 11's five went to %+v, not to sprite 12", a[12])
	}
	// From 127 the current sprite wraps to 0.
	f.io.WritePort(0x303B, 127)
	for _, v := range []uint8{1, 2, 3, 0x80, 5} {
		f.io.WritePort(0x0057, v)
	}
	if a := f.io.sprites.attrs; a[127].attr0 != 1 || a[0].attr0 != 5 {
		t.Fatalf("after sprite 127 the next byte went to %+v", a[0])
	}
	if got := f.io.ReadPort(0x303B); got != 0 {
		t.Fatalf("port 0x303B reads %#x, want 0: no collision, no overflow", got)
	}
}

func TestSpritePortsReachThePicture(t *testing.T) {
	// What a program does: load a pattern, place the sprite, turn sprites on.
	f := newFrameTest(t)
	f.palette(2, 0x1C, 0x1C) // green
	f.io.WritePort(0x303B, 0)
	for i := 0; i < 256; i++ {
		f.io.WritePort(0x005B, 0x1C)
	}
	f.io.WritePort(0x303B, 0)
	for _, v := range []uint8{100, 80, 0x00, 0x80} { // x, y, no flags, visible with pattern 0
		f.io.WritePort(0x0057, v)
	}
	f.io.nextRegWriteDirect(0x15, 0x01)
	img := f.frame()
	f.want(img, 100, 80, frameGreen, "sprite's corner")
	f.want(img, 115, 95, frameGreen, "sprite's far corner")
	if img.RGBAAt(116, 80) == frameGreen || img.RGBAAt(99, 80) == frameGreen {
		t.Error("the sprite is wider than 16 pixels or starts early")
	}
}

func TestSpriteRegistersWithIncrementAndLockstep(t *testing.T) {
	f := newFrameTest(t)
	// NextREG 0x75-0x79 write as 0x35-0x39 do, and the sprite number then goes up.
	f.io.nextRegWriteDirect(0x34, 20)
	f.io.nextRegWriteDirect(0x75, 11)
	f.io.nextRegWriteDirect(0x75, 22)
	a := f.io.sprites.attrs
	if a[20].attr0 != 11 || a[21].attr0 != 22 || f.io.sprites.selected != 22 || f.io.GetNextRegister(0x34) != 22 {
		t.Fatalf("after two writes to NextREG 0x75: sprites 20 and 21 have X %d and %d, selected %d", a[20].attr0, a[21].attr0, f.io.sprites.selected)
	}
	// Without lockstep the register and the port keep sprite numbers of their own.
	f.io.WritePort(0x303B, 50)
	f.io.nextRegWriteDirect(0x35, 77)
	f.io.WritePort(0x0057, 88)
	if a := f.io.sprites.attrs; a[22].attr0 != 77 || a[50].attr0 != 88 {
		t.Fatalf("without lockstep: sprite 22 X %d (want 77), sprite 50 X %d (want 88)", a[22].attr0, a[50].attr0)
	}
	// With it (NextREG 0x09 bit 4) each sets the other's.
	f.io.nextRegWriteDirect(0x09, 0x10)
	f.io.WritePort(0x303B, 60)
	f.io.nextRegWriteDirect(0x35, 33)
	f.io.nextRegWriteDirect(0x34, 70)
	f.io.WritePort(0x0057, 44)
	if a := f.io.sprites.attrs; a[60].attr0 != 33 || a[70].attr0 != 44 {
		t.Fatalf("in lockstep: sprite 60 X %d (want 33), sprite 70 X %d (want 44)", a[60].attr0, a[70].attr0)
	}
}

func TestSpritePortsAreTheNextsOnly(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	io := NewSpectrumIO(mem, nil)
	io.WritePort(0x303B, 1)
	io.WritePort(0x005B, 0x55)
	io.WritePort(0x0057, 0x66)
	if io.sprites.patterns[1][0] != 0 || io.sprites.attrs[1].attr0 != 0 {
		t.Fatal("a 128K took writes to the Next's sprite ports")
	}
}
