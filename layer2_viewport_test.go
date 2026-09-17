//go:build !headless

package main

import (
	"fmt"
	"testing"
)

// This file tests the GUI window/viewport-sizing logic added 2026-09-15
// (T-29's post-completion-pass GUI-wiring fix, prompted by Horacio
// pointing out that the wider Layer 2 modes' data-plane correctness --
// layer2Pixel addressing the right byte for a given (x,y) -- had never
// been checked end-to-end through the window/viewport/border machinery,
// which turned out to still assume the classic 256x192 canvas
// exclusively). None of these call a raylib draw function -- totalSize,
// minMultiplierForMode, classicScreenOffset and nextLayersViewportOffset
// are all pure arithmetic over DisplayManager's own fields plus (for
// maxMultiplierThatFits alone) a monitor-size query that already degrades
// safely with no window (see this file's existing menubar_checkmarks_test.go
// precedent) -- so, like layer2_gui_test.go/sprite_gui_test.go, these run
// against a DisplayManager built via NewDisplayManager with no live GL
// context at all.

// TestTotalSizeIsFixedReferenceCanvasForStandardRenderer confirms
// totalSize() returns the permanent RefWidth x RefHeight canvas for the
// standard renderer regardless of Layer 2's mode -- the central "the
// window must never shapeshift" requirement. totalSize() takes no mode
// input at all, so this is really confirming it depends on nothing but
// the active VideoRenderer's identity.
func TestTotalSizeIsFixedReferenceCanvasForStandardRenderer(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	dm.SetVideoRenderer(standardVideoRenderer{})

	w, h := dm.totalSize()
	if w != RefWidth || h != RefHeight {
		t.Fatalf("totalSize() = (%d,%d), want (%d,%d) = (RefWidth,RefHeight)", w, h, RefWidth, RefHeight)
	}
	if w != 320 {
		t.Errorf("RefWidth = %d, want 320 (must stay TotalWidth-equal: 320-wide Layer 2 mode already fits the classic border's total width exactly)", w)
	}
	if h != 256 {
		t.Errorf("RefHeight = %d, want 256 (must fit 320x256 Layer 2 mode's real height with zero cropping)", h)
	}
}

// TestTotalSizeUnaffectedByNonStandardRenderer confirms the fixed
// reference canvas is specific to the standard renderer -- any other
// renderer (Layer 2/sprites never run alongside one) keeps the
// pre-T-29-GUI-wiring-pass screenW/screenH+border computation exactly.
func TestTotalSizeUnaffectedByNonStandardRenderer(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	var hc hicolourVideoRenderer
	dm.SetVideoRenderer(hc)

	wantW, wantH := hc.Dimensions()
	bl, br, bt, bb := hc.BorderMargins()
	wantW += bl + br
	wantH += bt + bb

	w, h := dm.totalSize()
	if w != wantW || h != wantH {
		t.Fatalf("totalSize() with hicolourVideoRenderer = (%d,%d), want (%d,%d) (screenW/H+border, unaffected by RefWidth/RefHeight)", w, h, wantW, wantH)
	}
}

// TestMinMultiplierForModeGates640Mode confirms the 640x256x4bpp clamp:
// 1x is refused only when Layer 2 is enabled AND in that specific mode.
// Every other combination (disabled, or a different mode) allows 1x.
func TestMinMultiplierForModeGates640Mode(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)

	if got := dm.minMultiplierForMode(); got != 1 {
		t.Fatalf("minMultiplierForMode() with dm.io == nil = %d, want 1 (no gate)", got)
	}

	// A non-Next machine: layer2Enabled() (layer2.go) is io.memory.isNext,
	// NOT any NR 0x12 bit -- confirmed by reading that method directly
	// rather than assumed; a plain SpectrumMemory never calls EnableNext,
	// so layer2Enabled() is permanently false and the mode bits (which a
	// non-Next machine has no real NR 0x70 register behind anyway) can't
	// matter.
	classicMem := NewSpectrumMemory(screen)
	classicIO := NewSpectrumIO(classicMem, nil)
	classicIO.SetNextRegister(0x70, 0b10<<4) // mode bits irrelevant here
	dm.SetSpectrumIO(classicIO)
	if got := dm.minMultiplierForMode(); got != 1 {
		t.Fatalf("minMultiplierForMode() on a non-Next machine = %d, want 1 (layer2Enabled() must be false)", got)
	}

	// A Next machine (EnableNext, memory.go) across all three modes --
	// isNext has no reverse (no DisableNext), so this is a fresh
	// mem/io per subtest rather than one shared instance toggled.
	cases := []struct {
		name     string
		modeBits uint8 // NR 0x70 bits 5-4
		wantMin  int
	}{
		{"256x192 mode", 0b00 << 4, 1},
		{"320x256 mode", 0b01 << 4, 1},
		{"640x256 mode", 0b10 << 4, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mem := NewSpectrumMemory(screen)
			mem.EnableNext()
			io := NewSpectrumIO(mem, nil)
			io.SetNextRegister(0x70, c.modeBits)
			dm.SetSpectrumIO(io)
			if got := dm.minMultiplierForMode(); got != c.wantMin {
				t.Errorf("minMultiplierForMode() = %d, want %d", got, c.wantMin)
			}
		})
	}
}

// TestScaleDownRespectsModeMinimum confirms ScaleDown itself, not just
// the raw minMultiplierForMode helper, refuses to go below the mode's
// floor -- the actual behaviour a PageDown keypress (input.go) triggers.
func TestScaleDownRespectsModeMinimum(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	mem := NewSpectrumMemory(screen)
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	dm.SetSpectrumIO(io)
	io.SetNextRegister(0x70, 0b10<<4) // 640x256x4bpp

	dm.screen.multiplier = 2
	if ok := dm.ScaleDown(); ok {
		t.Fatalf("ScaleDown() from 2x while in 640-mode = true, want false (must not drop below minMultiplierForMode()=2)")
	}
	if dm.screen.multiplier != 2 {
		t.Fatalf("multiplier after refused ScaleDown() = %d, want unchanged 2", dm.screen.multiplier)
	}
}

// TestSetScaleRespectsModeMinimum is SetScale's equivalent of the above --
// used by the View menu's X1/X2/X3 items directly, a separate code path
// from ScaleUp/ScaleDown.
func TestSetScaleRespectsModeMinimum(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	mem := NewSpectrumMemory(screen)
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	dm.SetSpectrumIO(io)
	io.SetNextRegister(0x70, 0b10<<4) // 640x256x4bpp

	if ok := dm.SetScale(1); ok {
		t.Fatalf("SetScale(1) while in 640-mode = true, want false")
	}
	if ok := dm.SetScale(2); !ok {
		t.Fatalf("SetScale(2) while in 640-mode = false, want true (2x is the mode's own floor)")
	}
}

// TestClassicScreenOffsetAddsExtraTopMarginForStandardRenderer confirms
// the +4px top margin (classicScreenExtraTopMargin) is applied for the
// standard renderer at various multipliers, and NOT applied for a
// non-standard renderer -- the fix for RefHeight being 8px taller than
// the pre-existing TotalHeight without changing the classic picture's own
// border proportions asymmetrically.
func TestClassicScreenOffsetAddsExtraTopMarginForStandardRenderer(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	dm.SetVideoRenderer(standardVideoRenderer{})

	for _, mult := range []int{1, 2, 3} {
		dm.screen.multiplier = mult
		x, y := dm.classicScreenOffset()
		wantX := int32(BorderLeft * mult)
		wantY := int32((BorderTop + classicScreenExtraTopMargin) * mult)
		if x != wantX || y != wantY {
			t.Errorf("mult=%d: classicScreenOffset() = (%d,%d), want (%d,%d)", mult, x, y, wantX, wantY)
		}
	}
}

func TestClassicScreenOffsetUnchangedForNonStandardRenderer(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	var hc hicolourVideoRenderer
	dm.SetVideoRenderer(hc)
	dm.screen.multiplier = 2

	x, y := dm.classicScreenOffset()
	wantX := int32(dm.borderLeft * 2)
	wantY := int32(dm.borderTop * 2)
	if x != wantX || y != wantY {
		t.Errorf("classicScreenOffset() with hicolourVideoRenderer = (%d,%d), want (%d,%d) (no extra top margin)", x, y, wantX, wantY)
	}
}

// TestNextLayersViewportOffsetCentresEachMode is the core geometry test:
// for each Layer 2 mode, the viewport offset must centre that mode's real
// DISPLAYED footprint (layer2DisplayedSize, layer2.go -- not simply
// layer2FramebufferDimensions x mult) within the fixed RefWidth x
// RefHeight canvas. Verified by direct arithmetic, cross-checked against
// hand-computed expectations for each mode rather than re-deriving the
// implementation's own formula:
//   - 256x192 (base mode) is the one deliberate exception -- it does NOT
//     use the centring formula at all, but returns classicScreenOffset()
//     exactly (32, 28 at 1x: borderLeft, borderTop+classicScreenExtraTopMargin),
//     since Layer 2's base mode composites pixel-for-pixel with the ULA
//     layer and the two MUST share one offset or misalign -- see
//     nextLayersViewportOffset's own doc comment for the 4px mismatch this
//     caught before shipping.
//   - 320x256 centred in 320x256 is (0,0) exactly: RefWidth's own value
//     was chosen specifically so this mode has zero leftover margin.
//   - 640x256 ALSO centres to (0,0) at every multiplier, including 1x --
//     confirmed via an exact wiki.specnext.dev quote ("every byte is
//     displayed as two half-width paired pixels"): 640 half-width pixels
//     occupy the identical picture width as 320 full-width pixels, so
//     there is no overflow to centre away at any zoom level. This
//     corrects an earlier version of this test that expected a negative
//     offset at 1x from a (wrong) assumption that every mode's pixels
//     draw at uniform full width.
func TestNextLayersViewportOffsetCentresEachMode(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	dm.SetVideoRenderer(standardVideoRenderer{})
	mem := NewSpectrumMemory(screen)
	io := NewSpectrumIO(mem, nil)

	for _, mult := range []int{1, 2, 3} {
		dm.screen.multiplier = mult
		cases := []struct {
			name         string
			modeBits     uint8
			wantX, wantY int32
		}{
			{"256x192 (base mode, == classicScreenOffset)", 0b00 << 4, int32(BorderLeft * mult), int32((BorderTop + classicScreenExtraTopMargin) * mult)},
			{"320x256 (exact fit)", 0b01 << 4, 0, 0},
			{"640x256 (half-width pixels, also exact fit)", 0b10 << 4, 0, 0},
		}
		for _, c := range cases {
			t.Run(fmt.Sprintf("%s@%dx", c.name, mult), func(t *testing.T) {
				io.SetNextRegister(0x70, c.modeBits)
				x, y := dm.nextLayersViewportOffset(io)
				if x != c.wantX || y != c.wantY {
					t.Errorf("nextLayersViewportOffset() = (%d,%d), want (%d,%d)", x, y, c.wantX, c.wantY)
				}
			})
		}
	}
}

// TestLayer2DisplayedSizeHalvesWidthOnlyFor640Mode is the numerical
// verification underlying the whole design: 640-mode's displayed WIDTH
// must equal 320-mode's own displayed width at every multiplier (the
// wiki's "half-width paired pixels" claim), while its displayed HEIGHT
// stays identical to 320-mode's own height (only horizontal density
// doubles, confirmed by the wiki's own framing of 640-mode as "more like
// 320x256x8bpp mode" than a distinct picture size).
func TestLayer2DisplayedSizeHalvesWidthOnlyFor640Mode(t *testing.T) {
	for _, mult := range []int{1, 2, 3, 4, 5} {
		w320, h320 := layer2DisplayedSize(layer2Mode320x256x8bpp, mult)
		w640, h640 := layer2DisplayedSize(layer2Mode640x256x4bpp, mult)
		if w640 != w320 {
			t.Errorf("mult=%d: 640-mode displayed width = %v, want %v (== 320-mode's own)", mult, w640, w320)
		}
		if h640 != h320 {
			t.Errorf("mult=%d: 640-mode displayed height = %v, want %v (== 320-mode's own, unaffected by the width halving)", mult, h640, h320)
		}
	}
}

// TestNextLayersViewportOffsetFallsBackForNilIOOrNonStandardRenderer
// confirms the two documented fallback cases both reduce to
// classicScreenOffset()'s own value, matching this function's pre-T-29
// behaviour of reusing the classic offset unconditionally.
func TestNextLayersViewportOffsetFallsBackForNilIOOrNonStandardRenderer(t *testing.T) {
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	dm.SetVideoRenderer(standardVideoRenderer{})
	dm.screen.multiplier = 2

	wantX, wantY := dm.classicScreenOffset()
	if x, y := dm.nextLayersViewportOffset(nil); x != wantX || y != wantY {
		t.Errorf("nextLayersViewportOffset(nil) = (%d,%d), want classicScreenOffset() = (%d,%d)", x, y, wantX, wantY)
	}

	var hc hicolourVideoRenderer
	dm.SetVideoRenderer(hc)
	mem := NewSpectrumMemory(screen)
	io := NewSpectrumIO(mem, nil)
	wantX, wantY = dm.classicScreenOffset()
	if x, y := dm.nextLayersViewportOffset(io); x != wantX || y != wantY {
		t.Errorf("nextLayersViewportOffset(io) with hicolourVideoRenderer = (%d,%d), want classicScreenOffset() = (%d,%d)", x, y, wantX, wantY)
	}
}
