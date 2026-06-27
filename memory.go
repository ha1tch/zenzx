package main

// ============================================================================
// Memory Constants
// ============================================================================

const (
	ROMSize = 16384 // 16KB
	RAMSize = 49152 // 48KB
)

// ============================================================================
// Memory Implementation for Z80
// ============================================================================

type SpectrumMemory struct {
	// +3 model has:
	// - 4 ROM banks (16KB each)
	// - 8 RAM banks (16KB each)
	rom    [4][16384]byte // ROM banks 0-3
	ram    [8][16384]byte // RAM banks 0-7
	screen *SpectrumScreen

	// Memory configuration
	romBank     uint8 // Currently selected ROM bank (0-3)
	ramBankLow  uint8 // RAM bank at 0x4000-0x7FFF
	ramBankHigh uint8 // RAM bank at 0x8000-0xBFFF
	ramBankTop  uint8 // RAM bank at 0xC000-0xFFFF

	// Paging control
	pagingLocked bool  // When true, no more paging allowed
	screenBank   uint8 // Which RAM bank is displayed (5 or 7)
	is128K       bool  // True for 128K mode, false for 48K mode
	isPlus3      bool  // True for +2A/+3 mode with 4 ROMs
	specialMode  uint8 // +3 special paging mode (0=normal, 1-3=special configs)
	port7FFD     uint8 // Last value written to 0x7FFD
	port1FFD     uint8 // Last value written to 0x1FFD (+3 only)
}

func NewSpectrumMemory(screen *SpectrumScreen) *SpectrumMemory {
	m := &SpectrumMemory{
		screen:      screen,
		romBank:     0, // Start with ROM 0
		ramBankLow:  5, // Default config
		ramBankHigh: 2,
		ramBankTop:  0,
		screenBank:  5, // Normal screen in bank 5
		is128K:      false,
		isPlus3:     false,
		specialMode: 0, // Normal mode
	}
	return m
}

func (m *SpectrumMemory) Enable128K() {
	m.is128K = true
}

func (m *SpectrumMemory) EnablePlus3() {
	m.is128K = true
	m.isPlus3 = true
}

func (m *SpectrumMemory) Read(address uint16) uint8 {
	if !m.is128K {
		// 48K mode - simple memory map (no banking)
		// The 48K Spectrum has continuous RAM from 0x4000-0xFFFF
		// But this emulator stores it in banks 5, 2, 0 for compatibility
		if address < ROMSize {
			return m.rom[0][address]
		}

		// Map addresses to the correct RAM banks
		// 0x4000-0x7FFF -> bank 5 (screen area)
		// 0x8000-0xBFFF -> bank 2
		// 0xC000-0xFFFF -> bank 0
		if address < 0x8000 {
			// 0x4000-0x7FFF: RAM bank 5 (contains screen memory)
			offset := address - 0x4000
			val := m.ram[5][offset]

			// Handle screen memory reading
			if address >= 0x4000 && address < 0x5800 {
				return m.screen.bitmap[address-0x4000]
			} else if address >= 0x5800 && address < 0x5B00 {
				return m.screen.attributes[address-0x5800]
			}
			return val
		} else if address < 0xC000 {
			// 0x8000-0xBFFF: RAM bank 2
			offset := address - 0x8000
			return m.ram[2][offset]
		} else {
			// 0xC000-0xFFFF: RAM bank 0
			offset := address - 0xC000
			return m.ram[0][offset]
		}
	}

	// 128K/+3 mode with banking
	// Check for special paging modes (+3 only)
	if m.isPlus3 && m.specialMode > 0 {
		return m.readSpecialMode(address)
	}

	// Normal paging mode
	switch {
	case address < 0x4000:
		// ROM area (0x0000-0x3FFF)
		return m.rom[m.romBank][address]

	case address < 0x8000:
		// RAM bank at 0x4000-0x7FFF
		offset := address - 0x4000
		val := m.ram[m.ramBankLow][offset]

		// Handle screen memory reading
		if m.ramBankLow == 5 && m.screenBank == 5 {
			if address >= 0x4000 && address < 0x5800 {
				return m.screen.bitmap[address-0x4000]
			} else if address >= 0x5800 && address < 0x5B00 {
				return m.screen.attributes[address-0x5800]
			}
		} else if m.ramBankLow == 7 && m.screenBank == 7 {
			if address >= 0x4000 && address < 0x5800 {
				return m.screen.bitmap[address-0x4000]
			} else if address >= 0x5800 && address < 0x5B00 {
				return m.screen.attributes[address-0x5800]
			}
		}
		return val

	case address < 0xC000:
		// RAM bank at 0x8000-0xBFFF
		offset := address - 0x8000
		return m.ram[m.ramBankHigh][offset]

	default:
		// RAM bank at 0xC000-0xFFFF
		offset := address - 0xC000
		val := m.ram[m.ramBankTop][offset]

		// Handle screen memory if bank 7 is paged in at top
		if m.ramBankTop == 7 && m.screenBank == 7 {
			if address >= 0xC000 && address < 0xD800 {
				return m.screen.bitmap[address-0xC000]
			} else if address >= 0xD800 && address < 0xDB00 {
				return m.screen.attributes[address-0xD800]
			}
		} else if m.ramBankTop == 5 && m.screenBank == 5 {
			if address >= 0xC000 && address < 0xD800 {
				return m.screen.bitmap[address-0xC000]
			} else if address >= 0xD800 && address < 0xDB00 {
				return m.screen.attributes[address-0xD800]
			}
		}
		return val
	}
}

func (m *SpectrumMemory) readSpecialMode(address uint16) uint8 {
	// +3 special paging modes
	config := [4][4]uint8{
		{0, 1, 2, 3}, // Mode 0: ROM0, ROM1, ROM2, ROM3
		{4, 5, 6, 7}, // Mode 1: RAM4, RAM5, RAM6, RAM7
		{4, 5, 6, 3}, // Mode 2: RAM4, RAM5, RAM6, RAM3
		{4, 7, 6, 3}, // Mode 3: RAM4, RAM7, RAM6, RAM3
	}

	bank := address / 0x4000
	offset := address % 0x4000

	if m.specialMode == 0 {
		// All ROM mode
		return m.rom[config[0][bank]][offset]
	} else {
		// RAM configurations
		ramBank := config[m.specialMode][bank]
		val := m.ram[ramBank][offset]

		// Handle screen memory in special modes
		if ramBank == m.screenBank {
			relAddr := bank*0x4000 + offset
			if relAddr >= 0x0000 && relAddr < 0x1800 {
				return m.screen.bitmap[relAddr]
			} else if relAddr >= 0x1800 && relAddr < 0x1B00 {
				return m.screen.attributes[relAddr-0x1800]
			}
		}
		return val
	}
}

func (m *SpectrumMemory) Write(address uint16, value uint8) {
	if !m.is128K {
		// 48K mode - simple memory map (no banking)
		if address < ROMSize {
			return // ROM is read-only
		}

		// Map addresses to the correct RAM banks
		// 0x4000-0x7FFF -> bank 5 (screen area)
		// 0x8000-0xBFFF -> bank 2
		// 0xC000-0xFFFF -> bank 0
		if address < 0x8000 {
			// 0x4000-0x7FFF: RAM bank 5 (contains screen memory)
			offset := address - 0x4000
			m.ram[5][offset] = value

			// Update screen if writing to display memory
			if address >= 0x4000 && address < 0x5800 {
				m.screen.bitmap[address-0x4000] = value
			} else if address >= 0x5800 && address < 0x5B00 {
				m.screen.attributes[address-0x5800] = value
			}
		} else if address < 0xC000 {
			// 0x8000-0xBFFF: RAM bank 2
			offset := address - 0x8000
			m.ram[2][offset] = value
		} else {
			// 0xC000-0xFFFF: RAM bank 0
			offset := address - 0xC000
			m.ram[0][offset] = value
		}
		return
	}

	// 128K/+3 mode
	if m.isPlus3 && m.specialMode > 0 {
		m.writeSpecialMode(address, value)
		return
	}

	// Normal mode writes
	switch {
	case address < 0x4000:
		// ROM area - read only
		return

	case address < 0x8000:
		// RAM bank at 0x4000-0x7FFF
		offset := address - 0x4000
		m.ram[m.ramBankLow][offset] = value

		// Update screen if appropriate bank
		if m.ramBankLow == m.screenBank {
			if address >= 0x4000 && address < 0x5800 {
				m.screen.bitmap[address-0x4000] = value
			} else if address >= 0x5800 && address < 0x5B00 {
				m.screen.attributes[address-0x5800] = value
			}
		}

	case address < 0xC000:
		// RAM bank at 0x8000-0xBFFF
		offset := address - 0x8000
		m.ram[m.ramBankHigh][offset] = value

	default:
		// RAM bank at 0xC000-0xFFFF
		offset := address - 0xC000
		m.ram[m.ramBankTop][offset] = value

		// Update screen if appropriate bank
		if m.ramBankTop == m.screenBank {
			if address >= 0xC000 && address < 0xD800 {
				m.screen.bitmap[address-0xC000] = value
			} else if address >= 0xD800 && address < 0xDB00 {
				m.screen.attributes[address-0xD800] = value
			}
		}
	}
}

// Load writes a block of bytes into memory starting at the given address,
// going through the normal Write path so RAM banking and the screen mirror
// are honoured and ROM regions are protected. Bytes that would extend past
// the top of the 64K address space are silently dropped; callers that need to
// detect overflow should check len(data)+int(address) <= 0x10000 first.
//
// This mirrors the Load convention used by zen80's RAM/MappedMemory types
// (address is uint16, matching the Z80's 16-bit address bus).
func (m *SpectrumMemory) Load(address uint16, data []byte) {
	for i, b := range data {
		addr := int(address) + i
		if addr > 0xFFFF {
			break
		}
		m.Write(uint16(addr), b)
	}
}

func (m *SpectrumMemory) writeSpecialMode(address uint16, value uint8) {
	// +3 special paging mode writes
	if m.specialMode == 0 {
		// All ROM mode - no writes
		return
	}

	config := [4][4]uint8{
		{0, 1, 2, 3}, // Mode 0: not used here
		{4, 5, 6, 7}, // Mode 1: RAM4, RAM5, RAM6, RAM7
		{4, 5, 6, 3}, // Mode 2: RAM4, RAM5, RAM6, RAM3
		{4, 7, 6, 3}, // Mode 3: RAM4, RAM7, RAM6, RAM3
	}

	bank := address / 0x4000
	offset := address % 0x4000
	ramBank := config[m.specialMode][bank]

	m.ram[ramBank][offset] = value

	// Update screen if writing to screen bank
	if ramBank == m.screenBank {
		relAddr := bank*0x4000 + offset
		if relAddr >= 0x0000 && relAddr < 0x1800 {
			m.screen.bitmap[relAddr] = value
		} else if relAddr >= 0x1800 && relAddr < 0x1B00 {
			m.screen.attributes[relAddr-0x1800] = value
		}
	}
}

func (m *SpectrumMemory) LoadROM(data []byte) {
	if len(data) == 16384 {
		// 48K ROM
		copy(m.rom[0][:], data)
		m.is128K = false
		m.isPlus3 = false
	} else if len(data) == 32768 {
		// 128K ROM (2 banks)
		copy(m.rom[0][:], data[0:16384])
		copy(m.rom[1][:], data[16384:32768])
		m.Enable128K()
		m.isPlus3 = false
	} else if len(data) == 65536 {
		// +3 ROM (4 banks)
		copy(m.rom[0][:], data[0:16384])
		copy(m.rom[1][:], data[16384:32768])
		copy(m.rom[2][:], data[32768:49152])
		copy(m.rom[3][:], data[49152:65536])
		m.EnablePlus3()
	}
}

// SetPaging handles the 128K memory paging via port 0x7FFD
func (m *SpectrumMemory) SetPaging(value uint8) {
	if !m.is128K || m.pagingLocked {
		return
	}

	m.port7FFD = value

	// In special mode, only screen select works
	if m.isPlus3 && m.specialMode > 0 {
		// Bit 3: Screen select still works in special modes
		newScreenBank := uint8(5)
		if value&0x08 != 0 {
			newScreenBank = 7
		}
		m.updateScreenBank(newScreenBank)
		return
	}

	// Normal mode paging
	// Bit 0-2: RAM page at 0xC000
	m.ramBankTop = value & 0x07

	// Bit 3: Screen select (0=normal in bank 5, 1=shadow in bank 7)
	newScreenBank := uint8(5)
	if value&0x08 != 0 {
		newScreenBank = 7
	}
	m.updateScreenBank(newScreenBank)

	// Bit 4: ROM select
	if m.isPlus3 {
		// +3: bit 4 selects low bit of ROM number
		m.romBank = (m.romBank & 0x02) | ((value >> 4) & 0x01)
	} else {
		// 128K: bit 4 selects between ROM 0 and 1
		if value&0x10 != 0 {
			m.romBank = 1
		} else {
			m.romBank = 0
		}
	}

	// Bit 5: Lock paging
	if value&0x20 != 0 {
		m.pagingLocked = true
	}
}

// SetPlus3Paging handles +3 specific paging via port 0x1FFD
func (m *SpectrumMemory) SetPlus3Paging(value uint8) {
	if !m.isPlus3 || m.pagingLocked {
		return
	}

	m.port1FFD = value

	// Bit 0: Paging mode (0=normal, 1=special)
	if value&0x01 != 0 {
		// Special paging mode
		// Bits 1-2: Configuration
		m.specialMode = (value >> 1) & 0x03

		// Update memory configuration based on special mode
		switch m.specialMode {
		case 0: // All ROM
			// Banks 0-3 all show ROM 0-3
		case 1: // RAM 4,5,6,7
			m.ramBankLow = 5
			m.ramBankHigh = 6
			m.ramBankTop = 7
		case 2: // RAM 4,5,6,3
			m.ramBankLow = 5
			m.ramBankHigh = 6
			m.ramBankTop = 3
		case 3: // RAM 4,7,6,3
			m.ramBankLow = 7
			m.ramBankHigh = 6
			m.ramBankTop = 3
		}

		// Set special mode flag
		if m.specialMode > 0 {
			m.specialMode++ // Make it 1-3 instead of 0-2
		}
	} else {
		// Normal paging mode
		m.specialMode = 0
		m.ramBankLow = 5
		m.ramBankHigh = 2
		// ramBankTop is controlled by port 7FFD
	}

	// Bit 2: High bit of ROM selection (with bit 4 of port 7FFD)
	if m.specialMode == 0 { // Only in normal mode
		m.romBank = ((value >> 1) & 0x02) | (m.romBank & 0x01)
	}

	// Bit 3: Disk motor control (not implemented)
	// Bit 4: Printer strobe (not implemented)
}

func (m *SpectrumMemory) updateScreenBank(newBank uint8) {
	if newBank == m.screenBank {
		return
	}

	// Copy current screen to its RAM bank
	if m.screenBank == 5 {
		copy(m.ram[5][0:6144], m.screen.bitmap[:])
		copy(m.ram[5][6144:6912], m.screen.attributes[:])
	} else {
		copy(m.ram[7][0:6144], m.screen.bitmap[:])
		copy(m.ram[7][6144:6912], m.screen.attributes[:])
	}

	// Load new screen from RAM bank
	if newBank == 5 {
		copy(m.screen.bitmap[:], m.ram[5][0:6144])
		copy(m.screen.attributes[:], m.ram[5][6144:6912])
	} else {
		copy(m.screen.bitmap[:], m.ram[7][0:6144])
		copy(m.screen.attributes[:], m.ram[7][6144:6912])
	}

	m.screenBank = newBank
}