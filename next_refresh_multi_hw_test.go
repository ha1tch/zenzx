//go:build headless

package zenzx

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextZXOSTwoMachinesSeeCardWrites runs two concurrent Next machines, each
// with its own boot card and its own second card, and checks that a file
// written to one machine's second card (then re-inserted) is seen by that
// machine after its Browser remount -- and that the two machines stay
// independent. It is the two-machine form of G-07.
//
// The boot cards are minimal (280 MB) cards made bootable from the 26.11 card,
// so the test needs four small cards rather than four 1 GB ones. Gated on
// ZENZX_NEXT_FILES (dormant guard G-08).
func TestNextZXOSTwoMachinesSeeCardWrites(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-08)")
	}
	f := nextcard.In(dir)
	examples, err := nextExamples()
	if err != nil {
		t.Fatal(err)
	}
	var gallop, fuji, orbit nextcard.File
	for _, e := range examples {
		switch e.Name {
		case "gallop.nex":
			gallop = e
		case "fuji.nex":
			fuji = e
		case "orbit.nex":
			orbit = e
		}
	}

	// A machine and the cards it boots with.
	type rig struct {
		zx         *ZenZX
		secondCard string
		added      nextcard.File // the file this machine writes to its own card
	}
	makeRig := func(name string, seed, add nextcard.File) *rig {
		base := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
		// A minimal bootable boot card made from the 26.11 card. MakeBootable's
		// own VerifyCard rejects the real 26.11 card (its FSInfo free-cluster
		// count is approximate, as real cards' are), so the boot files are copied
		// the same way MakeBootable copies them, without that strict gate.
		bootCard, err := newEmptyCard(filepath.Join(base, "boot"), smallestCard)
		if err != nil {
			t.Fatalf("%s boot card: %v", name, err)
		}
		if err := copyBootFiles(f.Card, bootCard); err != nil {
			t.Fatalf("%s make bootable: %v", name, err)
		}
		// A small second card seeded with one file.
		second := filepath.Join(base, "second.img")
		if err := addToOwnCard(second, []nextcard.File{seed}); err != nil {
			t.Fatalf("%s second card: %v", name, err)
		}

		zx, err := newMachine("next", "./rom")
		if err != nil {
			t.Fatalf("%s machine: %v", name, err)
		}
		if _, err := bootNextFrom(zx, f, bootCard, false, false); err != nil {
			t.Fatalf("%s boot: %v", name, err)
		}
		if _, err := zx.InsertSecondCard(second, false); err != nil {
			t.Fatalf("%s second slot: %v", name, err)
		}
		zx.running = true
		return &rig{zx: zx, secondCard: second, added: add}
	}

	// Each card is seeded with one filler file; each machine then adds a
	// different example (A: fuji, B: orbit) to its own second card, so the row
	// each gains after its remount is distinct.
	seedA := nextcard.File{Name: "SEEDA.NEX", Data: gallop.Data}
	seedB := nextcard.File{Name: "SEEDB.NEX", Data: gallop.Data}
	a := makeRig("a", seedA, fuji)
	b := makeRig("b", seedB, orbit)
	defer a.zx.detachNextBootChain()
	defer b.zx.detachNextBootChain()
	defer a.zx.RemoveSecondCard()
	defer b.zx.RemoveSecondCard()
	rigs := []*rig{a, b}

	// Run both machines in lockstep.
	run := func(n int) {
		for i := 0; i < n; i++ {
			a.zx.RunFrame()
			b.zx.RunFrame()
		}
	}
	chordOf := func(names ...string) []matrixPos {
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
		return chord
	}
	// pressAll presses the same key on every machine at the same time and
	// advances them together, so the two machines stay in step (pressing one at
	// a time would advance the others mid-keystroke and desync their input).
	pressAll := func(names ...string) {
		chord := chordOf(names...)
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

	// Boot both to the Browser, on the second card's drive (D).
	run(4000)
	pressAll("space")
	run(150)
	pressAll("enter") // Browser
	run(250)
	pressAll("d") // drive D:
	run(200)

	// Baseline: each Browser now lists one file (the filler). Snapshot the file
	// area of each machine's frame. The Browser draws on the tilemap, so these
	// pixels change when the directory NextZXOS presents gains a row.
	baseA, baseB := inkInFileArea(a.zx), inkInFileArea(b.zx)

	// Each machine writes its own file to its own second card (as the drawer
	// does, card inserted) and re-inserts it (the fix).
	for _, r := range rigs {
		if err := nextcard.AddFiles(context.Background(), r.secondCard, "", []nextcard.File{r.added}, nil); err != nil {
			t.Fatalf("add to %s: %v", r.secondCard, err)
		}
		if _, err := r.zx.InsertSecondCard(r.secondCard, false); err != nil {
			t.Fatal(err)
		}
	}

	// Each machine remounts (M, then Y).
	pressAll("m")
	run(120)
	pressAll("y")
	run(250)

	// After the remount each Browser must list a second file (the added one),
	// so each machine's file area must have gained ink (a new row of text). A
	// machine whose FAT driver could not see the write (the bug) would show the
	// same one-file listing, with an unchanged file area.
	gainA := inkInFileArea(a.zx) - baseA
	gainB := inkInFileArea(b.zx) - baseB
	if gainA <= 0 {
		t.Errorf("machine A: the Browser did not gain a file row after the remount (ink %d -> %d)", baseA, baseA+gainA)
	}
	if gainB <= 0 {
		t.Errorf("machine B: the Browser did not gain a file row after the remount (ink %d -> %d)", baseB, baseB+gainB)
	}
	// Independence is structural: each machine boots its own card and has its
	// own second card in its own DivMMC, so the listings cannot cross; the two
	// machines each gaining their own row confirms both refreshed on their own.
}

// inkInFileArea counts lit (non-background) pixels in the Browser's file-listing
// band of a machine's composed frame. A new file row lands in this band, so the
// count rising means the listing grew. It reads the frame, not any text, so it
// needs no tilemap OCR.
func inkInFileArea(zx *ZenZX) int {
	img := zx.DecodeDisplay()
	if img == nil {
		return 0
	}
	b := img.Bounds()
	// The listing sits below the header; sample the upper-middle band and treat
	// the top-left pixel as the background colour.
	bg := img.RGBAAt(b.Min.X, b.Min.Y)
	y0 := b.Min.Y + b.Dy()/8
	y1 := b.Min.Y + b.Dy()/2
	ink := 0
	for y := y0; y < y1; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R != bg.R || c.G != bg.G || c.B != bg.B {
				ink++
			}
		}
	}
	return ink
}

// copyBootFiles copies the files that make a card bootable (BootFiles plus the
// optional BootExtras) from the source card onto dst, the way MakeBootable does
// but without its VerifyCard gate -- the real 26.11 card's approximate FSInfo
// free count fails that gate though the card boots fine.
func copyBootFiles(source, dst string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	vol, err := nextcard.Open(src)
	if err != nil {
		return err
	}
	var files []nextcard.File
	for _, path := range nextcard.BootFiles {
		n := vol.Root.Find(path)
		if n == nil || n.IsDir() {
			return os.ErrNotExist
		}
		data, err := vol.ReadFile(n)
		if err != nil {
			return err
		}
		files = append(files, nextcard.File{Name: path, Data: data})
	}
	for _, path := range nextcard.BootExtras {
		if n := vol.Root.Find(path); n != nil && !n.IsDir() {
			if data, err := vol.ReadFile(n); err == nil {
				files = append(files, nextcard.File{Name: path, Data: data})
			}
		}
	}
	return nextcard.AddTree(context.Background(), dst, files, nil)
}
