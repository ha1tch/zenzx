// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
)

// Recent Snapshots: the snapshots that were named, as the menu lists them. A snapshot is named when
// it is saved with Save As (anywhere, under a name of the person's choice) or loaded with Load
// Snapshot; the ones that Save Timestamped and Quick Save make have menus of their own and are not
// kept here. The list is kept in the settings file, newest first.

// recentSnapshotsMax is how many are kept.
const recentSnapshotsMax = 15

// snapshotLoadExts are the files that Load Snapshot offers: the formats a machine's state is loaded
// from (not .rzx, which plays a recording).
var snapshotLoadExts = []string{"zxs", "sna", "z80", "szx", MultiSnapshotExt}

// isNamedSnapshot says the file at path is a snapshot that someone named: of a format that is
// loaded, and neither one that Save Timestamped made nor the quick save.
func isNamedSnapshot(path string) bool {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(base))
	format := strings.TrimPrefix(ext, ".")
	loadable := false
	for _, e := range snapshotLoadExts {
		loadable = loadable || e == format
	}
	if !loadable {
		return false
	}
	if _, _, stamped := parseTimestampedName(base); stamped {
		return false
	}
	return strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base))) != "quicksave"
}

// addRecentSnapshot is list with the snapshot at path first, and once only: the list as it was
// when it is not a named snapshot. Paths are kept absolute, so that a different working folder
// does not make the same file two.
func addRecentSnapshot(list []string, path string) []string {
	if !isNamedSnapshot(path) {
		return list
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	out := make([]string, 0, len(list)+1)
	out = append(out, path)
	for _, p := range list {
		if p != path {
			out = append(out, p)
		}
	}
	if len(out) > recentSnapshotsMax {
		out = out[:recentSnapshotsMax]
	}
	return out
}

// existingSnapshots is the files of list that are still there.
func existingSnapshots(list []string) []string {
	var out []string
	for _, p := range list {
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			out = append(out, p)
		}
	}
	return out
}

// recentLabels are the names the menu shows for paths: the file's name, and for two of one name
// in different folders, the folder as well.
func recentLabels(paths []string) []string {
	count := map[string]int{}
	for _, p := range paths {
		count[filepath.Base(p)]++
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
		if count[out[i]] > 1 {
			out[i] += " (" + filepath.Base(filepath.Dir(p)) + ")"
		}
	}
	return out
}
