//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// A maximised or full screen window draws the picture in a canvas that is put in the middle of
// the window; a picture that flies out of the dock must land on the picture as it is there, not
// on where the canvas's own coordinates would put it in the window.
func TestPlaceInPictureFollowsTheCanvas(t *testing.T) {
	const band = 32
	// The display's window is 1024x(768+32); the window it is in is 1920x1080, with the bar 32 tall.
	p := centrePicture(1920, 1080, band, 1024, 768)
	if p.Scale != 1 || p.X != 448 || p.Y != float32(band+(1048-768)/2) {
		t.Fatalf("the picture is at %+v", p)
	}
	// The classic screen, 256x192 at a zoom of 3 with its border: the canvas has it at (192, 100).
	in := rl.Rectangle{X: 192, Y: 100, Width: 768, Height: 576}
	got := placeInPicture(in, p, band)
	want := rl.Rectangle{X: 448 + 192, Y: p.Y + (100 - band), Width: 768, Height: 576}
	if got != want {
		t.Errorf("at the picture's scale: %+v, want %+v", got, want)
	}
	// The whole of the canvas below its band is the whole of the picture.
	whole := placeInPicture(rl.Rectangle{X: 0, Y: band, Width: 1024, Height: 768}, p, band)
	if whole.X != p.X || whole.Y != p.Y || whole.Width != p.W || whole.Height != p.H {
		t.Errorf("the whole canvas: %+v, the picture %+v", whole, p)
	}
	// A window smaller than the picture scales it down, and what is in it with it.
	q := centrePicture(800, 632, band, 1024, 768)
	if q.Scale >= 1 {
		t.Fatalf("a window of 800x632 does not scale down: %+v", q)
	}
	small := placeInPicture(rl.Rectangle{X: 0, Y: band, Width: 1024, Height: 768}, q, band)
	if abs32(small.Width-q.W) > 0.01 || abs32(small.Height-q.H) > 0.01 || small.X != q.X || small.Y != q.Y {
		t.Errorf("scaled: %+v, the picture %+v", small, q)
	}
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// While the window is the size the display has, or the layout is tiled, nothing moves.
func TestInWindowIsTheIdentityWhereThereIsNoCanvas(t *testing.T) {
	dm := &DisplayManager{}
	in := rl.Rectangle{X: 10, Y: 20, Width: 30, Height: 40}
	if got := dm.inWindow(in); got != in {
		t.Errorf("no canvas: %+v", got)
	}
	dm.windowKept, dm.tiles = true, func() {}
	if got := dm.inWindow(in); got != in {
		t.Errorf("tiled: %+v", got)
	}
}

// The picture that flies out of the dock lands on the front machine's picture where it is in a
// maximised window: the classic screen with its border, or a machine's whole frame, in the
// middle of a window larger than the display's own.
func TestFlyLandsOnThePictureOfAMaximisedWindow(t *testing.T) {
	d := testDeck(t, 2)
	dm := d.dm
	dm.screen.multiplier = 3
	dm.reservedTopHeight = 32
	dm.currentWidth, dm.currentHeight = 1024, 768+32
	defer func(f func() (int, int)) { windowDrawable = f }(windowDrawable)
	windowDrawable = func() (int, int) { return 1920, 1080 }

	// The window is as the display has it: the canvas's place is the window's.
	dm.windowKept = false
	plain := d.screenRect(1)
	if plain.X < 0 || plain.Width <= 0 {
		t.Fatalf("no place: %+v", plain)
	}
	// Maximised: the display's picture is in the middle of the window.
	dm.windowKept = true
	p := centrePicture(1920, 1080, dm.frameTop(), dm.currentWidth, dm.currentHeight-dm.reservedTopHeight)
	want := placeInPicture(plain, p, float32(dm.reservedTopHeight))
	got := d.screenRect(1)
	if got != want {
		t.Errorf("in a maximised window the picture lands on %+v, want %+v", got, want)
	}
	// By hand: the 1024x768 picture is 448 units from the left of a window 1920 wide, and 140 below
	// the bar (1048 - 768, halved), the bar being 32 tall; the canvas has the bar's band above it.
	if got.X != plain.X+448 || got.Y != plain.Y-32+32+140 || got.Width != plain.Width || got.Height != plain.Height {
		t.Errorf("by hand: %+v from %+v", got, plain)
	}
	// The picture is in the middle of the window, so the place is as far from the left as from the
	// right, whatever the border: the mirror of its own margins.
	if l, r := got.X, 1920-(got.X+got.Width); abs32(l-r) > 1 && dm.borderView().Shown {
		t.Errorf("not centred: %v from the left, %v from the right", l, r)
	}
}
