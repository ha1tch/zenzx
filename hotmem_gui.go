//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"

	"github.com/ha1tch/zenui"
)

// ============================================================================
// The Hot Memory tab of the Binary window: a hex editor over the memory of the machine as it runs.
//
// What it shows is a space (hotmem.go) chosen from a drop-down: the memory as the CPU sees it, or
// one RAM page of the model. The dump is read again every frame, so what the running program does
// shows at once, and the bytes that changed light up for half a second. A byte is written the moment
// its second digit is typed.
//
// Two ways to type: as hex digits, into the hex columns, and as characters, into the column of
// characters. Hex or ASCII is chosen with the two buttons beside the address, by a click in the
// column to type into, or with Tab. The place where the next key goes is a block that flashes: on
// the digit of the byte that is next, or on the character; the same byte is outlined in the other
// column. The dump is drawn in a monospaced face (monofont_gui.go): the one the person chose for
// the program when it is monospaced, else the closest in size that is.
// ============================================================================

const (
	// hotPad is the border round the dump, in pixels.
	hotPad = 4
	// hotFlash is how many frames a byte that changed stays lit.
	hotFlash = 30
	// hotListRows is how many entries the drop-down shows at once.
	hotListRows = 7
	// hotMinRows is the fewest rows the dump is cut down to, on a screen that is short.
	hotMinRows = 4
	// hotBlink is how long the cursor is lit, and then dark, in seconds.
	hotBlink = 0.5
)

// hotMode is how the keyboard types into the dump.
type hotMode int

const (
	hotHex   hotMode = iota // hex digits, two to a byte
	hotASCII                // characters, one to a byte
)

// hotEditor is the tab.
type hotEditor struct {
	*spacePicker // the space showing, chosen from the list
	zx           *ZenZX

	pos  int     // the byte the cursor is on, in the space
	nib  int     // in hex: 0, or 1 when the first digit of a byte has been typed
	mode hotMode // what typing does
	top  int     // the offset of the first row showing: a multiple of perRow

	// idle is the seconds since the cursor moved or a key was typed: the cursor is lit for the
	// first hotBlink of every two.
	idle float32

	perRow, rows int

	goBox *hexBox

	// what was on show last frame, to see what changed
	prev    []byte
	prevTop int
	prevCur int
	age     []int

	note    string
	refused bool

	// geometry, set by layout
	goRect, readRect, gridRect zenui.Rect
	modeRect                   [2]zenui.Rect // the Hex and ASCII buttons
	charW, lineH, uiLineH      int
	scale                      int
}

// newHotEditor makes the tab for machine zx, on its CPU view.
func newHotEditor(zx *ZenZX) *hotEditor {
	h := &hotEditor{zx: zx, spacePicker: newSpacePicker(zx.memory), prevCur: -1, perRow: 8}
	h.goBox = newHexBox(4, 0)
	h.selectSpace(0)
	return h
}

// selectSpace changes to space i, with the cursor at the start of a page, or at 0x8000 on the CPU's.
func (h *hotEditor) selectSpace(i int) {
	h.set(i)
	h.nib, h.top = 0, 0
	h.pos = 0
	if h.space().Page < 0 {
		h.pos = 0x8000
	}
	h.goBox.Set(h.pos)
	h.note, h.refused = "", false
}

// goTo puts the cursor on offset p of the space, and the dump where it can be seen.
func (h *hotEditor) goTo(p int) {
	h.pos = min(max(p, 0), h.size()-1)
	h.nib = 0
	h.idle = 0
	h.reveal()
	h.goBox.Set(h.pos)
}

// setMode chooses whether typing is hex or characters.
func (h *hotEditor) setMode(m hotMode) {
	h.mode, h.nib, h.idle = m, 0, 0
}

// toggleMode is the other way to type: hex, or characters.
func (h *hotEditor) toggleMode() { h.setMode(1 - h.mode) }

// caretOn says whether the cursor is lit now.
func (h *hotEditor) caretOn() bool { return int(h.idle/hotBlink)%2 == 0 }

// reveal scrolls the dump so that the cursor's row is in it.
func (h *hotEditor) reveal() {
	row, top := h.pos/h.perRow, h.top/h.perRow
	switch {
	case row < top:
		top = row
	case row >= top+h.rows:
		top = row - h.rows + 1
	}
	h.setTop(top)
}

// setTop puts the first row showing at row, within the space.
func (h *hotEditor) setTop(row int) {
	total := (h.size() + h.perRow - 1) / h.perRow
	h.top = min(max(row, 0), max(total-h.rows, 0)) * h.perRow
}

// hotPerRow is how many bytes a row can have in width pixels: 16, 8 or 4.
func hotPerRow(charW, width int) int {
	for _, n := range []int{16, 8} {
		if hotRowChars(n)*charW+2*hotPad <= width {
			return n
		}
	}
	return 4
}

// hotRowChars is how many characters wide a row is: the address, two spaces, each byte and a space
// after it, a gap, then the characters.
func hotRowChars(perRow int) int { return 4 + 2 + 3*perRow + 1 + perRow }

// hotHeight is the height the tab takes at the scale: the two rows of controls, and the dump of rows.
func hotHeight(ui, mono zenui.Renderer, scale, rows int) int {
	lh := ui.LineHeight(scale)
	fieldH := lh + lh/2
	rowH := fieldH + lh/3
	return 2*rowH + rows*mono.LineHeight(scale) + 2*hotPad
}

// layout places the tab in body, with labels labelW wide, and a dump of rows rows.
func (h *hotEditor) layout(ui, mono zenui.Renderer, scale int, body zenui.Rect, labelW, rows int) {
	h.scale, h.rows = scale, rows
	h.uiLineH = max(ui.LineHeight(scale), 1)
	h.lineH = max(mono.LineHeight(scale), 1)
	h.charW = max(mono.TextWidth("0", scale), 1)
	was := h.perRow
	h.perRow = hotPerRow(h.charW, body.W)
	if h.perRow != was {
		h.top = h.top / h.perRow * h.perRow
		h.reveal()
	}
	fieldH := h.uiLineH + h.uiLineH/2
	rowH := fieldH + h.uiLineH/3
	sel := zenui.Rect{X: body.X + labelW, Y: body.Y, W: body.W - labelW, H: fieldH}
	goW := hexWidth(ui, scale, 4)
	h.goRect = zenui.Rect{X: sel.X, Y: body.Y + rowH, W: goW, H: fieldH}
	h.gridRect = zenui.Rect{X: body.X, Y: body.Y + 2*rowH, W: hotRowChars(h.perRow)*h.charW + 2*hotPad, H: rows*h.lineH + 2*hotPad}

	// the two buttons that say how typing goes, at the right end of the row of the address
	padX := h.uiLineH / 2
	hexW, ascW := ui.TextWidth("Hex", scale)+2*padX, ui.TextWidth("ASCII", scale)+2*padX
	x := body.X + body.W - hexW - ascW
	h.modeRect = [2]zenui.Rect{
		{X: x, Y: h.goRect.Y, W: hexW, H: fieldH},
		{X: x + hexW, Y: h.goRect.Y, W: ascW, H: fieldH},
	}
	rx := h.goRect.X + goW + h.uiLineH
	h.readRect = zenui.Rect{X: rx, Y: h.goRect.Y, W: max(0, x-h.uiLineH-rx), H: fieldH}
	h.setTop(h.top / h.perRow)

	// the drop-down, under its button, not past the dump's foot
	h.place(sel, h.uiLineH, h.gridRect.Y+h.gridRect.H-(sel.Y+sel.H))
}

// refresh reads what is on show, and lights what changed since the frame before.
func (h *hotEditor) refresh() []byte {
	data := h.zx.memory.hotRead(h.space(), h.top, h.rows*h.perRow)
	if h.prevCur == h.cur && h.prevTop == h.top && len(h.prev) == len(data) && len(h.age) == len(data) {
		for i := range data {
			if h.age[i] > 0 {
				h.age[i]--
			}
			if data[i] != h.prev[i] {
				h.age[i] = hotFlash
			}
		}
	} else {
		h.age = make([]int, len(data))
	}
	h.prev, h.prevTop, h.prevCur = data, h.top, h.cur
	return data
}

// openList opens the drop-down on the space that is showing, with the names as they are now.
func (h *hotEditor) openList() {
	h.open()
	h.goBox.Blur()
}

// editing says that the keyboard is the tab's own: the drop-down is open or the address is typed.
func (h *hotEditor) editing() bool { return h.dropOpen || h.goBox.Active() }

// hexDigit is the value of a hex digit, and whether it is one.
func hexDigit(r rune) (byte, bool) {
	switch {
	case r >= '0' && r <= '9':
		return byte(r - '0'), true
	case r >= 'a' && r <= 'f':
		return byte(r-'a') + 10, true
	case r >= 'A' && r <= 'F':
		return byte(r-'A') + 10, true
	}
	return 0, false
}

// put writes the byte at the cursor that change makes from the one there, and says whether it did:
// not where there is no RAM to write to.
func (h *hotEditor) put(change func(old byte) byte) bool {
	old := h.zx.memory.hotRead(h.space(), h.pos, 1)
	if len(old) == 0 {
		return false
	}
	if !h.zx.memory.hotWritable(h.space(), h.pos) {
		h.note, h.refused = fmt.Sprintf("0x%04X is ROM, or no RAM is paged in there: it can't be written. Choose a RAM page from the list to edit that.", h.pos), true
		return false
	}
	if err := h.zx.memory.hotWrite(h.space(), h.pos, change(old[0])); err != nil {
		h.note, h.refused = plainError(err), true
		return false
	}
	h.note, h.refused = "", false
	h.idle = 0
	return true
}

// typeDigit is a digit of a byte typed: the first sets the high half, the second the low, and
// writes, and the cursor moves on.
func (h *hotEditor) typeDigit(d byte) {
	nib := h.nib
	if !h.put(func(old byte) byte {
		if nib == 0 {
			return d<<4 | old&0x0F
		}
		return old&0xF0 | d
	}) {
		return
	}
	if h.nib == 0 {
		h.nib = 1
		return
	}
	h.nib = 0
	if h.pos < h.size()-1 {
		h.goTo(h.pos + 1)
	}
}

// typeChar is a character typed into the column of characters: the byte is its code, and the
// cursor moves on. Only the printable ASCII ones are bytes of it; the others are not taken.
func (h *hotEditor) typeChar(c rune) {
	if c < 32 || c > 126 {
		return
	}
	if !h.put(func(byte) byte { return byte(c) }) {
		return
	}
	if h.pos < h.size()-1 {
		h.goTo(h.pos + 1)
	}
}

// cellAt is the byte under a point of the dump, which column it is in, and whether the point is on
// one: in the hex columns or in the characters.
func (h *hotEditor) cellAt(x, y int) (pos int, mode hotMode, ok bool) {
	if !h.gridRect.Contains(x, y) {
		return 0, 0, false
	}
	row := (y - h.gridRect.Y - hotPad) / h.lineH
	col := (x - h.gridRect.X - hotPad) / h.charW
	if row < 0 || row >= h.rows {
		return 0, 0, false
	}
	hexAt, charAt := 6, 7+3*h.perRow
	i := -1
	switch {
	case col >= hexAt && col < hexAt+3*h.perRow:
		i, mode = (col-hexAt)/3, hotHex
	case col >= charAt && col < charAt+h.perRow:
		i, mode = col-charAt, hotASCII
	}
	p := h.top + row*h.perRow + i
	if i < 0 || p >= h.size() {
		return 0, 0, false
	}
	return p, mode, true
}

// update runs the tab for a frame, and says whether the Escape key was used up by it.
func (h *hotEditor) update(in zenui.Input, ui zenui.Renderer) (usedEsc bool) {
	textW := func(s string) int { return ui.TextWidth(s, h.scale) }
	h.refresh()
	h.idle += in.DeltaTime

	if h.dropOpen {
		chosen, esc := h.spacePicker.update(in, h.uiLineH)
		if chosen {
			h.selectSpace(h.cur)
		}
		return esc
	}
	if in.MousePressed && h.selRect.Contains(in.MouseX, in.MouseY) {
		h.openList()
		return false
	}
	switch h.goBox.Update(in, h.goRect, h.uiLineH, textW, h.size()-1) {
	case zenui.TextCommitted:
		h.goTo(h.goBox.Value())
	case zenui.TextCancelled:
		h.goBox.Set(h.pos)
		return true
	}
	if h.goBox.Active() {
		return false
	}

	if in.MousePressed {
		for i, r := range h.modeRect {
			if r.Contains(in.MouseX, in.MouseY) {
				h.setMode(hotMode(i))
			}
		}
		if p, m, ok := h.cellAt(in.MouseX, in.MouseY); ok {
			h.pos, h.nib, h.mode, h.idle = p, 0, m, 0
			h.goBox.Set(h.pos)
		}
	}
	if in.WheelY != 0 && h.gridRect.Contains(in.MouseX, in.MouseY) {
		h.setTop(h.top/h.perRow - int(in.WheelY)*3)
	}
	for _, c := range in.Chars {
		switch h.mode {
		case hotHex:
			if d, ok := hexDigit(c); ok {
				h.typeDigit(d)
			}
		case hotASCII:
			h.typeChar(c)
		}
	}
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyLeft, zenui.KeyBackspace:
			h.goTo(h.pos - 1)
		case zenui.KeyRight:
			h.goTo(h.pos + 1)
		case zenui.KeyUp:
			h.goTo(h.pos - h.perRow)
		case zenui.KeyDown:
			h.goTo(h.pos + h.perRow)
		case zenui.KeyPageUp:
			h.goTo(h.pos - h.rows*h.perRow)
		case zenui.KeyPageDown:
			h.goTo(h.pos + h.rows*h.perRow)
		case zenui.KeyHome:
			h.goTo(h.pos / h.perRow * h.perRow)
		case zenui.KeyEnd:
			h.goTo(h.pos/h.perRow*h.perRow + h.perRow - 1)
		}
	}
	return false
}

// focusGo puts the keyboard in the Go to box, with the caret at the end of what is there.
func (h *hotEditor) focusGo(ui zenui.Renderer) {
	h.dropOpen = false
	b := h.goBox.Bounds(h.goRect, h.uiLineH)
	h.goBox.Update(zenui.Input{MousePressed: true, MouseX: b.X + b.W - 1, MouseY: b.Y + b.H/2},
		h.goRect, h.uiLineH, func(s string) int { return ui.TextWidth(s, h.scale) }, h.size()-1)
}

// draw draws the tab: the controls in the program's font, the dump in mono.
func (h *hotEditor) draw(ui, mono zenui.Renderer, th zenui.Theme, labelX int) {
	h.drawControls(ui, th, labelX)
	h.drawGrid(mono, th, h.prev)
}

// drawControls draws the controls: the memory, the address, what is at it, and the Hex and ASCII buttons.
func (h *hotEditor) drawControls(ui zenui.Renderer, th zenui.Theme, labelX int) {
	h.spacePicker.draw(ui, th, h.scale, labelX, "Memory")
	fieldH := h.uiLineH + h.uiLineH/2
	ui.DrawText("Go to", labelX, h.goRect.Y+(fieldH-h.uiLineH)/2, h.scale, th.Text)
	h.goBox.Draw(ui, th, h.goRect, h.scale)
	data := h.prev
	if i := h.pos - h.top; i >= 0 && i < len(data) {
		h.drawReadout(ui, th, data[i])
	} else if b := h.zx.memory.hotRead(h.space(), h.pos, 1); len(b) == 1 {
		h.drawReadout(ui, th, b[0])
	}
	for i, lab := range []string{"Hex", "ASCII"} {
		r := h.modeRect[i]
		col := th.Text
		if int(h.mode) == i {
			ui.FillRect(r, th.SelFill)
			col = th.SelText
		}
		ui.StrokeRect(r, th.Border, 1)
		ui.DrawText(lab, r.X+(r.W-ui.TextWidth(lab, h.scale))/2, r.Y+(r.H-h.uiLineH)/2, h.scale, col)
	}
}

// drawList draws the drop-down when it is open: last, so that it is over whatever is under it.
func (h *hotEditor) drawList(ui zenui.Renderer, th zenui.Theme) {
	h.spacePicker.drawList(ui, th, h.scale)
}

// drawReadout says what is at the cursor, as much of it as the room has: the address, the byte in
// hex and decimal; then the byte alone.
func (h *hotEditor) drawReadout(ui zenui.Renderer, th zenui.Theme, v byte) {
	rom := ""
	if !h.zx.memory.hotWritable(h.space(), h.pos) {
		rom = "  (ROM)"
	}
	for _, s := range []string{
		fmt.Sprintf("0x%04X = 0x%02X = %d%s", h.pos, v, v, rom),
		fmt.Sprintf("0x%02X = %d%s", v, v, rom),
		fmt.Sprintf("0x%02X%s", v, rom),
		fmt.Sprintf("0x%02X", v),
	} {
		if ui.TextWidth(s, h.scale) <= h.readRect.W {
			ui.DrawText(s, h.readRect.X, h.readRect.Y+(h.readRect.H-h.uiLineH)/2, h.scale, th.DimText)
			return
		}
	}
}

// drawGrid draws the dump: each row is the address, the bytes, and the characters. The byte at the
// cursor is outlined in both columns, and the place the next key goes is a block that flashes, in
// the column typing goes into; a byte that changed is lit; one that is in ROM is dim, and has no
// block, as nothing typed will go in it.
func (h *hotEditor) drawGrid(mono zenui.Renderer, th zenui.Theme, data []byte) {
	mono.FillRect(h.gridRect, th.Panel)
	mono.StrokeRect(h.gridRect, th.Border, 1)
	sp := h.space()
	x0, y0 := h.gridRect.X+hotPad, h.gridRect.Y+hotPad
	for r := 0; r < h.rows; r++ {
		base := h.top + r*h.perRow
		if base >= h.size() {
			break
		}
		y := y0 + r*h.lineH
		mono.DrawText(fmt.Sprintf("%04X", base), x0, y, h.scale, th.DirText)
		for i := 0; i < h.perRow; i++ {
			p, k := base+i, r*h.perRow+i
			if p >= h.size() || k >= len(data) {
				break
			}
			hx := x0 + (6+3*i)*h.charW
			cx := x0 + (7+3*h.perRow+i)*h.charW
			writable := h.zx.memory.hotWritable(sp, p)
			fg := th.Text
			switch {
			case h.age[k] > 0:
				fg = th.DirText
			case !writable:
				fg = th.DimText
			}
			ch := "."
			if v := data[k]; v >= 32 && v < 127 {
				ch = string(rune(v))
			}
			hex := fmt.Sprintf("%02X", data[k])
			mono.DrawText(hex, hx, y, h.scale, fg)
			mono.DrawText(ch, cx, y, h.scale, fg)
			if p != h.pos {
				continue
			}
			hexCell := zenui.Rect{X: hx, Y: y, W: 2 * h.charW, H: h.lineH}
			chCell := zenui.Rect{X: cx, Y: y, W: h.charW, H: h.lineH}
			outline := th.SelFill
			if !writable {
				outline = th.DimText
			}
			mono.StrokeRect(hexCell, outline, 1)
			mono.StrokeRect(chCell, outline, 1)
			if !writable || !h.caretOn() {
				continue
			}
			block, text, dx := chCell, ch, 0
			if h.mode == hotHex {
				dx = h.nib * h.charW
				block, text = zenui.Rect{X: hx + dx, Y: y, W: h.charW, H: h.lineH}, hex[h.nib:h.nib+1]
			}
			mono.FillRect(block, th.SelFill)
			mono.DrawText(text, block.X, y, h.scale, th.SelText)
		}
	}
}
