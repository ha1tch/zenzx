package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ParseAddr parses an unsigned 16-bit address in hex (0x.. or $..) or decimal.
func ParseAddr(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	var v uint64
	var err error
	switch {
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0X"):
		v, err = strconv.ParseUint(s[2:], 16, 32)
	case strings.HasPrefix(s, "$"):
		v, err = strconv.ParseUint(s[1:], 16, 32)
	default:
		v, err = strconv.ParseUint(s, 10, 32)
	}
	if err != nil {
		return 0, err
	}
	if v > 0xFFFF {
		return 0, fmt.Errorf("address 0x%X exceeds 64K address space", v)
	}
	return uint16(v), nil
}

// ParseAddrSigned parses an address like ParseAddr, but accepts -1 to mean
// "leave PC unchanged". Returns -1 for that case, otherwise the address.
func ParseAddrSigned(s string) (int, error) {
	if strings.TrimSpace(s) == "-1" {
		return -1, nil
	}
	a, err := ParseAddr(s)
	if err != nil {
		return 0, err
	}
	return int(a), nil
}

// ============================================================================
// Raw binary (.bin) direct memory loading
//
// Loads a raw binary blob straight into the emulated address space, bypassing
// tape and disk. Bytes are written through memory.Write, so RAM banking and
// the screen mirror are honoured exactly as a running program would see them,
// and writes into the ROM region (< 0x4000 in the current paging) are ignored.
//
// This is primarily a development aid: assemble code to a known origin, drop
// it into memory at that origin, and optionally set PC to start executing it.
// ============================================================================

// LoadSCR loads a raw ZX Spectrum screen dump (.scr) onto the display.
//
// A .scr is exactly the display file: 6144 bytes of bitmap (0x4000-0x57FF)
// followed by 768 bytes of attributes (0x5800-0x5AFF), 6912 bytes total.
// Bytes are written through the normal memory path at 0x4000, so the screen
// mirror and the underlying RAM stay consistent (a running program would see
// the same bytes). Loading a .scr does not alter CPU state -- it paints the
// screen but does not start anything running.
//
// Files shorter than 6912 bytes are loaded as far as they go; anything beyond
// 6912 bytes is ignored.
func (zx *ZenZX) LoadSCR(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filename, err)
	}
	if len(data) == 0 {
		return fmt.Errorf("%s is empty", filename)
	}
	const scrSize = 6912
	if len(data) > scrSize {
		data = data[:scrSize]
	}
	zx.memory.Load(0x4000, data)
	return nil
}

// LoadBIN loads the file at filename into memory starting at loadAddr.
//
// If startAddr >= 0, the program counter is set to uint16(startAddr) so the
// blob begins executing on the next frame. Pass startAddr < 0 to load without
// altering PC (e.g. to stage data while existing code runs).
//
// Loading is bank-aware: bytes are written via memory.Write against the
// current paging configuration. Callers that need a specific bank paged in
// should configure paging before calling LoadBIN.
func (zx *ZenZX) LoadBIN(filename string, loadAddr uint16, startAddr int) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filename, err)
	}
	if len(data) == 0 {
		return fmt.Errorf("%s is empty", filename)
	}

	// Guard against wrapping past the top of the address space.
	end := int(loadAddr) + len(data)
	if end > 0x10000 {
		return fmt.Errorf("%s is %d bytes at 0x%04X: would exceed 64K address space by %d bytes",
			filename, len(data), loadAddr, end-0x10000)
	}

	zx.memory.Load(loadAddr, data)

	if startAddr >= 0 {
		if startAddr > 0xFFFF {
			return fmt.Errorf("start address 0x%X is outside the 64K address space", startAddr)
		}
		zx.cpu.PC = uint16(startAddr)
	}

	return nil
}
