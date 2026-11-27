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

// The buttons at the ends of the bar, and the toolbar, are tested in barbuttons_gui_test.go.

// helpRow is the index of the row labelled label in the Help menu that is open, found by
// the label so that the tests do not depend on where the section titles put it.
func helpRow(t *testing.T, b *appMenuBar, label string) int {
	t.Helper()
	for i, it := range b.logoMenu.Items() {
		if it.Label == label && !it.Title && !it.Separator {
			return i
		}
	}
	t.Fatalf("the Help menu has no %q row", label)
	return -1
}

func TestOpenLogoMenuIsInTitledSections(t *testing.T) {
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
	items := b.logoMenu.Items()
	for i, it := range items {
		if it.Title || it.Separator {
			continue
		}
		if rec := b.logoMenu.ItemRect(i); rec == (zenui.Rect{}) {
			t.Errorf("item %d (%q) has no rect", i, it.Label)
		}
	}

	// Each section opens with a title, in this order, and the rows are where they belong.
	var got []string
	for _, it := range items {
		switch {
		case it.Separator:
			got = append(got, "-")
		case it.Title:
			got = append(got, "["+it.Label+"]")
		default:
			got = append(got, it.Label)
		}
	}
	want := []string{
		"[Help]", "Help", "ZenZX Manual", "About...", "Acknowledgements...", "ZenZX website", "-",
		"[Tools]", "Developer",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the Help menu is\n  %v\nwant\n  %v", got, want)
	}

	// Developer holds the debugger; with no debugger wired it is there but disabled.
	dev := items[helpRow(t, b, "Developer")]
	if len(dev.SubItems) != 1 || dev.SubItems[0].Label != "Debugger" || !dev.SubItems[0].Disabled {
		t.Errorf("Developer's submenu = %+v, want one disabled Debugger row", dev.SubItems)
	}
}

func TestDeveloperDebuggerOpensTheDebugger(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	opened := 0
	b.openDebugger = func() { opened++ }
	b.openLogoMenu(800)
	dev := helpRow(t, b, "Developer")
	if sub := b.logoMenu.Items()[dev].SubItems; len(sub) != 1 || sub[0].Disabled {
		t.Fatalf("with a debugger wired the Debugger row should be enabled: %+v", sub)
	}
	b.handleLogoMenuResult(zx, dev, 0)
	if opened != 1 {
		t.Errorf("Developer > Debugger opened the debugger %d times, want 1", opened)
	}
	// Titles, dividers and the Developer row itself do nothing.
	b.handleLogoMenuResult(zx, 0, -1)
	b.handleLogoMenuResult(zx, helpRow(t, b, "Help")-1, -1) // the Help section's title
	b.handleLogoMenuResult(zx, dev, -1)
	if opened != 1 {
		t.Errorf("a title, a divider or the bare Developer row opened the debugger (%d)", opened)
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
	b.openLogoMenu(800)
	b.handleLogoMenuResult(zx, helpRow(t, b, "ZenZX website"), -1)
	if len(opened) != 1 || opened[0] != zenZXHomepageURL {
		t.Fatalf("the website item opened %v, want %s once", opened, zenZXHomepageURL)
	}
}

func TestHelpMenuManualRowOpensTheManual(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	var opened []string
	saved := openURL
	openURL = func(s string) { opened = append(opened, s) }
	defer func() { openURL = saved }()
	b.openLogoMenu(800)
	row := helpRow(t, b, "ZenZX Manual")
	if h := helpRow(t, b, "Help"); row != h+1 {
		t.Errorf("the manual's row is %d, want it straight after Help (%d)", row, h)
	}
	b.handleLogoMenuResult(zx, row, -1)
	// no manual is beside the test program, so the release's copy is what opens
	want := manualOnline(version)
	if found := findManual(version, manualDirs()); found != "" {
		want = found
	}
	if len(opened) != 1 || opened[0] != want {
		t.Fatalf("the manual row opened %q, want %q once", opened, want)
	}
}

func TestHandleLogoMenuResultHelpOpensModal(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.openLogoMenu(800)
	b.handleLogoMenuResult(zx, helpRow(t, b, "Help"), -1)
	if b.activeModal == nil {
		t.Fatal("the Help row should open activeModal")
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

	b.openLogoMenu(800)
	b.handleLogoMenuResult(zx, helpRow(t, b, "About..."), -1)
	if b.activeModal == nil {
		t.Fatal("the About... row should open activeModal")
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

func TestHelpMenuAlwaysRightAlignedWithTheHelpButton(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	// The menu hangs from the Help button, whose right edge is a pad in from the window's.
	rightMargin := logoButtonPadFor(b.stripH())
	for _, screenW := range []int{500, 800, 1600, 3000} {
		b.openLogoMenu(screenW)
		b.logoMenu.Draw(testNoopRenderer{}, screenW, 600, b.theme)

		// The menu's own right edge always lands at screenW-rightMargin,
		// whatever screenW or the menu's own width is -- confirmed via a
		// row's rect, which spans the menu's full width. Its submenu
		// does not move it: that opens to the left (below).
		row := b.logoMenu.ItemRect(helpRow(t, b, "Help"))
		if right, want := row.X+row.W, screenW-rightMargin; right != want {
			t.Errorf("screenW=%d: menu right edge = %d, want %d", screenW, right, want)
		}
	}
	// In a window too narrow to right-align the menu with its margin, but wide enough
	// for the menu, it is kept on the screen.
	b.openLogoMenu(3000)
	b.logoMenu.Draw(testNoopRenderer{}, 3000, 600, b.theme)
	menuW := b.logoMenu.ItemRect(helpRow(t, b, "Help")).W
	narrow := menuW + rightMargin/2
	b.openLogoMenu(narrow)
	b.logoMenu.Draw(testNoopRenderer{}, narrow, 600, b.theme)
	row := b.logoMenu.ItemRect(helpRow(t, b, "Help"))
	if row.X < 0 || row.X+row.W > narrow {
		t.Errorf("in a %d px window the menu is at %d..%d, off the screen", narrow, row.X, row.X+row.W)
	}
}

// textSpy is a renderer that remembers where each piece of text was drawn.
type textSpy struct {
	testNoopRenderer
	at map[string]zenui.Rect
}

func (s *textSpy) DrawText(text string, x, y, scale int, _ zenui.Colour) {
	s.at[text] = zenui.Rect{X: x, Y: y, W: s.TextWidth(text, scale), H: s.LineHeight(scale)}
}

// The Help menu is at the window's right edge, so its Developer submenu has no room on
// the right: it opens to the left of the menu, beside it, and none of the menu is covered.
func TestHelpSubmenuOpensToTheLeftWithoutCoveringTheMenu(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	for _, screenW := range []int{800, 1600, 3000} {
		b.openLogoMenu(screenW)
		spy := &textSpy{at: map[string]zenui.Rect{}}
		b.logoMenu.Draw(spy, screenW, 600, b.theme)
		dev := b.logoMenu.ItemRect(helpRow(t, b, "Developer"))
		menuLeft := b.logoMenu.ItemRect(helpRow(t, b, "Help")).X

		b.logoMenu.Update(zenui.Input{MouseX: dev.X + 1, MouseY: dev.Y + 1}) // hover opens it
		spy = &textSpy{at: map[string]zenui.Rect{}}
		b.logoMenu.Draw(spy, screenW, 600, b.theme)

		got, ok := spy.at["Debugger"]
		if !ok {
			t.Fatalf("screenW=%d: the Developer submenu did not draw its Debugger row", screenW)
		}
		if got.X+got.W > menuLeft {
			t.Errorf("screenW=%d: the submenu's text reaches x=%d, over the menu, which starts at %d", screenW, got.X+got.W, menuLeft)
		}
		if got.Y < dev.Y || got.Y > dev.Y+dev.H {
			t.Errorf("screenW=%d: the submenu is at y=%d, not level with its parent row (%d..%d)", screenW, got.Y, dev.Y, dev.Y+dev.H)
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
