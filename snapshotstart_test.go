//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zentools/pkg/szx"
)

// A snapshot dropped on the machine waits paused unless Preferences say it plays, whichever
// of the formats it is in; the choice is read when the file is loaded, not when it is saved.
func TestDroppedSnapshotStartFollowsThePreference(t *testing.T) {
	defer func(f func() bool) { snapshotStartsPlaying = f }(snapshotStartsPlaying)

	src := NewZenZX(AudioBackendOto)
	setDistinctiveState(src)
	dir := t.TempDir()
	files := []string{}
	for _, f := range []string{"a.zxs", "b.sna", "c.z80"} {
		p := filepath.Join(dir, f)
		var err error
		switch filepath.Ext(f) {
		case ".zxs":
			err = src.SaveSnapshot(p)
		case ".sna":
			err = src.SaveSNA(p)
		default:
			err = src.SaveZ80(p)
		}
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		files = append(files, p)
	}

	for _, plays := range []bool{false, true} {
		snapshotStartsPlaying = func() bool { return plays }
		for _, p := range files {
			dst := NewZenZX(AudioBackendOto)
			dst.paused = !plays // the opposite of what is wanted, so a no-op shows
			if msg := dst.loadDroppedFile(p); msg == "" {
				t.Fatalf("%s was not recognised", p)
			}
			if dst.paused == plays {
				t.Errorf("%s: plays=%v but paused=%v after the load", filepath.Base(p), plays, dst.paused)
			}
		}
	}

	snapshotStartsPlaying = nil // headless and tests: a loaded snapshot waits
	dst := NewZenZX(AudioBackendOto)
	dst.paused = false
	dst.loadDroppedFile(files[0])
	if !dst.paused {
		t.Error("with no preference wired the snapshot did not wait")
	}
}

// The same for a .szx, which ZenZX loads but does not save: made here by encoding the machine's
// state with the format's own package.
func TestDroppedSZXStartFollowsThePreference(t *testing.T) {
	defer func(f func() bool) { snapshotStartsPlaying = f }(snapshotStartsPlaying)
	src := NewZenZX(AudioBackendOto)
	setDistinctiveState(src)
	image, err := szx.Encode(src.toMachineState())
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "s.szx")
	if err := os.WriteFile(p, image, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, plays := range []bool{false, true} {
		snapshotStartsPlaying = func() bool { return plays }
		dst := NewZenZX(AudioBackendOto)
		dst.paused = !plays
		if msg := dst.loadDroppedFile(p); msg == "" || dst.paused == plays {
			t.Errorf("plays=%v: said %q, paused=%v after the load", plays, msg, dst.paused)
		}
	}
}
