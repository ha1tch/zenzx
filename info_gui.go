//go:build !headless

package zenzx

import (
	"fmt"
	"os"
	"strings"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The Info items of the Tape, Floppy Disk, SD Card and Snapshot menus.
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
	lines = append(lines, "", "## Blocks", "")
	for _, info := range zx.tape.GetBlockInfo() {
		lines = append(lines, "  "+strings.TrimPrefix(info, "  "))
	}
	return strings.Join(lines, "\n")
}

// diskInfoText describes the floppy drive and the disc in it.
func diskInfoText(zx *ZenZX) string {
	if !zx.io.hasFDC || zx.io.fdc == nil {
		return "## Floppy disk\n\nThis model has no floppy drive.\n\nThe +3 and the Spanish +3 have one."
	}
	fdc := zx.io.fdc
	if !fdc.diskPresent {
		return "## Floppy disk\n\nDrive A: is empty.\n\nOpen DSK Image... or Insert Blank Disk puts a disc in it."
	}
	file := "none: a blank disc, not saved yet"
	if fdc.diskFilename != "" {
		file = infoPath(fdc.diskFilename)
	}
	// A disc read from a file has the file's own geometry; a blank one is
	// the +3's standard disc.
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
	lines = append(lines, "", "The disc is held in memory while the machine uses it;",
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

// snapshotInfoText says how snapshots are loaded and saved.
func snapshotInfoText() string {
	return strings.Join([]string{
		"## Snapshots",
		"",
		"A snapshot is the whole machine at one moment, in a file.",
		"",
		infoLine("Load", "drop a .zxs, .sna, .z80 or .szx file on the window,"),
		infoLine("", "or start with -snapshot"),
		infoLine("Quick Save", "writes quicksave.zxs, or the format -format names;"),
		infoLine("", "Quick Load reads it back"),
		infoLine("Timestamped", "writes a new file named for the date and time"),
		"",
		"Run Diagnostics prints a report on the running machine's",
		"state to the terminal.",
	}, "\n")
}
