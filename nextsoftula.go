package zenzx

import "image"

// The part of a Next's picture that is always drawn by software.
//
// The GUI draws the standard Spectrum picture on the GPU, one attribute cell at a time,
// and that drawing knows nothing of the Timex screen modes or the tilemap. Both front ends
// therefore take the "U" layer from the one function below whenever a Next shows either:
// the headless build inside DecodeDisplay, the GUI as a texture in place of its cell
// drawing (display_nextula.go). Layer 2 and the sprites are put over it by each front end
// in its own way.

// nextSoftwareULA returns the "U" layer of a Next that is in a Timex screen mode or has the
// tilemap on, and nil for everything else, which is every other machine and a Next showing
// the standard screen. standard draws the ordinary 256x192 picture; it is only called when
// the tilemap is on without a Timex mode. The result is 256 or 512 pixels wide.
func (io *SpectrumIO) nextSoftwareULA(standard func() *image.RGBA, screen *SpectrumScreen) *image.RGBA {
	ula := io.DecodeNextTimex(screen)
	if ula == nil {
		if !io.tilemapEnabled() {
			return nil
		}
		ula = standard()
	}
	return io.CombineULAAndTilemap(ula, screen)
}
