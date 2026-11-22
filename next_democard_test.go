package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextExamplesCardStandalone checks that runNextExamplesCard writes a
// self-contained demo card holding the three example programs, needing no boot
// files -- the card the release ships beside the program. It is what the CI
// release's package step runs to produce next/zenzx-card.img.
func TestNextExamplesCardStandalone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zenzx-card.img")
	var out bytes.Buffer
	if err := runNextExamplesCard(path, &out); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	vol, err := nextcard.Open(f)
	if err != nil {
		t.Fatalf("the demo card is not a card the firmware would read: %v", err)
	}
	for _, name := range []string{"gallop.nex", "fuji.nex", "orbit.nex"} {
		if n := vol.Root.Find(name); n == nil || n.IsDir() {
			t.Errorf("%s is not on the demo card", name)
		}
	}

	// An empty path is refused, not silently a no-op.
	if err := runNextExamplesCard("", &out); err == nil {
		t.Errorf("an empty path was accepted")
	}
}

// TestNextDownloadDirPrefersBesideExe checks that, with a `next` folder beside
// the program, the download goes there (beside the shipped demo card) rather
// than the per-user folder -- so the downloaded boot files sit next to the card
// and the card auto-inserts. The flag and the environment variable still win.
func TestNextDownloadDirPrefersBesideExe(t *testing.T) {
	// A `next` folder beside the test binary (os.Executable() is the test binary).
	exe, err := os.Executable()
	if err != nil {
		t.Skip("no executable path")
	}
	beside := filepath.Join(filepath.Dir(exe), nextFolder)
	if err := os.MkdirAll(beside, 0o755); err != nil {
		t.Skip("cannot make a next folder beside the test binary")
	}
	t.Cleanup(func() { os.RemoveAll(beside) })

	// Run from a clean cwd (no `next` folder there) so the beside-the-program
	// folder is what decides, and clear the environment variable.
	t.Chdir(t.TempDir())
	t.Setenv(nextFilesEnv, "")

	got, err := nextDownloadDir("")
	if err != nil {
		t.Fatal(err)
	}
	if got != beside {
		t.Errorf("download dir = %q, want the next folder beside the program %q", got, beside)
	}

	// The flag still wins over the beside-the-program folder.
	flagDir := t.TempDir()
	if got, _ := nextDownloadDir(flagDir); got != flagDir {
		t.Errorf("with a flag, download dir = %q, want %q", got, flagDir)
	}
}
