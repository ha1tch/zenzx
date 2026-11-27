// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The tapes folder, and the tapes that were loaded lately. Load Tape starts in the folder the last
// tape came from, and in the tapes folder, tapes/ in the folder ZenZX runs in (as snapshots/ and
// screenshots/ are), when there is none. Manage Tapes lists what is in the tapes folder, and the
// tapes loaded from elsewhere, which Recent Tapes lists too.

// TapeDir is the tapes folder's default name, as SnapshotDir and ScreenshotDir are the others'.
const TapeDir = "tapes"

// tapeDir is the tapes folder in use: TapeDir, or what -tapes-dir says.
var tapeDir = TapeDir

// tapeExts are the formats of tape the deck takes.
var tapeExts = []string{"tap", "tzx"}

// recentTapesMax is how many loaded tapes are kept; the Recent Tapes menu lists the first
// recentTapesMenu of them and Manage Tapes the rest.
const (
	recentTapesMax  = 60
	recentTapesMenu = 15
)

// tapeInsertedHook is told of each tape that goes into a deck by being loaded (not one an example
// brings with it), with its path. The window sets it, to keep the Recent Tapes.
var tapeInsertedHook func(path string)

// isTapePath says path names a tape of a format the deck takes.
func isTapePath(path string) bool {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	for _, e := range tapeExts {
		if e == ext {
			return true
		}
	}
	return false
}

// ensureTapeDir makes the tapes folder, when it is not there.
func ensureTapeDir() error { return os.MkdirAll(tapeDir, 0o755) }

// tapeFile is one tape the Manage Tapes window lists.
type tapeFile struct {
	Path string
	Name string
	Ext  string // "tap" or "tzx"
	Size int64
	When time.Time
	// Here says the tape is in the tapes folder.
	Here bool
}

func newTapeFile(path string, info os.FileInfo, here bool) tapeFile {
	return tapeFile{Path: path, Name: filepath.Base(path), Ext: strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
		Size: info.Size(), When: info.ModTime(), Here: here}
}

// tapeLibrary is the tapes Manage Tapes lists: those loaded lately that are still there, newest
// first, and then the others in the tapes folder by name. A tape is listed once.
func tapeLibrary(dir string, recent []string) []tapeFile {
	absDir, _ := filepath.Abs(dir)
	here := func(p string) bool { return absDir != "" && filepath.Dir(p) == absDir }
	var out []tapeFile
	seen := map[string]bool{}
	for _, p := range recent {
		if seen[p] || !isTapePath(p) {
			continue
		}
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			seen[p] = true
			out = append(out, newTapeFile(p, st, here(p)))
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	var rest []tapeFile
	for _, e := range entries {
		if e.IsDir() || !isTapePath(e.Name()) {
			continue
		}
		p := filepath.Join(absDir, e.Name())
		info, err := e.Info()
		if seen[p] || err != nil || !info.Mode().IsRegular() {
			continue
		}
		rest = append(rest, newTapeFile(p, info, true))
	}
	sort.SliceStable(rest, func(i, j int) bool {
		return strings.ToLower(rest[i].Name) < strings.ToLower(rest[j].Name)
	})
	return append(out, rest...)
}

// addRecentTape is list with the tape at path first, and once only: the list as it was when path is
// not a tape. Paths are kept absolute, so that a different working folder does not make the same
// file two.
func addRecentTape(list []string, path string) []string {
	if path == "" || !isTapePath(path) {
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
	if len(out) > recentTapesMax {
		out = out[:recentTapesMax]
	}
	return out
}

// renameRecentTape is list with the tape at old called target instead.
func renameRecentTape(list []string, old, target string) []string {
	out := append([]string(nil), list...)
	for i, p := range out {
		if p == old {
			out[i] = target
		}
	}
	return out
}

// tapeStartDir is the folder Load Tape starts in: the folder of the tape loaded last, while it is
// still there; otherwise the tapes folder, which is made when it is not there; and when it cannot
// be made, the folder an Open dialog starts in. recent is the tapes loaded lately that are still
// there, newest first.
func tapeStartDir(recent []string) string {
	if len(recent) > 0 {
		dir := filepath.Dir(recent[0])
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir
		}
	}
	if err := ensureTapeDir(); err == nil {
		if abs, err := filepath.Abs(tapeDir); err == nil {
			return abs
		}
		return tapeDir
	}
	return openStartDir("")
}
