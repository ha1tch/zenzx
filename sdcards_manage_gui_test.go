//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

func TestRenameTarget(t *testing.T) {
	dir := testCardDir(t)
	a := filepath.Join(dir, "a.img")
	for _, c := range []struct{ typed, target, why string }{
		{"games", "games.img", ""},
		{"  games.IMG ", "games.IMG", ""},
		{"My Games", "My Games.img", ""},
		{"", "", "Type a name."},
		{".img", "", "Type a name."},
		{"a", "", "its name already"},
		{"b", "", "there already"},
		{"x/y", "", "slashes"},
		{`x\y`, "", "slashes"},
		{"x:y", "", "none of"},
		{".hidden", "", "dot"},
		{strings.Repeat("n", 101), "", "too long"},
	} {
		target, why := renameTarget(a, c.typed)
		if c.target != "" && target != filepath.Join(dir, c.target) {
			t.Errorf("%q: target %q, want %q", c.typed, target, c.target)
		}
		if (c.why == "") != (why == "") || !strings.Contains(why, c.why) {
			t.Errorf("%q: why %q, want one with %q", c.typed, why, c.why)
		}
		if why != "" && target != "" {
			t.Errorf("%q: refused (%q) but a target was given: %q", c.typed, why, target)
		}
	}
}

// cardsAsking is a window over a folder of two cards, a.img selected.
func cardsAsking(t *testing.T) (*cardsWindow, string) {
	t.Helper()
	dir := testCardDir(t)
	w := newCardsWindow(NewZenZX(AudioBackendOto), 1, dir, false, func(string) cardUse { return cardUse{} })
	w.selectPath(filepath.Join(dir, "a.img"))
	return w, dir
}

func key(w *cardsWindow, k zenui.Key) cardsResult {
	return w.Update(zenui.Input{Keys: []zenui.Key{k}}, testNoopRenderer{}, 960, 768)
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestRenameAsksTheNameThenAsksToConfirm(t *testing.T) {
	w, dir := cardsAsking(t)
	a, games := filepath.Join(dir, "a.img"), filepath.Join(dir, "games.img")

	w.do(cardBtnRename)
	if w.ask == nil || w.ask.kind != askRenameName || w.ask.text != "a" {
		t.Fatalf("Rename opened %+v, want the name question with the card's name", w.ask)
	}
	w.ask.text = "games"
	key(w, zenui.KeyEnter)
	if w.ask == nil || w.ask.kind != askRenameSure {
		t.Fatalf("after the name: %+v, want the confirmation", w.ask)
	}
	if !exists(a) || exists(games) {
		t.Fatal("the card was renamed before it was confirmed")
	}
	if text := strings.Join(w.ask.said, " "); !strings.Contains(text, "a.img to games.img") {
		t.Errorf("the confirmation does not say what is renamed to what: %q", text)
	}

	// Escape is a No: nothing changes.
	key(w, zenui.KeyEscape)
	if w.ask != nil || !exists(a) || exists(games) || !strings.Contains(w.note, "Not renamed") {
		t.Fatalf("Escape on the confirmation: ask %v, note %q", w.ask, w.note)
	}

	w.do(cardBtnRename)
	w.ask.text = "games"
	key(w, zenui.KeyEnter)
	key(w, zenui.KeyEnter) // Yes
	if w.ask != nil || exists(a) || !exists(games) {
		t.Fatalf("a confirmed rename: ask %v, a.img %v, games.img %v", w.ask, exists(a), exists(games))
	}
	if c, ok := w.selected(); !ok || !same(c.path, games) {
		t.Errorf("the selection is %v, want games.img", c.path)
	}
	if !strings.Contains(w.note, "a.img is now called games.img") {
		t.Errorf("note %q", w.note)
	}
}

func TestRenameRefusesABadNameWithoutAskingToConfirm(t *testing.T) {
	w, dir := cardsAsking(t)
	w.do(cardBtnRename)
	w.ask.text = "b" // taken
	key(w, zenui.KeyEnter)
	if w.ask == nil || w.ask.kind != askRenameName || !strings.Contains(w.ask.why, "there already") {
		t.Fatalf("a taken name: %+v", w.ask)
	}
	if w.ask.text != "b" {
		t.Error("the typed name was lost when it was refused")
	}
	if !exists(filepath.Join(dir, "a.img")) || !exists(filepath.Join(dir, "b.img")) {
		t.Error("a refused rename changed the folder")
	}
}

func TestDeleteWantsTheWordInCapitalsExactly(t *testing.T) {
	w, dir := cardsAsking(t)
	a, b := filepath.Join(dir, "a.img"), filepath.Join(dir, "b.img")
	w.do(cardBtnDelete)
	if w.ask == nil || w.ask.kind != askDelete {
		t.Fatalf("Delete opened %+v", w.ask)
	}
	for _, typed := range []string{"", "delete", "Delete", "DELETE ", " DELETE", "DELET", "DELETEDELETE", "YES"} {
		w.ask.text = typed
		key(w, zenui.KeyEnter)
		if w.ask == nil || !exists(a) {
			t.Fatalf("%q deleted the card (or closed the question)", typed)
		}
		if !strings.Contains(w.ask.why, "DELETE") {
			t.Errorf("%q: no reason shown: %q", typed, w.ask.why)
		}
	}
	// Escape leaves it.
	key(w, zenui.KeyEscape)
	if w.ask != nil || !exists(a) || !strings.Contains(w.note, "Not deleted") {
		t.Fatalf("Escape: ask %v, note %q", w.ask, w.note)
	}

	w.do(cardBtnDelete)
	w.ask.text = "DELETE"
	key(w, zenui.KeyEnter)
	if w.ask != nil || exists(a) || !exists(b) {
		t.Fatalf("DELETE: ask %v, a.img %v, b.img %v", w.ask, exists(a), exists(b))
	}
	if len(w.cards) != 1 || !same(w.cards[w.list.Selected].path, b) {
		t.Errorf("after the delete: %d cards, selected %d", len(w.cards), w.list.Selected)
	}
	if !strings.Contains(w.note, "a.img is deleted") {
		t.Errorf("note %q", w.note)
	}

	// The last card: nothing is left selected, and the buttons that need a card are off.
	w.do(cardBtnDelete)
	w.ask.text = "DELETE"
	key(w, zenui.KeyEnter)
	if len(w.cards) != 0 || w.list.Selected != -1 {
		t.Errorf("no cards left: %d listed, selection %d", len(w.cards), w.list.Selected)
	}
	if st := buttonState(w); st[cardBtnRename] || st[cardBtnDelete] || st[cardBtnCopy] {
		t.Errorf("buttons on with no card: %v", st)
	}
}

func TestRenameAndDeleteLeaveACardAMachineHasAlone(t *testing.T) {
	dir := testCardDir(t)
	uses := map[string]cardUse{"a.img": {machine: 2, slot: 2}}
	w := newCardsWindow(NewZenZX(AudioBackendOto), 1, dir, false, func(p string) cardUse { return uses[filepath.Base(p)] })
	w.selectPath(filepath.Join(dir, "a.img"))
	if st := buttonState(w); st[cardBtnRename] || st[cardBtnDelete] {
		t.Errorf("a card in a slot can be renamed or deleted: %v", st)
	}
	w.do(cardBtnDelete)
	w.do(cardBtnRename)
	if w.ask != nil {
		t.Error("do opened a question for a disabled button")
	}
	w.selectPath(filepath.Join(dir, "b.img"))
	if st := buttonState(w); !st[cardBtnRename] || !st[cardBtnDelete] {
		t.Errorf("a free card cannot be renamed or deleted: %v", st)
	}
}

func TestTheStandardCardCannotBeRenamedButCanBeDeletedWithAWarning(t *testing.T) {
	dir := t.TempDir()
	std := nextcard.In(dir).Card
	if _, err := nextcard.NewCard(std, nextcard.SmallestCard, nextcard.NewDir("")); err != nil {
		t.Fatal(err)
	}
	w := newCardsWindow(NewZenZX(AudioBackendOto), 1, dir, false, func(string) cardUse { return cardUse{} })
	w.selectPath(std)
	if st := buttonState(w); st[cardBtnRename] || !st[cardBtnDelete] {
		t.Errorf("standard card: rename %v, delete %v", st[cardBtnRename], st[cardBtnDelete])
	}
	w.do(cardBtnDelete)
	if text := strings.Join(w.ask.said, " "); !strings.Contains(text, "standard system card") {
		t.Errorf("no warning about the standard card: %q", text)
	}
}

func TestTheQuestionDrawsAndKeepsTheWindowFromTheKeys(t *testing.T) {
	w, _ := cardsAsking(t)
	w.do(cardBtnDelete)
	w.Draw(testNoopRenderer{}, 960, 768, zenui.DarkTheme())
	w.Draw(testNoopRenderer{}, 352, 296, zenui.DarkTheme())
	w.list.Selected = 0
	if key(w, zenui.KeyDown) != cardsOpen || w.list.Selected != 0 {
		t.Error("a key reached the list behind the question")
	}
	if key(w, zenui.KeyEscape) != cardsOpen || w.ask != nil {
		t.Error("Escape closed the window, or not the question")
	}
	if key(w, zenui.KeyEscape) != cardsClosed {
		t.Error("Escape with no question did not close the window")
	}
}

// inkLog is a renderer that notes the colour each text is drawn in.
type inkLog struct {
	testNoopRenderer
	seen *map[string][]zenui.Colour
}

func (l inkLog) DrawText(s string, _, _, _ int, c zenui.Colour) {
	(*l.seen)[s] = append((*l.seen)[s], c)
}

// The reason an answer was refused is drawn in the accent colour, over the label the form
// drew in the text colour; what informs is in the text colour alone.
func TestTheReasonAnAnswerWasRefusedIsInTheAccentColour(t *testing.T) {
	theme := zenui.DarkTheme()
	if theme.DirText == theme.Text {
		t.Fatal("the theme has no accent colour that differs from its text")
	}
	for _, c := range []struct {
		name  string
		start func(*cardsWindow)
		typed string
	}{
		{"rename", func(w *cardsWindow) { w.do(cardBtnRename) }, "b"},
		{"delete", func(w *cardsWindow) { w.do(cardBtnDelete) }, "delete"},
	} {
		w, _ := cardsAsking(t)
		c.start(w)
		seen := map[string][]zenui.Colour{}
		draw := func() { w.Draw(inkLog{seen: &seen}, 960, 768, theme) }

		draw()
		if len(w.ask.whyAt) != 0 {
			t.Fatalf("%s: a reason before any answer: %v", c.name, w.ask.said)
		}
		for text, cols := range seen {
			for _, col := range cols {
				if col == theme.DirText && strings.Contains(text, "already") {
					t.Errorf("%s: %q is in the accent colour before any answer", c.name, text)
				}
			}
		}

		w.ask.text = c.typed
		key(w, zenui.KeyEnter)
		if len(w.ask.whyAt) == 0 {
			t.Fatalf("%s: no reason after a refused answer", c.name)
		}
		seen = map[string][]zenui.Colour{}
		draw()
		for _, i := range w.ask.whyAt {
			line := w.ask.said[i]
			cols := seen[line]
			if len(cols) == 0 || cols[len(cols)-1] != theme.DirText {
				t.Errorf("%s: the reason %q is not drawn last in the accent colour: %v", c.name, line, cols)
			}
		}
		// The lines that inform are not drawn in it.
		for i, line := range w.ask.said {
			if marked := func() bool {
				for _, j := range w.ask.whyAt {
					if j == i {
						return true
					}
				}
				return false
			}(); !marked {
				for _, col := range seen[line] {
					if col == theme.DirText {
						t.Errorf("%s: %q informs and is in the accent colour", c.name, line)
					}
				}
			}
		}
	}
}
