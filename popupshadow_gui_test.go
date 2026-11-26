//go:build !headless

package zenzx

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
)

// shadowLog is a renderer that notes, in order, what the popups' shadow wrapper does.
type shadowLog struct {
	paintRenderer
	events []string
	clips  []zenui.Rect
}

func (s *shadowLog) FillRect(r zenui.Rect, c zenui.Colour) {
	s.events = append(s.events, "fill")
	s.paintRenderer.FillRect(r, c)
}
func (s *shadowLog) Clip(r zenui.Rect) {
	s.clips = append(s.clips, r)
	s.events = append(s.events, "clip")
}
func (s *shadowLog) ClipEnd() { s.events = append(s.events, "clipend") }

// The bar casts no shadow, the popups do: nothing of the strip is shadowed, and a panel is shadowed
// once, under it, inside a clip that keeps the shadow off the strip.
func TestPopupShadowIsForThePanelsAndNotTheBar(t *testing.T) {
	theme := zenui.LoadTheme(zenui.ThemeDark)
	inner := &shadowLog{}
	var boxes []rl.Rectangle
	r := &popupShadowRenderer{Renderer: inner, panel: theme.Panel, inBar: true,
		shadow: func(b rl.Rectangle) { boxes = append(boxes, b); inner.events = append(inner.events, "shadow") }}

	// The strip: its fill, even in the panel's colour, then the line that ends it.
	r.FillRect(zenui.Rect{W: 800, H: 24}, theme.Panel)
	r.StrokeRect(zenui.Rect{X: 0, Y: 23, W: 800, H: 1}, theme.Border, 1)
	if len(boxes) != 0 {
		t.Fatalf("the strip has a shadow: %v", boxes)
	}
	if r.top != 24 {
		t.Errorf("the shadow may begin at %d, want the row under the strip, 24", r.top)
	}
	// The panel, its selected row and its border.
	panel := zenui.Rect{X: 10, Y: 24, W: 200, H: 150}
	r.FillRect(panel, theme.Panel)
	r.FillRect(zenui.Rect{X: 10, Y: 30, W: 200, H: 16}, theme.SelFill)
	r.FillRect(zenui.Rect{X: 10, Y: 24, W: 200, H: 1}, theme.Border)
	if len(boxes) != 1 || boxes[0].X != 10 || boxes[0].Y != 24 || boxes[0].Width != 200 || boxes[0].Height != 150 {
		t.Fatalf("the shadows are %v, want one, of the panel", boxes)
	}
	// Under the panel, inside a clip that starts at the strip's bottom.
	want := []string{"fill", "clip", "shadow", "clipend", "fill", "fill", "fill"}
	if len(inner.events) != len(want) {
		t.Fatalf("events %v, want %v", inner.events, want)
	}
	for i := range want {
		if inner.events[i] != want[i] {
			t.Fatalf("events %v, want %v", inner.events, want)
		}
	}
	if len(inner.clips) != 1 || inner.clips[0].Y != 24 {
		t.Errorf("the clip is %v, want it to start at row 24", inner.clips)
	}
	// A submenu's panel, drawn after its parent's, is shadowed too.
	r.FillRect(zenui.Rect{X: 210, Y: 60, W: 120, H: 80}, theme.Panel)
	if len(boxes) != 2 {
		t.Errorf("a submenu has no shadow: %v", boxes)
	}
}

// A menu that is drawn without the strip (the logo menu) has its shadow from the first panel, and
// the real zenui menu draws one panel, first, and no other rectangle in the panel's colour: so
// that is the one the wrapper may take as a panel, in every theme.
func TestPopupShadowFindsTheRealMenusPanel(t *testing.T) {
	for _, name := range zenui.Themes {
		theme := zenui.LoadTheme(name)
		if theme.Panel == theme.SelFill || theme.Panel == theme.Border {
			t.Errorf("%s: the panel is the colour of the selected row or the border: its shadow could not be told", name)
		}
		menu := zenui.NewMenu(zenui.MenuConfig{
			Anchor: zenui.Rect{X: 40, Y: 0, W: 60, H: 24},
			Items: []zenui.Item{
				{Label: "One"}, {Label: "Two"}, {Separator: true}, {Label: "Three", SubItems: []zenui.Item{{Label: "Inner"}}},
			},
		})
		inner := &paintRenderer{}
		var boxes []rl.Rectangle
		r := &popupShadowRenderer{Renderer: inner, panel: theme.Panel, top: 24, shadow: func(b rl.Rectangle) { boxes = append(boxes, b) }}
		menu.Draw(r, 800, 600, theme)
		if len(boxes) != 1 {
			t.Fatalf("%s: %d shadows for one menu", name, len(boxes))
		}
		// the shadow is of the box that the panel is filled over
		var panel zenui.Rect
		for _, f := range inner.fills {
			if f.colour == theme.Panel {
				panel = f.rect
			}
		}
		if panel.W == 0 || float32(panel.X) != boxes[0].X || float32(panel.W) != boxes[0].Width || float32(panel.H) != boxes[0].Height {
			t.Errorf("%s: the shadow is of %v, the panel is %v", name, boxes[0], panel)
		}
	}
}

// With no shadow given, the bar draws as it always did.
func TestNoPopupShadowWithoutAShadow(t *testing.T) {
	b, _ := newTestBar(t, nil, "")
	var r zenui.Renderer = &paintRenderer{}
	if got := b.withPopupShadow(r, true, 0); got != r {
		t.Errorf("a bar with no shadow wraps its renderer: %T", got)
	}
	b.popupShadow = func(rl.Rectangle) {}
	if _, ok := b.withPopupShadow(r, true, 0).(*popupShadowRenderer); !ok {
		t.Error("a bar with a shadow does not wrap its renderer")
	}
}
