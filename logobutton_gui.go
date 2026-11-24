//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// The rainbow (Spectrum theme) or logo (Dark/Light themes) at the right of the bar is the
// Help menu's button, and behaves as one, after the Start button of the systems the
// people who use the menu come from: at rest it is the bare logo; under the pointer it is
// raised, with a light edge above and to the left and a dark one below and to the right; and
// while its menu is open it is pressed in, the edges the other way round and the face a
// little darker. The bevels are drawn over the logo in translucent white and black, so that
// they suit any theme, the Spectrum one's gradient included.

type logoButtonState int

const (
	logoIdle logoButtonState = iota
	logoHover
	logoPressed
)

const (
	// logoButtonPad is the room between the logo and the button's edge.
	logoButtonPad = 4
	// logoRightMargin is the gap zenui leaves between the logo and the window's edge.
	logoRightMargin = 8
)

// logoButton is the button's rectangle in the bar's own coordinates: the logo with a margin
// round it. ok is false when the theme drew no logo, or none that fitted, which is when the
// menu is reached from the whole of the bar to the right of the labels.
func (b *appMenuBar) logoButton(screenW int) (zenui.Rect, bool) {
	x := b.widget.DecorationX()
	if x <= 0 || x >= screenW {
		return zenui.Rect{}, false
	}
	pad := logoButtonPadFor(b.stripH())
	left := max(x-pad, b.widget.LabelsEndX())
	right := screenW - logoRightMargin + pad
	if right <= left {
		return zenui.Rect{}, false
	}
	return zenui.Rect{X: left, Y: 1, W: right - left, H: b.stripH() - 2}, true
}

// logoButtonState is how the button looks now.
func (b *appMenuBar) logoButtonState(screenW, mouseX, mouseY int) logoButtonState {
	if b.logoMenu != nil {
		return logoPressed
	}
	if r, ok := b.logoButton(screenW); ok && b.state == barShown && !b.modalOpen() && r.Contains(mouseX, mouseY) {
		return logoHover
	}
	return logoIdle
}

// drawLogoButton draws the bevel for state over the button, which sits barY pixels from where
// it would be with the bar fully shown (the bar slides in and out).
func drawLogoButton(r zenui.Renderer, rc zenui.Rect, barY int, state logoButtonState) {
	if state == logoIdle {
		return
	}
	rc.Y += barY
	light := zenui.Colour{R: 255, G: 255, B: 255, A: 210}
	dark := zenui.Colour{R: 0, G: 0, B: 0, A: 190}
	softLight := zenui.Colour{R: 255, G: 255, B: 255, A: 90}
	softDark := zenui.Colour{R: 0, G: 0, B: 0, A: 90}
	if state == logoPressed {
		light, dark = dark, light
		softLight, softDark = softDark, softLight
		r.FillRect(rc, zenui.Colour{R: 0, G: 0, B: 0, A: 45})
	}
	// The edges are as thick as the bar is tall in strips of its usual height: one pixel in the
	// usual strip, two in one half as tall again as that.
	t := logoBevelThickness(rc.H + 2)
	edges := func(box zenui.Rect, topLeft, bottomRight zenui.Colour) {
		r.FillRect(zenui.Rect{X: box.X, Y: box.Y, W: box.W, H: t}, topLeft)
		r.FillRect(zenui.Rect{X: box.X, Y: box.Y, W: t, H: box.H}, topLeft)
		r.FillRect(zenui.Rect{X: box.X, Y: box.Y + box.H - t, W: box.W, H: t}, bottomRight)
		r.FillRect(zenui.Rect{X: box.X + box.W - t, Y: box.Y, W: t, H: box.H}, bottomRight)
	}
	edges(rc, light, dark)
	inner := zenui.Rect{X: rc.X + t, Y: rc.Y + t, W: rc.W - 2*t, H: rc.H - 2*t}
	if inner.W > 2*t && inner.H > 2*t {
		edges(inner, softLight, softDark)
	}
}

// logoCursorWanted reports whether the pointer is on the button, where it is a hand: only
// there, and only while the button can be clicked -- the bar shown and no window over it.
func (b *appMenuBar) logoCursorWanted(screenW, mouseX, mouseY int) bool {
	r, ok := b.logoButton(screenW)
	return ok && b.state == barShown && !b.modalOpen() && r.Contains(mouseX, mouseY)
}

// updateLogoCursor makes the pointer a hand over the button and puts it back as it was when
// it leaves, touching the cursor only when that changes and never when something else
// has it.
func (b *appMenuBar) updateLogoCursor() {
	setHandCursor(&b.logoHand, b.logoCursorWanted(int(rl.GetScreenWidth()), int(rl.GetMouseX()), int(rl.GetMouseY())))
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

// paintLogoButton draws the button as the pointer and the menu leave it.
func (b *appMenuBar) paintLogoButton(screenW, barY int) {
	rc, ok := b.logoButton(screenW)
	if !ok {
		return
	}
	drawLogoButton(b.renderer, rc, barY, b.logoButtonState(screenW, int(rl.GetMouseX()), int(rl.GetMouseY())))
}
