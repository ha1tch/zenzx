//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"
	"reflect"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// The file browsers of File > Open and Floppy Disk > Load Disk. They start in the folder of the
// file opened last with each (the home folder when there is none), list the last files opened as
// their Recent place, take a path typed or pasted into the name field, and keep how the person
// looked at a folder (hidden files, details, order) from one time to the next.

// fileDialogKind says which of the two browsers diskDialog is, so that what is chosen in it is kept.
type fileDialogKind int

const (
	fileDlgNone fileDialogKind = iota // another browser (snapshots, cards, the Export form)
	fileDlgOpen                       // File > Open...
	fileDlgDisk                       // Floppy Disk > Load Disk...
	fileDlgTape                       // Tape > Load Tape...: no folder is kept, the recent tapes say where it starts
)

// fileOpenSets are the file types File > Open offers: everything it takes, and then each medium. The
// dialog adds "All files" after them.
func fileOpenSets() []zenui.FilterSet {
	return []zenui.FilterSet{
		{Name: "Spectrum files", Exts: fileOpenExts},
		{Name: "Tapes", Exts: []string{"tap", "tzx"}},
		{Name: "Snapshots", Exts: []string{"sna", "z80", "szx", "zxs", "zms"}},
		{Name: "Disk images", Exts: []string{"dsk"}},
		{Name: "Recordings", Exts: []string{"rzx"}},
		{Name: "Screens", Exts: []string{"scr"}},
		{Name: "Next programs", Exts: []string{"nex"}},
	}
}

// viewFromSettings is the dialog's view as the settings kept it: the default one when none was kept.
// The file type is not kept: each dialog starts on its own first.
func viewFromSettings(v *settingsconfig.FileDialogView) zenui.View {
	if v == nil {
		return zenui.View{}
	}
	key := zenui.SortName
	for i, w := range settingsconfig.FileSortWords {
		if w == v.Sort {
			key = zenui.SortKey(i)
		}
	}
	return zenui.View{ShowHidden: v.Hidden, Details: v.Details, Sort: key, Descending: v.Descending}
}

// settingsFromView is the view for the settings file; nil, which is kept as no key at all, when it is
// the default one.
func settingsFromView(v zenui.View) *settingsconfig.FileDialogView {
	if !v.ShowHidden && !v.Details && v.Sort == zenui.SortName && !v.Descending {
		return nil
	}
	sort := settingsconfig.FileSortWords[0]
	if i := int(v.Sort); i >= 0 && i < len(settingsconfig.FileSortWords) {
		sort = settingsconfig.FileSortWords[i]
	}
	return &settingsconfig.FileDialogView{Hidden: v.ShowHidden, Details: v.Details, Sort: sort, Descending: v.Descending}
}

// openDialogConfig is what the two Open browsers have in common: the folder kept for it, the
// recent files that are still there, and the way the person last looked at a folder.
func (b *appMenuBar) openDialogConfig(title, kept string) zenui.DialogConfig {
	return zenui.DialogConfig{
		Mode:     zenui.ModeOpen,
		Title:    title,
		StartDir: openStartDir(kept),
		Recent:   existingFiles(b.recentFiles),
		View:     viewFromSettings(b.fileView),
		FS:       zenui.OSFS{},
	}
}

// noteRecentFile puts the file at path first in the recent ones, when it is of a kind that is opened,
// and keeps the list in the settings.
func (b *appMenuBar) noteRecentFile(zx *ZenZX, path string) {
	list := addRecentFile(b.recentFiles, path)
	if reflect.DeepEqual(list, b.recentFiles) {
		return
	}
	b.recentFiles = list
	b.saveSettings(zx)
}

// endFileDialog is what is done when a browser closes, whether it was accepted or not: how the
// person looked at the folder is kept for the next one. It says which browser it was.
func (b *appMenuBar) endFileDialog(zx *ZenZX, d *zenui.Dialog) fileDialogKind {
	which := b.fileDlg
	b.fileDlg = fileDlgNone
	if which == fileDlgNone || d == nil {
		return which
	}
	if v := settingsFromView(d.View()); !reflect.DeepEqual(v, b.fileView) {
		b.fileView = v
		b.saveSettings(zx)
	}
	return which
}

// keepFolder remembers the folder of the file chosen in a browser: it is where that browser starts
// next time.
func (b *appMenuBar) keepFolder(zx *ZenZX, which fileDialogKind, path string) {
	dir := filepath.Dir(path)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	kept := &b.openFolder
	if which == fileDlgDisk {
		kept = &b.diskFolder
	}
	if *kept == dir {
		return
	}
	*kept = dir
	b.saveSettings(zx)
}
