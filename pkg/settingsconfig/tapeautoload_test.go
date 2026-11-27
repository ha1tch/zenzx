// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package settingsconfig

import (
	"path/filepath"
	"testing"
)

// A settings.json from before tapeAutoload existed autoloads tapes (the default is on), and the
// choice to turn it off survives a save and a load.
func TestTapeAutoloadDefaultsToOnAndRoundTrips(t *testing.T) {
	var none *Settings
	if !none.TapeAutoloadOn() || !(&Settings{}).TapeAutoloadOn() {
		t.Error("the default is not on")
	}
	off := false
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 1,
		DisplayScale: 2, TapeAutoload: &off}); err != nil {
		t.Fatal(err)
	}
	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil || !res.FromDisk {
		t.Fatalf("the saved file was refused: %v %s", err, res.Warning)
	}
	if res.Settings.TapeAutoloadOn() {
		t.Error("autoload read back as on after being saved off")
	}
}
