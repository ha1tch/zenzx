// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The snapshots folder. Quick Save, Save Timestamped and Save As put the machine's snapshots in
// one folder, snapshots/ in the folder ZenZX runs in (the way custom-roms/ is), and Quick Load,
// Load Timestamped, Snapshot Info and the Manage Snapshots window read it. Until 0.50.0 they went
// into the folder ZenZX runs in itself, among whatever else was there; migrateSnapshots moves
// those that ZenZX wrote.

// SnapshotDir is the snapshots folder's default name, as CustomROMDir is the custom ROMs'.
const SnapshotDir = "snapshots"

// snapshotDir is the snapshots folder in use: SnapshotDir, or what -snapshots-dir says.
var snapshotDir = SnapshotDir

// snapshotFormats are the formats ZenZX writes and the folder is read for, the one Quick Save
// uses first.
var snapshotFormats = []string{"zxs", "sna", "z80"}

// snapshotTimeLayout is the date and time in a timestamped snapshot's name.
const snapshotTimeLayout = "20060102_150405"

// snapshotPath is the path of a file of the snapshots folder.
func snapshotPath(name string) string { return filepath.Join(snapshotDir, name) }

// ensureSnapshotDir makes the folder, when it is not there.
func ensureSnapshotDir() error { return os.MkdirAll(snapshotDir, 0o755) }

// quickSaveName is the name of the quick save in a format.
func quickSaveName(format string) string { return "quicksave." + format }

// timestampedName is the name Save Timestamped gives a snapshot made at t.
func timestampedName(t time.Time, format string) string {
	return "snapshot_" + t.Format(snapshotTimeLayout) + "." + format
}

// parseTimestampedName reads a name made by timestampedName: when the snapshot was made and in
// which format. ok is false for any other name, which includes a timestamped snapshot that has
// been renamed or copied (Manage Snapshots), so that it is not offered by Load Timestamped: that
// is where the ones that Save Timestamped made are.
func parseTimestampedName(name string) (when time.Time, format string, ok bool) {
	ext := strings.ToLower(filepath.Ext(name))
	format = strings.TrimPrefix(ext, ".")
	if !isSnapshotFormat(format) {
		return time.Time{}, "", false
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	const prefix = "snapshot_"
	if !strings.HasPrefix(stem, prefix) || len(stem) != len(prefix)+len(snapshotTimeLayout) {
		return time.Time{}, "", false
	}
	when, err := time.ParseInLocation(snapshotTimeLayout, stem[len(prefix):], time.Local)
	if err != nil {
		return time.Time{}, "", false
	}
	return when, format, true
}

func isSnapshotFormat(format string) bool {
	for _, f := range snapshotFormats {
		if f == format {
			return true
		}
	}
	return false
}

// snapshotFile is one snapshot in the folder.
type snapshotFile struct {
	Path        string
	Name        string
	Format      string // "zxs", "sna" or "z80", or "zms" for a multisnapshot
	Size        int64
	When        time.Time // when it was made: from its name when it is timestamped, otherwise the file's
	Timestamped bool      // the name is one Save Timestamped made
	Quick       bool      // it is the quick save
}

// listSnapshots reads the snapshots in dir, the newest first. Anything that is not a snapshot of
// one of the formats is left out, and so is a folder.
func listSnapshots(dir string) []snapshotFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []snapshotFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
		if !isSnapshotFormat(format) && format != MultiSnapshotExt {
			continue // a multisnapshot is listed with the others, and loaded by its own way
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		f := snapshotFile{Path: filepath.Join(dir, name), Name: name, Format: format, Size: info.Size(), When: info.ModTime()}
		if when, _, ok := parseTimestampedName(name); ok {
			f.When, f.Timestamped = when, true
		}
		f.Quick = format != MultiSnapshotExt && strings.TrimSuffix(strings.ToLower(name), "."+format) == "quicksave"
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].When.Equal(out[j].When) {
			return out[i].When.After(out[j].When)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// timestampedSnapshots are the snapshots that Save Timestamped made, the newest first.
func timestampedSnapshots(all []snapshotFile) []snapshotFile {
	var out []snapshotFile
	for _, f := range all {
		if f.Timestamped {
			out = append(out, f)
		}
	}
	return out
}

// snapshotSignature says whether the folder has changed since it was last read, cheaply: the
// folder's own modification time and how big it is. A save, a copy, a rename or a delete in the
// folder changes its time; the menus read the folder again when this does.
func snapshotSignature(dir string) string {
	st, err := os.Stat(dir)
	if err != nil {
		return "none"
	}
	return fmt.Sprintf("%d/%d", st.ModTime().UnixNano(), st.Size())
}

// isLegacySnapshotName is a name that ZenZX gave a snapshot before they had a folder: the quick
// save in any format, and a timestamped one.
func isLegacySnapshotName(name string) bool {
	format := strings.TrimPrefix(filepath.Ext(name), ".")
	if !isSnapshotFormat(format) {
		return false
	}
	if strings.TrimSuffix(name, filepath.Ext(name)) == "quicksave" {
		return true
	}
	_, _, ok := parseTimestampedName(name)
	return ok
}

// migrateSnapshots moves the snapshots that ZenZX wrote into the folder ZenZX runs in (from) to
// the snapshots folder (to). A file with the name of one that is in the folder already stays
// where it is, and is reported in kept: nothing is written over. It is for a person who has
// snapshots from before they had a folder.
func migrateSnapshots(from, to string) (moved, kept []string, err error) {
	entries, rerr := os.ReadDir(from)
	if rerr != nil {
		return nil, nil, nil
	}
	for _, e := range entries {
		if e.IsDir() || !isLegacySnapshotName(e.Name()) {
			continue
		}
		src, dst := filepath.Join(from, e.Name()), filepath.Join(to, e.Name())
		if _, serr := os.Lstat(dst); serr == nil {
			kept = append(kept, e.Name())
			continue
		}
		if err = os.MkdirAll(to, 0o755); err != nil {
			return moved, kept, err
		}
		if err = os.Rename(src, dst); err != nil {
			return moved, kept, fmt.Errorf("moving %s: %w", e.Name(), err)
		}
		moved = append(moved, e.Name())
	}
	return moved, kept, nil
}

// migrateLegacySnapshots is migrateSnapshots from the folder ZenZX runs in to the snapshots
// folder, saying what it did, on the console and in the lines it returns (for the window to show).
func migrateLegacySnapshots() (notes []string) {
	if a, aerr := filepath.Abs("."); aerr == nil {
		if b, berr := filepath.Abs(snapshotDir); berr == nil && a == b {
			return nil // the snapshots folder is the folder ZenZX runs in: there is nowhere to move them to
		}
	}
	moved, kept, err := migrateSnapshots(".", snapshotDir)
	if len(moved) > 0 {
		sayf("Snapshots: %d moved into %s/ (%s)\n", len(moved), snapshotDir, strings.Join(moved, ", "))
		notes = append(notes, fmt.Sprintf("%d snapshots from before the snapshots folder were moved into %s/", len(moved), snapshotDir))
	}
	for _, n := range kept {
		sayf("Snapshots: %s stays where it is: %s/%s is there already\n", n, snapshotDir, n)
		notes = append(notes, fmt.Sprintf("%s was not moved into %s/: it has one of that name already", n, snapshotDir))
	}
	if err != nil {
		sayf("Snapshots: %v\n", err)
		notes = append(notes, fmt.Sprintf("The snapshots could not all be moved into %s/: %v", snapshotDir, err))
	}
	return notes
}
