//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The picture in a window that is not the size the display asked for (T-75; the arithmetic is
// picturefit.go).
//
// While the window is the size the display asked for, nothing here does anything: the picture is
// drawn straight to the window as it always was. When the system gives the window another size
// and keeps it for a few frames (macOS keeps a window inside the usable part of the screen, and
// a window dragged to a size does not always go back when asked), the display first asks for the
// zoom that fits what it really has, and where the window stays another size the picture is
// drawn into a canvas of the size the display thinks the window is and that canvas is put in the
// middle of the window, scaled down when the window is smaller than it, so that it is whole.

// pictureFit is what the display keeps of that.
type pictureFit struct {
	watch   sizeWatch
	adapted [2]int // the window size the zoom was last fitted to

	canvas      rl.RenderTexture2D
	canvasReady bool
	canvasW     int
	canvasH     int
}

// pictureNativeSize is the picture's size at a zoom of one: what the window is a whole zoom of.
func (dm *DisplayManager) pictureNativeSize(showBorder bool) (w, h int) {
	if showBorder {
		return dm.totalSize()
	}
	return dm.screenW, dm.screenH
}

// drawableSize is the window's size in drawing units as the framebuffer has it. Not
// GetScreenWidth: SetWindowSize sets that to the size asked for whether the system gave it or
// not.
func drawableSize() (int, int) {
	dpi := rl.GetWindowScaleDPI()
	return windowDrawSize(int(rl.GetRenderWidth()), int(rl.GetRenderHeight()), dpi.X, dpi.Y)
}

// pictureCanvasWanted is asked once a frame before the picture is drawn. It reports whether the
// picture goes through the canvas, and fits the zoom to the window when the window has settled
// at a size that is not the one asked for.
func (dm *DisplayManager) pictureCanvasWanted() bool {
	f := &dm.fit
	if dm.tiles != nil || dm.isAnimating {
		f.watch.reset() // a tiled layout places its own tiles, and the window is on its way to its size
		return false
	}
	w, h := drawableSize()
	if w <= 0 || h <= 0 || (w == dm.currentWidth && h == dm.currentHeight) {
		f.watch.reset()
		f.adapted = [2]int{}
		return false
	}
	if !f.watch.settled(w, h) {
		return false
	}
	if f.adapted != [2]int{w, h} {
		f.adapted = [2]int{w, h} // once for a size: a system that will not resize does not get asked in a loop
		nativeW, nativeH := dm.pictureNativeSize(dm.borderView().Shown)
		z := fitZoom(w, h, nativeW, nativeH, dm.reservedTopHeight, dm.minMultiplierForMode(), MaxMultiplier)
		if z != dm.screen.multiplier {
			dm.screen.multiplier = z
			dm.updateTargetSize()
			fmt.Printf("Scale: %dx (the window is %dx%d)\n", z, w, h)
			return false
		}
	}
	return true
}

// beginPictureCanvas starts drawing the picture into the canvas, which is as large as the window
// the display thinks it has, so that everything is drawn where it always is.
func (dm *DisplayManager) beginPictureCanvas() {
	f := &dm.fit
	if !f.canvasReady || f.canvasW != dm.currentWidth || f.canvasH != dm.currentHeight {
		dm.unloadPictureCanvas()
		f.canvas = rl.LoadRenderTexture(int32(dm.currentWidth), int32(dm.currentHeight))
		rl.SetTextureFilter(f.canvas.Texture, rl.FilterBilinear) // exact at one to one, smooth when scaled down
		f.canvasReady, f.canvasW, f.canvasH = true, dm.currentWidth, dm.currentHeight
	}
	rl.BeginTextureMode(f.canvas)
	rl.ClearBackground(rl.Black)
}

// endPictureCanvas ends the canvas and draws it in the middle of the window, the picture's
// margins the same on every side and nothing cut off. The bar's band at the top of the canvas is
// black and goes with it; the bar itself is drawn over the window afterwards.
func (dm *DisplayManager) endPictureCanvas() {
	f := &dm.fit
	rl.EndTextureMode()
	w, h := drawableSize()
	top := dm.reservedTopHeight
	r := centrePicture(w, h, top, dm.currentWidth, dm.currentHeight-top)
	rl.ClearBackground(rl.Black)
	dest := rl.Rectangle{
		X: r.X, Y: r.Y - float32(top)*r.Scale,
		Width: float32(dm.currentWidth) * r.Scale, Height: float32(dm.currentHeight) * r.Scale,
	}
	// A render texture is stored upside down: a negative source height turns it over.
	rl.DrawTexturePro(f.canvas.Texture,
		rl.Rectangle{X: 0, Y: 0, Width: float32(dm.currentWidth), Height: -float32(dm.currentHeight)},
		dest, rl.Vector2{}, 0, rl.White)
}

func (dm *DisplayManager) unloadPictureCanvas() {
	if dm.fit.canvasReady {
		rl.UnloadRenderTexture(dm.fit.canvas)
		dm.fit.canvasReady = false
	}
}
