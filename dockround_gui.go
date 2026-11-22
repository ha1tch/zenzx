//go:build !headless

package zenzx

import (
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/raycanvas"
	"github.com/ha1tch/zenui"
)

// The dock's rounded look.
//
// The dock's panel, the pictures in it and the frames round them have round
// corners. A picture is a live texture lying over other pictures and over
// the machine on the screen, so its corners cannot be faked by painting
// over them: raycanvas draws the picture as a mesh whose corners are part
// of its shape. The window has no multisampling (the machine's picture
// must stay pixel-exact), so the edges are raycanvas's soft ones.

const (
	dockFloat  = 8  // px between the dock and the window's bottom edge
	dockRadius = 16 // px, the panel's corners

	// The panel lets the machine behind it show through: half at its top
	// edge, not at all at its bottom edge, evenly in between. The line
	// round it does the same from seven tenths. The pictures in the dock
	// are never see-through.
	dockTopAlpha     = 0.5
	dockEdgeTopAlpha = 0.7
)

// faded is col at the share a of its own opacity.
func faded(col zenui.Colour, a float32) color.RGBA {
	c := rlColour(col)
	c.A = uint8(float32(c.A)*a + 0.5)
	return c
}

// drawPanel draws the dock's panel and the line round it.
func (d *machineDeck) drawPanel(panel rl.Rectangle, theme zenui.Theme) {
	c := d.canvas()
	c.FillRoundRectGradient(panel.X, panel.Y, panel.Width, panel.Height, dockRadius,
		faded(theme.Panel, dockTopAlpha), rlColour(theme.Panel))
	c.SetLineWidth(1)
	c.StrokeRoundRectGradient(panel.X, panel.Y, panel.Width, panel.Height, dockRadius,
		faded(theme.Border, dockEdgeTopAlpha), rlColour(theme.Border))
}

// canvas is the raycanvas context the dock draws its rounded shapes with.
// It draws inside the display's own frame: its BeginFrame and EndFrame are
// never called.
func (d *machineDeck) canvas() *raycanvas.Context {
	if d.round == nil {
		d.roundCache = raycanvas.NewSharedCache()
		d.round = raycanvas.NewContext(int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), d.roundCache)
		d.round.SetAntialias(true)
	}
	return d.round
}

// pictureRadius is the corner radius of a picture drawn in at: a twelfth
// of its height, so a magnified picture keeps its proportions.
func pictureRadius(at rl.Rectangle) float32 {
	r := at.Height / 12
	if r < 4 {
		r = 4
	} else if r > 14 {
		r = 14
	}
	return r
}

// roundFill fills at with corners of the given radius.
func (d *machineDeck) roundFill(at rl.Rectangle, radius float32, col zenui.Colour) {
	c := d.canvas()
	c.SetFillColor(rlColour(col))
	c.FillRoundRect(at.X, at.Y, at.Width, at.Height, radius)
}

// roundFrame draws a line round at, centred on its edge.
func (d *machineDeck) roundFrame(at rl.Rectangle, radius float32, col zenui.Colour, thick int) {
	c := d.canvas()
	c.SetStrokeColor(rlColour(col))
	c.SetLineWidth(float32(thick))
	c.StrokeRoundRect(at.X, at.Y, at.Width, at.Height, radius)
}

// drawPicture draws machine m's picture into at with round corners.
func (d *machineDeck) drawPicture(m *deckMachine, at rl.Rectangle, radius float32) {
	if m.thumbW == 0 {
		return
	}
	d.canvas().DrawImageRoundedCropped(m.thumb,
		rl.Rectangle{X: 0, Y: 0, Width: float32(m.thumbW), Height: float32(m.thumbH)},
		at, radius)
}

// drawPictureSquare draws machine m's picture into at as it is, with square
// corners: a tile of a tiled layout, which is part of the screen.
func (d *machineDeck) drawPictureSquare(m *deckMachine, at rl.Rectangle) {
	if m.thumbW == 0 {
		return
	}
	rl.DrawTexturePro(m.thumb,
		rl.Rectangle{X: 0, Y: 0, Width: float32(m.thumbW), Height: float32(m.thumbH)},
		at, rl.Vector2{}, 0, rl.White)
}
