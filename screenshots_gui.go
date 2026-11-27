//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// ============================================================================
// The Screenshots window (View > Screenshot > Manage Screenshots...)
//
// It lists what is in the screenshots folder (screenshotdir.go) the way the Snapshots window
// lists snapshots, with the questions that the Snapshots, SD Cards and Floppy Disks windows ask
// (sdcards_manage_gui.go): a name is asked for and then confirmed, and a delete asks for the word
// DELETE. What is new is the picture of each file: the list shows a thumbnail, as a grid of three
// across or as rows, and Preview shows the chosen screenshot as large as the window allows, a GIF
// playing as it was recorded.
//
// The thumbnails are textures that the list's Thumb callbacks draw. A thumbnail is read and made
// when its item first comes on show, a couple a frame so that a folder of many does not stop the
// window, and the ones that have not been on show for a while are given back.
// ============================================================================

// screenshotsResult is what a frame of the window ended in.
type screenshotsResult int

const (
	screenshotsOpen   screenshotsResult = iota // still on screen
	screenshotsClosed                          // closed
)

// The window's buttons.
const (
	shotsBtnPreview = "preview"
	shotsBtnRename  = "rename"
	shotsBtnDelete  = "delete"
	shotsBtnLayout  = "layout"
	shotsBtnClose   = "close"

	screenshotsRowLines  = 10  // rows of the list on show, as rows
	screenshotsGridRows  = 2   // and as the grid
	thumbLoadsPerFrame   = 2   // thumbnails made in one frame
	thumbCacheMax        = 48  // thumbnails kept once they are off show
	doubleClickSeconds   = 0.4 // two clicks on one item this close together are a Preview
	previewMarginPercent = 4   // the room round a preview, of the window's smaller side
)

// shotThumb is a screenshot's thumbnail: its texture, or why it has none.
type shotThumb struct {
	tex  rl.Texture2D
	w, h int
	ok   bool   // false: the file could not be read as a picture
	why  string // and why
	used int    // the frame it was last drawn in
	made bool   // tex is a texture
}

// shotPreview is a screenshot shown large.
type shotPreview struct {
	file   screenshotFile
	pic    *shotPicture
	err    error
	tex    rl.Texture2D
	made   bool
	frame  int     // the frame of a GIF that is in the texture
	clock  float32 // seconds since the preview came up, which says which frame of a GIF is due
	filter int     // how the texture is filtered: 1 as pixels, 2 smoothed, 0 not yet set
	pix    []color.RGBA
}

// screenshotsWindow is the Screenshots window.
type screenshotsWindow struct {
	dir  string
	all  []screenshotFile
	list *zenui.List
	// buttons is the row of buttons, ask the question being put (Rename or Delete) and note what
	// the last thing done came to.
	buttons *zenui.ButtonBar
	ask     *cardQuestion
	note    string

	thumbs map[string]*shotThumb // by the file's path, size and date: a file that changed is read again
	view   *shotPreview
	frame  int // frames the window has run
	budget int // thumbnails that may still be made this frame
	clock  float32
	click  struct {
		item int
		at   float32
	}
	noGL bool // no window to make textures in: a test

	panel, listArea, barArea zenui.Rect
	head                     []string
	scale, lineH             int
}

// newScreenshotsWindow opens the window on the folder dir, as the grid.
func newScreenshotsWindow(dir string) *screenshotsWindow {
	w := &screenshotsWindow{dir: dir, list: zenui.NewList(), buttons: zenui.NewButtonBar(), thumbs: map[string]*shotThumb{}}
	w.list.NoToolbar = true
	w.list.Columns = 3
	w.click.item = -1
	w.refresh()
	return w
}

// refresh reads the folder again and keeps the selection on the same screenshot. Thumbnails of
// files that are gone, or have changed, are given back.
func (w *screenshotsWindow) refresh() {
	keep := ""
	if w.list.Selected >= 0 && w.list.Selected < len(w.all) {
		keep = w.all[w.list.Selected].Path
	}
	w.all = listScreenshots(w.dir)
	w.list.Selected = -1
	live := map[string]bool{}
	for i, f := range w.all {
		live[thumbKey(f)] = true
		if f.Path == keep {
			w.list.Selected = i
		}
	}
	if w.list.Selected < 0 && len(w.all) > 0 {
		w.list.Selected = 0
	}
	for k, t := range w.thumbs {
		if !live[k] {
			w.dropThumb(k, t)
		}
	}
}

// thumbKey is what a thumbnail is kept under.
func thumbKey(f screenshotFile) string {
	return fmt.Sprintf("%s|%d|%d", f.Path, f.Size, f.When.UnixNano())
}

func (w *screenshotsWindow) dropThumb(k string, t *shotThumb) {
	if t.made && !w.noGL {
		rl.UnloadTexture(t.tex)
	}
	delete(w.thumbs, k)
}

// close gives back every texture the window made. The bar calls it when the window ends.
func (w *screenshotsWindow) close() {
	for k, t := range w.thumbs {
		w.dropThumb(k, t)
	}
	w.closePreview()
}

// selectPath puts the selection on the screenshot at path.
func (w *screenshotsWindow) selectPath(path string) {
	for i, f := range w.all {
		if f.Path == path {
			w.list.Selected = i
		}
	}
}

// selected is the chosen screenshot, if there is one.
func (w *screenshotsWindow) selected() (screenshotFile, bool) {
	if w.list.Selected < 0 || w.list.Selected >= len(w.all) {
		return screenshotFile{}, false
	}
	return w.all[w.list.Selected], true
}

// screenshotRow is a screenshot's line in the list as rows: when it was made, whose it is, what it
// is and how big. Only ASCII, which is what the typeface has.
func screenshotRow(f screenshotFile) string {
	when, who, kind, size := screenshotCells(f)
	return when + "  " + who + "  " + kind + "  " + size
}

// screenshotCells are the columns of a screenshot's row: when, whose (the machine's name in the
// file's, or the name a renamed file was given, without its extension), the format, and the size.
// The format is a column of its own, so that a name that is cut short never takes it with it.
func screenshotCells(f screenshotFile) (when, who, kind, size string) {
	who = f.Machine
	if who == "" {
		who = shotStem(f.Name)
	}
	return f.When.Format("2006-01-02 15:04:05"), who, strings.ToUpper(f.Ext), humanBytes(f.Size)
}

// screenshotCaption is a screenshot's name under its thumbnail in the grid.
func screenshotCaption(f screenshotFile) string {
	who := f.Machine
	if who == "" {
		who = shotStem(f.Name)
	}
	return who + " " + strings.ToUpper(f.Ext)
}

// screenshotKind says what a screenshot's format is.
func screenshotKind(f screenshotFile) string {
	switch f.Ext {
	case "png":
		return "A PNG picture."
	case "gif":
		return "An animated GIF: Preview plays it."
	case "scr":
		switch f.Size {
		case 6912:
			return "A ZX Spectrum screen file: the display memory, a bitmap and its attributes."
		case 12288:
			return "A Timex screen in a plain .scr file. It does not say whether it is hi-res or hi-colour, so Preview guesses."
		case 12289:
			return "A Timex hi-res screen in a plain .scr file: the last byte holds its ink."
		}
		return "A screen file of a size that is not a Spectrum's."
	case hiResExt:
		return "A Timex hi-res screen: 512x192 pixels in two colours, with its ink colour."
	case hiColExt:
		return "A Timex hi-colour screen: a bitmap, and an attribute for each of its bytes."
	case "nxi":
		return "A ZX Spectrum Next bitmap: a palette and the pixels."
	}
	return ""
}

// detail is the note for the chosen screenshot: how big and how old it is, and what it is.
func (w *screenshotsWindow) detail(f screenshotFile) string {
	dims := ""
	if t := w.thumbs[thumbKey(f)]; t != nil {
		switch {
		case t.ok:
			dims = fmt.Sprintf("%dx%d, ", t.w, t.h)
		default:
			return "This file cannot be shown as a picture: " + t.why
		}
	}
	return fmt.Sprintf("%s%s, %s. %s", dims, humanBytes(f.Size), f.When.Format("2006-01-02 15:04:05"), screenshotKind(f))
}

// setButtons makes the row of buttons: what a button may do is decided here, and do acts only on
// enabled ones.
func (w *screenshotsWindow) setButtons() {
	_, ok := w.selected()
	layout := "Rows"
	if w.list.Columns == 1 {
		layout = "Grid"
	}
	w.buttons.Buttons = []zenui.BarButton{
		{ID: shotsBtnPreview, Label: "Preview", Disabled: !ok},
		{ID: shotsBtnRename, Label: "Rename", Disabled: !ok},
		{ID: shotsBtnDelete, Label: "Delete", Disabled: !ok},
		{ID: shotsBtnLayout, Label: layout},
		{ID: shotsBtnClose, Label: "Close"},
	}
}

// do carries out a button.
func (w *screenshotsWindow) do(id string) screenshotsResult {
	for _, b := range w.buttons.Buttons {
		if b.ID == id && b.Disabled {
			return screenshotsOpen
		}
	}
	f, _ := w.selected()
	switch id {
	case shotsBtnClose:
		return screenshotsClosed
	case shotsBtnPreview:
		w.openPreview(w.list.Selected)
	case shotsBtnLayout:
		if w.list.Columns == 1 {
			w.list.Columns = 3
		} else {
			w.list.Columns = 1
		}
	case shotsBtnRename:
		stem := shotStem(f.Name)
		w.ask = &cardQuestion{kind: askRenameName, path: f.Path, size: f.Size, text: stem,
			noun: "screenshot", ext: shotSuffix(f.Name)}
		w.ask.build()
	case shotsBtnDelete:
		w.ask = &cardQuestion{kind: askDelete, path: f.Path, size: f.Size,
			noun: "screenshot", sizeText: humanBytes(f.Size)}
		w.ask.build()
	}
	return screenshotsOpen
}

// screenshotRenameTarget is the path a screenshot at old gets when the person types typed as its
// new name, or why it cannot have it. The extension stays what it was: a name typed with another
// screenshot format's is refused, since the extension is what says how the file is read.
func screenshotRenameTarget(old, typed string) (target, why string) {
	ext := shotSuffix(old)
	name := strings.TrimSpace(typed)
	// The name's own extension, whole: ".hires.scr" is not ".scr".
	if typedExt := shotExtOf(name); isScreenshotExt(typedExt) && typedExt != shotExtOf(old) {
		return "", "A screenshot stays " + ext + ": its format does not change with its name."
	}
	return renameTargetExt(old, name, ext)
}

// updateAsk runs the question for a frame. Enter answers it as the right-hand button does,
// Escape and the left-hand button drop it.
func (w *screenshotsWindow) updateAsk(in zenui.Input) {
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
func (w *screenshotsWindow) answer() {
	q := w.ask
	switch q.kind {
	case askRenameName:
		target, why := screenshotRenameTarget(q.path, q.text)
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

// rename renames the screenshot at path to target, and keeps the selection on it.
func (w *screenshotsWindow) rename(path, target string) {
	if err := renameFile(path, target); err != nil {
		w.note = "Not renamed: " + plainError(err)
		return
	}
	w.refresh()
	w.selectPath(target)
	w.note = fmt.Sprintf("%s is now called %s.", filepath.Base(path), filepath.Base(target))
	sayf("Screenshot renamed: %s to %s\n", path, target)
}

// remove deletes the screenshot at path, and puts the selection on the one that took its place.
func (w *screenshotsWindow) remove(path string) {
	at := w.list.Selected
	if err := os.Remove(path); err != nil {
		w.note = "Not deleted: " + plainError(err)
		return
	}
	w.refresh()
	w.list.Selected = min(max(at, 0), len(w.all)-1)
	w.note = fmt.Sprintf("%s is deleted.", filepath.Base(path))
	sayf("Screenshot deleted: %s\n", path)
}

// --- the pictures ----------------------------------------------------------------------------

// pixelsOf is img's pixels as raylib takes them, in buf when it is big enough.
func pixelsOf(img *image.RGBA, buf []color.RGBA) []color.RGBA {
	n := len(img.Pix) / 4
	if cap(buf) < n {
		buf = make([]color.RGBA, n)
	}
	buf = buf[:n]
	for i := range buf {
		p := img.Pix[i*4 : i*4+4 : i*4+4]
		buf[i] = color.RGBA{R: p[0], G: p[1], B: p[2], A: 0xFF}
	}
	return buf
}

// uploadTexture makes a texture of a picture. (NewImageFromImage sets every pixel through a call
// of its own, which is slow for a picture this size: the texture is made empty and filled at once.)
func uploadTexture(img *image.RGBA, buf []color.RGBA) rl.Texture2D {
	b := img.Bounds()
	blank := rl.GenImageColor(b.Dx(), b.Dy(), rl.Black)
	tex := rl.LoadTextureFromImage(blank)
	rl.UnloadImage(blank)
	rl.UpdateTexture(tex, pixelsOf(img, buf))
	return tex
}

// thumbFor is the thumbnail of f, made now if it is not there and the frame has not made its
// share, and nil when it has to wait for a later frame.
func (w *screenshotsWindow) thumbFor(f screenshotFile) *shotThumb {
	k := thumbKey(f)
	if t := w.thumbs[k]; t != nil {
		return t
	}
	if w.budget <= 0 {
		return nil
	}
	w.budget--
	t := &shotThumb{}
	pic, err := loadShotPicture(f.Path, true)
	switch {
	case err != nil:
		t.why = plainError(err)
	case len(pic.Frames) == 0:
		t.why = "it has no picture in it."
	default:
		t.ok = true
		t.w, t.h = pic.Size()
		if !w.noGL {
			t.tex = uploadTexture(pic.Frames[0], nil)
			rl.SetTextureFilter(t.tex, rl.FilterBilinear)
			t.made = true
		}
	}
	w.thumbs[k] = t
	return t
}

// fitRect is the largest rectangle of the picture's shape that fits in area, in its middle.
func fitRect(area zenui.Rect, pw, ph int) (x, y, w, h float32) {
	if pw <= 0 || ph <= 0 || area.W <= 0 || area.H <= 0 {
		return float32(area.X), float32(area.Y), 0, 0
	}
	s := min(float32(area.W)/float32(pw), float32(area.H)/float32(ph))
	w, h = float32(pw)*s, float32(ph)*s
	return float32(area.X) + (float32(area.W)-w)/2, float32(area.Y) + (float32(area.H)-h)/2, w, h
}

// drawThumb draws f's thumbnail in area: the list calls it for the items on show.
func (w *screenshotsWindow) drawThumb(r zenui.Renderer, f screenshotFile, area zenui.Rect) {
	r.FillRect(area, zenui.Colour{A: 0xFF})
	t := w.thumbFor(f)
	say := func(s string) {
		r.DrawText(s, area.X+(area.W-r.TextWidth(s, w.scale))/2, area.Y+(area.H-r.LineHeight(w.scale))/2, w.scale, zenui.Colour{R: 0x99, G: 0x99, B: 0x99, A: 0xFF})
	}
	switch {
	case t == nil:
		say("...")
	case !t.ok:
		say("?")
	case t.made:
		t.used = w.frame
		x, y, dw, dh := fitRect(area, t.w, t.h)
		rl.DrawTexturePro(t.tex, rl.Rectangle{Width: float32(t.w), Height: float32(t.h)},
			rl.Rectangle{X: x, Y: y, Width: dw, Height: dh}, rl.Vector2{}, 0, rl.White)
	}
}

// evict gives back the thumbnails that were not on show this frame, once there are many.
func (w *screenshotsWindow) evict() {
	if len(w.thumbs) <= thumbCacheMax {
		return
	}
	for k, t := range w.thumbs {
		if t.used != w.frame {
			w.dropThumb(k, t)
		}
	}
}

// --- the preview -----------------------------------------------------------------------------

// openPreview shows the screenshot at index i of the list large.
func (w *screenshotsWindow) openPreview(i int) {
	if i < 0 || i >= len(w.all) {
		return
	}
	w.closePreview()
	f := w.all[i]
	v := &shotPreview{file: f}
	v.pic, v.err = loadShotPicture(f.Path, false)
	if v.err == nil && len(v.pic.Frames) == 0 {
		v.err = fmt.Errorf("it has no picture in it")
	}
	if v.err == nil && !w.noGL {
		v.tex = uploadTexture(v.pic.Frames[0], nil)
		v.made = true
	}
	w.view = v
	w.list.Selected = i
}

// closePreview takes the preview off the screen.
func (w *screenshotsWindow) closePreview() {
	if v := w.view; v != nil && v.made && !w.noGL {
		rl.UnloadTexture(v.tex)
	}
	w.view = nil
}

// stepPreview shows the screenshot before (d = -1) or after (d = 1) the one on show.
func (w *screenshotsWindow) stepPreview(d int) {
	i := w.list.Selected + d
	if i < 0 || i >= len(w.all) {
		return
	}
	w.openPreview(i)
}

// updateView runs the preview for a frame: Escape and Enter close it, a click does, the arrow
// keys move to the screenshots next to it, and a GIF plays.
func (w *screenshotsWindow) updateView(in zenui.Input) {
	v := w.view
	v.clock += in.DeltaTime
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape, zenui.KeyEnter:
			w.closePreview()
			return
		case zenui.KeyLeft, zenui.KeyUp:
			w.stepPreview(-1)
			return
		case zenui.KeyRight, zenui.KeyDown:
			w.stepPreview(1)
			return
		}
	}
	if in.MousePressed {
		w.closePreview()
		return
	}
	if v.err != nil || v.pic == nil || !v.pic.Animated() {
		return
	}
	if f := v.pic.FrameAt(int(v.clock * 100)); f != v.frame {
		v.frame = f
		if v.made && !w.noGL {
			v.pix = pixelsOf(v.pic.Frames[f], v.pix)
			rl.UpdateTexture(v.tex, v.pix)
		}
	}
}

// previewLines are the lines under the picture of a preview.
func (v *shotPreview) lines() []string {
	f := v.file
	info := fmt.Sprintf("%s, %s", humanBytes(f.Size), f.When.Format("2006-01-02 15:04:05"))
	if v.pic != nil {
		pw, ph := v.pic.Size()
		info = fmt.Sprintf("%dx%d, %s", pw, ph, info)
		if v.pic.Animated() {
			total := 0
			for _, d := range v.pic.Delays {
				total += d
			}
			info += fmt.Sprintf(", %d frames, %.1f seconds", len(v.pic.Frames), float64(total)/100)
		}
	}
	return []string{f.Name, info, previewHint}
}

// previewHint is the last line under a preview.
const previewHint = "Left and Right show the screenshots beside it. Escape or a click closes it."

// wrapToWidth breaks s into lines that are no wider than width pixels at scale, at its spaces; a
// word wider than that is cut where it has to be.
func wrapToWidth(r zenui.Renderer, s string, width, scale int) []string {
	if width <= 0 || r.TextWidth(s, scale) <= width {
		return []string{s}
	}
	var lines []string
	line := ""
	flush := func() {
		if line != "" {
			lines = append(lines, line)
			line = ""
		}
	}
	for _, word := range strings.Fields(s) {
		for r.TextWidth(word, scale) > width { // a word that does not fit on a line of its own
			flush()
			rs := []rune(word)
			n := len(rs)
			for n > 1 && r.TextWidth(string(rs[:n]), scale) > width {
				n--
			}
			lines = append(lines, string(rs[:n]))
			word = string(rs[n:])
		}
		switch {
		case word == "":
		case line == "":
			line = word
		case r.TextWidth(line+" "+word, scale) <= width:
			line += " " + word
		default:
			flush()
			line = word
		}
	}
	flush()
	return lines
}

// fitHead shortens s, from its end, with two dots, to fit in width pixels.
func fitHead(r zenui.Renderer, s string, width, scale int) string {
	if r.TextWidth(s, scale) <= width {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[:len(rs)-1]
		if t := string(rs) + ".."; r.TextWidth(t, scale) <= width {
			return t
		}
	}
	return ""
}

// fitTail shortens s, from its start, with two dots, to fit in width pixels: for a path, whose
// end says the most.
func fitTail(r zenui.Renderer, s string, width, scale int) string {
	if r.TextWidth(s, scale) <= width {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[1:]
		if t := ".." + string(rs); r.TextWidth(t, scale) <= width {
			return t
		}
	}
	return ""
}

// drawPreview draws the preview over the window.
func (w *screenshotsWindow) drawPreview(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	v := w.view
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.Colour{A: 0xE6})
	margin := min(screenW, screenH) * previewMarginPercent / 100
	var lines []string // the caption, each line wrapped to the width there is
	for _, l := range v.lines() {
		lines = append(lines, wrapToWidth(r, l, screenW-2*margin, w.scale)...)
	}
	hintLines := len(wrapToWidth(r, previewHint, screenW-2*margin, w.scale)) // the last of them
	capH := len(lines)*w.lineH + w.lineH
	area := zenui.Rect{X: margin, Y: margin, W: screenW - 2*margin, H: screenH - 2*margin - capH}
	if v.err != nil {
		msg := "This file cannot be shown as a picture: " + plainError(v.err)
		for i, line := range wrapToWidth(r, msg, area.W, w.scale) {
			r.DrawText(line, area.X, area.Y+i*w.lineH, w.scale, theme.Text)
		}
	} else if v.made && v.pic != nil {
		pw, ph := v.pic.Size()
		// A picture that fits is shown at a whole number of times its size, so that its pixels
		// stay square and sharp; one that does not is shrunk, and smoothed.
		zoom := min(float32(area.W)/float32(pw), float32(area.H)/float32(ph))
		dw, dh := float32(pw)*zoom, float32(ph)*zoom
		nearest := false
		if zoom >= 1 {
			zoom = float32(int(zoom))
			dw, dh = float32(pw)*zoom, float32(ph)*zoom
			nearest = true
		}
		if want := map[bool]int{true: 1, false: 2}[nearest]; v.filter != want {
			v.filter = want
			if nearest {
				rl.SetTextureFilter(v.tex, rl.FilterPoint)
			} else {
				rl.SetTextureFilter(v.tex, rl.FilterBilinear)
			}
		}
		x := float32(area.X) + (float32(area.W)-dw)/2
		y := float32(area.Y) + (float32(area.H)-dh)/2
		rl.DrawTexturePro(v.tex, rl.Rectangle{Width: float32(pw), Height: float32(ph)},
			rl.Rectangle{X: x, Y: y, Width: dw, Height: dh}, rl.Vector2{}, 0, rl.White)
		r.StrokeRect(zenui.Rect{X: int(x) - 1, Y: int(y) - 1, W: int(dw) + 2, H: int(dh) + 2}, theme.Border, 1)
	}
	y := screenH - margin - capH + w.lineH
	for i, line := range lines {
		col := theme.Text
		if i >= len(lines)-hintLines {
			col = theme.DimText
		}
		r.DrawText(line, margin, y+i*w.lineH, w.scale, col)
	}
}

// --- the window ------------------------------------------------------------------------------

// headLines is the text above the list.
func (w *screenshotsWindow) headLines() []string {
	n := len(w.all)
	noun := "screenshots"
	if n == 1 {
		noun = "screenshot"
	}
	return []string{
		"Folder: " + w.dir + "/",
		fmt.Sprintf("%d %s. Preview shows the chosen one large; a GIF plays.", n, noun),
	}
}

// alignedRows are the rows of the list as rows, with each column as wide as its widest cell, so
// that the columns stand one under another: the cells are padded with spaces to the pixel width,
// which is exact in a monospaced face and within a space's width in another. A name too long for
// the room it has is cut short, and the format after it stays.
func (w *screenshotsWindow) alignedRows(r zenui.Renderer) []string {
	const columns = 4
	cells := make([][columns]string, len(w.all))
	var wide [columns]int
	for i, f := range w.all {
		a, b, c, d := screenshotCells(f)
		cells[i] = [columns]string{a, b, c, d}
		for j, s := range cells[i] {
			wide[j] = max(wide[j], r.TextWidth(s, w.scale))
		}
	}
	sp := max(r.TextWidth(" ", w.scale), 1)
	gap := 2 * sp
	if w.listArea.W > 0 {
		pad := max(2, w.lineH/4)
		room := w.listArea.W - 2*w.lineH - 3*pad - wide[0] - wide[2] - wide[3] - 3*gap
		room = max(room, 8*sp)
		if wide[1] > room {
			wide[1] = room
			for i := range cells {
				cells[i][1] = fitHead(r, cells[i][1], room, w.scale)
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
		rows[i] = pad(c[0], wide[0], false) + "  " + pad(c[1], wide[1], false) + "  " + pad(c[2], wide[2], false) + "  " + pad(c[3], wide[3], true)
	}
	return rows
}

// captionFor is the name under the thumbnail of f in the grid: cut short to fit under it, but never
// so that the format is lost.
func (w *screenshotsWindow) captionFor(r zenui.Renderer, f screenshotFile) string {
	who := screenshotCaption(f)
	if r == nil || w.scale == 0 || w.listArea.W == 0 {
		return who
	}
	kind := " " + strings.ToUpper(f.Ext)
	who = strings.TrimSuffix(who, kind)
	room := w.listArea.W/3 - 2*max(2, w.lineH/4) - r.TextWidth(kind, w.scale)
	return fitHead(r, who, room, w.scale) + kind
}

// fillItems makes the list's items from the files. r measures the text; with none (a test with no
// renderer) the names are not padded.
func (w *screenshotsWindow) fillItems(r zenui.Renderer) {
	w.list.Items = w.list.Items[:0]
	var rows []string
	if w.list.Columns == 1 && r != nil && w.scale > 0 {
		rows = w.alignedRows(r)
	}
	for i, f := range w.all {
		f := f
		name := w.captionFor(r, f)
		if w.list.Columns == 1 {
			name = screenshotRow(f)
			if rows != nil {
				name = rows[i]
			}
		}
		w.list.Items = append(w.list.Items, zenui.ListItem{Name: name,
			Thumb: func(r zenui.Renderer, area zenui.Rect) { w.drawThumb(r, f, area) }})
	}
}

// layout places the window's parts.
func (w *screenshotsWindow) layout(r zenui.Renderer, screenW, screenH int) {
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
	rows := screenshotsRowLines
	if w.list.Columns != 1 {
		rows = screenshotsGridRows
	}
	listH := rows*itemH + 2
	fixed := 2*pad + titleH + len(w.head)*w.lineH + pad + pad + 3*w.lineH + pad + barH
	if over := fixed + listH - (screenH - 2*pad); over > 0 {
		listH = max(listH-over, min(itemH, 2*(w.lineH+4)))
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
func (w *screenshotsWindow) Update(in zenui.Input, r zenui.Renderer, screenW, screenH int) screenshotsResult {
	w.frame++
	w.budget = thumbLoadsPerFrame
	w.clock += in.DeltaTime
	if w.ask != nil {
		w.updateAsk(in)
		return screenshotsOpen
	}
	if w.view != nil {
		w.scale = zenui.TextBodyScale(r, screenW)
		w.lineH = max(r.LineHeight(w.scale), 1)
		w.updateView(in)
		return screenshotsOpen
	}
	w.fillItems(r)
	w.layout(r, screenW, screenH)
	n := len(w.all)
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape:
			return screenshotsClosed
		case zenui.KeyEnter:
			return w.do(shotsBtnPreview)
		case zenui.KeyUp, zenui.KeyDown, zenui.KeyLeft, zenui.KeyRight:
			w.list.Step(k)
			w.list.Reveal(w.listArea, w.lineH)
		case zenui.KeyHome, zenui.KeyEnd, zenui.KeyPageUp, zenui.KeyPageDown:
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
	if ev, ok := w.list.Update(in, w.listArea, w.lineH); ok && ev.Kind == zenui.ListSelect {
		// Two clicks on one screenshot, close together, are a Preview.
		if ev.Index == w.click.item && w.clock-w.click.at < doubleClickSeconds {
			w.click.item = -1
			w.openPreview(ev.Index)
			return screenshotsOpen
		}
		w.click.item, w.click.at = ev.Index, w.clock
	}
	textW := func(s string) int { return r.TextWidth(s, w.scale) }
	if id, ok := w.buttons.Update(in, w.barArea, w.lineH, textW); ok {
		return w.do(id)
	}
	return screenshotsOpen
}

// Draw draws the window over the machine's picture.
func (w *screenshotsWindow) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	w.fillItems(r)
	w.layout(r, screenW, screenH)
	r.FillRect(zenui.Rect{W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(w.panel, theme.Panel)
	r.StrokeRect(w.panel, theme.Border, theme.BorderThickness)

	pad := r.LineHeight(1)
	x, y := w.panel.X+pad, w.panel.Y+pad
	r.DrawText("SCREENSHOTS", x, y, 2, theme.Text)
	y += r.LineHeight(2) + 8
	y = drawDialogHead(r, w.head, x, y, w.panel.W-2*pad, w.scale, w.lineH, theme)

	r.StrokeRect(zenui.Rect{X: w.listArea.X - 1, Y: w.listArea.Y - 1, W: w.listArea.W + 2, H: w.listArea.H + 2}, theme.Border, 1)
	if len(w.all) == 0 {
		for i, line := range wrapToWidth(r, "No screenshots in "+w.dir+"/ yet: View > Screenshot saves one.", w.listArea.W-8, w.scale) {
			r.DrawText(line, w.listArea.X+4, w.listArea.Y+4+i*w.lineH, w.scale, theme.DimText)
		}
	}
	w.list.Draw(r, theme, w.listArea, w.scale)
	w.evict()

	note := w.note
	if f, ok := w.selected(); ok && note == "" {
		note = w.detail(f)
	}
	noteY := w.listArea.Y + w.listArea.H + pad
	noteLines := wrapToWidth(r, note, w.listArea.W, w.scale)
	for i, line := range noteLines {
		if i > 2 {
			break
		}
		if i == 2 && len(noteLines) > 3 { // more than the three lines there are room for
			line = fitHead(r, line+" ...", w.listArea.W, w.scale)
		}
		r.DrawText(line, w.listArea.X, noteY+i*w.lineH, w.scale, theme.DimText)
	}
	w.buttons.Draw(r, theme, w.barArea, w.scale)
	if w.ask != nil {
		w.ask.draw(r, screenW, screenH, theme)
	}
	if w.view != nil {
		w.drawPreview(r, screenW, screenH, theme)
	}
}

// openScreenshots opens the Screenshots window.
func (b *appMenuBar) openScreenshots(zx *ZenZX) {
	b.closeMenus()
	b.shots = newScreenshotsWindow(screenshotDir)
}

// updateScreenshots runs the Screenshots window and gives its textures back when it closes.
func (b *appMenuBar) updateScreenshots(zx *ZenZX) {
	w := b.shots
	if w.Update(zenuiraylib.Input(), b.renderer, rl.GetScreenWidth(), rl.GetScreenHeight()) == screenshotsClosed {
		w.close()
		b.shots = nil
	}
}
