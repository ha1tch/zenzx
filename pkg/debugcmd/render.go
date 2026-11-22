// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package debugcmd

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zenzx/pkg/debug"
)

// RenderRegs formats a register dump in the fixed two-line form.
func RenderRegs(r debug.Regs) string {
	var b strings.Builder
	fmt.Fprintf(&b, " PC=%04X  SP=%04X  AF=%04X  BC=%04X  DE=%04X  HL=%04X\n",
		r.PC, r.SP, r.AF(), r.BC(), r.DE(), r.HL())
	fmt.Fprintf(&b, " IX=%04X  IY=%04X  IR=%04X  F=%s  (%s)  cyc=%d",
		r.IX(), r.IY(), r.IR(), r.FlagString(), r.IMString(), r.Cycles)
	return b.String()
}

// RenderDisasm formats a run of instructions: address, raw bytes, mnemonic
// and operands, aligned.
func RenderDisasm(ins []debug.Instruction) string {
	var b strings.Builder
	for i, in := range ins {
		if i > 0 {
			b.WriteByte('\n')
		}
		raw := make([]string, 0, len(in.Bytes))
		for _, by := range in.Bytes {
			raw = append(raw, fmt.Sprintf("%02X", by))
		}
		bytesCol := strings.Join(raw, " ")
		text := in.Mnemonic
		if in.Operands != "" {
			text += " " + in.Operands
		}
		fmt.Fprintf(&b, " %04X  %-11s  %s", in.Addr, bytesCol, text)
	}
	return b.String()
}

// RenderMem formats a hex+ASCII memory dump, 8 bytes a row, from base.
func RenderMem(base uint16, data []byte) string {
	var b strings.Builder
	for off := 0; off < len(data); off += 8 {
		end := off + 8
		if end > len(data) {
			end = len(data)
		}
		row := data[off:end]
		var hex strings.Builder
		var asc strings.Builder
		for i := 0; i < 8; i++ {
			if i < len(row) {
				fmt.Fprintf(&hex, "%02X ", row[i])
				c := row[i]
				if c >= 0x20 && c < 0x7F {
					asc.WriteByte(c)
				} else {
					asc.WriteByte('.')
				}
			} else {
				hex.WriteString("   ")
				asc.WriteByte(' ')
			}
		}
		if off > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, " %04X  %s %s", base+uint16(off), hex.String(), asc.String())
	}
	return b.String()
}
