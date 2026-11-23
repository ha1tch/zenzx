# Hot patches over the Next's firmware

Updated: 2026-11-23

> **Reconciliation, 2026-11-23 (0.50.0): built as way A, with these differences from what is
> proposed below.** The patch list is `romPatchSet` on the machine (`rom_patches.go`); the event is
> the soft reset that follows the firmware's NextREG 0x03, found through a flag the write sets
> (`nextConfigLeft`) and not through a hook set from outside. Items 1, 2, 3 and 5 are as written.
> Item 4 is one rule and not two: the physical address is `-rom-bank` x 16 KiB + `-rom-at`, so
> `-rom-at` needs no separate form, and the base of `-rom-bin` is the banks the file touches,
> whole, and not the 256 KiB (a zero fill over all of it would clear NextZXOS's ROMs and the
> DivMMC's). Item 6 is a line in Show Status (`NextROMLive`), not yet in the debugger's views or
> the status bar. Item 7, the custom ROM menu on a running Next, is not touched. The boot cache
> key carries one entry per patch in the field it already had. Guard G-16 is the real-firmware
> test. Everything below the line is as it was written.

A study of what it would take for the ROM switches (`-rom`, `-rom0` to `-rom3`, `-rom-bin`, the
Custom ROM menu) to work over the ROMs a Next's firmware has loaded. Nothing here is built. It is
item T-76 of [the register](../TRACKING.md); the user-facing account of ROMs is
[ROMS.md](../ROMS.md), and a second study of the same moment, a cached boot, is
[NEXT_BOOT_CACHE.md](NEXT_BOOT_CACHE.md).

## 1. What is there

The Next has no ROM chips. Its "ROM" is the first 256 KiB of the board's SRAM, sixteen banks of
16 KiB, which the firmware (`TBBLUE.FW`, started by the boot ROM) fills from the SD card. ZenZX
models the same thing (`nextconfig.go`, `nextphys.go`), but not as one array:

| SRAM bank | What | Where ZenZX keeps it |
|---|---|---|
| 0-3 | the machine's ROM: NextZXOS's four | `SpectrumMemory.rom[0..3]`, the array the classic models use |
| 4 | the DivMMC ROM, lower 8 KiB | the DivMMC's own `bootROM`, from `enNxtmmc.rom` |
| 5 | the Multiface: ROM lower 8 KiB, RAM upper | `nextCfg.scratch[1]`, allocated on first use |
| 6-7 | alternate ROM 0 and 1 | `nextCfg.scratch[2..3]` |
| 8-15 | DivMMC RAM, sixteen 8 KiB pages | the DivMMC's `sram` |

Above 0x040000 is the RAM the MMU pages: 1792 KiB in 224 pages of 8 KiB (`nextTotalPages`), and,
on an issue 5 board, 2 MiB more that only a physical address reaches. `nextPhysRead` and
`nextPhysWrite` already address all sixteen banks by one 22-bit physical address, which is the
only form of "the 256 KiB" the code has, and is enough to read and write any of them.

The firmware fills banks in *config mode*: NextREG 0x04 names the bank, the CPU sees it at
0x0000-0x3FFF, writable, and the firmware reads each ROM file straight into it. It leaves config
mode with a write to NextREG 0x03 that chooses the machine, and starts the machine with a soft
reset, NextREG 0x02 (`nextConfigRegWrite`, `serviceNextReset`).

### What was measured

On the real firmware (the G-04 files), booting a Next from the card:

| Event | Frame | At 50 frames a second |
|---|---|---|
| the firmware leaves config mode (hand-over) | 420-450 | about 9 s |
| NextZXOS has loaded its welcome guide | 500 | 10 s |
| the NextZXOS main menu | 760 | 15 s |

Three more facts, each from a run:

1. **Start-up ROM overrides are lost.** With the four banks overridden before the run, every one
   of them was changed by the firmware load and none of the overridden bytes was left. This is the
   case for `-rom`, `-rom0` to `-rom3`, and `-rom-bin`: they work on a Next without boot files
   and are silently discarded with them.
2. **A patch written after the load takes effect.** Seven bytes written to bank 0 right after
   the hand-over (`DI; LD A,2; OUT (0FEh),A; JR $`) ran: the border became red, the CPU spun at 5.
3. **The firmware stage writes nothing to the card.** The 1 GiB image, opened for writing,
   hashed the same before and after the hand-over. The alternate ROMs hold data at the hand-over
   and did not change on the way to the menu: the firmware fills them, NextZXOS does not rewrite
   them on that path. (Whether it does when switching to 48K or 128K mode has not been looked at.)

### What the CPU sees at 0x0000-0x3FFF

A patch to the storage is seen only if it is what the CPU reads. In the order `Read` takes them:
the boot ROM while it is enabled; the Multiface; the DivMMC (its ROM over 0x0000-0x1FFF while the
automap holds it); the config window; the Layer 2 access port; then the MMU slots, where slots 0
and 1 holding 0xFF mean ROM, and NextREG 0x8C, the ROM lock and `romBank` choose between ROM and
alternate ROM (`nextROMRead`). Any other value there maps RAM over the ROM. A patch to bank 3 does
nothing while bank 0 is the one running, and a patch to the ROM does nothing while the DivMMC ROM
is paged over it.

## 2. Hot patches over the firmware's ROMs

**Aim:** the switches do on a Next booted from the card what they do on a +3: the file ends up in
the bank, over what the firmware loaded.

### Ways to do it

| | A. Apply when the firmware has loaded | B. Overlay consulted at read time | C. Leave it to the guest |
|---|---|---|---|
| How | keep a list of patches on the machine; write them into the banks when config mode is left | the same list, checked in `nextROMRead` (and the DivMMC's) on every ROM read | a Z80 program patches through DMA to a physical address, which `nextPhysWrite` already allows |
| Survives a hard reset | yes: config mode is left again, the list is applied again | yes | no |
| Survives NextZXOS rewriting the bank later | no | yes | no |
| Cost on the read path | none | a check on every ROM read | none |
| Host-side (flags, menu, debugger) | yes | yes | no |

**Recommendation: A.** The storage is one array per bank and the event is exact. B costs the hot
path something to cover a case, NextZXOS rewriting the alternate ROMs, not seen yet; it can be
added over A later without changing the patch list. C exists already and needs no work.

### What A needs

1. **A patch list on the machine**, outside the memory arrays: a bank or physical address and the
   bytes, or the file, address and fill the flags gave. It is applied by writing through
   `nextPhysWrite`, so banks 0-3, the DivMMC ROM (4), the Multiface (5) and the alternate ROMs
   (6-7) are reached with no new storage code.
2. **An event: "the ROMs are in place".** Raised where config mode is left
   (`nextConfigRegWrite`, NextREG 0x03 with machine type 1-4 while `nextCfg.active`), through a
   hook set from outside the way `divMMCReadFn` is. On a Next with no boot files there is no
   config mode, so the list is applied at start, as it is today.
3. **The build is deferred to the event.** `-rom-fill rom` is "what the bank holds", and what it
   holds after the load is the firmware's, so `Build` runs at the event, with the base read by
   `nextPhysRead`. `romimage.Build` takes the size from the base, so a base of the 256 KiB works as
   it is: a file may then be placed at a physical address, 0x000000-0x03FFFF, and run across banks.
   Warnings (a cut file) and `-rom-save` come at the event too, about 9 s into the run at the real
   speed, and the log line should say so at the start.
4. **Addressing on a Next.** `-rom-bank` takes 0-7 (the table above), or `-rom-at` is a physical
   address when no bank is given; the per-bank size rule becomes "the bank's size": 16 KiB, 8 KiB
   for bank 4.
5. **A warning, at once and before any of that:** on a Next with the boot chain, `-rom`, `-romN` and
   `-rom-bin` say, at start, that the firmware replaces the bank. Today they say nothing.
6. **A way to see which ROM is live**: the lock bits, NextREG 0x8C, `romBank` and the slots,
   in the debugger and the status line. Without it a patch to the wrong bank looks like a patch
   that does nothing.
7. **The custom ROM menu on a running Next** writes into the banks after the load and resets
   (`applyCustomROMLive`); it has not been tried, and a reset on a Next goes back through the boot
   ROM only on a hard one.

### With a cached boot

A cached boot ([NEXT_BOOT_CACHE.md](NEXT_BOOT_CACHE.md)) starts the machine long after the ROMs
were loaded, with code already running from them and copied out of them. A patch laid over a
restored machine would reach only what runs from the ROM from then on. So the patch list is part
of the cache's key: a different list is a different cache, made by a cold boot with the patches
applied at the event.

### Tests

Golden-free, as the rest of the Next's:

- **Unit:** drive the config-mode sequence by hand (NextREG 0x03 to 0x07, 0x04 to pick a bank,
  writes through the window, 0x03 to leave) with a patch list set: the patch is in its bank, the
  rest of the loaded bank is as it was, a second hard reset and exit applies it again, the file
  and fill are the ones asked, and a patch cut at the end of the 256 KiB warns.
- **G-04:** boot the real firmware with a patch over bank 0's start: the border program runs; with
  a patch that is the same bytes, the machine still reaches the NextZXOS menu and passes the checks
  the boot test makes (a menu on the screen, steady).
- **Python:** `TestAgreesWithMkromPy` over a 256 KiB base, if `mkrom.py` takes one.

### Open

Whether NextZXOS rewrites the alternate ROMs when the machine's mode is switched; whether patches
should be in the saved session (see [workbench.md](workbench.md)); what the list of patches is
called in a settings file.
