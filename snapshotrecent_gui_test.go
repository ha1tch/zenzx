//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

func snapshotRows(t *testing.T, b *appMenuBar, label string) []string {
	t.Helper()
	b.refreshSnapshotItems()
	var rows []string
	for _, it := range b.widget.ItemsFor(barSnapshot) {
		if it.Label == label {
			for _, s := range it.SubItems {
				rows = append(rows, s.Label)
			}
			return rows
		}
	}
	t.Fatalf("no %q row in the Snapshot menu", label)
	return nil
}

// Load Snapshot... is in the menu and can be chosen with no machine running; Recent Snapshots says
// None until there is one.
func TestLoadSnapshotRowsWithNoMachine(t *testing.T) {
	inSnapshotDir(t)
	b, zx := newTestBar(t, nil, "")
	zx.stopped = true
	b.snapSig = ""
	b.refreshSnapshotItemsFor(zx)
	var load, recent = -1, -1
	for i, it := range b.widget.ItemsFor(barSnapshot) {
		switch it.Label {
		case "Load Snapshot...":
			load = i
			if it.Disabled {
				t.Error("Load Snapshot... is disabled with no machine running")
			}
		case "Recent Snapshots":
			recent = i
			if it.Disabled {
				t.Error("Recent Snapshots is disabled with no machine running")
			}
		}
	}
	if load < 0 || recent < 0 {
		t.Fatalf("rows missing: load %d, recent %d", load, recent)
	}
	if rows := snapshotRows(t, b, "Recent Snapshots"); len(rows) != 1 || rows[0] != "None" {
		t.Errorf("with none recent: %v", rows)
	}
	// choosing Load Snapshot... opens the browser for the load
	b.snapSpec.pick(zx, load, -1)
	if b.diskDialog == nil || b.dialogKind != dialogLoadSnapshot {
		t.Errorf("the dialog is %v of kind %v", b.diskDialog, b.dialogKind)
	}
}

// A named snapshot that is saved or loaded becomes a row of Recent Snapshots, newest first; one that
// is gone is not shown; the list is kept in the settings; the menu is made again when it changes.
func TestRecentSnapshotsMenu(t *testing.T) {
	inSnapshotDir(t)
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	b, zx := newTestBar(t, nil, settings)
	a := touch(t, dir, "alpha.zxs")
	c := touch(t, dir, "gamma.sna")
	b.noteRecentSnapshot(zx, a)
	b.noteRecentSnapshot(zx, c)
	if rows := snapshotRows(t, b, "Recent Snapshots"); len(rows) != 2 || rows[0] != "gamma.sna" || rows[1] != "alpha.zxs" {
		t.Fatalf("rows: %v", rows)
	}
	b.noteRecentSnapshot(zx, filepath.Join(dir, "quicksave.zxs")) // not named ones
	b.noteRecentSnapshot(zx, filepath.Join(dir, timestampedName(time.Now(), "zxs")))
	if len(b.recentSnaps) != 2 {
		t.Errorf("kept %v", b.recentSnaps)
	}
	res, err := settingsconfig.Load(settings, defaultSettingsJSON, validThemeNames, validFontNames)
	if err != nil || !res.FromDisk {
		t.Fatalf("what the bar wrote was refused: %v %v", err, res)
	}
	s := res.Settings
	if got := s.RecentSnapshotPaths(); len(got) != 2 || got[0] != c || got[1] != a {
		t.Errorf("the settings hold %v", got)
	}
	// a new bar from those settings has them
	b2, _ := newTestBar(t, s, settings)
	if rows := snapshotRows(t, b2, "Recent Snapshots"); len(rows) != 2 || rows[0] != "gamma.sna" {
		t.Errorf("after a restart: %v", rows)
	}
	// a file that went is not listed
	if err := os.Remove(c); err != nil {
		t.Fatal(err)
	}
	if rows := snapshotRows(t, b, "Recent Snapshots"); len(rows) != 1 || rows[0] != "alpha.zxs" {
		t.Errorf("with one gone: %v", rows)
	}
}

// snapRow is the place of the Snapshot menu's row called label.
func snapRow(t *testing.T, b *appMenuBar, label string) int {
	t.Helper()
	for i, it := range b.widget.ItemsFor(barSnapshot) {
		if it.Label == label {
			return i
		}
	}
	t.Fatalf("no %q row in the Snapshot menu", label)
	return -1
}
