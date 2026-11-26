// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenzx/pkg/fetch"
)

// ZenZX is also the downloader that the examples' launchers use to get Python, so that they
// need no curl, wget or PowerShell (a Windows 7 has none of them). -fetch is that, and it is
// in both builds: it runs before any window is made, and exits.

const (
	fetchHelp       = "Fetch something and exit: python (a Python for the examples), list (what can be fetched), or the address of a file to download (-fetch-to says where it goes)"
	fetchToHelp     = "Where -fetch puts it: the folder for a name (default: its own name, here), the file for an address"
	fetchHowHelp    = "How -fetch python gets Python: private (a downloaded copy), brew (Homebrew), apt, dnf or yum (the system's own package), uv or conda (a separate environment), manual. Not given: a terminal is asked, and anything else is told what to give"
	fetchYesHelp    = "-fetch asks nothing: it takes the way that Return would"
	fetchForceHelp  = "-fetch fetches again even when it is already there"
	fetchURLHelp    = "-fetch: download from this address instead of the usual one (with -fetch-sha256, which an empty value switches off)"
	fetchSHAHelp    = "-fetch: the SHA-256 the download must have (for a file by address; or with -fetch-url)"
	fetchUnpackHelp = "-fetch an address: take the zip or .tar.gz apart into this folder"
	fetchStripHelp  = "-fetch-unpack: drop this many leading folders of every name"
)

// fetchFlags are the switches of -fetch.
type fetchFlags struct {
	name, to, how, url, sha, unpack *string
	strip                           *int
	yes, force                      *bool
}

func addFetchFlags(fs *flag.FlagSet) *fetchFlags {
	return &fetchFlags{
		name:   fs.String("fetch", "", fetchHelp),
		to:     fs.String("fetch-to", "", fetchToHelp),
		how:    fs.String("fetch-how", "", fetchHowHelp),
		yes:    fs.Bool("fetch-yes", false, fetchYesHelp),
		force:  fs.Bool("fetch-force", false, fetchForceHelp),
		url:    fs.String("fetch-url", "", fetchURLHelp),
		sha:    fs.String("fetch-sha256", "", fetchSHAHelp),
		unpack: fs.String("fetch-unpack", "", fetchUnpackHelp),
		strip:  fs.Int("fetch-strip", 0, fetchStripHelp),
	}
}

func (f *fetchFlags) asked() bool { return *f.name != "" }

// stdinIsTerminal reports whether somebody is at the keyboard to be asked.
func stdinIsTerminal() bool { return fetch.IsTerminal(os.Stdin) }

// fetchProgress writes lines about how a download is going: a line when the stage changes, and
// one at each tenth.
func fetchProgress(w io.Writer) func(stage string, done, total int64) {
	last, tenth := "", int64(-1)
	return func(stage string, done, total int64) {
		if stage != last {
			last, tenth = stage, -1
			fmt.Fprintf(w, "%s...\n", stage)
		}
		if total > 0 {
			if t := done * 10 / total; t != tenth {
				tenth = t
				fmt.Fprintf(w, "  %3d%%  %.1f of %.1f MB\n", done*100/total, float64(done)/(1<<20), float64(total)/(1<<20))
			}
		}
	}
}

// runFetch does what -fetch asks and returns the exit status: 0 done, 1 not done (the reason
// is said), 2 asked wrongly.
func runFetch(f *fetchFlags, stdin io.Reader, terminal bool, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	name := strings.TrimSpace(*f.name)
	switch lower := strings.ToLower(name); {
	case lower == "list":
		for _, n := range fetch.Names() {
			about, _ := fetch.About(n)
			fmt.Fprintf(stdout, "%-8s %s\n", n, about)
		}
		return 0
	case strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://"):
		return fetchAddress(ctx, f, name, stderr)
	}
	return fetchNamed(ctx, f, name, stdin, terminal, stdout, stderr)
}

// fetchAddress downloads a file by its address, and takes it apart when asked.
func fetchAddress(ctx context.Context, f *fetchFlags, address string, stderr io.Writer) int {
	u, err := url.Parse(address)
	if err != nil || u.Host == "" {
		fmt.Fprintf(stderr, "Error: %q is not an address to fetch\n", address)
		return 2
	}
	dest := *f.to
	temp := false
	if dest == "" {
		dest = path.Base(u.Path)
		if dest == "" || dest == "/" || dest == "." {
			dest = "download"
		}
		if *f.unpack != "" {
			dest, temp = filepath.Join(os.TempDir(), "zenzx-fetch-"+dest), true
		}
	}
	progress := fetchProgress(stderr)
	if _, err := fetch.File(ctx, fetch.Request{URLs: []string{address}, Dest: dest, SHA256: *f.sha,
		Progress: func(d, t int64) { progress("Downloading "+path.Base(u.Path), d, t) }}); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if temp {
		defer os.Remove(dest)
	}
	if *f.unpack != "" {
		if err := fetch.Unpack(ctx, dest, *f.unpack, fetch.UnpackOptions{Strip: *f.strip,
			Progress: func(d, t int64) { progress("Unpacking", d, t) }}); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "Unpacked into %s\n", *f.unpack)
		return 0
	}
	fmt.Fprintf(stderr, "Saved as %s\n", dest)
	return 0
}

// fetchNamed fetches something ZenZX knows by name. For Python that can be asked in several
// ways, depending on what the computer has.
func fetchNamed(ctx context.Context, f *fetchFlags, name string, stdin io.Reader, terminal bool, stdout, stderr io.Writer) int {
	host := fetch.ThisHost()
	if _, ok := fetch.About(strings.ToLower(name)); !ok {
		fmt.Fprintf(stderr, "Error: there is nothing called %q to fetch (there is: %s; or give an address)\n", name, strings.Join(fetch.Names(), ", "))
		return 2
	}
	name = strings.ToLower(name)
	it, resolveErr := fetch.Resolve(name, host)
	dir := *f.to
	if dir == "" {
		dir = name
	}
	if !*f.force {
		if p := fetch.Interpreter(dir, host); p != "" {
			fmt.Fprintf(stdout, "Already there: %s\n", p)
			return 0
		}
	}

	ways := fetch.WaysFor(name, host, fetch.HostLook)
	var way fetch.Way
	switch {
	case *f.how != "":
		w, ok := fetch.FindWay(ways, *f.how)
		if !ok {
			var ids []string
			for _, x := range ways {
				ids = append(ids, x.ID)
			}
			fmt.Fprintf(stderr, "Error: %q is not a way to get it on this computer (there is: %s)\n", *f.how, strings.Join(ids, ", "))
			return 2
		}
		way = w
	case *f.yes:
		for _, w := range ways {
			if w.Default {
				way = w
			}
		}
		if way.ID == "" {
			fmt.Fprintf(stderr, "Error: %v\n", resolveErr)
			return 1
		}
	case terminal:
		w, ok := fetch.Choose(ways, stdin, stdout)
		if !ok {
			fmt.Fprintln(stdout)
			return 1
		}
		way = w
	default:
		var ids []string
		for _, x := range ways {
			ids = append(ids, x.ID)
		}
		fmt.Fprintf(stderr, "Error: nobody is here to ask how: give -fetch-how (%s), or -fetch-yes for the first\n", strings.Join(ids, ", "))
		return 2
	}

	switch way.ID {
	case "manual":
		fmt.Fprintln(stdout, way.Detail)
		return 1
	case "private":
		if resolveErr != nil {
			fmt.Fprintf(stderr, "Error: %v\n", resolveErr)
			return 1
		}
		if _, err := fetch.Install(ctx, it, dir, fetch.InstallOptions{Force: *f.force, URL: *f.url, SHA256: *f.sha,
			Progress: fetchProgress(stderr)}); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Ready: %s\n", fetch.Interpreter(dir, host))
		return 0
	}
	p, err := fetch.RunWay(ctx, host, way, dir, fetch.HostLook, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Ready: %s\n", p)
	return 0
}
