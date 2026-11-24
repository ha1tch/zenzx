// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"math"
	"testing"

	"github.com/ha1tch/zentools/pkg/tap"
)

// deckTape is a loaded Accurate tape of the given pulses, with one Program header.
func deckTape(pulses []int, name string) *Tape {
	t := NewTape(nil)
	t.st.Loaded = true
	t.st.Filename = "/tapes/" + name + ".tap"
	t.st.Mode = TapeAccurate
	t.st.Pulses = pulses
	if name != "" {
		t.st.Blocks = []TapeBlock{{IsHeader: true, HeaderType: tap.TypeProgram, HeaderName: name}}
	}
	return t
}

func TestTapeDeckPlaceWords(t *testing.T) {
	for i, w := range []string{"off", "top", "bottom"} {
		p := tapeDeckPlace(i)
		if p.String() != w {
			t.Errorf("place %d is %q, want %q", i, p.String(), w)
		}
		if got := parseTapeDeckPlace(w, tapeDeckBottom); got != p {
			t.Errorf("parse %q = %v, want %v", w, got, p)
		}
	}
	if got := parseTapeDeckPlace("sideways", tapeDeckTop); got != tapeDeckTop {
		t.Errorf("an unknown word gave %v, want the default", got)
	}
	if tapeDeckPlace(9).String() != "off" || tapeDeckPlace(-1).String() != "off" {
		t.Error("an out-of-range place does not read as off")
	}
}

// The buttons are in the order of a recorder: Play, Rewind, Stop, Pause.
func TestTapeButtonOrder(t *testing.T) {
	want := []string{"Play", "Rewind", "Stop", "Pause"}
	for i, w := range want {
		if tapeButtonNames[i] != w {
			t.Errorf("button %d is %q, want %q", i, tapeButtonNames[i], w)
		}
	}
	if tapeButtons != 4 || tapePlay != 0 || tapeRewind != 1 || tapeStop != 2 || tapePause != 3 {
		t.Error("the button constants are not Play, Rewind, Stop, Pause")
	}
}

func TestTapeClockSums(t *testing.T) {
	// More than two chunks, with lengths that make every prefix sum different.
	n := 2*tapeClockChunk + 100
	pulses := make([]int, n)
	for i := range pulses {
		pulses[i] = 1000 + i%7
	}
	var c tapeClock
	c.sync(pulses)
	var want int64
	for pos := 0; pos <= n; pos++ {
		if got := c.played(pulses, pos, 0); got != want {
			t.Fatalf("played at %d = %d, want %d", pos, got, want)
		}
		if pos < n {
			want += int64(pulses[pos])
		}
	}
	if c.total != want {
		t.Errorf("total = %d, want %d", c.total, want)
	}
	if got := c.played(pulses, 5, 77); got != c.played(pulses, 5, 0)+77 {
		t.Errorf("the offset into a pulse is not added: %d", got)
	}
	if got := c.played(pulses, 0, 30); got != 30 {
		t.Errorf("played at the start with an offset = %d, want 30", got)
	}
	if got := c.played(pulses, n+50, 0); got != c.total {
		t.Errorf("past the end = %d, want the total", got)
	}
}

func TestTapeClockResyncsOnNewPulses(t *testing.T) {
	a := []int{10, 20, 30}
	b := []int{100, 200, 300, 400}
	var c tapeClock
	c.sync(a)
	if c.total != 60 {
		t.Fatalf("total %d, want 60", c.total)
	}
	c.sync(a) // unchanged: nothing to do
	c.sync(b)
	if c.total != 1000 {
		t.Errorf("after a different tape the total is %d, want 1000", c.total)
	}
	// Same length, different slice.
	d := []int{1, 1, 1, 1}
	c.sync(d)
	if c.total != 4 {
		t.Errorf("a different slice of the same length kept the old sums: %d", c.total)
	}
	c.sync(nil)
	if c.total != 0 || len(c.chunks) != 0 {
		t.Error("an empty tape left sums behind")
	}
}

func TestDeckHiddenWhenOffFastOrEmpty(t *testing.T) {
	pulses := []int{3500000, 3500000}
	var d tapeDeck
	tp := deckTape(pulses, "Game")
	if v := d.Update(tp, 0.02); v.Visible {
		t.Error("deck visible while its place is off")
	}
	d.place = tapeDeckTop
	if v := d.Update(tp, 0.02); !v.Visible {
		t.Error("deck hidden for a loaded Accurate tape")
	}
	tp.st.Mode = TapeFast
	if v := d.Update(tp, 0.02); v.Visible {
		t.Error("deck visible in Fast mode")
	}
	tp.st.Mode = TapeTurbo
	if v := d.Update(tp, 0.02); !v.Visible {
		t.Error("deck hidden in Turbo mode")
	}
	tp.st.Loaded = false
	if v := d.Update(tp, 0.02); v.Visible {
		t.Error("deck visible with no tape")
	}
	if v := d.Update(nil, 0.02); v.Visible {
		t.Error("deck visible with a nil tape")
	}
	if v := d.Update(deckTape(nil, "Game"), 0.02); v.Visible {
		t.Error("deck visible for a tape with no pulses")
	}
}

func TestDeckProgress(t *testing.T) {
	// Pulses of unequal length: half the time is not half the pulses.
	pulses := []int{1_000_000, 1_000_000, 5_000_000}
	d := tapeDeck{place: tapeDeckBottom}
	tp := deckTape(pulses, "Game")
	v := d.Update(tp, 0.02)
	if v.Fraction != 0 || v.Elapsed != 0 {
		t.Errorf("a tape at its start shows %v / %v", v.Fraction, v.Elapsed)
	}
	if want := 7_000_000.0 / tapeTicksPerSecond; math.Abs(v.Length-want) > 1e-9 {
		t.Errorf("length = %v, want %v", v.Length, want)
	}
	tp.st.Position = 2
	tp.st.EdgeOffset = 1_500_000 // 1.5M into the 5M pulse: 3.5M of 7M played
	v = d.Update(tp, 0.02)
	if math.Abs(v.Fraction-0.5) > 1e-9 {
		t.Errorf("fraction = %v, want 0.5", v.Fraction)
	}
	if math.Abs(v.Elapsed-1.0) > 1e-9 {
		t.Errorf("elapsed = %v, want 1 s", v.Elapsed)
	}
	tp.st.Playing = true
	if v = d.Update(tp, 0.02); !v.Playing || v.Paused {
		t.Errorf("playing tape shows Playing=%v Paused=%v", v.Playing, v.Paused)
	}
}

func TestDeckLingersAtEnd(t *testing.T) {
	d := tapeDeck{place: tapeDeckTop, appear: 1} // a deck that is in its place already
	tp := deckTape([]int{100, 100}, "Game")
	tp.st.Position = 2
	var shown, sliding float64
	last := -1.0
	for i := 0; i < 600; i++ {
		v := d.Update(tp, 0.01)
		if !v.Visible {
			break
		}
		if !v.AtEnd || v.Fraction != 1 {
			t.Fatalf("at the end the view says AtEnd=%v fraction=%v", v.AtEnd, v.Fraction)
		}
		if v.Slide == 0 {
			shown += 0.01
			continue
		}
		if v.Slide < last {
			t.Fatalf("the slide went back from %.3f to %.3f", last, v.Slide)
		}
		last = v.Slide
		sliding += 0.01
	}
	if math.Abs(shown-tapeDeckLinger) > 0.05 {
		t.Errorf("the deck lingered %.2f s after the end, want %.1f", shown, tapeDeckLinger)
	}
	if math.Abs(sliding-tapeDeckSlideTime) > 0.03 {
		t.Errorf("the deck slid out for %.2f s, want %.2f", sliding, tapeDeckSlideTime)
	}
	if last <= 0 || last >= 1 {
		t.Errorf("the last visible frame of the slide was at %.3f, want it in (0, 1)", last)
	}
	// It stays hidden while the tape stays at its end.
	if v := d.Update(tp, 0.01); v.Visible {
		t.Error("the deck came back by itself at the end of the tape")
	}
	// Rewinding brings it back, and the end gets its linger again.
	tp.Rewind()
	if v := d.Update(tp, 0.01); !v.Visible || v.AtEnd {
		t.Errorf("after a rewind visible=%v atEnd=%v", v.Visible, v.AtEnd)
	}
	for i := 0; i < 30; i++ { // it slides back in
		d.Update(tp, 0.01)
	}
	tp.st.Position = 2
	if v := d.Update(tp, 0.01); !v.Visible || v.Slide != 0 {
		t.Errorf("the second end of the tape did not linger: visible=%v slide=%v", v.Visible, v.Slide)
	}
}

// A deck sliding out at the end of the tape comes straight back, in place, when the tape is
// wound back.
func TestDeckSlideIsCutShortByARewind(t *testing.T) {
	d := tapeDeck{place: tapeDeckTop, appear: 1}
	tp := deckTape([]int{100, 100}, "Game")
	tp.st.Position = 2
	var v tapeDeckView
	for i := 0; i < 600 && v.Slide < 0.5; i++ {
		v = d.Update(tp, 0.01)
	}
	if v.Slide < 0.5 || !v.Visible {
		t.Fatalf("no slide to cut short: %+v", v)
	}
	before := v.Slide
	tp.Rewind()
	// It turns round from where it has got to, and slides back in.
	if v = d.Update(tp, 0.01); !v.Visible || v.Slide >= before || v.AtEnd {
		t.Errorf("after the rewind: visible %v slide %v (was %v) atEnd %v", v.Visible, v.Slide, before, v.AtEnd)
	}
	for i := 0; i < 30; i++ {
		v = d.Update(tp, 0.01)
	}
	if v.Slide != 0 {
		t.Errorf("the deck did not come all the way back: slide %v", v.Slide)
	}
}

func TestDeckLabel(t *testing.T) {
	d := tapeDeck{place: tapeDeckTop}
	tp := deckTape([]int{100}, "Manic Miner")
	if v := d.Update(tp, 0.01); v.Label != "Manic Miner" {
		t.Errorf("label %q, want the first program's name", v.Label)
	}
	// The label is of the first Program header even before the tape has moved.
	tp.st.Blocks = []TapeBlock{
		{IsHeader: true, HeaderType: tap.TypeCode, HeaderName: "screen"},
		{IsHeader: true, HeaderType: tap.TypeProgram, HeaderName: "Loader   "},
		{IsHeader: true, HeaderType: tap.TypeProgram, HeaderName: "Second"},
	}
	tp.st.Filename = "/x/other.tzx"
	if v := d.Update(tp, 0.01); v.Label != "Loader" {
		t.Errorf("label %q, want Loader", v.Label)
	}
	// A tape with no Program header is labelled with its file's name.
	tp.st.Blocks = []TapeBlock{{IsHeader: true, HeaderType: tap.TypeCode, HeaderName: "screen"}}
	tp.st.Filename = "C:\\tapes\\Jet Set Willy.tzx"
	if v := d.Update(tp, 0.01); v.Label != "Jet Set Willy" {
		t.Errorf("fallback label %q, want the file's name", v.Label)
	}
	// The label is not worked out again every frame: it follows the file.
	tp.st.Blocks = nil
	if v := d.Update(tp, 0.01); v.Label != "Jet Set Willy" {
		t.Errorf("label changed with the same file: %q", v.Label)
	}
}

func TestTapePause(t *testing.T) {
	tp := deckTape([]int{100}, "G")
	tp.Pause() // stopped: nothing to hold
	if tp.st.Playing || tp.st.Paused {
		t.Error("Pause on a stopped tape did something")
	}
	tp.Play()
	tp.Pause()
	if tp.st.Playing || !tp.st.Paused {
		t.Errorf("Pause on a playing tape: playing=%v paused=%v", tp.st.Playing, tp.st.Paused)
	}
	tp.Pause()
	if !tp.st.Playing || tp.st.Paused {
		t.Errorf("a second Pause: playing=%v paused=%v", tp.st.Playing, tp.st.Paused)
	}
	tp.Pause()
	tp.Play() // Play lets a held tape go
	if !tp.st.Playing || tp.st.Paused {
		t.Errorf("Play on a held tape: playing=%v paused=%v", tp.st.Playing, tp.st.Paused)
	}
	tp.Pause()
	tp.Stop() // Stop lets go of Pause too
	if tp.st.Playing || tp.st.Paused {
		t.Errorf("Stop on a held tape: playing=%v paused=%v", tp.st.Playing, tp.st.Paused)
	}
	tp.st.Loaded = false
	tp.Play()
	tp.Pause()
	if tp.st.Playing || tp.st.Paused {
		t.Error("buttons did something with no tape")
	}
}

func TestDeckPress(t *testing.T) {
	var d tapeDeck
	tp := deckTape([]int{100, 100, 100}, "G")
	tp.st.Position = 2
	d.Press(tp, tapePlay)
	if !tp.st.Playing {
		t.Error("Play did not play")
	}
	d.Press(tp, tapePause)
	if tp.st.Playing || !tp.st.Paused || tp.st.Position != 2 {
		t.Errorf("Pause: playing=%v paused=%v pos=%d", tp.st.Playing, tp.st.Paused, tp.st.Position)
	}
	d.Press(tp, tapeStop)
	if tp.st.Playing || tp.st.Paused || tp.st.Position != 2 {
		t.Errorf("Stop moved or held the tape: playing=%v paused=%v pos=%d", tp.st.Playing, tp.st.Paused, tp.st.Position)
	}
	d.Press(tp, tapePlay)
	d.Press(tp, tapeRewind)
	if tp.st.Playing {
		t.Errorf("Rewind left the tape playing")
	}
	d.place = tapeDeckTop
	for i := 0; i < 1000 && tp.st.Position != 0; i++ { // it winds back, and gets there
		d.Update(tp, 0.01)
	}
	if tp.st.Position != 0 {
		t.Errorf("Rewind: pos=%d, want 0 once it has wound back", tp.st.Position)
	}
	d.Press(nil, tapePlay) // must not panic
}

func TestFirstProgramNameDoesNotNeedTapeToMove(t *testing.T) {
	tp := deckTape([]int{1}, "Hello")
	if tp.st.Position != 0 {
		t.Fatal("test tape has moved")
	}
	if got := tp.firstProgramName(); got != "Hello" {
		t.Errorf("firstProgramName = %q", got)
	}
	var nilTape *Tape
	if nilTape.firstProgramName() != "" {
		t.Error("a nil tape has a program")
	}
}

// ---- layout ----

func inside(outer, in deckRect) bool {
	return in.X >= outer.X && in.Y >= outer.Y && in.X+in.W <= outer.X+outer.W && in.Y+in.H <= outer.Y+outer.H
}

func overlap(a, b deckRect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func TestDeckLayoutHoldsTogether(t *testing.T) {
	areas := []deckRect{
		{0, 0, 320, 28}, {0, 0, 640, 56}, {0, 0, 960, 96}, {0, 0, 1280, 128},
		{10, 20, 700, 90}, {0, 0, 300, 120}, {0, 0, 200, 80},
	}
	for _, a := range areas {
		for _, lineH := range []int{8, 12, 16, 24} {
			l := layoutTapeDeck(a, lineH)
			if l.Panel.empty() {
				continue
			}
			where := func(s string) string { return s }
			_ = where
			if !inside(a, l.Panel) {
				t.Errorf("%v line %d: panel %v outside the area", a, lineH, l.Panel)
			}
			for name, r := range map[string]deckRect{"cassette": l.Cassette, "bar": l.Bar, "time": l.Time, "name": l.Name} {
				if r.empty() {
					continue
				}
				if !inside(l.Panel, r) {
					t.Errorf("%v line %d: %s %v outside the panel %v", a, lineH, name, r, l.Panel)
				}
			}
			if !inside(l.Cassette, l.Label) {
				t.Errorf("%v line %d: label %v outside the cassette %v", a, lineH, l.Label, l.Cassette)
			}
			for i, b := range l.Buttons {
				if !inside(l.Panel, b) || b.empty() {
					t.Errorf("%v line %d: button %d %v not in the panel", a, lineH, i, b)
				}
				if overlap(b, l.Cassette) || overlap(b, l.Bar) || overlap(b, l.Time) || overlap(b, l.Name) {
					t.Errorf("%v line %d: button %d %v overlaps another part", a, lineH, i, b)
				}
				if i > 0 {
					p := l.Buttons[i-1]
					if b.X < p.X+p.W {
						t.Errorf("%v line %d: button %d starts before button %d ends", a, lineH, i, i-1)
					}
					if b.Y != p.Y || b.W != p.W || b.H != p.H {
						t.Errorf("%v line %d: buttons differ in size or row", a, lineH)
					}
				}
			}
			if overlap(l.Bar, l.Cassette) || overlap(l.Bar, l.Name) || overlap(l.Time, l.Cassette) || overlap(l.Time, l.Bar) {
				t.Errorf("%v line %d: bar, counter and cassette overlap: %+v", a, lineH, l)
			}
		}
	}
}

func TestDeckLayoutCassetteKeepsItsProportions(t *testing.T) {
	for _, h := range []int{28, 56, 96, 128} {
		l := layoutTapeDeck(deckRect{0, 0, 1000, h}, 12)
		if l.Cassette.empty() {
			t.Fatalf("no cassette at height %d", h)
		}
		got := float64(l.Cassette.W) / float64(l.Cassette.H)
		want := float64(cassetteW) / float64(cassetteH)
		if math.Abs(got-want) > 0.06 {
			t.Errorf("height %d: cassette %dx%d has proportions %.3f, want %.3f", h, l.Cassette.W, l.Cassette.H, got, want)
		}
	}
}

func TestDeckLayoutLabelLegibility(t *testing.T) {
	small := layoutTapeDeck(deckRect{0, 0, 1000, 28}, 14)
	if small.LabelOnCassette {
		t.Error("the label is claimed readable on a 28 unit deck")
	}
	big := layoutTapeDeck(deckRect{0, 0, 1000, 120}, 12)
	if !big.LabelOnCassette {
		t.Error("the label is not readable on a 120 unit deck with 12 unit text")
	}
	if !big.Name.empty() {
		t.Error("a name row is given when the label is on the cassette")
	}
	mid := layoutTapeDeck(deckRect{0, 0, 1000, 80}, 16)
	if !mid.LabelOnCassette && mid.Name.empty() {
		t.Error("the label is unreadable on the cassette and has no row of its own")
	}
}

func TestDeckLayoutNoRoom(t *testing.T) {
	for _, a := range []deckRect{{}, {0, 0, 0, 50}, {0, 0, 50, 0}, {0, 0, 5, 5}, {0, 0, -3, 40}} {
		l := layoutTapeDeck(a, 12)
		if !l.Panel.empty() && !inside(a, l.Panel) {
			t.Errorf("%v: panel %v outside", a, l.Panel)
		}
	}
	if l := layoutTapeDeck(deckRect{0, 0, 400, 80}, 0); l.Panel.empty() {
		t.Error("a zero line height broke the layout")
	}
}

func TestDeckLayoutCentred(t *testing.T) {
	a := deckRect{100, 40, 1200, 100}
	l := layoutTapeDeck(a, 12)
	left, right := l.Panel.X-a.X, a.X+a.W-(l.Panel.X+l.Panel.W)
	if d := left - right; d < -1 || d > 1 {
		t.Errorf("panel is off-centre: %d left, %d right", left, right)
	}
}

func TestDeckRectContains(t *testing.T) {
	r := deckRect{10, 20, 30, 40}
	for _, c := range []struct {
		x, y float32
		in   bool
	}{{10, 20, true}, {39.9, 59.9, true}, {40, 30, false}, {20, 60, false}, {9.9, 30, false}} {
		if r.Contains(c.x, c.y) != c.in {
			t.Errorf("Contains(%v,%v) = %v", c.x, c.y, !c.in)
		}
	}
	if (deckRect{}).Contains(0, 0) {
		t.Error("an empty rect contains a point")
	}
}
