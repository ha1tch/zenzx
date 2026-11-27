//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"strings"
	"testing"
	"time"
)

// The Screenshots window with a .hires.scr and a .hicol.scr: the row, the caption and the note,
// and the renaming.

func TestScreenshotsWindowReadsTimexScreensAsOneExtension(t *testing.T) {
	for _, c := range []struct{ ext, kind, note string }{
		{"hires.scr", "HIRES.SCR", "hi-res"},
		{"hicol.scr", "HICOL.SCR", "hi-colour"},
	} {
		f := screenshotFile{Name: "screenshot_Horace_20201127_093001." + c.ext, Ext: c.ext, Machine: "Horace", Size: 12288, When: time.Now()}
		when, who, kind, size := screenshotCells(f)
		if who != "Horace" || kind != c.kind || size == "" || when == "" {
			t.Errorf("%s cells: %q %q %q %q", c.ext, when, who, kind, size)
		}
		if got := screenshotCaption(f); got != "Horace "+c.kind {
			t.Errorf("%s caption %q", c.ext, got)
		}
		if got := screenshotKind(f); !strings.Contains(got, c.note) || strings.Contains(got, "guess") {
			t.Errorf("%s kind %q: it knows what the screen is", c.ext, got)
		}
		// Renamed, it is shown without its extension, whole.
		renamed := screenshotFile{Name: "my level." + c.ext, Ext: c.ext, Size: 12288, When: time.Now()}
		if _, who, _, _ := screenshotCells(renamed); who != "my level" {
			t.Errorf("a renamed %s is shown as %q, want \"my level\"", c.ext, who)
		}
		if got := screenshotCaption(renamed); got != "my level "+c.kind {
			t.Errorf("%s caption %q", c.ext, got)
		}
	}
	// A plain .scr of 12288 bytes does not say, and its note does.
	plain := screenshotFile{Name: "a.scr", Ext: "scr", Size: 12288}
	if got := screenshotKind(plain); !strings.Contains(got, "guesses") {
		t.Errorf("a plain .scr of 12288 bytes: %q", got)
	}
}
