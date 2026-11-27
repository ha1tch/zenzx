//go:build !headless

package zenzx

import (
	"testing"

	"github.com/ha1tch/zenui"
)

// textRecorder is a renderer that remembers the text drawn and at what scale.
type textRecorder struct {
	testNoopRenderer
	drawn *[]textDraw
}
type textDraw struct {
	s     string
	scale int
}

func (r textRecorder) DrawText(s string, x, y, scale int, c zenui.Colour) {
	*r.drawn = append(*r.drawn, textDraw{s, scale})
}

func TestBarLabelsDrawAtTheBarScaleAndDropdownsDoNot(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()
	b.widget.SetScale(1) // the dropdowns at scale 1: the one a careless wrapper would also change

	openADropdown(t, b)
	label := b.widget.LabelFor(0)
	var drawn []textDraw
	r := &barScaleRenderer{Renderer: textRecorder{drawn: &drawn}, k: 3, inBar: true}
	b.widget.Draw(r, 800, 600, 0, 40, b.theme)

	var labels, items int
	for _, d := range drawn {
		isLabel := false
		for i := 0; b.widget.LabelFor(i) != ""; i++ {
			isLabel = isLabel || d.s == b.widget.LabelFor(i)
		}
		switch {
		case isLabel && d.s == label && d.scale == 3, isLabel && d.scale == 3:
			labels++
		case isLabel:
			t.Errorf("the label %q is drawn at scale %d, want the bar's, 3", d.s, d.scale)
		case d.scale == 1:
			items++
		default:
			t.Errorf("%q, in the dropdown, is drawn at scale %d, want the dropdown's, 1", d.s, d.scale)
		}
	}
	if labels < 8 {
		t.Errorf("%d labels were drawn at the bar's scale, want every one of the bar's", labels)
	}
	if items == 0 {
		t.Error("nothing of the open dropdown was drawn")
	}
}

func TestBarScaleFollowsTheWindowAndTheStripFollowsTheScale(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	if b.stripH() != menuStripHeight {
		t.Errorf("before the bar is fitted the strip is %d high, want %d", b.stripH(), menuStripHeight)
	}
	// The labels at each scale, by the bar's own count, give the widths the rule is made of.
	w := [4]int{}
	for k := 1; k <= 3; k++ {
		w[k] = b.barScaleWidth(k)
		if k > 1 && w[k] <= w[k-1] {
			t.Errorf("the labels take %d pixels at X%d and %d at X%d: not wider", w[k], k, w[k-1], k-1)
		}
	}
	for k := 1; k <= 3; k++ {
		screenW := (w[k]*barFitDenom + barFitNumer - 1) / barFitNumer // the narrowest window in which X_k fits
		const tall = 4000
		if got := b.barScale(screenW, tall); got != k {
			t.Errorf("a window %d wide (the labels are %d at X%d): scale %d, want %d", screenW, w[k], k, got, k)
		}
		if got := b.barScale(screenW-1, tall); k > 1 && got != k-1 {
			t.Errorf("a window %d wide: scale %d, want %d", screenW-1, got, k-1)
		}
		// The strip holds the text with room above and below it: the text is half of it or less,
		// and the room is never under barMinPad.
		b.barK = k
		line := b.renderer.LineHeight(k)
		if h := b.stripH(); h < menuStripHeight || 2*line > h || h < line+2*barMinPad {
			t.Errorf("at X%d the strip is %d high for text %d high: no room round it", k, h, line)
		}
		// And a window as wide as it needs to be, but too short to give the strip an eighth of
		// its height, does not take this scale.
		if k > 1 {
			if got, want := b.barScale(screenW, b.stripFor(k)*barHeightDenom-1), k-1; got != want {
				t.Errorf("X%d in a window %d high: scale %d, want %d", k, b.stripFor(k)*barHeightDenom-1, got, want)
			}
			if got := b.barScale(screenW, b.stripFor(k)*barHeightDenom); got != k {
				t.Errorf("X%d in a window %d high: scale %d, want %d", k, b.stripFor(k)*barHeightDenom, got, k)
			}
		}
	}
	// A window with no size at all, as the tests have before one opens: X1.
	if got := b.barScale(0, 0); got != 1 {
		t.Errorf("no window: scale %d, want 1", got)
	}
}

func TestWhatGoesWithTheStripFollowsItsHeight(t *testing.T) {
	for _, c := range []struct{ h, band, pad, bevel int }{
		{24, 16, 4, 1}, // the usual strip: what there always was
		{32, 21, 5, 1},
		{36, 24, 6, 2},
		{48, 32, 8, 2},
		{72, 48, 12, 3},
	} {
		if got := rainbowBandFor(c.h); got != c.band {
			t.Errorf("rainbowBandFor(%d) = %d, want %d", c.h, got, c.band)
		}
		if got := logoButtonPadFor(c.h); got != c.pad {
			t.Errorf("logoButtonPadFor(%d) = %d, want %d", c.h, got, c.pad)
		}
		if got := logoBevelThickness(c.h); got != c.bevel {
			t.Errorf("logoBevelThickness(%d) = %d, want %d", c.h, got, c.bevel)
		}
	}
}

func TestHelpButtonAndLogoGrowWithTheStrip(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	const screenW = 1000
	usual := b.stripH()
	left, right := barInsetsFor(usual)
	b.widget.SetInsets(left, right)
	b.widget.Draw(testNoopRenderer{}, screenW, 600, 0, usual, b.theme)
	logoX := b.widget.DecorationX()
	zone := b.helpHotZone(screenW)
	button := b.helpButton(screenW)
	if button.H != usual-2 || zone.H != usual {
		t.Fatalf("the usual strip, %d high: the button is %v and the hot zone %d high", usual, button, zone.H)
	}
	if logoX+0 >= button.X {
		t.Errorf("the logo starts at %d, not left of the Help button at %d", logoX, button.X)
	}

	b.barK = 3 // the largest text: the strip grows to hold it
	tall := b.stripH()
	if tall <= usual {
		t.Fatalf("the strip is %d high at X3 and %d at X1: it did not grow", tall, usual)
	}
	left, right = barInsetsFor(tall)
	b.widget.SetInsets(left, right)
	b.widget.Draw(testNoopRenderer{}, screenW, 600, 0, tall, b.theme)
	if got := b.widget.DecorationX(); got >= logoX {
		t.Errorf("the logo starts at %d in the tall strip and %d in the usual one: it did not grow", got, logoX)
	}
	zone = b.helpHotZone(screenW)
	button = b.helpButton(screenW)
	if button.H != tall-2 || zone.H != tall {
		t.Errorf("the tall strip, %d high: the button is %v and the hot zone %d high", tall, button, zone.H)
	}
	// The button is as wide as the strip is high, a pad of this strip from the window's edge.
	if want := screenW - logoButtonPadFor(tall) - helpWidthFor(tall); button.X != want {
		t.Errorf("the button starts at %d, want %d", button.X, want)
	}
	if d := b.widget.DecorationX(); d >= button.X {
		t.Errorf("the logo at %d runs into the button at %d", d, button.X)
	}
}
