package zenzx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/plus3/pkg/diskimg"
)

// makeBlankDSK writes a blank, formatted +3 disk image to a temp file and returns
// its path, for the floppy-drop tests.
func makeBlankDSK(t *testing.T) string {
	t.Helper()
	di := diskimg.NewDiskImage()
	if err := di.InitializeDirectory(); err != nil {
		t.Fatalf("format blank disk: %v", err)
	}
	path := filepath.Join(t.TempDir(), "blank.dsk")
	if err := di.SaveToFile(path); err != nil {
		t.Fatalf("save blank disk: %v", err)
	}
	return path
}

// discHasFile reports whether the +3DOS directory of the disk at path holds a
// file whose 8.3 name matches want (e.g. "GALLOP.NEX").
func discHasFile(t *testing.T, path, want string) bool {
	t.Helper()
	di, err := diskimg.LoadFromFile(path)
	if err != nil {
		t.Fatalf("reload disk: %v", err)
	}
	dir, err := di.GetDirectory()
	if err != nil {
		t.Fatalf("read directory: %v", err)
	}
	wantName := diskName(want)
	for _, e := range dir {
		name := trimName(e.Name[:])
		ext := trimName(e.Extension[:])
		full := name
		if ext != "" {
			full = name + "." + ext
		}
		if full == wantName {
			return true
		}
	}
	return false
}

func trimName(b []byte) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 || c == ' ' {
			continue
		}
		out = append(out, c&0x7F) // +3DOS uses the high bit of name bytes as flags
	}
	return string(out)
}

// TestAddFilesToDiscLands checks that addFilesToDisk writes a dropped file into
// the +3DOS filesystem of a .dsk and that the FDC, reloaded from that image,
// serves the new disk -- the core of drop-into-disk.
func TestAddFilesToDiscLands(t *testing.T) {
	dsk := makeBlankDSK(t)
	host := filepath.Join(t.TempDir(), "gallop.nex")
	if err := os.WriteFile(host, []byte("NEX payload bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := addFilesToDisk(dsk, []string{host})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 || written[0] != "GALLOP.NEX" {
		t.Fatalf("written = %v, want [GALLOP.NEX]", written)
	}
	if !discHasFile(t, dsk, "gallop.nex") {
		t.Errorf("GALLOP.NEX is not in the disk directory after the add")
	}

	// The FDC, loaded from the written image, must accept it as a disk.
	fdc := NewFDC765()
	if err := fdc.LoadDisk(dsk); err != nil {
		t.Fatalf("the FDC could not load the written disk: %v", err)
	}
	if !fdc.HasDisk() {
		t.Errorf("the FDC reports no disk after loading the written image")
	}
}

// TestAddFilesToDiscTargetsOneDisc checks that adding to one disk leaves another
// untouched -- the floppy form of the focused-machine guarantee: two +3 machines
// have two disks, and a drop must land only on the targeted one.
func TestAddFilesToDiscTargetsOneDisc(t *testing.T) {
	discA := makeBlankDSK(t)
	discB := makeBlankDSK(t)
	host := filepath.Join(t.TempDir(), "orbit.nex")
	if err := os.WriteFile(host, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Add only to disk B (the "focused" machine's drive).
	if _, err := addFilesToDisk(discB, []string{host}); err != nil {
		t.Fatal(err)
	}
	if !discHasFile(t, discB, "orbit.nex") {
		t.Errorf("disk B did not get the file")
	}
	if discHasFile(t, discA, "orbit.nex") {
		t.Errorf("disk A got the file -- the write went to the wrong disk")
	}
}
