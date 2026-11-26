//go:build !headless

package zenzx

import (
	"strings"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The dock shown for a machine made for a snapshot: its chevron, and how long the dock stays. What is
// drawn is the GL guard's (dockreveal_gl_test.go); the rest does not need a window.

// testTile is the picture the chevron bounces on.
var testTile = rl.Rectangle{X: 300, Y: 412, Width: 120, Height: 90}

// The chevron is as wide as the picture and over it, bouncing on it (a big hop and a small one,
// landing on the ground each time and never going into it), and seen from nothing as it begins to
// nothing as its time ends.
func TestTheChevronIsAsWideAsThePictureAndBouncesOnIt(t *testing.T) {
	ground := testTile.Y
	landed := chevronAt(testTile, ground, chevronPeriod, -1) // a whole round on: down on the ground
	if landed.Left != testTile.X {
		t.Errorf("the chevron starts at x %v, want the picture's, %v", landed.Left, testTile.X)
	}
	if landed.Bottom != ground {
		t.Errorf("landed, the chevron's bottom is at %v, want the ground, %v", landed.Bottom, ground)
	}
	// as wide as the picture, whatever the picture's width, and as high as its sprite is to its width
	for _, w := range []float32{40, 80, 120, 160, 240, 320} {
		pic := rl.Rectangle{X: 100, Y: 300, Width: w, Height: w * 3 / 4}
		p := chevronAt(pic, pic.Y, chevronPeriod, -1)
		if got := p.Scale * float32(len(chevronSprite[0])); got < w-0.001 || got > w+0.001 {
			t.Errorf("a picture %v wide: the chevron is %v wide", w, got)
		}
		if got, want := p.Bottom-p.Top(), w*float32(len(chevronSprite))/float32(len(chevronSprite[0])); got < want-0.001 || got > want+0.001 {
			t.Errorf("a picture %v wide: the chevron is %v high, want %v", w, got, want)
		}
	}
	var big, small, lowest float32 // the heights of the two hops, and the chevron's lowest point
	for since := time.Duration(0); since < 3*chevronPeriod; since += 3 * time.Millisecond {
		p := chevronAt(testTile, ground, since, -1)
		if p.Bottom > lowest {
			lowest = p.Bottom
		}
		h := ground - p.Bottom
		if h < -0.001 {
			t.Fatalf("at %v the chevron is %v px into the ground", since, -h)
		}
		if since%chevronPeriod < time.Duration(chevronBigPart*float64(chevronPeriod)) {
			if h > big {
				big = h
			}
		} else if h > small {
			small = h
		}
	}
	if lowest > ground+0.001 || lowest < ground-0.5 {
		t.Errorf("the chevron comes down to %v, want it to land on the ground, %v", lowest, ground)
	}
	if big < chevronHop-1 || big > chevronHop+0.001 {
		t.Errorf("the big hop is %v px, want %d", big, chevronHop)
	}
	if want := float32(chevronHop * chevronSmallHop); small < want-1 || small > want+0.001 {
		t.Errorf("the small hop is %v px, want %v", small, want)
	}
	if a := chevronAt(testTile, ground, 0, -1).Alpha; a != 0 {
		t.Errorf("at its first moment the chevron is seen %v, want 0", a)
	}
	if a := chevronAt(testTile, ground, chevronFadeIn, -1).Alpha; a != 1 {
		t.Errorf("fully in, the chevron is seen %v, want 1", a)
	}
	if a := chevronAt(testTile, ground, 5*time.Second, chevronFadeOut/2).Alpha; a < 0.49 || a > 0.51 {
		t.Errorf("half way out, the chevron is seen %v, want a half", a)
	}
	if a := chevronAt(testTile, ground, 5*time.Second, 0).Alpha; a != 0 {
		t.Errorf("as its time ends, the chevron is seen %v, want 0", a)
	}
	if a := chevronAt(testTile, ground, 900*time.Millisecond, -time.Millisecond).Alpha; a != 1 {
		t.Errorf("before the dock is in, the chevron is seen %v, want 1", a)
	}
	// a picture at the very top of the window: the chevron does not go off the top of it
	top := rl.Rectangle{X: 300, Y: 4, Width: 120, Height: 90}
	for since := time.Duration(0); since < chevronPeriod; since += 10 * time.Millisecond {
		if edge := chevronAt(top, top.Y, since, -1).Top(); edge < 2-0.001 {
			t.Fatalf("at %v the chevron's top is at %v, off the window", since, edge)
		}
	}
}

// The sprite is a picture of black and white and nothing: the same on the left as on the right,
// white with a black outline two pixels thick all the way round it (no white pixel has nothing within
// two pixels of it), and standing on its last row.
func TestTheChevronSpriteIsAWhiteVWithABlackOutline(t *testing.T) {
	w := len(chevronSprite[0])
	if len(chevronSprite) < 8 {
		t.Fatalf("the sprite is %d rows", len(chevronSprite))
	}
	at := func(r, c int) byte {
		if r < 0 || r >= len(chevronSprite) || c < 0 || c >= w {
			return '.'
		}
		return chevronSprite[r][c]
	}
	whites := 0
	for r, row := range chevronSprite {
		if len(row) != w {
			t.Fatalf("row %d is %d pixels across, want %d", r, len(row), w)
		}
		for c := 0; c < w; c++ {
			v := row[c]
			if v != '.' && v != '#' && v != 'o' {
				t.Errorf("row %d pixel %d is %q", r, c, v)
			}
			if v != at(r, w-1-c) {
				t.Errorf("row %d: pixel %d is %q and its mirror %q", r, c, v, at(r, w-1-c))
			}
			if v != 'o' {
				continue
			}
			whites++
			for dr := -2; dr <= 2; dr++ {
				for dc := -2; dc <= 2; dc++ {
					if at(r+dr, c+dc) == '.' {
						t.Errorf("the white pixel at row %d, %d has nothing round it at %d, %d", r, c, r+dr, c+dc)
					}
				}
			}
		}
	}
	if whites < 60 {
		t.Errorf("only %d white pixels", whites)
	}
	if last := chevronSprite[len(chevronSprite)-1]; !strings.Contains(last, "#") {
		t.Errorf("the last row %q has nothing in it, so the chevron would float above the ground", last)
	}
	if first := chevronSprite[0]; !strings.Contains(first, "#") {
		t.Errorf("the first row %q has nothing in it", first)
	}
}

// The chevron lands on the picture's top when the picture is in the window, and on the window's
// bottom edge when it is not (the dock is not in yet, or has gone).
func TestTheChevronLandsOnThePictureOrOnTheWindowsEdge(t *testing.T) {
	const h = 480
	for _, c := range []struct {
		name string
		top  float32
		want float32
	}{
		{"the picture is in the window", 412, 412},
		{"the dock is gone, its pictures below the window", h + 12, h - chevronEdgeGap},
		{"the dock is coming in, the picture is at the edge", h, h - chevronEdgeGap},
		{"the picture is just in", h - chevronEdgeGap - 1, h - chevronEdgeGap - 1},
	} {
		pic := rl.Rectangle{X: 300, Y: c.top, Width: 120, Height: 90}
		if got := chevronGround(pic, h); got != c.want {
			t.Errorf("%s: it lands at %v, want %v", c.name, got, c.want)
		}
	}
}

// revealed is a deck with the dock being shown for its first machine, as it is once openSnapshot has
// made one, without the dock's own sliding (which makes the pictures, textures).
func revealed(t *testing.T) (*machineDeck, time.Time) {
	t.Helper()
	d := testDeck(t, 2)
	now := time.Now()
	d.state = barSlidingIn
	d.reveal = dockReveal{on: true, machine: d.machines[1], began: now}
	return d, now
}

// The dock stays three seconds after it is in, and not before it is in or after them, whatever the
// pointer does.
func TestTheDockStaysThreeSecondsAfterItIsIn(t *testing.T) {
	d, t0 := revealed(t)
	if d.revealHolds(t0) {
		t.Error("the dock holds while it is still coming in: its time has not begun")
	}
	d.revealArrived(t0)
	d.revealArrived(t0.Add(500 * time.Millisecond)) // arriving is once
	if !d.revealHolds(t0.Add(dockRevealLinger - time.Millisecond)) {
		t.Error("the dock does not hold within its time")
	}
	if d.revealHolds(t0.Add(dockRevealLinger)) {
		t.Error("the dock holds after its time, or its time was begun again")
	}
	if dockRevealLinger != 3*time.Second {
		t.Errorf("the dock stays %v, want three seconds", dockRevealLinger)
	}
	var none machineDeck
	if none.revealHolds(t0) {
		t.Error("a deck that revealed nothing holds the dock")
	}
}

// The reveal ends when it is over, and not before: the pointer on another picture, or off the dock,
// does not end it; the dock's time being up does not, nor does the dock leaving or being gone. On the
// picture it points at, the chevron's own time up, or a picture flying does.
func TestTheRevealEndsWhenItIsOver(t *testing.T) {
	cases := []struct {
		name  string
		setup func(d *machineDeck, t0 time.Time) (hover int, now time.Time)
		over  bool
	}{
		{"within the dock's time, pointer elsewhere", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barShown
			return 0, t0.Add(300 * time.Millisecond)
		}, false},
		{"within the dock's time, pointer off the dock", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barShown
			return -1, t0.Add(300 * time.Millisecond)
		}, false},
		{"the pointer is on the picture", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barShown
			return 1, t0.Add(300 * time.Millisecond)
		}, true},
		{"the dock's time is up, the chevron's is not", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barShown
			return -1, t0.Add(dockRevealLinger + time.Second)
		}, false},
		{"the chevron's time is up", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barShown
			return -1, t0.Add(dockRevealLinger + chevronAfterDock)
		}, true},
		{"the dock is leaving", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barSlidingOut
			return -1, t0.Add(300 * time.Millisecond)
		}, false},
		{"the dock is gone", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barHidden
			return -1, t0.Add(dockRevealLinger + time.Second)
		}, false},
		{"the dock is gone and the chevron's time is up", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barHidden
			return -1, t0.Add(dockRevealLinger + chevronAfterDock + time.Millisecond)
		}, true},
		{"the dock is gone and the pointer is on the picture", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barHidden
			return 1, t0.Add(dockRevealLinger + time.Second)
		}, true},
		{"a picture is flying to the screen", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barShown
			d.flying = true
			return -1, t0.Add(300 * time.Millisecond)
		}, true},
		{"the machine is gone", func(d *machineDeck, t0 time.Time) (int, time.Time) {
			d.state = barShown
			d.machines = d.machines[:1]
			return -1, t0.Add(300 * time.Millisecond)
		}, true},
	}
	for _, c := range cases {
		d, t0 := revealed(t)
		d.revealArrived(t0)
		hover, now := c.setup(d, t0)
		d.revealTick(hover, now)
		if got := !d.reveal.on; got != c.over {
			t.Errorf("%s: the reveal is over %v, want %v", c.name, got, c.over)
		}
	}
}

// A dock that went before it was in (a key, a click) has had its time: the chevron's begins then, and
// is not for ever.
func TestADockThatWentBeforeItWasInStartsTheChevronsTime(t *testing.T) {
	d, t0 := revealed(t)
	d.state = barHidden
	d.revealTick(-1, t0)
	if !d.reveal.on || !d.reveal.holdUntil.Equal(t0) {
		t.Fatalf("the reveal is on %v with the dock's time up at %v, want it on and up at %v", d.reveal.on, d.reveal.holdUntil, t0)
	}
	d.revealTick(-1, t0.Add(chevronAfterDock-time.Millisecond))
	if !d.reveal.on {
		t.Error("the chevron went before its time was up")
	}
	d.revealTick(-1, t0.Add(chevronAfterDock))
	if d.reveal.on {
		t.Error("the chevron is still there when its time is up")
	}
	if until := (&machineDeck{}).revealUntil(); !until.IsZero() {
		t.Errorf("a deck that revealed nothing has the chevron till %v", until)
	}
	d2, _ := revealed(t)
	if until := d2.revealUntil(); !until.IsZero() {
		t.Errorf("the dock is not in yet and the chevron has its end at %v", until)
	}
}

// The reveal follows its machine, not its place: a machine before it is gone and it is the same.
func TestTheRevealFollowsItsMachine(t *testing.T) {
	d, _ := revealed(t)
	if i := d.revealIndex(); i != 1 {
		t.Fatalf("the reveal is of machine %d, want 1", i)
	}
	d.machines = d.machines[1:]
	if i := d.revealIndex(); i != 0 {
		t.Errorf("after the machine before it went, the reveal is of machine %d, want 0", i)
	}
	if (&machineDeck{}).revealIndex() != -1 {
		t.Error("a deck with no reveal has a place for it")
	}
}

// With the dock off in Preferences a machine made for a snapshot shows no dock, and no chevron.
func TestWithTheDockOffNoDockIsShownForANewMachine(t *testing.T) {
	d := testDeck(t, 2)
	d.dockOn = func() bool { return false }
	d.revealNew(d.machines[1])
	if d.state != barHidden || d.reveal.on {
		t.Errorf("dock off: the dock is %v, reveal %v", d.state, d.reveal.on)
	}
}
