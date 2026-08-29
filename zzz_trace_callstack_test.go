//go:build headless

package main

import (
	"fmt"
	"os"

	"github.com/ha1tch/zen80/z80"
)

// installM1Hook wires zx.cpu.M1Hook once, doing two independent jobs
// gated by their own flags so either can be used without the other:
//
//   - Stage 1 (m1Log): emit an M line for every M1 fetch -- normal
//     opcode fetches, prefix continuations (post-CB/DD/ED/FD), and
//     interrupt acknowledgement (NMI/IM0/IM0-fallback/IM1/IM2) -- with
//     the same armed/done windowing P lines already use.
//   - Stage 7 (ZTRACE_CALLSTACK=1): reconstruct call depth from the
//     opcode stream and emit an S line on every push/pop.
//
// Ordering note worth knowing when reading a trace: DebugPCHook fires
// "before fetch and interrupt handling" (zen80's own doc comment on
// it), so for a step that takes an interrupt, the P line for that step
// shows the pre-interrupt PC while the M/S lines for the SAME step
// show the interrupt vector -- both are correct, they're just recording
// different points within the same Step() call.
func installM1Hook(zx *ZenZX, out *os.File, m1Log bool, armed, done *bool, stepCount *int64, pcMin uint16, syms symTable) {
	callstackLog := os.Getenv("ZTRACE_CALLSTACK") == "1"
	if !m1Log && !callstackLog {
		return // leave M1Hook nil -- costs nothing, matches zen80's own cost model
	}

	var (
		depth     int
		pendingED bool
	)

	zx.cpu.M1Hook = func(pc uint16, opcode uint8, context string) {
		if !*armed || *done {
			return
		}
		if m1Log && pc >= pcMin {
			fmt.Fprintf(out, "M %d %04X %02X %s\n", *stepCount, pc, opcode, context)
		}
		if !callstackLog {
			return
		}
		switch context {
		case "normal":
			wasPendingED := pendingED
			pendingED = opcode == 0xED
			_ = wasPendingED
			switch {
			case opcode == 0xCD || isConditionalCallOpcode(opcode):
				if opcode == 0xCD || conditionMet(zx.cpu.F, opcode) {
					depth++
					fmt.Fprintf(out, "S %d %s depth %d\n", *stepCount, syms.annotate(pc), depth)
				}
			case isRSTOpcode(opcode):
				depth++
				fmt.Fprintf(out, "S %d %s depth %d\n", *stepCount, syms.annotate(pc), depth)
			case opcode == 0xC9 || isConditionalRetOpcode(opcode):
				if opcode == 0xC9 || conditionMet(zx.cpu.F, opcode) {
					if depth > 0 {
						depth--
					}
					fmt.Fprintf(out, "S %d %s depth %d\n", *stepCount, syms.annotate(pc), depth)
				}
			}
		case "post-ED":
			if pendingED && isRETNOrRETI(opcode) {
				if depth > 0 {
					depth--
				}
				fmt.Fprintf(out, "S %d %s depth %d\n", *stepCount, syms.annotate(pc), depth)
			}
			pendingED = false
		case "NMI", "IM0", "IM0-fallback", "IM1", "IM2":
			// An interrupt is a call with no matching CALL opcode -- the
			// hardware pushes PC and jumps, same as CALL does.
			depth++
			fmt.Fprintf(out, "S %d %s depth %d\n", *stepCount, syms.annotate(pc), depth)
		}
	}
}

// The classification tables below are static Z80 ISA facts, not
// implementation details of any particular core -- opcode encodings,
// not behaviour, so no zen80-specific verification was needed to write
// them (unlike everything else in this harness, which was checked
// against actual zen80/zenzx source before being relied on).

func isConditionalCallOpcode(op uint8) bool {
	switch op {
	case 0xC4, 0xCC, 0xD4, 0xDC, 0xE4, 0xEC, 0xF4, 0xFC:
		return true
	}
	return false
}

func isConditionalRetOpcode(op uint8) bool {
	switch op {
	case 0xC0, 0xC8, 0xD0, 0xD8, 0xE0, 0xE8, 0xF0, 0xF8:
		return true
	}
	return false
}

func isRSTOpcode(op uint8) bool {
	switch op {
	case 0xC7, 0xCF, 0xD7, 0xDF, 0xE7, 0xEF, 0xF7, 0xFF:
		return true
	}
	return false
}

// isRETNOrRETI covers RETN (ED 45, official) plus its documented
// undocumented aliases (ED 55/5D/65/6D/75/7D, which behave identically
// on real hardware) and RETI (ED 4D, official) -- all pop the return
// address the same way, so call-depth tracking treats them the same.
func isRETNOrRETI(op uint8) bool {
	switch op {
	case 0x45, 0x4D, 0x55, 0x5D, 0x65, 0x6D, 0x75, 0x7D:
		return true
	}
	return false
}

// conditionMet evaluates a conditional CALL/RET's condition code (bits
// 3-5 of the opcode, the standard Z80 cc field) against the flags
// register AT the M1 fetch of the CALL/RET opcode. This is safe to
// read here, not just before-the-fact: CALL/RET don't modify flags
// themselves, so F at fetch-time and F at the (later, same-step)
// execute-time decision point are identical -- there's no race between
// checking it here and the real hardware checking it during execution.
func conditionMet(f uint8, opcode uint8) bool {
	cc := (opcode >> 3) & 0x07
	switch cc {
	case 0:
		return f&z80.FlagZ == 0 // NZ
	case 1:
		return f&z80.FlagZ != 0 // Z
	case 2:
		return f&z80.FlagC == 0 // NC
	case 3:
		return f&z80.FlagC != 0 // C
	case 4:
		return f&z80.FlagPV == 0 // PO
	case 5:
		return f&z80.FlagPV != 0 // PE
	case 6:
		return f&z80.FlagS == 0 // P (positive)
	case 7:
		return f&z80.FlagS != 0 // M (minus)
	}
	return false
}
