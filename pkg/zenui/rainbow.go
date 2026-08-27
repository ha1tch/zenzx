package zenui

// RainbowBandWidth is each colour band's width in pixels.
// RainbowScanlines is how many rows the diagonal shift spans, one pixel
// of shift per row. Both are plain package variables, not derived from
// available screen space -- the shape is fixed and simple by design,
// matching the reference exactly, and can be retuned without touching
// the drawing logic at all. RainbowScanlines is normally left equal to
// the bar's own height so the rainbow fills it exactly.
var (
	RainbowBandWidth = 16
	RainbowScanlines = 24
)

// rainbowColours are the real 128K/+2/+3 boot screen's own title-strip
// rainbow bands. Originally measured pixel-by-pixel from an actual boot
// screenshot (numpy classification of every pixel across a wide row/
// column range); independently re-verified against a purpose-built
// reference asset (spectrum128.png) provided afterwards -- both agree on
// the same geometry (four equal-width bands, this colour order, a true
// 45-degree diagonal shift equal to the strip's own height) and the same
// four colours below.
var rainbowColours = []Colour{
	{R: 0xff, G: 0x00, B: 0x00, A: 0xff}, // bright red
	{R: 0xff, G: 0xff, B: 0x00, A: 0xff}, // bright yellow
	{R: 0x00, G: 0xff, B: 0x00, A: 0xff}, // bright green
	{R: 0x00, G: 0xff, B: 0xff, A: 0xff}, // bright cyan
}

// rainbowGeometry computes the rainbow's right-anchored X origin --
// x0 is row 0's leftmost pixel; every lower row starts exactly that
// row's own index further left, per drawRainbow -- and whether it fits
// at all between afterLabelsX and the bar's own right margin, including
// room for the full diagonal shift on the left (the last row's shift is
// scanlines-1 pixels). Pure function, no drawing, so the fit decision is
// directly testable independent of Draw.
func rainbowGeometry(afterLabelsX, screenW, height int) (x0 int, ok bool) {
	const rightMargin = 8

	scanlines := RainbowScanlines
	if height < scanlines {
		scanlines = height
	}
	totalW := RainbowBandWidth * len(rainbowColours)
	x0 = screenW - rightMargin - totalW

	maxShift := scanlines - 1
	if x0-maxShift < afterLabelsX {
		return 0, false
	}
	return x0, true
}

// drawRainbow draws the rainbow exactly as specified: for each of
// RainbowScanlines rows (capped at the bar's own height), one filled
// segment per colour band, each RainbowBandWidth pixels wide, with the
// whole four-band group shifted one pixel further left than the row
// above it. Reproduces the diagonal lean directly through per-row,
// per-band segments -- not diagonal DrawLine calls (produced a
// checkered artifact in an earlier attempt), and FillRect rather than
// horizontal DrawLine calls specifically: a 1px black seam appeared
// between adjacent bands with DrawLine (confirmed visually, not
// assumed), consistent with a line-drawing endpoint/rounding quirk
// that FillRect's unambiguous pixel coverage doesn't have. Every
// segment drawn here is already exactly the right width and position;
// nothing needs to be cut down to size afterwards, and adjacent bands
// now share their common edge pixel exactly rather than each stopping
// one short of it.
func drawRainbow(r Renderer, afterLabelsX, barY, screenW, height int) {
	x0, ok := rainbowGeometry(afterLabelsX, screenW, height)
	if !ok {
		return
	}

	scanlines := RainbowScanlines
	if height < scanlines {
		scanlines = height
	}
	for row := 0; row < scanlines; row++ {
		shift := row
		for i, c := range rainbowColours {
			x1 := x0 + i*RainbowBandWidth - shift
			r.FillRect(Rect{X: x1, Y: barY + row, W: RainbowBandWidth, H: 1}, c)
		}
	}
}
