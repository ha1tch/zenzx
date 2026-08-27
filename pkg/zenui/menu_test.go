package zenui

import "testing"

// With noopRenderer, dlgScale=2: TextWidth(s,2) = len(s)*16, LineHeight(2) = 16.
// padX = lh/2 = 8, padY = lh/4 = 4, itemH = lh+2*padY = 24.
// "Insert" -> TextWidth = 6*16 = 96, +2*padX(16) = 112, which exceeds
// minMenuW(80), so menuW = 112 for all three positioning cases below.

func testItems() []Item {
	return []Item{
		{Label: "Insert"},
		{Label: "Delete", Disabled: true},
		{Label: "Copy"},
	}
}

func TestMenuLayoutRoomToRight(t *testing.T) {
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 10, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 1280, 800, DefaultTheme())

	want := Rect{X: 10, Y: 74, W: 112, H: 24 * 3}
	if m.bounds != want {
		t.Fatalf("bounds = %+v, want %+v", m.bounds, want)
	}
}

func TestMenuLayoutRoomToLeftOnly(t *testing.T) {
	// screenW=1280; anchor near the right edge so the menu can't fit to the
	// right (1200+112=1312 > 1280) but fits aligned to the anchor's right
	// edge (1200+56-112=1144 >= 0).
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 1200, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 1280, 800, DefaultTheme())

	want := Rect{X: 1144, Y: 74, W: 112, H: 24 * 3}
	if m.bounds != want {
		t.Fatalf("bounds = %+v, want %+v", m.bounds, want)
	}
}

func TestMenuLayoutClampedWhenNeitherFits(t *testing.T) {
	// screenW=100 is narrower than menuW=112, so neither the right-aligned
	// nor the left-aligned placement fits; clamp to x=0.
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 0, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 100, 800, DefaultTheme())

	if m.bounds.X != 0 {
		t.Fatalf("bounds.X = %d, want 0 (clamped)", m.bounds.X)
	}
}

func TestMenuClickEnabledItemAccepts(t *testing.T) {
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 10, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 1280, 800, DefaultTheme())

	rec := m.itemRects[0] // "Insert"
	status := m.Update(Input{MouseX: rec.X + 4, MouseY: rec.Y + 4, MousePressed: true})

	if status != Accepted || m.Status() != Accepted {
		t.Fatalf("status = %v, want Accepted", status)
	}
	if m.Result() != 0 {
		t.Fatalf("Result() = %d, want 0", m.Result())
	}
}

func TestMenuClickDisabledItemIsNoop(t *testing.T) {
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 10, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 1280, 800, DefaultTheme())

	rec := m.itemRects[1] // "Delete", disabled
	status := m.Update(Input{MouseX: rec.X + 4, MouseY: rec.Y + 4, MousePressed: true})

	if status != Active {
		t.Fatalf("status = %v, want Active (click on disabled item is a no-op)", status)
	}
}

func TestMenuClickOutsideCancels(t *testing.T) {
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 10, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 1280, 800, DefaultTheme())

	status := m.Update(Input{MouseX: 900, MouseY: 700, MousePressed: true})

	if status != Cancelled {
		t.Fatalf("status = %v, want Cancelled", status)
	}
}

func TestMenuEscapeCancels(t *testing.T) {
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 10, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 1280, 800, DefaultTheme())

	status := m.Update(Input{Keys: []Key{KeyEscape}})

	if status != Cancelled {
		t.Fatalf("status = %v, want Cancelled", status)
	}
}

func TestMenuKeyboardNavSkipsDisabledThenAccepts(t *testing.T) {
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 10, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 1280, 800, DefaultTheme())

	m.Update(Input{Keys: []Key{KeyDown}}) // selected -> 0 ("Insert")
	if m.selected != 0 {
		t.Fatalf("after first Down, selected = %d, want 0", m.selected)
	}
	m.Update(Input{Keys: []Key{KeyDown}}) // selected -> 2 ("Copy"), skipping disabled 1
	if m.selected != 2 {
		t.Fatalf("after second Down, selected = %d, want 2 (skip disabled index 1)", m.selected)
	}

	status := m.Update(Input{Keys: []Key{KeyEnter}})
	if status != Accepted || m.Result() != 2 {
		t.Fatalf("status = %v, Result() = %d, want Accepted/2", status, m.Result())
	}
}

func TestMenuUpdateAfterConclusionIsIdempotent(t *testing.T) {
	m := NewMenu(MenuConfig{Items: testItems(), Anchor: Rect{X: 10, Y: 50, W: 56, H: 24}})
	m.Draw(noopRenderer{}, 1280, 800, DefaultTheme())

	m.Update(Input{Keys: []Key{KeyEscape}})
	if m.Status() != Cancelled {
		t.Fatalf("Status() = %v, want Cancelled", m.Status())
	}
	// A second Update after conclusion must not change anything further.
	status := m.Update(Input{MouseX: m.itemRects[0].X + 4, MouseY: m.itemRects[0].Y + 4, MousePressed: true})
	if status != Cancelled {
		t.Fatalf("status after post-conclusion Update = %v, want Cancelled (unchanged)", status)
	}
}

// drawRecorder logs every draw call it receives, for tests that need
// to assert on drawing order or exact colours used -- noopRenderer
// (dialog_test.go) intentionally discards everything, which can't answer
// "was the border drawn after the selection fill" or "what colour did
// the highlighted item's text actually use."
type drawCall struct {
	kind  string // "fill", "stroke", "text"
	rect  Rect
	label string
	col   Colour
}

type drawRecorder struct {
	calls *[]drawCall
}

func newDrawRecorder() drawRecorder {
	return drawRecorder{calls: &[]drawCall{}}
}

func (r drawRecorder) FillRect(rc Rect, c Colour) {
	*r.calls = append(*r.calls, drawCall{kind: "fill", rect: rc, col: c})
}
func (r drawRecorder) StrokeRect(rc Rect, c Colour, thickness int) {
	*r.calls = append(*r.calls, drawCall{kind: "stroke", rect: rc, col: c})
}
func (r drawRecorder) FillRectGradientV(rc Rect, top, bottom Colour) {
	*r.calls = append(*r.calls, drawCall{kind: "gradient", rect: rc, col: top})
}
func (r drawRecorder) FillRectGradientHMultiply(rc Rect, left, right Colour) {
	*r.calls = append(*r.calls, drawCall{kind: "gradientHMultiply", rect: rc, col: left})
}
func (r drawRecorder) DrawLine(x1, y1, x2, y2, thickness int, c Colour) {
	*r.calls = append(*r.calls, drawCall{kind: "line", rect: Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}, col: c})
}
func (r drawRecorder) DrawText(s string, x, y, scale int, c Colour) {
	*r.calls = append(*r.calls, drawCall{kind: "text", label: s, col: c})
}
func (r drawRecorder) TextWidth(s string, scale int) int { return len(s) * 8 * scale }
func (r drawRecorder) LineHeight(scale int) int          { return 8 * scale }
func (r drawRecorder) Clip(rc Rect) {
	*r.calls = append(*r.calls, drawCall{kind: "clip", rect: rc})
}
func (r drawRecorder) ClipEnd() {
	*r.calls = append(*r.calls, drawCall{kind: "clipend"})
}

func TestMenuBorderDrawnAfterSelectionFill(t *testing.T) {
	r := newDrawRecorder()
	theme := DefaultTheme()
	m := NewMenu(MenuConfig{Items: []Item{{Label: "One"}, {Label: "Two"}}})
	m.hover = 0 // "One" is highlighted

	m.Draw(r, 800, 600, theme)

	// The border is drawn as thin edge strips (drawBorder), not a
	// single StrokeRect call -- identify them by colour (theme.Border)
	// rather than by a "stroke" kind that no longer exists in the
	// recorded calls at all.
	selFillIdx, lastBorderIdx := -1, -1
	for i, c := range *r.calls {
		if c.kind != "fill" {
			continue
		}
		if c.col == theme.SelFill && selFillIdx == -1 {
			selFillIdx = i
		}
		if c.col == theme.Border {
			lastBorderIdx = i
		}
	}
	if selFillIdx == -1 || lastBorderIdx == -1 {
		t.Fatalf("expected both a selection fill and border edge fills, got %+v", *r.calls)
	}
	if lastBorderIdx < selFillIdx {
		t.Errorf("border (last edge at call %d) drawn before the selection fill (call %d) -- should be drawn after, so it isn't overlapped at the edges", lastBorderIdx, selFillIdx)
	}
}

func TestMenuHighlightedItemUsesSelText(t *testing.T) {
	r := newDrawRecorder()
	theme := DefaultTheme()
	m := NewMenu(MenuConfig{Items: []Item{{Label: "Selected"}, {Label: "Normal"}}})
	m.hover = 0

	m.Draw(r, 800, 600, theme)

	var gotSelected, gotNormal Colour
	found := 0
	for _, c := range *r.calls {
		if c.kind != "text" {
			continue
		}
		if c.label == "Selected" {
			gotSelected = c.col
			found++
		}
		if c.label == "Normal" {
			gotNormal = c.col
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected to find text draws for both items, found %d", found)
	}
	if gotSelected != theme.SelText {
		t.Errorf("highlighted item's text colour = %+v, want theme.SelText %+v", gotSelected, theme.SelText)
	}
	if gotNormal != theme.Text {
		t.Errorf("non-highlighted item's text colour = %+v, want theme.Text %+v", gotNormal, theme.Text)
	}
}

func TestMenuScaleFromConfig(t *testing.T) {
	r := newDrawRecorder()
	m := NewMenu(MenuConfig{Items: []Item{{Label: "X"}}, Scale: 3})
	m.Draw(r, 800, 600, DefaultTheme())

	// noopRenderer/drawRecorder: LineHeight(scale) = 8*scale. At
	// scale 3 the item row height should reflect that, not dlgScale's
	// default of 2.
	wantItemH := (8 * 3) + 2*((8*3)/4) // lh + 2*padY, matching layout()'s own formula
	if got := m.itemRects[0].H; got != wantItemH {
		t.Errorf("itemRects[0].H = %d, want %d (Scale:3 should flow through layout, not the package default)", got, wantItemH)
	}
}

func TestMenuScaleDefaultsWhenUnset(t *testing.T) {
	m := NewMenu(MenuConfig{Items: []Item{{Label: "X"}}})
	if m.scale != dlgScale {
		t.Errorf("scale = %d with Scale unset, want the package default %d", m.scale, dlgScale)
	}
}

func TestDrawBorderFourEdgesAtThickness(t *testing.T) {
	r := newDrawRecorder()
	bounds := Rect{X: 10, Y: 20, W: 100, H: 50}
	col := Colour{1, 2, 3, 255}

	drawBorder(r, bounds, col, 2, false)

	if len(*r.calls) != 4 {
		t.Fatalf("drawBorder with skipTop=false made %d fill calls, want 4", len(*r.calls))
	}
	want := []Rect{
		{X: 10, Y: 20, W: 100, H: 2}, // top
		{X: 10, Y: 68, W: 100, H: 2}, // bottom (Y+H-thickness = 20+50-2)
		{X: 10, Y: 20, W: 2, H: 50},  // left
		{X: 108, Y: 20, W: 2, H: 50}, // right (X+W-thickness = 10+100-2)
	}
	for i, w := range want {
		if (*r.calls)[i].rect != w {
			t.Errorf("edge %d rect = %+v, want %+v", i, (*r.calls)[i].rect, w)
		}
		if (*r.calls)[i].col != col {
			t.Errorf("edge %d colour = %+v, want %+v", i, (*r.calls)[i].col, col)
		}
	}
}

func TestDrawBorderSkipsTopWhenRequested(t *testing.T) {
	r := newDrawRecorder()
	bounds := Rect{X: 0, Y: 0, W: 100, H: 50}

	drawBorder(r, bounds, Colour{1, 2, 3, 255}, 2, true)

	if len(*r.calls) != 3 {
		t.Fatalf("drawBorder with skipTop=true made %d fill calls, want 3 (bottom, left, right)", len(*r.calls))
	}
	for _, c := range *r.calls {
		if c.rect.Y == 0 && c.rect.H == 2 && c.rect.W == 100 {
			t.Errorf("skipTop=true still drew a top-edge-shaped strip: %+v", c.rect)
		}
	}
}

func TestDrawBorderZeroThicknessDrawsNothing(t *testing.T) {
	r := newDrawRecorder()
	drawBorder(r, Rect{X: 0, Y: 0, W: 100, H: 50}, Colour{1, 2, 3, 255}, 0, false)
	if len(*r.calls) != 0 {
		t.Errorf("drawBorder with thickness=0 made %d calls, want 0", len(*r.calls))
	}
}

func TestMenuUsesThemeBorderThicknessAndSkipTop(t *testing.T) {
	r := newDrawRecorder()
	theme := DefaultTheme()
	theme.BorderThickness = 4
	theme.DropdownBorderSkipTop = true

	m := NewMenu(MenuConfig{Items: []Item{{Label: "One"}}})
	m.Draw(r, 800, 600, theme)

	borderCalls := 0
	for _, c := range *r.calls {
		if c.kind == "fill" && c.col == theme.Border {
			borderCalls++
			if c.rect.Y == m.bounds.Y && c.rect.H == 4 && c.rect.W == m.bounds.W {
				t.Error("DropdownBorderSkipTop=true but a top-edge strip was still drawn")
			}
		}
	}
	if borderCalls != 3 {
		t.Errorf("got %d border-coloured fill calls, want 3 (skipTop leaves bottom/left/right)", borderCalls)
	}
}
