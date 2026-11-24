// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package main

import (
	"github.com/ha1tch/zenzx"
	"github.com/ha1tch/zenzx/pkg/debug"
)

// machineAdapter bridges a zenzx.DebugView (the machine facade) to the
// engine's debug.MachineView interface. It lives here, in the command that
// imports both packages, so that pkg/debug need never import zenzx (which
// would be an import cycle, since the facade is in the zenzx package).
//
// The adapter is a thin shim: it copies the facade's register snapshot into
// the engine's identical Regs shape and maps the string register name onto
// the facade's typed RegName. Nothing here has logic of its own.
type machineAdapter struct {
	v zenzx.DebugView
}

func newMachineAdapter(v zenzx.DebugView) *machineAdapter { return &machineAdapter{v: v} }

func (a *machineAdapter) Regs() debug.Regs {
	r := a.v.Regs()
	return debug.Regs{
		A: r.A, F: r.F, B: r.B, C: r.C, D: r.D, E: r.E, H: r.H, L: r.L,
		IXH: r.IXH, IXL: r.IXL, IYH: r.IYH, IYL: r.IYL,
		I: r.I, R: r.R,
		SP: r.SP, PC: r.PC, WZ: r.WZ,
		IFF1: r.IFF1, IFF2: r.IFF2, Halted: r.Halted,
		Cycles: r.Cycles,
	}
}

func (a *machineAdapter) PC() uint16                        { return a.v.PC() }
func (a *machineAdapter) SetPC(addr uint16)                 { a.v.SetPC(addr) }
func (a *machineAdapter) ReadMem(addr uint16, n int) []byte { return a.v.ReadMem(addr, n) }
func (a *machineAdapter) WriteMem(addr uint16, data []byte) { a.v.WriteMem(addr, data) }
func (a *machineAdapter) Step() int                         { return a.v.Step() }
func (a *machineAdapter) Halted() bool                      { return a.v.Halted() }

func (a *machineAdapter) SetReg(name string, value uint16) bool {
	return a.v.SetReg(zenzx.RegName(name), value)
}
