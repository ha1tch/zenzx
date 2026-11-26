// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type entry struct {
	name, body, link string
	mode             int64
	dir, hard        bool
}

func makeTarGz(t *testing.T, es []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range es {
		h := &tar.Header{Name: e.name, Mode: e.mode}
		switch {
		case e.dir:
			h.Typeflag, h.Mode = tar.TypeDir, 0o755
		case e.hard:
			h.Typeflag, h.Linkname = tar.TypeLink, e.link
		case e.link != "":
			h.Typeflag, h.Linkname = tar.TypeSymlink, e.link
		default:
			h.Typeflag, h.Size = tar.TypeReg, int64(len(e.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func makeZip(t *testing.T, es []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range es {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(os.FileMode(e.mode))
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.body))
	}
	zw.Close()
	return buf.Bytes()
}

func write(t *testing.T, dir, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func pythonLike() []entry {
	return []entry{
		{name: "python/", dir: true},
		{name: "python/bin/", dir: true},
		{name: "python/bin/python3.12", body: "#!the program", mode: 0o755},
		{name: "python/bin/python3", link: "python3.12"},
		{name: "python/lib/x.py", body: "x = 1", mode: 0o644},
		{name: "python/bin/py", link: "python/bin/python3.12", hard: true},
	}
}

func TestUnpackTarGzStripsAndKeepsPrograms(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "p.tar.gz", makeTarGz(t, pythonLike()))
	dest := filepath.Join(dir, "out")
	if err := Unpack(context.Background(), a, dest, UnpackOptions{Strip: 1}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "bin", "python3")); string(b) != "#!the program" {
		t.Fatalf("bin/python3 (a link to python3.12) reads %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "bin", "py")); string(b) != "#!the program" {
		t.Fatalf("a hard link reads %q", b)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(filepath.Join(dest, "bin", "python3.12")); st.Mode()&0o100 == 0 {
			t.Error("the program lost its execute bit")
		}
		if st, _ := os.Stat(filepath.Join(dest, "lib", "x.py")); st.Mode()&0o111 != 0 {
			t.Error("a plain file gained an execute bit")
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "python")); err == nil {
		t.Error("the folder that Strip drops was made")
	}
}

func TestUnpackMakesLinksAsCopiesWhereTheSystemCannotLink(t *testing.T) {
	old := linksOK
	linksOK = false
	defer func() { linksOK = old }()
	dir := t.TempDir()
	// the link comes before what it points to, and one link points to another
	es := []entry{
		{name: "top/bin/python3", link: "python3.12"},
		{name: "top/bin/python", link: "python3"},
		{name: "top/bin/python3.12", body: "the program", mode: 0o755},
	}
	a := write(t, dir, "p.tar.gz", makeTarGz(t, es))
	dest := filepath.Join(dir, "out")
	if err := Unpack(context.Background(), a, dest, UnpackOptions{Strip: 1}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"python3", "python"} {
		p := filepath.Join(dest, "bin", n)
		st, err := os.Lstat(p)
		if err != nil || st.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%s: %v, mode %v: want a plain file", n, err, st)
		}
		if b, _ := os.ReadFile(p); string(b) != "the program" {
			t.Errorf("%s reads %q", n, b)
		}
	}
	// a link to nothing, and a link to a folder, are refused with the reason
	for name, es := range map[string][]entry{
		"to nothing":  {{name: "top/a", link: "nothing"}},
		"to a folder": {{name: "top/d/", dir: true}, {name: "top/l", link: "d"}},
	} {
		d2 := filepath.Join(dir, "o2")
		os.RemoveAll(d2)
		err := Unpack(context.Background(), write(t, dir, "q.tar.gz", makeTarGz(t, es)), d2, UnpackOptions{Strip: 1})
		if err == nil || !strings.Contains(err.Error(), "link") {
			t.Errorf("a link %s: %v", name, err)
		}
	}
}

func TestUnpackZip(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "p.zip", makeZip(t, []entry{
		{name: "python.exe", body: "MZ"},
		{name: "Lib\\site.py", body: "# backslashes are slashes"}, // names a Windows tool wrote
		{name: "DLLs/", dir: true},
		{name: "DLLs/x.pyd", body: "dll"},
	}))
	dest := filepath.Join(dir, "out")
	var done, total int64
	if err := Unpack(context.Background(), a, dest, UnpackOptions{Progress: func(d, tot int64) { done, total = d, tot }}); err != nil {
		t.Fatal(err)
	}
	for n, want := range map[string]string{"python.exe": "MZ", "Lib/site.py": "# backslashes are slashes", "DLLs/x.pyd": "dll"} {
		if b, _ := os.ReadFile(filepath.Join(dest, filepath.FromSlash(n))); string(b) != want {
			t.Errorf("%s reads %q", n, b)
		}
	}
	if done == 0 || done != total {
		t.Errorf("progress ended at %d of %d", done, total)
	}
}

func TestUnpackRefusesWhatEscapes(t *testing.T) {
	dir := t.TempDir()
	cases := map[string][]byte{
		"climbing name (tar)":  makeTarGz(t, []entry{{name: "../evil", body: "x"}}),
		"climbing in a path":   makeTarGz(t, []entry{{name: "a/../../evil", body: "x"}}),
		"absolute name (tar)":  makeTarGz(t, []entry{{name: "/etc/evil", body: "x"}}),
		"drive letter (tar)":   makeTarGz(t, []entry{{name: "C:/evil", body: "x"}}),
		"link out":             makeTarGz(t, []entry{{name: "l", link: "../outside"}}),
		"absolute link":        makeTarGz(t, []entry{{name: "l", link: "/etc/passwd"}}),
		"write through a link": makeTarGz(t, []entry{{name: "d/", dir: true}, {name: "l", link: "d"}, {name: "l/evil", body: "x"}}),
		"climbing name (zip)":  makeZip(t, []entry{{name: "../evil", body: "x"}}),
		"backslash climb":      makeZip(t, []entry{{name: "..\\evil", body: "x"}}),
		"absolute (zip)":       makeZip(t, []entry{{name: "/evil", body: "x"}}),
	}
	for name, data := range cases {
		root := filepath.Join(t.TempDir(), "root")
		err := Unpack(context.Background(), write(t, dir, "x.bin", data), root, UnpackOptions{})
		if err == nil {
			t.Errorf("%s: unpacked", name)
		}
		parent := filepath.Dir(root)
		for _, n := range []string{"evil", "outside"} {
			if _, e := os.Lstat(filepath.Join(parent, n)); e == nil {
				t.Errorf("%s: wrote %s outside the folder", name, n)
			}
		}
	}
}

func TestUnpackRefusesTheWrongKindOfFile(t *testing.T) {
	dir := t.TempDir()
	for name, b := range map[string][]byte{"text": []byte("hello, this is not an archive"), "empty": nil, "cut": makeZip(t, []entry{{name: "a", body: strings.Repeat("a", 1000)}})[:30]} {
		if err := Unpack(context.Background(), write(t, dir, "x.bin", b), filepath.Join(dir, "o"), UnpackOptions{}); err == nil {
			t.Errorf("%s: unpacked", name)
		}
	}
	// a .tar.gz cut short is a damaged archive, said so
	full := makeTarGz(t, []entry{{name: "a", body: strings.Repeat("x", 100000)}})
	if err := Unpack(context.Background(), write(t, dir, "y.bin", full[:len(full)/2]), filepath.Join(dir, "o2"), UnpackOptions{}); err == nil {
		t.Error("a cut tar.gz unpacked")
	}
}

func TestUnpackCanBeStopped(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "p.zip", makeZip(t, []entry{{name: "a", body: "x"}}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Unpack(ctx, a, filepath.Join(dir, "o"), UnpackOptions{}); err == nil {
		t.Fatal("a stopped unpacking finished")
	}
}

// --- Install and the recipes ---

func TestResolvePython(t *testing.T) {
	cases := []struct {
		h       Host
		version string
		check   string
		strip   int
	}{
		{Host{OS: "linux", Arch: "amd64"}, "3.12.16", "bin/python3", 1},
		{Host{OS: "linux", Arch: "arm64"}, "3.12.16", "bin/python3", 1},
		{Host{OS: "darwin", Arch: "arm64"}, "3.12.16", "bin/python3", 1},
		{Host{OS: "darwin", Arch: "amd64"}, "3.12.16", "bin/python3", 1},
		{Host{OS: "windows", Arch: "amd64", WinMajor: 10}, "3.12.10", "python.exe", 0},
		{Host{OS: "windows", Arch: "amd64", WinMajor: 6, WinMinor: 3}, "3.12.10", "python.exe", 0}, // 8.1
		{Host{OS: "windows", Arch: "amd64", WinMajor: 6, WinMinor: 1}, "3.8.10", "python.exe", 0},  // 7
		{Host{OS: "windows", Arch: "amd64", WinMajor: 6, WinMinor: 0}, "3.8.10", "python.exe", 0},  // Vista
		{Host{OS: "windows", Arch: "amd64"}, "3.12.10", "python.exe", 0},                           // not known
		{Host{OS: "windows", Arch: "arm64", WinMajor: 11}, "3.12.10", "python.exe", 0},
	}
	for _, c := range cases {
		it, err := Resolve("python", c.h)
		if err != nil {
			t.Errorf("%v: %v", c.h, err)
			continue
		}
		if !strings.Contains(it.Title, c.version) || !strings.Contains(it.URLs[0], c.version) || it.Check != c.check || it.Strip != c.strip {
			t.Errorf("%v: %+v", c.h, it)
		}
		if len(it.SHA256) != 64 || !isHex(it.SHA256) {
			t.Errorf("%v: %q is not a SHA-256", c.h, it.SHA256)
		}
		for _, u := range it.URLs {
			if !strings.HasPrefix(u, "https://") {
				t.Errorf("%v: %s is not https", c.h, u)
			}
		}
		if it.Name != "python" || it.Size == "" || it.From == "" || it.DirName != "python" {
			t.Errorf("%v: %+v is missing what a person is told", c.h, it)
		}
	}
	if _, err := Resolve("python", Host{OS: "freebsd", Arch: "amd64"}); err == nil || !strings.Contains(err.Error(), "freebsd/amd64") {
		t.Errorf("an unsupported computer gave: %v", err)
	}
	if _, err := Resolve("PYTHON", Host{OS: "linux", Arch: "amd64"}); err != nil {
		t.Errorf("the name is case-sensitive: %v", err)
	}
	if _, err := Resolve("perl", Host{OS: "linux", Arch: "amd64"}); err == nil || !strings.Contains(err.Error(), "python") {
		t.Errorf("an unknown name gave: %v", err)
	}
	if a, ok := About("python"); !ok || a == "" {
		t.Error("python has no description")
	}
	if n := Names(); len(n) == 0 || n[0] != "python" {
		t.Errorf("Names = %v", n)
	}
}

func TestInstall(t *testing.T) {
	archive := makeTarGz(t, pythonLike())
	var n int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n++; w.Write(archive) }))
	defer srv.Close()
	it := Item{Title: "Python", Strip: 1, Check: "bin/python3.12", DirName: "python"}
	root := t.TempDir()
	dest := filepath.Join(root, "python")
	var stages []string
	o := InstallOptions{URL: srv.URL, SHA256: sum(archive), Progress: func(s string, d, t int64) {
		if len(stages) == 0 || stages[len(stages)-1] != s {
			stages = append(stages, s)
		}
	}}
	did, err := Install(context.Background(), it, dest, o)
	if err != nil || !did {
		t.Fatalf("Install: %v, %v", did, err)
	}
	if !it.Installed(dest) || len(stages) != 2 || !strings.HasPrefix(stages[0], "Downloading") || !strings.HasPrefix(stages[1], "Unpacking") {
		t.Fatalf("installed %v, stages %v", it.Installed(dest), stages)
	}
	left, _ := filepath.Glob(filepath.Join(root, ".*"))
	if len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
	// already there: nothing downloaded, unless forced
	if did, err := Install(context.Background(), it, dest, o); err != nil || did || n != 1 {
		t.Fatalf("second Install: %v, %v, %d requests", did, err, n)
	}
	o.Force = true
	if did, err := Install(context.Background(), it, dest, o); err != nil || !did || n != 2 {
		t.Fatalf("forced Install: %v, %v, %d requests", did, err, n)
	}
}

func TestInstallFailureLeavesTheOldOneAlone(t *testing.T) {
	good := makeTarGz(t, []entry{{name: "p/bin/python3", body: "new", mode: 0o755}})
	empty := makeTarGz(t, []entry{{name: "p/other", body: "x"}}) // holds no python
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good":
			w.Write(good)
		case "/empty":
			w.Write(empty)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	it := Item{Title: "Python", Strip: 1, Check: "bin/python3"}
	root := t.TempDir()
	dest := filepath.Join(root, "python")
	os.MkdirAll(filepath.Join(dest, "bin"), 0o755)
	os.WriteFile(filepath.Join(dest, "bin", "python3"), []byte("old"), 0o755)

	for name, o := range map[string]InstallOptions{
		"wrong sum":        {URL: srv.URL + "/good", SHA256: sum([]byte("something else")), Force: true},
		"missing":          {URL: srv.URL + "/missing", Force: true},
		"no python inside": {URL: srv.URL + "/empty", Force: true},
	} {
		if _, err := Install(context.Background(), it, dest, o); err == nil {
			t.Errorf("%s: Install succeeded", name)
		}
		if b, _ := os.ReadFile(filepath.Join(dest, "bin", "python3")); string(b) != "old" {
			t.Errorf("%s: the old copy was damaged (%q)", name, b)
		}
		if left, _ := filepath.Glob(filepath.Join(root, ".*")); len(left) != 0 {
			t.Errorf("%s: left behind %v", name, left)
		}
	}
	// and a good one replaces it
	if _, err := Install(context.Background(), it, dest, InstallOptions{URL: srv.URL + "/good", SHA256: sum(good), Force: true}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "bin", "python3")); string(b) != "new" {
		t.Errorf("the new copy reads %q", b)
	}
}
