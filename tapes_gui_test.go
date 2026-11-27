//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// writeTape writes a tape file of the given bytes, and gives its absolute path.
func writeTape(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// aTapeWithAScreen is a loader and a screen: four blocks, the screen in the fourth.
func aTapeWithAScreen(fill byte) []byte {
	return concat(tap.EncodeProgram("LOADER", []byte{1, 2, 3}, 10), tap.EncodeCode("SCREEN$", aScreen(fill), 16384))
}

// aLongTape has many programs, so that its blocks do not all fit on show.
func aLongTape(programs int) []byte {
	var parts [][]byte
	for i := 0; i < programs; i++ {
		parts = append(parts, tap.EncodeProgram("P"+string(rune('A'+i)), []byte{byte(i)}, uint16(i+1)))
	}
	return concat(parts...)
}

// tapeMenuLabels is the Tape menu's rows in order, a separator as "-".
func tapeMenuLabels(b *appMenuBar, zx *ZenZX) []string {
	b.refreshTapeItems(zx)
	var got []string
	for _, it := range b.widget.ItemsFor(barTape) {
		if it.Separator {
			got = append(got, "-")
		} else {
			got = append(got, it.Label)
		}
	}
	return got
}

// tapeRecentRows are the labels of the Recent Tapes submenu.
func tapeRecentRows(t *testing.T, b *appMenuBar, zx *ZenZX) []string {
	t.Helper()
	b.refreshTapeItems(zx)
	for _, it := range b.widget.ItemsFor(barTape) {
		if it.Label == "Recent Tapes" {
			var rows []string
			for _, s := range it.SubItems {
				rows = append(rows, s.Label)
			}
			return rows
		}
	}
	t.Fatal("no Recent Tapes row in the Tape menu")
	return nil
}

// tapeRowAt is the place of the Tape menu's row called label.
func tapeRowAt(t *testing.T, b *appMenuBar, label string) int {
	t.Helper()
	for i, it := range b.widget.ItemsFor(barTape) {
		if it.Label == label {
			return i
		}
	}
	t.Fatalf("no %q row in the Tape menu", label)
	return -1
}

// hookTapes makes a tape that is loaded noted by the bar, as the program's start makes it.
func hookTapes(t *testing.T, b *appMenuBar, zx *ZenZX) {
	t.Helper()
	old := tapeInsertedHook
	tapeInsertedHook = func(path string) { b.noteRecentTape(zx, path) }
	t.Cleanup(func() { tapeInsertedHook = old })
}

func TestTapeMenuHasLoadTapeFirstThenRecentManageAndTheAutoloadCheckbox(t *testing.T) {
	homeAndCwd(t)
	b, zx := newTestBar(t, nil, "")
	got := tapeMenuLabels(b, zx)
	if len(got) < 7 || !reflect.DeepEqual(got[:6], []string{"Load", "Load Tape...", "Recent Tapes", "Manage Tapes...", "Autoload Tape", "-"}) {
		t.Fatalf("the Tape menu starts %v: want Load Tape..., Recent Tapes, Manage Tapes..., the Autoload Tape checkbox, a separator", got)
	}
	if got[6] != "Playback" {
		t.Errorf("after the Load rows comes %q, want the Playback section", got[6])
	}
	rows := tapeRows(b, zx)
	if rows["Autoload Tape"].Checked != &b.tapeAutoload {
		t.Error("the Autoload Tape row is not a checkbox on the bar's own setting")
	}
	if rows["Load Tape..."].Disabled || rows["Manage Tapes..."].Disabled {
		t.Error("Load Tape... and Manage Tapes... can be chosen with a machine")
	}
	if got := len(b.actions[barTape]); got != len(b.widget.ItemsFor(barTape)) {
		t.Errorf("%d actions for %d rows: they must line up", got, len(b.widget.ItemsFor(barTape)))
	}
}

func TestTapeMenuRowsThatNeedAMachineAreTheOnesThatActOnTheDeck(t *testing.T) {
	homeAndCwd(t)
	b, zx := newTestBar(t, nil, "")
	b.refreshTapeItems(zx)
	need := map[string]bool{}
	for i, it := range b.widget.ItemsFor(barTape) {
		if it.Title || it.Separator {
			continue
		}
		need[it.Label] = b.needsMachine(zenui.MenuBarSelection{BarIndex: barTape, ItemIndex: i})
	}
	for _, label := range []string{"Load Tape...", "Recent Tapes", "Play", "Stop", "Rewind", "Accurate", "Fast", "Tape Info"} {
		if !need[label] {
			t.Errorf("%q does not need a machine: it acts on the deck", label)
		}
	}
	for _, label := range []string{"Manage Tapes...", "Autoload Tape"} {
		if need[label] {
			t.Errorf("%q needs a machine: it lists files / sets a preference", label)
		}
	}
}

func TestAutoloadCheckboxSavesTheSetting(t *testing.T) {
	homeAndCwd(t)
	path := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, nil, path)
	before := b.tapeAutoload
	b.tapeAutoload = !before // what the menu does to the box
	b.tapeMenu.pick(zx, tapeRowAt(t, b, "Autoload Tape"), -1)
	res, err := settingsconfig.Load(path, defaultSettingsJSON, validThemeNames, validFontNames)
	if err != nil || !res.FromDisk {
		t.Fatalf("what the bar wrote was refused: %v", err)
	}
	if res.Settings.TapeAutoloadOn() == before {
		t.Errorf("the box is now %v, the saved setting is %v", b.tapeAutoload, res.Settings.TapeAutoloadOn())
	}
}

func TestLoadTapeRowOpensADialogInTheTapesFolder(t *testing.T) {
	homeAndCwd(t)
	tapes := withTapeDir(t)
	b, zx := newTestBar(t, nil, "")
	if !b.tapeMenu.pick(zx, tapeRowAt(t, b, "Load Tape..."), -1) {
		t.Fatal("picking Load Tape... ran nothing")
	}
	if b.diskDialog == nil || b.dialogKind != dialogLoadTape || b.fileDlg != fileDlgTape {
		t.Fatalf("the dialog is %v, kind %v, which %v", b.diskDialog, b.dialogKind, b.fileDlg)
	}
	if got := b.diskDialog.Dir(); got != tapes {
		t.Errorf("with no tape loaded the dialog starts in %q, want the tapes folder %q", got, tapes)
	}
	if st, err := os.Stat(tapes); err != nil || !st.IsDir() {
		t.Error("the tapes folder was not made")
	}
}

func TestLoadTapeDialogStartsWhereTheLastTapeWasAndLoadsWhatItIsGiven(t *testing.T) {
	homeAndCwd(t)
	withTapeDir(t)
	b, zx := newTestBar(t, nil, "")
	far := writeTape(t, t.TempDir(), "far.tap", aTapeWithAScreen(1))
	b.recentTapes = []string{far}
	b.openLoadTapeDialog()
	if got := b.diskDialog.Dir(); got != filepath.Dir(far) {
		t.Errorf("after a tape from %q the dialog starts in %q", filepath.Dir(far), got)
	}
	var loaded []string
	b.openFiles = func(files []string) { loaded = append(loaded, files...) }
	b.acceptDialog(zx, far)
	if !reflect.DeepEqual(loaded, []string{far}) {
		t.Errorf("accepting the dialog loaded %v, want %v: as a tape dropped on the window", loaded, far)
	}
}

func TestATapeThatIsLoadedIsAmongTheRecentOnesAndKept(t *testing.T) {
	homeAndCwd(t)
	dir := t.TempDir()
	path := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, nil, path)
	hookTapes(t, b, zx)
	if rows := tapeRecentRows(t, b, zx); !reflect.DeepEqual(rows, []string{"None"}) {
		t.Fatalf("before any tape: %v", rows)
	}
	a := writeTape(t, dir, "alpha.tap", aTapeWithAScreen(1))
	c := writeTape(t, dir, "gamma.tap", aTapeWithAScreen(2))
	for _, p := range []string{a, c} {
		if msg := zx.loadDroppedFile(p); !strings.Contains(msg, filepath.Base(p)) {
			t.Fatalf("loading %s said %q", p, msg)
		}
	}
	if rows := tapeRecentRows(t, b, zx); len(rows) != 2 || rows[0] != "gamma.tap" || rows[1] != "alpha.tap" {
		t.Fatalf("recent tapes: %v, want the last loaded first", rows)
	}
	zx.loadDroppedFile(a) // loaded again: first, and not twice
	b.tapeRecentAt = b.tapeRecentAt.Add(-2 * tapeRecentLook)
	if rows := tapeRecentRows(t, b, zx); len(rows) != 2 || rows[0] != "alpha.tap" {
		t.Errorf("after loading alpha again: %v", rows)
	}

	res, err := settingsconfig.Load(path, defaultSettingsJSON, validThemeNames, validFontNames)
	if err != nil || !res.FromDisk {
		t.Fatalf("what the bar wrote was refused: %v", err)
	}
	if got := res.Settings.RecentTapePaths(); !reflect.DeepEqual(got, []string{a, c}) {
		t.Errorf("the settings hold %v, want %v", got, []string{a, c})
	}
	b2, zx2 := newTestBar(t, res.Settings, path)
	if rows := tapeRecentRows(t, b2, zx2); len(rows) != 2 || rows[0] != "alpha.tap" {
		t.Errorf("after a restart: %v", rows)
	}

	// a tape that is gone is not listed, and one that is not a tape is not kept
	if err := os.Remove(c); err != nil {
		t.Fatal(err)
	}
	b.tapeRecentAt = b.tapeRecentAt.Add(-2 * tapeRecentLook)
	if rows := tapeRecentRows(t, b, zx); len(rows) != 1 || rows[0] != "alpha.tap" {
		t.Errorf("with one gone: %v", rows)
	}
	b.noteRecentTape(zx, filepath.Join(dir, "notes.txt"))
	if len(b.recentTapes) != 2 {
		t.Errorf("a file that is no tape was kept: %v", b.recentTapes)
	}
}

func TestRecentTapesMenuShowsTheFirstFewAndLeadsToTheRest(t *testing.T) {
	homeAndCwd(t)
	dir := t.TempDir()
	b, zx := newTestBar(t, nil, "")
	for i := 0; i < recentTapesMenu+4; i++ {
		b.noteRecentTape(zx, writeTape(t, dir, "t"+string(rune('a'+i))+".tap", aTapeWithAScreen(byte(i))))
	}
	rows := tapeRecentRows(t, b, zx)
	if len(rows) != recentTapesMenu+2 || rows[recentTapesMenu] != "-" && !strings.HasSuffix(rows[len(rows)-1], "Manage Tapes...") {
		t.Fatalf("%d rows: %v", len(rows), rows)
	}
	if want := "4 more: Manage Tapes..."; rows[len(rows)-1] != want {
		t.Errorf("the last row is %q, want %q", rows[len(rows)-1], want)
	}
	// choosing it opens the window; choosing a tape loads it
	var loaded []string
	b.openFiles = func(files []string) { loaded = append(loaded, files...) }
	at := tapeRowAt(t, b, "Recent Tapes")
	b.tapeMenu.pick(zx, at, 0)
	if len(loaded) != 1 || filepath.Base(loaded[0]) != "t"+string(rune('a'+recentTapesMenu+3))+".tap" {
		t.Errorf("the first recent row loaded %v, want the newest", loaded)
	}
	tapeDirBefore := tapeDir
	t.Cleanup(func() { tapeDir = tapeDirBefore })
	tapeDir = filepath.Join(t.TempDir(), "tapes")
	b.tapeMenu.pick(zx, at, recentTapesMenu+1)
	if b.tapes == nil {
		t.Fatal("the last row did not open Manage Tapes")
	}
	b.tapes.noGL = true
	b.tapes.close()
	b.tapes = nil
}

func TestManageTapesOpensTheWindowOnTheTapesFolder(t *testing.T) {
	homeAndCwd(t)
	tapes := withTapeDir(t)
	b, zx := newTestBar(t, nil, "")
	if b.tapes != nil {
		t.Fatal("the window is open already")
	}
	if !b.tapeMenu.pick(zx, tapeRowAt(t, b, "Manage Tapes..."), -1) {
		t.Fatal("picking Manage Tapes... ran nothing")
	}
	if b.tapes == nil || b.tapes.dir != tapeDir {
		t.Fatalf("the window is %+v, want one on %q", b.tapes, tapeDir)
	}
	if st, err := os.Stat(tapes); err != nil || !st.IsDir() {
		t.Error("the tapes folder was not made")
	}
	if !b.Active() {
		t.Error("the bar does not say a window is open")
	}
	b.tapes.noGL = true
	b.tapes.close()
	b.tapes = nil
}

// --- the window ------------------------------------------------------------------------------

// tapesOn is a window on a folder with the tapes it is given, with no textures made.
func tapesOn(t *testing.T, recent []string, files map[string][]byte) (*tapesWindow, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tapes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, data := range files {
		writeTape(t, dir, n, data)
	}
	w := newTapesWindow(dir, func() []string { return recent }, nil)
	w.noGL = true
	w.setButtons()
	return w, dir
}

// frame runs the window for a frame with the keys given.
func frameOf(w *tapesWindow, keys ...zenui.Key) tapesResult {
	return w.Update(zenui.Input{Keys: keys, DeltaTime: 0.016}, testNoopRenderer{}, 1280, 800)
}

func selectedTape(w *tapesWindow) string {
	f, ok := w.selected()
	if !ok {
		return ""
	}
	return f.Name
}

func TestTapesWindowListsRecentFirstThenTheFolderAndOpensAsAGrid(t *testing.T) {
	far := writeTape(t, t.TempDir(), "far.tzx", aTapeWithAScreen(9))
	w, _ := tapesOn(t, []string{far}, map[string][]byte{
		"b.tap": aTapeWithAScreen(2), "A.tap": aTapeWithAScreen(1), "notes.txt": []byte("x")})
	var names []string
	for _, f := range w.all {
		names = append(names, f.Name)
	}
	if want := []string{"far.tzx", "A.tap", "b.tap"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("the window lists %v, want %v", names, want)
	}
	if w.list.Columns != 3 || selectedTape(w) != "far.tzx" {
		t.Errorf("%d columns, %q chosen: want the grid and the first tape", w.list.Columns, selectedTape(w))
	}
	if got := tapeCaption(w.all[1]); got != "A TAP" {
		t.Errorf("the grid's caption is %q", got)
	}
	if row := tapeRow(w.all[2]); !strings.Contains(row, "b") || !strings.Contains(row, "TAP") || !strings.Contains(row, "KB") {
		t.Errorf("the row is %q", row)
	}
	w.do(tapesBtnLayout)
	w.setButtons()
	if w.list.Columns != 1 || w.buttons.Buttons[3].Label != "Grid" {
		t.Errorf("columns %d, the button says %q", w.list.Columns, w.buttons.Buttons[3].Label)
	}
	if r := w.do(tapesBtnClose); r != tapesClosed {
		t.Errorf("Close ended in %v", r)
	}
}

func TestTapesWindowWithNothingInIt(t *testing.T) {
	w, _ := tapesOn(t, nil, nil)
	if len(w.all) != 0 {
		t.Fatalf("%d tapes in an empty folder", len(w.all))
	}
	for _, b := range w.buttons.Buttons {
		if (b.ID == tapesBtnLoad || b.ID == tapesBtnRename || b.ID == tapesBtnDelete) && !b.Disabled {
			t.Errorf("%s can be chosen with no tape", b.ID)
		}
	}
	if r := frameOf(w); r != tapesOpen {
		t.Errorf("a frame ended in %v", r)
	}
	if r := w.do(tapesBtnLoad); r != tapesOpen {
		t.Errorf("Load with no tape ended in %v", r)
	}
	if r := frameOf(w, zenui.KeyEscape); r != tapesClosed {
		t.Errorf("Escape ended in %v", r)
	}
}

func TestTapesWindowShowsTheBlocksAndTheFirstScreenOfTheChosenTape(t *testing.T) {
	w, _ := tapesOn(t, nil, map[string][]byte{
		"a.tap": aTapeWithAScreen(1), "b.tap": concat(tap.EncodeProgram("ONLY", []byte{1}, 1))})
	frameOf(w)
	if selectedTape(w) != "a.tap" {
		t.Fatalf("%q is chosen", selectedTape(w))
	}
	lines := w.blocksOf()
	if len(lines) != 4 || lines[0].Name != "LOADER" || lines[2].Name != "SCREEN$" {
		t.Fatalf("the blocks of a.tap: %+v", lines)
	}
	e := w.entries[entryKey(w.all[0])]
	if e == nil || e.pic == nil || e.sum.ScreenAt != 4 {
		t.Fatalf("the first screen of a.tap: %+v", e)
	}
	if _, what := w.detail(w.all[0]); !strings.Contains(what, "4 blocks") || !strings.Contains(what, "block 4") {
		t.Errorf("the note for a.tap is %q", what)
	}
	// another tape gets the focus: its blocks are shown, and it has no screen
	frameOf(w, zenui.KeyRight)
	if selectedTape(w) != "b.tap" {
		t.Fatalf("%q is chosen after Right", selectedTape(w))
	}
	frameOf(w)
	if got := w.blocksOf(); len(got) != 2 || got[0].Name != "ONLY" {
		t.Errorf("the blocks of b.tap: %+v", got)
	}
	if e := w.entries[entryKey(w.all[1])]; e == nil || e.pic != nil {
		t.Errorf("a tape with no screen has a picture: %+v", e)
	}
	if _, what := w.detail(w.all[1]); !strings.Contains(what, "no screen") {
		t.Errorf("the note for b.tap is %q", what)
	}
}

func TestTapesWindowBlocksScrollWithTabAndStartAgainForAnotherTape(t *testing.T) {
	w, _ := tapesOn(t, nil, map[string][]byte{"a.tap": aLongTape(10), "b.tap": aLongTape(10)})
	frameOf(w)
	if n := len(w.blocksOf()); n != 20 || n <= w.blockLines {
		t.Fatalf("%d blocks, %d lines on show: want more blocks than lines", n, w.blockLines)
	}
	if w.blocksFocus || w.blockTop != 0 {
		t.Fatal("the window starts with the blocks scrolled or in focus")
	}
	frameOf(w, zenui.KeyDown) // the list's: the next tape
	if selectedTape(w) != "b.tap" {
		t.Fatalf("Down in the list left %q chosen", selectedTape(w))
	}
	frameOf(w, zenui.KeyUp)
	frameOf(w, zenui.KeyTab)
	if !w.blocksFocus {
		t.Fatal("Tab did not give the blocks the keys")
	}
	frameOf(w, zenui.KeyDown, zenui.KeyDown)
	if w.blockTop != 2 || selectedTape(w) != "a.tap" {
		t.Errorf("Down twice in the blocks: first block %d, %q chosen", w.blockTop, selectedTape(w))
	}
	frameOf(w, zenui.KeyEnd)
	if want := len(w.blocksOf()) - w.blockLines; w.blockTop != want {
		t.Errorf("End: first block %d, want %d", w.blockTop, want)
	}
	frameOf(w, zenui.KeyPageUp)
	frameOf(w, zenui.KeyHome)
	if w.blockTop != 0 {
		t.Errorf("Home: first block %d", w.blockTop)
	}
	frameOf(w, zenui.KeyEnd, zenui.KeyTab, zenui.KeyDown) // back to the list, to the next tape
	if selectedTape(w) != "b.tap" || w.blockTop != 0 {
		t.Errorf("another tape: %q, first block %d, want b.tap from block 1", selectedTape(w), w.blockTop)
	}
}

func TestTapesWindowLoadHandsBackTheChosenTape(t *testing.T) {
	w, dir := tapesOn(t, nil, map[string][]byte{"a.tap": aTapeWithAScreen(1), "b.tap": aTapeWithAScreen(2)})
	frameOf(w, zenui.KeyDown)
	if r := frameOf(w, zenui.KeyEnter); r != tapesLoad {
		t.Fatalf("Enter ended in %v, want Load", r)
	}
	if want, _ := filepath.Abs(filepath.Join(dir, "b.tap")); w.chosen != want {
		t.Errorf("chosen %q, want %q", w.chosen, want)
	}
	w.chosen = ""
	if r := w.do(tapesBtnLoad); r != tapesLoad || filepath.Base(w.chosen) != "b.tap" {
		t.Errorf("the Load button ended in %v with %q", r, w.chosen)
	}
}

func TestTapesWindowRenameKeepsTheExtensionAndTellsTheBar(t *testing.T) {
	w, dir := tapesOn(t, nil, map[string][]byte{"a.tap": aTapeWithAScreen(1)})
	var oldPath, newPath string
	w.renamed = func(o, n string) { oldPath, newPath = o, n }
	frameOf(w)
	w.do(tapesBtnRename)
	if w.ask == nil || w.ask.kind != askRenameName || w.ask.text != "a" {
		t.Fatalf("the question is %+v, want the name asked for, without the extension", w.ask)
	}
	w.ask.text = "jetpac"
	w.answer()
	if w.ask == nil || w.ask.kind != askRenameSure {
		t.Fatalf("the name was not confirmed: %+v", w.ask)
	}
	w.answer()
	if w.ask != nil {
		t.Fatal("the question is still open")
	}
	if _, err := os.Stat(filepath.Join(dir, "jetpac.tap")); err != nil {
		t.Errorf("the tape is not called jetpac.tap: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.tap")); err == nil {
		t.Error("the old name is still there")
	}
	if filepath.Base(oldPath) != "a.tap" || filepath.Base(newPath) != "jetpac.tap" {
		t.Errorf("the bar was told %q to %q", oldPath, newPath)
	}
	if selectedTape(w) != "jetpac.tap" {
		t.Errorf("%q is chosen after the rename", selectedTape(w))
	}
}

func TestTapesWindowDeleteAsksForTheWordAndOnlyThenDeletes(t *testing.T) {
	w, dir := tapesOn(t, nil, map[string][]byte{"a.tap": aTapeWithAScreen(1), "b.tap": aTapeWithAScreen(2)})
	frameOf(w)
	w.do(tapesBtnDelete)
	if w.ask == nil || w.ask.kind != askDelete {
		t.Fatalf("the question is %+v", w.ask)
	}
	w.ask.text = "delete"
	w.answer()
	if w.ask == nil || w.ask.why == "" {
		t.Fatal("a wrong word was accepted, or it was not said why not")
	}
	if _, err := os.Stat(filepath.Join(dir, "a.tap")); err != nil {
		t.Fatal("the tape went on a wrong word")
	}
	w.ask.text = deleteWord
	w.answer()
	if _, err := os.Stat(filepath.Join(dir, "a.tap")); err == nil {
		t.Error("the tape is still there after DELETE")
	}
	if len(w.all) != 1 || selectedTape(w) != "b.tap" {
		t.Errorf("%d tapes left, %q chosen: want b.tap alone", len(w.all), selectedTape(w))
	}
	// and a question that is cancelled leaves the tape as it was
	w.do(tapesBtnDelete)
	w.updateAsk(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
	if w.ask != nil || !strings.Contains(w.note, "Not deleted") {
		t.Errorf("after Escape: question %v, note %q", w.ask, w.note)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.tap")); err != nil {
		t.Error("the tape went when the question was cancelled")
	}
}

func TestTapesWindowTellsOfAFileThatCannotBeRead(t *testing.T) {
	w, _ := tapesOn(t, nil, map[string][]byte{"junk.tap": []byte("not a tape at all")})
	frameOf(w)
	e := w.entries[entryKey(w.all[0])]
	if e == nil || e.pic != nil {
		t.Fatalf("the entry of a file that is no tape: %+v", e)
	}
	if _, what := w.detail(w.all[0]); what == "" {
		t.Error("nothing is said of the file")
	}
	if r := frameOf(w); r != tapesOpen {
		t.Errorf("a frame ended in %v", r)
	}
}

func TestTapesWindowDrawsWithoutTextures(t *testing.T) {
	w, _ := tapesOn(t, nil, map[string][]byte{"a.tap": aTapeWithAScreen(1), "b.tap": aLongTape(10)})
	frameOf(w)
	w.Draw(testNoopRenderer{}, 1280, 800, zenui.DarkTheme())
	w.do(tapesBtnLayout)
	frameOf(w)
	w.Draw(testNoopRenderer{}, 1280, 800, zenui.DarkTheme())
	// a screen that is small still lays out
	w.Draw(testNoopRenderer{}, 320, 240, zenui.DarkTheme())
	if w.blockLines < 3 {
		t.Errorf("%d blocks on show in a small window", w.blockLines)
	}
}
