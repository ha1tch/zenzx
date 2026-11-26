//go:build !headless

package zenzx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The cards of a Next's snapshot, in the window (snapshotcards_gui.go, cardsask_gui.go): found by
// their identity and mounted before the state goes over the machine, asked for when they are not
// to be found, and the same for the Nexts of a multisnapshot.

// cardsFixture is a menu bar that has boot files (empty: nothing runs) to make a Next with, and a
// folder of cards.
type cardsFixture struct {
	bar   *appMenuBar
	cards string // where the cards are kept
}

func newCardsFixture(t *testing.T) *cardsFixture {
	t.Helper()
	boot := t.TempDir()
	f := nextcard.In(boot)
	for _, p := range []string{f.BootROM, f.DivMMCROM} {
		if err := os.WriteFile(p, make([]byte, 8192), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// the standard card, which has no signature of its own
	if _, err := nextcard.NewCard(f.Card, nextcard.SmallestCard, nextcard.NewDir("")); err != nil {
		t.Fatal(err)
	}
	zeroSignature(t, f.Card)
	was := nextFilesUsable
	nextFilesUsable = func(string) error { return nil }
	t.Cleanup(func() { nextFilesUsable = was })
	return &cardsFixture{bar: &appMenuBar{nextFilesDir: boot}, cards: t.TempDir()}
}

// nextWithCards is a Next with the cards in its slots, which is closed at the end of the test.
func (c *cardsFixture) nextWithCards(t *testing.T, slots cardSlots) *ZenZX {
	t.Helper()
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Skipf("next: %v", err)
	}
	if err := c.bar.mountCards(zx, slots); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { zx.detachNextBootChain() })
	if zx.io.divMMC.sd2 != nil {
		guestMounts(t, zx.io.divMMC.sd2) // the firmware mounted it
	}
	resetRun(zx, 1)
	return zx
}

// snapshotWithCards saves a snapshot of a Next that has the cards.
func (c *cardsFixture) snapshotWithCards(t *testing.T, slots cardSlots) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "n.zxs")
	if err := c.nextWithCards(t, slots).SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// wire gives a deck what the window gives it, and returns what it was asked.
func (c *cardsFixture) wire(d *machineDeck) *[]*cardsAsk {
	var asked []*cardsAsk
	d.cardDirs = func() []string { return []string{c.cards} }
	d.mountCards = c.bar.mountCards
	d.askCards = func(a *cardsAsk) { asked = append(asked, a) }
	return &asked
}

func cardsOf(zx *ZenZX) map[int]string {
	out := map[int]string{}
	for _, m := range zx.mountedCards() {
		out[m.slot] = m.path
	}
	return out
}

// A Next's snapshot is loaded over its own cards without a word, whatever they have been called
// since and wherever in the folder they are: the machine it makes has them in its slots.
func TestANextSnapshotFindsItsCardsByIdentity(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	two := searchCard(t, c.cards, "games.img")
	searchCard(t, c.cards, "decoy.img") // a card of the same size and layout, with another signature
	path := c.snapshotWithCards(t, cardSlots{one: one, two: two})

	// the cards are renamed, and each other's names exchanged
	one2, two2 := filepath.Join(c.cards, "was-games.img"), filepath.Join(c.cards, "was-system.img")
	if err := os.Rename(two, one2); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(one, two2); err != nil {
		t.Fatal(err)
	}

	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatalf("%v", err)
	}
	if len(*asked) != 0 {
		t.Fatalf("it asked for cards it could find: %v", (*asked)[0].said)
	}
	front := d.Front()
	if front.machineClass() != classNext {
		t.Fatalf("the machine is a %v", front.machineClass())
	}
	got := cardsOf(front)
	if got[1] != two2 || got[2] != one2 {
		t.Errorf("the cards in the slots are %v, want %s and %s", got, two2, one2)
	}
	if front.cardsApproved {
		t.Error("the cards are left approved after the load")
	}
}

// With the cards gone the person is asked; they choose the file, and it is loaded over the card
// they chose, which has been checked to be the one.
func TestACardThatIsNotFoundIsAskedForAndChosen(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	path := c.snapshotWithCards(t, cardSlots{one: one})
	away := t.TempDir()
	moved := filepath.Join(away, "somewhere-else.img")
	if err := os.Rename(one, moved); err != nil {
		t.Fatal(err)
	}

	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	before := len(d.machines)
	err := d.openSnapshot(path, "", false)
	if !errors.Is(err, errCardsAsked) {
		t.Fatalf("the answer was %v, want that it asks", err)
	}
	if len(*asked) != 1 || len(d.machines) != before {
		t.Fatalf("asked %d times, %d machines", len(*asked), len(d.machines))
	}
	a := (*asked)[0]
	text := strings.Join(a.said, " ")
	if !strings.Contains(text, "not found") || !strings.Contains(text, "\"next-card.img\" (") {
		t.Errorf("the dialogue says %q", text)
	}
	if strings.Join(a.buttons, ",") != "Cancel,Browse...,Choose,Load" {
		t.Errorf("buttons %v", a.buttons)
	}
	// Load before a card is chosen says that one is needed
	if _, ok := a.load(); ok || a.why == "" {
		t.Error("Load went ahead with no card")
	}
	a.choose(moved)
	if _, _, missing := a.target(); missing {
		t.Fatalf("the card that was chosen is not taken: %v", a.said)
	}
	plan, ok := a.load()
	if !ok {
		t.Fatalf("Load refused: %s", a.why)
	}
	a.done(plan)
	if len(d.machines) != before+1 || d.Front().machineClass() != classNext {
		t.Fatalf("%d machines after the load", len(d.machines))
	}
	if got := cardsOf(d.Front()); got[1] != moved {
		t.Errorf("the card in the slot is %v, want %s", got, moved)
	}
}

// A file that is not the card is said not to be, and may be gone on with by pressing Load again.
func TestAFileThatIsNotTheCardIsRefusedThenGoneOnWith(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	path := c.snapshotWithCards(t, cardSlots{one: one})
	if err := os.Remove(one); err != nil {
		t.Fatal(err)
	}
	other := differentCard(t, t.TempDir(), "other.img")

	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); !errors.Is(err, errCardsAsked) {
		t.Fatalf("%v", err)
	}
	a := (*asked)[0]
	a.choose(other)
	if !strings.Contains(strings.Join(a.said, " "), "laid out very differently") {
		t.Errorf("a card that is not the one is not said so: %v", a.said)
	}
	if _, ok := a.load(); ok || !a.confirm {
		t.Fatal("Load went ahead on a card that is not the one, without saying so")
	}
	// the warning says what it may come to and that this file has no system on it (a card made empty
	// here); the dialogue then gives the two options, and the second button is named for the second
	for _, want := range []string{"nothing of yours", "saving could change", "other.img has no NextZXOS files"} {
		if !strings.Contains(a.why, want) {
			t.Errorf("the warning lacks %q: %s", want, a.why)
		}
	}
	said := strings.Join(a.said, " ")
	for _, want := range []string{"Load goes on with the file you chose.", "Choose another lets you pick a different card."} {
		if !strings.Contains(said, want) {
			t.Errorf("the dialogue lacks %q: %s", want, said)
		}
	}
	if a.buttons[1] != "Choose another" || a.buttons[2] != "Load" {
		t.Errorf("the buttons are %v", a.buttons)
	}
	plan, ok := a.load()
	if !ok {
		t.Fatalf("Load a second time refused: %s", a.why)
	}
	a.done(plan)
	if d.Front().machineClass() != classNext || cardsOf(d.Front())[1] != other {
		t.Fatalf("the machine does not have the card that was chosen: %v", cardsOf(d.Front()))
	}
	if d.Front().cardsApproved {
		t.Error("the cards are left approved after the load")
	}
}

// A second card that is not found may be left out: the snapshot is loaded with the slot empty.
func TestASecondCardMayBeLeftOut(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	two := searchCard(t, c.cards, "games.img")
	path := c.snapshotWithCards(t, cardSlots{one: one, two: two})
	if err := os.Remove(two); err != nil {
		t.Fatal(err)
	}

	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); !errors.Is(err, errCardsAsked) {
		t.Fatalf("%v", err)
	}
	a := (*asked)[0]
	if !strings.Contains(strings.Join(a.said, " "), "second slot is then left empty") {
		t.Errorf("the dialogue does not say what leaving it out comes to: %v", a.said)
	}
	plan, ok := a.load()
	if !ok {
		t.Fatalf("Load refused: %s", a.why)
	}
	a.done(plan)
	got := cardsOf(d.Front())
	if got[1] != one || got[2] != "" || d.Front().machineClass() != classNext {
		t.Errorf("the cards are %v", got)
	}
}

// The first card cannot be left out.
func TestTheFirstCardIsNeeded(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	path := c.snapshotWithCards(t, cardSlots{one: one})
	os.Remove(one)
	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); !errors.Is(err, errCardsAsked) {
		t.Fatalf("%v", err)
	}
	a := (*asked)[0]
	if _, ok := a.load(); ok {
		t.Error("loaded with no first card")
	}
	if !strings.Contains(a.why, "needed") {
		t.Errorf("why = %q", a.why)
	}
}

// With no way to ask the snapshot is refused, and says which card.
func TestWithNoWayToAskACardThatIsNotFoundRefuses(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	path := c.snapshotWithCards(t, cardSlots{one: one})
	os.Remove(one)
	d, _ := snapshotDeck(t)
	c.wire(d)
	d.askCards = nil
	before := len(d.machines)
	err := d.openSnapshot(path, "", false)
	var nf *cardsNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("%v", err)
	}
	if len(d.machines) != before {
		t.Error("the refused snapshot changed the window")
	}
	if msg := snapshotRefusal("n.zxs", err); !strings.Contains(msg, "next-card.img") {
		t.Errorf("the refusal says %q", msg)
	}
}

// A card another machine has is not given to the machine of the snapshot, and is said whose it is.
func TestACardAnotherMachineHasIsNotGiven(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	path := c.snapshotWithCards(t, cardSlots{one: one})

	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatal(err)
	}
	holder := d.machines[1]
	if cardsOf(holder.zx)[1] != one {
		t.Fatalf("setting up: %v", cardsOf(holder.zx))
	}
	// the Next is not the one the snapshot is for: it is a machine of its own, with the card
	holder.fromSnapshot = ""
	d.flip(0)
	before := len(d.machines)
	if err := d.openSnapshot(path, "", false); !errors.Is(err, errCardsAsked) {
		t.Fatalf("the card another machine has was given: %v", err)
	}
	if len(d.machines) != before || cardsOf(holder.zx)[1] != one {
		t.Error("the machine that has the card was disturbed")
	}
	text := strings.Join((*asked)[0].said, " ")
	if !strings.Contains(text, "machine 2") || !strings.Contains(text, "next-card.img") {
		t.Errorf("it is not said whose the card is: %q", text)
	}
}

// ---- multisnapshots ----

// multiWithNext is a window of a 48K and a Next that has the cards, saved as a multisnapshot.
func (c *cardsFixture) multiWithNext(t *testing.T, slots cardSlots) (*machineDeck, string) {
	t.Helper()
	d, _ := snapshotDeck(t)
	c.wire(d)
	at := d.add("next")
	m := d.machines[at]
	if err := c.bar.mountCards(m.zx, slots); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.zx.detachNextBootChain() })
	if m.zx.io.divMMC.sd2 != nil {
		guestMounts(t, m.zx.io.divMMC.sd2) // the firmware mounted it
	}
	path := filepath.Join(t.TempDir(), "pair.zms")
	if err := d.saveMulti(path); err != nil {
		t.Fatal(err)
	}
	return d, path
}

// The Nexts of a multisnapshot are given their cards where they are found, by what they are.
func TestAMultisnapshotFindsTheCardsOfItsNexts(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	two := searchCard(t, c.cards, "games.img")
	d, path := c.multiWithNext(t, cardSlots{one: one, two: two})
	// the window's machines are closed to let the cards go; the cards are renamed
	for len(d.machines) > 1 {
		d.remove(len(d.machines) - 1)
	}
	renamed := filepath.Join(c.cards, "renamed.img")
	if err := os.Rename(two, renamed); err != nil {
		t.Fatal(err)
	}

	n, err := d.loadMulti(path)
	if err != nil || n != 2 {
		t.Fatalf("%d machines, %v", n, err)
	}
	var next *ZenZX
	for _, m := range d.machines {
		if m.zx.machineClass() == classNext {
			next = m.zx
			t.Cleanup(func() { m.zx.detachNextBootChain() })
		}
	}
	if next == nil {
		t.Fatal("no Next in the window that was loaded")
	}
	if got := cardsOf(next); got[1] != one || got[2] != renamed {
		t.Errorf("the cards are %v, want %s and %s", got, one, renamed)
	}
	if next.cardsApproved {
		t.Error("the cards are left approved")
	}
}

// With a card gone, loading with no one to ask is refused as a whole, and says which machine; asked
// first, the person's choice is what the machine is given.
func TestAMultisnapshotWhoseCardIsGoneIsAskedFor(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	d, path := c.multiWithNext(t, cardSlots{one: one})
	moved := filepath.Join(t.TempDir(), "elsewhere.img")
	for len(d.machines) > 1 {
		d.remove(len(d.machines) - 1)
	}
	if err := os.Rename(one, moved); err != nil {
		t.Fatal(err)
	}
	before := append([]*deckMachine(nil), d.machines...)

	// refused whole, the window as it was
	_, err := d.loadMulti(path)
	var nf *cardsNotFound
	if !errors.As(err, &nf) || !strings.Contains(err.Error(), "Machine 2") {
		t.Fatalf("%v", err)
	}
	if len(d.machines) != len(before) || d.machines[0] != before[0] {
		t.Error("a refused multisnapshot changed the window")
	}

	needs, search, err := d.multiCardNeeds(path)
	if err != nil || len(needs) != 1 || needs[0].machine != 1 || needs[0].label == "" {
		t.Fatalf("%+v, %v", needs, err)
	}
	if _, ok := planOf(needs); ok {
		t.Fatal("a card that is not found is in the plan")
	}
	a := newCardsAsk("pair.zms", true, needs, search, func(cardPlan) {}, nil)
	if !strings.Contains(strings.Join(a.said, " "), "Machine 2") {
		t.Errorf("the dialogue does not say which machine: %v", a.said)
	}
	a.choose(moved)
	plan, ok := a.load()
	if !ok {
		t.Fatalf("Load refused: %s", a.why)
	}
	n, err := d.loadMultiWith(path, plan)
	if err != nil || n != 2 {
		t.Fatalf("%d machines, %v", n, err)
	}
	for _, m := range d.machines {
		if m.zx.machineClass() == classNext {
			t.Cleanup(func() { m.zx.detachNextBootChain() })
			if got := cardsOf(m.zx)[1]; got != moved {
				t.Errorf("the card is %q, want %s", got, moved)
			}
		}
	}
}

// From the menu: the cards are asked for before the question about closing the machines, and the
// plan goes on through it to the load.
func TestMultisnapshotCardsAreAskedBeforeTheMachinesAreClosed(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	d, path := c.multiWithNext(t, cardSlots{one: one})
	moved := filepath.Join(t.TempDir(), "elsewhere.img")
	if err := os.Rename(one, moved); err != nil {
		t.Fatal(err)
	}

	b, zx := newTestBar(t, nil, "")
	b.layoutState = func() (deckLayout, int) { return layoutSingle, 2 }
	var plans []cardPlan
	b.multiCards = d.multiCardNeeds
	b.loadMulti = func(p string, plan cardPlan) (int, error) { plans = append(plans, plan); return 2, nil }

	b.loadMultiSnapshotAt(zx, path)
	if b.cardsAsk == nil || b.replaceAsk != nil || len(plans) != 0 {
		t.Fatalf("cards %v, question %v, loaded %d: the cards come first", b.cardsAsk, b.replaceAsk, len(plans))
	}
	if !b.modalOpen() || b.modalDrawer(800, 600) == nil {
		t.Error("the dialogue is not shown")
	}
	b.cardsAsk.choose(moved)
	load := -1
	for i, name := range b.cardsAsk.buttons {
		if name == "Load" {
			load = i
		}
	}
	b.answerCards(zx, zenui.Accepted, load, false)
	q := b.replaceAsk
	if b.cardsAsk != nil || q == nil || len(plans) != 0 {
		t.Fatalf("after the cards: dialogue %v, question %v, loaded %d", b.cardsAsk, q, len(plans))
	}
	q.text = replaceWord
	b.answerReplace(zx, zenui.Accepted, false)
	if len(plans) != 1 || plans[0][1].one != moved {
		t.Fatalf("loaded with %v", plans)
	}

	// Cancel in the dialogue loads nothing and asks nothing more
	plans = nil
	b.loadMultiSnapshotAt(zx, path)
	b.answerCards(zx, zenui.Cancelled, 0, false)
	if b.cardsAsk != nil || b.replaceAsk != nil || len(plans) != 0 {
		t.Errorf("cancelled: %v %v %d", b.cardsAsk, b.replaceAsk, len(plans))
	}
}

// The dialogue is drawn, Browse... opens the file browser for the card, the file it answers with is
// the one chosen, and with nothing left to choose Enter loads.
func TestChoosingACardInTheDialogue(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	path := c.snapshotWithCards(t, cardSlots{one: one})
	moved := filepath.Join(t.TempDir(), "elsewhere.img")
	if err := os.Rename(one, moved); err != nil {
		t.Fatal(err)
	}
	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); !errors.Is(err, errCardsAsked) {
		t.Fatalf("%v", err)
	}
	b, zx := newTestBar(t, nil, "")
	b.askForCards((*asked)[0])
	a := b.cardsAsk
	if !b.modalOpen() || b.modalDrawer(960, 768) == nil {
		t.Fatal("the dialogue is not shown")
	}
	a.draw(testNoopRenderer{}, 960, 768, zenui.DarkTheme())

	choose := -1
	for i, name := range a.buttons {
		if name == "Browse..." {
			choose = i
		}
	}
	b.answerCards(zx, zenui.Accepted, choose, false)
	if b.diskDialog == nil || b.dialogKind != dialogChooseCard {
		t.Fatalf("Browse... opened %v of kind %v", b.diskDialog, b.dialogKind)
	}
	if b.modalDrawer(960, 768) == nil {
		t.Error("the file browser is not shown over the dialogue")
	}
	b.acceptDialog(zx, moved)
	if b.cardsAsk == nil || b.diskDialog != nil {
		t.Fatal("choosing a file ended the dialogue")
	}
	if _, _, missing := a.target(); missing {
		t.Fatalf("the file chosen is not taken: %v", a.said)
	}
	if a.buttons[1] != "Load" {
		t.Errorf("with nothing left to choose the buttons are %v", a.buttons)
	}
	before := len(d.machines)
	b.answerCards(zx, zenui.Active, 0, true) // Enter
	if b.cardsAsk != nil || len(d.machines) != before+1 {
		t.Fatalf("Enter: dialogue %v, %d machines", b.cardsAsk, len(d.machines))
	}
	if got := cardsOf(d.Front())[1]; got != moved {
		t.Errorf("the card is %q", got)
	}
}

// The dialogue says which cards are missing by the names they had, each in quotes with its size and
// the slot it was in, and so does the message that nothing was loaded when it is cancelled.
func TestTheDialogueNamesTheCardsThatAreMissing(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "system-card.img")
	two := searchCard(t, c.cards, "my-games.img")
	path := c.snapshotWithCards(t, cardSlots{one: one, two: two})
	os.Remove(one)
	os.Remove(two)

	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); !errors.Is(err, errCardsAsked) {
		t.Fatalf("%v", err)
	}
	a := (*asked)[0]
	text := strings.Join(a.said, " ")
	for _, want := range []string{`"system-card.img" (`, `"my-games.img" (`, "Card 2", "not found"} {
		if !strings.Contains(text, want) {
			t.Errorf("the dialogue lacks %q: %s", want, text)
		}
	}
	b, zx := newTestBar(t, nil, "")
	var said []string
	zx.OnNotice = func(m string) { said = append(said, m) }
	b.askForCards(a)
	b.answerCards(zx, zenui.Cancelled, 0, false)
	if len(said) != 1 || !strings.Contains(said[0], `"system-card.img"`) || !strings.Contains(said[0], `"my-games.img"`) || !strings.Contains(said[0], "were not found") {
		t.Errorf("the message after Cancel: %v", said)
	}

	// and when the snapshot cannot be asked about, the refusal says which as well
	d.askCards = nil
	err := d.openSnapshot(path, "", false)
	if msg := snapshotRefusal("n.zxs", err); !strings.Contains(msg, `"system-card.img"`) {
		t.Errorf("the refusal says %q", msg)
	}
}

// A second card that was put in and not mounted is not asked for when it has gone: the snapshot is
// loaded without a word.
func TestAnUnmountedSecondCardIsNotAskedFor(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	two := searchCard(t, c.cards, "games.img")
	zx := c.nextWithCards(t, cardSlots{one: one, two: two})
	zx.io.divMMC.sd2.served = false
	path := filepath.Join(t.TempDir(), "n.zxs")
	if err := zx.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	os.Remove(two)

	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatalf("%v", err)
	}
	if len(*asked) != 0 || d.Front().machineClass() != classNext {
		t.Fatalf("asked %d times", len(*asked))
	}
	if got := cardsOf(d.Front()); got[1] != one || got[2] != "" {
		t.Errorf("the cards are %v", got)
	}
}
