//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "github.com/ha1tch/zenui"

// drawDialogHead draws the lines of text at the head of a dialog, from (x, y) down by lineH a
// line, and gives back the y below the last. No line is let out of the box: one that is wider
// than width pixels at scale is cut short with two dots. A line that names a folder is cut at its
// start, because the end of a path says the most; the others are cut at their end.
//
// The dialogs are laid out in lines of text whose lengths depend on what the person has: the
// name of a folder, of a disk, of a card. Each dialog's own text is kept short, and this is what
// makes sure that a long name cannot push a line out of the box all the same.
func drawDialogHead(r zenui.Renderer, lines []string, x, y, width, scale, lineH int, theme zenui.Theme) int {
	for _, line := range lines {
		zenui.DrawTextLine(r, fitDialogLine(r, line, width, scale), x, y, scale, theme)
		y += lineH
	}
	return y
}

// fitDialogLine is line made to fit in width pixels at scale; see drawDialogHead.
func fitDialogLine(r zenui.Renderer, line string, width, scale int) string {
	if isFolderLine(line) {
		return fitTail(r, line, width, scale)
	}
	return fitHead(r, line, width, scale)
}

// isFolderLine says whether a head line is the one that names the folder a window lists.
func isFolderLine(line string) bool {
	const prefix = "Folder: "
	return len(line) >= len(prefix) && line[:len(prefix)] == prefix
}

// drawWrapped draws text in colour from (x, y), broken at its spaces into lines no wider than
// width pixels, at most maxLines of them, a line apart by lineH. When there is more than the lines
// have room for, the last one ends in two dots. It gives back the number of lines drawn.
func drawWrapped(r zenui.Renderer, text string, x, y, width, scale, lineH, maxLines int, colour zenui.Colour) int {
	lines := wrapToWidth(r, text, width, scale)
	for i, line := range lines {
		if i >= maxLines {
			return maxLines
		}
		if i == maxLines-1 && len(lines) > maxLines {
			line = fitHead(r, line+" ...", width, scale)
		}
		r.DrawText(line, x, y+i*lineH, scale, colour)
	}
	return min(len(lines), maxLines)
}
