//go:build !headless

package zenzx

import (
	"fmt"
	"os"
	"strings"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The Info items of the Tape, Storage and Snapshot menus.
//
// Each used to print a line or two to the terminal, which someone who
// started the program from an icon never sees. Each now opens a small
// window with the text (the reader Help and About use), and still prints
// it, for a session that is being logged.
//
// The text is in the reader's own markup: "## " begins a heading, and a
// line that begins with two spaces is drawn dimmer. Lines are kept under
// 70 columns, which is what the reader's panel holds.

// infoLabelWidth is the width of the label column of an info window.
const infoLabelWidth = 16

// infoLine is one "label  value" line of an info window.
func infoLine(label, value string) string {
	return fmt.Sprintf("  %-*s%s", infoLabelWidth, label, value)
}

// infoPath shortens a file's path to what fits beside a label, keeping the
// end, which is the part that tells one file from another.
func infoPath(path string) string {
	const room = 68 - 2 - infoLabelWidth
	if len(path) <= room {
		return path
	}
	return "..." + path[len(path)-room+3:]
}

// showInfo opens an info window with text under title, and prints the text
// as well.
func (b *appMenuBar) showInfo(title, text string) {
	fmt.Printf("%s\n%s\n", title, strings.ReplaceAll(text, "## ", ""))
	// A short text gets a panel of its own height; a long one (a tape of
	// many blocks) gets the tall panel and scrolls.
	short := strings.Count(text, "\n") < 18
	b.activeModal = zenui.NewTextModal(title, text, short)
}

// tapeInfoText describes the tape in the machine's deck.
func tapeInfoText(zx *ZenZX) string {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return "## Tape\n\nNo tape is loaded.\n\nDrop a .tap or .tzx file on the window, or start with -tape."
	}
	st := zx.tape.st
	mode := map[TapeMode]string{TapeFast: "fast", TapeAccurate: "accurate", TapeTurbo: "turbo"}[st.Mode]
	state := "stopped"
	if st.Playing {
		state = "playing"
	}
	lines := []string{
		"## Tape",
		"",
		infoLine("File", infoPath(st.Filename)),
		infoLine("Mode", mode),
		infoLine("State", fmt.Sprintf("%s, at block %d of %d", state, min(st.Position+1, len(st.Blocks)), len(st.Blocks))),
	}
	if name := zx.FirstProgram(); name != "" {
		lines = append(lines, infoLine("First program", name))
	}
	size := 0
	for _, blk := range st.Blocks {
		size += len(blk.Data)
	}
	lines = append(lines, infoLine("Size", fmt.Sprintf("%d bytes in %d blocks", size, len(st.Blocks))))
	lines = append(lines, "", "## Blocks", "")
	for _, info := range zx.tape.GetBlockInfo() {
		lines = append(lines, "  "+strings.TrimPrefix(info, "  "))
	}
	return strings.Join(lines, "\n")
}

// diskInfoText describes the floppy drive and the disk in it.
func diskInfoText(zx *ZenZX) string {
	if !zx.io.hasFDC || zx.io.fdc == nil {
		return "## Floppy disk\n\nThis model has no floppy drive.\n\nThe +3 and the Spanish +3 have one."
	}
	fdc := zx.io.fdc
	if !fdc.diskPresent {
		return "## Floppy disk\n\nDrive A: is empty.\n\nLoad Disk... or Insert Blank Disk puts a disk in it."
	}
	file := "none: a blank disk, not saved yet"
	if fdc.diskFilename != "" {
		file = infoPath(fdc.diskFilename)
	}
	// A disk read from a file has the file's own geometry; a blank one is
	// the +3's standard disk.
	format := "40 tracks, one side (the +3's standard format)"
	if fdc.disk != nil {
		sides := "one side"
		if fdc.disk.NumSides == 2 {
			sides = "two sides"
		}
		format = fmt.Sprintf("%d tracks, %s", fdc.disk.NumTracks, sides)
	}
	changes := "none since it was loaded or saved"
	if fdc.diskModified {
		changes = "unsaved: in memory, not in the file yet"
	}
	lines := []string{
		"## Floppy disk, drive A:",
		"",
		infoLine("File", file),
		infoLine("Format", format),
		infoLine("Changes", changes),
	}
	if fdc.diskFilename != "" {
		if st, err := os.Stat(fdc.diskFilename); err == nil {
			lines = append(lines, infoLine("File size", fmt.Sprintf("%d KB", (st.Size()+1023)/1024)))
		}
		if fdc.FileChanged() {
			lines = append(lines, infoLine("On disk", "the file has changed since it was loaded"))
		}
	}
	if name := zx.FirstProgram(); name != "" {
		lines = append(lines, infoLine("First program", name))
	}
	lines = append(lines, "", "The disk is held in memory while the machine uses it;",
		"Save Disk writes it to the file, and Manage Disks... has the rest.")
	return strings.Join(lines, "\n")
}

// cardSlotText describes one SD card slot's card.
func cardSlotText(slot int, path string, protected bool) []string {
	lines := []string{fmt.Sprintf("## Slot %d", slot), "", infoLine("File", infoPath(path))}
	f, err := os.Open(path)
	if err != nil {
		why := err.Error()
		if pe, ok := err.(*os.PathError); ok {
			why = pe.Err.Error() // the path is on the line above already
		}
		return append(lines, infoLine("State", "the file cannot be read: "+why))
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil {
		lines = append(lines, infoLine("Size", fmt.Sprintf("%d MB", st.Size()>>20)))
	}
	write := "can be written to"
	if protected {
		write = "write-protected"
	}
	lines = append(lines, infoLine("Writing", write))
	vol, err := nextcard.Open(f)
	if err != nil {
		return append(lines, infoLine("Contents", "not a FAT32 card ZenZX can read: "+err.Error()))
	}
	files, bytes := 0, int64(0)
	vol.Root.Walk(func(_ string, n *nextcard.Node) {
		if !n.IsDir() {
			files++
			bytes += int64(n.Size)
		}
	})
	lines = append(lines, infoLine("Contents", fmt.Sprintf("%d files, %d KB", files, (bytes+1023)/1024)))
	boots := "yes"
	if err := nextcard.VerifyCard(f); err != nil {
		boots = "no: " + err.Error()
	}
	for i, part := range zenui.WrapText(boots, 68-2-infoLabelWidth, "") {
		label := ""
		if i == 0 {
			label = "Boots a Next"
		}
		lines = append(lines, infoLine(label, part))
	}
	return lines
}

// cardInfoText describes the machine's two SD card slots.
func cardInfoText(zx *ZenZX) string {
	if !usesSDCards(zx) {
		return "## SD cards\n\nThis machine has no card slots.\n\nA ZX Spectrum Next booting NextZXOS has two."
	}
	lines := cardSlotText(1, zx.next.files.Card, zx.next.protected)
	lines = append(lines, "")
	if zx.card2 == nil {
		lines = append(lines, "## Slot 2", "", "  Empty.")
	} else {
		lines = append(lines, cardSlotText(2, zx.card2.path, zx.card2.protected)...)
	}
	lines = append(lines, "", "What a card holds is read from its file as it is now: a machine",
		"that is writing to it may be a moment ahead of what is shown.")
	return strings.Join(lines, "\n")
}

// snapshotFileLine is one snapshot file as the Snapshot Info window lists it: its path, size
// and when it was written. ok is false when the file is not there.
func snapshotFileLine(label, path string) (line string, ok bool) {
	st, err := os.Stat(path)
	if err != nil {
		return infoLine(label, "none yet"), false
	}
	return infoLine(label, fmt.Sprintf("%s, %d KB, %s", infoPath(path), (st.Size()+1023)/1024,
		st.ModTime().Format("2006-01-02 15:04"))), true
}

// snapshotInfoText says what snapshots there are to load or save, and how.
func snapshotInfoText(zx *ZenZX) string {
	format := zx.snapshotFormat
	if format == "" || format == "auto" {
		format = "zxs"
	}
	lines := []string{"## Here", "",
		infoLine("Folder", snapshotDir+"/ (Manage Snapshots lists what is in it)"),
		infoLine("Saves in", strings.ToUpper(format)+", unless Save As says another")}
	for _, ext := range snapshotFormats {
		if line, ok := snapshotFileLine("Quick save ."+ext, snapshotPath(quickSaveName(ext))); ok {
			lines = append(lines, line)
		}
	}
	if len(lines) == 4 {
		lines = append(lines, infoLine("Quick save", "none yet: Quick Save writes "+snapshotPath(quickSaveName(format))))
	}
	// The newest of the timestamped saves in the snapshots folder.
	stamped := timestampedSnapshots(listSnapshots(snapshotDir))
	if len(stamped) == 0 {
		lines = append(lines, infoLine("Timestamped", "none yet"))
	} else {
		lines = append(lines, infoLine("Timestamped", fmt.Sprintf("%d saved; the newest", len(stamped))))
		line, _ := snapshotFileLine("", stamped[0].Path)
		lines = append(lines, line)
	}
	lines = append(lines, "", "## Loading", "",
		infoLine("From a file", "drop a .zxs, .sna, .z80 or .szx file on the window,"),
		infoLine("", "or start with -snapshot (a .rzx plays the same way)"),
		infoLine("Quick Load", "reads the quick save back, in any of the formats"),
		infoLine("The machine", "one of its kind (48K, 128K or +3) is made for it,"),
		infoLine("", "and is the one that starts when none is running"),
		"", "## Saving", "",
		infoLine("Quick Save", "writes "+snapshotDir+"/quicksave.<format>"),
		infoLine("Timestamped", "writes "+snapshotDir+"/snapshot_<date>_<time>.<format>"),
		infoLine("Save As", "the format (.zxs, .sna or .z80), then the name"),
		infoLine("Load Timestamped", "the newest ten, and More... for the rest"),
		"", "Run Diagnostics reports on the running machine.")
	return strings.Join(lines, "\n")
}
