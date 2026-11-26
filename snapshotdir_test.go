package zenzx

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func touch(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// A name that Save Timestamped makes reads back to the time and format it was made with; no other
// name does.
func TestTimestampedNamesRoundTrip(t *testing.T) {
	when := time.Date(2026, 3, 9, 14, 5, 7, 0, time.Local)
	for _, format := range snapshotFormats {
		name := timestampedName(when, format)
		got, f, ok := parseTimestampedName(name)
		if !ok || !got.Equal(when) || f != format {
			t.Errorf("%s reads as %v %q %v", name, got, f, ok)
		}
	}
	for _, name := range []string{
		"quicksave.zxs", "snapshot_20260309_140507.tap", "snapshot_20260309_1405.zxs",
		"snapshot_20260309_140507-2.zxs", "mysnapshot_20260309_140507.zxs",
		"snapshot_20261309_140507.zxs", "snapshot_20260309_246007.zxs", "snapshot_.zxs", "",
	} {
		if _, _, ok := parseTimestampedName(name); ok {
			t.Errorf("%q reads as a timestamped name", name)
		}
	}
	if got, _, ok := parseTimestampedName("snapshot_20260309_140507.ZXS"); !ok || !got.Equal(when) {
		t.Errorf("an upper-case extension is not read: %v %v", got, ok)
	}
}

// listSnapshots lists snapshots and nothing else, the newest first, a timestamped one by the time
// in its name and any other by the file's.
func TestListSnapshots(t *testing.T) {
	dir := t.TempDir()
	old := touch(t, dir, "snapshot_20250101_080000.zxs")
	mid := touch(t, dir, "snapshot_20260301_100000.sna")
	newest := touch(t, dir, "snapshot_20260309_140507.zxs")
	quick := touch(t, dir, "quicksave.z80")
	other := touch(t, dir, "my game.zxs")
	touch(t, dir, "notes.txt")
	touch(t, dir, "game.tap")
	if err := os.Mkdir(filepath.Join(dir, "sub.zxs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// the quick save and the other one are given times in the middle
	at := func(p string, when time.Time) {
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
	at(quick, time.Date(2026, 3, 5, 9, 0, 0, 0, time.Local))
	at(other, time.Date(2026, 3, 7, 9, 0, 0, 0, time.Local))

	got := listSnapshots(dir)
	var names []string
	for _, f := range got {
		names = append(names, f.Name)
	}
	want := []string{filepath.Base(newest), "my game.zxs", "quicksave.z80", filepath.Base(mid), filepath.Base(old)}
	if len(names) != len(want) {
		t.Fatalf("listed %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("listed %v, want %v", names, want)
		}
	}
	for _, f := range got {
		switch f.Name {
		case "quicksave.z80":
			if !f.Quick || f.Timestamped || f.Format != "z80" {
				t.Errorf("the quick save is %+v", f)
			}
		case "my game.zxs":
			if f.Quick || f.Timestamped {
				t.Errorf("another snapshot is %+v", f)
			}
		case filepath.Base(mid):
			if !f.Timestamped || f.Format != "sna" || !f.When.Equal(time.Date(2026, 3, 1, 10, 0, 0, 0, time.Local)) {
				t.Errorf("a timestamped snapshot is %+v", f)
			}
		}
	}
	stamped := timestampedSnapshots(got)
	if len(stamped) != 3 || stamped[0].Name != filepath.Base(newest) {
		t.Errorf("the timestamped snapshots are %+v", stamped)
	}
	if listSnapshots(filepath.Join(dir, "nowhere")) != nil {
		t.Error("a folder that is not there lists snapshots")
	}
}

// The signature of the folder changes when a file is made in it, and not otherwise.
func TestSnapshotSignature(t *testing.T) {
	dir := t.TempDir()
	if snapshotSignature(filepath.Join(dir, "nowhere")) != "none" {
		t.Error("a folder that is not there has a signature")
	}
	a := snapshotSignature(dir)
	if b := snapshotSignature(dir); a != b {
		t.Errorf("the signature of an unchanged folder changed: %s, %s", a, b)
	}
	// directory times can be coarse: set it back, then make a file
	past := time.Now().Add(-time.Hour)
	os.Chtimes(dir, past, past)
	a = snapshotSignature(dir)
	touch(t, dir, "quicksave.zxs")
	if b := snapshotSignature(dir); a == b {
		t.Error("the signature did not change when a snapshot was made")
	}
}

// Snapshots from before the folder are moved into it, and only those: the quick save, and a
// timestamped one, in a format. Nothing is written over.
func TestMigrateSnapshots(t *testing.T) {
	from, to := t.TempDir(), filepath.Join(t.TempDir(), "snapshots")
	touch(t, from, "quicksave.zxs")
	touch(t, from, "quicksave.sna")
	touch(t, from, "snapshot_20260309_140507.zxs")
	touch(t, from, "snapshot_20260309_140508.z80")
	// not ZenZX's
	touch(t, from, "my game.zxs")
	touch(t, from, "snapshot_20260309_1405.zxs")
	touch(t, from, "quicksave.txt")
	touch(t, from, "disk_20260309_140507.dsk")
	// already in the folder: it stays where it is
	touch(t, to, "quicksave.sna")
	if err := os.WriteFile(filepath.Join(to, "quicksave.sna"), []byte("the one in the folder"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, kept, err := migrateSnapshots(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 3 || len(kept) != 1 || kept[0] != "quicksave.sna" {
		t.Errorf("moved %v, kept %v", moved, kept)
	}
	for _, n := range []string{"quicksave.zxs", "snapshot_20260309_140507.zxs", "snapshot_20260309_140508.z80"} {
		if _, err := os.Stat(filepath.Join(to, n)); err != nil {
			t.Errorf("%s was not moved: %v", n, err)
		}
		if _, err := os.Stat(filepath.Join(from, n)); err == nil {
			t.Errorf("%s is in both folders", n)
		}
	}
	for _, n := range []string{"my game.zxs", "snapshot_20260309_1405.zxs", "quicksave.txt", "disk_20260309_140507.dsk", "quicksave.sna"} {
		if _, err := os.Stat(filepath.Join(from, n)); err != nil {
			t.Errorf("%s was moved, and is not a snapshot ZenZX made: %v", n, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(to, "quicksave.sna")); string(b) != "the one in the folder" {
		t.Errorf("the snapshot in the folder was written over: %q", b)
	}
	// again: nothing left to move
	moved, _, err = migrateSnapshots(from, to)
	if err != nil || len(moved) != 0 {
		t.Errorf("a second migration moved %v (%v)", moved, err)
	}
	// a folder that is not there has nothing to move
	if moved, kept, err := migrateSnapshots(filepath.Join(from, "nowhere"), to); moved != nil || kept != nil || err != nil {
		t.Errorf("%v %v %v", moved, kept, err)
	}
}

// With the snapshots folder being the folder ZenZX runs in there is nowhere to move them.
func TestMigrateLegacySnapshotsInTheSameFolder(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "quicksave.zxs")
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	was := snapshotDir
	defer func() { snapshotDir = was }()
	snapshotDir = "."
	migrateLegacySnapshots()
	if _, err := os.Stat(filepath.Join(dir, "quicksave.zxs")); err != nil {
		t.Errorf("the quick save moved: %v", err)
	}
	snapshotDir = "snapshots"
	migrateLegacySnapshots()
	if _, err := os.Stat(filepath.Join(dir, "snapshots", "quicksave.zxs")); err != nil {
		t.Errorf("the quick save was not moved into snapshots/: %v", err)
	}
}
