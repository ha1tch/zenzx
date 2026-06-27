//go:build headless

package main

import (
	"image"
	"image/color"
	"time"
)

// ============================================================================
// Headless display
//
// The GUI build (display.go) keeps the Spectrum framebuffer in GPU textures
// optimised for fast rendering. The headless build needs none of that: the
// ZX Spectrum display file is the source of truth, and the memory layer
// mirrors it into SpectrumScreen.bitmap (6144 bytes, 0x4000-0x57FF) and
// SpectrumScreen.attributes (768 bytes, 0x5800-0x5AFF) on every write.
//
// This file provides:
//   - a raylib-free SpectrumScreen / DisplayManager with exactly the fields
//     and methods the emulation core touches, and
//   - DecodeRGBA, which translates the display file into a standard
//     image.RGBA suitable for PNG export.
//
// The 256x192 pixel area only is decoded; the border is not part of the
// display file and is omitted from headless screenshots.
// ============================================================================

// ZXPalette is the raylib-free equivalent of ZXPaletteRGBA in display.go.
// Indices 0-7 are the normal (DIM) colours, 8-15 the BRIGHT variants. The
// RGB values match the GUI palette exactly (0xC8 for dim, 0xFF for bright).
var ZXPalette = [16]color.RGBA{
	{0x00, 0x00, 0x00, 0xff}, // Black
	{0x00, 0x00, 0xc8, 0xff}, // Blue
	{0xc8, 0x00, 0x00, 0xff}, // Red
	{0xc8, 0x00, 0xc8, 0xff}, // Magenta
	{0x00, 0xc8, 0x00, 0xff}, // Green
	{0x00, 0xc8, 0xc8, 0xff}, // Cyan
	{0xc8, 0xc8, 0x00, 0xff}, // Yellow
	{0xc8, 0xc8, 0xc8, 0xff}, // White
	{0x00, 0x00, 0x00, 0xff}, // Bright Black
	{0x00, 0x00, 0xff, 0xff}, // Bright Blue
	{0xff, 0x00, 0x00, 0xff}, // Bright Red
	{0xff, 0x00, 0xff, 0xff}, // Bright Magenta
	{0x00, 0xff, 0x00, 0xff}, // Bright Green
	{0x00, 0xff, 0xff, 0xff}, // Bright Cyan
	{0xff, 0xff, 0x00, 0xff}, // Bright Yellow
	{0xff, 0xff, 0xff, 0xff}, // Bright White
}

// SpectrumScreen holds the display file. Field names and types match the
// subset of the GUI SpectrumScreen that the memory layer and emulation core
// read and write: bitmap, attributes, flashEnabled, flashTickTock.
type SpectrumScreen struct {
	bitmap        []byte
	attributes    []byte
	multiplier    int
	flashTickTock bool
	flashEnabled  bool
	lastFlashTime time.Time
}

// NewSpectrumScreen allocates the 6144-byte bitmap and 768-byte attribute
// buffers, matching the GUI constructor's sizes and defaults.
func NewSpectrumScreen() *SpectrumScreen {
	return &SpectrumScreen{
		bitmap:        make([]byte, 6144),
		attributes:    make([]byte, 768),
		multiplier:    2,
		flashEnabled:  true,
		lastFlashTime: time.Now(),
	}
}

// updateFlash advances the flash phase on the standard ~320ms cadence. The
// headless path does not call this on a timer, but it is provided so flash
// state can be driven deterministically if a caller wants it.
func (s *SpectrumScreen) updateFlash() {
	if !s.flashEnabled {
		return
	}
	if time.Since(s.lastFlashTime) >= 320*time.Millisecond {
		s.flashTickTock = !s.flashTickTock
		s.lastFlashTime = time.Now()
	}
}

// calcByteOffset maps a screen pixel (x,y) to its byte index within the
// 6144-byte bitmap, honouring the Spectrum's thirds-interleaved layout.
// Identical arithmetic to the GUI SpectrumScreen.calcByteOffset.
func (s *SpectrumScreen) calcByteOffset(x, y int) int {
	yOffset := y & 0x07
	xOffset := x / 8
	rowOffset := (y / 8) % 8
	blockOffset := y / 64
	return blockOffset*2048 + rowOffset*32 + yOffset*256 + xOffset
}

// DecodeRGBA translates the current display file into a 256x192 image.RGBA.
//
// For each pixel it reads the bitmap bit and the 8x8 attribute cell (ink in
// bits 0-2, paper in bits 3-5, bright in bit 6, flash in bit 7). When flash
// is set and the flash phase is on, ink and paper are swapped, matching the
// GUI renderer's steady-state behaviour.
func (s *SpectrumScreen) DecodeRGBA() *image.RGBA {
	const w, h = ScreenWidth, ScreenHeight // 256 x 192
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		// Attribute row is one per 8 pixel rows: 32 cells across.
		attrRow := (y / 8) * 32
		for x := 0; x < w; x++ {
			bitOffset := s.calcByteOffset(x, y)
			b := s.bitmap[bitOffset]
			pixelOn := (b>>(7-uint(x%8)))&1 == 1

			attr := s.attributes[attrRow+x/8]
			ink := attr & 0x07
			paper := (attr >> 3) & 0x07
			bright := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			if flash == 1 && s.flashTickTock {
				ink, paper = paper, ink
			}

			var idx uint8
			if pixelOn {
				idx = ink
			} else {
				idx = paper
			}
			if bright == 1 {
				idx += 8
			}
			img.SetRGBA(x, y, ZXPalette[idx])
		}
	}
	return img
}

// ============================================================================
// Headless DisplayManager
//
// The core calls Render, SetBorderChanges and UpdateWindowSize once per frame.
// In a headless build these are no-ops: there is no window, and the border is
// not part of the display file. Border changes are still accepted (and the
// most recent colour retained) so callers that inspect border state continue
// to work.
// ============================================================================

type DisplayManager struct {
	screen        *SpectrumScreen
	borderColor   uint8
	borderChanges []BorderChange
	currentCycle  uint64
}

func NewDisplayManager(screen *SpectrumScreen) *DisplayManager {
	return &DisplayManager{
		screen:        screen,
		borderChanges: make([]BorderChange, 0),
	}
}

// Render is a no-op in the headless build. Screenshot capture is driven
// explicitly by the headless main via SpectrumScreen.DecodeRGBA.
func (dm *DisplayManager) Render(paused bool) {}

// SetBorderChanges records the frame's border history. Retained for parity
// with the GUI manager; the headless build does not draw the border.
func (dm *DisplayManager) SetBorderChanges(changes []BorderChange, currentCycle uint64) {
	dm.borderChanges = changes
	dm.currentCycle = currentCycle
	if n := len(changes); n > 0 {
		dm.borderColor = changes[n-1].Color
	}
}

// UpdateWindowSize is a no-op in the headless build.
func (dm *DisplayManager) UpdateWindowSize() {}

// SetAudioManager is a no-op in the headless build. The GUI manager uses the
// audio reference only for its debug overlay, which headless does not render.
func (dm *DisplayManager) SetAudioManager(audio *AudioWrapper) {}
