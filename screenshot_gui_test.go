//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// shotBar is a menu bar on a 48K machine with the screenshots folder in a temporary place, and
// the notices the machine gives collected.
func shotBar(t *testing.T) (b *appMenuBar, zx *ZenZX, notices *[]string) {
	t.Helper()
	old := screenshotDir
	screenshotDir = filepath.Join(t.TempDir(), "shots")
	t.Cleanup(func() { screenshotDir = old })
	zx = testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	var got []string
	zx.OnNotice = func(msg string) { got = append(got, msg) }
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	t.Cleanup(func() { b.text.Unload() })
	b.refreshViewItems(zx)
	return b, zx, &got
}

// shotItem is the Screenshot row of the View menu and its index.
func shotItem(t *testing.T, b *appMenuBar) (int, zenui.Item) {
	t.Helper()
	items := b.widget.ItemsFor(barView)
	for i, it := range items {
		if it.Label == "Screenshot" {
			return i, it
		}
	}
	t.Fatal("the View menu has no Screenshot row")
	return 0, zenui.Item{}
}

func TestScreenshotSubmenuRowsFollowTheMachine(t *testing.T) {
	b, zx, _ := shotBar(t)
	_, it := shotItem(t, b)
	if len(it.SubItems) != len(ShotFormats)+2 {
		t.Fatalf("%d rows under Screenshot, want %d (a row for each format, a divider, Manage Screenshots)", len(it.SubItems), len(ShotFormats)+2)
	}
	enabled := map[ShotFormat]bool{ShotPNG: true, ShotSCR: true, ShotGIF: true}
	for i, f := range ShotFormats {
		row := it.SubItems[i]
		if row.Label != screenshotLabels[f] || row.Disabled == enabled[f] {
			t.Errorf("row %d is %q (disabled %v), want %q, disabled %v on a 48K", i, row.Label, row.Disabled, screenshotLabels[f], !enabled[f])
		}
	}
	if !it.SubItems[len(ShotFormats)].Separator || it.SubItems[len(ShotFormats)+1].Label != "Manage Screenshots..." {
		t.Errorf("the last rows are %+v and %+v, want a divider and Manage Screenshots...", it.SubItems[len(ShotFormats)], it.SubItems[len(ShotFormats)+1])
	}

	// With no machine running every format is greyed.
	zx.stopped = true
	b.refreshViewItems(zx)
	_, it = shotItem(t, b)
	for i, f := range ShotFormats {
		if !it.SubItems[i].Disabled {
			t.Errorf("%s is offered with no machine running", screenshotLabels[f])
		}
	}
}

func TestScreenshotPickSavesAndSays(t *testing.T) {
	b, zx, notices := shotBar(t)
	at, _ := shotItem(t, b)
	if !b.viewMenu.pick(zx, at, 0) { // Save as PNG
		t.Fatal("picking Save as PNG ran nothing")
	}
	files, _ := os.ReadDir(screenshotDir)
	if len(files) != 1 {
		t.Fatalf("%d files in the screenshots folder, want 1", len(files))
	}
	name := files[0].Name()
	machine, _, ext, ok := parseScreenshotName(name)
	if !ok || ext != "png" || !strings.HasPrefix(machine, "Spectrum") {
		t.Errorf("the file is %q: machine %q, extension %q, a screenshot's name %v", name, machine, ext, ok)
	}
	if len(*notices) != 1 || !strings.Contains((*notices)[0], name) {
		t.Errorf("the notices are %q, want one that names %s", *notices, name)
	}
	// A format that does not apply is a greyed row, and picking it does nothing.
	before := len(*notices)
	if b.viewMenu.pick(zx, at, int(indexOfShot(ShotLayer2))) {
		t.Error("picking a greyed row ran something")
	}
	if len(*notices) != before {
		t.Errorf("a greyed row said %q", (*notices)[before:])
	}
}

func indexOfShot(f ShotFormat) int {
	for i, g := range ShotFormats {
		if g == f {
			return i
		}
	}
	return -1
}

func TestScreenshotGIFStartsOnceAndSaysSo(t *testing.T) {
	b, zx, notices := shotBar(t)
	at, _ := shotItem(t, b)
	gifRow := indexOfShot(ShotGIF)
	zx.paused = false
	b.viewMenu.pick(zx, at, gifRow)
	if _, on := zx.GIFRecording(); !on {
		t.Fatal("Save as GIF did not start a recording")
	}
	b.viewMenu.pick(zx, at, gifRow)
	if len(*notices) != 2 || !strings.Contains((*notices)[1], "already") {
		t.Errorf("the notices are %q, want the second to say a recording is under way already", *notices)
	}
	zx.CancelGIF()

	// A paused machine says that the GIF waits for it.
	zx.paused = true
	*notices = nil
	b.viewMenu.pick(zx, at, gifRow)
	if len(*notices) != 1 || !strings.Contains((*notices)[0], "paused") {
		t.Errorf("the notices are %q, want one that says the machine is paused", *notices)
	}
	zx.CancelGIF()
}
