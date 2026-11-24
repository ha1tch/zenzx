package zenzx

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The Next's boot chain as one thing: boot ROM, DivMMC ROM and SD card, found, attached and
// detached here for both front ends and for the menu's live model switch.
//
// A Next started without -bootrom, -divmmc-rom and -sdcard looks for the three files in a
// folder (nextFilesDirs gives the order). When they are there the machine boots NextZXOS
// from them. When they are not, the GUI offers to download them (nextsetup_gui.go) and the
// headless build says how (-next-firmware setup); without them the Next runs with the +3's ROMs, as
// it did before the boot chain existed. -next-classic asks for that outright.

// nextFilesEnv names a folder that holds the three files, as run_next_divmmc.sh and the
// Makefile set it.
const nextFilesEnv = "NEXT_FILES"

// nextFolder is the folder for the Next's files in a ZenZX tree: the boot files, the cards.
// thirdparty is where they went before there was one, and is still looked in.
const (
	nextFolder    = "next"
	nextFolderOld = "thirdparty"
)

// nextFilesDirs lists the folders searched for the boot files, in order: the one given
// with -next-files, the one NEXT_FILES names, ./next and the next folder beside the
// program (for a start from somewhere else, as from a desktop), ./thirdparty and the one
// beside the program, and the per-user folder.
func nextFilesDirs(flagDir string) []string {
	var dirs []string
	beside, besideOld := "", ""
	if exe, err := os.Executable(); err == nil {
		beside = filepath.Join(filepath.Dir(exe), nextFolder)
		besideOld = filepath.Join(filepath.Dir(exe), nextFolderOld)
	}
	for _, d := range []string{flagDir, os.Getenv(nextFilesEnv), nextFolder, beside, nextFolderOld, besideOld} {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	if d, err := nextcard.DefaultDir(); err == nil {
		dirs = append(dirs, d)
	}
	return dirs
}

// nextFilesUsable is the check locateNextFiles makes of a folder; a test replaces it.
var nextFilesUsable = nextcard.Usable

// locateNextFiles returns the first folder of nextFilesDirs that holds all three files in
// a state the Next can start from (nextcard.Usable). When there is none, note says why the
// first folder that did hold three files was passed over; it is empty when no folder held
// them at all.
func locateNextFiles(flagDir string) (f nextcard.Files, ok bool, note string) {
	for _, d := range nextFilesDirs(flagDir) {
		f := nextcard.In(d)
		if !f.Present() {
			continue
		}
		err := nextFilesUsable(d)
		if err == nil {
			return f, true, ""
		}
		if note == "" {
			note = d + ": " + err.Error()
		}
	}
	return nextcard.Files{}, false, note
}

// nextDownloadDir is where a download puts the files: the folder given with -next-files or
// NEXT_FILES when there is one; a `next` folder beside the program when there is one (a
// portable install, where the shipped demo card lives, so the boot files land beside it);
// ./next when the program is started where there is such a folder, as in a ZenZX tree; the
// per-user folder otherwise.
func nextDownloadDir(flagDir string) (string, error) {
	for _, d := range []string{flagDir, os.Getenv(nextFilesEnv)} {
		if d != "" {
			return d, nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		beside := filepath.Join(filepath.Dir(exe), nextFolder)
		if st, err := os.Stat(beside); err == nil && st.IsDir() {
			return beside, nil
		}
	}
	if st, err := os.Stat(nextFolder); err == nil && st.IsDir() {
		return nextFolder, nil
	}
	return nextcard.DefaultDir()
}

// nextChain is what attachNextBootChain leaves on the machine.
type nextChain struct {
	files     nextcard.Files
	card      *os.File
	protected bool

	// scratchDir is a temporary folder that holds the card, when it is one made to run a
	// program (nexrun.go); home is then the files the machine was started from, which a
	// second scratch card takes its system from, and which the person goes back to.
	scratchDir string
	home       *nextcard.Files

	// cachePending is set when the chain is attached and a boot cache is wanted: the next
	// frame boundary starts it (nextcacheuse.go).
	cachePending bool
}

// attachNextBootChain gives a machine that is already a Next its boot ROM, DivMMC and card,
// and leaves it in config mode with the boot ROM mapped; the caller resets the CPU. The
// card is opened for writing unless readOnly is set or the file does not allow it, and the
// result says whether it ended up write-protected.
func (zx *ZenZX) attachNextBootChain(f nextcard.Files, readOnly, debug bool) (protected bool, err error) {
	boot, err := os.ReadFile(f.BootROM)
	if err != nil {
		return false, err
	}
	rom, err := os.ReadFile(f.DivMMCROM)
	if err != nil {
		return false, err
	}
	card, protected, err := openSDCardImage(f.Card, readOnly)
	if err != nil {
		return false, err
	}
	info, err := card.Stat()
	if err != nil {
		card.Close()
		return false, err
	}
	zx.detachNextBootChain()
	zx.memory.SetBootROM(boot)
	zx.EnableDivMMC(card, info.Size(), rom)
	zx.io.divMMC.SD().wr.WriteProtected = protected
	zx.io.divMMC.Debug = debug
	zx.io.divMMC.SD().Debug = debug
	zx.next = &nextChain{files: f, card: card, protected: protected}
	zx.noteROMPatchesForFirmware()
	zx.armNextBootCache()
	return protected, nil
}

// detachNextBootChain takes the boot ROM, the DivMMC and the card away again, for a switch
// to another model. It does nothing when none was attached by attachNextBootChain.
func (zx *ZenZX) detachNextBootChain() {
	if zx.next == nil {
		return
	}
	zx.RemoveSecondCard()
	zx.cpu.M1Hook = nil
	zx.io.divMMC = nil
	zx.io.hasDivMMC = false
	zx.memory.divMMCReadFn = nil
	zx.memory.divMMCWriteFn = nil
	zx.memory.bootROMLoaded = false
	zx.memory.bootROMEnabled = false
	zx.memory.nextCfg.active = false
	zx.next.card.Close()
	if zx.next.scratchDir != "" {
		os.RemoveAll(zx.next.scratchDir)
	}
	zx.next = nil
}

// describeNextChain is the start-up line for an attached chain.
func describeNextChain(f nextcard.Files, protected bool) string {
	return fmt.Sprintf("Next boot chain attached from %s: %s, %s, %s (write-protected: %v)",
		f.Dir, nextcard.BootROMName, nextcard.DivMMCROMName, filepath.Base(f.Card), protected)
}

// nextSetupHint is what a front end prints when a Next has no boot files and nobody is
// there to be asked.
func nextSetupHint(binary, note string) string {
	dir, _ := nextDownloadDir("")
	if note != "" {
		note = "Next boot files found but not usable: " + note + "\n"
	}
	return note + "No usable Next boot files found (" + nextcard.BootROMName + ", " + nextcard.DivMMCROMName + ", " + nextcard.CardName +
		"): the Next runs with the +3's ROMs.\n  To download them once into " + dir + " and boot NextZXOS from then on:  " + binary + " -next-firmware setup"
}

// wantNextBootChain reports whether a front end should look for the boot files itself: the
// model is a Next, -next-classic was not given, the chain was not given file by file
// (-bootrom, -divmmc-rom, -sdcard), and nothing is being loaded into the machine at start
// (-snapshot, -bin, -tape, -scr), which wants the machine as it is and not NextZXOS.
func wantNextBootChain(model string, classic bool, explicit ...string) bool {
	if model != "next" || classic {
		return false
	}
	for _, e := range explicit {
		if e != "" {
			return false
		}
	}
	return true
}

// Help text of the flags the two front ends share.
const (
	nextFilesHelp   = "Next: folder that holds nextboot.rom, enNxtmmc.rom and next-card.img. Without it they are looked for in the folder NEXT_FILES names, in ./next, in ./thirdparty and in the per-user folder"
	nextClassicHelp = "Next: do not boot NextZXOS from the boot files; run with the +3's ROMs, as -model=next does when they are absent"
)
