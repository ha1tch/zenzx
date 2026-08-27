package zenui

import "testing"

func TestTitleRowShorterThanNormalItem(t *testing.T) {
	m := NewMenu(MenuConfig{Items: []Item{
		{Title: true, Label: "Group"},
		{Label: "One"},
	}})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	title := m.ItemRect(0)
	normal := m.ItemRect(1)
	if title.H >= normal.H {
		t.Errorf("title row height = %d, want less than a normal item's %d", title.H, normal.H)
	}
}

func TestTitleNeverRendersAsHighlighted(t *testing.T) {
	m := NewMenu(MenuConfig{Items: []Item{
		{Title: true, Label: "Group"},
		{Label: "One"},
	}})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())

	titleRect := m.ItemRect(0)
	m.Update(Input{MouseX: titleRect.X + 1, MouseY: titleRect.Y + 1})

	r := newDrawRecorder()
	m.Draw(r, 800, 600, DefaultTheme())
	for _, c := range *r.calls {
		if c.kind == "fill" && c.rect == titleRect {
			t.Error("title's rect should never be filled as a selection highlight")
		}
	}
}

func TestTitleNotClickable(t *testing.T) {
	m := NewMenu(MenuConfig{Items: []Item{
		{Title: true, Label: "Group"},
		{Label: "One"},
	}})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())

	titleRect := m.ItemRect(0)
	status := m.Update(Input{MouseX: titleRect.X + 1, MouseY: titleRect.Y + 1, MousePressed: true})

	if status == Accepted {
		t.Error("clicking a title's rect should not Accept it")
	}
	if m.Status() != Active {
		t.Errorf("Status() = %v after clicking a title, want Active", m.Status())
	}
}

func TestTitleSkippedByKeyboardNavigation(t *testing.T) {
	m := NewMenu(MenuConfig{Items: []Item{
		{Label: "One"},
		{Title: true, Label: "Group"},
		{Label: "Two"},
	}})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())

	m.Update(Input{Keys: []Key{KeyDown}}) // selects index 0
	if m.selected != 0 {
		t.Fatalf("setup: selected = %d, want 0", m.selected)
	}
	m.Update(Input{Keys: []Key{KeyDown}}) // should skip index 1 (title), land on 2

	if m.selected != 2 {
		t.Errorf("selected = %d, want 2 (Down should skip the title entirely)", m.selected)
	}
}

func TestTitleDrawnInDimTextAtSmallerScale(t *testing.T) {
	theme := DefaultTheme()
	r := newDrawRecorder()
	m := NewMenu(MenuConfig{Scale: 2, Items: []Item{
		{Title: true, Label: "Group"},
		{Label: "One"},
	}})
	m.Draw(r, 800, 600, theme)

	found := false
	for _, c := range *r.calls {
		if c.kind == "text" && c.label == "Group" {
			found = true
			if c.col != theme.DimText {
				t.Errorf("title text colour = %+v, want theme.DimText (%+v)", c.col, theme.DimText)
			}
		}
	}
	if !found {
		t.Error("expected the title's own text to be drawn")
	}
	if got := m.titleScale(); got != 1 {
		t.Errorf("titleScale() = %d, want 1 (menu scale 2, one step smaller)", got)
	}
}

func TestTitleScaleFlooredAtOne(t *testing.T) {
	m := NewMenu(MenuConfig{Scale: 1, Items: []Item{{Title: true, Label: "Group"}}})
	if got := m.titleScale(); got != 1 {
		t.Errorf("titleScale() = %d, want 1 (floored, menu scale was already 1)", got)
	}
}

func TestTitleReducesTotalMenuHeightLessThanNormalItem(t *testing.T) {
	withTitle := NewMenu(MenuConfig{Items: []Item{{Title: true, Label: "Group"}, {Label: "One"}}})
	withTitle.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	withItem := NewMenu(MenuConfig{Items: []Item{{Label: "Group"}, {Label: "One"}}})
	withItem.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	if withTitle.bounds.H >= withItem.bounds.H {
		t.Errorf("menu with a title (H=%d) should be shorter than one with a full normal item in its place (H=%d)", withTitle.bounds.H, withItem.bounds.H)
	}
}
