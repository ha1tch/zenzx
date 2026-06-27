//go:build headless

package main

import (
	"flag"
	"fmt"
	"image/png"
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
	model := flag.String("model", "48k", "Spectrum model: 48k, 128k, plus2, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3")
	romPath := flag.String("rom", "", "Path to custom ROM file")
	romDir := flag.String("romdir", "./rom", "Directory containing ROM files")
	snapshot := flag.String("snapshot", "", "Load snapshot on startup")
	snapshotFormat := flag.String("format", "auto", "Snapshot format: auto, zxs, sna, z80")
	tapeFile := flag.String("tape", "", "Load tape file (.tap or .tzx)")
	tapeMode := flag.String("tapemode", "fast", "Tape mode: fast or accurate")
	binFile := flag.String("bin", "", "Load a raw binary blob directly into memory")
	binAddr := flag.String("binaddr", "0x8000", "Load address for -bin (hex 0x.. or decimal)")
	binStart := flag.String("binstart", "", "PC start address after -bin (hex/decimal; empty = use load address; -1 = leave PC unchanged)")
	scrFile := flag.String("scr", "", "Load a raw .scr screen dump onto the display (still image)")
	noFdc := flag.Bool("nofdc", false, "Disable FDC emulation for +3")
	disk := flag.String("disk", "", "Path to +3 disk image (.dsk)")

	frames := flag.Int("frames", 100, "Number of frames to run")
	shotInterval := flag.Int("shot-interval", 0, "Capture a screenshot every N frames (0 = only the final frame)")
	shotDir := flag.String("shot-dir", ".", "Directory to write screenshots into")
	shotPrefix := flag.String("shot-prefix", "zenzx", "Filename prefix for screenshots")
	quiet := flag.Bool("quiet", false, "Suppress per-frame logging")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ZenZX %s (headless)\n", version)
		return
	}

	rom := func(name string) string { return filepath.Join(*romDir, name) }

	// Audio: headless always uses the Oto backend object but never opens a
	// device. NewZenZX wires the wrapper; we simply never call Initialize.
	zx := NewZenZX(AudioBackendOto)
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
	romLoaded := false
	if *romPath != "" {
		if err := zx.LoadROM(*romPath); err == nil {
			logf(*quiet, "Loaded custom ROM: %s", *romPath)
			romLoaded = true
		} else {
			fmt.Fprintf(os.Stderr, "Error loading custom ROM %s: %v\n", *romPath, err)
		}
	}

	if !romLoaded {
		switch *model {
		case "48k":
			romLoaded = tryLoad(*quiet, "48K", zx.LoadROM, rom("48.rom"))
		case "128k":
			romLoaded = tryLoad2(*quiet, "128K (Sinclair)", zx.Load128KROM, rom("128-0.rom"), rom("128-1.rom"))
		case "plus2":
			romLoaded = tryLoad2(*quiet, "Spectrum +2", zx.Load128KROM, rom("plus2-0.rom"), rom("plus2-1.rom"))
		case "plus3":
			if zx.LoadPlus3ROM(rom("plus3-0.rom"), rom("plus3-1.rom"), rom("plus3-2.rom"), rom("plus3-3.rom")) == nil {
				logf(*quiet, "Loaded Spectrum +3 ROM")
				if !*noFdc {
					zx.io.EnableFDC()
					if *disk != "" {
						if err := zx.io.LoadDisk(*disk); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to load disk %s: %v\n", *disk, err)
						}
					}
				}
				romLoaded = true
			}
		case "spanish48k":
			romLoaded = tryLoad(*quiet, "Spanish 48K", zx.LoadROM, rom("48-spanish.rom"))
		case "spanish128k":
			romLoaded = tryLoad2(*quiet, "Spanish 128K", zx.Load128KROM, rom("128-spanish-0.rom"), rom("128-spanish-1.rom"))
		case "spanishplus2":
			romLoaded = tryLoad2(*quiet, "Spanish +2", zx.Load128KROM, rom("plus2-spanish-0.rom"), rom("plus2-spanish-1.rom"))
		case "spanishplus3":
			if zx.LoadPlus3ROM(rom("plus3-spanish-0.rom"), rom("plus3-spanish-1.rom"), rom("plus3-spanish-2.rom"), rom("plus3-spanish-3.rom")) == nil {
				logf(*quiet, "Loaded Spanish Spectrum +3 ROM")
				if !*noFdc {
					zx.io.EnableFDC()
				}
				romLoaded = true
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown model: %s\n", *model)
		}
	}

	if !romLoaded {
		if zx.Load128KROM(rom("128-0.rom"), rom("128-1.rom")) == nil {
			logf(*quiet, "Loaded 128K ROM (default)")
			romLoaded = true
		} else if zx.LoadROM(rom("48.rom")) == nil {
			logf(*quiet, "Loaded 48K ROM (default)")
			romLoaded = true
		}
	}
	if !romLoaded {
		fmt.Fprintf(os.Stderr, "Error: no ROM files found in %s\n", *romDir)
		os.Exit(1)
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
			if strings.ToLower(*tapeMode) == "accurate" {
				zx.tape.SetMode(TapeAccurate)
			} else {
				zx.tape.SetMode(TapeFast)
			}
			logf(*quiet, "Loaded tape: %s (%s mode)", *tapeFile, *tapeMode)
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
	logf(*quiet, "Running %d frames (model=%s)", *frames, *model)
	shots := 0
	for f := 1; f <= *frames; f++ {
		zx.RunFrame()

		capture := false
		if *shotInterval > 0 && f%*shotInterval == 0 {
			capture = true
		}
		if f == *frames {
			capture = true // always capture the final frame
		}
		if capture {
			path := filepath.Join(*shotDir, fmt.Sprintf("%s-frame%06d.png", *shotPrefix, f))
			if err := writePNG(path, zx.screen); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
			} else {
				shots++
				logf(*quiet, "  frame %d -> %s", f, path)
			}
		}
	}

	fmt.Printf("Done: %d frames, %d screenshot(s) written to %s\n", *frames, shots, *shotDir)
}

func writePNG(path string, screen *SpectrumScreen) error {
	img := screen.DecodeRGBA()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
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
