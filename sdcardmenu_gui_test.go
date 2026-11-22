//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The bar's fourth menu is Floppy Disk until a Next with its boot chain is running, SD Card
// while it is, and Floppy Disk again afterwards; and the SD Card menu's actions change the
// second slot.
func TestStorageMenuFollowsTheMachine(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	b.refreshStorageMenu(zx)
	if got := b.widget.LabelFor(barFloppyDisk); got != storageLabelFloppy || b.storageIsSD {
		t.Fatalf("on a 48K the menu is %q", got)
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
	if got := b.widget.LabelFor(barFloppyDisk); got != storageLabelSD || !b.storageIsSD {
		t.Fatalf("on a Next that boots from its card the menu is %q", got)
	}
	items := b.widget.ItemsFor(barFloppyDisk)
	if len(items) != len(b.sdActions) {
		t.Fatalf("%d items and %d actions", len(items), len(b.sdActions))
	}
	if items[1].Label != "Slot 2: empty" || !items[5].Disabled {
		t.Errorf("with nothing in slot 2: %q, Remove disabled %v", items[1].Label, items[5].Disabled)
	}

	// Insert ZenZX's Card makes the card, with the examples, and puts it in.
	b.sdActions[4](zx)
	own := filepath.Join(dir, ownCardName)
	if zx.SecondCard() != own {
		t.Fatalf("slot 2 holds %q, want %q", zx.SecondCard(), own)
	}
	if st, err := os.Stat(own); err != nil || st.Size() != nextcard.SmallestCard {
		t.Fatalf("the card that was made: %v", err)
	}
	b.refreshStorageMenu(zx)
	items = b.widget.ItemsFor(barFloppyDisk)
	if items[1].Label != "Slot 2: "+ownCardName || items[5].Disabled {
		t.Errorf("with the card in: %q, Remove disabled %v", items[1].Label, items[5].Disabled)
	}
	// The boot card is not accepted for the second slot.
	b.insertCard(zx, filepath.Join(dir, nextcard.CardName))
	if zx.SecondCard() != own {
		t.Error("the card in slot 1 was put in slot 2 as well")
	}
	b.sdActions[5](zx) // Remove
	if zx.SecondCard() != "" || zx.io.divMMC.SD2() != nil {
		t.Error("Remove left a card in slot 2")
	}

	// New Empty Card: a size from its submenu, then a name; the card is made, is empty, and
	// goes into slot 2. A name that is taken is refused and the slot is left as it was.
	if sub := items[sdItemNewCard].SubItems; items[sdItemNewCard].Label != "New Empty Card" || len(sub) != len(newCardSizes) {
		t.Fatalf("item %d is %q with %d sizes", sdItemNewCard, items[sdItemNewCard].Label, len(sub))
	}
	b.storageSelected(zx, sdItemNewCard, 0)
	if b.diskDialog == nil || !b.dialogIsCard || b.newCardSize != nextcard.SmallestCard {
		t.Fatalf("choosing a size did not ask for a name (size %d)", b.newCardSize)
	}
	b.diskDialog, b.dialogIsCard, b.newCardSize = nil, false, 0
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

	// Away from the Next the Floppy Disk menu is back, as it was, and the slot is empty.
	b.sdActions[4](zx)
	zx.detachNextBootChain()
	if zx.SecondCard() != "" {
		t.Error("the second card stayed after the Next was left")
	}
	b.refreshStorageMenu(zx)
	if got := b.widget.LabelFor(barFloppyDisk); got != storageLabelFloppy || len(b.widget.ItemsFor(barFloppyDisk)) != floppyItems {
		t.Fatalf("after the Next the menu is %q with %d items", got, len(b.widget.ItemsFor(barFloppyDisk)))
	}
}
