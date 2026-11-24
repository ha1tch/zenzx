# The headless build

No window and no sound: it runs a machine for a number of frames, optionally under a script, and writes screenshots. It is what the tests and the debugging of guest software use. Scripts are described in [`zenscript.md`](zenscript.md).

## The workflow

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

## Loading a raw binary directly

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

## Loading a screen dump

`-scr` loads a raw 6912-byte `.scr` screen dump straight onto the display
(6144 bytes of bitmap + 768 bytes of attributes). It paints the screen without
running anything, so it pairs naturally with `-frames=1` to render a screen
image to PNG:

```
./zenzx-headless -scr=expected.scr -frames=1 -shot-prefix=expected
```

This is useful for verifying screen output against reference `.scr` files.

## Flags

The headless program's flags, with their defaults, are in the manual: [Command-line reference -- headless](MANUAL.md#command-line-reference--headless). `./zenzx-headless -help` prints them all.

The joystick and the mouse are chosen as in the GUI; [Joystick and mouse](MANUAL.md#joystick-and-mouse) in the manual has the choices and what each model gets by default.

## Screenshots

A screenshot is a PNG of the picture as the machine has it: 256x192 for every model but the Next, without the border, in the Spectrum's own colours. A Next's is its whole frame, border included: 320x256, or 640x256 when a layer with twice the horizontal resolution is on.
