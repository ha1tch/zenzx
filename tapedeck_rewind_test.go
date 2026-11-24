// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"math"
	"testing"
)

func TestTapeClockSeekIsThePlayedInverse(t *testing.T) {
	n := 2*tapeClockChunk + 300
	pulses := make([]int, n)
	for i := range pulses {
		pulses[i] = 500 + (i*37)%900
	}
	var c tapeClock
	c.sync(pulses)
	for _, ticks := range []int64{0, 1, 499, 500, 12345, c.total/2 + 7, c.total - 1} {
		pos, off := c.seek(pulses, ticks)
		if got := c.played(pulses, pos, off); got != ticks {
			t.Errorf("seek(%d) = %d,%d, played again = %d", ticks, pos, off, got)
		}
		if pos < len(pulses) && (off < 0 || off >= pulses[pos]) {
			t.Errorf("seek(%d): offset %d outside pulse %d of length %d", ticks, off, pos, pulses[pos])
		}
	}
	if pos, off := c.seek(pulses, c.total); pos != n || off != 0 {
		t.Errorf("seek(total) = %d,%d", pos, off)
	}
	if pos, off := c.seek(pulses, c.total+99); pos != n || off != 0 {
		t.Errorf("seek past the end = %d,%d", pos, off)
	}
	if pos, off := c.seek(pulses, -5); pos != 0 || off != 0 {
		t.Errorf("seek(-5) = %d,%d", pos, off)
	}
	if pos, off := c.seek(nil, 100); pos != 0 || off != 0 {
		t.Errorf("seek on nothing = %d,%d", pos, off)
	}
}

// a tape of ten seconds in 1000 pulses, played to the middle, with the deck on.
func rewindFixture() (*tapeDeck, *Tape) {
	pulses := make([]int, 1000)
	for i := range pulses {
		pulses[i] = tapeTicksPerSecond / 100
	}
	d := &tapeDeck{place: tapeDeckTop}
	tp := deckTape(pulses, "G")
	tp.st.Position = 500
	d.Update(tp, 0.01)
	return d, tp
}

func TestDeckRewindWindsBackOverTime(t *testing.T) {
	d, tp := rewindFixture()
	tp.Play()
	d.Press(tp, tapeRewind)
	if tp.st.Playing || tp.st.Position != 500 {
		t.Fatalf("at the press: playing=%v pos=%d (the tape stays where it is until it has wound)", tp.st.Playing, tp.st.Position)
	}
	v := d.Update(tp, 0.0)
	if !v.Rewinding || v.AtEnd || !v.Visible {
		t.Fatalf("view at the start of the rewind: %+v", v)
	}
	if math.Abs(v.Fraction-0.5) > 1e-6 {
		t.Errorf("the bar starts at %v, want 0.5", v.Fraction)
	}
	// 5 s played at 40x is 0.125 s, so the minimum rules.
	if math.Abs(d.rew.dur-tapeRewindMin) > 1e-9 {
		t.Errorf("duration %v, want the minimum %v", d.rew.dur, tapeRewindMin)
	}
	half := d.Update(tp, tapeRewindMin/2)
	if math.Abs(half.Fraction-0.25) > 1e-6 || !half.Rewinding {
		t.Errorf("half way back the bar reads %v (rewinding %v), want 0.25", half.Fraction, half.Rewinding)
	}
	if tp.st.Position != 500 {
		t.Error("the tape moved before the rewind was over")
	}
	end := d.Update(tp, tapeRewindMin)
	if end.Rewinding || end.Fraction != 0 || tp.st.Position != 0 || tp.st.EdgeOffset != 0 {
		t.Errorf("at the end of the rewind: %+v pos=%d", end, tp.st.Position)
	}
}

func TestDeckRewindDurationScalesAndIsLimited(t *testing.T) {
	long := make([]int, 100)
	for i := range long {
		long[i] = tapeTicksPerSecond * 20 // 2000 s of tape: 50 s at 40x, so the maximum rules
	}
	d := &tapeDeck{place: tapeDeckTop}
	tp := deckTape(long, "G")
	tp.st.Position = 100
	d.Update(tp, 0.01)
	d.Press(tp, tapeRewind)
	if d.rew.dur != tapeRewindMax {
		t.Errorf("a long tape rewinds in %v s, want %v", d.rew.dur, tapeRewindMax)
	}
	mid := make([]int, 100)
	for i := range mid {
		mid[i] = tapeTicksPerSecond * 2 // 200 s: 5 s at 40x
	}
	tp2 := deckTape(mid, "G")
	tp2.st.Position = 40 // 80 s played: 2 s
	var d2 tapeDeck
	d2.place = tapeDeckTop
	d2.Update(tp2, 0.01)
	d2.Press(tp2, tapeRewind)
	if math.Abs(d2.rew.dur-2.0) > 1e-9 {
		t.Errorf("80 s of tape rewinds in %v s, want 2", d2.rew.dur)
	}
}

func TestDeckRewindOfNothingIsInstant(t *testing.T) {
	d := &tapeDeck{place: tapeDeckTop}
	tp := deckTape([]int{100, 100}, "G")
	d.Update(tp, 0.01)
	d.Press(tp, tapeRewind)
	if d.rew.active || tp.st.Position != 0 {
		t.Error("a tape at its start winds")
	}
}

func TestDeckRewindFromTheEndKeepsTheDeckUp(t *testing.T) {
	d := &tapeDeck{place: tapeDeckBottom}
	tp := deckTape(make([]int, 10), "G")
	for i := range tp.st.Pulses {
		tp.st.Pulses[i] = tapeTicksPerSecond
	}
	tp.st.Position = 10
	for i := 0; i < 600 && d.Update(tp, 0.01).Visible; i++ { // let it linger and slide away
	}
	if d.Update(tp, 0.01).Visible {
		t.Fatal("the deck did not leave the end of the tape")
	}
	tp.st.Position = 10
	d.linger, d.ended = 1, true
	d.Press(tp, tapeRewind)   // from the end, with the deck up
	for i := 0; i < 60; i++ { // 0.6 s of the 0.7 s it takes
		v := d.Update(tp, 0.01)
		if !v.Visible || v.AtEnd || !v.Rewinding {
			t.Fatalf("frame %d of the rewind from the end: %+v", i, v)
		}
	}
}

func TestDeckRewindCutShortKeepsThePlace(t *testing.T) {
	for _, button := range []tapeButton{tapeStop, tapePlay} {
		d, tp := rewindFixture()
		d.Press(tp, tapeRewind)
		d.Update(tp, tapeRewindMin/2) // half way: 2.5 s of the 5 s are left
		d.Press(tp, button)
		if d.rew.active {
			t.Errorf("%s did not end the rewind", tapeButtonNames[button])
		}
		if tp.st.Position != 250 {
			t.Errorf("%s cut the rewind at pulse %d, want 250", tapeButtonNames[button], tp.st.Position)
		}
		if (button == tapePlay) != tp.st.Playing {
			t.Errorf("%s: playing=%v", tapeButtonNames[button], tp.st.Playing)
		}
		if v := d.Update(tp, 0.01); v.Rewinding {
			t.Errorf("%s: still rewinding in the view", tapeButtonNames[button])
		}
	}
	// The tape set going from outside (Alt+P) cuts it short as well.
	d, tp := rewindFixture()
	d.Press(tp, tapeRewind)
	d.Update(tp, tapeRewindMin/2)
	tp.Play()
	if v := d.Update(tp, 0.0); v.Rewinding || tp.st.Position != 250 {
		t.Errorf("Alt+P during a rewind: rewinding=%v pos=%d", v.Rewinding, tp.st.Position)
	}
}

// The reels turn slowly while the tape plays, fast the other way while it winds back, and not at all
// otherwise.
func TestDeckReelSpeeds(t *testing.T) {
	d, tp := rewindFixture()
	d.spin = 3
	tp.Play()
	play := d.Update(tp, 0.1).Spin - 3
	if math.Abs(play-0.1*cassetteSpinRate) > 1e-9 {
		t.Errorf("playing: turned %v in 0.1 s", play)
	}
	d.Press(tp, tapeRewind)
	before := d.spin
	after := d.Update(tp, 0.1).Spin
	turn := before - after
	if turn < 0 {
		turn += 2 * math.Pi
	}
	if math.Abs(turn-0.1*cassetteRewindSpinRate) > 1e-9 {
		t.Errorf("rewinding: turned back %v in 0.1 s, want %v", turn, 0.1*cassetteRewindSpinRate)
	}
	if cassetteRewindSpinRate < 5*cassetteSpinRate {
		t.Error("rewinding is not much faster than playing")
	}
	if cassetteSpinRate > 3 {
		t.Error("the reels turn quickly while playing")
	}
	for i := 0; i < 100; i++ {
		if s := d.Update(tp, 0.1).Spin; s < 0 || s >= 2*math.Pi {
			t.Fatalf("spin %v out of one turn", s)
		}
	}
}
