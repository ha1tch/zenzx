package zenzx

import (
	"math"
	"testing"
)

// The deck comes in and goes out of its edge, and moves from one form to another, eased, in the
// dock's time.

func TestDeckEaseIsTheDocksEasing(t *testing.T) {
	if deckEase(0) != 0 || deckEase(1) != 1 || deckEase(-3) != 0 || deckEase(7) != 1 {
		t.Errorf("the ends: %v %v, outside %v %v", deckEase(0), deckEase(1), deckEase(-3), deckEase(7))
	}
	if math.Abs(deckEase(0.5)-0.5) > 1e-12 {
		t.Errorf("the middle is %v", deckEase(0.5))
	}
	prev := 0.0
	for i := 1; i <= 100; i++ {
		x := deckEase(float64(i) / 100)
		if x < prev {
			t.Fatalf("not monotonic at %d: %v after %v", i, x, prev)
		}
		prev = x
	}
	// In and out: slow at both ends, fast in the middle.
	if deckEase(0.1) >= 0.1 || deckEase(0.9) <= 0.9 {
		t.Errorf("not eased: %v at a tenth, %v at nine tenths", deckEase(0.1), deckEase(0.9))
	}
	// The same easing as the dock's.
	for _, x := range []float64{0.1, 0.3, 0.5, 0.8} {
		if got, want := deckEase(x), float64(easeInOutCubicRef(float32(x))); math.Abs(got-want) > 1e-5 {
			t.Errorf("at %v: %v, the dock's is %v", x, got, want)
		}
	}
}

// easeInOutCubicRef is the dock's easing, as menubar_gui.go has it, for the test to hold the
// deck's to.
func easeInOutCubicRef(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	f := -2*t + 2
	return 1 - f*f*f/2
}

func TestDeckSlidesInWhenItAppears(t *testing.T) {
	d, tp := &tapeDeck{place: tapeDeckTop}, deckTape(make([]int, 100), "G")
	last := 2.0
	var total float64
	for i := 0; i < 100; i++ {
		v := d.Update(tp, 0.02)
		if !v.Visible {
			t.Fatalf("frame %d: not visible", i)
		}
		if v.Slide > last {
			t.Fatalf("frame %d: the slide went back out, %v after %v", i, v.Slide, last)
		}
		if i == 0 && (v.Slide >= 1 || v.Slide < 0.8) {
			t.Errorf("the first frame is at slide %v, want just in from the edge", v.Slide)
		}
		if last = v.Slide; v.Slide == 0 {
			total = float64(i+1) * 0.02
			break
		}
	}
	if math.Abs(total-tapeDeckSlideTime) > 0.03 {
		t.Errorf("the deck took %.2f s to come in, want %.2f", total, tapeDeckSlideTime)
	}
}

func TestDeckClosedSlidesOutAndComesBackWhenExpanded(t *testing.T) {
	d, tp := modeFixture()
	run(d, tp, 0.5)
	d.Hide()
	last, steps := 0.0, 0
	for ; steps < 100; steps++ {
		v := d.Update(tp, 0.02)
		if !v.Visible {
			break
		}
		if v.Slide < last {
			t.Fatalf("step %d: back in, %v after %v", steps, v.Slide, last)
		}
		last = v.Slide
	}
	if math.Abs(float64(steps+1)*0.02-tapeDeckSlideTime) > 0.03 {
		t.Errorf("the deck took %.2f s to go, want %.2f", float64(steps+1)*0.02, tapeDeckSlideTime)
	}
	if last <= 0 || last >= 1 {
		t.Errorf("the last frame shown was at slide %v, want it between 0 and 1", last)
	}
	// Brought back, it is shown at once and comes in again, from the edge.
	d.Expand()
	if v := d.Update(tp, 0.02); !v.Visible || v.Slide < 0.8 || v.Slide >= 1 {
		t.Errorf("expanded: visible=%v slide=%v", v.Visible, v.Slide)
	}
	for i := 0; i < 30; i++ {
		d.Update(tp, 0.02)
	}
	if v := d.Update(tp, 0.02); v.Slide != 0 || v.Mode != deckFull {
		t.Errorf("expanded and settled: slide=%v mode=%v", v.Slide, v.Mode)
	}
}

// A chip that is closed slides out as a chip, not as the full deck it would be if its mode were
// all there is.
func TestDeckSlidingOutFromTheChipKeepsItsForm(t *testing.T) {
	d, tp := modeFixture()
	d.Minimise()
	run(d, tp, 0.5)
	d.Hide()
	v := d.Update(tp, 0.05)
	if v.Mode != deckAway || v.Form != deckChip || !v.Visible {
		t.Errorf("sliding out: mode %v form %v visible %v, want away, the chip's form, shown", v.Mode, v.Form, v.Visible)
	}
	d2, tp2 := modeFixture()
	run(d2, tp2, 0.5)
	d2.Hide()
	if v := d2.Update(tp2, 0.05); v.Form != deckFull {
		t.Errorf("a full deck sliding out has form %v", v.Form)
	}
}

// ---------------------------------------------------------------------------------------------
// The parts moving
// ---------------------------------------------------------------------------------------------

var morphArea = deckRect{0, 2, 960, 68}

func morphFull(size tapeCassetteSize, bottom bool) tapeDeckLayout {
	return layoutTapeDeckSized(morphArea, 16, 8, size, bottom)
}
func morphChip(bottom bool) tapeDeckLayout {
	return chipAsLayout(layoutDeckChip(morphArea, 16, 100, bottom))
}

func within(inner, outer deckRect, slack int) bool {
	return inner.X >= outer.X-slack && inner.Y >= outer.Y-slack &&
		inner.X+inner.W <= outer.X+outer.W+slack && inner.Y+inner.H <= outer.Y+outer.H+slack
}

func parts(l tapeDeckLayout) map[string]deckRect {
	m := map[string]deckRect{"cassette": l.Cassette, "label": l.Label, "bar": l.Bar, "time": l.Time,
		"name": l.Name, "minimise": l.Minimise, "close": l.Close}
	for i, b := range l.Buttons {
		m[tapeButtonNames[i]] = b
	}
	return m
}

func TestLerpLayoutStartsAtOneAndEndsAtTheOther(t *testing.T) {
	for name, pair := range map[string][2]tapeDeckLayout{
		"small to big":    {morphFull(cassetteSmall, false), morphFull(cassetteBig, false)},
		"full to chip":    {morphFull(cassetteSmall, false), morphChip(false)},
		"chip to full":    {morphChip(false), morphFull(cassetteSmall, false)},
		"top to bottom":   {morphFull(cassetteSmall, false), morphFull(cassetteSmall, true)},
		"hidden to small": {morphFull(cassetteHidden, false), morphFull(cassetteSmall, false)},
	} {
		a, b := pair[0], pair[1]
		if a.Panel.empty() || b.Panel.empty() {
			t.Fatalf("%s: a layout is empty: %+v / %+v", name, a.Panel, b.Panel)
		}
		start, end := lerpLayout(a, b, 0), lerpLayout(a, b, 1)
		if start.Panel != a.Panel || end.Panel != b.Panel {
			t.Errorf("%s: the panel starts at %+v and ends at %+v, want %+v and %+v", name, start.Panel, end.Panel, a.Panel, b.Panel)
		}
		for part, ra := range parts(a) {
			if got := parts(start)[part]; ra.empty() != got.empty() || (!ra.empty() && !within(got, ra, 1)) || (!ra.empty() && !within(ra, got, 1)) {
				t.Errorf("%s: %s starts at %+v, want %+v", name, part, got, ra)
			}
		}
		for part, rb := range parts(b) {
			if got := parts(end)[part]; rb.empty() != got.empty() || (!rb.empty() && (!within(got, rb, 1) || !within(rb, got, 1))) {
				t.Errorf("%s: %s ends at %+v, want %+v", name, part, got, rb)
			}
		}
	}
}

// Whatever the moment, every part of the deck is inside the deck's panel: the buttons that the
// chip has not do not float in the middle of the window while the panel shrinks past them.
func TestLerpLayoutKeepsEveryPartInsideThePanel(t *testing.T) {
	forms := map[string]tapeDeckLayout{
		"small": morphFull(cassetteSmall, false), "big": morphFull(cassetteBig, false),
		"hidden": morphFull(cassetteHidden, false), "chip": morphChip(false), "chip bottom": morphChip(true),
		"small bottom": morphFull(cassetteSmall, true),
	}
	for an, a := range forms {
		for bn, b := range forms {
			for i := 0; i <= 20; i++ {
				l := lerpLayout(a, b, float64(i)/20)
				for part, r := range parts(l) {
					if !r.empty() && !within(r, l.Panel, 1) {
						t.Errorf("%s to %s at %d/20: %s %+v is outside the panel %+v", an, bn, i, part, r, l.Panel)
					}
				}
			}
		}
	}
}

func TestLerpLayoutPartsGrowAndShrinkRatherThanAppear(t *testing.T) {
	full, chip := morphFull(cassetteSmall, false), morphChip(false)
	if full.Buttons[tapeRewind].empty() || !chip.Buttons[tapeRewind].empty() {
		t.Fatal("the test's forms are not as it needs: the full deck has Rewind and the chip has not")
	}
	prev := full.Buttons[tapeRewind].W + 1
	for i := 0; i <= 10; i++ {
		got := lerpLayout(full, chip, float64(i)/10).Buttons[tapeRewind]
		if got.W > prev {
			t.Errorf("at %d/10 the Rewind button is wider (%d) than before (%d): not shrinking", i, got.W, prev)
		}
		prev = got.W
		if i == 10 && !got.empty() {
			t.Errorf("Rewind is still there at the end: %+v", got)
		}
	}
}

func TestLerpLayoutOfAnEmptyOneIsTheOther(t *testing.T) {
	l := morphFull(cassetteSmall, false)
	if got := lerpLayout(tapeDeckLayout{}, l, 0.4); got.Panel != l.Panel {
		t.Errorf("from nothing: %+v", got.Panel)
	}
	if got := lerpLayout(l, tapeDeckLayout{}, 0.4); got.Panel != l.Panel {
		t.Errorf("to nothing: %+v", got.Panel)
	}
}

func TestChipAndLayoutRoundTrip(t *testing.T) {
	c := layoutDeckChip(morphArea, 16, 100, false)
	if got := layoutAsChip(chipAsLayout(c)); got != c {
		t.Errorf("round trip: %+v, want %+v", got, c)
	}
	l := chipAsLayout(c)
	if l.Buttons[tapePlay] != c.Play || !l.Buttons[tapeStop].empty() || !l.Minimise.empty() {
		t.Errorf("the chip as a layout: %+v", l)
	}
}

// ---------------------------------------------------------------------------------------------
// The state of a move
// ---------------------------------------------------------------------------------------------

func TestMorphDoesNotMoveWhileTheFormIsTheSame(t *testing.T) {
	var m deckMorph
	key := deckKey{form: deckFull, size: cassetteSmall}
	l := morphFull(cassetteSmall, false)
	for i := 0; i < 50; i++ {
		geo, chip, moving := m.step(key, l, false, 0.016)
		if moving || chip || geo != l {
			t.Fatalf("frame %d: moving=%v chip=%v", i, moving, chip)
		}
	}
	// The same form at another place, because the window was resized, follows it with no move.
	moved := morphFull(cassetteSmall, false)
	moved.Panel.W -= 100
	if geo, _, moving := m.step(key, moved, false, 0.016); moving || geo != moved {
		t.Error("a resize is a move")
	}
}

func TestMorphMovesForTheDocksTimeAndEndsOnTheTarget(t *testing.T) {
	var m deckMorph
	small, big := morphFull(cassetteSmall, false), morphFull(cassetteBig, false)
	m.step(deckKey{form: deckFull, size: cassetteSmall}, small, false, 0.016)
	big2 := deckKey{form: deckFull, size: cassetteBig}
	frames, prevH := 0, small.Panel.H
	for {
		geo, _, moving := m.step(big2, big, false, 0.01)
		frames++
		if !moving {
			if geo != big {
				t.Errorf("it ends at %+v, want %+v", geo.Panel, big.Panel)
			}
			break
		}
		if geo.Panel.H < prevH {
			t.Fatalf("frame %d: the panel got shorter, %d after %d", frames, geo.Panel.H, prevH)
		}
		prevH = geo.Panel.H
		if frames > 100 {
			t.Fatal("a move that never ends")
		}
	}
	if secs := float64(frames) * 0.01; math.Abs(secs-deckMorphTime) > 0.02 {
		t.Errorf("a move took %.2f s, want %.2f", secs, deckMorphTime)
	}
}

func TestMorphIsEasedAtBothEnds(t *testing.T) {
	var m deckMorph
	a, b := morphFull(cassetteSmall, false), morphFull(cassetteSmall, true)
	m.step(deckKey{form: deckFull, size: cassetteSmall}, a, false, 0.016)
	span := float64(b.Panel.Y - a.Panel.Y)
	var ys []float64
	for {
		geo, _, moving := m.step(deckKey{form: deckFull, size: cassetteSmall, bottom: true}, b, false, 0.01)
		if !moving {
			break
		}
		ys = append(ys, float64(geo.Panel.Y-a.Panel.Y)/span)
	}
	if len(ys) < 10 {
		t.Fatalf("only %d frames of movement", len(ys))
	}
	first, last := ys[0]-0, 1-ys[len(ys)-1]
	mid := ys[len(ys)/2] - ys[len(ys)/2-1]
	if first >= mid || last >= mid {
		t.Errorf("not eased: the first step is %.3f and the last %.3f of the way, the middle one %.3f", first, last, mid)
	}
}

func TestMorphInterruptedContinuesFromWhereItHadGot(t *testing.T) {
	var m deckMorph
	small, big := morphFull(cassetteSmall, false), morphFull(cassetteBig, false)
	m.step(deckKey{form: deckFull, size: cassetteSmall}, small, false, 0.016)
	var mid tapeDeckLayout
	for i := 0; i < 11; i++ {
		mid, _, _ = m.step(deckKey{form: deckFull, size: cassetteBig}, big, false, 0.01)
	}
	// Turned back: the first frame is next to where it had got to, not at the start.
	geo, _, moving := m.step(deckKey{form: deckFull, size: cassetteSmall}, small, false, 0.001)
	if !moving || math.Abs(float64(geo.Panel.H-mid.Panel.H)) > 3 {
		t.Errorf("turned back at %+v from %+v: moving=%v", geo.Panel, mid.Panel, moving)
	}
}

func TestMorphFromTheChipToTheChipStaysTheChip(t *testing.T) {
	var m deckMorph
	top, bottom := morphChip(false), morphChip(true)
	m.step(deckKey{form: deckChip}, top, true, 0.016)
	if _, chip, moving := m.step(deckKey{form: deckChip, bottom: true}, bottom, true, 0.01); !moving || !chip {
		t.Errorf("the chip to the other edge: moving=%v chip=%v", moving, chip)
	}
	// To a full deck it is drawn as one, and back.
	var n deckMorph
	n.step(deckKey{form: deckChip}, top, true, 0.016)
	if _, chip, moving := n.step(deckKey{form: deckFull, size: cassetteSmall}, morphFull(cassetteSmall, false), false, 0.01); !moving || chip {
		t.Errorf("the chip to a full deck: moving=%v chip=%v", moving, chip)
	}
}

// A deck let go after being dragged moves from where it was let go, to its place, even when its
// place is the one it was taken from.
func TestMorphDropMovesFromWhereItWasLetGo(t *testing.T) {
	var m deckMorph
	home := morphFull(cassetteSmall, false)
	key := deckKey{form: deckFull, size: cassetteSmall}
	m.step(key, home, false, 0.016)
	dragged := home.shifted(300)
	m.drop(dragged, false)
	geo, _, moving := m.step(key, home, false, 0.01)
	if !moving {
		t.Fatal("a dropped deck does not move")
	}
	if d := geo.Panel.Y - home.Panel.Y; d < 250 || d > 300 {
		t.Errorf("the first frame is %d below home, want just short of the 300 it was let go at", d)
	}
	// And it gets home.
	for i := 0; i < 100; i++ {
		if geo, _, moving = m.step(key, home, false, 0.01); !moving {
			break
		}
	}
	if moving || geo != home {
		t.Errorf("it did not get home: moving=%v at %+v", moving, geo.Panel)
	}
}

func TestMorphResetForgetsTheLastForm(t *testing.T) {
	var m deckMorph
	l := morphFull(cassetteSmall, false)
	m.step(deckKey{form: deckFull, size: cassetteSmall}, l, false, 0.016)
	m.reset()
	if _, _, moving := m.step(deckKey{form: deckChip}, morphChip(false), true, 0.016); moving {
		t.Error("a deck that came back from nothing moves from the form it had before")
	}
}
