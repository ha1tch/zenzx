// Package zenuiraylib is the raylib-specific half of driving zenui widgets:
// a bitmap-font text renderer (BDFText) built on pkg/bdf, and a Renderer/
// Input pair that satisfies zenui's renderer-agnostic interfaces through it.
// zenui itself never imports raylib -- everything that touches rl.* calls
// lives here instead, so a headless build never needs to link against it.
package zenuiraylib

import (
	"image"
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenzx/pkg/bdf"
	"github.com/ha1tch/zenzx/pkg/zenui"
)

// BDFText draws text through a BDF bitmap font via raylib textures, rather
// than raylib's own font system. Each glyph is rasterised once by pkg/bdf
// into a white-on-transparent RGBA cell, uploaded as its own texture, and
// cached by rune. Colour comes entirely from Draw's tint parameter at
// draw time (white multiplied by any tint equals that tint unchanged) --
// deliberately not baked in at cache time, so the same cached textures
// serve every colour a caller ever draws with, including switching
// between themes with very different text colours (a theme with black
// text would, if baked in as the cache colour, multiply every future
// tint against black and render everything black regardless of the
// tint requested).
type BDFText struct {
	font  *bdf.Font
	cellW int
	cellH int
	cache map[rune]rl.Texture2D
}

// NewBDFText builds a text renderer for font. Must be called after
// rl.InitWindow -- glyph textures need a live GL context to upload into.
func NewBDFText(font *bdf.Font) *BDFText {
	return &BDFText{
		font:  font,
		cellW: font.CellWidth,
		cellH: font.CellHeight,
		cache: make(map[rune]rl.Texture2D),
	}
}

// CellW and CellH are the font's cell dimensions in source pixels.
func (t *BDFText) CellW() int { return t.cellW }
func (t *BDFText) CellH() int { return t.cellH }

// white is the fixed cache-time ink every glyph is rasterised with --
// see BDFText's own doc comment for why this must never vary per
// instance or per glyph.
var white = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

func (t *BDFText) glyph(r rune) rl.Texture2D {
	if tex, ok := t.cache[r]; ok {
		return tex
	}
	img, ok := t.font.GlyphImage(r, white)
	if !ok {
		img = image.NewRGBA(image.Rect(0, 0, t.cellW, t.cellH))
	}
	tex := texFromRGBA(img)
	t.cache[r] = tex
	return tex
}

// Draw renders s with its top-left at (x,y), scaled by an integer factor,
// tinted by tint. Returns the x position just past the last glyph.
func (t *BDFText) Draw(s string, x, y, scale int, tint rl.Color) int {
	if scale < 1 {
		scale = 1
	}
	cx := x
	for _, r := range s {
		tex := t.glyph(r)
		dst := rl.NewRectangle(float32(cx), float32(y), float32(t.cellW*scale), float32(t.cellH*scale))
		src := rl.NewRectangle(0, 0, float32(t.cellW), float32(t.cellH))
		rl.DrawTexturePro(tex, src, dst, rl.NewVector2(0, 0), 0, tint)
		cx += t.cellW * scale
	}
	return cx
}

// Measure returns the pixel width of s at the given scale.
func (t *BDFText) Measure(s string, scale int) int {
	if scale < 1 {
		scale = 1
	}
	n := 0
	for range s {
		n++
	}
	return n * t.cellW * scale
}

// Unload frees every cached glyph texture. Call before rl.CloseWindow.
func (t *BDFText) Unload() {
	for _, tex := range t.cache {
		rl.UnloadTexture(tex)
	}
	t.cache = map[rune]rl.Texture2D{}
}

func texFromRGBA(img *image.RGBA) rl.Texture2D {
	rlImg := rl.NewImageFromImage(img)
	tex := rl.LoadTextureFromImage(rlImg)
	rl.UnloadImage(rlImg)
	return tex
}

// Renderer satisfies zenui.Renderer by drawing through a BDFText.
type Renderer struct {
	Text *BDFText
}

func Colour(c zenui.Colour) rl.Color { return rl.NewColor(c.R, c.G, c.B, c.A) }

func (r Renderer) FillRect(rc zenui.Rect, c zenui.Colour) {
	rl.DrawRectangle(int32(rc.X), int32(rc.Y), int32(rc.W), int32(rc.H), Colour(c))
}

func (r Renderer) StrokeRect(rc zenui.Rect, c zenui.Colour, thickness int) {
	rl.DrawRectangleLinesEx(
		rl.NewRectangle(float32(rc.X), float32(rc.Y), float32(rc.W), float32(rc.H)),
		float32(thickness), Colour(c))
}

func (r Renderer) FillRectGradientV(rc zenui.Rect, top, bottom zenui.Colour) {
	rl.DrawRectangleGradientV(int32(rc.X), int32(rc.Y), int32(rc.W), int32(rc.H), Colour(top), Colour(bottom))
}

func (r Renderer) FillRectGradientHMultiply(rc zenui.Rect, left, right zenui.Colour) {
	rl.BeginBlendMode(rl.BlendMultiplied)
	rl.DrawRectangleGradientH(int32(rc.X), int32(rc.Y), int32(rc.W), int32(rc.H), Colour(left), Colour(right))
	rl.EndBlendMode()
}

func (r Renderer) DrawLine(x1, y1, x2, y2, thickness int, c zenui.Colour) {
	if thickness <= 1 {
		rl.DrawLine(int32(x1), int32(y1), int32(x2), int32(y2), Colour(c))
		return
	}
	rl.DrawLineEx(rl.NewVector2(float32(x1), float32(y1)), rl.NewVector2(float32(x2), float32(y2)), float32(thickness), Colour(c))
}

func (r Renderer) DrawText(s string, x, y, scale int, c zenui.Colour) {
	r.Text.Draw(s, x, y, scale, Colour(c))
}

func (r Renderer) TextWidth(s string, scale int) int { return r.Text.Measure(s, scale) }
func (r Renderer) LineHeight(scale int) int          { return r.Text.CellH() * scale }

func (r Renderer) Clip(rc zenui.Rect) {
	rl.BeginScissorMode(int32(rc.X), int32(rc.Y), int32(rc.W), int32(rc.H))
}
func (r Renderer) ClipEnd() { rl.EndScissorMode() }

// keymap pairs each raylib key a zenui widget reacts to with its logical
// zenui.Key -- fixed by the two APIs' own vocabularies, so it's data here
// rather than a chain of repeated if-statements.
var keymap = []struct {
	rlKey int32
	k     zenui.Key
}{
	{rl.KeyEnter, zenui.KeyEnter},
	{rl.KeyKpEnter, zenui.KeyEnter},
	{rl.KeyEscape, zenui.KeyEscape},
	{rl.KeyBackspace, zenui.KeyBackspace},
	{rl.KeyUp, zenui.KeyUp},
	{rl.KeyDown, zenui.KeyDown},
	{rl.KeyLeft, zenui.KeyLeft},
	{rl.KeyRight, zenui.KeyRight},
	{rl.KeyPageUp, zenui.KeyPageUp},
	{rl.KeyPageDown, zenui.KeyPageDown},
	{rl.KeyTab, zenui.KeyTab},
}

// Input snapshots raylib's current-frame input into a zenui.Input.
func Input() zenui.Input {
	in := zenui.Input{
		MouseX:            int(rl.GetMouseX()),
		MouseY:            int(rl.GetMouseY()),
		MouseDown:         rl.IsMouseButtonDown(rl.MouseLeftButton),
		MousePressed:      rl.IsMouseButtonPressed(rl.MouseLeftButton),
		MouseRightPressed: rl.IsMouseButtonPressed(rl.MouseRightButton),
		WheelY:            rl.GetMouseWheelMove(),
		DeltaTime:         rl.GetFrameTime(),
	}
	for {
		ch := rl.GetCharPressed()
		if ch == 0 {
			break
		}
		in.Chars = append(in.Chars, ch)
	}
	for _, m := range keymap {
		if rl.IsKeyPressed(m.rlKey) {
			in.Keys = append(in.Keys, m.k)
		}
	}
	return in
}
