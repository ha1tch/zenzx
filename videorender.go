package main

import (
	"fmt"
	"image"
	"image/color"
)

// ============================================================================
// Pluggable video rendering
//
// A VideoRenderer turns Spectrum display memory into a 256x192 picture. It
// is backend-agnostic: the GUI build uploads the result to one texture and
// draws it scaled (see DisplayManager.Render in display.go); the headless
// build encodes it straight to PNG (see writeScreenPNG in scheduler.go).
// Neither front end knows or cares which renderer is active -- they both
// call zx.DecodeDisplay(), which delegates to whichever VideoRenderer
// -ns-graphics selected at startup (see ZenZX.SelectVideoRenderer in
// zenzx.go).
//
// Before this refactor, the standard renderer's logic (DecodeRGBA) was
// duplicated near-identically in display.go and display_headless.go, and
// the GUI additionally carried a second, independent fast-path renderer
// (256 pre-baked bit-pattern textures blitted per character cell) that only
// knew how to draw the standard bitmap+attribute layout. Neither could have
// been swapped for an alternative mode without rewriting both front ends.
// This file is the seam: exactly one implementation of "bytes to pixels"
// per mode, callable identically from both builds.
//
// Registering a new mode's renderer (e.g. for T-09's mode-zenzx-01/02, or
// T-11's mode-timex-001-hicolour) means implementing VideoRenderer and
// calling RegisterVideoRenderer in an init() -- nothing in display.go,
// display_headless.go, or either main needs to change.
// ============================================================================

// VideoRenderer turns Spectrum display memory into a displayed picture.
type VideoRenderer interface {
	// Name is this renderer's registry key: "" for the standard renderer,
	// or the exact -ns-graphics value it implements (e.g.
	// NSGraphicsTimex001HiColour).
	Name() string
	// Decode reads whatever memory it needs -- through mem, through
	// screen, or both -- and returns a 256x192 image.RGBA ready to
	// display or save. mem is included even though the standard renderer
	// does not use it, because a renderer whose display memory does not
	// live on SpectrumScreen (e.g. a hi-colour screen-1 buffer, once T-11
	// decides where that lives) needs a way to read it.
	Decode(mem *SpectrumMemory, screen *SpectrumScreen) *image.RGBA

	// Dimensions reports the pixel size of images this renderer's Decode
	// returns. DisplayManager (GUI) and headless PNG output size
	// themselves from this rather than assuming 256x192, since not every
	// mode will be that size (see docs/TRACKING.md T-09's mode-zenzx-02,
	// 512x384).
	Dimensions() (width, height int)

	// BorderMargins reports this renderer's border thickness, in this
	// renderer's own output pixels, as (left, right, top, bottom). Border
	// is optional per mode: a renderer with no border returns all zeros,
	// and DisplayManager skips border compositing entirely in that case.
	BorderMargins() (left, right, top, bottom int)
}

var videoRenderers = map[string]VideoRenderer{}

// RegisterVideoRenderer adds a renderer to the registry, keyed by its own
// Name(). Panics on a duplicate name: two renderers claiming the same
// -ns-graphics value is always a programming error, not a runtime
// condition, and failing fast at init() time (rather than silently letting
// the second registration win) is safer than discovering it via a wrong
// picture at run time.
func RegisterVideoRenderer(r VideoRenderer) {
	name := r.Name()
	if _, exists := videoRenderers[name]; exists {
		panic(fmt.Sprintf("RegisterVideoRenderer: %q already registered", name))
	}
	videoRenderers[name] = r
}

// LookupVideoRenderer resolves the renderer for a -ns-graphics value ("" for
// standard). It errors rather than silently falling back to standard, so a
// valid-but-not-yet-implemented mode fails loudly at startup instead of
// quietly rendering the standard screen while the summary line claims
// something else is active.
func LookupVideoRenderer(graphicsMode string) (VideoRenderer, error) {
	r, ok := videoRenderers[graphicsMode]
	if !ok {
		return nil, fmt.Errorf("no video renderer registered for %q yet (see docs/TRACKING.md)", graphicsMode)
	}
	return r, nil
}

func init() {
	RegisterVideoRenderer(standardVideoRenderer{})
}

// ============================================================================
// Standard ZX Spectrum renderer
//
// FLASH is a standard-mode eccentricity: no other -ns-graphics mode
// supports it, by design (2026-08-17), regardless of what a
// mode's real hardware attribute format might technically permit -- e.g.
// hi-colour's attribute byte has a FLASH bit per docs/timex-modes.md, but
// this emulator's hi-colour renderer (T-11, not yet implemented) will not
// honour it. FLASH state (flashEnabled, flashTickTock, lastFlashTime) lives
// on SpectrumScreen only because that is the per-ZenZX-instance parameter
// already threaded into every Decode call, not because it is a generic
// part of the VideoRenderer contract -- a future renderer's Decode simply
// never reads those fields. screen.updateFlash() is called once per frame
// from the GUI's live path (zx.Render(), via DisplayManager) unconditionally
// regardless of which renderer is active; this is harmless (the fields sit
// unread by non-standard renderers) and preserves this codebase's existing
// asymmetry where headless screenshot capture never advances FLASH on its
// own. Border and border-stripe rendering are the opposite case: shared,
// optional infrastructure available to every mode via BorderMargins(),
// entirely independent of what Decode() draws inside it.
// ============================================================================

// zxPalette is the raylib-free 16-colour ZX Spectrum palette: indices 0-7
// are the normal (DIM) colours, 8-15 the BRIGHT variants.
var zxPalette = [16]color.RGBA{
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

// standardVideoRenderer decodes the standard 256x192, 32x24-attribute
// Spectrum display file. It reads only screen (bitmap + attributes +
// flashTickTock); mem is unused.
type standardVideoRenderer struct{}

func (standardVideoRenderer) Name() string { return "" }

// Dimensions: the standard Spectrum picture is always 256x192.
func (standardVideoRenderer) Dimensions() (int, int) { return ScreenWidth, ScreenHeight }

// BorderMargins: the standard 32/32/24/32-pixel border.
func (standardVideoRenderer) BorderMargins() (int, int, int, int) {
	return BorderLeft, BorderRight, BorderTop, BorderBottom
}

// Decode translates the display file into a 256x192 image.RGBA. For each
// pixel it reads the bitmap bit and the 8x8 attribute cell (ink in bits
// 0-2, paper in bits 3-5, bright in bit 6, flash in bit 7); when flash is
// set and the flash phase is on, ink and paper are swapped.
//
// This is byte-for-byte the same algorithm the pre-refactor GUI and
// headless builds each carried their own copy of (display.go's and
// display_headless.go's DecodeRGBA), unified into the one implementation
// both front ends now call.
func (standardVideoRenderer) Decode(mem *SpectrumMemory, screen *SpectrumScreen) *image.RGBA {
	const w, h = ScreenWidth, ScreenHeight // 256 x 192
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		attrRow := (y / 8) * 32
		for x := 0; x < w; x++ {
			b := screen.bitmap[screen.calcByteOffset(x, y)]
			pixelOn := (b>>(7-uint(x%8)))&1 == 1

			attr := screen.attributes[attrRow+x/8]
			ink := attr & 0x07
			paper := (attr >> 3) & 0x07
			bright := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			if screen.flashEnabled && flash == 1 && screen.flashTickTock {
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
			img.SetRGBA(x, y, zxPalette[idx])
		}
	}
	return img
}
