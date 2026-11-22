// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

// Package debugcmd is the command layer of the ZenZX debugger: it parses a
// line of text into a command and renders engine results as fixed-format
// text. One grammar and one renderer serve both surfaces (the standalone CLI
// and the in-ZenZX HUD), so a transcript from one matches the other.
package debugcmd

import (
	"fmt"
	"strconv"
	"strings"
)

// Verb is a parsed command's kind.
type Verb int

const (
	VerbNone Verb = iota // empty line
	VerbBreak
	VerbWatch
	VerbStep
	VerbRun
	VerbRunTo
	VerbRegs
	VerbMem
	VerbDis
	VerbSet
	VerbReset
	VerbInfo
	VerbHelp
	VerbQuit
)

// Command is a parsed command line.
type Command struct {
	Verb Verb
	// Addr is the primary address operand (break/watch/mem/dis/runto/set-mem).
	Addr uint16
	// HasAddr says whether Addr was given (break with no addr lists).
	HasAddr bool
	// Count is a repeat/length operand (step N, mem LEN, dis N).
	Count int
	// Delete marks a `-ADDR` form (break -ADDR, watch -ADDR).
	Delete bool
	// Reg and Value carry `set REG VALUE` or the target of `set (ADDR) VALUE`.
	Reg      string
	Value    uint16
	IsMem    bool // set (ADDR) VALUE rather than set REG VALUE
	HasValue bool
}

// Parse turns a line into a Command. An unrecognised verb returns an error
// with the offending word; a recognised verb with a bad operand likewise.
func Parse(line string) (Command, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return Command{Verb: VerbNone}, nil
	}
	verb := strings.ToLower(fields[0])
	args := fields[1:]

	switch verb {
	case "break", "b", "bp":
		return parseBreak(args)
	case "watch", "w":
		return parseWatch(args)
	case "step", "s":
		return parseStep(args)
	case "run", "r", "continue", "cont", "c":
		return parseRun(args)
	case "regs", "reg":
		return Command{Verb: VerbRegs}, nil
	case "mem", "m", "x":
		return parseMem(args)
	case "dis", "disasm", "u", "d":
		return parseDis(args)
	case "set":
		return parseSet(args)
	case "reset":
		return Command{Verb: VerbReset}, nil
	case "info", "i":
		return Command{Verb: VerbInfo}, nil
	case "help", "?", "h":
		return Command{Verb: VerbHelp}, nil
	case "quit", "q", "exit":
		return Command{Verb: VerbQuit}, nil
	default:
		return Command{}, fmt.Errorf("unknown command %q (try 'help')", fields[0])
	}
}

func parseBreak(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{Verb: VerbBreak}, nil // list
	}
	a := args[0]
	if strings.HasPrefix(a, "-") {
		addr, err := parseAddr(strings.TrimPrefix(a, "-"))
		if err != nil {
			return Command{}, err
		}
		return Command{Verb: VerbBreak, Addr: addr, HasAddr: true, Delete: true}, nil
	}
	addr, err := parseAddr(a)
	if err != nil {
		return Command{}, err
	}
	return Command{Verb: VerbBreak, Addr: addr, HasAddr: true}, nil
}

func parseWatch(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{Verb: VerbWatch}, nil // list
	}
	a := args[0]
	if strings.HasPrefix(a, "-") {
		addr, err := parseAddr(strings.TrimPrefix(a, "-"))
		if err != nil {
			return Command{}, err
		}
		return Command{Verb: VerbWatch, Addr: addr, HasAddr: true, Delete: true}, nil
	}
	addr, err := parseAddr(a)
	if err != nil {
		return Command{}, err
	}
	return Command{Verb: VerbWatch, Addr: addr, HasAddr: true}, nil
}

func parseStep(args []string) (Command, error) {
	n := 1
	if len(args) >= 1 {
		v, err := strconv.Atoi(args[0])
		if err != nil || v < 1 {
			return Command{}, fmt.Errorf("step count %q: want a positive integer", args[0])
		}
		n = v
	}
	return Command{Verb: VerbStep, Count: n}, nil
}

func parseRun(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{Verb: VerbRun}, nil
	}
	// run ADDR / run to ADDR
	target := args[0]
	if strings.ToLower(args[0]) == "to" && len(args) >= 2 {
		target = args[1]
	}
	addr, err := parseAddr(target)
	if err != nil {
		return Command{}, err
	}
	return Command{Verb: VerbRunTo, Addr: addr, HasAddr: true}, nil
}

func parseMem(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{}, fmt.Errorf("mem: want an address")
	}
	// mem ADDR..END or mem ADDR [LEN]
	if strings.Contains(args[0], "..") {
		parts := strings.SplitN(args[0], "..", 2)
		start, err := parseAddr(parts[0])
		if err != nil {
			return Command{}, err
		}
		end, err := parseAddr(parts[1])
		if err != nil {
			return Command{}, err
		}
		n := int(end) - int(start) + 1
		if n < 1 {
			n = 1
		}
		return Command{Verb: VerbMem, Addr: start, HasAddr: true, Count: n}, nil
	}
	addr, err := parseAddr(args[0])
	if err != nil {
		return Command{}, err
	}
	n := 64
	if len(args) >= 2 {
		v, err := strconv.Atoi(args[1])
		if err != nil || v < 1 {
			return Command{}, fmt.Errorf("mem length %q: want a positive integer", args[1])
		}
		n = v
	}
	return Command{Verb: VerbMem, Addr: addr, HasAddr: true, Count: n}, nil
}

func parseDis(args []string) (Command, error) {
	c := Command{Verb: VerbDis, Count: 8}
	if len(args) >= 1 {
		addr, err := parseAddr(args[0])
		if err != nil {
			return Command{}, err
		}
		c.Addr = addr
		c.HasAddr = true
	}
	if len(args) >= 2 {
		v, err := strconv.Atoi(args[1])
		if err != nil || v < 1 {
			return Command{}, fmt.Errorf("dis count %q: want a positive integer", args[1])
		}
		c.Count = v
	}
	return c, nil
}

func parseSet(args []string) (Command, error) {
	if len(args) < 2 {
		return Command{}, fmt.Errorf("set: want REG VALUE or (ADDR) VALUE")
	}
	val, err := parseAddr(args[1])
	if err != nil {
		return Command{}, err
	}
	target := args[0]
	// set (ADDR) VALUE
	if strings.HasPrefix(target, "(") && strings.HasSuffix(target, ")") {
		addr, err := parseAddr(strings.TrimSuffix(strings.TrimPrefix(target, "("), ")"))
		if err != nil {
			return Command{}, err
		}
		return Command{Verb: VerbSet, IsMem: true, Addr: addr, HasAddr: true, Value: val, HasValue: true}, nil
	}
	return Command{Verb: VerbSet, Reg: strings.ToUpper(target), Value: val, HasValue: true}, nil
}

// parseAddr reads an address or value: $hhhh or 0xhhhh is hex, #ddd is
// decimal, a bare token is hex (the ZX convention). Symbol resolution, when
// a symbol table is present, is layered above this by the caller.
func parseAddr(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "$"):
		return parseIn(s[1:], 16)
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0X"):
		return parseIn(s[2:], 16)
	case strings.HasPrefix(s, "#"):
		return parseIn(s[1:], 10)
	default:
		return parseIn(s, 16)
	}
}

func parseIn(s string, base int) (uint16, error) {
	v, err := strconv.ParseUint(s, base, 32)
	if err != nil {
		return 0, fmt.Errorf("address %q: not a %d-base number", s, base)
	}
	return uint16(v), nil
}

// ParseAddrPublic exposes parseAddr for surfaces that need to read an address
// operand outside the command grammar (e.g. a -bin FILE:ADDR flag).
func ParseAddrPublic(s string) (uint16, error) { return parseAddr(s) }
