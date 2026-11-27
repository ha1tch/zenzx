//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"strings"
	"time"

	"github.com/ha1tch/zenui"
)

// The Load rows of the Tape menu: Load Tape..., and Recent Tapes, which lists the tapes lately loaded
// the way Recent Snapshots lists snapshots. A tape that is chosen from either, or from Manage Tapes
// (tapes_gui.go), is loaded as one dropped on the window is: the deck takes it, and Autoload tape
// says whether the machine starts it. tapeInsertedHook (tapedir.go) keeps the tapes loaded, whichever
// way they came.

// tapeRecentLook is how long what the Recent Tapes menu knows about which tapes are still there is
// good for: the menu is made again every frame the bar is shown, and a stat for each tape each
// frame is not needed.
const tapeRecentLook = 2 * time.Second

// recentTapesNow is the tapes lately loaded that are still there, newest first.
func (b *appMenuBar) recentTapesNow() []string {
	now := time.Now()
	key := strings.Join(b.recentTapes, "\x00")
	if key != b.tapeRecentKey || now.Sub(b.tapeRecentAt) >= tapeRecentLook || b.tapeRecentAt.IsZero() {
		b.tapeRecentKey, b.tapeRecentAt, b.tapeRecentOK = key, now, existingFiles(b.recentTapes)
	}
	return b.tapeRecentOK
}

// recentTapesSpec is the submenu of Recent Tapes: the first tapes loaded that are still there, newest
// first, and a row that opens Manage Tapes for the rest.
func (b *appMenuBar) recentTapesSpec() menuSpec {
	paths := b.recentTapesNow()
	if len(paths) == 0 {
		return menuSpec{{label: "None", disabled: true}}
	}
	shown := paths
	if len(shown) > recentTapesMenu {
		shown = shown[:recentTapesMenu]
	}
	labels := recentLabels(shown)
	spec := make(menuSpec, 0, len(shown)+2)
	for i, p := range shown {
		path := p
		spec = append(spec, menuRow(labels[i], func(zx *ZenZX) { b.loadTapeAt(path) }))
	}
	if left := len(paths) - len(shown); left > 0 {
		spec = append(spec, menuSep(), menuRow(fmt.Sprintf("%d more: Manage Tapes...", left), func(zx *ZenZX) { b.openTapes(zx) }))
	}
	return spec
}

// loadTapeAt loads the tape at path as a file dropped on the window is loaded.
func (b *appMenuBar) loadTapeAt(path string) {
	if b.openFiles != nil {
		b.openFiles([]string{path})
	}
}

// noteRecentTape puts the tape at path first in the recent ones, and keeps the list in the settings.
func (b *appMenuBar) noteRecentTape(zx *ZenZX, path string) {
	list := addRecentTape(b.recentTapes, path)
	if strings.Join(list, "\x00") == strings.Join(b.recentTapes, "\x00") {
		return
	}
	b.recentTapes = list
	b.saveSettings(zx)
}

// renamedTape keeps a tape that Manage Tapes has renamed among the recent ones under its new name.
func (b *appMenuBar) renamedTape(zx *ZenZX, old, target string) {
	b.recentTapes = renameRecentTape(b.recentTapes, old, target)
	b.saveSettings(zx)
}

// openLoadTapeDialog opens the file browser for Load Tape: it starts where the last tape came from,
// or in the tapes folder, lists the recent tapes as its Recent place, and offers tapes only.
func (b *appMenuBar) openLoadTapeDialog() {
	recent := b.recentTapesNow()
	cfg := b.openDialogConfig("Load Tape", "")
	cfg.StartDir = tapeStartDir(recent)
	cfg.Recent = recent
	cfg.Filters = tapeExts
	b.diskDialog = zenui.NewDialog(cfg)
	b.dialogKind, b.fileDlg = dialogLoadTape, fileDlgTape
}
