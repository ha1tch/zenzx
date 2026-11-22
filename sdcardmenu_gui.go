//go:build !headless

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The SD Card menu.
//
// A Next that boots NextZXOS has no use for the Floppy Disk menu: what it reads and writes
// is on its SD cards. While such a machine is running, that place in the bar is the SD Card
// menu, which shows what is in the two slots and changes the second: any card image,
// ZenZX's own card with the example programs (nextadd.go), or a new empty one. The first slot holds the card
// NextZXOS boots from and is not changed here.
//
// NextZXOS looks at the slots when it starts and when its Browser is told to remount (M,
// then Y to confirm), so after a change one of those is needed; what the menu prints says so.

const (
	storageLabelFloppy = "Floppy Disk"
	storageLabelSD     = "SD Card"
)

// usesSDCards reports whether the machine is one the SD Card menu is for.
func usesSDCards(zx *ZenZX) bool {
	return zx.next != nil && zx.io.hasDivMMC && zx.io.divMMC != nil
}

// refreshStorageMenu makes the bar's fourth menu the one the machine has a use for. Called
// each frame the bar is shown, as the other menus whose contents change are.
func (b *appMenuBar) refreshStorageMenu(zx *ZenZX) {
	if !usesSDCards(zx) {
		if b.storageIsSD {
			b.storageIsSD = false
			b.widget.SetLabel(barFloppyDisk, storageLabelFloppy)
			b.widget.SetItems(barFloppyDisk, b.diskItems)
		}
		return
	}
	second := zx.SecondCard()
	slot2 := "Slot 2: empty"
	if second != "" {
		slot2 = "Slot 2: " + filepath.Base(second)
	}
	items := []zenui.Item{
		{Label: "Slot 1: " + filepath.Base(zx.next.files.Card), Disabled: true},
		{Label: slot2, Disabled: true},
		{Separator: true},
		{Label: "Insert Card in Slot 2..."},
		{Label: "Insert ZenZX's Card (Examples)"},
		{Label: "Remove Card from Slot 2", Disabled: second == ""},
		{Label: "New Empty Card", SubItems: newCardSizeItems()},
		{Separator: true},
		{Label: "Card Info"},
		{Label: "Manage Cards..."},
	}
	b.sdActions = []func(zx *ZenZX){
		nil, nil, nil,
		func(zx *ZenZX) { b.openCardDialog(zx) },
		func(zx *ZenZX) { b.insertOwnCard(zx) },
		func(zx *ZenZX) {
			zx.RemoveSecondCard()
			fmt.Println("SD card: slot 2 is empty. " + remountHint)
		},
		nil, // New Empty Card: a submenu, see storageSelected
		nil,
		func(zx *ZenZX) { b.showInfo("CARD INFO", cardInfoText(zx)) }, // info_gui.go
		func(zx *ZenZX) { b.openCards(zx) },
	}
	if !b.storageIsSD {
		b.storageIsSD = true
		b.widget.SetLabel(barFloppyDisk, storageLabelSD)
	}
	b.widget.SetItems(barFloppyDisk, items)
}

// sdItemNewCard is New Empty Card's place in the SD Card menu.
const sdItemNewCard = 6

// newCardSizes are the sizes New Empty Card offers. The first is the smallest a FAT32 card
// can be; the second is the size of the card the Next boots from. Either takes little
// disk until it is written to, where the file system lets files have holes.
var newCardSizes = []struct {
	label string
	bytes int64
}{
	{"280 MB", nextcard.SmallestCard},
	{"1 GB", 1 << 30},
}

func newCardSizeItems() []zenui.Item {
	items := make([]zenui.Item, len(newCardSizes))
	for i, s := range newCardSizes {
		items[i] = zenui.Item{Label: s.label}
	}
	return items
}

// storageSelected carries out a choice from the SD Card menu.
func (b *appMenuBar) storageSelected(zx *ZenZX, item, sub int) {
	if item == sdItemNewCard {
		if sub >= 0 && sub < len(newCardSizes) {
			b.openNewCardDialog(zx, newCardSizes[sub].bytes)
		}
		return
	}
	if item >= 0 && item < len(b.sdActions) && b.sdActions[item] != nil {
		b.sdActions[item](zx)
	}
}

// openNewCardDialog asks where the new card is to go, in the folder of the Next's files.
func (b *appMenuBar) openNewCardDialog(zx *ZenZX, size int64) {
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:       zenui.ModeSave,
		Title:      "New Empty SD Card Image",
		StartDir:   zx.next.files.Dir,
		Filters:    []string{"img"},
		DefaultExt: "img",
		FS:         zenui.OSFS{},
	})
	b.dialogIsCard, b.newCardSize = true, size
}

// newCard makes an empty card at path and puts it in the second slot.
func (b *appMenuBar) newCard(zx *ZenZX, path string, size int64) {
	path, err := newEmptyCard(path, size)
	if err != nil {
		fmt.Printf("SD card: %s could not be made: %v\n", path, err)
		return
	}
	fmt.Printf("SD card: made %s, empty, %d MB\n", path, size>>20)
	b.insertCard(zx, path)
}

const remountHint = "NextZXOS sees the change after M and then Y in its Browser (remount), or after a reset."

// openCardDialog opens the file browser for a card image, in the folder of the Next's files.
func (b *appMenuBar) openCardDialog(zx *ZenZX) {
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:     zenui.ModeOpen,
		Title:    "Insert SD Card Image in Slot 2",
		StartDir: zx.next.files.Dir,
		Filters:  []string{"img"},
		FS:       zenui.OSFS{},
	})
	b.dialogIsCard, b.newCardSize = true, 0
}

// insertCard puts a card image in the second slot and says what happened.
func (b *appMenuBar) insertCard(zx *ZenZX, path string) {
	if !usesSDCards(zx) {
		return
	}
	if same(path, zx.next.files.Card) {
		fmt.Println("SD card: that is the card in slot 1")
		return
	}
	if u := b.whereIs(path); u.machine != 0 && u.machine != b.numberOf(zx) {
		fmt.Printf("SD card: %s is in slot %d of machine %d; a card is in one machine at a time\n", path, u.slot, u.machine)
		return
	}
	protected, err := zx.InsertSecondCard(path, b.nextReadOnly)
	if err != nil {
		fmt.Printf("SD card: %s could not be used: %v\n", path, err)
		return
	}
	fmt.Printf("SD card: %s is in slot 2 (write-protected: %v). %s\n", path, protected, remountHint)
}

// insertOwnCard puts ZenZX's own card in the second slot, making it with the example
// programs on it when it is not there yet.
func (b *appMenuBar) insertOwnCard(zx *ZenZX) {
	if !usesSDCards(zx) {
		return
	}
	path := filepath.Join(zx.next.files.Dir, ownCardName)
	if _, err := os.Stat(path); err != nil {
		examples, err := nextExamples()
		if err == nil {
			err = addToOwnCard(path, examples)
		}
		if err != nil {
			fmt.Printf("SD card: %s could not be made: %v\n", path, err)
			return
		}
		fmt.Printf("SD card: made %s with the example programs on it\n", path)
	}
	b.insertCard(zx, path)
}

func same(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && x == y
}
