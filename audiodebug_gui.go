//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// The audio debug overlay (View's diagnostics, DisplayManager.renderDebugOverlay) is drawn
// through zenui's renderer and in the interface font, as the rest of the interface is: no
// raylib text and no raylib rectangles. It shows the audio buffer's level as a bar across the
// window under the menu bar, the speaker and CPU counters under it, and the buffer's health
// at the foot of the window.

var (
	overlayWhite  = zenui.Colour{R: 255, G: 255, B: 255, A: 255}
	overlaySky    = zenui.Colour{R: 102, G: 191, B: 255, A: 255}
	overlayRed    = zenui.Colour{R: 230, G: 41, B: 55, A: 255}
	overlayYellow = zenui.Colour{R: 253, G: 249, B: 0, A: 255}
	overlayGreen  = zenui.Colour{R: 0, G: 228, B: 48, A: 255}
)

// audioHealth is how a buffer level reads: the colour of its bar and the word for it.
func audioHealth(level float32) (zenui.Colour, string) {
	switch {
	case level < 30:
		return overlayRed, "UNDERRUN WARNING" // an underrun is likely
	case level < 60:
		return overlayYellow, "LOW"
	}
	return overlayGreen, "GOOD"
}

// drawAudioDebug draws the overlay for a window screenW by screenH: level is the buffer's
// fill in percent, and bufferText, speakerText and cycleText the three lines of figures. It
// draws nothing before the front end has said which font the interface uses.
func drawAudioDebug(screenW, screenH int, level float32, bufferText, speakerText, cycleText string) {
	if uiText == nil {
		return
	}
	t := uiText()
	if t == nil {
		return
	}
	r := zenuiraylib.Renderer{Text: t}
	// A line is drawn at the interface's size, one notch smaller when it would not fit the window.
	line := func(s string, x, y int, c zenui.Colour) {
		scale := uiTextScale
		if t.Measure(s, scale)+2*x > screenW {
			scale = 1
		}
		r.DrawText(s, x, y, scale, c)
	}

	const barY, barH = 40, 20 // below the top-left corner, where the status text was
	r.FillRect(zenui.Rect{X: 0, Y: barY, W: screenW, H: barH}, zenui.Colour{A: 128})
	colour, word := audioHealth(level)
	fill := int(float32(screenW) * level / 100)
	fill = max(0, min(fill, screenW))
	r.FillRect(zenui.Rect{X: 0, Y: barY, W: fill, H: barH}, colour)
	line(bufferText, 5, barY+2, overlayWhite)

	y := barY + barH + 5
	line(speakerText, 5, y, overlaySky)
	line(cycleText, 5, y+t.CellH()*uiTextScale+2, overlaySky)

	line("Buffer Health: "+word, 5, screenH-40, colour)
}
