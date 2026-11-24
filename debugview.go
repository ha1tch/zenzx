// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "github.com/ha1tch/zen80/z80"

// DebugView is the narrow, embedder-facing handle a debugger attaches to a
// running machine. It is the T-39 machine facade's debugging seam and
// nothing more: enough to read and write the CPU's registers and the
// machine's memory, to single-step, and to observe each instruction fetch,
// without exposing the whole ZenZX internals or letting a debugger disturb
// the emulation's own wiring.
//
// A DebugView is a value that borrows the live machine; it holds no state
// of its own. Driving execution through it (Step) advances the same CPU the
// host's RunFrame drives, so a debugger and the normal frame loop act on one
// machine, never on a copy.
//
// Multiple machines: the per-instruction fetch seam (SetFetchHook) wraps the
// package-global z80.DebugPCHook, which is shared across every z80.Z80 in
// the process. For a single standalone machine that is exact. An ensemble of
// machines must not rely on it; a debugger over several machines drives each
// one's Step directly instead, which needs no global hook. See
// docs/proposals/workbench.md (several machines) and zen80's DebugPCHook.
type DebugView struct {
	zx *ZenZX
}

// Debug returns a DebugView over this machine. It is the only entry point a
// debugger needs; the ZenZX internals it reaches stay unexported.
func (zx *ZenZX) Debug() DebugView {
	return DebugView{zx: zx}
}

// --- registers -------------------------------------------------------------

// Regs is a snapshot of the CPU's registers at one instant. It is a copy:
// reading it never touches the live CPU, and writing a field here does not
// change the machine. Use SetReg to write a register back.
type Regs struct {
	A, F, B, C, D, E, H, L uint8
	IXH, IXL, IYH, IYL     uint8
	I, R                   uint8
	SP, PC, WZ             uint16
	IFF1, IFF2, Halted     bool
	Cycles                 uint64
}

// Regs reads the live CPU's registers into a snapshot.
func (d DebugView) Regs() Regs {
	c := d.zx.cpu
	return Regs{
		A: c.A, F: c.F, B: c.B, C: c.C, D: c.D, E: c.E, H: c.H, L: c.L,
		IXH: c.IXH, IXL: c.IXL, IYH: c.IYH, IYL: c.IYL,
		I: c.I, R: c.R,
		SP: c.SP, PC: c.PC, WZ: c.WZ,
		IFF1: c.IFF1, IFF2: c.IFF2, Halted: c.Halted,
		Cycles: c.Cycles,
	}
}

// PC reads the live program counter.
func (d DebugView) PC() uint16 { return d.zx.cpu.PC }

// SetPC sets the live program counter.
func (d DebugView) SetPC(addr uint16) { d.zx.cpu.PC = addr }

// RegName is a writable register named for the debugger's `set REG VALUE`.
type RegName string

const (
	RegA  RegName = "A"
	RegF  RegName = "F"
	RegB  RegName = "B"
	RegC  RegName = "C"
	RegD  RegName = "D"
	RegE  RegName = "E"
	RegH  RegName = "H"
	RegL  RegName = "L"
	RegBC RegName = "BC"
	RegDE RegName = "DE"
	RegHL RegName = "HL"
	RegIX RegName = "IX"
	RegIY RegName = "IY"
	RegSP RegName = "SP"
	RegPC RegName = "PC"
)

// SetReg writes a register or register pair. An 8-bit register ignores the
// high byte of value. It reports whether name was recognised.
func (d DebugView) SetReg(name RegName, value uint16) (ok bool) {
	c := d.zx.cpu
	lo := uint8(value)
	switch name {
	case RegA:
		c.A = lo
	case RegF:
		c.F = lo
	case RegB:
		c.B = lo
	case RegC:
		c.C = lo
	case RegD:
		c.D = lo
	case RegE:
		c.E = lo
	case RegH:
		c.H = lo
	case RegL:
		c.L = lo
	case RegBC:
		c.B, c.C = uint8(value>>8), lo
	case RegDE:
		c.D, c.E = uint8(value>>8), lo
	case RegHL:
		c.H, c.L = uint8(value>>8), lo
	case RegIX:
		c.IXH, c.IXL = uint8(value>>8), lo
	case RegIY:
		c.IYH, c.IYL = uint8(value>>8), lo
	case RegSP:
		c.SP = value
	case RegPC:
		c.PC = value
	default:
		return false
	}
	return true
}

// --- memory ----------------------------------------------------------------

// ReadMem copies n bytes from the machine's memory starting at addr, wrapping
// the 16-bit address space. It reads through the same SpectrumMemory the CPU
// uses, so it sees the current paging.
func (d DebugView) ReadMem(addr uint16, n int) []byte {
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = d.zx.memory.Read(addr + uint16(i))
	}
	return out
}

// WriteMem writes bytes into the machine's memory starting at addr, wrapping
// the address space. It is the raw poke behind the debugger's `set (ADDR)`;
// the workbench's checked patch step (proposals/workbench.md, idea 2) is a
// later, separate thing.
func (d DebugView) WriteMem(addr uint16, data []byte) {
	for i, b := range data {
		d.zx.memory.Write(addr+uint16(i), b)
	}
}

// --- execution -------------------------------------------------------------

// Step executes one instruction and returns the T-states it took. PC, the
// registers and the cycle count advance as they do in normal running; only
// the caller, not the frame loop, decides when the next instruction runs.
func (d DebugView) Step() int {
	return d.zx.cpu.Step()
}

// Halted reports whether the CPU is halted.
func (d DebugView) Halted() bool { return d.zx.cpu.Halted }

// SetFetchHook installs a function called with the PC at the top of every
// Step, before the instruction executes, or clears it when hook is nil. It
// wraps zen80's unconditional DebugPCHook (not the DEBUG_M1-gated M1Hook), so
// it fires whatever the build's tracing flags are.
//
// The hook is process-global in zen80, so this is for a single machine. A
// debugger that drives Step itself in a loop (checking PC after each) needs
// no hook and is the path the ensemble case must take.
func (d DebugView) SetFetchHook(hook func(pc uint16)) {
	z80.DebugPCHook = hook
}
