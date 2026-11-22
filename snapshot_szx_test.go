// file: snapshot_szx_test.go
//
// .szx snapshots: detection by extension and by magic, loading through
// zentools/pkg/szx, and the refusal to save one.

package zenzx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zentools/pkg/build"
)

func TestDetectSnapshotFormatKnowsSZXByExtensionAndByMagic(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		name    string
		content []byte
	}{
		{"game.szx", []byte("the extension decides")},
		{"GAME.SZX", []byte("the extension decides")},
		{"game.bin", append([]byte("ZXST"), make([]byte, 16)...)}, // no extension to go by: the magic
	} {
		path := filepath.Join(dir, c.name)
		if err := os.WriteFile(path, c.content, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := DetectSnapshotFormat(path); got != "szx" {
			t.Errorf("%s: detected %q, want szx", c.name, got)
		}
	}
}

func TestLoadSZXBringsInTheCodeAndTheRegisters(t *testing.T) {
	code := []byte{0x3E, 0x5A, 0x32, 0x00, 0x81, 0x18, 0xFE} // LD A,0x5A; LD (0x8100),A; JR $
	image, err := build.EncodeSZX(build.Request{Name: "szxtest", Code: code, Origin: 0x8000, Start: 0x8000, SP: build.DefaultSP, Model: build.Model48K})
	if err != nil {
		t.Fatalf("EncodeSZX: %v", err)
	}
	path := filepath.Join(t.TempDir(), "game.szx")
	if err := os.WriteFile(path, image, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"", "szx"} { // found by detection, and named
		zx := NewZenZX(AudioBackendOto)
		if err := zx.LoadSnapshotFormat(path, format); err != nil {
			t.Fatalf("format %q: %v", format, err)
		}
		if zx.cpu.PC != 0x8000 {
			t.Errorf("format %q: PC = 0x%04X, want 0x8000", format, zx.cpu.PC)
		}
		if zx.cpu.IM != 1 {
			t.Errorf("format %q: IM = %d, want 1", format, zx.cpu.IM)
		}
		for i, b := range code { // bank 2 is 0x8000 to 0xBFFF on a 48K machine
			if zx.memory.ram[2][i] != b {
				t.Errorf("format %q: the code is not in RAM at 0x%04X (got 0x%02X, want 0x%02X)", format, 0x8000+i, zx.memory.ram[2][i], b)
				break
			}
		}
	}
}

func TestSaveSnapshotFormatRefusesSZXAndDoesNotLoopForEver(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	path := filepath.Join(t.TempDir(), "out.szx")
	for _, format := range []string{"", "szx"} {
		if err := zx.SaveSnapshotFormat(path, format); err == nil {
			t.Errorf("format %q: saving a .szx returned no error", format)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("a file was written for a format that cannot be saved")
	}
}
