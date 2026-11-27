//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// A tape dropped on the machine starts by itself when Preferences say so, and only waits in the
// deck when they do not (or when nothing is wired, as everywhere but the GUI).
func TestDroppedTapeFollowsTheAutoloadPreference(t *testing.T) {
	defer func(f func() bool) { tapeAutoloadWanted = f }(tapeAutoloadWanted)
	image, err := BasicTape("hello", "10 POKE 32768,171\n")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "hello.tap")
	if err := os.WriteFile(path, image, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name  string
		wired func() bool
		loads bool
	}{
		{"on", func() bool { return true }, true},
		{"off", func() bool { return false }, false},
		{"nothing wired", nil, false},
	} {
		tapeAutoloadWanted = c.wired
		zx := typistMachine(t, "48k")
		for i := 0; i < 100; i++ {
			zx.RunFrame()
		}
		msg := zx.loadDroppedFile(path)
		if zx.TypistActive() != c.loads {
			t.Errorf("%s: typist active %v, want %v (%q)", c.name, zx.TypistActive(), c.loads, msg)
		}
		if c.loads {
			if ranProgram(zx, 5000) < 0 {
				t.Errorf("%s: the dropped tape never loaded by itself:\n%s", c.name, screenText(zx))
			}
		} else if ranProgram(zx, 600) >= 0 {
			t.Errorf("%s: the program ran though autoload was off", c.name)
		}
		if !c.loads && msg == "" {
			t.Errorf("%s: no toast", c.name)
		}
	}
}

// The preference is in the Machines tab, is on by default, and what the bar writes is accepted
// by the settings file's schema and read back by a new bar.
func TestAutoloadTapePreferenceRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, nil, path)
	if !b.tapeAutoload {
		t.Error("autoload is not on by default")
	}
	row := prefRowNamed(t, b, zx, "Autoload tape")
	for _, want := range []bool{false, true, false} {
		v := 0
		if want {
			v = 1
		}
		row.set(v)
		if b.tapeAutoload != want || row.get() != v {
			t.Fatalf("set %d: the bar has %v, the row reads %d", v, b.tapeAutoload, row.get())
		}
		b.saveSettings(zx)
		res, err := settingsconfig.Load(path, defaultSettingsJSON, validThemeNames, validFontNames)
		if err != nil || !res.FromDisk {
			t.Fatalf("what the bar wrote was refused: %v %s", err, res.Warning)
		}
		if res.Settings.TapeAutoloadOn() != want {
			t.Errorf("saved %v, the file holds %v", want, res.Settings.TapeAutoloadOn())
		}
		b2, _ := newTestBar(t, res.Settings, path)
		if b2.tapeAutoload != want {
			t.Errorf("a new bar reads %v back, want %v", b2.tapeAutoload, want)
		}
	}
}
