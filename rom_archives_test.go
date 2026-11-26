package zenzx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeArchives puts text in dir as the ROM archives file.
func writeArchives(t *testing.T, dir, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ROMArchivesFile), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// With no file the list is the standard one, with no warning, and what comes back is a copy.
func TestLoadROMArchivesWithoutAFileIsTheStandardList(t *testing.T) {
	list, warning := loadROMArchives(t.TempDir())
	if warning != "" || !reflect.DeepEqual(list, defaultROMArchives) {
		t.Fatalf("no file: %+v, %q", list, warning)
	}
	list[0].Label = "changed"
	if defaultROMArchives[0].Label == "changed" {
		t.Error("the standard list was handed out, not a copy of it")
	}
	if list, _ := loadROMArchives(filepath.Join(t.TempDir(), "missing")); !reflect.DeepEqual(list, defaultROMArchives) {
		t.Errorf("a folder that is not there: %+v", list)
	}
}

// The file shipped in custom-roms/ is the standard list, written down.
func TestShippedROMArchivesFileIsTheStandardList(t *testing.T) {
	list, warning := loadROMArchives("custom-roms")
	if warning != "" {
		t.Errorf("the shipped %s has a warning: %s", ROMArchivesFile, warning)
	}
	if !reflect.DeepEqual(list, defaultROMArchives) {
		t.Errorf("the shipped %s is %+v, want %+v", ROMArchivesFile, list, defaultROMArchives)
	}
}

func TestLoadROMArchivesReadsTheFile(t *testing.T) {
	dir := t.TempDir()
	writeArchives(t, dir, `[
		{"text_label": " My ROMs ", "url": " https://example.org/roms "},
		{"text_label": "Plain http", "url": "http://example.net/x?a=1"}
	]`)
	list, warning := loadROMArchives(dir)
	want := []romArchive{{"My ROMs", "https://example.org/roms"}, {"Plain http", "http://example.net/x?a=1"}}
	if warning != "" || !reflect.DeepEqual(list, want) {
		t.Errorf("got %+v, %q; want %+v", list, warning, want)
	}
}

// An entry that cannot be used is left out and counted; if none can be, the standard list is used.
func TestLoadROMArchivesLeavesOutWhatIsUnusable(t *testing.T) {
	dir := t.TempDir()
	writeArchives(t, dir, `[
		{"text_label": "Good", "url": "https://example.org/"},
		{"text_label": "", "url": "https://example.org/nolabel"},
		{"text_label": "File", "url": "file:///etc/passwd"},
		{"text_label": "Script", "url": "javascript:alert(1)"},
		{"text_label": "NoHost", "url": "https://"},
		{"text_label": "NoScheme", "url": "example.org"},
		{"text_label": "Missing"}
	]`)
	list, warning := loadROMArchives(dir)
	if want := []romArchive{{"Good", "https://example.org/"}}; !reflect.DeepEqual(list, want) {
		t.Errorf("got %+v, want %+v", list, want)
	}
	if !strings.Contains(warning, "6 entries") {
		t.Errorf("the warning is %q", warning)
	}

	writeArchives(t, dir, `[{"text_label": "Only", "url": "ftp://example.org/"}, {"text_label": "", "url": "https://x.org/"}]`)
	list, warning = loadROMArchives(dir)
	if !reflect.DeepEqual(list, defaultROMArchives) || warning == "" {
		t.Errorf("every entry unusable: %+v, %q", list, warning)
	}

	writeArchives(t, dir, `[{"text_label": "A", "url": "https://a.org/"}, {"url": "https://b.org/"}]`)
	if _, warning = loadROMArchives(dir); !strings.Contains(warning, "1 entry") {
		t.Errorf("one entry left out: the warning is %q", warning)
	}
}

// A file that is not the list, and one that cannot be read, give the standard list and say so.
func TestLoadROMArchivesWithAFileThatIsNoListIsTheStandardList(t *testing.T) {
	for name, text := range map[string]string{
		"not json": "this is not json",
		"object":   `{"text_label": "A", "url": "https://a.org/"}`,
		"strings":  `["https://a.org/"]`,
		"empty":    ``,
	} {
		dir := t.TempDir()
		writeArchives(t, dir, text)
		list, warning := loadROMArchives(dir)
		if !reflect.DeepEqual(list, defaultROMArchives) || warning == "" {
			t.Errorf("%s: %+v, %q", name, list, warning)
		}
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ROMArchivesFile), 0o755); err != nil { // a folder, which cannot be read as a file
		t.Fatal(err)
	}
	if list, warning := loadROMArchives(dir); !reflect.DeepEqual(list, defaultROMArchives) || warning == "" {
		t.Errorf("a folder in the file's place: %+v, %q", list, warning)
	}
}

// An empty list is a list of nothing, not a missing file.
func TestLoadROMArchivesEmptyListIsNothing(t *testing.T) {
	dir := t.TempDir()
	writeArchives(t, dir, "[]")
	if list, warning := loadROMArchives(dir); len(list) != 0 || warning != "" {
		t.Errorf("an empty list: %+v, %q", list, warning)
	}
}

func TestROMArchivesStampChangesWithTheFile(t *testing.T) {
	dir := t.TempDir()
	absent := romArchivesStamp(dir)
	writeArchives(t, dir, "[]")
	one := romArchivesStamp(dir)
	writeArchives(t, dir, `[{"text_label": "A", "url": "https://a.org/"}]`)
	two := romArchivesStamp(dir)
	if absent != "absent" || one == absent || two == one {
		t.Errorf("stamps: %q, %q, %q", absent, one, two)
	}
}
