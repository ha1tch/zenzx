//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"math"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The animation that ends a machine (Machine > Stop, or Delete in the dock): a ground of cardboard,
// like the floor of a toy theatre, comes up from below the window with a bin on it and the machine
// standing in the bin as a small icon; a leg comes down from the top of the window and a boot
// stamps the icon flat, and goes up again; then the ground goes back down, with the bin and what is
// left of the machine. The bin has weight: it is jolted as the ground arrives, pushed down when the
// boot lands and shaken as the ground goes. It takes about two and a half seconds. The machine is
// held where it was while it plays, and is gone when it is over.
//
// The timing is stompAt, a function of the time since the start, so that it can be looked at
// without a window. The pictures are stompsprites_gui.go's, and can be replaced.

// When each part of the animation is over, in seconds from the start.
const (
	stompRiseEnd     = 0.41 // the ground, the bin and the machine have come up from below to their place (and go on a little past it)
	stompDropStart   = 0.95 // the ground has settled there, and the boot starts to fall
	stompDropEnd     = 1.45 // the boot is on the icon
	stompHoldEnd     = 1.65 // and stays there a moment
	stompLiftEnd     = 2.05 // and is back where it came from
	stompRetireStart = 2.25 // the ground starts to go back down
	stompDuration    = 2.80 // and is gone: how long the whole animation takes
)

const (
	stompFlatFrac = 0.2 // how much of its height the icon has left under the boot
	// stompGroundBrightness is how bright the ground is drawn, of what its pictures say: a quarter dimmer.
	stompGroundBrightness = 0.75
)

// stompOutline is the thin grey line round the machine while it is in one piece.
var stompOutline = rl.Color{R: 150, G: 150, B: 150, A: 255}

// The bin is jolted three times: it is thrown up a little when the ground stops under it, pushed
// down hard when the boot lands, and thrown up a little when the ground starts to go. Each jolt is
// a swing of stompJoltPeriod seconds that dies away at stompJoltDamp per second.
const (
	stompJoltPeriod = 0.30
	stompJoltDamp   = 8.0
)

// stompJolt is one jolt: when it happens, and how hard, in pixels of the picture (down is positive).
type stompJolt struct{ at, size float64 }

var stompJolts = [...]stompJolt{{stompRiseEnd, -2}, {stompDropEnd, 5}, {stompRetireStart, -2}}

// The ground comes up as a weight on a spring does that is let go below its place: it starts
// slowly, reaches its place at stompRiseEnd, goes a little past it and settles, by stompDropStart.
// riseSpring is that, t seconds after the start, 0 below the window and 1 in its place.
const (
	stompSpringOmega = 8.0 // radians per second, of the spring
	stompSpringZeta  = 0.7 // how much of the swing is damped: less than 1, so that it goes past
)

func riseSpring(t float64) float64 {
	wd := stompSpringOmega * math.Sqrt(1-stompSpringZeta*stompSpringZeta)
	k := stompSpringZeta / math.Sqrt(1-stompSpringZeta*stompSpringZeta)
	return max(0, 1-math.Exp(-stompSpringZeta*stompSpringOmega*t)*(math.Cos(wd*t)+k*math.Sin(wd*t)))
}

// easeInBack goes from 0 to 1, a little back before it leaves: the ground, pulled down, is pushed
// up a little first.

func easeInBack(u float64) float64 {
	const c1 = 0.6
	return (c1+1)*u*u*u - c1*u*u
}

// stompPose is where everything is at one moment.
type stompPose struct {
	boot   float32 // 0: the boot's sole is at the top edge, off the window; 1: it is on the flattened icon
	landed bool    // the boot has come down: the icon is flat from here on
	// stage is where the ground, the bin and the machine are: 0 is below the window, 1 in their place.
	// It goes a little past 1 when the ground comes up, and when it is about to go down.
	stage float32
	bump  float32 // how far the bin is jolted from its place, in pixels of the picture; down is positive
	dim   float32 // how far the machine's picture is dimmed behind the icon, 0 to 1
}

// stompAt is the pose t seconds after the start; done says the animation is over.
func stompAt(t float64) (p stompPose, done bool) {
	if t >= stompDuration {
		return stompPose{}, true
	}
	if t < 0 {
		t = 0
	}
	switch {
	case t < stompDropStart:
		p.stage = float32(riseSpring(t))
	case t < stompRetireStart:
		p.stage = 1
	default:
		p.stage = float32(1 - easeInBack((t-stompRetireStart)/(stompDuration-stompRetireStart)))
	}
	p.dim = max(0, min(1, p.stage))
	bump := 0.0
	for _, j := range stompJolts {
		if dt := t - j.at; dt > 0 {
			bump += j.size * math.Sin(2*math.Pi*dt/stompJoltPeriod) * math.Exp(-stompJoltDamp*dt)
		}
	}
	p.bump = float32(bump)
	switch {
	case t < stompDropStart:
	case t < stompDropEnd:
		u := float32((t - stompDropStart) / (stompDropEnd - stompDropStart))
		p.boot = u * u // it falls: slowly at first
	case t < stompHoldEnd:
		p.boot, p.landed = 1, true
	case t < stompLiftEnd:
		u := float32((t - stompHoldEnd) / (stompLiftEnd - stompHoldEnd))
		p.boot, p.landed = (1-u)*(1-u), true // it goes up fast, then eases
	default:
		p.landed = true
	}
	return p, false
}

// stompAnim is the animation of one machine.
type stompAnim struct {
	machine *deckMachine
	start   time.Time
	// target is where it plays: the machine's picture, or the whole window.
	target rl.Rectangle
}

func (a *stompAnim) elapsed(now time.Time) float64 { return now.Sub(a.start).Seconds() }

// draw draws the animation as it is at now, over what has been drawn of the machine.
func (a *stompAnim) draw(tex *stompTextures, now time.Time) {
	p, done := stompAt(a.elapsed(now))
	if done {
		return
	}
	tg := a.target
	// A dim over the machine's picture, so that the small icon can be seen on it.
	rl.DrawRectangleRec(tg, rl.Color{A: uint8(170 * p.dim)})

	s := int32(max(2, min(8, int(min(tg.Width, tg.Height))/4/int(tex.icon.Width))))
	iw, ih := tex.icon.Width*s, tex.icon.Height*s
	cx := int32(tg.X + tg.Width/2)
	// The machine stands two pixels into the top of a bin (covering its handle), which stands on the ground; the three
	// are centred together in the window.
	bnw, bnh := tex.bin.Width*s, tex.bin.Height*s
	iconBottom0 := int32(tg.Y+tg.Height/2) - (ih+bnh-2*s)/2 + ih

	// The three are below the window until the ground has come up: stage 0 is the machine's top
	// just below the window's bottom edge, 1 is where they stand. The bin is jolted from its place,
	// by whole pixels of the picture, and the machine with it; the ground is not.
	below := int32(tg.Y+tg.Height) - (iconBottom0 - ih)
	off := int32(float32(below) * (1 - p.stage))
	jolt := int32(math.Round(float64(p.bump))) * s
	iconBottom := iconBottom0 + off + jolt
	iconX := cx - iw/2
	flatH := max(s, int32(float32(ih)*stompFlatFrac))
	bw, bh := tex.boot.Width*s, tex.boot.Height*s

	// The boot's sole runs from the top edge (boot 0) to the flat icon's top (boot 1).
	top := int32(tg.Y)
	landing := iconBottom - flatH
	bootBottom := top + int32(float32(landing-top)*p.boot)
	iconH := ih
	if p.landed {
		iconH = flatH
	} else if h := iconBottom - bootBottom; h < ih {
		iconH = max(flatH, h) // the boot is pressing it down as it comes
	}

	white := rl.White
	// The ground is a quarter dimmer than the rest, so that it is a floor and not the scene.
	full := float32(255)
	g := uint8(full * stompGroundBrightness)
	dimmed := rl.Color{R: g, G: g, B: g, A: 255}

	// The ground, across the window, with its surface where the bin's bottom is, and the bin.
	binTop := iconBottom - 2*s
	groundY := iconBottom0 - 2*s + bnh - tex.surface*s + off
	tile := func(t rl.Texture2D, x, y int32, tint rl.Color) {
		rl.DrawTexturePro(t, rl.Rectangle{Width: float32(t.Width), Height: float32(t.Height)},
			rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(t.Width * s), Height: float32(t.Height * s)}, rl.Vector2{}, 0, tint)
	}
	// All of it is drawn inside the machine's picture: below it, the ground is not yet there.
	rl.BeginScissorMode(int32(tg.X), int32(tg.Y), int32(tg.Width), int32(tg.Height))
	defer rl.EndScissorMode()
	if gw := tex.groundTop.Width * s; gw > 0 {
		x0 := cx - gw/2
		for x0 > int32(tg.X) {
			x0 -= gw
		}
		for x := x0; x < int32(tg.X+tg.Width); x += gw {
			tile(tex.groundTop, x, groundY, dimmed)
			for y := groundY + tex.groundTop.Height*s; y < int32(tg.Y+tg.Height); y += tex.ground.Height * s {
				tile(tex.ground, x, y, dimmed)
			}
		}
	}
	tile(tex.bin, cx-bnw/2, binTop, white)

	if p.landed && tex.hasFlat {
		fw, fh := tex.flat.Width*s, tex.flat.Height*s
		rl.DrawTexturePro(tex.flat, rl.Rectangle{Width: float32(tex.flat.Width), Height: float32(tex.flat.Height)},
			rl.Rectangle{X: float32(cx - fw/2), Y: float32(iconBottom - fh), Width: float32(fw), Height: float32(fh)}, rl.Vector2{}, 0, white)
	} else {
		rl.DrawTexturePro(tex.icon, rl.Rectangle{Width: float32(tex.icon.Width), Height: float32(tex.icon.Height)},
			rl.Rectangle{X: float32(iconX), Y: float32(iconBottom - iconH), Width: float32(iw), Height: float32(iconH)}, rl.Vector2{}, 0, white)
		// While it is whole the black slab has a thin grey line round it, so that it can be seen
		// against the dimmed picture; it is outside the icon, which is drawn whole, and goes when the
		// boot has flattened it.
		if !p.landed {
			t := max(1, s/3)
			rl.DrawRectangleLinesEx(rl.Rectangle{X: float32(iconX - t), Y: float32(iconBottom - iconH - t),
				Width: float32(iw + 2*t), Height: float32(iconH + 2*t)}, float32(t), stompOutline)
		}
	}

	// The leg: the boot's height above the sole, then the leg up to the top edge.
	bootTop := bootBottom - bh
	lw := tex.leg.Width * s
	lh := tex.leg.Height * s
	legX := cx - bw/2 + tex.legMid2*s/2 - lw/2 // the leg joins the boot where the boot's top is
	for y := bootTop; y > top-lh; y -= lh {
		rl.DrawTexturePro(tex.leg, rl.Rectangle{Width: float32(tex.leg.Width), Height: float32(tex.leg.Height)},
			rl.Rectangle{X: float32(legX), Y: float32(y - lh), Width: float32(lw), Height: float32(lh)}, rl.Vector2{}, 0, rl.White)
	}
	rl.DrawTexturePro(tex.boot, rl.Rectangle{Width: float32(tex.boot.Width), Height: float32(tex.boot.Height)},
		rl.Rectangle{X: float32(cx - bw/2), Y: float32(bootTop), Width: float32(bw), Height: float32(bh)}, rl.Vector2{}, 0, rl.White)
}
