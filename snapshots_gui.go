//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// ============================================================================
// The Snapshots window (Snapshot > Manage Snapshots...)
//
// It lists what is in the snapshots folder (snapshotdir.go) and loads, duplicates, renames and
// deletes the chosen one, as the SD Cards and Floppy Disks windows do for theirs, and with the
// same questions: a name is asked for and then confirmed, and a delete asks for the word DELETE
// (sdcards_manage_gui.go). A snapshot is small, so a copy is done at once, with no progress bar.
//
// A snapshot that is renamed or duplicated has a name that Save Timestamped did not make, so
// Load Timestamped does not list it any more: it is here, with the others, and is loaded from
// here. Renaming keeps the format, since the file's name is what says how to read it.
// ============================================================================

// snapshotsResult is what a frame of the window ended in.
type snapshotsResult int

const (
	snapshotsOpen   snapshotsResult = iota // still on screen
	snapshotsClosed                        // closed
	snapshotsLoad                          // closed: the machine is to load loadPath
)

// The window's buttons.
const (
	snapBtnLoad      = "load"
	snapBtnDuplicate = "duplicate"
	snapBtnRename    = "rename"
	snapBtnDelete    = "delete"
	snapBtnClose     = "close"

	snapshotsWindowLines = 10 // rows of the list on show
)

// snapshotsWindow is the Snapshots window.
type snapshotsWindow struct {
	dir  string
	all  []snapshotFile
	list *zenui.List
	// buttons is the row of buttons, ask the question being put (Rename or Delete) and note
	// what the last thing done came to.
	buttons  *zenui.ButtonBar
	ask      *cardQuestion
	note     string
	loadPath string

	panel, listArea, barArea zenui.Rect
	head                     []string
	scale, lineH             int
}

// newSnapshotsWindow opens the window on the folder dir.
func newSnapshotsWindow(dir string) *snapshotsWindow {
	w := &snapshotsWindow{dir: dir, list: zenui.NewList(), buttons: zenui.NewButtonBar()}
	w.list.NoToolbar = true
	w.refresh()
	return w
}

// refresh reads the folder again and keeps the selection on the same snapshot.
func (w *snapshotsWindow) refresh() {
	keep := ""
	if w.list.Selected >= 0 && w.list.Selected < len(w.all) {
		keep = w.all[w.list.Selected].Path
	}
	w.all = listSnapshots(w.dir)
	w.list.Selected = -1
	for i, f := range w.all {
		if f.Path == keep {
			w.list.Selected = i
		}
	}
	if w.list.Selected < 0 && len(w.all) > 0 {
		w.list.Selected = 0
	}
}

// selectPath puts the selection on the snapshot at path.
func (w *snapshotsWindow) selectPath(path string) {
	for i, f := range w.all {
		if f.Path == path {
			w.list.Selected = i
		}
	}
}

// selected is the chosen snapshot, if there is one.
func (w *snapshotsWindow) selected() (snapshotFile, bool) {
	if w.list.Selected < 0 || w.list.Selected >= len(w.all) {
		return snapshotFile{}, false
	}
	return w.all[w.list.Selected], true
}

// snapshotListRow is a snapshot's line in the list: its name, how big it is and when it was made.
// Only ASCII, which is what the typeface has.
func snapshotListRow(f snapshotFile) string {
	name := f.Name
	if len(name) > 30 {
		name = name[:29] + "~"
	}
	return fmt.Sprintf("%-30s %8s  %s", name, humanBytes(f.Size), f.When.Format("2006-01-02 15:04"))
}

// snapshotKind says what a snapshot is among the folder's: the quick save, one that Load
// Timestamped lists, or another.
func snapshotKind(f snapshotFile) string {
	switch {
	case f.Quick:
		return "It is the quick save: Quick Load loads it."
	case f.Timestamped:
		return "Save Timestamped made it: Snapshot > Load Timestamped lists it."
	case f.Format == MultiSnapshotExt:
		return "A multisnapshot: Load closes the machines that are running and starts the ones in it."
	}
	return "Snapshot > Load Timestamped does not list it: its name is not one Save Timestamped gives."
}

// setButtons makes the row of buttons: what a button may do is decided here, and do acts only on
// enabled ones.
func (w *snapshotsWindow) setButtons() {
	_, ok := w.selected()
	w.buttons.Buttons = []zenui.BarButton{
		{ID: snapBtnLoad, Label: "Load", Disabled: !ok},
		{ID: snapBtnDuplicate, Label: "Duplicate", Disabled: !ok},
		{ID: snapBtnRename, Label: "Rename", Disabled: !ok},
		{ID: snapBtnDelete, Label: "Delete", Disabled: !ok},
		{ID: snapBtnClose, Label: "Close"},
	}
}

// do carries out a button.
func (w *snapshotsWindow) do(id string) snapshotsResult {
	for _, b := range w.buttons.Buttons {
		if b.ID == id && b.Disabled {
			return snapshotsOpen
		}
	}
	f, _ := w.selected()
	switch id {
	case snapBtnClose:
		return snapshotsClosed
	case snapBtnLoad:
		w.loadPath = f.Path
		return snapshotsLoad
	case snapBtnDuplicate:
		w.duplicate(f)
	case snapBtnRename:
		stem := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
		w.ask = &cardQuestion{kind: askRenameName, path: f.Path, size: f.Size, text: stem,
			noun: "snapshot", ext: filepath.Ext(f.Name)}
		w.ask.build()
	case snapBtnDelete:
		w.ask = &cardQuestion{kind: askDelete, path: f.Path, size: f.Size,
			noun: "snapshot", sizeText: humanBytes(f.Size)}
		w.ask.build()
	}
	return snapshotsOpen
}

// duplicate copies a snapshot to a new file beside it, never over another.
func (w *snapshotsWindow) duplicate(f snapshotFile) {
	dst := freeImageName(w.dir, copyStem(f.Path), filepath.Ext(f.Path))
	if err := copyCard(context.Background(), f.Path, dst, func(int64, int64) {}); err != nil {
		w.note = "The copy failed: " + plainError(err)
		return
	}
	w.refresh()
	w.selectPath(dst)
	w.note = filepath.Base(dst) + " is a copy of " + f.Name + "."
}

// snapshotRenameTarget is the path a snapshot at old gets when the person types typed as its new
// name, or why it cannot have it. The extension stays what it was: a name typed with another
// snapshot format's is refused, since the extension is what says how the file is read.
func snapshotRenameTarget(old, typed string) (target, why string) {
	ext := filepath.Ext(old)
	name := strings.TrimSpace(typed)
	lower := strings.ToLower(name)
	for _, format := range append(append([]string(nil), snapshotFormats...), MultiSnapshotExt) {
		other := "." + format
		if strings.HasSuffix(lower, other) && !strings.EqualFold(other, ext) {
			return "", "A snapshot stays " + ext + ": its format does not change with its name."
		}
	}
	return renameTargetExt(old, name, ext)
}

// updateAsk runs the question for a frame. Enter answers it as the right-hand button does,
// Escape and the left-hand button drop it.
func (w *snapshotsWindow) updateAsk(in zenui.Input) {
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
func (w *snapshotsWindow) answer() {
	q := w.ask
	switch q.kind {
	case askRenameName:
		target, why := snapshotRenameTarget(q.path, q.text)
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

// rename renames the snapshot at path to target, and keeps the selection on it.
func (w *snapshotsWindow) rename(path, target string) {
	if err := renameFile(path, target); err != nil {
		w.note = "Not renamed: " + plainError(err)
		return
	}
	w.refresh()
	w.selectPath(target)
	w.note = fmt.Sprintf("%s is now called %s.", filepath.Base(path), filepath.Base(target))
	sayf("Snapshot renamed: %s to %s\n", path, target)
}

// remove deletes the snapshot at path, and puts the selection on the one that took its place.
func (w *snapshotsWindow) remove(path string) {
	at := w.list.Selected
	if err := os.Remove(path); err != nil {
		w.note = "Not deleted: " + plainError(err)
		return
	}
	w.refresh()
	w.list.Selected = min(max(at, 0), len(w.all)-1)
	w.note = fmt.Sprintf("%s is deleted.", filepath.Base(path))
	sayf("Snapshot deleted: %s\n", path)
}

// headLines is the text above the list.
func (w *snapshotsWindow) headLines() []string {
	n := len(w.all)
	noun := "snapshots"
	if n == 1 {
		noun = "snapshot"
	}
	return []string{
		"Folder: " + w.dir + "/",
		fmt.Sprintf("%d %s. Load puts the chosen one in the front machine.", n, noun),
	}
}

// layout places the window's parts.
func (w *snapshotsWindow) layout(r zenui.Renderer, screenW, screenH int) {
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
	rowsH := w.lineH + 4
	if len(w.list.Items) > 0 {
		rowsH = w.list.ItemRect(0, zenui.Rect{W: inner, H: 1 << 14}, w.lineH).H
	}
	listH := snapshotsWindowLines*rowsH + 2
	fixed := 2*pad + titleH + len(w.head)*w.lineH + pad + pad + 3*w.lineH + pad + barH
	if over := fixed + listH - (screenH - 2*pad); over > 0 {
		listH -= over
		if listH < 2*(w.lineH+4) {
			listH = 2 * (w.lineH + 4)
		}
	}
	ph := fixed + listH
	px, py := (screenW-pw)/2, max((screenH-ph)/2, 0)
	w.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}
	y := py + pad + titleH + len(w.head)*w.lineH + pad
	w.listArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: listH}
	y += listH + pad + 3*w.lineH + pad
	w.barArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: barH}
}

// Update runs the window for a frame.
func (w *snapshotsWindow) Update(in zenui.Input, r zenui.Renderer, screenW, screenH int) snapshotsResult {
	if w.ask != nil {
		w.updateAsk(in)
		return snapshotsOpen
	}
	w.layout(r, screenW, screenH)
	w.list.Items = w.list.Items[:0]
	for _, f := range w.all {
		w.list.Items = append(w.list.Items, zenui.ListItem{Name: snapshotListRow(f)})
	}
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape:
			return snapshotsClosed
		case zenui.KeyEnter:
			return w.do(snapBtnLoad)
		case zenui.KeyUp, zenui.KeyDown, zenui.KeyHome, zenui.KeyEnd, zenui.KeyPageUp, zenui.KeyPageDown:
			w.list.Step(k)
			w.list.Reveal(w.listArea, w.lineH)
		}
	}
	w.list.Update(in, w.listArea, w.lineH)
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	if id, ok := w.buttons.Update(in, w.barArea, w.lineH, textW); ok {
		return w.do(id)
	}
	return snapshotsOpen
}

// Draw draws the window over the machine's picture.
func (w *snapshotsWindow) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	w.layout(r, screenW, screenH)
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(w.panel, theme.Panel)
	r.StrokeRect(w.panel, theme.Border, theme.BorderThickness)

	pad := r.LineHeight(1)
	x, y := w.panel.X+pad, w.panel.Y+pad
	r.DrawText("SNAPSHOTS", x, y, 2, theme.Text)
	y += r.LineHeight(2) + 8
	y = drawDialogHead(r, w.head, x, y, w.panel.W-2*pad, w.scale, w.lineH, theme)

	r.StrokeRect(zenui.Rect{X: w.listArea.X - 1, Y: w.listArea.Y - 1, W: w.listArea.W + 2, H: w.listArea.H + 2}, theme.Border, 1)
	if len(w.all) == 0 {
		drawWrapped(r, "No snapshots yet: Snapshot > Save Timestamped makes one.", w.listArea.X+4, w.listArea.Y+4, w.listArea.W-8, w.scale, w.lineH, 3, theme.DimText)
	}
	w.list.Draw(r, theme, w.listArea, w.scale)

	note := w.note
	if f, ok := w.selected(); ok && note == "" {
		note = snapshotKind(f)
	}
	noteY := w.listArea.Y + w.listArea.H + pad
	drawWrapped(r, note, w.listArea.X, noteY, w.listArea.W, w.scale, w.lineH, 3, theme.DimText)
	w.buttons.Draw(r, theme, w.barArea, w.scale)
	if w.ask != nil {
		w.ask.draw(r, screenW, screenH, theme)
	}
}

// openSnapshots opens the Snapshots window.
func (b *appMenuBar) openSnapshots(zx *ZenZX) {
	b.closeMenus()
	b.snaps = newSnapshotsWindow(snapshotDir)
}

// updateSnapshots runs the Snapshots window and acts on what it ends in.
func (b *appMenuBar) updateSnapshots(zx *ZenZX) {
	w := b.snaps
	switch w.Update(zenuiraylib.Input(), b.renderer, rl.GetScreenWidth(), rl.GetScreenHeight()) {
	case snapshotsClosed:
		b.snaps = nil
	case snapshotsLoad:
		b.snaps = nil
		if b.loadSnapshotAt(zx, w.loadPath) { // a multisnapshot asks first
			b.noteRecentSnapshot(zx, w.loadPath)
		}
	}
}
