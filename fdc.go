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
	disk         *Disk // parsed DSK structure (CHRN-addressable sectors)
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
	readIdIndex int // rotating index for Read ID across a track's sectors

	// Auto-commit debounce: lastWriteFrame is the frame at which the disk was
	// last modified; AutoCommit flushes once the disk has been quiet long
	// enough and the controller is idle.
	currentFrame    int // updated each frame by the front-end via Tick
	lastWriteFrame  int // frame of the most recent disk modification
	currentR    uint8 // sector ID (R) currently being transferred in a read
	eotSector   uint8 // EOT: last sector ID to transfer in a multi-sector read

	// Format Track state.
	fmtSizeCode  uint8 // N: sector size code for the track being formatted
	fmtNumSec    uint8 // SC: sectors per track
	fmtFiller    uint8 // D: filler byte for sector data
	fmtCollected int   // how many of the 4*SC id bytes have been received

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

// Tick advances the FDC's frame clock. The front-end calls it once per frame.
func (fdc *FDC765) Tick(frame int) {
	fdc.currentFrame = frame
}

// markModified flags the disk dirty and records the time of the write, so the
// auto-commit debounce window is measured from the most recent write.
func (fdc *FDC765) markModified() {
	fdc.diskModified = true
	fdc.lastWriteFrame = fdc.currentFrame
}

// AutoCommit flushes a modified disk back to its file, but only once writing
// has settled: it commits when the disk is dirty, the controller is idle (no
// command in execution or result phase), and at least debounceFrames have
// passed since the last write. This avoids serializing a torn image in the
// middle of a multi-sector operation. No-op when there is nothing to commit.
// Returns true if a commit occurred.
func (fdc *FDC765) AutoCommit(debounceFrames int) bool {
	if !fdc.diskModified || fdc.diskFilename == "" {
		return false
	}
	if fdc.commandPhase != 0 {
		return false // never commit mid-command
	}
	if fdc.currentFrame-fdc.lastWriteFrame < debounceFrames {
		return false // not quiet long enough
	}
	if err := fdc.SaveDisk(); err != nil {
		return false // retry on a later frame
	}
	return true
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
		// Standard 180KB raw sector dump (no DSK header).
		copy(fdc.diskImage, data)
		fdc.disk = nil // raw image: addressed by flat geometry
		fdc.diskPresent = true
		fdc.diskFilename = filename
		fdc.diskModified = false
		fdc.st3 |= 0x20 // Set Ready bit
		fmt.Printf("Loaded disk: %s (180KB raw)\n", filename)
		return nil
	} else if len(data) > 256 && (string(data[0:8]) == "EXTENDED" || string(data[0:8]) == "MV - CPC") {
		// Standard or extended DSK image: parse into a CHRN-addressable
		// track/sector structure.
		disk, err := ParseDSK(data)
		if err != nil {
			return fmt.Errorf("parsing DSK %s: %v", filename, err)
		}
		fdc.disk = disk
		fdc.diskPresent = true
		fdc.diskFilename = filename
		fdc.diskModified = false
		fdc.st3 |= 0x20 // Set Ready bit
		fmt.Printf("Loaded DSK: %s (%d tracks, %d side(s)%s)\n",
			filename, disk.NumTracks, disk.NumSides,
			map[bool]string{true: ", extended", false: ""}[disk.Extended])
		return nil
	}

	return fmt.Errorf("unsupported disk format (size: %d)", len(data))
}

// SaveDisk saves the disk image to file
func (fdc *FDC765) SaveDisk() error {
	if !fdc.diskModified || fdc.diskFilename == "" {
		return nil
	}

	if fdc.disk != nil {
		// Parsed DSK: re-serialize the (possibly modified) structure.
		if err := fdc.disk.SaveDSK(fdc.diskFilename); err != nil {
			return err
		}
		fdc.disk.Modified = false
		fdc.diskModified = false
		fmt.Printf("Saved DSK: %s\n", fdc.diskFilename)
		return nil
	}

	// Raw flat image.
	if err := os.WriteFile(fdc.diskFilename, fdc.diskImage, 0644); err != nil {
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
				// Sector drained. In a multi-sector read, advance to the next
				// sector ID (up to and including EOT) and keep transferring;
				// otherwise complete the command.
				if fdc.command() == 0x06 && fdc.currentR < fdc.eotSector {
					fdc.currentR++
					fdc.currentSector = fdc.currentR - 1
					fdc.loadSectorForRead(fdc.currentCylinder, fdc.currentHead, fdc.commandBuf[1]&0x03)
					// If the next sector wasn't found, loadSectorForRead has
					// already set an error result (phase 2); nothing more here.
				} else {
					fdc.completeCommand()
				}
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
				if fdc.command() == 0x0D {
					fdc.buildFormattedTrack()
				} else {
					fdc.writeDataToImage()
				}
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

	case 0x0D: // Format Track
		fdc.prepareFormatTrack()

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
		// Read ID returns the next sector header under the head. For a parsed
		// DSK, rotate through the current track's real sector list so +3DOS
		// sees the actual sector IDs; otherwise fall back to a synthetic CHRN.
		fdc.st0 = 0x00
		fdc.st1 = 0x00
		fdc.st2 = 0x00

		var c, h, r, n uint8 = fdc.currentCylinder, fdc.currentHead, fdc.currentSector + 1, 0x02
		if fdc.disk != nil {
			if trk := fdc.disk.Track(fdc.currentCylinder, fdc.currentHead); trk != nil && len(trk.Sectors) > 0 {
				if fdc.readIdIndex >= len(trk.Sectors) {
					fdc.readIdIndex = 0
				}
				s := trk.Sectors[fdc.readIdIndex]
				c, h, r, n = s.C, s.H, s.R, s.N
				fdc.readIdIndex++
			} else {
				// No formatted track here: missing address mark.
				fdc.st0 = 0x40
				fdc.st1 = 0x01
			}
		}

		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, fdc.st2, c, h, r, n}

		if fdc.debug {
			fmt.Printf("FDC: Read ID - C=%d H=%d R=%d N=%d\n", c, h, r, n)
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

// command returns the current command opcode (low 5 bits), or 0xFF if no
// command is buffered.
func (fdc *FDC765) command() uint8 {
	if len(fdc.commandBuf) == 0 {
		return 0xFF
	}
	return fdc.commandBuf[0] & 0x1F
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

	// Multi-sector read state: start at R (commandBuf[4]) and continue up to
	// and including EOT (commandBuf[6]). The data-completion path advances R
	// after each sector until R passes EOT.
	fdc.currentR = fdc.commandBuf[4]
	fdc.eotSector = fdc.commandBuf[6]

	if !fdc.diskPresent {
		// No disk - return "not ready" error
		// This tells +3DOS that there's no disk
		fdc.st0 = 0x48 | drv // Abnormal termination, not ready
		fdc.st1 = 0x05       // Missing address mark + not writable
		fdc.st2 = 0x01       // Missing data address mark

		// Return error result immediately
		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, fdc.st2, cylinder, head, fdc.currentR, 0x02}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil

		if fdc.debug {
			fmt.Printf("FDC: Read sector - no disk present, returning error\n")
		}
		return
	}

	fdc.loadSectorForRead(cylinder, head, drv)
}

// loadSectorForRead locates the sector with the current record ID and either
// arms the data-transfer phase or sets a sector-not-found error result. It is
// called for the first sector and again for each subsequent sector in a
// multi-sector read.
func (fdc *FDC765) loadSectorForRead(cylinder, head, drv uint8) {
	requestedR := fdc.currentR
	var sectorData []byte
	if fdc.disk != nil {
		sec := fdc.disk.FindSector(cylinder, head, requestedR)
		if sec != nil {
			sectorData = sec.Data
		}
	} else {
		sector := requestedR - 1
		track := cylinder
		if head == 1 {
			track += TracksPerSide
		}
		offset := (int(track)*SectorsPerTrack + int(sector)) * BytesPerSector
		if offset+BytesPerSector <= len(fdc.diskImage) {
			sectorData = fdc.diskImage[offset : offset+BytesPerSector]
		}
	}

	if sectorData != nil {
		// Read sector data
		fdc.dataBuffer = make([]byte, len(sectorData))
		copy(fdc.dataBuffer, sectorData)
		fdc.dataIdx = 0
		fdc.dataLen = len(sectorData)
		fdc.commandPhase = 1
		fdc.mainStatus = 0xF0 // RQM=1, DIO=1, EXM=1, CB=1 - ready to send data

		if fdc.debug {
			fmt.Printf("FDC: Reading sector C=%d H=%d R=%d (%d bytes)\n", cylinder, head, requestedR, len(sectorData))
		}
	} else {
		// Sector not found
		fdc.st0 = 0x40 | drv // Abnormal termination
		fdc.st1 = 0x04       // No data
		fdc.st2 = 0x00
		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, fdc.st2, cylinder, head, requestedR, 0x02}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil
		if fdc.debug {
			fmt.Printf("FDC: Sector not found C=%d H=%d R=%d\n", cylinder, head, requestedR)
		}
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
	fdc.currentR = fdc.commandBuf[4] // record ID for CHRN-based write targeting

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

// prepareFormatTrack begins a Format Track command. The command bytes are:
// 0x0D, (H<<2|drive), N (size code), SC (sectors/track), GPL, D (filler).
// The execution phase then receives 4 bytes per sector (C,H,R,N).
func (fdc *FDC765) prepareFormatTrack() {
	if len(fdc.commandBuf) < 6 {
		fdc.st0 = 0x40
		fdc.resultBuf = []uint8{fdc.st0, 0, 0, 0, 0, 0, 0}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil
		return
	}

	fdc.currentHead = (fdc.commandBuf[1] >> 2) & 0x01
	drv := fdc.commandBuf[1] & 0x03
	fdc.fmtSizeCode = fdc.commandBuf[2]
	fdc.fmtNumSec = fdc.commandBuf[3]
	fdc.fmtFiller = fdc.commandBuf[5]

	if !fdc.diskPresent || fdc.disk == nil {
		fdc.st0 = 0x48 | drv // not ready
		fdc.st1 = 0x02       // not writable
		fdc.resultBuf = []uint8{fdc.st0, fdc.st1, 0, 0, 0, 0, 0}
		fdc.commandPhase = 2
		fdc.resultIdx = 0
		fdc.mainStatus = 0xD0
		fdc.commandBuf = nil
		return
	}

	// Collect 4 ID bytes (C,H,R,N) per sector in the execution phase.
	fdc.dataBuffer = make([]byte, 4*int(fdc.fmtNumSec))
	fdc.dataIdx = 0
	fdc.dataLen = len(fdc.dataBuffer)
	fdc.commandPhase = 1
	fdc.mainStatus = 0xB0 // RQM=1, DIO=0, EXM=1, CB=1 - ready to receive

	if fdc.debug {
		fmt.Printf("FDC: Format track C=%d H=%d sectors=%d N=%d filler=0x%02X\n",
			fdc.currentCylinder, fdc.currentHead, fdc.fmtNumSec, fdc.fmtSizeCode, fdc.fmtFiller)
	}
}

// buildFormattedTrack constructs the track from the collected sector ID bytes,
// filling each sector's data with the filler byte, and stores it in the disk.
func (fdc *FDC765) buildFormattedTrack() {
	if fdc.disk == nil {
		return
	}
	idx := fdc.disk.trackIndex(fdc.currentCylinder, fdc.currentHead)
	if idx < 0 {
		return
	}

	secLen := 0x80 << fdc.fmtSizeCode
	trk := DiskTrack{
		Track:    fdc.currentCylinder,
		Side:     fdc.currentHead,
		SizeCode: fdc.fmtSizeCode,
		Filler:   fdc.fmtFiller,
	}
	for s := 0; s < int(fdc.fmtNumSec); s++ {
		b := fdc.dataBuffer[4*s : 4*s+4]
		data := make([]byte, secLen)
		for i := range data {
			data[i] = fdc.fmtFiller
		}
		trk.Sectors = append(trk.Sectors, DiskSector{
			C: b[0], H: b[1], R: b[2], N: b[3], Data: data,
		})
	}
	fdc.disk.Tracks[idx] = trk
	fdc.disk.Modified = true
	fdc.markModified()

	if fdc.debug {
		fmt.Printf("FDC: Formatted track %d/%d with %d sectors of %d bytes\n",
			fdc.currentCylinder, fdc.currentHead, len(trk.Sectors), secLen)
	}
}

// writeDataToImage writes sector data to disk image
func (fdc *FDC765) writeDataToImage() {
	if !fdc.diskPresent {
		// No disk - just discard the data
		return
	}

	if fdc.disk != nil {
		// Locate the target sector by CHRN (R was stored as currentR) and copy
		// the received data into it. Writing into the parsed structure means a
		// later SaveDisk re-serializes the change.
		sec := fdc.disk.FindSector(fdc.currentCylinder, fdc.currentHead, fdc.currentR)
		if sec != nil {
			n := len(fdc.dataBuffer)
			if n > len(sec.Data) {
				n = len(sec.Data)
			}
			copy(sec.Data, fdc.dataBuffer[:n])
			fdc.disk.Modified = true
			fdc.markModified()
			if fdc.debug {
				fmt.Printf("FDC: Wrote sector C=%d H=%d R=%d (%d bytes)\n",
					fdc.currentCylinder, fdc.currentHead, fdc.currentR, n)
			}
		} else if fdc.debug {
			fmt.Printf("FDC: Write target sector not found C=%d H=%d R=%d\n",
				fdc.currentCylinder, fdc.currentHead, fdc.currentR)
		}
		return
	}

	// Raw 180KB image: flat geometry.
	track := fdc.currentCylinder
	if fdc.currentHead == 1 {
		track += TracksPerSide
	}
	offset := (int(track)*SectorsPerTrack + int(fdc.currentSector)) * BytesPerSector
	if offset+BytesPerSector <= len(fdc.diskImage) {
		copy(fdc.diskImage[offset:], fdc.dataBuffer)
		fdc.markModified()
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
		fdc.currentR + 1,
		0x02, // 512 bytes per sector
	}
	fdc.commandPhase = 2
	fdc.resultIdx = 0
	fdc.mainStatus = 0xD0 // Ready to send result
	fdc.commandBuf = nil
}
