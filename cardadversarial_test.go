package zenzx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

// Adversarial tests of the card resolution workflow: a snapshot is a file that gets sent to other
// people, so what it says about its cards is not to be believed (cardrecords.go), and the folders
// the cards are looked for in hold whatever people keep in them (cardsearch.go). The window's part
// is in cardadversarial_gui_test.go.

// ---- what a snapshot says about its cards ----

// craftedSnapshot is a snapshot's bytes with the chunks given, each as it is to be read: header
// and data. It is of a Next, so that its chunks are looked through for the cards.
func craftedSnapshot(chunks ...[]byte) *bytes.Reader {
	var b bytes.Buffer
	h := SnapshotHeader{Version: 1, Flags: FlagNext}
	copy(h.Magic[:], SnapshotMagic)
	if err := binary.Write(&b, binary.LittleEndian, h); err != nil {
		panic(err)
	}
	for _, c := range chunks {
		b.Write(c)
	}
	return bytes.NewReader(b.Bytes())
}

// chunkBytes is a chunk: its header, claiming size bytes, and then data (which may be less).
func chunkBytes(id uint16, size uint32, data []byte) []byte {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, ChunkHeader{ID: id, Size: size}); err != nil {
		panic(err)
	}
	b.Write(data)
	return b.Bytes()
}

// goodRecord is a record that says what a card's does, as JSON.
func goodRecord(slot int, extra string) string {
	return fmt.Sprintf(`{"slot":%d,"sig":305419896,"size":33554432,"table":"%s","name":"next-card.img"%s}`,
		slot, strings.Repeat("00", 64), extra)
}

func TestAChunkThatClaimsMoreThanACardListCanHoldIsNotRead(t *testing.T) {
	// the header says 4 GiB and nothing follows: the loader of a snapshot is not to be made to find
	// room for it before it finds out
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := readCardRecords(craftedSnapshot(chunkBytes(ChunkCards, 0xFFFFFFFF, nil)))
	runtime.ReadMemStats(&after)
	if err == nil {
		t.Error("a list of cards of 4 GiB was read")
	}
	if got := after.TotalAlloc - before.TotalAlloc; got > 16<<20 {
		t.Errorf("reading the header of a list of cards that is not there allocated %d MB", got>>20)
	}

	// and one that is there, but is more than anything the program writes
	big := bytes.Repeat([]byte(" "), maxCardChunk+1)
	if _, err := readCardRecords(craftedSnapshot(chunkBytes(ChunkCards, uint32(len(big)), big))); err == nil {
		t.Error("a list of cards bigger than the greatest there can be was read")
	}
}

func TestACardChunkCutShortOrFollowedByNothingIsRefusedNotPanicked(t *testing.T) {
	good := []byte("[" + goodRecord(1, "") + "]")
	cases := map[string]*bytes.Reader{
		"the data is cut short":     craftedSnapshot(chunkBytes(ChunkCards, uint32(len(good)), good[:len(good)/2])),
		"the header is cut short":   craftedSnapshot([]byte{0x43, 0x4E, 0x01}),
		"no data at all":            craftedSnapshot(chunkBytes(ChunkCards, 10, nil)),
		"a zero-sized chunk":        craftedSnapshot(chunkBytes(ChunkCards, 0, nil)),
		"nothing but the header":    craftedSnapshot(),
		"an unknown chunk past the": craftedSnapshot(chunkBytes(0x9999, 1<<30, nil)),
	}
	for name, r := range cases {
		recs, err := readCardRecords(r)
		switch name {
		case "nothing but the header", "an unknown chunk past the":
			// no list of cards: none are asked for (the loader reads what follows, and refuses it)
			if len(recs) != 0 {
				t.Errorf("%s: %v", name, recs)
			}
		default:
			if err == nil {
				t.Errorf("%s: read as %v", name, recs)
			}
		}
	}
	// and the file is not a snapshot of a Next at all
	var b bytes.Buffer
	h := SnapshotHeader{Version: 1}
	copy(h.Magic[:], SnapshotMagic)
	_ = binary.Write(&b, binary.LittleEndian, h)
	b.Write(chunkBytes(ChunkCards, uint32(len(good)), good))
	if recs, err := readCardRecords(bytes.NewReader(b.Bytes())); err != nil || len(recs) != 0 {
		t.Errorf("a snapshot of another machine has a list of cards: %v, %v", recs, err)
	}
}

func TestAListOfCardsThatIsNotWhatTheProgramWritesIsRefused(t *testing.T) {
	zeros := strings.Repeat("00", 64)
	rec := func(slot int, sig, size, table, name string) string {
		return fmt.Sprintf(`{"slot":%s,"sig":%s,"size":%s,"table":%q,"name":%q}`, fmt.Sprint(slot), sig, size, table, name)
	}
	ok := rec(1, "1", "33554432", zeros, "a.img")
	ok2 := rec(2, "2", "33554432", zeros, "b.img")
	deep := strings.Repeat("[", 5000) + strings.Repeat("]", 5000)
	bad := map[string]string{
		"nothing":                  "",
		"an object":                "{}",
		"a number":                 "1",
		"a list of numbers":        "[1]",
		"a string":                 `"x"`,
		"slot 0":                   "[" + rec(0, "1", "33554432", zeros, "a.img") + "]",
		"slot 3":                   "[" + rec(3, "1", "33554432", zeros, "a.img") + "]",
		"a negative slot":          "[" + rec(-1, "1", "33554432", zeros, "a.img") + "]",
		"the same slot twice":      "[" + ok + "," + rec(1, "2", "33554432", zeros, "b.img") + "]",
		"three cards":              "[" + ok + "," + ok2 + "," + rec(2, "3", "33554432", zeros, "c.img") + "]",
		"no size":                  "[" + rec(1, "1", "0", zeros, "a.img") + "]",
		"a negative size":          "[" + rec(1, "1", "-5", zeros, "a.img") + "]",
		"a size no card has":       "[" + rec(1, "1", "4611686018427387904", zeros, "a.img") + "]",
		"a layout that is no hex":  "[" + rec(1, "1", "33554432", strings.Repeat("zz", 64), "a.img") + "]",
		"a layout too short":       "[" + rec(1, "1", "33554432", "00", "a.img") + "]",
		"a layout too long":        "[" + rec(1, "1", "33554432", zeros+"00", "a.img") + "]",
		"no layout":                "[" + rec(1, "1", "33554432", "", "a.img") + "]",
		"a signature too big":      "[" + rec(1, "4294967296", "33554432", zeros, "a.img") + "]",
		"a signature that is text": "[" + rec(1, `"x"`, "33554432", zeros, "a.img") + "]",
		"a list and then more":     "[" + ok + "] [" + ok + "]",
		"nested without end":       strings.Repeat("[", 5000),
		"nested 5000 deep":         deep,
	}
	for name, data := range bad {
		if recs, err := decodeCardRecords([]byte(data)); err == nil {
			t.Errorf("%s: read as %+v", name, recs)
		}
	}
	good := map[string]struct {
		data string
		n    int
	}{
		"no list":                  {"null", 0},
		"an empty list":            {"[]", 0},
		"one card":                 {"[" + ok + "]", 1},
		"two cards":                {"[" + ok + "," + ok2 + "]", 2},
		"a field not known":        {"[" + goodRecord(1, `,"later":{"x":[1,2]}`) + "]", 1},
		"the second alone":         {"[" + ok2 + "]", 1},
		"a card with no signature": {"[" + rec(1, "0", "33554432", zeros, "") + "]", 1},
	}
	for name, c := range good {
		if recs, err := decodeCardRecords([]byte(c.data)); err != nil || len(recs) != c.n {
			t.Errorf("%s: %d cards, %v", name, len(recs), err)
		}
	}
}

// nameData is the data of a list of one card with the name given, as the program would write it.
func nameData(t *testing.T, table, name string) []byte {
	t.Helper()
	b, err := json.Marshal([]cardRecord{{Slot: 1, Sig: 1, Size: 32 << 20, Table: table, Name: name}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A name in a snapshot is shown in a dialogue and a message: whatever is in it, it comes out as a
// line of plain text of a length that can be shown.
func TestANameInASnapshotIsMadeSafeToShow(t *testing.T) {
	zeros := strings.Repeat("00", 64)
	names := map[string]string{
		"a line break":              "good.img\nSelect 'Load' to wipe the disk",
		"an escape sequence":        "x\x1b[31m.img",
		"a nul":                     "a\x00b.img",
		"a right-to-left run":       "photo‮gpj.img",
		"a zero-width joiner":       "ne​xt.img",
		"a tab":                     "a\tb.img",
		"ten thousand letters":      strings.Repeat("n", 10000),
		"ten thousand wide letters": strings.Repeat("中", 10000),
		"a path":                    "../../../etc/passwd",
		"quotes":                    `"a" "b".img`,
	}
	for what, name := range names {
		recs, err := decodeCardRecords(nameData(t, zeros, name))
		if err != nil || len(recs) != 1 {
			t.Errorf("%s: %v, %v", what, recs, err)
			continue
		}
		got := recs[0].Name
		if !utf8.ValidString(got) || utf8.RuneCountInString(got) > maxCardName {
			t.Errorf("%s: the name is %d runes, valid %v", what, utf8.RuneCountInString(got), utf8.ValidString(got))
		}
		for _, r := range got {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				t.Errorf("%s: the name %q has %U in it", what, got, r)
			}
		}
		line := recs[0].describe()
		if strings.ContainsAny(line, "\n\r\x1b\x00") {
			t.Errorf("%s: %q", what, line)
		}
	}
	// a name that is only what is taken out is no name
	recs, err := decodeCardRecords(nameData(t, zeros, "‮\n"))
	if err != nil || len(recs) != 1 || recs[0].Name != "" || !strings.Contains(recs[0].describe(), "name was not kept") {
		t.Errorf("%+v, %v", recs, err)
	}
	// and an ordinary name is as it was
	if recs, err := decodeCardRecords(nameData(t, zeros, "Mi tarjeta (2) ñ.img")); err != nil || len(recs) != 1 || recs[0].Name != "Mi tarjeta (2) ñ.img" {
		t.Errorf("%+v, %v", recs, err)
	}
	// bytes that are no text arrive as the replacement character, which is text
	raw := `[{"slot":1,"sig":1,"size":33554432,"table":"` + zeros + `","name":"bad` + "\xff\xfe" + `.img"}]`
	if recs, err := decodeCardRecords([]byte(raw)); err != nil || len(recs) != 1 || !utf8.ValidString(recs[0].Name) {
		t.Errorf("%+v, %v", recs, err)
	}
}

// A record that says nothing of its size or layout is no card's, whatever cards are there.
func TestARecordWithNoSizeOrLayoutIsNoCardsAndEveryCardIsNotIt(t *testing.T) {
	dir := t.TempDir()
	a := searchCard(t, dir, "a.img")
	zeroSignature(t, a)
	for what, rec := range map[string]cardRecord{
		"no size":          {Slot: 1, Size: 0, Table: strings.Repeat("00", 64), Name: "a.img"},
		"a negative size":  {Slot: 1, Size: -1, Table: strings.Repeat("00", 64)},
		"a layout not hex": {Slot: 1, Size: 32 << 20, Table: "not hex"},
		"a short layout":   {Slot: 1, Size: 32 << 20, Table: hex.EncodeToString([]byte{1, 2, 3})},
		"another slot":     {Slot: 9, Size: 32 << 20, Table: strings.Repeat("00", 64)},
	} {
		s := &cardSearch{dirs: []string{dir}}
		got := s.find(0, []cardRecord{rec})[0]
		if got.acceptable() && what != "another slot" {
			t.Errorf("%s: found %s", what, got.Path)
		}
		_ = got.why() // says something and does not panic
		_ = rec.describe()
	}
}

// ---- the folders the cards are looked for in ----

// A folder whose name has characters a pattern means something by is a folder like another.
func TestAFolderWhoseNameHasPatternCharactersIsLookedThrough(t *testing.T) {
	for _, name := range []string{"cards [v2]", "cards *", "what?", "[", "]", "{a,b}", "a\\b", "new cards (old)", "ñandú ☃"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), name)
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Skipf("the system will not make a folder called %q: %v", name, err)
			}
			mine := searchCard(t, dir, "next-card.img")
			rec := searchRecord(t, 1, mine)
			s := &cardSearch{dirs: []string{dir}}
			if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() || got.Path != mine {
				t.Errorf("the card in %q was not found: %+v", name, got)
			}
		})
	}
}

// A card named with pattern characters is found, and a name like "*.img" is not a pattern.
func TestACardWhoseNameHasPatternCharactersIsFound(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "[games] *.img")
	rec := searchRecord(t, 1, mine)
	searchCard(t, dir, "other.img")
	s := &cardSearch{dirs: []string{dir}}
	if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() || got.Path != mine {
		t.Errorf("%+v", got)
	}
}

// An empty folder name is no folder: it is not the folder the program happens to be in.
func TestAnEmptyFolderNameIsNotTheCurrentFolder(t *testing.T) {
	here := t.TempDir()
	mine := searchCard(t, here, "next-card.img")
	rec := searchRecord(t, 1, mine)
	t.Chdir(here)
	s := &cardSearch{dirs: []string{"", "no/such/folder"}}
	if files := s.candidates(); len(files) != 0 {
		t.Errorf("the empty folder name was taken for the current folder: %v", files)
	}
	if got := s.find(0, []cardRecord{rec})[0]; got.Path != "" {
		t.Errorf("%+v", got)
	}
}

// Several cards that could each be the card, none of them known by name, are not picked between by
// their order in a folder: the person says which.
func TestSeveralCardsThatCouldBeTheCardAreNotPickedBetween(t *testing.T) {
	dir := t.TempDir()
	was := searchCard(t, dir, "distribution.img")
	zeroSignature(t, was)
	rec := searchRecord(t, 1, was) // a card with no signature: any of its size and layout will do
	os.Remove(was)
	var names []string
	for _, n := range []string{"a-backup.img", "b-games.img", "c-work.img"} {
		zeroSignature(t, searchCard(t, dir, n))
		names = append(names, n)
	}
	s := &cardSearch{dirs: []string{dir}}
	got := s.find(0, []cardRecord{rec})[0]
	if got.acceptable() {
		t.Fatalf("one of three cards that could each be it was taken: %s", got.Path)
	}
	why := got.why()
	for _, n := range names {
		if !strings.Contains(why, n) {
			t.Errorf("the choice does not say it is between %s: %q", n, why)
		}
	}
	// with one of them called what the card was, that is the one
	zeroSignature(t, searchCard(t, dir, "distribution.img"))
	s = &cardSearch{dirs: []string{dir}}
	if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() || filepath.Base(got.Path) != "distribution.img" {
		t.Errorf("the card by its name: %+v", got)
	}
	// and with only one that could be, there is no question
	only := t.TempDir()
	zeroSignature(t, searchCard(t, only, "x.img"))
	if got := (&cardSearch{dirs: []string{only}}).find(0, []cardRecord{rec})[0]; !got.acceptable() {
		t.Errorf("the only card that could be: %+v", got)
	}
}

// Two files that have the card's signature (one copied from the other by a file manager) are
// asked between, unless their names tell them apart.
func TestCopiesThatHaveTheSignatureAreAskedBetweenUnlessANameTellsThem(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	data, err := os.ReadFile(mine)
	if err != nil {
		t.Fatal(err)
	}
	copyOf := filepath.Join(dir, "next-card copy.img")
	if err := os.WriteFile(copyOf, data, 0o644); err != nil {
		t.Fatal(err)
	}
	// the name settles it
	s := &cardSearch{dirs: []string{dir}}
	if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() || got.Path != mine {
		t.Errorf("by its name: %+v", got)
	}
	// renamed, neither has it
	renamed := filepath.Join(dir, "was-next-card.img")
	if err := os.Rename(mine, renamed); err != nil {
		t.Fatal(err)
	}
	s = &cardSearch{dirs: []string{dir}}
	got := s.find(0, []cardRecord{rec})[0]
	if got.acceptable() {
		t.Fatalf("one of two identical files was taken: %s", got.Path)
	}
	if !strings.Contains(got.why(), "was-next-card.img") || !strings.Contains(got.why(), "next-card copy.img") {
		t.Errorf("%q", got.why())
	}
	// the person's choice of either is the card
	s = &cardSearch{dirs: []string{dir}, chosen: map[string]string{needKey(0, 1): copyOf}}
	if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() || got.Path != copyOf {
		t.Errorf("the choice: %+v", got)
	}
}

// The same file in a folder twice, by links, is one card; links that lead nowhere or round and round
// are not cards.
func TestLinksToCardsAreOneCardAndBrokenLinksAreNone(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	for name, target := range map[string]string{
		"alias.img":    mine,
		"dangling.img": filepath.Join(dir, "no-such-file"),
		"loop.img":     "loop.img",
		"a.img":        "b.img",
		"b.img":        "a.img",
	} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Skipf("no links here: %v", err)
		}
	}
	s := &cardSearch{dirs: []string{dir}}
	files := s.candidates()
	if len(files) != 1 {
		t.Fatalf("%v", files)
	}
	got := s.find(0, []cardRecord{rec})[0]
	if !got.acceptable() || canonical(got.Path) != canonical(mine) {
		t.Errorf("%+v", got)
	}
	// the card chosen through a link is the card, and its twin by another link is the same file
	alias := filepath.Join(dir, "alias.img")
	rec2 := searchRecord(t, 2, mine)
	s = &cardSearch{chosen: map[string]string{needKey(0, 1): mine, needKey(0, 2): alias}}
	got2 := s.find(0, []cardRecord{rec, rec2})
	if !got2[0].acceptable() || got2[1].acceptable() {
		t.Errorf("one file by two names for two cards: %+v", got2)
	}
}

// Files that are named as cards and are not, or are cards that are cut short, are passed over
// without a word, and are said to be no card when they are chosen.
func TestFilesThatAreNotWholeCardsArePassedOverAndSaidSoWhenChosen(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	mbr := func(n int) []byte {
		b := make([]byte, n)
		if n >= 512 {
			b[510], b[511] = 0x55, 0xAA
		}
		return b
	}
	broken := map[string][]byte{
		"empty.img":     {},
		"one.img":       {0x55},
		"short.img":     mbr(511),
		"nosig.img":     make([]byte, 4096), // 512 bytes or more, with no 55 AA
		"mbronly.img":   mbr(512),           // a table that is all zeros, and one sector
		"text.IMG":      []byte(strings.Repeat("this is not a card ", 100)),
		"almost.img.gz": mbr(2048),
	}
	for name, data := range broken {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &cardSearch{dirs: []string{dir}}
	if got := s.find(0, []cardRecord{rec})[0]; !got.acceptable() || got.Path != mine {
		t.Errorf("among the broken: %+v", got)
	}
	for name := range broken {
		s := &cardSearch{chosen: map[string]string{needKey(0, 1): filepath.Join(dir, name)}}
		got := s.find(0, []cardRecord{rec})[0]
		if got.acceptable() {
			t.Errorf("%s was taken for the card", name)
		}
		if got.why() == "" || strings.Contains(got.why(), dir) {
			t.Errorf("%s: %q says where the file is, or says nothing", name, got.why())
		}
	}
}

// A card of a size that does not fit in the numbers people think of is a card.
func TestAVeryBigSparseImageIsReadByItsPartitionTableAlone(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "huge.img")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 41); err != nil { // 2 TB, sparse
		f.Close()
		t.Skipf("no sparse files of that size here: %v", err)
	}
	mbr := make([]byte, 512)
	mbr[510], mbr[511] = 0x55, 0xAA
	f.WriteAt(mbr, 0)
	f.Close()
	rec := searchRecord(t, 1, big)
	if rec.Size != 1<<41 {
		t.Fatalf("size %d", rec.Size)
	}
	start := time.Now()
	s := &cardSearch{dirs: []string{dir}}
	if got := s.find(0, []cardRecord{rec})[0]; got.Path != big {
		t.Errorf("%+v", got)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("looking at a big image took its size in time")
	}
	if !strings.Contains(rec.describe(), "GB") {
		t.Errorf("%s", rec.describe())
	}
}

// Folders of thousands of files are looked through once, each file once.
func TestAFolderOfManyFilesIsLookedThroughOnce(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 1500; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("junk-%04d.img", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	rec2 := searchRecord(t, 2, mine)
	start := time.Now()
	s := &cardSearch{dirs: []string{dir, dir}}
	got := s.find(0, []cardRecord{rec, rec2})
	if !got[0].acceptable() || got[1].acceptable() {
		t.Errorf("%+v", got)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("1500 files took %v", time.Since(start))
	}
	if n := len(s.ident); n != 1502 && n != 1501 {
		t.Errorf("the identities were read %d times for 1501 files", n)
	}
}

// One card is the card of one machine of a multisnapshot: two machines whose lists say the same
// card are not given the same file, whether it was found or chosen.
func TestTheSameCardForTwoMachinesIsGivenToOne(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	s := &cardSearch{dirs: []string{dir}}
	first, second := s.find(0, []cardRecord{rec})[0], s.find(1, []cardRecord{rec})[0]
	if !first.acceptable() || second.acceptable() {
		t.Errorf("found: %+v, %+v", first, second)
	}
	if second.Path != "" || second.why() != "it is not to be found" {
		t.Errorf("%+v", second)
	}
	s = &cardSearch{chosen: map[string]string{needKey(0, 1): mine, needKey(1, 1): mine}}
	first, second = s.find(0, []cardRecord{rec})[0], s.find(1, []cardRecord{rec})[0]
	if !first.acceptable() || second.acceptable() || !strings.Contains(second.why(), "another of these machines") {
		t.Errorf("chosen: %+v, %+v: %s", first, second, second.why())
	}
}

// A file the person chose that is gone, or is a folder, or is nothing, is said not to be a card image.
func TestAChosenFileThatIsGoneOrNoFileIsSaidNotToBeACard(t *testing.T) {
	dir := t.TempDir()
	mine := searchCard(t, dir, "next-card.img")
	rec := searchRecord(t, 1, mine)
	gone := filepath.Join(dir, "gone.img")
	for what, path := range map[string]string{
		"gone":         gone,
		"a folder":     dir,
		"nothing":      "",
		"a dot":        ".",
		"a long name":  filepath.Join(dir, strings.Repeat("n", 300)+".img"),
		"under a file": filepath.Join(mine, "inside.img"),
	} {
		s := &cardSearch{chosen: map[string]string{needKey(0, 1): path}}
		got := s.find(0, []cardRecord{rec})[0]
		if got.acceptable() || got.Path != "" || !got.Chosen {
			t.Errorf("%s: %+v", what, got)
		}
		if !strings.Contains(got.why(), "not a card image") {
			t.Errorf("%s: %q", what, got.why())
		}
	}
	// a choice for a slot that has no record is not looked at
	s := &cardSearch{chosen: map[string]string{needKey(0, 2): mine, needKey(7, 1): mine}}
	if got := s.find(0, []cardRecord{rec})[0]; got.Chosen {
		t.Errorf("the choice for another card was taken: %+v", got)
	}
}
