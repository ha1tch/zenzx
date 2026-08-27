package zenui

import "testing"

func testBarItems() []MenuBarItem {
	return []MenuBarItem{
		{Label: "File", Items: []Item{{Label: "Open"}, {Label: "Save"}}},
		{Label: "Edit", Items: []Item{{Label: "Undo"}, {Label: "Redo"}}},
	}
}

func TestNewMenuBarNeverNil(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	if mb == nil {
		t.Fatal("NewMenuBar returned nil")
	}
	if mb.Active() {
		t.Error("a freshly constructed MenuBar should not be Active")
	}
}

func TestMenuBarDrawComputesLabelRects(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	if len(mb.labelRects) != 2 {
		t.Fatalf("labelRects has %d entries, want 2", len(mb.labelRects))
	}
	// noopRenderer: TextWidth(s,1) = len(s)*8. "File" -> 4*8=32, +16 = 48.
	want0 := Rect{X: 12, Y: 0, W: 48, H: 24}
	if mb.labelRects[0] != want0 {
		t.Errorf("labelRects[0] = %+v, want %+v", mb.labelRects[0], want0)
	}
	// second label starts where the first one ended: x = 12 + 48 = 60.
	// "Edit" -> 4*8=32, +16 = 48.
	want1 := Rect{X: 60, Y: 0, W: 48, H: 24}
	if mb.labelRects[1] != want1 {
		t.Errorf("labelRects[1] = %+v, want %+v", mb.labelRects[1], want1)
	}
}

func TestMenuBarHoverOpensDropdown(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	// Mouse over the "File" label (rect computed above: X 12-60, Y 0-24).
	mb.Update(Input{MouseX: 30, MouseY: 10})

	if !mb.Active() {
		t.Fatal("hovering a label should open its dropdown -- no click needed")
	}
	if mb.openIndex != 0 {
		t.Errorf("openIndex = %d, want 0 (File)", mb.openIndex)
	}
}

func TestMenuBarHoverSwitchesWithoutClick(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	mb.Update(Input{MouseX: 30, MouseY: 10}) // over "File"
	if mb.openIndex != 0 {
		t.Fatalf("setup: openIndex = %d, want 0", mb.openIndex)
	}

	// Move to "Edit" (rect X 60-108) with no click at all -- should switch
	// directly, matching a traditional menu bar once any dropdown is
	// already engaged.
	mb.Update(Input{MouseX: 80, MouseY: 10})
	if mb.openIndex != 1 {
		t.Errorf("openIndex = %d, want 1 (Edit) -- hover alone should switch once a dropdown is open", mb.openIndex)
	}
	if !mb.Active() {
		t.Error("MenuBar should still be Active after switching labels")
	}
}

func TestMenuBarHoveringSameOpenLabelDoesNotReset(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	mb.Update(Input{MouseX: 30, MouseY: 10})
	firstMenu := mb.menu

	// Still hovering the same label ("File") a second frame -- the
	// underlying *Menu instance should be the same one, not rebuilt
	// (rebuilding would silently discard any keyboard-selected row).
	mb.Update(Input{MouseX: 35, MouseY: 12})
	if mb.menu != firstMenu {
		t.Error("hovering the already-open label again should not rebuild its Menu")
	}
}

func TestMenuBarAcceptReturnsCorrectSelection(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())
	mb.Update(Input{MouseX: 30, MouseY: 10}) // open "File"
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	// Click "Save" (the second item, index 1) within the now-open dropdown.
	itemRect := mb.menu.ItemRect(1)
	clickX := itemRect.X + 1
	clickY := itemRect.Y + 1

	sel, ok := mb.Update(Input{MouseX: clickX, MouseY: clickY, MousePressed: true})
	if !ok {
		t.Fatal("clicking an enabled item should return ok=true")
	}
	want := MenuBarSelection{BarIndex: 0, ItemIndex: 1, SubIndex: -1}
	if sel != want {
		t.Errorf("selection = %+v, want %+v", sel, want)
	}
	if mb.Active() {
		t.Error("MenuBar should not be Active immediately after an accepted selection")
	}
}

func TestMenuBarClose(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())
	mb.Update(Input{MouseX: 30, MouseY: 10})
	if !mb.Active() {
		t.Fatal("setup: expected the dropdown to be open")
	}

	mb.Close()
	if mb.Active() {
		t.Error("Close() should force the bar to Active()==false")
	}
}

func TestMenuBarDrawAtNonZeroY(t *testing.T) {
	// A host animating the bar into view draws it at a shrinking or
	// sliding Y -- labelRects must track that, not assume Y=0.
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, -12, 24, DefaultTheme())

	if mb.labelRects[0].Y != -12 {
		t.Errorf("labelRects[0].Y = %d, want -12 (Draw's barY parameter)", mb.labelRects[0].Y)
	}
}

func TestMenuBarUsesSidebarNotPanel(t *testing.T) {
	r := newDrawRecorder()
	theme := DefaultTheme()
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(r, 800, 600, 0, 24, theme)

	// The very first fill call is the bar's own background.
	if len(*r.calls) == 0 || (*r.calls)[0].kind != "fill" {
		t.Fatalf("expected the bar's background fill as the first call, got %+v", *r.calls)
	}
	got := (*r.calls)[0].col
	if got != theme.Sidebar {
		t.Errorf("bar background = %+v, want theme.Sidebar %+v (not theme.Panel %+v)", got, theme.Sidebar, theme.Panel)
	}
}

func TestMenuBarLabelTextUsesSideText(t *testing.T) {
	r := newDrawRecorder()
	theme := DefaultTheme()
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(r, 800, 600, 0, 24, theme)

	found := false
	for _, c := range *r.calls {
		if c.kind == "text" && c.label == "File" {
			found = true
			if c.col != theme.SideText {
				t.Errorf("unselected label text colour = %+v, want theme.SideText %+v", c.col, theme.SideText)
			}
		}
	}
	if !found {
		t.Fatal("expected a text draw call for the \"File\" label")
	}
}

func TestMenuBarOpenLabelUsesSelTextOverSelFill(t *testing.T) {
	r := newDrawRecorder()
	theme := DefaultTheme()
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(r, 800, 600, 0, 24, theme) // compute labelRects
	mb.Update(Input{MouseX: 30, MouseY: 10})

	r2 := newDrawRecorder()
	mb.Draw(r2, 800, 600, 0, 24, theme)

	var gotFill, gotText Colour
	fillFound, textFound := false, false
	for _, c := range *r2.calls {
		if c.kind == "fill" && c.col == theme.SelFill {
			gotFill = c.col
			fillFound = true
		}
		if c.kind == "text" && c.label == "File" {
			gotText = c.col
			textFound = true
		}
	}
	if !fillFound {
		t.Fatal("expected a SelFill-coloured fill for the open label")
	}
	if !textFound {
		t.Fatal("expected a text draw for the open \"File\" label")
	}
	if gotText != theme.SelText {
		t.Errorf("open label's text colour = %+v, want theme.SelText %+v", gotText, theme.SelText)
	}
	_ = gotFill
}

func TestMenuBarBorderDrawnAfterOpenLabelFill(t *testing.T) {
	r := newDrawRecorder()
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(r, 800, 600, 0, 24, DefaultTheme())
	mb.Update(Input{MouseX: 30, MouseY: 10})

	r2 := newDrawRecorder()
	mb.Draw(r2, 800, 600, 0, 24, DefaultTheme())

	fillIdx, strokeIdx := -1, -1
	for i, c := range *r2.calls {
		if c.kind == "fill" && fillIdx == -1 && i > 0 { // skip the bar's own background fill (call 0)
			fillIdx = i
		}
		if c.kind == "stroke" && strokeIdx == -1 {
			strokeIdx = i
		}
	}
	if fillIdx == -1 || strokeIdx == -1 {
		t.Fatalf("expected both a selection fill and a border stroke, got %+v", *r2.calls)
	}
	if strokeIdx < fillIdx {
		t.Errorf("bar border (stroke, call %d) drawn before the open label's fill (call %d) -- should be after", strokeIdx, fillIdx)
	}
}

func TestMenuBarScaleThreadsToOpenedDropdown(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems(), Scale: 3})
	mb.Draw(newDrawRecorder(), 800, 600, 0, 24, DefaultTheme())
	mb.Update(Input{MouseX: 30, MouseY: 10}) // opens "File"

	if mb.menu == nil {
		t.Fatal("expected a dropdown to have opened")
	}
	if mb.menu.scale != 3 {
		t.Errorf("opened dropdown's scale = %d, want 3 (from MenuBarConfig.Scale)", mb.menu.scale)
	}
}

func TestMenuBarSetScaleAppliesToNextOpen(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.SetScale(3)
	mb.Draw(newDrawRecorder(), 800, 600, 0, 24, DefaultTheme())
	mb.Update(Input{MouseX: 30, MouseY: 10})

	if mb.menu == nil {
		t.Fatal("expected a dropdown to have opened")
	}
	if mb.menu.scale != 3 {
		t.Errorf("opened dropdown's scale = %d, want 3 (from SetScale)", mb.menu.scale)
	}
}

func TestMenuBarLabelsEndX(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(newDrawRecorder(), 800, 600, 0, 24, DefaultTheme())

	// testBarItems(): "File" (48px) then "Edit" (48px), starting at x=12
	// each -- see TestMenuBarDrawComputesLabelRects for the same maths.
	// LabelsEndX should be exactly where the second label's rect ends.
	want := mb.labelRects[len(mb.labelRects)-1].X + mb.labelRects[len(mb.labelRects)-1].W
	if got := mb.LabelsEndX(); got != want {
		t.Errorf("LabelsEndX() = %d, want %d (end of the last label's rect)", got, want)
	}
}

func TestMenuBarSkipsOwnBackgroundFillWhenGradientActive(t *testing.T) {
	r := newDrawRecorder()
	theme := DefaultTheme()
	theme.UseBarGradient = true

	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(r, 800, 600, 0, 24, theme)

	for _, c := range *r.calls {
		if c.kind == "fill" && c.col == theme.Sidebar {
			t.Errorf("MenuBar.Draw filled with theme.Sidebar (%+v) despite UseBarGradient=true -- should skip its own flat fill entirely", theme.Sidebar)
		}
	}
}

func TestMenuBarDrawsOwnBackgroundFillWhenGradientInactive(t *testing.T) {
	r := newDrawRecorder()
	theme := DefaultTheme() // UseBarGradient: false

	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(r, 800, 600, 0, 24, theme)

	found := false
	for _, c := range *r.calls {
		if c.kind == "fill" && c.col == theme.Sidebar {
			found = true
		}
	}
	if !found {
		t.Error("expected a Sidebar-coloured background fill when UseBarGradient=false, found none")
	}
}

func TestMenuBarAcceptFromSubmenuPopulatesSubIndex(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: []MenuBarItem{
		{Label: "View", Items: []Item{
			{Label: "Zoom", SubItems: []Item{{Label: "X1"}, {Label: "X2"}, {Label: "X3"}}},
		}},
	}})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())
	mb.Update(Input{MouseX: 30, MouseY: 10}) // open "View"
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	parentRect := mb.menu.ItemRect(0)                                    // "Zoom"
	mb.Update(Input{MouseX: parentRect.X + 1, MouseY: parentRect.Y + 1}) // hover opens submenu
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())             // layout the submenu

	subRect := mb.menu.subMenu.ItemRect(2) // "X3"
	sel, ok := mb.Update(Input{MouseX: subRect.X + 1, MouseY: subRect.Y + 1, MousePressed: true})

	if !ok {
		t.Fatal("clicking a submenu item should return ok=true")
	}
	want := MenuBarSelection{BarIndex: 0, ItemIndex: 0, SubIndex: 2}
	if sel != want {
		t.Errorf("selection = %+v, want %+v", sel, want)
	}
}

func TestMenuBarAcceptWithoutSubmenuHasSubIndexMinusOne(t *testing.T) {
	// Regression guard: Menu.subResult previously defaulted to Go's
	// zero value (0) rather than -1, so a plain item at index 0 could
	// be mistaken for "submenu item 0 was chosen" by anything checking
	// SubIndex >= 0.
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())
	mb.Update(Input{MouseX: 30, MouseY: 10}) // open "File"
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	itemRect := mb.menu.ItemRect(0) // "Open", index 0 -- the regression-triggering index
	sel, ok := mb.Update(Input{MouseX: itemRect.X + 1, MouseY: itemRect.Y + 1, MousePressed: true})

	if !ok {
		t.Fatal("clicking an enabled item should return ok=true")
	}
	if sel.SubIndex != -1 {
		t.Errorf("SubIndex = %d, want -1 (no submenu was involved)", sel.SubIndex)
	}
}

func TestMenuBarToggledDoesNotCloseDropdown(t *testing.T) {
	checked := false
	mb := NewMenuBar(MenuBarConfig{Items: []MenuBarItem{
		{Label: "View", Items: []Item{{Label: "FPS", Checked: &checked, Toggle: true}}},
	}})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())
	mb.Update(Input{MouseX: 30, MouseY: 10}) // open "View"
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	rec := mb.menu.ItemRect(0)
	sel, ok := mb.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1, MousePressed: true})

	if !ok {
		t.Fatal("Update should return ok=true for a Toggled selection, not just Accepted")
	}
	if sel.BarIndex != 0 || sel.ItemIndex != 0 {
		t.Errorf("selection = %+v, want BarIndex=0 ItemIndex=0", sel)
	}
	if !checked {
		t.Error("the toggle's own bool should have flipped")
	}
	if !mb.Active() {
		t.Error("MenuBar should still be Active after a toggle -- the dropdown must stay open")
	}
}

func TestMenuBarDrawsGradientOverlayWhenThemeRequestsIt(t *testing.T) {
	theme := SpectrumTheme()
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})

	r := newDrawRecorder()
	mb.Draw(r, 800, 600, 0, 24, theme)

	found := false
	for _, c := range *r.calls {
		if c.kind == "gradientHMultiply" {
			found = true
			if c.col != theme.GradientOverlayLeft {
				t.Errorf("overlay left colour = %+v, want %+v", c.col, theme.GradientOverlayLeft)
			}
		}
	}
	if !found {
		t.Error("expected a gradientHMultiply draw call, found none")
	}
}

func TestMenuBarSkipsGradientOverlayWhenThemeDoesNotRequestIt(t *testing.T) {
	theme := DarkTheme()
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})

	r := newDrawRecorder()
	mb.Draw(r, 800, 600, 0, 24, theme)

	for _, c := range *r.calls {
		if c.kind == "gradientHMultiply" {
			t.Error("unexpected gradientHMultiply call for a theme with UseGradientOverlay=false")
		}
	}
}

func TestMenuBarGradientOverlayDrawnBeforeLabels(t *testing.T) {
	// Confirms the ordering that keeps labels from being darkened: the
	// overlay call must appear before any label's own fill/text calls
	// in the recorded call sequence.
	theme := SpectrumTheme()
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})

	r := newDrawRecorder()
	mb.Draw(r, 800, 600, 0, 24, theme)

	overlayIdx, firstTextIdx := -1, -1
	for i, c := range *r.calls {
		if c.kind == "gradientHMultiply" && overlayIdx == -1 {
			overlayIdx = i
		}
		if c.kind == "text" && firstTextIdx == -1 {
			firstTextIdx = i
		}
	}
	if overlayIdx == -1 {
		t.Fatal("no gradientHMultiply call found")
	}
	if firstTextIdx == -1 {
		t.Fatal("no text call found")
	}
	if overlayIdx >= firstTextIdx {
		t.Errorf("overlay drawn at index %d, first label text at %d -- overlay should come first", overlayIdx, firstTextIdx)
	}
}

func TestSetItemsOnOpenDropdownPreservesItemRects(t *testing.T) {
	// Regression guard for a real bug: SetItems used to recreate the
	// open dropdown's Menu from scratch (via openLabel) every time it
	// was called, wiping itemRects (only repopulated by the next Draw)
	// -- fatal for a host that calls SetItems every frame a menu might
	// be open (refreshCustomROMItems/refreshViewItems in zenzx's own
	// menubar_gui.go), since hit-testing during that same frame's own
	// Update would always miss against the freshly-emptied itemRects,
	// making the dropdown feel completely unresponsive the whole time
	// it was open.
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())
	mb.openLabel(0) // opens "File"
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	before := mb.menu.ItemRect(0)
	if before == (Rect{}) {
		t.Fatal("setup: itemRects should be populated after Draw")
	}

	// Call SetItems repeatedly, as a per-frame refresh would, without
	// an intervening Draw -- the buggy version would leave itemRects
	// empty at this point since openLabel's fresh Menu hadn't been
	// drawn yet.
	for i := 0; i < 3; i++ {
		mb.SetItems(0, []Item{{Label: "Open"}, {Label: "Save"}})
	}

	after := mb.menu.ItemRect(0)
	if after == (Rect{}) {
		t.Error("itemRects should still be populated immediately after SetItems, without needing another Draw first")
	}
	if after != before {
		t.Errorf("ItemRect(0) changed from %+v to %+v after SetItems with identical items -- should be stable", before, after)
	}
}

func TestSetItemsOnOpenDropdownHitTestingStillWorks(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())
	mb.openLabel(0)
	mb.Draw(noopRenderer{}, 800, 600, 0, 24, DefaultTheme())

	rec := mb.menu.ItemRect(0) // "Open"

	// Simulate a per-frame refresh call, then immediately try to click
	// the item, in the same "frame" -- no Draw in between, matching
	// how Update (refresh, then widget.Update) actually runs.
	mb.SetItems(0, []Item{{Label: "Open"}, {Label: "Save"}})

	sel, ok := mb.Update(Input{MouseX: rec.X + 1, MouseY: rec.Y + 1, MousePressed: true})
	if !ok {
		t.Fatal("click on \"Open\" should have registered as Accepted, but the selection was rejected")
	}
	if sel.ItemIndex != 0 {
		t.Errorf("ItemIndex = %d, want 0 (\"Open\")", sel.ItemIndex)
	}
}

func TestSetItemsOnClosedDropdownDoesNotOpenIt(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	mb.SetItems(0, []Item{{Label: "New"}})
	if mb.Active() {
		t.Error("SetItems on a closed dropdown should not open it")
	}
	if got := mb.ItemsFor(0); len(got) != 1 || got[0].Label != "New" {
		t.Errorf("ItemsFor(0) = %+v, want a single \"New\" item -- SetItems should still update cfg.Items even when closed", got)
	}
}
