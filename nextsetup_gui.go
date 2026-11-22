//go:build !headless

package zenzx

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The window that appears the first time a Next is started without its boot files: it says
// why they are needed, asks before downloading, lets the two addresses be changed, shows
// the progress of the download and of the card being built, and reports the verification.
// The work itself is nextcard.Setup, on a goroutine of its own; the window only reads its
// progress, so the machine and the window keep running meanwhile.
//
// It is drawn as the Help and About windows are (markdownmodal_gui.go): the same backdrop
// and panel in the colours of the selected theme, the same text size, and a body that
// scrolls when the window is too small for it. Below the body it keeps what must always be
// in reach: the address fields, the progress bar and the buttons.

type nextSetupPhase int

const (
	nextSetupAsking nextSetupPhase = iota
	nextSetupRunning
	nextSetupFailed
)

// nextSetupResult is what update reports to the menu bar.
type nextSetupResult int

const (
	nextSetupOpen     nextSetupResult = iota // still on screen
	nextSetupDeclined                        // closed without the files
	nextSetupReady                           // the files are there and verified
)

type nextSetup struct {
	phase   nextSetupPhase
	dir     string
	distURL string
	bootURL string
	focus   int    // the address field that takes typing: 0 distribution, 1 boot ROM
	note    string // why the files that were found cannot be used, when there were some
	failure error

	// What the last draw laid out; update works on it.
	title    string
	lines    []string
	buttons  []string
	scroll   int
	panel    zenui.Rect
	body     zenui.Rect
	closeBx  zenui.Rect
	fields   [2]zenui.Rect
	bar      zenui.Rect
	btnRects []zenui.Rect
	track    zenui.Rect
	thumb    zenui.Rect
	visible  int
	bodyLH   int
	effScale int
	dragging bool
	dragOff  int
	mouseX   int
	mouseY   int

	cancel    context.CancelFunc
	cancelled bool // Cancel was chosen while the work was running
	// run is the function that does the work; a test replaces it.
	run func(ctx context.Context, opts nextcard.Options, progress nextcard.Progress) error
	// paste returns the clipboard's text when Ctrl+V (Cmd+V) was pressed this frame.
	paste func() (string, bool)

	mu       sync.Mutex
	stage    nextcard.Stage
	done     int64
	total    int64
	finished bool
	err      error
}

// newNextSetup makes the window for downloads to dir. note, when not empty, says why boot
// files that were found are not being used.
func newNextSetup(dir, note string) *nextSetup {
	return &nextSetup{dir: dir, note: note, run: nextcard.Setup, paste: clipboardPaste,
		distURL: nextcard.DefaultDistributionURL, bootURL: nextcard.DefaultBootROMURL}
}

func clipboardPaste() (string, bool) {
	ctrl := rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) ||
		rl.IsKeyDown(rl.KeyLeftSuper) || rl.IsKeyDown(rl.KeyRightSuper)
	if ctrl && rl.IsKeyPressed(rl.KeyV) {
		return rl.GetClipboardText(), true
	}
	return "", false
}

// nextSetupCols is the width the body text is written for, in characters; the panel is as
// wide as the Help window, which holds 70.
const nextSetupCols = 68

// content gives the title, the body lines and the buttons of the phase the window is in.
// Lines follow the Help window's conventions: "## " is a heading, two leading spaces dim.
func (s *nextSetup) content() (title string, lines, buttons []string) {
	folder := zenui.WrapText(s.dir, nextSetupCols-2, "  ")
	switch s.phase {
	case nextSetupRunning:
		s.mu.Lock()
		stage := s.stage
		s.mu.Unlock()
		lines = []string{fmt.Sprintf("## Step %d of 5: %s", int(stage)+1, stage), "", "The files go to:"}
		lines = append(lines, folder...)
		return "ZX SPECTRUM NEXT: GETTING THE FILES", lines, []string{"Cancel"}
	case nextSetupFailed:
		lines = []string{"## The files could not be made ready", ""}
		lines = append(lines, zenui.WrapText(s.failure.Error(), nextSetupCols-2, "  ")...)
		lines = append(lines, "",
			"Nothing that was already there was changed. The Next runs with the",
			"+3's ROMs until this succeeds.")
		return "ZX SPECTRUM NEXT: FIRST START", lines, []string{"Close", "Try again"}
	}
	lines = []string{
		"## Why this is needed",
		"The ZX Spectrum Next starts from an SD card that holds its firmware",
		"and NextZXOS, and from two small ROMs. ZenZX does not include them.",
		"",
		"## What happens",
		"They are downloaded now, once. They are kept in this folder, and",
		"every later start of the Next finds them there:",
	}
	lines = append(lines, folder...)
	lines = append(lines, "",
		"The download is about 70 MB. The card in it is then built again as",
		"one the Next's firmware accepts (1 GB; about 2.2 GB of disk is used",
		"while that runs), and the result is verified before the Next starts.",
		"",
		"Nothing is downloaded until you choose Download. Without the files",
		"the Next runs with the +3's ROMs. The two addresses can be changed.")
	if s.note != "" {
		lines = append(lines, "", "## Files that were found and not used")
		lines = append(lines, zenui.WrapText(s.note, nextSetupCols-2, "  ")...)
	}
	return "ZX SPECTRUM NEXT: FIRST START", lines, []string{"Not now", "Download"}
}

// layout places the panel, the scrolling body and what is fixed below it.
func (s *nextSetup) layout(r zenui.Renderer, screenW, screenH int) {
	s.title, s.lines, s.buttons = s.content()
	lh := r.LineHeight(1)
	pad := lh
	s.effScale = zenui.TextBodyScale(r, screenW)
	s.bodyLH = r.LineHeight(s.effScale)
	if s.bodyLH < 1 {
		s.bodyLH = 1
	}
	pw := zenui.TextPanelWidth(r, s.effScale)
	if pw < 320 {
		pw = 320
	}
	if pw > screenW-2*pad {
		pw = screenW - 2*pad
	}
	titleH := r.LineHeight(2) + 8
	rowH := s.bodyLH + 8 // an address field, the progress bar, a button
	gap := pad / 2
	sep := s.bodyLH // a blank line between the scrolling body and what is fixed below it

	// The fixed part, from the bottom up: buttons; then two labelled fields while asking,
	// or the bar and its figures while running.
	footerH := rowH
	switch s.phase {
	case nextSetupAsking:
		footerH += gap + 2*(s.bodyLH+rowH+gap)
	case nextSetupRunning:
		footerH += gap + rowH + s.bodyLH + gap
	}
	ph := 2*pad + titleH + len(s.lines)*s.bodyLH + sep + footerH
	if ph > screenH-2*pad {
		ph = screenH - 2*pad
	}
	px, py := (screenW-pw)/2, (screenH-ph)/2
	s.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}
	cb := r.LineHeight(2)
	s.closeBx = zenui.Rect{X: px + pw - pad - cb, Y: py + pad, W: cb, H: cb}

	bodyH := ph - 2*pad - titleH - sep - footerH
	if bodyH < s.bodyLH {
		bodyH = s.bodyLH
	}
	s.body = zenui.Rect{X: px + pad, Y: py + pad + titleH, W: pw - 2*pad - 12, H: bodyH}
	s.visible = bodyH / s.bodyLH
	if s.visible < 1 {
		s.visible = 1
	}
	s.clampScroll()

	inner := zenui.Rect{X: px + pad, W: pw - 2*pad}
	y := s.body.Y + s.body.H + sep
	s.fields, s.bar = [2]zenui.Rect{}, zenui.Rect{}
	switch s.phase {
	case nextSetupAsking:
		for i := range s.fields {
			y += s.bodyLH
			s.fields[i] = zenui.Rect{X: inner.X, Y: y, W: inner.W, H: rowH}
			y += rowH + gap
		}
	case nextSetupRunning:
		s.bar = zenui.Rect{X: inner.X, Y: y, W: inner.W, H: rowH}
		y += rowH + s.bodyLH + gap
	}
	s.btnRects = s.btnRects[:0]
	x := inner.X + inner.W
	for i := len(s.buttons) - 1; i >= 0; i-- {
		w := r.TextWidth(s.buttons[i], s.effScale) + 2*pad
		x -= w
		s.btnRects = append([]zenui.Rect{{X: x, Y: y, W: w, H: rowH}}, s.btnRects...)
		x -= gap
	}
}

func (s *nextSetup) clampScroll() {
	if max := len(s.lines) - s.visible; s.scroll > max {
		s.scroll = max
	}
	if s.scroll < 0 {
		s.scroll = 0
	}
}

// draw shows the window for the phase it is in, in the colours of theme.
func (s *nextSetup) draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	s.layout(r, screenW, screenH)
	lh := r.LineHeight(1)
	pad := lh

	r.FillRect(zenui.Rect{X: 0, Y: 0, W: screenW, H: screenH}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening))
	r.FillRect(s.panel, zenui.ScaleAlpha(theme.Panel, zenui.ModalPanelOpacity))
	r.StrokeRect(s.panel, theme.Border, 1)
	r.DrawText(s.title, s.panel.X+pad, s.panel.Y+pad, 2, theme.Text)

	hot := func(rec zenui.Rect) zenui.Colour {
		if rec.Contains(s.mouseX, s.mouseY) {
			return theme.ButtonHot
		}
		return theme.Button
	}
	r.FillRect(s.closeBx, hot(s.closeBx))
	r.StrokeRect(s.closeBx, theme.Border, 1)
	r.DrawText("x", s.closeBx.X+(s.closeBx.W-r.TextWidth("x", 1))/2, s.closeBx.Y+(s.closeBx.H-lh)/2, 1, theme.ButtonText)

	r.Clip(s.body)
	y := s.body.Y
	for i := s.scroll; i < len(s.lines) && i < s.scroll+s.visible; i++ {
		zenui.DrawTextLine(r, s.lines[i], s.body.X, y, s.effScale, theme)
		y += s.bodyLH
	}
	r.ClipEnd()

	s.track, s.thumb = zenui.Rect{}, zenui.Rect{}
	if total := len(s.lines); total > s.visible {
		s.track = zenui.Rect{X: s.body.X + s.body.W + 6, Y: s.body.Y, W: 8, H: s.body.H}
		r.FillRect(s.track, theme.Button)
		thumbH := s.track.H * s.visible / total
		if thumbH < 16 {
			thumbH = 16
		}
		if thumbH > s.track.H {
			thumbH = s.track.H
		}
		s.thumb = zenui.Rect{X: s.track.X, Y: s.track.Y + (s.track.H-thumbH)*s.scroll/(total-s.visible), W: s.track.W, H: thumbH}
		col := theme.DirText
		if s.dragging {
			col = theme.Text
		}
		r.FillRect(s.thumb, col)
	}

	textY := func(rec zenui.Rect) int { return rec.Y + (rec.H-s.bodyLH)/2 }
	switch s.phase {
	case nextSetupAsking:
		labels := [2]string{"Distribution (the card and the DivMMC ROM):", "Boot ROM:"}
		texts := [2]string{s.distURL, s.bootURL}
		for i, rec := range s.fields {
			r.DrawText(labels[i], rec.X, rec.Y-s.bodyLH, s.effScale, theme.DimText)
			r.FillRect(rec, theme.Field)
			text := texts[i]
			if i == s.focus {
				text += "_"
			}
			r.Clip(rec)
			r.DrawText(zenui.FitTail(r, text, s.effScale, rec.W-8), rec.X+4, textY(rec), s.effScale, theme.Text)
			r.ClipEnd()
			if i == s.focus {
				r.StrokeRect(rec, theme.SelFill, 2)
			} else {
				r.StrokeRect(rec, theme.Border, 1)
			}
		}
	case nextSetupRunning:
		s.mu.Lock()
		done, total := s.done, s.total
		s.mu.Unlock()
		zenui.DrawProgressBar(r, s.bar, done, total, theme)
		r.DrawText(zenui.ProgressText(done, total), s.bar.X, s.bar.Y+s.bar.H+2, s.effScale, theme.Text)
	}
	for i, rec := range s.btnRects {
		r.FillRect(rec, hot(rec))
		r.StrokeRect(rec, theme.Border, 1)
		label := s.buttons[i]
		r.DrawText(label, rec.X+(rec.W-r.TextWidth(label, s.effScale))/2, textY(rec), s.effScale, theme.ButtonText)
	}
}

func (s *nextSetup) start() {
	s.phase, s.scroll, s.cancelled = nextSetupRunning, 0, false
	s.mu.Lock()
	s.stage, s.done, s.total, s.finished, s.err = nextcard.StageBootROM, 0, 0, false, nil
	s.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	opts := nextcard.Options{Dir: s.dir, DistributionURL: strings.TrimSpace(s.distURL), BootROMURL: strings.TrimSpace(s.bootURL)}
	go func() {
		err := s.run(ctx, opts, func(stage nextcard.Stage, done, total int64) {
			s.mu.Lock()
			s.stage, s.done, s.total = stage, done, total
			s.mu.Unlock()
		})
		s.mu.Lock()
		s.finished, s.err = true, err
		s.mu.Unlock()
	}()
}

// scrollInput moves the body: wheel, Page Up and Down, the arrow keys, and the scrollbar.
func (s *nextSetup) scrollInput(in zenui.Input) {
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyUp:
			s.scroll--
		case zenui.KeyDown:
			s.scroll++
		case zenui.KeyPageUp:
			s.scroll -= s.visible
		case zenui.KeyPageDown:
			s.scroll += s.visible
		}
	}
	if in.WheelY != 0 {
		s.scroll -= int(in.WheelY) * 3
	}
	if in.MousePressed && s.thumb.W > 0 {
		if s.thumb.Contains(in.MouseX, in.MouseY) {
			s.dragging, s.dragOff = true, in.MouseY-s.thumb.Y
		} else if s.track.Contains(in.MouseX, in.MouseY) {
			if in.MouseY < s.thumb.Y {
				s.scroll -= s.visible
			} else {
				s.scroll += s.visible
			}
		}
	}
	if s.dragging {
		if !in.MouseDown {
			s.dragging = false
		} else if span := s.track.H - s.thumb.H; span > 0 {
			s.scroll = (in.MouseY - s.dragOff - s.track.Y) * (len(s.lines) - s.visible) / span
		}
	}
	s.clampScroll()
}

// clicked returns the index of the button under a press this frame, len(buttons) for the
// close box, and -1 for neither.
func (s *nextSetup) clicked(in zenui.Input) int {
	if !in.MousePressed {
		return -1
	}
	if s.closeBx.Contains(in.MouseX, in.MouseY) {
		return len(s.buttons)
	}
	for i, rec := range s.btnRects {
		if rec.Contains(in.MouseX, in.MouseY) {
			return i
		}
	}
	return -1
}

func pressed(in zenui.Input, key zenui.Key) bool {
	for _, k := range in.Keys {
		if k == key {
			return true
		}
	}
	return false
}

// editAddress applies this frame's typing to the address field that has the focus.
func (s *nextSetup) editAddress(in zenui.Input) {
	if pressed(in, zenui.KeyTab) {
		s.focus = 1 - s.focus
	}
	if in.MousePressed {
		for i, rec := range s.fields {
			if rec.Contains(in.MouseX, in.MouseY) {
				s.focus = i
			}
		}
	}
	text := &s.distURL
	if s.focus == 1 {
		text = &s.bootURL
	}
	if s.paste != nil {
		if clip, ok := s.paste(); ok {
			*text += strings.TrimSpace(clip)
		}
	}
	for _, c := range in.Chars {
		if c >= ' ' && c != 0x7F {
			*text += string(c)
		}
	}
	if pressed(in, zenui.KeyBackspace) && *text != "" {
		_, n := utf8.DecodeLastRuneInString(*text)
		*text = (*text)[:len(*text)-n]
	}
}

// update takes this frame's input and says whether the window is finished.
func (s *nextSetup) update(in zenui.Input) nextSetupResult {
	s.mouseX, s.mouseY = in.MouseX, in.MouseY
	s.scrollInput(in)
	click := s.clicked(in)
	closeBox := click == len(s.buttons) && click >= 0 && in.MousePressed
	escape := pressed(in, zenui.KeyEscape)
	enter := pressed(in, zenui.KeyEnter)

	switch s.phase {
	case nextSetupRunning:
		s.mu.Lock()
		finished, err := s.finished, s.err
		s.mu.Unlock()
		if finished {
			s.cancel()
			if err == nil {
				return nextSetupReady
			}
			if s.cancelled {
				return nextSetupDeclined
			}
			s.phase, s.failure, s.scroll = nextSetupFailed, err, 0
			return nextSetupOpen
		}
		if escape || closeBox || click == 0 {
			s.cancelled = true
			s.cancel() // the goroutine ends on its own; the window closes when it has
		}
	case nextSetupFailed:
		switch {
		case escape || closeBox || click == 0:
			return nextSetupDeclined
		case enter || click == 1:
			s.start()
		}
	default:
		s.editAddress(in)
		switch {
		case escape || closeBox || click == 0:
			return nextSetupDeclined
		case enter || click == 1:
			s.start()
		}
	}
	return nextSetupOpen
}
