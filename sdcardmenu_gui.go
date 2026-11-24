//go:build !headless

package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The SD Card menu.
//
// A Next that boots NextZXOS reads and writes mainly on its SD cards, so while such a machine
// is running, that place in the bar is the SD Card menu rather than the Floppy Disk menu --
// the two are never both on the bar at once. It shows what is in the two slots and changes the
// second: any card image, ZenZX's own card with the example programs (nextadd.go), or a new
// empty one. The first slot holds the card NextZXOS boots from and is not changed here.
//
// A Next is not drive-less, though: NextZXOS can mount a .dsk as one of its own drives (A:/B:).
// ZenZX does not emulate that yet (T-72), so the menu ends with a disabled "Virtual Floppy"
// section that marks where those actions will live -- the capability is signposted, not hidden,
// which is why a Next no longer simply loses the Floppy Disk menu whole.
//
// NextZXOS looks at the slots when it starts and when its Browser is told to remount (M,
// then Y to confirm), so after a change one of those is needed; what the menu prints says so.

// storageLabel is the one bar label for the storage menu, the same on every
// machine -- the menu's contents change with the machine (the +3's floppy drive,
// or a Next's SD cards and virtual floppy), but the label does not swap under the
// user. The old per-machine labels ("Floppy Disk", "SD Card") are now Title
// headings inside the menu instead (see refreshStorageMenu and diskItems).
const storageLabel = "Storage"

// usesSDCards reports whether the machine is one the SD Card menu is for.
func usesSDCards(zx *ZenZX) bool {
	return zx.next != nil && zx.io.hasDivMMC && zx.io.divMMC != nil
}

// refreshStorageMenu makes the bar's fourth menu the one the machine has a use for. Called
// each frame the bar is shown, as the other menus whose contents change are.
func (b *appMenuBar) refreshStorageMenu(zx *ZenZX) {
	spec := b.storageMenuSpec(zx)
	b.storageSpec = spec
	b.storageIsSD = usesSDCards(zx)
	b.actions[barFloppyDisk] = spec.actions()
	items := spec.items()
	b.widget.SetLabel(barFloppyDisk, storageLabel)
	if sig := storageSignature(items); sig != b.storageSig {
		b.storageSig = sig
		b.widget.SetItems(barFloppyDisk, items)
	}
}

// storageSignature is what the user can see of the menu, to tell whether it changed.
func storageSignature(items []zenui.Item) string {
	var sb strings.Builder
	for _, it := range items {
		fmt.Fprintf(&sb, "%s|%v|%v|%d\n", it.Label, it.Disabled, it.Title, len(it.SubItems))
	}
	return sb.String()
}

// storageMenuSpec is the Storage menu of the machine as it is now. Every machine has the
// same two sections in the same order, Floppy Disk and then SD Card: a section is live where
// the machine has that hardware, and where it has not it stays, disabled, with a line that
// says why -- the +3 has the drive, a Next the cards, a 48K, 128K, +2 or +2A neither. The
// menu does not change shape under the user, and a drive-less model has no live row that can
// only say "FDC not available".
func (b *appMenuBar) storageMenuSpec(zx *ZenZX) menuSpec {
	spec := b.floppySection(zx)
	spec = append(spec, menuSep())
	return append(spec, b.sdSection(zx)...)
}

// floppySection is the +3's drive: live when the machine has a floppy controller.
func (b *appMenuBar) floppySection(zx *ZenZX) menuSpec {
	if zx.io.hasFDC && zx.io.fdc != nil {
		return menuSpec{
			menuTitle("Floppy Disk"),
			menuRow("Open DSK Image...", func(zx *ZenZX) { b.openDiskDialog(zx) }),
			menuRow("Insert Blank Disk", func(zx *ZenZX) { zx.handleInsertBlankDisk() }),
			menuSep(),
			menuRow("Save Disk", func(zx *ZenZX) { zx.handleSaveDisk() }),
			menuRow("Save Disk As...", func(zx *ZenZX) { zx.handleSaveDiskAs() }),
			menuSep(),
			menuRow("Eject Disk", func(zx *ZenZX) { zx.handleEjectDisk() }),
			menuRow("Disk Info", func(zx *ZenZX) { b.showInfo("DISK INFO", diskInfoText(zx)) }),
			menuRow("Manage Disks...", func(zx *ZenZX) { b.openDisks(zx) }),
		}
	}
	if usesSDCards(zx) {
		// A Next reaches storage through its SD cards, but NextZXOS can also mount a .dsk as
		// one of its own drives (A:/B:). ZenZX does not emulate that yet (T-72), so the section
		// stands as disabled signposts -- the capability is shown where it will live, and lights
		// up when T-72 lands.
		return menuSpec{
			menuTitle("Floppy Disk"),
			{label: "    Mount DSK as Drive A...", disabled: true},
			{label: "    Mount DSK as Drive B...", disabled: true},
			{label: "    Unmount Drive", disabled: true},
		}
	}
	return menuSpec{menuTitle("Floppy Disk"), {label: sectionAbsent, disabled: true}}
}

// sectionAbsent is the one line a Storage section shows where the machine has none of that
// hardware, or has it switched off: not applicable, with no reason given.
const sectionAbsent = "    N/A"

// sdSection is the Next's SD cards: live while a Next that boots NextZXOS runs.
func (b *appMenuBar) sdSection(zx *ZenZX) menuSpec {
	if !usesSDCards(zx) {
		return menuSpec{menuTitle("SD Card"), {label: sectionAbsent, disabled: true}}
	}
	second := zx.SecondCard()
	slot2 := "Slot 2: empty"
	if second != "" {
		slot2 = "Slot 2: " + filepath.Base(second)
	}
	return menuSpec{
		menuTitle("SD Card"),
		{label: "Slot 1: " + filepath.Base(zx.next.files.Card), disabled: true},
		{label: slot2, disabled: true},
		menuSep(),
		menuRow("Insert Card in Slot 2...", func(zx *ZenZX) { b.openCardDialog(zx) }),
		menuRow("Insert ZenZX's Card (Examples)", func(zx *ZenZX) { b.insertOwnCard(zx) }),
		{label: "Remove Card from Slot 2", disabled: second == "", run: func(zx *ZenZX) {
			zx.RemoveSecondCard()
			zx.notice("Slot 2 is empty. %s", remountHint)
		}},
		{label: "New Empty Card", sub: b.newCardSizeSpec()},
		menuSep(),
		menuRow("Card Info", func(zx *ZenZX) { b.showInfo("CARD INFO", cardInfoText(zx)) }), // info_gui.go
		menuRow("Manage Cards...", func(zx *ZenZX) { b.openCards(zx) }),
	}
}

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

// newCardSizeSpec is the submenu of New Empty Card: one row per size, each opening the
// dialog that names the new card.
func (b *appMenuBar) newCardSizeSpec() menuSpec {
	spec := make(menuSpec, len(newCardSizes))
	for i, s := range newCardSizes {
		bytes := s.bytes
		spec[i] = menuRow(s.label, func(zx *ZenZX) { b.openNewCardDialog(zx, bytes) })
	}
	return spec
}

// storageSelected carries out a choice from the SD Card menu.
func (b *appMenuBar) storageSelected(zx *ZenZX, item, sub int) {
	b.storageSpec.pick(zx, item, sub)
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
	b.dialogKind, b.newCardSize = dialogNewCard, size
}

// newCard makes an empty card at path and puts it in the second slot.
func (b *appMenuBar) newCard(zx *ZenZX, path string, size int64) {
	path, err := newEmptyCard(path, size)
	if err != nil {
		zx.notice("The card %s could not be made: %v", filepath.Base(path), err)
		return
	}
	zx.notice("Made the card %s, empty, %d MB", filepath.Base(path), size>>20)
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
	b.dialogKind, b.newCardSize = dialogInsertCard, 0
}

// insertCard puts a card image in the second slot and says what happened.
func (b *appMenuBar) insertCard(zx *ZenZX, path string) {
	if !usesSDCards(zx) {
		return
	}
	if same(path, zx.next.files.Card) {
		zx.notice("That is the card in slot 1")
		return
	}
	if u := b.whereIs(path); u.machine != 0 && u.machine != b.numberOf(zx) {
		zx.notice("%s is in slot %d of machine %d: a card is in one machine at a time", filepath.Base(path), u.slot, u.machine)
		return
	}
	protected, err := zx.InsertSecondCard(path, b.nextReadOnly)
	if err != nil {
		zx.notice("The card %s could not be used: %v", filepath.Base(path), err)
		return
	}
	zx.notice("%s is in slot 2 (write-protected: %v). %s", filepath.Base(path), protected, remountHint)
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
			zx.notice("The card %s could not be made: %v", filepath.Base(path), err)
			return
		}
		zx.notice("Made the card %s with the example programs on it", filepath.Base(path))
	}
	b.insertCard(zx, path)
}

func same(a, b string) bool {
	x, err1 := filepath.Abs(a)
	y, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && x == y
}
