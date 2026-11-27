//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

// The toolbar's pictures for the windows it opens (barbuttons_gui.go), in the transport sprites'
// notation (hud_draw_gui.go): sixteen rows of ink/mask, one colour, drawn whole-pixel by
// transportIcon. The clock is the "icon-clock" sprite of gui-elements.zplay; the others are drawn
// in its manner (one pixel line, square corners) because that package has no cassette, floppy
// disk or SD card, and its "icon-picture" is a dither that is too dense at this size.
var toolbarSprites = map[string][16]string{
	// tape is a cassette, for the Tapes button.
	"tape": {
		"0000/0000", "0000/0000", "0000/0000", "7ffe/7ffe", "4002/4002", "5ffa/5ffa",
		"566a/566a", "566a/566a", "5ffa/5ffa", "4002/4002", "47e2/47e2", "7ffe/7ffe",
		"0000/0000", "0000/0000", "0000/0000", "0000/0000",
	},
	// disk is a floppy disk, for the Disks button.
	"disk": {
		"0000/0000", "7ffe/7ffe", "4002/4002", "4fe2/4fe2", "49e2/49e2", "4fe2/4fe2",
		"4002/4002", "4002/4002", "5ffa/5ffa", "500a/500a", "500a/500a", "500a/500a",
		"5ffa/5ffa", "4002/4002", "7ffe/7ffe", "0000/0000",
	},
	// card is an SD card with its corner cut, for the SD Cards button.
	"card": {
		"0000/0000", "1ff0/1ff0", "1010/1010", "1548/1548", "1544/1544", "1542/1542",
		"1002/1002", "1002/1002", "1002/1002", "1002/1002", "1002/1002", "1002/1002",
		"1ffe/1ffe", "0000/0000", "0000/0000", "0000/0000",
	},
	// snapshot is the clock of gui-elements.zplay ("icon-clock"): a moment held, for the Snapshots button.
	"snapshot": {
		"0000/0000", "0ff0/0ff0", "1008/1008", "2104/2104", "2104/2104", "2104/2104",
		"2104/2104", "2104/2104", "21e4/21e4", "2004/2004", "2004/2004", "2004/2004",
		"2004/2004", "1008/1008", "0ff0/0ff0", "0000/0000",
	},
	// screenshot is a picture, a sun over hills in a frame, for the Screenshots button.
	"screenshot": {
		"0000/0000", "7ffe/7ffe", "4002/4002", "4c02/4c02", "4c42/4c42", "40e2/40e2",
		"49f2/49f2", "5ffc/5ffc", "7ffe/7ffe", "0000/0000", "0000/0000", "0000/0000",
		"0000/0000", "0000/0000", "0000/0000", "0000/0000",
	},
}

// toolbarIcons are the toolbar sprites, parsed once; a sprite that does not parse is absent.
var toolbarIcons = parseToolbarSprites()

func parseToolbarSprites() map[string][16]iconRow {
	out := make(map[string][16]iconRow, len(toolbarSprites))
	for name, rows := range toolbarSprites {
		if icon, err := parseIcon(rows); err == nil {
			out[name] = icon
		}
	}
	return out
}
