# zenscript (`.zen`) reference

zenscript is a small, line-oriented automation format for driving ZenZX. A
`.zen` file is a timed list of actions performed on the virtual Spectrum,
modelled loosely on a subtitle file: each line carries a time offset and an
action. It lets you boot a model, load media, inject keypresses, capture
screenshots, and save snapshots in a reproducible, frame-accurate way.

Scripts work identically in both the windowed (`zenzx`) and headless
(`zenzx-headless`) front-ends, supplied with the `-script` flag:

```
zenzx-headless -model=48k -script=session.zen -frames=400 -shot-dir=out
zenzx          -model=plus3 -script=session.zen
```

The conventional file extension is `.zen`.

## Line format

```
<offset>  <verb>  [args...]
```

Fields are separated by whitespace. Blank lines and lines whose first
non-space character is `#` are ignored:

```
# boot, then screenshot two seconds in
2s   shot   booted
```

A small number of barrier verbs (currently only `wait-boot`) take no offset and
may appear as a lone token on their own line.

## Time offsets

The offset is a frame count from power-on by default. ZenZX runs at 50 frames
per second (PAL), so one frame is 20 ms. A unit suffix may be given and is
converted to frames when the script is loaded:

| Token   | Meaning            | Frames |
|---------|--------------------|--------|
| `100`   | 100 frames         | 100    |
| `100f`  | 100 frames         | 100    |
| `2s`    | 2 seconds          | 100    |
| `500ms` | 500 milliseconds   | 25     |

Fractional results are rounded to the nearest frame. Negative offsets are
rejected.

Within a script, actions are sorted by offset, so they need not be written in
order. Two actions sharing the same offset fire in the order written, and a
warning is logged (unless logging is suppressed).

## How offsets interact with blocking verbs

Two kinds of action block the timeline: `wait-boot` and keyboard injection
(`type`, `key`). While one is in progress, the clock that offsets are measured
against is held, and when it completes the clock is *rebased* to that moment.
Every offset after a blocking action is therefore measured from the moment that
action finished, not from power-on.

This is what makes a script portable across models with different boot times.
The following waits for boot on any model, then acts ten frames later --
whether the machine booted in 1.7 s (48K) or 2.7 s (+3):

```
wait-boot
10   shot   booted
20   quit
```

`wait-boot` also acts as a sort barrier: offsets before it are absolute from
power-on, offsets after it are relative to boot-ready, and the two groups are
sorted independently. Multiple `wait-boot` lines create multiple such
partitions.

## Verbs

### Media and state

| Verb | Arguments | Description |
|------|-----------|-------------|
| `snapshot` | `<path> [format]` | Load a snapshot (`.sna`, `.z80`, `.zxs`). Format defaults to auto-detection. |
| `snapshot-save` | `<path> [format]` | Save the current machine state to a snapshot. Format defaults to the path extension, or `zxs`. |
| `bin` | `<path> [loadAddr] [startAddr]` | Load a raw binary blob into memory. `loadAddr` defaults to `0x8000`; `startAddr` defaults to the load address. Addresses accept `0x` hex or decimal. |
| `scr` | `<path>` | Load a raw `.scr` screen dump onto the display. |
| `tape-play` | none | Start tape playback (a tape must be loaded). |
| `tape-stop` | none | Stop tape playback. |
| `reset` | none | Reset the CPU. |

### Control

| Verb | Arguments | Description |
|------|-----------|-------------|
| `wait-boot` | none (lone token) | Block until the current model has finished booting, then rebase the timeline. See above. |
| `quit` | none | End the run. In headless this stops the frame loop early; in the windowed build it closes the window. |

### Screenshots

| Verb | Arguments | Description |
|------|-----------|-------------|
| `shot` | `[name]` | Capture the 256x192 display to a PNG. With a name, that filename is used (a `.png` extension is appended if absent). Without one, an auto-generated name is used. |

When a script is driving the run, the front-end's own periodic screenshot
mechanism (`-shot-interval` and final-frame capture in headless) is disabled,
so the script's `shot` actions are the only source of screenshots.

### Keyboard injection

A Spectrum reads its keyboard once per frame, so a scripted keypress is held
across several frames for the ROM to register it, with an idle gap before the
next key. The defaults are a 5-frame hold and a 3-frame gap (about 0.16 s per
character). `type` and `key` block the timeline until the keys have been
delivered.

| Verb | Arguments | Description |
|------|-----------|-------------|
| `type` | `<text>` | Type a string. Quote it to preserve spaces (`type "10 PRINT"`). Each character is mapped to its matrix key, with SYMBOL SHIFT applied for symbols. Untypeable characters are skipped with a warning. |
| `key` | `<name>` | Press a single named key or chord: `enter`, `space`, `delete`, `edit`, `break`, `up`, `down`, `left`, `right`, `caps`, `sym`. |
| `press` | `<name>` | Hold a key down without releasing it (for held-key scenarios such as games). Does not block the timeline. |
| `release` | `<name>` or `all` | Release a previously held key, or `all` to clear the matrix. |

Notes and current limitations:

- Letters are typed case-insensitively via their unshifted key. Explicit
  CAPS SHIFT upper-case is not yet applied.
- On the 48K in keyword (`K`) cursor mode, letter keys produce BASIC keyword
  tokens rather than individual letters, so `type PRINT` will not spell the
  word out letter by letter. This matters for entering BASIC keywords; direct
  data entry (digits, already-tokenised input) is unaffected.

### Screen text recognition

ZenZX can read characters off the display the same way Sinclair BASIC's
`SCREEN$` does: by matching each 8x8 character cell against the ROM font. This
is a pure read and disturbs no machine state. Only the standard ROM font is
recognised -- programs that redefine the character set or draw text with custom
routines or UDGs will not be read correctly.

| Verb | Arguments | Description |
|------|-----------|-------------|
| `dump-screen` | `[row]` | Print recognised text. With a row (0-23), prints that row; without, prints all non-blank rows. |
| `expect-screen` | `<row> <text>` | Assert that a row contains the given text (case-sensitive substring) *at this instant*. Pass/fail is logged; failures are counted and reported, but do not abort the run. |
| `wait-screen` | `<row> <text> [timeout=<frames\|Ns\|Nms>]` | Block the timeline until the row contains the text, then rebase. Times out (default 5 s) with a counted failure if the text never appears. Acts as a timeline barrier like `wait-boot`. |
| `wait-attr` | `<rowspec> <colourspec> [count=N] [timeout=...]` | Block until at least `count` cells (default 1) in the region match the colour predicate, then rebase. Recognises screens by colour when their characters use a custom font the glyph matcher cannot read. |

`wait-attr` matches on attributes rather than glyphs. The `rowspec` is a single
row (`10`), an inclusive range (`8-12`), or `all`. The `colourspec` is one or
more of `ink=COLOUR`, `paper=COLOUR`, `bright`, or `bright=0|1`, where `COLOUR`
is a name (`black blue red magenta green cyan yellow white`) or `0`-`7`.
Unspecified fields are wildcards. For example, waiting for a 128K-style menu's
bright rainbow band, or a highlighted item drawn in cyan:

```
wait-attr 7    bright count=8       timeout=10s
wait-attr 0-14 paper=cyan count=1   timeout=10s
```

This is the tool for driving game menus whose text cannot be recognised: many
menus are identifiable by their colour layout alone.

`expect-screen` checks the screen at a fixed offset, which is fragile for
content whose timing varies (such as tape-load messages). `wait-screen`
synchronises on the content itself, so a script need not guess when a message
will appear -- it waits for it, robust to load speed, tape mode, or model. For
example, detecting a tape's load headers:

```
wait-boot
10   type   j
30   type   ""
60   key    enter
80   tape-play
0    wait-screen 1 Program: myprog  timeout=20s
0    wait-screen 1 Bytes: mycode    timeout=20s
10   quit
```

Example -- boot, then verify the 48K copyright line is present:

```
wait-boot
5    expect-screen 23 Sinclair
10   quit
```

## Examples

Boot a 48K, type a line of digits, capture it:

```
wait-boot
5    type   1234
15   shot   typed
25   quit
```

Load a tape and let it run, screenshotting along the way:

```
0     tape-play
2s    shot    loading
20s   shot    loaded
```

Save a snapshot once a game has reached its title screen:

```
wait-boot
300   snapshot-save   title.z80
310   quit
```

## Recipes: probing machine state

These patterns came out of using scripts to read post-boot system state (for
example, finding where the +3 BASIC program area lands) rather than just driving
the display. They are reliable because they let the ROM finish its own
initialisation before anything is read or patched.

### Read a system variable after boot

The robust way to inspect a system variable (or any RAM) is to let the machine
boot fully, save a snapshot, then read the bytes out of the snapshot on the host.
`wait-boot` is the key: offsets after it are relative to boot-ready, so the ROM
has already set up the system variables by the time the snapshot is taken.

```
wait-boot
0   snapshot-save   state.sna   sna
1   quit
```

Run it and parse the snapshot. In a standard `.sna`, RAM at `0x4000` begins at
file offset 27, so a system variable at address `A` (in the `0x4000`-`0x7FFF`
range, which is RAM page 5) is at snapshot offset `27 + (A - 0x4000)`, stored
little-endian. For example, PROG lives at `0x5C53`:

```
offset = 27 + (0x5C53 - 0x4000)   # = 27 + 0x1C53
prog   = snap[offset] | (snap[offset+1] << 8)
```

Cross-check against a couple of neighbouring variables to confirm the machine is
in the state you expect: on a freshly booted machine with no BASIC program
entered, `VARS` (`0x5C4B`) equals `PROG`, because the variables area sits
immediately after a zero-length program.

### Patch RAM after boot without redirecting execution

`bin` takes an optional start address. Passing `-1` loads the blob into memory but
leaves the program counter untouched, so the running system continues normally
with the patched bytes in place. This is the way to inject a probe, a patch, or
fixture data into an already-booted machine rather than jumping straight into a
blob from a cold start (where the ROM has not initialised the system variables
yet).

```
wait-boot
0   bin   patch.bin   0x8000   -1
```

Without `wait-boot`, a `bin` that *does* set a start address runs your code before
the ROM has set up the system, which is rarely what you want when the code expects
a normal machine environment.

## Validation

Unknown verbs are a hard error: the script is refused and the offending line is
named, rather than silently skipped. This catches typos early. The keyboard
verbs and other future actions are part of the known vocabulary, so a script
using them loads even where a particular action is not yet available.
