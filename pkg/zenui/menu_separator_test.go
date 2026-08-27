package zenui

import "testing"

func TestSeparatorHasShorterRowThanNormalItem(t *testing.T) {
	m := NewMenu(MenuConfig{Items: []Item{
		{Label: "One"},
		{Separator: true},
		{Label: "Two"},
	}})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	normal := m.ItemRect(0)
	sep := m.ItemRect(1)
	if sep.H >= normal.H {
		t.Errorf("separator row height = %d, want less than a normal item's %d", sep.H, normal.H)
	}
}

func TestSeparatorNeverRendersAsHighlighted(t *testing.T) {
	// m.hover tracking a separator's index is harmless on its own --
	// the same is already true for Disabled items -- what matters is
	// that it's never drawn as the selected/highlighted row and can
	// never be clicked, both gated through itemEnabled at draw/accept
	// time, not at hover-detection time.
	r := newDrawRecorder()
	m := NewMenu(MenuConfig{Items: []Item{
		{Label: "One"},
		{Separator: true},
		{Label: "Two"},
	}})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())

	sepRect := m.ItemRect(1)
	m.Update(Input{MouseX: sepRect.X + 1, MouseY: sepRect.Y + 1})
	m.Draw(r, 800, 600, DefaultTheme())

	for _, c := range *r.calls {
		if c.kind == "fill" && c.rect == sepRect {
			t.Error("separator's rect should never be filled as a selection highlight")
		}
	}
}

func TestSeparatorNotClickable(t *testing.T) {
	m := NewMenu(MenuConfig{Items: []Item{
		{Label: "One"},
		{Separator: true},
		{Label: "Two"},
	}})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())

	sepRect := m.ItemRect(1)
	status := m.Update(Input{MouseX: sepRect.X + 1, MouseY: sepRect.Y + 1, MousePressed: true})

	if status == Accepted {
		t.Error("clicking a separator's rect should not Accept it")
	}
	if m.Status() != Active {
		t.Errorf("Status() = %v after clicking a separator, want Active (should stay open, not close on a click that hit nothing selectable)", m.Status())
	}
}

func TestSeparatorSkippedByKeyboardNavigation(t *testing.T) {
	m := NewMenu(MenuConfig{Items: []Item{
		{Label: "One"},
		{Separator: true},
		{Label: "Two"},
	}})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())

	m.Update(Input{Keys: []Key{KeyDown}}) // selects index 0
	if m.selected != 0 {
		t.Fatalf("setup: selected = %d, want 0", m.selected)
	}
	m.Update(Input{Keys: []Key{KeyDown}}) // should skip index 1 (separator), land on 2

	if m.selected != 2 {
		t.Errorf("selected = %d, want 2 (Down should skip the separator entirely)", m.selected)
	}
}

func TestSeparatorDrawsLineNotText(t *testing.T) {
	r := newDrawRecorder()
	m := NewMenu(MenuConfig{Items: []Item{
		{Label: "One"},
		{Separator: true},
		{Label: "Two"},
	}})
	m.Draw(r, 800, 600, DefaultTheme())

	sepRect := m.ItemRect(1)
	foundLine := false
	textCalls := 0
	for _, c := range *r.calls {
		if c.kind == "line" && c.rect.Y >= sepRect.Y && c.rect.Y < sepRect.Y+sepRect.H {
			foundLine = true
		}
		if c.kind == "text" {
			textCalls++
		}
	}
	if !foundLine {
		t.Error("separator should draw a horizontal line within its own row")
	}
	// Only "One" and "Two" should ever call DrawText -- the separator's
	// own row hits a `continue` before reaching that call at all.
	if textCalls != 2 {
		t.Errorf("DrawText called %d times, want 2 (one per non-separator item; the separator's row should never reach that call)", textCalls)
	}
}

func TestSeparatorReducesTotalMenuHeight(t *testing.T) {
	withSep := NewMenu(MenuConfig{Items: []Item{{Label: "One"}, {Separator: true}, {Label: "Two"}}})
	withSep.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	withItem := NewMenu(MenuConfig{Items: []Item{{Label: "One"}, {Label: "Middle"}, {Label: "Two"}}})
	withItem.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	if withSep.bounds.H >= withItem.bounds.H {
		t.Errorf("menu with a separator (H=%d) should be shorter than one with a full normal item in its place (H=%d)", withSep.bounds.H, withItem.bounds.H)
	}
}
