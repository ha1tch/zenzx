//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What used to be said on the console only is said in the window too: the tests collect what the
// hooks the window sets (OnNotice, OnInfo) hear.

// said collects what a machine tells the window.
func said(zx *ZenZX) *[]string {
	var got []string
	zx.OnNotice = func(m string) { got = append(got, m) }
	return &got
}

func mentions(t *testing.T, got []string, want ...string) {
	t.Helper()
	all := strings.Join(got, "\n")
	if len(got) == 0 {
		t.Errorf("nothing was said, want %q", want)
	}
	for _, w := range want {
		if !strings.Contains(all, w) {
			t.Errorf("%q does not say %q", all, w)
		}
	}
}

// Alt+I opens the tape's window, with or without a tape.
func TestAltIOpensTheTapeInfoWindow(t *testing.T) {
	zx := testZX()
	var title, text string
	zx.OnInfo = func(ti, te string) { title, text = ti, te }
	zx.ShowTapeInfo()
	if title != "TAPE INFO" || !strings.Contains(text, "No tape is loaded") {
		t.Errorf("no tape: window %q says %q", title, text)
	}
	if err := zx.loadTape(t); err != nil {
		t.Fatal(err)
	}
	title, text = "", ""
	zx.ShowTapeInfo()
	if title != "TAPE INFO" || !strings.Contains(text, "Blocks") {
		t.Errorf("a tape: window %q says %q", title, text)
	}
}

// loadTape puts a tape of one block in the machine.
func (zx *ZenZX) loadTape(t *testing.T) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.tap")
	if err := os.WriteFile(path, []byte{0x03, 0x00, 0xFF, 0x01, 0xFE}, 0o644); err != nil {
		return err
	}
	if zx.tape == nil {
		zx.tape = NewTape(zx)
	}
	return zx.tape.LoadFile(path)
}

// F5 says how to use the blank disk, and what became of the disk it replaced, in the window.
func TestInsertBlankDiskSaysHowToUseIt(t *testing.T) {
	zx := machineOf(t, "plus3")
	got := said(zx)
	zx.handleInsertBlankDisk()
	mentions(t, *got, "blank", "SAVE", "CAT", "F8")
}

// A model that cannot be switched to, and a ROM that cannot be used, say so in the window.
func TestSwitchFailuresAreSaidInTheWindow(t *testing.T) {
	zx := testZX()
	got := said(zx)
	switchModelLive(zx, "no-such-model", "./rom")
	mentions(t, *got, "no-such-model")

	*got = nil
	applyCustomROMLive(zx, t.TempDir(), "missing.rom", 0)
	mentions(t, *got, "missing.rom", "could not be used")

	*got = nil
	switchModelLive(zx, "128k", "./rom")
	mentions(t, *got, "Switched to")
}

// The snapshots moved into their folder are returned for the window to say.
func TestMigrateLegacySnapshotsReturnsItsNotes(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "quicksave.zxs")
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	was := snapshotDir
	defer func() { snapshotDir = was }()
	snapshotDir = "snapshots"
	notes := migrateLegacySnapshots()
	mentions(t, notes, "1 snapshots", "snapshots/")
	if again := migrateLegacySnapshots(); len(again) != 0 {
		t.Errorf("the second time there was nothing to move, and %q was said", again)
	}
}

// What is found before the window is up is a toast when it is one thing, and a window when it is
// several; nothing when there is nothing.
func TestStartupNoticeShape(t *testing.T) {
	if toast, title, text := startupNotice(nil); toast+title+text != "" {
		t.Errorf("nothing found: %q %q %q", toast, title, text)
	}
	if toast, title, text := startupNotice([]string{"", "  "}); toast+title+text != "" {
		t.Errorf("blank lines: %q %q %q", toast, title, text)
	}
	if toast, title, _ := startupNotice([]string{"one thing"}); toast != "one thing" || title != "" {
		t.Errorf("one: toast %q title %q", toast, title)
	}
	toast, title, text := startupNotice([]string{"first", "second"})
	if toast != "" || title != "NOTICES" || !strings.Contains(text, "- first\n- second") {
		t.Errorf("two: toast %q title %q text %q", toast, title, text)
	}
}

// A disk whose file changed behind it is not written over, and the person is told, once.
func TestFDCTickTellsOnceOfAFileChangedOutside(t *testing.T) {
	path := tempDSK(t)
	zx := machineOf(t, "plus3")
	if err := zx.io.LoadDisk(path); err != nil {
		t.Fatal(err)
	}
	if w := zx.io.FDCTick(0, 50); w != "" {
		t.Errorf("a disk just read: %q", w)
	}
	scribble(t, zx.io.fdc)
	touchOutside(t, path)
	w := zx.io.FDCTick(300, 50)
	if !strings.Contains(w, filepath.Base(path)) || !strings.Contains(w, "changed") {
		t.Errorf("the file changed under the disk: %q", w)
	}
	if again := zx.io.FDCTick(400, 50); again != "" {
		t.Errorf("it was said twice: %q", again)
	}
}

// The keys that did nothing say why in the window, not only on the console.
func TestDisplayExplainsWhyAKeyDidNothing(t *testing.T) {
	dm := &DisplayManager{}
	var got []string
	dm.OnNotice = func(m string) { got = append(got, m) }
	dm.tiles = func() {}
	if !dm.tiledNoScale() {
		t.Fatal("a tiled layout has a zoom")
	}
	mentions(t, got, "tiled layout")
}
