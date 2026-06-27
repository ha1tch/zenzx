//go:build !headless

package main

import (
	"flag"
	"fmt"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	// Command line flags
	model := flag.String("model", "48k", "Spectrum model: 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2")
	romPath := flag.String("rom", "", "Path to custom ROM file(s)")
	scale := flag.Int("scale", 2, "Initial window scale (1-5)")
	noBorder := flag.Bool("noborder", false, "Start without border")
	noEsc := flag.Bool("noesc", false, "Disable ESC key (prevent accidental exit)")
	noFdc := flag.Bool("nofdc", false, "Disable FDC emulation for +3")
	disk := flag.String("disk", "", "Path to +3 disk image (.dsk)")
	debugFdc := flag.Bool("debugfdc", false, "Enable FDC debug output")
	snapshot := flag.String("snapshot", "", "Load snapshot on startup")
	snapshotFormat := flag.String("format", "auto", "Snapshot format: auto, zxs, sna, z80")
	tapeFile := flag.String("tape", "", "Load tape file (.tap or .tzx)")
	tapeMode := flag.String("tapemode", "fast", "Tape mode: fast or accurate")
	noAudio := flag.Bool("noaudio", false, "Disable audio")
	audioBackend := flag.String("audiobackend", "oto", "Audio backend: raylib or oto")
	binFile := flag.String("bin", "", "Load a raw binary blob directly into memory")
	binAddr := flag.String("binaddr", "0x8000", "Load address for -bin (hex 0x.. or decimal)")
	binStart := flag.String("binstart", "", "PC start address after -bin (hex/decimal; empty = use load address; -1 = leave PC unchanged)")
	scrFile := flag.String("scr", "", "Load a raw .scr screen dump onto the display (still image)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ZenZX %s\n", version)
		return
	}

	// Initialize display
	windowWidth, windowHeight := InitDisplay(*scale, !*noBorder)
	defer rl.CloseWindow()

	// Determine audio backend
	var backend AudioBackend
	switch *audioBackend {

	default:
		backend = AudioBackendOto
		fmt.Println("Using Oto audio backend")
	}

	// Create emulator with selected backend
	zx := NewZenZX(backend)

	// Initialize textures after window is created
	zx.screen.InitializeAfterWindow()

	zx.screen.SetMultiplier(*scale)
	zx.display.SetInitialSize(windowWidth, windowHeight)
	if *noBorder {
		zx.display.showBorder = false
	}
	zx.noEscKey = *noEsc

	// Initialize audio after window creation (unless disabled)
	if !*noAudio && zx.audio != nil {
		if err := zx.audio.Initialize(); err != nil {
			fmt.Printf("Warning: Audio initialization failed: %v\n", err)
			zx.audio.SetEnabled(false)
		} else {
			fmt.Println("Audio: Initialized")
		}
		defer func() {
			if zx.audio != nil {
				zx.audio.Close()
			}
		}()
	} else {
		if zx.audio != nil {
			zx.audio.SetEnabled(false)
		}
		fmt.Println("Audio: Disabled")
	}

	// Track if a snapshot will be loaded
	snapshotToLoad := ""
	if *snapshot != "" {
		snapshotToLoad = *snapshot
	}

	// Only reset CPU if we're NOT loading a snapshot
	if snapshotToLoad == "" {
		zx.cpu.Reset()
	}

	// Load ROM based on model or custom path
	romLoaded := false

	if *romPath != "" {
		// Custom ROM path specified
		if err := zx.LoadROM(*romPath); err == nil {
			fmt.Printf("Loaded custom ROM: %s\n", *romPath)
			romLoaded = true
		} else {
			fmt.Printf("Error loading custom ROM %s: %v\n", *romPath, err)
		}
	}

	if !romLoaded {
		// Load ROM based on model
		switch *model {
		case "48k":
			if err := zx.LoadROM("./rom/48.rom"); err == nil {
				fmt.Println("Loaded 48K ROM")
				romLoaded = true
			}

		case "128k":
			if err := zx.Load128KROM("./rom/128-0.rom", "../rom/128-1.rom"); err == nil {
				fmt.Println("Loaded 128K ROM (Sinclair)")
				romLoaded = true
			}

		case "plus2":
			if err := zx.Load128KROM("./rom/plus2-0.rom", "../rom/plus2-1.rom"); err == nil {
				fmt.Println("Loaded Spectrum +2 ROM (grey model)")
				romLoaded = true
			}

		case "plus2a":
			if err := zx.LoadPlus3ROM("./rom/plus2a-0.rom", "../rom/plus2a-1.rom",
				"../rom/plus2a-2.rom", "../rom/plus2a-3.rom"); err == nil {
				fmt.Println("Loaded Spectrum +2A ROM (black model, +3 architecture)")
				romLoaded = true
			} else {
				if err := zx.Load128KROM("./rom/plus2-0.rom", "./rom/plus2-1.rom"); err == nil {
					fmt.Println("Warning: Loaded +2 ROM for +2A model (not fully accurate)")
					romLoaded = true
				}
			}

		case "plus3":
			if err := zx.LoadPlus3ROM("./rom/plus3-0.rom", "./rom/plus3-1.rom",
				"./rom/plus3-2.rom", "./rom/plus3-3.rom"); err == nil {
				fmt.Println("Loaded Spectrum +3 ROM")

				if !*noFdc {
					if *debugFdc {
						zx.io.SetFDCDebug(true)
						fmt.Println("FDC debug output enabled")
					}
					zx.io.EnableFDC()
					fmt.Printf("FDC emulation enabled for +3\n")
				} else {
					fmt.Println("FDC emulation disabled")
				}

				if *disk != "" {
					if err := zx.io.LoadDisk(*disk); err == nil {
						fmt.Printf("Loaded disk image: %s\n", *disk)
					} else {
						fmt.Printf("Warning: Failed to load disk %s: %v\n", *disk, err)
					}
				}
				romLoaded = true
			}

		case "spanish48k":
			if err := zx.LoadROM("./rom/48-spanish.rom"); err == nil {
				fmt.Println("Loaded Spanish 48K ROM")
				romLoaded = true
			}

		case "spanish128k":
			if err := zx.Load128KROM("./rom/128-spanish-0.rom", "./rom/128-spanish-1.rom"); err == nil {
				fmt.Println("Loaded Spanish 128K ROM")
				romLoaded = true
			}

		case "spanishplus2":
			if err := zx.Load128KROM("./rom/plus2-spanish-0.rom", "./rom/plus2-spanish-1.rom"); err == nil {
				fmt.Println("Loaded Spanish Spectrum +2 ROM")
				romLoaded = true
			}

		case "spanishplus3":
			if err := zx.LoadPlus3ROM("./rom/plus3-spanish-0.rom", "./rom/plus3-spanish-1.rom",
				"./rom/plus3-spanish-2.rom", "./rom/plus3-spanish-3.rom"); err == nil {
				fmt.Println("Loaded Spanish Spectrum +3 ROM")
				if !*noFdc {
					zx.io.EnableFDC()
				}
				romLoaded = true
			}

		default:
			fmt.Printf("Unknown model: %s\n", *model)
			fmt.Println("Valid models: 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3")
		}
	}

	// If still no ROM loaded, try defaults
	if !romLoaded {
		if err := zx.Load128KROM("./rom/128-0.rom", "./rom/128-1.rom"); err == nil {
			fmt.Println("Loaded 128K ROM (default)")
			romLoaded = true
		} else if err := zx.LoadROM("./rom/48.rom"); err == nil {
			fmt.Println("Loaded 48K ROM (default)")
			romLoaded = true
		} else {
			fmt.Println("Error: No ROM files found in ../rom/")
			fmt.Println("Please ensure ROM files are present")
			return
		}
	}

	// Now load snapshot if specified
	if snapshotToLoad != "" {
		// Determine format
		format := *snapshotFormat
		if format == "auto" {
			format = DetectSnapshotFormat(snapshotToLoad)
			if format == "" {
				fmt.Printf("Cannot detect format for %s, trying ZXS\n", snapshotToLoad)
				format = "zxs"
			}
		}

		if err := zx.LoadSnapshotFormat(snapshotToLoad, format); err != nil {
			fmt.Printf("Error loading snapshot %s: %v\n", snapshotToLoad, err)
			// Reset CPU on error since we didn't reset it earlier
			zx.cpu.Reset()
		} else {
			fmt.Printf("Loaded %s snapshot: %s\n", strings.ToUpper(format), snapshotToLoad)
			// The snapshot will have set the appropriate CPU state
		}
	} else {
		// If no snapshot loaded, reset the CPU now
		zx.cpu.Reset()
	}

	// Store the preferred snapshot format for F9/F10 operations
	zx.snapshotFormat = *snapshotFormat

	// Load tape if specified
	if *tapeFile != "" && zx.tape != nil {
		if err := zx.tape.LoadFile(*tapeFile); err != nil {
			fmt.Printf("Error loading tape %s: %v\n", *tapeFile, err)
		} else {
			fmt.Printf("Loaded tape: %s\n", *tapeFile)

			// Set tape mode
			if strings.ToLower(*tapeMode) == "accurate" {
				zx.tape.SetMode(TapeAccurate)
				fmt.Println("Tape mode: Accurate")
			} else {
				zx.tape.SetMode(TapeFast)
				fmt.Println("Tape mode: Fast")
			}

			// Show first few blocks
			blocks := zx.tape.GetBlockInfo()
			if len(blocks) > 0 {
				fmt.Printf("Found %d blocks:\n", len(blocks))
				for i, info := range blocks {
					if i >= 3 {
						fmt.Printf("  ... and %d more blocks\n", len(blocks)-3)
						break
					}
					fmt.Printf("  %s\n", info)
				}
			}
		}
	}

	// Load raw binary directly into memory if specified
	if *binFile != "" {
		addr, err := ParseAddr(*binAddr)
		if err != nil {
			fmt.Printf("Invalid -binaddr %q: %v\n", *binAddr, err)
		} else {
			start := int(addr)
			if *binStart != "" {
				if s, err := ParseAddrSigned(*binStart); err != nil {
					fmt.Printf("Invalid -binstart %q: %v\n", *binStart, err)
					start = int(addr)
				} else {
					start = s
				}
			}
			if err := zx.LoadBIN(*binFile, addr, start); err != nil {
				fmt.Printf("Error loading binary %s: %v\n", *binFile, err)
			} else if start >= 0 {
				fmt.Printf("Loaded binary: %s at 0x%04X, PC=0x%04X\n", *binFile, addr, start)
			} else {
				fmt.Printf("Loaded binary: %s at 0x%04X (PC unchanged)\n", *binFile, addr)
			}
		}
	}

	// Load a .scr screen dump if specified
	if *scrFile != "" {
		if err := zx.LoadSCR(*scrFile); err != nil {
			fmt.Printf("Error loading screen %s: %v\n", *scrFile, err)
		} else {
			fmt.Printf("Loaded screen: %s\n", *scrFile)
		}
	}

	fmt.Printf("ZenZX Started - %s mode\n", *model)
	fmt.Println("Controls:")
	fmt.Println("  F1: Reset | F2: Pause | F3: Status | PgUp/PgDn: Scale")
	fmt.Println("  F9: Quick Save | F10: Quick Load | F11: Save Snapshot | F12: Load info (Shift+F12: diagnostics)")
	fmt.Println("  Alt+F: Toggle FPS | Alt+B: Toggle Border")
	if zx.tape != nil {
		fmt.Println("\nTape Controls:")
		fmt.Println("  Alt+P: Play/Stop | Alt+R: Rewind | Alt+T: Toggle mode | Alt+I: Info")
	}

	if zx.isPlus3 && zx.io.hasFDC {
		fmt.Println("\nDisk Operations:")
		fmt.Println("  F4: Save disk | F5: Insert blank | F6: Eject | F7: Load info | F8: Save as")
	}
	fmt.Printf("\nSnapshot format: %s (use -format flag to change)\n", *snapshotFormat)
	fmt.Printf("Audio backend: %s (use -audiobackend flag to change)\n", *audioBackend)
	fmt.Println("\nUsage examples:")
	fmt.Println("  ./zenzx -model=48k")
	fmt.Println("  ./zenzx -model=128k")
	fmt.Println("  ./zenzx -model=plus3 -disk=game.dsk")
	fmt.Println("  ./zenzx -snapshot=saved.sna -format=sna")
	fmt.Println("  ./zenzx -snapshot=game.z80 -format=auto")
	fmt.Println("  ./zenzx -tape=game.tap -tapemode=fast")
	fmt.Println("  ./zenzx -noaudio  (disable audio)")

	// Main loop
	for !rl.WindowShouldClose() || zx.noEscKey {
		// If ESC is disabled, only check for window close button
		if zx.noEscKey && rl.WindowShouldClose() && !rl.IsKeyPressed(rl.KeyEscape) {
			break
		}

		zx.HandleInput()
		zx.RunFrame()
		zx.Render()
	}

	// Clean up
	zx.screen.CleanupTextures()
	fmt.Println("ZenZX Stopped")
}
