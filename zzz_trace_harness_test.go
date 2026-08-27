package main

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/ha1tch/zen80/z80"
)

// TestTraceHarness is the zenzx half of the cross-emulator trace
// instrument (the FUSE half is that build's zenzx_debug.c; both emit
// identical line formats so /tmp/tracetools/tracediff.py can diff them
// directly). It skips unless configured, so it never runs as part of
// the ordinary suite. All behaviour is environment-driven -- no
// recompilation between experiments:
//
//	ZTRACE_OUT        output path (required; absent = skip)
//	ZTRACE_TAPE       tape path (default: the Batman corpus TZX)
//	ZTRACE_ENTRY      hex PC that arms the trace (default 5D15)
//	ZTRACE_MAX_STEPS  instruction budget after arming (default 6000)
//	ZTRACE_PC_MIN     hex; only PCs >= this are emitted (default 0)
//	ZTRACE_P_FROM     decimal step; P logging starts here (default 0)
//	ZTRACE_P_TO       decimal step; P logging ends here (default max)
//	ZTRACE_WRITES     "1" to emit memory-write deltas (default off)
//	ZTRACE_WMAX       write-record budget (default 5000000, ~100MB)
//	ZTRACE_WADDR_MIN  hex; only writes at addr >= this (default 4000)
//	ZTRACE_WPC_MIN    hex; only writes issued by PC >= this (default 0)
//	ZTRACE_MAX_FRAMES frame budget for the whole run (default 20000)
//	ZTRACE_PRESSKEY   decimal frame; press+release SPACE around that
//	                  frame of the main run (default: never). For
//	                  loaders that PAUSE for a key before the turbo
//	                  section.
//	ZTRACE_TAPELOG    "1" to log per-frame tape state as T lines:
//	                  T <frame> <cycles> <position> <edgeoffset> <ear>
//	ZTRACE_IOLOG      "1" to log port-read results as I lines
//	ZTRACE_IO_FROM    decimal step; I logging starts here (default 0)
//	ZTRACE_IO_TO      decimal step; I logging ends here (default max)
//	ZTRACE_LOADCMD    the typed load command (default `LOAD ""`).
//	                  `LOAD""` -- no space -- produces an E-LINE
//	                  byte-identical to FUSE's phantom typist (keyword
//	                  J + quotes), eliminating the known-benign
//	                  WORKSP-family +-1 delta from every comparison.
//
// Line formats:
//
//	P <step> <PC> <AF> <BC> <DE> <HL> <SP> <R>
//	W <n> <step> <PC> <addr> <old> <new>
func TestTraceHarness(t *testing.T) {
	outPath := os.Getenv("ZTRACE_OUT")
	if outPath == "" {
		t.Skip("ZTRACE_OUT not set; trace harness is opt-in")
	}
	envHex := func(name string, def uint64) uint64 {
		if v := os.Getenv(name); v != "" {
			n, err := strconv.ParseUint(v, 16, 32)
			if err != nil {
				t.Fatalf("%s=%q: %v", name, v, err)
			}
			return n
		}
		return def
	}
	envInt := func(name string, def int64) int64 {
		if v := os.Getenv(name); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				t.Fatalf("%s=%q: %v", name, v, err)
			}
			return n
		}
		return def
	}
	tape := os.Getenv("ZTRACE_TAPE")
	if tape == "" {
		tape = "/tmp/newdiv_corpus/Batman/Batman - Release 3.tzx"
	}
	entry := uint16(envHex("ZTRACE_ENTRY", 0x5D15))
	maxSteps := envInt("ZTRACE_MAX_STEPS", 6000)
	pcMin := uint16(envHex("ZTRACE_PC_MIN", 0))
	pFrom := envInt("ZTRACE_P_FROM", 0)
	pTo := envInt("ZTRACE_P_TO", 1<<62)
	writes := os.Getenv("ZTRACE_WRITES") == "1"
	wmax := envInt("ZTRACE_WMAX", 5000000)
	waddrMin := uint16(envHex("ZTRACE_WADDR_MIN", 0x4000))
	wpcMin := uint16(envHex("ZTRACE_WPC_MIN", 0))
	maxFrames := envInt("ZTRACE_MAX_FRAMES", 20000)
	ioLog := os.Getenv("ZTRACE_IOLOG") == "1"
	ioFrom := envInt("ZTRACE_IO_FROM", 0)
	ioTo := envInt("ZTRACE_IO_TO", 1<<62)

	zx := NewZenZX(AudioBackendOto)
	if err := zx.LoadROM("./rom/48.rom"); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}
	zx.tape.SetMode(TapeAccurate)
	if err := zx.tape.LoadFile(tape); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	zx.tape.Play()
	if skip := envInt("ZTRACE_TAPE_SKIP_T", 0); skip > 0 {
		zx.tape.Tick(int(skip))
	}

	det := NewBootDetector(false)
	for frame := 0; frame < 500 && !det.Ready(); frame++ {
		zx.RunFrame()
		det.Update(zx)
	}
	for extra := envInt("ZTRACE_EXTRA_BOOT_FRAMES", 0); extra > 0; extra-- {
		zx.RunFrame()
	}
	loadCmd := os.Getenv("ZTRACE_LOADCMD")
	if loadCmd == "" {
		loadCmd = `LOAD ""`
	}
	kq := NewKeyQueue(10, 5)
	kq.EnqueueText(loadCmd)
	kq.EnqueueChord([]matrixPos{{6, 0}})
	for kq.Active() {
		kq.Step(zx)
		zx.RunFrame()
	}
	zx.io.ResetKeyboard()

	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	var (
		armed      bool
		stepCount  int64
		writeCount int64
		lastPC     uint16
		done       bool
	)
	z80.DebugPCHook = func(pc uint16) {
		lastPC = pc
		if !armed {
			if pc == entry {
				armed = true
				fmt.Fprintf(out, "# armed at %04X AF=%04X BC=%04X DE=%04X HL=%04X SP=%04X frames=%d\n",
					pc, zx.cpu.AF(), zx.cpu.BC(), zx.cpu.DE(), zx.cpu.HL(), zx.cpu.SP, zx.frameCount)
			} else {
				return
			}
		}
		stepCount++
		if stepCount > maxSteps {
			if stepCount == maxSteps+1 {
				fmt.Fprintf(out, "# window closed at step=%d writes=%d\n", stepCount-1, writeCount)
			}
			done = true
			return
		}
		if pc >= pcMin && stepCount >= pFrom && stepCount <= pTo {
			fmt.Fprintf(out, "P %d %04X %04X %04X %04X %04X %04X %02X\n",
				stepCount, pc, zx.cpu.AF(), zx.cpu.BC(), zx.cpu.DE(), zx.cpu.HL(), zx.cpu.SP, zx.cpu.R)
		}
	}
	z80.DebugMemWriteHook = func(addr uint16, old, val uint8) {
		if !armed || !writes || done {
			return
		}
		if addr < waddrMin || writeCount >= wmax {
			return
		}
		if lastPC < wpcMin {
			return
		}
		writeCount++
		fmt.Fprintf(out, "W %d %d %04X %04X %02X %02X\n", writeCount, stepCount, lastPC, addr, old, val)
	}
	z80.DebugIOInHook = func(port uint16, val uint8) {
		if !ioLog || !armed || done {
			return
		}
		if stepCount < ioFrom || stepCount > ioTo {
			return
		}
		fmt.Fprintf(out, "I %d %d %04X %02X\n", stepCount, zx.cpu.Cycles, port, val)
	}
	defer func() {
		z80.DebugPCHook = nil
		z80.DebugMemWriteHook = nil
		z80.DebugIOInHook = nil
	}()

	tapeLog := os.Getenv("ZTRACE_TAPELOG") == "1"
	pressKey := envInt("ZTRACE_PRESSKEY", -1)
	for frame := int64(0); frame < maxFrames && !done; frame++ {
		if pressKey >= 0 && frame >= pressKey && frame < pressKey+10 {
			zx.io.PressKey(7, 0) // SPACE
		} else if pressKey >= 0 && frame == pressKey+10 {
			zx.io.ReleaseKey(7, 0)
		}
		zx.RunFrame()
		if tapeLog {
			ear := 0
			if zx.tape.st.EarLevel {
				ear = 1
			}
			ioEar := 0
			if zx.io.GetTapeEar() {
				ioEar = 1
			}
			fmt.Fprintf(out, "T %d %d %d %d %d %d\n", frame, zx.cpu.Cycles,
				zx.tape.st.Position, zx.tape.st.EdgeOffset, ear, ioEar)
		}
	}
	if !armed {
		t.Fatalf("never reached entry PC %04X within %d frames", entry, maxFrames)
	}
	t.Logf("trace: %s (steps=%d writes=%d)", outPath, stepCount, writeCount)
}
