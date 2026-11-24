//go:build !headless

package zenzx

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// testDeck is a deck of n 48K machines with no sound device; the first has
// the display.
func testDeck(t *testing.T, n int) *machineDeck {
	t.Helper()
	mk := func(model string) *ZenZX {
		zx := NewZenZX(AudioBackendOto)
		switchModelLive(zx, model, "./rom")
		return zx
	}
	first := mk("48k")
	d := newMachineDeck(first, "48k", first.window(), nil, mk)
	for i := 1; i < n; i++ {
		d.add("48k")
	}
	return d
}

func TestDeckFlipHandsOverTheDisplay(t *testing.T) {
	d := testDeck(t, 2)
	a, b := d.machines[0].zx, d.machines[1].zx
	dm := d.dm
	dm.screen.multiplier = 3

	d.flip(1)
	if d.Front() != b {
		t.Fatal("after flip(1) the front machine is not the second")
	}
	if b.display != Display(dm) {
		t.Error("the second machine was not given the window's display")
	}
	if _, ok := a.display.(*nullDisplay); !ok {
		t.Errorf("the first machine still has a display of type %T", a.display)
	}
	if dm.screen != b.screen {
		t.Error("the display still draws the first machine's screen")
	}
	if b.screen.multiplier != 3 {
		t.Errorf("zoom = %d after the flip, want the 3 the window had", b.screen.multiplier)
	}

	d.flip(0)
	if d.Front() != a || a.display != Display(dm) || dm.screen != a.screen {
		t.Error("flipping back did not give the first machine the display again")
	}
}

// With -focus-follow, the deck brings to the front the machine whose player
// beacon is set, but only on a clean hand-off (exactly one machine asserts
// it). While none or two do, it waits.
func TestFocusFollowsThePlayerBeacon(t *testing.T) {
	d := testDeck(t, 3)
	d.focusFollow = true
	a, b, c := d.machines[0].zx, d.machines[1].zx, d.machines[2].zx

	// Nobody holds the player yet: the front does not move.
	d.followPlayerFocus()
	if d.front != 0 {
		t.Fatalf("front moved to %d with no beacon set", d.front)
	}

	// The player is on machine 2: focus follows it there.
	c.memory.Write(playerBeaconAddr, 1)
	d.followPlayerFocus()
	if d.front != 2 {
		t.Fatalf("front = %d, want 2 (the machine holding the player)", d.front)
	}

	// Mid-crossing: both machine 2 (old) and machine 1 (new) assert it for a
	// frame. The deck waits rather than flip to an ambiguous holder.
	b.memory.Write(playerBeaconAddr, 1)
	d.followPlayerFocus()
	if d.front != 2 {
		t.Fatalf("front = %d during a two-beacon moment, want it held at 2", d.front)
	}

	// The hand-off completes: only machine 1 holds it now.
	c.memory.Write(playerBeaconAddr, 0)
	d.followPlayerFocus()
	if d.front != 1 {
		t.Fatalf("front = %d after the hand-off, want 1", d.front)
	}

	// With focus-follow off, a beacon change is ignored.
	d.focusFollow = false
	a.memory.Write(playerBeaconAddr, 1)
	b.memory.Write(playerBeaconAddr, 0)
	d.followPlayerFocus()
	if d.front != 1 {
		t.Fatalf("front = %d with focus-follow off, want it unchanged at 1", d.front)
	}
}

func TestDeckMachinesBehindKeepRunning(t *testing.T) {
	d := testDeck(t, 2)
	behind := d.machines[1].zx
	before := behind.cpu.PC
	ran := false
	for i := 0; i < 20; i++ {
		d.Front().RunFrame()
		d.runBehind(i, 1500)
		if behind.cpu.PC != before {
			ran = true
		}
	}
	if !ran {
		t.Error("the machine behind the front one did not run")
	}
}

func TestDeckRemove(t *testing.T) {
	d := testDeck(t, 1)
	d.remove(0)
	if len(d.machines) != 1 {
		t.Fatal("the last machine was removed")
	}

	d = testDeck(t, 3)
	a, b, c := d.machines[0].zx, d.machines[1].zx, d.machines[2].zx
	d.remove(0) // the front one: its neighbour comes to the front first
	if len(d.machines) != 2 || d.Front() != b {
		t.Fatalf("after removing the front machine: %d machines, front is the second: %v", len(d.machines), d.Front() == b)
	}
	if b.display != Display(d.dm) {
		t.Error("the new front machine has no display")
	}
	d.remove(1)
	if len(d.machines) != 1 || d.Front() != b {
		t.Error("removing a machine behind the front one disturbed the front")
	}
	_, _ = a, c
}

func TestDeckNewMachineBesideANextIsNotANext(t *testing.T) {
	d := testDeck(t, 1)
	d.setFrontModel("next")
	if got := d.modelForNew(); got != "48k" {
		t.Errorf("a new machine beside a Next would be a %q: two Nexts would share one SD card", got)
	}
	d.setFrontModel("plus3")
	if got := d.modelForNew(); got != "plus3" {
		t.Errorf("a new machine beside a +3 would be a %q, want a +3", got)
	}
}

func TestDockLayoutStaysInTheWindow(t *testing.T) {
	for _, n := range []int{1, 2, 5, 12} {
		for _, size := range [][2]int{{640, 480}, {960, 768}, {352, 296}} {
			d := testDeck(t, 1)
			for i := 1; i < n; i++ {
				d.machines = append(d.machines, &deckMachine{zx: d.machines[0].zx, model: "48k", mag: 1})
			}
			d.progress = 1
			l := d.layout(size[0], size[1], 8)
			if len(l.tiles) != n+1 {
				t.Fatalf("%d machines: %d tiles, want one each and the New tile", n, len(l.tiles))
			}
			if int(l.panel.Y+l.panel.Height) > size[1] || l.panel.Y < 0 {
				t.Errorf("%d machines in %v: the dock is not inside the window vertically: %+v", n, size, l.panel)
			}
			for i, r := range l.tiles {
				if r.X < l.panel.X || r.X+r.Width > l.panel.X+l.panel.Width+0.5 {
					t.Errorf("%d machines in %v: tile %d is outside the dock", n, size, i)
				}
				if i > 0 && r.X < l.tiles[i-1].X+l.tiles[i-1].Width {
					t.Errorf("%d machines in %v: tiles %d and %d overlap at rest", n, size, i-1, i)
				}
			}
			d.progress = 0
			if l := d.layout(size[0], size[1], 8); int(l.panel.Y) != size[1] {
				t.Errorf("hidden, the dock's top is at %v, want the window's bottom edge %d", l.panel.Y, size[1])
			}
		}
	}
}

// The New button has its states, and the pointer is a hand over it only while it can be used.
func TestDockNewButtonStatesAndCursor(t *testing.T) {
	d := testDeck(t, 2)
	add := len(d.machines)
	d.state = barShown
	if got := d.addState(); got != addIdle {
		t.Errorf("at rest: state %v, want idle", got)
	}
	d.addHover = true
	if got := d.addState(); got != addHover {
		t.Errorf("under the pointer: state %v, want hover", got)
	}
	d.addDown = true
	if got := d.addState(); got != addPressed {
		t.Errorf("held: state %v, want pressed", got)
	}
	d.addHover = false
	if got := d.addState(); got != addIdle {
		t.Errorf("held but the pointer has moved off: state %v, want idle (the press can be taken back)", got)
	}
	d.addDown = false

	if !d.addCursorWanted(add, false) {
		t.Error("pointer on New: want a hand")
	}
	if d.addCursorWanted(add-1, false) || d.addCursorWanted(-1, false) {
		t.Error("pointer on a picture or off the dock: want the arrow")
	}
	if d.addCursorWanted(add, true) {
		t.Error("something else has the pointer: want the arrow")
	}
	d.state = barSlidingOut
	if d.addCursorWanted(add, false) {
		t.Error("dock on its way out: want the arrow")
	}
	d.state = barShown
	d.pressOn = 0
	if d.addCursorWanted(add, false) {
		t.Error("a picture is being dragged: want the arrow")
	}
	d.pressOn = -1
	d.multiOn = func() bool { return false }
	if d.addCursorWanted(add, false) || d.addState() != addOff {
		t.Error("adding machines off: New is greyed and the pointer stays the arrow")
	}
}

// New grows as the pictures do, from 1, and a zero value counts as 1.
func TestDockNewButtonMagnifyDefaultsToOne(t *testing.T) {
	d := testDeck(t, 1)
	if got := d.addMagnify(); got != 1 {
		t.Errorf("addMagnify = %v before the dock has run, want 1", got)
	}
	d.addMag = 1.3
	if got := d.addMagnify(); got != 1.3 {
		t.Errorf("addMagnify = %v, want 1.3", got)
	}
}

func TestGrownStandsOnTheSameSpot(t *testing.T) {
	d := testDeck(t, 1)
	d.progress = 1
	r := d.layout(960, 768, 8).tiles[0]
	g := grown(r, 1.5)
	if g.Y+g.Height != r.Y+r.Height {
		t.Error("a magnified picture does not stand on the same line")
	}
	if g.X+g.Width/2 != r.X+r.Width/2 {
		t.Error("a magnified picture is not centred where it was")
	}
	if g.Width <= r.Width || g.Height <= r.Height {
		t.Error("a magnified picture is not larger")
	}
}

func TestTileGridAndAllowedLayouts(t *testing.T) {
	for _, c := range []struct {
		l          deckLayout
		n          int
		cols, rows int
	}{
		{layoutSingle, 5, 1, 1},
		{layoutColumns, 2, 2, 1}, {layoutColumns, 4, 4, 1},
		{layoutRows, 3, 1, 3},
		{layoutGrid, 3, 2, 2}, {layoutGrid, 4, 2, 2}, {layoutGrid, 5, 3, 2}, {layoutGrid, 9, 3, 3}, {layoutGrid, 10, 4, 3},
	} {
		cols, rows := tileGrid(c.l, c.n)
		if cols != c.cols || rows != c.rows {
			t.Errorf("%s of %d machines: %dx%d, want %dx%d", deckLayoutNames[c.l], c.n, cols, rows, c.cols, c.rows)
		}
		if cols*rows < c.n && c.l != layoutSingle {
			t.Errorf("%s of %d machines has only %d cells", deckLayoutNames[c.l], c.n, cols*rows)
		}
	}
	if layoutAllowed(layoutColumns, 1) || layoutAllowed(layoutRows, 1) || layoutAllowed(layoutGrid, 2) {
		t.Error("a tiled layout is offered where it means nothing")
	}
	if !layoutAllowed(layoutSingle, 1) || !layoutAllowed(layoutColumns, 2) || !layoutAllowed(layoutGrid, 3) {
		t.Error("a layout that means something is not offered")
	}
}

func TestTileScale(t *testing.T) {
	// Two machines side by side at zoom 3 want 1920 wide: they fit a 1920 monitor.
	if s := tileScale(2, 1, 3, 1920, 1080, 24); s != 3 {
		t.Errorf("2x1 at zoom 3 on 1920x1080: scale %d, want 3", s)
	}
	// Three do not; at 2 they do.
	if s := tileScale(3, 1, 3, 1920, 1080, 24); s != 2 {
		t.Errorf("3x1 at zoom 3 on 1920x1080: scale %d, want 2", s)
	}
	// Stacked, the height decides, with the menu bar's strip counted.
	if s := tileScale(1, 2, 3, 1920, 1080, 24); s != 2 {
		t.Errorf("1x2 at zoom 3 on 1920x1080: scale %d, want 2", s)
	}
	if s := tileScale(1, 2, 2, 1920, 1024, 24); s != 1 {
		t.Errorf("1x2 at zoom 2 on a 1024-high monitor with a 24-pixel strip: scale %d, want 1", s)
	}
	// Never above the zoom, never below 1, and the zoom when the monitor is unknown.
	if s := tileScale(2, 2, 1, 8000, 8000, 0); s != 1 {
		t.Errorf("scale %d above a zoom of 1", s)
	}
	if s := tileScale(9, 9, 3, 640, 480, 0); s != 1 {
		t.Errorf("scale %d, want 1 when nothing fits", s)
	}
	if s := tileScale(2, 1, 3, 0, 0, 0); s != 3 {
		t.Errorf("scale %d with no monitor size, want the zoom", s)
	}
}

func TestDeckTiledLayoutSizesTheWindow(t *testing.T) {
	d := testDeck(t, 1)
	dm := d.dm
	dm.screen.multiplier = 1

	d.setLayout(layoutColumns)
	if d.tiled() || dm.tiles != nil {
		t.Fatal("one machine was tiled")
	}
	d.add("48k")
	d.setLayout(layoutColumns)
	if !d.tiled() || dm.tiles == nil {
		t.Fatal("two machines could not be put side by side")
	}
	// One row at scale 1 is lower than the menus: the window is kept high
	// enough for them, and the tiles sit in its middle.
	if dm.targetWidth != 2*RefWidth || dm.targetHeight != minTiledHeight+dm.reservedTopHeight || !dm.isAnimating {
		t.Errorf("side by side at zoom 1: the window is going to %dx%d (animating %v), want %dx%d", dm.targetWidth, dm.targetHeight, dm.isAnimating, 2*RefWidth, minTiledHeight)
	}
	dm.currentWidth, dm.currentHeight = dm.targetWidth, dm.targetHeight
	if cells := d.tileRects(); cells[0].Height != RefHeight || cells[0].Y != float32(dm.reservedTopHeight)+(minTiledHeight-RefHeight)/2 || cells[1].X != RefWidth {
		t.Errorf("side by side in a window higher than its tiles: %+v", cells)
	}
	// The zoom does nothing while tiled.
	if dm.SetScale(1) || dm.ScaleUp() || dm.ScaleDown() {
		t.Error("the zoom answered in a tiled layout")
	}

	d.setLayout(layoutRows)
	if dm.targetWidth != RefWidth || dm.targetHeight != 2*RefHeight+dm.reservedTopHeight {
		t.Errorf("stacked: the window is going to %dx%d", dm.targetWidth, dm.targetHeight)
	}

	// A third machine: the window follows, and a grid becomes possible.
	d.add("48k")
	if dm.targetHeight != 3*RefHeight+dm.reservedTopHeight {
		t.Errorf("a third machine stacked: height %d", dm.targetHeight)
	}
	d.setLayout(layoutGrid)
	if dm.targetWidth != 2*RefWidth || dm.targetHeight != 2*RefHeight+dm.reservedTopHeight {
		t.Errorf("a grid of three: %dx%d, want 2x2 cells", dm.targetWidth, dm.targetHeight)
	}

	// The cells cover the window as it is now, in reading order.
	dm.currentWidth, dm.currentHeight = dm.targetWidth, dm.targetHeight
	cells := d.tileRects()
	if len(cells) != 3 || cells[0].X != 0 || cells[1].X != RefWidth || cells[2].X != 0 || cells[2].Y != cells[0].Y+RefHeight {
		t.Errorf("the grid's cells: %+v", cells)
	}
	p := pictureIn(cells[1], 256, 192)
	if p.X != cells[1].X+32 || p.Y != cells[1].Y+32 || p.Width != 256 || p.Height != 192 {
		t.Errorf("a classic picture in its cell: %+v", p)
	}
	if p := pictureIn(cells[1], 640, 256); p != cells[1] {
		t.Errorf("a Next's double-width frame does not fill its cell: %+v", p)
	}

	// Back to two machines: a grid means nothing, and the window is one machine's again.
	d.remove(2)
	if d.tiled() || dm.tiles != nil {
		t.Error("a grid of two machines was kept")
	}
	if dm.targetWidth == 2*RefWidth {
		t.Error("the window kept the grid's size")
	}
	if !dm.SetScale(1) {
		t.Error("the zoom does not answer with one machine filling the window")
	}
}

func TestFitOnAxis(t *testing.T) {
	for _, c := range []struct{ pos, size, origin, length, want int }{
		{160, 640, 0, 1280, 160},      // fits: stays
		{128, 1024, 0, 1024, 0},       // as tall as the monitor: to its top
		{900, 640, 0, 1280, 640},      // runs off the right: back by what is over
		{100, 2000, 0, 1280, 0},       // larger than the monitor: at its origin
		{2000, 640, 1920, 1280, 2000}, // on a second monitor, fits
		{3000, 640, 1920, 1280, 2560}, // on a second monitor, off its right
		{1900, 640, 1920, 1280, 1920}, // before the monitor's origin
	} {
		if got := fitOnAxis(c.pos, c.size, c.origin, c.length); got != c.want {
			t.Errorf("fitOnAxis(%d, %d, %d, %d) = %d, want %d", c.pos, c.size, c.origin, c.length, got, c.want)
		}
	}
}

func TestTilePlanWithAMainTile(t *testing.T) {
	// Three machines, the second in front, main on the right: the other two
	// stacked in the left column, the front one twice the size beside them.
	across, down, tiles := tilePlan(layoutMainRight, 3, 1)
	if across != 3 || down != 2 {
		t.Fatalf("three machines, main right: %d by %d cells, want 3 by 2", across, down)
	}
	want := []tileUnits{{0, 0, 1, 1}, {1, 0, 2, 2}, {0, 1, 1, 1}}
	for i := range want {
		if tiles[i] != want[i] {
			t.Errorf("main right, tile %d: %+v, want %+v", i, tiles[i], want[i])
		}
	}
	// Main on the left: the same, mirrored.
	_, _, tiles = tilePlan(layoutMainLeft, 3, 1)
	want = []tileUnits{{2, 0, 1, 1}, {0, 0, 2, 2}, {2, 1, 1, 1}}
	for i := range want {
		if tiles[i] != want[i] {
			t.Errorf("main left, tile %d: %+v, want %+v", i, tiles[i], want[i])
		}
	}

	// For any number: the tiles fill the plan exactly and none overlaps.
	for _, l := range []deckLayout{layoutMainRight, layoutMainLeft, layoutGrid, layoutColumns, layoutRows} {
		for n := 3; n <= 7; n++ {
			for front := 0; front < n; front++ {
				across, down, tiles := tilePlan(l, n, front)
				used := map[[2]int]int{}
				area := 0
				for i, u := range tiles {
					if u.x < 0 || u.y < 0 || u.x+u.w > across || u.y+u.h > down {
						t.Fatalf("%s, %d machines, front %d: tile %d %+v is outside %d by %d", deckLayoutNames[l], n, front, i, u, across, down)
					}
					area += u.w * u.h
					for x := u.x; x < u.x+u.w; x++ {
						for y := u.y; y < u.y+u.h; y++ {
							if j, taken := used[[2]int{x, y}]; taken {
								t.Fatalf("%s, %d machines, front %d: tiles %d and %d overlap", deckLayoutNames[l], n, front, j, i)
							}
							used[[2]int{x, y}] = i
						}
					}
				}
				if (l == layoutMainRight || l == layoutMainLeft) && area != across*down {
					t.Errorf("%s, %d machines: the tiles cover %d of %d cells", deckLayoutNames[l], n, area, across*down)
				}
				if (l == layoutMainRight || l == layoutMainLeft) && tiles[front].w != n-1 {
					t.Errorf("%s, %d machines: the front machine's tile is %d cells wide, want %d", deckLayoutNames[l], n, tiles[front].w, n-1)
				}
			}
		}
	}
	if layoutAllowed(layoutMainRight, 2) || !layoutAllowed(layoutMainLeft, 3) {
		t.Error("a main tile beside a stack should be offered from three machines on")
	}
}

func TestDeckMainTileFollowsTheFrontMachine(t *testing.T) {
	d := testDeck(t, 3)
	dm := d.dm
	dm.screen.multiplier = 1
	d.setLayout(layoutMainRight)
	if dm.targetWidth != 3*RefWidth || dm.targetHeight != 2*RefHeight+dm.reservedTopHeight {
		t.Fatalf("main right with three machines: the window is going to %dx%d, want %dx%d", dm.targetWidth, dm.targetHeight, 3*RefWidth, 2*RefHeight)
	}
	dm.currentWidth, dm.currentHeight = dm.targetWidth, dm.targetHeight

	main := rl.Rectangle{X: RefWidth, Y: float32(dm.reservedTopHeight), Width: 2 * RefWidth, Height: 2 * RefHeight}
	if got := d.tileRects()[d.front]; got != main {
		t.Errorf("the front machine's tile is %+v, want the main tile %+v", got, main)
	}
	// A classic picture in the main tile is twice the size, its border round it.
	if p := pictureIn(main, 256, 192); p.Width != 512 || p.Height != 384 || p.X != main.X+64 || p.Y != main.Y+64 {
		t.Errorf("a classic picture in the main tile: %+v", p)
	}

	// Another machine comes to the front: it has the main tile now, the
	// window keeps its size, and the dock's flight was aimed there.
	aim := d.tileRectsWithFront(2)[2]
	d.flip(2)
	if got := d.tileRects()[2]; got != main || aim != main {
		t.Errorf("after the flip the new front machine's tile is %+v (flight aimed at %+v), want the main tile", got, aim)
	}
	if dm.targetWidth != 3*RefWidth {
		t.Errorf("the flip changed the window's width to %d", dm.targetWidth)
	}
	if got := d.tileRects()[0]; got.Width != RefWidth || got.X != 0 {
		t.Errorf("the machine that was in front is not in the stack: %+v", got)
	}

	// Closing one leaves two, for which this layout means nothing.
	d.remove(1)
	if d.tiled() {
		t.Error("a main tile beside a stack of one was kept")
	}
}

func TestDeckMachineTakesItsFirstProgramsName(t *testing.T) {
	d := testDeck(t, 2)
	a, b := d.machines[0], d.machines[1]
	if a.title() != "48K" {
		t.Fatalf("a machine with no name is called %q, want its model", a.title())
	}

	// The machine behind loads a program from tape; the one in front is
	// renamed by hand and then loads one too.
	b.zx.firstProgram = "MANIC"
	d.rename(0, "  my machine ")
	a.zx.firstProgram = "JETSET"
	d.runBehind(1, 1500)
	if b.title() != "MANIC" {
		t.Errorf("the machine that loaded MANIC is called %q", b.title())
	}
	if a.title() != "my machine" || !a.renamed {
		t.Errorf("the machine named by hand is called %q (renamed %v)", a.title(), a.renamed)
	}

	// The first program names it once: a later one does not rename it.
	b.zx.firstProgram = "LATER"
	d.runBehind(2, 1500)
	if b.title() != "MANIC" {
		t.Errorf("a later program renamed the machine to %q", b.title())
	}

	// A name typed over an automatic one is the person's, and stays.
	d.rename(1, "willy")
	d.runBehind(3, 1500)
	if b.title() != "willy" {
		t.Errorf("after a rename the machine is called %q", b.title())
	}
	// An empty name hands the naming back.
	d.rename(0, "   ")
	d.runBehind(4, 1500)
	if a.title() != "JETSET" || a.renamed {
		t.Errorf("after its name was cleared the machine is called %q (renamed %v), want its first program's", a.title(), a.renamed)
	}
}

// order is the deck's machines as the letters given to them, in order.
func order(d *machineDeck, names map[*deckMachine]string) string {
	s := ""
	for _, m := range d.machines {
		s += names[m]
	}
	return s
}

func TestDeckRotate(t *testing.T) {
	d := testDeck(t, 3)
	names := map[*deckMachine]string{d.machines[0]: "A", d.machines[1]: "B", d.machines[2]: "C"}
	a := d.machines[0]

	// One machine filling the window: the next comes to the front.
	d.rotate()
	if names[d.machines[d.front]] != "B" || order(d, names) != "ABC" {
		t.Fatalf("rotate with one machine shown: front %s, order %s", names[d.machines[d.front]], order(d, names))
	}
	d.flip(0)

	// A plain grid: every machine moves to the next tile, the last to the
	// first, and the one in front stays in front.
	d.setLayout(layoutColumns)
	d.rotate()
	if order(d, names) != "CAB" {
		t.Errorf("rotate side by side: order %s, want CAB", order(d, names))
	}
	if d.machines[d.front] != a || a.zx.display != Display(d.dm) {
		t.Error("rotating the tiles changed which machine is in front")
	}
	d.rotate()
	d.rotate()
	if order(d, names) != "ABC" || d.machines[d.front] != a {
		t.Errorf("three rotations of three machines: order %s", order(d, names))
	}

	// A main tile: the next machine takes it.
	d.setLayout(layoutMainRight)
	d.rotate()
	if names[d.machines[d.front]] != "B" || d.machines[d.front].zx.display != Display(d.dm) {
		t.Errorf("rotate with a main tile: front %s", names[d.machines[d.front]])
	}

	// One machine alone: nothing to rotate.
	solo := testDeck(t, 1)
	solo.rotate()
	if solo.front != 0 {
		t.Error("rotating one machine moved it")
	}
}

func TestDeckDropOnATile(t *testing.T) {
	d := testDeck(t, 3)
	names := map[*deckMachine]string{d.machines[0]: "A", d.machines[1]: "B", d.machines[2]: "C"}
	front := func() string { return names[d.machines[d.front]] }

	// One machine filling the window: the window is the tile, and a machine
	// dropped on it comes to the front.
	d.place(2, d.front)
	if front() != "C" || order(d, names) != "ABC" {
		t.Fatalf("drop on the window: front %s, order %s", front(), order(d, names))
	}
	d.flip(0)

	// A plain grid: the two change tiles, the one in front stays in front.
	d.setLayout(layoutColumns)
	d.place(2, 0) // C onto A's tile
	if order(d, names) != "CBA" || front() != "A" {
		t.Errorf("drop on a tile of a row: order %s, front %s; want CBA with A still in front", order(d, names), front())
	}
	if d.machines[d.front].zx.display != Display(d.dm) {
		t.Error("the front machine lost the display")
	}
	d.place(1, 1) // onto its own tile: nothing
	if order(d, names) != "CBA" {
		t.Errorf("a drop on a machine's own tile moved it: %s", order(d, names))
	}

	// A main tile: dropped on it, a machine comes to the front, and the one
	// that had it goes where the dropped one was.
	d.setLayout(layoutMainRight)
	c := d.machines[0]
	d.place(0, d.front) // C onto the main tile, which A has
	if front() != "C" || c.zx.display != Display(d.dm) {
		t.Errorf("drop on the main tile: front %s", front())
	}
	if order(d, names) != "ABC" {
		t.Errorf("the machine that had the main tile is not where the dropped one was: %s", order(d, names))
	}
	// Dropped on a stacked tile, two stacked machines change places.
	d.place(0, 1)
	if order(d, names) != "BAC" || front() != "C" {
		t.Errorf("drop on a stacked tile: order %s, front %s", order(d, names), front())
	}

	// Where the pointer is decides the tile; over the dock or the menu
	// bar's strip it is none.
	d.dm.currentWidth, d.dm.currentHeight = d.dm.targetWidth, d.dm.targetHeight
	cells := d.tileRects()
	for i, cell := range cells {
		if got := d.dropSite(cell.X+cell.Width/2, cell.Y+cell.Height/2, 1e6); got != i {
			t.Errorf("the middle of tile %d is drop site %d", i, got)
		}
	}
	if got := d.dropSite(10, 300, 200); got != -1 {
		t.Errorf("a point under the dock is drop site %d", got)
	}
}

func TestLayoutByNameMatchesTheMenu(t *testing.T) {
	// Every name parseLayout can return is one layoutByName knows.
	for _, name := range layoutNames {
		if _, ok := layoutByName(name); !ok {
			t.Errorf("layoutByName has no layout called %q", name)
		}
	}
	if l, ok := layoutByName("Grid"); !ok || l != layoutGrid {
		t.Errorf("layoutByName(Grid) = %v, %v; want layoutGrid", l, ok)
	}
	if _, ok := layoutByName("nonesuch"); ok {
		t.Error("layoutByName accepted a name that is not a layout")
	}
}
