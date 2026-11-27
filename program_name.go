package zenzx

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// ============================================================================
// The first BASIC program a machine loads
//
// A host that shows several machines names each after the first BASIC
// program it loads, from whichever of its media that comes: tape, the +3's
// disk, a Next's SD card. Each medium notices in its own way and
// FirstProgram puts them together.
//
//   - Tape: the first Program header on the tape, once the tape has moved.
//     It is taken from the tape and not from the loading, because the
//     machine can load in three ways (the ROM's routine trapped, pulses, the
//     turbo path) and all three start from that header.
//   - Disk: a sector that begins a +3DOS file whose header says Program; the
//     name is the directory entry whose first block that sector is.
//   - SD card: a block that begins such a file; the name is the file of the
//     FAT volume that starts there. NextZXOS runs autoexec.1st and
//     autoexec.bas when it starts, which is the system's doing and not a
//     program the machine was asked to load, so those are passed over.
// ============================================================================

// plus3DOSProgram reports whether b is the start of a +3DOS file that holds
// a BASIC program: the signature, the soft end-of-file byte, and Program as
// the type in the BASIC header that follows.
func plus3DOSProgram(b []byte) bool {
	return len(b) >= 16 && string(b[:8]) == "PLUS3DOS" && b[8] == 0x1A && b[15] == 0
}

// programName is a file's name as a machine's: its base name without the
// .bas a disk or a card gives it, and without the spaces a tape pads it
// with. Names that are nothing but that come back empty.
func programName(file string) string {
	name := strings.TrimSpace(filepath.Base(strings.ReplaceAll(file, "\\", "/")))
	if ext := filepath.Ext(name); strings.EqualFold(ext, ".bas") {
		name = strings.TrimSpace(strings.TrimSuffix(name, ext))
	}
	if name == "." || name == "/" {
		return ""
	}
	return name
}

// FirstProgram is the name of the first BASIC program the machine has
// loaded, "" while it has loaded none. Once there is one it does not change.
func (zx *ZenZX) FirstProgram() string {
	if zx.firstProgram != "" {
		return zx.firstProgram
	}
	if n := zx.tapeProgram(); n != "" {
		zx.firstProgram = n
	} else if zx.io.fdc != nil && zx.io.fdc.firstProgram != "" {
		zx.firstProgram = zx.io.fdc.firstProgram
	} else if n := zx.cardProgram(); n != "" {
		zx.firstProgram = n
	}
	return zx.firstProgram
}

// tapeProgram is the first Program header of a tape that has moved.
func (zx *ZenZX) tapeProgram() string {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return ""
	}
	// Moved: a block delivered, or pulses played. A tape that has only been
	// set playing has loaded nothing yet.
	if zx.tape.st.Position == 0 {
		return ""
	}
	return zx.tape.firstProgramName()
}

// systemStartUp reports whether a file on a card is one NextZXOS runs by
// itself when it starts: autoexec.1st and autoexec.bas, wherever they are.
func systemStartUp(path string) bool {
	return strings.HasPrefix(strings.ToLower(filepath.Base(path)), "autoexec.")
}

// cardProgram looks up the programs the SD cards have seen begin, oldest
// first, and returns the first that is not NextZXOS's own start-up file.
func (zx *ZenZX) cardProgram() string {
	if zx.io.divMMC == nil {
		return ""
	}
	type slot struct {
		card *SDCard
		img  io.ReaderAt
	}
	var slots []slot
	if zx.next != nil && zx.next.card != nil {
		slots = append(slots, slot{zx.io.divMMC.SD(), zx.next.card})
	}
	if zx.card2 != nil && zx.io.divMMC.SD2() != nil {
		slots = append(slots, slot{zx.io.divMMC.SD2(), zx.card2.file})
	}
	for _, s := range slots {
		if s.card == nil || len(s.card.programAt) == 0 {
			continue
		}
		pending := s.card.programAt
		s.card.programAt = nil
		vol, err := nextcard.Open(s.img)
		if err != nil {
			continue
		}
		for _, offset := range pending {
			path, ok := vol.FileAt(offset)
			if !ok || systemStartUp(path) {
				continue
			}
			if n := programName(path); n != "" {
				return n
			}
		}
	}
	return ""
}
