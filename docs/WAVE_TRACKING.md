# Waves

## 1. Progress at a glance

```
Wave 1  trace harness generalization  ████████████████████   100%  (10/10 items)
Wave 2  advanced tracing features (priced, unscheduled)  ░░░░░░░░░░░░░░░░░░░░     0%  (0/6 items)
Wave 3  ti0.r0 — Z80N CPU support (adjustments)  ░░░░░░░░░░░░░░░░░░░░     0%  (0/1 items)
Wave 4  ti0.r1 — NextREG port handler  ████████████████████   100%  (3/3 items)
Wave 5  ti0.r2 — Extended MMU     ████████████████████   100%  (4/4 items)
Wave 6  ti0.r3 — Layer 2 graphics  ████████████████████   100%  (3/3 items)
Wave 7  ti0.r4 — Hardware sprites  ████████████████████   100%  (3/3 items)
Wave 8  ti0.r5 — Next palette     ████████████████████   100%  (2/2 items)
Wave 9  ti0.r6 — NEX file loading  ░░░░░░░░░░░░░░░░░░░░     0%  (0/3 items)
```

Overall by item count: 25 of 35 items ≈ **71%**

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

### Wave 3 — ti0.r0 — Z80N CPU support (adjustments) (1 item, ideal 0.5d, added 2026-09-14)

| # | Summary | Status | Register item |
|---|---|---|---|
| 17 | placeholder -- audit Z80N wiring as a real machine mode (currently a manual -z80n CPU-only flag) and confirm no gap beyond opcode semantics; scope not yet defined | ☐ | not yet filed |

**Wave 3: 0/1, not started.**

### Wave 4 — ti0.r1 — NextREG port handler (3 items, ideal 1.0d, added 2026-09-14)

| # | Summary | Status | Register item |
|---|---|---|---|
| 18 | NextREG register file + full 16-bit port match at 0x243B/0x253B in io.go | ✓ | T-27 |
| 24 | regression confirming the Z80N NEXTREG instruction reaches the new handler via ioOut | ✓ | T-27 |
| 25 | per-register read-back semantics matching the SpecNext wiki, register by register | ✓ | T-27 |
**Wave 4: 3/3, done.**

### Wave 5 — ti0.r2 — Extended MMU (4 items, ideal 4.0d, added 2026-09-14)

| # | Summary | Status | Register item |
|---|---|---|---|
| 19 | 8-slot, 8K-granularity paging model replacing ramBankLow/High/Top | ✓ | T-28 |
| 26 | wire NextREG MMU slot registers (0x50-0x57) through the new paging model | ✓ | T-28 |
| 27 | extension port 0xdffd for banks beyond 128K's original eight | ✓ | T-28 |
| 28 | regression confirming 48K/128K/+3 paging unaffected | ✓ | T-28 |

**Wave 5: 4/4, done 2026-09-15 (v0.7.0).** T-28 closed to RESOLVED.md.

### Wave 6 — ti0.r3 — Layer 2 graphics (3 items, ideal 3.0d, added 2026-09-14)

| # | Summary | Status | Register item |
|---|---|---|---|
| 20 | Layer 2 framebuffer type (256x192, 8-bit colour), base bank configurable via NextREG | ✓ | T-29 |
| 29 | clip window support (NR 0x18) | ✓ | T-29 |
| 30 | composite Layer 2 with the ULA layer per NR 0x15 priority bits | ✓ | T-29 |

**Wave 6: 3/3, done 2026-09-15.** T-29 stays open at ◐ in TRACKING.md. Update 2026-09-15: a completion pass closed the wider Layer 2 modes' data-plane addressing and IO port 0x123B (both listed here as out of scope when this wave closed); the real Next palette gap was actually already closed earlier by T-31, not by this pass. A second, corrective pass the same day closed the GUI window/viewport wiring gap the first pass left open (fixed reference canvas, correct half-width-pixel geometry for 640x256x4bpp mode confirmed against an exact wiki quote, and a scissor-bleed regression caught and fixed before shipping) -- see T-29's own two completion-pass notes in TRACKING.md for the full detail, including a real self-correction mid-pass (an initial fix assumed uniform pixel width across modes and was wrong). T-29 now stays open at ◐ for three items, none in this wave's original list: the X/Y scroll register gap, the headless/CPU decode path's still-fixed-256x192 assumption (a separate, pre-existing gap only surfaced while checking this pass's scope), and NR 0x18's 8-bit clip window not reaching past column 255.

### Wave 7 — ti0.r4 — Hardware sprites (3 items, ideal 4.0d, added 2026-09-14)

| # | Summary | Status | Register item |
|---|---|---|---|
| 21 | sprite attribute table (position, pattern index, palette offset, visibility) | ✓ | T-30 |
| 31 | sprite pattern memory (16x16x8-bit per sprite) | ✓ | T-30 |
| 32 | composite sprites into the ti0.r3 layer-priority path | ✓ | T-30 |

**Wave 7: 3/3, done 2026-09-15.** T-30 stays open at ◐ in TRACKING.md. Update 2026-09-15: a completion pass closed scaling, rotation, mirroring, both composite and unified relative sprites, attribute 4, sprite Y-position MSB, and NR 0x4B configurability -- all listed here as out of scope when this wave closed -- see T-30's own completion-pass note in TRACKING.md. T-30 now stays open at ◐ only for 4-bit sprite colour patterns, the alternate hardware port-0x303B/0x57/0x5B access path, and NR 0x19 (sprite clip window), none of which were in this wave's original item list.

### Wave 8 — ti0.r5 — Next palette (2 items, ideal 1.5d, added 2026-09-14)

| # | Summary | Status | Register item |
|---|---|---|---|
| 22 | NextPalette type in pkg/zxpalette (9-bit RGB), alongside the existing classic palette | ✓ | T-31 |
| 33 | 8 palettes (ULA/Layer2/Sprite/Tilemap x first/second), 256 entries each | ✓ | T-31 |

**Wave 8: 2/2, done 2026-09-15.** T-31 stays open at ◐ in TRACKING.md (ULA/Tilemap palette rendering is out of this wave's scope). Update 2026-09-15: NR 0x4B configurability, listed here as out of scope, was closed by the same completion pass that closed Wave 7's remaining sprite gaps -- filed against T-30 in TRACKING.md, since NR 0x4B is sprite transparency, not palette, machinery.

### Wave 9 — ti0.r6 — NEX file loading (3 items, ideal 2.0d, added 2026-09-14)

| # | Summary | Status | Register item |
|---|---|---|---|
| 23 | NEX v1.0-1.2 header parsing | ☐ | T-32 |
| 34 | direct page loading into the ti0.r2 MMU | ☐ | T-32 |
| 35 | Layer 2 screen/palette-from-header | ☐ | T-32 |

**Wave 9: 0/3, not started.**

---

