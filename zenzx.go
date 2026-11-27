package zenzx

import (
	"fmt"
	"image"
	"os"

	"github.com/ha1tch/zen80/z80"
)

// ============================================================================
// Constants
// ============================================================================

const (
	// Timing -- these remain the package-level PAL defaults, used as-is
	// by every model except TS2068. Genuinely per-instance timing lives
	// on ZenZX (cyclesPerFrame/interruptLength below), defaulting to
	// these values at construction and overridden only by
	// LoadTS2068ROM. CPUFrequency/FramesPerSecond stay global constants
	// deliberately, not yet threaded per-instance: TS2068 audio (Stage
	// 4, AY ports) isn't implemented yet, so nothing currently reads
	// them expecting TS2068's real 3.528MHz/60.1145Hz -- revisit when
	// Stage 4 lands, since AY pitch generation genuinely needs the true
	// clock. See docs/TS2068_DEVELOPMENT_PLAN.md Stage 2.
	CPUFrequency    = 3500000                        // 3.5 MHz
	FramesPerSecond = 50                             // 50 Hz PAL nominal
	CyclesPerFrame  = CPUFrequency / FramesPerSecond // 70000 -- the naive 3.5MHz/50Hz figure. Real PAL frames are ULAFrameCycles, giving the authentic ~50.08Hz; this remains only as display code's pre-first-frame fallback.
	ULAFrameCycles  = 69888                          // real interrupt-to-interrupt frame: 224 T-states x 312 lines (WoS FAQ; libspectrum agrees)
	InterruptLength = 32                             // T-states the ULA holds /INT low at the top of each frame
)

// ============================================================================
// CPU Wrapper for interface
// ============================================================================

// CPUWrapper wraps the Z80 CPU to implement the Z80CPU interface
type CPUWrapper struct {
	cpu *z80.Z80
}

func (cw *CPUWrapper) GetCycles() uint64 {
	return cw.cpu.Cycles
}

// ============================================================================
// Main ZenZX Emulator
// ============================================================================

// escapeMode is how the Escape key behaves when no menu or window is open for it to close:
// the saved setting "escape" (settings.json, and the Preferences window), or the -escape
// command-line switch for one session. The zero value is escapeOff, so that a ZenZX built
// without being told is one that Escape cannot quit.
type escapeMode int

const (
	// escapeOff: Escape never quits, and does nothing when nothing is open to close. The
	// default.
	escapeOff escapeMode = iota
	// escapeClose: Escape closes an open dialog, menu or the debugger HUD, and
	// never quits the program.
	escapeClose
	// escapeQuery: Escape asks for confirmation before quitting.
	escapeQuery
	// escapeOn: Escape quits the program at once.
	escapeOn
)

// escapeModeWords are the modes as the switch and settings.json spell them, in the order of
// the constants (and of settingsconfig.EscapeWords).
var escapeModeWords = [...]string{"off", "close", "query", "on"}

func (m escapeMode) String() string {
	if m < 0 || int(m) >= len(escapeModeWords) {
		return escapeModeWords[escapeOff]
	}
	return escapeModeWords[m]
}

// parseEscapeMode reads a mode word, from the -escape switch or from settings.json. An empty
// word is the default, off; a word that is not a mode is off as well and is reported to the
// caller, so that a mistyped switch never turns Escape on.
func parseEscapeMode(s string) (escapeMode, bool) {
	if s == "" {
		return escapeOff, true
	}
	for i, w := range escapeModeWords {
		if s == w {
			return escapeMode(i), true
		}
	}
	return escapeOff, false
}

type ZenZX struct {
	// Core components
	cpu    *z80.Z80
	memory *SpectrumMemory
	io     *SpectrumIO
	screen *SpectrumScreen

	// Per-instance frame timing. Defaults to the package-level PAL
	// constants at construction (NewZenZX); overridden only by
	// LoadTS2068ROM for genuine NTSC timing. Every non-TS2068 model's
	// behaviour is completely unchanged by this -- the values are
	// identical to the constants they replace in the hot paths
	// (RunFrame, display border-stripe timing, snapshot cycle
	// bookkeeping).
	cyclesPerFrame  int
	interruptLength int
	// frameOrigin is cpu.Cycles at the current frame's start -- the
	// frame-relative origin every piece of ULA timing (contention now;
	// the floating bus and border striping too) measures from.
	// Deriving this by modulo of the cumulative cycle count is wrong:
	// per-frame instruction overshoot and applied contention delays
	// make the two drift apart continuously.
	frameOrigin uint64

	// Turbo mode's fast-path lifecycle state (turbo.go). turboActive is
	// whether zx.cpu.FastMem/FastPort are currently populated (a
	// snapshot-in has run but the matching snapshot-out has not yet).
	// turboNextBoundary indexes into the currently-loading tape's own
	// BlockBoundaries -- which entry to watch Position against next.
	turboActive       bool
	turboNextBoundary int

	// auto is a pending autorun, nil once it has typed its keys: autorun.go.
	auto *autorun

	// typist is the phantom typist that starts a tape (phantomtypist.go). typistRunAhead is how many
	// extra frames the host runs for each of its own while it types, 0 to keep the clock exact;
	// typistMuted is whether the typist is what silenced the machine. customROM is whether a ROM of
	// the person's is in the machine, which the typist does not know the keys of.
	typist         *phantomTypist
	typistRunAhead int
	typistMuted    bool
	customROM      bool

	// rzx is an .rzx recording being played, nil when none is: rzx.go. OnRZX is where the host
	// hears of how it ended.
	rzx   *rzxReplay
	OnRZX func(msg string)

	// OnNotice is where the host hears what notice says (input.go).
	OnNotice func(msg string)

	// OpenSnapshot is how the host opens a snapshot: the window makes the machine it is of, or
	// reloads the one it made it for (snapshotopen_gui.go), and loads it there. nil loads it into
	// this machine, as the headless program does (openSnapshot, input.go).
	OpenSnapshot func(path, format string, wait bool) error

	// OpenMultisnapshot is how the host opens a multisnapshot (multisnapshot.go): the window asks
	// first, and replaces its machines with the ones in it. nil where there is no window to do it
	// (loadDroppedFile, input.go).
	OpenMultisnapshot func(path string)

	// cardsApproved is set by the window while it loads a snapshot of a Next over cards that the
	// person has chosen to go on with, though they are not the ones the snapshot was made over, or
	// with none in the second slot: the loader does not refuse it then (cardrecords.go).
	cardsApproved bool

	// OnDiagnostics is where the host hears of F3, which opens the diagnostics report in a
	// window as Snapshot > Run Diagnostics does (diagnostics_gui.go).
	OnDiagnostics func()

	// OnInfo is where the host hears of Alt+I and F12, which open a text window as the Tape and
	// Snapshot menus do (info_gui.go): the title and the text, whose lines starting "## " are headings.
	OnInfo func(title, text string)

	// nexRun is a .nex being run from the menu or the Browser, one frame at a time (nexrun.go);
	// nexScratch is the card made for it. OnNEXRun is where the host hears how it ended.
	nexRun     *nexRun
	nexScratch string
	OnNEXRun   func(msg string)
	// NEXAskReset, when set, is how the host asks the person whether a Next that is busy may
	// be reset to run a file (RunNEX); without it the file is not run.
	NEXAskReset func(path string)

	// Turbo mode's page-based physical memory model (turbo_pages.go,
	// turbo.go's turboSnapshotIn/turboRemap/turboSyncToReal). Mirrors
	// SpectrumMemory's own real rom[4]/ram[8] layout exactly: pages 0-3
	// are ROM banks 0-3, pages 4-11 are RAM banks 0-7.
	turboPageFlags [MemoryPageCount]MemoryFlags
	turboPages     [MemoryPageCount]MemoryPage16k
	// turboSlotPage[i] is the page index (0-11) currently mapped into
	// FastMem at logical slot i (0=0x0000, 1=0x4000, 2=0x8000, 3=0xC000).
	turboSlotPage [4]int
	// turboPort7FFD is the fast path's own tracked copy of the paging
	// register, updated immediately by FastPortWriteOut -- distinct from
	// SpectrumMemory's own port7FFD, which only updates via the real,
	// non-fast-path I/O route the fast path deliberately bypasses.
	turboPort7FFD uint8

	// Display management
	display Display

	// Audio support
	audio *AudioWrapper // Changed from *AudioManager to *AudioWrapper

	// Tape support
	tape *Tape

	// Timing
	cycleCount int
	frameCount int
	running    bool
	paused     bool
	// stopped is a machine that has been stopped (Machine > Stop) when it was the last: it
	// stays as the window's front, with nothing running and nothing drawn but the background,
	// until a machine is started in its place (machinedeck_gui.go). stopping is the few frames
	// of the stop animation before it, with the machine held where it was.
	stopped, stopping bool

	// System
	is128K  bool
	isPlus3 bool
	// escape is how the Escape key behaves when this machine starts: the -escape switch, or the
	// saved setting, or off (see escapeMode). The running program asks the menu bar (escapeNow),
	// since the Preferences window can change it at any time.
	escape escapeMode
	// uiOwnsEscape is set by the GUI's loop for a frame in which a menu or a
	// window of the menu bar is open: Escape closes that, not the program.
	uiOwnsEscape   bool
	scriptKeys     bool   // set by the GUI loop while a script has keys down (input.go)
	snapshotFormat string // Preferred snapshot format

	// next is the Next's boot chain when attachNextBootChain attached one (nextchain.go).
	next *nextChain
	// card2 is the card in the DivMMC's second slot, nil when it is empty (sdcard2.go).
	card2 *secondCard
	// onNextHandOver is called once, by serviceNextReset, when a Next's firmware has loaded
	// its ROMs, left config mode and reset the machine into them (nextstate.go). It clears
	// itself before it is called.
	onNextHandOver func()
	// romPatches are the ROM switches given on a Next, kept to be applied again when the
	// firmware has loaded its ROMs (rom_patches.go).
	romPatches romPatchSet
	// nextSettle is the watch for a Next that has settled, when one is wanted (nextsettle.go).
	nextSettle *nextSettleWatch

	// Non-standard features (see nonstandard.go). Set once at startup from
	// the validated -non-standard/-ns-* flags; not yet consumed by any
	// subsystem (see docs/TRACKING.md T-09, T-10, T-11).
	nonStandard NonStandardConfig

	// Active video renderer (see videorender.go). Defaults to the standard
	// renderer; SelectVideoRenderer switches it based on -ns-graphics.
	videoRenderer VideoRenderer
	// rendererSelected is set once SelectVideoRenderer has run, so a display
	// given to the machine afterwards is told (SetDisplay).
	rendererSelected bool

	// firstProgram is the first BASIC program the machine loaded, once it
	// has loaded one (program_name.go).
	firstProgram string

	// shots is what the machine's screenshots have under way: the GIF being recorded and the
	// ones that finished (screenshot_gif.go).
	shots shotState
}

func NewZenZX(audioBackend AudioBackend) *ZenZX {
	screen := NewSpectrumScreen()
	memory := NewSpectrumMemory(screen)

	// Create audio manager with selected backend
	audio := NewAudioWrapper(audioBackend)

	io := NewSpectrumIO(memory, audio) // Pass wrapper
	cpu := z80.New(memory, io)
	// A machine starts with nobody watching; SetDisplay gives it a display.
	display := newNullDisplay(screen)

	// Set CPU reference in IO for cycle tracking
	io.SetCPU(&CPUWrapper{cpu: cpu})

	zx := &ZenZX{
		cpu:             cpu,
		memory:          memory,
		io:              io,
		screen:          screen,
		display:         display,
		audio:           audio,
		running:         true,
		is128K:          false,
		isPlus3:         false,
		snapshotFormat:  "zxs", // Default format
		videoRenderer:   standardVideoRenderer{},
		cyclesPerFrame:  ULAFrameCycles,
		interruptLength: InterruptLength,
	}

	// EAR idle feedback and the floating bus both need live ZenZX
	// state (tape playing; frameOrigin + screen), so they wire as
	// closures here rather than static configuration.
	io.tapePlayingFn = func() bool {
		return zx.tape != nil && zx.tape.st != nil && zx.tape.st.Playing
	}
	io.floatingBusFn = zx.floatingBusByte
	io.nextVideoLineFn = zx.nextVideoLine       // NextREG 0x1E/0x1F, nextvideoline.go
	io.rasterEnable(zx.rasterTiming, zx.screen) // the Next's frame in pieces, and the copper

	// TS2068 dynamic video-mode switching (ts2068.go): a no-op for
	// every other model, since io.onTS2068VideoModeChange is only ever
	// consulted from ts2068WritePort, itself gated on io.memory.isTS2068.
	io.onTS2068VideoModeChange = func(modeBits uint8) {
		switch modeBits {
		case 0x00:
			zx.SelectVideoRenderer("") // standard
		case 0x02:
			zx.SelectVideoRenderer(NSGraphicsTimex001HiColour)
		default:
			// Not implemented (dual screen, 64-column, etc.) -- leave
			// whatever renderer is currently active rather than switch
			// to something wrong or error out of a port write.
		}
	}

	// Initialize tape system
	zx.tape = NewTape(zx)

	// Enable fast loader for tape
	fl := &FastLoader{Enabled: true}
	zx.tape.AttachFastLoader(fl)

	// Don't reset CPU here - let the caller do it after loading snapshot if needed

	// Ensure memory banking is properly initialized for 48K mode
	if !zx.is128K && !zx.isPlus3 {
		zx.memory.ramBankLow = 5
		zx.memory.ramBankHigh = 2
		zx.memory.ramBankTop = 0
		zx.memory.screenBank = 5 // This is correct but not "banking" in 48K
	}

	return zx
}

// EnableZ80N turns on the CPU core's Z80N (ZX Spectrum Next) extended
// instruction set. Off by default, matching zen80's own default -- none of
// ZenZX's existing -model options are the Next, and this method exists
// specifically for CPU-level Z80N verification (running a test binary and
// reading results back, e.g. via a "dump-mem" script action) rather than
// as a claim of Next hardware emulation (that is -model next: memory paging,
// sprites, the copper and the rest come with the machine, not with this). See -z80n in the headless and
// GUI entry points.
func (zx *ZenZX) EnableZ80N() {
	zx.cpu.Z80N = true
	// T-33: NEXTREG (ED 91/92) must write straight into the register
	// file without touching the port-0x243B select latch -- see
	// nextRegOpcodeWrite's doc comment in nextreg.go for why (real
	// hardware behaviour, and a real jnext bug/fix, GH #54).
	zx.cpu.NextregWrite = zx.io.nextRegOpcodeWrite
}

// DisableZ80N is the other way: the CPU is a plain Z80 again, as after a live switch from the Next to
// another model.
func (zx *ZenZX) DisableZ80N() {
	zx.cpu.Z80N = false
	zx.cpu.NextregWrite = nil
}

// EnableDivMMC attaches a DivMMC interface (T-36) to this instance:
// the automap/CONMEM memory overlay plus the SD-card-over-SPI
// transport (divmmc.go, sdcard.go, io.go's EnableDivMMC), AND wires
// the CPU's own M1Hook to drive automap paging -- the one piece
// SpectrumIO.EnableDivMMC can't do itself, since io.go only holds the
// narrow Z80CPU interface (SetCPU), never zen80's concrete *z80.Z80.
//
// M1Hook is a single func field, not a chain: this overwrites whatever
// hook was previously installed (e.g. the trace harness's own
// installM1Hook, zzz_trace_callstack_test.go) rather than composing
// with it. Harmless for this emulator's current uses -- DivMMC and the
// trace harness are never exercised in the same run -- but a real
// limitation if that ever changes; flagged rather than silently
// accepted.
func (zx *ZenZX) EnableDivMMC(sdImage SDCardImage, sdSize int64, romImage []byte) {
	zx.io.EnableDivMMC(sdImage, sdSize, romImage)
	zx.cpu.M1Hook = func(pc uint16, opcode uint8, context string) {
		if !zx.memory.mfPaged { // the Multiface is over the ROM: its code is at no entry point
			zx.io.divMMC.OnM1(pc, opcode, context)
		}
	}
	zx.io.divMMC.pcFn = func() uint16 { return zx.cpu.PC }
}

// LoadROMBytes is LoadROM's byte-data equivalent, used when the caller
// has already resolved the ROM's bytes (from the embedded binary or
// the filesystem -- see resolveROMBytes in zenzx_headless.go/
// zenzx_gui.go) rather than holding a bare path. LoadROM itself is now
// a thin wrapper around this.
func (zx *ZenZX) LoadROMBytes(data []byte) error {
	zx.memory.LoadROM(data)

	// Update ZenZX flags based on what was actually loaded
	zx.is128K = zx.memory.is128K
	zx.isPlus3 = zx.memory.isPlus3

	if zx.isPlus3 {
		sayln("+3 mode enabled (4 ROM banks)")
		zx.setupContention128() // see contention.go: known approximation for +3 too, not just 128K/+2
	} else if zx.is128K {
		sayln("128K mode enabled (2 ROM banks)")
		zx.setupContention128()
	} else {
		sayln("48K mode")
		zx.setupContention48()
	}

	return nil
}

func (zx *ZenZX) LoadROM(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return zx.LoadROMBytes(data)
}

// Load128KROMBytes is Load128KROM's byte-data equivalent.
func (zx *ZenZX) Load128KROMBytes(data0, data1 []byte) error {
	if len(data0) != 16384 {
		return fmt.Errorf("ROM 0 must be 16384 bytes, got %d", len(data0))
	}
	if len(data1) != 16384 {
		return fmt.Errorf("ROM 1 must be 16384 bytes, got %d", len(data1))
	}

	combined := make([]byte, 32768)
	copy(combined[0:16384], data0)
	copy(combined[16384:32768], data1)

	zx.memory.LoadROM(combined)
	zx.is128K = zx.memory.is128K
	zx.isPlus3 = zx.memory.isPlus3
	zx.setupContention128() // see contention.go: known approximation, not bank-aware

	return nil
}

// Load128KROM loads two separate 16KB ROM files for 128K models
func (zx *ZenZX) Load128KROM(rom0Path, rom1Path string) error {
	data0, err := os.ReadFile(rom0Path)
	if err != nil {
		return fmt.Errorf("error loading ROM 0: %v", err)
	}
	data1, err := os.ReadFile(rom1Path)
	if err != nil {
		return fmt.Errorf("error loading ROM 1: %v", err)
	}
	return zx.Load128KROMBytes(data0, data1)
}

// LoadPlus3ROMBytes is LoadPlus3ROM's byte-data equivalent.
func (zx *ZenZX) LoadPlus3ROMBytes(data0, data1, data2, data3 []byte) error {
	combined := make([]byte, 65536)
	copy(combined[0:16384], data0)
	copy(combined[16384:32768], data1)
	copy(combined[32768:49152], data2)
	copy(combined[49152:65536], data3)

	zx.memory.LoadROM(combined)
	zx.is128K = zx.memory.is128K
	zx.isPlus3 = zx.memory.isPlus3

	return nil
}

// LoadPlus3ROM loads four 16KB ROM files for +3 model
func (zx *ZenZX) LoadPlus3ROM(rom0Path, rom1Path, rom2Path, rom3Path string) error {
	data0, err := os.ReadFile(rom0Path)
	if err != nil {
		return fmt.Errorf("error loading ROM 0: %v", err)
	}

	data1, err := os.ReadFile(rom1Path)
	if err != nil {
		return fmt.Errorf("error loading ROM 1: %v", err)
	}

	data2, err := os.ReadFile(rom2Path)
	if err != nil {
		return fmt.Errorf("error loading ROM 2: %v", err)
	}

	data3, err := os.ReadFile(rom3Path)
	if err != nil {
		return fmt.Errorf("error loading ROM 3: %v", err)
	}

	return zx.LoadPlus3ROMBytes(data0, data1, data2, data3)
}

// maxROMBank returns the highest valid ROM bank index (0-based) for the
// currently loaded model -- used by OverrideROMBank to give a precise
// error rather than silently accepting, say, -rom2 on a 48K model.
func (zx *ZenZX) maxROMBank() int {
	if zx.memory.isTS2068 {
		return 1 // bank 0 = Home (16K), bank 1 = Extension (8K)
	}
	if zx.memory.isPlus3 {
		return 3 // banks 0-3
	}
	if zx.memory.is128K {
		return 1 // banks 0-1
	}
	return 0 // 48K: bank 0 only
}

// OverrideROMBank replaces a single ROM bank of the currently loaded
// model's ROM set in place, leaving every other bank exactly as -model
// loaded it -- the mechanism behind -rom0 through -rom3 and behind
// -rom's own multi-path form (zenzx_headless.go/zenzx_gui.go), letting
// a program swap e.g. just the +3DOS ROM (bank 3) on an otherwise-
// standard +3, or just TS2068's Extension ROM (bank 1) on an otherwise-
// standard TS2068, without needing to also respecify every other bank
// the way LoadROM/Load128KROM/LoadPlus3ROM/LoadTS2068ROM all require.
//
// Must be called after the model's own standard ROM set has already
// been loaded -- it edits banks in place, it doesn't establish which
// banks exist in the first place (that's is128K/isPlus3/isTS2068,
// already set by the -model load).
func (zx *ZenZX) OverrideROMBank(bank int, path string) error {
	if bank < 0 || bank > zx.maxROMBank() {
		return fmt.Errorf("rom bank %d not valid for this model (valid range: 0-%d)", bank, zx.maxROMBank())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading ROM bank %d override %s: %v", bank, path, err)
	}
	return zx.OverrideROMBankBytes(bank, data)
}

// OverrideROMBankBytes is OverrideROMBank for bytes already in hand: the ROM
// -rom-bin builds on the fly (rom_overlay.go) goes in this way, with no file
// between.
func (zx *ZenZX) OverrideROMBankBytes(bank int, data []byte) error {
	if bank < 0 || bank > zx.maxROMBank() {
		return fmt.Errorf("rom bank %d not valid for this model (valid range: 0-%d)", bank, zx.maxROMBank())
	}

	zx.customROM = true // the typist keeps off a machine with a ROM of the person's
	if zx.memory.isTS2068 {
		switch bank {
		case 0:
			if len(data) != 16384 {
				return fmt.Errorf("TS2068 Home ROM (bank 0) must be 16384 bytes, got %d", len(data))
			}
			copy(zx.memory.rom[0][:], data)
		case 1:
			if len(data) != 8192 {
				return fmt.Errorf("TS2068 Extension ROM (bank 1) must be 8192 bytes, got %d", len(data))
			}
			copy(zx.memory.ts2068ExtROM[:], data)
		}
		return nil
	}

	return zx.memory.SetROMBank(bank, data)
}

func (zx *ZenZX) Reset() {
	zx.rzxStop("")   // a reset gives up a recording that is playing
	zx.typist.stop() // and a tape that is being started: AutoloadTape starts it again after the reset
	zx.cpu.Reset()
	zx.cycleCount = 0
	zx.frameCount = 0

	// Reset audio cycle tracking
	if zx.audio != nil {
		zx.audio.UpdateCPUCycle(0)
	}

	// Reset keyboard
	if zx.io != nil {
		zx.io.ResetKeyboard()
	}

	// Stop tape if playing
	if zx.tape != nil && zx.tape.st != nil && zx.tape.st.Playing {
		zx.tape.Stop()
	}

	if zx.io != nil && zx.io.link != nil {
		zx.io.link.Reset()
	}

	// What a reset line puts back besides the CPU: reset.go.
	zx.resetHardware()

	// A paused machine runs no frame, which is where a Next's reset is carried out: it is carried out
	// here, or the machine would show, and be asked about, the state it had before the reset until Play.
	if zx.paused && zx.io != nil && zx.memory != nil && zx.memory.isNext {
		zx.serviceNextReset()
	}
}

func (zx *ZenZX) RunFrame() {
	if zx.paused {
		return
	}
	if zx.auto != nil {
		zx.autorunStep() // autorun.go
	}
	if zx.nexRun != nil {
		zx.nexRunStep() // nexrun.go
	}

	// Start new frame for border tracking
	zx.serviceNextReset()
	zx.nextFrameLength()
	zx.io.StartFrame()

	// New frame origin: all frame-relative ULA timing (contention,
	// border striping, later the floating bus) measures from here.
	zx.frameOrigin = zx.cpu.Cycles

	// A recording that is playing gives the frame its own length and its own interrupt: rzx.go.
	if zx.rzx != nil && zx.rzxFrame() {
		return
	}

	frameCycles := 0
	cycleUpdateCounter := 0

	// Maskable interrupt asserted at the START of the frame, as on real
	// hardware: the ULA holds /INT low for InterruptLength T-states at
	// the top of vertical retrace, and the whole frame-relative model
	// (contention from T-state 14335, first screen pixel at 14336) is
	// anchored to this moment. The previous scheme asserted INT at the
	// END of a 70000-cycle frame while contention wrapped a 69888
	// period of the cumulative counter -- two timebases precessing
	// against each other by 112+ T-states per frame, leaving the
	// contention window at a continuously rotating frame phase.
	zx.cpu.INT = !zx.io.nextIntRaise(nextIntULA) && !zx.io.nextULAIntOff() // NextREG 0x22 bit 2 turns the frame interrupt off; nextint.go

	for frameCycles < zx.interruptLength && zx.running {
		zx.checkAMXInterrupt()
		cycles := zx.stepCPU()
		frameCycles += cycles
		zx.cycleCount += cycles
		cycleUpdateCounter += cycles
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}

	// End of the ULA's /INT pulse. AMX exception as before: don't drop
	// a still-undelivered mouse step -- leave the line asserted so it
	// can be taken as soon as the guest re-enables interrupts.
	if zx.io.mouseMode != MouseAMX || (!zx.io.amxVectorPending && len(zx.io.amxIntQueue) == 0) {
		zx.cpu.INT = false
	}

	for frameCycles < zx.cyclesPerFrame && zx.running {
		zx.checkAMXInterrupt()
		cycles := zx.stepCPU()
		frameCycles += cycles
		zx.cycleCount += cycles
		cycleUpdateCounter += cycles
		if zx.io.nextLineInt.on {
			zx.lineInterrupt(frameCycles) // the Next's line interrupt: nextlineint.go
		}

		// Update audio system with current cycle count every ~500 cycles
		if cycleUpdateCounter >= 500 && zx.audio != nil && zx.audio.IsEnabled() {
			zx.audio.UpdateCPUCycle(zx.cpu.Cycles)
			cycleUpdateCounter = 0
		}

		// Tick the tape if present
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}

	// Final cycle update at end of frame
	if zx.audio != nil && zx.audio.IsEnabled() {
		zx.audio.UpdateCPUCycle(zx.cpu.Cycles)
	}

	// Pass border changes to display manager, anchored at the true
	// frame origin rather than a modulo-derived approximation.
	zx.display.SetBorderChanges(zx.io.GetBorderHistory(), zx.frameOrigin, zx.cyclesPerFrame)
	zx.io.EndFrame()

	zx.frameCount++
	zx.typistFrame() // the phantom typist's keys for the next frame (phantomtypist.go)
	zx.shotFrame()   // the GIF being recorded takes the frame (screenshot_gif.go)
}

// loadingFrame is a leaner RunFrame for the tape-loading phase specifically:
// identical CPU stepping and tape ticking (full correctness, no shortcuts),
// but skips per-instruction AMX-mouse and audio bookkeeping (irrelevant
// during headless tape loading) and the per-frame border-history-to-display
// handoff (nothing is rendering intermediate frames during loading anyway;
// io.StartFrame's own border-history Clear() still runs, so no state leaks
// across frames). Interrupt timing (the 50Hz INT each frame) is completely
// unchanged, since real games depend on it for correct behaviour even while
// loading.
func (zx *ZenZX) loadingFrame() {
	if zx.paused {
		return
	}
	zx.serviceNextReset()
	zx.nextFrameLength()
	zx.io.StartFrame()
	zx.io.raster.skip = true // no picture is kept of these frames; the copper still runs
	zx.frameOrigin = zx.cpu.Cycles
	frameCycles := 0

	zx.cpu.INT = !zx.io.nextIntRaise(nextIntULA) && !zx.io.nextULAIntOff()
	for frameCycles < zx.interruptLength && zx.running {
		cycles := zx.stepCPU()
		frameCycles += cycles
		zx.cycleCount += cycles
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}
	zx.cpu.INT = false

	for frameCycles < zx.cyclesPerFrame && zx.running {
		cycles := zx.stepCPU()
		frameCycles += cycles
		zx.cycleCount += cycles
		if zx.io.nextLineInt.on {
			zx.lineInterrupt(frameCycles)
		}
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}
	zx.io.EndFrame()

	zx.frameCount++
	zx.typistFrame()
	zx.shotFrame() // the GIF being recorded takes the frame (screenshot_gif.go)
}

// checkAMXInterrupt delivers the next queued AMX mouse step, if any, by
// asserting the CPU's interrupt line -- SpectrumIO.GetInterruptVector
// (mouse.go) supplies the correct vector (0xD0/0xD2) once zen80 actually
// accepts it. Only pops a new queue entry once the previous one has been
// delivered (amxVectorPending clear), so a burst of fast mouse movement
// can't overwrite an event that hasn't been picked up by the CPU yet.
// A no-op whenever AMX mode isn't active, at negligible per-instruction
// cost (one field read) for every other mode.
func (zx *ZenZX) checkAMXInterrupt() {
	if zx.io.mouseMode != MouseAMX || zx.io.amxVectorPending {
		return
	}
	if _, ok := zx.io.PopAMXInterrupt(); ok {
		zx.cpu.INT = true
	}
}

func (zx *ZenZX) Render() {
	zx.display.UpdateWindowSize()
	if b, ok := zx.display.(interface{ RenderBlank() }); ok && zx.stopped {
		b.RenderBlank() // a stopped machine shows the background and what is drawn over it, not a picture
		return
	}
	zx.display.Render(zx.paused, zx.memory, zx.screen, zx.io)
}

// SelectVideoRenderer resolves and activates the renderer for a
// -ns-graphics value ("" for standard), returning an error if no renderer
// is registered for it. See LookupVideoRenderer in videorender.go.
func (zx *ZenZX) SelectVideoRenderer(graphicsMode string) error {
	r, err := LookupVideoRenderer(graphicsMode)
	if err != nil {
		return err
	}
	zx.videoRenderer = r
	zx.rendererSelected = true
	zx.display.SetVideoRenderer(r)
	return nil
}

// SetDisplay gives the machine the display it draws through, and tells that
// display about the machine: its audio (for the debug overlay), its I/O (for
// the 640x256 Layer 2 mode's minimum-zoom clamp, see DisplayManager.io) and,
// if one has been selected, its video renderer. nil puts back the display that draws nothing.
func (zx *ZenZX) SetDisplay(d Display) {
	if d == nil {
		d = newNullDisplay(zx.screen)
	}
	d.SetAudioManager(zx.audio)
	d.SetSpectrumIO(zx.io)
	if zx.rendererSelected {
		d.SetVideoRenderer(zx.videoRenderer)
	}
	zx.display = d
}

// DecodeDisplay draws the picture of the machine as it is now. Both front ends can call it
// -- the headless build does, to encode a PNG -- so neither needs to know which renderer is
// active. It is 256x192 for every machine but the Next, whose picture is the whole 320x256
// frame, border included (640x256 with a double resolution layer): nextframe.go, and
// nextraster.go for what changes part of the way down it.
func (zx *ZenZX) DecodeDisplay() *image.RGBA {
	standard := func() *image.RGBA { return zx.videoRenderer.Decode(zx.memory, zx.screen) }
	if zx.io.nextFrameActive() {
		return zx.io.NextFrame(standard, zx.screen, zx.io.borderColor)
	}
	// On a Next in one of the Timex screen modes the picture is drawn by software
	// (nextsoftula.go).
	if ula := zx.io.nextSoftwareULA(standard, zx.screen); ula != nil {
		return ula
	}
	return standard()
}

// ============================================================================
// Audio Control Methods
// ============================================================================

// SetMasterVolume sets the master audio volume (0.0 to 1.0)
func (zx *ZenZX) SetMasterVolume(volume float32) {
	if zx.audio != nil {
		zx.audio.SetMasterVolume(volume)
	}
}

// SetBeeperVolume sets the beeper volume (0.0 to 1.0)
func (zx *ZenZX) SetBeeperVolume(volume float32) {
	if zx.audio != nil {
		zx.audio.SetBeeperVolume(volume)
	}
}

// SetAYVolume sets the AY chip volume (0.0 to 1.0)
func (zx *ZenZX) SetAYVolume(volume float32) {
	if zx.audio != nil {
		zx.audio.SetAYVolume(volume)
	}
}

// ToggleAudio toggles audio on/off
func (zx *ZenZX) ToggleAudio() {
	if zx.audio != nil {
		zx.audio.SetEnabled(!zx.audio.IsEnabled()) // Changed from !zx.audio.enabled
	}
}

// ============================================================================
// Main Entry Point
// ============================================================================
