//go:build headless

package zenzx

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

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
			expectFrames(t, "the NextZXOS menu", zx.frameCount, 150, 1500)
			for i := 0; i < 50; i++ {
				zx.RunFrame()
			}
			checkScreenLooksLikeAMenu(t, zx.DecodeDisplay(), "the menu from the minimal card")
			checkScreenIsSteady(t, zx, "the menu from the minimal card")
			// What the card is without is what a Next does not need to start, so its menu is
			// the menu of the full card it was made from. They are not compared pixel for
			// pixel: the two cards have different contents to list and a different cursor phase.
			full := nextMenuScreen(t, nextcard.In(dir), nextcard.In(dir).Card)
			same := screenSimilarity(zx.DecodeDisplay(), full)
			t.Logf("the menu from the minimal card is %.1f%% the same as the menu from the card it was made from", same*100)
			if same < 0.90 {
				t.Errorf("the menu from the minimal card is only %.1f%% the same as the menu from the full card; expected at least 90%%", same*100)
			}
			return
		}
	}
	t.Fatalf("the NextZXOS menu was not reached in 6000 frames (the firmware handed over at frame %d; 0 is never)", handedOver)
}
