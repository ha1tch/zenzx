//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// Snapshot > Save As has a submenu of the formats, each opening the file browser for a name in
// that format; the browser's answer saves the machine in it and the host is told.
func TestSnapshotSaveAsMenu(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barSnapshot)
	const saveAs = 5 // after the title of the section, Quick Save, Quick Load, Save Timestamped and Load Timestamped
	if items[saveAs].Label != "Save As" || len(items[saveAs].SubItems) != len(snapshotSaveFormats) {
		t.Fatalf("items[%d] = %q with %d choices, want Save As with %d formats", saveAs, items[saveAs].Label, len(items[saveAs].SubItems), len(snapshotSaveFormats))
	}
	for i, f := range snapshotSaveFormats {
		if got := items[saveAs].SubItems[i].Label; got != f.label {
			t.Errorf("choice %d is %q, want %q", i, got, f.label)
		}
	}

	dir := t.TempDir()
	for i, f := range snapshotSaveFormats {
		b.diskDialog, b.dialogKind = nil, dialogDisk
		b.snapSpec.pick(zx, saveAs, i)
		if b.diskDialog == nil || b.dialogKind != dialogSaveSnapshot || b.saveFormat != f.format {
			t.Fatalf("%s: the browser is %v, for %v, in format %q", f.format, b.diskDialog != nil, b.dialogKind, b.saveFormat)
		}
		var said []string
		zx.OnNotice = func(msg string) { said = append(said, msg) }
		b.acceptDialog(zx, filepath.Join(dir, "game"))
		zx.OnNotice = nil
		if _, err := os.Stat(filepath.Join(dir, "game."+f.format)); err != nil {
			t.Errorf("%s: the snapshot was not saved: %v", f.format, err)
		}
		if len(said) != 1 || !strings.Contains(said[0], "game."+f.format) {
			t.Errorf("%s: the host was told %q", f.format, said)
		}
		if b.diskDialog != nil || b.dialogKind != dialogDisk || b.saveFormat != "" {
			t.Errorf("%s: the browser's state was not cleared", f.format)
		}
	}
}

// The Snapshot menu's other items, and F9 to F11, say what they did to the host too.
func TestSnapshotMenuActionsSpeakToTheHost(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	got := notices(zx, func() { zx.handleQuickSave() })
	if len(got) != 1 || !strings.Contains(got[0], "Quick saved to "+filepath.Join(SnapshotDir, "quicksave.zxs")) {
		t.Errorf("Quick Save said %q", got)
	}
	got = notices(zx, func() { zx.handleAutoSave() })
	if len(got) != 1 || !strings.Contains(got[0], "Saved snapshot: "+SnapshotDir+"/snapshot_") {
		t.Errorf("Save Timestamped said %q", got)
	}
	got = notices(zx, func() { zx.handleQuickLoad() })
	if len(got) != 1 || !strings.Contains(got[0], "Quick loaded from "+filepath.Join(SnapshotDir, "quicksave.zxs")) {
		t.Errorf("Quick Load said %q", got)
	}
	os.Remove(filepath.Join(SnapshotDir, "quicksave.zxs"))
	got = notices(zx, func() { zx.handleQuickLoad() })
	if len(got) != 1 || !strings.Contains(got[0], "No quick save found") {
		t.Errorf("Quick Load with nothing saved said %q", got)
	}
}
