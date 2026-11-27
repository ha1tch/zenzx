//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// The two buttons at the ends of the menu bar, and the toolbar one of them brings in.
//
// At the left is the toolbar button: four small slanted bars, "////", drawn like the Spectrum logo's
// teeth, in one colour like the other menu icons, in a rounded rectangle. It swaps the menu's labels for a toolbar of the actions
// used most, sliding the labels up out of the strip and the toolbar up into it (and the other
// way back), while the bars turn. At the right is the burger button, a thin circle with three
// thin lines in it, which opens the Help menu the logo used to open: the logo is only a picture
// now, moved left to make room for it. Both are drawn here, over the strip that zenui draws;
// zenui keeps room for them (MenuBar.SetInsets) and does the sliding of the labels
// (MenuBar.SetLabelsOffset).

// toolbarSlideSeconds is how long the swap between the labels and the toolbar takes.
const toolbarSlideSeconds = 0.22

// toggleWidthFor and helpWidthFor are the widths of the two buttons in a strip h high.
func toggleWidthFor(h int) int { return h - 2 } // as high as the button is: a square
func helpWidthFor(h int) int   { return h }

// barInsetsFor is the room zenui leaves at the strip's two ends in a strip h high: at the left
// the toolbar button and a pad either side of it, so that the labels start where the toolbar
// does; at the right the Help button and the room between it and the logo. (zenui puts its own
// 8 pixels between the labels' end, the logo and the window's edge, and counts from here.)
func barInsetsFor(h int) (left, right int) {
	pad := logoButtonPadFor(h)
	left = max(0, pad+toggleWidthFor(h)+pad-12)
	right = max(0, pad+helpWidthFor(h)+pad*3/2-8)
	return left, right
}

// toggleButton is the toolbar button's rectangle in the bar's own coordinates: at the left end, a
// pad in, and as tall as the strip less its edges. It sits a pixel higher than the Help button
// (against the top, where that one has its pixel of margin), as it was asked to.
func (b *appMenuBar) toggleButton() zenui.Rect {
	h := b.stripH()
	return zenui.Rect{X: logoButtonPadFor(h), Y: 0, W: toggleWidthFor(h), H: h - 2}
}

// helpButton is the Help button's rectangle: at the right end, a pad in.
func (b *appMenuBar) helpButton(screenW int) zenui.Rect {
	h := b.stripH()
	w := helpWidthFor(h)
	return zenui.Rect{X: screenW - logoButtonPadFor(h) - w, Y: 1, W: w, H: h - 2}
}

// toggleHotZone is where a press opens or closes the toolbar: the button, run out to the
// window's edge and the strip's full height, so that the corner is easy to hit.
func (b *appMenuBar) toggleHotZone() zenui.Rect {
	r := b.toggleButton()
	return zenui.Rect{X: 0, Y: 0, W: r.X + r.W + logoButtonPadFor(b.stripH())/2, H: b.stripH()}
}

// helpHotZone is where a press opens the Help menu: the same, at the right.
func (b *appMenuBar) helpHotZone(screenW int) zenui.Rect {
	r := b.helpButton(screenW)
	x := r.X - logoButtonPadFor(b.stripH())/2
	return zenui.Rect{X: x, Y: 0, W: screenW - x, H: b.stripH()}
}

// toolbarItem is one button of the toolbar: a picture (every real button has one, and a hover hint
// that names it), or a word, and what it does.
type toolbarItem struct {
	icon      string // the sprite's name (transportIcons, toolbarIcons); empty for a word
	label     string
	tip       string // what a hover says for a button that is only a picture
	disabled  bool
	run       func(zx *ZenZX)
	gapBefore bool          // a space and a thin line before it
	more      []toolbarItem // for the overflow button: the buttons that did not fit, which it opens as a menu
}

// toolbarItems are the buttons as the machine is now: the transport rows of the Machine menu
// (enabled as they are there), and the windows that manage tapes, disks, SD cards, snapshots and
// screenshots, each a picture with its name as a hover hint. Disks and SD Cards are there only on a machine that has them: the toolbar is a
// short list of what can be used, not a list with dead buttons in it.
func (b *appMenuBar) toolbarItems(zx *ZenZX) []toolbarItem {
	off := zx.stopped || zx.stopping
	machine := b.actions[barMachine]
	transport := func(row int) func(*ZenZX) {
		return func(zx *ZenZX) {
			if row < len(machine) {
				machine[row](zx)
			}
		}
	}
	items := []toolbarItem{
		{icon: "recycle", tip: "Reset", disabled: off, run: transport(1)},
		{icon: "play", tip: "Play", disabled: zx.stopping || !(zx.paused || zx.stopped), run: transport(2)},
		{icon: "pause", tip: "Pause", disabled: off || zx.paused, run: transport(3)},
		{icon: "stop", tip: "Stop", disabled: off, run: transport(4)},
		{icon: "tape", tip: "Tapes", gapBefore: true, run: func(zx *ZenZX) { b.openTapes(zx) }},
	}
	if zx.io.hasFDC && zx.io.fdc != nil {
		items = append(items, toolbarItem{icon: "disk", tip: "Disks", run: func(zx *ZenZX) { b.openDisks(zx) }})
	}
	if usesSDCards(zx) {
		items = append(items, toolbarItem{icon: "card", tip: "SD Cards", run: func(zx *ZenZX) { b.openCards(zx) }})
	}
	return append(items,
		toolbarItem{icon: "snapshot", tip: "Snapshots", run: func(zx *ZenZX) { b.openSnapshots(zx) }},
		toolbarItem{icon: "screenshot", tip: "Screenshots", run: func(zx *ZenZX) { b.openScreenshots(zx) }})
}

// toolbarSlot is a toolbar button with the rectangle it has, in the bar's coordinates and as
// tall as the strip.
type toolbarSlot struct {
	toolbarItem
	rect zenui.Rect
}

// toolbarLayout puts the buttons of b.toolbarCur side by side from where the labels start. When
// they do not all fit before the logo or the Help button, the ones that fit are kept, and a "more"
// button after them stands for the rest: it opens them as a menu (openOverflow), so that nothing
// the toolbar has is out of reach in a narrow window.
func (b *appMenuBar) toolbarLayout(screenW int) []toolbarSlot {
	h, k := b.stripH(), b.scaleOfBar()
	pad := logoButtonPadFor(h)
	tg := b.toggleButton()
	start := tg.X + tg.W + pad
	end := b.helpButton(screenW).X - pad
	if d := b.widget.DecorationX(); d > 0 && d < screenW && d-pad < end {
		end = d - pad
	}
	cell := h - 2
	textPad := max(6, 3*k)
	var all []toolbarSlot
	x := start
	for _, it := range b.toolbarCur {
		if it.gapBefore {
			x += 2 * pad
		}
		w := cell
		if it.label != "" {
			w = b.renderer.TextWidth(it.label, k) + 2*textPad
		}
		all = append(all, toolbarSlot{toolbarItem: it, rect: zenui.Rect{X: x, Y: 0, W: w, H: h}})
		x += w
	}
	n := len(all)
	if n == 0 || x <= end {
		return all
	}
	for n > 0 && all[n-1].rect.X+all[n-1].rect.W+cell > end {
		n--
	}
	ox := start
	if n > 0 {
		ox = all[n-1].rect.X + all[n-1].rect.W
	}
	if ox+cell > end {
		return nil
	}
	var rest []toolbarItem
	for _, s := range all[n:] {
		rest = append(rest, s.toolbarItem)
	}
	more := toolbarSlot{toolbarItem: toolbarItem{icon: "more", tip: "More", more: rest}, rect: zenui.Rect{X: ox, Y: 0, W: cell, H: h}}
	return append(all[:n:n], more)
}

// openOverflow opens the buttons that did not fit as a menu under the "more" button, each row as
// its button is: the same name, enabled the same, doing the same.
func (b *appMenuBar) openOverflow(s toolbarSlot) {
	b.closeMenus()
	var spec menuSpec
	for _, it := range s.more {
		name := it.label
		if name == "" {
			name = it.tip
		}
		e := menuRow(name, it.run)
		e.disabled = it.disabled
		spec = append(spec, e)
	}
	b.logoSpec = spec
	b.overflowOpen = true
	b.logoMenu = zenui.NewMenu(zenui.MenuConfig{
		Items:  spec.items(),
		Anchor: zenui.Rect{X: s.rect.X, Y: 0, W: s.rect.W, H: b.stripH()},
		Scale:  b.scale,
	})
}

// toolbarRest is where the slide is at rest: all the way in when the toolbar is on, as a session
// that starts with it on does, and all the way out when it is not.
func toolbarRest(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// toolbarSettled is true while the toolbar is fully in, and so is run by the pointer.
func (b *appMenuBar) toolbarSettled() bool { return b.toolbarOn && b.toolbarT >= 1 }

// toolbarMoving is true while the labels and the toolbar are sliding.
func (b *appMenuBar) toolbarMoving() bool {
	return (b.toolbarOn && b.toolbarT < 1) || (!b.toolbarOn && b.toolbarT > 0)
}

// stepToolbar moves the slide on by dt seconds, towards the toolbar when it is on and towards the
// labels when it is not.
func (b *appMenuBar) stepToolbar(dt float32) {
	step := dt / toolbarSlideSeconds
	if b.toolbarOn {
		b.toolbarT = min(1, b.toolbarT+step)
	} else {
		b.toolbarT = max(0, b.toolbarT-step)
	}
}

// toggleToolbar swaps the labels and the toolbar, and keeps the choice in the settings file. A
// dropdown that is open is closed first.
func (b *appMenuBar) toggleToolbar(zx *ZenZX) {
	b.closeMenus()
	b.toolbarOn = !b.toolbarOn
	b.saveSettings(zx)
}

// slideOffset is how far the labels are drawn above where they rest, and the toolbar below
// where it rests: the strip's height at the end of the slide, none before it.
func (b *appMenuBar) slideOffset() int {
	return int(float32(b.stripH()) * easeInOutCubic(b.toolbarT))
}

// buttonsLive says the buttons at the ends of the bar and the toolbar can be used now: the bar is
// shown and at rest, and no window covers it.
func (b *appMenuBar) buttonsLive() bool {
	return b.state == barShown && !b.modalOpen() && !b.toolbarMoving()
}

// barButtonsInput acts on a press on one of the two buttons or on the toolbar, and says that it
// did, which is when the bar's labels are not asked.
func (b *appMenuBar) barButtonsInput(zx *ZenZX, in zenui.Input, screenW int) bool {
	if !in.MousePressed || !b.buttonsLive() {
		return false
	}
	switch {
	case b.toggleHotZone().Contains(in.MouseX, in.MouseY):
		b.toggleToolbar(zx)
		return true
	case b.helpHotZone(screenW).Contains(in.MouseX, in.MouseY):
		b.openLogoMenu(screenW)
		return true
	case b.toolbarSettled():
		for _, s := range b.toolbarLayout(screenW) {
			if s.rect.Contains(in.MouseX, in.MouseY) {
				switch {
				case s.more != nil:
					b.openOverflow(s)
				case !s.disabled && s.run != nil:
					s.run(zx)
				}
				return true
			}
		}
	}
	return false
}

// barButtonCursorWanted reports that the pointer is on something the bar's buttons or toolbar
// would act on, where it is a hand.
func (b *appMenuBar) barButtonCursorWanted(screenW, mouseX, mouseY int) bool {
	if !b.buttonsLive() {
		return false
	}
	if b.toggleHotZone().Contains(mouseX, mouseY) || b.helpHotZone(screenW).Contains(mouseX, mouseY) {
		return true
	}
	if b.toolbarSettled() {
		for _, s := range b.toolbarLayout(screenW) {
			if !s.disabled && s.rect.Contains(mouseX, mouseY) {
				return true
			}
		}
	}
	return false
}

// toggleButtonState is how the toolbar button looks: pressed in while the toolbar is the bar,
// raised under the pointer.
func (b *appMenuBar) toggleButtonState(mouseX, mouseY int) logoButtonState {
	if b.toolbarOn {
		return logoPressed
	}
	if b.buttonsLive() && b.toggleHotZone().Contains(mouseX, mouseY) {
		return logoHover
	}
	return logoIdle
}

// helpButtonState is how the Help button looks: pressed in while its menu is open.
func (b *appMenuBar) helpButtonState(screenW, mouseX, mouseY int) logoButtonState {
	if b.logoMenu != nil && !b.overflowOpen {
		return logoPressed
	}
	if b.buttonsLive() && b.helpHotZone(screenW).Contains(mouseX, mouseY) {
		return logoHover
	}
	return logoIdle
}

// roundedInset is how far in from the left a rounded rectangle h high with corners of radius rad
// starts on its row i, counted from the top.
func roundedInset(rad, i, h int) int {
	d := min(i, h-1-i)
	if rad < 1 || d >= rad {
		return 0
	}
	dy := float64(rad-d) - 0.5
	return rad - int(math.Floor(math.Sqrt(float64(rad*rad)-dy*dy)+0.5))
}

// fillRounded fills rc, with its corners rounded to radius rad, one row at a time (a pixel is
// painted once, so a translucent colour is not doubled where rows would meet).
func fillRounded(r zenui.Renderer, rc zenui.Rect, rad int, c zenui.Colour) {
	rad = min(rad, rc.W/2, rc.H/2)
	for i := 0; i < rc.H; i++ {
		in := roundedInset(rad, i, rc.H)
		r.FillRect(zenui.Rect{X: rc.X + in, Y: rc.Y + i, W: rc.W - 2*in, H: 1}, c)
	}
}

// strokeRounded draws the edge of a rounded rectangle, t pixels thick.
func strokeRounded(r zenui.Renderer, rc zenui.Rect, rad, t int, c zenui.Colour) {
	rad = min(rad, rc.W/2, rc.H/2)
	for i := 0; i < rc.H; i++ {
		o := roundedInset(rad, i, rc.H)
		if i < t || i >= rc.H-t {
			r.FillRect(zenui.Rect{X: rc.X + o, Y: rc.Y + i, W: rc.W - 2*o, H: 1}, c)
			continue
		}
		n := t + roundedInset(max(0, rad-t), i-t, rc.H-2*t)
		w := max(t, n-o)
		r.FillRect(zenui.Rect{X: rc.X + o, Y: rc.Y + i, W: w, H: 1}, c)
		r.FillRect(zenui.Rect{X: rc.X + rc.W - o - w, Y: rc.Y + i, W: w, H: 1}, c)
	}
}

// withAlpha is c with its opacity a.
func withAlpha(c zenui.Colour, a uint8) zenui.Colour { c.A = a; return c }

// drawBarButton draws the face of the toolbar button, rc in the bar's coordinates and barY
// pixels from where it rests (the bar slides in and out): a rounded rectangle with a thin light
// edge and no face at rest, a faint face under the pointer, and a darker face while it is pressed in.
// ink is the text's colour, so that the button is as light on a light theme as on a dark one.
func drawBarButton(r zenui.Renderer, rc zenui.Rect, barY int, state logoButtonState, ink zenui.Colour) {
	rc.Y += barY
	rad := max(2, rc.H/5)
	t := logoBevelThickness(rc.H + 2)
	edge := withAlpha(ink, 90)
	switch state {
	case logoHover:
		fillRounded(r, rc, rad, withAlpha(ink, 40))
		edge = withAlpha(ink, 170)
	case logoPressed:
		fillRounded(r, rc, rad, zenui.Colour{R: 0, G: 0, B: 0, A: 80})
		edge = withAlpha(ink, 130)
	}
	strokeRounded(r, rc, rad, t, edge)
}

// iconBox is the room for the picture in a button: its rectangle less a margin all round.
func iconBox(rc zenui.Rect) zenui.Rect {
	m := max(2, rc.H/10)
	return zenui.Rect{X: rc.X + m, Y: rc.Y + m, W: rc.W - 2*m, H: rc.H - 2*m}
}

// toggleSpinTurn is how far the toolbar button's picture turns, in half-turns, while the bar swaps
// between the labels and the toolbar: a half-turn (180 degrees) each way.
const toggleSpinTurn = 1.0

// slashGlyph is the toolbar button's picture before it is turned: four bars leaning like "////",
// each drawn as the Spectrum logo's teeth are (zenui's drawZSPLogo): a staircase of three blocks,
// each s wide and 2s high (the logo's 1:2 block), the lowest at the left and each next one a block
// higher and a block to the right. Where the logo's teeth touch, these are 3s apart in a row,
// so the picture is a small mark and the bars can be told apart. It is 6s high and 15s wide.
type slashGlyph struct{ s int }

func (g slashGlyph) height() int { return 6 * g.s }
func (g slashGlyph) width() int  { return 15 * g.s }

// slashGlyphIn is the largest glyph that fits box, centred in it by drawSlashBars, and false when
// the box is too small for even the smallest, 15 by 6.
func slashGlyphIn(box zenui.Rect) (slashGlyph, bool) {
	for s := box.H / 6; s >= 1; s-- {
		if g := (slashGlyph{s: s}); g.width() <= box.W {
			return g, true
		}
	}
	return slashGlyph{}, false
}

// has says whether the glyph has a pixel at (x, y), counted from its top left.
func (g slashGlyph) has(x, y int) bool {
	if y < 0 || y >= g.height() || x < 0 || x >= g.width() {
		return false
	}
	level := 2 - y/(2*g.s) // the top block of a bar is its third, the bottom one its first
	d := x - level*g.s
	if d < 0 {
		return false
	}
	return d%(4*g.s) < g.s && d/(4*g.s) < 4
}

// slashSamples is how many samples across a pixel, each way, a turned picture is smoothed with.
const slashSamples = 3

// drawSlashBars draws the toolbar button's picture in box in colour c (a menu icon: one colour,
// the text's), turned clockwise by turn half-turns about the centre of box. Upright (turn 0, or any
// whole number of full turns) it is drawn as it is. Turned, each pixel of box is as opaque
// as the part of it that the glyph covers (it is sampled slashSamples times each way), and the glyph
// shrinks as far as it needs to for what it turns through to stay within box.
func drawSlashBars(r zenui.Renderer, box zenui.Rect, turn float64, c zenui.Colour) {
	g, ok := slashGlyphIn(box)
	if !ok {
		return
	}
	x0, y0 := box.X+(box.W-g.width())/2, box.Y+(box.H-g.height())/2
	theta := turn * math.Pi
	sin, cos := math.Sin(theta), math.Cos(theta)
	if math.Abs(sin) < 1e-9 && cos > 0 {
		for y := 0; y < g.height(); y++ {
			for x := 0; x < g.width(); x++ {
				if g.has(x, y) {
					r.FillRect(zenui.Rect{X: x0 + x, Y: y0 + y, W: 1, H: 1}, c)
				}
			}
		}
		return
	}
	gw, gh := float64(g.width()), float64(g.height())
	boundW := gw*math.Abs(cos) + gh*math.Abs(sin) // the turned glyph's bounding box
	boundH := gw*math.Abs(sin) + gh*math.Abs(cos)
	scale := math.Min(1, math.Min(float64(box.W)/boundW, float64(box.H)/boundH))
	cx, cy := float64(x0)+gw/2, float64(y0)+gh/2
	for py := box.Y; py < box.Y+box.H; py++ {
		for px := box.X; px < box.X+box.W; px++ {
			n := 0
			for j := 0; j < slashSamples; j++ {
				for i := 0; i < slashSamples; i++ {
					dx := float64(px) + (float64(i)+0.5)/slashSamples - cx
					dy := float64(py) + (float64(j)+0.5)/slashSamples - cy
					// the point of the glyph that lands here: the opposite turn, then the scale undone
					sx := (cos*dx + sin*dy) / scale
					sy := (-sin*dx + cos*dy) / scale
					if g.has(int(math.Floor(sx+gw/2)), int(math.Floor(sy+gh/2))) {
						n++
					}
				}
			}
			if n > 0 {
				col := c
				col.A = uint8(int(c.A) * n / (slashSamples * slashSamples))
				r.FillRect(zenui.Rect{X: px, Y: py, W: 1, H: 1}, col)
			}
		}
	}
}

// circleSpan is the first column and the width of the row i (counted from the top) of a circle d
// across, taken at the middle of the row: its pixels are those whose centres are inside.
func circleSpan(d, i int) (x, w int) {
	rad := float64(d) / 2
	dy := float64(i) + 0.5 - rad
	half := math.Sqrt(math.Max(0, rad*rad-dy*dy))
	x = int(math.Floor(rad - half + 0.5))
	return x, max(0, d-2*x)
}

// fillCircle fills the circle that is the largest in rc and centred in it, a row at a time (each
// pixel once, so that a translucent colour is even).
func fillCircle(r zenui.Renderer, rc zenui.Rect, c zenui.Colour) {
	d := min(rc.W, rc.H)
	x0, y0 := rc.X+(rc.W-d)/2, rc.Y+(rc.H-d)/2
	for i := 0; i < d; i++ {
		x, w := circleSpan(d, i)
		if w > 0 {
			r.FillRect(zenui.Rect{X: x0 + x, Y: y0 + i, W: w, H: 1}, c)
		}
	}
}

// strokeCircle draws the edge of that circle, one pixel thick.
func strokeCircle(r zenui.Renderer, rc zenui.Rect, c zenui.Colour) {
	d := min(rc.W, rc.H)
	x0, y0 := rc.X+(rc.W-d)/2, rc.Y+(rc.H-d)/2
	for i := 0; i < d; i++ {
		x, w := circleSpan(d, i)
		if w <= 0 {
			continue
		}
		// the row's own ends, and the whole row where the row above or below is shorter (the
		// top and the bottom of the circle), so that the edge has no gaps
		lo, hi := w, w
		if i > 0 {
			_, wu := circleSpan(d, i-1)
			lo = (w - wu) / 2
		}
		if i < d-1 {
			_, wd := circleSpan(d, i+1)
			hi = (w - wd) / 2
		}
		n := max(1, max(lo, hi))
		if i == 0 || i == d-1 || 2*n >= w {
			r.FillRect(zenui.Rect{X: x0 + x, Y: y0 + i, W: w, H: 1}, c)
			continue
		}
		r.FillRect(zenui.Rect{X: x0 + x, Y: y0 + i, W: n, H: 1}, c)
		r.FillRect(zenui.Rect{X: x0 + x + w - n, Y: y0 + i, W: n, H: 1}, c)
	}
}

// drawBarCircle draws the face of the Help button: a circle with a thin light edge and, at rest, no
// face; a faint one under the pointer; a darker one while its menu is open. rc is in the bar's
// coordinates and barY pixels from where it rests (the bar slides). ink is the text's colour.
func drawBarCircle(r zenui.Renderer, rc zenui.Rect, barY int, state logoButtonState, ink zenui.Colour) {
	rc.Y += barY
	rc = circleOf(rc)
	edge := withAlpha(ink, 90)
	switch state {
	case logoHover:
		fillCircle(r, rc, withAlpha(ink, 40))
		edge = withAlpha(ink, 170)
	case logoPressed:
		fillCircle(r, rc, zenui.Colour{R: 0, G: 0, B: 0, A: 80})
		edge = withAlpha(ink, 130)
	}
	strokeCircle(r, rc, edge)
}

// circleOf is the square that holds the Help button's circle: the button's own, two pixels in on
// every side, and made odd across if it is even, so that the circle has a middle row and a middle
// column of pixels and what is drawn in it (three one-pixel lines) can be exactly centred.
func circleOf(rc zenui.Rect) zenui.Rect {
	d := max(1, min(rc.W, rc.H)-4)
	if d%2 == 0 {
		d++
	}
	d = min(d, max(1, min(rc.W, rc.H)))
	return zenui.Rect{X: rc.X + (rc.W-d)/2, Y: rc.Y + (rc.H-d)/2, W: d, H: d}
}

// circleIconBox is the room for a picture inside the circle of rc: a box that sits inside it, wider
// than high, with an odd width and height so that it is centred in the circle to the pixel. The
// burger's lines are spread over the whole height, so that is what sets the gaps between them.
func circleIconBox(rc zenui.Rect) zenui.Rect {
	rc = circleOf(rc)
	d := min(rc.W, rc.H)
	w, h := (d/2)|1, (d*5/16)|1+2 // two more rows than the lines need: a pixel more between them, the top one higher and the bottom one lower
	return zenui.Rect{X: rc.X + (rc.W-w)/2, Y: rc.Y + (rc.H-h)/2, W: w, H: h}
}

// drawBurger draws the Help button's picture in box: three thin horizontal lines, the
// middle one halfway.
func drawBurger(r zenui.Renderer, box zenui.Rect, c zenui.Colour) {
	t := max(1, box.H/8) | 1 // odd, so that the middle line can be centred in an odd box
	if box.H < 5*t {
		return
	}
	for i := 0; i < 3; i++ {
		y := box.Y + i*(box.H-t)/2
		r.FillRect(zenui.Rect{X: box.X, Y: y, W: box.W, H: t}, c)
	}
}

// drawToolbar draws the toolbar sliding in below the labels, clipped to the strip. barY is where
// the strip is (it slides too).
func (b *appMenuBar) drawToolbar(screenW, barY int) {
	if b.toolbarT <= 0 {
		return
	}
	r := b.renderer
	h, k := b.stripH(), b.scaleOfBar()
	dy := h - b.slideOffset()
	mx, my := int(rl.GetMouseX()), int(rl.GetMouseY())
	hot := b.toolbarSettled() && b.buttonsLive()
	r.Clip(zenui.Rect{X: 0, Y: barY, W: screenW, H: h - 1})
	for _, s := range b.toolbarLayout(screenW) {
		y := barY + dy
		col := b.theme.SideText
		face := s.rect
		face.Y, face.H = y+1, h-2
		if s.disabled {
			col = b.theme.DimText
		} else if hot && s.rect.Contains(mx, my) {
			fillRounded(r, face, max(2, face.H/5), b.theme.BarSelFill)
			col = b.theme.BarSelText
		}
		if s.gapBefore {
			pad := logoButtonPadFor(h)
			r.FillRect(zenui.Rect{X: s.rect.X - pad, Y: y + h/4, W: 1, H: h - 2*(h/4)}, zenui.Colour{R: 255, G: 255, B: 255, A: 70})
		}
		if s.more != nil {
			drawDots(r, zenui.Rect{X: s.rect.X, Y: y, W: s.rect.W, H: h}, col)
		} else if s.icon != "" {
			transportIcon(s.icon)(r, zenui.Rect{X: s.rect.X, Y: y, W: s.rect.W, H: h}, col)
		} else {
			r.DrawText(s.label, s.rect.X+(s.rect.W-r.TextWidth(s.label, k))/2, y+(h-r.LineHeight(k))/2, k, col)
		}
	}
	r.ClipEnd()
}

// drawDots draws the "more" button's picture in cell: three square dots in a row, centred.
func drawDots(r zenui.Renderer, cell zenui.Rect, c zenui.Colour) {
	d := max(2, cell.H/12)
	gap := d
	w := 3*d + 2*gap
	x, y := cell.X+(cell.W-w)/2, cell.Y+(cell.H-d)/2
	for i := 0; i < 3; i++ {
		r.FillRect(zenui.Rect{X: x + i*(d+gap), Y: y, W: d, H: d}, c)
	}
}

// toolbarDimming is how much of its opacity the //// has left once the toolbar is in: the button is
// dimmer, as a pressed-in one is, and the turn ends where it began.
const toolbarDimming = 0.55

// tipDelaySeconds is how long the pointer rests on a button before it says what it is.
const tipDelaySeconds = 0.5

// tipTarget is what the pointer at (mx, my) is over that has a hover hint: a key that stays the same
// while it is over the same thing, the words, and the thing's rectangle in the bar's coordinates.
// The key is empty when there is none, or when a menu is open.
func (b *appMenuBar) tipTarget(screenW, mx, my int) (key, text string, rc zenui.Rect) {
	if !b.buttonsLive() || b.logoMenu != nil || b.widget.Active() {
		return "", "", zenui.Rect{}
	}
	switch {
	case b.toggleHotZone().Contains(mx, my):
		text = "Toolbar"
		if b.toolbarOn {
			text = "Menus"
		}
		return "toggle:" + text, text, b.toggleButton()
	case b.helpHotZone(screenW).Contains(mx, my):
		return "help", "Help", b.helpButton(screenW)
	case b.toolbarSettled():
		for _, s := range b.toolbarLayout(screenW) {
			if s.tip != "" && s.rect.Contains(mx, my) {
				return "item:" + s.tip, s.tip, s.rect
			}
		}
	}
	return "", "", zenui.Rect{}
}

// drawTip draws the hover hint of what the pointer has rested on for tipDelaySeconds, under the
// bar, in the theme's panel colours.
func (b *appMenuBar) drawTip(screenW, barY, mx, my int, now float64) {
	key, text, rc := b.tipTarget(screenW, mx, my)
	if key != b.tipKey {
		b.tipKey, b.tipAt = key, now
	}
	if key == "" || now-b.tipAt < tipDelaySeconds {
		return
	}
	r, k := b.renderer, b.scaleOfBar()
	padX, padY := max(4, 2*k), max(2, k)
	w, h := r.TextWidth(text, k)+2*padX, r.LineHeight(k)+2*padY
	x := min(max(2, rc.X+(rc.W-w)/2), max(2, screenW-w-2))
	y := barY + b.stripH() + 3
	r.FillRect(zenui.Rect{X: x, Y: y, W: w, H: h}, b.theme.Border)
	r.FillRect(zenui.Rect{X: x + 1, Y: y + 1, W: w - 2, H: h - 2}, b.theme.Panel)
	r.DrawText(text, x+padX, y+padY, k, b.theme.Text)
}

// paintBarButtons draws what the bar has over the strip zenui drew: the toolbar, then the two
// buttons with their pictures.
func (b *appMenuBar) paintBarButtons(screenW, barY int) {
	b.drawToolbar(screenW, barY)
	mx, my := int(rl.GetMouseX()), int(rl.GetMouseY())
	tg := b.toggleButton()
	drawBarButton(b.renderer, tg, barY, b.toggleButtonState(mx, my), b.theme.SideText)
	pic := iconBox(tg)
	pic.Y += barY
	ease := float64(easeInOutCubic(b.toolbarT))
	ink := b.theme.SideText
	ink.A = uint8(float64(ink.A) * (1 - toolbarDimming*ease))
	drawSlashBars(b.renderer, pic, toggleSpinTurn*ease, ink)
	hp := b.helpButton(screenW)
	drawBarCircle(b.renderer, hp, barY, b.helpButtonState(screenW, mx, my), b.theme.SideText)
	pic = circleIconBox(hp)
	pic.Y += barY
	drawBurger(b.renderer, pic, b.theme.SideText)
	b.drawTip(screenW, barY, mx, my, rl.GetTime())
}

// logoTick lets the time pass for the logo's colours on a frame that does not hand the menu bar its
// input, as the toolbar does not (the labels have none while it is out), nor a dialog over the bar.
// The pointer is put far away and nothing is pressed, so that nothing but the clock moves; a
// dropdown that is open is left alone.
func (b *appMenuBar) logoTick() {
	if b.widget == nil || b.widget.Active() {
		return
	}
	b.widget.Update(zenui.Input{MouseX: -1 << 20, MouseY: -1 << 20, DeltaTime: rl.GetFrameTime()})
}
