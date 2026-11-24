package zenzx

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/romimage"
)

// T-76: the ROM switches on a Next that boots from its card, which loads its own ROMs over
// whatever they did. These tests drive the firmware's side of config mode by hand: NextREG
// 0x03 to enter, 0x04 to choose a bank, writes through the window, 0x03 to choose the machine,
// and the soft reset (NextREG 0x02) that starts it.

type romLog struct{ said, warned []string }

func (l *romLog) sink() romSink {
	return romSink{
		say:  func(s string) { l.said = append(l.said, s) },
		warn: func(s string) { l.warned = append(l.warned, s) },
	}
}

func nextReg(zx *ZenZX, reg, value uint8) {
	zx.io.WritePort(0x243B, reg)
	zx.io.WritePort(0x253B, value)
}

// firmwareLoads is what the firmware does in config mode: bank 0 filled with fill through the
// window; then it chooses machine type 3 and resets.
func firmwareLoads(zx *ZenZX, fill uint8) {
	nextReg(zx, 0x03, 0x07)
	nextReg(zx, 0x04, 0x00)
	for a := 0; a < 0x4000; a++ {
		zx.memory.nextConfigWrite(uint16(a), fill)
	}
	nextReg(zx, 0x03, 0x03)
	nextReg(zx, 0x02, 0x01)
	zx.RunFrame()
}

func nextWithBootROM(t *testing.T) *ZenZX {
	t.Helper()
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Skip(err)
	}
	zx.memory.SetBootROM(make([]byte, 8192)) // a Next that has its boot ROM re-enters config mode on a hard reset
	return zx
}

func binSwitch(t *testing.T, args ...string) *romBinFlags {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := addROMBinFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestROMSwitchesOnANextWaitForTheFirmware(t *testing.T) {
	zx := nextWithBootROM(t)
	dir := t.TempDir()
	binPath := filepath.Join(dir, "p.bin")
	savePath := filepath.Join(dir, "saved.rom")
	payload := []byte{1, 2, 3, 4}
	if err := os.WriteFile(binPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	standard := zx.memory.rom[0]

	var log romLog
	err := zx.applyROMSwitches(romSwitches{bin: binSwitch(t, "-rom-bin", binPath, "-rom-fill", "rom", "-rom-save", savePath)}, true, log.sink())
	if err != nil {
		t.Fatal(err)
	}
	if zx.memory.rom[0] != standard {
		t.Error("a switch on a Next that boots from its card acted before the firmware had loaded its ROMs")
	}
	if len(log.warned) != 1 || !strings.Contains(log.warned[0], "replaces the ROM banks") {
		t.Errorf("want one warning that the firmware replaces the ROMs, got %q", log.warned)
	}
	if _, err := os.Stat(savePath); err == nil {
		t.Error("-rom-save wrote the ROM before the firmware's ROMs were there to build it from")
	}

	// The firmware loads bank 0 and leaves config mode: the patch goes over what it loaded.
	firmwareLoads(zx, 0x11)
	rom := zx.memory.rom[0]
	if !bytes.Equal(rom[:4], payload) || rom[4] != 0x11 || rom[0x3FFF] != 0x11 {
		t.Errorf("after the firmware: ROM 0 starts % X, then 0x%02X ... 0x%02X; want the file, then what the firmware loaded (0x11)", rom[:4], rom[4], rom[0x3FFF])
	}
	saved, err := os.ReadFile(savePath)
	if err != nil || len(saved) != romimage.Size || !bytes.Equal(saved[:4], payload) || saved[4] != 0x11 {
		t.Errorf("-rom-save after the firmware: %d bytes, err %v; want the file over the firmware's ROM", len(saved), err)
	}

	// A hard reset runs the firmware again, and the patch is applied again over what it loads.
	nextReg(zx, 0x02, 0x02)
	zx.RunFrame()
	if !zx.memory.nextCfg.active {
		t.Fatal("a hard reset did not put the Next back in config mode")
	}
	firmwareLoads(zx, 0x22)
	rom = zx.memory.rom[0]
	if !bytes.Equal(rom[:4], payload) || rom[4] != 0x22 {
		t.Errorf("after a hard reset: ROM 0 starts % X, then 0x%02X; want the file, then 0x22", rom[:4], rom[4])
	}

	// A soft reset of the running machine is not the firmware loading, and leaves ROM alone.
	zx.memory.rom[0][0] = 0x99
	nextReg(zx, 0x02, 0x01)
	zx.RunFrame()
	if zx.memory.rom[0][0] != 0x99 {
		t.Error("a soft reset applied the ROM patches again")
	}
}

func TestROMFileSwitchesOnANextWaitForTheFirmware(t *testing.T) {
	zx := nextWithBootROM(t)
	p := filepath.Join(t.TempDir(), "bank3.rom")
	if err := os.WriteFile(p, bytes.Repeat([]byte{0x77}, romimage.Size), 0o644); err != nil {
		t.Fatal(err)
	}
	var log romLog
	if err := zx.applyROMSwitches(romSwitches{bank: [4]string{3: p}}, true, log.sink()); err != nil {
		t.Fatal(err)
	}
	if zx.memory.rom[3][0] == 0x77 {
		t.Error("-rom3 acted before the firmware loaded its ROMs")
	}
	firmwareLoads(zx, 0x11)
	if zx.memory.rom[3][0] != 0x77 || zx.memory.rom[3][0x3FFF] != 0x77 {
		t.Error("-rom3 was not applied when the firmware left config mode")
	}
	if got := zx.romPatchKeys(); len(got) != 1 || !strings.Contains(got[0], "rom bank 3 sha256:") {
		t.Errorf("the boot cache's key does not know about the switch: %q", got)
	}
}

// Without the firmware (a Next on the +3's ROMs) the switches act at once, as on any model.
func TestROMSwitchesOnANextWithoutBootFilesActAtOnce(t *testing.T) {
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Skip(err)
	}
	p := filepath.Join(t.TempDir(), "bank1.rom")
	if err := os.WriteFile(p, bytes.Repeat([]byte{0x66}, romimage.Size), 0o644); err != nil {
		t.Fatal(err)
	}
	var log romLog
	if err := zx.applyROMSwitches(romSwitches{list: "," + p}, false, log.sink()); err != nil {
		t.Fatal(err)
	}
	if zx.memory.rom[1][0] != 0x66 {
		t.Error("-rom did not put its file in bank 1")
	}
	if len(log.warned) != 0 {
		t.Errorf("unexpected warnings: %q", log.warned)
	}
}

// -rom-bank 0-7 and a physical -rom-at on a Next: a file can run across banks, and the alternate
// ROMs are reached.
func TestROMBinReachesTheNextsWholeROMSpace(t *testing.T) {
	file := func(t *testing.T, n int, b byte) string {
		p := filepath.Join(t.TempDir(), "p.bin")
		if err := os.WriteFile(p, bytes.Repeat([]byte{b}, n), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Skip(err)
	}
	m := zx.memory

	// 8 KiB at 0x3000 of bank 0 runs into bank 1; the zero fill clears the rest of both.
	var log romLog
	if err := zx.applyROMSwitches(romSwitches{bin: binSwitch(t, "-rom-bin", file(t, 8*1024, 0xAB), "-rom-at", "0x3000")}, false, log.sink()); err != nil {
		t.Fatal(err)
	}
	if m.rom[0][0x2FFF] != 0 || m.rom[0][0x3000] != 0xAB || m.rom[1][0x0FFF] != 0xAB || m.rom[1][0x1000] != 0 {
		t.Errorf("a file across banks 0 and 1: 0x2FFF=0x%02X 0x3000=0x%02X bank 1 0x0FFF=0x%02X 0x1000=0x%02X",
			m.rom[0][0x2FFF], m.rom[0][0x3000], m.rom[1][0x0FFF], m.rom[1][0x1000])
	}

	// -rom-bank 6 is the alternate ROM 0.
	log = romLog{}
	if err := zx.applyROMSwitches(romSwitches{bin: binSwitch(t, "-rom-bin", file(t, 16, 0xCD), "-rom-bank", "6", "-rom-at", "0x10")}, false, log.sink()); err != nil {
		t.Fatal(err)
	}
	if got := m.nextPhysRead(6*romimage.Size + 0x10); got != 0xCD {
		t.Errorf("-rom-bank 6: the alternate ROM holds 0x%02X at 0x10, want 0xCD", got)
	}

	// A file that runs past the 256 KiB (a physical address, the last 16 bytes of it) is cut, with a
	// warning that says so.
	log = romLog{}
	if err := zx.applyROMSwitches(romSwitches{bin: binSwitch(t, "-rom-bin", file(t, 32, 0xEF), "-rom-at", "0x3FFF0")}, false, log.sink()); err != nil {
		t.Fatal(err)
	}
	if len(log.warned) != 1 || !strings.Contains(log.warned[0], "past the 256 KiB ROM") {
		t.Errorf("a file cut at the end of the 256 KiB: warnings %q", log.warned)
	}

	// Banks 8 and up are the DivMMC's RAM, and an address past the 256 KiB is no ROM.
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-rom-bank", "8"}, "valid range: 0-7"},
		{[]string{"-rom-at", "0x40000"}, "past the Next's 256 KiB"},
	} {
		err := zx.applyROMSwitches(romSwitches{bin: binSwitch(t, append([]string{"-rom-bin", file(t, 4, 1)}, c.args...)...)}, false, (&romLog{}).sink())
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: error %v, want one saying %q", c.args, err, c.want)
		}
	}
}

// The view of which ROM is live: the lock, the alternate ROM and RAM over the ROM each change it.
func TestNextROMLive(t *testing.T) {
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Skip(err)
	}
	m := zx.memory
	m.nextCfg.active, m.bootROMEnabled = false, false
	for i := 0; i < 2; i++ {
		m.SetNextSlot(i, 0xFF)
	}
	cases := []struct {
		name  string
		set   func()
		wants string
	}{
		{"ROM chosen by the ports", func() { m.romBank = 2; m.nextAltROM = 0 }, "ROM 2, SRAM bank 2"},
		{"ROM locked to 3", func() { m.nextAltROM = 0x30 }, "ROM 3, SRAM bank 3"},
		{"alternate ROM for reads", func() { m.nextAltROM = 0x80 }, "alternate ROM, SRAM bank"},
		{"RAM over the ROM", func() { m.nextAltROM = 0; m.SetNextSlot(0, 0x20) }, "paged over the ROM by the MMU"},
	}
	for _, c := range cases {
		c.set()
		if got := zx.NextROMLive(); !strings.Contains(got, c.wants) || !strings.Contains(got, "NextREG 0x8C=") {
			t.Errorf("%s: %q does not say %q", c.name, got, c.wants)
		}
	}
}
