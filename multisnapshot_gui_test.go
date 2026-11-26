//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

const markAddr = 0x8000 // a byte of RAM, in every model, that a test marks a machine with

func markDeck(d *machineDeck, base byte) {
	for i, m := range d.machines {
		m.zx.memory.Write(markAddr, base+byte(i))
	}
}

func marks(d *machineDeck) []byte {
	var out []byte
	for _, m := range d.machines {
		out = append(out, m.zx.memory.Read(markAddr))
	}
	return out
}

// A multisnapshot of three machines saves them at one moment, leaves each as running or paused as
// it was, and loading it puts three machines in place of the ones that are there: the contents,
// the names, the front machine and the layout are the saved ones.
func TestMultiSnapshotSaveAndLoad(t *testing.T) {
	d := testDeck(t, 3)
	markDeck(d, 10)
	d.rename(0, "alpha")
	d.rename(2, "gamma")
	d.machines[1].zx.paused = true
	d.flip(2)
	d.setLayout(layoutColumns)
	if d.tiling != layoutColumns {
		t.Fatal("setting up: the layout did not take")
	}
	path := filepath.Join(t.TempDir(), "three.zms")
	if err := d.saveMulti(path); err != nil {
		t.Fatal(err)
	}
	for i, want := range []bool{false, true, false} {
		if d.machines[i].zx.paused != want {
			t.Errorf("machine %d paused = %v after the save, want what it was: %v", i+1, !want, want)
		}
	}

	// the window changes: other contents, another layout, another front machine
	markDeck(d, 90)
	d.flip(0)
	d.setLayout(layoutRows)
	before := append([]*deckMachine(nil), d.machines...)

	n, err := d.loadMulti(path)
	if err != nil || n != 3 {
		t.Fatalf("loadMulti: %d machines, %v", n, err)
	}
	if len(d.machines) != 3 {
		t.Fatalf("%d machines after the load, want 3", len(d.machines))
	}
	for i, m := range d.machines {
		for _, old := range before {
			if m == old {
				t.Errorf("machine %d is one of the machines that was running: they were to be replaced", i+1)
			}
		}
	}
	if got := marks(d); got[0] != 10 || got[1] != 11 || got[2] != 12 {
		t.Errorf("contents %v, want the saved 10 11 12", got)
	}
	if d.machines[0].name != "alpha" || !d.machines[0].renamed || d.machines[2].name != "gamma" {
		t.Errorf("names %q %q", d.machines[0].name, d.machines[2].name)
	}
	if d.front != 2 || d.Front() != d.machines[2].zx {
		t.Errorf("front is %d, want 2", d.front)
	}
	if d.tiling != layoutColumns {
		t.Errorf("layout is %d, want Side by Side", d.tiling)
	}
	if !d.machines[0].zx.paused { // a snapshot is loaded waiting, as Preferences have it by default
		t.Error("a loaded machine runs, with Preferences saying a snapshot waits")
	}
}

// Machines of different kinds come back as machines of those kinds.
func TestMultiSnapshotOfMixedMachines(t *testing.T) {
	d := testDeck(t, 1)
	d.add("128k")
	markDeck(d, 40)
	path := filepath.Join(t.TempDir(), "mix.zms")
	if err := d.saveMulti(path); err != nil {
		t.Fatal(err)
	}
	other := testDeck(t, 2) // two 48K: the second has to become a 128K
	if _, err := other.loadMulti(path); err != nil {
		t.Fatal(err)
	}
	if other.machines[0].zx.machineClass() != class48K || other.machines[1].zx.machineClass() != class128K {
		t.Errorf("the machines are %v and %v", other.machines[0].zx.machineClass(), other.machines[1].zx.machineClass())
	}
	if other.machines[1].model != "128k" {
		t.Errorf("the second machine is a %q", other.machines[1].model)
	}
	if got := marks(other); got[0] != 40 || got[1] != 41 {
		t.Errorf("contents %v", got)
	}
}

// A multisnapshot that cannot be loaded leaves the window as it was: the machines that were running
// are the same machines.
func TestMultiSnapshotThatFailsChangesNothing(t *testing.T) {
	d := testDeck(t, 2)
	markDeck(d, 1)
	good := filepath.Join(t.TempDir(), "good.zms")
	if err := d.saveMulti(good); err != nil {
		t.Fatal(err)
	}
	// the same file with the second machine's snapshot replaced by one that is damaged
	man, snaps, err := readMultiSnapshot(good)
	if err != nil {
		t.Fatal(err)
	}
	snaps[1] = snaps[1][:len(snaps[1])/2]
	bad := filepath.Join(t.TempDir(), "bad.zms")
	if err := writeMultiSnapshot(bad, man, snaps); err != nil {
		t.Fatal(err)
	}
	markDeck(d, 50)
	before := append([]*deckMachine(nil), d.machines...)
	front := d.front
	if _, err := d.loadMulti(bad); err == nil {
		t.Fatal("a damaged multisnapshot was loaded")
	} else if !strings.Contains(err.Error(), "machine 2") {
		t.Errorf("the error does not say which machine: %v", err)
	}
	if len(d.machines) != 2 || d.machines[0] != before[0] || d.machines[1] != before[1] || d.front != front {
		t.Error("the window is not as it was")
	}
	if got := marks(d); got[0] != 50 || got[1] != 51 {
		t.Errorf("the machines' contents changed: %v", got)
	}
	// not a multisnapshot at all
	junk := filepath.Join(t.TempDir(), "junk.zms")
	os.WriteFile(junk, []byte("junk"), 0o644)
	if _, err := d.loadMulti(junk); err == nil || len(d.machines) != 2 || d.machines[0] != before[0] {
		t.Errorf("junk: %v", err)
	}
}

// With Multi off in Preferences the window is of one machine: a multisnapshot of several is refused.
func TestMultiSnapshotRefusedWhenMultiIsOff(t *testing.T) {
	d := testDeck(t, 2)
	path := filepath.Join(t.TempDir(), "two.zms")
	if err := d.saveMulti(path); err != nil {
		t.Fatal(err)
	}
	d.multiOn = func() bool { return false }
	before := d.machines[0]
	if _, err := d.loadMulti(path); err == nil || !strings.Contains(err.Error(), "Multi") {
		t.Errorf("error %v", err)
	}
	if len(d.machines) != 2 || d.machines[0] != before {
		t.Error("the window changed")
	}
}

// With no machine running a multisnapshot loads into the empty window, and one machine cannot be
// saved as a multisnapshot.
func TestMultiSnapshotIntoAnEmptyWindow(t *testing.T) {
	src := testDeck(t, 2)
	markDeck(src, 70)
	path := filepath.Join(t.TempDir(), "two.zms")
	if err := src.saveMulti(path); err != nil {
		t.Fatal(err)
	}
	d := testDeck(t, 1)
	if err := d.saveMulti(filepath.Join(t.TempDir(), "one.zms")); err == nil {
		t.Error("a multisnapshot of one machine was saved")
	}
	d.makeIdle()
	if !d.Empty() {
		t.Fatal("setting up: the window is not empty")
	}
	if n, err := d.loadMulti(path); err != nil || n != 2 || len(d.machines) != 2 || d.Empty() {
		t.Fatalf("%d machines, %v", len(d.machines), err)
	}
	if got := marks(d); got[0] != 70 || got[1] != 71 {
		t.Errorf("contents %v", got)
	}
}

// The Snapshot menu: Save Multisnapshot is for two machines or more, Load Multisnapshot is for any.
func TestMultisnapshotRowsOfTheMenu(t *testing.T) {
	inSnapshotDir(t)
	b, zx := newTestBar(t, nil, "")
	count := 1
	b.layoutState = func() (deckLayout, int) { return layoutSingle, count }
	row := func(label string) zenui.Item {
		b.refreshSnapshotItemsFor(zx)
		return b.widget.ItemsFor(barSnapshot)[snapRow(t, b, label)]
	}
	if !row("Save Multisnapshot...").Disabled {
		t.Error("Save Multisnapshot... is enabled with one machine")
	}
	if row("Load Multisnapshot...").Disabled {
		t.Error("Load Multisnapshot... is disabled")
	}
	count = 2
	if row("Save Multisnapshot...").Disabled {
		t.Error("Save Multisnapshot... is disabled with two machines")
	}
	// the rows open the file browsers
	b.snapSpec.pick(zx, snapRow(t, b, "Save Multisnapshot..."), -1)
	if b.diskDialog == nil || b.dialogKind != dialogSaveMulti {
		t.Errorf("Save Multisnapshot... opened %v of kind %v", b.diskDialog, b.dialogKind)
	}
	b.diskDialog, b.dialogKind = nil, dialogDisk
	b.snapSpec.pick(zx, snapRow(t, b, "Load Multisnapshot..."), -1)
	if b.diskDialog == nil || b.dialogKind != dialogLoadMulti {
		t.Errorf("Load Multisnapshot... opened %v of kind %v", b.diskDialog, b.dialogKind)
	}
}

// Loading a multisnapshot over running machines says so and asks for YES; anything else, and Cancel,
// leave the machines alone; with none running it loads at once.
func TestLoadMultisnapshotAsksForYes(t *testing.T) {
	inSnapshotDir(t)
	dir := t.TempDir()
	src := testDeck(t, 2)
	path := filepath.Join(dir, "two.zms")
	if err := src.saveMulti(path); err != nil {
		t.Fatal(err)
	}
	b, zx := newTestBar(t, nil, "")
	running := 3
	b.layoutState = func() (deckLayout, int) { return layoutSingle, running }
	var loaded []string
	b.loadMulti = func(p string, _ cardPlan) (int, error) { loaded = append(loaded, p); return 2, nil }

	b.loadMultiSnapshotAt(zx, path)
	q := b.replaceAsk
	if q == nil || len(loaded) != 0 || !b.modalOpen() || b.modalDrawer(800, 600) == nil {
		t.Fatalf("with machines running: ask %v, loaded %v", q, loaded)
	}
	said := strings.Join(q.said, " ")
	for _, want := range []string{"two.zms", "3 machines", "closed and replaced", "its 2 machines", replaceWord} {
		if !strings.Contains(said, want) {
			t.Errorf("the question says %q, which lacks %q", said, want)
		}
	}
	// the wrong word: asked again, with the reason, and nothing loaded
	q.text = "yes"
	b.answerReplace(zx, zenui.Accepted, false)
	if b.replaceAsk == nil || len(loaded) != 0 || q.why == "" {
		t.Fatalf("a lower case yes: ask %v, loaded %v, why %q", b.replaceAsk, loaded, q.why)
	}
	// Cancel
	b.answerReplace(zx, zenui.Cancelled, false)
	if b.replaceAsk != nil || len(loaded) != 0 {
		t.Fatalf("cancelled: ask %v, loaded %v", b.replaceAsk, loaded)
	}
	// YES
	b.loadMultiSnapshotAt(zx, path)
	b.replaceAsk.text = replaceWord
	b.answerReplace(zx, zenui.Accepted, false)
	if b.replaceAsk != nil || len(loaded) != 1 || loaded[0] != path {
		t.Fatalf("YES: ask %v, loaded %v", b.replaceAsk, loaded)
	}
	if len(b.recentSnaps) != 1 || filepath.Base(b.recentSnaps[0]) != "two.zms" {
		t.Errorf("recent snapshots %v: a multisnapshot that was loaded is one", b.recentSnaps)
	}
	// no machine running: nothing to lose, so no question
	running, loaded = 0, nil
	b.loadMultiSnapshotAt(zx, path)
	if b.replaceAsk != nil || len(loaded) != 1 {
		t.Errorf("with none running: ask %v, loaded %v", b.replaceAsk, loaded)
	}
	// a file that is not a multisnapshot is refused before any question
	junk := filepath.Join(dir, "junk.zms")
	os.WriteFile(junk, []byte("junk"), 0o644)
	running, loaded = 2, nil
	b.loadMultiSnapshotAt(zx, junk)
	if b.replaceAsk != nil || len(loaded) != 0 {
		t.Errorf("junk: ask %v, loaded %v", b.replaceAsk, loaded)
	}
}

// Manage Snapshots lists a multisnapshot, says what loading it does, loads it by its own way, and
// does not let a rename make it another format.
func TestSnapshotsWindowHandlesMultisnapshots(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "plain.zxs")
	z := touch(t, dir, "pair.zms")
	touch(t, dir, "quicksave.zms")
	w := newSnapshotsWindow(dir)
	var found snapshotFile
	for _, f := range w.all {
		if f.Name == "pair.zms" {
			found = f
		}
		if f.Name == "quicksave.zms" && f.Quick {
			t.Error("a multisnapshot called quicksave is taken for the quick save")
		}
	}
	if found.Path != z || found.Format != "zms" {
		t.Fatalf("pair.zms is listed as %+v", found)
	}
	if note := snapshotKind(found); !strings.Contains(note, "multisnapshot") {
		t.Errorf("the note says %q", note)
	}
	w.selectPath(z)
	if got := w.do(snapBtnLoad); got != snapshotsLoad || w.loadPath != z {
		t.Errorf("Load: %v, %q", got, w.loadPath)
	}
	if _, why := snapshotRenameTarget(z, "pair.zxs"); why == "" {
		t.Error("a multisnapshot was renamed to another format")
	}
	if _, why := snapshotRenameTarget(filepath.Join(dir, "plain.zxs"), "plain.zms"); why == "" {
		t.Error("a snapshot was renamed to a multisnapshot")
	}
}
