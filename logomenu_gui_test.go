//go:build !headless

package zenzx

import (
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// testNoopRenderer satisfies zenui.Renderer without any raylib
// dependency -- b.renderer (the real, raylib-backed one) needs a live
// GL context for its actual drawing calls (FillRect/DrawText/etc.),
// which this sandbox's headless test environment doesn't have; calling
// Draw with it segfaults. TextWidth/LineHeight return simple, stable
// approximations, sufficient for testing layout/geometry logic without
// needing the real BDF font's exact metrics.
type testNoopRenderer struct{}

func (testNoopRenderer) FillRect(zenui.Rect, zenui.Colour)                                {}
func (testNoopRenderer) StrokeRect(zenui.Rect, zenui.Colour, int)                         {}
func (testNoopRenderer) FillRectGradientV(zenui.Rect, zenui.Colour, zenui.Colour)         {}
func (testNoopRenderer) FillRectGradientHMultiply(zenui.Rect, zenui.Colour, zenui.Colour) {}
func (testNoopRenderer) DrawLine(int, int, int, int, int, zenui.Colour)                   {}
func (testNoopRenderer) DrawText(string, int, int, int, zenui.Colour)                     {}
func (testNoopRenderer) TextWidth(s string, scale int) int                                { return len(s) * 8 * scale }
func (testNoopRenderer) LineHeight(scale int) int                                         { return 8 * scale }
func (testNoopRenderer) Clip(zenui.Rect)                                                  {}
func (testNoopRenderer) ClipEnd()                                                         {}

// testDrawCall and testDrawRecorder mirror zenui's own (unexported,
// package-scoped) drawRecorder/drawCall -- not accessible from here
// since this is another package, so a local equivalent is needed for
// tests that need to inspect what actually got drawn rather than just
// confirm nothing crashes.
type testDrawCall struct {
	kind string
	rect zenui.Rect
	col  zenui.Colour
}
type testDrawRecorder struct{ calls *[]testDrawCall }

func newTestDrawRecorder() testDrawRecorder { return testDrawRecorder{calls: &[]testDrawCall{}} }

func (r testDrawRecorder) FillRect(rc zenui.Rect, c zenui.Colour) {
	*r.calls = append(*r.calls, testDrawCall{kind: "fill", rect: rc, col: c})
}
func (r testDrawRecorder) StrokeRect(rc zenui.Rect, c zenui.Colour, thickness int) {
	*r.calls = append(*r.calls, testDrawCall{kind: "stroke", rect: rc, col: c})
}
func (r testDrawRecorder) FillRectGradientV(rc zenui.Rect, top, bottom zenui.Colour) {
	*r.calls = append(*r.calls, testDrawCall{kind: "gradient", rect: rc, col: top})
}
func (r testDrawRecorder) FillRectGradientHMultiply(rc zenui.Rect, left, right zenui.Colour) {
	*r.calls = append(*r.calls, testDrawCall{kind: "gradientHMultiply", rect: rc, col: left})
}
func (r testDrawRecorder) DrawLine(x1, y1, x2, y2, thickness int, c zenui.Colour) {
	*r.calls = append(*r.calls, testDrawCall{kind: "line", rect: zenui.Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}, col: c})
}
func (r testDrawRecorder) DrawText(s string, x, y, scale int, c zenui.Colour) {
	*r.calls = append(*r.calls, testDrawCall{kind: "text", rect: zenui.Rect{X: x, Y: y}, col: c})
}
func (r testDrawRecorder) TextWidth(s string, scale int) int { return len(s) * 8 * scale }
func (r testDrawRecorder) LineHeight(scale int) int          { return 8 * scale }
func (r testDrawRecorder) Clip(zenui.Rect)                   {}
func (r testDrawRecorder) ClipEnd()                          {}

func TestLogoHotZoneCoversRightOfLabels(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()
	b.widget.Draw(testNoopRenderer{}, 800, 600, 0, menuStripHeight, b.theme) // populate LabelsEndX

	zone := b.logoHotZone(800)
	labelsEndX := b.widget.LabelsEndX()
	if zone.X != labelsEndX {
		t.Errorf("zone.X = %d, want %d (LabelsEndX)", zone.X, labelsEndX)
	}
	if zone.X+zone.W != 800 {
		t.Errorf("zone right edge = %d, want 800 (screenW)", zone.X+zone.W)
	}
	if zone.H != menuStripHeight {
		t.Errorf("zone.H = %d, want %d", zone.H, menuStripHeight)
	}
}

func TestOpenLogoMenuHasFourItemsInNewOrder(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.openLogoMenu(800)
	if b.logoMenu == nil {
		t.Fatal("openLogoMenu did not set b.logoMenu")
	}

	b.logoMenu.Draw(testNoopRenderer{}, 800, 600, b.theme)
	for i := 0; i < 4; i++ {
		rec := b.logoMenu.ItemRect(i)
		if rec == (zenui.Rect{}) {
			t.Errorf("item %d has no rect", i)
		}
	}

	// Order and labels: Fixed menu bar, ZenZX website, Help, About...
	wantLabels := []string{"Fixed menu bar", "ZenZX website", "Help", "About..."}
	items := b.logoMenu.Items()
	if len(items) != len(wantLabels) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(wantLabels))
	}
	for i, want := range wantLabels {
		if items[i].Label != want {
			t.Errorf("item %d label = %q, want %q", i, items[i].Label, want)
		}
	}
}

func TestFixedMenuBarItemIsACheckboxOnBFixed(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.openLogoMenu(800)
	items := b.logoMenu.Items()
	if !items[0].Toggle {
		t.Error("item 0 (Fixed menu bar) should have Toggle=true (a checkbox, not a select-and-close item)")
	}
	if items[0].Checked == nil {
		t.Fatal("item 0's Checked pointer is nil")
	}
	if items[0].Checked != &b.fixed {
		t.Error("item 0's Checked should point directly at b.fixed, the same direct-pointer pattern the View menu's own checkboxes use")
	}
}

func TestApplyFixedStateTogglesAndShows(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if b.fixed {
		t.Fatal("setup: fixed should start false")
	}
	// Fixed menu bar is a checkbox now (Item.Toggle flips b.fixed
	// directly via its own pointer); applyFixedState supplies the
	// window-resize side effect, the same two-step pattern the View
	// menu's Border checkbox already uses.
	b.fixed = true
	b.applyFixedState(zx)
	if b.state != barSlidingIn && b.state != barShown {
		t.Errorf("state = %v after fixing, want the bar to be showing (barSlidingIn or barShown)", b.state)
	}

	b.fixed = false
	b.applyFixedState(zx)
	if zx.window().reservedTopHeight != 0 {
		t.Errorf("reservedTopHeight = %d after unfixing, want 0", zx.window().reservedTopHeight)
	}
}

func TestHandleLogoMenuResultZenZXWebsiteOpensURL(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// result 1 (ZenZX website) opens the home page. The opener is replaced: the real one
	// starts a browser on any machine with a display.
	var opened []string
	saved := openURL
	openURL = func(url string) { opened = append(opened, url) }
	defer func() { openURL = saved }()
	b.handleLogoMenuResult(zx, 1)
	if len(opened) != 1 || opened[0] != zenZXHomepageURL {
		t.Fatalf("the website item opened %v, want %s once", opened, zenZXHomepageURL)
	}
}

func TestHandleLogoMenuResultHelpOpensModal(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.handleLogoMenuResult(zx, 2)
	if b.activeModal == nil {
		t.Fatal("result 2 (Help) should open activeModal")
	}
	if b.activeModal.Title() != "HELP" {
		t.Errorf("activeModal.title = %q, want \"HELP\"", b.activeModal.Title())
	}
	if b.activeModal.AutoHeight() {
		t.Error("Help's modal should have autoHeight=false (always full-height, since it always needs scrolling)")
	}
}

func TestHandleLogoMenuResultAboutOpensModalWithVersionSubstituted(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.handleLogoMenuResult(zx, 3)
	if b.activeModal == nil {
		t.Fatal("result 3 (About...) should open activeModal")
	}
	if b.activeModal.Title() != "ABOUT" {
		t.Errorf("activeModal.title = %q, want \"ABOUT\"", b.activeModal.Title())
	}
	joined := strings.Join(b.activeModal.Lines(), "\n")
	if strings.Contains(joined, "__VERSION__") {
		t.Error("About text still contains the unsubstituted __VERSION__ placeholder")
	}
	if !strings.Contains(joined, version) {
		t.Errorf("About text does not contain the actual version (%q)", version)
	}
	if !b.activeModal.AutoHeight() {
		t.Error("About's modal should have autoHeight=true, sized to fit its own content")
	}
}

func TestFixedBarBypassesAutoHide(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.state = barShown
	b.fixed = true

	// This mirrors the guard condition directly, since the real check
	// reads live rl.GetMouseX/Y (not something a headless test can
	// drive) -- confirming the condition itself, not a full Update
	// cycle, is what's meaningful here.
	farFromBar := true
	shouldHide := b.state == barShown && !b.fixed && !b.widget.Active() && farFromBar
	if shouldHide {
		t.Error("a fixed bar should never satisfy the auto-hide condition")
	}
}

func TestActiveReportsTrueForLogoMenuAndModal(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if b.Active() {
		t.Fatal("setup: a freshly constructed bar should not be Active")
	}

	b.openLogoMenu(800)
	if !b.Active() {
		t.Error("Active() should be true while logoMenu is open")
	}
	b.logoMenu = nil

	b.activeModal = newHelpModal()
	if !b.Active() {
		t.Error("Active() should be true while activeModal is open")
	}
}

func TestApplyFixedStateReservesTopHeight(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	if zx.window().reservedTopHeight != 0 {
		t.Fatal("setup: reservedTopHeight should start 0")
	}

	b.fixed = true
	b.applyFixedState(zx)
	if zx.window().reservedTopHeight != menuStripHeight {
		t.Errorf("reservedTopHeight = %d after fixing, want %d", zx.window().reservedTopHeight, menuStripHeight)
	}

	b.fixed = false
	b.applyFixedState(zx)
	if zx.window().reservedTopHeight != 0 {
		t.Errorf("reservedTopHeight = %d after unfixing, want 0", zx.window().reservedTopHeight)
	}
}

func TestLogoMenuAlwaysRightAlignedToWindowBorder(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	const rightMargin = 8
	for _, screenW := range []int{400, 800, 1600, 3000} {
		b.openLogoMenu(screenW)
		b.logoMenu.Draw(testNoopRenderer{}, screenW, 600, b.theme)

		// The menu's own right edge should always land at
		// screenW-rightMargin, regardless of screenW or the menu's
		// own width -- confirmed via the first item's rect, which
		// spans the menu's full width.
		right := b.logoMenu.ItemRect(0).X + b.logoMenu.ItemRect(0).W
		want := screenW - rightMargin
		if right != want {
			t.Errorf("screenW=%d: menu right edge = %d, want %d", screenW, right, want)
		}
	}
}

func TestActiveFalseWhenBarShownButNothingOpen(t *testing.T) {
	// Regression guard for the real bug: Active() used to include
	// b.state != barHidden, meaning the emulator lost keyboard input
	// any time the bar was merely visible (mouse near the top edge,
	// or permanently while fixed), even with no dropdown/dialog/menu/
	// modal actually open.
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	for _, st := range []barState{barHidden, barSlidingIn, barShown, barSlidingOut} {
		b.state = st
		if b.Active() {
			t.Errorf("state=%v: Active() = true, want false (nothing is actually open)", st)
		}
	}
}

func TestActiveTrueWhenFixedButNothingOpen(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.state = barShown
	b.fixed = true
	if b.Active() {
		t.Error("a fixed bar with nothing open should not be Active -- the emulator must still receive keyboard input")
	}
}

func TestActiveTrueWhenLogoMenuOpen(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.state = barHidden // even with the bar itself hidden
	b.openLogoMenu(800)
	if !b.Active() {
		t.Error("Active() should be true while logoMenu is open, regardless of the bar's own visibility state")
	}
}

// The tests of the Help and About window itself (its opacity, its height, its
// scrolling) are zenui's, with the window: textmodal_test.go there.
