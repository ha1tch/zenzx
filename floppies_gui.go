//go:build !headless

package zenzx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// ============================================================================
// The Floppy Disks window
//
// The +3's drive is not like the Next's card slots in one respect that
// decides what this window is for: a disk is read whole into memory when it
// goes in the drive, the machine works on that copy, and the file gets the
// changes later (30 seconds after the last write, on Eject, on Save, at
// exit). So at any moment there can be two versions of a disk, the one in
// memory and the one in the file, and either can be ahead of the other: the
// memory when the machine has written, the file when something else has
// rewritten it since. The window shows which, and has the three ways out:
// write the memory to the file, read the file again, or keep both by saving
// the memory under a new name.
//
// The rest is as the SD Cards window: the disk images of a folder, who has
// each, putting one in the drive, copying, making a blank one. A disk file
// is in the drive of one machine at a time, for the same reason a card is.
// ============================================================================

// diskEntry is one disk image the window lists.
type diskEntry struct {
	path string
	size int64
	kind string // "raw 180K", "DSK 40x1", "EDSK 40x1"; "" when it is not a disk
	why  string // why not
}

// maxDiskFile is the largest file the window will read to see whether it is
// a disk: a double-sided 80-track extended image is well under it.
const maxDiskFile = 4 << 20

// describeDisk reads what the window needs to know of a disk image.
func describeDisk(path string) diskEntry {
	e := diskEntry{path: path}
	st, err := os.Stat(path)
	if err != nil {
		e.why = err.Error()
		return e
	}
	e.size = st.Size()
	if e.size > maxDiskFile {
		e.why = "too large to be a +3 disk"
		return e
	}
	data, err := os.ReadFile(path)
	if err != nil {
		e.why = err.Error()
		return e
	}
	switch {
	case len(data) == DiskImageSize:
		e.kind = "raw 180K"
	case len(data) > 256 && (string(data[0:8]) == "EXTENDED" || string(data[0:8]) == "MV - CPC"):
		d, err := ParseDSK(data)
		if err != nil {
			e.why = err.Error()
			return e
		}
		name := "DSK"
		if d.Extended {
			name = "EDSK"
		}
		e.kind = fmt.Sprintf("%s %dx%d", name, d.NumTracks, d.NumSides)
	default:
		e.why = "not a DSK image nor a 180K sector dump"
	}
	return e
}

// listDisks returns the disk images in dir, and those of extra that are not
// in it, in the order of their names.
func listDisks(dir string, extra []string) []diskEntry {
	seen := map[string]bool{}
	var paths []string
	add := func(p string) {
		abs, err := filepath.Abs(p)
		if err != nil || seen[abs] {
			return
		}
		seen[abs] = true
		paths = append(paths, p)
	}
	var names []string
	for _, pat := range []string{"*.dsk", "*.DSK"} {
		if found, err := filepath.Glob(filepath.Join(dir, pat)); err == nil {
			names = append(names, found...)
		}
	}
	sort.Strings(names)
	for _, p := range names {
		add(p)
	}
	for _, p := range extra {
		if _, err := os.Stat(p); err == nil {
			add(p)
		}
	}
	disks := make([]diskEntry, 0, len(paths))
	for _, p := range paths {
		disks = append(disks, describeDisk(p))
	}
	return disks
}

// The window's buttons.
const (
	diskBtnInsert  = "insert"
	diskBtnEject   = "eject"
	diskBtnWrite   = "write"
	diskBtnReload  = "reload"
	diskBtnSaveNew = "save-new"
	diskBtnCopy    = "copy"
	diskBtnNew     = "new"
	diskBtnOther   = "other"
	diskBtnClose   = "close"

	disksWindowLines = 5 // rows of the list on show
)

// disksWindow is the Floppy Disks window, open for one machine.
type disksWindow struct {
	zx      *ZenZX
	machine int // its number in the dock
	dir     string
	// use tells which machine has a disk file in its drive, 0 for none;
	// holder returns that machine.
	use    func(path string) int
	holder func(machine int) *ZenZX

	disks   []diskEntry
	extra   []string
	list    *zenui.List
	buttons *zenui.ButtonBar
	browse  *zenui.Dialog
	note    string
	// confirm is the button that was pressed once and loses something if
	// pressed again; any other action forgets it.
	confirm string

	theme                    zenui.Theme
	panel, listArea, barArea zenui.Rect
	noteY                    int
	head                     []string
	scale, lineH             int
}

// newDisksWindow opens the window for machine number machine.
func newDisksWindow(zx *ZenZX, machine int, use func(string) int, holder func(int) *ZenZX) *disksWindow {
	w := &disksWindow{zx: zx, machine: machine, dir: ".", use: use, holder: holder,
		list: zenui.NewList(), buttons: zenui.NewButtonBar()}
	w.list.NoToolbar = true
	if f := w.fdc().DiskFile(); f != "" {
		w.dir = filepath.Dir(f)
	}
	w.refresh()
	if f := w.fdc().DiskFile(); f != "" {
		w.selectPath(f)
	}
	return w
}

// fdc is the machine's controller; the window is opened only for a machine
// that has one.
func (w *disksWindow) fdc() *FDC765 { return w.zx.io.fdc }

// refresh reads the folder again and keeps the selection on the same disk.
func (w *disksWindow) refresh() {
	keep := ""
	if w.list.Selected >= 0 && w.list.Selected < len(w.disks) {
		keep = w.disks[w.list.Selected].path
	}
	extra := w.extra
	if f := w.fdc().DiskFile(); f != "" {
		extra = append(append([]string{}, extra...), f)
	}
	w.disks = listDisks(w.dir, extra)
	w.list.Selected = -1
	for i, d := range w.disks {
		if d.path == keep {
			w.list.Selected = i
		}
	}
	if w.list.Selected < 0 && len(w.disks) > 0 {
		w.list.Selected = 0
	}
}

// selectPath puts the selection on the disk at path.
func (w *disksWindow) selectPath(path string) {
	for i, d := range w.disks {
		if same(d.path, path) {
			w.list.Selected = i
		}
	}
}

// selected returns the chosen disk, if there is one.
func (w *disksWindow) selected() (diskEntry, bool) {
	if w.list.Selected < 0 || w.list.Selected >= len(w.disks) {
		return diskEntry{}, false
	}
	return w.disks[w.list.Selected], true
}

// where says who has a disk and whether its memory is ahead of its file.
func (w *disksWindow) where(path string) string {
	n := w.use(path)
	if n == 0 {
		return "free"
	}
	s := fmt.Sprintf("machine %d", n)
	if n == w.machine {
		s = "in the drive"
	}
	if h := w.holder(n); h != nil && h.io.fdc != nil && h.io.fdc.IsModified() {
		s += ", unsaved"
	}
	return s
}

// row is a disk's line in the list.
func (w *disksWindow) row(d diskEntry) string {
	kind := d.kind
	if kind == "" {
		kind = "no disk"
	}
	name := filepath.Base(d.path)
	if len(name) > 26 {
		name = name[:25] + "~"
	}
	return fmt.Sprintf("%-26s %4d KB  %-9s  %s", name, (d.size+1023)>>10, kind, w.where(d.path))
}

// headLines is the text above the list: the drive, and how far apart the
// disk in memory and its file are.
func (w *disksWindow) headLines() []string {
	fdc := w.fdc()
	drive := "empty"
	switch {
	case fdc.HasDisk() && fdc.DiskFile() == "":
		drive = "a blank disk with no file"
		if fdc.IsModified() {
			drive += ", written to: Save as new file keeps it"
		}
	case fdc.HasDisk():
		drive = filepath.Base(fdc.DiskFile())
		switch {
		case fdc.IsModified() && fdc.FileChanged():
			drive += ": changed here AND on disk since it was read"
		case fdc.IsModified():
			drive += ": changes here not yet in the file"
		case fdc.FileChanged():
			drive += ": the file has changed on disk since it was read"
		default:
			drive += ": the same as its file"
		}
	}
	return []string{
		fmt.Sprintf("## Machine %d", w.machine),
		"Drive A: " + drive,
		"A disk is read whole into memory when it goes in the drive. Changes",
		"reach its file 30 seconds after the last write, on Eject and at exit.",
	}
}

// setButtons makes the row of buttons for the present state. What a button
// may do is decided here: do acts only on enabled buttons.
func (w *disksWindow) setButtons() {
	fdc := w.fdc()
	d, ok := w.selected()
	free := ok && w.use(d.path) == 0
	label := func(id, text string) string {
		if w.confirm == id {
			return text + ": sure?"
		}
		return text
	}
	w.buttons.Buttons = []zenui.BarButton{
		{ID: diskBtnInsert, Label: "Put in drive", Disabled: !(free && d.kind != "")},
		{ID: diskBtnEject, Label: label(diskBtnEject, "Eject"), Disabled: !fdc.HasDisk()},
		{ID: diskBtnWrite, Label: label(diskBtnWrite, "Write to file now"), Disabled: !(fdc.IsModified() && fdc.DiskFile() != "")},
		{ID: diskBtnReload, Label: label(diskBtnReload, "Read file again"), Disabled: fdc.DiskFile() == ""},
		{ID: diskBtnSaveNew, Label: "Save as new file", Disabled: !fdc.HasDisk()},
		{ID: diskBtnCopy, Label: "Copy", Disabled: !ok},
		{ID: diskBtnNew, Label: "New blank disk"},
		{ID: diskBtnOther, Label: "Other file..."},
		{ID: diskBtnClose, Label: "Close"},
	}
}

// sure reports that button id may go ahead with something that loses data:
// the first press only asks, with why; the second goes ahead.
func (w *disksWindow) sure(id, why string) bool {
	if w.confirm == id {
		w.confirm = ""
		return true
	}
	w.confirm = id
	w.note = why
	return false
}

// do carries out a button.
func (w *disksWindow) do(id string) bool {
	for _, b := range w.buttons.Buttons {
		if b.ID == id && b.Disabled {
			return false
		}
	}
	if w.confirm != id {
		w.confirm = ""
	}
	fdc := w.fdc()
	d, _ := w.selected()
	switch id {
	case diskBtnClose:
		return true

	case diskBtnInsert:
		if fdc.HasDisk() && fdc.IsModified() {
			if fdc.DiskFile() == "" {
				w.note = "The blank disk in the drive has been written to and has no file. Save as new file first, or Eject it."
				return false
			}
			if fdc.Busy() {
				w.note = "The drive is in the middle of a command. Try again in a moment."
				return false
			}
			if err := fdc.SaveDisk(); err != nil {
				w.note = "The disk in the drive could not be written to its file: " + err.Error()
				return false
			}
		}
		if err := w.zx.io.LoadDisk(d.path); err != nil {
			w.note = "That disk could not be read: " + err.Error()
		} else {
			w.note = filepath.Base(d.path) + " is in the drive."
		}

	case diskBtnEject:
		if fdc.IsModified() && fdc.DiskFile() == "" &&
			!w.sure(id, "The blank disk has been written to and has no file: ejecting it loses that. Save as new file keeps it.") {
			return false
		}
		if fdc.IsModified() && fdc.FileChanged() &&
			!w.sure(id, "Ejecting writes the disk to its file, which has changed on disk since it was read: that change would be lost.") {
			return false
		}
		fdc.EjectDisk()
		w.note = "The drive is empty."

	case diskBtnWrite:
		if fdc.Busy() {
			w.note = "The drive is in the middle of a command. Try again in a moment."
			return false
		}
		if fdc.FileChanged() &&
			!w.sure(id, "The file has changed on disk since it was read. Writing now replaces it with the disk in memory.") {
			return false
		}
		if err := fdc.SaveDisk(); err != nil {
			w.note = "The disk could not be written: " + err.Error()
		} else {
			w.note = "The file now has the disk as it is in memory."
		}

	case diskBtnReload:
		if fdc.IsModified() &&
			!w.sure(id, "The disk in memory has changes that are not in the file. Reading the file again loses them.") {
			return false
		}
		file := fdc.DiskFile()
		if err := w.zx.io.LoadDisk(file); err != nil {
			w.note = "The file could not be read: " + err.Error()
		} else {
			w.note = "The drive has the disk as it is in the file."
		}

	case diskBtnSaveNew:
		if fdc.Busy() {
			w.note = "The drive is in the middle of a command. Try again in a moment."
			return false
		}
		stem := "disk"
		if f := fdc.DiskFile(); f != "" {
			stem = copyStem(f)
		}
		path := freeImageName(w.dir, stem, ".dsk")
		if err := w.zx.io.SaveDiskAs(path); err != nil {
			w.note = "The disk could not be saved: " + err.Error()
		} else {
			w.note = fmt.Sprintf("The disk in the drive is now %s; the file it came from is as it was.", filepath.Base(path))
			w.refresh()
			w.selectPath(path)
		}

	case diskBtnCopy:
		// The copy is of the file, so a machine that has the disk writes
		// its memory to the file first.
		if h := w.holder(w.use(d.path)); h != nil && h.io.fdc != nil && h.io.fdc.IsModified() {
			if h.io.fdc.Busy() {
				w.note = "The machine that has that disk is in the middle of writing to it. Try again in a moment."
				return false
			}
			if err := h.io.fdc.SaveDisk(); err != nil {
				w.note = "The disk could not be written to its file before copying: " + err.Error()
				return false
			}
		}
		dst := freeImageName(w.dir, copyStem(d.path), filepath.Ext(d.path))
		if err := copyCard(context.Background(), d.path, dst, func(int64, int64) {}); err != nil {
			w.note = "The copy failed: " + err.Error()
		} else {
			w.note = filepath.Base(dst) + " is a copy, free to use."
			w.refresh()
			w.selectPath(dst)
		}

	case diskBtnNew:
		path := freeImageName(w.dir, "disk", ".dsk")
		blank := make([]byte, DiskImageSize)
		for i := range blank {
			blank[i] = 0xE5
		}
		if err := os.WriteFile(path, blank, 0o644); err != nil {
			w.note = "The disk could not be made: " + err.Error()
		} else {
			w.note = fmt.Sprintf("Made %s, blank. +3DOS formats it: FORMAT \"a:\".", filepath.Base(path))
			w.refresh()
			w.selectPath(path)
		}

	case diskBtnOther:
		w.browse = zenui.NewDialog(zenui.DialogConfig{
			Mode: zenui.ModeOpen, Title: "Another Disk Image", StartDir: w.dir,
			Filters: []string{"dsk"}, FS: zenui.OSFS{},
		})
	}
	return false
}

// layout places the window's parts.
func (w *disksWindow) layout(r zenui.Renderer, screenW, screenH int) {
	w.setButtons()
	w.head = w.headLines()
	w.scale = zenui.TextBodyScale(r, screenW)
	w.lineH = r.LineHeight(w.scale)
	if w.lineH < 1 {
		w.lineH = 1
	}
	pad := r.LineHeight(1)
	pw := zenui.TextPanelWidth(r, w.scale)
	if pw > screenW-2*pad {
		pw = screenW - 2*pad
	}
	inner := pw - 2*pad
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	_, barH := w.buttons.Layout(zenui.Rect{W: inner}, w.lineH, textW)
	titleH := r.LineHeight(2) + 8
	// A row is as tall as the list makes it, which is more than a line: it
	// has the picture beside the name.
	rowsH := w.lineH + 4
	if len(w.list.Items) > 0 {
		rowsH = w.list.ItemRect(0, zenui.Rect{W: inner, H: 1 << 14}, w.lineH).H
	}
	listH := disksWindowLines*rowsH + 2
	noteH := 2 * w.lineH
	fixed := 2*pad + titleH + len(w.head)*w.lineH + pad + pad + noteH + pad + barH
	if over := fixed + listH - (screenH - 2*pad); over > 0 {
		listH -= over
		if listH < 2*(w.lineH+4) {
			listH = 2 * (w.lineH + 4)
		}
	}
	ph := fixed + listH
	px, py := (screenW-pw)/2, (screenH-ph)/2
	if py < 0 {
		py = 0
	}
	w.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}
	y := py + pad + titleH + len(w.head)*w.lineH + pad
	w.listArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: listH}
	y += listH + pad
	w.noteY = y
	y += noteH + pad
	w.barArea = zenui.Rect{X: px + pad, Y: y, W: inner, H: barH}
}

// Update runs the window for a frame; it returns false when it has closed.
func (w *disksWindow) Update(in zenui.Input, r zenui.Renderer, screenW, screenH int) bool {
	if w.browse != nil {
		switch w.browse.Update(in) {
		case zenui.Accepted:
			path := w.browse.Result()
			w.browse = nil
			w.extra = append(w.extra, path)
			w.refresh()
			w.selectPath(path)
		case zenui.Cancelled:
			w.browse = nil
		}
		return true
	}
	w.layout(r, screenW, screenH)
	w.list.Items = w.list.Items[:0]
	for _, d := range w.disks {
		w.list.Items = append(w.list.Items, zenui.ListItem{Name: w.row(d), Thumb: w.diskPicture(d.kind != "")})
	}
	before := w.list.Selected
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape:
			return false
		case zenui.KeyEnter:
			if w.do(diskBtnInsert) {
				return false
			}
		case zenui.KeyUp, zenui.KeyDown, zenui.KeyHome, zenui.KeyEnd, zenui.KeyPageUp, zenui.KeyPageDown:
			w.list.Step(k)
			w.list.Reveal(w.listArea, w.lineH)
		}
	}
	w.list.Update(in, w.listArea, w.lineH)
	if w.list.Selected != before {
		w.confirm = ""
	}
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	if id, ok := w.buttons.Update(in, w.barArea, w.lineH, textW); ok {
		return !w.do(id)
	}
	return true
}

// diskPicture draws a small disk beside a row: filled for a disk image, in
// outline for a file that is not one.
func (w *disksWindow) diskPicture(isDisk bool) func(r zenui.Renderer, area zenui.Rect) {
	return func(r zenui.Renderer, area zenui.Rect) {
		side := area.H - 4
		if side < 6 {
			return
		}
		c := zenui.Rect{X: area.X + (area.W-side)/2, Y: area.Y + 2, W: side, H: side}
		shutter := zenui.Rect{X: c.X + side/4, Y: c.Y, W: side / 2, H: side / 3}
		if !isDisk {
			r.StrokeRect(c, w.theme.DimText, 1)
			return
		}
		r.FillRect(c, w.theme.Text)
		r.FillRect(shutter, w.theme.Panel)
	}
}

// Draw draws the window over the machine's picture.
func (w *disksWindow) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	if w.browse != nil {
		w.browse.Draw(r, screenW, screenH, theme)
		return
	}
	w.theme = theme
	w.layout(r, screenW, screenH)
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(w.panel, theme.Panel)
	r.StrokeRect(w.panel, theme.Border, theme.BorderThickness)

	pad := r.LineHeight(1)
	x, y := w.panel.X+pad, w.panel.Y+pad
	r.DrawText("FLOPPY DISKS", x, y, 2, theme.Text)
	y += r.LineHeight(2) + 8
	y = drawDialogHead(r, w.head, x, y, w.panel.W-2*pad, w.scale, w.lineH, theme)
	r.StrokeRect(zenui.Rect{X: w.listArea.X - 1, Y: w.listArea.Y - 1, W: w.listArea.W + 2, H: w.listArea.H + 2}, theme.Border, 1)
	if len(w.disks) == 0 {
		drawWrapped(r, "No disk images in "+w.dir, w.listArea.X+4, w.listArea.Y+4, w.listArea.W-8, w.scale, w.lineH, 3, theme.DimText)
	}
	w.list.Draw(r, theme, w.listArea, w.scale)

	note := w.note
	if d, ok := w.selected(); ok && note == "" && d.kind == "" {
		note = "It is not a disk: " + d.why
	}
	drawWrapped(r, note, w.listArea.X, w.noteY, w.listArea.W, w.scale, w.lineH, 2, theme.DimText)
	w.buttons.Draw(r, theme, w.barArea, w.scale)
}

// diskUser tells which of the window's machines has a disk file in its
// drive, 0 for none; with no dock there is the one machine.
func (b *appMenuBar) diskUser(zx *ZenZX) func(path string) int {
	return func(path string) int {
		if b.diskUse != nil {
			return b.diskUse(path)
		}
		if zx.io.fdc != nil && zx.io.fdc.DiskFile() != "" && same(zx.io.fdc.DiskFile(), path) {
			return 1
		}
		return 0
	}
}

// openDisks opens the Floppy Disks window for a machine with a drive.
func (b *appMenuBar) openDisks(zx *ZenZX) {
	if !zx.io.hasFDC || zx.io.fdc == nil {
		zx.notice("This model has no disk drive: switch to a +3 first, from the Machine menu")
		return
	}
	holder := func(n int) *ZenZX {
		switch {
		case n == 0:
			return nil
		case b.machineAt != nil:
			return b.machineAt(n)
		}
		return zx
	}
	b.disks = newDisksWindow(zx, b.numberOf(zx), b.diskUser(zx), holder)
}

// updateDisks runs the Floppy Disks window.
func (b *appMenuBar) updateDisks() {
	if !b.disks.Update(zenuiraylib.Input(), b.renderer, rl.GetScreenWidth(), rl.GetScreenHeight()) {
		b.disks = nil
	}
}

// diskTakenElsewhere reports, with the machine's number, that a disk file is
// in the drive of a machine other than zx.
func (b *appMenuBar) diskTakenElsewhere(zx *ZenZX, path string) (int, bool) {
	n := b.diskUser(zx)(path)
	return n, n != 0 && n != b.numberOf(zx)
}
