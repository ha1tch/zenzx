package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDropMatrix reports, per model, which dropped-file load paths currently
// work. It is diagnostic: it does not assert a desired outcome, it prints the
// real state so a regression (e.g. introduced with Next support) is visible.
// Run: go test -tags headless,purego -run TestDropMatrix -v .
func TestDropMatrix(t *testing.T) {
	rom := func(name string) []byte {
		b, err := resolveROMBytes(name, "./rom")
		if err != nil {
			t.Fatalf("rom %s: %v", name, err)
		}
		return b
	}

	// Build the fixtures we don't already have on disk.
	dir := t.TempDir()
	scrPath := filepath.Join(dir, "blank.scr")
	if err := os.WriteFile(scrPath, make([]byte, 6912), 0o644); err != nil {
		t.Fatal(err)
	}

	// Real fixtures present in the tree.
	z80_48 := "testdata/snapshots/manicminer_v2_48k.z80"
	z80_128 := "testdata/snapshots/z80attack_v3_128k.z80"
	sna48 := "testdata/snapshots/z88dk_zx48.sna"
	dsk := "testdata/synthetic.dsk"
	tap := "examples/world/world.tap"

	// mk builds a machine of the named model.
	mk := func(model string) *ZenZX {
		zx := NewZenZX(AudioBackendOto)
		zx.cpu.Reset()
		switch model {
		case "48k":
			if err := zx.LoadROMBytes(rom("48.rom")); err != nil {
				t.Fatalf("48k rom: %v", err)
			}
		case "128k":
			if err := zx.Load128KROMBytes(rom("128-0.rom"), rom("128-1.rom")); err != nil {
				t.Fatalf("128k rom: %v", err)
			}
		case "next":
			if err := zx.Load128KROMBytes(rom("128-0.rom"), rom("128-1.rom")); err != nil {
				t.Fatalf("next rom: %v", err)
			}
			zx.memory.EnableNext()
		}
		return zx
	}

	// Each drop type maps to the call handleDroppedFiles makes for it.
	type dropCase struct {
		name string
		file string
		run  func(zx *ZenZX, file string) error
	}
	cases := []dropCase{
		{".scr", scrPath, func(zx *ZenZX, f string) error { return zx.screen.LoadFromFile(f) }},
		{".tap", tap, func(zx *ZenZX, f string) error {
			if zx.tape == nil {
				zx.tape = NewTape(zx)
				zx.tape.AttachFastLoader(&FastLoader{Enabled: true})
			}
			return zx.tape.LoadFile(f)
		}},
		{".sna", sna48, func(zx *ZenZX, f string) error { return zx.LoadSnapshotFormat(f, "sna") }},
		{".z80(48k file)", z80_48, func(zx *ZenZX, f string) error { return zx.LoadSnapshotFormat(f, "z80") }},
		{".z80(128k file)", z80_128, func(zx *ZenZX, f string) error { return zx.LoadSnapshotFormat(f, "z80") }},
		{".dsk", dsk, func(zx *ZenZX, f string) error {
			if zx.io.hasFDC && zx.io.fdc != nil {
				return zx.io.LoadDisk(f)
			}
			return errDropSkip
		}},
	}

	models := []string{"48k", "128k", "next"}
	t.Logf("drop-load matrix (OK = load returned no error):")
	for _, model := range models {
		var row []string
		for _, c := range cases {
			if _, err := os.Stat(c.file); err != nil {
				row = append(row, c.name+"=NOFIXTURE")
				continue
			}
			zx := mk(model)
			// +3 drive: enable the FDC where the model would have it, so .dsk
			// is actually exercised rather than skipped everywhere.
			if strings.HasPrefix(c.name, ".dsk") {
				zx.io.EnableFDC()
			}
			err := c.run(zx, c.file)
			switch {
			case err == errDropSkip:
				row = append(row, c.name+"=skip(noFDC)")
			case err != nil:
				row = append(row, c.name+"=FAIL("+short(err.Error())+")")
			default:
				row = append(row, c.name+"=OK")
			}
		}
		t.Logf("  %-5s : %s", model, strings.Join(row, "  "))
	}
}

var errDropSkip = os.ErrInvalid

func short(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}
