package nextcard

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAddFilesStampsTimes checks that a file added to a card gets a valid,
// non-zero FAT timestamp (a zero date is what kept NextZXOS from refreshing the
// card) and that the folder it goes into has its own write time advanced.
func TestAddFilesStampsTimes(t *testing.T) {
	// Pin the clock so the written date is predictable.
	fixed := time.Date(2026, time.November, 20, 13, 45, 30, 0, time.UTC)
	old := nowFunc
	nowFunc = func() time.Time { return fixed }
	defer func() { nowFunc = old }()

	img, _, _ := buildTestImage(t)
	card := filepath.Join(t.TempDir(), CardName)
	if err := os.WriteFile(card, img, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddFiles(context.Background(), card, "/zenzx", []File{{"stamp.nex", []byte("hi")}}, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(card)
	if err != nil {
		t.Fatal(err)
	}
	vol, err := Open(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	wantDate := uint16((fixed.Year()-1980)<<9) | uint16(fixed.Month())<<5 | uint16(fixed.Day())

	n := vol.Root.Find("/zenzx/stamp.nex")
	if n == nil {
		t.Fatal("the added file is not on the card")
	}
	// Write date is Times[9:11]; it must be the pinned date, not zero.
	wrtDate := binary.LittleEndian.Uint16(n.Times[9:11])
	if wrtDate == 0 {
		t.Error("the added file has a zero write date -- the NextZXOS refresh bug")
	}
	if wrtDate != wantDate {
		t.Errorf("file write date = 0x%04x, want 0x%04x", wrtDate, wantDate)
	}
	// Creation date (Times[3:5]) must also be set.
	if binary.LittleEndian.Uint16(n.Times[3:5]) == 0 {
		t.Error("the added file has a zero creation date")
	}

	// The folder it went into must have its write date advanced to the pinned
	// date too, so a FAT driver sees the folder as changed.
	dir := vol.Root.Find("/zenzx")
	if dir == nil {
		t.Fatal("the /zenzx folder is gone")
	}
	if got := binary.LittleEndian.Uint16(dir.Times[9:11]); got != wantDate {
		t.Errorf("folder write date = 0x%04x, want 0x%04x (advanced on add)", got, wantDate)
	}
}
