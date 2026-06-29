// file: snapshot_sentinel_test.go
//
// Loads real third-party snapshot files (game .z80 across versions, z88dk .sna)
// through zenzx's zentools-backed load path, verifies the emulator reaches a
// sane state, and confirms a re-save round-trips. These exercise the actual
// migrated code against genuine artifacts, not synthetic data.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zentools/pkg/snapshot"
)

func countNonZero48K(zx *ZenZX) int {
	n := 0
	for _, b := range []int{5, 2, 0} {
		for _, v := range zx.memory.ram[b] {
			if v != 0 {
				n++
			}
		}
	}
	return n
}

func TestSentinelLoadZ80(t *testing.T) {
	cases := []struct {
		file    string
		wantPC  uint16
		is128K  bool
		minData int // a real loaded game has substantial populated memory
	}{
		{"jetsetwilly_v1_48k.z80", 0x96AC, false, 10000},
		{"manicminer_v2_48k.z80", 0x0038, false, 200},
		{"z80attack_v3_128k.z80", 0xBE2F, true, 0},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			zx := NewZenZX(AudioBackendOto)
			if c.is128K {
				zx.is128K = true
				zx.memory.Enable128K()
			}
			path := filepath.Join("testdata/snapshots", c.file)
			if err := zx.LoadZ80(path); err != nil {
				t.Fatalf("LoadZ80(%s): %v", c.file, err)
			}
			if zx.cpu.PC != c.wantPC {
				t.Errorf("PC = 0x%04X, want 0x%04X", zx.cpu.PC, c.wantPC)
			}
			if !c.is128K {
				if got := countNonZero48K(zx); got < c.minData {
					t.Errorf("only %d non-zero bytes in 48K RAM; load likely incomplete", got)
				}
			}
		})
	}
}

func TestSentinelLoadSNA(t *testing.T) {
	// 48K z88dk sna: PC is pushed to the stack; after load it must be recovered.
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadSNA("testdata/snapshots/z88dk_zx48.sna"); err != nil {
		t.Fatalf("LoadSNA 48K: %v", err)
	}
	// z88dk's canonical 48K snapshot starts at 0x8000 with I=0x3F, IM=1.
	if zx.cpu.PC != 0x8000 {
		t.Errorf("48K sna PC = 0x%04X, want 0x8000", zx.cpu.PC)
	}
	if zx.cpu.I != 0x3F || zx.cpu.IM != 1 {
		t.Errorf("48K sna I=0x%02X IM=%d, want 0x3F/1", zx.cpu.I, zx.cpu.IM)
	}

	// 128K z88dk sna.
	zx128 := NewZenZX(AudioBackendOto)
	zx128.is128K = true
	zx128.memory.Enable128K()
	if err := zx128.LoadSNA("testdata/snapshots/z88dk_zx128.sna"); err != nil {
		t.Fatalf("LoadSNA 128K: %v", err)
	}
	if zx128.cpu.PC != 0x8000 {
		t.Errorf("128K sna PC = 0x%04X, want 0x8000", zx128.cpu.PC)
	}
}

// TestSentinelLoadResaveZ80 loads a real game, re-saves it through the new v3
// SaveZ80, and confirms the output decodes back to the same PC and memory.
func TestSentinelLoadResaveZ80(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadZ80("testdata/snapshots/jetsetwilly_v1_48k.z80"); err != nil {
		t.Fatalf("LoadZ80: %v", err)
	}
	beforePC := zx.cpu.PC
	beforeData := countNonZero48K(zx)

	dir := t.TempDir()
	out := filepath.Join(dir, "resaved.z80")
	if err := zx.SaveZ80(out); err != nil {
		t.Fatalf("SaveZ80: %v", err)
	}

	// Decode the re-saved file independently with zentools and compare.
	image, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s, err := snapshot.DecodeZ80(image)
	if err != nil {
		t.Fatalf("re-saved .z80 does not decode: %v", err)
	}
	if s.CPU.PC != beforePC {
		t.Errorf("re-saved PC = 0x%04X, want 0x%04X", s.CPU.PC, beforePC)
	}
	// The re-saved memory must match what we loaded (banks 5,2,0).
	if s.Memory.RAM[5] != zx.memory.ram[5] ||
		s.Memory.RAM[2] != zx.memory.ram[2] ||
		s.Memory.RAM[0] != zx.memory.ram[0] {
		t.Error("re-saved 48K memory differs from loaded state")
	}
	_ = beforeData
}

// independentDecodeV1RLE is a clean-room v1-RLE decoder used to verify zenzx's
// saved output without relying on zentools' own decoder.
func independentDecodeV1RLE(data []byte) []byte {
	var out []byte
	for i := 0; i < len(data); {
		if i+3 < len(data) && data[i] == 0xED && data[i+1] == 0xED {
			cnt := int(data[i+2])
			val := data[i+3]
			for k := 0; k < cnt; k++ {
				out = append(out, val)
			}
			i += 4
		} else {
			out = append(out, data[i])
			i++
		}
	}
	return out
}

// TestSentinelResaveIndependentDecode is the strongest guard: load a real v1
// game, re-save as v3, then decode the v3 output with an independent decoder
// (not zentools) and confirm the memory is byte-identical to the original. This
// proves zenzx now writes files other tools can read faithfully.
func TestSentinelResaveIndependentDecode(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadZ80("testdata/snapshots/jetsetwilly_v1_48k.z80"); err != nil {
		t.Fatalf("LoadZ80: %v", err)
	}
	// Snapshot the loaded 48K memory (banks 5,2,0 = 0x4000/0x8000/0xC000).
	var loaded []byte
	loaded = append(loaded, zx.memory.ram[5][:]...)
	loaded = append(loaded, zx.memory.ram[2][:]...)
	loaded = append(loaded, zx.memory.ram[0][:]...)

	dir := t.TempDir()
	out := filepath.Join(dir, "resaved_v3.z80")
	if err := zx.SaveZ80(out); err != nil {
		t.Fatalf("SaveZ80: %v", err)
	}
	d, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	// Parse the v3 per-page blocks independently.
	extLen := int(d[30]) | int(d[31])<<8
	pos := 32 + extLen
	banks := map[byte][]byte{}
	for pos+3 <= len(d) {
		blen := int(d[pos]) | int(d[pos+1])<<8
		page := d[pos+2]
		pos += 3
		var raw []byte
		if blen == 0xFFFF {
			raw = d[pos : pos+16384]
			pos += 16384
		} else {
			raw = independentDecodeV1RLE(d[pos : pos+blen])
			pos += blen
		}
		banks[page] = raw
	}
	// 48K: page 8 -> 0x4000, page 4 -> 0x8000, page 5 -> 0xC000.
	var decoded []byte
	decoded = append(decoded, banks[8]...)
	decoded = append(decoded, banks[4]...)
	decoded = append(decoded, banks[5]...)

	if len(decoded) != len(loaded) {
		t.Fatalf("decoded %d bytes, loaded %d", len(decoded), len(loaded))
	}
	for i := range loaded {
		if loaded[i] != decoded[i] {
			t.Fatalf("byte %d differs: loaded 0x%02X, independently-decoded 0x%02X", i, loaded[i], decoded[i])
		}
	}
}
