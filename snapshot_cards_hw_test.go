//go:build headless

package zenzx

import (
	"os"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// A snapshot of a Next that booted NextZXOS is loaded into a Next that was made without the boot
// chain, as the window makes one for it: the chain and the card are attached first, and the state
// goes over them, so the DivMMC has its card and the menu NextZXOS was at carries on. Against the
// real firmware.
//
// Dormant guard G-32 (docs/KNOWN_ISSUES.md): gated on ZENZX_NEXT_FILES, as G-04 is. The boot
// card is opened read-only, so the snapshot records it as it is, with no signature (the card the
// emulator distribution comes with has none), and any card of its size and layout is taken for it.
func TestANextSnapshotLoadsIntoAMachineMadeWithItsCard(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-32)")
	}
	f := nextcard.In(dir)

	// a Next at the NextZXOS menu, and its snapshot
	a, err := newMachine("next", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootNextFrom(a, f, f.Card, true, false); err != nil {
		t.Fatal(err)
	}
	defer a.detachNextBootChain()
	a.running = true
	cyan := func(zx *ZenZX) int {
		n := 0
		for cell := 4 * 32; cell < 16*32; cell++ {
			if zx.memory.screen.attributes[cell]&0x38 == 0x28 {
				n++
			}
		}
		return n
	}
	menu := false
	for frames := 0; frames < 8000 && !menu; frames += 50 {
		for i := 0; i < 50; i++ {
			a.RunFrame()
		}
		if frames > 600 && frames%300 == 0 && cyan(a) < 8 {
			a.io.PressKey(7, 0) // SPACE at the welcome screen
			for i := 0; i < 10; i++ {
				a.RunFrame()
			}
			a.io.ReleaseKey(7, 0)
		}
		menu = a.frameCount > 600 && cyan(a) >= 8
	}
	if !menu {
		t.Fatal("no NextZXOS menu to take a snapshot of")
	}
	for i := 0; i < 50; i++ {
		a.RunFrame()
	}
	path := nextSnapshotOf(t, a)
	recs, err := readSnapshotCards(path, "zxs")
	if err != nil || len(recs) != 1 || recs[0].Slot != 1 || recs[0].Name != "next-card.img" {
		t.Fatalf("the cards of the snapshot: %+v, %v", recs, err)
	}
	if id, err := nextcard.IdentityOfFile(f.Card); err != nil || id.Keyed() {
		t.Fatalf("the real card was given a signature (it is read-only here): %+v, %v", id, err)
	}

	// a Next as the deck makes it, with no card
	b, err := newMachine("next", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	b.running = true
	if err := b.LoadSnapshotFormat(path, "zxs"); err == nil {
		t.Fatal("a Next with no card took a snapshot made with one")
	}
	// the window attaches the chain and the card, and loads
	if _, err := b.attachNextBootChain(f, true, false); err != nil {
		t.Fatal(err)
	}
	defer b.detachNextBootChain()
	if err := b.LoadSnapshotFormat(path, "zxs"); err != nil {
		t.Fatalf("over its card: %v", err)
	}
	b.paused = false
	if n := cyan(b); n < 8 {
		t.Errorf("the menu was not put back: %d cells of its highlight", n)
	}
	for i := 0; i < 200; i++ {
		b.RunFrame()
	}
	checkScreenLooksLikeAMenu(t, b.DecodeDisplay(), "the Next after the snapshot")
	checkScreenIsSteady(t, b, "the Next after the snapshot")
}

// A card put in the second slot is mounted when the firmware reads from it, which it does for a card
// that was there when it started and not for one put in after (until the Browser is told to remount):
// the snapshot records the first and not the second. Dormant guard G-32.
func TestASecondCardCountsOnlyWhenTheFirmwareMountedIt(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-32)")
	}
	f := nextcard.In(dir)
	for _, early := range []bool{true, false} {
		zx, err := newMachine("next", "./rom")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bootNextFrom(zx, f, f.Card, true, false); err != nil {
			t.Fatal(err)
		}
		zx.running = true
		if early {
			if _, err := zx.InsertSecondCard(f.Card, true); err != nil {
				t.Fatal(err)
			}
		}
		resetRun(zx, 1500)
		if !early {
			if _, err := zx.InsertSecondCard(f.Card, true); err != nil {
				t.Fatal(err)
			}
			resetRun(zx, 600)
		}
		if !zx.io.divMMC.sd.served {
			t.Errorf("early=%v: the card the Next booted from was not read", early)
		}
		if got := zx.io.divMMC.sd2.served; got != early {
			t.Errorf("a second card put in %s the firmware started: served = %v, want %v", map[bool]string{true: "before", false: "after"}[early], got, early)
		}
		recs, err := readSnapshotCards(nextSnapshotOf(t, zx), "zxs")
		want := 1
		if early {
			want = 2
		}
		if err != nil || len(recs) != want {
			t.Errorf("early=%v: the snapshot holds %d cards, want %d (%v)", early, len(recs), want, err)
		}
		zx.detachNextBootChain()
	}
}
