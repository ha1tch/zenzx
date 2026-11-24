//go:build !headless

package zenzx

import (
	"sort"
	"strings"
	"testing"

	"github.com/ha1tch/zenui"
)

// textAt is a renderer that records where each text was drawn, for finding a link on the
// screen of a window drawn with it; the rest is testNoopRenderer's.
type textAt struct {
	testNoopRenderer
	at *[]drawnText
}

type drawnText struct {
	s    string
	x, y int
}

func (r textAt) DrawText(s string, x, y, scale int, _ zenui.Colour) {
	*r.at = append(*r.at, drawnText{s, x, y})
}

// clickOn draws m, finds the text drawn as s and presses the left button in the middle of it.
func clickOn(t *testing.T, m *zenui.TextModal, s string) {
	t.Helper()
	r := textAt{at: &[]drawnText{}}
	m.Draw(r, 1400, 1000, zenui.DarkTheme())
	for _, d := range *r.at {
		if d.s == s {
			in := zenui.Input{MouseX: d.x + testNoopRenderer{}.TextWidth(s, 2)/2, MouseY: d.y + 6, MousePressed: true, MouseDown: true}
			if !m.Update(in) {
				t.Errorf("a click on %q closed the window", s)
			}
			return
		}
	}
	t.Fatalf("%q was not drawn", s)
}

// opened replaces openLink for the length of a test and returns what it was asked to open.
func opened(t *testing.T) *[]string {
	t.Helper()
	var got []string
	was := openLink
	openLink = func(u string) { got = append(got, u) }
	t.Cleanup(func() { openLink = was })
	return &got
}

func TestAddressesInAboutAreLinks(t *testing.T) {
	got := opened(t)
	m := newAboutModal(strings.ReplaceAll(aboutTextTemplate, "__VERSION__", "0.0.0"))
	for _, url := range []string{
		"https://ha1tch.github.io/zsp/projects/zenzx",
		acknowledgementsURL,
		"https://www.apache.org/licenses/LICENSE-2.0",
	} {
		clickOn(t, m, url)
		if n := len(*got); n == 0 || (*got)[n-1] != url {
			t.Errorf("a click on %s opened %q", url, *got)
		}
	}
}

func TestLinksOfAcknowledgementsGoToTheirAddresses(t *testing.T) {
	src := ackLink.FindAllStringSubmatch(acknowledgementsSource, -1)
	if len(src) < 5 {
		t.Fatalf("%d links in ACKNOWLEDGEMENTS.md, want the file's many", len(src))
	}
	wantURLs := map[string]bool{}
	for _, m := range src {
		wantURLs[m[2]] = true
	}
	// What the window is given: every link there is, each in one piece on its line, to the
	// address the file gives it, and no other.
	raw := acknowledgementsModalText(acknowledgementsSource)
	gotURLs := map[string]bool{}
	for i, l := range strings.Split(raw, "\n") {
		found := ackLink.FindAllStringSubmatch(l, -1)
		if strings.Count(l, "[") != len(found) || strings.Count(l, "](") != len(found) {
			t.Errorf("line %d has a link broken by the wrapping: %q", i, l)
		}
		for _, m := range found {
			gotURLs[m[2]] = true
			if strings.TrimSpace(m[1]) == "" {
				t.Errorf("line %d has a link with no text: %q", i, l)
			}
		}
	}
	var missing, extra []string
	for u := range wantURLs {
		if !gotURLs[u] {
			missing = append(missing, u)
		}
	}
	for u := range gotURLs {
		if !wantURLs[u] {
			extra = append(extra, u)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("the window's links lack %q and add %q", missing, extra)
	}
}

func TestAcknowledgementsLinkIsClickable(t *testing.T) {
	got := opened(t)
	m := newAcknowledgementsModal()
	clickOn(t, m, "sturm.to")
	if len(*got) != 1 || (*got)[0] != "https://sturm.to" {
		t.Errorf("a click on a link opened %q, want https://sturm.to", *got)
	}
}

// The info windows show what a file says about itself (a tape's block names, a disk's labels):
// an address in it is text.
func TestInfoWindowsHaveNoLinks(t *testing.T) {
	got := opened(t)
	b := &appMenuBar{}
	b.showInfo("TAPE INFO", "## Blocks\n\nHeader 'https://example.org/x'")
	r := textAt{at: &[]drawnText{}}
	b.activeModal.Draw(r, 1400, 1000, zenui.DarkTheme())
	for _, d := range *r.at {
		if strings.Contains(d.s, "https://example.org/x") {
			b.activeModal.Update(zenui.Input{MouseX: d.x + 10, MouseY: d.y + 6, MousePressed: true})
		}
	}
	if len(*got) != 0 || b.activeModal.OverLink() {
		t.Errorf("an address in an info window opened %q", *got)
	}
}
