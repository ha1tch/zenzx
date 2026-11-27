//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/fonts"
)

// The Preferences window (File > Preferences...) is where the choices the program keeps
// between sessions are made, all in one place: the ones the Theme and View menus also
// make, and the ones that have no other home. The settings are in tabs, each tab holding
// titled groups (zenui's Tabs, as zenplay's pages use them), and the panel is as tall as the
// tallest tab so that it does not change size as the tabs are chosen. It is a table of rows, and the table is the
// whole of what the window knows: each row says what it is called, what it can be, how to
// read it and how to change it. The window reads every row afresh each frame and changes
// it only by its setter, so it cannot disagree with the menu that sets the same thing, and
// a change made there shows here the next frame.

// prefRow is one setting. A switch has no options; a choice has them, and get returns the
// one in force. A switch reads 1 for on and 0 for off.
type prefRow struct {
	label   string
	options []string
	get     func() int
	set     func(int)
	// off, when it returns text, says why the row cannot be changed now.
	off func() string
	// hint is help text for what the label cannot say. It is hidden until the pointer rests on
	// the row or on its group's title, and then shows in a box (preferences_tips_gui.go).
	hint string
}

// prefSection is a titled group of rows, on one tab of the window. Sections of the same tab
// are shown together, in the order they are listed in; the tabs are in the order their first
// section is.
type prefSection struct {
	tab   string
	title string
	rows  []prefRow
}

type prefsResult int

const (
	prefsOpen prefsResult = iota
	prefsClosed
)

// prefsWindow is the open Preferences window.
type prefsWindow struct {
	sections []prefSection
	// where is the file the choices are saved in, "" for none; save writes them.
	where string
	save  func()

	tabs     *zenui.Tabs // which of the tabs is showing is tabs.Selected
	tabNames []string
	radios   [][]*zenui.RadioGroup // [section][row]; nil for a switch
	checks   [][]*zenui.Checkbox   // [section][row]; nil for a choice
	buttons  *zenui.ButtonBar

	// The layout of the last frame.
	panel   zenui.Rect
	tabArea zenui.Rect
	barArea zenui.Rect
	heads   []zenui.Rect
	areas   [][]zenui.Rect // each row's control
	labels  [][]zenui.Rect // each choice's label
	scale   int
	lineH   int

	// What the pointer is doing (preferences_tips_gui.go).
	handWanted bool // it is over a tab
	tip        prefsTip
}

func newPrefsWindow(sections []prefSection, where string, save func()) *prefsWindow {
	w := &prefsWindow{sections: sections, where: where, save: save, buttons: zenui.NewButtonBar()}
	w.buttons.Buttons = []zenui.BarButton{{ID: "close", Label: "Close"}}
	for _, s := range sections {
		if w.tabIndex(s.tab) < 0 {
			w.tabNames = append(w.tabNames, s.tab)
		}
	}
	w.tabs = &zenui.Tabs{Labels: w.tabNames}
	for _, s := range sections {
		var rs []*zenui.RadioGroup
		var cs []*zenui.Checkbox
		for _, row := range s.rows {
			if row.options != nil {
				rs = append(rs, &zenui.RadioGroup{Options: row.options, Horizontal: true})
				cs = append(cs, nil)
			} else {
				rs = append(rs, nil)
				cs = append(cs, &zenui.Checkbox{Label: row.label})
			}
		}
		w.radios = append(w.radios, rs)
		w.checks = append(w.checks, cs)
	}
	return w
}

// tabIndex is the position of the tab called name, or -1.
func (w *prefsWindow) tabIndex(name string) int {
	for i, n := range w.tabNames {
		if n == name {
			return i
		}
	}
	return -1
}

// showing says whether section si is on the tab that is open.
func (w *prefsWindow) showing(si int) bool {
	return w.tabIndex(w.sections[si].tab) == w.tabs.Selected
}

// sync makes the controls show the rows as they are now.
func (w *prefsWindow) sync() {
	for si, s := range w.sections {
		for ri, row := range s.rows {
			off := row.off != nil && row.off() != ""
			if g := w.radios[si][ri]; g != nil {
				g.Selected = row.get()
				g.Disabled = off
			}
			if c := w.checks[si][ri]; c != nil {
				c.Checked = row.get() != 0
				c.Disabled = off
			}
		}
	}
}

// layout places the window's parts for a screen of this size.
func (w *prefsWindow) layout(r zenui.Renderer, screenW, screenH int) {
	w.sync()
	w.scale = zenui.TextBodyScale(r, screenW)
	w.lineH = max(1, r.LineHeight(w.scale))
	pad := r.LineHeight(1)
	pw := min(zenui.TextPanelWidth(r, w.scale), screenW-2*pad)
	inner := pw - 2*pad
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	rowH := (&zenui.Checkbox{}).Height(w.lineH)

	tabH := w.tabs.Height(w.lineH)

	// The labels of the choices on a tab share a column, so that their controls line up.
	labelW := make([]int, len(w.tabNames))
	for _, s := range w.sections {
		ti := w.tabIndex(s.tab)
		for _, row := range s.rows {
			if row.options != nil {
				labelW[ti] = max(labelW[ti], textW(row.label))
			}
		}
	}
	for i := range labelW {
		labelW[i] += 2 * w.lineH
	}

	// A first pass for the height, and the same walk again for the positions: the panel is
	// centred, and where it starts is not known until how tall it is is. Every tab starts at
	// the same place under the strip, and the panel is as tall as the tallest, so that
	// choosing another tab does not move or resize the window.
	type placed struct {
		y, h int
	}
	var heads []placed
	rows := make([][]placed, len(w.sections))
	tabTop := pad + r.LineHeight(2) + 8
	top := tabTop + tabH + pad
	ys := make([]int, len(w.tabNames))
	for i := range ys {
		ys[i] = top
	}
	for si, s := range w.sections {
		ti := w.tabIndex(s.tab)
		y := ys[ti]
		heads = append(heads, placed{y, zenui.SectionHeight(w.lineH)})
		y += zenui.SectionHeight(w.lineH) + 2
		for _, row := range s.rows {
			h := rowH
			if row.options != nil {
				_, used := (&zenui.RadioGroup{Options: row.options, Horizontal: true}).Layout(zenui.Rect{W: inner - labelW[ti]}, w.lineH, textW)
				h = max(rowH, used)
			}
			rows[si] = append(rows[si], placed{y, h})
			y += h + 2
			y += 4
		}
		ys[ti] = y + pad/2
	}
	y := top
	for _, v := range ys {
		y = max(y, v)
	}
	notesH := 2 * w.lineH
	y += notesH + pad
	_, barH := w.buttons.Layout(zenui.Rect{W: inner}, w.lineH, textW)
	ph := y + barH + pad
	px, py := (screenW-pw)/2, max(0, (screenH-ph)/2)
	w.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}
	w.tabArea = zenui.Rect{X: px + pad, Y: py + tabTop, W: inner, H: tabH}

	w.heads = w.heads[:0]
	w.areas, w.labels = nil, nil
	for si, s := range w.sections {
		ti := w.tabIndex(s.tab)
		w.heads = append(w.heads, zenui.Rect{X: px + pad, Y: py + heads[si].y, W: inner, H: heads[si].h})
		var as, ls []zenui.Rect
		for ri, row := range s.rows {
			p := rows[si][ri]
			x, aw := px+pad, inner
			var label zenui.Rect
			if row.options != nil {
				label = zenui.Rect{X: px + pad, Y: py + p.y, W: labelW[ti], H: p.h}
				x, aw = px+pad+labelW[ti], inner-labelW[ti]
			}
			as = append(as, zenui.Rect{X: x, Y: py + p.y, W: aw, H: p.h})
			ls = append(ls, label)
		}
		w.areas, w.labels = append(w.areas, as), append(w.labels, ls)
	}
	w.barArea = zenui.Rect{X: px + pad, Y: py + ph - pad - barH, W: inner, H: barH}
}

// Update runs the window for a frame. A change is made through its row's setter and then
// saved; the frame ends there, since a setter may have replaced the renderer.
func (w *prefsWindow) Update(in zenui.Input, r zenui.Renderer, screenW, screenH int) prefsResult {
	w.layout(r, screenW, screenH)
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	w.hover(in, textW)
	n := len(w.tabNames)
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape, zenui.KeyEnter:
			return prefsClosed
		case zenui.KeyLeft:
			w.tabs.Selected = (w.tabs.Selected + n - 1) % n
		case zenui.KeyRight, zenui.KeyTab:
			w.tabs.Selected = (w.tabs.Selected + 1) % n
		}
	}
	if _, changed := w.tabs.Update(in, w.tabArea, w.lineH); changed {
		return prefsOpen // the click that chose the tab is not also a click on what is under it
	}
	for si, s := range w.sections {
		if !w.showing(si) {
			continue
		}
		for ri, row := range s.rows {
			area := w.areas[si][ri]
			if g := w.radios[si][ri]; g != nil {
				if idx, changed := g.Update(in, area, w.lineH, textW); changed {
					row.set(idx)
					w.save()
					return prefsOpen
				}
			}
			if c := w.checks[si][ri]; c != nil {
				if v, changed := c.Update(in, area, w.lineH, textW); changed {
					n := 0
					if v {
						n = 1
					}
					row.set(n)
					w.save()
					return prefsOpen
				}
			}
		}
	}
	if _, ok := w.buttons.Update(in, w.barArea, w.lineH, textW); ok {
		return prefsClosed
	}
	return prefsOpen
}

// Draw draws the window over the machine's picture.
func (w *prefsWindow) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	w.layout(r, screenW, screenH)
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(w.panel, theme.Panel)
	r.StrokeRect(w.panel, theme.Border, theme.BorderThickness)
	pad := r.LineHeight(1)
	r.DrawText("PREFERENCES", w.panel.X+pad, w.panel.Y+pad, 2, theme.Text)

	w.tabs.Draw(r, theme, w.tabArea, w.scale)
	for si, s := range w.sections {
		if !w.showing(si) {
			continue
		}
		zenui.DrawSection(r, theme, w.heads[si], w.scale, s.title)
		for ri, row := range s.rows {
			if g := w.radios[si][ri]; g != nil {
				colour := theme.Text
				if g.Disabled {
					colour = theme.Disabled
				}
				l := w.labels[si][ri]
				r.DrawText(row.label, l.X, l.Y+(l.H-w.lineH)/2, w.scale, colour)
				g.Draw(r, theme, w.areas[si][ri], w.scale)
			}
			if c := w.checks[si][ri]; c != nil {
				c.Draw(r, theme, w.areas[si][ri], w.scale)
			}
		}
	}

	y := w.barArea.Y - w.lineH*2 - pad
	kept := "Not saved: there is no settings file for this run."
	if w.where != "" {
		kept = "Saved in " + infoPath(w.where) + " as you change them."
	}
	r.DrawText(kept, w.panel.X+pad, y, w.scale, theme.DimText)
	r.DrawText("Border and the FPS counter are in the View menu, for the session.", w.panel.X+pad, y+w.lineH, w.scale, theme.DimText)
	w.buttons.Draw(r, theme, w.barArea, w.scale)
	w.drawTip(r, screenW, screenH, theme)
}

// openPrefs opens the Preferences window for the machine zx.
func (b *appMenuBar) openPrefs(zx *ZenZX) {
	b.closeMenus()
	b.prefs = newPrefsWindow(b.prefSections(zx), b.settingsPath, func() { b.saveSettings(zx) })
}

// prefSections is the table the Preferences window is made from: the choices the bar
// keeps between sessions, each as the row that reads and changes it.
func (b *appMenuBar) prefSections(zx *ZenZX) []prefSection {
	onOff := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	themes := make([]string, len(zenui.Themes))
	for i, t := range zenui.Themes {
		themes[i] = string(t)
	}
	names := make([]string, len(fonts.All))
	for i, f := range fonts.All {
		names[i] = string(f)
	}
	zooms := make([]string, len(zoomLevels))
	for i, z := range zoomLevels {
		zooms[i] = fmt.Sprintf("X%d", z)
	}
	indexOf := func(n int, of []int) int {
		for i, v := range of {
			if v == n {
				return i
			}
		}
		return 0
	}
	dropNames := []string{"Ask", "Copy to SD card", "Load and run"}
	displayZooms := []int{1, 2, 3}
	displayNames := []string{"X1", "X2", "X3"}

	return []prefSection{
		{"Appearance", "Colours and text", []prefRow{
			{label: "Theme", options: themes,
				get: func() int {
					for i, t := range zenui.Themes {
						if t == b.themeName {
							return i
						}
					}
					return 0
				},
				set: func(i int) { b.ApplyTheme(zenui.Themes[i]) }},
			{label: "Font", options: names,
				get: func() int {
					for i, f := range fonts.All {
						if f == b.fontName {
							return i
						}
					}
					return 0
				},
				set: func(i int) {
					if err := b.ApplyFont(fonts.All[i]); err != nil {
						fmt.Printf("Warning: could not switch to font %s: %v\n", fonts.All[i], err)
						zx.notice("The font %s could not be used: %v", fonts.All[i], err)
					}
				}},
			{label: "Font zoom", options: zooms,
				get:  func() int { return indexOf(b.scale, zoomLevels) },
				set:  func(i int) { b.ApplyZoom(zoomLevels[i]) },
				hint: "The size of the menus' text."},
		}},
		{"Appearance", "Window", []prefRow{
			{label: "Display zoom", options: displayNames,
				get: func() int { return indexOf(zx.window().screen.multiplier, displayZooms) },
				set: func(i int) {
					dm := zx.window()
					n := displayZooms[i]
					if n > dm.maxMultiplierThatFits() || n < dm.MinMultiplierForMode() {
						return // the View menu disables the same sizes
					}
					dm.SetScale(n)
				},
				off: func() string {
					if b.layoutState != nil {
						if l, _ := b.layoutState(); l != layoutSingle {
							return "Not while the machines are tiled."
						}
					}
					return ""
				}},
			{label: "Fixed menu bar", get: func() int { return onOff(b.fixed) },
				set:  func(v int) { b.fixed = v == 1 },
				hint: "Keeps the bar on show at the top of the window."},
		}},
		{"Machines", "Dock and machines", []prefRow{
			{label: "Machine dock", get: func() int { return onOff(b.dockOn) },
				set:  func(v int) { b.dockOn = v == 1 },
				hint: "Slides up from the bottom edge, and with Alt+M."},
			{label: "Several machines", get: func() int { return onOff(b.multiOn) },
				set:  func(v int) { b.multiOn = v == 1 },
				hint: "Lets the dock's New tile add machines. -multi still starts them."},
			{label: "Stop animation", get: func() int { return onOff(b.stopAnimation) },
				set:  func(v int) { b.stopAnimation = v == 1 },
				hint: "When a machine is stopped, a boot comes down and stamps a small Spectrum flat. Off, the machine goes at once."},
		}},
		{"Machines", "Snapshots", []prefRow{
			{label: "A loaded snapshot", options: []string{"Waits paused", "Plays at once"},
				get: func() int {
					if b.snapshotPlay {
						return 1
					}
					return 0
				},
				set:  func(i int) { b.snapshotPlay = i == 1 },
				hint: "What a snapshot does once it is loaded: wait for Play, or run."},
		}},
		{"Machines", "Tape", []prefRow{
			{label: "Autoload tape", get: func() int { return onOff(b.tapeAutoload) },
				set:  func(v int) { b.tapeAutoload = v == 1 },
				hint: "A tape that is loaded starts by itself: the machine types what loads it, as a person would, on its own ROM. Off, it only waits in the deck and you type LOAD \"\" (or choose Tape Loader on a 128K)."},
			{label: "Tape deck", options: []string{"Off", "Top", "Bottom"},
				get:  func() int { return int(b.tapeDeck) },
				set:  func(i int) { b.tapeDeck = tapeDeckPlace(i) },
				hint: "A cassette, a progress bar and Play, Rewind, Stop and Pause, while a tape loads in Accurate mode: in the border, or over the picture when there is none. The wheel over it resizes it, its corner buttons make it small or send it away, and dragging it carries it, and let go it settles at the top or the bottom, whichever it is nearer."},
			{label: "Cassette", options: []string{"Small", "Big", "Hidden"},
				get:  func() int { return int(b.tapeCassette) },
				set:  func(i int) { b.tapeCassette = tapeCassetteSize(i) },
				hint: "Small, beside the buttons; big, under them, with the program's name written on its label; or hidden, leaving the buttons."},
		}},
		{"Machines", "Microdigital clones", []prefRow{
			{label: "Language switch", options: []string{"Spanish", "Portuguese"},
				get: func() int {
					if b.microdigitalNow() {
						return 1
					}
					return 0
				},
				set:  func(i int) { b.setMicrodigital(i == 1) },
				hint: "The diode of the TK90X and TK95 that picked the language of their messages: the ROMs that read it show Portuguese when it pulls bit 7 of the ULA port low, Spanish when it leaves it high, as on every other Spectrum. Only the 48K models read it; the Amstrad and Timex models and the Next are left as they were. -language-switch sets it for one run."},
		}},
		{"Keyboard", "The Escape key", []prefRow{
			{label: "Escape", options: []string{"Disabled", "Closes windows only", "Asks before quitting", "Quits at once"},
				get:  func() int { return int(b.escapeNow()) },
				set:  func(i int) { b.setEscape(escapeMode(i)) },
				hint: "What Escape does when no menu or window is open to close. Disabled, it never quits and leaves the debugger open (menus and windows still close with it); closing windows only, it also closes the debugger; asking, the second press within a moment quits; the last quits at once. -escape sets it for one run."},
		}},
		{"Files", "Dropped on a Next at its menu", []prefRow{
			{label: "A plain drop", options: dropNames,
				get:  func() int { return int(b.dropPlain) },
				set:  func(i int) { b.dropPlain = dropAction(i) },
				hint: "A tape, screen, snapshot or .nex. A .dsk is always copied."},
			{label: "With Shift held", options: dropNames,
				get:  func() int { return int(b.dropShift) },
				set:  func(i int) { b.dropShift = dropAction(i) },
				hint: "The same files, when Shift is down as they are let go."},
		}},
	}
}
