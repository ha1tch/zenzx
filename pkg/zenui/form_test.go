package zenui

import "testing"

func TestNewFormNeverNilAndFocusesFirstFocusable(t *testing.T) {
	name := ""
	f := NewForm(FormConfig{Fields: []FormField{
		{Label: "Heading", Type: FieldLabel},
		{Label: "Name", Type: FieldText, Text: &name},
	}})
	if f == nil {
		t.Fatal("NewForm returned nil")
	}
	if f.focus != 1 {
		t.Errorf("focus = %d, want 1 (the first focusable field, skipping the label)", f.focus)
	}
}

func TestNewFormNoFocusableFieldsLeavesFocusUnset(t *testing.T) {
	f := NewForm(FormConfig{Fields: []FormField{{Label: "Just a label", Type: FieldLabel}}})
	if f.focus != -1 {
		t.Errorf("focus = %d, want -1 (no focusable fields)", f.focus)
	}
}

func TestFormGridSizeDefaultsToEight(t *testing.T) {
	f := NewForm(FormConfig{Fields: []FormField{{Label: "X", Type: FieldLabel}}})
	if f.gridSize != 8 {
		t.Errorf("gridSize = %d, want 8 (the package default)", f.gridSize)
	}
}

func TestFormFieldRectsSnapToGridSize(t *testing.T) {
	name := ""
	f := NewForm(FormConfig{
		GridSize: 8,
		Fields: []FormField{
			{Label: "Name", Type: FieldText, Text: &name, Row: 0, Col: 0},
		},
	})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	rec := f.FieldRect(0)
	if rec.W%8 != 0 {
		t.Errorf("field width = %d, want a multiple of 8 (GridSize)", rec.W)
	}
	if rec.H%8 != 0 {
		t.Errorf("field height = %d, want a multiple of 8 (GridSize)", rec.H)
	}
}

func TestFormTwoColumnsLayoutSideBySide(t *testing.T) {
	name, email := "", ""
	f := NewForm(FormConfig{Fields: []FormField{
		{Label: "Name", Type: FieldText, Text: &name, Row: 0, Col: 0},
		{Label: "Email", Type: FieldText, Text: &email, Row: 0, Col: 1},
	}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	r0, r1 := f.FieldRect(0), f.FieldRect(1)
	if r1.X < r0.X+r0.W {
		t.Errorf("second column (X=%d) overlaps or precedes the first column's right edge (X+W=%d)", r1.X, r0.X+r0.W)
	}
	if r0.Y != r1.Y {
		t.Errorf("fields in the same row have different Y: %d vs %d", r0.Y, r1.Y)
	}
}

func TestFormTwoRowsStackVertically(t *testing.T) {
	name, email := "", ""
	f := NewForm(FormConfig{Fields: []FormField{
		{Label: "Name", Type: FieldText, Text: &name, Row: 0, Col: 0},
		{Label: "Email", Type: FieldText, Text: &email, Row: 1, Col: 0},
	}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	r0, r1 := f.FieldRect(0), f.FieldRect(1)
	if r1.Y < r0.Y+r0.H {
		t.Errorf("second row (Y=%d) overlaps or precedes the first row's bottom edge (Y+H=%d)", r1.Y, r0.Y+r0.H)
	}
	if r0.X != r1.X {
		t.Errorf("fields in the same column have different X: %d vs %d", r0.X, r1.X)
	}
}

func TestFormMinWidthCellsWidensField(t *testing.T) {
	short, wide := "", ""
	f := NewForm(FormConfig{
		GridSize: 8,
		Fields: []FormField{
			{Label: "A", Type: FieldText, Text: &short, Row: 0, Col: 0},
			{Label: "B", Type: FieldText, Text: &wide, Row: 1, Col: 0, MinWidthCells: 20},
		},
	})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	// Both are in column 0, so the column (and therefore both fields'
	// width, single-column) takes the wider field's requirement.
	r0, r1 := f.FieldRect(0), f.FieldRect(1)
	if r0.W != r1.W {
		t.Errorf("same-column fields have different widths: %d vs %d (column width should be shared)", r0.W, r1.W)
	}
	if r0.W < 20*8 {
		t.Errorf("column width = %d, want at least MinWidthCells*GridSize = %d", r0.W, 20*8)
	}
}

func TestFormColSpanCoversMultipleColumns(t *testing.T) {
	f := NewForm(FormConfig{Fields: []FormField{
		{Label: "Heading spanning both columns", Type: FieldLabel, Row: 0, Col: 0, ColSpan: 2},
		{Label: "A", Type: FieldLabel, Row: 1, Col: 0},
		{Label: "B", Type: FieldLabel, Row: 1, Col: 1},
	}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	spanning := f.FieldRect(0)
	a, b := f.FieldRect(1), f.FieldRect(2)
	wantW := (b.X + b.W) - a.X
	if spanning.W != wantW {
		t.Errorf("spanning field width = %d, want %d (sum of both columns it spans)", spanning.W, wantW)
	}
}

func TestFormTextFieldTypingAppends(t *testing.T) {
	name := ""
	f := NewForm(FormConfig{Fields: []FormField{{Label: "Name", Type: FieldText, Text: &name}}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	f.Update(Input{Chars: []rune{'H', 'i'}})
	if name != "Hi" {
		t.Errorf("name = %q, want %q", name, "Hi")
	}
}

func TestFormTextFieldBackspaceRemovesLastChar(t *testing.T) {
	name := "Hi"
	f := NewForm(FormConfig{Fields: []FormField{{Label: "Name", Type: FieldText, Text: &name}}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	f.Update(Input{Keys: []Key{KeyBackspace}})
	if name != "H" {
		t.Errorf("name = %q, want %q", name, "H")
	}
}

func TestFormTextFieldBackspaceOnEmptyIsNoop(t *testing.T) {
	name := ""
	f := NewForm(FormConfig{Fields: []FormField{{Label: "Name", Type: FieldText, Text: &name}}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	f.Update(Input{Keys: []Key{KeyBackspace}})
	if name != "" {
		t.Errorf("name = %q, want empty (backspace on empty should be a no-op)", name)
	}
}

func TestFormTextFieldFiltersControlCharacters(t *testing.T) {
	name := ""
	f := NewForm(FormConfig{Fields: []FormField{{Label: "Name", Type: FieldText, Text: &name}}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	f.Update(Input{Chars: []rune{'A', 0x7f, 0x1b, 'B'}}) // A, DEL, ESC, B
	if name != "AB" {
		t.Errorf("name = %q, want %q (control characters filtered out)", name, "AB")
	}
}

func TestFormTabMovesFocusForwardSkippingLabelsAndDisabled(t *testing.T) {
	a, b := "", ""
	f := NewForm(FormConfig{Fields: []FormField{
		{Label: "Heading", Type: FieldLabel},
		{Label: "A", Type: FieldText, Text: &a},
		{Label: "Disabled", Type: FieldText, Text: &b, Disabled: true},
		{Label: "C", Type: FieldCheckbox, Checked: new(bool)},
	}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	if f.focus != 1 {
		t.Fatalf("setup: focus = %d, want 1", f.focus)
	}
	f.Update(Input{Keys: []Key{KeyTab}})
	if f.focus != 3 {
		t.Errorf("focus after Tab = %d, want 3 (skipping the label and the disabled field)", f.focus)
	}
	f.Update(Input{Keys: []Key{KeyTab}})
	if f.focus != 1 {
		t.Errorf("focus after a second Tab = %d, want 1 (wrapped back around)", f.focus)
	}
}

func TestFormClickingCheckboxTogglesAndFocuses(t *testing.T) {
	checked := false
	a := ""
	f := NewForm(FormConfig{Fields: []FormField{
		{Label: "A", Type: FieldText, Text: &a, Row: 0},
		{Label: "Enable", Type: FieldCheckbox, Checked: &checked, Row: 1},
	}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	rec := f.FieldRect(1)
	f.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1, MousePressed: true})

	if !checked {
		t.Error("clicking the checkbox did not toggle it")
	}
	if f.focus != 1 {
		t.Errorf("focus after clicking the checkbox = %d, want 1", f.focus)
	}
}

func TestFormClickingTextFieldFocusesWithoutEditing(t *testing.T) {
	a, b := "unchanged-a", "unchanged-b"
	f := NewForm(FormConfig{Fields: []FormField{
		{Label: "A", Type: FieldText, Text: &a, Row: 0},
		{Label: "B", Type: FieldText, Text: &b, Row: 1},
	}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	rec := f.FieldRect(1)
	f.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1, MousePressed: true})

	if f.focus != 1 {
		t.Errorf("focus after clicking field B = %d, want 1", f.focus)
	}
	if a != "unchanged-a" || b != "unchanged-b" {
		t.Error("clicking a text field should not itself edit any field's text")
	}
}

func TestFormAnchorOverridesAutoCentring(t *testing.T) {
	f := NewForm(FormConfig{
		Anchor: Rect{X: 50, Y: 60, W: 0, H: 0},
		Fields: []FormField{{Label: "X", Type: FieldLabel}},
	})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	if f.Bounds().X != 50 || f.Bounds().Y != 60 {
		t.Errorf("Bounds() = %+v, want origin at the given Anchor (50,60)", f.Bounds())
	}
}

func TestFormWithoutAnchorCentresOnScreen(t *testing.T) {
	f := NewForm(FormConfig{Fields: []FormField{{Label: "X", Type: FieldLabel}}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	b := f.Bounds()
	centreX, centreY := b.X+b.W/2, b.Y+b.H/2
	if centreX < 350 || centreX > 450 {
		t.Errorf("form's horizontal centre = %d, want roughly 400 (screen centre)", centreX)
	}
	if centreY < 250 || centreY > 350 {
		t.Errorf("form's vertical centre = %d, want roughly 300 (screen centre)", centreY)
	}
}

func TestFormStatusAlwaysActive(t *testing.T) {
	f := NewForm(FormConfig{Fields: []FormField{{Label: "X", Type: FieldLabel}}})
	f.Draw(noopRenderer{}, 800, 600, DefaultTheme())
	got := f.Update(Input{})
	if got != Active {
		t.Errorf("Update() = %v, want Active (Form never Accepts/Cancels itself)", got)
	}
	if f.Status() != Active {
		t.Errorf("Status() = %v, want Active", f.Status())
	}
}
