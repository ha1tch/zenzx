//go:build headless

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

func main() {
	model := flag.String("model", "48k", "Spectrum model: 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3, ts2068, next")
	romPath := flag.String("rom", "", "Custom ROM bank(s), comma-separated, positionally mapped to bank 0,1,2,3 (up to the model's own bank count); applied on top of -model's standard set, not instead of it")
	rom0 := flag.String("rom0", "", "Override just ROM bank 0, leaving the rest of -model's standard set intact")
	rom1 := flag.String("rom1", "", "Override just ROM bank 1, leaving the rest of -model's standard set intact")
	rom2 := flag.String("rom2", "", "Override just ROM bank 2, leaving the rest of -model's standard set intact")
	rom3 := flag.String("rom3", "", "Override just ROM bank 3 (e.g. +3DOS on a +3), leaving the rest of -model's standard set intact")
	customROMsMenu := flag.Bool("custom-roms-menu", false, "Interactively pick a ROM from -custom-roms-dir and a bank to apply it to")
	customROMsDir := flag.String("custom-roms-dir", CustomROMDir, "Directory scanned by -custom-roms-menu")
	romDir := flag.String("romdir", "./rom", "Directory containing ROM files")
	snapshot := flag.String("snapshot", "", "Load snapshot on startup")
	snapshotFormat := flag.String("format", "auto", "Snapshot format: auto, zxs, sna, z80, szx (load only)")
	tapeFile := flag.String("tape", "", "Load tape file (.tap or .tzx)")
	tapeMode := flag.String("tapemode", "fast", "Tape mode: fast, accurate, or turbo (identical accurate-mode CPU/tape correctness, with per-instruction AMX/audio bookkeeping and per-frame border rendering skipped while the tape is actively loading)")
	binFile := flag.String("bin", "", "Load a raw binary blob directly into memory")
	binAddr := flag.String("binaddr", "0x8000", "Load address for -bin (hex 0x.. or decimal)")
	binStart := flag.String("binstart", "", "PC start address after -bin (hex/decimal; empty = use load address; -1 = leave PC unchanged)")
	z80n := flag.Bool("z80n", false, "Enable the CPU core's Z80N (ZX Spectrum Next) extended instruction set. CPU-level only -- no other Next hardware (memory paging, sprites, the copper, etc.) is emulated; intended for Z80N instruction verification (e.g. via -bin plus a \"dump-mem\" script action), not as a claim of Next machine emulation.")
	scrFile := flag.String("scr", "", "Load a raw .scr screen dump onto the display (still image)")
	noFdc := flag.Bool("nofdc", false, "Disable FDC emulation for +3")
	disk := flag.String("disk", "", "Path to +3 disk image (.dsk)")
	sdCard := flag.String("sdcard", "", "Path to a raw FAT16/FAT32 SD card image, attached via DivMMC (T-36, docs/TRACKING.md) -- requires -divmmc-rom too. Pages in via automap at the usual RST-vector entry points on ANY model, not just -model=next; does not by itself model the Next's own separate config-mode boot ROM (item 6)")
	divMMCROM := flag.String("divmmc-rom", "", "Path to the DivMMC/ESXDOS boot ROM image (8K) that -sdcard pages in -- a real ESXDOS build, not sourced or embedded by zenzx itself")
	bootROM := flag.String("bootrom", "", "Path to the FPGA config-mode boot ROM image (8K, e.g. nextboot.rom) -- a real, separate first-stage ROM, not enNxtmmc.rom/-divmmc-rom; runs SD init/FAT32/TBBLUE.FW loading before NextREG 0x03 disables it (T-36)")
	divMMCDebug := flag.Bool("divmmc-debug", false, "Log DivMMC automap transitions and SD card command/response frames to stderr")

	frames := flag.Int("frames", 100, "Number of frames to run")
	shotInterval := flag.Int("shot-interval", 0, "Capture a screenshot every N frames (0 = only the final frame)")
	shotDir := flag.String("shot-dir", ".", "Directory to write screenshots into")
	shotPrefix := flag.String("shot-prefix", "zenzx", "Filename prefix for screenshots")
	quiet := flag.Bool("quiet", false, "Suppress per-frame logging")
	scriptFile := flag.String("script", "", "Path to a .zen action script to drive the emulator")
	nonStandard := flag.String("non-standard", "off", "Master switch for non-standard features: on or off. Gates all -ns-* flags.")
	nsGraphics := flag.String("ns-graphics", "", "Non-standard graphics mode (requires -non-standard on): "+nsGraphicsUsage)
	nsStorage := flag.String("ns-storage", "", "Non-standard storage backend (requires -non-standard on): "+nsStorageUsage)
	joystick := flag.String("joystick", "auto", "Joystick emulation: auto (the selected -model's own built-in configuration), none, kempston, sinclair (alias for sinclair1), sinclair1, sinclair2, sinclair-both")
	mouse := flag.String("mouse", "none", "Mouse emulation: none or kempston")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

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
	mouseMode, err := ParseMouseMode(*mouse)
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
				logf(*quiet, "Loaded ZX Spectrum Next (+3 ROM base, matching real hardware's classic-compatible boot personality -- no NextZXOS boot chain yet, see T-32)")
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
		sdFile, err := os.Open(*sdCard)
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
		if *divMMCDebug {
			zx.io.divMMC.Debug = true
			zx.io.divMMC.SD().Debug = true
		}
		logf(*quiet, "DivMMC attached: rom=%s sdcard=%s (%d bytes)", *divMMCROM, *sdCard, info.Size())
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

	logf(*quiet, "Running %d frames (model=%s)", *frames, *model)
	shots := 0
	for f := 1; f <= *frames; f++ {
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
				path := filepath.Join(*shotDir, fmt.Sprintf("%s-frame%06d.png", *shotPrefix, f))
				if err := writeScreenPNG(path, zx, 1); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
				} else {
					shots++
					logf(*quiet, "  frame %d -> %s", f, path)
				}
			}
		}
	}

	// Commit any disk changes made during the run (e.g. by a script driving a
	// SAVE) back to the .dsk file before exiting.
	if zx.io.hasFDC && zx.io.fdc != nil && zx.io.fdc.IsModified() && zx.io.fdc.diskFilename != "" {
		if err := zx.io.SaveDisk(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save disk on exit: %v\n", err)
		}
	}

	if sched != nil {
		fmt.Printf("Done: %d frames, %d script screenshot(s) written to %s\n", *frames, sched.ShotsTaken(), *shotDir)
	} else {
		fmt.Printf("Done: %d frames, %d screenshot(s) written to %s\n", *frames, shots, *shotDir)
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
