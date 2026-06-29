package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ha1tch/zentools/pkg/snapshot"
)

// ============================================================================
// SNA Format Support (48K and 128K)
// ============================================================================

// SaveSNA saves the current state in .sna format
// SaveSNA saves the current state in .sna format, delegating to
// zentools/pkg/snapshot (the verified codec). Replaces the previous in-tree
// implementation, which lost PC in 48K output.
func (zx *ZenZX) SaveSNA(filename string) error {
	if !strings.HasSuffix(filename, ".sna") {
		filename += ".sna"
	}
	wasPaused := zx.paused
	zx.paused = true
	defer func() { zx.paused = wasPaused }()

	s := zx.toMachineState()
	var image []byte
	var err error
	if s.Model.Is128KFamily() {
		image, err = snapshot.EncodeSNA128(s)
	} else {
		image, err = snapshot.EncodeSNA(s)
	}
	if err != nil {
		return fmt.Errorf("encode .sna: %w", err)
	}
	return os.WriteFile(filename, image, 0o644)
}

// LoadSNA loads a .sna snapshot, delegating to zentools/pkg/snapshot.
func (zx *ZenZX) LoadSNA(filename string) error {
	image, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var s *snapshot.MachineState
	// 48K .sna is 49179 bytes; 128K is larger. Let the decoder pick by size.
	if len(image) > 49179 {
		s, err = snapshot.DecodeSNA128(image)
	} else {
		s, err = snapshot.DecodeSNA(image)
	}
	if err != nil {
		return fmt.Errorf("decode .sna: %w", err)
	}
	zx.fromMachineState(s)
	zx.resyncAfterLoad()
	return nil
}

// SaveZ80 saves the current state in .z80 format (version 3, 128K-capable),
// delegating to zentools/pkg/snapshot.
func (zx *ZenZX) SaveZ80(filename string) error {
	if !strings.HasSuffix(filename, ".z80") {
		filename += ".z80"
	}
	wasPaused := zx.paused
	zx.paused = true
	defer func() { zx.paused = wasPaused }()

	image, err := snapshot.EncodeZ80v3(zx.toMachineState())
	if err != nil {
		return fmt.Errorf("encode .z80: %w", err)
	}
	return os.WriteFile(filename, image, 0o644)
}

// LoadZ80 loads a .z80 snapshot (v1/v2/v3, 48K/128K), delegating to
// zentools/pkg/snapshot.
func (zx *ZenZX) LoadZ80(filename string) error {
	image, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	s, err := snapshot.DecodeZ80(image)
	if err != nil {
		return fmt.Errorf("decode .z80: %w", err)
	}
	zx.fromMachineState(s)
	zx.resyncAfterLoad()
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
