//go:build !headless

package zenzx

import (
	"image/color"
	"os"
	"runtime"
	"testing"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TestGLSnapshotMachineRevealsTheDock: a snapshot of another kind makes a machine, and the dock slides
// in for it with its picture growing into place and a chevron over it pointing down; the chevron is
// white with a black outline, made of big square pixels, as wide as the
// picture, and is gone when the reveal is. (That it bounces, fades, and ends when it should is tested without a window,
// dockreveal_gui_test.go.)
//
// A dormant guard, G-31 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	Xvfb :99 -screen 0 1280x1024x24 &
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLSnapshotMachineReveals -v .
func TestGLSnapshotMachineRevealsTheDock(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-31)")
	}
	runtime.LockOSThread()
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(640, 480, "ZenZX dock reveal")
	t.Cleanup(rl.CloseWindow)

	d, _ := snapshotDeck(t)
	d.dockOn = nil // the dock is allowed
	path := snapshotOf(t, "128k", "g.zxs")
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatal(err)
	}
	if d.state != barSlidingIn || !d.reveal.on || d.revealIndex() != 1 {
		t.Fatalf("after the snapshot: dock %v, reveal %v of machine %d, want the dock coming in for machine 1",
			d.state, d.reveal.on, d.revealIndex())
	}
	if m := d.machines[1].mag; m != dockRevealGrow {
		t.Errorf("the new picture starts at %v of its size, want %v", m, dockRevealGrow)
	}
	if d.byKey {
		t.Error("the dock is held open as a keyboard's is")
	}
	d.revealArrived(time.Now())
	if !d.revealHolds(time.Now()) {
		t.Error("the dock does not stay its second once it is in")
	}

	// A snapshot loaded again into the machine it made makes none, and no dock.
	d.state, d.progress, d.reveal = barHidden, 0, dockReveal{}
	if err := d.openSnapshot(path, "", false); err != nil {
		t.Fatal(err)
	}
	if d.state != barHidden || d.reveal.on || len(d.machines) != 2 {
		t.Errorf("the same file again: dock %v, reveal %v, %d machines, want nothing made and no dock", d.state, d.reveal.on, len(d.machines))
	}

	// The chevron, drawn: the sprite expanded to the width of the picture, over the dock and then at
	// the bottom of the window when the dock is gone. Every pixel of the sprite is looked at, at the
	// middle of the square it is expanded to: white where it is white, black where it is the outline,
	// and the ground where it is nothing.
	const w, h = 640, 480
	d.state, d.progress = barShown, 1
	// drawn is the moment the chevron is drawn at: a whole round on, so that it is down on the ground
	began := time.Now()
	drawn := began.Add(chevronPeriod)
	d.reveal = dockReveal{on: true, machine: d.machines[1], began: began, holdUntil: began.Add(time.Second)}
	l := d.layout(w, h, 8)
	ground := color.RGBA{0, 0, 215, 255}
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	frame := func() []color.RGBA {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Color{R: ground.R, G: ground.G, B: ground.B, A: 255})
		// a ground that is not flat: the outline is seen against it as against a game
		for x := 0; x < w; x += 16 {
			for y := 0; y < h; y += 16 {
				if (x/16+y/16)%2 == 0 {
					rl.DrawRectangle(int32(x), int32(y), 16, 16, rl.Color{R: 215, G: 0, B: 215, A: 255})
				}
			}
		}
		d.drawReveal(d.layout(w, h, 8), h, drawn)
		rl.DrawRenderBatchActive()
		img := rl.LoadImageFromScreen()
		rl.EndDrawing()
		cols := rl.LoadImageColors(img)
		rl.UnloadImage(img)
		out := make([]color.RGBA, len(cols))
		for i, c := range cols {
			out[i] = color.RGBA{c.R, c.G, c.B, c.A}
		}
		return out
	}
	// check measures a frame of the chevron landed on picture, standing on groundY
	check := func(name string, pic rl.Rectangle, groundY float32) {
		t.Helper()
		rest := chevronAt(pic, groundY, chevronPeriod, -1)
		on := frame()
		wrong := 0
		for r, row := range chevronSprite {
			for c := 0; c < len(row); c++ {
				x := int(rest.Left + (float32(c)+0.5)*rest.Scale)
				y := int(rest.Top() + (float32(r)+0.5)*rest.Scale)
				got := on[y*w+x]
				switch row[c] {
				case 'o':
					if got != white {
						wrong++
						t.Errorf("%s: sprite pixel %d,%d is %v, want white", name, r, c, got)
					}
				case '#':
					if got != black {
						wrong++
						t.Errorf("%s: sprite pixel %d,%d is %v, want black", name, r, c, got)
					}
				default:
					if got == white || got == black {
						wrong++
						t.Errorf("%s: sprite pixel %d,%d is %v, want the ground", name, r, c, got)
					}
				}
				if wrong > 6 {
					t.Fatalf("%s: too many wrong", name)
				}
			}
		}
		// as wide as the picture: white and black go from its left edge to its right
		minX, maxX := w, -1
		for y := int(rest.Top()); y < int(rest.Bottom); y++ {
			for x := 0; x < w; x++ {
				if on[y*w+x] == black {
					if x < minX {
						minX = x
					}
					if x > maxX {
						maxX = x
					}
				}
			}
		}
		if minX != int(pic.X+0.5) || maxX < int(pic.X+pic.Width)-1 || maxX > int(pic.X+pic.Width) {
			t.Errorf("%s: the outline goes from x %d to %d, want %v to %v, the picture's width", name, minX, maxX, pic.X, pic.X+pic.Width)
		}
		// and it stands on the ground
		if bottom := int(rest.Bottom); on[(bottom-1)*w+int(rest.Left+rest.Scale*float32(len(chevronSprite[0]))/2)] != black {
			t.Errorf("%s: nothing at the bottom of the chevron, %d", name, bottom)
		}
	}
	tile := grown(l.tiles[1], 1) // as wide as its place, not as it grows into it
	check("over the dock", tile, chevronGround(tile, h))

	// the dock gone: it is at the bottom of the window, where the dock comes from
	d.state, d.progress = barHidden, 0
	d.machines[1].mag = 1
	gone := grown(d.layout(w, h, 8).tiles[1], 1)
	if gone.Y <= float32(h) {
		t.Fatalf("a hidden dock has its pictures at %v, in the window", gone.Y)
	}
	check("the dock gone", gone, float32(h)-chevronEdgeGap)

	// and nothing when there is no reveal
	d.reveal = dockReveal{}
	rest := chevronAt(gone, float32(h)-chevronEdgeGap, chevronPeriod, -1)
	px := int(rest.Left + rest.Scale*16)
	if got := frame()[int(rest.Top()+rest.Scale*4)*w+px]; got == white || got == black {
		t.Errorf("with no reveal the pixel is %v, want the ground", got)
	}
}
