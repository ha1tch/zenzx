// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestManualOnlineIsTheReleasesFileOrTheLatestPage(t *testing.T) {
	if got, want := manualOnline("0.50.0"), "https://github.com/ha1tch/zenzx/releases/download/v0.50.0/zenzx-0.50.0-manual.pdf"; got != want {
		t.Errorf("a release: %s, want %s", got, want)
	}
	for _, v := range []string{"", "dev", "0.50.0-3-gabc1234", "0.50.0-3-gabc1234-dirty", "v0.50.0", "0.50"} {
		if got, want := manualOnline(v), "https://github.com/ha1tch/zenzx/releases/latest"; got != want {
			t.Errorf("%q has no manual of its own, and got %s, want %s", v, got, want)
		}
	}
}

func TestFindManualTakesTheFirstFolderThatHasIt(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write := func(dir, name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("%PDF-1.4\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write(a, "zenzx-0.49.0-manual.pdf") // another version's is not this one's
	if got := findManual("0.50.0", []string{a, b}); got != "" {
		t.Errorf("found %q with only another version's manual", got)
	}
	if err := os.Mkdir(filepath.Join(a, "zenzx-0.50.0-manual.pdf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findManual("0.50.0", []string{a, b}); got != "" {
		t.Errorf("a folder of that name was taken for the manual: %q", got)
	}
	want := write(b, "zenzx-0.50.0-manual.pdf")
	if got := findManual("0.50.0", []string{a, b}); got != want {
		t.Errorf("found %q, want %q", got, want)
	}
	first := write(a, "x") // keep a's own copy out of the way of the next line
	os.Remove(first)
	os.Remove(filepath.Join(a, "zenzx-0.50.0-manual.pdf"))
	nearer := write(a, "zenzx-0.50.0-manual.pdf")
	if got := findManual("0.50.0", []string{a, b}); got != nearer {
		t.Errorf("the first folder's copy was not preferred: %q", got)
	}
}

func TestOpenManualOpensTheFileBesideTheProgramOrTheReleasesCopy(t *testing.T) {
	dir := t.TempDir()
	var opened []string
	open := func(s string) { opened = append(opened, s) }

	openManual("0.50.0", []string{dir}, open)
	if want := []string{manualOnline("0.50.0")}; !reflect.DeepEqual(opened, want) {
		t.Fatalf("with none beside the program: %q, want %q", opened, want)
	}

	p := filepath.Join(dir, "zenzx-0.50.0-manual.pdf")
	if err := os.WriteFile(p, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened = nil
	openManual("0.50.0", []string{dir}, open)
	if want := []string{p}; !reflect.DeepEqual(opened, want) {
		t.Errorf("with one beside the program: %q, want %q", opened, want)
	}

	// a path that cannot be handed to the system's opener safely is not
	quoted := filepath.Join(dir, "it's here")
	if err := os.MkdirAll(quoted, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(quoted, "zenzx-0.50.0-manual.pdf"), []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened = nil
	openManual("0.50.0", []string{quoted}, open)
	if want := []string{manualOnline("0.50.0")}; !reflect.DeepEqual(opened, want) {
		t.Errorf("with a quote in the path: %q, want %q", opened, want)
	}
}

func TestManualDirsStartWithTheProgramsOwnFolder(t *testing.T) {
	dirs := manualDirs()
	exe, err := os.Executable()
	if err != nil {
		t.Skip("no executable path here")
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	if len(dirs) < 5 || dirs[0] != filepath.Dir(exe) || dirs[1] != filepath.Join(filepath.Dir(exe), "docs") {
		t.Errorf("manualDirs() = %q", dirs)
	}
}
