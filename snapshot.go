package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// Snapshot Format Constants
// ============================================================================

const (
	// Magic header for ZenZX snapshots
	SnapshotMagic = "ZENZX"

	// Version numbers
	SnapshotVersion1       = 1 // Initial version
	SnapshotVersion2       = 2 // Added +3 FDC state
	SnapshotVersion3       = 3 // Added JSON metadata
	CurrentSnapshotVersion = SnapshotVersion3

	// Chunk IDs
	ChunkCPU      = 0x4350 // "CP" - CPU state
	ChunkMem48    = 0x4D34 // "M4" - 48K memory
	ChunkMem128   = 0x4D31 // "M1" - 128K memory banks
	ChunkMemPlus3 = 0x4D33 // "M3" - +3 memory banks
	ChunkScreen   = 0x5343 // "SC" - Screen state
	ChunkIO       = 0x494F // "IO" - I/O state
	ChunkPaging   = 0x5047 // "PG" - Memory paging state
	ChunkFDC      = 0x4644 // "FD" - FDC state (+3 only)
	ChunkDisk     = 0x4453 // "DS" - Disk image (+3 only)
	ChunkAudio    = 0x4155 // "AU" - Audio state
	ChunkEnd      = 0x454E // "EN" - End marker
)

// ============================================================================
// JSON Metadata Structures
// ============================================================================

// SnapshotMetadata contains human-readable snapshot information
type SnapshotMetadata struct {
	Version     int                    `json:"version"`
	Timestamp   time.Time              `json:"timestamp"`
	Model       string                 `json:"model"`
	Description string                 `json:"description,omitempty"`
	Emulator    string                 `json:"emulator"`
	EmulatorVer string                 `json:"emulator_version"`
	CPU         CPUMetadata            `json:"cpu"`
	Memory      MemoryMetadata         `json:"memory"`
	IO          IOMetadata             `json:"io"`
	FDC         *FDCMetadata           `json:"fdc,omitempty"`
	Audio       *AudioMetadata         `json:"audio,omitempty"`
	Checksums   map[string]string      `json:"checksums"`
	ChunkSizes  map[string]int         `json:"chunk_sizes"`
	Debug       map[string]interface{} `json:"debug,omitempty"`
}

// CPUMetadata contains CPU state for debugging
type CPUMetadata struct {
	PC     string `json:"pc"`
	SP     string `json:"sp"`
	AF     string `json:"af"`
	BC     string `json:"bc"`
	DE     string `json:"de"`
	HL     string `json:"hl"`
	IX     string `json:"ix"`
	IY     string `json:"iy"`
	I      string `json:"i"`
	R      string `json:"r"`
	IFF1   bool   `json:"iff1"`
	IFF2   bool   `json:"iff2"`
	IM     int    `json:"im"`
	Halted bool   `json:"halted"`
	Cycles uint64 `json:"cycles"`
}

// MemoryMetadata contains memory configuration
type MemoryMetadata struct {
	Model        string `json:"model"`
	ROMBank      int    `json:"rom_bank"`
	RAMBankTop   int    `json:"ram_bank_top"`
	ScreenBank   int    `json:"screen_bank"`
	PagingLocked bool   `json:"paging_locked"`
	SpecialMode  int    `json:"special_mode,omitempty"`
	Port7FFD     string `json:"port_7ffd,omitempty"`
	Port1FFD     string `json:"port_1ffd,omitempty"`
}

// IOMetadata contains I/O state
type IOMetadata struct {
	BorderColor int    `json:"border_color"`
	Speaker     bool   `json:"speaker"`
	AYRegister  int    `json:"ay_register,omitempty"`
	Keyboard    string `json:"keyboard"`
}

// FDCMetadata contains FDC state
type FDCMetadata struct {
	Present      bool   `json:"present"`
	DiskPresent  bool   `json:"disk_present"`
	DiskModified bool   `json:"disk_modified"`
	DiskFilename string `json:"disk_filename,omitempty"`
	DiskSize     int    `json:"disk_size,omitempty"`
	Track        int    `json:"track"`
	Head         int    `json:"head"`
	Sector       int    `json:"sector"`
}

// AudioMetadata contains audio state for metadata
type AudioMetadata struct {
	Enabled      bool     `json:"enabled"`
	MasterVolume float32  `json:"master_volume"`
	BeeperVolume float32  `json:"beeper_volume"`
	AYVolume     float32  `json:"ay_volume"`
	Speaker      bool     `json:"speaker"`
	AYRegisters  []string `json:"ay_registers,omitempty"`
}

// ============================================================================
// Snapshot Structures (Binary)
// ============================================================================

// SnapshotHeader is the main file header
type SnapshotHeader struct {
	Magic     [5]byte // "ZENZX"
	Version   uint8   // Snapshot format version
	Flags     uint16  // Bit flags for features
	Timestamp uint64  // Unix timestamp
	Model     uint8   // 0=48K, 1=128K, 2=+2, 3=+2A, 4=+3
	Reserved  [7]byte // Reserved for future use
}

// SnapshotFlags bit definitions
const (
	FlagCompressed  = 1 << 0 // Data chunks are gzip compressed
	Flag128K        = 1 << 1 // 128K model
	FlagPlus3       = 1 << 2 // +3 model with 4 ROM banks
	FlagFDC         = 1 << 3 // FDC present and active
	FlagDiskPresent = 1 << 4 // Disk image included
	FlagHasMetadata = 1 << 5 // JSON metadata file exists
)

// ChunkHeader precedes each data chunk
type ChunkHeader struct {
	ID   uint16 // Chunk identifier
	Size uint32 // Size of chunk data (excluding header)
}

// CPUState contains all Z80 registers and state
type CPUState struct {
	// Main registers
	AF, BC, DE, HL uint16
	// Alternate registers
	AF_, BC_, DE_, HL_ uint16
	// Index registers
	IX, IY uint16
	// Special registers
	SP, PC, WZ uint16
	I, R       uint8
	// Interrupt state
	IFF1, IFF2 bool
	IM         uint8
	Halted     bool
	// Timing
	Cycles uint64
	// Additional state
	INT     bool // Current INT line state
	NMI     bool // Current NMI state
	NMIEdge bool // NMI edge detection
}

// PagingState contains memory banking configuration
type PagingState struct {
	ROMBank      uint8
	RAMBankTop   uint8
	ScreenBank   uint8
	PagingLocked bool
	Port7FFD     uint8
	Port1FFD     uint8 // +3 only
	SpecialMode  uint8 // +3 special paging mode
}

// IOState contains I/O device states
type IOState struct {
	BorderColor uint8
	Keyboard    [8]uint8
	AYRegister  uint8
	Speaker     bool
	AYRegisters [16]uint8 // AY register cache
}

// AudioState contains audio subsystem state
type AudioState struct {
	BeeperLevel  bool
	AYRegisters  [16]uint8
	MasterVolume float32
	BeeperVolume float32
	AYVolume     float32
	AudioEnabled bool
}

// FDCState contains FDC controller state (+3 only)
type FDCState struct {
	Present            bool
	DiskPresent        bool
	DiskModified       bool
	DiskFilename       string
	CurrentTrack       uint8
	CurrentHead        uint8
	CurrentSector      uint8
	MainStatus         uint8
	ST0, ST1, ST2, ST3 uint8
}

// ============================================================================
// Helper Functions
// ============================================================================

// calculateChecksum calculates a simple checksum for data verification
func calculateChecksum(data []byte) string {
	var sum uint32
	for i := 0; i < len(data); i += 4 {
		if i+3 < len(data) {
			sum += uint32(data[i]) | (uint32(data[i+1]) << 8) |
				(uint32(data[i+2]) << 16) | (uint32(data[i+3]) << 24)
		} else {
			for j := i; j < len(data); j++ {
				sum += uint32(data[j]) << (8 * uint(j-i))
			}
		}
	}
	return fmt.Sprintf("%08X", sum)
}

// getModelName returns human-readable model name
func getModelName(zx *ZenZX) string {
	if zx.isPlus3 {
		return "Spectrum +3"
	} else if zx.is128K {
		return "Spectrum 128K"
	}
	return "Spectrum 48K"
}

// ============================================================================
// Snapshot Loading and Saving with Metadata
// ============================================================================

// SaveSnapshot saves the complete emulator state to a file with metadata
func (zx *ZenZX) SaveSnapshot(filename string) error {
	// Ensure .zxs extension
	if !strings.HasSuffix(filename, ".zxs") {
		filename += ".zxs"
	}

	// Pause the emulator during save to ensure consistent state
	wasPaused := zx.paused
	zx.paused = true
	defer func() {
		zx.paused = wasPaused // Restore previous pause state
	}()

	// IMPORTANT: Complete any in-progress instruction
	// The Z80 might be mid-instruction with PC already incremented past the opcode.
	// We need to ensure we're at an instruction boundary for clean save/load.
	// Since we're paused, we can't actually step, but we can check if we're
	// in a clean state by examining the CPU's internal state.

	// Note: The Z80 package doesn't expose instruction boundaries directly,
	// so we'll document this as a known limitation. Snapshots taken during
	// multi-byte instruction execution may have PC pointing mid-instruction.

	// Create metadata
	metadata := &SnapshotMetadata{
		Version:     CurrentSnapshotVersion,
		Timestamp:   time.Now(),
		Model:       getModelName(zx),
		Emulator:    "ZenZX",
		EmulatorVer: "1.0",
		Checksums:   make(map[string]string),
		ChunkSizes:  make(map[string]int),
		Debug:       make(map[string]interface{}),
	}

	// Fill CPU metadata
	metadata.CPU = CPUMetadata{
		PC:     fmt.Sprintf("%04X", zx.cpu.PC),
		SP:     fmt.Sprintf("%04X", zx.cpu.SP),
		AF:     fmt.Sprintf("%04X", zx.cpu.AF()),
		BC:     fmt.Sprintf("%04X", zx.cpu.BC()),
		DE:     fmt.Sprintf("%04X", zx.cpu.DE()),
		HL:     fmt.Sprintf("%04X", zx.cpu.HL()),
		IX:     fmt.Sprintf("%04X", zx.cpu.IX()),
		IY:     fmt.Sprintf("%04X", zx.cpu.IY()),
		I:      fmt.Sprintf("%02X", zx.cpu.I),
		R:      fmt.Sprintf("%02X", zx.cpu.R),
		IFF1:   zx.cpu.IFF1,
		IFF2:   zx.cpu.IFF2,
		IM:     int(zx.cpu.IM),
		Halted: zx.cpu.Halted,
		Cycles: zx.cpu.Cycles,
	}

	// Fill Memory metadata
	metadata.Memory = MemoryMetadata{
		Model:        getModelName(zx),
		ROMBank:      int(zx.memory.romBank),
		RAMBankTop:   int(zx.memory.ramBankTop),
		ScreenBank:   int(zx.memory.screenBank),
		PagingLocked: zx.memory.pagingLocked,
		SpecialMode:  int(zx.memory.specialMode),
		Port7FFD:     fmt.Sprintf("%02X", zx.memory.port7FFD),
		Port1FFD:     fmt.Sprintf("%02X", zx.memory.port1FFD),
	}

	// Fill I/O metadata
	keyboardStr := ""
	for i, k := range zx.io.keyboard {
		if i > 0 {
			keyboardStr += " "
		}
		keyboardStr += fmt.Sprintf("%02X", k)
	}
	metadata.IO = IOMetadata{
		BorderColor: int(zx.io.borderColor),
		Speaker:     zx.io.speaker,
		AYRegister:  int(zx.io.ayRegister),
		Keyboard:    keyboardStr,
	}

	// Fill Audio metadata if present
	if zx.audio != nil {
		ayRegs := zx.io.GetAYRegisters()
		ayRegStrs := make([]string, 16)
		for i, reg := range ayRegs {
			ayRegStrs[i] = fmt.Sprintf("%02X", reg)
		}

		metadata.Audio = &AudioMetadata{
			Enabled:      zx.audio.IsEnabled(),
			MasterVolume: zx.audio.GetMasterVolume(),
			BeeperVolume: zx.audio.GetBeeperVolume(),
			AYVolume:     zx.audio.GetAYVolume(),
			Speaker:      zx.io.speaker,
			AYRegisters:  ayRegStrs,
		}
	}

	// Fill FDC metadata if present
	if zx.isPlus3 && zx.io.hasFDC && zx.io.fdc != nil {
		metadata.FDC = &FDCMetadata{
			Present:      true,
			DiskPresent:  zx.io.fdc.HasDisk(),
			DiskModified: zx.io.fdc.IsModified(),
			DiskFilename: zx.io.fdc.diskFilename,
			Track:        int(zx.io.fdc.currentTrack),
			Head:         int(zx.io.fdc.currentHead),
			Sector:       int(zx.io.fdc.currentSector),
		}
		if zx.io.fdc.HasDisk() {
			metadata.FDC.DiskSize = len(zx.io.fdc.diskImage)
		}
	}

	// Debug info
	metadata.Debug["frameCount"] = zx.frameCount
	metadata.Debug["cycleCount"] = zx.cycleCount
	metadata.Debug["paused"] = zx.paused
	metadata.Debug["running"] = zx.running

	// Create binary snapshot data
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating snapshot file: %w", err)
	}
	defer file.Close()

	// Use buffered writer for better performance
	writer := &bytes.Buffer{}

	// Write header
	header := SnapshotHeader{
		Version:   CurrentSnapshotVersion,
		Timestamp: uint64(time.Now().Unix()),
	}
	copy(header.Magic[:], SnapshotMagic)

	// Set model and flags
	if zx.isPlus3 {
		header.Model = 4
		header.Flags |= FlagPlus3 | Flag128K | FlagHasMetadata
		if zx.io.hasFDC {
			header.Flags |= FlagFDC
			if zx.io.fdc != nil && zx.io.fdc.HasDisk() {
				header.Flags |= FlagDiskPresent
			}
		}
	} else if zx.is128K {
		header.Model = 1
		header.Flags |= Flag128K | FlagHasMetadata
	} else {
		header.Model = 0
		header.Flags |= FlagHasMetadata
	}

	if err := binary.Write(writer, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Track chunk data for checksums
	chunks := make(map[string][]byte)

	// Write CPU state chunk
	cpuData := &bytes.Buffer{}
	if err := writeCPUChunk(zx, cpuData); err != nil {
		return fmt.Errorf("writing CPU chunk: %w", err)
	}
	chunks["cpu"] = cpuData.Bytes()
	writer.Write(cpuData.Bytes())

	// Write memory chunks
	memData := &bytes.Buffer{}
	if err := writeMemoryChunks(zx, memData); err != nil {
		return fmt.Errorf("writing memory chunks: %w", err)
	}
	chunks["memory"] = memData.Bytes()
	writer.Write(memData.Bytes())

	// Write screen chunk
	screenData := &bytes.Buffer{}
	if err := writeScreenChunk(zx, screenData); err != nil {
		return fmt.Errorf("writing screen chunk: %w", err)
	}
	chunks["screen"] = screenData.Bytes()
	writer.Write(screenData.Bytes())

	// Write I/O state chunk
	ioData := &bytes.Buffer{}
	if err := writeIOChunk(zx, ioData); err != nil {
		return fmt.Errorf("writing I/O chunk: %w", err)
	}
	chunks["io"] = ioData.Bytes()
	writer.Write(ioData.Bytes())

	// Write paging state for 128K/+3
	if zx.is128K || zx.isPlus3 {
		pagingData := &bytes.Buffer{}
		if err := writePagingChunk(zx, pagingData); err != nil {
			return fmt.Errorf("writing paging chunk: %w", err)
		}
		chunks["paging"] = pagingData.Bytes()
		writer.Write(pagingData.Bytes())
	}

	// Write audio state if available
	if zx.audio != nil {
		audioData := &bytes.Buffer{}
		if err := writeAudioChunk(zx, audioData); err != nil {
			return fmt.Errorf("writing audio chunk: %w", err)
		}
		chunks["audio"] = audioData.Bytes()
		writer.Write(audioData.Bytes())
	}

	// Write FDC state for +3
	if zx.isPlus3 && zx.io.hasFDC && zx.io.fdc != nil {
		fdcData := &bytes.Buffer{}
		if err := writeFDCChunk(zx, fdcData); err != nil {
			return fmt.Errorf("writing FDC chunk: %w", err)
		}
		chunks["fdc"] = fdcData.Bytes()
		writer.Write(fdcData.Bytes())

		// Write disk image if present
		if zx.io.fdc.HasDisk() {
			diskData := &bytes.Buffer{}
			if err := writeDiskChunk(zx, diskData); err != nil {
				return fmt.Errorf("writing disk chunk: %w", err)
			}
			chunks["disk"] = diskData.Bytes()
			writer.Write(diskData.Bytes())
		}
	}

	// Write end marker
	endHeader := ChunkHeader{ID: ChunkEnd, Size: 0}
	if err := binary.Write(writer, binary.LittleEndian, endHeader); err != nil {
		return fmt.Errorf("writing end marker: %w", err)
	}

	// Calculate checksums and sizes for metadata
	for name, data := range chunks {
		metadata.Checksums[name] = calculateChecksum(data)
		metadata.ChunkSizes[name] = len(data)
	}

	// Write binary file
	if _, err := file.Write(writer.Bytes()); err != nil {
		return fmt.Errorf("writing to file: %w", err)
	}

	// Save JSON metadata
	metaFilename := strings.TrimSuffix(filename, ".zxs") + ".json"
	metaData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	if err := os.WriteFile(metaFilename, metaData, 0644); err != nil {
		// Non-fatal: snapshot is still valid without metadata
		fmt.Printf("Warning: Could not save metadata file: %v\n", err)
	} else {
		fmt.Printf("Saved metadata to %s\n", metaFilename)
	}

	fmt.Printf("Snapshot saved: %s (PC=%04X, Halted=%v, Cycles=%d)\n",
		filename, zx.cpu.PC, zx.cpu.Halted, zx.cpu.Cycles)

	return nil
}

// LoadSnapshot loads emulator state from a file with optional metadata validation
func (zx *ZenZX) LoadSnapshot(filename string) error {
	fmt.Printf("Loading snapshot from %s\n", filename)

	// Check for metadata file
	metaFilename := strings.TrimSuffix(filename, ".zxs") + ".json"
	var metadata *SnapshotMetadata

	if metaData, err := os.ReadFile(metaFilename); err == nil {
		metadata = &SnapshotMetadata{}
		if err := json.Unmarshal(metaData, metadata); err != nil {
			fmt.Printf("Warning: Could not parse metadata file: %v\n", err)
			metadata = nil
		} else {
			fmt.Printf("Found metadata: Model=%s, Time=%s, PC=%s, Halted=%v\n",
				metadata.Model, metadata.Timestamp.Format("2006-01-02 15:04:05"),
				metadata.CPU.PC, metadata.CPU.Halted)
		}
	}

	// CRITICAL: Pause the emulator during load to prevent race conditions
	zx.paused = true

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading snapshot file: %w", err)
	}

	reader := bytes.NewReader(data)

	// Read and validate header
	var header SnapshotHeader
	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	if string(header.Magic[:]) != SnapshotMagic {
		return fmt.Errorf("invalid snapshot magic: %s", header.Magic)
	}

	if header.Version > CurrentSnapshotVersion {
		return fmt.Errorf("unsupported snapshot version: %d", header.Version)
	}

	// Save current state that we want to preserve
	currentDisplay := zx.display // Preserve display manager
	currentNoEsc := zx.noEscKey  // Preserve user preferences

	// Clear any pending interrupts before loading new state
	zx.cpu.INT = false
	zx.cpu.NMI = false

	fmt.Printf("Loading snapshot: PC before = 0x%04X, running = %v\n", zx.cpu.PC, zx.running)

	// Track checksums if metadata available
	actualChecksums := make(map[string]string)

	// Process chunks
	for {
		var chunkHeader ChunkHeader
		if err := binary.Read(reader, binary.LittleEndian, &chunkHeader); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("reading chunk header: %w", err)
		}

		if chunkHeader.ID == ChunkEnd {
			break
		}

		// Read chunk data
		chunkData := make([]byte, chunkHeader.Size)
		if _, err := io.ReadFull(reader, chunkData); err != nil {
			return fmt.Errorf("reading chunk data: %w", err)
		}

		// Calculate checksum for validation
		chunkName := getChunkName(chunkHeader.ID)
		if chunkName != "" {
			actualChecksums[chunkName] = calculateChecksum(chunkData)
		}

		// Process chunk based on ID
		chunkReader := bytes.NewReader(chunkData)
		switch chunkHeader.ID {
		case ChunkCPU:
			if err := loadCPUChunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading CPU chunk: %w", err)
			}

		case ChunkMem48:
			if err := loadMem48Chunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading 48K memory: %w", err)
			}

		case ChunkMem128:
			if err := loadMem128Chunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading 128K memory: %w", err)
			}

		case ChunkMemPlus3:
			if err := loadMemPlus3Chunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading +3 memory: %w", err)
			}

		case ChunkScreen:
			if err := loadScreenChunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading screen: %w", err)
			}

		case ChunkIO:
			if err := loadIOChunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading I/O state: %w", err)
			}

		case ChunkPaging:
			if err := loadPagingChunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading paging state: %w", err)
			}

		case ChunkAudio:
			if err := loadAudioChunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading audio state: %w", err)
			}

		case ChunkFDC:
			if err := loadFDCChunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading FDC state: %w", err)
			}

		case ChunkDisk:
			if err := loadDiskChunk(zx, chunkReader); err != nil {
				return fmt.Errorf("loading disk image: %w", err)
			}

		default:
			// Unknown chunk - skip it for forward compatibility
			fmt.Printf("Skipping unknown chunk: 0x%04X\n", chunkHeader.ID)
		}
	}

	// Validate checksums if metadata available
	if metadata != nil && len(metadata.Checksums) > 0 {
		for name, expected := range metadata.Checksums {
			if actual, ok := actualChecksums[name]; ok {
				if actual != expected {
					fmt.Printf("Warning: Checksum mismatch for %s chunk (expected %s, got %s)\n",
						name, expected, actual)
				}
			}
		}
	}

	// Restore preserved state
	zx.running = true           // Ensure emulator is running
	zx.display = currentDisplay // Keep the same display manager
	zx.noEscKey = currentNoEsc  // Keep user preferences

	// Restore pause state from metadata if available
	if metadata != nil {
		if pausedVal, ok := metadata.Debug["paused"].(bool); ok {
			zx.paused = pausedVal
			fmt.Printf("Restored pause state: %v\n", zx.paused)
		} else {
			// If no pause state in metadata, stay paused for safety
			zx.paused = true
		}
	} else {
		// No metadata - stay paused for safety
		zx.paused = true
	}

	fmt.Printf("Snapshot loaded: PC after = 0x%04X, Halted = %v, IFF1 = %v, Cycles = %d\n",
		zx.cpu.PC, zx.cpu.Halted, zx.cpu.IFF1, zx.cpu.Cycles)

	// Log additional info if metadata available
	if metadata != nil {
		fmt.Printf("Snapshot info: Created %s, Model %s, Emulator %s %s\n",
			metadata.Timestamp.Format("2006-01-02 15:04:05"),
			metadata.Model, metadata.Emulator, metadata.EmulatorVer)
	}

	return nil
}

// getChunkName returns a string name for a chunk ID (for checksums)
func getChunkName(id uint16) string {
	switch id {
	case ChunkCPU:
		return "cpu"
	case ChunkMem48, ChunkMem128, ChunkMemPlus3:
		return "memory"
	case ChunkScreen:
		return "screen"
	case ChunkIO:
		return "io"
	case ChunkPaging:
		return "paging"
	case ChunkAudio:
		return "audio"
	case ChunkFDC:
		return "fdc"
	case ChunkDisk:
		return "disk"
	default:
		return ""
	}
}

// ============================================================================
// Quick Save/Load (for F9/F10 keys)
// ============================================================================

// QuickSave saves to a fixed filename for quick save functionality
func (zx *ZenZX) QuickSave() error {
	filename := "quicksave.zxs"
	if err := zx.SaveSnapshot(filename); err != nil {
		return err
	}
	fmt.Printf("Quick saved to %s\n", filename)
	return nil
}

// QuickLoad loads from the quick save file
func (zx *ZenZX) QuickLoad() error {
	filename := "quicksave.zxs"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("no quick save found")
	}
	if err := zx.LoadSnapshot(filename); err != nil {
		return err
	}
	fmt.Printf("Quick loaded from %s\n", filename)
	return nil
}

// AutoSave creates an autosave with timestamp
func (zx *ZenZX) AutoSave() error {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("autosave_%s.zxs", timestamp)
	if err := zx.SaveSnapshot(filename); err != nil {
		return err
	}
	fmt.Printf("Auto-saved to %s\n", filename)
	return nil
}

// ============================================================================
// Diagnostic Functions
// ============================================================================

// ValidateSnapshot checks if a snapshot file is valid and returns metadata
func ValidateSnapshot(filename string) (*SnapshotMetadata, error) {
	// Try to load metadata first
	metaFilename := strings.TrimSuffix(filename, ".zxs") + ".json"
	if metaData, err := os.ReadFile(metaFilename); err == nil {
		metadata := &SnapshotMetadata{}
		if err := json.Unmarshal(metaData, metadata); err == nil {
			return metadata, nil
		}
	}

	// If no metadata, at least validate the binary file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot read snapshot: %w", err)
	}

	reader := bytes.NewReader(data)
	var header SnapshotHeader
	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("invalid header: %w", err)
	}

	if string(header.Magic[:]) != SnapshotMagic {
		return nil, fmt.Errorf("invalid magic: %s", header.Magic)
	}

	// Create basic metadata from header
	metadata := &SnapshotMetadata{
		Version:   int(header.Version),
		Timestamp: time.Unix(int64(header.Timestamp), 0),
		Model:     getModelFromCode(header.Model),
	}

	return metadata, nil
}

// getModelFromCode converts model code to string
func getModelFromCode(code uint8) string {
	switch code {
	case 0:
		return "Spectrum 48K"
	case 1:
		return "Spectrum 128K"
	case 2:
		return "Spectrum +2"
	case 3:
		return "Spectrum +2A"
	case 4:
		return "Spectrum +3"
	default:
		return fmt.Sprintf("Unknown (%d)", code)
	}
}

// ListSnapshots lists all snapshot files in a directory
func ListSnapshots(dir string) ([]string, error) {
	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var snapshots []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".zxs") {
			fullPath := filepath.Join(dir, entry.Name())
			if metadata, err := ValidateSnapshot(fullPath); err == nil {
				info := fmt.Sprintf("%s - %s, %s",
					entry.Name(),
					metadata.Model,
					metadata.Timestamp.Format("2006-01-02 15:04"))
				snapshots = append(snapshots, info)
			} else {
				snapshots = append(snapshots, fmt.Sprintf("%s (invalid: %v)", entry.Name(), err))
			}
		}
	}

	return snapshots, nil
}
