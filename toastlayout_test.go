package zenzx

import (
	"strings"
	"testing"
)

// A font with 8-pixel cells, as the Sinclair face is.
func cells(s string, scale int) int { return len([]rune(s)) * 8 * scale }

const cellH = 8 // and 8 pixels high

func TestToastFitsAtTheUsualScaleWhenItCan(t *testing.T) {
	lines, scale := toastLayout("Loaded game.tap", cells, cellH, 400, 0)
	if scale != 2 || len(lines) != 1 || lines[0] != "Loaded game.tap" {
		t.Fatalf("%q at %d", lines, scale)
	}
}

// A message too long for one line is wrapped at the usual scale, not drawn smaller on one line.
func TestToastWrapsAtTheUsualScaleBeforeItGoesDown(t *testing.T) {
	// 30 letters: 480 px at scale 2, 240 at scale 1 -- more than the 200 there is, at either.
	text := "alpha beta gamma delta epsilon"
	lines, scale := toastLayout(text, cells, cellH, 200, 0)
	if scale != 2 || len(lines) != 3 || lines[0] != "alpha beta" || lines[1] != "gamma delta" || lines[2] != "epsilon" {
		t.Fatalf("%q at %d, want three lines at scale 2", lines, scale)
	}
	// The same words in 260 px fit one line at scale 1 (243) but are wrapped at scale 2 all the same.
	lines, scale = toastLayout(text, cells, cellH, 260, 0)
	if scale != 2 || len(lines) < 2 {
		t.Fatalf("%q at %d: it fits on one line at scale 1, but is to be wrapped at scale 2", lines, scale)
	}
	// The ink counts: a word as wide as the cap, ink and all, does not fit it at scale 2.
	if l, s := toastLayout("abcdefghijklmnopqrst", cells, cellH, 320, 0); s != 2-1 {
		t.Fatalf("%q at %d: a word as wide as the cap, ink and all, was kept at scale 2", l, s)
	}
	if l, s := toastLayout("abcdefghijklmnopqrst", cells, cellH, 325, 0); len(l) != 1 || s != 2 {
		t.Fatalf("%q at %d, want one line at scale 2 when the ink fits", l, s)
	}
}

// It goes down a notch only when the lines at the usual scale would stand taller than the room.
func TestToastGoesDownANotchWhenTheLinesAreTooTall(t *testing.T) {
	text := "alpha beta gamma delta epsilon"
	// three lines of 16 pixels are 48 high
	if lines, scale := toastLayout(text, cells, cellH, 200, 48); scale != 2 || len(lines) != 3 {
		t.Fatalf("%q at %d with room for them", lines, scale)
	}
	lines, scale := toastLayout(text, cells, cellH, 200, 47)
	if scale != 1 || len(lines) != 2 || lines[0] != "alpha beta gamma delta" {
		t.Fatalf("%q at %d, want two lines at scale 1 where three at scale 2 are too tall", lines, scale)
	}
}

func TestToastWrapsBetweenWords(t *testing.T) {
	text := "The tape long-name.tap is loaded and rewound; press PLAY on the tape to start it"
	for _, maxW := range []int{120, 200, 333, 640} {
		lines, scale := toastLayout(text, cells, cellH, maxW, 0)
		if scale == 1 && maxW >= 333 {
			t.Fatalf("maxW %d: drawn at scale 1, where wrapping at scale 2 would do", maxW)
		}
		for _, l := range lines {
			if cells(l, scale)+toastInk(scale) > maxW {
				t.Errorf("maxW %d: %q is %d wide", maxW, l, cells(l, scale)+toastInk(scale))
			}
		}
		if got := strings.Join(strings.Fields(strings.Join(lines, " ")), " "); got != text {
			t.Errorf("maxW %d: the words changed: %q", maxW, got)
		}
	}
}

// A word too wide for a line at the usual scale takes the smaller one, and is cut only when it does
// not fit a line at that either.
func TestToastBreaksAWordThatIsWiderThanTheCap(t *testing.T) {
	if lines, scale := toastLayout("see supercalifragilisticexp now", cells, cellH, 220, 0); scale != 1 || len(lines) != 2 || lines[0] != "see supercalifragilisticexp" {
		t.Fatalf("a word that fits a line only at scale 1: %q at %d", lines, scale)
	}
	lines, scale := toastLayout("supercalifragilisticexpialidocious", cells, cellH, 100, 0)
	if scale != 1 || len(lines) < 3 {
		t.Fatalf("%q at %d", lines, scale)
	}
	if strings.Join(lines, "") != "supercalifragilisticexpialidocious" {
		t.Fatalf("letters lost: %q", lines)
	}
	for _, l := range lines {
		if cells(l, 1)+toastInk(1) > 100 {
			t.Errorf("%q is too wide", l)
		}
	}
}

func TestToastOnAWindowTooNarrowForALetterStillEnds(t *testing.T) {
	lines, _ := toastLayout("abc", cells, cellH, 4, 0)
	if strings.Join(lines, "") != "abc" || len(lines) != 3 {
		t.Fatalf("%q", lines)
	}
	if l, _ := toastLayout("", cells, cellH, 100, 0); len(l) != 1 {
		t.Fatalf("an empty toast: %q", l)
	}
}

func TestToastKeepsAHardLineBreak(t *testing.T) {
	lines, scale := toastLayout("one\ntwo", cells, cellH, 1000, 0)
	if len(lines) != 2 || scale != 2 {
		t.Fatalf("%q at %d", lines, scale)
	}
}
