//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"
	"strings"
)

// Snapshot > Save As: the person chooses the format and then names the file. This is the
// formats that menu offers and what is done with the name; the menu and its file browser are
// snapshotsaveas_gui.go.
//
// The three are the formats ZenZX writes (SaveSnapshotFormat). .szx and .rzx are read and not
// written, so they are not offered.
var snapshotSaveFormats = []struct{ format, label string }{
	{"zxs", "ZenZX snapshot (.zxs)"},
	{"sna", "SNA (.sna)"},
	{"z80", "Z80 (.z80)"},
}

// saveSnapshotAs saves the machine to path in format and says what happened: the result is a
// toast (and a line on the terminal), as the other items of the Snapshot menu say theirs. A name
// without the format's extension gets it, as the writers add it. The result is the path that was
// saved, or "" when it was not.
func (zx *ZenZX) saveSnapshotAs(path, format string) (saved string) {
	if !strings.HasSuffix(path, "."+format) {
		path += "." + format
	}
	if err := zx.SaveSnapshotFormat(path, format); err != nil {
		zx.notice("The snapshot %s could not be saved: %v", filepath.Base(path), err)
		return ""
	}
	zx.notice("Saved snapshot %s (%s format)", filepath.Base(path), strings.ToUpper(format))
	return path
}
