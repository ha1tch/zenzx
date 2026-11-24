//go:build headless

package zenzx

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextZXOSRemountRefreshesBrowser checks that forceNextZXOSRemount makes the
// NextZXOS Browser re-read its card and show a file just written to it, without
// any physical keypress -- the host drives the firmware's own M-then-Y remount by
// feeding its key sysvars. It is the keyless form of the refresh G-07 performs by
// hand (dormant guard G-09).
//
// The setup matches G-07/G-08: a minimal bootable boot card made from the 26.11
// card (so the test needs small cards, not a full one) and a small second card
// seeded with one file. After booting to the Browser on the second card's drive,
// a file is written to that card and the handle re-inserted (the G-07 fix), then
// forceNextZXOSRemount drives the refresh. The Browser's file area must gain a row
// (the added file), exactly as a physical M-then-Y would make it.
func TestNextZXOSRemountRefreshesBrowser(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-09)")
	}
	f := nextcard.In(dir)
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

	bootCard, err := newEmptyCard(filepath.Join(t.TempDir(), "boot"), smallestCard)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyBootFiles(f.Card, bootCard); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(t.TempDir(), "second.img")
	if err := addToOwnCard(second, []nextcard.File{{Name: "SEED.NEX", Data: gallop.Data}}); err != nil {
		t.Fatal(err)
	}

	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootNextFrom(zx, f, bootCard, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := zx.InsertSecondCard(second, false); err != nil {
		t.Fatal(err)
	}
	zx.running = true
	defer zx.detachNextBootChain()
	defer zx.RemoveSecondCard()

	run := func(n int) {
		for i := 0; i < n; i++ {
			zx.RunFrame()
		}
	}
	press := func(name string) {
		var chord []matrixPos
		if c, ok := namedKeys[name]; ok {
			chord = c
		} else if p, ok := baseKeys[[]rune(name)[0]]; ok {
			chord = []matrixPos{p}
		} else {
			t.Fatalf("no key %q", name)
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

	// Boot to the Browser, on the second card's drive (D).
	run(4000)
	press("space")
	run(150)
	press("enter")
	run(250)
	press("d")
	run(200)

	// Baseline: the Browser lists one file (the seed).
	base := inkInFileArea(zx)

	// Write a second file to the second card and re-insert it (the G-07 fix),
	// then drive the remount from the host -- no physical keypress.
	if err := nextcard.AddFiles(context.Background(), second, "", []nextcard.File{fuji}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := zx.InsertSecondCard(second, false); err != nil {
		t.Fatal(err)
	}
	if !zx.forceNextZXOSRemount(func() { zx.RunFrame() }) {
		t.Fatal("forceNextZXOSRemount declined (no NextZXOS chain or no Browser idle loop found)")
	}
	run(60) // let the Browser settle after the rebuild

	// The Browser must now show a second file row: its file area gained ink.
	if gain := inkInFileArea(zx) - base; gain <= 0 {
		t.Errorf("the Browser did not gain a file row after the host-driven remount (ink %d -> %d)", base, base+gain)
	}

	// Calling it again with nothing new must be a stable no-op, not a corruption.
	after := inkInFileArea(zx)
	if !zx.forceNextZXOSRemount(func() { zx.RunFrame() }) {
		t.Fatal("second forceNextZXOSRemount declined")
	}
	run(60)
	if again := inkInFileArea(zx); again != after {
		t.Errorf("a second remount with nothing new changed the listing (ink %d -> %d)", after, again)
	}
}
