// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "math"

// The frame around the picture, where the window is larger than the picture and the picture is
// centred in it (picturefit.go, T-75).
//
// The frame is a vertical gradient, dark grey under the menu bar and a darker grey at the bottom of the
// window, crossed by five faint beveled lines, and the picture casts a soft shadow on it. The shadow is drawn from one small texture,
// made on the processor once and kept on the graphics card: a frame costs one gradient rectangle
// and nine textured quads, and nothing is computed per frame. The arithmetic is here, apart from
// the window, so that it can be tested without one; the drawing is pictureframe_gui.go.

const (
	// frameTopGrey is the grey at the top of the frame: 80% grey in the printer's sense, 80% of
	// the way from white to black, which is 20% of white. (It was 70%, 30% of white; the
	// frame was made 10 points darker.)
	frameTopGrey = 51

	// frameBottomGrey is the grey at the bottom of the frame: 90% grey, 10% of white. (It was
	// black; the frame was made 10 points lighter.)
	frameBottomGrey = 26

	// The shadow: a blur of shadowSigma pixels (the standard deviation of the Gaussian that the
	// picture's edge is blurred with), black at shadowPeak opacity, moved shadowDropY pixels down
	// and not sideways, as a light from above would.
	shadowSigma = 10.0
	shadowPeak  = 0.60
	shadowDropY = 6

	// shadowReach is how far the blur reaches beyond the picture's edge, in pixels: about three
	// and a half standard deviations, where a Gaussian edge is under a thousandth.
	shadowReach = 36
)

// shadowSide is the side of the shadow's texture, in pixels. The texture is a whole shadow of a
// picture 1 pixel across, cut in nine at shadowReach*2 from each edge (a nine-patch): the corners
// are drawn as they are and the middle, which is one flat colour, is stretched.
const shadowSide = 4*shadowReach + 1

// shadowEdge is the share of the shadow's full opacity at d pixels inside the edge of a box (d is
// negative outside it) that is blurred by a Gaussian of standard deviation sigma: the integral of
// the Gaussian up to d, half at the edge itself.
func shadowEdge(d, sigma float64) float64 {
	return 0.5 * (1 + math.Erf(d/(sigma*math.Sqrt2)))
}

// shadowProfile is the opacity, from 0 to 1, along one line through the nine-patch's texture,
// shadowSide values long: the edge of the box is at reach pixels in from the end, and the line is
// the same at the other end. A box blurred by a Gaussian is the product of this along x and along
// y at every pixel, so the texture is made of it twice.
func shadowProfile(reach int, sigma float64) []float64 {
	n := 4*reach + 1
	p := make([]float64, n)
	for i := 0; i <= 2*reach; i++ {
		p[i] = shadowEdge(float64(i-reach)+0.5, sigma) // the centre of pixel i
		p[n-1-i] = p[i]
	}
	return p
}

// shadowAlphas is the alpha of every pixel of the shadow's texture, row by row, shadowSide by
// shadowSide: the profile along x times the profile along y times the shadow's peak opacity.
func shadowAlphas() []uint8 {
	p := shadowProfile(shadowReach, shadowSigma)
	a := make([]uint8, shadowSide*shadowSide)
	for y := 0; y < shadowSide; y++ {
		for x := 0; x < shadowSide; x++ {
			a[y*shadowSide+x] = uint8(p[x]*p[y]*shadowPeak*255 + 0.5)
		}
	}
	return a
}

// shadowFits says whether a picture of picW by picH has room for the nine-patch: the middle of it
// is the picture less the corners, and has to be there. Smaller than that, no shadow is drawn.
func shadowFits(picW, picH float32) bool {
	return picW >= 2*shadowReach && picH >= 2*shadowReach
}

// The lines across the gradient: frameLines of them, evenly spaced between the bar and the
// bottom of the window, each a groove of two pixels, a dark one and a light one under it. The dark
// row is at frameLineDarkAlpha of 255, which is 28%, and the light row at frameLineAlpha, which is
// 18%. They are drawn over the gradient and under the shadow and the picture.
const (
	frameLines         = 5
	frameLineAlpha     = 46 // the light row: 18% of 255, to the nearest
	frameLineDarkAlpha = 71 // the dark row: 28% of 255, to the nearest
)

// frameLineYs are the rows of the dark half of each line in a window winH high under a bar of top
// units: the part below the bar is cut in frameLines+1 equal bands, and a line is where two meet,
// to a whole row. Each is followed by its light row. A window with no room for them (twelve rows
// or fewer under the bar) has none.
func frameLineYs(top, winH int) []int {
	room := winH - top
	if room <= 2*(frameLines+1) {
		return nil
	}
	ys := make([]int, frameLines)
	for i := range ys {
		ys[i] = top + (i+1)*room/(frameLines+1)
	}
	return ys
}
