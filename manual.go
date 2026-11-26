// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Help > ZenZX Manual opens the manual as a PDF. Every release carries it beside the programs,
// as zenzx-<version>-manual.pdf; one that was put next to the program is opened from there,
// and without one the copy on the release's page is.

// manualReleases is the page of the project's releases, whose files are what manualOnline
// points at.
const manualReleases = "https://github.com/ha1tch/zenzx/releases"

// releaseVersion is what the version of a release looks like; a build made from the source tree
// between releases says more (a count of commits, a hash) and has no manual of its own.
var releaseVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// manualName is the name the release gives its manual.
func manualName(version string) string { return "zenzx-" + version + "-manual.pdf" }

// manualOnline is where the manual of this version is on the web: the file of its release, or,
// for a build that is not a release, the page of the latest one.
func manualOnline(version string) string {
	if !releaseVersion.MatchString(version) {
		return manualReleases + "/latest"
	}
	return manualReleases + "/download/v" + version + "/" + manualName(version)
}

// manualDirs are the folders searched for a manual that was put near the program: the
// program's own, the one above it (a package with the program in bin/), a docs folder in each,
// and the folder the program was started from.
func manualDirs() []string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		if r, err := filepath.EvalSymlinks(exe); err == nil {
			exe = r
		}
		d := filepath.Dir(exe)
		up := filepath.Dir(d)
		dirs = append(dirs, d, filepath.Join(d, "docs"), up, filepath.Join(up, "docs"))
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	return dirs
}

// findManual is the file of this version's manual in the first of dirs that has one, or "".
func findManual(version string, dirs []string) string {
	for _, d := range dirs {
		p := filepath.Join(d, manualName(version))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// openManual hands the manual to open: the file when there is one, the address of the release's
// copy when not. open is the system's way of opening a file or a page (a browser, or the PDF
// reader that is set for the file). A path with a quote in it cannot be handed over safely, so
// that is the same as none.
func openManual(version string, dirs []string, open func(string)) {
	if p := findManual(version, dirs); p != "" && !strings.ContainsAny(p, `'"`) {
		open(p)
		return
	}
	open(manualOnline(version))
}
