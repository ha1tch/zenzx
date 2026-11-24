package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// plus3DOSFile is the first bytes of a +3DOS file of the given BASIC type.
func plus3DOSFile(kind byte) []byte {
	b := make([]byte, 512)
	copy(b, "PLUS3DOS")
	b[8], b[9], b[10] = 0x1A, 1, 0
	b[15] = kind
	return b
}

func TestProgramName(t *testing.T) {
	for in, want := range map[string]string{
		"MANIC     ":             "MANIC",
		"GAME.BAS":               "GAME",
		"/demos/intro.bas":       "intro",
		"loader":                 "loader",
		"MENU.B":                 "MENU.B",
		"   ":                    "",
		"":                       "",
		"/nextzxos/autoexec.bas": "autoexec",
	} {
		if got := programName(in); got != want {
			t.Errorf("programName(%q) = %q, want %q", in, got, want)
		}
	}
	for path, want := range map[string]bool{"/nextzxos/autoexec.1st": true, "/NEXTZXOS/AUTOEXEC.BAS": true, "/demos/autoexec2.bas": false, "/games/manic.bas": false} {
		if got := systemStartUp(path); got != want {
			t.Errorf("systemStartUp(%q) = %v, want %v", path, got, want)
		}
	}
	if !plus3DOSProgram(plus3DOSFile(0)) {
		t.Error("the start of a +3DOS BASIC program is not recognised")
	}
	if plus3DOSProgram(plus3DOSFile(3)) || plus3DOSProgram([]byte("PLUS3DOS")) || plus3DOSProgram(make([]byte, 512)) {
		t.Error("something that is not the start of a BASIC program is taken for one")
	}
}

func TestFirstProgramFromTape(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	zx.tape.st.Loaded = true
	zx.tape.st.Blocks = []TapeBlock{
		{IsHeader: true, HeaderType: tap.TypeCode, HeaderName: "screen"},
		{},
		{IsHeader: true, HeaderType: tap.TypeProgram, HeaderName: "MANIC"},
		{IsHeader: true, HeaderType: tap.TypeProgram, HeaderName: "second"},
	}
	if got := zx.FirstProgram(); got != "" {
		t.Fatalf("a tape that has not moved named the machine %q", got)
	}
	zx.tape.st.Position = 1
	if got := zx.FirstProgram(); got != "MANIC" {
		t.Fatalf("FirstProgram = %q, want the tape's first Program header", got)
	}
	// It is the first, and stays.
	zx.tape.st.Blocks[2].HeaderName = "other"
	if got := zx.FirstProgram(); got != "MANIC" {
		t.Errorf("FirstProgram changed to %q", got)
	}
}

// discWithProgram is a +3 disc as a flat image with one file in its
// directory, name.ext, whose first block is block and whose content begins
// as a +3DOS file of the given BASIC type.
func discWithProgram(name, ext string, block int, kind byte) []byte {
	img := bytes.Repeat([]byte{0xE5}, DiskImageSize)
	sector := func(logical int) []byte { return img[logical*BytesPerSector : (logical+1)*BytesPerSector] }
	// The directory: the first two blocks after the one reserved track.
	entry := sector(SectorsPerTrack)[32:64] // the second entry; the first stays unused
	for i := range entry {
		entry[i] = 0
	}
	copy(entry[1:9], []byte(name + "        ")[:8])
	copy(entry[9:12], []byte(ext + "   ")[:3])
	entry[16] = byte(block)
	copy(sector(SectorsPerTrack+block*2), plus3DOSFile(kind))
	return img
}

func TestFirstProgramFromDisc(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disc.dsk")
	if err := os.WriteFile(path, discWithProgram("GAME", "BAS", 5, 0), 0o644); err != nil {
		t.Fatal(err)
	}
	zx := NewZenZX(AudioBackendOto)
	if err := zx.io.LoadDisk(path); err != nil {
		t.Fatal(err)
	}
	fdc := zx.io.fdc
	read := func(logical int) {
		fdc.currentR = uint8(logical%SectorsPerTrack + 1)
		fdc.loadSectorForRead(uint8(logical/SectorsPerTrack), 0, 0)
	}

	read(SectorsPerTrack) // the directory: not a program
	if got := zx.FirstProgram(); got != "" {
		t.Fatalf("reading the directory named the machine %q", got)
	}
	read(SectorsPerTrack + 5*2) // the file's first sector
	if got := zx.FirstProgram(); got != "GAME" {
		t.Fatalf("FirstProgram = %q, want GAME", got)
	}
	// The name outlives the disc.
	fdc.EjectDisk()
	if got := zx.FirstProgram(); got != "GAME" {
		t.Errorf("FirstProgram = %q after the disc was ejected", got)
	}

	// A file that is not a program names nothing.
	other := filepath.Join(filepath.Dir(path), "code.dsk")
	if err := os.WriteFile(other, discWithProgram("SCREEN", "BIN", 5, 3), 0o644); err != nil {
		t.Fatal(err)
	}
	zx2 := NewZenZX(AudioBackendOto)
	if err := zx2.io.LoadDisk(other); err != nil {
		t.Fatal(err)
	}
	zx2.io.fdc.currentR = uint8((SectorsPerTrack+10)%SectorsPerTrack + 1)
	zx2.io.fdc.loadSectorForRead(uint8((SectorsPerTrack+10)/SectorsPerTrack), 0, 0)
	if got := zx2.FirstProgram(); got != "" {
		t.Errorf("a CODE file named the machine %q", got)
	}
}

func TestCardFileAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "card.img")
	program := plus3DOSFile(0)
	copy(program[128:], "the program's own bytes")
	root := nextcard.NewDir("")
	root.Add("readme.txt", []byte("not a program"))
	root.AddDir("demos").Add("intro.bas", program)
	if _, err := nextcard.NewCard(path, nextcard.SmallestCard, root); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Find where the program is in the image by looking for it.
	offset := int64(-1)
	buf := make([]byte, 1<<20)
	for at := int64(0); offset < 0; at += int64(len(buf)) - 512 {
		n, _ := f.ReadAt(buf, at)
		if n < 512 {
			break
		}
		if i := bytes.Index(buf[:n], program[:160]); i >= 0 {
			offset = at + int64(i)
		}
	}
	if offset < 0 {
		t.Fatal("the program is not in the card's image")
	}
	vol, err := nextcard.Open(f)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := vol.FileAt(offset); !ok || got != "/demos/intro.bas" {
		t.Errorf("FileAt(%d) = %q, %v; want /demos/intro.bas", offset, got, ok)
	}
	if got, ok := vol.FileAt(offset + 512); ok {
		t.Errorf("FileAt in the middle of a file = %q", got)
	}

	// The card notes the block when it is read, and only such blocks.
	card := NewSDCard(f, nextcard.SmallestCard)
	block := make([]byte, 512)
	f.ReadAt(block, offset)
	card.noteBlock(0, make([]byte, 512))
	card.noteBlock(offset, block)
	if len(card.programAt) != 1 || card.programAt[0] != offset {
		t.Errorf("the card noted %v, want the program's place", card.programAt)
	}
}
