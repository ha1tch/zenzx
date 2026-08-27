package main

import (
	"strings"
	"testing"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zentools/pkg/tzx"
)

// TestLoadTZX_UnknownBlockID_SurfacesErrorButKeepsGoodBlocks now exercises
// loadTZX's own behaviour after its structural parsing was migrated to
// zentools/pkg/tzx.Decode -- the in-tree parser this test originally
// caught a real break-vs-loop bug in (five sites treating a bare `break`
// inside a switch as if it exited the enclosing loop; see the historical
// git record) no longer exists in this package at all, so that specific
// bug class cannot recur here regardless of what this test checks.
//
// What is still worth verifying: zentools' Decode returns a genuine error
// for an unrecognised block ID (stricter than the old silent-stop
// behaviour, which never told a caller anything had gone wrong), and
// loadTZX both surfaces that error *and* still keeps whatever was
// successfully parsed before it, rather than discarding an otherwise-good
// tape entirely over one bad trailing block.
func TestLoadTZX_UnknownBlockID_SurfacesErrorButKeepsGoodBlocks(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)

	// One genuine, valid 0x10 block.
	data = append(data, 0x10, 0x00, 0x00, 0x01, 0x00, 0x00)

	// An unrecognised block ID -- zentools/pkg/tzx.Decode reports this as
	// an error rather than guessing at a length to skip.
	data = append(data, 0xFF)

	tp := &Tape{st: &TapeState{}}
	err := tp.loadTZX(data)
	if err == nil {
		t.Fatal("expected an error for the unrecognised block ID, got none")
	}

	if len(tp.st.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1 -- the one valid block before the "+
			"unrecognised ID should still be kept despite the error", len(tp.st.Blocks))
	}
}

// TestGenPulses_PilotLengthUsesFlagBit7 is the regression test for a real
// bug: genPulses decided pilot-tone length by an exact match against flag
// byte 0x00 (data[0] == 0x00), when the correct criterion -- confirmed
// directly against MartianGirl's SpecIde, github.com/MartianGirl/SpecIde,
// independently in both source/src/TZXFile.cc ("pilotLength = (flagByte &
// 0x80) ? 3223 : 8063") and source/src/TAPFile.cc (the same expression,
// same two constants) -- is the top bit of the flag byte: any flag with
// bit 7 clear (0x00-0x7F, not just the conventional 0x00) gets the long
// "header-style" pilot (8063 pulses); any flag with bit 7 set (0x80-0xFF,
// not just the conventional 0xFF) gets the short "data-style" pilot
// (3223). The standard ROM header (0x00) and data (0xFF) flags happen to
// agree with both criteria, which is exactly why this bug would not show
// up on ordinary tapes -- it only bites custom loaders using a flag byte
// in 0x01-0x7F, which the exact-match version misclassifies as
// data-style despite bit 7 being clear.
func TestGenPulses_PilotLengthUsesFlagBit7(t *testing.T) {
	tp := &Tape{st: &TapeState{}}

	cases := []struct {
		name       string
		flag       byte
		wantPilots int
	}{
		{"standard header flag 0x00", 0x00, 8063},
		{"standard data flag 0xFF", 0xFF, 3223},
		{"custom flag 0x01, bit 7 clear -> header-style pilot", 0x01, 8063},
		{"custom flag 0x7F, bit 7 clear -> header-style pilot", 0x7F, 8063},
		{"custom flag 0x80, bit 7 set -> data-style pilot", 0x80, 3223},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pulses := tp.genPulses([]byte{c.flag, 0x00})
			got := 0
			for _, p := range pulses {
				if p != 2168 {
					break
				}
				got++
			}
			if got != c.wantPilots {
				t.Errorf("flag 0x%02X: got %d pilot pulses, want %d", c.flag, got, c.wantPilots)
			}
		})
	}
}

// tzxHeaderAnd is a small test helper: a minimal 10-byte valid TZX header
// followed by whatever block bytes the caller supplies.
func tzxHeaderAnd(blockBytes ...byte) []byte {
	data := []byte("ZXTape!")
	data = append(data, 0x1A, 1, 20)
	data = append(data, blockBytes...)
	return data
}

// le16 encodes v as two little-endian bytes, for building TZX block
// headers by hand in tests.
func le16(v uint16) []byte {
	return []byte{byte(v), byte(v >> 8)}
}

// le24 encodes v (must fit in 24 bits) as three little-endian bytes.
func le24(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16)}
}

// le32 encodes v as four little-endian bytes.
func le32(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

// TestLoadTZX_PauseBlock_UsesSpecIdeThreePulsePattern is the regression
// test for a real gap: the 0x20 (pause) block appended a single flat
// pause*3500 pulse, when SpecIde's own addPause (source/src/TZXFile.cc)
// always emits three: {945, 3500, 3500*pauseMs} -- a short settling
// pattern before the actual requested silence, not the silence alone.
// walkLevels replays a pulse stream's implicit toggling (initial level
// LOW, flip at each pulse end) and returns, for each pulse, the level
// it plays at.
func walkLevels(pulses []int) []bool {
	levels := make([]bool, len(pulses))
	level := false
	for i := range pulses {
		levels[i] = level
		level = !level
	}
	return levels
}

// TestLoadTZX_PauseBlock_SitsLowPerSpec asserts the pause SEMANTICS the
// TZX spec and libspectrum (END_OF_BLOCK_NEXT_LOW) require, not any
// particular pulse-triple shape: the dominant part of a pause plays
// LOW, its total duration is the requested one, and the stream's parity
// afterwards guarantees the NEXT block starts at LOW. (The predecessor
// of this test asserted the SpecIde {945, 3500, 3500*ms} pattern by
// shape -- an encoding whose long segment sat at whatever level it
// entered on, measured playing entirely HIGH through four of Batman's
// five pauses.)
func TestLoadTZX_PauseBlock_SitsLowPerSpec(t *testing.T) {
	data := tzxHeaderAnd(append([]byte{0x20}, le16(100)...)...)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	total := 0
	levels := walkLevels(tp.st.Pulses)
	for i, p := range tp.st.Pulses {
		total += p
		if p >= 3500 && levels[i] {
			t.Errorf("pulse %d (%d T-states) plays HIGH; every >=1ms part of a pause must sit LOW", i, p)
		}
	}
	if total != 3500*100 {
		t.Errorf("pause total = %d T-states, want %d (100ms)", total, 3500*100)
	}
	if len(tp.st.Pulses)%2 != 0 {
		t.Errorf("stream parity after pause is odd: the next block would start HIGH, spec requires LOW")
	}
}

// TestLoadTZX_PauseBlock_HighEntryHoldsHeadThenLow covers the
// HIGH-entry half of the spec: a pause entered at HIGH level (here via
// a 3-pulse Pure Tone, odd count) holds the current level for ~1ms,
// then the remainder plays LOW, and the following block still starts at
// LOW. (This replaces the PolarityReset test: SpecIde's parity-reset
// preamble existed only to steer a parity-dependent encoding's entry
// level, and was removed along with that encoding.)
func TestLoadTZX_PauseBlock_HighEntryHoldsHeadThenLow(t *testing.T) {
	blocks := append([]byte{0x12}, le16(2168)...) // Pure Tone, 3 pulses: odd, HIGH entry into the pause
	blocks = append(blocks, le16(3)...)
	blocks = append(blocks, 0x20)
	blocks = append(blocks, le16(50)...)
	data := tzxHeaderAnd(blocks...)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}
	if len(tp.st.Pulses) < 5 {
		t.Fatalf("got %d pulses %v, want the 3 tone pulses plus the pause encoding", len(tp.st.Pulses), tp.st.Pulses)
	}

	levels := walkLevels(tp.st.Pulses)
	pause := tp.st.Pulses[3:]
	pauseLevels := levels[3:]
	total := 0
	for i, p := range pause {
		total += p
		if p >= 3500*2 && pauseLevels[i] {
			t.Errorf("pause segment %d (%d T-states) plays HIGH; only the ~1ms head may", i, p)
		}
	}
	if !pauseLevels[0] || pause[0] != 3500 {
		t.Errorf("pause head = %d T-states at level=%v, want exactly 1ms held at the entry HIGH level", pause[0], pauseLevels[0])
	}
	if total != 3500*50 {
		t.Errorf("pause total = %d T-states, want %d (50ms)", total, 3500*50)
	}
	if len(tp.st.Pulses)%2 != 0 {
		t.Errorf("stream parity after pause is odd: the next block would start HIGH, spec requires LOW")
	}
}

// TestLoadTZX_PauseBlock_ZeroMeansStopTheTape is the regression test for
// the more serious gap: pause==0 in a 0x20 block is TZX's own "stop the
// tape" signal -- used by real, correctly-authored multi-part loaders to
// mark a genuine halt point -- not "no pause, do nothing". The previous
// code (`if pause > 0 {...}`) silently dropped it entirely. Confirmed
// against SpecIde: a fixed 1000ms settling pause, then the position is
// recorded as a stop point (its own set, stopData, in SpecIde's model).
func TestLoadTZX_PauseBlock_ZeroMeansStopTheTape(t *testing.T) {
	data := tzxHeaderAnd(append([]byte{0x20}, le16(0)...)...)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	if len(tp.st.Pulses) == 0 {
		t.Fatalf("got no pulses; want the fixed 1000ms settling pause")
	}
	total := 0
	levels := walkLevels(tp.st.Pulses)
	for i, p := range tp.st.Pulses {
		total += p
		if p >= 3500 && levels[i] {
			t.Errorf("pulse %d (%d T-states) plays HIGH; the settling pause must sit LOW", i, p)
		}
	}
	if total != 3500*1000 {
		t.Errorf("settling pause total = %d T-states, want %d (the fixed 1000ms)", total, 3500*1000)
	}

	stopIdx := len(tp.st.Pulses) - 1
	if !tp.st.StopPoints[stopIdx] {
		t.Errorf("final pulse (index %d) is not marked as a stop point: playback must halt only after the full settling pause has played", stopIdx)
	}
}

// TestTick_HaltsPlaybackAtStopPoint confirms the stop point recorded by
// a pause==0 block actually halts accurate-mode playback once reached,
// not just that it is recorded. The stop point is deliberately placed
// mid-stream (index 2 of 5), not at the final pulse -- if it were at the
// last index, halting there would be indistinguishable from the
// pre-existing "ran off the end of Pulses" behaviour Tick already had,
// and this test would not actually prove StopPoints is doing anything.
// Requesting far more cycles than needed to reach the stop point (500,
// when only 300 are needed) and confirming Position stops at exactly 3,
// with unconsumed cycles implied, is what proves it halted there rather
// than merely running out of cycles to advance further.
func TestTick_HaltsPlaybackAtStopPoint(t *testing.T) {
	zx := &ZenZX{}
	tp := &Tape{
		zx: zx,
		st: &TapeState{
			Loaded:  true,
			Playing: true,
			Mode:    TapeAccurate,
			Pulses:  []int{100, 100, 100, 100, 100},
			StopPoints: map[int]bool{
				2: true, // mid-stream, not the last pulse
			},
		},
	}

	tp.Tick(500) // far more than the 300 needed to reach the stop point

	if tp.st.Playing {
		t.Error("Playing is still true after reaching a marked stop point -- playback should have halted")
	}
	if tp.st.Position != 3 {
		t.Errorf("Position = %d, want 3 (halted right after the stop-point pulse, not run to the end of Pulses at 5)", tp.st.Position)
	}
}

// TestLoadTZX_StandardBlock_TrailingPauseUsesSpecIdeThreePulsePattern is
// the regression test for a real bug found while migrating loadTZX's
// structural parsing to zentools/pkg/tzx.Decode: the previous in-tree
// parser's own 0x10 case appended a single flat pause*3500 pulse for its
// trailing pause, when SpecIde's own case 0x10 (source/src/TZXFile.cc)
// uses the same three-pulse addPause pattern {945, 3500, 3500*pauseMs} as
// 0x20 does -- confirmed directly by reading SpecIde's own case 0x10 body
// ("indexData.insert(...); addPause(pause, pulseData);"), not assumed
// from the 0x20 fix generalising. Unlike 0x20, SpecIde does not precede
// this with a polarity-reset check for 0x10 specifically -- also checked
// directly, not assumed, and preserved here deliberately.
func TestLoadTZX_StandardBlock_TrailingPauseSitsLowPerSpec(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)
	// 0x10 block: pause=200ms, length=1, one data byte (0xFF, flag byte
	// with bit 7 set -- data-style pilot, keeping the pilot tone short
	// enough that this test is not dominated by thousands of 2168 pulses).
	data = append(data, 0x10, 0xC8, 0x00, 0x01, 0x00, 0xFF)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	// Semantic contract, not shape: the block's 200ms trailing pause
	// must dominate the stream's longest pulse, play LOW, and leave
	// even parity so the next block starts LOW. (HIGH-entry encodes as
	// {3500, 3500*199, 0}, LOW-entry as {3500*200, 0}; both satisfy
	// this.)
	levels := walkLevels(tp.st.Pulses)
	longest, longestIdx := 0, -1
	for i, p := range tp.st.Pulses {
		if p > longest {
			longest, longestIdx = p, i
		}
	}
	if longest < 3500*199 {
		t.Fatalf("longest pulse = %d T-states, want >= %d (the pause's LOW body)", longest, 3500*199)
	}
	if levels[longestIdx] {
		t.Errorf("the pause's %d T-state body plays HIGH, want LOW", longest)
	}
	if len(tp.st.Pulses)%2 != 0 {
		t.Errorf("stream parity after the trailing pause is odd: the next block would start HIGH, spec requires LOW")
	}
}

// TestGetBlockInfo_TAP confirms GetBlockInfo's header line uses the
// HeaderType/HeaderName/DataLength fields tape_types.go populates from
// zentools/pkg/tap's own parser at load time, for an actual .tap file
// -- not re-derived from raw byte offsets.
func TestGetBlockInfo_TAP(t *testing.T) {
	img := tap.EncodeCode("MYCODE", []byte{1, 2, 3, 4}, 0x8000)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTAP(img); err != nil {
		t.Fatalf("loadTAP: %v", err)
	}
	tp.st.Loaded = true

	info := tp.GetBlockInfo()
	if len(info) != 2 {
		t.Fatalf("got %d info lines, want 2 (header + data)", len(info))
	}
	if !strings.Contains(info[0], "MYCODE") || !strings.Contains(info[0], "Code") || !strings.Contains(info[0], "4 bytes") {
		t.Errorf("header line = %q, want it to mention MYCODE, Code, and 4 bytes", info[0])
	}
	if !strings.Contains(info[1], "Data") {
		t.Errorf("data line = %q, want it to mention Data", info[1])
	}
}

// TestGetBlockInfo_TZX confirms the same thing for a TZX 0x10 block --
// the path that goes through tap.DecodeBlock (tape_types.go) rather than
// tap.Decode directly, since a 0x10 payload is TAP-format bytes carried
// in a TZX container.
func TestGetBlockInfo_TZX(t *testing.T) {
	// A minimal, valid TZX header block wrapping the same TAP-format
	// header bytes EncodeCode itself would produce for a header block.
	tapImg := tap.EncodeCode("TESTNAME", []byte{9}, 0x9000)
	// tapImg is [len_lo len_hi <19-byte header block>] [len_lo len_hi <data block>].
	headerRaw := tapImg[2:21] // the 19-byte header block's own raw bytes

	data := []byte("ZXTape!")
	data = append(data, 0x1A, 1, 20)
	data = append(data, 0x10)                       // Standard Speed Data
	data = append(data, 0x00, 0x00)                 // pause
	data = append(data, byte(len(headerRaw)), 0x00) // length (19, fits in one byte)
	data = append(data, headerRaw...)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}
	tp.st.Loaded = true

	info := tp.GetBlockInfo()
	if len(info) != 1 {
		t.Fatalf("got %d info lines, want 1", len(info))
	}
	if !strings.Contains(info[0], "TESTNAME") || !strings.Contains(info[0], "Code") {
		t.Errorf("header line = %q, want it to mention TESTNAME and Code", info[0])
	}
}

// TestLoadTAP_InsertsInterBlockPause is the regression test for a real,
// confirmed gap: loadTAP never inserted any pause between blocks, when
// both SpecIde (TAPFile::parse()) and libspectrum (tap.c, "Give a 1s
// pause after each block") independently insert exactly the same
// three-pulse settling+pause pattern -- {945, 3500, 3500*1000} T-states
// -- after every TAP block. Confirmed as a genuine, two-way-independent
// convention before fixing, not assumed from one implementation alone.
func TestLoadTAP_InsertsInterBlockPause(t *testing.T) {
	img := tap.EncodeCode("X", []byte{1}, 0x8000) // header block + one-byte data block

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTAP(img); err != nil {
		t.Fatalf("loadTAP: %v", err)
	}

	// Semantic check: one LOW pause body of (at least) 999ms after each
	// of the two blocks, and even final parity so anything following
	// would start LOW. The exact segment shapes depend on each pause's
	// entry level (see addTZXPause).
	levels := walkLevels(tp.st.Pulses)
	count := 0
	for i, p := range tp.st.Pulses {
		if p >= 3500*(tapInterBlockPauseMs-1) {
			if levels[i] {
				t.Errorf("pause body at pulse %d (%d T-states) plays HIGH, want LOW", i, p)
			}
			count++
		}
	}
	if count != 2 {
		t.Errorf("found %d pause bodies of >=999ms, want 2 (once after each of the two blocks)", count)
	}
	if len(tp.st.Pulses)%2 != 0 {
		t.Errorf("final stream parity is odd: a following block would start HIGH, spec requires LOW")
	}
}

// TestLoadTZX_TurboSpeedData_ReachesBlocksForFastLoad is the regression
// test for a real, high-value gap: trapLoad (tape.go) only ever reads
// t.st.Blocks[i].Data -- it is completely pulse/timing-agnostic already,
// happy to fast-load any block shaped like flag+payload+checksum
// regardless of which TZX block ID it came from. But loadTZX's switch
// only ever populated t.st.Blocks for 0x10, silently dropping every
// other block type entirely -- including 0x11 (Turbo Speed Data), the
// single most commercially common non-standard block (most commercial
// games' custom loaders use it for their main data). A real TZX file
// built around 0x11 blocks could not fast-load, or even accurate-mode
// play, at all: not because the decoder could not handle it (it always
// could -- zentools/pkg/tzx has supported 0x11 since early in this
// effort), but because nothing wired its data into the block list
// trapLoad actually reads.
func TestLoadTZX_TurboSpeedData_ReachesBlocksForFastLoad(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)

	data = append(data, 0x11)
	data = append(data, le16(2168)...) // pilotPulse
	data = append(data, le16(667)...)  // syncPulse1
	data = append(data, le16(735)...)  // syncPulse2
	data = append(data, le16(855)...)  // dataPulse0
	data = append(data, le16(1710)...) // dataPulse1
	data = append(data, le16(100)...)  // pilotLength (kept small: this test does not care about pulses)
	data = append(data, 8)             // bitsInLastByte
	data = append(data, le16(0)...)    // pause
	data = append(data, le24(3)...)    // dataLength
	data = append(data, 0xFF, 0x42, 0xBD)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	if len(tp.st.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1 -- the 0x11 block's data never reached t.st.Blocks, "+
			"so trapLoad could never fast-load it", len(tp.st.Blocks))
	}
	want := []byte{0xFF, 0x42, 0xBD}
	if string(tp.st.Blocks[0].Data) != string(want) {
		t.Errorf("Blocks[0].Data = %v, want %v", tp.st.Blocks[0].Data, want)
	}
}

// TestLoadTZX_PureData_ReachesBlocksForFastLoad is 0x11's sibling test:
// 0x14 (Pure Data) is the natural companion block, used for continuation
// data after a separately-declared pilot/sync (0x12/0x13), and has the
// same gap for the same reason.
func TestLoadTZX_PureData_ReachesBlocksForFastLoad(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)

	data = append(data, 0x14)
	data = append(data, le16(855)...)  // dataPulse0
	data = append(data, le16(1710)...) // dataPulse1
	data = append(data, 8)             // bitsInLastByte
	data = append(data, le16(0)...)    // pause
	data = append(data, le24(2)...)    // dataLength
	data = append(data, 0x11, 0x22)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	if len(tp.st.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(tp.st.Blocks))
	}
	want := []byte{0x11, 0x22}
	if string(tp.st.Blocks[0].Data) != string(want) {
		t.Errorf("Blocks[0].Data = %v, want %v", tp.st.Blocks[0].Data, want)
	}
}

// TestLoadTZX_PureTone_PulsesReachPlayback and its 0x13 sibling below are
// the cheap half of this stage: zentools/pkg/tzx already computes a
// direct pulse array for these two block types (Block.Pulses) -- there
// is no pulse-generation logic to write, only plumbing to wire what
// zentools already produced into t.st.Pulses for accurate-mode playback.
func TestLoadTZX_PureTone_PulsesReachPlayback(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)
	data = append(data, 0x12)
	data = append(data, le16(2168)...) // pilotPulse
	data = append(data, le16(5)...)    // pilotLength -- deliberately small and exact for this test

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	want := []int{2168, 2168, 2168, 2168, 2168}
	if len(tp.st.Pulses) != len(want) {
		t.Fatalf("got %d pulses %v, want %v", len(tp.st.Pulses), tp.st.Pulses, want)
	}
	for i := range want {
		if tp.st.Pulses[i] != want[i] {
			t.Errorf("Pulses[%d] = %d, want %d", i, tp.st.Pulses[i], want[i])
		}
	}
}

func TestLoadTZX_PulseSequence_PulsesReachPlayback(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)
	data = append(data, 0x13)
	data = append(data, 3) // 3 pulses follow
	data = append(data, le16(667)...)
	data = append(data, le16(735)...)
	data = append(data, le16(954)...)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	want := []int{667, 735, 954}
	if len(tp.st.Pulses) != len(want) {
		t.Fatalf("got %d pulses %v, want %v", len(tp.st.Pulses), tp.st.Pulses, want)
	}
	for i := range want {
		if tp.st.Pulses[i] != want[i] {
			t.Errorf("Pulses[%d] = %d, want %d", i, tp.st.Pulses[i], want[i])
		}
	}
}

// TestGenCustomPulses_TurboSpeedData confirms accurate-mode pulse
// generation for 0x11 (Turbo Speed Data): the block's own custom pilot/
// sync/data timing, not 0x10's fixed standard values, with the last
// byte using only BitsInLastByte bits (a real, common case for custom
// loaders -- not always a full 8). Algorithm confirmed against SpecIde's
// own case 0x11 (source/src/TZXFile.cc): pilot tone repeated
// PilotLength times, two sync pulses, then per-byte bit encoding, the
// final byte using BitsInLastByte instead of 8, then the block's own
// trailing pause via the same addTZXPause three-pulse pattern as 0x10.
func TestGenCustomPulses_TurboSpeedData(t *testing.T) {
	tp := &Tape{st: &TapeState{}}
	b := tzx.Block{
		ID:             0x11,
		PilotPulse:     2168,
		PilotLength:    3,
		SyncPulse1:     667,
		SyncPulse2:     735,
		DataPulse0:     855,
		DataPulse1:     1710,
		BitsInLastByte: 4, // deliberately partial: only the top 4 bits of the last byte count
		Pause:          10,
		Data:           []byte{0xFF, 0xF0}, // second byte: only bits 7-4 (all 1s) should be encoded
	}

	pulses := tp.genCustomPulses(b)

	want := []int{
		2168, 2168, 2168, // pilot x3
		667, 735, // sync
	}
	// First byte 0xFF: full 8 bits, all 1 -> dataPulse1 pairs.
	for i := 0; i < 8; i++ {
		want = append(want, 1710, 1710)
	}
	// Second byte 0xF0: only top 4 bits count (BitsInLastByte=4), all 1 -> dataPulse1 pairs.
	for i := 0; i < 4; i++ {
		want = append(want, 1710, 1710)
	}
	// genCustomPulses deliberately does NOT include the trailing pause:
	// its spec-correct encoding depends on the entry level of the FULL
	// stream at append time, which only the caller (loadTZX's case
	// 0x11, via addTZXPause) knows. An earlier version inlined a fixed
	// {945, 3500, 3500*ms} pattern here regardless of entry level --
	// the fossil that inverted Batman's block #23 polarity; pause
	// behaviour itself is covered separately by the
	// TestLoadTZX_PauseBlock_* and TestLoadTZX_StandardBlock_* family.

	if len(pulses) != len(want) {
		t.Fatalf("got %d pulses, want %d\ngot:  %v\nwant: %v", len(pulses), len(want), pulses, want)
	}
	for i := range want {
		if pulses[i] != want[i] {
			t.Errorf("pulse[%d] = %d, want %d", i, pulses[i], want[i])
		}
	}
}

// TestGenCustomPulses_PureData confirms 0x14's own pulse generation:
// identical data-bit encoding to 0x11, but no pilot or sync pulses at
// all -- confirmed against SpecIde's own case 0x14, which starts
// straight into the per-byte bit loop.
func TestGenCustomPulses_PureData(t *testing.T) {
	tp := &Tape{st: &TapeState{}}
	b := tzx.Block{
		ID:             0x14,
		DataPulse0:     855,
		DataPulse1:     1710,
		BitsInLastByte: 8,
		Pause:          0, // no pause: addTZXPause(0) is a no-op
		Data:           []byte{0x00},
	}

	pulses := tp.genCustomPulses(b)

	want := []int{}
	for i := 0; i < 8; i++ {
		want = append(want, 855, 855) // all-zero byte -> dataPulse0 pairs throughout
	}

	if len(pulses) != len(want) {
		t.Fatalf("got %d pulses, want %d\ngot:  %v\nwant: %v", len(pulses), len(want), pulses, want)
	}
	for i := range want {
		if pulses[i] != want[i] {
			t.Errorf("pulse[%d] = %d, want %d", i, pulses[i], want[i])
		}
	}
}

// TestLoadTZX_TurboSpeedData_PulsesReachPlayback confirms the wiring, not
// just genCustomPulses in isolation: loading an actual TZX file with a
// 0x11 block produces both a t.st.Blocks entry (fast-load, already
// covered by TestLoadTZX_TurboSpeedData_ReachesBlocksForFastLoad) AND a
// non-empty t.st.Pulses (accurate-mode playback).
func TestLoadTZX_TurboSpeedData_PulsesReachPlayback(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)

	data = append(data, 0x11)
	data = append(data, le16(2168)...) // pilotPulse
	data = append(data, le16(667)...)  // syncPulse1
	data = append(data, le16(735)...)  // syncPulse2
	data = append(data, le16(855)...)  // dataPulse0
	data = append(data, le16(1710)...) // dataPulse1
	data = append(data, le16(10)...)   // pilotLength
	data = append(data, 8)             // bitsInLastByte
	data = append(data, le16(0)...)    // pause
	data = append(data, le24(1)...)    // dataLength
	data = append(data, 0xAA)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	// 10 pilot + 2 sync + 8*2 data pulses = 28.
	if len(tp.st.Pulses) != 28 {
		t.Fatalf("got %d pulses, want 28", len(tp.st.Pulses))
	}
	if tp.st.Pulses[0] != 2168 || tp.st.Pulses[10] != 667 || tp.st.Pulses[11] != 735 {
		t.Errorf("pilot/sync pulses look wrong: %v", tp.st.Pulses[:12])
	}
}

// TestLoadTZX_Stop48K_MarksStopPointWithNoPulses is the regression test
// for a real gap: 0x2A (Stop The Tape If In 48K Mode) was recognised and
// safely skipped by zentools, but never wired into playback at all --
// appears in 13 real corpus games (see the tape compatibility report),
// none of which stopped correctly on a 48K-class machine. Confirmed
// against SpecIde's own case 0x2A (source/src/TZXFile.cc):
// "stopIf48K.insert(pulseData.size())" -- unlike 0x20's pause==0 case,
// no pulses are added at all, just a marker at the current position.
func TestLoadTZX_Stop48K_MarksStopPointWithNoPulses(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)

	// A genuine pulse before the marker, so the test can confirm the
	// marker lands at the right index rather than trivially at 0.
	data = append(data, 0x13, 1)
	data = append(data, le16(667)...)

	data = append(data, 0x2A)
	data = append(data, le32(0)...) // 4-byte length field, always zero

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	if len(tp.st.Pulses) != 1 {
		t.Fatalf("got %d pulses, want 1 -- 0x2A must not add any pulses of its own", len(tp.st.Pulses))
	}
	if !tp.st.StopPointsIf48K[1] {
		t.Errorf("StopPointsIf48K[1] not set -- want a marker right after the one pulse already present")
	}
	if tp.st.StopPoints[1] {
		t.Errorf("StopPoints[1] (the unconditional set) should not be set by 0x2A -- it is 48K-conditional, a separate set")
	}
}

// TestTick_Stop48K_OnlyHaltsOn48KClassMachines confirms the conditional
// half: playback only halts at a StopPointsIf48K marker when the machine
// is 48K-class (is128K false) -- confirmed against SpecIde's own
// Tape::advance(): "if (is48K && stopIf48K.find(pointer) != ...)". A
// 128K/+2/+3 machine reaching the same marker plays straight through.
func TestTick_Stop48K_OnlyHaltsOn48KClassMachines(t *testing.T) {
	for _, tc := range []struct {
		name      string
		is128K    bool
		wantHalts bool
	}{
		{"48K machine halts", false, true},
		{"128K machine plays through", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zx := &ZenZX{memory: &SpectrumMemory{is128K: tc.is128K}}
			tp := &Tape{
				zx: zx,
				st: &TapeState{
					Loaded:  true,
					Playing: true,
					Mode:    TapeAccurate,
					// Five pulses, marker at index 2 (mid-stream, not
					// the last one) -- if the marker were at the final
					// index, "continues past it" would be
					// indistinguishable from simply reaching the
					// natural end of the pulse array, which is exactly
					// the flaw an earlier version of this test had:
					// the 128K case "passed" by hitting end-of-tape at
					// the same point a halt would have, not by
					// genuinely continuing past the marker.
					Pulses: []int{100, 100, 100, 100, 100},
					StopPointsIf48K: map[int]bool{
						2: true,
					},
				},
			}
			tp.Tick(500) // far more than the 300 needed to reach the marker

			if tc.wantHalts {
				if tp.st.Playing {
					t.Error("expected playback to halt on a 48K-class machine, but it did not")
				}
				if tp.st.Position != 3 {
					t.Errorf("Position = %d, want 3 (halted right after the marked pulse, not run further)", tp.st.Position)
				}
			} else {
				// Playing is not checked here: reaching the natural end
				// of this short, 5-pulse test tape correctly sets
				// Playing=false regardless of the marker (a separate,
				// pre-existing "ran off the end" path, not this test's
				// concern). Position is the real, meaningful signal --
				// 5 means it played straight through the marker to the
				// natural end; 3 (the halt case's own expected value)
				// would mean it stopped early at the marker instead,
				// which is exactly what must NOT happen here.
				if tp.st.Position != 5 {
					t.Errorf("Position = %d, want 5 (ran to the natural end, having played straight through the marker)", tp.st.Position)
				}
			}
		})
	}
}

// TestLoadTZX_BlockBoundaries_MarksEachPulseContributingBlock confirms
// BlockBoundaries records the pulse index at which each pulse-
// contributing block's own data begins, and only for those block types
// -- metadata-only blocks between them do not get a spurious entry,
// since they never touch Pulses at all.
func TestLoadTZX_BlockBoundaries_MarksEachPulseContributingBlock(t *testing.T) {
	data := []byte{}
	data = append(data, []byte("ZXTape!")...)
	data = append(data, 0x1A, 1, 20)

	// Block 1: 0x13 (Sequence Of Pulses), 2 pulses -- boundary should be at 0.
	data = append(data, 0x13, 2)
	data = append(data, le16(100)...)
	data = append(data, le16(200)...)

	// A metadata-only block in between -- must NOT add a boundary entry.
	data = append(data, 0x30, 1, 'X')

	// Block 2: another 0x13, 3 pulses -- boundary should be at 2 (after the first block's 2 pulses).
	data = append(data, 0x13, 3)
	data = append(data, le16(300)...)
	data = append(data, le16(400)...)
	data = append(data, le16(500)...)

	tp := &Tape{st: &TapeState{}}
	if err := tp.loadTZX(data); err != nil {
		t.Fatalf("loadTZX: %v", err)
	}

	want := []int{0, 2}
	if len(tp.st.BlockBoundaries) != len(want) {
		t.Fatalf("got %v, want %v", tp.st.BlockBoundaries, want)
	}
	for i := range want {
		if tp.st.BlockBoundaries[i] != want[i] {
			t.Errorf("BlockBoundaries[%d] = %d, want %d", i, tp.st.BlockBoundaries[i], want[i])
		}
	}
	if len(tp.st.Pulses) != 5 {
		t.Fatalf("got %d total pulses, want 5", len(tp.st.Pulses))
	}
}
