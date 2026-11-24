package settingsconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// A settings.json that is empty, or has nothing in it, or is broken, is no reason for the
// program not to start: it starts from the default and the file is made again.

func loadFrom(t *testing.T, content string) (*Result, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil {
		t.Fatalf("Load of %q failed instead of starting from the default: %v", content, err)
	}
	return res, path
}

func TestLoadRecreatesAFileWithNothingInIt(t *testing.T) {
	def, _ := parseAndValidate([]byte(validSettingsJSON()), testThemes, testFonts)
	for _, content := range []string{"", "  \n\t ", "{}", " { } ", "null"} {
		res, path := loadFrom(t, content)
		if res.Settings == nil || *res.Settings != *def {
			t.Errorf("%q: settings %+v, want the default %+v", content, res.Settings, def)
		}
		if !res.Recreated || res.Note == "" || res.Warning != "" {
			t.Errorf("%q: recreated %v note %q warning %q; want a recreated file, a note and no warning", content, res.Recreated, res.Note, res.Warning)
		}
		if _, err := os.Stat(path + ".bad"); err == nil {
			t.Errorf("%q: a file with nothing in it was kept as .bad", content)
		}
		again, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
		if err != nil || !again.FromDisk || again.Recreated || again.Note != "" || again.Warning != "" || *again.Settings != *def {
			t.Errorf("%q: the file it wrote is not a good settings file: %+v err %v", content, again, err)
		}
	}
}

func TestLoadMovesABrokenFileAsideAndRecreatesIt(t *testing.T) {
	for _, content := range []string{"{not json", "[]", "3", `{"theme":"Bogus"}`, `{"fontZoom":99}`, `{"dock":"yes"}`} {
		res, path := loadFrom(t, content)
		if res.Settings == nil || !res.Recreated || res.Warning == "" {
			t.Errorf("%q: settings %v recreated %v warning %q; want the default, a new file and a warning", content, res.Settings, res.Recreated, res.Warning)
			continue
		}
		kept, err := os.ReadFile(path + ".bad")
		if err != nil || string(kept) != content {
			t.Errorf("%q: the old file is %q (%v) at .bad, want it kept as it was", content, kept, err)
		}
		if again, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts); err != nil || !again.FromDisk || again.Warning != "" {
			t.Errorf("%q: the file it wrote is not a good settings file: %+v err %v", content, again, err)
		}
	}
}

// Some of the keys: the rest are the default's, and the file is left as it is.
func TestLoadFillsMissingKeysFromTheDefault(t *testing.T) {
	res, path := loadFrom(t, `{"theme":"Light","dock":false}`)
	if !res.FromDisk || res.Recreated || res.Warning != "" || res.Note != "" {
		t.Fatalf("a short file was not simply used: %+v", res)
	}
	def, _ := parseAndValidate([]byte(validSettingsJSON()), testThemes, testFonts)
	s := res.Settings
	if s.Theme != "Light" || s.DockOn() {
		t.Errorf("the file's own values were lost: theme %q dock %v", s.Theme, s.DockOn())
	}
	if s.Font != def.Font || s.FontZoom != def.FontZoom || s.DisplayScale != def.DisplayScale || s.Version != def.Version {
		t.Errorf("the missing keys are not the default's: %+v, default %+v", s, def)
	}
	if b, _ := os.ReadFile(path); string(b) != `{"theme":"Light","dock":false}` {
		t.Errorf("a short file was rewritten: %q", b)
	}
}

func TestLoadDoesNotCreateAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil || res.Settings == nil || res.Recreated || res.Warning != "" || res.Note != "" {
		t.Fatalf("a missing file: %+v err %v", res, err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("Load created a file that was not there")
	}
}

// A folder that cannot be written to is no reason not to start.
func TestLoadStartsEvenWhereTheFileCannotBeRecreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if f, err := os.Create(filepath.Join(dir, "probe")); err == nil { // running as a user the mode does not stop
		f.Close()
		t.Skip("the folder can be written to whatever its mode")
	}
	res, err := Load(path, []byte(validSettingsJSON()), testThemes, testFonts)
	if err != nil || res.Settings == nil {
		t.Fatalf("Load failed where the file could not be written: %v", err)
	}
	if res.Warning == "" {
		t.Error("no warning that the file could not be recreated")
	}
}
