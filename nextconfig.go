package zenzx

// Next config mode: how the firmware puts the machine's ROMs in place (T-36).
//
// On the Next the "ROM" is not a ROM. The first 256K of the board's SRAM hold what a classic
// machine has in ROM chips, and the firmware (TBBLUE.FW, started by the boot ROM) fills them
// from files on the SD card before it starts the machine. It does so in config mode, the
// state the machine is in at power-on:
//
//   - NextREG 0x04 names a 16K SRAM bank, and while the machine is in config mode that bank
//     is what the CPU sees at 0x0000-0x3FFF, readable and writable. The firmware reads each
//     ROM file straight to address 0 with the destination bank selected.
//   - NextREG 0x03, bits 2-0: 111 enters config mode; 001 to 100 chooses the machine (48K,
//     128K, +3, Pentagon) and leaves config mode, and is accepted only in config mode; 000
//     changes nothing. Any write turns the boot ROM off (nextreg.go).
//   - NextREG 0x02, bit 0: soft reset. The CPU and the paging start again and memory keeps
//     its contents, so the machine starts from the ROM the firmware has just loaded. Bit 1
//     is the hard reset, which reloads the FPGA: boot ROM on, config mode again.
//
// SRAM banks, 16K each, as the firmware's own hardware.h (shipped in the distribution's
// /src/firmware) lays them out:
//
//	0x00-0x03  the machine's ROM, four banks
//	0x04       DivMMC ROM, lower 8K
//	0x05       Multiface ROM (lower 8K) and RAM (upper 8K)
//	0x06-0x07  alternate ROM 0 and 1
//	0x08-0x0F  DivMMC RAM, sixteen 8K pages
//	0x10-      RAM: bank 0x10 + n is the pair of 8K MMU pages 2n and 2n+1
//
// Sources: the Next's register documentation, the firmware source on the distribution card
// (boot.c: loadFile, load_roms and the hand-over at the end of its main), and the developer
// guides of jnext and zx_go, which describe the same mechanism. No code was taken from
// either emulator.

const (
	nextCfgBankROM       = 0x00
	nextCfgBankDivMMCROM = 0x04
	nextCfgBankScratch   = 0x05 // 0x05-0x07: Multiface and alternate ROMs, stored but not yet used
	nextCfgBankDivMMCRAM = 0x08
	nextCfgBankRAM       = 0x10

	nextResetSoft = 1
	nextResetHard = 2
)

// nextConfigState is the part of SpectrumMemory that exists only for config mode.
type nextConfigState struct {
	active bool  // in config mode
	bank   uint8 // NextREG 0x04
	// machineType is the last machine chosen through NextREG 0x03 (1 48K, 2 128K, 3 +3,
	// 4 Pentagon); 0 until the firmware chooses one.
	machineType uint8
	// scratch holds banks 0x05-0x07 and the unused upper half of 0x04, allocated on first
	// use.
	scratch *[4][16384]byte
	// DivMMC storage, wired by SpectrumIO.EnableDivMMC; nil without a DivMMC.
	divROM *[8192]byte
	divRAM *[16][8192]byte
}

// nextConfigMapped reports whether the config window answers this address.
func (m *SpectrumMemory) nextConfigMapped(address uint16) bool {
	return m.isNext && m.nextCfg.active && address < 0x4000
}

func (m *SpectrumMemory) nextConfigScratch(bank uint8) *[16384]byte {
	if m.nextCfg.scratch == nil {
		m.nextCfg.scratch = new([4][16384]byte)
	}
	return &m.nextCfg.scratch[bank-nextCfgBankDivMMCROM]
}

func (m *SpectrumMemory) nextConfigRead(address uint16) uint8 {
	bank := m.nextCfg.bank
	switch {
	case bank < nextCfgBankDivMMCROM:
		return m.rom[bank][address]
	case bank == nextCfgBankDivMMCROM && address < 0x2000 && m.nextCfg.divROM != nil:
		return m.nextCfg.divROM[address]
	case bank < nextCfgBankDivMMCRAM:
		return m.nextConfigScratch(bank)[address]
	case bank < nextCfgBankRAM:
		if m.nextCfg.divRAM == nil {
			return 0xFF
		}
		return m.nextCfg.divRAM[(bank-nextCfgBankDivMMCRAM)*2+uint8(address>>13)][address&0x1FFF]
	}
	page := int(bank-nextCfgBankRAM)*2 + int(address>>13)
	if page >= nextTotalPages {
		return 0xFF
	}
	return m.nextPageRead(uint8(page), address&0x1FFF)
}

func (m *SpectrumMemory) nextConfigWrite(address uint16, value uint8) {
	bank := m.nextCfg.bank
	switch {
	case bank < nextCfgBankDivMMCROM:
		m.rom[bank][address] = value
		return
	case bank == nextCfgBankDivMMCROM && address < 0x2000 && m.nextCfg.divROM != nil:
		m.nextCfg.divROM[address] = value
		return
	case bank < nextCfgBankDivMMCRAM:
		m.nextConfigScratch(bank)[address] = value
		return
	case bank < nextCfgBankRAM:
		if m.nextCfg.divRAM != nil {
			m.nextCfg.divRAM[(bank-nextCfgBankDivMMCRAM)*2+uint8(address>>13)][address&0x1FFF] = value
		}
		return
	}
	page := int(bank-nextCfgBankRAM)*2 + int(address>>13)
	if page >= nextTotalPages {
		return
	}
	offset := address & 0x1FFF
	m.nextPageWrite(uint8(page), offset, value)
	m.nextMirrorScreenWrite(uint8(page), offset, value)
}

// nextMirrorScreenWrite keeps the renderer's cached screen in step with a write to the
// lower half of the bank being displayed: page 10 for bank 5, page 14 for bank 7 (port
// 0x7FFD bit 3). A write to the other bank must not reach the cache.
func (m *SpectrumMemory) nextMirrorScreenWrite(page uint8, offset uint16, value uint8) {
	if page != m.screenBank*2 {
		return
	}
	if offset < 0x1800 {
		m.screen.bitmap[offset] = value
	} else if offset < 0x1B00 {
		m.screen.attributes[offset-0x1800] = value
	}
}

// nextConfigRegWrite takes the writes to NextREG 0x02, 0x03 and 0x04 that concern config
// mode. It is called for every write to those registers, from both write paths, before the
// register file stores the value.
func (io *SpectrumIO) nextConfigRegWrite(reg uint8, value uint8) {
	m := io.memory
	if m == nil || !m.isNext {
		return
	}
	debug := io.hasDivMMC && io.divMMC != nil && io.divMMC.Debug
	switch reg {
	case 0x02:
		switch {
		case value&0x02 != 0:
			io.nextResetRequest = nextResetHard
		case value&0x01 != 0:
			io.nextResetRequest = nextResetSoft
		case value&0x08 != 0:
			io.mfRequestNMI() // nextmultiface.go
		}
		if debug && io.nextResetRequest != 0 {
			sayf("[nextreg] 0x02 <- 0x%02X (reset requested: %d)\n", value, io.nextResetRequest)
		}
	case 0x03:
		switch t := value & 0x07; {
		case t == 0x07:
			m.nextCfg.active = true
		case t >= 1 && t <= 4 && m.nextCfg.active:
			m.nextCfg.machineType = t
			m.nextCfg.active = false
			if t != 3 {
				sayf("Next: the firmware chose machine type %d; only type 3 (+3, the Next's own) is modelled, continuing as +3\n", t)
			}
		}
		if debug {
			sayf("[nextreg] 0x03 <- 0x%02X (config mode %v, machine type %d)\n", value, m.nextCfg.active, m.nextCfg.machineType)
		}
	case 0x04:
		m.nextCfg.bank = value & 0x7F
	}
}

// wireNextConfigDivMMC lets the config window reach the DivMMC's ROM and RAM.
func (io *SpectrumIO) wireNextConfigDivMMC() {
	if io.memory == nil || io.divMMC == nil {
		return
	}
	io.memory.nextCfg.divROM = &io.divMMC.bootROM
	io.memory.nextCfg.divRAM = &io.divMMC.sram
	if io.memory.isNext {
		m := io.memory
		// ROM 3, the 48K BASIC, is at 0x0000: what most automap entry points need.
		io.divMMC.rom3Fn = func() bool { return m.nextSlots[0] == 0xFF && m.nextROMIndex() == 3 }
		io.divMMC.controlReadable = true
		if io.nextRegs.regs[0x02] == 0 {
			io.nextRegs.regs[0x02] = nextResetHard // power on counts as a hard reset
		}
		io.nextRegs.regs[0xB8], io.nextRegs.regs[0xB9] = divMMCEntry0Reset, divMMCValid0Reset
		io.nextRegs.regs[0xBA], io.nextRegs.regs[0xBB] = divMMCTiming0Reset, divMMCEntry1Reset
	}
}

// nextMMUResetDefaults are NextREG 0x50-0x57 after a reset: ROM in the bottom 16K, then
// banks 5, 2 and 0.
var nextMMUResetDefaults = [8]uint8{0xFF, 0xFF, 0x0A, 0x0B, 0x04, 0x05, 0x00, 0x01}

// serviceNextReset carries out a reset the guest asked for through NextREG 0x02. It runs
// between frames: the firmware spins after asking, so nothing is lost by waiting for the
// frame to end, and the CPU is never reset half-way through an instruction.
func (zx *ZenZX) serviceNextReset() {
	kind := zx.io.nextResetRequest
	if kind == 0 {
		return
	}
	zx.io.nextResetRequest = 0
	m := zx.memory

	// Paging starts again; memory keeps its contents.
	m.pagingLocked = false
	m.SetNextLegacyBank7FFD(0)
	m.port1FFD = 0
	m.romBank = 0
	for slot, page := range nextMMUResetDefaults {
		zx.io.SetNextRegister(uint8(0x50+slot), page)
	}
	zx.io.layer2Access = layer2AccessPort{}
	zx.io.copperStop() // its program stays in memory
	zx.io.nextSpeed = nextCPUSpeed{}
	zx.io.nextRegs.regs[0x07] = 0
	zx.io.nextLineInt = nextLineInt{}
	zx.io.dma.reset()
	zx.io.nextInt = nextInterrupts{}
	zx.io.mfReset()
	for _, r := range []uint8{0xC0, 0xC4, 0xC5, 0xC8, 0xC9} {
		zx.io.nextRegs.regs[r] = 0
	}
	zx.io.nextRegs.regs[0x22], zx.io.nextRegs.regs[0x23] = 0, 0
	if zx.io.divMMC != nil {
		zx.io.divMMC.softReset()
		zx.io.wireNextConfigDivMMC() // NextREG 0xB8-0xBB read back their reset values again
	}
	// NextREG 0x8C: a soft reset takes bits 3-0 into bits 7-4; a hard reset clears it.
	alt := m.nextAltROM&0x0F | m.nextAltROM&0x0F<<4
	if kind == nextResetHard {
		alt = 0
	}
	m.nextAltROM = alt
	zx.io.nextRegs.regs[0x8C] = alt
	// NextREG 0x02 reads back the kind of the last reset: bit 1 hard, bit 0 soft.
	zx.io.nextRegs.regs[0x02] = uint8(kind)
	if kind == nextResetHard {
		m.nextCfg.active = true
		m.nextCfg.bank = 0
		if m.bootROMLoaded {
			m.bootROMEnabled = true
		}
	}
	zx.cpu.Reset()
	if zx.io.hasDivMMC && zx.io.divMMC != nil && zx.io.divMMC.Debug {
		sayf("[next] reset %d done: config mode %v, boot ROM %v, machine type %d\n",
			kind, m.nextCfg.active, m.bootROMEnabled, m.nextCfg.machineType)
	}
}

// softReset is what a machine reset does to the DivMMC: the automap latch and the control
// register clear, except MAPRAM, which only a power cycle clears; the RAM keeps its
// contents.
func (d *DivMMC) softReset() {
	d.paged = false
	d.pending = divPendingNone
	d.ctrl &= 0x40
	d.resetEntryPoints()
}
