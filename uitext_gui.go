//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui/zenuiraylib"
)

// The text the display manager puts over the picture (the FPS counter and the border-change
// count) is drawn in the font the person chose for the interface, with the BDF text renderer
// the menus use, and never with raylib's own font. The front end says which renderer that is
// with uiText; it is asked every time, because choosing another font in the View menu makes
// a new one.
var uiText func() *zenuiraylib.BDFText

// uiTextScale is the size of that text in pixels per font pixel: 16 pixels high in an 8-pixel face.
const uiTextScale = 2

// drawUIText draws s at (x, y) in the interface font. It draws nothing before the front end
// has said which font that is.
func drawUIText(s string, x, y, scale int, c rl.Color) {
	if uiText == nil {
		return
	}
	if t := uiText(); t != nil {
		t.Draw(s, x, y, scale, c)
	}
}

// drawFPS draws the frame rate at the bottom right of the window.
func (dm *DisplayManager) drawFPS() {
	dm.fps = rl.GetFPS()
	if uiText == nil {
		return
	}
	t := uiText()
	if t == nil {
		return
	}
	s := fmt.Sprintf("FPS: %d", dm.fps)
	x := rl.GetScreenWidth() - t.Measure(s, uiTextScale) - 10
	y := rl.GetScreenHeight() - t.CellH()*uiTextScale - 10
	t.Draw(s, x, y, uiTextScale, rl.Red)
}
