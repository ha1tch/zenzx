//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// The dialogue for the cards of a snapshot is a statement of what is needed and a list of the card
// images to choose one from (cardsask_gui.go). These tests are of the list, of the file browser that
// Browse... opens (it can be cancelled, and it opens where the cards are), and of the buttons.

// frameWith runs one frame of the dialogue with in, as the bar does, and gives what it came to to
// answerCards.
func frameWith(b *appMenuBar, zx *ZenZX, in zenui.Input) {
	a := b.cardsAsk
	status, result, enter := a.frame(in, testNoopRenderer{}, 960, 768)
	b.answerCards(zx, status, result, enter)
}

// press clicks the button of the dialogue named name, as the frame does: the dialogue is drawn, and
// the click goes to it. It says whether the button is there.
func press(t *testing.T, b *appMenuBar, zx *ZenZX, name string) {
	t.Helper()
	a := b.cardsAsk
	r := testNoopRenderer{}
	a.draw(r, 960, 768, zenui.DarkTheme())
	rects, _ := a.bar.Layout(a.barArea, a.lineH, func(s string) int { return r.TextWidth(s, a.scale) })
	for i, bb := range a.bar.Buttons {
		if bb.Label == name {
			frameWith(b, zx, zenui.Input{MouseX: rects[i].X + 2, MouseY: rects[i].Y + 2, MousePressed: true})
			return
		}
	}
	t.Fatalf("no button %q among %v", name, a.buttons)
}

// clickRow clicks the row of the list that is for the file called name.
func clickRow(t *testing.T, b *appMenuBar, zx *ZenZX, name string) {
	t.Helper()
	a := b.cardsAsk
	a.draw(testNoopRenderer{}, 960, 768, zenui.DarkTheme())
	for i, r := range a.rows {
		if filepath.Base(r.path) == name {
			rec := a.list.ItemRect(i, a.listArea, a.lineH)
			frameWith(b, zx, zenui.Input{MouseX: rec.X + 4, MouseY: rec.Y + 2, MousePressed: true})
			return
		}
	}
	t.Fatalf("no row for %q among %v", name, a.rows)
}

func rowFor(t *testing.T, a *cardsAsk, name string) cardRow {
	t.Helper()
	for _, r := range a.rows {
		if filepath.Base(r.path) == name {
			return r
		}
	}
	t.Fatalf("no row for %q among %v", name, a.rows)
	return cardRow{}
}

// Cancelling the browser brings back the dialogue, with nothing answered, and Browse... works again,
// and Cancel ends it.
func TestCancellingTheFileBrowserBringsBackTheDialogue(t *testing.T) {
	c := newCardsFixture(t)
	k := c.movedCard(t)
	b, zx := dialogueOver(t, k)
	for round := 1; round <= 3; round++ {
		press(t, b, zx, "Browse...")
		if b.diskDialog == nil || b.dialogKind != dialogChooseCard {
			t.Fatalf("round %d: Browse... opened %v", round, b.diskDialog)
		}
		// what a frame does with a browser that was cancelled (Escape, Cancel, or a click outside it)
		b.diskDialog, b.dialogKind = nil, dialogDisk
		status, _, _ := k.a.frame(zenui.Input{}, testNoopRenderer{}, 960, 768)
		if status != zenui.Active {
			t.Fatalf("round %d: the dialogue answers %v by itself after the browser was cancelled", round, status)
		}
		if b.cardsAsk == nil {
			t.Fatalf("round %d: the dialogue went with the browser", round)
		}
	}
	// Enter with no card in the list to choose does not open the browser, or anything
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyEnter}})
	if b.diskDialog != nil || b.cardsAsk == nil {
		t.Error("Enter with nothing to choose did something")
	}
	// and with a file chosen that is not the card, Choose another lets go of it and brings back the list
	other := differentCard(t, t.TempDir(), "other.img")
	k.a.choose(other)
	if k.a.listed() || k.a.buttons[1] != "Choose another" {
		t.Fatalf("with a file that is not the card chosen the buttons are %v", k.a.buttons)
	}
	k.a.load()
	if !k.a.confirm {
		t.Fatal("Load did not say once what going on with a file that is not the card comes to")
	}
	press(t, b, zx, "Choose another")
	if !k.a.listed() || k.a.confirm || k.a.why != "" || k.a.search.chosen[needKey(0, 1)] != "" {
		t.Fatalf("Choose another did not bring back the list: listed %v, confirm %v, why %q", k.a.listed(), k.a.confirm, k.a.why)
	}
	press(t, b, zx, "Browse...")
	if b.diskDialog == nil {
		t.Fatal("Browse... did not open the browser again")
	}
	b.diskDialog, b.dialogKind = nil, dialogDisk
	// Escape ends the dialogue
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
	if b.cardsAsk != nil {
		t.Error("Escape did not end the dialogue")
	}
	b.askForCards(k.a)
	press(t, b, zx, "Cancel")
	if b.cardsAsk != nil {
		t.Error("Cancel did not end the dialogue")
	}
}

// The browser opens in the folder of the card that another machine has, where its copies are; once a
// file has been chosen, in the folder it was chosen from; else in the first folder looked through
// that is there.
func TestTheFileBrowserOpensWhereTheCardsAre(t *testing.T) {
	cards, elsewhere, searched := t.TempDir(), t.TempDir(), t.TempDir()
	held := searchCard(t, cards, "next-card.img")
	rec := searchRecord(t, 1, held)
	missing := filepath.Join(t.TempDir(), "gone")
	search := &cardSearch{dirs: []string{"", missing, searched}}
	need := cardNeed{recs: []cardRecord{rec}, found: []cardFound{{Rec: rec, Held: 2, HeldName: "next-card.img", HeldPath: held}}}
	a := newCardsAsk("warhawk.zxs", false, []cardNeed{need}, search, nil, nil)

	if got := a.browseDir(0, 1); got != cards {
		t.Errorf("the browser opens in %q, want the folder of the card in use, %q", got, cards)
	}
	b, zx := newTestBar(t, nil, "")
	b.askForCards(a)
	press(t, b, zx, "Browse...")
	if b.diskDialog == nil || b.diskDialog.Dir() != cards {
		t.Fatalf("the browser is %v, want it open in %s", b.diskDialog, cards)
	}
	b.diskDialog, b.dialogKind = nil, dialogDisk

	// after a choice, where it was made from
	chosen := searchCard(t, elsewhere, "copy.img")
	a.choose(chosen)
	if got := a.browseDir(0, 1); got != elsewhere {
		t.Errorf("after a file was chosen the browser opens in %q, want %q", got, elsewhere)
	}

	// with no card in use, the first folder looked through that is there (not "" and not one that has gone)
	free := cardNeed{recs: []cardRecord{rec}, found: []cardFound{{Rec: rec}}}
	b2 := newCardsAsk("warhawk.zxs", false, []cardNeed{free}, search, nil, nil)
	if got := b2.browseDir(0, 1); got != searched {
		t.Errorf("the browser opens in %q, want %q", got, searched)
	}
	if got := newCardsAsk("x", false, []cardNeed{free}, &cardSearch{}, nil, nil).browseDir(0, 1); got != "." {
		t.Errorf("with no folder to look in the browser opens in %q", got)
	}
	// a card in use whose folder has gone is not a place to open
	gone := cardNeed{recs: []cardRecord{rec}, found: []cardFound{{Rec: rec, Held: 2, HeldName: "x.img", HeldPath: filepath.Join(missing, "x.img")}}}
	if got := newCardsAsk("x", false, []cardNeed{gone}, search, nil, nil).browseDir(0, 1); got != searched {
		t.Errorf("the folder of a card in use that has gone is opened: %q", got)
	}
}

// The card that is wanted is in use by another machine, which is what the dialogue says, in a line,
// with the card images there are below it: the one in use is there, labelled IN USE, and cannot be
// chosen, however it is tried; the ones that can are told apart, and Choose takes the one selected.
func TestTheListOfCardsHasTheOnesInUseLabelledAndNotToBeChosen(t *testing.T) {
	cards := t.TempDir()
	held := searchCard(t, cards, "next-card.img")
	searchCard(t, cards, "copy.img") // copies: the card's size and layout
	searchCard(t, cards, "copy2.img")
	differentCard(t, cards, "odd.img") // laid out otherwise
	if err := os.WriteFile(filepath.Join(cards, "junk.img"), []byte("not a card"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := searchRecord(t, 1, held)
	search := &cardSearch{dirs: []string{cards}, held: func(p string) int {
		if same(p, held) {
			return 2
		}
		return 0
	}}
	need := cardNeed{recs: []cardRecord{rec}}
	need.found = search.find(0, need.recs)
	if f := need.found[0]; f.Held != 2 || f.Path != "" {
		t.Fatalf("setting up: the card is %+v", f)
	}
	a := newCardsAsk("warhawk.zxs", false, []cardNeed{need}, search, nil, nil)

	say := strings.Join(a.said, " ")
	if want := `The snapshot warhawk.zxs needs the card "next-card.img" (`; !strings.Contains(say, want) {
		t.Errorf("the dialogue lacks %q: %s", want, say)
	}
	for _, want := range []string{"which is in use by machine 2", "Choose another card below."} {
		if !strings.Contains(say, want) {
			t.Errorf("the dialogue lacks %q: %s", want, say)
		}
	}
	if len(a.said) > 3 {
		t.Errorf("the dialogue says %d lines, which is too many to read at a glance: %v", len(a.said), a.said)
	}

	// the rows, with what is said of each, in the order: those that can be chosen, then those that cannot
	tags := map[string]string{}
	usable := map[string]bool{}
	for _, r := range a.rows {
		tags[filepath.Base(r.path)] = r.tag
		usable[filepath.Base(r.path)] = r.usable
	}
	want := map[string]string{"next-card.img": "IN USE, machine 2", "copy2.img": "same layout", "copy.img": "same layout", "odd.img": "very different", "junk.img": "NOT A CARD IMAGE"}
	for name, tag := range want {
		if tags[name] != tag {
			t.Errorf("%s is labelled %q, want %q", name, tags[name], tag)
		}
	}
	if usable["next-card.img"] || usable["junk.img"] || !usable["copy2.img"] || !usable["copy.img"] || !usable["odd.img"] {
		t.Errorf("what can be chosen: %v", usable)
	}
	var order []string
	for _, r := range a.rows {
		order = append(order, filepath.Base(r.path))
	}
	if order[0] != "copy.img" || order[1] != "copy2.img" || order[2] != "odd.img" {
		t.Errorf("the rows are in the order %v: the cards that can be chosen come first", order)
	}
	if got := filepath.Base(a.rows[a.list.Selected].path); got != "copy.img" {
		t.Errorf("%s is selected, want the first card that can be chosen", got)
	}
	if !strings.Contains(a.list.Items[0].Name, "MB") || !strings.Contains(rowLine(a, "next-card.img"), "IN USE") {
		t.Errorf("the rows read %q", rowLine(a, "next-card.img"))
	}
	if a.buttons[0] != "Cancel" || a.buttons[1] != "Browse..." || a.buttons[2] != "Choose" || a.buttons[3] != "Load" {
		t.Errorf("the buttons are %v", a.buttons)
	}
	if a.bar.Buttons[3].Disabled != true || a.bar.Buttons[2].Disabled {
		t.Errorf("Load, with the first card not chosen, can be pressed, or Choose, with a card selected, cannot: %+v", a.bar.Buttons)
	}

	b, zx := newTestBar(t, nil, "")
	b.askForCards(a)
	// a click on the card in use, or on a file that is no card, chooses nothing and moves nothing
	for _, name := range []string{"next-card.img", "junk.img"} {
		clickRow(t, b, zx, name)
		if got := filepath.Base(a.rows[a.list.Selected].path); got != "copy.img" {
			t.Errorf("a click on %s moved the selection to %s", name, got)
		}
	}
	// the keys move over the rows that cannot be chosen
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyDown}})
	if got := filepath.Base(a.rows[a.list.Selected].path); got != "copy2.img" {
		t.Errorf("Down went to %s", got)
	}
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyDown}})
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyDown}})
	if got := filepath.Base(a.rows[a.list.Selected].path); got != "odd.img" {
		t.Errorf("Down at the last card that can be chosen went to %s", got)
	}
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyUp}})
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyUp}})
	if got := filepath.Base(a.rows[a.list.Selected].path); got != "copy.img" {
		t.Errorf("Up went to %s", got)
	}
	// a click on a card that can be chosen selects it, and Choose takes it
	clickRow(t, b, zx, "copy2.img")
	if got := filepath.Base(a.rows[a.list.Selected].path); got != "copy2.img" {
		t.Fatalf("a click on copy2.img selected %s", got)
	}
	press(t, b, zx, "Choose")
	if a.listed() || a.search.chosen[needKey(0, 1)] != filepath.Join(cards, "copy2.img") {
		t.Fatalf("Choose did not take the card selected: chosen %v, listed %v", a.search.chosen, a.listed())
	}
	// a copy, which has the card's size and layout, is taken for it: no warning, and Load goes ahead
	// at once, with the cards approved for the loader
	say = strings.Join(a.said, " ")
	if !strings.Contains(say, "you chose copy2.img, which has the same size and layout") || a.confirm || a.why != "" {
		t.Errorf("a copy is not taken for the card: %s", say)
	}
	if strings.Join(a.buttons, ",") != "Cancel,Load" {
		t.Errorf("the buttons after the choice are %v", a.buttons)
	}
	plan, ok := a.load()
	if !ok || plan[0].one != filepath.Join(cards, "copy2.img") || !plan[0].approved {
		t.Errorf("Load: %+v, %v (%s)", plan, ok, a.why)
	}
}

// What is very different from the card is another matter: it is chosen, and said to be, and Load
// says what going on with it comes to.
func TestACardThatIsVeryDifferentIsSaidSoAndMayBeGoneOnWith(t *testing.T) {
	cards := t.TempDir()
	held := searchCard(t, cards, "next-card.img")
	odd := differentCard(t, cards, "odd.img")
	rec := searchRecord(t, 1, held)
	search := &cardSearch{dirs: []string{cards}, held: func(p string) int {
		if same(p, held) {
			return 2
		}
		return 0
	}}
	need := cardNeed{recs: []cardRecord{rec}}
	need.found = search.find(0, need.recs)
	a := newCardsAsk("x.zxs", false, []cardNeed{need}, search, nil, nil)
	a.choose(odd)
	say := strings.Join(a.said, " ")
	if !strings.Contains(say, "You chose odd.img for the card \"next-card.img\"") || !strings.Contains(say, "odd.img is laid out very differently from the card") {
		t.Errorf("a card that is very different is not said to be: %s", say)
	}
	if strings.Join(a.buttons, ",") != "Cancel,Choose another,Load" {
		t.Errorf("the buttons after the choice are %v", a.buttons)
	}
	if _, ok := a.load(); ok || !a.confirm {
		t.Error("Load went ahead on a card that is very different, without saying so")
	}
}

func rowLine(a *cardsAsk, name string) string {
	for i, r := range a.rows {
		if filepath.Base(r.path) == name {
			return a.list.Items[i].Name
		}
	}
	return ""
}

// Enter chooses the card selected. When every card in the list is in use there is none to choose: no
// row is selected, Choose cannot be pressed, Enter does nothing, and Browse... is what is left, and
// Load for a second card, which may be left out.
func TestWithEveryCardInUseThereIsNothingToChooseButBrowse(t *testing.T) {
	cards := t.TempDir()
	held := searchCard(t, cards, "next-card.img")
	rec := searchRecord(t, 1, held)
	search := &cardSearch{dirs: []string{cards}, held: func(string) int { return 3 }}
	need := cardNeed{recs: []cardRecord{rec}}
	need.found = search.find(0, need.recs)
	a := newCardsAsk("warhawk.zxs", false, []cardNeed{need}, search, nil, nil)
	if a.list.Selected != -1 || a.selectedRow() != -1 || !a.bar.Buttons[2].Disabled {
		t.Fatalf("selected %d, buttons %+v", a.list.Selected, a.bar.Buttons)
	}
	b, zx := newTestBar(t, nil, "")
	b.askForCards(a)
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyEnter}})
	press(t, b, zx, "Choose")
	if b.cardsAsk == nil || len(a.search.chosen) != 0 || b.diskDialog != nil {
		t.Errorf("something was chosen or opened with every card in use: %v", a.search.chosen)
	}
	frameWith(b, zx, zenui.Input{Keys: []zenui.Key{zenui.KeyDown}, MouseX: 1, MouseY: 1})
	if a.list.Selected != -1 {
		t.Errorf("a row that cannot be chosen got selected")
	}
	// Load is not there to press while the first card has no file
	press(t, b, zx, "Load")
	if b.cardsAsk == nil {
		t.Error("Load went ahead with no card")
	}

	// with no card images at all, the list says where to look
	empty := &cardSearch{dirs: []string{t.TempDir()}}
	n2 := cardNeed{recs: []cardRecord{rec}}
	n2.found = empty.find(0, n2.recs)
	a2 := newCardsAsk("warhawk.zxs", false, []cardNeed{n2}, empty, nil, nil)
	var drawn []drawnText
	a2.draw(textAt{at: &drawn}, 960, 768, zenui.DarkTheme())
	seen := false
	for _, d := range drawn {
		seen = seen || strings.Contains(d.s, "No card images")
	}
	if !seen || len(a2.rows) != 0 || !strings.Contains(strings.Join(a2.said, " "), "which was not found") {
		t.Errorf("an empty list: drawn %v, said %v", drawn, a2.said)
	}
}

// A file chosen with Browse... is in the list from then on, with the card images of the folders, so
// that it can be chosen again, and is not listed twice when it is one of them.
func TestAFileChosenWithBrowseIsListedAfterwards(t *testing.T) {
	cards, away := t.TempDir(), t.TempDir()
	held := searchCard(t, cards, "next-card.img")
	rec := searchRecord(t, 1, held)
	search := &cardSearch{dirs: []string{cards}, held: func(p string) int {
		if same(p, held) {
			return 2
		}
		return 0
	}}
	need := cardNeed{recs: []cardRecord{rec}}
	need.found = search.find(0, need.recs)
	a := newCardsAsk("x.zxs", false, []cardNeed{need}, search, nil, nil)
	if len(a.rows) != 1 {
		t.Fatalf("rows %v", a.rows)
	}
	far := differentCard(t, away, "far.img")
	a.choose(far)
	a.choose(held) // chosen, and in use: a file in a folder that is listed is not listed twice
	if len(a.rows) != 2 || filepath.Base(a.rows[0].path) != "far.img" || filepath.Base(a.rows[1].path) != "next-card.img" {
		t.Errorf("the rows are %v", a.rows)
	}
	if a.list.Selected != 0 {
		t.Errorf("the card that can be chosen is not selected: %d", a.list.Selected)
	}
}

// Cards that could each be the one wanted (cards that had no signature, which are told apart by
// nothing) are in the list as the same card, and the dialogue says which, and that one is to be chosen.
func TestSeveralCardsThatCouldBeTheOneAreAllListedAsIt(t *testing.T) {
	cards := t.TempDir()
	one, two := searchCard(t, cards, "a.img"), searchCard(t, cards, "b.img")
	zeroSignature(t, one)
	zeroSignature(t, two)
	rec := searchRecord(t, 1, one)
	rec.Name = "gone.img"
	search := &cardSearch{dirs: []string{cards}}
	need := cardNeed{recs: []cardRecord{rec}}
	need.found = search.find(0, need.recs)
	a := newCardsAsk("x.zxs", false, []cardNeed{need}, search, nil, nil)
	say := strings.Join(a.said, " ")
	if !strings.Contains(say, "More than one card could be it: a.img and b.img. Choose the one you want below.") {
		t.Errorf("the dialogue says %q", say)
	}
	if len(a.rows) != 2 || !a.rows[0].same || !a.rows[1].same || a.rows[0].tag != "the same card" {
		t.Errorf("the rows are %+v", a.rows)
	}
}
