// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package debug

import "sort"

// --- breakpoints -----------------------------------------------------------

// AddBreak arms an address breakpoint. Arming one already armed is a no-op.
func (e *Engine) AddBreak(addr uint16) {
	e.bps[addr] = Breakpoint{Addr: addr}
}

// DelBreak removes a breakpoint. Removing one not set is a no-op.
func (e *Engine) DelBreak(addr uint16) {
	delete(e.bps, addr)
}

// Breaks returns the armed breakpoints, ascending by address.
func (e *Engine) Breaks() []Breakpoint {
	out := make([]Breakpoint, 0, len(e.bps))
	for _, b := range e.bps {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Addr < out[j].Addr })
	return out
}

// --- watchpoints -----------------------------------------------------------

// AddWatch arms a watch on an address, seeding it with the current byte so
// the first change (not the current value) is what fires.
func (e *Engine) AddWatch(addr uint16) {
	cur := e.m.ReadMem(addr, 1)[0]
	for i := range e.watch {
		if e.watch[i].Addr == addr {
			e.watch[i].last = cur
			return
		}
	}
	e.watch = append(e.watch, Watchpoint{Addr: addr, last: cur})
}

// DelWatch removes a watch. Removing one not set is a no-op.
func (e *Engine) DelWatch(addr uint16) {
	for i := range e.watch {
		if e.watch[i].Addr == addr {
			e.watch = append(e.watch[:i], e.watch[i+1:]...)
			return
		}
	}
}

// Watches returns the armed watch addresses, ascending.
func (e *Engine) Watches() []uint16 {
	out := make([]uint16, 0, len(e.watch))
	for _, w := range e.watch {
		out = append(out, w.Addr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// reseedWatches refreshes every watch's last-seen byte to the current memory,
// so a fresh run measures change from now, not from whenever the watch was
// armed. Called at the start of each Run/StepN.
func (e *Engine) reseedWatches() {
	for i := range e.watch {
		e.watch[i].last = e.m.ReadMem(e.watch[i].Addr, 1)[0]
	}
}

// checkWatches tests every watch after a step, returning the first that
// changed (and updating all last-seen bytes). pcBefore is the PC of the
// instruction that just ran, reported as the writer.
func (e *Engine) checkWatches(pcBefore uint16) (StopWatch, bool) {
	var hit StopWatch
	found := false
	for i := range e.watch {
		now := e.m.ReadMem(e.watch[i].Addr, 1)[0]
		if now != e.watch[i].last {
			if !found {
				hit = StopWatch{Addr: e.watch[i].Addr, Old: e.watch[i].last, New: now, PC: pcBefore}
				found = true
			}
			e.watch[i].last = now
		}
	}
	return hit, found
}

// --- execution -------------------------------------------------------------

// Step executes exactly one instruction, ignoring breakpoints (a breakpoint
// at the current PC does not stop a deliberate single step). A watch that the
// step trips is still reported.
func (e *Engine) Step() StopReason {
	e.reseedWatches()
	pc := e.m.PC()
	if e.m.Halted() {
		e.lastStop = StopHalted{}
		return e.lastStop
	}
	e.m.Step()
	if w, ok := e.checkWatches(pc); ok {
		e.lastStop = w
		return w
	}
	if e.m.Halted() {
		e.lastStop = StopHalted{}
		return e.lastStop
	}
	e.lastStop = StopStepsDone{Count: 1}
	return e.lastStop
}

// StepN steps up to n instructions, stopping early on a breakpoint (checked
// before each step after the first), a watch, or HALT. The first instruction
// always runs even if a breakpoint sits on the starting PC, so repeated
// StepN(1) makes progress past a breakpoint.
func (e *Engine) StepN(n int) StopReason {
	if n <= 0 {
		e.lastStop = StopStepsDone{Count: 0}
		return e.lastStop
	}
	e.reseedWatches()
	done := 0
	for done < n {
		if e.m.Halted() {
			e.lastStop = StopHalted{}
			return e.lastStop
		}
		// Breakpoints gate instructions after the first.
		if done > 0 {
			if _, ok := e.bps[e.m.PC()]; ok {
				e.lastStop = StopBreakpoint{Addr: e.m.PC()}
				return e.lastStop
			}
		}
		pc := e.m.PC()
		e.m.Step()
		done++
		if w, ok := e.checkWatches(pc); ok {
			e.lastStop = w
			return w
		}
	}
	e.lastStop = StopStepsDone{Count: done}
	return e.lastStop
}

// DefaultRunBudget bounds a single Run/RunTo so an unreachable breakpoint or
// target cannot spin the engine (and the surface above it) forever. It is a
// large but finite number of instructions; a real program reaches a
// breakpoint, HALTs, or idles in a loop long before this. A run that hits the
// budget stops with StopBudget, and the caller can simply run again.
const DefaultRunBudget = 20_000_000

// RunBudget is the per-Run instruction cap. Zero or negative means
// DefaultRunBudget. Exposed so a surface can lower it (a responsive HUD) or a
// test can set it small.
func (e *Engine) SetRunBudget(n int) { e.runBudget = n }

func (e *Engine) budget() int {
	if e.runBudget > 0 {
		return e.runBudget
	}
	return DefaultRunBudget
}

// Run executes until an armed breakpoint, a watch, HALT, or the run budget.
// It is the self-driven loop (no process-global hook): the PC is tested
// before each instruction, so execution stops with PC at the breakpoint and
// that instruction not yet executed. A breakpoint on the starting PC does not
// trap immediately (so `run` after stopping at one makes progress); it traps
// on the next time that address is reached.
func (e *Engine) Run() StopReason {
	e.reseedWatches()
	first := true
	budget := e.budget()
	for i := 0; i < budget; i++ {
		if e.m.Halted() {
			e.lastStop = StopHalted{}
			return e.lastStop
		}
		pc := e.m.PC()
		if !first {
			if _, ok := e.bps[pc]; ok {
				e.lastStop = StopBreakpoint{Addr: pc}
				return e.lastStop
			}
		}
		first = false
		e.m.Step()
		if w, ok := e.checkWatches(pc); ok {
			e.lastStop = w
			return w
		}
	}
	e.lastStop = StopBudget{Steps: budget}
	return e.lastStop
}

// RunTo runs until the PC reaches target (or a breakpoint, watch, HALT, or
// the run budget intervenes first). target is not treated as a breakpoint on
// the starting PC; execution always makes at least one step.
func (e *Engine) RunTo(target uint16) StopReason {
	e.reseedWatches()
	budget := e.budget()
	for i := 0; i < budget; i++ {
		if e.m.Halted() {
			e.lastStop = StopHalted{}
			return e.lastStop
		}
		pc := e.m.PC()
		e.m.Step()
		if w, ok := e.checkWatches(pc); ok {
			e.lastStop = w
			return w
		}
		if e.m.PC() == target {
			e.lastStop = StopBreakpoint{Addr: target}
			return e.lastStop
		}
		if _, ok := e.bps[e.m.PC()]; ok {
			e.lastStop = StopBreakpoint{Addr: e.m.PC()}
			return e.lastStop
		}
	}
	e.lastStop = StopBudget{Steps: budget}
	return e.lastStop
}
