//go:build !headless

package zenzx

import (
	"math/bits"
	"testing"
)

func TestAddIconParses(t *testing.T) {
	if addIconErr != nil {
		t.Fatalf("the New tile's sprite does not parse: %v", addIconErr)
	}
	x0, y0, x1, y1 := iconBounds(addIcon)
	if x0 != 0 || y0 != 2 || x1 != 16 || y1 != 15 {
		t.Errorf("the sprite's box is (%d,%d)-(%d,%d), want (0,2)-(16,15)", x0, y0, x1, y1)
	}
	// The plus sign in the corner: its centre and its four arms are ink,
	// the corners of its box are paper.
	ink := func(x, y int) bool { return addIcon[y].ink&(0x8000>>x) != 0 }
	paper := func(x, y int) bool { return !ink(x, y) && addIcon[y].mask&(0x8000>>x) != 0 }
	for _, p := range [][2]int{{12, 11}, {12, 10}, {12, 12}, {11, 11}, {13, 11}} {
		if !ink(p[0], p[1]) {
			t.Errorf("pixel %v of the plus sign is not ink", p)
		}
	}
	for _, p := range [][2]int{{10, 9}, {14, 9}, {10, 13}, {14, 13}} {
		if !paper(p[0], p[1]) {
			t.Errorf("pixel %v beside the plus sign is not paper", p)
		}
	}
	if ink(0, 0) || paper(0, 0) || paper(3, 13) {
		t.Error("a pixel outside the sprite is drawn")
	}
}

func TestIconRunsCoverEveryPixelOnce(t *testing.T) {
	var inkN, paperN int
	for _, row := range addIcon {
		inkN += bits.OnesCount16(row.ink)
		paperN += bits.OnesCount16(row.mask &^ row.ink)
	}
	var gotInk, gotPaper int
	seen := map[[2]int]bool{}
	for _, run := range iconRuns(addIcon) {
		if run.n <= 0 {
			t.Fatalf("empty run %+v", run)
		}
		for x := run.x; x < run.x+run.n; x++ {
			if seen[[2]int{x, run.y}] {
				t.Errorf("pixel (%d,%d) is in two runs", x, run.y)
			}
			seen[[2]int{x, run.y}] = true
			isInk := addIcon[run.y].ink&(0x8000>>x) != 0
			if isInk != run.ink {
				t.Errorf("pixel (%d,%d) is in a run of the wrong kind", x, run.y)
			}
		}
		if run.ink {
			gotInk += run.n
		} else {
			gotPaper += run.n
		}
	}
	if gotInk != inkN || gotPaper != paperN {
		t.Errorf("runs cover %d ink and %d paper pixels, the sprite has %d and %d", gotInk, gotPaper, inkN, paperN)
	}
}

func TestParseIconRejectsBadRows(t *testing.T) {
	rows := dockAddIcon
	rows[3] = "fffe"
	if _, err := parseIcon(rows); err == nil {
		t.Error("a row without a mask was accepted")
	}
	rows[3] = "ffzz/fffe"
	if _, err := parseIcon(rows); err == nil {
		t.Error("a row that is not hexadecimal was accepted")
	}
}

func TestIconScaleIsWholeAndFits(t *testing.T) {
	for _, c := range []struct {
		h    float32
		want int
	}{{24, 1}, {84, 2}, {128, 4}, {160, 5}} {
		if got := iconScale(c.h); got != c.want {
			t.Errorf("in a tile %v px high the sprite is drawn at %d, want %d", c.h, got, c.want)
		}
		if got := iconScale(c.h); float32(got*16) > c.h {
			t.Errorf("at %d the sprite is taller than its tile of %v px", got, c.h)
		}
	}
}
