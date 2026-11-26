//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// With no settings file, an old one, or one with no "escape" in it, the Escape key is disabled,
// and the embedded defaults say so.
func TestEscapeIsDisabledByDefault(t *testing.T) {
	for name, s := range map[string]*settingsconfig.Settings{
		"no file":  nil,
		"old file": {Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 2, DisplayScale: 2},
	} {
		b, zx := newTestBar(t, s, "")
		if b.escapeNow() != escapeOff {
			t.Errorf("%s: Escape is %v, want off", name, b.escapeNow())
		}
		if r := prefRowNamed(t, b, zx, "Escape"); r.get() != int(escapeOff) {
			t.Errorf("%s: the Escape row reads %d, want Disabled", name, r.get())
		}
	}
	res, err := settingsconfig.Load("", defaultSettingsJSON, validThemeNames, validFontNames)
	if err != nil {
		t.Fatal(err)
	}
	if res.Settings.EscapeWord() != "off" {
		t.Errorf("the embedded defaults say Escape is %q", res.Settings.EscapeWord())
	}
	if (*appMenuBar)(nil).escapeNow() != escapeOff {
		t.Error("with no bar at all, Escape is not off")
	}
}

// The Preferences row has the four choices, changes the bar, and is saved and read back.
func TestEscapePreferencesRowIsSavedAndComesBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, nil, path)
	row := prefRowNamed(t, b, zx, "Escape")
	if got := strings.Join(row.options, "|"); got != "Disabled|Closes windows only|Asks before quitting|Quits at once" {
		t.Errorf("the Escape row offers %q", got)
	}
	for i, want := range []escapeMode{escapeOff, escapeClose, escapeQuery, escapeOn} {
		row.set(i)
		if b.escapeNow() != want || row.get() != i {
			t.Errorf("row set %d: the bar has %v, the row reads %d", i, b.escapeNow(), row.get())
		}
		b.saveSettings(zx)
		res, err := settingsconfig.Load(path, defaultSettingsJSON, validThemeNames, validFontNames)
		if err != nil || !res.FromDisk {
			t.Fatalf("what the bar wrote was refused: %v %s", err, res.Warning)
		}
		if res.Settings.EscapeWord() != want.String() {
			t.Errorf("saved %v, the file holds %q", want, res.Settings.EscapeWord())
		}
		b2, _ := newTestBar(t, res.Settings, path)
		if b2.escapeNow() != want {
			t.Errorf("a new bar reads %v back, want %v", b2.escapeNow(), want)
		}
	}
}

// -escape wins for the session without being saved; the Preferences window then has the say.
func TestEscapeSwitchOverridesOneSessionAndIsNotSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, &settingsconfig.Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 2,
		DisplayScale: 2, Escape: "close"}, path)
	on := escapeOn
	b.escapeFlag = &on
	if b.escapeNow() != escapeOn {
		t.Fatalf("with -escape=on Escape is %v", b.escapeNow())
	}
	b.saveSettings(zx)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"escape": "close"`) {
		t.Errorf("the switch was written to the file, or the saved choice lost:\n%s", data)
	}
	prefRowNamed(t, b, zx, "Escape").set(int(escapeQuery))
	if b.escapeNow() != escapeQuery || b.escapeFlag != nil {
		t.Errorf("after the window changed it: Escape %v, the switch still stands: %v", b.escapeNow(), b.escapeFlag != nil)
	}
}
