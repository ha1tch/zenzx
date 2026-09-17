# ZX Spectrum Next support — staged development plan

Updated: 2026-09-14

Design intent for `docs/TRACKING.md` T-27..T-32 (the six filed Tier 0
prerequisites) plus the unfiled Z80N placeholder. Tracked as waves
`ti0.r0`-`ti0.r6` in `docs/WAVE_TRACKING.md` / `docs/WAVE_PLAN.md` — no
separate `NEXT_TRACKING.md` stage tracker exists, deliberately: the wave
mechanism already provides per-item status, a progress bar, and a plan
paragraph, so a second tracking-flavoured document would only duplicate
it. Per tracking-document practice, this plan is frozen once the first
wave (`ti0.r1`) begins executing; deviations are recorded against the
relevant wave's own row in `docs/WAVE_TRACKING.md`, not by silently
editing this file.

Sources: the SpecNext wiki (`wiki.specnext.dev/Specifications`,
`/tbblue-io-port-system`, `/Extended_Z80_instruction_set`), Wikipedia's
ZX Spectrum Next hardware table, `jorgegv/jnext`
(github.com/jorgegv/jnext) as a full-fidelity reference implementation,
and direct inspection of zenzx's own source (`memory.go`, `io.go`,
`pkg/zxpalette`) and of `zen80@v0.5.5`'s `z80/z80n.go` and
`Z80N_ZESARUX_CROSSCHECK.md`.

## Tier 0 requirements

Seven requirements. Without all seven, this is a 128K/+3 clone with
some Z80N opcodes, not Next emulation:

1. `ti0.r0` -- Z80N CPU support (opcodes already implemented in `zen80`;
   this wave covers wiring it as a real machine mode)
2. `ti0.r1` -- NextREG port handler
3. `ti0.r2` -- Extended MMU
4. `ti0.r3` -- Layer 2 graphics
5. `ti0.r4` -- Hardware sprites
6. `ti0.r5` -- Next palette
7. `ti0.r6` -- NEX file loading

See the wave sections below for each.

## Tier 1 -- real compatibility

The features a large share of real Next software assumes beyond the
Tier 0 floor.

**CPU turbo speeds.** The Z80N core runs at 3.5, 7, 14, or 28 MHz,
selected via NextREG 0x07. Each speed carries its own memory/IO
wait-state and contention behaviour -- the FPGA's memory arbitration
timing changes proportionally to the selected speed rather than staying
fixed while the CPU simply executes faster. Real Next software switches
speed routinely: loading screens, block copies, and BASIC interpretation
all commonly run at an elevated speed and drop back to 3.5 MHz for
cycle-accurate timing-sensitive sections.

**TurboSound (2nd and 3rd AY-3-8910/YM2149).** The Next multiplies the
classic single AY sound chip into three independently addressable
chips, distinguished by a chip-select mechanism layered over the
classic AY port pair, each independently configurable as mono or
stereo (ABC or ACB panning). Real Next chiptune music commonly composes
across two or three chips at once for polyphony and stereo separation a
single three-channel AY cannot produce on its own.

**Copper co-processor.** An instruction sequencer that runs
independently of and in lockstep with the video timing generator,
executing a small WAIT/MOVE instruction set that writes to NextREG at
precise scanline and pixel positions. This is the mechanism behind most
Next demo-scene and game raster effects -- per-scanline palette
changes, parallax bands, and other timing-critical visual tricks -- the
CPU loads a copper program once and the hardware free-runs it against
the video beam thereafter, with no further CPU involvement.

## Tier 2 -- jnext-parity

The full scope jnext itself implements, verified against the official
VHDL sources.

**Real NextZXOS/DivMMC/SD-card boot chain.** The authentic sequence
real hardware follows on power-up: FPGA boot ROM, then TBBLUE.FW, then
NextZXOS's own welcome screen and menu, with every machine and
peripheral ROM read from files on an SD-card image rather than baked
into the emulator. DivMMC is the bridge between that SD card and the
Z80 bus -- its own automap logic, 8KB of SRAM, and the esxDOS
filesystem layer sit between the two.

**DMA (ZXnDMA).** A subset-compatible implementation of the Z80 DMA
chip: memory-to-memory and memory-to-IO block transfers using short
two-cycle read/write bus cycles, plus a burst mode that streams sample
data to a DAC port at a programmable rate while still yielding the bus
back to the CPU between transfers. Used for sample playback and block
copies that would otherwise cost many CPU cycles per byte.

**Tilemap.** A second hardware character-display layer, distinct from
both Layer 2 and the ULA, in 40x32 or 80x32 modes. Glyphs and the
character map itself live at programmable locations in bank 5, with
hardware pixel scroll in X and Y and per-glyph rotation/mirroring. A
second tilemap mode trades that rotation/mirroring capability for extra
per-cell colour information, defining glyphs as monochrome UDGs instead.

**RZX record/playback.** A format that captures deterministic input
replay -- the sequence of values a running program read from IN
instructions -- alongside an embedded snapshot, so a recorded session
can be replayed bit-for-bit later: the same inputs land at the same
points in execution and produce an identical run.

**Emulated ESP-01 Wi-Fi.** A virtual Wi-Fi module attached to one of
the Next's UART connections, speaking the same AT-command set
(AT+CIPSTART, AT+CIPSEND, and the rest) real Next software already
targets, with that traffic bridged to genuine outbound TCP/UDP on the
host.

**USB gamepad support.** Host USB controllers mapped onto the Next's
two physical joystick-connector protocols (Kempston, Sinclair, Cursor,
MegaDrive), as an alternative input source alongside keyboard-driven
joystick emulation.

**A full debugger.** CPU, memory, MMU, NextREG, sprite, and copper
inspection panels; breakpoints and watchpoints; disassembly with symbol
tables; and comparable development-time tooling built on top of, rather
than as part of, the emulation core itself.


## Guiding principle: NextREG is the backbone, build it once

Almost every Next-only feature — Layer 2, sprites, the palette, the MMU
slots — is configured by reading and writing NextREG. It is not seven
independent hardware quirks to bolt on separately; it is one register
bus with seven consumers. `zen80`'s `NEXTREG` instruction (`ED 91`/`ED
92`) already calls the CPU's own `ioOut` rather than writing to some
internal register array directly — confirmed by reading `z80n.go`
itself, not assumed — which means the instruction-decode half of
NextREG access is already correct and future-proof. The one piece
missing is the handler on the other end of that call. Get `ti0.r1`
right once, and every wave after it is "write through the register
file," not "invent another one-off port branch."

This is also why the waves are sequenced the way they are: `ti0.r1`
gates everything, and nothing downstream should need to touch I/O
dispatch again once it lands.

## Deferred within Tier 0 (sub-features)

These are refinements within already-staged Tier 0 items, deferred to
keep each wave's initial scope achievable:

- Sprite scaling/rotation/mirroring/composite sprites, within `ti0.r4`.
- Layer 2's 320×256/640×256 modes and 4-bit sprite colour, within
  `ti0.r3`/`ti0.r4`.
- NEX v1.3, within `ti0.r6` — matching jnext's own experimental
  opt-in treatment of that format.

Each is noted as deferred within its own wave section below, not
silently dropped.

## Wave ti0.r0 — Z80N CPU support (adjustments)

**Goal:** confirm the Z80N opcode implementation (`zen80` v0.5.5, all 26
opcodes, cross-checked against ZEsarUX) behaves correctly once exercised
as part of a real Next machine type, not only as a standalone `-z80n`
CPU-only flag.

- Add a `next` machine model (model switch in `zenzx_headless.go` /
  `zenzx_gui.go`) that sets `cpu.Z80N = true` as part of machine
  selection, replacing the manual-flag-only path.
- Exercise the opcode table against a real Next test binary once
  `ti0.r1` provides enough of NextREG to run something — the opcodes
  were verified against ZEsarUX in isolation, not yet against a running
  machine with real I/O underneath them.
- No concrete defect is filed yet (wave item 17 is deliberately "not yet
  filed") — this wave exists to hold whatever surfaces once the above
  two points are actually tried, not to describe known work.

**Done when:** `next` is a selectable machine type, Z80N opcodes decode
correctly within it, and at least one real Z80N test binary runs
correctly under it.

## Wave ti0.r1 — NextREG port handler

**Goal:** ports `0x243B` (select) / `0x253B` (data) have a real register
file behind them, reachable identically via a raw `OUT` and via the
Z80N `NEXTREG` instruction.

- Add a NextREG register file to `io.go` — a new field on `SpectrumIO`
  or a dedicated type — with a full 16-bit port match at `0x243B`/
  `0x253B`. Next ports are fully decoded, unlike the ULA's partial
  decode, so this must be a new branch, not a reuse of the existing
  low-byte-match branches those rely on.
- Confirm `zen80`'s `z80nNextregNN`/`z80nNextregA` (already routed
  through `ioOut`) reach this new handler with no CPU-side change
  required — this is a prediction from reading the CPU source, to be
  verified by a regression, not assumed correct by construction.
- Per-register read-back semantics matching the SpecNext wiki's I/O
  port documentation register by register — most are R/W, a few are
  read-only, write-only, or have side effects on write.

**Done when:** both a raw port `OUT` and the `NEXTREG` instruction
produce identical, correct register-file state, verified by a
regression that exercises both paths against the same register.

## Wave ti0.r2 — Extended MMU

**Goal:** replace `memory.go`'s fixed three-slot, 16K-granularity model
with the Next's eight independent 8K MMU slots, reaching at least the
768K-1MB unexpanded-machine tier.

- New paging model: 8 slots × 8K each, covering the full 64K address
  space, each independently mappable to any 8K page of RAM/ROM. This
  replaces `ramBankLow`/`ramBankHigh`/`ramBankTop` — it does not extend
  them; the existing fields assume 16K granularity throughout `Read`/
  `Write`.
- Wire NextREG MMU slot registers (`0x50`-`0x57`) through `ti0.r1`'s
  register file.
- Keep the existing 128K/+3 paging path (`0x7ffd`/`0x1ffd`) working
  unchanged for non-Next machine types — the MMU is additive at the
  machine-type level, not a replacement for classic paging.
- Extension port `0xdffd` for banks beyond 128K's original eight.

**Done when:** a `next`-mode test program can page all 8 MMU slots
independently and read back correct data from banks beyond bank 7, with
a regression confirming 48K/128K/+3 machine types are unaffected.

## Wave ti0.r3 — Layer 2 graphics

**Goal:** 256×192, 8-bit-colour Layer 2 renders correctly, composited
against the ULA layer.

- New Layer 2 framebuffer type, base bank configurable via NextREG.
- Clip window support (NR `0x18`).
- Composite with the existing ULA layer per NR `0x15`'s priority bits —
  start with the two most common orderings (sprites-over-Layer2-over-ULA,
  the reset default, and Layer2-over-ULA) rather than all six from day
  one.
- 320×256 and 640×256 4-bit modes deferred (see Deferred within Tier 0, above).

**Done when:** a `next`-mode test program can select 256×192 Layer 2,
write a known pattern, and the compositor produces the correct on-screen
result, verified by pixel comparison.

## Wave ti0.r4 — Hardware sprites

**Goal:** 128 sprites, 16×16, unscaled 8-bit colour, composited per NR
`0x15`.

- Sprite attribute table (position, pattern index, palette offset,
  visibility), reachable via `ti0.r1`'s NextREG mechanism.
- Sprite pattern memory (16×16×8-bit per sprite).
- Composite into the same layer-priority path `ti0.r3` builds, rather
  than a second, parallel compositor.
- Scaling, rotation, mirroring, and composite (anchor+relative) sprites
  deferred (see Deferred within Tier 0, above).

**Done when:** a `next`-mode test program can position and display at
least one sprite correctly, verified by pixel comparison, with the
existing ULA/Layer2 layers unaffected when zero sprites are active.

## Wave ti0.r5 — Next palette

**Goal:** 9-bit RGB, 512-colour palette with NextREG palette control,
correct for both Layer 2 and sprites.

- New `NextPalette` type in `pkg/zxpalette`, alongside — not replacing —
  the existing classic 16-colour palette. `zxclassicpalette.go`'s own
  comment already anticipates this split, so this is additive to the
  existing structure.
- 8 palettes (ULA/Layer2/Sprite/Tilemap × first/second), 256 entries
  each, per NextREG palette-index/palette-value ports.
- `ti0.r3`/`ti0.r4`'s renderers read through this once it lands, rather
  than continuing to assume classic colours.

**Done when:** a `next`-mode test program can write a known palette
entry and the rendered pixel for that index matches the expected 9-bit
RGB value, for both Layer 2 and sprites.

## Wave ti0.r6 — NEX file loading

**Goal:** NEX v1.0-1.2 loads and runs a real Next program directly,
without a DivMMC/SD/NextZXOS boot chain underneath it.

- Header parsing for v1.0/1.1/1.2 — v1.3 explicitly out of scope,
  matching jnext's own experimental opt-in treatment of that format.
- Direct page loading into `ti0.r2`'s MMU.
- Layer 2 screen/palette-from-header, using `ti0.r3`/`ti0.r5`.
- Follows zenzx's existing snapshot-loading pattern (`.sna`/`.z80`)
  rather than inventing a new loader architecture for this one format.

**Done when:** a real, publicly available NEX v1.0-1.2 file (e.g. from
the SpecBong tutorial series) loads and runs correctly, verified by
screenshot.

## Dependency order

`ti0.r1` gates everything else — it is the register bus every other
wave reads and writes through, and the CPU-instruction half of that
path is already confirmed correct. `ti0.r2` should follow immediately:
Layer 2's framebuffer and NEX's own page loading both need addressable
RAM beyond the 128K ceiling. `ti0.r3` and `ti0.r4` are independent of
each other but both depend on `ti0.r1`+`ti0.r2`, and both land in the
same compositor work, so doing them close together avoids revisiting
the compositor twice. `ti0.r5` depends on nothing structurally but is
only useful once `ti0.r3`/`ti0.r4` exist to display through it —
sequenced after them for that reason, not a technical dependency.
`ti0.r6` is last by construction: it exercises `ti0.r2`, `ti0.r3`, and
`ti0.r5` together, so it can only be verified once all three work.
`ti0.r0` sits outside this chain entirely — a standing placeholder, not
a blocking dependency — revisited whenever a gap surfaces or once
`ti0.r1` gives it something real to test the CPU against.
