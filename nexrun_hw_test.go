//go:build headless

package zenzx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextRunsANEXFromItsMenus runs a NEX the way File > Open and a drop do (RunNEX), against
// the real firmware: from the welcome screen with the second slot empty, from the Browser
// with a card that can be written in it, and, when the Next is busy, after a reset.
//
// Dormant guard G-12 (docs/KNOWN_ISSUES.md): gated on ZENZX_NEXT_FILES, as G-04, G-06 and
// G-11 are. The boot card is opened read-only; every other card is made in a temporary
// folder.
func TestNextRunsANEXFromItsMenus(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-12)")
	}
	examples, err := nextExamples()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	nexPath := map[string]string{}
	for _, ex := range examples {
		p := filepath.Join(tmp, ex.Name)
		if err := os.WriteFile(p, ex.Data, 0o644); err != nil {
			t.Fatal(err)
		}
		nexPath[ex.Name] = p
	}
	gallop := func(zx *ZenZX) bool {
		return zx.io.copper.mode == 3 && zx.io.nextRegs.regs[0x6B] == 0xA9 && zx.io.nextRegs.regs[0x15] == 0x01
	}
	fuji := func(zx *ZenZX) bool {
		return zx.io.copper.mode == 3 && zx.io.nextRegs.regs[0x70] == 0x10 && zx.io.nextRegs.regs[0x6B] == 0x83
	}
	orbit := func(zx *ZenZX) bool { return zx.io.nextInt.hw && zx.cpu.IM == 2 }

	boot := func(t *testing.T, card2 string) *ZenZX {
		t.Helper()
		f := nextcard.In(dir)
		zx, err := newMachine("next", "./rom")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bootNextFrom(zx, f, f.Card, true, false); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { zx.dropNEXScratch(); zx.detachNextBootChain() })
		if card2 != "" {
			if _, err := zx.InsertSecondCard(card2, false); err != nil {
				t.Fatal(err)
			}
		}
		zx.running = true
		var notes []string
		zx.OnNEXRun = func(m string) { notes = append(notes, m) }
		t.Cleanup(func() {
			if t.Failed() {
				t.Logf("notes: %q", notes)
			}
		})
		return zx
	}
	frames := func(zx *ZenZX, n int) {
		for i := 0; i < n; i++ {
			zx.RunFrame()
		}
	}
	// finish runs the machine until the run ends.
	finish := func(t *testing.T, zx *ZenZX) {
		t.Helper()
		n := 0
		for ; n < nexRunGiveUp+500 && zx.NEXRunning(); n++ {
			zx.RunFrame()
			if r := zx.nexRun; r != nil && n%200 == 0 {
				t.Logf("frame %d stage %d inStage %d idle %d busy %d pc %04x", n, r.stage, r.inStage, r.idle, r.busy, zx.cpu.PC)
			}
		}
		t.Logf("the run took %d frames", n)
		if zx.NEXRunning() {
			t.Fatal("the run did not end")
		}
		frames(zx, 60)
	}
	key := func(zx *ZenZX, names ...string) {
		for _, n := range names {
			for _, k := range namedKeys[n] {
				zx.io.PressKey(k.row, k.col)
			}
		}
		frames(zx, 6)
		for _, n := range names {
			for _, k := range namedKeys[n] {
				zx.io.ReleaseKey(k.row, k.col)
			}
		}
		frames(zx, 8)
	}

	t.Run("from the welcome screen, slot 2 empty", func(t *testing.T) {
		zx := boot(t, "")
		frames(zx, 600)
		msg, reset := zx.RunNEX(nexPath["gallop.nex"], false)
		if reset {
			t.Fatalf("a Next at its welcome screen asked for a reset: %s", msg)
		}
		finish(t, zx)
		if !gallop(zx) {
			t.Errorf("gallop is not running (%s)", msg)
		}
	})

	t.Run("from the Browser, a card that can be written in slot 2", func(t *testing.T) {
		card := filepath.Join(tmp, "own.img")
		if err := addToOwnCard(card, nil); err != nil {
			// an empty card: made from nothing
			if _, err := newEmptyCard(card, smallestCard); err != nil {
				t.Fatal(err)
			}
		}
		zx := boot(t, card)
		frames(zx, 600)
		key(zx, "space") // the menu
		frames(zx, 150)
		key(zx, "enter") // the Browser
		frames(zx, 300)
		if _, reset := zx.RunNEX(nexPath["fuji.nex"], false); reset {
			t.Fatal("a Next at its Browser asked for a reset")
		}
		finish(t, zx)
		if !fuji(zx) {
			t.Error("fuji is not running")
		}
		if zx.SecondCard() != card {
			t.Errorf("slot 2 holds %q, want the card that was there", zx.SecondCard())
		}
	})

	t.Run("a scratch card that runs it by itself", func(t *testing.T) {
		zx := boot(t, "")
		frames(zx, 600)
		data, err := os.ReadFile(nexPath["fuji.nex"])
		if err != nil {
			t.Fatal(err)
		}
		r := &nexRun{name: nexScratchName, label: "fuji.nex", keys: NewKeyQueue(6, 8), data: data}
		zx.nexRun = r
		if err := zx.bootNEXScratch(r); err != nil {
			t.Fatal(err)
		}
		finish(t, zx)
		if !fuji(zx) {
			t.Error("fuji is not running from the scratch card")
		}
		if zx.next.scratchDir == "" || zx.next.home == nil {
			t.Error("the chain does not remember its scratch card and where it came from")
		}
		dir := zx.next.scratchDir
		zx.detachNextBootChain()
		if _, err := os.Stat(dir); err == nil {
			t.Error("the scratch card's folder was not removed")
		}
	})

	t.Run("busy: asks, then resets and runs", func(t *testing.T) {
		zx := boot(t, "")
		frames(zx, 600)
		zx.RunNEX(nexPath["gallop.nex"], false)
		finish(t, zx)
		if !gallop(zx) {
			t.Fatal("gallop did not start")
		}
		msg, reset := zx.RunNEX(nexPath["orbit.nex"], false)
		if !reset {
			t.Fatalf("a Next running a program did not ask for a reset: %s", msg)
		}
		if zx.NEXRunning() {
			t.Fatal("asking changed the machine")
		}
		if _, reset := zx.RunNEX(nexPath["orbit.nex"], true); reset {
			t.Fatal("still asking after the reset was allowed")
		}
		finish(t, zx)
		if !orbit(zx) {
			t.Error("orbit is not running after the reset")
		}
	})
}
