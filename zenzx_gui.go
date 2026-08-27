//go:build !headless

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// resolveExplicitOrPersisted returns explicitValue if flagName was
// actually passed on the command line (per explicitFlags, built from
// flag.Visit after flag.Parse), otherwise persistedValue -- the
// precedence -theme uses against its own persisted settings.json
// value: an explicitly-passed flag always wins outright; an unset one
// falls back to whatever's saved, not to the flag's own hardcoded
// default, so the emulator actually resumes the last session's theme
// when -theme isn't given at all.
func resolveExplicitOrPersisted(explicitFlags map[string]bool, flagName, explicitValue, persistedValue string) string {
	if explicitFlags[flagName] {
		return explicitValue
	}
	return persistedValue
}

// resolveExplicitOrPersistedInt is resolveExplicitOrPersisted's int
// counterpart, for -scale against settings.json's own displayScale.
func resolveExplicitOrPersistedInt(explicitFlags map[string]bool, flagName string, explicitValue, persistedValue int) int {
	if explicitFlags[flagName] {
		return explicitValue
	}
	return persistedValue
}

func main() {
	// Command line flags
	model := flag.String("model", "48k", "Spectrum model: 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3, ts2068")
	romPath := flag.String("rom", "", "Custom ROM bank(s), comma-separated, positionally mapped to bank 0,1,2,3 (up to the model's own bank count); applied on top of -model's standard set, not instead of it")
	rom0 := flag.String("rom0", "", "Override just ROM bank 0, leaving the rest of -model's standard set intact")
	rom1 := flag.String("rom1", "", "Override just ROM bank 1, leaving the rest of -model's standard set intact")
	rom2 := flag.String("rom2", "", "Override just ROM bank 2, leaving the rest of -model's standard set intact")
	rom3 := flag.String("rom3", "", "Override just ROM bank 3 (e.g. +3DOS on a +3), leaving the rest of -model's standard set intact")
	customROMsMenu := flag.Bool("custom-roms-menu", false, "Interactively pick a ROM from -custom-roms-dir and a bank to apply it to")
	customROMsDir := flag.String("custom-roms-dir", CustomROMDir, "Directory scanned by -custom-roms-menu")
	themeFlag := flag.String("theme", "Dark", "Menu bar UI theme: Dark, Light, or Spectrum (case-insensitive)")
	scale := flag.Int("scale", 2, "Initial window scale (1-5)")
	settingsPathFlag := flag.String("settings", "settings.json", "Path to settings.json (persists theme/font/font-zoom/display-scale/fixed-menu-bar across sessions -- whichever of -theme/-scale you don't pass explicitly falls back to whatever's saved there, not to this flag's own default)")
	noBorder := flag.Bool("noborder", false, "Start without border")
	noEsc := flag.Bool("noesc", false, "Disable ESC key (prevent accidental exit)")
	noFdc := flag.Bool("nofdc", false, "Disable FDC emulation for +3")
	disk := flag.String("disk", "", "Path to +3 disk image (.dsk)")
	debugFdc := flag.Bool("debugfdc", false, "Enable FDC debug output")
	snapshot := flag.String("snapshot", "", "Load snapshot on startup")
	snapshotFormat := flag.String("format", "auto", "Snapshot format: auto, zxs, sna, z80")
	tapeFile := flag.String("tape", "", "Load tape file (.tap or .tzx)")
	tapeMode := flag.String("tapemode", "fast", "Tape mode: fast, accurate, or turbo (identical accurate-mode CPU/tape correctness, with per-instruction AMX/audio bookkeeping and per-frame border rendering skipped while the tape is actively loading)")
	noAudio := flag.Bool("noaudio", false, "Disable audio")
	audioBackend := flag.String("audiobackend", "oto", "Audio backend: raylib or oto")
	binFile := flag.String("bin", "", "Load a raw binary blob directly into memory")
	binAddr := flag.String("binaddr", "0x8000", "Load address for -bin (hex 0x.. or decimal)")
	binStart := flag.String("binstart", "", "PC start address after -bin (hex/decimal; empty = use load address; -1 = leave PC unchanged)")
	scrFile := flag.String("scr", "", "Load a raw .scr screen dump onto the display (still image)")
	scriptFile := flag.String("script", "", "Path to a .zen action script to drive the emulator")
	nonStandard := flag.String("non-standard", "off", "Master switch for non-standard features: on or off. Gates all -ns-* flags.")
	nsGraphics := flag.String("ns-graphics", "", "Non-standard graphics mode (requires -non-standard on): "+nsGraphicsUsage)
	nsStorage := flag.String("ns-storage", "", "Non-standard storage backend (requires -non-standard on): "+nsStorageUsage)
	joystick := flag.String("joystick", "auto", "Joystick emulation: auto (the selected -model's own built-in configuration), none, kempston, sinclair (alias for sinclair1), sinclair1, sinclair2, sinclair-both")
	mouse := flag.String("mouse", "none", "Mouse emulation: none or kempston")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ZenZX %s\n", version)
		return
	}

	// Resolve theme/scale precedence: an explicitly-passed -theme/
	// -scale flag wins outright; otherwise fall back to whatever's
	// persisted in settings.json (which itself falls back to its own
	// embedded, hardcoded default if no valid disk file exists at
	// -settings' own path) -- rather than this flag's own "Dark"/2
	// default, so the emulator actually picks up where the last
	// session left off when neither flag is explicitly given.
	explicitFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicitFlags[f.Name] = true })

	settingsRes, err := settingsconfig.Load(*settingsPathFlag, defaultSettingsJSON, validThemeNames, validFontNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if settingsRes.FromDisk {
		fmt.Printf("Settings: loaded %s\n", settingsRes.DiskPath)
	} else if settingsRes.Warning != "" {
		fmt.Println("Warning:", settingsRes.Warning)
	}

	effectiveTheme := resolveExplicitOrPersisted(explicitFlags, "theme", *themeFlag, settingsRes.Settings.Theme)
	effectiveScale := resolveExplicitOrPersistedInt(explicitFlags, "scale", *scale, settingsRes.Settings.DisplayScale)

	nsConfig, err := ParseNonStandardConfig(*nonStandard, *nsGraphics, *nsStorage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if s := nsConfig.Summary(); s != "" {
		fmt.Println(s)
	}
	joystickMode, err := resolveJoystickMode(*joystick, *model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if strings.EqualFold(strings.TrimSpace(*joystick), "auto") && joystickMode != JoystickNone {
		fmt.Printf("Joystick: %s (built into -model=%s)\n", joystickMode, *model)
	}
	mouseMode, err := ParseMouseMode(*mouse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if (joystickMode == JoystickKempston || joystickMode == JoystickKempstonBoth) && mouseMode == MouseAMX {
		fmt.Fprintln(os.Stderr, "Error: -joystick kempston (or kempston-both, which also uses the first Kempston port) and -mouse amx cannot be used together (both use port 0x1F on real hardware)")
		os.Exit(1)
	}

	// Determine audio backend
	var backend AudioBackend
	switch *audioBackend {

	default:
		backend = AudioBackendOto
		fmt.Println("Using Oto audio backend")
	}

	// Create the emulator and resolve the video renderer before the window
	// exists, so InitDisplay can size the window to the renderer's own
	// dimensions (e.g. mode-zenzx-02's 512x384, not the standard 256x192 --
	// see videorender.go).
	zx := NewZenZX(backend)
	zx.nonStandard = nsConfig
	if err := zx.SelectVideoRenderer(nsConfig.Graphics); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	zx.io.SetJoystickMode(joystickMode)
	zx.io.SetMouseMode(mouseMode)

	// Initialize display
	windowWidth, windowHeight := InitDisplay(effectiveScale, !*noBorder, zx.videoRenderer)
	defer rl.CloseWindow()

	// Initialize textures after window is created
	zx.display.InitializeAfterWindow()

	zx.screen.SetMultiplier(effectiveScale)
	zx.display.SetInitialSize(windowWidth, windowHeight)
	if *noBorder {
		zx.display.showBorder = false
	}
	zx.noEscKey = *noEsc

	// Demo overlay: bogus menu/notification/file-dialog for visually
	// confirming the widget system works live over running emulation.
	// Shift+F1/F2/F3 trigger it -- see the main loop below for how input
	// gets redirected away from the emulator while it's active.
	overlay, err := newDemoOverlay()
	if err != nil {
		fmt.Printf("Warning: demo overlay unavailable: %v\n", err)
	}
	if overlay != nil {
		defer overlay.Unload()
	}

	bar, err := newAppMenuBar(zx, *customROMsDir, parseThemeFlag(effectiveTheme), *model, settingsRes.Settings, *settingsPathFlag)
	if err != nil {
		fmt.Printf("Warning: menu bar unavailable: %v\n", err)
	}
	if bar != nil {
		defer bar.Unload()
	}

	// Draws inside Render's own BeginDrawing/EndDrawing bracket -- see
	// preEndDrawHook's doc comment for why this can't just be a plain
	// call after zx.Render() returns. The bar draws after the demo
	// overlay so a dropdown (which can extend below the bar itself)
	// never ends up hidden behind it.
	zx.display.SetPreEndDrawHook(func() {
		screenW, screenH := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
		if overlay != nil && overlay.Active() {
			overlay.Draw(screenW, screenH)
		}
		if bar != nil {
			bar.Draw(screenW, screenH)
		}
	})

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

	// Load ROM based on model
	// -model's own standard ROM set always loads first (no longer gated
	// behind whether -rom was given); -rom/-rom0..-rom3 layer individual
	// bank overrides on top of it afterward. Named ROMs resolve
	// filesystem-first, embedded-fallback (resolveROMBytes in
	// embedded_roms.go) rather than the old hardcoded "./rom/" paths.
	const guiROMDir = "./rom"
	romLoaded := false

	switch *model {
	case "48k":
		if data, err := resolveROMBytes("48.rom", guiROMDir); err == nil {
			if zx.LoadROMBytes(data) == nil {
				fmt.Println("Loaded 48K ROM")
				romLoaded = true
			}
		}

	case "128k":
		if data0, err := resolveROMBytes("128-0.rom", guiROMDir); err == nil {
			if data1, err := resolveROMBytes("128-1.rom", guiROMDir); err == nil {
				if zx.Load128KROMBytes(data0, data1) == nil {
					fmt.Println("Loaded 128K ROM (Sinclair)")
					romLoaded = true
				}
			}
		}

	case "plus2":
		if data0, err := resolveROMBytes("plus2-0.rom", guiROMDir); err == nil {
			if data1, err := resolveROMBytes("plus2-1.rom", guiROMDir); err == nil {
				if zx.Load128KROMBytes(data0, data1) == nil {
					fmt.Println("Loaded Spectrum +2 ROM (grey model)")
					romLoaded = true
				}
			}
		}

	case "plus2a":
		// +2A/+2B: shares the +3's motherboard and uses the exact same 64K
		// +3 ROM set (including +3DOS, which goes unused). The only hardware
		// difference is the absent floppy controller, so we load the +3 ROMs
		// but do NOT enable the FDC. (Previously this pointed at
		// "plus2a-*.rom" files that never existed in rom/, silently falling
		// through to a "not fully accurate" +2-ROM warning every time --
		// fixed here to match the headless main's already-correct approach.)
		if data, err := loadPlus3Bytes(guiROMDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			if zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil {
				fmt.Println("Loaded Spectrum +2A ROM (black model, +3 architecture)")
				romLoaded = true
			}
		}

	case "plus3":
		if data, err := loadPlus3Bytes(guiROMDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			if zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil {
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
		}

	case "spanish48k":
		if data, err := resolveROMBytes("48-spanish.rom", guiROMDir); err == nil {
			if zx.LoadROMBytes(data) == nil {
				fmt.Println("Loaded Spanish 48K ROM")
				romLoaded = true
			}
		}

	case "spanish128k":
		if data0, err := resolveROMBytes("128-spanish-0.rom", guiROMDir); err == nil {
			if data1, err := resolveROMBytes("128-spanish-1.rom", guiROMDir); err == nil {
				if zx.Load128KROMBytes(data0, data1) == nil {
					fmt.Println("Loaded Spanish 128K ROM")
					romLoaded = true
				}
			}
		}

	case "spanishplus2":
		if data0, err := resolveROMBytes("plus2-spanish-0.rom", guiROMDir); err == nil {
			if data1, err := resolveROMBytes("plus2-spanish-1.rom", guiROMDir); err == nil {
				if zx.Load128KROMBytes(data0, data1) == nil {
					fmt.Println("Loaded Spanish Spectrum +2 ROM")
					romLoaded = true
				}
			}
		}

	case "spanishplus3":
		if data, err := loadPlus3Bytes(guiROMDir, "plus3-spanish-0.rom", "plus3-spanish-1.rom", "plus3-spanish-2.rom", "plus3-spanish-3.rom"); err == nil {
			if zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil {
				fmt.Println("Loaded Spanish Spectrum +3 ROM")
				if !*noFdc {
					zx.io.EnableFDC()
				}
				romLoaded = true
			}
		}

	case "ts2068":
		if home, err := resolveROMBytes("ts2068-0.rom", guiROMDir); err == nil {
			if ext, err := resolveROMBytes("ts2068-1.rom", guiROMDir); err == nil {
				if zx.LoadTS2068ROMBytes(home, ext) == nil {
					fmt.Println("Loaded TS2068 ROM (Home + Extension)")
					romLoaded = true
				}
			}
		}

	default:
		fmt.Printf("Unknown model: %s\n", *model)
		fmt.Println("Valid models: 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3, ts2068")
	}

	// If still no ROM loaded, try defaults
	if !romLoaded {
		if data0, err := resolveROMBytes("128-0.rom", guiROMDir); err == nil {
			if data1, err := resolveROMBytes("128-1.rom", guiROMDir); err == nil {
				if zx.Load128KROMBytes(data0, data1) == nil {
					fmt.Println("Loaded 128K ROM (default)")
					romLoaded = true
				}
			}
		}
	}
	if !romLoaded {
		if data, err := resolveROMBytes("48.rom", guiROMDir); err == nil {
			if zx.LoadROMBytes(data) == nil {
				fmt.Println("Loaded 48K ROM (default)")
				romLoaded = true
			}
		}
	}
	if !romLoaded {
		fmt.Println("Error: No ROM files found in ./rom/ or embedded in the binary")
		fmt.Println("Please ensure ROM files are present")
		return
	}

	// --- ROM bank overrides --------------------------------------------
	// Layered on top of the standard -model set just loaded, not instead
	// of it -- -rom's positions fill in from bank 0 first, then any
	// -romN individually overrides that specific bank regardless of what
	// -rom already did.
	if *romPath != "" {
		for i, p := range strings.Split(*romPath, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if err := zx.OverrideROMBank(i, p); err != nil {
				fmt.Printf("Warning: -rom bank %d: %v\n", i, err)
				continue
			}
			fmt.Printf("ROM bank %d overridden: %s\n", i, p)
		}
	}
	for bank, p := range map[int]*string{0: rom0, 1: rom1, 2: rom2, 3: rom3} {
		if *p == "" {
			continue
		}
		if err := zx.OverrideROMBank(bank, *p); err != nil {
			fmt.Printf("Warning: -rom%d: %v\n", bank, err)
			continue
		}
		fmt.Printf("ROM bank %d overridden: %s\n", bank, *p)
	}

	// --- Interactive custom ROM menu ------------------------------------
	// Applied last, after every other ROM-loading path -- the most
	// specific layer, and the one a person driving the menu interactively
	// expects to win over whatever -model/-rom/-romN already set up. The
	// GUI build draws this through the window (custom_roms_menu_gui.go)
	// now that it has a live GL context to do so; the headless build has
	// no window to draw into and keeps the stdin prompt.
	if *customROMsMenu {
		runGraphicalCustomROMMenu(zx, *customROMsDir)
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
			switch strings.ToLower(*tapeMode) {
			case "accurate":
				zx.tape.SetMode(TapeAccurate)
				fmt.Println("Tape mode: Accurate")
			case "turbo":
				zx.tape.SetMode(TapeTurbo)
				fmt.Println("Tape mode: Turbo")
			default:
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

	// Action script (.zen): drive the emulator from a timed action list.
	var sched *Scheduler
	if *scriptFile != "" {
		script, err := ParseScriptFile(*scriptFile)
		if err != nil {
			fmt.Printf("Error parsing script: %v\n", err)
			return
		}
		sched = NewScheduler(script, ScheduleConfig{
			ShotDir:    ".",
			ShotPrefix: "zenzx",
			Quiet:      false,
			Model:      *model,
		})
		fmt.Printf("Loaded script: %s (%d actions)\n", *scriptFile, len(script.Actions))
	}

	// Main loop
	// Auto-commit a modified disk after it has been quiet for ~30s (50 fps).
	const autoCommitDebounceFrames = 30 * 50
	frameNo := 0
	for !rl.WindowShouldClose() || zx.noEscKey {
		// If ESC is disabled, only check for window close button
		if zx.noEscKey && rl.WindowShouldClose() && !rl.IsKeyPressed(rl.KeyEscape) {
			break
		}

		if sched != nil {
			sched.Tick(zx)
			if sched.QuitRequested() {
				rl.CloseWindow()
				break
			}
		}

		// Demo overlay trigger keys -- only checked when nothing is
		// already active, so Shift+F1 while the menu is open doesn't
		// stomp it with a fresh one. Also gated on the bar not being
		// active, so its own dropdowns don't get stomped either.
		if overlay != nil && !overlay.Active() && (bar == nil || !bar.Active()) {
			shift := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
			switch {
			case shift && rl.IsKeyPressed(rl.KeyF1):
				overlay.TriggerMenu()
			case shift && rl.IsKeyPressed(rl.KeyF2):
				overlay.TriggerNotification()
			case shift && rl.IsKeyPressed(rl.KeyF3):
				overlay.TriggerDialog()
			}
		}

		// The emulated machine receives no input while a demo widget or
		// the menu bar owns the frame -- HandleInput is skipped outright
		// rather than filtered, so nothing leaks through by accident.
		guiHasInput := (overlay != nil && overlay.Active()) || (bar != nil && bar.Active())
		if !guiHasInput {
			zx.HandleInput()
		}
		// RunTurboAwareFrame owns the whole Turbo-mode lifecycle (fast-path
		// memory/IO snapshot-in, per-block sync, snapshot-out once the tape
		// finishes) and falls through to plain RunFrame in every other case
		// -- see turbo.go. Same pattern as zenzx_headless.go. One real,
		// user-visible consequence worth knowing: turbo's screen buffer is
		// frozen throughout the fast path by design (the real render only
		// happens on the first call after Playing clears), so the GUI will
		// visibly freeze during a turbo-mode load rather than show
		// incremental loading stripes, then jump to the final state --
		// different from accurate mode's live redraw, not a bug.
		blockedOnRealMemory := sched != nil && sched.BlockingOnRealMemory()
		zx.RunTurboAwareFrame(blockedOnRealMemory)
		zx.Render()

		// Emulation above always ran regardless of any GUI overlay -- the
		// Spectrum keeps stepping and its own screen keeps updating even
		// while a demo widget or the menu bar is shown, only its input
		// was withheld this frame. Drawing already happened inside
		// zx.Render() via the preEndDrawHook set up above; Update just
		// needs to run once per frame so animation timers (the toast's
		// dt, the bar's slide/idle timers) stay correct.
		if overlay != nil {
			screenW, screenH := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
			overlay.Update(screenW, screenH)
		}
		if bar != nil {
			bar.Update(zx)
		}

		frameNo++
		zx.io.FDCTick(frameNo, autoCommitDebounceFrames)
	}

	// Clean up
	// Commit any unsaved disk changes before exiting, so writes made during
	// the session (e.g. a BASIC SAVE) persist to the .dsk file without needing
	// a manual save. Only writes back to an existing file; a blank disk with no
	// filename is left for the user to save explicitly (F8).
	if zx.io.hasFDC && zx.io.fdc != nil && zx.io.fdc.IsModified() && zx.io.fdc.diskFilename != "" {
		if err := zx.io.SaveDisk(); err != nil {
			fmt.Printf("Warning: failed to save disk on exit: %v\n", err)
		} else {
			fmt.Println("Disk changes saved on exit")
		}
	}

	zx.display.CleanupTextures()
	fmt.Println("ZenZX Stopped")
}
