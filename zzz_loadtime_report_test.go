package main

// One-off load-time measurement for two specific titles, requested
// directly rather than via the 31-game corpus sweep. Reuses
// zzz_batch_test.go's runOneMode/pristineBootHash unchanged -- same
// evidence standard (stop-tape/menu screen, verified against the
// pristine-boot and post-LOAD-command baselines, not merely "a
// screenshot exists") and same accurate/fast/turbo three-mode
// coverage. NOT part of the permanent 31-title batchGames list: these
// two entries (particularly the 128K Chase H.Q. TAP, distinct from the
// 48K TZX already in that list) are scoped to this request only.
//
// Run: go test -tags headless -count=1 -run '^TestLoadTimeReport$' -v .

import (
	"fmt"
	"os"
	"testing"
)

var loadTimeGames = []batchGame{
	{"Chase H.Q. (128K)", "Chase H.Q/Chase HQ 128K.tap", "128k"},
	{"Cybernoid 2 (128K)", "Cybernoid II - The Revenge/Cybernoid 2 - Side 1.tzx", "128k"},
}

func TestLoadTimeReport(t *testing.T) {
	os.MkdirAll(shotDir, 0755)
	bootHash128 := pristineBootHash(t, "128k")

	for _, g := range loadTimeGames {
		t.Logf("=== %s (%s) ===", g.Name, g.Model)
		for _, mode := range []struct {
			m    TapeMode
			name string
		}{
			{TapeFast, "fast"},
			{TapeAccurate, "accurate"},
			{TapeTurbo, "turbo"},
		} {
			res := runOneMode(t, g, mode.m, mode.name, bootHash128)
			wallStr := fmt.Sprintf("%.2fs", res.WallSeconds)
			frameSecs := float64(res.Frames) / 50.0 // 50Hz PAL frame rate
			t.Logf("  %-9s completed=%-5v verified=%-5v frames=%-7d emulated=%.2fs  wall=%s  shots=[%s , %s]  %s",
				mode.name, res.Completed, res.Verified, res.Frames, frameSecs, wallStr,
				res.ScreenshotA, res.ScreenshotB, res.ErrorNote)
		}
	}
}
