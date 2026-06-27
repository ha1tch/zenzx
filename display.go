//go:build !headless

package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ============================================================================
// Display Constants
// ============================================================================

// ============================================================================
// Color Palette
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
// Optimized Screen Rendering
// ============================================================================

type SpectrumScreen struct {
	bitmap               []byte
	attributes           []byte
	multiplier           int
	flashTickTock        bool
	lastFlashTime        time.Time
	flashEnabled         bool
	bitPatternTextures   [256]rl.Texture2D
	paperColorTextures   [16]rl.Texture2D
	borderTexture        rl.RenderTexture2D
	borderStripesEnabled bool
}

func NewSpectrumScreen() *SpectrumScreen {
	screen := &SpectrumScreen{
		bitmap:               make([]byte, 6144),
		attributes:           make([]byte, 768),
		multiplier:           2,
		flashEnabled:         true,
		lastFlashTime:        time.Now(),
		borderStripesEnabled: true, // Enable by default
	}

	// Don't generate textures here - window not created yet
	return screen
}

func (s *SpectrumScreen) generateTextures() {
	// Generate bit pattern textures
	for bitPattern := 0; bitPattern < 256; bitPattern++ {
		img := image.NewRGBA(image.Rect(0, 0, 8, 1))
		for x := 0; x < 8; x++ {
			if (bitPattern & (1 << uint(7-x))) != 0 {
				img.Set(x, 0, color.RGBA{255, 255, 255, 255})
			} else {
				img.Set(x, 0, color.RGBA{0, 0, 0, 0})
			}
		}
		s.bitPatternTextures[bitPattern] = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
	}

	// Generate paper color textures
	for colorIndex := 0; colorIndex < 16; colorIndex++ {
		brightness := (colorIndex >> 3) & 1
		paperColor := (colorIndex & 7) | (brightness << 3)

		img := image.NewRGBA(image.Rect(0, 0, 8, 8))
		c := ZXPaletteRGBA[paperColor]
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				img.Set(x, y, color.RGBA{c.R, c.G, c.B, c.A})
			}
		}
		s.paperColorTextures[colorIndex] = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
	}
}

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

func (s *SpectrumScreen) render() {
	s.updateFlash()

	// Render paper colors
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			attrOffset := row*32 + col
			attr := s.attributes[attrOffset]
			paperColor := (attr >> 3) & 0x07
			brightness := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			colorIndex := paperColor | (brightness << 3)
			if s.flashEnabled && flash == 1 && s.flashTickTock {
				inkColor := attr & 0x07
				colorIndex = inkColor | (brightness << 3)
			}

			rl.DrawTextureEx(s.paperColorTextures[colorIndex],
				rl.NewVector2(float32(col*8*s.multiplier), float32(row*8*s.multiplier)),
				0, float32(s.multiplier), rl.White)
		}
	}

	// Render bitmap with ink colors
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			for y := 0; y < 8; y++ {
				byteOffset := s.calcByteOffset(col*8, row*8+y)
				byteValue := s.bitmap[byteOffset]

				attrOffset := row*32 + col
				attr := s.attributes[attrOffset]
				inkColor := attr & 0x07
				brightness := (attr >> 6) & 0x01
				flash := (attr >> 7) & 0x01

				var color rl.Color
				if s.flashEnabled && flash == 1 && s.flashTickTock {
					paperColor := (attr >> 3) & 0x07
					color = ZXPaletteRGBA[paperColor|(brightness<<3)]
				} else {
					color = ZXPaletteRGBA[inkColor|(brightness<<3)]
				}

				rl.DrawTextureEx(s.bitPatternTextures[byteValue],
					rl.NewVector2(float32(col*8*s.multiplier), float32((row*8+y)*s.multiplier)),
					0, float32(s.multiplier), color)
			}
		}
	}
}

func (s *SpectrumScreen) calcByteOffset(x, y int) int {
	yOffset := y & 0x07
	xOffset := x / 8
	rowOffset := (y / 8) % 8
	blockOffset := y / 64
	return blockOffset*2048 + rowOffset*32 + yOffset*256 + xOffset
}

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

func (s *SpectrumScreen) Clear() {
	for i := range s.bitmap {
		s.bitmap[i] = 0
	}
	for i := range s.attributes {
		s.attributes[i] = 0x38 // Black ink on white paper
	}
}

func (s *SpectrumScreen) SetMultiplier(mult int) {
	if mult >= 1 && mult <= MaxMultiplier {
		s.multiplier = mult
	}
}

func (s *SpectrumScreen) GetMultiplier() int {
	return s.multiplier
}

func (s *SpectrumScreen) ToggleFlash() {
	s.flashEnabled = !s.flashEnabled
}

func (s *SpectrumScreen) IsFlashEnabled() bool {
	return s.flashEnabled
}

func (s *SpectrumScreen) ToggleBorderStripes() {
	s.borderStripesEnabled = !s.borderStripesEnabled
}

func (s *SpectrumScreen) IsBorderStripesEnabled() bool {
	return s.borderStripesEnabled
}

// CleanupTextures should be called before closing the window
func (s *SpectrumScreen) CleanupTextures() {
	for i := range s.bitPatternTextures {
		rl.UnloadTexture(s.bitPatternTextures[i])
	}
	for i := range s.paperColorTextures {
		rl.UnloadTexture(s.paperColorTextures[i])
	}
	rl.UnloadRenderTexture(s.borderTexture)
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
	fps              int32
	currentWidth     int
	currentHeight    int
	targetWidth      int
	targetHeight     int
	isAnimating      bool
	borderChanges    []BorderChange
	currentCycle     uint64
	audio            *AudioWrapper
}

func NewDisplayManager(screen *SpectrumScreen) *DisplayManager {
	dm := &DisplayManager{
		screen:           screen,
		showFPS:          true,
		showBorder:       true,
		showDebugOverlay: false,
		borderColor:      0,
		currentWidth:     (ScreenWidth + 64) * 2,
		currentHeight:    (ScreenHeight + 56) * 2,
		targetWidth:      (ScreenWidth + 64) * 2,
		targetHeight:     (ScreenHeight + 56) * 2,
		borderChanges:    make([]BorderChange, 0),
	}
	return dm
}

func (dm *DisplayManager) SetBorderColor(color uint8) {
	dm.borderColor = color & 0x07
}

func (dm *DisplayManager) SetBorderChanges(changes []BorderChange, currentCycle uint64) {
	dm.borderChanges = changes
	dm.currentCycle = currentCycle
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
	// Update target window size based on border visibility
	if dm.showBorder {
		dm.targetWidth = (ScreenWidth + 64) * dm.screen.multiplier
		dm.targetHeight = (ScreenHeight + 56) * dm.screen.multiplier
	} else {
		dm.targetWidth = ScreenWidth * dm.screen.multiplier
		dm.targetHeight = ScreenHeight * dm.screen.multiplier
	}
	dm.isAnimating = true
	fmt.Printf("Border: %v\n", dm.showBorder)
}

func (dm *DisplayManager) ScaleUp() bool {
	if dm.screen.multiplier < MaxMultiplier {
		dm.screen.multiplier++
		dm.updateTargetSize()
		fmt.Printf("Scale: %dx\n", dm.screen.multiplier)
		return true
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

func (dm *DisplayManager) updateTargetSize() {
	if dm.showBorder {
		dm.targetWidth = (ScreenWidth + 64) * dm.screen.multiplier
		dm.targetHeight = (ScreenHeight + 56) * dm.screen.multiplier
	} else {
		dm.targetWidth = ScreenWidth * dm.screen.multiplier
		dm.targetHeight = ScreenHeight * dm.screen.multiplier
	}
	dm.isAnimating = true
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

func InitDisplay(scale int, showBorder bool) (int, int) {
	initialScale := scale
	if initialScale < 1 {
		initialScale = 1
	}
	if initialScale > MaxMultiplier {
		initialScale = MaxMultiplier
	}

	windowWidth := ScreenWidth * initialScale
	windowHeight := ScreenHeight * initialScale
	if showBorder {
		windowWidth = (ScreenWidth + 64) * initialScale
		windowHeight = (ScreenHeight + 56) * initialScale
	}

	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(int32(windowWidth), int32(windowHeight), "ZenZX")
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

func (dm *DisplayManager) Render(paused bool) {
	rl.BeginDrawing()

	// Set background to border color if showing border
	if dm.showBorder {
		// First render border with stripes to texture
		dm.screen.renderBorderStripes(dm.borderChanges, dm.currentCycle, dm.borderColor)

		// Clear screen
		rl.ClearBackground(rl.Black)

		destRect := rl.Rectangle{
			X:      0,
			Y:      0,
			Width:  float32(dm.currentWidth),
			Height: float32(dm.currentHeight),
		}

		// Draw with proper texture coordinates
		// The texture is rendered upside down, so we flip it
		rl.DrawTexturePro(
			dm.screen.borderTexture.Texture,
			rl.Rectangle{0, 0, float32(TotalWidth), -float32(TotalHeight)}, // Negative to flip
			destRect,
			rl.Vector2{0, 0},
			0,
			rl.White,
		)

		// Calculate screen position (centered within border)
		// The screen is 256x192, centered in the 320x248 total area
		borderH := int32(BorderLeft * dm.screen.multiplier) // 32 pixels on each side
		borderV := int32(BorderTop * dm.screen.multiplier)  // 24 pixels top, 32 bottom

		// Create a viewport for the screen area only
		screenWidth := int32(ScreenWidth * dm.screen.multiplier)
		screenHeight := int32(ScreenHeight * dm.screen.multiplier)

		rl.BeginScissorMode(borderH, borderV, screenWidth, screenHeight)

		// Render the Spectrum screen at the offset position
		rl.PushMatrix()
		rl.Translatef(float32(borderH), float32(borderV), 0)
		dm.screen.render()
		rl.PopMatrix()

		rl.EndScissorMode()
	} else {
		// No border - just render the screen
		rl.ClearBackground(rl.Black)
		dm.screen.render()
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
		if dm.showBorder && dm.screen.borderStripesEnabled && len(dm.borderChanges) > 1 {
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

	rl.EndDrawing()
}

// Let's also check the border texture initialization
func (s *SpectrumScreen) InitializeAfterWindow() {
	s.generateTextures()

	// Create border render texture with the full border area size
	// This should be 320x248 (TotalWidth x TotalHeight)
	s.borderTexture = rl.LoadRenderTexture(int32(TotalWidth), int32(TotalHeight))

	// Debug: Verify texture size
	fmt.Printf("Border texture created: %dx%d\n", TotalWidth, TotalHeight)
}

func (s *SpectrumScreen) renderBorderStripes(borderChanges []BorderChange, currentCycle uint64, defaultColor uint8) {
	rl.BeginTextureMode(s.borderTexture)
	rl.ClearBackground(ZXPaletteRGBA[defaultColor&0x07])

	if !s.borderStripesEnabled || len(borderChanges) == 0 {
		rl.EndTextureMode()
		return
	}

	// First, build the stripe pattern from actual changes
	var stripePattern []struct {
		heightRatio float32
		color       uint8
	}

	frameStartCycle := currentCycle - (currentCycle % CyclesPerFrame)
	lastY := float32(0)
	lastColor := defaultColor

	for _, change := range borderChanges {
		cycleInFrame := change.Cycle - frameStartCycle
		if cycleInFrame >= CyclesPerFrame {
			continue
		}

		y := float32(cycleInFrame) / float32(CyclesPerFrame)
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
		for rep := 0; rep < repetitions && currentY < TotalHeight; rep++ {
			for _, stripe := range stripePattern {
				stripeHeight := int32(stripe.heightRatio * float32(TotalHeight))
				if currentY+stripeHeight > TotalHeight {
					stripeHeight = TotalHeight - currentY
				}

				color := ZXPaletteRGBA[stripe.color&0x07]
				rl.DrawRectangle(0, currentY, int32(TotalWidth), stripeHeight, color)

				currentY += stripeHeight
				if currentY >= TotalHeight {
					break
				}
			}
		}
	}

	rl.EndTextureMode()
}
