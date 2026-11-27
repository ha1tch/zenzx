//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"strings"
	"time"
)

// Recent Disks, in the Floppy Disk menu: the disk images lately loaded, from Load Disk..., File > Open
// or a drop, newest first, as Recent Tapes and Recent Snapshots list theirs. They are the recent
// files (recentfiles.go) that are disks, so no list of its own is kept in the settings.

// recentDisksNow is the recent disk images that are still there, newest first. What the menu knows
// about which are still there is good for tapeRecentLook: the menu is made again every frame the bar
// is shown.
func (b *appMenuBar) recentDisksNow() []string {
	now := time.Now()
	key := strings.Join(b.recentFiles, "\x00")
	if key != b.diskRecentKey || now.Sub(b.diskRecentAt) >= tapeRecentLook || b.diskRecentAt.IsZero() {
		b.diskRecentKey, b.diskRecentAt, b.diskRecentOK = key, now, disksOf(existingFiles(b.recentFiles))
	}
	return b.diskRecentOK
}

// recentDisksSpec is the Recent Disks submenu: the first disks, and a row that opens Manage Disks for
// the rest.
func (b *appMenuBar) recentDisksSpec() menuSpec {
	paths := b.recentDisksNow()
	if len(paths) == 0 {
		return menuSpec{{label: "None", disabled: true}}
	}
	shown := paths
	if len(shown) > recentDisksMenu {
		shown = shown[:recentDisksMenu]
	}
	labels := recentLabels(shown)
	spec := make(menuSpec, 0, len(shown)+2)
	for i, p := range shown {
		path := p
		spec = append(spec, menuRow(labels[i], func(zx *ZenZX) { b.loadDisk(zx, path, fileDlgDisk) }))
	}
	if left := len(paths) - len(shown); left > 0 {
		spec = append(spec, menuSep(), menuRow(fmt.Sprintf("%d more: Manage Disks...", left), func(zx *ZenZX) { b.openDisks(zx) }))
	}
	return spec
}
