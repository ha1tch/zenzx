// file: snapshot_roundtrip_test.go
//
// Regression gate for the zentools/pkg/snapshot migration: a full
// state -> save -> load -> state cycle must preserve registers (including PC,
// which the previous in-tree 48K SaveSNA silently dropped) and memory.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// setDistinctiveState fills CPU registers, border, and a few memory locations
// with recognisable values so a faithful round trip is easy to verify.
func setDistinctiveState(zx *ZenZX) {
	zx.cpu.SetAF(0x1234)
	zx.cpu.SetBC(0x5678)
	zx.cpu.SetDE(0x9ABC)
	zx.cpu.SetHL(0xDEF0)
	zx.cpu.SetIX(0xAAAA)
	zx.cpu.SetIY(0xBBBB)
	zx.cpu.SP = 0xFF00
	zx.cpu.PC = 0x8000 // the value the old 48K SaveSNA lost
	zx.cpu.I = 0x3F
	zx.cpu.R = 0x7E
	zx.cpu.IM = 1
	zx.cpu.IFF1 = true
	zx.cpu.IFF2 = true
	zx.io.borderColor = 5

	// Memory markers across the three 48K banks (5 @ 0x4000, 2 @ 0x8000, 0 @ 0xC000).
	zx.memory.ram[5][0] = 0x11
	zx.memory.ram[5][6143] = 0x22
	zx.memory.ram[2][100] = 0x33
	zx.memory.ram[0][16383] = 0x44
}

func assertStateRestored(t *testing.T, zx *ZenZX, label string) {
	t.Helper()
	checks := []struct {
		name string
		got  uint16
		want uint16
	}{
		{"AF", zx.cpu.AF(), 0x1234},
		{"BC", zx.cpu.BC(), 0x5678},
		{"DE", zx.cpu.DE(), 0x9ABC},
		{"HL", zx.cpu.HL(), 0xDEF0},
		{"IX", zx.cpu.IX(), 0xAAAA},
		{"IY", zx.cpu.IY(), 0xBBBB},
		{"SP", zx.cpu.SP, 0xFF00},
		{"PC", zx.cpu.PC, 0x8000}, // the bug-fix assertion
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: %s = 0x%04X, want 0x%04X", label, c.name, c.got, c.want)
		}
	}
	if zx.cpu.I != 0x3F || zx.cpu.IM != 1 || !zx.cpu.IFF1 || !zx.cpu.IFF2 {
		t.Errorf("%s: I=0x%02X IM=%d IFF1=%v IFF2=%v", label, zx.cpu.I, zx.cpu.IM, zx.cpu.IFF1, zx.cpu.IFF2)
	}
	if zx.io.borderColor != 5 {
		t.Errorf("%s: border = %d, want 5", label, zx.io.borderColor)
	}
	if zx.memory.ram[5][0] != 0x11 || zx.memory.ram[5][6143] != 0x22 ||
		zx.memory.ram[2][100] != 0x33 || zx.memory.ram[0][16383] != 0x44 {
		t.Errorf("%s: memory markers not preserved", label)
	}
}

func TestSNARoundTrip48K(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sna")

	src := NewZenZX(AudioBackendOto)
	setDistinctiveState(src)
	if err := src.SaveSNA(path); err != nil {
		t.Fatalf("SaveSNA: %v", err)
	}

	dst := NewZenZX(AudioBackendOto)
	if err := dst.LoadSNA(path); err != nil {
		t.Fatalf("LoadSNA: %v", err)
	}
	assertStateRestored(t, dst, "sna")
}

func TestZ80RoundTrip48K(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.z80")

	src := NewZenZX(AudioBackendOto)
	setDistinctiveState(src)
	if err := src.SaveZ80(path); err != nil {
		t.Fatalf("SaveZ80: %v", err)
	}

	dst := NewZenZX(AudioBackendOto)
	if err := dst.LoadZ80(path); err != nil {
		t.Fatalf("LoadZ80: %v", err)
	}
	assertStateRestored(t, dst, "z80")
}

// TestSavedSNAHasPC guards specifically against the old bug: the 48K .sna must
// carry PC (pushed to the stack), so a fresh decode recovers 0x8000, not 0.
func TestSavedSNAHasPC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pc.sna")
	src := NewZenZX(AudioBackendOto)
	setDistinctiveState(src)
	if err := src.SaveSNA(path); err != nil {
		t.Fatalf("SaveSNA: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not written: %v", err)
	}
	dst := NewZenZX(AudioBackendOto)
	if err := dst.LoadSNA(path); err != nil {
		t.Fatalf("LoadSNA: %v", err)
	}
	if dst.cpu.PC != 0x8000 {
		t.Errorf("PC = 0x%04X after round trip, want 0x8000 (the old SaveSNA bug)", dst.cpu.PC)
	}
}

// TestSNA128RoundTrip exercises the 128K path: all eight RAM banks plus the
// paging port must survive save/load.
func TestSNA128RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test128.sna")

	src := NewZenZX(AudioBackendOto)
	src.is128K = true
	src.memory.Enable128K()
	setDistinctiveState(src)
	src.cpu.PC = 0xC000
	src.memory.port7FFD = 0x03 // page bank 3 at 0xC000
	// Mark every bank so the full 128K set is checked.
	for b := 0; b < 8; b++ {
		src.memory.ram[b][b*10] = byte(0xA0 + b)
	}

	if err := src.SaveSNA(path); err != nil {
		t.Fatalf("SaveSNA 128K: %v", err)
	}

	dst := NewZenZX(AudioBackendOto)
	dst.is128K = true
	dst.memory.Enable128K()
	if err := dst.LoadSNA(path); err != nil {
		t.Fatalf("LoadSNA 128K: %v", err)
	}

	if dst.cpu.PC != 0xC000 {
		t.Errorf("PC = 0x%04X, want 0xC000", dst.cpu.PC)
	}
	if dst.memory.port7FFD != 0x03 {
		t.Errorf("port7FFD = 0x%02X, want 0x03", dst.memory.port7FFD)
	}
	for b := 0; b < 8; b++ {
		if dst.memory.ram[b][b*10] != byte(0xA0+b) {
			t.Errorf("bank %d marker = 0x%02X, want 0x%02X", b, dst.memory.ram[b][b*10], 0xA0+b)
		}
	}
}
