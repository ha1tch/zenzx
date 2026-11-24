package settingsconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

// A settings.json from before tapeDeck existed reads as "top", the word is a closed set, and
// the choice survives a save and a load.
func TestTapeDeckDefaultsToTopAndRoundTrips(t *testing.T) {
	var none *Settings
	if none.TapeDeckWord() != "top" || (&Settings{}).TapeDeckWord() != "top" {
		t.Error("the default is not top")
	}
	for _, w := range []string{"off", "top", "bottom"} {
		ok := strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, "tapeDeck": "`+w+`"`, 1)
		if _, err := parseAndValidate([]byte(ok), testThemes, testFonts); err != nil {
			t.Errorf(`"tapeDeck": %q refused: %v`, w, err)
		}
	}
	bad := strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, "tapeDeck": "left"`, 1)
	if _, err := parseAndValidate([]byte(bad), testThemes, testFonts); err == nil {
		t.Error(`"tapeDeck": "left" was accepted`)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 1,
		DisplayScale: 2, TapeDeck: "bottom"}); err != nil {
		t.Fatal(err)
	}
	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil || !res.FromDisk {
		t.Fatalf("the saved file was refused: %v %s", err, res.Warning)
	}
	if res.Settings.TapeDeckWord() != "bottom" {
		t.Errorf("read back %q", res.Settings.TapeDeckWord())
	}
}
