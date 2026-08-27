# Timex SCLD extended video modes

Reference notes for `-ns-graphics mode-timex-NNN-*`. The Timex Sinclair
2068 / Timex Computer 2048 replaced the standard Spectrum ULA with the
Timex SCLD ("Standard Cell Logic Device"), which does everything the ULA
does plus three additional video modes. All three modes use a *second*
6912-byte screen area ("screen 1") alongside the standard one ("screen
0"): screen 0 occupies the usual `$4000`–`$5AFF` display file, screen 1
is a second, identically-shaped 6912-byte block. On real hardware screen
1 lives behind the SCLD's own DOCK/EX-ROM bank switching, not in the
linear RAM map a 48K/128K Spectrum uses — see "Memory layout" below.

## Mode select: port 0xFF

Bits 0–2 of port `0xFF` choose the active mode:

| Bits 0–2 | Mode |
|---|---|
| `000` | Screen 0 (standard) |
| `001` | Screen 1 (a second standard screen, used alone) |
| `010` | Hi-colour (Extended Colour Mode) |
| `110` | Hi-res |

Bits 3–5 select the hi-res ink/paper palette (see below). Bit 6 disables
the SCLD's own timer interrupt. Bit 7 selects DOCK vs EX-ROM bank paging
— unrelated to video, sharing the register with everything above; any
read-modify-write of port `0xFF` for a graphics mode change must
preserve the existing paging bits rather than clobbering them.

Source: [z88dk/z88dk#1069](https://github.com/z88dk/z88dk/issues/1069),
corroborated by the [Sinclair Wiki](https://sinclair.wiki.zxnet.co.uk/wiki/Timex_2000_series),
the ZX-Uno project's manual (an FPGA reimplementation; see "ZX-Uno
corroboration" below), and now the original 1983 Timex Computer
Corporation / Sinclair Research **T/S 2068 User Manual** (Appendix C,
"Display Modes"), which gives `OUT 255,62` for full-width/hi-res white-
on-black and an 8-entry palette table for bits 3-5 that matches the
table above exactly, entry for entry, once converted from decimal to
binary. Four independent sources, including the manufacturer's own
documentation, agreeing bit-for-bit is about as settled as a spec gets
without dumping a real machine's ROM.

## Screen 0 / Screen 1 (modes `000` / `001`)

Each is a complete, standard Spectrum screen (6144-byte bitmap + 768-byte
attribute block, normal 8×8 attribute-clash rules). Selecting one or the
other just changes which of the two the SCLD reads for output — no new
rendering logic needed beyond pointing the existing renderer at a
different base address.

"Dual screen" is not a fourth hardware mode: it is software toggling
`000`/`001` every frame, so persistence of vision blends two independent
full-colour images into more apparent simultaneous colours. Implementing
it is a driver-level frame-parity toggle over the two modes above, not a
new renderer.

## Hi-colour / Extended Colour Mode (mode `010`) — `mode-timex-001-hicolour`

**256×192 pixels, up to 15 colours, colour resolution 32×192** (Wikipedia:
[Timex Sinclair 2068](https://en.wikipedia.org/wiki/Timex_Sinclair_2068),
[Timex Computer 2048](https://en.wikipedia.org/wiki/Timex_Computer_2048)).
Where the standard screen gives one ink/paper attribute pair per 8×8
pixel block (32×24 resolution), hi-colour gives one pair per **8×1**
pixel row — an 8x improvement in vertical attribute-clash granularity,
though clash within a single pixel row is still possible.

Mechanism, per the z88dk issue above (bracketed terminology aligned to
this document):

- **Screen 0's bitmap area** (the 6144-byte pixel data at `$4000`,
  standard non-linear Spectrum addressing) supplies the pixel bits, same
  as a normal screen.
- **Screen 1's bitmap area** (the 6144-byte block at the equivalent
  offset in screen 1, addressed with the *same* non-linear scheme as a
  bitmap — not the standard linear 32×24 attribute layout) supplies the
  attribute byte for each 8×1 strip. Each byte uses the ordinary
  Spectrum attribute format (FLASH, BRIGHT, PAPER×3, INK×3).
- Neither screen's own 768-byte attribute block is used. A hi-colour
  screen dump is therefore exactly 12,288 bytes — two 6144-byte planes,
  no attribute blocks — which matches the Timex hi-colour `.SCR`/`.MLT`
  format documented on the
  [Sinclair Wiki's ULAplus page](https://sinclair.wiki.zxnet.co.uk/wiki/ULAplus).

This is now corroborated by four independent sources, the last of which
is the original manufacturer's own documentation, not a clone's account
of it: Wikipedia's resolution figures; the z88dk issue's mechanism
description; the ZX-Uno project's manual (see below); and the **T/S 2068
User Manual itself** (Chapter 17 and Appendix C). The manual's own words,
lower-level than anything else cited here:

> "In Extended Color Mode, the organization of attributes (which reside
> in memory starting at 6000H) is the same as the organization of pixel
> data."

This is the manufacturer stating, in Appendix C's "Attribute Data
Organization" section, precisely the mechanism this document describes:
Screen 1's bitmap-shaped region holds the attributes, addressed the same
way as pixel data. Chapter 17 gives the same fact in plainer language:
"in Extended Color Mode, up to eight choices of color for each character
position — each row of 1 x 8 pixels can have a different color." Treat
the mechanism as verified against primary documentation, with the
residual (unavoidable) caveat that this is the manufacturer's *written
spec*, not a byte-level silicon trace or a real machine's captured
screen dump.

### ZX-Uno corroboration

The [ZX-Uno](http://www.zxuno.com/wiki/index.php/ZX_Spectrum) is an
FPGA-based Spectrum/Timex clone whose ZX Spectrum core implements these
same three modes (calling this one "HiColour"). Its manual states the
port `0xFF` bit layout identically to the table above, down to the same
eight-entry hi-res palette, and gives the exact activation values: `OUT
255, 2` for hi-colour, `OUT 252, 6` for hi-res (black on white). It also
confirms the hi-colour mechanism independently, in its own words: Screen
0's pixel data is used as-is, and Screen 1 supplies "attributes... placed
exactly like the screen pixels are defined, first row first, then 8th
row, etc." — i.e. the same non-linear addressing as the bitmap, matching
this document's description exactly.

### Original manual corroboration

The **T/S 2068 User Manual** (Timex Computer Corporation / Sinclair
Research, 1983 — the actual manufacturer documentation for this
hardware) confirms the same port `0xFF` values independently: `OUT
255,0` for screen 0, `OUT 255,1` for "dual screen mode" (screen 1),
`OUT 255,2` for Extended Color Mode, and the full-width/hi-res palette
table matching the one above exactly (see "Mode select: port 0xFF"
above). Its Appendix C additionally gives the memory addresses this
document already cites (`4000-57FF`/`5800-5AFF` for screen 0,
`6000-77FF`/`7800-7AFF` for screen 1) and states outright that Extended
Color Mode's attributes, at `6000H`, use "the same organization" as
pixel data — the exact claim this document makes about non-linear
addressing, from the people who built the machine.

## Hi-res (mode `110`)

**512×192 pixels, 2 colours (monochrome).** Columns are taken alternately
from screen 0's and screen 1's bitmap areas to double horizontal
resolution; the attribute areas are unused entirely. All colours
(including the border) are forced BRIGHT, and the border takes the PAPER
colour. The ink/paper pair comes from bits 3–5 of port `0xFF`, one of
eight fixed combinations (black/white, blue/yellow, red/cyan,
magenta/green, and their BRIGHT-forced complements).

## ZX-Uno extensions (not genuine Timex hardware)

The ZX-Uno's core goes beyond the real SCLD, adding features with no
1983 hardware equivalent. Listed here for completeness (per session
request to have all Timex-adjacent documentation handy) and to keep
them clearly separated from what `mode-timex-001-hicolour` (or any
future genuinely-Timex-compatible mode) should implement:

- **Radastan mode**: a ZX-Uno-exclusive low-res mode, 128×96, 16 colours,
  no attribute clash — linear, one nibble per pixel (two pixels per
  byte, `[AAAABBBB]`), using only screen 0's 6144 bytes. Activated via
  ZX-Uno's own `RADASCTRL` register (`0x40`), not port `0xFF`. Not a
  Timex mode at all; would be its own `-ns-graphics` value if ever
  implemented (not currently planned).
- **ULAplus**: a 64-colour palette extension (from the wider Spectrum
  clone community, not Timex-specific), layered on top of any of the
  above modes via two additional ports. Out of scope here.
- **4-screen 128K paging**: the ZX-Uno combines its screen-0/screen-1
  paging with 128K bank-select bit 3 (port `0x7FFD`) to offer four
  screens total (`$4000`, `$6000`, `$C000`, `$E000`). The genuine TS2068
  predates the 128K Spectrum and has no such combination; this is a
  ZX-Uno-only extension.

## Memory layout — resolved, from the Extension ROM's own source

Two earlier drafts of this section were both wrong in opposite
directions, corrected here against the actual `CHG_VID`/`OPDFIL`/`CLDFIL`
Z80 source in the **T/S 2068 Technical Manual** (uploaded to this
session, ~400 pages, includes a full source listing of the Home and
Extension ROMs) rather than inference from OCR'd diagrams:

- **First draft:** claimed screen 1 needs DOCK/EX-ROM-style bank
  switching to be reachable. Wrong -- that bit pages expansion
  cartridge memory, unrelated to the built-in second screen, which is
  ordinary directly-addressable RAM at `6000H-77FFH`.
- **Second draft, over-corrected after that error was pointed out:**
  claimed nothing is relocated when a dual-screen mode is engaged, that
  the "before/after" memory maps were just two static illustrative
  snapshots. Also wrong -- there is a real, active relocation, performed
  by a documented ROM service, not the raw port write.

**What actually happens**, per the Technical Manual's §3.2.2.3 and the
disassembled `OPDFIL`/`CLDFIL` routines: writing to port `0xFF` directly
only changes the SCLD's video-generation register -- it relocates
nothing. Opening the second display file *properly* means calling the
Extension ROM's `CHNG_VID` service, which:

1. Checks free memory against `RAMTOP` and refuses with "Error 4, Out of
   Memory" if there isn't room.
2. Moves the User-Defined-Graphics table down to make space.
3. Moves 2112 bytes (`$0840`) of **OS-resident code and the machine
   stack** from `$6000` up to `$F7C0` (verified in the source: `LD
   DE,$F7C0 / LD HL,$6000 / LD BC,$0840 / LDIR`), then fixes up internal
   jump/call addresses in the moved code via a fix-up table.
4. Zeroes the newly-opened `$6000-$7AFF`.
5. Only *then* writes the mode to port `0xFF`, preserving bit 7 (EXROM
   select) via `AND $7F` / `AND $80` / `OR B`.
6. Updates internal ROM pointers (`ERRSP`, `LISTSP`, `MSTBOT`) by the
   same fixed `$97C0` offset.

`CLDFIL` reverses all of it when returning to mode 0.

**What does *not* move**, confirmed by what the source doesn't touch:
the fixed BASIC system-variable block (`5C00H`+, per Technical Manual
§3.3.1) and, importantly, **the user's own BASIC program and
variables**. The relocation protects the OS's own internal plumbing --
because that plumbing normally lives at `$6200`+, colliding with screen
1 -- not the user's code. Keeping a running program clear of
`6000H-77FFH` was never automatic; both manuals separately advise
setting `RAMTOP` low enough in advance.

**For zenzx**, this resolves cleanly rather than remaining open: zenzx
does not emulate the T/S 2068's Extension ROM, so there is no `CHNG_VID`
to call and no OS-resident code sitting at `$6000` needing protection in
the first place. `memory.go` already treats `6000H-77FFH` as ordinary
bank-5 RAM (confirmed by reading `Read`/`Write` directly -- no special
casing exists there, unlike the screen-0 range), which is exactly what
that address range *is* on real hardware outside of a video mode
reading it. `mode-timex-001-hicolour`'s renderer (`videorender_hicolour.go`)
therefore reads it straight through `mem.Read()`, with no new storage
and no relocation logic -- matching real hardware's actual behaviour,
not a simplification of it. The scope this settles on: hi-colour is a
static, host-selected rendering mode (no port `0xFF` I/O emulation, so
nothing a running guest program does can engage or disengage it) for
content authored with that in mind, not a promise that arbitrary
Sinclair BASIC programs can safely flip into it mid-run the way real
T/S 2068 software could.
