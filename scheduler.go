package main

// ============================================================================
// Action scheduler
//
// The scheduler drives a parsed .zen Script against a running emulator. It is
// front-end agnostic: both the GUI and headless main loops construct one and
// call Tick(zx) exactly once per frame, immediately before zx.RunFrame().
//
// The scheduler keeps its own monotonic frame counter rather than relying on
// the loop variable, because the GUI loop is unbounded and has no frame index
// of its own. Actions whose Frame is <= the current counter and have not yet
// fired are dispatched in order. Multiple actions due on the same frame all
// fire; a warning is logged (subject to verbosity) so collisions are visible.
//
// Phase 1 wires the verbs that correspond to existing functionality and touch
// no keyboard state. Phase 2 keyboard verbs are present in the dispatch table
// but return a "not yet wired" error so that forward-looking scripts fail
// clearly rather than silently.
// ============================================================================

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// actionHandler executes one action against the emulator. It returns an error
// describing any failure; the scheduler logs but does not abort on handler
// errors, so a single bad action does not kill an automation run.
type actionHandler func(s *Scheduler, zx *ZenZX, a Action) error

// ScheduleConfig carries the front-end-specific settings the handlers need.
// Both main loops populate this from their own flags before constructing the
// scheduler. The GUI front-end has no -shot-dir/-shot-prefix flags, so it
// passes its own sensible defaults.
type ScheduleConfig struct {
	ShotDir    string // directory screenshots are written into
	ShotPrefix string // filename prefix for auto-named screenshots
	Quiet      bool   // suppress informational and warning logging
	Model      string // Spectrum model name, for boot detection (wait-boot)
}

// Scheduler holds a script and the run-time state needed to drive it.
type Scheduler struct {
	script   *Script
	cfg      ScheduleConfig
	frame    int  // monotonic frame counter, incremented each Tick
	rebase   int  // frame offset subtracted to give the effective clock
	next     int  // index of the next un-fired action in script.Actions
	quit     bool // set true by the 'quit' action
	shotSeq    int  // sequence number for auto-named screenshots
	shotsTaken int  // total successful screenshots taken by the script
	expectFailures int // count of failed expect-screen assertions
	waiting  *BootDetector // non-nil while a wait-boot action is blocking
	keys     *KeyQueue     // non-nil while scripted keys are being injected
	keyHold  int           // configured key hold frames
	keyGap   int           // configured key gap frames

	// wait-screen blocking state, active while waitScreenActive is true.
	waitScreenActive  bool
	waitScreenRow     int
	waitScreenText    string
	waitScreenElapsed int
	waitScreenTimeout int
	waitScreenLine    int

	// wait-attr blocking state, active while waitAttrActive is true.
	waitAttrActive   bool
	waitAttrMatch    attrMatch
	waitAttrRowStart int
	waitAttrRowEnd   int
	waitAttrCount    int
	waitAttrElapsed  int
	waitAttrTimeout  int
	waitAttrLine     int
	waitAttrDesc     string

	handlers map[string]actionHandler
}

// NewScheduler builds a scheduler for the given script and configuration.
func NewScheduler(script *Script, cfg ScheduleConfig) *Scheduler {
	s := &Scheduler{script: script, cfg: cfg, keyHold: defaultKeyHoldFrames, keyGap: defaultKeyGapFrames}
	s.handlers = map[string]actionHandler{
		"shot":      (*Scheduler).doShot,
		"reset":     (*Scheduler).doReset,
		"tape-play": (*Scheduler).doTapePlay,
		"tape-stop": (*Scheduler).doTapeStop,
		"snapshot":  (*Scheduler).doSnapshot,
		"snapshot-save": (*Scheduler).doSnapshotSave,
		"bin":       (*Scheduler).doBin,
		"scr":       (*Scheduler).doScr,
		"quit":      (*Scheduler).doQuit,
		"dump-screen":   (*Scheduler).doDumpScreen,
		"expect-screen": (*Scheduler).doExpectScreen,
		// Keyboard injection (Phase 2).
		"type":    (*Scheduler).doType,
		"key":     (*Scheduler).doKey,
		"press":   (*Scheduler).doPress,
		"release": (*Scheduler).doRelease,
	}
	return s
}

// QuitRequested reports whether a 'quit' action has fired. The headless loop
// checks this to break early; the GUI loop may ignore it or use it to close.
func (s *Scheduler) QuitRequested() bool { return s.quit }

// ShotsTaken returns the number of screenshots the script has captured.
func (s *Scheduler) ShotsTaken() int { return s.shotsTaken }

// ExpectFailures returns the number of failed expect-screen assertions.
func (s *Scheduler) ExpectFailures() int { return s.expectFailures }

// Tick advances the scheduler by one frame and fires any due actions. It must
// be called exactly once per emulated frame, before RunFrame.
//
// Actions fire against an *effective* clock (s.frame - s.rebase). A wait-boot
// action blocks the timeline: while it is the next action and boot is not yet
// ready, no later action fires and the effective clock does not advance past
// it. When boot completes, the rebase origin is set to the current frame, so
// every offset after a wait-boot is measured from boot-ready -- making scripts
// portable across models with different boot times.
func (s *Scheduler) Tick(zx *ZenZX) {
	if s.script == nil {
		return
	}

	// Handle a blocking wait-boot at the head of the queue.
	if s.next < len(s.script.Actions) && s.script.Actions[s.next].Verb == "wait-boot" {
		if s.waiting == nil {
			rainbow := false
			if b, ok := bootBaselines[s.cfg.Model]; ok {
				rainbow = b.rainbow
			}
			s.waiting = NewBootDetector(rainbow)
			s.logf("script: waiting for boot (model=%s)", s.cfg.Model)
		}
		s.waiting.Update(zx)
		if s.waiting.Ready() {
			// Rebase the effective clock to start at zero now, so offsets on
			// post-wait actions are measured from this moment. This frame is a
			// short confirmation window (stableConfirmFrames) after the screen
			// actually settled; that built-in margin is harmless and authors
			// add any further delay via the first post-wait offset.
			s.rebase = s.frame
			s.logf("script: boot ready at frame %d (settled ~%d); rebasing timeline",
				s.frame, s.waiting.ReadyFrame())
			s.waiting = nil
			s.next++
		} else {
			// Still booting: advance the real frame counter but hold the
			// effective clock and fire nothing further this tick.
			s.frame++
			return
		}
	}

	// Drain an active key queue. Like wait-boot, key injection blocks the
	// timeline: while keys are playing, the effective clock is held and no
	// later action fires, so a following offset is measured from the moment
	// typing completes. When the queue empties, rebase to now and clear the
	// matrix so held keys are released.
	if s.keys != nil {
		if s.keys.Step(zx) {
			s.frame++
			return
		}
		// Queue just finished on this frame.
		zx.io.ResetKeyboard()
		s.keys = nil
		s.rebase = s.frame
		s.logf("script: key input complete at frame %d; rebasing timeline", s.frame)
		s.frame++
		return
	}

	// Handle a blocking wait-screen at the head of the queue: pause the
	// timeline until the target text appears on the given row, or the timeout
	// elapses. On match (or timeout) the timeline rebases to that moment, like
	// wait-boot, so following offsets are measured from when the text appeared.
	if !s.waitScreenActive && s.next < len(s.script.Actions) && s.script.Actions[s.next].Verb == "wait-screen" {
		a := s.script.Actions[s.next]
		if err := s.beginWaitScreen(a); err != nil {
			s.warnf("line %d: wait-screen: %v", a.Line, err)
			s.next++ // skip the malformed action
		}
	}
	if s.waitScreenActive {
		got := strings.TrimRight(ReadScreenRow(zx, s.waitScreenRow), " ")
		if strings.Contains(got, s.waitScreenText) {
			s.logf("script: wait-screen matched %q on row %d at frame %d; rebasing timeline",
				s.waitScreenText, s.waitScreenRow, s.frame)
			s.waitScreenActive = false
			s.rebase = s.frame
			s.next++
			s.frame++
			return
		}
		s.waitScreenElapsed++
		if s.waitScreenElapsed >= s.waitScreenTimeout {
			s.expectFailures++
			s.warnf("line %d: wait-screen TIMED OUT after %d frames waiting for %q on row %d (got %q)",
				s.waitScreenLine, s.waitScreenTimeout, s.waitScreenText, s.waitScreenRow, got)
			s.waitScreenActive = false
			s.rebase = s.frame
			s.next++
			s.frame++
			return
		}
		// Still waiting: hold the effective clock, fire nothing else.
		s.frame++
		return
	}

	// Handle a blocking wait-attr at the head of the queue: pause until at
	// least the requested number of cells in the region match the colour
	// predicate, or the timeout elapses. Same block-and-rebase semantics as
	// wait-screen. Useful for recognising menus/screens by colour when their
	// characters use a custom font the glyph matcher cannot read.
	if !s.waitAttrActive && s.next < len(s.script.Actions) && s.script.Actions[s.next].Verb == "wait-attr" {
		a := s.script.Actions[s.next]
		if err := s.beginWaitAttr(a); err != nil {
			s.warnf("line %d: wait-attr: %v", a.Line, err)
			s.next++
		}
	}
	if s.waitAttrActive {
		n := CountAttrMatches(zx, s.waitAttrMatch, s.waitAttrRowStart, s.waitAttrRowEnd)
		if n >= s.waitAttrCount {
			s.logf("script: wait-attr matched %s (%d cells) at frame %d; rebasing timeline",
				s.waitAttrDesc, n, s.frame)
			s.waitAttrActive = false
			s.rebase = s.frame
			s.next++
			s.frame++
			return
		}
		s.waitAttrElapsed++
		if s.waitAttrElapsed >= s.waitAttrTimeout {
			s.expectFailures++
			s.warnf("line %d: wait-attr TIMED OUT after %d frames waiting for %s (best %d/%d cells)",
				s.waitAttrLine, s.waitAttrTimeout, s.waitAttrDesc, n, s.waitAttrCount)
			s.waitAttrActive = false
			s.rebase = s.frame
			s.next++
			s.frame++
			return
		}
		s.frame++
		return
	}

	eff := s.frame - s.rebase

	// Count how many actions are due on this effective frame, for collision
	// warning. Actions are sorted by frame, so a contiguous run starting at
	// s.next shares the lowest due frame.
	due := 0
	for i := s.next; i < len(s.script.Actions) && !isBarrier(s.script.Actions[i].Verb) && s.script.Actions[i].Frame <= eff; i++ {
		due++
	}
	if due > 1 && s.script.Actions[s.next].Frame == s.script.Actions[s.next+due-1].Frame {
		s.warnf("frame %d: %d actions scheduled on the same frame; firing all in order", eff, due)
	}

	for s.next < len(s.script.Actions) {
		a := s.script.Actions[s.next]
		if isBarrier(a.Verb) {
			break // handled at the top of the next Tick
		}
		if a.Frame > eff {
			break
		}
		s.next++
		h, ok := s.handlers[a.Verb]
		if !ok {
			// Should be unreachable: the parser rejects unknown verbs.
			s.warnf("line %d: no handler for verb %q", a.Line, a.Verb)
			continue
		}
		if err := h(s, zx, a); err != nil {
			s.warnf("line %d: %s: %v", a.Line, a.Verb, err)
		}
	}

	s.frame++
}

// logf prints an informational message unless quiet.
func (s *Scheduler) logf(format string, args ...any) {
	if !s.cfg.Quiet {
		fmt.Printf(format+"\n", args...)
	}
}

// warnf prints a warning unless quiet.
func (s *Scheduler) warnf(format string, args ...any) {
	if !s.cfg.Quiet {
		fmt.Fprintf(os.Stderr, "script warning: "+format+"\n", args...)
	}
}

// ----------------------------------------------------------------------------
// Phase 1 handlers
// ----------------------------------------------------------------------------

// doShot captures the current display to a PNG. With an argument, the argument
// is used as the filename (a .png extension is appended if absent) inside the
// configured shot directory. Without an argument, an auto-incrementing name
// based on the prefix and current frame is used.
func (s *Scheduler) doShot(zx *ZenZX, a Action) error {
	var name string
	if len(a.Args) > 0 {
		name = a.Args[0]
		if !strings.HasSuffix(strings.ToLower(name), ".png") {
			name += ".png"
		}
	} else {
		s.shotSeq++
		name = fmt.Sprintf("%s-frame%06d.png", s.cfg.ShotPrefix, s.frame)
	}
	path := filepath.Join(s.cfg.ShotDir, name)
	if err := writeScreenPNG(path, zx.screen); err != nil {
		return err
	}
	s.shotsTaken++
	s.logf("script: frame %d -> %s", s.frame, path)
	return nil
}

// doReset resets the CPU.
func (s *Scheduler) doReset(zx *ZenZX, _ Action) error {
	zx.cpu.Reset()
	s.logf("script: frame %d -> reset", s.frame)
	return nil
}

// doTapePlay starts tape playback if a tape is loaded.
func (s *Scheduler) doTapePlay(zx *ZenZX, _ Action) error {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return fmt.Errorf("no tape loaded")
	}
	zx.tape.Play()
	s.logf("script: frame %d -> tape-play", s.frame)
	return nil
}

// doTapeStop stops tape playback if a tape is loaded.
func (s *Scheduler) doTapeStop(zx *ZenZX, _ Action) error {
	if zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Loaded {
		return fmt.Errorf("no tape loaded")
	}
	zx.tape.Stop()
	s.logf("script: frame %d -> tape-stop", s.frame)
	return nil
}

// doSnapshot loads a snapshot. Usage: snapshot <path> [format]. Format
// defaults to auto-detection.
func (s *Scheduler) doSnapshot(zx *ZenZX, a Action) error {
	if len(a.Args) < 1 {
		return fmt.Errorf("usage: snapshot <path> [format]")
	}
	path := a.Args[0]
	format := "auto"
	if len(a.Args) >= 2 {
		format = a.Args[1]
	}
	if format == "auto" {
		if format = DetectSnapshotFormat(path); format == "" {
			format = "zxs"
		}
	}
	if err := zx.LoadSnapshotFormat(path, format); err != nil {
		return err
	}
	s.logf("script: frame %d -> snapshot %s (%s)", s.frame, path, strings.ToUpper(format))
	return nil
}

// doSnapshotSave saves a snapshot of the current machine state. Usage:
// snapshot-save <path> [format]. Format defaults to the path's extension, or
// zxs if none is recognised.
func (s *Scheduler) doSnapshotSave(zx *ZenZX, a Action) error {
	if len(a.Args) < 1 {
		return fmt.Errorf("usage: snapshot-save <path> [format]")
	}
	path := a.Args[0]
	format := "auto"
	if len(a.Args) >= 2 {
		format = a.Args[1]
	}
	if format == "auto" {
		if format = DetectSnapshotFormat(path); format == "" {
			format = "zxs"
		}
	}
	if err := zx.SaveSnapshotFormat(path, format); err != nil {
		return err
	}
	s.logf("script: frame %d -> snapshot-save %s (%s)", s.frame, path, strings.ToUpper(format))
	return nil
}
// loadAddr defaults to 0x8000; startAddr defaults to the load address.
func (s *Scheduler) doBin(zx *ZenZX, a Action) error {
	if len(a.Args) < 1 {
		return fmt.Errorf("usage: bin <path> [loadAddr] [startAddr]")
	}
	path := a.Args[0]

	addr := uint16(0x8000)
	if len(a.Args) >= 2 {
		v, err := ParseAddr(a.Args[1])
		if err != nil {
			return fmt.Errorf("invalid load address %q: %v", a.Args[1], err)
		}
		addr = v
	}

	start := int(addr)
	if len(a.Args) >= 3 {
		v, err := ParseAddrSigned(a.Args[2])
		if err != nil {
			return fmt.Errorf("invalid start address %q: %v", a.Args[2], err)
		}
		start = v
	}

	if err := zx.LoadBIN(path, addr, start); err != nil {
		return err
	}
	if start >= 0 {
		s.logf("script: frame %d -> bin %s at 0x%04X, PC=0x%04X", s.frame, path, addr, start)
	} else {
		s.logf("script: frame %d -> bin %s at 0x%04X (PC unchanged)", s.frame, path, addr)
	}
	return nil
}

// doScr loads a raw .scr screen dump onto the display. Usage: scr <path>.
func (s *Scheduler) doScr(zx *ZenZX, a Action) error {
	if len(a.Args) < 1 {
		return fmt.Errorf("usage: scr <path>")
	}
	if err := zx.LoadSCR(a.Args[0]); err != nil {
		return err
	}
	s.logf("script: frame %d -> scr %s", s.frame, a.Args[0])
	return nil
}

// doQuit requests that the run end after this frame.
func (s *Scheduler) doQuit(_ *ZenZX, _ Action) error {
	s.quit = true
	s.logf("script: frame %d -> quit", s.frame)
	return nil
}

// doDumpScreen prints recognised screen text. Usage: dump-screen [row]. With a
// row (0-23) it prints that row; without, it prints all 24 rows. Trailing
// blank space is trimmed for readability.
func (s *Scheduler) doDumpScreen(zx *ZenZX, a Action) error {
	if len(a.Args) >= 1 {
		row, err := strconv.Atoi(a.Args[0])
		if err != nil || row < 0 || row >= screenRows {
			return fmt.Errorf("row must be 0..%d", screenRows-1)
		}
		fmt.Printf("screen[%02d]: %q\n", row, strings.TrimRight(ReadScreenRow(zx, row), " "))
		return nil
	}
	for r, line := range ReadScreen(zx) {
		trimmed := strings.TrimRight(line, " ")
		if trimmed != "" {
			fmt.Printf("screen[%02d]: %q\n", r, trimmed)
		}
	}
	return nil
}

// doExpectScreen asserts that a screen row contains the expected text. Usage:
// expect-screen <row> <text...>. The match is a case-sensitive substring of
// the recognised row. A failure is logged and counted (see ExpectFailures)
// but does not abort the run.
func (s *Scheduler) doExpectScreen(zx *ZenZX, a Action) error {
	if len(a.Args) < 2 {
		return fmt.Errorf("usage: expect-screen <row> <text>")
	}
	row, err := strconv.Atoi(a.Args[0])
	if err != nil || row < 0 || row >= screenRows {
		return fmt.Errorf("row must be 0..%d", screenRows-1)
	}
	want := unquote(strings.Join(a.Args[1:], " "))
	got := strings.TrimRight(ReadScreenRow(zx, row), " ")
	if strings.Contains(got, want) {
		s.logf("script: frame %d -> expect-screen row %d: PASS (%q)", s.frame, row, want)
	} else {
		s.expectFailures++
		s.warnf("frame %d: expect-screen row %d FAILED: wanted %q, got %q", s.frame, row, want, got)
	}
	return nil
}

// notWired is the dispatch stub for known-but-unimplemented Phase 2 verbs.
func notWired(_ *Scheduler, _ *ZenZX, a Action) error {
	return fmt.Errorf("verb %q is not yet wired", a.Verb)
}

// defaultWaitScreenTimeout is how long wait-screen waits before failing, in
// frames (5 s at 50 fps). Long enough for an accurate-mode tape header.
const defaultWaitScreenTimeout = 250

// beginWaitScreen parses a wait-screen action and arms the blocking state.
// Usage: wait-screen <row> <text...> [timeout=<frames|Ns|Nms>]. The timeout is
// an optional final token of the form timeout=VALUE.
func (s *Scheduler) beginWaitScreen(a Action) error {
	if len(a.Args) < 2 {
		return fmt.Errorf("usage: wait-screen <row> <text> [timeout=<frames>]")
	}
	row, err := strconv.Atoi(a.Args[0])
	if err != nil || row < 0 || row >= screenRows {
		return fmt.Errorf("row must be 0..%d", screenRows-1)
	}

	args := a.Args[1:]
	timeout := defaultWaitScreenTimeout
	if last := args[len(args)-1]; strings.HasPrefix(last, "timeout=") {
		v := strings.TrimPrefix(last, "timeout=")
		t, err := parseOffset(v)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %v", v, err)
		}
		timeout = t
		args = args[:len(args)-1]
		if len(args) == 0 {
			return fmt.Errorf("usage: wait-screen <row> <text> [timeout=<frames>]")
		}
	}

	s.waitScreenActive = true
	s.waitScreenRow = row
	s.waitScreenText = unquote(strings.Join(args, " "))
	s.waitScreenElapsed = 0
	s.waitScreenTimeout = timeout
	s.waitScreenLine = a.Line
	s.logf("script: waiting for %q on row %d (timeout %d frames)", s.waitScreenText, row, timeout)
	return nil
}

// beginWaitAttr parses a wait-attr action and arms the blocking state. Usage:
// wait-attr <rowspec> <colourspec...> [count=N] [timeout=<frames>]. rowspec is
// a single row (10), an inclusive range (8-12), or "all". colourspec is one or
// more of ink=COLOUR, paper=COLOUR, bright, bright=0|1 (COLOUR is a name or
// 0-7). count defaults to 1 (at least one matching cell).
func (s *Scheduler) beginWaitAttr(a Action) error {
	if len(a.Args) < 2 {
		return fmt.Errorf("usage: wait-attr <rowspec> <colourspec> [count=N] [timeout=<frames>]")
	}

	rowStart, rowEnd, err := parseRowSpec(a.Args[0])
	if err != nil {
		return err
	}

	args := a.Args[1:]
	count := 1
	timeout := defaultWaitScreenTimeout
	// Strip trailing count= / timeout= tokens (order-independent).
	for len(args) > 0 {
		last := args[len(args)-1]
		if strings.HasPrefix(last, "count=") {
			n, err := strconv.Atoi(strings.TrimPrefix(last, "count="))
			if err != nil || n < 1 {
				return fmt.Errorf("invalid count %q", last)
			}
			count = n
			args = args[:len(args)-1]
		} else if strings.HasPrefix(last, "timeout=") {
			t, err := parseOffset(strings.TrimPrefix(last, "timeout="))
			if err != nil {
				return fmt.Errorf("invalid timeout %q", last)
			}
			timeout = t
			args = args[:len(args)-1]
		} else {
			break
		}
	}
	if len(args) == 0 {
		return fmt.Errorf("no colour spec given")
	}

	m, err := parseAttrSpec(args)
	if err != nil {
		return err
	}

	s.waitAttrActive = true
	s.waitAttrMatch = m
	s.waitAttrRowStart = rowStart
	s.waitAttrRowEnd = rowEnd
	s.waitAttrCount = count
	s.waitAttrElapsed = 0
	s.waitAttrTimeout = timeout
	s.waitAttrLine = a.Line
	s.waitAttrDesc = fmt.Sprintf("rows %d-%d %v count>=%d", rowStart, rowEnd, args, count)
	s.logf("script: waiting for attributes %s (timeout %d frames)", s.waitAttrDesc, timeout)
	return nil
}

// parseRowSpec parses "10", "8-12", or "all" into an inclusive row range.
func parseRowSpec(spec string) (int, int, error) {
	if spec == "all" {
		return 0, screenRows - 1, nil
	}
	if i := strings.IndexByte(spec, '-'); i >= 0 {
		a, err1 := strconv.Atoi(spec[:i])
		b, err2 := strconv.Atoi(spec[i+1:])
		if err1 != nil || err2 != nil || a < 0 || b >= screenRows || a > b {
			return 0, 0, fmt.Errorf("bad row range %q", spec)
		}
		return a, b, nil
	}
	r, err := strconv.Atoi(spec)
	if err != nil || r < 0 || r >= screenRows {
		return 0, 0, fmt.Errorf("bad row %q", spec)
	}
	return r, r, nil
}

// keyQueue returns the active key queue, creating one if needed.
func (s *Scheduler) keyQueue() *KeyQueue {
	if s.keys == nil {
		s.keys = NewKeyQueue(s.keyHold, s.keyGap)
	}
	return s.keys
}

// doType enqueues a string to be typed. The text may be quoted to preserve
// spaces; the parser passes the raw remainder as args, so we rejoin them.
// Usage: type <text...>  (e.g. type LOAD "" or type "10 PRINT HELLO")
func (s *Scheduler) doType(_ *ZenZX, a Action) error {
	if len(a.Args) == 0 {
		return fmt.Errorf("usage: type <text>")
	}
	text := unquote(strings.Join(a.Args, " "))
	skipped := s.keyQueue().EnqueueText(text)
	if skipped > 0 {
		s.warnf("line %d: type: %d character(s) not typeable, skipped", a.Line, skipped)
	}
	s.logf("script: frame %d -> type %q", s.frame, text)
	return nil
}

// doKey enqueues one named key or chord. Usage: key <name> (e.g. key enter,
// key delete, key up).
func (s *Scheduler) doKey(_ *ZenZX, a Action) error {
	if len(a.Args) < 1 {
		return fmt.Errorf("usage: key <name>")
	}
	name := strings.ToLower(a.Args[0])
	chord, ok := namedKeys[name]
	if !ok {
		return fmt.Errorf("unknown key name %q", a.Args[0])
	}
	s.keyQueue().EnqueueChord(chord)
	s.logf("script: frame %d -> key %s", s.frame, name)
	return nil
}

// doPress holds a key down indefinitely (until release), for held-key
// scenarios like games. Usage: press <name>. Unlike type/key, press applies
// immediately and does not block the timeline.
func (s *Scheduler) doPress(zx *ZenZX, a Action) error {
	if len(a.Args) < 1 {
		return fmt.Errorf("usage: press <name>")
	}
	name := strings.ToLower(a.Args[0])
	chord, ok := namedKeys[name]
	if !ok {
		// Allow a single typeable character too.
		if r := []rune(name); len(r) == 1 {
			if pos, ok2 := baseKeys[r[0]]; ok2 {
				chord = []matrixPos{pos}
				ok = true
			}
		}
	}
	if !ok {
		return fmt.Errorf("unknown key name %q", a.Args[0])
	}
	for _, k := range chord {
		zx.io.PressKey(k.row, k.col)
	}
	s.logf("script: frame %d -> press %s", s.frame, name)
	return nil
}

// doRelease releases a held key (or all keys with "all"). Usage: release
// <name|all>.
func (s *Scheduler) doRelease(zx *ZenZX, a Action) error {
	if len(a.Args) < 1 {
		return fmt.Errorf("usage: release <name|all>")
	}
	name := strings.ToLower(a.Args[0])
	if name == "all" {
		zx.io.ResetKeyboard()
		s.logf("script: frame %d -> release all", s.frame)
		return nil
	}
	chord, ok := namedKeys[name]
	if !ok {
		if r := []rune(name); len(r) == 1 {
			if pos, ok2 := baseKeys[r[0]]; ok2 {
				chord = []matrixPos{pos}
				ok = true
			}
		}
	}
	if !ok {
		return fmt.Errorf("unknown key name %q", a.Args[0])
	}
	for _, k := range chord {
		zx.io.ReleaseKey(k.row, k.col)
	}
	s.logf("script: frame %d -> release %s", s.frame, name)
	return nil
}

// unquote strips a single pair of surrounding double quotes only when they
// wrap content containing whitespace -- the sole reason to quote an argument,
// since the parser splits on whitespace. When the argument has no internal
// space (including the literal two-character string ""), the quotes are taken
// as characters to type, not as wrapping, so `type ""` types two quote marks.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		inner := s[1 : len(s)-1]
		if strings.ContainsAny(inner, " \t") {
			return inner
		}
	}
	return s
}

// ----------------------------------------------------------------------------
// Shared screenshot helper
// ----------------------------------------------------------------------------

// writeScreenPNG encodes the current Spectrum framebuffer to a PNG file. It is
// shared by the scheduler's shot handler across both front-ends.
func writeScreenPNG(path string, screen *SpectrumScreen) error {
	img := screen.DecodeRGBA()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
