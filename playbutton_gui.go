//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/raycanvas"
	"github.com/ha1tch/zenui"
)

// The Play button is what a paused machine shows in the middle of its picture, where the word
// PAUSED used to be in a corner. It is the debugger's Play sprite (transportSprites, the same
// pixel art) enlarged by whole pixels, so that it stays sharp, on a button made like the dock's
// New button (dockadd_gui.go): a face that is raised at rest and a little brighter under the
// pointer, pressed in while the mouse button is held on it, and a hand for the pointer over
// it. Like New, it acts when the mouse button comes up on it, so that a press can be taken
// back by moving away before letting go. It resumes the machine, as F2 and the Machine menu's
// Pause/Resume do.

const (
	playMinScale = 4  // screen pixels per sprite pixel, at the least
	playMaxScale = 10 // and at the most
)

// playScale is how many screen pixels one pixel of the sprite is on a picture of the given
// size: the button is about a quarter of the picture's shorter side, in whole pixels.
func playScale(w, h float32) int {
	short := w
	if h < short {
		short = h
	}
	s := int(short / 4 / 16)
	if s < playMinScale {
		s = playMinScale
	}
	if s > playMaxScale {
		s = playMaxScale
	}
	return s
}

// playButtonRect is the button's face, centred in area: the sprite's 16-pixel cell, enlarged.
func playButtonRect(area rl.Rectangle) rl.Rectangle {
	side := float32(16 * playScale(area.Width, area.Height))
	return rl.Rectangle{
		X:      area.X + (area.Width-side)/2,
		Y:      area.Y + (area.Height-side)/2,
		Width:  side,
		Height: side,
	}
}

// playButtonArea is the picture of the machine in front: its cell when the window is tiled,
// the window below the reserved top otherwise.
func playButtonArea(zx *ZenZX, deck *machineDeck) rl.Rectangle {
	if deck != nil && deck.tiled() {
		if rects := deck.tileRects(); deck.front >= 0 && deck.front < len(rects) {
			return rects[deck.front]
		}
	}
	top := float32(zx.window().reservedTopHeight)
	return rl.Rectangle{X: 0, Y: top, Width: float32(rl.GetScreenWidth()), Height: float32(rl.GetScreenHeight()) - top}
}

type playButton struct {
	icon   [16]iconRow
	iconOK bool

	hover, down, hand bool

	round      *raycanvas.Context
	roundCache *raycanvas.SharedCache
}

func newPlayButton() *playButton {
	b := &playButton{}
	b.icon, b.iconOK = parseTransportSprites()["play"]
	return b
}

// Update is called once a frame. shown says the machine is paused and its picture is on the
// screen; owned says something else has the pointer (a menu, the dock, the debugger, a
// dialog). It reports whether the button was clicked: pressed and released on it.
func (b *playButton) Update(area rl.Rectangle, shown, owned bool) (clicked bool) {
	b.hover = false
	if shown && !owned {
		b.hover = rl.CheckCollisionPointRec(rl.GetMousePosition(), playButtonRect(area))
	}
	if b.hover && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		b.down = true
	}
	if b.down && rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		clicked = b.hover
		b.down = false
	}
	if !rl.IsMouseButtonDown(rl.MouseButtonLeft) || !shown {
		b.down = false
	}
	setHandCursor(&b.hand, b.hover)
	return clicked
}

func (b *playButton) canvas() *raycanvas.Context {
	if b.round == nil {
		b.roundCache = raycanvas.NewSharedCache()
		b.round = raycanvas.NewContext(int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), b.roundCache)
		b.round.SetAntialias(true)
	}
	return b.round
}

// Draw paints the button over the picture in area, inside the frame being drawn.
func (b *playButton) Draw(area rl.Rectangle, theme zenui.Theme) {
	at := playButtonRect(area)
	radius := pictureRadius(at)
	edge, thick := theme.Text, 2
	var top, bottom color.RGBA
	pressed := b.hover && b.down
	switch {
	case pressed:
		top, bottom = color.RGBA{R: 0, G: 0, B: 0, A: 95}, color.RGBA{R: 255, G: 255, B: 255, A: 25}
	case b.hover:
		top, bottom = color.RGBA{R: 255, G: 255, B: 255, A: 85}, color.RGBA{R: 0, G: 0, B: 0, A: 25}
	default:
		top, bottom = color.RGBA{R: 255, G: 255, B: 255, A: 45}, color.RGBA{R: 0, G: 0, B: 0, A: 45}
		edge = theme.DimText
	}
	c := b.canvas()
	// The picture behind is anything, so the face is first made solid enough to read against it;
	// the New button has the dock's panel for that.
	base := rlColour(theme.Panel)
	base.A = 215
	c.SetFillColor(base)
	c.FillRoundRect(at.X, at.Y, at.Width, at.Height, radius)
	c.FillRoundRectGradient(at.X, at.Y, at.Width, at.Height, radius, top, bottom)
	c.SetStrokeColor(rlColour(edge))
	c.SetLineWidth(float32(thick))
	c.StrokeRoundRect(at.X, at.Y, at.Width, at.Height, radius)
	if pressed {
		at.X++
		at.Y++
	}
	if !b.iconOK {
		return
	}
	// The sprite is enlarged by whole pixels: every pixel a square, none blurred.
	s := int(at.Width) / 16
	x0, y0, x1, y1 := iconBounds(b.icon)
	left := int(at.X+at.Width/2) - (x1-x0)*s/2
	topY := int(at.Y+at.Height/2) - (y1-y0)*s/2
	ink := rlColour(theme.Text)
	for _, run := range iconRuns(b.icon) {
		if run.ink {
			rl.DrawRectangle(int32(left+(run.x-x0)*s), int32(topY+(run.y-y0)*s), int32(run.n*s), int32(s), ink)
		}
	}
}
