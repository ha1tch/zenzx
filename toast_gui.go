//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"image/color"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/raycanvas"

	"github.com/ha1tch/zenui/zenuiraylib"
)

// toast is the caption that confirms what was just done: it rises from below the bottom-right
// corner of the window, holds, and fades. It moves like zenui's zenuiraylib.OSD, whose
// sparks and timings it keeps, and differs in what that cannot be asked for: the face of the
// text is white inside a black stroke, and it is never wider than half the window (the rule
// is toastLayout's, in toastlayout.go): a long message is drawn one notch smaller, and then
// broken into lines. Behind the text is a darkened box with round corners, which, like the
// dock's panel, is more see-through at its top edge than at its bottom one; the width cap
// is that of the whole box, the padding included.
const (
	toastHold      = 100.0 // pixels risen fully opaque before the fade begins
	toastFade      = 100.0 // pixels over which it then fades to nothing
	toastTravel    = toastHold + toastFade
	toastSpeed     = 60.0 // pixels per second the caption rises
	toastSparkMS   = 0.2  // seconds between refreshes of the sparks
	toastSparkN    = 10   // sparks around the caption
	toastSparkCell = 3    // screen pixels per spark cell
	toastSparkBand = 26   // how far, in pixels, the sparks scatter beyond the text
	toastMargin    = 10   // from the window's right and bottom edges

	// The box behind the text: black, the share of its opacity at the top edge and at the
	// bottom edge (the dock's panel is half at the top and whole at the bottom), the
	// padding between it and the text, and the radius of its corners.
	toastTopAlpha    = 0.45
	toastBottomAlpha = 0.80
	toastPadX        = 12
	toastPadY        = 8
	toastRadius      = 10
)

var toastSparkShapes = [][]string{
	{".#.", "#.#", ".#."},
	{"##", "##"},
	{"#.#", ".#.", "#.#"},
	{"#...", "....", "....", "...#"},
	{"...#", "....", "....", "#..."},
}

type toastSpark struct {
	x, y  float32
	shape int
}

type toast struct {
	txt *zenuiraylib.BDFText
	rng *rand.Rand

	seq     int
	text    string
	active  bool
	travel  float32
	sparkT  float32
	sparks  []toastSpark
	lines   []string // the text as laid out for laidFor
	scale   int
	laidFor int // the window width the layout was made for; 0: not laid out
	boxW    int // the text's width and height; the box is toastPad* larger on each side
	boxH    int

	// round draws the box's rounded shape, inside the window's own frame like the dock's.
	// It is made for the window size it was last drawn at.
	round          *raycanvas.Context
	roundCache     *raycanvas.SharedCache
	roundW, roundH int
}

// canvas is the raycanvas context the box is drawn with, made again if the window has
// changed size.
func (t *toast) canvas(w, h int) *raycanvas.Context {
	if t.round == nil || t.roundW != w || t.roundH != h {
		if t.roundCache == nil {
			t.roundCache = raycanvas.NewSharedCache()
		}
		t.round = raycanvas.NewContext(int32(w), int32(h), t.roundCache)
		t.round.SetAntialias(true)
		t.roundW, t.roundH = w, h
	}
	return t.round
}

func newToast(txt *zenuiraylib.BDFText) *toast {
	return &toast{txt: txt, rng: rand.New(rand.NewSource(1))}
}

// Active reports whether a caption is on its way up.
func (t *toast) Active() bool { return t != nil && t.active }

// Start begins a new caption, replacing the one in progress.
func (t *toast) Start(text string) {
	t.text, t.travel, t.active = text, 0, true
	t.sparkT = toastSparkMS
	t.laidFor = 0
}

// fit lays the text out for a window screenW wide, when that is not already done.
func (t *toast) fit(screenW int) {
	if t.laidFor == screenW && t.lines != nil {
		return
	}
	t.lines, t.scale = toastLayout(t.text, t.txt.Measure, screenW/2-2*toastPadX)
	t.boxW = 0
	for _, l := range t.lines {
		if w := t.txt.Measure(l, t.scale); w > t.boxW {
			t.boxW = w
		}
	}
	t.boxH = len(t.lines) * t.txt.CellH() * t.scale
	t.laidFor = screenW
}

// Update moves the caption on by dt seconds. A seq other than the last one seen starts text.
func (t *toast) Update(dt float32, seq int, text string, screenW int) {
	if seq != t.seq {
		t.seq = seq
		if text != "" {
			t.Start(text)
		}
	}
	if !t.active {
		return
	}
	t.fit(screenW)
	t.travel += toastSpeed * dt
	if t.travel >= toastTravel {
		t.active, t.sparks = false, nil
		return
	}
	t.sparkT += dt
	if t.sparkT >= toastSparkMS {
		t.sparkT = 0
		t.scatter()
	}
}

func (t *toast) scatter() {
	t.sparks = t.sparks[:0]
	b := float32(toastSparkBand)
	for i := 0; i < toastSparkN; i++ {
		var x, y float32
		for tries := 0; tries < 8; tries++ {
			x = -b + t.rng.Float32()*(float32(t.boxW)+2*b)
			y = -b + t.rng.Float32()*(float32(t.boxH)+2*b)
			if x < 0 || x > float32(t.boxW) || y < 0 || y > float32(t.boxH) {
				break // outside the text, so the sparks are around it and not on it
			}
		}
		t.sparks = append(t.sparks, toastSpark{x: x, y: y, shape: t.rng.Intn(len(toastSparkShapes))})
	}
}

func (t *toast) alpha() float32 {
	if t.travel <= toastHold {
		return 1
	}
	a := 1 - (t.travel-toastHold)/toastFade
	if a < 0 {
		a = 0
	}
	return a
}

// Draw paints the caption for a window screenW by screenH. The block of lines rises from the
// bottom-right corner, its right edge toastMargin from the window's.
func (t *toast) Draw(screenW, screenH int) {
	if !t.Active() {
		return
	}
	t.fit(screenW)
	a := uint8(t.alpha() * 255)
	outerW, outerH := t.boxW+2*toastPadX, t.boxH+2*toastPadY
	bx := screenW - toastMargin - outerW
	by := screenH - toastMargin - int(t.travel)
	x, y := bx+toastPadX, by+toastPadY
	radius := float32(min(toastRadius, outerH/2))
	t.canvas(screenW, screenH).FillRoundRectGradient(float32(bx), float32(by), float32(outerW), float32(outerH), radius,
		color.RGBA{A: uint8(toastTopAlpha*float32(a) + 0.5)}, color.RGBA{A: uint8(toastBottomAlpha*float32(a) + 0.5)})

	orange := rl.NewColor(0xff, 0x9a, 0x10, a)
	white := rl.NewColor(0xff, 0xff, 0xff, a)
	black := rl.NewColor(0, 0, 0, a)

	for _, sp := range t.sparks {
		drawToastSpark(sp.shape, x+int(sp.x), y+int(sp.y), orange, black)
	}
	// The stroke first, a font pixel wide in eight directions, so that the face is on top of
	// it; then the face twice, a screen pixel apart, which reads bolder than once.
	lineH := t.txt.CellH() * t.scale
	for i, l := range t.lines {
		ly := y + i*lineH
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			t.txt.Draw(l, x+d[0]*t.scale, ly+d[1]*t.scale, t.scale, black)
		}
		t.txt.Draw(l, x, ly, t.scale, white)
		t.txt.Draw(l, x+1, ly, t.scale, white)
	}
}

func drawToastSpark(shape, x, y int, fill, outline rl.Color) {
	s := int32(toastSparkCell)
	for r, row := range toastSparkShapes[shape] {
		for c := 0; c < len(row); c++ {
			if row[c] != '#' {
				continue
			}
			px, py := int32(x)+int32(c)*s, int32(y)+int32(r)*s
			rl.DrawRectangle(px-1, py-1, s+2, s+2, outline)
			rl.DrawRectangle(px, py, s, s, fill)
		}
	}
}
