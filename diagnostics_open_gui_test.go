//go:build !headless

package zenzx

import (
	"strings"
	"testing"
)

// Snapshot > Run Diagnostics, and F3 (which asks the program to do the same, zx.OnDiagnostics),
// open the report in a window; the View menu has no Show Status row.
func TestDiagnosticsOpenInAWindow(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	if b.activeModal != nil {
		t.Fatal("setup: a window is open")
	}
	b.showDiagnostics(zx)
	if b.activeModal == nil {
		t.Fatal("showDiagnostics opened no window")
	}
	b.activeModal = nil

	b.pickMenuRow(t, zx, barSnapshot, "Run Diagnostics")
	if b.activeModal == nil {
		t.Fatal("Snapshot > Run Diagnostics opened no window")
	}
	b.activeModal = nil

	for _, it := range b.widget.ItemsFor(barView) {
		if strings.Contains(it.Label, "Status") {
			t.Errorf("the View menu has the row %q", it.Label)
		}
	}
}
