# TS2068 model support — staged development plan

Updated: 2026-08-17

Design intent for `docs/TRACKING.md` T-12. Per tracking-document practice,
this plan is frozen once Stage 1 begins executing; deviations from it are
recorded in `docs/TS2068_TRACKING.md`, not by silently editing this file.

Sources: the T/S 2068 Technical Manual and User Manual (both uploaded to
session; `docs/timex-modes.md` already cites both extensively), and direct
inspection of `rom/ts2068-0.rom` / `rom/ts2068-1.rom`.

## Guiding principle: the substrate does the work, not us

Almost everything that looks like "implement TS2068 system-software
behaviour X" is not custom Go logic to write. We have the **real, verified
ROM images**. A real Z80 CPU emulator running real ROM bytes against a
*correctly emulated hardware substrate* (memory banking, I/O ports, timing,
interrupts) reproduces the ROM's behaviour automatically — including
things that sound like standalone features: the system-initialisation copy
of OS-resident code from Extension ROM to RAM, the Function Dispatcher, and
`CHNG_VID` itself. None of these need a parallel Go implementation of their
algorithm. They need the substrate under them to be right, and a test that
confirms the real ROM code produced the right result when run.

This is why the plan's stages are almost entirely about memory banking,
port dispatch, and timing — the small number of places zenzx's current
model doesn't already provide what real TS2068 hardware provides — rather
than about BASIC or OS features individually.

## Explicitly out of scope (not stages — will not be built)

Per direction: of everything flagged as deferrable, only `CHNG_VID` is in
scope. These remain unimplemented indefinitely, not "future stages":

- Full 8-chunk Dock/cartridge (LROS/AROS) banking — real feature, needed
  only for cartridge software, not for BASIC or the ROM's own operation.
- TS2040 dot-matrix printer protocol (port `FB`).
- Composite/RF analogue video generation detail (not applicable to a
  digital emulator's output regardless).

## Stage 1 — Model skeleton, ROM loading, chunk-0 banking, boot

**Goal:** `-model ts2068` boots to the BASIC command line ("K" cursor /
copyright screen) and executes ordinary Home-ROM BASIC statements
(`PRINT`, `LET`, `FOR`/`NEXT`, etc.) correctly.

- Register `ts2068` in the model switch (`zenzx_headless.go`,
  `zenzx_gui.go`), loading `rom/ts2068-0.rom` (Home, 16K) as the ROM at
  `0000H-3FFFH` and `rom/ts2068-1.rom` (Extension, 8K) into a new
  Extension ROM store.
- **Chunk-0 Home/Extension banking** (`memory.go`): new state tracking
  port `F4H` (HSR) bit 0 and port `FFH` bit 7 (EXROM/Dock select). When
  HSR bit 0 is set and bit 7 is set, reads/writes to `0000H-1FFFH` go to
  the Extension ROM; otherwise to the first 8K of the Home ROM. `2000H-
  3FFFH` (chunk 1) always comes from the Home ROM's second half — the
  Extension ROM is 8K and only ever maps onto chunk 0. HSR bits 1-7
  (Dock/cartridge chunks) are tracked but have no effect (no Dock content
  exists) — logged, not silently dropped, so a program trying to use them
  gets an honest "nothing there" rather than an unexplained hang.
  Necessary for the very first boot: system initialisation copies OS-
  resident routines from the Extension ROM to RAM chunk 3 as one of its
  first acts (Technical Manual §3.3.4), which requires this banking to
  already work.
- Reuse port `FEH` (keyboard/tape/border/beep) as-is — bit layout is
  identical to standard Spectrum per the Technical Manual's own table.
  Verify the TS2068's 42-key matrix (User Manual Ch. 2, keyboard code
  table in the Technical Manual's assembler include file) maps correctly;
  adjust `keyboard.go`'s scancode table if it doesn't.
- Port `FFH` bits 0-2/3-5/6 are already implemented for `-ns-graphics`
  (`videorender_hicolour.go`, `nonstandard.go`) — reusable, not new work,
  though real TS2068 boot always starts in mode `000` regardless of
  `-ns-graphics`.

**Done when:** headless boot to the copyright screen and `0 OK` prompt,
verified via `screen_read.go`'s OCR-style text recognition (already used
elsewhere in the test suite); a short BASIC program (`PRINT`, a `FOR` loop)
runs and produces correct screen output.

## Stage 2 — Frame timing (NTSC clock and frame rate)

**Goal:** correct cycle-per-frame and interrupt cadence for this model,
not the existing PAL-ish approximation.

- `CPUFrequency`, `FramesPerSecond`, `CyclesPerFrame` are currently
  compile-time `const`s (`zenzx.go`: 3.5MHz / 50Hz / 70,000 cycles). Real
  TS2068: 3.528MHz (14.112MHz/4), 262 lines/frame at 60.1145Hz (Technical
  Manual §2.1.8.2/§2.1.8.3) — roughly 58,696 cycles/frame. These need to
  become per-instance fields (a small `TimingProfile`-shaped addition to
  `ZenZX`/`SpectrumIO`), threaded through `scheduler.go` and anywhere else
  `CyclesPerFrame` is currently referenced directly, rather than left as a
  global constant with one hardcoded value.
- Do this now, not later: everything from Stage 3 onward (interrupt-driven
  keyboard scanning, `CHNG_VID`'s own timing-sensitive corrections around
  `VIDMOD`, tape pulse timing) depends on correct cycle accounting. Fixing
  it after those stages exist would mean revisiting all of them.

**Done when:** the 17ms (actually 15.635ms, Technical Manual §2.1.8.4)
interrupt cadence is measurably correct in a headless run; existing
border-stripe and audio timing code (which reads `CyclesPerFrame`)
continues to produce correct output for the *other* models (regression,
not just new-model correctness).

## Stage 3 — `CHNG_VID` (the one deferred item in scope)

**Goal:** a running BASIC/machine-code program can call the real
Extension ROM's Video Mode Change Service (via the Extension ROM
Interface Routine pattern, Technical Manual Figure 3.2.2-2) and get a
correctly relocated second display file — genuine hardware-level dual-
screen/hi-colour/64-column switching, not the static `-ns-graphics`
approximation.

- Per the guiding principle: this is **not** a Go reimplementation of
  `OPDFIL`/`CLDFIL`/`CHG_V`. Those are real ROM bytes at known addresses
  (`CHNG_VID EQU 0DB0H` within the Extension ROM, per the Extension ROM
  Map). If Stage 1's chunk-0 banking is correct and the CPU/memory
  emulation is (already) correct, calling it should already work.
  `OPDFIL`'s own body only touches: generic Z80 block-move instructions
  (`LDIR`/`LDDR`, already correct), system variables (`RAMTOP`, `STKEND`,
  `UDG` — ordinary RAM reads/writes, no special casing needed), and port
  `FFH` (already implemented). It does not itself touch chunk banking —
  only *reaching* it via `IFRTN` does, which Stage 1 covers.
- The actual work here is verification, not implementation: confirm
  nothing else is missing, then write a regression that boots `ts2068`,
  drives a BASIC/machine-code sequence that calls the real `CHNG_VID`
  (via `IFRTN`, per Figure 3.2.2-2's pattern) to open the second display
  file, and asserts the relocation happened correctly — RAM moved from
  `6000H` to `F7C0H`, `VIDMOD` updated, port `FFH` updated with bit 7
  preserved. Then the reverse (`CLDFIL`) closing it again.
- Once this works, `mode-timex-001-hicolour`'s existing renderer
  (`videorender_hicolour.go`) should render whatever the real ROM put in
  the (correctly located) second display file with zero changes to the
  renderer itself — it already reads screen 1 through `mem.Read()`, which
  doesn't care whether the bytes got there via a test poke or via real
  ROM code executing `CHNG_VID`.

**Done when:** the regression above passes, and a manual/headless
screenshot of a `CHNG_VID`-driven mode switch visually matches what
`-ns-graphics mode-timex-001-hicolour` already produces for equivalent
screen 1 content.

## Stage 4 — AY sound chip ports and joystick

**Goal:** sound and joystick input work via TS2068's actual port
assignments, not the 128K-style ones.

- `io.go` currently matches AY register-select/data at `port&0xC002==
  0xC000`/`0x8000` (128K-style `0xFFFD`/`0xBFFD`). Add TS2068-specific
  dispatch at `F5H` (address, write-only) / `F6H` (data, R/W), gated on
  model. The AY emulation itself (`ay8912.go`) is unchanged — this is
  purely a new dispatch path pointed at existing code, per the guiding
  principle.
- Joystick: read via the AY chip's I/O port register (R14), not a
  dedicated Kempston-style port. Sequence per Technical Manual §2.1.6.1/
  §2.1.7: write `0EH` to `F5H` (select R14), read `F6H` with address bit
  8 (player 1) or bit 9 (player 2) selecting which stick; bits 0-3 are
  up/down/left/right, bit 7 is the fire button (Table 2.4.4-1), all
  active low. Needs new logic tied to the AY register-14 read path, not
  a standalone port handler.
- Note the User Manual/community-documented errata already on record:
  the "+5V and ground available for external logic" note in some TS2068
  documentation is wrong for this board revision — not relevant to
  emulation, but worth not propagating into any doc this project writes.

**Done when:** `BEEP`/`SOUND`/`PLAY`-equivalent BASIC produces audio
through the AY chip at the correct ports; a headless zenscript driving
simulated joystick input produces the expected `STICK`/`IN` results.

## Stage 5 — Tape I/O

**Goal:** `LOAD`/`SAVE` work in accurate (pulse-level) mode; fast-mode
ROM-trap short-circuiting works if reasonably achievable.

- **Accurate mode should already work once Stage 1 is solid** (per the
  guiding principle) — the EAR bit is exposed via port `FE` (reused
  as-is), and the real ROM's own tape routines (in the Extension ROM,
  reachable via Stage 1's chunk-0 banking) decode pulses through their
  own code. Verify with a regression rather than assuming.
- **Fast mode's trap addresses are known, not guesswork**: the Technical
  Manual's own Extension ROM Interface Routine listing gives them
  directly — `W_TAPE EQU 0068H`, `R_TAPE EQU 00FCH` (cross-confirmed
  against the Extension ROM Map's `TAPE` module and the EXROM Services
  address table). Wiring the existing `FastLoader` pattern
  (`tape.go`) for this model means trapping on these PCs **and** the
  chunk-0-currently-Extension-ROM condition together — matching by PC
  alone isn't sufficient the way it is for the standard models, since
  `0x0068`/`0x00FC` are ordinary low addresses that only mean "tape
  routine" when chunk 0 is actually banked to Extension ROM.

**Done when:** a `.tap`/`.tzx` loads correctly in accurate mode
(regression, screenshot or `SCREEN$`-style assertion); fast mode is
attempted, but accurate mode alone is an acceptable stage completion if
fast-mode's chunk-aware trap proves fiddly — record the outcome either
way in the tracker, don't silently drop it.

## Stage 6 — Memory contention

**Goal:** TS2068's own display-RAM contention pattern (Technical Manual
§2.1.8.2: CPU clock stops when it addresses display RAM during a video
fetch), not the standard Spectrum ULA's contention table.

- Lowest priority of the in-scope stages — affects cycle-exact timing
  fidelity, not functional correctness of anything built in Stages 1-5.
- Needs establishing first whether zenzx currently models Spectrum ULA
  contention timing *at all* for the existing models. If not, this stage
  is new infrastructure, not adaptation of an existing table, and should
  be scoped/estimated accordingly once that's known.

**Done when:** TBD pending the investigation above — revisit stage
scoping once it's answered, rather than estimating blind now.

## Dependency order

Stage 1 gates everything (boot + chunk-0 banking). Stage 2 (timing)
should follow immediately, before anything timing-sensitive is built on
top of the wrong constants. Stage 3 (`CHNG_VID`) depends only on Stages
1-2 and is otherwise independent of Stages 4-5 — it's placed early
because it directly extends this session's `mode-timex-001-hicolour`
work and is the one deferred-item exception, not because anything else
requires it first. Stages 4 and 5 are independent of each other and of
Stage 3. Stage 6 depends on nothing above except wanting a stable
substrate to measure against, and its own scope is genuinely unknown
until investigated.
