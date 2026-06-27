//go:build !headless

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ============================================================================
// Keyboard Mapping
// ============================================================================

// SpectrumKey represents a key position on the Spectrum keyboard matrix
type SpectrumKey struct {
	Row uint8
	Col uint8
}

// KeyMapping represents a mapping from PC key to Spectrum key(s)
type KeyMapping struct {
	Key  int32         // Raylib key code
	Keys []SpectrumKey // Spectrum keys to press (for combinations)
}

// ============================================================================
// Input Handler
// ============================================================================

// HandleInput processes all input for the emulator
func (zx *ZenZX) HandleInput() {
	// Handle ESC key to close window (unless disabled)
	if !zx.noEscKey && rl.IsKeyPressed(rl.KeyEscape) {
		rl.CloseWindow()
		return
	}

	// Handle special Alt combinations first (system controls)
	if rl.IsKeyDown(rl.KeyLeftAlt) {
		zx.handleAltCombinations()
	}

	// Process keyboard input
	zx.handleKeyboard()

	// Handle function keys
	zx.handleFunctionKeys()

	// Handle window scaling
	if rl.IsKeyPressed(rl.KeyPageUp) {
		zx.display.ScaleUp()
	}

	if rl.IsKeyPressed(rl.KeyPageDown) {
		zx.display.ScaleDown()
	}

	// Handle dropped files
	zx.handleDroppedFiles()

	// Update border color in display manager
	zx.display.SetBorderColor(zx.io.borderColor)
}

// handleAltCombinations processes Alt+key combinations for system controls
func (zx *ZenZX) handleAltCombinations() {
	if rl.IsKeyPressed(rl.KeyF) {
		zx.display.ToggleFPS()
	}

	if rl.IsKeyPressed(rl.KeyB) {
		zx.display.ToggleBorder()
	}

	if rl.IsKeyPressed(rl.KeyT) {
		// Toggle tape mode (Accurate/Fast)
		if zx.tape != nil && zx.tape.st != nil && zx.tape.st.Loaded {
			if zx.tape.st.Mode == TapeAccurate {
				zx.tape.SetMode(TapeFast)
				fmt.Println("Tape: Fast mode")
			} else {
				zx.tape.SetMode(TapeAccurate)
				fmt.Println("Tape: Accurate mode")
			}
		}
	}

	if rl.IsKeyPressed(rl.KeyP) {
		// Play/Stop tape
		if zx.tape != nil && zx.tape.st != nil && zx.tape.st.Loaded {
			if zx.tape.st.Playing {
				zx.tape.Stop()
				fmt.Println("Tape: Stopped")
			} else {
				zx.tape.Play()
				modeStr := "Fast"
				if zx.tape.st.Mode == TapeAccurate {
					modeStr = "Accurate"
				}
				fmt.Printf("Tape: Playing (%s mode)\n", modeStr)
			}
		}
	}

	if rl.IsKeyPressed(rl.KeyR) {
		// Rewind tape
		if zx.tape != nil && zx.tape.st != nil && zx.tape.st.Loaded {
			zx.tape.Rewind()
			fmt.Println("Tape: Rewound")
		}
	}

	if rl.IsKeyPressed(rl.KeyI) {
		// Show tape info
		if zx.tape != nil && zx.tape.st != nil && zx.tape.st.Loaded {
			status := zx.tape.GetStatus()
			fmt.Printf("Tape: %s\n", status)

			blocks := zx.tape.GetBlockInfo()
			if len(blocks) > 0 {
				fmt.Println("Blocks:")
				for _, info := range blocks {
					fmt.Println(info)
				}
			}
		}
	}
}

// handleKeyboard processes keyboard input and maps to Spectrum keys
func (zx *ZenZX) handleKeyboard() {
	// Clear all keyboard state first
	for i := range zx.io.keyboard {
		zx.io.keyboard[i] = 0x1F // All keys released
	}

	// Define basic key mappings (direct 1:1 mappings)
	basicKeyMap := map[int32][2]uint8{
		// Letters - QWERTY layout
		rl.KeyA: {1, 0}, rl.KeyS: {1, 1}, rl.KeyD: {1, 2}, rl.KeyF: {1, 3}, rl.KeyG: {1, 4},
		rl.KeyQ: {2, 0}, rl.KeyW: {2, 1}, rl.KeyE: {2, 2}, rl.KeyR: {2, 3}, rl.KeyT: {2, 4},
		rl.KeyZ: {0, 1}, rl.KeyX: {0, 2}, rl.KeyC: {0, 3}, rl.KeyV: {0, 4},
		rl.KeyY: {5, 4}, rl.KeyU: {5, 3}, rl.KeyI: {5, 2}, rl.KeyO: {5, 1}, rl.KeyP: {5, 0},
		rl.KeyH: {6, 4}, rl.KeyJ: {6, 3}, rl.KeyK: {6, 2}, rl.KeyL: {6, 1},
		rl.KeyB: {7, 4}, rl.KeyN: {7, 3}, rl.KeyM: {7, 2},

		// Numbers
		rl.KeyOne: {3, 0}, rl.KeyTwo: {3, 1}, rl.KeyThree: {3, 2}, rl.KeyFour: {3, 3}, rl.KeyFive: {3, 4},
		rl.KeySix: {4, 4}, rl.KeySeven: {4, 3}, rl.KeyEight: {4, 2}, rl.KeyNine: {4, 1}, rl.KeyZero: {4, 0},

		// Special keys
		rl.KeySpace: {7, 0},
		rl.KeyEnter: {6, 0},
	}

	// Process combination keys (PC key -> multiple Spectrum keys)

	// Shift keys -> CAPS SHIFT
	if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT (row 0, col 0)
	}

	// Control keys -> Symbol Shift
	if rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT (row 7, col 1)
	}

	// Tab = EXTEND MODE (CAPS SHIFT + SYMBOL SHIFT) - improved mapping
	if rl.IsKeyDown(rl.KeyTab) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
	}

	// Tilde/Backtick = EDIT (CAPS SHIFT + 1) - improved mapping
	if rl.IsKeyDown(rl.KeyGrave) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[3] &^= 0x01 // 1
	}

	// Backspace = DELETE (CAPS SHIFT + 0)
	if rl.IsKeyDown(rl.KeyBackspace) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x01 // 0
	}

	// Delete key also maps to DELETE (alternative)
	if rl.IsKeyDown(rl.KeyDelete) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x01 // 0
	}

	// Caps Lock = CAPS LOCK (CAPS SHIFT + 2)
	if rl.IsKeyDown(rl.KeyCapsLock) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[3] &^= 0x02 // 2
	}

	// Arrow keys with CAPS SHIFT
	if rl.IsKeyDown(rl.KeyLeft) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[3] &^= 0x10 // 5
	}

	if rl.IsKeyDown(rl.KeyDown) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x10 // 6
	}

	if rl.IsKeyDown(rl.KeyUp) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x08 // 7
	}

	if rl.IsKeyDown(rl.KeyRight) {
		zx.io.keyboard[0] &^= 0x01 // CAPS SHIFT
		zx.io.keyboard[4] &^= 0x04 // 8
	}

	// Punctuation and symbols

	// Comma key
	if rl.IsKeyDown(rl.KeyComma) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[7] &^= 0x08 // N
	}

	// Period key
	if rl.IsKeyDown(rl.KeyPeriod) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[7] &^= 0x04 // M
	}

	// Semicolon -> SYMBOL SHIFT + O (;)
	if rl.IsKeyDown(rl.KeySemicolon) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[5] &^= 0x02 // O
	}

	// Apostrophe -> SYMBOL SHIFT + 7 (')
	if rl.IsKeyDown(rl.KeyApostrophe) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[4] &^= 0x08 // 7
	}

	// Slash -> SYMBOL SHIFT + V (/)
	if rl.IsKeyDown(rl.KeySlash) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[0] &^= 0x10 // V
	}

	// Minus -> SYMBOL SHIFT + J (-)
	if rl.IsKeyDown(rl.KeyMinus) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[6] &^= 0x08 // J
	}

	// Equal -> SYMBOL SHIFT + L (=)
	if rl.IsKeyDown(rl.KeyEqual) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[6] &^= 0x02 // L
	}

	// Left bracket -> SYMBOL SHIFT + 8 ([)
	if rl.IsKeyDown(rl.KeyLeftBracket) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[4] &^= 0x04 // 8
	}

	// Right bracket -> SYMBOL SHIFT + 9 (])
	if rl.IsKeyDown(rl.KeyRightBracket) {
		zx.io.keyboard[7] &^= 0x02 // SYMBOL SHIFT
		zx.io.keyboard[4] &^= 0x02 // 9
	}

	// Process basic keys (but don't override combinations)
	// Skip if Alt is held to prevent conflicts with Alt+key combinations
	if !rl.IsKeyDown(rl.KeyLeftAlt) {
		for key, pos := range basicKeyMap {
			if rl.IsKeyDown(key) {
				zx.io.keyboard[pos[0]] &^= (1 << pos[1])
			}
		}
	}
}

// handleFunctionKeys processes function key inputs
func (zx *ZenZX) handleFunctionKeys() {
	// F1 - Reset
	if rl.IsKeyPressed(rl.KeyF1) {
		zx.Reset()
		fmt.Println("Reset")
	}

	// F2 - Pause/Resume
	if rl.IsKeyPressed(rl.KeyF2) {
		zx.paused = !zx.paused
		if zx.paused {
			fmt.Println("Paused")
		} else {
			fmt.Println("Resumed")
		}
	}

	// F3 - Show status
	if rl.IsKeyPressed(rl.KeyF3) {
		zx.showStatus()
	}

	// F4 - Save disk (if modified)
	if rl.IsKeyPressed(rl.KeyF4) {
		zx.handleSaveDisk()
	}

	// F5 - Insert blank disk
	if rl.IsKeyPressed(rl.KeyF5) {
		zx.handleInsertBlankDisk()
	}

	// F6 - Eject disk
	if rl.IsKeyPressed(rl.KeyF6) {
		zx.handleEjectDisk()
	}

	// F7 - Load disk info
	if rl.IsKeyPressed(rl.KeyF7) {
		fmt.Println("To load a disk image:")
		fmt.Println("  Restart with: ./zenzx -model=plus3 -disk=filename.dsk")
		fmt.Println("Or drag and drop a .dsk file (not implemented yet)")
	}

	// F8 - Save disk as new file
	if rl.IsKeyPressed(rl.KeyF8) {
		zx.handleSaveDiskAs()
	}

	// F9 - Quick save snapshot
	if rl.IsKeyPressed(rl.KeyF9) {
		zx.handleQuickSave()
	}

	// F10 - Quick load snapshot
	if rl.IsKeyPressed(rl.KeyF10) {
		zx.handleQuickLoad()
	}

	// F11 - Save timestamped snapshot
	if rl.IsKeyPressed(rl.KeyF11) {
		zx.handleAutoSave()
	}

	// F12 - Load snapshot info / Run diagnostics with Shift
	if rl.IsKeyPressed(rl.KeyF12) {
		if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
			// Shift+F12 - Run diagnostics
			fmt.Println("\nRunning snapshot diagnostics...")
			debugger := NewDebugSnapshot(zx)
			debugger.RunFullDiagnostics()
		} else {
			fmt.Println("Snapshot loading: Drop a .zxs file onto the window")
			fmt.Println("(Hold Shift+F12 for diagnostics)")
		}
	}
}

// showStatus displays current emulator status
func (zx *ZenZX) showStatus() {
	if zx.isPlus3 {
		fmt.Printf("+3 Mode - ROM: %d, RAM@C000: %d, Screen: %d, ",
			zx.memory.romBank, zx.memory.ramBankTop, zx.memory.screenBank)
		if zx.memory.specialMode > 0 {
			fmt.Printf("Special Mode: %d, ", zx.memory.specialMode)
		}
		fmt.Printf("Locked: %v\n", zx.memory.pagingLocked)
		if zx.io.hasFDC && zx.io.fdc != nil {
			if zx.io.fdc.HasDisk() {
				if zx.io.fdc.diskFilename != "" {
					fmt.Printf("Disk: %s (modified: %v)\n", zx.io.fdc.diskFilename, zx.io.fdc.IsModified())
				} else {
					fmt.Printf("Disk: Blank disk (modified: %v)\n", zx.io.fdc.IsModified())
				}
			} else {
				fmt.Println("Disk: No disk inserted")
			}
		}
	} else if zx.is128K {
		fmt.Printf("128K Mode - ROM: %d, RAM@C000: %d, Screen: %d, Locked: %v\n",
			zx.memory.romBank, zx.memory.ramBankTop, zx.memory.screenBank, zx.memory.pagingLocked)
	} else {
		fmt.Println("48K Mode")
	}

	// Show tape status if loaded
	if zx.tape != nil && zx.tape.st != nil && zx.tape.st.Loaded {
		status := zx.tape.GetStatus()
		fmt.Printf("Tape: %s\n", status)
	}

}

// Disk operation handlers
func (zx *ZenZX) handleSaveDisk() {
	if zx.io.hasFDC && zx.io.fdc != nil && zx.io.fdc.IsModified() {
		if zx.io.fdc.diskFilename != "" {
			if err := zx.io.SaveDisk(); err != nil {
				fmt.Printf("Error saving disk: %v\n", err)
			} else {
				fmt.Println("Disk saved successfully")
			}
		} else {
			fmt.Println("Blank disk - use F8 to save as new file")
		}
	} else {
		fmt.Println("No disk to save or disk not modified")
	}
}

func (zx *ZenZX) handleInsertBlankDisk() {
	if zx.io.hasFDC && zx.io.fdc != nil {
		if zx.io.fdc.IsModified() && zx.io.fdc.diskFilename != "" {
			fmt.Println("Saving current disk before inserting blank...")
			zx.io.SaveDisk()
		}

		zx.io.fdc.CreateBlankDisk()
		fmt.Println("Inserted blank formatted disk")
		fmt.Println("Tip: Use SAVE \"filename\" to save BASIC programs")
		fmt.Println("     Use CAT to list disk contents")
		fmt.Println("     Press F8 to save disk image to file")
	} else {
		fmt.Println("FDC not available")
	}
}

func (zx *ZenZX) handleEjectDisk() {
	if zx.io.hasFDC && zx.io.fdc != nil {
		if zx.io.fdc.HasDisk() {
			zx.io.fdc.EjectDisk()
			fmt.Println("Disk ejected")
		} else {
			fmt.Println("No disk to eject")
		}
	} else {
		fmt.Println("FDC not available")
	}
}

func (zx *ZenZX) handleSaveDiskAs() {
	if zx.io.hasFDC && zx.io.fdc != nil && zx.io.fdc.HasDisk() {
		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("disk_%s.dsk", timestamp)

		if err := zx.io.SaveDiskAs(filename); err != nil {
			fmt.Printf("Error saving disk: %v\n", err)
		} else {
			fmt.Printf("Disk saved as: %s\n", filename)
		}
	} else {
		fmt.Println("No disk to save")
	}
}

// handleDroppedFiles processes files dropped onto the window
func (zx *ZenZX) handleDroppedFiles() {
	// Handle screen files (.scr)
	HandleDroppedFiles(zx.screen)

	// Handle snapshot files (.zxs)
	if rl.IsFileDropped() {
		files := rl.LoadDroppedFiles()
		for _, file := range files {
			if len(file) > 4 {
				ext := file[len(file)-4:]

				switch ext {
				case ".zxs", ".ZXS":
					// Pause before loading dropped snapshot
					zx.paused = true

					if err := zx.LoadSnapshot(file); err != nil {
						fmt.Printf("Error loading snapshot %s: %v\n", file, err)
						zx.paused = false // Unpause on error
					} else {
						fmt.Printf("Loaded snapshot: %s\n", file)
						// Don't unpause - let the snapshot determine pause state
					}

				case ".dsk", ".DSK":
					// Handle disk images for +3
					if zx.io.hasFDC && zx.io.fdc != nil {
						if err := zx.io.LoadDisk(file); err != nil {
							fmt.Printf("Error loading disk %s: %v\n", file, err)
						} else {
							fmt.Printf("Loaded disk: %s\n", file)
						}
					} else {
						fmt.Println("Disk loading requires +3 mode with FDC enabled")
					}

				case ".tap", ".TAP", ".tzx", ".TZX":
					// Handle tape files
					if zx.tape == nil {
						zx.tape = NewTape(zx)
						// Enable fast loader
						fl := &FastLoader{Enabled: true}
						zx.tape.AttachFastLoader(fl)
					}

					if err := zx.tape.LoadFile(file); err != nil {
						fmt.Printf("Error loading tape %s: %v\n", file, err)
					} else {
						fmt.Printf("Loaded tape: %s\n", file)
						fmt.Printf("  %d blocks found\n", len(zx.tape.st.Blocks))

						// Auto-rewind and show info
						zx.tape.Rewind()

						// Show first few blocks
						blocks := zx.tape.GetBlockInfo()
						for i, info := range blocks {
							if i >= 5 {
								fmt.Printf("  ... and %d more blocks\n", len(blocks)-5)
								break
							}
							fmt.Printf("  %s\n", info)
						}

						fmt.Println("Tape controls:")
						fmt.Println("  Alt+P: Play/Stop")
						fmt.Println("  Alt+R: Rewind")
						fmt.Println("  Alt+T: Toggle Fast/Accurate mode")
						fmt.Println("  Alt+I: Show tape info")
					}

				case ".sna", ".SNA", ".z80", ".Z80":
					// Handle other snapshot formats
					zx.paused = true

					format := strings.ToLower(strings.TrimPrefix(ext, "."))
					if err := zx.LoadSnapshotFormat(file, format); err != nil {
						fmt.Printf("Error loading snapshot %s: %v\n", file, err)
						zx.paused = false // Unpause on error
					} else {
						fmt.Printf("Loaded %s snapshot: %s\n", strings.ToUpper(format), file)
						// Don't unpause - let the snapshot determine pause state
					}
				}
			}
		}
		rl.UnloadDroppedFiles()
	}
}

// ============================================================================
// Keyboard Helper Functions
// ============================================================================

// PressKey simulates a key press at the given matrix position
func (zx *ZenZX) PressKey(row, col uint8) {
	zx.io.PressKey(row, col)
}

// ReleaseKey simulates a key release at the given matrix position
func (zx *ZenZX) ReleaseKey(row, col uint8) {
	zx.io.ReleaseKey(row, col)
}

// TypeText simulates typing a string of text
// This is useful for auto-typing commands
func (zx *ZenZX) TypeText(text string) {
	// This would need to be implemented with proper timing
	// to simulate actual typing with key press/release delays
	// For now, this is a placeholder
	fmt.Printf("TypeText not yet implemented: %s\n", text)
}

// ============================================================================
// Additional Keyboard Mappings Reference
// ============================================================================

// Spectrum keyboard matrix reference:
// Row 0: CAPS SHIFT, Z, X, C, V
// Row 1: A, S, D, F, G
// Row 2: Q, W, E, R, T
// Row 3: 1, 2, 3, 4, 5
// Row 4: 0, 9, 8, 7, 6
// Row 5: P, O, I, U, Y
// Row 6: ENTER, L, K, J, H
// Row 7: SPACE, SYMBOL SHIFT, M, N, B

// Common key combinations:
// DELETE = CAPS SHIFT + 0
// EDIT = CAPS SHIFT + 1
// CAPS LOCK = CAPS SHIFT + 2
// TRUE VIDEO = CAPS SHIFT + 3
// INV VIDEO = CAPS SHIFT + 4
// Cursor Left = CAPS SHIFT + 5
// Cursor Down = CAPS SHIFT + 6
// Cursor Up = CAPS SHIFT + 7
// Cursor Right = CAPS SHIFT + 8
// GRAPHICS = CAPS SHIFT + 9
// EXTEND MODE = CAPS SHIFT + SYMBOL SHIFT
// BREAK = CAPS SHIFT + SPACE

// Snapshot operation handlers
func (zx *ZenZX) handleQuickSave() {
	wasPaused := zx.paused
	zx.paused = true

	// Determine format and extension
	format := zx.snapshotFormat
	if format == "auto" {
		format = "zxs" // Default for quick save
	}

	extension := "." + format
	filename := "quicksave" + extension

	if err := zx.SaveSnapshotFormat(filename, format); err != nil {
		fmt.Printf("Quick save failed: %v\n", err)
	} else {
		fmt.Printf("Quick saved to %s (%s format)\n", filename, strings.ToUpper(format))
	}

	zx.paused = wasPaused
}

func (zx *ZenZX) handleQuickLoad() {
	// Pause emulator before loading - CRITICAL!
	zx.paused = true

	// Try different formats
	formats := []string{"zxs", "sna", "z80"}
	if zx.snapshotFormat != "auto" && zx.snapshotFormat != "" {
		// Try preferred format first
		formats = append([]string{zx.snapshotFormat}, formats...)
	}

	loaded := false
	for _, format := range formats {
		filename := "quicksave." + format
		if _, err := os.Stat(filename); err == nil {
			if err := zx.LoadSnapshotFormat(filename, format); err == nil {
				fmt.Printf("Quick loaded from %s (%s format)\n", filename, strings.ToUpper(format))
				loaded = true
				break
			}
		}
	}

	if !loaded {
		fmt.Println("No quick save found (tried .zxs, .sna, .z80)")
		zx.paused = false // Unpause on error
	}
	// Note: Don't unpause here - the snapshot load will handle it
	// The loaded state determines if we should be paused or not
}

func (zx *ZenZX) handleAutoSave() {
	wasPaused := zx.paused
	zx.paused = true

	timestamp := time.Now().Format("20060102_150405")

	// Determine format and extension
	format := zx.snapshotFormat
	if format == "auto" {
		format = "zxs" // Default for auto save
	}

	extension := "." + format
	filename := fmt.Sprintf("snapshot_%s%s", timestamp, extension)

	if err := zx.SaveSnapshotFormat(filename, format); err != nil {
		fmt.Printf("Save snapshot failed: %v\n", err)
	} else {
		fmt.Printf("Saved snapshot: %s (%s format)\n", filename, strings.ToUpper(format))
	}

	zx.paused = wasPaused
}
