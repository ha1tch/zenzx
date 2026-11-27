//go:build !headless

package zenzx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ha1tch/zenui"
)

// Stopping a machine: the animation's timing, its pictures (which can be replaced), the deck's
// part in it (the machine is gone from the dock, and the last one leaves the background), and
// the Machine menu's rows that ask for it.

func TestStompTimingTheGroundRisesTheBootStampsAndTheGroundGoes(t *testing.T) {
	const step = 0.005
	// The ground comes up from below: nothing is there at the start, the stage only rises until
	// it has passed its place a little, and then it settles there.
	if p, _ := stompAt(0); p.stage != 0 || p.boot != 0 || p.landed {
		t.Errorf("at the start: %+v", p)
	}
	peak, peakAt := float32(0), 0.0
	for t0 := 0.0; t0 < stompDropStart; t0 += step {
		p, done := stompAt(t0)
		if done || p.landed || p.boot != 0 || p.stage < 0 || p.stage > 1.15 || p.dim < 0 || p.dim > 1 {
			t.Fatalf("at %.3fs while the ground comes up: %+v done=%v", t0, p, done)
		}
		if p.stage > peak {
			peak, peakAt = p.stage, t0
		}
	}
	if peak <= 1 || peak > 1.12 || peakAt <= stompRiseEnd || peakAt >= stompDropStart {
		t.Errorf("the ground goes up to %.3f at %.3fs: want a little past its place, after it first reaches it (%.2fs) and before it settles (%.2fs)", peak, peakAt, stompRiseEnd, stompDropStart)
	}
	if p, _ := stompAt(stompRiseEnd); p.stage < 0.97 || p.stage > 1.03 {
		t.Errorf("at %.2fs the ground is at %v: it should be reaching its place", stompRiseEnd, p.stage)
	}
	if p, _ := stompAt(stompDropStart - 0.001); p.stage < 0.99 || p.stage > 1.01 {
		t.Errorf("when the boot starts to fall the ground is at %v: it should have settled", p.stage)
	}
	for t0 := 0.0; t0 < peakAt-step; t0 += step {
		if a, _ := stompAt(t0); a.stage > func() float32 { b, _ := stompAt(t0 + step); return b.stage }() {
			t.Fatalf("the ground went down at %.3fs, on its way up", t0)
		}
	}
	for t0 := stompDropStart; t0 < stompRetireStart; t0 += step {
		if p, _ := stompAt(t0); p.stage != 1 || p.dim != 1 {
			t.Fatalf("at %.3fs the ground is at %v, dim %v: it should stay in its place", t0, p.stage, p.dim)
		}
	}

	// The drop: the boot comes down only after that, and has not landed.
	var last float32 = -1
	for t0 := 0.0; t0 < stompDropEnd-0.001; t0 += step {
		p, done := stompAt(t0)
		if done || p.landed || p.boot < last {
			t.Fatalf("at %.3fs during the drop: %+v done=%v (the last position was %v)", t0, p, done, last)
		}
		if t0 < stompDropStart && p.boot != 0 {
			t.Fatalf("at %.3fs the boot is at %v, before it starts to fall", t0, p.boot)
		}
		last = p.boot
	}
	on := (stompDropEnd + stompHoldEnd) / 2
	if p, _ := stompAt(on); p.boot != 1 || !p.landed {
		t.Errorf("at %.3fs the boot is %+v, want it on the icon", on, p)
	}
	last = 2
	for t0 := stompHoldEnd; t0 < stompLiftEnd-0.001; t0 += step { // the lift: only up
		p, _ := stompAt(t0)
		if p.boot > last || !p.landed {
			t.Fatalf("at %.3fs during the lift: %+v (the last position was %v)", t0, p, last)
		}
		last = p.boot
	}
	if p, _ := stompAt(stompLiftEnd); p.boot != 0 || !p.landed || p.stage != 1 {
		t.Errorf("when the boot is back at the top: %+v", p)
	}

	// The bin has weight: thrown up a little when the ground stops, pushed down when the boot
	// lands, thrown up a little when the ground goes, and still in between and at the end.
	bumpAt := func(t0 float64) float32 { p, _ := stompAt(t0); return p.bump }
	if b := bumpAt(stompRiseEnd + 0.07); b > -0.5 {
		t.Errorf("when the ground stops the bin is at %v: it should be thrown up", b)
	}
	if b := bumpAt(stompDropEnd + 0.07); b < 2 {
		t.Errorf("when the boot lands the bin is at %v: it should be pushed down", b)
	}
	if b := bumpAt(stompRetireStart + 0.07); b > -0.5 {
		t.Errorf("when the ground starts to go the bin is at %v: it should be thrown up", b)
	}
	for _, t0 := range []float64{0, stompRiseEnd - 0.01, stompDropStart, stompDropEnd - 0.01, stompLiftEnd, stompRetireStart - 0.01} {
		if b := bumpAt(t0); b > 0.2 || b < -0.2 {
			t.Errorf("at %.3fs the bin is jolted by %v: it should be still", t0, b)
		}
	}

	// The retiring: the ground goes back down, with the bin and what is left of the machine, a
	// little up first, and is gone at the end.
	if p, _ := stompAt(stompRetireStart); p.stage != 1 {
		t.Errorf("when the ground starts to go: %+v", p)
	}
	mid := (stompRetireStart + stompDuration) / 2
	if p, _ := stompAt(mid); p.stage <= 0 || p.stage >= 1 || !p.landed {
		t.Errorf("midway through the retiring: %+v", p)
	}
	prev := float32(2)
	for t0 := stompRetireStart + 0.15; t0 < stompDuration-0.001; t0 += step {
		p, _ := stompAt(t0)
		if p.stage > prev {
			t.Fatalf("the ground came up at %.3fs, on its way down", t0)
		}
		prev = p.stage
	}
	if _, done := stompAt(stompDuration - 0.001); done {
		t.Error("over before its time")
	}
	if p, done := stompAt(stompDuration); !done || p.stage != 0 || p.dim != 0 {
		t.Errorf("at the end: %+v done=%v", p, done)
	}
	if stompDuration >= 3 {
		t.Errorf("the stomp takes %vs: it is meant to be over in under three seconds", stompDuration)
	}
}

func TestStompPicturesAreDrawnAndAFileInTheFolderReplacesOne(t *testing.T) {
	own := loadStompImages(emptyFS{})
	for name, img := range map[string]*image.RGBA{"icon": own.icon, "boot": own.boot, "leg": own.leg} {
		opaque := 0
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0 {
				opaque++
			}
		}
		if opaque == 0 {
			t.Errorf("ZenZX's own %s is blank", name)
		}
	}
	if own.flat != nil {
		t.Error("ZenZX has a flat picture of its own: the squashed icon is meant to be drawn from the icon")
	}
	// The leg is stacked: its top row and bottom row must be alike, or a seam shows.
	for x := 0; x < own.leg.Bounds().Dx(); x++ {
		if own.leg.RGBAAt(x, 0) != own.leg.RGBAAt(x, own.leg.Bounds().Dy()-1) {
			t.Errorf("the leg's top and bottom differ at column %d", x)
		}
	}

	var buf bytes.Buffer
	mine := image.NewRGBA(image.Rect(0, 0, 3, 5))
	mine.Set(1, 2, color.RGBA{R: 9, G: 8, B: 7, A: 255})
	if err := png.Encode(&buf, mine); err != nil {
		t.Fatal(err)
	}
	got := loadStompImages(fstest.MapFS{"icon.png": {Data: buf.Bytes()}, "flat.png": {Data: buf.Bytes()}, "boot.png": {Data: []byte("not a png")}})
	if b := got.icon.Bounds(); b.Dx() != 3 || b.Dy() != 5 || got.icon.RGBAAt(1, 2).R != 9 {
		t.Errorf("a PNG called icon.png was not used: %v", got.icon.Bounds())
	}
	if got.flat == nil {
		t.Error("flat.png was not used")
	}
	if got.boot.Bounds() != own.boot.Bounds() || got.leg.Bounds() != own.leg.Bounds() {
		t.Error("a picture with no usable file did not keep ZenZX's own")
	}
}

func TestTheFolderShippedHasNoPicturesYetAndSaysHowToAddThem(t *testing.T) {
	if _, err := stompFiles.ReadFile("sprites/stomp/README.txt"); err != nil {
		t.Errorf("sprites/stomp/README.txt is not embedded: %v", err)
	}
	if got := embeddedStompImages(); got.icon == nil || got.boot == nil || got.leg == nil || got.bin == nil || got.groundTop == nil || got.ground == nil {
		t.Error("the embedded folder gave no pictures")
	}
}

func TestMachineMenuHeadSaysWhatItWillDoAndHasItsIcon(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	labels := func() []string {
		var l []string
		for _, it := range b.widget.ItemsFor(barMachine)[:machineHeadRows] {
			l = append(l, it.Label)
		}
		return l
	}
	b.refreshMachineItems(zx)
	if got := labels(); got[0] != "Machine" || got[1] != "Reset" || got[2] != "Play" || got[3] != "Pause" || got[4] != "Stop" {
		t.Fatalf("the rows are %v, want Reset, Play, Pause and Stop whatever the machine is doing", got)
	}
	// Each row is enabled only when it can act, as the Tape menu's are: Reset, Pause and Stop while
	// the machine runs; Play while it is paused or stopped.
	enabled := func() string {
		var s string
		for _, it := range b.widget.ItemsFor(barMachine)[1:machineHeadRows] {
			if it.Disabled {
				s += "-"
			} else {
				s += "+"
			}
		}
		return s
	}
	if got := enabled(); got != "+-++" {
		t.Errorf("a running machine has Reset, Play, Pause, Stop enabled as %s, want +-++", got)
	}
	zx.paused = true
	b.refreshMachineItems(zx)
	head := b.widget.ItemsFor(barMachine)[:machineHeadRows]
	if got := enabled(); got != "++-+" {
		t.Errorf("a paused machine has them enabled as %s, want ++-+", got)
	}
	for i := 1; i < machineHeadRows; i++ {
		if !head[0].Title || head[i].Icon == nil {
			t.Errorf("row %d (%s) has no icon", i, head[i].Label)
		}
	}
	zx.paused, zx.stopped = false, true
	b.refreshMachineItems(zx)
	if got := enabled(); got != "-+--" {
		t.Errorf("a stopped machine has them enabled as %s, want -+--", got)
	}
	// The rows still lead to the same things: the model rows keep their places after them.
	items := b.widget.ItemsFor(barMachine)
	if len(items) != machineHeadRows+len(b.machineGroupItems) {
		t.Errorf("%d rows, want the head and the %d model rows", len(items), len(b.machineGroupItems))
	}

	// The icons are the transport sprites: Play and Pause differ, and each draws something.
	draw := func(name string) []paintFill {
		r := &paintRenderer{}
		transportIcon(name)(r, zenui.Rect{X: 10, Y: 10, W: 16, H: 16}, zenui.Colour{R: 1, G: 2, B: 3, A: 255})
		return r.fills
	}
	play, pause, stop, recycle := draw("play"), draw("pause"), draw("stop"), draw("recycle")
	if len(play) == 0 || len(pause) == 0 || len(stop) == 0 || len(recycle) == 0 {
		t.Fatalf("an icon drew nothing: play %d, pause %d, stop %d, recycle %d", len(play), len(pause), len(stop), len(recycle))
	}
	if len(play) == len(pause) && play[0] == pause[0] {
		t.Error("the Play and Pause icons draw the same thing")
	}
	for _, f := range append(append(append(play, pause...), stop...), recycle...) {
		if f.rect.X < 10 || f.rect.Y < 10 || f.rect.X+f.rect.W > 26 || f.rect.Y+f.rect.H > 26 || f.colour.R != 1 {
			t.Fatalf("an icon pixel %+v is outside its cell or in the wrong colour", f)
		}
	}
}

func TestStopRowAsksTheDeckAndMachineRowsAreDisabledWhenNoneRuns(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	stopped := 0
	b.stopMachine = func() { stopped++ }
	b.actions[barMachine][4](zx) // Stop: after the title and Reset, Play and Pause
	if stopped != 1 {
		t.Errorf("Stop asked the deck %d times, want once", stopped)
	}
	zx.stopped = true
	b.refreshMachineItems(zx)
	for i, it := range b.widget.ItemsFor(barMachine)[1:machineHeadRows] { // the title is not a row to disable
		// Play is the one row that does something with none running: it starts a machine
		if want := i+1 != machinePlayRow; it.Disabled != want {
			t.Errorf("row %d (%s) has Disabled %v with no machine running, want %v", i+1, it.Label, it.Disabled, want)
		}
	}
	// the rows of the Snapshot menu that act on a machine: Quick Save, Save Timestamped, Save As,
	// Load Binary, Snapshot Info and Run Diagnostics
	snapNeeds := map[string]bool{"Quick Save": true, "Save Timestamped": true, "Save As": true,
		"Save Multisnapshot...": true, "Load Binary...": true, "Snapshot Info": true, "Run Diagnostics": true}
	for i, e := range b.snapSpec {
		sel := zenui.MenuBarSelection{BarIndex: barSnapshot, ItemIndex: i}
		if got := b.needsMachine(sel); got != snapNeeds[e.label] {
			t.Errorf("Snapshot > %q needs a machine: %v, want %v", e.label, got, snapNeeds[e.label])
		}
	}
	for _, sel := range []zenui.MenuBarSelection{{BarIndex: barFile, ItemIndex: 1}, {BarIndex: barTape, ItemIndex: 1}, {BarIndex: barFloppyDisk}, {BarIndex: barCustomROM}} {
		if !b.needsMachine(sel) {
			t.Errorf("%+v is allowed with no machine", sel)
		}
	}
	// File > Open is allowed with none: a snapshot opened makes its machine
	for _, sel := range []zenui.MenuBarSelection{{BarIndex: barFile, ItemIndex: 0}, {BarIndex: barFile, ItemIndex: 2}, {BarIndex: barFile, ItemIndex: 4}, {BarIndex: barMachine, ItemIndex: machineHeadRows}, {BarIndex: barTape, ItemIndex: 3}, {BarIndex: barTape, ItemIndex: 4}, {BarIndex: barView}, {BarIndex: barTheme}} {
		if b.needsMachine(sel) {
			t.Errorf("%+v is refused with no machine: it does not need one", sel)
		}
	}
}

func finishStomp(d *machineDeck) {
	d.stomp.start = time.Now().Add(-time.Duration((stompDuration + 1) * float64(time.Second)))
	d.tickStomp(time.Now())
}

func TestStoppingTheFrontMachineHoldsItThenRemovesItFromTheDock(t *testing.T) {
	d := testDeck(t, 3)
	a, b, c := d.machines[0].zx, d.machines[1].zx, d.machines[2].zx
	d.stop(0)
	if !d.Stopping() || !a.stopping || !a.paused {
		t.Fatalf("stopping=%v machine.stopping=%v paused=%v: the machine is not held", d.Stopping(), a.stopping, a.paused)
	}
	d.stop(1) // one at a time
	if b.stopping || len(d.machines) != 3 {
		t.Error("a second stop was taken while the first plays")
	}
	d.tickStomp(time.Now())
	if len(d.machines) != 3 {
		t.Error("the machine was gone before the animation was over")
	}
	finishStomp(d)
	if d.Stopping() || len(d.machines) != 2 || d.Front() != b || a.stopping {
		t.Errorf("after: stopping=%v, %d machines, front is the second: %v", d.Stopping(), len(d.machines), d.Front() == b)
	}
	if d.machines[0].zx != b || d.machines[1].zx != c {
		t.Error("the dock does not list the two that are left, in order")
	}
}

func TestAMachineNotOnScreenIsStoppedAtOnce(t *testing.T) {
	d := testDeck(t, 2)
	b := d.machines[1].zx
	d.stop(1) // behind the front one, in the single layout: there is nothing to play on
	if d.Stopping() || len(d.machines) != 1 || d.Front() == b {
		t.Errorf("stopping=%v, %d machines", d.Stopping(), len(d.machines))
	}
}

func TestStoppingTheLastMachineLeavesTheBackgroundAndANewOneTakesItsPlace(t *testing.T) {
	d := testDeck(t, 1)
	only := d.machines[0].zx
	d.stop(0)
	finishStomp(d)
	if !d.Empty() || len(d.machines) != 0 {
		t.Fatalf("%d machines are left", len(d.machines))
	}
	if d.Front() != only || !only.stopped || !only.paused || only.stopping {
		t.Errorf("the last machine is not left stopped: front=%v stopped=%v paused=%v stopping=%v", d.Front() == only, only.stopped, only.paused, only.stopping)
	}
	if d.frontModel() != "48k" || d.frontCaption() == "" || d.modelForNew() != "48k" {
		t.Errorf("an empty deck says %q, %q, %q", d.frontModel(), d.frontCaption(), d.modelForNew())
	}
	// Nothing in the deck breaks with no machine in it.
	d.flip(0)
	d.rotate()
	d.place(0, 1)
	d.setFrontModel("128k")
	d.stopFront()
	d.remove(0)
	d.followPlayerFocus()
	d.runBehind(1, 100)
	d.setLayout(layoutColumns)
	if d.tiled() {
		t.Error("a tiled layout of no machines")
	}
	if d.Stopping() {
		t.Error("a stop was started with no machine")
	}

	// New (the dock's, or a model in the Machine menu) starts one, which has the window.
	nz := d.start("128k")
	if nz == nil || d.Empty() || d.Front() != nz || d.frontModel() != "128k" {
		t.Fatalf("start gave %v, front is it: %v", nz, d.Front() == nz)
	}
	if nz.stopped {
		t.Error("the new machine is stopped")
	}
	if nz.display != Display(d.dm) {
		t.Error("the new machine has no display")
	}
	if _, ok := only.display.(*nullDisplay); !ok {
		t.Errorf("the stopped machine still has the window's display: %T", only.display)
	}
	if d.start("48k") != nil {
		t.Error("start made a machine when one was running")
	}
}

func TestDockNewStartsAMachineWhenNoneRunsAndDeleteStopsOne(t *testing.T) {
	d := testDeck(t, 1)
	d.stop(0)
	finishStomp(d)
	d.choose(0, dockLayout{}) // the one tile there is is New
	if d.Empty() || d.Front().stopped {
		t.Fatal("New did not start a machine")
	}
	if d.modelForNew() != "48k" {
		t.Errorf("the next model is %q", d.modelForNew())
	}
}

func TestPickingAModelWithNoMachineRunningStartsOne(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	var started []string
	zx.stopped = true
	b.startMachine = func(model string) *ZenZX { started = append(started, model); return NewZenZX(AudioBackendOto) }
	// 128K is a model row of the default Machine menu.
	row := -1
	for i, f := range b.machineModelFlags {
		if f == "128k" {
			row = i
		}
	}
	if row < 0 {
		t.Skip("no 128k row")
	}
	b.pickMachineModel(zx, zenui.MenuBarSelection{BarIndex: barMachine, ItemIndex: row, SubIndex: -1})
	if len(started) != 1 || started[0] != "128k" {
		t.Errorf("started %v, want the 128k", started)
	}
}

// With the Stop animation turned off in the Preferences a machine goes at once, whether it is in
// front or not, and the last one leaves the background as it would after the animation.
func TestStoppingWithTheAnimationOffIsAtOnce(t *testing.T) {
	d := testDeck(t, 2)
	animate := false
	d.stopAnimation = func() bool { return animate }
	front := d.machines[d.front].zx
	d.stop(d.front)
	if d.Stopping() || front.stopping {
		t.Fatal("the animation is playing though it is off")
	}
	if len(d.machines) != 1 {
		t.Fatalf("%d machines are left, want 1 without waiting", len(d.machines))
	}
	d.stopFront()
	if d.Stopping() || !d.Empty() {
		t.Errorf("the last machine is not gone at once: stopping=%v empty=%v", d.Stopping(), d.Empty())
	}
	// And on, it plays.
	d.start("48k")
	animate = true
	d.stop(0)
	if !d.Stopping() {
		t.Error("the animation does not play when it is on")
	}
}

// The Play row of the Machine menu starts a machine when none runs, of the model the menu has ticked,
// and lets a paused one go on.
func TestPlayRowStartsTheModelThatWasStoppedAndResumesAPausedMachine(t *testing.T) {
	b, zx := newTestBar(t, nil, "")
	var started []string
	b.startMachine = func(model string) *ZenZX { started = append(started, model); return NewZenZX(AudioBackendOto) }
	b.setCurrentModel("128k")
	zx.paused = true
	b.actions[barMachine][machinePlayRow](zx)
	if zx.paused || len(started) != 0 {
		t.Errorf("Play on a paused machine: paused %v, started %v; want it going on and nothing started", zx.paused, started)
	}
	b.actions[barMachine][machinePlayRow](zx) // running: nothing to do
	if zx.paused || len(started) != 0 {
		t.Errorf("Play on a running machine changed something: paused %v, started %v", zx.paused, started)
	}
	b.actions[barMachine][3](zx) // Pause
	if !zx.paused {
		t.Error("Pause did not pause")
	}
	b.actions[barMachine][3](zx) // Pause on a paused machine stays paused
	if !zx.paused {
		t.Error("Pause on a paused machine let it go")
	}
	zx.paused, zx.stopped = false, true
	b.actions[barMachine][machinePlayRow](zx)
	if len(started) != 1 || started[0] != "128k" {
		t.Errorf("Play on a stopped machine started %v, want the 128k that was ticked", started)
	}
	if b.needsMachine(zenui.MenuBarSelection{BarIndex: barMachine, ItemIndex: machinePlayRow}) {
		t.Error("Play is refused with no machine running")
	}
}
