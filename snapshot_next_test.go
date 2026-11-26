//go:build headless

package zenzx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A Next's snapshot (snapshot_next.go): the machine's whole state, which a Next puts back as it was.

func nextSnapshotOf(t *testing.T, zx *ZenZX) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "next.zxs")
	if err := zx.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// A Next saved and loaded into another Next is the same machine, to the last field the boot
// cache keeps, and is paused as a loaded snapshot is.
func TestANextSnapshotPutsTheNextBack(t *testing.T) {
	a := bareNext(t)
	pokeNext(a)
	path := nextSnapshotOf(t, a)

	b := bareNext(t)
	if dumpOf(t, a) == dumpOf(t, b) {
		t.Fatal("the poking changed nothing, so the test proves nothing")
	}
	if err := b.LoadSnapshotFormat(path, "zxs"); err != nil {
		t.Fatal(err)
	}
	if da, db := dumpOf(t, a), dumpOf(t, b); da != db {
		t.Fatalf("a Next loaded from its snapshot differs: %s", firstDifference(da, db))
	}
	if !b.paused {
		t.Error("the Next waits running, as nothing loaded does")
	}
}

// The file says what it is of: the header, and the reading of it that makes the machine.
func TestANextSnapshotIsOfANext(t *testing.T) {
	a := bareNext(t)
	path := nextSnapshotOf(t, a)
	got, err := readSnapshotMachine(path, "")
	if err != nil || got != (snapshotMachine{classNext, "next"}) {
		t.Errorf("a Next's .zxs is of %+v (%v), want a Next", got, err)
	}
	if a.machineClass() != classNext || classOfModel("next") != classNext {
		t.Errorf("a Next is of kind %v, its model's is %v", a.machineClass(), classOfModel("next"))
	}
	image, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var h SnapshotHeader
	if err := binary.Read(bytes.NewReader(image), binary.LittleEndian, &h); err != nil {
		t.Fatal(err)
	}
	if h.Model != snapshotModelNext || h.Flags&FlagNext == 0 || h.Flags&(FlagPlus3|FlagFDC) != 0 {
		t.Errorf("the header says model %d, flags %b: not a Next, or still a +3", h.Model, h.Flags)
	}
	// a snapshot saved by a build that had no Next's own ("a +3 with no disc controller") is no Next's
	if m := snapshotMachineOfHeader(SnapshotHeader{Model: 4, Flags: FlagPlus3 | Flag128K}); m.Class != classPlus3 || m.Model != "plus2a" {
		t.Errorf("a +3 header with no controller is %+v", m)
	}
}

// A snapshot is loaded into the machine it is of: a Next's into a 48K and a 48K's into a Next are
// refused, with the reason, and the machine is as it was.
func TestANextSnapshotAndTheOtherKindsAreNotMixed(t *testing.T) {
	a := bareNext(t)
	pokeNext(a)
	nextFile := nextSnapshotOf(t, a)

	zx48, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skip(err)
	}
	pc := zx48.cpu.PC
	err = zx48.LoadSnapshotFormat(nextFile, "zxs")
	m, ok := snapshotMismatchOf(err)
	if !ok || m.Snapshot != classNext || m.Machine != class48K {
		t.Fatalf("a Next's snapshot into a 48K: %v, want a refusal that says so", err)
	}
	if zx48.cpu.PC != pc {
		t.Error("the 48K was changed by a snapshot it refused")
	}
	if got := snapshotRefusal("n.zxs", err); got != "n.zxs is a Next snapshot and this machine is a 48K" {
		t.Errorf("the window says %q", got)
	}

	before := dumpOf(t, a)
	err = a.LoadSnapshotFormat("testdata/snapshots/z88dk_zx48.sna", "sna")
	if m, ok := snapshotMismatchOf(err); !ok || m.Snapshot != class48K || m.Machine != classNext {
		t.Fatalf("a 48K snapshot into a Next: %v, want a refusal that says so", err)
	}
	if dumpOf(t, a) != before {
		t.Error("the Next was changed by a snapshot it refused")
	}
}

// A snapshot of another build's layout is refused, in words, and the machine is not touched.
func TestANextSnapshotOfAnotherBuildIsRefused(t *testing.T) {
	a := bareNext(t)
	pokeNext(a)
	path := nextSnapshotOf(t, a)
	image, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// header, chunk header, the state's magic: the next 32 bytes are the layout's fingerprint
	at := binary.Size(SnapshotHeader{}) + binary.Size(ChunkHeader{}) + len("ZXMSTATE1\n")
	image[at] ^= 0xFF
	os.WriteFile(path, image, 0o644)

	b := bareNext(t)
	before := dumpOf(t, b)
	err = b.LoadSnapshotFormat(path, "zxs")
	if !errors.Is(err, errNextSnapshotOtherBuild) {
		t.Fatalf("a snapshot of another layout: %v, want the refusal for another build", err)
	}
	if got := snapshotRefusal("n.zxs", err); got != "n.zxs was saved by another build of ZenZX and cannot be loaded" {
		t.Errorf("the window says %q", got)
	}
	if dumpOf(t, b) != before {
		t.Error("the Next was changed by a snapshot it refused")
	}
}

// A boot cache that was to be put over the machine, or made from it, is called off by a snapshot:
// it would put the boot's state where the snapshot's is.
func TestANextSnapshotCallsOffTheBootCache(t *testing.T) {
	a := bareNext(t)
	path := nextSnapshotOf(t, a)

	b := bareNext(t)
	b.next = &nextChain{cachePending: true}
	b.onNextHandOver = func() {}
	b.watchNextSettle(func(int) {}, func() {})
	if err := b.LoadSnapshotFormat(path, "zxs"); err != nil {
		t.Fatal(err)
	}
	if b.next.cachePending || b.onNextHandOver != nil || b.nextSettle != nil {
		t.Errorf("after the snapshot a boot cache is still to be made: pending %v, hand-over %v, settle %v",
			b.next.cachePending, b.onNextHandOver != nil, b.nextSettle != nil)
	}
}
