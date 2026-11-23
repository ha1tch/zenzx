//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// fitsThePanel fails if any line of an info text is wider than the reader's
// panel holds.
func fitsThePanel(t *testing.T, what, text string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if len([]rune(line)) > 70 {
			t.Errorf("%s: a line of %d columns: %q", what, len([]rune(line)), line)
		}
	}
	if !strings.HasPrefix(text, "## ") {
		t.Errorf("%s: the text does not begin with a heading: %q", what, strings.SplitN(text, "\n", 2)[0])
	}
}

func TestTapeInfoText(t *testing.T) {
	zx := newSideMachine(t, "48k", 0, "").zx
	text := tapeInfoText(zx)
	fitsThePanel(t, "no tape", text)
	if !strings.Contains(text, "No tape is loaded") {
		t.Errorf("with no tape: %q", text)
	}
	if err := zx.AutorunBASIC("receiver", "10 PRINT 1\n20 GO TO 10\n", "48k"); err != nil {
		t.Fatal(err)
	}
	text = tapeInfoText(zx)
	fitsThePanel(t, "a tape", text)
	for _, want := range []string{"receiver", "fast", "stopped, at block 1 of 2", "## Blocks", "Header 'receiver", "Program"} {
		if !strings.Contains(text, want) {
			t.Errorf("the tape's text lacks %q:\n%s", want, text)
		}
	}
}

func TestDiskInfoText(t *testing.T) {
	zx := newSideMachine(t, "48k", 0, "").zx
	text := diskInfoText(zx)
	fitsThePanel(t, "no drive", text)
	if !strings.Contains(text, "no floppy drive") {
		t.Errorf("on a 48K: %q", text)
	}

	plus3 := NewZenZX(AudioBackendOto)
	switchModelLive(plus3, "plus3", "./rom")
	text = diskInfoText(plus3)
	fitsThePanel(t, "an empty drive", text)
	if !strings.Contains(text, "Drive A: is empty") {
		t.Errorf("with an empty drive: %q", text)
	}

	plus3.io.fdc.CreateBlankDisk()
	text = diskInfoText(plus3)
	fitsThePanel(t, "a blank disc", text)
	for _, want := range []string{"a blank disc, not saved yet", "40 tracks, one side"} {
		if !strings.Contains(text, want) {
			t.Errorf("a blank disc's text lacks %q:\n%s", want, text)
		}
	}

	// Saved to a file with a long path, then changed in memory.
	deep := filepath.Join(t.TempDir(), strings.Repeat("a-rather-long-folder-name/", 3))
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(deep, "work.dsk")
	if err := plus3.io.SaveDiskAs(file); err != nil {
		t.Fatal(err)
	}
	plus3.io.fdc.diskModified = true
	text = diskInfoText(plus3)
	fitsThePanel(t, "a disc with a long path", text)
	for _, want := range []string{"work.dsk", "...", "unsaved: in memory", "File size"} {
		if !strings.Contains(text, want) {
			t.Errorf("a saved, changed disc's text lacks %q:\n%s", want, text)
		}
	}
}

func TestCardInfoText(t *testing.T) {
	zx := newSideMachine(t, "48k", 0, "").zx
	text := cardInfoText(zx)
	fitsThePanel(t, "no slots", text)
	if !strings.Contains(text, "no card slots") {
		t.Errorf("on a 48K: %q", text)
	}

	// A slot's card: one that boots, one that does not, one that is not there.
	dir := testBootFolder(t)
	std := filepath.Join(dir, "next-card.img")
	lines := strings.Join(cardSlotText(1, std, true), "\n")
	fitsThePanel(t, "a system card", lines)
	for _, want := range []string{"next-card.img", "280 MB", "write-protected", "8 files", "Boots a Next    yes"} {
		if !strings.Contains(lines, want) {
			t.Errorf("a system card's text lacks %q:\n%s", want, lines)
		}
	}
	empty, err := newEmptyCard(filepath.Join(dir, "empty"), smallestCard)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Join(cardSlotText(2, empty, false), "\n")
	fitsThePanel(t, "an empty card", lines)
	for _, want := range []string{"## Slot 2", "can be written to", "0 files", "Boots a Next    no: the card has no TBBLUE.FW"} {
		if !strings.Contains(lines, want) {
			t.Errorf("an empty card's text lacks %q:\n%s", want, lines)
		}
	}
	lines = strings.Join(cardSlotText(2, filepath.Join(dir, "gone.img"), false), "\n")
	fitsThePanel(t, "a missing card", lines)
	if !strings.Contains(lines, "cannot be read") {
		t.Errorf("a missing card's text:\n%s", lines)
	}
}

func TestInfoItemsOpenAWindow(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatal(err)
	}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	fitsThePanel(t, "snapshots", snapshotInfoText())
	// The Info item of each menu, by its place in that menu.
	cases := []struct {
		menu, item int
		title      string
		says       string
	}{
		{barTape, 4, "TAPE INFO", "No tape is loaded"},
		{barFloppyDisk, 8, "DISK INFO", "no floppy drive"}, // index 8: a Floppy Disk title now precedes the items
		{barSnapshot, 4, "SNAPSHOT INFO", "Quick Save"},
	}
	for _, c := range cases {
		b.activeModal = nil
		b.actions[c.menu][c.item](zx)
		if b.activeModal == nil {
			t.Errorf("%s: no window was opened", c.title)
			continue
		}
		if b.activeModal.Title() != c.title {
			t.Errorf("the window's title is %q, want %q", b.activeModal.Title(), c.title)
		}
		if text := strings.Join(b.activeModal.Lines(), "\n"); !strings.Contains(text, c.says) {
			t.Errorf("%s does not say %q:\n%s", c.title, c.says, text)
		}
		if !b.activeModal.AutoHeight() {
			t.Errorf("%s: a short text should have a panel of its own height", c.title)
		}
		if !b.Active() {
			t.Errorf("%s: the bar should count as active while the window is open", c.title)
		}
	}
	// A long text scrolls in the tall panel.
	b.showInfo("TAPE INFO", "## Blocks\n"+strings.Repeat("  0: Data 100 bytes\n", 40))
	if b.activeModal.AutoHeight() {
		t.Error("a text of forty lines got a panel of its own height")
	}
}
