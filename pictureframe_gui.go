//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"image"
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The frame around a centred picture: the gradient and the picture's shadow. The arithmetic and
// what they are made of are pictureframe.go; endPictureCanvas (display_fit_gui.go) calls this.

// frameShadow is the shadow's texture, which lives on the graphics card from the first frame
// that wants it to the window's closing.
type frameShadow struct {
	tex   rl.Texture2D
	ready bool
}

// shadowTexture makes the shadow's texture, once: black, with the opacity of shadowAlphas as its
// alpha channel, filtered so that the stretched middle and the corners meet without a seam.
func (dm *DisplayManager) shadowTexture() rl.Texture2D {
	s := &dm.fit.shadow
	if s.ready {
		return s.tex
	}
	alphas := shadowAlphas()
	pic := image.NewRGBA(image.Rect(0, 0, shadowSide, shadowSide)) // black: red, green and blue stay 0
	for i, a := range alphas {
		pic.Pix[4*i+3] = a
	}
	img := rl.NewImageFromImage(pic)
	s.tex = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.SetTextureFilter(s.tex, rl.FilterBilinear)
	rl.SetTextureWrap(s.tex, rl.WrapClamp)
	s.ready = true
	return s.tex
}

// unloadFrameShadow gives the shadow's texture back, before the window closes.
func (dm *DisplayManager) unloadFrameShadow() {
	if s := &dm.fit.shadow; s.ready {
		rl.UnloadTexture(s.tex)
		s.ready = false
	}
}

// drawFrame paints what is around the picture r in a window of winW by winH under a bar of top
// units: black in the bar's own band, the gradient from grey to darker grey below it, the five
// lines across that, and the picture's shadow on that. It is drawn before the picture, which covers the middle of the shadow.
func (dm *DisplayManager) drawFrame(r pictureRect, top, winW, winH int) {
	rl.ClearBackground(rl.Black)
	if winH <= top {
		return
	}
	rl.DrawRectangleGradientV(0, int32(top), int32(winW), int32(winH-top),
		color.RGBA{R: frameTopGrey, G: frameTopGrey, B: frameTopGrey, A: 255},
		color.RGBA{R: frameBottomGrey, G: frameBottomGrey, B: frameBottomGrey, A: 255})
	for _, y := range frameLineYs(top, winH) { // each a groove: dark, and the light under it
		rl.DrawRectangle(0, int32(y), int32(winW), 1, color.RGBA{A: frameLineDarkAlpha})
		rl.DrawRectangle(0, int32(y+1), int32(winW), 1, color.RGBA{R: 255, G: 255, B: 255, A: frameLineAlpha})
	}
	if !shadowFits(r.W, r.H) {
		return
	}
	drawBoxShadow(dm.shadowTexture(), rl.Rectangle{X: r.X, Y: r.Y, Width: r.W, Height: r.H})
}

// drawBoxShadow draws the shadow of a box from the shadow texture: the box's size grown by the
// reach of the blur on every side, and moved down. A box with room for the nine-patch (shadowFits)
// has it; a smaller one, a short submenu, has the whole texture squeezed into the size it would
// have, which is a little tighter and the same in kind. The popup menus use it too
// (popupshadow_gui.go).
func drawBoxShadow(tex rl.Texture2D, r rl.Rectangle) { drawBoxShadowAt(tex, r, 1) }

// drawBoxShadowAt is drawBoxShadow with the shadow's strength scaled: 1 is the texture as it is, 0.7 is
// 30% less dark.
func drawBoxShadowAt(tex rl.Texture2D, r rl.Rectangle, strength float32) {
	tint := shadowTint(strength)
	const reach = float32(shadowReach)
	dest := rl.Rectangle{X: r.X - reach, Y: r.Y - reach + shadowDropY, Width: r.Width + 2*reach, Height: r.Height + 2*reach}
	if !shadowFits(r.Width, r.Height) {
		rl.DrawTexturePro(tex, rl.Rectangle{Width: shadowSide, Height: shadowSide}, dest, rl.Vector2{}, 0, tint)
		return
	}
	info := rl.NPatchInfo{
		Source: rl.Rectangle{Width: shadowSide, Height: shadowSide},
		Left:   2 * shadowReach, Top: 2 * shadowReach, Right: 2 * shadowReach, Bottom: 2 * shadowReach,
		Layout: rl.NPatchNinePatch,
	}
	rl.DrawTextureNPatch(tex, info, dest, rl.Vector2{}, 0, tint)
}
