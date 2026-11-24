// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// ============================================================================
// The cassette's size
//
// Preferences > Machines > Tape > Cassette: the cassette is small, beside the buttons (the deck
// as tapedeck_layout.go lays it out), big, under the buttons and the bar with the title written
// large on its label, or hidden, leaving the buttons, the bar and the counter.
// ============================================================================

// tapeCassetteSize is how the cassette is shown in the tape deck.
type tapeCassetteSize int

const (
	cassetteSmall tapeCassetteSize = iota
	cassetteBig
	cassetteHidden
)

// tapeCassetteWords are the sizes as settings.json spells them, in the order of the constants.
var tapeCassetteWords = [...]string{"small", "big", "hidden"}

func (s tapeCassetteSize) String() string {
	if s < 0 || int(s) >= len(tapeCassetteWords) {
		return tapeCassetteWords[cassetteSmall]
	}
	return tapeCassetteWords[s]
}

// parseTapeCassetteSize reads a settings word, returning def for one it does not know.
func parseTapeCassetteSize(w string, def tapeCassetteSize) tapeCassetteSize {
	for i, x := range tapeCassetteWords {
		if w == x {
			return tapeCassetteSize(i)
		}
	}
	return def
}

// The room a deck of each size wants in a band of the border, in window units: below it the
// deck goes over the picture instead.
const (
	deckBigMinBand  = 150 // a big cassette needs a tall band; the border is never this thick below zoom 5
	deckHiddenLines = 22  // the width of a deck with no cassette, in lines of text
)

// cassetteLabelRect is the label's rectangle on a cassette drawn into c.
func cassetteLabelRect(c deckRect) deckRect {
	sx, sy := float64(c.W)/cassetteW, float64(c.H)/cassetteH
	return deckRect{
		X: c.X + int(cassetteLabelX0*sx+0.5),
		Y: c.Y + int(cassetteLabelY0*sy+0.5),
		W: int((cassetteLabelX1-cassetteLabelX0)*sx + 0.5),
		H: int((cassetteLabelY1-cassetteLabelY0)*sy + 0.5),
	}
}

// buttonRow puts the four buttons in a row from x, at y, h high, as wide as the room allows up to
// 5/4 of their height, and says where the row ends.
func buttonRow(l *tapeDeckLayout, x, y, h, gap, room int) (end int) {
	n := int(tapeButtons)
	bw := h * 5 / 4
	if bw*n+gap*(n-1) > room && room > 0 {
		bw = max(6, (room-gap*(n-1))/n)
	}
	for i := range l.Buttons {
		l.Buttons[i] = deckRect{x, y, bw, h}
		x += bw + gap
	}
	return x - gap
}

// layoutTapeDeckSized is the deck in area for a cassette of the given size. A small cassette is
// layoutTapeDeckMin's; the others fit their own height, against the top of area or, when bottom
// is set, its bottom.
func layoutTapeDeckSized(area deckRect, lineH, minLineH int, size tapeCassetteSize, bottom bool) tapeDeckLayout {
	switch size {
	case cassetteBig:
		return layoutBigCassette(area, minLineH, bottom)
	case cassetteHidden:
		return layoutNoCassette(area, lineH, bottom)
	}
	return layoutTapeDeckMin(area, lineH, minLineH)
}

// layoutBigCassette puts the buttons and the counter in a row, the bar under it, and under that
// the cassette, as large as area allows with its proportions kept; the title is written on its
// label, so there is no row for the name.
func layoutBigCassette(area deckRect, minLineH int, bottom bool) tapeDeckLayout {
	var l tapeDeckLayout
	if area.empty() {
		return l
	}
	if minLineH < 1 {
		minLineH = 1
	}
	H := area.H
	bh := clampInt(H/9, 14, 44)
	pad := max(2, H/50)
	gap := pad
	barH := clampInt(bh/3, 4, 12)
	fixed := 2*pad + bh + gap + barH + gap
	ch := H - fixed
	if ch < 8 {
		return l
	}
	cw := ch * cassetteW / cassetteH
	if maxw := area.W - 2*pad; cw > maxw {
		cw = maxw
		ch = cw * cassetteH / cassetteW
	}
	if cw < 8 || ch < 8 {
		return l
	}
	W, Hp := cw+2*pad, fixed+ch
	px := area.X + (area.W-W)/2
	py := area.Y
	if bottom {
		py = area.Y + area.H - Hp
	}
	l.Panel = deckRect{px, py, W, Hp}
	y := py + pad
	end := buttonRow(&l, px+pad, y, bh, gap, cw*3/5)
	if tx := end + pad; tx < px+W-pad {
		placeTimeAndCorner(&l, tx, y, bh, px+W-pad, gap)
	}
	y += bh + gap
	l.Bar = deckRect{px + pad, y, cw, barH}
	y += barH + gap
	l.Cassette = deckRect{px + pad, y, cw, ch}
	l.Label = cassetteLabelRect(l.Cassette)
	l.LabelOnCassette = minLineH <= l.Label.H
	return l
}

// layoutNoCassette is the deck with no cassette: the name of the program in a row of its own
// when there is the height for it, the buttons and the counter, and the bar.
func layoutNoCassette(area deckRect, lineH int, bottom bool) tapeDeckLayout {
	var l tapeDeckLayout
	if area.empty() {
		return l
	}
	if lineH < 1 {
		lineH = 1
	}
	H := area.H
	bh := clampInt(H*2/5, 14, 44)
	pad := max(2, bh/6)
	gap := max(1, pad/2)
	barH := clampInt(bh/4, 4, 12)
	rows := bh + gap + barH
	name := rows+gap+lineH+2*pad <= H
	if name {
		rows += gap + lineH
	}
	if rows+2*pad > H { // too little height: the buttons give way
		bh = max(10, H-2*pad-gap-barH)
		rows = bh + gap + barH
		name = false
	}
	W := min(area.W, 2*pad+deckHiddenLines*lineH)
	Hp := rows + 2*pad
	if W < 8*pad || Hp > H {
		return l
	}
	px := area.X + (area.W-W)/2
	py := area.Y
	if bottom {
		py = area.Y + area.H - Hp
	}
	l.Panel = deckRect{px, py, W, Hp}
	y := py + pad
	inner := W - 2*pad
	if name {
		l.Name = deckRect{px + pad, y, inner, lineH}
		y += lineH + gap
	}
	end := buttonRow(&l, px+pad, y, bh, gap, inner*3/5)
	if tx := end + pad; tx < px+W-pad {
		placeTimeAndCorner(&l, tx, y, bh, px+W-pad, gap)
	}
	l.Bar = deckRect{px + pad, y + bh + gap, inner, barH}
	return l
}
