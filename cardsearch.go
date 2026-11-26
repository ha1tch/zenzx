// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// Finding the cards of a snapshot (cardrecords.go): the card images in some folders are looked
// through for the ones that are the cards the snapshot was made over, by their identity and not by
// where they are or what they are called. A card is found when it has the signature the record
// has, and the size and layout; a record of a card that could not be given a signature is found by
// the size and layout alone, which a card of a Next usually has in common with its copies.
//
// A card image is in one slot of one machine (sdcards_gui.go): a card that another machine has is
// not found for this one, and the search says whose it is.

// cardFound is what the search came to for one card of a snapshot.
type cardFound struct {
	Rec      cardRecord
	Path     string         // the file that is the card, "" when none is
	Match    nextcard.Match // how far Path is the card
	Chosen   bool           // Path is what the person chose, and is here to be said not to be the card
	Held     int            // the machine that has the card, when that is why Path is ""
	HeldName string         // the name of the file it has
	HeldPath string         // where that file is, which is where the others of its kind are likely to be
	Said     string         // why a file that was looked at is not the card
	Several  []string       // the names of the files that could each be the card, when it is not for the search to say which
	Layout   bool           // Path is a file the person chose that has the card's size and partition layout and another signature: a copy, taken for it
}

// acceptable is whether the file is the card, or as near to it as a card can be known: it has the
// signature, or it has none and nor did the card that the snapshot was made over.
func (f cardFound) acceptable() bool {
	return f.Path != "" && (f.Match == nextcard.Same || (f.Match == nextcard.Alike && f.Rec.Sig == 0) || f.Layout)
}

// why says in words why the card is not settled.
func (f cardFound) why() string {
	switch {
	case f.Held != 0:
		return fmt.Sprintf("%s is in machine %d, and a card is in one machine at a time", f.HeldName, f.Held)
	case len(f.Several) > 0:
		return "more than one card could be it: " + joinNames(f.Several)
	case f.Said != "":
		return f.Said
	}
	return "it is not to be found"
}

// joinNames says names in a line: "a", "a and b", "a, b and c".
func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// cardsNotFound is the refusal of a snapshot whose cards are not to be found where there is no
// one to ask.
type cardsNotFound struct{ found []cardFound }

func (e *cardsNotFound) Error() string {
	for _, f := range e.found {
		if !f.acceptable() {
			return fmt.Sprintf("the %s %s of the snapshot is not found: %s", slotWords(f.Rec.Slot), f.Rec.describe(), f.why())
		}
	}
	return "the cards of the snapshot are not found"
}

// cardSearch is a search for the cards of one or more snapshots.
type cardSearch struct {
	dirs   []string          // the folders whose card images (*.img) are looked through
	held   func(string) int  // the machine that has the card image, and is not going away; 0 for none
	chosen map[string]string // the file the person chose for a card, by needKey
	ident  map[string]identityResult
	used   map[string]bool // the files that are the card of an earlier need of this search
}

type identityResult struct {
	id  nextcard.Identity
	err error
}

// needKey names a card of a snapshot for the person's choices: the machine it is for (0 for a
// snapshot of one machine) and the slot.
func needKey(machine, slot int) string { return fmt.Sprintf("%d/%d", machine, slot) }

func (s *cardSearch) identityOf(path string) (nextcard.Identity, error) {
	if r, ok := s.ident[path]; ok {
		return r.id, r.err
	}
	id, err := nextcard.IdentityOfFile(path)
	if s.ident == nil {
		s.ident = map[string]identityResult{}
	}
	s.ident[path] = identityResult{id, err}
	return id, err
}

// canonical is a name for the file a path is, for telling that two are the same.
func canonical(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return path
}

// candidates lists the card images of the folders, once each, in the order of their names. A folder
// is read as it is (a pattern would take "[" and "*" in its name for what they mean in one), the empty
// name is no folder, and what is not an ordinary file is not looked at: a pipe waits for someone to
// write to it, and a device may not end.
func (s *cardSearch) candidates() []string {
	var out []string
	seen := map[string]bool{}
	for _, dir := range s.dirs {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir) // in the order of the names
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.EqualFold(filepath.Ext(e.Name()), ".img") {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
				continue
			}
			if key := canonical(p); !seen[key] {
				seen[key] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// find looks for the cards recs say, for the machine numbered machine (0 for a snapshot of one
// machine). A file the person chose for a card is looked at as that card, and said not to be it
// when it is not; for the others the folders are searched.
func (s *cardSearch) find(machine int, recs []cardRecord) []cardFound {
	if s.used == nil {
		s.used = map[string]bool{}
	}
	files := s.candidates()
	found := make([]cardFound, 0, len(recs))
	for _, rec := range recs {
		var f cardFound
		if path, ok := s.chosen[needKey(machine, rec.Slot)]; ok {
			f = s.look(rec, path)
			f.Chosen = true
		} else {
			f = s.search(rec, files)
		}
		if f.Path != "" {
			s.used[canonical(f.Path)] = true
		}
		found = append(found, f)
	}
	return found
}

// look is the file the person chose, looked at as the card rec is.
func (s *cardSearch) look(rec cardRecord, path string) cardFound {
	f := cardFound{Rec: rec}
	name := filepath.Base(path)
	if s.used[canonical(path)] {
		f.Said = name + " is the card of another of these machines"
		return f
	}
	if s.held != nil {
		if n := s.held(path); n != 0 {
			f.Held, f.HeldName, f.HeldPath = n, name, path
			return f
		}
	}
	id, err := s.identityOf(path)
	if err != nil {
		f.Said = name + " is not a card image: " + cardErrorText(err)
		return f
	}
	f.Path, f.Match = path, rec.identity().Compare(id)
	if want := rec.identity(); f.Match == nextcard.Different && id.Size == want.Size && id.Table == want.Table {
		// the size and the layout are what the card had and the signature is another's: a copy of it, which
		// the person chose, and which is taken for it without reading what is on it
		f.Layout = true
	}
	if !f.acceptable() {
		f.Said = notTheCard(rec, name, id)
	}
	return f
}

// notTheCard says why a card image called name, with identity id, is not the card rec is.
func notTheCard(rec cardRecord, name string, id nextcard.Identity) string {
	switch {
	case id.Size != rec.Size:
		return fmt.Sprintf("%s is %s, and the card was %s", name, cardSizeWords(id.Size), cardSizeWords(rec.Size))
	case rec.identity().Table != id.Table:
		return name + " is laid out very differently from the card"
	case rec.Sig != 0 && !id.Keyed():
		return name + " has no signature, so it cannot be told from the card"
	}
	return name + " is a very different card"
}

// search looks through files for the card rec is: the one that has its signature, and of those the
// one that has its name. When several files are as likely to be it as each other (cards that could
// each be one that had no signature, or copies of a card made outside the program) and nothing tells
// them apart by name, it is not for the search to pick one by where it came in a folder: it says
// which, and the person chooses. Files of the same name in different folders are not told apart by
// their names, and the first folder looked through is the one that is meant.
func (s *cardSearch) search(rec cardRecord, files []string) cardFound {
	var held *cardFound
	var found []cardFound
	rank := func(f cardFound) int {
		n := 0
		if f.Match == nextcard.Same {
			n += 2
		}
		if strings.EqualFold(filepath.Base(f.Path), rec.Name) {
			n++
		}
		return n
	}
	for _, p := range files {
		if s.used[canonical(p)] {
			continue
		}
		id, err := s.identityOf(p)
		if err != nil {
			continue
		}
		f := cardFound{Rec: rec, Path: p, Match: rec.identity().Compare(id)}
		if !f.acceptable() {
			continue
		}
		if s.held != nil {
			if n := s.held(p); n != 0 {
				if held == nil {
					held = &cardFound{Rec: rec, Held: n, HeldName: filepath.Base(p), HeldPath: p}
				}
				continue
			}
		}
		found = append(found, f)
	}
	if len(found) == 0 {
		if held != nil {
			return *held
		}
		return cardFound{Rec: rec}
	}
	best := found[0]
	for _, f := range found[1:] {
		if rank(f) > rank(best) {
			best = f
		}
	}
	var names []string
	seen := map[string]bool{}
	for _, f := range found {
		if rank(f) != rank(best) {
			continue
		}
		name := filepath.Base(f.Path)
		if key := strings.ToLower(name); !seen[key] {
			seen[key] = true
			names = append(names, name)
		}
	}
	if len(names) > 1 {
		return cardFound{Rec: rec, Several: names}
	}
	return best
}
