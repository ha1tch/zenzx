//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
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
	floppyItems := len(b.widget.ItemsFor(barFloppyDisk))

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
	// Index 0 is the "SD Card" title, so slot 2 is item 2 and Remove is item 6.
	if items[0].Label != "SD Card" || !items[0].Title {
		t.Errorf("item 0 should be the SD Card title, got %q (title %v)", items[0].Label, items[0].Title)
	}
	if items[2].Label != "Slot 2: empty" || !items[6].Disabled {
		t.Errorf("with nothing in slot 2: %q, Remove disabled %v", items[2].Label, items[6].Disabled)
	}
	// The Virtual Floppy section is present and every item in it is disabled (T-72).
	if !hasDisabledVirtualFloppy(items) {
		t.Error("the Next storage menu should carry a disabled Virtual Floppy section (T-72)")
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
	if items[2].Label != "Slot 2: "+ownCardName || items[6].Disabled {
		t.Errorf("with the card in: %q, Remove disabled %v", items[2].Label, items[6].Disabled)
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

	// Away from the Next the Floppy Disk contents are back, as they were, under the same
	// "Storage" label, and the slot is empty.
	b.storageSpec.pick(zx, row("Insert ZenZX's Card (Examples)"), -1)
	zx.detachNextBootChain()
	if zx.SecondCard() != "" {
		t.Error("the second card stayed after the Next was left")
	}
	b.refreshStorageMenu(zx)
	if got := b.widget.LabelFor(barFloppyDisk); got != storageLabel || len(b.widget.ItemsFor(barFloppyDisk)) != floppyItems {
		t.Fatalf("after the Next the menu label is %q with %d items", got, len(b.widget.ItemsFor(barFloppyDisk)))
	}
}

// hasDisabledVirtualFloppy reports whether items carries the Virtual Floppy title and at
// least one disabled item after it -- the signpost section for the unimplemented NextZXOS
// disc-image mounting (T-72).
func hasDisabledVirtualFloppy(items []zenui.Item) bool {
	for i, it := range items {
		if it.Title && it.Label == "Virtual Floppy" {
			for _, rest := range items[i+1:] {
				if rest.Disabled {
					return true
				}
			}
		}
	}
	return false
}
