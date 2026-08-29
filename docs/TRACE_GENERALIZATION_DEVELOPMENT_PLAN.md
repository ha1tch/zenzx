# Trace harness generalization — staged development plan

Updated: 2026-08-28

Design intent for `docs/TRACKING.md` T-26. Per tracking-document
practice, this plan is frozen once Stage 1 begins executing; deviations
from it are recorded in `docs/TRACE_GENERALIZATION_TRACKING.md`, not by
silently editing this file.

Sources: direct reading of `zzz_trace_harness_test.go`,
`zzz_pulsedump_test.go`, and zen80 v0.5.1's `z80.go` (via the module
cache) this session; `docs/hardware-accuracy.md`'s account of what the
existing instrumentation was built to verify (T-16, the Batman-class
investigation).

## Guiding principle: extend the instrument that already works, don't replace it

`TestTraceHarness` isn't broken — it did exactly what it was built for
(T-16's contention/floating-bus/TZX-pause verification, T-22's Batman
investigation) and the `P`/`W`/`I` line protocol plus
`/tmp/tracetools/tracediff.py` are worth keeping. The gaps are that (a)
it only knows how to *reach* a traceable state one way, and (b) it can
only *see* three of the five things worth tracing. Every stage below is
additive — new env vars, new line types, new optional setup paths — none
require touching the existing tape-load path's behavior when its env
vars aren't set. A run with today's exact env vars should produce today's
exact output at every stage.

## Backlog (priced, not scheduled)

Not "out of scope" — that framing presumed a decision that was never actually made. These were priced (2026-08-28, cross-referencing `zenas` and `queryfy` directly rather than guessing) and are staying off the active stage list only because their cost is Moderate or higher. Nothing here is rejected; it's unscheduled. Revisit any of these on request.

- **Opcode disassembly / mnemonic decoding (High).** Checked whether
  `zenas`'s encoder (`assembler/encoder.go`) could be inverted for
  this — it can't: keyed by mnemonic name, not byte pattern. A real
  disassembler ("zendis") is a separate project, realistically
  comparable in size to zen80's own `decode.go` + `z80n.go` combined —
  every base opcode, every prefix form, all Z80N extensions. De-risked
  by zen80's decode logic already being ZEXALL/ZEXDOC-verified (no ISA
  reverse-engineering needed, "just" a large mechanical build), but
  still multi-day-scale, not a stage.
- **Multiple simultaneous trace windows in one run (Low–Moderate).**
  Generalizing the single `armed`/`done` pair into a slice of
  independent window-states is bounded work, but every use case so far
  (T-16, T-22) needed only one contiguous window — priced above pure
  Low because it's a real refactor of existing control flow, not purely
  additive like the Stage 7–10 items below.
- **A general conditional-breakpoint expression language (Moderate).**
  Checked whether `queryfy` (already a zenzx dependency) would
  shortcut this — it doesn't: `queryfy/query/` is a JSON dot-notation
  *path* language, not a boolean expression evaluator over named
  variables. A register-condition grammar (`A==0x42 && HL>0x8000`) is
  a different, smaller thing — a hand-written tokenizer plus
  recursive-descent parser, ~150–250 lines — but still real parser work
  Stage 5's explicit `ZTRACE_STOP_*` variables avoid entirely.
- **Ring-buffer / crash-log tracing (Moderate).** Keep only the last N
  instructions in memory, dump on trigger, instead of a full linear log
  — useful for long runs where a full trace would be gigabytes. Bounded
  but self-contained new machinery, not an extension of something that
  already exists.
- **Live/interactive debugger (Very high).** A different order of
  magnitude from everything else here: step, continue, set breakpoint
  from a REPL, inspect on demand. Everything in this plan is "run to
  completion or to a trigger, read a log" — a REPL needs pausable
  execution control and a command interface, closer to a new tool than
  an extension of this one.
- **Reverse/step-back execution (Very high, likely not worth it).**
  Needs full per-step state snapshots or reversible micro-ops. Real
  tools that have this (rr-style debuggers, some MAME debug builds) pay
  a large complexity cost for it. Neither T-16 nor T-22 — the two real
  investigations this instrument has supported — ever needed to go
  backward; noted because it's a real feature other tools have, not
  because there's a concrete need for it here yet.

## Stage 1 — Wire up `M1Hook` for interrupt and prefix-fetch visibility

**Goal:** the harness can show *why* a PC transition happened —
interrupt-serviced vs. ordinary control flow — with no zen80 change and
no dependency bump. This closes the interrupt-tracking gap entirely on
its own.

- Add `zx.cpu.M1Hook = func(pc uint16, opcode uint8, context string) {...}`
  alongside the existing three hook assignments, gated by a new
  `ZTRACE_M1LOG` env var (default off, since it roughly doubles output
  volume — one `M` line per instruction, not just per traced `P` line).
- New line format: `M <step> <PC> <opcode> <context>`, where `context`
  is whatever `M1Hook` passes through unchanged (`"normal"`, `"post-CB"`,
  `"post-DD"`, `"post-ED"`, `"post-FD"`, `"NMI"`, `"IM0"`,
  `"IM0-fallback"`, `"IM1"`, `"IM2"`).
- Respect the existing `armed`/`done`/step-budget state so `M` lines
  follow the same window as `P` lines, not a separate one.
- Update `tracediff.py` (external to this repo, `/tmp/tracetools/`) to
  ignore `M` lines it doesn't recognise rather than fail, so this stage
  doesn't require a matching FUSE-side change to land safely.

**Done when:** tracing a known interrupt-driven routine (e.g. a short
hand-assembled program with IM1 enabled, run long enough to take at
least one interrupt) shows an `M` line with context `"IM1"` at the exact
step immediately preceding a `P` line where PC jumps to `0x0038` with no
corresponding `CALL`/`JP` in the preceding instruction stream — the
signal that today's harness cannot produce at all.

## Stage 2 — Pluggable setup (stop assuming tape-load)

**Goal:** reach a traceable state without booting to BASIC and typing
`LOAD ""`.

- New `ZTRACE_SETUP` env var: `tape` (default, today's exact behavior),
  `snapshot`, `bin`, or `boot-only`.
- `snapshot`: `ZTRACE_SNAPSHOT` path, loaded via the existing
  `LoadSNA`/`LoadZ80` entry points (already used elsewhere in the test
  suite — `snapshot_roundtrip_test.go`, `snapshot_sentinel_test.go`) —
  no new loading code, just routing.
- `bin`: `ZTRACE_BIN`/`ZTRACE_BINADDR`/`ZTRACE_BINSTART`, reusing
  `LoadBIN` exactly as the CLI flags already do (`loadbin_test.go`).
  This is the path for tracing a small hand-assembled test program with
  no ROM/BASIC/tape involvement at all — the cleanest way to verify
  Stage 1's `M1Hook` wiring without a real tape corpus.
- `boot-only`: boots to BASIC, stops there — for tracing what the ready
  prompt's own idle loop or a subsequently-typed short BASIC line does,
  without any load.
- Add `ZTRACE_MODEL` (default `48k`, today's only option), reusing the
  same model-switch table `zenzx_headless.go`/`zenzx_gui.go` already
  build for `-model`.
- `ZTRACE_TAPELOG`/`T` lines become a silent no-op when `ZTRACE_SETUP`
  isn't `tape`, rather than emitting meaningless static tape state.

**Done when:** `ZTRACE_SETUP=bin` with a small hand-assembled program
(written for this stage, not third-party content — same principle
`contrib/tape-corpus-harness`'s 0.6.2 entry already established for this
codebase) traces correctly from its own entry point, verified against
hand-worked-out expected register values for a handful of instructions.

## Stage 3 — Optional pre-setup hook installation

**Goal:** support tracing from cold reset or during ROM boot itself,
which the harness structurally cannot see today — hooks install at
line 140, after boot and after the tape-load typing loop already ran
unhooked (lines 106–125).

- `ZTRACE_FROM_RESET=1`: install all hooks *before* `NewZenZX`/boot runs,
  changing "armed" semantics so `ZTRACE_ENTRY=0000` legitimately traces
  from the reset vector.
- Interacts with Stage 2's `ZTRACE_SETUP=boot-only` naturally — reset
  tracing doesn't need a load path at all.
- Budget concern to size during implementation, not guess at now: boot
  is typically under 500 frames (existing `det.Ready()` loop bound), so
  the step count from reset to BASIC ready is bounded but unmeasured
  today — get a real number before picking a new default
  `ZTRACE_MAX_STEPS` for this mode.

**Done when:** a `ZTRACE_FROM_RESET=1` trace's first several `P` lines
match known ROM disassembly landmarks for the 48K reset routine (e.g.
the initial stack-pointer setup and the jump into the standard
initialization sequence).

## Stage 4 — Memory-read and port-write tracing

**Goal:** add `R` (memory read) and `O` (port write) line types.

**Blocked on:** `docs/proposals/zen80-tracing-hooks.md` landing in a
tagged zen80 release and `go.mod`'s zen80 requirement bumping to it.
Nothing in this stage can start before that.

- `R <step> <PC> <addr> <val>` from `DebugMemReadHook`, filterable by
  the same `waddrMin`/`wpcMin`-shaped thresholds `W` lines already use
  (reuse the filter variables, don't invent a second set).
- `O <step> <PC> <port> <val>` from `DebugIOOutHook`, filterable
  symmetrically with the existing `I`-line `ioFrom`/`ioTo` filters.
- Note the known limitation from the proposal doc: `R` lines fire on
  opcode-fetch reads too, not only operand reads — document this in the
  harness's own doc comment rather than silently filtering it, so a
  consumer isn't surprised by fetch noise in what looks like a data-read
  trace.

**Done when:** tracing a port-OUT-heavy routine (AY register
programming is the natural real example, already in this codebase) with
`O` lines enabled shows every register-select/data-write pair, and an
intentionally-introduced one-byte OUT-value regression in a test fixture
is visible in the trace where it would be invisible today.

## Stage 5 — Register-equality stop conditions

**Goal:** close some of the "conditional breakpoint" gap without
building a general expression language (see Explicitly out of scope).

- `ZTRACE_STOP_A`, `ZTRACE_STOP_HL` (and similarly for the other
  16-bit/8-bit register pairs, added as needed rather than all at once):
  hex value; when set, tracing disarms (`done = true`) the step a
  matching register value is observed, same as today's step-budget
  disarm path.
- Multiple stop conditions, if more than one is set, disarm on whichever
  fires first (OR semantics, not AND) — simplest to reason about and
  matches how a human would use "stop when we get there" during a live
  investigation.

**Done when:** a trace with `ZTRACE_STOP_HL=6912` (or another known
sentinel address reached by an existing test scenario) stops at that
exact step without needing `ZTRACE_MAX_STEPS` to be pre-calculated by
hand first.

## Stage 6 — Coverage summary mode

**Goal:** a lightweight companion output for "which addresses executed,
how often" — useful for coverage questions on a long run without
generating a full step-by-step log.

- `ZTRACE_COVERAGE_OUT`: a second output path, a plain `<addr> <count>`
  table built from the same `DebugPCHook` calls already firing, no
  separate instrumentation needed.
- Runs alongside the existing `P`-line trace, not instead of it — both
  can be requested in the same run.

**Done when:** running today's exact tape-load scenario with
`ZTRACE_COVERAGE_OUT` set produces a table whose count for one sampled
hot-loop address matches a manual count from the corresponding `P`-line
log for the same run.

## Stage 7 — Call-stack reconstruction

**Depends on:** Stage 1 (`M1Hook` wiring).

**Goal:** a trace can show which call frame each instruction ran under,
not just SP deltas the reader has to interpret by hand.

- A fixed classification table over the opcode byte `M1Hook` already
  reports: CALL (`0xCD` + 8 conditional forms), RST (8 forms), RET
  (`0xC9` + 8 conditional forms), RETI/RETN (`ED 4D`/`ED 45`, plus the
  documented undocumented-RETN ED aliases) push/pop a shadow stack
  entry; the interrupt contexts `M1Hook` already reports
  (`"NMI"`/`"IM0"`/`"IM1"`/`"IM2"`/`"IM0-fallback"`) push one too, since
  an interrupt is a call with no matching CALL opcode.
- New line type: `S <step> <PC> depth <N>` on every push/pop, or fold
  depth into existing `P` lines as an extra field — pick whichever
  keeps `tracediff.py` compatibility simplest when this is built, not
  guessed now.
- Self-contained in the harness; no zen80 change.

**Done when:** a trace through a short hand-assembled program with a
nested CALL two levels deep shows depth incrementing on each CALL and
decrementing on each matching RET, ending back at depth 0.

## Stage 8 — Symbol-table-aware trace output

**Goal:** trace lines show `LD_BYTES+3` instead of `05E7` when the
address is known, without needing a separate disassembly (Stage 8
predates any disassembler and doesn't need one).

- `zenas` already emits a pasmo-format symbol file via `--sym[=path]`
  — confirmed by reading `zenas/main.go` directly, not assumed. A small
  parser for that format (checked: simple, well-established text
  layout) builds an addr→label map.
- `ZTRACE_SYMFILE=path.sym`: when set, every address-bearing field
  (`P`'s PC, `W`/`R`'s addr, `S`'s call target once Stage 7 exists) is
  annotated with the matching label when one exists at that exact
  address, left as plain hex otherwise. Annotate, don't replace — the
  raw hex stays in the line for tools that don't care about symbols.
- No dependency on any other stage; can be built whenever, but is most
  useful once there's more than one line type worth reading, so
  sequenced after the core stages rather than first.

**Done when:** assembling a small test program with `zenas --sym`,
tracing it, and setting `ZTRACE_SYMFILE` to the resulting `.sym`
produces `P` lines annotated with the program's own label names at the
addresses the assembler placed them.

## Stage 9 — Watchpoints as stop conditions

**Depends on:** Stage 4 (memory-read/port-write tracing, itself blocked
on the zen80 proposal) and Stage 5 (stop-condition mechanism).

**Goal:** promote "write to this address" from a log filter to a
disarm condition, alongside Stage 5's register-equality stops.

- `ZTRACE_STOP_WADDR`: hex address; when a `W`-line (or, once Stage 4
  lands, an `O`-line port) matches, tracing disarms the same step,
  reusing Stage 5's OR-semantics disarm path rather than inventing a
  second one.
- Natural, small extension once both dependencies exist — no new
  mechanism, just a new trigger source feeding the one Stage 5 already
  built.

**Done when:** a trace with `ZTRACE_STOP_WADDR=5C00` (or another known
address an existing test scenario writes to) stops at the exact step
of that write, without needing the step count known in advance.

## Stage 10 — Snapshot-on-trigger

**Depends on:** Stage 5 (stop-condition mechanism).

**Goal:** when a trace disarms — step budget, register-equality stop,
or (once Stage 9 lands) a watchpoint — optionally dump a loadable
snapshot of that exact moment, so the paused state can be opened in the
interactive GUI instead of only read as a log.

- `ZTRACE_SNAPSHOT_OUT=path.zxs` (or `.z80`/`.sna`): on disarm, call
  the harness's `zx` instance's existing `SaveSnapshot` (zenzx's own
  native `.zxs` format, with metadata) or `SaveZ80`/`SaveSNA` — the
  extension picks the method. All three confirmed already existing
  (`SaveSnapshot` in `snapshot.go`, `SaveZ80`/`SaveSNA` in
  `snapshot_formats.go`), nothing new to write on the save side.
  `.zxs` is the natural default given it's zenzx's own format and
  carries metadata the other two don't, but all three are supported
  since `.sna`/`.z80` are what a real machine or another emulator would
  recognise.
- No new save logic at all; this stage is entirely about calling an
  existing method at the right moment.

**Done when:** a trace with `ZTRACE_STOP_HL=6912` and
`ZTRACE_SNAPSHOT_OUT` set to each of the three extensions in turn
produces a loadable file each time that, opened in the GUI separately,
shows HL at the expected value in the debugger/register display.

## Dependency summary

| Stage | Depends on |
|---|---|
| 1 | Nothing — `M1Hook` already exists in zen80 v0.5.1 |
| 2 | Nothing new — reuses existing `LoadSNA`/`LoadZ80`/`LoadBIN`/model-switch code |
| 3 | Nothing new — reuses Stage 2's setup routing |
| 4 | `docs/proposals/zen80-tracing-hooks.md` landing upstream |
| 5 | Nothing new |
| 6 | Nothing new |
| 7 | Stage 1 |
| 8 | Nothing new — reuses `zenas`'s existing `--sym` output |
| 9 | Stage 4 (external) and Stage 5 |
| 10 | Stage 5 — reuses existing `SaveSnapshot`/`SaveZ80`/`SaveSNA` |

Stages 1, 2, 3, 5, 6, 7, 8, and 10 have no external dependency. Stage 4
is the only one gated on work outside this repo; Stage 9 inherits that
gate transitively through Stage 4.
