// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package debug

import (
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// fakeMachine is a tiny MachineView over a flat 64K memory and a trivial CPU
// model: it decodes just enough to step a handful of opcodes used in the
// tests (NOP, LD A,n, LD (nn),A, JP nn, HALT). It exists so the engine's
// invariants are tested without constructing a whole ZenZX machine; the
// cmd/zendbg script tests exercise the real machine.
type fakeMachine struct {
	mem    [65536]byte
	pc     uint16
	a      uint8
	cycles uint64
	halted bool
}

func (f *fakeMachine) Regs() Regs {
	return Regs{A: f.a, PC: f.pc, Cycles: f.cycles, Halted: f.halted}
}
func (f *fakeMachine) PC() uint16     { return f.pc }
func (f *fakeMachine) SetPC(a uint16) { f.pc = a }
func (f *fakeMachine) Halted() bool   { return f.halted }
func (f *fakeMachine) ReadMem(a uint16, n int) []byte {
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = f.mem[a+uint16(i)]
	}
	return out
}
func (f *fakeMachine) WriteMem(a uint16, d []byte) {
	for i, b := range d {
		f.mem[a+uint16(i)] = b
	}
}
func (f *fakeMachine) SetReg(name string, v uint16) bool {
	if name == "A" {
		f.a = uint8(v)
		return true
	}
	return false
}

// Step executes one instruction of the tiny model and returns its t-states.
func (f *fakeMachine) Step() int {
	op := f.mem[f.pc]
	switch op {
	case 0x00: // NOP
		f.pc++
		f.cycles += 4
		return 4
	case 0x3E: // LD A,n
		f.a = f.mem[f.pc+1]
		f.pc += 2
		f.cycles += 7
		return 7
	case 0x32: // LD (nn),A
		addr := uint16(f.mem[f.pc+1]) | uint16(f.mem[f.pc+2])<<8
		f.mem[addr] = f.a
		f.pc += 3
		f.cycles += 13
		return 13
	case 0xC3: // JP nn
		f.pc = uint16(f.mem[f.pc+1]) | uint16(f.mem[f.pc+2])<<8
		f.cycles += 10
		return 10
	case 0x76: // HALT
		f.halted = true
		f.cycles += 4
		return 4
	default:
		f.pc++
		f.cycles += 4
		return 4
	}
}

// loadProgram writes bytes at addr and sets PC there.
func (f *fakeMachine) load(addr uint16, prog []byte) {
	for i, b := range prog {
		f.mem[addr+uint16(i)] = b
	}
	f.pc = addr
}

func newEngineWith(prog []byte, at uint16) (*Engine, *fakeMachine) {
	m := &fakeMachine{}
	m.load(at, prog)
	return NewEngine(m, disasm.TargetZ80), m
}

// I1: after `break A; run`, the machine stops with PC == A, the breakpoint
// instruction not yet executed.
func TestRunStopsBeforeBreakpointInstruction(t *testing.T) {
	// 8000: 3E 01   LD A,1
	// 8002: 00      NOP        <- breakpoint here
	// 8003: 76      HALT
	e, _ := newEngineWith([]byte{0x3E, 0x01, 0x00, 0x76}, 0x8000)
	e.AddBreak(0x8002)
	reason := e.Run()
	bp, ok := reason.(StopBreakpoint)
	if !ok {
		t.Fatalf("want StopBreakpoint, got %T (%s)", reason, StopString(reason))
	}
	if bp.Addr != 0x8002 {
		t.Errorf("breakpoint addr = %04X, want 8002", bp.Addr)
	}
	if pc := e.Regs().PC; pc != 0x8002 {
		t.Errorf("PC after run = %04X, want 8002 (instruction not yet executed)", pc)
	}
	// A was set by LD A,1 before the breakpoint; the NOP at 8002 has NOT run.
	if a := e.Regs().A; a != 0x01 {
		t.Errorf("A = %02X, want 01 (LD A,1 ran, NOP did not)", a)
	}
}

// I2: step advances PC by one instruction's length and cycles by its t-states.
func TestStepAdvancesOneInstruction(t *testing.T) {
	e, _ := newEngineWith([]byte{0x3E, 0x01, 0x00}, 0x8000) // LD A,1 ; NOP
	c0 := e.Regs().Cycles
	e.StepN(1)
	if pc := e.Regs().PC; pc != 0x8002 {
		t.Errorf("PC after one step = %04X, want 8002 (LD A,1 is 2 bytes)", pc)
	}
	if dc := e.Regs().Cycles - c0; dc != 7 {
		t.Errorf("cycles advanced %d, want 7 (LD A,n)", dc)
	}
}

// I3: stepN stops early on a breakpoint and reports it.
func TestStepNStopsOnBreakpoint(t *testing.T) {
	e, _ := newEngineWith([]byte{0x00, 0x00, 0x00, 0x00, 0x76}, 0x8000)
	e.AddBreak(0x8002)
	reason := e.StepN(10)
	bp, ok := reason.(StopBreakpoint)
	if !ok {
		t.Fatalf("want StopBreakpoint, got %T (%s)", reason, StopString(reason))
	}
	if bp.Addr != 0x8002 {
		t.Errorf("stopped at %04X, want 8002", bp.Addr)
	}
}

// I4: inspection (regs/mem/dis) does not mutate the CPU — the cycle counter
// is unchanged by reads.
func TestInspectionIsReadOnly(t *testing.T) {
	e, _ := newEngineWith([]byte{0x3E, 0x01, 0x00}, 0x8000)
	c0 := e.Regs().Cycles
	_ = e.Regs()
	_ = e.ReadMem(0x8000, 16)
	_ = e.Disasm(0x8000, 4)
	if c := e.Regs().Cycles; c != c0 {
		t.Errorf("cycles changed by inspection: %d -> %d", c0, c)
	}
}

// I6: a watchpoint fires on the first write and reports the writing PC.
func TestWatchpointFiresWithWritingPC(t *testing.T) {
	// 8000: 3E AA     LD A,0xAA
	// 8002: 32 00 40  LD (0x4000),A   <- writes the watched address
	// 8005: 76        HALT
	e, _ := newEngineWith([]byte{0x3E, 0xAA, 0x32, 0x00, 0x40, 0x76}, 0x8000)
	e.AddWatch(0x4000)
	reason := e.Run()
	w, ok := reason.(StopWatch)
	if !ok {
		t.Fatalf("want StopWatch, got %T (%s)", reason, StopString(reason))
	}
	if w.Addr != 0x4000 || w.New != 0xAA {
		t.Errorf("watch = @%04X %02X->%02X, want @4000 00->AA", w.Addr, w.Old, w.New)
	}
	if w.PC != 0x8002 {
		t.Errorf("writing PC = %04X, want 8002 (the LD (nn),A)", w.PC)
	}
}

// Run halts cleanly on HALT.
func TestRunStopsOnHalt(t *testing.T) {
	e, _ := newEngineWith([]byte{0x00, 0x76}, 0x8000)
	reason := e.Run()
	if _, ok := reason.(StopHalted); !ok {
		t.Fatalf("want StopHalted, got %T (%s)", reason, StopString(reason))
	}
}

// An unreachable breakpoint stops via the run budget instead of hanging.
func TestRunBudgetStopsUnreachable(t *testing.T) {
	// 8000: C3 00 80  JP 0x8000   (tight infinite loop)
	e, _ := newEngineWith([]byte{0xC3, 0x00, 0x80}, 0x8000)
	e.SetRunBudget(1000)
	e.AddBreak(0x9000) // never reached
	reason := e.Run()
	if _, ok := reason.(StopBudget); !ok {
		t.Fatalf("want StopBudget, got %T (%s)", reason, StopString(reason))
	}
}
