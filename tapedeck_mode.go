// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "slices"

// ============================================================================
// How the deck is shown: full, the chip, or away
//
// A deck that stays up for good is in the way once the tape has stopped or the game has loaded.
// So the deck is in one of three states, for the tape that is in it:
//
//	full  the deck of Preferences > Machines > Tape, at its size
//	chip  a mini cassette with a play button, the counter and a close button, at one end of the
//	      deck's strip
//	away  not shown
//
// A deck that is not running (stopped, paused, or never started) with the pointer not on it
// turns into the chip after tapeDeckIdle seconds. The wheel over the deck steps its size, and
// past the smallest it is the chip; the deck's corner buttons make it the chip or send it away,
// and a click on the chip makes it full. A deck that is the chip or away comes back as a full
// deck when the tape is set going or another tape is loaded.
// ============================================================================

// deckMode is how the deck is shown.
type deckMode int

const (
	deckFull deckMode = iota
	deckChip
	deckAway
)

// tapeDeckIdle is how long a deck that is not running waits, with the pointer off it, before it
// becomes the chip, in seconds.
const tapeDeckIdle = 6.0

// tapeDeckWheelCool is the least time between two steps of the wheel, in seconds, so that a
// trackpad's long spin is not a run through every size.
const tapeDeckWheelCool = 0.12

// Minimise makes the deck the chip.
func (d *tapeDeck) Minimise() { d.mode, d.idle = deckChip, 0 }

// Hide sends the deck away until the tape is set going again or another is loaded.
func (d *tapeDeck) Hide() { d.mode, d.idle = deckAway, 0 }

// Expand makes the deck full.
func (d *tapeDeck) Expand() { d.mode, d.idle = deckFull, 0 }

// PressFromChip is a click on the chip's play button: Play when the tape is not playing, and
// Pause (which a second press lets go) when it is. The chip stays the chip: the tape starting
// is not the deck coming back.
func (d *tapeDeck) PressFromChip(t *Tape) {
	if t == nil || t.st == nil || !t.st.Loaded {
		return
	}
	if t.st.Playing {
		d.Press(t, tapePause)
	} else {
		d.Press(t, tapePlay)
	}
	d.wasPlaying = t.st.Playing
}

// Wheel takes the turn of the wheel over a frame of dt seconds and says whether it made a step:
// +1 for a turn away from the user (bigger), -1 for a turn towards, 0 for none. A step takes a
// whole notch, and the next waits tapeDeckWheelCool seconds.
func (d *tapeDeck) Wheel(delta, dt float64) int {
	if d.wheelCool -= dt; d.wheelCool < 0 {
		d.wheelCool = 0
	}
	if delta == 0 {
		return 0
	}
	if d.wheel != 0 && (delta > 0) != (d.wheel > 0) {
		d.wheel = 0 // a turn the other way drops what was not yet a step
	}
	d.wheel += delta
	if d.wheelCool > 0 {
		return 0
	}
	switch {
	case d.wheel >= 1:
		d.wheel, d.wheelCool = 0, tapeDeckWheelCool
		return 1
	case d.wheel <= -1:
		d.wheel, d.wheelCool = 0, tapeDeckWheelCool
		return -1
	}
	return 0
}

// stepDeckSize is the size one step of the wheel makes of size: dir > 0 is bigger, dir < 0
// smaller. The sizes in order are big, small and hidden (no cassette); a step smaller than the
// last says minimise, and a step bigger than the first changes nothing.
func stepDeckSize(size tapeCassetteSize, dir int) (next tapeCassetteSize, minimise bool) {
	order := []tapeCassetteSize{cassetteBig, cassetteSmall, cassetteHidden}
	i := slices.Index(order, size)
	if i < 0 {
		i = 1
	}
	i -= dir
	switch {
	case i < 0:
		return order[0], false
	case i >= len(order):
		return size, true
	}
	return order[i], false
}
