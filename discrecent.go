// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"
	"strings"
)

// recentDisksMenu is how many Recent Disks rows the Floppy Disk menu shows: the rest are one row
// that opens Manage Disks.
const recentDisksMenu = 15

// isDiskPath says path is a disk image the drive takes.
func isDiskPath(path string) bool { return strings.EqualFold(filepath.Ext(path), ".dsk") }

// disksOf is the disk images among the recent files, in the order the list has them (newest first).
// Recent Disks is the recent files seen through the one medium, as the Open DSK dialog's Recent place is.
func disksOf(list []string) []string {
	var out []string
	for _, p := range list {
		if isDiskPath(p) {
			out = append(out, p)
		}
	}
	return out
}
