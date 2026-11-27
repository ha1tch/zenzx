//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// homeAndCwd points the home folder and the working directory at folders of their own, so a test can
// tell which one a dialog starts in.
func homeAndCwd(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Chdir(t.TempDir())
	return home
}

func writeTestFile(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func keyEnter() zenui.Input { return zenui.Input{Keys: []zenui.Key{zenui.KeyEnter}} }

// T-106: File > Open and Load Disk start in the home folder when no folder is kept, and not in
// the folder the program was run from.
func TestOpenDialogsStartInTheHomeFolderNotTheWorkingDirectory(t *testing.T) {
	home := homeAndCwd(t)
	b, _ := newTestBar(t, nil, "")
	b.openFileDialog()
	if got := b.diskDialog.Dir(); got != home {
		t.Errorf("File > Open starts in %q, want the home folder %q", got, home)
	}
	b.diskDialog, b.fileDlg = nil, fileDlgNone
	zx := testZXWithDrive()
	b.openDiskDialog(zx)
	if b.diskDialog == nil {
		t.Fatal("Load Disk did not open on a machine with a drive")
	}
	if got := b.diskDialog.Dir(); got != home {
		t.Errorf("Load Disk starts in %q, want the home folder %q", got, home)
	}
}

// T-106: after a file is opened from a folder the next File > Open starts there, across a restart; a
// folder that has gone sends it home again.
func TestOpenStartsWhereTheLastFileWasOpenedAcrossARestart(t *testing.T) {
	home := homeAndCwd(t)
	settings := filepath.Join(t.TempDir(), "settings.json")
	tapes := filepath.Join(t.TempDir(), "tapes")
	tape := writeTestFile(t, tapes, "jsw.tap")

	b, zx := newTestBar(t, nil, settings)
	b.openFiles = func([]string) {}
	b.openFileDialog()
	b.acceptDialog(zx, tape)

	res, err := settingsconfig.Load(settings, defaultSettingsJSON, validThemeNames, validFontNames)
	if err != nil || !res.FromDisk {
		t.Fatalf("what the bar wrote was refused: %v %s", err, res.Warning)
	}
	b2, _ := newTestBar(t, res.Settings, settings)
	b2.openFileDialog()
	if got := b2.diskDialog.Dir(); got != tapes {
		t.Errorf("after a restart File > Open starts in %q, want %q", got, tapes)
	}

	if err := os.RemoveAll(tapes); err != nil {
		t.Fatal(err)
	}
	b2.diskDialog, b2.fileDlg = nil, fileDlgNone
	b2.openFileDialog()
	if got := b2.diskDialog.Dir(); got != home {
		t.Errorf("with the folder gone File > Open starts in %q, want the home folder %q", got, home)
	}
}

// The two browsers keep a folder each: tapes are not kept with disks.
func TestOpenAndOpenDSKKeepAFolderEach(t *testing.T) {
	homeAndCwd(t)
	root := t.TempDir()
	tape := writeTestFile(t, filepath.Join(root, "tapes"), "a.tap")
	disk := writeTestFile(t, filepath.Join(root, "disks"), "g.dsk")
	b, _ := newTestBar(t, nil, "")
	b.openFiles = func([]string) {}
	zx := testZXWithDrive()

	b.openFileDialog()
	b.acceptDialog(zx, tape)
	b.openDiskDialog(zx)
	if got := b.diskDialog.Dir(); got == filepath.Dir(tape) {
		t.Errorf("Load Disk started where a tape was opened: %q", got)
	}
	b.fileDlg = fileDlgDisk
	b.keepFolder(zx, fileDlgDisk, disk)
	b.diskDialog, b.fileDlg = nil, fileDlgNone

	b.openFileDialog()
	if got := b.diskDialog.Dir(); got != filepath.Dir(tape) {
		t.Errorf("File > Open starts in %q, want the tapes folder", got)
	}
	b.diskDialog, b.fileDlg = nil, fileDlgNone
	b.openDiskDialog(zx)
	if got := b.diskDialog.Dir(); got != filepath.Dir(disk) {
		t.Errorf("Load Disk starts in %q, want the disks folder", got)
	}
}

// T-107 and T-104: a path pasted into File > Open opens the tape it names, wherever it is.
func TestFileOpenTakesAPastedPathToATape(t *testing.T) {
	homeAndCwd(t)
	tape := writeTestFile(t, filepath.Join(t.TempDir(), "far", "away"), "Jetpac.TZX")
	b, zx := newTestBar(t, nil, "")
	var got [][]string
	b.openFiles = func(files []string) { got = append(got, files) }

	b.openFileDialog()
	d := b.diskDialog
	if st := d.Update(zenui.Input{Paste: "\"" + tape + "\"\n"}); st != zenui.Active { // as Explorer's Copy as path gives it
		t.Fatalf("pasting ended the dialog: %v", st)
	}
	if st := d.Update(keyEnter()); st != zenui.Accepted {
		t.Fatalf("Enter on a pasted path gave %v, want Accepted", st)
	}
	b.acceptDialog(zx, d.Result())
	if len(got) != 1 || !reflect.DeepEqual(got[0], []string{tape}) {
		t.Errorf("openFiles got %v, want the tape %q", got, tape)
	}
	// It is the next Open's Recent file, and its folder is where the next Open starts.
	if !reflect.DeepEqual(b.recentFiles, []string{tape}) {
		t.Errorf("recent files %v, want the tape", b.recentFiles)
	}
}

// T-104: the Recent place of the Open dialogs lists the files opened, each medium's own under its file
// type, and not what has gone.
func TestRecentPlaceListsTheFilesOpenedByMedium(t *testing.T) {
	homeAndCwd(t)
	dir := t.TempDir()
	tape := writeTestFile(t, dir, "a.tap")
	disk := writeTestFile(t, dir, "b.dsk")
	snap := writeTestFile(t, dir, "c.z80")
	gone := filepath.Join(dir, "gone.tap")
	b, zx := newTestBar(t, nil, "")
	b.openFiles = func([]string) {}
	for _, p := range []string{gone, tape, snap, disk} {
		b.noteRecentFile(zx, p)
	}
	// With no recent files there is no Recent place.
	empty, _ := newTestBar(t, nil, "")
	empty.openFileDialog()
	if _, ok := drawDialog(empty.diskDialog).at["Recent"]; ok {
		t.Error("a Recent place with nothing recent")
	}

	names := []string{"a.tap", "b.dsk", "c.z80", "gone.tap"}
	b.openFileDialog()
	d := b.diskDialog
	spy := drawDialog(d)
	if _, ok := spy.at["Recent"]; !ok {
		t.Fatal("File > Open has no Recent place after files were opened")
	}
	spy = clickText(t, d, spy, "Recent")
	if got := shown(spy, names...); !reflect.DeepEqual(got, []string{"b.dsk", "c.z80", "a.tap"}) {
		t.Errorf("File > Open's Recent: %v, want every file that is there, newest first (and not the gone one)", got)
	}

	// Load Disk shows only the disks.
	b.diskDialog, b.fileDlg = nil, fileDlgNone
	b.openDiskDialog(testZXWithDrive())
	d = b.diskDialog
	spy = clickText(t, d, drawDialog(d), "Recent")
	if got := shown(spy, names...); !reflect.DeepEqual(got, []string{"b.dsk"}) {
		t.Errorf("Load Disk's Recent: %v, want only the disk", got)
	}
}

// How the person looks at a folder is kept, whether the dialog was accepted or not, and comes back.
func TestFileDialogViewIsKeptAndComesBack(t *testing.T) {
	homeAndCwd(t)
	settings := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, nil, settings)
	b.openFiles = func([]string) {}

	b.openFileDialog()
	d := b.diskDialog
	d.Update(zenui.Input{Ctrl: true, CtrlChars: []rune{'h'}})
	d.Update(zenui.Input{Ctrl: true, CtrlChars: []rune{'d'}})
	if st := d.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}}); st != zenui.Cancelled {
		t.Fatalf("Escape gave %v", st)
	}
	b.endFileDialog(zx, d) // what the bar does when a browser is cancelled
	b.diskDialog = nil

	res, err := settingsconfig.Load(settings, defaultSettingsJSON, validThemeNames, validFontNames)
	if err != nil || !res.FromDisk {
		t.Fatalf("what the bar wrote was refused: %v %s", err, res.Warning)
	}
	if v := res.Settings.FileDialogChoice(); v == nil || !v.Hidden || !v.Details {
		t.Fatalf("the settings kept %+v, want hidden and details on", v)
	}
	b2, _ := newTestBar(t, res.Settings, settings)
	b2.openFileDialog()
	if v := b2.diskDialog.View(); !v.ShowHidden || !v.Details {
		t.Errorf("a new dialog starts with the view %+v, want hidden and details on", v)
	}
	// Putting everything back to the default drops the key.
	b2.diskDialog.Update(zenui.Input{Ctrl: true, CtrlChars: []rune{'h'}})
	b2.diskDialog.Update(zenui.Input{Ctrl: true, CtrlChars: []rune{'d'}})
	b2.endFileDialog(testZX(), b2.diskDialog)
	if b2.fileView != nil {
		t.Errorf("the default view is kept as %+v, want no key", b2.fileView)
	}
}

func TestViewAndSettingsConvertBothWays(t *testing.T) {
	if settingsFromView(zenui.View{}) != nil {
		t.Error("the default view is kept as a key")
	}
	for _, v := range []zenui.View{
		{ShowHidden: true},
		{Details: true, Sort: zenui.SortSize},
		{Sort: zenui.SortDate, Descending: true},
		{ShowHidden: true, Details: true, Sort: zenui.SortType, Descending: true},
	} {
		if back := viewFromSettings(settingsFromView(v)); back != v {
			t.Errorf("view %+v came back as %+v", v, back)
		}
	}
	// A word that is no sort is the default one.
	if got := viewFromSettings(&settingsconfig.FileDialogView{Sort: "colour"}); got.Sort != zenui.SortName {
		t.Errorf("an unknown sort reads as %v", got.Sort)
	}
}

// Every kind File > Open takes is in a medium's set, and every set is made of kinds it takes.
func TestFileOpenSetsCoverExactlyWhatItTakes(t *testing.T) {
	sets := fileOpenSets()
	if !reflect.DeepEqual(sets[0].Exts, fileOpenExts) {
		t.Errorf("the first set %v is not everything File > Open takes %v", sets[0].Exts, fileOpenExts)
	}
	seen := map[string]bool{}
	for _, s := range sets[1:] {
		for _, e := range s.Exts {
			seen[e] = true
		}
	}
	for _, e := range fileOpenExts {
		if !seen[e] {
			t.Errorf(".%s is taken by File > Open but is in no medium's set", e)
		}
	}
	for e := range seen {
		found := false
		for _, x := range fileOpenExts {
			found = found || x == e
		}
		if !found {
			t.Errorf(".%s is in a medium's set but File > Open does not take it", e)
		}
	}
}

// Helpers over zenui's Dialog, which shows its rows only by drawing them: the dialog is drawn with a
// renderer that remembers where each text went, and a click is made where a label was drawn.

type dlgSpy struct {
	testNoopRenderer
	at map[string][2]int // a text -> where it was last drawn
}

func (s *dlgSpy) DrawText(str string, x, y, scale int, c zenui.Colour) { s.at[str] = [2]int{x, y} }

func drawDialog(d *zenui.Dialog) *dlgSpy {
	s := &dlgSpy{at: map[string][2]int{}}
	d.Draw(s, 1000, 700, zenui.DarkTheme())
	return s
}

// clickText clicks the label drawn as text and draws the dialog again.
func clickText(t *testing.T, d *zenui.Dialog, spy *dlgSpy, text string) *dlgSpy {
	t.Helper()
	p, ok := spy.at[text]
	if !ok {
		t.Fatalf("%q is not on the dialog", text)
	}
	d.Update(zenui.Input{MouseX: p[0] + 2, MouseY: p[1] + 2, MousePressed: true})
	return drawDialog(d)
}

// shown is which of names are drawn, from the top down.
func shown(spy *dlgSpy, names ...string) []string {
	var out []string
	for _, n := range names {
		if _, ok := spy.at[n]; ok {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return spy.at[out[i]][1] < spy.at[out[j]][1] })
	return out
}
