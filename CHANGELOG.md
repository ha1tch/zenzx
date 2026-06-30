# Changelog

All notable changes to ZenZX are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.4.2] - 2026-06-30

### Fixed

- **Snapshots saved with a blank screen.** After loading a screen directly (e.g.
  a `.scr` image) and saving a `.sna` or `.z80`, reloading that snapshot restored
  the border but left the screen blank. The display is held authoritatively in
  the render buffers (`screen.bitmap`/`attributes`); a directly loaded screen
  populates those buffers without going through the RAM write path, so the bank
  5 screen region stayed stale and the snapshot captured an empty screen. The
  border survived because it is a single restored value. `toMachineState` now
  copies the display buffers into the displayed bank's screen region before
  encoding (the mirror of `resyncAfterLoad`, which copies the other way on load),
  so a snapshot always captures what is actually on screen. The round-trip tests
  now set and check screen-region markers through the display buffers, matching
  how the screen is really stored, and reproduce the reported scenario.

## [0.4.1] - 2026-06-29

### Added

- **Headless `-model plus2a`**: the +2A is now selectable in the headless build,
  not only the GUI. It loads the +3 ROM set (the +2A shares the +3's ROM,
  including the unused +3DOS) but does not enable the floppy controller, since
  the +2A has no drive. A measured boot baseline (frame 55) was added for the
  `wait-boot` action; the +2A settles far sooner than the +3 (frame 135)
  precisely because it has no FDC seeking for a boot disk.

### Fixed

- **README fast-tape note corrected.** The previous note stated flatly that fast
  tape loading does not work. In fact the instant-inject path (used when a tape
  is loaded with `-tape` in fast mode) places `.tap` and `.tzx` CODE blocks into
  memory byte-identically to the source. The note now distinguishes this working
  path from the separate, still-unverified ROM-trap path that would intercept a
  guest's own `LOAD ""`, and records that a CODE tape is loaded but not run.

## [0.4.0] - 2026-06-29

Migrates the standard snapshot codecs (`.sna`, `.z80`) onto the shared
`zentools/pkg/snapshot` library, replacing ZenZX's in-tree implementations with
thin delegations over a neutral machine-state adapter. The proprietary `.zxs`
format stays native to ZenZX. This both removes duplicated format code and fixes
real defects the in-tree versions carried.

### Added

- **`snapshot_adapter.go`**: `toMachineState` / `fromMachineState` map ZenZX's
  live emulator state to and from `zentools`' neutral `MachineState`. The format
  codecs never see ZenZX's types; this adapter is the only coupling point.
- **Snapshot regression suite** validated against real third-party artifacts:
  round-trip tests for 48K/128K `.sna` and `.z80`, plus sentinel tests that load
  genuine game snapshots (Jet Set Willy v1, Manic Miner v2, Z80 Attack v3 128K)
  and a load -> re-save -> independent-decode check confirming byte-identical
  memory across a v1-to-v3 conversion.

### Changed

- **`.z80` save now writes version 3** (extended header, 128K-capable) rather
  than the previous version 1. Loading still accepts v1, v2, and v3.
- `SaveSNA` / `LoadSNA` / `SaveZ80` / `LoadZ80` are now thin wrappers over
  `zentools`; roughly 460 lines of in-tree codec were removed.

### Fixed

- **`.sna` 48K save lost the program counter.** The previous in-tree `SaveSNA`
  never pushed PC onto the stack for 48K snapshots (the 48K SNA header has no PC
  field, so PC must be saved on the stack), silently losing it. The `zentools`
  codec does this correctly; a regression test pins PC survival across a round
  trip.
- **`.sna` and `.z80` snapshots now load correctly**, resolving the prior
  known-issue. Verified by running loaded game snapshots headlessly to their
  title/menu screens.

## [0.3.5] - 2026-06-28

### Fixed

- **Headless fast-load**: a tape loaded in the headless runner is now played
  automatically, so fast-load injection actually fires. `LoadFile` leaves the
  tape stopped, and the headless main set the tape mode but never called
  `Play()`; the tape `Tick` returns early unless the tape is playing, so in fast
  mode no block was ever injected. The headless main now calls `Play()` after
  loading, matching the GUI path. Verified by loading a CODE tape and reading the
  target memory directly: the block's bytes are placed at the address its header
  specifies.

## [0.3.4] - 2026-06-27

Smooths AY-3-8912 sound. The chip was clocked accurately but its output was
read once per audio sample (point sampling), which aliases the square and noise
harmonics into the audible band before any output filter can act -- the
roughness left after the beeper was cleaned up in 0.3.3.

### Fixed

- **AY anti-aliasing**: the chip's mixed output is now area-sampled --
  accumulated at every AY clock within each sample window and averaged -- rather
  than read once per sample. This is first-order anti-aliasing at the source,
  matching the beeper's duty-cycle approach. In a synthetic test this reduced
  energy above 14 kHz by roughly forty times with no loss of the tone's
  fundamental. The internal chip logic (tone, noise LFSR, envelope, channel
  mixing) is unchanged; only the sampling of the result differs.

## [0.3.3] - 2026-06-27

Smooths beeper audio. The square wave was generated by per-sample duty-cycle
averaging, which leaves residual high-frequency content that folds back as
aliasing -- the jagged, buzzy edge of the sound.

### Fixed

- **Beeper anti-aliasing**: the mixed audio output now passes through a DC
  blocker (removing the offset of the unipolar beeper signal) and a 14 kHz
  Butterworth lowpass that removes the aliased high harmonics while leaving the
  beeper's tone intact. In a synthetic test this reduced energy above 14 kHz by
  roughly two and a half orders of magnitude with no measurable loss of the
  fundamental. The filtering is on by default and can be toggled off for a
  deliberately harsher sound.

## [0.3.2] - 2026-06-27

Makes disk writes persist automatically. Previously, modifications to a disk
(such as a BASIC SAVE) lived only in memory and were written to the .dsk file
only on a manual save (F4) or eject; exiting discarded them.

### Added

- **Save on exit**: a modified disk loaded from a file is flushed back to that
  file when the emulator exits, in both the windowed and headless front-ends.
  A blank in-memory disk with no filename is left untouched (use Save As).
- **Debounced auto-commit**: a modified disk is committed to its file
  automatically once writing has settled -- roughly 30 seconds after the last
  write, and only while the controller is idle (never mid-command). Each new
  write restarts the window, so a multi-block save commits after the final
  write rather than partway through, avoiding a torn image. An idle or
  read-only session writes nothing.

Manual save (F4) and save-on-eject (F6) continue to work for an immediate
commit.

## [0.3.1] - 2026-06-27

Reworks +3 floppy handling to read, write, and format real DSK images
correctly, and fixes a +3 memory-paging crash. Includes a freshly formatted
DSK image for experimentation.

### Added

- **DSK image parsing** (standard and extended CPC/+3 formats): disks are now
  parsed into a track/sector model where each sector carries its own CHRN
  address (cylinder, head, record ID, size code), recorded FDC status, and
  data. This replaces a flat fixed-geometry model that could not represent real
  disks. Verified against a range of commercial images, including ones with
  irregular sector sizes, non-sequential high-numbered sector IDs, and sparse
  formatting.
- **DSK serialization and save-back**: the parsed structure can be written back
  to a valid extended-DSK file, so modifications persist. Verified to
  round-trip losslessly.
- **Sector write and Format Track**: the FDC write path stores data into the
  parsed structure by CHRN, and Format Track (previously unimplemented) builds
  a track's sectors with the requested geometry and filler byte. Both persist
  via save-back.
- A freshly formatted blank DSK image, `zenzx-formatted.dsk`, is bundled for
  experimentation with disk writing.

### Fixed

- **FDC read path** now locates sectors by matching the requested record ID
  against the track's sector list (correct CHRN addressing), with multi-sector
  reads that advance the record ID up to the end-of-track parameter. Read ID
  returns real sector headers rotating through the track. Previously the read
  path used flat-offset arithmetic against an unpopulated buffer, so a parsed
  disk mounted but returned no data.
- **+3 special paging crash**: writing certain values to port 0x1FFD could
  index the paging configuration table out of bounds and panic. The special
  paging state is now tracked correctly (an active flag separate from the 0-3
  configuration index), and the four all-RAM configurations are mapped
  correctly. The +3 no longer crashes when boot code triggers special paging.
- **Keyboard injection**: corrected the matrix position for the colon
  character, and lengthened key hold timing so the ROM keyboard scan registers
  presses reliably, including consecutive identical keys.

## [0.3.0] - 2026-06-27

Adds zenscript, a timestamped automation format for driving the emulator, and
the screen-reading and keyboard-injection primitives needed to write
self-synchronising automated tests. Scripts work in both the windowed and
headless front-ends via a new `-script` flag.

### Added

- **zenscript (`.zen`) action scripts** (`-script <file>`): a line-oriented,
  subtitle-style format of `<offset> <verb> [args]` driving the machine
  frame-by-frame. Offsets are frame counts from power-on, with `f`/`s`/`ms`
  suffixes (50 fps). A frame-driven scheduler, shared by both front-ends, fires
  actions against an effective clock; blocking verbs rebase that clock so
  scripts are portable across models. Documented in `docs/zenscript.md`.
- **Media and state verbs**: `snapshot`, `snapshot-save`, `bin`, `scr`,
  `tape-play`, `tape-stop`, `reset`, `quit`, and `shot` (named or auto-named
  PNG capture). While a script drives a run, the headless front-end's own
  interval/final-frame capture is disabled so the two do not compete.
- **`wait-boot`**: blocks until the current model has finished booting, then
  rebases the timeline, so one script adapts to each model's boot time. Boot is
  detected by a combination of the 128-family rainbow band, the bottom two text
  rows being written, and screen stabilisation. Measured per-model baselines
  (48K 1.70 s, 128K 1.22 s, +2 1.24 s, +3 2.70 s, Spanish 48K 1.70 s, Spanish
  128K 0.88 s) are recorded with a +-5% regression guard.
- **Keyboard injection**: `type <text>` (with SYMBOL SHIFT applied for
  symbols), `key <name>` (enter, delete, cursors, break, and so on), and
  `press`/`release` for held keys. Keypresses are held across several frames
  (default 5-frame hold, 3-frame gap) so the ROM keyboard scan registers them;
  shifted keys are staged (shift settles first, then the key) so consecutive
  shifted characters are not merged.
- **Screen text recognition** (`dump-screen`, `expect-screen`): reads
  characters off the display by matching each cell against the ROM font, the
  way Sinclair BASIC's `SCREEN$` does, as a pure non-invasive read. Inverse
  video is handled; only the standard ROM font is recognised.
- **Condition waits** (`wait-screen`, `wait-attr`): block the timeline until a
  row contains given text, or until a region's attributes match a colour
  predicate (`ink=`, `paper=`, `bright`, with a cell-count threshold), each
  with a timeout. These let a script synchronise on what the machine actually
  did rather than on a guessed frame, including recognising menus by colour
  when their characters use a custom font.

## [0.2.0] - 2026-06-27

Adds a CGO-free headless build and raw-binary loading, intended for automated
testing and for debugging guest software (such as zxgui) without a display.

### Added

- **Headless build** (`-tags headless`): a CGO-free variant with no window and
  no audio device. It boots a Spectrum model, runs a fixed number of frames,
  and writes PNG screenshots of the 256x192 display decoded directly from
  display memory at 0x4000. Flags: `-frames`, `-shot-interval`, `-shot-dir`,
  `-shot-prefix`, `-quiet`, plus the existing model/ROM/snapshot/tape options.
- **Raw binary (`.bin`) direct memory loading**: load a binary blob straight
  into the address space, bypassing tape and disk. New `LoadBIN` and a
  bank-aware `SpectrumMemory.Load(address uint16, data []byte)` primitive
  (matching zen80's RAM/MappedMemory `Load` convention) write through the
  normal memory path, so RAM banking and the screen mirror are honoured and
  ROM regions are protected. Both binaries accept `-bin`, `-binaddr` (hex
  `0x..`/`$..` or decimal), and `-binstart` (PC after load; empty = load
  address, `-1` = leave PC unchanged).
- **Screen dump (`.scr`) loading**: `LoadSCR` and a `-scr` flag on both binaries
  paint a raw 6912-byte screen dump (6144 bitmap + 768 attributes) onto the
  display via the normal memory path, without running anything. Pairs with
  `-frames=1` to render a `.scr` to PNG. (`.scr` was already loadable via GUI
  drag-and-drop; this adds command-line and headless access.)
- `build_headless.sh` for the CGO-free build, and a `-version` flag on both
  binaries (version injected via `-ldflags "-X main.version=..."`).
- `README.md` documenting both build variants, the headless debugging
  workflow, and a **Known issues** section (broken SNA/Z80/DSK loaders and fast
  tape loading; harsh/aliased beeper audio with approximate pitch); `LICENSE`
  (Apache 2.0), `NOTICE`, and `.gitignore`.
- Unit tests for `.bin` loading, `memory.Load`, and the address parsers.

### Changed

- GUI rendering, keyboard input, and the raylib audio backend are now behind a
  `!headless` build constraint; raylib-free equivalents back the headless
  build. Shared display constants moved to `display_constants.go`.
- The on-screen GUI controls help no longer advertises unimplemented shortcuts
  (Alt+A/V/M for audio, Alt+H/S for overlay/stripes were printed but never
  wired); it now lists only the shortcuts that are actually handled. The README
  documents the full runtime and Spectrum key mappings.
- The build scripts (`build.sh`, `build_linux.sh`, `build_windows.sh`) now use
  the Go package build instead of hand-maintained per-file lists, which had
  drifted out of sync with the sources. The Linux Docker image is bumped to
  `golang:1.25` to match the `go.mod` directive.

### Removed

- Deleted the orphaned `audio.go` (the legacy raylib audio backend). It was
  excluded from every build script and duplicated the `AYChip` type that is
  canonically defined in `ay8912.go`; its removal resolves a duplicate-symbol
  error that prevented a clean module build.

## [0.1.0] - 2026-06-26

First versioned release. Establishes the release-hygiene baseline for the
existing ZenZX codebase.

### Added

- `go.mod` declaring `module github.com/ha1tch/zenzx` (`go 1.25`), depending on
  `github.com/ha1tch/zen80`.
- `pkg/version` package and the `main.version` build-time string.
- Release scaffolding: `VERSION`, `syncver.sh`, `release.sh`, and this
  changelog.

[0.2.0]: https://github.com/ha1tch/zenzx/releases/tag/v0.2.0
[0.1.0]: https://github.com/ha1tch/zenzx/releases/tag/v0.1.0
