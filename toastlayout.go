// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "strings"

// A toast is never wider, nor taller, than half the window. toastLayout is the rule, kept apart from
// the drawing so that it can be tested without a window: the text at the usual scale, on one line
// if it fits and otherwise broken between words into as many lines as it takes; and only when
// that is taller than half the window, or has a word wider than a line, one notch down, where it is
// broken in the same way (and a word wider than a line is cut).
const (
	toastScale    = 2 // the usual size of the text, in pixels per font pixel
	toastMinScale = 1 // one notch down, and the smallest
)

// toastInk is what the stroke and the doubled face add to a line's width at a scale: the
// stroke goes one font pixel to each side, and the face is drawn twice, a screen pixel apart,
// to be bolder. A toast's width is counted with it, because it is what is seen.
func toastInk(scale int) int { return 2*scale + 1 }

// toastLayout fits text into maxW by maxH pixels (a maxH of 0 is no limit on the height). measure
// gives a string's width at a scale and cellH is the font's height at scale 1. It returns the lines
// and the scale they are drawn at. A word wider than a whole line is cut where the line ends, and a
// line is never empty, so the loop below always ends.
func toastLayout(text string, measure func(s string, scale int) int, cellH, maxW, maxH int) (lines []string, scale int) {
	text = strings.TrimSpace(text)
	fits := func(s string, sc int) bool { return measure(s, sc)+toastInk(sc) <= maxW }
	paras := strings.Split(text, "\n")
	// wrap is the paragraphs at a scale, each on one line where it fits and else broken between
	// words, and whether every word is whole on its line.
	wrap := func(sc int) (out []string, whole bool) {
		whole = true
		for _, para := range paras {
			if fits(para, sc) {
				out = append(out, para)
				continue
			}
			for _, word := range strings.Fields(para) {
				whole = whole && fits(word, sc)
			}
			out = append(out, wrapToast(para, func(s string) bool { return fits(s, sc) })...)
		}
		return out, whole
	}
	for sc := toastScale; sc > toastMinScale; sc-- {
		out, whole := wrap(sc)
		if whole && (maxH <= 0 || len(out)*cellH*sc <= maxH) {
			return out, sc
		}
	}
	lines, _ = wrap(toastMinScale)
	return lines, toastMinScale
}

// wrapToast breaks one paragraph into lines that each satisfy fits.
func wrapToast(para string, fits func(string) bool) []string {
	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}
	for _, word := range strings.Fields(para) {
		for {
			try := word
			if cur != "" {
				try = cur + " " + word
			}
			if fits(try) {
				cur = try
				break
			}
			if cur != "" { // the word starts a line of its own
				flush()
				continue
			}
			// A word wider than a line: take as many of its letters as fit, and at least one.
			r := []rune(word)
			n := 1
			for n < len(r) && fits(string(r[:n+1])) {
				n++
			}
			lines = append(lines, string(r[:n]))
			word = string(r[n:])
			if word == "" {
				break
			}
		}
	}
	flush()
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}
