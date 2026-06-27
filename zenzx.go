package main

import (
	"fmt"
	"os"

	"github.com/ha1tch/zen80/z80"
)

// ============================================================================
// Constants
// ============================================================================

const (
	// Timing
	CPUFrequency    = 3500000                        // 3.5 MHz
	FramesPerSecond = 50                             // 50 Hz PAL
	CyclesPerFrame  = CPUFrequency / FramesPerSecond // 70000
	InterruptCycles = 69888                          // Cycles before interrupt
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
		cpu:            cpu,
		memory:         memory,
		io:             io,
		screen:         screen,
		display:        display,
		audio:          audio,
		running:        true,
		is128K:         false,
		isPlus3:        false,
		snapshotFormat: "zxs", // Default format
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

func (zx *ZenZX) LoadROM(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	zx.memory.LoadROM(data)

	// Update ZenZX flags based on what was actually loaded
	zx.is128K = zx.memory.is128K
	zx.isPlus3 = zx.memory.isPlus3

	if zx.isPlus3 {
		fmt.Println("+3 mode enabled (4 ROM banks)")
	} else if zx.is128K {
		fmt.Println("128K mode enabled (2 ROM banks)")
	} else {
		fmt.Println("48K mode")
	}

	return nil
}

// Load128KROM loads two separate 16KB ROM files for 128K models
func (zx *ZenZX) Load128KROM(rom0Path, rom1Path string) error {
	data0, err := os.ReadFile(rom0Path)
	if err != nil {
		return fmt.Errorf("error loading ROM 0: %v", err)
	}
	if len(data0) != 16384 {
		return fmt.Errorf("ROM 0 must be 16384 bytes, got %d", len(data0))
	}

	data1, err := os.ReadFile(rom1Path)
	if err != nil {
		return fmt.Errorf("error loading ROM 1: %v", err)
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

	frameCycles := 0
	cycleUpdateCounter := 0

	// Run until interrupt time
	for frameCycles < InterruptCycles && zx.running {
		cycles := zx.cpu.Step()
		frameCycles += cycles
		zx.cycleCount += cycles
		cycleUpdateCounter += cycles

		// Update audio system with current cycle count every ~500 cycles (lightweight)
		if cycleUpdateCounter >= 500 && zx.audio != nil && zx.audio.IsEnabled() {
			zx.audio.UpdateCPUCycle(zx.cpu.Cycles)
			cycleUpdateCounter = 0
		}

		// Tick the tape if present
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}

	// Generate maskable interrupt
	zx.cpu.INT = true

	// Complete the frame
	for frameCycles < CyclesPerFrame && zx.running {
		cycles := zx.cpu.Step()
		frameCycles += cycles
		zx.cycleCount += cycles
		cycleUpdateCounter += cycles

		// Update audio system with current cycle count every ~500 cycles
		if cycleUpdateCounter >= 500 && zx.audio != nil && zx.audio.IsEnabled() {
			zx.audio.UpdateCPUCycle(zx.cpu.Cycles)
			cycleUpdateCounter = 0
		}

		// Continue ticking tape
		if zx.tape != nil {
			zx.tape.Tick(cycles)
		}
	}

	// Final cycle update at end of frame
	if zx.audio != nil && zx.audio.IsEnabled() {
		zx.audio.UpdateCPUCycle(zx.cpu.Cycles)
	}

	// Pass border changes to display manager
	zx.display.SetBorderChanges(zx.io.GetBorderHistory(), zx.cpu.Cycles)

	zx.cpu.INT = false
	zx.frameCount++
}

func (zx *ZenZX) Render() {
	zx.display.UpdateWindowSize()
	zx.display.Render(zx.paused)
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

