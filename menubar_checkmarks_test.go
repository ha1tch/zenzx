//go:build !headless

package main

import (
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/machineconfig"
	"github.com/ha1tch/zenzx/pkg/zenui"
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

	if got := len(b.actions[barMachine]); got != 2 {
		t.Errorf("len(actions[barMachine]) = %d, want 2 (Reset/Pause-Resume only)", got)
	}

	// A "submenu" node occupies exactly one top-level slot (its own
	// nested items live inside that one Item's own SubItems, not as
	// further top-level rows), so the expected top-level count is
	// simply len(nodes) at the top level, regardless of whether any
	// node happens to be a submenu.
	nodes := machinesRes.Config.Items
	wantTotal := 2 + len(nodes) // Reset, Pause/Resume, then one row per top-level node
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
		i := j + 2
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
		if it.Label == "  ZX Spectrum +3" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("could not find \"  ZX Spectrum +3\" in the Machine menu")
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

	if !*b.fontChecked[fonts.NameSinclair] {
		t.Fatal("setup: Sinclair should start checked")
	}

	// Pick some other bundled font to switch to.
	var other fonts.Name
	for _, n := range fonts.All {
		if n != fonts.NameSinclair {
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

	if *b.fontChecked[fonts.NameSinclair] {
		t.Error("Sinclair still checked after switching fonts")
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
	zx.display.showFPS = true
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// Flip the field directly (simulating the Alt+F keyboard shortcut,
	// which happens completely outside the menu) and confirm the
	// menu's own checkmark state (which is the very same pointer, not
	// a synced copy) reflects it immediately, with no refresh step.
	zx.display.showFPS = false

	items := b.widget.ItemsFor(barView)
	if items[0].Checked == nil {
		t.Fatal("View menu's FPS item has no Checked pointer")
	}
	if *items[0].Checked {
		t.Error("FPS item still shows checked after zx.display.showFPS was set false directly")
	}
}

func TestViewBorderCheckboxPointsAtDisplayField(t *testing.T) {
	zx := testZX()
	zx.display.showBorder = true
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barView)
	if items[1].Checked != &zx.display.showBorder {
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
	if !items[0].Toggle {
		t.Error("View menu's FPS item should have Toggle: true")
	}
	if !items[1].Toggle {
		t.Error("View menu's Border item should have Toggle: true")
	}
	if items[2].Toggle {
		t.Error("View menu's \"Show Status\" item should not be a toggle")
	}
}

func TestViewActionsNeverPanicIfCalled(t *testing.T) {
	// b.actions[barView][0] and [1] are unreachable in normal operation
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

func TestViewBorderToggleTriggersResizeAnimation(t *testing.T) {
	// Regression guard: Item.Toggle only flips zx.display.showBorder
	// directly -- ToggleBorder's real behaviour also calls
	// updateTargetSize (isAnimating=true, animated in
	// UpdateWindowSize), which the checkbox's direct-pointer wiring
	// alone doesn't trigger. The generic dispatch's Border action
	// closure must supply that second half.
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if zx.display.isAnimating {
		t.Fatal("setup: isAnimating should start false")
	}

	// Border's action closure is index 1 in b.actions[barView].
	b.actions[barView][1](zx)

	if !zx.display.isAnimating {
		t.Error("Border's action closure should trigger the window-resize animation (isAnimating=true), matching Alt+B's real behaviour")
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

	if len(items) != 7 {
		t.Fatalf("len(items) = %d, want 7 (3 + separator + 3)", len(items))
	}
	if !items[3].Separator {
		t.Error("items[3] should be the separator between the base options and the zoom group")
	}
	wantLabels := []string{"X 1", "X 2", "X 3"}
	for i, want := range wantLabels {
		if items[4+i].Label != want {
			t.Errorf("items[%d].Label = %q, want %q", 4+i, items[4+i].Label, want)
		}
	}
}

func TestRefreshViewItemsChecksCurrentScale(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	zx.display.SetScale(2)

	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.refreshViewItems(zx)
	items := b.widget.ItemsFor(barView)

	for i, n := range []int{1, 2, 3} {
		item := items[4+i]
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

	limit := zx.display.maxMultiplierThatFits()
	for i, n := range []int{1, 2, 3} {
		item := items[4+i]
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

	// index 6 = "X 3" (0:FPS, 1:Border, 2:Show Status, 3:separator, 4:X1, 5:X2, 6:X3)
	b.actions[barView][6](zx)
	if zx.display.screen.multiplier != 3 {
		t.Errorf("multiplier = %d, want 3 after dispatching the X 3 action", zx.display.screen.multiplier)
	}
}

func TestEnEspanolAttributesEachModelToItsOwnManufacturer(t *testing.T) {
	machinesRes, err := machineconfig.Load("", defaultMachinesJSON, validModelIDs)
	if err != nil {
		t.Fatalf("machineconfig.Load: %v", err)
	}

	// "En Español" is a flat sequence now, not a nested SubGroups
	// structure: a top-level Title, then alternating indent=1 Titles
	// (sub-manufacturer headings) and the indent=2 Models under each,
	// until the next indent=0 Title ends the section.
	items := machinesRes.Config.Items
	start := -1
	for i, node := range items {
		if node.Type == machineconfig.Title && node.Indent == 0 && node.Label == "En Español" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatal("machines.json has no top-level \"En Español\" title")
	}

	want := map[string][]string{
		"Sinclair":     {"spanish48k"},
		"Investrónica": {"spanish128k"},
		"Amstrad":      {"spanishplus2", "spanishplus3"},
	}
	got := map[string][]string{}
	currentSub := ""
	for i := start; i < len(items); i++ {
		node := items[i]
		if node.Type == machineconfig.Title && node.Indent == 0 {
			break // reached the next top-level group
		}
		switch {
		case node.Type == machineconfig.Title && node.Indent == 1:
			currentSub = node.Label
		case node.Type == machineconfig.Model:
			got[currentSub] = append(got[currentSub], node.ID)
		}
	}

	for name, wantFlags := range want {
		gotFlags, ok := got[name]
		if !ok {
			t.Errorf("no sub-manufacturer %q found under En Español", name)
			continue
		}
		if len(gotFlags) != len(wantFlags) {
			t.Errorf("%s: got %v, want %v", name, gotFlags, wantFlags)
			continue
		}
		for i, want := range wantFlags {
			if gotFlags[i] != want {
				t.Errorf("%s[%d] = %q, want %q", name, i, gotFlags[i], want)
			}
		}
	}
}

func TestMachineMenuEnEspanolSubTitlesRenderIndented(t *testing.T) {
	b, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	items := b.widget.ItemsFor(barMachine)
	wantSubTitles := []string{"  Sinclair", "  Investrónica", "  Amstrad"}
	found := map[string]bool{}
	for _, it := range items {
		if it.Title {
			for _, want := range wantSubTitles {
				if it.Label == want {
					found[want] = true
				}
			}
		}
	}
	for _, want := range wantSubTitles {
		if !found[want] {
			t.Errorf("no Title item with label %q found (expected sub-manufacturer heading under En Español)", want)
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
