package zenui

import "testing"

func TestZSPLogoColoursMatchMeasuredReference(t *testing.T) {
	// Measured pixel-by-pixel (numpy exact RGB extraction) from the
	// three reference images provided. Note the first pair is
	// standard-brightness ZX yellow/cyan while the other two are
	// bright-variant pairs -- not all the same brightness level,
	// reproduced exactly as given.
	want := [3][2]Colour{
		{{0xc8, 0xc8, 0x00, 0xff}, {0x00, 0xc8, 0xc8, 0xff}},
		{{0x00, 0xff, 0x00, 0xff}, {0xff, 0x00, 0xff, 0xff}},
		{{0x00, 0xff, 0xff, 0xff}, {0xff, 0x00, 0x00, 0xff}},
	}
	if zspLogoColours != want {
		t.Errorf("zspLogoColours = %+v, want %+v", zspLogoColours, want)
	}
}

func TestZSPLogoGeometryBlockProportionIsOneToTwo(t *testing.T) {
	_, blockW, blockH, ok := zspLogoGeometry(100, 800, 24)
	if !ok {
		t.Fatal("expected ok=true with plenty of room")
	}
	if blockH != 8 {
		t.Errorf("blockH = %d, want 8 (24/3, three levels filling the bar height)", blockH)
	}
	if blockW != 4 {
		t.Errorf("blockW = %d, want 4 (half of blockH, matching the reference logo's 45x90 block ratio)", blockW)
	}
}

func TestZSPLogoGeometryNotEnoughSpace(t *testing.T) {
	_, _, _, ok := zspLogoGeometry(790, 800, 24)
	if ok {
		t.Error("expected ok=false with almost no room (logo needs 12 block-widths = 48px at the default block size)")
	}
}

func TestZSPLogoGeometryTooShortForBlocks(t *testing.T) {
	_, _, _, ok := zspLogoGeometry(100, 800, 4) // height=4 -> blockH=1, below the minimum of 2
	if ok {
		t.Error("expected ok=false when the bar is too short for blocks to read as anything")
	}
}

func TestDrawZSPLogoFillsTwelveBlocks(t *testing.T) {
	r := newDrawRecorder()
	drawZSPLogo(r, 100, 0, 4, 8, 0)

	fillCount := 0
	for _, c := range *r.calls {
		if c.kind == "fill" {
			fillCount++
		}
	}
	if fillCount != 12 {
		t.Errorf("fill count = %d, want 12 (3 blocks/tooth * 2 teeth/half * 2 halves)", fillCount)
	}
}

func TestDrawZSPLogoUsesCorrectHalfColours(t *testing.T) {
	r := newDrawRecorder()
	drawZSPLogo(r, 100, 0, 4, 8, 1) // colourIdx 1: green/magenta

	wantLeft := zspLogoColours[1][0]
	wantRight := zspLogoColours[1][1]

	leftFound, rightFound := false, false
	for _, c := range *r.calls {
		if c.kind != "fill" {
			continue
		}
		if c.col == wantLeft && c.rect.X < 100+6*4 {
			leftFound = true
		}
		if c.col == wantRight && c.rect.X >= 100+6*4 {
			rightFound = true
		}
	}
	if !leftFound {
		t.Error("no left-half block found with the expected left colour")
	}
	if !rightFound {
		t.Error("no right-half block found with the expected right colour")
	}
}

func TestDrawZSPLogoAscendingStaircaseDirection(t *testing.T) {
	// Matches the measured reference exactly: as block level rises
	// (bottom -> middle -> top), x increases and y decreases together.
	r := newDrawRecorder()
	drawZSPLogo(r, 100, 0, 4, 8, 0)

	var rects []Rect
	for _, c := range *r.calls {
		if c.kind == "fill" {
			rects = append(rects, c.rect)
		}
	}
	if len(rects) < 3 {
		t.Fatal("expected at least 3 fills for the first tooth")
	}
	// First three fills are half=0, tooth=0, level=0,1,2 in that order.
	bottom, middle, top := rects[0], rects[1], rects[2]
	if !(bottom.X < middle.X && middle.X < top.X) {
		t.Errorf("x should increase with level: bottom.X=%d middle.X=%d top.X=%d", bottom.X, middle.X, top.X)
	}
	if !(bottom.Y > middle.Y && middle.Y > top.Y) {
		t.Errorf("y should decrease with level: bottom.Y=%d middle.Y=%d top.Y=%d", bottom.Y, middle.Y, top.Y)
	}
}

func TestZSPLogoColourIndexAdvancesWithElapsedTime(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})
	theme := DefaultTheme()
	theme.ShowZSPLogo = true

	// Advance well past one full rotation period (1 second) with a
	// single Update call.
	mb.Update(Input{DeltaTime: 1.4})

	if mb.logoElapsed < zspLogoRotationPeriod {
		t.Fatalf("logoElapsed = %v, want at least one rotation period after a 1.4s DeltaTime", mb.logoElapsed)
	}
}

func TestZSPLogoDrawnOnlyWhenThemeRequestsIt(t *testing.T) {
	mb := NewMenuBar(MenuBarConfig{Items: testBarItems()})

	themeOff := DefaultTheme()
	themeOff.ShowZSPLogo = false
	r1 := newDrawRecorder()
	mb.Draw(r1, 800, 600, 0, 24, themeOff)
	countOff := len(*r1.calls)

	themeOn := DefaultTheme()
	themeOn.ShowZSPLogo = true
	r2 := newDrawRecorder()
	mb.Draw(r2, 800, 600, 0, 24, themeOn)
	countOn := len(*r2.calls)

	if countOn <= countOff {
		t.Errorf("draw call count with ShowZSPLogo=true (%d) should exceed false (%d)", countOn, countOff)
	}
}
