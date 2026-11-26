// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "time"

// A window that closes does not vanish: for a moment it is drawn again, fainter and a little
// smaller each frame, until nothing is left of it (dialogfade_gui.go does the drawing). This file
// is the curve, kept apart from the drawing so that it can be tested without a window.
const (
	// dialogFadeTime is how long a closed window takes to go: long enough to be seen, short enough
	// that nothing waits for it. The window beneath it is already live.
	dialogFadeTime = 160 * time.Millisecond
	// dialogFadeEnd is the size the window has when it is gone, as a fraction of its size.
	dialogFadeEnd = 0.94
)

// dialogFadeAt is how a closed window looks elapsed after it was closed: its opacity (1 down to 0), its
// size as a fraction of the size it had (1 down to dialogFadeEnd), and whether it is gone.
func dialogFadeAt(elapsed time.Duration) (alpha, scale float32, done bool) {
	if elapsed <= 0 {
		return 1, 1, false
	}
	if elapsed >= dialogFadeTime {
		return 0, dialogFadeEnd, true
	}
	p := float32(elapsed) / float32(dialogFadeTime)
	return 1 - p, 1 - (1-dialogFadeEnd)*p, false
}
