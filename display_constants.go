package main

// Shared display constants, raylib-free, compiled into both GUI and headless
// builds. The GUI renderer (display.go) and the headless framebuffer decoder
// (display_headless.go) both depend on these.
const (
	// Display dimensions
	ScreenWidth   = 256
	ScreenHeight  = 192
	MaxMultiplier = 5

	// Border dimensions in Spectrum pixels
	BorderLeft   = 32
	BorderRight  = 32
	BorderTop    = 24
	BorderBottom = 32

	// Total display area including border
	TotalWidth  = ScreenWidth + BorderLeft + BorderRight  // 320
	TotalHeight = ScreenHeight + BorderTop + BorderBottom // 248

	// Timing constants for border effects
	ScanlinesPerFrame = 312
	CyclesPerScanline = 224

	// ZX Spectrum Colors
	ZX_BLACK   = 0
	ZX_BLUE    = 1
	ZX_RED     = 2
	ZX_MAGENTA = 3
	ZX_GREEN   = 4
	ZX_CYAN    = 5
	ZX_YELLOW  = 6
	ZX_WHITE   = 7
	ZX_BRIGHT  = 1
	ZX_DIM     = 0
)
