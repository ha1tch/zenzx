//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"
)

// File > Export...
//
// The machine is stopped the moment the item is picked, and what is exported is the machine as
// it was then: the name is asked for after, in the Export form (export_dialog_gui.go), and a
// person may take a while over it. The machine stays stopped while the form is open, and goes on
// as it was (running or paused) when the name is given and the program written, or when the form
// is cancelled. What is written is standalone.go's: the running ZenZX with the machine after it.

// exportJob is an export in progress: the machine as it was when the item was picked.
type exportJob struct {
	program   *Program
	wasPaused bool // whether the machine was already paused, to put it back as it was
}

// beginExport stops the machine, takes its state, and opens the form that asks for the program's name.
func (b *appMenuBar) beginExport(zx *ZenZX) {
	model := b.currentModel()
	if !programModels[model] {
		zx.notice("A program can be exported from the 48K, 128K, +2, +2A and +3 machines")
		return
	}
	wasPaused := zx.paused
	zx.paused = true
	p, err := zx.programState(model)
	if err != nil {
		zx.paused = wasPaused
		zx.notice("The machine could not be exported: %v", err)
		return
	}
	// The window as it stands now goes with the machine: the program starts with the border and
	// the menu bar as they are, and the rest of the menus' preferences.
	p.View = &ProgramView{Settings: b.currentSettings(zx), Border: zx.window().showBorder, FPS: zx.window().showFPS}
	b.export = &exportJob{program: p, wasPaused: wasPaused}
	b.openExportAsk(model)
}

// finishExport writes the program the person named, and lets the machine go on.
func (b *appMenuBar) finishExport(zx *ZenZX, path string) {
	job := b.export
	b.export = nil
	if job == nil {
		return
	}
	defer func() { zx.paused = job.wasPaused }()
	path = ProgramFileName(path)
	b.exportDir = filepath.Dir(path)
	job.program.Title = trimExt(filepath.Base(path))
	stub, err := ownPath()
	if err == nil {
		err = ExportProgram(stub, path, job.program)
	}
	if err != nil {
		zx.notice("The program %s could not be exported: %v", filepath.Base(path), err)
		return
	}
	zx.notice("Exported %s: one file that starts as the machine is now", filepath.Base(path))
}

// cancelExport drops the export, and lets the machine go on.
func (b *appMenuBar) cancelExport(zx *ZenZX) {
	if job := b.export; job != nil {
		b.export = nil
		zx.paused = job.wasPaused
	}
}

// trimExt is a file name without its extension.
func trimExt(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}
