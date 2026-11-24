package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// testSystemCard makes a card at path that holds every boot file, with
// made-up contents but a config.ini that says what a system card's must.
func testSystemCard(t *testing.T, path string) {
	t.Helper()
	root := nextcard.NewDir("")
	for _, p := range nextcard.BootFiles {
		dir := root
		parts := strings.Split(p, "/")
		for _, d := range parts[:len(parts)-1] {
			if next := dir.Find(d); next != nil {
				dir = next
			} else {
				dir = dir.AddDir(d)
			}
		}
		data := []byte("contents of " + p)
		if strings.HasSuffix(p, "config.ini") {
			data = []byte("timing=0\r\n")
		}
		dir.Add(parts[len(parts)-1], data)
	}
	if _, err := nextcard.NewCard(path, nextcard.SmallestCard, root); err != nil {
		t.Fatal(err)
	}
}

// testBootFolder makes a folder that passes for the Next's boot files: the
// two ROMs (made up) and a standard card that is a system card.
func testBootFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	f := nextcard.In(dir)
	for _, rom := range []string{f.BootROM, f.DivMMCROM} {
		if err := os.WriteFile(rom, make([]byte, 8192), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	testSystemCard(t, f.Card)
	return dir
}

func TestRunNextBootable(t *testing.T) {
	dir := testBootFolder(t)
	if _, err := newEmptyCard(filepath.Join(dir, "mine"), smallestCard); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	// By its name alone: found in the boot-files folder, .img added.
	if err := runNextBootable(dir, "mine", &out); err != nil {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(out.String(), "can boot a Next now: 8 files") {
		t.Errorf("it said: %s", out.String())
	}
	card, err := os.Open(filepath.Join(dir, "mine.img"))
	if err != nil {
		t.Fatal(err)
	}
	defer card.Close()
	if err := nextcard.VerifyCard(card); err != nil {
		t.Errorf("the card is not bootable afterwards: %v", err)
	}
	if err := runNextBootable(dir, "no-such-card", &out); err == nil || !strings.Contains(err.Error(), "no SD card") {
		t.Errorf("a card that is not there gave %v", err)
	}
	if err := runNextBootable(dir, nextcard.CardName, &out); err == nil || !strings.Contains(err.Error(), "copied from") {
		t.Errorf("the standard card itself gave %v", err)
	}
}
