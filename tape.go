package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ============================================================================
// Complete Tick() Implementation
// ============================================================================

// Tick advances the tape by a number of CPU T-states and toggles EAR for accurate mode.
// In Fast mode, it tries the ROM fastloader (if any), then instant-injects CODE blocks.
func (t *Tape) Tick(cycles int) {
	if t == nil || t.st == nil || !t.st.Playing || !t.st.Loaded {
		return
	}

	if t.st.Mode == TapeAccurate {
		// Advance through pulse stream
		for cycles > 0 && t.st.Position < len(t.st.Pulses) {
			remain := t.st.Pulses[t.st.Position] - t.st.EdgeOffset
			if cycles < remain {
				t.st.EdgeOffset += cycles
				cycles = 0
				break
			}
			// End of this pulse
			cycles -= remain
			t.st.Position++
			t.st.EdgeOffset = 0
			// Toggle EAR level each pulse end
			t.st.EarLevel = !t.st.EarLevel

			// Update the I/O port EAR bit
			if t.zx != nil && t.zx.io != nil {
				t.zx.io.tapeEar = t.st.EarLevel
			}
		}

		if t.st.Position >= len(t.st.Pulses) {
			// Stop at end
			t.st.Playing = false
			fmt.Println("Tape: End of tape reached")
		}
		return
	}

	// Fast mode - try ROM trap or do instant injection
	if t.st.Mode == TapeFast {
		// Option 1: Try ROM fastloader interception (if available)
		if t.fl != nil && t.fl.Enabled {
			if t.fl.TryIntercept(t.zx, t) {
				// Successfully intercepted and loaded
				t.st.Playing = false
				return
			}
		}

		// Option 2: Check if we're in ROM LOAD routine (addresses for 48K ROM)
		// Common addresses: 0x0556 (LD-BYTES), 0x04C2 (SA-BYTES)
		pc := t.zx.cpu.PC
		if pc >= 0x0556 && pc <= 0x0605 {
			// We're in the ROM loader area, inject next block
			t.fastInjectNextBlock()
			return
		}

		// Option 3: If just started playing, inject all CODE blocks immediately
		if t.st.Position == 0 {
			t.fastInjectAll()
			t.st.Playing = false
		}
	}
}

// fastInjectNextBlock injects the next block at current position
func (t *Tape) fastInjectNextBlock() {
	if t.st.Position >= len(t.st.Blocks) {
		t.st.Playing = false
		return
	}

	blk := t.st.Blocks[t.st.Position]

	// If it's a header and there's a data block following, load both
	if blk.IsHeader && t.st.Position+1 < len(t.st.Blocks) {
		dataBlk := t.st.Blocks[t.st.Position+1]
		if instantLoadHeaderAndData(t.zx, blk.Data, dataBlk.Data) {
			fmt.Printf("Tape: Fast-loaded block %d (header+data)\n", t.st.Position)
			t.st.Position += 2
		} else {
			t.st.Position++ // Skip failed block
		}
	} else {
		// Try to load as standalone data block
		if len(blk.Data) > 0 {
			// For non-header blocks, try direct memory injection if it looks like CODE
			// This is a simplified approach - you may want more sophisticated detection
			t.st.Position++
		}
	}

	if t.st.Position >= len(t.st.Blocks) {
		t.st.Playing = false
		fmt.Println("Tape: Fast load complete")
	}
}

// fastInjectAll injects all CODE blocks immediately
func (t *Tape) fastInjectAll() {
	loaded := 0
	for i := 0; i < len(t.st.Blocks); i++ {
		blk := t.st.Blocks[i]
		if blk.IsHeader && i+1 < len(t.st.Blocks) {
			dataBlk := t.st.Blocks[i+1]
			if instantLoadHeaderAndData(t.zx, blk.Data, dataBlk.Data) {
				loaded++
				i++ // Skip data block since we processed it
			}
		}
	}
	if loaded > 0 {
		fmt.Printf("Tape: Fast-loaded %d blocks\n", loaded)
	}
	t.st.Position = len(t.st.Blocks) // Mark as complete
}

// ============================================================================
// Status and Control Methods
// ============================================================================

// GetStatus returns human-readable tape status
func (t *Tape) GetStatus() string {
	if t == nil || t.st == nil || !t.st.Loaded {
		return "No tape loaded"
	}

	mode := "Accurate"
	if t.st.Mode == TapeFast {
		mode = "Fast"
	}

	status := "Stopped"
	if t.st.Playing {
		status = "Playing"
	}

	progress := 0
	if t.st.Mode == TapeAccurate && len(t.st.Pulses) > 0 {
		progress = (t.st.Position * 100) / len(t.st.Pulses)
	} else if len(t.st.Blocks) > 0 {
		progress = (t.st.Position * 100) / len(t.st.Blocks)
	}

	return fmt.Sprintf("%s [%s, %s, %d%%]",
		getFilenameOnly(t.st.Filename), mode, status, progress)
}

// GetBlockInfo returns information about tape blocks
func (t *Tape) GetBlockInfo() []string {
	if t == nil || t.st == nil || !t.st.Loaded {
		return nil
	}

	var info []string
	for i, blk := range t.st.Blocks {
		marker := "  "
		if t.st.Playing && i == t.st.Position {
			marker = "> "
		}

		if blk.IsHeader && len(blk.Data) == 19 {
			// Decode header
			blockType := blk.Data[1]
			filename := string(blk.Data[2:12])
			length := binary.LittleEndian.Uint16(blk.Data[12:14])

			typeStr := "Unknown"
			switch blockType {
			case 0:
				typeStr = "Program"
			case 1:
				typeStr = "Num Array"
			case 2:
				typeStr = "Char Array"
			case 3:
				typeStr = "Code"
			}

			info = append(info, fmt.Sprintf("%s%d: Header '%s' %s %d bytes",
				marker, i, strings.TrimRight(filename, " "), typeStr, length))
		} else {
			info = append(info, fmt.Sprintf("%s%d: Data %d bytes",
				marker, i, len(blk.Data)))
		}
	}

	return info
}

// ============================================================================
// Fast Loader Hook System
// ============================================================================

type FastLoader struct {
	Enabled bool
}

// TryIntercept attempts to intercept ROM loading routine and inject data directly
func (fl *FastLoader) TryIntercept(zx *ZenZX, t *Tape) bool {
	if !fl.Enabled || zx == nil || t == nil {
		return false
	}

	// Check if we're at a ROM loading entry point
	pc := zx.cpu.PC

	// 48K ROM LOAD-BYTES routine entry points
	if pc == 0x0556 || pc == 0x053F {
		// The ROM expects:
		// IX = start address
		// DE = length
		// A = flag byte (0x00 for header, 0xFF for data)
		// Carry flag set means LOAD, clear means VERIFY

		// For now, just do a simple fast load of next block
		if t.st.Position < len(t.st.Blocks) {
			t.fastInjectNextBlock()

			// Set up return conditions as if load succeeded
			// Set carry flag (bit 0 of F register) to indicate success
			zx.cpu.F |= 0x01

			// Return from the ROM routine
			// Pop the return address and jump there
			low := zx.memory.Read(zx.cpu.SP)
			high := zx.memory.Read(zx.cpu.SP + 1)
			zx.cpu.SP += 2
			zx.cpu.PC = uint16(high)<<8 | uint16(low)

			return true
		}
	}

	return false
}

// ============================================================================
// Helper Functions
// ============================================================================

// instantLoadHeaderAndData emulates the ROM loader result for CODE blocks.
// Returns true if successfully loaded.
func instantLoadHeaderAndData(zx *ZenZX, header, data []byte) bool {
	if len(header) != 19 || header[0] != 0x00 {
		return false
	}

	typeByte := header[1]
	length := binary.LittleEndian.Uint16(header[12:14])
	start := binary.LittleEndian.Uint16(header[14:16])

	// Only handle CODE blocks (type 3) for now
	if typeByte != 3 {
		return false
	}

	// Data block usually starts with flag byte (0xFF for data)
	payload := data
	if len(payload) > 0 && (payload[0] == 0xFF || payload[0] == 0x00) {
		payload = payload[1:] // Skip flag byte
	}

	// Ensure we have enough data (minus checksum byte)
	if len(payload) < int(length) {
		return false
	}

	// Load data into memory
	for i := 0; i < int(length); i++ {
		zx.memory.Write(start+uint16(i), payload[i])
	}

	// Flash border to indicate load (like ROM does)
	if zx.io != nil {
		zx.io.borderColor = (zx.io.borderColor + 1) & 0x07
	}

	return true
}

// hasSuffixFold checks if filename has suffix (case-insensitive)
func hasSuffixFold(s, suffix string) bool {
	return strings.HasSuffix(strings.ToLower(s), strings.ToLower(suffix))
}

// getFilenameOnly returns just the filename from a path
func getFilenameOnly(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}
