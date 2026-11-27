package zenzx

// The hardware side of a machine reset.
//
// ZenZX.Reset (zenzx.go) is the reset button: it restarts the CPU and then calls
// resetHardware, which puts back what a reset line puts back on the real machine, model
// by model. What a reset leaves alone is as much a part of the model as what it clears:
//
//	cleared     the memory paging latches (0x7FFD, 0x1FFD, their lock and the +3 special
//	            paging), the TS2068's ports 0xF4 and 0xFF, the AY (its registers and its
//	            counters; the 128K, +2, +2A, +3, TS2068 and Next all wire its /RESET to the
//	            machine's), the +3's disk controller (the drive's motor, the command in
//	            progress; the disk stays in the drive), and on a Next a hard reset
//	            (nextconfig.go), which restarts the firmware
//	kept        RAM and its contents, the ULA's border, speaker and MIC latch (the ULA has no
//	            reset line; the ROM writes them again), a disk in the drive, a tape in the
//	            deck, the host's devices (joysticks, mice, the card in a DivMMC)
//
// Before this file a reset restarted the CPU and nothing else, so a 128K that had a program
// in ROM 1 and RAM bank 7 started again in ROM 1 (the 48K BASIC, not the 128K menu), and a
// Next started again with its firmware's settings still in place (docs/TRACKING.md T-74).

// resetHardware puts back the machine's reset state apart from the CPU, which the caller has
// restarted.
func (zx *ZenZX) resetHardware() {
	if zx.memory != nil {
		zx.memory.resetPaging()
	}
	if zx.io != nil {
		zx.io.resetHardware()
	}
	// The TS2068's video mode is in its port 0xFF; the screen mode the program chose goes with it.
	if zx.memory != nil && zx.memory.isTS2068 {
		zx.SelectVideoRenderer("")
	}
	// A Next resets as a Next does: the firmware starts again from its boot ROM
	// (serviceNextReset, between frames, where the CPU is never cut off half-way through an
	// instruction).
	if zx.memory != nil && zx.memory.isNext && zx.io != nil {
		zx.io.nextResetRequest = nextResetHard
	}
}

// resetPaging puts the memory map back as the machine has it when it is switched on: ROM 0,
// RAM bank 0 at the top, the normal screen, paging unlocked, the +3's special paging off.
// RAM keeps its contents.
func (m *SpectrumMemory) resetPaging() {
	m.pagingLocked = false
	m.specialPaging = false
	m.specialMode = 0
	m.port7FFD, m.port1FFD = 0, 0
	m.romBank = 0
	m.ramBankLow, m.ramBankHigh, m.ramBankTop = 5, 2, 0
	m.updateScreenBank(5) // what was on the shadow screen is kept in its bank
	m.ts2068HSRChunk0, m.ts2068ExRomSelect = false, false
}

// resetHardware is the reset line as the I/O side of the machine sees it.
func (io *SpectrumIO) resetHardware() {
	io.resetAY()
	io.ts2068HSR, io.ts2068Port0xFF = 0, 0
	if io.fdc != nil {
		io.fdc.resetController()
	}
}

// ResetAY puts the AY chip back as it was made.
func (aw *AudioWrapper) ResetAY() {
	if aw.backend == AudioBackendOto && aw.oto != nil && aw.oto.ay != nil {
		aw.oto.ayMutex.Lock()
		aw.oto.ay.Reset()
		aw.oto.ayMutex.Unlock()
	}
}

// Reset puts the chip back as NewAYChip made it: every register and counter cleared (the
// mixer register reads 0xFF, every channel off, as it does there), the noise generator
// seeded, the envelope at its start.
func (ay *AYChip) Reset() {
	*ay = AYChip{clockFreq: ay.clockFreq, noiseShift: 1, envelopeDir: 1}
	ay.registers[7] = 0xFF
}

// resetController is what the 765's reset input does: the command being taken, the
// result being read, the transfer in progress and the drive's motor are cleared and the
// status registers start again, as NewFDC765 has them. The disk in the drive, the file it
// came from and what has not been written back to it stay.
func (fdc *FDC765) resetController() {
	fdc.mainStatus, fdc.dataReg = 0x80, 0
	fdc.commandBuf, fdc.commandLen, fdc.commandPhase = nil, 0, 0
	fdc.resultBuf, fdc.resultIdx = nil, 0
	fdc.currentTrack, fdc.currentHead, fdc.currentSector, fdc.currentCylinder = 0, 0, 0, 0
	fdc.dataBuffer, fdc.dataIdx, fdc.dataLen = nil, 0, 0
	fdc.motorOn, fdc.driveSelect = false, 0
	fdc.st0, fdc.st1, fdc.st2, fdc.st3 = 0xC0, 0, 0, 0x38|0x20
	fdc.resetCount, fdc.seekEnd, fdc.readIdIndex = 4, false, 0
	fdc.currentR, fdc.eotSector = 0, 0
	fdc.fmtSizeCode, fdc.fmtNumSec, fdc.fmtFiller, fdc.fmtCollected = 0, 0, 0, 0
}

// resetAY puts the AY back, the chip and the copy of its registers the snapshots take.
func (io *SpectrumIO) resetAY() {
	io.ayRegister = 0
	io.ayRegisters = [16]uint8{}
	if io.audio != nil {
		io.audio.ResetAY()
	}
}

// resetNextHard is what a Next's hard reset does to everything that is not memory: every
// NextREG goes back to its power-on value, and the peripherals they drive start again as
// at power on, the AY, the Timex port, the clip windows, the tilemap's, the sprites (their
// attributes and patterns), the palettes, the copper and the DivMMC's automap. serviceNextReset
// (nextconfig.go) calls it first, and then puts back what a reset puts back in its own way.
// The firmware, which runs again from the boot ROM, sets what it wants from there.
//
// The Next's RAM keeps its contents, as it does on the hardware.
func (io *SpectrumIO) resetNextHard() {
	io.resetAY()
	*io.nextRegs = *newNextRegisterFile()
	*io.nextRegClipWin = *newNextRegClip()
	*io.nextClipSprites = *newNextRegClip()
	*io.nextClipULA = *newNextRegClip()
	io.tilemap = newTilemapState()
	// What a renderer has cached from the sprites' patterns and the palettes is told that it
	// has changed: the counters only ever go up.
	generation := io.sprites.patternGeneration
	*io.sprites = *newSpriteSystem()
	for i := range generation {
		io.sprites.patternGeneration[i] = generation[i] + 1
	}
	*io.nextPalette = *newNextPaletteSystem()
	io.nextPaletteVersion++
	io.nextTimexPort = 0
	io.copper = nextCopper{}
	if io.divMMC != nil {
		io.divMMC.SetAutomapEnabled(false) // NextREG 0x0A is back at 0x01: bit 4, automap, clear
	}
}
