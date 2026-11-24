// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"math"
	"math/rand/v2"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ha1tch/zentools/pkg/tap"
)

// ============================================================================
// The tape deck
//
// A cassette, a bar that fills as the tape plays, and the buttons of a tape recorder, shown
// while a tape loads in Accurate (or Turbo) mode. This file is what it knows and decides, with
// no window in it: whether it is shown, how far the tape has played, what the cassette's label
// says, and what the buttons do to the tape. Where it goes on the screen is tapedeck_layout.go,
// and drawing it and taking the mouse is tapedeck_gui.go.
// ============================================================================

// tapeDeckPlace is where the Preferences window puts the tape deck: not at all, in the top of
// the border (or, when the border is not shown, floating near the top of the picture), or in
// the bottom of it (or floating near the bottom).
type tapeDeckPlace int

const (
	tapeDeckOff tapeDeckPlace = iota
	tapeDeckTop
	tapeDeckBottom
)

// tapeDeckWords are the places as settings.json spells them, in the order of the constants.
var tapeDeckWords = [...]string{"off", "top", "bottom"}

func (p tapeDeckPlace) String() string {
	if p < 0 || int(p) >= len(tapeDeckWords) {
		return tapeDeckWords[tapeDeckOff]
	}
	return tapeDeckWords[p]
}

// parseTapeDeckPlace reads a settings word, returning def for one it does not know.
func parseTapeDeckPlace(s string, def tapeDeckPlace) tapeDeckPlace {
	for i, w := range tapeDeckWords {
		if s == w {
			return tapeDeckPlace(i)
		}
	}
	return def
}

// tapeButton is one of the deck's buttons.
type tapeButton int

// tapeButtonOrder is the buttons from the left, in the order the recorders of the time had
// them: Play first, then Rewind (a Datassette or a Panasonic RQ-309 reads REC, PLAY, REW,
// FF, STOP, with Pause the last of the row), and the Stop and Pause that end it. A deck
// with this one list changed is a deck in another order.
const (
	tapePlay tapeButton = iota
	tapeRewind
	tapeStop
	tapePause
	tapeButtons // the number of buttons
)

// tapeButtonNames are the buttons' names, as a tooltip or a test would say them.
var tapeButtonNames = [tapeButtons]string{"Play", "Rewind", "Stop", "Pause"}

// tapeTicksPerSecond turns the T-states the pulses are counted in into the seconds a tape
// counter shows: the 48K's 3.5 MHz, which is how the rest of ZenZX counts a tape (T-03).
const tapeTicksPerSecond = 3_500_000

// tapeDeckLinger is how long the deck stays on the screen after the tape has played to its end,
// in seconds, so that the bar is seen to reach it. Then it slides out over tapeDeckSlideTime
// seconds, towards the edge it is at, as the machine dock does (the time is the dock's own,
// menuBarSlideTime). Rewinding brings it back. A deck comes on, and goes when it is closed, the
// same way: it slides in from its edge and out to it, and what the deck does between full and
// the chip, and from one size or edge to another, takes the same time.
const (
	tapeDeckLinger    = 4.0
	tapeDeckSlideTime = 0.22
)

// ============================================================================
// How far the tape has played
// ============================================================================

// tapeClock adds up the pulses of a tape in T-states, so that how much of the tape has played
// is a time and not a count of pulses: the pilot tone is thousands of pulses of one sound and
// the data's are two lengths, so a pulse count is not a bar that fills evenly. A tape is some
// million pulses, so the sums are kept for each tapeClockChunk of them, and the rest of
// the way to where the tape is now is added up when it is asked for.
type tapeClock struct {
	first  *int // the pulses the sums are of: their first element, and how many there are
	n      int
	chunks []int64 // chunks[i] is the length of pulses [0, i*tapeClockChunk)
	total  int64
}

const tapeClockChunk = 4096

// sync makes the sums those of pulses, doing the work only when they are not already.
func (c *tapeClock) sync(pulses []int) {
	if len(pulses) == 0 {
		*c = tapeClock{}
		return
	}
	if c.n == len(pulses) && c.first == &pulses[0] {
		return
	}
	c.first, c.n = &pulses[0], len(pulses)
	c.chunks = c.chunks[:0]
	var sum int64
	for i, p := range pulses {
		if i%tapeClockChunk == 0 {
			c.chunks = append(c.chunks, sum)
		}
		sum += int64(p)
	}
	c.total = sum
}

// played is how many T-states of pulses there are before pulse pos, and off into it.
func (c *tapeClock) played(pulses []int, pos, off int) int64 {
	if len(pulses) == 0 || pos <= 0 {
		return int64(max(off, 0))
	}
	if pos >= len(pulses) {
		return c.total
	}
	chunk := pos / tapeClockChunk
	sum := c.chunks[chunk]
	for _, p := range pulses[chunk*tapeClockChunk : pos] {
		sum += int64(p)
	}
	return sum + int64(off)
}

// seek is the place in the pulses that is ticks T-states in: the pulse it falls in, and how far
// into that pulse. It is the opposite of played.
func (c *tapeClock) seek(pulses []int, ticks int64) (pos, off int) {
	if len(pulses) == 0 || ticks <= 0 {
		return 0, 0
	}
	if ticks >= c.total {
		return len(pulses), 0
	}
	chunk := sort.Search(len(c.chunks), func(i int) bool { return c.chunks[i] > ticks }) - 1
	sum := c.chunks[chunk]
	for pos = chunk * tapeClockChunk; pos < len(pulses); pos++ {
		if sum+int64(pulses[pos]) > ticks {
			return pos, int(ticks - sum)
		}
		sum += int64(pulses[pos])
	}
	return len(pulses), 0
}

// ============================================================================
// What the deck shows
// ============================================================================

// tapeDeckView is the deck as one frame of it has to be drawn.
type tapeDeckView struct {
	Visible   bool
	Playing   bool    // the Play button is down
	Paused    bool    // the Pause button is down
	Fraction  float64 // how much of the tape has played, 0 to 1
	Elapsed   float64 // seconds played, and the tape's length in seconds
	Length    float64
	Rewinding bool     // the tape is being wound back: the bar runs back and the reels turn the other way
	AtEnd     bool     // the tape has played to its end
	Label     string   // what the cassette's label says: the tape's first program
	Variant   int      // which colour of cassette (cassette_vector.go): a new one for every new tape
	Spin      float64  // how far the reels have turned, in radians
	Mode      deckMode // full, the chip, or away (tapedeck_mode.go)
	Form      deckMode // what it is drawn as: Mode, or for a deck that is away the form it slides out as
	Slide     float64  // how far it is slid out of its edge, 0 (in place) to 1 (gone), in time (not yet eased)
}

// tapeDeck is the deck's state between frames.
type tapeDeck struct {
	place   tapeDeckPlace
	linger  float64 // seconds of showing left after the tape ended
	file    string  // the tape the label below is of
	label   string
	labelOK bool
	ended   bool // the end of this tape has been seen, so the linger has been started

	variant int     // the colour of the cassette in the deck now
	spin    float64 // the turn of its reels
	// pick chooses the colour of the next cassette, given how many there are and the one in
	// the deck; nil is nextCassette. A test sets it.
	pick func(n, prev int) int

	// How the deck is shown (tapedeck_mode.go).
	mode       deckMode // full, the chip, or away, for the tape in the deck
	idle       float64  // seconds the deck has stood still, with the pointer off it
	hot        bool     // the pointer is on the deck: the window sets it before Update
	wasPlaying bool     // the tape was playing last frame, so that its starting is seen
	appear     float64  // how far in its place the deck is, 0 (out of sight) to 1, in time
	form       deckMode // the last mode that was not away: what a deck that is sliding out looks like
	wheel      float64  // turns of the wheel not yet a step
	wheelCool  float64  // seconds before the next step may be made

	// rew is a rewind in progress: the deck's Rewind winds the tape back over some time, the
	// reels turning fast the other way, and the tape's own position is put back at its end (or
	// where Stop or Play cut it short).
	rew struct {
		active    bool
		from      int64   // T-states played when it began
		dur, left float64 // seconds it takes, and seconds still to go
	}
}

// nextCassette is a colour other than the one in the deck: any of the others, at random.
func nextCassette(n, prev int) int {
	if n < 2 {
		return 0
	}
	if prev < 0 || prev >= n {
		return rand.IntN(n)
	}
	return (prev + 1 + rand.IntN(n-1)) % n
}

// cassetteSpinRate is how fast the reels turn while the tape plays, in radians a second.
const cassetteSpinRate = 2.0

// Rewinding is faster: the reels turn the other way at cassetteRewindSpinRate, and the tape goes
// back tapeRewindSpeedUp times as fast as it plays, but never in under tapeRewindMin seconds, so
// that a tape that has hardly played is still seen to wind, nor over tapeRewindMax.
const (
	cassetteRewindSpinRate = 22.0
	tapeRewindSpeedUp      = 40.0
	tapeRewindMin          = 0.7
	tapeRewindMax          = 4.0
)

// firstProgramName is the name of the first Program header on the tape, whether or not it has
// played, and "" for a tape with none.
func (t *Tape) firstProgramName() string {
	if t == nil || t.st == nil {
		return ""
	}
	for _, b := range t.st.Blocks {
		if b.IsHeader && b.HeaderType == tap.TypeProgram {
			if n := programName(b.HeaderName); n != "" {
				return n
			}
		}
	}
	return ""
}

// deckLabel is what the cassette's label says: the tape's first program, as a person would
// have written it on the cassette, or, for a tape with no program (a screen, a code block),
// the name of its file, which is what they would have written then.
func (t *Tape) deckLabel() string {
	if n := t.firstProgramName(); n != "" {
		return n
	}
	name := filepath.Base(strings.ReplaceAll(t.st.Filename, "\\", "/"))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if name == "." || name == "/" {
		return ""
	}
	return name
}

// deckWanted reports whether a tape is in the deck for it to be shown: one is loaded, and it
// is played pulse by pulse, which is what Accurate and Turbo are. In Fast mode the ROM's loader
// is short-circuited and there is no tape to watch.
func deckWanted(t *Tape) bool {
	if t == nil || t.st == nil || !t.st.Loaded || len(t.st.Pulses) == 0 {
		return false
	}
	return t.st.Mode == TapeAccurate || t.st.Mode == TapeTurbo
}

// Update works out the deck for a frame that took dt seconds.
func (d *tapeDeck) Update(t *Tape, dt float64) tapeDeckView {
	if d.place == tapeDeckOff || !deckWanted(t) {
		d.linger, d.appear = 0, 0
		d.mode, d.form, d.idle, d.wasPlaying = deckFull, deckFull, 0, false // a deck brought back is a full one
		return tapeDeckView{}
	}
	st := t.st
	before := t.clock.first
	t.clock.sync(st.Pulses)
	fresh := t.clock.first != before // other pulses: another tape, or this one loaded again
	if d.rew.active {
		if st.Playing { // something else (Alt+P) set the tape going: the rewind stops where it is
			d.cutRewind(t)
		} else if d.rew.left -= dt; d.rew.left <= 0 {
			d.rew.active = false
			t.Rewind()
		}
	}
	v := tapeDeckView{
		Visible:   true,
		Playing:   st.Playing,
		Paused:    st.Paused,
		Rewinding: d.rew.active,
		AtEnd:     st.Position >= len(st.Pulses) && !d.rew.active,
		Length:    float64(t.clock.total) / tapeTicksPerSecond,
	}
	played := t.clock.played(st.Pulses, st.Position, st.EdgeOffset)
	if d.rew.active {
		played = int64(float64(d.rew.from) * d.rew.left / d.rew.dur)
	}
	v.Elapsed = float64(played) / tapeTicksPerSecond
	if t.clock.total > 0 {
		v.Fraction = float64(played) / float64(t.clock.total)
	}
	if v.Fraction > 1 {
		v.Fraction = 1
	}
	if d.file != st.Filename || !d.labelOK || fresh {
		d.mode, d.idle = deckFull, 0
		// A new tape is a new cassette: another colour, with its title on the label.
		pick := d.pick
		if pick == nil {
			pick = nextCassette
		}
		if d.labelOK {
			d.variant = pick(cassetteVariants, d.variant)
		} else {
			d.variant = pick(cassetteVariants, -1)
		}
		d.file, d.label, d.labelOK = st.Filename, t.deckLabel(), true
	}
	v.Label = d.label
	v.Variant = d.variant
	// A tape set going brings the deck back. A deck that is not running, and has the pointer off
	// it, turns into the chip when it has waited long enough.
	if st.Playing && !d.wasPlaying {
		d.mode, d.idle = deckFull, 0
	}
	d.wasPlaying = st.Playing
	running := (st.Playing && !st.Paused && !v.AtEnd) || d.rew.active
	if d.mode == deckFull && !running && !d.hot && !v.AtEnd {
		if d.idle += dt; d.idle >= tapeDeckIdle {
			d.mode = deckChip
		}
	} else {
		d.idle = 0
	}
	v.Mode = d.mode
	switch {
	case d.rew.active:
		d.spin -= dt * cassetteRewindSpinRate
	case st.Playing && !v.AtEnd:
		d.spin += dt * cassetteSpinRate
	}
	if d.spin = math.Mod(d.spin, 2*math.Pi); d.spin < 0 {
		d.spin += 2 * math.Pi
	}
	v.Spin = d.spin

	if v.AtEnd {
		if d.linger <= 0 && !d.ended {
			d.linger, d.ended = tapeDeckLinger, true
		}
		d.linger -= dt
	} else {
		d.linger, d.ended = 0, false
	}
	// The deck is in its place, or on its way there, unless it has been sent away or has
	// lingered at the end of the tape for long enough; then it is on its way out, to the edge it
	// is at. Either way it takes tapeDeckSlideTime, from wherever it has got to.
	away := d.mode == deckAway || (v.AtEnd && d.linger <= 0)
	if away {
		d.appear -= dt / tapeDeckSlideTime
	} else {
		d.appear += dt / tapeDeckSlideTime
	}
	d.appear = math.Min(math.Max(d.appear, 0), 1)
	if d.mode != deckAway {
		d.form = d.mode
	}
	v.Form, v.Slide = d.form, 1-d.appear
	v.Visible = !away || d.appear > 0
	return v
}

// Press is what a click on a button does to the tape. Rewind is a recorder's: the tape stops,
// and then goes back to its start.
func (d *tapeDeck) Press(t *Tape, b tapeButton) {
	if t == nil || t.st == nil || !t.st.Loaded {
		return
	}
	switch b {
	case tapePlay:
		d.cutRewind(t)
		t.Play()
	case tapeStop:
		d.cutRewind(t)
		t.Stop()
	case tapePause:
		t.Pause()
	case tapeRewind:
		d.startRewind(t)
	}
}

// startRewind stops the tape and sets it winding back. A tape that has not played is already
// there.
func (d *tapeDeck) startRewind(t *Tape) {
	t.Stop()
	d.rew.active = false
	t.clock.sync(t.st.Pulses)
	played := t.clock.played(t.st.Pulses, t.st.Position, t.st.EdgeOffset)
	if played <= 0 {
		t.Rewind()
		return
	}
	dur := float64(played) / tapeTicksPerSecond / tapeRewindSpeedUp
	d.rew.active, d.rew.from = true, played
	d.rew.dur = math.Min(math.Max(dur, tapeRewindMin), tapeRewindMax)
	d.rew.left = d.rew.dur
	d.ended, d.linger = false, 0
}

// cutRewind ends a rewind where it has got to: the tape is left at that place.
func (d *tapeDeck) cutRewind(t *Tape) {
	if !d.rew.active {
		return
	}
	d.rew.active = false
	ticks := int64(float64(d.rew.from) * d.rew.left / d.rew.dur)
	t.clock.sync(t.st.Pulses)
	t.st.Position, t.st.EdgeOffset = t.clock.seek(t.st.Pulses, ticks)
	t.st.EarLevel = false
}
