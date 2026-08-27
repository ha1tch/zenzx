package main

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zentools/pkg/tap"
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

	if t.st.Mode == TapeAccurate || t.st.Mode == TapeTurbo {
		// Advance through pulse stream
		for t.st.Position < len(t.st.Pulses) {
			remain := t.st.Pulses[t.st.Position] - t.st.EdgeOffset
			// Zero-length pulses are pure level flips and consume no
			// time: they must collapse NOW, even with the cycle
			// budget exhausted. The previous `cycles > 0` loop guard
			// deferred a trailing zero-length pulse to the next Tick
			// whenever a pulse ended exactly on the budget boundary,
			// leaving its transient level observable for one
			// instruction -- measured doing exactly that to Batman's
			// Speedlock loader: one spurious HIGH sample read
			// mid-pause at a Tick seam, corrupting the load from that
			// bit onward.
			if remain > 0 && cycles <= 0 {
				break
			}
			if cycles < remain {
				t.st.EdgeOffset += cycles
				cycles = 0
				break
			}
			// End of this pulse
			cycles -= remain
			completedIndex := t.st.Position
			t.st.Position++
			t.st.EdgeOffset = 0
			// Toggle EAR level each pulse end
			t.st.EarLevel = !t.st.EarLevel

			// Update the I/O port EAR bit
			if t.zx != nil && t.zx.io != nil {
				t.zx.io.tapeEar = t.st.EarLevel
			}

			// TZX's own "stop the tape" signal (a 0x20 pause block with
			// pause==0, see loadTZX) marks the final pulse of its
			// settling pattern as a stop point: once that pulse has
			// fully played, real hardware halts and waits rather than
			// continuing, matching a genuine multi-part loader's
			// intended pause for user/program action. A nil StopPoints
			// map reads as false for any index, so this is a no-op for
			// tapes that never used a stop-tape block.
			if t.st.StopPoints[completedIndex] {
				t.st.Playing = false
				break
			}

			// 0x2A's own, separate signal: stop only on a 48K-class
			// machine, matching SpecIde's own Tape::advance() ("if
			// (is48K && stopIf48K.find(pointer) ...)"). is128K is
			// false for both genuine 48K and TS2068 -- neither has
			// 128K's extra paged memory, which is what made this
			// stop unnecessary there; TS2068 is not explicitly
			// covered by anything SpecIde-verified for this specific
			// signal, so its inclusion here is a reasoned choice
			// (same memory-map class), not a directly confirmed one.
			if t.st.StopPointsIf48K[completedIndex] && t.zx != nil && t.zx.memory != nil && !t.zx.memory.is128K {
				t.st.Playing = false
				break
			}
		}

		if t.st.Position >= len(t.st.Pulses) {
			// Stop at end
			t.st.Playing = false
			fmt.Println("Tape: End of tape reached")
		}
		return
	}

	// Fast mode: try the ROM-trap fast loader. This is a precise
	// short-circuit for the standard ROM loading routine, not a
	// universal accelerator -- if the trap doesn't apply this instant
	// (CPU hasn't reached LD-BYTES yet, wrong ROM context, or a
	// non-standard/custom loader that never calls it), fast mode simply
	// does nothing this tick and waits, rather than guessing. Custom
	// loaders need -tapemode accurate; there is no longer a heuristic
	// fallback here (see CHANGELOG 0.4.x for what was removed and why:
	// this used to inject blocks the instant playback started, or
	// whenever PC fell anywhere in a 175-byte range, regardless of
	// whether the CPU had actually asked to load anything).
	if t.st.Mode == TapeFast {
		if t.fl != nil && t.fl.Enabled {
			t.fl.TryIntercept(t.zx, t)
		}
	}
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

		if blk.IsHeader {
			// HeaderType/HeaderName/DataLength are populated by
			// zentools/pkg/tap's own parser at load time (tape_types.go),
			// not re-derived from raw byte offsets here as this used to.
			typeStr := "Unknown"
			switch blk.HeaderType {
			case tap.TypeProgram:
				typeStr = "Program"
			case tap.TypeNumArray:
				typeStr = "Num Array"
			case tap.TypeCharArray:
				typeStr = "Char Array"
			case tap.TypeCode:
				typeStr = "Code"
			}

			info = append(info, fmt.Sprintf("%s%d: Header '%s' %s %d bytes",
				marker, i, blk.HeaderName, typeStr, blk.DataLength))
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

// TS2068's own tape routines, in the Extension ROM (reached only via
// chunk-0 Home/Extension banking, ts2068.go) -- confirmed by direct
// disassembly of rom/ts2068-1.rom to be a byte-for-byte relocated copy
// of the standard 48K ROM's LD-BYTES/SA-BYTES, offset by a constant
// 0x045A (0x0556-0x00FC = 0x045A = 0x04C2-0x0068). Register conventions
// are therefore identical -- trapLoad is reused unchanged; only the
// trap addresses and the ROM-context check differ. ts2068RTapeTrapPC
// (0x0112 = 0x00FC+0x16, the same +0x16 offset from entry to
// pre-slow-call point that 0x0556->0x056C uses) is reached via
// R_TAPE's entry at 0x00FC; ts2068WTapeTrapPC is W_TAPE's own entry at
// 0x0068, matching saBytesTrapPC's reasoning (no housekeeping-before-
// the-slow-part split exists on the save side).
const (
	ts2068RTapeTrapPC = 0x0112
	ts2068WTapeTrapPC = 0x0068
)

// isTS2068ExtensionROMActive reports whether TS2068 tape trap addresses
// currently mean what they should -- chunk 0 must genuinely be switched
// to Extension ROM right now (ts2068.go), or 0x0112/0x0068 are just
// ordinary low addresses that could mean anything, including normal
// Home ROM code.
func (m *SpectrumMemory) isTS2068ExtensionROMActive() bool {
	return m.isTS2068 && m.ts2068HSRChunk0 && m.ts2068ExRomSelect
}

type FastLoader struct {
	Enabled bool
}

// Real 48K ROM addresses (verified directly against rom/48.rom's actual
// bytes, not assumed from documentation or copied from another
// emulator's own internal timing offsets):
//
//	ldBytesTrapPC: NOT LD-BYTES's entry (0x0556). By this point the real
//	ROM has already executed EX AF,AF' (moving the caller's original
//	flag byte + LOAD/VERIFY carry into the shadow registers), flashed
//	the border, and performed its own BREAK-key check -- all genuine
//	ROM code, not reimplemented here. 0x056C is immediately before the
//	slow pulse-reading call (CALL 0x05E7) this trap replaces. Trapping
//	at the naive entry point instead would silently skip the BREAK
//	check, which is exactly the kind of gap a careful reference
//	implementation (SpecIde, github.com/MartianGirl/SpecIde) traps past,
//	not through.
//	saBytesTrapPC: SA-BYTES's genuine entry point. No equivalent
//	housekeeping-before-the-slow-part split exists on the save side --
//	the entire routine from here on is pulse generation -- so trapping
//	at the entry itself is correct.
const (
	ldBytesTrapPC = 0x056C
	saBytesTrapPC = 0x04C2
)

// TryIntercept attempts to intercept the real ROM's tape loading
// routine -- the standard 48K-compatible LD-BYTES (any model whose
// currently-paged ROM bank is genuinely that one) or TS2068's own
// R_TAPE (reached only when chunk 0 is genuinely switched to Extension
// ROM) -- and inject a parsed block directly, replacing only the slow
// pulse-by-pulse reading loop; everything before the trap point ran as
// genuine ROM code. Both routines share an identical register contract
// (confirmed by direct disassembly: TS2068's R_TAPE/W_TAPE are a
// byte-for-byte relocated copy of LD-BYTES/SA-BYTES, offset by a
// constant 0x045A), so trapLoad handles both without needing to know
// which one fired. Returns true if it fired (successfully or not -- a
// wrong/missing block is a legitimate real-hardware outcome, reported
// via Carry, not a reason to fall through to something else).
//
// Save-side (SA-BYTES/W_TAPE) is intentionally not implemented: zenzx
// has no existing tape-save capture to hook into, and adding one is new
// feature work, not part of hardening the load path. Both save trap
// points are still checked so a program calling into them does not
// risk falling through to whatever real ROM code happens to sit at
// that address being misread as something else -- they currently just
// decline (return false) and let the real save code run normally
// (slowly, generating real pulses nobody currently records, but that
// is a known, pre-existing gap).
func (fl *FastLoader) TryIntercept(zx *ZenZX, t *Tape) bool {
	if !fl.Enabled || zx == nil || t == nil {
		return false
	}

	if zx.memory.is48KROMActive() {
		switch zx.cpu.PC {
		case ldBytesTrapPC:
			return fl.trapLoad(zx, t)
		case saBytesTrapPC:
			return false // see doc comment: save-side capture not implemented
		}
	}

	if zx.memory.isTS2068ExtensionROMActive() {
		switch zx.cpu.PC {
		case ts2068RTapeTrapPC:
			return fl.trapLoad(zx, t) // identical register contract, confirmed by disassembly
		case ts2068WTapeTrapPC:
			return false // same reasoning as saBytesTrapPC
		}
	}

	return false
}

// trapLoad replicates the real LD-BYTES routine's own contract at
// ldBytesTrapPC, verified against rom/48.rom directly and cross-checked
// against SpecIde's independent implementation of the same routine
// (github.com/MartianGirl/SpecIde, source/src/Spectrum.cc) for the exit
// register conventions this comment cites individually below.
func (fl *FastLoader) trapLoad(zx *ZenZX, t *Tape) bool {
	cpu := zx.cpu

	// EX AF,AF' has already run (real ROM code, before our trap) -- the
	// caller's original flag byte and LOAD/VERIFY carry are now in the
	// shadow registers, not the main ones.
	expectedFlag := cpu.A_
	verify := cpu.F_&0x01 == 0 // Carry clear = VERIFY, set = LOAD

	if t.st.Position >= len(t.st.Blocks) {
		return false // nothing left to load -- let real ROM code report its own failure
	}
	block := t.st.Blocks[t.st.Position].Data
	flagOk := len(block) > 0 && block[0] == expectedFlag

	addr := cpu.IX()
	length := uint16(cpu.D)<<8 | uint16(cpu.E)

	if flagOk {
		checksum := block[0]
		pos := 1
		remaining := length
		ok := true
		for remaining > 0 && pos < len(block)-1 {
			b := block[pos]
			if verify {
				if zx.memory.Read(addr) != b {
					ok = false
					break
				}
			} else {
				zx.memory.Write(addr, b)
			}
			checksum ^= b
			addr++
			pos++
			remaining--
		}
		if ok && remaining == 0 && pos < len(block) {
			checksum ^= block[len(block)-1] // include the trailing checksum byte
		}

		cpu.SetIX(addr)
		cpu.D = uint8(remaining >> 8)
		cpu.E = uint8(remaining)
		cpu.H = checksum
		cpu.B = 0xB0 // SpecIde-derived exit convention (see doc comment)
		cpu.C ^= 0x03

		if ok && remaining == 0 && checksum == 0 {
			cpu.F |= 0x01 // Carry: genuine success
			// Real hardware note (SpecIde): release SPACE if held --
			// we're not supposed to be pressing it while loading, and a
			// stale press could look like an unwanted BREAK afterwards.
			zx.io.ReleaseKey(7, 0)
		} else {
			cpu.F &^= 0x01 // Carry clear: genuine failure (mismatch or short block)
		}
	} else {
		cpu.F &^= 0x01 // wrong block for what the caller asked for
	}

	t.st.Position++
	if t.st.Position >= len(t.st.Blocks) {
		t.st.Playing = false
	}

	// Return to LD-BYTES's own caller -- the return address already on
	// the stack from whoever CALLed LD-BYTES in the first place.
	low := zx.memory.Read(cpu.SP)
	high := zx.memory.Read(cpu.SP + 1)
	cpu.SP += 2
	cpu.PC = uint16(high)<<8 | uint16(low)
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
