//go:build !headless

package zenzx

import (
	"fmt"
	"image"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ============================================================================
// Color Palette
//
// Used only by the border-stripe renderer below. The main 256x192 picture
// is decoded by the active VideoRenderer (videorender.go, raylib-free,
// shared with the headless build) into a plain image.RGBA; DisplayManager
// uploads that to a single texture rather than drawing it cell by cell.
// ============================================================================

var ZXPaletteRGBA = []rl.Color{
	rl.NewColor(0x00, 0x00, 0x00, 0xff), // Black
	rl.NewColor(0x00, 0x00, 0xc8, 0xff), // Blue
	rl.NewColor(0xc8, 0x00, 0x00, 0xff), // Red
	rl.NewColor(0xc8, 0x00, 0xc8, 0xff), // Magenta
	rl.NewColor(0x00, 0xc8, 0x00, 0xff), // Green
	rl.NewColor(0x00, 0xc8, 0xc8, 0xff), // Cyan
	rl.NewColor(0xc8, 0xc8, 0x00, 0xff), // Yellow
	rl.NewColor(0xc8, 0xc8, 0xc8, 0xff), // White
	rl.NewColor(0x00, 0x00, 0x00, 0xff), // Bright Black
	rl.NewColor(0x00, 0x00, 0xff, 0xff), // Bright Blue
	rl.NewColor(0xff, 0x00, 0x00, 0xff), // Bright Red
	rl.NewColor(0xff, 0x00, 0xff, 0xff), // Bright Magenta
	rl.NewColor(0x00, 0xff, 0x00, 0xff), // Bright Green
	rl.NewColor(0x00, 0xff, 0xff, 0xff), // Bright Cyan
	rl.NewColor(0xff, 0xff, 0x00, 0xff), // Bright Yellow
	rl.NewColor(0xff, 0xff, 0xff, 0xff), // Bright White
}

// ============================================================================
// Display Manager
// ============================================================================
type DisplayManager struct {
	screen           *SpectrumScreen
	showFPS          bool
	showBorder       bool
	showDebugOverlay bool // Debug histogram overlay
	borderColor      uint8
	// preEndDrawHook, if set, runs at the very end of Render, still inside
	// the BeginDrawing/EndDrawing bracket -- for anything (like the demo
	// overlay) that needs to draw on top of the frame Render just
	// produced. raylib draw calls outside that bracket don't land on the
	// frame actually presented, so this exists specifically so callers
	// don't have to duplicate Render's own Begin/End handling themselves.
	preEndDrawHook func()
	fps            int32
	currentWidth   int
	currentHeight  int
	targetWidth    int
	targetHeight   int
	// fit is what puts the picture in the middle of a window that is not the size asked for
	// (display_fit_gui.go).
	fit            pictureFit
	wait           *waitTextures // the figure that waits in a window with no machine (waitfigure_gui.go)
	isAnimating    bool
	borderChanges  []BorderChange
	frameOrigin    uint64
	cyclesPerFrame int // Set via SetBorderChanges; defaults to 0 until the first call, guarded in renderBorderStripes
	audio          *AudioWrapper

	// io (2026-09-15, T-29 GUI-wiring pass), set via SetSpectrumIO, is
	// used only by maxMultiplierThatFits/SetScale to clamp the minimum
	// zoom to 2x while Layer 2's 640x256x4bpp mode is active (that mode's
	// real width exceeds RefWidth's fixed 320-column reference canvas at
	// 1x, so 1x would either crop real picture data or force the window
	// to grow -- both rejected in favour of simply refusing 1x for this
	// one mode, matching the existing "not every zoom factor fits every
	// mode" precedent maxMultiplierThatFits already establishes for
	// monitor-size clamping). Render itself still takes its own io
	// parameter each frame, unrelated to this field -- this exists solely
	// because ScaleUp/ScaleDown/SetScale/maxMultiplierThatFits have no io
	// parameter of their own and every call site (input.go, menubar_gui.go)
	// already has zx.io in scope to pass here once, so a stored reference
	// avoids changing four public method signatures and their existing
	// test call sites for a value that changes only when NR 0x70 does.
	// nil (the pre-SetSpectrumIO / headless / classic-machine default)
	// means "no Layer 2 mode gate" -- unchanged pre-T-29 behaviour.
	io *SpectrumIO

	// reservedTopHeight is extra window-pixel space at the top,
	// reserved for a fixed (pinned, non-auto-hiding) menu bar so it
	// doesn't overlap the Spectrum display -- 0 normally. Set via
	// SetReservedTopHeight, which also triggers updateTargetSize so the
	// window animates to the new size the same way toggling the border
	// already does. Applied to the destination rect Render draws the
	// screen/border texture into (Y offset down by this much, Height
	// reduced by this much), not to the texture's own native size, so
	// the Spectrum content itself is shifted, not squished, once the
	// window has finished animating to its new size.
	reservedTopHeight int

	// barOverlay is how tall the fixed menu bar is drawn now, which is more than
	// reservedTopHeight where the window is larger than the display made it and the bar's
	// text has grown to suit (menubarscale.go): the reservation is worked out from the
	// display's own size, so that it, and the window with it, do not follow the window's
	// size. Where the picture is centred in a larger window it goes below the taller of the
	// two. 0 while the bar is not fixed.
	barOverlay int

	// windowKept is true while the window is a size that the person or the system gave it and
	// the display has taken as it is (display_fit_gui.go): the picture is fitted to it and
	// centred, and a change of the space reserved at the top or of the border follows the
	// picture's size without asking the window to be that size. Zooming from the menu asks for
	// a size, and ends it.
	windowKept bool

	// tiles, when set, draws the window's whole content below the reserved
	// top in place of one machine's picture: the tiled layouts of
	// machinedeck_gui.go. tilesW and tilesH are the size that content asks
	// of the window.
	tiles          func()
	tilesW, tilesH int

	// borderApplied is whether the border was shown the last time the window was sized
	// (updateTargetSize). borderView is the one derivation of that; UpdateWindowSize
	// re-sizes the window when the derivation and this disagree, so the window cannot be
	// left shaped for a border that is no longer there, or the reverse.
	borderApplied bool

	// GPU fast-path texture caches (videorender_gpu.go), baked once in
	// InitializeAfterWindow and freed in CleanupTextures. bitPatternGPU
	// holds one texture per possible byte value (which of 8 bits are
	// set) and paperColourGPU one per palette entry -- both are
	// universal concepts, not renderer-specific, so every FastGUIRenderer
	// implementation shares this same pair rather than baking its own.
	bitPatternGPU    [256]rl.Texture2D
	paperColourGPU   [16]rl.Texture2D
	gpuTexturesReady bool

	// spriteGPUCache is T-30 (wave ti0.r4)'s sprite fast-path cache
	// (videorender_gpu.go): a capped, LRU-evicted cache of baked sprite
	// textures keyed by (pattern, paletteOffset) -- see spriteGPUCache's
	// own doc comment for the full design and why a plain per-pattern-slot
	// cache (drafted and rejected first) can't express sprite attribute
	// 2's per-SPRITE palette offset correctly. Lazily populated by
	// renderSpritesGPU as sprites are actually drawn, not pre-baked here
	// in generateGPUTextures -- unlike bitPatternGPU/paperColourGPU (fixed
	// sets known in full at startup), the set of (pattern, paletteOffset)
	// combinations a running program will actually use isn't known ahead
	// of time. Initialised by NewDisplayManager at
	// defaultSpriteGPUCacheCapacity; SetSpriteGPUCacheCapacity changes it
	// at runtime.
	spriteGPUCache *spriteGPUCache

	// layer2ColourGPU is T-31's Layer 2 GPU fast-path cache
	// (videorender_gpu.go): one texture per possible 8-bit Next palette
	// index (0-255), baked lazily by renderLayer2GPU against whichever
	// of Layer 2's two palettes is currently displayed (nextPaletteSystem,
	// nextpalette.go) -- NOT pre-baked in generateGPUTextures, since that
	// method has no SpectrumIO to resolve a palette through, and Layer
	// 2's palette is per-machine state, not a fixed global the way the
	// classic zxPalette is. layer2ColourGPUVersion records which
	// io.nextPaletteVersion the cache was last baked against; a stale
	// version triggers a full 256-entry rebake, cheap enough (comparable
	// to generateGPUTextures's own 16-entry paperColourGPU bake) to do in
	// full rather than track per-entry dirtiness the way sprites' LRU
	// cache must. Superseded reasoning, kept here as history rather than
	// silently deleted: before T-31, this file's own comment argued Layer
	// 2 needed no cache of its own and could reuse paperColourGPU
	// directly, because the pre-T-31 interim colour mapping always
	// resolved a raw byte down to one of the same fixed 16 zxPalette
	// entries regardless of NR 0x70's palette-offset register -- true
	// only because the underlying palette itself was immutable. T-31
	// makes the underlying 256-entry palette genuinely writable via NR
	// 0x40/0x41/0x43/0x44, so that reasoning no longer holds: a bake keyed
	// on the resolved index must now track when the palette itself
	// changes, which is exactly what layer2ColourGPUVersion does.
	layer2ColourGPU        [256]rl.Texture2D
	layer2ColourGPUReady   bool
	layer2ColourGPUVersion uint32

	// videoRenderer is the currently-active renderer, set alongside the
	// geometry fields below by SetVideoRenderer -- Render checks this for
	// the optional FastGUIRenderer interface each frame.
	videoRenderer VideoRenderer

	// Active renderer's reported geometry (videorender.go). Window sizing
	// and drawing read these instead of assuming the standard 256x192 /
	// 32-32-24-32 border, so a mode with different dimensions or no border
	// at all (points 2/3, 2026-08-17) is handled without special
	// cases here. Defaults to the standard renderer's values;
	// SetVideoRenderer updates them -- see that method's comment for the
	// ordering requirement.
	screenW, screenH                                 int
	borderLeft, borderRight, borderTop, borderBottom int

	screenTexture        rl.Texture2D // uploaded from the active renderer's output each frame -- only used by the Decode()-based (non-fast-path) draw
	screenTextureReady   bool
	borderTexture        rl.RenderTexture2D
	borderTextureReady   bool
	borderStripesEnabled bool
}

// showMachine turns the display to another machine: it draws that machine's
// screen at the zoom it had, and forgets what it had baked on the GPU for the
// machine it showed before, whose version counters mean nothing for this one.
// The caller gives the machine the display with SetDisplay.
func (dm *DisplayManager) showMachine(zx *ZenZX) {
	zx.screen.multiplier = dm.screen.multiplier
	dm.screen = zx.screen
	dm.spriteGPUCache.clear()
	if dm.layer2ColourGPUReady {
		for i := range dm.layer2ColourGPU {
			rl.UnloadTexture(dm.layer2ColourGPU[i])
		}
		dm.layer2ColourGPUReady = false
	}
}

// window returns the machine's GUI display, giving it one if it has none
// yet: the GUI program sets it at start-up, and GUI tests that build a bare
// machine get it on first use.
func (zx *ZenZX) window() *DisplayManager {
	if dm, ok := zx.display.(*DisplayManager); ok {
		return dm
	}
	dm := NewDisplayManager(zx.screen)
	zx.SetDisplay(dm)
	return dm
}

func NewDisplayManager(screen *SpectrumScreen) *DisplayManager {
	var std standardVideoRenderer
	w, h := std.Dimensions()
	bl, br, bt, bb := std.BorderMargins()

	dm := &DisplayManager{
		screen:               screen,
		showFPS:              false, // a diagnostic: the View menu or Alt+F shows it
		showBorder:           true,
		borderApplied:        true,
		showDebugOverlay:     false,
		borderColor:          0,
		screenW:              w,
		screenH:              h,
		borderLeft:           bl,
		borderRight:          br,
		borderTop:            bt,
		borderBottom:         bb,
		borderChanges:        make([]BorderChange, 0),
		borderStripesEnabled: true,
		spriteGPUCache:       newSpriteGPUCache(defaultSpriteGPUCacheCapacity),
	}
	totalW, totalH := dm.totalSize()
	dm.currentWidth = totalW * 2
	dm.currentHeight = totalH * 2
	dm.targetWidth = dm.currentWidth
	dm.targetHeight = dm.currentHeight
	return dm
}

// SetVideoRenderer updates the cached geometry DisplayManager draws from.
// Both front ends resolve -ns-graphics and call this before the window (and
// therefore the GPU textures) exist, so sizing always reflects the right
// renderer from the start; this does not support switching renderers after
// InitializeAfterWindow has run.
func (dm *DisplayManager) SetVideoRenderer(r VideoRenderer) {
	dm.videoRenderer = r
	dm.screenW, dm.screenH = r.Dimensions()
	dm.borderLeft, dm.borderRight, dm.borderTop, dm.borderBottom = r.BorderMargins()
	dm.updateTargetSize()
}

// SetSpriteGPUCacheCapacity changes the sprite GPU texture cache's
// (spriteGPUCache, videorender_gpu.go) capacity at runtime -- the
// "buffer size configurable" knob from T-30's own design direction.
// Shrinking evicts least-recently-used entries immediately; growing just
// raises the ceiling for future insertions. Safe to call before
// InitializeAfterWindow (the cache itself is constructed by
// NewDisplayManager, independent of the GL context bitPatternGPU/
// paperColourGPU need) and at any point during a running session.
func (dm *DisplayManager) SetSpriteGPUCacheCapacity(capacity int) {
	if dm.spriteGPUCache == nil {
		dm.spriteGPUCache = newSpriteGPUCache(capacity)
		return
	}
	dm.spriteGPUCache.setCapacity(capacity)
}

// totalSize is the window's client-area size, in the active renderer's
// own pixels. For the standard 256x192 renderer (the only one Layer 2/
// sprites ever run alongside) this is the FIXED reference canvas
// (RefWidth, RefHeight -- display_constants.go), not screenW/screenH plus
// border, so that switching Layer 2 resolution mode at runtime (NR 0x70)
// never itself changes the window size -- only an explicit user zoom
// (ScaleUp/ScaleDown/SetScale) does. Any other renderer (e.g. hicolour)
// keeps the pre-existing screenW/screenH+border computation unchanged,
// since Layer 2/sprites never run alongside it.
func (dm *DisplayManager) totalSize() (int, int) {
	if _, ok := dm.videoRenderer.(standardVideoRenderer); ok {
		return RefWidth, RefHeight
	}
	return dm.screenW + dm.borderLeft + dm.borderRight, dm.screenH + dm.borderTop + dm.borderBottom
}

// hasBorder reports whether the active renderer has a border at all.
// Border is optional per mode (point 2, 2026-08-17): a renderer
// with no border reports all-zero margins, and every border code path
// below is skipped for it.
func (dm *DisplayManager) hasBorder() bool {
	return dm.borderLeft+dm.borderRight+dm.borderTop+dm.borderBottom > 0
}

// borderView is how the border stands for the window as it is now: the one place that
// says whether it is on the screen and whether the Border Display choice has any say in
// that. The window's size, the drawing, the tiles, the View menu's checkbox and Alt+B all
// ask it, so they cannot disagree; a new reason for the border to be fixed one way or the
// other is added here and nowhere else.
type borderView struct {
	Shown     bool   // the border is on the screen
	Choosable bool   // the Border Display choice (showBorder) decides Shown
	Why       string // when not Choosable, why, in a line
}

func (dm *DisplayManager) borderView() borderView {
	switch {
	case dm.tiles != nil:
		// A tile is a cell of a machine's whole frame with its border round the picture.
		return borderView{Shown: true, Why: "a tiled layout shows every machine with its border"}
	case !dm.hasBorder():
		return borderView{Shown: false, Why: "this display mode has no border"}
	case dm.io != nil && dm.io.layer2Enabled() && dm.io.layer2CurrentMode() != layer2Mode256x192x8bpp:
		// The wider Layer 2 modes fill the whole frame; without the border they would be cut.
		return borderView{Shown: true, Why: "the Layer 2 mode in use fills the whole frame"}
	}
	return borderView{Shown: dm.showBorder, Choosable: true}
}

func (dm *DisplayManager) SetBorderColor(color uint8) {
	dm.borderColor = color & 0x07
}

func (dm *DisplayManager) SetBorderChanges(changes []BorderChange, frameOrigin uint64, cyclesPerFrame int) {
	dm.borderChanges = changes
	dm.frameOrigin = frameOrigin
	dm.cyclesPerFrame = cyclesPerFrame
}

func (dm *DisplayManager) SetAudioManager(audio *AudioWrapper) {
	dm.audio = audio
}

// SetSpectrumIO stores io for maxMultiplierThatFits/SetScale's 640x256
// minimum-zoom clamp -- see the io field's own doc comment above for why
// this is a stored reference rather than a parameter on those methods.
// Safe to call at any point, including before InitializeAfterWindow; nil
// is a valid value (restores the pre-T-29 "no Layer 2 gate" behaviour).
func (dm *DisplayManager) SetSpectrumIO(io *SpectrumIO) {
	dm.io = io
}

// minMultiplierForMode returns the smallest zoom multiplier the active
// Layer 2 mode allows. Every mode except 640x256x4bpp allows 1x.
//
// CORRECTED reasoning (2026-09-15): an earlier draft of this comment
// claimed 640-mode's 640 real columns "exceed the fixed 320-column
// reference canvas at 1x" and would force cropping or window growth --
// wrong, and contradicted by an exact wiki.specnext.dev/
// Layer_2_Control_Register quote found while double-checking the tech team's
// own stated design against it: "every byte is displayed as two
// half-width paired pixels". 640-mode's pixels are HALF the physical
// width of a normal pixel, so 640 of them occupy exactly the SAME
// picture width as 320-mode's own 320 full-width pixels -- confirmed by
// layer2PixelWidthScale/layer2DisplayedSize (layer2.go), which is what
// nextLayersViewportOffset actually centres against. There is no
// canvas-overflow problem at any multiplier, including 1x.
//
// The real (and much narrower) reason 1x is still refused: a half-width
// pixel at multiplier m draws at m/2 screen pixels wide. That is a whole
// number only when m is even (m=2 -> 1px, m=4 -> 2px); at m=1 it is 0.5px,
// which can't render as a clean, sharp pixel edge. Refusing 1x catches
// the worst case cheaply. It does NOT catch every case -- m=3 (1.5px) is
// still allowed by this gate and still renders with sub-pixel blur; fully
// solving that would mean restricting 640-mode to only even multipliers
// (skipping 3 during ScaleUp/ScaleDown, not just flooring at 2), which
// this pass does not implement -- a known, minor, documented cosmetic
// limitation rather than an invented claim of full precision.
//
// Returns 1 (no gate) when dm.io is nil -- headless/classic-machine
// callers, and any test that hasn't called SetSpectrumIO.
func (dm *DisplayManager) MinMultiplierForMode() int {
	return dm.minMultiplierForMode()
}

func (dm *DisplayManager) minMultiplierForMode() int {
	if dm.io == nil {
		return 1
	}
	if dm.io.layer2Enabled() && dm.io.layer2CurrentMode() == layer2Mode640x256x4bpp {
		return 2
	}
	return 1
}

func (dm *DisplayManager) ToggleDebugOverlay() {
	dm.showDebugOverlay = !dm.showDebugOverlay
	fmt.Printf("Debug overlay: %v\n", dm.showDebugOverlay)
}

func (dm *DisplayManager) ToggleFPS() {
	dm.showFPS = !dm.showFPS
}

func (dm *DisplayManager) ToggleBorder() {
	if v := dm.borderView(); !v.Choosable {
		fmt.Printf("Border: %s\n", v.Why)
		return
	}
	dm.showBorder = !dm.showBorder
	dm.updateTargetSize()
	fmt.Printf("Border: %v\n", dm.showBorder)
}

func (dm *DisplayManager) ToggleBorderStripes() {
	dm.borderStripesEnabled = !dm.borderStripesEnabled
}

func (dm *DisplayManager) IsBorderStripesEnabled() bool {
	return dm.borderStripesEnabled
}

// maxMultiplierThatFits returns the largest multiplier from
// minMultiplierForMode() to MaxMultiplier whose resulting window (screen
// plus border, if shown) still fits the current monitor. Not every zoom
// factor fits every mode's resolution (point 3, 2026-08-17):
// mode-zenzx-02's 512x384 at 5x with a proportional border would be far
// larger than most displays. If the monitor size can't be determined, no
// upper clamping is applied, but the mode-driven minimum (2026-09-15,
// T-29 GUI-wiring pass -- see minMultiplierForMode) still is, since that
// floor is about the active Layer 2 mode's own real width, not the
// monitor. In the degenerate case where the monitor is too small even for
// the mode's minimum, the minimum still wins -- there is no sane fallback
// below it (see minMultiplierForMode's own comment).
func (dm *DisplayManager) maxMultiplierThatFits() int {
	min := dm.minMultiplierForMode()
	monW := int(rl.GetMonitorWidth(rl.GetCurrentMonitor()))
	monH := int(rl.GetMonitorHeight(rl.GetCurrentMonitor()))
	if monW <= 0 || monH <= 0 {
		return MaxMultiplier
	}
	totalW, totalH := dm.totalSize()
	for m := MaxMultiplier; m >= min; m-- {
		if totalW*m <= monW && totalH*m+dm.reservedTopHeight <= monH {
			return m
		}
	}
	return min
}

// tiledNoScale says, once asked, why the zoom does nothing in a tiled layout.
func (dm *DisplayManager) tiledNoScale() bool {
	if dm.tiles == nil {
		return false
	}
	fmt.Println("Scale: the zoom is for one machine filling the window; a tiled layout sizes its tiles itself (View menu)")
	return true
}

func (dm *DisplayManager) ScaleUp() bool {
	if dm.tiledNoScale() {
		return false
	}
	limit := dm.maxMultiplierThatFits()
	if dm.screen.multiplier < limit {
		dm.screen.multiplier++
		dm.updateTargetSize()
		fmt.Printf("Scale: %dx\n", dm.screen.multiplier)
		return true
	}
	if limit < MaxMultiplier {
		fmt.Printf("Scale: %dx is the most that fits this display\n", limit)
	}
	return false
}

func (dm *DisplayManager) ScaleDown() bool {
	if dm.tiledNoScale() {
		return false
	}
	min := dm.minMultiplierForMode()
	if dm.screen.multiplier > min {
		dm.screen.multiplier--
		dm.updateTargetSize()
		fmt.Printf("Scale: %dx\n", dm.screen.multiplier)
		return true
	}
	return false
}

// SetScale sets the display multiplier directly to n, if n is within
// [minMultiplierForMode(), maxMultiplierThatFits()] -- returns false and
// does nothing otherwise. Used by the View menu's zoom items (X1/X2/X3),
// which jump straight to a specific scale rather than stepping one at a
// time the way ScaleUp/ScaleDown do.
func (dm *DisplayManager) SetScale(n int) bool {
	if dm.tiledNoScale() {
		return false
	}
	if n < dm.minMultiplierForMode() || n > dm.maxMultiplierThatFits() {
		return false
	}
	if dm.screen.multiplier == n {
		return true // already there, nothing to do
	}
	dm.screen.multiplier = n
	dm.updateTargetSize()
	fmt.Printf("Scale: %dx\n", dm.screen.multiplier)
	return true
}

// setTiles hands the window's content to draw, which wants it w by h, and
// starts the window on its way to that size as a change of zoom does. A nil
// draw gives the window back to the one machine's picture.
func (dm *DisplayManager) setTiles(w, h int, draw func()) {
	dm.tiles, dm.tilesW, dm.tilesH = draw, w, h
	dm.updateTargetSize()
}

// syncCurrentToWindow sets the animated current size to the actual OS window
// size, so a snap that follows a user drag animates from where the window
// really is rather than from a stale model size.
func (dm *DisplayManager) syncCurrentToWindow(w, h int) {
	dm.currentWidth, dm.currentHeight = w, h
}

// HandleSingleResize re-asserts the single-machine window size after the user
// drags the window edge. The window is resizable so a tiled layout can be
// dragged, but a single machine keeps its own zoom-step size; a drag would
// otherwise leave the window a size the picture does not fill. When the OS
// window no longer matches the zoom-step size (targetWidth/Height), this snaps
// it back from where the drag left it, animating the change like a zoom step.
// It is for the single-machine path only (the deck has its own
// HandleWindowResize for tiled layouts).
func (dm *DisplayManager) HandleSingleResize() {
	if !rl.IsWindowResized() {
		return
	}
	w, h := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
	if w == dm.targetWidth && h == dm.targetHeight {
		return // already the right size (e.g. our own SetWindowSize)
	}
	dm.syncCurrentToWindow(w, h)
	dm.isAnimating = true // animateResize snaps current -> target and calls SetWindowSize
}

// renderTiles is Render while the window is tiled: the tiles, then what
// Render draws over any picture.
func (dm *DisplayManager) renderTiles(paused bool) {
	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)
	dm.tiles()
	if dm.showFPS {
		dm.drawFPS()
	}
	if dm.preEndDrawHook != nil {
		dm.preEndDrawHook()
	}
	rl.EndDrawing()
}

// RenderBlank is Render for a stopped machine: the background, and what is drawn over it. The
// background is the one that is around a picture, the gradient and its lines (drawFrame), with
// no picture and so no shadow.
func (dm *DisplayManager) RenderBlank() {
	rl.BeginDrawing()
	w, h := drawableSize()
	top := dm.frameTop()
	dm.drawFrame(pictureRect{}, top, w, h)
	dm.drawWaiting(top, w, h)
	if dm.preEndDrawHook != nil {
		dm.preEndDrawHook()
	}
	rl.EndDrawing()
}

func (dm *DisplayManager) updateTargetSize() {
	dm.windowKept = false // the window is being sent to a size: it is no longer left as it was
	showBorder := dm.borderView().Shown
	dm.borderApplied = showBorder
	if dm.tiles != nil {
		dm.targetWidth = dm.tilesW
		dm.targetHeight = dm.tilesH + dm.reservedTopHeight
		dm.isAnimating = true
		return
	}
	nativeW, nativeH := dm.pictureNativeSize(showBorder)
	dm.targetWidth = nativeW * dm.screen.multiplier
	dm.targetHeight = nativeH*dm.screen.multiplier + dm.reservedTopHeight
	dm.isAnimating = true
}

// classicScreenExtraTopMargin is how much of RefHeight's extra 8px over TotalHeight
// (display_constants.go) goes above the classic 256x192 picture, on top of BorderTop: all
// of it. That puts the picture 32 pixels down as well as 32 in, which is where a Next has
// it in its 320x256 frame (nextframe.go), so the picture does not move when a Next turns
// one of its layers on and the frame takes over the window. Until 0.13.0 the 8 pixels were
// split 4 and 4. Zero for any renderer other than the standard one (totalSize() doesn't
// use RefHeight for those, so there is no extra margin to distribute).
const classicScreenExtraTopMargin = 8
const classicScreenExtraBottomMargin = 0

// classicScreenOffset returns where the classic 256x192 (or active
// non-standard renderer's own) picture should be drawn within the
// window, in window pixels at the current multiplier -- the ONE place
// BorderTop's extra margin (see classicScreenExtraTopMargin above) is
// actually applied. For any renderer other than the standard one, this is
// exactly borderLeft*mult, borderTop*mult, unchanged from every version
// of this code before the T-29 GUI-wiring pass.
func (dm *DisplayManager) classicScreenOffset() (x, y int32) {
	mult := int32(dm.screen.multiplier)
	x = int32(dm.borderLeft) * mult
	y = int32(dm.borderTop) * mult
	if _, ok := dm.videoRenderer.(standardVideoRenderer); ok {
		y += int32(classicScreenExtraTopMargin) * mult
	}
	y += int32(dm.reservedTopHeight)
	return x, y
}

// nextLayersViewportOffset returns where Layer 2/sprites (renderNextLayersGPU)
// should be drawn: the active Layer 2 mode's own real displayed footprint
// (layer2DisplayedSize, layer2.go -- NOT simply layer2FramebufferDimensions
// x mult, since 640x256 mode's pixels are half-width, see that function's
// own comment), CENTRED within the fixed RefWidth x RefHeight reference
// canvas at the current multiplier.
//
// The base 256x192 Layer 2 mode is the one deliberate exception: it is
// NOT independently centred by this formula, but forced to exactly
// classicScreenOffset()'s own value. Layer 2's base mode composites
// pixel-for-pixel with the ULA layer at the identical 256x192 resolution
// (NR 0x15 only controls which is drawn on top, not a coordinate
// transform between them), so the two MUST share the exact same offset or
// they misalign visually once composited -- an independent centring
// formula was tried first and found, on inspection, to disagree with
// classicScreenOffset's own y by 4px (32 from pure centring within
// RefHeight vs 28 from BorderTop+classicScreenExtraTopMargin), which
// would have shown as a 4px vertical seam between the two layers in the
// most common Layer 2 mode -- caught before shipping by comparing the two
// formulas directly, not by a visual check this sandbox can't perform.
//
// When io is nil, when Layer 2 isn't enabled, or for any non-standard
// renderer (Layer 2 never runs alongside one), also falls back to
// classicScreenOffset's own values, matching this function's pre-T-29
// behaviour of reusing the classic screen's offset exactly.
func (dm *DisplayManager) nextLayersViewportOffset(io *SpectrumIO) (x, y int32) {
	if io == nil {
		return dm.classicScreenOffset()
	}
	if _, ok := dm.videoRenderer.(standardVideoRenderer); !ok {
		return dm.classicScreenOffset()
	}
	mode := io.layer2CurrentMode()
	if mode == layer2Mode256x192x8bpp {
		return dm.classicScreenOffset()
	}
	mult := dm.screen.multiplier
	dispW, dispH := layer2DisplayedSize(mode, mult)
	x = int32((float32(RefWidth*mult) - dispW) / 2)
	y = int32((float32(RefHeight*mult)-dispH)/2) + int32(dm.reservedTopHeight)
	return x, y
}

// SetReservedTopHeight sets how much extra window-pixel space at the
// top is reserved for a fixed menu bar (0 to release it), then
// triggers the same animated resize toggling the border already uses
// -- the window grows or shrinks to accommodate the change rather than
// jumping to it instantly. A no-op if the value isn't actually
// changing, matching ToggleBorder's own "don't animate to where you
// already are" behaviour.
func (dm *DisplayManager) SetReservedTopHeight(h int) {
	if dm.reservedTopHeight == h {
		return
	}
	dm.reservedTopHeight = h
	dm.retarget()
}

// retarget is updateTargetSize for a change that is not the person asking for a size: a window
// that is being kept as it is follows the picture's size (adoptPictureSize), and any other is
// sent to its new size.
func (dm *DisplayManager) retarget() {
	if dm.windowKept && dm.tiles == nil {
		dm.adoptPictureSize()
		return
	}
	dm.updateTargetSize()
}

// followBorder sets the window's target size anew if the border is not as it was when the
// window was last sized, whatever changed it: the View menu, Alt+B, a layout, or the
// machine going into a mode that needs it.
func (dm *DisplayManager) followBorder() {
	if dm.borderView().Shown != dm.borderApplied {
		dm.retarget()
	}
}

func (dm *DisplayManager) UpdateWindowSize() {
	dm.followBorder()
	if !dm.isAnimating {
		return
	}

	if dm.currentWidth != dm.targetWidth || dm.currentHeight != dm.targetHeight {
		deltaW := (dm.targetWidth - dm.currentWidth) / 10
		deltaH := (dm.targetHeight - dm.currentHeight) / 10

		if deltaW == 0 {
			deltaW = 1 * sgn(dm.targetWidth-dm.currentWidth)
		}
		if deltaH == 0 {
			deltaH = 1 * sgn(dm.targetHeight-dm.currentHeight)
		}

		dm.currentWidth += deltaW
		dm.currentHeight += deltaH

		if abs(dm.targetWidth-dm.currentWidth) <= abs(deltaW) {
			dm.currentWidth = dm.targetWidth
		}
		if abs(dm.targetHeight-dm.currentHeight) <= abs(deltaH) {
			dm.currentHeight = dm.targetHeight
		}

		rl.SetWindowSize(dm.currentWidth, dm.currentHeight)
	} else {
		dm.isAnimating = false
	}
}

func (dm *DisplayManager) renderDebugOverlay() {
	bufferLevel, generated, requested := dm.audio.GetBufferStatus()
	speakerChanges, cpuCycle := dm.audio.GetDebugInfo()
	drawAudioDebug(rl.GetScreenWidth(), rl.GetScreenHeight(), bufferLevel,
		fmt.Sprintf("Audio Buffer: %.1f%% (%d/%d samples)", bufferLevel, generated, requested),
		fmt.Sprintf("Speaker changes: %d", speakerChanges),
		fmt.Sprintf("CPU Cycle: %d", cpuCycle))
}

func (dm *DisplayManager) GetCurrentSize() (int, int) {
	return dm.currentWidth, dm.currentHeight
}

func (dm *DisplayManager) SetInitialSize(width, height int) {
	dm.currentWidth = width
	dm.currentHeight = height
	dm.targetWidth = width
	dm.targetHeight = height
}

// ============================================================================
// Helper Functions
// ============================================================================

func sgn(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ============================================================================
// Window Management
// ============================================================================

// InitDisplay creates the window sized for renderer's own dimensions and
// border margins (not the hardcoded standard 256x192 -- see videorender.go),
// then clamps the requested scale down if it wouldn't fit the current
// monitor (point 3, 2026-08-17). For the standard renderer specifically,
// the window's client area is the fixed reference canvas (RefWidth,
// RefHeight -- display_constants.go, 2026-09-15 T-29 GUI-wiring pass)
// rather than screenW/screenH+border, matching DisplayManager.totalSize's
// own logic exactly (this free function runs before a usable
// DisplayManager exists, so it can't just call totalSize() -- see this
// project's zenzx_gui.go call site comment on why renderer resolution
// happens before InitDisplay), so the very first window drawn already has
// room for Layer 2's wider modes without an initial-frame resize.
func InitDisplay(scale int, showBorder bool, renderer VideoRenderer) (int, int) {
	screenW, screenH := renderer.Dimensions()
	bl, br, bt, bb := renderer.BorderMargins()
	drawBorder := showBorder && (bl+br+bt+bb > 0)

	totalW, totalH := screenW, screenH
	if drawBorder {
		totalW = screenW + bl + br
		totalH = screenH + bt + bb
		if _, ok := renderer.(standardVideoRenderer); ok {
			totalW, totalH = RefWidth, RefHeight
		}
	}

	initialScale := scale
	if initialScale < 1 {
		initialScale = 1
	}
	if initialScale > MaxMultiplier {
		initialScale = MaxMultiplier
	}

	windowWidth := totalW * initialScale
	windowHeight := totalH * initialScale

	rl.SetTraceLogLevel(rl.LogNone)
	// Resizable so a tiled layout can be dragged to a new size; the drag is
	// snapped back to the layout's own proportions (machinedeck_gui.go's
	// snapTiledResize). A single-machine view ignores drags and keeps its zoom
	// steps, re-asserting its size on the next frame.
	rl.SetConfigFlags(rl.FlagWindowResizable)
	rl.InitWindow(int32(windowWidth), int32(windowHeight), "ZenZX")

	if monW, monH := int(rl.GetMonitorWidth(rl.GetCurrentMonitor())), int(rl.GetMonitorHeight(rl.GetCurrentMonitor())); monW > 0 && monH > 0 {
		fitted := initialScale
		for fitted > 1 && (totalW*fitted > monW || totalH*fitted > monH) {
			fitted--
		}
		if fitted != initialScale {
			initialScale = fitted
			windowWidth = totalW * initialScale
			windowHeight = totalH * initialScale
			fmt.Printf("Requested scale %dx doesn't fit this display; using %dx\n", scale, initialScale)
			rl.SetWindowSize(windowWidth, windowHeight)
		}
	}

	rl.SetTargetFPS(50) // Match Spectrum frame rate

	return windowWidth, windowHeight
}

// Render draws the current frame: the active VideoRenderer's output,
// composited with the border texture if the active renderer has a border
// and it is currently shown. If the active renderer implements
// FastGUIRenderer, it draws directly via pre-baked, GPU-resident
// textures (videorender_gpu.go) -- no CPU-side image.RGBA is built and
// no texture is re-uploaded that frame. Otherwise mem/screen are decoded
// once here and the result uploaded to a single texture, exactly as
// before. Every mode draws through this one path; nothing here is
// standard-Spectrum-specific.
//
// The io parameter (T-29/T-30) is used by renderNextLayersGPU
// (videorender_gpu.go) to draw Layer 2 and/or the sprite layer as
// ADDITIONAL passes after the ULA layer above, in the correct relative
// order for the current NR 0x15 priority (layersAboveULA, layer2.go) --
// neither Layer 2 nor sprites are themselves a VideoRenderer, and neither
// takes over this function's existing fast/slow-path branching.
// renderNextLayersGPU draws nothing extra on every classic machine and on
// a Next machine that hasn't enabled Layer 2 or sprites, so this
// parameter changes no existing behaviour by itself.
func (dm *DisplayManager) Render(paused bool, mem *SpectrumMemory, screen *SpectrumScreen, io *SpectrumIO) {
	// Advances the FLASH blink phase once per frame, unconditionally,
	// regardless of which VideoRenderer is active -- see the design
	// comment above standardVideoRenderer for why this lives here rather
	// than in a renderer's own Decode.
	dm.screen.updateFlash()

	if dm.tiles != nil {
		dm.renderTiles(paused)
		return
	}

	fastRenderer, useFastPath := dm.videoRenderer.(FastGUIRenderer)
	useFastPath = useFastPath && dm.gpuTexturesReady
	// A Next in a Timex screen mode or with the tilemap on is drawn by software.
	soft := dm.nextSoftwareULA(mem, screen, io)
	if soft != nil {
		useFastPath = false
	}
	// A Next with Layer 2, the sprites or the tilemap on: the whole frame, border included,
	// is composed by software (nextframe.go) and shown as one texture over the picture area.
	var frame *image.RGBA
	if io != nil && io.nextFrameActive() {
		frame = io.NextFrame(func() *image.RGBA { return dm.videoRenderer.Decode(mem, screen) }, screen, io.borderColor)
		soft, useFastPath = frame, false
	}

	rl.BeginDrawing()

	if !useFastPath {
		img := soft
		if img == nil {
			img = dm.videoRenderer.Decode(mem, screen)
		}
		dm.showInScreenTexture(img)
	}

	drawBorder := dm.borderView().Shown

	// The border's stripes go in their own render texture, which cannot be drawn into from inside
	// another: so before the canvas, when there is one (display_fit_gui.go).
	if frame == nil && drawBorder {
		dm.renderBorderStripes(dm.borderChanges, dm.frameOrigin, dm.borderColor)
	}
	canvas := dm.pictureCanvasWanted()
	if canvas {
		dm.beginPictureCanvas()
	}

	if frame != nil {
		dm.drawNextFrame(drawBorder)
	} else if drawBorder {
		rl.ClearBackground(rl.Black)

		totalW, totalH := dm.totalSize()
		destRect := rl.Rectangle{
			X:      0,
			Y:      float32(dm.reservedTopHeight),
			Width:  float32(dm.currentWidth),
			Height: float32(dm.currentHeight - dm.reservedTopHeight),
		}

		// The border texture is a RenderTexture2D (an FBO), which OpenGL
		// stores bottom-up; flip it via a negative source height.
		rl.DrawTexturePro(
			dm.borderTexture.Texture,
			rl.Rectangle{X: 0, Y: 0, Width: float32(totalW), Height: -float32(totalH)},
			destRect,
			rl.Vector2{X: 0, Y: 0},
			0,
			rl.White,
		)

		borderH, borderV := dm.classicScreenOffset()
		screenWidth := int32(dm.screenW * dm.screen.multiplier)
		screenHeight := int32(dm.screenH * dm.screen.multiplier)

		rl.BeginScissorMode(borderH, borderV, screenWidth, screenHeight)
		if useFastPath {
			fastRenderer.RenderGPU(dm, mem, screen, float32(borderH), float32(borderV), dm.screen.multiplier)
		} else {
			rl.DrawTexturePro(
				dm.screenTexture,
				rl.Rectangle{X: 0, Y: 0, Width: float32(dm.screenTexture.Width), Height: float32(dm.screenTexture.Height)},
				rl.Rectangle{X: float32(borderH), Y: float32(borderV), Width: float32(screenWidth), Height: float32(screenHeight)},
				rl.Vector2{X: 0, Y: 0},
				0,
				rl.White,
			)
		}
		rl.EndScissorMode()
	} else {
		rl.ClearBackground(rl.Black)
		if useFastPath {
			fastRenderer.RenderGPU(dm, mem, screen, 0, float32(dm.reservedTopHeight), dm.screen.multiplier)
		} else {
			rl.DrawTexturePro(
				dm.screenTexture,
				rl.Rectangle{X: 0, Y: 0, Width: float32(dm.screenTexture.Width), Height: float32(dm.screenTexture.Height)},
				rl.Rectangle{X: 0, Y: float32(dm.reservedTopHeight), Width: float32(dm.currentWidth), Height: float32(dm.currentHeight - dm.reservedTopHeight)},
				rl.Vector2{X: 0, Y: 0},
				0,
				rl.White,
			)
		}
	}

	if canvas {
		dm.endPictureCanvas()
	}

	// Draw status
	if dm.showFPS {
		dm.drawFPS()

		// Show border stripes status
		if drawBorder && dm.borderStripesEnabled && len(dm.borderChanges) > 1 {
			stripesText := fmt.Sprintf("Border changes: %d", len(dm.borderChanges))
			drawUIText(stripesText, 10, rl.GetScreenHeight()-30, uiTextScale, rl.Green)
		}
	}

	// Draw debug overlay if enabled (after status text)
	if dm.showDebugOverlay && dm.audio != nil {
		dm.renderDebugOverlay()
	}

	if dm.preEndDrawHook != nil {
		dm.preEndDrawHook()
	}

	rl.EndDrawing()
}

// SetPreEndDrawHook installs fn to run at the end of every future Render
// call, still inside the BeginDrawing/EndDrawing bracket. Pass nil to
// remove a previously-installed hook.
func (dm *DisplayManager) SetPreEndDrawHook(fn func()) {
	dm.preEndDrawHook = fn
}

// InitializeAfterWindow creates the border texture, sized to the active
// renderer's geometry. The screen texture is created lazily on the first
// Render call, once there is an actual decoded image to size it from.
func (dm *DisplayManager) InitializeAfterWindow() {
	totalW, totalH := dm.totalSize()
	dm.borderTexture = rl.LoadRenderTexture(int32(totalW), int32(totalH))
	dm.borderTextureReady = true
	fmt.Printf("Border texture created: %dx%d\n", totalW, totalH)

	dm.generateGPUTextures()
}

// CleanupTextures should be called before closing the window.
func (dm *DisplayManager) CleanupTextures() {
	dm.unloadPictureCanvas()
	dm.unloadFrameShadow()
	dm.unloadWaiting()
	if dm.screenTextureReady {
		rl.UnloadTexture(dm.screenTexture)
	}
	if dm.borderTextureReady {
		rl.UnloadRenderTexture(dm.borderTexture)
	}
	if dm.gpuTexturesReady {
		for i := range dm.bitPatternGPU {
			rl.UnloadTexture(dm.bitPatternGPU[i])
		}
		for i := range dm.paperColourGPU {
			rl.UnloadTexture(dm.paperColourGPU[i])
		}
		dm.gpuTexturesReady = false
	}
	if dm.spriteGPUCache != nil {
		dm.spriteGPUCache.clear()
	}
	if dm.layer2ColourGPUReady {
		for i := range dm.layer2ColourGPU {
			rl.UnloadTexture(dm.layer2ColourGPU[i])
		}
		dm.layer2ColourGPUReady = false
	}
}

func (dm *DisplayManager) renderBorderStripes(borderChanges []BorderChange, frameOrigin uint64, defaultColor uint8) {
	cyclesPerFrame := dm.cyclesPerFrame
	if cyclesPerFrame <= 0 {
		cyclesPerFrame = CyclesPerFrame // not yet set by a SetBorderChanges call -- PAL default, matches pre-Stage-2 behaviour exactly
	}

	rl.BeginTextureMode(dm.borderTexture)
	rl.ClearBackground(ZXPaletteRGBA[defaultColor&0x07])

	if !dm.borderStripesEnabled || len(borderChanges) == 0 {
		rl.EndTextureMode()
		return
	}

	totalW, totalH := dm.totalSize()

	// First, build the stripe pattern from actual changes
	var stripePattern []struct {
		heightRatio float32
		color       uint8
	}

	// The caller passes the true frame origin (RunFrame's frameOrigin).
	// The old modulo derivation drifted continuously against per-frame
	// instruction overshoot and applied contention delays.
	frameStartCycle := frameOrigin
	lastY := float32(0)
	lastColor := defaultColor

	for _, change := range borderChanges {
		cycleInFrame := change.Cycle - frameStartCycle
		if cycleInFrame >= uint64(cyclesPerFrame) {
			continue
		}

		y := float32(cycleInFrame) / float32(cyclesPerFrame)
		if y > lastY {
			stripePattern = append(stripePattern, struct {
				heightRatio float32
				color       uint8
			}{y - lastY, lastColor})
		}
		lastColor = change.Color
		lastY = y
	}

	// Now repeat the pattern to fill the full height
	if len(stripePattern) > 0 {
		totalPatternHeight := lastY
		repetitions := int(1.0/totalPatternHeight) + 1

		currentY := int32(0)
		for rep := 0; rep < repetitions && currentY < int32(totalH); rep++ {
			for _, stripe := range stripePattern {
				stripeHeight := int32(stripe.heightRatio * float32(totalH))
				if currentY+stripeHeight > int32(totalH) {
					stripeHeight = int32(totalH) - currentY
				}

				color := ZXPaletteRGBA[stripe.color&0x07]
				rl.DrawRectangle(0, currentY, int32(totalW), stripeHeight, color)

				currentY += stripeHeight
				if currentY >= int32(totalH) {
					break
				}
			}
		}
	}

	rl.EndTextureMode()
}
