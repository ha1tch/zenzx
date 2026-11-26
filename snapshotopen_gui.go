//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"
)

// Opening a snapshot in the window. A snapshot is of a kind of machine (snapshotmodel.go), and it is
// loaded into a machine of that kind: the window makes one, as the New tile of the dock does, and
// loads the snapshot into it. With no machine running that machine is the one that fills the window.
// What is not made new is the machine that is the snapshot's already: the one a snapshot was loaded
// into, loaded again from the same file (Quick Load, the same row of Load Timestamped), is not
// followed by another of the same, and a window that may not add machines (Preferences) puts the
// snapshot into the front machine, which becomes the model it needs when it is not of that kind.
// A machine that is made is shown in the dock, which slides in for it and goes again (dockreveal_gui.go).

// snapshotKey names a snapshot file for the deck to know it by.
func snapshotKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

// openSnapshot loads the snapshot at path (of format, "" to find it out) as that paragraph says, and
// brings the machine it loaded into to the front.
func (d *machineDeck) openSnapshot(path, format string, wait bool) error {
	return d.openSnapshotWith(path, format, wait, nil)
}

// snapshotTarget is the machine a snapshot of the kind want is loaded into when it is loaded again
// from the file it came from, or when none may be made: the front one. nil when it is a machine to
// be made. It is what the choice below comes to, which is made before the cards are looked for, since
// the cards of the machine that is the target are free to it.
func (d *machineDeck) snapshotTarget(want snapshotMachine, key string) *ZenZX {
	if d.Empty() {
		return nil
	}
	front := d.machines[d.front]
	switch {
	case front.zx.machineClass() == want.Class && front.fromSnapshot == key:
		return front.zx
	case d.multiAllowed() && len(d.machines) < multiMax:
		return nil
	}
	return front.zx
}

// openSnapshotWith is openSnapshot, with the cards the snapshot is to be loaded over when the person
// has been asked for them (nil: they are looked for).
func (d *machineDeck) openSnapshotWith(path, format string, wait bool, given *cardSlots) error {
	if format == "" {
		format = DetectSnapshotFormat(path)
	}
	want, err := readSnapshotMachine(path, format)
	if err != nil {
		return err
	}
	key := snapshotKey(path)

	// A Next that had cards is loaded over them: they are looked for, and the person is asked for the
	// ones that are not found, before anything is changed.
	var slots cardSlots
	cards, err := readSnapshotCards(path, format)
	if err != nil {
		return err
	}
	if len(cards) > 0 {
		if given != nil {
			slots = *given
		} else {
			var asked bool
			if slots, asked, err = d.cardsForSnapshot(path, format, wait, cards, d.snapshotTarget(want, key)); asked || err != nil {
				if asked {
					return errCardsAsked
				}
				return err
			}
		}
	}

	if len(cards) > 0 {
		// the cards are looked at again: the person may have answered some time ago
		target := d.snapshotTarget(want, key)
		if _, err := d.checkCardPlan(cardPlan{0: slots}, func(zx *ZenZX) bool { return zx == target }); err != nil {
			return err
		}
	}

	var m, made *deckMachine // made is the machine that was made for the snapshot, when one was
	var undo func()
	switch {
	case d.Empty():
		d.revive(want.Model)
		m = d.machines[0]
		made = m
		undo = d.makeIdle
	default:
		front := d.machines[d.front]
		fits := front.zx.machineClass() == want.Class
		switch {
		case fits && front.fromSnapshot == key:
			m = front // the same file again: the machine it was loaded into
		case d.multiAllowed() && len(d.machines) < multiMax:
			prev := d.front
			i := d.add(want.Model)
			d.flip(i)
			m = d.machines[i]
			made = m
			undo = func() {
				d.flip(prev)
				d.remove(i)
			}
		case fits:
			m = front
		case d.retype != nil:
			// no machine may be added: the one there is becomes the model the snapshot is of
			d.retype(front.zx, want.Model)
			front.model = want.Model
			m = front
		default:
			return &snapshotMismatch{Snapshot: want.Class, Machine: front.zx.machineClass()}
		}
	}

	if len(cards) > 0 {
		if err := d.giveCards(m.zx, slots); err != nil {
			if undo != nil {
				undo()
			}
			return err
		}
	}
	err = m.zx.loadSnapshotHere(path, format, wait)
	m.zx.cardsApproved = false
	if err != nil {
		if undo != nil {
			undo() // the machine that was made for it is not left behind
		}
		return err
	}
	m.fromSnapshot = key
	if made != nil {
		d.revealNew(made) // the dock slides in to show it
	}
	return nil
}
