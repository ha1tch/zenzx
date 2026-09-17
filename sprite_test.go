package main

import (
	"image/color"
	"testing"

	"github.com/ha1tch/zenzx/pkg/zxpalette"
)

// T-30 (wave ti0.r4) tests. Pattern A (register/memory-only, no CPU) below
// covers the register-level and pixel-decode plumbing; Pattern B (full
// ZenZX + Z80N opcodes) at the bottom is the end-to-end regression proving
// a Next-mode program can position and display a sprite via real
// NEXTREG/OUT instructions -- matching TestT29NextModeLayer2EndToEnd's own
// style. Pattern memory itself has no NextREG-mirrored write path on real
// hardware (see sprite.go's own file header) so the end-to-end test writes
// pattern data via WriteSpritePatternByte directly, honestly representing
// that this emulator's sprite pattern memory access is a Go-level method,
// not a CPU-visible port, while attributes go through real NEXTREG/OUT.

func TestSpriteDisabledOnClassicMachine(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	if io.spriteEnabled() {
		t.Fatalf("spriteEnabled() = true on a classic (non-Next) machine, want false")
	}
	if got := io.DecodeSprites(); got != nil {
		t.Fatalf("DecodeSprites() = %v on a classic machine, want nil", got)
	}
}

func TestSpriteDisabledWhenNR15Bit0Clear(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	// NR 0x15 defaults to 0x08 (bit 0 clear) at construction -- sprites
	// start disabled even in Next mode until a program explicitly sets
	// bit 0, matching the wiki's own "sprite visibility enable" framing.
	if io.spriteEnabled() {
		t.Fatalf("spriteEnabled() = true with NR 0x15 bit 0 clear (0x%02X), want false", io.GetNextRegister(0x15))
	}
}

func TestSpriteEnabledOnceNR15Bit0Set(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x15, io.GetNextRegister(0x15)|0x01)
	if !io.spriteEnabled() {
		t.Fatalf("spriteEnabled() = false after setting NR 0x15 bit 0, want true")
	}
	img := io.DecodeSprites()
	if img == nil {
		t.Fatalf("DecodeSprites() = nil once sprites are enabled, want a non-nil image")
	}
	if img.Bounds().Dx() != layer2Width || img.Bounds().Dy() != layer2Height {
		t.Fatalf("DecodeSprites() size = %v, want %dx%d", img.Bounds(), layer2Width, layer2Height)
	}
}

func TestSpriteSelectAndAttributeWritesRouteToCorrectSprite(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	// Select sprite 5, write X=0x30 (attr0).
	io.SetNextRegister(0x34, 5)
	io.SetNextRegister(0x35, 0x30)
	// Select a different sprite (10), write a different X.
	io.SetNextRegister(0x34, 10)
	io.SetNextRegister(0x35, 0x40)

	resolved := io.sprites.effectiveAttrs()
	if got := resolved[5].x; got != 0x30 {
		t.Fatalf("sprite 5's X after re-selecting sprite 10 = 0x%02X, want 0x30 (must not be clobbered)", got)
	}
	if got := resolved[10].x; got != 0x40 {
		t.Fatalf("sprite 10's X = 0x%02X, want 0x40", got)
	}
}

func TestSpriteAttribute2SetsXMSBAndPaletteOffset(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x35, 0xFF) // X low byte = 0xFF
	io.SetNextRegister(0x37, 0x31) // palette offset 3 (bits 7-4), X MSB set (bit 0)

	a := io.sprites.effectiveAttrs()[0]
	if a.x != 0x1FF {
		t.Fatalf("sprite 0's X after attr2 sets the MSB = 0x%03X, want 0x1FF (0xFF | 0x100)", a.x)
	}
	if a.paletteOffset != 3 {
		t.Fatalf("sprite 0's paletteOffset = %d, want 3", a.paletteOffset)
	}

	// Clearing the MSB bit in a later attr2 write must clear the X MSB
	// without touching the low byte written earlier.
	io.SetNextRegister(0x37, 0x30) // same palette offset, X MSB now clear
	if got := io.sprites.effectiveAttrs()[0].x; got != 0xFF {
		t.Fatalf("sprite 0's X after clearing attr2's MSB bit = 0x%03X, want 0x0FF", got)
	}
}

func TestSpriteAttribute3SetsVisibleAndPattern(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 42)
	io.SetNextRegister(0x38, 0x80|0x15) // visible=1, pattern=0x15

	a := io.sprites.effectiveAttrs()[42]
	if !a.visible {
		t.Fatalf("sprite 42's visible = false, want true")
	}
	if a.pattern != 0x15 {
		t.Fatalf("sprite 42's pattern = 0x%02X, want 0x15", a.pattern)
	}

	io.SetNextRegister(0x38, 0x00|0x15) // visible=0, same pattern
	if io.sprites.effectiveAttrs()[42].visible {
		t.Fatalf("sprite 42's visible = true after clearing bit 7, want false")
	}
}

func TestWriteSpritePatternByteBumpsGeneration(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	if io.sprites.patternGeneration[3] != 0 {
		t.Fatalf("pattern slot 3's generation at construction = %d, want 0", io.sprites.patternGeneration[3])
	}
	io.sprites.WriteSpritePatternByte(3, 0, 0, 0xF0)
	if io.sprites.patternGeneration[3] != 1 {
		t.Fatalf("pattern slot 3's generation after one write = %d, want 1", io.sprites.patternGeneration[3])
	}
	// A write to a DIFFERENT slot must not bump slot 3's generation.
	io.sprites.WriteSpritePatternByte(4, 0, 0, 0xF0)
	if io.sprites.patternGeneration[3] != 1 {
		t.Fatalf("pattern slot 3's generation after writing slot 4 = %d, want unchanged at 1", io.sprites.patternGeneration[3])
	}
}

func TestWriteSpritePatternByteOutOfRangeIsNoOp(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	// Out-of-range slot, x, and y must all be silently ignored rather than
	// panicking (real hardware has no documented error path for an
	// out-of-range pattern write either).
	io.sprites.WriteSpritePatternByte(spritePatternSlots, 0, 0, 0xFF)
	io.sprites.WriteSpritePatternByte(0, -1, 0, 0xFF)
	io.sprites.WriteSpritePatternByte(0, 0, spriteHeight, 0xFF)
	if io.sprites.patternGeneration[0] != 0 {
		t.Fatalf("generation bumped by an out-of-range write, want unchanged at 0")
	}
}

func TestSpritePixelResolvesThroughSharedSeamAndHonoursPaletteOffset(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x37, 0x30) // palette offset 3
	io.SetNextRegister(0x38, 0x80) // visible, pattern 0

	io.sprites.WriteSpritePatternByte(0, 2, 1, 0x00) // raw 0 + (offset 3 << 4) -> idx 0x30

	// Write a known, distinguishable colour into the Sprites palette at
	// index 0x30 (NR 0x40 select index, NR 0x43 target Sprites-1st, NR
	// 0x41 8-bit value RRRGGGBB = 010 011 01 -> R=2 G=3 B synthesised from
	// the 2-bit field's OR'd bits) so this test proves spritePixel
	// resolves through the real palette (T-31), not just index
	// arithmetic.
	io.SetNextRegister(0x43, 0x20) // select Sprites-1st (bits 6-4 = 010) as R/W target
	io.SetNextRegister(0x40, 0x30) // palette index 0x30 (resolveNextPixelIndex's actual output)
	io.SetNextRegister(0x41, 0x4D) // RRRGGGBB = 01001101

	want := zxpalette.DecodeRGB9From8Bit(0x4D).Colour()

	c, opaque := io.spritePixel(io.sprites.effectiveAttrs()[0], 2, 1)
	if !opaque {
		t.Fatalf("spritePixel(0,2,1) reported transparent, want opaque")
	}
	if c != want {
		t.Fatalf("spritePixel(0,2,1) = %+v, want %+v (palette offset applied via resolveNextPixelIndex, then resolved through Sprites palette index 3)", c, want)
	}
}

func TestSpritePixelTransparentIndexIsNotDrawn(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x38, 0x80) // visible, pattern 0, no palette offset

	// raw 0xE3 + offset 0 -> idx 0xE3 == spriteTransparentIndex (NR 0x4B's
	// documented default).
	io.sprites.WriteSpritePatternByte(0, 0, 0, 0xE3)
	_, opaque := io.spritePixel(io.sprites.effectiveAttrs()[0], 0, 0)
	if opaque {
		t.Fatalf("spritePixel at the transparent index reported opaque, want transparent")
	}
}

func TestDecodeSpritesDrawsOnlyVisibleSprites(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x15, io.GetNextRegister(0x15)|0x01) // enable sprites

	// Give the Sprites-1st palette a known, bright colour at index 0xF0
	// (T-31) -- the power-on default is all-black (nextpalette.go's own
	// documented gap), so a real palette write is needed for this test's
	// colour assertion to mean anything.
	io.SetNextRegister(0x43, 0x20) // R/W target Sprites-1st (bits 6-4 = 010)
	io.SetNextRegister(0x40, 0xF0)
	io.SetNextRegister(0x41, 0xFF) // RRRGGGBB = 11111111 -> bright white

	// Sprite 0: visible, pattern 0, positioned at (10, 20), bright white.
	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x35, 10)
	io.SetNextRegister(0x36, 20)
	io.SetNextRegister(0x38, 0x80) // visible, pattern 0
	io.sprites.WriteSpritePatternByte(0, 0, 0, 0xF0)

	// Sprite 1: NOT visible, pattern 1, positioned at (50, 50) -- must not
	// appear in the decoded image at all.
	io.SetNextRegister(0x34, 1)
	io.SetNextRegister(0x35, 50)
	io.SetNextRegister(0x36, 50)
	io.SetNextRegister(0x38, 0x01) // visible=0, pattern 1
	io.sprites.WriteSpritePatternByte(1, 0, 0, 0xF0)

	img := io.DecodeSprites()
	got := img.RGBAAt(10, 20)
	wantN := zxpalette.DecodeRGB9From8Bit(0xFF).Colour()
	want := color.RGBA{R: wantN.R, G: wantN.G, B: wantN.B, A: wantN.A} // opaque, so NRGBA==RGBA channel-for-channel
	if got != want {
		t.Fatalf("visible sprite 0's pixel (10,20) = %v, want bright white %v", got, want)
	}
	if hidden := img.RGBAAt(50, 50); hidden.A != 0 {
		t.Fatalf("invisible sprite 1 drew a pixel at (50,50) = %v, want fully transparent", hidden)
	}
}

func TestDecodeSpritesHigherIndexWinsOverlap(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x15, io.GetNextRegister(0x15)|0x01)

	// Give the Sprites-1st palette known, distinguishable colours at
	// indices 0x10 (sprite 0's resolved index) and 0x20 (sprite 1's) --
	// T-31's power-on default is all-black, so both indices need explicit
	// colours for this test's "which one wins" assertion to be meaningful.
	io.SetNextRegister(0x43, 0x20) // R/W target Sprites-1st
	io.SetNextRegister(0x40, 0x10)
	io.SetNextRegister(0x41, 0x20) // RRRGGGBB = 00100000 -> dim red-ish
	io.SetNextRegister(0x40, 0x20)
	io.SetNextRegister(0x41, 0x40) // RRRGGGBB = 01000000 -> a different colour

	// Sprites 0 and 1 both cover pixel (0,0), same pattern slot 0, but
	// different colours via different palette offsets -- sprite 1 (higher
	// index) must win per DecodeSprites' own documented draw order.
	io.sprites.WriteSpritePatternByte(0, 0, 0, 0x00)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x35, 0)
	io.SetNextRegister(0x36, 0)
	io.SetNextRegister(0x37, 0x10) // palette offset 1
	io.SetNextRegister(0x38, 0x80) // visible, pattern 0

	io.SetNextRegister(0x34, 1)
	io.SetNextRegister(0x35, 0)
	io.SetNextRegister(0x36, 0)
	io.SetNextRegister(0x37, 0x20) // palette offset 2
	io.SetNextRegister(0x38, 0x80) // visible, pattern 0

	img := io.DecodeSprites()
	got := img.RGBAAt(0, 0)
	wantN := zxpalette.DecodeRGB9From8Bit(0x40).Colour() // sprite 1's resolved index 0x20
	want := color.RGBA{R: wantN.R, G: wantN.G, B: wantN.B, A: wantN.A}
	if got != want {
		t.Fatalf("overlapping pixel (0,0) = %v, want sprite 1's colour %v (higher index wins)", got, want)
	}
}

// TestNextLayerOrderAgreesWithLayer2ULAOnTop cross-checks nextLayerOrder's
// full three-layer decode against layer2ULAOnTop's own already-shipped,
// already-tested two-layer decode for every one of the 6 real NR 0x15 bit
// patterns -- both functions decode the SAME three bits from the same
// register and must never be allowed to silently disagree about the
// relative ULA/Layer2 ordering, per nextLayerOrder's own doc comment
// promising this was checked before either was trusted.
func TestNextLayerOrderAgreesWithLayer2ULAOnTop(t *testing.T) {
	for bits := uint8(0); bits <= 0b101; bits++ {
		nr15 := bits << 2
		order := nextLayerOrder(nr15)
		uIdx, lIdx := -1, -1
		for i, l := range order {
			switch l {
			case nextLayerULA:
				uIdx = i
			case nextLayerLayer2:
				lIdx = i
			}
		}
		ulaAboveLayer2 := uIdx > lIdx // higher index = drawn later = on top
		mem, _ := newTestMemoryAndScreen()
		mem.EnableNext()
		io := NewSpectrumIO(mem, nil)
		io.SetNextRegister(0x15, nr15)
		if got := io.layer2ULAOnTop(); got != ulaAboveLayer2 {
			t.Fatalf("bits=%03b: nextLayerOrder says ULA-above-Layer2=%v but layer2ULAOnTop() = %v -- must agree", bits, ulaAboveLayer2, got)
		}
	}
}

func TestLayersAboveULAOmitsLayersBelowULA(t *testing.T) {
	cases := []struct {
		name  string
		nr15  uint8
		above []nextLayer
	}{
		{"000 S L U -> both L and S above U", 0b00000000, []nextLayer{nextLayerLayer2, nextLayerSprites}},
		{"100 U S L -> nothing above U", 0b00010000, nil},
		{"010 S U L -> only S above U", 0b00001000, []nextLayer{nextLayerSprites}},
		{"011 L U S -> only L above U", 0b00001100, []nextLayer{nextLayerLayer2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := layersAboveULA(c.nr15)
			if len(got) != len(c.above) {
				t.Fatalf("layersAboveULA(0x%02X) = %v, want %v", c.nr15, got, c.above)
			}
			for i := range got {
				if got[i] != c.above[i] {
					t.Fatalf("layersAboveULA(0x%02X) = %v, want %v", c.nr15, got, c.above)
				}
			}
		})
	}
}

// TestT30NextModeSpriteEndToEnd writes a sprite configuration via real
// Z80N NEXTREG/OUT instructions (matching TestT29NextModeLayer2EndToEnd's
// own style) plus a pattern write via WriteSpritePatternByte (no
// NextREG-mirrored pattern-memory path exists -- see sprite.go's file
// header), then confirms DecodeDisplay produces a correctly composited
// picture with the sprite visible over both ULA and Layer 2, and that
// disabling sprites again (NR 0x15 bit 0 cleared) removes it.
func TestT30NextModeSpriteEndToEnd(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	zx.cpu.Reset()
	zx.memory.EnableNext()
	zx.EnableZ80N()

	run := func(addr uint16, prog []uint8) {
		for i, b := range prog {
			zx.memory.Write(addr+uint16(i), b)
		}
		zx.cpu.PC = addr
		for zx.cpu.PC != addr+uint16(len(prog)) {
			zx.cpu.Step()
		}
	}

	// Enable sprites (NR 0x15 bit 0) while keeping the existing priority
	// field at its construction default (0x08 = S U L -- sprites on top).
	run(0x8000, []uint8{0xED, 0x91, 0x15, 0x09})
	// Select sprite 0, position (5,5), pattern 0, visible.
	run(0x8010, []uint8{0xED, 0x91, 0x34, 0x00})
	run(0x8020, []uint8{0xED, 0x91, 0x35, 0x05})
	run(0x8030, []uint8{0xED, 0x91, 0x36, 0x05})
	run(0x8040, []uint8{0xED, 0x91, 0x38, 0x80})

	zx.io.sprites.WriteSpritePatternByte(0, 0, 0, 0xF0) // bright white at sprite-local (0,0)

	// Give the Sprites-1st palette a known bright colour at index 0xF0
	// (T-31) -- power-on default is all-black.
	run(0x8060, []uint8{0xED, 0x91, 0x43, 0x20}) // R/W target Sprites-1st
	run(0x8070, []uint8{0xED, 0x91, 0x40, 0xF0})
	run(0x8080, []uint8{0xED, 0x91, 0x41, 0xFF}) // RRRGGGBB = 11111111 -> bright white

	img := zx.DecodeDisplay()
	got := img.RGBAAt(5, 5)
	wantN := zxpalette.DecodeRGB9From8Bit(0xFF).Colour()
	want := color.RGBA{R: wantN.R, G: wantN.G, B: wantN.B, A: wantN.A}
	if got != want {
		t.Fatalf("pixel (5,5) with sprite visible = %v, want bright white %v", got, want)
	}

	// Disable sprites again (clear bit 0) and confirm the pixel reverts.
	run(0x8090, []uint8{0xED, 0x91, 0x15, 0x08})
	img2 := zx.DecodeDisplay()
	got2 := img2.RGBAAt(5, 5)
	if got2 == want {
		t.Fatalf("pixel (5,5) after disabling sprites still shows the sprite's colour %v", want)
	}
}

// TestT30SpriteAboveLayer2EndToEnd confirms sprites and Layer 2 composite
// in the correct relative order when both are active simultaneously --
// the scenario layer2ULAOnTop's own boolean collapse could never express
// (see that function's doc comment) and the reason nextLayerOrder/
// layersAboveULA exist.
func TestT30SpriteAboveLayer2EndToEnd(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	zx.cpu.Reset()
	zx.memory.EnableNext()
	zx.EnableZ80N()

	run := func(addr uint16, prog []uint8) {
		for i, b := range prog {
			zx.memory.Write(addr+uint16(i), b)
		}
		zx.cpu.PC = addr
		for zx.cpu.PC != addr+uint16(len(prog)) {
			zx.cpu.Step()
		}
	}

	// NR 0x15 = %001 L S U (bits 4-2) with bit 0 set to enable sprites:
	// bottom->top is U, S, L -- Layer 2 on top of sprites on top of ULA.
	run(0x8000, []uint8{0xED, 0x91, 0x15, 0b00000101})
	// Layer 2 base bank = page 16, resolved index 0x20 (raw 0x20, offset
	// 0) at (0,0).
	run(0x8010, []uint8{0xED, 0x91, 0x12, 0x10})
	zx.memory.nextPageWrite(16, 0, 0x20)

	// Give the Layer2-1st palette a known red at index 0x20, and the
	// Sprites-1st palette a known bright white at index 0xF0 (T-31) --
	// power-on default is all-black for both.
	run(0x8060, []uint8{0xED, 0x91, 0x43, 0x10}) // R/W target Layer2-1st (bits 6-4 = 001, since nextPaletteTarget iota order is ULA1st=0,Layer2_1st=1,Sprites1st=2,...)
	run(0x8070, []uint8{0xED, 0x91, 0x40, 0x20})
	run(0x8080, []uint8{0xED, 0x91, 0x41, 0x20}) // RRRGGGBB = 00100000 -> red-ish

	// Sprite 0 also covers (0,0), bright white -- must lose to Layer 2
	// under this ordering (L is above S).
	run(0x8020, []uint8{0xED, 0x91, 0x34, 0x00})
	run(0x8030, []uint8{0xED, 0x91, 0x35, 0x00})
	run(0x8040, []uint8{0xED, 0x91, 0x36, 0x00})
	run(0x8050, []uint8{0xED, 0x91, 0x38, 0x80})
	zx.io.sprites.WriteSpritePatternByte(0, 0, 0, 0xF0)
	run(0x8090, []uint8{0xED, 0x91, 0x43, 0x20}) // R/W target Sprites-1st
	run(0x80a0, []uint8{0xED, 0x91, 0x40, 0xF0})
	run(0x80b0, []uint8{0xED, 0x91, 0x41, 0xFF}) // RRRGGGBB = 11111111 -> bright white

	img := zx.DecodeDisplay()
	got := img.RGBAAt(0, 0)
	wantN := zxpalette.DecodeRGB9From8Bit(0x20).Colour()
	want := color.RGBA{R: wantN.R, G: wantN.G, B: wantN.B, A: wantN.A}
	if got != want {
		t.Fatalf("pixel (0,0) with Layer2-over-Sprites-over-ULA priority = %v, want Layer 2's red %v", got, want)
	}
}

// TestDecodeDisplayNoOpOnClassicMachineWithSprites re-confirms T-29's own
// "no destructive change" guarantee still holds with sprite.go compiled
// in: a classic machine's decoded image is unaffected by the sprite
// system existing at all, since DecodeSprites returns nil exactly like
// DecodeLayer2 does.
// ============================================================================
// T-29/T-30 completion pass: Y-MSB, NR 0x4B, scale/rotate/mirror,
// composite and unified relative sprites.
// ============================================================================

func TestSpriteYMSBViaAttribute4(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x36, 0xFF)      // Y low byte = 0xFF
	io.SetNextRegister(0x38, 0x80|0x40) // visible, E bit set (enable attr4), pattern 0
	io.SetNextRegister(0x39, 0x01)      // anchor, Y MSB set

	r := io.sprites.effectiveAttrs()[0]
	if r.y != 0x1FF {
		t.Fatalf("sprite 0's Y with attr4 Y-MSB set = 0x%03X, want 0x1FF (0xFF | 0x100)", r.y)
	}

	io.SetNextRegister(0x39, 0x00) // clear Y MSB, same anchor
	if got := io.sprites.effectiveAttrs()[0].y; got != 0xFF {
		t.Fatalf("sprite 0's Y after clearing attr4's Y-MSB = 0x%03X, want 0x0FF", got)
	}
}

func TestSpriteAttribute4IgnoredWithoutEBit(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x36, 0xFF)
	io.SetNextRegister(0x38, 0x80) // visible, E bit CLEAR, pattern 0
	io.SetNextRegister(0x39, 0x01) // would set Y-MSB if consulted

	r := io.sprites.effectiveAttrs()[0]
	if r.y != 0xFF {
		t.Fatalf("sprite 0's Y with attr3's E bit clear = 0x%03X, want 0x0FF (attr4 must be ignored entirely)", r.y)
	}
	if r.scaleX != spriteScale1x || r.scaleY != spriteScale1x {
		t.Fatalf("sprite 0's scale with E bit clear = (%v,%v), want (1x,1x)", r.scaleX, r.scaleY)
	}
}

func TestSpriteTransparencyComparesRawByteBeforeOffset(t *testing.T) {
	// Regression for the bug this pass found: transparency must compare
	// the RAW pattern byte against NR 0x4B, before resolveNextPixelIndex
	// applies the palette offset -- not the offset-resolved index.
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x37, 0x50) // palette offset 5
	io.SetNextRegister(0x38, 0x80) // visible, pattern 0

	// NR 0x4B default is 0xE3. Raw byte 0xE3 must be treated as
	// transparent regardless of the non-zero palette offset -- the
	// OFFSET-RESOLVED index (0xE3 + 5<<4 = 0x133, truncated to uint8
	// 0x33) is NOT what gets compared.
	io.sprites.WriteSpritePatternByte(0, 0, 0, 0xE3)
	r := io.sprites.effectiveAttrs()[0]
	_, opaque := io.spritePixel(r, 0, 0)
	if opaque {
		t.Fatalf("raw pattern byte 0xE3 (NR 0x4B's default) with a non-zero palette offset reported opaque, want transparent (pre-offset comparison)")
	}

	// A raw byte that only becomes 0xE3 AFTER the offset is applied must
	// NOT be treated as transparent -- confirms the fix didn't just
	// invert which value is compared.
	io.sprites.WriteSpritePatternByte(0, 1, 0, 0x93) // 0x93 + 0x50 = 0xE3 post-offset
	_, opaque2 := io.spritePixel(r, 1, 0)
	if !opaque2 {
		t.Fatalf("raw pattern byte 0x93 (only reaches 0xE3 after the offset) reported transparent, want opaque")
	}
}

func TestSpriteTransparentIndexIsConfigurable(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	if got := io.spriteTransparentIndex(); got != spriteTransparentIndexDefault {
		t.Fatalf("spriteTransparentIndex() at construction = 0x%02X, want default 0x%02X", got, spriteTransparentIndexDefault)
	}
	io.SetNextRegister(0x4B, 0x00) // reconfigure transparency to raw index 0
	if got := io.spriteTransparentIndex(); got != 0x00 {
		t.Fatalf("spriteTransparentIndex() after writing NR 0x4B = 0x%02X, want 0x00", got)
	}

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x38, 0x80)
	io.sprites.WriteSpritePatternByte(0, 0, 0, 0x00)
	r := io.sprites.effectiveAttrs()[0]
	_, opaque := io.spritePixel(r, 0, 0)
	if opaque {
		t.Fatalf("raw pattern byte 0x00 with NR 0x4B reconfigured to 0x00 reported opaque, want transparent")
	}
}

func TestSpriteScaleFactorDecoding(t *testing.T) {
	cases := []struct {
		bits uint8
		want spriteScale
		mult int
	}{
		{0b00, spriteScale1x, 1},
		{0b01, spriteScale2x, 2},
		{0b10, spriteScale4x, 4},
		{0b11, spriteScale8x, 8},
	}
	for _, c := range cases {
		got := decodeSpriteScale(c.bits)
		if got != c.want {
			t.Fatalf("decodeSpriteScale(0b%02b) = %v, want %v", c.bits, got, c.want)
		}
		if got.factor() != c.mult {
			t.Fatalf("spriteScale(0b%02b).factor() = %d, want %d", c.bits, got.factor(), c.mult)
		}
	}
}

func TestSpriteAnchorScaleAffectsFootprint(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x38, 0x80|0x40)  // visible, E bit set, pattern 0
	io.SetNextRegister(0x39, 0b00001110) // X scale %01=2x (bits4-3), Y scale %11=8x (bits2-1)

	r := io.sprites.effectiveAttrs()[0]
	if r.scaleX != spriteScale2x {
		t.Fatalf("scaleX = %v, want 2x", r.scaleX)
	}
	if r.scaleY != spriteScale8x {
		t.Fatalf("scaleY = %v, want 8x", r.scaleY)
	}
	w, h := spriteFootprint(r)
	if w != 32 || h != 128 {
		t.Fatalf("spriteFootprint() = (%d,%d), want (32,128) for 2x/8x unrotated", w, h)
	}
}

func TestSpriteRotationSwapsFootprint(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x37, 0x02) // rotate bit set (attr2 bit 1)
	io.SetNextRegister(0x38, 0x80|0x40)
	io.SetNextRegister(0x39, 0b00001110) // X scale 2x, Y scale 8x

	r := io.sprites.effectiveAttrs()[0]
	w, h := spriteFootprint(r)
	if w != 128 || h != 32 {
		t.Fatalf("spriteFootprint() with rotate set = (%d,%d), want (128,32) -- rotation swaps the 2x/8x footprint", w, h)
	}
}

// TestSpriteLocalCoordRoundTripsUnrotatedUnmirrored confirms
// spriteLocalCoord's inverse-transform math against the plain identity
// case (no scale/rotate/mirror): every screen offset must map straight
// back to itself.
func TestSpriteLocalCoordRoundTripsUnrotatedUnmirrored(t *testing.T) {
	r := spriteResolved{scaleX: spriteScale1x, scaleY: spriteScale1x}
	for ly := 0; ly < spriteHeight; ly++ {
		for lx := 0; lx < spriteWidth; lx++ {
			gotLx, gotLy, ok := spriteLocalCoord(r, lx, ly)
			if !ok || gotLx != lx || gotLy != ly {
				t.Fatalf("spriteLocalCoord(identity, %d,%d) = (%d,%d,%v), want (%d,%d,true)", lx, ly, gotLx, gotLy, ok, lx, ly)
			}
		}
	}
}

// TestSpriteLocalCoordScaling confirms a 2x/3x-scaled sprite maps every
// on-screen point in a scaleX x scaleY block back to the same source
// pattern pixel.
func TestSpriteLocalCoordScaling(t *testing.T) {
	r := spriteResolved{scaleX: spriteScale2x, scaleY: spriteScale4x}
	w, h := spriteFootprint(r)
	if w != 32 || h != 64 {
		t.Fatalf("footprint = (%d,%d), want (32,64)", w, h)
	}
	// Source pattern pixel (3,5) should own screen block
	// x in [6,7], y in [20,23].
	for _, ox := range []int{6, 7} {
		for _, oy := range []int{20, 21, 22, 23} {
			lx, ly, ok := spriteLocalCoord(r, ox, oy)
			if !ok || lx != 3 || ly != 5 {
				t.Fatalf("spriteLocalCoord(scale2x/4x, %d,%d) = (%d,%d,%v), want (3,5,true)", ox, oy, lx, ly, ok)
			}
		}
	}
}

// TestSpriteLocalCoordMirrorAndRotate exercises spriteLocalCoord's
// inverse mirror and inverse rotate paths against known, hand-derived
// corner mappings on an unscaled 16x16 sprite -- the same numerical
// verification method used to catch and fix this pass's own rotation
// inverse bug (see spriteLocalCoord's own doc comment), applied here as
// a permanent regression rather than a one-off manual check.
func TestSpriteLocalCoordMirrorAndRotate(t *testing.T) {
	unscaled := spriteResolved{scaleX: spriteScale1x, scaleY: spriteScale1x}

	t.Run("xMirror", func(t *testing.T) {
		r := unscaled
		r.xMirror = true
		// Screen-space top-left (0,0) must read pattern column 15 (the
		// far right), same row.
		lx, ly, ok := spriteLocalCoord(r, 0, 0)
		if !ok || lx != 15 || ly != 0 {
			t.Fatalf("xMirror (0,0) = (%d,%d,%v), want (15,0,true)", lx, ly, ok)
		}
		// Screen-space top-right (15,0) must read pattern column 0.
		lx, ly, ok = spriteLocalCoord(r, 15, 0)
		if !ok || lx != 0 || ly != 0 {
			t.Fatalf("xMirror (15,0) = (%d,%d,%v), want (0,0,true)", lx, ly, ok)
		}
	})

	t.Run("yMirror", func(t *testing.T) {
		r := unscaled
		r.yMirror = true
		lx, ly, ok := spriteLocalCoord(r, 0, 0)
		if !ok || lx != 0 || ly != 15 {
			t.Fatalf("yMirror (0,0) = (%d,%d,%v), want (0,15,true)", lx, ly, ok)
		}
	})

	t.Run("rotate90CW_cornersRoundTrip", func(t *testing.T) {
		// Rather than assert one hand-derived corner (error-prone, per
		// this pass's own caught mistake), verify the FULL 16x16
		// pattern round-trips through forward-rotate-then-inverse for
		// every source coordinate: simulate the forward pipeline's
		// rotation (pattern (lx,ly) -> footprint (ox,oy)) using the
		// SAME formula spriteLocalCoord's own doc comment states
		// (rx,ry)=(H-1-py,px) with W=H=16 for an unscaled square
		// pattern, then confirm the inverse recovers it.
		r := unscaled
		r.rotate = true
		const w, h = spriteWidth, spriteHeight // pre-rotation, square so w==h
		for py := 0; py < h; py++ {
			for px := 0; px < w; px++ {
				ox, oy := h-1-py, px // forward rotation formula
				lx, ly, ok := spriteLocalCoord(r, ox, oy)
				if !ok || lx != px || ly != py {
					t.Fatalf("rotate90CW round-trip: pattern(%d,%d) -> screen(%d,%d) -> back = (%d,%d,%v), want (%d,%d,true)", px, py, ox, oy, lx, ly, ok, px, py)
				}
			}
		}
	})
}

func TestRotatePoint90CWMatchesPhysicalIntuition(t *testing.T) {
	// Right (1,0) rotated 90 clockwise on a Y-down screen must point
	// down (0,1); up (0,-1) rotated 90 clockwise must point right (1,0).
	// Verified numerically against these two independent physical
	// reference cases before use in effectiveAttrs' unified-mode offset
	// transform.
	if x, y := rotatePoint90CW(1, 0); x != 0 || y != 1 {
		t.Fatalf("rotatePoint90CW(1,0) = (%d,%d), want (0,1)", x, y)
	}
	if x, y := rotatePoint90CW(0, -1); x != 1 || y != 0 {
		t.Fatalf("rotatePoint90CW(0,-1) = (%d,%d), want (1,0)", x, y)
	}
}

// TestRelativeSpriteCompositeMode confirms a composite relative sprite:
// independent transform, position offset by (dx,dy) from the anchor with
// NO rotation/scale applied to the offset itself, and visibility ANDed
// with the anchor's.
func TestRelativeSpriteCompositeMode(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	// Sprite 0: anchor at (100,100), visible, composite type (attr4 bit5=0).
	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x35, 100)
	io.SetNextRegister(0x36, 100)
	io.SetNextRegister(0x38, 0x80|0x40|0x02) // visible, E set, pattern 2
	io.SetNextRegister(0x39, 0x00)           // anchor, composite type, 1x/1x

	// Sprite 1: relative to sprite 0, offset (+10,-5), own pattern 3,
	// own palette offset 7 (PR bit clear -- independent).
	io.SetNextRegister(0x34, 1)
	io.SetNextRegister(0x35, uint8(int8(10)))
	var negFive int8 = -5
	io.SetNextRegister(0x36, uint8(negFive))
	io.SetNextRegister(0x37, 0x70)           // palette offset 7, PR clear
	io.SetNextRegister(0x38, 0x80|0x40|0x03) // visible, E set, pattern 3
	io.SetNextRegister(0x39, 0b01000000)     // bits7-6=%01 relative, type composite (bit5=0)

	resolved := io.sprites.effectiveAttrs()
	rel := resolved[1]
	if rel.x != 110 || rel.y != 95 {
		t.Fatalf("composite relative position = (%d,%d), want (110,95) (anchor (100,100) + offset (10,-5))", rel.x, rel.y)
	}
	if rel.pattern != 3 {
		t.Fatalf("composite relative pattern = %d, want 3 (PO clear -- own pattern, not added to anchor's)", rel.pattern)
	}
	if rel.paletteOffset != 7 {
		t.Fatalf("composite relative paletteOffset = %d, want 7 (PR clear -- independent)", rel.paletteOffset)
	}
	if !rel.visible {
		t.Fatalf("composite relative visible = false, want true (both anchor and relative visible)")
	}
}

// TestRelativeSpriteInheritsPaletteAndPatternWhenEnabled confirms PR/PO
// bits add the anchor's own values rather than replacing the relative's.
func TestRelativeSpriteInheritsPaletteAndPatternWhenEnabled(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x37, 0x50)           // anchor palette offset 5
	io.SetNextRegister(0x38, 0x80|0x40|0x02) // visible, E set, pattern 2 (anchor)
	io.SetNextRegister(0x39, 0x00)           // anchor, composite

	io.SetNextRegister(0x34, 1)
	io.SetNextRegister(0x37, 0x21)           // relative's own palette offset 2, PR bit SET (bit0)
	io.SetNextRegister(0x38, 0x80|0x40|0x03) // visible, E set, own pattern 3
	io.SetNextRegister(0x39, 0b01000001)     // relative, composite, PO bit SET (bit0)

	rel := io.sprites.effectiveAttrs()[1]
	if rel.paletteOffset != 5+2 {
		t.Fatalf("relative paletteOffset with PR set = %d, want %d (anchor's 5 + own 2)", rel.paletteOffset, 5+2)
	}
	if rel.pattern != 2+3 {
		t.Fatalf("relative pattern with PO set = %d, want %d (anchor's 2 + own 3)", rel.pattern, 2+3)
	}
}

// TestRelativeSpriteVisibilityANDedWithAnchor confirms an invisible
// anchor hides all its relatives regardless of their own visible bit.
func TestRelativeSpriteVisibilityANDedWithAnchor(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x38, 0x00|0x40) // NOT visible, E set, pattern 0 (anchor)
	io.SetNextRegister(0x39, 0x00)

	io.SetNextRegister(0x34, 1)
	io.SetNextRegister(0x38, 0x80|0x40) // visible=1 on the relative itself
	io.SetNextRegister(0x39, 0b01000000)

	if io.sprites.effectiveAttrs()[1].visible {
		t.Fatalf("relative sprite visible = true with an invisible anchor, want false (visibility is ANDed)")
	}
}

// TestRelativeSpriteUnifiedModeInheritsTransform confirms a unified
// relative's (dx,dy) offset is rotated/mirrored/scaled by the ANCHOR's
// own transform before being added to the anchor's position, and that
// the relative's own transform fields are overridden to match the
// anchor's (not its own attr2 bits).
func TestRelativeSpriteUnifiedModeInheritsTransform(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	// Anchor at (100,100), X-mirrored, unified type (attr4 bit5=1), 1x/1x.
	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x35, 100)
	io.SetNextRegister(0x36, 100)
	io.SetNextRegister(0x37, 0x08)       // X mirror set (bit3)
	io.SetNextRegister(0x38, 0x80|0x40)  // visible, E set, pattern 0
	io.SetNextRegister(0x39, 0b00100000) // anchor, TYPE=unified (bit5), 1x/1x

	// Relative offset (+10, 0). Own attr2 mirror/rotate bits set
	// DIFFERENTLY from the anchor -- must be ignored/overridden in
	// unified mode.
	io.SetNextRegister(0x34, 1)
	io.SetNextRegister(0x35, uint8(int8(10)))
	io.SetNextRegister(0x36, 0)
	io.SetNextRegister(0x37, 0x00)       // relative's own mirror bits clear
	io.SetNextRegister(0x38, 0x80|0x40)  // visible, E set, pattern 0
	io.SetNextRegister(0x39, 0b01100000) // relative, bit5=1 (decoded, not consumed per sprite.go)

	rel := io.sprites.effectiveAttrs()[1]
	// Anchor is X-mirrored, so the +10 offset must be negated: final
	// position = anchor (100) + (-10) = 90.
	if rel.x != 90 {
		t.Fatalf("unified relative X = %d, want 90 (anchor's X-mirror negates the +10 offset)", rel.x)
	}
	if rel.y != 100 {
		t.Fatalf("unified relative Y = %d, want 100 (dy=0, unaffected by X mirror)", rel.y)
	}
	if !rel.xMirror {
		t.Fatalf("unified relative xMirror = false, want true (inherited from anchor, overriding its own attr2 bits)")
	}
}

func TestRelativeSpriteWithNoPrecedingAnchorFallsBackSafely(t *testing.T) {
	// Sprite 0 itself decoded as relative (degenerate -- no source
	// consulted disallows this, but real hardware has no anchor state
	// to inherit from at index 0). Must not panic; must fall back to a
	// clearly-defined (0,0) pseudo-anchor rather than reading
	// uninitialised state.
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x34, 0)
	io.SetNextRegister(0x35, 5)
	io.SetNextRegister(0x36, 5)
	io.SetNextRegister(0x38, 0x80|0x40)  // visible, E set
	io.SetNextRegister(0x39, 0b01000000) // relative, composite -- but sprite 0 has no real anchor

	resolved := io.sprites.effectiveAttrs()
	r := resolved[0]
	if r.x != 5 || r.y != 5 {
		t.Fatalf("relative sprite 0 with no preceding anchor = (%d,%d), want (5,5) (offset from the (0,0) fallback pseudo-anchor)", r.x, r.y)
	}
}

func TestDecodeDisplayNoOpOnClassicMachineWithSprites(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	rom := make([]byte, 16384)
	if err := zx.LoadROMBytes(rom); err != nil {
		t.Fatalf("LoadROMBytes: %v", err)
	}
	zx.cpu.Reset()
	if zx.memory.isNext {
		t.Fatalf("48K ROM unexpectedly entered Next mode")
	}
	direct := zx.videoRenderer.Decode(zx.memory, zx.screen)
	composited := zx.DecodeDisplay()
	if composited.Bounds() != direct.Bounds() {
		t.Fatalf("DecodeDisplay() bounds %v != direct Decode() bounds %v on a classic machine", composited.Bounds(), direct.Bounds())
	}
	for i := range direct.Pix {
		if composited.Pix[i] != direct.Pix[i] {
			t.Fatalf("DecodeDisplay() diverges from direct Decode() at byte %d on a classic machine: %d != %d", i, composited.Pix[i], direct.Pix[i])
		}
	}
}
