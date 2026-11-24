// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package nextcard

import (
	"os"
	"path/filepath"
	"testing"
)

// Cleanup removes what an unfinished download leaves and nothing else: the three files a
// Next boots from stay, the card above all, and so does anything else in the folder.
func TestCleanupRemovesLeftoversAndNeverTheBootFiles(t *testing.T) {
	dir := t.TempDir()
	keep := []string{BootROMName, DivMMCROMName, CardName, "zenzx-card.img", "notes.txt"}
	gone := []string{BootROMName + ".part", "distribution.zip", "distribution.zip.part", "distribution.img.part", CardName + ".part"}
	for _, n := range append(append([]string{}, keep...), gone...) {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := Leftovers(dir); len(got) != len(gone) {
		t.Fatalf("Leftovers lists %d files, want %d: %v", len(got), len(gone), got)
	}
	removed, err := Cleanup(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != len(gone) {
		t.Fatalf("Cleanup removed %d files, want %d", len(removed), len(gone))
	}
	for _, r := range removed {
		if r.Size != 10 {
			t.Errorf("%s reported as %d bytes, want 10", r.Path, r.Size)
		}
	}
	for _, n := range gone {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			t.Errorf("%s is still there", n)
		}
	}
	for _, n := range keep {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s was removed: %v", n, err)
		}
	}
	if again, err := Cleanup(dir); err != nil || len(again) != 0 {
		t.Errorf("a second Cleanup removed %v (err %v), want nothing", again, err)
	}
}

// A folder that is not there has no leftovers and is not an error.
func TestCleanupOfAMissingFolder(t *testing.T) {
	removed, err := Cleanup(filepath.Join(t.TempDir(), "nowhere"))
	if err != nil || len(removed) != 0 {
		t.Fatalf("Cleanup of a missing folder: %v, %v", removed, err)
	}
}
