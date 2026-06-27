package main

import (
	"bytes"
	//	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// ============================================================================
// SNA Format Support (48K and 128K)
// ============================================================================

// SNA48Header represents the header for 48K .sna files
type SNA48Header struct {
	I                  uint8
	HL_, DE_, BC_, AF_ uint16
	HL, DE, BC, IY, IX uint16
	IFF2               uint8 // Bit 2 contains IFF2 (0=DI, 1=EI)
	R                  uint8
	AF, SP             uint16
	IntMode            uint8
	BorderColor        uint8
}

// SNA128Extension represents the additional data for 128K .sna files
type SNA128Extension struct {
	PC          uint16
	Port7FFD    uint8
	TRDROMPaged uint8          // 1 if TR-DOS ROM is paged
	RAMBanks    [5][16384]byte // Banks 0,1,3,4,6 (2,5,7 are in main file)
}

// SaveSNA saves the current state in .sna format
func (zx *ZenZX) SaveSNA(filename string) error {
	// Ensure .sna extension
	if !strings.HasSuffix(filename, ".sna") {
		filename += ".sna"
	}

	// Pause during save
	wasPaused := zx.paused
	zx.paused = true
	defer func() { zx.paused = wasPaused }()

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating SNA file: %w", err)
	}
	defer file.Close()

	// Create SNA header
	header := SNA48Header{
		I:           zx.cpu.I,
		HL_:         uint16(zx.cpu.H_)<<8 | uint16(zx.cpu.L_),
		DE_:         uint16(zx.cpu.D_)<<8 | uint16(zx.cpu.E_),
		BC_:         uint16(zx.cpu.B_)<<8 | uint16(zx.cpu.C_),
		AF_:         uint16(zx.cpu.A_)<<8 | uint16(zx.cpu.F_),
		HL:          zx.cpu.HL(),
		DE:          zx.cpu.DE(),
		BC:          zx.cpu.BC(),
		IY:          zx.cpu.IY(),
		IX:          zx.cpu.IX(),
		IFF2:        0,
		R:           zx.cpu.R,
		AF:          zx.cpu.AF(),
		SP:          zx.cpu.SP,
		IntMode:     zx.cpu.IM,
		BorderColor: zx.io.borderColor,
	}

	// Set IFF2 bit
	if zx.cpu.IFF2 {
		header.IFF2 = 0x04
	}

	// Write header
	if err := binary.Write(file, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("writing SNA header: %w", err)
	}

	if !zx.is128K {
		// 48K snapshot - write 48KB of RAM (banks 5,2,0)
		if _, err := file.Write(zx.memory.ram[5][:]); err != nil {
			return err
		}
		if _, err := file.Write(zx.memory.ram[2][:]); err != nil {
			return err
		}
		if _, err := file.Write(zx.memory.ram[0][:]); err != nil {
			return err
		}
	} else {
		// 128K snapshot
		// First write the 48KB that would be in a 48K snapshot
		// This is the current paging configuration
		currentTop := zx.memory.ramBankTop

		// Write 16KB from 0x4000 (always bank 5)
		if _, err := file.Write(zx.memory.ram[5][:]); err != nil {
			return err
		}
		// Write 16KB from 0x8000 (always bank 2)
		if _, err := file.Write(zx.memory.ram[2][:]); err != nil {
			return err
		}
		// Write 16KB from 0xC000 (current paged bank)
		if _, err := file.Write(zx.memory.ram[currentTop][:]); err != nil {
			return err
		}

		// Now write 128K extension
		// PC must be saved separately for 128K
		pc := zx.cpu.PC
		if err := binary.Write(file, binary.LittleEndian, pc); err != nil {
			return err
		}

		// Port 0x7FFD value
		if err := binary.Write(file, binary.LittleEndian, zx.memory.port7FFD); err != nil {
			return err
		}

		// TR-DOS ROM paged flag (always 0 for us)
		if err := binary.Write(file, binary.LittleEndian, uint8(0)); err != nil {
			return err
		}

		// Write remaining RAM banks (those not in the main 48KB)
		// We need to write all banks except 5, 2, and the currently paged one
		banksToWrite := []uint8{}
		for i := uint8(0); i < 8; i++ {
			if i != 5 && i != 2 && i != currentTop {
				banksToWrite = append(banksToWrite, i)
			}
		}

		// SNA expects exactly 5 additional banks
		for _, bank := range banksToWrite {
			if _, err := file.Write(zx.memory.ram[bank][:]); err != nil {
				return err
			}
		}
	}

	fmt.Printf("Saved SNA snapshot: %s (%s)\n", filename,
		map[bool]string{true: "128K", false: "48K"}[zx.is128K])
	return nil
}

// LoadSNA loads a .sna format snapshot
func (zx *ZenZX) LoadSNA(filename string) error {
	fmt.Printf("Loading SNA snapshot from %s\n", filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading SNA file: %w", err)
	}

	if len(data) < 27+49152 {
		return fmt.Errorf("SNA file too small")
	}

	// Pause during load
	zx.paused = true

	// Read header
	reader := bytes.NewReader(data)
	var header SNA48Header
	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("reading SNA header: %w", err)
	}

	// Restore CPU state
	zx.cpu.I = header.I
	zx.cpu.H_ = uint8(header.HL_ >> 8)
	zx.cpu.L_ = uint8(header.HL_)
	zx.cpu.D_ = uint8(header.DE_ >> 8)
	zx.cpu.E_ = uint8(header.DE_)
	zx.cpu.B_ = uint8(header.BC_ >> 8)
	zx.cpu.C_ = uint8(header.BC_)
	zx.cpu.A_ = uint8(header.AF_ >> 8)
	zx.cpu.F_ = uint8(header.AF_)
	zx.cpu.SetHL(header.HL)
	zx.cpu.SetDE(header.DE)
	zx.cpu.SetBC(header.BC)
	zx.cpu.SetIY(header.IY)
	zx.cpu.SetIX(header.IX)
	zx.cpu.R = header.R
	zx.cpu.SetAF(header.AF)
	zx.cpu.SP = header.SP
	zx.cpu.IM = header.IntMode
	zx.io.borderColor = header.BorderColor

	// Restore interrupt state
	zx.cpu.IFF1 = (header.IFF2 & 0x04) != 0
	zx.cpu.IFF2 = (header.IFF2 & 0x04) != 0

	is128K := len(data) > 27+49152

	if !is128K {
		// 48K snapshot
		// Read 48KB of RAM into banks 5,2,0
		if _, err := io.ReadFull(reader, zx.memory.ram[5][:]); err != nil {
			return err
		}
		if _, err := io.ReadFull(reader, zx.memory.ram[2][:]); err != nil {
			return err
		}
		if _, err := io.ReadFull(reader, zx.memory.ram[0][:]); err != nil {
			return err
		}

		// Set up for 48K mode
		zx.is128K = false
		zx.memory.is128K = false
		zx.memory.ramBankLow = 5
		zx.memory.ramBankHigh = 2
		zx.memory.ramBankTop = 0
		zx.memory.screenBank = 5

		// For 48K, PC is obtained from stack
		// The return address at SP gives us PC
		low := zx.memory.Read(zx.cpu.SP)
		high := zx.memory.Read(zx.cpu.SP + 1)
		zx.cpu.PC = uint16(high)<<8 | uint16(low)
		zx.cpu.SP += 2 // Adjust SP since we "popped" the return address

	} else {
		// 128K snapshot
		// Read the 48KB that's in the main part
		if _, err := io.ReadFull(reader, zx.memory.ram[5][:]); err != nil {
			return err
		}
		if _, err := io.ReadFull(reader, zx.memory.ram[2][:]); err != nil {
			return err
		}

		// This next bank is the one that was paged at 0xC000
		tempBank := make([]byte, 16384)
		if _, err := io.ReadFull(reader, tempBank); err != nil {
			return err
		}

		// Read 128K extension
		var pc uint16
		if err := binary.Read(reader, binary.LittleEndian, &pc); err != nil {
			return err
		}
		zx.cpu.PC = pc

		var port7FFD uint8
		if err := binary.Read(reader, binary.LittleEndian, &port7FFD); err != nil {
			return err
		}

		var trdROM uint8
		if err := binary.Read(reader, binary.LittleEndian, &trdROM); err != nil {
			return err
		}

		// Which bank was paged at 0xC000?
		pagedBank := port7FFD & 0x07

		// Copy the temp bank to the correct location
		copy(zx.memory.ram[pagedBank][:], tempBank)

		// Read remaining banks
		banksToRead := []uint8{}
		for i := uint8(0); i < 8; i++ {
			if i != 5 && i != 2 && i != pagedBank {
				banksToRead = append(banksToRead, i)
			}
		}

		for _, bank := range banksToRead {
			if _, err := io.ReadFull(reader, zx.memory.ram[bank][:]); err != nil {
				return err
			}
		}

		// Set up for 128K mode
		zx.is128K = true
		zx.memory.is128K = true
		zx.memory.Enable128K()

		// Restore paging from port7FFD
		zx.memory.SetPaging(port7FFD)
	}

	// Copy screen data to display
	copy(zx.screen.bitmap[:], zx.memory.ram[5][0:6144])
	copy(zx.screen.attributes[:], zx.memory.ram[5][6144:6912])

	// Clear any pending interrupts
	zx.cpu.INT = false
	zx.cpu.NMI = false
	zx.cpu.Halted = false // SNA doesn't save halt state

	fmt.Printf("Loaded SNA snapshot: %s mode, PC=%04X\n",
		map[bool]string{true: "128K", false: "48K"}[is128K], zx.cpu.PC)

	return nil
}

// ============================================================================
// Z80 Format Support (v1, v2, v3)
// ============================================================================

// Z80V1Header represents the v1 .z80 format header (30 bytes)
type Z80V1Header struct {
	A, F              uint8
	BC, HL, PC, SP    uint16
	I, R              uint8
	Flags1            uint8 // Bit 0: R bit 7, Bits 1-3: Border, Bit 4: SamROM, Bit 5: Compressed
	DE, BC_, DE_, HL_ uint16
	A_, F_            uint8
	IY, IX            uint16
	IFF1, IFF2        uint8
	Flags2            uint8 // Bits 0-1: IM mode, other bits for various features
}

// SaveZ80 saves the current state in .z80 format
func (zx *ZenZX) SaveZ80(filename string) error {
	// Ensure .z80 extension
	if !strings.HasSuffix(filename, ".z80") {
		filename += ".z80"
	}

	// Pause during save
	wasPaused := zx.paused
	zx.paused = true
	defer func() { zx.paused = wasPaused }()

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating Z80 file: %w", err)
	}
	defer file.Close()

	// For simplicity, we'll implement v1 format for 48K only
	// v2/v3 would be needed for 128K support

	if zx.is128K {
		return fmt.Errorf("Z80 format 128K support not yet implemented")
	}

	// Create v1 header
	header := Z80V1Header{
		A:   zx.cpu.A,
		F:   zx.cpu.F,
		BC:  zx.cpu.BC(),
		HL:  zx.cpu.HL(),
		PC:  zx.cpu.PC,
		SP:  zx.cpu.SP,
		I:   zx.cpu.I,
		R:   zx.cpu.R & 0x7F, // Lower 7 bits
		DE:  zx.cpu.DE(),
		BC_: uint16(zx.cpu.B_)<<8 | uint16(zx.cpu.C_),
		DE_: uint16(zx.cpu.D_)<<8 | uint16(zx.cpu.E_),
		HL_: uint16(zx.cpu.H_)<<8 | uint16(zx.cpu.L_),
		A_:  zx.cpu.A_,
		F_:  zx.cpu.F_,
		IY:  zx.cpu.IY(),
		IX:  zx.cpu.IX(),
	}

	// Set flags1: R bit 7 and border color
	header.Flags1 = (zx.cpu.R&0x80)>>7 | (zx.io.borderColor << 1) | 0x20 // Bit 5 = compressed

	// Set IFF flags
	if zx.cpu.IFF1 {
		header.IFF1 = 1
	}
	if zx.cpu.IFF2 {
		header.IFF2 = 1
	}

	// Set IM mode in flags2
	header.Flags2 = zx.cpu.IM & 0x03

	// Write header
	if err := binary.Write(file, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("writing Z80 header: %w", err)
	}

	// Compress memory using Z80 compression (simple RLE)
	compressed := compressZ80Memory(zx)

	// Write compressed data
	if _, err := file.Write(compressed); err != nil {
		return err
	}

	fmt.Printf("Saved Z80 v1 snapshot: %s\n", filename)
	return nil
}

// LoadZ80 loads a .z80 format snapshot
func (zx *ZenZX) LoadZ80(filename string) error {
	fmt.Printf("Loading Z80 snapshot from %s\n", filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading Z80 file: %w", err)
	}

	if len(data) < 30 {
		return fmt.Errorf("Z80 file too small")
	}

	// Pause during load
	zx.paused = true

	reader := bytes.NewReader(data)
	var header Z80V1Header
	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("reading Z80 header: %w", err)
	}

	// Check for v2/v3 format (PC = 0 means extended header)
	if header.PC == 0 {
		return fmt.Errorf("Z80 v2/v3 format not yet implemented")
	}

	// Restore CPU state
	zx.cpu.A = header.A
	zx.cpu.F = header.F
	zx.cpu.SetBC(header.BC)
	zx.cpu.SetHL(header.HL)
	zx.cpu.PC = header.PC
	zx.cpu.SP = header.SP
	zx.cpu.I = header.I
	zx.cpu.R = header.R | ((header.Flags1 & 0x01) << 7) // Restore bit 7
	zx.cpu.SetDE(header.DE)
	zx.cpu.B_ = uint8(header.BC_ >> 8)
	zx.cpu.C_ = uint8(header.BC_)
	zx.cpu.D_ = uint8(header.DE_ >> 8)
	zx.cpu.E_ = uint8(header.DE_)
	zx.cpu.H_ = uint8(header.HL_ >> 8)
	zx.cpu.L_ = uint8(header.HL_)
	zx.cpu.A_ = header.A_
	zx.cpu.F_ = header.F_
	zx.cpu.SetIY(header.IY)
	zx.cpu.SetIX(header.IX)
	zx.cpu.IFF1 = header.IFF1 != 0
	zx.cpu.IFF2 = header.IFF2 != 0
	zx.cpu.IM = header.Flags2 & 0x03

	// Restore border color
	zx.io.borderColor = (header.Flags1 >> 1) & 0x07

	// Read compressed memory
	compressedData := make([]byte, len(data)-30)
	if _, err := io.ReadFull(reader, compressedData); err != nil {
		return err
	}

	// Decompress based on flag
	if header.Flags1&0x20 != 0 {
		// Compressed
		if err := decompressZ80Memory(zx, compressedData); err != nil {
			return err
		}
	} else {
		// Uncompressed - just copy 48KB
		copy(zx.memory.ram[5][:], compressedData[0:16384])
		copy(zx.memory.ram[2][:], compressedData[16384:32768])
		copy(zx.memory.ram[0][:], compressedData[32768:49152])
	}

	// Set up for 48K mode
	zx.is128K = false
	zx.memory.is128K = false
	zx.memory.ramBankLow = 5
	zx.memory.ramBankHigh = 2
	zx.memory.ramBankTop = 0
	zx.memory.screenBank = 5

	// Copy screen data to display
	copy(zx.screen.bitmap[:], zx.memory.ram[5][0:6144])
	copy(zx.screen.attributes[:], zx.memory.ram[5][6144:6912])

	// Clear pending interrupts
	zx.cpu.INT = false
	zx.cpu.NMI = false
	zx.cpu.Halted = false

	fmt.Printf("Loaded Z80 v1 snapshot: PC=%04X\n", zx.cpu.PC)
	return nil
}

// compressZ80Memory compresses 48KB memory using Z80 RLE compression
func compressZ80Memory(zx *ZenZX) []byte {
	var result []byte

	// Combine the three banks for 48K
	memory := make([]byte, 49152)
	copy(memory[0:16384], zx.memory.ram[5][:])
	copy(memory[16384:32768], zx.memory.ram[2][:])
	copy(memory[32768:49152], zx.memory.ram[0][:])

	i := 0
	for i < len(memory) {
		// Check for run of same bytes
		//runStart := i
		runByte := memory[i]
		runLength := 1

		for i+runLength < len(memory) && memory[i+runLength] == runByte && runLength < 255 {
			runLength++
		}

		if runLength >= 5 || runByte == 0xED {
			// Use RLE compression
			result = append(result, 0xED, 0xED, byte(runLength), runByte)
			i += runLength
		} else {
			// Output single byte
			result = append(result, runByte)
			i++
		}
	}

	// End marker
	result = append(result, 0x00, 0xED, 0xED, 0x00)

	return result
}

// decompressZ80Memory decompresses Z80 RLE compressed memory
func decompressZ80Memory(zx *ZenZX, compressed []byte) error {
	memory := make([]byte, 0, 49152)

	i := 0
	for i < len(compressed) {
		if i+3 < len(compressed) && compressed[i] == 0xED && compressed[i+1] == 0xED {
			// RLE sequence
			count := compressed[i+2]
			value := compressed[i+3]

			if count == 0 {
				// End marker
				break
			}

			for j := 0; j < int(count); j++ {
				memory = append(memory, value)
			}
			i += 4
		} else {
			// Single byte
			memory = append(memory, compressed[i])
			i++
		}
	}

	if len(memory) != 49152 {
		return fmt.Errorf("decompressed size mismatch: got %d, expected 49152", len(memory))
	}

	// Copy to RAM banks
	copy(zx.memory.ram[5][:], memory[0:16384])
	copy(zx.memory.ram[2][:], memory[16384:32768])
	copy(zx.memory.ram[0][:], memory[32768:49152])

	return nil
}

// ============================================================================
// Format Detection
// ============================================================================

// DetectSnapshotFormat detects the format of a snapshot file
func DetectSnapshotFormat(filename string) string {
	// First check extension
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".sna") {
		return "sna"
	} else if strings.HasSuffix(lower, ".z80") {
		return "z80"
	} else if strings.HasSuffix(lower, ".zxs") {
		return "zxs"
	}

	// Try to detect by content
	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Read first few bytes
	header := make([]byte, 10)
	if _, err := file.Read(header); err != nil {
		return ""
	}

	// Check for ZXS magic
	if string(header[:5]) == "ZENZX" {
		return "zxs"
	}

	// Check file size for SNA
	stat, err := file.Stat()
	if err == nil {
		size := stat.Size()
		if size == 49179 { // 48K SNA
			return "sna"
		} else if size == 131103 || size == 147487 { // 128K SNA
			return "sna"
		}
	}

	// Default to Z80 for other files
	return "z80"
}

// ============================================================================
// Unified Save/Load Functions
// ============================================================================

// SaveSnapshotFormat saves a snapshot in the specified format
func (zx *ZenZX) SaveSnapshotFormat(filename string, format string) error {
	switch format {
	case "sna":
		return zx.SaveSNA(filename)
	case "z80":
		return zx.SaveZ80(filename)
	case "zxs":
		return zx.SaveSnapshot(filename) // Original ZXS format
	default:
		// Auto-detect by extension
		detected := DetectSnapshotFormat(filename)
		if detected != "" {
			return zx.SaveSnapshotFormat(filename, detected)
		}
		return fmt.Errorf("unknown snapshot format: %s", format)
	}
}

// LoadSnapshotFormat loads a snapshot in the specified format
func (zx *ZenZX) LoadSnapshotFormat(filename string, format string) error {
	if format == "" {
		// Auto-detect
		format = DetectSnapshotFormat(filename)
		if format == "" {
			return fmt.Errorf("cannot detect snapshot format for: %s", filename)
		}
	}

	switch format {
	case "sna":
		return zx.LoadSNA(filename)
	case "z80":
		return zx.LoadZ80(filename)
	case "zxs":
		return zx.LoadSnapshot(filename) // Original ZXS format
	default:
		return fmt.Errorf("unknown snapshot format: %s", format)
	}
}
