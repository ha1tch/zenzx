//go:build !headless

package zenzx

import (
	"testing"

	"github.com/ha1tch/zenui"
)

// The backdrop, the rectangle over the whole screen, is held back; everything else is drawn.
func TestBackdropCatcherKeepsTheBackdropToItself(t *testing.T) {
	rec := &paintRenderer{}
	c := &backdropCatcher{Renderer: rec, w: 800, h: 600}
	back := zenui.Colour{R: 1, G: 2, B: 3, A: 200}
	c.FillRect(zenui.Rect{W: 800, H: 600}, back)
	c.FillRect(zenui.Rect{X: 100, Y: 80, W: 600, H: 440}, zenui.Colour{R: 9, A: 255})
	if !c.caught || c.colour != back {
		t.Fatalf("caught %v colour %v, want the backdrop", c.caught, c.colour)
	}
	if len(rec.fills) != 1 || rec.fills[0].rect.W != 600 {
		t.Fatalf("drawn: %v, want the panel alone", rec.fills)
	}
	// only the first one: a second rectangle as large (a panel as big as the window) is drawn
	c.FillRect(zenui.Rect{W: 800, H: 600}, zenui.Colour{R: 5, A: 255})
	if len(rec.fills) != 2 {
		t.Fatalf("a second full-screen rectangle was held back too: %v", rec.fills)
	}
}

// The windows the bar can have open are found by modalDrawer in the order Draw has always used,
// and none is open when none is.
func TestModalDrawerIsNilWithNoWindowAndDrawsTheOneThatIsOpen(t *testing.T) {
	b, _ := newTestBar(t, nil, "")
	if b.modalDrawer(800, 600) != nil {
		t.Fatal("a window is drawn with none open")
	}
	b.activeModal = newHelpModal()
	draw := b.modalDrawer(800, 600)
	if draw == nil {
		t.Fatal("an open window has no drawing")
	}
	rec := &paintRenderer{}
	draw(rec)
	if len(rec.fills) == 0 || len(rec.texts) == 0 {
		t.Fatalf("the Help window drew %d fills and %d texts", len(rec.fills), len(rec.texts))
	}
}
