package settingsconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

// A settings.json from before "escape" existed, or one that leaves it out, or names a word
// that is no mode, reads as "off": the Escape key is disabled unless somebody chose otherwise.
func TestEscapeDefaultsToOffAndRoundTrips(t *testing.T) {
	var none *Settings
	if none.EscapeWord() != "off" || (&Settings{}).EscapeWord() != "off" || (&Settings{Escape: "quit"}).EscapeWord() != "off" {
		t.Error("the default is not off")
	}
	for _, w := range EscapeWords {
		ok := strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, "escape": "`+w+`"`, 1)
		if _, err := parseAndValidate([]byte(ok), testThemes, testFonts); err != nil {
			t.Errorf(`"escape": %q refused: %v`, w, err)
		}
	}
	bad := strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, "escape": "quit"`, 1)
	if _, err := parseAndValidate([]byte(bad), testThemes, testFonts); err == nil {
		t.Error(`"escape": "quit" was accepted`)
	}
	for _, w := range EscapeWords {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := Save(path, &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 1,
			DisplayScale: 2, Escape: w}); err != nil {
			t.Fatal(err)
		}
		res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
		if err != nil || !res.FromDisk {
			t.Fatalf("the saved file was refused: %v %s", err, res.Warning)
		}
		if res.Settings.EscapeWord() != w {
			t.Errorf("saved %q, read back %q", w, res.Settings.EscapeWord())
		}
	}
}

// The words are the ones the Preferences window and -escape offer, in the order of the
// escapeMode constants in package zenzx.
func TestEscapeWordsAreTheFourModes(t *testing.T) {
	if got := strings.Join(EscapeWords, ","); got != "off,close,query,on" {
		t.Errorf("EscapeWords = %s", got)
	}
}
