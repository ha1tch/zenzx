// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package nextcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func fd(path, content string) FileDigest {
	sum := sha256.Sum256([]byte(content))
	return FileDigest{Path: path, Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}
}

func firmwareFiles() []FileDigest {
	return []FileDigest{
		fd("TBBLUE.FW", "firmware"), fd("machines/next/config.ini", "timing=0"),
		fd("machines/next/menu.def", "menu"), fd("machines/next/enNextZX.rom", "roms"),
		fd("nextzxos/enSystem.sys", "system"),
	}
}

func digestOf(t *testing.T, files []FileDigest) string {
	t.Helper()
	d, _, err := Combine(files)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDigestDoesNotDependOnTheOrderTheFilesAreMet(t *testing.T) {
	files := firmwareFiles()
	want := digestOf(t, files)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		shuffled := append([]FileDigest(nil), files...)
		r.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		if got := digestOf(t, shuffled); got != want {
			t.Fatalf("shuffle %d: %s, want %s", i, got, want)
		}
	}
}

func TestDigestDoesNotDependOnTheLetterCaseOrTheSlashOfAPath(t *testing.T) {
	files := firmwareFiles()
	want := digestOf(t, files)
	var other []FileDigest
	for _, f := range files {
		f.Path = "\\" + f.Path
		other = append(other, f)
	}
	other[0].Path = "tbblue.fw"
	other[1].Path = "MACHINES\\NEXT\\CONFIG.INI"
	if got := digestOf(t, other); got != want {
		t.Fatal("case or separator reaches the digest")
	}
}

func TestDigestChangesWithAnythingThatMatters(t *testing.T) {
	files := firmwareFiles()
	base := digestOf(t, files)
	mutate := func(name string, f func([]FileDigest) []FileDigest) {
		t.Helper()
		got := digestOf(t, f(append([]FileDigest(nil), files...)))
		if got == base {
			t.Errorf("%s: the digest did not change", name)
		}
	}
	mutate("one byte of one file", func(fs []FileDigest) []FileDigest { fs[0] = fd("TBBLUE.FW", "firmwarE"); return fs })
	mutate("a file one byte longer", func(fs []FileDigest) []FileDigest { fs[0] = fd("TBBLUE.FW", "firmware!"); return fs })
	mutate("two files swapping contents", func(fs []FileDigest) []FileDigest {
		fs[0].SHA256, fs[1].SHA256 = fs[1].SHA256, fs[0].SHA256
		fs[0].Size, fs[1].Size = fs[1].Size, fs[0].Size
		return fs
	})
	mutate("a file renamed", func(fs []FileDigest) []FileDigest { fs[2].Path = "machines/next/menu2.def"; return fs })
	mutate("a file moved", func(fs []FileDigest) []FileDigest { fs[2].Path = "menu.def"; return fs })
	mutate("a file left out", func(fs []FileDigest) []FileDigest { return fs[1:] })
	mutate("a file added", func(fs []FileDigest) []FileDigest { return append(fs, fd("extra.bin", "x")) })
}

func TestDigestCannotBeConfusedByWhereOnePathEndsAndTheNextBegins(t *testing.T) {
	a := []FileDigest{fd("AB", "x"), fd("C", "y")}
	b := []FileDigest{fd("A", "x"), fd("BC", "y")}
	if digestOf(t, a) == digestOf(t, b) {
		t.Fatal("two different sets of files give one digest")
	}
}

func TestDigestRefusesAFileListedTwiceOrWithoutADigest(t *testing.T) {
	if _, _, err := Combine([]FileDigest{fd("A", "1"), fd("a", "2")}); err == nil {
		t.Fatal("a file listed twice, in two cases, was accepted")
	}
	if _, _, err := Combine([]FileDigest{{Path: "A", Size: 1}}); err == nil {
		t.Fatal("a file with no SHA-256 was accepted")
	}
}

// digestFixture pins the definition: if this changes, digestTag must change with it.
func TestDigestDefinitionIsPinned(t *testing.T) {
	d := digestOf(t, []FileDigest{fd("B", "2"), fd("A", "1")})
	const want = "pinned in TestDigestDefinitionIsPinned"
	_ = want
	if len(d) != 64 {
		t.Fatalf("digest %q", d)
	}
	// Computed by hand from the definition: tag, then the records of A and of B.
	h := sha256.New()
	h.Write([]byte("zenzx-next-firmware-v1"))
	for _, f := range []struct{ p, c string }{{"A", "1"}, {"B", "2"}} {
		h.Write([]byte{0, 0, 0, 1})
		h.Write([]byte(f.p))
		h.Write([]byte{0, 0, 0, 0, 0, 0, 0, 1})
		s := sha256.Sum256([]byte(f.c))
		h.Write(s[:])
	}
	if hex.EncodeToString(h.Sum(nil)) != d {
		t.Fatal("the digest is not the one the definition gives")
	}
}

func testKey(t *testing.T) Key {
	files := firmwareFiles()
	d, sorted, err := Combine(files)
	if err != nil {
		t.Fatal(err)
	}
	return Key{BootROM: "b1", DivMMCROM: "d1", Firmware: Firmware{Digest: d, Files: sorted}, Issue: "2", Version: "0.50.0", Layout: "l1"}
}

func TestWriteThenReadReturnsTheState(t *testing.T) {
	dir := t.TempDir()
	k := testKey(t)
	state := bytes.Repeat([]byte{1, 2, 3, 4}, 1000)
	if err := Write(dir, Entry{Key: k, Stage: HandOver, Frames: 428, State: state}); err != nil {
		t.Fatal(err)
	}
	e, err := Read(dir, k, HandOver)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(e.State, state) || e.Frames != 428 || e.Stage != HandOver {
		t.Fatalf("got %+v", e)
	}
	if _, err := Read(dir, k, Settled); !errors.Is(err, ErrAbsent) {
		t.Fatalf("another stage: %v", err)
	}
}

func TestEachPartOfTheKeyInvalidatesAndIsNamed(t *testing.T) {
	dir := t.TempDir()
	k := testKey(t)
	if err := Write(dir, Entry{Key: k, Stage: HandOver, State: []byte("s")}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(*Key)
		want string
	}{
		{"boot ROM", func(k *Key) { k.BootROM = "x" }, "the boot ROM"},
		{"DivMMC ROM", func(k *Key) { k.DivMMCROM = "x" }, "the DivMMC ROM"},
		{"board issue", func(k *Key) { k.Issue = "5" }, "the board issue"},
		{"patches", func(k *Key) { k.Patches = []string{"p"} }, "the ROM patches"},
		{"version", func(k *Key) { k.Version = "0.51.0" }, "the ZenZX version"},
		{"layout", func(k *Key) { k.Layout = "l2" }, "the layout of the machine's state"},
	}
	for _, c := range cases {
		q := k
		c.edit(&q)
		_, err := Read(dir, q, HandOver)
		var se *StaleError
		if !errors.As(err, &se) || len(se.Differs) != 1 || se.Differs[0] != c.want {
			t.Errorf("%s: got %v, want stale naming %q", c.name, err, c.want)
		}
	}
}

func TestAChangedFirmwareFileIsAnotherCacheAndIsNamedWhenCompared(t *testing.T) {
	dir := t.TempDir()
	k := testKey(t)
	if err := Write(dir, Entry{Key: k, Stage: HandOver, State: []byte("s")}); err != nil {
		t.Fatal(err)
	}
	files := firmwareFiles()
	files[1] = fd("machines/next/config.ini", "timing=1")
	d, sorted, _ := Combine(files)
	q := k
	q.Firmware = Firmware{Digest: d, Files: sorted}
	if _, err := Read(dir, q, HandOver); !errors.Is(err, ErrAbsent) {
		t.Fatalf("a changed firmware has no cache: %v", err)
	}
	diff := k.Diff(q)
	if len(diff) != 1 || diff[0] != "firmware file MACHINES/NEXT/CONFIG.INI" {
		t.Fatalf("got %v", diff)
	}
	infos := List(dir)
	if len(infos) != 1 || infos[0].Key.Diff(q)[0] != diff[0] {
		t.Fatalf("a listing does not let check say what differs: %+v", infos)
	}
}

func TestADamagedFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	k := testKey(t)
	if err := Write(dir, Entry{Key: k, Stage: Settled, State: bytes.Repeat([]byte{7}, 500)}); err != nil {
		t.Fatal(err)
	}
	path := Path(dir, k.Firmware, Settled)
	orig, _ := os.ReadFile(path)
	for name, mod := range map[string][]byte{
		"a flipped byte in the state": append(append([]byte(nil), orig[:len(orig)-10]...), append([]byte{orig[len(orig)-10] ^ 1}, orig[len(orig)-9:]...)...),
		"cut short":                   orig[:len(orig)-1],
		"extended":                    append(append([]byte(nil), orig...), 0),
		"not a cache":                 []byte("hello"),
		"empty":                       nil,
	} {
		os.WriteFile(path, mod, 0o644)
		if _, err := Read(dir, k, Settled); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: got %v, want ErrCorrupt", name, err)
		}
	}
}

func TestWriteLeavesNothingBehindAndReplacesAtOnce(t *testing.T) {
	dir := t.TempDir()
	k := testKey(t)
	for i := 0; i < 3; i++ {
		if err := Write(dir, Entry{Key: k, Stage: HandOver, State: []byte{byte(i)}}); err != nil {
			t.Fatal(err)
		}
	}
	names, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(names) != 1 {
		t.Fatalf("files: %v", names)
	}
	e, _ := Read(dir, k, HandOver)
	if e.State[0] != 2 {
		t.Fatal("not replaced")
	}
}

func TestListPruneAndClear(t *testing.T) {
	dir := t.TempDir()
	k := testKey(t)
	Write(dir, Entry{Key: k, Stage: HandOver, State: []byte("a")})
	Write(dir, Entry{Key: k, Stage: Settled, State: []byte("b")})
	files := firmwareFiles()
	files[0] = fd("TBBLUE.FW", "other")
	d, sorted, _ := Combine(files)
	k2 := k
	k2.Firmware = Firmware{Digest: d, Files: sorted}
	Write(dir, Entry{Key: k2, Stage: HandOver, State: []byte("c")})
	os.WriteFile(filepath.Join(dir, "bad"+Ext), []byte("junk"), 0o644)
	os.WriteFile(filepath.Join(dir, ".nextcache-1.tmp"), []byte("x"), 0o644)
	if n := len(List(dir)); n != 4 {
		t.Fatalf("listed %d", n)
	}
	n, err := Prune(dir, func(in Info) bool { return in.Key.Firmware.Digest == k.Firmware.Digest })
	if err != nil || n != 3 { // the other firmware's, the damaged one, the leftover
		t.Fatalf("pruned %d, %v", n, err)
	}
	if got := len(List(dir)); got != 2 {
		t.Fatalf("left %d", got)
	}
	if n, _ := Clear(dir); n != 2 {
		t.Fatalf("cleared %d", n)
	}
	if n, err := Clear(filepath.Join(dir, "none")); n != 0 || err != nil {
		t.Fatalf("a folder that is not there: %d %v", n, err)
	}
}
