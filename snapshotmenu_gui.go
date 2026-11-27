//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
)

// The Snapshot menu, and Load Timestamped in it. Load Timestamped is a submenu of the newest
// snapshots that Save Timestamped made, ten of them, and a "More..." row for the rest. zenui has
// one level of submenu, so what More... opens is a menu of its own, which opens where the row was
// chosen and has the groups (snapshotgroups.go) as its submenus. The arithmetic of what is listed
// is snapshotgroups.go; the folder is snapshotdir.go; the window that manages them is
// snapshots_gui.go.

// snapshotSpec is the Snapshot menu as the snapshots folder stands at now. Loading a snapshot
// makes the machine it is of (snapshotopen_gui.go), so what loads one is there to be chosen with no
// machine running: Quick Load, Load Timestamped and Manage Snapshots. The rest acts on a machine,
// and is greyed with none.
func (b *appMenuBar) snapshotSpec(now time.Time) menuSpec {
	plan := planSnapshotMenu(timestampedSnapshots(listSnapshots(snapshotDir)), now)
	spec := menuSpec{
		menuTitle("Load"),
		// Load Snapshot and Recent Snapshots: any snapshot, and the named ones lately saved or loaded
		menuRow("Load Snapshot...", func(zx *ZenZX) { b.openLoadSnapshotDialog() }),
		{label: "Recent Snapshots", sub: b.recentSnapshotsSpec()},
		// Manage Snapshots: the window of snapshots_gui.go
		menuRow("Manage Snapshots...", func(zx *ZenZX) { b.openSnapshots(zx) }),
		menuSep(),
		menuTitle("Save and Load"),
		machineRow("Quick Save", func(zx *ZenZX) { zx.handleQuickSave() }),
		menuRow("Quick Load", func(zx *ZenZX) { zx.handleQuickLoad() }),
		machineRow("Save Timestamped", func(zx *ZenZX) { zx.handleAutoSave() }),
		{label: "Load Timestamped", sub: b.loadTimestampedSpec(plan)},
		// Save As: the format, then the name (snapshotsaveas_gui.go)
		{label: "Save As", sub: b.saveAsSpec(), needsMachine: true},
		// Multisnapshots: every machine of the window, and how it is laid out (multisnapshot_menu_gui.go)
		{label: "Save Multisnapshot...", run: func(zx *ZenZX) { b.openSaveMultiDialog() }, needsMachine: true, disabled: b.snapMachines < 2},
		menuRow("Load Multisnapshot...", func(zx *ZenZX) { b.openLoadMultiDialog() }),
		// Load Binary: the window of binary_gui.go, on its Load file tab
		machineRow("Load Binary...", func(zx *ZenZX) { b.openBinary(zx, binTabLoad) }),
		menuSep(),
		menuTitle("Information"),
		machineRow("Snapshot Info", func(zx *ZenZX) { b.showInfo("SNAPSHOT INFO", snapshotInfoText(zx)) }),
		machineRow("Run Diagnostics", func(zx *ZenZX) { b.showDiagnostics(zx) }), // diagnostics_gui.go
	}
	if b.snapStopped {
		for i := range spec {
			spec[i].disabled = spec[i].disabled || spec[i].needsMachine
		}
	}
	return spec
}

// recentSnapshotsSpec is the submenu of Recent Snapshots: the named snapshots that are still there,
// newest first, which load as any snapshot does (a machine is made for one that needs it).
func (b *appMenuBar) recentSnapshotsSpec() menuSpec {
	paths := existingSnapshots(b.recentSnaps)
	if len(paths) == 0 {
		return menuSpec{{label: "None", disabled: true}}
	}
	labels := recentLabels(paths)
	spec := make(menuSpec, len(paths))
	for i, p := range paths {
		path := p
		spec[i] = menuRow(labels[i], func(zx *ZenZX) {
			if b.loadSnapshotAt(zx, path) {
				b.noteRecentSnapshot(zx, path)
			}
		})
	}
	return spec
}

// noteRecentSnapshot puts the snapshot at path first in the recent ones (a timestamped snapshot or
// the quick save is not kept: they have menus of their own) and keeps the list in the settings.
func (b *appMenuBar) noteRecentSnapshot(zx *ZenZX, path string) {
	list := addRecentSnapshot(b.recentSnaps, path)
	if strings.Join(list, "\x00") == strings.Join(b.recentSnaps, "\x00") {
		return
	}
	b.recentSnaps = list
	b.saveSettings(zx)
}

// openLoadSnapshotDialog opens the file browser for Load Snapshot, in the snapshots folder, showing
// the files of every format a snapshot is loaded from. What is chosen is loaded as a file dropped
// on the window is.
func (b *appMenuBar) openLoadSnapshotDialog() {
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:     zenui.ModeOpen,
		Title:    "Load Snapshot",
		StartDir: snapshotStartDir(),
		Filters:  snapshotLoadExts,
		FS:       zenui.OSFS{},
	})
	b.dialogKind = dialogLoadSnapshot
}

// snapshotRow is the row that loads one snapshot.
func (b *appMenuBar) snapshotRow(f snapshotFile, withDate bool) menuEntry {
	path := f.Path
	return menuRow(snapshotRowLabel(f, withDate), func(zx *ZenZX) { b.loadSnapshotAt(zx, path) })
}

// loadSnapshotAt loads the snapshot at path into the machine, as a file dropped on the window is:
// the machine waits paused, or plays, as Preferences says a loaded snapshot does.
func (b *appMenuBar) loadSnapshotAt(zx *ZenZX, path string) (restored bool) {
	if isMultiSnapshotPath(path) {
		b.loadMultiSnapshotAt(zx, path) // asks first, and keeps the recent ones when it is loaded
		return false
	}
	msg := zx.loadDroppedFile(path)
	if msg != "" {
		zx.notice("%s", msg)
	}
	return strings.HasSuffix(msg, "has been restored") // what loadDroppedFile says of a snapshot that went in
}

// loadTimestampedSpec is the submenu of Load Timestamped: the newest snapshots, and More... for
// the rest.
func (b *appMenuBar) loadTimestampedSpec(plan snapshotPlan) menuSpec {
	if len(plan.Head) == 0 {
		return menuSpec{{label: "None", disabled: true}}
	}
	spec := make(menuSpec, 0, len(plan.Head)+2)
	for _, f := range plan.Head {
		spec = append(spec, b.snapshotRow(f, true))
	}
	if len(plan.More) > 0 {
		groups := plan.More
		spec = append(spec, menuSep(), menuRow("More...", func(zx *ZenZX) { b.openMoreSnapshots(groups) }))
	}
	return spec
}

// moreSnapshotsSpec is the menu that More... opens: the older snapshots as they are when there
// are not many, otherwise one row for each group, with the group's snapshots as its submenu.
func (b *appMenuBar) moreSnapshotsSpec(groups []snapshotGroup) menuSpec {
	var spec menuSpec
	for _, g := range groups {
		if g.Kind == groupFlat {
			for _, f := range g.Items {
				spec = append(spec, b.snapshotRow(f, true))
			}
			continue
		}
		shown, left := shownInGroup(g)
		withDate := g.Kind == groupWeek || g.Kind == groupMonth
		sub := make(menuSpec, 0, len(shown)+2)
		for _, f := range shown {
			sub = append(sub, b.snapshotRow(f, withDate))
		}
		if left > 0 {
			sub = append(sub, menuSep(),
				menuRow(fmt.Sprintf("%d more: Manage Snapshots...", left), func(zx *ZenZX) { b.openSnapshots(zx) }))
		}
		spec = append(spec, menuEntry{label: groupMenuLabel(g), sub: sub})
	}
	return spec
}

// openMoreSnapshots opens More...'s menu where the pointer is, which is on the row that was just
// chosen. It is the logo menu's own surface (a menu of the bar's that is not one of its labels'):
// one menu at a time, closed by Escape or a click outside it.
func (b *appMenuBar) openMoreSnapshots(groups []snapshotGroup) {
	spec := b.moreSnapshotsSpec(groups)
	b.closeMenus()
	b.logoSpec = spec
	b.logoMenu = zenui.NewMenu(zenui.MenuConfig{
		Items:  spec.items(),
		Anchor: zenui.Rect{X: int(rl.GetMouseX()), Y: int(rl.GetMouseY()) - 4},
		Scale:  b.scale,
	})
}

// snapshotStamp is what the Snapshot menu depends on: the snapshots folder, and the day.
func snapshotStamp(now time.Time) string {
	return snapshotSignature(snapshotDir) + now.Format("|20060102")
}

// snapshotSig is what the Snapshot menu depends on beyond that: whether a machine is running, for
// the rows that need one.
func (b *appMenuBar) snapshotSig(now time.Time) string {
	sig := snapshotStamp(now) + "|" + fmt.Sprint(b.snapMachines) + "|" + strings.Join(existingSnapshots(b.recentSnaps), "\x00") // the recent ones that are there, which may be anywhere
	if b.snapStopped {
		return sig + "|none"
	}
	return sig
}

// refreshSnapshotItemsFor is refreshSnapshotItems for the machine the bar acts on: with none
// running (the deck is empty), the rows that need one are greyed.
func (b *appMenuBar) refreshSnapshotItemsFor(zx *ZenZX) {
	b.snapStopped = zx != nil && zx.stopped
	b.snapMachines = b.machinesRunning()
	b.refreshSnapshotItems()
}

// refreshSnapshotItems makes the Snapshot menu right for the snapshots folder as it is, each
// frame the bar is shown: the folder is read again only when it has changed (a save, a copy, a
// rename or a delete changes its time) or the day has (the groups count back from today), so a
// frame costs one stat.
func (b *appMenuBar) refreshSnapshotItems() {
	now := time.Now()
	if sig := b.snapshotSig(now); sig != b.snapSig {
		b.snapSig = sig
		b.snapSpec = b.snapshotSpec(now)
	} else {
		return
	}
	b.actions[barSnapshot] = b.snapSpec.actions()
	b.widget.SetItems(barSnapshot, b.snapSpec.items())
}
