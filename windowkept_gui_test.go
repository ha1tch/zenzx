//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"testing"

	"github.com/ha1tch/zenui"
)

// The window the person enlarged or maximised stays as it is (T-75): nothing the display or
// the bar does of its own accord may send it back to the display's size.

// keptDisplay is a display at zoom 2 that has its window at the size it asked for.
func keptDisplay(t *testing.T) *DisplayManager {
	t.Helper()
	dm := testZX().window()
	dm.screen.multiplier = 2
	dm.updateTargetSize()
	dm.currentWidth, dm.currentHeight = dm.targetWidth, dm.targetHeight
	dm.isAnimating = false
	return dm
}

func TestAKeptWindowIsNotSentToANewSize(t *testing.T) {
	dm := keptDisplay(t)
	nativeW, nativeH := dm.pictureNativeSize(dm.borderView().Shown)

	// A window that is being kept follows a new reservation at the top without being asked to
	// move: the display's size is the picture's, at once, and nothing is animated.
	dm.windowKept = true
	dm.SetReservedTopHeight(32)
	if dm.isAnimating {
		t.Error("the reservation changed on a kept window and the window is being resized")
	}
	if want := nativeH*2 + 32; dm.currentHeight != want || dm.targetHeight != want {
		t.Errorf("the display's height is %d (target %d), want %d", dm.currentHeight, dm.targetHeight, want)
	}
	if dm.currentWidth != nativeW*2 || dm.targetWidth != nativeW*2 {
		t.Errorf("the display's width is %d (target %d), want %d", dm.currentWidth, dm.targetWidth, nativeW*2)
	}
	if !dm.windowKept {
		t.Error("the window is no longer kept after a change of the reservation")
	}

	// A window that is not kept is sent to the new size, as it always was.
	dm = keptDisplay(t)
	dm.SetReservedTopHeight(32)
	if !dm.isAnimating || dm.targetHeight != nativeH*2+32 {
		t.Errorf("the reservation changed on a window that is not kept: animating %v, target height %d", dm.isAnimating, dm.targetHeight)
	}

	// Asking for a size ends it: the window is on its way to a size, and is not kept.
	dm = keptDisplay(t)
	dm.windowKept = true
	dm.updateTargetSize()
	if dm.windowKept {
		t.Error("a window that was sent to a size is still kept")
	}
}

func TestTheSpaceAFixedBarReservesDoesNotFollowTheWindow(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()
	dm := zx.window()
	dm.screen.multiplier = 2
	dm.updateTargetSize()

	// The scale the bar is drawn at follows the window it is in, however wide: this is what a
	// maximised window does to it. The reservation, which sizes the window, does not.
	want := b.stripFor(b.barScale(dm.targetWidth, dm.targetHeight-dm.reservedTopHeight))
	for _, k := range []int{1, 2, 3} {
		b.barK = k
		if got := b.reserveH(dm); got != want {
			t.Errorf("with the bar drawn at X%d the reservation is %d, want %d (the display's own window decides it)", k, got, want)
		}
	}
	// And it does follow the display's own size, which a zoom changes: more room, a bigger bar.
	small := b.reserveH(dm)
	dm.screen.multiplier = 5
	dm.updateTargetSize()
	if big := b.reserveH(dm); big < small {
		t.Errorf("at zoom 5 the reservation is %d, less than %d at zoom 2", big, small)
	}
}

func TestAFixedBarDrawnTallerThanItsReservationMovesTheFrame(t *testing.T) {
	dm := keptDisplay(t)
	dm.reservedTopHeight = 24
	dm.barOverlay = 0
	if got := dm.frameTop(); got != 24 {
		t.Errorf("the frame begins at %d, want the reservation, 24", got)
	}
	dm.barOverlay = 48 // the bar, drawn at a bigger scale in a window larger than the display's
	if got := dm.frameTop(); got != 48 {
		t.Errorf("the frame begins at %d under a bar 48 high, want 48", got)
	}
	dm.barOverlay = 16 // a smaller bar than what is reserved: the reservation stands
	if got := dm.frameTop(); got != 24 {
		t.Errorf("the frame begins at %d, want the reservation, 24", got)
	}
}
