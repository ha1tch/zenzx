# ZenZX Manual

A reference for building, running, and driving ZenZX -- a ZX Spectrum
emulator in Go, built on the [zen80](https://github.com/ha1tch/zen80) Z80
CPU core. This manual is the organised reference; [`README.md`](../README.md)
at the project root is the fuller narrative account of individual features
as they landed, and is worth reading for the detail this manual summarises.

## Contents

1. [Overview](#overview)
2. [Installing and building](#installing-and-building)
3. [Quick start](#quick-start)
4. [Supported models](#supported-models)
5. [Command-line reference -- GUI](#command-line-reference--gui)
6. [Command-line reference -- headless](#command-line-reference--headless)
7. [Keyboard](#keyboard)
8. [Joystick and mouse](#joystick-and-mouse)
9. [Media formats](#media-formats)
10. [The menu bar (GUI)](#the-menu-bar-gui)
11. [Configuration files](#configuration-files)
12. [Scripting with zenscript](#scripting-with-zenscript)
13. [Headless debugging workflow](#headless-debugging-workflow)
14. [Non-standard features](#non-standard-features)
15. [Known issues and further reading](#known-issues-and-further-reading)
16. [Versioning and releases](#versioning-and-releases)
17. [Project layout](#project-layout)
18. [License and contact](#license-and-contact)

## Overview

ZenZX emulates the 48K, 128K, +2, +2A, +3 (including the +3 floppy disc
controller), the Spanish-market variants of each of those, and the Timex
Sinclair 2068, with tape and snapshot support. It builds in two variants
from the same emulation core:

- **GUI** (default) -- raylib rendering with audio, for interactive use.
  Requires CGO and system libraries (OpenGL, X11/Wayland, ALSA).
- **Headless** (`-tags headless`) -- no window, no audio device, CGO-free.
  Boots a model, runs a fixed number of frames, and writes PNG screenshots
  of the screen decoded directly from display memory. Built for automated
  testing and for debugging guest software without a display.

Both share the same underlying machine emulation (`zenzx.go`, `memory.go`,
`io.go`, `fdc.go`, tape/snapshot/audio/joystick/mouse handling) -- the split
is only in front-end plumbing (windowing, audio device, input), so a
`.zen` script or a raw binary behaves identically in either build.

## Installing and building

Requirements:

- Go 1.25 or later
- For the GUI build only: a C toolchain and the raylib/oto system libraries
  (OpenGL, X11 or Wayland, ALSA)

```
./build.sh            # native GUI binary (needs CGO + system libs)
./build_headless.sh   # CGO-free headless binary
./build_linux.sh       # cross-compile the GUI binary for Linux
./build_windows.sh     # cross-compile the GUI binary for Windows
```

Both binaries embed the version via `-ldflags "-X main.version=..."`,
derived from `git describe`. Confirm with `-version`:

```
./zenzx -version
./zenzx-headless -version
```

The standard ROM set is bundled directly into the binary -- no separate
`rom/` folder is required alongside a distributed executable. `-romdir`
(default `./rom`) still works for local development or a different ROM
set entirely: any file found there takes priority over the embedded copy,
which is used only when a named ROM isn't found on disk.

## Quick start

```
./zenzx -model=48k
./zenzx -model=128k
./zenzx -model=plus3
./zenzx -model=ts2068
./zenzx -tape=game.tap -tapemode=accurate
```

Headless, for automated inspection rather than interactive play:

```
./zenzx-headless -model=48k -frames=100
./zenzx-headless -model=128k -frames=500 -shot-interval=100 -shot-dir=out
```

## Supported models

`-model` accepts: `48k`, `128k`, `plus2`, `plus2a`, `plus3`, `spanish48k`,
`spanish128k`, `spanishplus2`, `spanishplus3`, `ts2068`.

The Spanish-market variants span three different manufacturers across their
own history rather than one: **Sinclair** (`spanish48k` -- Sinclair's own
design, distributed and locally assembled in Spain), **Investrónica**
(`spanish128k` -- genuinely designed by Investrónica itself with Sinclair's
permission, and launched in Spain months before the UK got the 128K), and
**Amstrad** (`spanishplus2`/`spanishplus3`).

TS2068 support (`-model=ts2068`) boots to a genuinely responsive BASIC
prompt, with its own NTSC timing, dynamic hi-colour mode switching
(guest-triggered, not a static host flag), AY sound, built-in joystick
ports, and both tape modes -- verified against the real ROM, not assumed.
Deliberately out of scope: full 8-chunk Dock/cartridge banking and the
TS2040 printer. Memory contention is modelled precisely for 48K
(measured ~0.33% memory / ~0% I/O position-error bounds against real
loader execution); 128K's model is an address-range approximation, not
bank-aware (see `docs/TRACKING.md` T-20). See
[`docs/TS2068_DEVELOPMENT_PLAN.md`](TS2068_DEVELOPMENT_PLAN.md)
and [`docs/TS2068_TRACKING.md`](TS2068_TRACKING.md) for stage-by-stage detail.

## Command-line reference -- GUI

| Flag | Default | Description |
|------|---------|-------------|
| `-model` | `48k` | Spectrum model: `48k`, `128k`, `plus2`, `plus2a`, `plus3`, `spanish48k`, `spanish128k`, `spanishplus2`, `spanishplus3`, `ts2068` |
| `-rom` | *(none)* | Custom ROM bank(s), comma-separated, positionally mapped to bank 0,1,2,3 (up to the model's own bank count); applied on top of `-model`'s standard set, not instead of it |
| `-rom0` .. `-rom3` | *(none)* | Override a single ROM bank, leaving the rest of `-model`'s standard set intact |
| `-custom-roms-menu` | `false` | Interactively pick a ROM from `-custom-roms-dir` and a bank to apply it to |
| `-custom-roms-dir` | `custom-roms` | Directory scanned by `-custom-roms-menu` |
| `-theme` | `Dark` | Menu bar UI theme: `Dark`, `Light`, or `Spectrum` (case-insensitive) |
| `-scale` | `2` | Initial window scale (1-5) |
| `-settings` | `settings.json` | Path to the settings file (persists theme/font/font-zoom/display-scale/fixed-menu-bar across sessions) |
| `-noborder` | `false` | Start without border |
| `-noesc` | `false` | Disable the `Esc` quit key (prevent accidental exit) |
| `-nofdc` | `false` | Disable FDC emulation for +3 |
| `-disk` | *(none)* | Path to a +3 disk image (`.dsk`) |
| `-debugfdc` | `false` | Enable FDC debug output |
| `-snapshot` | *(none)* | Load a snapshot on startup |
| `-format` | `auto` | Snapshot format: `auto`, `zxs`, `sna`, `z80` |
| `-tape` | *(none)* | Load a tape file (`.tap` or `.tzx`) |
| `-tapemode` | `fast` | Tape mode: `fast`, `accurate`, or `turbo` (same correctness as `accurate`, rendering/audio bookkeeping skipped while loading -- see [Media formats](#media-formats)) |
| `-noaudio` | `false` | Disable audio |
| `-audiobackend` | `oto` | Audio backend: `raylib` or `oto` |
| `-bin` | *(none)* | Load a raw binary blob directly into memory |
| `-binaddr` | `0x8000` | Load address for `-bin` (hex `0x..` or decimal) |
| `-binstart` | *(load address)* | PC start address after `-bin`; empty = use load address, `-1` = leave PC unchanged |
| `-scr` | *(none)* | Load a raw `.scr` screen dump onto the display (still image) |
| `-script` | *(none)* | Path to a `.zen` action script to drive the emulator -- see [Scripting with zenscript](#scripting-with-zenscript) |
| `-non-standard` | `off` | Master switch for non-standard features: `on` or `off`. Gates all `-ns-*` flags |
| `-ns-graphics` | *(none)* | Non-standard graphics mode (requires `-non-standard on`) -- see [Non-standard features](#non-standard-features) |
| `-ns-storage` | *(none)* | Non-standard storage backend (requires `-non-standard on`) |
| `-joystick` | `auto` | `auto` (the model's own built-in configuration), `none`, `kempston`, `sinclair` (alias for `sinclair1`), `sinclair1`, `sinclair2`, `sinclair-both` |
| `-mouse` | `none` | `none` or `kempston` |
| `-version` | `false` | Print version and exit |

## Command-line reference -- headless

The headless binary shares all tape/model/ROM/snapshot flags with the
GUI build (`-tapemode` included -- all three modes, `fast`/`accurate`/
`turbo`, work identically in both) but has no window, audio, or
settings-persistence flags, and adds its own frame-count and
screenshot controls.

| Flag | Default | Description |
|------|---------|-------------|
| `-frames` | `100` | Number of frames to run |
| `-shot-interval` | `0` | Capture a screenshot every N frames (`0` = only the final frame) |
| `-shot-dir` | `.` | Directory to write screenshots into |
| `-shot-prefix` | `zenzx` | Filename prefix for auto-named screenshots |
| `-quiet` | `false` | Suppress per-frame logging |
| `-romdir` | `./rom` | Directory containing ROM files |

When a `-script` is given, the front-end's own periodic screenshot
mechanism (`-shot-interval` and final-frame capture) is disabled -- the
script's `shot` actions become the only source of screenshots. See
[Headless debugging workflow](#headless-debugging-workflow) for a worked
example, and [`docs/zenscript.md`](zenscript.md) for the full scripting
reference.

## Keyboard

### Runtime shortcuts (GUI)

These host keys control the emulator directly and are not passed to the
guest:

| Key | Action |
|-----|--------|
| `Esc` | Quit (unless started with `-noesc`) |
| `F1` | Reset the machine |
| `F2` | Pause / resume |
| `F3` | Print machine status to the console |
| `F4` | Save the current disk image (+3, if modified) |
| `F5` | Insert a blank disk (+3) |
| `F6` | Eject the disk (+3) |
| `F7` | Print disk-loading instructions |
| `F8` | Save the disk image to a new file (+3) |
| `F9` | Quick-save snapshot |
| `F10` | Quick-load snapshot |
| `F11` | Save a timestamped snapshot |
| `F12` | Snapshot info; `Shift+F12` runs snapshot diagnostics |
| `Page Up` / `Page Down` | Increase / decrease window scale |
| `Alt+F` | Toggle the FPS counter |
| `Alt+B` | Toggle the border |
| `Alt+P` | Play / stop the tape |
| `Alt+R` | Rewind the tape |
| `Alt+T` | Toggle tape mode (accurate / fast) |
| `Alt+I` | Print tape status and block list |

### Spectrum keyboard mapping

Host keys map onto the emulated Spectrum keyboard. Letters and digits map
directly; the composite and symbol keys are:

| Host key | Spectrum |
|----------|----------|
| `Shift` | CAPS SHIFT |
| `Ctrl` | SYMBOL SHIFT |
| `Tab` | EXTEND MODE (CAPS SHIFT + SYMBOL SHIFT) |
| `` ` `` (backtick) | EDIT (CAPS SHIFT + 1) |
| `Caps Lock` | CAPS LOCK (CAPS SHIFT + 2) |
| `Backspace` / `Delete` | DELETE (CAPS SHIFT + 0) |
| Arrow keys | CAPS SHIFT + 5/6/7/8 (left/down/up/right) |
| `,` `.` `;` `'` `/` `-` `=` `[` `]` | SYMBOL SHIFT + the matching Spectrum symbol |

The headless build has no interactive input; this mapping applies to the
GUI build only. Headless (and GUI) scripted key injection via `.zen`
scripts is a separate mechanism -- see
[Scripting with zenscript](#scripting-with-zenscript).

## Joystick and mouse

`-joystick` selects emulated joystick hardware: `auto` uses the selected
model's own built-in default configuration, `none` disables it, and
`kempston`, `sinclair1`, `sinclair2`, and `sinclair-both` select specific
hardware (`sinclair` is an alias for `sinclair1`). `-mouse` selects `none`
(default) or `kempston` for Kempston mouse emulation.

## Media formats

Standard snapshot formats (`.sna`, `.z80`) and the proprietary `.zxs`
chunk format are supported via `-snapshot`/`snapshot-save`, with
`-format auto` detecting from content. Tapes (`.tap`, `.tzx`) load via
`-tape`, with `-tapemode fast|accurate|turbo` controlling load speed
in both builds -- `fast` traps the ROM's own loader for a near-instant
result but only while a standard loader is active; `accurate` runs the
real pulse timeline at the tape's own recorded speed; `turbo` is the
same correctness as `accurate` with rendering/audio bookkeeping
skipped while loading, for a real but modest wall-clock speedup. In
the GUI build specifically, the screen visibly freezes during a turbo
load rather than redrawing incrementally, then jumps to the final
state once the tape finishes -- turbo's screen buffer is frozen
throughout the fast path by design, not a bug. See
`docs/TAPE_LOADING_HANDOVER.md` in the repo for the fuller comparison
and known limitations of each mode. +3 disk
images (`.dsk`) load via `-disk` (requires +3 mode with the FDC enabled;
`-nofdc` disables FDC emulation entirely). A raw `.scr` screen dump loads
via `-scr` in either build -- a still image only, since it paints the
framebuffer without touching CPU state or memory.

In the GUI build, dropping a file onto the window loads it by extension:

| Extension | Loads as |
|-----------|----------|
| `.scr` | A raw 6912-byte screen dump (6144 bytes bitmap + 768 bytes attributes), painted directly onto the framebuffer |
| `.zxs` | A ZenZX snapshot |
| `.tap` / `.tzx` | A tape image, then driven with the `Alt+P/R/T/I` controls |
| `.dsk` | A +3 disk image (requires +3 mode with the FDC enabled) |
| `.sna` / `.z80` | Standard snapshots (48K and 128K), via the shared `zentools` library |

## The menu bar (GUI)

Hover the mouse within 10px of the top window edge and hold it still for
100ms -- a menu bar slides in with seven menus: **Machine** (reset,
pause/resume, and every supported model grouped by manufacturer, switched
live without restarting), **Custom ROM**, **Tape**, **Floppy Disk**,
**Snapshot**, **View** (FPS counter, border display, display scale),
and **Theme** (Dark/Light/Spectrum, plus a font submenu with six bundled
BDF faces). A logo/rainbow element at the right of the bar opens its own
dropdown: **Fixed menu bar** (pin it permanently shown), **ZenZX website**,
**Help** (a scrollable in-app reference covering every menu, shortcut, and
flag), and **About**.

The emulator keeps running the whole time the bar is open -- only keyboard
input to the guest is withheld while a dropdown, dialog, or modal is
actually open. The bar's layout, theming, and behaviour are covered in
full prose detail (including the exact palette and rendering choices for
each theme) in the project [`README.md`](../README.md#top-menu-bar); this
manual gives the practical summary above rather than repeating it.

## Configuration files

**`machines.json`** controls the Machine menu's layout -- which models
exist under which heading, and how they're grouped. A copy is bundled
into the binary as the default; a valid file at the project root
overrides it entirely (an invalid one is reported and the built-in
default is used, never treated as fatal). Validated against a
[queryfy](https://github.com/ha1tch/queryfy) schema before use. Every
supported `-model` identifier must appear exactly once across the file;
grouping and labelling are otherwise freely configurable.

**`settings.json`** persists theme, font, font zoom, display scale, and
whether the menu bar is pinned shown, updated automatically whenever one
changes via a menu. `-settings=PATH` points at a different file. An
explicitly passed `-theme` or `-scale` flag always wins over the
persisted value; leave either unset and the persisted value applies
instead of that flag's own default. The running `-model` and FPS/border
visibility are deliberately not persisted.

## Scripting with zenscript

`.zen` scripts drive the emulator through a timed sequence of actions --
loading media, injecting keypresses, waiting on screen text, colour
attributes, or (as of this release) live memory values resolved by symbol
name, and capturing screenshots -- identically in both front-ends via
`-script`. This is the primary way to build reproducible, frame-accurate
automated checks against guest software.

```
zenzx-headless -model=48k -script=session.zen -frames=400 -shot-dir=out
zenzx          -model=plus3 -script=session.zen
```

See [`docs/zenscript.md`](zenscript.md) for the complete verb reference,
including the newest additions: `sym` (load a `.sym` symbol table),
`wait-mem`/`expect-mem` (block on or assert a live memory value by symbol
name or address), and `shot`'s `zoom=N` argument (nearest-neighbour
upscaling of a capture, for inspecting fine sprite/tile detail).

## Headless debugging workflow

The headless build is the tool for iterating on guest software without a
display: load a program, run it for a set number of frames, capture the
screen as PNG, inspect or diff the result.

Boot a model and capture the final frame:

```
./zenzx-headless -model=48k -frames=100
```

Run longer and capture periodically:

```
./zenzx-headless -model=128k -frames=500 -shot-interval=100 -shot-dir=out
```

Load a raw binary directly (bypassing tape/snapshot entirely) and run it
from a cold start:

```
./zenzx-headless -model=48k -bin=game.bin -binaddr=0x8000 -frames=100
```

For anything beyond "run N frames and screenshot the end" -- driving
input, waiting on state, multi-stage checks -- a `.zen` script (previous
section) is the more capable tool; the flags above cover the simple case
without needing one.

## Non-standard features

`-non-standard on` is the master switch gating every `-ns-*` flag; with it
left at `off` (the default), every `-ns-*` flag is rejected and behaviour
is standard-hardware-accurate throughout.

- `-ns-graphics` accepts one of: `mode-timex-001-hicolour` (Extended
  Colour Mode: 256x192 with 32x192 attribute resolution / 8x1 blocks --
  genuine Timex period hardware behaviour, gated behind this flag
  because it's non-standard relative to a *plain* Spectrum, not because
  it's fictional; see [`docs/timex-modes.md`](timex-modes.md)),
  `mode-zenzx-01` (256x192, 3px/byte, linear framebuffer, no attribute
  clash -- not a real historical mode), or `mode-zenzx-02` (512x384,
  double resolution -- likewise not a real historical mode).
- `-ns-storage` accepts: `storage-zenzx-posix` -- a non-standard storage
  backend.

These are opt-in and clearly separated from standard emulation so that
default behaviour is never in question; see
[`docs/KNOWN_ISSUES.md`](KNOWN_ISSUES.md) for the reasoning behind
specific choices.

## Known issues and further reading

Open defects and gaps live in [`docs/TRACKING.md`](TRACKING.md) (the live
register); intentional limits and the dormant-guard table live in
[`docs/KNOWN_ISSUES.md`](KNOWN_ISSUES.md); closed items are recorded in
[`docs/RESOLVED.md`](RESOLVED.md). Headline items at this version:

- **Pitch is approximate.** The CPU clock is modelled as 3.5 MHz exactly,
  whereas a real 48K runs at 3.5469 MHz (~1.3% higher); audio sample
  timing is derived from that constant with per-sample truncation, so
  pitch is slightly low and can drift.
- **AY audio: residual aliasing.** The beeper is DC-blocked and low-pass
  filtered (14 kHz Butterworth) and the AY-3-8912 output is
  area-sampled, which removed most of the harshness; some high-frequency
  aliasing remains on the AY.
- **Real-world tape loading is not uniformly reliable.** A meaningful
  fraction of real commercial tapes use custom/protected loaders that
  either don't decode correctly yet (Speedlock-class protection, root
  cause open after extensive investigation) or get no benefit from
  `-tapemode=fast`'s ROM-trap once such a loader takes over. More
  fundamentally, no tape mode's own self-reported "finished loading"
  signal has proven reliably trustworthy on its own. See
  `docs/TAPE_LOADING_HANDOVER.md` for the full picture and
  `contrib/tape-corpus-harness/` for a tool that measures this against
  a real corpus.

Audio issues do not affect the headless build, which produces no audio.

Other subsystem-specific documents: [`docs/video-architecture.md`](video-architecture.md)
(display rendering pipeline, GPU and hi-colour paths) and
[`docs/timex-modes.md`](timex-modes.md) (TS2068 hi-colour mode detail).

## Versioning and releases

`VERSION` is canonical; `pkg/version/version.go` is a synced stamp.
Release tooling lives in `repoman/` (the
[repoman](https://github.com/ha1tch/repoman) toolset, driven by
`.repoman.json`):

```
python3 repoman/syncver.py show
python3 repoman/register.py list         # open work
python3 repoman/guards.py stale          # dormant guards not run since the last release
python3 repoman/relcore.py <version>     # sync, gates, build, test, smoke, checkpoint zip
```

`relcore.py` journals each step in `.release-state.json` and resumes with
`--resume`; full output goes to `release-<version>.log`. Before a
release, every stale dormant guard is run, handed off, or its skip is
recorded in the changelog entry.

`.github/workflows/ci.yml` builds and tests every push; pushing a `v*`
tag runs `.github/workflows/release.yml`, publishing prebuilt archives as
GitHub release assets. This is separate from `relcore.py`'s local
checkpoint -- run the local release first to gate and verify, then tag
once it's green.

## Project layout

Emulation core (raylib-free, shared by both builds): `zenzx.go`,
`memory.go`, `io.go`, `fdc.go`, `tape*.go`, `snapshot*.go`, `ay8912.go`,
`audio_*.go`, `loadbin.go`, `symtab.go`, `ts2068.go`, `joystick.go`,
`mouse.go`, `scheduler.go`, `script.go`. GUI-only (`!headless`):
`display.go`, `input.go`, `audio_oto.go`, `zenzx_gui.go`. Headless-only:
`display_headless.go`, `audio_oto_headless.go`, `zenzx_headless.go`.

`contrib/tape-corpus-harness/` is a standalone tool for measuring
real-world tape-loading coverage, in its own Go module with no import
relationship to zenzx in either direction -- it's outside both the
release build and the test gate. See its own README.

## License and contact

Licensed under the Apache License, Version 2.0. See
[`LICENSE`](../LICENSE) or <http://www.apache.org/licenses/LICENSE-2.0>.

Email: h@ual.li
Fediverse: <https://oldbytes.space/@haitchfive>
