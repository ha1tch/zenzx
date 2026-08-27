package zenui

import "testing"

func TestNewModalNeverNil(t *testing.T) {
	m := NewModal(ModalConfig{
		Form:    FormConfig{Fields: []FormField{{Label: "Are you sure?", Type: FieldLabel}}},
		Buttons: []string{"OK"},
	})
	if m == nil {
		t.Fatal("NewModal returned nil")
	}
	if m.Status() != Active {
		t.Errorf("Status() = %v, want Active for a freshly constructed modal", m.Status())
	}
}

func TestModalButtonsPositionedLeftToRight(t *testing.T) {
	m := NewModal(ModalConfig{
		Form:    FormConfig{Fields: []FormField{{Label: "Msg", Type: FieldLabel}}},
		Buttons: []string{"Cancel", "OK"},
	})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	if len(m.buttonRects) != 2 {
		t.Fatalf("len(buttonRects) = %d, want 2", len(m.buttonRects))
	}
	cancel, ok := m.buttonRects[0], m.buttonRects[1]
	if ok.X < cancel.X+cancel.W {
		t.Errorf("\"OK\" (X=%d) should be to the right of \"Cancel\" (X+W=%d) -- Buttons order is left to right", ok.X, cancel.X+cancel.W)
	}
}

func TestModalButtonRowBelowForm(t *testing.T) {
	m := NewModal(ModalConfig{
		Form:    FormConfig{Fields: []FormField{{Label: "Msg", Type: FieldLabel}}},
		Buttons: []string{"OK"},
	})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	formBounds := m.form.Bounds()
	btn := m.buttonRects[0]
	if btn.Y < formBounds.Y+formBounds.H {
		t.Errorf("button row (Y=%d) should be below the form (Y+H=%d)", btn.Y, formBounds.Y+formBounds.H)
	}
}

func TestModalClickingButtonAccepts(t *testing.T) {
	m := NewModal(ModalConfig{
		Form:              FormConfig{Fields: []FormField{{Label: "Msg", Type: FieldLabel}}},
		Buttons:           []string{"Cancel", "OK"},
		CancelButtonIndex: 0,
	})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	okRect := m.buttonRects[1]
	status := m.Update(Input{MouseX: okRect.X + 1, MouseY: okRect.Y + 1, MousePressed: true})

	if status != Accepted {
		t.Errorf("Update() = %v, want Accepted", status)
	}
	if m.Result() != 1 {
		t.Errorf("Result() = %d, want 1 (\"OK\", the second button)", m.Result())
	}
}

func TestModalClickingCancelButtonCancels(t *testing.T) {
	m := NewModal(ModalConfig{
		Form:              FormConfig{Fields: []FormField{{Label: "Msg", Type: FieldLabel}}},
		Buttons:           []string{"Cancel", "OK"},
		CancelButtonIndex: 0,
	})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	cancelRect := m.buttonRects[0]
	status := m.Update(Input{MouseX: cancelRect.X + 1, MouseY: cancelRect.Y + 1, MousePressed: true})

	if status != Cancelled {
		t.Errorf("Update() = %v, want Cancelled", status)
	}
	if m.Result() != 0 {
		t.Errorf("Result() = %d, want 0 (\"Cancel\", the CancelButtonIndex)", m.Result())
	}
}

func TestModalEscapeTriggersCancelButtonIndex(t *testing.T) {
	m := NewModal(ModalConfig{
		Form:              FormConfig{Fields: []FormField{{Label: "Msg", Type: FieldLabel}}},
		Buttons:           []string{"No", "Yes"},
		CancelButtonIndex: 0,
	})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	status := m.Update(Input{Keys: []Key{KeyEscape}})
	if status != Cancelled {
		t.Errorf("Update() after Escape = %v, want Cancelled", status)
	}
	if m.Result() != 0 {
		t.Errorf("Result() = %d, want 0 (CancelButtonIndex)", m.Result())
	}
}

func TestModalEscapeDoesNothingWithoutCancelButtonIndex(t *testing.T) {
	m := NewModal(ModalConfig{
		Form:              FormConfig{Fields: []FormField{{Label: "Msg", Type: FieldLabel}}},
		Buttons:           []string{"OK"},
		CancelButtonIndex: -1, // explicit: no Escape handling
	})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	status := m.Update(Input{Keys: []Key{KeyEscape}})
	if status != Active {
		t.Errorf("Update() after Escape with CancelButtonIndex=-1 = %v, want Active (unaffected)", status)
	}
}

func TestModalFormFieldsWorkThroughModal(t *testing.T) {
	name := ""
	m := NewModal(ModalConfig{
		Form:    FormConfig{Fields: []FormField{{Label: "Name", Type: FieldText, Text: &name}}},
		Buttons: []string{"OK"},
	})
	m.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	m.Update(Input{Chars: []rune{'H', 'i'}})
	if name != "Hi" {
		t.Errorf("name = %q, want %q (Modal.Update should delegate typing to the wrapped Form)", name, "Hi")
	}
}

func TestModalDrawNoopsOnceClosed(t *testing.T) {
	r := newDrawRecorder()
	m := NewModal(ModalConfig{
		Form:    FormConfig{Fields: []FormField{{Label: "Msg", Type: FieldLabel}}},
		Buttons: []string{"OK"},
	})
	m.Draw(r, 800, 600, DefaultTheme())
	okRect := m.buttonRects[0]
	m.Update(Input{MouseX: okRect.X + 1, MouseY: okRect.Y + 1, MousePressed: true})

	r2 := newDrawRecorder()
	m.Draw(r2, 800, 600, DefaultTheme())
	if len(*r2.calls) != 0 {
		t.Errorf("Draw after Accepted made %d calls, want 0 (a closed modal should draw nothing)", len(*r2.calls))
	}
}
