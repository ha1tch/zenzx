//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// The Snapshot menu's Save Multisnapshot... and Load Multisnapshot..., and the Snapshots window's
// Load for a .zms (multisnapshot.go is the file, multisnapshot_gui.go what the deck does).

// openSaveMultiDialog opens the file browser to name the multisnapshot, in the snapshots folder.
func (b *appMenuBar) openSaveMultiDialog() {
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:       zenui.ModeSave,
		Title:      "Save Multisnapshot as ." + MultiSnapshotExt,
		StartDir:   snapshotStartDir(),
		Filters:    []string{MultiSnapshotExt},
		DefaultExt: MultiSnapshotExt,
		FS:         zenui.OSFS{},
	})
	b.dialogKind = dialogSaveMulti
}

// openLoadMultiDialog opens the file browser to choose a multisnapshot.
func (b *appMenuBar) openLoadMultiDialog() {
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:     zenui.ModeOpen,
		Title:    "Load Multisnapshot",
		StartDir: snapshotStartDir(),
		Filters:  []string{MultiSnapshotExt},
		FS:       zenui.OSFS{},
	})
	b.dialogKind = dialogLoadMulti
}

// machinesRunning is how many machines the window has: none when it shows only its background.
func (b *appMenuBar) machinesRunning() int {
	if b.layoutState == nil {
		return 0
	}
	_, n := b.layoutState()
	return n
}

// saveMultiSnapshotAt saves every machine of the window to the multisnapshot at path.
func (b *appMenuBar) saveMultiSnapshotAt(zx *ZenZX, path string) {
	if !isMultiSnapshotPath(path) {
		path += "." + MultiSnapshotExt
	}
	base := filepath.Base(path)
	if b.saveMulti == nil || b.machinesRunning() < 2 {
		zx.notice("A multisnapshot is of two machines or more")
		return
	}
	n := b.machinesRunning()
	if err := b.saveMulti(path); err != nil {
		sayf("Multisnapshot not saved: %s: %v\n", path, err)
		zx.notice("The multisnapshot %s could not be saved: %v", base, err)
		return
	}
	sayf("Multisnapshot saved: %s (%d machines)\n", path, n)
	zx.notice("The multisnapshot %s has been saved: %d machines, taken together", base, n)
	b.noteRecentSnapshot(zx, path)
}

// loadMultiSnapshotAt loads the multisnapshot at path in place of the machines that are running,
// which asks first: the machines are closed, and what they hold that is not in a snapshot is
// lost. With none running there is nothing to lose, and it loads at once.
func (b *appMenuBar) loadMultiSnapshotAt(zx *ZenZX, path string) {
	if b.loadMulti == nil {
		zx.notice("This window cannot load a multisnapshot")
		return
	}
	man, err := peekMultiSnapshot(path)
	if err != nil {
		zx.notice("The multisnapshot %s could not be loaded: %v", filepath.Base(path), err)
		return
	}
	// The cards of its Nexts come first: what is asked for here is asked before anything is closed.
	var plan cardPlan
	if b.multiCards != nil {
		needs, search, err := b.multiCards(path)
		if err != nil {
			zx.notice("The multisnapshot %s could not be loaded: %v", filepath.Base(path), err)
			return
		}
		if p, ok := planOf(needs); ok {
			plan = p
		} else {
			b.askForCards(newCardsAsk(filepath.Base(path), true, needs, search, func(p cardPlan) {
				b.replaceOrLoadMulti(zx, path, len(man.Machines), p)
			}, nil))
			return
		}
	}
	b.replaceOrLoadMulti(zx, path, len(man.Machines), plan)
}

// replaceOrLoadMulti asks before the machines that are running are closed, and loads when there are
// none, or the question has been answered.
func (b *appMenuBar) replaceOrLoadMulti(zx *ZenZX, path string, incoming int, plan cardPlan) {
	if running := b.machinesRunning(); running > 0 {
		b.closeMenus()
		b.replaceAsk = &cardQuestion{kind: askReplace, path: path, machines: running, incoming: incoming, plan: plan}
		b.replaceAsk.build()
		return
	}
	b.doLoadMulti(zx, path, plan)
}

// doLoadMulti loads the multisnapshot, the question having been answered.
func (b *appMenuBar) doLoadMulti(zx *ZenZX, path string, plan cardPlan) {
	base := filepath.Base(path)
	n, err := b.loadMulti(path, plan)
	if err != nil {
		sayf("Multisnapshot not loaded: %s: %v\n", path, err)
		zx.notice("The multisnapshot %s could not be loaded: %v", base, err)
		return
	}
	sayf("Multisnapshot loaded: %s (%d machines)\n", path, n)
	zx.notice("The multisnapshot %s has been restored: %d machines", base, n)
	b.noteRecentSnapshot(zx, path)
}

// updateReplaceAsk runs the question for a frame. Enter answers it as the right-hand button does,
// Escape and the left-hand button drop it.
func (b *appMenuBar) updateReplaceAsk(zx *ZenZX) {
	in := zenuiraylib.Input()
	status := b.replaceAsk.modal.Update(in)
	enter := false
	for _, k := range in.Keys {
		enter = enter || k == zenui.KeyEnter
	}
	b.answerReplace(zx, status, enter)
}

// answerReplace takes what a frame of the question came to: cancelled, accepted or Enter.
func (b *appMenuBar) answerReplace(zx *ZenZX, status zenui.Status, enter bool) {
	q := b.replaceAsk
	switch {
	case status == zenui.Cancelled:
		b.replaceAsk = nil
		zx.notice("Not loaded: the machines are as they were")
	case status == zenui.Accepted || enter:
		if q.text != replaceWord {
			q.why = "That is not " + replaceWord + ": type it in capitals, exactly."
			q.build()
			return
		}
		b.replaceAsk = nil
		b.doLoadMulti(zx, q.path, q.plan)
	}
}
