//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// These tests state the invariants the bar and the display keep, rather than one case of
// each bug that led to them:
//
//   - the border is derived in one place (borderView) and everything that depends on it
//     follows it;
//   - the fixed menu bar's reserved height is derived from the fixed choice, every frame;
//   - the bar shows one thing at a time;
//   - a menu's items and actions are made from one table, so they cannot differ.

// ---- the border ------------------------------------------------------------------

// noBorderRenderer is the standard picture in a mode whose renderer has no border, which
// is what a renderer reports with all-zero margins.
type noBorderRenderer struct{ standardVideoRenderer }

func (noBorderRenderer) BorderMargins() (int, int, int, int) { return 0, 0, 0, 0 }

func newNextDisplay(t *testing.T, layer2Mode uint8, layer2On bool) *DisplayManager {
	t.Helper()
	screen := NewSpectrumScreen()
	dm := NewDisplayManager(screen)
	mem := NewSpectrumMemory(screen)
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	io.SetNextRegister(0x70, layer2Mode<<4)
	if layer2On {
		io.nextRegWriteDirect(0x69, 0x80)
	}
	dm.SetSpectrumIO(io)
	return dm
}

func TestBorderViewCases(t *testing.T) {
	// The ordinary case: the user's choice decides.
	dm := NewDisplayManager(NewSpectrumScreen())
	if v := dm.borderView(); !v.Shown || !v.Choosable {
		t.Errorf("default: %+v, want shown and choosable", v)
	}
	dm.showBorder = false
	if v := dm.borderView(); v.Shown || !v.Choosable {
		t.Errorf("border off by choice: %+v, want hidden and choosable", v)
	}

	// A tiled layout shows every machine with its border whatever the choice was, and
	// the choice is kept for when the layout is left.
	dm.setTiles(640, 336, func() {})
	if v := dm.borderView(); !v.Shown || v.Choosable || v.Why == "" {
		t.Errorf("tiled: %+v, want shown, not choosable, with a reason", v)
	}
	if dm.showBorder {
		t.Error("tiling changed the user's border choice")
	}
	dm.setTiles(0, 0, nil)
	if v := dm.borderView(); v.Shown || !v.Choosable {
		t.Errorf("after the layout: %+v, want the choice (off) back", v)
	}

	// A mode with no border cannot have one, and says it is not shown.
	dm = NewDisplayManager(NewSpectrumScreen())
	dm.SetVideoRenderer(noBorderRenderer{})
	if v := dm.borderView(); v.Shown || v.Choosable {
		t.Errorf("a renderer without a border: %+v, want hidden and not choosable", v)
	}

	// Layer 2's wider modes fill the frame: cutting them to the classic picture would lose
	// the picture, so the border is part of them. The base mode, and Layer 2 off, leave the
	// choice free.
	for _, c := range []struct {
		name      string
		mode      uint8
		on        bool
		choosable bool
	}{
		{"320x256 displayed", 0b01, true, false},
		{"640x256 displayed", 0b10, true, false},
		{"320x256 selected but Layer 2 off", 0b01, false, true},
		{"256x192 displayed", 0b00, true, true},
	} {
		dm := newNextDisplay(t, c.mode, c.on)
		dm.showBorder = false
		v := dm.borderView()
		if v.Choosable != c.choosable || (!c.choosable && !v.Shown) || (c.choosable && v.Shown) {
			t.Errorf("%s: %+v, want choosable=%v", c.name, v, c.choosable)
		}
	}
}

func TestToggleBorderIsRefusedWhereTheBorderIsNotChoosable(t *testing.T) {
	dm := NewDisplayManager(NewSpectrumScreen())
	dm.setTiles(640, 336, func() {})
	dm.ToggleBorder()
	if !dm.showBorder {
		t.Error("Alt+B changed the choice in a tiled layout, where it has no effect")
	}
	dm.setTiles(0, 0, nil)
	dm.ToggleBorder()
	if dm.showBorder {
		t.Error("Alt+B did nothing where the border is the user's to choose")
	}
}

func TestWindowFollowsTheBorderWhateverChangedIt(t *testing.T) {
	// The machine goes into a Layer 2 mode that needs the border while the user's choice
	// is "off": nobody toggled anything, yet the window must grow to the whole frame.
	dm := newNextDisplay(t, 0b00, false) // base mode, Layer 2 off
	dm.showBorder = false
	dm.updateTargetSize()
	small := dm.targetWidth
	if want := dm.screenW * dm.screen.multiplier; small != want {
		t.Fatalf("without the border the window is %d wide, want %d", small, want)
	}
	dm.io.nextRegWriteDirect(0x69, 0x80)
	dm.io.SetNextRegister(0x70, 0b01<<4) // 320x256 displayed
	dm.followBorder()
	if dm.targetWidth <= small {
		t.Errorf("the window stayed %d wide when the mode began to need the border", dm.targetWidth)
	}
	// And it is quiet when nothing changed.
	dm.isAnimating = false
	dm.followBorder()
	if dm.isAnimating {
		t.Error("followBorder restarted the window's animation although nothing changed")
	}
}

func TestViewMenuBorderRowTellsTheTruth(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	border := func() zenui.Item {
		for _, it := range b.viewSpec(zx).items() {
			if it.Label == "Border" {
				return it
			}
		}
		t.Fatal("no Border row")
		return zenui.Item{}
	}
	dm := zx.window()
	if it := border(); it.Disabled || it.Toggle || it.Checked != &dm.showBorder {
		t.Errorf("when the border is the user's to choose the row is ticked by showBorder and closes the menu: %+v", it)
	}
	// Tiled, with the choice off: the border is on the screen, so the row says so, and
	// cannot be changed.
	dm.showBorder = false
	dm.setTiles(640, 336, func() {})
	if it := border(); !it.Disabled || it.Checked == nil || !*it.Checked {
		t.Errorf("tiled: the row should be disabled and ticked (the border is shown): %+v", it)
	}
	// A mode with no border: the row is disabled and not ticked.
	dm.setTiles(0, 0, nil)
	dm.showBorder = true
	dm.SetVideoRenderer(noBorderRenderer{})
	if it := border(); !it.Disabled || it.Checked == nil || *it.Checked {
		t.Errorf("no border in the mode: the row should be disabled and clear: %+v", it)
	}
}

// ---- the fixed menu bar -----------------------------------------------------------

func TestSavedFixedBarReservesItsHeightFromTheFirstFrame(t *testing.T) {
	zx := testZX()
	saved := &settingsconfig.Settings{Version: 1, Theme: "Dark", Font: string(fonts.NameSinclair), FontZoom: 2, DisplayScale: 2, FixedMenuBar: true}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", saved, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	if !b.fixed || zx.window().reservedTopHeight != 0 {
		t.Fatalf("setup: fixed=%v, reserved=%d", b.fixed, zx.window().reservedTopHeight)
	}
	b.reconcileFixed(zx) // what Update does first thing each frame
	if got := zx.window().reservedTopHeight; got != menuStripHeight {
		t.Errorf("a bar saved as fixed reserves %d px, want %d", got, menuStripHeight)
	}
	if b.state != barSlidingIn && b.state != barShown {
		t.Errorf("a fixed bar is on show, but its state is %v", b.state)
	}
}

func TestFixedChoiceIsSavedWhenItChangesAndOnlyThen(t *testing.T) {
	zx := testZX()
	path := filepath.Join(t.TempDir(), "settings.json")
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	b.reconcileFixed(zx)
	if _, err := os.Stat(path); err == nil {
		t.Error("the settings file was written although nothing had changed")
	}
	b.fixed = true // what the checkbox does
	b.reconcileFixed(zx)
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(strings.ReplaceAll(string(data), " ", ""), `"fixedMenuBar":true`) {
		t.Fatalf("the fixed choice was not saved: %v\n%s", err, data)
	}
	os.Remove(path)
	b.reconcileFixed(zx) // nothing changed since
	if _, err := os.Stat(path); err == nil {
		t.Error("the settings were written again with no change")
	}
	b.fixed = false
	b.reconcileFixed(zx)
	if zx.window().reservedTopHeight != 0 {
		t.Errorf("unfixing left %d px reserved", zx.window().reservedTopHeight)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("unfixing was not saved")
	}
}

// ---- one thing at a time ---------------------------------------------------------

// openADropdown has the bar's first dropdown open, as hovering its label does.
func openADropdown(t *testing.T, b *appMenuBar) {
	t.Helper()
	b.widget.Draw(testNoopRenderer{}, 800, 600, 0, menuStripHeight, b.theme) // lays the labels out
	// the pointer goes over the labels until one opens
	for x := 0; x < b.widget.LabelsEndX() && !b.widget.Active(); x += 4 {
		b.widget.Update(zenui.Input{MouseX: x, MouseY: menuStripHeight / 2})
	}
	if !b.widget.Active() {
		t.Fatal("setup: hovering the first label did not open its dropdown")
	}
}

func TestOpeningTheHelpMenuClosesADropdown(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	openADropdown(t, b)
	b.openLogoMenu(800)
	if b.widget.Active() {
		t.Error("a bar dropdown stayed open under the Help menu")
	}
	if b.logoMenu == nil {
		t.Error("the Help menu did not open")
	}
}

func TestEnforceSingleSurface(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	// Whatever opened them, a window has the frame to itself.
	openADropdown(t, b)
	b.logoMenu = zenui.NewMenu(zenui.MenuConfig{Items: b.helpSpec().items()})
	b.activeModal = newHelpModal()
	b.enforceSingleSurface()
	if b.widget.Active() || b.logoMenu != nil {
		t.Errorf("under a window: dropdown open=%v, Help menu open=%v", b.widget.Active(), b.logoMenu != nil)
	}
	if b.activeModal == nil || !b.Active() {
		t.Error("the window itself must stay")
	}
	b.activeModal = nil

	// The Help menu and a dropdown are never both open.
	openADropdown(t, b)
	b.logoMenu = zenui.NewMenu(zenui.MenuConfig{Items: b.helpSpec().items()})
	b.enforceSingleSurface()
	if b.widget.Active() {
		t.Error("a dropdown stayed open beside the Help menu")
	}
	if b.logoMenu == nil {
		t.Error("the Help menu was closed instead of the dropdown")
	}
	// And the Custom ROM two-step is not left half done.
	b.bankMode, b.pendingROM = true, "x.rom"
	openADropdown(t, b)
	b.enforceSingleSurface()
	if b.bankMode || b.pendingROM != "" {
		t.Error("a Custom ROM pick was left waiting for its bank under the Help menu")
	}
}

// ---- items and actions from one table ----------------------------------------------

func TestMenuSpecItemsAndActionsCannotDiffer(t *testing.T) {
	var ran []string
	mark := func(s string) func(*ZenZX) { return func(*ZenZX) { ran = append(ran, s) } }
	on := true
	spec := menuSpec{
		menuTitle("T"),
		menuRow("a", mark("a")),
		menuSep(),
		menuCheckbox("c", &on, mark("c")),
		{label: "off", disabled: true, run: mark("off")},
		{label: "sub", sub: menuSpec{menuRow("s0", mark("s0")), {label: "s1", disabled: true, run: mark("s1")}}},
		menuRow("none", nil),
	}
	items, actions := spec.items(), spec.actions()
	if len(items) != len(spec) || len(actions) != len(spec) {
		t.Fatalf("%d rows, %d items, %d actions", len(spec), len(items), len(actions))
	}
	for i, a := range actions {
		if a == nil {
			t.Fatalf("action %d is nil", i)
		}
		a(nil) // the index dispatch; titles, dividers, disabled rows and submenu parents do nothing
	}
	if strings.Join(ran, ",") != "a,c" {
		t.Errorf("dispatching every action ran %v, want a,c only", ran)
	}
	ran = nil
	for _, at := range [][2]int{{1, -1}, {0, -1}, {2, -1}, {4, -1}, {5, 0}, {5, 1}, {5, -1}, {6, -1}, {99, -1}, {-1, -1}} {
		spec.pick(nil, at[0], at[1])
	}
	if strings.Join(ran, ",") != "a,s0" {
		t.Errorf("pick ran %v, want a,s0 (titles, dividers, disabled rows, bare parents and bad indices run nothing)", ran)
	}
	if len(items[5].SubItems) != 2 || !items[5].SubItems[1].Disabled {
		t.Errorf("the submenu's items: %+v", items[5].SubItems)
	}
}

// TestEveryDispatchedMenuHasOneActionPerRow checks the menus the bar dispatches by index,
// in each state they are rebuilt in.
func TestEveryDispatchedMenuHasOneActionPerRow(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()

	check := func(when string) {
		t.Helper()
		for _, bar := range []int{barTape, barFloppyDisk, barSnapshot, barView} {
			if got, want := len(b.actions[bar]), len(b.widget.ItemsFor(bar)); got != want {
				t.Errorf("%s: menu %d has %d items and %d actions", when, bar, want, got)
			}
		}
	}
	check("at the start")
	b.refreshViewItems(zx)
	check("after the View menu was rebuilt")
	b.layoutState = func() (deckLayout, int) { return layoutGrid, 4 }
	zx.window().setTiles(640, 336, func() {})
	b.refreshViewItems(zx)
	check("in a tiled layout")
	for _, a := range b.actions[barView] {
		a(zx) // none may panic
	}
	if spec := b.helpSpec(); len(spec.items()) != len(spec.actions()) {
		t.Error("the Help menu's items and actions differ")
	}
}
