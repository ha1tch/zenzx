//go:build !headless

package zenzx

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenui"
)

// With no machine running, what loads a snapshot can be chosen in the Snapshot menu, and the rows
// that act on a machine are greyed; with one running none is.
func TestTheSnapshotMenuWithNoMachineRunning(t *testing.T) {
	inSnapshotDir(t)
	b, zx := newTestBar(t, nil, "")
	zx.stopped = true
	b.refreshSnapshotItemsFor(zx)
	items := b.widget.ItemsFor(barSnapshot)
	if len(items) != len(b.snapSpec) {
		t.Fatalf("%d items for %d rows", len(items), len(b.snapSpec))
	}
	enabledWithNone := map[string]bool{"Quick Load": true, "Load Snapshot...": true, "Load Multisnapshot...": true, "Manage Snapshots...": true}
	for i, e := range b.snapSpec {
		if e.title || e.sep {
			continue
		}
		if e.label == "Load Timestamped" || e.label == "Recent Snapshots" {
			continue // its rows are the snapshots, or None
		}
		want := !enabledWithNone[e.label]
		if items[i].Disabled != want {
			t.Errorf("with no machine, %q is disabled: %v, want %v", e.label, items[i].Disabled, want)
		}
	}
	zx.stopped = false
	b.refreshSnapshotItemsFor(zx)
	for i, it := range b.widget.ItemsFor(barSnapshot) {
		// Save Multisnapshot is of two machines or more, and one is running
		if it.Label != "Load Timestamped" && it.Label != "Recent Snapshots" && it.Label != "Save Multisnapshot..." && it.Disabled {
			t.Errorf("with a machine running, %q (row %d) is disabled", it.Label, i)
		}
	}
}

// Quick Load with no machine running loads the quick save as a window does (openSnapshot), and does
// not ask for a machine first.
func TestQuickLoadWorksWithNoMachineRunning(t *testing.T) {
	dir := inSnapshotDir(t)
	b, zx := newTestBar(t, nil, "")
	if err := ensureSnapshotDir(); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, quickSaveName("zxs"))
	if err := zx.SaveSnapshot(want); err != nil {
		t.Fatal(err)
	}
	zx.stopped = true
	var opened []string
	zx.OpenSnapshot = func(path, format string, wait bool) error {
		opened = append(opened, fmt.Sprint(path, "|", format, "|", wait))
		return nil
	}
	b.refreshSnapshotItemsFor(zx)
	for i, e := range b.snapSpec {
		if e.label == "Quick Load" {
			if b.needsMachine(zenui.MenuBarSelection{BarIndex: barSnapshot, ItemIndex: i}) {
				t.Error("Quick Load is said to need a machine")
			}
			b.snapSpec.pick(zx, i, -1)
		}
	}
	if len(opened) != 1 || opened[0] != want+"|zxs|true" {
		t.Errorf("Quick Load opened %v, want the quick save", opened)
	}
}
