// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package romimage

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The Go builder and scripts/mkrom.py are two programs for one job. This runs
// them on the same inputs and requires the same ROM and the same warning, byte
// for byte, so that neither can drift from the other. It is skipped where there
// is no python3 (or no script), and says so.
func TestAgreesWithMkromPy(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3: the comparison with scripts/mkrom.py was not run")
	}
	script, _ := filepath.Abs(filepath.Join("..", "..", "scripts", "mkrom.py"))
	original, err := os.ReadFile(filepath.Join("..", "..", "rom", "48.rom"))
	if err != nil || len(original) != Size {
		t.Skipf("no rom/48.rom to fill from: %v", err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skip("no scripts/mkrom.py")
	}

	dir := t.TempDir()
	rng := rand.New(rand.NewSource(66))
	sizes := []int{0, 1, 2, 100, 0x100, 0x2000, 0x3FFF, Size, Size + 1, 51200}
	addrs := []int{0, 1, 0x38, 0x1234, 0x3D00, 0x3F00, 0x3FFE, 0x3FFF}

	for n := 0; n < 60; n++ {
		size := sizes[rng.Intn(len(sizes))]
		addr := addrs[rng.Intn(len(addrs))]
		fill := []string{"zero", "rom"}[rng.Intn(2)]
		payload := make([]byte, size)
		rng.Read(payload)

		in := filepath.Join(dir, "in.bin")
		out := filepath.Join(dir, "out.rom")
		if err := os.WriteFile(in, payload, 0o644); err != nil {
			t.Fatal(err)
		}
		os.Remove(out)
		text := fmt.Sprintf("0x%X", addr)
		args := []string{"-I", script, in, "-o", out, "-a", text, "-f", fill, "-r", filepath.Join("..", "..", "rom", "48.rom")}
		var stderr bytes.Buffer
		cmd := exec.Command(py, args...)
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("mkrom.py %v: %v\n%s", args[3:], err, stderr.String())
		}
		want, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}

		base := Zeroes(Size)
		if fill == "rom" {
			base = original
		}
		parsed, err := ParseAddress(text)
		if err != nil {
			t.Fatal(err)
		}
		got, cut, err := Build(payload, parsed, base)
		if err != nil {
			t.Fatalf("size %d at %s: %v", size, text, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("size %d at %s fill %s: the Go image and mkrom.py's differ", size, text, fill)
		}

		// The warning, when there is one, reads the same.
		pyWarned := strings.Contains(stderr.String(), "past the 16 KiB ROM")
		if pyWarned != (cut > 0) {
			t.Fatalf("size %d at %s: mkrom.py warned %v, Go cut %d", size, text, pyWarned, cut)
		}
		if cut > 0 && !strings.Contains(stderr.String(), CutWarning(size, addr, Size, cut)) {
			t.Fatalf("size %d at %s: warnings differ:\n  python: %s  go: %s", size, text, stderr.String(), CutWarning(size, addr, Size, cut))
		}
	}
}
