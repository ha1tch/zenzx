package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zentools/pkg/tzx"
)

// blockNames gives the human-readable name for every TZX block ID
// zentools/pkg/tzx recognises. Kept here rather than exported from that
// package, since it's report-presentation detail, not decoder behaviour.
var blockNames = map[byte]string{
	0x10: "Standard Speed Data",
	0x11: "Turbo Speed Data",
	0x12: "Pure Tone",
	0x13: "Sequence Of Pulses",
	0x14: "Pure Data",
	0x15: "Direct Recording",
	0x16: "C64 ROM Type Data",
	0x17: "C64 Turbo Tape Data",
	0x18: "CSW Recording",
	0x19: "Generalized Data Block",
	0x20: "Pause / Stop The Tape",
	0x21: "Group Start",
	0x22: "Group End",
	0x23: "Jump To Block",
	0x24: "Loop Start",
	0x25: "Loop End",
	0x26: "Call Sequence",
	0x27: "Return From Sequence",
	0x28: "Select Block",
	0x2A: "Stop The Tape If In 48K Mode",
	0x2B: "Set Signal Level",
	0x30: "Text Description",
	0x31: "Message",
	0x32: "Archive Info",
	0x33: "Hardware Type",
	0x35: "Custom Info Block",
	0x5A: `"Glue" Block`,
}

// fastLoadWired lists TZX block IDs whose data reaches t.st.Blocks in
// loadTZX (tape_types.go), which is all trapLoad (tape.go) ever needs for
// fast-load. KEEP IN SYNC WITH loadTZX's OWN SWITCH -- its doc comment
// there is the source of truth; this table exists only to describe that
// switch for reporting, not to define it independently.
var fastLoadWired = map[byte]bool{
	0x10: true, 0x11: true, 0x14: true,
}

// playbackWired lists TZX block IDs that meaningfully affect accurate-mode
// playback in loadTZX/Tick -- either by contributing pulse timings, or (for
// 0x2A specifically) by conditionally halting playback with no pulses of
// its own. Same sync requirement as fastLoadWired above.
var playbackWired = map[byte]bool{
	0x10: true, 0x11: true, 0x12: true, 0x13: true, 0x14: true, 0x20: true, 0x2A: true,
}

// dataBearingBlocks lists TZX block IDs that carry loadable payload data
// (as opposed to metadata, control flow, or pure timing blocks) -- these
// are the ones fast-load coverage actually matters for.
var dataBearingBlocks = map[byte]bool{
	0x10: true, 0x11: true, 0x14: true, 0x15: true, 0x18: true, 0x19: true,
}

// pulseRelevantBlocks lists every TZX block ID that represents actual tape
// audio content -- dataBearingBlocks, plus pure-pulse/timing blocks that
// carry no loadable payload but do need to contribute to (or otherwise
// affect) the pulse stream for a file to play back correctly in accurate
// mode: pilot tone, an explicit pulse sequence, an ordinary pause, and the
// 48K-conditional stop signal. Metadata and control-flow blocks (groups,
// jump/loop/call, archive info, and the like) are deliberately excluded --
// they carry no audio content of their own, so their wiring status has no
// bearing on whether a file plays back correctly.
var pulseRelevantBlocks = map[byte]bool{
	0x10: true, 0x11: true, 0x12: true, 0x13: true, 0x14: true,
	0x15: true, 0x18: true, 0x19: true, 0x20: true, 0x2A: true,
}

// BlockStat summarises one TZX block ID's usage across a corpus.
type BlockStat struct {
	ID        byte
	IDHex     string
	Name      string
	Files     int
	FilePct   float64
	Instances int
	Kind      string // "data" or "meta"
	FastLoad  string // "yes", "no", "n/a"
	Playback  string // "yes", "no", "n/a"
}

// Stats holds everything the report template needs.
type Stats struct {
	Games    int
	TZXFiles int
	TZXOK    int
	TZXErr   int
	TAPFiles int
	TAPOK    int
	TAPErr   int
	TotalMB  float64

	Blocks []BlockStat

	FilesWithUnwiredDataBlock int

	// Per-file classification -- the practical question the block-type
	// table alone doesn't answer: of the actual TZX files in this corpus,
	// how many will genuinely work in each mode right now. A file
	// containing zero pulse-relevant blocks at all (metadata-only, no
	// real audio content) counts toward neither FastLoadable nor
	// Playable -- there's nothing to load or play, so neither claim
	// would mean anything -- and is excluded from NonPlayable too, for
	// the same reason: "does not work" is not a meaningful claim about
	// a file with nothing to play in the first place.
	FastLoadableFiles int
	FastLoadablePct   float64
	PlayableFiles     int
	PlayablePct       float64
	NonPlayableFiles  int
	NonPlayablePct    float64
}

func computeStats(root string) (*Stats, error) {
	var tzxFiles, tapFiles []string
	var totalSize int64
	games := map[string]bool{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if filepath.Dir(path) == root {
				games[path] = true
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".tzx":
			tzxFiles = append(tzxFiles, path)
			totalSize += info.Size()
		case ".tap":
			tapFiles = append(tapFiles, path)
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s := &Stats{
		Games:    len(games),
		TZXFiles: len(tzxFiles),
		TAPFiles: len(tapFiles),
		TotalMB:  float64(totalSize) / 1048576,
	}

	blockIDCounts := map[byte]int{}
	filesUsingBlockID := map[byte]map[string]bool{}
	filesWithUnwired := map[string]bool{}

	filesWithPulseContent := 0
	for _, path := range tzxFiles {
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}
		blocks, derr := tzx.Decode(data)

		// Per-file classification: does every pulse-relevant block this
		// specific file actually uses have fast-load/playback wiring.
		// hasPulseContent tracks whether the file has any pulse-relevant
		// block at all -- a file with none (pure metadata, no real audio
		// content) makes neither claim meaningful and is excluded from
		// all three per-file totals below.
		fileFastLoadable := true
		filePlayable := true
		hasPulseContent := false

		for _, b := range blocks {
			blockIDCounts[b.ID]++
			if filesUsingBlockID[b.ID] == nil {
				filesUsingBlockID[b.ID] = map[string]bool{}
			}
			filesUsingBlockID[b.ID][path] = true
			if dataBearingBlocks[b.ID] && !fastLoadWired[b.ID] {
				filesWithUnwired[path] = true
			}
			if dataBearingBlocks[b.ID] && !fastLoadWired[b.ID] {
				fileFastLoadable = false
			}
			if pulseRelevantBlocks[b.ID] {
				hasPulseContent = true
				if !playbackWired[b.ID] {
					filePlayable = false
				}
			}
		}

		if hasPulseContent {
			filesWithPulseContent++
			if fileFastLoadable {
				s.FastLoadableFiles++
			}
			if filePlayable {
				s.PlayableFiles++
			}
			if !fileFastLoadable && !filePlayable {
				s.NonPlayableFiles++
			}
		}

		if derr != nil {
			s.TZXErr++
		} else {
			s.TZXOK++
		}
	}
	if filesWithPulseContent > 0 {
		s.FastLoadablePct = 100 * float64(s.FastLoadableFiles) / float64(filesWithPulseContent)
		s.PlayablePct = 100 * float64(s.PlayableFiles) / float64(filesWithPulseContent)
		s.NonPlayablePct = 100 * float64(s.NonPlayableFiles) / float64(filesWithPulseContent)
	}

	for _, path := range tapFiles {
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}
		if _, derr := tap.Decode(data); derr != nil {
			s.TAPErr++
		} else {
			s.TAPOK++
		}
	}

	for id, files := range filesUsingBlockID {
		kind := "meta"
		if dataBearingBlocks[id] {
			kind = "data"
		}
		fl := "n/a"
		if dataBearingBlocks[id] {
			fl = "no"
			if fastLoadWired[id] {
				fl = "yes"
			}
		}
		pb := "no"
		if playbackWired[id] {
			pb = "yes"
		}
		name := blockNames[id]
		if name == "" {
			name = "Unknown"
		}
		s.Blocks = append(s.Blocks, BlockStat{
			ID:        id,
			IDHex:     hexByte(id),
			Name:      name,
			Files:     len(files),
			FilePct:   100 * float64(len(files)) / float64(len(tzxFiles)),
			Instances: blockIDCounts[id],
			Kind:      kind,
			FastLoad:  fl,
			Playback:  pb,
		})
	}
	sort.Slice(s.Blocks, func(i, j int) bool { return s.Blocks[i].Files > s.Blocks[j].Files })

	s.FilesWithUnwiredDataBlock = len(filesWithUnwired)

	return s, nil
}

func hexByte(b byte) string {
	const hexDigits = "0123456789ABCDEF"
	return "0x" + string(hexDigits[b>>4]) + string(hexDigits[b&0xF])
}
