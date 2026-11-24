# Trace harness generalization — stage tracker

Version: 0.6.10
Last reviewed: 2026-08-29

Stage-status table for `docs/TRACE_GENERALIZATION_DEVELOPMENT_PLAN.md`
(frozen once Stage 1 begins; record deviations here, don't edit the plan
to match reality). Status legend: ✓ done · ◐ partial · ☐ not started ·
✗ dropped.

| Stage | Name | Status | Notes |
|---|---|---|---|
| 1 | Wire up `M1Hook` for interrupt/prefix-fetch visibility | ✓ | Implemented in `installM1Hook` (`zzz_trace_callstack_test.go`), shared with Stage 7. Verified: real M lines with correct opcode/context ("normal") against a hand-assembled program. |
| 2 | Pluggable setup (stop assuming tape-load) | ✓ | `ZTRACE_SETUP`/`ZTRACE_MODEL` implemented. `bin`, `snapshot`, `boot-only` verified directly (hand-assembled program; a real `testdata/snapshots/*.z80`; `128k` model). `tape` mode is structurally unchanged from the original working code but not re-exercised this session — no tape corpus available in this sandbox. |
| 3 | Optional pre-setup hook installation | ✓ | `ZTRACE_FROM_RESET` implemented. Verified rigorously: PC-pattern eyeballing was actively misleading here (a transient early loop looked like "ready" at step ~20 but wasn't) — cross-checked against the codebase's own `BootDetector` instead, which gave a real number: 48K boot-to-ready is 768,551 steps over 87 frames. The trace's own transition into the idle loop lands at exactly that step, confirming both the measurement and the implementation. `ZTRACE_MAX_STEPS`'s default is now mode-aware: 6000 (unchanged) normally, 1,000,000 when `ZTRACE_FROM_RESET=1`, based on that measurement. |
| 4 | Memory-read and port-write tracing | ✓ | `DebugMemReadHook`/`DebugIOOutHook` wired (`R`/`O` lines). Verified with exact hand-checked values (port `0x55FE` for `OUT (0xFE),A` with A=0x55, matching `decode.go`'s own `addr := uint16(port) \| (uint16(cpu.A) << 8)`). Was ◐ pending zen80 v0.5.5 landing as a real tag — confirmed live 2026-08-29 (`git ls-remote`, and resolvable via `https://proxy.golang.org/github.com/ha1tch/zen80/@v/v0.5.5.info` directly), `go.mod`'s local `replace` removed, `go clean -modcache && go mod tidy` pulled it fresh with a real checksum in `go.sum`, `gomod.py check` now clean, and the R/W/watchpoint behavior re-verified against the real published module. |
| 5 | Register-equality stop conditions | ✓ | `ZTRACE_STOP_A/BC/DE/HL/SP` implemented, OR semantics. Verified: `ZTRACE_STOP_HL=1234` disarmed at exactly the step HL became `0x1234`, before the following instruction executed. |
| 6 | Coverage summary mode | ✓ | `ZTRACE_COVERAGE_OUT` implemented. Verified against the plan's own stated bar: the coverage count for a hot-loop address matched a manual `grep -c` count from the corresponding P-line trace exactly (43 both ways). |
| 7 | Call-stack reconstruction | ✓ | `ZTRACE_CALLSTACK` implemented (CALL/RST/RET/RETN/RETI classification, conditional-call/ret condition evaluated from `F` at fetch time since CALL/RET don't modify flags). Verified with a real CALL→RET round trip: depth 0→1→0, and the pushed/popped return address matched exactly (`0x8005`, the byte right after the 3-byte CALL). |
| 8 | Symbol-table-aware trace output | ✓ | `ZTRACE_SYMFILE` implemented, pasmo format confirmed byte-for-byte against a real `zenas --sym` build this session (`label\t\tEQU 0XXXXH`), not assumed. Verified end-to-end: assembled a program with zenas, traced it, got `P 3 8009[sub]` — the label zenas itself placed at that exact address. |
| 9 | Watchpoints as stop conditions | ✓ | `ZTRACE_STOP_WADDR` implemented (W-line and O-line trigger). Verified: disarmed at the exact step address `0xFFFE` was written (the CALL's own stack push) -- re-verified against real zen80 v0.5.5 too, same result. No longer inherits any caveat now that Stage 4 is resolved. |
| 10 | Snapshot-on-trigger | ✓ | `SaveSnapshot`/`SaveZ80`/`SaveSNA` wired via `ZTRACE_SNAPSHOT_OUT`'s extension, centralized in a `disarm()` helper so every stop path (budget/stop-condition/watchpoint) triggers it consistently. Verified: a real `.zxs` file was written, reporting the exact PC (`800D`) where the run was stopped by `ZTRACE_STOP_HL`. |

## Deviations from the plan

- **Stage 4/9 briefly shipped as ◐ (0.6.9), then resolved to ✓
  (2026-08-29)** once zen80 v0.5.5 was actually pushed and tagged —
  confirmed via `git ls-remote` and the real Go module proxy, not just
  taken on the person's word. `go.mod`'s local `replace` removed,
  dependency re-pulled from scratch (`go clean -modcache && go mod
  tidy`) with a real `go.sum` checksum, and both stages' behavior
  re-verified against the published module rather than assuming the
  local dev copy and the published one behave identically.

- **`ZTRACE_BINADDR`/`ZTRACE_BINSTART` use `ParseAddr`/`ParseAddrSigned`
  (hex `0x..` or decimal), not a hex-only parser as first implemented.**
  The plan said "hex load address" for `ZTRACE_BINADDR`; the first
  implementation matched that literally and silently misparsed
  `ZTRACE_BINSTART` as decimal (a real bug caught by testing, not
  reasoning) — fixed by reusing the CLI's own `-binaddr`/`-binstart`
  parser exactly instead of inventing a second one.
- **Non-tape setups default `ZTRACE_ENTRY` to wherever the loaded state
  actually left PC, not the tape-specific `0x5D15`.** Not specified
  either way in the plan; without this, `ZTRACE_SETUP=snapshot` never
  armed at all in testing, since a game snapshot's PC is never
  `0x5D15`. Known gap: combined with `ZTRACE_FROM_RESET` and no
  explicit `ZTRACE_ENTRY`, the setup phase runs before this default is
  computed — documented in the harness's own doc comment, not solved.
- **`ZTRACE_MAX_STEPS`'s default is mode-aware** (6000 normally,
  1,000,000 for `ZTRACE_FROM_RESET`) — the plan flagged this as "size
  during implementation," done via direct measurement against
  `BootDetector`, not guessed.
- **Call-stack depth uses a new `S` line**, not folded into `P` lines —
  the plan left this open ("pick whichever keeps `tracediff.py`
  compatibility simplest when this is built"); a separate line type
  needs no change to `P`'s existing field count at all, which seemed
  the lower-risk choice for an external, unversioned diff tool.
- **Stage 4 (and transitively Stage 9) depend on an unpushed local
  zen80 checkout** via a `go.mod` `replace` directive, not the tagged
  release the plan's own "Blocked on" line specified. Deliberate,
  matching this repo's own established pattern for this exact
  situation (see the 0.6.7 CHANGELOG entry) — but genuinely not what
  "done" meant when the plan was written, hence both stay ◐.
