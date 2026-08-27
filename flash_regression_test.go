package main

import (
	"testing"
	"time"
)

// TestStandardFlashSwapsInkAndPaper is the standard-mode counterpart to
// videorender_hicolour_test.go's TestHicolourFlashIgnored: that test
// verifies hi-colour correctly ignores FLASH (a deliberate design choice);
// this one verifies standard mode correctly honours it, which had no test
// coverage at all before this regression -- a real gap that let FLASH stop
// blinking on every model, not just TS2068's hi-colour mode, go unnoticed.
func TestStandardFlashSwapsInkAndPaper(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()

	off := screen.calcByteOffset(0, 0)
	screen.bitmap[off] = 0x80 // pixel (0,0) is an "ink" bitmap bit
	// ink=blue(1), paper=yellow(6), FLASH bit (0x80) set. Same encoding
	// TestHicolourFlashIgnored uses, for the same attribute byte meaning
	// in both tests.
	screen.attributes[0] = 0x80 | (6 << 3) | 1

	renderer := standardVideoRenderer{}

	screen.flashEnabled = true

	screen.flashTickTock = false
	img := renderer.Decode(mem, screen)
	gotInkPhase := img.At(0, 0)
	wantInk := zxPalette[1] // blue
	if gotInkPhase != wantInk {
		t.Errorf("flashTickTock=false: pixel (0,0) = %+v, want ink colour %+v (not yet swapped)", gotInkPhase, wantInk)
	}

	screen.flashTickTock = true
	img = renderer.Decode(mem, screen)
	gotPaperPhase := img.At(0, 0)
	wantPaper := zxPalette[6] // yellow
	if gotPaperPhase != wantPaper {
		t.Errorf("flashTickTock=true: pixel (0,0) = %+v, want paper colour %+v (ink/paper should be swapped)", gotPaperPhase, wantPaper)
	}
}

func TestStandardFlashDisabledNeverSwaps(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()

	off := screen.calcByteOffset(0, 0)
	screen.bitmap[off] = 0x80                  // pixel (0,0) is an "ink" bitmap bit
	screen.attributes[0] = 0x80 | (6 << 3) | 1 // same as above: FLASH set, blue on yellow

	screen.flashEnabled = false
	screen.flashTickTock = true // would swap if flashEnabled were honoured

	renderer := standardVideoRenderer{}
	img := renderer.Decode(mem, screen)
	got := img.At(0, 0)
	want := zxPalette[1] // ink (blue) -- must stay unswapped
	if got != want {
		t.Errorf("flashEnabled=false: pixel (0,0) = %+v, want ink colour %+v (FLASH must be fully disabled, not just un-ticked)", got, want)
	}
}

// TestUpdateFlashTogglesAfterInterval is the direct regression test for
// the actual bug: updateFlash() was fully correct in isolation (this test
// would have passed even before the fix), but nothing anywhere called it
// during real rendering (display.go's Render()), so flashTickTock never
// advanced no matter how long the emulator ran. This test covers
// updateFlash()'s own timing correctness; the missing call site itself is
// fixed directly in display.go's Render (verified by inspection -- it's
// the function's first line -- since exercising the full raylib Render
// path needs a live GL context this environment doesn't have).
func TestUpdateFlashTogglesAfterInterval(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	screen.flashEnabled = true
	initial := screen.flashTickTock

	// Not enough time elapsed yet -- should not toggle.
	screen.lastFlashTime = time.Now()
	screen.updateFlash()
	if screen.flashTickTock != initial {
		t.Error("updateFlash toggled flashTickTock before 320ms elapsed")
	}

	// Simulate 320ms+ having passed since the last toggle.
	screen.lastFlashTime = time.Now().Add(-321 * time.Millisecond)
	screen.updateFlash()
	if screen.flashTickTock == initial {
		t.Error("updateFlash did not toggle flashTickTock after 320ms elapsed")
	}
}

func TestUpdateFlashDisabledNeverTicks(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	screen.flashEnabled = false
	initial := screen.flashTickTock

	screen.lastFlashTime = time.Now().Add(-time.Second) // well past 320ms
	screen.updateFlash()
	if screen.flashTickTock != initial {
		t.Error("updateFlash ticked while flashEnabled=false")
	}
}
