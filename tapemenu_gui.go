//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// The Tape menu is in three titled sections: Playback, with Play, Stop and Rewind (each with its button, the sprites of the Machine menu and of the tape deck); Mode, with
// Accurate and Fast; and Information. Play and Stop are rows of their own, each with the transport
// sprite the Machine menu's Pause, Play and Stop have (transportIcon), and each can be chosen only
// when it would do something: Play when a tape is loaded and is not playing, Stop while it plays.
// Alt+P still plays and stops with one key (PlayStopTape).
//
// The loading mode is a choice of one of two, Accurate or Fast, so it is two rows that are radio
// buttons: a round ring, with a dot in the one in force (radioIcon). A tape in Turbo mode (chosen
// with -tapemode turbo) loads as Accurate does, so it shows Accurate chosen, and choosing
// Accurate then leaves it as it is. With no tape loaded neither can be chosen, as Alt+T does nothing
// then. Alt+T still switches with one key (ToggleTapeMode).

import (
	"fmt"
	"math"

	"github.com/ha1tch/zenui"
)

// radioIcon is the Item.Icon of a radio button: a ring, round so that it cannot be taken for a
// checkbox, and in the one that is chosen a dot inside it, in the colour of the row. It is drawn the
// way zenui's RadioGroup draws its buttons: four fifths of a line high, a ring an eighth of that
// thick, and a dot of a little over half the ring's radius, centred in the cell the row gives it.
func radioIcon(chosen bool) func(r zenui.Renderer, cell zenui.Rect, col zenui.Colour) {
	return func(r zenui.Renderer, cell zenui.Rect, col zenui.Colour) {
		size := cell.W * 8 / 10
		if size < 8 {
			size = 8
		}
		if size > cell.H {
			size = cell.H
		}
		thick := size / 8
		if thick < 2 {
			thick = 2
		}
		cx := float64(cell.X) + float64(cell.W)/2
		cy := float64(cell.Y) + float64(cell.H)/2
		outer := float64(size) / 2
		inner := outer - float64(thick)
		dot := outer * 0.55
		// half is the half width of the circle of radius rad at height dy from its centre, or -1
		// where the row is outside it
		half := func(rad, dy float64) float64 {
			if dy*dy > rad*rad {
				return -1
			}
			return math.Sqrt(rad*rad - dy*dy)
		}
		span := func(y int, x0, x1 float64) {
			l, rt := int(math.Round(cx-x1)), int(math.Round(cx+x1))
			if x0 > 0 { // two pieces, either side of the hollow
				li, ri := int(math.Round(cx-x0)), int(math.Round(cx+x0))
				if li > l {
					r.FillRect(zenui.Rect{X: l, Y: y, W: li - l, H: 1}, col)
				}
				if rt > ri {
					r.FillRect(zenui.Rect{X: ri, Y: y, W: rt - ri, H: 1}, col)
				}
				return
			}
			if rt > l {
				r.FillRect(zenui.Rect{X: l, Y: y, W: rt - l, H: 1}, col)
			}
		}
		for y := int(math.Floor(cy - outer)); y <= int(math.Ceil(cy+outer)); y++ {
			dy := float64(y) + 0.5 - cy
			o := half(outer, dy)
			if o < 0 {
				continue
			}
			if i := half(inner, dy); i >= 0 {
				span(y, i, o) // the ring: what is left of the outer circle outside the inner one
			} else {
				span(y, 0, o) // above and below the hollow, the ring is whole
			}
			if chosen {
				if d := half(dot, dy); d >= 0 {
					span(y, 0, d)
				}
			}
		}
	}
}

// tapeAccurate says the tape in zx's deck loads in Accurate mode, which Turbo is a kind of.
func tapeAccurate(zx *ZenZX) bool {
	return zx.tape != nil && zx.tape.st != nil && (zx.tape.st.Mode == TapeAccurate || zx.tape.st.Mode == TapeTurbo)
}

// chooseTapeMode is what the Accurate and Fast rows do: it makes mode the tape's, unless it is the
// one in force already (Turbo counts as Accurate and is kept), or no tape is loaded.
func chooseTapeMode(zx *ZenZX, accurate bool) {
	if !tapeLoaded(zx) || tapeAccurate(zx) == accurate {
		return
	}
	if accurate {
		zx.tape.SetMode(TapeAccurate)
		fmt.Println("Tape: Accurate mode")
	} else {
		zx.tape.SetMode(TapeFast)
		fmt.Println("Tape: Fast mode")
	}
}

// tapePlaying says a tape is loaded in zx's deck and is playing.
func tapePlaying(zx *ZenZX) bool {
	return zx.tape != nil && zx.tape.st != nil && zx.tape.st.Loaded && zx.tape.st.Playing
}

// tapeLoaded says zx's deck has a tape in it.
func tapeLoaded(zx *ZenZX) bool {
	return zx.tape != nil && zx.tape.st != nil && zx.tape.st.Loaded
}

// tapeSpec is the Tape menu as zx's tape is now.
func (b *appMenuBar) tapeSpec(zx *ZenZX) menuSpec {
	playing := tapePlaying(zx)
	return menuSpec{
		menuTitle("Playback"),
		{label: "Play", icon: transportIcon("play"), disabled: playing || !tapeLoaded(zx), run: func(zx *ZenZX) {
			if !tapePlaying(zx) {
				zx.PlayStopTape()
			}
		}},
		{label: "Stop", icon: transportIcon("stop"), disabled: !playing, run: func(zx *ZenZX) {
			if tapePlaying(zx) {
				zx.PlayStopTape()
			}
		}},
		{label: "Rewind", icon: transportIcon("rewind"), disabled: !tapeLoaded(zx), run: func(zx *ZenZX) { zx.RewindTape() }},
		menuSep(),
		menuTitle("Mode"),
		{label: "Accurate", icon: radioIcon(tapeAccurate(zx)), disabled: !tapeLoaded(zx), run: func(zx *ZenZX) { chooseTapeMode(zx, true) }},
		{label: "Fast", icon: radioIcon(!tapeAccurate(zx)), disabled: !tapeLoaded(zx), run: func(zx *ZenZX) { chooseTapeMode(zx, false) }},
		menuSep(),
		menuTitle("Information"),
		menuRow("Show Info", func(zx *ZenZX) { b.showInfo("TAPE INFO", tapeInfoText(zx)) }), // info_gui.go
	}
}

// refreshTapeItems makes the Tape menu again for the tape as it is now, each frame the bar is shown
// (the way the View and Machine menus are): a tape can start or end on its own, or by Alt+P.
func (b *appMenuBar) refreshTapeItems(zx *ZenZX) {
	spec := b.tapeSpec(zx)
	b.actions[barTape] = spec.actions()
	b.widget.SetItems(barTape, spec.items())
}
