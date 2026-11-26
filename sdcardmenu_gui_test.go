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

// The bar's storage menu keeps one label ("Storage") on every machine -- it no longer
// swaps under the user. Its contents follow the machine: the +3's Floppy Disk items off a
// Next, and the SD Card slots plus the (disabled) Virtual Floppy section while a Next with
// its boot chain runs. The SD Card actions change the second slot.
func TestStorageMenuFollowsTheMachine(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	b.refreshStorageMenu(zx)
	if got := b.widget.LabelFor(barFloppyDisk); got != storageLabel || b.storageIsSD {
		t.Fatalf("on a 48K the menu label is %q (storageIsSD=%v)", got, b.storageIsSD)
	}
	// Both sections are there on a model with neither a drive nor cards, and nothing in them
	// does anything: a row that can only say "no drive" is not shown as live.
	none := b.widget.ItemsFor(barFloppyDisk)
	for _, title := range []string{"Floppy Disk", "SD Card"} {
		if !hasSection(none, title) || len(liveRows(none, title)) != 0 {
			t.Errorf("a 48K's %q section: present %v, live rows %v", title, hasSection(none, title), liveRows(none, title))
		}
	}

	// The +3's drive makes its section live, and only that one.
	zx.io.EnableFDC()
	b.refreshStorageMenu(zx)
	plus3 := b.widget.ItemsFor(barFloppyDisk)
	if len(liveRows(plus3, "Floppy Disk")) == 0 || len(liveRows(plus3, "SD Card")) != 0 {
		t.Errorf("a +3's rows: floppy %v, SD %v", liveRows(plus3, "Floppy Disk"), liveRows(plus3, "SD Card"))
	}
	if b.storageIsSD {
		t.Error("a +3 is not a Next")
	}
	zx.io.DisableFDC()
	b.refreshStorageMenu(zx)
	if len(liveRows(b.widget.ItemsFor(barFloppyDisk), "Floppy Disk")) != 0 {
		t.Error("the floppy section stayed live with the controller off")
	}

	// A Next with its boot chain: placeholders for the files are enough for the menu.
	dir := t.TempDir()
	writeNextFiles(t, dir)
	zx.memory.Enable128K()
	zx.memory.EnablePlus3()
	zx.memory.EnableNext()
	if _, err := zx.attachNextBootChain(nextcard.In(dir), true, false); err != nil {
		t.Fatal(err)
	}
	b.refreshStorageMenu(zx)
	if got := b.widget.LabelFor(barFloppyDisk); got != storageLabel || !b.storageIsSD {
		t.Fatalf("on a Next that boots from its card the menu label is %q (storageIsSD=%v)", got, b.storageIsSD)
	}
	items := b.widget.ItemsFor(barFloppyDisk)
	// The menu is for the person using it: no tracker item numbers, and no notes about what
	// the emulator does not do yet, which belong in the log and the register.
	for _, it := range items {
		low := strings.ToLower(it.Label)
		if strings.Contains(it.Label, "T-") || strings.Contains(low, "not emulated") || strings.Contains(low, "yet") {
			t.Errorf("the Storage menu has the development note %q", it.Label)
		}
	}
	if len(items) != len(b.storageSpec) || len(b.storageSpec.actions()) != len(items) {
		t.Fatalf("%d items, %d rows, %d actions", len(items), len(b.storageSpec), len(b.storageSpec.actions()))
	}
	// A row is found by its label: the section titles decide where it is, and the tests
	// should not.
	row := func(label string) int {
		for i, it := range b.widget.ItemsFor(barFloppyDisk) {
			if it.Label == label {
				return i
			}
		}
		t.Fatalf("the SD Card menu has no %q row", label)
		return -1
	}
	// The same two sections as on any model: the card one live, the floppy one a disabled
	// signpost for the NextZXOS disc images (T-72).
	if !hasSection(items, "SD Card") || len(liveRows(items, "SD Card")) == 0 {
		t.Error("a Next's SD Card section is not live")
	}
	if !hasSection(items, "Floppy Disk") || len(liveRows(items, "Floppy Disk")) != 0 {
		t.Errorf("a Next's Floppy Disk section should be a disabled signpost (T-72), live rows %v", liveRows(items, "Floppy Disk"))
	}
	if items[row("Slot 2: empty")].Label != "Slot 2: empty" || !items[row("Remove Card from Slot 2")].Disabled {
		t.Errorf("with nothing in slot 2, Remove should be disabled")
	}

	// Insert ZenZX's Card makes the card, with the examples, and puts it in (action 5).
	b.storageSpec.pick(zx, row("Insert ZenZX's Card (Examples)"), -1)
	own := filepath.Join(dir, ownCardName)
	if zx.SecondCard() != own {
		t.Fatalf("slot 2 holds %q, want %q", zx.SecondCard(), own)
	}
	if st, err := os.Stat(own); err != nil || st.Size() != nextcard.SmallestCard {
		t.Fatalf("the card that was made: %v", err)
	}
	b.refreshStorageMenu(zx)
	items = b.widget.ItemsFor(barFloppyDisk)
	if items[row("Slot 2: "+ownCardName)].Disabled != true || items[row("Remove Card from Slot 2")].Disabled {
		t.Errorf("with the card in, slot 2 should be named and Remove live")
	}
	// The boot card is not accepted for the second slot.
	b.insertCard(zx, filepath.Join(dir, nextcard.CardName))
	if zx.SecondCard() != own {
		t.Error("the card in slot 1 was put in slot 2 as well")
	}
	b.storageSpec.pick(zx, row("Remove Card from Slot 2"), -1)
	if zx.SecondCard() != "" || zx.io.divMMC.SD2() != nil {
		t.Error("Remove left a card in slot 2")
	}

	// New Empty Card: a size from its submenu, then a name; the card is made, is empty, and
	// goes into slot 2. A name that is taken is refused and the slot is left as it was.
	newCardRow := row("New Empty Card")
	if sub := items[newCardRow].SubItems; len(sub) != len(newCardSizes) {
		t.Fatalf("New Empty Card has %d sizes, want %d", len(sub), len(newCardSizes))
	}
	b.storageSelected(zx, newCardRow, 0)
	if b.diskDialog == nil || b.dialogKind != dialogNewCard || b.newCardSize != nextcard.SmallestCard {
		t.Fatalf("choosing a size did not ask for a name (size %d)", b.newCardSize)
	}
	b.diskDialog, b.dialogKind, b.newCardSize = nil, dialogDisk, 0
	fresh := filepath.Join(dir, "mine")
	b.newCard(zx, fresh, nextcard.SmallestCard)
	if zx.SecondCard() != fresh+".img" {
		t.Fatalf("slot 2 holds %q, want the new card", zx.SecondCard())
	}
	f, err := os.Open(fresh + ".img")
	if err != nil {
		t.Fatal(err)
	}
	vol, err := nextcard.Open(f)
	f.Close()
	if err != nil || len(vol.Root.Children) != 0 {
		t.Fatalf("the new card: %v, %d entries in its top folder", err, len(vol.Root.Children))
	}
	b.newCard(zx, own, nextcard.SmallestCard)
	if zx.SecondCard() != fresh+".img" {
		t.Error("a name that was taken changed slot 2")
	}
	if st, _ := os.Stat(own); st == nil || st.Size() != nextcard.SmallestCard {
		t.Error("the card whose name was taken was written over")
	}
	zx.RemoveSecondCard()

	// Away from the Next the menu is the drive-less one again, under the same "Storage" label.
	b.storageSpec.pick(zx, row("Insert ZenZX's Card (Examples)"), -1)
	zx.detachNextBootChain()
	if zx.SecondCard() != "" {
		t.Error("the second card stayed after the Next was left")
	}
	b.refreshStorageMenu(zx)
	if got := b.widget.LabelFor(barFloppyDisk); got != storageLabel || len(liveRows(b.widget.ItemsFor(barFloppyDisk), "SD Card")) != 0 {
		t.Fatalf("after the Next the menu label is %q, SD rows %v", got, liveRows(b.widget.ItemsFor(barFloppyDisk), "SD Card"))
	}
}

// What the disk actions say goes to the host as well as the console: a window has no console
// in front of it, and the host shows it as a toast.
func TestDiskActionsSpeakToTheHost(t *testing.T) {
	zx := testZX()
	var said []string
	zx.OnNotice = func(msg string) { said = append(said, msg) }
	expect := func(want string) {
		t.Helper()
		if len(said) != 1 || said[0] != want {
			t.Fatalf("said %q, want %q", said, want)
		}
		said = nil
	}

	zx.handleEjectDisk()
	expect("This model has no disk drive")
	zx.handleSaveDisk()
	expect("There is nothing to save: the disk has not changed")

	zx.io.EnableFDC()
	zx.handleEjectDisk()
	expect("There is no disk in the drive") // a new drive is empty
	zx.handleInsertBlankDisk()
	expect("A blank formatted disk is in the drive: SAVE \"name\" puts a BASIC program on it, CAT lists it, F8 keeps it in a file")
	zx.handleEjectDisk()
	expect("The disk is ejected")
	zx.handleEjectDisk()
	expect("There is no disk in the drive")
}

// A section the machine cannot use is one line, N/A: a 48K without a drive, a +3 with its
// controller switched off.
func TestStorageMenuMarksAnAbsentSectionNA(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()
	note := func() string {
		for _, it := range b.storageMenuSpec(zx).items() {
			if it.Disabled && strings.TrimSpace(it.Label) == "N/A" {
				return it.Label
			}
		}
		return ""
	}
	if got := note(); got != sectionAbsent {
		t.Errorf("48K: %q", got)
	}
	zx.memory.Enable128K()
	zx.memory.EnablePlus3()
	if got := note(); got != sectionAbsent {
		t.Errorf("+3 without its controller: %q", got)
	}
}

// hasSection reports whether items has a title row of that name.
func hasSection(items []zenui.Item, title string) bool {
	for _, it := range items {
		if it.Title && it.Label == title {
			return true
		}
	}
	return false
}

// liveRows are the labels of the rows that do something in the section that starts at the
// title row: the rows after it up to the next title, leaving out the disabled ones and the
// dividers.
func liveRows(items []zenui.Item, title string) []string {
	var rows []string
	in := false
	for _, it := range items {
		switch {
		case it.Title:
			in = it.Label == title
		case in && it.Label != "" && !it.Disabled:
			rows = append(rows, it.Label)
		}
	}
	return rows
}
