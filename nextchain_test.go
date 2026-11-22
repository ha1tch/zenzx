package zenzx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// Finding, attaching and detaching the Next's boot chain.

func writeNextFiles(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, size := range map[string]int{nextcard.BootROMName: 8192, nextcard.DivMMCROMName: 8192, nextcard.CardName: 4096} {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWantNextBootChain(t *testing.T) {
	for _, c := range []struct {
		name     string
		model    string
		classic  bool
		explicit []string
		want     bool
	}{
		{"a Next on its own", "next", false, []string{"", "", ""}, true},
		{"another model", "plus3", false, nil, false},
		{"-next-classic", "next", true, nil, false},
		{"the chain given file by file", "next", false, []string{"boot.rom", "", ""}, false},
		{"a snapshot to load", "next", false, []string{"", "", "", "game.sna"}, false},
	} {
		if got := wantNextBootChain(c.model, c.classic, c.explicit...); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLocateNextFilesOrderAndCompleteness(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base) // ./thirdparty is looked for here
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "home", "config"))
	t.Setenv(nextFilesEnv, "")

	// The files written here are placeholders; what makes a card usable is tested in
	// pkg/nextcard. A folder named "bad" stands for one whose files are not usable.
	saved := nextFilesUsable
	nextFilesUsable = func(dir string) error {
		if filepath.Base(dir) == "bad" {
			return errors.New("next-card.img is not a FAT32 volume")
		}
		return nil
	}
	defer func() { nextFilesUsable = saved }()

	if _, ok, note := locateNextFiles(""); ok || note != "" {
		t.Fatalf("boot files were found where there are none (note %q)", note)
	}
	// An incomplete folder does not count.
	flagDir, envDir := filepath.Join(base, "flag"), filepath.Join(base, "env")
	writeNextFiles(t, envDir)
	os.Remove(filepath.Join(envDir, nextcard.CardName))
	t.Setenv(nextFilesEnv, envDir)
	if _, ok, note := locateNextFiles(""); ok || note != "" {
		t.Fatalf("a folder without the card was accepted (note %q)", note)
	}
	// A folder that holds all three but cannot be used is passed over, and the reason
	// comes back for the window to show.
	bad := filepath.Join(base, "bad")
	writeNextFiles(t, bad)
	if _, ok, note := locateNextFiles(bad); ok || !strings.Contains(note, "not a FAT32 volume") || !strings.Contains(note, bad) {
		t.Fatalf("unusable folder: ok=%v note=%q", ok, note)
	}
	// The per-user folder is the last resort and the place downloads go to.
	userDir, err := nextcard.DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	writeNextFiles(t, userDir)
	if f, ok, _ := locateNextFiles(""); !ok || f.Dir != userDir {
		t.Fatalf("found %q (%v), want the per-user folder %q", f.Dir, ok, userDir)
	}
	// With usable files further down the list, an unusable folder earlier in it does not
	// stand in the way: no window is needed.
	if f, ok, note := locateNextFiles(bad); !ok || f.Dir != userDir || note != "" {
		t.Fatalf("with an unusable -next-files folder: found %q (%v), note %q", f.Dir, ok, note)
	}
	// ./thirdparty comes before it, ./next before that, NEXT_FILES before that, -next-files first.
	writeNextFiles(t, "thirdparty")
	if f, _, _ := locateNextFiles(""); f.Dir != "thirdparty" {
		t.Fatalf("found %q, want ./thirdparty", f.Dir)
	}
	writeNextFiles(t, "next")
	if f, _, _ := locateNextFiles(""); f.Dir != "next" {
		t.Fatalf("found %q, want ./next", f.Dir)
	}
	writeNextFiles(t, envDir)
	if f, _, _ := locateNextFiles(""); f.Dir != envDir {
		t.Fatalf("found %q, want the NEXT_FILES folder", f.Dir)
	}
	writeNextFiles(t, flagDir)
	if f, _, _ := locateNextFiles(flagDir); f.Dir != flagDir {
		t.Fatalf("found %q, want the -next-files folder", f.Dir)
	}

	if d, _ := nextDownloadDir(""); d != envDir {
		t.Fatalf("downloads would go to %q, want the NEXT_FILES folder", d)
	}
	t.Setenv(nextFilesEnv, "")
	if d, _ := nextDownloadDir(""); d != "next" {
		t.Fatalf("downloads would go to %q, want ./next, which is there", d)
	}
	os.RemoveAll("next")
	if d, _ := nextDownloadDir(""); d != userDir {
		t.Fatalf("downloads would go to %q, want the per-user folder", d)
	}
}

func TestAttachAndDetachNextBootChain(t *testing.T) {
	dir := t.TempDir()
	writeNextFiles(t, dir)
	zx := NewZenZX(AudioBackendOto)
	zx.memory.Enable128K()
	zx.memory.EnablePlus3()
	zx.memory.EnableNext()

	protected, err := zx.attachNextBootChain(nextcard.In(dir), false, false)
	if err != nil || protected {
		t.Fatalf("attach: protected=%v err=%v", protected, err)
	}
	if !zx.memory.bootROMEnabled || !zx.io.hasDivMMC || zx.io.divMMC == nil || zx.next == nil {
		t.Fatal("after attaching, the boot ROM, the DivMMC or the record of the chain is missing")
	}
	if !zx.memory.nextCfg.active {
		t.Fatal("the machine is not in config mode with the boot ROM mapped")
	}
	zx.detachNextBootChain()
	if zx.memory.bootROMEnabled || zx.io.hasDivMMC || zx.io.divMMC != nil || zx.next != nil || zx.cpu.M1Hook != nil {
		t.Fatal("after detaching, part of the boot chain is still on the machine")
	}
	if zx.memory.nextCfg.active {
		t.Fatal("config mode survived the detach")
	}
	zx.detachNextBootChain() // a second time does nothing

	// A card that cannot be written ends up write-protected, and -sdcard-readonly asks for it.
	if protected, err = zx.attachNextBootChain(nextcard.In(dir), true, false); err != nil || !protected {
		t.Fatalf("read-only attach: protected=%v err=%v", protected, err)
	}
	zx.detachNextBootChain()

	os.Remove(filepath.Join(dir, nextcard.BootROMName))
	if _, err := zx.attachNextBootChain(nextcard.In(dir), false, false); err == nil {
		t.Fatal("attaching without the boot ROM gave no error")
	}
	if zx.next != nil || zx.memory.bootROMEnabled {
		t.Fatal("a failed attach left part of a chain behind")
	}
}
