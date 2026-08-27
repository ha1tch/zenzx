package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempROM creates a temp file of exactly n zero bytes and returns
// its path -- these tests only need correctly-sized dummy data, not
// real ROM content, since SetROMBank/OverrideROMBank validate size and
// placement, not what's inside the bytes.
func writeTempROM(t *testing.T, n int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.rom")
	if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
		t.Fatalf("writeTempROM: %v", err)
	}
	return p
}

func TestSetROMBankValidation(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	m := NewSpectrumMemory(screen)

	if err := m.SetROMBank(-1, make([]byte, 16384)); err == nil {
		t.Error("bank -1 should be rejected")
	}
	if err := m.SetROMBank(4, make([]byte, 16384)); err == nil {
		t.Error("bank 4 should be rejected (only 0-3 exist)")
	}
	if err := m.SetROMBank(0, make([]byte, 8192)); err == nil {
		t.Error("wrong-sized data (8192, want 16384) should be rejected")
	}
	if err := m.SetROMBank(2, make([]byte, 16384)); err != nil {
		t.Errorf("valid bank+size should succeed, got: %v", err)
	}
}

func TestSetROMBankWritesInPlace(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	m := NewSpectrumMemory(screen)

	// Seed all four banks with a distinct marker byte each, confirming
	// SetROMBank only touches the one bank requested.
	for i := 0; i < 4; i++ {
		data := make([]byte, 16384)
		data[0] = byte(0xA0 + i)
		m.rom[i][0] = byte(0xA0 + i) // seed directly, bypassing SetROMBank
		_ = data
	}

	override := make([]byte, 16384)
	override[0] = 0xFF
	if err := m.SetROMBank(2, override); err != nil {
		t.Fatalf("SetROMBank: %v", err)
	}

	if m.rom[2][0] != 0xFF {
		t.Errorf("bank 2 = 0x%02X, want 0xFF (the override)", m.rom[2][0])
	}
	for i, want := range []byte{0xA0, 0xA1, 0xA3} {
		bank := []int{0, 1, 3}[i]
		if m.rom[bank][0] != want {
			t.Errorf("bank %d = 0x%02X, want unchanged 0x%02X", bank, m.rom[bank][0], want)
		}
	}
}

func TestMaxROMBankPerModel(t *testing.T) {
	cases := []struct {
		name                      string
		is128K, isPlus3, isTS2068 bool
		want                      int
	}{
		{"48K", false, false, false, 0},
		{"128K/+2", true, false, false, 1},
		{"+2A/+3", true, true, false, 3},
		{"TS2068", false, false, true, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			zx := NewZenZX(AudioBackendOto)
			zx.memory.is128K = c.is128K
			zx.memory.isPlus3 = c.isPlus3
			zx.memory.isTS2068 = c.isTS2068
			if got := zx.maxROMBank(); got != c.want {
				t.Errorf("maxROMBank() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestOverrideROMBankOnPlus3(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadPlus3ROM("./rom/plus3-0.rom", "./rom/plus3-1.rom", "./rom/plus3-2.rom", "./rom/plus3-3.rom"); err != nil {
		t.Fatalf("LoadPlus3ROM: %v", err)
	}

	// Confirm bank 0's original byte before touching anything, so the
	// "everything else stays intact" assertion below is meaningful, not
	// just checking two zeroed banks against each other.
	originalBank0Byte0 := zx.memory.rom[0][0]

	overridePath := writeTempROM(t, 16384)
	if err := zx.OverrideROMBank(3, overridePath); err != nil {
		t.Fatalf("OverrideROMBank(3): %v", err)
	}

	// Bank 3 should now be all zero bytes (the override content).
	allZero := true
	for _, b := range zx.memory.rom[3] {
		if b != 0 {
			allZero = false
			break
		}
	}
	if !allZero {
		t.Error("bank 3 was not overwritten with the override content")
	}

	// Bank 0 (the real +3DOS-adjacent boot ROM) must be untouched.
	if zx.memory.rom[0][0] != originalBank0Byte0 {
		t.Errorf("bank 0 byte 0 changed from 0x%02X to 0x%02X -- override leaked into another bank",
			originalBank0Byte0, zx.memory.rom[0][0])
	}
}

func TestOverrideROMBankRejectsOutOfRange(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}

	p := writeTempROM(t, 16384)
	if err := zx.OverrideROMBank(1, p); err == nil {
		t.Error("bank 1 on a 48K model (only bank 0 exists) should be rejected")
	}
}

func TestOverrideROMBankOnTS2068(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadTS2068ROM("./rom/ts2068-0.rom", "./rom/ts2068-1.rom"); err != nil {
		t.Fatalf("LoadTS2068ROM: %v", err)
	}

	// Bank 0 (Home) is 16K, bank 1 (Extension) is 8K -- TS2068's own
	// special case in OverrideROMBank, distinct from every other model's
	// uniform 16K banks (memory.rom[4][16384]byte can't hold an 8K
	// Extension ROM directly, hence the separate ts2068ExtROM field).
	home := writeTempROM(t, 16384)
	if err := zx.OverrideROMBank(0, home); err != nil {
		t.Errorf("OverrideROMBank(0, 16K) on TS2068 should succeed: %v", err)
	}

	ext := writeTempROM(t, 8192)
	if err := zx.OverrideROMBank(1, ext); err != nil {
		t.Errorf("OverrideROMBank(1, 8K) on TS2068 should succeed: %v", err)
	}

	// Wrong size for either bank must be rejected, not silently truncated
	// or silently accepted into the wrong-sized slot.
	wrongSizeForHome := writeTempROM(t, 8192)
	if err := zx.OverrideROMBank(0, wrongSizeForHome); err == nil {
		t.Error("8K data for TS2068 bank 0 (Home, needs 16K) should be rejected")
	}
	wrongSizeForExt := writeTempROM(t, 16384)
	if err := zx.OverrideROMBank(1, wrongSizeForExt); err == nil {
		t.Error("16K data for TS2068 bank 1 (Extension, needs 8K) should be rejected")
	}

	// Bank 2 doesn't exist on TS2068 (maxROMBank == 1).
	p := writeTempROM(t, 16384)
	if err := zx.OverrideROMBank(2, p); err == nil {
		t.Error("bank 2 on TS2068 (only banks 0-1 exist) should be rejected")
	}
}

func TestOverrideROMBankMissingFile(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	if err := zx.OverrideROMBank(0, "/nonexistent/path/does-not-exist.rom"); err == nil {
		t.Error("nonexistent file should be rejected, not silently ignored")
	}
}
