# ZenZX — resolution record

Closed register items, newest first. Each entry is the item's detail
text verbatim as at closure, stamped with the closing version and date.
The changelog says what shipped; this record says what was wrong and
how it was resolved. Append-only.

## [0.6.6] T-25 — GUI build's -tapemode flag never gained a turbo case; passing -tapemode=turbo to the GUI silently falls back to fast instead of erroring or working (v0.6.6, 2026-08-27)

Theme: tape · closed 0.6.6 · 2026-08-27


Found auditing README.md/MANUAL.md for staleness: zenzx_gui.go's own -tapemode flag.String default/help text still says only "fast or accurate", and the only comparison in its parsing is 'if strings.ToLower(*tapeMode) == "accurate"' -- any other value, including "turbo", falls through to fast with no warning. Turbo mode itself (loadingFrame + zen80 FastMem, T-19 fixed for 128K) is headless-only as a result, not by documented design -- there's no indication this was a deliberate scope decision, just that GUI wiring was never added when turbo was built. Low priority since turbo's actual value (a real but modest ~15-27% wall-clock speedup, not a correctness improvement) matters most for batch/headless use where it already works; but the silent fallback (no error, no warning) means a GUI user who tries -tapemode=turbo gets fast mode's very different behavior without being told.

Cross-ref: CHANGELOG 0.6.6.

## [0.6.0] T-16 — No memory contention modeling exists for any model (v0.6.0, 2026-08-25)

Theme: timing · closed 0.6.0 · 2026-08-25


- **Trigger:** 2026-08-17, surfaced while scoping TS2068 Stage 6 (docs/TS2068_TRACKING.md). Checked directly rather than from memory: no contention modeling exists anywhere in zenzx or zen80 (the CPU core), for any model -- not 48K, not 128K/+3, not TS2068. Every memory access currently takes the same fixed number of T-states regardless of address or where the ULA is in its own scanline at that moment.
- **Scope, deliberately general, not TS2068-specific:** TS2068 Stage 6 was originally framed as "TS2068's own contention pattern, not the standard ULA's" -- but there is no standard-ULA baseline to diverge from either. Building contention only for TS2068 would leave it more cycle-accurate than the models it shares a substrate with, a strange asymmetry to introduce as a side effect of one model's checklist. This item covers the real gap: general per-address, per-scanline contention timing, threaded into the CPU stepping loop, correctly synchronized with ULA scanline position -- new infrastructure affecting every model equally, not an adaptation of an existing table.
- **Impact, honestly scoped:** affects cycle-exact border effects, 128K "snow," and a handful of extremely timing-sensitive demos/loaders that depend on exact contention stalls. Does not affect ordinary program correctness -- BASIC, most games, and everything verified across TS2068 Stages 1-5 works fine without it. Not a functional blocker for any current feature.
- **Fix:** design a per-address/per-cycle contention timing table for each model's own ULA (48K, 128K/+2/+2A, +3, and eventually TS2068's own SCLD-driven pattern per Technical Manual 2.1.8.2/2.1.8.3), integrate into the CPU step loop's cycle accounting, verify against known contention-sensitive test cases (e.g. the community's standard contention test ROMs) before trusting it.

Cross-ref: CHANGELOG 0.6.0.

## [0.5.1] T-19 — Turbo tape mode (-tapemode=turbo) fails on 128K titles; 48K confirmed correct (v0.5.1, 2026-08-24)

Theme: tape · closed 0.5.1 · 2026-08-24


**Trigger:** Building Turbo mode (zen80 FastMem/FastPort fast path + zenzx per-block memory/paging sync, turbo.go) for accelerated tape-loading batch runs. 48K validated correct (Chase HQ: byte-identical outcome to accurate mode, confirmed via direct screenshot comparison and full memory diff, after fixing a real zen80 decode.go bug -- 16 call sites bypassing the fast path entirely due to a receiver-variable-name mismatch the original mechanical substitution missed).

128K fails: reproduced with Cybernoid 2 (Cybernoid II - The Revenge/Cybernoid 2 - Side 1.tzx, -model=128k). Symptom: CPU ends up stuck at PC=0x0049 (idling in the interrupt handler) instead of the game's own code, despite Tape.Position reaching the tape's natural end -- the same symptom class the decode.go fix resolved for 48K, but confirmed to be a DIFFERENT root cause:
  - A comprehensive grep across every zen80 file for any remaining direct <recv>.Memory./<recv>.IO. call (any receiver name, not just z./cpu.) found nothing else missed -- the CPU core itself is confirmed complete.
  - Not a bank-switching/paging bug: traced with a temporary ioOut hook: zero writes to port 0x7FFD occur during the entire observed loading window, so turboSyncBlock's paging-port replay was never even exercised.
  - Snapshot diff (zx.SaveSnapshot at the same checkpoint in both modes) confirms accurate mode reaches real game code (PC=0xF9D3) while turbo mode does not, at a substantial (~35,768 T-state) cycle-count divergence -- a genuine stall, not just interrupt-timing sampling noise.

**Root cause (found after the above was filed):** the original write-based reproduction was itself unreliable -- traced to a flawed test-harness boot wait (a fixed 150-frame guess rather than a proper BootDetector), which caused both modes to fail identically in a masking way and produced a false "zero writes to port 0x7FFD" reading. With correct boot detection, Cybernoid 2 genuinely, actively switches RAM and ROM banks via port 0x7FFD *during ongoing code execution* -- not just between tape blocks (47 paging writes observed in the first 4000 turbo frames alone, including rapid ROM-bank toggling and sequential RAM-bank cycling). The flat 64K FastMem snapshot, synced only at tape-block boundaries, structurally could not represent this: between syncs, any bank switch left FastMem showing stale data for whichever bank had just been paged out.

**Fix:** replaced the flat-snapshot model with a page-based one matching SpectrumMemory's own real layout exactly -- a fixed pool of 12 independently-addressable 16K pages (turbo_pages.go: `MemoryPageCount`, `MemoryPage16k`, `MemoryFlags`; pages 0-3 = ROM banks 0-3, pages 4-11 = RAM banks 0-7), with a `turboSlotPage[4]` tracking which page is currently mapped at each of the four logical 16K slots. zen80 gained a new hook, `FastPortWriteOut` (fires on every OUT while FastPort is active, mirroring FastPortReadIn's existing design), letting turboRemap fire immediately the moment a paging write actually happens, not at the next block boundary -- swapping FastMem's affected slot to the newly-selected page there and then, preserving whatever the loader had just written to the page being paged out. turboSyncBlock (renamed turboSyncToReal) now writes back all 12 pages, not just the four currently mapped, since a bank the loader paged out earlier still needs its data to land in real SpectrumMemory. Scope: standard 128K/+2/+3 paging via port 0x7FFD only; +3's "special" all-RAM paging mode (0x1FFD) and TS2068's separate Extension-ROM scheme are out of scope for now, both being rare for tape-based titles specifically.

Verified: Cybernoid 2 now reaches real game code (PC=0xF9D9, IM=2, I=0xFE -- identical to accurate mode) and produces a screenshot pixel-identical to the accurate-mode ground truth, via the real CLI end to end. Chase HQ (48K) re-verified still correct after the rewrite. Performance: ~14-17% turbo-vs-accurate speedup (down from ~23% pre-fix on 48K, due to the small added per-OUT check in FastPortWriteOut) -- a reasonable trade for 128K correctness, which was completely broken before this fix, not merely slow.

Cross-ref: CHANGELOG 0.5.1.

## [0.4.35] T-18 — FLASH attribute stopped blinking on every model (not just TS2068) -- updateFlash() documented as called from Render but no call site existed (v0.4.35, 2026-08-18)

Theme: video · closed 0.4.35 · 2026-08-18


screen.updateFlash() advances flashTickTock on a 320ms cadence and is the sole driver of the classic FLASH ink/paper swap. videorender.go's design comment documents it as "called once per frame from the GUI's live path (zx.Render(), via DisplayManager) unconditionally regardless of which renderer is active" -- but no such call site existed anywhere in the codebase. flashTickTock never advanced past its initial value, so FLASH-attributed cells never blinked, on every model including standard 48K/128K/+2/+2A/+3, not just TS2068. Caught because TS2068 hi-colour work correctly added TestHicolourFlashIgnored (confirming hi-colour deliberately ignores FLASH) but no equivalent test existed confirming standard mode actually honours it -- a coverage gap, not a design gap. Fixed: added the missing dm.screen.updateFlash() call as the first line of display.go's Render(). 4 new tests (flash_regression_test.go): standard-mode swap correctness, flashEnabled=false fully disabling (not just un-ticking), and updateFlash()'s own 320ms timing directly.

Cross-ref: CHANGELOG 0.4.35.

## [0.4.23] T-12 — -model ts2068 not implemented (own ROM, 8K-chunk banking, dedicated AY ports) (v0.4.23, 2026-08-17)

Theme: model · closed 0.4.23 · 2026-08-17


- **Trigger:** 2026-08-17, deferred from T-11 (hi-colour mode is testable and implementable against the existing models without a TS2068 boot ROM; this is a separate, larger feature).
- **ROMs in-tree, 2026-08-17:** `rom/ts2068-0.rom` (16384 bytes, Home) and `rom/ts2068-1.rom` (8192 bytes, Extension) -- verified sizes, sensible Z80 reset vectors, expected embedded copyright strings. Provenance and copyright status in `rom/TIMEX.txt`.
- **Staged plan, 2026-08-17:** `docs/TS2068_DEVELOPMENT_PLAN.md` (frozen design) and `docs/TS2068_TRACKING.md` (stage status) -- 6 stages: model skeleton/ROM loading/chunk-0 banking/boot; NTSC frame timing; `CHNG_VID` (the only deferred-list item in scope, per direction -- everything else deferrable stays unimplemented indefinitely: full 8-chunk Dock/cartridge banking, the TS2040 printer protocol, composite/RF video generation); AY ports + joystick; tape I/O; memory contention. Guiding principle recorded in the plan: because zenzx runs the *real* ROM images, most of what looks like "implement ROM service X" is actually "get the memory/port/timing substrate correct and let the real ROM code do it" -- this reshapes `CHNG_VID` specifically from "reimplement this routine" to "verify it executes correctly once chunk-0 banking is right."
- **Stages 1-2 done, 2026-08-17** (`ts2068.go`, `ts2068_test.go`): boots to the real copyright screen and, confirmed by injecting a keypress, to a genuinely responsive BASIC-ready prompt (`PRINT 9` [ENTER] produces `9` and `0 OK`) -- not just a static screenshot that looks the same whether idling correctly or stalled. Genuine NTSC frame timing (58688 cycles/frame, not the PAL default) verified per-instance, every other model confirmed unaffected. Detail in `docs/TS2068_TRACKING.md`.
- **Stage 3 done, 2026-08-17, rescoped from the original plan** (`ts2068.go`, `ts2068_test.go`): real TS2068 software engaged hi-colour mode via a direct port `FFH` write, not the documented Extension ROM `CHNG_VID` service -- and the hardware never auto-clears the screen, so real software (and this test) cleared the hi-colour plane by hand. Implemented dynamic renderer switching on the guest's own port write. Verified end to end with garbage pre-seeded in the framebuffer to prove the clear is real, not assumed. The full real-service-call route was attempted and hit a genuine, undiagnosed divergence -- filed separately as T-15, not blocking since real software didn't depend on that path either.
- **Stage 4 done, 2026-08-17** (`ts2068.go`, `ts2068_test.go`, `input.go`): AY sound chip at `F5H`/`F6H` reusing the existing AY emulation via a new dispatch path only; joystick via AY register 14, both ports (address bit 8/9), active low, always on for this model. Detail in `docs/TS2068_TRACKING.md`.
- **Stage 5 done, 2026-08-17** (`tape.go`, `tape_fastload_test.go`): fast mode reuses T-02's hardened `trapLoad` unchanged -- `R_TAPE`/`W_TAPE` confirmed by direct disassembly to be a byte-for-byte relocated copy of the standard ROM's tape routines, same register contract, only the trap addresses and a new chunk-0-aware context check differ. Accurate mode confirmed genuinely working with zero TS2068-specific code, both verified via real ROM execution (not assumption). Detail in `docs/TS2068_TRACKING.md`.
- **Stage 6 postponed, 2026-08-17, spun out as a general item.** Investigation found no memory contention modeling exists for *any* model, zenzx or zen80 alike -- meaning this was never really "TS2068's own pattern vs the standard one," since there is no standard-model baseline to diverge from. Filed as T-16, deliberately not TS2068-scoped, rather than built one-off here (which would have left TS2068 more cycle-accurate than the models it shares a substrate with for no good reason). Affects cycle-exact border effects and a handful of timing-sensitive demos only, not ordinary program correctness -- everything verified across Stages 1-5 already works without it.
- **Closed, 2026-08-17: 5 of 6 stages complete, the 6th a deliberate, recorded decision (T-16) rather than a stalled task.** `-model ts2068` boots to a genuinely responsive BASIC prompt, with real NTSC timing, dynamic hi-colour switching, AY sound and built-in joystick, and both tape modes -- all verified against the real ROM throughout, not assumed. `README.md`, `docs/KNOWN_ISSUES.md`, and `smoke_headless.sh` (the release-gate boot check) updated to reflect TS2068 as a supported model, not just code that happens to exist.

Cross-ref: CHANGELOG 0.4.23.

## [0.4.21] T-02 — ROM-trap tape LOAD path unverified end to end (v0.4.21, 2026-08-17)

Theme: tape · closed 0.4.21 · 2026-08-17


- **Trigger:** README Known issues (0.4.1). The instant-inject fast-tape path is verified byte-identical for `.tap`/`.tzx` CODE blocks. The separate ROM-trap path that intercepts a guest's own `LOAD ""` from BASIC in `-tapemode=fast` is present but has never been driven end to end.
- **Fix:** write a headless zenscript that types `LOAD ""` against a known tape and asserts on screen state; either verify the trap or remove the dead path.

Cross-ref: CHANGELOG 0.4.21.

## [0.4.17] T-14 — AMX mouse not implemented (raw quadrature encoder via Z80 PIO, not a position register) (v0.4.17, 2026-08-17)

Theme: input · closed 0.4.17 · 2026-08-17


- **Trigger:** 2026-08-17, deliberately deferred from the -mouse kempston implementation (0.4.14). -mouse amx is a recognised flag value that hard-errors with an explanation (mouse.go's ParseMouseMode), rather than being silently accepted and doing nothing.
- **Why this is a different, harder problem than Kempston Mouse:** per the Sinclair Wiki's AMX Mouse technical page, AMX uses a Z80 PIO chip and exposes movement as a raw quadrature-encoded signal, not an accumulated position register: port 0x1F bit 0 carries X-axis quadrature (note: same port as Kempston Joystick -- real hardware conflict, would need explicit mutual exclusion if both are ever selectable together), port 0x3F bit 0 carries Y-axis quadrature, port 0xDF carries the 3 buttons. Software decodes direction from the relative phase/timing of transitions on these bits, sampled repeatedly -- fundamentally a bit-level timing problem, not a value-write problem the way Kempston's three byte-ports are.
- **Existing-emulator research (2026-08-17, condensed):** before the primary source below was available, checked four active open-source emulators (Fuse, ZEsarUX, SpecEmu, SpecIde) directly in source -- none had a working AMX implementation; only a vestigial, unused `.szx` snapshot-format chunk in Fuse's `libspectrum`. Full trail in `docs/RESOLVED.md` once this closes.
- **Primary source obtained, 2026-08-17 -- the gap above is now closed.** A genuine AMX Art tape (`AMXART_1.TAP`, 32 blocks, multiple loadable tools) was supplied this session. Extracted every CODE block to its correct load address (from the header's param1) and disassembled with the `z80dis` package, then confirmed the ports found by disassembling `ART 1.1`'s own copy independently -- both agree exactly, so this is the real, consistent protocol, not a one-off routine.
- **The mechanism is materially simpler than assumed, and different in kind:** it is not software-side quadrature phase-decoding at all. AMX's Z80 PIO does the edge-detection and debounce in hardware and delivers **one interrupt per step of movement**, direction pre-decoded into a single bit. The interrupt handler's entire job is: read the port once, look at bit 0, increment or decrement a position counter by exactly 1.
  ```
  ; X-axis interrupt handler (installed via IM2 vector table; identical
  ; pattern at 0x82E7 in ART 1.1 and 0xE843 in the AMX driver block)
  PUSH AF
  IN A,(0x1F)        ; ONE read per interrupt, not a sampling loop
  ...
  AND 1              ; bit 0 = direction
  JR NZ,+
  INC (HL)           ; bit0=0: step one way
  JR ++
  DEC (HL)           ; bit0=1: step the other way
  ...
  EI
  RETI
  ```
  Y-axis: byte-for-byte the same pattern, different vector, port `0x3F`.
- **Setup, before the handlers are live**: `IM 2`; `LD A,0xE9 / LD I,A`; then PIO control-word writes -- `0x4F, 0xD0, 0x87` to port `0x5F` and `0x4F, 0xD2, 0x87` to port `0x7F` (a *separate* control-port pair from the `0x1F`/`0x3F` data ports). These program interrupt vectors `0xD0` (Y, table entry `0xE9D0`) and `0xD2` (X, table entry `0xE9D2`); the code then writes `JP <handler>` addresses into those table slots directly.
- **Buttons, confirmed precisely**: polled, not interrupt-driven, from port `0xDF` -- **bits 5, 6, 7** (not bits 0-2, which is Kempston Mouse's layout on a different port; genuinely different device). Each poll retries up to 10 times waiting for its bit (`LD B,10` / `IN A,(0xDF)` / `BIT n,A` / `JR NZ` / `DJNZ`), a hardware debounce pattern, returning a distinct code (`0x7F`/`0xBF`/`0xDF`) per button -- consistent with Wikipedia's "3 buttons."
- **What this means for implementation**: the hard part was never the three data ports -- it's that AMX needs a *second, independent interrupt source* from the standard 50/60Hz frame interrupt, IM2-vectored, firing once per host-mouse step in each axis at whatever vector table address the guest program installs (read from `I`/the written vector bytes, not hardcoded). zenzx's Z80 core already supports IM2 (needed for other software regardless of AMX); what's new is the *triggering* -- generating that interrupt from host mouse movement, correctly interleaved with the standard frame interrupt, not a new decode algorithm. This is real, scoped work (new interrupt-source plumbing), not a mystery anymore.
- **Fix:** implement following mouse.go's existing MouseMode/MouseState separation, adding: (1) PIO control-port writes at `0x5F`/`0x7F` (mode/vector setup, mostly bookkeeping), (2) an IM2-vectored interrupt fired once per accumulated step in each axis (reusing the host-delta-accumulation logic already in mouse.go's Kempston path, but emitting a CPU interrupt per whole step instead of updating a counter register), (3) ports `0x1F`/`0x3F` returning a single direction bit per read, (4) port `0xDF` bits 5-7 for buttons with the same active-high-when-pressed convention observed above (poll-based, no interrupt). Port `0x1F` conflicts with Kempston Joystick -- needs explicit mutual exclusion if both are ever selectable in the same session, same as noted before.

Cross-ref: CHANGELOG 0.4.17.

## [0.4.9] T-11 — Timex hi-colour mode (mode-timex-001-hicolour) accepted but not implemented (v0.4.9, 2026-08-17)

Theme: graphics · closed 0.4.9 · 2026-08-17


- **Trigger:** 2026-08-17, per session request. Documented in `docs/timex-modes.md`. `-ns-graphics mode-timex-001-hicolour` validates and stores the request but no renderer reads it -- display stays standard.
- **What it needs, per docs/timex-modes.md:** screen 0's bitmap area as pixel data (standard non-linear addressing), screen 1's bitmap area (same non-linear addressing, *not* the standard 32x24 attribute layout) as one FLASH/BRIGHT/PAPER/INK attribute byte per 8x1 pixel strip, neither screen's own attribute block used.
- **Open design question before any rendering code, corrected 2026-08-17 against the original T/S 2068 manual:** the manual's own memory-map diagrams (Appendix C) show screen 1 is *ordinary directly-addressable RAM* at `6000H-77FFH` -- earlier text here wrongly inferred it needed DOCK/EX-ROM-style bank switching (that bit pages expansion cartridges, unrelated to the built-in second screen). What the diagrams actually show: enabling the second screen relocates where the machine's "free RAM" (system variables, BASIC program/variables) starts -- from ~5B00H (right after screen 0) to ~7B00H (right after screen 1) -- the ROM does this housekeeping on mode switch, not just an unenforced convention. zenzx's memory model (`memory.go`, Sinclair +3-style 4x16K ROM / 8x16K RAM paging, bank-granular, no notion of a relocatable "free RAM start") has no equivalent mechanism. The concrete question is now: how does a fixed 6912-byte carve-out in the middle of a 16K bank window get modelled given zenzx's paging is bank-granular and the real machine's isn't -- decide this before writing a renderer, not while writing one.
- **Verification gap closed 2026-08-17:** the hi-colour mechanism in docs/timex-modes.md is now corroborated by four independent sources -- Wikipedia, the z88dk issue, the ZX-Uno project's manual, and (uploaded to this session) the original 1983 Timex Computer Corporation / Sinclair Research **T/S 2068 User Manual** itself. The manual states outright, in Appendix C: "the organization of attributes (which reside in memory starting at 6000H) is the same as the organization of pixel data" -- the manufacturer's own documentation confirming the exact mechanism this register describes. Port `0xFF`'s full 8-entry palette table also matches the manual's Appendix C table exactly, bit for bit. This is as settled as a written spec gets short of a real machine's ROM dump. The memory-layout question (above) is the real remaining work, and per the correction above, it turned out different from what was assumed, not just unresolved.
- **Update 2026-08-17:** the video pipeline is now pluggable (`videorender.go`; see `docs/video-architecture.md`). This renderer's `Decode(mem, screen)` will need to read the screen-1 buffer through `mem` once the memory-layout question above is settled (screen's own bitmap/attributes fields are standard-screen-only). `Dimensions()` = (256, 192) -- hi-colour does not change resolution, only attribute granularity. This mode must not implement FLASH even though its attribute byte format has a FLASH bit (see docs/timex-modes.md) -- no non-standard mode supports FLASH, by design.
- Needs a headless zenscript regression (boot, engage the mode, screenshot, assert) before being considered done.

Cross-ref: CHANGELOG 0.4.9.

## [0.4.3] T-01 — README carries stale claims (.dsk loaders broken, beeper unfiltered) (v0.4.3, 2026-08-17)

Theme: docs · closed 0.4.3 · 2026-08-17


- **Trigger:** session review 2026-08-17. README lines 43 and 86 still say `.dsk` images do not load and point at a Known-issues bullet that no longer exists; floppy read/write/format shipped in 0.3.x. The Known-issues audio bullet still describes the beeper as unfiltered/unipolar; the DC blocker and 14 kHz lowpass shipped in 0.3.3.
- **Fix:** rewrite the affected README passages against current behaviour; move intentional limits to `docs/KNOWN_ISSUES.md` and defects to this register so the README stops doubling as the register.

Cross-ref: CHANGELOG 0.4.3.

## Namespace

The T-nn namespace opened at 0.4.2 (2026-08-17) with T-01. IDs are never
reused or renumbered.
