//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ha1tch/zenui"
)

// The Export form (File > Export...): what Export does, in a few lines, a box for the name of
// the program, and the folder it goes to, which is the one the last export went to or the
// folder ZenZX was started in, and which Browse... changes. Nothing else is asked.

// exportAskScale is the text scale of the form: the other dialogues' own.
const exportAskScale = 2

// The form's buttons, left to right. Cancel is also Escape.
const (
	exportBtnCancel = iota
	exportBtnBrowse
	exportBtnExport
)

// exportExplain says what Export does.
const exportExplain = "Export creates a program you can run on its own. It starts with this machine just as it is now, window included."

// exportAsk is the form while it is open.
type exportAsk struct {
	dir  string // the folder, absolute
	name string // what the person has typed
	why  string // why the last answer was refused, shown under the folder
	// replace is the file the person was told is there already: asking again for the same one
	// replaces it.
	replace string

	modal *zenui.Modal
	said  []string // the text of each label field, "" for the box, by field
	whyAt []int    // the fields that say why
}

// openExportAsk opens the form for a machine of model: the name it is given to start with
// says what the program is, and the folder is where the last export went, or here.
func (b *appMenuBar) openExportAsk(model string) {
	dir := b.exportDir
	if st, err := os.Stat(dir); dir == "" || err != nil || !st.IsDir() {
		dir, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	b.exportAsk = &exportAsk{dir: dir, name: "spectrum-" + model}
	b.exportAsk.build()
}

// build makes the form for the state it is in.
func (q *exportAsk) build() {
	var fields []zenui.FormField
	row := 0
	q.said, q.whyAt = q.said[:0], q.whyAt[:0]
	say := func(s string, why bool) {
		for _, line := range zenui.WrapText(s, 44, "") {
			if why {
				q.whyAt = append(q.whyAt, len(fields))
			}
			fields = append(fields, zenui.FormField{Label: line, Type: zenui.FieldLabel, Row: row})
			q.said = append(q.said, line)
			row++
		}
	}
	say(exportExplain, false)
	say("File name:", false)
	q.said = append(q.said, "")
	fields = append(fields, zenui.FormField{Type: zenui.FieldText, Text: &q.name, Row: row, MinWidthCells: 36})
	row++
	say("Folder: "+shortPath(q.dir, 36), false)
	if q.why != "" {
		say(q.why, true)
	}
	q.modal = zenui.NewModal(zenui.ModalConfig{
		Form:    zenui.FormConfig{Title: "EXPORT", Fields: fields},
		Buttons: []string{"Cancel", "Browse...", "Export"}, CancelButtonIndex: exportBtnCancel, Scale: exportAskScale,
	})
}

// shortPath is p, or its last n characters after "..." when it is longer.
func shortPath(p string, n int) string {
	r := []rune(p)
	if len(r) <= n {
		return p
	}
	return "..." + string(r[len(r)-(n-3):])
}

// draw draws the form. The reason an answer was refused is the theme's accent colour, drawn
// again over its label (the form draws every label in one colour).
func (q *exportAsk) draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	q.modal.Draw(r, screenW, screenH, theme)
	lh := r.LineHeight(exportAskScale)
	for _, i := range q.whyAt {
		rec := q.modal.Form().FieldRect(i)
		r.FillRect(rec, theme.Panel)
		r.DrawText(fitHead(r, q.said[i], rec.W, exportAskScale), rec.X, rec.Y+(rec.H-lh)/2, exportAskScale, theme.DirText)
	}
}

// updateExport runs the form for a frame. Escape and Cancel drop the export, Browse... opens
// the file browser, Export (and Enter) writes the program.
func (b *appMenuBar) updateExport(zx *ZenZX, in zenui.Input) {
	q := b.exportAsk
	status := q.modal.Update(in)
	enter := false
	for _, k := range in.Keys {
		if k == zenui.KeyEnter {
			enter = true
		}
	}
	switch {
	case status == zenui.Cancelled:
		b.exportAsk = nil
		b.cancelExport(zx)
	case status == zenui.Accepted && q.modal.Result() == exportBtnBrowse:
		b.browseExport()
	case status == zenui.Accepted || enter:
		b.answerExport(zx)
	}
}

// browseExport opens a file browser on the form's folder. What it answers goes into the form:
// the folder, and the name if one was typed in the browser.
func (b *appMenuBar) browseExport() {
	q := b.exportAsk
	q.build() // a form that has not been answered, so that it is not asked again when the browser is gone
	ext := ""
	if runtime.GOOS == "windows" {
		ext = "exe"
	}
	b.diskDialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:       zenui.ModeSave,
		Title:      "Export: choose a folder",
		StartDir:   q.dir,
		DefaultExt: ext,
		FS:         zenui.OSFS{},

		// The form asks about a file that is there (once, as it always did); the browser does not ask first.
		NoOverwriteConfirm: true,
	})
	b.dialogKind = dialogExportBrowse
}

// exportBrowsed takes the browser's answer, the path of a file in the folder chosen, into the form.
func (b *appMenuBar) exportBrowsed(path string) {
	q := b.exportAsk
	if q == nil {
		return
	}
	q.dir = filepath.Dir(path)
	name := filepath.Base(path)
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(name), ".exe") {
		name = strings.TrimSuffix(name, filepath.Ext(name)) // the system's own extension is added again
	}
	q.name, q.why, q.replace = name, "", ""
	q.build()
}

// exportTarget is the path a program typed as typed is written to in dir, and whether a file
// is there already, or why it cannot have that name or go to that folder.
func exportTarget(dir, typed string) (path string, exists bool, why string) {
	name := strings.TrimSpace(typed)
	switch {
	case name == "":
		return "", false, "Type a name for the program."
	case strings.ContainsAny(name, "/\\\x00"):
		return "", false, "A name has no slashes in it: Browse... chooses the folder."
	case strings.ContainsAny(name, "<>:\"|?*"):
		return "", false, "A name has none of < > : \" | ? *"
	case strings.HasPrefix(name, "."):
		return "", false, "A name does not begin with a dot."
	case len(name) > 100:
		return "", false, "That name is too long."
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", false, "The folder is not there: Browse... chooses another."
	}
	path = filepath.Join(dir, ProgramFileName(name))
	if st, err := os.Lstat(path); err == nil {
		if st.IsDir() {
			return "", false, "A folder is called that already."
		}
		exists = true
	}
	return path, exists, ""
}

// answerExport writes the program the form describes, or says why it cannot be. A file that is
// there already is replaced only when the person has been told so and asks again.
func (b *appMenuBar) answerExport(zx *ZenZX) {
	q := b.exportAsk
	path, exists, why := exportTarget(q.dir, q.name)
	if why != "" {
		q.why, q.replace = why, ""
		q.build()
		return
	}
	if exists && q.replace != path {
		q.replace = path
		q.why = "A file called " + filepath.Base(path) + " is there already. Export again to replace it."
		q.build()
		return
	}
	b.exportAsk = nil
	b.finishExport(zx, path)
}
