//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"
	"strings"
	"testing"
)

// Renaming a .hires.scr or a .hicol.scr in the Screenshots window: the extension stays whole.

func TestRenamingATimexScreenKeepsItsExtension(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{"hires.scr", "hicol.scr"} {
		old := filepath.Join(dir, "screenshot_Horace_20201127_093001."+ext)
		for typed, want := range map[string]string{
			"Level 3":                         "Level 3." + ext,
			"Level 3." + ext:                  "Level 3." + ext,
			strings.ToUpper("Level 3." + ext): strings.ToUpper("Level 3." + ext),
		} {
			target, why := screenshotRenameTarget(old, typed)
			if why != "" || filepath.Base(target) != want {
				t.Errorf("%s, %q: %q, %q, want %q", ext, typed, target, why, want)
			}
		}
		refused := []string{"Level 3.scr", "Level 3.png", "Level 3.nxi", "Level 3.gif", "Level 3.timex.scr"}
		other := "hicol.scr"
		if ext == "hicol.scr" {
			other = "hires.scr"
		}
		for _, typed := range append(refused, "Level 3."+other) {
			if _, why := screenshotRenameTarget(old, typed); !strings.Contains(why, "stays ."+ext) {
				t.Errorf("%s, %q: %q, want it refused as a change of format", ext, typed, why)
			}
		}
	}
	// A plain .scr cannot become a Timex screen by its name either.
	for _, typed := range []string{"b.hires.scr", "b.hicol.scr"} {
		if _, why := screenshotRenameTarget(filepath.Join(dir, "a.scr"), typed); !strings.Contains(why, "stays .scr") {
			t.Errorf("a .scr typed as %q: %q", typed, why)
		}
	}
}
