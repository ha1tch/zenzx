//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/zenui"
)

// windowOn makes a Snapshots window over a folder of the files given, each holding its own name.
func windowOn(t *testing.T, names ...string) (*snapshotsWindow, string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("contents of "+n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := newSnapshotsWindow(dir)
	w.setButtons()
	return w, dir
}

func selectedName(w *snapshotsWindow) string {
	f, ok := w.selected()
	if !ok {
		return ""
	}
	return f.Name
}

// type answers the window's question with text, and presses the right-hand button.
func answer(w *snapshotsWindow, text string) {
	w.ask.text = text
	w.answer()
}

// The window lists the folder's snapshots, newest first, with the newest chosen, and Load hands the
// chosen one to the bar.
func TestSnapshotsWindowListsAndLoads(t *testing.T) {
	w, dir := windowOn(t, "snapshot_20260309_140507.zxs", "snapshot_20260310_090000.sna", "quicksave.zxs", "notes.txt")
	old := time.Date(2026, 1, 1, 8, 0, 0, 0, time.Local)
	if err := os.Chtimes(filepath.Join(dir, "quicksave.zxs"), old, old); err != nil {
		t.Fatal(err)
	}
	w.list.Selected = -1 // as if it had been opened after
	w.refresh()
	if len(w.all) != 3 {
		t.Fatalf("%d snapshots listed", len(w.all))
	}
	if got := selectedName(w); got != "snapshot_20260310_090000.sna" {
		t.Errorf("the chosen one is %q, want the newest", got)
	}
	w.list.Selected = 1
	if r := w.do(snapBtnLoad); r != snapshotsLoad || w.loadPath != filepath.Join(dir, selectedName(w)) {
		t.Errorf("Load ended in %v with %q", r, w.loadPath)
	}
	if r := w.do(snapBtnClose); r != snapshotsClosed {
		t.Errorf("Close ended in %v", r)
	}
	// each row says its name, size and date
	row := snapshotListRow(w.all[0])
	if !strings.Contains(row, "snapshot_20260310_090000.sna") || !strings.Contains(row, "2026-03-10 09:00") || !strings.Contains(row, " B") {
		t.Errorf("the row is %q", row)
	}
	if got := snapshotKind(w.all[0]); !strings.Contains(got, "Load Timestamped lists it") {
		t.Errorf("a timestamped snapshot: %q", got)
	}
	for _, f := range w.all {
		if f.Quick && !strings.Contains(snapshotKind(f), "Quick Load") {
			t.Errorf("the quick save: %q", snapshotKind(f))
		}
	}
}

// With nothing in the folder, no button but Close works.
func TestSnapshotsWindowWithNoSnapshots(t *testing.T) {
	w, _ := windowOn(t, "notes.txt")
	if len(w.all) != 0 {
		t.Fatal("a file that is not a snapshot is listed")
	}
	for _, b := range w.buttons.Buttons {
		if b.ID != snapBtnClose && !b.Disabled {
			t.Errorf("%s can be pressed with nothing chosen", b.ID)
		}
	}
	for _, id := range []string{snapBtnLoad, snapBtnDuplicate, snapBtnRename, snapBtnDelete} {
		if r := w.do(id); r != snapshotsOpen || w.ask != nil || w.loadPath != "" {
			t.Errorf("%s did something with nothing chosen", id)
		}
	}
}

// Duplicate makes a copy beside the snapshot, named so that nothing is written over, and chooses it.
func TestSnapshotsWindowDuplicates(t *testing.T) {
	w, dir := windowOn(t, "quicksave.zxs", "quicksave-2.zxs")
	w.selectPath(filepath.Join(dir, "quicksave.zxs"))
	w.do(snapBtnDuplicate)
	if got := selectedName(w); got != "quicksave-3.zxs" {
		t.Fatalf("the copy is %q (note %q)", got, w.note)
	}
	for _, n := range []string{"quicksave.zxs", "quicksave-2.zxs", "quicksave-3.zxs"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		want := "contents of quicksave.zxs"
		if n == "quicksave-2.zxs" {
			want = "contents of quicksave-2.zxs"
		}
		if err != nil || string(b) != want {
			t.Errorf("%s holds %q (%v), want %q", n, b, err, want)
		}
	}
	// a copy of a copy is numbered from the original's name
	w.do(snapBtnDuplicate)
	if got := selectedName(w); got != "quicksave-4.zxs" {
		t.Errorf("the copy of a copy is %q", got)
	}
	// a timestamped snapshot's copy is not one Save Timestamped made
	w2, dir2 := windowOn(t, "snapshot_20260309_140507.sna")
	w2.do(snapBtnDuplicate)
	if got := selectedName(w2); got != "snapshot_20260309_140507-2.sna" {
		t.Fatalf("the copy is %q", got)
	}
	if f, _ := w2.selected(); f.Timestamped {
		t.Error("a copy is listed by Load Timestamped")
	}
	if stamped := timestampedSnapshots(listSnapshots(dir2)); len(stamped) != 1 {
		t.Errorf("%d timestamped snapshots after a copy, want the 1", len(stamped))
	}
}

// Rename asks for a name, and then whether to go on; the name is checked before the question; the
// extension stays; nothing is written over.
func TestSnapshotsWindowRenames(t *testing.T) {
	w, dir := windowOn(t, "snapshot_20260309_140507.zxs", "taken.zxs", "other.sna")
	w.selectPath(filepath.Join(dir, "snapshot_20260309_140507.zxs"))
	w.do(snapBtnRename)
	if w.ask == nil || w.ask.kind != askRenameName || w.ask.text != "snapshot_20260309_140507" {
		t.Fatalf("the question is %+v", w.ask)
	}
	if got := strings.Join(w.ask.said, " "); !strings.Contains(got, "Type the new name. .zxs is added if it is left out") {
		t.Errorf("the question says %q", got)
	}
	for text, why := range map[string]string{
		"":          "Type a name.",
		"a/b":       "no slashes",
		".hidden":   "dot",
		"taken":     "is there already",
		"taken.zxs": "is there already",
		"new.sna":   "stays .zxs",
		"new.z80":   "stays .zxs",
		"what?":     "none of",
	} {
		answer(w, text)
		if w.ask == nil || w.ask.kind != askRenameName || !strings.Contains(w.ask.why, why) {
			t.Errorf("%q: asking %+v, why %q, want it to say %q", text, w.ask, w.ask.why, why)
		}
	}
	// a name that can be had is asked about, and the file is still where it was
	answer(w, "My game")
	if w.ask == nil || w.ask.kind != askRenameSure || filepath.Base(w.ask.target) != "My game.zxs" {
		t.Fatalf("the question is %+v", w.ask)
	}
	if !exists(filepath.Join(dir, "snapshot_20260309_140507.zxs")) {
		t.Fatal("the snapshot was renamed before the question was answered")
	}
	w.answer()
	if w.ask != nil || exists(filepath.Join(dir, "snapshot_20260309_140507.zxs")) || !exists(filepath.Join(dir, "My game.zxs")) {
		t.Fatalf("the snapshot was not renamed (note %q)", w.note)
	}
	if got := selectedName(w); got != "My game.zxs" {
		t.Errorf("the chosen one is %q", got)
	}
	if f, _ := w.selected(); f.Timestamped || f.Format != "zxs" {
		t.Errorf("a renamed snapshot is %+v", f)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "My game.zxs")); string(b) != "contents of snapshot_20260309_140507.zxs" {
		t.Errorf("the renamed snapshot holds %q", b)
	}
	// the extension typed, in either case, is not doubled
	w.do(snapBtnRename)
	answer(w, "Game.ZXS")
	w.answer()
	if !exists(filepath.Join(dir, "Game.ZXS")) || exists(filepath.Join(dir, "Game.ZXS.zxs")) {
		t.Errorf("the extension was doubled: %v", listNames(dir))
	}
	// declining leaves it as it was
	w.do(snapBtnRename)
	answer(w, "elsewhere")
	w.ask.modal.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
	w.updateAsk(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
	if w.ask != nil || !exists(filepath.Join(dir, "Game.ZXS")) || exists(filepath.Join(dir, "elsewhere.ZXS")) {
		t.Errorf("a refused rename changed things: %v (note %q)", listNames(dir), w.note)
	}
}

func listNames(dir string) []string {
	var out []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// Delete asks for the word DELETE, typed exactly; anything else keeps the file.
func TestSnapshotsWindowDeletes(t *testing.T) {
	w, dir := windowOn(t, "a.zxs", "b.zxs", "c.zxs")
	w.selectPath(filepath.Join(dir, "b.zxs"))
	at := w.list.Selected
	w.do(snapBtnDelete)
	if w.ask == nil || w.ask.kind != askDelete {
		t.Fatalf("the question is %+v", w.ask)
	}
	said := strings.Join(w.ask.said, "\n")
	if !strings.Contains(said, "Delete b.zxs, 17 B?") || strings.Contains(said, "MB") || !strings.Contains(said, "DELETE") {
		t.Errorf("the question says:\n%s", said)
	}
	for _, text := range []string{"", "delete", "Delete", "DELETE ", " DELETE", "DELET", "yes"} {
		answer(w, text)
		if w.ask == nil || w.ask.kind != askDelete || !strings.Contains(w.ask.why, "not DELETE") || !exists(filepath.Join(dir, "b.zxs")) {
			t.Fatalf("%q: asking %+v", text, w.ask)
		}
	}
	answer(w, deleteWord)
	if w.ask != nil || exists(filepath.Join(dir, "b.zxs")) {
		t.Fatalf("the snapshot was not deleted (note %q)", w.note)
	}
	if len(w.all) != 2 || w.list.Selected != at || selectedName(w) != "a.zxs" && selectedName(w) != "c.zxs" {
		t.Errorf("%d listed, chosen %d (%q)", len(w.all), w.list.Selected, selectedName(w))
	}
	// the last one deleted leaves the window empty, and nothing chosen
	for _, n := range []string{"a.zxs", "c.zxs"} {
		w.selectPath(filepath.Join(dir, n))
		w.do(snapBtnDelete)
		answer(w, deleteWord)
	}
	if len(w.all) != 0 || w.list.Selected != -1 {
		t.Errorf("%d listed, chosen %d", len(w.all), w.list.Selected)
	}
}

// Escape drops a question and leaves the file; it does not close the window.
func TestSnapshotsWindowEscapeDropsTheQuestion(t *testing.T) {
	w, dir := windowOn(t, "a.zxs")
	w.do(snapBtnDelete)
	w.ask.text = deleteWord
	w.updateAsk(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
	if w.ask != nil || !exists(filepath.Join(dir, "a.zxs")) || !strings.Contains(w.note, "Not deleted") {
		t.Errorf("Escape: asking %+v, note %q", w.ask, w.note)
	}
}

// A card's questions are as they were: the questions the window shares with the SD Cards window.
func TestCardQuestionsAreUnchanged(t *testing.T) {
	q := &cardQuestion{kind: askDelete, path: "x/card.img", size: 280 << 20, standard: true}
	q.build()
	said := strings.Join(q.said, "\n")
	for _, want := range []string{"Delete card.img, 280 MB?", "standard system card"} {
		if !strings.Contains(said, want) {
			t.Errorf("the delete question lacks %q:\n%s", want, said)
		}
	}
	q = &cardQuestion{kind: askRenameName, path: "x/card.img", text: "card"}
	q.build()
	if said := strings.Join(q.said, " "); !strings.Contains(said, ".img is added if it is left out") {
		t.Errorf("the rename question: %s", said)
	}
	if target, why := renameTarget("x/card.img", "new"); why != "" || target != filepath.Join("x", "new.img") {
		t.Errorf("renameTarget: %q %q", target, why)
	}
}

// The window fits any screen the program runs in, and draws without a snapshot, with one, and over
// a question.
func TestSnapshotsWindowFitsTheScreen(t *testing.T) {
	for _, size := range [][2]int{{640, 480}, {800, 600}, {1920, 1080}} {
		w, dir := windowOn(t)
		for i := 0; i < 14; i++ {
			touch(t, dir, timestampedName(time.Date(2026, 3, 9, 10, i, 0, 0, time.Local), "zxs"))
		}
		w.refresh()
		r := &paintRenderer{}
		theme := zenui.LoadTheme(zenui.ThemeDark)
		w.Update(zenui.Input{}, r, size[0], size[1])
		w.Draw(r, size[0], size[1], theme)
		p := w.panel
		if p.X < 0 || p.Y < 0 || p.X+p.W > size[0] || p.Y+p.H > size[1] {
			t.Errorf("%v: the panel %+v is off the screen", size, p)
		}
		if w.listArea.H <= 0 || w.barArea.Y+w.barArea.H > p.Y+p.H {
			t.Errorf("%v: the list %+v and buttons %+v do not fit in %+v", size, w.listArea, w.barArea, p)
		}
		w.do(snapBtnDelete)
		w.Draw(r, size[0], size[1], theme)
		w.ask = nil
		w.do(snapBtnRename)
		w.Draw(r, size[0], size[1], theme)
	}
}
