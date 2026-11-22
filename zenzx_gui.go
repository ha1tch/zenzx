//go:build !headless

package zenzx

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
// fs.Visit after fs.Parse), otherwise persistedValue -- the
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

// escQueryArmedAt is when the first Escape of a -escape=query confirmation was
// pressed, in rl.GetTime() seconds; 0 means unarmed. A second Escape within
// escQueryWindow confirms the quit.
var escQueryArmedAt float64

const escQueryWindow = 2.0 // seconds to press Escape again to confirm a quit

// confirmQuit implements -escape=query: the first Escape arms a short window
// and shows a hint in the window title; a second Escape inside that window
// returns true (quit). Outside the window the first press re-arms. It is
// non-blocking -- the emulator keeps running between the two presses.
func confirmQuit(zx *ZenZX) bool {
	now := rl.GetTime()
	if escQueryArmedAt != 0 && now-escQueryArmedAt <= escQueryWindow {
		escQueryArmedAt = 0
		return true
	}
	escQueryArmedAt = now
	rl.SetWindowTitle("ZenZX -- press Esc again to quit")
	return false
}

// RunGUI is the ZenZX program with its window: it parses args (the command
// line without the program's name), runs until the window is closed, and
// exits the process on a bad argument. cmd/zenzx is this and nothing else.
func RunGUI(args []string) {
	SetLogOutput(os.Stdout)
	SetTraceOutput(os.Stderr)
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	// Command line flags
	model := fs.String("model", "48k", "Spectrum model: 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3, ts2068, next")
	romPath := fs.String("rom", "", "Custom ROM bank(s), comma-separated, positionally mapped to bank 0,1,2,3 (up to the model's own bank count); applied on top of -model's standard set, not instead of it")
	rom0 := fs.String("rom0", "", "Override just ROM bank 0, leaving the rest of -model's standard set intact")
	rom1 := fs.String("rom1", "", "Override just ROM bank 1, leaving the rest of -model's standard set intact")
	rom2 := fs.String("rom2", "", "Override just ROM bank 2, leaving the rest of -model's standard set intact")
	rom3 := fs.String("rom3", "", "Override just ROM bank 3 (e.g. +3DOS on a +3), leaving the rest of -model's standard set intact")
	customROMsMenu := fs.Bool("custom-roms-menu", false, "Interactively pick a ROM from -custom-roms-dir and a bank to apply it to")
	customROMsDir := fs.String("custom-roms-dir", CustomROMDir, "Directory scanned by -custom-roms-menu")
	themeFlag := fs.String("theme", "Dark", "Menu bar UI theme: Dark, Light, or Spectrum (case-insensitive)")
	scale := fs.Int("scale", 2, "Initial window scale (1-5)")
	settingsPathFlag := fs.String("settings", "settings.json", "Path to settings.json (persists theme/font/font-zoom/display-scale/fixed-menu-bar across sessions -- whichever of -theme/-scale you don't pass explicitly falls back to whatever's saved there, not to this flag's own default)")
	noBorder := fs.Bool("noborder", false, "Start without border")
	escapeFlag := fs.String("escape", "on", "Escape key behaviour: on (quit at once), off (never quit), close (close an open dialog or the HUD only), query (ask before quitting)")
	noFdc := fs.Bool("nofdc", false, "Disable FDC emulation for +3")
	disk := fs.String("disk", "", "Path to +3 disk image (.dsk)")
	sdCard := fs.String("sdcard", "", "Path to a raw FAT16/FAT32 SD card image, attached via DivMMC (T-36, docs/TRACKING.md) -- requires -divmmc-rom too. Pages in via automap at the usual RST-vector entry points on ANY model, not just -model=next; does not by itself model the Next's own separate config-mode boot ROM (item 6)")
	divMMCROM := fs.String("divmmc-rom", "", "Path to the DivMMC/ESXDOS boot ROM image (8K) that -sdcard pages in -- a real ESXDOS build, not sourced or embedded by zenzx itself")
	bootROM := fs.String("bootrom", "", "Path to the FPGA config-mode boot ROM image (8K, e.g. nextboot.rom) -- a real, separate first-stage ROM, not enNxtmmc.rom/-divmmc-rom; runs SD init/FAT32/TBBLUE.FW loading before NextREG 0x03 disables it (T-36)")
	sdCardReadOnly := fs.Bool("sdcard-readonly", false, "Write-protect the -sdcard image: the machine reads it and every write is refused. Without this the image is opened for writing and the firmware and NextZXOS change it, as they do a real card")
	divMMCDebug := fs.Bool("divmmc-debug", false, "Log DivMMC automap transitions and SD card command/response frames to stderr")
	nextRegLogFlag := fs.String("nextreg-log", "", "Next: log writes to these NextREG registers to stderr: hex numbers separated by commas, or all")
	nextFilesFlag := fs.String("next-files", "", nextFilesHelp)
	nextClassic := fs.Bool("next-classic", false, nextClassicHelp)
	nextExamplesFlag := fs.Bool("next-examples", false, nextExamplesHelp)
	nextAddFlag := fs.String("next-add", "", nextAddHelp)
	sdCard2 := fs.String("sdcard2", "", sdCard2Help)
	nextNewCard := fs.String("next-new-card", "", nextNewCardHelp)
	nextBootable := fs.String("next-bootable", "", nextBootableHelp)
	nextIssue := fs.Int("next-issue", 5, nextIssueHelp)
	debugFdc := fs.Bool("debugfdc", false, "Enable FDC debug output")
	snapshot := fs.String("snapshot", "", "Load snapshot on startup")
	snapshotFormat := fs.String("format", "auto", "Snapshot format: auto, zxs, sna, z80, szx (load only)")
	tapeFile := fs.String("tape", "", "Load tape file (.tap or .tzx)")
	tapeMode := fs.String("tapemode", "fast", "Tape mode: fast, accurate, or turbo (identical accurate-mode CPU/tape correctness, with per-instruction AMX/audio bookkeeping and per-frame border rendering skipped while the tape is actively loading)")
	noAudio := fs.Bool("noaudio", false, "Disable audio")
	audioBackend := fs.String("audiobackend", "oto", "Audio backend: raylib or oto")
	binFile := fs.String("bin", "", "Load a raw binary blob directly into memory")
	binAddr := fs.String("binaddr", "0x8000", "Load address for -bin (hex 0x.. or decimal)")
	binStart := fs.String("binstart", "", "PC start address after -bin (hex/decimal; empty = use load address; -1 = leave PC unchanged)")
	scrFile := fs.String("scr", "", "Load a raw .scr screen dump onto the display (still image)")
	scriptFile := fs.String("script", "", "Path to a .zen action script to drive the emulator")
	nonStandard := fs.String("non-standard", "off", "Master switch for non-standard features: on or off. Gates all -ns-* flags.")
	nsGraphics := fs.String("ns-graphics", "", "Non-standard graphics mode (requires -non-standard on): "+nsGraphicsUsage)
	nsStorage := fs.String("ns-storage", "", "Non-standard storage backend (requires -non-standard on): "+nsStorageUsage)
	joystick := fs.String("joystick", "auto", "Joystick emulation: auto (the selected -model's own built-in configuration; on a Next, what NextREG 0x05 chooses for each port), none, kempston, kempston2, kempston-both, sinclair (alias for sinclair1), sinclair1, sinclair2, sinclair-both, cursor")
	mouse := fs.String("mouse", "auto", "Mouse emulation: auto (kempston on a Next, none on the other models), none, kempston or amx")
	multiFlag := fs.String("multi", "", multiHelp)
	listenFlag := fs.String("listen", "", listenHelp)
	listenHost := fs.String("listen-host", "127.0.0.1", listenHostHelp)
	autorunFlag := fs.String("autorun", "", autorunHelp)
	layoutFlag := fs.String("layout", "", layoutHelp)
	focusFollow := fs.Bool("focus-follow", false, focusFollowHelp)
	showVersion := fs.Bool("version", false, "Print version and exit")
	fs.Parse(args)

	// Several machines (multi.go). The primary one is set up below as a
	// single machine always was, with -model standing for its model; the
	// others join it once the dock exists.
	models, cardNames, multiErr := parseMulti(*multiFlag, *model)
	if multiErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", multiErr)
		os.Exit(2)
	}
	primary := primaryMachine(models, cardNames)
	*model = models[primary]
	cards, cardFiles, cardErr := nextCardsOf(models, cardNames, *nextFilesFlag)
	if cardErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", cardErr)
		os.Exit(2)
	}
	if cards[primary] != "" && (*sdCard != "" || *nextClassic) {
		fmt.Fprintln(os.Stderr, "Error: -multi names an SD card for this Next: it cannot be used with -sdcard or -next-classic")
		os.Exit(2)
	}
	listenPorts, listenErr := parseListen(*listenFlag, len(models))
	if listenErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", listenErr)
		os.Exit(2)
	}
	autorunFiles, autorunErr := parseAutorun(*autorunFlag, len(models))
	if autorunErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", autorunErr)
		os.Exit(2)
	}
	layoutName, layoutErr := parseLayout(*layoutFlag)
	if layoutErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", layoutErr)
		os.Exit(2)
	}
	if autorunFiles[primary] != "" && (*tapeFile != "" || *snapshot != "") {
		fmt.Fprintln(os.Stderr, "Error: -autorun puts its own tape in the machine and runs it from a cold start: it cannot be used with -tape or -snapshot")
		os.Exit(2)
	}

	if *nextNewCard != "" {
		path, err := newEmptyCard(*nextNewCard, smallestCard)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Made %s: an empty FAT32 card, %d MB\n", path, smallestCard>>20)
		return
	}
	if *nextBootable != "" {
		if err := runNextBootable(*nextFilesFlag, *nextBootable, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *nextExamplesFlag || *nextAddFlag != "" {
		if err := runNextAdd(*nextFilesFlag, *nextExamplesFlag, *nextAddFlag, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

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
	fs.Visit(func(f *flag.Flag) { explicitFlags[f.Name] = true })

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
	mouseMode, err := resolveMouseMode(*mouse, *model)
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
	zx.SetDisplay(NewDisplayManager(zx.screen))
	audioDev := NewOtoAudioDevice()
	zx.audio.SetDevice(audioDev)
	if err := zx.io.setNextRegLog(*nextRegLogFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := zx.memory.SetNextIssue(*nextIssue); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
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
	// Escape is this program's to interpret (the loop below): raylib would
	// close the window on it whatever is open.
	rl.SetExitKey(0)

	// Initialize textures after window is created
	zx.window().InitializeAfterWindow()

	zx.screen.SetMultiplier(effectiveScale)
	zx.window().SetInitialSize(windowWidth, windowHeight)
	if *noBorder {
		zx.window().showBorder = false
	}
	escMode, escOK := parseEscapeMode(*escapeFlag)
	if !escOK {
		fmt.Printf("Warning: unknown -escape %q; using \"on\". Valid: on, off, close, query\n", *escapeFlag)
	}
	zx.escape = escMode

	// Demo overlay: bogus menu/notification/file-dialog for visually
	// confirming the widget system works live over running emulation.
	// Shift+F1/F2/F3 trigger it -- see the main loop below for how input
	// gets redirected away from the emulator while it's active.
	overlay, err := newDemoOverlay()
	if err != nil {
		fmt.Printf("Warning: demo overlay unavailable: %v\n", err)
	}

	// The debugger HUD (hud_gui.go): a command line over the running
	// machine, backtick toggles it. The PoC decodes as Z80 (48K/128K); a
	// Next passes Z80N once the facade exposes the model (a later increment).
	hud, hudErr := newDebugHUD(zx)
	if hudErr != nil {
		fmt.Printf("Warning: debugger HUD unavailable: %v\n", hudErr)
	}

	// The mount drawer (mountdrawer_gui.go): slots to drop files into, shown
	// while a drag gesture is over the window.
	drawer := newMountDrawer(zx)
	defer drawer.Unload()
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

	// The machines of this window and the dock that flips between them
	// (machinedeck_gui.go). It starts with the one machine; the dock adds
	// more, set up as this one was.
	var deck *machineDeck
	if bar != nil {
		deck = newMachineDeck(zx, *model, zx.window(), audioDev, func(m string) *ZenZX {
			nz := NewZenZX(backend)
			nz.escape = escMode
			nz.snapshotFormat = *snapshotFormat
			nz.nonStandard = nsConfig
			switchModelLive(nz, m, bar.romDir)
			nz.io.SetMouseMode(mouseMode)
			if *noAudio {
				nz.audio.SetEnabled(false)
			}
			return nz
		})
		defer deck.Unload()
		bar.cardUse = deck.cardUse
		bar.machineNumber = deck.numberOf
		bar.diskUse = deck.diskUse
		bar.machineAt = deck.machineAt
		bar.layoutState = func() (deckLayout, int) { return deck.tiling, len(deck.machines) }
		bar.setLayout = deck.setLayout
		bar.rotateMachines = deck.rotate
		deck.focusFollow = *focusFollow
		// Several machines: the mount drawer must act on whichever machine is
		// focused (the deck's front) at the moment of a drop, not the one it was
		// built with -- otherwise a file dropped while the second Next is focused
		// still lands on the first. (multidrop)
		drawer.front = deck.Front
	}

	// The other machines of -multi, the links and the autoruns (multi.go).
	machines := []*ZenZX{zx}
	if len(models) > 1 {
		if deck == nil {
			fmt.Fprintln(os.Stderr, "Error: -multi needs the menu bar, which this run does not have")
			os.Exit(2)
		}
		machines = deck.startMachines(models, primary)
	}
	links, linkErr := startLinks(machines, *listenHost, listenPorts)
	if linkErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", linkErr)
		os.Exit(1)
	}
	defer func() {
		for _, l := range links {
			l.Close()
		}
	}()
	for i, l := range links {
		fmt.Printf("Machine %d (%s): link on %s\n", i+1, models[i], l.Addr())
	}
	if err := startAutoruns(machines, models, autorunFiles); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Draws inside Render's own BeginDrawing/EndDrawing bracket -- see
	// preEndDrawHook's doc comment for why this can't just be a plain
	// call after zx.Render() returns. The bar draws after the demo
	// overlay so a dropdown (which can extend below the bar itself)
	// never ends up hidden behind it.
	zx.window().SetPreEndDrawHook(func() {
		screenW, screenH := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
		if overlay != nil && overlay.Active() {
			overlay.Draw(screenW, screenH)
		}
		if deck != nil {
			deck.Draw(bar.renderer, screenW, screenH, bar.theme)
		}
		// The HUD draws after the deck so, when both could be on screen, the
		// HUD is on top. In practice they are mutually exclusive (opening the
		// HUD hides the dock), but drawing last keeps that robust.
		if hud != nil && hud.Visible() {
			hud.Draw(screenW, screenH)
		}
		if drawer != nil {
			if drawer.Visible() {
				drawer.Draw(bar.renderer, bar.theme, screenW, screenH)
			}
			drawer.DrawToast(screenW, screenH)
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

	case "next":
		if data, err := loadPlus3Bytes(guiROMDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			if zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil {
				fmt.Println("Loaded ZX Spectrum Next (+3 ROM base; NextZXOS boots with -bootrom, -divmmc-rom and -sdcard, see docs/MANUAL.md)")
				zx.EnableZ80N()
				zx.memory.EnableNext()
				zx.memory.SetNextSlot(0, 0xFF) // ROM low half -- defers to legacy ROM (readNext)
				zx.memory.SetNextSlot(1, 0xFF) // ROM high half -- ditto
				zx.memory.SetNextSlot(2, 10)   // bank 5 low half (screen) -- classic reset default
				zx.memory.SetNextSlot(3, 11)   // bank 5 high half
				zx.memory.SetNextSlot(4, 4)    // bank 2 low half
				zx.memory.SetNextSlot(5, 5)    // bank 2 high half
				zx.memory.SetNextSlot(6, 0)    // bank 0 low half -- reset default before any 7FFD write
				zx.memory.SetNextSlot(7, 1)    // bank 0 high half
				fmt.Println("Z80N CPU + Next extended MMU enabled")
				romLoaded = true
				// This model shares the +3 ROM base, and T-36 debugging
				// against real ESXDOS/DivMMC firmware found its boot
				// code -- when it can't get anywhere over SD -- falls
				// through to the same +3DOS disk-boot ROM -model=plus3
				// uses. That ROM needs a real drive-status answer, not
				// just an enabled-but-empty controller (Sense Drive
				// Status reports "not ready" with no disk loaded, and
				// +3DOS hangs waiting for that to change); see the
				// headless entry point's identical comment for the
				// full explanation.
				if !*noFdc {
					zx.io.EnableFDC()
					if *disk != "" {
						if err := zx.io.LoadDisk(*disk); err != nil {
							fmt.Printf("Error loading disk %s: %v\n", *disk, err)
						} else {
							fmt.Printf("Loaded disk: %s\n", *disk)
						}
					} else if zx.io.fdc != nil {
						zx.io.fdc.CreateBlankDisk()
						fmt.Println("No -disk given: created a blank formatted disk so +3DOS's drive detection doesn't hang")
					}
				}
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
		fmt.Println("Valid models: 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3, ts2068, next")
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

	// FPGA config-mode boot ROM (T-36, docs/TRACKING.md)
	if *bootROM != "" {
		data, err := os.ReadFile(*bootROM)
		if err != nil {
			fmt.Printf("Error reading -bootrom %s: %v\n", *bootROM, err)
			os.Exit(1)
		}
		zx.memory.SetBootROM(data)
		fmt.Printf("Boot ROM attached: %s (%d bytes, overlay enabled -- disabled by the first NextREG 0x03 write)\n", *bootROM, len(data))
	}

	// DivMMC / SD card (T-36, docs/TRACKING.md)
	if *sdCard != "" || *divMMCROM != "" {
		if *sdCard == "" || *divMMCROM == "" {
			fmt.Println("Error: -sdcard and -divmmc-rom must both be given together")
			os.Exit(1)
		}
		romBytes, err := os.ReadFile(*divMMCROM)
		if err != nil {
			fmt.Printf("Error reading -divmmc-rom %s: %v\n", *divMMCROM, err)
			os.Exit(1)
		}
		// Left open for the process's lifetime -- see the headless
		// entry point's identical comment for why.
		sdFile, sdProtected, err := openSDCardImage(*sdCard, *sdCardReadOnly)
		if err != nil {
			fmt.Printf("Error opening -sdcard %s: %v\n", *sdCard, err)
			os.Exit(1)
		}
		info, err := sdFile.Stat()
		if err != nil {
			fmt.Printf("Error stat'ing -sdcard %s: %v\n", *sdCard, err)
			os.Exit(1)
		}
		zx.EnableDivMMC(sdFile, info.Size(), romBytes)
		zx.io.divMMC.SD().wr.WriteProtected = sdProtected
		if *divMMCDebug {
			zx.io.divMMC.Debug = true
			zx.io.divMMC.SD().Debug = true
		}
		fmt.Printf("DivMMC attached: rom=%s sdcard=%s (%d bytes, write-protected: %v)\n", *divMMCROM, *sdCard, info.Size(), sdProtected)
		if sdProtected && !*sdCardReadOnly {
			fmt.Printf("Warning: %s could not be opened for writing, so the card is write-protected and nothing can be saved to it\n", *sdCard)
		}
	}

	// The Next's boot chain from its folder, or the first-start dialog (nextchain.go).
	if bar != nil {
		bar.nextFilesDir, bar.nextReadOnly, bar.nextDebug, bar.nextClassic = *nextFilesFlag, *sdCardReadOnly, *divMMCDebug, *nextClassic
		bar.nextCard2 = *sdCard2
	}
	if wantNextBootChain(*model, *nextClassic, *bootROM, *divMMCROM, *sdCard, *snapshot, *binFile, *tapeFile, *scrFile) {
		if bar != nil && cards[primary] != "" {
			f := cardFiles
			f.Card = cards[primary] // the card -multi names for this Next
			bar.attachNext(zx, f)
		} else if bar != nil {
			bar.startNext(zx)
		} else if f, ok, note := locateNextFiles(*nextFilesFlag); ok {
			if cards[primary] != "" {
				f.Card = cards[primary]
			}
			protected, err := zx.attachNextBootChain(f, *sdCardReadOnly, *divMMCDebug)
			if err != nil {
				fmt.Printf("Error: the Next boot files in %s could not be used: %v\n", f.Dir, err)
				os.Exit(1)
			}
			fmt.Println(describeNextChain(f, protected))
			if line, err := zx.insertOwnOrGivenCard(*sdCard2, *sdCardReadOnly); err != nil {
				fmt.Printf("Error: the second SD card could not be used: %v\n", err)
				os.Exit(1)
			} else if line != "" {
				fmt.Println(line)
			}
		} else {
			fmt.Println(nextSetupHint("zenzx-headless", note))
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
	fmt.Println("  Alt+F: Toggle FPS | Alt+B: Toggle Border | Alt+M: Machines")
	fmt.Println("  Alt+1..9: focus that machine (with -multi)")
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
	barWasActive := false
	// The machines of -multi take their listed order now that the primary
	// one has been set up through the display it held (multi_gui.go).
	if deck != nil && len(models) > 1 && primary != 0 {
		deck.settleMachines(primary)
		zx = deck.Front()
		bar.setCurrentModel(deck.frontModel())
	}
	// Every Next but the primary one has a card of its own (parseMulti) and
	// boots from it. They come after the primary one so that it is the
	// primary one that gets the card of the second slot, as a Next on its
	// own does.
	if deck != nil {
		for i, m := range models {
			if m == "next" && i != primary {
				f := cardFiles
				f.Card = cards[i]
				bar.attachNext(machines[i], f)
			}
		}
		// -layout tiles the machines at the start (multi.go names the
		// layout; layoutByName, machinedeck_gui.go, turns it into one).
		if l, ok := layoutByName(layoutName); ok && l != layoutSingle {
			deck.setLayout(l)
		}
	}

	for {
		// The window's close button always ends the program. Escape's own
		// behaviour is the -escape mode. In every mode, an Escape that a menu
		// or a bar window owns closes that, not the program (uiOwnsEscape); the
		// frame after one in which something was open counts too, since the bar
		// takes its keys later in the frame than this. Beyond that: a HUD that
		// is open is closed by Escape first in every mode but "off". Then, with
		// nothing open to close, "on" quits, "close" and "off" do nothing, and
		// "query" asks for confirmation before quitting.
		barActive := bar.Active() || (deck != nil && deck.Active())
		zx.uiOwnsEscape = barActive || barWasActive
		barWasActive = barActive
		if rl.WindowShouldClose() {
			break
		}
		if rl.IsKeyPressed(rl.KeyEscape) && !zx.uiOwnsEscape {
			quit := false
			switch {
			case hud != nil && hud.Active() && zx.escape != escapeOff:
				// Escape closes an open HUD first, whatever the quit mode.
				hud.Close()
			case zx.escape == escapeOn:
				quit = true
			case zx.escape == escapeQuery:
				quit = confirmQuit(zx)
			case zx.escape == escapeClose, zx.escape == escapeOff:
				// Nothing open to close, and these modes never quit.
			}
			if quit {
				break
			}
		}

		if sched != nil {
			sched.Tick(zx)
			zx.scriptKeys = sched.OwnsKeyboard()
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

		// The debugger HUD toggles on backtick when no other GUI element
		// owns the frame. While it is open it takes keyboard input and the
		// machine is paused (see the RunFrame gate below).
		if hud != nil && (bar == nil || !bar.Active()) && (overlay == nil || !overlay.Active()) {
			altHeld := rl.IsKeyDown(rl.KeyLeftAlt) || rl.IsKeyDown(rl.KeyRightAlt)
			if altHeld && rl.IsKeyPressed(rl.KeyD) {
				if !hud.Active() {
					// Opening: in -multi, bind to the front machine and hide the
					// dock -- the HUD and the dock are mutually exclusive, so one
					// never paints over or steals focus from the other.
					if deck != nil {
						hud.AttachMachine(deck.Front(), deck.frontCaption())
						deck.hide()
					}
					hud.Toggle()
				} else {
					hud.Close()
				}
			}
		}
		hudPointer := false
		if hud != nil && hud.Active() {
			hud.HandleInput()
			hudPointer = hud.UpdateResize(int(rl.GetScreenWidth()), int(rl.GetScreenHeight()))
		}
		if hud != nil {
			hud.UpdateAnim() // advances the open/close slide, no-op when settled
		}
		if drawer != nil {
			// The drawer must not arm on a drag that another widget owns: a HUD
			// resize or hover, an open HUD, menu or dock, or a demo overlay. Any
			// of those "takes" the pointer, so the drag is not a mount gesture.
			pointerTaken := hudPointer ||
				(hud != nil && hud.Visible()) ||
				(bar != nil && bar.Active()) ||
				(deck != nil && deck.Active()) ||
				(overlay != nil && overlay.Active())
			drawer.Update(pointerTaken) // arms on a drag gesture, advances its slide
			drawer.Tick()               // drives the result toast (persists after close)
		}
		// A file drop is routed here, once: onto a drawer slot if it landed on
		// one (mount into that slot), otherwise into the machine (snapshot,
		// tape, disc, .scr). Read the drop exactly once -- the binding frees
		// the list on the first LoadDroppedFiles call.
		if rl.IsFileDropped() {
			files := rl.LoadDroppedFiles()
			if drawer == nil || !drawer.TryDrop(files) {
				for _, f := range files {
					if res := zx.loadDroppedFile(f); res != "" && drawer != nil {
						drawer.say(res)
					}
				}
			}
			rl.UnloadDroppedFiles()
		}

		// The emulated machine receives no input while a demo widget, the
		// menu bar or the HUD owns the frame -- HandleInput is skipped
		// outright rather than filtered, so nothing leaks through by accident.
		guiHasInput := (overlay != nil && overlay.Active()) || (bar != nil && bar.Active()) || (deck != nil && deck.Active()) || (hud != nil && hud.Active())
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
		// While the HUD is open the machine is paused: the HUD drives
		// stepping through the debug engine, so the normal 50fps advance
		// must not also run (it would race the engine on the same CPU).
		// The screen is still drawn, frozen at the last frame.
		if hud == nil || !hud.Active() {
			blockedOnRealMemory := sched != nil && sched.BlockingOnRealMemory()
			zx.RunTurboAwareFrame(blockedOnRealMemory)
		}
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
		// Whether the bar had the pointer before this frame's click was dealt
		// with: a click that chose a menu item must not also land on what is
		// under the menu.
		barHadInput := bar != nil && bar.Active()
		if bar != nil {
			bar.Update(zx)
		}

		frameNo++
		zx.io.FDCTick(frameNo, autoCommitDebounceFrames)

		// The machines behind the front one run on, and the dock may bring
		// another to the front: from the next frame on, that one is zx.
		if deck != nil {
			deck.runBehind(frameNo, autoCommitDebounceFrames)
			// A user drag of the window edge, in a tiled layout, is snapped
			// back to the layout's own proportions.
			deck.HandleWindowResize()
			// The HUD and the dock are mutually exclusive: while the HUD is
			// open the dock takes no input (Alt+M cannot bring it back over
			// the HUD) and stays hidden.
			busy := barHadInput || bar.Active() || (overlay != nil && overlay.Active()) || (hud != nil && hud.Active())
			deck.Update(int(rl.GetScreenWidth()), int(rl.GetScreenHeight()), bar.renderer, busy)
			if front := deck.Front(); front != zx {
				zx = front
				bar.setCurrentModel(deck.frontModel())
			} else if m := bar.currentModel(); m != "" {
				deck.setFrontModel(m)
			}
		}
	}

	// Clean up
	// Commit any unsaved disk changes before exiting, so writes made during
	// the session (e.g. a BASIC SAVE) persist to the .dsk file without needing
	// a manual save. Only writes back to an existing file; a blank disk with no
	// filename is left for the user to save explicitly (F8).
	if deck != nil {
		deck.closeAll()
	} else {
		saveDiskOnClose(zx)
	}

	zx.window().CleanupTextures()
	fmt.Println("ZenZX Stopped")
}
