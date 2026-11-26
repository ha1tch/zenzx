// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package fetch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Way is one way of getting Python onto a computer that has none. Which ways are offered
// depends on what the computer has: a private copy can always be downloaded, Homebrew is
// offered only where "brew" is, the system's own package only where its manager is (and the
// administrator, or sudo, to use it), and a separate environment only where a tool that
// makes them (uv, conda) is.
type Way struct {
	ID      string     // what -fetch-how takes: "private", "brew", "apt", "dnf", "yum", "uv", "conda", "manual"
	Title   string     // one line, for a person
	Detail  string     // what it does and what it leaves, in a sentence or two
	Steps   [][]string // what to run, in order ("{dir}" in an argument is the folder); none for a way that runs nothing
	InDir   bool       // the Python ends up in the folder asked for, rather than on the whole computer
	Default bool       // what pressing Return takes
}

// Looker finds a program: its path, or "" when the computer has none by that name.
type Looker func(name string) string

// extraDirs are where programs that people install for themselves put them, which a program
// started from a double-click, with a short PATH, would not see.
func extraDirs() []string {
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin", "/home/linuxbrew/.linuxbrew/bin"}
	if home, err := os.UserHomeDir(); err == nil {
		for _, d := range []string{".local/bin", ".cargo/bin", "miniconda3/bin", "anaconda3/bin", "miniforge3/bin", "mambaforge/bin", "micromamba/bin", ".pixi/bin"} {
			dirs = append(dirs, filepath.Join(home, d))
		}
	}
	return dirs
}

// HostLook finds a program on the PATH, and then where such programs are usually put.
func HostLook(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, d := range extraDirs() {
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

// pythonPage is the page a person is sent to when they would rather install Python themselves.
func pythonPage(h Host) string {
	if h.OS == "windows" {
		return "https://www.python.org/downloads/windows/"
	}
	return "https://www.python.org/downloads/"
}

// PythonWays lists the ways of getting Python on host h, given what look finds there. The
// private copy comes first and is what Return takes; installing it yourself is always last.
func PythonWays(h Host, look Looker) []Way {
	var ways []Way
	if it, err := Resolve("python", h); err == nil {
		ways = append(ways, Way{
			ID:      "private",
			Title:   fmt.Sprintf("A private copy of %s (%s, from %s)", it.Title, it.Size, it.From),
			Detail:  "Nothing is installed: the copy goes into one folder, and deleting that folder removes it.",
			InDir:   true,
			Default: true,
		})
	}
	if h.OS != "windows" {
		if brew := look("brew"); brew != "" {
			ways = append(ways, Way{
				ID:     "brew",
				Title:  "Install it with Homebrew (brew install python)",
				Detail: "Homebrew is on this computer. Python then belongs to the whole computer, and Homebrew keeps it up to date.",
				Steps:  [][]string{{brew, "install", "python"}},
			})
		}
	}
	if w, ok := systemWay(h, look); ok {
		ways = append(ways, w)
	}
	if uv := look("uv"); uv != "" {
		ways = append(ways, Way{
			ID:     "uv",
			Title:  "A separate environment, made with uv",
			Detail: "uv is on this computer. It makes a Python environment in the folder, fetching Python 3.12 for it, and nothing outside the folder changes.",
			Steps:  [][]string{{uv, "venv", "--python", "3.12", "{dir}"}},
			InDir:  true,
		})
	}
	for _, tool := range []string{"conda", "mamba", "micromamba"} {
		if p := look(tool); p != "" {
			ways = append(ways, Way{
				ID:     "conda",
				Title:  "A separate environment, made with " + tool,
				Detail: tool + " is on this computer. It makes a Python 3.12 environment in the folder, and nothing outside the folder changes.",
				Steps:  [][]string{{p, "create", "-y", "-p", "{dir}", "-c", "conda-forge", "--override-channels", "python=3.12"}},
				InDir:  true,
			})
			break
		}
	}
	ways = append(ways, Way{
		ID:     "manual",
		Title:  "Install it yourself, from python.org",
		Detail: "Nothing is done here: " + pythonPage(h) + " has the installer, and the examples find Python by themselves afterwards.",
	})
	return ways
}

// WaysFor lists the ways of getting what name is, on host h. Only Python has more than one;
// anything else that can be fetched is a private copy, in the folder asked for.
func WaysFor(name string, h Host, look Looker) []Way {
	if strings.EqualFold(name, "python") {
		return PythonWays(h, look)
	}
	it, err := Resolve(name, h)
	if err != nil {
		return nil
	}
	return []Way{{ID: "private", Title: "A copy of " + it.Title + " (" + it.Size + ")", Detail: "It goes into one folder; deleting that folder removes it.", InDir: true, Default: true}}
}

// euid is who the program runs as; a variable so that a test can be root.
var euid = os.Geteuid

// systemWay is the way that uses the computer's own package manager, on Linux: apt on Debian and
// Ubuntu, dnf (or, on an older one, yum) on Fedora and Red Hat. Installing a package takes
// the administrator, so it is offered only to the administrator, or where sudo is there to ask
// for the password.
func systemWay(h Host, look Looker) (Way, bool) {
	if h.OS != "linux" {
		return Way{}, false
	}
	var id, name, pkg string
	var install [][]string
	switch {
	case look("apt-get") != "":
		id, name, pkg = "apt", "apt", "Debian, Ubuntu and their relatives"
		install = [][]string{{look("apt-get"), "update"}, {look("apt-get"), "install", "-y", "python3"}}
	case look("dnf") != "":
		id, name, pkg = "dnf", "dnf", "Fedora, Red Hat and their relatives"
		install = [][]string{{look("dnf"), "install", "-y", "python3"}}
	case look("yum") != "":
		id, name, pkg = "yum", "yum", "Red Hat and its relatives"
		install = [][]string{{look("yum"), "install", "-y", "python3"}}
	default:
		return Way{}, false
	}
	detail := "The package of " + pkg + ", installed with " + name + ". Python then belongs to the whole computer, and the system keeps it up to date."
	if euid() != 0 {
		sudo := look("sudo")
		if sudo == "" {
			return Way{}, false
		}
		for i := range install {
			install[i] = append([]string{sudo}, install[i]...)
		}
		detail += " It asks for your password, as sudo does."
	}
	return Way{ID: id, Title: "Install it with the system's package manager (" + name + ")", Detail: detail, Steps: install}, true
}

// FindWay is the way with the given ID, or false.
func FindWay(ways []Way, id string) (Way, bool) {
	for _, w := range ways {
		if strings.EqualFold(w.ID, id) {
			return w, true
		}
	}
	return Way{}, false
}

// Choose puts the ways to a person and reads which they want: a number, Return for the one
// that is the default, or Q (or the end of the input) for none. ok is false for none.
func Choose(ways []Way, in io.Reader, out io.Writer) (w Way, ok bool) {
	def := 0
	for i, x := range ways {
		if x.Default {
			def = i
		}
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  How would you like to get it?")
	fmt.Fprintln(out)
	for i, x := range ways {
		mark := ""
		if i == def {
			mark = "   (press Return for this one)"
		}
		fmt.Fprintf(out, "    %d. %s%s\n       %s\n\n", i+1, x.Title, mark, x.Detail)
	}
	r := bufio.NewReader(in)
	for tries := 0; tries < 5; tries++ {
		fmt.Fprintf(out, "  Type a number and press Return, or Q to quit: ")
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)
		switch {
		case line == "" && err == nil:
			return ways[def], true
		case line == "" || strings.EqualFold(line, "q"):
			return Way{}, false
		}
		if n, e := strconv.Atoi(line); e == nil && n >= 1 && n <= len(ways) {
			return ways[n-1], true
		}
		fmt.Fprintf(out, "  That is not one of the choices.\n")
		if err != nil {
			return Way{}, false
		}
	}
	return Way{}, false
}

// Interpreter is the python program that is in dir, in the layout of a private copy, a
// virtual environment or a conda environment, or "".
func Interpreter(dir string, h Host) string {
	rel := []string{"bin/python3", "bin/python"}
	if h.OS == "windows" {
		rel = []string{"python.exe", "Scripts/python.exe"}
	}
	for _, r := range rel {
		p := filepath.Join(dir, filepath.FromSlash(r))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// RunWay does what a way that runs programs says, in order, with their own output going to
// out, and returns the python it made: in dir for the ways that put it there, and for the ones
// that install it on the computer, the python3 that is there afterwards. It is not for
// "private" (Install is) or "manual", which make nothing.
func RunWay(ctx context.Context, h Host, w Way, dir string, look Looker, out io.Writer) (string, error) {
	if len(w.Steps) == 0 {
		return "", fmt.Errorf("%q is not something to run", w.ID)
	}
	if w.InDir {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return "", err
		}
		if st, err := os.Stat(dir); err == nil && st.IsDir() && Interpreter(dir, h) == "" {
			os.RemoveAll(dir) // a folder that holds no Python: the tools want to make it themselves
		}
	}
	for _, step := range w.Steps {
		argv := make([]string, len(step))
		for i, a := range step {
			argv[i] = strings.ReplaceAll(a, "{dir}", dir)
		}
		fmt.Fprintf(out, "Running: %s\n", strings.Join(argv, " "))
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Stdout, cmd.Stderr, cmd.Stdin = out, out, os.Stdin
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return "", &Error{Kind: Stopped, Err: ctx.Err()}
			}
			return "", fmt.Errorf("%s did not finish: %v", filepath.Base(argv[0]), err)
		}
	}
	if w.InDir {
		if p := Interpreter(dir, h); p != "" {
			return p, nil
		}
		return "", fmt.Errorf("%s finished, but there is no Python in %s", filepath.Base(w.Steps[len(w.Steps)-1][0]), dir)
	}
	// Homebrew keeps the python3 it has just installed in its own prefix, which is not always on the PATH
	if w.ID == "brew" {
		if prefix, err := exec.CommandContext(ctx, w.Steps[0][0], "--prefix").Output(); err == nil {
			p := filepath.Join(strings.TrimSpace(string(prefix)), "bin", "python3")
			if st, e := os.Stat(p); e == nil && !st.IsDir() {
				return p, nil
			}
		}
	}
	if p := look("python3"); p != "" {
		return p, nil
	}
	return "", fmt.Errorf("%s finished, but python3 is not to be found afterwards", w.ID)
}
