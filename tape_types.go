package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// ============================================================================
// Tape Types and Constants
// ============================================================================

// TapeMode represents the tape loading mode
type TapeMode int

const (
	TapeAccurate TapeMode = iota // Accurate pulse-level emulation
	TapeFast                      // Fast/instant loading
)

// TapeBlock represents a single block of tape data
type TapeBlock struct {
	Data     []byte // Raw block data (including flag byte)
	IsHeader bool   // True if this is a header block
}

// TapeState holds the current tape state
type TapeState struct {
	Loaded     bool        // Is a tape loaded?
	Filename   string      // Current tape filename
	Mode       TapeMode    // Current loading mode
	Playing    bool        // Is tape playing?
	Position   int         // Current position (block index or pulse index)
	EdgeOffset int         // Sub-pulse position for accurate mode
	EarLevel   bool        // Current EAR output level
	Blocks     []TapeBlock // Parsed tape blocks
	Pulses     []int       // Pulse durations in T-states (for accurate mode)
}

// ============================================================================
// Main Tape Structure
// ============================================================================

// Tape manages tape loading and playback
type Tape struct {
	zx *ZenZX       // Reference to main emulator
	st *TapeState   // Current tape state
	fl *FastLoader  // Fast loader implementation (optional)
}

// NewTape creates a new tape manager
func NewTape(zx *ZenZX) *Tape {
	return &Tape{
		zx: zx,
		st: &TapeState{
			Mode: TapeFast, // Default to fast mode
		},
	}
}

// ============================================================================
// Basic Tape Operations
// ============================================================================

// LoadFile loads a tape file (TAP or TZX format)
func (t *Tape) LoadFile(filename string) error {
	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Detect format and load
	if strings.HasSuffix(strings.ToLower(filename), ".tzx") {
		err = t.loadTZX(data)
	} else {
		err = t.loadTAP(data)
	}

	if err != nil {
		return err
	}

	t.st.Loaded = true
	t.st.Filename = filename
	t.st.Position = 0
	t.st.Playing = false

	return nil
}

// Play starts tape playback
func (t *Tape) Play() {
	if t.st != nil && t.st.Loaded {
		t.st.Playing = true
	}
}

// Stop stops tape playback
func (t *Tape) Stop() {
	if t.st != nil {
		t.st.Playing = false
	}
}

// Rewind resets tape to beginning
func (t *Tape) Rewind() {
	if t.st != nil {
		t.st.Position = 0
		t.st.EdgeOffset = 0
		t.st.EarLevel = false
	}
}

// SetMode sets the tape loading mode
func (t *Tape) SetMode(mode TapeMode) {
	if t.st != nil {
		t.st.Mode = mode
	}
}

// AttachFastLoader attaches a fast loader implementation
func (t *Tape) AttachFastLoader(fl *FastLoader) {
	t.fl = fl
}

// ============================================================================
// TAP Format Loading
// ============================================================================

// loadTAP loads a TAP format tape file
func (t *Tape) loadTAP(data []byte) error {
	t.st.Blocks = nil
	t.st.Pulses = nil

	pos := 0
	for pos < len(data) {
		// Check for enough data for length header
		if pos+2 > len(data) {
			break
		}

		// Read block length (little-endian)
		blockLen := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
		pos += 2

		// Check for enough data for block
		if pos+blockLen > len(data) {
			break
		}

		// Extract block data
		blockData := data[pos : pos+blockLen]
		pos += blockLen

		// Determine if header block (flag byte 0x00 and length 19)
		isHeader := len(blockData) == 19 && blockData[0] == 0x00

		// Add block
		t.st.Blocks = append(t.st.Blocks, TapeBlock{
			Data:     blockData,
			IsHeader: isHeader,
		})

		// Generate pulses for accurate mode
		pulses := t.genPulses(blockData)
		t.st.Pulses = append(t.st.Pulses, pulses...)
	}

	return nil
}

// ============================================================================
// TZX Format Loading (Simplified)
// ============================================================================

// loadTZX loads a TZX format tape file
func (t *Tape) loadTZX(data []byte) error {
	// Check TZX header
	if len(data) < 10 || string(data[0:7]) != "ZXTape!" {
		return fmt.Errorf("invalid TZX file")
	}

	t.st.Blocks = nil
	t.st.Pulses = nil

	pos := 10 // Skip header
	for pos < len(data) {
		if pos >= len(data) {
			break
		}

		blockID := data[pos]
		pos++

		switch blockID {
		case 0x10: // Standard speed data block
			if pos+4 > len(data) {
				break
			}
			pause := binary.LittleEndian.Uint16(data[pos : pos+2])
			length := binary.LittleEndian.Uint16(data[pos+2 : pos+4])
			pos += 4

			if pos+int(length) > len(data) {
				break
			}

			blockData := data[pos : pos+int(length)]
			pos += int(length)

			isHeader := len(blockData) == 19 && blockData[0] == 0x00
			t.st.Blocks = append(t.st.Blocks, TapeBlock{
				Data:     blockData,
				IsHeader: isHeader,
			})

			// Generate pulses
			pulses := t.genPulses(blockData)
			if pause > 0 {
				// Add pause in T-states (pause is in milliseconds)
				pulses = append(pulses, int(pause)*3500)
			}
			t.st.Pulses = append(t.st.Pulses, pulses...)

		case 0x20: // Pause block
			if pos+2 > len(data) {
				break
			}
			pause := binary.LittleEndian.Uint16(data[pos : pos+2])
			pos += 2
			if pause > 0 {
				t.st.Pulses = append(t.st.Pulses, int(pause)*3500)
			}

		default:
			// Skip unknown blocks - would need full TZX implementation
			// For now, just try to continue
			if blockID >= 0x10 && blockID <= 0x5A {
				// Has a length field we can use to skip
				if pos+4 <= len(data) {
					length := binary.LittleEndian.Uint32(data[pos : pos+4])
					pos += 4 + int(length)
				} else {
					break
				}
			} else {
				// Unknown block, stop parsing
				break
			}
		}
	}

	return nil
}

// ============================================================================
// Pulse Generation
// ============================================================================

// genPulses generates pulse timings for a data block
func (t *Tape) genPulses(data []byte) []int {
	var pulses []int

	if len(data) == 0 {
		return pulses
	}

	// Determine pilot length based on flag byte
	pilotLen := 3223 // Data block
	if data[0] == 0x00 {
		pilotLen = 8063 // Header block
	}

	// Pilot tone (2168 T-states per pulse)
	for i := 0; i < pilotLen; i++ {
		pulses = append(pulses, 2168)
	}

	// Sync pulses
	pulses = append(pulses, 667)  // First sync
	pulses = append(pulses, 735)  // Second sync

	// Data pulses
	for _, b := range data {
		for bit := 7; bit >= 0; bit-- {
			if (b>>bit)&1 == 1 {
				// One bit: two pulses of 1710 T-states
				pulses = append(pulses, 1710, 1710)
			} else {
				// Zero bit: two pulses of 855 T-states
				pulses = append(pulses, 855, 855)
			}
		}
	}

	return pulses
}