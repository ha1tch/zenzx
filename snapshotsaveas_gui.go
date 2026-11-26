//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "github.com/ha1tch/zenui"

// saveAsSpec is the submenu of Snapshot > Save As: one row per format (snapshotSaveFormats),
// each opening the file browser for a name in that format.
func (b *appMenuBar) saveAsSpec() menuSpec {
	spec := make(menuSpec, len(snapshotSaveFormats))
	for i, f := range snapshotSaveFormats {
		format := f.format
		spec[i] = menuRow(f.label, func(zx *ZenZX) { b.openSaveSnapshotDialog(format) })
	}
	return spec
}

// snapshotStartDir is the folder the Save As browser opens in: the snapshots folder, made when it
// is not there yet (the browser needs a folder that exists), and the folder ZenZX runs in when it
// cannot be made.
func snapshotStartDir() string {
	if err := ensureSnapshotDir(); err != nil {
		return "."
	}
	return snapshotDir
}

// openSaveSnapshotDialog opens the file browser to name a snapshot of the machine, in the
// snapshots folder (where the quick saves go), showing the files of that format.
func (b *appMenuBar) openSaveSnapshotDialog(format string) {
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:       zenui.ModeSave,
		Title:      "Save Snapshot as ." + format,
		StartDir:   snapshotStartDir(),
		Filters:    []string{format},
		DefaultExt: format,
		FS:         zenui.OSFS{},
	})
	b.dialogKind, b.saveFormat = dialogSaveSnapshot, format
}
