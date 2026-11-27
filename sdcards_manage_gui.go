//go:build !headless

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// ============================================================================
// Renaming and deleting a card image, in the SD Cards window
//
// Both ask before they act, in a dialogue over the window. Rename asks for the new
// name and then whether to go ahead: the name is checked before the second question, so
// that the question is only asked about a rename that can be done. Delete asks for the
// word DELETE, typed in capitals and exactly: nothing else, not a lower-case word, not
// the word with a space after it, deletes anything. The file is deleted, not moved to a
// bin: a card image is large, and a person who deletes one means to have the room.
//
// Only a card that no machine has can be renamed or deleted: a card in a slot is open,
// and the machine would be writing to a file that is gone or under another name. The
// standard system card cannot be renamed, since the Next's files are found by its name;
// it can be deleted, with a line saying what it is for.
// ============================================================================

// cardAskScale is the text scale of the questions: the dialogues' own, named here because the
// reason an answer was refused is drawn over its label, in the accent colour.
const cardAskScale = 2

// deleteWord is what must be typed to delete a card image.
const deleteWord = "DELETE"

// replaceWord is what must be typed to load a multisnapshot over the machines that are running.
const replaceWord = "YES"

// cardAsk is the question the window is asking.
type cardAsk int

const (
	askRenameName cardAsk = iota // the new name
	askRenameSure                // whether to rename
	askDelete                    // the word DELETE
	askReplace                   // the word YES: the machines that are running are to be replaced (a multisnapshot)
)

// cardQuestion is the question the window has put, and what has been answered so far.
type cardQuestion struct {
	kind     cardAsk
	modal    *zenui.Modal
	path     string // the card
	size     int64
	standard bool     // the card is the standard system card
	text     string   // what has been typed in the box
	target   string   // for askRenameSure: the path the card would get
	why      string   // why the last answer was refused
	noun     string   // what it is a question about: "card" when empty (the Snapshots window asks about snapshots)
	ext      string   // the extension its files have, which renaming keeps: ".img" when empty
	sizeText string   // the file's size in words, when it is not in MB
	machines int      // for askReplace: how many machines are running
	incoming int      // and how many the multisnapshot has
	plan     cardPlan // for askReplace: the cards the multisnapshot's Nexts are given
	said     []string // the lines the dialogue says, for a test to read
	whyAt    []int    // which of them say why an answer was refused: drawn in the accent colour
}

// renameTarget is the path a card at old gets when the person types typed as its new name,
// or why it cannot have that name. ".img" is added when the name does not end in it, since
// the window lists the files that do.
func renameTarget(old, typed string) (target, why string) {
	return renameTargetExt(old, typed, ".img")
}

// renameTargetExt is renameTarget for files whose names end in ext: the Snapshots window names its
// files with it too.
func renameTargetExt(old, typed, ext string) (target, why string) {
	name := strings.TrimSpace(typed)
	switch {
	case name == "" || strings.EqualFold(name, ext):
		return "", "Type a name."
	case strings.ContainsAny(name, "/\\\x00"):
		return "", "A name has no slashes in it."
	case strings.ContainsAny(name, "<>:\"|?*"):
		return "", "A name has none of < > : \" | ? *"
	case strings.HasPrefix(name, "."):
		return "", "A name does not begin with a dot."
	case len(name) > 100:
		return "", "That name is too long."
	}
	if !strings.HasSuffix(strings.ToLower(name), strings.ToLower(ext)) {
		name += ext
	}
	if name == filepath.Base(old) {
		return "", "That is its name already."
	}
	target = filepath.Join(filepath.Dir(old), name)
	if st, err := os.Lstat(target); err == nil {
		// The same file under a name that differs in case (a folder that ignores case) is
		// a rename; any other file is in the way.
		if so, err := os.Stat(old); err != nil || !os.SameFile(so, st) {
			return "", "A file called " + name + " is there already."
		}
	}
	return target, ""
}

// renameFile renames a card image, never over another file.
func renameFile(old, target string) error {
	if st, err := os.Lstat(target); err == nil {
		if so, err := os.Stat(old); err != nil || !os.SameFile(so, st) {
			return fmt.Errorf("a file called %s is there already", filepath.Base(target))
		}
	}
	return os.Rename(old, target)
}

// plainError is an error without the path the message already names.
func plainError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	if le, ok := err.(*os.LinkError); ok {
		return le.Err.Error()
	}
	return err.Error()
}

// word is what the question is about.
func (q *cardQuestion) word() string {
	if q.noun == "" {
		return "card"
	}
	return q.noun
}

// extension is what a name ends in.
func (q *cardQuestion) extension() string {
	if q.ext == "" {
		return ".img"
	}
	return q.ext
}

// sizeWords is the size of the file, as the question says it.
func (q *cardQuestion) sizeWords() string {
	if q.sizeText != "" {
		return q.sizeText
	}
	return fmt.Sprintf("%d MB", q.size>>20)
}

// build makes the dialogue for the question as it stands.
func (q *cardQuestion) build() {
	name := filepath.Base(q.path)
	var fields []zenui.FormField
	row := 0
	q.said, q.whyAt = q.said[:0], q.whyAt[:0]
	say := func(s string) {
		for _, line := range zenui.WrapText(s, 44, "") {
			fields = append(fields, zenui.FormField{Label: line, Type: zenui.FieldLabel, Row: row})
			q.said = append(q.said, line)
			row++
		}
	}
	// sayWhy says why an answer was refused, in the lines whyAt marks. The labels come before
	// the text box, so a label's place in said is its place among the fields.
	sayWhy := func(s string) {
		first := len(q.said)
		say(s)
		for i := first; i < len(q.said); i++ {
			q.whyAt = append(q.whyAt, i)
		}
	}
	box := func() {
		fields = append(fields, zenui.FormField{Type: zenui.FieldText, Text: &q.text, Row: row, MinWidthCells: 36})
		row++
	}
	title, buttons := "", []string(nil)
	switch q.kind {
	case askRenameName:
		title, buttons = "RENAME "+strings.ToUpper(q.word()), []string{"Cancel", "Rename"}
		say("Rename " + name)
		say("Type the new name. " + q.extension() + " is added if it is left out.")
		if q.why != "" {
			sayWhy(q.why)
		}
		box()
	case askRenameSure:
		title, buttons = "RENAME "+strings.ToUpper(q.word()), []string{"No", "Yes"}
		say("Rename " + name + " to " + filepath.Base(q.target) + "?")
		say("What the " + q.word() + " holds is not touched.")
	case askDelete:
		title, buttons = "DELETE "+strings.ToUpper(q.word()), []string{"Cancel", "Delete"}
		say(fmt.Sprintf("Delete %s, %s?", name, q.sizeWords()))
		say("The file is deleted for good: it is not kept in a bin, and it cannot be got back.")
		if q.standard {
			say("It is the standard system card. Make bootable, and every Next that boots NextZXOS, use it.")
		}
		say("Type " + deleteWord + ", in capitals, and press Delete.")
		if q.why != "" {
			sayWhy(q.why)
		}
		box()
	case askReplace:
		title, buttons = "REPLACE THE MACHINES", []string{"Cancel", "Load"}
		running := "The machine that is running now is"
		if q.machines != 1 {
			running = fmt.Sprintf("The %d machines that are running now are", q.machines)
		}
		have := "one machine"
		if q.incoming != 1 {
			have = fmt.Sprintf("its %d machines", q.incoming)
		}
		say("Load " + name + "?")
		say(running + " going to be closed and replaced by " + have + ". Whatever they hold that is not in a snapshot is lost.")
		say("Type " + replaceWord + ", in capitals, and press Load.")
		if q.why != "" {
			sayWhy(q.why)
		}
		box()
	}
	q.modal = zenui.NewModal(zenui.ModalConfig{
		Form:    zenui.FormConfig{Title: title, Fields: fields},
		Buttons: buttons, CancelButtonIndex: 0, Scale: cardAskScale,
	})
}

// draw draws the question. The reason an answer was refused is the theme's accent colour
// (DirText, the colour of the headings), not the colour of what the dialogue informs: the form
// draws every label in one colour, so the reason is drawn again over its label.
func (q *cardQuestion) draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	q.modal.Draw(r, screenW, screenH, theme)
	lh := r.LineHeight(cardAskScale)
	for _, i := range q.whyAt {
		rec := q.modal.Form().FieldRect(i)
		r.FillRect(rec, theme.Panel)
		r.DrawText(fitHead(r, q.said[i], rec.W, cardAskScale), rec.X, rec.Y+(rec.H-lh)/2, cardAskScale, theme.DirText)
	}
}

// isStandard reports whether the card at path is the standard system card of the window's
// folder.
func (w *cardsWindow) isStandard(path string) bool {
	return sameCardFile(path, nextcard.In(w.dir).Card)
}

// startRename asks for card c's new name.
func (w *cardsWindow) startRename(c cardEntry) {
	stem := strings.TrimSuffix(filepath.Base(c.path), filepath.Ext(c.path))
	w.ask = &cardQuestion{kind: askRenameName, path: c.path, size: c.size, text: stem}
	w.ask.build()
}

// startDelete asks for the word that deletes card c.
func (w *cardsWindow) startDelete(c cardEntry) {
	w.ask = &cardQuestion{kind: askDelete, path: c.path, size: c.size, standard: w.isStandard(c.path)}
	w.ask.build()
}

// updateAsk runs the question for a frame. Enter answers it as the right-hand button does,
// Escape and the left-hand button drop it.
func (w *cardsWindow) updateAsk(in zenui.Input) cardsResult {
	q := w.ask
	status := q.modal.Update(in)
	enter := false
	for _, k := range in.Keys {
		if k == zenui.KeyEnter {
			enter = true
		}
	}
	switch {
	case status == zenui.Cancelled:
		w.ask = nil
		if q.kind == askDelete {
			w.note = fmt.Sprintf("Not deleted: %s is as it was.", filepath.Base(q.path))
		} else {
			w.note = fmt.Sprintf("Not renamed: %s is as it was.", filepath.Base(q.path))
		}
	case status == zenui.Accepted || enter:
		w.answer()
	}
	return cardsOpen
}

// answer takes the answer the person has given to the question.
func (w *cardsWindow) answer() {
	q := w.ask
	switch q.kind {
	case askRenameName:
		target, why := renameTarget(q.path, q.text)
		if why != "" {
			q.why = why
			q.build()
			return
		}
		q.kind, q.target, q.why = askRenameSure, target, ""
		q.build()
	case askRenameSure:
		w.ask = nil
		w.renameCard(q.path, q.target)
	case askDelete:
		if q.text != deleteWord {
			q.why = "That is not " + deleteWord + ": type it in capitals, exactly."
			q.build()
			return
		}
		w.ask = nil
		w.deleteCard(q.path)
	}
}

// renameCard renames the card at path to target, and keeps the selection on it.
func (w *cardsWindow) renameCard(path, target string) {
	if err := renameFile(path, target); err != nil {
		w.note = "Not renamed: " + plainError(err)
		return
	}
	for i, p := range w.extra {
		if same(p, path) {
			w.extra[i] = target
		}
	}
	w.refresh()
	w.selectPath(target)
	w.note = fmt.Sprintf("%s is now called %s.", filepath.Base(path), filepath.Base(target))
	sayf("Card renamed: %s to %s\n", path, target)
}

// deleteCard deletes the card at path, and puts the selection on the card that took its
// place in the list.
func (w *cardsWindow) deleteCard(path string) {
	at := w.list.Selected
	if err := os.Remove(path); err != nil {
		w.note = "Not deleted: " + plainError(err)
		return
	}
	for i := len(w.extra) - 1; i >= 0; i-- {
		if same(w.extra[i], path) {
			w.extra = append(w.extra[:i], w.extra[i+1:]...)
		}
	}
	w.refresh()
	w.list.Selected = min(max(at, 0), len(w.cards)-1)
	w.note = fmt.Sprintf("%s is deleted.", filepath.Base(path))
	sayf("Card deleted: %s\n", path)
}
