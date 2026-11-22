package zenzx

// NextREG 0x8E: the classic paging ports in one register (T-36).
//
// NextZXOS changes ROM through this register rather than through ports 0x7FFD and 0x1FFD:
// its inter-ROM call (RST 20h) writes 0x8E and expects the very next opcode fetch to come
// from the other ROM. A register that only stores the value leaves the old ROM mapped, and
// the call returns into the wrong code.
//
// Bits, from the Next's register documentation:
//
//	7    port 0xDFFD bit 0
//	6-4  port 0x7FFD bits 2-0 (RAM bank)
//	3    1 = take bits 7 and 6-4 (change the RAM bank); 0 = leave the RAM bank alone
//	2    port 0x1FFD bit 0 (paging mode: 0 normal, 1 all-RAM)
//	1    port 0x1FFD bit 2 (ROM select, high bit)
//	0    normal mode: port 0x7FFD bit 4 (ROM select, low bit); all-RAM mode: port 0x1FFD bit 1
//
// The write behaves as if the ports had been written, and takes effect at once.

func (io *SpectrumIO) nextReg8EWrite(v uint8) {
	m := io.memory
	if m == nil || !m.isNext {
		return
	}
	p7, p1, pd := m.port7FFD, m.port1FFD, m.nextLegacyPortDFFD
	changeRAM := v&0x08 != 0
	if changeRAM {
		p7 = p7&^0x07 | (v>>4)&0x07
		pd = pd&^0x01 | v>>7
	}
	p1 = p1&^0x01 | (v>>2)&0x01
	if v&0x04 == 0 {
		p1 = p1&^0x04 | (v&0x02)<<1
		p7 = p7&^0x10 | (v&0x01)<<4
	} else {
		p1 = p1&^0x06 | (v&0x02)<<1 | (v&0x01)<<1
	}
	// The register is not held by the 0x7FFD lock bit; the lock itself is left as it was.
	locked := m.pagingLocked
	m.pagingLocked = false
	m.SetPaging(p7 &^ 0x20)
	m.port7FFD = p7
	m.SetPlus3Paging(p1)
	m.pagingLocked = locked
	if changeRAM {
		m.nextLegacyPortDFFD = pd
		m.SetNextLegacyBank7FFD(p7)
	}
}

func (io *SpectrumIO) nextReg8ERead() uint8 {
	m := io.memory
	if m == nil {
		return 0
	}
	v := (m.nextLegacyPortDFFD&0x01)<<7 | (m.port7FFD&0x07)<<4 | 0x08 | (m.port1FFD&0x01)<<2
	if m.port1FFD&0x01 == 0 {
		v |= (m.port1FFD&0x04)>>1 | (m.port7FFD&0x10)>>4
	} else {
		v |= (m.port1FFD&0x04)>>1 | (m.port1FFD&0x02)>>1
	}
	return v
}
