package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
)

// ============================================================================
// Chunk Writers
// ============================================================================

func writeCPUChunk(zx *ZenZX, w io.Writer) error {
	state := CPUState{
		AF:      zx.cpu.AF(),
		BC:      zx.cpu.BC(),
		DE:      zx.cpu.DE(),
		HL:      zx.cpu.HL(),
		AF_:     uint16(zx.cpu.A_)<<8 | uint16(zx.cpu.F_),
		BC_:     uint16(zx.cpu.B_)<<8 | uint16(zx.cpu.C_),
		DE_:     uint16(zx.cpu.D_)<<8 | uint16(zx.cpu.E_),
		HL_:     uint16(zx.cpu.H_)<<8 | uint16(zx.cpu.L_),
		IX:      zx.cpu.IX(),
		IY:      zx.cpu.IY(),
		SP:      zx.cpu.SP,
		PC:      zx.cpu.PC,
		WZ:      zx.cpu.WZ,
		I:       zx.cpu.I,
		R:       zx.cpu.R,
		IFF1:    zx.cpu.IFF1,
		IFF2:    zx.cpu.IFF2,
		IM:      zx.cpu.IM,
		Halted:  zx.cpu.Halted,
		Cycles:  zx.cpu.Cycles,
		INT:     zx.cpu.INT,
		NMI:     zx.cpu.NMI,
		NMIEdge: false, // We'll handle this properly on load
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, state); err != nil {
		return err
	}

	header := ChunkHeader{ID: ChunkCPU, Size: uint32(buf.Len())}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writeMemoryChunks(zx *ZenZX, w io.Writer) error {
	if zx.isPlus3 {
		// Write all ROM and RAM banks for +3
		buf := &bytes.Buffer{}

		// Write 4 ROM banks
		for i := 0; i < 4; i++ {
			if _, err := buf.Write(zx.memory.rom[i][:]); err != nil {
				return err
			}
		}

		// Write 8 RAM banks
		for i := 0; i < 8; i++ {
			if _, err := buf.Write(zx.memory.ram[i][:]); err != nil {
				return err
			}
		}

		// Compress if large
		compressed := &bytes.Buffer{}
		gw := gzip.NewWriter(compressed)
		if _, err := gw.Write(buf.Bytes()); err != nil {
			return err
		}
		gw.Close()

		header := ChunkHeader{ID: ChunkMemPlus3, Size: uint32(compressed.Len())}
		if err := binary.Write(w, binary.LittleEndian, header); err != nil {
			return err
		}
		_, err := w.Write(compressed.Bytes())
		return err

	} else if zx.is128K {
		// Write 2 ROM banks and 8 RAM banks for 128K
		buf := &bytes.Buffer{}

		// Write 2 ROM banks
		for i := 0; i < 2; i++ {
			if _, err := buf.Write(zx.memory.rom[i][:]); err != nil {
				return err
			}
		}

		// Write 8 RAM banks
		for i := 0; i < 8; i++ {
			if _, err := buf.Write(zx.memory.ram[i][:]); err != nil {
				return err
			}
		}

		// Compress
		compressed := &bytes.Buffer{}
		gw := gzip.NewWriter(compressed)
		if _, err := gw.Write(buf.Bytes()); err != nil {
			return err
		}
		gw.Close()

		header := ChunkHeader{ID: ChunkMem128, Size: uint32(compressed.Len())}
		if err := binary.Write(w, binary.LittleEndian, header); err != nil {
			return err
		}
		_, err := w.Write(compressed.Bytes())
		return err

	} else {
		// 48K model - write ROM and 3 RAM banks
		buf := &bytes.Buffer{}

		// Write ROM
		if _, err := buf.Write(zx.memory.rom[0][:]); err != nil {
			return err
		}

		// Write RAM banks 5, 2, 0 (48K layout)
		if _, err := buf.Write(zx.memory.ram[5][:]); err != nil {
			return err
		}
		if _, err := buf.Write(zx.memory.ram[2][:]); err != nil {
			return err
		}
		if _, err := buf.Write(zx.memory.ram[0][:]); err != nil {
			return err
		}

		header := ChunkHeader{ID: ChunkMem48, Size: uint32(buf.Len())}
		if err := binary.Write(w, binary.LittleEndian, header); err != nil {
			return err
		}
		_, err := w.Write(buf.Bytes())
		return err
	}
}

func writeScreenChunk(zx *ZenZX, w io.Writer) error {
	buf := &bytes.Buffer{}

	// Write bitmap and attributes
	if _, err := buf.Write(zx.screen.bitmap); err != nil {
		return err
	}
	if _, err := buf.Write(zx.screen.attributes); err != nil {
		return err
	}

	// Write screen state
	if err := binary.Write(buf, binary.LittleEndian, zx.screen.flashTickTock); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, zx.screen.flashEnabled); err != nil {
		return err
	}

	header := ChunkHeader{ID: ChunkScreen, Size: uint32(buf.Len())}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writeIOChunk(zx *ZenZX, w io.Writer) error {
	// Get AY registers safely
	ayRegisters := [16]uint8{}
	if zx.io != nil {
		ayRegisters = zx.io.GetAYRegisters()
	}

	state := IOState{
		BorderColor: zx.io.borderColor,
		Keyboard:    zx.io.keyboard,
		AYRegister:  zx.io.ayRegister,
		Speaker:     zx.io.speaker,
		AYRegisters: ayRegisters,
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, state); err != nil {
		return err
	}

	header := ChunkHeader{ID: ChunkIO, Size: uint32(buf.Len())}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writePagingChunk(zx *ZenZX, w io.Writer) error {
	state := PagingState{
		ROMBank:      zx.memory.romBank,
		RAMBankTop:   zx.memory.ramBankTop,
		ScreenBank:   zx.memory.screenBank,
		PagingLocked: zx.memory.pagingLocked,
		Port7FFD:     zx.memory.port7FFD,
		Port1FFD:     zx.memory.port1FFD,
		SpecialMode:  zx.memory.specialMode,
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, state); err != nil {
		return err
	}

	header := ChunkHeader{ID: ChunkPaging, Size: uint32(buf.Len())}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writeAudioChunk(zx *ZenZX, w io.Writer) error {
	if zx.audio == nil {
		return nil // No audio to save
	}

	// Get AY registers safely
	ayRegisters := [16]uint8{}
	if zx.io != nil {
		ayRegisters = zx.io.GetAYRegisters()
	}

	state := AudioState{
		BeeperLevel:  zx.io.speaker,
		AYRegisters:  ayRegisters,
		MasterVolume: zx.audio.GetMasterVolume(),
		BeeperVolume: zx.audio.GetBeeperVolume(),
		AYVolume:     zx.audio.GetAYVolume(),
		AudioEnabled: zx.audio.IsEnabled(),
	}

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, state); err != nil {
		return err
	}

	header := ChunkHeader{ID: ChunkAudio, Size: uint32(buf.Len())}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writeFDCChunk(zx *ZenZX, w io.Writer) error {
	fdc := zx.io.fdc

	buf := &bytes.Buffer{}

	// Write fixed-size fields individually
	if err := binary.Write(buf, binary.LittleEndian, zx.io.hasFDC); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.diskPresent); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.diskModified); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.currentTrack); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.currentHead); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.currentSector); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.mainStatus); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.st0); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.st1); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.st2); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, fdc.st3); err != nil {
		return err
	}

	// Write filename as length-prefixed string
	filenameBytes := []byte(fdc.diskFilename)
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(filenameBytes))); err != nil {
		return err
	}
	if _, err := buf.Write(filenameBytes); err != nil {
		return err
	}

	header := ChunkHeader{ID: ChunkFDC, Size: uint32(buf.Len())}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writeDiskChunk(zx *ZenZX, w io.Writer) error {
	if !zx.io.fdc.diskPresent {
		return nil
	}

	// Compress disk image
	compressed := &bytes.Buffer{}
	gw := gzip.NewWriter(compressed)
	if _, err := gw.Write(zx.io.fdc.diskImage); err != nil {
		return err
	}
	gw.Close()

	header := ChunkHeader{ID: ChunkDisk, Size: uint32(compressed.Len())}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}

	_, err := w.Write(compressed.Bytes())
	return err
}

// ============================================================================
// Chunk Loaders
// ============================================================================

func loadCPUChunk(zx *ZenZX, r io.Reader) error {
	var state CPUState
	if err := binary.Read(r, binary.LittleEndian, &state); err != nil {
		return err
	}

	// Restore CPU registers
	zx.cpu.SetAF(state.AF)
	zx.cpu.SetBC(state.BC)
	zx.cpu.SetDE(state.DE)
	zx.cpu.SetHL(state.HL)

	// Restore alternate registers
	zx.cpu.A_ = uint8(state.AF_ >> 8)
	zx.cpu.F_ = uint8(state.AF_)
	zx.cpu.B_ = uint8(state.BC_ >> 8)
	zx.cpu.C_ = uint8(state.BC_)
	zx.cpu.D_ = uint8(state.DE_ >> 8)
	zx.cpu.E_ = uint8(state.DE_)
	zx.cpu.H_ = uint8(state.HL_ >> 8)
	zx.cpu.L_ = uint8(state.HL_)

	// Restore other registers
	zx.cpu.SetIX(state.IX)
	zx.cpu.SetIY(state.IY)
	zx.cpu.SP = state.SP
	zx.cpu.PC = state.PC
	zx.cpu.WZ = state.WZ
	zx.cpu.I = state.I
	zx.cpu.R = state.R

	// Restore interrupt state
	zx.cpu.IFF1 = state.IFF1
	zx.cpu.IFF2 = state.IFF2
	zx.cpu.IM = state.IM
	zx.cpu.Halted = state.Halted
	zx.cpu.Cycles = state.Cycles

	// Don't restore INT/NMI as these are transient states
	// We want them clear to avoid spurious interrupts
	zx.cpu.INT = false
	zx.cpu.NMI = false

	// Also restore the cycle/frame counters in the main emulator
	zx.cycleCount = int(state.Cycles % uint64(CyclesPerFrame))

	return nil
}

func loadMem48Chunk(zx *ZenZX, r io.Reader) error {
	// Read ROM
	if _, err := io.ReadFull(r, zx.memory.rom[0][:]); err != nil {
		return err
	}

	// Read RAM banks
	if _, err := io.ReadFull(r, zx.memory.ram[5][:]); err != nil {
		return err
	}
	if _, err := io.ReadFull(r, zx.memory.ram[2][:]); err != nil {
		return err
	}
	if _, err := io.ReadFull(r, zx.memory.ram[0][:]); err != nil {
		return err
	}

	zx.is128K = false
	zx.isPlus3 = false
	zx.memory.is128K = false
	zx.memory.isPlus3 = false

	return nil
}

func loadMem128Chunk(zx *ZenZX, r io.Reader) error {
	// Decompress data
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	// Read 2 ROM banks
	for i := 0; i < 2; i++ {
		if _, err := io.ReadFull(gr, zx.memory.rom[i][:]); err != nil {
			return err
		}
	}

	// Read 8 RAM banks
	for i := 0; i < 8; i++ {
		if _, err := io.ReadFull(gr, zx.memory.ram[i][:]); err != nil {
			return err
		}
	}

	zx.is128K = true
	zx.isPlus3 = false
	zx.memory.Enable128K()

	return nil
}

func loadMemPlus3Chunk(zx *ZenZX, r io.Reader) error {
	// Decompress data
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	// Read 4 ROM banks
	for i := 0; i < 4; i++ {
		if _, err := io.ReadFull(gr, zx.memory.rom[i][:]); err != nil {
			return err
		}
	}

	// Read 8 RAM banks
	for i := 0; i < 8; i++ {
		if _, err := io.ReadFull(gr, zx.memory.ram[i][:]); err != nil {
			return err
		}
	}

	zx.is128K = true
	zx.isPlus3 = true
	zx.memory.EnablePlus3()

	return nil
}

func loadScreenChunk(zx *ZenZX, r io.Reader) error {
	// Read bitmap and attributes
	if _, err := io.ReadFull(r, zx.screen.bitmap); err != nil {
		return err
	}
	if _, err := io.ReadFull(r, zx.screen.attributes); err != nil {
		return err
	}

	// Read screen state
	if err := binary.Read(r, binary.LittleEndian, &zx.screen.flashTickTock); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &zx.screen.flashEnabled); err != nil {
		return err
	}

	return nil
}

func loadIOChunk(zx *ZenZX, r io.Reader) error {
	var state IOState
	if err := binary.Read(r, binary.LittleEndian, &state); err != nil {
		return err
	}

	zx.io.borderColor = state.BorderColor
	zx.io.keyboard = state.Keyboard
	zx.io.ayRegister = state.AYRegister
	zx.io.speaker = state.Speaker

	// Restore AY registers if present
	if zx.io != nil {
		zx.io.SetAYRegisters(state.AYRegisters)
	}

	return nil
}

func loadPagingChunk(zx *ZenZX, r io.Reader) error {
	var state PagingState
	if err := binary.Read(r, binary.LittleEndian, &state); err != nil {
		return err
	}

	zx.memory.romBank = state.ROMBank
	zx.memory.ramBankTop = state.RAMBankTop
	zx.memory.screenBank = state.ScreenBank
	zx.memory.pagingLocked = state.PagingLocked
	zx.memory.port7FFD = state.Port7FFD
	zx.memory.port1FFD = state.Port1FFD
	zx.memory.specialMode = state.SpecialMode

	// Update other banking registers based on model
	if zx.memory.specialMode == 0 {
		// Normal mode
		zx.memory.ramBankLow = 5
		zx.memory.ramBankHigh = 2
	} else {
		// Special mode configurations
		switch zx.memory.specialMode {
		case 1:
			zx.memory.ramBankLow = 5
			zx.memory.ramBankHigh = 6
			zx.memory.ramBankTop = 7
		case 2:
			zx.memory.ramBankLow = 5
			zx.memory.ramBankHigh = 6
			zx.memory.ramBankTop = 3
		case 3:
			zx.memory.ramBankLow = 7
			zx.memory.ramBankHigh = 6
			zx.memory.ramBankTop = 3
		}
	}

	return nil
}

func loadAudioChunk(zx *ZenZX, r io.Reader) error {
	var state AudioState
	if err := binary.Read(r, binary.LittleEndian, &state); err != nil {
		return err
	}

	// Restore speaker state
	zx.io.speaker = state.BeeperLevel

	// Restore AY registers
	if zx.io != nil {
		zx.io.SetAYRegisters(state.AYRegisters)
	}

	// Restore volume settings if audio is available
	if zx.audio != nil {
		zx.audio.SetMasterVolume(state.MasterVolume)
		zx.audio.SetBeeperVolume(state.BeeperVolume)
		zx.audio.SetAYVolume(state.AYVolume)
		zx.audio.SetEnabled(state.AudioEnabled)
	}

	return nil
}

func loadFDCChunk(zx *ZenZX, r io.Reader) error {
	// Read fixed-size fields individually
	var present bool
	if err := binary.Read(r, binary.LittleEndian, &present); err != nil {
		return err
	}

	var diskPresent bool
	if err := binary.Read(r, binary.LittleEndian, &diskPresent); err != nil {
		return err
	}

	var diskModified bool
	if err := binary.Read(r, binary.LittleEndian, &diskModified); err != nil {
		return err
	}

	var currentTrack uint8
	if err := binary.Read(r, binary.LittleEndian, &currentTrack); err != nil {
		return err
	}

	var currentHead uint8
	if err := binary.Read(r, binary.LittleEndian, &currentHead); err != nil {
		return err
	}

	var currentSector uint8
	if err := binary.Read(r, binary.LittleEndian, &currentSector); err != nil {
		return err
	}

	var mainStatus uint8
	if err := binary.Read(r, binary.LittleEndian, &mainStatus); err != nil {
		return err
	}

	var st0, st1, st2, st3 uint8
	if err := binary.Read(r, binary.LittleEndian, &st0); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &st1); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &st2); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &st3); err != nil {
		return err
	}

	// Read filename as length-prefixed string
	var filenameLen uint32
	if err := binary.Read(r, binary.LittleEndian, &filenameLen); err != nil {
		return err
	}

	var diskFilename string
	if filenameLen > 0 && filenameLen < 1024 { // Sanity check
		filenameBytes := make([]byte, filenameLen)
		if _, err := io.ReadFull(r, filenameBytes); err != nil {
			return err
		}
		diskFilename = string(filenameBytes)
	}

	// Enable FDC if it was present
	if present {
		if zx.io.fdc == nil {
			zx.io.EnableFDC()
		}

		// Restore FDC state
		fdc := zx.io.fdc
		fdc.diskPresent = diskPresent
		fdc.diskModified = diskModified
		fdc.diskFilename = diskFilename
		fdc.currentTrack = currentTrack
		fdc.currentHead = currentHead
		fdc.currentSector = currentSector
		fdc.mainStatus = mainStatus
		fdc.st0 = st0
		fdc.st1 = st1
		fdc.st2 = st2
		fdc.st3 = st3
	}

	return nil
}

func loadDiskChunk(zx *ZenZX, r io.Reader) error {
	// Decompress disk image
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	// Ensure FDC exists
	if zx.io.fdc == nil {
		zx.io.EnableFDC()
	}

	// Read disk image
	if _, err := io.ReadFull(gr, zx.io.fdc.diskImage); err != nil {
		return err
	}

	zx.io.fdc.diskPresent = true

	return nil
}
