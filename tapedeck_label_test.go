// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "testing"

// The title goes on the cassette's label whenever the smallest text fits it, even when the
// text of the rest of the deck is larger; below that it goes in a row of its own.
func TestDeckLabelUsesTheSmallestText(t *testing.T) {
	band := deckRect{0, 2, 960, 68} // the border's band at a zoom of 3
	if l := layoutTapeDeckMin(band, 16, 8); !l.LabelOnCassette || !l.Name.empty() {
		t.Errorf("an 8 unit title does not go on the label of a 68 unit band: %+v", l.Label)
	}
	if l := layoutTapeDeck(band, 16); l.LabelOnCassette {
		t.Error("16 unit text claims to fit a label 9 units high")
	}
	if l := layoutTapeDeckMin(deckRect{0, 2, 640, 44}, 8, 8); l.LabelOnCassette || l.Name.empty() {
		t.Errorf("at zoom 2 the label is claimed to hold 8 unit text, or the name has no row (%v)", l.Name)
	}
	// Written there as large as it fits: the label is at least as tall as the smallest text.
	for _, h := range []int{40, 56, 68, 96, 128} {
		l := layoutTapeDeckMin(deckRect{0, 0, 1000, h}, 16, 8)
		if l.LabelOnCassette && l.Label.H < 8 {
			t.Errorf("height %d: label %d high claimed to hold 8 unit text", h, l.Label.H)
		}
	}
}
