// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "math"

// ============================================================================
// How the deck moves from one form to another
//
// The deck is full (at one of its sizes), or the chip, at the top or at the bottom. When any of
// those changes, the deck does not jump: every part of it moves from where it was to where it is
// to be, over deckMorphTime, easing in and out as the machine dock does. A part that only one of
// the two forms has (the Rewind button, which the chip has not) grows from, or shrinks into, its
// own place in the panel, which is itself on its way from one size to the other. A deck that has
// been dragged and let go does the same, from where it was let go to its edge.
//
// This file is the arithmetic of it, with no window: tapedeck_gui.go does the drawing.
// ============================================================================

// deckMorphTime is how long a change of form takes, in seconds: the time of the dock's slide.
const deckMorphTime = tapeDeckSlideTime

// deckEase is the dock's easing: slow at the start, fast in the middle, slow at the end, for t
// from 0 to 1.
func deckEase(t float64) float64 {
	t = math.Min(math.Max(t, 0), 1)
	if t < 0.5 {
		return 4 * t * t * t
	}
	f := -2*t + 2
	return 1 - f*f*f/2
}

// deckKey is what makes one form of the deck other than another.
type deckKey struct {
	form   deckMode         // full or the chip
	size   tapeCassetteSize // the cassette's size, for a full deck (0 for the chip)
	bottom bool             // at the bottom edge, not the top
}

// chipAsLayout is the chip's parts as a full deck's, so that the two can be moved into each
// other: the play button is the Play button, and the parts the chip has not are empty.
func chipAsLayout(c deckChipLayout) tapeDeckLayout {
	l := tapeDeckLayout{Panel: c.Panel, Cassette: c.Cassette, Time: c.Time, Close: c.Close, Bar: c.Bar}
	l.Buttons[tapePlay] = c.Play
	return l
}

// layoutAsChip is chipAsLayout the other way round.
func layoutAsChip(l tapeDeckLayout) deckChipLayout {
	return deckChipLayout{
		Panel: l.Panel, Cassette: l.Cassette, Play: l.Buttons[tapePlay],
		Time: l.Time, Close: l.Close, Bar: l.Bar,
	}
}

func lerpInt(a, b int, t float64) int { return a + int(math.Round(float64(b-a)*t)) }

// deckMinPart is the least a part can be, in units, in either direction, to be drawn: a part
// that is growing or shrinking is not shown as a speck.
const deckMinPart = 3

// lerpRect is the rectangle t of the way from a to b.
func lerpRect(a, b deckRect, t float64) deckRect {
	return deckRect{lerpInt(a.X, b.X, t), lerpInt(a.Y, b.Y, t), lerpInt(a.W, b.W, t), lerpInt(a.H, b.H, t)}
}

// lerpLayout is the deck t of the way (0 to 1) from the layout a to the layout b. A part that is
// in both moves and changes size in a straight line, as does the panel; one that only a has
// shrinks, and one that only b has grows, about the same place in the panel as it has there,
// the panel being the one that is on its way.
func lerpLayout(a, b tapeDeckLayout, t float64) tapeDeckLayout {
	t = math.Min(math.Max(t, 0), 1)
	if a.Panel.empty() {
		return b
	}
	if b.Panel.empty() {
		return a
	}
	var l tapeDeckLayout
	l.Panel = lerpRect(a.Panel, b.Panel, t)
	// part moves one part. Where only one of the two has it, it is scaled about its own place in
	// the panel it is in, which is put in the panel that is shown.
	part := func(ra, rb deckRect) deckRect {
		switch {
		case ra.empty() && rb.empty():
			return deckRect{}
		case !ra.empty() && !rb.empty():
			return lerpRect(ra, rb, t)
		}
		r, home, k := rb, b.Panel, t // growing: b's part
		if rb.empty() {
			r, home, k = ra, a.Panel, 1-t // shrinking: a's part
		}
		w, h := int(math.Round(float64(r.W)*k)), int(math.Round(float64(r.H)*k))
		if w < deckMinPart || h < deckMinPart || home.W <= 0 || home.H <= 0 {
			return deckRect{}
		}
		fx := (float64(r.X) + float64(r.W)/2 - float64(home.X)) / float64(home.W)
		fy := (float64(r.Y) + float64(r.H)/2 - float64(home.Y)) / float64(home.H)
		cx := float64(l.Panel.X) + fx*float64(l.Panel.W)
		cy := float64(l.Panel.Y) + fy*float64(l.Panel.H)
		return deckRect{int(math.Round(cx)) - w/2, int(math.Round(cy)) - h/2, w, h}
	}
	l.Cassette = part(a.Cassette, b.Cassette)
	l.Label = part(a.Label, b.Label)
	l.Bar = part(a.Bar, b.Bar)
	l.Time = part(a.Time, b.Time)
	l.Name = part(a.Name, b.Name)
	l.Minimise = part(a.Minimise, b.Minimise)
	l.Close = part(a.Close, b.Close)
	for i := range l.Buttons {
		l.Buttons[i] = part(a.Buttons[i], b.Buttons[i])
	}
	l.LabelOnCassette = a.LabelOnCassette
	if t >= 0.5 {
		l.LabelOnCassette = b.LabelOnCassette
	}
	return l
}

// deckMorph is the deck moving from one form to another. step is called once a frame with the
// form the deck is to be in; drop when a dragged deck is let go.
type deckMorph struct {
	key    deckKey
	keyed  bool // key is the form of the last frame
	active bool // a move is under way
	t      float64

	from     tapeDeckLayout // where the move began, and whether that was the chip
	fromChip bool

	last     tapeDeckLayout // what was drawn last frame, and whether the chip's look
	lastChip bool

	dropped    bool // a dragged deck was let go: the next frame moves it from dropFrom
	dropFrom   tapeDeckLayout
	dropIsChip bool
}

// reset forgets everything: the deck is not on the screen, and comes back where it belongs.
func (m *deckMorph) reset() { *m = deckMorph{} }

// Active reports whether a move is under way.
func (m *deckMorph) Active() bool { return m.active }

// drop says that a dragged deck is let go where it is, as the layout from, and is to go to its
// place, which the next step is given. The chip says whether it is drawn as the chip.
func (m *deckMorph) drop(from tapeDeckLayout, chip bool) {
	m.dropped, m.dropFrom, m.dropIsChip = true, from, chip
}

// step takes one frame of dt seconds, for a deck whose form is key and whose parts are at
// target, which is the chip's, as a layout, when chip is set. It returns where the parts are
// to be drawn, whether that is as the chip, and whether they are in the middle of a move.
func (m *deckMorph) step(key deckKey, target tapeDeckLayout, chip bool, dt float64) (geo tapeDeckLayout, asChip, moving bool) {
	switch {
	case !m.keyed || m.last.Panel.empty():
		m.active, m.dropped = false, false
	case m.dropped:
		m.from, m.fromChip, m.t, m.active = m.dropFrom, m.dropIsChip, 0, true
	case key != m.key:
		m.from, m.fromChip, m.t, m.active = m.last, m.lastChip, 0, true
	}
	m.dropped = false
	m.key, m.keyed = key, true
	if m.active {
		if m.t += dt / deckMorphTime; m.t >= 1 {
			m.active = false
		}
	}
	if !m.active {
		m.last, m.lastChip = target, chip
		return target, chip, false
	}
	geo = lerpLayout(m.from, target, deckEase(m.t))
	// A move from the chip to the chip (to the other edge) stays the chip; any other is drawn as
	// a full deck, whose parts the chip's are among.
	asChip = m.fromChip && chip
	m.last, m.lastChip = geo, asChip
	return geo, asChip, true
}
