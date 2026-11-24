//go:build !headless

package zenzx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"
)

func pngOf(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.SetRGBA(i%w, i/w, c)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// With no sequence of its own, a folder's pictures are each shown a fifth of a second, in turn,
// starting with the first and going round.
func TestWaitFigureWithNoSequenceShowsEachPictureAFifthOfASecond(t *testing.T) {
	for _, c := range []struct {
		at   float64
		n    int
		want int
	}{{-1, 10, 0}, {0, 10, 0}, {0.19, 10, 0}, {0.2, 10, 1}, {0.41, 10, 2}, {1.99, 10, 9}, {2.0, 10, 0}, {2.2, 10, 1}, {7.25, 10, 6}, {0.3, 2, 1}, {0.5, 1, 0}} {
		if got := defaultWaitScript(c.n).at(c.at); got != c.want {
			t.Errorf("at %vs of %d pictures the picture is %d, want %d", c.at, c.n, got, c.want)
		}
	}
	if got := (waitScript{}).at(3); got != 0 {
		t.Errorf("a script with nothing in it shows picture %d", got)
	}
}

// sequence.txt: pictures and seconds, comments, an introduction played once before "loop", and
// everything going round when there is no such line.
func TestWaitScriptReadsItsFile(t *testing.T) {
	s, ok := parseWaitScript("# a comment\n0 0.5  # then\n\n1 1\nloop\n2 2\n0 0.25\n", 3)
	if !ok {
		t.Fatal("a right sequence was refused")
	}
	for _, c := range []struct {
		at   float64
		want int
	}{{0, 0}, {0.49, 0}, {0.5, 1}, {1.49, 1}, {1.5, 2}, {3.49, 2}, {3.5, 0}, {3.74, 0}, {3.75, 2}, {5.74, 2}, {5.8, 0}, {5.99, 0}, {6.0, 2}, {7.99, 2}, {8.0, 0}, {-3, 0}} {
		if got := s.at(c.at); got != c.want {
			t.Errorf("at %vs the picture is %d, want %d", c.at, got, c.want)
		}
	}
	all, ok := parseWaitScript("1 1\n0 1\n", 2)
	if !ok || len(all.intro) != 0 || all.at(0.5) != 1 || all.at(1.5) != 0 || all.at(2.5) != 1 {
		t.Errorf("with no loop line everything goes round: %+v, %v", all, ok)
	}
}

func TestWaitScriptRefusesWhatIsNotRight(t *testing.T) {
	for name, text := range map[string]string{
		"a picture that is not there": "0 1\n3 1\n",
		"a negative picture":          "-1 1\n",
		"no time":                     "0 0\n",
		"a negative time":             "0 -1\n",
		"a time that is no number":    "0 soon\n",
		"a picture that is no number": "first 1\n",
		"a line with three things":    "0 1 2\n",
		"a line with one thing":       "0\n",
		"two loop lines":              "0 1\nloop\n1 1\nloop\n",
		"a loop with nothing in it":   "0 1\nloop\n",
		"nothing at all":              "# only a comment\n",
		"a minute and more":           "0 61\n",
	} {
		if s, ok := parseWaitScript(text, 3); ok {
			t.Errorf("%s: taken as %+v", name, s)
		}
	}
	// A folder whose sequence is not right shows its pictures in turn instead.
	red := color.RGBA{255, 0, 0, 255}
	got := loadWaitImages(fstest.MapFS{
		"frame0.png": {Data: pngOf(t, 4, 4, red)}, "frame1.png": {Data: pngOf(t, 4, 4, red)},
		"sequence.txt": {Data: []byte("0 1\n7 1\n")},
	})
	if got.script.at(0.1) != 0 || got.script.at(0.3) != 1 {
		t.Errorf("a wrong sequence was used: %+v", got.script)
	}
	got = loadWaitImages(fstest.MapFS{
		"frame0.png": {Data: pngOf(t, 4, 4, red)}, "frame1.png": {Data: pngOf(t, 4, 4, red)},
		"sequence.txt": {Data: []byte("1 3\nloop\n0 1\n")},
	})
	if got.script.at(1) != 1 || got.script.at(3.5) != 0 {
		t.Errorf("a right sequence was not used: %+v", got.script)
	}
}

// What the figure this build ships does: it settles, rests a long while, and then, over and over,
// blinks, rests, taps its foot three times, rests a long while, looks to its right, rests a long
// while, blinks twice, and rests a long while.
func TestWaitFigureStaysQuietBlinksAndTaps(t *testing.T) {
	s := embeddedWaitImages().script
	if s.at(0) != 0 || s.at(0.3) != 1 || s.at(0.6) != 2 || s.at(6.4) != 2 || s.at(6.55) != 3 {
		t.Errorf("the figure settles as %d, %d, %d, rests until %d, and blinks as %d, want 0, 1, 2, 2, 3",
			s.at(0), s.at(0.3), s.at(0.6), s.at(6.4), s.at(6.55))
	}
	const start, step = 6.5, 0.01 // the loop begins when the settling and its rest are over
	loop := 0.0
	for _, st := range s.loop {
		loop += st.secs
	}
	type run struct {
		frame int
		secs  float64
	}
	var runs []run
	for t0 := 0.0; t0 < loop-step/2; t0 += step {
		f := s.at(start + t0 + step/2)
		if n := len(runs); n > 0 && runs[n-1].frame == f {
			runs[n-1].secs += step
		} else {
			runs = append(runs, run{f, step})
		}
	}
	var frames []int
	for _, r := range runs {
		frames = append(frames, r.frame)
	}
	// blink, rest, three taps, rest, look right (two pictures), rest, two blinks, rest.
	want := []int{3, 2, 5, 2, 5, 2, 5, 2, 0, 1, 2, 3, 2, 3, 2}
	if len(frames) != len(want) {
		t.Fatalf("one round is %v, want %v", frames, want)
	}
	for i := range want {
		if frames[i] != want[i] {
			t.Fatalf("one round is %v, want %v", frames, want)
		}
	}
	// Every gesture is followed by a long wait: after the taps and before the look, after the
	// look, and after the two blinks; and the blink has a while before the taps.
	for _, c := range []struct {
		i    int
		min  float64
		what string
	}{{1, 2.4, "after the blink"}, {7, 6.0, "after the taps"}, {10, 5.9, "after the look"}, {14, 5.9, "after the two blinks"}} {
		if runs[c.i].secs < c.min {
			t.Errorf("the rest %s is %.2fs, want at least %.1fs", c.what, runs[c.i].secs, c.min)
		}
	}
	for _, i := range []int{0, 11, 13} {
		if runs[i].secs > 0.3 {
			t.Errorf("a blink (run %d) lasts %.2fs", i, runs[i].secs)
		}
	}
	if runs[12].secs > 0.3 {
		t.Errorf("the eyes are open for %.2fs between the two blinks: want a moment", runs[12].secs)
	}
	// And it goes round, without the settling: a round later is the same.
	for t0 := 0.0; t0 < loop; t0 += 0.07 {
		if a, b := s.at(start+t0), s.at(start+t0+loop); a != b {
			t.Fatalf("at %.2fs of the round the picture is %d, and %d a round later", t0, a, b)
		}
	}
}

// A figure of one's own: every frame*.png of the folder, in the order of their names, whatever
// their sizes; and none at all if one is not a picture.
func TestWaitFigureCanBeReplaced(t *testing.T) {
	red, blue, green := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}, color.RGBA{0, 255, 0, 255}
	three := fstest.MapFS{
		"frame02.png": {Data: pngOf(t, 5, 7, green)},
		"frame00.png": {Data: pngOf(t, 7, 9, red)},
		"frame01.png": {Data: pngOf(t, 7, 11, blue)},
		"README.txt":  {Data: []byte("not a picture, and not a frame")},
	}
	got := loadWaitImages(three)
	if len(got.frames) != 3 {
		t.Fatalf("%d pictures, want 3", len(got.frames))
	}
	for i, want := range []struct {
		w, h int
		c    color.RGBA
	}{{7, 9, red}, {7, 11, blue}, {5, 7, green}} {
		if b := got.frames[i].Bounds(); b.Dx() != want.w || b.Dy() != want.h || got.frames[i].RGBAAt(2, 2) != want.c {
			t.Errorf("picture %d is %v, %v: want %dx%d, %v", i, b, got.frames[i].RGBAAt(2, 2), want.w, want.h, want.c)
		}
	}
	if one := loadWaitImages(fstest.MapFS{"frame0.png": {Data: pngOf(t, 4, 4, red)}}); len(one.frames) != 1 {
		t.Errorf("a single picture gives %d", len(one.frames))
	}
	for name, fsys := range map[string]fstest.MapFS{
		"a broken one": {"frame00.png": {Data: pngOf(t, 7, 9, red)}, "frame01.png": {Data: []byte("not a picture")}},
		"none":         {},
	} {
		if got := loadWaitImages(fsys); len(got.frames) != 0 {
			t.Errorf("with %s, %d pictures: want none", name, len(got.frames))
		}
	}
}

// The figure starts its sequence when it appears, and again when it comes back after a machine
// has been in the window; drawn every frame it goes on.
func TestWaitFigureStartsAgainWhenItComesBack(t *testing.T) {
	w := &waitTextures{script: embeddedWaitImages().script}
	// Drawn every frame (here every 50 ms), the figure goes on; the window has been open for an
	// hour when the last machine is stopped.
	start := 3600.0
	now, got := start, w.frameAt(start)
	if got != 0 {
		t.Errorf("when it appears the picture is %d, want the first", got)
	}
	advance := func(to float64) int {
		for now < to-1e-9 {
			now += 0.05
			got = w.frameAt(now)
		}
		return got
	}
	for _, c := range []struct {
		after float64
		want  int
	}{{0.3, 1}, {0.6, 2}, {3.0, 2}, {6.55, 3}, {7.0, 2}} {
		if got := advance(start + c.after); got != c.want {
			t.Errorf("%.2fs after it appeared the picture is %d, want %d", c.after, got, c.want)
		}
	}
	// A machine is started, and stopped a minute later: the figure begins again.
	back := now + 60
	if got := w.frameAt(back); got != 0 {
		t.Errorf("when it comes back the picture is %d, want the first", got)
	}
	now = back
	if got := advance(back + 0.3); got != 1 {
		t.Errorf("0.3s after it came back the picture is %d, want the second", got)
	}
}
