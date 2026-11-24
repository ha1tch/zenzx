// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// ============================================================================
// The two corner buttons of a full deck, and the chip
//
// A full deck has a Minimise button and a Close button at the end of the row with the counter,
// where there is room for them. The chip (tapedeck_mode.go) is the deck made small: a mini
// cassette, a play button, the counter, a close button, and a hairline of progress under them.
//
//	+-------------------------------------------+
//	| [cassette] [>] 1:23 / 4:56 [x]            |
//	| ===========-------------------------------|
//	+-------------------------------------------+
// ============================================================================

// placeTimeAndCorner puts the counter and the two corner buttons in the part of the row from tx
// to right: the counter takes what the buttons leave. A row with less room than the buttons and
// a counter of four characters or so need has no corner buttons, and the counter has the room.
func placeTimeAndCorner(l *tapeDeckLayout, tx, y, bh, right, gap int) {
	g := clampInt(bh*3/5, 10, 24)
	gap = max(gap, 3)
	if right-tx >= 6*g+2*gap {
		gy := y + (bh-g)/2
		l.Close = deckRect{right - g, gy, g, g}
		l.Minimise = deckRect{l.Close.X - gap - g, gy, g, g}
		if tw := l.Minimise.X - gap - tx; tw > 0 {
			l.Time = deckRect{tx, y, tw, bh}
		}
		return
	}
	l.Time = deckRect{tx, y, right - tx, bh}
}

// moved is r dy units lower (higher, for a negative dy).
func (r deckRect) moved(dy int) deckRect { r.Y += dy; return r }

// shifted is the layout dy units lower: the deck as it slides in or out.
func (l tapeDeckLayout) shifted(dy int) tapeDeckLayout {
	l.Panel, l.Cassette, l.Label, l.Bar = l.Panel.moved(dy), l.Cassette.moved(dy), l.Label.moved(dy), l.Bar.moved(dy)
	l.Time, l.Name, l.Minimise, l.Close = l.Time.moved(dy), l.Name.moved(dy), l.Minimise.moved(dy), l.Close.moved(dy)
	for i := range l.Buttons {
		l.Buttons[i] = l.Buttons[i].moved(dy)
	}
	return l
}

// shifted is the chip dy units lower.
func (l deckChipLayout) shifted(dy int) deckChipLayout {
	l.Panel, l.Cassette, l.Play = l.Panel.moved(dy), l.Cassette.moved(dy), l.Play.moved(dy)
	l.Time, l.Close, l.Bar = l.Time.moved(dy), l.Close.moved(dy), l.Bar.moved(dy)
	return l
}

// deckChipLayout is where the parts of the chip are. A rectangle with no size is a part that
// has no room.
type deckChipLayout struct {
	Panel    deckRect // the chip's background: a click on it makes the deck full
	Cassette deckRect // the mini cassette
	Play     deckRect // the play (or pause) button
	Time     deckRect // the counter
	Close    deckRect
	Bar      deckRect // the hairline of progress along the bottom
}

// layoutDeckChip puts the chip at the right end of area, against its top or, when bottom is
// set, its bottom. lineH is the height of a line of text and timeW the width of the counter; a
// chip that would be wider than area has no counter, and one still too wide, or too short for
// its buttons, is not laid out at all.
func layoutDeckChip(area deckRect, lineH, timeW int, bottom bool) deckChipLayout {
	var l deckChipLayout
	if area.empty() || lineH < 1 {
		return l
	}
	pad := max(2, lineH/4)
	gap := max(2, pad)
	barH := clampInt(lineH/4, 2, 4)
	rowH := max(lineH+4, 16)
	H := rowH + gap + barH + 2*pad
	if H > area.H {
		H = area.H
		rowH = H - gap - barH - 2*pad
	}
	if rowH < 10 {
		return l
	}
	cw := rowH * cassetteW / cassetteH
	pw := rowH * 5 / 4
	g := clampInt(rowH*3/5, 8, 24)
	width := func(tw int) int {
		w := 2*pad + cw + gap + pw + gap + g
		if tw > 0 {
			w += tw + gap
		}
		return w
	}
	if width(timeW) > area.W {
		timeW = 0
	}
	W := width(timeW)
	if W > area.W {
		return l
	}
	px := area.X + area.W - W
	py := area.Y
	if bottom {
		py = area.Y + area.H - H
	}
	l.Panel = deckRect{px, py, W, H}
	y := py + pad
	x := px + pad
	l.Cassette = deckRect{x, y, cw, rowH}
	x += cw + gap
	l.Play = deckRect{x, y, pw, rowH}
	x += pw + gap
	if timeW > 0 {
		l.Time = deckRect{x, y, timeW, rowH}
		x += timeW + gap
	}
	l.Close = deckRect{x, y + (rowH-g)/2, g, g}
	l.Bar = deckRect{px + pad, y + rowH + gap, W - 2*pad, barH}
	return l
}
