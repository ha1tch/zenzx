package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// Finding the cards of a snapshot (cardsearch.go).

func searchCard(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if _, err := nextcard.NewCard(path, nextcard.SmallestCard, nextcard.NewDir("")); err != nil {
		t.Fatal(err)
	}
	return path
}

func searchRecord(t *testing.T, slot int, path string) cardRecord {
	t.Helper()
	id, err := nextcard.IdentityOfFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return recordOfCard(slot, path, id)
}

// zeroSignature takes the signature out of the card at path.
func zeroSignature(t *testing.T, path string) {
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

// A card is found by what it is and not by what it is called or where it was: renamed and with
// another of the same size beside it, it is the one that has its signature.
func TestACardIsFoundByItsIdentityNotItsName(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	searchCard(t, dir, "decoy.img") // the same size and layout, another signature
	renamed := filepath.Join(dir, "my-games.img")
	if err := os.Rename(mine, renamed); err != nil {
		t.Fatal(err)
	}
	s := &cardSearch{dirs: []string{dir}}
	got := s.find(0, []cardRecord{rec})[0]
	if !got.acceptable() || got.Path != renamed || got.Match != nextcard.Same {
		t.Fatalf("%+v", got)
	}
}

func TestACardThatIsNotThereIsNotFoundAndThePristineCardIsNotIt(t *testing.T) {
	dir := t.TempDir()
	lost := searchCard(t, dir, "lost.img")
	rec := searchRecord(t, 1, lost)
	if err := os.Remove(lost); err != nil {
		t.Fatal(err)
	}
	pristine := searchCard(t, dir, "next-card.img")
	zeroSignature(t, pristine) // the standard card, which has no signature
	s := &cardSearch{dirs: []string{dir}}
	got := s.find(0, []cardRecord{rec})[0]
	if got.Path != "" || got.acceptable() || got.why() != "it is not to be found" {
		t.Errorf("a card with a signature was taken for one without: %+v", got)
	}
}

// A card that had no signature when the snapshot was made is any card of its size and layout.
func TestACardThatHadNoSignatureIsFoundAmongCardsLikeIt(t *testing.T) {
	dir := t.TempDir()
	was := searchCard(t, dir, "was.img")
	zeroSignature(t, was)
	rec := searchRecord(t, 1, was)
	os.Remove(was)
	twin := searchCard(t, dir, "twin.img")
	s := &cardSearch{dirs: []string{dir}}
	got := s.find(0, []cardRecord{rec})[0]
	if !got.acceptable() || got.Path != twin || got.Match != nextcard.Alike {
		t.Fatalf("%+v", got)
	}
}

// Two cards of a snapshot that are alike are not given the same file.
func TestTwoCardsOfASnapshotAreNotGivenOneFile(t *testing.T) {
	dir := t.TempDir()
	a := searchCard(t, dir, "a.img")
	b := searchCard(t, dir, "b.img")
	zeroSignature(t, a)
	zeroSignature(t, b)
	recs := []cardRecord{searchRecord(t, 1, a), searchRecord(t, 2, b)}
	s := &cardSearch{dirs: []string{dir}}
	got := s.find(0, recs)
	if !got[0].acceptable() || !got[1].acceptable() || got[0].Path == got[1].Path {
		t.Fatalf("%+v", got)
	}
	// with one file only, the second card is not found
	os.Remove(b)
	s = &cardSearch{dirs: []string{dir}}
	got = s.find(0, recs)
	if !got[0].acceptable() || got[1].acceptable() {
		t.Fatalf("%+v", got)
	}
}

func TestACardInAnotherMachineIsNotGivenAndIsSaidWhoseItIs(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	s := &cardSearch{dirs: []string{dir}, held: func(p string) int {
		if filepath.Base(p) == "next-card.img" {
			return 2
		}
		return 0
	}}
	got := s.find(0, []cardRecord{rec})[0]
	if got.acceptable() || got.Held != 2 || !strings.Contains(got.why(), "machine 2") || !strings.Contains(got.why(), "next-card.img") {
		t.Errorf("%+v: %s", got, got.why())
	}
	// and a file the person chooses that another machine has
	s = &cardSearch{dirs: []string{dir}, held: func(string) int { return 3 }, chosen: map[string]string{needKey(0, 1): mine}}
	got = s.find(0, []cardRecord{rec})[0]
	if got.acceptable() || got.Held != 3 || !got.Chosen {
		t.Errorf("%+v", got)
	}
}

func TestAFileThePersonChoosesIsLookedAt(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	other := differentCard(t, dir, "other.img")
	text := filepath.Join(dir, "notes.img")
	if err := os.WriteFile(text, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// a card laid out otherwise
	got := (&cardSearch{chosen: map[string]string{needKey(0, 1): other}}).find(0, []cardRecord{rec})[0]
	if got.acceptable() || !got.Chosen || !strings.Contains(got.why(), "laid out very differently") {
		t.Errorf("another card: %+v: %s", got, got.why())
	}
	// a copy: the same size and layout with another signature is taken for the card, when it is chosen
	copyOf := searchCard(t, dir, "copy.img")
	got = (&cardSearch{chosen: map[string]string{needKey(0, 1): copyOf}}).find(0, []cardRecord{rec})[0]
	if !got.acceptable() || !got.Layout || !got.Chosen || got.Path != copyOf {
		t.Errorf("a copy: %+v", got)
	}
	// but it is not found by the search, which tells cards apart by their signatures
	got = (&cardSearch{dirs: []string{t.TempDir()}}).find(0, []cardRecord{rec})[0]
	if got.acceptable() {
		t.Errorf("found with nothing to find: %+v", got)
	}
	// the card itself, from wherever
	got = (&cardSearch{chosen: map[string]string{needKey(0, 1): mine}}).find(0, []cardRecord{rec})[0]
	if !got.acceptable() || got.Path != mine {
		t.Errorf("the card: %+v", got)
	}
	// not a card
	got = (&cardSearch{chosen: map[string]string{needKey(0, 1): text}}).find(0, []cardRecord{rec})[0]
	if got.acceptable() || !strings.Contains(got.why(), "not a card image") {
		t.Errorf("a text file: %+v: %s", got, got.why())
	}
	// a card of another size
	big := filepath.Join(dir, "big.img")
	if _, err := nextcard.NewCard(big, nextcard.SmallestCard+(64<<20), nextcard.NewDir("")); err != nil {
		t.Fatal(err)
	}
	got = (&cardSearch{chosen: map[string]string{needKey(0, 1): big}}).find(0, []cardRecord{rec})[0]
	if got.acceptable() || !strings.Contains(got.why(), "MB") {
		t.Errorf("a bigger card: %+v: %s", got, got.why())
	}
	// the same file for both cards of a snapshot
	rec2 := searchRecord(t, 2, mine)
	s := &cardSearch{chosen: map[string]string{needKey(0, 1): mine, needKey(0, 2): mine}}
	got2 := s.find(0, []cardRecord{rec, rec2})
	if !got2[0].acceptable() || got2[1].acceptable() {
		t.Errorf("one file for two cards: %+v", got2)
	}
}

func TestFindingLooksAtEachFolderOnceAndSkipsWhatIsNotACardImage(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "folder.img"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := &cardSearch{dirs: []string{dir, dir, filepath.Join(dir, "..", filepath.Base(dir))}}
	if files := s.candidates(); len(files) != 1 {
		t.Fatalf("%v", files)
	}
	if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() {
		t.Errorf("%+v", got)
	}
}

// differentCard is a card image that is not a copy of any other: its partition layout is not theirs.
func differentCard(t *testing.T, dir, name string) string {
	t.Helper()
	path := searchCard(t, dir, name)
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteAt([]byte{0x77, 0x01}, 0x1BE+8); err != nil { // where the first partition starts
		t.Fatal(err)
	}
	return path
}
