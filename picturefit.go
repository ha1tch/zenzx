// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// Where the picture goes when the window is not the size the display asked for (T-75).
//
// The display sizes the window to the picture, a whole-number zoom of it, and draws the picture
// to fill what it thinks the window is. That holds only while the system gives the window the
// size it was asked for. A system may not: macOS keeps a window inside the usable part of the
// screen, below the menu bar and above the Dock, so a zoom that fits the monitor can be taller
// than the window it gets; and a window dragged to another size stays there when the request to
// put it back is not taken. The picture is then cropped, or shown with a black area beside it,
// and not in the middle.
//
// This file is the arithmetic of putting it right, kept apart from the window so that it can be
// tested at any size and any HiDPI factor: the size the window really has in drawing units, the
// largest whole zoom that fits it, and the middle of the picture in it. The drawing is
// display_fit_gui.go.

// pictureSettleFrames is how many frames in a row the window must have a size other than the
// one asked for before the display stops waiting for it. A window that is on its way to a new
// size shows other sizes for a frame or two, and is not touched while it does.
const pictureSettleFrames = 3

// windowDrawSize is the window's size in the units the display draws in, from the size of its
// framebuffer in pixels and the factor between the two: 1 on an ordinary display, 2 on a
// retina one, where the framebuffer has twice the pixels the window has points. A factor below 1
// is not one.
func windowDrawSize(renderW, renderH int, dpiX, dpiY float32) (w, h int) {
	if dpiX < 1 {
		dpiX = 1
	}
	if dpiY < 1 {
		dpiY = 1
	}
	return int(float32(renderW)/dpiX + 0.5), int(float32(renderH)/dpiY + 0.5)
}

// fitZoom is the largest whole zoom, from minZoom to maxZoom, at which a picture of nativeW by
// nativeH under a bar of top units fits a window of winW by winH. When even minZoom does not
// fit it is minZoom: the picture is scaled down to the window then (centrePicture).
func fitZoom(winW, winH, nativeW, nativeH, top, minZoom, maxZoom int) int {
	if nativeW <= 0 || nativeH <= 0 {
		return minZoom
	}
	for z := maxZoom; z > minZoom; z-- {
		if nativeW*z <= winW && nativeH*z+top <= winH {
			return z
		}
	}
	return minZoom
}

// pictureRect is where the picture goes in the window, and the scale it is drawn at relative to
// the size it was made at.
type pictureRect struct {
	X, Y, W, H float32
	Scale      float32
}

// centrePicture puts a picture of picW by picH, made for a window of that size under a bar of
// top units, in the middle of a window of winW by winH: the same margin to its left and right,
// and the same above and below it, below the bar. A picture that does not fit is scaled down,
// the same each way, until it does; one that fits is not scaled up, because the zoom is for
// that, and is placed on whole units so that its pixels stay sharp.
func centrePicture(winW, winH, top, picW, picH int) pictureRect {
	availH := winH - top
	scale := float32(1)
	if picW > 0 && float32(winW) < float32(picW) {
		scale = float32(winW) / float32(picW)
	}
	if picH > 0 && float32(availH)/float32(picH) < scale {
		scale = float32(availH) / float32(picH)
	}
	if scale < 0 {
		scale = 0
	}
	w, h := float32(picW)*scale, float32(picH)*scale
	r := pictureRect{W: w, H: h, Scale: scale}
	if scale == 1 {
		r.X = float32((winW - picW) / 2)
		r.Y = float32(top + (availH-picH)/2)
		return r
	}
	r.X = (float32(winW) - w) / 2
	r.Y = float32(top) + (float32(availH)-h)/2
	return r
}

// sizeWatch counts the frames a window has stayed at one size other than the asked for.
type sizeWatch struct{ w, h, frames int }

// settled is told the window's size on a frame on which it is not the asked-for one, and says
// whether it has been this size for pictureSettleFrames frames running.
func (s *sizeWatch) settled(w, h int) bool {
	if w != s.w || h != s.h {
		s.w, s.h, s.frames = w, h, 0
	}
	s.frames++
	return s.frames >= pictureSettleFrames
}

// reset forgets the size: the window is the asked-for one, or is being resized on purpose.
func (s *sizeWatch) reset() { *s = sizeWatch{} }
