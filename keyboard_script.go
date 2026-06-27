package main

// ============================================================================
// Scripted keyboard injection
//
// Spectrum keys are read by the ULA scanning the 8x8 key matrix once per
// 50 Hz frame. A scripted keypress must therefore be *held* across several
// frames for the ROM to register it, and separated from the next press by an
// idle gap so repeated identical keys are seen as distinct. A keystroke is
// thus not instantaneous: it expands into a timed press/release sequence the
// scheduler drives frame by frame.
//
// Matrix reference (row, bit):
//   Row 0: CAPS SHIFT, Z, X, C, V          Row 4: 0, 9, 8, 7, 6
//   Row 1: A, S, D, F, G                    Row 5: P, O, I, U, Y
//   Row 2: Q, W, E, R, T                    Row 6: ENTER, L, K, J, H
//   Row 3: 1, 2, 3, 4, 5                    Row 7: SPACE, SYM SHIFT, M, N, B
//
// CAPS SHIFT is (0,0); SYMBOL SHIFT is (7,1). Symbols are produced by holding
// SYMBOL SHIFT with a letter/number key.
// ============================================================================

// Default key timing, in frames. "Very safe" values: a 5-frame hold clears
// several ULA scans, and a 3-frame gap cleanly separates repeats.
const (
	defaultKeyHoldFrames = 10
	defaultKeyGapFrames  = 5
)

// matrixPos is a key position: row 0-7, col (bit) 0-4.
type matrixPos struct{ row, col uint8 }

var (
	capsShift = matrixPos{0, 0}
	symShift  = matrixPos{7, 1}
)

// baseKeys maps a character to the matrix key that produces it with no shift.
// Letters are stored lower-case here; typing is case-insensitive for letters
// (the Spectrum's unshifted letter keys produce the letter; CAPS handling for
// upper-case is a refinement noted below).
var baseKeys = map[rune]matrixPos{
	// Letters
	'a': {1, 0}, 's': {1, 1}, 'd': {1, 2}, 'f': {1, 3}, 'g': {1, 4},
	'q': {2, 0}, 'w': {2, 1}, 'e': {2, 2}, 'r': {2, 3}, 't': {2, 4},
	'1': {3, 0}, '2': {3, 1}, '3': {3, 2}, '4': {3, 3}, '5': {3, 4},
	'0': {4, 0}, '9': {4, 1}, '8': {4, 2}, '7': {4, 3}, '6': {4, 4},
	'p': {5, 0}, 'o': {5, 1}, 'i': {5, 2}, 'u': {5, 3}, 'y': {5, 4},
	'l': {6, 1}, 'k': {6, 2}, 'j': {6, 3}, 'h': {6, 4},
	'm': {7, 2}, 'n': {7, 3}, 'b': {7, 4},
	'z': {0, 1}, 'x': {0, 2}, 'c': {0, 3}, 'v': {0, 4},
	' ':  {7, 0}, // SPACE
	'\n': {6, 0}, // ENTER
}

// symKeys maps a character to a key pressed together with SYMBOL SHIFT.
// These follow the Spectrum's symbol-shifted keyboard legends.
var symKeys = map[rune]matrixPos{
	'!': {3, 0}, '@': {3, 1}, '#': {3, 2}, '$': {3, 3}, '%': {3, 4},
	'&': {4, 4}, '\'': {4, 3}, '(': {4, 2}, ')': {4, 1}, '_': {4, 0},
	'<': {7, 3}, '>': {7, 2}, // N, M
	';': {5, 1}, '"': {5, 0}, // O, P
	'=': {6, 1}, '+': {6, 2}, '-': {6, 3}, // L, K, J
	'?': {0, 3}, '/': {0, 4}, '*': {7, 4}, // C, V, B
	',': {7, 3}, '.': {7, 2}, // N, M (comma/period share with <,>)
	':': {0, 1}, // Z (SYM SHIFT + Z = colon)
}

// keyStep is a single frame-level instruction in the key queue: which keys to
// hold this many frames, or an idle gap.
type keyStep struct {
	keys   []matrixPos // keys to hold (empty = idle gap)
	frames int         // how many frames to hold/idle
}

// KeyQueue is a frame-driven queue of key steps. The scheduler advances it one
// frame per Tick via Step, which applies the current hold to the matrix.
type KeyQueue struct {
	steps     []keyStep
	idx       int // current step
	remaining int // frames left in the current step
	hold      int // configured hold frames
	gap       int // configured gap frames
	lastKeys  []matrixPos // previous chord, for detecting repeated keys
}

// NewKeyQueue returns an empty queue with the given hold/gap timing.
func NewKeyQueue(hold, gap int) *KeyQueue {
	if hold <= 0 {
		hold = defaultKeyHoldFrames
	}
	if gap <= 0 {
		gap = defaultKeyGapFrames
	}
	return &KeyQueue{hold: hold, gap: gap, remaining: 0}
}

// Active reports whether the queue still has steps to play.
func (q *KeyQueue) Active() bool {
	return q.idx < len(q.steps)
}

// enqueueChord adds a hold of the given keys followed by an idle gap.
func (q *KeyQueue) enqueueChord(keys []matrixPos) {
	gap := q.gap
	if sameChord(keys, q.lastKeys) {
		// A repeated identical key needs a longer key-up so the ROM does not
		// treat the second press as keyboard bounce and merge the two (e.g.
		// the double L in "HELLO").
		gap = q.hold + q.gap
	}
	q.steps = append(q.steps, keyStep{keys: keys, frames: q.hold})
	q.steps = append(q.steps, keyStep{keys: nil, frames: gap})
	q.lastKeys = keys
}

// sameChord reports whether two key chords are identical.
func sameChord(a, b []matrixPos) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// enqueueShiftedChord adds a key pressed together with a shift, but staged the
// way a human presses it: the shift is held alone for a settle period first,
// then the key is added while the shift stays down, then both release and gap.
// This gives the ROM a clean shift transition before the key, which matters
// for consecutive shifted keys (e.g. typing two quotes) where pressing and
// releasing shift in lockstep with the key can merge into a single event.
func (q *KeyQueue) enqueueShiftedChord(shift, key matrixPos) {
	q.steps = append(q.steps, keyStep{keys: []matrixPos{shift}, frames: q.hold})       // shift settles
	q.steps = append(q.steps, keyStep{keys: []matrixPos{shift, key}, frames: q.hold})  // add key, hold shift
	q.steps = append(q.steps, keyStep{keys: nil, frames: q.gap})                        // release + gap
	q.lastKeys = []matrixPos{shift, key}
}

// EnqueueText expands a string into key chords. Unmappable characters are
// skipped and their count returned so the caller can warn.
func (q *KeyQueue) EnqueueText(text string) (skipped int) {
	for _, r := range text {
		lower := r
		if r >= 'A' && r <= 'Z' {
			lower = r + ('a' - 'A')
		}
		if pos, ok := baseKeys[lower]; ok {
			q.enqueueChord([]matrixPos{pos})
			continue
		}
		if pos, ok := symKeys[r]; ok {
			q.enqueueShiftedChord(symShift, pos)
			continue
		}
		skipped++
	}
	return skipped
}

// EnqueueChord adds a single explicit chord. A two-key chord whose first key
// is a shift (CAPS or SYMBOL) is staged like a human shift-press; other chords
// are pressed together.
func (q *KeyQueue) EnqueueChord(keys []matrixPos) {
	if len(keys) == 2 && (keys[0] == capsShift || keys[0] == symShift) {
		q.enqueueShiftedChord(keys[0], keys[1])
		return
	}
	q.enqueueChord(keys)
}

// Step advances the queue by one frame and applies the current hold to the
// emulator's key matrix. It clears any previously-held scripted keys first by
// resetting the matrix, then pressing the current step's keys. It returns true
// while the queue is still active.
//
// Note: this resets the whole matrix each frame, which is correct for the
// headless build (no other keyboard source). The GUI build, whose HandleInput
// rewrites the matrix every frame, applies scripted keys after HandleInput so
// they win; see the GUI wiring.
func (q *KeyQueue) Step(zx *ZenZX) bool {
	if q.idx >= len(q.steps) {
		return false
	}
	if q.remaining == 0 {
		q.remaining = q.steps[q.idx].frames
	}

	// Apply the current step's held keys.
	zx.io.ResetKeyboard()
	for _, k := range q.steps[q.idx].keys {
		zx.io.PressKey(k.row, k.col)
	}

	q.remaining--
	if q.remaining == 0 {
		q.idx++
	}
	return q.idx < len(q.steps)
}

// namedKeys maps symbolic key names (for the `key` verb) to matrix chords.
var namedKeys = map[string][]matrixPos{
	"enter":  {{6, 0}},
	"space":  {{7, 0}},
	"caps":   {capsShift},
	"sym":    {symShift},
	"delete": {capsShift, {4, 0}}, // CAPS SHIFT + 0
	"edit":   {capsShift, {3, 0}}, // CAPS SHIFT + 1
	"break":  {capsShift, {7, 0}}, // CAPS SHIFT + SPACE
	"left":   {capsShift, {3, 4}}, // CAPS SHIFT + 5
	"down":   {capsShift, {4, 4}}, // CAPS SHIFT + 6
	"up":     {capsShift, {4, 3}}, // CAPS SHIFT + 7
	"right":  {capsShift, {4, 2}}, // CAPS SHIFT + 8
}
