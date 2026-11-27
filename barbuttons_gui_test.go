//go:build !headless

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// testBar is a menu bar on a machine with a floppy controller, shown, drawn once at screenW so
// that its labels and logo have places, with the toolbar's buttons as the machine has them.
func testBar(t *testing.T, screenW int) (*appMenuBar, *ZenZX) {
	t.Helper()
	zx := testZXWithDrive()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	t.Cleanup(b.text.Unload)
	b.state = barShown
	left, right := barInsetsFor(b.stripH())
	b.widget.SetInsets(left, right)
	b.widget.Draw(testNoopRenderer{}, screenW, 600, 0, b.stripH(), b.theme)
	b.toolbarCur = b.toolbarItems(zx)
	return b, zx
}

func clickAt(x, y int) zenui.Input { return zenui.Input{MouseX: x, MouseY: y, MousePressed: true} }

func centre(r zenui.Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

// The room kept at the ends is the buttons and their pads, less what zenui keeps of its own.
func TestBarInsetsFor(t *testing.T) {
	for _, h := range []int{24, 32, 36, 48, 72} {
		left, right := barInsetsFor(h)
		pad := logoButtonPadFor(h)
		if want := pad + toggleWidthFor(h) + pad - 12; left != want {
			t.Errorf("h=%d: left %d, want %d", h, left, want)
		}
		if left <= 0 || right <= 0 {
			t.Errorf("h=%d: insets %d, %d: both ends need room for a button", h, left, right)
		}
	}
}

// The two buttons sit at the two ends, inside the strip, and never overlap each other or the
// logo, which moved left for the Help button and is no longer a button.
func TestBarButtonsAreAtTheEnds(t *testing.T) {
	const screenW = 900
	b, _ := testBar(t, screenW)
	h := b.stripH()
	tg, hp := b.toggleButton(), b.helpButton(screenW)
	if tg.X <= 0 || tg.Y != 0 || tg.H != h-2 || tg.W != toggleWidthFor(h) {
		t.Errorf("toggle button %+v in a strip %d high", tg, h)
	}
	if hp.X+hp.W >= screenW || hp.Y != 1 || hp.H != h-2 || hp.W != helpWidthFor(h) {
		t.Errorf("help button %+v in a strip %d high, %d wide", hp, h, screenW)
	}
	if tg.X+tg.W > b.widget.LabelsEndX() && b.widget.LabelsEndX() > 0 && tg.X+tg.W > 12+mustLeft(h) {
		t.Errorf("the toggle button %+v runs into the labels", tg)
	}
	if d := b.widget.DecorationX(); d > 0 && d < screenW && d >= hp.X {
		t.Errorf("the logo starts at %d, under or right of the Help button at %d", d, hp.X)
	}
	// The labels start after the toggle button.
	if got := b.widget.LabelsEndX(); got <= tg.X+tg.W {
		t.Errorf("the labels end at %d, before the toggle button's end %d", got, tg.X+tg.W)
	}
	// The hot zones reach the window's edge and the strip's full height.
	if z := b.toggleHotZone(); z.X != 0 || z.H != h || !z.Contains(tg.X, tg.Y) || !z.Contains(tg.X+tg.W-1, tg.Y+tg.H-1) {
		t.Errorf("toggle hot zone %+v does not cover the button %+v", z, tg)
	}
	if z := b.helpHotZone(screenW); z.X+z.W != screenW || z.H != h || !z.Contains(hp.X, hp.Y) || !z.Contains(hp.X+hp.W-1, hp.Y+hp.H-1) {
		t.Errorf("help hot zone %+v does not cover the button %+v", z, hp)
	}
	if b.toggleHotZone().Contains(screenW-5, 5) || b.helpHotZone(screenW).Contains(5, 5) {
		t.Error("a hot zone reaches the other end of the bar")
	}
}

func mustLeft(h int) int { l, _ := barInsetsFor(h); return l }

// The buttons grow with the strip, as the logo and the rest of the bar do.
func TestBarButtonsGrowWithTheStrip(t *testing.T) {
	const screenW = 1200
	b, _ := testBar(t, screenW)
	usual := b.stripH()
	tgU, hpU := b.toggleButton(), b.helpButton(screenW)
	b.barK = 3
	tall := b.stripH()
	if tall <= usual {
		t.Fatalf("the strip did not grow: %d at X3, %d at X1", tall, usual)
	}
	tgT, hpT := b.toggleButton(), b.helpButton(screenW)
	if tgT.H != tall-2 || tgT.W <= tgU.W || hpT.H != tall-2 || hpT.W <= hpU.W {
		t.Errorf("buttons %+v %+v in the tall strip, %+v %+v in the usual one", tgT, hpT, tgU, hpU)
	}
}

// What the toolbar offers, and what it offers disabled, follows the machine.
func TestToolbarItemsFollowTheMachine(t *testing.T) {
	b, zx := testBar(t, 900)
	byLabel := func(items []toolbarItem, name string) toolbarItem {
		for _, it := range items {
			if it.tip == name || it.icon == name {
				return it
			}
		}
		t.Fatalf("no %q in the toolbar", name)
		return toolbarItem{}
	}
	items := b.toolbarItems(zx)
	for _, name := range []string{"recycle", "play", "pause", "stop", "Tapes", "Disks", "Snapshots", "Screenshots"} {
		it := byLabel(items, name)
		if it.run == nil {
			t.Errorf("%q does nothing", name)
		}
	}
	for _, it := range items {
		if it.tip == "" || (it.icon == "" && it.label == "") {
			t.Errorf("the %q button has no hover hint or no picture", it.icon)
		}
		if it.tip == "SD Cards" {
			t.Error("SD Cards on a machine with no SD card")
		}
	}
	if !byLabel(items, "play").disabled {
		t.Error("Play enabled while the machine runs")
	}
	if byLabel(items, "pause").disabled || byLabel(items, "stop").disabled || byLabel(items, "recycle").disabled {
		t.Error("Pause, Stop and Reset disabled while the machine runs")
	}
	if !byLabel(items, "Tapes").gapBefore {
		t.Error("no gap between the transport buttons and the windows")
	}

	zx.paused = true
	items = b.toolbarItems(zx)
	if byLabel(items, "play").disabled || !byLabel(items, "pause").disabled {
		t.Error("paused: Play should be enabled and Pause disabled")
	}
	zx.paused = false
	zx.stopped = true
	items = b.toolbarItems(zx)
	if byLabel(items, "play").disabled || !byLabel(items, "stop").disabled || !byLabel(items, "recycle").disabled || !byLabel(items, "pause").disabled {
		t.Error("stopped: only Play should be enabled")
	}

	// No drive, no Disks button: it is left out and not greyed.
	for _, it := range b.toolbarItems(testZX()) {
		if it.tip == "Disks" || it.tip == "SD Cards" {
			t.Errorf("%q on a machine that has none", it.tip)
		}
	}
}

// The buttons that fit, side by side, in order, between the toggle button and the logo or the
// Help button. When they do not all fit, a "more" button follows those that do, and stands for
// the rest.
func TestToolbarLayoutFitsOrOverflows(t *testing.T) {
	b, _ := testBar(t, 1200)
	wide := b.toolbarLayout(1200)
	if len(wide) != len(b.toolbarCur) {
		t.Fatalf("1200 wide: %d of %d buttons fit", len(wide), len(b.toolbarCur))
	}
	tg := b.toggleButton()
	end := b.helpButton(1200).X
	prev := tg.X + tg.W
	for i, s := range wide {
		if s.more != nil {
			t.Errorf("button %d is the overflow button although everything fits", i)
		}
		if s.rect.X < prev || s.rect.H != b.stripH() || s.rect.W <= 0 {
			t.Errorf("button %d at %+v after %d", i, s.rect, prev)
		}
		if s.rect.X+s.rect.W > end {
			t.Errorf("button %d at %+v reaches the Help button at %d", i, s.rect, end)
		}
		prev = s.rect.X + s.rect.W
		if s.label == "" && s.rect.W != b.stripH()-2 {
			t.Errorf("icon button %d is %d wide, want square %d", i, s.rect.W, b.stripH()-2)
		}
	}
	overflowed := false
	for n := 1100; n >= 150; n -= 50 {
		b.widget.Draw(testNoopRenderer{}, n, 600, 0, b.stripH(), b.theme)
		got := b.toolbarLayout(n)
		if len(got) == 0 {
			continue
		}
		kept := got
		var rest []toolbarItem
		if last := got[len(got)-1]; last.more != nil {
			overflowed = true
			kept, rest = got[:len(got)-1], last.more
			if last.tip == "" || last.rect.W != b.stripH()-2 {
				t.Errorf("%d wide: the overflow button %+v needs a hint and a square cell", n, last)
			}
		}
		// every button is either shown or in the overflow menu, once, in order
		if len(kept)+len(rest) != len(b.toolbarCur) {
			t.Fatalf("%d wide: %d shown and %d in the overflow menu, of %d", n, len(kept), len(rest), len(b.toolbarCur))
		}
		for i, s := range kept {
			if s.toolbarItem.label != b.toolbarCur[i].label || s.icon != b.toolbarCur[i].icon {
				t.Errorf("%d wide: button %d is %q%q, want %q%q", n, i, s.icon, s.label, b.toolbarCur[i].icon, b.toolbarCur[i].label)
			}
		}
		for i, it := range rest {
			if want := b.toolbarCur[len(kept)+i]; it.label != want.label || it.icon != want.icon {
				t.Errorf("%d wide: overflow item %d is %q%q, want %q%q", n, i, it.icon, it.label, want.icon, want.label)
			}
		}
		if hp := b.helpButton(n); got[len(got)-1].rect.X+got[len(got)-1].rect.W > hp.X {
			t.Errorf("%d wide: the last button %+v reaches the Help button at %d", n, got[len(got)-1].rect, hp.X)
		}
	}
	if !overflowed {
		t.Error("no width between 150 and 1100 needed an overflow button")
	}
	b.widget.Draw(testNoopRenderer{}, 60, 600, 0, b.stripH(), b.theme)
	if got := b.toolbarLayout(60); len(got) != 0 {
		t.Errorf("60 wide: %d buttons, want none", len(got))
	}
}

// The overflow button opens the buttons that did not fit as a menu, which does what they do.
func TestOverflowMenuHoldsTheRest(t *testing.T) {
	const screenW = 160
	b, zx := testBar(t, screenW)
	ran := 0
	b.toolbarCur = []toolbarItem{
		{icon: "play", tip: "Play", run: func(*ZenZX) {}},
		{label: "Tapes", run: func(*ZenZX) {}},
		{label: "Snapshots", run: func(*ZenZX) { ran++ }},
		{label: "Screenshots", disabled: true, run: func(*ZenZX) { ran += 10 }},
	}
	b.toolbarOn, b.toolbarT = true, 1
	b.widget.Draw(testNoopRenderer{}, screenW, 600, 0, b.stripH(), b.theme)
	slots := b.toolbarLayout(screenW)
	last := slots[len(slots)-1]
	if last.more == nil || len(last.more) < 2 {
		t.Fatalf("at %d wide the last slot is %+v: want an overflow button with the dropped buttons", screenW, last)
	}
	x, y := centre(last.rect)
	if !b.barButtonsInput(zx, clickAt(x, y), screenW) || b.logoMenu == nil || !b.overflowOpen {
		t.Fatalf("a press on the overflow button: menu %v, overflowOpen %v", b.logoMenu != nil, b.overflowOpen)
	}
	if len(b.logoSpec) != len(last.more) {
		t.Fatalf("the menu has %d rows for %d buttons", len(b.logoSpec), len(last.more))
	}
	for i, e := range b.logoSpec {
		it := last.more[i]
		name := it.label
		if name == "" {
			name = it.tip
		}
		if e.label != name || e.disabled != it.disabled {
			t.Errorf("row %d is %q (disabled %v), want %q (%v)", i, e.label, e.disabled, name, it.disabled)
		}
	}
	if st := b.helpButtonState(screenW, 0, 0); st != logoIdle {
		t.Errorf("the Help button is %v while the overflow menu is open", st)
	}
	for i, e := range b.logoSpec {
		if e.label == "Snapshots" {
			b.logoSpec.pick(zx, i, -1)
		}
	}
	if ran != 1 {
		t.Errorf("choosing Snapshots ran %d", ran)
	}
	// The Help button's own menu is the Help button's again.
	b.closeMenus()
	b.openLogoMenu(screenW)
	if b.overflowOpen {
		t.Error("overflowOpen stays set for the Help menu")
	}
	hx, hy := centre(b.helpButton(screenW))
	if st := b.helpButtonState(screenW, hx, hy); st != logoPressed {
		t.Errorf("the Help button is %v while its menu is open", st)
	}
}

// The toolbar mode is kept in the settings file, and a session starts the way it was left.
func TestToolbarModeIsSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.text.Unload()
	if b.toolbarOn || b.toolbarT != 0 {
		t.Fatal("a session with no settings starts with the labels")
	}
	b.toggleToolbar(zx)
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(strings.ReplaceAll(string(data), " ", ""), `"toolbar":true`) {
		t.Fatalf("the toolbar mode was not saved: %v\n%s", err, data)
	}
	res, err := settingsconfig.Load(path, data, validThemeNames, validFontNames)
	if err != nil || res.Settings == nil || !res.Settings.Toolbar {
		t.Fatalf("the saved file does not load back with the toolbar on: %v %+v", err, res)
	}
	again, err := newAppMenuBar(testZX(), "./custom-roms", zenui.ThemeDark, "48k", res.Settings, path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.text.Unload()
	if !again.toolbarOn || again.toolbarT != 1 || !again.toolbarSettled() {
		t.Errorf("the next session: on=%v t=%v, want the toolbar in already", again.toolbarOn, again.toolbarT)
	}
	again.toggleToolbar(zx)
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), `"toolbar"`) {
		t.Errorf("with the labels the file still says toolbar:\n%s", data)
	}
}

// What the pointer rests on has a hover hint, after a short wait and not before, for what is only
// a picture.
func TestHoverHints(t *testing.T) {
	const screenW = 900
	b, _ := testBar(t, screenW)
	tx, ty := centre(b.toggleButton())
	if _, text, _ := b.tipTarget(screenW, tx, ty); text != "Toolbar" {
		t.Errorf("over the toggle button with the menus: %q", text)
	}
	hx, hy := centre(b.helpButton(screenW))
	if _, text, _ := b.tipTarget(screenW, hx, hy); text != "Help" {
		t.Errorf("over the Help button: %q", text)
	}
	if key, _, _ := b.tipTarget(screenW, screenW/2, ty); key != "" {
		t.Errorf("over the middle of the bar: %q", key)
	}
	b.toolbarOn, b.toolbarT = true, 1
	if _, text, _ := b.tipTarget(screenW, tx, ty); text != "Menus" {
		t.Errorf("over the toggle button with the toolbar: %q", text)
	}
	seen := map[string]string{}
	for _, s := range b.toolbarLayout(screenW) {
		x, y := centre(s.rect)
		key, text, rc := b.tipTarget(screenW, x, y)
		if s.tip == "" {
			if key != "" {
				t.Errorf("%q has a hint %q although its name is on it", s.label, text)
			}
			continue
		}
		if text != s.tip || rc != s.rect || key == "" {
			t.Errorf("over %q: key %q, text %q, rect %+v", s.tip, key, text, rc)
		}
		seen[key] = text
	}
	if want := len(b.toolbarLayout(screenW)); len(seen) != want || want < 5 {
		t.Errorf("%d buttons with hints, want all %d (a picture each, none a word): %v", len(seen), want, seen)
	}
	b.openLogoMenu(screenW)
	if key, _, _ := b.tipTarget(screenW, tx, ty); key != "" {
		t.Error("a hint under an open menu")
	}
}

// The slide runs towards the toolbar while it is on and back while it is off, between 0 and 1,
// and the offset it gives is the strip's height at the end and nothing at the start.
func TestToolbarSlide(t *testing.T) {
	b, zx := testBar(t, 900)
	h := b.stripH()
	if b.toolbarMoving() || b.toolbarSettled() || b.slideOffset() != 0 {
		t.Fatal("the bar starts with the labels at rest")
	}
	b.toggleToolbar(zx)
	if !b.toolbarOn || !b.toolbarMoving() || b.toolbarSettled() {
		t.Fatalf("after toggling: on=%v moving=%v settled=%v", b.toolbarOn, b.toolbarMoving(), b.toolbarSettled())
	}
	last := -1
	for i := 0; i < 100 && !b.toolbarSettled(); i++ {
		b.stepToolbar(0.016)
		if off := b.slideOffset(); off < last || off > h {
			t.Fatalf("offset %d after %d (strip %d): not rising to the strip's height", off, last, h)
		} else {
			last = off
		}
	}
	if !b.toolbarSettled() || b.toolbarT != 1 || b.slideOffset() != h {
		t.Fatalf("settled=%v t=%v offset=%d, want in at 1 and %d", b.toolbarSettled(), b.toolbarT, b.slideOffset(), h)
	}
	b.stepToolbar(1) // no further
	if b.toolbarT != 1 {
		t.Errorf("t = %v after a step beyond the end", b.toolbarT)
	}
	b.toggleToolbar(zx)
	if b.toolbarOn || !b.toolbarMoving() {
		t.Fatalf("toggled back: on=%v moving=%v", b.toolbarOn, b.toolbarMoving())
	}
	for i := 0; i < 100 && b.toolbarMoving(); i++ {
		b.stepToolbar(0.016)
	}
	if b.toolbarT != 0 || b.slideOffset() != 0 || b.toolbarMoving() {
		t.Errorf("back at rest: t=%v offset=%d moving=%v", b.toolbarT, b.slideOffset(), b.toolbarMoving())
	}
	// It takes about toolbarSlideSeconds, not an instant and not for ever.
	b.toggleToolbar(zx)
	b.stepToolbar(toolbarSlideSeconds / 2)
	if b.toolbarT <= 0 || b.toolbarT >= 1 {
		t.Errorf("half the time in: t = %v", b.toolbarT)
	}
}

// Toggling closes a dropdown that is open, the Help menu included.
func TestToggleToolbarClosesMenus(t *testing.T) {
	b, zx := testBar(t, 900)
	b.openLogoMenu(900)
	if b.logoMenu == nil {
		t.Fatal("the Help menu did not open")
	}
	b.toggleToolbar(zx)
	if b.logoMenu != nil {
		t.Error("the Help menu stayed open under the toolbar")
	}
}

func TestPressOnTheToggleButtonSwapsTheBar(t *testing.T) {
	const screenW = 900
	b, zx := testBar(t, screenW)
	x, y := centre(b.toggleButton())
	if !b.barButtonsInput(zx, clickAt(x, y), screenW) || !b.toolbarOn {
		t.Fatalf("a press on the toggle button: toolbarOn=%v", b.toolbarOn)
	}
	// Moving: nothing is live until it has settled, not even the button that started it.
	if b.barButtonsInput(zx, clickAt(x, y), screenW) || !b.toolbarOn {
		t.Error("a press in the middle of the slide was acted on")
	}
	b.toolbarT = 1
	if !b.barButtonsInput(zx, clickAt(x, y), screenW) || b.toolbarOn {
		t.Errorf("a press on the toggle button with the toolbar in: toolbarOn=%v", b.toolbarOn)
	}
	// No press, no action; and a press elsewhere on the labels is for the labels.
	if b.barButtonsInput(zx, zenui.Input{MouseX: x, MouseY: y}, screenW) {
		t.Error("the pointer resting on the button was acted on")
	}
	if b.barButtonsInput(zx, clickAt(b.widget.LabelsEndX()-5, 5), screenW) {
		t.Error("a press on a label was taken by the buttons")
	}
	// Hidden bar, or a window over it: nothing.
	b.state = barHidden
	if b.barButtonsInput(zx, clickAt(x, y), screenW) {
		t.Error("a hidden bar took a press")
	}
	b.state = barShown
	b.prefs = &prefsWindow{}
	if b.barButtonsInput(zx, clickAt(x, y), screenW) {
		t.Error("a window over the bar did not stop the press")
	}
}

func TestPressOnTheHelpButtonOpensTheHelpMenu(t *testing.T) {
	const screenW = 900
	b, zx := testBar(t, screenW)
	x, y := centre(b.helpButton(screenW))
	if !b.barButtonsInput(zx, clickAt(x, y), screenW) || b.logoMenu == nil {
		t.Fatalf("a press on the Help button: menu open = %v", b.logoMenu != nil)
	}
	// The menu hangs from the button's right edge, as the logo's used to hang from the window's.
	if hb := b.helpButton(screenW); b.logoMenu == nil || hb.X+hb.W > screenW {
		t.Errorf("Help button %+v is not inside the window", hb)
	}
	// The logo is only a picture.
	if d := b.widget.DecorationX(); d > 0 && d < screenW {
		b.closeMenus()
		if b.barButtonsInput(zx, clickAt(d+4, 5), screenW) || b.logoMenu != nil {
			t.Error("a press on the logo did something")
		}
	}
}

func TestPressOnAToolbarButtonRunsIt(t *testing.T) {
	const screenW = 1200
	b, zx := testBar(t, screenW)
	ran := map[string]int{}
	b.toolbarCur = nil
	for _, name := range []string{"one", "two", "off"} {
		name := name
		b.toolbarCur = append(b.toolbarCur, toolbarItem{label: name, disabled: name == "off", run: func(*ZenZX) { ran[name]++ }})
	}
	b.toolbarOn, b.toolbarT = true, 1
	slots := b.toolbarLayout(screenW)
	if len(slots) != 3 {
		t.Fatalf("%d slots", len(slots))
	}
	for i, name := range []string{"one", "two"} {
		x, y := centre(slots[i].rect)
		if !b.barButtonsInput(zx, clickAt(x, y), screenW) || ran[name] != 1 {
			t.Errorf("%s: ran %d times", name, ran[name])
		}
	}
	// A disabled one takes the press and does nothing.
	x, y := centre(slots[2].rect)
	if !b.barButtonsInput(zx, clickAt(x, y), screenW) || ran["off"] != 0 {
		t.Errorf("the disabled button ran %d times", ran["off"])
	}
	// The labels' places are empty of buttons when the toolbar is not in.
	b.toolbarOn, b.toolbarT = false, 0
	x, y = centre(slots[0].rect)
	if b.barButtonsInput(zx, clickAt(x, y), screenW) || ran["one"] != 1 {
		t.Error("a toolbar button ran with the toolbar out")
	}
}

// The transport buttons do what the Machine menu rows do.
func TestToolbarTransportCallsTheMachineMenu(t *testing.T) {
	b, zx := testBar(t, 900)
	called := map[int]int{}
	for row := 1; row <= 4; row++ {
		row := row
		b.actions[barMachine][row] = func(*ZenZX) { called[row]++ }
	}
	for _, it := range b.toolbarItems(zx)[:4] {
		it.run(zx)
	}
	for row := 1; row <= 4; row++ {
		if called[row] != 1 {
			t.Errorf("Machine row %d called %d times", row, called[row])
		}
	}
}

// The pointer is a hand over the buttons, and over the toolbar's enabled buttons once it is in,
// and nowhere else.
func TestBarButtonCursor(t *testing.T) {
	const screenW = 1200
	b, _ := testBar(t, screenW)
	tx, ty := centre(b.toggleButton())
	hx, hy := centre(b.helpButton(screenW))
	if !b.barButtonCursorWanted(screenW, tx, ty) || !b.barButtonCursorWanted(screenW, hx, hy) {
		t.Error("the buttons want a hand")
	}
	if b.barButtonCursorWanted(screenW, screenW/2, ty) {
		t.Error("the middle of the bar wants a hand")
	}
	if b.barButtonCursorWanted(screenW, tx, menuStripHeight+20) {
		t.Error("below the bar wants a hand")
	}
	b.toolbarCur = []toolbarItem{{label: "On", run: func(*ZenZX) {}}, {label: "Off", disabled: true}}
	b.toolbarOn, b.toolbarT = true, 1
	slots := b.toolbarLayout(screenW)
	if x, y := centre(slots[0].rect); !b.barButtonCursorWanted(screenW, x, y) {
		t.Error("an enabled toolbar button wants a hand")
	}
	if x, y := centre(slots[1].rect); b.barButtonCursorWanted(screenW, x, y) {
		t.Error("a disabled toolbar button wants a hand")
	}
	b.toolbarT = 0.5 // sliding
	if b.barButtonCursorWanted(screenW, tx, ty) {
		t.Error("a hand while the bar is sliding")
	}
	b.toolbarT = 1
	b.state = barHidden
	if b.barButtonCursorWanted(screenW, tx, ty) {
		t.Error("a hand over a hidden bar")
	}
	b.state = barShown
	b.prefs = &prefsWindow{}
	if b.barButtonCursorWanted(screenW, tx, ty) {
		t.Error("a hand under a window")
	}
}

// The toolbar button is pressed in while the toolbar is the bar; the Help button while its menu is
// open; both are raised under the pointer and plain otherwise.
func TestBarButtonStates(t *testing.T) {
	const screenW = 900
	b, _ := testBar(t, screenW)
	tx, ty := centre(b.toggleButton())
	hx, hy := centre(b.helpButton(screenW))
	if got := b.toggleButtonState(tx, ty); got != logoHover {
		t.Errorf("toggle under the pointer: %v", got)
	}
	if got := b.toggleButtonState(screenW/2, ty); got != logoIdle {
		t.Errorf("toggle at rest: %v", got)
	}
	b.toolbarOn = true
	if got := b.toggleButtonState(screenW/2, ty); got != logoPressed {
		t.Errorf("toggle with the toolbar on: %v", got)
	}
	b.toolbarOn = false
	if got := b.helpButtonState(screenW, hx, hy); got != logoHover {
		t.Errorf("help under the pointer: %v", got)
	}
	if got := b.helpButtonState(screenW, 5, hy); got != logoIdle {
		t.Errorf("help at rest: %v", got)
	}
	b.openLogoMenu(screenW)
	if got := b.helpButtonState(screenW, 5, hy); got != logoPressed {
		t.Errorf("help with its menu open: %v", got)
	}
}

func TestRoundedShapes(t *testing.T) {
	// A rounded rectangle is symmetric, and the corners cut more at the top than inside.
	for _, c := range []struct{ rad, h int }{{2, 22}, {4, 22}, {5, 30}, {7, 46}} {
		prev := 1 << 30
		for i := 0; i < c.h; i++ {
			in := roundedInset(c.rad, i, c.h)
			if in != roundedInset(c.rad, c.h-1-i, c.h) {
				t.Errorf("rad %d h %d: row %d inset %d, its mirror %d", c.rad, c.h, i, in, roundedInset(c.rad, c.h-1-i, c.h))
			}
			if i <= c.h/2 && in > prev {
				t.Errorf("rad %d h %d: inset grows from %d to %d going down", c.rad, c.h, prev, in)
			}
			if in < 0 || in > c.rad {
				t.Errorf("rad %d h %d: row %d inset %d out of range", c.rad, c.h, i, in)
			}
			if i <= c.h/2 {
				prev = in
			}
		}
		if roundedInset(c.rad, c.h/2, c.h) != 0 || roundedInset(c.rad, 0, c.h) == 0 {
			t.Errorf("rad %d h %d: the middle should be square and the top cut", c.rad, c.h)
		}
	}
	// Painted once: no pixel of a fill is covered twice, so a translucent colour is even.
	rc := zenui.Rect{X: 3, Y: 2, W: 40, H: 22}
	seen := map[[2]int]int{}
	rec := pixelRecorder{seen: seen}
	fillRounded(rec, rc, 4, zenui.Colour{A: 60})
	area := 0
	for p, n := range seen {
		if n != 1 {
			t.Errorf("pixel %v painted %d times", p, n)
		}
		if p[0] < rc.X || p[0] >= rc.X+rc.W || p[1] < rc.Y || p[1] >= rc.Y+rc.H {
			t.Errorf("pixel %v outside %+v", p, rc)
		}
		area++
	}
	if area >= rc.W*rc.H || area < rc.W*rc.H-4*4*4 {
		t.Errorf("filled %d of %d pixels: the corners are not rounded off", area, rc.W*rc.H)
	}
	// The stroke is inside the fill's shape, symmetric left to right and top to bottom.
	stroke := map[[2]int]int{}
	strokeRounded(pixelRecorder{seen: stroke}, rc, 4, 1, zenui.Colour{A: 100})
	for p, n := range stroke {
		if n != 1 {
			t.Errorf("stroke pixel %v painted %d times", p, n)
		}
		if seen[p] == 0 {
			t.Errorf("stroke pixel %v outside the fill", p)
		}
		mx := [2]int{2*rc.X + rc.W - 1 - p[0], p[1]}
		my := [2]int{p[0], 2*rc.Y + rc.H - 1 - p[1]}
		if stroke[mx] != 1 || stroke[my] != 1 {
			t.Errorf("stroke pixel %v has no mirror image", p)
		}
	}
	if len(stroke) == 0 || len(stroke) >= area {
		t.Errorf("stroke has %d pixels of %d", len(stroke), area)
	}
}

// pixelRecorder counts how often each pixel is filled.
type pixelRecorder struct {
	testNoopRenderer
	seen map[[2]int]int
}

func (r pixelRecorder) FillRect(rc zenui.Rect, c zenui.Colour) {
	for y := rc.Y; y < rc.Y+rc.H; y++ {
		for x := rc.X; x < rc.X+rc.W; x++ {
			r.seen[[2]int{x, y}]++
		}
	}
}

// pixels is the set of pixels a drawing fills.
func pixels(draw func(r zenui.Renderer)) map[[2]int]int {
	seen := map[[2]int]int{}
	draw(pixelRecorder{seen: seen})
	return seen
}

func bounds(px map[[2]int]int) (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = 1<<30, 1<<30, -1<<30, -1<<30
	for p := range px {
		x0, y0, x1, y1 = min(x0, p[0]), min(y0, p[1]), max(x1, p[0]), max(y1, p[1])
	}
	return
}

// The upright picture is four thin slanted bars in one colour, leaning like "////", in a square
// centred in the box.
func TestSlashBarsUpright(t *testing.T) {
	box := zenui.Rect{X: 10, Y: 20, W: 24, H: 16}
	g, ok := slashGlyphIn(box)
	if !ok || g.s != 1 || g.height() != 6 || g.width() != 15 {
		t.Fatalf("glyph %+v %v for a box %+v", g, ok, box)
	}
	red := zenui.Colour{R: 200, A: 255}
	cols := map[zenui.Colour]bool{}
	px := map[[2]int]int{}
	drawSlashBars(colourSpy{pixelRecorder{seen: px}, cols}, box, 0, red)
	if len(px) == 0 {
		t.Fatal("nothing drawn")
	}
	if len(cols) != 1 || !cols[red] {
		t.Errorf("colours used %v, want only %v", cols, red)
	}
	x0, y0, x1, y1 := bounds(px)
	if x0 < box.X || x1 >= box.X+box.W || y0 != box.Y+(box.H-g.height())/2 || y1 != y0+g.height()-1 {
		t.Errorf("drawn in x %d..%d y %d..%d, outside or short of the box %+v", x0, x1, y0, y1, box)
	}
	// Every row has four bars, and each row is a pixel or so to the left of the one above.
	left := func(y int) (n int, first int) {
		first = -1
		prev := false
		for x := box.X; x < box.X+box.W; x++ {
			on := px[[2]int{x, y}] > 0
			if on && !prev {
				n++
			}
			if on && first < 0 {
				first = x
			}
			prev = on
		}
		return
	}
	for y := y0; y <= y1; y++ {
		if n, _ := left(y); n != 4 {
			t.Errorf("row %d has %d bars, want 4", y, n)
		}
	}
	if _, top := left(y0); top <= func() int { _, b := left(y1); return b }() {
		t.Error("the bars do not lean like ////")
	}
	// A whole number of full turns is the same picture; too small a box draws nothing.
	again := pixels(func(r zenui.Renderer) { drawSlashBars(r, box, 2, red) })
	if len(again) != len(px) {
		t.Errorf("a full turn draws %d pixels, upright %d", len(again), len(px))
	}
	if got := pixels(func(r zenui.Renderer) { drawSlashBars(r, zenui.Rect{W: 6, H: 10}, 0, red) }); len(got) != 0 {
		t.Errorf("a box too small drew %d pixels", len(got))
	}
}

// colourSpy remembers every colour it is asked to fill with.
type colourSpy struct {
	pixelRecorder
	cols map[zenui.Colour]bool
}

func (s colourSpy) FillRect(rc zenui.Rect, c zenui.Colour) {
	s.cols[c] = true
	s.pixelRecorder.FillRect(rc, c)
}

// Turned, the picture stays inside its box at every angle and is never empty; a half-turn
// keeps its pixels, and a quarter-turn lays the bars on their side.
func TestSlashBarsTurnInsideTheirBox(t *testing.T) {
	box := zenui.Rect{X: 10, Y: 20, W: 24, H: 16}
	c := zenui.Colour{A: 255}
	upright := pixels(func(r zenui.Renderer) { drawSlashBars(r, box, 0, c) })
	for i := 1; i < 40; i++ {
		turn := float64(i) / 20
		px := pixels(func(r zenui.Renderer) { drawSlashBars(r, box, turn, c) })
		if len(px) == 0 {
			t.Errorf("turn %.2f: nothing drawn", turn)
		}
		x0, y0, x1, y1 := bounds(px)
		if x0 < box.X || x1 >= box.X+box.W || y0 < box.Y || y1 >= box.Y+box.H {
			t.Fatalf("turn %.2f: drawn in x %d..%d y %d..%d, outside %+v", turn, x0, x1, y0, y1, box)
		}
	}
	// The glyph is a parallelogram, so turned half-way round it is the picture again.
	half := pixels(func(r zenui.Renderer) { drawSlashBars(r, box, 1, c) })
	diff := 0
	for p := range upright {
		if half[p] == 0 && half[[2]int{p[0] - 1, p[1]}] == 0 && half[[2]int{p[0] + 1, p[1]}] == 0 {
			diff++ // (a pixel either side counts: the bars may land a pixel across)
		}
	}
	if diff > len(upright)/4 {
		t.Errorf("the half-turn differs from the upright picture in %d of %d pixels", diff, len(upright))
	}
	// A quarter-turn: the bars lie down, so the picture is wider than the upright's is, row for column.
	q := pixels(func(r zenui.Renderer) { drawSlashBars(r, box, 0.5, c) })
	if len(q) == 0 {
		t.Fatal("quarter-turn: nothing drawn")
	}
}

func TestSlashGlyphHas(t *testing.T) {
	g, ok := slashGlyphIn(zenui.Rect{W: 24, H: 16})
	if !ok {
		t.Fatal("no glyph")
	}
	if g.has(-1, 0) || g.has(0, -1) || g.has(g.width(), 0) || g.has(0, g.height()) {
		t.Error("a pixel outside the glyph")
	}
	if !g.has(0, g.height()-1) || !g.has(g.width()-1, 0) {
		t.Error("the bottom left and the top right are the glyph's corners")
	}
	if g.has(g.width()-1, g.height()-1) || g.has(0, 0) {
		t.Error("the top left and the bottom right are empty: the bars lean")
	}
	// A bigger box gets a bigger glyph, never too wide or high for it.
	big, ok := slashGlyphIn(zenui.Rect{W: 60, H: 56})
	if !ok || big.s <= g.s || big.width() > 60 || big.height() > 56 {
		t.Errorf("the glyph for 60x56 is %+v (%v), the one for 24x16 %+v", big, ok, g)
	}
}

// The glyph is the Spectrum logo's teeth (zenui's drawZSPLogo) pixel by pixel: each bar a staircase of
// three blocks, a block 1 wide and 2 high at the smallest size, the lowest at the left, each next
// one a block higher and a block to the right, the bars 3 apart in a row where the logo's touch.
func TestSlashGlyphIsTheLogosTeeth(t *testing.T) {
	g := slashGlyph{s: 1}
	want := []string{
		"..#...#...#...#",
		"..#...#...#...#",
		".#...#...#...#.",
		".#...#...#...#.",
		"#...#...#...#..",
		"#...#...#...#..",
	}
	for y, row := range want {
		got := make([]byte, g.width())
		for x := range got {
			got[x] = '.'
			if g.has(x, y) {
				got[x] = '#'
			}
		}
		if string(got) != row {
			t.Errorf("row %d is %s, want %s", y, got, row)
		}
	}
	// At twice the size every block is twice as wide and high.
	g2 := slashGlyph{s: 2}
	for y := 0; y < g2.height(); y++ {
		for x := 0; x < g2.width(); x++ {
			if g2.has(x, y) != g.has(x/2, y/2) {
				t.Fatalf("size 2 differs from size 1 at %d,%d", x, y)
			}
		}
	}
}

func TestDrawBarButtonStates(t *testing.T) {
	rc := zenui.Rect{X: 4, Y: 1, W: 36, H: 22}
	ink := zenui.Colour{R: 200, G: 190, B: 180, A: 255}
	count := func(state logoButtonState, barY int) []testDrawCall {
		r := newTestDrawRecorder()
		drawBarButton(r, rc, barY, state, ink)
		return *r.calls
	}
	idle, hover, pressed := count(logoIdle, 0), count(logoHover, 0), count(logoPressed, 0)
	if len(idle) == 0 || len(hover) <= len(idle) || len(pressed) <= len(idle) {
		t.Fatalf("idle draws %d, hover %d, pressed %d: the states add a face to the edge", len(idle), len(hover), len(pressed))
	}
	// At rest only a light edge, in the text's colour: light on a dark theme and dark on a light one.
	for _, c := range idle {
		if c.col.R != ink.R || c.col.G != ink.G || c.col.B != ink.B || c.col.A >= 130 {
			t.Fatalf("the idle edge %v is not a light %v", c.col, ink)
		}
	}
	// The face comes first: faint ink under the pointer, dark while pressed.
	if hover[0].col.R != ink.R || hover[0].col.A >= 100 || pressed[0].col.R != 0 || pressed[0].col.A == 0 {
		t.Errorf("faces: hover %v, pressed %v", hover[0].col, pressed[0].col)
	}
	// The bar slides and the button follows.
	for _, c := range count(logoHover, -24) {
		if c.rect.Y < rc.Y-24 || c.rect.Y > rc.Y-24+rc.H {
			t.Fatalf("a row at y=%d, not following the bar %d up", c.rect.Y, 24)
		}
	}
	// Everything is inside the rectangle.
	for _, c := range idle {
		if c.rect.X < rc.X || c.rect.X+c.rect.W > rc.X+rc.W || c.rect.Y < rc.Y || c.rect.Y+c.rect.H > rc.Y+rc.H {
			t.Errorf("a %s at %+v outside %+v", c.kind, c.rect, rc)
		}
	}
}

func TestDrawBurger(t *testing.T) {
	box := zenui.Rect{X: 5, Y: 6, W: 14, H: 12}
	r := newTestDrawRecorder()
	drawBurger(r, box, zenui.Colour{A: 255})
	if len(*r.calls) != 3 {
		t.Fatalf("%d lines, want three", len(*r.calls))
	}
	for i, c := range *r.calls {
		if c.rect.X != box.X || c.rect.W != box.W || c.rect.H != 1 || c.rect.Y < box.Y || c.rect.Y+c.rect.H > box.Y+box.H {
			t.Errorf("line %d %+v is not a thin line inside %+v", i, c.rect, box)
		}
		if i > 0 && c.rect.Y <= (*r.calls)[i-1].rect.Y+1 {
			t.Errorf("line %d touches the one above", i)
		}
	}
	if (*r.calls)[0].rect.Y != box.Y || (*r.calls)[2].rect.Y+(*r.calls)[2].rect.H != box.Y+box.H {
		t.Errorf("the lines do not span the box: %+v", *r.calls)
	}
	r = newTestDrawRecorder()
	drawBurger(r, zenui.Rect{W: 10, H: 4}, zenui.Colour{})
	if len(*r.calls) != 0 {
		t.Error("drew in a box too short for three lines")
	}
}

// The Help button is a circle: symmetric, inside its rectangle, each pixel painted once, the edge
// inside the disc, and the burger's box inside the circle.
func TestHelpCircle(t *testing.T) {
	for _, rc := range []zenui.Rect{{X: 900, Y: 1, W: 24, H: 22}, {X: 10, Y: 1, W: 36, H: 34}, {X: 0, Y: 0, W: 13, H: 13}, {X: 3, Y: 2, W: 70, H: 70}} {
		orig := rc
		rc = circleOf(rc)
		d := min(rc.W, rc.H)
		x0, y0 := rc.X+(rc.W-d)/2, rc.Y+(rc.H-d)/2
		disc := pixels(func(r zenui.Renderer) { fillCircle(r, rc, zenui.Colour{A: 60}) })
		edge := pixels(func(r zenui.Renderer) { strokeCircle(r, rc, zenui.Colour{A: 60}) })
		for name, set := range map[string]map[[2]int]int{"disc": disc, "edge": edge} {
			for p, n := range set {
				if n != 1 {
					t.Errorf("%+v %s: pixel %v painted %d times", rc, name, p, n)
				}
				if p[0] < x0 || p[0] >= x0+d || p[1] < y0 || p[1] >= y0+d {
					t.Errorf("%+v %s: pixel %v outside the circle's square", rc, name, p)
				}
				if set[[2]int{2*x0 + d - 1 - p[0], p[1]}] == 0 || set[[2]int{p[0], 2*y0 + d - 1 - p[1]}] == 0 {
					t.Errorf("%+v %s: pixel %v has no mirror image", rc, name, p)
				}
			}
		}
		for p := range edge {
			if disc[p] == 0 {
				t.Errorf("%+v: edge pixel %v is outside the disc", rc, p)
			}
		}
		if len(disc) == 0 || len(disc) >= d*d || len(edge) == 0 || len(edge) >= len(disc) {
			t.Errorf("%+v: disc %d, edge %d of %d: not a circle", rc, len(disc), len(edge), d*d)
		}
		// the middle is in the disc and not the edge; the corner of the square is in neither
		mid := [2]int{x0 + d/2, y0 + d/2}
		if d > 8 && (disc[mid] == 0 || edge[mid] != 0) {
			t.Errorf("%+v: the centre should be face and not edge", rc)
		}
		if disc[[2]int{x0, y0}] != 0 {
			t.Errorf("%+v: the corner of the square is in the disc", rc)
		}
		// the burger fits within the circle: its corners are inside the disc
		ib := circleIconBox(orig)
		if d > 8 {
			for _, p := range [][2]int{{ib.X, ib.Y}, {ib.X + ib.W - 1, ib.Y}, {ib.X, ib.Y + ib.H - 1}, {ib.X + ib.W - 1, ib.Y + ib.H - 1}} {
				if disc[p] == 0 || edge[p] != 0 {
					t.Errorf("%+v: the burger's corner %v is not clear inside the circle", rc, p)
				}
			}
		}
	}
}

func TestDrawBarCircleStates(t *testing.T) {
	rc := zenui.Rect{X: 100, Y: 1, W: 24, H: 22}
	count := func(state logoButtonState, barY int) []testDrawCall {
		r := newTestDrawRecorder()
		drawBarCircle(r, rc, barY, state, zenui.Colour{R: 200, G: 190, B: 180, A: 255})
		return *r.calls
	}
	idle, hover, pressed := count(logoIdle, 0), count(logoHover, 0), count(logoPressed, 0)
	if len(idle) == 0 || len(hover) <= len(idle) || len(pressed) <= len(idle) {
		t.Fatalf("idle draws %d, hover %d, pressed %d: the states add a face to the edge", len(idle), len(hover), len(pressed))
	}
	if hover[0].col.R != 200 || pressed[0].col.R != 0 {
		t.Errorf("faces: hover %v, pressed %v: want the text's colour, then dark", hover[0].col, pressed[0].col)
	}
	if idle[0].col.A >= 130 {
		t.Errorf("the idle edge %v is not light", idle[0].col)
	}
	for _, c := range count(logoHover, -24) {
		if c.rect.Y < rc.Y-24 || c.rect.Y > rc.Y-24+rc.H {
			t.Fatalf("a row at y=%d, not following the bar", c.rect.Y)
		}
	}
}

// The toolbar button is a rounded square, and the //// fits in it whole.
func TestToggleButtonIsASquare(t *testing.T) {
	b, _ := testBar(t, 900)
	for _, k := range []int{1, 2, 3} {
		b.barK = k
		tg := b.toggleButton()
		if tg.W != tg.H {
			t.Errorf("X%d: the toggle button is %dx%d, not square", k, tg.W, tg.H)
		}
		if _, ok := slashGlyphIn(iconBox(tg)); !ok {
			t.Errorf("X%d: the //// does not fit in the %dx%d button", k, tg.W, tg.H)
		}
	}
}

// The Help button's circle is two pixels in from the button on every side, and one pixel more
// where that would leave it even across, so that it has a middle row and column of pixels.
func TestHelpCircleIsTwoPixelsSmaller(t *testing.T) {
	for _, rc := range []zenui.Rect{{X: 900, Y: 1, W: 24, H: 22}, {X: 10, Y: 1, W: 36, H: 34}, {X: 10, Y: 1, W: 31, H: 29}} {
		c := circleOf(rc)
		if c.W != c.H || c.W%2 == 0 || c.W > min(rc.W, rc.H)-3 || c.W < min(rc.W, rc.H)-4 {
			t.Errorf("%+v: circle square %+v: want odd, and 3 or 4 less than the button is high", rc, c)
		}
		if c.X < rc.X || c.Y < rc.Y || c.X+c.W > rc.X+rc.W || c.Y+c.H > rc.Y+rc.H {
			t.Errorf("%+v: circle square %+v outside it", rc, c)
		}
		if got := pixels(func(r zenui.Renderer) { drawBarCircle(r, rc, 0, logoIdle, zenui.Colour{A: 255}) }); len(got) == 0 {
			t.Errorf("%+v: nothing drawn", rc)
		} else if x0, y0, x1, y1 := bounds(got); x1-x0+1 != c.W || y1-y0+1 != c.H {
			t.Errorf("%+v: drawn %dx%d, want %d across", rc, x1-x0+1, y1-y0+1, c.W)
		}
	}
}

// The burger is exactly centred in its circle, to the pixel, both ways, at every size of the bar,
// and is the same from the top as from the bottom.
func TestBurgerIsCentredInItsCircle(t *testing.T) {
	for _, rc := range []zenui.Rect{{X: 900, Y: 1, W: 24, H: 22}, {X: 10, Y: 1, W: 32, H: 30}, {X: 10, Y: 1, W: 48, H: 46}, {X: 0, Y: 0, W: 70, H: 70}} {
		c := circleOf(rc)
		got := pixels(func(r zenui.Renderer) { drawBurger(r, circleIconBox(rc), zenui.Colour{A: 255}) })
		x0, y0, x1, y1 := bounds(got)
		if l, r := x0-c.X, c.X+c.W-1-x1; l != r {
			t.Errorf("%+v: %d pixels from the circle's left and %d from its right", rc, l, r)
		}
		if tp, bt := y0-c.Y, c.Y+c.H-1-y1; tp != bt {
			t.Errorf("%+v: %d pixels from the circle's top and %d from its bottom", rc, tp, bt)
		}
		for p := range got {
			if got[[2]int{p[0], y0 + y1 - p[1]}] == 0 {
				t.Errorf("%+v: row %d of the burger has no mirror row", rc, p[1])
				break
			}
		}
	}
}

// Every button the toolbar has is a picture that exists, and each picture of the windows draws
// ink inside its 16x16 box, differently from every other.
func TestToolbarButtonsArePictures(t *testing.T) {
	if len(toolbarIcons) != len(toolbarSprites) {
		t.Fatalf("%d of %d toolbar sprites parse", len(toolbarIcons), len(toolbarSprites))
	}
	b, zx := testBar(t, 900)
	zx.io.hasFDC = true
	items := b.toolbarItems(zx)
	seen := map[[16]iconRow]string{}
	for _, it := range items {
		if it.label != "" || it.icon == "" {
			t.Errorf("the %q button is not a picture", it.tip)
			continue
		}
		icon, ok := transportIcons[it.icon]
		if !ok {
			icon, ok = toolbarIcons[it.icon]
		}
		if !ok {
			t.Errorf("the %q button's picture %q does not exist", it.tip, it.icon)
			continue
		}
		x0, y0, x1, y1 := iconBounds(icon)
		if x1 <= x0 || y1 <= y0 {
			t.Errorf("the picture %q is empty", it.icon)
		}
		if other, dup := seen[icon]; dup {
			t.Errorf("the pictures %q and %q are the same", other, it.icon)
		}
		seen[icon] = it.icon
	}
}
