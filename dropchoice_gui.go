//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"
	"strings"

	"github.com/ha1tch/zenui"
)

// When a tape, a .scr, a snapshot or a .nex is dropped on a Next sitting at its menu or
// Browser, it could be meant for the SD card (to pick from the Browser later) or
// to be loaded and run at once. dropChoice is the small modal that asks which --
// unless Shift was held at the drop, which means run, so no dialog is shown.
//
// It follows the look of the other windows (nextsetup_gui.go): a panel over a
// darkened backdrop, a title, a line naming the file(s), and two buttons. The
// backdrop uses the shared ModalBackdropDarkening so it dims rather than blacks
// out the machine behind it.

// dropChoiceExts are the dropped-file types that can be either copied to the SD
// card or loaded and run on a Next at an idle prompt: tapes, screen dumps,
// snapshots and Next programs (run by NextZXOS's own .nexload, started from the menus
// for the person -- nexrun.go). Both outcomes are real for these, so a drop offers the
// choice.
var dropChoiceExts = map[string]bool{
	".tap": true, ".tzx": true,
	".scr": true,
	".sna": true, ".z80": true, ".szx": true,
	".nex": true,
}

// dropCopyOnlyExts are types a Next at an idle prompt can only copy to the SD
// card, because ZenZX has no "run" path for them on the Next: a .dsk (real NextZXOS
// mounts disc images as its own drives, but ZenZX does not emulate that for the Next
// yet -- T-72 -- so a dropped disc can only be copied to the card as a file for now).
// There is no second outcome to choose between, so a drop of only these is copied
// straight to the card with no dialog -- it reaches the card instead of being a dead end.
var dropCopyOnlyExts = map[string]bool{
	".dsk": true,
}

// anyDropChoice reports whether any of the dropped files is a type the Next drop
// routing applies to -- either the copy-or-run choice or the copy-only path.
func anyDropChoice(files []string) bool {
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if dropChoiceExts[ext] || dropCopyOnlyExts[ext] {
			return true
		}
	}
	return false
}

// splitDropChoice divides the dropped files into the ones that offer the
// copy-or-run choice (runnable) and the ones that can only be copied to the card
// (copyOnly). Files of neither kind are ignored by the Next drop routing.
func splitDropChoice(files []string) (runnable, copyOnly []string) {
	for _, f := range files {
		switch ext := strings.ToLower(filepath.Ext(f)); {
		case dropChoiceExts[ext]:
			runnable = append(runnable, f)
		case dropCopyOnlyExts[ext]:
			copyOnly = append(copyOnly, f)
		}
	}
	return runnable, copyOnly
}

// dropAction is what is done with a tape, a .scr or a snapshot given to a Next at its menu
// or Browser: ask which, copy it to the SD card, or load and run it. The person sets what
// a plain drop does and what a drop with Shift held does (File > Preferences).
type dropAction int

const (
	dropAsk dropAction = iota
	dropCopy
	dropRun
)

// dropActionNames are the settings file's words for the actions, in order.
var dropActionNames = []string{"ask", "copy", "run"}

func (a dropAction) String() string { return dropActionNames[a] }

// parseDropAction is the action a settings file's word stands for, or def for any other.
func parseDropAction(s string, def dropAction) dropAction {
	for i, n := range dropActionNames {
		if n == s {
			return dropAction(i)
		}
	}
	return def
}

// dropContext is what decides where files given to the program go.
type dropContext struct {
	atPrompt  bool       // a Next sitting at its menu or Browser
	hasSlots  bool       // somewhere to copy to
	shiftDown bool       // Shift was held when the files were given
	plain     dropAction // what a plain drop does at the prompt
	shift     dropAction // what a drop with Shift does there
}

// dropPlan is where each of the files goes.
type dropPlan struct {
	copyToCard []string // to the SD card, at once
	ask        []string // the person is asked: copy, or load and run
	load       []string // the usual way: onto a slot if they are not machine files, else into the machine
}

// planDrop decides where files go. It is the one place that does, for a drop on the window
// and for File > Open alike. Away from a Next's menu everything goes the usual way (a .nex
// is then run after the person is asked whether the busy Next may be reset). At it, a .dsk
// can only be copied -- the Next has no way to run one yet -- so it is copied whatever the
// person has chosen and whether Shift is held; the tapes, screens, snapshots and Next
// programs do what the action in force says; and a file of any other kind is copied as
// well, which is where it would end up in any case.
func planDrop(files []string, c dropContext) dropPlan {
	var p dropPlan
	if !c.atPrompt || !c.hasSlots || !anyDropChoice(files) {
		p.load = files
		return p
	}
	act := c.plain
	if c.shiftDown {
		act = c.shift
	}
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		switch {
		case ext == ".zms":
			p.load = append(p.load, f) // a multisnapshot is the window's own: it is never copied to a card
		case dropChoiceExts[ext]:
			switch act {
			case dropAsk:
				p.ask = append(p.ask, f)
			case dropCopy:
				p.copyToCard = append(p.copyToCard, f)
			default:
				p.load = append(p.load, f)
			}
		default: // a .nex, a .dsk, or a file that is not for the machine at all
			p.copyToCard = append(p.copyToCard, f)
		}
	}
	return p
}

// dropChoice is the copy-or-run modal for files dropped on a Next at its prompt.
type dropChoice struct {
	active bool
	files  []string
	// x is where the files were dropped, so that "Copy to SD Card" goes to the slot they
	// were dropped on and not to whichever is under the button that was pressed.
	x int32
	// reset is the other question the dialog asks: the Next is busy, may it be reset to run
	// the file (files[0])? The left button then cancels and the right one says yes.
	reset bool

	panel          zenui.Rect
	copyBtn        zenui.Rect
	runBtn         zenui.Rect
	mouseX, mouseY int
}

// dropChoiceResult is what the dialog decided once it closes.
type dropChoiceResult int

const (
	dropChoicePending dropChoiceResult = iota // still open
	dropChoiceCopy                            // copy to the SD card
	dropChoiceRun                             // load and run
	dropChoiceCancel                          // dismissed, do nothing
)

// open arms the dialog for a set of dropped files.
func (d *dropChoice) open(files []string, x int32) {
	d.reset = false
	d.x = x
	d.files = append(d.files[:0], files...)
	d.active = true
}

// openReset arms the dialog to ask whether the busy Next may be reset to run file.
func (d *dropChoice) openReset(file string) {
	d.open([]string{file}, -1)
	d.reset = true
}

// Active reports whether the dialog is open (and so owns input this frame).
func (d *dropChoice) Active() bool { return d != nil && d.active }

// message names the dropped file(s) for the dialog body.
func (d *dropChoice) message() string {
	switch len(d.files) {
	case 0:
		return ""
	case 1:
		return filepath.Base(d.files[0])
	default:
		return countInWords(len(d.files)) + " files"
	}
}

// layout places the panel and buttons centred for the current window size.
func (d *dropChoice) layout(r zenui.Renderer, screenW, screenH int) {
	lh := r.LineHeight(1)
	pad := lh
	pw := screenW * 3 / 5
	if min := 320; pw < min {
		pw = min
	}
	if pw > screenW-2*pad {
		pw = screenW - 2*pad
	}
	ph := pad*2 + r.LineHeight(2) + lh*2 + pad*2 + lh + pad
	px := (screenW - pw) / 2
	py := (screenH - ph) / 2
	d.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}

	bw := (pw - 3*pad) / 2
	bh := lh + pad
	by := py + ph - pad - bh
	d.copyBtn = zenui.Rect{X: px + pad, Y: by, W: bw, H: bh}
	d.runBtn = zenui.Rect{X: px + pad*2 + bw, Y: by, W: bw, H: bh}
}

// Draw renders the dialog over the running machine.
func (d *dropChoice) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	if !d.active {
		return
	}
	d.paint(r, screenW, screenH, theme)
}

// paint draws the dialog as it was last asked, whether it is open or not: it is what the fade
// of a closed dialog draws once more (dialogfade_gui.go).
func (d *dropChoice) paint(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	d.layout(r, screenW, screenH)
	lh := r.LineHeight(1)
	pad := lh

	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(d.panel, zenui.ScaleAlpha(theme.Panel, zenui.ModalPanelOpacity))
	r.StrokeRect(d.panel, theme.Border, 1)

	x := d.panel.X + pad
	y := d.panel.Y + pad
	title, question, left, right := "Dropped on the Next", "Copy it to the SD card, or load and run it?", "Copy to SD Card", "Load and Run"
	if d.reset {
		title, question, left, right = "Run on the Next", "The Next is busy. Reset it and run this from a fresh start?", "Cancel", "Reset and Run"
	}
	r.DrawText(title, x, y, 2, theme.Text)
	y += r.LineHeight(2) + pad
	r.DrawText(d.message(), x, y, 1, theme.DimText)
	y += lh
	r.DrawText(question, x, y, 1, theme.DimText)

	hot := func(rec zenui.Rect) zenui.Colour {
		if rec.Contains(d.mouseX, d.mouseY) {
			return theme.ButtonHot
		}
		return theme.Button
	}
	drawBtn := func(rec zenui.Rect, label string) {
		r.FillRect(rec, hot(rec))
		r.StrokeRect(rec, theme.Border, 1)
		r.DrawText(label, rec.X+(rec.W-r.TextWidth(label, 1))/2, rec.Y+(rec.H-lh)/2, 1, theme.ButtonText)
	}
	drawBtn(d.copyBtn, left)
	drawBtn(d.runBtn, right)
}

// Update handles input for the open dialog and returns the decision. Escape (or a
// click outside the panel) cancels.
func (d *dropChoice) Update(in zenui.Input) dropChoiceResult {
	if !d.active {
		return dropChoicePending
	}
	d.mouseX, d.mouseY = in.MouseX, in.MouseY
	if pressed(in, zenui.KeyEscape) {
		d.close()
		return dropChoiceCancel
	}
	if in.MousePressed {
		switch {
		case d.copyBtn.Contains(in.MouseX, in.MouseY):
			d.close()
			if d.reset {
				return dropChoiceCancel
			}
			return dropChoiceCopy
		case d.runBtn.Contains(in.MouseX, in.MouseY):
			d.close()
			return dropChoiceRun
		case !d.panel.Contains(in.MouseX, in.MouseY):
			d.close()
			return dropChoiceCancel
		}
	}
	return dropChoicePending
}

// takeFiles returns the pending files and clears them.
func (d *dropChoice) takeFiles() []string {
	f := d.files
	d.files = nil
	return f
}

func (d *dropChoice) close() { d.active = false }
