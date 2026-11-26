//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// The cards of a snapshot, in the window (cardrecords.go, cardsearch.go, cardsask_gui.go): what the
// deck does before a snapshot of a Next that had cards is loaded, which is to find the cards, or
// to ask the person for them, and after the machine is chosen, which is to give it them.

// cardSearchFor makes the search for the cards of a snapshot at path: through the folders card
// images are kept in, the folders of the cards the machines have, and the snapshot's own. A card that
// a machine has is not free, but for the machines that are going to go (their cards are free).
func (d *machineDeck) cardSearchFor(path string, goes func(zx *ZenZX) bool) *cardSearch {
	var dirs []string
	if d.cardDirs != nil {
		dirs = append(dirs, d.cardDirs()...)
	}
	for _, m := range d.machines {
		for _, c := range m.zx.mountedCards() {
			dirs = append(dirs, filepath.Dir(c.path))
		}
	}
	dirs = append(dirs, filepath.Dir(path))
	return &cardSearch{dirs: dirs, held: func(p string) int {
		u := d.cardUse(p)
		if u.machine == 0 || (goes != nil && goes(d.machineAt(u.machine))) {
			return 0
		}
		return u.machine
	}}
}

// cardsForSnapshot finds the cards of the snapshot at path for the machine target (nil for a machine
// that is made). When they are all found they are the answer; when one is not, the person is asked
// for it, and the snapshot is loaded when they have answered (asked says so), and with no way to
// ask it is refused.
func (d *machineDeck) cardsForSnapshot(path, format string, wait bool, recs []cardRecord, target *ZenZX) (slots cardSlots, asked bool, err error) {
	s := d.cardSearchFor(path, func(zx *ZenZX) bool { return zx == target })
	found := s.find(0, recs)
	if allFound([]cardNeed{{found: found}}) {
		slots, _ = slotsOf(found, false)
		return slots, false, nil
	}
	if d.askCards == nil {
		return slots, false, &cardsNotFound{found: found}
	}
	base := filepath.Base(path)
	a := newCardsAsk(base, false, []cardNeed{{recs: recs, found: found}}, s, func(plan cardPlan) {
		p := plan[0]
		err := d.openSnapshotWith(path, format, wait, &p)
		front := d.Front()
		if front == nil {
			return
		}
		if err != nil {
			front.notice("%s", snapshotRefusal(base, err))
			return
		}
		front.notice("The snapshot %s has been restored", base)
		if d.opened != nil {
			d.opened(path)
		}
	}, nil)
	d.askCards(a)
	return slots, true, nil
}

// checkCardPlan is the last look at the cards the person has answered with, before anything is
// changed: the world has moved since they were chosen. Each file has to be there still, and to be an
// ordinary file; no file is to be the card of two slots or of two machines of the plan; and none is to
// be in a machine that is staying (goes says which are not: the machine a snapshot is loaded into has
// its own cards free to it). It says which machine of the plan it is that is refused.
func (d *machineDeck) checkCardPlan(plan cardPlan, goes func(*ZenZX) bool) (machine int, err error) {
	var order []int
	for m := range plan {
		order = append(order, m)
	}
	sort.Ints(order)
	givenTo := map[string]int{}
	for _, m := range order {
		s := plan[m]
		for _, path := range []string{s.one, s.two} {
			if path == "" {
				continue
			}
			name := filepath.Base(path)
			st, err := os.Stat(path)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				return m, cardsUnusablef("%s is not there any more", name)
			case err != nil:
				return m, cardsUnusablef("%s cannot be used: %s", name, cardErrorText(err))
			case !st.Mode().IsRegular():
				return m, cardsUnusablef("%s is not a card image", name)
			}
			key := canonical(path)
			if to, ok := givenTo[key]; ok {
				if to == m {
					return m, cardsUnusablef("%s is given for both slots, and a card is in one slot at a time", name)
				}
				return m, cardsUnusablef("%s is given to two machines, and a card is in one machine at a time", name)
			}
			givenTo[key] = m
			if u := d.cardUse(path); u.machine != 0 && (goes == nil || !goes(d.machineAt(u.machine))) {
				return m, cardsUnusablef("%s is in machine %d now, and a card is in one machine at a time", name, u.machine)
			}
		}
	}
	return 0, nil
}

// giveCards puts the cards in the slots of zx, a Next, and tells the loader whether it is to take
// them as they are.
func (d *machineDeck) giveCards(zx *ZenZX, s cardSlots) error {
	if d.mountCards == nil {
		return errors.New("this window cannot give a Next its cards")
	}
	if err := d.mountCards(zx, s); err != nil {
		return err
	}
	zx.cardsApproved = s.approved
	return nil
}

// mountCards is what the menu bar does to give a Next the cards: it attaches the boot chain over the card
// in the first slot, and the second card in the second, from the Next's boot files. A Next that has the cards
// already is left as it is.
func (b *appMenuBar) mountCards(zx *ZenZX, c cardSlots) error {
	if !zx.memory.isNext {
		return errors.New("only a Next has cards")
	}
	if c.one == "" {
		return errors.New("there is no card for the first slot")
	}
	have := map[int]string{}
	for _, m := range zx.mountedCards() {
		have[m.slot] = m.path
	}
	if sameCardPath(have[1], c.one) && sameCardPath(have[2], c.two) {
		return nil
	}
	f, ok, note := locateNextFiles(b.nextFilesDir)
	if !ok {
		msg := "the Next's boot files are not available, and a Next that boots NextZXOS cannot be made without them"
		if note != "" {
			msg += " (" + note + ")"
		}
		return errors.New(msg)
	}
	f.Card = c.one
	if _, err := zx.attachNextBootChain(f, b.nextReadOnly, b.nextDebug); err != nil {
		return err
	}
	if c.two != "" {
		if _, err := zx.InsertSecondCard(c.two, b.nextReadOnly); err != nil {
			return fmt.Errorf("the second card %s could not be used: %w", filepath.Base(c.two), err)
		}
	}
	return nil
}

// sameCardPath is whether two paths are the same file; two empty ones are the same.
func sameCardPath(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	return err == nil && os.SameFile(sa, sb)
}

// cardDirsOf lists the folders the Next's card images are kept in: the folders the boot files are
// looked for in.
func (b *appMenuBar) cardDirsOf() []string {
	var dirs []string
	for _, d := range nextFilesDirs(b.nextFilesDir) {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// needsOfMulti finds the cards of the machines of a multisnapshot that had cards: man and snaps are
// what readMultiSnapshot made of the file at path. Every machine that is running is going, and its
// cards are free for the ones that come.
func (d *machineDeck) needsOfMulti(path string, man multiManifest, snaps [][]byte) ([]cardNeed, *cardSearch, error) {
	s := d.cardSearchFor(path, func(*ZenZX) bool { return true })
	var needs []cardNeed
	for i := range snaps {
		recs, err := readCardRecords(bytes.NewReader(snaps[i]))
		if err != nil {
			return nil, nil, fmt.Errorf("machine %d: %w", i+1, err)
		}
		if len(recs) == 0 {
			continue
		}
		needs = append(needs, cardNeed{
			machine: i,
			label:   fmt.Sprintf("Machine %d (%s)", i+1, modelLabel(man.Machines[i].Model)),
			recs:    recs,
			found:   s.find(i, recs),
		})
	}
	return needs, s, nil
}

// multiCardNeeds is needsOfMulti for the multisnapshot at path, which it reads.
func (d *machineDeck) multiCardNeeds(path string) ([]cardNeed, *cardSearch, error) {
	man, snaps, err := readMultiSnapshot(path)
	if err != nil {
		return nil, nil, err
	}
	return d.needsOfMulti(path, man, snaps)
}

// planOf is the cards of every machine that needs them, when every one is found.
func planOf(needs []cardNeed) (cardPlan, bool) {
	if !allFound(needs) {
		return nil, false
	}
	plan := cardPlan{}
	for _, n := range needs {
		s, _ := slotsOf(n.found, false)
		plan[n.machine] = s
	}
	return plan, true
}
