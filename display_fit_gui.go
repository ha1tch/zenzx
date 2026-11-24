//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"os"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The picture in a window that is not the size the display asked for (T-75; the arithmetic is
// picturefit.go).
//
// While the window is the size the display asked for, nothing here does anything: the picture is
// drawn straight to the window as it always was. When the system gives the window another size
// and keeps it for a few frames (macOS keeps a window inside the usable part of the screen, and
// a window dragged or maximised to a size does not always go back when asked), the display takes
// the zoom that fits what it really has (without asking the window to be that size: the window
// is left as it was given), and where the window is another size the picture is
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

	shadow frameShadow // the picture's shadow in the frame around it (pictureframe_gui.go)

	lastDebug string // the last line windowDebug printed
}

// windowDebug is set by ZENZX_WINDOW_DEBUG=1: the display then prints one line, each time it
// changes, of what the window reports and what the display makes of it. It is for seeing why a
// window is not framed and centred on a system that cannot be reached from where the code is
// written (T-75).
var windowDebug = os.Getenv("ZENZX_WINDOW_DEBUG") != ""

// debugWindow prints the line, when it differs from the last one.
func (dm *DisplayManager) debugWindow(canvas bool) {
	f := &dm.fit
	dw, dh := drawableSize()
	dpi := rl.GetWindowScaleDPI()
	line := fmt.Sprintf("window: drawable %dx%d  render %dx%d  screen %dx%d  dpi %.2fx%.2f  current %dx%d  target %dx%d  zoom %d  top %d  animating %v  tiled %v  settled %d  adapted %v  canvas %v",
		dw, dh, rl.GetRenderWidth(), rl.GetRenderHeight(), rl.GetScreenWidth(), rl.GetScreenHeight(), dpi.X, dpi.Y,
		dm.currentWidth, dm.currentHeight, dm.targetWidth, dm.targetHeight, dm.screen.multiplier, dm.reservedTopHeight,
		dm.isAnimating, dm.tiles != nil, f.watch.frames, f.adapted, canvas)
	if line != f.lastDebug {
		f.lastDebug = line
		fmt.Println(line)
	}
}

// frameTop is where the frame under the bar begins in the window: the space reserved for a
// fixed bar, or the bar as it is drawn when that is taller (barOverlay).
func (dm *DisplayManager) frameTop() int { return max(dm.reservedTopHeight, dm.barOverlay) }

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
func (dm *DisplayManager) pictureCanvasWanted() (canvas bool) {
	f := &dm.fit
	if windowDebug {
		defer func() { dm.debugWindow(canvas) }()
	}
	if dm.tiles != nil || dm.isAnimating {
		f.watch.reset() // a tiled layout places its own tiles, and the window is on its way to its size
		return false
	}
	w, h := drawableSize()
	if w <= 0 || h <= 0 || (w == dm.currentWidth && h == dm.currentHeight) {
		f.watch.reset()
		f.adapted = [2]int{}
		dm.windowKept = false // the window is the size the display has: nothing is being kept
		return false
	}
	if !f.watch.settled(w, h) {
		return false
	}
	if f.adapted != [2]int{w, h} {
		f.adapted = [2]int{w, h} // once for a size: a system that will not resize does not get asked in a loop
		nativeW, nativeH := dm.pictureNativeSize(dm.borderView().Shown)
		z := fitZoom(w, h, nativeW, nativeH, dm.frameTop(), dm.minMultiplierForMode(), MaxMultiplier)
		if z != dm.screen.multiplier {
			dm.screen.multiplier = z
			dm.adoptPictureSize()
			fmt.Printf("Scale: %dx (the window is %dx%d)\n", z, w, h)
		}
	}
	dm.windowKept = true
	return true
}

// adoptPictureSize makes the display's idea of its window the picture's size at the zoom it has
// now, without asking the window to be that size. The window is the size the person or the
// system gave it, and stays so: a window that was maximised is not shrunk back to the zoom (and
// on a system that keeps it maximised, asking would only leave the display and the window
// disagreeing about what the window is). What changes a window's size on purpose, a zoom chosen
// from the menu or the border switched, goes through updateTargetSize as it always did.
func (dm *DisplayManager) adoptPictureSize() {
	showBorder := dm.borderView().Shown
	dm.borderApplied = showBorder
	nativeW, nativeH := dm.pictureNativeSize(showBorder)
	dm.targetWidth = nativeW * dm.screen.multiplier
	dm.targetHeight = nativeH*dm.screen.multiplier + dm.reservedTopHeight
	dm.currentWidth, dm.currentHeight = dm.targetWidth, dm.targetHeight
	dm.isAnimating = false
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
// margins the same on every side and nothing cut off. The margins are the frame (drawFrame): a
// gradient and the picture's shadow. The bar's band at the top of the canvas is left out of what
// is drawn; the bar itself is drawn over the window afterwards.
func (dm *DisplayManager) endPictureCanvas() {
	f := &dm.fit
	rl.EndTextureMode()
	w, h := drawableSize()
	band := dm.reservedTopHeight // the canvas's own band at the top, left out of what is drawn
	top := dm.frameTop()
	r := centrePicture(w, h, top, dm.currentWidth, dm.currentHeight-band)
	dm.drawFrame(r, top, w, h)
	dest := rl.Rectangle{X: r.X, Y: r.Y, Width: r.W, Height: r.H}
	// A render texture is stored upside down: a negative source height turns it over, and takes
	// the rows from the bottom of the texture up, which is the picture under the bar's band.
	rl.DrawTexturePro(f.canvas.Texture,
		rl.Rectangle{X: 0, Y: 0, Width: float32(dm.currentWidth), Height: -float32(dm.currentHeight - band)},
		dest, rl.Vector2{}, 0, rl.White)
}

func (dm *DisplayManager) unloadPictureCanvas() {
	if dm.fit.canvasReady {
		rl.UnloadRenderTexture(dm.fit.canvas)
		dm.fit.canvasReady = false
	}
}
