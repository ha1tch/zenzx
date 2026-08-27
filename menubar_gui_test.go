//go:build !headless

package main

import (
	"testing"
	"time"

	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
	"github.com/ha1tch/zenzx/pkg/zenui"
)

func TestEaseInOutCubicEndpoints(t *testing.T) {
	if got := easeInOutCubic(0); got != 0 {
		t.Errorf("easeInOutCubic(0) = %v, want 0", got)
	}
	if got := easeInOutCubic(1); got != 1 {
		t.Errorf("easeInOutCubic(1) = %v, want 1", got)
	}
	if got := easeInOutCubic(0.5); got != 0.5 {
		t.Errorf("easeInOutCubic(0.5) = %v, want 0.5 (symmetric midpoint)", got)
	}
}

func TestEaseInOutCubicMonotonic(t *testing.T) {
	prev := float32(-1)
	for i := 0; i <= 20; i++ {
		t2 := float32(i) / 20
		got := easeInOutCubic(t2)
		if got < prev {
			t.Fatalf("easeInOutCubic not monotonic: t=%v got %v, previous was %v", t2, got, prev)
		}
		prev = got
	}
}

func TestEaseInOutCubicSlowAtEnds(t *testing.T) {
	// "Ease in, ease out" means progress near the endpoints should be
	// slower than linear -- e.g. at t=0.1 (10% through), less than 10% of
	// the distance should have been covered.
	if got := easeInOutCubic(0.1); got >= 0.1 {
		t.Errorf("easeInOutCubic(0.1) = %v, want < 0.1 (should ease in, not move at linear rate yet)", got)
	}
	if got := easeInOutCubic(0.9); got <= 0.9 {
		t.Errorf("easeInOutCubic(0.9) = %v, want > 0.9 (should already be easing out toward 1)", got)
	}
}

func TestBarIndexConstants(t *testing.T) {
	// appMenuBar.actions and the two-step Custom ROM handling in Update
	// both key off these exact values matching the widget's own Items
	// slice order (Machine, Custom ROM, Tape, Floppy Disk, Snapshot,
	// View) -- a reordering of either side without updating the other
	// would silently misdirect every selection.
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"barMachine", barMachine, 0},
		{"barCustomROM", barCustomROM, 1},
		{"barTape", barTape, 2},
		{"barFloppyDisk", barFloppyDisk, 3},
		{"barSnapshot", barSnapshot, 4},
		{"barView", barView, 5},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// testZX builds a minimal *ZenZX for tests that only need a valid
// zx.display to construct an appMenuBar -- no ROM load required, since
// NewZenZX already initialises display directly.
func testZX() *ZenZX {
	return NewZenZX(AudioBackendOto)
}

func TestNewAppMenuBarActionCounts(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// Machine: Reset, Pause/Resume only -- model switching now happens
	// via the Standard/Spanish/Timex submenus (dispatched separately,
	// not through b.actions), not one flat action per model.
	wantMachine := 2
	if got := len(b.actions[barMachine]); got != wantMachine {
		t.Errorf("len(actions[barMachine]) = %d, want %d", got, wantMachine)
	}

	for _, bar := range []int{barTape, barFloppyDisk, barSnapshot, barView} {
		if len(b.actions[bar]) == 0 {
			t.Errorf("actions[%d] is empty -- every populated menu should have at least one action", bar)
		}
	}

	// Custom ROM has no static actions table -- it's handled by the
	// dedicated two-step logic in Update instead.
	if _, ok := b.actions[barCustomROM]; ok {
		t.Error("actions[barCustomROM] should not be set -- Custom ROM uses dedicated handling, not the flat actions table")
	}
}

func TestApplyThemeUpdatesThemeAndName(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.ApplyTheme(zenui.ThemeLight)
	if b.themeName != zenui.ThemeLight {
		t.Errorf("themeName = %s, want %s", b.themeName, zenui.ThemeLight)
	}
	if b.theme != zenui.LightTheme() {
		t.Errorf("theme = %+v, want LightTheme() %+v", b.theme, zenui.LightTheme())
	}
}

func TestApplyFontUpdatesFontNameAndFreesOldTexture(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	originalText := b.text
	if err := b.ApplyFont(fonts.NameCozette); err != nil {
		t.Fatalf("ApplyFont(Cozette): %v", err)
	}
	if b.fontName != fonts.NameCozette {
		t.Errorf("fontName = %s, want %s", b.fontName, fonts.NameCozette)
	}
	if b.text == originalText {
		t.Error("ApplyFont should replace b.text with a new instance, not reuse the old one")
	}
	if b.renderer.Text != b.text {
		t.Error("b.renderer.Text should point at the newly-applied font, not the old one")
	}
}

func TestApplyFontUnknownNameLeavesCurrentFontUnchanged(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	originalText := b.text
	originalName := b.fontName
	if err := b.ApplyFont(fonts.Name("NotARealFont")); err == nil {
		t.Fatal("ApplyFont with an unrecognised name should return an error")
	}
	if b.fontName != originalName {
		t.Errorf("fontName changed to %s after a failed ApplyFont -- should stay %s", b.fontName, originalName)
	}
	if b.text != originalText {
		t.Error("b.text was replaced despite ApplyFont failing -- the old font/textures should be left untouched")
	}
}

func TestThemeAndFontMenusPopulated(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if got := len(b.actions[barTheme]); got != len(zenui.Themes) {
		t.Errorf("len(actions[barTheme]) = %d, want %d (one per zenui.Themes entry)", got, len(zenui.Themes))
	}
	// +1 for the separator between the font names and the zoom levels
	// (a placeholder action, never actually invoked).
	wantFontActions := len(fonts.All) + 1 + len(zoomLevels)
	if got := len(b.fontActions); got != wantFontActions {
		t.Errorf("len(fontActions) = %d, want %d (one per fonts.All entry, one separator placeholder, one per zoomLevels entry)", got, wantFontActions)
	}
}

func TestUpdateSkipsWidgetInteractionWhileAnimating(t *testing.T) {
	// appMenuBar.Update reads raylib's global mouse position (rl.GetMouseX/Y),
	// not an injectable Input, so this can't simulate "mouse hovering a
	// specific label mid-animation" the way zenui.MenuBar's own tests can.
	// What it does verify: the actual fix (the `if b.state != barShown`
	// gate) doesn't panic and genuinely skips widget interaction -- the
	// widget stays inactive and no action fires -- while the bar is
	// mid-slide, which is the state the reported bug ("menu floating
	// mid-air, bar not yet unrolled") depended on reaching at all.
	for _, state := range []barState{barHidden, barSlidingIn, barSlidingOut} {
		b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
		if err != nil {
			t.Fatalf("newAppMenuBar: %v", err)
		}
		b.state = state
		b.stateChangedAt = time.Now()

		b.Update(nil) // zx unused on every path Update can reach in this state

		if b.widget.Active() {
			t.Errorf("state=%v: widget became Active() during Update, want interaction skipped entirely", state)
		}
		b.text.Unload()
	}
}

func TestUpdateAllowsWidgetInteractionWhenShown(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}

	b.state = barShown
	b.progress = 1

	// No panic and no crash reaching refreshCustomROMItems and the
	// widget.Update call itself is the meaningful assertion here -- see
	// the mid-animation test above for why simulating an actual hover
	// isn't possible from this environment.
	b.Update(zx)
}

func TestApplyZoomUpdatesScale(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.ApplyZoom(3)
	if b.scale != 3 {
		t.Errorf("b.scale = %d, want 3", b.scale)
	}

	// zenui.MenuBar's own scale field isn't visible from package main
	// (it's an internal detail of pkg/zenui), so this can't also assert
	// b.widget picked the change up directly -- that propagation is
	// covered independently by pkg/zenui's own
	// TestMenuBarSetScaleAppliesToNextOpen, which ApplyZoom's
	// implementation (a direct call to b.widget.SetScale) relies on.
}

func TestZoomActionsApplyCorrectLevel(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// The last len(zoomLevels) actions in Font's own action list should
	// be the zoom actions, in zoomLevels' own order -- each one,
	// invoked on b itself, should set b.scale to its own corresponding
	// level. A mismatch here means a zoom label and its action fell
	// out of sync.
	list := b.fontActions
	offset := len(list) - len(zoomLevels)
	for i, want := range zoomLevels {
		idx := offset + i
		list[idx](nil) // zx unused by a zoom action
		if b.scale != want {
			t.Errorf("fontActions[%d] (zoom entry %d) set scale to %d, want %d", idx, i, b.scale, want)
		}
	}
}

func TestNewAppMenuBarUsesRequestedInitialTheme(t *testing.T) {
	for _, name := range zenui.Themes {
		b, err := newAppMenuBar(testZX(), "./custom-roms", name, "48k", nil, "")
		if err != nil {
			t.Fatalf("newAppMenuBar(%s): %v", name, err)
		}
		if b.themeName != name {
			t.Errorf("themeName = %s, want %s", b.themeName, name)
		}
		want := zenui.LoadTheme(name)
		if b.theme != want {
			t.Errorf("theme for %s = %+v, want %+v (zenui.LoadTheme's own value, not zenui.DefaultTheme -- the previously-shipped startup bug)", name, b.theme, want)
		}
		b.text.Unload()
	}
}

func TestParseThemeFlag(t *testing.T) {
	cases := []struct {
		raw  string
		want zenui.ThemeName
	}{
		{"Dark", zenui.ThemeDark},
		{"dark", zenui.ThemeDark},
		{"LIGHT", zenui.ThemeLight},
		{"Spectrum", zenui.ThemeSpectrum},
		{"spectrum", zenui.ThemeSpectrum},
		{"SPECTRUM", zenui.ThemeSpectrum},
		{"  Dark  ", zenui.ThemeDark}, // normalize's ReplaceAll strips every space, leading/trailing/internal alike
	}
	for _, c := range cases {
		if got := parseThemeFlag(c.raw); got != c.want {
			t.Errorf("parseThemeFlag(%q) = %s, want %s", c.raw, got, c.want)
		}
	}
}

func TestParseThemeFlagUnknownFallsBackToDark(t *testing.T) {
	if got := parseThemeFlag("NotARealTheme"); got != zenui.ThemeDark {
		t.Errorf("parseThemeFlag(unknown) = %s, want %s", got, zenui.ThemeDark)
	}
}

func TestParseThemeFlagOldSpectrum128FormNoLongerRecognised(t *testing.T) {
	// The Spectrum theme's own name changed from "Spectrum 128" to
	// "Spectrum" -- confirms this is a real, deliberate behaviour
	// change, not just stale identifier text: the old form is simply
	// unrecognised now, falling back to Dark the same as any other
	// typo, rather than still resolving to ThemeSpectrum.
	if got := parseThemeFlag("Spectrum 128"); got != zenui.ThemeDark {
		t.Errorf("parseThemeFlag(\"Spectrum 128\") = %s, want %s (the old form is no longer valid)", got, zenui.ThemeDark)
	}
}

func TestFloppyDiskMenuHasOpenDSKImage(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// 6 real actions (Open, Insert Blank, Save, Save As, Eject, Info)
	// plus 2 separator placeholders, grouped as [mount] | [save] |
	// [current-disk operations].
	if got := len(b.actions[barFloppyDisk]); got != 8 {
		t.Errorf("len(actions[barFloppyDisk]) = %d, want 8 (6 real actions + 2 separator placeholders)", got)
	}
	items := b.widget.ItemsFor(barFloppyDisk)
	if items[0].Label != "Open DSK Image..." {
		t.Errorf("items[0].Label = %q, want \"Open DSK Image...\"", items[0].Label)
	}
}

func TestOpenDiskDialogRefusesWithoutFDC(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	// 48K has no floppy controller -- io.hasFDC is false by default.

	b.openDiskDialog(zx)
	if b.diskDialog != nil {
		t.Error("openDiskDialog opened a dialog despite zx.io.hasFDC being false")
	}
}

func TestOpenDiskDialogOpensWithFDC(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	zx.io.hasFDC = true // simulate a +3-family model without a full switchModelLive

	b.openDiskDialog(zx)
	if b.diskDialog == nil {
		t.Fatal("openDiskDialog did not open a dialog despite zx.io.hasFDC being true")
	}
	if !b.Active() {
		t.Error("Active() should be true while the disk dialog is open")
	}
}

func TestPrintDiskInfoDoesNotPanic(t *testing.T) {
	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}

	// No FDC.
	printDiskInfo(zx)

	// FDC present, no disk loaded.
	zx.io.hasFDC = true
	printDiskInfo(zx)
}

func TestLastBarLabelIsThemeNotFontAfterFontBecameASubmenu(t *testing.T) {
	// Confirms the gradient overlay/rainbow/logo hot zone -- all
	// anchored to whatever MenuBar.LabelsEndX() reports as the last
	// label, not to "Font" by name -- automatically still work
	// correctly now that Font folded into Theme as a submenu: this
	// bar has exactly 7 top-level labels (Machine, Custom ROM, Tape,
	// Floppy Disk, Snapshot, View, Theme), and index 6 (barTheme)
	// really is the last one, with LabelsEndX reflecting that without
	// any code change being needed in the decorations themselves.
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if got := b.widget.LabelCount(); got != 7 {
		t.Fatalf("LabelCount() = %d, want 7 (Font folded into Theme, no longer its own label)", got)
	}
	if barTheme != 6 {
		t.Fatalf("barTheme = %d, want 6 (the last index, 0-based, of 7 labels)", barTheme)
	}
}

func TestFontIsASubmenuOfThemeNotATopLevelLabel(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	themeItems := b.widget.ItemsFor(barTheme)
	found := false
	for _, it := range themeItems {
		if it.Label == "Font" {
			found = true
			if len(it.SubItems) == 0 {
				t.Error("Theme's own \"Font\" item should have SubItems set")
			}
		}
	}
	if !found {
		t.Error("Theme's dropdown should contain a \"Font\" item")
	}
}

func TestThemeFontItemIndexMatchesDispatch(t *testing.T) {
	// The dispatch in Update computes len(zenui.Themes)+1 as Font's own
	// index within Theme's items -- confirm this actually matches
	// where "Font" really landed at construction (themes, then a
	// separator, then Font), so a future change to either side can't
	// silently drift out of sync.
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barTheme)
	wantIdx := len(zenui.Themes) + 1
	if wantIdx >= len(items) {
		t.Fatalf("computed Font index %d is out of range (len(items)=%d)", wantIdx, len(items))
	}
	if items[wantIdx].Label != "Font" {
		t.Errorf("items[%d].Label = %q, want \"Font\" (len(zenui.Themes)+1 should land exactly on it)", wantIdx, items[wantIdx].Label)
	}
	if !items[len(zenui.Themes)].Separator {
		t.Errorf("items[%d] should be the separator between the theme choices and Font", len(zenui.Themes))
	}
}

func TestFontActionsSelectsCorrectFont(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// Find TomThumb's own index among fontActions via fonts.All's own
	// order (fontActions mirrors it 1:1 for the font entries).
	idx := -1
	for i, name := range fonts.All {
		if name == fonts.NameTomThumb {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("fonts.All does not contain NameTomThumb")
	}
	b.fontActions[idx](zx)
	if *b.fontChecked[fonts.NameTomThumb] != true {
		t.Error("fontChecked[TomThumb] should be true after selecting it via fontActions")
	}
}

func TestTapeMenuHasSeparatorBetweenPlaybackAndSettings(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barTape)
	wantLabels := []string{"Play / Stop", "Rewind", "", "Accurate/Fast Mode", "Show Info"}
	if len(items) != len(wantLabels) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(wantLabels))
	}
	if !items[2].Separator {
		t.Error("items[2] should be the separator between playback and settings/info")
	}
	for i, want := range wantLabels {
		if want == "" {
			continue // the separator slot, already checked above
		}
		if items[i].Label != want {
			t.Errorf("items[%d].Label = %q, want %q", i, items[i].Label, want)
		}
	}
}

func TestFloppyDiskMenuGrouping(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barFloppyDisk)
	wantLabels := []string{
		"Open DSK Image...", "Insert Blank Disk", "",
		"Save Disk", "Save Disk As...", "",
		"Eject Disk", "Disk Info",
	}
	if len(items) != len(wantLabels) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(wantLabels))
	}
	for i, want := range wantLabels {
		if want == "" {
			if !items[i].Separator {
				t.Errorf("items[%d] should be a separator", i)
			}
			continue
		}
		if items[i].Label != want {
			t.Errorf("items[%d].Label = %q, want %q", i, items[i].Label, want)
		}
	}
}

func TestFloppyDiskActionsMatchReorderedItems(t *testing.T) {
	// Confirms b.actions[barFloppyDisk] wasn't left in the old order
	// after diskItems was reorganised -- each real action index should
	// invoke the handler matching that position's own label.
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	zx.io.EnableFDC() // properly initializes zx.io.fdc (not just hasFDC), needed since this test actually dereferences it via HasDisk()

	// index 1 = "Insert Blank Disk"
	b.actions[barFloppyDisk][1](zx)
	if !zx.io.fdc.HasDisk() {
		t.Error("actions[barFloppyDisk][1] should insert a blank disk, but none is loaded")
	}
}

func TestSnapshotMenuGrouping(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barSnapshot)
	wantLabels := []string{"Quick Save", "Quick Load", "Save Timestamped", "", "Snapshot Info", "Run Diagnostics"}
	if len(items) != len(wantLabels) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(wantLabels))
	}
	if !items[3].Separator {
		t.Error("items[3] should be the separator between the save operations and info/diagnostics")
	}
	for i, want := range wantLabels {
		if want == "" {
			continue
		}
		if items[i].Label != want {
			t.Errorf("items[%d].Label = %q, want %q", i, items[i].Label, want)
		}
	}
}

func TestNewAppMenuBarNilSettingsUsesHardcodedDefaults(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if b.fontName != fonts.NameSinclair {
		t.Errorf("fontName = %s, want %s (nil settings should fall back to the original hardcoded default)", b.fontName, fonts.NameSinclair)
	}
	if b.scale != 2 {
		t.Errorf("scale = %d, want 2 (nil settings should fall back to the original hardcoded default)", b.scale)
	}
	if b.fixed {
		t.Error("fixed = true, want false (nil settings should fall back to the original hardcoded default)")
	}
}

func TestNewAppMenuBarUsesProvidedInitialSettings(t *testing.T) {
	s := &settingsconfig.Settings{
		Version:      1,
		Theme:        "Dark",
		Font:         "TomThumb",
		FontZoom:     3,
		DisplayScale: 4,
		FixedMenuBar: true,
	}
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", s, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if b.fontName != fonts.NameTomThumb {
		t.Errorf("fontName = %s, want %s", b.fontName, fonts.NameTomThumb)
	}
	if b.scale != 3 {
		t.Errorf("scale = %d, want 3", b.scale)
	}
	if !b.fixed {
		t.Error("fixed = false, want true")
	}
}

func TestNewAppMenuBarFontCheckmarkMatchesActualInitialFont(t *testing.T) {
	// Regression guard: an earlier version of this constructor hardcoded
	// the font checkmark to fonts.NameSinclair regardless of which font
	// was actually loaded from initialSettings, so a non-Sinclair
	// initial font would load correctly but show the wrong checkmark.
	s := &settingsconfig.Settings{
		Version: 1, Theme: "Dark", Font: "TomThumb", FontZoom: 2, DisplayScale: 2, FixedMenuBar: false,
	}
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", s, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if checked, ok := b.fontChecked[fonts.NameTomThumb]; !ok || !*checked {
		t.Error("TomThumb should be checked, since it's the actual initial font")
	}
	if checked, ok := b.fontChecked[fonts.NameSinclair]; ok && *checked {
		t.Error("Sinclair should not be checked -- TomThumb is the actual initial font, not Sinclair")
	}
}

func TestSaveSettingsIsNoopWithoutSettingsPath(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// Should not panic or error -- settingsPath is "" (the default for
	// nearly every test's own construction), so this must be a genuine
	// no-op.
	b.saveSettings(zx)
}

func TestSaveSettingsWritesActualCurrentState(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.json"

	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, path)
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.ApplyTheme(zenui.ThemeSpectrum)
	b.scale = 3
	b.fixed = true
	b.saveSettings(zx)

	res, err := settingsconfig.Load(path, []byte(`{"version":1,"theme":"Dark","font":"Sinclair","fontZoom":2,"displayScale":2,"fixedMenuBar":false}`), validThemeNames, validFontNames)
	if err != nil {
		t.Fatalf("settingsconfig.Load: %v", err)
	}
	if !res.FromDisk {
		t.Fatal("expected the just-saved file to be used")
	}
	if res.Settings.Theme != string(zenui.ThemeSpectrum) {
		t.Errorf("saved Theme = %q, want %q", res.Settings.Theme, string(zenui.ThemeSpectrum))
	}
	if res.Settings.FontZoom != 3 {
		t.Errorf("saved FontZoom = %d, want 3", res.Settings.FontZoom)
	}
	if !res.Settings.FixedMenuBar {
		t.Error("saved FixedMenuBar = false, want true")
	}
}
