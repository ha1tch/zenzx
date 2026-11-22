// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

// Package debug is the ZenZX debugger engine: a debug session over one
// running machine, address-keyed, built on the machine's DebugView facade
// and zendis for disassembly. It is the engine layer of the workbench's
// debugger (docs/proposals/workbench.md, idea 1); the command grammar and
// the surfaces (CLI, HUD) sit above it.
//
// The engine never imports ZenZX internals: it drives the machine through
// the narrow handle the machine hands it (see MachineView), so the same
// engine serves a standalone machine today and, with an ensemble-aware
// handle, several machines later.
package debug

import "fmt"

// Regs is a snapshot of the CPU registers at one instant. It mirrors the
// machine facade's register snapshot so the engine does not depend on the
// zenzx package's own type.
type Regs struct {
	A, F, B, C, D, E, H, L uint8
	IXH, IXL, IYH, IYL     uint8
	I, R                   uint8
	SP, PC, WZ             uint16
	IFF1, IFF2, Halted     bool
	Cycles                 uint64
}

// Pair reads a 16-bit register pair by the two bytes it is made of.
func (r Regs) AF() uint16 { return uint16(r.A)<<8 | uint16(r.F) }
func (r Regs) BC() uint16 { return uint16(r.B)<<8 | uint16(r.C) }
func (r Regs) DE() uint16 { return uint16(r.D)<<8 | uint16(r.E) }
func (r Regs) HL() uint16 { return uint16(r.H)<<8 | uint16(r.L) }
func (r Regs) IX() uint16 { return uint16(r.IXH)<<8 | uint16(r.IXL) }
func (r Regs) IY() uint16 { return uint16(r.IYH)<<8 | uint16(r.IYL) }
func (r Regs) IR() uint16 { return uint16(r.I)<<8 | uint16(r.R) }

// FlagString renders F as the conventional [SZ5H3PNC] block, a dot where a
// flag is clear and its letter where set. Bit order high to low: S Z 5 H 3
// P/V N C.
func (r Regs) FlagString() string {
	bits := []struct {
		mask uint8
		set  byte
	}{
		{0x80, 'S'}, {0x40, 'Z'}, {0x20, '5'}, {0x10, 'H'},
		{0x08, '3'}, {0x04, 'P'}, {0x02, 'N'}, {0x01, 'C'},
	}
	out := make([]byte, 0, 10)
	out = append(out, '[')
	for _, b := range bits {
		if r.F&b.mask != 0 {
			out = append(out, b.set)
		} else {
			out = append(out, '.')
		}
	}
	out = append(out, ']')
	return string(out)
}

// IMString names the interrupt mode and enable state for the register dump.
// The machine facade does not expose the raw IM number yet, so this reports
// only the enable state from IFF1; the mode is added when the facade carries
// it.
func (r Regs) IMString() string {
	if r.IFF1 {
		return "EI"
	}
	return "DI"
}

// StopReason says why a Run or StepN returned. Exactly one concrete type is
// returned per stop; callers type-switch on it.
type StopReason interface{ stopReason() }

// StopBreakpoint: execution reached an armed breakpoint; Addr is the PC of
// the instruction that was about to execute (and did not).
type StopBreakpoint struct{ Addr uint16 }

// StopWatch: a watched address changed. Addr is the address, Old and New the
// bytes, PC the program counter of the instruction that caused the change.
type StopWatch struct {
	Addr     uint16
	Old, New uint8
	PC       uint16
}

// StopHalted: the CPU executed HALT and is halted.
type StopHalted struct{}

// StopStepsDone: a StepN completed its full count with no breakpoint, watch
// or halt. Count is how many instructions ran.
type StopStepsDone struct{ Count int }

// StopEnded: a Run's own limit was reached (a bounded run), no other stop.
type StopEnded struct{}

// StopBudget: Run/RunTo exhausted its instruction budget without reaching a
// breakpoint, watch, or HALT — typically an unreachable target or a tight
// idle loop. Steps is how many instructions ran. The caller can run again.
type StopBudget struct{ Steps int }

func (StopBreakpoint) stopReason() {}
func (StopWatch) stopReason()      {}
func (StopHalted) stopReason()     {}
func (StopStepsDone) stopReason()  {}
func (StopEnded) stopReason()      {}
func (StopBudget) stopReason()     {}

// String renders a stop reason as a one-line status, hex addresses upper
// case, the form the command layer prints after a run.
func StopString(s StopReason) string {
	switch v := s.(type) {
	case StopBreakpoint:
		return fmt.Sprintf("breakpoint @%04X", v.Addr)
	case StopWatch:
		return fmt.Sprintf("watch @%04X %02X->%02X (by PC %04X)", v.Addr, v.Old, v.New, v.PC)
	case StopHalted:
		return "halted"
	case StopStepsDone:
		return fmt.Sprintf("stepped %d", v.Count)
	case StopBudget:
		return fmt.Sprintf("run budget reached (%d instrs) — no breakpoint hit", v.Steps)
	case StopEnded:
		return "ended"
	default:
		return "stopped"
	}
}
