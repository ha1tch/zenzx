//go:build !headless

package zenzx

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

func TestDockFloatsAboveTheBottomEdge(t *testing.T) {
	d := testDeck(t, 1)
	d.progress = 1
	for _, size := range [][2]int{{640, 480}, {960, 768}, {352, 296}} {
		l := d.layout(size[0], size[1], 8)
		if gap := float32(size[1]) - (l.panel.Y + l.panel.Height); gap != dockFloat {
			t.Errorf("in %v the dock stands %v px above the bottom edge, want %d", size, gap, dockFloat)
		}
		for i, r := range l.tiles {
			if r.Y+r.Height > l.panel.Y+l.panel.Height-dockRadius/2 {
				t.Errorf("in %v tile %d reaches into the panel's bottom corners", size, i)
			}
		}
	}
}

func TestFadedScalesOnlyTheAlpha(t *testing.T) {
	col := zenui.Colour{R: 10, G: 20, B: 30, A: 200}
	if got := faded(col, dockTopAlpha); got.R != 10 || got.G != 20 || got.B != 30 || got.A != 100 {
		t.Errorf("faded to a half = %v, want the same colour at alpha 100", got)
	}
	if got := faded(zenui.Colour{A: 255}, dockEdgeTopAlpha); got.A != 179 {
		t.Errorf("the line's alpha at the top = %d, want 179 (seven tenths of 255)", got.A)
	}
	if got := faded(col, 1); got.A != 200 {
		t.Errorf("faded by nothing changed the alpha to %d", got.A)
	}
}

func TestPictureRadiusKeepsWithinBounds(t *testing.T) {
	for _, c := range []struct{ h, want float32 }{{24, 4}, {84, 7}, {120, 10}, {160, 13.333333}, {480, 14}} {
		got := pictureRadius(rl.Rectangle{Width: c.h * 4 / 3, Height: c.h})
		if d := got - c.want; d > 0.001 || d < -0.001 {
			t.Errorf("a picture %v px high has corners of %v, want %v", c.h, got, c.want)
		}
	}
}
