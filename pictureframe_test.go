// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"math"
	"testing"
)

func TestFrameGreys(t *testing.T) {
	if want := int(math.Round(0.2 * 255)); frameTopGrey != want {
		t.Errorf("frameTopGrey = %d, want %d (80%% grey: 80%% of the way from white to black)", frameTopGrey, want)
	}
	if want := int(math.Round(0.1 * 255)); frameBottomGrey != want {
		t.Errorf("frameBottomGrey = %d, want %d (90%% grey: 90%% of the way from white to black)", frameBottomGrey, want)
	}
	if frameBottomGrey >= frameTopGrey {
		t.Errorf("the frame goes from %d to %d: it should darken downwards", frameTopGrey, frameBottomGrey)
	}
}

func TestShadowProfile(t *testing.T) {
	p := shadowProfile(shadowReach, shadowSigma)
	if len(p) != shadowSide {
		t.Fatalf("the profile is %d long, want %d", len(p), shadowSide)
	}
	for i := range p {
		if j := len(p) - 1 - i; p[i] != p[j] {
			t.Fatalf("p[%d] = %v but p[%d] = %v: not the same at both ends", i, p[i], j, p[j])
		}
		if i > 0 && i <= len(p)/2 && p[i] < p[i-1] {
			t.Fatalf("p[%d] = %v is less than p[%d] = %v: it does not rise towards the middle", i, p[i], i-1, p[i-1])
		}
	}
	// Outside the reach of the blur there is nothing, and well inside it there is all.
	if p[0]*shadowPeak*255 >= 0.5 {
		t.Errorf("the outer edge is %v of 255: the texture is cut off while there is still shadow", p[0]*shadowPeak*255)
	}
	if mid := p[len(p)/2]; mid < 0.999 {
		t.Errorf("the middle is %v, want 1", mid)
	}
	// At the box's own edge, half.
	if e := p[shadowReach-1]/2 + p[shadowReach]/2; math.Abs(e-0.5) > 0.01 {
		t.Errorf("at the edge the profile is %v, want one half", e)
	}
}

func TestShadowAlphas(t *testing.T) {
	a := shadowAlphas()
	if len(a) != shadowSide*shadowSide {
		t.Fatalf("%d alphas, want %d", len(a), shadowSide*shadowSide)
	}
	at := func(x, y int) uint8 { return a[y*shadowSide+x] }
	mid := shadowSide / 2
	if want := int(math.Round(shadowPeak * 255)); int(at(mid, mid)) != want {
		t.Errorf("the middle is %d, want the peak %d", at(mid, mid), want)
	}
	if at(0, 0) != 0 || at(shadowSide-1, shadowSide-1) != 0 || at(0, shadowSide-1) != 0 {
		t.Error("a corner of the texture is not clear")
	}
	for _, c := range [][2]int{{3, 50}, {20, 70}, {60, 5}, {100, 140}} {
		x, y := c[0], c[1]
		if at(x, y) != at(shadowSide-1-x, y) || at(x, y) != at(x, shadowSide-1-y) || at(x, y) != at(y, x) {
			t.Errorf("(%d, %d) = %d: the texture is not the same all four ways and on the diagonal", x, y, at(x, y))
		}
	}
	// A corner's pixel is the product of the two lines, a quarter of the way into the blur at both.
	if corner := at(shadowReach, shadowReach); corner < 20 || corner > 45 {
		t.Errorf("at the corner of the box the shadow is %d, want about a quarter of the peak (%d)", corner, int(math.Round(shadowPeak*255/4)))
	}
}

func TestFrameLines(t *testing.T) {
	if frameLines != 5 {
		t.Errorf("%d lines, want 5", frameLines)
	}
	if got := float64(frameLineAlpha) / 255; got < 0.175 || got > 0.185 {
		t.Errorf("the light row's opacity is %v, want 18%%", got)
	}
	if got := float64(frameLineDarkAlpha) / 255; got < 0.275 || got > 0.285 {
		t.Errorf("the dark row's opacity is %v, want 28%%", got)
	}
	for _, c := range []struct{ top, winH int }{{24, 584}, {0, 600}, {32, 1000}, {48, 301}, {24, 24 + 13}} {
		ys := frameLineYs(c.top, c.winH)
		if len(ys) != frameLines {
			t.Fatalf("top %d, window %d high: %d lines, want %d", c.top, c.winH, len(ys), frameLines)
		}
		room := c.winH - c.top
		band := room / (frameLines + 1)
		prev := c.top
		for i, y := range ys {
			// A line is a groove of two rows, both in the window and below the bar, and none
			// touches the next.
			if y <= c.top || y+1 >= c.winH || y < prev+2 && i > 0 {
				t.Errorf("top %d, window %d high: line %d at %d (the one before at %d)", c.top, c.winH, i, y, prev)
			}
			// Regular: each band is as tall as any other, to a whole row.
			if d := y - prev; d < band || d > band+1 {
				t.Errorf("top %d, window %d high: the band above line %d is %d rows, want %d or %d", c.top, c.winH, i, d, band, band+1)
			}
			prev = y
		}
	}
	// Too little room: none, rather than lines on top of each other or under the window's edge.
	for _, winH := range []int{0, 24, 30, 36} {
		if ys := frameLineYs(24, winH); ys != nil {
			t.Errorf("a window %d high under a bar of 24 has lines at %v", winH, ys)
		}
	}
}

func TestShadowFits(t *testing.T) {
	if !shadowFits(320, 256) {
		t.Error("the smallest picture has no room for its shadow")
	}
	if shadowFits(2*shadowReach-1, 300) || shadowFits(300, 2*shadowReach-1) {
		t.Error("a picture narrower than the nine-patch's middle gets a shadow")
	}
}
