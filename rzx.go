package zenzx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zentools/pkg/rzx"
	"github.com/ha1tch/zentools/pkg/snapshot"
	"github.com/ha1tch/zentools/pkg/szx"
)

// ============================================================================
// RZX replay
//
// An .rzx file is a recording of a session: a snapshot to start from, and then, frame by
// frame, how many opcodes the machine fetched before the next frame interrupt and what every
// IN instruction read. Playing it back needs no keyboard, joystick or tape: the machine is
// put in the recorded state and run, the interrupt is raised when the fetch count of the frame
// has been reached (not when the ULA's own timing would), and each IN gets the next recorded
// byte instead of asking the hardware. A machine that follows the program exactly ends every
// frame in the same place the recording did, whatever its own timing is like.
//
// The container is read by zentools' pkg/rzx (spec 0.13). What is here is the machine side:
//
//   - the fetch count of an instruction is how far R went up while it ran: one for every
//     opcode fetch, prefixes included, and one for every halted cycle. Two kinds are counted
//     by hand instead. LD R,A writes R, so what R shows afterwards says nothing: it is the two
//     fetches it is. And every repeat of LDIR and its kin (the CPU runs them one at a time) is
//     two fetches, the ED and the second opcode byte, which are fetched again on each repeat;
//     zen80 puts R up by three on a repeat, one too many, so R cannot be trusted there either.
//   - the interrupt acknowledge adds one to R as well, but is not counted by the format, so
//     the step that takes the interrupt is told from the others by what it leaves behind: the
//     interrupts off, the stack two bytes lower and the old PC on top of it.
//   - the interrupt is offered at one instruction boundary only, the first of the frame, and is
//     lost if the CPU cannot take it there (the interrupts are off, or it is the instruction
//     after an EI). The line is not held for the 32 or so cycles it is live on a real machine:
//     the recorders count every trigger as a frame of its own, and the recordings show that
//     one that finds the interrupts off, then back on a few cycles later, is not taken then.
//   - a frame the recorder ended between a prefix byte and its opcode is played with the
//     instruction whole, which is one fetch more than the frame holds: that one fetch is the
//     next frame's, and is counted there.
//
// A recording that goes out of step -- a different machine from the one it was made on, or
// something the emulation does not do the way the recorder's did -- is noticed at the end of
// the frame (more or fewer fetches or INs than were recorded) and playback stops, leaving the
// machine where it is to carry on by itself. Only playback is built: nothing records.
// ============================================================================

// rzxMagic starts every .rzx file.
const rzxMagic = "RZX!"

// rzxReplay is a recording being played.
type rzxReplay struct {
	file *rzx.File
	name string // the file's name, for messages

	blk    int         // the next block of file.Blocks to look at
	frames []rzx.Frame // the frames of the recording block being played
	frame  int         // the next of frames to play
	fresh  bool        // a snapshot was loaded since the last frame: no interrupt starts the next
	have   bool        // a snapshot has been loaded: what the recording plays from

	tstates uint32 // the block's t-states at its start: where in the frame its first one begins

	last  []byte // the previous frame's reads, for a frame that repeats them
	reads []byte // this frame's
	rd    int    // how many of reads have been given
	over  bool   // an IN wanted more than the frame recorded

	want  int // this frame's fetch count
	count int // fetches so far in it
	carry int // fetches of the next frame the last one ran into, to start its count with

	total  int // every frame of every recording block, for what is said
	played int
}

// rzxFramesOf is the number of frames the file's recording blocks hold in all.
func rzxFramesOf(f *rzx.File) int {
	n := 0
	for _, b := range f.Blocks {
		if rb, ok := b.(rzx.RecordingBlock); ok {
			n += len(rb.Frames)
		}
	}
	return n
}

// LoadRZX opens an .rzx file: the machine is put in the state of its first snapshot and the
// recording plays from the next frame. The machine must be the model the recording was made
// on (the snapshot says which), which is not switched here.
func (zx *ZenZX) LoadRZX(filename string) error {
	image, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return zx.LoadRZXBytes(filepath.Base(filename), image)
}

// LoadRZXBytes is LoadRZX for a file already read; name is only used in what is said.
func (zx *ZenZX) LoadRZXBytes(name string, image []byte) error {
	f, err := rzx.Decode(image)
	if err != nil {
		return fmt.Errorf("decode .rzx: %w", err)
	}
	if zx.memory.isNext {
		return fmt.Errorf("an .rzx recording is for a 48K, 128K, +2, +2A or +3, not a Next")
	}
	r := &rzxReplay{file: f, name: name, total: rzxFramesOf(f)}
	if r.total == 0 {
		return fmt.Errorf("the recording has no frames")
	}
	zx.rzxStop("") // a recording that was playing is given up, whatever happens next
	ok, err := r.advance(zx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("the recording has no frames")
	}
	zx.rzx = r
	zx.io.replayIn = r.in
	return nil
}

// advance goes to the next recording block, loading the snapshot blocks it passes: a
// recording made across a tape load has a new snapshot before each part. It reports whether
// there is one to play.
func (r *rzxReplay) advance(zx *ZenZX) (bool, error) {
	for r.blk < len(r.file.Blocks) {
		switch b := r.file.Blocks[r.blk].(type) {
		case rzx.SnapshotBlock:
			r.blk++
			if err := zx.rzxLoadSnapshot(b); err != nil {
				return false, err
			}
			r.fresh, r.have = true, true
		case rzx.RecordingBlock:
			r.blk++
			if b.Protected {
				return false, fmt.Errorf("the recording is encrypted, which is not supported")
			}
			if len(b.Frames) == 0 {
				continue
			}
			if !r.have {
				return false, fmt.Errorf("the recording starts with no snapshot")
			}
			r.frames, r.frame, r.last = b.Frames, 0, nil
			r.tstates = b.TStatesStart
			return true, nil
		default: // the creator and the security blocks say nothing the playback needs
			r.blk++
		}
	}
	return false, nil
}

// rzxLoadSnapshot puts the machine in the state of a snapshot block.
func (zx *ZenZX) rzxLoadSnapshot(b rzx.SnapshotBlock) error {
	if b.External {
		return fmt.Errorf("the snapshot is a file outside the recording, which is not supported")
	}
	var s *snapshot.MachineState
	var err error
	switch ext := strings.ToLower(strings.Trim(b.Extension, "\x00 .")); ext {
	case "z80":
		s, err = snapshot.DecodeZ80(b.Data)
	case "sna":
		if len(b.Data) > 49179 {
			s, err = snapshot.DecodeSNA128(b.Data)
		} else {
			s, err = snapshot.DecodeSNA(b.Data)
		}
	case "szx":
		s, err = szx.Decode(b.Data)
	default:
		return fmt.Errorf("the recording's snapshot is a .%s, which is not supported", ext)
	}
	if err != nil {
		return fmt.Errorf("the recording's snapshot: %w", err)
	}
	if err := zx.rzxModelFits(s.Model); err != nil {
		return err
	}
	r := zx.rzx
	zx.rzx = nil // loading a snapshot gives up a recording, and this one is not to be
	zx.fromMachineState(s)
	zx.resyncAfterLoad()
	zx.rzx = r
	zx.cpu.INT = false
	zx.cpu.Halted = false
	return nil
}

// rzxModelFits is nil if the machine is the kind the recording was made on.
func (zx *ZenZX) rzxModelFits(m snapshot.Model) error {
	have, want := "48K", "48K"
	switch {
	case zx.memory.isPlus3:
		have = "+2A or +3"
	case zx.memory.is128K:
		have = "128K or +2"
	}
	switch m {
	case snapshot.Model128K, snapshot.ModelPlus2:
		want = "128K or +2"
	case snapshot.ModelPlus2A, snapshot.ModelPlus3:
		want = "+2A or +3"
	}
	if have != want {
		return fmt.Errorf("the recording was made on a %s, and this machine is a %s: start that model first", want, have)
	}
	return nil
}

// in is what an IN instruction reads while a recording plays: the next byte recorded for the
// frame. Past the end of what the frame recorded it gives way to the machine and the frame
// is told off at its end.
func (r *rzxReplay) in() (uint8, bool) {
	if r.rd < len(r.reads) {
		v := r.reads[r.rd]
		r.rd++
		return v, true
	}
	r.over = true
	return 0, false
}

// rzxStop ends a recording that is playing, with msg said if there is one, and leaves the
// machine as it is.
func (zx *ZenZX) rzxStop(msg string) {
	if zx.rzx == nil {
		return
	}
	zx.rzx = nil
	zx.io.replayIn = nil
	if msg != "" {
		fmt.Println(msg)
		if zx.OnRZX != nil {
			zx.OnRZX(msg)
		}
	}
}

// RZXPlaying reports whether a recording is playing, with the frame it is at and the number
// of them all.
func (zx *ZenZX) RZXPlaying() (playing bool, frame, total int) {
	if zx.rzx == nil {
		return false, 0, 0
	}
	return true, zx.rzx.played, zx.rzx.total
}

// How the fetches of an instruction are counted: by how far R went up, or by hand (see above).
const (
	rzxByR   = iota
	rzxLDRA  // LD R,A
	rzxBlock // LDIR, LDDR, CPIR, CPDR, INIR, INDR, OTIR or OTDR
)

// rzxKind says how the fetches of the instruction at pc are to be counted.
func (zx *ZenZX) rzxKind(pc uint16) int {
	if zx.memory.Read(pc) != 0xED {
		return rzxByR
	}
	switch op := zx.memory.Read(pc + 1); {
	case op == 0x4F:
		return rzxLDRA
	case op&0xF4 == 0xB0: // B0-B3 and B8-BB
		return rzxBlock
	}
	return rzxByR
}

// rzxTookInterrupt says whether the step just taken was the interrupt being acknowledged
// rather than an instruction: the interrupts are off, the stack is two lower and the PC the
// CPU was at is on top of it.
func (zx *ZenZX) rzxTookInterrupt(pc, sp uint16) bool {
	c := zx.cpu
	if c.IFF1 || c.SP != sp-2 {
		return false
	}
	return uint16(zx.memory.Read(c.SP))|uint16(zx.memory.Read(c.SP+1))<<8 == pc
}

// rzxFrame plays one recorded frame in place of the ULA's own, and reports whether it did:
// false means no recording is playing (any longer) and the caller runs the frame as usual.
// It is RunFrame's, after the frame has been started.
func (zx *ZenZX) rzxFrame() bool {
	r := zx.rzx
	if r == nil {
		return false
	}
	if r.frame >= len(r.frames) {
		ok, err := r.advance(zx)
		switch {
		case err != nil:
			zx.rzxStop(fmt.Sprintf("%s stopped at frame %d of %d: %v", r.name, r.played, r.total, err))
			return false
		case !ok:
			zx.rzxStop(fmt.Sprintf("%s has ended: all %d frames have played", r.name, r.total))
			return false
		}
	}

	f := r.frames[r.frame]
	r.frame++
	r.played++
	if f.Repeated {
		r.reads = r.last
	} else {
		r.reads, r.last = f.PortReads, f.PortReads
	}
	r.rd, r.over = 0, false
	r.want, r.count, r.carry = int(f.FetchCount), r.carry, 0

	raise := !r.fresh // a snapshot is loaded between interrupts, so none starts the frame that follows it
	if r.fresh && uint64(r.tstates) <= zx.cpu.Cycles {
		// Where in its frame the recording began: the contention and the border measure from there.
		zx.frameOrigin = zx.cpu.Cycles - uint64(r.tstates)
	}
	r.fresh = false

	zx.cpu.INT = raise
	cycleUpdateCounter := 0
	prefixed := false // the last instruction began with a prefix byte
	for r.count < r.want && zx.running {
		pc, sp, rBefore := zx.cpu.PC, zx.cpu.SP, zx.cpu.R
		kind := zx.rzxKind(pc)
		offered := zx.cpu.INT
		lead := zx.memory.Read(pc)
		cycles := zx.stepCPU()
		zx.cpu.INT = false // offered for the one step, taken or not
		zx.cycleCount += cycles
		cycleUpdateCounter += cycles
		prefixed = !offered && (lead == 0xDD || lead == 0xFD || lead == 0xED || lead == 0xCB)

		if offered && zx.rzxTookInterrupt(pc, sp) {
			// taken: the acknowledge is not one of the frame's fetches
		} else if kind == rzxLDRA || (kind == rzxBlock && zx.cpu.PC == pc) { // a block repeating leaves the PC where it was
			r.count += 2
		} else {
			r.count += int(zx.cpu.R-rBefore) & 0x7F
		}
		if zx.audio != nil && cycleUpdateCounter >= 500 && zx.audio.IsEnabled() {
			zx.audio.UpdateCPUCycle(zx.cpu.Cycles)
			cycleUpdateCounter = 0
		}
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}
	zx.cpu.INT = false
	if r.count == r.want+1 && prefixed {
		// The recorder's frame ended between the prefix and the opcode, which then belongs to
		// the next frame: this instruction was one fetch of that one already.
		r.count, r.carry = r.want, 1
	}

	if zx.audio != nil && zx.audio.IsEnabled() {
		zx.audio.UpdateCPUCycle(zx.cpu.Cycles)
	}
	zx.display.SetBorderChanges(zx.io.GetBorderHistory(), zx.frameOrigin, zx.cyclesPerFrame)
	zx.io.EndFrame()
	zx.frameCount++
	zx.shotFrame() // the GIF being recorded takes the frame (screenshot_gif.go)

	if zx.running {
		switch {
		case r.count != r.want:
			zx.rzxStop(fmt.Sprintf("%s went out of step at frame %d of %d (%d opcodes fetched where %d were recorded): playback has stopped and the machine runs on by itself",
				r.name, r.played, r.total, r.count, r.want))
		case r.over || r.rd != len(r.reads):
			zx.rzxStop(fmt.Sprintf("%s went out of step at frame %d of %d (%d IN reads where %d were recorded): playback has stopped and the machine runs on by itself",
				r.name, r.played, r.total, r.rd, len(r.reads)))
		}
	}
	return true
}
