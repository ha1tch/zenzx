//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// The states of a button at the ends of the bar (barbuttons_gui.go): the toolbar button at the
// left, and the Help button, the burger, at the right. At rest it is a rounded rectangle with its
// picture; under the pointer it is lighter; and while it is in use (the toolbar showing, the
// Help menu open) it is pressed in, darker. The logo between the labels and the burger is
// only a picture: it is not a button, and the pointer does nothing over it.

type logoButtonState int

const (
	logoIdle logoButtonState = iota
	logoHover
	logoPressed
)

// logoButtonPad is the room between a button and the bar's edge, and between the buttons and
// what is beside them, in the usual strip.
const logoButtonPad = 4

// updateBarCursor makes the pointer a hand over a button of the bar or a toolbar button that
// can be used, and puts it back as it was when it leaves, touching the cursor only when that
// changes and never when something else has it.
func (b *appMenuBar) updateBarCursor() {
	if !rl.IsWindowReady() {
		return // no window, no pointer to change
	}
	setHandCursor(&b.logoHand, b.barButtonCursorWanted(int(rl.GetScreenWidth()), int(rl.GetMouseX()), int(rl.GetMouseY())))
}

// setHandCursor makes the pointer a hand when want is true and the arrow again when it is
// false, for the one thing that owns *hand: it touches the cursor only when want changes, so
// the arrow is only put back by whatever made the hand, and two such things that are never
// under the pointer together do not undo each other.
func setHandCursor(hand *bool, want bool) {
	if want == *hand {
		return
	}
	*hand = want
	if want {
		rl.SetMouseCursor(rl.MouseCursorPointingHand)
	} else {
		rl.SetMouseCursor(rl.MouseCursorDefault)
	}
}
