//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// ============================================================================
// The Tapes window (Tape > Manage Tapes...)
//
// It lists the tapes in the tapes folder (tapedir.go) and the tapes loaded from elsewhere, the way
// the Screenshots window lists screenshots, and asks the questions that window asks: a name is asked
// for and then confirmed, and a delete asks for the word DELETE. Each tape in the list shows the
// first screen on it, as a grid of three across or as rows; and below the list, for the tape that is
// chosen, the blocks the tape holds. Load puts the chosen tape in the deck, as Load Tape does.
//
// What is read of a tape (tapeinfo.go) is read when it first comes on show, a couple a frame so
// that a folder of many does not stop the window, and the chosen one at once. Thumbnails that have
// not been on show for a while are given back.
// ============================================================================

// tapesResult is what a frame of the window ended in.
type tapesResult int

const (
	tapesOpen   tapesResult = iota // still on screen
	tapesClosed                    // closed
	tapesLoad                      // the tape in chosen is to be loaded
)

// The window's buttons.
const (
	tapesBtnLoad   = "load"
	tapesBtnRename = "rename"
	tapesBtnDelete = "delete"
	tapesBtnLayout = "layout"
	tapesBtnClose  = "close"

	tapesRowLines   = 6 // rows of the list on show, as rows
	tapesGridRows   = 2 // and as the grid
	tapesBlockLines = 7 // blocks on show
)

// tapeEntry is what the window has read of one tape file: its blocks and its first screen.
type tapeEntry struct {
	sum  tapeSummary
	err  string      // why the file could not be read
	rows []tapeLine  // the blocks
	pic  *image.RGBA // the first screen, when there is one
	tex  rl.Texture2D
	made bool // tex is a texture
	used int  // the frame it was last drawn in
}

// tapesWindow is the Tapes window.
type tapesWindow struct {
	dir     string
	recent  func() []string // the tapes loaded lately, newest first
	renamed func(old, target string)
	all     []tapeFile
	list    *zenui.List
	buttons *zenui.ButtonBar
	ask     *cardQuestion
	note    string

	entries map[string]*tapeEntry // by the file's path, size and date: a file that changed is read again
	frame   int                   // frames the window has run
	budget  int                   // tapes that may still be read this frame
	clock   float32
	click   struct {
		item int
		at   float32
	}
	noGL   bool   // no window to make textures in: a test
	chosen string // the path Load chose

	// The block list: the first block on show, and whether the keys are its (Tab) and not the list's.
	blockTop    int
	blocksFocus bool
	lastSel     int

	panel, listArea, blockArea, barArea zenui.Rect
	head                                []string
	scale, lineH, blockLines            int
}

// newTapesWindow opens the window on the folder dir and the tapes recent gives, as the grid.
// renamed is told when a tape is renamed.
func newTapesWindow(dir string, recent func() []string, renamed func(old, target string)) *tapesWindow {
	w := &tapesWindow{dir: dir, recent: recent, renamed: renamed, list: zenui.NewList(), buttons: zenui.NewButtonBar(),
		entries: map[string]*tapeEntry{}, blockLines: tapesBlockLines, lastSel: -2}
	w.list.NoToolbar = true
	w.list.Columns = 3
	w.click.item = -1
	w.refresh()
	return w
}

// entryKey is what a tape's entry is kept under.
func entryKey(f tapeFile) string {
	return fmt.Sprintf("%s|%d|%d", f.Path, f.Size, f.When.UnixNano())
}

// refresh reads the folder and the recent tapes again and keeps the selection on the same tape.
// Entries of files that are gone, or have changed, are given back.
func (w *tapesWindow) refresh() {
	keep := ""
	if w.list.Selected >= 0 && w.list.Selected < len(w.all) {
		keep = w.all[w.list.Selected].Path
	}
	var recent []string
	if w.recent != nil {
		recent = w.recent()
	}
	w.all = tapeLibrary(w.dir, recent)
	w.list.Selected = -1
	live := map[string]bool{}
	for i, f := range w.all {
		live[entryKey(f)] = true
		if f.Path == keep {
			w.list.Selected = i
		}
	}
	if w.list.Selected < 0 && len(w.all) > 0 {
		w.list.Selected = 0
	}
	for k, e := range w.entries {
		if !live[k] {
			w.dropEntry(k, e)
		}
	}
}

func (w *tapesWindow) dropEntry(k string, e *tapeEntry) {
	if e.made && !w.noGL {
		rl.UnloadTexture(e.tex)
	}
	delete(w.entries, k)
}

// close gives back every texture the window made. The bar calls it when the window ends.
func (w *tapesWindow) close() {
	for k, e := range w.entries {
		w.dropEntry(k, e)
	}
}

// selectPath puts the selection on the tape at path.
func (w *tapesWindow) selectPath(path string) {
	for i, f := range w.all {
		if f.Path == path {
			w.list.Selected = i
		}
	}
}

// selected is the chosen tape, if there is one.
func (w *tapesWindow) selected() (tapeFile, bool) {
	if w.list.Selected < 0 || w.list.Selected >= len(w.all) {
		return tapeFile{}, false
	}
	return w.all[w.list.Selected], true
}

// stem is a tape's name without its extension.
func tapeStem(name string) string { return strings.TrimSuffix(name, filepath.Ext(name)) }

// tapeCells are the columns of a tape's row: its name without the extension, its format, its size
// and the day it was last changed.
func tapeCells(f tapeFile) [4]string {
	return [4]string{tapeStem(f.Name), strings.ToUpper(f.Ext), humanBytes(f.Size), f.When.Format("2006-01-02")}
}

// tapeRow is a tape's line in the list as rows, when the widths of the columns are not known.
func tapeRow(f tapeFile) string {
	c := tapeCells(f)
	return c[0] + "  " + c[1] + "  " + c[2] + "  " + c[3]
}

// tapeCaption is a tape's name under its picture in the grid.
func tapeCaption(f tapeFile) string { return tapeStem(f.Name) + " " + strings.ToUpper(f.Ext) }

// entryFor is what has been read of f: it is read now when it has not been and the frame has not
// read its share (or force says so), and nil when it has to wait for a later frame.
func (w *tapesWindow) entryFor(f tapeFile, force bool) *tapeEntry {
	k := entryKey(f)
	if e := w.entries[k]; e != nil {
		return e
	}
	if !force && w.budget <= 0 {
		return nil
	}
	w.budget--
	e := &tapeEntry{}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		e.err = plainError(err)
	} else {
		e.sum = summariseTape(data, f.Ext == "tzx")
		e.rows = e.sum.Lines
		if e.sum.Screen != nil {
			if img, serr := decodeShotSCR(e.sum.Screen); serr == nil {
				e.pic = img
				if !w.noGL {
					e.tex = uploadTexture(img, nil)
					rl.SetTextureFilter(e.tex, rl.FilterBilinear)
					e.made = true
				}
			}
		}
	}
	w.entries[k] = e
	return e
}

// detail is the note for the chosen tape: where it is, and what is on it.
func (w *tapesWindow) detail(f tapeFile) (where, what string) {
	where = f.Path
	e := w.entries[entryKey(f)]
	switch {
	case e == nil:
		return where, "Reading the tape..."
	case e.err != "":
		return where, "This file cannot be read: " + e.err
	}
	n := len(e.rows)
	what = fmt.Sprintf("%s, %s, %d blocks. ", strings.ToUpper(f.Ext), humanBytes(f.Size), n)
	switch {
	case e.sum.ScreenAt > 0:
		what += fmt.Sprintf("The screen shown is in block %d.", e.sum.ScreenAt)
	default:
		what += "There is no screen on it."
	}
	if e.sum.Partial != "" {
		what += " The file ends early: " + e.sum.Partial + "."
	}
	return where, what
}

// setButtons makes the row of buttons: what a button may do is decided here, and do acts only on
// enabled ones.
func (w *tapesWindow) setButtons() {
	_, ok := w.selected()
	layout := "Rows"
	if w.list.Columns == 1 {
		layout = "Grid"
	}
	w.buttons.Buttons = []zenui.BarButton{
		{ID: tapesBtnLoad, Label: "Load", Disabled: !ok},
		{ID: tapesBtnRename, Label: "Rename", Disabled: !ok},
		{ID: tapesBtnDelete, Label: "Delete", Disabled: !ok},
		{ID: tapesBtnLayout, Label: layout},
		{ID: tapesBtnClose, Label: "Close"},
	}
}

// do carries out a button.
func (w *tapesWindow) do(id string) tapesResult {
	for _, b := range w.buttons.Buttons {
		if b.ID == id && b.Disabled {
			return tapesOpen
		}
	}
	f, _ := w.selected()
	switch id {
	case tapesBtnClose:
		return tapesClosed
	case tapesBtnLoad:
		w.chosen = f.Path
		return tapesLoad
	case tapesBtnLayout:
		if w.list.Columns == 1 {
			w.list.Columns = 3
		} else {
			w.list.Columns = 1
		}
	case tapesBtnRename:
		w.ask = &cardQuestion{kind: askRenameName, path: f.Path, size: f.Size, text: tapeStem(f.Name),
			noun: "tape", ext: filepath.Ext(f.Name)}
		w.ask.build()
	case tapesBtnDelete:
		w.ask = &cardQuestion{kind: askDelete, path: f.Path, size: f.Size,
			noun: "tape", sizeText: humanBytes(f.Size)}
		w.ask.build()
	}
	return tapesOpen
}

// updateAsk runs the question for a frame. Enter answers it as the right-hand button does,
// Escape and the left-hand button drop it.
func (w *tapesWindow) updateAsk(in zenui.Input) {
	q := w.ask
	status := q.modal.Update(in)
	enter := false
	for _, k := range in.Keys {
		if k == zenui.KeyEnter {
			enter = true
		}
	}
	switch {
	case status == zenui.Cancelled:
		w.ask = nil
		if q.kind == askDelete {
			w.note = fmt.Sprintf("Not deleted: %s is as it was.", filepath.Base(q.path))
		} else {
			w.note = fmt.Sprintf("Not renamed: %s is as it was.", filepath.Base(q.path))
		}
	case status == zenui.Accepted || enter:
		w.answer()
	}
}

// answer takes the answer the person has given to the question.
func (w *tapesWindow) answer() {
	q := w.ask
	switch q.kind {
	case askRenameName:
		target, why := renameTargetExt(q.path, q.text, q.ext)
		if why != "" {
			q.why = why
			q.build()
			return
		}
		q.kind, q.target, q.why = askRenameSure, target, ""
		q.build()
	case askRenameSure:
		w.ask = nil
		w.rename(q.path, q.target)
	case askDelete:
		if q.text != deleteWord {
			q.why = "That is not " + deleteWord + ": type it in capitals, exactly."
			q.build()
			return
		}
		w.ask = nil
		w.remove(q.path)
	}
}

// rename renames the tape at path to target, and keeps the selection on it.
func (w *tapesWindow) rename(path, target string) {
	if err := renameFile(path, target); err != nil {
		w.note = "Not renamed: " + plainError(err)
		return
	}
	if w.renamed != nil {
		w.renamed(path, target)
	}
	w.refresh()
	w.selectPath(target)
	w.note = fmt.Sprintf("%s is now called %s.", filepath.Base(path), filepath.Base(target))
	sayf("Tape renamed: %s to %s\n", path, target)
}

// remove deletes the tape at path, and puts the selection on the one that took its place.
func (w *tapesWindow) remove(path string) {
	at := w.list.Selected
	if err := os.Remove(path); err != nil {
		w.note = "Not deleted: " + plainError(err)
		return
	}
	w.refresh()
	w.list.Selected = min(max(at, 0), len(w.all)-1)
	w.note = fmt.Sprintf("%s is deleted.", filepath.Base(path))
	sayf("Tape deleted: %s\n", path)
}

// --- the pictures ----------------------------------------------------------------------------

// drawThumb draws f's first screen in area: the list calls it for the items on show.
func (w *tapesWindow) drawThumb(r zenui.Renderer, f tapeFile, area zenui.Rect) {
	r.FillRect(area, zenui.Colour{A: 0xFF})
	e := w.entryFor(f, false)
	say := func(s string) {
		s = fitHead(r, s, area.W, w.scale)
		r.DrawText(s, area.X+(area.W-r.TextWidth(s, w.scale))/2, area.Y+(area.H-r.LineHeight(w.scale))/2, w.scale, zenui.Colour{R: 0x99, G: 0x99, B: 0x99, A: 0xFF})
	}
	switch {
	case e == nil:
		say("...")
	case e.err != "":
		say("?")
	case e.pic == nil:
		say("No screen")
	case e.made:
		e.used = w.frame
		pw, ph := e.pic.Bounds().Dx(), e.pic.Bounds().Dy()
		x, y, dw, dh := fitRect(area, pw, ph)
		rl.DrawTexturePro(e.tex, rl.Rectangle{Width: float32(pw), Height: float32(ph)},
			rl.Rectangle{X: x, Y: y, Width: dw, Height: dh}, rl.Vector2{}, 0, rl.White)
	}
}

// evict gives back the textures of the tapes that were not on show this frame, once there are many.
func (w *tapesWindow) evict() {
	if len(w.entries) <= thumbCacheMax {
		return
	}
	for k, e := range w.entries {
		if e.used != w.frame && k != "" {
			w.dropEntry(k, e)
		}
	}
}

// --- the window ------------------------------------------------------------------------------

// headLines is the text above the list.
func (w *tapesWindow) headLines() []string {
	n := len(w.all)
	noun := "tapes"
	if n == 1 {
		noun = "tape"
	}
	dir := w.dir
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return []string{
		"Folder: " + dir + "/ (and tapes from elsewhere)",
		fmt.Sprintf("%d %s. Load puts the chosen one in the deck.", n, noun),
	}
}

// alignedRows are the rows of the list as rows, with each column as wide as its widest cell, so that
// the columns stand one under another. A name too long for the room it has is cut short.
func (w *tapesWindow) alignedRows(r zenui.Renderer) []string {
	const columns = 4
	cells := make([][columns]string, len(w.all))
	var wide [columns]int
	for i, f := range w.all {
		cells[i] = tapeCells(f)
		for j, s := range cells[i] {
			wide[j] = max(wide[j], r.TextWidth(s, w.scale))
		}
	}
	sp := max(r.TextWidth(" ", w.scale), 1)
	gap := 2 * sp
	if w.listArea.W > 0 {
		pad := max(2, w.lineH/4)
		room := w.listArea.W - 2*w.lineH - 3*pad - wide[1] - wide[2] - wide[3] - 3*gap
		room = max(room, 8*sp)
		if wide[0] > room {
			wide[0] = room
			for i := range cells {
				cells[i][0] = fitHead(r, cells[i][0], room, w.scale)
			}
		}
	}
	pad := func(s string, width int, right bool) string {
		n := max((width-r.TextWidth(s, w.scale))/sp, 0)
		if right {
			return strings.Repeat(" ", n) + s
		}
		return s + strings.Repeat(" ", n)
	}
	rows := make([]string, len(cells))
	for i, c := range cells {
		rows[i] = pad(c[0], wide[0], false) + "  " + pad(c[1], wide[1], false) + "  " + pad(c[2], wide[2], true) + "  " + pad(c[3], wide[3], false)
	}
	return rows
}

// captionFor is the name under the picture of f in the grid: cut short to fit under it, but never so
// that the format is lost.
func (w *tapesWindow) captionFor(r zenui.Renderer, f tapeFile) string {
	who := tapeCaption(f)
	if r == nil || w.scale == 0 || w.listArea.W == 0 {
		return who
	}
	kind := " " + strings.ToUpper(f.Ext)
	who = strings.TrimSuffix(who, kind)
	room := w.listArea.W/3 - 2*max(2, w.lineH/4) - r.TextWidth(kind, w.scale)
	return fitHead(r, who, room, w.scale) + kind
}

// fillItems makes the list's items from the tapes. r measures the text; with none (a test with no
// renderer) the names are not padded.
func (w *tapesWindow) fillItems(r zenui.Renderer) {
	w.list.Items = w.list.Items[:0]
	var rows []string
	if w.list.Columns == 1 && r != nil && w.scale > 0 {
		rows = w.alignedRows(r)
	}
	for i, f := range w.all {
		f := f
		name := w.captionFor(r, f)
		if w.list.Columns == 1 {
			name = tapeRow(f)
			if rows != nil {
				name = rows[i]
			}
		}
		w.list.Items = append(w.list.Items, zenui.ListItem{Name: name,
			Thumb: func(r zenui.Renderer, area zenui.Rect) { w.drawThumb(r, f, area) }})
	}
}

// blocksOf is the blocks of the chosen tape, none when it is not read or cannot be.
func (w *tapesWindow) blocksOf() []tapeLine {
	f, ok := w.selected()
	if !ok {
		return nil
	}
	if e := w.entries[entryKey(f)]; e != nil {
		return e.rows
	}
	return nil
}

// scrollBlocks moves the first block on show by d, and keeps it where there are blocks.
func (w *tapesWindow) scrollBlocks(d int) {
	last := max(len(w.blocksOf())-w.blockLines, 0)
	w.blockTop = min(max(w.blockTop+d, 0), last)
}

// layout places the window's parts.
func (w *tapesWindow) layout(r zenui.Renderer, screenW, screenH int) {
	w.setButtons()
	w.head = w.headLines()
	w.scale = zenui.TextBodyScale(r, screenW)
	w.lineH = max(r.LineHeight(w.scale), 1)
	pad := r.LineHeight(1)
	pw := zenui.TextPanelWidth(r, w.scale)
	if pw > screenW-2*pad {
		pw = screenW - 2*pad
	}
	inner := pw - 2*pad
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	_, barH := w.buttons.Layout(zenui.Rect{W: inner}, w.lineH, textW)
	titleH := r.LineHeight(2) + 8
	itemH := w.lineH + 4
	if len(w.list.Items) > 0 {
		itemH = w.list.ItemRect(0, zenui.Rect{W: inner, H: 1 << 14}, w.lineH).H
	} else if w.list.Columns != 1 {
		itemH = inner/3 + w.lineH + 3*max(2, w.lineH/4)
	}
	rows := tapesRowLines
	if w.list.Columns != 1 {
		rows = tapesGridRows
	}
	listH := rows*itemH + 2
	blocks := tapesBlockLines
	boxH := func(n int) int { return n*w.lineH + 6 }
	// the title, the head, the list, the line over the blocks, the blocks, two lines of note, the buttons
	fixed := func(n int) int {
		return 2*pad + titleH + len(w.head)*w.lineH + pad + pad + w.lineH + boxH(n) + pad + 2*w.lineH + pad + barH
	}
	if over := fixed(blocks) + listH - (screenH - 2*pad); over > 0 {
		listH = max(listH-over, min(itemH, 2*(w.lineH+4)))
	}
	for blocks > 3 && fixed(blocks)+listH > screenH-2*pad {
		blocks--
	}
	w.blockLines = blocks
	ph := fixed(blocks) + listH
	px, py := (screenW-pw)/2, max((screenH-ph)/2, 0)
	w.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}
	y := py + pad + titleH + len(w.head)*w.lineH + pad
	w.listArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: listH}
	y += listH + pad + w.lineH
	w.blockArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: boxH(blocks)}
	y += boxH(blocks) + pad + 2*w.lineH + pad
	w.barArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: barH}
	w.scrollBlocks(0)
}

// Update runs the window for a frame.
func (w *tapesWindow) Update(in zenui.Input, r zenui.Renderer, screenW, screenH int) tapesResult {
	w.frame++
	w.budget = thumbLoadsPerFrame
	w.clock += in.DeltaTime
	if w.ask != nil {
		w.updateAsk(in)
		return tapesOpen
	}
	w.fillItems(r)
	w.layout(r, screenW, screenH)
	if f, ok := w.selected(); ok {
		w.entryFor(f, true) // the chosen one is read at once: its blocks are shown
	}
	n := len(w.all)
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape:
			return tapesClosed
		case zenui.KeyEnter:
			return w.do(tapesBtnLoad)
		case zenui.KeyTab:
			w.blocksFocus = !w.blocksFocus
		case zenui.KeyUp, zenui.KeyDown, zenui.KeyLeft, zenui.KeyRight:
			if w.blocksFocus {
				switch k {
				case zenui.KeyUp, zenui.KeyLeft:
					w.scrollBlocks(-1)
				default:
					w.scrollBlocks(1)
				}
				break
			}
			w.list.Step(k)
			w.list.Reveal(w.listArea, w.lineH)
		case zenui.KeyHome, zenui.KeyEnd, zenui.KeyPageUp, zenui.KeyPageDown:
			if w.blocksFocus {
				switch k {
				case zenui.KeyHome:
					w.blockTop = 0
				case zenui.KeyEnd:
					w.scrollBlocks(1 << 20)
				case zenui.KeyPageUp:
					w.scrollBlocks(-w.blockLines)
				case zenui.KeyPageDown:
					w.scrollBlocks(w.blockLines)
				}
				break
			}
			if n == 0 {
				break
			}
			page := max(1, w.list.Columns) * 2
			switch k {
			case zenui.KeyHome:
				w.list.Selected = 0
			case zenui.KeyEnd:
				w.list.Selected = n - 1
			case zenui.KeyPageUp:
				w.list.Selected = max(w.list.Selected-page, 0)
			case zenui.KeyPageDown:
				w.list.Selected = min(w.list.Selected+page, n-1)
			}
			w.list.Reveal(w.listArea, w.lineH)
		}
	}
	// The wheel over the blocks scrolls them; a click says whose the keys are.
	if in.WheelY != 0 && w.blockArea.Contains(in.MouseX, in.MouseY) {
		w.scrollBlocks(-int(in.WheelY * 3))
	}
	if in.MousePressed {
		switch {
		case w.blockArea.Contains(in.MouseX, in.MouseY):
			w.blocksFocus = true
		case w.listArea.Contains(in.MouseX, in.MouseY):
			w.blocksFocus = false
		}
	}
	if ev, ok := w.list.Update(in, w.listArea, w.lineH); ok && ev.Kind == zenui.ListSelect {
		// Two clicks on one tape, close together, are a Load.
		if ev.Index == w.click.item && w.clock-w.click.at < doubleClickSeconds {
			w.click.item = -1
			return w.do(tapesBtnLoad)
		}
		w.click.item, w.click.at = ev.Index, w.clock
	}
	if w.list.Selected != w.lastSel { // another tape has the focus: its blocks are shown from the first
		w.lastSel = w.list.Selected
		w.blockTop = 0
		w.note = ""
	}
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	if id, ok := w.buttons.Update(in, w.barArea, w.lineH, textW); ok {
		return w.do(id)
	}
	return tapesOpen
}

// drawBlocks draws the blocks of the chosen tape in the box under the list, with the line over it.
func (w *tapesWindow) drawBlocks(r zenui.Renderer, theme zenui.Theme) {
	lines := w.blocksOf()
	f, have := w.selected()
	head := "Blocks"
	if have {
		head = "Blocks of " + f.Name
	}
	if n := len(lines); n > w.blockLines {
		head += fmt.Sprintf("  (%d to %d of %d)", w.blockTop+1, min(w.blockTop+w.blockLines, n), n)
	}
	if w.blocksFocus {
		head += "  The arrow and Page keys scroll"
	} else if len(lines) > w.blockLines {
		head += "  Tab scrolls"
	}
	r.DrawText(fitHead(r, head, w.blockArea.W, w.scale), w.blockArea.X, w.blockArea.Y-w.lineH, w.scale, theme.Text)

	border := theme.Border
	if w.blocksFocus {
		border = theme.DirText
	}
	r.StrokeRect(zenui.Rect{X: w.blockArea.X - 1, Y: w.blockArea.Y - 1, W: w.blockArea.W + 2, H: w.blockArea.H + 2}, border, 1)
	if len(lines) == 0 {
		msg := "No tape is chosen."
		if have {
			msg = "Reading the tape..."
			if e := w.entries[entryKey(f)]; e != nil {
				msg = "No blocks in it."
				if e.err != "" {
					msg = "This file cannot be read: " + e.err
				}
			}
		}
		r.DrawText(fitHead(r, msg, w.blockArea.W-8, w.scale), w.blockArea.X+4, w.blockArea.Y+3, w.scale, theme.DimText)
		return
	}
	// Each column is as wide as its widest cell among the blocks on show, so that they stand one under another.
	last := min(w.blockTop+w.blockLines, len(lines))
	var wNum, wKind, wName int
	for i := w.blockTop; i < last; i++ {
		l := lines[i]
		wNum = max(wNum, r.TextWidth(fmt.Sprint(i+1), w.scale))
		wKind = max(wKind, r.TextWidth(l.Kind, w.scale))
		wName = max(wName, r.TextWidth(asciiOnly(l.Name), w.scale))
	}
	gap := 2 * max(r.TextWidth(" ", w.scale), 1)
	x0 := w.blockArea.X + 4
	for i := w.blockTop; i < last; i++ {
		l := lines[i]
		y := w.blockArea.Y + 3 + (i-w.blockTop)*w.lineH
		col := theme.Text
		if f, ok := w.selected(); ok {
			if e := w.entries[entryKey(f)]; e != nil && e.sum.ScreenAt == i+1 {
				col = theme.DirText // the block the picture is from
			}
		}
		num := fmt.Sprint(i + 1)
		r.DrawText(num, x0+wNum-r.TextWidth(num, w.scale), y, w.scale, theme.DimText)
		x := x0 + wNum + gap
		r.DrawText(l.Kind, x, y, w.scale, col)
		x += wKind + gap
		r.DrawText(asciiOnly(l.Name), x, y, w.scale, col)
		x += wName + gap
		r.DrawText(fitHead(r, l.Detail, w.blockArea.X+w.blockArea.W-4-x, w.scale), x, y, w.scale, col)
	}
}

// Draw draws the window over the machine's picture.
func (w *tapesWindow) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	w.fillItems(r)
	w.layout(r, screenW, screenH)
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(w.panel, theme.Panel)
	r.StrokeRect(w.panel, theme.Border, theme.BorderThickness)

	pad := r.LineHeight(1)
	x, y := w.panel.X+pad, w.panel.Y+pad
	r.DrawText("TAPES", x, y, 2, theme.Text)
	y += r.LineHeight(2) + 8
	y = drawDialogHead(r, w.head, x, y, w.panel.W-2*pad, w.scale, w.lineH, theme)

	border := theme.Border
	if !w.blocksFocus {
		border = theme.DirText
	}
	r.StrokeRect(zenui.Rect{X: w.listArea.X - 1, Y: w.listArea.Y - 1, W: w.listArea.W + 2, H: w.listArea.H + 2}, border, 1)
	if len(w.all) == 0 {
		msg := "No tapes yet: put some in " + w.dir + "/, or choose Load Tape in the Tape menu."
		for i, line := range wrapToWidth(r, msg, w.listArea.W-8, w.scale) {
			r.DrawText(line, w.listArea.X+4, w.listArea.Y+4+i*w.lineH, w.scale, theme.DimText)
		}
	}
	w.list.Unfocused = w.blocksFocus
	w.list.Draw(r, theme, w.listArea, w.scale)
	w.evict()

	w.drawBlocks(r, theme)

	noteY := w.blockArea.Y + w.blockArea.H + pad
	var where, what string
	if f, ok := w.selected(); ok {
		where, what = w.detail(f)
	}
	if w.note != "" {
		where, what = w.note, ""
	}
	r.DrawText(fitTail(r, where, w.listArea.W, w.scale), w.listArea.X, noteY, w.scale, theme.DimText)
	r.DrawText(fitHead(r, what, w.listArea.W, w.scale), w.listArea.X, noteY+w.lineH, w.scale, theme.DimText)
	w.buttons.Draw(r, theme, w.barArea, w.scale)
	if w.ask != nil {
		w.ask.draw(r, screenW, screenH, theme)
	}
}

// openTapes opens the Tapes window, and makes the tapes folder when it is not there, so that there is
// a place to put tapes.
func (b *appMenuBar) openTapes(zx *ZenZX) {
	b.closeMenus()
	if err := ensureTapeDir(); err != nil {
		zx.notice("The tapes folder %s could not be made: %v", tapeDir, err)
	}
	b.tapes = newTapesWindow(tapeDir, func() []string { return b.recentTapes },
		func(old, target string) { b.renamedTape(zx, old, target) })
}

// updateTapes runs the Tapes window, gives its textures back when it closes, and loads the tape that
// Load chose.
func (b *appMenuBar) updateTapes(zx *ZenZX) {
	w := b.tapes
	switch w.Update(zenuiraylib.Input(), b.renderer, rl.GetScreenWidth(), rl.GetScreenHeight()) {
	case tapesClosed:
		w.close()
		b.tapes = nil
	case tapesLoad:
		path := w.chosen
		w.close()
		b.tapes = nil
		b.loadTapeAt(path)
	}
}
