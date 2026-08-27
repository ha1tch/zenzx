package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// newTestMemoryAndScreen builds a minimal SpectrumMemory + SpectrumScreen
// pair for renderer tests, without needing a full ZenZX/ROM boot.
func newTestMemoryAndScreen() (*SpectrumMemory, *SpectrumScreen) {
	screen := NewSpectrumScreen()
	mem := NewSpectrumMemory(screen)
	mem.ramBankLow = 5
	mem.screenBank = 5
	return mem, screen
}

func TestHicolourRegistered(t *testing.T) {
	r, err := LookupVideoRenderer(NSGraphicsTimex001HiColour)
	if err != nil {
		t.Fatalf("mode-timex-001-hicolour should be registered: %v", err)
	}
	if r.Name() != NSGraphicsTimex001HiColour {
		t.Errorf("Name() = %q, want %q", r.Name(), NSGraphicsTimex001HiColour)
	}
	w, h := r.Dimensions()
	if w != ScreenWidth || h != ScreenHeight {
		t.Errorf("Dimensions() = (%d,%d), want (%d,%d)", w, h, ScreenWidth, ScreenHeight)
	}
	bl, br, bt, bb := r.BorderMargins()
	if bl != BorderLeft || br != BorderRight || bt != BorderTop || bb != BorderBottom {
		t.Errorf("BorderMargins() = (%d,%d,%d,%d), want standard (%d,%d,%d,%d)",
			bl, br, bt, bb, BorderLeft, BorderRight, BorderTop, BorderBottom)
	}
}

// TestHicolourAttributeAddressing checks the pixel-byte-to-attribute-byte
// correspondence against the T/S 2068 Technical Manual's own worked
// example (section 5.2.2): "the byte written to Location 4000H has its
// Attribute byte at Location 6000H, the byte at 47FFH ... has its
// Attribute byte at Location 67FFH, the byte at 57FFH ... has its
// Attribute byte at Location 77FFH."
func TestHicolourAttributeAddressing(t *testing.T) {
	cases := []struct {
		pixelAddr, wantAttrAddr uint16
	}{
		{0x4000, 0x6000},
		{0x47FF, 0x67FF},
		{0x57FF, 0x77FF},
	}
	for _, c := range cases {
		offset := int(c.pixelAddr - 0x4000)
		gotAttrAddr := uint16(hicolourAttrBase + offset)
		if gotAttrAddr != c.wantAttrAddr {
			t.Errorf("pixel byte %04X: attribute address = %04X, want %04X (Technical Manual 5.2.2)",
				c.pixelAddr, gotAttrAddr, c.wantAttrAddr)
		}
	}
}

func TestHicolourDecodeBasicPixel(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	renderer := hicolourVideoRenderer{}

	// Pixel (0,0): first byte of the bitmap, top bit set (pixel on).
	off := screen.calcByteOffset(0, 0)
	screen.bitmap[off] = 0x80 // 10000000: leftmost pixel on, rest off
	// Attribute for that 8x1 row: ink=red(2), paper=cyan(5), bright=0.
	mem.ram[5][hicolourAttrBase+off-0x4000] = (5 << 3) | 2

	img := renderer.Decode(mem, screen)

	gotOn := img.RGBAAt(0, 0)
	wantOn := zxPalette[2] // ink=red, not bright
	if gotOn != wantOn {
		t.Errorf("pixel (0,0) [on] = %+v, want ink colour %+v", gotOn, wantOn)
	}
	gotOff := img.RGBAAt(1, 0)
	wantOff := zxPalette[5] // paper=cyan
	if gotOff != wantOff {
		t.Errorf("pixel (1,0) [off] = %+v, want paper colour %+v", gotOff, wantOff)
	}
}

func TestHicolourDecodeBright(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	renderer := hicolourVideoRenderer{}

	off := screen.calcByteOffset(0, 0)
	screen.bitmap[off] = 0x80
	// ink=green(4), paper=black(0), bright=1
	mem.ram[5][hicolourAttrBase+off-0x4000] = (1 << 6) | (0 << 3) | 4

	img := renderer.Decode(mem, screen)
	got := img.RGBAAt(0, 0)
	want := zxPalette[4+8] // bright green
	if got != want {
		t.Errorf("pixel (0,0) with BRIGHT = %+v, want bright ink colour %+v", got, want)
	}
}

// TestHicolourFlashSwapsInkAndPaper confirms FLASH is honoured in
// hi-colour mode, matching the ZX-Uno manual's own attribute
// description for this mode ("paper/ink/bright/flash attribute per each
// 8x1 pixels block") and this project's own docs/timex-modes.md. An
// earlier version of this test (then named TestHicolourFlashIgnored)
// asserted the opposite -- that setting FLASH must never change the
// decoded colour -- based on a design comment that incorrectly
// described a scope decision as if it were a real-hardware fact.
func TestHicolourFlashSwapsInkAndPaper(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	renderer := hicolourVideoRenderer{}

	off := screen.calcByteOffset(0, 0)
	screen.bitmap[off] = 0x80 // pixel (0,0) is an "ink" bitmap bit
	// ink=blue(1), paper=yellow(6), FLASH bit (0x80) set.
	mem.ram[5][hicolourAttrBase+off-0x4000] = 0x80 | (6 << 3) | 1

	screen.flashEnabled = true

	screen.flashTickTock = false
	img := renderer.Decode(mem, screen)
	got := img.RGBAAt(0, 0)
	wantInk := zxPalette[1] // blue -- not yet swapped
	if got != wantInk {
		t.Errorf("flashTickTock=false: pixel (0,0) = %+v, want ink colour %+v (not yet swapped)", got, wantInk)
	}

	screen.flashTickTock = true
	img = renderer.Decode(mem, screen)
	got = img.RGBAAt(0, 0)
	wantPaper := zxPalette[6] // yellow -- swapped
	if got != wantPaper {
		t.Errorf("flashTickTock=true: pixel (0,0) = %+v, want paper colour %+v (ink/paper should be swapped)", got, wantPaper)
	}
}

// TestHicolourFlashDisabledNeverSwaps mirrors the standard renderer's
// equivalent guard (flash_regression_test.go): flashEnabled=false must
// fully disable FLASH, not just leave it un-ticked.
func TestHicolourFlashDisabledNeverSwaps(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	renderer := hicolourVideoRenderer{}

	off := screen.calcByteOffset(0, 0)
	screen.bitmap[off] = 0x80
	mem.ram[5][hicolourAttrBase+off-0x4000] = 0x80 | (6 << 3) | 1 // same as above

	screen.flashEnabled = false
	screen.flashTickTock = true // would swap if flashEnabled were honoured

	img := renderer.Decode(mem, screen)
	got := img.RGBAAt(0, 0)
	want := zxPalette[1] // ink (blue) -- must stay unswapped
	if got != want {
		t.Errorf("flashEnabled=false: pixel (0,0) = %+v, want ink colour %+v", got, want)
	}
}

// TestHicolourIndependentFromScreen0Attributes confirms screen 0's own
// 32x24 attribute block (5800H-5AFFH) has no effect in this mode -- only
// screen 1 (6000H+) is read.
func TestHicolourIndependentFromScreen0Attributes(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	renderer := hicolourVideoRenderer{}

	off := screen.calcByteOffset(0, 0)
	screen.bitmap[off] = 0x80
	// Deliberately wrong colours in the standard attribute block -- must
	// be ignored entirely.
	screen.attributes[0] = (7 << 3) | 7 // white on white
	// Correct colours in screen 1.
	mem.ram[5][hicolourAttrBase+off-0x4000] = (0 << 3) | 3 // ink=magenta, paper=black

	img := renderer.Decode(mem, screen)
	got := img.RGBAAt(0, 0)
	want := zxPalette[3]
	if got != want {
		t.Errorf("pixel (0,0) = %+v, want %+v (screen 0's own attribute block must be ignored)", got, want)
	}
}

// TestHicolourEndToEnd is the end-to-end regression T-11 calls for: a
// real ZenZX (not a bare memory+screen pair), engaging the mode through
// the same ZenZX.SelectVideoRenderer entry point -ns-graphics uses,
// poking a pattern only hi-colour mode can render correctly (eight
// different ink colours in eight consecutive 8x1 rows within a single
// 8x8 character cell -- standard mode can only show one attribute for
// the whole cell), and decoding through zx.DecodeDisplay(), the exact
// method both front ends call. Also writes a PNG so the result can be
// looked at, not just asserted on.
func TestHicolourEndToEnd(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.SelectVideoRenderer(NSGraphicsTimex001HiColour); err != nil {
		t.Fatalf("SelectVideoRenderer: %v", err)
	}

	inks := []uint8{1, 2, 3, 4, 5, 6, 0, 7} // blue,red,magenta,green,cyan,yellow,black,white
	for row := 0; row < 8; row++ {
		off := zx.screen.calcByteOffset(0, row)
		zx.screen.bitmap[off] = 0xFF
		zx.memory.Write(uint16(0x6000+off), inks[row]) // paper=black(0), ink=inks[row]
	}

	img := zx.DecodeDisplay()

	for row := 0; row < 8; row++ {
		got := img.RGBAAt(0, row)
		want := zxPalette[inks[row]]
		if got != want {
			t.Errorf("row %d: pixel (0,%d) = %+v, want %+v", row, row, got, want)
		}
	}

	path := filepath.Join(t.TempDir(), "hicolour_e2e.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding png: %v", err)
	}
}
