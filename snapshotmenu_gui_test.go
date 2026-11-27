//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/zenui"
)

// inSnapshotDir points the snapshots folder at a folder of the test's own for the test.
func inSnapshotDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "snapshots")
	was := snapshotDir
	snapshotDir = dir
	t.Cleanup(func() { snapshotDir = was })
	return dir
}

// loadTimestamped is the Load Timestamped row of the Snapshot menu, and its place there.
const loadTimestampedAt = 9 // after the Load group (title, Load Snapshot, Recent, Manage), a separator, and the next title, Quick Save, Quick Load, Save Timestamped

func loadTimestampedItems(b *appMenuBar) []zenui.Item {
	items := b.widget.ItemsFor(barSnapshot)
	if items[loadTimestampedAt].Label != "Load Timestamped" {
		return nil
	}
	return items[loadTimestampedAt].SubItems
}

func makeStamped(t *testing.T, dir string, n int, start time.Time, gap time.Duration) {
	t.Helper()
	for i := 0; i < n; i++ {
		touch(t, dir, timestampedName(start.Add(-time.Duration(i)*gap), "zxs"))
	}
}

// With no timestamped snapshot Load Timestamped says so and cannot be chosen; with a few they are
// its rows, the newest first, and no More...; with many, ten and More....
func TestLoadTimestampedRows(t *testing.T) {
	dir := inSnapshotDir(t)
	b, _ := newTestBar(t, nil, "")
	sub := loadTimestampedItems(b)
	if len(sub) != 1 || !sub[0].Disabled || sub[0].Label != "None" {
		t.Fatalf("with no snapshots: %+v", sub)
	}

	start := at(2026, 3, 9, 15, 0, 0)
	makeStamped(t, dir, 3, start, time.Hour)
	touch(t, dir, "quicksave.zxs") // not a timestamped one: not listed
	b.snapSig = ""                 // the folder was changed: as refreshSnapshotItems finds out
	b.refreshSnapshotItems()
	sub = loadTimestampedItems(b)
	if len(sub) != 3 {
		t.Fatalf("with three snapshots: %d rows %+v", len(sub), sub)
	}
	for i, want := range []string{"Mon 9 Mar 15:00:00", "Mon 9 Mar 14:00:00", "Mon 9 Mar 13:00:00"} {
		if sub[i].Label != want || sub[i].Disabled {
			t.Errorf("row %d is %q, want %q", i, sub[i].Label, want)
		}
	}

	makeStamped(t, dir, 10, start.Add(-24*time.Hour), time.Hour) // thirteen in all
	b.snapSig = ""
	b.refreshSnapshotItems()
	sub = loadTimestampedItems(b)
	if len(sub) != loadTimestampedRows+2 {
		t.Fatalf("with thirteen snapshots: %d rows", len(sub))
	}
	if !sub[loadTimestampedRows].Separator || sub[loadTimestampedRows+1].Label != "More..." {
		t.Errorf("the rows end %+v, %+v", sub[loadTimestampedRows], sub[loadTimestampedRows+1])
	}
	if sub[0].Label != "Mon 9 Mar 15:00:00" {
		t.Errorf("the first row is %q", sub[0].Label)
	}
}

// The menu is made again when the folder changes, and not otherwise.
func TestSnapshotMenuFollowsTheFolder(t *testing.T) {
	dir := inSnapshotDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	os.Chtimes(dir, past, past)
	b, _ := newTestBar(t, nil, "")
	spec := b.snapSpec
	b.refreshSnapshotItems()
	if len(b.snapSpec) != len(spec) || &b.snapSpec[0] != &spec[0] {
		t.Error("the menu was made again with nothing changed")
	}
	touch(t, dir, timestampedName(time.Now(), "zxs"))
	b.refreshSnapshotItems()
	if sub := loadTimestampedItems(b); len(sub) != 1 || sub[0].Disabled {
		t.Errorf("after a snapshot was saved: %+v", sub)
	}
	// the day is part of what the menu depends on
	now := time.Now()
	if snapshotStamp(now) == snapshotStamp(now.AddDate(0, 0, 1)) {
		t.Error("the menu does not depend on the day")
	}
}

// Choosing a row loads that snapshot into the machine, which waits paused as it does for a file
// dropped on the window, and says so.
func TestLoadTimestampedLoadsTheSnapshot(t *testing.T) {
	dir := inSnapshotDir(t)
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatal(err)
	}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := timestampedName(time.Now(), "zxs")
	if err := zx.SaveSnapshotFormat(filepath.Join(dir, name), "zxs"); err != nil {
		t.Fatal(err)
	}
	b.snapSig = ""
	b.refreshSnapshotItems()
	zx.paused = false
	got := notices(zx, func() { b.snapSpec.pick(zx, loadTimestampedAt, 0) })
	if len(got) != 1 || !strings.Contains(got[0], name) || !strings.Contains(got[0], "restored") {
		t.Errorf("loading said %q", got)
	}
	if !zx.paused {
		t.Error("the machine plays after a snapshot was loaded, with Preferences saying it waits")
	}
	// a snapshot that is not there any more
	os.Remove(filepath.Join(dir, name))
	got = notices(zx, func() { b.snapSpec.pick(zx, loadTimestampedAt, 0) })
	if len(got) != 1 || !strings.Contains(got[0], "could not be loaded") {
		t.Errorf("loading a snapshot that is gone said %q", got)
	}
}

// More... opens a menu of the older snapshots where the pointer is: grouped by day, week and month
// when there are many, with the group's snapshots as its submenu.
func TestMoreSnapshotsOpensTheGroups(t *testing.T) {
	dir := inSnapshotDir(t)
	b, zx := newTestBar(t, nil, "")
	now := time.Now()
	makeStamped(t, dir, 60, now, 7*time.Hour) // about 17 days of them
	b.snapSig = ""
	b.refreshSnapshotItems()
	sub := loadTimestampedItems(b)
	if len(sub) != loadTimestampedRows+2 {
		t.Fatalf("%d rows", len(sub))
	}
	b.snapSpec.pick(zx, loadTimestampedAt, loadTimestampedRows+1)
	if b.logoMenu == nil {
		t.Fatal("More... did not open its menu")
	}
	if !b.Active() {
		t.Error("the bar does not know a menu is open")
	}
	if len(b.logoSpec) < 3 {
		t.Fatalf("More... has %d rows", len(b.logoSpec))
	}
	total := 0
	for i, e := range b.logoSpec {
		if e.sub == nil || !strings.Contains(e.label, "(") {
			t.Errorf("row %d (%q) is not a group", i, e.label)
		}
		total += len(e.sub)
	}
	if total != 50 {
		t.Errorf("the groups hold %d snapshots, want the 50 that the menu does not", total)
	}
	plan := planSnapshotMenu(timestampedSnapshots(listSnapshots(dir)), time.Now())
	if want := groupMenuLabel(plan.More[0]); b.logoSpec[0].label != want {
		t.Errorf("the first group is %q, want the newest, %q", b.logoSpec[0].label, want)
	}
	b.closeMenus()
	if b.logoMenu != nil {
		t.Error("closing the menus left More... open")
	}
}

// A few older snapshots are More's rows as they are.
func TestMoreSnapshotsFlatWhenFew(t *testing.T) {
	dir := inSnapshotDir(t)
	b, zx := newTestBar(t, nil, "")
	makeStamped(t, dir, loadTimestampedRows+5, time.Now(), time.Hour)
	b.snapSig = ""
	b.refreshSnapshotItems()
	b.snapSpec.pick(zx, loadTimestampedAt, loadTimestampedRows+1)
	if b.logoMenu == nil || len(b.logoSpec) != 5 {
		t.Fatalf("More... has %d rows", len(b.logoSpec))
	}
	for i, e := range b.logoSpec {
		if e.sub != nil || e.run == nil {
			t.Errorf("row %d (%q) is not a snapshot to load", i, e.label)
		}
	}
	b.closeMenus()
}

// A group of more than groupRowsMax snapshots lists that many and points to the window for the
// rest, which opens.
func TestMoreSnapshotsOfAFullGroup(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	now := at(2026, 3, 11, 15, 0, 0)
	snaps := stamps(groupRowsMax+7, now, time.Minute)
	spec := b.moreSnapshotsSpec([]snapshotGroup{{Kind: groupToday, Label: "Today", Items: snaps}})
	if len(spec) != 1 || len(spec[0].sub) != groupRowsMax+2 {
		t.Fatalf("the group's menu has %d rows", len(spec[0].sub))
	}
	last := spec[0].sub[groupRowsMax+1]
	if last.label != "7 more: Manage Snapshots..." {
		t.Errorf("the last row is %q", last.label)
	}
	last.run(zx)
	if b.snaps == nil {
		t.Error("the row did not open the Snapshots window")
	}
}

// The window is a modal one: the bar knows it is open, and the fade draws it.
func TestSnapshotsWindowIsModal(t *testing.T) {
	inSnapshotDir(t)
	b, zx := newTestBar(t, nil, "")
	if b.modalOpen() {
		t.Fatal("a window is open")
	}
	manage := snapRow(t, b, "Manage Snapshots...")
	b.snapSpec.pick(zx, manage, -1)
	if b.snaps == nil || !b.modalOpen() || !b.Active() {
		t.Fatal("Manage Snapshots... did not open the window")
	}
	if b.modalDrawer(800, 600) == nil {
		t.Error("the window is not drawn by the fade")
	}
}
