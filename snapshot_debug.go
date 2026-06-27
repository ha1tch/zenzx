package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
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
	fmt.Println("\n=== CPU State Analysis ===")

	// Register values
	fmt.Printf("Main Registers:\n")
	fmt.Printf("  AF = %04X (A=%02X, F=%02X)\n", ds.zx.cpu.AF(), ds.zx.cpu.A, ds.zx.cpu.F)
	fmt.Printf("  BC = %04X (B=%02X, C=%02X)\n", ds.zx.cpu.BC(), ds.zx.cpu.B, ds.zx.cpu.C)
	fmt.Printf("  DE = %04X (D=%02X, E=%02X)\n", ds.zx.cpu.DE(), ds.zx.cpu.D, ds.zx.cpu.E)
	fmt.Printf("  HL = %04X (H=%02X, L=%02X)\n", ds.zx.cpu.HL(), ds.zx.cpu.H, ds.zx.cpu.L)

	fmt.Printf("\nAlternate Registers:\n")
	fmt.Printf("  AF' = %04X (A'=%02X, F'=%02X)\n",
		uint16(ds.zx.cpu.A_)<<8|uint16(ds.zx.cpu.F_), ds.zx.cpu.A_, ds.zx.cpu.F_)
	fmt.Printf("  BC' = %04X (B'=%02X, C'=%02X)\n",
		uint16(ds.zx.cpu.B_)<<8|uint16(ds.zx.cpu.C_), ds.zx.cpu.B_, ds.zx.cpu.C_)
	fmt.Printf("  DE' = %04X (D'=%02X, E'=%02X)\n",
		uint16(ds.zx.cpu.D_)<<8|uint16(ds.zx.cpu.E_), ds.zx.cpu.D_, ds.zx.cpu.E_)
	fmt.Printf("  HL' = %04X (H'=%02X, L'=%02X)\n",
		uint16(ds.zx.cpu.H_)<<8|uint16(ds.zx.cpu.L_), ds.zx.cpu.H_, ds.zx.cpu.L_)

	fmt.Printf("\nIndex Registers:\n")
	fmt.Printf("  IX = %04X (IXH=%02X, IXL=%02X)\n", ds.zx.cpu.IX(), ds.zx.cpu.IXH, ds.zx.cpu.IXL)
	fmt.Printf("  IY = %04X (IYH=%02X, IYL=%02X)\n", ds.zx.cpu.IY(), ds.zx.cpu.IYH, ds.zx.cpu.IYL)

	fmt.Printf("\nSpecial Registers:\n")
	fmt.Printf("  PC = %04X\n", ds.zx.cpu.PC)
	fmt.Printf("  SP = %04X\n", ds.zx.cpu.SP)
	fmt.Printf("  I  = %02X\n", ds.zx.cpu.I)
	fmt.Printf("  R  = %02X (bit 7=%d, lower=%02X)\n", ds.zx.cpu.R, (ds.zx.cpu.R>>7)&1, ds.zx.cpu.R&0x7F)

	fmt.Printf("\nFlags (F=%02X):\n", ds.zx.cpu.F)
	fmt.Printf("  S=%d Z=%d Y=%d H=%d X=%d P=%d N=%d C=%d\n",
		btoi(ds.zx.cpu.F&0x80 != 0), // Sign
		btoi(ds.zx.cpu.F&0x40 != 0), // Zero
		btoi(ds.zx.cpu.F&0x20 != 0), // Y (bit 5)
		btoi(ds.zx.cpu.F&0x10 != 0), // Half-carry
		btoi(ds.zx.cpu.F&0x08 != 0), // X (bit 3)
		btoi(ds.zx.cpu.F&0x04 != 0), // Parity/Overflow
		btoi(ds.zx.cpu.F&0x02 != 0), // Add/Subtract
		btoi(ds.zx.cpu.F&0x01 != 0)) // Carry

	fmt.Printf("\nInterrupt State:\n")
	fmt.Printf("  IFF1=%v, IFF2=%v, IM=%d\n", ds.zx.cpu.IFF1, ds.zx.cpu.IFF2, ds.zx.cpu.IM)
	fmt.Printf("  INT=%v, NMI=%v, Halted=%v\n", ds.zx.cpu.INT, ds.zx.cpu.NMI, ds.zx.cpu.Halted)

	fmt.Printf("\nTiming:\n")
	fmt.Printf("  Total Cycles: %d\n", ds.zx.cpu.Cycles)
	fmt.Printf("  Frame Count: %d\n", ds.zx.frameCount)
	fmt.Printf("  Cycle in Frame: %d / %d\n", ds.zx.cycleCount, CyclesPerFrame)
}

// AnalyzeMemoryState prints memory configuration analysis
func (ds *DebugSnapshot) AnalyzeMemoryState() {
	fmt.Println("\n=== Memory State Analysis ===")

	fmt.Printf("Model Configuration:\n")
	fmt.Printf("  48K mode: %v\n", !ds.zx.memory.is128K)
	fmt.Printf("  128K mode: %v\n", ds.zx.memory.is128K && !ds.zx.memory.isPlus3)
	fmt.Printf("  +3 mode: %v\n", ds.zx.memory.isPlus3)

	if ds.zx.memory.is128K || ds.zx.memory.isPlus3 {
		fmt.Printf("\nBanking State:\n")
		fmt.Printf("  ROM Bank: %d\n", ds.zx.memory.romBank)
		fmt.Printf("  RAM Bank at 0x4000: %d\n", ds.zx.memory.ramBankLow)
		fmt.Printf("  RAM Bank at 0x8000: %d\n", ds.zx.memory.ramBankHigh)
		fmt.Printf("  RAM Bank at 0xC000: %d\n", ds.zx.memory.ramBankTop)
		fmt.Printf("  Screen Bank: %d\n", ds.zx.memory.screenBank)
		fmt.Printf("  Paging Locked: %v\n", ds.zx.memory.pagingLocked)
		fmt.Printf("  Port 0x7FFD: %02X\n", ds.zx.memory.port7FFD)
		fmt.Printf("  Port 0x1FFD: %02X\n", ds.zx.memory.port1FFD)
		fmt.Printf("  Special Mode: %d\n", ds.zx.memory.specialMode)
	} else {
		fmt.Printf("\n48K Mode - No banking, but internal state shows:\n")
		fmt.Printf("  ramBankLow: %d (should be 5 for 48K)\n", ds.zx.memory.ramBankLow)
		fmt.Printf("  ramBankHigh: %d (should be 2 for 48K)\n", ds.zx.memory.ramBankHigh)
		fmt.Printf("  ramBankTop: %d (should be 0 for 48K)\n", ds.zx.memory.ramBankTop)
		fmt.Printf("  screenBank: %d (should be 5, but irrelevant for 48K)\n", ds.zx.memory.screenBank)
	}
}

// TestEndianness verifies that 16-bit values are saved correctly
func (ds *DebugSnapshot) TestEndianness() {
	fmt.Println("\n=== Endianness Test ===")

	// Test saving a known CPU state
	testState := CPUState{
		AF: 0x1234, // Should save as 34 12 in little-endian
		BC: 0x5678, // Should save as 78 56 in little-endian
		PC: 0xABCD, // Should save as CD AB in little-endian
	}

	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.LittleEndian, testState)
	if err != nil {
		fmt.Printf("Error writing test state: %v\n", err)
		return
	}

	data := buf.Bytes()
	fmt.Printf("Test values written (hex dump of first 16 bytes):\n")
	fmt.Printf("%s\n", hex.Dump(data[:16]))

	// Verify the actual values
	fmt.Printf("\nVerifying saved values:\n")
	fmt.Printf("  AF=0x1234 saved as: %02X %02X (should be 34 12)\n", data[0], data[1])
	fmt.Printf("  BC=0x5678 saved as: %02X %02X (should be 78 56)\n", data[2], data[3])

	// Now test loading it back
	reader := bytes.NewReader(data)
	var loadedState CPUState
	err = binary.Read(reader, binary.LittleEndian, &loadedState)
	if err != nil {
		fmt.Printf("Error reading test state: %v\n", err)
		return
	}

	fmt.Printf("\nLoaded values:\n")
	fmt.Printf("  AF: %04X (should be 1234)\n", loadedState.AF)
	fmt.Printf("  BC: %04X (should be 5678)\n", loadedState.BC)
	fmt.Printf("  PC: %04X (should be ABCD)\n", loadedState.PC)

	if loadedState.AF == 0x1234 && loadedState.BC == 0x5678 && loadedState.PC == 0xABCD {
		fmt.Println("\n✓ Endianness is CORRECT")
	} else {
		fmt.Println("\n✗ Endianness is WRONG!")
	}
}

// AnalyzeRRegister investigates the R register behavior
func (ds *DebugSnapshot) AnalyzeRRegister() {
	fmt.Println("\n=== R Register Analysis ===")

	fmt.Printf("Current R value: %02X\n", ds.zx.cpu.R)
	fmt.Printf("  Bit 7: %d (preserved)\n", (ds.zx.cpu.R>>7)&1)
	fmt.Printf("  Lower 7 bits: %02X (counter)\n", ds.zx.cpu.R&0x7F)

	// Check if R increments properly
	fmt.Println("\nTesting R register increment...")
	initialR := ds.zx.cpu.R
	initialCycles := ds.zx.cpu.Cycles

	// Run a few instructions
	for i := 0; i < 10 && ds.zx.running; i++ {
		cycles := ds.zx.cpu.Step()
		fmt.Printf("  Step %d: %d cycles, R=%02X\n", i+1, cycles, ds.zx.cpu.R)
	}

	finalR := ds.zx.cpu.R
	finalCycles := ds.zx.cpu.Cycles

	fmt.Printf("\nR changed from %02X to %02X\n", initialR, finalR)
	fmt.Printf("Cycles executed: %d\n", finalCycles-initialCycles)

	expectedRIncrease := 10 // Roughly 1 per instruction
	actualRIncrease := int((finalR & 0x7F) - (initialR & 0x7F))
	if actualRIncrease < 0 {
		actualRIncrease += 128 // Handle wrap
	}

	fmt.Printf("Expected R increase: ~%d\n", expectedRIncrease)
	fmt.Printf("Actual R increase: %d\n", actualRIncrease)
}

// CheckStackContents examines what's on the stack
func (ds *DebugSnapshot) CheckStackContents() {
	fmt.Println("\n=== Stack Analysis ===")

	sp := ds.zx.cpu.SP
	fmt.Printf("Stack Pointer: %04X\n", sp)

	// Show the next 16 bytes on the stack
	fmt.Println("Stack contents (next 16 words that would be popped):")
	for i := uint16(0); i < 16 && sp+i*2 < 0xFFFF; i++ {
		addr := sp + i*2
		low := ds.zx.memory.Read(addr)
		high := ds.zx.memory.Read(addr + 1)
		word := uint16(high)<<8 | uint16(low)
		fmt.Printf("  [%04X]: %04X", addr, word)

		// Try to identify what this might be
		if word < 0x4000 {
			fmt.Printf(" (ROM address)")
		} else if word >= 0x4000 && word < 0x5800 {
			fmt.Printf(" (Screen memory)")
		} else if word >= 0x5800 && word < 0x5B00 {
			fmt.Printf(" (Attributes)")
		} else if word >= 0x5C00 && word < 0x6000 {
			fmt.Printf(" (System variables)")
		}
		fmt.Println()
	}
}

// CheckROMLocation checks what's at the current PC in ROM
func (ds *DebugSnapshot) CheckROMLocation() {
	fmt.Println("\n=== ROM Location Analysis ===")

	pc := ds.zx.cpu.PC
	fmt.Printf("PC = %04X\n", pc)

	if pc < 0x4000 {
		fmt.Println("Executing from ROM")

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
			fmt.Printf("Near: %04X %s (+%d bytes)\n", nearest, nearestName, pc-nearest)
		}

		// Disassemble a few bytes at PC
		fmt.Printf("\nBytes at PC:\n")
		for i := uint16(0); i < 8; i++ {
			b := ds.zx.memory.Read(pc + i)
			fmt.Printf("  [%04X]: %02X\n", pc+i, b)
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
	fmt.Println("====================")
	fmt.Println("SNAPSHOT DIAGNOSTICS")
	fmt.Println("====================")

	ds.TestEndianness()
	ds.AnalyzeCPUState()
	ds.AnalyzeMemoryState()
	ds.CheckROMLocation()
	ds.CheckStackContents()
	ds.AnalyzeRRegister()

	fmt.Println("\n" + "===============")
	fmt.Println("END DIAGNOSTICS")
	fmt.Println("===============")
}
