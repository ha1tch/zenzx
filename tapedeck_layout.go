// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// ============================================================================
// Where the parts of the tape deck go
//
// The arithmetic of the deck's layout, kept apart from the window so that it can be tested at
// any size: given the room the deck has (a band of the border, or a patch of the picture) and
// the height of a line of text, where the cassette, its label, the bar, the buttons and the
// counter go. The drawing is tapedeck_gui.go.
//
//	+-----------------------------------------------+
//	|  +-----------+  [Play][Rew][Stop][Pause]  1:23/4:56
//	|  | cassette  |  ===========-----------------  |
//	|  +-----------+                                |
//	+-----------------------------------------------+
//
// ============================================================================

// deckRect is a rectangle in the window's drawing units.
type deckRect struct{ X, Y, W, H int }

func (r deckRect) empty() bool { return r.W <= 0 || r.H <= 0 }

// Contains reports whether the point is inside the rectangle.
func (r deckRect) Contains(px, py float32) bool {
	return !r.empty() && px >= float32(r.X) && px < float32(r.X+r.W) &&
		py >= float32(r.Y) && py < float32(r.Y+r.H)
}

// tapeDeckLayout is where everything is. A rectangle with no size is a part that has no room and
// is not drawn.
type tapeDeckLayout struct {
	Panel    deckRect              // the deck's background
	Cassette deckRect              // the picture
	Label    deckRect              // the cassette's label, on the picture
	Bar      deckRect              // the progress bar's track
	Time     deckRect              // the counter, elapsed over the tape's length
	Name     deckRect              // the program's name, when the cassette's label is too small to read
	Buttons  [tapeButtons]deckRect // the buttons, from the left
	Minimise deckRect              // the button that makes the deck the chip, when there is room
	Close    deckRect              // the button that sends the deck away

	// LabelOnCassette is whether the cassette is large enough for the program's name to be
	// written on its label and read there. When it is not, the name goes in Name.
	LabelOnCassette bool
}

// Layout limits, as multiples of the height of a line of text.
const (
	deckMaxHeightLines = 12 // the deck is never taller than this many lines
	deckRightLines     = 17 // the room the bar and the buttons would like, in lines
	// A label is readable when the smallest line of text is at most this much of its height.
	deckLabelFill = 1.0
)

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// layoutTapeDeck lays the deck out in area, centred across it, with lineH the height of a line
// of the text drawn on it. The deck is as tall as the area, up to deckMaxHeightLines lines.
func layoutTapeDeck(area deckRect, lineH int) tapeDeckLayout {
	return layoutTapeDeckMin(area, lineH, lineH)
}

// layoutTapeDeckMin is layoutTapeDeck for text that can be drawn in more than one size: lineH
// is the size the text is drawn in elsewhere, and minLineH the smallest it can be drawn in,
// which is what the title on the cassette's label has to fit (it is written there as large as
// the label allows, at least that).
func layoutTapeDeckMin(area deckRect, lineH, minLineH int) tapeDeckLayout {
	var l tapeDeckLayout
	if minLineH < 1 {
		minLineH = 1
	}
	if lineH < 1 {
		lineH = 1
	}
	if area.empty() {
		return l
	}
	H := area.H
	if m := deckMaxHeightLines * lineH; H > m {
		H = m
	}
	pad := max(2, H/12)
	ch := H - 2*pad
	if ch < 4 {
		return l
	}
	cw := ch * cassetteW / cassetteH

	// How wide the panel would like to be, and what it is given.
	wantRight := deckRightLines * lineH
	W := 2*pad + cw + pad + wantRight
	if W > area.W {
		W = area.W
	}
	// A narrow area gives the cassette no more than 2/5 of the room, and the picture is
	// kept in proportion by making it shorter.
	if inner := W - 2*pad; cw > inner*2/5 {
		cw = inner * 2 / 5
		ch = cw * cassetteH / cassetteW
	}
	if cw < 4 || ch < 4 {
		return l
	}
	px := area.X + (area.W-W)/2
	py := area.Y + (area.H-H)/2
	l.Panel = deckRect{px, py, W, H}
	l.Cassette = deckRect{px + pad, py + (H-ch)/2, cw, ch}
	// The label's rectangle is in the cassette's design space (cassette_vector.go).
	sx, sy := float64(cw)/cassetteW, float64(ch)/cassetteH
	l.Label = deckRect{
		X: l.Cassette.X + int(cassetteLabelX0*sx+0.5),
		Y: l.Cassette.Y + int(cassetteLabelY0*sy+0.5),
		W: int((cassetteLabelX1-cassetteLabelX0)*sx + 0.5),
		H: int((cassetteLabelY1-cassetteLabelY0)*sy + 0.5),
	}
	l.LabelOnCassette = float64(minLineH) <= float64(l.Label.H)*deckLabelFill

	// The right-hand part: [name], buttons and counter, bar.
	rx := l.Cassette.X + cw + pad
	rw := px + W - pad - rx
	if rw < 8 {
		return l
	}
	barH := clampInt(H/10, 4, 12)
	bh := clampInt(H*2/5, 14, 44)
	gap := max(1, pad/2)

	rows := bh + gap + barH
	withName := !l.LabelOnCassette && rows+gap+lineH <= ch
	if withName {
		rows += gap + lineH
	}
	if rows > H-2*pad { // too little height: the buttons give way
		bh = max(8, H-2*pad-gap-barH)
		rows = bh + gap + barH
		withName = false
	}
	y := py + (H-rows)/2
	if withName {
		l.Name = deckRect{rx, y, rw, lineH}
		y += lineH + gap
	}
	// The buttons are sized to the room; when it is short they get narrower together.
	bw := bh * 5 / 4
	n := int(tapeButtons)
	if avail := rw * 3 / 5; bw*n+gap*(n-1) > avail && avail > 0 {
		bw = max(6, (avail-gap*(n-1))/n)
	}
	bx := rx
	for i := range l.Buttons {
		l.Buttons[i] = deckRect{bx, y, bw, bh}
		bx += bw + gap
	}
	if tx := bx + pad; tx < rx+rw {
		placeTimeAndCorner(&l, tx, y, bh, rx+rw, gap)
	}
	l.Bar = deckRect{rx, y + bh + gap, rw, barH}
	return l
}
