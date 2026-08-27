package settingsconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	testThemes = []string{"Dark", "Light", "Spectrum"}
	testFonts  = []string{"Sinclair", "TomThumb"}
)

func validSettingsJSON() string {
	return `{
		"version": 1,
		"theme": "Dark",
		"font": "Sinclair",
		"fontZoom": 2,
		"displayScale": 2,
		"fixedMenuBar": false
	}`
}

func TestParseAndValidateAcceptsValidSettings(t *testing.T) {
	s, err := parseAndValidate([]byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatalf("parseAndValidate: %v", err)
	}
	if s.Theme != "Dark" || s.Font != "Sinclair" || s.FontZoom != 2 || s.DisplayScale != 2 || s.FixedMenuBar != false {
		t.Errorf("unexpected settings: %+v", s)
	}
}

func TestParseAndValidateRejectsInvalidJSON(t *testing.T) {
	_, err := parseAndValidate([]byte(`{not valid`), testThemes, testFonts)
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestParseAndValidateRejectsUnknownTheme(t *testing.T) {
	data := `{"version":1,"theme":"Nonexistent","font":"Sinclair","fontZoom":2,"displayScale":2,"fixedMenuBar":false}`
	_, err := parseAndValidate([]byte(data), testThemes, testFonts)
	if err == nil {
		t.Fatal("expected an error for a theme not in validThemes")
	}
}

func TestParseAndValidateRejectsUnknownFont(t *testing.T) {
	data := `{"version":1,"theme":"Dark","font":"Nonexistent","fontZoom":2,"displayScale":2,"fixedMenuBar":false}`
	_, err := parseAndValidate([]byte(data), testThemes, testFonts)
	if err == nil {
		t.Fatal("expected an error for a font not in validFonts")
	}
}

func TestParseAndValidateRejectsFontZoomOutOfRange(t *testing.T) {
	data := `{"version":1,"theme":"Dark","font":"Sinclair","fontZoom":5,"displayScale":2,"fixedMenuBar":false}`
	_, err := parseAndValidate([]byte(data), testThemes, testFonts)
	if err == nil {
		t.Fatal("expected an error for fontZoom outside 1-3")
	}
}

func TestParseAndValidateRejectsDisplayScaleOutOfRange(t *testing.T) {
	data := `{"version":1,"theme":"Dark","font":"Sinclair","fontZoom":2,"displayScale":99,"fixedMenuBar":false}`
	_, err := parseAndValidate([]byte(data), testThemes, testFonts)
	if err == nil {
		t.Fatal("expected an error for displayScale above MaxDisplayScale")
	}
}

func TestParseAndValidateRejectsMissingRequiredField(t *testing.T) {
	data := `{"version":1,"theme":"Dark","font":"Sinclair","fontZoom":2,"displayScale":2}`
	_, err := parseAndValidate([]byte(data), testThemes, testFonts)
	if err == nil {
		t.Fatal("expected an error for a missing required field (fixedMenuBar)")
	}
}

func TestLoadUsesValidDiskFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(validSettingsJSON()), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !res.FromDisk {
		t.Error("FromDisk should be true when a valid disk file exists")
	}
	if res.Warning != "" {
		t.Errorf("Warning should be empty on success, got %q", res.Warning)
	}
}

func TestLoadFallsBackOnInvalidDiskFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"theme":"Bogus"}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatalf("Load should fall back, not error: %v", err)
	}
	if res.FromDisk {
		t.Error("FromDisk should be false when the disk file is invalid")
	}
	if res.Warning == "" {
		t.Error("Warning should explain why the disk file was rejected")
	}
}

func TestLoadFallsBackSilentlyOnMissingDiskFile(t *testing.T) {
	res, err := Load("/nonexistent/path/settings.json", []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.FromDisk {
		t.Error("FromDisk should be false when the disk file doesn't exist")
	}
	if res.Warning != "" {
		t.Errorf("a missing (not merely invalid) disk file is the expected common case, should not warn: %q", res.Warning)
	}
}

func TestLoadWithEmptyDiskPathUsesEmbeddedOnly(t *testing.T) {
	res, err := Load("", []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.FromDisk {
		t.Error("FromDisk should be false when diskPath is empty")
	}
	if res.Settings == nil {
		t.Fatal("Settings should still be populated from embedded")
	}
}

func TestLoadErrorsOnInvalidEmbedded(t *testing.T) {
	_, err := Load("", []byte(`{"version":1,"theme":"Bogus"}`), testThemes, testFonts)
	if err == nil {
		t.Fatal("Load should error when even the embedded default is invalid")
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	want := &Settings{
		Version:      1,
		Theme:        "Spectrum",
		Font:         "TomThumb",
		FontZoom:     3,
		DisplayScale: 4,
		FixedMenuBar: true,
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !res.FromDisk {
		t.Fatal("Load should have used the just-saved disk file")
	}
	got := res.Settings
	if *got != *want {
		t.Errorf("round-tripped settings = %+v, want %+v", got, want)
	}
}

func TestSaveIsAtomicNoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	s := &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 2, DisplayScale: 2, FixedMenuBar: false}

	if err := Save(path, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("the .tmp file should not remain after a successful Save (rename should have consumed it)")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the final settings.json should exist: %v", err)
	}
}

func TestSaveOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	first := &Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 2, DisplayScale: 2, FixedMenuBar: false}
	second := &Settings{Version: 1, Theme: "Light", Font: "TomThumb", FontZoom: 1, DisplayScale: 3, FixedMenuBar: true}

	if err := Save(path, first); err != nil {
		t.Fatalf("Save (first): %v", err)
	}
	if err := Save(path, second); err != nil {
		t.Fatalf("Save (second): %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "Light") {
		t.Error("the file should contain the second Save's values, not the first's")
	}
}

func TestSchemaMaxDisplayScaleMatchesDisplayPackage(t *testing.T) {
	// MaxDisplayScale is deliberately duplicated here rather than
	// imported from the display package (this package has no GUI-layer
	// dependency). This is the guard that keeps it honest: if the
	// display package's own MaxMultiplier ever changes, this constant
	// (and this test) must be updated to match by hand.
	if MaxDisplayScale != 5 {
		t.Errorf("MaxDisplayScale = %d, want 5 (must match display.MaxMultiplier -- update both together)", MaxDisplayScale)
	}
}
