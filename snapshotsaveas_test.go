//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// notices runs f with the machine's OnNotice collecting what it says.
func notices(zx *ZenZX, f func()) []string {
	var got []string
	zx.OnNotice = func(msg string) { got = append(got, msg) }
	defer func() { zx.OnNotice = nil }()
	f()
	return got
}

func TestSaveSnapshotAsEachFormat(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, f := range snapshotSaveFormats {
		// A name with no extension gets the format's; one with it is kept.
		for _, name := range []string{"plain", "named." + f.format} {
			path := filepath.Join(dir, f.format+"-"+name)
			got := notices(zx, func() { zx.saveSnapshotAs(path, f.format) })
			want := path
			if !strings.HasSuffix(want, "."+f.format) {
				want += "." + f.format
			}
			if _, err := os.Stat(want); err != nil {
				t.Errorf("%s: the file %s is not there: %v", f.format, filepath.Base(want), err)
				continue
			}
			if len(got) != 1 || !strings.Contains(got[0], "Saved snapshot "+filepath.Base(want)) || !strings.Contains(got[0], strings.ToUpper(f.format)) {
				t.Errorf("%s: the notice is %q, want one that says the snapshot %s is saved", f.format, got, filepath.Base(want))
			}
			// What was saved is the format asked for, and loads back.
			if det := DetectSnapshotFormat(want); det != f.format {
				t.Errorf("%s: the file is detected as %q", f.format, det)
			}
			if err := zx.LoadSnapshotFormat(want, f.format); err != nil {
				t.Errorf("%s: the saved file does not load: %v", f.format, err)
			}
		}
	}
}

func TestSaveSnapshotAsSaysWhenItCannot(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatal(err)
	}
	// A folder that is not there: the write fails, and the person is told, with the file's name.
	path := filepath.Join(t.TempDir(), "no-such-folder", "x")
	got := notices(zx, func() { zx.saveSnapshotAs(path, "z80") })
	if len(got) != 1 || !strings.Contains(got[0], "could not be saved") || !strings.Contains(got[0], "x.z80") {
		t.Errorf("the notice is %q, want one that says x.z80 could not be saved", got)
	}
	// .szx is read, not written: it is not offered, and asking for it says why.
	for _, f := range snapshotSaveFormats {
		if f.format == "szx" || f.format == "rzx" {
			t.Errorf("%s is offered by Save As, and ZenZX does not write it", f.format)
		}
	}
	got = notices(zx, func() { zx.saveSnapshotAs(filepath.Join(t.TempDir(), "s"), "szx") })
	if len(got) != 1 || !strings.Contains(got[0], "not supported") {
		t.Errorf("the notice is %q, want one that says .szx is not supported", got)
	}
}
