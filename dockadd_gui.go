//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// The dock's New button is a button, not a frame: it has a face that is raised at rest and a
// little brighter under the pointer, pressed in (the face darker, the shading the other way
// up, the picture a pixel down and to the right) while the mouse button is held on it, and
// greyed with no hand when adding machines is off in Preferences. It rises to the pointer
// as the machines' pictures do, and is drawn among them in the same order. It acts when the
// mouse button comes up on it, not when it goes down, so that a press can be taken back by
// moving away before letting go.

// addTileState is how the New button looks now.
type addTileState int

const (
	addIdle addTileState = iota
	addHover
	addPressed
	addOff
)

// addMagnify is how far the New button has grown: 1 until the dock has shown it.
func (d *machineDeck) addMagnify() float32 {
	if d.addMag == 0 {
		return 1
	}
	return d.addMag
}

// addState is how the New button looks.
func (d *machineDeck) addState() addTileState {
	switch {
	case !d.multiAllowed():
		return addOff
	case d.addHover && d.addDown:
		return addPressed
	case d.addHover:
		return addHover
	}
	return addIdle
}

// addCursorWanted reports whether the pointer should be a hand: it is on the New button
// (hover is the tile under it) and the button can be used -- the dock shown and still, no
// name being typed, no picture being dragged, nothing else holding the pointer.
func (d *machineDeck) addCursorWanted(hover int, busy bool) bool {
	return !busy && d.state == barShown && !d.flying && d.edit == nil && d.pressOn < 0 &&
		hover == len(d.machines) && d.multiAllowed()
}

// drawAddTile draws the New button in at, selected being whether the keyboard has it.
func (d *machineDeck) drawAddTile(r zenui.Renderer, at rl.Rectangle, selected bool, theme zenui.Theme) {
	state := d.addState()
	radius := pictureRadius(at)
	edge, thick := theme.DimText, 1
	var top, bottom color.RGBA
	switch state {
	case addPressed:
		top, bottom = color.RGBA{R: 0, G: 0, B: 0, A: 95}, color.RGBA{R: 255, G: 255, B: 255, A: 25}
		edge = theme.Text
	case addHover:
		top, bottom = color.RGBA{R: 255, G: 255, B: 255, A: 85}, color.RGBA{R: 0, G: 0, B: 0, A: 25}
		edge = theme.Text
	case addOff:
		edge = theme.Disabled
	default:
		top, bottom = color.RGBA{R: 255, G: 255, B: 255, A: 45}, color.RGBA{R: 0, G: 0, B: 0, A: 45}
	}
	if selected && state != addOff {
		edge, thick = theme.SelFill, 2
	}
	if state != addOff {
		d.canvas().FillRoundRectGradient(at.X, at.Y, at.Width, at.Height, radius, top, bottom)
	}
	d.roundFrame(at, radius, edge, thick)
	if state == addPressed {
		at.X++
		at.Y++
	}
	drawAddIcon(r, at)
}
