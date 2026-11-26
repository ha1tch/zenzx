// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package fetch

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Host is the kind of computer something is fetched for.
type Host struct {
	OS, Arch string // Go's names: "windows", "linux", "darwin"; "amd64", "arm64"
	// WinMajor and WinMinor are the Windows version (6.1 is Windows 7, 6.3 is 8.1, 10.0 is 10
	// and 11); 0 when it is not Windows, or not known.
	WinMajor, WinMinor int
}

func (h Host) String() string { return h.OS + "/" + h.Arch }

// Item is one thing ZenZX knows how to fetch, resolved for one computer.
type Item struct {
	Name    string   // what it is asked for as: "python"
	Title   string   // how a person is told of it: "Python 3.12.16"
	Size    string   // about how big the download is, for a person: "about 35 MB"
	From    string   // where it comes from, for a person: "github.com/astral-sh/python-build-standalone"
	URLs    []string // the download, tried in order
	SHA256  string   // of the download
	Strip   int      // leading folders of the archive that are dropped
	Check   string   // a file that is there once it is installed, relative to the folder
	DirName string   // the name the folder is given when nobody says otherwise
}

// recipes are the things that can be fetched by name. Each says, for a computer, what to fetch.
var recipes = map[string]struct {
	about   string
	resolve func(Host) (Item, error)
}{
	"python": {
		about:   "a private copy of Python, which the examples' helper programs need (installs nothing)",
		resolve: resolvePython,
	},
}

// Names are the things that can be fetched by name, in order.
func Names() []string {
	var n []string
	for k := range recipes {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

// About says in a line what a name fetches.
func About(name string) (string, bool) {
	r, ok := recipes[name]
	return r.about, ok
}

// Resolve says what to fetch for name on host h.
func Resolve(name string, h Host) (Item, error) {
	r, ok := recipes[strings.ToLower(name)]
	if !ok {
		return Item{}, fmt.Errorf("there is nothing called %q to fetch (there is: %s)", name, strings.Join(Names(), ", "))
	}
	it, err := r.resolve(h)
	if err != nil {
		return Item{}, err
	}
	it.Name = strings.ToLower(name)
	return it, nil
}

// The Python that is fetched. For Linux and macOS it is the python-build-standalone project's
// build, which unpacks to a folder of its own and runs from wherever it is put; for Windows it
// is the embeddable package from python.org, which does the same. Windows 7 gets 3.8.10, the
// last Python that runs there; every later Windows gets 3.12.10, the last 3.12 that python.org
// made an embeddable package of.
const (
	pbsTag    = "20261124"
	pbsPython = "3.12.16"
	pbsBase   = "https://github.com/astral-sh/python-build-standalone/releases/download/" + pbsTag
	pyOrgBase = "https://www.python.org/ftp/python/"
)

func pbs(triple, sha string) Item {
	file := "cpython-" + pbsPython + "%2B" + pbsTag + "-" + triple + "-install_only_stripped.tar.gz"
	return Item{
		Title: "Python " + pbsPython, Size: "about 35 MB", From: "github.com/astral-sh/python-build-standalone",
		URLs: []string{pbsBase + "/" + file}, SHA256: sha, Strip: 1, Check: "bin/python3", DirName: "python",
	}
}

func embeddable(version, arch, sha string) Item {
	return Item{
		Title: "Python " + version, Size: "about 10 MB", From: "python.org",
		URLs:   []string{pyOrgBase + version + "/python-" + version + "-embed-" + arch + ".zip"},
		SHA256: sha, Check: "python.exe", DirName: "python",
	}
}

func resolvePython(h Host) (Item, error) {
	switch h.OS + "/" + h.Arch {
	case "linux/amd64":
		return pbs("x86_64-unknown-linux-gnu", "7e1adaea26c959d13aa1120be1ae62a1ea1bb52adcb382679fffe9217e22adff"), nil
	case "linux/arm64":
		return pbs("aarch64-unknown-linux-gnu", "518e24b37e356775d82d5609cd9d48e27e211381a39ae416381ba696bf76c25c"), nil
	case "darwin/arm64":
		return pbs("aarch64-apple-darwin", "9d6d9d0d85feda3f9cb4eb3e048f0fed21ed280feda087927eaa4fbc7124826f"), nil
	case "darwin/amd64":
		return pbs("x86_64-apple-darwin", "6d7ef4efa1adf65f312e6e865040ce03ef8a4bd7e1b45415193295c9ffc0dbdd"), nil
	case "windows/amd64":
		if h.WinMajor != 0 && (h.WinMajor < 6 || h.WinMajor == 6 && h.WinMinor < 3) { // Windows 7 and Vista
			return embeddable("3.8.10", "amd64", "abbe314e9b41603dde0a823b76f5bbbe17b3de3e5ac4ef06b759da5466711271"), nil
		}
		return embeddable("3.12.10", "amd64", "4acbed6dd1c744b0376e3b1cf57ce906f9dc9e95e68824584c8099a63025a3c3"), nil
	case "windows/arm64":
		return embeddable("3.12.10", "arm64", "3065efc3d382d1cda66757ac71ade11904fa6e350f5a97eb74811acd71ba5532"), nil
	}
	return Item{}, fmt.Errorf("there is no ready-made Python for this kind of computer (%s)", h)
}

// InstallOptions say how Install does its work.
type InstallOptions struct {
	// Force installs again even when the thing is already there.
	Force bool
	// URL and SHA256, when URL is not empty, replace the recipe's address and its checksum; an
	// empty SHA256 then means the download is not checked. For tests, and for a mirror.
	URL, SHA256 string
	Client      *http.Client
	// Progress is told the stage ("Downloading Python 3.12.16", "Unpacking") and how far on it is.
	Progress func(stage string, done, total int64)
}

// Installed reports whether it is already in dir: the file Check names is there.
func (it Item) Installed(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(it.Check)))
	return err == nil && !st.IsDir()
}

// Install fetches it, checks it, unpacks it into a folder beside dir and, only when that holds
// the file Check names, puts it in place of dir. A failure leaves dir as it was. It returns
// false when it was already there and Force was not set.
func Install(ctx context.Context, it Item, dir string, o InstallOptions) (bool, error) {
	if o.Progress == nil {
		o.Progress = func(string, int64, int64) {}
	}
	if !o.Force && it.Installed(dir) {
		return false, nil
	}
	urls, sha := it.URLs, it.SHA256
	if o.URL != "" {
		urls, sha = []string{o.URL}, o.SHA256
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return false, err
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return false, err
	}
	base := filepath.Base(dir)
	archive := filepath.Join(parent, "."+base+"-download")
	staging := filepath.Join(parent, "."+base+"-new")
	os.RemoveAll(staging)
	defer os.RemoveAll(staging)

	stage := "Downloading " + it.Title
	if _, err := File(ctx, Request{URLs: urls, Dest: archive, SHA256: sha, Client: o.Client,
		Progress: func(d, t int64) { o.Progress(stage, d, t) }}); err != nil {
		return false, err
	}
	defer os.Remove(archive)
	if err := Unpack(ctx, archive, staging, UnpackOptions{Strip: it.Strip,
		Progress: func(d, t int64) { o.Progress("Unpacking "+it.Title, d, t) }}); err != nil {
		return false, fmt.Errorf("%s could not be unpacked: %w", it.Title, err)
	}
	if !it.Installed(staging) {
		return false, fmt.Errorf("what was downloaded does not hold %s where it should (no %s)", it.Title, it.Check)
	}
	if err := os.RemoveAll(dir); err != nil {
		return false, fmt.Errorf("the old %s could not be removed: %w", base, err)
	}
	if err := os.Rename(staging, dir); err != nil {
		return false, err
	}
	return true, nil
}
