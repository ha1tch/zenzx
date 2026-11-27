// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"reflect"
	"testing"
)

func TestIsDiskPathTakesDSKInAnyCase(t *testing.T) {
	for path, want := range map[string]bool{
		"a.dsk":          true,
		"/x/y/GAME.DSK":  true,
		"/x/y/Game.Dsk":  true,
		"a.tap":          false,
		"a.z80":          false,
		"dsk":            false,
		"a.dsk.bak":      false,
		"/folder.dsk/ab": false,
	} {
		if got := isDiskPath(path); got != want {
			t.Errorf("isDiskPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestDisksOfKeepsTheDisksInTheOrderGiven(t *testing.T) {
	list := []string{"/a/one.tap", "/a/b.dsk", "/a/snap.z80", "/c/A.DSK", "/d/two.tzx"}
	want := []string{"/a/b.dsk", "/c/A.DSK"}
	if got := disksOf(list); !reflect.DeepEqual(got, want) {
		t.Errorf("disksOf = %v, want %v", got, want)
	}
	if got := disksOf(nil); len(got) != 0 {
		t.Errorf("disksOf(nil) = %v, want nothing", got)
	}
	if got := disksOf([]string{"/a/one.tap"}); len(got) != 0 {
		t.Errorf("a list with no disk gave %v", got)
	}
}
