//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
)

// A full screen window is not asked for a size, and the zoom it is fitted to is not the person's
// choice. No window is opened: the state is set as SetFullscreen leaves it.
func TestFullscreenAsksForNoSizeAndKeepsTheChosenZoom(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	screen.multiplier = 5 // what the window was fitted to
	dm := NewDisplayManager(screen)
	dm.fullscreen, dm.windowedMultiplier = true, 2
	dm.isAnimating = true

	if got := dm.chosenMultiplier(); got != 2 {
		t.Errorf("chosenMultiplier in full screen = %d, want the zoom the person had (2)", got)
	}
	if dm.SetScale(3) || dm.screen.multiplier != 5 {
		t.Errorf("SetScale in full screen changed the zoom to %d, want it refused", dm.screen.multiplier)
	}
	dm.UpdateWindowSize()
	if dm.isAnimating {
		t.Error("UpdateWindowSize left the window animating to a size in full screen")
	}
	dm.HandleSingleResize() // must not reach for the window, which there is none of here

	dm.fullscreen = false
	if got := dm.chosenMultiplier(); got != 5 {
		t.Errorf("chosenMultiplier outside full screen = %d, want the display's own (5)", got)
	}
}

// The View menu has the row, ticked while the window is full screen, and the zoom rows are greyed
// then: the zoom is fitted to the monitor.
func TestViewMenuFullScreenRow(t *testing.T) {
	zx := testZX()
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.refreshViewItems(zx)
	items := b.widget.ItemsFor(barView)
	if items[3].Label != "Full Screen" || items[3].Checked == nil || *items[3].Checked {
		t.Fatalf("View item 3 is %q (checked %v), want Full Screen, not ticked", items[3].Label, items[3].Checked)
	}
	zx.window().fullscreen = true // as SetFullscreen leaves it, without a window
	defer func() { zx.window().fullscreen = false }()
	b.refreshViewItems(zx)
	items = b.widget.ItemsFor(barView)
	if !*items[3].Checked {
		t.Error("Full Screen is not ticked while the window is full screen")
	}
	for i := 7; i < 10; i++ { // X 1 to X 3
		if !items[i].Disabled {
			t.Errorf("%s is offered in full screen", items[i].Label)
		}
	}
}

// TestGLFullscreen is full screen in a window: the window covers the monitor, the picture is
// whole and fitted to it, the zoom the person chose is not changed and comes back, and the window
// is the size it was. Part of dormant guard G-22 (docs/KNOWN_ISSUES.md); it opens a window:
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLFullscreen -v .
func TestGLFullscreen(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-22)")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	const top = 24
	const mult = 2
	asked := [2]int{RefWidth * mult, RefHeight*mult + top}
	rl.SetTraceLogLevel(rl.LogNone)
	rl.SetConfigFlags(rl.FlagWindowResizable)
	rl.InitWindow(int32(asked[0]), int32(asked[1]), "ZenZX full screen")
	defer rl.CloseWindow()
	monW, monH := int(rl.GetMonitorWidth(rl.GetCurrentMonitor())), int(rl.GetMonitorHeight(rl.GetCurrentMonitor()))
	if monW < asked[0]+100 || monH < asked[1]+100 {
		t.Skipf("the monitor is %dx%d, too small to tell a window from a full screen", monW, monH)
	}

	mem, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	screen.multiplier = mult
	dm := NewDisplayManager(screen)
	defer dm.CleanupTextures()
	dm.SetVideoRenderer(standardVideoRenderer{})
	dm.SetSpectrumIO(io)
	dm.reservedTopHeight = top
	dm.currentWidth, dm.currentHeight = asked[0], asked[1]
	dm.targetWidth, dm.targetHeight = asked[0], asked[1]
	dm.borderColor = 2
	dm.InitializeAfterWindow()

	frames := func(n int) {
		for i := 0; i < n; i++ {
			dm.UpdateWindowSize()
			dm.HandleSingleResize() // as the main loop calls it
			dm.Render(false, mem, screen, io)
		}
	}
	frames(5)
	if w, h := drawableSize(); w != asked[0] || h != asked[1] {
		t.Fatalf("setting up: the window is %dx%d, want %dx%d", w, h, asked[0], asked[1])
	}

	dm.ToggleFullscreen()
	frames(60)
	if !dm.Fullscreen() || !rl.IsWindowState(rl.FlagBorderlessWindowedMode) {
		t.Fatalf("not full screen: display %v, raylib borderless %v", dm.Fullscreen(), rl.IsWindowState(rl.FlagBorderlessWindowedMode))
	}
	if w, h := drawableSize(); w != monW || h != monH {
		t.Errorf("the window is %dx%d in full screen, want the monitor's %dx%d", w, h, monW, monH)
	}
	nativeW, nativeH := dm.pictureNativeSize(dm.borderView().Shown)
	want := fitZoom(monW, monH, nativeW, nativeH, dm.frameTop(), dm.minMultiplierForMode(), MaxMultiplier)
	if dm.screen.multiplier != want {
		t.Errorf("the zoom in full screen is %d, want %d (the largest whole zoom that fits %dx%d)", dm.screen.multiplier, want, monW, monH)
	}
	if want <= mult {
		t.Errorf("the monitor %dx%d gives a zoom of %d, which does not test a larger one than %d", monW, monH, want, mult)
	}
	if !dm.windowKept || !dm.fit.canvasReady {
		t.Errorf("the picture is not fitted into the window: kept %v, canvas %v", dm.windowKept, dm.fit.canvasReady)
	}
	if dm.chosenMultiplier() != mult {
		t.Errorf("the settings would keep a zoom of %d, want the chosen %d", dm.chosenMultiplier(), mult)
	}
	if dm.SetScale(3) {
		t.Error("the zoom was changed in full screen")
	}
	if dm.isAnimating {
		t.Error("the window is being animated to a size in full screen")
	}

	dm.ToggleFullscreen()
	frames(60)
	if dm.Fullscreen() || rl.IsWindowState(rl.FlagBorderlessWindowedMode) {
		t.Fatalf("still full screen: display %v, raylib borderless %v", dm.Fullscreen(), rl.IsWindowState(rl.FlagBorderlessWindowedMode))
	}
	if w, h := drawableSize(); w != asked[0] || h != asked[1] {
		t.Errorf("after full screen the window is %dx%d, want it back at %dx%d", w, h, asked[0], asked[1])
	}
	if dm.screen.multiplier != mult || dm.windowKept {
		t.Errorf("after full screen the zoom is %d (kept %v), want %d and the window not kept", dm.screen.multiplier, dm.windowKept, mult)
	}

	// A tiled layout: the tiles are laid out in the window, so the display's size is the window's
	// in full screen, and the layout's own size comes back.
	tw, th := 2*RefWidth, RefHeight
	dm.setTiles(tw, th, func() {})
	frames(80)
	if dm.currentWidth != tw || dm.currentHeight != th+top {
		t.Fatalf("setting up: the tiled window is %dx%d, want %dx%d", dm.currentWidth, dm.currentHeight, tw, th+top)
	}
	dm.ToggleFullscreen()
	frames(40)
	if dm.currentWidth != monW || dm.currentHeight != monH || dm.isAnimating {
		t.Errorf("tiled in full screen the display is %dx%d (animating %v), want the monitor's %dx%d", dm.currentWidth, dm.currentHeight, dm.isAnimating, monW, monH)
	}
	dm.ToggleFullscreen()
	frames(80)
	if w, h := drawableSize(); w != tw || h != th+top || dm.currentWidth != tw || dm.currentHeight != th+top {
		t.Errorf("after full screen the tiled window is %dx%d (display %dx%d), want %dx%d", w, h, dm.currentWidth, dm.currentHeight, tw, th+top)
	}
}
