package nextcard

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// mbrImage is a card image of size bytes whose first sector is a partition table with one FAT32
// partition and the signature sig.
func mbrImage(t *testing.T, size int64, sig uint32) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "card.img")
	mbr := make([]byte, sectorSize)
	binary.LittleEndian.PutUint32(mbr[sigOffset:], sig)
	e := mbr[tableOffset:]
	e[4] = 0x0C
	binary.LittleEndian.PutUint32(e[8:], 2048)
	binary.LittleEndian.PutUint32(e[12:], uint32(size/sectorSize)-2048)
	mbr[510], mbr[511] = 0x55, 0xAA
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(mbr, 0); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustIdentity(t *testing.T, path string) Identity {
	t.Helper()
	id, err := IdentityOfFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestIdentityCompare(t *testing.T) {
	const size = 8 << 20
	a := mustIdentity(t, mbrImage(t, size, 0x11111111))
	same := mustIdentity(t, mbrImage(t, size, 0x11111111))
	other := mustIdentity(t, mbrImage(t, size, 0x22222222))
	bare := mustIdentity(t, mbrImage(t, size, 0))
	bigger := mustIdentity(t, mbrImage(t, 2*size, 0x11111111))
	for _, c := range []struct {
		name string
		x, y Identity
		want Match
	}{
		{"the same signature, size and layout", a, same, Same},
		{"another signature", a, other, Different},
		{"another size", a, bigger, Different},
		{"one has no signature", a, bare, Alike},
		{"the other has none", bare, a, Alike},
		{"neither has one", bare, bare, Alike},
	} {
		if got := c.x.Compare(c.y); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// Stamping gives a card without a signature one, touches nothing else of the sector, and leaves a
// card that has one alone.
func TestStampGivesASignatureOnce(t *testing.T) {
	path := mbrImage(t, 8<<20, 0)
	before, _ := os.ReadFile(path)
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	id, stamped, err := Stamp(f, 8<<20)
	if err != nil || !stamped || !id.Keyed() {
		t.Fatalf("Stamp = %+v, %v, %v", id, stamped, err)
	}
	after, _ := os.ReadFile(path)
	diff := 0
	for i := range before {
		if before[i] != after[i] {
			diff++
			if i < sigOffset || i >= sigOffset+4 {
				t.Errorf("byte %d changed: a stamp writes the signature only", i)
			}
		}
	}
	if diff == 0 {
		t.Error("nothing was written")
	}
	if got := mustIdentity(t, path); got != id {
		t.Errorf("the card now says %+v, Stamp said %+v", got, id)
	}
	again, stamped, err := Stamp(f, 8<<20)
	if err != nil || stamped || again != id {
		t.Errorf("a second Stamp = %+v, %v, %v: the signature stays", again, stamped, err)
	}
}

func TestStampOfACardThatCannotBeWritten(t *testing.T) {
	path := mbrImage(t, 8<<20, 0)
	f, err := os.Open(path) // read-only
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	id, stamped, err := Stamp(f, 8<<20)
	_ = id
	if stamped || err == nil {
		t.Errorf("a read-only card was stamped: %v, %v", stamped, err)
	}
	if mustIdentity(t, path).Keyed() {
		t.Error("the file has a signature")
	}
}

func TestResignChangesTheSignatureOnly(t *testing.T) {
	path := mbrImage(t, 8<<20, 0xCAFEF00D)
	before, _ := os.ReadFile(path)
	if err := Resign(path); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if bytes.Equal(before, after) {
		t.Fatal("the card is as it was")
	}
	if id := mustIdentity(t, path); id.Sig == 0xCAFEF00D || !id.Keyed() {
		t.Errorf("the signature is %08x", id.Sig)
	}
	for i := range before {
		if before[i] != after[i] && (i < sigOffset || i >= sigOffset+4) {
			t.Errorf("byte %d changed", i)
		}
	}
}

func TestReadIdentityRefusesAnImageWithNoPartitionTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.img")
	if err := os.WriteFile(path, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := IdentityOfFile(path); err == nil {
		t.Error("an image of zeros has an identity")
	}
	if _, err := IdentityOfFile(t.TempDir()); err == nil {
		t.Error("a folder has an identity")
	}
}

func TestNewSignatureIsNeverZero(t *testing.T) {
	for i := 0; i < 1000; i++ {
		if NewSignature() == 0 {
			t.Fatal("a zero signature")
		}
	}
}

// A card ZenZX makes has an identity, and adding files to it, which makes it again, keeps it.
func TestANewCardIsSignedAndKeepsItsSignatureWhenFilesAreAdded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "own.img")
	if _, err := NewCard(path, SmallestCard, NewDir("")); err != nil {
		t.Fatal(err)
	}
	id := mustIdentity(t, path)
	if !id.Keyed() {
		t.Fatal("a new card has no signature")
	}
	if err := AddFiles(t.Context(), path, "/", []File{{Name: "a.txt", Data: []byte("hello")}}, nil); err != nil {
		t.Fatal(err)
	}
	if got := mustIdentity(t, path); got != id {
		t.Errorf("the card was %+v, and is %+v after files were added to it", id, got)
	}
}
