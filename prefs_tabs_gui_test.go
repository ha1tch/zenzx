//go:build !headless

package zenzx

import (
	"reflect"
	"testing"

	"github.com/ha1tch/zenui"
)

// The Preferences window is a strip of tabs, and under it the groups of the tab that is open.

func prefsClickAt(w *prefsWindow, r zenui.Renderer, x, y int) prefsResult {
	return w.Update(zenui.Input{MouseX: x, MouseY: y, MouseDown: true, MousePressed: true}, r, 1000, 800)
}

func TestPreferencesTabsGroupEveryRow(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	w := newPrefsWindow(b.prefSections(zx), "", func() {})
	if want := []string{"Appearance", "Machines", "Keyboard", "Files"}; !reflect.DeepEqual(w.tabNames, want) {
		t.Fatalf("tabs %v, want %v", w.tabNames, want)
	}
	if !reflect.DeepEqual(w.tabs.Labels, w.tabNames) {
		t.Errorf("the strip shows %v, the window has %v", w.tabs.Labels, w.tabNames)
	}
	tabOf := map[string]string{}
	for _, s := range w.sections {
		if w.tabIndex(s.tab) < 0 {
			t.Errorf("group %q is on tab %q, which is not in the strip", s.title, s.tab)
		}
		for _, row := range s.rows {
			tabOf[row.label] = s.tab
		}
	}
	for label, tab := range map[string]string{
		"Theme": "Appearance", "Font": "Appearance", "Font zoom": "Appearance",
		"Display zoom": "Appearance", "Fixed menu bar": "Appearance",
		"Machine dock": "Machines", "Several machines": "Machines", "A loaded snapshot": "Machines",
		"A plain drop": "Files", "With Shift held": "Files", "Escape": "Keyboard",
	} {
		if tabOf[label] != tab {
			t.Errorf("%q is on %q, want %q", label, tabOf[label], tab)
		}
	}
}

func TestPreferencesTabClickChoosesATabAndActsOnNothingElse(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	saved := 0
	w := newPrefsWindow(b.prefSections(zx), "", func() { saved++ })
	r := testNoopRenderer{}
	w.layout(r, 1000, 800)
	if w.tabs.Selected != 0 {
		t.Fatalf("the window opens on tab %d, want the first", w.tabs.Selected)
	}
	rects := w.tabs.Layout(w.tabArea, w.lineH)
	for i := 1; i < len(rects); i++ {
		if res := prefsClickAt(w, r, rects[i].X+rects[i].W/2, rects[i].Y+rects[i].H/2); res != prefsOpen {
			t.Fatalf("a click on tab %d closed the window", i)
		}
		if w.tabs.Selected != i {
			t.Errorf("a click on tab %d left tab %d open", i, w.tabs.Selected)
		}
	}
	if saved != 0 {
		t.Errorf("choosing tabs saved %d times; it changes no setting", saved)
	}
}

// A row on a tab that is not open is not there: a click over where it would be does nothing.
func TestPreferencesOnlyTheOpenTabsRowsAct(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	saved := 0
	w := newPrefsWindow(b.prefSections(zx), "", func() { saved++ })
	r := testNoopRenderer{}
	w.layout(r, 1000, 800)
	dock := b.dockOn
	for si, s := range w.sections {
		for ri, row := range s.rows {
			if row.label == "Machine dock" {
				a := w.areas[si][ri]
				prefsClickAt(w, r, a.X+3, a.Y+a.H/2) // the Appearance tab is open
			}
		}
	}
	if b.dockOn != dock || saved != 0 {
		t.Errorf("a click over a row on another tab changed it: dock %v, saved %d", b.dockOn, saved)
	}
}

// Every tab shows in a panel of one size, so that choosing a tab does not move the window,
// and every row is inside it and under the strip.
func TestPreferencesPanelIsTheSameOnEveryTab(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	w := newPrefsWindow(b.prefSections(zx), "/some/where/settings.json", func() {})
	r := testNoopRenderer{}
	for _, size := range [][2]int{{1000, 800}, {800, 600}, {640, 480}} {
		var panel, strip, bar zenui.Rect
		for i := range w.tabNames {
			w.tabs.Selected = i
			w.layout(r, size[0], size[1])
			if i == 0 {
				panel, strip, bar = w.panel, w.tabArea, w.barArea
			} else if w.panel != panel || w.tabArea != strip || w.barArea != bar {
				t.Errorf("%dx%d tab %d: panel %+v strip %+v bar %+v differ from the first tab's %+v %+v %+v",
					size[0], size[1], i, w.panel, w.tabArea, w.barArea, panel, strip, bar)
			}
			if w.panel.H > size[1] || w.panel.W > size[0] {
				t.Errorf("%dx%d tab %d: the panel is %dx%d", size[0], size[1], i, w.panel.W, w.panel.H)
			}
			for si, s := range w.sections {
				if !w.showing(si) {
					continue
				}
				for ri := range s.rows {
					a := w.areas[si][ri]
					if a.Y < w.tabArea.Y+w.tabArea.H || a.Y+a.H > w.barArea.Y {
						t.Errorf("%dx%d tab %d: row %q at %+v is not between the strip and the Close button", size[0], size[1], i, s.rows[ri].label, a)
					}
				}
			}
		}
	}
}

func TestPreferencesKeysMoveBetweenTabs(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	w := newPrefsWindow(b.prefSections(zx), "", func() {})
	r := testNoopRenderer{}
	key := func(k zenui.Key) { w.Update(zenui.Input{Keys: []zenui.Key{k}}, r, 1000, 800) }
	n := len(w.tabNames)
	key(zenui.KeyRight)
	if w.tabs.Selected != 1 {
		t.Errorf("Right: tab %d, want 1", w.tabs.Selected)
	}
	key(zenui.KeyTab)
	if w.tabs.Selected != 2%n {
		t.Errorf("Tab: tab %d, want 2", w.tabs.Selected)
	}
	for w.tabs.Selected != n-1 { // on to the last tab, however many there are
		key(zenui.KeyRight)
	}
	key(zenui.KeyRight)
	if w.tabs.Selected != 0 {
		t.Errorf("Right on the last tab: tab %d, want the first", w.tabs.Selected)
	}
	key(zenui.KeyLeft)
	if w.tabs.Selected != n-1 {
		t.Errorf("Left on the first tab: tab %d, want the last", w.tabs.Selected)
	}
}
