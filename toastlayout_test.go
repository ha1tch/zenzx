package zenzx

import (
	"strings"
	"testing"
)

// A font with 8-pixel cells, as the Sinclair face is.
func cells(s string, scale int) int { return len([]rune(s)) * 8 * scale }

func TestToastFitsAtTheUsualScaleWhenItCan(t *testing.T) {
	lines, scale := toastLayout("Loaded game.tap", cells, 400)
	if scale != 2 || len(lines) != 1 || lines[0] != "Loaded game.tap" {
		t.Fatalf("%q at %d", lines, scale)
	}
}

func TestToastGoesDownANotchBeforeItWraps(t *testing.T) {
	// 20 letters: 320 px at scale 2 (+5 for the ink), 160 at scale 1 (+3).
	text := "abcdefghijklmnopqrst"
	lines, scale := toastLayout(text, cells, 200)
	if scale != 1 || len(lines) != 1 || lines[0] != text {
		t.Fatalf("%q at %d", lines, scale)
	}
	// The ink counts: at exactly the width of the text it no longer fits at scale 2.
	if _, s := toastLayout(text, cells, 320); s != 1 {
		t.Fatalf("a toast as wide as the cap, ink and all, was drawn at scale %d", s)
	}
	if _, s := toastLayout(text, cells, 325); s != 2 {
		t.Fatalf("scale %d, want 2 when the ink fits", s)
	}
}

func TestToastWrapsBetweenWordsAtTheSmallerScale(t *testing.T) {
	text := "The tape long-name.tap is loaded and rewound; press PLAY on the tape to start it"
	for _, maxW := range []int{120, 200, 333, 640} {
		lines, scale := toastLayout(text, cells, maxW)
		if scale != 1 && len(lines) > 1 {
			t.Fatalf("maxW %d: %d lines at scale %d", maxW, len(lines), scale)
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

func TestToastBreaksAWordThatIsWiderThanTheCap(t *testing.T) {
	lines, scale := toastLayout("supercalifragilisticexpialidocious", cells, 100)
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
	lines, _ := toastLayout("abc", cells, 4)
	if strings.Join(lines, "") != "abc" || len(lines) != 3 {
		t.Fatalf("%q", lines)
	}
	if l, _ := toastLayout("", cells, 100); len(l) != 1 {
		t.Fatalf("an empty toast: %q", l)
	}
}

func TestToastKeepsAHardLineBreak(t *testing.T) {
	lines, scale := toastLayout("one\ntwo", cells, 1000)
	if len(lines) != 2 || scale != 2 {
		t.Fatalf("%q at %d", lines, scale)
	}
}
