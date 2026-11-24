// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "testing"

func TestTapeCassetteSizeWords(t *testing.T) {
	for i, w := range []string{"small", "big", "hidden"} {
		s := tapeCassetteSize(i)
		if s.String() != w || parseTapeCassetteSize(w, cassetteHidden) != s {
			t.Errorf("size %d: %q / %v", i, s.String(), parseTapeCassetteSize(w, cassetteHidden))
		}
	}
	if parseTapeCassetteSize("huge", cassetteBig) != cassetteBig {
		t.Error("an unknown word did not give the default")
	}
	if tapeCassetteSize(7).String() != "small" || tapeCassetteSize(-1).String() != "small" {
		t.Error("an out-of-range size does not read as small")
	}
}

func checkDeckParts(t *testing.T, what string, area deckRect, l tapeDeckLayout) {
	t.Helper()
	if !inside(area, l.Panel) {
		t.Errorf("%s: panel %v outside the area %v", what, l.Panel, area)
	}
	parts := map[string]deckRect{"cassette": l.Cassette, "bar": l.Bar, "time": l.Time, "name": l.Name}
	for n, r := range parts {
		if !r.empty() && !inside(l.Panel, r) {
			t.Errorf("%s: %s %v outside the panel %v", what, n, r, l.Panel)
		}
	}
	all := []deckRect{l.Cassette, l.Bar, l.Time, l.Name}
	all = append(all, l.Buttons[:]...)
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			if !all[i].empty() && !all[j].empty() && overlap(all[i], all[j]) {
				t.Errorf("%s: parts %d and %d overlap: %v %v", what, i, j, all[i], all[j])
			}
		}
	}
	for i, b := range l.Buttons {
		if b.empty() || !inside(l.Panel, b) {
			t.Errorf("%s: button %d %v not in the panel", what, i, b)
		}
		if i > 0 && b.X < l.Buttons[i-1].X+l.Buttons[i-1].W {
			t.Errorf("%s: buttons %d and %d are out of order", what, i-1, i)
		}
	}
}

func TestBigCassetteIsUnderTheButtons(t *testing.T) {
	for _, a := range []deckRect{{0, 8, 960, 460}, {0, 8, 640, 318}, {0, 8, 320, 158}, {0, 0, 1280, 300}, {5, 5, 200, 400}} {
		for _, bottom := range []bool{false, true} {
			l := layoutTapeDeckSized(a, 16, 8, cassetteBig, bottom)
			if l.Panel.empty() || l.Cassette.empty() {
				t.Errorf("%v: no big deck", a)
				continue
			}
			checkDeckParts(t, "big", a, l)
			// Under the buttons and the bar, and the title is on its label.
			if l.Cassette.Y < l.Bar.Y+l.Bar.H || l.Bar.Y < l.Buttons[0].Y+l.Buttons[0].H {
				t.Errorf("%v: the cassette is not under the bar, or the bar under the buttons: %+v", a, l)
			}
			if !l.Name.empty() {
				t.Errorf("%v: a big cassette has a name row", a)
			}
			if bottom && l.Panel.Y+l.Panel.H != a.Y+a.H {
				t.Errorf("%v: the deck is not against the bottom of its area", a)
			}
			if !bottom && l.Panel.Y != a.Y {
				t.Errorf("%v: the deck is not against the top of its area", a)
			}
			if l.Label.H < 8 {
				continue
			}
			if !l.LabelOnCassette {
				t.Errorf("%v: label %v is claimed too small for 8 unit text", a, l.Label)
			}
		}
	}
	// It is bigger than the small cassette in the same area.
	a := deckRect{0, 8, 960, 460}
	if big, small := layoutTapeDeckSized(a, 16, 8, cassetteBig, false), layoutTapeDeckSized(a, 16, 8, cassetteSmall, false); big.Cassette.W <= small.Cassette.W {
		t.Errorf("big %d wide, small %d", big.Cassette.W, small.Cassette.W)
	}
	// And its label is large enough for the title to be several times the smallest text.
	if l := layoutTapeDeckSized(a, 16, 8, cassetteBig, false); l.Label.H < 3*8 {
		t.Errorf("the big label is %d high", l.Label.H)
	}
}

func TestHiddenCassetteLeavesTheButtons(t *testing.T) {
	for _, a := range []deckRect{{0, 2, 960, 68}, {0, 2, 640, 44}, {0, 8, 320, 56}, {0, 8, 960, 96}} {
		l := layoutTapeDeckSized(a, 16, 8, cassetteHidden, false)
		if l.Panel.empty() {
			t.Errorf("%v: no deck", a)
			continue
		}
		checkDeckParts(t, "hidden", a, l)
		if !l.Cassette.empty() || !l.Label.empty() {
			t.Errorf("%v: a hidden cassette has a cassette", a)
		}
		if l.Bar.empty() {
			t.Errorf("%v: no bar", a)
		}
	}
	// Given the height, the program's name has a row, above the buttons.
	l := layoutTapeDeckSized(deckRect{0, 0, 960, 96}, 16, 8, cassetteHidden, false)
	if l.Name.empty() || l.Name.Y+l.Name.H > l.Buttons[0].Y {
		t.Errorf("name row %v, buttons %v", l.Name, l.Buttons[0])
	}
	// Smaller than a small deck: it only holds what it shows.
	small := layoutTapeDeckSized(deckRect{0, 0, 960, 96}, 16, 8, cassetteSmall, false)
	if l.Panel.W >= small.Panel.W+1 && l.Panel.H > small.Panel.H {
		t.Errorf("hidden deck %v is not smaller than the small %v", l.Panel, small.Panel)
	}
}

func TestSizedLayoutDegenerate(t *testing.T) {
	for _, s := range []tapeCassetteSize{cassetteSmall, cassetteBig, cassetteHidden} {
		for _, a := range []deckRect{{}, {0, 0, 0, 80}, {0, 0, 80, 0}, {0, 0, 6, 6}, {0, 0, -5, 40}} {
			l := layoutTapeDeckSized(a, 0, 0, s, true)
			if !l.Panel.empty() && !inside(a, l.Panel) {
				t.Errorf("%v %v: panel %v outside", s, a, l.Panel)
			}
		}
	}
}
