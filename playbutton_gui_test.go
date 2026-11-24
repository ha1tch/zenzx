//go:build !headless

package zenzx

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The button is about a quarter of the picture's shorter side, in whole sprite pixels,
// between playMinScale and playMaxScale, and sits in the middle of its area.
func TestPlayButtonGeometry(t *testing.T) {
	for _, c := range []struct {
		w, h float32
		want int
	}{
		{320, 256, 4},    // a quarter of 256 is 64: 4 pixels to a sprite pixel
		{960, 768, 12},   // 768/4/16 is 12, over the cap
		{480, 384, 6},    // 384/4/16
		{100, 100, 4},    // too small for the least
		{4000, 3000, 10}, // and the most
		{1280, 100, 4},   // the shorter side rules
	} {
		got := playScale(c.w, c.h)
		want := c.want
		if want > playMaxScale {
			want = playMaxScale
		}
		if got != want {
			t.Errorf("playScale(%v,%v) = %d, want %d", c.w, c.h, got, want)
		}
		area := rl.Rectangle{X: 7, Y: 30, Width: c.w, Height: c.h}
		r := playButtonRect(area)
		if r.Width != r.Height || r.Width != float32(16*got) {
			t.Errorf("%vx%v: the face is %vx%v, want a %d square", c.w, c.h, r.Width, r.Height, 16*got)
		}
		cx, cy := r.X+r.Width/2, r.Y+r.Height/2
		if cx != area.X+area.Width/2 || cy != area.Y+area.Height/2 {
			t.Errorf("%vx%v: centred at (%v,%v), want the middle of its area", c.w, c.h, cx, cy)
		}
		if r.Width > area.Width || r.Height > area.Height {
			t.Errorf("%vx%v: the face %vx%v does not fit", c.w, c.h, r.Width, r.Height)
		}
	}
}

// In a tiled layout the button is in the front machine's own cell, whichever machine is in
// front and whichever layout: it fits the cell and is centred in it.
func TestPlayButtonIsInTheFrontMachinesCell(t *testing.T) {
	for _, layout := range []deckLayout{layoutColumns, layoutRows, layoutGrid, layoutMainRight, layoutMainLeft} {
		d := testDeck(t, 3)
		d.setLayout(layout)
		if !d.tiled() {
			t.Fatalf("layout %v did not tile three machines", layout)
		}
		for front := 0; front < 3; front++ {
			d.front = front
			cell := d.tileRects()[front]
			got := playButtonArea(d.machines[front].zx, d)
			if got != cell {
				t.Errorf("layout %v, machine %d in front: the area is %v, want its cell %v", layout, front, got, cell)
			}
			r := playButtonRect(got)
			if r.X < cell.X || r.Y < cell.Y || r.X+r.Width > cell.X+cell.Width || r.Y+r.Height > cell.Y+cell.Height {
				t.Errorf("layout %v, machine %d: the button %v does not fit its cell %v", layout, front, r, cell)
			}
			if cx, cy := r.X+r.Width/2, r.Y+r.Height/2; cx != cell.X+cell.Width/2 || cy != cell.Y+cell.Height/2 {
				t.Errorf("layout %v, machine %d: the button is centred at (%v,%v), not in the middle of its cell", layout, front, cx, cy)
			}
		}
	}
}
