//go:build !headless

package zenzx

import (
	"fmt"
	"math/bits"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// The New tile's picture.
//
// It is a 16x16 Spectrum sprite drawn in zenplay: the ZX badge with a plus
// sign over its corner. It is kept here in zenplay's own notation for a
// sprite frame, one string a row, the pixels and then the mask in
// hexadecimal: a set pixel is ink, a pixel the mask covers without being
// set is paper, and the rest is not drawn. It came from the package
// gui-elements.zplay, where it is the sprite "Sprite 45".
var dockAddIcon = [16]string{
	"0000/0000",
	"0000/0000",
	"7ffc/7ffc",
	"fffe/fffe",
	"edb6/fffe",
	"edb6/fffe",
	"db6e/fffe",
	"db6e/fffe",
	"b6ff/ffff",
	"b6c1/ffff",
	"ffc9/ffff",
	"7fdd/7fff",
	"0049/007f",
	"0041/007f",
	"007f/007f",
	"0000/0000",
}

// iconRow is one row of a sprite: bit 15 is the leftmost pixel.
type iconRow struct{ ink, mask uint16 }

// parseIcon reads sixteen rows of zenplay's notation.
func parseIcon(rows [16]string) ([16]iconRow, error) {
	var out [16]iconRow
	for i, row := range rows {
		ink, mask, ok := strings.Cut(row, "/")
		if !ok || len(ink) != 4 || len(mask) != 4 {
			return out, fmt.Errorf("row %d %q: want four hexadecimal digits, a slash and four more", i, row)
		}
		p, err1 := strconv.ParseUint(ink, 16, 16)
		m, err2 := strconv.ParseUint(mask, 16, 16)
		if err1 != nil || err2 != nil {
			return out, fmt.Errorf("row %d %q: not hexadecimal", i, row)
		}
		out[i] = iconRow{ink: uint16(p), mask: uint16(m) | uint16(p)}
	}
	return out, nil
}

// iconBounds is the box of the pixels a sprite draws: columns x0 to x1 and
// rows y0 to y1, the ends not included. An empty sprite has an empty box.
func iconBounds(icon [16]iconRow) (x0, y0, x1, y1 int) {
	x0, y0 = 16, 16
	for y, row := range icon {
		if row.mask == 0 {
			continue
		}
		if y < y0 {
			y0 = y
		}
		y1 = y + 1
		if l := bits.LeadingZeros16(row.mask); l < x0 {
			x0 = l
		}
		if r := 16 - bits.TrailingZeros16(row.mask); r > x1 {
			x1 = r
		}
	}
	if x1 == 0 {
		return 0, 0, 0, 0
	}
	return x0, y0, x1, y1
}

// iconRun is a horizontal run of pixels of one kind in a sprite.
type iconRun struct {
	x, y, n int
	ink     bool
}

// iconRuns is a sprite as runs of ink and of paper, row by row.
func iconRuns(icon [16]iconRow) []iconRun {
	var runs []iconRun
	for y, row := range icon {
		for x := 0; x < 16; {
			bit := uint16(0x8000) >> x
			if row.mask&bit == 0 {
				x++
				continue
			}
			ink := row.ink&bit != 0
			start := x
			for x < 16 && row.mask&(uint16(0x8000)>>x) != 0 && (row.ink&(uint16(0x8000)>>x) != 0) == ink {
				x++
			}
			runs = append(runs, iconRun{x: start, y: y, n: x - start, ink: ink})
		}
	}
	return runs
}

// iconScale is how many screen pixels one pixel of the sprite takes in a
// tile of the given height: whole, so the sprite stays sharp, and such that
// it fills about half the tile.
func iconScale(tileHeight float32) int {
	s := int(tileHeight / 2 / 16)
	if s < 1 {
		s = 1
	}
	return s
}

// addIcon is dockAddIcon parsed; it is read once.
var addIcon, addIconErr = parseIcon(dockAddIcon)

// The New tile's picture is black ink on white paper whatever the theme,
// with nothing drawn round it: the dock shows through where the sprite has
// no pixel.
var (
	addIconInk   = zenui.Colour{R: 0, G: 0, B: 0, A: 255}
	addIconPaper = zenui.Colour{R: 255, G: 255, B: 255, A: 255}
)

// drawAddIcon draws the New tile's picture in the middle of tile.
func drawAddIcon(r zenui.Renderer, tile rl.Rectangle) {
	if addIconErr != nil {
		return
	}
	x0, y0, x1, y1 := iconBounds(addIcon)
	s := iconScale(tile.Height)
	left := int(tile.X+tile.Width/2) - (x1-x0)*s/2
	top := int(tile.Y+tile.Height/2) - (y1-y0)*s/2
	for _, run := range iconRuns(addIcon) {
		col := addIconPaper
		if run.ink {
			col = addIconInk
		}
		r.FillRect(zenui.Rect{X: left + (run.x-x0)*s, Y: top + (run.y-y0)*s, W: run.n * s, H: s}, col)
	}
}
