package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zentools/pkg/tzx"
)

// ============================================================================
// Tape Types and Constants
// ============================================================================

// TapeMode represents the tape loading mode
type TapeMode int

const (
	TapeAccurate TapeMode = iota // Accurate pulse-level emulation
	TapeFast                     // Fast/instant loading
	TapeTurbo                    // Accurate pulse-level emulation, with idle busy-wait loops skipped ahead in time (see loopskip.go)
)

// tapInterBlockPauseMs is the pause inserted after every TAP block during
// accurate-mode pulse generation, in milliseconds -- confirmed as a real,
// two-way-independent convention, not one implementation's idiosyncrasy:
// SpecIde's own TAPFile::parse() pushes exactly this pattern
// (945, 3500, 3500*1000 T-states) after every block, and libspectrum's
// own tap.c has the identical comment "Give a 1s pause after each
// block". A named constant rather than a literal specifically so this is
// easy to find and change later without hunting through loadTAP's own
// logic -- promote to a real user-facing setting (settingsconfig) if it
// ever needs to be adjustable without a rebuild, but there is no
// evidence yet that it does.
const tapInterBlockPauseMs = 1000

// TapeBlock represents a single block of tape data
type TapeBlock struct {
	Data     []byte // Raw block data (including flag byte)
	IsHeader bool   // True if this is a header block

	// Header fields, valid only when IsHeader is true. Populated by
	// zentools/pkg/tap's own parser -- for a .tap file, directly from
	// tap.Decode's own Block; for a TZX 0x10 (standard-speed data)
	// block, by running tap.DecodeBlock over its Data, since a 0x10
	// payload IS a TAP-format block's raw bytes (flag+payload+checksum),
	// just carried in a TZX container. GetBlockInfo (tape.go) reads
	// these directly rather than re-deriving them from raw byte offsets
	// itself.
	HeaderType byte   // TypeProgram / TypeCode / ... (see zentools/pkg/tap)
	HeaderName string // 10-char name, trailing spaces trimmed
	DataLength uint16
}

// TapeState holds the current tape state
type TapeState struct {
	Loaded     bool        // Is a tape loaded?
	Filename   string      // Current tape filename
	Mode       TapeMode    // Current loading mode
	Playing    bool        // Is tape playing?
	Position   int         // Current position (block index or pulse index)
	EdgeOffset int         // Sub-pulse position for accurate mode
	EarLevel   bool        // Current EAR output level
	Blocks     []TapeBlock // Parsed tape blocks
	Pulses     []int       // Pulse durations in T-states (for accurate mode)

	// BlockBoundaries marks the pulse indices (keys into Pulses) at which
	// each new TZX block's own pulses begin -- populated by loadTZX as
	// each block's pulses are appended, one entry per block that
	// contributed any pulses at all. Turbo mode's fast memory/IO path
	// (loopskip.go... [see turbo.go]) uses this to synchronise its flat,
	// paging-unaware scratch memory against the real, paging-aware
	// SpectrumMemory once per block rather than once per file -- correct
	// for the overwhelmingly common case of a 128K loader switching RAM
	// banks between blocks, not mid-block (switching mid-receive, in the
	// middle of a time-sensitive bit-sampling loop, would be
	// self-defeating for the loader itself, not just rare in practice).
	// nil is a valid, common state (a TAP-sourced tape, or any tape with
	// only one contributing block) and simply means no boundary besides
	// the file's own start and end.
	BlockBoundaries []int

	// StopPoints marks pulse indices (keys into Pulses) where accurate-mode
	// playback should halt once that pulse has fully played, matching TZX's
	// own "stop the tape" signal (a 0x20 pause block with pause==0) --
	// confirmed against SpecIde's own stopData set (source/src/TZXFile.cc).
	// A genuine, distinct signal from an ordinary pause, used by real
	// multi-part loaders to mark a point where the tape should wait rather
	// than keep playing through. nil is a valid, common state (no stop
	// points recorded) and reads as "not a stop point" for any index.
	StopPoints map[int]bool

	// StopPointsIf48K is TZX's separate 0x2A ("Stop The Tape If In 48K
	// Mode") signal -- a distinct set from StopPoints, matching SpecIde's
	// own separate stopIf48K set (source/src/Tape.h: "Stop points only
	// for 48K mode"). Real multi-part loaders use this to require manual
	// intervention on memory-constrained machines while letting 128K-class
	// machines, which don't have the same constraint, play straight
	// through -- confirmed against SpecIde's own Tape::advance(): "if
	// (is48K && stopIf48K.find(pointer) ...)". No pulses accompany this
	// marker (see loadTZX's own case 0x2A): unlike an ordinary pause,
	// SpecIde's own case 0x2A adds nothing to the pulse stream, just
	// records the current position.
	StopPointsIf48K map[int]bool
}

// ============================================================================
// Main Tape Structure
// ============================================================================

// Tape manages tape loading and playback
type Tape struct {
	zx *ZenZX      // Reference to main emulator
	st *TapeState  // Current tape state
	fl *FastLoader // Fast loader implementation (optional)
}

// NewTape creates a new tape manager
func NewTape(zx *ZenZX) *Tape {
	return &Tape{
		zx: zx,
		st: &TapeState{
			Mode: TapeFast, // Default to fast mode
		},
	}
}

// ============================================================================
// Basic Tape Operations
// ============================================================================

// LoadFile loads a tape file (TAP or TZX format)
func (t *Tape) LoadFile(filename string) error {
	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Detect format and load
	if strings.HasSuffix(strings.ToLower(filename), ".tzx") {
		err = t.loadTZX(data)
	} else {
		err = t.loadTAP(data)
	}

	if err != nil {
		return err
	}

	t.st.Loaded = true
	t.st.Filename = filename
	t.st.Position = 0
	t.st.Playing = false

	return nil
}

// Play starts tape playback
func (t *Tape) Play() {
	if t.st != nil && t.st.Loaded {
		t.st.Playing = true
	}
}

// Stop stops tape playback
func (t *Tape) Stop() {
	if t.st != nil {
		t.st.Playing = false
	}
}

// Rewind resets tape to beginning
func (t *Tape) Rewind() {
	if t.st != nil {
		t.st.Position = 0
		t.st.EdgeOffset = 0
		t.st.EarLevel = false
	}
}

// SetMode sets the tape loading mode
func (t *Tape) SetMode(mode TapeMode) {
	if t.st != nil {
		t.st.Mode = mode
	}
}

// AttachFastLoader attaches a fast loader implementation
func (t *Tape) AttachFastLoader(fl *FastLoader) {
	t.fl = fl
}

// ============================================================================
// TAP Format Loading
// ============================================================================

// loadTAP loads a TAP format tape file, delegating all structural parsing
// to zentools/pkg/tap.Decode -- the verified, shared codec, replacing the
// previous in-tree byte-level parser (the same migration snapshot_formats.go
// already made for .sna/.z80 loading). This function's own remaining job is
// squarely the "timing" work: turning each decoded block into T-state pulse
// timings for accurate-mode playback.
func (t *Tape) loadTAP(data []byte) error {
	t.st.Blocks = nil
	t.st.Pulses = nil

	blocks, err := tap.Decode(data)
	// Same reasoning as loadTZX below: process whatever tap.Decode did
	// manage to parse, regardless of err, then still surface err
	// afterward -- a truncated file leaves every earlier, complete block
	// usable rather than discarding a partially-good tape entirely.
	for _, b := range blocks {
		// zentools/pkg/tap splits a block into Flag/Data/Checksum rather
		// than keeping the combined raw bytes; genPulses (and TapeBlock.Data
		// generally) want the whole thing back together, matching what this
		// package always stored here.
		raw := make([]byte, 0, len(b.Data)+2)
		raw = append(raw, b.Flag)
		raw = append(raw, b.Data...)
		raw = append(raw, b.Checksum)

		t.st.Blocks = append(t.st.Blocks, TapeBlock{
			Data:       raw,
			IsHeader:   b.IsHeader, // zentools' own criterion (flag 0x00, length 19) matches this package's previous one exactly
			HeaderType: b.Type,
			HeaderName: b.Name,
			DataLength: b.DataLength,
		})

		pulses := t.genPulses(raw)
		t.st.Pulses = append(t.st.Pulses, pulses...)

		// Confirmed as a real, two-way-independent convention -- see
		// tapInterBlockPauseMs's own doc comment -- not something this
		// package invented: SpecIde and libspectrum both insert the
		// same pause after every TAP block, not just between some.
		t.addTZXPause(tapInterBlockPauseMs)
	}

	if err != nil {
		return fmt.Errorf("TAP file loaded partially (%d block(s)): %w", len(blocks), err)
	}
	return nil
}

// ============================================================================
// TZX Format Loading
// ============================================================================

// loadTZX loads a TZX format tape file, delegating all structural parsing
// to zentools/pkg/tzx.Decode. zentools' own decoder recognises all 25
// SpecIde-documented TZX block types.
//
// Wiring status of this switch (Blocks feeds trapLoad's fast-load,
// Pulses feeds accurate-mode playback):
//   - 0x10 (Standard Speed): both -- including its own trailing pause.
//   - 0x11 (Turbo Speed) / 0x14 (Pure Data): both -- trapLoad (tape.go)
//     only ever reads Blocks[i].Data and is completely pulse/timing-
//     agnostic, so Blocks alone is enough for fast-load; genCustomPulses
//     below covers accurate-mode playback using each block's own custom
//     pilot/sync/data timing, distinct from 0x10's fixed standard
//     values.
//   - 0x12 (Pure Tone) / 0x13 (Sequence Of Pulses): Pulses only --
//     zentools already computes these as ready-to-use pulse data (a
//     scalar PilotPulse/PilotLength pair for 0x12, expanded here the
//     same way SpecIde's own case 0x12 does inline; an already-expanded
//     array for 0x13). Neither carries loadable byte data, so neither
//     reaches Blocks.
//   - 0x20 (Pause / Stop The Tape): both, including its polarity-reset
//     handling and the distinct pause==0 "stop the tape" signal.
//   - 0x2A (Stop The Tape If In 48K Mode): a stop marker only, no
//     pulses -- see StopPointsIf48K's own doc comment on TapeState.
//   - Every other recognised block type is safely, correctly parsed and
//     skipped by zentools itself, but does not yet reach Blocks or
//     Pulses -- a natural, separately-scoped follow-up, not a
//     regression: none of them carry loadable data, and none currently
//     appear in real-world tapes tested against (see the tape
//     compatibility report), so this is a genuine gap in breadth, not
//     one that has been observed to affect any real game.
func (t *Tape) loadTZX(data []byte) error {
	t.st.Blocks = nil
	t.st.Pulses = nil

	blocks, err := tzx.Decode(data)
	// zentools/pkg/tzx.Decode returns whatever it successfully parsed
	// alongside an error, not just nil -- e.g. an unrecognised block ID
	// partway through a file still leaves every earlier block usable. The
	// previous in-tree parser's own spirit (keep what could be parsed,
	// stop cleanly at the point that couldn't) is preserved by processing
	// blocks regardless of err, then still returning err afterward so the
	// caller knows loading was incomplete -- an improvement over the old
	// parser, which never surfaced this at all, not a stricter regression
	// that would throw away an otherwise-good tape over one bad block.
	for _, b := range blocks {
		// Mark this block's own boundary in the pulse stream before any
		// of its pulses are appended -- but only for block types that
		// actually contribute pulses (see BlockBoundaries' own doc
		// comment on TapeState). A single, centralised check here rather
		// than duplicating this inside each case below, so it can never
		// drift out of sync with which cases actually append to Pulses.
		switch b.ID {
		case 0x10, 0x11, 0x12, 0x13, 0x14, 0x20:
			t.st.BlockBoundaries = append(t.st.BlockBoundaries, len(t.st.Pulses))
		}

		switch b.ID {
		case 0x10: // Standard Speed Data
			// A 0x10 payload IS a TAP-format block's raw bytes
			// (flag+payload+checksum) -- tap.DecodeBlock gives the same
			// parsed header fields loadTAP gets for an actual .tap file,
			// rather than this switch re-deriving IsHeader by hand.
			tapBlock, tapErr := tap.DecodeBlock(b.Data)
			tb := TapeBlock{Data: b.Data}
			if tapErr == nil {
				tb.IsHeader = tapBlock.IsHeader
				tb.HeaderType = tapBlock.Type
				tb.HeaderName = tapBlock.Name
				tb.DataLength = tapBlock.DataLength
			}
			t.st.Blocks = append(t.st.Blocks, tb)
			pulses := t.genPulses(b.Data)
			t.st.Pulses = append(t.st.Pulses, pulses...)

			// The block's own trailing pause, via addTZXPause's
			// spec-correct explicit-level encoding (hold ~1ms, LOW for
			// the remainder, next block starts LOW). The earlier
			// SpecIde-shape pattern this replaced is documented at
			// addTZXPause itself.
			if b.Pause > 0 {
				t.addTZXPause(int(b.Pause))
			}

		case 0x11, 0x14: // Turbo Speed Data / Pure Data
			// Both carry the same flag+payload+checksum shape 0x10 does
			// (0x11 with its own custom pilot/sync/data timing; 0x14
			// with no pilot/sync at all, a continuation block after a
			// separately-declared 0x12/0x13). trapLoad (tape.go) only
			// ever reads t.st.Blocks[i].Data -- it is completely
			// pulse/timing-agnostic -- so wiring these into Blocks alone
			// is enough for fast-load. genCustomPulses now also gives
			// these two real accurate-mode playback, using the block's
			// own custom timing fields rather than 0x10's fixed
			// standard ones. 0x11 is by far the more commercially
			// important of the two: most commercial games' custom
			// loaders use it for their main data.
			tapBlock, tapErr := tap.DecodeBlock(b.Data)
			tb := TapeBlock{Data: b.Data}
			if tapErr == nil {
				tb.IsHeader = tapBlock.IsHeader
				tb.HeaderType = tapBlock.Type
				tb.HeaderName = tapBlock.Name
				tb.DataLength = tapBlock.DataLength
			}
			t.st.Blocks = append(t.st.Blocks, tb)

			pulses := t.genCustomPulses(b)
			t.st.Pulses = append(t.st.Pulses, pulses...)
			t.addTZXPause(int(b.Pause))

		case 0x2A: // Stop The Tape If In 48K Mode
			// A stop marker only -- no pulses accompany it. Confirmed
			// against SpecIde's own case 0x2A (source/src/TZXFile.cc):
			// "stopIf48K.insert(pulseData.size())", nothing else.
			if t.st.StopPointsIf48K == nil {
				t.st.StopPointsIf48K = make(map[int]bool)
			}
			t.st.StopPointsIf48K[len(t.st.Pulses)] = true

		case 0x12: // Pure Tone
			// Unlike 0x13 below, zentools' own Block does NOT expand
			// this into a Pulses array -- it stores PilotPulse/
			// PilotLength as two scalar fields (Block's own doc
			// comment). The repeat-N-times expansion below is exactly
			// what SpecIde's own case 0x12 does inline
			// ("pulseData.insert(pulseData.end(), pilotLength,
			// pilotPulse)") -- confirmed no trailing pause or other
			// logic exists for this block type there either.
			for i := uint16(0); i < b.PilotLength; i++ {
				t.st.Pulses = append(t.st.Pulses, int(b.PilotPulse))
			}

		case 0x13: // Sequence Of Pulses
			// Already a direct pulse array in zentools' own
			// Block.Pulses -- confirmed against SpecIde's own case 0x13:
			// no trailing pause or other logic, just the raw pulse
			// values pushed as-is. No pulse-generation work here, only
			// plumbing what zentools already computed into playback.
			t.st.Pulses = append(t.st.Pulses, b.Pulses...)

		case 0x20: // Pause / Stop The Tape
			// SpecIde's parity-reset preamble ("if ((pulseData.size()
			// % 2) == 0) { addPause(1, ...) }") is deliberately NOT
			// carried over: it existed to steer the entry level of a
			// parity-dependent pause encoding. addTZXPause now derives
			// the entry level itself and forces the pause LOW per the
			// TZX spec, so the dance is obsolete.
			if b.Pause > 0 {
				t.addTZXPause(int(b.Pause))
			} else {
				// pause == 0 is TZX's own "stop the tape" signal, a
				// genuine, distinct meaning from "no pause" -- real
				// multi-part loaders use this to mark a point where
				// playback should halt and wait, not keep playing
				// through. Confirmed against SpecIde: a fixed 1000ms
				// settling pause, then the final pulse of that pattern
				// is recorded as a stop point.
				t.addTZXPause(1000)
				if t.st.StopPoints == nil {
					t.st.StopPoints = make(map[int]bool)
				}
				t.st.StopPoints[len(t.st.Pulses)-1] = true
			}

		default:
			// Recognised and correctly skipped by zentools/pkg/tzx itself;
			// no pulses generated for these yet (matches this package's
			// scope prior to this migration -- see this function's own
			// doc comment).
		}
	}

	if err != nil {
		return fmt.Errorf("TZX file loaded partially (%d block(s)): %w", len(blocks), err)
	}
	return nil
}

// ============================================================================
// Pulse Generation
// ============================================================================

// addTZXPause encodes a TZX pause per the spec and libspectrum's
// END_OF_BLOCK_NEXT_LOW semantics: the current level holds for ~1ms,
// the line then sits LOW for the remainder, and the NEXT block starts
// from LOW. In a toggle-per-pulse stream the level during the pulse
// about to be appended at index n is HIGH iff n is odd (the stream
// starts LOW and flips at every pulse end), so the entry level is
// derived, not tracked. A zero-length pulse is a pure level flip
// (Tick consumes it in 0 T-states); it restores the parity that
// guarantees the following block plays its first pulse at LOW.
//
// This replaces the SpecIde-derived {945, 3500, 3500*ms} pattern,
// whose long segment sat at whatever level it happened to enter on:
// measured on Batman's stream, four of its five long pauses -- among
// them the 3587ms one at exactly the tape Position where its
// Speedlock loader was first observed stalling -- played entirely
// HIGH, where FUSE presents LOW.
func (t *Tape) addTZXPause(pauseMs int) {
	if pauseMs == 0 {
		return
	}
	entryHigh := len(t.st.Pulses)%2 == 1
	if entryHigh {
		if pauseMs == 1 {
			// 1ms at the current level; the flip at its end lands the
			// stream on LOW for whatever follows.
			t.st.Pulses = append(t.st.Pulses, 3500)
			return
		}
		// 1ms at the current HIGH, the remainder LOW, then a
		// zero-length flip so the next block starts at LOW.
		t.st.Pulses = append(t.st.Pulses, 3500, 3500*(pauseMs-1), 0)
		return
	}
	// Already LOW: the whole pause sits LOW; the zero-length flip
	// restores even parity so the next block also starts at LOW.
	t.st.Pulses = append(t.st.Pulses, 3500*pauseMs, 0)
}

// genCustomPulses generates accurate-mode pulse timings for a block
// carrying its own custom pilot/sync/data pulse lengths -- 0x11 (Turbo
// Speed Data, with a pilot and sync) and 0x14 (Pure Data, a
// continuation block with neither). Unlike genPulses below (0x10's own,
// fixed-timing generator), every pulse length here comes from the block
// itself, and the final byte may use fewer than 8 bits.
//
// Algorithm confirmed against SpecIde's own case 0x11 and case 0x14
// (source/src/TZXFile.cc), not assumed from the general shape of
// genPulses: pilot tone repeated PilotLength times at PilotPulse
// T-states (0x11 only -- 0x14 has neither PilotPulse/PilotLength nor
// SyncPulse1/SyncPulse2, both zero-valued on a zentools Block decoded
// from a 0x14, so the pilot/sync loop below is naturally a no-op for
// that case without needing a separate branch), two sync pulses (0x11
// only), then per-byte bit encoding at DataPulse0/DataPulse1 T-states,
// the final byte using BitsInLastByte bits instead of a full 8, then
// the block's own trailing pause via addTZXPause (spec-correct
// explicit-level encoding, shared with 0x10 and 0x20).
func (t *Tape) genCustomPulses(b tzx.Block) []int {
	var pulses []int

	for i := uint16(0); i < b.PilotLength; i++ {
		pulses = append(pulses, int(b.PilotPulse))
	}
	if b.PilotLength > 0 {
		pulses = append(pulses, int(b.SyncPulse1), int(b.SyncPulse2))
	}

	for i, byteVal := range b.Data {
		bitsInByte := 8
		if i == len(b.Data)-1 {
			bitsInByte = int(b.BitsInLastByte)
		}
		v := byteVal
		for bit := 0; bit < bitsInByte; bit++ {
			if v&0x80 != 0 {
				pulses = append(pulses, int(b.DataPulse1), int(b.DataPulse1))
			} else {
				pulses = append(pulses, int(b.DataPulse0), int(b.DataPulse0))
			}
			v <<= 1
		}
	}

	// The block's trailing pause is deliberately NOT emitted here: its
	// spec-correct encoding depends on the entry level, i.e. on the
	// parity of the FULL stream at append time, which only the caller
	// knows. The caller appends this slice and then calls addTZXPause.
	// (An earlier version inlined the pre-fix SpecIde triple here --
	// the one site the pause-level fix missed. Found by the static
	// stream audit: its 1ms HIGH excursion and +1 parity shift
	// inverted the polarity of everything after Batman's block #23,
	// including the entire 47KB main Speedlock payload.)
	return pulses
}

// genPulses generates pulse timings for a data block
func (t *Tape) genPulses(data []byte) []int {
	var pulses []int

	if len(data) == 0 {
		return pulses
	}

	// Determine pilot length from the flag byte's top bit, not an exact
	// match against 0x00. Confirmed against MartianGirl's SpecIde
	// (github.com/MartianGirl/SpecIde) independently in both
	// source/src/TZXFile.cc and source/src/TAPFile.cc: "pilotLength =
	// (flagByte & 0x80) ? 3223 : 8063". The standard ROM header (0x00)
	// and data (0xFF) flags satisfy an exact-0x00 check the same way
	// they satisfy the bit-7 check, which is why this only bites custom
	// loaders using a flag byte in 0x01-0x7F -- bit 7 clear, so it
	// should get the long header-style pilot, but an exact-match check
	// against 0x00 alone would wrongly give it the short one.
	pilotLen := 3223 // bit 7 set: data-style pilot
	if data[0]&0x80 == 0 {
		pilotLen = 8063 // bit 7 clear: header-style pilot
	}

	// Pilot tone (2168 T-states per pulse)
	for i := 0; i < pilotLen; i++ {
		pulses = append(pulses, 2168)
	}

	// Sync pulses
	pulses = append(pulses, 667) // First sync
	pulses = append(pulses, 735) // Second sync

	// Data pulses
	for _, b := range data {
		for bit := 7; bit >= 0; bit-- {
			if (b>>bit)&1 == 1 {
				// One bit: two pulses of 1710 T-states
				pulses = append(pulses, 1710, 1710)
			} else {
				// Zero bit: two pulses of 855 T-states
				pulses = append(pulses, 855, 855)
			}
		}
	}

	return pulses
}
