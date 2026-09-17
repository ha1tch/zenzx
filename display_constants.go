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

	// RefWidth/RefHeight (2026-09-15, T-29 GUI-wiring pass) are the
	// window's permanent client-area size whenever the standard 256x192
	// video renderer is active, REPLACING TotalWidth/TotalHeight for
	// window-sizing purposes (TotalWidth/TotalHeight above are kept as
	// pure documentation of the classic screen+border geometry, still
	// read by nothing else in this codebase). The goal, per Horacio's own
	// framing: the window must never shapeshift just because a Next
	// program switches Layer 2 resolution mode at runtime -- only an
	// explicit user zoom (ScaleUp/ScaleDown/SetScale) changes the window
	// size. RefWidth stays 320 (TotalWidth unchanged: 320-wide Layer 2
	// mode's own 320 columns already fit the classic border's total
	// width exactly, with zero horizontal margin left over in that mode).
	// RefHeight grows from TotalHeight's 248 to 256 -- the smallest value
	// that fits 320x256 mode's real 256 rows without cropping -- so it is
	// 8px taller than pre-existing classic-only window height. That 8px
	// is permanent, not conditional on Layer 2 being enabled: DisplayManager.
	// totalSize() (display.go) returns these constants unconditionally
	// for the standard renderer rather than switching between two frame
	// sizes depending on mode, which is what "never shapeshifts" requires.
	// The classic 256x192 picture's own border margins (BorderTop/Bottom
	// above) are consequently increased by 4px each (not 8 on one side)
	// to consume the extra height symmetrically -- see
	// DisplayManager.classicScreenOffset in display.go, which is the only
	// place that extra 4px top/bottom is actually applied; BorderTop/
	// BorderBottom themselves are intentionally left unchanged here so
	// every other reader of those two constants (there are none outside
	// this file today, but the constants document real 1985 hardware
	// values and should keep meaning that) is unaffected.
	//
	// Non-standard renderers (e.g. hicolourVideoRenderer) do not use
	// RefWidth/RefHeight at all -- Layer 2/sprites only ever run
	// alongside the standard renderer in this codebase, so
	// DisplayManager.totalSize() falls back to its pre-existing
	// screenW/screenH+border computation for any other renderer,
	// unchanged.
	RefWidth  = TotalWidth      // 320 -- unchanged; already exact for 320-wide Layer 2 mode
	RefHeight = TotalHeight + 8 // 256 -- fits 320x256 mode's real height with 0 spare

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
