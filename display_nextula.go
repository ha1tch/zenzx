//go:build !headless

package zenzx

import (
	"image"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// nextSoftwareULA is the picture Render must show in place of its cell-by-cell drawing: a
// Next in a Timex screen mode or with the tilemap on (nextsoftula.go). nil otherwise.
func (dm *DisplayManager) nextSoftwareULA(mem *SpectrumMemory, screen *SpectrumScreen, io *SpectrumIO) *image.RGBA {
	if io == nil {
		return nil
	}
	return io.nextSoftwareULA(func() *image.RGBA { return dm.videoRenderer.Decode(mem, screen) }, screen)
}

// showInScreenTexture puts img in the screen texture, making the texture again when the
// size has changed: the picture is 256 pixels wide in most modes and 512 in the Next's
// hi-res and 80 column ones.
func (dm *DisplayManager) showInScreenTexture(img *image.RGBA) {
	w, h := int32(img.Rect.Dx()), int32(img.Rect.Dy())
	if dm.screenTextureReady && (dm.screenTexture.Width != w || dm.screenTexture.Height != h) {
		rl.UnloadTexture(dm.screenTexture)
		dm.screenTextureReady = false
	}
	if !dm.screenTextureReady {
		dm.screenTexture = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
		dm.screenTextureReady = true
		return
	}
	rl.UpdateTexture(dm.screenTexture, img)
}

// drawNextFrame shows the Next's composed frame (nextframe.go), which showInScreenTexture
// has put in the screen texture, over the whole picture area: all 320x256 of it when the
// border is shown, and the classic picture's part of it when the border is hidden.
func (dm *DisplayManager) drawNextFrame(withBorder bool) {
	rl.ClearBackground(rl.Black)
	tex := dm.screenTexture
	source := rl.Rectangle{X: 0, Y: 0, Width: float32(tex.Width), Height: float32(tex.Height)}
	if !withBorder {
		scale := float32(tex.Width / nextFrameWidth)
		source = rl.Rectangle{X: nextFrameInset * scale, Y: nextFrameInset, Width: ScreenWidth * scale, Height: ScreenHeight}
	}
	dest := rl.Rectangle{X: 0, Y: float32(dm.reservedTopHeight), Width: float32(dm.currentWidth), Height: float32(dm.currentHeight - dm.reservedTopHeight)}
	rl.DrawTexturePro(tex, source, dest, rl.Vector2{}, 0, rl.White)
}
