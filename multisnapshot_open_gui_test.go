//go:build !headless

package zenzx

import (
	"path/filepath"
	"strings"
	"testing"
)

func writeSampleZMS(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	man, snaps := sampleManifest(2)
	if err := writeMultiSnapshot(path, man, snaps); err != nil {
		t.Fatal(err)
	}
	return path
}

// A multisnapshot goes the way every other file does: File > Open, Load Snapshot..., Recent
// Snapshots, and a file dropped on the window all end in loadDroppedFile, which gives it to the
// window; it is never copied to a Next's card.
func TestAMultisnapshotIsOpenedLikeAnyOtherFile(t *testing.T) {
	for name, list := range map[string][]string{"File > Open": fileOpenExts, "Load Snapshot...": snapshotLoadExts} {
		found := false
		for _, e := range list {
			found = found || e == MultiSnapshotExt
		}
		if !found {
			t.Errorf("%s does not offer .zms", name)
		}
	}
	if !machineLoadExts[".zms"] {
		t.Error("a dropped .zms would be mounted on a card: it is not a machine-load type")
	}
	// at a Next's prompt with a card, a .zms dropped with a tape is loaded, and not copied to the card
	plan := planDrop([]string{"/x/g.tap", "/x/pair.zms"}, dropContext{atPrompt: true, hasSlots: true, plain: dropCopy, shift: dropRun})
	if len(plan.copyToCard) != 1 || plan.copyToCard[0] != "/x/g.tap" || len(plan.load) != 1 || plan.load[0] != "/x/pair.zms" {
		t.Errorf("the plan is %+v", plan)
	}

	zx := testZX()
	if msg := zx.loadDroppedFile("/x/pair.zms"); !strings.Contains(msg, "only the window") {
		t.Errorf("with no window: %q", msg)
	}
	var opened []string
	zx.OpenMultisnapshot = func(path string) { opened = append(opened, path) }
	if msg := zx.loadDroppedFile("/x/pair.zms"); msg != "" || len(opened) != 1 || opened[0] != "/x/pair.zms" {
		t.Errorf("with a window: %q, opened %v", msg, opened)
	}
	// with no machine running a multisnapshot is the file that makes them, as a snapshot is
	zx.stopped = true
	if msg := zx.loadDroppedFile("/x/other.zms"); msg != "" || len(opened) != 2 {
		t.Errorf("with no machine running: %q, opened %v", msg, opened)
	}
	if msg := zx.loadDroppedFile("/x/x.tap"); !strings.HasPrefix(msg, "No machine is running") {
		t.Errorf("a tape with no machine: %q", msg)
	}
}

// Load Snapshot... and a row of Recent Snapshots that is a multisnapshot load it by its own way:
// asking first when machines are running.
func TestLoadSnapshotLoadsAMultisnapshot(t *testing.T) {
	inSnapshotDir(t)
	path := writeSampleZMS(t, "pair.zms")
	b, zx := newTestBar(t, nil, "")
	running := 2
	b.layoutState = func() (deckLayout, int) { return layoutSingle, running }
	var loaded []string
	b.loadMulti = func(p string, _ cardPlan) (int, error) { loaded = append(loaded, p); return 2, nil }

	b.openLoadSnapshotDialog()
	b.acceptDialog(zx, path) // what the browser hands back
	if b.replaceAsk == nil || len(loaded) != 0 {
		t.Fatalf("with two machines running: ask %v, loaded %v", b.replaceAsk, loaded)
	}
	b.replaceAsk.text = replaceWord
	b.answerReplace(zx, 0, true)
	if len(loaded) != 1 || loaded[0] != path || len(b.recentSnaps) != 1 {
		t.Fatalf("after YES: loaded %v, recent %v", loaded, b.recentSnaps)
	}
	// the row of Recent Snapshots
	running, loaded = 0, nil
	rows := snapshotRows(t, b, "Recent Snapshots")
	if len(rows) != 1 || rows[0] != "pair.zms" {
		t.Fatalf("recent: %v", rows)
	}
	b.refreshSnapshotItems()
	b.snapSpec.pick(zx, snapRow(t, b, "Recent Snapshots"), 0)
	if len(loaded) != 1 || loaded[0] != path {
		t.Errorf("the recent row loaded %v", loaded)
	}
}
