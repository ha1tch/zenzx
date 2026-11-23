// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "github.com/ha1tch/zenzx/pkg/romimage"

// The Next's "ROM" for -rom-bin: the first 256 KiB of the board's SRAM, sixteen banks of 16 KiB
// (nextphys.go), reached by physical address so that every bank has the one way in. Banks 0-3
// are NextZXOS's four ROMs, 4 the DivMMC's ROM, 5 the Multiface, 6 and 7 the alternate ROMs;
// banks 8-15 are the DivMMC's RAM and are not ROM, so -rom-bank stops at 7 (T-76).
const (
	nextROMBinBanks = 8
	nextROMSpace    = 16 * romimage.Size // the 256 KiB below the RAM
)

// nextROMRegion is n bytes of the Next's SRAM from the physical address start.
func (zx *ZenZX) nextROMRegion(start, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = zx.memory.nextPhysRead(uint32(start + i))
	}
	return out
}

// writeNextROMRegion puts data in the Next's SRAM at the physical address start.
func (zx *ZenZX) writeNextROMRegion(start int, data []byte) {
	for i, b := range data {
		zx.memory.nextPhysWrite(uint32(start+i), b)
	}
}
