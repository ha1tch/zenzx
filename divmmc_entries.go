package main

import (
	"fmt"
	"os"
	"strings"
)

// DivMMC automap: which opcode fetches map the interface in and out, and when (T-36).
//
// The original DivMMC has fixed entry points. The Next's are programmable through NextREG
// 0xB8-0xBB, and two properties of them matter to NextZXOS:
//
//   - Most entry points count only while ROM 3, the 48K BASIC, is the ROM at 0x0000
//     ("only when rom3 is present"). At reset only 0x0000 is unconditional. NextZXOS runs
//     its own interrupt handler at 0x0038 in its other ROMs; an interface that maps itself
//     in there takes every interrupt away from the system.
//   - Mapping is either instant (the opcode at the entry point is already fetched from the
//     DivMMC) or delayed (that opcode comes from the ROM, and the DivMMC is there from the
//     next memory access on). Unmapping at 0x1FF8-0x1FFF is delayed: the opcode there is
//     the DivMMC's own.
//
// From the Next's registers.txt (in the distribution, /docs/extra-hw/io-port-system):
//
//	0xB8  entry points 0: bit n enables address 8n (0x0000 ... 0x0038); reset 0x83
//	0xB9  valid 0: bit n = 1 always, 0 only with ROM 3; reset 0x01
//	0xBA  timing 0: bit n = 1 instant, 0 delayed; reset 0x00
//	0xBB  entry points 1; reset 0xCD
//	      bit 7  0x3Dxx, instant, ROM 3          bit 3  0x0562, delayed, ROM 3
//	      bit 6  unmap at 0x1FF8-0x1FFF, delayed  bit 2  0x04C6, delayed, ROM 3
//	      bit 5  0x056A, delayed, ROM 3           bit 1  0x0066 with the button, instant
//	      bit 4  0x04D7, delayed, ROM 3           bit 0  0x0066 with the button, delayed
//
// The NMI button is not modelled, so 0x0066 never maps. Without a Next (rom3Fn nil) the
// same registers at their reset values, with ROM 3 taken as present, give the original
// interface's entry points.
//
// Ochenta's DivMMC documents the same behaviour and pointed at it; the code here is
// written from the register reference.

const (
	divMMCEntry0Reset  = 0x83
	divMMCValid0Reset  = 0x01
	divMMCTiming0Reset = 0x00
	divMMCEntry1Reset  = 0xCD
)

const (
	divPendingNone = iota
	divPendingMap
	divPendingUnmap
)

// resetEntryPoints puts NextREG 0xB8-0xBB back to their reset values.
func (d *DivMMC) resetEntryPoints() {
	d.entry0, d.valid0, d.timing0, d.entry1 = divMMCEntry0Reset, divMMCValid0Reset, divMMCTiming0Reset, divMMCEntry1Reset
}

// setEntryReg takes a write to NextREG 0xB8-0xBB.
func (d *DivMMC) setEntryReg(reg uint8, value uint8) {
	switch reg {
	case 0xB8:
		d.entry0 = value
	case 0xB9:
		d.valid0 = value
	case 0xBA:
		d.timing0 = value
	case 0xBB:
		d.entry1 = value
	}
}

// clearMapRAM is NextREG 0x09 bit 3: the one way, short of a power cycle, to clear MAPRAM.
func (d *DivMMC) clearMapRAM() {
	d.ctrl &^= 0x40
}

func (d *DivMMC) rom3() bool {
	return d.rom3Fn == nil || d.rom3Fn()
}

// OnM1 is the CPU's M1 hook. zen80 calls it after it has read the opcode at pc and before
// the instruction does anything else, so a delayed change set here takes effect at the
// right moment: on the next memory access. An instant entry point cannot be handled here,
// because its opcode has to come from the DivMMC already; instantEntry does that from the
// memory read itself, when the CPU's program counter is available (pcFn). Without pcFn an
// instant entry point maps here, one opcode late.
func (d *DivMMC) OnM1(pc uint16, opcode uint8, context string) {
	_ = opcode
	// A delayed change left over from the last opcode, when that instruction touched no
	// memory after its fetch.
	d.applyPending()
	// zen80 also reports the acknowledge cycle of an interrupt here, as an M1 at the vector
	// (0x0038 for IM 1, 0x0066 for an NMI) with the context naming the interrupt. That is
	// not a fetch of the opcode there: the real fetch follows and is reported as "normal".
	// Taking the acknowledge as the fetch maps the DivMMC in one opcode early, so the
	// opcode at 0x0038 comes from the DivMMC and not from the ROM, and the two ROMs'
	// interrupt entries are written to be run the other way round.
	if context == "NMI" || strings.HasPrefix(context, "IM") {
		return
	}
	if d.Debug && pc == 0x0000 {
		fmt.Fprintf(os.Stderr, "[divmmc] PC=0x0000 reached (automapEnabled=%v)\n", d.automapEnabled)
	}

	action, instant := divPendingNone, false
	switch {
	case pc&0xFFC7 == 0: // 0x0000, 0x0008 ... 0x0038
		bit := uint8(1) << (pc >> 3)
		if d.entry0&bit != 0 && (d.valid0&bit != 0 || d.rom3()) {
			action, instant = divPendingMap, d.timing0&bit != 0
		}
	case pc&0xFF00 == 0x3D00:
		if d.entry1&0x80 != 0 && d.rom3() {
			action, instant = divPendingMap, true
		}
	case pc == 0x04C6:
		if d.entry1&0x04 != 0 && d.rom3() {
			action = divPendingMap
		}
	case pc == 0x0562:
		if d.entry1&0x08 != 0 && d.rom3() {
			action = divPendingMap
		}
	case pc == 0x04D7:
		if d.entry1&0x10 != 0 && d.rom3() {
			action = divPendingMap
		}
	case pc == 0x056A:
		if d.entry1&0x20 != 0 && d.rom3() {
			action = divPendingMap
		}
	case pc >= 0x1FF8 && pc <= 0x1FFF:
		if d.entry1&0x40 != 0 {
			action = divPendingUnmap
		}
	}
	if action == divPendingMap && !d.automapEnabled {
		return // NextREG 0x0A bit 4 gates mapping in; mapping out is never gated
	}
	if action == divPendingNone {
		return
	}
	if instant {
		d.setPaged(action == divPendingMap, pc)
		return
	}
	d.pending, d.fetchPC = action, pc
}

// instantEntry maps the DivMMC in when the read at address is the opcode fetch of an
// instant entry point: 0x3Dxx, or one of 0x0000 ... 0x0038 with its timing bit set. The
// test for "opcode fetch" is that the CPU's program counter is the address being read;
// data reads of the same addresses (the 48K ROM's character set is at 0x3D00) come from
// code elsewhere and do not match.
func (d *DivMMC) instantEntry(address uint16) {
	if d.pcFn == nil || d.paged || !d.automapEnabled {
		return
	}
	switch {
	case address&0xFF00 == 0x3D00:
		if d.entry1&0x80 == 0 || !d.rom3() {
			return
		}
	case address&0xFFC7 == 0:
		bit := uint8(1) << (address >> 3)
		if d.entry0&bit == 0 || d.timing0&bit == 0 || (d.valid0&bit == 0 && !d.rom3()) {
			return
		}
	default:
		return
	}
	if d.pcFn() == address {
		d.setPaged(true, address)
	}
}

// settle is called on every memory access that reaches the DivMMC. A delayed change takes
// effect on the first access after the opcode fetch that caused it.
func (d *DivMMC) settle(address uint16) {
	d.applyPending()
	d.instantEntry(address)
}

func (d *DivMMC) applyPending() {
	if d.pending == divPendingNone {
		return
	}
	d.setPaged(d.pending == divPendingMap, d.fetchPC)
	d.pending = divPendingNone
}

func (d *DivMMC) setPaged(paged bool, pc uint16) {
	if d.Debug && paged != d.paged {
		dir := "OUT"
		if paged {
			dir = "IN "
		}
		fmt.Fprintf(os.Stderr, "[divmmc] automap %s at PC=0x%04X (ctrl=0x%02X)\n", dir, pc, d.ctrl)
	}
	d.paged = paged
}
