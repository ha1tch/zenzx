//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ha1tch/zenui"
)

// ============================================================================
// The inputs of the Binary window (Snapshot > Load Binary...)
//
// They are made so that what can be typed is only what can be used: a box of hex digits that cannot
// hold more than the memory has, and a list of the memories there are in place of a page number to
// get wrong. What cannot be done with what is chosen is said as it is chosen, and its button is
// off: nothing is typed and then refused.
// ============================================================================

// hexBox is a text box of hex digits, no more than digits of them, whose value is never past its
// limit: a digit that would take it past is a digit that makes it the limit. It shows the digits in
// capitals, and four of them when it is not being edited. Clicking in it selects what is in it, so
// that typing a new value replaces the old one.
type hexBox struct {
	*zenui.TextField
	digits int
	fresh  bool // what is in it is selected: the next digit replaces it
}

// newHexBox makes a box of digits digits, holding value.
func newHexBox(digits, value int) *hexBox {
	f := zenui.NewTextField()
	f.MaxLen = digits
	f.Accept = func(r rune) bool { _, ok := hexDigit(r); return ok }
	b := &hexBox{TextField: f, digits: digits}
	b.Set(value)
	return b
}

// Value is the number in the box: 0 when it has no digits, as it can while it is being edited.
func (b *hexBox) Value() int {
	v, _ := strconv.ParseUint(b.Text, 16, 32)
	return int(v)
}

// Set puts v in the box, in full.
func (b *hexBox) Set(v int) { b.Text = fmt.Sprintf("%0*X", b.digits, v) }

// keep holds the text to its rules: capitals, not past limit, and in full when it is not being edited.
func (b *hexBox) keep(limit int) {
	t := strings.ToUpper(b.Text)
	v, err := strconv.ParseUint(t, 16, 32)
	if err == nil && int(v) > limit {
		t, v = fmt.Sprintf("%X", limit), uint64(limit)
	}
	if !b.Active() {
		t = fmt.Sprintf("%0*X", b.digits, min(int(v), limit))
	}
	b.Text = t
}

// Update runs the box for a frame. limit is the largest value it can hold.
func (b *hexBox) Update(in zenui.Input, area zenui.Rect, lineH int, textW func(string) int, limit int) zenui.TextEvent {
	was := b.Active()
	if was && b.fresh {
		for _, c := range in.Chars {
			if _, ok := hexDigit(c); ok {
				b.Text, b.fresh = "", false
				break
			}
		}
		for _, k := range in.Keys {
			switch k {
			case zenui.KeyBackspace:
				b.Text, b.fresh = "", false
			case zenui.KeyLeft, zenui.KeyRight:
				b.fresh = false
			}
		}
	}
	if was && in.MousePressed && b.Bounds(area, lineH).Contains(in.MouseX, in.MouseY) {
		b.fresh = false // a click in it puts the caret where it was clicked
	}
	ev := b.TextField.Update(in, area, lineH, textW)
	switch {
	case !was && b.Active():
		b.fresh = true
	case !b.Active():
		b.fresh = false
	}
	b.keep(limit)
	return ev
}

// Draw draws the box, with what is selected in the selection colours.
func (b *hexBox) Draw(r zenui.Renderer, th zenui.Theme, area zenui.Rect, scale int) {
	b.TextField.Draw(r, th, area, scale)
	if b.Active() && b.fresh && b.Text != "" {
		lh := r.LineHeight(scale)
		bd := b.Bounds(area, lh)
		sel := zenui.Rect{X: bd.X + max(2, lh/4), Y: bd.Y + (bd.H-lh)/2, W: r.TextWidth(b.Text, scale), H: lh}
		r.FillRect(sel, th.SelFill)
		r.DrawText(b.Text, sel.X, sel.Y, scale, th.SelText)
	}
}

// hexWidth is the width of a box of digits digits.
func hexWidth(r zenui.Renderer, scale, digits int) int {
	return r.TextWidth(strings.Repeat("0", digits), scale) + 2*max(2, r.LineHeight(scale)/4) + r.TextWidth("0", scale)
}

// spacePicker is the choice of a memory: the CPU's 64K as paged in now, or one RAM page of the
// model, in a button that opens a list. A model with no pages has only the first, and no picker is
// shown for it.
type spacePicker struct {
	mem      *SpectrumMemory
	spaces   []hotSpace
	cur      int // the space chosen: an index into spaces
	drop     *zenui.List
	dropOpen bool

	// geometry, set by place
	selRect, popRect zenui.Rect

	// th is the theme as last drawn, for the pictures in the list
	th zenui.Theme
}

// newSpacePicker makes the choice for a memory, on its first space: the CPU's.
func newSpacePicker(m *SpectrumMemory) *spacePicker {
	p := &spacePicker{mem: m, spaces: m.hotSpaces(), drop: zenui.NewList()}
	p.drop.NoToolbar = true
	return p
}

// paged says whether there is a choice to make.
func (p *spacePicker) paged() bool { return len(p.spaces) > 1 }

// space is the one chosen.
func (p *spacePicker) space() hotSpace { return p.spaces[p.cur] }

// page is the RAM page chosen, or -1 for the memory as the CPU sees it.
func (p *spacePicker) page() int { return p.space().Page }

// size is how many bytes the space has: what an address in it runs up to.
func (p *spacePicker) size() int { return p.mem.hotSize(p.space()) }

// set chooses space i, if there is one.
func (p *spacePicker) set(i int) { p.cur = min(max(i, 0), len(p.spaces)-1) }

// place puts the button at sel, and the list under it: as tall as its entries up to hotListRows, and
// no taller than room pixels below the button less a foot.
func (p *spacePicker) place(sel zenui.Rect, lineH, room int) {
	p.selRect = sel
	pad := max(2, lineH/4)
	itemH := 2*lineH + 2*pad
	foot := lineH + 2*pad
	fit := (room - foot) / itemH
	n := min(len(p.spaces), min(hotListRows, max(fit, 3)))
	height := n * itemH
	if len(p.spaces) > n {
		height += foot
	}
	p.popRect = zenui.Rect{X: sel.X, Y: sel.Y + sel.H, W: sel.W, H: height}
}

// open opens the list on the space chosen, with the names as they are now.
func (p *spacePicker) open() {
	p.drop.Items = p.drop.Items[:0]
	for _, sp := range p.spaces {
		p.drop.Items = append(p.drop.Items, zenui.ListItem{Name: p.mem.hotName(sp), Thumb: p.thumb(sp)})
	}
	p.drop.Selected = p.cur
	p.dropOpen = true
}

// thumb draws the picture in a space's entry of the list: its number, or CPU for the CPU's.
func (p *spacePicker) thumb(sp hotSpace) func(r zenui.Renderer, area zenui.Rect) {
	label := "CPU"
	if sp.Page >= 0 {
		label = fmt.Sprint(sp.Page)
	}
	return func(r zenui.Renderer, area zenui.Rect) {
		r.StrokeRect(area, p.th.Border, 1)
		x := area.X + (area.W-r.TextWidth(label, 1))/2
		r.DrawText(label, x, area.Y+(area.H-r.LineHeight(1))/2, 1, p.th.DimText)
	}
}

// update runs the choice for a frame. chosen says that a space was picked, and usedEsc that Escape
// closed the list. While the list is open it has the keyboard and the pointer.
func (p *spacePicker) update(in zenui.Input, lineH int) (chosen, usedEsc bool) {
	if !p.dropOpen {
		if p.paged() && in.MousePressed && p.selRect.Contains(in.MouseX, in.MouseY) {
			p.open()
		}
		return false, false
	}
	if ev, ok := p.drop.Update(in, p.popRect, lineH); ok && ev.Kind == zenui.ListSelect {
		p.cur, p.dropOpen = ev.Index, false
		return true, false
	}
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyUp, zenui.KeyDown:
			p.drop.Step(k)
		case zenui.KeyEnter:
			p.cur, p.dropOpen = p.drop.Selected, false
			return true, false
		case zenui.KeyEscape:
			p.dropOpen = false
			return false, true
		}
	}
	if in.MousePressed && !p.popRect.Contains(in.MouseX, in.MouseY) {
		p.dropOpen = false // a click away, or on the button again, closes it
	}
	return false, false
}

// draw draws the button, with its label at labelX.
func (p *spacePicker) draw(ui zenui.Renderer, th zenui.Theme, scale, labelX int, label string) {
	p.th = th
	lh := ui.LineHeight(scale)
	y := p.selRect.Y + (p.selRect.H-lh)/2
	ui.DrawText(label, labelX, y, scale, th.Text)
	ui.FillRect(p.selRect, th.Panel)
	col := th.Border
	if p.dropOpen {
		col = th.SelFill
	}
	ui.StrokeRect(p.selRect, col, 1)
	name := p.mem.hotName(p.space())
	room := p.selRect.W - 2*hotPad - ui.TextWidth(" v", scale)
	for len(name) > 1 && ui.TextWidth(name, scale) > room {
		name = name[:len(name)-1]
	}
	ui.DrawText(name, p.selRect.X+hotPad, y, scale, th.Text)
	ui.DrawText("v", p.selRect.X+p.selRect.W-hotPad-ui.TextWidth("v", scale), y, scale, th.DimText)
}

// drawList draws the list when it is open: last, so that it is over whatever is under it.
func (p *spacePicker) drawList(ui zenui.Renderer, th zenui.Theme, scale int) {
	if !p.dropOpen {
		return
	}
	ui.FillRect(p.popRect, th.Panel)
	p.drop.Draw(ui, th, p.popRect, scale)
	ui.StrokeRect(p.popRect, th.Border, 1)
}
