//go:build !headless

package zenzx

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/raycanvas"
	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
)

// ============================================================================
// Several machines in one window, and the dock that flips between them
//
// The window shows one machine, the front one: it has the display, the sound
// device and the keyboard. The others keep running behind it, silent and
// unseen. The dock rises from the bottom edge the way the menu bar comes down
// from the top, with a live picture of every machine; choosing one makes its
// picture grow out of the dock until it fills the screen, and that machine is
// then the front one.
//
// The one DisplayManager and the one sound device are handed from machine to
// machine (SetDisplay, the audio's SetDevice): nothing is made per machine but
// the machine itself and its picture in the dock.
// ============================================================================

const (
	dockTileMin      = 84                     // px, the least height of a picture in the dock at rest
	dockTileMax      = 160                    // px, the most; between them it is a sixth of the window's height
	dockTileGap      = 14                     // px between pictures
	dockPad          = 12                     // px around them
	dockMagnify      = 0.45                   // how much the picture under the pointer grows
	dockFlyTime      = 380 * time.Millisecond // a picture's way from the dock to the screen
	dockHideMargin   = 48                     // px above the dock before it leaves again
	dockThumbEvery   = 4                      // frames between refreshes of the pictures
	dockMagnifySpeed = 0.35                   // how fast a picture follows the pointer, per frame
)

// deckFocusKeys and deckFocusKeysKP are the 1..9 keys, main row and keypad,
// that with Alt held bring that machine to the front (index 0..8). Alt+digit
// is free of OS shortcuts on macOS, Windows and common Linux desktops, and
// fits the program's other Alt controls (Alt+M, Alt+F, Alt+P ...).
var deckFocusKeys = [9]int32{
	rl.KeyOne, rl.KeyTwo, rl.KeyThree, rl.KeyFour, rl.KeyFive,
	rl.KeySix, rl.KeySeven, rl.KeyEight, rl.KeyNine,
}
var deckFocusKeysKP = [9]int32{
	rl.KeyKp1, rl.KeyKp2, rl.KeyKp3, rl.KeyKp4, rl.KeyKp5,
	rl.KeyKp6, rl.KeyKp7, rl.KeyKp8, rl.KeyKp9,
}

// deckMachine is one machine of the deck and its picture in the dock.
type deckMachine struct {
	zx    *ZenZX
	model string
	// name is the machine's name when it has one of its own: the first BASIC
	// program it loaded, or what the person called it. renamed says the
	// person did, after which the name is theirs and is left alone.
	name    string
	renamed bool

	thumb  rl.Texture2D
	thumbW int
	thumbH int
	pixels []color.RGBA
	mag    float32

	// fromSnapshot is the file (snapshotKey) the machine was made for, or loaded last: the machine
	// that a snapshot opened again is loaded into, and not followed by another (snapshotopen_gui.go).
	fromSnapshot string
}

// machineDeck is the machines of one window and the dock.
type machineDeck struct {
	machines []*deckMachine
	front    int

	// idle is the machine that was stopped last, when no machine is left to show (machines is
	// then empty): the window keeps it as the machine it shows, with nothing running and
	// nothing drawn but the background, until a machine is started to take its place
	// (machinedeck_stop_gui.go). nil while there are machines.
	idle      *ZenZX
	idleModel string // the model it was, for the next machine to be
	// stomp is the animation of a machine being stopped, nil when none is; textures are its
	// pictures, made when it first plays.
	stomp    *stompAnim
	textures *stompTextures

	// dockOn and multiOn, when set, are the Preferences: whether the dock may be shown, and
	// whether it may add machines. Unset is on, as it always was.
	dockOn, multiOn func() bool
	// stopAnimation, when set, is the Preference for the animation that plays as a machine is
	// stopped (stomp_gui.go): off, the machine goes at once.
	stopAnimation func() bool

	// focusFollow, when set, moves the front (and so the keyboard) to the
	// machine whose player beacon is set -- for an example whose program
	// writes playerBeaconAddr. Off by default: an ordinary multi-machine
	// session has no such convention.
	focusFollow bool

	dm  *DisplayManager
	dev AudioDevice
	// make builds a new machine of a model, set up as the program's options ask.
	make func(model string) *ZenZX
	// retype makes a machine another model, and tells the menu bar so; the way a snapshot gets the
	// machine it needs when none may be added (snapshotopen_gui.go).
	retype func(zx *ZenZX, model string)
	// cardDirs lists the folders card images are kept in, mountCards gives a Next the cards a snapshot of
	// a Next was made over, askCards puts the question to the person when one is not to be found, and
	// opened hears that a snapshot that had to wait for the answer has been loaded (snapshotopen_gui.go).
	cardDirs   func() []string
	mountCards func(zx *ZenZX, c cardSlots) error
	askCards   func(a *cardsAsk)
	opened     func(path string)

	state          barState
	progress       float32
	stateChangedAt time.Time
	byKey          bool // opened with the keyboard: the pointer leaving does not close it
	sel            int  // the chosen tile; len(machines) is the "+" tile
	frame          int

	lastMouseX, lastMouseY int32
	idling                 bool
	idleSince              time.Time
	// edgeArmed is false from the dock's closing until the pointer has left
	// the bottom edge, so that a pointer still resting there does not open
	// it again at once.
	edgeArmed bool

	// tiling is how the machines are shown: one filling the window, or all
	// of them in tiles. theme is the one the dock was last drawn with, for the
	// frame round the front machine's tile.
	tiling deckLayout
	theme  zenui.Theme

	// round draws the dock's rounded shapes; see dockround_gui.go.
	round      *raycanvas.Context
	roundCache *raycanvas.SharedCache

	flying   bool
	flyTo    int
	flyFrom  rl.Rectangle
	flyStart time.Time

	// A press on a machine's picture in the dock is a choice if the button
	// comes up where it went down, and the start of a drag if the pointer
	// moves first: pressOn is the picture (-1 for none), dragging that it
	// has become a drag.
	pressOn        int
	pressX, pressY int32
	dragging       bool
	dragX, dragY   float32
	// edit is the field a machine's name is being typed into, editing the
	// machine (-1 for none).
	edit    *zenui.TextField
	editing int

	// The New button: how far it has grown under the pointer, as a picture does; whether the
	// pointer is on it and whether a press that began on it is still held (it acts when it is
	// let go on it, so that a press can be taken back by moving away); and whether the pointer
	// is a hand because of it.
	addMag   float32
	addHover bool
	addDown  bool
	addHand  bool

	// reveal is the dock being shown for a machine made for a snapshot (dockreveal_gui.go).
	reveal dockReveal
}

// newMachineDeck makes a deck holding one machine, which has the display and
// the sound device already.
func newMachineDeck(zx *ZenZX, model string, dm *DisplayManager, dev AudioDevice, make func(model string) *ZenZX) *machineDeck {
	return &machineDeck{
		machines:  []*deckMachine{{zx: zx, model: model, mag: 1}},
		dm:        dm,
		dev:       dev,
		make:      make,
		edgeArmed: true,
		pressOn:   -1,
		editing:   -1,
	}
}

// Front returns the machine the window shows.
func (d *machineDeck) Front() *ZenZX {
	if len(d.machines) == 0 {
		return d.idle
	}
	return d.machines[d.front].zx
}

// frontCaption is a short label for the front machine, for the debugger HUD:
// its dock number and its title (name or model), e.g. "Machine 2 · 128K".
func (d *machineDeck) frontCaption() string {
	if len(d.machines) == 0 {
		return "No machine"
	}
	m := d.machines[d.front]
	return fmt.Sprintf("Machine %d - %s", d.front+1, m.title())
}

// Active reports that the dock has the keyboard and the pointer.
func (d *machineDeck) Active() bool { return d.state != barHidden || d.flying }

// modelLabel is a model's name as the dock writes it.
func modelLabel(model string) string {
	switch model {
	case "48k":
		return "48K"
	case "128k":
		return "128K"
	case "plus2":
		return "+2"
	case "plus2a":
		return "+2A"
	case "plus3":
		return "+3"
	case "spanish48k":
		return "48K ES"
	case "spanish128k":
		return "128K ES"
	case "spanishplus2":
		return "+2 ES"
	case "spanishplus3":
		return "+3 ES"
	case "next":
		return "Next"
	case "ts2068":
		return "TS2068"
	}
	return strings.ToUpper(model)
}

// modelForNew is the model of a machine added from the dock: the front
// machine's, except that a second Next would boot from the first one's SD card
// while that one writes to it, so beside a Next the new machine is a 48K.
func (d *machineDeck) modelForNew() string {
	if len(d.machines) == 0 {
		return d.idleModel
	}
	m := d.machines[d.front].model
	if m == "next" {
		return "48k"
	}
	return m
}

// add makes a new machine and returns its place in the deck. It runs behind
// the front machine until it is flipped to.
func (d *machineDeck) add(model string) int {
	if len(d.machines) == 0 {
		return d.revive(model)
	}
	zx := d.make(model)
	d.machines = append(d.machines, &deckMachine{zx: zx, model: model, mag: 1})
	d.applyLayout()
	sayf("Machines: added a %s (%d running)\n", modelLabel(model), len(d.machines))
	return len(d.machines) - 1
}

// remove ends machine i. The last machine cannot be removed; removing the
// front one brings its neighbour to the front first.
func (d *machineDeck) remove(i int) {
	if len(d.machines) < 2 || i < 0 || i >= len(d.machines) {
		return
	}
	if i == d.front {
		next := i + 1
		if next >= len(d.machines) {
			next = i - 1
		}
		d.flip(next)
	}
	m := d.machines[i]
	saveDiskOnClose(m.zx)
	m.zx.detachNextBootChain()
	if m.thumbW != 0 {
		rl.UnloadTexture(m.thumb)
	}
	d.machines = append(d.machines[:i], d.machines[i+1:]...)
	if d.front > i {
		d.front--
	}
	if d.sel >= len(d.machines) {
		d.sel = len(d.machines) - 1
	}
	d.applyLayout()
	sayf("Machines: closed a %s (%d running)\n", modelLabel(m.model), len(d.machines))
}

// flip brings machine i to the front: the display, the sound device and the
// keyboard pass from the machine that had them to it.
func (d *machineDeck) flip(i int) {
	if len(d.machines) == 0 || i == d.front || i < 0 || i >= len(d.machines) {
		return
	}
	prev, next := d.machines[d.front].zx, d.machines[i].zx

	prev.io.ResetKeyboard()
	prev.audio.Close()
	prev.audio.SetDevice(nil)
	prev.SetDisplay(nil)

	d.dm.showMachine(next)
	next.SetDisplay(d.dm)
	d.dm.SetVideoRenderer(next.videoRenderer)
	next.io.ResetKeyboard()
	if next.audio.IsEnabled() {
		next.audio.SetDevice(d.dev)
		if err := next.audio.Initialize(); err != nil {
			sayf("Warning: Audio initialization failed: %v\n", err)
			next.notice("The sound could not be started for this machine: %v", err)
		}
	}
	d.front = i
}

// playerBeaconAddr is the byte a focus-follow example writes 1 to on the
// machine that currently holds the player, and 0 elsewhere. It is a fixed
// address so the deck needs no symbol table; the examples/world engine keeps
// it in sync with its own player-present flag.
const playerBeaconAddr = 0xBFFE

// followPlayerFocus, when focus-follow is on, brings to the front whichever
// machine has its player beacon set, so the keyboard always drives the
// machine the player is on. It acts only on a clean hand-off: exactly one
// machine's beacon is set and it is not already the front one. While the
// player is mid-crossing (no machine, or two, assert the beacon) it waits,
// so a dropped or duplicated frame never steals focus.
func (d *machineDeck) followPlayerFocus() {
	if !d.focusFollow {
		return
	}
	holder, count := -1, 0
	for i, m := range d.machines {
		if m.zx.memory.Read(playerBeaconAddr) != 0 {
			holder, count = i, count+1
		}
	}
	if count == 1 && holder != d.front {
		d.flip(holder)
	}
}

// runBehind runs a frame of every machine but the front one, which the
// program's loop runs itself, and lets their disks commit as its does.
func (d *machineDeck) runBehind(frameNo, debounceFrames int) {
	for i, m := range d.machines {
		if i != d.front {
			// The turbo-aware frame, as for the front machine: a tape loading
			// in turbo mode goes on loading behind.
			m.zx.RunTurboAwareFrame(false)
			if w := m.zx.io.FDCTick(frameNo, debounceFrames); w != "" {
				m.zx.notice("%s", w)
			}
		}
		// A machine nobody has named takes the name of the first BASIC
		// program it loads, once.
		if !m.renamed && m.name == "" {
			m.name = m.zx.FirstProgram()
		}
	}
	d.followPlayerFocus()
}

// title is what the dock calls a machine: its name, or its model while it
// has none.
func (m *deckMachine) title() string {
	if m.name != "" {
		return m.name
	}
	return modelLabel(m.model)
}

// rename gives machine i the name the person typed. From then on the name
// is theirs. An empty name gives the machine back to naming itself.
func (d *machineDeck) rename(i int, name string) {
	if i < 0 || i >= len(d.machines) {
		return
	}
	m := d.machines[i]
	m.name, m.renamed = strings.TrimSpace(name), strings.TrimSpace(name) != ""
}

// usesMainTile reports that the layout gives the front machine a place of
// its own: the main tile, or the whole window.
func (d *machineDeck) usesMainTile() bool {
	return d.tiling == layoutSingle || d.tiling == layoutMainRight || d.tiling == layoutMainLeft
}

// rotate moves every machine to the next place in the layout. Where the
// front machine has a place of its own, that brings the next machine to the
// front; in the plain grids the machines change tiles and the same one
// stays in front.
func (d *machineDeck) rotate() {
	n := len(d.machines)
	if n < 2 {
		return
	}
	if d.usesMainTile() {
		d.flip((d.front + 1) % n)
		return
	}
	last := d.machines[n-1]
	copy(d.machines[1:], d.machines[:n-1])
	d.machines[0] = last
	d.front = (d.front + 1) % n
	d.sel = d.front
}

// place puts machine i where machine j is, and j where i was: what dropping
// a machine on a tile does. Where the front machine has a place of its own,
// taking it is coming to the front; in the plain grids the two change tiles
// and whichever was in front stays so.
func (d *machineDeck) place(i, j int) {
	n := len(d.machines)
	if i == j || i < 0 || j < 0 || i >= n || j >= n {
		return
	}
	if d.tiling == layoutSingle {
		d.flip(i) // the one tile there is: the machine comes to the front
		return
	}
	if d.usesMainTile() {
		switch d.front {
		case i:
			d.flip(j)
		case j:
			d.flip(i)
		}
	}
	inFront := d.machines[d.front]
	d.machines[i], d.machines[j] = d.machines[j], d.machines[i]
	for k, m := range d.machines {
		if m == inFront {
			d.front = k
		}
	}
	d.sel = d.front
}

// dropSite is the tile under the pointer, -1 when it is over none: every
// tile takes a dropped machine, and with one machine filling the window
// that is the window.
func (d *machineDeck) dropSite(x, y float32, dockTop float32) int {
	if y >= dockTop || y < float32(d.dm.reservedTopHeight) {
		return -1
	}
	if !d.tiled() {
		return d.front
	}
	for i, cell := range d.tileRects() {
		if rl.CheckCollisionPointRec(rl.Vector2{X: x, Y: y}, cell) {
			return i
		}
	}
	return -1
}

// nameField is where machine i's name is typed, under its picture.
func (d *machineDeck) nameField(l dockLayout, i int) zenui.Rect {
	t := l.tiles[i]
	w := int(t.Width)
	if w < 16*8 {
		w = 16 * 8
	}
	return zenui.Rect{X: int(t.X + t.Width/2 - float32(w)/2), Y: int(t.Y+t.Height) + 2, W: w, H: l.lineH + l.lineH/2}
}

// startRename opens the name of machine i for typing.
func (d *machineDeck) startRename(i int, l dockLayout, textW func(string) int) {
	if i < 0 || i >= len(d.machines) {
		return
	}
	f := zenui.NewTextField()
	f.Text, f.MaxLen = d.machines[i].name, 16
	area := d.nameField(l, i)
	// A field takes the keyboard when it is clicked: click it, at its end.
	f.Update(zenui.Input{MouseX: area.X + area.W - 2, MouseY: area.Y + area.H/2, MousePressed: true}, area, l.lineH, textW)
	d.edit, d.editing = f, i
}

// frontModel and setFrontModel are the front machine's model as the dock
// labels it; the Machine menu changes it.
func (d *machineDeck) frontModel() string {
	if len(d.machines) == 0 {
		return d.idleModel
	}
	return d.machines[d.front].model
}

func (d *machineDeck) setFrontModel(m string) {
	if len(d.machines) > 0 {
		d.machines[d.front].model = m
	}
}

// cardUse tells which machine of the deck has the card image at path, and
// in which slot; the zero value when none has.
func (d *machineDeck) cardUse(path string) cardUse {
	for i, m := range d.machines {
		if m.zx.next != nil && same(m.zx.next.files.Card, path) {
			return cardUse{machine: i + 1, slot: 1}
		}
		if second := m.zx.SecondCard(); second != "" && same(second, path) {
			return cardUse{machine: i + 1, slot: 2}
		}
	}
	return cardUse{}
}

// diskUse tells which machine of the deck has the disk file at path in its
// drive, 0 for none.
func (d *machineDeck) diskUse(path string) int {
	for i, m := range d.machines {
		if fdc := m.zx.io.fdc; fdc != nil && fdc.DiskFile() != "" && same(fdc.DiskFile(), path) {
			return i + 1
		}
	}
	return 0
}

// machineAt is the machine with number n in the dock, nil for none.
func (d *machineDeck) machineAt(n int) *ZenZX {
	if n < 1 || n > len(d.machines) {
		return nil
	}
	return d.machines[n-1].zx
}

// numberOf is a machine's number as the dock writes it, 0 for a machine
// that is not in the deck.
func (d *machineDeck) numberOf(zx *ZenZX) int {
	for i, m := range d.machines {
		if m.zx == zx {
			return i + 1
		}
	}
	return 0
}

// closeAll commits every machine's disk changes, at the program's end.
func (d *machineDeck) closeAll() {
	for _, m := range d.machines {
		saveDiskOnClose(m.zx)
	}
}

// refreshThumb draws machine m's picture as it is now into its texture.
func (m *deckMachine) refreshThumb() {
	img := m.zx.DecodeDisplay()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w != m.thumbW || h != m.thumbH {
		if m.thumbW != 0 {
			rl.UnloadTexture(m.thumb)
		}
		m.thumb = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
		rl.SetTextureFilter(m.thumb, rl.FilterBilinear)
		m.thumbW, m.thumbH = w, h
		m.pixels = make([]color.RGBA, w*h)
		return
	}
	for i := range m.pixels {
		p := img.Pix[i*4 : i*4+4 : i*4+4]
		m.pixels[i] = color.RGBA{R: p[0], G: p[1], B: p[2], A: p[3]}
	}
	rl.UpdateTexture(m.thumb, m.pixels)
}

// dockAllowed and multiAllowed are the Preferences' say on the dock and on adding machines.
func (d *machineDeck) dockAllowed() bool  { return d.dockOn == nil || d.dockOn() }
func (d *machineDeck) multiAllowed() bool { return d.multiOn == nil || d.multiOn() }

func (d *machineDeck) show(byKey bool) {
	if d.state == barShown || d.state == barSlidingIn || !d.dockAllowed() {
		return
	}
	d.state = barSlidingIn
	d.stateChangedAt = time.Now()
	d.byKey = byKey
	d.sel = d.front
	d.addMag, d.addDown = 1, false
	for _, m := range d.machines {
		m.mag = 1
		m.refreshThumb()
	}
}

func (d *machineDeck) hide() {
	if d.state == barHidden || d.state == barSlidingOut {
		return
	}
	d.state = barSlidingOut
	d.stateChangedAt = time.Now()
	d.edgeArmed = false
}

// dockLayout is where the dock and its tiles are in the window.
type dockLayout struct {
	panel rl.Rectangle
	tiles []rl.Rectangle // at rest; the last is the "+" tile
	lineH int
	text  int // the labels' text scale
}

// layout places the dock for a window of the given size. lineH is the height
// of a line of text at scale 1; the labels are written at twice that once
// the pictures are large enough to carry it.
func (d *machineDeck) layout(screenW, screenH, lineH int) dockLayout {
	tileH := float32(screenH) / 6
	if tileH < dockTileMin {
		tileH = dockTileMin
	} else if tileH > dockTileMax {
		tileH = dockTileMax
	}
	widths := func() (ws []float32, total float32) {
		for _, m := range d.machines {
			w := tileH * 4 / 3
			if m.thumbH != 0 {
				w = tileH * float32(m.thumbW) / float32(m.thumbH)
				if w > tileH*2 {
					w = tileH * 2
				}
			}
			ws = append(ws, w)
			total += w + dockTileGap
		}
		ws = append(ws, tileH)
		total += tileH + 2*dockPad
		return ws, total
	}
	ws, total := widths()
	if avail := float32(screenW); total > avail && avail > 0 {
		// Only the pictures shrink: the gaps between them and the padding
		// stay as they are, so they are taken out of both sides before the
		// pictures are scaled to what is left.
		fixed := float32(len(d.machines))*dockTileGap + 2*dockPad
		if avail > fixed {
			tileH *= (avail - fixed) / (total - fixed)
		}
		if tileH < 24 {
			tileH = 24
		}
		ws, total = widths()
	}
	text := 1
	if tileH >= 110 {
		text = 2
	}
	lineH *= text
	dockH := float32(2*dockPad+lineH+4) + tileH
	l := dockLayout{lineH: lineH, text: text}
	// Room at the sides for the first and last pictures to grow into.
	side := ws[0] * dockMagnify / 2
	if total+2*side > float32(screenW) {
		side = 0
	}
	l.panel = rl.Rectangle{
		X:      (float32(screenW)-total)/2 - side,
		Y:      float32(screenH) - (dockH+dockFloat)*d.progress,
		Width:  total + 2*side,
		Height: dockH,
	}
	x := l.panel.X + side + dockPad
	for _, w := range ws {
		l.tiles = append(l.tiles, rl.Rectangle{X: x, Y: l.panel.Y + dockPad, Width: w, Height: tileH})
		x += w + dockTileGap
	}
	return l
}

// grown is tile r magnified by mag, standing on the same spot.
func grown(r rl.Rectangle, mag float32) rl.Rectangle {
	w, h := r.Width*mag, r.Height*mag
	return rl.Rectangle{X: r.X + (r.Width-w)/2, Y: r.Y + r.Height - h, Width: w, Height: h}
}

// screenRect is where machine i's picture is when it is the front one, so
// that a picture flying out of the dock lands on its own place, in the window: where the
// window is a size the system gave it (maximised, full screen) the picture is drawn into a
// canvas that is put in the middle of the window, and the place is the canvas's, moved and
// scaled with it (inWindow).
func (d *machineDeck) screenRect(i int) rl.Rectangle {
	m := d.machines[i]
	if d.tiled() {
		// Its tile as it will be once it is in front, which in the layouts
		// with a main tile is the main one.
		return pictureIn(d.tileRectsWithFront(i)[i], m.thumbW, m.thumbH)
	}
	top := float32(d.dm.reservedTopHeight)
	if m.thumbW == 256 && m.thumbH == 192 && d.dm.borderView().Shown {
		x, y := d.dm.classicScreenOffset()
		mult := float32(d.dm.screen.multiplier)
		return d.dm.inWindow(rl.Rectangle{X: float32(x), Y: float32(y), Width: 256 * mult, Height: 192 * mult})
	}
	return d.dm.inWindow(rl.Rectangle{X: 0, Y: top, Width: float32(d.dm.currentWidth), Height: float32(d.dm.currentHeight) - top})
}

// choose acts on tile i: a machine flies to the front, the "+" tile adds one
// and sends it to the front.
func (d *machineDeck) choose(i int, l dockLayout) {
	if i == len(d.machines) {
		if len(d.machines) == 0 { // none is running: New starts the one that fills the window
			d.add(d.modelForNew())
			d.hide()
			return
		}
		if !d.multiAllowed() {
			sayf("Machines: adding machines is off (File > Preferences...)\n")
			d.machines[d.front].zx.notice("Adding machines is off: File > Preferences... turns it on")
			return
		}
		at := l.tiles[i]
		i = d.add(d.modelForNew())
		d.machines[i].refreshThumb()
		d.startFly(i, at)
		return
	}
	if i == d.front {
		d.hide()
		return
	}
	d.startFly(i, grown(l.tiles[i], d.machines[i].mag))
}

func (d *machineDeck) startFly(i int, from rl.Rectangle) {
	d.flying, d.flyTo, d.flyFrom, d.flyStart = true, i, from, time.Now()
	d.hide()
}

// Update runs the dock for one frame: it opens at the bottom edge or with
// Alt+M, takes the arrow keys, Enter, Delete and Escape and the pointer while
// it is open, and carries a chosen picture to the screen. A press on a
// picture that moves before it is let go drags the machine to a tile; F2, or
// a click on a name, renames. busy says something else (the menu bar, a
// window) has the keyboard and the pointer.
func (d *machineDeck) Update(screenW, screenH int, r zenui.Renderer, busy bool) {
	d.frame++
	d.tickStomp(time.Now())
	d.addHover = false
	wantHand := false
	defer func() { setHandCursor(&d.addHand, wantHand) }()
	if !d.dockAllowed() {
		d.hide() // turned off in Preferences while it was open
	}
	lineH := r.LineHeight(1)
	mx, my := rl.GetMouseX(), rl.GetMouseY()
	moved := mx != d.lastMouseX || my != d.lastMouseY
	alt := rl.IsKeyDown(rl.KeyLeftAlt) || rl.IsKeyDown(rl.KeyRightAlt)

	if d.flying {
		if time.Since(d.flyStart) >= dockFlyTime {
			d.flip(d.flyTo)
			d.flying = false
		}
	} else if !busy {
		// A click on a tile brings its machine to the front.
		if d.tiled() && d.state == barHidden && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			for i, cell := range d.tileRects() {
				if i != d.front && rl.CheckCollisionPointRec(rl.Vector2{X: float32(mx), Y: float32(my)}, cell) {
					d.flip(i)
					break
				}
			}
		}
		// Alt+1..9 brings that machine to the front without the mouse. The
		// chord avoids OS shortcuts (macOS Spaces/Cmd, common Linux desktops)
		// and the digits never reach the emulated keyboard, which is skipped
		// while Alt is held.
		if alt {
			for n, key := range deckFocusKeys {
				if rl.IsKeyPressed(key) || rl.IsKeyPressed(deckFocusKeysKP[n]) {
					d.flip(n)
					break
				}
			}
		}
		if alt && rl.IsKeyPressed(rl.KeyM) {
			if d.state == barHidden || d.state == barSlidingOut {
				d.show(true)
			} else {
				d.hide()
			}
		}
		if d.state == barHidden {
			if !moved && !rl.IsMouseButtonDown(rl.MouseLeftButton) {
				if !d.idling {
					d.idling, d.idleSince = true, time.Now()
				}
			} else {
				d.idling = false
			}
			atEdge := int(my) >= screenH-1-menuBarEdgeDistance && int(my) < screenH
			if !atEdge {
				d.edgeArmed = true
			}
			if atEdge && d.edgeArmed && d.idling && time.Since(d.idleSince) >= menuBarIdleDelay {
				d.show(false)
			}
		}
	}
	d.lastMouseX, d.lastMouseY = mx, my

	switch d.state {
	case barSlidingIn:
		t := float32(time.Since(d.stateChangedAt)) / float32(menuBarSlideTime)
		if t >= 1 {
			d.progress, d.state = 1, barShown
			d.revealArrived(time.Now())
		} else {
			d.progress = easeInOutCubic(t)
		}
	case barSlidingOut:
		t := float32(time.Since(d.stateChangedAt)) / float32(menuBarSlideTime)
		if t >= 1 {
			d.progress, d.state = 0, barHidden
		} else {
			d.progress = 1 - easeInOutCubic(t)
		}
	}
	if d.state == barHidden {
		d.revealTick(-1, time.Now()) // the chevron stays at the edge while its time lasts
		return
	}

	if d.frame%dockThumbEvery == 0 {
		for _, m := range d.machines {
			m.refreshThumb()
		}
	}

	l := d.layout(screenW, screenH, lineH)
	fmx, fmy := float32(mx), float32(my)
	near := fmy >= l.panel.Y-dockHideMargin
	hover := -1
	for i, r := range l.tiles {
		target := float32(1)
		if near && d.state == barShown && !d.flying {
			if dist := absf(fmx - (r.X + r.Width/2)); dist < r.Width*1.5 {
				f := 1 - dist/(r.Width*1.5)
				target = 1 + dockMagnify*f*f*(3-2*f)
			}
			if fmx >= r.X && fmx < r.X+r.Width && fmy >= l.panel.Y {
				hover = i
			}
		}
		if i < len(d.machines) {
			m := d.machines[i]
			m.mag += (target - m.mag) * dockMagnifySpeed
		} else {
			if !d.multiAllowed() {
				target = 1 // off in Preferences: it does not rise to the pointer
			}
			d.addMag = d.addMagnify() + (target-d.addMagnify())*dockMagnifySpeed
		}
	}
	d.addHover = hover == len(d.machines)
	wantHand = d.addCursorWanted(hover, busy)
	d.revealTick(hover, time.Now())
	if d.state != barShown || d.flying {
		return
	}
	// A name being typed has the keyboard, and the pointer too: a click
	// elsewhere ends the edit and does nothing else.
	if d.edit != nil {
		textW := func(s string) int { return r.TextWidth(s, l.text) }
		switch d.edit.Update(zenuiraylib.Input(), d.nameField(l, d.editing), l.lineH, textW) {
		case zenui.TextCommitted:
			d.rename(d.editing, d.edit.Text)
			d.edit, d.editing = nil, -1
		case zenui.TextCancelled:
			d.edit, d.editing = nil, -1
		}
		return
	}

	// A drag in progress: the picture follows the pointer until the button
	// comes up, over a tile or not.
	if d.pressOn >= 0 {
		if !d.dragging && (absf(float32(mx-d.pressX)) > 6 || absf(float32(my-d.pressY)) > 6) {
			d.dragging = true
		}
		d.dragX, d.dragY = fmx, fmy
		if rl.IsMouseButtonDown(rl.MouseLeftButton) {
			return
		}
		on, dragged := d.pressOn, d.dragging
		d.pressOn, d.dragging = -1, false
		if on >= len(d.machines) {
			return
		}
		if !dragged {
			d.choose(on, l)
			return
		}
		if site := d.dropSite(fmx, fmy, l.panel.Y); site >= 0 {
			d.place(on, site)
			d.hide()
		}
		return
	}

	// A press on New is held until the button comes up: let go on it, it adds a machine;
	// let go anywhere else, it does nothing.
	if d.addDown {
		if !rl.IsMouseButtonDown(rl.MouseLeftButton) {
			d.addDown = false
			if hover == len(d.machines) {
				d.choose(hover, l)
			}
		}
		return
	}

	if hover >= 0 && (moved || rl.IsMouseButtonPressed(rl.MouseLeftButton)) {
		d.sel = hover
	}

	n := len(d.machines) + 1
	switch {
	case rl.IsKeyPressed(rl.KeyF2):
		d.startRename(d.sel, l, func(s string) int { return r.TextWidth(s, l.text) })
	case rl.IsKeyPressed(rl.KeyRight):
		d.sel = (d.sel + 1) % n
	case rl.IsKeyPressed(rl.KeyLeft):
		d.sel = (d.sel + n - 1) % n
	case rl.IsKeyPressed(rl.KeyEnter), rl.IsKeyPressed(rl.KeySpace):
		d.choose(d.sel, l)
	case rl.IsKeyPressed(rl.KeyDelete), rl.IsKeyPressed(rl.KeyBackspace):
		if d.sel < len(d.machines) {
			d.stop(d.sel)
		}
	case rl.IsKeyPressed(rl.KeyEscape):
		d.hide()
	case rl.IsMouseButtonPressed(rl.MouseLeftButton):
		named := -1
		for i := range d.machines {
			if d.nameField(l, i).Contains(int(mx), int(my)) {
				named = i
			}
		}
		switch {
		case hover == len(d.machines):
			d.addDown = true // New: acts when let go on it
		case hover >= 0 && fmy < l.tiles[hover].Y+l.tiles[hover].Height:
			d.pressOn, d.pressX, d.pressY, d.dragX, d.dragY = hover, mx, my, fmx, fmy
		case named >= 0:
			d.sel = named
			d.startRename(named, l, func(s string) int { return r.TextWidth(s, l.text) })
		case fmy < l.panel.Y:
			d.hide()
		}
	case !d.byKey && !near && !d.revealHolds(time.Now()):
		d.hide()
	}
}

func absf(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func rlColour(c zenui.Colour) rl.Color { return rl.Color{R: c.R, G: c.G, B: c.B, A: c.A} }

func toRect(r rl.Rectangle) zenui.Rect {
	return zenui.Rect{X: int(r.X), Y: int(r.Y), W: int(r.Width), H: int(r.Height)}
}

// Draw draws the dock and a picture on its way to the screen. It is called
// inside the display's frame, after the machine's picture.
func (d *machineDeck) Draw(r zenui.Renderer, screenW, screenH int, theme zenui.Theme) {
	defer d.drawStomp()
	d.theme = theme
	if d.state != barHidden {
		l := d.layout(screenW, screenH, r.LineHeight(1))
		d.drawPanel(l.panel, theme)

		// The pictures, the one under the pointer last so that it lies over
		// its neighbours.
		// The New button is one of them: it grows as they do, and lies over or under its
		// neighbour by the same rule, instead of always over.
		mag := func(i int) float32 {
			if i == len(d.machines) {
				return d.addMagnify()
			}
			return d.machines[i].mag
		}
		order := make([]int, 0, len(d.machines)+1)
		top := -1
		for i := 0; i <= len(d.machines); i++ {
			if d.flying && i == d.flyTo {
				continue
			}
			if top < 0 || mag(i) > mag(top) {
				if top >= 0 {
					order = append(order, top)
				}
				top = i
			} else {
				order = append(order, i)
			}
		}
		if top >= 0 {
			order = append(order, top)
		}
		for _, i := range order {
			if i == len(d.machines) {
				d.drawAddTile(r, grown(l.tiles[i], d.addMagnify()), d.sel == i, theme)
				continue
			}
			m := d.machines[i]
			at := grown(l.tiles[i], m.mag)
			d.drawPicture(m, at, pictureRadius(at))
			edge, thick := theme.Border, 1
			if i == d.sel {
				edge, thick = theme.SelFill, 2
			}
			d.roundFrame(at, pictureRadius(at), edge, thick)
		}
		for i, m := range d.machines {
			t := l.tiles[i]
			if i == d.editing && d.edit != nil {
				continue // its name is being typed: the field is drawn below
			}
			label := fmt.Sprintf("%d %s", i+1, m.title())
			for len(label) > 1 && r.TextWidth(label, l.text) > int(t.Width) {
				label = label[:len(label)-1]
			}
			lx := int(t.X + (t.Width-float32(r.TextWidth(label, l.text)))/2)
			ly := int(t.Y + t.Height + 4)
			if i == d.front {
				d.roundFill(rl.Rectangle{X: t.X, Y: float32(ly - 2), Width: t.Width, Height: float32(l.lineH + 4)}, float32(l.lineH+4)/2, theme.SelFill)
				r.DrawText(label, lx, ly, l.text, theme.SelText)
			} else {
				r.DrawText(label, lx, ly, l.text, theme.Text)
			}
		}

		if d.edit != nil && d.editing < len(d.machines) {
			area := d.nameField(l, d.editing)
			r.FillRect(area, theme.Panel)
			d.edit.Draw(r, theme, area, l.text)
		}

		// The New button's name.
		p := l.tiles[len(d.machines)]
		label, ink := "New", theme.DimText
		switch d.addState() {
		case addHover, addPressed:
			ink = theme.Text
		case addOff:
			ink = theme.Disabled
		}
		r.DrawText(label, int(p.X+(p.Width-float32(r.TextWidth(label, l.text)))/2), int(p.Y+p.Height+4), l.text, ink)

	}

	// The chevron of a machine made for a snapshot: over the dock, or at the bottom of the window
	// when the dock has gone.
	if d.reveal.on {
		d.drawReveal(d.layout(screenW, screenH, r.LineHeight(1)), screenH, time.Now())
	}

	// A machine being dragged: its picture under the pointer, and a frame
	// round the tile it would be dropped on.
	if d.dragging && d.pressOn >= 0 && d.pressOn < len(d.machines) {
		dockTop := float32(screenH)
		if d.state != barHidden {
			dockTop = d.layout(screenW, screenH, r.LineHeight(1)).panel.Y
		}
		if site := d.dropSite(d.dragX, d.dragY, dockTop); site >= 0 {
			cell := rl.Rectangle{X: 0, Y: float32(d.dm.reservedTopHeight), Width: float32(d.dm.currentWidth), Height: float32(d.dm.currentHeight - d.dm.reservedTopHeight)}
			if d.tiled() {
				cell = d.tileRects()[site]
			}
			r.StrokeRect(toRect(cell), theme.SelFill, 4)
		}
		m := d.machines[d.pressOn]
		h := float32(dockTileMin) * 1.4
		w := h * 4 / 3
		if m.thumbH != 0 {
			w = h * float32(m.thumbW) / float32(m.thumbH)
		}
		at := rl.Rectangle{X: d.dragX - w/2, Y: d.dragY - h/2, Width: w, Height: h}
		d.drawPicture(m, at, pictureRadius(at))
		d.roundFrame(at, pictureRadius(at), theme.SelFill, 2)
	}

	if d.flying {
		t := float32(time.Since(d.flyStart)) / float32(dockFlyTime)
		if t > 1 {
			t = 1
		}
		e := easeInOutCubic(t)
		to := d.screenRect(d.flyTo)
		at := rl.Rectangle{
			X:      d.flyFrom.X + (to.X-d.flyFrom.X)*e,
			Y:      d.flyFrom.Y + (to.Y-d.flyFrom.Y)*e,
			Width:  d.flyFrom.Width + (to.Width-d.flyFrom.Width)*e,
			Height: d.flyFrom.Height + (to.Height-d.flyFrom.Height)*e,
		}
		m := d.machines[d.flyTo]
		if d.frame%2 == 0 {
			m.refreshThumb()
		}
		// The corners square off as the picture lands on the screen.
		radius := pictureRadius(d.flyFrom) * (1 - e)
		d.drawPicture(m, at, radius)
		d.roundFrame(at, radius, theme.Border, 1)
	}
}

// Unload frees the dock's pictures.
func (d *machineDeck) Unload() {
	for _, m := range d.machines {
		if m.thumbW != 0 {
			rl.UnloadTexture(m.thumb)
			m.thumbW, m.thumbH = 0, 0
		}
	}
	if d.roundCache != nil {
		d.roundCache.Unload()
		d.round, d.roundCache = nil, nil
	}
	d.textures.unload()
	d.textures = nil
}

// ----------------------------------------------------------------------------
// Tiled layouts
//
// In a tiled layout every machine is on the screen at once, each in a cell
// the size of a machine's whole frame. The window takes the shape of the
// grid, on its way there as it is after a change of zoom. The zoom itself is
// for one machine filling the window: tiles are drawn at the largest whole
// scale, up to that zoom, at which the grid fits the monitor.
// ----------------------------------------------------------------------------

// deckLayout is how the window shows its machines.
type deckLayout int

const (
	layoutSingle    deckLayout = iota // the front machine fills the window
	layoutColumns                     // side by side
	layoutRows                        // one above the other
	layoutGrid                        // rows and columns, as square as the number allows
	layoutMainRight                   // the others stacked on the left, the front machine large on the right
	layoutMainLeft                    // the front machine large on the left, the others stacked on the right
)

// deckLayoutNames are the layouts as the View menu names them.
var deckLayoutNames = []string{"One Machine", "Side by Side", "Stacked", "Grid", "Stack Left, Main Right", "Main Left, Stack Right"}

// layoutByName returns the deckLayout whose View-menu name is name (one of
// deckLayoutNames, as parseLayout returns), and whether there was one.
func layoutByName(name string) (deckLayout, bool) {
	for i, n := range deckLayoutNames {
		if n == name {
			return deckLayout(i), true
		}
	}
	return layoutSingle, false
}

// tileGrid is the grid a layout makes of n machines.
func tileGrid(l deckLayout, n int) (cols, rows int) {
	switch l {
	case layoutColumns:
		return n, 1
	case layoutRows:
		return 1, n
	case layoutGrid:
		cols = 1
		for cols*cols < n {
			cols++
		}
		return cols, (n + cols - 1) / cols
	}
	return 1, 1
}

// tileUnits is a tile's place in a layout, counted in cells: a cell is one
// machine's whole frame at the tiles' scale.
type tileUnits struct{ x, y, w, h int }

// tilePlan is where each of n machines goes in a layout, in cells, and how
// many cells the whole is across and down. Most layouts are a plain grid in
// the machines' order. The two with a main tile depend on which machine is
// in front: that one is the main tile, as tall as the stack of all the
// others beside it and as wide in proportion, so that with three machines
// it is twice the size of the two it stands beside.
func tilePlan(l deckLayout, n, front int) (across, down int, tiles []tileUnits) {
	tiles = make([]tileUnits, n)
	if l == layoutMainRight || l == layoutMainLeft {
		big := n - 1 // the main tile is big cells each way, the stack one cell wide
		stackX, mainX := 0, 1
		if l == layoutMainLeft {
			stackX, mainX = big, 0
		}
		row := 0
		for i := range tiles {
			if i == front {
				tiles[i] = tileUnits{mainX, 0, big, big}
				continue
			}
			tiles[i] = tileUnits{stackX, row, 1, 1}
			row++
		}
		return big + 1, big, tiles
	}
	cols, rows := tileGrid(l, n)
	for i := range tiles {
		tiles[i] = tileUnits{i % cols, i / cols, 1, 1}
	}
	return cols, rows, tiles
}

// layoutAllowed reports whether a layout means anything for n machines:
// tiles want two, and a grid or a main tile beside a stack differs from a
// row only from three on.
func layoutAllowed(l deckLayout, n int) bool {
	switch l {
	case layoutSingle:
		return true
	case layoutGrid, layoutMainRight, layoutMainLeft:
		return n >= 3
	}
	return n >= 2
}

// tileScale is the whole scale tiles are drawn at: the largest, up to the
// zoom, at which cols by rows cells fit a monitor of monW by monH with top
// pixels kept for the menu bar; 1 at the least, and the zoom when the
// monitor's size is not known.
func tileScale(cols, rows, zoom, monW, monH, top int) int {
	if zoom < 1 {
		zoom = 1
	}
	if monW <= 0 || monH <= 0 {
		return zoom
	}
	for s := zoom; s > 1; s-- {
		if cols*RefWidth*s <= monW && rows*RefHeight*s+top <= monH {
			return s
		}
	}
	return 1
}

// minTiledHeight is the least height of a tiled window below the menu bar's
// strip. A single row of tiles at scale 1 is 256 high, which is less than
// the View menu that chose it; the window is made this high and the tiles
// are centred in it.
const minTiledHeight = 336

// tiled reports that the window shows its machines in tiles.
func (d *machineDeck) tiled() bool { return d.tiling != layoutSingle }

// setLayout changes the layout. One that means nothing for the number of
// machines there are leaves the window as it is.
func (d *machineDeck) setLayout(l deckLayout) {
	if !layoutAllowed(l, len(d.machines)) {
		return
	}
	d.tiling = l
	d.applyLayout()
}

// applyLayout sizes the window for the layout and the machines there are
// now; a layout the number no longer allows falls back to one machine.
func (d *machineDeck) applyLayout() {
	if !layoutAllowed(d.tiling, len(d.machines)) {
		d.tiling = layoutSingle
	}
	if !d.tiled() {
		if d.dm.tiles != nil {
			d.dm.setTiles(0, 0, nil)
		}
		return
	}
	cols, rows, _ := tilePlan(d.tiling, len(d.machines), d.front)
	monW := rl.GetMonitorWidth(rl.GetCurrentMonitor())
	monH := rl.GetMonitorHeight(rl.GetCurrentMonitor())
	s := tileScale(cols, rows, d.dm.screen.multiplier, monW, monH, d.dm.reservedTopHeight)
	w, h := cols*RefWidth*s, rows*RefHeight*s
	if h < minTiledHeight {
		h = minTiledHeight // room for the menus; the tiles sit in the middle
	}
	d.dm.setTiles(w, h, d.drawTiles)
	keepOnMonitor(w, h+d.dm.reservedTopHeight, monW, monH)
}

// HandleWindowResize checks for a user window-drag this frame and, in a tiled
// layout, snaps the window back to the layout's proportions. A no-op for a
// single-machine view, whose own size is re-asserted by the display.
func (d *machineDeck) HandleWindowResize() {
	if d == nil || !d.tiled() || d.dm.fullscreen || !rl.IsWindowResized() { // a full screen window is not a drag
		return
	}
	osW, osH := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
	// Animate the snap from where the window actually is after the drag, not
	// from the stale model size.
	d.dm.syncCurrentToWindow(osW, osH)
	d.snapTiledResize(osW, osH)
}

// snapTiledResize responds to the user dragging the window edge while a tiled
// layout is shown. A tiled layout fixes proportions, not a size: its tile area
// has the aspect (cols*RefWidth):(rows*RefHeight) -- wide for columns, tall for
// a stack -- so a free drag is snapped back to that aspect. The dimension the
// user drove most is kept and the other is derived from the aspect; the top
// strip reserved for the menus is a fixed band that does not scale. The snapped
// size is clamped to a sensible minimum and to the monitor. A no-op when not
// tiled (a single view re-asserts its own size elsewhere).
func (d *machineDeck) snapTiledResize(osW, osH int) {
	if !d.tiled() {
		return
	}
	cols, rows, _ := tilePlan(d.tiling, len(d.machines), d.front)
	top := d.dm.reservedTopHeight

	// The tile area is the window minus the fixed menu band. Aspect is the grid
	// of machine-proportioned cells.
	areaAspect := float64(cols*RefWidth) / float64(rows*RefHeight)

	dragH := osH - top
	if dragH < 1 {
		dragH = 1
	}
	dragW := osW

	// Keep the dimension the user drove further from the current tile area, and
	// derive the other from the aspect, so the snap follows the intended drag.
	curW, curH := d.dm.tilesW, d.dm.tilesH
	var w, h int
	if abs(dragW-curW) >= abs(dragH-curH) {
		w = dragW
		h = int(float64(w)/areaAspect + 0.5)
	} else {
		h = dragH
		w = int(float64(h)*areaAspect + 0.5)
	}

	// Aspect is king: both floor and monitor clamp keep (cols*RefWidth):
	// (rows*RefHeight) exactly, deriving one dimension from the other rather
	// than clamping each independently (which would distort the tiles). The
	// floor is one machine-cell per tile, so the grid never collapses; a wide
	// columns layout is allowed to be short, a tall stack allowed to be narrow.
	if w < cols*RefWidth || h < rows*RefHeight {
		w, h = cols*RefWidth, rows*RefHeight
	}

	// Clamp to the monitor (tile area plus the fixed menu band must fit), again
	// re-deriving the paired dimension so the aspect is preserved.
	monW := rl.GetMonitorWidth(rl.GetCurrentMonitor())
	monH := rl.GetMonitorHeight(rl.GetCurrentMonitor())
	if monW > 0 && monH > 0 {
		if w > monW {
			w = monW
			h = int(float64(w)/areaAspect + 0.5)
		}
		if h+top > monH {
			h = monH - top
			w = int(float64(h)*areaAspect + 0.5)
		}
	}

	d.dm.setTiles(w, h, d.drawTiles)
	keepOnMonitor(w, h+top, monW, monH)
}

// keepOnMonitor moves the window up and to the left, as far as it must, for
// a window of w by h to end inside its monitor: a tiled window is wider or
// taller than the one it grew from, and would otherwise run off the edge.
func keepOnMonitor(w, h, monW, monH int) {
	if monW <= 0 || monH <= 0 || !rl.IsWindowReady() || rl.IsWindowState(rl.FlagBorderlessWindowedMode) {
		return // nor is a full screen window moved: it is the monitor's
	}
	origin := rl.GetMonitorPosition(rl.GetCurrentMonitor())
	pos := rl.GetWindowPosition()
	x, y := int(pos.X), int(pos.Y)
	nx, ny := fitOnAxis(x, w, int(origin.X), monW), fitOnAxis(y, h, int(origin.Y), monH)
	if nx != x || ny != y {
		rl.SetWindowPosition(nx, ny)
	}
}

// fitOnAxis is where a span of size starting at pos must start to end
// inside a monitor that runs from origin for length: where it is if it
// fits, else as little to the left as brings its end in, and never before
// the origin.
func fitOnAxis(pos, size, origin, length int) int {
	if over := pos + size - (origin + length); over > 0 {
		pos -= over
	}
	if pos < origin {
		pos = origin
	}
	return pos
}

// tileRects are the machines' cells in the window as it is now, which while
// the window is on its way to a new shape is not yet their final size.
func (d *machineDeck) tileRects() []rl.Rectangle { return d.tileRectsWithFront(d.front) }

// tileRectsWithFront is tileRects as it would be with machine front in
// front: in the layouts with a main tile that decides who has it.
func (d *machineDeck) tileRectsWithFront(front int) []rl.Rectangle {
	cols, rows, tiles := tilePlan(d.tiling, len(d.machines), front)
	top := float32(d.dm.reservedTopHeight)
	availW, availH := float32(d.dm.currentWidth), float32(d.dm.currentHeight)-top
	// The tiles keep a machine's proportions: the largest such grid that
	// fits what there is of the window, in its middle.
	w, h := availW/float32(cols), availH/float32(rows)
	if w*RefHeight > h*RefWidth {
		w = h * RefWidth / RefHeight
	} else {
		h = w * RefHeight / RefWidth
	}
	x0 := (availW - w*float32(cols)) / 2
	y0 := top + (availH-h*float32(rows))/2
	cells := make([]rl.Rectangle, len(tiles))
	for i, t := range tiles {
		cells[i] = rl.Rectangle{X: x0 + float32(t.x)*w, Y: y0 + float32(t.y)*h, Width: float32(t.w) * w, Height: float32(t.h) * h}
	}
	return cells
}

// pictureIn is where a picture of w by h goes in a cell: a classic machine's
// 256 by 192 in the middle with its border round it, a Next's whole frame
// over the whole cell (its double-width frames at the cell's width).
func pictureIn(cell rl.Rectangle, w, h int) rl.Rectangle {
	if w > RefWidth {
		w = RefWidth
	}
	if h > RefHeight {
		h = RefHeight
	}
	// A cell is a whole number of frames each way; the picture is scaled as
	// its cell is.
	sx, sy := cell.Width/RefWidth, cell.Height/RefHeight
	pw, ph := float32(w)*sx, float32(h)*sy
	return rl.Rectangle{X: cell.X + (cell.Width-pw)/2, Y: cell.Y + (cell.Height-ph)/2, Width: pw, Height: ph}
}

// drawTiles draws every machine in its cell. The display calls it in place
// of drawing the front machine.
func (d *machineDeck) drawTiles() {
	cells := d.tileRects()
	for i, m := range d.machines {
		if i != d.front {
			m.zx.screen.updateFlash() // the front machine's is advanced by the display
		}
		m.refreshThumb()
		cell := cells[i]
		b := zxPalette[m.zx.io.borderColor&7]
		rl.DrawRectangleRec(cell, rl.Color{R: b.R, G: b.G, B: b.B, A: 255})
		d.drawPictureSquare(m, pictureIn(cell, m.thumbW, m.thumbH))
	}
	// The front machine, the one the keyboard types into, has a frame.
	mark := rl.Color{R: 0, G: 255, B: 255, A: 255}
	if c := d.theme.SelFill; c.A != 0 {
		mark = rlColour(c)
	}
	rl.DrawRectangleLinesEx(cells[d.front], 2, mark)
}

// saveDiskOnClose commits a machine's unsaved disk changes to the file the
// disk came from, as the program does for its machine when it ends.
func saveDiskOnClose(zx *ZenZX) {
	if zx.io.hasFDC && zx.io.fdc != nil && zx.io.fdc.IsModified() && zx.io.fdc.diskFilename != "" {
		if err := zx.io.SaveDisk(); err != nil {
			sayf("Warning: failed to save disk on exit: %v\n", err)
			if zx.OnNotice != nil { // the window is still up when a machine is closed on its own
				zx.notice("The disk changes were not saved when the machine closed: %v", err)
			}
		} else {
			sayln("Disk changes saved on exit")
		}
	}
}
