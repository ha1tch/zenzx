// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "testing"

func TestDeckCornerButtonsFitWhereTheyAreAndNowhereElse(t *testing.T) {
	present := map[tapeCassetteSize]bool{}
	for _, size := range []tapeCassetteSize{cassetteSmall, cassetteBig, cassetteHidden} {
		for _, bottom := range []bool{false, true} {
			for _, w := range []int{1000, 640, 320, 200} {
				for _, h := range []int{60, 100, 200, 460} {
					for _, lineH := range []int{8, 16} {
						area := deckRect{10, 20, w, h}
						l := layoutTapeDeckSized(area, lineH, lineH/2, size, bottom)
						if l.Panel.empty() {
							continue
						}
						where := func() string { return size.String() + " " + itoaDeck(w) + "x" + itoaDeck(h) }
						if l.Minimise.empty() != l.Close.empty() {
							t.Errorf("%s: one corner button without the other: %+v %+v", where(), l.Minimise, l.Close)
						}
						if l.Minimise.empty() {
							continue
						}
						present[size] = true
						for name, r := range map[string]deckRect{"Minimise": l.Minimise, "Close": l.Close} {
							if !inside(l.Panel, r) {
								t.Errorf("%s: %s %+v is not in the panel %+v", where(), name, r, l.Panel)
							}
							if r.W != r.H {
								t.Errorf("%s: %s %+v is not square", where(), name, r)
							}
							for i, b := range l.Buttons {
								if overlap(r, b) {
									t.Errorf("%s: %s overlaps button %d", where(), name, i)
								}
							}
							for pn, p := range map[string]deckRect{"Time": l.Time, "Bar": l.Bar, "Cassette": l.Cassette, "Name": l.Name} {
								if !p.empty() && overlap(r, p) {
									t.Errorf("%s: %s %+v overlaps %s %+v", where(), name, r, pn, p)
								}
							}
						}
						if l.Minimise.X+l.Minimise.W > l.Close.X {
							t.Errorf("%s: Minimise is not left of Close", where())
						}
						if l.Time.empty() {
							t.Errorf("%s: corner buttons took all the room of the counter", where())
						}
					}
				}
			}
		}
	}
	for _, size := range []tapeCassetteSize{cassetteSmall, cassetteBig, cassetteHidden} {
		if !present[size] {
			t.Errorf("a %s deck never has corner buttons, at any size", size)
		}
	}
}

func itoaDeck(n int) string { return deckItoa(n) }

func TestDeckChipLayout(t *testing.T) {
	for _, bottom := range []bool{false, true} {
		for _, w := range []int{1000, 400, 160, 90, 40} {
			for _, h := range []int{24, 40, 56, 128} {
				for _, lineH := range []int{8, 16} {
					area := deckRect{10, 20, w, h}
					timeW := 12 * lineH / 2
					l := layoutDeckChip(area, lineH, timeW, bottom)
					if l.Panel.empty() {
						continue
					}
					if !inside(area, l.Panel) {
						t.Errorf("%dx%d line %d: the chip %+v is not in the area", w, h, lineH, l.Panel)
					}
					if l.Panel.X+l.Panel.W != area.X+area.W {
						t.Errorf("%dx%d: the chip is not at the right end", w, h)
					}
					if bottom && l.Panel.Y+l.Panel.H != area.Y+area.H || !bottom && l.Panel.Y != area.Y {
						t.Errorf("%dx%d bottom=%v: the chip is not against its edge: %+v", w, h, bottom, l.Panel)
					}
					parts := map[string]deckRect{"Cassette": l.Cassette, "Play": l.Play, "Time": l.Time, "Close": l.Close, "Bar": l.Bar}
					for n, r := range parts {
						if r.empty() {
							if n == "Time" {
								continue
							}
							t.Errorf("%dx%d: the chip has no %s", w, h, n)
							continue
						}
						if !inside(l.Panel, r) {
							t.Errorf("%dx%d: %s %+v is not in the chip %+v", w, h, n, r, l.Panel)
						}
						for n2, r2 := range parts {
							if n < n2 && !r2.empty() && overlap(r, r2) {
								t.Errorf("%dx%d: %s and %s overlap", w, h, n, n2)
							}
						}
					}
					if got, want := l.Cassette.W*cassetteH, l.Cassette.H*cassetteW; got-want > cassetteH || want-got > cassetteH {
						t.Errorf("%dx%d: the mini cassette %+v is out of proportion", w, h, l.Cassette)
					}
				}
			}
		}
	}
	// Room enough: the counter is there. Narrow: it gives way before the buttons do.
	if l := layoutDeckChip(deckRect{0, 0, 1000, 60}, 8, 90, false); l.Time.empty() {
		t.Error("a chip with room has no counter")
	}
	narrow := layoutDeckChip(deckRect{0, 0, 120, 60}, 16, 200, false)
	if narrow.Panel.empty() || !narrow.Time.empty() {
		t.Errorf("a narrow chip: panel %+v time %+v, want a chip with no counter", narrow.Panel, narrow.Time)
	}
	if !layoutDeckChip(deckRect{0, 0, 20, 60}, 16, 200, false).Panel.empty() {
		t.Error("a chip wider than the area is laid out")
	}
	if !layoutDeckChip(deckRect{0, 0, 1000, 8}, 8, 90, false).Panel.empty() {
		t.Error("a chip in an area too short for its buttons is laid out")
	}
}

func TestShiftedLayoutsMoveEveryPartAndNothingElse(t *testing.T) {
	l := layoutTapeDeckSized(deckRect{10, 20, 1000, 100}, 16, 8, cassetteSmall, false)
	s := l.shifted(-37)
	pairs := map[string][2]deckRect{
		"Panel": {l.Panel, s.Panel}, "Cassette": {l.Cassette, s.Cassette}, "Label": {l.Label, s.Label},
		"Bar": {l.Bar, s.Bar}, "Time": {l.Time, s.Time}, "Name": {l.Name, s.Name},
		"Minimise": {l.Minimise, s.Minimise}, "Close": {l.Close, s.Close},
	}
	for i := range l.Buttons {
		pairs["button "+deckItoa(i)] = [2]deckRect{l.Buttons[i], s.Buttons[i]}
	}
	for n, p := range pairs {
		if p[0].empty() {
			if !p[1].empty() {
				t.Errorf("%s: an empty part became %+v", n, p[1])
			}
			continue
		}
		if p[1] != (deckRect{p[0].X, p[0].Y - 37, p[0].W, p[0].H}) {
			t.Errorf("%s: %+v became %+v", n, p[0], p[1])
		}
	}
	c := layoutDeckChip(deckRect{0, 0, 1000, 60}, 8, 90, false)
	sc := c.shifted(25)
	if sc.Panel != (deckRect{c.Panel.X, c.Panel.Y + 25, c.Panel.W, c.Panel.H}) || sc.Close.Y != c.Close.Y+25 || sc.Bar.Y != c.Bar.Y+25 {
		t.Errorf("the chip did not move as a whole: %+v -> %+v", c, sc)
	}
}
