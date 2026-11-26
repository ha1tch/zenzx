//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Full screen. A window manager on macOS and Linux has a full screen of its own; Windows has
// none for a program that does not make it, which is this. raylib's ToggleBorderlessWindowed
// puts the window, without its border, over the whole of the monitor it is on, at the mode that
// monitor already has (GLFW's glfwSetWindowMonitor with the current video mode), so the desktop's
// resolution does not change, and puts it back where and as large as it was.
//
// While it lasts the window is a size the system gave it, which the display already copes with
// (T-75, display_fit_gui.go): the picture is fitted to it at the largest whole zoom that fits and
// centred, with the frame round it. What would ask the window for a size, a drag taken as a
// request to snap back, the animation to a zoom step, the zoom from the menu, does not.

// Fullscreen reports whether the window is full screen.
func (dm *DisplayManager) Fullscreen() bool { return dm.fullscreen }

// ToggleFullscreen enters full screen, or leaves it.
func (dm *DisplayManager) ToggleFullscreen() { dm.SetFullscreen(!dm.fullscreen) }

// SetFullscreen enters or leaves full screen. A no-op where it already is as asked.
func (dm *DisplayManager) SetFullscreen(on bool) {
	if on == dm.fullscreen {
		return
	}
	if on {
		dm.windowedMultiplier = dm.screen.multiplier
		dm.windowedW, dm.windowedH = dm.currentWidth, dm.currentHeight
		dm.isAnimating = false // a window on its way to a size is stopped where it is
		dm.fullscreen = true
		rl.ToggleBorderlessWindowed()
		dm.followFullscreen()
		fmt.Println("Full screen: on")
		return
	}
	rl.ToggleBorderlessWindowed()
	dm.fullscreen = false
	// The window is back at the size it had, which the display takes as its own before sending it
	// to the zoom it had: from there, not from the monitor's size, so that it does not grow to
	// the monitor and shrink again.
	dm.screen.multiplier = dm.windowedMultiplier
	dm.syncCurrentToWindow(dm.windowedW, dm.windowedH)
	dm.fit.watch.reset()
	dm.fit.adapted = [2]int{}
	dm.updateTargetSize()
	fmt.Println("Full screen: off")
}

// chosenMultiplier is the zoom the person has chosen, for the settings: while the window is full
// screen the zoom is the one fitted to the monitor, and what they chose is the one that comes
// back.
func (dm *DisplayManager) chosenMultiplier() int {
	if dm.fullscreen {
		return dm.windowedMultiplier
	}
	return dm.screen.multiplier
}

// followFullscreen is updateTargetSize for a full screen window: what is to be drawn has changed
// (the border, a layout, a machine's mode) and the window has not, so the zoom is fitted again
// to it, from the picture's size now.
func (dm *DisplayManager) followFullscreen() {
	dm.fit.watch.reset()
	dm.fit.adapted = [2]int{}
	dm.windowKept = false
	dm.borderApplied = dm.borderView().Shown
	dm.isAnimating = false
	if dm.tiles != nil {
		dm.fillWindowWithTiles()
		return
	}
	dm.adoptPictureSize()
}

// holdFullscreen is UpdateWindowSize while the window is full screen: nothing is animated, and a
// tiled layout is the size of the window it is in.
func (dm *DisplayManager) holdFullscreen() {
	dm.isAnimating = false
	if dm.tiles != nil {
		dm.fillWindowWithTiles()
	}
}

// fillWindowWithTiles makes the display's idea of its window the window's own size, which is
// what the tiles are placed in (tileRects): they keep a machine's proportions and fill it.
func (dm *DisplayManager) fillWindowWithTiles() {
	if w, h := drawableSize(); w > 0 && h > 0 {
		dm.currentWidth, dm.currentHeight = w, h
	}
}
