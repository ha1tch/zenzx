package main

// ============================================================================
// .zen action-script format
//
// A .zen file is a flat, human-editable, timestamped list of actions to
// perform on the virtual Spectrum, modelled loosely on a subtitle file. Each
// significant line has the form:
//
//	<offset>  <verb>  [args...]
//
// Fields are whitespace-separated. Blank lines and lines whose first
// non-space character is '#' are ignored. A trailing '#' comment is NOT
// stripped from a verb line -- keep comments on their own lines.
//
// Offset units
//   The offset is a frame count from boot by default. A unit suffix may be
//   given and is converted to frames at load time (50 fps, PAL):
//	   100      -> frame 100
//	   100f     -> frame 100   (explicit frames)
//	   2s       -> frame 100   (2 seconds * 50)
//	   500ms    -> frame 25     (500 ms / 20 ms-per-frame)
//   Fractional results are rounded to the nearest frame.
//
// Verbs are validated at parse time. An unknown verb is a hard error and the
// script is refused. Verbs that are known but not yet wired (the Phase 2
// keyboard verbs) parse successfully and fail at dispatch with a clear
// message, so that forward-looking scripts remain loadable.
// ============================================================================

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// framesPerSecond is the PAL frame rate the offset units are converted at.
const framesPerSecond = 50.0

// Action is a single scheduled instruction parsed from a .zen file.
type Action struct {
	Frame int      // absolute frame offset from boot (>= 0)
	Verb  string   // canonical verb, lower-case
	Args  []string // raw positional arguments after the verb
	Line  int      // 1-based source line number, for diagnostics
}

// Script is an ordered set of actions, sorted by frame then source order.
type Script struct {
	Actions []Action
}

// knownVerbs is the complete vocabulary the parser accepts. Membership here
// only means "this is a real verb"; whether it is wired for execution is a
// separate question answered by the scheduler's dispatch table. Phase 2
// keyboard verbs are listed so that scripts using them load today and simply
// fail at dispatch until the handlers are implemented.
var knownVerbs = map[string]bool{
	// Phase 1 -- wired against existing functionality.
	"shot":      true,
	"reset":     true,
	"tape-play": true,
	"tape-stop": true,
	"snapshot":  true,
	"snapshot-save": true,
	"bin":       true,
	"scr":       true,
	"quit":      true,
	"wait-boot": true,
	"wait-screen":   true,
	"wait-attr":     true,
	"dump-screen":   true,
	"expect-screen": true,
	// Phase 2 -- known but not yet wired.
	"key":     true,
	"type":    true,
	"press":   true,
	"release": true,
}

// isBarrier reports whether a verb acts as a sort/timeline barrier. These
// verbs partition the timeline: offsets do not sort across them. wait-screen
// takes arguments (so is not a lone-token barrierVerb) but is still a barrier.
func isBarrier(verb string) bool {
	return verb == "wait-boot" || verb == "wait-screen" || verb == "wait-attr"
}

// ParseScriptFile reads and parses a .zen file from disk.
func ParseScriptFile(path string) (*Script, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseScript(bufio.NewScanner(f), path)
}

// parseScript parses scanner lines into a validated, sorted Script. The path
// is used only to qualify error messages.
func parseScript(sc *bufio.Scanner, path string) (*Script, error) {
	var actions []Action
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)

		// Barrier verbs (wait-boot, wait-screen, wait-attr) take no leading
		// offset: they are positioned by source order and the scheduler blocks
		// on them. When the first token is a barrier verb, the rest of the line
		// is its arguments. They are recorded at frame 0.
		if isBarrier(strings.ToLower(fields[0])) {
			actions = append(actions, Action{
				Frame: 0,
				Verb:  strings.ToLower(fields[0]),
				Args:  fields[1:],
				Line:  lineNo,
			})
			continue
		}

		if len(fields) < 2 {
			return nil, fmt.Errorf("%s:%d: expected '<offset> <verb> [args...]', got %q", path, lineNo, raw)
		}

		frame, err := parseOffset(fields[0])
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %v", path, lineNo, err)
		}

		verb := strings.ToLower(fields[1])
		if !knownVerbs[verb] {
			return nil, fmt.Errorf("%s:%d: unknown verb %q", path, lineNo, fields[1])
		}

		actions = append(actions, Action{
			Frame: frame,
			Verb:  verb,
			Args:  fields[2:],
			Line:  lineNo,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: read error: %v", path, err)
	}

	// Sort by frame within each partition delimited by wait-boot barriers.
	// Offsets before the first wait-boot are absolute from power-on; offsets
	// after a wait-boot are relative to boot-ready (the scheduler rebases the
	// clock there). Sorting across a barrier would scramble these two frames
	// of reference, so wait-boot markers stay fixed in source order and only
	// the runs between them are sorted. Within a run the sort is stable, so
	// same-frame actions keep their authored order.
	sortPartitions(actions)

	return &Script{Actions: actions}, nil
}

// sortPartitions stably sorts actions by frame within each maximal run that
// contains no wait-boot, leaving wait-boot markers in their source positions
// as barriers.
func sortPartitions(actions []Action) {
	start := 0
	flush := func(end int) {
		seg := actions[start:end]
		sort.SliceStable(seg, func(i, j int) bool {
			return seg[i].Frame < seg[j].Frame
		})
	}
	for i, a := range actions {
		if isBarrier(a.Verb) {
			flush(i)      // sort the run before the barrier
			start = i + 1 // next run starts after it
		}
	}
	flush(len(actions)) // final run after the last barrier
}

// parseOffset converts an offset token (with optional f/s/ms suffix) into an
// absolute frame number. Negative offsets are rejected.
func parseOffset(tok string) (int, error) {
	t := strings.ToLower(strings.TrimSpace(tok))

	var numStr string
	var toFrames func(float64) float64

	switch {
	case strings.HasSuffix(t, "ms"):
		numStr = strings.TrimSuffix(t, "ms")
		toFrames = func(v float64) float64 { return v / 1000.0 * framesPerSecond }
	case strings.HasSuffix(t, "s"):
		numStr = strings.TrimSuffix(t, "s")
		toFrames = func(v float64) float64 { return v * framesPerSecond }
	case strings.HasSuffix(t, "f"):
		numStr = strings.TrimSuffix(t, "f")
		toFrames = func(v float64) float64 { return v }
	default:
		numStr = t
		toFrames = func(v float64) float64 { return v }
	}

	v, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid offset %q", tok)
	}
	if v < 0 {
		return 0, fmt.Errorf("negative offset %q", tok)
	}

	frame := int(math.Round(toFrames(v)))
	return frame, nil
}
