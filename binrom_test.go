package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func binFileOf(t *testing.T, data ...byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.bin")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// -bin without a page, and ZenScript's bin, wrote a file that was to land in the ROM and dropped
// every byte of it without a word. It is refused now, with where, and nothing is written; and so
// is a run that begins in RAM and ends in the ROM's place, or past the top.
func TestLoadBINRefusesWhatWouldLandInROM(t *testing.T) {
	zx := machineOf(t, "48k")
	rom := zx.memory.Read(0x0100)
	p := binFileOf(t, 1, 2, 3, 4)
	for _, c := range []struct {
		addr uint16
		want string
	}{{0x0000, "0x0000 is ROM"}, {0x0100, "0x0100 is ROM"}, {0x3FFE, "0x3FFE is ROM"}} {
		pc := zx.cpu.PC
		err := zx.LoadBIN(p, c.addr, 0x9000)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("LoadBIN at 0x%04X: %v, want %q", c.addr, err, c.want)
		}
		if zx.cpu.PC != pc {
			t.Errorf("LoadBIN at 0x%04X refused and still moved PC to 0x%04X", c.addr, zx.cpu.PC)
		}
	}
	if got := zx.memory.Read(0x0100); got != rom {
		t.Errorf("the ROM changed: 0x%02X, was 0x%02X", got, rom)
	}
	// RAM is as before, up to the last byte
	if err := zx.LoadBIN(p, 0x4000, -1); err != nil {
		t.Errorf("LoadBIN at 0x4000: %v", err)
	}
	if err := zx.LoadBIN(p, 0xFFFC, -1); err != nil {
		t.Errorf("LoadBIN to the last byte: %v", err)
	}
	if err := zx.LoadBIN(p, 0xFFFD, -1); err == nil || !strings.Contains(err.Error(), "64K") {
		t.Errorf("LoadBIN past the top: %v", err)
	}
	if zx.memory.Read(0xFFFF) != 4 {
		t.Errorf("the last byte is 0x%02X, want 4", zx.memory.Read(0xFFFF))
	}
}

// bin of ZenScript has a fourth word, a RAM page, as -binpage has.
func TestScriptBinNamesAPage(t *testing.T) {
	zx := machineOf(t, "128k")
	s := NewScheduler(&Script{}, ScheduleConfig{})
	p := binFileOf(t, 0xAA, 0xBB, 0xCC)
	pc := zx.cpu.PC
	if err := s.doBin(zx, Action{Verb: "bin", Args: []string{p, "0x0010", "-1", "6"}}); err != nil {
		t.Fatalf("bin into bank 6: %v", err)
	}
	if got, err := zx.memory.PeekPage(6, 0x0010, 3); err != nil || len(got) != 3 || got[0] != 0xAA || got[2] != 0xCC {
		t.Errorf("bank 6 holds %X, want AABBCC at 0x10", got)
	}
	if zx.cpu.PC != pc {
		t.Errorf("a page with a start of -1 moved PC to 0x%04X", zx.cpu.PC)
	}
	if err := s.doBin(zx, Action{Verb: "bin", Args: []string{p, "0x0010", "0x9000", "6"}}); err != nil || zx.cpu.PC != 0x9000 {
		t.Errorf("bin with a start and a page: %v, PC=0x%04X", err, zx.cpu.PC)
	}
	for _, bad := range [][]string{{p, "0", "-1", "9"}, {p, "0", "-1", "x"}, {p, "0x3FFF", "-1", "7"}} {
		if err := s.doBin(zx, Action{Verb: "bin", Args: bad}); err == nil {
			t.Errorf("bin %v was accepted", bad[1:])
		}
	}
	// a page "cpu" is the memory as the CPU sees it, as with no page; the ROM is refused there
	if err := s.doBin(zx, Action{Verb: "bin", Args: []string{p, "0x8000", "-1", "cpu"}}); err != nil {
		t.Errorf("bin with page cpu: %v", err)
	}
	if err := s.doBin(zx, Action{Verb: "bin", Args: []string{p, "0x0000", "-1"}}); err == nil || !strings.Contains(err.Error(), "ROM") {
		t.Errorf("bin into the ROM: %v", err)
	}
	// a machine without pages refuses one
	zx48 := machineOf(t, "48k")
	if err := s.doBin(zx48, Action{Verb: "bin", Args: []string{p, "0", "-1", "1"}}); err == nil {
		t.Errorf("bin with a page on a 48K was accepted")
	}
	// the script parser takes the fourth word
	sc, err := ParseScriptFile(writeTemp(t, "0   bin   x.bin   0x8000   -1   3\n"))
	if err != nil || len(sc.Actions) != 1 || len(sc.Actions[0].Args) != 4 {
		t.Errorf("parsing bin with a page: %v, %+v", err, sc)
	}
}

func writeTemp(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.zen")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
