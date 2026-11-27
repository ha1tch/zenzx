// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/zentools/pkg/scr"
	"github.com/ha1tch/zentools/pkg/zxn"
)

// The Timex screens are saved as .hires.scr and .hicol.scr: extensions of two words, which
// everything that reads a screenshot's name has to read as one, and which say what the 12288
// bytes are.

func TestTimexScreensHaveTheirOwnExtensions(t *testing.T) {
	for f, want := range map[ShotFormat]string{ShotSCR: "scr", ShotHiRes: "hires.scr", ShotHiColour: "hicol.scr"} {
		if got := f.Ext(); got != want {
			t.Errorf("format %d: extension %q, want %q", f, got, want)
		}
	}
	when := time.Date(2020, 11, 27, 9, 30, 1, 0, time.Local)
	for _, ext := range []string{"hires.scr", "hicol.scr"} {
		name := screenshotName("Horace", when, ext)
		if name != "screenshot_Horace_20201127_093001."+ext {
			t.Fatalf("the name is %q", name)
		}
		machine, at, got, ok := parseScreenshotName(name)
		if !ok || machine != "Horace" || got != ext || !at.Equal(when) {
			t.Errorf("%s: parsed %q, %v, %q, %v", name, machine, at, got, ok)
		}
	}
	// A plain .scr is still the ULA's. A name that merely has another word before its .scr is
	// not one of ours, and is read as the .scr it ends in.
	if _, _, ext, ok := parseScreenshotName("screenshot_Horace_20201127_093001.scr"); !ok || ext != "scr" {
		t.Errorf("a plain .scr parsed as %q, %v", ext, ok)
	}
	if _, _, _, ok := parseScreenshotName("screenshot_Horace_20201127_093001.timex.scr"); ok {
		t.Error("a .timex.scr was taken for one of ours")
	}
	if _, _, _, ok := parseScreenshotName("screenshot_Horace_20201127_093001.hires.png"); ok {
		t.Error("a .hires.png was taken for one of ours")
	}
	for name, want := range map[string]string{"A.HIRES.SCR": "hires.scr", "a.Hicol.Scr": "hicol.scr", "A.Scr": "scr", "x.timex.scr": "scr", "noext": ""} {
		if got := shotExtOf(name); got != want {
			t.Errorf("shotExtOf(%q) = %q, want %q", name, got, want)
		}
	}
	for name, want := range map[string][2]string{
		"a b.hires.scr":   {"a b", ".hires.scr"},
		"a.HICOL.SCR":     {"a", ".HICOL.SCR"},
		"a.hires.scr.png": {"a.hires.scr", ".png"},
		"a.scr":           {"a", ".scr"},
		"plain":           {"plain", ""},
	} {
		if shotStem(name) != want[0] || shotSuffix(name) != want[1] {
			t.Errorf("%q: stem %q suffix %q, want %q", name, shotStem(name), shotSuffix(name), want)
		}
	}
}

func TestSaveHiResAndHiColourScreenshots(t *testing.T) {
	n := nextShotMachine()
	n.memory.nextPageWrite(10, 0, 0xA1) // the first pixel and the third of the bitmap's first byte are ink
	n.memory.nextPageWrite(11, 0, 0xB2)
	dir := t.TempDir()
	save := func(f ShotFormat, sec int) string {
		t.Helper()
		path, err := withShotDir(dir, func() (string, error) {
			return n.SaveScreenshot(f, true, time.Date(2020, 11, 27, 9, 30, sec, 0, time.Local))
		})
		if err != nil {
			t.Fatal(err)
		}
		return path
	}

	// Hi-res, with magenta (3) ink: its file has the port's byte after the halves, and the picture
	// has the ink and its complement for paper, both bright, as the machine draws it.
	n.io.nextTimexPortWrite(0x1E)
	res := save(ShotHiRes, 0)
	if !strings.HasSuffix(res, "_20201127_093000.hires.scr") {
		t.Errorf("hi-res saved as %s", res)
	}
	// Hi-colour.
	n.io.nextTimexPortWrite(0x02)
	col := save(ShotHiColour, 1)
	if !strings.HasSuffix(col, "_20201127_093001.hicol.scr") {
		t.Errorf("hi-colour saved as %s", col)
	}

	got := listScreenshots(dir)
	if len(got) != 2 {
		t.Fatalf("the folder lists %+v", got)
	}
	for _, f := range got {
		switch f.Ext {
		case "hicol.scr":
			if f.Size != zxn.TimexFileLen {
				t.Errorf("the hi-colour file is %d bytes", f.Size)
			}
		case "hires.scr":
			if f.Size != zxn.TimexFileLen+1 {
				t.Errorf("the hi-res file is %d bytes", f.Size)
			}
		default:
			t.Errorf("listed with the extension %q", f.Ext)
		}
		if f.Machine == "" || f.When.Day() != 27 {
			t.Errorf("not read as a name of ours: %+v", f)
		}
	}

	p, err := loadShotPicture(res, true)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 512 || h != 192 {
		t.Fatalf("the hi-res picture is %dx%d", w, h)
	}
	img := p.Frames[0]
	ink, paper := scr.ULAColor(3, true), scr.ULAColor(4, true)
	if img.RGBAAt(0, 0) != ink || img.RGBAAt(1, 0) != paper || img.RGBAAt(8, 0) != ink {
		t.Errorf("hi-res pixels %v %v %v, want ink %v, paper %v, ink", img.RGBAAt(0, 0), img.RGBAAt(1, 0), img.RGBAAt(8, 0), ink, paper)
	}
	p, err = loadShotPicture(col, true)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 256 || h != 192 {
		t.Errorf("the hi-colour picture is %dx%d", w, h)
	}
}

func TestLoadTimexScreensWantTheirSizes(t *testing.T) {
	dir := t.TempDir()
	bitmap := make([]byte, zxn.TimexFileLen) // a bitmap in both halves: hi-res to a guess
	for i := range bitmap {
		bitmap[i] = byte(i * 13)
	}
	attrs := make([]byte, zxn.TimexFileLen) // a few attributes in the second: hi-colour to a guess
	copy(attrs, bitmap[:zxn.TimexBitmapLen])
	for i := zxn.TimexBitmapLen; i < zxn.TimexFileLen; i++ {
		attrs[i] = byte(0x38 + i%3)
	}
	plus := func(b []byte, extra byte) []byte { return append(append([]byte{}, b...), extra) }
	for _, c := range []struct {
		name string
		data []byte
		w    int // the picture's width, 0 for refused
	}{
		{"a.hires.scr", bitmap, 512},
		{"b.HIRES.SCR", plus(bitmap, 0x06), 512},
		{"c.hires.scr", make([]byte, 6912), 0}, // a ULA screen is not a Timex one
		{"d.hires.scr", make([]byte, 100), 0},
		{"e.hicol.scr", attrs, 256},
		{"f.hicol.scr", plus(attrs, 0x02), 256},
		{"g.hicol.scr", make([]byte, 6912), 0},
		// The extension decides: a screen that looks like the other kind is read as the kind it says.
		{"h.hires.scr", attrs, 512},
		{"i.hicol.scr", bitmap, 256},
		// A plain .scr of the Timex's size, from another tool, has nothing to say and is guessed.
		{"j.scr", bitmap, 512},
		{"k.scr", attrs, 256},
		{"l.scr", plus(bitmap, 0x06), 512},
		// With the port's byte after it, a plain .scr is hi-res, whatever its second half is like.
		{"m.scr", plus(attrs, 0x06), 512},
	} {
		p, err := loadShotPicture(writeShotFile(t, dir, c.name, c.data), true)
		if c.w == 0 {
			if err == nil {
				t.Errorf("%s of %d bytes was read", c.name, len(c.data))
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if w, h := p.Size(); w != c.w || h != 192 {
			t.Errorf("%s is %dx%d, want %dx192", c.name, w, h, c.w)
		}
	}
}

// Opening a .hires.scr or a .hicol.scr puts both halves in and sets the mode (T-103).

// timexTestScreen is 12288 bytes whose first half is all a and whose second is all b.
func timexTestScreen(a, b byte) []byte {
	d := make([]byte, zxn.TimexFileLen)
	for i := range d {
		d[i] = a
		if i >= zxn.TimexBitmapLen {
			d[i] = b
		}
	}
	return d
}

func timexTempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadTimexScreensIntoANext(t *testing.T) {
	for _, c := range []struct {
		name     string
		data     []byte
		wantPort uint8
		format   ShotFormat
	}{
		{"a.hires.scr", append(timexTestScreen(0xA5, 0x5A), 0x1E), 0x18, ShotHiRes},
		{"a.hires.scr", timexTestScreen(0xA5, 0x5A), 0, ShotHiRes}, // no byte of port 0xFF: ink 0
		{"a.scr", append(timexTestScreen(0xA5, 0x5A), 0x2E), 0x28, ShotHiRes},
		{"a.hicol.scr", timexTestScreen(0xA5, 0x5A), 0, ShotHiColour},
	} {
		n := nextShotMachine()
		if err := n.LoadSCR(timexTempFile(t, c.name, c.data)); err != nil {
			t.Fatalf("%s (%d bytes): %v", c.name, len(c.data), err)
		}
		if got := n.io.bank5(0); got != 0xA5 {
			t.Errorf("%s: the first half starts with %02X, want A5", c.name, got)
		}
		if got := n.io.bank5(0x2000); got != 0x5A {
			t.Errorf("%s: the second half starts with %02X, want 5A", c.name, got)
		}
		if got := n.io.bank5(zxn.TimexBitmapLen - 1); got != 0xA5 {
			t.Errorf("%s: the first half ends with %02X, want A5", c.name, got)
		}
		wantMode := uint8(6)
		if c.format == ShotHiColour {
			wantMode = 2
		}
		if got := n.io.nextTimexPort; got&7 != wantMode || got&0x38 != c.wantPort&0x38 {
			t.Errorf("%s: port 0xFF is %02X, want mode %d and ink bits %02X", c.name, got, wantMode, c.wantPort&0x38)
		}
		if !n.ShotApplicable(c.format) {
			t.Errorf("%s: the machine is not in the mode the screen is", c.name)
		}
		// And it saves again as it was loaded: the halves, and for hi-res the port's byte.
		got, err := n.ShotBytes(c.format, true)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got[:zxn.TimexFileLen], timexTestScreen(0xA5, 0x5A)) {
			t.Errorf("%s: saving it again gives other halves", c.name)
		}
		if c.format == ShotHiRes && (len(got) != zxn.TimexFileLen+1 || got[zxn.TimexFileLen]&0x3F != 0x06|c.wantPort&0x38) {
			t.Errorf("%s: saved again it is %d bytes ending in %02X", c.name, len(got), got[len(got)-1])
		}
	}
}

func TestLoadTimexScreensIntoAClassicMachine(t *testing.T) {
	// A classic machine has no hi-res, and is left as it was when asked for one.
	zx := NewZenZX(AudioBackendOto)
	zx.memory.Write(0x4000, 0x77)
	err := zx.LoadSCR(timexTempFile(t, "a.hires.scr", append(timexTestScreen(1, 2), 0x06)))
	if err == nil || !strings.Contains(err.Error(), "hi-res") {
		t.Errorf("a hi-res screen on a 48K: %v", err)
	}
	err = zx.LoadSCR(timexTempFile(t, "a.hicol.scr", timexTestScreen(1, 2)))
	if err == nil || !strings.Contains(err.Error(), "hi-colour") {
		t.Errorf("a hi-colour screen on a 48K: %v", err)
	}
	if got := zx.memory.Read(0x4000); got != 0x77 {
		t.Errorf("a refused screen left 0x4000 as %02X, want 77", got)
	}

	// With the hi-colour renderer it is a machine that has the mode: both halves go in.
	if err := zx.SelectVideoRenderer(NSGraphicsTimex001HiColour); err != nil {
		t.Fatal(err)
	}
	if err := zx.LoadSCR(timexTempFile(t, "a.hicol.scr", timexTestScreen(0xC3, 0x3C))); err != nil {
		t.Fatalf("a hi-colour screen with the hi-colour renderer: %v", err)
	}
	if a, b := zx.memory.Read(0x4000), zx.memory.Read(0x6000); a != 0xC3 || b != 0x3C {
		t.Errorf("the halves are %02X and %02X, want C3 and 3C", a, b)
	}
	got, err := zx.ShotBytes(ShotHiColour, true)
	if err != nil || !bytes.Equal(got, timexTestScreen(0xC3, 0x3C)) {
		t.Errorf("saved again: %d bytes, %v", len(got), err)
	}
	// The renderer has no hi-res either.
	if err := zx.LoadSCR(timexTempFile(t, "b.hires.scr", timexTestScreen(1, 2))); err == nil {
		t.Error("a hi-res screen with the hi-colour renderer was loaded")
	}
}

func TestLoadTimexScreenOfTheWrongSize(t *testing.T) {
	n := nextShotMachine()
	n.memory.nextPageWrite(10, 0, 0x99)
	for _, c := range []struct {
		name string
		size int
	}{
		{"a.hires.scr", 6912}, {"a.hires.scr", 12290}, {"a.hicol.scr", 12290}, {"a.hicol.scr", 100},
	} {
		if err := n.LoadSCR(timexTempFile(t, c.name, make([]byte, c.size))); err == nil {
			t.Errorf("%s of %d bytes was loaded", c.name, c.size)
		}
	}
	if got := n.io.bank5(0); got != 0x99 {
		t.Errorf("a refused file changed the screen: %02X", got)
	}
	// A plain .scr of 6912 bytes is still a standard screen, in a Next as in any machine.
	std := make([]byte, 6912)
	std[0] = 0x3C
	if err := n.LoadSCR(timexTempFile(t, "std.scr", std)); err != nil {
		t.Fatal(err)
	}
	if got := n.memory.Read(0x4000); got != 0x3C {
		t.Errorf("a standard screen: 0x4000 is %02X", got)
	}
}
