package main

import (
	"fmt"
	"os"
)

// +3 Disk format constants
const (
	SectorsPerTrack = 9
	BytesPerSector  = 512
	TracksPerSide   = 40
	Sides           = 2                                                        // Double-sided for +3
	DiskImageSize   = TracksPerSide * Sides * SectorsPerTrack * BytesPerSector // 180KB
)

// FDC765 implements a basic NEC 765 Floppy Disk Controller
type FDC765 struct {
	// Disk image
	diskImage    []byte
	diskFilename string
	diskPresent  bool
	diskModified bool

	// FDC registers
	mainStatus uint8
	dataReg    uint8

	// Command processing
	commandBuf   []uint8
	commandLen   int
	commandPhase int // 0=command, 1=execution, 2=result
	resultBuf    []uint8
	resultIdx    int

	// Current operation
	currentTrack    uint8
	currentHead     uint8
	currentSector   uint8
	currentCylinder uint8

	// Data transfer
	dataBuffer []uint8
	dataIdx    int
	dataLen    int

	// Motor and drive status
	motorOn     bool
	driveSelect uint8

	// Status registers (ST0, ST1, ST2, ST3)
	st0 uint8
	st1 uint8
	st2 uint8
	st3 uint8

	// Interrupt status for Sense Interrupt Status command
	// The +3 ROM expects exactly 4 reset interrupts (one per drive)
	resetCount int // Number of reset interrupts reported
	seekEnd    bool

	// Debug flag
	debug bool
}

// CreateBlankDisk creates a blank formatted +3 disk
func (fdc *FDC765) CreateBlankDisk() {
	// Initialize with 0xE5 (standard formatting byte for CP/M and +3DOS)
	for i := range fdc.diskImage {
		fdc.diskImage[i] = 0xE5
	}

	// Mark as having a disk present but not modified (it's blank)
	fdc.diskPresent = true
	fdc.diskModified = false
	fdc.diskFilename = ""

	// Update ST3 to indicate disk is ready
	fdc.st3 |= 0x20 // Set Ready bit

	if fdc.debug {
		fmt.Println("FDC: Created blank formatted disk")
	}
}

// EjectDisk removes the current disk
func (fdc *FDC765) EjectDisk() {
	// Save if modified
	if fdc.diskModified && fdc.diskFilename != "" {
		fdc.SaveDisk()
	}

	// Clear disk
	fdc.diskPresent = false
	fdc.diskModified = false
	fdc.diskFilename = ""

	// Update ST3 to indicate no disk
	fdc.st3 &^= 0x20 // Clear Ready bit

	if fdc.debug {
		fmt.Println("FDC: Disk ejected")
	}
}

// HasDisk returns true if a disk is present
func (fdc *FDC765) HasDisk() bool {
	return fdc.diskPresent
}

// IsModified returns true if the disk has been modified
func (fdc *FDC765) IsModified() bool {
	return fdc.diskModified
}

func NewFDC765() *FDC765 {
	fdc := &FDC765{
		diskImage: make([]byte, DiskImageSize),
		// Main status register initial state:
		// Bit 7: RQM (Request for Master) = 1 (ready)
		// Bit 6: DIO (Data Input/Output) = 0 (CPU -> FDC)
		// Bit 5: EXM (Execution Mode) = 0 (not in execution phase)
		// Bit 4: CB (FDC Busy) = 0 (not busy)
		// Bits 3-0: Drive busy status = 0 (no drives busy)
		mainStatus: 0x80, // RQM=1, ready for command

		// After reset, need to report 4 interrupts (one per drive)
		resetCount: 4,

		// ST0 initial state (after reset):
		// Bits 7-6: IC (Interrupt Code) = 11 (abnormal termination/reset)
		// Bit 5: SE (Seek End) = 0
		// Bit 4: EC (Equipment Check) = 0
		// Bit 3: Not used
		// Bit 2: H (Head Address) = 0
		// Bits 1-0: Drive Select = 0
		st0: 0xC0, // Reset/abnormal termination state

		// ST3 initial state:
		// Bit 7: FT (Fault) = 0
		// Bit 6: WP (Write Protect) = 0
		// Bit 5: RY (Ready) = 1 (drive is ready, even without disk)
		// Bit 4: T0 (Track 0) = 1 initially
		// Bit 3: TS (Two Side) = 1 for double-sided
		// Bit 2: HD (Head) = 0
		// Bits 1-0: US (Unit Select) = 0
		st3: 0x38, // Track 0, Two-sided, not ready initially

		seekEnd: false,
		debug:   false, // Set to true to enable debug output

		diskPresent: false, // No disk initially
	}

	// The drive hardware is always "ready" even without a disk
	// This allows the +3 to detect the FDC
	fdc.st3 |= 0x20 // Set Ready bit

	return fdc
}

// EnableDebug enables debug output
func (fdc *FDC765) EnableDebug(enable bool) {
	fdc.debug = enable
}

// LoadDisk loads a disk image from file
func (fdc *FDC765) LoadDisk(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Check for standard +3 disk size or extended DSK format
	if len(data) == DiskImageSize {
		// Standard 180KB disk
		copy(fdc.diskImage, data)
		fdc.diskPresent = true
		fdc.diskFilename = filename
		fdc.diskModified = false
		// Update ST3 to indicate disk is ready
		fdc.st3 |= 0x20 // Set Ready bit
		fmt.Printf("Loaded disk: %s (180KB)\n", filename)
		return nil
	} else if len(data) > 256 && string(data[0:8]) == "EXTENDED" {
		// Extended DSK format (with header)
		// Skip header and load track data
		// This is a simplified implementation
		fmt.Printf("Loaded extended DSK: %s\n", filename)
		// TODO: Properly parse extended DSK format
		fdc.diskPresent = true
		fdc.diskFilename = filename
		fdc.st3 |= 0x20 // Set Ready bit
		return nil
	} else if len(data) > 256 && string(data[0:8]) == "MV - CPC" {
		// Standard DSK format
		fmt.Printf("Loaded standard DSK: %s\n", filename)
		// TODO: Parse standard DSK header
		fdc.diskPresent = true
		fdc.diskFilename = filename
		fdc.st3 |= 0x20 // Set Ready bit
		return nil
	}

	return fmt.Errorf("unsupported disk format (size: %d)", len(data))
}

// SaveDisk saves the disk image to file
func (fdc *FDC765) SaveDisk() error {
	if !fdc.diskModified || fdc.diskFilename == "" {
		return nil
	}

	err := os.WriteFile(fdc.diskFilename, fdc.diskImage, 0644)
	if err != nil {
		return err
	}

	fdc.diskModified = false
	fmt.Printf("Saved disk: %s\n", fdc.diskFilename)
	return nil
}

// ReadStatus reads the main status register
func (fdc *FDC765) ReadStatus() uint8 {
	// The +3 ROM detection sequence:
	// 1. Reads status register - expects RQM=1 (bit 7)
	// 2. If RQM=1, sends Sense Interrupt Status commands
	// 3. Expects to get 4 reset status responses (one per drive)

	status := fdc.mainStatus

	if fdc.debug {
		fmt.Printf("FDC: ReadStatus() = 0x%02X (phase=%d, resultLen=%d)\n",
			status, fdc.commandPhase, len(fdc.resultBuf))
	}

	return status
}

// ReadData reads from the data register
func (fdc *FDC765) ReadData() uint8 {
	var data uint8 = 0xFF

	if fdc.commandPhase == 2 && len(fdc.resultBuf) > 0 {
		// Reading result phase
		if fdc.resultIdx < len(fdc.resultBuf) {
			data = fdc.resultBuf[fdc.resultIdx]
			fdc.resultIdx++

			if fdc.resultIdx >= len(fdc.resultBuf) {
				// Result phase complete
				fdc.commandPhase = 0
				fdc.mainStatus = 0x80 // RQM=1, DIO=0, ready for next command
				fdc.resultBuf = nil
				fdc.resultIdx = 0

				if fdc.debug {
					fmt.Printf("FDC: Result phase complete, ready for next command\n")
				}
			} else {
				// More result bytes to send
				fdc.mainStatus = 0xD0 // RQM=1, DIO=1, CB=1
			}

			if fdc.debug {
				fmt.Printf("FDC: ReadData() = 0x%02X (result byte %d)\n", data, fdc.resultIdx-1)
			}
			return data
		}
	} else if fdc.commandPhase == 1 && fdc.dataLen > 0 {
		// Reading data during execution phase
		if fdc.dataIdx < fdc.dataLen {
			data = fdc.dataBuffer[fdc.dataIdx]
			fdc.dataIdx++
			if fdc.dataIdx >= fdc.dataLen {
				// Data transfer complete
				fdc.completeCommand()
			}
			return data
		}
	}

	if fdc.debug && data == 0xFF {
		fmt.Printf("FDC: ReadData() = 0xFF (no data available)\n")
	}

	return data
}

// WriteData writes to the data register
func (fdc *FDC765) WriteData(value uint8) {
	if fdc.debug {
		fmt.Printf("FDC: WriteData(0x%02X) phase=%d\n", value, fdc.commandPhase)
	}

	if fdc.commandPhase == 0 {
		// Command phase - collecting command bytes
		fdc.commandBuf = append(fdc.commandBuf, value)
		fdc.mainStatus = 0x10 // CB=1, RQM=0, busy processing command

		// Check if we have a complete command
		if fdc.isCommandComplete() {
			fdc.executeCommand()
		} else {
			// Ready for next command byte
			fdc.mainStatus = 0x90 // RQM=1, CB=1
		}
	} else if fdc.commandPhase == 1 && fdc.dataLen > 0 {
		// Writing data during execution phase
		if fdc.dataIdx < fdc.dataLen {
			fdc.dataBuffer[fdc.dataIdx] = value
			fdc.dataIdx++
			if fdc.dataIdx >= fdc.dataLen {
				// Data transfer complete
				fdc.writeDataToImage()
				fdc.completeCommand()
			}
		}
	}
}

// isCommandComplete checks if we have all bytes for current command
func (fdc *FDC765) isCommandComplete() bool {
	if len(fdc.commandBuf) == 0 {
		return false
	}

	cmd := fdc.commandBuf[0] & 0x1F

	// Command lengths
	switch cmd {
	case 0x03: // Specify
		return len(fdc.commandBuf) >= 3
	case 0x04: // Sense Drive Status
		return len(fdc.commandBuf) >= 2
	case 0x05: // Write Data
		return len(fdc.commandBuf) >= 9
	case 0x06: // Read Data
		return len(fdc.commandBuf) >= 9
	case 0x07: // Recalibrate
		return len(fdc.commandBuf) >= 2
	case 0x08: // Sense Interrupt Status
		return len(fdc.commandBuf) >= 1
	case 0x0A: // Read ID
		return len(fdc.commandBuf) >= 2
	case 0x0D: // Format Track
		return len(fdc.commandBuf) >= 6
	case 0x0F: // Seek
		return len(fdc.commandBuf) >= 3
	case 0x11: // Scan Equal
		return len(fdc.commandBuf) >= 9
	case 0x19: // Scan Low or Equal
		return len(fdc.commandBuf) >= 9
	case 0x1D: // Scan High or Equal
		return len(fdc.commandBuf) >= 9
	default:
		// Unknown command - assume single byte
		return true
	}
}

// executeCommand executes the current command
func (fdc *FDC765) executeCommand() {
	if len(fdc.commandBuf) == 0 {
		return
	}

	cmd := fdc.commandBuf[0] & 0x1F

	if fdc.debug {
		fmt.Printf("FDC: Executing command 0x%02X\n", cmd)
	}

	switch cmd {
	case 0x03: // Specify - set drive parameters
		// Parameters: SRT/HUT, HLT/ND
		// Just accept them
		if fdc.debug {
			fmt.Printf("FDC: Specify command\n")
		}
		fdc.commandPhase = 0
		fdc.mainStatus = 0x80 // Ready for next command
		fdc.commandBuf = nil

	case 0x04: // Sense Drive Status
		// Returns ST3
		drive := uint8(0)
		if len(fdc.commandBuf) > 1 {
			drive = fdc.commandBuf[1] & 0x03
		}

		fdc.st3 = (fdc.st3 & 0xF8) | drive // Update drive in ST3, keep other bits

		// Report drive status based on disk presence
		if fdc.diskPresent {
			fdc.st3 |= 0x20  // Ready
			fdc.st3 &^= 0x40 // Not write protected (bit 6 = 0)
		} else {
			fdc.st3 &^= 0x20 // Not ready
		}

		fdc.st3 |= 0x08 // Two-sided (always for +3)

		if fdc.currentTrack == 0 {
			fdc.st3 |= 0x10 // Track 0
		} else {
			fdc.st3 &^= 0x10
		}

		if fdc.debug {
			fmt.Printf("FDC: Sense Drive Status - drive=%d, disk=%v, ST3=0x%02X\n",
				drive, fdc.diskPresent, fdc.st3)
		}

		fdc.resultBuf = []uint8{fdc.st3}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0 // RQM=1, DIO=1, CB=1, ready to send result
		fdc.commandBuf = nil

	case 0x06: // Read Data
		fdc.readSector()

	case 0x05: // Write Data
		fdc.prepareSectorWrite()

	case 0x07: // Recalibrate - seek to track 0
		drv := uint8(0)
		if len(fdc.commandBuf) > 1 {
			drv = fdc.commandBuf[1] & 0x03
		}
		fdc.currentTrack = 0
		fdc.currentCylinder = 0
		// Set interrupt status for next Sense Interrupt Status
		fdc.st0 = 0x20 | drv // Seek End, normal termination
		fdc.seekEnd = true

		if fdc.debug {
			fmt.Printf("FDC: Recalibrate drive %d\n", drv)
		}

		fdc.commandPhase = 0
		fdc.mainStatus = 0x80
		fdc.commandBuf = nil

	case 0x08: // Sense Interrupt Status
		// This is THE critical command for +3 detection!
		// The +3 ROM expects exactly 4 reset status responses after power-on

		if fdc.resetCount > 0 {
			// Report reset condition for drives 0-3
			drive := uint8(4 - fdc.resetCount)
			fdc.resultBuf = []uint8{
				0xC0 | drive, // ST0: Reset condition (IC=11) + drive number
				0x00,         // PCN: Present Cylinder Number = 0 after reset
			}
			fdc.resetCount--

			if fdc.debug {
				fmt.Printf("FDC: Sense Interrupt Status - Reset for drive %d (ST0=0x%02X, PCN=0x00)\n",
					drive, fdc.resultBuf[0])
			}

		} else if fdc.seekEnd {
			// Report seek complete
			fdc.resultBuf = []uint8{fdc.st0, fdc.currentCylinder}
			fdc.seekEnd = false

			if fdc.debug {
				fmt.Printf("FDC: Sense Interrupt Status - Seek complete (ST0=0x%02X, PCN=0x%02X)\n",
					fdc.st0, fdc.currentCylinder)
			}

		} else {
			// No interrupt pending - return invalid command
			// The +3 ROM uses this to detect end of reset interrupts
			fdc.resultBuf = []uint8{0x80, 0x00} // Invalid command status

			if fdc.debug {
				fmt.Printf("FDC: Sense Interrupt Status - No interrupt (ST0=0x80)\n")
			}
		}

		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0 // Ready to send result
		fdc.commandBuf = nil

	case 0x0A: // Read ID - read sector header
		// This command reads the next sector header
		// For simplicity, return the current position
		fdc.st0 = 0x00 // Normal termination
		fdc.st1 = 0x00
		fdc.st2 = 0x00

		fdc.resultBuf = []uint8{
			fdc.st0,               // ST0
			fdc.st1,               // ST1
			fdc.st2,               // ST2
			fdc.currentCylinder,   // C
			fdc.currentHead,       // H
			fdc.currentSector + 1, // R (sector numbers start at 1)
			0x02,                  // N (512 bytes = 2^(7+2))
		}

		if fdc.debug {
			fmt.Printf("FDC: Read ID - C=%d H=%d R=%d N=%d\n",
				fdc.currentCylinder, fdc.currentHead, fdc.currentSector+1, 2)
		}

		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil

	case 0x0F: // Seek
		if len(fdc.commandBuf) >= 3 {
			drive := fdc.commandBuf[1] & 0x03
			fdc.currentCylinder = fdc.commandBuf[2]
			fdc.currentTrack = fdc.commandBuf[2]
			// Set interrupt status for next Sense Interrupt Status
			fdc.st0 = 0x20 | drive // Seek End, normal termination
			fdc.seekEnd = true

			if fdc.debug {
				fmt.Printf("FDC: Seek drive %d to cylinder %d\n", drive, fdc.currentCylinder)
			}
		}
		fdc.commandPhase = 0
		fdc.mainStatus = 0x80
		fdc.commandBuf = nil

	default:
		// Unknown/unimplemented command
		if fdc.debug {
			fmt.Printf("FDC: Unknown command 0x%02X\n", cmd)
		}
		// Set error in ST0
		fdc.st0 = 0x80 // Invalid command
		fdc.commandPhase = 0
		fdc.mainStatus = 0x80
		fdc.commandBuf = nil
	}
}

// readSector reads a sector from disk image
func (fdc *FDC765) readSector() {
	if len(fdc.commandBuf) < 9 {
		fdc.st0 = 0x40 // Abnormal termination
		fdc.st1 = 0x01 // Missing address mark
		fdc.st2 = 0x00
		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, fdc.st2, 0, 0, 0, 0}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil
		return
	}

	// Extract parameters
	head := (fdc.commandBuf[1] >> 2) & 0x01
	drv := fdc.commandBuf[1] & 0x03
	cylinder := fdc.commandBuf[2]
	sector := fdc.commandBuf[4] - 1 // Sectors numbered from 1

	// Update current position
	fdc.currentHead = head
	fdc.currentCylinder = cylinder
	fdc.currentSector = sector

	if !fdc.diskPresent {
		// No disk - return "not ready" error
		// This tells +3DOS that there's no disk
		fdc.st0 = 0x48 | drv // Abnormal termination, not ready
		fdc.st1 = 0x05       // Missing address mark + not writable
		fdc.st2 = 0x01       // Missing data address mark

		// Return error result immediately
		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, fdc.st2, cylinder, head, sector + 1, 0x02}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil

		if fdc.debug {
			fmt.Printf("FDC: Read sector - no disk present, returning error\n")
		}
		return
	}

	// Calculate offset in disk image
	track := cylinder
	if head == 1 {
		track += TracksPerSide
	}

	offset := (int(track)*SectorsPerTrack + int(sector)) * BytesPerSector

	if offset+BytesPerSector <= len(fdc.diskImage) {
		// Read sector data
		fdc.dataBuffer = make([]byte, BytesPerSector)
		copy(fdc.dataBuffer, fdc.diskImage[offset:offset+BytesPerSector])
		fdc.dataIdx = 0
		fdc.dataLen = BytesPerSector
		fdc.commandPhase = 1
		fdc.mainStatus = 0xF0 // RQM=1, DIO=1, EXM=1, CB=1 - ready to send data
		// Don't clear command buffer yet - needed for result phase

		if fdc.debug {
			fmt.Printf("FDC: Reading sector C=%d H=%d R=%d\n", cylinder, head, sector+1)
		}
	} else {
		// Sector not found
		fdc.st0 = 0x40 | drv // Abnormal termination
		fdc.st1 = 0x04       // No data
		fdc.st2 = 0x00
		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, fdc.st2, cylinder, head, sector + 1, 0x02}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil
	}
}

// prepareSectorWrite prepares to write a sector
func (fdc *FDC765) prepareSectorWrite() {
	if len(fdc.commandBuf) < 9 {
		fdc.st0 = 0x40 // Abnormal termination
		fdc.st1 = 0x01 // Missing address mark
		fdc.st2 = 0x00
		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, fdc.st2, 0, 0, 0, 0}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil
		return
	}

	// Extract and store parameters for later
	fdc.currentHead = (fdc.commandBuf[1] >> 2) & 0x01
	drv := fdc.commandBuf[1] & 0x03
	fdc.currentCylinder = fdc.commandBuf[2]
	fdc.currentSector = fdc.commandBuf[4] - 1

	if !fdc.diskPresent {
		// No disk - return "not ready" error immediately
		// Don't accept data if there's no disk
		fdc.st0 = 0x48 | drv // Abnormal termination, not ready
		fdc.st1 = 0x05       // Missing address mark + not writable
		fdc.st2 = 0x01       // Missing data address mark

		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, fdc.st2,
			fdc.currentCylinder, fdc.currentHead, fdc.currentSector + 1, 0x02}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil

		if fdc.debug {
			fmt.Printf("FDC: Write sector - no disk present, returning error\n")
		}
		return
	}

	// Prepare to receive data
	fdc.dataBuffer = make([]byte, BytesPerSector)
	fdc.dataIdx = 0
	fdc.dataLen = BytesPerSector
	fdc.commandPhase = 1
	fdc.mainStatus = 0xB0 // RQM=1, DIO=0, EXM=1, CB=1 - ready to receive data

	if fdc.debug {
		fmt.Printf("FDC: Preparing to write sector C=%d H=%d R=%d\n",
			fdc.currentCylinder, fdc.currentHead, fdc.currentSector+1)
	}
}

// writeDataToImage writes sector data to disk image
func (fdc *FDC765) writeDataToImage() {
	if !fdc.diskPresent {
		// No disk - just discard the data
		return
	}

	track := fdc.currentCylinder
	if fdc.currentHead == 1 {
		track += TracksPerSide
	}

	offset := (int(track)*SectorsPerTrack + int(fdc.currentSector)) * BytesPerSector

	if offset+BytesPerSector <= len(fdc.diskImage) {
		copy(fdc.diskImage[offset:], fdc.dataBuffer)
		fdc.diskModified = true
	}
}

// completeCommand completes the current command
func (fdc *FDC765) completeCommand() {
	// Set up result phase based on the command that just completed
	drive := uint8(0)
	if len(fdc.commandBuf) > 1 {
		drive = fdc.commandBuf[1] & 0x03
	}

	// Success status
	fdc.st0 = 0x00 | drive // Normal termination
	fdc.st1 = 0x00
	fdc.st2 = 0x00

	fdc.resultBuf = []uint8{
		fdc.st0,
		fdc.st1,
		fdc.st2,
		fdc.currentCylinder,
		fdc.currentHead,
		fdc.currentSector + 1,
		0x02, // 512 bytes per sector
	}
	fdc.commandPhase = 2
	fdc.resultIdx = 0
	fdc.mainStatus = 0xD0 // Ready to send result
	fdc.commandBuf = nil
}
