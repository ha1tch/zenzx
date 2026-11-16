package main

// NextREG 0x8C: the alternate ROM and the ROM lock (T-36).
//
// Two more 16K ROM images live in SRAM banks 6 and 7: alternate ROM 0, which stands in for
// the 128K ROM, and alternate ROM 1, for the 48K ROM. NextZXOS fills them through the ROM's
// own addresses and then runs from them. From the Next's registers.txt:
//
//	bit 7  alternate ROM enabled
//	bit 6  1: seen only by writes (reads still come from the ROM); 0: replaces the ROM for
//	       reads
//	bit 5  lock ROM 1 (48K)       bit 4  lock ROM 0 (128K)
//	bits 3-0  the same four, taken into bits 7-4 at a soft reset
//
// On the +3 and the Next the two lock bits, when not both zero, are the number of the ROM
// held in place whatever ports 0x7FFD and 0x1FFD say; this holds with the alternate ROM
// off too. Which alternate ROM is used follows the lock (bit 5: 1, bit 4: 0) and otherwise
// port 0x7FFD bit 4.

// nextROMIndex is the ROM at 0x0000-0x3FFF: the locked one, or the one the ports choose.
func (m *SpectrumMemory) nextROMIndex() uint8 {
	if lock := (m.nextAltROM >> 4) & 0x03; lock != 0 {
		return lock
	}
	return m.romBank
}

func (m *SpectrumMemory) nextAltROMBank() *[16384]byte {
	index := (m.port7FFD >> 4) & 0x01
	switch {
	case m.nextAltROM&0x20 != 0:
		index = 1
	case m.nextAltROM&0x10 != 0:
		index = 0
	}
	return m.nextConfigScratch(0x06 + index)
}

// nextROMRead reads the ROM area of a Next: the alternate ROM when it replaces the ROM for
// reads, the ROM otherwise.
func (m *SpectrumMemory) nextROMRead(address uint16) uint8 {
	if m.nextAltROM&0xC0 == 0x80 {
		return m.nextAltROMBank()[address]
	}
	return m.rom[m.nextROMIndex()][address]
}

// nextROMWrite takes a write to the ROM area: it reaches the alternate ROM when that is
// enabled for writes, and is dropped otherwise.
func (m *SpectrumMemory) nextROMWrite(address uint16, value uint8) {
	if m.nextAltROM&0xC0 == 0xC0 {
		m.nextAltROMBank()[address] = value
	}
}
