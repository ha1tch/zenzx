// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "testing"

func TestFitBarScale(t *testing.T) {
	// Labels that take 400 pixels at X1, 800 at X2 and 1200 at X3. The labels may take 60%
	// of the window: X1 from 667 wide, X2 from 1334, X3 from 2000.
	width := func(k int) int { return 400 * k }
	// Strips of 24, 32 and 48 pixels, which the window's height may give an eighth of: X2 from a
	// window 256 high and X3 from 384.
	strip := func(k int) int { return [4]int{0, 24, 32, 48}[k] }
	const tall = 2000
	for _, c := range []struct{ screenW, want int }{
		{0, 1},    // no window to speak of: the bar is never smaller than it was
		{300, 1},  // not even X1 fits: still X1
		{666, 1},  // X1 does not quite fit, and is what there is
		{667, 1},  // X1 fits
		{1333, 1}, // X2 does not fit
		{1334, 2}, // X2 fits (800 is under 60% of 1334)
		{1999, 2}, // X3 does not fit
		{2000, 3}, // X3 takes exactly 60%: it fits
		{5000, 3}, // the largest there is
	} {
		if got := fitBarScale(c.screenW, tall, width, strip); got != c.want {
			t.Errorf("a window %d wide: scale %d, want %d", c.screenW, got, c.want)
		}
	}
	// A window wide enough for X3 and of every height: the strip's share of it decides too.
	for _, c := range []struct{ screenH, want int }{
		{0, 1},   // no window
		{100, 1}, // short: not even X2's strip (32) is an eighth of it
		{255, 1}, // X2's strip is a little over an eighth
		{256, 2}, // X2's strip is exactly an eighth: it fits
		{383, 2}, // X3's strip (48) is a little over an eighth
		{384, 3}, // X3's strip is exactly an eighth: it fits
		{900, 3},
	} {
		if got := fitBarScale(5000, c.screenH, width, strip); got != c.want {
			t.Errorf("a window %d high: scale %d, want %d", c.screenH, got, c.want)
		}
	}
}

func TestBarPad(t *testing.T) {
	for _, c := range []struct{ line, pad int }{{0, 4}, {8, 4}, {9, 4}, {16, 8}, {24, 12}, {32, 16}} {
		if got := barPadFor(c.line); got != c.pad {
			t.Errorf("barPadFor(%d) = %d, want %d", c.line, got, c.pad)
		}
	}
}
