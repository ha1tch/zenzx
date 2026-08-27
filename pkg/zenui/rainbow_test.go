package zenui

import "testing"

func TestRainbowColoursMatchesMeasuredReference(t *testing.T) {
	// Measured pixel-by-pixel from an actual +3 boot screen screenshot
	// (numpy, exact colour classification): exactly 4 bands, left to
	// right red/yellow/green/cyan, pure bright RGB.
	want := []Colour{
		{R: 0xff, G: 0x00, B: 0x00, A: 0xff},
		{R: 0xff, G: 0xff, B: 0x00, A: 0xff},
		{R: 0x00, G: 0xff, B: 0x00, A: 0xff},
		{R: 0x00, G: 0xff, B: 0xff, A: 0xff},
	}
	if len(rainbowColours) != len(want) {
		t.Fatalf("len(rainbowColours) = %d, want %d", len(rainbowColours), len(want))
	}
	for i, c := range want {
		if rainbowColours[i] != c {
			t.Errorf("rainbowColours[%d] = %+v, want %+v", i, rainbowColours[i], c)
		}
	}
}

func TestRainbowGeometryFits(t *testing.T) {
	oldW, oldS := RainbowBandWidth, RainbowScanlines
	defer func() { RainbowBandWidth, RainbowScanlines = oldW, oldS }()
	RainbowBandWidth, RainbowScanlines = 4, 5

	// Mirrors the example diagram exactly: screenW wide enough that
	// x0 - maxShift (4) still clears afterLabelsX.
	x0, ok := rainbowGeometry(100, 800, 5)
	if !ok {
		t.Fatal("expected ok=true with plenty of room")
	}
	wantX0 := 800 - 8 - 4*4 // rightMargin=8, totalW=16
	if x0 != wantX0 {
		t.Errorf("x0 = %d, want %d", x0, wantX0)
	}
}

func TestRainbowGeometryRefusesWhenShiftWouldCollideWithLabels(t *testing.T) {
	oldW, oldS := RainbowBandWidth, RainbowScanlines
	defer func() { RainbowBandWidth, RainbowScanlines = oldW, oldS }()
	RainbowBandWidth, RainbowScanlines = 4, 5

	// x0 = 800-8-16 = 776; maxShift = 4; leftmost = 772. Push
	// afterLabelsX right up against that boundary.
	_, ok := rainbowGeometry(773, 800, 5)
	if ok {
		t.Error("expected ok=false: the shift's leftmost pixel (772) falls short of afterLabelsX (773)")
	}
	_, ok = rainbowGeometry(772, 800, 5)
	if !ok {
		t.Error("expected ok=true: the shift's leftmost pixel (772) exactly meets afterLabelsX (772)")
	}
}

// TestDrawRainbowMatchesExampleDiagramExactly reproduces the exact
// 5-row, 4px-band example given directly, pixel for pixel:
//
//	. . . . . . . . . . . . . . R R R R Y Y Y Y G G G G C C C C
//	. . . . . . . . . . . . . R R R R Y Y Y Y G G G G C C C C
//	. . . . . . . . . . . . R R R R Y Y Y Y G G G G C C C C
//	. . . . . . . . . . . R R R R Y Y Y Y G G G G C C C C
//	. . . . . . . . . . R R R R Y Y Y Y G G G G C C C C
//
// Rebuilds each row from the recorded per-band lines and compares the
// resulting colour-per-column strings against these five rows directly.
func TestDrawRainbowMatchesExampleDiagramExactly(t *testing.T) {
	oldW, oldS := RainbowBandWidth, RainbowScanlines
	defer func() { RainbowBandWidth, RainbowScanlines = oldW, oldS }()
	RainbowBandWidth, RainbowScanlines = 4, 5

	want := []string{
		"..............RRRRYYYYGGGGCCCC",
		".............RRRRYYYYGGGGCCCC.",
		"............RRRRYYYYGGGGCCCC..",
		"...........RRRRYYYYGGGGCCCC...",
		"..........RRRRYYYYGGGGCCCC....",
	}
	screenW := len(want[0]) + 8 // leave room for rightMargin=8 past the row-0 pattern's own width
	afterLabelsX := 0

	r := newDrawRecorder()
	drawRainbow(r, afterLabelsX, 0, screenW, 5)

	colourLetter := map[Colour]byte{
		{R: 0xff, G: 0x00, B: 0x00, A: 0xff}: 'R',
		{R: 0xff, G: 0xff, B: 0x00, A: 0xff}: 'Y',
		{R: 0x00, G: 0xff, B: 0x00, A: 0xff}: 'G',
		{R: 0x00, G: 0xff, B: 0xff, A: 0xff}: 'C',
	}

	rows := make([]([]byte), 5)
	for i := range rows {
		row := make([]byte, len(want[0]))
		for j := range row {
			row[j] = '.'
		}
		rows[i] = row
	}

	for _, c := range *r.calls {
		if c.kind != "fill" {
			continue
		}
		letter, known := colourLetter[c.col]
		if !known {
			t.Fatalf("unexpected colour drawn: %+v", c.col)
		}
		y := c.rect.Y
		if y < 0 || y >= 5 {
			t.Fatalf("fill at unexpected row y=%d", y)
		}
		x1 := c.rect.X
		x2 := x1 + c.rect.W - 1 // drawRecorder stores FillRect's W as the actual width
		for x := x1; x <= x2; x++ {
			if x < 0 || x >= len(rows[y]) {
				continue
			}
			rows[y][x] = letter
		}
	}

	for i, row := range rows {
		got := string(row)
		if got != want[i] {
			t.Errorf("row %d:\n got %q\nwant %q", i, got, want[i])
		}
	}
}

func TestDrawRainbowDrawsNothingWhenNoRoom(t *testing.T) {
	oldW, oldS := RainbowBandWidth, RainbowScanlines
	defer func() { RainbowBandWidth, RainbowScanlines = oldW, oldS }()
	RainbowBandWidth, RainbowScanlines = 16, 24

	r := newDrawRecorder()
	drawRainbow(r, 790, 0, 800, 24) // almost no room at all

	if len(*r.calls) != 0 {
		t.Errorf("expected no draw calls with insufficient room, got %d", len(*r.calls))
	}
}

func TestDrawRainbowNoClipping(t *testing.T) {
	r := newDrawRecorder()
	drawRainbow(r, 100, 0, 800, 24)

	for _, c := range *r.calls {
		if c.kind == "clip" || c.kind == "clipend" {
			t.Errorf("unexpected %s call -- the direct per-row, per-band approach needs no clipping at all", c.kind)
		}
	}
}

func TestDrawRainbowScanlinesCappedByBarHeight(t *testing.T) {
	oldW, oldS := RainbowBandWidth, RainbowScanlines
	defer func() { RainbowBandWidth, RainbowScanlines = oldW, oldS }()
	RainbowBandWidth, RainbowScanlines = 4, 100 // far taller than any real bar

	r := newDrawRecorder()
	height := 24
	drawRainbow(r, 100, 0, 800, height)

	maxY := -1
	for _, c := range *r.calls {
		if c.kind == "line" && c.rect.Y > maxY {
			maxY = c.rect.Y
		}
	}
	if maxY >= height {
		t.Errorf("drew a scanline at y=%d, want capped below the bar's own height (%d)", maxY, height)
	}
}

func TestDrawRainbowAdjacentBandsShareCommonEdgePixel(t *testing.T) {
	// Regression guard for the 1px black seam a DrawLine-based version
	// left between bands: a band's right edge (x0+W-1) must be exactly
	// one less than the next band's left edge (x0), with no gap column
	// left unfilled by either.
	r := newDrawRecorder()
	drawRainbow(r, 100, 0, 800, 24)

	// row 0 (shift=0) is the simplest case to check directly.
	filled := map[int]Colour{}
	for _, c := range *r.calls {
		if c.kind == "fill" && c.rect.Y == 0 {
			for x := c.rect.X; x < c.rect.X+c.rect.W; x++ {
				filled[x] = c.col
			}
		}
	}
	x0, _ := rainbowGeometry(100, 800, 24)
	for i := 0; i < len(rainbowColours)-1; i++ {
		lastOfBand := x0 + i*RainbowBandWidth + RainbowBandWidth - 1
		firstOfNext := x0 + (i+1)*RainbowBandWidth
		if _, ok := filled[lastOfBand]; !ok {
			t.Errorf("band %d's last column (x=%d) was never filled", i, lastOfBand)
		}
		if _, ok := filled[firstOfNext]; !ok {
			t.Errorf("band %d's first column (x=%d) was never filled", i+1, firstOfNext)
		}
		if firstOfNext != lastOfBand+1 {
			t.Errorf("gap between band %d (ends %d) and band %d (starts %d)", i, lastOfBand, i+1, firstOfNext)
		}
	}
}
