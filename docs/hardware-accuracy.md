# Hardware accuracy — 2026-08-25/26 changes

Updated: 2026-08-26

What was fixed, what was measured (not assumed), and what is still
open, as of 2026-08-26. Scope is the 48K ULA/Z80 model; 128K and
TS2068 caveats are noted where they apply. This is a point-in-time
report, not a living document — see `TRACKING.md` and
`KNOWN_ISSUES.md` for current status.

## Summary

Five hardware-fidelity gaps were closed and measured against a
reference emulator (FUSE) and an independent TZX-spec implementation,
using permanent, env-parameterised trace instrumentation added to both
zenzx and zen80 (`zzz_trace_harness_test.go`, zen80's `DebugPCHook`/
`DebugMemWriteHook`/`DebugIOInHook`, and `/tmp/tracetools/`). Three
undocumented Z80 behaviours were also fixed in zen80 (0.5.0) after
being caught corrupting a real protection scheme's keystream.

Corpus result: **19/27** tracked titles verify in accurate mode
(45/81 mode-passes across fast/accurate/turbo). This did not change
materially as a *result* of the fixes below — the eight remaining
failures are unrelated to what was fixed (see
[Corpus impact](#corpus-impact-and-what-remains) below) — but every
fix stands on independent measurement, not on the corpus pass/fail
signal.

## Fixes landed, each with what was measured

**ULA contention (closes T-16).** Was entirely absent. Now implements
the WoS FAQ model for 48K: memory pattern `{6,5,4,3,2,1,0,0}` from
cycle 14335 over `0x4000-0x7FFF`, and I/O contention as four cascaded
`C:1` rounds per the FAQ's port table. Wired through zen80 0.5.0's new
offset-corrected contention hooks (within-instruction access-position
tracking, not just instruction-start). Measured against 4.2M real
instructions from the Speedlock loader workload: position-error bounds
of **~0.33% for memory contention and ~0% for I/O**, versus ~6% and
~20% at plain instruction-start positions. 128K uses the same model
extrapolated to an address-range approximation — not bank-aware, frame
geometry unverified (T-20).

**Frame timebase.** Was two clocks precessing against each other:
70000-cycle frames with INT at frame *end*, while contention wrapped a
cumulative counter mod 69888 — a phase drift of 112+ T-states per
frame. Now a single coherent timebase: `/INT` asserted for 32 T-states
at frame *start*, real 69888-cycle PAL frames, one `frameOrigin`
anchoring contention, the floating bus, border striping, and snapshot
restore. TS2068 keeps its 58688-cycle frame on the same model.

**EAR idle feedback.** With the tape not playing, port `0xFE` bit 6
now presents the level fed back from the last ULA `OUT`, matching
Issue 2/3 hardware behaviour (Issue 3 default: bit 4; Issue 2: bits
4|3; +2A/+3: always low) — the exact signal Speedlock-class protection
checks probe. While the tape plays, it drives bit 6 as before.

**Floating bus.** Was absent; unattached port reads returned a fixed
idle value regardless of the ULA's current fetch. Now mirrors FUSE's
`spectrum_unattached_port`: returns the byte the ULA is fetching at
the current frame-relative T-state, frame-origin anchored, gated off
for +2A/+3 (no floating bus there) and TS2068. Granularity
(instruction-start, not true T-state-exact) and 128K port-decode
subtleties are unmodeled (T-21).

**ULA port idle bits.** Bits 5 and 7 now read high (idle `0xBF`),
matching real hardware and FUSE; bit 5 previously read low.

**TZX pulse generation — verified against an independent spec
implementation.** A static audit built a second, from-scratch TZX
pulse generator directly from the TZX 1.20 spec (`tzx_expect.py`,
sharing no code with zenzx) and diffed it against zenzx's actual
generated stream for the Batman corpus tape (828,282 pulses). This
caught a fossil: `genCustomPulses` still inlined a pre-fix SpecIde
pause pattern that an earlier pause-level fix had missed at one call
site, producing a 1ms HIGH excursion and a parity shift that inverted
the polarity of an entire 47KB payload region. After the fix: **the
generated stream matches the independent spec expectation exactly —
828,282 = 828,282 pulses, zero mismatches**, across the whole tape.
Also fixed in the same pass: a zero-length pulse (a pure level flip
with no duration) could be deferred past a `Tick` budget boundary,
leaving a transient level observable for one instruction — a real bug,
confirmed via the same instrumentation to be corrupting one EAR sample
mid-load, though not the tape's root failure cause.

**Tape delivery, verified end-to-end.** Per-frame instrumentation
(`ZTRACE_TAPELOG`) confirmed `io.tapeEar` matches `tape.st.EarLevel` at
every one of 12,477 logged frames spanning a full load — zero seam
disagreements between tape state and what the CPU actually reads.

**Cross-emulator instruction agreement.** With loader-timing
conveniences on both sides identified and controlled (FUSE's
`accelerate_loader` and `detect_loader`, both on by default — the
latter, confirmed by direct test, is FUSE's *own* substitute for
human PLAY-button timing, not a shortcut invalidating comparison), a
60,000-instruction window of zenzx and continuous-tape FUSE runs in
exact **instruction lockstep**: identical PC sequence, identical
registers including R, zero drift measured over tens of thousands of
records. A parallel 41,027-record RAM-write stream (address/old/new,
filtered to writing-PC ≥ `0x5B00`) matches identically between pre-
and post-fossil-fix runs, confirming the fossil fix changed encoding
correctness without altering *this* title's control flow.

**Z80 undocumented-flag and refresh-timer fixes (zen80, unreleased).**
Caught auditing behaviour that a real protection scheme (Speedlock)
deliberately probes:
- `BIT n,(HL)`'s undocumented X/Y flags now take their value from
  WZ's (MEMPTR's) *inherited* high byte, left by earlier instructions
  — not from a WZ refresh keyed to HL, which a previous fix had
  introduced. Real hardware does not refresh MEMPTR on a plain CB
  `(HL)` operation.
- `HALT` now advances R on every M-cycle it executes, matching a
  halted Z80's continuous NOP M1 fetches. R was previously frozen for
  the duration of a HALT.
- `JP cc,nn` and `CALL cc,nn` now set WZ from the fetched operand
  unconditionally, including when the condition is not met — WZ is a
  side effect of the operand fetch, not of the branch. Both
  instructions previously left WZ stale on the not-taken path.

Each has a dedicated test (`TestBIT_HL_XYFromWZHigh`,
`TestHALT_RAdvances`, `TestJPcc_NotTaken_StillSetsWZ`,
`TestCALLcc_NotTaken_StillSetsWZ`); full zen80 suite green throughout.

## Corpus impact and what remains

19/27 tracked titles verify in accurate mode. The eight that don't:

| Title | Loader family |
|---|---|
| Cobra | Speedlock-class (Ocean/Imagine) |
| WEC Le Mans | Speedlock-class |
| Robocop | Speedlock-class (fast mode alone verifies) |
| The Great Escape | Speedlock-class |
| Batman | Speedlock-class |
| Sabre Wulf | Custom loader |
| Lotus Esprit Turbo Challenge | Custom loader |
| R-Type | Custom loader |

The five Speedlock-class failures share a mechanism, investigated in
depth for Batman below but **deliberately left open**: stage-3
of the loader decrypts its own code via a keystream that folds Z80
refresh-register (R) residue through flag state. R's value at that
point is a function of the entire boot's HALT-frame-sync history,
which is legitimately boot-variant hardware behaviour — not
necessarily a zenzx defect. Whether the protection's algebra cancels
this R-dependence downstream (making any correct boot load
successfully) or is a genuine per-boot lottery is an open question,
filed as T-22, not resolved by the fixes above. Sabre Wulf, Lotus, and
R-Type use unrelated custom loaders, not yet investigated.

## Instrumentation left in place

All measurement above is reproducible in ~90-second runs, not
one-off: the FUSE reference build's `zenzx_debug.c` (env-driven:
entry PC, step/write/IO budgets, address and writing-PC filters),
zenzx's `zzz_trace_harness_test.go` (opt-in via `ZTRACE_OUT`, mirrors
every FUSE knob) and `zzz_pulsedump_test.go`, `/tmp/tracetools/
tracediff.py` (sequence-aligned PC/register/write-stream diffing with
16-bit store-order tolerance) and `tzx_expect.py` (independent TZX
generator), and `/tmp/tracetools/run_compare.sh` (one-command
both-sides-plus-diff). zen80's `DebugPCHook`/`DebugMemWriteHook`/
`DebugIOInHook` are permanent, nil-default, and documented in its
CHANGELOG.

## See also

- `TRACKING.md` — T-16 (closed 2026-08-25), T-20, T-21, T-22
- `RESOLVED.md` — T-16's full resolution record
- `KNOWN_ISSUES.md` — dormant guards and recorded limits
- zen80 `CHANGELOG.md` — the `[Unreleased]` Z80-core fixes
