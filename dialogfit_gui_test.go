//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// textSpan is one piece of text a window drew, and where.
type textSpan struct {
	text        string
	x, y, scale int
}

// spanRenderer notes where every piece of text goes.
type spanRenderer struct {
	paintRenderer
	spans []textSpan
}

func (s *spanRenderer) DrawText(text string, x, y, scale int, c zenui.Colour) {
	s.spans = append(s.spans, textSpan{text, x, y, scale})
	s.paintRenderer.DrawText(text, x, y, scale, c)
}

// drawsInside draws a window and says what it drew that is wider than its panel.
func drawsInside(t *testing.T, name string, draw func(r zenui.Renderer, w, h int), panel func() zenui.Rect, size [2]int) {
	t.Helper()
	r := &spanRenderer{}
	draw(r, size[0], size[1])
	p := panel()
	for _, s := range r.spans {
		if s.x < p.X || s.x+r.TextWidth(s.text, s.scale) > p.X+p.W {
			t.Errorf("%s at %v: %q is drawn from %d to %d, outside the dialog's %d to %d",
				name, size, s.text, s.x, s.x+r.TextWidth(s.text, s.scale), p.X, p.X+p.W)
		}
	}
}

// No text in a dialog goes beyond the dialog, however long the name of the folder or of the file
// it shows: the windows cut what does not fit.
func TestDialogTextNeverLeavesTheBox(t *testing.T) {
	long := filepath.Join(t.TempDir(), strings.Repeat("a-very-long-folder-name-", 6))
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Fatal(err)
	}
	theme := zenui.DarkTheme()
	for _, size := range [][2]int{{960, 768}, {640, 480}, {352, 296}} {
		sn := newSnapshotsWindow(long)
		drawsInside(t, "snapshots", func(r zenui.Renderer, w, h int) { sn.Draw(r, w, h, theme) },
			func() zenui.Rect { return sn.panel }, size)

		sh := newScreenshotsWindow(long)
		sh.noGL = true
		drawsInside(t, "screenshots", func(r zenui.Renderer, w, h int) { sh.Draw(r, w, h, theme) },
			func() zenui.Rect { return sh.panel }, size)

		tp := newTapesWindow(long, func() []string { return nil }, nil)
		drawsInside(t, "tapes", func(r zenui.Renderer, w, h int) { tp.Draw(r, w, h, theme) },
			func() zenui.Rect { return tp.panel }, size)

		dzx, _ := diskMachine(t)
		dk := soloDisks(dzx)
		dk.dir = long
		drawsInside(t, "disks", func(r zenui.Renderer, w, h int) { dk.Draw(r, w, h, theme) },
			func() zenui.Rect { return dk.panel }, size)

		dir := testCardDir(t)
		zx := NewZenZX(AudioBackendOto)
		for _, must := range []bool{false, true} {
			cd := newCardsWindow(zx, 2, dir, false, func(string) cardUse { return cardUse{} })
			cd.must, cd.taken = must, filepath.Join(dir, "a.img")
			drawsInside(t, "cards", func(r zenui.Renderer, w, h int) { cd.Draw(r, w, h, theme) },
				func() zenui.Rect { return cd.panel }, size)
		}
	}
}

// The text of the dialogs' own is short enough to show whole at the usual size, with no folder
// in it to be long.
func TestSnapshotsHeadIsShort(t *testing.T) {
	w := newSnapshotsWindow(t.TempDir())
	for _, line := range w.headLines()[1:] {
		if n := len([]rune(line)); n > 60 {
			t.Errorf("%q is %d characters", line, n)
		}
	}
}
