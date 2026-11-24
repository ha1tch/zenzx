// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

// Command zendbg is the standalone CLI surface of the ZenZX debugger: it
// builds one headless machine, attaches the debug engine, and reads commands
// on stdin. It is raylib-free, so it builds and runs the same on Linux and
// macOS, and it is the engine's test harness in script mode (commands piped
// on stdin).
//
// It is the standalone-machine proof of concept (docs/proposals/workbench.md,
// idea 1): no time machine, no GUI; the in-ZenZX HUD is a separate surface
// over the same engine and command layer.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ha1tch/zendis/pkg/disasm"
	"github.com/ha1tch/zenzx"
	"github.com/ha1tch/zenzx/pkg/debug"
	"github.com/ha1tch/zenzx/pkg/debugcmd"
)

func main() {
	model := flag.String("model", "48k", "machine model: 48k or 128k")
	romDir := flag.String("romdir", "./rom", "directory to look for ROMs in before the embedded copies")
	loadBIN := flag.String("bin", "", "raw binary to load (as FILE:ADDR, hex ADDR), PC left unchanged")
	flag.Parse()

	zx, err := buildMachine(*model, *romDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zendbg: %v\n", err)
		os.Exit(1)
	}

	if *loadBIN != "" {
		if err := loadRawBIN(zx, *loadBIN); err != nil {
			fmt.Fprintf(os.Stderr, "zendbg: %v\n", err)
			os.Exit(1)
		}
	}

	tgt := disasm.TargetZ80
	if strings.EqualFold(*model, "next") {
		tgt = disasm.TargetZ80N
	}
	eng := debug.NewEngine(newMachineAdapter(zx.Debug()), tgt)

	fmt.Printf("zendbg: %s machine ready. 'help' for commands.\n", *model)
	repl(eng, zx, os.Stdin, os.Stdout)
}

// buildMachine constructs a headless machine of the given model with its ROM
// loaded. Audio is never initialised: the debugger drives Step directly, so
// the audio engine (the one cgo-touching part) never spins up.
func buildMachine(model, romDir string) (*zenzx.ZenZX, error) {
	zx := zenzx.NewZenZX(zenzx.AudioBackendOto)
	return loadModel(zx, model, romDir)
}

func loadModel(zx *zenzx.ZenZX, model, romDir string) (*zenzx.ZenZX, error) {
	rom := func(name string) ([]byte, error) { return zenzx.ResolveROMBytes(name, romDir) }
	switch strings.ToLower(model) {
	case "48k":
		data, err := rom("48.rom")
		if err != nil {
			return nil, fmt.Errorf("48k ROM: %w", err)
		}
		if err := zx.LoadROMBytes(data); err != nil {
			return nil, fmt.Errorf("48k ROM load: %w", err)
		}
	case "128k":
		r0, err := rom("128-0.rom")
		if err != nil {
			return nil, fmt.Errorf("128k ROM 0: %w", err)
		}
		r1, err := rom("128-1.rom")
		if err != nil {
			return nil, fmt.Errorf("128k ROM 1: %w", err)
		}
		if err := zx.Load128KROMBytes(r0, r1); err != nil {
			return nil, fmt.Errorf("128k ROM load: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown model %q (want 48k or 128k)", model)
	}
	return zx, nil
}

// loadRawBIN loads a FILE:ADDR spec into memory without changing the PC.
func loadRawBIN(zx *zenzx.ZenZX, spec string) error {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("bin %q: want FILE:ADDR", spec)
	}
	addr, err := debugcmd.ParseAddrPublic(parts[1])
	if err != nil {
		return err
	}
	data, err := os.ReadFile(parts[0])
	if err != nil {
		return fmt.Errorf("bin: %w", err)
	}
	zx.Debug().WriteMem(addr, data)
	return nil
}

// repl reads commands until EOF or quit. It is used for both interactive and
// script (piped) input; in script mode it echoes each command so a transcript
// is self-describing.
func repl(eng *debug.Engine, zx *zenzx.ZenZX, in *os.File, out *os.File) {
	sc := bufio.NewScanner(in)
	w := bufio.NewWriter(out)
	defer w.Flush()

	info, _ := in.Stat()
	interactive := (info.Mode() & os.ModeCharDevice) != 0

	for {
		if interactive {
			fmt.Fprint(w, "> ")
			w.Flush()
		}
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if !interactive && line != "" {
			fmt.Fprintf(w, "> %s\n", line) // echo in script mode
		}
		if line == "" {
			continue
		}

		cmd, err := debugcmd.Parse(line)
		if err != nil {
			fmt.Fprintf(w, " error: %v\n", err)
			w.Flush()
			continue
		}
		// reset is a machine op the surface owns.
		if cmd.Verb == debugcmd.VerbReset {
			zx.Reset()
			fmt.Fprintln(w, " machine reset")
			w.Flush()
			continue
		}
		text, quit, err := debugcmd.Exec(eng, cmd)
		if err != nil {
			fmt.Fprintf(w, " error: %v\n", err)
			w.Flush()
			continue
		}
		if quit {
			break
		}
		if text != "" {
			fmt.Fprintln(w, text)
		}
		w.Flush()
	}
}
