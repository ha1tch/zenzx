//go:build !headless

package zenzx

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

func newTestBar(t *testing.T, s *settingsconfig.Settings, path string) (*appMenuBar, *ZenZX) {
	t.Helper()
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", s, path)
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	t.Cleanup(b.text.Unload)
	return b, zx
}

func fileRows(b *appMenuBar) []string {
	var got []string
	for _, it := range b.widget.ItemsFor(barFile) {
		if it.Separator {
			got = append(got, "-")
		} else {
			got = append(got, it.Label)
		}
	}
	return got
}

func TestFileMenuIsFirstOnTheBarWithOpenPreferencesAndQuit(t *testing.T) {
	b, _ := newTestBar(t, nil, "")
	if barFile != 0 {
		t.Errorf("barFile = %d: File is the first menu of the bar", barFile)
	}
	want := []string{"Open...", "-", "Preferences...", "-", "Quit"}
	if got := fileRows(b); !reflect.DeepEqual(got, want) {
		t.Errorf("the File menu is %v, want %v", got, want)
	}
	if got := len(b.actions[barFile]); got != len(want) {
		t.Errorf("%d actions for %d rows: they must line up", got, len(want))
	}
}

func TestFileQuitAsksTheHostToEndTheProgram(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	quit := 0
	b.quit = func() { quit++ }
	b.actions[barFile][4](zx)
	if quit != 1 {
		t.Errorf("Quit called the host's quit %d times, want once", quit)
	}
	b.quit = nil
	b.actions[barFile][4](zx) // a bar with no host must not panic
}

func TestFileOpenBrowsesForAnyLoadableFileAndRoutesItLikeADrop(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	var got [][]string
	b.openFiles = func(files []string) { got = append(got, files) }

	b.actions[barFile][0](zx)
	if b.diskDialog == nil || b.dialogKind != dialogOpenFile {
		t.Fatalf("Open... opened dialog=%v kind=%v", b.diskDialog, b.dialogKind)
	}
	if !b.modalOpen() {
		t.Error("the browser does not count as an open modal")
	}
	// Every kind of file a drop loads can be chosen.
	for _, ext := range []string{"tap", "tzx", "sna", "z80", "szx", "zxs", "scr", "dsk", "nex"} {
		found := false
		for _, e := range fileOpenExts {
			found = found || e == ext
		}
		if !found {
			t.Errorf("File > Open does not offer .%s", ext)
		}
		if !machineLoadExts["."+ext] {
			t.Errorf(".%s is offered but a drop would not load it", ext)
		}
	}

	b.acceptDialog(zx, "/games/jetpac.tap")
	if len(got) != 1 || !reflect.DeepEqual(got[0], []string{"/games/jetpac.tap"}) {
		t.Errorf("openFiles got %v, want the chosen file", got)
	}
	if b.diskDialog != nil || b.dialogKind != dialogDisk {
		t.Error("the browser is still open, or still of the Open kind, after a file was chosen")
	}
}

func TestFloppyDialogStillLoadsADiskNotARoutedFile(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	routed := 0
	b.openFiles = func([]string) { routed++ }
	zx.io.hasFDC = false
	b.openDiskDialog(zx) // no controller: no dialog
	if b.diskDialog != nil {
		t.Fatal("Open DSK Image opened on a machine with no drive")
	}
	b.dialogKind = dialogDisk
	b.acceptDialog(zx, "/nonexistent/x.dsk")
	if routed != 0 {
		t.Error("the Floppy Disk menu's browser was routed through File > Open")
	}
}

func TestHelpMenuNoLongerCarriesThePreferences(t *testing.T) {
	b, _ := newTestBar(t, nil, "")
	b.openLogoMenu(800)
	for _, it := range b.logoMenu.Items() {
		if it.Label == "Fixed menu bar" || it.Label == "Settings..." || it.Label == "Preferences" {
			t.Errorf("the Help menu still has %q, which is File > Preferences now", it.Label)
		}
	}
}

func prefRowNamed(t *testing.T, b *appMenuBar, zx *ZenZX, label string) prefRow {
	t.Helper()
	for _, s := range b.prefSections(zx) {
		for _, r := range s.rows {
			if r.label == label {
				return r
			}
		}
	}
	t.Fatalf("Preferences has no %q row", label)
	return prefRow{}
}

func TestPreferencesRowsReadAndChangeWhatTheMenusDo(t *testing.T) {
	b, zx := newTestBar(t, nil, "")

	// Every setting the bar keeps is a row.
	for _, label := range []string{"Theme", "Font", "Font zoom", "Display zoom", "Fixed menu bar",
		"Machine dock", "Several machines", "A plain drop", "With Shift held"} {
		prefRowNamed(t, b, zx, label)
	}

	theme := prefRowNamed(t, b, zx, "Theme")
	theme.set(1)
	if b.themeName != zenui.Themes[1] || theme.get() != 1 {
		t.Errorf("theme row set 1: bar has %v, row reads %d", b.themeName, theme.get())
	}
	if !*b.themeChecked[zenui.Themes[1]] || *b.themeChecked[zenui.Themes[0]] {
		t.Error("the Theme menu's checkmarks did not follow the Preferences row")
	}

	zoom := prefRowNamed(t, b, zx, "Font zoom")
	zoom.set(2)
	if b.scale != zoomLevels[2] {
		t.Errorf("font zoom row set 2: scale %d, want %d", b.scale, zoomLevels[2])
	}

	fixed := prefRowNamed(t, b, zx, "Fixed menu bar")
	fixed.set(1)
	if !b.fixed || fixed.get() != 1 {
		t.Error("the fixed bar row did not set b.fixed")
	}

	// The dock and several machines start on, and are the bar's to say.
	dock, multi := prefRowNamed(t, b, zx, "Machine dock"), prefRowNamed(t, b, zx, "Several machines")
	if dock.get() != 1 || multi.get() != 1 {
		t.Errorf("dock=%d multi=%d: both start on", dock.get(), multi.get())
	}
	dock.set(0)
	multi.set(0)
	if b.dockOn || b.multiOn {
		t.Error("turning the rows off left the dock or multi on")
	}

	plain, shift := prefRowNamed(t, b, zx, "A plain drop"), prefRowNamed(t, b, zx, "With Shift held")
	if dropAction(plain.get()) != dropAsk || dropAction(shift.get()) != dropRun {
		t.Errorf("drop defaults are %v/%v, want ask/run", dropAction(plain.get()), dropAction(shift.get()))
	}
	plain.set(int(dropCopy))
	shift.set(int(dropAsk))
	if b.dropPlain != dropCopy || b.dropShift != dropAsk {
		t.Errorf("drop rows set copy/ask: bar has %v/%v", b.dropPlain, b.dropShift)
	}
}

func TestPreferencesAreSavedAndComeBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, nil, path)
	b.dockOn, b.multiOn = false, false
	b.dropPlain, b.dropShift = dropCopy, dropAsk
	b.fixed = true
	b.saveSettings(zx)

	res, err := settingsconfig.Load(path, defaultSettingsJSON, validThemeNames, validFontNames)
	if err != nil {
		t.Fatal(err)
	}
	if !res.FromDisk {
		t.Fatalf("what the bar wrote was refused: %s", res.Warning)
	}
	b2, _ := newTestBar(t, res.Settings, path)
	if b2.dockOn || b2.multiOn || b2.dropPlain != dropCopy || b2.dropShift != dropAsk || !b2.fixed {
		t.Errorf("a new bar read dock=%v multi=%v drop=%v/%v fixed=%v back", b2.dockOn, b2.multiOn, b2.dropPlain, b2.dropShift, b2.fixed)
	}
	if b.savedFixed != b.fixed {
		t.Error("saving settings did not record that the file holds the fixed choice")
	}
}

func TestAnOldSettingsFileLeavesTheDockAndMultiOn(t *testing.T) {
	b, _ := newTestBar(t, &settingsconfig.Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 2, DisplayScale: 2}, "")
	if !b.dockOn || !b.multiOn || b.dropPlain != dropAsk || b.dropShift != dropRun {
		t.Errorf("dock=%v multi=%v drop=%v/%v: a file with no such keys means the old behaviour", b.dockOn, b.multiOn, b.dropPlain, b.dropShift)
	}
}

func TestPreferencesWindowDrivesTheRows(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	saved := 0
	w := newPrefsWindow(b.prefSections(zx), "", func() { saved++ })
	r := testNoopRenderer{}
	w.layout(r, 1000, 800)

	// A click on the Machine dock checkbox, on the Machines tab, turns the dock off, and saves.
	w.tabs.Selected = w.tabIndex("Machines")
	var si, ri int
	for s, sec := range w.sections {
		for i, row := range sec.rows {
			if row.label == "Machine dock" {
				si, ri = s, i
			}
		}
	}
	a := w.areas[si][ri]
	click := zenui.Input{MouseX: a.X + 3, MouseY: a.Y + a.H/2, MouseDown: true, MousePressed: true}
	if res := w.Update(click, r, 1000, 800); res != prefsOpen {
		t.Fatalf("a click closed the window")
	}
	if b.dockOn || saved != 1 {
		t.Errorf("after the click dock=%v saved=%d, want off and one save", b.dockOn, saved)
	}

	// A click on the second Theme option, on the Appearance tab, chooses it.
	w.tabs.Selected = w.tabIndex("Appearance")
	for s, sec := range w.sections {
		for i, row := range sec.rows {
			if row.label == "Theme" {
				si, ri = s, i
			}
		}
	}
	rects, _ := w.radios[si][ri].Layout(w.areas[si][ri], w.lineH, func(s string) int { return r.TextWidth(s, w.scale) })
	opt := rects[1]
	w.Update(zenui.Input{MouseX: opt.X + 2, MouseY: opt.Y + opt.H/2, MouseDown: true, MousePressed: true}, r, 1000, 800)
	if b.themeName != zenui.Themes[1] || saved != 2 {
		t.Errorf("after the click theme=%v saved=%d", b.themeName, saved)
	}

	// Escape closes it, as does Close.
	if res := w.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}}, r, 1000, 800); res != prefsClosed {
		t.Error("Escape did not close the window")
	}
}

func TestPreferencesWindowFitsALittleScreen(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	w := newPrefsWindow(b.prefSections(zx), "/some/where/settings.json", func() {})
	r := testNoopRenderer{}
	for _, size := range [][2]int{{1000, 800}, {800, 600}, {640, 480}} {
		w.layout(r, size[0], size[1])
		if w.panel.W > size[0] {
			t.Errorf("%dx%d: the panel is %d wide", size[0], size[1], w.panel.W)
		}
		if w.panel.H > size[1] {
			t.Errorf("%dx%d: the panel is %d tall", size[0], size[1], w.panel.H)
		}
		for si := range w.sections {
			for ri := range w.sections[si].rows {
				a := w.areas[si][ri]
				if a.X < w.panel.X || a.X+a.W > w.panel.X+w.panel.W || a.Y < w.panel.Y || a.Y+a.H > w.panel.Y+w.panel.H {
					t.Errorf("%dx%d: row %q is outside the panel: %+v in %+v", size[0], size[1], w.sections[si].rows[ri].label, a, w.panel)
				}
			}
		}
		if w.barArea.Y+w.barArea.H > w.panel.Y+w.panel.H {
			t.Errorf("%dx%d: the Close button is below the panel", size[0], size[1])
		}
	}
	// Opening it through the menu makes it the one surface.
	b.actions[barFile][2](zx)
	if b.prefs == nil || !b.modalOpen() {
		t.Error("File > Preferences... did not open the window")
	}
	b.widget.Close()
	b.enforceSingleSurface()
	if b.widget.Active() || b.logoMenu != nil {
		t.Error("a menu stayed open under the Preferences window")
	}
	if !strings.Contains(strings.Join(fileRows(b), "|"), "Preferences...") {
		t.Error("the File menu lost its Preferences row")
	}
}

func TestDockAndMultiAreOnUnlessPreferencesSayOtherwise(t *testing.T) {
	d := &machineDeck{}
	if !d.dockAllowed() || !d.multiAllowed() {
		t.Error("a deck with no preferences wired must allow both")
	}
	off := false
	d = &machineDeck{dockOn: func() bool { return off }, multiOn: func() bool { return off }}
	d.show(true)
	if d.state != barHidden {
		t.Error("the dock slid in while it is off in Preferences")
	}
	off = true
	d.show(true)
	if d.state == barHidden {
		t.Error("the dock did not show once allowed")
	}
}

func TestPlanDropAwayFromANextMenuLeavesFilesToTheUsualRoute(t *testing.T) {
	files := []string{"/x/g.tap", "/x/d.nex", "/x/p.scr"}
	for _, c := range []dropContext{
		{atPrompt: false, hasSlots: true, plain: dropAsk, shift: dropRun},
		{atPrompt: true, hasSlots: false, plain: dropAsk, shift: dropRun},
	} {
		p := planDrop(files, c)
		if !reflect.DeepEqual(p.load, files) || p.ask != nil || p.copyToCard != nil {
			t.Errorf("%+v: plan %+v, want everything left to the usual route", c, p)
		}
	}
	// A drop of nothing the Next routing knows is left alone even at the prompt.
	p := planDrop([]string{"/x/readme.txt"}, dropContext{atPrompt: true, hasSlots: true})
	if len(p.load) != 1 || p.copyToCard != nil {
		t.Errorf("a plain file at the prompt: %+v", p)
	}
}

func TestPlanDropAtANextMenu(t *testing.T) {
	tap, scr, nex, dsk := "/x/g.tap", "/x/p.scr", "/x/d.nex", "/x/d.dsk"
	at := func(plain, shift dropAction, shiftDown bool) dropContext {
		return dropContext{atPrompt: true, hasSlots: true, plain: plain, shift: shift, shiftDown: shiftDown}
	}
	cases := []struct {
		name  string
		files []string
		ctx   dropContext
		copy  []string
		ask   []string
		load  []string
	}{
		{"plain drop asks", []string{tap, scr}, at(dropAsk, dropRun, false), nil, []string{tap, scr}, nil},
		{"Shift runs", []string{tap, scr}, at(dropAsk, dropRun, true), nil, nil, []string{tap, scr}},
		{"plain drop set to copy", []string{tap}, at(dropCopy, dropRun, false), []string{tap}, nil, nil},
		{"plain drop set to run", []string{tap}, at(dropRun, dropAsk, false), nil, nil, []string{tap}},
		{"Shift set to ask", []string{tap}, at(dropRun, dropAsk, true), nil, []string{tap}, nil},
		{"Shift set to copy", []string{tap}, at(dropAsk, dropCopy, true), []string{tap}, nil, nil},
		// A .nex is run from the menus, so it does what a tape does.
		{".nex plain", []string{nex}, at(dropAsk, dropRun, false), nil, []string{nex}, nil},
		{".nex with Shift", []string{nex}, at(dropAsk, dropRun, true), nil, nil, []string{nex}},
		{".nex set to copy", []string{nex}, at(dropCopy, dropRun, false), []string{nex}, nil, nil},
		{".nex when plain runs", []string{nex}, at(dropRun, dropRun, false), nil, nil, []string{nex}},
		// A .dsk has no run on a Next: copied whatever is chosen, Shift or not.
		{".dsk plain", []string{dsk}, at(dropAsk, dropRun, false), []string{dsk}, nil, nil},
		{".dsk with Shift", []string{dsk}, at(dropAsk, dropRun, true), []string{dsk}, nil, nil},
		// Mixed: each file does what it can.
		{"tape and .nex, plain", []string{tap, nex}, at(dropAsk, dropRun, false), nil, []string{tap, nex}, nil},
		{"tape and .dsk, Shift", []string{tap, dsk}, at(dropAsk, dropRun, true), []string{dsk}, nil, []string{tap}},
		{"a plain file goes to the card", []string{tap, "/x/notes.txt"}, at(dropAsk, dropRun, true), []string{"/x/notes.txt"}, nil, []string{tap}},
	}
	for _, c := range cases {
		p := planDrop(c.files, c.ctx)
		if !reflect.DeepEqual(p.copyToCard, c.copy) || !reflect.DeepEqual(p.ask, c.ask) || !reflect.DeepEqual(p.load, c.load) {
			t.Errorf("%s: copy=%v ask=%v load=%v, want copy=%v ask=%v load=%v", c.name, p.copyToCard, p.ask, p.load, c.copy, c.ask, c.load)
		}
	}
}

func TestDropChoiceRemembersWhereTheFilesWereLetGo(t *testing.T) {
	d := &dropChoice{}
	d.open([]string{"/x/a.tap"}, 321)
	if d.x != 321 {
		t.Errorf("x = %d, want the drop's 321", d.x)
	}
	d.takeFiles()
	if d.x != 321 {
		t.Error("taking the files forgot where they were dropped")
	}
}

// The question of whether a busy Next may be reset to run a file reuses the drop dialog:
// the left button says no, the right one yes, and it comes back to the ordinary question
// the next time it is opened for a drop.
func TestDropChoiceResetQuestion(t *testing.T) {
	d := &dropChoice{}
	d.openReset("/x/game.nex")
	if !d.Active() || !d.reset || len(d.files) != 1 {
		t.Fatalf("openReset left %+v", d)
	}
	d.layout(testNoopRenderer{}, 800, 600)
	if got := d.Update(zenui.Input{MouseX: d.copyBtn.X + 2, MouseY: d.copyBtn.Y + 2, MousePressed: true}); got != dropChoiceCancel {
		t.Errorf("the left button of the reset question gave %v, want cancel", got)
	}
	d.openReset("/x/game.nex")
	d.layout(testNoopRenderer{}, 800, 600)
	if got := d.Update(zenui.Input{MouseX: d.runBtn.X + 2, MouseY: d.runBtn.Y + 2, MousePressed: true}); got != dropChoiceRun || !d.reset {
		t.Errorf("the right button gave %v (reset %v), want run with the question still known", got, d.reset)
	}
	d.open([]string{"/x/a.tap"}, 10)
	if d.reset {
		t.Error("a drop opened the reset question")
	}
	var rec = newTestDrawRecorder()
	d.openReset("/x/game.nex")
	d.Draw(rec, 800, 600, zenui.LoadTheme(zenui.ThemeDark))
	if len(*rec.calls) == 0 {
		t.Error("the reset question drew nothing")
	}
}
