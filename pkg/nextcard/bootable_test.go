package nextcard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// systemCard makes a card holding BootFiles, each with its own name as its
// contents but config.ini, which says what VerifyCard looks for; without
// names the listed files are left out.
func systemCard(t *testing.T, path string, without ...string) {
	t.Helper()
	root := NewDir("")
	for _, p := range append(append([]string{}, BootFiles...), BootExtras...) {
		skip := false
		for _, w := range without {
			skip = skip || w == p
		}
		if skip {
			continue
		}
		dir := root
		parts := strings.Split(p, "/")
		for _, d := range parts[:len(parts)-1] {
			if next := dir.Find(d); next != nil {
				dir = next
			} else {
				dir = dir.AddDir(d)
			}
		}
		data := []byte("contents of " + p)
		if strings.HasSuffix(p, "config.ini") {
			data = []byte("scandoubler=1\r\ntiming=0\r\n")
		}
		dir.Add(parts[len(parts)-1], data)
	}
	if _, err := NewCard(path, SmallestCard, root); err != nil {
		t.Fatal(err)
	}
}

func TestMakeBootable(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "next-card.img")
	systemCard(t, source)

	// A card with the user's own files and an old config.ini.
	target := filepath.Join(dir, "mine.img")
	root := NewDir("")
	root.AddDir("games").Add("mine.bas", []byte("10 PRINT 1"))
	root.AddDir("machines").AddDir("next").Add("CONFIG.INI", []byte("timing=7\r\n"))
	if _, err := NewCard(target, SmallestCard, root); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(target)
	if err := VerifyCard(f); err == nil {
		t.Fatal("the card is bootable before it is made so")
	}
	f.Close()

	var last, total int64
	n, err := MakeBootable(context.Background(), source, target, func(done, all int64) { last, total = done, all })
	if err != nil {
		t.Fatal(err)
	}
	if n <= 0 || total == 0 || last != total {
		t.Errorf("%d bytes copied; progress ended at %d of %d", n, last, total)
	}
	f, err = os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := VerifyCard(f); err != nil {
		t.Fatalf("the card is not bootable afterwards: %v", err)
	}
	vol, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range BootFiles {
		node := vol.Root.Find(p)
		if node == nil {
			t.Errorf("%s is not on the card", p)
			continue
		}
		data, _ := vol.ReadFile(node)
		want := []byte("contents of " + p)
		if strings.HasSuffix(p, "config.ini") {
			want = []byte("scandoubler=1\r\ntiming=0\r\n")
		}
		if !bytes.Equal(data, want) {
			t.Errorf("%s holds %q", p, data)
		}
	}
	for _, p := range BootExtras {
		if vol.Root.Find(p) != nil {
			t.Errorf("%s was copied; only the files that are needed should be", p)
		}
	}
	if node := vol.Root.Find("games/mine.bas"); node == nil {
		t.Error("the user's file is gone")
	} else if data, _ := vol.ReadFile(node); string(data) != "10 PRINT 1" {
		t.Errorf("the user's file now holds %q", data)
	}
	// The old config.ini was replaced, not joined by a second one.
	count := 0
	for _, c := range vol.Root.Find("machines/next").Children {
		if strings.EqualFold(c.Name, "config.ini") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("%d files called config.ini in machines/next", count)
	}
}

func TestMakeBootableRefusals(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "next-card.img")
	systemCard(t, source)
	empty := filepath.Join(dir, "empty.img")
	if _, err := NewCard(empty, SmallestCard, NewDir("")); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := MakeBootable(ctx, source, source, nil); err == nil || !strings.Contains(err.Error(), "copied from") {
		t.Errorf("the source as its own target gave %v", err)
	}
	// A source that is not a system card.
	if _, err := MakeBootable(ctx, empty, source, nil); err == nil || !strings.Contains(err.Error(), "not usable") {
		t.Errorf("an empty card as the source gave %v", err)
	}
	// A source that passes as a system card but lacks one of the files.
	partial := filepath.Join(dir, "partial.img")
	systemCard(t, partial, "nextzxos/enSystem.sys")
	if _, err := MakeBootable(ctx, partial, empty, nil); err == nil || !strings.Contains(err.Error(), "has no nextzxos/enSystem.sys") {
		t.Errorf("a source without enSystem.sys gave %v", err)
	}
	// A target that is not a card: left as it was.
	junk := filepath.Join(dir, "junk.img")
	if err := os.WriteFile(junk, []byte("not a card"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := MakeBootable(ctx, source, junk, nil); err == nil {
		t.Error("a file that is not a card was made bootable")
	}
	if data, _ := os.ReadFile(junk); string(data) != "not a card" {
		t.Error("the file that is not a card was changed")
	}
	// Stopped part way: the card is as it was.
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	before, _ := os.ReadFile(empty)
	if _, err := MakeBootable(stopped, source, empty, nil); err == nil {
		t.Error("a stopped run reported success")
	}
	after, _ := os.ReadFile(empty)
	if !bytes.Equal(before, after) {
		t.Error("a stopped run changed the card")
	}
	if _, err := os.Stat(empty + ".part"); err == nil {
		t.Error("a stopped run left its working file behind")
	}
}

func TestAddTreePutsFilesInTheirFolders(t *testing.T) {
	card := filepath.Join(t.TempDir(), "c.img")
	root := NewDir("")
	root.AddDir("Docs").Add("old.txt", []byte("old"))
	if _, err := NewCard(card, SmallestCard, root); err != nil {
		t.Fatal(err)
	}
	err := AddTree(context.Background(), card, []File{
		{Name: "top.txt", Data: []byte("1")},
		{Name: "/docs/new.txt", Data: []byte("2")}, // an existing folder, whatever its case
		{Name: "a/b/c/deep.txt", Data: []byte("3")},
		{Name: "docs/old.txt", Data: []byte("4")}, // replaces
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(card)
	defer f.Close()
	vol, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"top.txt": "1", "Docs/new.txt": "2", "a/b/c/deep.txt": "3", "Docs/old.txt": "4"} {
		node := vol.Root.Find(path)
		if node == nil {
			t.Errorf("%s is not on the card", path)
			continue
		}
		if data, _ := vol.ReadFile(node); string(data) != want {
			t.Errorf("%s holds %q, want %q", path, data, want)
		}
	}
	folders := 0
	for _, c := range vol.Root.Children {
		if strings.EqualFold(c.Name, "docs") {
			folders++
		}
	}
	if folders != 1 {
		t.Errorf("%d folders called docs", folders)
	}
	if err := AddTree(context.Background(), card, []File{{Name: "docs/old.txt/x", Data: nil}}, nil); err == nil {
		t.Error("a file where a folder would go was accepted")
	}
}
