package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListCustomROMsEmpty(t *testing.T) {
	// A directory that doesn't exist at all.
	if got := listCustomROMs("/nonexistent/definitely/not/here"); got != nil {
		t.Errorf("listCustomROMs on a missing dir = %v, want nil", got)
	}

	// A directory that exists but has nothing in it.
	empty := t.TempDir()
	if got := listCustomROMs(empty); got != nil {
		t.Errorf("listCustomROMs on an empty dir = %v, want nil", got)
	}
}

func TestListCustomROMsFiltersAndSorts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zzz.rom", "aaa.rom", "readme.txt", "notes.md", "MIXED.ROM"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte{0}, 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	// A subdirectory ending in .rom should never be listed as a file.
	if err := os.Mkdir(filepath.Join(dir, "sub.rom"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got := listCustomROMs(dir)
	want := []string{"MIXED.ROM", "aaa.rom", "zzz.rom"} // sorted; case affects sort order
	if len(got) != len(want) {
		t.Fatalf("listCustomROMs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("listCustomROMs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReadInt(t *testing.T) {
	cases := []struct {
		input  string
		lo, hi int
		wantN  int
		wantOK bool
	}{
		{"3\n", 0, 5, 3, true},
		{"0\n", 0, 5, 0, true},
		{"5\n", 0, 5, 5, true},
		{"6\n", 0, 5, 0, false},  // out of range
		{"-1\n", 0, 5, 0, false}, // out of range
		{"abc\n", 0, 5, 0, false},
		{"\n", 0, 5, 0, false}, // blank line
		{"  2  \n", 0, 5, 2, true},
	}
	for _, c := range cases {
		r := bufio.NewReader(strings.NewReader(c.input))
		n, ok := readInt(r, c.lo, c.hi)
		if ok != c.wantOK || (ok && n != c.wantN) {
			t.Errorf("readInt(%q, %d, %d) = (%d, %v), want (%d, %v)",
				c.input, c.lo, c.hi, n, ok, c.wantN, c.wantOK)
		}
	}
}

func TestRunCustomROMMenuEmptyDir(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	empty := t.TempDir()
	in := bufio.NewReader(strings.NewReader(""))
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer out.Close()

	runCustomROMMenu(zx, empty, in, out)

	out.Seek(0, 0)
	buf := make([]byte, 1024)
	n, _ := out.Read(buf)
	msg := string(buf[:n])
	if !strings.Contains(msg, "No .rom files found") {
		t.Errorf("expected 'no roms found' message, got: %q", msg)
	}
}

func TestRunCustomROMMenuSkip(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	originalByte0 := zx.memory.rom[0][0]

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.rom"), make([]byte, 16384), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	in := bufio.NewReader(strings.NewReader("0\n")) // 0 = skip
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer out.Close()

	runCustomROMMenu(zx, dir, in, out)

	if zx.memory.rom[0][0] != originalByte0 {
		t.Error("choosing 'skip' (0) must not modify the ROM")
	}
}

func TestRunCustomROMMenuSelectsAndAppliesToBank(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	// +3-style model: 4 banks, so the menu should prompt for a bank too.
	if err := zx.LoadPlus3ROM("./rom/plus3-0.rom", "./rom/plus3-1.rom", "./rom/plus3-2.rom", "./rom/plus3-3.rom"); err != nil {
		t.Fatalf("LoadPlus3ROM: %v", err)
	}
	originalBank0Byte0 := zx.memory.rom[0][0]

	dir := t.TempDir()
	overrideData := make([]byte, 16384)
	overrideData[0] = 0xEE
	if err := os.WriteFile(filepath.Join(dir, "translated.rom"), overrideData, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Select ROM #1 (the only one), then bank 2.
	in := bufio.NewReader(strings.NewReader("1\n2\n"))
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer out.Close()

	runCustomROMMenu(zx, dir, in, out)

	if zx.memory.rom[2][0] != 0xEE {
		t.Errorf("bank 2 byte 0 = 0x%02X, want 0xEE (the selected ROM's content)", zx.memory.rom[2][0])
	}
	if zx.memory.rom[0][0] != originalBank0Byte0 {
		t.Error("bank 0 must be untouched -- selection was applied to bank 2, not bank 0")
	}
}

func TestRunCustomROMMenuInvalidSelectionKeepsStandardROM(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	originalByte0 := zx.memory.rom[0][0]

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.rom"), make([]byte, 16384), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	in := bufio.NewReader(strings.NewReader("99\n")) // out of range
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer out.Close()

	runCustomROMMenu(zx, dir, in, out)

	if zx.memory.rom[0][0] != originalByte0 {
		t.Error("an out-of-range selection must not modify the ROM")
	}
}
