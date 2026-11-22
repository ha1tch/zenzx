//go:build headless

package zenzx

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

// ============================================================================
// Headless entry point
//
// Boots a Spectrum model, runs a fixed number of frames, and writes PNG
// screenshots of the 256x192 display at a configurable interval. No window,
// no raylib, no audio device -- suitable for CI smoke tests and automated
// rendering checks.
//
// Examples:
//   zenzx-headless -model=48k -frames=200 -shot-interval=50
//   zenzx-headless -snapshot=game.z80 -frames=1 -shot-dir=out
//   zenzx-headless -tape=game.tap -frames=2000 -shot-interval=100
// ============================================================================

// RunHeadless is the ZenZX program without a window: it parses args (the
// command line without the program's name), runs the frames asked for, and
// exits the process on error. cmd/zenzx-headless is this and nothing else.
func RunHeadless(args []string) {
	SetLogOutput(os.Stdout)
	SetTraceOutput(os.Stderr)
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	model := fs.String("model", "48k", "Spectrum model: 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3, ts2068, next")
	romPath := fs.String("rom", "", "Custom ROM bank(s), comma-separated, positionally mapped to bank 0,1,2,3 (up to the model's own bank count); applied on top of -model's standard set, not instead of it")
	rom0 := fs.String("rom0", "", "Override just ROM bank 0, leaving the rest of -model's standard set intact")
	rom1 := fs.String("rom1", "", "Override just ROM bank 1, leaving the rest of -model's standard set intact")
	rom2 := fs.String("rom2", "", "Override just ROM bank 2, leaving the rest of -model's standard set intact")
	rom3 := fs.String("rom3", "", "Override just ROM bank 3 (e.g. +3DOS on a +3), leaving the rest of -model's standard set intact")
	customROMsMenu := fs.Bool("custom-roms-menu", false, "Interactively pick a ROM from -custom-roms-dir and a bank to apply it to")
	customROMsDir := fs.String("custom-roms-dir", CustomROMDir, "Directory scanned by -custom-roms-menu")
	romDir := fs.String("romdir", "./rom", "Directory containing ROM files")
	snapshot := fs.String("snapshot", "", "Load snapshot on startup")
	snapshotFormat := fs.String("format", "auto", "Snapshot format: auto, zxs, sna, z80, szx (load only)")
	tapeFile := fs.String("tape", "", "Load tape file (.tap or .tzx)")
	tapeMode := fs.String("tapemode", "fast", "Tape mode: fast, accurate, or turbo (identical accurate-mode CPU/tape correctness, with per-instruction AMX/audio bookkeeping and per-frame border rendering skipped while the tape is actively loading)")
	binFile := fs.String("bin", "", "Load a raw binary blob directly into memory")
	binAddr := fs.String("binaddr", "0x8000", "Load address for -bin (hex 0x.. or decimal)")
	binStart := fs.String("binstart", "", "PC start address after -bin (hex/decimal; empty = use load address; -1 = leave PC unchanged)")
	z80n := fs.Bool("z80n", false, "Enable the CPU core's Z80N (ZX Spectrum Next) extended instruction set. CPU-level only -- no other Next hardware (memory paging, sprites, the copper, etc.) is emulated; intended for Z80N instruction verification (e.g. via -bin plus a \"dump-mem\" script action), not as a claim of Next machine emulation.")
	scrFile := fs.String("scr", "", "Load a raw .scr screen dump onto the display (still image)")
	noFdc := fs.Bool("nofdc", false, "Disable FDC emulation for +3")
	disk := fs.String("disk", "", "Path to +3 disk image (.dsk)")
	sdCard := fs.String("sdcard", "", "Path to a raw FAT16/FAT32 SD card image, attached via DivMMC (T-36, docs/TRACKING.md) -- requires -divmmc-rom too. Pages in via automap at the usual RST-vector entry points on ANY model, not just -model=next; does not by itself model the Next's own separate config-mode boot ROM (item 6)")
	divMMCROM := fs.String("divmmc-rom", "", "Path to the DivMMC/ESXDOS boot ROM image (8K) that -sdcard pages in -- a real ESXDOS build, not sourced or embedded by zenzx itself")
	bootROM := fs.String("bootrom", "", "Path to the FPGA config-mode boot ROM image (8K, e.g. nextboot.rom) -- a real, separate first-stage ROM, not enNxtmmc.rom/-divmmc-rom; runs SD init/FAT32/TBBLUE.FW loading before NextREG 0x03 disables it (T-36)")
	nextRegLogFlag := fs.String("nextreg-log", "", "Next: log writes to these NextREG registers to stderr: hex numbers separated by commas, or all")
	nextDump := fs.Bool("next-dump", false, "Next: at the end of the run print the registers, the memory mapping and the start of the palettes")
	sdCardReadOnly := fs.Bool("sdcard-readonly", false, "Write-protect the -sdcard image: the machine reads it and every write is refused. Without this the image is opened for writing and the firmware and NextZXOS change it, as they do a real card")
	divMMCDebug := fs.Bool("divmmc-debug", false, "Log DivMMC automap transitions and SD card command/response frames to stderr")
	nextFilesFlag := fs.String("next-files", "", nextFilesHelp)
	nextClassic := fs.Bool("next-classic", false, nextClassicHelp)
	nextExamplesFlag := fs.Bool("next-examples", false, nextExamplesHelp)
	nextAddFlag := fs.String("next-add", "", nextAddHelp)
	sdCard2 := fs.String("sdcard2", "", sdCard2Help)
	nextNewCard := fs.String("next-new-card", "", nextNewCardHelp)
	nextBootable := fs.String("next-bootable", "", nextBootableHelp)
	nextExamplesCard := fs.String("next-examples-card", "", nextExCardHelp)
	nextIssue := fs.Int("next-issue", 5, nextIssueHelp)
	nextSetupFlag := fs.Bool("next-setup", false, "Next: download the boot files once (the emulator distribution, about 70 MB, and the boot ROM), build and verify the SD card, and exit. They go to the -next-files folder, or the per-user one. What is already there is kept")
	nextDistURL := fs.String("next-distribution", "", "Next: address of the emulator distribution zip for -next-setup (default: the 26.11 distribution at specnext.com)")
	nextBootURL := fs.String("next-bootrom-url", "", "Next: address of the boot ROM for -next-setup")

	frames := fs.Int("frames", 100, framesHelp)
	multiFlag := fs.String("multi", "", multiHelp)
	listenFlag := fs.String("listen", "", listenHelp)
	listenHost := fs.String("listen-host", "127.0.0.1", listenHostHelp)
	autorunFlag := fs.String("autorun", "", autorunHelp)
	layoutFlag := fs.String("layout", "", layoutHelp)
	paceFlag := fs.String("pace", "", paceHelp)
	shotInterval := fs.Int("shot-interval", 0, "Capture a screenshot every N frames (0 = only the final frame)")
	shotDir := fs.String("shot-dir", ".", "Directory to write screenshots into")
	shotPrefix := fs.String("shot-prefix", "zenzx", "Filename prefix for screenshots")
	quiet := fs.Bool("quiet", false, "Suppress per-frame logging")
	scriptFile := fs.String("script", "", "Path to a .zen action script to drive the emulator")
	nonStandard := fs.String("non-standard", "off", "Master switch for non-standard features: on or off. Gates all -ns-* flags.")
	nsGraphics := fs.String("ns-graphics", "", "Non-standard graphics mode (requires -non-standard on): "+nsGraphicsUsage)
	nsStorage := fs.String("ns-storage", "", "Non-standard storage backend (requires -non-standard on): "+nsStorageUsage)
	joystick := fs.String("joystick", "auto", "Joystick emulation: auto (the selected -model's own built-in configuration; on a Next, what NextREG 0x05 chooses for each port), none, kempston, kempston2, kempston-both, sinclair (alias for sinclair1), sinclair1, sinclair2, sinclair-both, cursor")
	mouse := fs.String("mouse", "auto", "Mouse emulation: auto (kempston on a Next, none on the other models), none, kempston or amx")
	showVersion := fs.Bool("version", false, "Print version and exit")
	fs.Parse(args)

	// Several machines (multi.go). The primary one is set up below as a
	// single machine always was, with -model standing for its model; the
	// others are made just before the run.
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
	if _, err := parseLayout(*layoutFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
	if autorunFiles[primary] != "" && (*tapeFile != "" || *snapshot != "") {
		fmt.Fprintln(os.Stderr, "Error: -autorun puts its own tape in the machine and runs it from a cold start: it cannot be used with -tape or -snapshot")
		os.Exit(2)
	}
	if len(listenPorts) > 0 && !flagGiven(fs, "frames") {
		*frames = 0 // a machine that listens runs until it is stopped
	}
	paceReal, paceErr := realPace(*paceFlag, *frames)
	if paceErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", paceErr)
		os.Exit(2)
	}

	if *nextSetupFlag {
		dir, err := nextDownloadDir(*nextFilesFlag)
		if err == nil {
			err = runNextSetup(dir, *nextDistURL, *nextBootURL, os.Stderr)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
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
	if *nextExamplesCard != "" {
		if err := runNextExamplesCard(*nextExamplesCard, os.Stderr); err != nil {
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
		fmt.Printf("ZenZX %s (headless)\n", version)
		return
	}

	nsConfig, err := ParseNonStandardConfig(*nonStandard, *nsGraphics, *nsStorage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if s := nsConfig.Summary(); s != "" {
		logf(*quiet, "%s", s)
	}
	joystickMode, err := resolveJoystickMode(*joystick, *model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if strings.EqualFold(strings.TrimSpace(*joystick), "auto") && joystickMode != JoystickNone {
		logf(*quiet, "Joystick: %s (built into -model=%s)", joystickMode, *model)
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

	// Audio: headless always uses the Oto backend object but never opens a
	// device. NewZenZX wires the wrapper; we simply never call Initialize.
	zx := NewZenZX(AudioBackendOto)
	if err := zx.io.setNextRegLog(*nextRegLogFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := zx.memory.SetNextIssue(*nextIssue); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *z80n {
		zx.EnableZ80N()
		logf(*quiet, "Z80N (ZX Spectrum Next) instruction set enabled")
	}
	zx.nonStandard = nsConfig
	if err := zx.SelectVideoRenderer(nsConfig.Graphics); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	zx.io.SetJoystickMode(joystickMode)
	zx.io.SetMouseMode(mouseMode)
	if zx.audio != nil {
		zx.audio.SetEnabled(false)
	}

	if err := os.MkdirAll(*shotDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating screenshot dir %s: %v\n", *shotDir, err)
		os.Exit(1)
	}

	// Reset only if not loading a snapshot (snapshot sets CPU state).
	if *snapshot == "" {
		zx.cpu.Reset()
	}

	// --- ROM loading -------------------------------------------------------
	// Always load -model's own standard ROM set first (no longer gated
	// behind whether -rom was given -- -rom/-rom0..-rom3 now layer
	// individual bank overrides on top of it afterward, rather than
	// replacing it outright the way the old single-file -rom did).
	// Named ROMs resolve filesystem-first, embedded-fallback (see
	// resolveROMBytes in embedded_roms.go) rather than bare paths.
	romLoaded := false
	switch *model {
	case "48k":
		romLoaded = tryLoadBytes(*quiet, "48K", zx.LoadROMBytes, "48.rom", *romDir)
	case "128k":
		romLoaded = tryLoad2Bytes(*quiet, "128K (Sinclair)", zx.Load128KROMBytes, "128-0.rom", "128-1.rom", *romDir)
	case "plus2":
		romLoaded = tryLoad2Bytes(*quiet, "Spectrum +2", zx.Load128KROMBytes, "plus2-0.rom", "plus2-1.rom", *romDir)
	case "plus2a":
		// +2A/+2B: shares the +3's motherboard and uses the exact same 64K
		// +3 ROM set (including +3DOS, which goes unused). The only hardware
		// difference is the absent floppy controller, so we load the +3 ROMs
		// but do NOT enable the FDC.
		if data, err := loadPlus3Bytes(*romDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			if zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil {
				logf(*quiet, "Loaded Spectrum +2A (+3 ROM, no floppy controller)")
				romLoaded = true
			}
		}
	case "plus3":
		if data, err := loadPlus3Bytes(*romDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			if zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil {
				logf(*quiet, "Loaded Spectrum +3 ROM")
				if !*noFdc {
					zx.io.EnableFDC()
					if os.Getenv("ZENZX_FDC_DEBUG") != "" {
						zx.io.SetFDCDebug(true)
					}
					if *disk != "" {
						if err := zx.io.LoadDisk(*disk); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to load disk %s: %v\n", *disk, err)
						}
					}
				}
				romLoaded = true
			}
		}
	case "spanish48k":
		romLoaded = tryLoadBytes(*quiet, "Spanish 48K", zx.LoadROMBytes, "48-spanish.rom", *romDir)
	case "spanish128k":
		romLoaded = tryLoad2Bytes(*quiet, "Spanish 128K", zx.Load128KROMBytes, "128-spanish-0.rom", "128-spanish-1.rom", *romDir)
	case "spanishplus2":
		romLoaded = tryLoad2Bytes(*quiet, "Spanish +2", zx.Load128KROMBytes, "plus2-spanish-0.rom", "plus2-spanish-1.rom", *romDir)
	case "spanishplus3":
		if data, err := loadPlus3Bytes(*romDir, "plus3-spanish-0.rom", "plus3-spanish-1.rom", "plus3-spanish-2.rom", "plus3-spanish-3.rom"); err == nil {
			if zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil {
				logf(*quiet, "Loaded Spanish Spectrum +3 ROM")
				if !*noFdc {
					zx.io.EnableFDC()
				}
				romLoaded = true
			}
		}
	case "next":
		romLoaded = false
		if data, err := loadPlus3Bytes(*romDir, "plus3-0.rom", "plus3-1.rom", "plus3-2.rom", "plus3-3.rom"); err == nil {
			if zx.LoadPlus3ROMBytes(data[0], data[1], data[2], data[3]) == nil {
				logf(*quiet, "Loaded ZX Spectrum Next (+3 ROM base; NextZXOS boots with -bootrom, -divmmc-rom and -sdcard, see docs/MANUAL.md)")
				romLoaded = true
			}
		}
		if romLoaded {
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
			logf(*quiet, "Z80N CPU + Next extended MMU enabled")
			// This model shares the +3 ROM base, and T-36 debugging
			// against real ESXDOS/DivMMC firmware found its
			// boot code -- when it can't get anywhere over SD -- falls
			// through to the same +3DOS disk-boot ROM -model=plus3
			// uses. That ROM needs a real drive-status answer, not
			// just an enabled-but-empty controller: Sense Drive Status
			// reports "not ready" whenever no disk is loaded, and
			// +3DOS hangs waiting for that to change. Neither FDC
			// itself nor -disk was wired into this case at all before
			// (only -model=plus3 had either), so a blank formatted
			// disk is the default when -disk isn't given -- matching
			// what a real +3 user would do (insert a blank disk)
			// rather than trying to emulate an empty drive's own
			// boot-retry/give-up semantics correctly.
			if !*noFdc {
				zx.io.EnableFDC()
				if *disk != "" {
					if err := zx.io.LoadDisk(*disk); err != nil {
						logf(*quiet, "Error loading disk %s: %v", *disk, err)
					} else {
						logf(*quiet, "Loaded disk: %s", *disk)
					}
				} else if zx.io.fdc != nil {
					zx.io.fdc.CreateBlankDisk()
					logf(*quiet, "No -disk given: created a blank formatted disk so +3DOS's drive detection doesn't hang")
				}
			}
		}
	case "ts2068":
		if home, err := resolveROMBytes("ts2068-0.rom", *romDir); err == nil {
			if ext, err := resolveROMBytes("ts2068-1.rom", *romDir); err == nil {
				if zx.LoadTS2068ROMBytes(home, ext) == nil {
					logf(*quiet, "Loaded TS2068 ROM (Home + Extension)")
					romLoaded = true
				}
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown model: %s\n", *model)
	}

	if !romLoaded {
		if data0, err := resolveROMBytes("128-0.rom", *romDir); err == nil {
			if data1, err := resolveROMBytes("128-1.rom", *romDir); err == nil {
				if zx.Load128KROMBytes(data0, data1) == nil {
					logf(*quiet, "Loaded 128K ROM (default)")
					romLoaded = true
				}
			}
		}
	}
	if !romLoaded {
		if data, err := resolveROMBytes("48.rom", *romDir); err == nil {
			if zx.LoadROMBytes(data) == nil {
				logf(*quiet, "Loaded 48K ROM (default)")
				romLoaded = true
			}
		}
	}
	if !romLoaded {
		fmt.Fprintf(os.Stderr, "Error: no ROM files found in %s or embedded in the binary\n", *romDir)
		os.Exit(1)
	}

	// --- ROM bank overrides --------------------------------------------
	// Layered on top of the standard -model set just loaded, not instead
	// of it -- -rom's positions fill in from bank 0 first, then any
	// -romN individually overrides that specific bank regardless of what
	// -rom already did, so -rom=a.rom,b.rom -rom3=c.rom composes exactly
	// as it reads: banks 0 and 1 from -rom, bank 3 from -rom3, bank 2
	// left as -model's own standard ROM.
	if *romPath != "" {
		for i, p := range strings.Split(*romPath, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if err := zx.OverrideROMBank(i, p); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: -rom bank %d: %v\n", i, err)
				continue
			}
			logf(*quiet, "ROM bank %d overridden: %s", i, p)
		}
	}
	for bank, p := range map[int]*string{0: rom0, 1: rom1, 2: rom2, 3: rom3} {
		if *p == "" {
			continue
		}
		if err := zx.OverrideROMBank(bank, *p); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: -rom%d: %v\n", bank, err)
			continue
		}
		logf(*quiet, "ROM bank %d overridden: %s", bank, *p)
	}

	// --- Interactive custom ROM menu ------------------------------------
	// Applied last, after every other ROM-loading path -- the most
	// specific layer, and the one a person driving the menu interactively
	// expects to win over whatever -model/-rom/-romN already set up.
	if *customROMsMenu {
		runCustomROMMenu(zx, *customROMsDir, bufio.NewReader(os.Stdin), os.Stdout)
	}

	// --- Snapshot ----------------------------------------------------------
	if *snapshot != "" {
		format := *snapshotFormat
		if format == "auto" {
			if format = DetectSnapshotFormat(*snapshot); format == "" {
				format = "zxs"
			}
		}
		if err := zx.LoadSnapshotFormat(*snapshot, format); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading snapshot %s: %v\n", *snapshot, err)
			zx.cpu.Reset()
		} else {
			logf(*quiet, "Loaded %s snapshot: %s", strings.ToUpper(format), *snapshot)
		}
	} else {
		zx.cpu.Reset()
	}

	// --- Tape --------------------------------------------------------------
	if *tapeFile != "" && zx.tape != nil {
		if err := zx.tape.LoadFile(*tapeFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading tape %s: %v\n", *tapeFile, err)
		} else {
			switch strings.ToLower(*tapeMode) {
			case "accurate":
				zx.tape.SetMode(TapeAccurate)
			case "turbo":
				zx.tape.SetMode(TapeTurbo)
			default:
				zx.tape.SetMode(TapeFast)
			}
			// LoadFile leaves the tape stopped; start playback so the Tick loop
			// actually advances it (in fast mode this triggers block injection).
			zx.tape.Play()
			logf(*quiet, "Loaded tape: %s (%s mode)", *tapeFile, *tapeMode)
		}
	}

	// --- FPGA config-mode boot ROM (T-36, docs/TRACKING.md) -----------------
	if *bootROM != "" {
		data, err := os.ReadFile(*bootROM)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading -bootrom %s: %v\n", *bootROM, err)
			os.Exit(1)
		}
		zx.memory.SetBootROM(data)
		logf(*quiet, "Boot ROM attached: %s (%d bytes, overlay enabled -- disabled by the first NextREG 0x03 write)", *bootROM, len(data))
	}

	// --- DivMMC / SD card (T-36, docs/TRACKING.md) --------------------------
	if *sdCard != "" || *divMMCROM != "" {
		if *sdCard == "" || *divMMCROM == "" {
			fmt.Fprintln(os.Stderr, "Error: -sdcard and -divmmc-rom must both be given together")
			os.Exit(1)
		}
		romBytes, err := os.ReadFile(*divMMCROM)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading -divmmc-rom %s: %v\n", *divMMCROM, err)
			os.Exit(1)
		}
		// Left open for the process's lifetime: SDCard reads from it on
		// every emulated sector access for as long as zx runs, and this
		// is a short-lived CLI process where exit is an acceptable
		// substitute for an explicit Close.
		sdFile, sdProtected, err := openSDCardImage(*sdCard, *sdCardReadOnly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening -sdcard %s: %v\n", *sdCard, err)
			os.Exit(1)
		}
		info, err := sdFile.Stat()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error stat'ing -sdcard %s: %v\n", *sdCard, err)
			os.Exit(1)
		}
		zx.EnableDivMMC(sdFile, info.Size(), romBytes)
		zx.io.divMMC.SD().wr.WriteProtected = sdProtected
		if *divMMCDebug {
			zx.io.divMMC.Debug = true
			zx.io.divMMC.SD().Debug = true
		}
		logf(*quiet, "DivMMC attached: rom=%s sdcard=%s (%d bytes, write-protected: %v)", *divMMCROM, *sdCard, info.Size(), sdProtected)
		if sdProtected && !*sdCardReadOnly {
			fmt.Fprintf(os.Stderr, "Warning: %s could not be opened for writing, so the card is write-protected and nothing can be saved to it\n", *sdCard)
		}
	}

	// The Next's boot chain from its folder (nextchain.go).
	if wantNextBootChain(*model, *nextClassic, *bootROM, *divMMCROM, *sdCard, *snapshot, *binFile, *tapeFile, *scrFile) {
		if f, ok, note := locateNextFiles(*nextFilesFlag); ok {
			if cards[primary] != "" {
				f.Card = cards[primary] // the card -multi names for this Next
			}
			protected, err := zx.attachNextBootChain(f, *sdCardReadOnly, *divMMCDebug)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: the Next boot files in %s could not be used: %v\n", f.Dir, err)
				os.Exit(1)
			}
			logf(*quiet, "%s", describeNextChain(f, protected))
			line, err := zx.insertOwnOrGivenCard(*sdCard2, *sdCardReadOnly)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: the second SD card could not be used: %v\n", err)
				os.Exit(1)
			}
			if line != "" {
				logf(*quiet, "%s", line)
			}
		} else {
			logf(*quiet, "%s", nextSetupHint(os.Args[0], note))
		}
	}

	// --- Raw binary --------------------------------------------------------
	if *binFile != "" {
		addr, err := ParseAddr(*binAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid -binaddr %q: %v\n", *binAddr, err)
			os.Exit(1)
		}
		start := int(addr) // default: start at load address
		if *binStart != "" {
			s, err := ParseAddrSigned(*binStart)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid -binstart %q: %v\n", *binStart, err)
				os.Exit(1)
			}
			start = s
		}
		if err := zx.LoadBIN(*binFile, uint16(addr), start); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading binary %s: %v\n", *binFile, err)
			os.Exit(1)
		}
		if start >= 0 {
			logf(*quiet, "Loaded binary: %s at 0x%04X, PC=0x%04X", *binFile, addr, start)
		} else {
			logf(*quiet, "Loaded binary: %s at 0x%04X (PC unchanged)", *binFile, addr)
		}
	}

	// --- Screen dump (.scr) ------------------------------------------------
	if *scrFile != "" {
		if err := zx.LoadSCR(*scrFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading screen %s: %v\n", *scrFile, err)
			os.Exit(1)
		}
		logf(*quiet, "Loaded screen: %s", *scrFile)
	}

	// --- Run ---------------------------------------------------------------
	// --- Action script (.zen) ---------------------------------------------
	var sched *Scheduler
	if *scriptFile != "" {
		script, err := ParseScriptFile(*scriptFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing script: %v\n", err)
			os.Exit(1)
		}
		sched = NewScheduler(script, ScheduleConfig{
			ShotDir:    *shotDir,
			ShotPrefix: *shotPrefix,
			Quiet:      *quiet,
			Model:      *model,
		})
		logf(*quiet, "Loaded script: %s (%d actions)", *scriptFile, len(script.Actions))
	}

	// --- The other machines, the links and the autoruns (multi.go) --------
	machines := make([]*ZenZX, len(models))
	machines[primary] = zx
	for i, m := range models {
		if i == primary {
			continue
		}
		other, err := newMachine(m, *romDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: machine %d: %v\n", i+1, err)
			os.Exit(1)
		}
		other.nonStandard = nsConfig
		if m == "next" {
			// Every Next but the primary one has a card of its own (parseMulti).
			line, err := bootNextFrom(other, cardFiles, cards[i], *sdCardReadOnly, *divMMCDebug)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: machine %d: %v\n", i+1, err)
				os.Exit(1)
			}
			logf(*quiet, "Machine %d: %s", i+1, line)
		}
		machines[i] = other
	}
	if len(machines) > 1 {
		logf(*quiet, "%d machines: %s", len(machines), strings.Join(models, ", "))
	}
	links, err := startLinks(machines, *listenHost, listenPorts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	for i, l := range links {
		logf(*quiet, "Machine %d (%s): link on %s", i+1, models[i], l.Addr())
	}
	if err := startAutoruns(machines, models, autorunFiles); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	for i, file := range autorunFiles {
		if file != "" && machines[i].AutorunPending() {
			logf(*quiet, "Machine %d (%s): will run %s", i+1, models[i], file)
		}
	}

	if *frames == 0 {
		logf(*quiet, "Running until interrupted (model=%s)", *model)
	} else {
		logf(*quiet, "Running %d frames (model=%s)", *frames, *model)
	}
	stopAsked := interrupted()
	var clock frameClock
	shots := 0
	ran := 0
	for f := 1; *frames == 0 || f <= *frames; f++ {
		if stopAsked() {
			logf(*quiet, "Interrupted after %d frames", ran)
			if sched == nil && ran > 0 {
				shots += captureMachines(machines, *shotDir, *shotPrefix, ran, *quiet)
			}
			break
		}
		ran = f
		if sched != nil {
			sched.Tick(zx)
			if sched.QuitRequested() {
				logf(*quiet, "Script requested quit at frame %d", f)
				break
			}
		}

		// RunTurboAwareFrame owns the whole Turbo-mode lifecycle (fast-
		// path memory/IO snapshot-in, per-block sync, snapshot-out once
		// the tape finishes) and falls through to plain RunFrame in
		// every other case -- see turbo.go. blockedOnRealMemory is false
		// whenever there's no script driving the run at all (sched nil):
		// nothing could be waiting on real screen/memory content in that
		// case, so there's nothing to pause for.
		blockedOnRealMemory := sched != nil && sched.BlockingOnRealMemory()
		zx.RunTurboAwareFrame(blockedOnRealMemory)
		zx.io.FDCTick(f, 30*50)
		for i, other := range machines {
			if i != primary {
				other.RunTurboAwareFrame(false)
				other.io.FDCTick(f, 30*50)
			}
		}

		// Legacy interval/final-frame capture is disabled while a script is
		// driving the run: the script's own shot actions own capture, and
		// running both would write competing screenshots into the same
		// directory (and could collide on the auto-name pattern).
		if sched == nil {
			capture := false
			if *shotInterval > 0 && f%*shotInterval == 0 {
				capture = true
			}
			if f == *frames {
				capture = true // always capture the final frame
			}
			if capture {
				shots += captureMachines(machines, *shotDir, *shotPrefix, f, *quiet)
			}
		}
		if paceReal {
			clock.wait()
		}
	}
	for _, l := range links {
		l.Close()
	}

	// Commit any disk changes made during the run (e.g. by a script driving a
	// SAVE) back to the .dsk file before exiting.
	if zx.io.hasFDC && zx.io.fdc != nil && zx.io.fdc.IsModified() && zx.io.fdc.diskFilename != "" {
		if err := zx.io.SaveDisk(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save disk on exit: %v\n", err)
		}
	}

	if sched != nil {
		fmt.Printf("Done: %d frames, %d script screenshot(s) written to %s\n", ran, sched.ShotsTaken(), *shotDir)
	} else {
		fmt.Printf("Done: %d frames, %d screenshot(s) written to %s\n", ran, shots, *shotDir)
	}
	if *nextDump {
		zx.DumpNextState(os.Stdout)
	}
}

func logf(quiet bool, format string, args ...any) {
	if !quiet {
		fmt.Printf(format+"\n", args...)
	}
}

func tryLoad(quiet bool, label string, fn func(string) error, path string) bool {
	if fn(path) == nil {
		logf(quiet, "Loaded %s ROM", label)
		return true
	}
	return false
}

func tryLoad2(quiet bool, label string, fn func(string, string) error, a, b string) bool {
	if fn(a, b) == nil {
		logf(quiet, "Loaded %s ROM", label)
		return true
	}
	return false
}

// tryLoadBytes/tryLoad2Bytes mirror tryLoad/tryLoad2 exactly, but
// resolve named ROMs via resolveROMBytes (filesystem-first, embedded
// fallback -- embedded_roms.go) instead of taking bare paths, so the
// standard -model loading path works identically whether or not a
// rom/ folder exists alongside the binary.
func tryLoadBytes(quiet bool, label string, fn func([]byte) error, name, romDir string) bool {
	data, err := resolveROMBytes(name, romDir)
	if err != nil {
		return false
	}
	if fn(data) == nil {
		logf(quiet, "Loaded %s ROM", label)
		return true
	}
	return false
}

func tryLoad2Bytes(quiet bool, label string, fn func([]byte, []byte) error, nameA, nameB, romDir string) bool {
	dataA, err := resolveROMBytes(nameA, romDir)
	if err != nil {
		return false
	}
	dataB, err := resolveROMBytes(nameB, romDir)
	if err != nil {
		return false
	}
	if fn(dataA, dataB) == nil {
		logf(quiet, "Loaded %s ROM", label)
		return true
	}
	return false
}
