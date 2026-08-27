//go:build !headless

package main

import (
	"fmt"

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
	isAnimating    bool
	borderChanges  []BorderChange
	frameOrigin    uint64
	cyclesPerFrame int // Set via SetBorderChanges; defaults to 0 until the first call, guarded in renderBorderStripes
	audio          *AudioWrapper

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

	// GPU fast-path texture caches (videorender_gpu.go), baked once in
	// InitializeAfterWindow and freed in CleanupTextures. bitPatternGPU
	// holds one texture per possible byte value (which of 8 bits are
	// set) and paperColourGPU one per palette entry -- both are
	// universal concepts, not renderer-specific, so every FastGUIRenderer
	// implementation shares this same pair rather than baking its own.
	bitPatternGPU    [256]rl.Texture2D
	paperColourGPU   [16]rl.Texture2D
	gpuTexturesReady bool

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

func NewDisplayManager(screen *SpectrumScreen) *DisplayManager {
	var std standardVideoRenderer
	w, h := std.Dimensions()
	bl, br, bt, bb := std.BorderMargins()

	dm := &DisplayManager{
		screen:               screen,
		showFPS:              true,
		showBorder:           true,
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

// totalSize is the screen plus its border margins, in the active
// renderer's own pixels.
func (dm *DisplayManager) totalSize() (int, int) {
	return dm.screenW + dm.borderLeft + dm.borderRight, dm.screenH + dm.borderTop + dm.borderBottom
}

// hasBorder reports whether the active renderer has a border at all.
// Border is optional per mode (point 2, 2026-08-17): a renderer
// with no border reports all-zero margins, and every border code path
// below is skipped for it.
func (dm *DisplayManager) hasBorder() bool {
	return dm.borderLeft+dm.borderRight+dm.borderTop+dm.borderBottom > 0
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

func (dm *DisplayManager) ToggleDebugOverlay() {
	dm.showDebugOverlay = !dm.showDebugOverlay
	fmt.Printf("Debug overlay: %v\n", dm.showDebugOverlay)
}

func (dm *DisplayManager) ToggleFPS() {
	dm.showFPS = !dm.showFPS
}

func (dm *DisplayManager) ToggleBorder() {
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

// maxMultiplierThatFits returns the largest multiplier from 1 to
// MaxMultiplier whose resulting window (screen plus border, if shown)
// still fits the current monitor. Not every zoom factor fits every mode's
// resolution (point 3, 2026-08-17): mode-zenzx-02's 512x384 at 5x
// with a proportional border would be far larger than most displays. If
// the monitor size can't be determined, no clamping is applied.
func (dm *DisplayManager) maxMultiplierThatFits() int {
	monW := int(rl.GetMonitorWidth(rl.GetCurrentMonitor()))
	monH := int(rl.GetMonitorHeight(rl.GetCurrentMonitor()))
	if monW <= 0 || monH <= 0 {
		return MaxMultiplier
	}
	totalW, totalH := dm.totalSize()
	for m := MaxMultiplier; m >= 1; m-- {
		if totalW*m <= monW && totalH*m <= monH {
			return m
		}
	}
	return 1
}

func (dm *DisplayManager) ScaleUp() bool {
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
	if dm.screen.multiplier > 1 {
		dm.screen.multiplier--
		dm.updateTargetSize()
		fmt.Printf("Scale: %dx\n", dm.screen.multiplier)
		return true
	}
	return false
}

// SetScale sets the display multiplier directly to n, if n is within
// [1, maxMultiplierThatFits()] -- returns false and does nothing
// otherwise. Used by the View menu's zoom items (X1/X2/X3), which jump
// straight to a specific scale rather than stepping one at a time the
// way ScaleUp/ScaleDown do.
func (dm *DisplayManager) SetScale(n int) bool {
	if n < 1 || n > dm.maxMultiplierThatFits() {
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

func (dm *DisplayManager) updateTargetSize() {
	if dm.showBorder && dm.hasBorder() {
		totalW, totalH := dm.totalSize()
		dm.targetWidth = totalW * dm.screen.multiplier
		dm.targetHeight = totalH*dm.screen.multiplier + dm.reservedTopHeight
	} else {
		dm.targetWidth = dm.screenW * dm.screen.multiplier
		dm.targetHeight = dm.screenH*dm.screen.multiplier + dm.reservedTopHeight
	}
	dm.isAnimating = true
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
	dm.updateTargetSize()
}

func (dm *DisplayManager) UpdateWindowSize() {
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
	screenWidth := int32(rl.GetScreenWidth())
	screenHeight := int32(rl.GetScreenHeight())

	// Get audio buffer status
	bufferLevel, generated, requested := dm.audio.GetBufferStatus()

	// Draw histogram bar at top of screen
	barHeight := int32(20)
	barY := int32(40) // Position below any "PAUSED" text

	// Background for the bar
	rl.DrawRectangle(0, barY, screenWidth, barHeight, rl.NewColor(0, 0, 0, 128))

	// Calculate bar width based on buffer level
	barWidth := int32(float32(screenWidth) * bufferLevel / 100.0)

	// Choose color based on level
	var barColor rl.Color
	if bufferLevel < 30 {
		barColor = rl.Red // Critical - underrun likely
	} else if bufferLevel < 60 {
		barColor = rl.Yellow // Warning - getting low
	} else {
		barColor = rl.Green // Good level
	}

	// Draw the level bar
	rl.DrawRectangle(0, barY, barWidth, barHeight, barColor)

	// Draw text overlay
	text := fmt.Sprintf("Audio Buffer: %.1f%% (%d/%d samples)", bufferLevel, generated, requested)
	rl.DrawText(text, 5, barY+2, 16, rl.White)

	// Draw additional debug info
	debugY := barY + barHeight + 5

	// Get additional debug info
	speakerChanges, cpuCycle := dm.audio.GetDebugInfo()

	// Speaker history size
	historyText := fmt.Sprintf("Speaker changes: %d", speakerChanges)
	rl.DrawText(historyText, 5, debugY, 14, rl.SkyBlue)

	// CPU cycle info
	cycleText := fmt.Sprintf("CPU Cycle: %d", cpuCycle)
	rl.DrawText(cycleText, 5, debugY+16, 14, rl.SkyBlue)

	// Buffer health indicator
	healthY := screenHeight - 40
	healthText := "Buffer Health: "
	if bufferLevel < 30 {
		healthText += "UNDERRUN WARNING"
		rl.DrawText(healthText, 5, healthY, 16, rl.Red)
	} else if bufferLevel < 60 {
		healthText += "LOW"
		rl.DrawText(healthText, 5, healthY, 16, rl.Yellow)
	} else {
		healthText += "GOOD"
		rl.DrawText(healthText, 5, healthY, 16, rl.Green)
	}
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
// monitor (point 3, 2026-08-17).
func InitDisplay(scale int, showBorder bool, renderer VideoRenderer) (int, int) {
	screenW, screenH := renderer.Dimensions()
	bl, br, bt, bb := renderer.BorderMargins()
	drawBorder := showBorder && (bl+br+bt+bb > 0)

	totalW, totalH := screenW, screenH
	if drawBorder {
		totalW = screenW + bl + br
		totalH = screenH + bt + bb
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

func HandleDroppedFiles(screen *SpectrumScreen) {
	if rl.IsFileDropped() {
		files := rl.LoadDroppedFiles()
		if len(files) > 0 {
			filename := files[0]
			// Check file extension
			if len(filename) > 4 {
				ext := filename[len(filename)-4:]
				if ext == ".scr" || ext == ".SCR" {
					if err := screen.LoadFromFile(filename); err == nil {
						fmt.Printf("Loaded screen: %s\n", filename)
					} else {
						fmt.Printf("Error loading screen %s: %v\n", filename, err)
					}
				}
			}
		}
		rl.UnloadDroppedFiles()
	}
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
func (dm *DisplayManager) Render(paused bool, mem *SpectrumMemory, screen *SpectrumScreen) {
	// Advances the FLASH blink phase once per frame, unconditionally,
	// regardless of which VideoRenderer is active -- see the design
	// comment above standardVideoRenderer for why this lives here rather
	// than in a renderer's own Decode.
	dm.screen.updateFlash()

	fastRenderer, useFastPath := dm.videoRenderer.(FastGUIRenderer)
	useFastPath = useFastPath && dm.gpuTexturesReady

	rl.BeginDrawing()

	if !useFastPath {
		img := dm.videoRenderer.Decode(mem, screen)
		if !dm.screenTextureReady {
			dm.screenTexture = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
			dm.screenTextureReady = true
		} else {
			rl.UpdateTexture(dm.screenTexture, img)
		}
	}

	drawBorder := dm.showBorder && dm.hasBorder()

	if drawBorder {
		dm.renderBorderStripes(dm.borderChanges, dm.frameOrigin, dm.borderColor)

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

		borderH := int32(dm.borderLeft * dm.screen.multiplier)
		borderV := int32(dm.borderTop*dm.screen.multiplier) + int32(dm.reservedTopHeight)
		screenWidth := int32(dm.screenW * dm.screen.multiplier)
		screenHeight := int32(dm.screenH * dm.screen.multiplier)

		rl.BeginScissorMode(borderH, borderV, screenWidth, screenHeight)
		if useFastPath {
			fastRenderer.RenderGPU(dm, mem, screen, float32(borderH), float32(borderV), dm.screen.multiplier)
		} else {
			rl.DrawTexturePro(
				dm.screenTexture,
				rl.Rectangle{X: 0, Y: 0, Width: float32(dm.screenW), Height: float32(dm.screenH)},
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
				rl.Rectangle{X: 0, Y: 0, Width: float32(dm.screenW), Height: float32(dm.screenH)},
				rl.Rectangle{X: 0, Y: float32(dm.reservedTopHeight), Width: float32(dm.currentWidth), Height: float32(dm.currentHeight - dm.reservedTopHeight)},
				rl.Vector2{X: 0, Y: 0},
				0,
				rl.White,
			)
		}
	}

	// Draw status
	if dm.showFPS {
		dm.fps = rl.GetFPS()
		fpsText := fmt.Sprintf("FPS: %d", dm.fps)
		rl.DrawText(fpsText, int32(rl.GetScreenWidth()-100), int32(rl.GetScreenHeight()-30), 20, rl.Red)

		if paused {
			rl.DrawText("PAUSED", 10, 10, 20, rl.Yellow)
		}

		// Show border stripes status
		if drawBorder && dm.borderStripesEnabled && len(dm.borderChanges) > 1 {
			stripesText := fmt.Sprintf("Border changes: %d", len(dm.borderChanges))
			rl.DrawText(stripesText, 10, int32(rl.GetScreenHeight()-30), 16, rl.Green)
		}
	} else if paused {
		// Show PAUSED even if FPS is off
		rl.DrawText("PAUSED", 10, 10, 20, rl.Yellow)
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
