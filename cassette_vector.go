// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"image/color"
	"math"
	"strings"
)

// ============================================================================
// The cassette, as shapes
//
// The tape deck's cassette is not a picture: it is a short list of shapes (rounded rectangles,
// circles, lines, polygons) in a design space of 512 by 325 units, which the window draws at
// any size with raycanvas, and which cassetteSVG writes out as an SVG file. A cassette of
// another colour is the same shapes with every colour's hue turned by some degrees, so there
// are twelve of them, 30 degrees apart, from the red the design was made in round the wheel.
// The greys (the label, the window, the screws, the holes) have no hue to turn and stay.
//
// The shapes are the plain geometry of a compact cassette: a case with a screw in each
// corner, a label with its rules, a plate with two reels and the tape's window, and the
// trapezoid at the bottom with its holes. The reels' notches turn and the packs of tape in
// the window shift as the tape plays; those two are why the list is made for a spin and a
// progress and not once.
// ============================================================================

// cassetteW and cassetteH are the design space the shapes are in. The proportions are those of
// a compact cassette, 100.4 by 63.8 mm.
const (
	cassetteW = 512
	cassetteH = 325
)

// The title goes on the label, in this rectangle of the design space: the writing area of the
// paper, above its first rule. It is the same in every colour.
const (
	cassetteLabelX0 = 96.0
	cassetteLabelY0 = 36.0
	cassetteLabelX1 = 460.0
	cassetteLabelY1 = 84.0
)

// cassetteVariants is how many colours of cassette there are, and cassetteHueStep how many
// degrees apart they are.
const (
	cassetteVariants = 12
	cassetteHueStep  = 360 / cassetteVariants
)

// cassetteNames are the colours' names, from the red the design was made in.
var cassetteNames = [cassetteVariants]string{
	"red", "orange", "amber", "lime", "green", "teal",
	"cyan", "blue", "indigo", "purple", "magenta", "rose",
}

type cassetteOp int

const (
	opRoundRect cassetteOp = iota // X, Y, W, H, R
	opCircle                      // X, Y the centre, R the radius
	opLine                        // Pts[0] to Pts[1]
	opPolygon                     // Pts, closed
)

// cassetteShape is one shape of a cassette. A shape is filled when Fill is not transparent
// (from Fill at the top to Fill2 at the bottom when Gradient), and outlined when StrokeW is
// not zero. A line is only ever stroked.
type cassetteShape struct {
	Op         cassetteOp
	X, Y, W, H float32
	R          float32
	Pts        [][2]float32
	Fill       color.RGBA
	Fill2      color.RGBA
	Gradient   bool
	Stroke     color.RGBA
	StrokeW    float32
}

func rgb(hex uint32) color.RGBA {
	return color.RGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 255}
}

func cassetteAlpha(hex uint32, a uint8) color.RGBA {
	c := rgb(hex)
	c.A = a
	return c
}

// rotateHue turns the hue of c by deg degrees, keeping its lightness, saturation and alpha. A
// grey has no hue and comes back as it was.
func rotateHue(c color.RGBA, deg float64) color.RGBA {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (mx + mn) / 2
	d := mx - mn
	if d < 1e-9 {
		return c
	}
	var s float64
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	var h float64
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h = math.Mod(h*60+deg, 360)
	if h < 0 {
		h += 360
	}
	hue2 := func(p, q, t float64) float64 {
		t = math.Mod(t+1, 1)
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hn := h / 360
	to8 := func(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	return color.RGBA{R: to8(hue2(p, q, hn+1.0/3)), G: to8(hue2(p, q, hn)), B: to8(hue2(p, q, hn-1.0/3)), A: c.A}
}

// cassetteShapes is the cassette of the given colour (0 to cassetteVariants-1; others wrap),
// with its reels turned spin radians and progress (0 to 1) of the tape on the right-hand reel.
func cassetteShapes(variant int, spin, progress float64) []cassetteShape {
	variant = ((variant % cassetteVariants) + cassetteVariants) % cassetteVariants
	deg := float64(variant * cassetteHueStep)
	hue := func(c color.RGBA) color.RGBA { return rotateHue(c, deg) }
	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}

	var s []cassetteShape
	add := func(sh cassetteShape) { s = append(s, sh) }
	rr := func(x, y, w, h, r float32, fill color.RGBA) cassetteShape {
		return cassetteShape{Op: opRoundRect, X: x, Y: y, W: w, H: h, R: r, Fill: fill}
	}
	line := func(x0, y0, x1, y1 float32, c color.RGBA, w float32) cassetteShape {
		return cassetteShape{Op: opLine, Pts: [][2]float32{{x0, y0}, {x1, y1}}, Stroke: c, StrokeW: w}
	}
	circle := func(x, y, r float32, fill color.RGBA) cassetteShape {
		return cassetteShape{Op: opCircle, X: x, Y: y, R: r, Fill: fill}
	}

	// The case, with a lighter rim, and the ridged wings either side of the bottom.
	body := rr(0, 0, cassetteW, cassetteH, 16, hue(rgb(0xB32826)))
	body.Fill2, body.Gradient = hue(rgb(0xA50D0D)), true
	add(body)
	rim := rr(5, 5, cassetteW-10, cassetteH-10, 12, color.RGBA{})
	rim.Stroke, rim.StrokeW = hue(cassetteAlpha(0xE2605C, 150)), 2
	add(rim)
	ridge := hue(cassetteAlpha(0x4A0505, 110))
	for i := 0; i < 9; i++ {
		y := float32(252 + i*7)
		add(line(14, y, 98, y, ridge, 1.6))
		add(line(cassetteW-98, y, cassetteW-14, y, ridge, 1.6))
	}

	// The label: paper, the side mark, the rules the title is written on.
	add(rr(33, 32, 446, 196, 7, rgb(0xF6F6F6)))
	add(rr(49, 40, 37, 44, 2, rgb(0x111111)))
	letter := rgb(0xFFFFFF)
	add(line(57, 76, 67.5, 48, letter, 4))
	add(line(67.5, 48, 78, 76, letter, 4))
	add(line(60.5, 67, 74.5, 67, letter, 3.4))
	rule := cassetteAlpha(0x222222, 200)
	add(line(96, 86, 460, 86, rule, 1.3))
	add(line(96, 95, 460, 95, rule, 1.3))

	// The plate, its reels and the window.
	plate := rr(97, 100, 318, 99, 8, hue(rgb(0x8C0D0F)))
	plate.Stroke, plate.StrokeW = hue(rgb(0xC23A36)), 2
	add(plate)
	for _, cx := range []float32{147, 365} {
		add(circle(cx, 150, 34, rgb(0xDADADA)))
		add(circle(cx, 150, 29, rgb(0xFFFFFF)))
		for k := 0; k < 6; k++ {
			a := spin + float64(k)*math.Pi/3
			ca, sa := float32(math.Cos(a)), float32(math.Sin(a))
			add(line(cx+13*ca, 150+13*sa, cx+23*ca, 150+23*sa, rgb(0xB4B4B4), 5.5))
		}
		add(circle(cx, 150, 11, rgb(0xEDEDED)))
	}
	add(rr(193, 117, 126, 66, 3, rgb(0x262626)))
	left := float32(7 + (1-progress)*26)
	right := float32(7 + progress*26)
	add(rr(195, 119, left, 62, 2, rgb(0x5A5A5A)))
	add(rr(317-right, 119, right, 62, 2, rgb(0x5A5A5A)))
	add(rr(195+left+2, 143, 120-left-right-4, 14, 1, rgb(0x8E8E8E)))
	for k := 0; k < 7; k++ {
		x := float32(215 + k*13)
		add(line(x, 126, x, 126+float32(10+(k%3)*4), cassetteAlpha(0xFFFFFF, 90), 1.4))
	}

	// The trapezoid at the bottom, its holes and its screw.
	trap := cassetteShape{Op: opPolygon,
		Pts:  [][2]float32{{102, 244}, {410, 244}, {428, 319}, {84, 319}},
		Fill: hue(rgb(0xA50808)), Stroke: hue(rgb(0x6E0606)), StrokeW: 1.5}
	add(trap)
	hole := rgb(0x1B1010)
	add(circle(132, 303, 8.5, hole))
	add(circle(378, 303, 8.5, hole))
	add(rr(176, 293, 14, 19, 4, hole))
	add(rr(322, 293, 14, 19, 4, hole))

	// Screws: the four corners and the one above the trapezoid.
	screw := func(x, y, r float32) {
		c := circle(x, y, r, rgb(0xD2D2D2))
		c.Stroke, c.StrokeW = rgb(0x8A8A8A), 1.2
		add(c)
		add(line(x-r*0.6, y+r*0.6, x+r*0.6, y-r*0.6, rgb(0x6A6A6A), 1.8))
	}
	for _, p := range [][2]float32{{16, 14}, {496, 14}, {16, 311}, {496, 311}} {
		screw(p[0], p[1], 7)
	}
	screw(256, 262, 7.5)

	// The light along the top edge.
	add(line(20, 2.5, cassetteW-20, 2.5, cassetteAlpha(0xFFFFFF, 90), 2))
	return s
}

// hex is the colour as an SVG attribute, and its opacity.
func svgColour(c color.RGBA) (string, float64) {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B), float64(c.A) / 255
}

// cassetteSVG is the cassette of a colour as an SVG document, at rest (reels at 0, tape half
// across), with the label's rectangle outlined only when guide is set.
func cassetteSVG(variant int, guide bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`+"\n", cassetteW, cassetteH, cassetteW, cassetteH)
	fmt.Fprintf(&b, "<title>cassette, %s</title>\n", cassetteNames[((variant%cassetteVariants)+cassetteVariants)%cassetteVariants])
	paint := func(attr string, c color.RGBA) string {
		hexc, op := svgColour(c)
		if c.A == 0 {
			return attr + `="none"`
		}
		if op < 1 {
			return fmt.Sprintf(`%s="%s" %s-opacity="%.3f"`, attr, hexc, attr, op)
		}
		return fmt.Sprintf(`%s="%s"`, attr, hexc)
	}
	grad := 0
	for _, sh := range cassetteShapes(variant, 0, 0.5) {
		fill := paint("fill", sh.Fill)
		if sh.Gradient {
			grad++
			t, _ := svgColour(sh.Fill)
			u, _ := svgColour(sh.Fill2)
			fmt.Fprintf(&b, `<defs><linearGradient id="g%d" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`+"\n", grad, t, u)
			fill = fmt.Sprintf(`fill="url(#g%d)"`, grad)
		}
		stroke := ""
		if sh.StrokeW > 0 {
			stroke = fmt.Sprintf(` %s stroke-width="%.2f" stroke-linecap="round"`, paint("stroke", sh.Stroke), sh.StrokeW)
		}
		switch sh.Op {
		case opRoundRect:
			fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" %s%s/>`+"\n", sh.X, sh.Y, sh.W, sh.H, sh.R, fill, stroke)
		case opCircle:
			fmt.Fprintf(&b, `<circle cx="%.2f" cy="%.2f" r="%.2f" %s%s/>`+"\n", sh.X, sh.Y, sh.R, fill, stroke)
		case opLine:
			fmt.Fprintf(&b, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f"%s/>`+"\n", sh.Pts[0][0], sh.Pts[0][1], sh.Pts[1][0], sh.Pts[1][1], stroke)
		case opPolygon:
			pts := make([]string, len(sh.Pts))
			for i, p := range sh.Pts {
				pts[i] = fmt.Sprintf("%.2f,%.2f", p[0], p[1])
			}
			fmt.Fprintf(&b, `<polygon points="%s" %s%s/>`+"\n", strings.Join(pts, " "), fill, stroke)
		}
	}
	if guide {
		fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" fill="none" stroke="#00a0ff" stroke-dasharray="4 3"/>`+"\n",
			cassetteLabelX0, cassetteLabelY0, cassetteLabelX1-cassetteLabelX0, cassetteLabelY1-cassetteLabelY0)
	}
	b.WriteString("</svg>\n")
	return b.String()
}
