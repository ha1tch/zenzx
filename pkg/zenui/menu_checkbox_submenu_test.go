package zenui

import "testing"

func TestCheckIndicatorReservesGutterOnlyWhenUsed(t *testing.T) {
	r := newDrawRecorder()

	plain := NewMenu(MenuConfig{Items: []Item{{Label: "One"}}})
	plain.Draw(r, 800, 600, DefaultTheme())
	if plain.checkGutter != 0 {
		t.Errorf("checkGutter = %d for a menu with no Checked items, want 0", plain.checkGutter)
	}

	checked := true
	withCheck := NewMenu(MenuConfig{Items: []Item{{Label: "One", Checked: &checked}}})
	withCheck.Draw(r, 800, 600, DefaultTheme())
	if withCheck.checkGutter == 0 {
		t.Error("checkGutter = 0 for a menu with a Checked item, want non-zero")
	}
}

func TestCheckmarkDrawsTickForCheckedItem(t *testing.T) {
	// Toggle: false (the default) -- a checkmark, for mutually
	// exclusive options. Checked draws a tick (two short lines), not a
	// filled square.
	r := newDrawRecorder()
	theme := DefaultTheme()
	checked := true
	m := NewMenu(MenuConfig{Items: []Item{{Label: "On", Checked: &checked}}})
	m.Draw(r, 800, 600, theme)

	lineCount := 0
	for _, c := range *r.calls {
		if c.kind == "line" {
			lineCount++
		}
	}
	if lineCount != 2 {
		t.Errorf("line count = %d, want 2 (the tick mark's two segments)", lineCount)
	}
}

func TestCheckmarkDrawsNothingForUncheckedItem(t *testing.T) {
	// Toggle: false -- unlike a checkbox, an unchecked checkmark item
	// draws no indicator at all (no persistent "off" box).
	r := newDrawRecorder()
	theme := DefaultTheme()
	unchecked := false
	m := NewMenu(MenuConfig{Items: []Item{{Label: "Off", Checked: &unchecked}}})
	m.Draw(r, 800, 600, theme)

	for _, c := range *r.calls {
		if c.kind == "stroke" && c.rect.W > 0 && c.rect.W < 20 && c.rect.H == c.rect.W {
			t.Error("unchecked checkmark item should draw no box at all, found a small stroked square")
		}
		if c.kind == "line" {
			t.Error("unchecked checkmark item should draw no tick, found a line")
		}
	}
}

func TestCheckboxDrawsOutlinedBoxRegardlessOfState(t *testing.T) {
	// Toggle: true -- a checkbox, for non-mutually-exclusive options.
	// The box itself is always visible, checked or not.
	for _, checked := range []bool{true, false} {
		r := newDrawRecorder()
		theme := DefaultTheme()
		c := checked
		m := NewMenu(MenuConfig{Items: []Item{{Label: "X", Checked: &c, Toggle: true}}})
		m.Draw(r, 800, 600, theme)

		strokedSmall := false
		for _, call := range *r.calls {
			if call.kind == "stroke" && call.rect.W > 0 && call.rect.W < 20 && call.rect.H == call.rect.W {
				strokedSmall = true
			}
		}
		if !strokedSmall {
			t.Errorf("checked=%v: expected a small stroked square (the checkbox's own box), found none", checked)
		}
	}
}

func TestCheckboxDrawsCrossOnlyWhenChecked(t *testing.T) {
	checked := true
	r := newDrawRecorder()
	m := NewMenu(MenuConfig{Items: []Item{{Label: "X", Checked: &checked, Toggle: true}}})
	m.Draw(r, 800, 600, DefaultTheme())

	lineCount := 0
	for _, c := range *r.calls {
		if c.kind == "line" {
			lineCount++
		}
	}
	if lineCount != 2 {
		t.Errorf("checked checkbox: line count = %d, want 2 (the cross's two diagonals)", lineCount)
	}

	unchecked := false
	r2 := newDrawRecorder()
	m2 := NewMenu(MenuConfig{Items: []Item{{Label: "X", Checked: &unchecked, Toggle: true}}})
	m2.Draw(r2, 800, 600, DefaultTheme())

	for _, c := range *r2.calls {
		if c.kind == "line" {
			t.Error("unchecked checkbox should draw no cross, found a line")
		}
	}
}

func TestToggleItemStaysActiveAfterClick(t *testing.T) {
	r := newDrawRecorder()
	checked := false
	m := NewMenu(MenuConfig{Items: []Item{{Label: "Bold", Checked: &checked, Toggle: true}}})
	m.Draw(r, 800, 600, DefaultTheme())

	rec := m.ItemRect(0)
	status := m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1, MousePressed: true})

	if status != Toggled {
		t.Errorf("Update() = %v, want Toggled", status)
	}
	if m.Status() != Active {
		t.Errorf("Status() = %v after a toggle, want Active (a checkbox menu must stay open)", m.Status())
	}
	if !checked {
		t.Error("Checked value was not flipped by the toggle")
	}
	if m.Result() != 0 {
		t.Errorf("Result() = %d, want 0", m.Result())
	}
}

func TestToggleItemCanBeToggledMultipleTimes(t *testing.T) {
	checked := false
	m := NewMenu(MenuConfig{Items: []Item{{Label: "Bold", Checked: &checked, Toggle: true}}})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())
	rec := m.ItemRect(0)

	for i, want := range []bool{true, false, true} {
		status := m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1, MousePressed: true})
		if status != Toggled {
			t.Fatalf("click %d: Update() = %v, want Toggled", i, status)
		}
		if checked != want {
			t.Errorf("click %d: checked = %v, want %v", i, checked, want)
		}
		if m.Status() != Active {
			t.Fatalf("click %d: Status() = %v, want Active", i, m.Status())
		}
	}
}

func TestNonToggleItemStillClosesOnClick(t *testing.T) {
	// A plain item (Toggle: false, the default) must keep its existing
	// select-and-close behaviour unchanged, checkmark or not.
	checked := true
	m := NewMenu(MenuConfig{Items: []Item{{Label: "Info", Checked: &checked}}})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())
	rec := m.ItemRect(0)

	status := m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1, MousePressed: true})
	if status != Accepted {
		t.Errorf("Update() = %v, want Accepted (non-toggle item)", status)
	}
	if m.Status() != Accepted {
		t.Errorf("Status() = %v, want Accepted", m.Status())
	}
}

func TestSubmenuOpensOnHover(t *testing.T) {
	m := NewMenu(MenuConfig{
		Items: []Item{{Label: "Zoom", SubItems: []Item{{Label: "X1"}, {Label: "X2"}}}},
	})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())
	rec := m.ItemRect(0)

	m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1})

	if m.subOpen != 0 {
		t.Errorf("subOpen = %d, want 0 after hovering the submenu-having item", m.subOpen)
	}
	if m.subMenu == nil {
		t.Fatal("subMenu is nil after hovering a submenu-having item")
	}
}

func TestSubmenuParentNotDirectlySelectable(t *testing.T) {
	m := NewMenu(MenuConfig{
		Items: []Item{{Label: "Zoom", SubItems: []Item{{Label: "X1"}}}},
	})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())
	rec := m.ItemRect(0)

	// Hover opens the submenu; a click on the parent row itself (not
	// yet moved into the submenu) must not Accept the parent.
	m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1})
	status := m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1, MousePressed: true})

	if status == Accepted {
		t.Error("clicking a submenu-parent row directly should not Accept it")
	}
}

func TestSubmenuChoicePropagatesToParent(t *testing.T) {
	m := NewMenu(MenuConfig{
		Items: []Item{
			{Label: "Other"},
			{Label: "Zoom", SubItems: []Item{{Label: "X1"}, {Label: "X2"}}},
		},
	})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())
	parentRec := m.ItemRect(1)

	m.Update(Input{MouseX: parentRec.X + 1, MouseY: parentRec.Y + 1}) // open submenu
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())               // layout the submenu itself

	subRec := m.subMenu.ItemRect(1) // "X2"
	status := m.Update(Input{MouseX: subRec.X + 1, MouseY: subRec.Y + 1, MousePressed: true})

	if status != Accepted {
		t.Fatalf("Update() = %v, want Accepted", status)
	}
	if m.Result() != 1 {
		t.Errorf("Result() = %d, want 1 (the parent \"Zoom\" item's own index)", m.Result())
	}
	if m.SubResult() != 1 {
		t.Errorf("SubResult() = %d, want 1 (\"X2\", the second submenu item)", m.SubResult())
	}
}

func TestSubmenuCancelReturnsToParentWithoutClosingIt(t *testing.T) {
	m := NewMenu(MenuConfig{
		Items: []Item{{Label: "Zoom", SubItems: []Item{{Label: "X1"}}}},
	})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())
	rec := m.ItemRect(0)

	m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1}) // open submenu
	status := m.Update(Input{Keys: []Key{KeyEscape}})     // Escape cancels the submenu

	if status != Active {
		t.Errorf("Update() = %v after Escape inside a submenu, want Active (parent stays open)", status)
	}
	if m.subOpen != -1 {
		t.Errorf("subOpen = %d, want -1 (submenu closed)", m.subOpen)
	}
}

func TestHoveringDifferentSubmenuItemSwitches(t *testing.T) {
	m := NewMenu(MenuConfig{
		Items: []Item{
			{Label: "Zoom", SubItems: []Item{{Label: "X1"}}},
			{Label: "Font", SubItems: []Item{{Label: "Sinclair"}}},
		},
	})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())

	rec0 := m.ItemRect(0)
	m.Update(Input{MouseX: rec0.X + 1, MouseY: rec0.Y + 1})
	if m.subOpen != 0 {
		t.Fatalf("setup: subOpen = %d, want 0", m.subOpen)
	}
	firstSub := m.subMenu

	rec1 := m.ItemRect(1)
	m.Update(Input{MouseX: rec1.X + 1, MouseY: rec1.Y + 1})
	if m.subOpen != 1 {
		t.Errorf("subOpen = %d, want 1 after hovering a different submenu-having item", m.subOpen)
	}
	if m.subMenu == firstSub {
		t.Error("subMenu should be a new instance after switching to a different parent")
	}
}

func TestCheckIndicatorNotHardAgainstLeftBorder(t *testing.T) {
	// Regression guard: the indicator's X used to be computed from
	// rec.X directly, ignoring padX entirely, putting it right against
	// the panel's own left border with no breathing room.
	r := newDrawRecorder()
	checked := true
	theme := DefaultTheme()
	m := NewMenu(MenuConfig{Items: []Item{{Label: "X", Checked: &checked, Toggle: true}}})
	m.Draw(r, 800, 600, theme)

	rec := m.ItemRect(0)
	for _, c := range *r.calls {
		if c.kind == "stroke" && c.rect.W > 0 && c.rect.W < 20 && c.rect.H == c.rect.W {
			if c.rect.X < rec.X+m.padX {
				t.Errorf("checkbox box X=%d, want at least rec.X+padX=%d (should sit after the same left margin as item text)", c.rect.X, rec.X+m.padX)
			}
		}
	}
}

func TestSubmenuAlwaysDrawsFullBorderRegardlessOfSkipTop(t *testing.T) {
	theme := SpectrumTheme() // DropdownBorderSkipTop: true for this theme
	if !theme.DropdownBorderSkipTop {
		t.Fatal("setup: SpectrumTheme should have DropdownBorderSkipTop=true")
	}

	m := NewMenu(MenuConfig{
		Items: []Item{{Label: "Zoom", SubItems: []Item{{Label: "X1"}, {Label: "X2"}}}},
	})
	m.Draw(newDrawRecorder(), 800, 600, theme)
	rec := m.ItemRect(0)
	m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1}) // open submenu

	r := newDrawRecorder()
	m.Draw(r, 800, 600, theme)

	if !m.subMenu.isSubmenu {
		t.Fatal("subMenu.isSubmenu should be true")
	}

	// A full border means 4 stroke/fill edges were drawn for the
	// submenu's own bounds -- specifically, confirm at least one
	// stroke/fill call's Y matches the submenu's own top edge
	// (bounds.Y), which theme.DropdownBorderSkipTop would otherwise
	// omit entirely.
	found := false
	for _, c := range *r.calls {
		if (c.kind == "fill" || c.kind == "stroke") && c.rect.Y == m.subMenu.bounds.Y && c.rect.W == m.subMenu.bounds.W {
			found = true
		}
	}
	if !found {
		t.Error("submenu should draw its own top border edge even though the theme's DropdownBorderSkipTop is true")
	}
}

func TestSubmenuPositionedFourPixelsLeftOfParentEdge(t *testing.T) {
	m := NewMenu(MenuConfig{
		Items: []Item{{Label: "Zoom", SubItems: []Item{{Label: "X1"}}}},
	})
	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme())
	rec := m.ItemRect(0)
	m.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1}) // open submenu

	m.Draw(newDrawRecorder(), 800, 600, DefaultTheme()) // layout the submenu

	want := rec.X + rec.W - 4
	if m.subMenu.bounds.X != want {
		t.Errorf("submenu bounds.X = %d, want %d (parent row's right edge - 4px)", m.subMenu.bounds.X, want)
	}
}

func TestSpectrumCheckmarkUsesGreenNotComputedColour(t *testing.T) {
	theme := SpectrumTheme()
	if !theme.UseCheckmarkColour {
		t.Fatal("setup: SpectrumTheme should have UseCheckmarkColour=true")
	}
	wantGreen := Colour{0x00, 0xc8, 0x00, 0xff}
	if theme.CheckmarkColour != wantGreen {
		t.Fatalf("setup: CheckmarkColour = %+v, want %+v (standard, not bright, Spectrum green)", theme.CheckmarkColour, wantGreen)
	}

	r := newDrawRecorder()
	checked := true
	m := NewMenu(MenuConfig{Items: []Item{{Label: "Dark", Checked: &checked}}})
	m.Draw(r, 800, 600, theme)

	foundGreenLine := false
	foundBlackLine := false
	for _, c := range *r.calls {
		if c.kind != "line" {
			continue
		}
		if c.col == wantGreen {
			foundGreenLine = true
		}
		if c.col == theme.Text { // Spectrum's Text is black
			foundBlackLine = true
		}
	}
	if !foundGreenLine {
		t.Error("expected the checkmark's tick to be drawn in Spectrum's bright green")
	}
	if foundBlackLine {
		t.Error("checkmark tick should not use the theme's normal (black) text colour when UseCheckmarkColour is set")
	}
}

func TestOtherThemesCheckmarkUsesComputedColourNotOverride(t *testing.T) {
	for _, theme := range []Theme{DefaultTheme(), DarkTheme(), LightTheme()} {
		if theme.UseCheckmarkColour {
			t.Errorf("UseCheckmarkColour = true, want false for this theme")
		}
	}
}

func TestCheckboxEmptyUsesGreyAcrossAllThemes(t *testing.T) {
	wantGrey := Colour{0x80, 0x80, 0x80, 0xff}
	for name, theme := range map[string]Theme{
		"Default":  DefaultTheme(),
		"Dark":     DarkTheme(),
		"Light":    LightTheme(),
		"Spectrum": SpectrumTheme(),
	} {
		if theme.CheckboxEmptyColour != wantGrey {
			t.Errorf("%s: CheckboxEmptyColour = %+v, want %+v", name, theme.CheckboxEmptyColour, wantGrey)
		}
	}
}

func TestUncheckedCheckboxDrawsGreyBox(t *testing.T) {
	theme := DarkTheme()
	r := newDrawRecorder()
	unchecked := false
	m := NewMenu(MenuConfig{Items: []Item{{Label: "X", Checked: &unchecked, Toggle: true}}})
	m.Draw(r, 800, 600, theme)

	foundGreyStroke := false
	for _, c := range *r.calls {
		if c.kind == "stroke" && c.rect.W > 0 && c.rect.W < 20 && c.rect.H == c.rect.W {
			if c.col == theme.CheckboxEmptyColour {
				foundGreyStroke = true
			}
		}
	}
	if !foundGreyStroke {
		t.Error("unchecked checkbox's own box should be stroked in CheckboxEmptyColour (grey)")
	}
}

func TestCheckedCheckboxDoesNotUseGreyBox(t *testing.T) {
	theme := DarkTheme()
	r := newDrawRecorder()
	checked := true
	m := NewMenu(MenuConfig{Items: []Item{{Label: "X", Checked: &checked, Toggle: true}}})
	m.Draw(r, 800, 600, theme)

	for _, c := range *r.calls {
		if c.kind == "stroke" && c.rect.W > 0 && c.rect.W < 20 && c.rect.H == c.rect.W {
			if c.col == theme.CheckboxEmptyColour {
				t.Error("checked checkbox's box should not use CheckboxEmptyColour -- only the unchecked/empty state does")
			}
		}
	}
}

func TestSpectrumSeparatorUsesGreyNotBlackBorder(t *testing.T) {
	theme := SpectrumTheme()
	wantGrey := Colour{0x80, 0x80, 0x80, 0xff}
	if theme.SeparatorColour != wantGrey {
		t.Fatalf("setup: SeparatorColour = %+v, want %+v", theme.SeparatorColour, wantGrey)
	}
	if theme.SeparatorColour == theme.Border {
		t.Fatal("setup: SeparatorColour should differ from Border for this theme (black would be near-invisible on white)")
	}

	r := newDrawRecorder()
	m := NewMenu(MenuConfig{Items: []Item{{Label: "One"}, {Separator: true}, {Label: "Two"}}})
	m.Draw(r, 800, 600, theme)

	foundGreyLine := false
	for _, c := range *r.calls {
		if c.kind == "line" && c.col == wantGrey {
			foundGreyLine = true
		}
		if c.kind == "line" && c.col == theme.Border {
			t.Error("separator line should not use theme.Border for this theme")
		}
	}
	if !foundGreyLine {
		t.Error("expected the separator line to be drawn in SeparatorColour (grey)")
	}
}

func TestSpectrumCheckboxUsesBlueWhenChecked(t *testing.T) {
	theme := SpectrumTheme()
	if !theme.UseCheckboxColour {
		t.Fatal("setup: SpectrumTheme should have UseCheckboxColour=true")
	}
	wantBlue := Colour{0x00, 0x00, 0xc8, 0xff}
	if theme.CheckboxColour != wantBlue {
		t.Fatalf("setup: CheckboxColour = %+v, want %+v", theme.CheckboxColour, wantBlue)
	}

	r := newDrawRecorder()
	checked := true
	m := NewMenu(MenuConfig{Items: []Item{{Label: "X", Checked: &checked, Toggle: true}}})
	m.Draw(r, 800, 600, theme)

	foundBlueStroke, foundBlueLine := false, false
	for _, c := range *r.calls {
		if c.kind == "stroke" && c.col == wantBlue {
			foundBlueStroke = true
		}
		if c.kind == "line" && c.col == wantBlue {
			foundBlueLine = true
		}
		if c.kind == "line" && c.col == theme.CheckmarkColour {
			t.Error("checkbox cross should use CheckboxColour (blue), not CheckmarkColour (green) -- the two indicator kinds should differ")
		}
	}
	if !foundBlueStroke {
		t.Error("checked checkbox's box outline should use CheckboxColour (blue)")
	}
	if !foundBlueLine {
		t.Error("checked checkbox's cross should use CheckboxColour (blue)")
	}
}

func TestOtherThemesDoNotUseCheckboxColourOverride(t *testing.T) {
	for _, theme := range []Theme{DefaultTheme(), DarkTheme(), LightTheme()} {
		if theme.UseCheckboxColour {
			t.Error("UseCheckboxColour = true, want false for this theme")
		}
	}
}
