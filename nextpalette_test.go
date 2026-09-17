package main

import (
	"testing"

	"github.com/ha1tch/zenzx/pkg/zxpalette"
)

// T-31 (wave ti0.r5) tests. Pattern A (register/memory-only, no CPU) below
// exercises nextpalette.go's own protocol mechanics directly -- the
// two-write NR 0x44 sequence, auto-increment, per-target storage, and the
// display-select bits' independence from the read/write-target bits.
// Pattern B (full ZenZX + Z80N opcodes) at the bottom is the end-to-end
// regression from this wave's own "done when" bar: a Next-mode program can
// write a known palette entry via real NEXTREG/OUT instructions and have
// the rendered pixel for that index match the expected 9-bit RGB value,
// for both Layer 2 and sprites -- matching TestT29NextModeLayer2EndToEnd
// and TestT30NextModeSpriteEndToEnd's own established style. layer2_test.go
// and sprite_test.go's own end-to-end tests additionally exercise a real
// palette write as part of proving their own layers' compositing, but this
// file is where the palette protocol itself -- not a consuming layer -- is
// the thing under test.

func TestPaletteIndexRegisterSelectsWriteIndex(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	io.SetNextRegister(0x40, 0x42)
	if got := io.nextPalette.writeIndex; got != 0x42 {
		t.Fatalf("writeIndex after NR 0x40=0x42 = 0x%02X, want 0x42", got)
	}
}

func TestPaletteControlRegisterSelectsWriteTargetAndDisplayBits(t *testing.T) {
	// bits 6-4 = 101 (Layer2_2nd), bit 3 = 1 (display 2nd Sprites),
	// bit 2 = 1 (display 2nd Layer2), bit 1 = 0 (display 1st ULA),
	// bit 0 = 1 (ULANext enable, stored only), bit 7 = 0 (auto-increment
	// stays enabled).
	io := newTestNextPaletteIO(t)
	io.SetNextRegister(0x43, 0b01011101)

	p := io.nextPalette
	if p.writeTarget != nextPaletteLayer2_2nd {
		t.Fatalf("writeTarget = %d, want nextPaletteLayer2_2nd (%d)", p.writeTarget, nextPaletteLayer2_2nd)
	}
	if !p.displaySecondSprites {
		t.Fatalf("displaySecondSprites = false, want true")
	}
	if !p.displaySecondLayer2 {
		t.Fatalf("displaySecondLayer2 = false, want true")
	}
	if p.displaySecondULA {
		t.Fatalf("displaySecondULA = true, want false")
	}
	if !p.ulaNextEnabled {
		t.Fatalf("ulaNextEnabled = false, want true")
	}
	if p.autoIncrementDisabled {
		t.Fatalf("autoIncrementDisabled = true, want false")
	}
}

func TestPaletteValue8WriteAndAutoIncrement(t *testing.T) {
	io := newTestNextPaletteIO(t)
	io.SetNextRegister(0x43, 0x20) // target Sprites-1st, auto-increment enabled
	io.SetNextRegister(0x40, 10)
	io.SetNextRegister(0x41, 0x4D) // RRRGGGBB = 01001101

	want := zxpalette.DecodeRGB9From8Bit(0x4D)
	if got := io.nextPalette.palettes[nextPaletteSprites1st][10]; got != want {
		t.Fatalf("palettes[Sprites1st][10] = %+v, want %+v", got, want)
	}
	if got := io.nextPalette.writeIndex; got != 11 {
		t.Fatalf("writeIndex after one NR 0x41 write = %d, want 11 (auto-increment)", got)
	}
}

func TestPaletteValue8AutoIncrementDisabledByControlBit7(t *testing.T) {
	io := newTestNextPaletteIO(t)
	io.SetNextRegister(0x43, 0xA0) // bit 7 set (disable auto-increment) + target Sprites-1st
	io.SetNextRegister(0x40, 10)
	io.SetNextRegister(0x41, 0x4D)

	if got := io.nextPalette.writeIndex; got != 10 {
		t.Fatalf("writeIndex after NR 0x41 write with auto-increment disabled = %d, want 10 (unchanged)", got)
	}
}

func TestPaletteValue8ReadReturnsCurrentEntryNotLastWrite(t *testing.T) {
	io := newTestNextPaletteIO(t)
	io.SetNextRegister(0x43, 0x20) // target Sprites-1st
	io.SetNextRegister(0x40, 5)
	io.SetNextRegister(0x41, 0x4D) // writes index 5, auto-increments to 6

	// The write above already advanced the index to 6 (an untouched,
	// still-black entry) -- re-select index 5 explicitly before reading,
	// since NR 0x41's read path (per this file's header and
	// nextpalette.go's own doc comment) reads the CURRENT index, not
	// "whatever was last written to".
	io.SetNextRegister(0x40, 5)
	got := io.GetNextRegister(0x41)
	want := zxpalette.EncodeRGB9To8Bit(zxpalette.DecodeRGB9From8Bit(0x4D))
	if got != want {
		t.Fatalf("NR 0x41 read after selecting index 5 = 0x%02X, want 0x%02X (re-encoded stored entry)", got, want)
	}

	// Reading must not itself advance the index.
	if idx := io.nextPalette.writeIndex; idx != 5 {
		t.Fatalf("writeIndex after an NR 0x41 read = %d, want 5 (unchanged by reads)", idx)
	}
}

func TestPaletteValue9TwoWriteSequenceAndAutoIncrement(t *testing.T) {
	io := newTestNextPaletteIO(t)
	io.SetNextRegister(0x43, 0x20) // target Sprites-1st
	io.SetNextRegister(0x40, 20)

	// First byte of the pair: RRRGGGBB = 01001101 (same top bits as the
	// 8-bit-path tests above), buffered only -- must not yet write
	// anything to the palette.
	io.SetNextRegister(0x44, 0x4D)
	if got := io.nextPalette.palettes[nextPaletteSprites1st][20]; got != (zxpalette.RGB9{}) {
		t.Fatalf("palettes[Sprites1st][20] after only the first NR 0x44 byte = %+v, want the zero value (write not yet completed)", got)
	}
	if !io.nextPalette.awaitingSecondByte {
		t.Fatalf("awaitingSecondByte after one NR 0x44 write = false, want true")
	}

	// Second byte: bit 0 = 1 supplies the true third blue bit (replacing
	// the 8-bit path's OR-synthesised approximation), bit 7 = 1 exercises
	// the Layer-2-only priority flag this emulator accepts but does not
	// store (see nextpalette.go's file header).
	io.SetNextRegister(0x44, 0x81)

	want := zxpalette.DecodeRGB9From9Bit(0x4D, 0x81)
	if got := io.nextPalette.palettes[nextPaletteSprites1st][20]; got != want {
		t.Fatalf("palettes[Sprites1st][20] after both NR 0x44 bytes = %+v, want %+v", got, want)
	}
	if io.nextPalette.awaitingSecondByte {
		t.Fatalf("awaitingSecondByte after the completing NR 0x44 write = true, want false (sequence reset)")
	}
	if got := io.nextPalette.writeIndex; got != 21 {
		t.Fatalf("writeIndex after a completed NR 0x44 pair = %d, want 21 (auto-increment)", got)
	}
}

func TestPaletteValue9ThirdBlueBitNotSynthesised(t *testing.T) {
	// Confirms NR 0x44's second byte supplies the true third blue bit
	// directly rather than the 8-bit path's bit1-OR-bit0 synthesis --
	// the whole reason NR 0x44 exists as a separate, wider write path.
	// byte1 blue field = 00 (both bits clear), so the 8-bit path would
	// synthesise blue=0; byte2 bit0=1 must still produce blue=1.
	io := newTestNextPaletteIO(t)
	io.SetNextRegister(0x43, 0x20)
	io.SetNextRegister(0x40, 30)
	io.SetNextRegister(0x44, 0x00) // R=0 G=0 B(2-bit field)=00
	io.SetNextRegister(0x44, 0x01) // third blue bit = 1

	got := io.nextPalette.palettes[nextPaletteSprites1st][30]
	if got.B != 1 {
		t.Fatalf("blue channel after NR 0x44 pair (0x00,0x01) = %d, want 1 (true third bit, not OR-synthesised)", got.B)
	}
}

func TestPaletteIndexRegisterResetsValue9Sequence(t *testing.T) {
	// A pending first byte of an NR 0x44 pair, followed by a write to
	// NR 0x40, must discard the pending byte -- the next NR 0x44 write is
	// treated as a fresh first byte, not the second of the old pair.
	io := newTestNextPaletteIO(t)
	io.SetNextRegister(0x43, 0x20)
	io.SetNextRegister(0x40, 40)
	io.SetNextRegister(0x44, 0xFF) // first byte of a pair, buffered

	io.SetNextRegister(0x40, 41) // re-select index -- must reset the sequence

	if io.nextPalette.awaitingSecondByte {
		t.Fatalf("awaitingSecondByte after NR 0x40 write = true, want false (sequence reset)")
	}

	// This write is now treated as a fresh first byte, not a completing
	// second byte -- so it must not write anything to index 41 yet.
	io.SetNextRegister(0x44, 0x00)
	if got := io.nextPalette.palettes[nextPaletteSprites1st][41]; got != (zxpalette.RGB9{}) {
		t.Fatalf("palettes[Sprites1st][41] after one NR 0x44 write post-reset = %+v, want the zero value (still buffering a first byte)", got)
	}
}

func TestPaletteControlRegisterResetsValue9Sequence(t *testing.T) {
	// Mirrors the NR 0x40 case above for NR 0x43 -- this emulator's own
	// documented, inferred-not-confirmed defensive choice (nextpalette.go's
	// file header): switching the write target mid-sequence must not leave
	// a stale pending first byte from a different palette.
	io := newTestNextPaletteIO(t)
	io.SetNextRegister(0x43, 0x20) // target Sprites-1st
	io.SetNextRegister(0x40, 50)
	io.SetNextRegister(0x44, 0xFF) // first byte of a pair, buffered

	io.SetNextRegister(0x43, 0x20) // re-write the SAME control byte -- must still reset

	if io.nextPalette.awaitingSecondByte {
		t.Fatalf("awaitingSecondByte after an NR 0x43 write = true, want false (sequence reset)")
	}
}

func TestPaletteDisplaySelectBitsAreIndependentOfWriteTarget(t *testing.T) {
	// The core claim of NR 0x43's split bit ranges: bits 6-4 (read/write
	// target) and bits 3-1 (per-layer display select) are orthogonal.
	// Write known, distinct colours into Layer2's two palettes at the
	// same index, then confirm nextColourFor picks whichever one the
	// display-select bit currently names, regardless of which palette is
	// the current read/write target.
	io := newTestNextPaletteIO(t)

	io.SetNextRegister(0x43, 0x10) // target Layer2-1st (bits 6-4 = 001), display 1st (bit 2 clear)
	io.SetNextRegister(0x40, 7)
	io.SetNextRegister(0x41, 0x20) // a colour for Layer2-1st

	io.SetNextRegister(0x43, 0x50) // target Layer2-2nd (bits 6-4 = 101), display 1st STILL (bit 2 clear)
	io.SetNextRegister(0x40, 7)
	io.SetNextRegister(0x41, 0x40) // a different colour for Layer2-2nd

	// Display bit still selects 1st -- nextColourFor must return Layer2-1st's
	// colour even though Layer2-2nd is the current write target.
	got := io.nextColourFor(nextLayerLayer2, 7)
	want := zxpalette.DecodeRGB9From8Bit(0x20).Colour()
	if got != want {
		t.Fatalf("nextColourFor(Layer2, 7) with write-target=2nd but display-select=1st = %+v, want %+v (1st palette's colour)", got, want)
	}

	// Now flip the display-select bit (bit 2) to show the 2nd palette
	// instead, write target left unchanged -- the resolved colour must
	// flip to the 2nd palette's entry without any further palette write.
	io.SetNextRegister(0x43, 0x54) // bit 2 set: display 2nd Layer2; write target still Layer2-2nd
	got2 := io.nextColourFor(nextLayerLayer2, 7)
	want2 := zxpalette.DecodeRGB9From8Bit(0x40).Colour()
	if got2 != want2 {
		t.Fatalf("nextColourFor(Layer2, 7) after flipping display-select to 2nd = %+v, want %+v (2nd palette's colour)", got2, want2)
	}
}

func TestPaletteEntriesArePerTargetNotShared(t *testing.T) {
	// Writing the same index in two different targets (Sprites-1st vs
	// Layer2-1st) must not alias -- confirms the 8x256 storage is
	// genuinely per-palette, not a single shared 256-entry table NR 0x43
	// merely offsets into.
	io := newTestNextPaletteIO(t)

	io.SetNextRegister(0x43, 0x10) // target Layer2-1st
	io.SetNextRegister(0x40, 99)
	io.SetNextRegister(0x41, 0x20)

	io.SetNextRegister(0x43, 0x20) // target Sprites-1st
	io.SetNextRegister(0x40, 99)
	io.SetNextRegister(0x41, 0x40)

	layer2Colour := io.nextPalette.palettes[nextPaletteLayer2_1st][99]
	spritesColour := io.nextPalette.palettes[nextPaletteSprites1st][99]
	if layer2Colour == spritesColour {
		t.Fatalf("palettes[Layer2_1st][99] and palettes[Sprites1st][99] are equal (%+v) after different writes, want distinct per-palette storage", layer2Colour)
	}
	if want := zxpalette.DecodeRGB9From8Bit(0x20); layer2Colour != want {
		t.Fatalf("palettes[Layer2_1st][99] = %+v, want %+v", layer2Colour, want)
	}
	if want := zxpalette.DecodeRGB9From8Bit(0x40); spritesColour != want {
		t.Fatalf("palettes[Sprites1st][99] = %+v, want %+v", spritesColour, want)
	}
}

// newTestNextPaletteIO is a small helper shared by this file's Pattern A
// tests: a Next-mode SpectrumIO with no further per-test setup needed,
// matching the newTestMemoryAndScreen()+EnableNext()+NewSpectrumIO
// sequence every other T-29/T-30/T-31 register-level test already repeats
// inline -- factored out here since this file's tests share it more
// densely than layer2_test.go/sprite_test.go's own mix of register-only
// and end-to-end tests did.
func newTestNextPaletteIO(t *testing.T) *SpectrumIO {
	t.Helper()
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	return NewSpectrumIO(mem, nil)
}

// TestT31NextModePaletteEndToEnd writes a Layer 2 pixel and a sprite pixel,
// each through a distinct palette entry set via real Z80N NEXTREG/OUT
// instructions (matching TestT29NextModeLayer2EndToEnd and
// TestT30NextModeSpriteEndToEnd's own style), and confirms DecodeDisplay's
// rendered pixels match the expected 9-bit RGB values -- this wave's own
// "done when" bar (NEXT_SUPPORT_DEVELOPMENT_PLAN.md's ti0.r5 entry) stated
// directly as a single end-to-end regression, rather than only inferred
// from the two consuming layers' own tests.
func TestT31NextModePaletteEndToEnd(t *testing.T) {
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

	// The exact priority order isn't the point here, but both Layer 2 and
	// Sprites must be ABOVE the ULA layer, or the opaque ULA screen (its
	// own default black paper) occludes both target pixels -- discovered
	// by this test itself initially failing with %010 S U L, which puts
	// ULA above Layer 2. NR 0x15 = %000 S L U (bits 4-2), bit 0 set to
	// enable sprites: bottom->top is U, L, S -- both consuming layers sit
	// above ULA, and Layer 2/Sprites don't overlap each other at the two
	// screen positions this test uses anyway.
	run(0x8000, []uint8{0xED, 0x91, 0x15, 0b00000001})

	// Layer 2: base bank = page 16, raw byte 0x11 at (0,0) -> resolved
	// index 0x11 (no palette offset, NR 0x70 left at its default 0).
	run(0x8010, []uint8{0xED, 0x91, 0x12, 0x10})
	zx.memory.nextPageWrite(16, 0, 0x11)

	// Write a known 9-bit colour to Layer2-1st palette index 0x11 via the
	// two-write NR 0x44 path (this wave's wider, more accurate write --
	// the "9-bit" the plan's own "done when" bar names explicitly).
	run(0x8020, []uint8{0xED, 0x91, 0x43, 0x10}) // target Layer2-1st
	run(0x8030, []uint8{0xED, 0x91, 0x40, 0x11}) // index 0x11
	run(0x8040, []uint8{0xED, 0x91, 0x44, 0b11100100})
	run(0x8050, []uint8{0xED, 0x91, 0x44, 0b00000001}) // third blue bit = 1

	// Sprite 0: visible, pattern 0, positioned at (100, 100), pattern
	// pixel (0,0) = raw 0x22 -> resolved index 0x22.
	run(0x8060, []uint8{0xED, 0x91, 0x34, 0x00})
	run(0x8070, []uint8{0xED, 0x91, 0x35, 100})
	run(0x8080, []uint8{0xED, 0x91, 0x36, 100})
	run(0x8090, []uint8{0xED, 0x91, 0x38, 0x80})
	zx.io.sprites.WriteSpritePatternByte(0, 0, 0, 0x22)

	// Write a different known 9-bit colour to Sprites-1st palette index
	// 0x22.
	run(0x80a0, []uint8{0xED, 0x91, 0x43, 0x20}) // target Sprites-1st
	run(0x80b0, []uint8{0xED, 0x91, 0x40, 0x22}) // index 0x22
	run(0x80c0, []uint8{0xED, 0x91, 0x44, 0b00011000})
	run(0x80d0, []uint8{0xED, 0x91, 0x44, 0b00000000}) // third blue bit = 0

	img := zx.DecodeDisplay()

	wantLayer2 := zxpalette.DecodeRGB9From9Bit(0b11100100, 0b00000001).Colour()
	gotLayer2 := img.RGBAAt(0, 0)
	if gotLayer2.R != wantLayer2.R || gotLayer2.G != wantLayer2.G || gotLayer2.B != wantLayer2.B {
		t.Fatalf("Layer 2 pixel (0,0) = %+v, want %+v (9-bit palette entry at index 0x11)", gotLayer2, wantLayer2)
	}

	wantSprite := zxpalette.DecodeRGB9From9Bit(0b00011000, 0b00000000).Colour()
	gotSprite := img.RGBAAt(100, 100)
	if gotSprite.R != wantSprite.R || gotSprite.G != wantSprite.G || gotSprite.B != wantSprite.B {
		t.Fatalf("Sprite pixel (100,100) = %+v, want %+v (9-bit palette entry at index 0x22)", gotSprite, wantSprite)
	}
}
