// Package zenui is a renderer-agnostic file Open/Save dialog widget. The
// dialog owns navigation, selection, filename entry, scrolling and keyboard
// handling; the host supplies a thin Renderer (to draw rectangles and text) and
// an Input snapshot (mouse and keyboard state) each frame. Because the package
// imports no graphics library, the same dialog can be driven by raylib, Ebiten,
// a software framebuffer, or a test harness.
package zenui

// Colour is a straight RGBA colour, 0..255 per channel. The host maps it to its
// own colour type when drawing.
type Colour struct{ R, G, B, A uint8 }

// Rect is an axis-aligned rectangle in pixels.
type Rect struct{ X, Y, W, H int }

// Contains reports whether (px,py) lies inside the rectangle.
func (r Rect) Contains(px, py int) bool {
	return px >= r.X && px < r.X+r.W && py >= r.Y && py < r.Y+r.H
}

// Renderer is the drawing surface the host provides. All coordinates are in
// pixels. TextWidth must agree with how DrawText lays the string out so the
// dialog can measure and clip strings correctly.
type Renderer interface {
	FillRect(r Rect, c Colour)
	StrokeRect(r Rect, c Colour, thickness int)
	// FillRectGradientV fills r with a vertical gradient, top at colour
	// top fading to bottom at colour bottom.
	FillRectGradientV(r Rect, top, bottom Colour)
	// FillRectGradientHMultiply fills r with a horizontal gradient
	// (left to right), composited onto whatever's already drawn
	// beneath it using multiply blend mode -- darkens toward whichever
	// end is closer to black, leaves the underlying content unchanged
	// wherever the gradient is white.
	FillRectGradientHMultiply(r Rect, left, right Colour)
	// DrawLine draws a single straight line from (x1,y1) to (x2,y2).
	// Thickness is in pixels; hosts without native variable-width line
	// support may round to the nearest whole pixel.
	DrawLine(x1, y1, x2, y2, thickness int, c Colour)
	// DrawText draws s with its top-left at (x,y) and returns the x just past
	// the last glyph. scale is an integer multiplier (1 = native font size).
	DrawText(s string, x, y, scale int, c Colour)
	TextWidth(s string, scale int) int
	LineHeight(scale int) int
	// Clip restricts subsequent drawing to r until ClipEnd is called. Nested
	// clips are not required.
	Clip(r Rect)
	ClipEnd()
}

// Key is a logical key the dialog reacts to. The host maps its own key codes to
// these in the Input snapshot.
type Key int

const (
	KeyNone Key = iota
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPageUp
	KeyPageDown
	KeyTab
)

// Input is the per-frame snapshot of pointer and keyboard state the host hands
// to the dialog. Pressed-style fields are edge-triggered (true only on the frame
// the event occurred); MouseDown is level-triggered.
type Input struct {
	MouseX, MouseY    int
	MouseDown         bool // left button held
	MousePressed      bool // left button went down this frame
	MouseRightPressed bool // right button went down this frame
	WheelY            float32
	// DeltaTime is the seconds elapsed since the last Update call --
	// zero is a valid value (nothing time-based simply doesn't
	// advance that frame), used only by widgets with their own
	// animation state (MenuBar's ZSP logo colour rotation).
	DeltaTime float32

	// Chars are the printable runes typed this frame (for the filename field).
	Chars []rune
	// Keys are the logical keys pressed this frame (edge-triggered).
	Keys []Key
}

// pressed reports whether key k is among the keys pressed this frame.
func (in Input) pressed(k Key) bool {
	for _, kk := range in.Keys {
		if kk == k {
			return true
		}
	}
	return false
}
