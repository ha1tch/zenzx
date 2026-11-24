// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package nextcard

import (
	"os"
	"path/filepath"
)

// leftoverNames are the files a download leaves behind when it is stopped or fails, by the
// names Setup gives them: the ".part" of each file being written, and the distribution zip
// that is deleted once the card has been made from it.
var leftoverNames = []string{
	BootROMName + ".part",
	"distribution.zip",
	"distribution.zip.part",
	"distribution.img.part",
	CardName + ".part",
}

// Leftover is a file of a download that did not finish.
type Leftover struct {
	Path string
	Size int64
}

// Leftovers lists what an interrupted or failed Setup left in dir. The three files a Next
// boots from are never in it, whatever state they are in.
func Leftovers(dir string) []Leftover {
	var out []Leftover
	for _, name := range leftoverNames {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, Leftover{Path: p, Size: st.Size()})
		}
	}
	return out
}

// Cleanup deletes the leftovers of dir and returns what it deleted. It does not touch the
// boot ROM, the DivMMC ROM or the card: the card may hold what the user has saved on it.
// It stops at the first file it cannot delete and says which.
func Cleanup(dir string) ([]Leftover, error) {
	var removed []Leftover
	for _, l := range Leftovers(dir) {
		if err := os.Remove(l.Path); err != nil {
			return removed, err
		}
		removed = append(removed, l)
	}
	return removed, nil
}
