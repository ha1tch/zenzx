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

// safeHaltAddr is where LoadBINWithSafeReturn writes its infinite-loop
// return stub: high enough (0xFFF0) to sit well clear of any address a
// test binary would plausibly use for its own code or data, and clear of
// the default 48K stack-top area a just-booted machine starts with.
const safeHaltAddr = 0xFFF0

// LoadBINWithSafeReturn is LoadBIN, but for startAddr >= 0 it also gives
// the loaded code a safe place to return TO: a two-byte "JR $" (opcode
// 0x18, 0xFE -- jump to self, an infinite loop) is written at safeHaltAddr,
// and its address is pushed onto the stack before PC is set to startAddr.
//
// This exists because a raw -bin load has no calling convention behind it
// the way Sinclair BASIC's "RANDOMIZE USR n" does on real hardware (which
// itself pushes a return address before jumping) -- plain LoadBIN leaves
// the stack exactly as the machine's own boot sequence left it. A test
// binary ending in a bare RET, expecting to be CALLed properly, instead
// pops whatever the stack already contained and jumps there: on real
// hardware or in CSpect that lands back in BASIC or NextZXOS safely, but a
// bare LoadBIN start has no such target, and execution runs away into
// unrelated memory, corrupting anything it happens to touch as it goes --
// confirmed directly: two dump-mem reads of the same "finished" test
// result, taken at different frame counts after the same run, returned
// different values each time, worse the longer the wait, which is exactly
// the signature of an ongoing runaway write rather than a stable result.
// Routing the same RET into a harmless self-loop instead makes the result
// stable forever once written, so any sufficiently long wait after loading
// gives the same, correct answer.
//
// Safe to call with SP already low (near 0x0000) in unusual configurations
// -- it just decrements SP by 2 and writes through the normal memory path,
// the same as two PUSH byte-writes would.
func (zx *ZenZX) LoadBINWithSafeReturn(filename string, loadAddr uint16, startAddr int) error {
	if err := zx.LoadBIN(filename, loadAddr, -1); err != nil {
		return err
	}
	if startAddr < 0 {
		return nil
	}
	if startAddr > 0xFFFF {
		return fmt.Errorf("start address 0x%X is outside the 64K address space", startAddr)
	}

	zx.memory.Write(safeHaltAddr, 0x18)   // JR
	zx.memory.Write(safeHaltAddr+1, 0xFE) // -2 (relative to itself): jump to self

	zx.cpu.SP -= 2
	zx.memory.Write(zx.cpu.SP, uint8(safeHaltAddr&0xFF))        // low byte
	zx.memory.Write(zx.cpu.SP+1, uint8((safeHaltAddr>>8)&0xFF)) // high byte

	zx.cpu.PC = uint16(startAddr)
	return nil
}
