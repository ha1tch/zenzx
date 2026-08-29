//go:build headless

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/ha1tch/zen80/z80"
)

// TestTraceHarness is the zenzx half of the cross-emulator trace
// instrument (the FUSE half is that build's zenzx_debug.c; both emit
// identical line formats so /tmp/tracetools/tracediff.py can diff them
// directly). It skips unless configured, so it never runs as part of
// the ordinary suite. All behaviour is environment-driven -- no
// recompilation between experiments.
//
// Generalized per docs/TRACE_GENERALIZATION_DEVELOPMENT_PLAN.md,
// Stages 1-10 (T-26). Every env var below not set falls back to
// today's exact original behaviour -- a run with only the pre-existing
// vars set produces the same trace it always did.
//
//	ZTRACE_OUT        output path (required; absent = skip)
//
// -- setup (Stage 2) --
//
//	ZTRACE_SETUP      tape (default) | snapshot | bin | boot-only
//	ZTRACE_MODEL      48k (default) | 128k | plus3
//	ZTRACE_TAPE       tape path, ZTRACE_SETUP=tape only (default: the Batman corpus TZX)
//	ZTRACE_SNAPSHOT   snapshot path, ZTRACE_SETUP=snapshot only (.sna or .z80)
//	ZTRACE_BIN        raw binary path, ZTRACE_SETUP=bin only
//	ZTRACE_BINADDR    load address for ZTRACE_BIN, hex "0x.." or decimal
//	                  (default "0x8000") -- ParseAddr, same as -binaddr
//	ZTRACE_BINSTART   PC after load, hex "0x.."/decimal/"-1" for "leave
//	                  unchanged" (default: load address) -- ParseAddrSigned,
//	                  same as -binstart
//	ZTRACE_FROM_RESET "1" to install hooks before boot, tracing from the
//	                  reset vector onward (Stage 3). Pairs naturally with
//	                  ZTRACE_SETUP=boot-only. ZTRACE_ENTRY=0000 arms
//	                  immediately in this mode. Known gap: combined with
//	                  a non-tape setup AND no explicit ZTRACE_ENTRY, the
//	                  setup phase itself runs before the "arm at wherever
//	                  we ended up" default below is computed -- set
//	                  ZTRACE_ENTRY explicitly when using FROM_RESET.
//
// -- arming and windowing (original) --
//
//	ZTRACE_ENTRY      hex PC that arms the trace (default 5D15)
//	ZTRACE_MAX_STEPS  instruction budget after arming (default 6000)
//	ZTRACE_PC_MIN     hex; only PCs >= this are emitted (default 0)
//	ZTRACE_P_FROM     decimal step; P logging starts here (default 0)
//	ZTRACE_P_TO       decimal step; P logging ends here (default max)
//	ZTRACE_MAX_FRAMES frame budget for the whole run (default 20000)
//	ZTRACE_PRESSKEY   decimal frame; press+release SPACE around that
//	                  frame of the main run (default: never). For
//	                  loaders that PAUSE for a key before the turbo
//	                  section.
//	ZTRACE_LOADCMD    the typed load command, ZTRACE_SETUP=tape only
//	                  (default `LOAD ""`).
//
// -- line types (original P/W/I, plus Stage 1/4 additions) --
//
//	ZTRACE_WRITES     "1" to emit memory-write deltas (W lines, default off)
//	ZTRACE_WMAX       write-record budget (default 5000000, ~100MB)
//	ZTRACE_WADDR_MIN  hex; only writes at addr >= this (default 4000).
//	                  Also filters R lines (Stage 4) -- one threshold
//	                  pair for both, not a second set.
//	ZTRACE_WPC_MIN    hex; only writes issued by PC >= this (default 0).
//	                  Also filters R lines.
//	ZTRACE_READS      "1" to emit memory-read deltas (R lines, Stage 4).
//	                  Fires on opcode-fetch reads too, not only operand
//	                  reads -- memRead doesn't distinguish them upstream.
//	ZTRACE_IOLOG      "1" to log port-read results (I lines)
//	ZTRACE_IO_FROM    decimal step; I/O logging starts here (default 0)
//	ZTRACE_IO_TO      decimal step; I/O logging ends here (default max).
//	                  Also bounds O lines (Stage 4).
//	ZTRACE_OLOG       "1" to log port-write results (O lines, Stage 4)
//	ZTRACE_M1LOG      "1" to log every M1 fetch as an M line (Stage 1):
//	                  normal opcode fetches, prefix continuations, and
//	                  interrupt acknowledgement, with context. Roughly
//	                  doubles output volume -- off by default.
//	ZTRACE_CALLSTACK  "1" to reconstruct call depth (Stage 7) and emit
//	                  an S line on every CALL/RST/interrupt (push) and
//	                  RET/RETN/RETI (pop). Independent of ZTRACE_M1LOG
//	                  -- shares the M1Hook installation, not the flag.
//	ZTRACE_TAPELOG    "1" to log per-frame tape state as T lines
//	                  (ZTRACE_SETUP=tape only; silent no-op otherwise):
//	                  T <frame> <cycles> <position> <edgeoffset> <ear>
//
// -- stop conditions (Stage 5, 9) --
//
//	ZTRACE_STOP_A     hex; disarm the step A equals this value
//	ZTRACE_STOP_BC    hex; disarm the step BC equals this value
//	ZTRACE_STOP_DE    hex; disarm the step DE equals this value
//	ZTRACE_STOP_HL    hex; disarm the step HL equals this value
//	ZTRACE_STOP_SP    hex; disarm the step SP equals this value
//	                  Multiple stops: first one to fire wins (OR, not AND).
//	ZTRACE_STOP_WADDR hex; disarm the step this address is written (a W
//	                  line) or, once ZTRACE_OLOG is on, this port is
//	                  written (an O line) -- Stage 9, same disarm path.
//
// -- companion outputs (Stage 6, 10) --
//
//	ZTRACE_COVERAGE_OUT  path; on completion, write a plain <addr> <count>
//	                     table from the same DebugPCHook calls already
//	                     firing (Stage 6). Alongside the P-line trace,
//	                     not instead of it.
//	ZTRACE_SYMFILE       pasmo-format .sym path (zenas's --sym output,
//	                     format confirmed against a real zenas build);
//	                     annotates P's PC, W/R's addr, and S's PC with
//	                     a bracketed label when one exists at that exact
//	                     address -- appends, never replaces, the hex
//	                     (Stage 8).
//	                     save the paused state via SaveSnapshot (zenzx's
//	                     own format, with metadata), SaveZ80, or SaveSNA
//	                     respectively (Stage 10) -- the extension picks
//	                     the method, no new save logic.
//
// Line formats:
//
//	P <step> <PC> <AF> <BC> <DE> <HL> <SP> <R>
//	W <n> <step> <PC> <addr> <old> <new>
//	R <n> <step> <PC> <addr> <val>
//	I <step> <cycles> <port> <val>
//	O <step> <cycles> <port> <val>
//	M <step> <PC> <opcode> <context>
//	S <step> <PC> depth <N>
//	T <frame> <cycles> <position> <edgeoffset> <ear> <ioear>
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
	envHexOpt := func(name string) (uint64, bool) {
		v := os.Getenv(name)
		if v == "" {
			return 0, false
		}
		n, err := strconv.ParseUint(v, 16, 32)
		if err != nil {
			t.Fatalf("%s=%q: %v", name, v, err)
		}
		return n, true
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
	envStr := func(name, def string) string {
		if v := os.Getenv(name); v != "" {
			return v
		}
		return def
	}

	// -- setup selection (Stage 2) --
	setupMode := envStr("ZTRACE_SETUP", "tape")
	model := envStr("ZTRACE_MODEL", "48k")
	fromReset := os.Getenv("ZTRACE_FROM_RESET") == "1"

	// -- arming / windowing --
	entryExplicit := os.Getenv("ZTRACE_ENTRY") != ""
	entry := uint16(envHex("ZTRACE_ENTRY", 0x5D15))
	// Default step budget: 6000 for the original tape-investigation
	// entry points. FROM_RESET traces the whole boot, which needs far
	// more -- measured directly (not guessed) via BootDetector: 48K
	// boot-to-ready takes 768,551 steps over 87 frames. 1,000,000
	// leaves headroom without being wildly oversized for the other
	// models this harness doesn't measure yet.
	defaultMaxSteps := int64(6000)
	if fromReset {
		defaultMaxSteps = 1000000
	}
	maxSteps := envInt("ZTRACE_MAX_STEPS", defaultMaxSteps)
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

	// -- Stage 1: M1 visibility --
	m1Log := os.Getenv("ZTRACE_M1LOG") == "1"

	// -- Stage 4: reads / port-writes --
	reads := os.Getenv("ZTRACE_READS") == "1"
	oLog := os.Getenv("ZTRACE_OLOG") == "1"

	// -- Stage 5/9: stop conditions --
	stopA, hasStopA := envHexOpt("ZTRACE_STOP_A")
	stopBC, hasStopBC := envHexOpt("ZTRACE_STOP_BC")
	stopDE, hasStopDE := envHexOpt("ZTRACE_STOP_DE")
	stopHL, hasStopHL := envHexOpt("ZTRACE_STOP_HL")
	stopSP, hasStopSP := envHexOpt("ZTRACE_STOP_SP")
	stopWaddr, hasStopWaddr := envHexOpt("ZTRACE_STOP_WADDR")

	// -- Stage 6/10: companion outputs --
	coverageOut := os.Getenv("ZTRACE_COVERAGE_OUT")
	snapshotOut := os.Getenv("ZTRACE_SNAPSHOT_OUT")

	// -- Stage 8: symbol annotation --
	var syms symTable
	if symPath := os.Getenv("ZTRACE_SYMFILE"); symPath != "" {
		var serr error
		syms, serr = loadSymFile(symPath)
		if serr != nil {
			t.Fatalf("loading ZTRACE_SYMFILE %s: %v", symPath, serr)
		}
	}

	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	var (
		zx         *ZenZX
		armed      bool
		stepCount  int64
		writeCount int64
		readCount  int64
		lastPC     uint16
		done       bool
		coverage   map[uint16]int
		snapSaved  bool
	)
	if coverageOut != "" {
		coverage = make(map[uint16]int)
	}

	// disarm centralizes every place tracing should stop: logs the
	// closing marker once, saves a trigger snapshot if requested
	// (Stage 10), and sets done. Called instead of assigning `done =
	// true` directly so the snapshot-on-trigger and closing-log
	// behaviour can never be forgotten at a new stop site.
	disarm := func(reason string) {
		if done {
			return
		}
		done = true
		fmt.Fprintf(out, "# window closed at step=%d writes=%d reads=%d (%s)\n",
			stepCount, writeCount, readCount, reason)
		if snapshotOut != "" && !snapSaved && zx != nil {
			snapSaved = true
			var serr error
			lower := strings.ToLower(snapshotOut)
			switch {
			case strings.HasSuffix(lower, ".z80"):
				serr = zx.SaveZ80(snapshotOut)
			case strings.HasSuffix(lower, ".sna"):
				serr = zx.SaveSNA(snapshotOut)
			default:
				serr = zx.SaveSnapshot(snapshotOut) // .zxs, or any other/missing extension
			}
			if serr != nil {
				fmt.Fprintf(out, "# snapshot save failed: %v\n", serr)
			} else {
				fmt.Fprintf(out, "# snapshot saved: %s\n", snapshotOut)
			}
		}
	}

	// checkStops evaluates Stage 5's register-equality conditions
	// against the CPU state at the top of the current step (before
	// this instruction executes). OR semantics: first match wins.
	checkStops := func() {
		switch {
		case hasStopA && uint64(zx.cpu.A) == stopA:
			disarm(fmt.Sprintf("ZTRACE_STOP_A=%02X reached", stopA))
		case hasStopBC && uint64(zx.cpu.BC()) == stopBC:
			disarm(fmt.Sprintf("ZTRACE_STOP_BC=%04X reached", stopBC))
		case hasStopDE && uint64(zx.cpu.DE()) == stopDE:
			disarm(fmt.Sprintf("ZTRACE_STOP_DE=%04X reached", stopDE))
		case hasStopHL && uint64(zx.cpu.HL()) == stopHL:
			disarm(fmt.Sprintf("ZTRACE_STOP_HL=%04X reached", stopHL))
		case hasStopSP && uint64(zx.cpu.SP) == stopSP:
			disarm(fmt.Sprintf("ZTRACE_STOP_SP=%04X reached", stopSP))
		}
	}

	installHooks := func() {
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
			if done {
				return
			}
			stepCount++
			if stepCount > maxSteps {
				if stepCount == maxSteps+1 {
					disarm("step budget exhausted")
				}
				return
			}
			if coverage != nil {
				coverage[pc]++
			}
			checkStops()
			if done {
				return
			}
			if pc >= pcMin && stepCount >= pFrom && stepCount <= pTo {
				fmt.Fprintf(out, "P %d %s %04X %04X %04X %04X %04X %02X\n",
					stepCount, syms.annotate(pc), zx.cpu.AF(), zx.cpu.BC(), zx.cpu.DE(), zx.cpu.HL(), zx.cpu.SP, zx.cpu.R)
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
			fmt.Fprintf(out, "W %d %d %04X %s %02X %02X\n", writeCount, stepCount, lastPC, syms.annotate(addr), old, val)
			if hasStopWaddr && uint64(addr) == stopWaddr {
				disarm(fmt.Sprintf("ZTRACE_STOP_WADDR=%04X written", stopWaddr))
			}
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
		z80.DebugMemReadHook = func(addr uint16, val uint8) {
			if !armed || !reads || done {
				return
			}
			if addr < waddrMin || readCount >= wmax {
				return
			}
			if lastPC < wpcMin {
				return
			}
			readCount++
			fmt.Fprintf(out, "R %d %d %04X %s %02X\n", readCount, stepCount, lastPC, syms.annotate(addr), val)
		}
		z80.DebugIOOutHook = func(port uint16, val uint8) {
			if !armed || done {
				return
			}
			if hasStopWaddr && uint64(port) == stopWaddr {
				defer disarm(fmt.Sprintf("ZTRACE_STOP_WADDR=%04X port written", stopWaddr))
			}
			if !oLog {
				return
			}
			if stepCount < ioFrom || stepCount > ioTo {
				return
			}
			fmt.Fprintf(out, "O %d %d %04X %02X\n", stepCount, zx.cpu.Cycles, port, val)
		}
		installM1Hook(zx, out, m1Log, &armed, &done, &stepCount, pcMin, syms)
	}
	defer func() {
		z80.DebugPCHook = nil
		z80.DebugMemWriteHook = nil
		z80.DebugIOInHook = nil
		z80.DebugMemReadHook = nil
		z80.DebugIOOutHook = nil
	}()

	if fromReset {
		zx = NewZenZX(AudioBackendOto)
		installHooks()
	} else {
		zx = NewZenZX(AudioBackendOto)
	}

	if err := loadModelROM(zx, model); err != nil {
		t.Fatalf("loading ROM for ZTRACE_MODEL=%q: %v", model, err)
	}

	tapeLog := os.Getenv("ZTRACE_TAPELOG") == "1" && setupMode == "tape"
	pressKey := envInt("ZTRACE_PRESSKEY", -1)

	switch setupMode {
	case "tape":
		tape := envStr("ZTRACE_TAPE", "/tmp/newdiv_corpus/Batman/Batman - Release 3.tzx")
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
		loadCmd := envStr("ZTRACE_LOADCMD", `LOAD ""`)
		kq := NewKeyQueue(10, 5)
		kq.EnqueueText(loadCmd)
		kq.EnqueueChord([]matrixPos{{6, 0}})
		for kq.Active() {
			kq.Step(zx)
			zx.RunFrame()
		}
		zx.io.ResetKeyboard()

	case "snapshot":
		snapPath := os.Getenv("ZTRACE_SNAPSHOT")
		if snapPath == "" {
			t.Fatal("ZTRACE_SETUP=snapshot requires ZTRACE_SNAPSHOT")
		}
		var loadErr error
		if strings.HasSuffix(strings.ToLower(snapPath), ".sna") {
			loadErr = zx.LoadSNA(snapPath)
		} else {
			loadErr = zx.LoadZ80(snapPath)
		}
		if loadErr != nil {
			t.Fatalf("loading snapshot %s: %v", snapPath, loadErr)
		}

	case "bin":
		binPath := os.Getenv("ZTRACE_BIN")
		if binPath == "" {
			t.Fatal("ZTRACE_SETUP=bin requires ZTRACE_BIN")
		}
		loadAddr, aerr := ParseAddr(envStr("ZTRACE_BINADDR", "0x8000"))
		if aerr != nil {
			t.Fatalf("ZTRACE_BINADDR: %v", aerr)
		}
		startAddr := int(loadAddr)
		if bs := os.Getenv("ZTRACE_BINSTART"); bs != "" {
			s, serr := ParseAddrSigned(bs)
			if serr != nil {
				t.Fatalf("ZTRACE_BINSTART: %v", serr)
			}
			startAddr = s
		}
		if err := zx.LoadBIN(binPath, loadAddr, startAddr); err != nil {
			t.Fatalf("LoadBIN %s: %v", binPath, err)
		}

	case "boot-only":
		det := NewBootDetector(false)
		for frame := 0; frame < 500 && !det.Ready(); frame++ {
			zx.RunFrame()
			det.Update(zx)
		}

	default:
		t.Fatalf("unknown ZTRACE_SETUP=%q (want tape, snapshot, bin, or boot-only)", setupMode)
	}

	// 0x5D15 is a tape-investigation-specific landmark (Speedlock/Batman,
	// see T-22) with no meaning for the other setups -- for those, an
	// unset ZTRACE_ENTRY means "arm wherever this setup actually left
	// PC," which is what choosing snapshot/bin/boot-only over tape
	// almost always means in practice.
	if !entryExplicit && setupMode != "tape" {
		entry = zx.cpu.PC
	}

	if !fromReset {
		installHooks()
	}

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

	if coverageOut != "" {
		cf, cerr := os.Create(coverageOut)
		if cerr != nil {
			t.Fatalf("creating coverage output %s: %v", coverageOut, cerr)
		}
		defer cf.Close()
		for addr, count := range coverage {
			fmt.Fprintf(cf, "%04X %d\n", addr, count)
		}
		t.Logf("coverage: %s (%d distinct addresses)", coverageOut, len(coverage))
	}

	t.Logf("trace: %s (steps=%d writes=%d reads=%d)", outPath, stepCount, writeCount, readCount)
}

// loadModelROM loads the ROM set for ZTRACE_MODEL. Only the models the
// trace harness has actually been exercised against are supported;
// extend this switch the same way to add another (the pattern mirrors
// zenzx_headless.go's own -model table, just not the full set of it --
// no need to duplicate every model here until a trace investigation
// actually needs one).
func loadModelROM(zx *ZenZX, model string) error {
	switch model {
	case "", "48k":
		return zx.LoadROM("./rom/48.rom")
	case "128k":
		return zx.Load128KROM("./rom/128-0.rom", "./rom/128-1.rom")
	case "plus3":
		return zx.LoadPlus3ROM("./rom/plus3-0.rom", "./rom/plus3-1.rom", "./rom/plus3-2.rom", "./rom/plus3-3.rom")
	default:
		return fmt.Errorf("ZTRACE_MODEL=%q not supported by the trace harness yet -- 48k, 128k, plus3 only; extend loadModelROM to add another", model)
	}
}
