//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/zenui"
)

// shotsOn makes a Screenshots window over a folder holding the files given, each a picture that
// can be read: a PNG, or for the names ending in .gif a GIF of two frames. It has no window to
// make textures in, as the tests have none.
func shotsOn(t *testing.T, names ...string) (*screenshotsWindow, string) {
	t.Helper()
	dir := t.TempDir()
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, solidFrame(32, 24, color.RGBA{0xD7, 0, 0, 0xFF})); err != nil {
		t.Fatal(err)
	}
	gifData, err := encodeGIF([]*image.RGBA{
		solidFrame(32, 24, color.RGBA{0xD7, 0, 0, 0xFF}), solidFrame(32, 24, color.RGBA{0, 0, 0xD7, 0xFF})}, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		data := pngData.Bytes()
		if strings.HasSuffix(n, ".gif") {
			data = gifData
		}
		if err := os.WriteFile(filepath.Join(dir, n), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := newScreenshotsWindow(dir)
	w.noGL = true
	w.setButtons()
	return w, dir
}

func selectedShot(w *screenshotsWindow) string {
	f, ok := w.selected()
	if !ok {
		return ""
	}
	return f.Name
}

func answerShot(w *screenshotsWindow, text string) {
	w.ask.text = text
	w.answer()
}

func TestScreenshotsWindowListsNewestFirstAsAGrid(t *testing.T) {
	w, dir := shotsOn(t, "screenshot_Manic-Miner_20201120_100000.png", "screenshot_Manic-Miner_20201127_093000.gif",
		"screenshot_Next_20201127_093001.nxi", "notes.txt")
	if len(w.all) != 3 {
		t.Fatalf("%d screenshots listed, want the 3 pictures", len(w.all))
	}
	if w.list.Columns != 3 {
		t.Errorf("the window opens with %d columns, want the grid", w.list.Columns)
	}
	if got := selectedShot(w); got != "screenshot_Next_20201127_093001.nxi" {
		t.Errorf("the chosen one is %q, want the newest", got)
	}
	f := w.all[1]
	if got := screenshotCaption(f); got != "Manic-Miner GIF" {
		t.Errorf("the grid's caption is %q", got)
	}
	if row := screenshotRow(f); !strings.Contains(row, "2020-11-27 09:30:00") || !strings.Contains(row, "Manic-Miner") || !strings.Contains(row, "GIF") || !strings.Contains(row, " B") {
		t.Errorf("the row is %q", row)
	}
	// a file that was renamed has no machine in its name: its own name is shown
	renamed := screenshotFile{Name: "my picture.png", Ext: "png", Size: 10, When: time.Now()}
	if got := screenshotCaption(renamed); got != "my picture PNG" {
		t.Errorf("a renamed file's caption is %q", got)
	}
	// the Rows button changes the layout and says what it would change it back to
	w.fillItems(nil)
	if w.list.Items[0].Name != "Next NXI" {
		t.Errorf("the first caption is %q", w.list.Items[0].Name)
	}
	w.do(shotsBtnLayout)
	w.setButtons()
	if w.list.Columns != 1 || w.buttons.Buttons[3].Label != "Grid" {
		t.Errorf("columns %d, the button says %q", w.list.Columns, w.buttons.Buttons[3].Label)
	}
	w.fillItems(nil)
	if !strings.Contains(w.list.Items[0].Name, "Next") || !strings.Contains(w.list.Items[0].Name, "NXI") {
		t.Errorf("the first row is %q", w.list.Items[0].Name)
	}
	if r := w.do(shotsBtnClose); r != screenshotsClosed {
		t.Errorf("Close ended in %v", r)
	}
	_ = dir
}

func TestScreenshotsWindowWithNothingInIt(t *testing.T) {
	w, _ := shotsOn(t, "notes.txt")
	if len(w.all) != 0 {
		t.Fatal("a file that is not a screenshot is listed")
	}
	for _, b := range w.buttons.Buttons {
		if b.ID != shotsBtnClose && b.ID != shotsBtnLayout && !b.Disabled {
			t.Errorf("%s can be pressed with nothing chosen", b.ID)
		}
	}
	for _, id := range []string{shotsBtnPreview, shotsBtnRename, shotsBtnDelete} {
		if r := w.do(id); r != screenshotsOpen || w.ask != nil || w.view != nil {
			t.Errorf("%s did something with nothing chosen", id)
		}
	}
}

func TestScreenshotKindsSayWhatTheyAre(t *testing.T) {
	for _, c := range []struct {
		f    screenshotFile
		want string
	}{
		{screenshotFile{Ext: "png"}, "PNG"},
		{screenshotFile{Ext: "gif"}, "plays"},
		{screenshotFile{Ext: "scr", Size: 6912}, "attributes"},
		{screenshotFile{Ext: "scr", Size: 12288}, "guesses"},
		{screenshotFile{Ext: "scr", Size: 12289}, "hi-res"},
		{screenshotFile{Ext: "scr", Size: 77}, "not a Spectrum"},
		{screenshotFile{Ext: "nxi"}, "palette"},
	} {
		if got := screenshotKind(c.f); !strings.Contains(got, c.want) {
			t.Errorf("%s of %d bytes: %q, want it to say %q", c.f.Ext, c.f.Size, got, c.want)
		}
	}
}

func TestScreenshotsWindowRenames(t *testing.T) {
	w, dir := shotsOn(t, "screenshot_Next_20201127_093001.png", "taken.png", "other.gif")
	w.selectPath(filepath.Join(dir, "screenshot_Next_20201127_093001.png"))
	w.do(shotsBtnRename)
	if w.ask == nil || w.ask.kind != askRenameName || w.ask.text != "screenshot_Next_20201127_093001" || w.ask.noun != "screenshot" {
		t.Fatalf("the question is %+v", w.ask)
	}
	for text, why := range map[string]string{
		"":          "Type a name.",
		"a/b":       "no slashes",
		"taken":     "is there already",
		"taken.png": "is there already",
		"new.gif":   "stays .png",
		"new.scr":   "stays .png",
		"new.nxi":   "stays .png",
		"what?":     "none of",
	} {
		answerShot(w, text)
		if w.ask == nil || w.ask.kind != askRenameName || !strings.Contains(w.ask.why, why) {
			t.Errorf("%q: asking %+v, want it to say %q", text, w.ask, why)
		}
	}
	answerShot(w, "Level 3")
	if w.ask == nil || w.ask.kind != askRenameSure || filepath.Base(w.ask.target) != "Level 3.png" {
		t.Fatalf("the question is %+v", w.ask)
	}
	if !exists(filepath.Join(dir, "screenshot_Next_20201127_093001.png")) {
		t.Fatal("the screenshot was renamed before the question was answered")
	}
	w.answer()
	if w.ask != nil || !exists(filepath.Join(dir, "Level 3.png")) || exists(filepath.Join(dir, "screenshot_Next_20201127_093001.png")) {
		t.Fatalf("the screenshot was not renamed (note %q)", w.note)
	}
	if got := selectedShot(w); got != "Level 3.png" {
		t.Errorf("the chosen one is %q", got)
	}
	if f, _ := w.selected(); f.Machine != "" {
		t.Errorf("a renamed screenshot still has the machine %q", f.Machine)
	}
	// declining leaves it as it was
	w.do(shotsBtnRename)
	answerShot(w, "elsewhere")
	w.updateAsk(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
	if w.ask != nil || !exists(filepath.Join(dir, "Level 3.png")) || exists(filepath.Join(dir, "elsewhere.png")) {
		t.Errorf("a refused rename changed things: %v", listNames(dir))
	}
}

func TestScreenshotsWindowDeletes(t *testing.T) {
	w, dir := shotsOn(t, "a.png", "b.png", "c.gif")
	w.selectPath(filepath.Join(dir, "b.png"))
	at := w.list.Selected
	w.do(shotsBtnDelete)
	if w.ask == nil || w.ask.kind != askDelete || w.ask.noun != "screenshot" {
		t.Fatalf("the question is %+v", w.ask)
	}
	for _, text := range []string{"", "delete", "DELETE ", "yes"} {
		answerShot(w, text)
		if w.ask == nil || w.ask.kind != askDelete || !exists(filepath.Join(dir, "b.png")) {
			t.Fatalf("%q: asking %+v", text, w.ask)
		}
	}
	answerShot(w, deleteWord)
	if w.ask != nil || exists(filepath.Join(dir, "b.png")) {
		t.Fatalf("the screenshot was not deleted (note %q)", w.note)
	}
	if len(w.all) != 2 || w.list.Selected != at {
		t.Errorf("%d listed, chosen %d, want 2 and %d", len(w.all), w.list.Selected, at)
	}
	// the thumbnail of a file that is gone is given back
	for _, k := range keys(w.thumbs) {
		if strings.Contains(k, "b.png") {
			t.Errorf("the deleted file still has a thumbnail: %s", k)
		}
	}
}

func keys[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestScreenshotsWindowEscapeDropsTheQuestion(t *testing.T) {
	w, dir := shotsOn(t, "a.png")
	w.do(shotsBtnDelete)
	w.ask.text = deleteWord
	w.updateAsk(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
	if w.ask != nil || !exists(filepath.Join(dir, "a.png")) || !strings.Contains(w.note, "Not deleted") {
		t.Errorf("Escape: asking %+v, note %q", w.ask, w.note)
	}
}

func TestThumbnailsAreMadeALittleAtATimeAndFailuresShowAsSuch(t *testing.T) {
	w, dir := shotsOn(t, "a.png", "b.png", "c.png", "d.gif")
	if err := os.WriteFile(filepath.Join(dir, "broken.png"), []byte("not a png"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.refresh()
	if len(w.all) != 5 {
		t.Fatalf("%d listed", len(w.all))
	}
	w.budget = thumbLoadsPerFrame
	made, waiting := 0, 0
	for _, f := range w.all {
		if w.thumbFor(f) != nil {
			made++
		} else {
			waiting++
		}
	}
	if made != thumbLoadsPerFrame || waiting != len(w.all)-thumbLoadsPerFrame {
		t.Errorf("%d thumbnails made in a frame, %d left waiting, want %d made", made, waiting, thumbLoadsPerFrame)
	}
	// the rest, a frame at a time
	for frame := 0; frame < 5; frame++ {
		w.budget = thumbLoadsPerFrame
		for _, f := range w.all {
			w.thumbFor(f)
		}
	}
	for _, f := range w.all {
		t0 := w.thumbs[thumbKey(f)]
		switch {
		case t0 == nil:
			t.Errorf("%s has no thumbnail after many frames", f.Name)
		case f.Name == "broken.png" && (t0.ok || t0.why == ""):
			t.Errorf("a file that is not a PNG: ok %v, why %q", t0.ok, t0.why)
		case f.Name != "broken.png" && (!t0.ok || t0.w != 32 || t0.h != 24):
			t.Errorf("%s: ok %v, %dx%d, want 32x24", f.Name, t0.ok, t0.w, t0.h)
		}
	}
	// the note says why a file has no picture
	for _, f := range w.all {
		if f.Name == "broken.png" {
			if got := w.detail(f); !strings.Contains(got, "cannot be shown") {
				t.Errorf("the note for a broken file is %q", got)
			}
		}
	}
	// and gives the picture's size for one that has
	if got := w.detail(w.all[1]); !strings.Contains(got, "32x24") {
		t.Errorf("the note is %q, want the picture's size in it", got)
	}
}

func TestPreviewPlaysAGIFAndTheKeysMoveAboutIt(t *testing.T) {
	w, _ := shotsOn(t, "screenshot_A_20201127_090000.png", "screenshot_B_20201127_090100.gif", "screenshot_C_20201127_090200.png")
	w.selectPath(filepath.Join(w.dir, "screenshot_B_20201127_090100.gif"))
	w.do(shotsBtnPreview)
	v := w.view
	if v == nil || v.err != nil || !v.pic.Animated() || len(v.pic.Frames) != 2 {
		t.Fatalf("the preview is %+v", v)
	}
	lines := strings.Join(v.lines(), "\n")
	if !strings.Contains(lines, "32x24") || !strings.Contains(lines, "2 frames") || !strings.Contains(lines, "0.1 seconds") {
		t.Errorf("the preview says:\n%s", lines)
	}
	// a delay of 4 hundredths each: the second frame is due after 0.04 s
	w.updateView(zenui.Input{DeltaTime: 0.02})
	if w.view.frame != 0 {
		t.Errorf("frame %d after 0.02 s, want 0", w.view.frame)
	}
	w.updateView(zenui.Input{DeltaTime: 0.03})
	if w.view.frame != 1 {
		t.Errorf("frame %d after 0.05 s, want 1", w.view.frame)
	}
	w.updateView(zenui.Input{DeltaTime: 0.04})
	if w.view.frame != 0 {
		t.Errorf("frame %d after 0.09 s, want it to have gone round to 0", w.view.frame)
	}
	// the arrow keys show the screenshots beside it (the list is newest first: C, B, A)
	w.updateView(zenui.Input{Keys: []zenui.Key{zenui.KeyRight}})
	if w.view == nil || w.view.file.Name != "screenshot_A_20201127_090000.png" || selectedShot(w) != w.view.file.Name {
		t.Fatalf("Right shows %+v", w.view)
	}
	w.updateView(zenui.Input{Keys: []zenui.Key{zenui.KeyRight}}) // there is none after the last
	if w.view == nil || w.view.file.Name != "screenshot_A_20201127_090000.png" {
		t.Errorf("Right at the end shows %+v", w.view)
	}
	w.updateView(zenui.Input{Keys: []zenui.Key{zenui.KeyLeft}})
	w.updateView(zenui.Input{Keys: []zenui.Key{zenui.KeyLeft}})
	if w.view == nil || w.view.file.Name != "screenshot_C_20201127_090200.png" {
		t.Errorf("two Lefts show %+v", w.view)
	}
	// Escape closes the preview, not the window; a click closes it too
	w.updateView(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}})
	if w.view != nil {
		t.Error("Escape did not close the preview")
	}
	w.openPreview(0)
	w.updateView(zenui.Input{MousePressed: true})
	if w.view != nil {
		t.Error("a click did not close the preview")
	}
}

func TestPreviewOfAFileThatIsNotAPictureSaysSo(t *testing.T) {
	w, dir := shotsOn(t, "a.png")
	if err := os.WriteFile(filepath.Join(dir, "z.scr"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	w.refresh()
	w.selectPath(filepath.Join(dir, "z.scr"))
	w.do(shotsBtnPreview)
	if w.view == nil || w.view.err == nil {
		t.Fatalf("the preview of a bad SCR is %+v", w.view)
	}
	w.updateView(zenui.Input{DeltaTime: 1}) // nothing to play
	w.closePreview()
}

// The window runs with the real renderer: Enter opens the preview, Escape closes it, and a second
// Escape closes the window; two clicks on one screenshot are a Preview.
func TestScreenshotsWindowRunsFrames(t *testing.T) {
	b, _, _ := shotBar(t)
	w, _ := shotsOn(t, "screenshot_A_20201127_090000.png", "screenshot_B_20201127_090100.png")
	key := func(k zenui.Key) zenui.Input { return zenui.Input{Keys: []zenui.Key{k}, DeltaTime: 0.016} }
	const sw, sh = 1280, 800
	if r := w.Update(zenui.Input{DeltaTime: 0.016}, b.renderer, sw, sh); r != screenshotsOpen {
		t.Fatalf("a quiet frame ended in %v", r)
	}
	if w.listArea.W <= 0 || w.listArea.H <= 0 || w.panel.W > sw || w.panel.H > sh {
		t.Fatalf("the panel is %+v and the list %+v on a %dx%d screen", w.panel, w.listArea, sw, sh)
	}
	if r := w.Update(key(zenui.KeyEnter), b.renderer, sw, sh); r != screenshotsOpen || w.view == nil {
		t.Fatalf("Enter: %v, preview %v", r, w.view)
	}
	if r := w.Update(key(zenui.KeyEscape), b.renderer, sw, sh); r != screenshotsOpen || w.view != nil {
		t.Fatalf("Escape in the preview: %v, preview %v", r, w.view)
	}
	if r := w.Update(key(zenui.KeyEscape), b.renderer, sw, sh); r != screenshotsClosed {
		t.Fatalf("Escape in the window ended in %v", r)
	}

	// two clicks on the first item
	w.Update(zenui.Input{DeltaTime: 0.016}, b.renderer, sw, sh)
	cell := w.list.ItemRect(0, w.listArea, w.lineH)
	click := zenui.Input{MouseX: cell.X + cell.W/2, MouseY: cell.Y + cell.H/2, MousePressed: true, DeltaTime: 0.05}
	w.Update(click, b.renderer, sw, sh)
	if w.view != nil || w.list.Selected != 0 {
		t.Fatalf("one click: preview %v, chosen %d", w.view, w.list.Selected)
	}
	w.Update(click, b.renderer, sw, sh)
	if w.view == nil {
		t.Error("two clicks close together did not open the preview")
	}
	w.closePreview()
	// two clicks far apart are two selections
	w.Update(click, b.renderer, sw, sh)
	w.Update(zenui.Input{DeltaTime: 1}, b.renderer, sw, sh)
	w.Update(click, b.renderer, sw, sh)
	if w.view != nil {
		t.Error("two clicks a second apart opened the preview")
	}
	// the window fits a small screen too
	w.Update(zenui.Input{DeltaTime: 0.016}, b.renderer, 640, 360)
	if w.panel.H > 360 || w.panel.W > 640 {
		t.Errorf("the panel is %dx%d on a 640x360 screen", w.panel.W, w.panel.H)
	}
}

func TestManageScreenshotsOpensTheWindow(t *testing.T) {
	b, zx, _ := shotBar(t)
	at, _ := shotItem(t, b)
	if b.shots != nil {
		t.Fatal("the window is open already")
	}
	if !b.viewMenu.pick(zx, at, len(ShotFormats)+1) { // Manage Screenshots...
		t.Fatal("picking Manage Screenshots ran nothing")
	}
	if b.shots == nil || b.shots.dir != screenshotDir {
		t.Fatalf("the window is %+v, want one on %q", b.shots, screenshotDir)
	}
	if !b.Active() {
		t.Error("the bar does not say a window is open")
	}
	b.shots.noGL = true
	b.shots.close()
	b.shots = nil
}

func TestWrapAndFitKeepTextInsideItsWidth(t *testing.T) {
	b, _, _ := shotBar(t)
	r := b.renderer
	const scale = 1
	text := "A ZX Spectrum screen file: the display memory, a bitmap and its attributes, and some more words after them."
	for _, width := range []int{120, 200, 333, 640} {
		lines := wrapToWidth(r, text, width, scale)
		if strings.Join(lines, " ") != text {
			t.Errorf("width %d: the lines %q do not make the text again", width, lines)
		}
		for _, l := range lines {
			if r.TextWidth(l, scale) > width {
				t.Errorf("width %d: %q is %d wide", width, l, r.TextWidth(l, scale))
			}
		}
	}
	// a word wider than the line is cut, and nothing is lost
	long := "x" + strings.Repeat("abcdefghij", 20)
	lines := wrapToWidth(r, long, 100, scale)
	if strings.Join(lines, "") != long || len(lines) < 2 {
		t.Errorf("a long word came out as %q", lines)
	}
	for _, l := range lines {
		if r.TextWidth(l, scale) > 100 {
			t.Errorf("%q is %d wide, over 100", l, r.TextWidth(l, scale))
		}
	}
	path := "Folder: /home/somebody/documents/a/rather/long/path/to/the/screenshots/"
	for _, width := range []int{100, 250, 10000} {
		tail, head := fitTail(r, path, width, scale), fitHead(r, path, width, scale)
		if r.TextWidth(tail, scale) > width || r.TextWidth(head, scale) > width {
			t.Errorf("width %d: %q and %q do not fit", width, tail, head)
		}
		if width == 10000 && (tail != path || head != path) {
			t.Errorf("text that fits was changed: %q, %q", tail, head)
		}
		if width == 250 && (!strings.HasSuffix(tail, "screenshots/") || !strings.HasPrefix(head, "Folder:")) {
			t.Errorf("a path keeps its end (%q) and a sentence its start (%q)", tail, head)
		}
	}
	if got := wrapToWidth(r, "", 100, scale); len(got) != 1 {
		t.Errorf("empty text wraps to %q", got)
	}
}

// In rows, the format is a column of its own and the columns stand one under another, whatever the
// names are; a name that is too long is cut short, and the format after it stays.
func TestScreenshotRowsAreInColumnsAndKeepTheirFormat(t *testing.T) {
	b, _, _ := shotBar(t)
	r := b.renderer
	w, dir := shotsOn(t, "screenshot_A_20201127_090000.png", "screenshot_Jet-Set-Willy_20201127_090100.gif",
		"screenshot_Next_20201127_090200.nxi", "my picture.scr",
		"screenshot_An-extremely-long-name-for-a-machine-that-goes-on-and-on-and-on-and-on_20201127_090300.png")
	_ = dir
	w.scale, w.lineH = 1, max(r.LineHeight(1), 1)
	w.listArea = zenui.Rect{W: 600, H: 400}
	w.list.Columns = 1
	w.fillItems(r)
	sp := r.TextWidth(" ", 1)
	var at []int
	for i, it := range w.list.Items {
		f := w.all[i]
		kind := "  " + strings.ToUpper(f.Ext) + "  "
		k := strings.LastIndex(it.Name, kind)
		if k < 0 {
			t.Fatalf("row %q has no column for the format %q", it.Name, strings.ToUpper(f.Ext))
		}
		at = append(at, r.TextWidth(it.Name[:k], 1))
		if r.TextWidth(it.Name, 1) > w.listArea.W {
			t.Errorf("row %q is %d wide, over the %d there is", it.Name, r.TextWidth(it.Name, 1), w.listArea.W)
		}
	}
	for i, x := range at {
		if d := x - at[0]; d > sp || d < -sp {
			t.Errorf("row %d has its format at %d, row 0 at %d: not in a column (a space is %d wide)", i, x, at[0], sp)
		}
	}
	// the file with no machine in its name shows the name it was given, without its extension
	for _, it := range w.list.Items {
		if strings.Contains(it.Name, "my picture") && strings.Contains(it.Name, ".scr") {
			t.Errorf("the extension is in the name as well as in its column: %q", it.Name)
		}
	}
	// in the grid, a caption cut short keeps the format
	w.list.Columns = 3
	w.listArea.W = 300
	w.fillItems(r)
	for i, it := range w.list.Items {
		if !strings.HasSuffix(it.Name, " "+strings.ToUpper(w.all[i].Ext)) {
			t.Errorf("the caption %q has lost its format", it.Name)
		}
		if r.TextWidth(it.Name, 1) > w.listArea.W/3 {
			t.Errorf("the caption %q is wider than its column", it.Name)
		}
	}
}
