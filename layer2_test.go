package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/ha1tch/zenzx/pkg/zxpalette"
)

// T-29 (wave ti0.r3) tests. Pattern A (register/memory-only, no CPU) below
// covers the register-level and pixel-decode plumbing; Pattern B (full
// ZenZX + Z80N opcodes) at the bottom is the end-to-end regression proving
// a Next-mode program can write a Layer 2 pattern via real NEXTREG/OUT
// instructions and have DecodeDisplay produce the correct composited
// picture, matching TestT28NextModeMultiSlotProgram's own style.

func TestLayer2DisabledOnClassicMachine(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	if io.layer2Enabled() {
		t.Fatalf("layer2Enabled() = true on a classic (non-Next) machine, want false")
	}
	if got := io.DecodeLayer2(); got != nil {
		t.Fatalf("DecodeLayer2() = %v on a classic machine, want nil", got)
	}
}

func TestLayer2EnabledOnceNextModeEnabled(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	if !io.layer2Enabled() {
		t.Fatalf("layer2Enabled() = false after EnableNext(), want true")
	}
	img := io.DecodeLayer2()
	if img == nil {
		t.Fatalf("DecodeLayer2() = nil after EnableNext(), want a non-nil image")
	}
	if img.Bounds().Dx() != layer2Width || img.Bounds().Dy() != layer2Height {
		t.Fatalf("DecodeLayer2() size = %v, want %dx%d", img.Bounds(), layer2Width, layer2Height)
	}
}

func TestLayer2BaseBankReadsRegister0x12(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x12, 0x20)
	if got := io.layer2BaseBank(); got != 0x20 {
		t.Fatalf("layer2BaseBank() = 0x%02X, want 0x20", got)
	}
}

func TestLayer2PixelReadsFromConfiguredBaseBank(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	// Base bank 16 (nextExtraPageBase) is the first genuinely-extra page,
	// clear of the aliased-to-ram low range -- writing directly via
	// nextPageWrite and reading back through layer2Pixel proves the two
	// paths agree on addressing without going through nextSlots/CPU space
	// at all. NR 0x12 defaults to page 0 (see nextreg.go's own registered
	// default), so this must explicitly select page 16 as the base bank --
	// discovered by this test itself initially failing without it.
	io.SetNextRegister(0x12, 16)
	mem.nextPageWrite(16, 0, 0xF0) // pixel (0,0): raw byte 0xF0, offset 0 -> idx 0xF0
	got := mem.layer2Pixel(io, 0, 0)
	if got != 0xF0 {
		t.Fatalf("layer2Pixel(0,0) = %d, want 0xF0 (0xF0 + palette offset 0<<4, post-T-31 full-8-bit index)", got)
	}
}

func TestLayer2PixelHonoursPaletteOffset(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	mem.nextPageWrite(16, 0, 0x00) // raw pixel value 0
	io.SetNextRegister(0x70, 0x03) // palette offset 3 in bits 3-0
	got := mem.layer2Pixel(io, 0, 0)
	if got != 0x30 {
		t.Fatalf("layer2Pixel with palette offset 3 on raw 0x00 = %d, want 0x30 (0 + 3<<4, post-T-31 full-8-bit index)", got)
	}
}

// ============================================================================
// T-29 completion pass: wider Layer 2 modes (NR 0x70 bits 5-4) and the
// Layer 2 Access Port (0x123B).
// ============================================================================

func TestLayer2CurrentModeDecodesNR0x70(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	cases := []struct {
		bits uint8
		want layer2Mode
	}{
		{0b00, layer2Mode256x192x8bpp},
		{0b01, layer2Mode320x256x8bpp},
		{0b10, layer2Mode640x256x4bpp},
		{0b11, layer2Mode256x192x8bpp}, // reserved -- safe fallback
	}
	for _, c := range cases {
		io.SetNextRegister(0x70, c.bits<<4)
		if got := io.layer2CurrentMode(); got != c.want {
			t.Fatalf("layer2CurrentMode() with NR 0x70 bits5-4=0b%02b = %v, want %v", c.bits, got, c.want)
		}
	}
}

func TestLayer2FramebufferDimensionsPerMode(t *testing.T) {
	cases := []struct {
		mode      layer2Mode
		wantW     int
		wantH     int
		wantPages uint8
	}{
		{layer2Mode256x192x8bpp, 256, 192, 6},
		{layer2Mode320x256x8bpp, 320, 256, 10},
		{layer2Mode640x256x4bpp, 640, 256, 10},
	}
	for _, c := range cases {
		w, h := layer2FramebufferDimensions(c.mode)
		if w != c.wantW || h != c.wantH {
			t.Fatalf("layer2FramebufferDimensions(%v) = (%d,%d), want (%d,%d)", c.mode, w, h, c.wantW, c.wantH)
		}
		if got := layer2FramebufferPages(c.mode); got != c.wantPages {
			t.Fatalf("layer2FramebufferPages(%v) = %d, want %d", c.mode, got, c.wantPages)
		}
	}
}

// TestLayer2Pixel320x256UsesColumnMajorAddressing verifies address =
// x*256+y (column-major: for each column x left-to-right, all of that
// column's rows y=0..255 top-to-bottom, matching the wiki's own "stored
// ... going from top to bottom and left to right").
//
// Cross-checked against the wiki's own two illustrative examples under
// 1-indexed ordinal numbering ("second byte" = address 1, "256th byte" =
// address 255): "second byte is first pixel on second line" matches
// EXACTLY (addr(x=0,y=1) = 0*256+1 = 1); "256th byte is second pixel on
// first line" is off by exactly one under this formula (addr(x=1,y=0) =
// 1*256+0 = 256, not 255) -- a one-sided discrepancy consistent with the
// wiki using "the 256th byte" colloquially to mean "at offset 256"
// rather than strictly ordinal, not evidence the formula itself is
// wrong. This test asserts the formula (address=x*256+y), not the
// wiki's second, looser illustrative phrase.
func TestLayer2Pixel320x256UsesColumnMajorAddressing(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x12, 16)        // base bank
	io.SetNextRegister(0x70, 0b0001<<4) // mode %01, 320x256x8bpp

	// Screen (x=0,y=1) -- "first pixel on second line" -- must land at
	// byte offset 1 (the wiki's own "second byte", 1-indexed), matching
	// address=x*256+y=0*256+1=1.
	mem.nextPageWrite(16, 1, 0x77)
	if got := mem.layer2Pixel(io, 0, 1); got != 0x77 {
		t.Fatalf("layer2Pixel(0,1) in 320x256 mode = 0x%02X, want 0x77 (column-major: address=x*256+y=1)", got)
	}

	// (x=0,y=0) and (x=1,y=0) must NOT collide -- row-major addressing
	// would put them at adjacent offsets 0/1; column-major puts them a
	// full column (256 bytes) apart.
	mem.nextPageWrite(16, 0, 0xAA)   // (0,0) -> address 0
	mem.nextPageWrite(16, 256, 0xBB) // (1,0) -> address 1*256+0=256
	if got := mem.layer2Pixel(io, 0, 0); got != 0xAA {
		t.Fatalf("layer2Pixel(0,0) in 320x256 mode = 0x%02X, want 0xAA", got)
	}
	if got := mem.layer2Pixel(io, 1, 0); got != 0xBB {
		t.Fatalf("layer2Pixel(1,0) in 320x256 mode = 0x%02X, want 0xBB (column-major addressing, NOT row-major's 0x01)", got)
	}
}

func TestLayer2Pixel640x256PacksTwoPixelsPerByte(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x12, 16)
	io.SetNextRegister(0x70, 0b0010<<4) // mode %10, 640x256x4bpp

	// Byte at column-major address (pairX=0, y=0) = address 0. Top
	// nibble (0xA) is the "left" pixel (x=0), bottom nibble (0xB) is
	// the "right" pixel (x=1), per the wiki's explicit wording.
	mem.nextPageWrite(16, 0, 0xAB)
	left := mem.layer2Pixel(io, 0, 0)
	right := mem.layer2Pixel(io, 1, 0)
	if left != 0x0A {
		t.Fatalf("layer2Pixel(0,0) in 640x256 mode (left/top nibble) = 0x%02X, want 0x0A", left)
	}
	if right != 0x0B {
		t.Fatalf("layer2Pixel(1,0) in 640x256 mode (right/bottom nibble) = 0x%02X, want 0x0B", right)
	}
}

func TestLayer2AccessPortStandardModeMapsReadsAndWrites(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x12, 32) // visible base bank = page 32

	// Port 0x123B: bits7-6=00 (first 16K third), bit3=0 (target NR 0x12,
	// visible), bit2=1 (read enable), bit0=1 (write enable).
	io.WritePort(layer2AccessPortAddr, 0b00000101)

	// A write to CPU address 0x0000 (slot 0, offset 0) should land in
	// Layer 2 page 32, offset 0 -- not in whatever ROM/RAM 0x0000
	// normally maps to.
	mem.Write(0x0000, 0x42)
	got := mem.nextPageRead(32, 0)
	if got != 0x42 {
		t.Fatalf("Layer 2 page 32 offset 0 after writing CPU address 0x0000 through the access port = 0x%02X, want 0x42", got)
	}

	// A read from the same CPU address must reflect Layer 2 memory, not
	// the underlying MMU slot's own page.
	mem.nextPageWrite(32, 1, 0x99)
	if got := mem.Read(0x0001); got != 0x99 {
		t.Fatalf("CPU read of address 0x0001 through the access port = 0x%02X, want 0x99 (Layer 2 page 32 offset 1)", got)
	}
}

func TestLayer2AccessPortReadDisabledFallsThroughToMMU(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	NewSpectrumIO(mem, nil) // wires mem.layer2AccessReadFn/WriteFn -- io itself unused, only its construction side effect on mem matters here

	// Port not written at all (power-on state): both enable bits clear.
	// A CPU access to the low 16K must fall through to the ordinary MMU
	// slot path entirely untouched.
	mem.SetNextSlot(0, 40)
	mem.nextPageWrite(40, 0, 0x11)
	if got := mem.Read(0x0000); got != 0x11 {
		t.Fatalf("Read(0x0000) with the access port never written = 0x%02X, want 0x11 (falls through to slot 0's own page 40)", got)
	}
}

func TestLayer2AccessPortUsesShadowBankWhenSelected(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x12, 32) // visible bank
	io.SetNextRegister(0x13, 64) // shadow bank

	// bit3=1 selects NR 0x13 (shadow) as the mapped bank; read+write
	// enabled.
	io.WritePort(layer2AccessPortAddr, 0b00001101)
	mem.Write(0x0000, 0x55)
	if got := mem.nextPageRead(64, 0); got != 0x55 {
		t.Fatalf("write through the access port with the shadow bit set landed in page %d's data, want page 64 (NR 0x13)", got)
	}
	if got := mem.nextPageRead(32, 0); got == 0x55 {
		t.Fatalf("write through the access port with the shadow bit set ALSO landed in the visible bank (page 32) -- must only affect the shadow bank")
	}
}

func TestLayer2AccessPortReadBackReturnsStoredByte(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.WritePort(layer2AccessPortAddr, 0xC7)
	if got := io.ReadPort(layer2AccessPortAddr); got != 0xC7 {
		t.Fatalf("ReadPort(0x123B) = 0x%02X, want 0xC7 (plain read-back of the last written control byte)", got)
	}
}

func TestLayer2ClipWindowDefaultsFullScreen(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	x1, x2, y1, y2 := io.layer2ClipWindow()
	if x1 != 0 || x2 != 255 || y1 != 0 || y2 != 191 {
		t.Fatalf("layer2ClipWindow() = (%d,%d,%d,%d), want (0,255,0,191)", x1, x2, y1, y2)
	}
}

func TestLayer2ClipWindowMasksOutsidePixels(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	// Fill the whole framebuffer with a bright, non-zero pixel value so a
	// transparent (clipped) pixel is unambiguous from an opaque one.
	for page := uint8(16); page < 16+layer2PageCount; page++ {
		for off := uint16(0); off < 8192; off++ {
			mem.nextPageWrite(page, off, 0xF0)
		}
	}
	// Clip window sequential writes: X1,X2,Y1,Y2 -- restrict to a small box.
	io.WritePort(nextRegSelectPort, 0x18)
	for _, v := range []uint8{10, 20, 10, 20} {
		io.WritePort(nextRegDataPort, v)
	}
	img := io.DecodeLayer2()
	if img == nil {
		t.Fatalf("DecodeLayer2() = nil, want an image")
	}
	inside := img.RGBAAt(15, 15)
	if inside.A == 0 {
		t.Fatalf("pixel (15,15) inside clip window (10-20,10-20) is transparent, want opaque")
	}
	outside := img.RGBAAt(0, 0)
	if outside.A != 0 {
		t.Fatalf("pixel (0,0) outside clip window is opaque (%v), want transparent", outside)
	}
}

func TestLayer2ULAOnTopDecodesRegister0x15(t *testing.T) {
	cases := []struct {
		name   string
		value  uint8 // full NR 0x15 value, bits 4-2 are the priority field
		ulaTop bool
	}{
		{"000 S L U -> Layer2 on top", 0b00000000, false},
		{"001 L S U -> Layer2 on top", 0b00000100, false},
		{"010 S U L -> ULA on top", 0b00001000, true},
		{"011 L U S -> Layer2 on top", 0b00001100, false},
		{"100 U S L -> ULA on top", 0b00010000, true},
		{"101 U L S -> ULA on top", 0b00010100, true},
		{"110 blend fallback -> ULA on top", 0b00011000, true},
		{"111 blend fallback -> ULA on top", 0b00011100, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mem, _ := newTestMemoryAndScreen()
			mem.EnableNext()
			io := NewSpectrumIO(mem, nil)
			io.SetNextRegister(0x15, c.value)
			if got := io.layer2ULAOnTop(); got != c.ulaTop {
				t.Fatalf("layer2ULAOnTop() with NR 0x15=0x%02X = %v, want %v", c.value, got, c.ulaTop)
			}
		})
	}
}

func TestCompositeLayer2PrefersULAWhenOnTop(t *testing.T) {
	ula := image.NewRGBA(image.Rect(0, 0, 2, 2))
	l2 := image.NewRGBA(image.Rect(0, 0, 2, 2))
	ula.SetRGBA(0, 0, zxPalette[1])
	l2.SetRGBA(0, 0, zxPalette[2])
	out := CompositeLayer2(ula, l2, true)
	if out.RGBAAt(0, 0) != zxPalette[1] {
		t.Fatalf("CompositeLayer2 with ulaOnTop=true picked %v, want ULA's colour %v", out.RGBAAt(0, 0), zxPalette[1])
	}
}

func TestCompositeLayer2PrefersLayer2WhenOnTopAndOpaque(t *testing.T) {
	ula := image.NewRGBA(image.Rect(0, 0, 2, 2))
	l2 := image.NewRGBA(image.Rect(0, 0, 2, 2))
	ula.SetRGBA(0, 0, zxPalette[1])
	l2.SetRGBA(0, 0, zxPalette[2])
	out := CompositeLayer2(ula, l2, false)
	if out.RGBAAt(0, 0) != zxPalette[2] {
		t.Fatalf("CompositeLayer2 with ulaOnTop=false picked %v, want Layer 2's colour %v", out.RGBAAt(0, 0), zxPalette[2])
	}
}

func TestCompositeLayer2TransparentLayer2FallsThroughToULA(t *testing.T) {
	ula := image.NewRGBA(image.Rect(0, 0, 2, 2))
	l2 := image.NewRGBA(image.Rect(0, 0, 2, 2)) // left zero-valued: alpha 0
	ula.SetRGBA(0, 0, zxPalette[1])
	out := CompositeLayer2(ula, l2, false) // Layer2-on-top priority, but transparent
	if out.RGBAAt(0, 0) != zxPalette[1] {
		t.Fatalf("CompositeLayer2 with a transparent Layer2 pixel picked %v, want ULA's colour %v (fall-through)", out.RGBAAt(0, 0), zxPalette[1])
	}
}

// TestDecodeDisplayNoOpOnClassicMachine confirms T-29's own "no destructive
// change" requirement directly at the DecodeDisplay call site both GUI and
// headless front ends use: a classic machine's decoded image is byte-for-
// byte identical whether or not layer2.go exists at all, since DecodeLayer2
// returns nil and DecodeDisplay short-circuits to the plain ULA decode.
func TestDecodeDisplayNoOpOnClassicMachine(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	rom := make([]byte, 16384) // 48K
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

// TestT29NextModeLayer2EndToEnd writes a Layer 2 configuration and a known
// framebuffer pattern via real Z80N NEXTREG/OUT instructions (matching
// TestT28NextModeMultiSlotProgram's style), then confirms DecodeDisplay
// produces a correctly composited picture: explicitly-selected Layer2-
// over-ULA priority (%000 S L U) shows the Layer 2 pattern, and switching
// NR 0x15 to put ULA on top makes the same pixel show the ULA screen
// instead.
func TestT29NextModeLayer2EndToEnd(t *testing.T) {
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

	// NEXTREG $12,$10 -- Layer 2 visible base bank = page 16 (first extra
	// page, clear of the aliased-to-ram range).
	run(0x8000, []uint8{0xED, 0x91, 0x12, 0x10})
	// Explicitly select %000 S L U (Layer2-over-ULA) via NR 0x15 rather than
	// relying on the construction-time default -- that default is actually
	// 0x08 (bits 4-2 = %010 S U L, ULA-on-top per layer2ULAOnTop's corrected
	// decode), NOT 0x00/%000 as this test's comment previously and
	// incorrectly assumed; that wrong assumption happened to still pass
	// before this session's %010 classification fix, since the old (buggy)
	// layer2ULAOnTop also returned Layer2-on-top for %010, for the wrong
	// reason. Explicit here so this test's pass/fail no longer depends on
	// what NR 0x15 happens to default to.
	run(0x8020, []uint8{0xED, 0x91, 0x15, 0b00000000})

	// Give the ULA screen a known, different colour (paper attribute byte
	// 0x07 = white paper/black ink at 0,0's attribute cell) so a wrong
	// composite is visibly distinguishable from the Layer 2 pattern.
	zx.screen.attributes[0] = 0x07

	// Write a Layer 2 pixel (raw 0xF0, palette offset 0 -> resolved index
	// 0xF0) directly into page 16 offset 0 -- pixel (0,0) -- then give
	// that index a known, bright colour in the Layer2-1st palette via
	// real NEXTREG writes (T-31), since the palette's power-on default is
	// all-black (nextpalette.go's own documented gap) and this test needs
	// a colour visibly distinguishable from both black and the ULA
	// screen's white-paper/black-ink attribute set above.
	zx.memory.nextPageWrite(16, 0, 0xF0)
	// NEXTREG $43,$10 -- R/W target Layer2-1st (bits 6-4 = 001, since
	// nextPaletteTarget's iota order is ULA1st=0, Layer2_1st=1,
	// Sprites1st=2, Tilemap1st=3, ...2nd variants 4-7), display defaults
	// (2nd-palette bits clear) so Layer2-1st is also what's shown.
	run(0x8030, []uint8{0xED, 0x91, 0x43, 0x10})
	// NEXTREG $40,$F0 -- select palette index 0xF0 (matching the resolved
	// index above).
	run(0x8040, []uint8{0xED, 0x91, 0x40, 0xF0})
	// NEXTREG $41,$FF -- RRRGGGBB = 11111111 -> bright white once decoded
	// (R=7 G=7 B=3, third blue bit synthesised as 1 OR 1 = 1 -> B=7).
	run(0x8050, []uint8{0xED, 0x91, 0x41, 0xFF})

	img := zx.DecodeDisplay()
	got := img.RGBAAt(0, 0)
	wantN := zxpalette.DecodeRGB9From8Bit(0xFF).Colour()
	want := color.RGBA{R: wantN.R, G: wantN.G, B: wantN.B, A: wantN.A} // opaque (A=0xff), so NRGBA==RGBA channel-for-channel
	if got != want {
		t.Fatalf("pixel (0,0) with Layer2-over-ULA priority = %v, want Layer 2's bright white %v", got, want)
	}

	// Now flip NR 0x15 to %100 (U S L -- ULA on top) via the port path and
	// confirm the same pixel now shows the ULA layer instead.
	run(0x8010, []uint8{0xED, 0x91, 0x15, 0b00010000})
	img2 := zx.DecodeDisplay()
	got2 := img2.RGBAAt(0, 0)
	if got2 == want {
		t.Fatalf("pixel (0,0) after switching NR 0x15 to ULA-on-top still shows Layer 2's colour %v", want)
	}
}

// TestNextPaletteVersionBumpsOnlyOnRegister0x70 exercises the palette-
// version counter proposed in docs/proposals/next-gpu-compositing-seam.md
// (nextPaletteVersion, io.go field; incremented in nextRegWriteDirect and
// SetNextRegister, nextreg.go): writes to NR 0x70 must bump it, on both
// live-write paths (port 0x253B, and the Z80N NEXTREG-opcode path via
// nextRegWriteDirect directly, matching how nextRegWriteSelected/
// nextRegOpcodeWrite both funnel into it) and the snapshot-restore path
// (SetNextRegister) -- and writes to every OTHER register covered by this
// test's sweep must leave it completely unchanged, since a cache keyed on
// this counter (renderLayer2GPU's future consumer, T-30's spritePatternGPU
// later) would otherwise rebake far more often than any resolved colour
// actually changed.
func TestNextPaletteVersionBumpsOnlyOnRegister0x70(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)

	if io.nextPaletteVersion != 0 {
		t.Fatalf("nextPaletteVersion at construction = %d, want 0", io.nextPaletteVersion)
	}

	// A sweep of registers that are NOT 0x70, touching several already-
	// meaningful ones (Layer 2's own 0x12/0x18, a T-28 MMU slot, and a
	// read-only identification register via the port path, where the
	// read-only gate actually applies -- SetNextRegister's own
	// snapshot-restore path does not consult it at all) so this test would
	// catch the increment being accidentally hung off a wider condition
	// than "reg == 0x70", e.g. off nextRegRW's access check, or off "any
	// Layer 2 register", either of which this sweep deliberately exercises.
	io.WritePort(nextRegSelectPort, 0x12)
	io.WritePort(nextRegDataPort, 0x10)
	io.WritePort(nextRegSelectPort, 0x50)
	io.WritePort(nextRegDataPort, 0x03)
	io.SetNextRegister(0x18, 0x00)
	io.WritePort(nextRegSelectPort, 0x00)
	io.WritePort(nextRegDataPort, 0xFF) // 0x00 is read-only -- port path ignores this write
	if io.nextPaletteVersion != 0 {
		t.Fatalf("nextPaletteVersion after writes to registers other than 0x70 = %d, want 0", io.nextPaletteVersion)
	}

	// Live write via the port-0x253B path (nextRegWriteSelected ->
	// nextRegWriteDirect).
	io.WritePort(nextRegSelectPort, 0x70)
	io.WritePort(nextRegDataPort, 0x03)
	if io.nextPaletteVersion != 1 {
		t.Fatalf("nextPaletteVersion after one NR 0x70 port write = %d, want 1", io.nextPaletteVersion)
	}

	// Live write via the Z80N NEXTREG-opcode path (nextRegWriteDirect
	// called directly, bypassing io.nextRegs.selected entirely -- see
	// nextRegWriteDirect's own doc comment on why this path exists
	// separately from the port path).
	io.nextRegWriteDirect(0x70, 0x05)
	if io.nextPaletteVersion != 2 {
		t.Fatalf("nextPaletteVersion after one NR 0x70 nextRegWriteDirect call = %d, want 2", io.nextPaletteVersion)
	}

	// Snapshot-restore write via SetNextRegister -- must bump the counter
	// too, the same as the two live-write paths above, per this function's
	// own doc comment on mirroring nextRegWriteDirect's 0x70 handling.
	io.SetNextRegister(0x70, 0x07)
	if io.nextPaletteVersion != 3 {
		t.Fatalf("nextPaletteVersion after one NR 0x70 SetNextRegister call = %d, want 3", io.nextPaletteVersion)
	}
}
