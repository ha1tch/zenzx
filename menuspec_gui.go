//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "github.com/ha1tch/zenui"

// A menu is written once, as a menuSpec: a table of rows each carrying its own label,
// state and behaviour. The zenui items the widget draws and the actions the host runs
// are both made from it, so a row cannot have another row's action, and adding or
// removing a row cannot shift the ones after it out from under their actions. (Menus
// used to be a list of items and a separate list of actions that had to be kept the same
// length by hand; every index in them was a place for the two to drift apart.)

// menuEntry is one row of a menu.
type menuEntry struct {
	label    string
	title    bool // a group heading: not selectable
	sep      bool // a divider: not selectable
	disabled bool
	checked  *bool // a checkmark that follows the pointed-to bool
	toggle   bool  // with checked: a checkbox that flips it and keeps the menu open
	box      bool  // with checked: the checkbox glyph, but choosing the row closes the menu as any row does
	// icon is a picture drawn before the label, as the Machine menu's Pause, Play and Stop have.
	icon func(r zenui.Renderer, cell zenui.Rect, col zenui.Colour)
	sub  menuSpec
	run  func(zx *ZenZX) // what picking the row does; nil for none
	// needsMachine says the row acts on the machine that is running, so that it is disabled
	// with none (the Snapshot menu: snapshotmenu_gui.go).
	needsMachine bool
}

// menuTitle, menuSep and menuRow are the entries' constructors.
func menuTitle(label string) menuEntry { return menuEntry{label: label, title: true} }
func menuSep() menuEntry               { return menuEntry{sep: true} }
func menuRow(label string, run func(zx *ZenZX)) menuEntry {
	return menuEntry{label: label, run: run}
}

// machineRow is a row that acts on the machine that is running.
func machineRow(label string, run func(zx *ZenZX)) menuEntry {
	return menuEntry{label: label, run: run, needsMachine: true}
}

// menuCheckbox is a row that flips *on and keeps the menu open; then, if not nil, runs.
func menuCheckbox(label string, on *bool, run func(zx *ZenZX)) menuEntry {
	return menuEntry{label: label, checked: on, toggle: true, run: run}
}

// menuSpec is a menu: its rows, in order.
type menuSpec []menuEntry

// items are the rows as the widget draws them.
func (s menuSpec) items() []zenui.Item {
	out := make([]zenui.Item, len(s))
	for i, e := range s {
		out[i] = zenui.Item{
			Label:     e.label,
			Title:     e.title,
			Separator: e.sep,
			Disabled:  e.disabled,
			Checked:   e.checked,
			Toggle:    e.toggle,
			Box:       e.box,
			Icon:      e.icon,
		}
		if len(e.sub) > 0 {
			out[i].SubItems = e.sub.items()
		}
	}
	return out
}

// actions are the rows' behaviours, one per row, for a host that dispatches on an index.
// A row with a submenu, a title, a divider or no behaviour does nothing here.
func (s menuSpec) actions() []func(zx *ZenZX) {
	out := make([]func(zx *ZenZX), len(s))
	for i, e := range s {
		run := e.run
		if run == nil || e.title || e.sep || e.disabled || len(e.sub) > 0 {
			run = func(*ZenZX) {}
		}
		out[i] = run
	}
	return out
}

// pick runs the row at index, or the row at sub within its submenu when the row has
// one, and says whether there was one to run. A disabled row, a title and a divider
// are never run, whatever the index says.
func (s menuSpec) pick(zx *ZenZX, index, sub int) bool {
	if index < 0 || index >= len(s) {
		return false
	}
	e := s[index]
	if e.title || e.sep || e.disabled {
		return false
	}
	if len(e.sub) > 0 {
		return e.sub.pick(zx, sub, -1)
	}
	if e.run == nil {
		return false
	}
	e.run(zx)
	return true
}
