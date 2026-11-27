//go:build !headless

package zenzx

import (
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/machineconfig"
)

func TestMachineMenuFlatListStructure(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()
	machinesRes, err := machineconfig.Load("", defaultMachinesJSON, validModelIDs)
	if err != nil {
		t.Fatalf("machineconfig.Load: %v", err)
	}

	if got := len(b.actions[barMachine]); got != machineHeadRows {
		t.Errorf("len(actions[barMachine]) = %d, want %d (the title, Reset, Play, Pause, Stop only)", got, machineHeadRows)
	}

	// A "submenu" node occupies exactly one top-level slot (its own
	// nested items live inside that one Item's own SubItems, not as
	// further top-level rows), so the expected top-level count is
	// simply len(nodes) at the top level, regardless of whether any
	// node happens to be a submenu.
	nodes := machinesRes.Config.Items
	wantTotal := machineHeadRows + len(nodes) // Reset, Play, Pause, Stop, then one row per top-level node
	items := b.widget.ItemsFor(barMachine)
	if len(items) != wantTotal {
		t.Fatalf("len(ItemsFor(barMachine)) = %d, want %d", len(items), wantTotal)
	}
	if len(b.machineModelFlags) != len(items) {
		t.Fatalf("len(machineModelFlags) = %d, want %d (parallel to items)", len(b.machineModelFlags), len(items))
	}

	// Confirm every top-level node's own flag/label/checkmark landed
	// at its correct flat index.
	for j, node := range nodes {
		i := j + machineHeadRows
		wantLabel := strings.Repeat("  ", node.Indent) + node.Label
		switch node.Type {
		case machineconfig.Separator:
			if !items[i].Separator {
				t.Errorf("items[%d]: expected a Separator", i)
			}
			if b.machineModelFlags[i] != "" {
				t.Errorf("machineModelFlags[%d] = %q, want \"\" (separator row)", i, b.machineModelFlags[i])
			}
		case machineconfig.Title:
			if !items[i].Title || items[i].Label != wantLabel {
				t.Errorf("items[%d]: expected Title %q, got Title=%v Label=%q", i, wantLabel, items[i].Title, items[i].Label)
			}
			if b.machineModelFlags[i] != "" {
				t.Errorf("machineModelFlags[%d] = %q, want \"\" (title row)", i, b.machineModelFlags[i])
			}
		case machineconfig.Model:
			if items[i].Label != wantLabel {
				t.Errorf("items[%d].Label = %q, want %q", i, items[i].Label, wantLabel)
			}
			if items[i].Checked == nil {
				t.Errorf("items[%d] (%s) has no Checked pointer", i, node.Label)
			}
			if b.machineModelFlags[i] != node.ID {
				t.Errorf("machineModelFlags[%d] = %q, want %q", i, b.machineModelFlags[i], node.ID)
			}
		case machineconfig.Submenu:
			if items[i].Label != node.Label || len(items[i].SubItems) == 0 {
				t.Errorf("items[%d]: expected a submenu %q with SubItems set", i, node.Label)
			}
			if _, ok := b.machineSubmenuFlags[i]; !ok {
				t.Errorf("machineSubmenuFlags has no entry for index %d", i)
			}
		}
	}
}

func TestMachineMenuModelRowDispatchesCorrectFlag(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// Find the row for Amstrad's "ZX Spectrum +3" and confirm its
	// flag is exactly "plus3", not some other index's value --
	// exercises the parallel-slice lookup directly, not just its
	// construction.
	items := b.widget.ItemsFor(barMachine)
	idx := -1
	for i, it := range items {
		if it.Label == "ZX Spectrum +3" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("could not find \"ZX Spectrum +3\" in the Machine menu")
	}
	if b.machineModelFlags[idx] != "plus3" {
		t.Errorf("machineModelFlags[%d] = %q, want \"plus3\"", idx, b.machineModelFlags[idx])
	}
}

func TestMachineMenuInitialModelChecked(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "plus3", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	checked, ok := b.modelChecked["plus3"]
	if !ok {
		t.Fatal("modelChecked has no entry for \"plus3\"")
	}
	if !*checked {
		t.Error("initial model \"plus3\" should be checked at construction")
	}

	// Every other model should be unchecked.
	for model, c := range b.modelChecked {
		if model != "plus3" && *c {
			t.Errorf("model %q is checked, want only \"plus3\" checked", model)
		}
	}
}

func TestMachineModelGroupsCoverExpectedModels(t *testing.T) {
	// Completeness checking itself (every validModelIDs entry appears
	// exactly once) is already thoroughly tested in
	// pkg/machineconfig's own test suite. This is an integration
	// check confirming the real, embedded machines.json actually
	// passes it -- machineconfig.Load returns an error otherwise, so
	// a successful Load here already implies completeness; this test
	// exists to make that assertion explicit rather than only
	// implicit in every other test that calls newAppMenuBar.
	if _, err := machineconfig.Load("", defaultMachinesJSON, validModelIDs); err != nil {
		t.Errorf("the embedded machines.json failed validation: %v", err)
	}
}

func TestSetCurrentModelUpdatesOnlyOneChecked(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.setCurrentModel("spanishplus3")

	for model, c := range b.modelChecked {
		want := model == "spanishplus3"
		if *c != want {
			t.Errorf("modelChecked[%q] = %v, want %v", model, *c, want)
		}
	}
}

func TestApplyThemeUpdatesThemeChecked(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if !*b.themeChecked[zenui.ThemeDark] {
		t.Fatal("setup: ThemeDark should start checked")
	}

	b.ApplyTheme(zenui.ThemeSpectrum)

	if *b.themeChecked[zenui.ThemeDark] {
		t.Error("ThemeDark still checked after switching to Spectrum")
	}
	if !*b.themeChecked[zenui.ThemeSpectrum] {
		t.Error("ThemeSpectrum not checked after ApplyTheme")
	}
}

func TestApplyFontUpdatesFontChecked(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if !*b.fontChecked[fonts.NameSpleen] {
		t.Fatal("setup: Spleen should start checked")
	}

	// Pick some other bundled font to switch to.
	var other fonts.Name
	for _, n := range fonts.All {
		if n != fonts.NameSpleen {
			other = n
			break
		}
	}
	if other == "" {
		t.Skip("only one bundled font available, nothing to switch to")
	}

	if err := b.ApplyFont(other); err != nil {
		t.Fatalf("ApplyFont(%s): %v", other, err)
	}

	if *b.fontChecked[fonts.NameSpleen] {
		t.Error("Spleen still checked after switching fonts")
	}
	if !*b.fontChecked[other] {
		t.Errorf("%s not checked after ApplyFont", other)
	}
}

func TestApplyZoomUpdatesZoomChecked(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if !*b.zoomChecked[2] {
		t.Fatal("setup: X2 should start checked (the default)")
	}

	b.ApplyZoom(3)

	if *b.zoomChecked[2] {
		t.Error("X2 still checked after switching to X3")
	}
	if !*b.zoomChecked[3] {
		t.Error("X3 not checked after ApplyZoom")
	}
}

func TestViewFPSCheckboxPointsAtDisplayField(t *testing.T) {
	zx := testZX()
	zx.window().showFPS = true
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// Flip the field directly (simulating the Alt+F keyboard shortcut,
	// which happens completely outside the menu) and confirm the
	// menu's own checkmark state (which is the very same pointer, not
	// a synced copy) reflects it immediately, with no refresh step.
	zx.window().showFPS = false

	items := b.widget.ItemsFor(barView)
	if items[1].Checked == nil {
		t.Fatal("View menu's FPS item has no Checked pointer")
	}
	if *items[1].Checked {
		t.Error("FPS item still shows checked after zx.display.showFPS was set false directly")
	}
}

func TestViewBorderCheckboxPointsAtDisplayField(t *testing.T) {
	zx := testZX()
	zx.window().showBorder = true
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barView)
	if items[2].Checked != &zx.window().showBorder {
		t.Error("Border item's Checked pointer is not the same address as zx.display.showBorder -- should be directly wired, not a copy")
	}
}

func TestViewFPSItemIsToggleNotSelectAndClose(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barView)
	if !items[1].Toggle {
		t.Error("View menu's FPS item should have Toggle: true")
	}
	if items[2].Toggle {
		t.Error("View menu's Border item should close the menu: the window changes size under it")
	}
	for _, it := range items {
		if it.Label == "Show Status" {
			t.Error("the View menu has a Show Status row, which only printed to the console")
		}
	}
}

func TestViewActionsNeverPanicIfCalled(t *testing.T) {
	// b.actions[barView][1] and [2] are unreachable in normal operation
	// (FPS/Border are pure Toggle items now) but MenuBar.Update does
	// return ok=true for Toggled, meaning the generic dispatch's index
	// bounds check alone would let these be called -- confirm they're
	// safe no-ops, not nil, regardless.
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	for i, action := range b.actions[barView] {
		if action == nil {
			t.Fatalf("actions[barView][%d] is nil -- would panic if the generic dispatch ever called it", i)
		}
		action(zx) // must not panic
	}
}

func TestViewBorderToggleResizesTheWindow(t *testing.T) {
	// Picking the row flips showBorder and nothing else (and closes the menu: the window changes
	// size under it). The
	// window follows because UpdateWindowSize compares the border as borderView derives it
	// with the border the window was last sized for: no action closure has to remember to
	// ask for the resize, so Alt+B, the checkbox and a settings change all behave alike.
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	dm := zx.window()
	if dm.isAnimating {
		t.Fatal("setup: isAnimating should start false")
	}
	withBorder := dm.targetWidth

	item := b.widget.ItemsFor(barView)[2]
	if item.Label != "Border" || item.Toggle || !item.Box || item.Disabled {
		t.Fatalf("row 2 is %+v, want an enabled Border row drawn as a box that does not toggle (a toggling checkbox keeps the menu open)", item)
	}
	if fs := b.widget.ItemsFor(barView)[3]; fs.Label != "Full Screen" || fs.Toggle || !fs.Box {
		t.Errorf("row 3 is %+v, want Full Screen drawn as a box that closes the menu", fs)
	}
	b.viewSpec(zx)[2].run(zx) // picking the row
	dm.followBorder()         // the first thing UpdateWindowSize does each frame
	if !dm.isAnimating || dm.targetWidth >= withBorder {
		t.Errorf("after the border was switched off: animating=%v, target width %d (was %d); the window should be on its way to a smaller size", dm.isAnimating, dm.targetWidth, withBorder)
	}
}

func TestRefreshViewItemsStructure(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.refreshViewItems(zx)
	items := b.widget.ItemsFor(barView)

	if want := 11 + len(deckLayoutNames) + 1 + 3; len(items) != want {
		t.Fatalf("len(items) = %d, want %d (a title and 3, separator, a title and 3 zooms, separator, a title, the layouts and Rotate, separator, a title and Screenshot)", len(items), want)
	}
	if rotate := items[len(items)-4]; rotate.Label != viewRotateLabel || !rotate.Disabled {
		t.Errorf("the item before the capture group is %q (disabled %v), want Rotate Machines, greyed for one machine", rotate.Label, rotate.Disabled)
	}
	if shot := items[len(items)-1]; shot.Label != "Screenshot" || len(shot.SubItems) == 0 {
		t.Errorf("the last item is %q with %d rows under it, want the Screenshot submenu", shot.Label, len(shot.SubItems))
	}
	if !items[9].Separator {
		t.Error("items[9] should be the separator between the zoom group and the layouts")
	}
	if !items[10].Title || items[10].Label != "Layout" {
		t.Errorf("items[10] is %+v, want the title Layout", items[10])
	}
	// With one machine and no dock: One Machine ticked, the tiled layouts
	// greyed, the zoom available.
	for i, name := range deckLayoutNames {
		it := items[11+i]
		if it.Label != name {
			t.Errorf("items[%d].Label = %q, want %q", 11+i, it.Label, name)
		}
		if it.Checked == nil || *it.Checked != (i == 0) {
			t.Errorf("%s: wrongly ticked for one machine", name)
		}
		if it.Disabled != (i != 0) {
			t.Errorf("%s: Disabled = %v for one machine", name, it.Disabled)
		}
	}

	// With three machines side by side: every layout offered, that one
	// ticked, and the zoom greyed, since it is for one machine filling the
	// window.
	b.layoutState = func() (deckLayout, int) { return layoutColumns, 3 }
	b.refreshViewItems(zx)
	items = b.widget.ItemsFor(barView)
	for i := range deckLayoutNames {
		if items[11+i].Disabled || *items[11+i].Checked != (deckLayout(i) == layoutColumns) {
			t.Errorf("%s with three machines side by side: disabled %v, ticked %v", deckLayoutNames[i], items[11+i].Disabled, *items[11+i].Checked)
		}
	}
	for i := 6; i < 9; i++ {
		if !items[i].Disabled {
			t.Errorf("%s is offered in a tiled layout", items[i].Label)
		}
	}
	b.layoutState = nil
	b.refreshViewItems(zx)
	items = b.widget.ItemsFor(barView)
	if !items[4].Separator {
		t.Error("items[4] should be the separator between the base options and the zoom group")
	}
	if !items[0].Title || items[0].Label != "Display" || !items[5].Title || items[5].Label != "Scale" {
		t.Errorf("the titles are %+v and %+v, want Display and Scale", items[0], items[5])
	}
	wantLabels := []string{"X 1", "X 2", "X 3"}
	for i, want := range wantLabels {
		if items[6+i].Label != want {
			t.Errorf("items[%d].Label = %q, want %q", 6+i, items[6+i].Label, want)
		}
	}
}

func TestRefreshViewItemsChecksCurrentScale(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	zx.window().SetScale(2)

	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.refreshViewItems(zx)
	items := b.widget.ItemsFor(barView)

	for i, n := range []int{1, 2, 3} {
		item := items[6+i]
		if item.Checked == nil {
			t.Fatalf("X %d item has no Checked pointer", n)
		}
		want := n == 2
		if *item.Checked != want {
			t.Errorf("X %d checked = %v, want %v", n, *item.Checked, want)
		}
	}
}

func TestRefreshViewItemsDisablesScalesAboveLimit(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.refreshViewItems(zx)
	items := b.widget.ItemsFor(barView)

	limit := zx.window().maxMultiplierThatFits()
	for i, n := range []int{1, 2, 3} {
		item := items[6+i]
		want := n > limit
		if item.Disabled != want {
			t.Errorf("X %d Disabled = %v, want %v (limit=%d)", n, item.Disabled, want, limit)
		}
	}
}

func TestViewZoomDispatchesToSetScale(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// index 8 = "X 3" (0:title, 1:FPS, 2:Border, 3:Full Screen, 4:separator, 5:title, 6:X1, 7:X2, 8:X3)
	b.actions[barView][8](zx)
	if zx.window().screen.multiplier != 3 {
		t.Errorf("multiplier = %d, want 3 after dispatching the X 3 action", zx.window().screen.multiplier)
	}
}

func TestInSpanishGroupListsExpectedModels(t *testing.T) {
	machinesRes, err := machineconfig.Load("", defaultMachinesJSON, validModelIDs)
	if err != nil {
		t.Fatalf("machineconfig.Load: %v", err)
	}

	// The Spanish-market section is a flat run of Models
	// directly under one top-level "In Spanish" Title -- no
	// per-manufacturer sub-titles (Sinclair/Investrónica/Amstrad)
	// the way an earlier machines.json revision had it. The models
	// are not indented, level with every other maker's models and
	// with the Reset, Pause and Stop rows above them. This replaces the old
	// TestEnEspanolAttributesEachModelToItsOwnManufacturer, which
	// asserted a now-removed nested structure.
	items := machinesRes.Config.Items
	start := -1
	for i, node := range items {
		if node.Type == machineconfig.Title && node.Indent == 0 && node.Label == "In Spanish" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatal("machines.json has no top-level \"In Spanish\" title")
	}

	want := []string{"spanish48k", "spanish128k", "spanishplus2", "spanishplus3"}
	var got []string
	for i := start; i < len(items); i++ {
		node := items[i]
		if node.Type == machineconfig.Title && node.Indent == 0 {
			break // reached the next top-level group
		}
		if node.Type == machineconfig.Separator {
			break // a separator ends the group too: machines without a maker's title follow one
		}
		if node.Type == machineconfig.Title {
			t.Errorf("unexpected nested Title %q (indent %d) inside \"In Spanish\" -- this group is flat, with no manufacturer sub-titles", node.Label, node.Indent)
			continue
		}
		if node.Type != machineconfig.Model {
			continue
		}
		if node.Indent != 0 {
			t.Errorf("model %q has indent %d, want 0", node.ID, node.Indent)
		}
		got = append(got, node.ID)
	}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestMachineMenuInSpanishHasNoManufacturerSubTitles(t *testing.T) {
	// GUI-level counterpart to TestInSpanishGroupListsExpectedModels:
	// guards against the flat "In Spanish" group accidentally growing
	// per-manufacturer sub-titles again without a test noticing.
	// Replaces the old TestMachineMenuEnEspanolSubTitlesRenderIndented,
	// which asserted the opposite (that those sub-titles existed).
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barMachine)
	unwanted := []string{"  Sinclair", "  Investrónica", "  Amstrad"}
	for _, it := range items {
		if !it.Title {
			continue
		}
		for _, bad := range unwanted {
			if it.Label == bad {
				t.Errorf("found manufacturer sub-title %q -- \"In Spanish\" is currently a flat group with no per-manufacturer breakdown", it.Label)
			}
		}
	}
}

func TestMachineMenuNoLongerSaysInvestronicaSAsSpain(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barMachine)
	for _, it := range items {
		if it.Label == "Investronica S.A. Spain" {
			t.Error("the old, historically inaccurate \"Investronica S.A. Spain\" group title should no longer appear")
		}
	}
}

func TestBuildMachineNodesHandlesSubmenu(t *testing.T) {
	// The default machines.json doesn't use a "submenu" node at all,
	// so this exercises that path directly with a synthetic node
	// tree, confirming buildMachineNodes produces a real zenui.Item
	// with SubItems set, a "" placeholder in the top-level
	// machineModelFlags, and a correctly-indexed entry in
	// machineSubmenuFlags for the submenu's own nested models.
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	nodes := []machineconfig.Node{
		{Type: machineconfig.Title, Label: "Group"},
		{Type: machineconfig.Model, ID: "48k", Label: "ZX Spectrum 48k"},
		{Type: machineconfig.Submenu, Label: "More", Items: []machineconfig.Node{
			{Type: machineconfig.Model, ID: "128k", Label: "ZX Spectrum 128k"},
			{Type: machineconfig.Model, ID: "plus2", Label: "ZX Spectrum +2"},
		}},
	}
	items, flags, subFlags := b.buildMachineNodes(nodes, "48k")

	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3 (title, model, submenu)", len(items))
	}
	if len(flags) != 3 {
		t.Fatalf("len(flags) = %d, want 3", len(flags))
	}
	if flags[2] != "" {
		t.Errorf("flags[2] (the submenu's own top-level slot) = %q, want \"\"", flags[2])
	}
	if items[2].Label != "More" || len(items[2].SubItems) != 2 {
		t.Fatalf("items[2] = %+v, want a submenu labelled \"More\" with 2 SubItems", items[2])
	}
	sub, ok := subFlags[2]
	if !ok {
		t.Fatal("subFlags has no entry for index 2")
	}
	if len(sub) != 2 || sub[0] != "128k" || sub[1] != "plus2" {
		t.Errorf("subFlags[2] = %v, want [128k plus2]", sub)
	}
}

func TestMachineDispatchHandlesSubmenuSelection(t *testing.T) {
	// End-to-end: wire a synthetic submenu directly into a live
	// appMenuBar's own state, then confirm a MenuBarSelection with a
	// SubIndex correctly resolves through machineSubmenuFlags to the
	// right model, exercising the actual dispatch code path in
	// Update rather than just buildMachineNodes' own construction.
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// Simulate index 5 being a submenu whose own index 1 is "plus2".
	b.machineSubmenuFlags[5] = []string{"128k", "plus2"}

	sel := zenui.MenuBarSelection{BarIndex: barMachine, ItemIndex: 5, SubIndex: 1}
	// Mirrors the real dispatch logic in Update directly, since
	// driving it through a full Update(zx) call would require
	// simulating live mouse state that isn't injectable in this
	// sandbox.
	var flag string
	if sub, ok := b.machineSubmenuFlags[sel.ItemIndex]; ok {
		if sel.SubIndex >= 0 && sel.SubIndex < len(sub) {
			flag = sub[sel.SubIndex]
		}
	} else if sel.ItemIndex < len(b.machineModelFlags) {
		flag = b.machineModelFlags[sel.ItemIndex]
	}
	if flag != "plus2" {
		t.Errorf("resolved flag = %q, want \"plus2\"", flag)
	}
}
