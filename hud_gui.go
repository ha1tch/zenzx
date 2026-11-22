//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zenui"
	"github.com/ha1tch/zenui/zenuiraylib"
	"github.com/ha1tch/zenzx/pkg/debug"
	"github.com/ha1tch/zenzx/pkg/debugcmd"
	"github.com/ha1tch/zenzx/pkg/fonts"
)

// The debugger HUD: a command line drawn over the running machine, the same
// engine and grammar as the standalone zendbg CLI (pkg/debug + pkg/debugcmd),
// with a scrollback pane, a one-line input, and a row of pixel-art transport
// buttons in the dock icon's style. It is one surface over the shared engine,
// so a HUD transcript matches a CLI transcript (the workbench's idea 1, see
// docs/proposals/workbench.md).
//
// Following the demoOverlay pattern (gui_demo_overlay.go): the emulator keeps
// running every frame; only keyboard input to the machine is gated while the
// HUD owns the frame. The HUD draws with the BDF font system (fonts.SinclairNarrow,
// zenuiraylib.BDFText), never raylib's own font.

const (
	hudMaxScrollback = 200 // lines kept
	hudVisibleLines  = 16  // lines shown in the pane

	// The panel's height as a fraction of the screen: default, and the clamp
	// the resize gripper enforces.
	hudDefaultFrac = 0.50
	hudMinFrac     = 0.22
	hudMaxFrac     = 0.80

	// hudGripZone is how many device pixels above and below the panel's top
	// edge count as "hovering the gripper".
	hudGripZone = 10
)

// hudSession is one machine's own debugger state: its engine (breakpoints,
// watches, decode target) and its own scrollback. A machine keeps its session
// across HUD open/close and across dock flips, so each machine has its own
// command history and breakpoints -- switching to another machine shows that
// machine's buffer, not a shared one.
type hudSession struct {
	e      *debug.Engine
	scroll []string
}

// debugHUD is the overlay. It keeps one hudSession per machine and shows the
// session of whichever machine it is currently attached to. The host gives it
// a machine to attach to and toggles it with a key.
type debugHUD struct {
	active bool

	zx       *ZenZX                 // the machine currently shown
	sessions map[*ZenZX]*hudSession // per-machine state
	cur      *hudSession            // the attached machine's session
	caption  string                 // "Machine N · <title>", shown in the HUD

	text     *zenuiraylib.BDFText
	renderer zenuiraylib.Renderer
	theme    zenui.Theme

	input []rune // current input line (shared: only the front machine takes input)

	// transport button sprites, parsed once.
	sprites map[string][16]iconRow

	// Slide animation (matches the dock's easeInOutCubic over menuBarSlideTime).
	// anim is the eased 0..1 position: 0 fully hidden (off the bottom edge), 1
	// fully shown. opening says which way it is heading while in motion. The
	// HUD is drawn whenever anim > 0, so the close animation finishes on screen.
	anim      float32
	opening   bool
	animStart time.Time

	// Resize: the panel's height as a fraction of the screen, draggable by the
	// gripper on its top edge, clamped to [hudMinFrac, hudMaxFrac]. gripGlow is
	// the gripper's eased 0..1 appearance, fading in while the pointer is near
	// the top edge or a drag is in progress. resizing is true during a drag.
	heightFrac float32
	gripGlow   float32
	resizing   bool
}

// Engine is a convenience alias so the field name reads naturally; the real
// type is pkg/debug.Engine.
type Engine = debug.Engine

// hudMachineAdapter bridges this machine's DebugView to the engine's
// MachineView, the same shim cmd/zendbg carries, kept here so the GUI build
// needs no separate command binary.
type hudMachineAdapter struct{ v DebugView }

func (a hudMachineAdapter) Regs() debug.Regs {
	r := a.v.Regs()
	return debug.Regs{
		A: r.A, F: r.F, B: r.B, C: r.C, D: r.D, E: r.E, H: r.H, L: r.L,
		IXH: r.IXH, IXL: r.IXL, IYH: r.IYH, IYL: r.IYL,
		I: r.I, R: r.R, SP: r.SP, PC: r.PC, WZ: r.WZ,
		IFF1: r.IFF1, IFF2: r.IFF2, Halted: r.Halted, Cycles: r.Cycles,
	}
}
func (a hudMachineAdapter) PC() uint16                        { return a.v.PC() }
func (a hudMachineAdapter) SetPC(addr uint16)                 { a.v.SetPC(addr) }
func (a hudMachineAdapter) ReadMem(addr uint16, n int) []byte { return a.v.ReadMem(addr, n) }
func (a hudMachineAdapter) WriteMem(addr uint16, data []byte) { a.v.WriteMem(addr, data) }
func (a hudMachineAdapter) Step() int                         { return a.v.Step() }
func (a hudMachineAdapter) Halted() bool                      { return a.v.Halted() }
func (a hudMachineAdapter) SetReg(name string, v uint16) bool {
	return a.v.SetReg(RegName(name), v)
}

// newDebugHUD builds an idle HUD attached to zx, loading the Sinclair face
// once (shared glyph cache). The PoC decodes as Z80; a Next passes Z80N once
// the facade exposes the model.
func newDebugHUD(zx *ZenZX) (*debugHUD, error) {
	face, err := fonts.SinclairNarrow()
	if err != nil {
		return nil, err
	}
	text := zenuiraylib.NewBDFText(face)
	h := &debugHUD{
		sessions:   make(map[*ZenZX]*hudSession),
		text:       text,
		renderer:   zenuiraylib.Renderer{Text: text},
		theme:      zenui.DefaultTheme(),
		sprites:    parseTransportSprites(),
		heightFrac: hudDefaultFrac,
	}
	// The initial machine, captioned plainly; the host re-captions with the
	// machine's dock number/title when it attaches in -multi.
	h.AttachMachine(zx, "")
	return h, nil
}

// sessionFor returns the machine's session, creating it (with its own engine
// and empty scrollback) the first time.
func (h *debugHUD) sessionFor(zx *ZenZX) *hudSession {
	if s, ok := h.sessions[zx]; ok {
		return s
	}
	s := &hudSession{e: debug.NewEngine(hudMachineAdapter{v: zx.Debug()}, disasm.TargetZ80)}
	h.sessions[zx] = s
	return s
}

// Active reports whether the HUD owns input and pauses the machine. It is true
// from the moment opening begins until closing begins -- i.e. it tracks intent,
// not the animation, so input and the pause engage immediately on open and
// release immediately on close.
func (h *debugHUD) Active() bool { return h != nil && h.active }

// Visible reports whether the HUD should be drawn this frame: true whenever any
// of it is on screen, including through the close animation after Active has
// gone false.
func (h *debugHUD) Visible() bool { return h != nil && (h.active || h.anim > 0) }

// Toggle opens the HUD if closed, or closes it if open, starting the matching
// slide animation.
func (h *debugHUD) Toggle() {
	if h == nil {
		return
	}
	if h.active {
		h.Close()
	} else {
		h.open()
	}
}

// open begins the show animation and engages input/pause at once.
func (h *debugHUD) open() {
	if h == nil || h.active {
		return
	}
	h.active = true
	h.opening = true
	h.animStart = time.Now()
}

// Close begins the hide animation; input/pause release at once, but the HUD
// keeps drawing until the slide-out finishes.
func (h *debugHUD) Close() {
	if h == nil || !h.active {
		return
	}
	h.active = false
	h.opening = false
	h.animStart = time.Now()
}

// UpdateAnim advances the slide animation one frame. Called by the host every
// frame while the HUD is visible, using the same curve and duration as the dock
// (easeInOutCubic over menuBarSlideTime). anim goes 0->1 opening, 1->0 closing.
func (h *debugHUD) UpdateAnim() {
	if h == nil {
		return
	}
	t := float32(time.Since(h.animStart)) / float32(menuBarSlideTime)
	if t >= 1 {
		if h.opening {
			h.anim = 1
		} else {
			h.anim = 0
		}
		return
	}
	if h.opening {
		h.anim = easeInOutCubic(t)
	} else {
		h.anim = 1 - easeInOutCubic(t)
	}
}

// panelTopY is the device-pixel Y of the panel's top edge at the current
// height fraction and slide position, for the given screen height.
func (h *debugHUD) panelTopY(screenH int) int {
	panelH := int(float32(screenH) * h.heightFrac)
	slide := int(float32(panelH) * (1 - h.anim))
	return screenH - panelH + slide
}

// UpdateResize handles the top-edge resize gripper: it fades the gripper in
// while the pointer is near the top edge, starts a drag on a press there, and
// while dragging sets the panel height from the pointer, clamped to the min and
// max. Called by the host each frame while the HUD owns input. It returns true
// when it is consuming the pointer (hovering the gripper or dragging), so the
// host can keep that click from reaching anything behind the HUD.
func (h *debugHUD) UpdateResize(screenW, screenH int) (usingPointer bool) {
	if h == nil || !h.active || h.anim < 1 {
		// Only resizable once fully open and settled.
		h.fadeGrip(false)
		return false
	}
	my := int(rl.GetMouseY())
	topY := h.panelTopY(screenH)
	near := my >= topY-hudGripZone && my <= topY+hudGripZone

	if h.resizing {
		// Drag in progress: the top edge follows the pointer; height is the
		// distance from there to the screen bottom, clamped.
		frac := float32(screenH-int(rl.GetMouseY())) / float32(screenH)
		if frac < hudMinFrac {
			frac = hudMinFrac
		}
		if frac > hudMaxFrac {
			frac = hudMaxFrac
		}
		h.heightFrac = frac
		if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
			h.resizing = false
		}
		h.fadeGrip(true)
		return true
	}

	if near && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
		h.resizing = true
		h.fadeGrip(true)
		return true
	}
	h.fadeGrip(near)
	return near
}

// fadeGrip eases gripGlow toward 1 when the gripper should show, toward 0 when
// it should not -- a short per-frame approach, so it fades rather than snaps.
// The step is deliberately gentle so the grow/fade of the gripper and its edge
// line reads as a smooth reveal; it was halved (0.12 -> 0.06) to make the show
// and hide animations 100% slower (twice as long).
func (h *debugHUD) fadeGrip(show bool) {
	const step = 0.06
	if show {
		h.gripGlow += step
		if h.gripGlow > 1 {
			h.gripGlow = 1
		}
	} else {
		h.gripGlow -= step
		if h.gripGlow < 0 {
			h.gripGlow = 0
		}
	}
}

// AttachMachine re-points the engine at a machine, so the HUD debugs whichever
// machine is in front. The breakpoints and watches of the previous engine are
// not carried over -- a different machine is a different address space -- so a
// fresh engine is built. Called by the host when opening the HUD in -multi,
// where the front machine can be any of the several.
func (h *debugHUD) AttachMachine(zx *ZenZX, caption string) {
	if h == nil || zx == nil {
		return
	}
	h.zx = zx
	h.cur = h.sessionFor(zx) // its own engine + scrollback, kept across flips
	if caption != "" {
		h.caption = caption
	} else if h.caption == "" {
		h.caption = "machine"
	}
}

// println appends a line (possibly multi-line) to the attached machine's own
// scrollback, trimming to the cap.
func (h *debugHUD) println(s string) {
	if h.cur == nil {
		return
	}
	for _, ln := range strings.Split(s, "\n") {
		h.cur.scroll = append(h.cur.scroll, ln)
	}
	if len(h.cur.scroll) > hudMaxScrollback {
		h.cur.scroll = h.cur.scroll[len(h.cur.scroll)-hudMaxScrollback:]
	}
}

// engine is the attached machine's engine.
func (h *debugHUD) engine() *debug.Engine { return h.cur.e }

// HandleInput consumes keystrokes while the HUD is active: typed characters go
// to the input line, Enter runs the command, Backspace edits, backtick closes.
// Called by the host only when Active().
func (h *debugHUD) HandleInput() {
	if !h.active {
		return
	}
	// Primary: raylib's character queue (honours layout, shift, dead keys).
	got := 0
	for {
		r := rl.GetCharPressed()
		if r == 0 {
			break
		}
		if r >= 0x20 && r != 0x7F { // printable only
			h.input = append(h.input, r)
			got++
		}
	}
	// Fallback: on some platforms/builds the character queue is not populated
	// (the HUD only ever saw an empty queue). When nothing came through it this
	// frame, map the freshly-pressed keys to characters ourselves, honouring
	// Shift. This guarantees typing works regardless of the char-queue path.
	if got == 0 {
		shift := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
		for _, ch := range pressedChars(shift) {
			h.input = append(h.input, ch)
		}
	}
	if rl.IsKeyPressed(rl.KeyBackspace) && len(h.input) > 0 {
		h.input = h.input[:len(h.input)-1]
	}
	if rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter) {
		h.submit()
	}
}

// pressedChars maps the raylib keys pressed this frame to characters, for the
// fallback input path. It covers what the debugger grammar needs: letters,
// digits, space, and the few symbols addresses and commands use ($ # . - ( )
// .. etc.). Shift selects the upper symbol where there is one.
func pressedChars(shift bool) []rune {
	var out []rune
	// Letters: always lower-case for commands (the grammar is case-insensitive
	// on verbs and hex is accepted either case).
	for k := int32(rl.KeyA); k <= int32(rl.KeyZ); k++ {
		if rl.IsKeyPressed(k) {
			out = append(out, rune('a'+(k-int32(rl.KeyA))))
		}
	}
	// Digits and their shifted symbols.
	digitShift := map[int32]rune{
		rl.KeyOne: '!', rl.KeyThree: '#', rl.KeyFour: '$', rl.KeyFive: '%',
		rl.KeyNine: '(', rl.KeyZero: ')',
	}
	for k := int32(rl.KeyZero); k <= int32(rl.KeyNine); k++ {
		if rl.IsKeyPressed(k) {
			if shift {
				if sym, ok := digitShift[k]; ok {
					out = append(out, sym)
					continue
				}
			}
			out = append(out, rune('0'+(k-int32(rl.KeyZero))))
		}
	}
	// Space and a few punctuation keys the grammar uses.
	if rl.IsKeyPressed(rl.KeySpace) {
		out = append(out, ' ')
	}
	if rl.IsKeyPressed(rl.KeyPeriod) {
		out = append(out, '.')
	}
	if rl.IsKeyPressed(rl.KeyMinus) {
		if shift {
			out = append(out, '_')
		} else {
			out = append(out, '-')
		}
	}
	return out
}

// submit runs the current input line through the shared command layer.
func (h *debugHUD) submit() {
	line := strings.TrimSpace(string(h.input))
	h.input = h.input[:0]
	if line == "" {
		return
	}
	h.println("> " + line)

	cmd, err := debugcmd.Parse(line)
	if err != nil {
		h.println(" error: " + err.Error())
		return
	}
	if cmd.Verb == debugcmd.VerbReset {
		h.zx.Reset()
		h.println(" machine reset")
		return
	}
	text, quit, err := debugcmd.Exec(h.engine(), cmd)
	if err != nil {
		h.println(" error: " + err.Error())
		return
	}
	if quit {
		h.active = false
		return
	}
	if text != "" {
		h.println(text)
	}
}

// fire runs a transport button's action directly (the buttons are shortcuts
// for the matching commands).
func (h *debugHUD) fire(name string) {
	switch name {
	case "play", "run":
		h.println("> run")
		if text, _, err := debugcmd.Exec(h.engine(), debugcmd.Command{Verb: debugcmd.VerbRun}); err == nil && text != "" {
			h.println(text)
		}
	case "pause":
		// Pausing is the HUD being open; nothing to run. Report state.
		h.println("> (paused)")
	case "step":
		if text, _, err := debugcmd.Exec(h.engine(), debugcmd.Command{Verb: debugcmd.VerbStep, Count: 1}); err == nil && text != "" {
			h.println(text)
		}
	case "stop":
		h.zx.Reset()
		h.println("> reset")
	}
}
