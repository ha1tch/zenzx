//go:build !headless

package zenzx

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The first-start dialog's states, with the download replaced: nothing here reaches the
// network or needs a window.

// until gives the dialog frames until it reports something other than "still open", or
// fails after two seconds.
func untilNextSetup(t *testing.T, s *nextSetup, stop func() bool) nextSetupResult {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r := s.update(zenui.Input{}); r != nextSetupOpen || stop() {
			return r
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the dialog did not get there within two seconds")
	return nextSetupOpen
}

func TestNextSetupAsksBeforeAnythingRuns(t *testing.T) {
	s := newNextSetup(t.TempDir(), "")
	ran := false
	s.run = func(context.Context, nextcard.Options, nextcard.Progress) error { ran = true; return nil }
	if s.phase != nextSetupAsking {
		t.Fatal("the dialog does not start by asking")
	}
	for i := 0; i < 5; i++ {
		if r := s.update(zenui.Input{}); r != nextSetupOpen {
			t.Fatalf("the dialog closed by itself: %v", r)
		}
	}
	if ran {
		t.Fatal("the download started without confirmation")
	}
	if r := s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}}); r != nextSetupDeclined {
		t.Fatalf("Escape gave %v, want the dialog declined", r)
	}
	if ran {
		t.Fatal("the download started although the dialog was declined")
	}
}

func TestNextSetupDownloadsWithTheAddressesShown(t *testing.T) {
	dir := t.TempDir()
	s := newNextSetup(dir, "")
	if s.distURL != nextcard.DefaultDistributionURL || s.bootURL != nextcard.DefaultBootROMURL {
		t.Fatal("the dialog does not start with the default addresses")
	}
	s.distURL = "  https://example.org/other.zip "
	var got nextcard.Options
	release := make(chan struct{})
	s.run = func(_ context.Context, opts nextcard.Options, progress nextcard.Progress) error {
		got = opts
		progress(nextcard.StageDownload, 30, 120)
		<-release
		return nil
	}
	s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyEnter}})
	if s.phase != nextSetupRunning {
		t.Fatal("Enter did not start the download")
	}
	untilNextSetup(t, s, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.total == 120
	})
	s.mu.Lock()
	stage, done, total := s.stage, s.done, s.total
	s.mu.Unlock()
	if stage != nextcard.StageDownload || done != 30 || total != 120 {
		t.Fatalf("progress seen by the dialog: stage %v, %d of %d", stage, done, total)
	}
	if line := zenui.ProgressText(done, total); !strings.HasPrefix(line, " 25%") {
		t.Fatalf("progress line %q, want 25%%", line)
	}
	close(release)
	if r := untilNextSetup(t, s, func() bool { return false }); r != nextSetupReady {
		t.Fatalf("after the work finished the dialog reported %v, want ready", r)
	}
	if got.Dir != dir || got.DistributionURL != "https://example.org/other.zip" || got.BootROMURL != nextcard.DefaultBootROMURL {
		t.Fatalf("the work was given %+v", got)
	}
}

func TestNextSetupFailureCanBeRetried(t *testing.T) {
	s := newNextSetup(t.TempDir(), "")
	calls := 0
	s.run = func(context.Context, nextcard.Options, nextcard.Progress) error {
		calls++
		if calls == 1 {
			return errors.New("the distribution: no route to host")
		}
		return nil
	}
	s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyEnter}})
	untilNextSetup(t, s, func() bool { return s.phase == nextSetupFailed })
	if s.phase != nextSetupFailed {
		t.Fatal("a failed download did not bring up the failure dialog")
	}
	// Enter on the failure dialog is "Try again".
	s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyEnter}})
	if r := untilNextSetup(t, s, func() bool { return false }); r != nextSetupReady || calls != 2 {
		t.Fatalf("after trying again: %v with %d runs, want ready after 2", r, calls)
	}
}

func TestNextSetupCancelStopsTheWork(t *testing.T) {
	s := newNextSetup(t.TempDir(), "")
	s.run = func(ctx context.Context, _ nextcard.Options, _ nextcard.Progress) error {
		<-ctx.Done()
		return ctx.Err()
	}
	s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyEnter}})
	if r := s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyEscape}}); r != nextSetupOpen {
		t.Fatalf("Cancel closed the dialog before the work had stopped: %v", r)
	}
	if r := untilNextSetup(t, s, func() bool { return false }); r != nextSetupDeclined {
		t.Fatalf("after Cancel the dialog reported %v, want declined", r)
	}
}

// paintRenderer records what the window draws: every filled rectangle with its colour, and
// every text. Text is 6 pixels a character and 8 high at scale 1.
type paintRenderer struct {
	fills []paintFill
	texts []string
}

type paintFill struct {
	rect   zenui.Rect
	colour zenui.Colour
}

func (p *paintRenderer) FillRect(r zenui.Rect, c zenui.Colour) {
	p.fills = append(p.fills, paintFill{r, c})
}
func (p *paintRenderer) StrokeRect(zenui.Rect, zenui.Colour, int)                 {}
func (p *paintRenderer) FillRectGradientV(zenui.Rect, zenui.Colour, zenui.Colour) {}
func (p *paintRenderer) FillRectGradientHMultiply(zenui.Rect, zenui.Colour, zenui.Colour) {
}
func (p *paintRenderer) DrawLine(int, int, int, int, int, zenui.Colour) {}
func (p *paintRenderer) DrawText(s string, _, _, _ int, _ zenui.Colour) {
	p.texts = append(p.texts, s)
}
func (p *paintRenderer) TextWidth(s string, scale int) int { return 6 * scale * len([]rune(s)) }
func (p *paintRenderer) LineHeight(scale int) int          { return 8 * scale }
func (p *paintRenderer) Clip(zenui.Rect)                   {}
func (p *paintRenderer) ClipEnd()                          {}

func (p *paintRenderer) drew(text string) bool {
	for _, s := range p.texts {
		if strings.Contains(s, text) {
			return true
		}
	}
	return false
}

func TestNextSetupIsDrawnInTheSelectedTheme(t *testing.T) {
	for _, name := range []zenui.ThemeName{zenui.ThemeDark, zenui.ThemeLight, zenui.ThemeSpectrum} {
		theme := zenui.LoadTheme(name)
		s := newNextSetup("/somewhere/zenzx/next", "")
		r := &paintRenderer{}
		s.draw(r, 1600, 1200, theme)
		if len(r.fills) < 2 {
			t.Fatalf("%s: nothing was drawn", name)
		}
		// As the Help and About windows: the theme's backdrop over the whole screen, let
		// through a little more, and the theme's panel, slightly see-through.
		if got, want := r.fills[0], (paintFill{zenui.Rect{W: 1600, H: 1200}, zenui.ScaleAlpha(theme.Backdrop, zenui.ModalBackdropDarkening)}); got != want {
			t.Errorf("%s: backdrop %+v, want %+v", name, got, want)
		}
		if got, want := r.fills[1].colour, zenui.ScaleAlpha(theme.Panel, zenui.ModalPanelOpacity); got != want {
			t.Errorf("%s: panel colour %+v, want %+v", name, got, want)
		}
		if r.fills[1].rect != s.panel || s.panel.W >= 1600 || s.panel.H >= 1200 {
			t.Errorf("%s: panel %+v", name, s.panel)
		}
		for _, want := range []string{"FIRST START", "Why this is needed", "/somewhere/zenzx/next", "Download", "Not now", nextcard.DefaultDistributionURL} {
			if !r.drew(want) {
				t.Errorf("%s: %q is not on the window", name, want)
			}
		}
	}
}

func TestNextSetupScrollsWhenTheWindowIsSmall(t *testing.T) {
	theme := zenui.LoadTheme(zenui.ThemeDark)
	s := newNextSetup("/somewhere/zenzx/next", "")
	r := &paintRenderer{}

	s.draw(r, 1600, 1200, theme)
	if s.visible < len(s.lines) || s.thumb.W != 0 {
		t.Fatalf("on a large screen %d of %d lines show and there is a scrollbar: %v", s.visible, len(s.lines), s.thumb.W != 0)
	}

	// A window too low for the text: the panel stays inside it, the fields and both
	// buttons stay on it, and the body scrolls.
	const w, h = 640, 200
	*r = paintRenderer{}
	s.draw(r, w, h, theme)
	if s.panel.Y < 0 || s.panel.Y+s.panel.H > h || s.panel.X < 0 || s.panel.X+s.panel.W > w {
		t.Fatalf("the panel %+v does not fit a %dx%d window", s.panel, w, h)
	}
	if s.visible >= len(s.lines) || s.thumb.W == 0 {
		t.Fatalf("in a low window all %d lines show (%d) or there is no scrollbar", len(s.lines), s.visible)
	}
	inside := func(rec zenui.Rect) bool {
		return rec.W > 0 && rec.Y >= s.panel.Y && rec.Y+rec.H <= s.panel.Y+s.panel.H && rec.X >= s.panel.X && rec.X+rec.W <= s.panel.X+s.panel.W
	}
	for i, rec := range append(s.fields[:], s.btnRects...) {
		if !inside(rec) {
			t.Errorf("field or button %d at %+v is not inside the panel %+v", i, rec, s.panel)
		}
	}
	if !r.drew("Why this is needed") || r.drew("The two addresses can be changed") {
		t.Fatal("the top of the text should show and the end should not, before scrolling")
	}
	s.update(zenui.Input{WheelY: -100})
	*r = paintRenderer{}
	s.draw(r, w, h, theme)
	if s.scroll != len(s.lines)-s.visible || !r.drew("The two addresses can be changed") || r.drew("Why this is needed") {
		t.Fatalf("after scrolling to the end: scroll %d of %d lines, %d visible", s.scroll, len(s.lines), s.visible)
	}
	// The buttons work where they are drawn.
	dl := s.btnRects[1]
	ran := make(chan struct{}, 1)
	s.run = func(context.Context, nextcard.Options, nextcard.Progress) error { ran <- struct{}{}; return nil }
	s.update(zenui.Input{MouseX: dl.X + 2, MouseY: dl.Y + 2, MousePressed: true})
	if s.phase != nextSetupRunning {
		t.Fatal("a click on Download did not start the download")
	}
	<-ran
}

func TestNextSetupAddressFieldsTakeTyping(t *testing.T) {
	s := newNextSetup(t.TempDir(), "")
	s.paste = func() (string, bool) { return "", false }
	s.distURL = "https://a/b"
	s.update(zenui.Input{Chars: []rune("c")})
	s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyBackspace}})
	s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyBackspace}})
	if s.distURL != "https://a/" {
		t.Fatalf("after typing and two backspaces the distribution address is %q", s.distURL)
	}
	boot := s.bootURL
	s.update(zenui.Input{Keys: []zenui.Key{zenui.KeyTab}})
	s.paste = func() (string, bool) { return " /pasted\n", true }
	s.update(zenui.Input{})
	if s.bootURL != boot+"/pasted" || s.distURL != "https://a/" {
		t.Fatalf("after Tab and a paste: boot ROM %q, distribution %q", s.bootURL, s.distURL)
	}
}

func TestNextSetupSaysWhyFoundFilesAreNotUsed(t *testing.T) {
	s := newNextSetup("/somewhere", "thirdparty: next-card.img has 32758 clusters; the firmware wants at least 65525")
	r := &paintRenderer{}
	s.draw(r, 1600, 1200, zenui.LoadTheme(zenui.ThemeLight))
	if !r.drew("Files that were found and not used") || !r.drew("32758 clusters") {
		t.Fatal("the reason the found files are not used is not on the window")
	}
}
