package zenui

// zspLogoColours are the Zen Spectrum Project logo's three colour
// rotations, [left-half, right-half] each -- measured directly from the
// three reference images provided (numpy, exact RGB extraction, not a
// visual guess). Note the three pairs are not all the same brightness
// level (the first is standard-brightness ZX yellow/cyan, the other two
// are bright-variant pairs) -- reproduced exactly as given rather than
// "corrected" to a brightness that wasn't actually provided.
var zspLogoColours = [3][2]Colour{
	{{R: 0xc8, G: 0xc8, B: 0x00, A: 0xff}, {R: 0x00, G: 0xc8, B: 0xc8, A: 0xff}}, // yellow / cyan
	{{R: 0x00, G: 0xff, B: 0x00, A: 0xff}, {R: 0xff, G: 0x00, B: 0xff, A: 0xff}}, // green / magenta
	{{R: 0x00, G: 0xff, B: 0xff, A: 0xff}, {R: 0xff, G: 0x00, B: 0x00, A: 0xff}}, // cyan / red
}

// zspLogoRotationPeriod is how long each colour arrangement is shown
// before advancing to the next, in seconds -- one second per frame (of
// three), so a full three-arrangement cycle takes three seconds. An
// earlier third-of-a-second period read as too fast once actually seen
// running.
const zspLogoRotationPeriod = 1.0

// zspLogoGeometry computes where the logo should draw and its block
// size, given the X position just past the bar's last label and the
// screen width. Pure function, no drawing -- mirrors rainbowGeometry's
// own shape. Unlike the rainbow's stripes, the logo's block size isn't
// flexible (it has to stay a clean 1:2 width:height ratio to read as the
// reference shape, not stretch to fill space) -- it's derived once from
// the bar's own height and only checked for whether it fits, not
// resized to available width.
func zspLogoGeometry(afterLabelsX, screenW, height int) (x0, blockW, blockH int, ok bool) {
	const rightMargin = 8

	// Three stacked levels fill the bar's height exactly; block width
	// is half block height, matching the reference logo's own 45x90
	// block proportion.
	blockH = height / 3
	if blockH < 2 {
		return 0, 0, 0, false // too short for the blocks to read as anything
	}
	blockW = blockH / 2
	if blockW < 1 {
		blockW = 1
	}
	// 3 blocks/tooth * 2 teeth/half * 2 halves = 12 block-widths total.
	totalW := blockW * 12

	available := screenW - rightMargin - afterLabelsX
	if totalW > available {
		return 0, 0, 0, false
	}
	x0 = screenW - rightMargin - totalW
	return x0, blockW, blockH, true
}

// drawZSPLogo draws the logo: two colour-halves, each two ascending
// three-block "teeth" (bottom-left to top-right, matching the measured
// reference exactly -- x increases and y decreases together as the
// block level rises), colourIdx selecting which of the three rotation
// states zspLogoColours holds. Entirely through Renderer (FillRect), so
// it works identically under any host, not just raylib.
func drawZSPLogo(r Renderer, x0, barY, blockW, blockH, colourIdx int) {
	colours := zspLogoColours[colourIdx%len(zspLogoColours)]
	for half := 0; half < 2; half++ {
		c := colours[half]
		for tooth := 0; tooth < 2; tooth++ {
			for level := 0; level < 3; level++ {
				x := x0 + half*6*blockW + tooth*3*blockW + level*blockW
				y := barY + (2-level)*blockH
				r.FillRect(Rect{X: x, Y: y, W: blockW, H: blockH}, c)
			}
		}
	}
}
