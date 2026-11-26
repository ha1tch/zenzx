//go:build !headless

package zenzx

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// Adversarial tests of the window's part of the card resolution workflow (snapshotcards_gui.go,
// cardsask_gui.go): the world changes between the question and the answer, the answers are not
// the ones that were hoped for, and the files are not what they were.

// askedCard is a window that has put the question about a card to the person: the snapshot's card
// has been moved to a folder nobody looks in, and the dialogue is open.
type askedCard struct {
	d       *machineDeck
	a       *cardsAsk
	retyped *[]string
	path    string // the snapshot
	moved   string // the card
}

func (c *cardsFixture) movedCard(t *testing.T) *askedCard {
	t.Helper()
	one := searchCard(t, c.cards, "next-card.img")
	path := c.snapshotWithCards(t, cardSlots{one: one})
	moved := filepath.Join(t.TempDir(), "elsewhere.img")
	if err := os.Rename(one, moved); err != nil {
		t.Fatal(err)
	}
	d, retyped := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); !errors.Is(err, errCardsAsked) {
		t.Fatalf("%v", err)
	}
	return &askedCard{d: d, a: (*asked)[0], retyped: retyped, path: path, moved: moved}
}

// choose answers the question with the card, and returns what Load comes to.
func (k *askedCard) choose(t *testing.T) cardSlots {
	t.Helper()
	k.a.choose(k.moved)
	plan, ok := k.a.load()
	if !ok {
		t.Fatalf("Load refused the card: %s", k.a.why)
	}
	return plan[0]
}

// A card that another machine took while the dialogue was open is not given to a second one: two
// machines writing the same image would ruin it.
func TestACardAnotherMachineTookWhileTheDialogueWasOpenIsNotGiven(t *testing.T) {
	c := newCardsFixture(t)
	k := c.movedCard(t)
	slots := k.choose(t)

	at := k.d.add("next")
	holder := k.d.machines[at].zx
	if err := c.bar.mountCards(holder, cardSlots{one: k.moved}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { holder.detachNextBootChain() })
	before := len(k.d.machines)

	err := k.d.openSnapshotWith(k.path, "", false, &slots)
	if err == nil || !strings.Contains(err.Error(), "machine 2") {
		t.Fatalf("the card was given, or the refusal does not say whose it is: %v", err)
	}
	if len(k.d.machines) != before || cardsOf(holder)[1] != k.moved {
		t.Error("the refusal disturbed the machines")
	}
	if k.d.cardUse(k.moved).machine != 2 {
		t.Errorf("the card is in machine %d", k.d.cardUse(k.moved).machine)
	}
	if msg := snapshotRefusal("n.zxs", err); !strings.Contains(msg, "elsewhere.img") {
		t.Errorf("the refusal says %q", msg)
	}
}

// A card given another signature after the person chose it is not the card any more, and the
// snapshot is not loaded over it: the machine made for it is not left behind, and the card is free.
func TestACardThatWasResignedAfterItWasChosenIsRefusedAndLeavesNothing(t *testing.T) {
	c := newCardsFixture(t)
	k := c.movedCard(t)
	slots := k.choose(t)
	if slots.approved {
		t.Fatal("the card was taken for the one on a promise")
	}
	if err := nextcard.Resign(k.moved); err != nil {
		t.Fatal(err)
	}
	before := len(k.d.machines)
	err := k.d.openSnapshotWith(k.path, "", false, &slots)
	var mm *cardMismatch
	if !errors.As(err, &mm) {
		t.Fatalf("the load over a card that is not the one went ahead or failed otherwise: %v", err)
	}
	if len(k.d.machines) != before {
		t.Errorf("%d machines after, %d before", len(k.d.machines), before)
	}
	if u := k.d.cardUse(k.moved); u.machine != 0 {
		t.Errorf("the refused load left the card in machine %d", u.machine)
	}
	for _, m := range k.d.machines {
		if m.zx.cardsApproved {
			t.Error("the cards are left approved after a refusal")
		}
	}
}

// A card that is gone by the time Load is pressed is said to be, before anything is changed: the
// machine that is there is not turned into a Next, nor given another card, for a load that cannot be.
func TestACardThatIsGoneWhenLoadIsPressedChangesNothing(t *testing.T) {
	c := newCardsFixture(t)
	k := c.movedCard(t)
	slots := k.choose(t)
	if err := os.Remove(k.moved); err != nil {
		t.Fatal(err)
	}
	// the window is at one machine, so that the snapshot goes into the one there is, a 48K
	k.d.multiOn = func() bool { return false }
	front := k.d.machines[k.d.front]
	err := k.d.openSnapshotWith(k.path, "", false, &slots)
	if err == nil || !strings.Contains(err.Error(), "elsewhere.img") {
		t.Fatalf("a card that is not there was loaded over, or the refusal does not name it: %v", err)
	}
	if len(*k.retyped) != 0 || front.zx.machineClass() == classNext || k.d.machines[k.d.front] != front {
		t.Errorf("the machine was turned into a Next (%v) for a load that could not be", *k.retyped)
	}
}

// The same file for both slots is one card in two places, and is not mounted.
func TestTheSameFileForBothSlotsIsRefused(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	two := searchCard(t, c.cards, "games.img")
	path := c.snapshotWithCards(t, cardSlots{one: one, two: two})
	d, _ := snapshotDeck(t)
	c.wire(d)
	before := len(d.machines)
	for what, slots := range map[string]cardSlots{
		"one name":   {one: one, two: one},
		"two names":  {one: one, two: linkTo(t, one)},
		"the second": {one: two, two: two, approved: true},
	} {
		err := d.openSnapshotWith(path, "", false, &slots)
		if err == nil || !strings.Contains(err.Error(), "both slots") {
			t.Errorf("%s: %v", what, err)
		}
		if len(d.machines) != before {
			t.Errorf("%s: %d machines", what, len(d.machines))
		}
	}
}

// linkTo is a second name for the file at path, in its folder.
func linkTo(t *testing.T, path string) string {
	t.Helper()
	alias := filepath.Join(filepath.Dir(path), "alias-of-"+filepath.Base(path))
	if err := os.Symlink(path, alias); err != nil {
		t.Skipf("no links here: %v", err)
	}
	return alias
}

// A plan that gives one card to two machines of a multisnapshot is refused whole, before a machine is
// made or closed.
func TestAPlanThatGivesOneCardToTwoMachinesIsRefusedWhole(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	d, path := c.multiWithNext(t, cardSlots{one: one})
	before := append([]*deckMachine(nil), d.machines...)
	_, err := d.loadMultiWith(path, cardPlan{0: {one: one}, 1: {one: one}})
	if err == nil || !strings.Contains(err.Error(), "two machines") {
		t.Fatalf("%v", err)
	}
	if len(d.machines) != len(before) {
		t.Errorf("%d machines after, %d before", len(d.machines), len(before))
	}
	for i, m := range before {
		if d.machines[i] != m {
			t.Error("a machine was replaced by a refused load")
		}
	}
}

// Two cards of a multisnapshot given the same signature (a card copied by a file manager) are
// told apart by what they were called, and a card that is not there is not made up from the copy.
func TestMultisnapshotCardsThatNowShareASignatureAreGivenOnce(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	two := searchCard(t, c.cards, "games.img")
	d, path := c.multiWithNext(t, cardSlots{one: one, two: two})
	// the second card is now a copy of the first, signature and all
	id, err := nextcard.IdentityOfFile(one)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(two, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	var sig [4]byte
	binary.LittleEndian.PutUint32(sig[:], id.Sig)
	if _, err := f.WriteAt(sig[:], 0x1B8); err != nil {
		t.Fatal(err)
	}
	f.Close()

	needs, _, err := d.multiCardNeeds(path)
	if err != nil || len(needs) != 1 || len(needs[0].found) != 2 {
		t.Fatalf("%+v, %v", needs, err)
	}
	got := needs[0].found
	if !got[0].acceptable() || got[0].Path != one {
		t.Errorf("the first card is %+v", got[0])
	}
	if got[1].acceptable() {
		t.Errorf("the second card was made up from a copy of the first: %s", got[1].Path)
	}
	if _, ok := planOf(needs); ok {
		t.Error("a plan was made with a card that is not there")
	}
}

// ---- the dialogue ----

// dialogueOver is a menu bar and a machine, with the dialogue for the card k has put.
func dialogueOver(t *testing.T, k *askedCard) (*appMenuBar, *ZenZX) {
	t.Helper()
	b, zx := newTestBar(t, nil, "")
	b.askForCards(k.a)
	return b, zx
}

// An answer that is not one of the buttons is no answer: it does not load, and does not drop the
// dialogue.
func TestAnAnswerThatIsNoButtonDoesNothing(t *testing.T) {
	c := newCardsFixture(t)
	k := c.movedCard(t)
	b, zx := dialogueOver(t, k)
	before := len(k.d.machines)
	for _, result := range []int{-1, 3, 99, 1 << 30, -1 << 30} {
		b.answerCards(zx, zenui.Accepted, result, false)
		if b.cardsAsk == nil || len(k.d.machines) != before {
			t.Fatalf("answer %d: dialogue %v, %d machines", result, b.cardsAsk, len(k.d.machines))
		}
	}
	// and with the card chosen, so that Load would go ahead, a button that is not there still does not
	k.a.choose(k.moved)
	for _, result := range []int{-1, 5, 99} {
		b.answerCards(zx, zenui.Accepted, result, false)
		if b.cardsAsk == nil || len(k.d.machines) != before {
			t.Fatalf("with a card chosen, answer %d: dialogue %v, %d machines", result, b.cardsAsk, len(k.d.machines))
		}
	}
}

// Cancelling twice, or answering a dialogue that has gone, is nothing: it is not a crash.
func TestAnswersToADialogueThatHasGoneAreNothing(t *testing.T) {
	c := newCardsFixture(t)
	k := c.movedCard(t)
	b, zx := dialogueOver(t, k)
	cancelled := 0
	k.a.cancel = func() { cancelled++ }
	b.answerCards(zx, zenui.Cancelled, 0, false)
	b.answerCards(zx, zenui.Cancelled, 0, false)
	b.answerCards(zx, zenui.Accepted, 0, true)
	b.answerCards(zx, zenui.Accepted, 2, false)
	b.openChooseCardDialog()
	if b.cardsAsk != nil || cancelled != 1 {
		t.Errorf("dialogue %v, cancelled %d times", b.cardsAsk, cancelled)
	}
	if b.diskDialog != nil {
		t.Error("a file browser was opened for a dialogue that is not there")
	}
	// the file browser's answer, after the dialogue has gone
	b.dialogKind = dialogChooseCard
	b.acceptDialog(zx, k.moved)
}

// A dialogue that has nothing to do on Load or on Cancel, which is how a test or a script makes one,
// is not a crash either.
func TestADialogueWithNoCallbacksIsNotACrash(t *testing.T) {
	c := newCardsFixture(t)
	k := c.movedCard(t)
	k.a.done, k.a.cancel = nil, nil
	b, zx := dialogueOver(t, k)
	k.a.choose(k.moved)
	b.answerCards(zx, zenui.Accepted, 1, false) // Load
	if b.cardsAsk != nil {
		t.Error("Load with a card chosen did not end the dialogue")
	}
	b.askForCards(k.a)
	b.answerCards(zx, zenui.Cancelled, 0, false)
}

// Choosing when there is nothing left to choose, or choosing the same file for the second card, or
// something that is not a file, leaves the dialogue as it was and says why.
func TestChoosingWhatIsNotToBeChosen(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	two := searchCard(t, c.cards, "games.img")
	path := c.snapshotWithCards(t, cardSlots{one: one, two: two})
	away := t.TempDir()
	movedOne, movedTwo := filepath.Join(away, "one.img"), filepath.Join(away, "two.img")
	if err := os.Rename(one, movedOne); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(two, movedTwo); err != nil {
		t.Fatal(err)
	}
	d, _ := snapshotDeck(t)
	asked := c.wire(d)
	if err := d.openSnapshot(path, "", false); !errors.Is(err, errCardsAsked) {
		t.Fatalf("%v", err)
	}
	a := (*asked)[0]

	// not a card
	for _, junk := range []string{"", away, filepath.Join(away, "no-such.img"), path} {
		a.choose(junk)
		if m, s, ok := a.target(); !ok || m != 0 || s != 1 {
			t.Errorf("after %q the card to choose is %d/%d, %v", junk, m, s, ok)
		}
		if !strings.Contains(strings.Join(a.said, " "), "not a card image") {
			t.Errorf("after %q the dialogue says %v", junk, a.said)
		}
	}
	// the first card, then the same file for the second
	a.choose(movedOne)
	if _, s, ok := a.target(); !ok || s != 2 {
		t.Fatalf("the second card is not the one to choose next: %d, %v", s, ok)
	}
	a.choose(movedOne)
	if !strings.Contains(strings.Join(a.said, " "), "another of these machines") {
		t.Errorf("one file for both cards is not said to be: %v", a.said)
	}
	plan, ok := a.load()
	if !ok || plan[0].one != movedOne || plan[0].two != "" || !plan[0].approved {
		t.Errorf("Load after the same file for both: %+v, %v (%s)", plan, ok, a.why)
	}
	// the right one for the second
	a.choose(movedTwo)
	plan, ok = a.load()
	if !ok || plan[0].one != movedOne || plan[0].two != movedTwo || plan[0].approved {
		t.Errorf("Load with both: %+v, %v (%s)", plan, ok, a.why)
	}
	// with nothing left to choose, choosing is nothing
	before := append([]cardNeed(nil), a.needs...)
	a.choose(movedOne)
	if len(a.needs) != len(before) || !a.needs[0].found[0].acceptable() || !a.needs[0].found[1].acceptable() {
		t.Errorf("choosing when there was nothing to choose changed the cards: %+v", a.needs[0].found)
	}
}

// A file that is not the card, gone on with, and then another chosen that is: the warning goes, and
// nothing is approved.
func TestChoosingAgainAfterTheWarningStartsOver(t *testing.T) {
	c := newCardsFixture(t)
	k := c.movedCard(t)
	other := differentCard(t, t.TempDir(), "other.img")
	k.a.choose(other)
	if _, ok := k.a.load(); ok || !k.a.confirm {
		t.Fatal("Load went ahead on a card that is not the one")
	}
	k.a.choose(k.moved)
	if k.a.confirm || k.a.why != "" {
		t.Errorf("the warning is still there: %v, %q", k.a.confirm, k.a.why)
	}
	plan, ok := k.a.load()
	if !ok || plan[0].one != k.moved || plan[0].approved {
		t.Errorf("%+v, %v (%s)", plan, ok, k.a.why)
	}
}

// A dialogue for a great many machines, each with long names, is drawn in a window of any size
// without a crash.
func TestADialogueOfManyCardsIsDrawnAtAnySize(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("a long name ", 20)
	var needs []cardNeed
	s := &cardSearch{dirs: []string{dir}}
	for i := 0; i < 40; i++ {
		recs := []cardRecord{
			{Slot: 1, Sig: uint32(i + 1), Size: 32 << 20, Table: strings.Repeat("00", 64), Name: cleanCardName(long)},
			{Slot: 2, Sig: uint32(i + 100), Size: 1 << 40, Table: strings.Repeat("00", 64)},
		}
		needs = append(needs, cardNeed{machine: i, label: "Machine " + string(rune('A'+i%26)), recs: recs, found: s.find(i, recs)})
	}
	a := newCardsAsk(strings.Repeat("x", 300)+".zms", true, needs, s, nil, nil)
	if len(a.said) < 64 {
		t.Fatalf("the dialogue is %d lines: not the case to try", len(a.said))
	}
	for _, size := range [][2]int{{0, 0}, {1, 1}, {100, 60}, {320, 200}, {640, 480}, {960, 768}, {4000, 100}} {
		a.draw(testNoopRenderer{}, size[0], size[1], zenui.DarkTheme())
	}
	a.why = strings.Repeat("it is not to be found ", 100)
	a.confirm = true
	a.build()
	a.draw(testNoopRenderer{}, 320, 200, zenui.DarkTheme())
	_ = a.missingText()
	_ = a.warning()
}

// ---- the loader's last look ----

// The loader refuses a Next's snapshot over cards that are not its own whatever the way it is loaded
// by (a script's snapshot verb, the command line), and the refusal is not a state of the machine: the
// approval does not outlive the load, whichever way the load ends.
func TestTheApprovalOfCardsDoesNotOutliveALoadThatFails(t *testing.T) {
	c := newCardsFixture(t)
	one := searchCard(t, c.cards, "next-card.img")
	path := c.snapshotWithCards(t, cardSlots{one: one})
	other := searchCard(t, c.cards, "other.img")

	// the snapshot with its end cut off: it has its cards and fails when the state is read
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cut := filepath.Join(t.TempDir(), "cut.zxs")
	if err := os.WriteFile(cut, data[:len(data)-len(data)/3], 0o644); err != nil {
		t.Fatal(err)
	}
	d, _ := snapshotDeck(t)
	c.wire(d)
	before := len(d.machines)
	err = d.openSnapshotWith(cut, "", false, &cardSlots{one: other, approved: true})
	if err == nil {
		t.Fatal("a cut snapshot was loaded")
	}
	if len(d.machines) != before {
		t.Errorf("%d machines after a failed load, %d before", len(d.machines), before)
	}
	if u := d.cardUse(other); u.machine != 0 {
		t.Errorf("the failed load left the card in machine %d", u.machine)
	}
	for _, m := range d.machines {
		if m.zx.cardsApproved {
			t.Error("the approval of the cards outlived a failed load")
		}
	}

	// and the loader, asked by anything but the window, does not take the other card for the one
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Skipf("next: %v", err)
	}
	if err := c.bar.mountCards(zx, cardSlots{one: other}); err != nil {
		t.Fatal(err)
	}
	defer zx.detachNextBootChain()
	var mm *cardMismatch
	if err := zx.LoadSnapshotFormat(path, "zxs"); !errors.As(err, &mm) {
		t.Errorf("the loader took a card that is not the one: %v", err)
	}
	if zx.cardsApproved {
		t.Error("the loader left the cards approved")
	}
}
