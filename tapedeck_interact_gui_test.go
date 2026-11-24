//go:build !headless

package zenzx

import "testing"

// deckRig drives the deck's pointer handling without a window: the deck, its tape, and a strip of
// the border it is laid out in, one frame at a time.
type deckRig struct {
	t      *testing.T
	u      *tapeDeckUI
	tp     *Tape
	place  tapeDeckPlace
	size   tapeCassetteSize
	area   deckRect
	screen float32
}

func newDeckRig(t *testing.T) *deckRig {
	pulses := make([]int, 2000)
	for i := range pulses {
		pulses[i] = tapeTicksPerSecond / 100
	}
	r := &deckRig{t: t, u: newTapeDeckUI(), tp: deckTape(pulses, "Game"),
		place: tapeDeckTop, size: cassetteSmall, area: deckRect{0, 2, 960, 68}, screen: 768}
	r.u.scale = 2
	r.layout(0.01)
	return r
}

// layout runs the deck for dt seconds and lays out what it shows.
func (r *deckRig) layout(dt float64) {
	r.u.deck.place = r.place
	r.u.deck.hot = r.u.shown.Contains(0, 0) // not used by the rig's own frames
	r.u.view = r.u.deck.Update(r.tp, dt)
	r.u.layout, r.u.chip, r.u.shown = tapeDeckLayout{}, deckChipLayout{}, deckRect{}
	r.u.hover, r.u.hit = -1, hitNone
	bottom := r.place == tapeDeckBottom
	if !r.u.view.Visible {
		return
	}
	if r.u.chipStyle = r.u.view.Form == deckChip; r.u.chipStyle {
		r.u.chip = layoutDeckChip(r.area, 16, 100, bottom)
		r.u.shown = r.u.chip.Panel
	} else {
		r.u.layout = layoutTapeDeckSized(r.area, 16, 8, r.size, bottom)
		r.u.shown = r.u.layout.Panel
	}
}

// frame is one frame with the pointer p.
func (r *deckRig) frame(p deckPointer, owned bool) (changed bool) {
	r.layout(0.016)
	p.ScreenH = r.screen
	r.u.carry(p.Y, int(r.screen))
	return r.u.interact(r.tp, p, &r.place, &r.size, owned, 0.016)
}

func (r *deckRig) click(x, y float32) {
	r.frame(deckPointer{X: x, Y: y}, false)
	r.frame(deckPointer{X: x, Y: y, Pressed: true, Down: true}, false)
	r.frame(deckPointer{X: x, Y: y, Released: true}, false)
}

func (r *deckRig) at(rc deckRect) (float32, float32) {
	x, y := deckMid(rc)
	return float32(x), float32(y)
}

func TestDeckMinimiseAndCloseButtonsAndChipClicks(t *testing.T) {
	r := newDeckRig(t)
	if r.u.layout.Minimise.empty() || r.u.layout.Close.empty() {
		t.Fatalf("the rig's deck has no corner buttons: %+v", r.u.layout)
	}
	r.click(r.at(r.u.layout.Minimise))
	r.layout(0.016)
	if r.u.view.Mode != deckChip || r.u.chip.Panel.empty() {
		t.Fatalf("Minimise left the deck mode %v", r.u.view.Mode)
	}
	// A click on the chip's body makes the deck full again.
	r.click(float32(r.u.chip.Time.X+2), float32(r.u.chip.Time.Y+2))
	r.layout(0.016)
	if r.u.view.Mode != deckFull {
		t.Fatalf("a click on the chip left the deck mode %v", r.u.view.Mode)
	}
	// Close sends it away: it slides out of its edge, and then nothing is drawn.
	r.click(r.at(r.u.layout.Close))
	r.layout(0.016)
	if !r.u.view.Visible || r.u.view.Slide <= 0 {
		t.Errorf("Close did not start the deck sliding out: visible=%v slide=%v", r.u.view.Visible, r.u.view.Slide)
	}
	for i := 0; i < 30; i++ {
		r.layout(0.016)
	}
	if r.u.view.Visible || !r.u.shown.empty() {
		t.Errorf("Close left the deck visible=%v shown=%+v", r.u.view.Visible, r.u.shown)
	}
	// The chip's own close does the same.
	r = newDeckRig(t)
	r.u.deck.Minimise()
	r.layout(0.016)
	r.click(r.at(r.u.chip.Close))
	for i := 0; i < 30; i++ {
		r.layout(0.016)
	}
	if r.u.view.Visible {
		t.Error("the chip's close left the deck visible")
	}
}

func TestDeckChipPlayButtonPlaysAndPausesWithoutOpeningTheDeck(t *testing.T) {
	r := newDeckRig(t)
	r.u.deck.Minimise()
	r.layout(0.016)
	r.click(r.at(r.u.chip.Play))
	if !r.tp.st.Playing {
		t.Fatal("the chip's play did not start the tape")
	}
	r.layout(0.016)
	if r.u.view.Mode != deckChip {
		t.Errorf("the chip opened when the tape started: mode %v", r.u.view.Mode)
	}
	r.click(r.at(r.u.chip.Play))
	if r.tp.st.Playing || !r.tp.st.Paused {
		t.Errorf("the chip's button on a playing tape: playing %v paused %v", r.tp.st.Playing, r.tp.st.Paused)
	}
}

func TestDeckButtonsStillWork(t *testing.T) {
	r := newDeckRig(t)
	r.click(r.at(r.u.layout.Buttons[tapePlay]))
	if !r.tp.st.Playing {
		t.Error("the Play button does not play")
	}
	r.click(r.at(r.u.layout.Buttons[tapeStop]))
	if r.tp.st.Playing {
		t.Error("the Stop button does not stop")
	}
}

func TestDeckOwnedPointerDoesNothing(t *testing.T) {
	r := newDeckRig(t)
	x, y := r.at(r.u.layout.Minimise)
	r.frame(deckPointer{X: x, Y: y}, true)
	r.frame(deckPointer{X: x, Y: y, Pressed: true, Down: true}, true)
	r.frame(deckPointer{X: x, Y: y, Released: true}, true)
	r.layout(0.016)
	if r.u.view.Mode != deckFull {
		t.Errorf("a click while something else had the pointer made the deck mode %v", r.u.view.Mode)
	}
	if r.frame(deckPointer{X: x, Y: y, Wheel: 1}, true) {
		t.Error("the wheel changed the deck while something else had the pointer")
	}
}

// A drag carries the deck with the pointer; let go, it goes to the edge its middle is nearer, and
// the place is only changed then.
func TestDeckDragCarriesItAndLetGoItGoesToTheNearerEdge(t *testing.T) {
	for _, chip := range []bool{false, true} {
		r := newDeckRig(t)
		if chip {
			r.u.deck.Minimise()
			r.layout(0.016)
		}
		body := r.u.layout.Panel
		if chip {
			body = r.u.chip.Panel
		}
		home := r.u.shown
		x, y := float32(body.X+2), float32(body.Y+body.H/2)
		r.frame(deckPointer{X: x, Y: y}, false)
		r.frame(deckPointer{X: x, Y: y, Pressed: true, Down: true}, false)
		// A small move is not a drag: the deck stays.
		if r.frame(deckPointer{X: x, Y: y + deckDragSlop - 2, Down: true}, false) || r.u.dragged || r.u.shown != home {
			t.Fatalf("chip=%v: a move of %d pixels is a drag", chip, deckDragSlop-2)
		}
		// Past the slop the press is a drag; the deck has not moved yet, as it is carried from where
		// the pointer was when it began.
		if r.frame(deckPointer{X: x, Y: y + 40, Down: true}, false) || !r.u.dragged {
			t.Fatalf("chip=%v: a move of 40 pixels is not a drag", chip)
		}
		// The pointer goes down the window: the deck goes with it, by as much, and the place is
		// still the top, however far.
		for _, to := range []float32{y + 140, r.screen / 2, r.screen - 100} {
			if r.frame(deckPointer{X: x, Y: to, Down: true}, false) || r.place != tapeDeckTop {
				t.Fatalf("chip=%v: the place changed during the drag, to %v", chip, r.place)
			}
			if want := home.Y + int(to-(y+40)); r.u.shown.Y != want {
				t.Errorf("chip=%v: pointer at %.0f, the deck is at %d, want %d", chip, to, r.u.shown.Y, want)
			}
		}
		// Let go in the lower half: the bottom, reported once, and the deck is to move from where it
		// was let go.
		letgo := r.u.shown
		if !r.frame(deckPointer{X: x, Y: r.screen - 100, Released: true}, false) || r.place != tapeDeckBottom {
			t.Errorf("chip=%v: let go in the lower half, the deck is at %v", chip, r.place)
		}
		if !r.u.morph.dropped || r.u.morph.dropFrom.Panel != letgo {
			t.Errorf("chip=%v: the drop is not set up to move from %+v: %+v", chip, letgo, r.u.morph.dropFrom.Panel)
		}
		if r.u.morph.dropIsChip != chip {
			t.Errorf("chip=%v: the drop says chip=%v", chip, r.u.morph.dropIsChip)
		}
		r.layout(0.016)
		if mode := map[bool]deckMode{false: deckFull, true: deckChip}[chip]; r.u.view.Mode != mode {
			t.Errorf("chip=%v: a drag changed the deck's mode to %v (a drag is not a click)", chip, r.u.view.Mode)
		}
	}
}

// Let go still in the half it came from, the deck goes back to its own edge: nothing changes
// but the deck springs back (a drop is set up), and it is not a click.
func TestDeckDragLetGoNearItsEdgeSpringsBack(t *testing.T) {
	r := newDeckRig(t)
	p := r.u.layout.Panel
	x, y := float32(p.X+2), float32(p.Y+p.H/2)
	r.frame(deckPointer{X: x, Y: y}, false)
	r.frame(deckPointer{X: x, Y: y, Pressed: true, Down: true}, false)
	r.frame(deckPointer{X: x, Y: y + 40, Down: true}, false)
	r.frame(deckPointer{X: x, Y: y + 120, Down: true}, false)
	if r.u.shown.Y <= r.area.Y {
		t.Fatalf("the deck is not carried: at %d", r.u.shown.Y)
	}
	if r.frame(deckPointer{X: x, Y: y + 120, Released: true}, false) || r.place != tapeDeckTop {
		t.Errorf("let go in the top half the place is %v", r.place)
	}
	if !r.u.morph.dropped {
		t.Error("a deck let go away from its edge is not set to move back to it")
	}
	r.layout(0.016)
	if r.u.dragDY != 0 && r.u.press != hitNone {
		t.Errorf("still carried after it was let go: %d, press %v", r.u.dragDY, r.u.press)
	}
}

// A deck is never carried out of the window, whatever the pointer does.
func TestDeckDragStaysInsideTheWindow(t *testing.T) {
	r := newDeckRig(t)
	p := r.u.layout.Panel
	x, y := float32(p.X+2), float32(p.Y+p.H/2)
	r.frame(deckPointer{X: x, Y: y}, false)
	r.frame(deckPointer{X: x, Y: y, Pressed: true, Down: true}, false)
	r.frame(deckPointer{X: x, Y: y + 40, Down: true}, false)
	for _, to := range []float32{r.screen * 3, -500, r.screen} {
		r.frame(deckPointer{X: x, Y: to, Down: true}, false)
		if s := r.u.shown; s.Y < 0 || s.Y+s.H > int(r.screen) {
			t.Errorf("pointer at %.0f: the deck is at %+v in a window %.0f high", to, s, r.screen)
		}
	}
}

// A press that goes on to be a drag and is let go over the chip is not a click on it.
func TestDeckChipDraggedALittleIsNotAClick(t *testing.T) {
	r := newDeckRig(t)
	r.u.deck.Minimise()
	r.layout(0.016)
	c := r.u.chip.Panel
	x, y := float32(c.X+c.W-3), float32(c.Y+2)
	r.frame(deckPointer{X: x, Y: y}, false)
	r.frame(deckPointer{X: x, Y: y, Pressed: true, Down: true}, false)
	y2 := y + deckDragSlop + 2
	if y2 >= float32(c.Y+c.H) {
		t.Fatalf("the rig's chip %+v is too short for the test", c)
	}
	r.frame(deckPointer{X: x, Y: y2, Down: true}, false)
	r.frame(deckPointer{X: x, Y: y2, Released: true}, false)
	r.layout(0.016)
	if r.u.view.Mode != deckChip {
		t.Errorf("a drag let go over the chip opened it: mode %v", r.u.view.Mode)
	}
}

func TestDeckWheelResizesAndMinimises(t *testing.T) {
	r := newDeckRig(t)
	x, y := r.at(r.u.layout.Panel)
	notch := func(dir float32) bool {
		// A notch, then enough quiet frames for the wheel's pause to pass.
		c := r.frame(deckPointer{X: x, Y: y, Wheel: dir}, false)
		for i := 0; i < 10; i++ {
			r.frame(deckPointer{X: x, Y: y}, false)
		}
		return c
	}
	r.size = cassetteSmall
	if !notch(1) || r.size != cassetteBig {
		t.Errorf("wheel up on a small deck: size %v", r.size)
	}
	if notch(1) || r.size != cassetteBig {
		t.Errorf("wheel up on a big deck changed something: size %v", r.size)
	}
	if !notch(-1) || r.size != cassetteSmall {
		t.Errorf("wheel down on a big deck: size %v", r.size)
	}
	if !notch(-1) || r.size != cassetteHidden {
		t.Errorf("wheel down on a small deck: size %v", r.size)
	}
	// Past the smallest it is the chip, and the saved size is left as it was.
	if notch(-1) {
		t.Error("the step to the chip says a setting changed")
	}
	r.layout(0.016)
	if r.u.view.Mode != deckChip || r.size != cassetteHidden {
		t.Errorf("wheel down on the smallest: mode %v size %v", r.u.view.Mode, r.size)
	}
	// On the chip, wheel down does nothing, and wheel up makes the deck full.
	x, y = r.at(r.u.chip.Panel)
	notch(-1)
	r.layout(0.016)
	if r.u.view.Mode != deckChip {
		t.Errorf("wheel down on the chip left mode %v", r.u.view.Mode)
	}
	notch(1)
	r.layout(0.016)
	if r.u.view.Mode != deckFull || r.size != cassetteHidden {
		t.Errorf("wheel up on the chip: mode %v size %v", r.u.view.Mode, r.size)
	}
}

func TestDeckWheelOffTheDeckDoesNothing(t *testing.T) {
	r := newDeckRig(t)
	for i := 0; i < 20; i++ {
		if r.frame(deckPointer{X: 5, Y: 500, Wheel: 1}, false) {
			t.Fatal("the wheel away from the deck changed it")
		}
	}
	if r.size != cassetteSmall || r.u.view.Mode != deckFull {
		t.Errorf("the wheel away from the deck: size %v mode %v", r.size, r.u.view.Mode)
	}
}

func TestDeckHandOverWhatAnswersAClick(t *testing.T) {
	r := newDeckRig(t)
	for name, c := range map[string]struct {
		x, y float32
		hand bool
	}{
		"Play button":  {x: float32(mid2x(r.u.layout.Buttons[tapePlay])), y: float32(mid2y(r.u.layout.Buttons[tapePlay])), hand: true},
		"Minimise":     {x: float32(mid2x(r.u.layout.Minimise)), y: float32(mid2y(r.u.layout.Minimise)), hand: true},
		"Close":        {x: float32(mid2x(r.u.layout.Close)), y: float32(mid2y(r.u.layout.Close)), hand: true},
		"the panel":    {x: float32(r.u.layout.Panel.X + 2), y: float32(r.u.layout.Panel.Y + 2), hand: false},
		"off the deck": {x: 5, y: 500, hand: false},
	} {
		r.frame(deckPointer{X: c.x, Y: c.y}, false)
		if r.u.wantHand != c.hand {
			t.Errorf("%s: hand %v, want %v", name, r.u.wantHand, c.hand)
		}
	}
	r.u.deck.Minimise()
	r.layout(0.016)
	for name, c := range map[string]struct {
		rc   deckRect
		hand bool
	}{"chip play": {r.u.chip.Play, true}, "chip close": {r.u.chip.Close, true}, "chip body": {deckRect{r.u.chip.Time.X, r.u.chip.Time.Y, 2, 2}, true}} {
		x, y := deckMid(c.rc)
		r.frame(deckPointer{X: float32(x), Y: float32(y)}, false)
		if r.u.wantHand != c.hand {
			t.Errorf("%s: hand %v, want %v", name, r.u.wantHand, c.hand)
		}
	}
}

func deckMid(r deckRect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }
func mid2x(r deckRect) int          { x, _ := deckMid(r); return x }
func mid2y(r deckRect) int          { _, y := deckMid(r); return y }

func TestDeckSlideOutGoesTowardsItsOwnEdgeAndClearsTheWindow(t *testing.T) {
	const screenH = 768
	top := deckRect{300, 4, 360, 64}
	bottom := deckRect{300, 700, 360, 64}
	if got := deckSlideOffset(top, false, screenH, 0); got != 0 {
		t.Errorf("at 0 the deck is already %d off", got)
	}
	prevT, prevB := 0, 0
	for i := 1; i <= 10; i++ {
		s := float32(i) / 10
		dt, db := deckSlideOffset(top, false, screenH, s), deckSlideOffset(bottom, true, screenH, s)
		if dt > prevT || db < prevB {
			t.Errorf("slide %.1f: top %d (was %d), bottom %d (was %d): not moving to its own edge", s, dt, prevT, db, prevB)
		}
		prevT, prevB = dt, db
	}
	if top.Y+top.H+prevT > 0 {
		t.Errorf("a top deck at the end of the slide still shows: bottom edge at %d", top.Y+top.H+prevT)
	}
	if bottom.Y+prevB < screenH {
		t.Errorf("a bottom deck at the end of the slide still shows: top edge at %d", bottom.Y+prevB)
	}
	// Eased: a quarter of the time is less than a quarter of the way.
	if q := deckSlideOffset(bottom, true, screenH, 0.25); q >= (screenH-bottom.Y)/4 {
		t.Errorf("a quarter of the time went %d of %d: not eased", q, screenH-bottom.Y)
	}
}
