// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

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

// romBinFlagsFor parses a -rom-bin command line the way the programs do.
func romBinFlagsFor(t *testing.T, args ...string) *romBinFlags {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := addROMBinFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("flags %v: %v", args, err)
	}
	return f
}

func writeBin(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func machine48(t *testing.T) *ZenZX {
	t.Helper()
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	return zx
}

func TestROMBinDefaultsAreZeroFillAtZero(t *testing.T) {
	zx := machine48(t)
	payload := []byte{1, 2, 3, 4, 5}
	f := romBinFlagsFor(t, "-rom-bin", writeBin(t, "p.bin", payload))
	if _, warns, err := f.apply(zx); err != nil || len(warns) != 0 {
		t.Fatalf("apply: warnings %v, err %v", warns, err)
	}
	got := zx.memory.rom[0]
	if !bytes.Equal(got[:5], payload) {
		t.Error("the file is not at 0x0000")
	}
	if !bytes.Equal(got[5:], make([]byte, romimage.Size-5)) {
		t.Error("what is around the file is not zeroes")
	}
}

func TestROMBinOverlaysTheStandardROM(t *testing.T) {
	zx := machine48(t)
	original := zx.memory.rom[0]
	patch := bytes.Repeat([]byte{0xFF}, 0x20)
	f := romBinFlagsFor(t, "-rom-bin", writeBin(t, "p.bin", patch), "-rom-at", "0x0038", "-rom-fill", "rom")
	if _, _, err := f.apply(zx); err != nil {
		t.Fatal(err)
	}
	got := zx.memory.rom[0]
	if !bytes.Equal(got[0x38:0x58], patch) || !bytes.Equal(got[:0x38], original[:0x38]) || !bytes.Equal(got[0x58:], original[0x58:]) {
		t.Error("the overlay changed more than the patch, or not the patch")
	}
}

func TestROMBinOverlaysWhatRomFlagsLeftInTheBank(t *testing.T) {
	zx := machine48(t)
	other := bytes.Repeat([]byte{0x77}, romimage.Size)
	if err := zx.OverrideROMBank(0, writeBin(t, "other.rom", other)); err != nil {
		t.Fatal(err)
	}
	f := romBinFlagsFor(t, "-rom-bin", writeBin(t, "p.bin", []byte{9, 9}), "-rom-at", "2", "-rom-fill", "rom")
	if _, _, err := f.apply(zx); err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0x77, 0x77, 9, 9}, bytes.Repeat([]byte{0x77}, romimage.Size-4)...)
	if !bytes.Equal(zx.memory.rom[0][:], want) {
		t.Error("-rom-fill rom did not take the ROM the bank had, but the standard one")
	}
}

func TestROMBinThatRunsOffTheEndIsCutAndWarned(t *testing.T) {
	zx := machine48(t)
	f := romBinFlagsFor(t, "-rom-bin", writeBin(t, "big.bin", bytes.Repeat([]byte{0x55}, 0x300)), "-rom-at", "0x3F00")
	summary, warns, err := f.apply(zx)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "512 bytes left out") {
		t.Errorf("warnings = %q, want one saying 512 bytes were left out", warns)
	}
	if !strings.Contains(summary, "0x3F00-0x3FFF (256 bytes)") {
		t.Errorf("the summary does not say what was placed: %q", summary)
	}
	if !bytes.Equal(zx.memory.rom[0][0x3F00:], bytes.Repeat([]byte{0x55}, 0x100)) {
		t.Error("the kept part is not at the end of the ROM")
	}
}

func TestROMBinOnABankOtherThanZero(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadPlus3ROM("./rom/plus3-0.rom", "./rom/plus3-1.rom", "./rom/plus3-2.rom", "./rom/plus3-3.rom"); err != nil {
		t.Fatalf("LoadPlus3ROM: %v", err)
	}
	bank0 := zx.memory.rom[0]
	bank3 := zx.memory.rom[3]
	f := romBinFlagsFor(t, "-rom-bin", writeBin(t, "p.bin", []byte{0xAB, 0xCD}), "-rom-bank", "3", "-rom-at", "0x100", "-rom-fill", "rom")
	if _, _, err := f.apply(zx); err != nil {
		t.Fatal(err)
	}
	if zx.memory.rom[0] != bank0 {
		t.Error("bank 0 changed: the ROM went to the wrong bank")
	}
	got := zx.memory.rom[3]
	if got[0x100] != 0xAB || got[0x101] != 0xCD || !bytes.Equal(got[:0x100], bank3[:0x100]) || !bytes.Equal(got[0x102:], bank3[0x102:]) {
		t.Error("bank 3 is not its own ROM with the file over it")
	}
}

func TestROMBinOnTheTS2068ExtensionROMIs8KiB(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadTS2068ROMBytes(make([]byte, 16384), bytes.Repeat([]byte{0x11}, 8192)); err != nil {
		t.Fatalf("LoadTS2068ROMBytes: %v", err)
	}
	f := romBinFlagsFor(t, "-rom-bin", writeBin(t, "p.bin", bytes.Repeat([]byte{0x22}, 0x100)), "-rom-bank", "1", "-rom-at", "0x1F80", "-rom-fill", "rom", "-rom-save", filepath.Join(t.TempDir(), "ext.rom"))
	_, warns, err := f.apply(zx)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "8 KiB") {
		t.Errorf("warnings = %q, want a cut at the end of an 8 KiB ROM", warns)
	}
	want := append(bytes.Repeat([]byte{0x11}, 0x1F80), bytes.Repeat([]byte{0x22}, 0x80)...)
	if !bytes.Equal(zx.memory.ts2068ExtROM[:], want) {
		t.Error("the extension ROM is not as built")
	}
	saved, err := os.ReadFile(*f.save)
	if err != nil || !bytes.Equal(saved, want) {
		t.Errorf("the saved file is %d bytes, want the 8192 of the bank (err %v)", len(saved), err)
	}
}

// What -rom-save writes is the ROM that was built, a whole 16 KiB one, and
// loads again with -rom to the same effect.
func TestROMBinSaveWritesTheROMAndItLoadsBack(t *testing.T) {
	zx := machine48(t)
	save := filepath.Join(t.TempDir(), "made.rom")
	f := romBinFlagsFor(t, "-rom-bin", writeBin(t, "p.bin", []byte{0xDE, 0xAD}), "-rom-at", "$0100", "-rom-fill", "rom", "-rom-save", save)
	summary, _, err := f.apply(zx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "saved as "+save) {
		t.Errorf("the summary does not say the ROM was saved: %q", summary)
	}
	saved, err := os.ReadFile(save)
	if err != nil || len(saved) != romimage.Size {
		t.Fatalf("saved ROM: %d bytes, err %v; want %d", len(saved), err, romimage.Size)
	}
	if !bytes.Equal(saved, zx.memory.rom[0][:]) {
		t.Error("the saved ROM is not the one in the bank")
	}

	again := machine48(t)
	if err := again.OverrideROMBank(0, save); err != nil {
		t.Fatalf("the saved ROM does not load as a -rom: %v", err)
	}
	if again.memory.rom[0] != zx.memory.rom[0] {
		t.Error("loaded back, the ROM is not the same")
	}
}

func TestROMBinRefusesWhatIsWrong(t *testing.T) {
	bin := writeBin(t, "p.bin", []byte{1})
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing file", []string{"-rom-bin", filepath.Join(t.TempDir(), "nowhere.bin")}, "cannot read"},
		{"address past the ROM", []string{"-rom-bin", bin, "-rom-at", "0x4000"}, "outside the ROM"},
		{"negative address", []string{"-rom-bin", bin, "-rom-at=-1"}, "outside the ROM"},
		{"not an address", []string{"-rom-bin", bin, "-rom-at", "banana"}, "not an address"},
		{"bank the model lacks", []string{"-rom-bin", bin, "-rom-bank", "1"}, "not valid for this model"},
		{"unknown fill", []string{"-rom-bin", bin, "-rom-fill", "ones"}, "zero or rom"},
		{"address without a file", []string{"-rom-at", "0x38"}, "needs -rom-bin"},
		{"fill without a file", []string{"-rom-fill", "rom"}, "needs -rom-bin"},
		{"save without a file", []string{"-rom-save", "x.rom"}, "needs -rom-bin"},
		{"saving over the input", []string{"-rom-bin", bin, "-rom-save", bin}, "is the -rom-bin file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			zx := machine48(t)
			before := zx.memory.rom[0]
			_, _, err := romBinFlagsFor(t, c.args...).apply(zx)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want one saying %q", err, c.want)
			}
			if zx.memory.rom[0] != before {
				t.Error("a refused -rom-bin still changed the ROM")
			}
		})
	}
	if _, err := os.ReadFile(bin); err != nil {
		t.Errorf("the input file was damaged: %v", err)
	}
}

func TestROMBinEmptyFileWarns(t *testing.T) {
	zx := machine48(t)
	_, warns, err := romBinFlagsFor(t, "-rom-bin", writeBin(t, "empty.bin", nil)).apply(zx)
	if err != nil || len(warns) != 1 || !strings.Contains(warns[0], "empty") {
		t.Errorf("warnings %q, err %v: want one warning that the file is empty", warns, err)
	}
}

func TestROMBinIsInactiveWithoutTheFlag(t *testing.T) {
	if f := romBinFlagsFor(t); f.active() || f.check() != nil {
		t.Error("no flags given, yet -rom-bin is active or refused")
	}
}

// A ROM built from a few bytes of Z80 is the ROM the machine runs: the
// program, which sets the first attribute byte and loops, does it. The value is
// one the standard ROM never leaves there (it paints 0x38), so that finding it
// means the program ran.
func TestROMBinProgramRunsAsTheROM(t *testing.T) {
	// Without it, the standard ROM does not produce the value: the test can fail.
	std := machine48(t)
	std.Reset()
	runUntil(std, 5, func() bool { return false })
	if std.memory.Read(0x5800) == 0x16 {
		t.Fatal("the standard ROM leaves 0x16 in the first attribute byte, so the test below proves nothing")
	}

	program := []byte{
		0xF3,             // DI
		0x21, 0x00, 0x58, // LD HL,0x5800
		0x36, 0x16, // LD (HL),0x16
		0x18, 0xFE, // JR $
	}
	for _, fill := range []string{"zero", "rom"} {
		t.Run(fill, func(t *testing.T) {
			zx := machine48(t)
			f := romBinFlagsFor(t, "-rom-bin", writeBin(t, "prog.bin", program), "-rom-fill", fill)
			if _, _, err := f.apply(zx); err != nil {
				t.Fatal(err)
			}
			zx.Reset()
			if n := runUntil(zx, 5, func() bool { return zx.memory.Read(0x5800) == 0x16 }); n >= 5 {
				t.Fatalf("the program did not run: attribute byte is 0x%02X after %d frames", zx.memory.Read(0x5800), n)
			}
		})
	}
}
