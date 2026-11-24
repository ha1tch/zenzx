//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"strconv"
	"strings"

	"github.com/ha1tch/zenui"
)

// The Preferences window's help text is hidden until it is asked for. A row's hint, or the
// reason it cannot be changed now, shows in a small box beside the pointer when the pointer
// rests on the row's control or its caption; resting it on a group's title shows the hints of
// every row in the group. The box comes after a short pause, goes when the pointer moves to
// something else, and stays away after a click until the pointer has moved off what it
// clicked. The window also learns here whether the pointer is over a tab, which the menu bar
// turns into the hand cursor.

// prefsTipDelay is how long, in seconds, the pointer rests on something before its box shows.
const prefsTipDelay = 0.35

// prefsTip is what the pointer is resting on.
type prefsTip struct {
	key    string     // which row or group, "" for nothing that has help text
	text   string     // what the box would say
	anchor zenui.Rect // the thing it is about; the box goes under it, or over it for want of room
	x      int        // where the pointer is, across: the box starts there
	dwell  float32    // seconds the pointer has been on key
	hidden bool       // a click on key hid its box
}

// rowNote is the help text of a row: why it cannot be changed now, when that is so,
// and otherwise its hint.
func rowNote(row prefRow) string {
	if row.off != nil {
		if why := row.off(); why != "" {
			return why
		}
	}
	return row.hint
}

// groupNote is the help text of a group: the note of each of its rows that has one, each
// starting with the row's caption and set apart from the next by a blank line.
func groupNote(s prefSection) string {
	var lines []string
	for _, row := range s.rows {
		if n := rowNote(row); n != "" {
			lines = append(lines, row.label+": "+n)
		}
	}
	return strings.Join(lines, "\n\n")
}

// tipTarget is what is at the point (x, y) of the tab that is open and has help text: its key,
// its text and the rectangle it is about. A group's title answers for the group; a row is
// answered for by its caption and by the part of its control that answers a click, and not by
// the empty space beside them.
func (w *prefsWindow) tipTarget(x, y int, textW func(string) int) (key, text string, anchor zenui.Rect) {
	for si, s := range w.sections {
		if !w.showing(si) {
			continue
		}
		if h := w.heads[si]; h.Contains(x, y) {
			if n := groupNote(s); n != "" {
				return "g" + strconv.Itoa(si), n, h
			}
			return "", "", zenui.Rect{}
		}
		for ri, row := range s.rows {
			n := rowNote(row)
			if n == "" {
				continue
			}
			area := w.areas[si][ri]
			var hit zenui.Rect
			found := false
			try := func(r zenui.Rect) {
				if !found && r.Contains(x, y) {
					hit, found = r, true
				}
			}
			if g := w.radios[si][ri]; g != nil {
				try(w.labels[si][ri])
				rects, _ := g.Layout(area, w.lineH, textW)
				for _, r := range rects {
					try(r)
				}
			}
			if c := w.checks[si][ri]; c != nil {
				try(c.Bounds(area, w.lineH, textW))
			}
			if found {
				if w.labels[si][ri].W > 0 {
					hit = zenui.Rect{X: w.labels[si][ri].X, Y: hit.Y, W: hit.X + hit.W - w.labels[si][ri].X, H: hit.H}
				}
				return "r" + strconv.Itoa(si) + "." + strconv.Itoa(ri), n, hit
			}
		}
	}
	return "", "", zenui.Rect{}
}

// hover reads the pointer for the frame: whether it is on a tab, and how long it has rested on
// something with help text. The window must have been laid out.
func (w *prefsWindow) hover(in zenui.Input, textW func(string) int) {
	w.handWanted = false
	for _, t := range w.tabs.Layout(w.tabArea, w.lineH) {
		if t.Contains(in.MouseX, in.MouseY) {
			w.handWanted = true
		}
	}
	key, text, anchor := w.tipTarget(in.MouseX, in.MouseY, textW)
	if key != w.tip.key {
		w.tip = prefsTip{key: key}
	}
	w.tip.text, w.tip.anchor, w.tip.x = text, anchor, in.MouseX
	if key == "" {
		return
	}
	if in.MousePressed {
		w.tip.hidden = true
	}
	w.tip.dwell += in.DeltaTime
}

// updatePrefsCursor makes the pointer a hand while it is over a tab of the Preferences window,
// and the arrow again when it leaves one or the window closes.
func (b *appMenuBar) updatePrefsCursor() {
	setHandCursor(&b.prefsHand, b.prefs != nil && b.prefs.handWanted)
}

// tipShowing says whether the help box is on screen this frame.
func (w *prefsWindow) tipShowing() bool {
	t := w.tip
	return t.key != "" && t.text != "" && !t.hidden && t.dwell >= prefsTipDelay
}

// wrapPx breaks text into lines no wider than maxW, at spaces, a paragraph (a "\n") at a
// time. A word wider than maxW is kept whole on a line of its own.
func wrapPx(text string, maxW int, textW func(string) int) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			try := word
			if line != "" {
				try = line + " " + word
			}
			if line != "" && textW(try) > maxW {
				out = append(out, line)
				line = word
				continue
			}
			line = try
		}
		out = append(out, line)
	}
	return out
}

// placeTip is where a box of bw by bh goes for a thing at anchor and a pointer at x: under
// the thing, starting at the pointer, or over it when there is no room under, and always
// inside the screen.
func placeTip(anchor zenui.Rect, x, bw, bh, screenW, screenH int) zenui.Rect {
	const gap = 4
	px := min(x, screenW-gap-bw)
	px = max(px, gap)
	py := anchor.Y + anchor.H + gap
	if py+bh > screenH-gap {
		py = anchor.Y - bh - gap
	}
	py = max(0, min(py, screenH-bh))
	return zenui.Rect{X: px, Y: py, W: bw, H: bh}
}

// tipBox is where the help box goes, its lines, and its padding.
func (w *prefsWindow) tipBox(r zenui.Renderer, screenW, screenH int) (box zenui.Rect, lines []string, pad int) {
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	pad = max(4, w.lineH/2)
	lines = wrapPx(w.tip.text, max(10*w.lineH, w.panel.W*3/5)-2*pad, textW)
	widest := 0
	for _, l := range lines {
		widest = max(widest, textW(l))
	}
	box = placeTip(w.tip.anchor, w.tip.x, widest+2*pad, len(lines)*w.lineH+2*pad, screenW, screenH)
	return box, lines, pad
}

// drawTip draws the help box, when there is one to show, over everything else.
func (w *prefsWindow) drawTip(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	if !w.tipShowing() {
		return
	}
	box, lines, pad := w.tipBox(r, screenW, screenH)
	// The box is the window's colours the other way round, so that it stands out of the panel
	// it lies on in every theme.
	r.FillRect(box, theme.Text)
	r.StrokeRect(box, theme.Border, max(1, theme.BorderThickness))
	for i, l := range lines {
		r.DrawText(l, box.X+pad, box.Y+pad+i*w.lineH, w.scale, theme.Panel)
	}
}
