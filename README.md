# ZenZX

A ZX Spectrum emulator in Go, built on the [zen80](https://github.com/ha1tch/zen80)
Z80 CPU core. ZenZX emulates the 48K, 128K, +2, and +3 models (including the +3
floppy disc controller) and the Timex Sinclair 2068, with tape and snapshot
support.

See [`docs/MANUAL.md`](docs/MANUAL.md) for the organised reference (full CLI
flag tables, keyboard/joystick/media reference, configuration files); this
README is the fuller narrative account of individual features.

It builds in two variants:

- **GUI** (default): raylib rendering with audio, for interactive use. Requires
  CGO and system libraries (OpenGL, X11/Wayland, ALSA).
- **Headless** (`-tags headless`): no window, no audio device, CGO-free. Boots a
  model, runs a fixed number of frames, and writes PNG screenshots of the screen
  decoded directly from display memory. Intended for automated testing and for
  debugging guest software (such as zxgui) without a display.

## Requirements

- Go 1.25 or later
- For the GUI build only: a C toolchain and the raylib/oto system libraries
  (OpenGL, X11 or Wayland, ALSA)

## Building

```
./build.sh            # native GUI binary (needs CGO + system libs)
./build_headless.sh   # CGO-free headless binary
```

Both embed the version via `-ldflags "-X main.version=..."`, derived from
`git describe`. `./build_linux.sh` and `./build_windows.sh` cross-compile the
GUI binary. Run `./zenzx -version` or `./zenzx-headless -version` to confirm.

## Running (GUI)

```
./zenzx -model=48k
./zenzx -model=128k
./zenzx -model=plus3
./zenzx -model=ts2068
./zenzx -tape=game.tap -tapemode=accurate
```

The standard ROM set is bundled directly into the binary as of this
release -- no separate `rom/` folder is required alongside a distributed
executable. `-romdir` (default `./rom`) still works exactly as before for
local development or supplying a different ROM set entirely: any file
found there takes priority over the embedded copy, which is used only
when a named ROM isn't found on disk. See `-help` for the full flag list,
and **Known issues** below for what is currently open.

### Top menu bar

`-theme=Dark|Light|Spectrum` (default `Dark`, case-insensitive) sets
the theme active from the very first frame, without needing to open
the Theme menu first -- when `-theme` isn't given at all, the theme
last saved to `settings.json`
is used instead of this flag's own default (see **Configuring the
Machine menu and persisted preferences** below).

Hover the mouse pointer within 10px of the top window edge and hold it
perfectly still for 100ms -- a menu bar slides in (eased in, not linear)
with seven menus. Once the bar is shown, hovering any label opens its
dropdown immediately -- no click needed -- and moving to a different label
switches directly to it, the same as a traditional desktop menu bar once
any menu is already engaged.

- **Machine** -- Reset, Pause/Resume, and every supported model,
  grouped by manufacturer under a small heading and separator each:
  **Sinclair Research Ltd** (the original 48k/128k), **Amstrad plc**
  (+2/+2A/+3), **En Español** (the Spanish-market variants -- which,
  unlike the other three groups, span three different manufacturers
  across their own history rather than one, so each model gets its
  own indented sub-heading naming its actual maker: **Sinclair**
  (48k, Sinclair's own design, distributed and locally assembled in
  Spain), **Investrónica** (128k, genuinely designed by Investrónica
  itself with Sinclair's permission, and launched in Spain months
  before the UK got it), and **Amstrad** (+2 and +3, the +2
  specifically the first computer launched under Amstrad's own
  ownership after its 1986 acquisition of Sinclair)), and **Timex**
  (the Timex Sinclair 2068) -- with a checkmark on whichever model is
  currently running. Picking a model switches the running machine
  live, without restarting: reloads the target's standard ROM set,
  resets, and resets the joystick to that
  model's own default.

  This grouping is entirely configurable, not hardcoded: it's loaded
  from `machines.json` at startup -- a file in the current directory
  if one exists and validates, otherwise the copy built into the
  binary (the layout described above). A hand-edited `machines.json`
  is validated against a schema (using
  [queryfy](https://github.com/ha1tch/queryfy)) before it's used; an
  invalid one is reported and skipped in favour of the built-in
  default, never treated as fatal. The file is a flat list of nodes:

  ```json
  {"type": "separator"}
  {"type": "title", "label": "Sinclair Research Ltd"}
  {"type": "model", "id": "48k", "label": "ZX Spectrum 48k", "indent": 1}
  {"type": "submenu", "label": "More", "items": [ ... ]}
  ```

  `separator` (a divider line), `title` (a small, non-selectable
  heading, `indent` optional), `model` (a selectable entry -- `id`
  must be one of the fixed `-model` identifiers the emulator actually
  supports, `indent` optional), and `submenu` (a hover-opened nested
  menu, capped at one level of nesting) can appear in any order and
  combination. `indent` adds two spaces of leading whitespace per
  level, purely visual. What can't change is *which* models exist:
  every one of the emulator's supported `-model` identifiers must
  appear exactly once across the whole file -- rearrange, retitle,
  and regroup freely, but nothing can go missing or get duplicated.
- **Custom ROM** -- pick a file from `-custom-roms-dir` and apply it
  live (same two-step ROM-then-bank flow as `-custom-roms-menu`, for
  models with more than one bank).
- **Tape** -- Play/Stop, Rewind, toggle Accurate/Fast mode, show tape
  info -- the same actions as the `Alt+P/R/T/I` shortcuts.
- **Floppy Disk** -- **Open DSK Image...** (a live file browser, +3
  and Spanish +3 only -- prints a message and does nothing on any
  other model rather than failing silently), Save, Insert Blank,
  Eject, Save As, and Disk Info, which now reports the actual loaded
  filename and modified state rather than a static "restart with
  -disk=..." message. The rest are the same actions as `F4`-`F8`.
- **Snapshot** -- Quick Save/Load, Save Timestamped, snapshot info, run
  diagnostics -- the same actions as `F9`-`F12`.
- **View** -- **FPS Counter** and **Border Display** are checkboxes,
  showing current on/off state directly (wired to the display
  manager's own fields, so it's always correct whether last changed
  from here or via `Alt+F`/`Alt+B`), and clicking one keeps the menu
  open for toggling the other without reopening it. **Show Status** is
  unchanged, the same action as `F3`. Below a separator, **X 1**/**X
  2**/**X 3** set the emulated display's own scale directly (the same
  scale `PgUp`/`PgDn` step through one at a time), with a checkmark on
  the current one and any scale too large for the current monitor
  disabled -- refreshed every frame the bar is shown, since the scale
  can change outside the menu entirely.
- **Theme** -- switch the bar's (and every dropdown's) colour scheme
  live, with a checkmark on the current selection: **Dark** (the
  default) and **Light** (a macOS Aqua-inspired scheme) both show the
  Zen Spectrum Project's own logo on the right side of the bar, space
  permitting -- two colour-halves of ascending three-block "teeth",
  drawn as filled rectangles (not a bitmap), rotating through three
  colour arrangements every third of a second. **Spectrum** (the
  real Sinclair 128K/+2/+3 boot menu's own palette -- white panel,
  black text, bright cyan selection in the dropdown, grey separators,
  a darker standard-brightness green checkmark, and a fixed blue for
  checkboxes; a vertical gradient bar background (40% grey at the top
  to black at the bottom) with a second, horizontal gradient layered
  on top in multiply blend mode over the rainbow's own region --
  white at that region's left edge fading to black at the window's
  right edge, darkening the background there without affecting the
  rainbow itself, drawn on top of it; a 2px border with no top edge on
  the dropdown, since it always opens directly under the bar; tight
  item spacing plus extra bottom margin below the last row, matching
  the real menu's own compact rows) shows that title strip's own
  four-band diagonal rainbow instead, in the same bar slot -- red/
  yellow/green/cyan, measured pixel-by-pixel from an actual boot
  screenshot and independently confirmed against a purpose-built
  reference asset, drawn as adjacent 1px scanline segments (not a
  bitmap, and not lines -- filled rects, since raylib's own line
  drawing left a 1px gap between bands), whenever there's enough room
  for it. Below a separator, **Font** opens as its own submenu:
  switch the bar's (and every dropdown's) typeface live, with a
  checkmark on the current selection, among the bundled BDF faces --
  **Sinclair** (the default, the ZX Spectrum 48K ROM's own 8x8
  character set), **TomThumb**, **Spleen**, **Cozette**, **Creep**,
  and **HaxorMedium** -- then, below its own separator, **Zoom X1**/
  **X2**/**X3** (likewise checkmarked; unlike the font choice itself,
  this magnification factor applies only to dropdown text and layout,
  never the bar strip, which always draws at X1 regardless). X2 is
  the default, matching the size dropdowns have always drawn at.

The rainbow (Spectrum theme) or logo (Dark/Light themes) on the
right of the bar has its own dropdown -- click it directly, no hover
needed, always right-aligned to the window's own right edge regardless
of the hot zone's own width:

- **Fixed menu bar** -- a checkbox. Pins the bar permanently shown,
  bypassing the idle-at-edge auto-hide entirely, until unchecked --
  the window grows taller to accommodate it (animated, the same way
  toggling the border already animates a resize) rather than
  overlapping the Spectrum display, and shrinks back when unfixed.
- **ZenZX website** -- opens the project's own page
  (`https://ha1tch.github.io/zsp/projects/zenzx`) in the default
  browser.
- **Help** -- a scrollable reference covering every menu, keyboard
  shortcut, and command-line flag.
- **About...** -- version, project link, and licence/attribution.

The emulator keeps running the whole time -- CPU stepping and the
Spectrum's own screen never pause while the bar or a dropdown is open.
Only keyboard input to the emulated machine is withheld while a
dropdown, dialog, the logo menu, or a modal is actually open -- the
bar being merely shown (or fixed) never withholds it by itself. Move
the mouse away from the bar (with nothing open, and not fixed) and it
slides back out the same way it came in.

The bar itself (`pkg/zenui.MenuBar`) and every dropdown it opens
(`pkg/zenui.Menu`) are general, renderer-agnostic components -- reusable
by any host, not specific to this menu's contents or to raylib.

### Configuring the Machine menu (`machines.json`) and persisted preferences (`settings.json`)

The Machine menu's own layout -- which models exist, how they're
grouped, and under what headings -- comes from `machines.json`, not a
hardcoded list. A copy is bundled into the binary as the default; a
valid `machines.json` next to the executable overrides it entirely (an
invalid one is reported and the built-in default is used instead,
rather than treated as fatal). The file is a flat list of nodes:

```json
{"type": "separator"}
{"type": "title", "label": "Sinclair Research Ltd"}
{"type": "model", "id": "48k", "label": "ZX Spectrum 48k", "indent": 1}
{"type": "submenu", "label": "Amstrad plc", "items": [ ... ]}
```

- **`separator`** -- a thin divider line.
- **`title`** -- a small, non-selectable heading; `indent` (default 0)
  adds two spaces per level, for a sub-heading within a group.
- **`model`** -- a selectable entry. `id` must be one of the fixed
  `-model` identifiers (`48k`, `128k`, `plus2`, `plus2a`, `plus3`,
  `spanish48k`, `spanish128k`, `spanishplus2`, `spanishplus3`,
  `ts2068`) -- the file controls how models are presented and grouped,
  never which models exist, and every one of them must appear exactly
  once across the whole file.
- **`submenu`** -- a hover-opened nested menu, holding its own list of
  `separator`/`title`/`model` nodes (not a further `submenu` -- one
  level of nesting is the limit the menu bar's own selection reporting
  can express).

Groups, titles, and separators can be arranged in any order and
combined freely; `machines.json` at the project root is the actual
default configuration and the clearest example. Validated against a
[queryfy](https://github.com/ha1tch/queryfy) schema (`pkg/machineconfig`)
before use.

Theme, font, font zoom, display scale, and whether the menu bar is
pinned shown are saved to `settings.json` (`pkg/settingsconfig`, same
embedded-default-with-disk-override pattern, also queryfy-validated)
every time one of them changes via a menu, so the next launch resumes
where the last session left off. `-settings=PATH` points at a
different file instead of the default `settings.json`. An explicitly
passed `-theme` or `-scale` flag always wins over whatever's
persisted; leave either unset and the persisted value is used instead
of that flag's own hardcoded default. The running `-model` and FPS/
border visibility are deliberately not persisted -- an explicit model
should never be silently overridden by a stale save, and FPS/border
are session-level diagnostic toggles, not the kind of preference most
applications carry across restarts.

### Runtime keyboard shortcuts (GUI)

While the emulator is running, these host keys control it. They are intercepted
by ZenZX and are not passed to the guest.

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
| `Shift+F1` | Show a demo dropdown menu, live over the running emulator |
| `Shift+F2` | Play a demo animated notification, live over the running emulator |
| `Shift+F3` | Show a demo file-open dialog, live over the running emulator |
| `Page Up` | Increase window scale |
| `Page Down` | Decrease window scale |
| `Alt+F` | Toggle the FPS counter |
| `Alt+B` | Toggle the border |
| `Alt+P` | Play / stop the tape |
| `Alt+R` | Rewind the tape |
| `Alt+T` | Toggle tape mode (accurate / fast) |
| `Alt+I` | Print tape status and block list |

### Drag and drop

In the GUI build, dropping a file onto the window loads it by extension:

| Extension | Loads as |
|-----------|----------|
| `.scr` | A raw 6912-byte screen dump, written straight onto the framebuffer (6144 bytes of bitmap + 768 bytes of attributes). This only paints the screen — it does not change CPU state or memory, so it is a still image, not a running program. |
| `.zxs` | A ZenZX snapshot (the proprietary chunk format). |
| `.tap` / `.tzx` | A tape image (loaded, then driven with the `Alt+P/R/T/I` controls). |
| `.dsk` | A +3 disk image (requires +3 mode with the FDC enabled). |
| `.sna` / `.z80` | Standard snapshots (48K and 128K), via the shared `zentools` library. |

A `.scr` can also be loaded headlessly with `-scr` (see above).

### Spectrum keyboard mapping

Host keys are mapped to the Spectrum keyboard during emulation. Letters and
digits map directly. The composite and symbol keys are:

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

(The headless build has no input; these apply to the GUI build only.)

## Headless debugging workflow

The headless build is the tool for iterating on guest software. It loads a
program, runs it for a set number of frames, and captures the screen as PNG,
so a render can be inspected (or diffed against a reference) with no display.

Boot a model and capture the final frame:

```
./zenzx-headless -model=48k -frames=100
```

Run longer and capture periodically:

```
./zenzx-headless -model=128k -frames=500 -shot-interval=100 -shot-dir=out
```

### Loading a raw binary directly

`-bin` loads an assembled blob straight into memory, bypassing tape and disk.
This is the fastest path for debugging: assemble to a known origin, drop it in,
and run.

```
./zenzx-headless -bin=program.bin -binaddr=0x8000 -frames=200
```

- `-binaddr` is the load address (hex `0x..` / `$..`, or decimal). Default
  `0x8000`.
- `-binstart` sets PC after loading. If omitted, PC is set to the load address
  so the blob runs immediately. Pass `-binstart=-1` to load without changing PC
  (stage data while existing code runs), or any address to enter elsewhere.

Loading goes through the normal memory path, so RAM banking and the screen
mirror are honoured and writes into ROM are ignored. The address is bank-aware:
configure paging (e.g. via `-model`) before relying on a specific bank.

### Loading a screen dump

`-scr` loads a raw 6912-byte `.scr` screen dump straight onto the display
(6144 bytes of bitmap + 768 bytes of attributes). It paints the screen without
running anything, so it pairs naturally with `-frames=1` to render a screen
image to PNG:

```
./zenzx-headless -scr=expected.scr -frames=1 -shot-prefix=expected
```

This is useful for verifying screen output against reference `.scr` files.

### Headless flags

| Flag | Default | Purpose |
|------|---------|---------|
| `-model` | `48k` | 48k, 128k, plus2, plus2a, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3, ts2068 |
| `-rom` | | Custom ROM bank(s), comma-separated, positionally mapped to bank 0,1,2,3 (up to the model's own bank count); layers on top of `-model`'s standard set |
| `-rom0` | | Override just ROM bank 0, leaving the rest of `-model`'s standard set intact |
| `-rom1` | | Override just ROM bank 1, leaving the rest of `-model`'s standard set intact |
| `-rom2` | | Override just ROM bank 2, leaving the rest of `-model`'s standard set intact |
| `-rom3` | | Override just ROM bank 3 (e.g. +3DOS on a +3), leaving the rest of `-model`'s standard set intact |
| `-romdir` | `./rom` | Directory checked first for ROM files; falls back to the binary's embedded copy if not found there |
| `-custom-roms-menu` | `false` | Interactively pick a ROM from `-custom-roms-dir` and a bank to apply it to -- drawn as an on-screen menu in the GUI build, a text prompt in the headless build |
| `-custom-roms-dir` | `custom-roms` | Directory scanned by `-custom-roms-menu` |
| `-bin` | | Raw binary to load into memory |
| `-binaddr` | `0x8000` | Load address for `-bin` |
| `-binstart` | | PC after `-bin` (empty = load address, `-1` = unchanged) |
| `-scr` | | Load a `.scr` screen dump onto the display (still image) |
| `-snapshot` | | Snapshot to load on startup |
| `-format` | `auto` | Snapshot format: auto, zxs, sna, z80 |
| `-tape` | | Tape file (.tap / .tzx) |
| `-tapemode` | `fast` | Tape mode: `fast`, `accurate`, or `turbo` (accurate mode's real CPU/tape correctness with rendering and audio/mouse bookkeeping skipped while the tape plays -- a genuine but modest wall-clock speedup, not a shortcut; the screen visibly freezes during a turbo load rather than redrawing incrementally, then jumps to the final state; see `docs/TAPE_LOADING_HANDOVER.md` for how the three modes actually compare) |
| `-disk` | | Path to a +3 disk image (.dsk) |
| `-nofdc` | `false` | Disable FDC emulation for +3 |
| `-non-standard` | `off` | Master switch for non-standard features (`on`/`off`); gates all `-ns-*` flags below |
| `-ns-graphics` | | Non-standard graphics mode (requires `-non-standard on`): mode-timex-001-hicolour, mode-zenzx-01, mode-zenzx-02 |
| `-ns-storage` | | Non-standard storage backend (requires `-non-standard on`): storage-zenzx-posix |
| `-joystick` | `auto` | Joystick emulation: auto (the selected `-model`'s own built-in configuration -- see below), none, kempston, kempston2, kempston-both, sinclair (alias for sinclair1), sinclair1, sinclair2, sinclair-both |
| `-mouse` | `none` | Mouse emulation: none, kempston, amx |
| `-script` | | Path to a `.zen` action script to drive the emulator |
| `-frames` | `100` | Number of frames to run |
| `-shot-interval` | `0` | Capture every N frames (0 = final frame only) |
| `-shot-dir` | `.` | Directory for screenshots |
| `-shot-prefix` | `zenzx` | Screenshot filename prefix |
| `-quiet` | | Suppress per-frame logging |

`-joystick auto` (the default) configures whichever joystick hardware the
selected `-model` genuinely came with, not a guess -- verified against real
hardware history, not assumed:

- **48K, and the original Sinclair 128K** ("Toastrack", pre-Amstrad): no
  built-in joystick port of any kind. Sinclair-compatible ports were "a
  feature that really should have been part of the 128 specification from
  the start" and only arrived with Amstrad's +2.
- **+2 (grey), +2A, +3** (and their Spanish variants): two built-in
  Sinclair-protocol ports (`sinclair-both`), badged "SJS" -- genuinely
  Sinclair Interface 2's own keyboard-mapping mechanism electrically, not
  Kempston, confirmed against multiple independent sources including real
  +2/+3 hardware port testing.
- **TS2068**: has its own, separate, always-on built-in joystick mechanism
  (via the AY-3-8912's I/O port, not Kempston-compatible -- confirmed
  directly against timexsinclair.com, the Timex/Sinclair preservation
  project) that is active for that model regardless of `-joystick`
  entirely; `auto` correctly resolves to `none` for the canonical-Spectrum
  mechanism this flag configures, since it doesn't apply. See
  `docs/timex-modes.md` and `docs/TS2068_DEVELOPMENT_PLAN.md` for detail.

An explicit `-joystick` value always overrides the model default -- a
third-party interface (Kempston being the most common) was a legitimate
choice on any model historically, including ones with their own built-in
ports, and remains one here.

`kempston2`/`kempston-both` (a second, independent Kempston-protocol
port at `0x37`) are never a `-model` default -- no stock Sinclair/
Amstrad/Timex machine ever had one built in. They're real, but belong to
modern "neo-Spectrum" platforms (ZX-Uno, ZX-Tres, the Omni, the ZX
Spectrum Next), confirmed against the Next's own I/O port register
documentation and cross-checked against a separate hobbyist interface's
own port numbering. Select them explicitly if you're specifically
emulating one of those platforms' own enhancements -- the same category
of thing `-ns-graphics`/`-ns-storage` exist for.

Screenshots are 256x192 RGB PNGs of the screen area (the border is not part of
the display file and is omitted). The decoder reproduces the Spectrum's true
palette, so output is white/black/colour, not a remapped monochrome.

## Known issues

Open defects and gaps live in [`docs/TRACKING.md`](docs/TRACKING.md) (the
live register); intentional limits and the dormant-guard table live in
[`docs/KNOWN_ISSUES.md`](docs/KNOWN_ISSUES.md); closed items are recorded in
[`docs/RESOLVED.md`](docs/RESOLVED.md). The headline items at this version:

- **TS2068 support.** `-model ts2068` boots to a genuinely responsive
  BASIC prompt, with its own NTSC timing, dynamic hi-colour mode
  switching (guest-triggered, not the static host flag other non-standard
  modes use), AY sound and built-in joystick ports, and both tape modes
  -- all verified against the real ROM, not assumed. Deliberately out of
  scope: full 8-chunk Dock/cartridge banking and the TS2040 printer.
  Memory contention is modelled precisely for 48K (T-16, closed --
  measured position-error bounds of ~0.33% memory / ~0% I/O against
  real loader execution); 128K's model is an address-range
  approximation, not bank-aware (T-20, open). See
  `docs/TS2068_DEVELOPMENT_PLAN.md` and `docs/TS2068_TRACKING.md` for
  the full stage-by-stage detail.
- **Pitch is approximate** (T-03). The CPU clock is modelled as 3.5 MHz exactly,
  whereas a real 48K runs at 3.5469 MHz (~1.3% higher); audio sample timing is
  derived from that constant with per-sample truncation, so pitch is slightly
  low and can drift.
- **AY audio: residual aliasing** (T-04). The beeper is DC-blocked and low-pass
  filtered (14 kHz Butterworth) and the AY-3-8912 output is area-sampled, which
  removed most of the harshness; some high-frequency aliasing remains on the AY.

Audio issues do not affect the headless build, which produces no audio.

- **Real-world tape loading is not uniformly reliable** (T-22, T-23, T-24).
  A meaningful fraction of real commercial tapes use custom/protected
  loaders that either don't decode correctly yet (T-22 -- Speedlock-class
  protection, root cause open after extensive investigation) or get no
  benefit from `-tapemode=fast`'s ROM-trap once such a loader takes over
  (T-23). More fundamentally, no tape mode's own self-reported "finished
  loading" signal has proven reliably trustworthy on its own (T-24) --
  see `docs/TAPE_LOADING_HANDOVER.md` for the full picture and
  `contrib/tape-corpus-harness/` for a tool that measures this against a
  real corpus.

## Versioning and releases

`VERSION` is canonical; `pkg/version/version.go` is a synced stamp. Tooling
lives in `repoman/` (the [repoman](https://github.com/ha1tch/repoman)
toolset, driven by `.repoman.json`):

```
python3 repoman/syncver.py show
python3 repoman/register.py list         # open work
python3 repoman/guards.py stale          # dormant guards not run since the last release
python3 repoman/relcore.py <version>     # sync, gates, build, test, smoke, checkpoint zip
```

`relcore.py` journals each step in `.release-state.json` and resumes with
`--resume`; full output goes to `release-<version>.log`. Before a release, every
stale dormant guard is run, handed off (`guards.py handoff`), or its skip is
recorded in the changelog entry.

## Continuous integration and cross-platform builds

`.github/workflows/ci.yml` builds and tests every push: the cgo-free
headless core, the GUI cross-compiled for darwin/windows (amd64 and
arm64, cgo-free via raylib-go's `purego` backend), and the GUI natively
for linux/amd64 and linux/arm64 (cgo, via apt-installed ALSA/X11/Wayland/
GL dev libraries). See `docs/KNOWN_ISSUES.md` ("CI and cross-platform
builds") for why the split falls where it does, and why BSD isn't in it.

Pushing a `v*` tag runs `.github/workflows/release.yml`, which builds and
publishes prebuilt archives (headless and GUI, all platforms above, plus
headless-only for freebsd/amd64) as GitHub release assets. This is
separate from `relcore.py`'s local checkpoint: run the local release
first to gate and verify, then tag once it's green.

## Project layout

Emulation core (raylib-free, shared by both builds): `zenzx.go`, `memory.go`,
`io.go`, `fdc.go`, `tape*.go`, `snapshot*.go`, `ay8912.go`, `audio_*.go`,
`loadbin.go`, `ts2068.go`, `joystick.go`, `mouse.go`. GUI-only (`!headless`):
`display.go`, `input.go`, `audio_oto.go`, `zenzx_gui.go`. Headless-only:
`display_headless.go`, `audio_oto_headless.go`, `zenzx_headless.go`.

`contrib/tape-corpus-harness/` is a standalone tool for measuring
real-world tape-loading coverage, in its own Go module with no import
relationship to zenzx in either direction -- `go list ./...` from the
repo root doesn't see it, and neither does the release build or test
gate. See its own README.


## Contact

Email: h@ual.li

https://oldbytes.space/@haitchfive

## License

Copyright 2026 h@ual.li

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
