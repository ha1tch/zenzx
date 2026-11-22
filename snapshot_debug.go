package zenzx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
)

// DebugSnapshot provides detailed debugging information for snapshots
type DebugSnapshot struct {
	zx *ZenZX
}

// NewDebugSnapshot creates a new snapshot debugger
func NewDebugSnapshot(zx *ZenZX) *DebugSnapshot {
	return &DebugSnapshot{zx: zx}
}

// AnalyzeCPUState prints detailed CPU state analysis
func (ds *DebugSnapshot) AnalyzeCPUState() {
	sayln("\n=== CPU State Analysis ===")

	// Register values
	sayf("Main Registers:\n")
	sayf("  AF = %04X (A=%02X, F=%02X)\n", ds.zx.cpu.AF(), ds.zx.cpu.A, ds.zx.cpu.F)
	sayf("  BC = %04X (B=%02X, C=%02X)\n", ds.zx.cpu.BC(), ds.zx.cpu.B, ds.zx.cpu.C)
	sayf("  DE = %04X (D=%02X, E=%02X)\n", ds.zx.cpu.DE(), ds.zx.cpu.D, ds.zx.cpu.E)
	sayf("  HL = %04X (H=%02X, L=%02X)\n", ds.zx.cpu.HL(), ds.zx.cpu.H, ds.zx.cpu.L)

	sayf("\nAlternate Registers:\n")
	sayf("  AF' = %04X (A'=%02X, F'=%02X)\n",
		uint16(ds.zx.cpu.A_)<<8|uint16(ds.zx.cpu.F_), ds.zx.cpu.A_, ds.zx.cpu.F_)
	sayf("  BC' = %04X (B'=%02X, C'=%02X)\n",
		uint16(ds.zx.cpu.B_)<<8|uint16(ds.zx.cpu.C_), ds.zx.cpu.B_, ds.zx.cpu.C_)
	sayf("  DE' = %04X (D'=%02X, E'=%02X)\n",
		uint16(ds.zx.cpu.D_)<<8|uint16(ds.zx.cpu.E_), ds.zx.cpu.D_, ds.zx.cpu.E_)
	sayf("  HL' = %04X (H'=%02X, L'=%02X)\n",
		uint16(ds.zx.cpu.H_)<<8|uint16(ds.zx.cpu.L_), ds.zx.cpu.H_, ds.zx.cpu.L_)

	sayf("\nIndex Registers:\n")
	sayf("  IX = %04X (IXH=%02X, IXL=%02X)\n", ds.zx.cpu.IX(), ds.zx.cpu.IXH, ds.zx.cpu.IXL)
	sayf("  IY = %04X (IYH=%02X, IYL=%02X)\n", ds.zx.cpu.IY(), ds.zx.cpu.IYH, ds.zx.cpu.IYL)

	sayf("\nSpecial Registers:\n")
	sayf("  PC = %04X\n", ds.zx.cpu.PC)
	sayf("  SP = %04X\n", ds.zx.cpu.SP)
	sayf("  I  = %02X\n", ds.zx.cpu.I)
	sayf("  R  = %02X (bit 7=%d, lower=%02X)\n", ds.zx.cpu.R, (ds.zx.cpu.R>>7)&1, ds.zx.cpu.R&0x7F)

	sayf("\nFlags (F=%02X):\n", ds.zx.cpu.F)
	sayf("  S=%d Z=%d Y=%d H=%d X=%d P=%d N=%d C=%d\n",
		btoi(ds.zx.cpu.F&0x80 != 0), // Sign
		btoi(ds.zx.cpu.F&0x40 != 0), // Zero
		btoi(ds.zx.cpu.F&0x20 != 0), // Y (bit 5)
		btoi(ds.zx.cpu.F&0x10 != 0), // Half-carry
		btoi(ds.zx.cpu.F&0x08 != 0), // X (bit 3)
		btoi(ds.zx.cpu.F&0x04 != 0), // Parity/Overflow
		btoi(ds.zx.cpu.F&0x02 != 0), // Add/Subtract
		btoi(ds.zx.cpu.F&0x01 != 0)) // Carry

	sayf("\nInterrupt State:\n")
	sayf("  IFF1=%v, IFF2=%v, IM=%d\n", ds.zx.cpu.IFF1, ds.zx.cpu.IFF2, ds.zx.cpu.IM)
	sayf("  INT=%v, NMI=%v, Halted=%v\n", ds.zx.cpu.INT, ds.zx.cpu.NMI, ds.zx.cpu.Halted)

	sayf("\nTiming:\n")
	sayf("  Total Cycles: %d\n", ds.zx.cpu.Cycles)
	sayf("  Frame Count: %d\n", ds.zx.frameCount)
	sayf("  Cycle in Frame: %d / %d\n", ds.zx.cycleCount, ds.zx.cyclesPerFrame)
}

// AnalyzeMemoryState prints memory configuration analysis
func (ds *DebugSnapshot) AnalyzeMemoryState() {
	sayln("\n=== Memory State Analysis ===")

	sayf("Model Configuration:\n")
	sayf("  48K mode: %v\n", !ds.zx.memory.is128K)
	sayf("  128K mode: %v\n", ds.zx.memory.is128K && !ds.zx.memory.isPlus3)
	sayf("  +3 mode: %v\n", ds.zx.memory.isPlus3)

	if ds.zx.memory.is128K || ds.zx.memory.isPlus3 {
		sayf("\nBanking State:\n")
		sayf("  ROM Bank: %d\n", ds.zx.memory.romBank)
		sayf("  RAM Bank at 0x4000: %d\n", ds.zx.memory.ramBankLow)
		sayf("  RAM Bank at 0x8000: %d\n", ds.zx.memory.ramBankHigh)
		sayf("  RAM Bank at 0xC000: %d\n", ds.zx.memory.ramBankTop)
		sayf("  Screen Bank: %d\n", ds.zx.memory.screenBank)
		sayf("  Paging Locked: %v\n", ds.zx.memory.pagingLocked)
		sayf("  Port 0x7FFD: %02X\n", ds.zx.memory.port7FFD)
		sayf("  Port 0x1FFD: %02X\n", ds.zx.memory.port1FFD)
		sayf("  Special Mode: %d\n", ds.zx.memory.specialMode)
	} else {
		sayf("\n48K Mode - No banking, but internal state shows:\n")
		sayf("  ramBankLow: %d (should be 5 for 48K)\n", ds.zx.memory.ramBankLow)
		sayf("  ramBankHigh: %d (should be 2 for 48K)\n", ds.zx.memory.ramBankHigh)
		sayf("  ramBankTop: %d (should be 0 for 48K)\n", ds.zx.memory.ramBankTop)
		sayf("  screenBank: %d (should be 5, but irrelevant for 48K)\n", ds.zx.memory.screenBank)
	}
}

// TestEndianness verifies that 16-bit values are saved correctly
func (ds *DebugSnapshot) TestEndianness() {
	sayln("\n=== Endianness Test ===")

	// Test saving a known CPU state
	testState := CPUState{
		AF: 0x1234, // Should save as 34 12 in little-endian
		BC: 0x5678, // Should save as 78 56 in little-endian
		PC: 0xABCD, // Should save as CD AB in little-endian
	}

	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.LittleEndian, testState)
	if err != nil {
		sayf("Error writing test state: %v\n", err)
		return
	}

	data := buf.Bytes()
	sayf("Test values written (hex dump of first 16 bytes):\n")
	sayf("%s\n", hex.Dump(data[:16]))

	// Verify the actual values
	sayf("\nVerifying saved values:\n")
	sayf("  AF=0x1234 saved as: %02X %02X (should be 34 12)\n", data[0], data[1])
	sayf("  BC=0x5678 saved as: %02X %02X (should be 78 56)\n", data[2], data[3])

	// Now test loading it back
	reader := bytes.NewReader(data)
	var loadedState CPUState
	err = binary.Read(reader, binary.LittleEndian, &loadedState)
	if err != nil {
		sayf("Error reading test state: %v\n", err)
		return
	}

	sayf("\nLoaded values:\n")
	sayf("  AF: %04X (should be 1234)\n", loadedState.AF)
	sayf("  BC: %04X (should be 5678)\n", loadedState.BC)
	sayf("  PC: %04X (should be ABCD)\n", loadedState.PC)

	if loadedState.AF == 0x1234 && loadedState.BC == 0x5678 && loadedState.PC == 0xABCD {
		sayln("\n✓ Endianness is CORRECT")
	} else {
		sayln("\n✗ Endianness is WRONG!")
	}
}

// AnalyzeRRegister investigates the R register behavior
func (ds *DebugSnapshot) AnalyzeRRegister() {
	sayln("\n=== R Register Analysis ===")

	sayf("Current R value: %02X\n", ds.zx.cpu.R)
	sayf("  Bit 7: %d (preserved)\n", (ds.zx.cpu.R>>7)&1)
	sayf("  Lower 7 bits: %02X (counter)\n", ds.zx.cpu.R&0x7F)

	// Check if R increments properly
	sayln("\nTesting R register increment...")
	initialR := ds.zx.cpu.R
	initialCycles := ds.zx.cpu.Cycles

	// Run a few instructions
	for i := 0; i < 10 && ds.zx.running; i++ {
		cycles := ds.zx.cpu.Step()
		sayf("  Step %d: %d cycles, R=%02X\n", i+1, cycles, ds.zx.cpu.R)
	}

	finalR := ds.zx.cpu.R
	finalCycles := ds.zx.cpu.Cycles

	sayf("\nR changed from %02X to %02X\n", initialR, finalR)
	sayf("Cycles executed: %d\n", finalCycles-initialCycles)

	expectedRIncrease := 10 // Roughly 1 per instruction
	actualRIncrease := int((finalR & 0x7F) - (initialR & 0x7F))
	if actualRIncrease < 0 {
		actualRIncrease += 128 // Handle wrap
	}

	sayf("Expected R increase: ~%d\n", expectedRIncrease)
	sayf("Actual R increase: %d\n", actualRIncrease)
}

// CheckStackContents examines what's on the stack
func (ds *DebugSnapshot) CheckStackContents() {
	sayln("\n=== Stack Analysis ===")

	sp := ds.zx.cpu.SP
	sayf("Stack Pointer: %04X\n", sp)

	// Show the next 16 bytes on the stack
	sayln("Stack contents (next 16 words that would be popped):")
	for i := uint16(0); i < 16 && sp+i*2 < 0xFFFF; i++ {
		addr := sp + i*2
		low := ds.zx.memory.Read(addr)
		high := ds.zx.memory.Read(addr + 1)
		word := uint16(high)<<8 | uint16(low)
		sayf("  [%04X]: %04X", addr, word)

		// Try to identify what this might be
		if word < 0x4000 {
			sayf(" (ROM address)")
		} else if word >= 0x4000 && word < 0x5800 {
			sayf(" (Screen memory)")
		} else if word >= 0x5800 && word < 0x5B00 {
			sayf(" (Attributes)")
		} else if word >= 0x5C00 && word < 0x6000 {
			sayf(" (System variables)")
		}
		sayln()
	}
}

// CheckROMLocation checks what's at the current PC in ROM
func (ds *DebugSnapshot) CheckROMLocation() {
	sayln("\n=== ROM Location Analysis ===")

	pc := ds.zx.cpu.PC
	sayf("PC = %04X\n", pc)

	if pc < 0x4000 {
		sayln("Executing from ROM")

		// Known ROM locations in 48K ROM
		knownLocations := map[uint16]string{
			0x0000: "RESET - Cold start",
			0x0008: "ERROR-1 - Error restart",
			0x0010: "PRINT-A-1 - Print character",
			0x0018: "GET-CHAR - Get current character",
			0x0020: "NEXT-CHAR - Get next character",
			0x0028: "FP-CALC - Floating point calculator",
			0x0030: "BC-SPACES - Create BC spaces",
			0x0038: "MASK-INT - Maskable interrupt",
			0x0048: "KEY-INT - Keyboard interrupt routine",
			0x0066: "NMI - Non-maskable interrupt",
			0x11CB: "START/NEW - Basic initialization",
			0x1219: "RAM-CHECK - Memory check routine",
		}

		// Find nearest known location
		var nearest uint16
		var nearestName string
		for addr, name := range knownLocations {
			if addr <= pc && addr > nearest {
				nearest = addr
				nearestName = name
			}
		}

		if nearestName != "" {
			sayf("Near: %04X %s (+%d bytes)\n", nearest, nearestName, pc-nearest)
		}

		// Disassemble a few bytes at PC
		sayf("\nBytes at PC:\n")
		for i := uint16(0); i < 8; i++ {
			b := ds.zx.memory.Read(pc + i)
			sayf("  [%04X]: %02X\n", pc+i, b)
		}
	}
}

// btoi converts bool to int for display
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// RunFullDiagnostics runs all diagnostic tests
func (ds *DebugSnapshot) RunFullDiagnostics() {
	sayln("====================")
	sayln("SNAPSHOT DIAGNOSTICS")
	sayln("====================")

	ds.TestEndianness()
	ds.AnalyzeCPUState()
	ds.AnalyzeMemoryState()
	ds.CheckROMLocation()
	ds.CheckStackContents()
	ds.AnalyzeRRegister()

	sayln("\n" + "===============")
	sayln("END DIAGNOSTICS")
	sayln("===============")
}
