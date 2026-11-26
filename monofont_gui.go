//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"

	"github.com/ha1tch/zenzx/pkg/fonts"
)

// monoRenderer is the renderer for text that has to line up in columns, such as a hex dump: in
// the font the person chose for the program when that is monospaced, else in the monospaced face
// whose cell is the closest in size to it (fonts.NearestMonospaced). It is made when first wanted,
// and again if the font is changed. Where there is no window to hold the glyph textures, as in a
// test, it is the program's own renderer.
func (b *appMenuBar) monoRenderer() zenui.Renderer {
	want := fonts.NearestMonospaced(b.fontName)
	if want == b.fontName || !rl.IsWindowReady() {
		return b.renderer
	}
	if b.monoText == nil || b.monoName != want {
		face, err := fonts.Load(want)
		if err != nil {
			return b.renderer
		}
		if b.monoText != nil {
			b.monoText.Unload()
		}
		b.monoText, b.monoName = zenuiraylib.NewBDFText(face), want
	}
	return zenuiraylib.Renderer{Text: b.monoText}
}
