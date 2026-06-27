//go:build headless

package main

import (
	"os"
	"testing"
)

func TestParseAddr(t *testing.T) {
	cases := map[string]uint16{
		"0x8000": 0x8000, "0X8000": 0x8000, "$C000": 0xC000,
		"16384": 16384, "0xFFFF": 0xFFFF, "0": 0,
	}
	for in, want := range cases {
		got, err := ParseAddr(in)
		if err != nil {
			t.Errorf("ParseAddr(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseAddr(%q) = 0x%04X, want 0x%04X", in, got, want)
		}
	}
	if _, err := ParseAddr("0x10000"); err == nil {
		t.Error("ParseAddr(0x10000) should fail (exceeds 64K)")
	}
	if _, err := ParseAddr("notanumber"); err == nil {
		t.Error("ParseAddr(notanumber) should fail")
	}
}

func TestParseAddrSigned(t *testing.T) {
	if v, err := ParseAddrSigned("-1"); err != nil || v != -1 {
		t.Errorf("ParseAddrSigned(-1) = %d, %v; want -1, nil", v, err)
	}
	if v, err := ParseAddrSigned("0x8000"); err != nil || v != 0x8000 {
		t.Errorf("ParseAddrSigned(0x8000) = %d, %v; want 32768, nil", v, err)
	}
}

func TestMemoryLoad(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	zx.cpu.Reset()

	// Load a pattern into RAM at 0x8000 (48K bank 2) and read it back.
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	zx.memory.Load(0x8000, data)
	for i, b := range data {
		got := zx.memory.Read(0x8000 + uint16(i))
		if got != b {
			t.Errorf("Read(0x%04X) = 0x%02X, want 0x%02X", 0x8000+i, got, b)
		}
	}

	// Load into screen attribute area and confirm the screen mirror updated.
	zx.memory.Load(0x5800, []byte{0x42})
	if zx.screen.attributes[0] != 0x42 {
		t.Errorf("screen.attributes[0] = 0x%02X, want 0x42 (screen mirror)", zx.screen.attributes[0])
	}

	// Writes to ROM region must be ignored.
	romBefore := zx.memory.Read(0x0000)
	zx.memory.Load(0x0000, []byte{^romBefore})
	if zx.memory.Read(0x0000) != romBefore {
		t.Error("Load into ROM region should be ignored")
	}
}

func TestLoadBINErrors(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadBIN("/nonexistent/file.bin", 0x8000, -1); err == nil {
		t.Error("LoadBIN of missing file should error")
	}
}

func TestLoadSCR(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	zx.cpu.Reset()

	// 6912-byte screen: bitmap 0xAA, attributes 0x07.
	scr := make([]byte, 6912)
	for i := 0; i < 6144; i++ {
		scr[i] = 0xAA
	}
	for i := 6144; i < 6912; i++ {
		scr[i] = 0x07
	}
	dir := t.TempDir()
	path := dir + "/test.scr"
	if err := os.WriteFile(path, scr, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := zx.LoadSCR(path); err != nil {
		t.Fatalf("LoadSCR: %v", err)
	}
	// Bitmap and attributes must be mirrored onto the screen buffers.
	if zx.screen.bitmap[0] != 0xAA {
		t.Errorf("screen.bitmap[0] = 0x%02X, want 0xAA", zx.screen.bitmap[0])
	}
	if zx.screen.attributes[0] != 0x07 {
		t.Errorf("screen.attributes[0] = 0x%02X, want 0x07", zx.screen.attributes[0])
	}
	// And readable back through memory at 0x4000 / 0x5800.
	if zx.memory.Read(0x4000) != 0xAA {
		t.Errorf("Read(0x4000) = 0x%02X, want 0xAA", zx.memory.Read(0x4000))
	}
	if zx.memory.Read(0x5800) != 0x07 {
		t.Errorf("Read(0x5800) = 0x%02X, want 0x07", zx.memory.Read(0x5800))
	}

	if err := zx.LoadSCR("/nonexistent/x.scr"); err == nil {
		t.Error("LoadSCR of missing file should error")
	}
}
