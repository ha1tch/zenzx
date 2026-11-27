//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"path/filepath"
	"time"
)

// View > Screenshot: the formats the picture on the machine can be saved in (screenshot.go), the
// one it is not applicable to greyed, and Manage Screenshots, the window that lists what has
// been saved (screenshots_gui.go).

// screenshotLabels are the rows of the Screenshot submenu.
var screenshotLabels = map[ShotFormat]string{
	ShotPNG:      "PNG",
	ShotSCR:      "SCR",
	ShotHiRes:    "Hi-Res SCR",
	ShotHiColour: "Hi-Colour SCR",
	ShotLayer2:   "Layer 2",
	ShotLoRes:    "LoRes",
	ShotGIF:      "GIF",
}

// screenshotSpec is the Screenshot submenu as the machine stands: a row for each format, enabled
// when the picture can be saved in it, and the window that manages what was saved.
func (b *appMenuBar) screenshotSpec(zx *ZenZX) menuSpec {
	spec := make(menuSpec, 0, len(ShotFormats)+2)
	for _, f := range ShotFormats {
		f := f
		spec = append(spec, menuEntry{
			label:    screenshotLabels[f],
			disabled: zx.stopped || !zx.ShotApplicable(f),
			run:      func(zx *ZenZX) { b.saveScreenshot(zx, f) },
		})
	}
	return append(spec, menuSep(), menuRow("Manage Screenshots...", func(zx *ZenZX) { b.openScreenshots(zx) }))
}

// saveScreenshot saves the machine's picture in a format and says where it went. A GIF is two
// seconds of the machine running: it starts now, and its toast comes when it has been written.
func (b *appMenuBar) saveScreenshot(zx *ZenZX, f ShotFormat) {
	border := zx.window().borderView().Shown
	if f == ShotGIF {
		switch {
		case !zx.StartGIF(border, time.Now()):
			zx.notice("A GIF is being recorded already")
		case zx.paused:
			zx.notice("The machine is paused: the GIF is recorded when it plays")
		default:
			zx.notice("Recording the next two seconds")
		}
		return
	}
	path, err := zx.SaveScreenshot(f, border, time.Now())
	if err != nil {
		zx.notice("The screenshot could not be saved: %v", err)
		return
	}
	zx.notice("Saved %s", filepath.Base(path))
}
