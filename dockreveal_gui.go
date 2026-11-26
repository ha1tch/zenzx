//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"math"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The dock shown for a machine that was made for a snapshot (snapshotopen_gui.go). The machine
// appears in the dock, which slides into view for it, with a chevron over the dock pointing down at
// its picture, as wide as the picture. Some seconds after the dock is in, when the pointer is not on
// the dock, the dock slides out again as it does when the pointer leaves it; the chevron does not go
// with it: it stays at the bottom of the window, over whatever the machine is showing, pointing at
// where the dock is, until the pointer is on the picture or a few seconds more have gone. (Over the
// busy picture of a game a small mark is lost; one as wide as a picture, white with a black outline, is not.) A pointer that is on
// the dock keeps it open as it always does, and a dock the keyboard opened stays open.

const (
	dockRevealLinger = 3 * time.Second        // how long the dock stays after it is in
	chevronAfterDock = 6 * time.Second        // how long the chevron stays after the dock's time is up, the dock gone or not
	dockRevealGrow   = 0.25                   // how big the new picture starts, of its size at rest
	chevronPeriod    = 900 * time.Millisecond // one round of the chevron's bouncing: a big hop and a small one
	chevronFadeIn    = 150 * time.Millisecond // from nothing to seen
	chevronFadeOut   = 500 * time.Millisecond // from seen to nothing, ending as its time does
	chevronHop       = 30                     // px, the height of its big hop above where it lands
	chevronSmallHop  = 0.35                   // the small hop, of the big one
	chevronBigPart   = 0.6                    // the share of a round that the big hop takes

	chevronEdgeGap = 2 // px, between where it lands at the window's edge and the edge
)

// dockReveal is the dock being shown for a machine just made.
type dockReveal struct {
	on        bool
	machine   *deckMachine
	began     time.Time
	holdUntil time.Time // zero until the dock is in
}

// revealNew shows the dock for m, a machine just made for a snapshot: the dock slides in and the
// picture of m grows into its place in it. Where the dock is off in Preferences it is not shown.
func (d *machineDeck) revealNew(m *deckMachine) {
	d.reveal = dockReveal{}
	if !d.dockAllowed() {
		d.hide()
		return
	}
	now := time.Now()
	d.show(false)
	m.mag = dockRevealGrow
	d.reveal = dockReveal{on: true, machine: m, began: now}
	if d.state == barShown {
		d.reveal.holdUntil = now.Add(dockRevealLinger) // it was open already
	}
}

// revealArrived is the dock's coming in finished: its second begins.
func (d *machineDeck) revealArrived(now time.Time) {
	if d.reveal.on && d.reveal.holdUntil.IsZero() {
		d.reveal.holdUntil = now.Add(dockRevealLinger)
	}
}

// revealHolds says the dock is to stay open at now whatever the pointer does: it is within its
// second.
func (d *machineDeck) revealHolds(now time.Time) bool {
	return d.reveal.on && !d.reveal.holdUntil.IsZero() && now.Before(d.reveal.holdUntil)
}

// revealIndex is the place of the machine the dock was shown for, -1 when there is none.
func (d *machineDeck) revealIndex() int {
	if !d.reveal.on {
		return -1
	}
	for i, m := range d.machines {
		if m == d.reveal.machine {
			return i
		}
	}
	return -1
}

// revealUntil is when the chevron is gone: the dock's time and then chevronAfterDock, zero while the
// dock's time has not begun.
func (d *machineDeck) revealUntil() time.Time {
	if !d.reveal.on || d.reveal.holdUntil.IsZero() {
		return time.Time{}
	}
	return d.reveal.holdUntil.Add(chevronAfterDock)
}

// revealTick ends the reveal when it is over: the machine is gone, a picture is on its way to the
// screen, the pointer is on the picture the chevron points at, or the chevron's time is up. The dock
// leaving does not end it (the chevron stays at the window's edge): it ends the dock's time, which
// is then taken to be up.
func (d *machineDeck) revealTick(hover int, now time.Time) {
	if !d.reveal.on {
		return
	}
	if d.reveal.holdUntil.IsZero() && (d.state == barSlidingOut || d.state == barHidden) {
		d.reveal.holdUntil = now // the dock went before it was in: its time is up
	}
	i := d.revealIndex()
	switch {
	case i < 0, d.flying:
		d.reveal = dockReveal{}
	case hover == i:
		d.reveal = dockReveal{}
	case !d.reveal.holdUntil.IsZero() && !now.Before(d.revealUntil()):
		d.reveal = dockReveal{}
	}
}

// chevronSprite is the chevron as a picture of 34 by 16 pixels: a white V pointing down with a black
// outline, '.' for nothing, '#' black and 'o' white. It is drawn at the size of the picture it points
// at with each of its pixels a square of the same size (nearest neighbour), so that it is as wide as
// the picture and made of big pixels, as the Spectrum's own are.
var chevronSprite = []string{
	"#####........................#####",
	"#######....................#######",
	"##o######................######o##",
	"##ooo######............######ooo##",
	"##ooooo######........######ooooo##",
	"##ooooooo######....######ooooooo##",
	"##ooooooooo############ooooooooo##",
	"###oooooooooo########oooooooooo###",
	"#####oooooooooo####oooooooooo#####",
	".######oooooooooooooooooooo######.",
	"...######oooooooooooooooo######...",
	".....######oooooooooooo######.....",
	".......######oooooooo######.......",
	".........######oooo######.........",
	"...........############...........",
	".............########.............",
}

// chevronPlace is where the chevron is and how much of it is seen: the sprite at Scale screen pixels
// to a pixel of its own, its left edge at Left and its bottom at Bottom.
type chevronPlace struct {
	Left   float32
	Bottom float32 // the lowest edge of the sprite
	Scale  float32 // screen pixels to a pixel of the sprite
	Alpha  float32 // 0 to 1
}

// Top is the sprite's top edge.
func (p chevronPlace) Top() float32 { return p.Bottom - float32(len(chevronSprite))*p.Scale }

// chevronHeight is how high the chevron is above where it lands a share phase (0 to 1) of the way
// through a round: a ball thrown up and let fall, twice, the second time lower and quicker. It is 0
// as a hop begins and ends, which is the chevron on the ground.
func chevronHeight(phase float64) float32 {
	hop := func(u float64) float64 { return 4 * u * (1 - u) } // 0 at both ends, 1 half way
	if phase < chevronBigPart {
		return chevronHop * float32(hop(phase/chevronBigPart))
	}
	return chevronHop * chevronSmallHop * float32(hop((phase-chevronBigPart)/(1-chevronBigPart)))
}

// chevronAt is the chevron over picture, the rectangle the machine's picture is drawn in, landing
// on ground (the picture's top when the dock is in, the window's bottom edge when it is not), at
// since the reveal began and left before the chevron's time is up (negative while the dock is
// still coming in).
func chevronAt(picture rl.Rectangle, ground float32, since, left time.Duration) chevronPlace {
	phase := float64(since%chevronPeriod) / float64(chevronPeriod)
	p := chevronPlace{
		Left:  picture.X,
		Scale: picture.Width / float32(len(chevronSprite[0])),
		Alpha: 1,
	}
	p.Bottom = ground - chevronHeight(phase)
	if top := p.Top(); top < 2 {
		p.Bottom += 2 - top // not above the window
	}
	if since < chevronFadeIn {
		p.Alpha = float32(since) / float32(chevronFadeIn)
	}
	if left >= 0 && left < chevronFadeOut {
		if f := float32(left) / float32(chevronFadeOut); f < p.Alpha {
			p.Alpha = f
		}
	}
	if p.Alpha < 0 {
		p.Alpha = 0
	}
	return p
}

// chevronGround is where the chevron lands: on the picture's top, and where the picture is not in
// the window (the dock is coming in, or is gone) on the window's bottom edge.
func chevronGround(picture rl.Rectangle, screenH int) float32 {
	edge := float32(screenH) - chevronEdgeGap
	if picture.Y > edge {
		return edge
	}
	return picture.Y
}

// drawReveal draws the chevron, pointing at the picture of the machine that was made: over the dock
// when it is there, and at the bottom of the window, where the dock comes from, when it is not.
func (d *machineDeck) drawReveal(l dockLayout, screenH int, now time.Time) {
	i := d.revealIndex()
	if i < 0 {
		return
	}
	left := time.Duration(-1)
	if until := d.revealUntil(); !until.IsZero() {
		left = until.Sub(now)
	}
	// the chevron is as wide as the picture's place in the dock, not the picture while it grows into
	// it, so that it is its full width from the first moment; a picture magnified by the pointer is
	// as wide as that
	mag := d.machines[i].mag
	if mag < 1 {
		mag = 1
	}
	picture := grown(l.tiles[i], mag)
	p := chevronAt(picture, chevronGround(picture, screenH), now.Sub(d.reveal.began), left)
	if p.Alpha <= 0 {
		return
	}
	// The sprite, expanded: each of its pixels is a square of p.Scale, its edges put on whole screen
	// pixels (so a pixel is a whole number of them wide, one more or less than its neighbour's when
	// the scale is not a whole number, as nearest neighbour does), and a run of pixels of a colour
	// along a row is one rectangle.
	edge := func(base float32, n int) int32 { return int32(math.Round(float64(base + float32(n)*p.Scale))) }
	alpha := uint8(p.Alpha * 255)
	colours := map[byte]rl.Color{'#': {R: 0, G: 0, B: 0, A: alpha}, 'o': {R: 255, G: 255, B: 255, A: alpha}}
	for r, row := range chevronSprite {
		y0, y1 := edge(p.Top(), r), edge(p.Top(), r+1)
		for c := 0; c < len(row); {
			k := c
			for k < len(row) && row[k] == row[c] {
				k++
			}
			if col, ok := colours[row[c]]; ok {
				x0, x1 := edge(p.Left, c), edge(p.Left, k)
				rl.DrawRectangle(x0, y0, x1-x0, y1-y0, col)
			}
			c = k
		}
	}
}
