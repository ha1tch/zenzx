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
		maxSize: 256, // Limit to prevent runaway memory
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
	tapeEar     bool            // Current tape input level (bit 6 of port 0xFE)
	memory      *SpectrumMemory // Need reference to memory for paging
	ayRegister  uint8           // Selected AY-3-8912 register (128K sound chip)
	hasFDC      bool            // Floppy Disk Controller present (+3 only)
	fdc         *FDC765         // FDC implementation
	fdcDebug    bool            // Debug flag for FDC operations
	
	// Audio support
	audio       *AudioWrapper  // Reference to audio manager wrapper (changed from *AudioManager)
	ayRegisters [16]uint8      // AY register cache for snapshots
	
	// Border effect support
	borderHistory *BorderHistory // Track border changes per frame
	cpu           Z80CPU         // Reference to CPU for cycle count (interface, not pointer)
}

// Z80CPU interface to avoid circular dependency
type Z80CPU interface {
	GetCycles() uint64
}

func NewSpectrumIO(memory *SpectrumMemory, audio *AudioWrapper) *SpectrumIO {
	io := &SpectrumIO{
		memory:        memory,
		audio:         audio,  // Initialize audio reference (can be nil)
		hasFDC:        false,
		fdc:           nil,
		fdcDebug:      false,
		borderHistory: NewBorderHistory(),
	}
	for i := range io.keyboard {
		io.keyboard[i] = 0x1F // No keys pressed
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
	// ULA port (0xFE) - keyboard and tape
	if port&0x01 == 0 {
		result := uint8(0x1F)

		// Check keyboard rows
		for row := 0; row < 8; row++ {
			if port&(1<<uint(row+8)) == 0 {
				result &= io.keyboard[row]
			}
		}

		// Tape EAR bit (bit 6)
		if io.tapeEar {
			result |= 0x40
		}
		
		// Bit 7 is always 1
		result |= 0x80

		return result
	}

	// Kempston joystick
	if port&0xFF == 0x1F {
		return 0x00
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

	return 0xFF
}

func (io *SpectrumIO) WritePort(port uint16, value uint8) {
	// ULA port (0xFE) - border and speaker
	if port&0x01 == 0 {
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
	}

	// +3 memory paging port - 0x1FFD
	if port&0xF002 == 0x1000 {
		io.memory.SetPlus3Paging(value)
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

	// AY-3-8912 sound chip register select (128K) - 0xFFFD
	if port&0xC002 == 0xC000 {
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