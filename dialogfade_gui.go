//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// dialogFader makes a window that has closed fade away and shrink a little, all on the host's side
// of zenui: zenui draws a window or does not, and has no notion of one on its way out.
//
// While a window is open the bar tells the fader how to draw it (watch). The frame after it has
// closed the bar asks the fader to capture (the window, still a value the fader holds, is drawn
// one last time into a texture of the window's size, with its backdrop held back, since the backdrop
// covers the whole screen and would shrink with the window, showing its edges); and each frame
// until dialogFadeAt says it is gone the bar has it paint: the backdrop alone, over the whole
// screen at the opacity the curve gives, and the texture over it, scaled about the middle of the
// window at that opacity. Nothing in the picture has any life of its own: the window beneath is already
// the live one, so no input waits for the fade.
type dialogFader struct {
	last        func(r zenui.Renderer) // how the window last shown is drawn; nil once captured
	hold        bool                   // a window the bar does not draw is open: nothing is captured yet
	ghost       rl.RenderTexture2D
	have        bool
	w, h        int
	backdrop    zenui.Colour
	hasBackdrop bool
	start       time.Time
	now         func() time.Time // time.Now, unless a test says otherwise
}

func (f *dialogFader) clock() time.Time {
	if f.now != nil {
		return f.now()
	}
	return time.Now()
}

// watch says that a window is open and how it is drawn. A fade still going is over: the new window
// is in front of it.
func (f *dialogFader) watch(draw func(r zenui.Renderer)) {
	f.stop()
	f.last = draw
}

// stop takes a fade away, and frees its texture.
func (f *dialogFader) stop() {
	if f.have {
		rl.UnloadRenderTexture(f.ghost)
		f.have = false
	}
}

// backdropCatcher is a renderer that keeps the window's backdrop, the first rectangle over the whole
// screen, to itself instead of drawing it, and draws everything else.
type backdropCatcher struct {
	zenui.Renderer
	w, h   int
	colour zenui.Colour
	caught bool
}

func (c *backdropCatcher) FillRect(r zenui.Rect, col zenui.Colour) {
	if !c.caught && r.X <= 0 && r.Y <= 0 && r.W >= c.w && r.H >= c.h {
		c.colour, c.caught = col, true
		return
	}
	c.Renderer.FillRect(r, col)
}

// The factors of the blend the window is drawn with into a texture that starts out clear, so that
// the colour and the opacity both come out as they would over anything: the colour premultiplied,
// the opacity added up as it is by the "over" rule (GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA, GL_ONE,
// GL_FUNC_ADD).
const (
	glOne, glSrcAlpha, glOneMinusSrcAlpha, glFuncAdd = 1, 0x0302, 0x0303, 0x8006
)

// capture draws the window that has just closed, if one has, into a texture, and starts its fade.
func (f *dialogFader) capture(r zenui.Renderer, w, h int) {
	if f.last == nil || f.hold {
		return
	}
	draw := f.last
	f.last = nil
	f.stop()
	f.ghost = rl.LoadRenderTexture(int32(w), int32(h))
	f.w, f.h = w, h
	catch := &backdropCatcher{Renderer: r, w: w, h: h}
	rl.BeginTextureMode(f.ghost)
	rl.ClearBackground(rl.Blank)
	rl.SetBlendFactorsSeparate(glSrcAlpha, glOneMinusSrcAlpha, glOne, glOneMinusSrcAlpha, glFuncAdd, glFuncAdd)
	rl.BeginBlendMode(rl.BlendCustomSeparate)
	draw(catch)
	rl.EndBlendMode()
	rl.EndTextureMode()
	f.backdrop, f.hasBackdrop = catch.colour, catch.caught
	f.start = f.clock()
	f.have = true
}

// paint draws the fade as it is now, over what is drawn so far, and ends it when it is done.
func (f *dialogFader) paint(r zenui.Renderer) {
	if !f.have {
		return
	}
	alpha, scale, done := dialogFadeAt(f.clock().Sub(f.start))
	if done {
		f.stop()
		return
	}
	if f.hasBackdrop {
		r.FillRect(zenui.Rect{W: f.w, H: f.h}, zenui.ScaleAlpha(f.backdrop, float64(alpha)))
	}
	w, h := float32(f.w), float32(f.h)
	// the texture is upside down, as a render texture is; the colour is premultiplied by the opacity
	a := uint8(alpha*255 + 0.5)
	rl.BeginBlendMode(rl.BlendAlphaPremultiply)
	rl.DrawTexturePro(f.ghost.Texture,
		rl.NewRectangle(0, 0, w, -h),
		rl.NewRectangle(w*(1-scale)/2, h*(1-scale)/2, w*scale, h*scale),
		rl.NewVector2(0, 0), 0, rl.NewColor(a, a, a, a))
	rl.EndBlendMode()
}

// modalDrawer is how the window that is open, if one is, is drawn: the one that comes first of
// those that cover the whole screen with a backdrop of their own. nil when none is open.
func (b *appMenuBar) modalDrawer(w, h int) func(r zenui.Renderer) {
	th := b.theme
	switch {
	case b.prefs != nil:
		p := b.prefs
		return func(r zenui.Renderer) { p.Draw(r, w, h, th) }
	case b.cards != nil:
		c := b.cards
		return func(r zenui.Renderer) { c.Draw(r, w, h, th) }
	case b.replaceAsk != nil:
		q := b.replaceAsk
		return func(r zenui.Renderer) { q.draw(r, w, h, th) }
	case b.cardsAsk != nil && b.diskDialog == nil:
		q := b.cardsAsk
		return func(r zenui.Renderer) { q.draw(r, w, h, th) }
	case b.snaps != nil:
		s := b.snaps
		return func(r zenui.Renderer) { s.Draw(r, w, h, th) }
	case b.shots != nil:
		s := b.shots
		return func(r zenui.Renderer) { s.Draw(r, w, h, th) }
	case b.tapes != nil:
		s := b.tapes
		return func(r zenui.Renderer) { s.Draw(r, w, h, th) }
	case b.bin != nil:
		x := b.bin
		return func(r zenui.Renderer) { x.Draw(r, w, h, th) }
	case b.disks != nil:
		d := b.disks
		return func(r zenui.Renderer) { d.Draw(r, w, h, th) }
	case b.nextSetup != nil:
		n := b.nextSetup
		return func(r zenui.Renderer) { n.draw(r, w, h, th) }
	case b.activeModal != nil:
		m := b.activeModal
		return func(r zenui.Renderer) { m.Draw(r, w, h, th) }
	case b.diskDialog != nil:
		d := b.diskDialog
		return func(r zenui.Renderer) { d.Draw(r, w, h, th) }
	case b.exportAsk != nil:
		q := b.exportAsk
		return func(r zenui.Renderer) { q.draw(r, w, h, th) }
	}
	return nil
}

// watchDropChoice is what the program does each frame, before the bar is drawn, for the drop-choice
// dialog, which is drawn by the program over everything and not by the bar: while it is open it is the
// window the fader watches, and holds the capture back; the frame after it closes the bar's Draw
// captures it and the fade begins like any other.
func (b *appMenuBar) watchDropChoice(d *dropChoice, w, h int) {
	b.fade.hold = d.Active()
	if b.fade.hold {
		th := b.theme
		b.fade.watch(func(r zenui.Renderer) { d.paint(r, w, h, th) })
	}
}
