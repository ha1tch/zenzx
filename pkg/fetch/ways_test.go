// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package fetch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// have is a Looker for a computer that has exactly the programs named, at /fake/<name>.
func have(names ...string) Looker {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return func(name string) string {
		if set[name] {
			return "/fake/" + name
		}
		return ""
	}
}

func ids(ways []Way) string {
	var s []string
	for _, w := range ways {
		s = append(s, w.ID)
	}
	return strings.Join(s, ",")
}

func TestPythonWaysOnlyOffersWhatTheComputerHas(t *testing.T) {
	mac := Host{OS: "darwin", Arch: "arm64"}
	cases := []struct {
		name string
		h    Host
		look Looker
		want string
	}{
		{"a bare Mac", mac, have(), "private,manual"},
		{"a Mac with Homebrew", mac, have("brew"), "private,brew,manual"},
		{"Homebrew and uv", mac, have("brew", "uv"), "private,brew,uv,manual"},
		{"with conda", mac, have("brew", "uv", "conda"), "private,brew,uv,conda,manual"},
		{"mamba when there is no conda", mac, have("mamba"), "private,conda,manual"},
		{"micromamba last", mac, have("micromamba"), "private,conda,manual"},
		{"Linux with Homebrew", Host{OS: "linux", Arch: "amd64"}, have("brew"), "private,brew,manual"},
		{"Windows has no Homebrew", Host{OS: "windows", Arch: "amd64", WinMajor: 10}, have("brew", "uv"), "private,uv,manual"},
		{"a computer with no ready-made copy", Host{OS: "freebsd", Arch: "amd64"}, have("uv"), "uv,manual"},
	}
	for _, c := range cases {
		if got := ids(PythonWays(c.h, c.look)); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestPythonWaysAreDescribedAndHaveOneDefault(t *testing.T) {
	ways := PythonWays(Host{OS: "darwin", Arch: "arm64"}, have("brew", "uv", "conda"))
	defaults := 0
	for _, w := range ways {
		if w.Title == "" || w.Detail == "" {
			t.Errorf("%s has no description", w.ID)
		}
		if w.Default {
			defaults++
			if w.ID != "private" {
				t.Errorf("the default is %s, want the private copy", w.ID)
			}
		}
		switch w.ID {
		case "brew":
			if fmt.Sprint(w.Steps) != "[[/fake/brew install python]]" || w.InDir {
				t.Errorf("brew: %v in dir %v", w.Steps, w.InDir)
			}
		case "uv":
			if !w.InDir || !strings.Contains(fmt.Sprint(w.Steps), "venv --python 3.12 {dir}") {
				t.Errorf("uv: %v", w.Steps)
			}
		case "conda":
			if !w.InDir || !strings.Contains(fmt.Sprint(w.Steps), "create -y -p {dir}") {
				t.Errorf("conda: %v", w.Steps)
			}
		case "manual":
			if !strings.Contains(w.Detail, "python.org/downloads") || len(w.Steps) != 0 {
				t.Errorf("manual: %q %v", w.Detail, w.Steps)
			}
		}
	}
	if defaults != 1 {
		t.Errorf("%d defaults", defaults)
	}
	// On Windows the page that is named is the Windows one
	win := PythonWays(Host{OS: "windows", Arch: "amd64"}, have())
	if last := win[len(win)-1]; !strings.Contains(last.Detail, "downloads/windows") {
		t.Errorf("Windows manual way: %q", last.Detail)
	}
	// The ways are the same for another name that is fetched: one private copy
	if w := WaysFor("python", Host{OS: "linux", Arch: "amd64"}, have()); len(w) != 2 {
		t.Errorf("WaysFor python: %s", ids(w))
	}
	if w := WaysFor("nothing", Host{OS: "linux", Arch: "amd64"}, have()); w != nil {
		t.Errorf("WaysFor an unknown name: %v", w)
	}
}

func TestChoose(t *testing.T) {
	ways := PythonWays(Host{OS: "darwin", Arch: "arm64"}, have("brew", "uv"))
	ask := func(input string) (Way, bool, string) {
		var out bytes.Buffer
		w, ok := Choose(ways, strings.NewReader(input), &out)
		return w, ok, out.String()
	}
	if w, ok, out := ask("\n"); !ok || w.ID != "private" || !strings.Contains(out, "press Return for this one") {
		t.Errorf("Return: %v %v", w.ID, ok)
	}
	if w, ok, _ := ask("2\n"); !ok || w.ID != "brew" {
		t.Errorf("2: %v %v", w.ID, ok)
	}
	if w, ok, _ := ask("4\n"); !ok || w.ID != "manual" {
		t.Errorf("4: %v %v", w.ID, ok)
	}
	for _, q := range []string{"q\n", "Q\n", ""} {
		if _, ok, _ := ask(q); ok {
			t.Errorf("%q chose something", q)
		}
	}
	// a wrong answer is said so and asked again
	if w, ok, out := ask("9\nx\n3\n"); !ok || w.ID != "uv" || strings.Count(out, "not one of the choices") != 2 {
		t.Errorf("wrong answers: %v %v\n%s", w.ID, ok, out)
	}
	// and it does not ask for ever
	if _, ok, _ := ask(strings.Repeat("zz\n", 20)); ok {
		t.Error("a stream of wrong answers chose something")
	}
	// every choice is shown with its number and its explanation
	_, _, out := ask("q\n")
	for i, w := range ways {
		if !strings.Contains(out, string(rune('1'+i))+". "+w.Title) || !strings.Contains(out, w.Detail) {
			t.Errorf("the menu does not show %s:\n%s", w.ID, out)
		}
	}
}

func TestInterpreterByLayout(t *testing.T) {
	dir := t.TempDir()
	put := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o755)
	}
	unix, win := Host{OS: "linux"}, Host{OS: "windows"}
	if Interpreter(dir, unix) != "" || Interpreter(dir, win) != "" {
		t.Fatal("found an interpreter in an empty folder")
	}
	put("bin/python")
	if got := Interpreter(dir, unix); !strings.HasSuffix(filepath.ToSlash(got), "bin/python") {
		t.Errorf("bin/python: %q", got)
	}
	put("bin/python3")
	if got := Interpreter(dir, unix); !strings.HasSuffix(filepath.ToSlash(got), "bin/python3") {
		t.Errorf("bin/python3 comes first: %q", got)
	}
	put("Scripts/python.exe")
	if got := Interpreter(dir, win); !strings.HasSuffix(filepath.ToSlash(got), "Scripts/python.exe") {
		t.Errorf("a virtual environment on Windows: %q", got)
	}
	put("python.exe")
	if got := Interpreter(dir, win); !strings.HasSuffix(filepath.ToSlash(got), "/python.exe") || strings.Contains(got, "Scripts") {
		t.Errorf("an embeddable copy: %q", got)
	}
}

// fakeTool writes a shell script named name that runs body, and returns its path.
func fakeTool(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunWayMakesAnEnvironmentInTheFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake tools are shell scripts")
	}
	// a stand-in for "uv venv --python 3.12 DIR": it makes DIR/bin/python3
	uv := fakeTool(t, "uv", `[ "$1" = venv ] && [ "$2" = --python ] && [ "$3" = 3.12 ] && mkdir -p "$4/bin" && : > "$4/bin/python3" && chmod +x "$4/bin/python3"`)
	h := Host{OS: runtime.GOOS, Arch: runtime.GOARCH}
	w, _ := FindWay(PythonWays(h, func(n string) string {
		if n == "uv" {
			return uv
		}
		return ""
	}), "uv")
	dir := filepath.Join(t.TempDir(), "examples", "python")
	var out bytes.Buffer
	p, err := RunWay(context.Background(), h, w, dir, HostLook, &out)
	if err != nil || p != filepath.Join(dir, "bin", "python3") {
		t.Fatalf("RunWay: %q, %v\n%s", p, err, out.String())
	}
	if !strings.Contains(out.String(), "Running: "+uv+" venv --python 3.12 "+dir) {
		t.Errorf("the command was not said:\n%s", out.String())
	}
	// a leftover folder with no Python in it does not stop the tool
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "junk"), []byte("x"), 0o644)
	if p, err := RunWay(context.Background(), h, w, dir, HostLook, &out); err != nil || p == "" {
		t.Fatalf("with a leftover folder: %q, %v", p, err)
	}
}

func TestRunWayFailuresSayWhy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake tools are shell scripts")
	}
	h := Host{OS: runtime.GOOS, Arch: runtime.GOARCH}
	failing := fakeTool(t, "uv", `echo "no network" >&2; exit 3`)
	empty := fakeTool(t, "uv", `exit 0`) // finishes, makes nothing
	for name, tool := range map[string]string{"fails": failing, "makes nothing": empty} {
		w := Way{ID: "uv", Steps: [][]string{{tool, "venv", "{dir}"}}, InDir: true}
		var out bytes.Buffer
		_, err := RunWay(context.Background(), h, w, filepath.Join(t.TempDir(), "p"), HostLook, &out)
		if err == nil {
			t.Errorf("%s: succeeded", name)
			continue
		}
		if name == "makes nothing" && !strings.Contains(err.Error(), "no Python in") {
			t.Errorf("%s: %v", name, err)
		}
		if name == "fails" && (!strings.Contains(err.Error(), "did not finish") || !strings.Contains(out.String(), "no network")) {
			t.Errorf("%s: %v / %q", name, err, out.String())
		}
	}
	if _, err := RunWay(context.Background(), h, Way{ID: "manual"}, "x", HostLook, &bytes.Buffer{}); err == nil {
		t.Error("a way with nothing to run ran")
	}
}

func TestRunWayHomebrewFindsItsPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake tools are shell scripts")
	}
	prefix := t.TempDir()
	os.MkdirAll(filepath.Join(prefix, "bin"), 0o755)
	os.WriteFile(filepath.Join(prefix, "bin", "python3"), []byte("x"), 0o755)
	brew := fakeTool(t, "brew", `[ "$1" = --prefix ] && { echo "`+prefix+`"; exit 0; }; [ "$1" = install ] && [ "$2" = python ]`)
	h := Host{OS: "darwin", Arch: "arm64"}
	w, _ := FindWay(PythonWays(h, func(n string) string {
		if n == "brew" {
			return brew
		}
		return ""
	}), "brew")
	p, err := RunWay(context.Background(), h, w, "unused", HostLook, &bytes.Buffer{})
	if err != nil || p != filepath.Join(prefix, "bin", "python3") {
		t.Fatalf("RunWay brew: %q, %v", p, err)
	}
}

// asRoot makes systemWay see the administrator, or somebody else, for the length of a test.
func asRoot(t *testing.T, root bool) {
	t.Helper()
	was := euid
	t.Cleanup(func() { euid = was })
	if root {
		euid = func() int { return 0 }
	} else {
		euid = func() int { return 1000 }
	}
}

func TestSystemPackageWays(t *testing.T) {
	linux := Host{OS: "linux", Arch: "amd64"}
	cases := []struct {
		name string
		h    Host
		root bool
		look Looker
		want string // the ways, and for the system's own the steps
		step string
	}{
		{"apt as the administrator", linux, true, have("apt-get"), "private,apt,manual", "[[/fake/apt-get update] [/fake/apt-get install -y python3]]"},
		{"apt beats dnf where both are", linux, true, have("apt-get", "dnf", "yum"), "private,apt,manual", ""},
		{"dnf", linux, true, have("dnf"), "private,dnf,manual", "[[/fake/dnf install -y python3]]"},
		{"dnf beats yum", linux, true, have("dnf", "yum"), "private,dnf,manual", ""},
		{"yum on an older Red Hat", linux, true, have("yum"), "private,yum,manual", "[[/fake/yum install -y python3]]"},
		{"sudo asks for the password", linux, false, have("apt-get", "sudo"), "private,apt,manual", "[[/fake/sudo /fake/apt-get update] [/fake/sudo /fake/apt-get install -y python3]]"},
		{"dnf by sudo", linux, false, have("dnf", "sudo"), "private,dnf,manual", "[[/fake/sudo /fake/dnf install -y python3]]"},
		{"nobody to install it", linux, false, have("apt-get"), "private,manual", ""},
		{"Homebrew and apt both", linux, true, have("brew", "apt-get"), "private,brew,apt,manual", ""},
		{"a Mac has no apt", Host{OS: "darwin", Arch: "arm64"}, true, have("apt-get", "dnf"), "private,manual", ""},
		{"Windows has no apt", Host{OS: "windows", Arch: "amd64", WinMajor: 10}, true, have("apt-get"), "private,manual", ""},
		{"FreeBSD has no apt", Host{OS: "freebsd", Arch: "amd64"}, true, have("apt-get"), "manual", ""},
		{"apt next to uv", linux, true, have("apt-get", "uv"), "private,apt,uv,manual", ""},
	}
	for _, c := range cases {
		asRoot(t, c.root)
		ways := PythonWays(c.h, c.look)
		if got := ids(ways); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
			continue
		}
		for _, id := range []string{"apt", "dnf", "yum"} {
			w, ok := FindWay(ways, id)
			if !ok {
				continue
			}
			if w.InDir || w.Default || w.Detail == "" || w.Title == "" {
				t.Errorf("%s: %+v", c.name, w)
			}
			if c.step != "" && fmt.Sprint(w.Steps) != c.step {
				t.Errorf("%s: steps %v, want %s", c.name, w.Steps, c.step)
			}
			if !c.root != strings.Contains(w.Detail, "password") {
				t.Errorf("%s: the password is said (or not) wrongly: %q", c.name, w.Detail)
			}
		}
	}
}

func TestRunWaySystemPackage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake tools are shell scripts")
	}
	for _, root := range []bool{true, false} {
		asRoot(t, root)
		work := t.TempDir()
		log := filepath.Join(work, "log")
		python := fakeTool(t, "python3", `exit 0`)
		// stand-ins that write down what they were asked, and finish
		apt := fakeTool(t, "apt-get", `echo "apt-get $*" >> `+log)
		sudo := fakeTool(t, "sudo", `echo "sudo" >> `+log+`; exec "$@"`)
		look := func(n string) string {
			switch n {
			case "apt-get":
				return apt
			case "sudo":
				return sudo
			case "python3":
				return python
			}
			return ""
		}
		h := Host{OS: "linux", Arch: "amd64"}
		w, ok := FindWay(PythonWays(h, look), "apt")
		if !ok {
			t.Fatalf("root %v: no apt way", root)
		}
		var out bytes.Buffer
		p, err := RunWay(context.Background(), h, w, "unused", look, &out)
		if err != nil || p != python {
			t.Fatalf("root %v: RunWay: %q, %v\n%s", root, p, err, out.String())
		}
		got, _ := os.ReadFile(log)
		want := "apt-get update\napt-get install -y python3\n"
		if !root {
			want = "sudo\napt-get update\nsudo\napt-get install -y python3\n"
		}
		if string(got) != want {
			t.Errorf("root %v: ran\n%s\nwant\n%s", root, got, want)
		}
		if !strings.Contains(out.String(), "Running: ") {
			t.Errorf("root %v: the commands were not said:\n%s", root, out.String())
		}
	}
}

func TestRunWaySystemPackageThatLeavesNoPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake tools are shell scripts")
	}
	asRoot(t, true)
	dnf := fakeTool(t, "dnf", `exit 0`)
	failing := fakeTool(t, "dnf", `echo "no space left" >&2; exit 1`)
	h := Host{OS: "linux", Arch: "amd64"}
	for name, tool := range map[string]string{"finishes, nothing there": dnf, "fails": failing} {
		look := func(n string) string {
			if n == "dnf" {
				return tool
			}
			return ""
		}
		w, _ := FindWay(PythonWays(h, look), "dnf")
		var out bytes.Buffer
		_, err := RunWay(context.Background(), h, w, "unused", look, &out)
		if err == nil {
			t.Errorf("%s: succeeded", name)
		} else if name == "fails" && (!strings.Contains(err.Error(), "did not finish") || !strings.Contains(out.String(), "no space left")) {
			t.Errorf("%s: %v / %q", name, err, out.String())
		} else if name != "fails" && !strings.Contains(err.Error(), "python3 is not to be found") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestHostLookFindsWhatIsOnThePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script")
	}
	tool := fakeTool(t, "zenzx-test-tool", "exit 0")
	t.Setenv("PATH", filepath.Dir(tool)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got := HostLook("zenzx-test-tool"); got != tool {
		t.Errorf("HostLook = %q, want %q", got, tool)
	}
	if got := HostLook("zenzx-no-such-tool"); got != "" {
		t.Errorf("HostLook found %q", got)
	}
}
