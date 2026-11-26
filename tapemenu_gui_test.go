//go:build !headless

package zenzx

import (
	"testing"

	"github.com/ha1tch/zenui"
)

// tapeRows is the Tape menu's rows by label, after the bar has made them again for the tape as it is.
func tapeRows(b *appMenuBar, zx *ZenZX) map[string]zenui.Item {
	b.refreshTapeItems(zx)
	rows := map[string]zenui.Item{}
	for _, it := range b.widget.ItemsFor(barTape) {
		rows[it.Label] = it
	}
	return rows
}

// Play and Stop are rows of their own with the Machine menu's transport sprites, and each can be chosen
// only when it would do something.
func TestTapeMenuPlayAndStopHaveTheirButtons(t *testing.T) {
	b, zx := newTestBar(t, nil, "")

	rows := tapeRows(b, zx)
	if rows["Play"].Icon == nil || rows["Stop"].Icon == nil {
		t.Fatalf("Play has an icon %v, Stop has one %v", rows["Play"].Icon != nil, rows["Stop"].Icon != nil)
	}
	if rows["Rewind"].Icon == nil {
		t.Error("Rewind has no icon")
	}
	if rows["Show Info"].Icon != nil {
		t.Error("a row that is no button has an icon")
	}
	// they draw the sprites of the Machine menu's Play and Stop
	draw := func(it zenui.Item) []paintFill {
		r := &paintRenderer{}
		it.Icon(r, zenui.Rect{X: 10, Y: 10, W: 16, H: 16}, zenui.Colour{R: 1, A: 255})
		return r.fills
	}
	same := func(a, b []paintFill) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if want := draw(zenui.Item{Icon: transportIcon("play")}); len(want) == 0 || !same(draw(rows["Play"]), want) {
		t.Error("the Play row does not draw the play sprite")
	}
	if want := draw(zenui.Item{Icon: transportIcon("stop")}); len(want) == 0 || !same(draw(rows["Stop"]), want) {
		t.Error("the Stop row does not draw the stop sprite")
	}
	// and Rewind the tape deck's rewind sprite
	if want := draw(zenui.Item{Icon: transportIcon("rewind")}); len(want) == 0 || !same(draw(rows["Rewind"]), want) {
		t.Error("the Rewind row does not draw the rewind sprite")
	}

	// with no tape, neither does anything to choose
	if !rows["Play"].Disabled || !rows["Stop"].Disabled || !rows["Rewind"].Disabled {
		t.Errorf("no tape: Play disabled %v, Stop disabled %v, Rewind disabled %v, want all three",
			rows["Play"].Disabled, rows["Stop"].Disabled, rows["Rewind"].Disabled)
	}

	// a tape that is loaded and stopped can be played
	zx.tape.st.Loaded = true
	rows = tapeRows(b, zx)
	if rows["Play"].Disabled || !rows["Stop"].Disabled || rows["Rewind"].Disabled {
		t.Errorf("a stopped tape: Play disabled %v, Stop disabled %v, Rewind disabled %v, want Stop only",
			rows["Play"].Disabled, rows["Stop"].Disabled, rows["Rewind"].Disabled)
	}
	b.pickMenuRow(t, zx, barTape, "Stop") // nothing to stop: it does nothing
	if zx.tape.st.Playing {
		t.Fatal("Stop started the tape")
	}
	b.pickMenuRow(t, zx, barTape, "Play")
	if !zx.tape.st.Playing {
		t.Fatal("Play did not start the tape")
	}

	// a playing tape can be stopped, and Play is then no choice
	rows = tapeRows(b, zx)
	if !rows["Play"].Disabled || rows["Stop"].Disabled {
		t.Errorf("a playing tape: Play disabled %v, Stop disabled %v, want Stop only", rows["Play"].Disabled, rows["Stop"].Disabled)
	}
	b.pickMenuRow(t, zx, barTape, "Play") // already playing: it must not stop it
	if !zx.tape.st.Playing {
		t.Fatal("Play on a playing tape stopped it")
	}
	b.pickMenuRow(t, zx, barTape, "Stop")
	if zx.tape.st.Playing {
		t.Fatal("Stop did not stop the tape")
	}
}

// pickMenuRow runs the action of the row of the bar's menu with the label, as the dispatch would.
func (b *appMenuBar) pickMenuRow(t *testing.T, zx *ZenZX, bar int, label string) {
	t.Helper()
	for i, it := range b.widget.ItemsFor(bar) {
		if it.Label == label {
			b.actions[bar][i](zx)
			return
		}
	}
	t.Fatalf("the menu has no row %q", label)
}

// radioFills draws a radio button's icon in a 16 by 16 cell and returns what it painted.
func radioFills(chosen bool) []paintFill {
	r := &paintRenderer{}
	radioIcon(chosen)(r, zenui.Rect{X: 10, Y: 10, W: 16, H: 16}, zenui.Colour{R: 9, A: 255})
	return r.fills
}

// fillArea is the number of pixels the fills cover (rows of one pixel, none overlapping).
func fillArea(f []paintFill) int {
	n := 0
	for _, p := range f {
		n += p.rect.W * p.rect.H
	}
	return n
}

// A radio button is round: a ring in the cell, with a dot in it when chosen, inside the cell and in the
// row's colour, and the dot is the only difference between the two.
func TestRadioIconIsARingWithADotWhenChosen(t *testing.T) {
	ring, dotted := radioFills(false), radioFills(true)
	if len(ring) == 0 {
		t.Fatal("the ring drew nothing")
	}
	if fillArea(dotted) <= fillArea(ring) {
		t.Errorf("a chosen button covers %d pixels, an unchosen one %d: the dot is missing", fillArea(dotted), fillArea(ring))
	}
	for _, f := range dotted {
		if f.rect.X < 10 || f.rect.Y < 10 || f.rect.X+f.rect.W > 26 || f.rect.Y+f.rect.H > 26 || f.colour.R != 9 {
			t.Fatalf("a pixel %+v is outside the cell or in the wrong colour", f)
		}
	}
	// round, not square: the corners of the box are empty, and so is the middle of the ring
	covered := func(f []paintFill, x, y int) bool {
		for _, p := range f {
			if x >= p.rect.X && x < p.rect.X+p.rect.W && y >= p.rect.Y && y < p.rect.Y+p.rect.H {
				return true
			}
		}
		return false
	}
	// the icon is centred on (18, 18), a ring about 6 pixels in radius
	if covered(ring, 12, 12) || covered(ring, 24, 24) {
		t.Error("the corner of the ring's box is painted: it is square")
	}
	if covered(ring, 18, 18) {
		t.Error("the middle of an unchosen button is painted")
	}
	if !covered(dotted, 18, 18) {
		t.Error("the middle of a chosen button is empty")
	}
	if !covered(ring, 18, 12) || !covered(ring, 12, 18) {
		t.Error("the ring is not there at the top and the side of its circle")
	}
}

// Accurate and Fast are radio buttons: one is chosen, the one the tape is in, a tape in Turbo mode
// counts as Accurate and keeps its mode when Accurate is chosen, and with no tape neither can be chosen.
func TestTapeMenuModeRowsAreRadioButtons(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	chosen := func(it zenui.Item) bool { return fillArea(drawIcon(it)) > fillArea(radioFills(false)) }

	rows := tapeRows(b, zx)
	if rows["Accurate"].Icon == nil || rows["Fast"].Icon == nil {
		t.Fatal("the mode rows have no radio buttons")
	}
	if !rows["Accurate"].Disabled || !rows["Fast"].Disabled {
		t.Error("the modes can be chosen with no tape")
	}
	if _, ok := rows["Accurate/Fast Mode"]; ok {
		t.Error("the old single row is still there")
	}

	zx.tape.st.Loaded = true
	zx.tape.SetMode(TapeFast)
	rows = tapeRows(b, zx)
	if rows["Accurate"].Disabled || rows["Fast"].Disabled {
		t.Fatal("the modes cannot be chosen with a tape loaded")
	}
	if chosen(rows["Accurate"]) || !chosen(rows["Fast"]) {
		t.Errorf("a Fast tape: Accurate chosen %v, Fast chosen %v", chosen(rows["Accurate"]), chosen(rows["Fast"]))
	}
	b.pickMenuRow(t, zx, barTape, "Accurate")
	if zx.tape.st.Mode != TapeAccurate {
		t.Fatalf("Accurate left the mode at %v", zx.tape.st.Mode)
	}
	rows = tapeRows(b, zx)
	if !chosen(rows["Accurate"]) || chosen(rows["Fast"]) {
		t.Errorf("an Accurate tape: Accurate chosen %v, Fast chosen %v", chosen(rows["Accurate"]), chosen(rows["Fast"]))
	}
	b.pickMenuRow(t, zx, barTape, "Accurate") // already so: nothing changes
	b.pickMenuRow(t, zx, barTape, "Fast")
	if zx.tape.st.Mode != TapeFast {
		t.Fatalf("Fast left the mode at %v", zx.tape.st.Mode)
	}

	zx.tape.SetMode(TapeTurbo)
	rows = tapeRows(b, zx)
	if !chosen(rows["Accurate"]) || chosen(rows["Fast"]) {
		t.Error("a Turbo tape does not show Accurate chosen")
	}
	b.pickMenuRow(t, zx, barTape, "Accurate")
	if zx.tape.st.Mode != TapeTurbo {
		t.Errorf("choosing Accurate took a Turbo tape out of Turbo: %v", zx.tape.st.Mode)
	}
}

// drawIcon draws the row's icon in a 16 by 16 cell and returns what it painted.
func drawIcon(it zenui.Item) []paintFill {
	r := &paintRenderer{}
	it.Icon(r, zenui.Rect{X: 10, Y: 10, W: 16, H: 16}, zenui.Colour{R: 9, A: 255})
	return r.fills
}
