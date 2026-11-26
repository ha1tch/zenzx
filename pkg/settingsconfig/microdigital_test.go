package settingsconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

// A settings.json from before "microdigital" existed, or one that leaves it out or names a word
// that is no position, reads as "spanish": the switch is left alone unless somebody turned it.
func TestMicrodigitalDefaultsToSpanishAndRoundTrips(t *testing.T) {
	var none *Settings
	if none.MicrodigitalWord() != "spanish" || (&Settings{}).MicrodigitalWord() != "spanish" ||
		(&Settings{Microdigital: "portugues"}).MicrodigitalWord() != "spanish" {
		t.Error("the default is not spanish")
	}
	for _, w := range MicrodigitalWords {
		ok := strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, "microdigital": "`+w+`"`, 1)
		if _, err := parseAndValidate([]byte(ok), testThemes, testFonts); err != nil {
			t.Errorf(`"microdigital": %q refused: %v`, w, err)
		}
	}
	bad := strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, "microdigital": "english"`, 1)
	if _, err := parseAndValidate([]byte(bad), testThemes, testFonts); err == nil {
		t.Error(`"microdigital": "english" was accepted`)
	}
	for _, w := range MicrodigitalWords {
		path := filepath.Join(t.TempDir(), "settings.json")
		if err := Save(path, &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 1,
			DisplayScale: 2, Microdigital: w}); err != nil {
			t.Fatal(err)
		}
		res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
		if err != nil || !res.FromDisk {
			t.Fatalf("the saved file was refused: %v %s", err, res.Warning)
		}
		if res.Settings.MicrodigitalWord() != w {
			t.Errorf("saved %q, read back %q", w, res.Settings.MicrodigitalWord())
		}
	}
}

// The words are the ones the Preferences window and -language-switch offer.
func TestMicrodigitalWordsAreTheTwoPositions(t *testing.T) {
	if got := strings.Join(MicrodigitalWords, ","); got != "spanish,portuguese" {
		t.Errorf("MicrodigitalWords = %s", got)
	}
}
