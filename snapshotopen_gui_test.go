//go:build !headless

package zenzx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotDeck is a deck of one 48K whose snapshots are opened as the window opens them: by the
// deck, which makes the machine each is of.
func snapshotDeck(t *testing.T) (*machineDeck, *[]string) {
	t.Helper()
	d := testDeck(t, 1)
	d.dockOn = func() bool { return false } // the dock slid in for a made machine is the GL guard's (dockreveal_gl_test.go)
	var retyped []string
	d.retype = func(zx *ZenZX, model string) {
		retyped = append(retyped, model)
		switchModelLive(zx, model, "./rom")
	}
	for _, m := range d.machines {
		m.zx.OpenSnapshot = d.openSnapshot
	}
	return d, &retyped
}

// snapshotOf saves a machine of model as a snapshot file, and gives its path.
func snapshotOf(t *testing.T, model, name string) string {
	t.Helper()
	zx, err := newMachine(model, "./rom")
	if err != nil {
		t.Skipf("%s: %v", model, err)
	}
	resetRun(zx, 10)
	path := filepath.Join(t.TempDir(), name)
	if err := zx.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func classes(d *machineDeck) []machineClass {
	var out []machineClass
	for _, m := range d.machines {
		out = append(out, m.zx.machineClass())
	}
	return out
}

// A snapshot of another kind than the front machine's is loaded into a new machine of its kind, which
// is brought to the front and is the one that plays it; the machine that was there is as it was.
func TestASnapshotOfAnotherKindMakesANewMachine(t *testing.T) {
	d, _ := snapshotDeck(t)
	first := d.Front()
	pc := first.cpu.PC
	path := snapshotOf(t, "128k", "g.zxs")
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatal(err)
	}
	if got := classes(d); len(got) != 2 || got[0] != class48K || got[1] != class128K {
		t.Fatalf("the deck holds %v, want a 48K and then a 128K", got)
	}
	if d.front != 1 || d.Front() == first {
		t.Errorf("the front machine is %d: the one made for the snapshot is not shown", d.front)
	}
	if d.machines[1].model != "128k" {
		t.Errorf("the new machine is a %q", d.machines[1].model)
	}
	if d.machines[1].fromSnapshot != snapshotKey(path) {
		t.Errorf("the new machine does not know the file it was made for: %q", d.machines[1].fromSnapshot)
	}
	if first.cpu.PC != pc || first.paused {
		t.Errorf("the machine that was running was changed (pc %04X to %04X, paused %v)", pc, first.cpu.PC, first.paused)
	}
}

// The same snapshot opened again is loaded into the machine it was made for, and not followed by
// another of the same; another file of the same kind is a machine of its own.
func TestTheSameSnapshotAgainReloadsItsMachine(t *testing.T) {
	d, _ := snapshotDeck(t)
	path := snapshotOf(t, "128k", "g.zxs")
	other := snapshotOf(t, "128k", "h.zxs")
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatal(err)
	}
	made := d.Front()
	made.cpu.PC = 0x1234 // the machine has moved on
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatal(err)
	}
	if len(d.machines) != 2 || d.Front() != made {
		t.Fatalf("opened again: %d machines, the same front %v", len(d.machines), d.Front() == made)
	}
	if made.cpu.PC == 0x1234 {
		t.Error("the snapshot was not loaded again into the machine")
	}
	if err := d.openSnapshot(other, "", false); err != nil {
		t.Fatal(err)
	}
	if len(d.machines) != 3 || d.Front() == made {
		t.Errorf("another file of the same kind: %d machines, want 3, and a new one in front", len(d.machines))
	}
}

// With no machine running a snapshot starts the one it is of, in the window.
func TestASnapshotWithNoMachineStartsTheMachineItIsOf(t *testing.T) {
	for _, model := range []string{"48k", "128k", "plus3"} {
		d, _ := snapshotDeck(t)
		path := snapshotOf(t, model, "g.zxs")
		d.makeIdle()
		if !d.Empty() || !d.Front().stopped {
			t.Fatal("the deck was not emptied")
		}
		if err := d.openSnapshot(path, "", false); err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		if d.Empty() || len(d.machines) != 1 {
			t.Fatalf("%s: %d machines after the snapshot", model, len(d.machines))
		}
		zx := d.Front()
		if zx.stopped || zx.machineClass() != classOfModel(model) || d.frontModel() != model {
			t.Errorf("%s: the machine started is stopped %v, kind %v, model %q", model, zx.stopped, zx.machineClass(), d.frontModel())
		}
		if zx.display != Display(d.dm) {
			t.Errorf("%s: the machine started was not given the window", model)
		}
	}
}

// Where adding machines is off, the snapshot goes into the front machine, which becomes the model it
// needs when it is not of that kind, and stays what it is when it is.
func TestWithAddingMachinesOffTheFrontMachineTakesTheSnapshot(t *testing.T) {
	d, retyped := snapshotDeck(t)
	d.multiOn = func() bool { return false }
	first := d.Front()
	same := snapshotOf(t, "48k", "a.zxs")
	if err := d.openSnapshot(same, "", false); err != nil {
		t.Fatal(err)
	}
	if len(d.machines) != 1 || d.Front() != first || len(*retyped) != 0 {
		t.Errorf("a 48K snapshot into a 48K: %d machines, retyped %v", len(d.machines), *retyped)
	}
	other := snapshotOf(t, "plus3", "b.zxs")
	if err := d.openSnapshot(other, "", false); err != nil {
		t.Fatal(err)
	}
	if len(d.machines) != 1 || d.Front() != first {
		t.Fatalf("a +3 snapshot: %d machines, the front is not the one that was there", len(d.machines))
	}
	if first.machineClass() != classPlus3 || d.frontModel() != "plus3" || len(*retyped) != 1 || (*retyped)[0] != "plus3" {
		t.Errorf("the machine is of kind %v, model %q, retyped %v", first.machineClass(), d.frontModel(), *retyped)
	}
}

// A snapshot that is not loaded leaves nothing behind: not the machine made for it, and not an
// emptied window filled.
func TestASnapshotThatFailsLeavesNoMachineBehind(t *testing.T) {
	d, _ := snapshotDeck(t)
	good := snapshotOf(t, "128k", "g.zxs")
	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(t.TempDir(), "bad.zxs")
	os.WriteFile(bad, data[:40], 0o644) // the header, and the first chunk cut short

	if err := d.openSnapshot(bad, "", false); err == nil {
		t.Fatal("a cut snapshot was loaded")
	}
	if len(d.machines) != 1 || d.front != 0 {
		t.Errorf("a failed snapshot left %d machines, front %d", len(d.machines), d.front)
	}
	// into an empty window
	d.makeIdle()
	if err := d.openSnapshot(bad, "", false); err == nil {
		t.Fatal("a cut snapshot was loaded into an empty window")
	}
	if !d.Empty() || !d.Front().stopped {
		t.Error("a failed snapshot started a machine in an empty window")
	}
	// a file that is no snapshot, or not there, makes nothing
	for _, p := range []string{filepath.Join(t.TempDir(), "nothing.sna"), "testdata/synthetic.dsk"} {
		if err := d.openSnapshot(p, "", false); err == nil {
			t.Errorf("%s was opened", p)
		}
	}
	if !d.Empty() {
		t.Error("a file that is no snapshot started a machine")
	}
}

// Every way a file reaches a machine ends in the deck: the dropped file, Quick Load's file, and the
// menu's rows all go through loadDroppedFile or openSnapshot of the machine that is shown.
func TestADroppedSnapshotGoesThroughTheDeck(t *testing.T) {
	d, _ := snapshotDeck(t)
	path := snapshotOf(t, "128k", "g.zxs")
	msg := d.Front().loadDroppedFile(path)
	if !strings.Contains(msg, "has been restored") || len(d.machines) != 2 || d.Front().machineClass() != class128K {
		t.Errorf("dropped: %q, %d machines, front of kind %v", msg, len(d.machines), d.Front().machineClass())
	}
	// the one that does not fit is said to, in words, where no machine can be made
	d2, _ := snapshotDeck(t)
	d2.multiOn = func() bool { return false }
	d2.retype = nil
	msg = d2.Front().loadDroppedFile(path)
	if msg != "g.zxs is a 128K snapshot and this machine is a 48K" {
		t.Errorf("refused with no way to make the machine: %q", msg)
	}
}

// With no machine running only a snapshot can be opened: anything else says there is none.
func TestWithNoMachineOnlyASnapshotCanBeDropped(t *testing.T) {
	d, _ := snapshotDeck(t)
	d.makeIdle()
	idle := d.Front()
	idle.OpenSnapshot = d.openSnapshot
	for _, f := range []string{"x.tap", "x.tzx", "x.dsk", "x.scr", "x.rzx", "x.nex"} {
		if msg := idle.loadDroppedFile(f); !strings.HasPrefix(msg, "No machine is running") {
			t.Errorf("%s dropped on a stopped window: %q", f, msg)
		}
	}
	path := snapshotOf(t, "48k", "g.zxs")
	if msg := idle.loadDroppedFile(path); !strings.Contains(msg, "has been restored") || d.Empty() {
		t.Errorf("a snapshot dropped on a stopped window: %q, empty %v", msg, d.Empty())
	}
}

// With no window to make a machine, openSnapshot loads into the machine it is given, and refuses
// what does not fit.
func TestWithNoHostASnapshotIsOpenedInThisMachine(t *testing.T) {
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skip(err)
	}
	if err := zx.openSnapshot("testdata/snapshots/z88dk_zx48.sna", "sna", false); err != nil {
		t.Fatal(err)
	}
	if err := zx.openSnapshot("testdata/snapshots/z88dk_zx128.sna", "sna", false); err == nil {
		t.Error("a 128K snapshot was opened in a 48K")
	}
	// a host that is set decides
	var asked string
	zx.OpenSnapshot = func(path, format string, wait bool) error { asked = path + "|" + format; return nil }
	if err := zx.openSnapshot("a.sna", "sna", false); err != nil || asked != "a.sna|sna" {
		t.Errorf("the host was asked %q (%v)", asked, err)
	}
}

// The machine a snapshot is loaded into waits paused or plays as Preferences say, except after Quick
// Load, which waits whatever they say, as it always has.
func TestTheMachineOfASnapshotPlaysOrWaitsAsPreferencesSay(t *testing.T) {
	was := snapshotStartsPlaying
	snapshotStartsPlaying = func() bool { return true }
	t.Cleanup(func() { snapshotStartsPlaying = was })
	d, _ := snapshotDeck(t)
	path := snapshotOf(t, "128k", "g.zxs")
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatal(err)
	}
	if d.Front().paused {
		t.Error("Preferences say play at once, and the machine of the snapshot waits")
	}
	if err := d.openSnapshot(path, "", true); err != nil {
		t.Fatal(err)
	}
	if !d.Front().paused {
		t.Error("Quick Load left the machine playing")
	}
}

// A Next's snapshot makes a Next, which was a +3 without its drive until the snapshot had a kind of
// its own (T-99); a snapshot of another kind makes its machine beside it, and with adding machines
// off, a Next's front machine cannot take it and is not changed.
func TestANextSnapshotMakesANextMachine(t *testing.T) {
	d, _ := snapshotDeck(t)
	path := snapshotOf(t, "next", "n.zxs")
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatal(err)
	}
	if got := classes(d); len(got) != 2 || got[0] != class48K || got[1] != classNext {
		t.Fatalf("the deck holds %v, want a 48K and then a Next", got)
	}
	if d.machines[1].model != "next" || d.front != 1 {
		t.Errorf("the new machine is a %q, the front is %d", d.machines[1].model, d.front)
	}
	// the same file again is loaded into the machine it made, not followed by another
	if err := d.openSnapshot(path, "", false); err != nil || len(d.machines) != 2 {
		t.Errorf("the same Next snapshot again: %d machines, %v", len(d.machines), err)
	}
	// a 48K snapshot with the Next in front makes a 48K beside it
	k := snapshotOf(t, "48k", "k.zxs")
	if err := d.openSnapshot(k, "", false); err != nil {
		t.Fatal(err)
	}
	if got := classes(d); len(got) != 3 || got[2] != class48K {
		t.Errorf("the deck holds %v", got)
	}

	// with adding machines off, a window of one Next, and no way to retype it, refuses a 48K
	d2 := testDeck(t, 1)
	d2.dockOn = func() bool { return false }
	d2.multiOn = func() bool { return false }
	d2.machines[0].zx.OpenSnapshot = d2.openSnapshot
	if err := d2.openSnapshot(path, "", false); err != nil { // a Next into a 48K with no retype: refused
		var mm *snapshotMismatch
		if !errors.As(err, &mm) {
			t.Errorf("error %v, want a mismatch", err)
		}
	} else {
		t.Error("a Next snapshot was loaded into a 48K with nothing to retype it")
	}
	if len(d2.machines) != 1 || d2.machines[0].zx.machineClass() != class48K {
		t.Error("the refused snapshot changed the machine")
	}
}
