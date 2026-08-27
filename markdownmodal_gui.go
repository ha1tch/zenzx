//go:build !headless

package main

import (
	_ "embed"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenzx/pkg/zenui"
)

//go:embed help.txt
var helpText string

//go:embed about.txt
var aboutTextTemplate string

// markdownModal is a scrollable text reader for a small, lightweight
// markdown subset -- "# H1"/"## H2" headings, indented lines (two
// leading spaces, dimmer text, for shortcut tables), everything else
// plain body text. Shown over the running emulator with a dim backdrop,
// dismissed via Escape, its own close box, or a click outside the
// panel.
//
// Adapted from zenimate's own helpModal
// (github.com/ha1tch/zenimate/cmd/zenimate-gui/helpmodal.go, the same
// project pkg/zenui/pkg/bdf/pkg/zxpalette/pkg/zenuiraylib were
// originally ported from), generalised to take its title and content
// directly rather than a single embedded help.txt, so Help and About
// can share one implementation instead of two near-identical copies --
// and to take the active zenui.Theme as a parameter, since zenzx has
// several themes where zenimate had one fixed one.
const (
	markdownBodyScaleBase = 2
	markdownTargetCols    = 70 // preferred panel width, in characters
)

// markdownPanelWidth returns the panel width for a 70-column body at
// the given text scale: the measured text width plus horizontal
// padding and the scrollbar gutter.
func markdownPanelWidth(r zenui.Renderer, scale int) int {
	pad := r.LineHeight(1)
	cols := strings.Repeat("M", markdownTargetCols)
	return r.TextWidth(cols, scale) + 2*pad + 12
}

// markdownBodyScaleFor picks the body text scale: the base scale if a
// 70-column panel fits within the screen width at that scale,
// otherwise 1. Bitmap fonts only look crisp at whole-number scales.
func markdownBodyScaleFor(r zenui.Renderer, screenW int) int {
	pad := r.LineHeight(1)
	if markdownPanelWidth(r, markdownBodyScaleBase) <= screenW-2*pad {
		return markdownBodyScaleBase
	}
	return 1
}

type markdownModal struct {
	title  string
	lines  []string
	scroll int // first visible line index

	// autoHeight, if true, sizes the panel to fit every line without
	// scrolling (About: short, fixed-length content, where a
	// full-height panel would leave most of it empty) rather than
	// always filling most of the screen (Help: long enough that it
	// always needs scrolling regardless, and screen-filling gives the
	// most room for that).
	autoHeight bool

	panel    zenui.Rect
	body     zenui.Rect // text area (inside the panel, below the title)
	closeBx  zenui.Rect
	visible  int // lines that fit in the body (set during layout)
	total    int
	bodyLH   int // body line height at the effective scale (set during layout)
	effScale int // effective body text scale for the current screen width

	// scrollbar drag state
	track      zenui.Rect
	thumb      zenui.Rect
	dragging   bool
	dragOffset int // pointer offset within the thumb at grab time
}

func newMarkdownModal(title, content string, autoHeight bool) *markdownModal {
	return &markdownModal{
		title:      title,
		lines:      strings.Split(strings.TrimRight(content, "\n"), "\n"),
		autoHeight: autoHeight,
	}
}

// markdown line classification for the lightweight markdown this reader
// understands.
type markdownLineKind int

const (
	markdownBody     markdownLineKind = iota // ordinary prose
	markdownIndented                         // indented (shortcut tables etc.)
	markdownH1                               // "# " heading
	markdownH2                               // "## " heading
)

// classifyMarkdownLine classifies a line and returns the text to draw
// (with any markdown heading marker stripped).
func classifyMarkdownLine(line string) (markdownLineKind, string) {
	switch {
	case strings.HasPrefix(line, "## "):
		return markdownH2, line[3:]
	case strings.HasPrefix(line, "# "):
		return markdownH1, line[2:]
	case strings.HasPrefix(line, "  "):
		return markdownIndented, line
	default:
		return markdownBody, line
	}
}

func (h *markdownModal) layout(r zenui.Renderer, screenW, screenH int) {
	lh := r.LineHeight(1)
	pad := lh
	h.effScale = markdownBodyScaleFor(r, screenW)

	pw := markdownPanelWidth(r, h.effScale)
	if minW := 320; pw < minW {
		pw = minW
	}
	if pw > screenW-2*pad {
		pw = screenW - 2*pad
	}

	titleH := r.LineHeight(2) + 8
	h.bodyLH = r.LineHeight(h.effScale)
	if h.bodyLH < 1 {
		h.bodyLH = 1
	}
	h.total = len(h.lines)

	var ph int
	if h.autoHeight {
		// Estimate the content's own height (every line at the body's
		// line height) and size the panel to fit it exactly, rather
		// than always filling most of the screen -- correct as an
		// estimate rather than a guaranteed exact fit, since actual
		// text rendering could in principle wrap or round slightly
		// differently, but line-count * line-height is what every
		// other size in this layout is already derived from too.
		contentH := h.total * h.bodyLH
		ph = contentH + 2*pad + titleH
		if ph > screenH-2*pad {
			ph = screenH - 2*pad // still cap to the screen, in case content somehow doesn't fit
		}
	} else {
		ph = screenH - 4*pad
	}
	if ph < lh*10 {
		ph = lh * 10
	}
	px := (screenW - pw) / 2
	py := (screenH - ph) / 2
	h.panel = zenui.Rect{X: px, Y: py, W: pw, H: ph}

	cb := r.LineHeight(2)
	h.closeBx = zenui.Rect{X: px + pw - pad - cb, Y: py + pad, W: cb, H: cb}

	h.body = zenui.Rect{
		X: px + pad,
		Y: py + pad + titleH,
		W: pw - 2*pad - 12, // leave room for the scrollbar on the right
		H: ph - 2*pad - titleH,
	}
	h.visible = h.body.H / h.bodyLH
	if h.visible < 1 {
		h.visible = 1
	}
	h.clampScroll()
}

func (h *markdownModal) clampScroll() {
	maxScroll := h.total - h.visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if h.scroll > maxScroll {
		h.scroll = maxScroll
	}
	if h.scroll < 0 {
		h.scroll = 0
	}
}

// update handles scrolling and dismissal. Returns false when the modal
// should close.
func (h *markdownModal) update(in zenui.Input) bool {
	for _, k := range in.Keys {
		switch k {
		case zenui.KeyEscape:
			return false
		case zenui.KeyUp:
			h.scroll--
		case zenui.KeyDown:
			h.scroll++
		case zenui.KeyPageUp:
			h.scroll -= h.visible
		case zenui.KeyPageDown:
			h.scroll += h.visible
		}
	}
	// Home/End are not in the shared zenui key set; read them directly.
	if rl.IsKeyPressed(rl.KeyHome) {
		h.scroll = 0
	}
	if rl.IsKeyPressed(rl.KeyEnd) {
		h.scroll = h.total
	}
	if in.WheelY != 0 {
		h.scroll -= int(in.WheelY) * 3
	}

	if in.MousePressed && h.thumb.W > 0 {
		if h.thumb.Contains(in.MouseX, in.MouseY) {
			h.dragging = true
			h.dragOffset = in.MouseY - h.thumb.Y
		} else if h.track.Contains(in.MouseX, in.MouseY) {
			if in.MouseY < h.thumb.Y {
				h.scroll -= h.visible
			} else {
				h.scroll += h.visible
			}
		}
	}
	if h.dragging {
		if !in.MouseDown {
			h.dragging = false
		} else if h.track.H > h.thumb.H {
			rel := float32(in.MouseY-h.dragOffset-h.track.Y) / float32(h.track.H-h.thumb.H)
			if rel < 0 {
				rel = 0
			}
			if rel > 1 {
				rel = 1
			}
			h.scroll = int(rel * float32(h.total-h.visible))
		}
	}
	h.clampScroll()

	if in.MousePressed {
		if h.closeBx.Contains(in.MouseX, in.MouseY) {
			return false
		}
		if !h.panel.Contains(in.MouseX, in.MouseY) {
			return false
		}
	}
	return true
}

// scaleAlpha returns c with its alpha multiplied by factor -- used to
// adjust the modal's own panel/backdrop opacity relative to the active
// theme's own values, scoped to this modal (Help/About) specifically
// rather than changing theme.Panel/theme.Backdrop themselves, which
// every other dialog (Dialog, Modal, MessageBox) also draws with.
func scaleAlpha(c zenui.Colour, factor float64) zenui.Colour {
	c.A = uint8(float64(c.A) * factor)
	return c
}

const (
	// modalPanelOpacity is the client area's own opacity, independent
	// of whatever alpha theme.Panel itself carries (always 0xff/fully
	// opaque across every current theme) -- text and widgets drawn on
	// top afterwards, at their own full opacity, still render fully
	// solid regardless, since each draw call blends independently
	// against whatever's already there.
	modalPanelOpacity = 0.85
	// modalBackdropDarkening scales the backdrop's own alpha down by
	// 20%, relative to whatever value the active theme already uses
	// (which varies by theme) -- less alpha means less of the dimming
	// colour shows through, i.e. a less dark overlay.
	modalBackdropDarkening = 0.8
)

func (h *markdownModal) draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	h.layout(r, screenW, screenH)
	lh := r.LineHeight(1)
	pad := lh

	r.FillRect(zenui.Rect{X: 0, Y: 0, W: screenW, H: screenH}, scaleAlpha(theme.Backdrop, modalBackdropDarkening))
	r.FillRect(h.panel, scaleAlpha(theme.Panel, modalPanelOpacity))
	r.StrokeRect(h.panel, theme.Border, 1)

	r.DrawText(h.title, h.panel.X+pad, h.panel.Y+pad, 2, theme.Text)

	mx, my := int(rl.GetMouseX()), int(rl.GetMouseY())
	cbg := theme.Button
	if h.closeBx.Contains(mx, my) {
		cbg = theme.ButtonHot
	}
	r.FillRect(h.closeBx, cbg)
	r.StrokeRect(h.closeBx, theme.Border, 1)
	r.DrawText("x", h.closeBx.X+(h.closeBx.W-r.TextWidth("x", 1))/2,
		h.closeBx.Y+(h.closeBx.H-lh)/2, 1, theme.ButtonText)

	r.Clip(h.body)
	y := h.body.Y
	end := h.scroll + h.visible
	if end > h.total {
		end = h.total
	}
	for i := h.scroll; i < end; i++ {
		kind, text := classifyMarkdownLine(h.lines[i])
		switch kind {
		case markdownH1, markdownH2:
			// Accent colour, bold via a one-pixel horizontal double-strike.
			r.DrawText(text, h.body.X, y, h.effScale, theme.DirText)
			r.DrawText(text, h.body.X+1, y, h.effScale, theme.DirText)
		case markdownIndented:
			r.DrawText(text, h.body.X, y, h.effScale, theme.DimText)
		default:
			r.DrawText(text, h.body.X, y, h.effScale, theme.Text)
		}
		y += h.bodyLH
	}
	r.ClipEnd()

	if h.total > h.visible {
		h.track = zenui.Rect{X: h.body.X + h.body.W + 6, Y: h.body.Y, W: 8, H: h.body.H}
		r.FillRect(h.track, theme.Button)
		frac := float32(h.visible) / float32(h.total)
		thumbH := int(float32(h.track.H) * frac)
		if thumbH < 16 {
			thumbH = 16
		}
		var pos float32
		if h.total-h.visible > 0 {
			pos = float32(h.scroll) / float32(h.total-h.visible)
		}
		thumbY := h.track.Y + int(float32(h.track.H-thumbH)*pos)
		h.thumb = zenui.Rect{X: h.track.X, Y: thumbY, W: h.track.W, H: thumbH}
		thumbCol := theme.DirText
		if h.dragging {
			thumbCol = theme.Text
		}
		r.FillRect(h.thumb, thumbCol)
	} else {
		h.track = zenui.Rect{}
		h.thumb = zenui.Rect{}
	}
}
