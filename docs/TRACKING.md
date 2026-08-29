# ZenZX — live register

Version: 0.6.11
Last reviewed: 2026-08-29

Open, actionable items only. Closed items move verbatim to
`RESOLVED.md` (see `repoman/register.py close`); a closed item still
listed here is itself a defect. Intentional limits and the dormant-guard
table live in `KNOWN_ISSUES.md`.

Status legend: ✓ done · ◐ partial · ☐ not started · ✗ dropped.

## Status table

| ID | Summary | Theme | Priority | Status | Blocks |
|---|---|---|---|---|---|

| T-03 | CPU clock modelled as 3.5 MHz; real 48K is 3.5469 MHz (~1.3 % pitch/timing error) | timing | P3 | ☐ | — |
| T-04 | AY-3-8912 residual aliasing beyond the 0.3.4 area-sampling baseline | audio | P3 | ☐ | — |
| T-06 | 15 Go files are not gofmt-clean | hygiene | P3 | ☐ | — |
| T-07 | Dockerfile and build_bsd.sh are broken (unresolved template variables) | ci | P3 | ☐ | — |
| T-08 | GUI cross-platform CI depends on raylib-go/oto staying cgo-free on darwin+windows | ci | P3 | ☐ | — |
| T-09 | Non-standard graphics modes accepted but not implemented (mode-zenzx-01, mode-zenzx-02) | graphics | P2 | ☐ | — |
| T-10 | Non-standard storage backend accepted but not implemented (storage-zenzx-posix) | storage | P2 | ☐ | — |
| T-13 | Joystick emulation: Cursor and headless zenscript driving not implemented | input | P3 | ◐ | — |
| T-15 | Real Extension ROM CHNG_VID service diverges partway through OPDFIL's relocation, root cause undiagnosed | model | P3 | ☐ | — |
| T-17 | AY-3-8912 ports (128K-style 0xFFFD/0xBFFD) respond regardless of -model, including 48K which has no AY chip | model | P3 | ☐ | — |
| T-20 | 128K contention model is address-range based, not bank-aware; frame geometry (228 t/line, start 14361) is an unverified extrapolation | timing | P3 | ☐ | — |
| T-21 | Floating bus: instruction-start position granularity; 128K port-decode subtleties unmodeled; TS2068 FE-port idle levels legacy/unverified | model | P3 | ☐ | — |
| T-22 | Batman-class load failures: root cause still open after five hardware-fidelity fixes; next instrument is deep RAM-PC sequence + memory-write-delta trace | model | P2 | ☐ | — |
| T-23 | Fast mode's ROM-trap only accelerates while PC is literally inside the standard LD-BYTES routine; any custom/protected loader (common in real commercial tapes) gets zero benefit the moment it takes over | tape | P2 | ☐ | — |
| T-24 | No mode-independent, trustworthy signal exists for 'did this tape actually load correctly' -- every mode's own self-reported completion (fast's trap return, turbo's Position-reached-end, accurate's Playing==false) has been observed true while the load was wrong | tape | P1 | ☐ | — |

## timing

### T-03. CPU clock modelled as 3.5 MHz; real 48K is 3.5469 MHz (~1.3 % pitch/timing error)

Theme: timing · Priority: P3 · Status: ☐

- **Trigger:** README Known issues. Audio sample timing derives from the 3.5 MHz constant with per-sample integer truncation, so pitch is ~1.3 % low and can drift; frame timing is marginally affected.
- **Fix:** model the per-machine clock (48K 3.5 MHz vs 128K-family 3.5469 MHz) and derive the audio sample step from it with a fractional accumulator, not truncation.


### T-20. 128K contention model is address-range based, not bank-aware; frame geometry (228 t/line, start 14361) is an unverified extrapolation

Theme: timing · Priority: P3 · Status: ☐

contention.go's 128K path checks 0x4000-0x7FFF only: a program paging a contended bank elsewhere (or an uncontended one into that window) gets wrong delays. The pattern shape and start cycle for 128K were extrapolated from the 48K FAQ table, not confirmed against an authoritative source. 48K is precise (measured bounds ~0.33% mem / ~0% IO with zen80 0.5.0's offset tracking).

## audio

### T-04. AY-3-8912 residual aliasing beyond the 0.3.4 area-sampling baseline

Theme: audio · Priority: P3 · Status: ☐

- **Trigger:** 0.3.4 cycle. Area-sampling (accumulate `mixOutput()` per AY clock, average per output sample) cut HF energy ~40x. A polyBLEP attempt was abandoned because the measurement harness could not be trusted (contradictory metrics; the big-edge acceptance metric was wrong for band-limited signals).
- **Fix:** build a trustworthy inharmonic-energy metric first, then evaluate polyBLEP or a proper decimating lowpass against it. Do not touch the synthesis path until the metric is validated on a known signal.


## hygiene

### T-06. 15 Go files are not gofmt-clean

Theme: hygiene · Priority: P3 · Status: ☐

- **Trigger:** session review 2026-08-17: `gofmt -l .` lists audio_oto.go, audio_sinc_filter.go, fdc.go, fdc_read_test.go, io.go, keyboard_script.go, memory.go, scheduler.go, screen_read.go, script.go, script_test.go, snapshot_formats.go, tape_types.go, version_main.go, zenzx.go. Pre-existing; not introduced this session.
- **Fix:** run `gofmt -w` on the tree in a dedicated commit with no other changes, then add a gofmt check as a release step so it cannot recur.


## ci

### T-07. Dockerfile and build_bsd.sh are broken (unresolved template variables)

Theme: ci · Priority: P3 · Status: ☐

- **Trigger:** session review 2026-08-17 while wiring CI. `Dockerfile` has empty/unresolved variables (`${BINARY_NAME}`, `${VERSION}`, blank `CC=`/`CXX=++`) and never worked as committed. `build_bsd.sh` / `build_example_bsd.sh` are similarly a broken template (empty shell variables, a garbled heredoc, hardcoded absolute paths from another machine). Neither is exercised by CI or the release workflow — CI/release now cover linux/darwin/windows natively; BSD has no purego path (see KNOWN_ISSUES) and was never covered by these scripts working correctly either.
- **Fix:** rewrite both from the working `build_linux.sh` pattern (real variables, no leftover template placeholders), or remove them if BSD support is not worth maintaining.


### T-08. GUI cross-platform CI depends on raylib-go/oto staying cgo-free on darwin+windows

Theme: ci · Priority: P3 · Status: ☐

- **Trigger:** session review 2026-08-17. The darwin/windows GUI build in CI and the release workflow is cgo-free only because raylib-go v0.60.0's purego backend embeds a native library and oto v3.4.0 has no cgo import on those platforms (verified by reading both vendored sources). Neither guarantee is part of either library's stated compatibility contract.
- **Fix:** when bumping raylib-go or oto, re-run `go build` with `CGO_ENABLED=0` for darwin/amd64, darwin/arm64, windows/amd64, windows/arm64 locally before assuming the CI matrix still passes; grep the new vendored source for `import "C"` on those platforms as a quick check.

## graphics

### T-09. Non-standard graphics modes accepted but not implemented (mode-zenzx-01, mode-zenzx-02)

Theme: graphics · Priority: P2 · Status: ☐

- **Trigger:** 2026-08-17, master-switch infrastructure landed in nonstandard.go. `-ns-graphics` validates and stores the requested mode on `zx.nonStandard.Graphics` but no rendering code reads that field yet -- display stays standard regardless of the value. (Timex hi-colour split out to T-11.)
- **Update 2026-08-17:** the video pipeline is now pluggable (`videorender.go`; see `docs/video-architecture.md`). Implementing either mode means writing a `VideoRenderer` (`Name`, `Decode`, `Dimensions`, `BorderMargins`) and calling `RegisterVideoRenderer` in an `init()` -- no changes needed to display.go, display_headless.go, or either main.
- **Fix, per mode:**
  - `mode-zenzx-01`: 256x192, 3 pixels/byte, linear framebuffer, no attribute clash -- a genuinely new pixel format distinct from the standard bitmap+attribute layout, needs its own memory region, write path, and renderer. `Dimensions()` = (256, 192); `BorderMargins()` needs a decision (proportional to standard, or none -- point 2 in video-architecture.md, border is optional per mode).
  - `mode-zenzx-02`: 512x384, double resolution -- needs a renderer producing 4x the standard pixel count and a decision on how source data maps to the higher resolution (upscale vs a native double-res source format). `Dimensions()` = (512, 384); `BorderMargins()` needs the same decision, doubled if proportional. `DisplayManager.maxMultiplierThatFits()` already clamps the window to the monitor, but verify it in practice once this exists -- untested against a real high-res mode so far.
  - Neither mode supports FLASH (no other mode does -- see video-architecture.md); do not read `screen.flashEnabled`/`flashTickTock` from either renderer.
  - Each mode needs a headless zenscript regression (boot, engage the mode, screenshot, assert) before being considered done.

## storage

### T-10. Non-standard storage backend accepted but not implemented (storage-zenzx-posix)

Theme: storage · Priority: P2 · Status: ☐

- **Trigger:** 2026-08-17, master-switch infrastructure landed in nonstandard.go. `-ns-storage storage-zenzx-posix` validates and stores the request on `zx.nonStandard.Storage` but no storage backend reads that field yet -- the +3 FDC/DSK path (fdc.go) is unaffected.
- **Fix:** design and implement a POSIX-filesystem-backed storage device (distinct from FDC/DSK emulation -- presumably direct host-filesystem access from guest code via a custom port or trap protocol). Needs a design note before implementation: what guest-side interface exposes it (new I/O ports? ROM trap? memory-mapped?), and what host paths it is allowed to touch.


## model

### T-15. Real Extension ROM CHNG_VID service diverges partway through OPDFIL's relocation, root cause undiagnosed

Theme: model · Priority: P3 · Status: ☐

- **Trigger:** 2026-08-17, during TS2068 Stage 3 verification. Attempted to call the real Extension ROM CHNG_VID service (via the documented IFRTN trampoline pattern) to engage hi-colour mode. Confirmed working correctly via direct register tracing: chunk-0 Home/Extension banking itself; CHG_V's early logic (VIDMOD check, memory-availability check); the call into REMGSZ via CALL_BANK (a nested Home-Bank round-trip, genuinely completes, 14-iteration bounded loop, correct RET); OPDFIL's relocation fix-up table walk (61 entries, correctly zero-terminated in the real ROM, confirmed progressing to completion via direct HL tracing); OPDFIL's 'clear the second display file' loop (genuinely progresses byte-by-byte from 0x6000, confirmed via direct HL tracing, NOT stuck as initially misread -- an early debugging mistake, caught by checking real register values instead of trusting the repeating-address pattern).
- **What actually goes wrong:** partway through the clear loop (observed around HL=0x6842 of the expected 0x7AFF/6912-byte target, not completing), execution ends up in a genuine repeating cycle through Home ROM addresses that look like character-output/print machinery (0x11F6 -> 0x1264 -> 0x0C0E -> 0x1202 -> 0x11D9 -> back to 0x11F6), with chunk-0 banking fully reverted to Home (chunk0=false, exrom=false) -- meaning execution left CHG_V's Extension ROM context entirely, not stuck inside it.
- **Hypothesis tested and ruled out:** that this was fallout from OS-resident code never being copied to RAM during normal boot (a real, separately-known gap -- T-12/Stage 1's note that chunk-0 banking wasn't observed to engage during ordinary boot). Checked directly: genuine, real-looking Z80 code exists at 0x6200 after boot, not zeros/garbage. Root cause remains unknown.
- **Why not pursued further:** real TS2068 software, per direct correction during this session, predominantly engaged hi-colour mode via a raw port-FFH write, not the documented CHNG_VID service call -- Stage 3 was rescoped around that (dynamic renderer switching on the guest's own port write, see docs/TS2068_TRACKING.md Stage 3), so this gap doesn't block any current functionality. Left open in case genuine TS2068 software is ever found that specifically requires the real service call (e.g. dual-screen mode, which does go through this same CHNG_VID path).
- **Fix:** would need either a CALL_BANK/REMGSZ semantics reference beyond what's been disassembled so far, or continued register-level tracing past the point this investigation stopped (immediately after the clear loop diverges, before reaching the Home ROM print-code cycle).

### T-17. AY-3-8912 ports (128K-style 0xFFFD/0xBFFD) respond regardless of -model, including 48K which has no AY chip

Theme: model · Priority: P3 · Status: ☐

- **Trigger:** 2026-08-17, noticed while implementing per-model default configuration for -joystick (this session's work matching real built-in hardware per -model -- see joystick.go's defaultJoystickModeForModel). The AY port dispatch (io.go, 'AY-3-8912 sound chip register select (128K)' / 'data write (128K)' comments) is unconditional -- port 0xFFFD/0xBFFD access works identically regardless of io.memory.is128K, meaning a 48K-model program reading/writing these ports gets real AY chip responses even though real 48K hardware has no AY-3-8912 at all (beeper only).
- **Pre-existing, not a regression from this session's work** -- this gating gap predates the joystick/mouse/TS2068 work entirely; noted here because it's the same underlying architectural principle (-model should imply which hardware genuinely exists) applied to a different subsystem, and was directly observed while auditing port dispatch for the joystick fix.
- **Scope:** genuine 128K-family AY presence, confirmed real: 128K/+2/+2A/+3 all have AY-3-8912 (128K sound); 48K does not (beeper only); TS2068 has its own, per Technical Manual, already correctly gated to its own F5H/F6H ports (ts2068.go) and unaffected by this item.
- **Fix:** gate the existing 0xFFFD/0xBFFD dispatch in io.go behind io.memory.is128K (already tracked per-instance), matching how ts2068WritePort/ts2068ReadPort are already gated behind io.memory.isTS2068. Low-risk, mechanical change -- add the same guard already used for the TS2068 ports.

### T-21. Floating bus: instruction-start position granularity; 128K port-decode subtleties unmodeled; TS2068 FE-port idle levels legacy/unverified

Theme: model · Priority: P3 · Status: ☐

floatingbus.go uses cpu.Cycles (up to ~11 T-states early for the IN's IO cycle) -- fine for sync polling, not pixel-exact effects; the within-instruction offset zen80's contention hooks receive could be plumbed in if needed. The 128K shares the 48K model, ignoring 128K-specific unattached-port decode rules. Separately, io.go's TS2068 branch deliberately keeps legacy FE-port idle behaviour (bit 6 tape-driven, bit 7 high) -- FUSE uses a fixed 0x5F for Timex; unverified for TS2068 here.

### T-22. Batman-class load failures: root cause still open after five hardware-fidelity fixes; next instrument is deep RAM-PC sequence + memory-write-delta trace

Theme: model · Priority: P2 · Status: ☐

Eliminated by measurement this session: ULA contention magnitude and precision, frame-timebase incoherence, EAR idle feedback, floating bus, TZX pause levels. FUSE-vs-zenzx instruction diff from Speedlock entry (0x5D15): RAM-resident PC sequences and registers IDENTICAL through step 6000; all observed deltas (WORKSP/DE +-1 from typed-space, R at entry, interrupt landing phase) are history artifacts a real machine also varies -- ruled out by the any-kid-typing invariance argument. Fork lies beyond step 6000 of ~10841 frames. Next: full-depth RAM-only PC sequence diff plus memory-write-delta streams (addr/old/new, RAM-filtered, ~100MB budget per side; FUSE side builds on the existing ZENZXWRITE hook pattern).

## input

### T-13. Joystick emulation: Cursor and headless zenscript driving not implemented

Theme: input · Priority: P3 · Status: ◐

- **Trigger:** 2026-08-17, deliberately out of scope for the initial -joystick kempston/sinclair implementation (0.4.13). Per direction, only Kempston and Sinclair Joystick 1 were requested.
- **Sinclair Joystick 2 done, 2026-08-17** (joystick.go, `JoystickSinclair2`): needed sooner than expected -- turned out to be required to correctly represent the +2/+2A/+3's real built-in hardware (two simultaneous Sinclair ports, not one), not merely a nice-to-have completion. `JoystickSinclairBoth` drives both from two independent host inputs at once, matching the real machine; `-model`'s own `auto` default now resolves to it for those models. See T-12/`docs/TS2068_TRACKING.md` and the 0.4.24 changelog entry for the full model-defaulting work this prompted.
- **Second Kempston port done, 2026-08-17** (`JoystickKempston2`/`JoystickKempstonBoth`, port `0x37`): a genuinely different case from Sinclair 2 -- real, but never a `-model` default, since no stock model ever had a second Kempston port built in. Confirmed against the ZX Spectrum Next's own I/O port register documentation (`specnext.com/tbblue-io-port-system`: "Kempston 2 (port 0x37)") and cross-checked against a separate, independent hobbyist interface's own port numbering ("KEMPSTON_MAX 2": "Kempston Port 55" = `0x37` hex). Represents modern neo-Spectrum platforms (ZX-Uno, ZX-Tres, the Omni, the Next) rather than any classic-era hardware -- explicit-choice only, same category as `-ns-graphics`/`-ns-storage`.
- **Cursor/AGF/Protek joystick**: same keyboard-remapping mechanism as Sinclair, different key set (commonly 5,6,7,8,0 or similar depending on source -- verify exact mapping before implementing, don't assume from memory). Still open.
- **Headless zenscript joystick driving**: no .zen verb currently exists to simulate joystick input for automated testing (script.go's knownVerbs has no 'joystick' entry). joystick.go's JoystickState/SetJoystickState split was deliberately designed so this is cheap to add later -- a new verb parsing up/down/left/right/fire arguments and calling the same SetJoystickState a real gamepad poll would, no new translation logic needed. Still open.


## tape

### T-23. Fast mode's ROM-trap only accelerates while PC is literally inside the standard LD-BYTES routine; any custom/protected loader (common in real commercial tapes) gets zero benefit the moment it takes over

Theme: tape · Priority: P2 · Status: ☐

Confirmed with the Chase H.Q. corpus (four working tape variants tested in all three modes): trapLoad's entry condition (tape.go TryIntercept, PC==ldBytesTrapPC) fired and completed near-instantly for two tapes (CHASHQ48.TAP, chase.tap) but never engaged at all for the other two (Side 1.tzx, (48K-128K).tap) -- those sat at the ordinary in-progress ROM/128K loading UI ("Program: CHASE HQ", "Tape Loader -- press BREAK twice") for the full duration, meaning fast mode provided no speed benefit whatsoever and would need the same 5-12 minutes of real tape-time as accurate mode to complete. Root cause structural, not a bug: once a tape's bulk data is delivered via a non-standard loader (flag bytes outside 0x00/0xFF, which all four working Chase H.Q. tapes use for their post-header blocks), PC never revisits ldBytesTrapPC, so TryIntercept has nothing to catch. This is likely the majority case for real 1980s-90s commercial releases (T-22's whole Speedlock investigation is one large example of the same pattern). Fast mode's actual real-world coverage is narrower than its historical verification (T-02, 'byte-identical for .tap/.tzx CODE blocks') suggested -- that verification used a tape whose structure happened to route fully through the standard ROM loader, not a representative sample.


### T-24. No mode-independent, trustworthy signal exists for 'did this tape actually load correctly' -- every mode's own self-reported completion (fast's trap return, turbo's Position-reached-end, accurate's Playing==false) has been observed true while the load was wrong

Theme: tape · Priority: P1 · Status: ☐

Pattern recurring across this project's own history, not a one-off: turbo mode's Position reached the tape's exact end while the screen was completely blank and no game code had ever been read (T-19's original bug, RESOLVED.md) -- the exact signal a race/fallback mechanism would use to declare success. This session's own Chase H.Q. investigation found the corpus-report 'verified' heuristic (screen hash differs from boot + has visible content) gave a false positive for eleven minutes of static title-screen before the true stuck state was found (three of seven Chase H.Q. tape variants). And CHASHQ48.TAP's fast-mode result reached a screen that differed from boot and had content, yet showed visibly corrupted icon graphics plus a crack-signature line -- 'verified' by the existing heuristic, not actually correct. Blocks any confidence-based automation (mode racing, fast-with-fallback, unattended corpus scoring) until a real verifier exists: one that checks actual memory/screen content against something independent of the mode's own bookkeeping, not merely 'differs from the pre-load state.' See docs/TAPE_LOADING_HANDOVER.md for the fuller writeup and a candidate design sketch.

---
