//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "github.com/ha1tch/zenui"

// The bar's text magnification (menubarscale.go has the rule: the width and the height of the
// window) and how it is drawn.
//
// zenui.MenuBar draws its labels at scale 1 and its dropdowns at its own scale, and has no say
// for the labels. So the bar is drawn through barScaleRenderer, which turns the labels' scale 1
// into the bar's scale while the strip itself is being drawn, and leaves everything else as it
// is: the dropdowns, which MenuBar draws last, are never touched by it. Where zenui takes a
// scale for the labels this can go (T-87).

// What goes with the strip when it is taller than its usual menuStripHeight: the logo and the
// rainbow are made from the strip's height (zenui does that, and the rainbow's two sizes are the
// package variables that drawBar sets), and the Help button's room and edges follow it here.

// rainbowBandFor is the width of each of the rainbow's bands in a strip h high: 16 pixels in
// the usual one of 24, and in the same proportion in another, so the rainbow keeps its shape.
func rainbowBandFor(h int) int { return max(1, 16*h/menuStripHeight) }

// logoButtonPadFor is the room between the logo and the Help button's edge in a strip h high:
// logoButtonPad in the usual one, more in a taller one.
func logoButtonPadFor(h int) int { return max(logoButtonPad, logoButtonPad*h/menuStripHeight) }

// logoBevelThickness is the thickness of the Help button's edges in a strip h high: one pixel in
// the usual one, and in a taller one the nearest whole number of pixels in proportion.
func logoBevelThickness(h int) int { return max(1, (h+menuStripHeight/2)/menuStripHeight) }

// barScaleWidth is the width the bar's labels take at scale k: each label and one blank space.
func (b *appMenuBar) barScaleWidth(k int) int {
	w := 0
	space := b.renderer.TextWidth(" ", k)
	for i := 0; ; i++ {
		label := b.widget.LabelFor(i)
		if label == "" {
			return w
		}
		w += b.renderer.TextWidth(label, k) + space
	}
}

// barScale is the bar's text scale for a window screenW wide and screenH high.
func (b *appMenuBar) barScale(screenW, screenH int) int {
	return fitBarScale(screenW, screenH, b.barScaleWidth, b.stripFor)
}

// scaleOfBar is the scale the bar is drawn at now: 1 until it has been fitted to a window.
func (b *appMenuBar) scaleOfBar() int {
	if b.barK < 1 {
		return 1
	}
	return b.barK
}

// stripFor is the height of the strip that holds text at scale k: the usual height, or more
// where the text with its room above and below (barPadFor) takes more than that.
func (b *appMenuBar) stripFor(k int) int {
	line := b.renderer.LineHeight(k)
	return max(menuStripHeight, line+2*barPadFor(line))
}

// stripH is the strip's height at the bar's scale.
func (b *appMenuBar) stripH() int { return b.stripFor(b.scaleOfBar()) }

// reserveH is the height of the strip that a fixed bar reserves at the top of the window: the
// strip at the scale that the display's own window, not the window as it now is, would give
// it. It must not follow the window's size: the window's size follows what is reserved, and a
// window the user has enlarged would otherwise change the bar's scale, then the reservation,
// then be put back to the display's size. The bar is drawn at the scale of the window it is in
// (drawBar); where that is a taller strip than this the extra overlies the frame.
func (b *appMenuBar) reserveH(dm *DisplayManager) int {
	return b.stripFor(b.barScale(dm.targetWidth, dm.targetHeight-dm.reservedTopHeight))
}

// barY is where the strip's top is: 0 when shown, above the window by its height when hidden,
// and between while it slides.
func (b *appMenuBar) barY() int {
	return int(float32(-b.stripH()) * (1 - b.progress))
}

// drawBar fits the bar's scale to the window and draws the strip and the open dropdown, and
// returns where the strip was put.
func (b *appMenuBar) drawBar(screenW, screenH int) int {
	b.barK = b.barScale(screenW, screenH)
	barY := b.barY()
	var r zenui.Renderer = b.renderer
	if k := b.scaleOfBar(); k > 1 {
		r = &barScaleRenderer{Renderer: b.renderer, k: k, inBar: true}
	}
	// The rainbow of the Spectrum theme is as tall as the strip and keeps its shape; the logo of
	// the other themes is made from the strip's height by zenui itself.
	zenui.RainbowScanlines, zenui.RainbowBandWidth = b.stripH(), rainbowBandFor(b.stripH())
	b.widget.Draw(r, screenW, screenH, barY, b.stripH(), b.theme)
	return barY
}

// barScaleRenderer draws the strip's labels at scale k where zenui asks for scale 1. A MenuBar
// draws the strip first (its fill, the labels, the decorations) and ends it with a one pixel
// line across the bottom of the window's width; what it draws after that is the open dropdown.
// inBar is true until that line, and while it is, scale 1 is k.
type barScaleRenderer struct {
	zenui.Renderer
	k     int
	inBar bool
}

func (r *barScaleRenderer) scale(s int) int {
	if r.inBar && s == 1 {
		return r.k
	}
	return s
}

func (r *barScaleRenderer) TextWidth(s string, scale int) int {
	return r.Renderer.TextWidth(s, r.scale(scale))
}

func (r *barScaleRenderer) LineHeight(scale int) int { return r.Renderer.LineHeight(r.scale(scale)) }

func (r *barScaleRenderer) DrawText(s string, x, y, scale int, c zenui.Colour) {
	r.Renderer.DrawText(s, x, y, r.scale(scale), c)
}

func (r *barScaleRenderer) StrokeRect(rc zenui.Rect, c zenui.Colour, thickness int) {
	if r.inBar && rc.X == 0 && rc.H == 1 {
		r.inBar = false // the strip's bottom edge: the strip is done, the dropdown is next
	}
	r.Renderer.StrokeRect(rc, c, thickness)
}
