# ZenZX

A ZX Spectrum emulator in Go, built on the [zen80](https://github.com/ha1tch/zen80)
Z80 CPU core. ZenZX emulates the 48K, 128K, +2, and +3 models (including the +3
floppy disc controller), with tape and snapshot support.

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
./zenzx -tape=game.tap -tapemode=accurate
```

ROM files are loaded from `./rom`. See `-help` for the full flag list, and
**Known issues** below for loaders that do not currently work (`.dsk` images).

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

See **Known issues** for the formats that do not currently load correctly
(`.dsk`). A `.scr` can also be loaded headlessly with `-scr` (see above).

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
| `-model` | `48k` | 48k, 128k, plus2, plus3, spanish48k, spanish128k, spanishplus2, spanishplus3 |
| `-rom` | | Path to a custom ROM file |
| `-romdir` | `./rom` | Directory containing the ROM files |
| `-bin` | | Raw binary to load into memory |
| `-binaddr` | `0x8000` | Load address for `-bin` |
| `-binstart` | | PC after `-bin` (empty = load address, `-1` = unchanged) |
| `-scr` | | Load a `.scr` screen dump onto the display (still image) |
| `-snapshot` | | Snapshot to load on startup |
| `-format` | `auto` | Snapshot format: auto, zxs, sna, z80 |
| `-tape` | | Tape file (.tap / .tzx) |
| `-tapemode` | `fast` | Tape mode: fast or accurate |
| `-frames` | `100` | Number of frames to run |
| `-shot-interval` | `0` | Capture every N frames (0 = final frame only) |
| `-shot-dir` | `.` | Directory for screenshots |
| `-shot-prefix` | `zenzx` | Screenshot filename prefix |
| `-quiet` | | Suppress per-frame logging |

Screenshots are 256x192 RGB PNGs of the screen area (the border is not part of
the display file and is omitted). The decoder reproduces the Spectrum's true
palette, so output is white/black/colour, not a remapped monochrome.

## Known issues

As of this version, the following are known not to work or not to work well.

**Loading**

- **Fast tape loading does not work.** Use `-tapemode=accurate`, which performs
  pulse-level emulation and loads correctly. The default `-tapemode=fast`
  (ROM-trap acceleration) is broken.

The `-bin` direct-memory loader (see above) is unaffected by these and is the
most reliable way to get code into the machine for testing.

**Audio**

- **Beeper audio is harsh and aliased.** The beeper is synthesised by
  integrating the speaker's duty cycle per output sample (44.1kHz) with no
  reconstruction or low-pass filtering, and the output is unipolar (0..volume).
  A square-wave source resampled this way aliases badly, which is heard as a
  strong jagged/buzzy edge on the tone. Proper band-limited synthesis (or at
  least a low-pass / anti-aliasing stage) would smooth this.
- **Pitch is approximate.** The CPU clock is modelled as 3.5 MHz exactly,
  whereas a real 48K runs at 3.5469 MHz (~1.3% higher). Audio sample timing is
  derived from that constant with per-sample integer truncation, so pitch is
  slightly off and can drift. This also affects frame timing marginally.

These audio issues do not affect the headless build, which produces no audio.



Versioning follows the same scheme as the wider toolchain. `VERSION` and
`pkg/version/version.go` are kept in sync by `syncver.sh`; `release.sh`
validates, builds, tests, runs a headless smoke test, and cuts a checkpoint.

```
./syncver.sh show
./release.sh <version>
```

## Project layout

Emulation core (raylib-free, shared by both builds): `zenzx.go`, `memory.go`,
`io.go`, `fdc.go`, `tape*.go`, `snapshot*.go`, `ay8912.go`, `audio_*.go`,
`loadbin.go`. GUI-only (`!headless`): `display.go`, `input.go`, `audio_oto.go`,
`zenzx_gui.go`. Headless-only: `display_headless.go`, `audio_oto_headless.go`,
`zenzx_headless.go`.


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
