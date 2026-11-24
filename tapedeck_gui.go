//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"image/color"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/raycanvas"
	"github.com/ha1tch/zenui"
)

// The tape deck on the screen (tapedeck.go is what it knows, tapedeck_layout.go where its parts
// go): a cassette drawn from shapes (cassette_vector.go), the title on its label, a bar that
// fills as the tape plays, and the buttons of a recorder. It goes in the top or the bottom of the
// border, or, when the border is not shown or is too thin for it, over the picture near the top
// or the bottom, as the Preferences window says.
//
// The buttons are made like the Play button's (playbutton_gui.go): a raised face, a little
// brighter under the pointer, pressed in while the mouse button is held, acting on release.
// Play and Pause stay pressed in while they are on, as a recorder's keys do.

// deckMinBand is the least a band of the border can be, in window units, for the deck to be
// put in it; a thinner border has the deck over the picture instead.
const deckMinBand = 40

// deckPictureBands is where the picture is in the window, and how tall the top and the bottom of
// its border are, in window units. ok is false when there is no single picture to put a deck
// on: a tiled layout, or a window on its way to a new size.
func (dm *DisplayManager) deckPictureBands() (r pictureRect, top, bottom float32, ok bool) {
	if dm.tiles != nil || dm.isAnimating {
		return r, 0, 0, false
	}
	if dm.windowKept {
		w, h := drawableSize()
		r = centrePicture(w, h, dm.frameTop(), dm.currentWidth, dm.currentHeight-dm.reservedTopHeight)
	} else {
		r = pictureRect{
			Y: float32(dm.reservedTopHeight), W: float32(dm.currentWidth),
			H: float32(dm.currentHeight - dm.reservedTopHeight), Scale: 1,
		}
	}
	if r.W <= 0 || r.H <= 0 {
		return r, 0, 0, false
	}
	if dm.borderView().Shown {
		mult := float32(dm.screen.multiplier)
		_, y := dm.classicScreenOffset()
		t := float32(int(y) - dm.reservedTopHeight)
		_, total := dm.totalSize()
		b := float32(total)*mult - t - float32(dm.screenH)*mult
		top, bottom = max(t, 0)*r.Scale, max(b, 0)*r.Scale
	}
	return r, top, bottom, true
}

// deckArea is the room the deck has, in window units: the top or the bottom band of the border
// when the border is shown and thick enough, otherwise a strip of the picture near the top or the
// bottom.
func (dm *DisplayManager) deckArea(place tapeDeckPlace, size tapeCassetteSize) (deckRect, bool) {
	r, top, bottom, ok := dm.deckPictureBands()
	if !ok || place == tapeDeckOff {
		return deckRect{}, false
	}
	const margin = 2
	x, w := int(r.X), int(r.W)
	band := top
	if place == tapeDeckBottom {
		band = bottom
	}
	need := float32(deckMinBand)
	if size == cassetteBig {
		need = deckBigMinBand
	}
	if band >= need {
		y := int(r.Y) + margin
		if place == tapeDeckBottom {
			y = int(r.Y+r.H-band) + margin
		}
		return deckRect{x, y, w, int(band) - 2*margin}, true
	}
	h := clampInt(int(r.H*0.24), 56, 128)
	switch size {
	case cassetteBig:
		h = clampInt(int(r.H*0.62), 120, 460)
	case cassetteHidden:
		h = clampInt(int(r.H*0.16), 56, 96)
	}
	y := int(r.Y) + 8
	if place == tapeDeckBottom {
		y = int(r.Y+r.H) - h - 8
	}
	return deckRect{x, y, w, h}, true
}

// deckHit is the part of the deck the pointer is on, other than its four buttons.
type deckHit int

const (
	hitNone      deckHit = iota
	hitMinimise          // the full deck's minimise button
	hitClose             // the full deck's close button
	hitBody              // the full deck's panel: a drag carries it, to the edge it is nearer when let go
	hitChipPlay          // the chip's play (or pause) button
	hitChipClose         // the chip's close button
	hitChipBody          // the chip's panel: a click makes the deck full, a drag carries it
)

// deckDragSlop is how far the pointer goes after a press, in window units, before it is a drag
// and not a click.
const deckDragSlop = 6

// tapeDeckUI is the deck as the window has it.
type tapeDeckUI struct {
	deck   tapeDeck
	view   tapeDeckView
	layout tapeDeckLayout
	scale  int // the text's size in pixels per font pixel
	size   tapeCassetteSize

	icons  [tapeButtons][16]iconRow
	iconOK bool

	hover, down tapeButton // the button under the pointer, and the one held; -1 for none
	hand        bool

	chip    deckChipLayout // the chip, when the deck is one
	shown   deckRect       // the panel or the chip as drawn, which the pointer is found on
	hit     deckHit        // what the pointer is on, other than a button
	press   deckHit        // what the mouse button went down on
	dragged bool           // the pointer has gone far enough since the press for it to be a drag
	pressY  float32        // where it was
	dragY0  float32        // where the pointer was when the press became a drag: the deck is carried from there
	dragDY  int            // how far the deck has been carried from its edge, up or down, in window units

	morph     deckMorph // the deck moving from one form to another (tapedeck_morph.go)
	morphing  bool      // a move is under way: the deck is not to be clicked
	chipStyle bool      // the deck is drawn as the chip (the chip's own layout is the one in use)

	wantHand bool // the pointer is on something that answers a click

	round      *raycanvas.Context
	roundCache *raycanvas.SharedCache
	roundW     int32
	roundH     int32
}

func newTapeDeckUI() *tapeDeckUI {
	u := &tapeDeckUI{hover: -1, down: -1}
	sprites := parseTransportSprites()
	var ok [tapeButtons]bool
	for b, name := range [tapeButtons]string{"play", "rewind", "stop", "pause"} {
		u.icons[b], ok[b] = sprites[name]
	}
	u.iconOK = ok[0] && ok[1] && ok[2] && ok[3]
	return u
}

func (u *tapeDeckUI) canvas() *raycanvas.Context {
	w, h := int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight())
	if u.round == nil || u.roundW != w || u.roundH != h {
		if u.round != nil {
			u.round.Unload()
		}
		if u.roundCache == nil {
			u.roundCache = raycanvas.NewSharedCache()
		}
		u.round = raycanvas.NewContext(w, h, u.roundCache)
		u.round.SetAntialias(true)
		u.roundW, u.roundH = w, h
	}
	return u.round
}

// Visible reports whether the deck is on the screen this frame.
func (u *tapeDeckUI) Visible() bool { return u.view.Visible && !u.shown.empty() }

// reset lets go of whatever the pointer had hold of, for a frame with no deck on the screen.
func (u *tapeDeckUI) reset() {
	u.down, u.press, u.dragged = -1, hitNone, false
	u.dragDY, u.morphing, u.chipStyle = 0, false, false
	u.morph.reset()
	setHandCursor(&u.hand, false)
}

// carry puts a deck that is being dragged where the pointer, at y in a window screenH high, has
// taken it, kept inside the window. It is called with the deck laid out at its edge.
func (u *tapeDeckUI) carry(y float32, screenH int) {
	u.dragDY = 0
	if !u.dragged || (u.press != hitBody && u.press != hitChipBody) {
		return
	}
	u.dragDY = clampInt(int(y-u.dragY0), -u.shown.Y, screenH-u.shown.Y-u.shown.H)
	u.layout, u.chip, u.shown = u.layout.shifted(u.dragDY), u.chip.shifted(u.dragDY), u.shown.moved(u.dragDY)
}

// geometry is the parts of the deck as drawn this frame, as a full deck's layout whichever of the
// two they are.
func (u *tapeDeckUI) geometry() tapeDeckLayout {
	if u.chipStyle {
		return chipAsLayout(u.chip)
	}
	return u.layout
}

// deckSlideOffset is how far, in window units, a panel at panel has slid by slide (0 to 1 in
// time, eased here as the dock's slide is): up for a deck at the top, down for one at the bottom,
// and by slide 1 clear of a window screenH high.
func deckSlideOffset(panel deckRect, bottom bool, screenH int, slide float32) int {
	full := -(panel.Y + panel.H)
	if bottom {
		full = screenH - panel.Y
	}
	return int(float32(full) * easeInOutCubic(min(max(slide, 0), 1)))
}

// hitAt is the part of the deck, other than its four buttons, that the point is on.
func (u *tapeDeckUI) hitAt(x, y float32) deckHit {
	if u.chipStyle {
		switch {
		case u.chip.Play.Contains(x, y):
			return hitChipPlay
		case u.chip.Close.Contains(x, y):
			return hitChipClose
		case u.chip.Panel.Contains(x, y):
			return hitChipBody
		}
		return hitNone
	}
	switch {
	case u.layout.Minimise.Contains(x, y):
		return hitMinimise
	case u.layout.Close.Contains(x, y):
		return hitClose
	case u.layout.Panel.Contains(x, y):
		return hitBody
	}
	return hitNone
}

// Update is called once a frame, with the tape of the machine in front and where the deck is to
// go. owned says something else has the pointer (a menu, the dock, the debugger, a dialog). The
// pointer changes the deck too: the wheel over it steps its size, and a drag carries it with the
// pointer and, when it is let go, it goes to the top or the bottom, whichever its middle is
// nearer; place and size are changed to say so, and changed is true when either was, for the
// caller to save. The deck moves from one form to another, and comes in and goes out, eased
// (tapedeck_morph.go).
func (u *tapeDeckUI) Update(zx *ZenZX, place *tapeDeckPlace, size *tapeCassetteSize, owned bool) (changed bool) {
	dt := float64(rl.GetFrameTime())
	p := rl.GetMousePosition()
	u.deck.place = *place
	u.size = *size
	u.deck.hot = !owned && u.shown.Contains(p.X, p.Y) // where the deck was last frame
	u.view = u.deck.Update(zx.tape, dt)
	u.layout, u.chip, u.shown = tapeDeckLayout{}, deckChipLayout{}, deckRect{}
	u.hover, u.hit = -1, hitNone
	if !u.view.Visible {
		u.reset()
		return false
	}
	t := uiTextFor()
	if t == nil {
		return false
	}
	dm := zx.window()
	u.scale = 1
	if dm.screen.multiplier >= 3 {
		u.scale = 2
	}
	bottom := *place == tapeDeckBottom
	chipForm := u.view.Form == deckChip
	key := deckKey{form: u.view.Form, size: *size, bottom: bottom}
	var target tapeDeckLayout // where the parts of the form the deck is to be in go
	if chipForm {
		key.size = 0
		if area, ok := dm.deckArea(*place, cassetteHidden); ok {
			target = chipAsLayout(layoutDeckChip(area, t.CellH()*u.scale, t.Measure("00:00 / 00:00", u.scale), bottom))
		}
	} else if area, ok := dm.deckArea(*place, *size); ok {
		target = layoutTapeDeckSized(area, t.CellH()*u.scale, t.CellH(), *size, bottom)
	}
	if target.Panel.empty() {
		u.reset()
		return false
	}
	// From one form to another the parts move, eased; on the way the deck is not to be clicked.
	geo, asChip, moving := u.morph.step(key, target, chipForm, dt)
	u.morphing, u.chipStyle = moving, asChip
	u.layout, u.chip = tapeDeckLayout{}, deckChipLayout{}
	if asChip {
		u.chip = layoutAsChip(geo)
	} else {
		u.layout = geo
	}
	u.shown = geo.Panel
	if moving {
		owned = true
	}
	u.dragDY = 0
	if !moving {
		u.carry(p.Y, rl.GetScreenHeight())
	}
	if u.view.Slide > 0 {
		// A deck coming in or going out (at the end of the tape, or sent away) slides along its own
		// edge's way, eased as the dock is; on the way it is not to be clicked.
		dy := deckSlideOffset(u.shown, bottom, rl.GetScreenHeight(), float32(u.view.Slide))
		u.layout, u.chip, u.shown = u.layout.shifted(dy), u.chip.shifted(dy), u.shown.moved(dy)
		owned = true
	}
	changed = u.interact(zx.tape, deckPointer{
		X: p.X, Y: p.Y, ScreenH: float32(rl.GetScreenHeight()),
		Pressed:  rl.IsMouseButtonPressed(rl.MouseButtonLeft),
		Released: rl.IsMouseButtonReleased(rl.MouseButtonLeft),
		Down:     rl.IsMouseButtonDown(rl.MouseButtonLeft),
		Wheel:    rl.GetMouseWheelMove(),
	}, place, size, owned, dt)
	setHandCursor(&u.hand, u.wantHand)
	return changed
}

// deckPointer is the pointer for a frame: where it is, what the left button did, and the wheel.
type deckPointer struct {
	X, Y                    float32
	Pressed, Released, Down bool
	Wheel                   float32
	ScreenH                 float32 // the window's height, whose middle is where a drag changes edge
}

// interact is what the pointer does to the deck of this frame, once it has been laid out: the
// hover, the clicks, a drag, the wheel. It says whether place or size changed.
func (u *tapeDeckUI) interact(tape *Tape, p deckPointer, place *tapeDeckPlace, size *tapeCassetteSize, owned bool, dt float64) (changed bool) {
	if !owned {
		for b := tapeButton(0); b < tapeButtons; b++ {
			if u.layout.Buttons[b].Contains(p.X, p.Y) {
				u.hover = b
			}
		}
		if u.hover < 0 {
			u.hit = u.hitAt(p.X, p.Y)
		}
	}
	pressed, released := p.Pressed, p.Released
	// A deck that was being carried and is let go goes to the edge its middle is nearer, from
	// where it was let go.
	if released && u.dragged && (u.press == hitBody || u.press == hitChipBody) {
		u.morph.drop(u.geometry(), u.chipStyle)
		want := tapeDeckTop
		if u.shown.Y+u.shown.H/2 >= int(p.ScreenH/2) {
			want = tapeDeckBottom
		}
		if want != *place {
			*place, changed = want, true
		}
	}
	if pressed {
		if u.hover >= 0 {
			u.down = u.hover
		}
		if u.hit != hitNone {
			u.press, u.dragged, u.pressY = u.hit, false, p.Y
		}
	}
	if u.down >= 0 && released {
		if u.hover == u.down {
			u.deck.Press(tape, u.down)
			fmt.Printf("Tape: %s\n", tapeButtonNames[u.down])
		}
		u.down = -1
	}
	if u.press != hitNone && released {
		if u.hit == u.press {
			switch u.press {
			case hitMinimise:
				u.deck.Minimise()
			case hitClose, hitChipClose:
				u.deck.Hide()
			case hitChipPlay:
				u.deck.PressFromChip(tape)
			case hitChipBody:
				if !u.dragged {
					u.deck.Expand()
				}
			}
		}
		u.press = hitNone
	}
	if !p.Down {
		u.down, u.press = -1, hitNone
	}

	// A press on the deck's panel that goes on to move is a drag: the deck is carried with the
	// pointer (Update does that), from where the pointer was when it became one.
	if u.press == hitBody || u.press == hitChipBody {
		if !u.dragged && math.Abs(float64(p.Y-u.pressY)) > deckDragSlop {
			u.dragged, u.dragY0 = true, p.Y
		}
	}

	// The wheel over the deck steps its size; past the smallest it is the chip, and from the
	// chip a turn the other way makes it full.
	wheel := 0.0
	if !owned && u.shown.Contains(p.X, p.Y) {
		wheel = float64(p.Wheel)
	}
	if step := u.deck.Wheel(wheel, dt); step != 0 {
		if u.view.Mode == deckChip {
			if step > 0 {
				u.deck.Expand()
			}
		} else if next, mini := stepDeckSize(*size, step); mini {
			u.deck.Minimise()
		} else if next != *size {
			*size, changed = next, true
		}
	}
	u.wantHand = u.hover >= 0 || (u.hit != hitNone && u.hit != hitBody)
	return changed
}

// uiTextFor is the interface font's text renderer, or nil before the front end has one.
func uiTextFor() interface {
	CellH() int
	CellW() int
	Draw(s string, x, y, scale int, tint rl.Color) int
	Measure(s string, scale int) int
} {
	if uiText == nil {
		return nil
	}
	if t := uiText(); t != nil {
		return t
	}
	return nil
}

// fitText cuts s down, with ... on the end, until it is at most w wide at the scale.
func fitText(t interface{ Measure(string, int) int }, s string, scale, w int) string {
	if t.Measure(s, scale) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		if c := string(r) + "..."; t.Measure(c, scale) <= w {
			return c
		}
	}
	return ""
}

// clock is a time as a recorder's counter would say it.
func deckClock(seconds float64) string {
	s := int(seconds + 0.5)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func (r deckRect) rl() rl.Rectangle {
	return rl.Rectangle{X: float32(r.X), Y: float32(r.Y), Width: float32(r.W), Height: float32(r.H)}
}

// drawCassette draws the shapes of a cassette into at.
func drawCassette(c *raycanvas.Context, at deckRect, shapes []cassetteShape) {
	sx := float32(at.W) / cassetteW
	sy := float32(at.H) / cassetteH
	px := func(x float32) float32 { return float32(at.X) + x*sx }
	py := func(y float32) float32 { return float32(at.Y) + y*sy }
	c.SetLineCap("round")
	for _, sh := range shapes {
		filled := sh.Fill.A > 0
		stroked := sh.StrokeW > 0 && sh.Stroke.A > 0
		if stroked {
			c.SetStrokeColor(sh.Stroke)
			c.SetLineWidth(max(1, sh.StrokeW*sx))
		}
		switch sh.Op {
		case opRoundRect:
			x, y, w, h, r := px(sh.X), py(sh.Y), sh.W*sx, sh.H*sy, sh.R*sx
			switch {
			case filled && sh.Gradient:
				c.FillRoundRectGradient(x, y, w, h, r, sh.Fill, sh.Fill2)
			case filled:
				c.SetFillColor(sh.Fill)
				c.FillRoundRect(x, y, w, h, r)
			}
			if stroked {
				c.StrokeRoundRect(x, y, w, h, r)
			}
		case opCircle:
			x, y, r := px(sh.X), py(sh.Y), sh.R*sx
			if filled {
				c.SetFillColor(sh.Fill)
				c.FillCircle(x, y, r)
			}
			if stroked {
				c.StrokeCircle(x, y, r)
			}
		case opLine:
			if !stroked {
				continue
			}
			c.BeginPath()
			c.MoveTo(px(sh.Pts[0][0]), py(sh.Pts[0][1]))
			c.LineTo(px(sh.Pts[1][0]), py(sh.Pts[1][1]))
			c.Stroke()
		case opPolygon:
			c.BeginPath()
			for i, p := range sh.Pts {
				if i == 0 {
					c.MoveTo(px(p[0]), py(p[1]))
				} else {
					c.LineTo(px(p[0]), py(p[1]))
				}
			}
			c.ClosePath()
			if filled {
				c.SetFillColor(sh.Fill)
				c.Fill()
			}
			if stroked {
				c.Stroke()
			}
		}
	}
}

// Draw paints the deck over the picture, inside the frame being drawn.
func (u *tapeDeckUI) Draw(theme zenui.Theme) {
	if !u.Visible() {
		return
	}
	l := u.layout
	c := u.canvas()
	text := uiTextFor()
	if u.chipStyle {
		u.drawChip(c, text, theme)
		return
	}

	u.drawPanel(c, l.Panel, theme)

	// The cassette, when the Preferences window has one.
	progress := u.view.Fraction
	if !l.Cassette.empty() {
		drawCassette(c, l.Cassette, cassetteShapes(u.view.Variant, u.view.Spin, progress))
	}

	u.drawBar(c, l.Bar)

	// The buttons.
	for b := tapeButton(0); b < tapeButtons; b++ {
		u.drawButton(c, b, theme)
	}
	u.drawGlyph(c, l.Minimise, hitMinimise, theme)
	u.drawGlyph(c, l.Close, hitClose, theme)

	// Text goes over what the canvas drew.
	if text == nil {
		return
	}
	if u.view.Label != "" {
		if l.LabelOnCassette {
			// As large as the label allows: the big cassette's is much taller than the text.
			s := clampInt(l.Label.H/text.CellH(), 1, 8)
			if u.size != cassetteBig {
				s = min(s, u.scale)
			}
			for s > 1 && text.Measure(u.view.Label, s) > l.Label.W {
				s--
			}
			str := fitText(text, u.view.Label, s, l.Label.W)
			text.Draw(str, l.Label.X+2, l.Label.Y+l.Label.H-text.CellH()*s, s, rl.Color{R: 22, G: 24, B: 70, A: 255})
		} else if !l.Name.empty() {
			str := fitText(text, u.view.Label, u.scale, l.Name.W)
			text.Draw(str, l.Name.X, l.Name.Y, u.scale, rlColour(theme.Text))
		}
	}
	if !l.Time.empty() {
		str := deckClock(u.view.Elapsed) + " / " + deckClock(u.view.Length)
		if text.Measure(str, u.scale) > l.Time.W {
			str = deckClock(u.view.Elapsed)
		}
		if text.Measure(str, u.scale) <= l.Time.W {
			y := l.Time.Y + (l.Time.H-text.CellH()*u.scale)/2
			text.Draw(str, l.Time.X+l.Time.W-text.Measure(str, u.scale), y, u.scale, rlColour(theme.DimText))
		}
	}
}

// drawPanel paints the deck's (or the chip's) background: solid enough to read against whatever
// the border or the picture is.
func (u *tapeDeckUI) drawPanel(c *raycanvas.Context, r deckRect, theme zenui.Theme) {
	panel := r.rl()
	radius := pictureRadius(panel)
	base := rlColour(theme.Panel)
	base.A = 225
	c.SetFillColor(base)
	c.FillRoundRect(panel.X, panel.Y, panel.Width, panel.Height, radius)
	c.SetStrokeColor(rlColour(theme.DimText))
	c.SetLineWidth(1)
	c.StrokeRoundRect(panel.X, panel.Y, panel.Width, panel.Height, radius)
}

// drawBar paints the progress bar: a track, and the tape played so far in the cassette's own
// colour.
func (u *tapeDeckUI) drawBar(c *raycanvas.Context, bar deckRect) {
	if bar.empty() {
		return
	}
	br := float32(bar.H) / 2
	c.SetFillColor(color.RGBA{R: 0, G: 0, B: 0, A: 150})
	c.FillRoundRect(float32(bar.X), float32(bar.Y), float32(bar.W), float32(bar.H), br)
	if w := float32(bar.W) * float32(u.view.Fraction); w >= 1 {
		c.SetFillColor(rotateHue(rgb(0xE8453F), float64(((u.view.Variant%cassetteVariants)+cassetteVariants)%cassetteVariants*cassetteHueStep)))
		c.FillRoundRect(float32(bar.X), float32(bar.Y), max(w, float32(bar.H)), float32(bar.H), br)
	}
	c.SetStrokeColor(color.RGBA{R: 255, G: 255, B: 255, A: 60})
	c.SetLineWidth(1)
	c.StrokeRoundRect(float32(bar.X), float32(bar.Y), float32(bar.W), float32(bar.H), br)
}

// drawButtonFace paints the raised face of a button, in the state it is in.
func drawButtonFace(c *raycanvas.Context, r deckRect, pressed, hover bool, theme zenui.Theme) {
	at := r.rl()
	radius := min(pictureRadius(at), at.Height/3)
	var top, bottom color.RGBA
	edge := theme.DimText
	switch {
	case pressed:
		top, bottom = color.RGBA{A: 110}, color.RGBA{R: 255, G: 255, B: 255, A: 25}
		edge = theme.Text
	case hover:
		top, bottom = color.RGBA{R: 255, G: 255, B: 255, A: 85}, color.RGBA{A: 25}
		edge = theme.Text
	default:
		top, bottom = color.RGBA{R: 255, G: 255, B: 255, A: 45}, color.RGBA{A: 45}
	}
	base := rlColour(theme.Panel)
	base.A = 235
	c.SetFillColor(base)
	c.FillRoundRect(at.X, at.Y, at.Width, at.Height, radius)
	c.FillRoundRectGradient(at.X, at.Y, at.Width, at.Height, radius, top, bottom)
	c.SetStrokeColor(rlColour(edge))
	c.SetLineWidth(1)
	c.StrokeRoundRect(at.X, at.Y, at.Width, at.Height, radius)
}

// drawButton paints one button: its face, then its sprite.
func (u *tapeDeckUI) drawButton(c *raycanvas.Context, b tapeButton, theme zenui.Theme) {
	r := u.layout.Buttons[b]
	if r.empty() {
		return
	}
	on := (b == tapePlay && u.view.Playing) || (b == tapePause && u.view.Paused) || (b == tapeRewind && u.view.Rewinding)
	pressed := on || (u.hover == b && u.down == b)
	drawButtonFace(c, r, pressed, u.hover == b, theme)
	u.drawSprite(r, b, pressed, theme)
}

// drawSprite paints a button's sprite, enlarged by whole pixels so none is blurred, in the
// middle of the face r.
func (u *tapeDeckUI) drawSprite(r deckRect, b tapeButton, pressed bool, theme zenui.Theme) {
	if !u.iconOK || r.H < 12 {
		return
	}
	s := clampInt((r.H-4)/12, 1, 4)
	x0, y0, x1, y1 := iconBounds(u.icons[b])
	left := r.X + r.W/2 - (x1-x0)*s/2
	topY := r.Y + r.H/2 - (y1-y0)*s/2
	if pressed {
		left++
		topY++
	}
	ink := rlColour(theme.Text)
	for _, run := range iconRuns(u.icons[b]) {
		if run.ink {
			rl.DrawRectangle(int32(left+(run.x-x0)*s), int32(topY+(run.y-y0)*s), int32(run.n*s), int32(s), ink)
		}
	}
}

// drawGlyph paints a corner button of the deck or the chip: a face with a dash (minimise) or a
// cross (close) on it. kind says which, and is what the pointer is on when it is hovered.
func (u *tapeDeckUI) drawGlyph(c *raycanvas.Context, r deckRect, kind deckHit, theme zenui.Theme) {
	if r.empty() {
		return
	}
	hover := u.hit == kind
	pressed := hover && u.press == kind && rl.IsMouseButtonDown(rl.MouseButtonLeft)
	drawButtonFace(c, r, pressed, hover, theme)
	off := float32(0)
	if pressed {
		off = 1
	}
	x, y, w, h := float32(r.X)+off, float32(r.Y)+off, float32(r.W), float32(r.H)
	m := h / 4
	c.SetStrokeColor(rlColour(theme.Text))
	c.SetLineWidth(max(1, h/9))
	c.SetLineCap("round")
	c.BeginPath()
	if kind == hitMinimise {
		c.MoveTo(x+m, y+h*0.68)
		c.LineTo(x+w-m, y+h*0.68)
	} else {
		c.MoveTo(x+m, y+m)
		c.LineTo(x+w-m, y+h-m)
		c.MoveTo(x+w-m, y+m)
		c.LineTo(x+m, y+h-m)
	}
	c.Stroke()
}

// drawChip paints the deck as the chip: the mini cassette, the play button (the pause button
// while the tape plays), the counter, the close button and a hairline of progress.
func (u *tapeDeckUI) drawChip(c *raycanvas.Context, text interface {
	CellH() int
	Measure(s string, scale int) int
	Draw(s string, x, y, scale int, tint rl.Color) int
}, theme zenui.Theme) {
	l := u.chip
	u.drawPanel(c, l.Panel, theme)
	drawCassette(c, l.Cassette, cassetteShapes(u.view.Variant, u.view.Spin, u.view.Fraction))
	hover := u.hit == hitChipPlay
	held := hover && u.press == hitChipPlay && rl.IsMouseButtonDown(rl.MouseButtonLeft)
	drawButtonFace(c, l.Play, held || u.view.Playing, hover, theme)
	icon := tapePlay
	if u.view.Playing {
		icon = tapePause
	}
	u.drawSprite(l.Play, icon, held || u.view.Playing, theme)
	u.drawGlyph(c, l.Close, hitChipClose, theme)
	u.drawBar(c, l.Bar)
	if text == nil || l.Time.empty() {
		return
	}
	str := deckClock(u.view.Elapsed) + " / " + deckClock(u.view.Length)
	if text.Measure(str, u.scale) > l.Time.W {
		str = deckClock(u.view.Elapsed)
	}
	if text.Measure(str, u.scale) <= l.Time.W {
		text.Draw(str, l.Time.X, l.Time.Y+(l.Time.H-text.CellH()*u.scale)/2, u.scale, rlColour(theme.DimText))
	}
}
