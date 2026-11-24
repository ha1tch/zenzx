//go:build !headless

package zenzx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// testCardDir is a folder with two small card images that boot nothing.
func testCardDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"a.img", "b.img"} {
		if _, err := nextcard.NewCard(filepath.Join(dir, name), nextcard.SmallestCard, nextcard.NewDir("")); err != nil {
			t.Fatalf("making %s: %v", name, err)
		}
	}
	return dir
}

func TestListCardsFindsTheFoldersImagesAndTheExtras(t *testing.T) {
	dir := testCardDir(t)
	other := filepath.Join(t.TempDir(), "elsewhere.img")
	if _, err := nextcard.NewCard(other, nextcard.SmallestCard, nextcard.NewDir("")); err != nil {
		t.Fatal(err)
	}
	cards := listCards(dir, []string{other, filepath.Join(dir, "a.img"), filepath.Join(dir, "gone.img")})
	var names []string
	for _, c := range cards {
		names = append(names, filepath.Base(c.path))
		if c.system {
			t.Errorf("%s: an empty card is listed as one a Next boots from", c.path)
		}
		if c.why == "" {
			t.Errorf("%s: no reason given for not booting", c.path)
		}
		if c.size != nextcard.SmallestCard {
			t.Errorf("%s: size %d, want %d", c.path, c.size, int64(nextcard.SmallestCard))
		}
	}
	if got := strings.Join(names, " "); got != "a.img b.img elsewhere.img" {
		t.Errorf("listed %q, want the folder's two, then the one from elsewhere, each once", got)
	}
}

func TestFreeCardNameAndCopyStem(t *testing.T) {
	dir := testCardDir(t)
	if got := filepath.Base(freeCardName(dir, "card")); got != "card.img" {
		t.Errorf("first free name %q, want card.img", got)
	}
	if got := filepath.Base(freeCardName(dir, "a")); got != "a-2.img" {
		t.Errorf("free name beside a.img is %q, want a-2.img", got)
	}
	for in, want := range map[string]string{"next-card.img": "next-card", "next-card-2.img": "next-card", "x/y/games-12.img": "games", "plain": "plain", "-3.img": "-3"} {
		if got := copyStem(in); got != want {
			t.Errorf("copyStem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCopyCard(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.img"), filepath.Join(dir, "dst.img")
	data := bytes.Repeat([]byte("zenzx card "), 300000) // over 3 MB: several reads
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var last, total int64
	if err := copyCard(context.Background(), src, dst, func(d, tot int64) { last, total = d, tot }); err != nil {
		t.Fatalf("copyCard: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("the copy is not the same bytes")
	}
	if last != int64(len(data)) || total != int64(len(data)) {
		t.Errorf("progress ended at %d of %d, want %d of %d", last, total, len(data), len(data))
	}

	if err := copyCard(context.Background(), src, dst, func(int64, int64) {}); err == nil {
		t.Error("copying over an existing card succeeded")
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, data) {
		t.Error("a refused copy changed or removed the existing card")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stopped := filepath.Join(dir, "stopped.img")
	if err := copyCard(ctx, src, stopped, func(int64, int64) {}); err != context.Canceled {
		t.Errorf("a cancelled copy returned %v", err)
	}
	if _, err := os.Stat(stopped); err == nil {
		t.Error("a cancelled copy left a file")
	}
}

// buttonState is the window's buttons as id -> enabled.
func buttonState(w *cardsWindow) map[string]bool {
	w.setButtons()
	out := map[string]bool{}
	for _, b := range w.buttons.Buttons {
		out[b.ID] = !b.Disabled
	}
	return out
}

func TestCardsWindowButtonsFollowWhoHasTheCard(t *testing.T) {
	dir := testCardDir(t)
	a := filepath.Join(dir, "a.img")
	zx := NewZenZX(AudioBackendOto)
	uses := map[string]cardUse{}
	use := func(p string) cardUse { return uses[filepath.Base(p)] }
	w := newCardsWindow(zx, 2, dir, false, use)
	if len(w.cards) != 2 || w.list.Selected != 0 {
		t.Fatalf("%d cards listed, selection %d", len(w.cards), w.list.Selected)
	}

	// A free data card, on a machine that is not a running Next.
	st := buttonState(w)
	if st[cardBtnBoot] || st[cardBtnSlot2] || st[cardBtnRemove] {
		t.Errorf("a free card that does not boot, on a machine with no slots: %v", st)
	}
	if !st[cardBtnCopy] || !st[cardBtnNewSmall] || !st[cardBtnClose] {
		t.Errorf("copy, new and close should be available: %v", st)
	}

	// The same card, were it one that boots.
	w.cards[0].system = true
	if st := buttonState(w); !st[cardBtnBoot] {
		t.Error("a free card that boots cannot be booted from")
	}
	// In another machine: not this one's to take.
	uses["a.img"] = cardUse{machine: 1, slot: 1}
	if st := buttonState(w); st[cardBtnBoot] || st[cardBtnSlot2] {
		t.Errorf("a card in another machine can be taken: %v", st)
	}
	if got := w.row(w.cards[0]); !strings.Contains(got, "machine 1, slot 1") || !strings.Contains(got, "boots") {
		t.Errorf("row %q does not say who has the card and that it boots", got)
	}
	// do refuses a disabled button even when asked directly.
	if res := w.do(cardBtnBoot); res != cardsOpen || w.bootCard != "" {
		t.Error("do carried out a disabled button")
	}
	// In this machine's second slot: it can be taken out, or booted from.
	uses["a.img"] = cardUse{machine: 2, slot: 2}
	if st := buttonState(w); !st[cardBtnRemove] || !st[cardBtnBoot] {
		t.Errorf("a card in this machine's slot 2: %v", st)
	}
	if got := w.row(w.cards[0]); !strings.Contains(got, "slot 2 here") {
		t.Errorf("row %q does not say the card is here", got)
	}
	uses["a.img"] = cardUse{}
	if res := w.do(cardBtnBoot); res != cardsBoot || !same(w.bootCard, a) {
		t.Errorf("booting from the free card: result %v, card %q", res, w.bootCard)
	}
}

func TestCardsWindowNewAndCopy(t *testing.T) {
	dir := testCardDir(t)
	zx := NewZenZX(AudioBackendOto)
	w := newCardsWindow(zx, 1, dir, false, func(string) cardUse { return cardUse{} })

	w.do(cardBtnNewSmall)
	if len(w.cards) != 3 || filepath.Base(w.cards[w.list.Selected].path) != "card.img" {
		t.Fatalf("after New: %d cards, selected %q", len(w.cards), filepath.Base(w.cards[w.list.Selected].path))
	}

	w.selectPath(filepath.Join(dir, "b.img"))
	w.do(cardBtnCopy)
	for w.busy() {
	}
	w.finishCopy()
	if len(w.cards) != 4 || filepath.Base(w.cards[w.list.Selected].path) != "b-2.img" {
		t.Fatalf("after Copy: %d cards, selected %q; note %q", len(w.cards), filepath.Base(w.cards[w.list.Selected].path), w.note)
	}
	want, _ := os.ReadFile(filepath.Join(dir, "b.img"))
	got, _ := os.ReadFile(filepath.Join(dir, "b-2.img"))
	if !bytes.Equal(want, got) {
		t.Error("the copy differs from the card")
	}
}

func TestCardsWindowLaysOutInSmallAndLargeWindows(t *testing.T) {
	dir := testCardDir(t)
	zx := NewZenZX(AudioBackendOto)
	for _, must := range []bool{false, true} {
		w := newCardsWindow(zx, 2, dir, false, func(string) cardUse { return cardUse{} })
		w.must, w.taken = must, filepath.Join(dir, "a.img")
		for _, size := range [][2]int{{960, 768}, {640, 480}, {352, 296}} {
			r := testNoopRenderer{}
			w.Draw(r, size[0], size[1], zenui.DarkTheme())
			if w.panel.X < 0 || w.panel.X+w.panel.W > size[0] {
				t.Errorf("must=%v %v: the panel is wider than the window: %+v", must, size, w.panel)
			}
			if w.listArea.H < 2*(w.lineH+4) {
				t.Errorf("must=%v %v: the list shows fewer than two cards", must, size)
			}
			if w.barArea.Y < w.listArea.Y+w.listArea.H {
				t.Errorf("must=%v %v: the buttons overlap the list", must, size)
			}
			if w.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyDown}}, r, size[0], size[1]) != cardsOpen || w.list.Selected != 1 {
				t.Errorf("must=%v %v: Down did not move to the second card", must, size)
			}
			w.list.Selected = 0
		}
		if w.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}}, testNoopRenderer{}, 960, 768) != cardsClosed {
			t.Errorf("must=%v: Escape did not close the window", must)
		}
	}
}

func TestDeckCardUse(t *testing.T) {
	d := testDeck(t, 2)
	dir := testCardDir(t)
	a, b := filepath.Join(dir, "a.img"), filepath.Join(dir, "b.img")
	fa, err := os.Open(a)
	if err != nil {
		t.Fatal(err)
	}
	defer fa.Close()
	d.machines[1].zx.next = &nextChain{files: nextcard.Files{Dir: dir, Card: a}, card: fa}
	if u := d.cardUse(a); u != (cardUse{machine: 2, slot: 1}) {
		t.Errorf("cardUse of the second machine's boot card = %+v", u)
	}
	if u := d.cardUse(b); u != (cardUse{}) {
		t.Errorf("cardUse of a card nobody has = %+v", u)
	}
	if n := d.numberOf(d.machines[1].zx); n != 2 {
		t.Errorf("numberOf the second machine = %d", n)
	}
	d.machines[1].zx.next = nil
}

func TestCardsWindowMakeBootable(t *testing.T) {
	dir := testBootFolder(t) // the standard card, a system card, is in it
	empty, err := newEmptyCard(filepath.Join(dir, "empty"), smallestCard)
	if err != nil {
		t.Fatal(err)
	}
	zx := NewZenZX(AudioBackendOto)
	uses := map[string]cardUse{}
	w := newCardsWindow(zx, 1, dir, false, func(p string) cardUse { return uses[filepath.Base(p)] })
	enabled := func() bool {
		w.setButtons()
		for _, b := range w.buttons.Buttons {
			if b.ID == cardBtnBootable {
				return !b.Disabled
			}
		}
		t.Fatal("the window has no Make bootable button")
		return false
	}

	w.selectPath(nextcard.In(dir).Card)
	if enabled() {
		t.Error("Make bootable is offered for a card that boots already")
	}
	w.selectPath(empty)
	if !enabled() {
		t.Fatal("Make bootable is not offered for an empty card with the standard card beside it")
	}
	uses["empty.img"] = cardUse{machine: 2, slot: 2}
	if enabled() {
		t.Error("Make bootable is offered for a card that is in a machine")
	}
	uses["empty.img"] = cardUse{}

	w.setButtons()
	w.do(cardBtnBootable)
	w.setButtons()
	if len(w.buttons.Buttons) != 1 || w.buttons.Buttons[0].Label != "Stop" {
		t.Errorf("while it works the buttons are %+v, want one that says Stop", w.buttons.Buttons)
	}
	for w.busy() {
	}
	w.finishCopy()
	c, ok := w.selected()
	if !ok || c.path != empty || !c.system {
		t.Fatalf("afterwards the selected card is %q, bootable %v; note %q", c.path, c.system, w.note)
	}
	if !strings.Contains(w.note, "boots a Next now") {
		t.Errorf("the note says %q", w.note)
	}
	if enabled() {
		t.Error("Make bootable is still offered for the card it has just made bootable")
	}

	// With no standard card to copy from there is nothing to offer.
	bare := t.TempDir()
	other, err := newEmptyCard(filepath.Join(bare, "other"), smallestCard)
	if err != nil {
		t.Fatal(err)
	}
	w = newCardsWindow(zx, 1, bare, false, func(string) cardUse { return cardUse{} })
	w.selectPath(other)
	if enabled() {
		t.Error("Make bootable is offered with no standard card to copy from")
	}
}
