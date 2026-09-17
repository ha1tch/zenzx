package main

import (
	"fmt"
	"os"
)

// ============================================================================
// I/O Implementation for Z80
// ============================================================================

// BorderChange records a border color change at a specific cycle
type BorderChange struct {
	Cycle uint64 // CPU cycle when change occurred
	Color uint8  // New border color (0-7)
}

// BorderHistory tracks border changes for current frame
type BorderHistory struct {
	changes []BorderChange
	maxSize int
}

// NewBorderHistory creates a border change tracker
func NewBorderHistory() *BorderHistory {
	return &BorderHistory{
		changes: make([]BorderChange, 0, 256), // Pre-allocate for performance
		maxSize: 256,                          // Limit to prevent runaway memory
	}
}

// Record adds a border change at the current cycle
func (bh *BorderHistory) Record(cycle uint64, color uint8) {
	if len(bh.changes) < bh.maxSize {
		bh.changes = append(bh.changes, BorderChange{
			Cycle: cycle,
			Color: color & 0x07,
		})
	}
}

// Clear resets the history for a new frame
func (bh *BorderHistory) Clear() {
	bh.changes = bh.changes[:0] // Reuse slice
}

// GetChanges returns the border changes for the current frame
func (bh *BorderHistory) GetChanges() []BorderChange {
	return bh.changes
}

type SpectrumIO struct {
	keyboard    [8]uint8
	borderColor uint8
	speaker     bool
	tapeIn      bool
	tapeEar     bool  // Current tape input level (bit 6 of port 0xFE)
	ulaLastOut  uint8 // Last byte written to the ULA port (0xFE); source of the idle EAR feedback
	issue2      bool  // Issue 2 EAR feedback (bit 6 = OUT bits 4|3); false = Issue 3 (bit 6 = OUT bit 4)
	// tapePlayingFn reports whether the tape is actively playing; wired
	// by NewZenZX. While playing the tape drives bit 6; otherwise the
	// idle level derived from ulaLastOut does -- the Speedlock-class
	// hardware check (write EAR/MIC, read bit 6 back) probes exactly
	// that feedback.
	tapePlayingFn func() bool
	// floatingBusFn returns the byte the ULA is fetching at the current
	// T-state (the 48K/128K floating bus); wired by NewZenZX. nil, or
	// non-fetch cycles, read as 0xFF.
	floatingBusFn func() uint8
	memory        *SpectrumMemory // Need reference to memory for paging
	ayRegister    uint8           // Selected AY-3-8912 register (128K sound chip)
	hasFDC        bool            // Floppy Disk Controller present (+3 only)
	fdc           *FDC765         // FDC implementation
	fdcDebug      bool            // Debug flag for FDC operations

	// Audio support
	audio       *AudioWrapper // Reference to audio manager wrapper (changed from *AudioManager)
	ayRegisters [16]uint8     // AY register cache for snapshots

	// Border effect support
	borderHistory *BorderHistory // Track border changes per frame
	cpu           Z80CPU         // Reference to CPU for cycle count (interface, not pointer)

	// Joystick support (joystick.go)
	joystickMode   JoystickMode
	kempstonState  uint8 // Current Kempston port 0x1F byte, updated by SetJoystickState
	kempston2State uint8 // Current second Kempston port 0x37 byte (neo-Spectrum platforms only, joystick.go)

	// Mouse support (mouse.go)
	mouseMode            MouseMode
	kempstonMouseX       uint8   // Wrapping X counter, port 0xFBDF
	kempstonMouseY       uint8   // Wrapping Y counter, port 0xFFDF
	kempstonMouseButtons uint8   // Port 0xFADF, active low, unused bits high
	mouseRemX, mouseRemY float32 // Fractional remainder carried between frames
	amxMouseButtons      uint8   // Port 0xDF bits 5-7, active high
	amxPortValue         uint8   // What port 0x1F/0x3F return until the next popped request
	amxIntQueue          []amxInterruptRequest
	amxPendingVector     uint8 // Vector GetInterruptVector should hand out next
	amxVectorPending     bool

	// TS2068 model support (ts2068.go)
	ts2068HSR      uint8 // Last value written to port F4H
	ts2068Port0xFF uint8 // Last value written to port FFH

	// onTS2068VideoModeChange, if set, is called whenever TS2068 mode
	// guest code writes new video-mode bits (0-2) to port FFH -- lets a
	// running program dynamically switch the active renderer itself
	// (real hardware behaviour: real software engaged hi-colour mode via
	// a direct OUT to this port, not a documented ROM service call).
	// Set by NewZenZX as a closure over the owning ZenZX, since
	// SelectVideoRenderer lives there, not on SpectrumIO.
	onTS2068VideoModeChange func(modeBits uint8)

	// ts2068Joy1/Joy2 hold the current state of TS2068's own built-in
	// joystick ports, read via AY register 14 (ts2068.go).
	ts2068Joy1, ts2068Joy2 JoystickState

	// ZX Spectrum Next NextREG register bus (nextreg.go, T-27/ti0.r1).
	// Present on every SpectrumIO regardless of the active machine
	// model -- there is no separate "next" machine type yet (that is
	// ti0.r0, deliberately sequenced outside this dependency chain) --
	// so the register file is reachable via 0x243B/0x253B the same way
	// on any model, matching how the wave item itself is scoped
	// (the port handler, not machine-type gating).
	nextRegs       *NextRegisterFile
	nextRegClipWin *nextRegClip

	// sprites is T-30 (wave ti0.r4)'s attribute table and pattern memory
	// (sprite.go), constructed alongside nextRegs/nextRegClipWin by
	// NewSpectrumIO -- never nil on a SpectrumIO built that way. Present
	// unconditionally, the same as nextRegs, since there is no separate
	// Next machine type gating registers themselves (see nextRegs' own
	// doc comment above).
	sprites *spriteSystem

	// nextPalette is T-31 (wave ti0.r5)'s full 8-palette, 256-entry-each
	// Next colour storage (nextpalette.go), constructed alongside
	// nextRegs/nextRegClipWin/sprites by NewSpectrumIO -- never nil on a
	// SpectrumIO built that way, same convention as those.
	nextPalette *nextPaletteSystem

	// layer2Access is IO port 0x123B's decoded state (layer2accessport.go,
	// T-29 completion pass): CPU-side memory paging into Layer 2 memory,
	// entirely separate from which bank is displayed (always NR 0x12).
	// Zero-value on construction means both readEnabled/writeEnabled are
	// false, matching real hardware's power-on state of not intercepting
	// the normal memory map until this port is explicitly written.
	layer2Access layer2AccessPort

	// nextPaletteVersion is the GPU-cache-invalidation hook proposed in
	// docs/proposals/next-gpu-compositing-seam.md: incremented on every
	// write that could change what a resolved Next-mode pixel colour
	// means, so a GPU texture cache keyed on resolved colour (Layer 2's
	// renderLayer2GPU today, a future sprite pattern cache) can compare
	// its own baked-against version against this live value and rebake
	// only when they diverge, rather than every frame regardless of
	// whether anything changed. Two register groups bump it: NR 0x70
	// (the Layer 2 palette offset -- see resolveNextPixelIndex and
	// nextRegWriteDirect/SetNextRegister's own handling of 0x70, since a
	// fixed raw byte resolves to a different index once the offset
	// changes), and, as of T-31 (wave ti0.r5), NR 0x41/0x43/0x44 (a
	// genuine palette-entry write, or a change of which palette a layer
	// displays via 0x43's display-select bits -- see
	// nextRegWriteDirect/SetNextRegister's own 0x40/0x41/0x43/0x44
	// handling in nextreg.go). NR 0x40 (index select alone) and a
	// buffered, non-completing first byte of an NR 0x44 pair do NOT bump
	// this -- see nextRegWriteDirect's own 0x44 handling for why only the
	// completing write counts. renderLayer2GPU (videorender_gpu.go) is
	// the first real consumer: its layer2ColourGPUVersion field compares
	// against this live value and rebakes only when they diverge, rather
	// than every frame regardless of whether anything changed.
	nextPaletteVersion uint32
}

// Z80CPU interface to avoid circular dependency
type Z80CPU interface {
	GetCycles() uint64
}

func NewSpectrumIO(memory *SpectrumMemory, audio *AudioWrapper) *SpectrumIO {
	io := &SpectrumIO{
		memory:         memory,
		audio:          audio, // Initialize audio reference (can be nil)
		hasFDC:         false,
		fdc:            nil,
		fdcDebug:       false,
		borderHistory:  NewBorderHistory(),
		nextRegs:       newNextRegisterFile(),
		nextRegClipWin: newNextRegClip(),
		sprites:        newSpriteSystem(),
		nextPalette:    newNextPaletteSystem(),
	}
	for i := range io.keyboard {
		io.keyboard[i] = 0x1F // No keys pressed
	}
	io.kempstonMouseButtons = 0xFF // Active low: no buttons pressed
	// T-28 item #26: newNextRegisterFile's mmuBootDefaults seed
	// io.nextRegs.mmuSlots directly (no *SpectrumMemory reference exists
	// inside that standalone constructor), so without this the paging
	// model's nextSlots would sit at its own zero-value default -- all
	// slots pointing at page 0 -- silently disagreeing with the register
	// file's real boot defaults (ROM sentinel in slots 0/1, classic
	// 128K-layout mirror in slots 2-7) until the first NR 0x50-0x57
	// write. Push the already-computed defaults into memory here so the
	// two stay in sync from construction, the same way every later
	// write does via nextRegWriteDirect/SetNextRegister.
	if memory != nil {
		for i, page := range io.nextRegs.mmuSlots {
			memory.SetNextSlot(i, page)
		}
		// Layer 2 Access Port (T-29 completion pass): wire memory's
		// readNext/writeNext hook back to this io, the same
		// closure-injection direction memory.go's own field comment
		// describes. See layer2AccessPortRead/Write's own doc comments
		// (layer2accessport.go) for the ok/handled contract these
		// closures must honour.
		memory.layer2AccessReadFn = io.layer2AccessPortRead
		memory.layer2AccessWriteFn = io.layer2AccessPortWrite
	}
	return io
}

// SetCPU sets the CPU reference for cycle tracking
func (io *SpectrumIO) SetCPU(cpu Z80CPU) {
	io.cpu = cpu
}

// ============================================================================
// Z80 I/O Interface Implementation
// ============================================================================

func (io *SpectrumIO) In(port uint16) uint8 {
	return io.ReadPort(port)
}

func (io *SpectrumIO) Out(port uint16, value uint8) {
	io.WritePort(port, value)
}

// ============================================================================
// Port I/O Implementation
// ============================================================================

func (io *SpectrumIO) ReadPort(port uint16) uint8 {
	// ZX Spectrum Next NextREG ports (0x243B/0x253B, nextreg.go): fully
	// 16-bit decoded on real hardware, checked first so no partial-decode
	// branch below (which only matches a handful of address bits) can
	// ever shadow them -- both port values are odd, so they don't
	// collide with the ULA's even-port branch either way, but matching
	// exactly first keeps this correct regardless of decode order.
	if port == nextRegSelectPort {
		return io.nextRegSelected()
	}
	if port == nextRegDataPort {
		return io.nextRegReadSelected()
	}
	if port == layer2AccessPortAddr {
		// Read-back of the port's own last-written control byte -- no
		// source consulted documents this port returning anything other
		// than its own stored value on a read (unlike, say, NR 0x41's
		// documented "current entry" read-back semantics), so this is
		// the plain, unsurprising choice.
		return io.layer2Access.raw
	}

	// TS2068 HSR (F4H) and display-enhancement-control (FFH) -- must be
	// checked before the ULA keyboard branch below: F4H is even (bit0=0)
	// and would otherwise be swallowed by the "any even port = keyboard"
	// catch-all that's correct for the standard ULA's incomplete decode
	// but wrong here, since TS2068's SCLD chip decodes F4H specifically,
	// distinct from FEH (Technical Manual Table 2.1.13-1).
	if v, ok := io.ts2068ReadPort(port); ok {
		return v
	}

	// ULA port (0xFE) - keyboard and tape
	if port&0x01 == 0 {
		result := uint8(0x1F)

		// Check keyboard rows
		for row := 0; row < 8; row++ {
			if port&(1<<uint(row+8)) == 0 {
				result &= io.keyboard[row]
			}
		}

		if io.memory != nil && io.memory.isTS2068 {
			// TS2068: legacy behaviour, deliberately unchanged (bit 6
			// tape-driven, bit 7 high); its real idle levels are
			// unverified here (FUSE uses a fixed 0x5F for Timex).
			if io.tapeEar {
				result |= 0x40
			}
			return result | 0x80
		}

		// Bits 5 and 7 read high on real Spectrums. Bit 6 (EAR): the
		// tape drives it while playing; otherwise it presents the idle
		// level fed back from the last ULA OUT -- Issue 3: OUT bit 4;
		// Issue 2: OUT bits 4|3; +2A/+3: no feedback, always low.
		// Returning a frozen last-tape-level here is not just
		// imprecise: Speedlock-class hardware checks detect it.
		result |= 0xA0
		if io.tapePlayingFn != nil && io.tapePlayingFn() {
			if io.tapeEar {
				result |= 0x40
			}
		} else if io.memory == nil || !io.memory.isPlus3 {
			feedback := io.ulaLastOut & 0x10
			if io.issue2 {
				feedback = io.ulaLastOut & 0x18
			}
			if feedback != 0 {
				result |= 0x40
			}
		}

		return result
	}

	// AMX mouse X/Y (session work, T-14): interrupt-driven, not a
	// position register -- ports return whatever queueAMXInterrupt's
	// last popped request left in amxPortValue (bit0 = direction),
	// current only immediately after the corresponding interrupt has
	// been delivered, exactly matching real hardware's own handlers
	// (which only ever read the port from inside their own ISR).
	// Takes priority over Kempston joystick on 0x1F when AMX mode is
	// active -- real hardware has the same port conflict; ZenZX wiring
	// rejects selecting both simultaneously at startup.
	if io.mouseMode == MouseAMX {
		if port&0xFF == 0x1F {
			return io.amxPortValue
		}
		if port&0xFF == 0x3F {
			return io.amxPortValue
		}
		if port&0xFF == 0xDF {
			return io.amxMouseButtons
		}
	}

	// Kempston joystick (first port)
	if port&0xFF == 0x1F {
		return io.kempstonState
	}

	// Second Kempston port (0x37) -- confirmed against the ZX Spectrum
	// Next's own I/O port register documentation and cross-checked
	// against the "KEMPSTON_MAX 2" hobbyist interface's own port
	// numbering (joystick.go). No classic-era hardware has this; only
	// relevant when JoystickKempston2/JoystickKempstonBoth is explicitly
	// configured, in which case io.kempston2State holds real data --
	// otherwise it's simply always zero, indistinguishable from "nothing
	// connected here."
	if port&0xFF == 0x37 {
		return io.kempston2State
	}

	// Kempston mouse (0xFADF/0xFBDF/0xFFDF -- upper 4 address bits are
	// don't-care on real hardware, matched here via a 12-bit mask, per
	// the partial-decode table documented at spectrumcomputing.co.uk)
	if port&0x0FFF == 0x0BDF {
		return io.kempstonMouseX
	}
	if port&0x0FFF == 0x0FDF {
		return io.kempstonMouseY
	}
	if port&0x0FFF == 0x0ADF {
		return io.kempstonMouseButtons
	}

	// +3 FDC ports
	if io.memory.isPlus3 {
		if io.fdcDebug && ((port&0xF000) == 0x2000 || (port&0xF000) == 0x3000) {
			fmt.Printf("Port read: 0x%04X (A0=%d, A7=%d, A10=%d, bits 12-13=%02b)\n",
				port, port&1, (port>>7)&1, (port>>10)&1, (port>>12)&3)
		}

		// Check for FDC status register
		if (port&0x0001) == 0x0001 && (port&0x3000) == 0x2000 {
			if io.hasFDC && io.fdc != nil {
				status := io.fdc.ReadStatus()
				if io.fdcDebug {
					fmt.Printf("FDC Status Read: port=0x%04X status=0x%02X\n", port, status)
				}
				return status
			}
			return 0xFF
		}

		// Check for FDC data register
		if (port&0x0001) == 0x0001 && (port&0x3000) == 0x3000 {
			if io.hasFDC && io.fdc != nil {
				data := io.fdc.ReadData()
				if io.fdcDebug {
					fmt.Printf("FDC Data Read: port=0x%04X data=0x%02X\n", port, data)
				}
				return data
			}
			return 0xFF
		}
	}

	// AY-3-8912 sound chip data read (128K)
	if port&0xC002 == 0xC000 {
		if io.audio != nil && io.ayRegister < 16 {
			// Use the wrapper's thread-safe read method
			value := io.audio.ReadAYRegister(io.ayRegister)
			return value
		}
		return 0xFF
	}

	// Unattached port: the 48K/128K data bus floats to whatever byte
	// the ULA is fetching at this T-state (idle 0xFF outside fetch
	// cycles). Ocean-era code polls this for raster sync.
	if io.floatingBusFn != nil {
		return io.floatingBusFn()
	}
	return 0xFF
}

func (io *SpectrumIO) WritePort(port uint16, value uint8) {
	// ZX Spectrum Next NextREG ports: see the matching comment in
	// ReadPort above.
	if port == nextRegSelectPort {
		io.nextRegSelect(value)
		return
	}
	if port == nextRegDataPort {
		io.nextRegWriteSelected(value)
		return
	}
	if port == layer2AccessPortAddr {
		io.layer2Access.raw = value
		return
	}

	// TS2068 HSR (F4H) and display-enhancement-control (FFH) -- same
	// ordering reason as ReadPort above: F4H is even and must not be
	// swallowed by the ULA border/speaker catch-all.
	if io.ts2068WritePort(port, value) {
		return
	}

	// ULA port (0xFE) - border and speaker
	if port&0x01 == 0 {
		io.ulaLastOut = value // idle EAR feedback source (see In)
		newBorderColor := value & 0x07
		newSpeaker := (value & 0x10) != 0

		// Track border color change
		if newBorderColor != io.borderColor && io.cpu != nil {
			io.borderHistory.Record(io.cpu.GetCycles(), newBorderColor)
		}

		io.borderColor = newBorderColor

		// Update speaker state with cycle-accurate timing
		if newSpeaker != io.speaker {
			io.speaker = newSpeaker
			// Notify audio manager of speaker change with current cycle count
			if io.audio != nil && io.cpu != nil {
				io.audio.UpdateSpeaker(newSpeaker, io.cpu.GetCycles())
			}
		}
	}

	// Memory paging port (128K/+2) - 0x7FFD
	if port&0xC002 == 0x4000 {
		io.memory.SetPaging(value)
		// T-28 item #27: on a genuine Next machine, this same physical
		// port also recomposes the Next MMU's own slots 6/7 -- see
		// SetNextLegacyBank7FFD's doc comment (memory.go). A no-op
		// (isNext gate inside the method) on every classic
		// 48K/128K/+3/TS2068 machine; does not read or affect anything
		// SetPaging just did above.
		io.memory.SetNextLegacyBank7FFD(value)
	}

	// +3 memory paging port - 0x1FFD
	if port&0xF002 == 0x1000 {
		io.memory.SetPlus3Paging(value)
	}

	// Next/Profi extended paging port - 0xDFFD (T-28 item #27). Decode
	// per jnext's own VHDL citation (src/memory/mmu.h, zxnext.vhd:2596):
	// A15:12="1101" AND A1:0="01". Has no defined role on any non-Next
	// machine this emulator models -- SetNextLegacyPortDFFD is a no-op
	// unless isNext (memory.go).
	if port&0xF003 == 0xD001 {
		io.memory.SetNextLegacyPortDFFD(value)
	}

	// +3 FDC control ports
	if io.memory.isPlus3 {
		if (port&0x0081) == 0x0081 && (port&0xF400) == 0x3400 {
			if io.hasFDC && io.fdc != nil {
				if io.fdcDebug {
					fmt.Printf("FDC Data Write: port=%04X value=%02X\n", port, value)
				}
				io.fdc.WriteData(value)
			}
		}
	}

	// AY-3-8912 sound chip register select (128K) - 0xFFFD. T-28 item
	// #27: this coarse address-line decode (A15:14="11", A1="1",
	// A0="0") also matches the literal address 0xDFFD
	// (0xDFFD&0xC002==0xC000), which is now a genuine, separately
	// decoded Next/Profi extended-paging port (see the 0xDFFD block
	// above). Without the exclusion below, a Next-mode 0xDFFD write
	// would ALSO be misread here as an AY register-select write,
	// silently corrupting io.ayRegister. The exclusion is exact-address
	// and isNext-gated, so it is provably a no-op on every classic
	// 48K/128K/+3/TS2068 machine: isNext is always false there, making
	// `!(false && ...)` unconditionally true and leaving this condition
	// identical to its pre-existing form.
	if port&0xC002 == 0xC000 && !(io.memory.isNext && port == 0xDFFD) {
		io.ayRegister = value & 0x0F
	}

	// AY-3-8912 sound chip data write (128K) - 0xBFFD
	if port&0xC002 == 0x8000 {
		if io.ayRegister < 16 {
			// Update audio chip if available using thread-safe wrapper method
			if io.audio != nil {
				io.audio.WriteAYRegister(io.ayRegister, value)
			}
			// Always cache for snapshots
			io.ayRegisters[io.ayRegister] = value
		}
	}
}

// ============================================================================
// Border History Functions
// ============================================================================

// StartFrame prepares for a new frame's border tracking
func (io *SpectrumIO) StartFrame() {
	io.borderHistory.Clear()
	// Record initial border color at start of frame
	if io.cpu != nil {
		io.borderHistory.Record(io.cpu.GetCycles(), io.borderColor)
	}
}

// GetBorderHistory returns the border change history for rendering
func (io *SpectrumIO) GetBorderHistory() []BorderChange {
	return io.borderHistory.GetChanges()
}

// ============================================================================
// Keyboard Functions
// ============================================================================

func (io *SpectrumIO) PressKey(row, col uint8) {
	if row < 8 && col < 5 {
		io.keyboard[row] &^= (1 << col)
	}
}

func (io *SpectrumIO) ReleaseKey(row, col uint8) {
	if row < 8 && col < 5 {
		io.keyboard[row] |= (1 << col)
	}
}

func (io *SpectrumIO) ResetKeyboard() {
	for i := range io.keyboard {
		io.keyboard[i] = 0x1F // No keys pressed
	}
}

// ============================================================================
// Audio State Functions
// ============================================================================

// GetAYRegisters returns a copy of the cached AY registers
func (io *SpectrumIO) GetAYRegisters() [16]uint8 {
	return io.ayRegisters
}

// SetAYRegisters restores AY registers from a snapshot
func (io *SpectrumIO) SetAYRegisters(registers [16]uint8) {
	io.ayRegisters = registers

	// Update the actual AY chip if available via wrapper
	if io.audio != nil {
		for i := 0; i < 16; i++ {
			io.audio.WriteAYRegister(uint8(i), registers[i])
		}
	}
}

// ============================================================================
// FDC Functions
// ============================================================================

// EnableFDC enables Floppy Disk Controller emulation for +3
func (io *SpectrumIO) EnableFDC() {
	if io.fdc == nil {
		io.fdc = NewFDC765()
	}
	io.hasFDC = true
	if io.fdc != nil {
		io.fdc.EnableDebug(io.fdcDebug)
	}
	fmt.Println("FDC enabled, hasFDC =", io.hasFDC)
}

// DisableFDC disables Floppy Disk Controller emulation
func (io *SpectrumIO) DisableFDC() {
	io.hasFDC = false
	fmt.Println("FDC disabled, hasFDC =", io.hasFDC)
}

// SetFDCDebug enables or disables FDC debug output
func (io *SpectrumIO) SetFDCDebug(enable bool) {
	io.fdcDebug = enable
	if io.fdc != nil {
		io.fdc.EnableDebug(enable)
	}
	fmt.Printf("FDC debug set to %v\n", enable)
}

// LoadDisk loads a disk image into the FDC
func (io *SpectrumIO) LoadDisk(filename string) error {
	if io.fdc == nil {
		io.fdc = NewFDC765()
	}
	io.hasFDC = true
	return io.fdc.LoadDisk(filename)
}

// FDCTick advances the FDC frame clock and runs the auto-commit debounce. The
// front-end calls it once per frame. A modified disk is flushed to its file
// after it has been quiet for debounceFrames and the controller is idle.
func (io *SpectrumIO) FDCTick(frame, debounceFrames int) {
	if io.fdc == nil || !io.hasFDC {
		return
	}
	io.fdc.Tick(frame)
	if io.fdc.AutoCommit(debounceFrames) {
		fmt.Printf("Disk auto-committed: %s\n", io.fdc.diskFilename)
	}
}

// SaveDisk saves the current disk image
func (io *SpectrumIO) SaveDisk() error {
	if io.fdc != nil && io.hasFDC {
		return io.fdc.SaveDisk()
	}
	return nil
}

// SaveDiskAs saves the current disk image with a new filename
func (io *SpectrumIO) SaveDiskAs(filename string) error {
	if io.fdc != nil && io.hasFDC && io.fdc.HasDisk() {
		err := os.WriteFile(filename, io.fdc.diskImage, 0644)
		if err != nil {
			return err
		}
		io.fdc.diskFilename = filename
		io.fdc.diskModified = false
		return nil
	}
	return fmt.Errorf("no disk to save")
}

// ============================================================================
// Tape Functions
// ============================================================================

// SetTapeEar sets the tape input level
func (io *SpectrumIO) SetTapeEar(level bool) {
	io.tapeEar = level
}

// GetTapeEar returns the current tape input level
func (io *SpectrumIO) GetTapeEar() bool {
	return io.tapeEar
}

// ============================================================================
// State Access Functions
// ============================================================================

// GetBorderColor returns the current border color
func (io *SpectrumIO) GetBorderColor() uint8 {
	return io.borderColor
}

// SetBorderColor sets the border color
func (io *SpectrumIO) SetBorderColor(color uint8) {
	io.borderColor = color & 0x07
}

// GetSpeaker returns the current speaker state
func (io *SpectrumIO) GetSpeaker() bool {
	return io.speaker
}

// GetAYRegister returns the currently selected AY register
func (io *SpectrumIO) GetAYRegister() uint8 {
	return io.ayRegister
}

// GetKeyboardState returns a copy of the keyboard matrix
func (io *SpectrumIO) GetKeyboardState() [8]uint8 {
	return io.keyboard
}
