//go:build !headless

package zenzx

import (
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

func TestAcknowledgementsWindowReadsTheFileInTheTree(t *testing.T) {
	text := acknowledgementsModalText(acknowledgementsSource)
	lines := strings.Split(text, "\n")

	if lines[0] != "# Acknowledgements" {
		t.Errorf("first line = %q, want the file's title as a heading", lines[0])
	}
	for _, name := range []string{"Thomas Sturm", "Martian Girl / Marta", "Steve Wetherill", "Jim Blimey"} {
		if !strings.Contains(text, "## "+name+"\n") {
			t.Errorf("%q is not a heading of the window", name)
		}
	}
	// A link whose text is not its address keeps its address; one whose text is the
	// address, or has it in it, does not repeat it.
	// A line break may fall inside a phrase, or inside an address, where the wrapper breaks a
	// long word at a slash; so what is compared has no white space in it at all.
	squash := func(s string) string { return strings.Join(strings.Fields(s), "") }
	flat := squash(text)
	for _, want := range []string{
		"BitPrism (https://gitlab.com/thomassturm/bitprism)",
		"blog (https://blog.stevewetherill.com)",
		"Substack (https://stevewetherill.substack.com)",
		"sturm.to",
		"www.jimblimey.com",
	} {
		if !strings.Contains(flat, squash(want)) {
			t.Errorf("the window lacks %q", want)
		}
	}
	if strings.Contains(flat, "(https://sturm.to)") || strings.Contains(flat, "(https://www.jimblimey.com)") {
		t.Error("a link whose text is its address repeats the address")
	}
	if !strings.Contains(flat, squash("Thank you all.")) {
		t.Error("the window ends before the file does")
	}

	for i, l := range lines {
		for _, r := range l {
			if r > 126 {
				t.Errorf("line %d has %q, which the font has no glyph for: %q", i, r, l)
				break
			}
		}
		if len(l) > 70 {
			t.Errorf("line %d is %d columns, over the reader's 70: %q", i, len(l), l)
		}
		if strings.Contains(l, "](") || strings.Contains(l, "**") {
			t.Errorf("line %d still has markdown in it: %q", i, l)
		}
	}
}

func TestHelpMenuOpensAcknowledgementsAndAboutPointsToThem(t *testing.T) {
	zx := testZX()
	b, err := newAppMenuBar(zx, "./custom-roms", zenui.ThemeDark, "48k", nil, "")
	if err != nil {
		t.Fatalf("newAppMenuBar: %v", err)
	}
	defer b.text.Unload()

	b.openLogoMenu(800)
	row := helpRow(t, b, "Acknowledgements...")
	b.activeModal = nil
	if !b.logoSpec.pick(zx, row, -1) {
		t.Fatal("picking Acknowledgements... ran nothing")
	}
	if m := b.activeModal; m == nil || m.Title() != "ACKNOWLEDGEMENTS" {
		t.Errorf("activeModal = %#v, want the Acknowledgements window", m)
	}

	if !strings.Contains(aboutTextTemplate, acknowledgementsURL) {
		t.Errorf("About does not point to %s", acknowledgementsURL)
	}
	if !strings.Contains(aboutTextTemplate, "Help > Acknowledgements...") {
		t.Error("About does not say where in the program the window is")
	}
}
