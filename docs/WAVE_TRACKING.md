# Waves

## 1. Progress at a glance

```
Wave 1  trace harness generalization  ████████████████████   100%  (10/10 items)
Wave 2  advanced tracing features (priced, unscheduled)  ░░░░░░░░░░░░░░░░░░░░     0%  (0/6 items)
```

Overall by item count: 10 of 16 items ≈ **62%**

### Wave 1 — trace harness generalization (10 items, ideal 8.0d, added 2026-08-28)

| # | Summary | Status | Register item |
|---|---|---|---|
| 1 | Wire up M1Hook for interrupt and prefix-fetch visibility | ✓ | T-26 |
| 2 | Pluggable setup (snapshot/bin/boot-only, not just tape-load) | ✓ | T-26 |
| 3 | Optional pre-setup hook installation (trace from cold reset) | ✓ | T-26 |
| 4 | Memory-read and port-write tracing | ✓ | T-26 |
| 5 | Register-equality stop conditions | ✓ | T-26 |
| 6 | Coverage summary mode | ✓ | T-26 |
| 7 | Call-stack reconstruction | ✓ | T-26 |
| 8 | Symbol-table-aware trace output (zenas --sym) | ✓ | T-26 |
| 9 | Watchpoints as stop conditions | ✓ | T-26 |
| 10 | Snapshot-on-trigger (SaveSnapshot .zxs, SaveZ80, SaveSNA) | ✓ | T-26 |

**Wave 1: complete 2026-08-29.** All 10 items implemented and verified
this session (hand-checked expected values, or cross-checked against
`zenas`/`BootDetector` directly — see
`docs/TRACE_GENERALIZATION_TRACKING.md` for what was checked per stage).
Items 4 and 9 were briefly ◐ pending zen80 v0.5.5 being pushed and
tagged — confirmed live on GitHub and resolvable via the real Go module
proxy 2026-08-29, `go.mod`'s local `replace` removed, `gomod.py check`
clean, and both items re-verified against the real published module
(not just the local dev checkout that originally proved them).

### Wave 2 — advanced tracing features (priced, unscheduled) (6 items, ideal 35.0d, added 2026-08-28)

| # | Summary | Status | Register item |
|---|---|---|---|
| 11 | Opcode disassembly / mnemonic decoding (zendis) -- High | ☐ | not yet filed |
| 12 | Multiple simultaneous trace windows in one run -- Low-Moderate | ☐ | not yet filed |
| 13 | General conditional-breakpoint expression language -- Moderate | ☐ | not yet filed |
| 14 | Ring-buffer / crash-log tracing -- Moderate | ☐ | not yet filed |
| 15 | Live/interactive debugger (step, continue, REPL) -- Very high | ☐ | not yet filed |
| 16 | Reverse/step-back execution -- Very high, likely not worth it | ☐ | not yet filed |

**Wave 2: 0/6, not started.**

---

