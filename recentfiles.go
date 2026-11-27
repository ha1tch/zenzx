// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
)

// Recent files: the tapes, disks, snapshots, recordings, screens and programs that were opened with
// File > Open or Load Disk, or given to the window, as the Recent place of those dialogs lists
// them (zenui's Dialog filters the list by the file type chosen, so each medium sees its own). The
// list is kept in the settings file, newest first. Recent Snapshots (snapshotrecent.go) is the
// snapshots' own menu and is kept apart.

// fileOpenExts are the extensions File > Open offers: everything a drop loads.
var fileOpenExts = []string{"tap", "tzx", "sna", "z80", "szx", "zxs", "zms", "rzx", "scr", "dsk", "nex"}

// recentFilesMax is how many are kept.
const recentFilesMax = 20

// isOpenableFile says path is of a kind File > Open takes: one of fileOpenExts. (Snapshots that
// Save Timestamped or Quick Save make are files like the rest, and may be listed.)
func isOpenableFile(path string) bool {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	for _, e := range fileOpenExts {
		if e == ext {
			return true
		}
	}
	return false
}

// addRecentFile is list with the file at path first, and once only: the list as it was when the
// file is not of a kind that is opened. Paths are kept absolute, so that a different working folder
// does not make the same file two.
func addRecentFile(list []string, path string) []string {
	if path == "" || !isOpenableFile(path) {
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
	if len(out) > recentFilesMax {
		out = out[:recentFilesMax]
	}
	return out
}

// existingFiles is the files of list that are still there, as existingSnapshots makes it for the
// snapshots: a Recent place does not offer what has gone.
func existingFiles(list []string) []string { return existingSnapshots(list) }

// openStartDir is the folder an Open dialog starts in: the folder kept from the file opened last,
// while it is still there; otherwise the home folder. The folder the program was started in is
// not a choice: from a launcher or a packaged program it is whatever the launcher sets.
func openStartDir(kept string) string {
	if kept != "" {
		if st, err := os.Stat(kept); err == nil && st.IsDir() {
			return kept
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return "."
}
