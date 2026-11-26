# ROMs in ZenZX

Where a machine's ROM comes from, how to replace it, and how to make one. For the classic
models -- 48K, 128K, +2, +2A, +3, TS2068 and their Spanish variants -- everything here works as
written. The Spectrum Next is different, and has [a section of its own](#the-spectrum-next).

Every example below was run, against the headless build, before it was written down.

## The ROMs of each model

A ROM is a bank of 16 KiB (the TS2068's extension ROM is 8 KiB). A model has one to four banks,
numbered from 0, and each is a file in `rom/`:

| `-model` | Files | Banks |
|---|---|---|
| `48k` | `48.rom` | 0 |
| `128k` | `128-0.rom`, `128-1.rom` | 0-1 |
| `plus2` | `plus2-0.rom`, `plus2-1.rom` | 0-1 |
| `plus2a`, `plus3` | `plus3-0.rom` to `plus3-3.rom` | 0-3 |
| `spanish48k` | `48-spanish.rom` | 0 |
| `spanish128k` | `128-spanish-0.rom`, `128-spanish-1.rom` | 0-1 |
| `spanishplus2` | `plus2-spanish-0.rom`, `plus2-spanish-1.rom` | 0-1 |
| `spanishplus3` | `plus3-spanish-0.rom` to `plus3-spanish-3.rom` | 0-3 |
| `ts2068` | `ts2068-0.rom` (Home, 16 KiB), `ts2068-1.rom` (Extension, 8 KiB) | 0-1 |
| `next` | the `plus3` set, as a base | 0-3 (see [the Next](#the-spectrum-next)) |

The files are built into the program, so a release needs no `rom/` folder beside it. A file found
on disk wins over the built-in copy: the headless build looks in `-romdir` (`./rom` by default)
first, the GUI build in `./rom`, and both fall back to the built-in ones. The notices in
`rom/SINCLAIR.txt` and `rom/TIMEX.txt` say what may be done with the images.

## Replacing a ROM

The machine starts from its standard set, and what follows is laid over it, in this order:

| Order | What | Takes | If it goes wrong |
|---|---|---|---|
| 1 | `-model` | the standard set | no ROM found: an error, and the program stops |
| 2 | `-rom a.rom,b.rom` | banks 0, 1, ... in turn | a warning; that bank keeps the standard ROM |
| 3 | `-rom0` to `-rom3` | one bank each | the same |
| 4 | `-rom-bin FILE` | a file of any size, made into a ROM | an error and exit status 1, nothing changed |
| 5 | `-custom-roms-menu` | a ROM picked from `custom-roms/`, for a bank | a message; the ROM stays |

Each file given to `-rom` or `-romN` must be exactly the size of its bank: 16384 bytes, or 8192
for the TS2068's bank 1. Only the banks the model has can be named: bank 0 on a 48K, 0-1 on a
128K, +2 or TS2068, 0-3 on a +2A or +3.

In the GUI the Custom ROM menu does the same live, for any `.rom` in `custom-roms/` and in its subfolders, two levels down
(each folder is a submenu, and what does not fit the window goes under **More...**; the selector of
`-custom-roms-menu` lists them as `folder/x.rom`): it asks for
the bank (on a 48K there is only one, and it does not ask), loads the file and resets the machine. A model chosen from the Machine menu loads that
model's standard set, and what was laid over the old one does not go with it. A reset does not
reload the ROM: a ROM laid over stays until the model changes or ZenZX is closed.

`custom-roms/` has the Microdigital TK90X and TK95 ROMs in a folder, `Microdigital`, and the zenpoint swap
ROMs in another, `Zenpoint Project`; its `README.md` says which is which.

## Making a ROM from a file

A patch, a boot loader or a test program is a few bytes, and a ROM is 16 KiB. `-rom-bin FILE`
makes the ROM from the file: it goes at `-rom-at` (0x0000 unless said; `0x1234`, `$1234` or
decimal) in the bank `-rom-bank` names (0 unless said), and what is around it is zeroes or, with
`-rom-fill rom`, the bank's own ROM, so that the file is an overlay on it. A file that would end
past the end of the ROM is cut at the last byte, with a warning. `-rom-save OUT` also writes the
ROM to a file, to load later with `-rom`. The flags are in
[the manual](MANUAL.md#making-a-rom-from-a-binary-file), and `scripts/mkrom.py` makes the same ROM
outside ZenZX, byte for byte.

### A program as the ROM

Sixteen bytes of Z80 that paint every attribute cell red and stop. `printf` takes octal here, which
works in every shell, macOS's included:

```
printf '\363\041\000\130\021\001\130\001\377\002\066\022\355\260\030\376' > red.bin
./zenzx-headless -rom-bin red.bin -frames 5 -shot-dir shots
```

The screenshot in `shots/` is red from corner to corner. The program is at 0x0000, so it is the
first thing the machine runs; `DI`, `LD HL,5800h`, `LD DE,5801h`, `LD BC,02FFh`, `LD (HL),12h`,
`LDIR`, `JR $`.

### A patch over the real ROM

The 48K ROM prints its start-up message from 0x153B. A string in that ROM ends at the character
that has bit 7 set, so a shorter message ends where its last character does (`d` is 0x64, and
0xE4 with the bit, octal `\344`):

```
printf '1982 ZenZX patche\344' > msg.bin
printf 'wait-boot\n5 expect-screen 23 ZenZX patched\n10 quit\n' > patched.zen
./zenzx-headless -frames 600 -rom-bin msg.bin -rom-at 0x153B -rom-fill rom -script patched.zen
```

The script's line says `PASS`; without the three `-rom-` flags the same script says `FAILED` and
shows the real message, so the check is one that can fail.

### Keeping it

```
./zenzx-headless -rom-bin msg.bin -rom-at 0x153B -rom-fill rom -rom-save patched.rom -frames 1
./zenzx -rom patched.rom
```

`-frames 1` stops the headless build after the first frame, so the run only makes the file. The
file is a whole 16 KiB ROM, as any other, and goes in `custom-roms/` if it is to be picked from the
menu. With Python 3 the script does it without ZenZX:
`python3 scripts/mkrom.py msg.bin -a 0x153B -f rom -o patched.rom`.

## The Spectrum Next

The classic models have ROM chips. The Next has none: its "ROM" is the first 256 KiB of the
board's SRAM -- sixteen banks of 16 KiB -- which the firmware fills from files on the SD card
when the machine starts. ZenZX models it as the firmware does, and that decides what the switches
above can do:

| The Next is started | What its ROM is | The ROM switches |
|---|---|---|
| without the boot files, or with `-next-classic` | the `plus3` ROMs, four banks | all of them work, as on a +3 |
| from the card, as NextZXOS | what the firmware loads into those banks | applied when the firmware has loaded them, and again after a hard reset |

The second row is measured, on the real firmware: by about 450 frames into the start the firmware
has loaded its ROMs into the four banks, and the `plus3` ROMs there, and anything laid over them
at start-up, are gone. So on such a Next ZenZX keeps `-rom`, `-romN` and `-rom-bin` as a list of
patches on the machine and writes them when the firmware leaves config mode, over what it loaded,
and again after every hard reset (the Machine menu's reset, F1, the debugger's), which runs the
firmware again. A warning at the start says so. `-rom-fill rom`, `-rom-save` and the warning for
a file that does not fit work from the ROMs the firmware loaded, so they happen at that moment,
about 9 seconds into the run at real speed, and not at the start. A Next's boot cache is made
for one list of patches: another list is another cache.

On a Next `-rom-bin` reaches the whole 256 KiB. `-rom-bank` takes 0-7 (0-3 are NextZXOS's ROMs,
4 the DivMMC's ROM, 5 the Multiface, 6 and 7 the alternate ROMs) and `-rom-at` counts from the
start of that bank, so a file can run on into the banks after it; the banks it touches are
built whole, and an address past the 256 KiB is refused. The Diagnostics report (Snapshot > Run Diagnostics, or F3) says which ROM the CPU is
reading and why: the lock bits, NextREG 0x8C, the ROM the ports choose and the two lowest MMU
slots, so that a patch to the wrong bank can be told from a patch that does nothing. Whether
NextZXOS rewrites the alternate ROMs when the machine's mode is changed has not been looked at.
The design is in [`proposals/NEXT_ROM_OVERLAY.md`](proposals/NEXT_ROM_OVERLAY.md) (item T-76 of
[the register](RESOLVED.md)).
