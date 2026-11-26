//go:build headless

package zenzx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The cards of a Next's snapshot (cardrecords.go).

// newTestCard makes a card image in dir, signed as ZenZX makes one.
func newTestCard(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if _, err := nextcard.NewCard(path, nextcard.SmallestCard, nextcard.NewDir("")); err != nil {
		t.Fatal(err)
	}
	return path
}

// unsign takes a card's signature away, as the standard card has none.
func unsign(t *testing.T, path string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteAt([]byte{0, 0, 0, 0}, 0x1B8); err != nil {
		t.Fatal(err)
	}
}

// chainedNext is a Next with the boot chain attached to the card, and a second card when there is
// one. The boot files are empty: nothing runs.
func chainedNext(t *testing.T, card, second string) *ZenZX {
	t.Helper()
	dir := t.TempDir()
	f := nextcard.In(dir)
	for _, p := range []string{f.BootROM, f.DivMMCROM} {
		if err := os.WriteFile(p, make([]byte, 8192), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f.Card = card
	zx := bareNext(t)
	if _, err := zx.attachNextBootChain(f, false, false); err != nil {
		t.Fatal(err)
	}
	if second != "" {
		if _, err := zx.InsertSecondCard(second, false); err != nil {
			t.Fatal(err)
		}
		guestMounts(t, zx.io.divMMC.sd2) // the firmware mounted it
	}
	t.Cleanup(func() { zx.detachNextBootChain() })
	return zx
}

func TestANextSnapshotCarriesItsCards(t *testing.T) {
	dir := t.TempDir()
	one := newTestCard(t, dir, "next-card.img")
	two := newTestCard(t, dir, "games.img")
	unsign(t, two) // a card with no signature gets one when a snapshot is made over it
	zx := chainedNext(t, one, two)

	path := nextSnapshotOf(t, zx)
	recs, err := readSnapshotCards(path, "zxs")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || recs[0].Slot != 1 || recs[1].Slot != 2 {
		t.Fatalf("the snapshot holds %+v", recs)
	}
	for i, p := range []string{one, two} {
		id, err := nextcard.IdentityOfFile(p)
		if err != nil || !id.Keyed() {
			t.Fatalf("%s has no signature after the snapshot: %+v, %v", p, id, err)
		}
		if got := recs[i].identity(); got != id {
			t.Errorf("slot %d: the record says %+v, the card %+v", i+1, got, id)
		}
		if recs[i].Name != filepath.Base(p) {
			t.Errorf("slot %d: the name is %q", i+1, recs[i].Name)
		}
	}
}

func TestASnapshotOfANextWithNoCardsHasNone(t *testing.T) {
	path := nextSnapshotOf(t, bareNext(t))
	recs, err := readSnapshotCards(path, "zxs")
	if err != nil || len(recs) != 0 {
		t.Fatalf("%+v, %v", recs, err)
	}
	// and loads into any Next, as it did before cards were recorded
	if err := bareNext(t).LoadSnapshotFormat(path, "zxs"); err != nil {
		t.Fatal(err)
	}
}

func TestASnapshotLoadsOverItsOwnCards(t *testing.T) {
	dir := t.TempDir()
	one := newTestCard(t, dir, "next-card.img")
	two := newTestCard(t, dir, "games.img")
	path := nextSnapshotOf(t, chainedNext(t, one, two))
	if err := chainedNext(t, one, two).LoadSnapshotFormat(path, "zxs"); err != nil {
		t.Fatalf("over the same cards: %v", err)
	}
	// the cards have been written to since, which changes what is on them and not which they are
	if err := nextcard.AddFiles(t.Context(), one, "/", []nextcard.File{{Name: "new.txt", Data: []byte("x")}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := chainedNext(t, one, two).LoadSnapshotFormat(path, "zxs"); err != nil {
		t.Fatalf("over the same card after it was written: %v", err)
	}
}

func TestASnapshotIsRefusedOverOtherCards(t *testing.T) {
	dir := t.TempDir()
	one := newTestCard(t, dir, "next-card.img")
	other := newTestCard(t, dir, "other.img")
	path := nextSnapshotOf(t, chainedNext(t, one, ""))

	target := chainedNext(t, other, "")
	before := dumpOf(t, target)
	err := target.LoadSnapshotFormat(path, "zxs")
	var mm *cardMismatch
	if !errors.As(err, &mm) {
		t.Fatalf("over another card: %v", err)
	}
	if mm.Want.Name != "next-card.img" || mm.Have != "other.img" || mm.Want.Slot != 1 {
		t.Errorf("%+v", mm)
	}
	if dumpOf(t, target) != before {
		t.Error("the machine was changed by a snapshot that was refused")
	}
	// the person's choice to go on with it lets it through
	target.cardsApproved = true
	if err := target.LoadSnapshotFormat(path, "zxs"); err != nil {
		t.Errorf("with the cards approved: %v", err)
	}
}

func TestASnapshotIsRefusedOverNoCard(t *testing.T) {
	dir := t.TempDir()
	one := newTestCard(t, dir, "next-card.img")
	two := newTestCard(t, dir, "games.img")
	path := nextSnapshotOf(t, chainedNext(t, one, two))

	for name, target := range map[string]*ZenZX{
		"a Next with no cards":       bareNext(t),
		"a Next with no second card": chainedNext(t, one, ""),
	} {
		var mm *cardMismatch
		if err := target.LoadSnapshotFormat(path, "zxs"); !errors.As(err, &mm) || mm.Have != "" {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A card that could not be given a signature (it was opened read-only) is recorded as it is, and a
// snapshot made over it goes over a card of its size and layout.
func TestASnapshotOfACardThatCannotBeSignedTakesAnyCardLikeIt(t *testing.T) {
	dir := t.TempDir()
	one := newTestCard(t, dir, "next-card.img")
	unsign(t, one)
	twin := newTestCard(t, dir, "twin.img")
	unsign(t, twin)

	zx := bareNext(t)
	f := nextcard.In(t.TempDir())
	for _, p := range []string{f.BootROM, f.DivMMCROM} {
		if err := os.WriteFile(p, make([]byte, 8192), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f.Card = one
	if protected, err := zx.attachNextBootChain(f, true, false); err != nil || !protected {
		t.Fatalf("%v %v", protected, err)
	}
	defer zx.detachNextBootChain()
	path := nextSnapshotOf(t, zx)
	recs, _ := readSnapshotCards(path, "zxs")
	if len(recs) != 1 || recs[0].Sig != 0 {
		t.Fatalf("%+v", recs)
	}
	if id, _ := nextcard.IdentityOfFile(one); id.Keyed() {
		t.Error("a read-only card was written")
	}
	if err := chainedNext(t, twin, "").LoadSnapshotFormat(path, "zxs"); err != nil {
		t.Errorf("over a card of the same size and layout: %v", err)
	}
}

func TestAnUndecodableListOfCardsIsRefused(t *testing.T) {
	if _, err := decodeCardRecords([]byte("not json")); err == nil {
		t.Error("accepted")
	}
	r := cardRecord{Slot: 1, Size: 1 << 30, Table: "zz"}
	if r.identity().Size != -1 {
		t.Error("a record with a damaged layout is some card's")
	}
}

// A card that was put in a slot and not mounted is not one the snapshot needs: the machine has not
// read from it, so nothing in its state depends on it. It is not asked for, not given an identity,
// and the snapshot loads over a Next that has another card in the slot, or none.
func TestACardThatWasInsertedAndNotMountedIsNotNeeded(t *testing.T) {
	dir := t.TempDir()
	one := newTestCard(t, dir, "next-card.img")
	two := newTestCard(t, dir, "games.img")
	unsign(t, two)
	other := newTestCard(t, dir, "other.img")
	zx := chainedNext(t, one, two)
	zx.io.divMMC.sd2.served = false // put in after the firmware had looked at the slots

	path := nextSnapshotOf(t, zx)
	recs, err := readSnapshotCards(path, "zxs")
	if err != nil || len(recs) != 1 || recs[0].Slot != 1 {
		t.Fatalf("the snapshot holds %+v, %v", recs, err)
	}
	if id, _ := nextcard.IdentityOfFile(two); id.Keyed() {
		t.Error("a card that was not mounted was given an identity")
	}
	for name, target := range map[string]*ZenZX{
		"no second card":      chainedNext(t, one, ""),
		"another second card": chainedNext(t, one, other),
	} {
		if err := target.LoadSnapshotFormat(path, "zxs"); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// once the firmware has read from it, it is one
	zx.io.divMMC.sd2.served = true
	if recs, _ := readSnapshotCards(nextSnapshotOf(t, zx), "zxs"); len(recs) != 2 {
		t.Errorf("a card that was mounted is not in the snapshot: %+v", recs)
	}
}
