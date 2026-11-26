//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Saving and loading the window's machines together (multisnapshot.go is the file).
//
// Saving pauses every machine, so that they are taken at one moment, takes a snapshot of each, and
// puts them in the file with the layout the window has and the machine that is in front; the
// machines are then as they were, running or paused. Loading is all or nothing: every snapshot is
// read and the machines of them are made and loaded before the machines that were running are
// closed, so that a multisnapshot that cannot be loaded leaves the window as it was.

// saveMulti saves every machine of the deck to a multisnapshot at path.
func (d *machineDeck) saveMulti(path string) error {
	n := len(d.machines)
	if n < 2 {
		return errors.New("a multisnapshot is of two machines or more")
	}
	was := make([]bool, n)
	for i, m := range d.machines {
		was[i], m.zx.paused = m.zx.paused, true
	}
	defer func() {
		for i, m := range d.machines {
			if i < n {
				m.zx.paused = was[i]
			}
		}
	}()
	tmp, err := os.MkdirTemp("", "zenzx-zms-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	man := multiManifest{Layout: deckLayoutNames[d.tiling], Front: d.front, Zoom: d.dm.screen.multiplier}
	snaps := make([][]byte, n)
	for i, m := range d.machines {
		file := filepath.Join(tmp, multiFileName(i))
		if err := m.zx.SaveSnapshotFormat(file, "zxs"); err != nil {
			return fmt.Errorf("machine %d (%s): %w", i+1, modelLabel(m.model), err)
		}
		if snaps[i], err = os.ReadFile(file); err != nil {
			return err
		}
		man.Machines = append(man.Machines, multiMachineEntry{Model: m.model, Name: m.name, Renamed: m.renamed})
	}
	return writeMultiSnapshot(path, man, snaps)
}

// multiModel is the model machine e is made as for the snapshot that is its own: the model it was
// when that is of the snapshot's kind (a +2 stays a +2), the snapshot's own otherwise.
func multiModel(e multiMachineEntry, want snapshotMachine) string {
	if e.Model != "" && classOfModel(e.Model) == want.Class {
		return e.Model
	}
	return want.Model
}

// loadMulti replaces the deck's machines with those of the multisnapshot at path, and shows them as
// it was saved. It returns how many machines it loaded. A Next that had cards is given them from
// where they are found, and the load is refused when one is not (loadMultiWith, the window asking
// for them first).
func (d *machineDeck) loadMulti(path string) (int, error) { return d.loadMultiWith(path, nil) }

// loadMultiWith is loadMulti with the cards of each Next given: plan has the slots of the machines
// that had cards, by their place in the multisnapshot. With no plan they are looked for.
func (d *machineDeck) loadMultiWith(path string, plan cardPlan) (int, error) {
	man, snaps, err := readMultiSnapshot(path)
	if err != nil {
		return 0, err
	}
	n := len(snaps)
	if n > 1 && !d.multiAllowed() {
		return 0, fmt.Errorf("it has %d machines, and Preferences have the window at one: turn Multi on", n)
	}
	tmp, err := os.MkdirTemp("", "zenzx-zms-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmp)
	files := make([]string, n)
	models := make([]string, n)
	for i, e := range man.Machines {
		files[i] = filepath.Join(tmp, multiFileName(i))
		if err := os.WriteFile(files[i], snaps[i], 0o600); err != nil {
			return 0, err
		}
		want, err := readSnapshotMachine(files[i], "zxs")
		if err != nil {
			return 0, fmt.Errorf("machine %d: %w", i+1, snapshotErrorFor(err))
		}
		models[i] = multiModel(e, want)
	}
	if plan == nil {
		needs, _, err := d.needsOfMulti(path, man, snaps)
		if err != nil {
			return 0, err
		}
		p, ok := planOf(needs)
		if !ok {
			for _, n := range needs {
				if !allFound([]cardNeed{n}) {
					return 0, fmt.Errorf("%s: %w", n.label, &cardsNotFound{found: n.found})
				}
			}
		}
		plan = p
	}

	// the cards are looked at again, before a machine is made or closed: the person may have answered
	// some time ago. Every machine that is running is going.
	if m, err := d.checkCardPlan(plan, func(*ZenZX) bool { return true }); err != nil {
		return 0, fmt.Errorf("machine %d: %w", m+1, err)
	}

	old := append([]*deckMachine(nil), d.machines...)
	prevFront := d.front
	var made []*deckMachine
	undo := func() {
		if len(old) > 0 {
			d.flip(prevFront)
			for _, m := range made {
				d.remove(d.indexOf(m))
			}
			return
		}
		for len(made) > 1 {
			d.remove(d.indexOf(made[len(made)-1]))
			made = made[:len(made)-1]
		}
		if len(d.machines) == 1 {
			d.makeIdle()
		}
	}
	for i := range man.Machines {
		at := d.add(models[i])
		m := d.machines[at]
		made = append(made, m)
		d.flip(at)
		if s, ok := plan[i]; ok {
			if err := d.giveCards(m.zx, s); err != nil {
				undo()
				return 0, fmt.Errorf("machine %d: %w", i+1, err)
			}
		}
		err := m.zx.loadSnapshotHere(files[i], "zxs", false)
		m.zx.cardsApproved = false
		if err != nil {
			undo()
			return 0, fmt.Errorf("machine %d: %w", i+1, snapshotErrorFor(err))
		}
		m.fromSnapshot = ""
		if e := man.Machines[i]; e.Name != "" {
			m.name, m.renamed = e.Name, e.Renamed
		}
	}
	for _, m := range old {
		if i := d.indexOf(m); i >= 0 {
			d.remove(i)
		}
	}
	d.flip(man.Front)
	d.sel = d.front

	// the way the window showed them: one machine's zoom first, then the layout
	d.tiling = layoutSingle
	d.applyLayout()
	if man.Zoom > 0 {
		d.dm.SetScale(man.Zoom)
	}
	if l, ok := layoutByName(man.Layout); ok {
		d.setLayout(l)
	}
	return n, nil
}

// indexOf is machine m's place in the deck, -1 when it is not in it.
func (d *machineDeck) indexOf(m *deckMachine) int {
	for i, x := range d.machines {
		if x == m {
			return i
		}
	}
	return -1
}

// snapshotErrorFor is the error as the person is told it: a snapshot of another build, or of a
// machine that cannot be had, says so in its own words.
func snapshotErrorFor(err error) error {
	if errors.Is(err, errNextSnapshotOtherBuild) {
		return errors.New("it was saved by another build of ZenZX and cannot be loaded")
	}
	return err
}
