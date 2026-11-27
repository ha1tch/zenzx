package settingsconfig

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A settings.json from before the Open dialogs kept anything has none of their keys, and reads as
// nothing kept: no recent files, no folders, no view.
func TestOpenDialogKeysAreOptional(t *testing.T) {
	res, err := Load(filepath.Join(t.TempDir(), "none.json"), []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatal(err)
	}
	s := res.Settings
	if len(s.RecentFilePaths()) != 0 || s.OpenFolderPath() != "" || s.DiskFolderPath() != "" || s.FileDialogChoice() != nil {
		t.Errorf("the defaults are not empty: %v %q %q %v", s.RecentFilePaths(), s.OpenFolderPath(), s.DiskFolderPath(), s.FileDialogChoice())
	}
	var none *Settings
	if none.RecentFilePaths() != nil || none.OpenFolderPath() != "" || none.DiskFolderPath() != "" || none.FileDialogChoice() != nil {
		t.Error("a nil Settings is not empty")
	}
}

func TestOpenDialogKeysRoundTrip(t *testing.T) {
	want := &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 1, DisplayScale: 2,
		RecentFiles: []string{"/home/u/tapes/jsw.tzx", "/home/u/disks/game.dsk"},
		OpenFolder:  "/home/u/tapes",
		DiskFolder:  "/home/u/disks",
		FileDialog:  &FileDialogView{Hidden: true, Details: true, Sort: "date", Descending: true},
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil || !res.FromDisk {
		t.Fatalf("the saved file was refused: %v %s", err, res.Warning)
	}
	got := res.Settings
	if !reflect.DeepEqual(got.RecentFilePaths(), want.RecentFiles) || got.OpenFolderPath() != want.OpenFolder ||
		got.DiskFolderPath() != want.DiskFolder || !reflect.DeepEqual(got.FileDialogChoice(), want.FileDialog) {
		t.Errorf("read back %+v, saved %+v", got, want)
	}
	// What is handed out is a copy: changing it does not change the settings.
	c := got.FileDialogChoice()
	c.Hidden = false
	r := got.RecentFilePaths()
	r[0] = "x"
	if !got.FileDialog.Hidden || got.RecentFiles[0] == "x" {
		t.Error("the accessors hand out the settings' own values")
	}
}

func TestOpenDialogKeysAreValidated(t *testing.T) {
	with := func(extra string) error {
		_, err := parseAndValidate([]byte(strings.Replace(validSettingsJSON(), `"fixedMenuBar": false`, `"fixedMenuBar": false, `+extra, 1)), testThemes, testFonts)
		return err
	}
	for _, ok := range []string{
		`"recentFiles": ["/a/b.tap"]`,
		`"openFolder": "/a"`,
		`"diskFolder": "/b"`,
		`"fileDialog": {}`,
		`"fileDialog": {"hidden": true, "details": false, "sort": "size", "descending": true}`,
	} {
		if err := with(ok); err != nil {
			t.Errorf("%s refused: %v", ok, err)
		}
	}
	for _, bad := range []string{
		`"recentFiles": [""]`,
		`"recentFiles": "/a/b.tap"`,
		`"openFolder": ""`,
		`"diskFolder": 3`,
		`"fileDialog": {"sort": "colour"}`,
		`"fileDialog": {"hidden": "yes"}`,
	} {
		if err := with(bad); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
	for _, w := range FileSortWords {
		if err := with(`"fileDialog": {"sort": "` + w + `"}`); err != nil {
			t.Errorf("sort %q refused: %v", w, err)
		}
	}
}
