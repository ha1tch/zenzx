package zenzx

import "fmt"

// The Next's memory by its physical address.
//
// The SRAM has one address space, which the MMU, the DivMMC and the ROM selection all look
// into from the CPU's 64K. DMA+ (nextdma.go) can be given an address in it directly, 22
// bits, and that is the only way a program reaches all of the memory of the newest boards:
//
//	0x000000-0x03FFFF  sixteen 16K banks: the ROMs, the DivMMC's ROM and RAM, the
//	                   Multiface (the layout of nextconfig.go's config window)
//	0x040000-0x1FFFFF  the RAM the MMU pages: 16K bank n at 0x40000 + n*0x4000, 8K page n
//	                   at 0x40000 + n*0x2000, pages 0 to 223
//	0x200000-0x3FFFFF  two megabytes more, on an issue 5 board only, with no page numbers:
//	                   the MMU's are eight bits and are used up
//
// How much is there is the board's (the guide's 2.5, and CSpect's -issue option): issue 5
// has 4 MB; issue 4 has 2 MB and drops address bit 21, so higher addresses wrap; an issue 2
// with its expansion is the same 2 MB. ZenZX is an issue 5 unless told otherwise.
//
// Banks 5 and 7 are not in the SRAM on the machine: they are in the FPGA's block RAM, for
// the ULA. A physical write to where they would be goes to SRAM nothing else reads, and
// does not reach the screen; that SRAM is kept here on its own so that what is written
// there reads back.

const (
	nextPhysRAMBase = 0x040000 // where the MMU's pages start
	nextPhysHigh    = 0x200000 // where an issue 5's extra memory starts
	nextPhysSize    = 0x400000
)

// nextPhysical is the part of the SRAM that only a physical address reaches.
type nextPhysical struct {
	issue  int              // 2, 4 or 5; 0 is 5
	high   []byte           // 0x200000-0x3FFFFF, made when first written
	shadow *[4][0x2000]byte // the SRAM under banks 5 and 7: pages 10, 11, 14, 15
}

// SetNextIssue chooses the board issue, which is how much SRAM a physical address reaches.
func (m *SpectrumMemory) SetNextIssue(issue int) error {
	switch issue {
	case 2, 4, 5:
		m.nextPhys.issue = issue
		return nil
	}
	return fmt.Errorf("-next-issue: %d is not a board issue ZenZX knows: use 2, 4 or 5", issue)
}

// NextIssue returns the board issue: 2, 4 or 5.
func (m *SpectrumMemory) NextIssue() int {
	if m.nextPhys.issue == 0 {
		return 5
	}
	return m.nextPhys.issue
}

// nextPhysAddr takes an address as the DMA puts it out to the one the board decodes.
func (m *SpectrumMemory) nextPhysAddr(addr uint32) uint32 {
	if m.NextIssue() == 5 {
		return addr & (nextPhysSize - 1)
	}
	return addr & (nextPhysHigh - 1)
}

// nextShadowPage returns which of the four pages under banks 5 and 7 a page is, -1 for
// any other.
func nextShadowPage(page uint32) int {
	switch page {
	case 10, 11:
		return int(page - 10)
	case 14, 15:
		return int(page - 12)
	}
	return -1
}

func (m *SpectrumMemory) nextPhysRead(addr uint32) uint8 {
	addr = m.nextPhysAddr(addr)
	switch {
	case addr >= nextPhysHigh:
		if m.nextPhys.high == nil {
			return 0
		}
		return m.nextPhys.high[addr-nextPhysHigh]
	case addr >= nextPhysRAMBase:
		page, offset := (addr-nextPhysRAMBase)>>13, uint16(addr&0x1FFF)
		if i := nextShadowPage(page); i >= 0 {
			if m.nextPhys.shadow == nil {
				return 0
			}
			return m.nextPhys.shadow[i][offset]
		}
		return m.nextPageRead(uint8(page), offset)
	}
	// The sixteen banks below the RAM, as the config window shows them.
	bank, offset := uint8(addr>>14), uint16(addr&0x3FFF)
	switch {
	case bank < nextCfgBankDivMMCROM:
		return m.rom[bank][offset]
	case bank == nextCfgBankDivMMCROM && offset < 0x2000 && m.nextCfg.divROM != nil:
		return m.nextCfg.divROM[offset]
	case bank < nextCfgBankDivMMCRAM:
		if m.nextCfg.scratch == nil {
			return 0
		}
		return m.nextCfg.scratch[bank-nextCfgBankDivMMCROM][offset]
	}
	if m.nextCfg.divRAM == nil {
		return 0xFF
	}
	return m.nextCfg.divRAM[(bank-nextCfgBankDivMMCRAM)*2+uint8(offset>>13)][offset&0x1FFF]
}

func (m *SpectrumMemory) nextPhysWrite(addr uint32, value uint8) {
	addr = m.nextPhysAddr(addr)
	switch {
	case addr >= nextPhysHigh:
		if m.nextPhys.high == nil {
			m.nextPhys.high = make([]byte, nextPhysSize-nextPhysHigh)
		}
		m.nextPhys.high[addr-nextPhysHigh] = value
		return
	case addr >= nextPhysRAMBase:
		page, offset := (addr-nextPhysRAMBase)>>13, uint16(addr&0x1FFF)
		if i := nextShadowPage(page); i >= 0 {
			if m.nextPhys.shadow == nil {
				m.nextPhys.shadow = new([4][0x2000]byte)
			}
			m.nextPhys.shadow[i][offset] = value
			return
		}
		m.nextPageWrite(uint8(page), offset, value)
		return
	}
	bank, offset := uint8(addr>>14), uint16(addr&0x3FFF)
	switch {
	case bank < nextCfgBankDivMMCROM:
		m.rom[bank][offset] = value
	case bank == nextCfgBankDivMMCROM && offset < 0x2000 && m.nextCfg.divROM != nil:
		m.nextCfg.divROM[offset] = value
	case bank < nextCfgBankDivMMCRAM:
		m.nextConfigScratch(bank)[offset] = value
	case m.nextCfg.divRAM != nil:
		m.nextCfg.divRAM[(bank-nextCfgBankDivMMCRAM)*2+uint8(offset>>13)][offset&0x1FFF] = value
	}
}
