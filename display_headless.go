//go:build headless

package main

// ============================================================================
// Headless display
//
// The GUI build (display.go) uploads the active VideoRenderer's output to
// one texture and draws it scaled. The headless build needs none of that:
// screenshot capture calls zx.DecodeDisplay() directly (see writeScreenPNG
// in scheduler.go) and PNG-encodes the result. This file provides only a
// no-op DisplayManager with the same method set as the GUI's, so untagged
// callers (zx.Render() in zenzx.go) can call it identically in either
// build.
//
// SpectrumScreen and the standard renderer's Decode logic used to be
// duplicated here (near-identically) and in display.go; both now live once,
// raylib-free, in screen.go and videorender.go.
// ============================================================================

type DisplayManager struct {
	screen         *SpectrumScreen
	borderColor    uint8
	borderChanges  []BorderChange
	frameOrigin    uint64
	cyclesPerFrame int
}

func NewDisplayManager(screen *SpectrumScreen) *DisplayManager {
	return &DisplayManager{
		screen:        screen,
		borderChanges: make([]BorderChange, 0),
	}
}

// Render is a no-op in the headless build. Screenshot capture is driven
// explicitly by the headless main and scheduler via zx.DecodeDisplay().
func (dm *DisplayManager) Render(paused bool, mem *SpectrumMemory, screen *SpectrumScreen) {}

// SetBorderChanges records the frame's border history. Retained for parity
// with the GUI manager; the headless build does not draw the border.
func (dm *DisplayManager) SetBorderChanges(changes []BorderChange, frameOrigin uint64, cyclesPerFrame int) {
	dm.borderChanges = changes
	dm.frameOrigin = frameOrigin
	dm.cyclesPerFrame = cyclesPerFrame
	if n := len(changes); n > 0 {
		dm.borderColor = changes[n-1].Color
	}
}

// UpdateWindowSize is a no-op in the headless build.
func (dm *DisplayManager) UpdateWindowSize() {}

// SetAudioManager is a no-op in the headless build. The GUI manager uses the
// audio reference only for its debug overlay, which headless does not render.
func (dm *DisplayManager) SetAudioManager(audio *AudioWrapper) {}

// SetVideoRenderer is a no-op in the headless build: there is no window or
// texture to resize. zx.DecodeDisplay() already calls the active renderer
// directly regardless of what DisplayManager knows.
func (dm *DisplayManager) SetVideoRenderer(r VideoRenderer) {}
