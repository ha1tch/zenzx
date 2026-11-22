//go:build headless

package zenzx

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextZXOSRemountTargetsFocusedMachine checks that, with two Next machines
// running, a host-driven remount refreshes the machine the drop is routed to and
// leaves the other untouched. The mount drawer resolves the focused machine (the
// deck's front) at drop time and runs forceNextZXOSRemount against that machine's
// own memory, so this exercises the same per-machine targeting: a file written to
// machine B's card, with the remount driven on B, must appear on B and not on A
// (dormant guard G-10).
//
// It is the focus-routing form of G-09: G-09 proves the refresh works on one
// machine, this proves it goes to the right machine when there is more than one.
// The drawer type itself is GUI-only (built without the headless tag), so the
// test drives the per-machine API the drawer calls rather than the drawer.
func TestNextZXOSRemountTargetsFocusedMachine(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-10)")
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

	type rig struct {
		zx  *ZenZX
		sec string
	}
	makeRig := func(name string) *rig {
		base := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
		bootCard, err := newEmptyCard(filepath.Join(base, "boot"), smallestCard)
		if err != nil {
			t.Fatalf("%s boot card: %v", name, err)
		}
		if err := copyBootFiles(f.Card, bootCard); err != nil {
			t.Fatalf("%s make bootable: %v", name, err)
		}
		sec := filepath.Join(base, "second.img")
		if err := addToOwnCard(sec, []nextcard.File{{Name: "SEED.NEX", Data: gallop.Data}}); err != nil {
			t.Fatalf("%s second card: %v", name, err)
		}
		zx, err := newMachine("next", "./rom")
		if err != nil {
			t.Fatalf("%s machine: %v", name, err)
		}
		if _, err := bootNextFrom(zx, f, bootCard, false, false); err != nil {
			t.Fatalf("%s boot: %v", name, err)
		}
		if _, err := zx.InsertSecondCard(sec, false); err != nil {
			t.Fatalf("%s second slot: %v", name, err)
		}
		zx.running = true
		return &rig{zx: zx, sec: sec}
	}

	a := makeRig("a")
	b := makeRig("b")
	defer a.zx.detachNextBootChain()
	defer b.zx.detachNextBootChain()
	defer a.zx.RemoveSecondCard()
	defer b.zx.RemoveSecondCard()
	rigs := []*rig{a, b}

	run := func(n int) {
		for i := 0; i < n; i++ {
			a.zx.RunFrame()
			b.zx.RunFrame()
		}
	}
	pressAll := func(name string) {
		var chord []matrixPos
		if c, ok := namedKeys[name]; ok {
			chord = c
		} else if p, ok := baseKeys[[]rune(name)[0]]; ok {
			chord = []matrixPos{p}
		} else {
			t.Fatalf("no key %q", name)
		}
		for _, r := range rigs {
			for _, k := range chord {
				r.zx.io.PressKey(k.row, k.col)
			}
		}
		run(8)
		for _, r := range rigs {
			for _, k := range chord {
				r.zx.io.ReleaseKey(k.row, k.col)
			}
		}
		run(12)
	}

	run(4000)
	pressAll("space")
	run(150)
	pressAll("enter")
	run(250)
	pressAll("d")
	run(200)
	baseA, baseB := inkInFileArea(a.zx), inkInFileArea(b.zx)

	// Focus is on B: the drawer's front() resolver returns B, so the drop is
	// written to B's card and the remount driven on B. (Here front() is modelled
	// directly, since the drawer type is GUI-only.)
	front := func() *ZenZX { return b.zx }
	target := front()

	if err := nextcard.AddFiles(context.Background(), b.sec, "", []nextcard.File{fuji}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := target.InsertSecondCard(b.sec, false); err != nil {
		t.Fatal(err)
	}
	if !target.forceNextZXOSRemount(func() { run(1) }) {
		t.Fatal("forceNextZXOSRemount declined on the focused machine")
	}
	run(60)

	gainA := inkInFileArea(a.zx) - baseA
	gainB := inkInFileArea(b.zx) - baseB
	if gainB <= 0 {
		t.Errorf("the focused machine (B) did not refresh (ink %d -> %d)", baseB, baseB+gainB)
	}
	if gainA != 0 {
		t.Errorf("the unfocused machine (A) changed (ink %d -> %d) -- the remount went to the wrong machine", baseA, baseA+gainA)
	}
}
