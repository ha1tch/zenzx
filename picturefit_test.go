package zenzx

import "testing"

// T-75: the arithmetic of putting the picture in the middle of a window that is not the size the
// display asked for, at HiDPI factors 1 and 2.

func TestWindowDrawSizeAtHiDPIFactors(t *testing.T) {
	cases := []struct {
		name         string
		rw, rh       int
		dpi          float32
		wantW, wantH int
	}{
		{"ordinary display", 960, 720, 1, 960, 720},
		{"retina: twice the pixels", 1920, 1440, 2, 960, 720},
		{"retina, an odd pixel count", 1921, 1441, 2, 961, 721},
		{"a factor of 1.5", 1440, 1080, 1.5, 960, 720},
		{"a factor below 1 is no factor", 960, 720, 0, 960, 720},
	}
	for _, c := range cases {
		w, h := windowDrawSize(c.rw, c.rh, c.dpi, c.dpi)
		if w != c.wantW || h != c.wantH {
			t.Errorf("%s: %dx%d at %v is %dx%d, want %dx%d", c.name, c.rw, c.rh, c.dpi, w, h, c.wantW, c.wantH)
		}
	}
}

func TestFitZoom(t *testing.T) {
	const nw, nh = 320, 256 // a classic machine's picture with its border
	cases := []struct {
		name           string
		winW, winH     int
		top, min, want int
	}{
		{"exactly 2x", 640, 512, 0, 1, 2},
		{"exactly 3x under a 24 bar", 960, 768 + 24, 24, 1, 3},
		{"a window a little short of 3x", 960, 768 + 23, 24, 1, 2},
		{"wide and short: the height decides", 2000, 520, 0, 1, 2},
		{"narrow and tall: the width decides", 700, 2000, 0, 1, 2},
		{"more than the most", 5000, 5000, 0, 1, 5},
		{"smaller than 1x: the least", 200, 150, 0, 1, 1},
		{"the least is 2 for a mode that needs it", 400, 300, 0, 2, 2},
	}
	for _, c := range cases {
		if got := fitZoom(c.winW, c.winH, nw, nh, c.top, c.min, MaxMultiplier); got != c.want {
			t.Errorf("%s: %dx%d under %d is %dx, want %dx", c.name, c.winW, c.winH, c.top, got, c.want)
		}
	}
}

// The same window, as a retina display reports it and as an ordinary one does, puts the picture
// in the same place: the arithmetic is in drawing units.
func TestCentrePictureIsTheSameAtHiDPIFactors(t *testing.T) {
	for _, dpi := range []float32{1, 2} {
		winW, winH := windowDrawSize(int(1000*dpi), int(700*dpi), dpi, dpi)
		r := centrePicture(winW, winH, 24, 640, 512) // 2x, in a window 1000 by 700 under a 24 bar
		if r.Scale != 1 || r.W != 640 || r.H != 512 {
			t.Errorf("dpi %v: the picture is scaled: %+v", dpi, r)
		}
		// Left and right margins 180 each; below the bar there are 676, so 82 above and below.
		if r.X != 180 || r.Y != 24+82 {
			t.Errorf("dpi %v: the picture is at (%v, %v), want (180, 106)", dpi, r.X, r.Y)
		}
	}
}

// Whatever the window, the margins are the same on both sides, and the picture is whole in it.
func TestCentrePictureMarginsAreEqualAndNothingIsCropped(t *testing.T) {
	const top, picW, picH = 24, 640, 512
	for _, win := range [][2]int{{640, 536}, {700, 560}, {1000, 700}, {641, 537}, {1280, 1024}, {640, 400}, {500, 536}, {320, 280}, {200, 150}, {100, 400}} {
		r := centrePicture(win[0], win[1], top, picW, picH)
		left, right := r.X, float32(win[0])-(r.X+r.W)
		above, below := r.Y-top, float32(win[1])-(r.Y+r.H)
		if left < 0 || right < 0 || above < 0 || below < 0 {
			t.Errorf("window %v: the picture is cropped: margins left %v right %v above %v below %v", win, left, right, above, below)
		}
		// Whole units when the picture is not scaled leave at most one unit between the two
		// sides of an odd margin; a scaled one is exact.
		if d := left - right; d < -1 || d > 1 {
			t.Errorf("window %v: left margin %v, right %v", win, left, right)
		}
		if d := above - below; d < -1 || d > 1 {
			t.Errorf("window %v: margin above %v, below %v", win, above, below)
		}
		if r.Scale > 1 {
			t.Errorf("window %v: the picture is scaled up to %v", win, r.Scale)
		}
	}
}

// A picture that is larger than the window is scaled down by the same factor each way, so its
// proportions are kept.
func TestCentrePictureScalesDownKeepingProportions(t *testing.T) {
	r := centrePicture(320, 280, 24, 640, 512) // half as wide, and 256 of the 512 high
	if r.Scale != 0.5 {
		t.Fatalf("scale %v, want 0.5 (the width decides)", r.Scale)
	}
	if r.W != 320 || r.H != 256 {
		t.Errorf("size %vx%v, want 320x256", r.W, r.H)
	}
	if r.X != 0 || r.Y != 24+0 {
		t.Errorf("at (%v, %v), want (0, 24): the 256 left under the bar are 256, the picture's height", r.X, r.Y)
	}
}

func TestSizeWatchSettlesOnlyOnASteadySize(t *testing.T) {
	var s sizeWatch
	if s.settled(700, 560) || s.settled(700, 560) {
		t.Fatal("settled before the window had kept its size for pictureSettleFrames frames")
	}
	if !s.settled(700, 560) {
		t.Fatal("did not settle on the third frame at one size")
	}
	if s.settled(701, 560) {
		t.Fatal("a new size settled at once")
	}
	s.reset()
	if s.settled(701, 560) {
		t.Fatal("a size seen after a reset settled at once")
	}
}
