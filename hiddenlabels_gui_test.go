//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"slices"
	"testing"

	"github.com/ha1tch/zenui"
)

// labelSpy measures with the bar's own font, draws nothing, and notes the text it is asked to draw.
type labelSpy struct {
	testNoopRenderer
	b     *appMenuBar
	drawn *[]string
}

func (s labelSpy) TextWidth(t string, scale int) int { return s.b.renderer.TextWidth(t, scale) }
func (s labelSpy) LineHeight(scale int) int          { return s.b.renderer.LineHeight(scale) }
func (s labelSpy) DrawText(t string, _, _, _ int, _ zenui.Colour) {
	*s.drawn = append(*s.drawn, t)
}

func barLabels(b *appMenuBar) []string {
	var out []string
	for i := 0; b.widget.LabelFor(i) != ""; i++ {
		out = append(out, b.widget.LabelFor(i))
	}
	return out
}

// In a window too narrow for all the labels, those that would run under the Help button are not
// drawn, and the pointer over where they would be does nothing; in a wide one every label is
// drawn and the pointer is left as it is.
func TestLabelsUnderTheHelpButtonAreHidden(t *testing.T) {
	b, _ := testBar(t, 900)
	all := barLabels(b)
	if len(all) < 4 {
		t.Fatalf("only %d labels", len(all))
	}
	drawAt := func(screenW int) []string {
		var drawn []string
		left, right := barInsetsFor(b.stripH())
		b.widget.SetInsets(left, right)
		r := &barScaleRenderer{Renderer: labelSpy{b: b, drawn: &drawn}, k: 1, inBar: true, limit: b.labelsLimit(screenW)}
		b.widget.Draw(r, screenW, 600, 0, b.stripH(), b.theme)
		return drawn
	}

	if got := drawAt(900); !slices.Equal(got, all) {
		t.Errorf("a wide window drew %v, want every label %v", got, all)
	}
	if from := b.hiddenLabelsFrom(900); from != int(^uint(0)>>1) {
		t.Errorf("a wide window hides labels from x=%d", from)
	}
	wide := clickAt(600, 5)
	if got := b.maskHiddenLabels(wide, 900); got.MouseX != wide.MouseX || got.MouseY != wide.MouseY || !got.MousePressed {
		t.Errorf("the pointer was taken away in a wide window: %+v", got)
	}

	// Narrower and narrower: the labels drawn are always the first ones, in order, and each ends
	// before the Help button; the pointer is masked from where the first hidden one starts.
	hid := false
	for w := 900; w >= 200; w -= 25 {
		got := drawAt(w)
		if len(got) > len(all) || !slices.Equal(got, all[:len(got)]) {
			t.Fatalf("%d wide: drew %v, not the first of %v", w, got, all)
		}
		x := 12
		left, _ := barInsetsFor(b.stripH())
		x += left
		for _, l := range got {
			x += b.renderer.TextWidth(l, 1) + 16
		}
		if x > b.labelsLimit(w) {
			t.Errorf("%d wide: the labels end at %d, past the limit %d", w, x, b.labelsLimit(w))
		}
		if len(got) < len(all) {
			hid = true
			from := b.hiddenLabelsFrom(w)
			if from != x {
				t.Errorf("%d wide: hidden labels start at %d, want %d", w, from, x)
			}
			in := b.maskHiddenLabels(clickAt(from+3, 5), w)
			if in.MouseX >= 0 || in.MousePressed {
				t.Errorf("%d wide: the pointer over a hidden label is still there: %+v", w, in)
			}
			if keep := clickAt(from-3, 5); b.maskHiddenLabels(keep, w).MouseX != keep.MouseX {
				t.Errorf("%d wide: the pointer over a shown label was taken away", w)
			}
			below := zenui.Input{MouseX: from + 3, MouseY: b.stripH() + 10}
			if b.maskHiddenLabels(below, w).MouseX != below.MouseX {
				t.Errorf("%d wide: the pointer below the strip was taken away", w)
			}
		}
	}
	if !hid {
		t.Error("no window was narrow enough to hide a label")
	}
}
