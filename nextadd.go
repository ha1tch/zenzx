package zenzx

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// Programs for the Next: ZenZX's own card, -next-examples and -next-add.
//
// A Next program is started from NextZXOS, from an SD card. The card the Next boots from is
// the distribution's and is best left alone, and the Next has a second slot: ZenZX keeps a
// card of its own for it, zenzx-card.img, beside the boot files. -next-examples puts
// ZenZX's example programs (examples/next, which are inside this program) on that card and
// -next-add puts any files there; each makes the card if it is not there yet. A Next that
// boots NextZXOS starts with the card in its second slot when the file exists, and NextZXOS
// shows it as a drive of its own.

//go:embed examples/next/gallop/gallop.nex examples/next/fuji/fuji.nex examples/next/orbit/orbit.nex
var nextExampleFiles embed.FS

// ownCardName is ZenZX's own card, in the folder of the Next's boot files.
const ownCardName = "zenzx-card.img"

const (
	nextExamplesHelp = "Next: put ZenZX's example programs (gallop.nex, fuji.nex, orbit.nex) on ZenZX's own card, " + ownCardName + " beside the Next's boot files, making the card if needed, and exit. The Next starts with that card in its second slot"
	nextAddHelp      = "Next: put these files (separated by commas) on ZenZX's own card, as -next-examples does its examples, and exit"
	nextIssueHelp    = "Next: the board issue, which is how much memory DMA+ reaches by physical address: 5 (4 MB, the default), 4 or 2 (2 MB)"
	nextNewCardHelp  = "Next: make a new, empty SD card image at this path (FAT32, 280 MB, which takes little disk where files can have holes), and exit. Use it with -sdcard2 or the SD Card menu"
	nextExCardHelp   = "Next: write a standalone demo card with ZenZX's example programs (gallop.nex, fuji.nex, orbit.nex) to this path, and exit. Needs no boot files; the release ships this card beside the program"
	sdCard2Help      = "Next: the card image for the second SD slot (default: " + ownCardName + " beside the Next's boot files, when it is there; none for an empty slot)"
)

// nextExamples returns the example programs.
func nextExamples() ([]nextcard.File, error) {
	var files []nextcard.File
	for _, name := range []string{"gallop", "fuji", "orbit"} {
		data, err := nextExampleFiles.ReadFile("examples/next/" + name + "/" + name + ".nex")
		if err != nil {
			return nil, err
		}
		files = append(files, nextcard.File{Name: name + ".nex", Data: data})
	}
	return files, nil
}

// ownCardPath is where ZenZX's own card is, or would be: beside the boot files the Next
// would start from.
func ownCardPath(flagDir string) (string, error) {
	f, ok, note := locateNextFiles(flagDir)
	if !ok {
		if note != "" {
			return "", errors.New("the Next's boot files cannot be used (" + note + ")")
		}
		return "", errors.New("the Next's boot files were not found: get them first (the GUI offers to when the Next is chosen; -next-firmware setup does it from the command line)")
	}
	return filepath.Join(f.Dir, ownCardName), nil
}

// runNextAdd puts the examples, the files of list, or both on ZenZX's own card, making it
// when it is not there, and says what it did on w.
func runNextAdd(flagDir string, examples bool, list string, w io.Writer) error {
	card, err := ownCardPath(flagDir)
	if err != nil {
		return err
	}
	var files []nextcard.File
	if examples {
		ex, err := nextExamples()
		if err != nil {
			return err
		}
		files = append(files, ex...)
	}
	for _, path := range strings.Split(list, ",") {
		if path = strings.TrimSpace(path); path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, nextcard.File{Name: filepath.Base(path), Data: data})
	}
	if len(files) == 0 {
		return errors.New("no files to put on the card")
	}
	if err := addToOwnCard(card, files); err != nil {
		return err
	}
	fmt.Fprintf(w, "%s:\n", card)
	for _, file := range files {
		fmt.Fprintf(w, "  %s  %d bytes\n", file.Name, len(file.Data))
	}
	fmt.Fprintf(w, "A Next that boots NextZXOS now starts with this card in its second slot. In the Browser, D changes the drive; ENTER on a .nex file starts it.\n")
	return nil
}

// runNextExamplesCard writes a standalone demo card with ZenZX's example programs
// (gallop, fuji, orbit) to path, making a fresh FAT32 card there. Unlike -next-examples
// it does not need the Next's boot files: the card is self-contained, so the release
// can ship it in the `next` folder beside the program, where the boot files download
// to sit beside it. It says what it wrote on w.
func runNextExamplesCard(path string, w io.Writer) error {
	if path == "" {
		return errors.New("no path for the examples card")
	}
	files, err := nextExamples()
	if err != nil {
		return err
	}
	if err := addToOwnCard(path, files); err != nil {
		return err
	}
	fmt.Fprintf(w, "%s:\n", path)
	for _, file := range files {
		fmt.Fprintf(w, "  %s  %d bytes\n", file.Name, len(file.Data))
	}
	return nil
}

// addToOwnCard puts files in the top folder of the card at path, making the card first
// when there is none.
func addToOwnCard(path string, files []nextcard.File) error {
	if _, err := os.Stat(path); err != nil {
		root := nextcard.NewDir("")
		for _, f := range files {
			root.Add(f.Name, f.Data)
		}
		_, err := nextcard.NewCard(path, nextcard.SmallestCard, root)
		return err
	}
	return nextcard.AddFiles(context.Background(), path, "/", files, nil)
}

// insertOwnOrGivenCard puts a card in the second slot of a Next that has its boot chain:
// the one named with -sdcard2, or ZenZX's own when that is there. "none" leaves the slot
// empty. It returns a line to print, "" when the slot stays empty.
func (zx *ZenZX) insertOwnOrGivenCard(flag string, readOnly bool) (string, error) {
	if zx.next == nil || flag == "none" {
		return "", nil
	}
	path := flag
	if path == "" {
		path = filepath.Join(zx.next.files.Dir, ownCardName)
		if _, err := os.Stat(path); err != nil {
			return "", nil
		}
	}
	protected, err := zx.InsertSecondCard(path, readOnly)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Second SD card: %s (write-protected: %v)", path, protected), nil
}

// smallestCard is the size -next-new-card makes.
const smallestCard = nextcard.SmallestCard

// newEmptyCard makes an empty card image at path, with .img added to a name that has no
// extension, and returns the name it wrote. The file must not be there already.
func newEmptyCard(path string, size int64) (string, error) {
	if filepath.Ext(path) == "" {
		path += ".img"
	}
	_, err := nextcard.NewCard(path, size, nextcard.NewDir(""))
	return path, err
}
