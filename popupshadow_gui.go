//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
)

// The popup menus' drop shadow: the menus that open from the bar (and their submenus) and the
// logo menu cast the shadow that the picture casts in its frame, from the same texture
// (pictureframe_gui.go). The bar itself casts none, and the shadow is clipped to below it, so it
// does not fall on the strip.
//
// zenui draws a menu's panel with one FillRect of the theme's Panel colour, first, and before
// anything else of that menu. popupShadowRenderer sits between zenui and the real renderer and
// draws the shadow just before such a FillRect, so the shadow lies under the panel and over
// whatever is there, the parent menu included for a submenu. zenui is not changed.

// menuShadowStrength is how strong the popup menus' shadow is next to the picture's and the windows'
// (1): 30% less dark. Only the menus' own; every other shadow is drawn as it was.
const menuShadowStrength = 0.7

// shadowTint is the tint that draws a shadow texture at a strength from 0 to 1: white, as clear as
// the strength leaves it.
func shadowTint(strength float32) rl.Color {
	return rl.Color{R: 255, G: 255, B: 255, A: uint8(min(max(strength, 0), 1)*255 + 0.5)}
}

// withPopupShadow is r with the popups' shadow added, when the window has given the bar one to draw.
// inBar says the bar's own strip is drawn through it first; top is where the shadow may begin
// otherwise (the bottom of the strip).
func (b *appMenuBar) withPopupShadow(r zenui.Renderer, inBar bool, top int) zenui.Renderer {
	if b.popupShadow == nil {
		return r
	}
	return &popupShadowRenderer{Renderer: r, panel: b.theme.Panel, inBar: inBar, top: top, shadow: b.popupShadow}
}

// popupShadowRenderer draws a shadow under every popup panel that is drawn through it.
type popupShadowRenderer struct {
	zenui.Renderer
	panel zenui.Colour // the theme's Panel: what a popup's panel is filled with
	// inBar is true while the bar's own strip is being drawn, which ends with the one pixel line
	// across the bottom of it (as barScaleRenderer finds it); nothing is a popup before that.
	inBar bool
	// top is the first row of the window the shadow may fall on: the bottom of the strip.
	top int
	// shadow draws the shadow of a box. It is the texture's, set by the window.
	shadow func(box rl.Rectangle)
}

func (r *popupShadowRenderer) FillRect(rc zenui.Rect, c zenui.Colour) {
	if !r.inBar && c == r.panel && r.shadow != nil {
		r.Renderer.Clip(zenui.Rect{X: 0, Y: r.top, W: 1 << 14, H: 1 << 14})
		r.shadow(rl.Rectangle{X: float32(rc.X), Y: float32(rc.Y), Width: float32(rc.W), Height: float32(rc.H)})
		r.Renderer.ClipEnd()
	}
	r.Renderer.FillRect(rc, c)
}

func (r *popupShadowRenderer) StrokeRect(rc zenui.Rect, c zenui.Colour, thickness int) {
	if r.inBar && rc.X == 0 && rc.H == 1 {
		r.inBar = false // the strip's bottom edge: the strip is done, the dropdown is next
		r.top = rc.Y + 1
	}
	r.Renderer.StrokeRect(rc, c, thickness)
}
