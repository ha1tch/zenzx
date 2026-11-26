package zenzx

import (
	"path/filepath"
	"testing"
	"time"
)

// A snapshot is a named one when it is of a format that is loaded and was not made by Save
// Timestamped or Quick Save: those have menus of their own.
func TestIsNamedSnapshot(t *testing.T) {
	stamped := timestampedName(time.Date(2026, 3, 9, 15, 0, 0, 0, time.Local), "zxs")
	for path, want := range map[string]bool{
		"/a/warhawk.zxs":      true,
		"/a/Warhawk.SNA":      true,
		"b.z80":               true,
		"/a/c.szx":            true,
		"/a/" + stamped:       false,
		"/a/quicksave.zxs":    false,
		"/a/QuickSave.z80":    false,
		"/a/game.tap":         false,
		"/a/game.rzx":         false,
		"/a/noextension":      false,
		"/a/quicksave2.zxs":   true, // only the quick save's own name is left out
		"/a/my.quicksave.sna": true,
	} {
		if got := isNamedSnapshot(path); got != want {
			t.Errorf("isNamedSnapshot(%q) = %v, want %v", path, got, want)
		}
	}
}

// The recent snapshots are newest first, once each, and no more than the cap.
func TestAddRecentSnapshot(t *testing.T) {
	dir := t.TempDir()
	p := func(n string) string { return filepath.Join(dir, n) }
	var list []string
	list = addRecentSnapshot(list, p("a.zxs"))
	list = addRecentSnapshot(list, p("b.sna"))
	list = addRecentSnapshot(list, p("a.zxs")) // again: moves to the top, does not repeat
	if len(list) != 2 || list[0] != p("a.zxs") || list[1] != p("b.sna") {
		t.Fatalf("list is %v", list)
	}
	before := list
	list = addRecentSnapshot(list, p("quicksave.zxs"))
	list = addRecentSnapshot(list, p(timestampedName(time.Now(), "zxs")))
	list = addRecentSnapshot(list, p("tape.tap"))
	if len(list) != 2 || list[0] != before[0] {
		t.Errorf("a snapshot that is not a named one was kept: %v", list)
	}
	for i := 0; i < recentSnapshotsMax+5; i++ {
		list = addRecentSnapshot(list, p(string(rune('a'+i))+"x.zxs"))
	}
	if len(list) != recentSnapshotsMax {
		t.Errorf("%d kept, want %d", len(list), recentSnapshotsMax)
	}
	if want := p(string(rune('a'+recentSnapshotsMax+4)) + "x.zxs"); list[0] != want {
		t.Errorf("the newest is %q, want %q", list[0], want)
	}
	// a relative path is kept as an absolute one, so that the same file is not two
	rel := addRecentSnapshot(nil, "rel.zxs")
	if len(rel) != 1 || !filepath.IsAbs(rel[0]) {
		t.Errorf("relative path kept as %v", rel)
	}
	if again := addRecentSnapshot(rel, filepath.Base(rel[0])); len(again) != 1 {
		t.Errorf("the same file twice: %v", again)
	}
}

// Files that are gone are left out; two of one name show the folder as well.
func TestExistingSnapshotsAndLabels(t *testing.T) {
	dir := t.TempDir()
	a := touch(t, filepath.Join(dir, "one"), "game.zxs")
	b := touch(t, filepath.Join(dir, "two"), "game.zxs")
	c := touch(t, dir, "solo.sna")
	gone := filepath.Join(dir, "gone.zxs")
	got := existingSnapshots([]string{a, gone, b, c})
	if len(got) != 3 || got[0] != a || got[1] != b || got[2] != c {
		t.Fatalf("existing: %v", got)
	}
	labels := recentLabels(got)
	want := []string{"game.zxs (one)", "game.zxs (two)", "solo.sna"}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("label %d is %q, want %q", i, labels[i], want[i])
		}
	}
	if existingSnapshots(nil) != nil {
		t.Error("nothing is not nothing")
	}
}
