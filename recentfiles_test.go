// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAddRecentFileKeepsNewestFirstOnceEach(t *testing.T) {
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a.tap"), filepath.Join(dir, "b.dsk"), filepath.Join(dir, "c.z80")
	var list []string
	for _, p := range []string{a, b, c, a} {
		list = addRecentFile(list, p)
	}
	if want := []string{a, c, b}; !reflect.DeepEqual(list, want) {
		t.Errorf("list %v, want %v", list, want)
	}
	// A file that is of no kind that is opened is not kept, and an empty path is nothing.
	if got := addRecentFile(list, filepath.Join(dir, "notes.txt")); !reflect.DeepEqual(got, list) {
		t.Errorf("a .txt was kept: %v", got)
	}
	if got := addRecentFile(list, ""); !reflect.DeepEqual(got, list) {
		t.Errorf("an empty path was kept: %v", got)
	}
	// The kind is read in either case, and the list does not grow past its limit.
	big := []string{}
	for i := 0; i < recentFilesMax+5; i++ {
		big = addRecentFile(big, filepath.Join(dir, string(rune('a'+i%26))+string(rune('a'+i/26))+".TAP"))
	}
	if len(big) != recentFilesMax {
		t.Errorf("%d kept, want the limit %d", len(big), recentFilesMax)
	}
}

func TestAddRecentFileKeepsAbsolutePathsSoOneFileIsNotTwo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	got := addRecentFile(nil, "x.tap")
	want, _ := filepath.Abs("x.tap")
	if len(got) != 1 || got[0] != want || !filepath.IsAbs(got[0]) {
		t.Errorf("kept %v, want %q", got, want)
	}
	got = addRecentFile(got, filepath.Join(dir, "x.tap"))
	if len(got) != 1 {
		t.Errorf("one file kept twice: %v", got)
	}
}

func TestEveryKindFileOpenOffersIsOpenable(t *testing.T) {
	for _, ext := range fileOpenExts {
		if !isOpenableFile("/g/x." + ext) {
			t.Errorf(".%s is offered by File > Open but not kept as a recent file", ext)
		}
	}
	if isOpenableFile("/g/x.txt") || isOpenableFile("/g/tap") {
		t.Error("a file of no kind that is opened counts as openable")
	}
}

func TestExistingFilesDropsWhatIsGone(t *testing.T) {
	dir := t.TempDir()
	here, gone := filepath.Join(dir, "here.tap"), filepath.Join(dir, "gone.tap")
	if err := os.WriteFile(here, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := existingFiles([]string{gone, here, dir}); !reflect.DeepEqual(got, []string{here}) {
		t.Errorf("existing %v, want only the file that is there", got)
	}
}

// An Open dialog starts in the folder kept for it; with none kept, or one that has gone, in the home
// folder -- never the folder the program was started in.
func TestOpenStartDirIsTheKeptFolderOrHomeNeverTheWorkingDirectory(t *testing.T) {
	home, kept, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Chdir(cwd)
	if got := openStartDir(kept); got != kept {
		t.Errorf("a kept folder that is there: %q, want %q", got, kept)
	}
	for name, k := range map[string]string{
		"none kept":      "",
		"folder gone":    filepath.Join(kept, "gone"),
		"kept is a file": mustFile(t, kept, "f.tap"),
	} {
		if got := openStartDir(k); got != home {
			t.Errorf("%s: %q, want the home folder %q", name, got, home)
		}
	}
}

func mustFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
