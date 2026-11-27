//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/zenui"
)

// diskBar is a menu bar on a machine with a drive, with the home folder and the working directory
// of their own.
func diskBar(t *testing.T) (*appMenuBar, *ZenZX) {
	t.Helper()
	homeAndCwd(t)
	zx := testZXWithDrive()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	t.Cleanup(b.text.Unload)
	return b, zx
}

func diskLabels(spec menuSpec) []string {
	var out []string
	for _, e := range spec {
		if e.sep {
			out = append(out, "-")
		} else {
			out = append(out, e.label)
		}
	}
	return out
}

func TestRecentDisksListsOnlyTheDisksNewestFirst(t *testing.T) {
	b, zx := diskBar(t)
	dir := t.TempDir()
	for _, n := range []string{"first.dsk", "tune.tap", "snap.z80", "second.DSK"} {
		b.noteRecentFile(zx, writeTestFile(t, dir, n))
	}
	got := diskLabels(b.recentDisksSpec())
	if len(got) != 2 || !strings.Contains(got[0], "second.DSK") || !strings.Contains(got[1], "first.dsk") {
		t.Errorf("Recent Disks is %v, want the two disks, the newest first", got)
	}
}

func TestRecentDisksSaysNoneWhenThereAreNone(t *testing.T) {
	b, zx := diskBar(t)
	spec := b.recentDisksSpec()
	if len(spec) != 1 || spec[0].label != "None" || !spec[0].disabled {
		t.Fatalf("with nothing recent the submenu is %v, want one disabled None", diskLabels(spec))
	}
	// a tape or a snapshot is not a disk
	b.noteRecentFile(zx, writeTestFile(t, t.TempDir(), "only.tap"))
	b.diskRecentAt = time.Time{}
	if spec := b.recentDisksSpec(); len(spec) != 1 || spec[0].label != "None" {
		t.Errorf("a tape made Recent Disks %v", diskLabels(spec))
	}
}

func TestRecentDisksDropsADiskThatIsGone(t *testing.T) {
	b, zx := diskBar(t)
	dir := t.TempDir()
	keep := writeTestFile(t, dir, "keep.dsk")
	gone := writeTestFile(t, dir, "gone.dsk")
	b.noteRecentFile(zx, keep)
	b.noteRecentFile(zx, gone)
	if got := diskLabels(b.recentDisksSpec()); len(got) != 2 {
		t.Fatalf("before: %v", got)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	b.diskRecentAt = time.Time{} // the menu looks again every two seconds
	got := diskLabels(b.recentDisksSpec())
	if len(got) != 1 || !strings.Contains(got[0], "keep.dsk") {
		t.Errorf("after the file went: %v, want keep.dsk alone", got)
	}
}

func TestRecentDisksShowsTheFirstFewAndLeadsToManageDisks(t *testing.T) {
	b, zx := diskBar(t)
	dir := t.TempDir()
	for i := 0; i < recentDisksMenu+3; i++ {
		b.noteRecentFile(zx, writeTestFile(t, dir, fmt.Sprintf("d%02d.dsk", i)))
	}
	spec := b.recentDisksSpec()
	got := diskLabels(spec)
	if len(got) != recentDisksMenu+2 || got[recentDisksMenu] != "-" {
		t.Fatalf("%d rows: %v", len(got), got)
	}
	if want := "3 more: Manage Disks..."; got[len(got)-1] != want {
		t.Errorf("the last row is %q, want %q", got[len(got)-1], want)
	}
	if b.disks != nil {
		t.Fatal("the Manage Disks window is open already")
	}
	if !spec.pick(zx, len(spec)-1, -1) {
		t.Fatal("the last row ran nothing")
	}
	if b.disks == nil {
		t.Error("the last row did not open Manage Disks")
	}
}

func TestPickingARecentDiskLoadsItAndKeepsItFirst(t *testing.T) {
	path := tempDSK(t) // before the working directory moves: it copies a file from testdata/
	other := tempDSK(t)
	b, zx := diskBar(t)
	b.noteRecentFile(zx, path)
	b.noteRecentFile(zx, other) // newest
	var said []string
	zx.OnNotice = func(msg string) { said = append(said, msg) }

	spec := b.recentDisksSpec()
	if len(spec) != 2 {
		t.Fatalf("Recent Disks is %v", diskLabels(spec))
	}
	if !spec.pick(zx, 1, -1) { // the older one
		t.Fatal("the row ran nothing")
	}
	if zx.io.fdc.DiskFile() != path {
		t.Errorf("the drive has %q, want %q", zx.io.fdc.DiskFile(), path)
	}
	if len(said) != 1 || !strings.HasPrefix(said[0], "The disk disk.dsk is in the drive") {
		t.Errorf("said %q", said)
	}
	if len(b.recentFiles) == 0 || b.recentFiles[0] != path {
		t.Errorf("the disk loaded last is not first in the recent files: %v", b.recentFiles)
	}
	if got := b.diskFolder; got != filepath.Dir(path) {
		t.Errorf("the Load Disk folder is %q, want where the disk came from, %q", got, filepath.Dir(path))
	}
}

func TestPickingARecentDiskThatCannotBeReadSaysSo(t *testing.T) {
	b, zx := diskBar(t)
	bad := writeTestFile(t, t.TempDir(), "bad.dsk") // "data": not a disk
	b.noteRecentFile(zx, bad)
	var said []string
	zx.OnNotice = func(msg string) { said = append(said, msg) }
	if !b.recentDisksSpec().pick(zx, 0, -1) {
		t.Fatal("the row ran nothing")
	}
	if len(said) != 1 || !strings.HasPrefix(said[0], "The disk bad.dsk could not be loaded") {
		t.Errorf("said %q", said)
	}
}

func TestFloppyDiskMenuCarriesRecentDisksAndManageDisksBeforeTheRest(t *testing.T) {
	b, _ := diskBar(t)
	items := b.widget.ItemsFor(barFloppyDisk)
	var order []string
	for _, it := range items[:5] {
		order = append(order, it.Label)
	}
	want := []string{"Floppy Disk", "Load Disk...", "Recent Disks", "Manage Disks...", ""}
	if strings.Join(order, "|") != strings.Join(want, "|") || !items[4].Separator {
		t.Errorf("the Floppy Disk menu starts %v, want %v and then a separator", order, want[:4])
	}
}
