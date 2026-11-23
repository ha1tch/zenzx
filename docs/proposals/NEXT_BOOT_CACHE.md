# A Next that boots once

Updated: 2026-11-23

> **Reconciliation, 2026-11-23 (0.50.0).** This was built, as T-77, and the register item is closed.
> What shipped, against what is proposed below, which is left as it was written:
>
> - **The state format** is not a hand-listed field format. `pkg/machinestate` walks the machine's
>   structs by reflection, with one list (`nextStatePolicy`, `nextstate.go`) of what is the host's
>   and not the machine's (windows, audio devices, files, callbacks, the settle watcher); a test
>   fails when a field is added that is neither saved nor on it. A hash of the layout is in the file
>   and in the key, so a change to the machine's structs makes a cache out of date and never misread.
> - **The key** is as section 4 has it, with the digest defined as there, and has the layout
>   fingerprint besides; the patch list is empty until T-76 is built. The settled cache's key adds a
>   second digest, over the firmware files and those of `nextcard.BootExtras` the card has, so
>   that a changed welcome guide or Browser setting is not shown from a cache made before it
>   (the hand-over cache is taken before NextZXOS reads any of them and does not have it).
> - **The settled state** is detected by `nextsettle.go`: the display steady and the SD card idle for
>   a number of frames, at frame 555 on the 26.11 firmware (the welcome guide); the hand-over is at
>   frame 428. A chain on a scratch card never uses or makes a settled cache.
> - **The changed card, the question of section 3**, was measured and the answer is that it does not
>   matter: with files and folders added to the card after the cache was made, a machine started from
>   the settled cache shows the same pictures as a cold one at rest, at the menu, in the Browser and
>   inside a folder, and a NextBASIC `SAVE` from it lands beside the added files, none overwritten, on
>   a card whose free-space hint is not the one the cache was made on. NextZXOS reads directories
>   and the free space when it needs them, not at start. The two machines still differ in about ten
>   fields of memory (the CPU's cycle counter and R, one page of screen RAM, two bytes of the DivMMC's
>   SRAM, the SD controller's program pointer, the border history). The pictures and the SAVE agree;
>   those fields have not been analysed one by one, and this is left open.
> - **Not done:** `-next-firmware check` and the caches are for the boot chain's card as it is; a
>   card of another firmware simply has caches of its own, kept until `cleanup`. The real-time clock
>   is not an open question for the cache: on the Next it is a battery-backed chip behind the I2C
>   ports (0x103B and 0x113B), not memory, and ZenZX does not emulate it, so the host's time is
>   nowhere in the machine's state and a restored machine cannot show a stale one. The clock driver
>   (`RTC.SYS`) is one of the start-up files in the key. If an I2C clock is added, its registers
>   are state like the rest, and the question is then whether it follows the host's time while the
>   machine runs, which would be a field the policy skips.
> - **Flags:** `-next-boot-cache auto|off|refresh` (the window's default is `auto`, the headless
>   build's `off`), and `-next-firmware check` reports each stage's cache and what differs;
>   `cleanup` and `setup` delete the caches that no firmware here can use (the study said `setup`
>   would clear them all; pruning keeps the valid ones, which a download that changed nothing
>   leaves).

A study. Nothing here is built. It is item T-77 of [the register](../TRACKING.md). The Next's memory
and how it is loaded are described in [NEXT_ROM_OVERLAY.md](NEXT_ROM_OVERLAY.md), section 1.

## The idea

The first time a Next boots, ZenZX notices that NextZXOS is up and has settled, and keeps the state
of the whole machine. From then on the machine does not boot: it starts from that state, as if the
state were a ROM. The boot ROM, the firmware and NextZXOS's start-up are done once.

On the real firmware the start to the NextZXOS menu is 760 frames: 15 s at the real speed (2.7 s at
full speed, as the headless build runs). The GUI is where it is felt.

## What it takes

### 1. Knowing the machine has booted and is stable

Nothing is cached before it is true, and a state cached half-way is worse than none. Three things,
all of which have to hold:

- **NextZXOS is waiting for a key.** `nextAtIdlePrompt` already reports a Next at its menu or
  Browser by the key-wait loop it spins in, found by signature; it gates the drop dialog today.
- **The screen has stopped changing.** The G-04 boot test already asks for this (`checkScreenIsSteady`:
  nearly all of the display the same 25 frames on). That is test code; the check in the program is
  the same comparison made on the display memory, over some frames, with a limit so that a machine
  that never settles caches nothing.
- **Nothing is in flight.** No DMA, no SD command, no tape playing, and the card's controller at
  rest.

It happens once: the first time the three hold after a boot that came from the firmware, and the
state is written then. It is not taken again while a valid cache exists.

Which state that is depends on the card: NextZXOS starts at its welcome guide on the card G-04 uses,
and at the menu after a key. The first idle state it reaches is what is cached. Whether
`nextAtIdlePrompt` holds at the welcome guide as it does at the menu has not been checked.

### 2. A state format for the Next, new

The snapshots (`.zxs`, `.sna`, `.z80`) are for the 48K and 128K models and hold nothing of the
Next, so this is a format of its own. It holds:

- the CPU (registers, Z80N state, interrupt state) and the frame and cycle counters;
- memory: banks 0-7, the 224 RAM pages (sparse: a page never written is absent, as `nextExtraRAM`
  is allocated), `nextSlots`, the legacy paging ports, `nextAltROM`, the Multiface, `nextCfg`, and
  the 2 MiB above for an issue 5;
- the NextREG file (256 values) and what hangs off it: Layer 2, the copper, sprites, palettes, DMA,
  interrupts, the CPU speed;
- the video state the display is drawn from, and the AY;
- the DivMMC (paged, control, entry points, `sram`).

It is compressed (`compress/flate`) and carries a version. The card is not in it.

### 3. The card

The cache is the machine as the *firmware* made it, so what identifies the card to the cache is the
firmware on it: the SHA-256 of the files the Next boots from (`nextcard.BootFiles`), taken when the
cache is made and kept in its key, and taken again at each start. If the digests differ the cache is
invalid and the machine boots from the firmware. The rest of the card is the user's, a disk the
machine reads and writes, and a change to it does not touch the cache: ZenZX copying a tape to the
card, NextZXOS saving a file, another tool adding one, none of them costs a boot. The firmware is a
few megabytes, so the hash is quick at any card size, and the whole image is never read for it.

What that leaves open is the settled state. At the menu NextZXOS may hold the card's FAT, directories
or buffers in RAM, and a state restored over a card that has changed since could show a listing that
is out of date, and a write from it could be made against a free-cluster count that is no longer true.
That is a property of the state, not of the key, and it is measured, not argued: restore the settled
state over a card with a file added, removed and rewritten, and look at the listing, then save a file
and check the card with a FAT check. If NextZXOS does hold such state, the settled cache is not
usable over a changed card, and the cache taken at the hand-over (frame 420-450, before NextZXOS has
read the card) is, which is the first stage in any case (section "Size and order").

### 4. The key

A cache is used only if it matches: the boot ROM, the DivMMC ROM, the firmware on the card, as one
SHA-256 over its files (`TBBLUE.FW` and the others `nextcard.BootFiles` lists, `config.ini` and
`menu.def` among them; section 3), the board issue, the patch list of
[NEXT_ROM_OVERLAY.md](NEXT_ROM_OVERLAY.md) and the ZenZX version, since a change in the emulation
changes the state. Anything else, or a file that does not read back, and the machine boots from the
firmware. The files are in the per-user folder (`nextcard.DefaultDir`), one per firmware digest, never on the
card and never in the tree: they hold the firmware's and NextZXOS's own contents, which are not
ZenZX's to ship.

**The firmware digest is defined, so that it comes out the same however the files are met.** The
order of `nextcard.BootFiles`, the order of a directory on the card, and the order a walk happens to
visit them in must not reach the result. The definition:

1. Each file is named by its path on the card, from the root, with `/` as the separator and the
   letters upper-cased, because FAT does not tell `Tbblue.fw` from `TBBLUE.FW`.
2. Each file gives a record: the length of the path as 4 bytes, big-endian; the path; the size of the
   file as 8 bytes, big-endian; and the 32 bytes of the SHA-256 of its contents. Lengths come first so
   that no two different sets of files can be written as the same run of bytes.
3. The records are sorted by path, as raw bytes, and are joined.
4. The digest is the SHA-256 of a fixed tag, `zenzx-next-firmware-v1`, followed by the joined
   records. The tag is the version of this definition: changing the definition changes the tag, and a
   cache made under the old one is then invalid, not misread.

A file in the list that the card lacks is an error and no cache is made or used; it is never skipped,
because a digest over fewer files would match no card and some card by accident. The per-file digests
are kept in the cache beside the combined one, so that `-next-firmware check` can name the file that
differs instead of saying only that the digest does.

### 5. Starting, and starting cold

Restoring replaces `attachNextBootChain`'s hand-off to the boot ROM: the chain is attached as usual
(boot ROM, DivMMC, card), the state goes in over it, and the machine runs from where it was. A log
line says whether it started from a cache or from the firmware.

`-next-boot-cache auto|off|refresh`: `auto` in the GUI; `off` in the headless build, so that G-04
and every test of the real firmware keep running the firmware; `refresh` boots cold and makes a new
cache. A menu item clears it. A hard reset inside a session (NextREG 0x02, bit 1) is still a real
one: the firmware runs again.

**The checks are `-next-firmware`'s.** That switch already looks at the boot files: `check` says
whether a Next can start from them and whether they are as a download leaves them, `setup` builds
them, `cleanup` removes what an unfinished download left, and the window's first-start dialog runs
the same `firmwareJob`. Whether a cache is still good is one more thing it knows, and the
comparison with the key (section 4) is one function, beside those checks, that nothing else
duplicates:

| | What it does with the cache |
|---|---|
| start-up | calls the function before restoring; any difference and the machine boots from the firmware |
| `-next-firmware check` | says whether the cache is valid and, when it is not, which part of the key differs: a firmware file on the card, the ZenZX version, the patch list. A stale cache is a line, not an error: the exit status stays what it is now |
| `-next-firmware setup` | changes the boot files, so it clears every cache when it finishes, and the next start makes a new one |
| `-next-firmware cleanup` | also removes caches whose firmware files are gone, and any in the format of another version |

### 6. Tests

The strong one is a *difference*: boot cold, boot from the cache, and run both from the same
point; memory, registers and the screen must agree, bit for bit where the emulator is deterministic,
the clock's values excepted. A field the format missed shows there, not as a bug found weeks later.
The same run presses SPACE and requires the menu in the same number of frames. Also:

- the detector fires once, after the welcome guide has loaded and not before, and not at all on a
  machine that never settles;
- the digest is the same for the files listed in any order, from a card whose directory holds them in
  any order, and for paths written in any letter case; it differs when one byte of one file changes,
  when two files swap contents, when a file is renamed or moved, and when a file is missing (which is
  an error, not a digest);
- a changed firmware file on the card (one byte), a changed ZenZX version and a corrupt cache file
  each fall back to the firmware;
- the other way, a card with other files added, removed or rewritten still uses the cache, shows
  its real listing and survives a save, checked with a FAT check (section 3);
- nothing is written when the boot did not complete or the card was written during it;
- the patch list changes the key;
- `-next-firmware check` names the part of the key that differs, for each part in turn, and keeps its
  exit status; `setup` leaves no cache; `cleanup` removes only the caches it should.

## Size and order

This is the larger of the two studies: a new format across the CPU, memory, the NextREG file, the
DivMMC, the SD controller, video and the AY, each of which has to be written out and read back
exactly, and the cost of a missed field is a machine that is subtly wrong only after a restore.
The order that keeps that visible: the format and the cold-versus-restored test first, taken at the
hand-over (frame 420-450), where the state is smallest and the card is untouched; then the settled
state of section 1, with the changed-card test of section 3, which is the part that makes it a boot that happens once.

## Open

How large the file is (the RAM pages that matter are few, not measured); whether the SD controller
is at rest at the settled state on every card or only this one; whether NextZXOS holds the card's structures in RAM at the settled state
(section 3); whether the machine reads a real-time
clock while booting, which would put its value in the state (and a stale time on the screen); whether
`nextAtIdlePrompt` holds at the welcome guide.
