//go:build !headless

package zenzx

import "testing"

// While a window or dialog is open the bar owns the keyboard, and the main loop does not give it to
// the machine: Page Up and Page Down, which zoom the picture, are never read as zoom with a dialog
// up. They belong to the dialog (the hex dump pages with them). Every window has to count as open.
func TestEveryWindowOwnsTheKeyboardWhileOpen(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	if b.Active() {
		t.Fatal("a bar with nothing open owns the keyboard")
	}
	for _, c := range []struct {
		name  string
		open  func()
		close func()
	}{
		{"Load Binary", func() { b.openBinary(zx, binTabLoad) }, func() { b.bin = nil }},
		{"Hot Memory", func() { b.openBinary(zx, binTabHot) }, func() { b.bin = nil }},
		{"Peek and Poke", func() { b.openBinary(zx, binTabPeek) }, func() { b.bin = nil }},
		{"Export", func() { b.openExportAsk("48k") }, func() { b.exportAsk = nil }},
		{"Preferences", func() { b.openPrefs(zx) }, func() { b.prefs = nil }},
		{"Snapshots", func() { b.openSnapshots(zx) }, func() { b.snaps = nil }},
		{"an information window", func() { b.showInfo("Info", "text") }, func() { b.activeModal = nil }},
	} {
		c.open()
		if !b.Active() || !b.modalOpen() {
			t.Errorf("%s is open and the bar does not own the keyboard", c.name)
		}
		c.close()
		if b.Active() {
			t.Errorf("%s is closed and the bar still owns the keyboard", c.name)
		}
	}
}
