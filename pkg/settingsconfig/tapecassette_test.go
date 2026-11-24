package settingsconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

// A settings.json from before tapeCassette existed reads as "big", the word is a closed set,
// and the choice survives a save and a load.
func TestTapeCassetteDefaultsToBigAndRoundTrips(t *testing.T) {
	var none *Settings
	if none.TapeCassetteWord() != "big" || (&Settings{}).TapeCassetteWord() != "big" {
		t.Error("the default is not big")
	}
	for _, w := range []string{"small", "big", "hidden"} {
		ok := strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, "tapeCassette": "`+w+`"`, 1)
		if _, err := parseAndValidate([]byte(ok), testThemes, testFonts); err != nil {
			t.Errorf(`"tapeCassette": %q refused: %v`, w, err)
		}
	}
	bad := strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, "tapeCassette": "huge"`, 1)
	if _, err := parseAndValidate([]byte(bad), testThemes, testFonts); err == nil {
		t.Error(`"tapeCassette": "huge" was accepted`)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 1,
		DisplayScale: 2, TapeCassette: "big"}); err != nil {
		t.Fatal(err)
	}
	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil || !res.FromDisk {
		t.Fatalf("the saved file was refused: %v %s", err, res.Warning)
	}
	if res.Settings.TapeCassetteWord() != "big" {
		t.Errorf("read back %q", res.Settings.TapeCassetteWord())
	}
}
