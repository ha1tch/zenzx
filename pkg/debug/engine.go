// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package debug

import (
	"github.com/ha1tch/zendis/pkg/disasm"
)

// MachineView is what the engine needs from a machine: read and write its
// registers and memory, and step it one instruction. zenzx.DebugView
// satisfies this, so the engine depends on the facade's shape, not on the
// zenzx package. An ensemble-aware view (several machines) satisfies the
// same interface, so the engine is unchanged when that lands.
//
// The method set is deliberately the facade's, minus the process-global
// fetch hook: the engine drives Step itself and tests the PC in its own
// loop, which needs no hook and is correct for several machines.
type MachineView interface {
	Regs() RegsSnapshot
	PC() uint16
	SetPC(addr uint16)
	SetReg(name string, value uint16) bool
	ReadMem(addr uint16, n int) []byte
	WriteMem(addr uint16, data []byte)
	Step() int
	Halted() bool
}

// RegsSnapshot is the register snapshot the facade returns. It is an alias
// of the engine's Regs so a facade can return the engine's type directly, or
// convert to it; both the facade and the engine name the same fields.
type RegsSnapshot = Regs

// Breakpoint is one armed address breakpoint.
type Breakpoint struct {
	Addr uint16
}

// Watchpoint watches one address for a change in its byte value.
type Watchpoint struct {
	Addr uint16
	last uint8 // the value last seen, for change detection
}

// Instruction is one decoded instruction at an address, the engine's own
// small shape over zendis's richer disasm.Instruction so callers need not
// import zendis.
type Instruction struct {
	Addr      uint16
	Bytes     []byte
	Mnemonic  string
	Operands  string
	Length    int
	Undefined bool
}

// Engine is a debug session over one machine.
type Engine struct {
	m     MachineView
	bps   map[uint16]Breakpoint
	watch []Watchpoint
	tgt   disasm.Target // TargetZ80 or TargetZ80N

	runBudget int // per-Run instruction cap; 0 => DefaultRunBudget
	lastStop  StopReason
}

// NewEngine makes an engine over a machine view. tgt selects the Z80 or Z80N
// decoding for disassembly (a Next machine passes TargetZ80N).
func NewEngine(m MachineView, tgt disasm.Target) *Engine {
	return &Engine{
		m:   m,
		bps: make(map[uint16]Breakpoint),
		tgt: tgt,
	}
}

// --- inspection ------------------------------------------------------------

// Regs reads the live registers.
func (e *Engine) Regs() Regs { return e.m.Regs() }

// ReadMem copies n bytes from addr (wrapping the address space).
func (e *Engine) ReadMem(addr uint16, n int) []byte { return e.m.ReadMem(addr, n) }

// WriteMem writes bytes at addr.
func (e *Engine) WriteMem(addr uint16, data []byte) { e.m.WriteMem(addr, data) }

// SetReg writes a register or pair; reports whether the name was known.
func (e *Engine) SetReg(name string, value uint16) bool { return e.m.SetReg(name, value) }

// LastStop is why the last Run or StepN returned, or nil before any run.
func (e *Engine) LastStop() StopReason { return e.lastStop }

// Target reports the decoding target (Z80 or Z80N).
func (e *Engine) Target() disasm.Target { return e.tgt }

// Disasm decodes count instructions starting at addr, reading the live
// memory each instruction needs. A short or undefined opcode still advances
// by the instruction's real length, so the stream never stalls.
func (e *Engine) Disasm(addr uint16, count int) []Instruction {
	out := make([]Instruction, 0, count)
	cur := addr
	for i := 0; i < count; i++ {
		// A Z80 instruction is at most 4 bytes; read a small window live.
		window := e.m.ReadMem(cur, 4)
		ins, err := disasm.DecodeTarget(window, 0, cur, e.tgt)
		if err != nil || ins == nil {
			// Undecodable: emit a 1-byte DB and move on, so the view holds.
			out = append(out, Instruction{
				Addr:      cur,
				Bytes:     []byte{window[0]},
				Mnemonic:  "DB",
				Operands:  hexByte(window[0]),
				Length:    1,
				Undefined: true,
			})
			cur++
			continue
		}
		out = append(out, Instruction{
			Addr:      ins.Address,
			Bytes:     ins.Bytes,
			Mnemonic:  ins.Mnemonic,
			Operands:  ins.Operands,
			Length:    ins.Length,
			Undefined: ins.Undefined,
		})
		if ins.Length <= 0 {
			cur++ // defensive: never fail to advance
		} else {
			cur += uint16(ins.Length)
		}
	}
	return out
}

func hexByte(b uint8) string {
	const digits = "0123456789ABCDEF"
	return "0x" + string([]byte{digits[b>>4], digits[b&0x0F]})
}
