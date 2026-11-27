// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// The magnification of the menu bar's own text: the labels along the strip (File, Machine, ...),
// as distinct from the dropdowns' text, which is the Font zoom setting.
//
// The bar draws its labels at the largest of the factors X1, X2 and X3 that fit the window both
// ways. Across: all the labels, each with one blank space after it, take no more than four
// fifths of the window's width. Down: the strip that holds the text, with room above and below
// it (as much as half the text's height, and never under 4 pixels), is no more than an eighth of
// the window's height, so that a short window does not give most of itself to the bar. A narrow
// or a short window gets X1, as the bar always had; a wide and tall one gets a bigger bar, in
// proportion to what it has room for, and what is left of the strip is the logo and the Help
// menu.

// barScaleLevels are the factors the bar's text may have, smallest first.
var barScaleLevels = []int{1, 2, 3}

// barFitNumer and barFitDenom are the share of the window's width that the labels may take, as a
// fraction: four fifths, which is 80%. barHeightDenom is the share of its height that the strip may take: one
// eighth.
const (
	barFitNumer    = 4
	barFitDenom    = 5
	barHeightDenom = 8
)

// barMinPad is the least room, in pixels, above the bar's text and the same below it.
const barMinPad = 4

// barPadFor is the room above a line of text lineH high, and the same below it: half its height,
// and no less than barMinPad. The text takes half of the strip it is in, or less.
func barPadFor(lineH int) int { return max(barMinPad, lineH/2) }

// fitBarScale is the largest of barScaleLevels that fits a window of screenW by screenH: widthAt
// is the width in pixels that the labels and their blank spaces take at a factor, and stripAt the
// height of the strip that holds the text at it, the room round the text included. The labels
// may take 80% of screenW and the strip an eighth of screenH. When not even the
// smallest fits, it is the smallest: the bar is never smaller than it was.
func fitBarScale(screenW, screenH int, widthAt, stripAt func(scale int) int) int {
	best := barScaleLevels[0]
	for _, k := range barScaleLevels {
		if widthAt(k)*barFitDenom <= screenW*barFitNumer && stripAt(k)*barHeightDenom <= screenH {
			best = k
		}
	}
	return best
}
