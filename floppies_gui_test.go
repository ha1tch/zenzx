//go:build !headless

package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// diskMachine is a +3 with the synthetic test disc, a copy in a new folder,
// in its drive.
func diskMachine(t *testing.T) (*ZenZX, string) {
	t.Helper()
	zx := NewZenZX(AudioBackendOto)
	zx.io.EnableFDC()
	path := tempDSK(t)
	if err := zx.io.LoadDisk(path); err != nil {
		t.Fatal(err)
	}
	return zx, path
}

// soloDisks opens the window for a machine that is alone in its window.
func soloDisks(zx *ZenZX) *disksWindow {
	use := func(path string) int {
		if f := zx.io.fdc.DiskFile(); f != "" && same(f, path) {
			return 1
		}
		return 0
	}
	return newDisksWindow(zx, 1, use, func(n int) *ZenZX {
		if n == 1 {
			return zx
		}
		return nil
	})
}

func diskButtons(w *disksWindow) map[string]bool {
	w.setButtons()
	out := map[string]bool{}
	for _, b := range w.buttons.Buttons {
		out[b.ID] = !b.Disabled
	}
	return out
}

func TestDescribeDisk(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.dsk")
	os.WriteFile(raw, bytes.Repeat([]byte{0xE5}, DiskImageSize), 0o644)
	junk := filepath.Join(dir, "junk.dsk")
	os.WriteFile(junk, []byte("not a disc"), 0o644)

	if e := describeDisk(tempDSK(t)); !strings.Contains(e.kind, "DSK") || e.why != "" {
		t.Errorf("the synthetic disc: kind %q, why %q", e.kind, e.why)
	}
	if e := describeDisk(raw); e.kind != "raw 180K" {
		t.Errorf("a 180K sector dump: kind %q, why %q", e.kind, e.why)
	}
	if e := describeDisk(junk); e.kind != "" || e.why == "" {
		t.Errorf("a file that is not a disc: kind %q, why %q", e.kind, e.why)
	}
	if got := listDisks(dir, nil); len(got) != 2 || filepath.Base(got[0].path) != "junk.dsk" {
		t.Errorf("listDisks found %d files", len(got))
	}
}

func TestDisksWindowShowsTheDriveAgainstItsFile(t *testing.T) {
	zx, path := diskMachine(t)
	w := soloDisks(zx)
	if d, ok := w.selected(); !ok || !same(d.path, path) {
		t.Fatal("the window does not open on the disc in the drive")
	}
	head := func() string { return strings.Join(w.headLines(), "\n") }

	if !strings.Contains(head(), "the same as its file") {
		t.Errorf("a disc just read:\n%s", head())
	}
	st := diskButtons(w)
	if st[diskBtnWrite] || st[diskBtnInsert] || !st[diskBtnEject] || !st[diskBtnReload] || !st[diskBtnSaveNew] {
		t.Errorf("buttons for a disc just read: %v", st)
	}

	scribble(t, zx.io.fdc)
	if !strings.Contains(head(), "changes here not yet in the file") {
		t.Errorf("a disc written to:\n%s", head())
	}
	if !strings.Contains(w.row(w.disks[w.list.Selected]), "in the drive, unsaved") {
		t.Errorf("row: %q", w.row(w.disks[w.list.Selected]))
	}
	if !diskButtons(w)[diskBtnWrite] {
		t.Error("a disc with changes cannot be written to its file")
	}

	touchOutside(t, path)
	if !strings.Contains(head(), "changed here AND on disk") {
		t.Errorf("changed on both sides:\n%s", head())
	}
}

func TestDisksWindowAsksBeforeLosingEitherSide(t *testing.T) {
	zx, path := diskMachine(t)
	w := soloDisks(zx)
	fdc := zx.io.fdc

	// Changes in memory: reading the file again asks first.
	scribble(t, fdc)
	inMemory := fdc.disk.Serialize()
	w.setButtons()
	w.do(diskBtnReload)
	if !fdc.IsModified() || w.confirm != diskBtnReload || !strings.Contains(w.note, "loses them") {
		t.Fatalf("the first press of Read file again: modified %v, confirm %q, note %q", fdc.IsModified(), w.confirm, w.note)
	}
	// Another button forgets the question.
	w.setButtons()
	w.do(diskBtnNew)
	if w.confirm != "" {
		t.Error("another action did not withdraw the question")
	}
	w.selectPath(path)

	// The file changed outside as well: writing asks first, then writes.
	touchOutside(t, path)
	w.setButtons()
	w.do(diskBtnWrite)
	if !fdc.IsModified() || w.confirm != diskBtnWrite {
		t.Fatalf("the first press of Write over a changed file wrote it: modified %v, confirm %q", fdc.IsModified(), w.confirm)
	}
	w.setButtons()
	if label := w.buttons.Buttons[2].Label; !strings.Contains(label, "sure?") {
		t.Errorf("the button awaiting a second press reads %q", label)
	}
	w.do(diskBtnWrite)
	if fdc.IsModified() || fdc.FileChanged() {
		t.Fatalf("the second press did not write: modified %v, changed %v", fdc.IsModified(), fdc.FileChanged())
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, inMemory) {
		t.Error("the file does not have the disc as it was in memory")
	}

	// Reading again with nothing to lose needs no second press.
	touchOutside(t, path)
	w.setButtons()
	w.do(diskBtnReload)
	if fdc.FileChanged() || w.confirm != "" {
		t.Errorf("Read file again with no changes in memory: changed %v, confirm %q", fdc.FileChanged(), w.confirm)
	}
}

func TestDisksWindowSaveAsNewKeepsBoth(t *testing.T) {
	zx, path := diskMachine(t)
	w := soloDisks(zx)
	before, _ := os.ReadFile(path)
	scribble(t, zx.io.fdc)
	w.setButtons()
	w.do(diskBtnSaveNew)
	newFile := zx.io.fdc.DiskFile()
	if same(newFile, path) || filepath.Base(newFile) != "disc-2.dsk" {
		t.Fatalf("the drive's file after Save as new: %q", newFile)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, before) {
		t.Error("the file the disc came from was changed")
	}
	if zx.io.fdc.IsModified() {
		t.Error("the disc still counts as unsaved")
	}
	if d, ok := w.selected(); !ok || !same(d.path, newFile) {
		t.Error("the new file is not listed and selected")
	}
}

func TestDisksWindowBlankDiscAndEject(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	zx.io.EnableFDC()
	dir := t.TempDir()
	w := soloDisks(zx)
	w.dir = dir
	w.refresh()
	if st := diskButtons(w); st[diskBtnEject] || st[diskBtnSaveNew] || st[diskBtnInsert] || !st[diskBtnNew] {
		t.Errorf("buttons with an empty drive and an empty folder: %v", st)
	}

	w.do(diskBtnNew)
	d, ok := w.selected()
	if !ok || filepath.Base(d.path) != "disk.dsk" || d.kind != "raw 180K" {
		t.Fatalf("after New blank disc: %+v", d)
	}
	w.setButtons()
	w.do(diskBtnInsert)
	if !same(zx.io.fdc.DiskFile(), d.path) {
		t.Fatalf("the new disc is not in the drive: %q", w.note)
	}

	// A blank disc with no file, written to: Eject asks, since it would be lost.
	zx.io.fdc.CreateBlankDisk()
	zx.io.fdc.markModified()
	w.setButtons()
	w.do(diskBtnEject)
	if !zx.io.fdc.HasDisk() || w.confirm != diskBtnEject {
		t.Fatal("ejecting a written blank disc with no file did not ask first")
	}
	// Putting another disc in is refused outright while that one is unsaved.
	w.selectPath(d.path)
	w.setButtons()
	w.do(diskBtnInsert)
	if zx.io.fdc.DiskFile() != "" {
		t.Error("another disc replaced a written blank disc that had no file")
	}
	w.setButtons()
	w.do(diskBtnEject)
	w.setButtons()
	w.do(diskBtnEject)
	if zx.io.fdc.HasDisk() {
		t.Error("the second press of Eject did not eject")
	}
}

func TestDisksWindowAcrossMachines(t *testing.T) {
	a, path := diskMachine(t)
	b := NewZenZX(AudioBackendOto)
	b.io.EnableFDC()
	machines := []*ZenZX{a, b}
	use := func(p string) int {
		for i, m := range machines {
			if f := m.io.fdc.DiskFile(); f != "" && same(f, p) {
				return i + 1
			}
		}
		return 0
	}
	holder := func(n int) *ZenZX {
		if n < 1 || n > len(machines) {
			return nil
		}
		return machines[n-1]
	}
	w := newDisksWindow(b, 2, use, holder)
	w.dir = filepath.Dir(path)
	w.refresh()
	w.selectPath(path)

	if diskButtons(w)[diskBtnInsert] {
		t.Error("a disc in another machine's drive can be put in this one")
	}
	if row := w.row(w.disks[w.list.Selected]); !strings.Contains(row, "machine 1") {
		t.Errorf("row %q does not say machine 1 has the disc", row)
	}

	// Copying a disc another machine has changes to writes those first.
	scribble(t, a.io.fdc)
	want := a.io.fdc.disk.Serialize()
	w.setButtons()
	w.do(diskBtnCopy)
	if a.io.fdc.IsModified() {
		t.Error("the machine's changes were not written before the copy")
	}
	d, _ := w.selected()
	if filepath.Base(d.path) != "disc-2.dsk" {
		t.Fatalf("the copy is %q; note %q", d.path, w.note)
	}
	if got, _ := os.ReadFile(d.path); !bytes.Equal(got, want) {
		t.Error("the copy does not have the changes the machine had made")
	}
	w.setButtons()
	w.do(diskBtnInsert)
	if !same(b.io.fdc.DiskFile(), d.path) {
		t.Errorf("the copy could not be put in the second machine's drive: %q", w.note)
	}
}

func TestDisksWindowLaysOut(t *testing.T) {
	zx, _ := diskMachine(t)
	w := soloDisks(zx)
	for _, size := range [][2]int{{960, 768}, {640, 480}, {352, 296}} {
		r := testNoopRenderer{}
		w.Draw(r, size[0], size[1], zenui.DarkTheme())
		if w.panel.X < 0 || w.panel.X+w.panel.W > size[0] {
			t.Errorf("%v: the panel is wider than the window: %+v", size, w.panel)
		}
		if w.barArea.Y < w.noteY || w.noteY < w.listArea.Y+w.listArea.H {
			t.Errorf("%v: the list, the note and the buttons overlap", size)
		}
	}
	if w.Update(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}}, testNoopRenderer{}, 960, 768) {
		t.Error("Escape did not close the window")
	}
}

func TestDeckDiskUse(t *testing.T) {
	d := testDeck(t, 2)
	path := tempDSK(t)
	d.machines[1].zx.io.EnableFDC()
	if err := d.machines[1].zx.io.LoadDisk(path); err != nil {
		t.Fatal(err)
	}
	if n := d.diskUse(path); n != 2 {
		t.Errorf("diskUse = %d, want machine 2", n)
	}
	if n := d.diskUse(filepath.Join(filepath.Dir(path), "other.dsk")); n != 0 {
		t.Errorf("diskUse of a disc nobody has = %d", n)
	}
	if d.machineAt(2) != d.machines[1].zx || d.machineAt(3) != nil || d.machineAt(0) != nil {
		t.Error("machineAt does not follow the dock's numbers")
	}
}
