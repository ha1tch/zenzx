package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The disk in the drive is a copy in memory of a file (fdc.go). These tests
// are about the two drifting apart.

// tempDSK copies the synthetic test disk to a new folder and returns its path.
func tempDSK(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/synthetic.dsk")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "disk.dsk")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// scribble changes the disk in memory, as a write from the machine does.
func scribble(t *testing.T, fdc *FDC765) {
	t.Helper()
	if fdc.disk == nil || len(fdc.disk.Tracks) == 0 || len(fdc.disk.Tracks[0].Sectors) == 0 {
		t.Fatal("the test disk has no first sector to write to")
	}
	fdc.disk.Tracks[0].Sectors[0].Data[0] ^= 0xFF
	fdc.markModified()
}

// touchOutside rewrites a file as another program would, with a later time.
func touchOutside(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
}

func TestBlankDiskReplacesADiskReadFromADSK(t *testing.T) {
	fdc := NewFDC765()
	if err := fdc.LoadDisk(tempDSK(t)); err != nil {
		t.Fatal(err)
	}
	if fdc.disk == nil {
		t.Fatal("the test disk was not read as a DSK")
	}
	fdc.CreateBlankDisk()
	if fdc.disk != nil {
		t.Error("after a blank disk is put in, reads would still come from the DSK that was there before")
	}
	if !fdc.HasDisk() || fdc.DiskFile() != "" || fdc.IsModified() {
		t.Errorf("a blank disk: present %v, file %q, modified %v", fdc.HasDisk(), fdc.DiskFile(), fdc.IsModified())
	}
}

func TestSaveDiskAsWritesTheDiskTheDriveReads(t *testing.T) {
	path := tempDSK(t)
	zx := NewZenZX(AudioBackendOto)
	if err := zx.io.LoadDisk(path); err != nil {
		t.Fatal(err)
	}
	fdc := zx.io.fdc
	scribble(t, fdc)
	want := fdc.disk.Serialize()

	out := filepath.Join(filepath.Dir(path), "saved.dsk")
	if err := zx.io.SaveDiskAs(out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Save As wrote %d bytes that are not the disk in the drive (%d bytes): a DSK is in the parsed disk, not the flat image", len(got), len(want))
	}
	if fdc.DiskFile() != out || fdc.IsModified() || fdc.FileChanged() {
		t.Errorf("after Save As: file %q, modified %v, changed %v", fdc.DiskFile(), fdc.IsModified(), fdc.FileChanged())
	}
	again := NewFDC765()
	if err := again.LoadDisk(out); err != nil {
		t.Errorf("the saved disk cannot be read back: %v", err)
	}
}

func TestFileChangedBehindTheDisk(t *testing.T) {
	path := tempDSK(t)
	fdc := NewFDC765()
	if err := fdc.LoadDisk(path); err != nil {
		t.Fatal(err)
	}
	if fdc.FileChanged() {
		t.Fatal("a disk just read is reported as changed on disk")
	}
	touchOutside(t, path)
	if !fdc.FileChanged() {
		t.Fatal("a file rewritten by something else is not noticed")
	}
	if err := fdc.LoadDisk(path); err != nil {
		t.Fatal(err)
	}
	if fdc.FileChanged() {
		t.Error("reading the file again did not settle it")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if !fdc.FileChanged() {
		t.Error("a removed file is not noticed")
	}
}

func TestAutoCommitDoesNotWriteOverAFileChangedOutside(t *testing.T) {
	path := tempDSK(t)
	fdc := NewFDC765()
	if err := fdc.LoadDisk(path); err != nil {
		t.Fatal(err)
	}

	// The ordinary case first: a quiet, modified disk is committed.
	fdc.Tick(0)
	scribble(t, fdc)
	fdc.Tick(100)
	if !fdc.AutoCommit(50) {
		t.Fatal("a modified disk that has been quiet was not committed")
	}
	if fdc.IsModified() || fdc.FileChanged() {
		t.Fatalf("after a commit: modified %v, changed %v", fdc.IsModified(), fdc.FileChanged())
	}

	// Now something else rewrites the file while the machine has changes.
	scribble(t, fdc)
	touchOutside(t, path)
	outside, _ := os.ReadFile(path)
	fdc.Tick(300)
	if fdc.AutoCommit(50) {
		t.Fatal("the auto-commit wrote over a file that had changed on disk")
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, outside) {
		t.Fatal("the file changed although the commit was refused")
	}
	if !fdc.IsModified() {
		t.Error("the refused commit dropped the disk's changes")
	}
	// Save is the person's decision, and does write.
	if err := fdc.SaveDisk(); err != nil {
		t.Fatal(err)
	}
	if fdc.IsModified() || fdc.FileChanged() {
		t.Errorf("after Save: modified %v, changed %v", fdc.IsModified(), fdc.FileChanged())
	}
}
