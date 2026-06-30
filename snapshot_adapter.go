// file: snapshot_adapter.go
//
// Adapter between ZenZX's live emulator state and zentools' neutral
// snapshot.MachineState. The format codecs in zentools/pkg/snapshot operate on
// MachineState and never see ZenZX's types; this file is the only place that
// knows both sides. ZenZX keeps its proprietary .zxs format and all
// emulator-coupled playback; the portable .sna/.z80 codecs come from zentools.

package main

import (
	"github.com/ha1tch/zentools/pkg/snapshot"
)

// toMachineState captures the current emulator state into a neutral
// MachineState suitable for handing to a zentools snapshot encoder.
func (zx *ZenZX) toMachineState() *snapshot.MachineState {
	s := &snapshot.MachineState{}

	// Model.
	switch {
	case zx.isPlus3:
		s.Model = snapshot.ModelPlus3
	case zx.is128K:
		s.Model = snapshot.Model128K
	default:
		s.Model = snapshot.Model48K
	}

	// CPU registers. zen80 exposes 16-bit accessors and individual shadow bytes.
	s.CPU.AF = zx.cpu.AF()
	s.CPU.BC = zx.cpu.BC()
	s.CPU.DE = zx.cpu.DE()
	s.CPU.HL = zx.cpu.HL()
	s.CPU.AF_ = uint16(zx.cpu.A_)<<8 | uint16(zx.cpu.F_)
	s.CPU.BC_ = uint16(zx.cpu.B_)<<8 | uint16(zx.cpu.C_)
	s.CPU.DE_ = uint16(zx.cpu.D_)<<8 | uint16(zx.cpu.E_)
	s.CPU.HL_ = uint16(zx.cpu.H_)<<8 | uint16(zx.cpu.L_)
	s.CPU.IX = zx.cpu.IX()
	s.CPU.IY = zx.cpu.IY()
	s.CPU.SP = zx.cpu.SP
	s.CPU.PC = zx.cpu.PC
	s.CPU.I = zx.cpu.I
	s.CPU.R = zx.cpu.R
	s.CPU.IFF1 = zx.cpu.IFF1
	s.CPU.IFF2 = zx.cpu.IFF2
	s.CPU.IM = zx.cpu.IM

	// Memory: copy all eight RAM banks verbatim. zentools decides which banks a
	// given format writes (48K uses 5/2/0; 128K writes all eight).
	for b := 0; b < 8; b++ {
		s.Memory.RAM[b] = zx.memory.ram[b]
	}

	// The display buffers (zx.screen.bitmap/attributes) are the authoritative
	// copy of the screen: memory writes update them, and some paths (e.g. loading
	// a .scr screenshot directly) populate them without going through the RAM
	// write path, leaving ram[bank]'s screen region stale. Copy the buffers into
	// the displayed bank's screen region so the snapshot always captures what is
	// actually on screen. Mirror of resyncAfterLoad, which copies the other way.
	bank := zx.memory.screenBank
	if bank != 5 && bank != 7 {
		bank = 5
	}
	copy(s.Memory.RAM[bank][0:6144], zx.screen.bitmap)
	copy(s.Memory.RAM[bank][6144:6912], zx.screen.attributes)

	// Paging and IO.
	s.Paging.Port7FFD = zx.memory.port7FFD
	s.Paging.Port1FFD = zx.memory.port1FFD
	s.Paging.Locked = zx.memory.pagingLocked
	s.IO.Border = zx.io.borderColor

	return s
}

// fromMachineState restores emulator state from a neutral MachineState produced
// by a zentools snapshot decoder.
func (zx *ZenZX) fromMachineState(s *snapshot.MachineState) {
	// CPU registers.
	zx.cpu.SetAF(s.CPU.AF)
	zx.cpu.SetBC(s.CPU.BC)
	zx.cpu.SetDE(s.CPU.DE)
	zx.cpu.SetHL(s.CPU.HL)
	zx.cpu.A_ = uint8(s.CPU.AF_ >> 8)
	zx.cpu.F_ = uint8(s.CPU.AF_)
	zx.cpu.B_ = uint8(s.CPU.BC_ >> 8)
	zx.cpu.C_ = uint8(s.CPU.BC_)
	zx.cpu.D_ = uint8(s.CPU.DE_ >> 8)
	zx.cpu.E_ = uint8(s.CPU.DE_)
	zx.cpu.H_ = uint8(s.CPU.HL_ >> 8)
	zx.cpu.L_ = uint8(s.CPU.HL_)
	zx.cpu.SetIX(s.CPU.IX)
	zx.cpu.SetIY(s.CPU.IY)
	zx.cpu.SP = s.CPU.SP
	zx.cpu.PC = s.CPU.PC
	zx.cpu.I = s.CPU.I
	zx.cpu.R = s.CPU.R
	zx.cpu.IFF1 = s.CPU.IFF1
	zx.cpu.IFF2 = s.CPU.IFF2
	zx.cpu.IM = s.CPU.IM

	// Memory.
	for b := 0; b < 8; b++ {
		zx.memory.ram[b] = s.Memory.RAM[b]
	}

	// Paging and IO.
	zx.memory.port7FFD = s.Paging.Port7FFD
	zx.memory.port1FFD = s.Paging.Port1FFD
	zx.memory.pagingLocked = s.Paging.Locked
	zx.io.borderColor = s.IO.Border
}

// resyncAfterLoad re-applies derived emulator state after a raw snapshot load
// has written registers and RAM directly. Two things need refreshing because
// the load bypasses the normal write paths:
//
//  1. 128K paging: the loaded port 0x7FFD value must be applied so the RAM bank
//     at 0xC000, the screen bank, and ROM selection reconfigure to match it.
//  2. Screen buffers: the renderer reads zx.screen.bitmap/attributes, which are
//     populated by memory writes during normal running. A direct RAM load
//     leaves them stale, so the displayed bank's screen region is copied in.
func (zx *ZenZX) resyncAfterLoad() {
	// Re-apply 128K paging from the loaded port value. SetPaging is a no-op on
	// 48K or when paging is locked, so guard the lock: a snapshot may legitimately
	// carry a locked-paging state, but the bank layout must be established first.
	if zx.memory.is128K {
		wasLocked := zx.memory.pagingLocked
		zx.memory.pagingLocked = false
		zx.memory.SetPaging(zx.memory.port7FFD)
		zx.memory.pagingLocked = wasLocked
	}

	// Refresh the screen buffers from whichever bank is currently displayed.
	bank := zx.memory.screenBank
	if bank != 5 && bank != 7 {
		bank = 5
	}
	copy(zx.screen.bitmap[:], zx.memory.ram[bank][0:6144])
	copy(zx.screen.attributes[:], zx.memory.ram[bank][6144:6912])
}
