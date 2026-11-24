package zenzx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zentools/pkg/rzx"
	"github.com/ha1tch/zentools/pkg/snapshot"
)

// These tests play recordings made by hand: a few bytes of program in RAM (no ROM is needed),
// and frames whose fetch counts are worked out from the instructions, so that what they check
// is the counting rules of rzx.go and not a recorder that follows the same ones.

// rzxProg is the program the counting tests run, at 0x8000, with the number of opcodes
// fetched by each instruction:
//
//	8000 DI                  1
//	8001 LD BC,3             1
//	8004 LD HL,9000          1
//	8007 LD DE,9100          1
//	800A LDIR                2 for every one of the 3 repeats
//	800C LD A,55             1
//	800E LD R,A              2
//	8010 INC IXH             2   (DD 24)
//	8012 NOP                 1
//	8013 IN A,(FE)           1
//	8015 LD (9200),A         1
//	8018 JR 8018             1 each time
var rzxProg = []byte{
	0xF3,
	0x01, 0x03, 0x00,
	0x21, 0x00, 0x90,
	0x11, 0x00, 0x91,
	0xED, 0xB0,
	0x3E, 0x55,
	0xED, 0x4F,
	0xDD, 0x24,
	0x00,
	0xDB, 0xFE,
	0x32, 0x00, 0x92,
	0x18, 0xFE,
}

// rzxTestMachine is a 48K with no ROM.
func rzxTestMachine(t *testing.T) *ZenZX {
	t.Helper()
	zx := NewZenZX(AudioBackendOto)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}
	zx.cpu.Reset()
	return zx
}

// rzxSnapshot is a .z80 of a machine with prog at 0x8000 about to run it.
func rzxSnapshot(t *testing.T, prog []byte, setup func(zx *ZenZX)) []byte {
	t.Helper()
	zx := rzxTestMachine(t)
	for i, b := range prog {
		zx.memory.Write(0x8000+uint16(i), b)
	}
	for i, b := range []byte{1, 2, 3} {
		zx.memory.Write(0x9000+uint16(i), b)
	}
	zx.cpu.PC, zx.cpu.SP, zx.cpu.IFF1, zx.cpu.IFF2 = 0x8000, 0xF000, false, false
	if setup != nil {
		setup(zx)
	}
	path := filepath.Join(t.TempDir(), "start.z80")
	if err := zx.SaveZ80(path); err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return image
}

// rzxImage is an .rzx of snapshot and frames.
func rzxImage(t *testing.T, snap []byte, frames ...rzx.Frame) []byte {
	t.Helper()
	image, err := rzx.Encode(&rzx.File{
		MajorVersion: 0, MinorVersion: 13,
		Blocks: []rzx.Block{
			rzx.SnapshotBlock{Compressed: true, Extension: "z80", Data: snap},
			rzx.RecordingBlock{Frames: frames},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return image
}

// rzxPlay loads image on a fresh machine and says what the recording says when it stops.
func rzxPlay(t *testing.T, image []byte) (*ZenZX, *string) {
	t.Helper()
	zx := rzxTestMachine(t)
	said := new(string)
	zx.OnRZX = func(msg string) { *said = msg }
	if err := zx.LoadRZXBytes("test.rzx", image); err != nil {
		t.Fatal(err)
	}
	return zx, said
}

func rzxFrames(counts ...int) []rzx.Frame {
	var f []rzx.Frame
	for _, c := range counts {
		f = append(f, rzx.Frame{FetchCount: uint16(c)})
	}
	return f
}

// Every instruction is counted the way the recording counted it, the repeats of a block
// instruction and LD R,A included, and IN reads what was recorded, not what is pressed.
func TestRZXReplayCountsAndReads(t *testing.T) {
	snap := rzxSnapshot(t, rzxProg, nil)
	// 1+1+1+1 + 3 repeats of LDIR at 2 + 1 + LD R,A at 2 + INC IXH at 2 + NOP: 16 fetches, which
	// ends the frame at the IN. The second frame is the IN, the store and three JRs.
	image := rzxImage(t, snap,
		rzx.Frame{FetchCount: 16},
		rzx.Frame{FetchCount: 5, PortReads: []byte{0x5A}},
	)
	zx, said := rzxPlay(t, image)
	zx.io.PressKey(1, 0) // what the keyboard says now is not what the recording read

	zx.RunFrame()
	if *said != "" {
		t.Fatalf("frame 1: %s", *said)
	}
	if zx.cpu.PC != 0x8013 {
		t.Fatalf("after frame 1 PC = %04X, want 8013 (the IN)", zx.cpu.PC)
	}
	for i, want := range []byte{1, 2, 3} {
		if got := zx.memory.Read(0x9100 + uint16(i)); got != want {
			t.Errorf("LDIR copied %02X to %04X, want %02X", got, 0x9100+i, want)
		}
	}

	zx.RunFrame()
	if *said != "" {
		t.Fatalf("frame 2: %s", *said)
	}
	if got := zx.memory.Read(0x9200); got != 0x5A {
		t.Errorf("IN gave %02X, want the recorded 5A", got)
	}
	if zx.cpu.PC != 0x8018 {
		t.Errorf("after frame 2 PC = %04X, want 8018", zx.cpu.PC)
	}
	if playing, frame, total := zx.RZXPlaying(); !playing || frame != 2 || total != 2 {
		t.Errorf("RZXPlaying = %v, %d, %d", playing, frame, total)
	}

	zx.RunFrame() // nothing is left to play
	if playing, _, _ := zx.RZXPlaying(); playing {
		t.Error("still playing after the last frame")
	}
	if !strings.Contains(*said, "ended") {
		t.Errorf("end said %q", *said)
	}
}

// A frame the recorder ended between a prefix and its opcode: the instruction runs whole, and
// the opcode's fetch counts toward the next frame.
func TestRZXFrameEndsInsideAPrefix(t *testing.T) {
	snap := rzxSnapshot(t, rzxProg, nil)
	image := rzxImage(t, snap,
		rzx.Frame{FetchCount: 14},                         // up to the DD of INC IXH
		rzx.Frame{FetchCount: 3, PortReads: []byte{0x5A}}, // its opcode, the NOP and the IN
	)
	zx, said := rzxPlay(t, image)
	zx.RunFrame()
	zx.RunFrame()
	if *said != "" {
		t.Fatal(*said)
	}
	if zx.cpu.PC != 0x8015 || zx.cpu.A != 0x5A {
		t.Errorf("PC = %04X, A = %02X, want 8015 and 5A", zx.cpu.PC, zx.cpu.A)
	}
}

// The interrupt starts every frame but the first, is counted as no fetch, and is taken or lost
// at that one instruction boundary.
func TestRZXInterrupt(t *testing.T) {
	prog := make([]byte, 0x300)
	copy(prog, []byte{0xFB, 0x76, 0x18, 0xFD}) // 8000 EI; 8001 HALT; 8002 JR 8001
	// IM 2 reads its vector from I:FF, and the table there (80FF, 8100) points at 8200
	prog[0xFF], prog[0x100] = 0x00, 0x82
	copy(prog[0x200:], []byte{0x21, 0x00, 0x93, 0x34, 0xFB, 0xED, 0x4D}) // LD HL,9300; INC (HL); EI; RETI
	snap := rzxSnapshot(t, prog, func(zx *ZenZX) {
		zx.cpu.I, zx.cpu.IM = 0x80, 2
	})
	// 1: EI and 99 halted fetches, no interrupt. 2 and 3: the handler (LD, INC, EI, RETI: 5),
	// a JR and the HALT until the next.
	image := rzxImage(t, snap, rzxFrames(100, 50, 50)...)
	zx, said := rzxPlay(t, image)
	zx.RunFrame()
	if got := zx.memory.Read(0x9300); got != 0 {
		t.Fatalf("the first frame ran the handler (%d)", got)
	}
	zx.RunFrame()
	zx.RunFrame()
	if *said != "" {
		t.Fatal(*said)
	}
	if got := zx.memory.Read(0x9300); got != 2 {
		t.Errorf("the handler ran %d times in two frames, want 2", got)
	}
}

// A recording that does not fit the machine ends where it breaks, and the machine goes on.
func TestRZXOutOfStepStops(t *testing.T) {
	snap := rzxSnapshot(t, rzxProg, nil)
	cases := []struct {
		name   string
		frames []rzx.Frame
		frame  string
	}{
		{"too many fetches", []rzx.Frame{{FetchCount: 17}}, "frame 1 of 1"},
		{"missing reads", []rzx.Frame{{FetchCount: 16}, {FetchCount: 5}}, "frame 2 of 2"},
		{"unused reads", []rzx.Frame{{FetchCount: 16, PortReads: []byte{1}}}, "frame 1 of 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			zx, said := rzxPlay(t, rzxImage(t, snap, c.frames...))
			for i := 0; i < len(c.frames) && *said == ""; i++ {
				zx.RunFrame()
			}
			if !strings.Contains(*said, "went out of step at "+c.frame) {
				t.Fatalf("said %q, want it out of step at %s", *said, c.frame)
			}
			if playing, _, _ := zx.RZXPlaying(); playing {
				t.Error("still playing")
			}
			zx.RunFrame() // the machine runs on by itself
		})
	}
}

// What the recording needs from the machine is checked when it is opened, and anything that
// takes the machine somewhere else ends it.
func TestRZXLoadChecks(t *testing.T) {
	snap := rzxSnapshot(t, rzxProg, nil)

	if _, err := rzx.Decode([]byte("not an rzx file")); err == nil {
		t.Fatal("decoded junk")
	}
	if err := rzxTestMachine(t).LoadRZXBytes("junk.rzx", []byte("not an rzx file")); err == nil {
		t.Error("junk loaded")
	}

	empty, err := rzx.Encode(&rzx.File{MinorVersion: 13, Blocks: []rzx.Block{
		rzx.SnapshotBlock{Compressed: true, Extension: "z80", Data: snap},
		rzx.RecordingBlock{},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := rzxTestMachine(t).LoadRZXBytes("empty.rzx", empty); err == nil || !strings.Contains(err.Error(), "no frames") {
		t.Errorf("no frames: %v", err)
	}

	external, err := rzx.Encode(&rzx.File{MinorVersion: 13, Blocks: []rzx.Block{
		rzx.SnapshotBlock{External: true, Extension: "z80", Data: []byte("0123456789abcdefname.z80")},
		rzx.RecordingBlock{Frames: rzxFrames(1)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := rzxTestMachine(t).LoadRZXBytes("ext.rzx", external); err == nil {
		t.Error("a snapshot kept outside the file loaded")
	}

	zx := rzxTestMachine(t)
	if err := zx.rzxModelFits(snapshot.Model48K); err != nil {
		t.Errorf("48K on a 48K: %v", err)
	}
	for _, m := range []snapshot.Model{snapshot.Model128K, snapshot.ModelPlus2A} {
		if err := zx.rzxModelFits(m); err == nil || !strings.Contains(err.Error(), "start that model first") {
			t.Errorf("model %v on a 48K: %v", m, err)
		}
	}

	// Reset, and loading anything else, give a recording up.
	good := rzxImage(t, snap, rzxFrames(16, 5)...)
	zx, _ = rzxPlay(t, good)
	zx.Reset()
	if playing, _, _ := zx.RZXPlaying(); playing {
		t.Error("still playing after a reset")
	}
	zx, _ = rzxPlay(t, good)
	path := filepath.Join(t.TempDir(), "other.z80")
	if err := os.WriteFile(path, snap, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := zx.LoadSnapshotFormat(path, ""); err != nil {
		t.Fatal(err)
	}
	if playing, _, _ := zx.RZXPlaying(); playing {
		t.Error("still playing after a snapshot was loaded")
	}
}

// The .rzx is found by its extension and opened by the one call every snapshot goes through.
func TestRZXThroughLoadSnapshot(t *testing.T) {
	snap := rzxSnapshot(t, rzxProg, nil)
	path := filepath.Join(t.TempDir(), "game.rzx")
	if err := os.WriteFile(path, rzxImage(t, snap, rzxFrames(16)...), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectSnapshotFormat(path); got != "rzx" {
		t.Fatalf("detected %q", got)
	}
	zx := rzxTestMachine(t)
	if err := zx.LoadSnapshotFormat(path, ""); err != nil {
		t.Fatal(err)
	}
	if playing, _, total := zx.RZXPlaying(); !playing || total != 1 {
		t.Errorf("RZXPlaying = %v of %d", playing, total)
	}
	if err := zx.SaveSnapshotFormat(filepath.Join(t.TempDir(), "x.rzx"), "rzx"); err == nil {
		t.Error("saved an .rzx")
	}
}
