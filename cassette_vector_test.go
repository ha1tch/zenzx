// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"encoding/xml"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateHue(t *testing.T) {
	grey := color.RGBA{R: 120, G: 120, B: 120, A: 255}
	if rotateHue(grey, 90) != grey {
		t.Error("a grey changed under a hue rotation")
	}
	red := color.RGBA{R: 200, G: 0, B: 0, A: 255}
	for _, c := range []struct {
		deg  float64
		want color.RGBA
	}{
		{120, color.RGBA{R: 0, G: 200, B: 0, A: 255}},
		{240, color.RGBA{R: 0, G: 0, B: 200, A: 255}},
		{60, color.RGBA{R: 200, G: 200, B: 0, A: 255}},
		{360, red},
		{0, red},
		{-120, color.RGBA{R: 0, G: 0, B: 200, A: 255}},
	} {
		if got := rotateHue(red, c.deg); got != c.want {
			t.Errorf("red turned %v degrees = %v, want %v", c.deg, got, c.want)
		}
	}
	// Alpha and lightness survive.
	c := color.RGBA{R: 180, G: 40, B: 38, A: 77}
	got := rotateHue(c, 137)
	if got.A != 77 {
		t.Errorf("alpha changed: %d", got.A)
	}
	lum := func(c color.RGBA) float64 {
		mx := math.Max(float64(c.R), math.Max(float64(c.G), float64(c.B)))
		mn := math.Min(float64(c.R), math.Min(float64(c.G), float64(c.B)))
		return (mx + mn) / 2
	}
	if math.Abs(lum(got)-lum(c)) > 1.5 {
		t.Errorf("lightness changed: %v -> %v", lum(c), lum(got))
	}
}

func bodyOf(v int) color.RGBA { return cassetteShapes(v, 0, 0)[0].Fill }

func TestCassetteVariantsDiffer(t *testing.T) {
	if len(cassetteNames) != cassetteVariants || cassetteVariants*cassetteHueStep != 360 {
		t.Fatal("the variants do not go once round the wheel")
	}
	seen := map[color.RGBA]string{}
	names := map[string]bool{}
	for v := 0; v < cassetteVariants; v++ {
		c := bodyOf(v)
		if prev, dup := seen[c]; dup {
			t.Errorf("%s has the body colour of %s", cassetteNames[v], prev)
		}
		seen[c] = cassetteNames[v]
		if names[cassetteNames[v]] || cassetteNames[v] == "" {
			t.Errorf("name %q repeated or empty", cassetteNames[v])
		}
		names[cassetteNames[v]] = true
	}
	if bodyOf(cassetteVariants) != bodyOf(0) || bodyOf(-cassetteVariants) != bodyOf(0) || bodyOf(-1) != bodyOf(cassetteVariants-1) {
		t.Error("variant numbers do not wrap")
	}
	// The first is the red the design was made in; the others are not red.
	r := bodyOf(0)
	if !(r.R > 150 && r.G < 80 && r.B < 80) {
		t.Errorf("variant 0 is not red: %v", r)
	}
	b := bodyOf(7) // blue
	if !(b.B > b.R && b.B > b.G) {
		t.Errorf("variant %q is not blue: %v", cassetteNames[7], b)
	}
	y := bodyOf(2) // amber/yellow-ish: red and green high, blue low
	if !(y.R > y.B && y.G > y.B) {
		t.Errorf("variant %q is not yellowish: %v", cassetteNames[2], y)
	}
}

// Only the coloured parts turn; every grey is the same in every cassette.
func TestCassetteGreysDoNotTurn(t *testing.T) {
	base := cassetteShapes(0, 0, 0.5)
	for v := 1; v < cassetteVariants; v++ {
		got := cassetteShapes(v, 0, 0.5)
		if len(got) != len(base) {
			t.Fatalf("variant %d has %d shapes, the red has %d", v, len(got), len(base))
		}
		for i := range base {
			a, b := base[i], got[i]
			if a.Op != b.Op || a.X != b.X || a.Y != b.Y || a.W != b.W || a.H != b.H || a.R != b.R {
				t.Fatalf("variant %d: shape %d is somewhere else", v, i)
			}
			grey := func(c color.RGBA) bool { return c.R == c.G && c.G == c.B }
			if grey(a.Fill) && a.Fill != b.Fill || grey(a.Stroke) && a.Stroke != b.Stroke {
				t.Errorf("variant %d: grey of shape %d changed", v, i)
			}
		}
	}
}

func TestCassetteShapesStayInside(t *testing.T) {
	for _, sh := range cassetteShapes(3, 1.3, 0.7) {
		var x0, y0, x1, y1 float32
		switch sh.Op {
		case opRoundRect:
			x0, y0, x1, y1 = sh.X, sh.Y, sh.X+sh.W, sh.Y+sh.H
		case opCircle:
			x0, y0, x1, y1 = sh.X-sh.R, sh.Y-sh.R, sh.X+sh.R, sh.Y+sh.R
		default:
			x0, y0, x1, y1 = sh.Pts[0][0], sh.Pts[0][1], sh.Pts[0][0], sh.Pts[0][1]
			for _, p := range sh.Pts {
				x0, x1 = min(x0, p[0]), max(x1, p[0])
				y0, y1 = min(y0, p[1]), max(y1, p[1])
			}
		}
		if x0 < 0 || y0 < 0 || x1 > cassetteW || y1 > cassetteH {
			t.Errorf("shape %+v leaves the %dx%d design space", sh, cassetteW, cassetteH)
		}
	}
}

// The label is a rectangle of the paper: the title is written in the same place on every colour.
func TestCassetteLabelIsOnThePaper(t *testing.T) {
	shapes := cassetteShapes(0, 0, 0)
	paper := shapes[2+9*2] // body, rim, 18 ridges, then the paper
	if paper.Fill != rgb(0xF6F6F6) {
		t.Fatalf("shape %d is not the paper: %+v", 2+9*2, paper)
	}
	if cassetteLabelX0 < float64(paper.X) || cassetteLabelY0 < float64(paper.Y) ||
		cassetteLabelX1 > float64(paper.X+paper.W) || cassetteLabelY1 > float64(paper.Y+paper.H) {
		t.Errorf("the label rectangle is off the paper %v", paper)
	}
	// Nothing but the paper and the rules is drawn in it, so the title is on white: the
	// side mark and the plate are outside the rectangle, and the first rule is below it.
	for i, sh := range shapes {
		if i <= 2+9*2 || sh.Op != opRoundRect {
			continue
		}
		if sh.X < cassetteLabelX1 && sh.X+sh.W > cassetteLabelX0 && sh.Y < cassetteLabelY1 && sh.Y+sh.H > cassetteLabelY0 && sh.Fill.A > 0 {
			t.Errorf("shape %d (%+v) covers the label rectangle", i, sh)
		}
	}
	if cassetteLabelY1 >= 86 {
		t.Error("the label rectangle runs over the first rule")
	}
}

func TestCassetteTapePacksShift(t *testing.T) {
	width := func(p float64, right bool) float32 {
		var packs []float32
		for _, sh := range cassetteShapes(0, 0, p) {
			if sh.Op == opRoundRect && sh.Fill == rgb(0x5A5A5A) {
				packs = append(packs, sh.W)
			}
		}
		if len(packs) != 2 {
			t.Fatalf("%d packs of tape", len(packs))
		}
		if right {
			return packs[1]
		}
		return packs[0]
	}
	if !(width(0, false) > width(1, false) && width(0, true) < width(1, true)) {
		t.Error("the tape does not move from the left pack to the right as it plays")
	}
	if width(-2, false) != width(0, false) || width(5, true) != width(1, true) {
		t.Error("progress is not limited to 0..1")
	}
}

func TestCassetteReelsTurn(t *testing.T) {
	spokes := func(spin float64) [][2]float32 {
		var out [][2]float32
		for _, sh := range cassetteShapes(0, spin, 0) {
			if sh.Op == opLine && sh.StrokeW == 5.5 {
				out = append(out, sh.Pts[1])
			}
		}
		return out
	}
	a, b := spokes(0), spokes(0.3)
	if len(a) != 12 || len(b) != 12 {
		t.Fatalf("%d and %d notches, want 12 (6 on each reel)", len(a), len(b))
	}
	if a[0] == b[0] {
		t.Error("the notches did not move with the spin")
	}
	// A sixth of a turn puts the notches where they were.
	c := spokes(math.Pi / 3)
	if math.Abs(float64(c[0][0]-a[1][0])) > 0.01 || math.Abs(float64(c[0][1]-a[1][1])) > 0.01 {
		t.Error("the notches are not six-fold")
	}
}

func TestCassetteSVG(t *testing.T) {
	for v := 0; v < cassetteVariants; v++ {
		s := cassetteSVG(v, v == 0)
		d := xml.NewDecoder(strings.NewReader(s))
		var rects int
		for {
			tok, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s is not well-formed XML: %v", cassetteNames[v], err)
			}
			if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "rect" {
				rects++
			}
		}
		if rects < 10 {
			t.Errorf("%s: %d rects", cassetteNames[v], rects)
		}
		if !strings.Contains(s, "cassette, "+cassetteNames[v]) {
			t.Errorf("%s: no title", cassetteNames[v])
		}
		if (v == 0) != strings.Contains(s, "stroke-dasharray") {
			t.Errorf("%s: the label guide is wrong", cassetteNames[v])
		}
	}
}

// ZENZX_WRITE_CASSETTES=dir writes the twelve cassettes as SVG files (and a contact sheet) into dir.
func TestWriteCassettes(t *testing.T) {
	dir := os.Getenv("ZENZX_WRITE_CASSETTES")
	if dir == "" {
		t.Skip("set ZENZX_WRITE_CASSETTES=dir to write the SVG files")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var sheet strings.Builder
	sheet.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2100 1020" width="2100" height="1020"><rect width="2100" height="1020" fill="#f2f2f2"/>` + "\n")
	for v := 0; v < cassetteVariants; v++ {
		name := "cassette-" + cassetteNames[v] + ".svg"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(cassetteSVG(v, false)), 0o644); err != nil {
			t.Fatal(err)
		}
		inner := cassetteSVG(v, false)
		inner = inner[strings.Index(inner, ">")+2:]
		inner = strings.TrimSuffix(inner, "</svg>\n")
		inner = strings.ReplaceAll(inner, `id="g1"`, `id="g`+cassetteNames[v]+`"`)
		inner = strings.ReplaceAll(inner, `url(#g1)`, `url(#g`+cassetteNames[v]+`)`)
		x, y := 20+(v%4)*520, 20+(v/4)*330
		sheet.WriteString(`<g transform="translate(` + deckItoa(x) + ` ` + deckItoa(y) + `)">` + inner + "</g>\n")
	}
	sheet.WriteString("</svg>\n")
	if err := os.WriteFile(filepath.Join(dir, "cassettes.svg"), []byte(sheet.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func deckItoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}
