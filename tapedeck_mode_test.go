// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "testing"

// modeFixture is a deck on a long tape, stopped at its start, that has just been shown.
func modeFixture() (*tapeDeck, *Tape) {
	pulses := make([]int, 2000)
	for i := range pulses {
		pulses[i] = tapeTicksPerSecond / 100
	}
	d := &tapeDeck{place: tapeDeckTop}
	tp := deckTape(pulses, "Game")
	d.Update(tp, 0.01)
	return d, tp
}

// run advances the deck for secs seconds in frames of 0.1, and gives the last view.
func run(d *tapeDeck, tp *Tape, secs float64) tapeDeckView {
	var v tapeDeckView
	for i := 0; i < int(secs*10+0.5); i++ {
		v = d.Update(tp, 0.1)
	}
	return v
}

func TestDeckStoppedBecomesTheChipAfterTheIdleTime(t *testing.T) {
	d, tp := modeFixture()
	if v := run(d, tp, tapeDeckIdle-1); v.Mode != deckFull || !v.Visible {
		t.Fatalf("before the idle time the deck is mode %v visible %v", v.Mode, v.Visible)
	}
	if v := run(d, tp, 2); v.Mode != deckChip || !v.Visible {
		t.Fatalf("after the idle time the deck is mode %v visible %v, want the chip, shown", v.Mode, v.Visible)
	}
}

func TestDeckWithThePointerOnItStaysFull(t *testing.T) {
	d, tp := modeFixture()
	d.hot = true
	if v := run(d, tp, 3*tapeDeckIdle); v.Mode != deckFull {
		t.Errorf("a deck with the pointer on it is mode %v", v.Mode)
	}
	d.hot = false
	if v := run(d, tp, tapeDeckIdle-1); v.Mode != deckFull {
		t.Errorf("the wait did not start again from the pointer leaving: mode %v", v.Mode)
	}
}

func TestDeckPlayingStaysFullAndPausedWaits(t *testing.T) {
	d, tp := modeFixture()
	tp.Play()
	if v := run(d, tp, 2*tapeDeckIdle); v.Mode != deckFull {
		t.Errorf("a playing deck is mode %v", v.Mode)
	}
	tp.Pause()
	if v := run(d, tp, tapeDeckIdle+1); v.Mode != deckChip {
		t.Errorf("a paused deck is mode %v after the idle time, want the chip", v.Mode)
	}
	tp.Pause() // let go: it plays on, and the deck is full again
	if v := d.Update(tp, 0.1); v.Mode != deckFull {
		t.Errorf("a tape set going again left the deck in mode %v", v.Mode)
	}
}

func TestDeckRewindingIsNotIdle(t *testing.T) {
	d, tp := modeFixture()
	tp.st.Position = 1500
	d.Update(tp, 0.1)
	d.Press(tp, tapeRewind)
	if v := run(d, tp, 0.5); v.Mode != deckFull || !v.Rewinding {
		t.Errorf("mid-rewind: mode %v rewinding %v", v.Mode, v.Rewinding)
	}
}

func TestDeckChipAndAwayComeBackWhenTheTapeStartsOrChanges(t *testing.T) {
	for _, away := range []bool{false, true} {
		d, tp := modeFixture()
		if away {
			d.Hide()
		} else {
			d.Minimise()
		}
		v := run(d, tp, 0.5) // long enough for a deck sent away to have slid out
		if away && v.Visible {
			t.Fatal("a deck sent away is shown")
		}
		if !away && (v.Mode != deckChip || !v.Visible) {
			t.Fatalf("the chip is mode %v visible %v", v.Mode, v.Visible)
		}
		tp.Play()
		if v := d.Update(tp, 0.1); v.Mode != deckFull || !v.Visible {
			t.Errorf("away=%v: after Play the deck is mode %v visible %v", away, v.Mode, v.Visible)
		}

		// Another tape does it too.
		d.Hide()
		other := deckTape(make([]int, 50), "Other")
		other.st.Filename = "/tapes/other.tap"
		if v := d.Update(other, 0.1); v.Mode != deckFull || !v.Visible {
			t.Errorf("away=%v: a new tape left the deck in mode %v visible %v", away, v.Mode, v.Visible)
		}
		// The same file loaded again is other pulses.
		d.Hide()
		again := deckTape(make([]int, 50), "Other")
		again.st.Filename = "/tapes/other.tap"
		if v := d.Update(again, 0.1); v.Mode != deckFull {
			t.Errorf("away=%v: the same tape loaded again left the deck in mode %v", away, v.Mode)
		}
	}
}

func TestDeckTurnedOffAndOnIsFullAgain(t *testing.T) {
	d, tp := modeFixture()
	d.Hide()
	d.place = tapeDeckOff
	d.Update(tp, 0.1)
	d.place = tapeDeckTop
	if v := d.Update(tp, 0.1); v.Mode != deckFull || !v.Visible {
		t.Errorf("a deck turned off and on is mode %v visible %v", v.Mode, v.Visible)
	}
}

func TestChipPlayButtonStartsTheTapeAndKeepsTheChip(t *testing.T) {
	d, tp := modeFixture()
	d.Minimise()
	d.Update(tp, 0.1)
	d.PressFromChip(tp)
	if !tp.st.Playing {
		t.Fatal("the chip's play did not start the tape")
	}
	if v := d.Update(tp, 0.1); v.Mode != deckChip {
		t.Errorf("starting the tape from the chip made the deck mode %v", v.Mode)
	}
	d.PressFromChip(tp) // playing: it holds the tape
	if !tp.st.Paused || tp.st.Playing {
		t.Errorf("the chip's button on a playing tape: playing %v paused %v", tp.st.Playing, tp.st.Paused)
	}
	d.PressFromChip(tp) // held: it lets go
	if !tp.st.Playing {
		t.Error("the chip's button did not let a held tape go")
	}
	if v := d.Update(tp, 0.1); v.Mode != deckChip {
		t.Errorf("the chip came back to full by the tape going on: mode %v", v.Mode)
	}
}

func TestDeckWheelStepsOneNotchAtATimeAndWaits(t *testing.T) {
	d := &tapeDeck{}
	if got := d.Wheel(0.4, 0.01); got != 0 {
		t.Errorf("a part of a notch made a step: %d", got)
	}
	if got := d.Wheel(0.7, 0.01); got != 1 {
		t.Errorf("a notch made %d, want +1", got)
	}
	if got := d.Wheel(1, 0.01); got != 0 {
		t.Errorf("a second notch at once made a step: %d", got)
	}
	// After the wait, the next notch is a step again.
	if got := d.Wheel(1, tapeDeckWheelCool); got != 1 {
		t.Errorf("a notch after the wait made %d, want +1", got)
	}
	d2 := &tapeDeck{}
	if d2.Wheel(-1, 0.01) != -1 {
		t.Error("a turn towards the user is not -1")
	}
	// A turn the other way drops a part of a notch the first way.
	d3 := &tapeDeck{}
	d3.Wheel(0.6, 0.2)
	if d3.Wheel(-0.6, 0.2) != 0 {
		t.Error("a turn the other way did not drop the part-notch")
	}
}

func TestStepDeckSizeGoesBigSmallHiddenChip(t *testing.T) {
	type step struct {
		from tapeCassetteSize
		dir  int
		to   tapeCassetteSize
		mini bool
	}
	for _, s := range []step{
		{cassetteBig, -1, cassetteSmall, false},
		{cassetteSmall, -1, cassetteHidden, false},
		{cassetteHidden, -1, cassetteHidden, true},
		{cassetteHidden, 1, cassetteSmall, false},
		{cassetteSmall, 1, cassetteBig, false},
		{cassetteBig, 1, cassetteBig, false},
	} {
		to, mini := stepDeckSize(s.from, s.dir)
		if to != s.to || mini != s.mini {
			t.Errorf("%v %+d: got %v mini=%v, want %v mini=%v", s.from, s.dir, to, mini, s.to, s.mini)
		}
	}
}
