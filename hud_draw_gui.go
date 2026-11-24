//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"time"

	"github.com/ha1tch/zenui"
)

// Transport-button sprites, in the dock icon's own ink/mask-hex notation
// (dockicon_gui.go): one string a row, four hex digits of ink, a slash, four
// of mask; bit 15 is the leftmost pixel. Monochromatic, drawn as horizontal
// runs by the same parseIcon/iconRuns the dock tile uses.
var transportSprites = map[string][16]string{
	"play": {
		"0000/0000", "0000/0000", "0600/0600", "0700/0700", "0780/0780", "07c0/07c0",
		"07e0/07e0", "07f0/07f0", "07f0/07f0", "07e0/07e0", "07c0/07c0", "0780/0780",
		"0700/0700", "0600/0600", "0000/0000", "0000/0000",
	},
	// rewind is two triangles pointing left, for the tape deck (tapedeck_gui.go); the
	// debugger's row (transportOrder) has no rewind.
	"rewind": {
		"0000/0000", "0000/0000", "0204/0204", "060c/060c", "0e1c/0e1c", "1e3c/1e3c",
		"3e7c/3e7c", "7efc/7efc", "7efc/7efc", "3e7c/3e7c", "1e3c/1e3c", "0e1c/0e1c",
		"060c/060c", "0204/0204", "0000/0000", "0000/0000",
	},
	"pause": {
		"0000/0000", "0000/0000", "0e38/0e38", "0e38/0e38", "0e38/0e38", "0e38/0e38",
		"0e38/0e38", "0e38/0e38", "0e38/0e38", "0e38/0e38", "0e38/0e38", "0e38/0e38",
		"0e38/0e38", "0e38/0e38", "0000/0000", "0000/0000",
	},
	"step": {
		"0000/0000", "0000/0000", "1818/1818", "1c18/1c18", "1e18/1e18", "1f18/1f18",
		"1f98/1f98", "1fd8/1fd8", "1fd8/1fd8", "1f98/1f98", "1f18/1f18", "1e18/1e18",
		"1c18/1c18", "1818/1818", "0000/0000", "0000/0000",
	},
	"run": {
		"0000/0000", "0000/0000", "1860/1860", "1c70/1c70", "1e78/1e78", "1f7c/1f7c",
		"1ffe/1ffe", "1fff/1fff", "1fff/1fff", "1ffe/1ffe", "1f7c/1f7c", "1e78/1e78",
		"1c70/1c70", "1860/1860", "0000/0000", "0000/0000",
	},
	"stop": {
		"0000/0000", "0000/0000", "0ff0/0ff0", "0ff0/0ff0", "0ff0/0ff0", "0ff0/0ff0",
		"0ff0/0ff0", "0ff0/0ff0", "0ff0/0ff0", "0ff0/0ff0", "0ff0/0ff0", "0ff0/0ff0",
		"0ff0/0ff0", "0000/0000", "0000/0000", "0000/0000",
	},
}

// transportOrder is the left-to-right button order.
var transportOrder = []string{"play", "pause", "step", "run", "stop"}

// parseTransportSprites parses every transport sprite once, reusing
// dockicon_gui.go's parseIcon. A sprite that fails to parse is simply absent
// (drawn as a labelled gap); none should, since they are literals here.
func parseTransportSprites() map[string][16]iconRow {
	out := make(map[string][16]iconRow, len(transportSprites))
	for name, rows := range transportSprites {
		if icon, err := parseIcon(rows); err == nil {
			out[name] = icon
		}
	}
	return out
}

// drawSprite draws one transport sprite at (x,y) with integer scale s in the
// given ink colour, reusing dockicon_gui.go's iconRuns. Only ink pixels are
// drawn; paper is left transparent so the panel shows through (the dock
// icon's own convention for an on-panel glyph).
func (h *debugHUD) drawSprite(icon [16]iconRow, x, y, s int, ink zenui.Colour) {
	x0, y0, x1, y1 := iconBounds(icon)
	_ = x1
	_ = y1
	for _, run := range iconRuns(icon) {
		if !run.ink {
			continue
		}
		h.renderer.FillRect(zenui.Rect{
			X: x + (run.x-x0)*s,
			Y: y + (run.y-y0)*s,
			W: run.n * s,
			H: s,
		}, ink)
	}
}

// Draw renders the HUD over the current frame: a dim backdrop, a panel with
// the transport button row, the scrollback pane and the input line. screenW
// and screenH are the current framebuffer size in device pixels. It returns
// the button rectangles so the host can hit-test clicks (kept simple: the HUD
// also accepts the keyboard commands the buttons stand for).
func (h *debugHUD) Draw(screenW, screenH int) {
	if !h.Visible() {
		return
	}
	th := h.theme
	anim := h.anim // 0 hidden .. 1 shown

	// The dim over the running screen fades in with the slide.
	h.renderer.FillRect(zenui.Rect{X: 0, Y: 0, W: screenW, H: screenH}, zenui.Colour{R: 0, G: 0, B: 0, A: uint8(90 * anim)})

	// Panel across the bottom, its height the draggable fraction, slid down by
	// (1-anim) of its own height so it eases up from the bottom edge on open and
	// back down on close.
	panelH := int(float32(screenH) * h.heightFrac)
	slide := int(float32(panelH) * (1 - anim))
	panelY := screenH - panelH + slide
	h.renderer.FillRect(zenui.Rect{X: 0, Y: panelY, W: screenW, H: panelH}, th.Panel)

	// Resize gripper on the top edge: a short horizontal handle that, as it
	// appears, grows from the centre out to its full width while it fades in --
	// gripGlow drives width, colour and alpha together, so showing it eases
	// open from the middle and hiding it eases back to a point. It brightens
	// from the border colour toward the accent as it appears.
	if h.gripGlow > 0 {
		g := easeInOutCubic(h.gripGlow) // ease the grow/fade, not linear

		// A thin 1px accent line along the panel's top edge, growing from the
		// centre to the full width as it fades in -- the same animation as the
		// gripper, a full-width underline beneath the handle.
		lineW := int(float32(screenW) * g) // 0 -> full screen width, centred
		if lineW > 0 {
			lineCol := zenui.Colour{R: th.DirText.R, G: th.DirText.G, B: th.DirText.B, A: uint8(255 * g)}
			h.renderer.FillRect(zenui.Rect{X: (screenW - lineW) / 2, Y: panelY, W: lineW, H: 1}, lineCol)
		}

		// The gripper handle: a short bar that grows from the centre out to its
		// full width while it fades in. gripGlow drives width, colour and alpha
		// together, so showing eases open from the middle and hiding eases back
		// to a point. It brightens from the border colour toward the accent.
		fullW := screenW / 6
		gw := int(float32(fullW) * g) // grows from 0 at the centre
		gh := 4
		gx := (screenW - gw) / 2 // stays centred as it widens
		gy := panelY - gh/2
		col := zenui.Colour{
			R: uint8(float32(th.Border.R) + (float32(th.DirText.R)-float32(th.Border.R))*g),
			G: uint8(float32(th.Border.G) + (float32(th.DirText.G)-float32(th.Border.G))*g),
			B: uint8(float32(th.Border.B) + (float32(th.DirText.B)-float32(th.Border.B))*g),
			A: uint8(255 * g),
		}
		if gw > 0 {
			h.renderer.FillRect(zenui.Rect{X: gx, Y: gy, W: gw, H: gh}, col)
		}
	}

	pad := 12
	scale := 2 // text scale
	lineH := h.renderer.LineHeight(scale)

	// Transport button row along the panel top.
	btnScale := 2
	btnPx := 16 * btnScale
	btnGap := 10
	bx := pad
	by := panelY + pad
	for _, name := range transportOrder {
		icon, ok := h.sprites[name]
		if ok {
			// A subtle button cell behind the glyph.
			h.renderer.FillRect(zenui.Rect{X: bx - 3, Y: by - 3, W: btnPx + 6, H: btnPx + 6}, th.Button)
			h.drawSprite(icon, bx, by, btnScale, th.ButtonText)
		}
		bx += btnPx + btnGap
	}

	// Caption + status to the right of the buttons: which machine is being
	// debugged, then its run state. The caption matters in -multi, where the
	// HUD follows whichever machine is in front.
	state := "running"
	if h.active {
		state = "stopped"
	}
	cap := h.caption
	if cap == "" {
		cap = "machine"
	}
	// The caption is drawn in the theme's DirText accent (the warm highlight
	// used for directory entries) -- a mild contrast against the panel that
	// marks "which machine this is" without the strong blue of SelFill. The
	// Sinclair BDF face has no '[' ']' glyphs, so the caption carries no
	// brackets.
	statusY := by + (btnPx-lineH)/2
	capText := " " + cap + "  "
	capW := h.renderer.TextWidth(capText, scale)
	h.renderer.DrawText(capText, bx+btnGap, statusY, scale, th.DirText)
	h.renderer.DrawText(state, bx+btnGap+capW, statusY, scale, th.DimText)

	// Scrollback pane below the buttons.
	textTop := by + btnPx + pad
	avail := (panelY + panelH) - textTop - lineH - pad
	maxLines := avail / lineH
	if maxLines > hudVisibleLines {
		maxLines = hudVisibleLines
	}
	start := 0
	if len(h.cur.scroll) > maxLines {
		start = len(h.cur.scroll) - maxLines
	}
	y := textTop
	for _, ln := range h.cur.scroll[start:] {
		h.renderer.DrawText(ln, pad, y, scale, th.Text)
		y += lineH
	}

	// Input line at the panel bottom, with a blinking caret.
	inputY := panelY + panelH - lineH - pad/2
	caret := ""
	if time.Now().UnixMilli()/500%2 == 0 { // ~1Hz blink
		caret = "_"
	}
	h.renderer.DrawText("> "+string(h.input)+caret, pad, inputY, scale, th.Text)
}
