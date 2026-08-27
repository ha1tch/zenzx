package zenui

import "testing"

// Compile-time check that MessageDialog really is MessageBox (a type
// alias, not a distinct type) -- if this ever stops compiling, the alias
// relationship documented on MessageDialog has been broken.
var _ *MessageDialog = (*MessageBox)(nil)

func TestNewMessageBoxNeverNil(t *testing.T) {
	mb := NewMessageBox(MessageBoxConfig{Title: "Title", Message: "Message", Buttons: []string{"OK"}})
	if mb == nil {
		t.Fatal("NewMessageBox returned nil")
	}
	if mb.Status() != Active {
		t.Errorf("Status() = %v, want Active", mb.Status())
	}
}

func TestOKMessageBoxHasSingleOKButton(t *testing.T) {
	mb := NewOKMessageBox("Title", "Done.")
	mb.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	if len(mb.buttonRects) != 1 {
		t.Fatalf("len(buttonRects) = %d, want 1", len(mb.buttonRects))
	}
	if mb.cfg.CancelButtonIndex != 0 {
		t.Errorf("CancelButtonIndex = %d, want 0 (the only button)", mb.cfg.CancelButtonIndex)
	}
}

func TestYesNoMessageBoxButtonOrderAndCancel(t *testing.T) {
	mb := NewYesNoMessageBox("Title", "Continue?")
	if len(mb.cfg.Buttons) != 2 || mb.cfg.Buttons[0] != "No" || mb.cfg.Buttons[1] != "Yes" {
		t.Errorf("Buttons = %v, want [No Yes]", mb.cfg.Buttons)
	}
	if mb.cfg.CancelButtonIndex != 0 {
		t.Errorf("CancelButtonIndex = %d, want 0 (\"No\")", mb.cfg.CancelButtonIndex)
	}
}

func TestYesNoCancelMessageBoxButtonOrderAndCancel(t *testing.T) {
	mb := NewYesNoCancelMessageBox("Title", "Save changes?")
	want := []string{"Cancel", "No", "Yes"}
	if len(mb.cfg.Buttons) != len(want) {
		t.Fatalf("len(Buttons) = %d, want %d", len(mb.cfg.Buttons), len(want))
	}
	for i, w := range want {
		if mb.cfg.Buttons[i] != w {
			t.Errorf("Buttons[%d] = %q, want %q", i, mb.cfg.Buttons[i], w)
		}
	}
	if mb.cfg.CancelButtonIndex != 0 {
		t.Errorf("CancelButtonIndex = %d, want 0 (\"Cancel\")", mb.cfg.CancelButtonIndex)
	}
}

func TestOKCancelMessageBoxButtonOrderAndCancel(t *testing.T) {
	mb := NewOKCancelMessageBox("Title", "Proceed?")
	want := []string{"Cancel", "OK"}
	if len(mb.cfg.Buttons) != len(want) {
		t.Fatalf("len(Buttons) = %d, want %d", len(mb.cfg.Buttons), len(want))
	}
	for i, w := range want {
		if mb.cfg.Buttons[i] != w {
			t.Errorf("Buttons[%d] = %q, want %q", i, mb.cfg.Buttons[i], w)
		}
	}
	if mb.cfg.CancelButtonIndex != 0 {
		t.Errorf("CancelButtonIndex = %d, want 0 (\"Cancel\")", mb.cfg.CancelButtonIndex)
	}
}

func TestMessageBoxClickAcceptsWithCorrectResult(t *testing.T) {
	mb := NewYesNoMessageBox("Title", "Continue?")
	mb.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	yesRect := mb.buttonRects[1] // "Yes" is the second button
	status := mb.Update(Input{MouseX: yesRect.X + 1, MouseY: yesRect.Y + 1, MousePressed: true})

	if status != Accepted {
		t.Errorf("Update() = %v, want Accepted", status)
	}
	if mb.Result() != 1 {
		t.Errorf("Result() = %d, want 1 (\"Yes\")", mb.Result())
	}
}

func TestMessageDialogConstructorsMatchMessageBoxConstructors(t *testing.T) {
	// Every NewXMessageDialog should produce the identical button
	// configuration as its NewXMessageBox counterpart -- the whole
	// point of the alias relationship.
	cases := []struct {
		name   string
		dialog *MessageDialog
		box    *MessageBox
	}{
		{"OK", NewOKMessageDialog("T", "M"), NewOKMessageBox("T", "M")},
		{"YesNo", NewYesNoMessageDialog("T", "M"), NewYesNoMessageBox("T", "M")},
		{"YesNoCancel", NewYesNoCancelMessageDialog("T", "M"), NewYesNoCancelMessageBox("T", "M")},
		{"OKCancel", NewOKCancelMessageDialog("T", "M"), NewOKCancelMessageBox("T", "M")},
	}
	for _, c := range cases {
		if len(c.dialog.cfg.Buttons) != len(c.box.cfg.Buttons) {
			t.Errorf("%s: dialog has %d buttons, box has %d", c.name, len(c.dialog.cfg.Buttons), len(c.box.cfg.Buttons))
			continue
		}
		for i := range c.dialog.cfg.Buttons {
			if c.dialog.cfg.Buttons[i] != c.box.cfg.Buttons[i] {
				t.Errorf("%s: button %d = %q (dialog) vs %q (box)", c.name, i, c.dialog.cfg.Buttons[i], c.box.cfg.Buttons[i])
			}
		}
		if c.dialog.cfg.CancelButtonIndex != c.box.cfg.CancelButtonIndex {
			t.Errorf("%s: CancelButtonIndex = %d (dialog) vs %d (box)", c.name, c.dialog.cfg.CancelButtonIndex, c.box.cfg.CancelButtonIndex)
		}
	}
}

func TestMessageBoxGenericArbitraryButtons(t *testing.T) {
	// The generic constructor isn't limited to OK/Yes/No/Cancel.
	mb := NewMessageBox(MessageBoxConfig{
		Title: "Export format", Message: "Choose a format:",
		Buttons: []string{"PNG", "JPEG", "BMP"},
	})
	mb.Draw(noopRenderer{}, 800, 600, DefaultTheme())

	if len(mb.buttonRects) != 3 {
		t.Fatalf("len(buttonRects) = %d, want 3", len(mb.buttonRects))
	}
	jpegRect := mb.buttonRects[1]
	status := mb.Update(Input{MouseX: jpegRect.X + 1, MouseY: jpegRect.Y + 1, MousePressed: true})
	if status != Accepted || mb.Result() != 1 {
		t.Errorf("clicking \"JPEG\": status=%v result=%d, want Accepted/1", status, mb.Result())
	}
}
