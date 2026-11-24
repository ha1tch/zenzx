//go:build !headless

package zenzx

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// The Preferences window shows the pointer as a hand over its tabs, and keeps its help text
// hidden until the pointer rests on a row, a caption or a group's title.

// tipLog is a renderer that remembers the text drawn, in order.
type tipLog struct {
	testNoopRenderer
	texts *[]string
}

func (l tipLog) DrawText(s string, _, _, _ int, _ zenui.Colour) { *l.texts = append(*l.texts, s) }

// tipWindow is a window on the tab of the row labelled label, laid out, and where that row is.
func tipWindow(t *testing.T, label string) (w *prefsWindow, si, ri int) {
	t.Helper()
	b, zx := newTestBar(t, nil, "")
	w = newPrefsWindow(b.prefSections(zx), "", func() {})
	for i, s := range w.sections {
		for j, row := range s.rows {
			if row.label == label {
				si, ri = i, j
			}
		}
	}
	if w.sections[si].rows[ri].label != label {
		t.Fatalf("no row %q", label)
	}
	w.tabs.Selected = w.tabIndex(w.sections[si].tab)
	w.layout(testNoopRenderer{}, 1000, 800)
	return w, si, ri
}

// frame runs one frame of the window with the pointer at (x, y), dt seconds on.
func frame(w *prefsWindow, x, y int, dt float32, pressed bool) {
	w.Update(zenui.Input{MouseX: x, MouseY: y, DeltaTime: dt, MousePressed: pressed}, testNoopRenderer{}, 1000, 800)
}

func mid(r zenui.Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

// onRow is a point on row (si, ri) that answers: its caption, or a switch's box and words.
func onRow(w *prefsWindow, si, ri int) (int, int) {
	if l := w.labels[si][ri]; l.W > 0 {
		return mid(l)
	}
	r := testNoopRenderer{}
	return mid(w.checks[si][ri].Bounds(w.areas[si][ri], w.lineH, func(s string) int { return r.TextWidth(s, w.scale) }))
}

func TestPreferencesPointerIsAHandOverTheTabsAndNowhereElse(t *testing.T) {
	w, si, ri := tipWindow(t, "Theme")
	for i, r := range w.tabs.Layout(w.tabArea, w.lineH) {
		x, y := mid(r)
		frame(w, x, y, 0, false)
		if !w.handWanted {
			t.Errorf("the pointer is on tab %d and is not a hand", i)
		}
	}
	for name, p := range map[string][2]int{
		"a row's control": {w.areas[si][ri].X + 2, w.areas[si][ri].Y + 2},
		"a group title":   {w.heads[si].X + 2, w.heads[si].Y + 2},
		"the corner":      {0, 0},
		"the Close bar":   {w.barArea.X + 2, w.barArea.Y + 2},
	} {
		frame(w, p[0], p[1], 0, false)
		if w.handWanted {
			t.Errorf("the pointer is on %s and is a hand", name)
		}
	}
}

func TestPreferencesHintsAreHiddenUntilTheyAreAskedFor(t *testing.T) {
	w, si, ri := tipWindow(t, "Font zoom")
	hint := w.sections[si].rows[ri].hint
	if hint == "" {
		t.Fatal("the row has no hint to hide")
	}
	var texts []string
	w.Draw(tipLog{texts: &texts}, 1000, 800, zenui.DarkTheme())
	if got := strings.Join(texts, " "); strings.Contains(got, hint) {
		t.Errorf("the hint %q is drawn with nothing under the pointer", hint)
	}

	// Rests on the control long enough: the hint is drawn, in a box.
	x, y := onRow(w, si, ri)
	for i := 0; i < 5; i++ {
		frame(w, x, y, 0.1, false)
	}
	texts = nil
	w.Draw(tipLog{texts: &texts}, 1000, 800, zenui.DarkTheme())
	if got := strings.Join(texts, " "); !strings.Contains(got, hint) {
		t.Errorf("the hint is not drawn after the pointer rested on the row; drawn: %q", got)
	}
}

func TestPreferencesHelpComesAfterAPauseAndGoesWithThePointer(t *testing.T) {
	w, si, ri := tipWindow(t, "Font zoom")
	x, y := onRow(w, si, ri)
	frame(w, x, y, 0.1, false)
	if w.tipShowing() {
		t.Error("the box shows at once, with no pause")
	}
	for i := 0; i < 4; i++ {
		frame(w, x, y, 0.1, false)
	}
	if !w.tipShowing() {
		t.Fatal("the box does not show after the pointer rested")
	}
	frame(w, 0, 0, 0.1, false)
	if w.tipShowing() {
		t.Error("the box stays when the pointer is off the row")
	}
	// Back on it, the pause starts again.
	frame(w, x, y, 0.1, false)
	if w.tipShowing() {
		t.Error("the box comes back with no pause")
	}
}

func TestPreferencesAClickHidesTheBoxUntilThePointerMovesOff(t *testing.T) {
	w, si, ri := tipWindow(t, "Font zoom")
	x, y := onRow(w, si, ri)
	for i := 0; i < 5; i++ {
		frame(w, x, y, 0.1, false)
	}
	if !w.tipShowing() {
		t.Fatal("no box to hide")
	}
	frame(w, x, y, 0.1, true)
	if w.tipShowing() {
		t.Error("the box stays after a click")
	}
	for i := 0; i < 5; i++ {
		frame(w, x, y, 0.1, false)
	}
	if w.tipShowing() {
		t.Error("the box comes back while the pointer is still on what it clicked")
	}
}

func TestPreferencesHelpIsOnTheCaptionTheControlAndTheTitle(t *testing.T) {
	r := testNoopRenderer{}
	// A choice with a hint: its caption and each of its options answer.
	w, si, ri := tipWindow(t, "A loaded snapshot")
	tw := func(s string) int { return r.TextWidth(s, w.scale) }
	hit := func(x, y int) string { k, _, _ := w.tipTarget(x, y, tw); return k }
	want := "r" + strconv.Itoa(si) + "." + strconv.Itoa(ri)
	if x, y := mid(w.labels[si][ri]); hit(x, y) != want {
		t.Errorf("the caption does not answer: %q", hit(x, y))
	}
	rects, _ := w.radios[si][ri].Layout(w.areas[si][ri], w.lineH, tw)
	for i, rc := range rects {
		if x, y := mid(rc); hit(x, y) != want {
			t.Errorf("option %d does not answer: %q", i, hit(x, y))
		}
	}
	// The empty space to the right of the options does not.
	area := w.areas[si][ri]
	if k := hit(area.X+area.W-1, area.Y+area.H/2); k != "" {
		t.Errorf("empty space to the right of the options answers: %q", k)
	}

	// A switch: the box and the words beside it answer; the empty rest of the row does not.
	w, si, ri = tipWindow(t, "Machine dock")
	tw = func(s string) int { return r.TextWidth(s, w.scale) }
	hit = func(x, y int) string { k, _, _ := w.tipTarget(x, y, tw); return k }
	b := w.checks[si][ri].Bounds(w.areas[si][ri], w.lineH, tw)
	if x, y := mid(b); hit(x, y) == "" {
		t.Error("a switch does not answer")
	}
	if a := w.areas[si][ri]; hit(a.X+a.W-1, a.Y+a.H/2) != "" {
		t.Error("the empty end of a switch's row answers")
	}

	// A title answers for the whole group, and says every hint in it.
	title := w.heads[si]
	k, text, _ := w.tipTarget(title.X+2, title.Y+title.H/2, tw)
	if k == "" {
		t.Fatal("the group's title does not answer")
	}
	for _, row := range w.sections[si].rows {
		if row.hint != "" && !strings.Contains(text, row.label+": "+row.hint) {
			t.Errorf("the title's help lacks %q: %q", row.label, text)
		}
	}
}

func TestPreferencesARowWithNothingToSayHasNoBox(t *testing.T) {
	w, si, ri := tipWindow(t, "Theme") // no hint
	x, y := onRow(w, si, ri)
	for i := 0; i < 6; i++ {
		frame(w, x, y, 0.1, false)
	}
	if w.tipShowing() {
		t.Error("a row with no hint shows a box")
	}
}

func TestPreferencesWhyARowIsOffReplacesItsHint(t *testing.T) {
	row := prefRow{label: "X", hint: "the hint", off: func() string { return "Not now." }}
	if got := rowNote(row); got != "Not now." {
		t.Errorf("a row that is off says %q", got)
	}
	row.off = func() string { return "" }
	if got := rowNote(row); got != "the hint" {
		t.Errorf("a row that is on says %q", got)
	}
}

func TestPreferencesHintsNoLongerTakeRoomInTheLayout(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	w := newPrefsWindow(b.prefSections(zx), "", func() {})
	w.layout(testNoopRenderer{}, 1000, 800)
	for si, s := range w.sections {
		for ri := 1; ri < len(s.rows); ri++ {
			prev, cur := w.areas[si][ri-1], w.areas[si][ri]
			if gap := cur.Y - (prev.Y + prev.H); gap >= w.lineH {
				t.Errorf("%q: %d pixels between rows %d and %d, a line (%d) or more: a hint's room is still kept",
					s.title, gap, ri-1, ri, w.lineH)
			}
		}
	}
}

func TestPlaceTipStaysOnTheScreen(t *testing.T) {
	const W, H = 800, 600
	for _, a := range []zenui.Rect{
		{X: 0, Y: 0, W: 40, H: 16}, {X: 760, Y: 0, W: 40, H: 16},
		{X: 0, Y: 584, W: 40, H: 16}, {X: 760, Y: 584, W: 40, H: 16}, {X: 400, Y: 300, W: 40, H: 16},
	} {
		for _, x := range []int{0, 400, 799} {
			b := placeTip(a, x, 300, 90, W, H)
			if b.X < 0 || b.Y < 0 || b.X+b.W > W || b.Y+b.H > H {
				t.Errorf("anchor %+v pointer %d: the box %+v is not on the %dx%d screen", a, x, b, W, H)
			}
			if b.Y < a.Y+a.H && b.Y+b.H > a.Y {
				t.Errorf("anchor %+v pointer %d: the box %+v covers what it is about", a, x, b)
			}
		}
	}
}

func TestWrapPxKeepsToTheWidthAndKeepsEveryWord(t *testing.T) {
	tw := func(s string) int { return len(s) * 8 }
	text := "A cassette, a progress bar and Play, Rewind, Stop and Pause.\nSecond paragraph here."
	lines := wrapPx(text, 20*8, tw)
	for _, l := range lines {
		if tw(l) > 20*8 {
			t.Errorf("line %q is wider than the limit", l)
		}
	}
	if got, want := strings.Join(strings.Fields(strings.Join(lines, " ")), " "), strings.Join(strings.Fields(text), " "); got != want {
		t.Errorf("words lost or changed: %q, want %q", got, want)
	}
	if len(lines) < 4 {
		t.Errorf("%d lines for two paragraphs of that length: %q", len(lines), lines)
	}
	// A word wider than the limit stays whole.
	if l := wrapPx("abcdefghijklmnopqrstuvwxyz", 5*8, tw); len(l) != 1 || l[0] != "abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("a long word was broken: %q", l)
	}
}
