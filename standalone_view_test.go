package zenzx

import (
	"bytes"
	"testing"

	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// A program carries the window it was exported from: the menus' preferences, the border and the
// FPS counter. A program from the headless ZenZX, or from before this, carries none.

var (
	testThemes = []string{"Dark", "Light", "Spectrum"}
	testFonts  = []string{"Sinclair", "Spleen"}
)

func TestAProgramCarriesTheWindowItWasExportedFrom(t *testing.T) {
	dock := false
	p := testProgram("48k")
	p.View = &ProgramView{
		Settings: &settingsconfig.Settings{Version: 1, Theme: "Light", Font: "Spleen", FontZoom: 3, DisplayScale: 4,
			FixedMenuBar: false, Dock: &dock, TapeDeck: "off"},
		Border: false,
		FPS:    true,
	}
	tail, err := p.payload()
	if err != nil {
		t.Fatal(err)
	}
	file := append(fakeStub(10), tail...)
	got, _, err := readProgram(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	v := got.View
	if v == nil || v.Settings == nil {
		t.Fatalf("the program carries no view: %+v", got)
	}
	if v.Border || !v.FPS {
		t.Errorf("border %v and FPS %v, want the border hidden and the counter shown", v.Border, v.FPS)
	}
	s := v.Settings
	if s.Theme != "Light" || s.Font != "Spleen" || s.FontZoom != 3 || s.DisplayScale != 4 || s.FixedMenuBar || s.DockOn() || s.TapeDeckWord() != "off" {
		t.Errorf("the settings read back are %+v", s)
	}
	if v.checked(testThemes, testFonts) == nil {
		t.Error("settings that were valid when exported are not valid when read")
	}
}

func TestAProgramWithoutAWindowReadsNone(t *testing.T) {
	tail, err := testProgram("128k").payload()
	if err != nil {
		t.Fatal(err)
	}
	file := append(fakeStub(10), tail...)
	got, _, err := readProgram(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	if got.View != nil {
		t.Errorf("a program exported with no window carries one: %+v", got.View)
	}
	if (*ProgramView)(nil).checked(testThemes, testFonts) != nil || (&ProgramView{}).checked(testThemes, testFonts) != nil {
		t.Error("no settings were given, and some came out")
	}
}

// Settings this ZenZX does not know (a theme or a font from another version) are not used: the
// settings of the person who runs the program stand instead, and the border is still honoured.
func TestSettingsThisZenZXDoesNotKnowAreNotUsed(t *testing.T) {
	bad := &ProgramView{Settings: &settingsconfig.Settings{Version: 1, Theme: "Mauve", Font: "Sinclair", FontZoom: 2, DisplayScale: 2}, Border: false}
	if bad.checked(testThemes, testFonts) != nil {
		t.Error("a theme that does not exist was accepted")
	}
	if bad.Border {
		t.Error("the view lost its border")
	}
	worse := &ProgramView{Settings: &settingsconfig.Settings{Version: 1, Theme: "Dark", Font: "Sinclair", FontZoom: 9, DisplayScale: 2}}
	if worse.checked(testThemes, testFonts) != nil {
		t.Error("a zoom of 9 was accepted")
	}
}
