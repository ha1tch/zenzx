//go:build headless

package zenzx

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextZXOSSeesCardWriteAfterReinsert boots real NextZXOS from the 26.11 card
// and reproduces, then verifies the fix for, the user's reported symptom: files
// written to the second SD card do not show up, even after NextZXOS's own
// remount.
//
// Root cause: pkg/nextcard.AddFiles writes a new image and renames it over the
// card file, which orphans the open file handle the emulator holds -- it keeps
// serving the OLD inode, so NextZXOS reads stale contents no matter how often
// its Browser remounts. The mount drawer fixes this by re-inserting the card
// (reinsertIfLive) after a write, which this test stands in for with a direct
// InsertSecondCard. It then drives NextZXOS's Browser remount (M, then Y) and
// checks the new file is listed.
//
// The listing is read from the composed frame, not the ULA screen, because the
// Browser draws on the tilemap. Gated on ZENZX_NEXT_FILES like the other
// NextZXOS hardware guards (dormant guard G-07).
func TestNextZXOSSeesCardWriteAfterReinsert(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-07)")
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return data
	}
	examples, err := nextExamples()
	if err != nil {
		t.Fatal(err)
	}
	var gallop, fuji nextcard.File
	for _, e := range examples {
		switch e.Name {
		case "gallop.nex":
			gallop = e
		case "fuji.nex":
			fuji = e
		}
	}
	if gallop.Data == nil || fuji.Data == nil {
		t.Fatal("need gallop.nex and fuji.nex examples")
	}

	// Second card starts with gallop only; fuji is written while it is inserted.
	ownCard := filepath.Join(t.TempDir(), "next-card.img")
	if err := addToOwnCard(ownCard, []nextcard.File{gallop}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	zx := NewZenZX(AudioBackendOto)
	if err := loadModelROM(zx, "next"); err != nil {
		t.Fatal(err)
	}
	zx.memory.SetBootROM(read(nextcard.BootROMName))
	boot, err := os.Open(filepath.Join(dir, nextcard.CardName))
	if err != nil {
		t.Fatal(err)
	}
	defer boot.Close()
	info, _ := boot.Stat()
	zx.EnableDivMMC(boot, info.Size(), read(nextcard.DivMMCROMName))
	zx.io.divMMC.SD().wr.WriteProtected = true
	if _, err := zx.InsertSecondCard(ownCard, false); err != nil {
		t.Fatal(err)
	}
	zx.cpu.Reset()
	zx.running = true

	run := func(n int) {
		for i := 0; i < n; i++ {
			zx.RunFrame()
		}
	}
	press := func(names ...string) {
		var chord []matrixPos
		for _, n := range names {
			if c, ok := namedKeys[n]; ok {
				chord = append(chord, c...)
			} else if pos, ok := baseKeys[[]rune(n)[0]]; ok {
				chord = append(chord, pos)
			} else {
				t.Fatalf("no key %q", n)
			}
		}
		for _, k := range chord {
			zx.io.PressKey(k.row, k.col)
		}
		run(8)
		for _, k := range chord {
			zx.io.ReleaseKey(k.row, k.col)
		}
		run(12)
	}

	// Boot, then open the Browser and switch to the second card's drive (D).
	run(3500)
	press("space")
	run(150)
	press("enter") // Browser (the default menu entry)
	run(250)
	press("d") // Drive -> D:
	run(200)

	// Write fuji while the card is inserted (as the drawer does), then re-insert
	// so the emulator reads the new image (the fix). Without the re-insert the
	// emulator serves the stale handle and the remount below shows only gallop.
	if err := nextcard.AddFiles(context.Background(), ownCard, "", []nextcard.File{fuji}, nil); err != nil {
		t.Fatalf("add fuji: %v", err)
	}
	if _, err := zx.InsertSecondCard(ownCard, false); err != nil {
		t.Fatal(err)
	}

	// NextZXOS's Browser remount: M, then Y at the "Remove/insert SD" prompt.
	press("m")
	run(120)
	press("y")
	run(250)

	// After the remount the newly-added fuji.nex sorts to the top and is
	// selected; run it with ENTER. If NextZXOS could not see it (the bug), there
	// is nothing to run and fuji's NextREG state never appears.
	isFuji := func() bool {
		r := &zx.io.nextRegs.regs
		return zx.io.copper.mode == 3 && r[0x70] == 0x10 && r[0x6B] == 0x83 && r[0x15] == 0x0B
	}
	if isFuji() {
		t.Fatal("fuji state present before it was run")
	}
	press("enter")
	run(400)
	if !isFuji() {
		t.Errorf("after writing fuji.nex and re-inserting the card, NextZXOS's remount did not show it (ENTER on the top entry did not run fuji)")
	}
}
