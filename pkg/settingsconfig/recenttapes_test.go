package settingsconfig

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A settings.json from before the Tape menu kept tapes has no recentTapes, and reads as none.
func TestRecentTapesAreOptionalAndRoundTrip(t *testing.T) {
	res, err := Load(filepath.Join(t.TempDir(), "none.json"), []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatal(err)
	}
	var none *Settings
	if len(res.Settings.RecentTapePaths()) != 0 || none.RecentTapePaths() != nil {
		t.Error("the default is not no tapes")
	}

	want := &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 1, DisplayScale: 2,
		RecentTapes: []string{"/home/u/tapes/jsw.tzx", "/home/u/tapes/manic.tap"}}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil || !back.FromDisk {
		t.Fatalf("the saved file was refused: %v %s", err, back.Warning)
	}
	got := back.Settings.RecentTapePaths()
	if !reflect.DeepEqual(got, want.RecentTapes) {
		t.Errorf("read back %v, saved %v", got, want.RecentTapes)
	}
	got[0] = "x" // what is handed out is a copy
	if back.Settings.RecentTapes[0] == "x" {
		t.Error("the accessor hands out the settings' own list")
	}
}

func TestRecentTapesAreValidated(t *testing.T) {
	with := func(extra string) error {
		_, err := parseAndValidate([]byte(strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, `+extra, 1)), testThemes, testFonts)
		return err
	}
	if err := with(`"recentTapes": ["/a/b.tap"]`); err != nil {
		t.Errorf("a list of tapes was refused: %v", err)
	}
	for _, bad := range []string{`"recentTapes": [""]`, `"recentTapes": "/a/b.tap"`, `"recentTapes": [3]`} {
		if err := with(bad); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}
