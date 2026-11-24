// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package debugcmd

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zenzx/pkg/debug"
)

// Exec runs one parsed command against an engine and returns the text to show
// (possibly empty) and an error for a bad command. It is the single dispatch
// point both surfaces call, so their behaviour cannot drift.
//
// Quit is signalled by the returned quit bool; the caller owns the loop.
func Exec(e *debug.Engine, c Command) (out string, quit bool, err error) {
	switch c.Verb {
	case VerbNone:
		return "", false, nil

	case VerbHelp:
		return helpText(), false, nil

	case VerbQuit:
		return "", true, nil

	case VerbRegs:
		return RenderRegs(e.Regs()), false, nil

	case VerbInfo:
		return infoText(e), false, nil

	case VerbBreak:
		if !c.HasAddr {
			return listBreaks(e), false, nil
		}
		if c.Delete {
			e.DelBreak(c.Addr)
			return fmt.Sprintf(" breakpoint @%04X removed", c.Addr), false, nil
		}
		e.AddBreak(c.Addr)
		return fmt.Sprintf(" breakpoint @%04X", c.Addr), false, nil

	case VerbWatch:
		if !c.HasAddr {
			return listWatches(e), false, nil
		}
		if c.Delete {
			e.DelWatch(c.Addr)
			return fmt.Sprintf(" watch @%04X removed", c.Addr), false, nil
		}
		e.AddWatch(c.Addr)
		return fmt.Sprintf(" watch @%04X", c.Addr), false, nil

	case VerbStep:
		reason := e.StepN(c.Count)
		return stopAndContext(e, reason), false, nil

	case VerbRun:
		reason := e.Run()
		return stopAndContext(e, reason), false, nil

	case VerbRunTo:
		reason := e.RunTo(c.Addr)
		return stopAndContext(e, reason), false, nil

	case VerbMem:
		data := e.ReadMem(c.Addr, c.Count)
		return RenderMem(c.Addr, data), false, nil

	case VerbDis:
		at := c.Addr
		if !c.HasAddr {
			at = e.Regs().PC
		}
		return RenderDisasm(e.Disasm(at, c.Count)), false, nil

	case VerbSet:
		if c.IsMem {
			e.WriteMem(c.Addr, []byte{byte(c.Value)})
			return fmt.Sprintf(" (%04X)=%02X", c.Addr, byte(c.Value)), false, nil
		}
		if ok := e.SetReg(c.Reg, c.Value); !ok {
			return "", false, fmt.Errorf("set: unknown register %q", c.Reg)
		}
		return fmt.Sprintf(" %s=%04X", c.Reg, c.Value), false, nil

	case VerbReset:
		// Reset is a machine operation; the engine exposes it via a PC set to
		// 0 only as a stand-in here. A real reset goes through the facade; the
		// PoC's CLI wires the machine's Reset in cmd/zendbg where the machine
		// is visible. Here we report that it is handled by the caller.
		return "", false, fmt.Errorf("reset: handled by the surface, not the engine")

	default:
		return "", false, fmt.Errorf("command not implemented")
	}
}

// stopAndContext renders a stop reason followed by the current register line,
// the form shown after any run or step.
func stopAndContext(e *debug.Engine, reason debug.StopReason) string {
	var b strings.Builder
	fmt.Fprintf(&b, " stopped: %s\n", debug.StopString(reason))
	b.WriteString(RenderRegs(e.Regs()))
	return b.String()
}

func listBreaks(e *debug.Engine) string {
	bs := e.Breaks()
	if len(bs) == 0 {
		return " no breakpoints"
	}
	var b strings.Builder
	fmt.Fprintf(&b, " %d breakpoint(s):", len(bs))
	for _, bp := range bs {
		fmt.Fprintf(&b, "\n  @%04X", bp.Addr)
	}
	return b.String()
}

func listWatches(e *debug.Engine) string {
	ws := e.Watches()
	if len(ws) == 0 {
		return " no watchpoints"
	}
	var b strings.Builder
	fmt.Fprintf(&b, " %d watchpoint(s):", len(ws))
	for _, w := range ws {
		fmt.Fprintf(&b, "\n  @%04X", w)
	}
	return b.String()
}

func infoText(e *debug.Engine) string {
	r := e.Regs()
	tgt := "Z80"
	if e.Target().String() == "Z80N" {
		tgt = "Z80N"
	}
	last := "—"
	if e.LastStop() != nil {
		last = debug.StopString(e.LastStop())
	}
	return fmt.Sprintf(" target=%s  PC=%04X  cyc=%d  breaks=%d  watches=%d  last=%s",
		tgt, r.PC, r.Cycles, len(e.Breaks()), len(e.Watches()), last)
}

func helpText() string {
	return strings.TrimRight(`
 commands:
  break ADDR | break | break -ADDR     arm / list / remove a breakpoint
  watch ADDR | watch | watch -ADDR     arm / list / remove a memory watch
  step [N] | s                         execute N instructions (default 1)
  run | r | continue                   run until breakpoint, watch or HALT
  run [to] ADDR                        run until PC reaches ADDR
  regs                                 register dump
  mem ADDR [LEN] | mem ADDR..END       hex+ASCII memory dump (default 64)
  dis [ADDR] [N] | u                   disassemble N instrs (default 8, from PC)
  set REG VALUE | set (ADDR) VALUE     poke a register or a memory byte
  reset                                reset the machine
  info                                 target, PC, cycles, armed breaks/watches
  help | ?                             this list
  quit | q                             leave
 addresses: $hhhh or 0xhhhh hex, #ddd decimal, bare token hex`, "\n")
}
