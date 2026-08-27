package main

import (
	"os"
	"time"
)

// ============================================================================
// SpectrumScreen: display-file storage
//
// This is the ZX Spectrum's display file -- 6144 bytes of bitmap plus 768
// bytes of attributes -- and nothing else. memory.go reads and writes these
// slices directly for the standard screen address ranges (0x4000-0x57FF /
// 0x5800-0x5AFF on bank 5, 0xC000-0xD7FF / 0xD800-0xDAFF on bank 7), so this
// type IS the backing store, not a cache of it.
//
// Turning bytes into pixels is a separate concern, handled by whichever
// VideoRenderer is active (see videorender.go) -- this file has no
// rendering logic and no GUI/raylib dependency, so it compiles identically
// into both the GUI and headless builds. Before this refactor, an
// equivalent (but not identical) type was duplicated per build tag in
// display.go and display_headless.go; GUI-only rendering resources (bit
// pattern textures, the border texture) now live on the GUI's
// DisplayManager instead, where they belong.
// ============================================================================

// SpectrumScreen holds the display file plus the two pieces of state a
// renderer needs alongside it: the current FLASH phase, and the live-display
// scale factor (also read by DisplayManager for window sizing).
type SpectrumScreen struct {
	bitmap        []byte
	attributes    []byte
	multiplier    int
	flashTickTock bool
	flashEnabled  bool
	lastFlashTime time.Time
}

// NewSpectrumScreen allocates the 6144-byte bitmap and 768-byte attribute
// buffers.
func NewSpectrumScreen() *SpectrumScreen {
	return &SpectrumScreen{
		bitmap:        make([]byte, 6144),
		attributes:    make([]byte, 768),
		multiplier:    2,
		flashEnabled:  true,
		lastFlashTime: time.Now(),
	}
}

// updateFlash advances the FLASH phase on the standard ~320ms cadence. The
// GUI's live render path calls this once per frame; headless screenshot
// capture does not call it automatically, so a headless run's FLASH phase
// only changes if something else in the run advances wall-clock time and a
// caller invokes this explicitly (matching this codebase's behaviour before
// this refactor).
func (s *SpectrumScreen) updateFlash() {
	if !s.flashEnabled {
		return
	}
	now := time.Now()
	if now.Sub(s.lastFlashTime) >= 320*time.Millisecond {
		s.flashTickTock = !s.flashTickTock
		s.lastFlashTime = now
	}
}

// calcByteOffset maps a screen pixel (x,y) to its byte index within the
// 6144-byte bitmap, honouring the Spectrum's thirds-interleaved layout.
func (s *SpectrumScreen) calcByteOffset(x, y int) int {
	yOffset := y & 0x07
	xOffset := x / 8
	rowOffset := (y / 8) % 8
	blockOffset := y / 64
	return blockOffset*2048 + rowOffset*32 + yOffset*256 + xOffset
}

// LoadFromFile reads a standard 6912-byte .scr dump (6144 bytes of bitmap
// followed by 768 bytes of attributes) directly into the display file.
func (s *SpectrumScreen) LoadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Read(s.bitmap)
	file.Read(s.attributes)
	return nil
}

// Clear blanks the display file: an empty bitmap, black ink on white paper
// throughout.
func (s *SpectrumScreen) Clear() {
	for i := range s.bitmap {
		s.bitmap[i] = 0
	}
	for i := range s.attributes {
		s.attributes[i] = 0x38 // Black ink on white paper
	}
}

// SetMultiplier and GetMultiplier control the live-display scale factor.
// Headless builds carry this field for shape parity but never read it.
func (s *SpectrumScreen) SetMultiplier(mult int) {
	if mult >= 1 && mult <= MaxMultiplier {
		s.multiplier = mult
	}
}

func (s *SpectrumScreen) GetMultiplier() int {
	return s.multiplier
}

// ToggleFlash and IsFlashEnabled control whether FLASH is honoured at all.
func (s *SpectrumScreen) ToggleFlash() {
	s.flashEnabled = !s.flashEnabled
}

func (s *SpectrumScreen) IsFlashEnabled() bool {
	return s.flashEnabled
}
