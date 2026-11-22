package zenzx

import (
	"bytes"
	"context"
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

// A card with nothing on it but the files MakeBootable puts there boots a
// Next to the NextZXOS menu. It needs the real boot files (dormant guard
// G-04, with the other TestNextZXOS tests).
func TestNextZXOSBootsFromAMinimalCard(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-04)")
	}
	f := nextcard.In(dir)
	card, err := newEmptyCard(filepath.Join(t.TempDir(), "minimal"), smallestCard)
	if err != nil {
		t.Fatal(err)
	}
	n, err := nextcard.MakeBootable(context.Background(), f.Card, card, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d files, %d bytes", len(nextcard.BootFiles), n)

	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootNextFrom(zx, f, card, true, false); err != nil {
		t.Fatal(err)
	}
	defer zx.detachNextBootChain()
	// The firmware first runs in config mode, chooses the machine and leaves
	// it; then NextZXOS draws its main menu, whose highlighted line is a row
	// of cyan cells. With no welcome screen on the card the menu is the
	// first thing NextZXOS shows.
	cyan := func() int {
		n := 0
		for cell := 4 * 32; cell < 16*32; cell++ {
			if zx.memory.screen.attributes[cell]&0x38 == 0x28 {
				n++
			}
		}
		return n
	}
	handedOver := 0
	for f := 50; f <= 6000; f += 50 {
		for i := 0; i < 50; i++ {
			zx.RunFrame()
		}
		if handedOver == 0 {
			if zx.memory.nextCfg.machineType == 3 && !zx.memory.nextCfg.active && !zx.memory.bootROMEnabled {
				handedOver = f
				t.Logf("the firmware chose the machine and left config mode after %d frames", f)
			}
			continue
		}
		if cyan() >= 8 {
			t.Logf("the NextZXOS menu after %d frames", f)
			if out := os.Getenv("ZENZX_TEST_SHOTS"); out != "" {
				writeScreenPNG(filepath.Join(out, "minimal-card-menu.png"), zx, 1)
			}
			return
		}
	}
	t.Fatalf("the NextZXOS menu was not reached in 6000 frames (the firmware handed over at frame %d; 0 is never)", handedOver)
}
