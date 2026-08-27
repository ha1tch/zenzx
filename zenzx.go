package main

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
	display *DisplayManager

	// Audio support
	audio *AudioWrapper // Changed from *AudioManager to *AudioWrapper

	// Tape support
	tape *Tape

	// Timing
	cycleCount int
	frameCount int
	running    bool
	paused     bool

	// System
	is128K         bool
	isPlus3        bool
	noEscKey       bool
	snapshotFormat string // Preferred snapshot format

	// Non-standard features (see nonstandard.go). Set once at startup from
	// the validated -non-standard/-ns-* flags; not yet consumed by any
	// subsystem (see docs/TRACKING.md T-09, T-10, T-11).
	nonStandard NonStandardConfig

	// Active video renderer (see videorender.go). Defaults to the standard
	// renderer; SelectVideoRenderer switches it based on -ns-graphics.
	videoRenderer VideoRenderer
}

func NewZenZX(audioBackend AudioBackend) *ZenZX {
	screen := NewSpectrumScreen()
	memory := NewSpectrumMemory(screen)

	// Create audio manager with selected backend
	audio := NewAudioWrapper(audioBackend)

	io := NewSpectrumIO(memory, audio) // Pass wrapper
	cpu := z80.New(memory, io)
	display := NewDisplayManager(screen)

	// Pass audio reference to display for debug overlay
	display.SetAudioManager(audio) // Pass wrapper

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
// as a claim of full Next hardware emulation (memory paging, sprites, the
// copper, and so on are all still absent). See -z80n in the headless and
// GUI entry points.
func (zx *ZenZX) EnableZ80N() {
	zx.cpu.Z80N = true
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
		fmt.Println("+3 mode enabled (4 ROM banks)")
		zx.setupContention128() // see contention.go: known approximation for +3 too, not just 128K/+2
	} else if zx.is128K {
		fmt.Println("128K mode enabled (2 ROM banks)")
		zx.setupContention128()
	} else {
		fmt.Println("48K mode")
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
}

func (zx *ZenZX) RunFrame() {
	if zx.paused {
		return
	}

	// Start new frame for border tracking
	zx.io.StartFrame()

	// New frame origin: all frame-relative ULA timing (contention,
	// border striping, later the floating bus) measures from here.
	zx.frameOrigin = zx.cpu.Cycles

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
	zx.cpu.INT = true

	for frameCycles < zx.interruptLength && zx.running {
		zx.checkAMXInterrupt()
		cycles := zx.cpu.Step()
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
		cycles := zx.cpu.Step()
		frameCycles += cycles
		zx.cycleCount += cycles
		cycleUpdateCounter += cycles

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

	zx.frameCount++
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
	zx.io.StartFrame()
	zx.frameOrigin = zx.cpu.Cycles
	frameCycles := 0

	zx.cpu.INT = true
	for frameCycles < zx.interruptLength && zx.running {
		cycles := zx.cpu.Step()
		frameCycles += cycles
		zx.cycleCount += cycles
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}
	zx.cpu.INT = false

	for frameCycles < zx.cyclesPerFrame && zx.running {
		cycles := zx.cpu.Step()
		frameCycles += cycles
		zx.cycleCount += cycles
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}

	zx.frameCount++
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
	zx.display.Render(zx.paused, zx.memory, zx.screen)
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
	zx.display.SetVideoRenderer(r)
	return nil
}

// DecodeDisplay renders the current display memory through the active
// video renderer into a 256x192 image.RGBA. Both front ends call this --
// the GUI to upload a texture each frame, headless to encode a PNG -- so
// neither needs to know which renderer is active.
func (zx *ZenZX) DecodeDisplay() *image.RGBA {
	return zx.videoRenderer.Decode(zx.memory, zx.screen)
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
