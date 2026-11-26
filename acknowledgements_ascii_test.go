//go:build !headless

package zenzx

import (
	"strings"
	"testing"
)

// The window's font has nothing beyond ASCII, so what it is given has none: the Spanish title of
// Rodolfo Guerra's book is written without its accent, and nothing else of the file gets through
// as a character the font cannot draw.
func TestAcknowledgementsWindowTextIsASCII(t *testing.T) {
	text := acknowledgementsModalText(acknowledgementsSource)
	for i, r := range text {
		if r > 0x7e || (r < 0x20 && r != '\n') {
			t.Errorf("character %q (U+%04X) at byte %d of the window's text", r, r, i)
			break
		}
	}
	if !strings.Contains(strings.Join(strings.Fields(text), " "), "Generacion TK90X") {
		t.Error("the window does not name the book Generacion TK90X")
	}
	if !strings.Contains(acknowledgementsSource, "*Generación TK90X*") {
		t.Error("ACKNOWLEDGEMENTS.md has not the book's title as published")
	}
}
