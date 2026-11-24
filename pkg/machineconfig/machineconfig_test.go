package machineconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testModelIDs = []string{"48k", "128k", "plus2"}

func validMinimalJSON() string {
	return `{
		"version": 1,
		"items": [
			{"type": "separator"},
			{"type": "title", "label": "Sinclair"},
			{"type": "model", "id": "48k", "label": "ZX Spectrum 48k"},
			{"type": "model", "id": "128k", "label": "ZX Spectrum 128k"},
			{"type": "separator"},
			{"type": "submenu", "label": "Amstrad", "items": [
				{"type": "model", "id": "plus2", "label": "ZX Spectrum +2"}
			]}
		]
	}`
}

func TestParseAndValidateAcceptsValidConfig(t *testing.T) {
	cfg, err := parseAndValidate([]byte(validMinimalJSON()), testModelIDs)
	if err != nil {
		t.Fatalf("parseAndValidate: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}
	if len(cfg.Items) != 6 {
		t.Errorf("len(Items) = %d, want 6", len(cfg.Items))
	}
}

func TestParseAndValidateRejectsInvalidJSON(t *testing.T) {
	_, err := parseAndValidate([]byte(`{not valid json`), testModelIDs)
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

func TestParseAndValidateRejectsUnknownNodeType(t *testing.T) {
	data := `{"version":1,"items":[{"type":"bogus"}]}`
	_, err := parseAndValidate([]byte(data), testModelIDs)
	if err == nil {
		t.Fatal("expected an error for an unknown node type")
	}
}

func TestParseAndValidateRejectsUnknownModelID(t *testing.T) {
	data := `{"version":1,"items":[{"type":"model","id":"nonexistent","label":"X"}]}`
	_, err := parseAndValidate([]byte(data), testModelIDs)
	if err == nil {
		t.Fatal("expected an error for a model id not in validModelIDs")
	}
}

func TestParseAndValidateRejectsTitleWithoutLabel(t *testing.T) {
	data := `{"version":1,"items":[{"type":"title"}]}`
	_, err := parseAndValidate([]byte(data), testModelIDs)
	if err == nil {
		t.Fatal("expected an error for a title node missing its required label")
	}
}

func TestParseAndValidateRejectsNestedSubmenu(t *testing.T) {
	data := `{"version":1,"items":[
		{"type":"submenu","label":"Outer","items":[
			{"type":"submenu","label":"Inner","items":[
				{"type":"model","id":"48k","label":"X"}
			]}
		]}
	]}`
	_, err := parseAndValidate([]byte(data), testModelIDs)
	if err == nil {
		t.Fatal("expected an error for a submenu nested inside a submenu (only one level is supported)")
	}
}

func TestParseAndValidateRejectsMissingModel(t *testing.T) {
	// Covers 48k and 128k but not plus2.
	data := `{"version":1,"items":[
		{"type":"model","id":"48k","label":"X"},
		{"type":"model","id":"128k","label":"Y"}
	]}`
	_, err := parseAndValidate([]byte(data), testModelIDs)
	if err == nil {
		t.Fatal("expected an error for a missing model id (plus2)")
	}
	if !strings.Contains(err.Error(), "plus2") {
		t.Errorf("error should name the missing id: %v", err)
	}
}

func TestParseAndValidateRejectsDuplicateModel(t *testing.T) {
	data := `{"version":1,"items":[
		{"type":"model","id":"48k","label":"X"},
		{"type":"model","id":"48k","label":"X again"},
		{"type":"model","id":"128k","label":"Y"},
		{"type":"model","id":"plus2","label":"Z"}
	]}`
	_, err := parseAndValidate([]byte(data), testModelIDs)
	if err == nil {
		t.Fatal("expected an error for a duplicated model id (48k)")
	}
	if !strings.Contains(err.Error(), "48k") {
		t.Errorf("error should name the duplicated id: %v", err)
	}
}

func TestCheckCompletenessWalksIntoSubmenus(t *testing.T) {
	// All three ids present, but plus2 only reachable inside a submenu
	// -- confirms checkCompleteness actually recurses rather than only
	// checking the top level.
	items := []Node{
		{Type: Model, ID: "48k"},
		{Type: Model, ID: "128k"},
		{Type: Submenu, Items: []Node{
			{Type: Model, ID: "plus2"},
		}},
	}
	if err := checkCompleteness(items, testModelIDs); err != nil {
		t.Errorf("checkCompleteness should have found plus2 inside the submenu: %v", err)
	}
}

func TestLoadUsesValidDiskFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "machines.json")
	if err := os.WriteFile(path, []byte(validMinimalJSON()), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Load(path, []byte(validMinimalJSON()), testModelIDs)
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
	path := filepath.Join(dir, "machines.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"items":[{"type":"bogus"}]}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Load(path, []byte(validMinimalJSON()), testModelIDs)
	if err != nil {
		t.Fatalf("Load should fall back, not error: %v", err)
	}
	if res.FromDisk {
		t.Error("FromDisk should be false when the disk file is invalid")
	}
	if res.Warning == "" {
		t.Error("Warning should explain why the disk file was rejected")
	}
	if !strings.Contains(res.Warning, path) {
		t.Errorf("Warning should mention the disk path: %q", res.Warning)
	}
}

func TestLoadFallsBackSilentlyOnMissingDiskFile(t *testing.T) {
	res, err := Load("/nonexistent/path/machines.json", []byte(validMinimalJSON()), testModelIDs)
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
	res, err := Load("", []byte(validMinimalJSON()), testModelIDs)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.FromDisk {
		t.Error("FromDisk should be false when diskPath is empty")
	}
	if res.Config == nil {
		t.Fatal("Config should still be populated from embedded")
	}
}

func TestLoadErrorsOnInvalidEmbedded(t *testing.T) {
	_, err := Load("", []byte(`{"version":1,"items":[{"type":"bogus"}]}`), testModelIDs)
	if err == nil {
		t.Fatal("Load should error when even the embedded default is invalid -- a bug in the built-in default should fail loudly")
	}
}
