//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// restoreMicrodigital puts the program-wide switch back after a test that turns it.
func restoreMicrodigital(t *testing.T) {
	t.Helper()
	was := MicrodigitalPortuguese()
	t.Cleanup(func() { SetMicrodigitalLanguage(was) })
}

// The Preferences window has a row for the switch, with Spanish first, and turning it turns the
// switch the ULA port reads, saves the choice, and reads back on the next start.
func TestPreferencesLanguageSwitchRowTurnsTheSwitchAndSaves(t *testing.T) {
	restoreMicrodigital(t)
	SetMicrodigitalLanguage(false)
	path := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, &settingsconfig.Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 2,
		DisplayScale: 2}, path)

	row := prefRowNamed(t, b, zx, "Language switch")
	if len(row.options) != 2 || row.options[0] != "Spanish" || row.options[1] != "Portuguese" {
		t.Fatalf("the row offers %q", row.options)
	}
	if row.get() != 0 {
		t.Errorf("the switch starts at %d, want Spanish (0)", row.get())
	}
	row.set(1)
	if !MicrodigitalPortuguese() || row.get() != 1 {
		t.Error("choosing Portuguese did not turn the switch")
	}
	b.saveSettings(zx)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"microdigital": "portuguese"`) {
		t.Errorf("the choice was not saved:\n%s", data)
	}

	// the next start reads it back
	again, _ := newTestBar(t, &settingsconfig.Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 2,
		DisplayScale: 2, Microdigital: "portuguese"}, path)
	if !again.microdigitalNow() {
		t.Error("a saved Portuguese reads back as Spanish")
	}

	row.set(0)
	if MicrodigitalPortuguese() {
		t.Error("choosing Spanish left the switch at Portuguese")
	}
}

// -language-switch decides for one session and is never written to the file; the window's
// own choice ends its say.
func TestLanguageSwitchFlagOverridesOneSessionAndIsNotSaved(t *testing.T) {
	restoreMicrodigital(t)
	path := filepath.Join(t.TempDir(), "settings.json")
	b, zx := newTestBar(t, &settingsconfig.Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 2,
		DisplayScale: 2}, path)
	pt := true
	b.microdigitalFlag = &pt
	if !b.microdigitalNow() {
		t.Fatal("with -language-switch=portuguese the switch reads Spanish")
	}
	b.saveSettings(zx)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "portuguese") || !strings.Contains(string(data), `"microdigital": "spanish"`) {
		t.Errorf("the switch was written to the file, or the saved choice lost:\n%s", data)
	}
	prefRowNamed(t, b, zx, "Language switch").set(0)
	if b.microdigitalNow() || b.microdigitalFlag != nil {
		t.Errorf("after the window changed it: Portuguese %v, the switch still stands: %v", b.microdigitalNow(), b.microdigitalFlag != nil)
	}
}
