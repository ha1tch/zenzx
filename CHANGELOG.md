# Changelog

All notable changes to ZenZX are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.50.0] - 2026-11-21

A substantial minor release gathering the drawer, SD-card and floppy work, the
shipped Next demo card, the Microdigital and zenpoint custom ROMs, the menu
rework, and a documentation sweep.

### Added

- **Drop a file onto a storage slot.** Dragging a file over the window brings
  down a slot drawer (styled like the machine dock); dropping onto a slot copies
  the file into that slot's media -- a Next's SD card (slot 2) or the +3's disc,
  through its +3DOS filesystem. The drop goes to the focused machine.
- **The Next Browser refreshes without a keypress.** After a file is written to a
  live SD card, the host drives NextZXOS's own M-then-Y remount so the Browser
  re-reads and shows the file, located by a firmware-version-robust signature and
  targeted at the focused machine.
- **The release ships ZenZX's own Next demo card** (gallop, fuji, orbit) beside
  the program, with the boot files downloading beside it; `zenzx-headless
  -next-examples-card PATH` builds one on demand.
- **Custom ROMs in `custom-roms/`:** the Microdigital TK90X (standard, Brazilian,
  Uruguayan) and TK95 (Portuguese, Spanish), and our own zenpoint 48K swap ROMs
  (Spanish, English).

### Changed

- **The Machine menu groups every model under its maker** -- Sinclair Research
  Ltd, Amstrad plc, TBBlue / SpecNext Ltd, In Spanish, Timex -- all at one level,
  with the Spanish models brought up level with the rest.
- **The mount drawer follows the dock's rounded, floating look.**
- **A card write is seen after a remount**, new disc/card files carry valid FAT
  timestamps, and the debugger HUD's grip animation is half speed.
- **Documentation sweep:** the README compressed with captioned Next-example
  screenshots; `gui.md`/`MANUAL.md` document the slot drop; the stale `-noesc`
  flag corrected to `-escape` and `-z80n` documented; and the changelog
  consolidated, with older per-patch detail moved to
  [`docs/CHANGELOG-ARCHIVE.md`](docs/CHANGELOG-ARCHIVE.md).## [0.42.9] - 2026-11-21

### Added

- **The release ships ZenZX's own Next demo card.** Each release package now
  carries a `next/zenzx-card.img` beside the program -- a self-contained card
  holding the example programs (gallop, fuji, orbit) -- so a first Next launch
  boots NextZXOS with the examples already in its second slot, no `-next-examples`
  step. The card is built from the embedded programs with no boot files and no
  assembler; `zenzx-headless -next-examples-card PATH` writes one on demand, and
  the CI package step uses it. It adds about 370 KB to a package (the card is a
  sparse FAT32 image that compresses to almost nothing).

### Changed

- **The Next download prefers a `next` folder beside the program.** When one
  exists (as in a release package), the boot files download into it, beside the
  shipped demo card, so the card auto-inserts. The `-next-files` flag and the
  `NEXT_FILES` variable still take precedence, and the per-user folder remains the
  fallback.

## [0.42.8] - 2026-11-20

### Changed

- **Documentation sync and README tidy.** The README's surplus detail (the
  multi-machine/link specifics and the library-build specifics) moved to
  `docs/link.md` and `docs/development.md`, leaving the README a lean overview
  with pointers; the Next code-example screenshots (gallop, fuji, orbit) gained
  captions matching the games grids. `docs/gui.md` now documents dropping a file
  onto a storage slot -- the drawer, the per-slot routing to a card or disc, the
  focused-machine rule, and the refusal toast for an inert slot.

## [0.42.7] - 2026-11-20

### Changed

- **The mount drawer follows the dock's look.** The slot drawer that slides down
  on a drag now matches the dock: a rounded panel that floats off the top edge and
  lets the machine behind it show through (solid at the top edge, fading toward the
  machine), with rounded tiles inside, drawn with raycanvas's soft corners the same
  way the dock draws. It replaces the earlier flat, square-edged panel and tiles.

## [0.42.6] - 2026-11-20

### Added

- **Drop files onto a +3 floppy.** Dropping a file onto the "+3 Drive A" slot now
  writes it into the disc in the drive, the floppy counterpart of dropping onto an
  SD card. The file is added to the disc's +3DOS filesystem through plus3
  (`github.com/ha1tch/plus3`, its `diskimg` package), the image is saved and the
  drive reloaded from it, so the running +3 reads the new disc; the new file is
  seen on the next directory access (a CAT or a loader). As with the SD slots, the
  write goes to the focused machine's drive -- two +3 machines keep their own
  discs -- and the acknowledgement is the toast. Drive A only (the FDC models one
  drive). A drop is refused, with a toast, when the drive is empty.

## [0.42.5] - 2026-11-20

### Fixed

- **The host-driven Browser refresh now targets the focused machine.** With two
  or more Next machines running, a file dropped while the second was focused
  still refreshed the first machine's Browser (and landed on its card), because
  the mount drawer acted on the machine it was built with rather than the one in
  focus. The drawer now resolves the focused machine (the deck's front) at drop
  time and both writes the file and drives the remount against that machine, and
  it captures the target when the refresh is armed so a focus change mid-sequence
  cannot redirect it. A single machine is unaffected. Verified on real NextZXOS
  26.11 with two machines (dormant guard G-10).

## [0.42.4] - 2026-11-20

SD-card refresh in NextZXOS: a file copied to a card now shows up in the
Browser, and can be made to show up without the manual remount.

### Added

- **Host-driven Browser refresh, no keypress.** After a file is written to a
  live card, the host now drives NextZXOS's own Browser remount (its M-then-Y)
  so the Browser re-reads the directory and shows the new file without the user
  pressing anything. It works by feeding the firmware's own key sysvars
  (LAST_K `$5C08`, FLAGS `$5C3B` bit 5) the two keycodes its idle key-wait loop
  reads, so the firmware runs its real remount-and-rebuild path on its own
  thread. The loop is found by its byte signature (`76 21 3B 5C CB 6E` --
  HALT / LD HL,FLAGS / BIT 5,(HL)), the classic Spectrum key-wait idiom, not a
  fixed address, so it survives firmware versions that keep the idiom and
  declines rather than guesses where they do not. Driven across frames by the
  mount drawer so it never blocks the render loop. Verified on real NextZXOS
  26.11 (dormant guard G-09).

### Fixed

- **A card write is now seen by NextZXOS after a remount.** Writing files to a
  card (`AddFiles`) replaces the card image by renaming a new file over it,
  which orphaned the open handle the emulator held, so NextZXOS kept seeing the
  old contents even after its Browser remount. The card is now re-inserted after
  the write, reopening the handle, so a remount (manual or host-driven) re-reads
  the new image. Verified single-machine (G-07) and for two concurrent Next
  machines with independent card sets (G-08).
- **New files on a card carry valid FAT timestamps.** Directory entries written
  by `AddFiles` had zero (invalid) FAT dates; they now carry a real date/time,
  with parent-directory write times bumped. A correctness fix, independent of the
  refresh above.

### Changed

- **The debugger HUD's grip reveal/hide is half speed.** The gripper's
  show/hide ease was halved (step 0.12 -> 0.06) for a smoother, slower reveal.

## [0.42.3] - 2026-11-20

Fixes to the mount drawer's drag handling, a proportional resize for tiled
layouts, and font and layout polish.

### Added

- **Proportional window resize for tiled layouts.** A tiled layout specifies
  proportions, not a fixed size, so its window is now resizable: dragging an
  edge snaps the window back to the layout's own aspect
  (`(cols*RefWidth):(rows*RefHeight)` -- wide for columns, tall for a stack),
  with the dimension dragged furthest kept and the other derived. The tiles
  stretch to fill, the aspect is preserved through the floor and the monitor
  clamp, and the snap animates from where the window actually is. A
  single-machine view is unchanged -- it keeps its zoom steps.

### Fixed

- **A HUD resize no longer pops the mount drawer open.** The drawer armed on
  any button-drag over the window, which caught the debugger HUD's resize
  drag. It now stands down whenever another widget owns the pointer (a HUD
  resize or hover, an open HUD, menu, dock or overlay).
- **OS file drops now reach the mount drawer.** raylib reports an OS file drag
  only at the drop, never during it, so the drawer could not appear mid-drag
  and drops fell through to machine-load. Dropping is now drop-then-reveal:
  a drop of a non-machine file opens the drawer as confirmation and copies to
  the slot under the cursor (or the nearest writable slot), then auto-closes.
  Routing is by file type -- tapes, snapshots, discs, `.scr` and `.nex` load
  into the machine; anything else is copied to a card.
- **A dropped `.nex` is acknowledged.** NEX loading is not implemented yet
  (tracked), so a dropped `.nex` now reports that rather than silently doing
  nothing.
- **The mount drawer's slot legends no longer overlap.** The slot title is
  drawn at double size; the lines under it now advance past its real height
  instead of sitting on top of it.

### Changed

- **Sinclair Narrow digits 0 and 7** are nudged one column right so every
  digit shares the same right edge, lining up the disassembly's hex columns.

## [0.42.2] - 2026-11-20

Drag-and-drop mounting with on-screen acknowledgements, an authentic Sinclair
font set, and a new narrow face for the debugger HUD.

### Added

- **A mount drawer for drag-and-drop.** When a machine has storage mounted and
  a file is dragged in, a drawer slides down from the top showing the machine's
  storage slots as drop targets. Dropping a file onto an eligible slot writes
  it into that slot's filesystem (SD cards, via `pkg/nextcard`), with the
  capacity and read-only checks enforced; a read-only or over-capacity slot is
  inert, not a target. The `+3` drive is shown but its drops are deferred.
- **On-screen acknowledgements (OSD toasts).** A mount, or a file loaded into
  the machine (tape, disc, snapshot, `.scr`), now raises a prominent rising
  caption naming what happened and where -- files are named outright when few,
  counted when many -- rather than only printing to stdout.
- **`tools/mkfont`**, a reproducible BDF generator that extracts the ZX
  Spectrum character set from a ROM. It emits the 8x8 `sinclair.bdf` and, with
  `-narrow`, the new 6x8 `sinclair-narrow.bdf`.
- **Sinclair Narrow** (`pkg/fonts/sinclair-narrow.bdf`), a 6x8 face derived
  from the Spectrum character set: each 8-wide cell loses two columns, with a
  number of glyphs hand-redrawn for legibility and the lowercase right-aligned
  in the cell. Available as `fonts.SinclairNarrow()`.

### Fixed

- **Dropped-file loading regressed to `.scr` only.** The drop list was read
  twice per frame, and the raylib binding frees it on the first read, so every
  loader after the first saw nothing. The drop is now read once and dispatched,
  restoring `.tap`/`.tzx`, snapshots and `.dsk` across 48K/128K/Next.
- **`sinclair.bdf` carried internationalised-ROM glyphs.** Codes for
  `[ \ ] { | } ~` showed Spanish locale shapes; the font is regenerated from a
  genuine UK 48K ROM so those are the correct Spectrum glyphs.

### Changed

- **The debugger HUD now draws in Sinclair Narrow** instead of the 8x8 Sinclair
  face.

## [0.42.1] - 2026-11-19

Debugger HUD fixes and a reworked Escape key.

### Fixed

- **Typing in the debugger HUD** now works where the platform's character
  queue is not populated: the HUD reads `GetCharPressed` first and falls back
  to mapping the keys pressed that frame to characters, so the command line
  accepts input regardless of the char-queue path.

### Changed

- The debugger HUD no longer shows a banner line when it opens; it opens to a
  clean prompt.
- **`-noesc` is replaced by `-escape`** with four modes: `on` (Escape quits at
  once, the previous default), `off` (Escape never quits), `close` (Escape
  closes an open dialog, menu or the HUD only, never quitting), and `query`
  (Escape asks for confirmation -- a second Escape within two seconds -- before
  quitting). In every mode but `off`, Escape closes an open HUD before any
  quit. Escape handling now lives in one place, the GUI's main loop.

## [0.42.0] - 2026-11-19

A debugger for the running machine: a standalone CLI and an in-emulator HUD
over one shared engine.

### Added

- **A debug engine over the CPU** (`pkg/debug`): breakpoints, single-step and
  run, memory watchpoints, register and memory inspection, and live
  disassembly through zendis. It drives the machine through a narrow
  `DebugView` facade on the machine (`debugview.go`), leaving the zen80 core
  untouched; the fetch seam is zen80's unconditional `DebugPCHook`, and a
  bounded run budget means an unreachable breakpoint stops rather than hangs.
- **A command layer** (`pkg/debugcmd`): one grammar and one fixed-format
  renderer (`break`, `watch`, `step`, `run`, `regs`, `mem`, `dis`, `set`,
  `reset`, `info`, `help`), shared by both surfaces so their output matches.
- **A standalone debugger CLI** (`cmd/zendbg`): builds a headless machine,
  attaches the engine and reads commands on stdin; raylib-free, so it runs on
  Linux and macOS, and doubles as the engine's script-mode test harness.
- **An in-emulator debugger HUD** (Alt+D): the same engine and grammar drawn
  over the running machine in the BDF Sinclair font, with pixel-art transport
  buttons in the dock-icon style, a slide animation matching the dock, and a
  machine caption in the theme accent. In `-multi` it binds to the front
  machine, keeps a separate command buffer and breakpoints per machine, and is
  mutually exclusive with the dock. The HUD slides open and shut with an
  easing animation, and its height is adjustable by a gripper on its top edge
  that grows from the centre as it fades in (clamped between a minimum and
  maximum height).

### Changed

- The machine library gained a `Debug()` facade (a T-39 increment) exposing a
  narrow, read-mostly debug view without exporting the CPU or memory directly.
- `zendis` is now a dependency (v0.8.2), which brings `zentools` up to v0.16.1.

## [0.41.0] - 2026-11-18

A connected-world example across nine Spectrums, and machine-code autorun.

### Added

- **A connected world across nine Spectrums** (`examples/world/`): a character
  walks around a tiled screen and off its edge onto the neighbouring machine's
  screen, and a ball bounces from screen to screen, over the existing
  `-multi`/`-listen`/`-autorun` link. `world.asm` is a compact top-down engine
  (tile background, walking character, drifting ball, wall collision, link
  edge-crossing); `host.py` holds the 3x3 grid and routes what leaves one
  screen onto the right edge of the next, reflecting at the world's border.
  The tile and sprite art is `brickything`'s, rebuilt into Spectrum bitmaps
  with zentools' `zx scr`; the sprite blitter and screen-address routines are
  the Oakhollow engine's, included verbatim. The game tape (`world.tap`)
  ships assembled, so the example runs without an assembler; `run.sh`
  launches it and rebuilds the tape only when `zenas` is present and the
  sources changed.
- **Autorun a prebuilt tape** (`-autorun`, both programs): `-autorun` now
  accepts a `.tap`/`.tzx` file as well as text BASIC. A file with that
  extension is loaded straight into the deck and started with the same
  `LOAD ""`, so an assembled machine-code program (such as the world example)
  autoloads the same way a BASIC program does. `AutorunFile` routes by
  extension to the new `AutorunTAP`; both share `autorunTape`. (T-70)
- **Launch scripts for the link examples**: `examples/link/chat.sh` and
  `examples/world/run.sh` start each example with the right parameters
  (finding `zenzx` in `dist/` or the repo root, or via `ZENZX`), so neither
  needs the command typed by hand.
- **`-focus-follow`** (GUI, with `-multi`): move the keyboard focus to the
  machine whose player beacon (the byte at `0xBFFE`) is set, so the controls
  follow the player across screens. For an example whose program maintains
  that beacon, such as the connected world; off by default.

### Fixed

- The connected-world example now has exactly one character and one ball in
  the whole world, each owned by a single machine at a time: a machine draws
  and moves an entity only while it holds it, so crossing to a neighbour
  removes it from the source screen instead of leaving a copy behind. The
  main loop is synchronised to the frame interrupt (`HALT`), which stops the
  character flickering, and the walk cycle is paced so the legs no longer
  blur. With `-focus-follow` the keyboard follows the character across the
  grid. The world now wraps (a torus): walk off the top and you come out the
  bottom, off the left and you come out the right. The ball is centred in its
  frame, so it no longer looks like it bounces before reaching a wall. Two
  more movers join it -- a pair of scissors and a plant pot -- each starting
  in its own room; all three share one table-driven move/bounce/cross
  routine. Each mover travels its own way: the ball diagonally, the scissors
  straight across (a horizontal rook), the pot straight up and down (a
  vertical rook). An entity now crosses only through the opening in the wall
  border; against a solid wall section the hero is blocked and a mover
  bounces, so a mover can no longer wedge at the edge and flood the link
  (which had clogged the room and stopped the hero passing through it). At the
  centre of a room a rook mover now picks a new direction -- any of the three
  that are not a reversal -- so the scissors and pot wander the grid along the
  corridors instead of running a fixed line (a small per-machine RNG makes the
  choice). The pot moves at half the scissors' speed.
- **`Alt+1`..`Alt+9`** (GUI, with `-multi`): bring that machine to the front
  by keyboard -- the display, sound and keyboard pass to it, the same as
  clicking its tile, so a grid of machines can be driven without the mouse.
  A digit past the number of machines does nothing. `machinedeck_gui.go`
  (`deckFocusKeys`, in the dock's `Update`). (T-71)

### Changed

- The world example's character now faces the way it walks: a mirrored,
  left-facing set of walk frames is selected when it moves left (O), the
  right-facing set when it moves right (P). `assets.asm` carries both sets;
  `world.asm` keeps a `facing` byte.

## [0.40.0] - 2026-11-18

A layout switch, and a chat-room example.

### Added

- **`-layout`** (both programs): with several machines, how the GUI tiles them at the start -- `single` (the default), `columns`, `rows`, `grid`, `main-left`, `main-right`, the View menu's own layouts. So `-multi 4:48k,48k,128k,plus3 -layout grid` opens already tiled, with no trip to the View menu, which still changes it afterwards. The headless program renders each machine on its own and only checks the value. `multi.go` (`parseLayout`, `layoutHelp`), `machinedeck_gui.go` (`layoutByName`), applied in `zenzx_gui.go` once the deck is settled.
- **A chat room over the link** (`examples/link/`): `chatroom.py`, a host that relays each line one machine sends to all the others, each in its own ANSI colour, and `chat.bas` with `chat-ada`, `chat-clive` and `chat-grace`, four clients that show what they are sent and send a line of their own. Four Spectrums in one room on the existing `-multi`/`-listen`/`-autorun`, tiled with `-layout grid`. `docs/link.md` has the command.

### Known state

- Seen under Xvfb: the four clients tiled in a grid, each showing the whole conversation, with the Python host relaying and colouring each line.
- Not seen on macOS or Windows.

## [0.39.0] - 2026-11-18

The Info items open a window.

### Changed

- **Show Info (Tape), Disk Info, Card Info and Snapshot Info open a window with their text**, where each printed a line or two to the terminal, which someone who started the program from an icon never saw. The window is the reader Help and About use (`zenui.TextModal`); the text is still printed as well. `info_gui.go`.
- **They say more than the printed lines did**, since there is room:

  | Window | Says |
  |---|---|
  | Tape | file, mode, playing or stopped and at which block, the first program's name, every block. With no tape, how to load one (it used to say nothing at all) |
  | Disk | file or "a blank disc, not saved yet", tracks and sides, unsaved changes, the file's size, whether the file has changed on disk since it was loaded |
  | Card | for each slot: file, size, write protection, how many files and how much, and whether a Next boots from it, with the reason when not |
  | Snapshot | how to load one, and what Quick Save and Save Timestamped write (it used to name `.zxs` only) |

- A text of fewer than eighteen lines gets a panel of its own height; a longer one, a tape of many blocks, gets the tall panel and scrolls. Paths too long for a line are cut at the front, keeping the file's name.
- `printDiskInfo` and `printCardInfo` are gone; `Alt+I` still prints the tape's blocks to the terminal (`ShowTapeInfo`), since a key pressed in the middle of a game should not put a window over it.

### Known state

- Seen under Xvfb: the Tape window on a 48K with an autorun program, the Disk window on a +3 with a blank disc, the Card window on a Next with two cards. Run Diagnostics still prints its report to the terminal: it is several screens long and is a debugging aid.
- Not seen on macOS or Windows.

## [0.38.0] - 2026-11-18

Making a card bootable.

### Added

- **Make bootable, in the SD Cards window.** It takes the chosen card, which may be empty or hold files of your own, and puts on it the files a Next needs to start NextZXOS, copied from the standard card in the same folder. What the card held is kept. It is offered for a free card that does not boot, when the standard card is there to copy from; it shows its progress, can be stopped (the card is then as it was), and afterwards the card is listed as one that boots. `sdcards_gui.go`.
- **`-next-bootable=CARD`**, both programs: the same from the command line. The name is looked for as `-multi` looks for a card's.
- **The files, and how they were found** (`pkg/nextcard/bootable.go`): eight, 773 KB. `TBBLUE.FW`; `config.ini`, `menu.def`, `enNextZX.rom`, `enAltZX.rom`, `enNxtmmc.rom` and `keymap.bin` in `machines/next`; `nextzxos/enSystem.sys`. The 26.11 distribution's card has 1,393 files (0.38.0 as released said 1,350, which was an estimate); a Next reads thirteen of them on the way to the menu and into the Browser, and starts without five of those (the Multiface's ROM, the `gw03v33` ROM, the clock driver, the welcome screen, the Browser's settings), which was found by leaving each out in turn and then all five together. `MakeBootable` copies the eight and checks the result; `AddTree` is the one rewrite of the card that puts files in several folders.
- **A card made this way boots**: `TestNextZXOSBootsFromAMinimalCard` makes an empty card, makes it bootable from the real standard card and runs a Next from it to the NextZXOS menu (dormant guard G-04, with the other tests that need the real boot files).

### Fixed

- A rewrite of a card that was stopped could still replace the card when every file being added was new to it, because only files already on the card were read through the step that looks for the stop. It is looked for once more before the new card takes the old one's place (`pkg/nextcard/add.go`).

### Known state

- Tried here: a new 280 MB card made bootable from the command line and from the window under Xvfb; the Next restarted from it and showed the NextZXOS menu with the card in slot 1; the Browser and NextBASIC opened on such a card in the headless program. The menu comes up after about 500 frames, which is when the full card shows its welcome screen.
- The card has no dot commands, guides or programs, no welcome screen and no clock driver. That is what was asked for, and the manual says so where it describes the button.
- The list is that of the 26.11 distribution. Another distribution's card as the source is checked for each of the eight files before anything is written, and the copy is refused if one is missing.
- Not seen on macOS or Windows.

## [0.37.0] - 2026-11-18

Several Nexts from the command line, each with its own SD card.

### Added

- **`-multi` takes a card for a Next**: `-multi 3:next/sdcard-01,next/sdcard-02,next/sdcard-03` starts three Nexts, each booting from the card named after the slash (`next=sdcard-01` means the same). 0.36.0 allowed one Next and no more, because two cannot boot from one card. Now every Next but one must name a card, and the one that names none boots from the standard card, so `4:48k,next/sdcard-01,128k,next` is two Nexts as well. Both programs.
- **How a name becomes a file**: as written, then with `.img` added, first from where the program was started and then in the folder of the Next's boot files; so `next/sdcard-01` is `sdcard-01.img` in that folder and `next/cards/a.img` is a path. Before any machine is made each card is checked to be one a Next can boot from (`nextcard.VerifyCard`: a sound FAT32, `TBBLUE.FW`, `config.ini` with `timing=0`), and a card named twice, or the standard card named while another Next is using it, is refused with the numbers of the two machines. `multi.go`: `parseMulti`, `resolveNextCards`, `nextCardsOf`, `bootNextFrom`.

### Changed

- The machine the single-machine options go to, when there are Nexts: the Next with no card named, or the first Next if each has its own (`primaryMachine`). It is also the only one that gets a card in its second slot.
- A count before a colon is a count only when it is a number, so a path with a colon in it can be a card's name.

### Known state

- Tried here with three copies of the distribution's card: three Nexts in the headless program, each at the NextZXOS welcome screen after 1,500 frames; the same three in the GUI under Xvfb; and `4:48k,next/sdcard-01,128k,next` in the GUI, the four in their listed places.
- The boot files must already be in place when a card is named; the first-start download is not offered on this path, and the error says how to fetch them.
- `-autorun` is still not available on a Next (T-69).
- Not seen on macOS or Windows.

## [0.36.0] - 2026-11-18

Several machines from the command line, a socket into each, and a BASIC program from a text file.

### Added

- **`-multi`: several machines at start-up.** `-multi 12,128k` is twelve 128Ks; `-multi 4:48k,48k,128k,next` is four machines, a model each; a list alone (`48k,128k`) and a count alone (`3`, of `-model`) are accepted too; the most is 32. In the GUI they are the dock's machines, in the order given, the first in front. In the headless program they run side by side, which it could not do before, and a screenshot of each is written (`<prefix>-m<N>-frame<F>.png`). `multi.go`, `multi_gui.go`, `multi_headless.go`.
- **`-listen`: the link.** A TCP port for each machine (one number and the others follow it, or one each). What a program outside writes to the socket reaches a program on the machine a byte at a time through I/O ports 16384 (data) and 16385 (control), by a four-step handshake a BASIC program keeps up with; what the program writes to port 16384 comes back on the socket. The ports are opened on `127.0.0.1` unless `-listen-host` says otherwise. `link.go`; the protocol is in `docs/link.md`.
- **`-autorun`: a BASIC program from a text file.** The text is tokenised with zentools' tokeniser, made into a tape of one program that runs from its first line, and loaded by pressing the keys for the machine once it has started: `LOAD ""` on a 48K, ENTER on a 128K, +2, +2A or +3. One file for all the machines or one place each. `autorun.go`.
- **Headless: a run with no end, and its pace.** `-frames 0` runs until interrupted, and is the default with `-listen`; `-pace real` keeps to 50 frames a second (the default for a run with no end) and `-pace max` does not. A run that is interrupted writes its screenshots before it stops.
- **`examples/link/`**: `receiver.bas` (prints what it is sent), `echo.bas` (sends it back in capitals) and `send.py`.
- **For a program that embeds ZenZX**: `NewLink`, `Link.Listen`, `Link.Send`, `Link.Receive`, `Link.SetPorts`, `ZenZX.AttachLink`, `ZenZX.AutorunBASIC`, `ZenZX.AutorunFile`, `BasicTape`.

### Changed

- **One place sets a machine up as a model**: `loadModelROMs` (`machine_model.go`) is the ROM set and hardware of each model, taken out of the GUI's live model switch, which now calls it, so that the headless program's extra machines are made the same way (`newMachine`). The first machine of each program is still set up by its own start-up code. A step of T-39.

### Fixed

- **A dock with more machines than fit ran off both edges of the window.** It shrinks its pictures to fit, but scaled them as if the gaps between them shrank too, so the row stayed too wide by a share of the gaps: with twelve machines in a window at 3x the first picture and the New tile were cut off. The gaps and padding are now taken out before the pictures are scaled (`machinedeck_gui.go`, `layout`). Found by starting twelve machines.

### Known state

- **The two port numbers are not free on the hardware**: an even port is the ULA's and 16385 has the address bits of the 128K's paging port. A machine with a link answers exactly those two numbers itself, before anything else; a machine without one is unchanged, and a test holds both halves of that on a 128K.
- Tried here: four machines (48K, 48K, 128K, +3) in the headless program and four (48K, 48K, 128K, Next) and twelve 128Ks in the GUI under Xvfb, each sent text from a Python client over TCP, the echo read back. The BASIC programs take about 19 to 37 bytes a second at the machines' own speed. Twelve headless 128Ks run at about six times their own speed on this computer.
- Left out, in the register as T-69: `-autorun` on a Next or a TS2068; more than one Next from the command line; a link for a machine added from the dock; single-machine options for any machine but the first.
- Not seen on macOS or Windows.

## [0.35.2] - 2026-11-17

The New tile has a picture.

### Changed

- **The dock's New tile shows a sprite instead of two crossed lines**: the ZX badge with a plus sign over its corner, a 16x16 Spectrum sprite drawn in zenplay (the sprite "Sprite 45" of the package `gui-elements.zplay`). It is kept in `dockicon_gui.go` in zenplay's own notation for a sprite frame, pixels and mask a row, and drawn at a whole number of screen pixels to each of its own so that it stays sharp: about half the tile's height, which is 4 to 1 in a window at 3x. It is black ink on white paper in every theme, and nothing is drawn round it: where the sprite has no pixel the dock shows through. The frame round the tile still takes the selection's colour when the tile is chosen.

### Known state

- The package has no sprite called `zx-add`; "Sprite 45" is the one that is the ZX badge with the add sign. If another was meant, the sixteen rows in `dockicon_gui.go` are all that changes.
- Nothing of zenplay is imported: the sprite is sixteen strings.
- Not seen on macOS.

## [0.35.1] - 2026-11-17

The dock lets the machine behind it show through.

### Changed

- **The dock's panel is see-through at the top and solid at the bottom.** Its opacity runs evenly from 50% at the top edge to 100% at the bottom edge, and the line round it from 70% to 100%. The pictures of the machines, their frames, the names and the New tile are as they were: a picture is never see-through. The two figures are `dockTopAlpha` and `dockEdgeTopAlpha` in `dockround_gui.go`.
- **raycanvas v0.3.1**, which adds the two calls this uses, `FillRoundRectGradient` and `StrokeRoundRectGradient`. `go.sum` records `h1:RALSi+GrOj7i+4fpzDLprYUE9/pKy4IjaqMLl/brSrk=`, computed as 0.35.0's was.

### Known state

- **raycanvas v0.3.1 has to be on GitHub, tagged, before this builds anywhere but here.** v0.3.0 need not be pushed for ZenZX's sake any more; if it already is, v0.3.1 goes on top of it.
- Not seen on macOS.

## [0.35.0] - 2026-11-17

The dock and the pictures in it have round corners.

### Changed

- **The machine dock is rounded.** Its panel, the pictures of the machines, the frames round them, the New tile and the mark under the machine in front all have round corners with smooth edges, and the dock stands 8 pixels above the bottom edge of the window instead of on it. A picture's corners are a twelfth of its height (between 4 and 14 pixels), so a picture that grows under the pointer keeps its shape; a picture flying out to the screen squares its corners off as it lands. The tiles of a tiled layout are part of the screen and stay square.
- **How.** A picture in the dock is a live texture that lies over other pictures and over the machine on the screen, so its corners cannot be painted over in a background colour. It is drawn by [raycanvas](https://github.com/ha1tch/raycanvas) v0.3.0 as a mesh whose corners are part of its shape (`DrawImageRoundedCropped`), and the panel and frames by the same library's rounded fill and stroke. The window has no multisampling, which the machine's own picture must not have, so the edges are raycanvas's soft ones (`SetAntialias`). The code is `dockround_gui.go`; `machinedeck_gui.go` calls it where it drew rectangles.

### Added

- **A dependency: `github.com/ha1tch/raycanvas` v0.3.0**, in the GUI build only (the headless program does not import it). With it come its own: `fogleman/gg`, `golang/freetype`, `srwiley/oksvg`, `srwiley/rasterx`, `golang.org/x/image`, `golang.org/x/net` and `golang.org/x/text`, all pure Go. The stripped GUI binary grows from 11,975,224 to 12,570,968 bytes here (5%).

### Known state

- raycanvas was examined before it was used: it had no tests and several defects, among them the rounded clip that would have been the obvious way to do this. They are fixed in its v0.3.0, which has a test suite that reads pixels back; its CHANGELOG has the list. **raycanvas v0.3.0 has to be on GitHub, tagged, before this builds anywhere but here**, and its tagged tree has to be the delivered one: `go.sum` records `h1:El+LqLPrjyvzBO/GQ2m6W4N5+WFyoAfeaVPufh5272c=`, computed here with Go's own module archiver (`golang.org/x/mod/zip`, checked against zenui v0.2.0's published hash) from that tree.
- Compared with 0.34.2 under Xvfb: the picture at start-up and with the View menu open are the same pixel for pixel outside the frame counter. The manual's picture of a machine being dragged from the dock was taken again.
- The dock is still ZenZX's own and not a zenui widget (T-67).
- Not seen on macOS.

## [0.34.2] - 2026-11-17

`go.sum` has the hash of zenui v0.2.0 as it is published.

### Fixed

- **`go.sum` recorded a hash for zenui v0.2.0 that no published zenui has**, so a build failed its checksum verification once zenui was on GitHub. The hash had been taken here from an archive made with `git archive`, which is not a module archive as Go makes one (it lists directories, and at the time it lacked `MANIFEST.sha256`). `go.sum` now has `h1:56msK0YKFjQhNdPPsh6GxZR1z6BMnMTDqa40XtLdreM=`, which is what the checksum database (`sum.golang.org`) records for `github.com/ha1tch/zenui` v0.2.0 as tagged on GitHub (commit `800ff13`); that tag was compared with the delivered archive file by file and is the same 94 files. `go mod tidy` leaves `go.mod` and `go.sum` as they are.

### Known state

- Nothing else changed: the code is 0.34.1's.
- Do not delete `go.mod` to refresh the dependencies, and do not use `go get -u`: `go.mod` pins the versions this was built and tested with (raylib-go 0.60.0, oto 3.4.0, zentools 0.8.6), and `-u` replaces them all with the newest. `go mod tidy` is the command that brings `go.sum` up to date.

## [0.34.1] - 2026-11-17

The formatting check that CI runs is a step of every release.

### Fixed

- **A release cannot be cut with a Go file gofmt would change.** CI's first step is `gofmt -l .`, and a tree that fails it fails CI before anything is built. 0.27.0 went out with `io.go` unformatted, the last of the fifteen files T-06 listed in August; it was formatted in 0.28.0 and the tree has been clean since. `check_gofmt.sh` is now the one check: a step of `repoman relcore` that runs every time, `make fmt-check`, and CI's own step. It uses the gofmt of the Go the module builds with (`go env GOROOT`), not the first on `PATH`: here that was an older Go's, which can pass a tree the right version rejects. `sh check_gofmt.sh -w` (or `make fmt`) formats. T-06 is closed.

### Known state

- Run here before this release, as CI does: `gofmt -l .` with Go 1.26.5 over the 200 Go files of the tree and of the unpacked archive, both clean; `go vet -tags headless ./...`; the headless build and tests; the twelve cross builds of `make cross` (headless on seven platforms, the GUI without cgo on four, and with cgo on this one); the register check. CI's "Importable as a library" step, like every build, needs zenui v0.2.0 on GitHub.

## [0.34.0] - 2026-11-17

Machines have names, can be renamed, rotated and dragged onto tiles.

### Added

- **A machine takes the name of the first BASIC program it loads**, from tape, a +3 disc or a Next's SD card, unless it has been named by hand. Until then it is called by its model. `ZenZX.FirstProgram` (`program_name.go`) is where the three media meet: the tape's first Program header once the tape has delivered something; on disc, a sector that begins a +3DOS file of type Program, named from the directory entry whose first block it is; on a card, a block that begins such a file, named from the FAT volume. NextZXOS's `autoexec.1st` and `autoexec.bas` are passed over: the system runs them itself when it starts.
- **Renaming.** In the dock, `F2` or a click on a name opens it for typing; `Enter` keeps it, `Escape` does not. A name given by hand is never changed automatically afterwards. An empty name hands the naming back.
- **View menu: Rotate Machines.** Every machine moves to the next place in the layout. In a row, a column or a grid each moves along one tile and the machine in front stays in front; where the front machine has a place of its own (a main tile, or the whole window) the next machine takes it.
- **Dragging from the dock onto a tile.** A press on a picture that moves before it is let go drags the machine; every tile is a drop site and shows a frame while the pointer is over it. The dropped machine goes there and the one that was there takes its place. On the main tile, or on the window when one machine fills it, the dropped machine comes to the front. A press let go where it went down is still a choice, as before.

### Changed

- **A tiled window is at least 336 pixels high below the menu bar's strip.** A single row of tiles at scale 1 is 256 high, less than the View menu that chooses it; the tiles are centred in the window, and keep a machine's proportions while the window is on its way to a new size.
- `nextcard.Volume.FileAt` tells which file of a FAT volume starts at a byte of the image.

### Known state

- Seen under Xvfb with software OpenGL: a 48K loading a tape and its machine becoming "game" in the dock; `F2`, typing and `Enter`; with three 128K machines side by side, one dragged from the dock onto the third tile (the two changed places, the front machine stayed in front) and Rotate Machines moving each along one tile. The names from disc and from a card are covered by tests on a disc image and a card image made for them, and by booting NextZXOS from the real card, which reads one BASIC program of its own (`autoexec.1st`) and names nothing. A program loaded from a card or a +3 disc by hand in the window has not been tried. Not seen on a Mac.
- From tape the name is the tape's first Program header, which is what nearly every tape loads first; a tape whose first program is skipped names the machine after it all the same (T-67).
- In fast tape mode the tape has to be playing (`Alt+P`) for `LOAD ""` to find it, as before; the name comes when the first block is delivered.

## [0.33.0] - 2026-11-17

Two more tiled layouts, with a main tile.

### Added

- **View menu: Stack Left, Main Right and Main Left, Stack Right.** The machine in front has a large tile and the others are stacked beside it: with three machines, two on one side and one, twice their size, on the other. From three machines on. The main tile is as tall as the stack, so with four machines it is three times a small tile's size.
- **The main tile follows the front machine.** A click on a stacked tile, or the dock, brings that machine to the front and into the main tile; the one that was there joins the stack, and the window keeps its size. The dock's flight lands on the main tile.

### Known state

- Seen under Xvfb with software OpenGL on a 1280x1024 screen at zoom 3, with three 128K machines: main right (scale 1, 960x512), a click on a stacked tile, and main left with that machine large. Not seen on a Mac.
- The names say what the layouts are for any number of machines; with three they are the "two on the left, one on the right" and "one on the left, two on the right" that were asked for.

## [0.32.0] - 2026-11-17

Tiled layouts: every machine on the screen at once.

### Added

- **View menu: One Machine, Side by Side, Stacked, Grid.** With more than one machine running, the tiled layouts show them all, each in a cell the size of a machine's whole frame. The machine in front, which has the keyboard and the sound, has a frame round its tile; a click on a tile brings that machine to the front, and the dock's flight lands on the tile. Grid is offered from three machines on.
- **The window takes the layout's shape**, on its way there as it is after a change of zoom, and is moved up or left when it would otherwise run off its monitor.
- **The zoom is for one machine filling the window.** X 1 to X 3 are greyed in a tiled layout, and `Page Up`/`Page Down` say why they do nothing. Tiles are drawn at the largest whole scale, up to the zoom, at which the grid fits the monitor.

### Fixed

- A click that chose an item of the menu bar could also land on what was under the menu in the same frame. With tiles under it that brought another machine to the front.

### Known state

- Seen under Xvfb with software OpenGL on a 1280x1024 screen at zoom 3: three 128K machines side by side (scale 1, 960x256) and in a grid (scale 2, 1280x1024), two 48Ks stacked (scale 2, the window moved to the top to fit), a click on a tile, and the way back to one machine at 960x768. Not seen on a Mac, nor on a screen large enough for tiles at zoom 3.
- In a tile the border is the machine's present border colour; the stripes of a tape loading are drawn only for one machine filling the window.
- The pointer over a tile is not passed to that machine as its mouse, and the layout is not kept between sessions (T-67).

## [0.31.0] - 2026-11-17

The Floppy Disks window, for the +3: the SD Cards window's counterpart, built around what is different about discs.

### Added

- **The Floppy Disks window** (`floppies_gui.go`; Floppy Disk menu, **Manage Disks...**). A disc is read whole into memory when it goes in the drive, and its file gets the changes later, so the disc in memory and the file can differ either way. The window's second line says which: the same, changes here not yet in the file, the file changed on disk since it was read, or both. Its buttons are the ways out: **Write to file now**, **Read file again**, and **Save as new file**, which keeps both. Below are the folder's disc images with size, kind and who has each, and **Put in drive**, **Eject**, **Copy**, **New blank disc** and **Other file...**.
- **It asks before losing a version.** Read file again over unwritten changes, Write or Eject over a file that changed on disk, and Eject of a written blank disc that has no file each take a second press, with what would be lost said under the list.
- **A disc file is in the drive of one machine at a time.** The window does not offer a disc another machine has, Open DSK Image refuses one, and Copy has the machine that holds a disc write its changes to the file first, so the copy has them.

### Changed

- **The 30-second write no longer writes over a file that changed on disk.** The controller remembers the file as it was when read or last written (`FDC765.FileChanged`). If another program has rewritten the file since, and the +3 has changes of its own, the automatic write is not done; it says so once, and Save still writes on the person's say. Before, the disc in memory silently replaced whatever was there.

### Fixed

- **Insert Blank Disk after a DSK.** Reads prefer the parsed DSK to the flat image, and the blank disc only cleared the flat image: after a DSK had been in the drive, the "blank" disc went on reading as that DSK.
- **Save Disk As (F8) of a DSK** wrote the flat image, which a DSK does not use, so the file was not the disc. It writes what the drive reads from.
- Both windows' lists are sized by the height of their rows; they showed fewer rows than intended.

### Known state

- Seen under Xvfb with software OpenGL: the window opened from the menu on a +3 with a disc in its drive, listing that disc, another and a file that is not a disc. The states and the buttons are covered by tests on real files (written, changed outside, both; the two-press questions; Save as new; a blank disc; two machines and Copy), not by a +3 saving from BASIC under the window. Not seen on a Mac.
- A disc dropped on the window or given with `-disk` is not checked against the other machines' drives (T-67).
- One drive, A:, as before.

## [0.30.0] - 2026-11-17

The SD Cards window, and with it a second Next in the same window.

### Added

- **The SD Cards window** (`sdcards_gui.go`; SD Card menu, **Manage Cards...**). It lists the card images in the folder of the Next's files, each with its size, whether a Next boots from it and who has it: free, a slot of this machine, or a slot of another machine. From it: boot (or restart) from another card, put a card in the second slot or take it out, copy a card to a new file with a progress bar, make an empty card of 280 MB or 1 GB, and add an image from another folder. It is made of zenui's `List`, `ButtonBar` and progress bar.
- **A second Next.** A card image is in one slot of one machine at a time. Choosing ZX Spectrum Next from the Machine menu while another machine has the boot card no longer refuses: the SD Cards window opens and asks for a card of this machine's own. Copy the boot card, boot from the copy, and the window has two Nexts. Closing the window leaves the machine as it was.

### Changed

- **Insert Card in Slot 2** refuses a card that is in another machine, and a Next that starts while another has ZenZX's own card leaves its second slot empty and says so.
- The start-up line and the SD Card menu name the card that is actually in slot 1; they named `next-card.img` whatever it was.

### Fixed

- **A correction to 0.28.0's and 0.29.0's notes.** Both said the window had been compared with the earlier build "with the Machine menu open". In those runs the pointer was two pixels short of the top edge and the menu bar never came down, so what was compared was the window without a menu, twice. Done again with the menu open: start-up and the open Machine menu are the same picture in 0.27.0 and now.

### Known state

- Seen under Xvfb with software OpenGL: a Next booted to NextZXOS, a 48K added from the dock and asked to become a Next, the window opening with the boot card shown as machine 1's, the 1 GB copy with its bar, booting from the copy, and both machines at NextZXOS's welcome screen in the dock; and the window opened from the menu on a single Next and closed with Escape. The cards were write-protected throughout (`-sdcard-readonly`), so two Nexts writing to their own cards at once has not been exercised. Not seen on a Mac.
- **Copy** copies the file as it is at that moment. A card that a running machine is writing to can be copied in a half-written state; the window does not stop the machine.
- Escape does not close an open dropdown of the menu bar (it does close the windows, and does not end the program while a menu is open). 0.27.0's note said it closed "the open menu or window"; for a dropdown that was not so then either. T-68.

## [0.29.0] - 2026-11-17

Several machines in one window, and a dock to flip between them.

### Added

- **The machine dock.** Rest the mouse at the bottom edge of the window, or press `Alt+M`, and a dock rises from the bottom the way the menu bar comes down from the top, with a live picture of every running machine and a **New** tile. The picture under the pointer grows. Click one, or choose it with the arrow keys and `Enter`, and it grows out of the dock until it fills the screen, eased in and out with the curve the menu bar slides on; that machine is then in front. `Delete` closes the chosen machine, `Escape` the dock. `machinedeck_gui.go`; the manual's "Several machines".
- **Any number of machines.** One is in front: it has the screen, the sound and the keyboard, and the menu bar acts on it. The others run on behind it, silent. A new machine is the model of the one in front and starts from power-on.
- This is the first use of what 0.28.0 made possible: the window's one display and its one sound device are handed from machine to machine with `SetDisplay` and the audio's `SetDevice`.

### Known state

- Beside a Next, **New** makes a 48K, and the Machine menu refuses a second Next while one is booting from the SD card: two machines cannot share a card image.
- A script given with `-script` drives whichever machine is in front.
- Seen under Xvfb with software OpenGL: the dock opened by the edge and by `Alt+M`, a machine added, typed into and flipped away from and back, three 128K machines in three different states, a machine closed, `Escape` closing the dock and not the program, and a pointer left resting at the edge not reopening it. With one machine and the dock never opened the window is the same picture as 0.28.0's, pixel for pixel. Not seen on a Mac, and the sound passing from one machine to the next has not been heard.
- The dock is ZenZX's own for now. It draws through zenui's renderer and theme, and the place for it as a widget is zenui, with zenplay as its second user.

## [0.28.0] - 2026-11-17

The machine can be imported. The root package is a Go library, `github.com/ha1tch/zenzx`, and the two programs are thin commands over it. The first three of the four stages of `docs/MACHINE_MAP.md` (T-39); the programs behave as they did.

### Changed

- **The root package is `package zenzx`**, not `package main`. `RunGUI(args)` and `RunHeadless(args)` are what the two `main` functions were, each with its own flag set; `cmd/zenzx` and `cmd/zenzx-headless` call them and do nothing else. The build scripts, the Makefile and CI build those two directories. Building by hand is `go build ./cmd/zenzx` and `go build -tags headless ./cmd/zenzx-headless`; `go build .` now only compiles the library.
- **The version is linked into `github.com/ha1tch/zenzx.version`**, where it was `main.version`. The build scripts pass it as before.
- **The core no longer prints.** What the machine has to say goes to a sink set with `SetLogOutput`, and its opt-in traces and script warnings to one set with `SetTraceOutput` (`logging.go`). Both discard until set. The two programs set standard output and standard error, so they print what they printed.
- **One audio engine, with a device.** `AudioManagerOto` existed twice, the real one in the GUI build and a silent stub in the headless one. There is now one `AudioEngine` (`audio_engine.go`) in every build, which plays through an `AudioDevice` when it is given one (`SetDevice`) and is silent, doing no work, when it is not. The GUI program gives it the oto device. oto allows one context in a process, so the devices share it: more than one machine can play.
- **The display is an interface.** `DisplayManager` also existed twice. The core draws through `Display` (`display_core.go`); a machine starts with a display that draws nothing, and the GUI program gives it the window's with `SetDisplay`.

### Added

- **Two machines in one program, tested** (`twomachines_test.go`): a 48K and a 128K machine, with different things typed at different times, are each run alone, then in turns, then at the same time. Each must end with the RAM, registers and picture it ends with alone.
- **`scripts/check_import.sh`** (`make check-import`, and a CI step): a module made on the spot imports the package with `-tags headless,purego`, runs a 48K machine from a ROM it reads itself, reads the picture back, and fails if the library printed anything.
- **`docs/MACHINE_MAP.md`, `docs/machine_clusters.json` and `tools/depmap`**, from zenplay, where they were made for the 0.9.0 tree; measured again before and after this release.

### Fixed

- **Sound can be switched on again after being switched off.** Switching it off closed the audio context and switching it on tried to make a second one, which oto refuses, so sound stayed off for the rest of the session with a message on the terminal. The shared context is suspended and resumed instead. Found by reading the code while splitting it; not heard, since there is no sound card here.

### Known state

- What an embedder can reach is small: `NewZenZX`, the ROM loaders that take bytes, `Reset`, `RunFrame`, `DecodeDisplay`, `SetDisplay` and the audio's `SetDevice`. Choosing a model, the Next's wiring, snapshots from bytes and input are still done by the two programs with unexported names. The exported `Machine` over those is stage 4, and T-39 stays open at ◐ for it.
- Compared with 0.27.0: the headless program prints the same in nine runs (five models, a snapshot, a tape, a script, the Next booting from a card) and writes the same screenshot; the GUI's window is the same picture, pixel for pixel, at start-up, with the Machine menu open and after a key, under Xvfb with software OpenGL.
- Not verified here: sound on a real sound card after the audio change; the two-machine test under the race detector on more than the 2 cores this was run on.
- A snapshot saved by the headless program records the machine's volume settings as the GUI's defaults (0.8, 0.5, 0.5) where the stub recorded 1.0; sound is still recorded as off, since the headless program switches it off.
- The oto code is in two files, `audio_oto.go` (the shared context) and `audio_device_oto.go` (the device), because the edit journal tracks both names and has no way to be told a tracked file was removed on purpose (T-66, which also records that the archive's manifest has to be put back in the repository's order after `relcore`).

## [0.27.0] - 2026-11-16

ZenZX is on zenui, the toolkit it shares with zenimate and zenplay.

### Changed

- **The menus, dialogs and windows are `github.com/ha1tch/zenui` 0.2.0**, and the BDF font reader is zentools' `pkg/bdf`. This tree's own copies are gone: `pkg/zenui`, `pkg/zenuiraylib`, `pkg/zxpalette` and `pkg/bdf`. A program that imported them imports `github.com/ha1tch/zenui`, `zenui/zenuiraylib`, `zenui/zxpalette` and `github.com/ha1tch/zentools/pkg/bdf`. Closes T-38.
- **It was a merge.** What this copy had gained since zenui was extracted went into zenui 0.2.0 first: `MenuBar.SetLabel` and `LabelFor` (the SD Card menu), the Next's colour levels as compared with CSpect, and, as widgets any host can use, the Help and About window (`zenui.TextModal`), the progress bar and its figures (`DrawProgressBar`, `ProgressText`), `WrapText`, `FitTail` and the translucent panel (`ScaleAlpha`). `markdownmodal_gui.go` and `nextsetup_gui.go` use them from there; the first-start dialog for the Next is otherwise as it was.

### Fixed

- **Escape closes the open menu or window and not the program.** It ended the emulator whatever was open, Help included, whose own text says Escape closes it. With nothing open Escape quits as before, and `-noesc` still disables that.

### Removed

- **From the repository's root:** five tape images of commercial games and six quick-save files, which nothing referred to; `zenzx-formatted.dsk` (the Floppy Disk menu makes a blank disc); `build_bsd.sh` and `build_example_bsd.sh`, of which `scripts/retired` has the same files. `.gitignore` now keeps quick saves and screenshots written beside the program out of a commit.
- **Moved to `scripts/retired`:** `_bsd.sh` and `Dockerfile` (T-07), and `release.sh`, `syncver.sh` and `zxspectrum.txt`, the release tooling from before `repoman relcore`. `ztrace_next.sh` is `scripts/ztrace_next.sh`.
- **From `scripts/retired`:** its copies of `build.sh`, `build_linux.sh` and `build_windows.sh`. They were the same files as the live ones in the root, which the README, the manual and CI call; the note there that called them superseded was wrong and is corrected.

### Known state

- `TESTING.md` is generated again; it was last generated for 0.4.65.
- This is the first release cut with `repoman relcore`'s own archive step. Its exclusion of `.git/*` reached one level only, and the first archive carried the repository's git directory; `.repoman.json` now excludes it at every depth, and the examples' build output, and no longer excludes every `.png`, which left the README's pictures and the examples' art out. The archive now holds exactly the tracked files. The first one was not delivered.
- Compared with the build before the change, under Xvfb with software OpenGL: the Machine menu, Help at its top and at its end, About and the Next's first-start dialog are the same picture, pixel for pixel.
- The GUI-tagged tests: 445 pass. They were 458; the 13 that are not here test the Help window and `WrapText` and are zenui's now, with the code. The headless suite and `./pkg/...` pass.
- Not seen: the first-start dialog while it downloads (its bar is `DrawProgressBar` now; the dialog's test of what it draws passes), and the windows on a Mac.
- zenui is not published: `go.sum` has the hash of the 0.2.0 tree delivered with this release.

## [0.26.0] - 2026-11-16

4 MB and DMA+: the Next is an issue 5 board (the KS3) with core 3.03.00.

### Added

- **4 MB of memory** (`nextphys.go`): the machine's memory by its physical address, 22 bits. The MMU still pages 224 pages of 8K, as on every board; the two megabytes above 0x200000 have no page numbers and are reached by DMA+ only, which is how the issue 5 board has them.
- **DMA+ (zxnDDMA)** on port 0x6B (`nextdma.go`), from the guide on the distribution's card (`/docs/zxnDDMA/zxnddma.gde`): the extended address bytes (0xD7, 0xDB), the flags (0xDF: physical addresses, 2D, transparent, fill, Layer 2 X and Y, the hidden screen), row length and rows (0xE3, 0xE7), a stride for each port (0xEB, 0xEF), the transparent colour (0xF3) and the fill value (0xF7). Port 0x0B has none of it and drops those bytes.
- **`-next-issue=5|4|2`** (both builds): the board. 5 is the default; 4 and 2 have 2 MB and drop address bit 21, so higher addresses wrap.

### Changed

- **The core version the Next reports is 3.03.00** (NextREG 0x01 and 0x0E; it was 3.02.03): programs that ask for DMA+ check for it.
- **The DMA as the guide corrects it**, for every program: a length of 0 moves one byte on port 0x6B; a reset (0xC3) keeps the start addresses, the direction, the port types, the mode and the length, and puts back the timings; cycle length 11 is four cycles; WR4 asking for an interrupt control byte with no address stops the decoder until the machine is reset, as on the board.

### Known state

- The guide's nine examples, assembled from their sources with sjasmplus 1.24.1 (built for this, not kept), run from the second card and draw what their texts say. `zxnddma_compare` finds DMA+ and times its five ways of drawing at 7.2, 15.6, 8.1, 2.1 and 2.1 ms; the guide's figures from an issue 5 are 7.2, 15.6, 8.2, 2.1 and 2.1 ms.
- NextZXOS boots and runs the card's programs and ZenZX's examples as before with the new core version (guards G-04, G-06).
- Not compared with a machine or with CSpect 4.5, which does not run here. What is left out is T-64.

## [0.25.0] - 2026-11-16

New empty SD cards.

### Added

- **New Empty Card** in the SD Card menu: a size, 280 MB or 1 GB, then a name in the folder of the Next's files. The card is made empty, formatted FAT32 with the card builder that makes the boot card, and put in the second slot. A name that is already taken is refused.
- **`-next-new-card=PATH`** (both builds): makes an empty 280 MB card and exits; `.img` is added to a name without an extension.

### Known state

- An empty card made this way was put in the second slot of a Next booted from the distribution's card: NextZXOS showed it as drive D, made a folder on it and saved a BASIC program to it, and another FAT reader (pyfatfs) then found both. The file took 576 KB of disk before and 584 KB after.
- The 1 GB size was made and read back by the tests' own reader; it was not put in a running Next.

## [0.24.0] - 2026-11-16

A second SD card, an SD Card menu for the Next, and a `next` folder.

### Added

- **The Next's second SD card slot** (`sdcard2.go`): port 0xE7 bit 1 selects it, as bit 0 does the first. NextZXOS gives a card there drive D. `-sdcard2=PATH` (both builds) puts an image in it; `none` leaves it empty.
- **ZenZX's own card**, `zenzx-card.img` beside the boot files: a 280 MB FAT32 card (`nextcard.NewCard`) that takes about a megabyte of disk. A Next that boots NextZXOS starts with it in the second slot when it is there.
- **The SD Card menu** (GUI, `sdcardmenu_gui.go`): while a Next that boots NextZXOS is running it is where the Floppy Disk menu is for the other machines. It shows the two slots and, for the second, has Insert Card in Slot 2... (a browser for `.img` files), Insert ZenZX's Card (Examples), Remove Card from Slot 2 and Card Info. A card changed while NextZXOS runs is seen after M and Y in its Browser.
- **The `next` folder** (`next/README.md`): the first place the Next's files are looked for after `-next-files` and `NEXT_FILES`, where a download goes when ZenZX is started beside it, and where the SD Card menu's browser opens. `make next-card`, `scripts/get_next_card.sh` and `run_next_divmmc.sh` use it. `thirdparty` is still looked in.

### Changed

- **`-next-examples` and `-next-add`** put their files on ZenZX's own card, making it when needed, and no longer write the card the Next boots from again. In 0.23.0 they copied to `/zenzx` on that card; anything put there stays there.

### Known state

- Seen under NextZXOS from the distribution's card: the second card is drive D at start; put in while the Browser is open it appears after M and Y, and taken out it is gone after the same; each example starts from it (G-06 now runs them that way).
- The SD Card menu was looked at in the GUI under Xvfb: the label, the two slot lines, the card browser. Its actions are covered by a test; clicking each in a window was not done here.
- Not there: a way to change the card in the first slot from the menu, write-protecting each slot separately (`-sdcard-readonly` is for both), and a message in the window when a card is changed (it is printed to the terminal).

## [0.23.0] - 2026-11-16

Three example programs for the Next, and a way to put programs on the card.

### Added

- **Examples for the Next** (`examples/next`): `gallop`, `fuji` and `orbit`, each a `.nex` file that uses several of the Next's features together. `gallop` has Muybridge's horse, as 16-colour sprites whose patterns the DMA streams, crossing a Bierstadt painting on Layer 2, with copper bands and text-mode tilemap captions in the border. `fuji` has Hokusai's Red Fuji as a tilemap over a 320x256 Layer 2 sky that the copper scrolls in four bands, with the palette going from day to dusk. `orbit` has the Moon going behind and in front of the Earth by changing the layer order, in the Next's own interrupt mode with a CTC timer. They are written in Z80N assembly for zenas, their pictures are encoded and the `.nex` files built with zentools' `zx`, and `examples/next/build.sh` builds them on Linux and macOS with those two tools alone. The pictures are from Wikimedia Commons, all public domain; `examples/next/README.md` lists each with its source.
- **`-next-examples`** and **`-next-add=FILE,FILE`** (both builds, `nextadd.go`): copy the examples, which are inside the program, or any files to the folder `/zenzx` of the card the Next boots from, and exit. The rest of the card is kept (`nextcard.AddFiles`).
- A dormant guard, G-06 (`TestNextZXOSRunsTheExamples`): the examples are put on a copy of the card and each is started from NextZXOS.

### Fixed

- **Sprite positions go round at 512**, as jnext shows them: a sprite at X 504 has its right-hand half at the left of the frame, which is how a sprite comes in from the left edge. It used to be drawn only where it was, so it appeared at the left all at once.

### Known state

- Each example shows the same picture in ZenZX and in jnext 1.3.0, started with `.nexload` from the NextZXOS command line; `gallop` was also started from the Browser. They were not run in CSpect or on a Next.
- Found while making them, and worked round in the examples' build: `zx` does not dither to the Next's three bits a channel (zentools' own T-09), so the paintings are dithered when they are prepared; a 320x256 loading screen makes a NEX V1.3 file, which the `.nexload` on the distribution's card refuses, so `fuji` carries its sky as banks; `zx next nex build -bank` takes one file for each bank, so that sky is cut into five with `dd` first.

## [0.22.0] - 2026-11-16

Snapshots started from NextZXOS: the Multiface.

### Added

- **The Next's Multiface** (`nextmultiface.go`, T-52): its ROM, which the firmware already loaded from the card, and its 8K of RAM go over the bottom 16K when a Multiface NMI is taken, and RETN takes them away again. NextREG 0x02 bit 3 asks for the NMI; the Multiface's two ports (0x3F and 0xBF for the +3 kind NextZXOS chooses, the others by NextREG 0x0A) page it out and in while the NMI is served, and on the +3 kind the enable port gives back the paging ports' last values and the border. With NextREG 0xC0 bit 3 the NMI is stackless: the return address goes to NextREG 0xC2/0xC3 and RETN takes it from there. NextZXOS starts a snapshot this way, so `SPECTRUM "file.snx"` and a snapshot chosen in the Browser now run.

### Fixed

- **IM 2 with a table of two bytes** (`im2vector.go`, T-60): the handler's address is read from I*256 plus the whole vector byte, 0xFF from the ULA, as the Z80 does. The CPU core clears bit 0 of the vector first, which only shows with a program that sets just the two bytes that are read and not a table of 257 equal ones. This is on every model, not only the Next. It is put right here until the core is.
- DivMMC's entry points no longer fire on the Multiface ROM's own code while it is paged in.

### Known state

- The palette cycling demo (`/demos/PaletteCycle`) runs from NextBASIC and from the Browser: its text, then each picture with its colours cycling. The card's three other snapshots start too: Pogie to its first screen, The Hollow Earth Hypothesis to its title, OnlyYou6ch to its level meters.
- Neither reference runs the demo, so there is nothing to compare it with: jnext 1.3.0 is back at the welcome screen, CSpect 3.3.1.0 starts the snapshot and stays at a black screen.
- Not there: a key for the Multiface's NMI button in the front ends (the menu it would show draws: it was seen while this was being worked on), the DivMMC NMI of NextREG 0x02 bit 2, the I/O traps of NextREG 0xD8. Not tried: the Multiface kinds other than the +3 (T-61).

## [0.21.0] - 2026-11-16

The Next's own interrupt mode, and the CTC.

### Added

- **Hardware IM 2 mode** (`nextint.go`, T-59): with NextREG 0xC0 bit 0 set an interrupt waits until the CPU takes it, and the vector says which source it was: the top three bits from NextREG 0xC0, then 0 for the line, 3 to 10 for the CTC channels, 11 for the frame. NextREG 0xC4 and 0xC5 enable the sources (0xC4's two are the same as NextREG 0x22's), and 0xC8 and 0xC9 read which have interrupted, a 1 written clearing a bit. The lowest source waiting is taken first.
- **The CTC**: four channels on ports 0x183B, 0x193B, 0x1A3B and 0x1B3B, programmed as the Zilog part is. A timer counts the 28 MHz clock through a prescaler of 16 or 256 down from its time constant; a counter counts the pulses of the channel before it; reading the port gives the count.

### Known state

- Six more of the card's programs were run beside jnext with the same keys, into their games: Santa's Chimney Challenge, Revival Survival, Halls of the Things, the widescreen image demo, the tilemap test and Spectron 2084. All show the same screens in both. That is eleven programs the same as jnext's, and nothing found wrong in this round.
- TX-1696, which turns this mode and a CTC channel on, shows the same screens as before and now reaches each at the same step as jnext; without the CTC it ran ahead.
- Not heard: any sound. Not there: the UARTs, a CTC interrupt in the ordinary mode, and the chain that holds a lower source back while a higher one's handler runs (T-59 stays open for these).

## [0.20.0] - 2026-11-16

4-bit sprites and LoRes, found by running the games beside jnext.

### Added

- **4-bit sprite patterns** (`sprite.go`): attribute 4 bit 7 on an anchor sprite, with N6 in bit 6 choosing which half of the pattern slot; a relative sprite has its anchor's kind and its own N6 in bit 5. A nibble is a pixel, the palette offset is the colour's high four bits, and the low four bits of NextREG 0x4B are the transparent nibble. They were drawn as 8-bit patterns, which is what made TX-1696's logo, menu and ship a scatter of small wrong shapes: that whole screen is sprites.
- **LoRes** (`nextlores.go`): NextREG 0x15 bit 7 puts the 128x96 picture in the ULA's place, a byte a pixel from the two 6144 byte blocks of bank 5, or in Radastan mode (NextREG 0x6A) a nibble a pixel with the palette offset, from either display file. NextREG 0x32 and 0x33 scroll it. ScrollNutter's plasma and bars are LoRes; the ULA's memory was being drawn in their place.

### How it was found

- The programs ZenZX had trouble with were run in jnext and in ZenZX with the same keys, from the NextZXOS command line with `.nexload`, into their games, and jnext's layers were captured one at a time where the two differed. Warhawk, Nextoid, AngryBloaters and The Lords of Midnight are the same as jnext's into the game itself. TX-1696, started from its own folder, differed on every sprite; ScrollNutter differed from the moment its background came on.
- ScrollNutter was then run in CSpect too: CSpect, jnext and ZenZX show the same thing at each stage.

### Known state

- Nothing was found wrong with the DMA or with the order of the layers in these programs.
- TX-1696 turns on the Next's own interrupt mode and a CTC channel, neither of which is emulated (T-59, new). Its first screens and its game are nonetheless the same as jnext's.
- The palette cycling demo, a snapshot started from NextZXOS, runs in neither: jnext is back at the welcome screen after it, ZenZX is at a black screen (T-52).
- The comparisons of games are by eye, at moments that are not the same frame in two emulators.

## [0.19.2] - 2026-11-16

Documents: jnext agrees with CSpect.

### Corrected

- 0.19.1 said that on show512 jnext differs from CSpect. It does not. That run started the program with jnext's `--load`, which puts it on top of a NextZXOS that is already running; started as it was in CSpect and ZenZX, from the NextZXOS command line with `.nexload` after a boot through the firmware, jnext's picture is the same as CSpect's on every pixel.

### Known state

- jnext 1.3.0 against CSpect 3.3.1.0, each booted through the firmware and typed into: the NextZXOS welcome screen and show512 are the same on every pixel. The copper list that waits for horizontal 0 of each line gives the same bars, the full width of the frame, in all three emulators. With the WAIT's horizontal position going from 0 to 47, jnext also draws each line whole, and shows a line's own colour up to position 39 and the line before's from 40; ZenZX changes over at 36, and CSpect is irregular from 34. So jnext meets CSpect's standard wherever the two were compared, and it is the reference with source to read when CSpect's behaviour has to be explained. CSpect stays the standard for the picture.

## [0.19.1] - 2026-11-16

Documents: a second reference emulator.

### Known state

- **jnext runs here as a reference.** jnext 1.3.0 was built without its GUI and runs headless on the same SD card, taking a screenshot after a number of frames. Eight of the card's programs were run in it beside ZenZX. Warhawk, Nextoid, AngryBloaters, The Lords of Midnight, Night-Knight and Spectron 2084 show the same screens in both, by eye. ScrollNutter does not: jnext has its logo and sprites on black, ZenZX has them over a grey screen of noise, which is the LoRes layer that ZenZX does not draw (T-51). On show512 jnext differs from CSpect and ZenZX, which agree with each other: its text has a magenta ground where theirs is black. `docs/next-programs.md` has the table.
- jnext is used as a program that is run, as CSpect is. Nothing of its source is in ZenZX.

## [0.19.0] - 2026-11-16

The Next's DMA.

### Added

- **The zxnDMA** (`nextdma.go`, T-58), on port 0x6B and, in the Z80 DMA's mode that moves one byte more, on port 0x0B. It is programmed as the Z80 DMA is: the seven write registers with their parameter bytes, load, continue, enable and disable, the read sequence and the status byte. It moves blocks between memory and ports in either direction with each address counting up, down or staying. A block with no prescaler goes at once and its CPU cycles (the two ports' cycle lengths for each byte) are charged to the instruction that started it, so a long one runs past the end of the frame. With a prescaler a byte goes every 4 x prescaler T-states of the 3.5 MHz clock while the CPU runs, which is how samples are played.

### Fixed by it

- Warhawk's title and high score table are drawn on a cleared screen; Nextoid's menu is clear of noise; AngryBloaters shows its title and its table one at a time; The Lords of Midnight's screens after the title are drawn right. The README has Warhawk and AngryBloaters among its screenshots now.

### Known state

- Before writing it, what the games send was read from a build that printed it: fills of the screen and its attributes from one byte, copies of 16K blocks (the copy onto itself one byte on, which fills), 256 byte blocks from memory to port 0x5B for sprite patterns, and streams to the NextREG data port. All of it is in the tests.
- Not compared with CSpect: 3.3.1.0 shows a black screen for Warhawk, and 4.5.1.0 does not start under the Mono that is installed. T-58 stays open for that and for the sound of a timed transfer, which was not listened to.
- ScrollNutter still draws wrongly: it uses LoRes (T-51). TX-1696 is still a black screen.

## [0.18.1] - 2026-11-16

Documents: a short README, with the Next in it.

### Changed

- **The README is an introduction**: what ZenZX is, the machines, six screenshots of a Next (the firmware's boot screen, the NextZXOS menu, the Browser, and three of the card's games), how to build and run, and a table of the documents. What it used to hold at length is in new documents: `docs/gui.md` (the menu bar, `machines.json` and `settings.json`, keys, drag and drop), `docs/headless.md` (the debugging workflow, loading binaries and screens) and `docs/development.md` (versioning, releases, continuous integration, the source's layout). The headless flag table, which the manual also had, is the manual's alone.
- **`docs/next-programs.md`** (new) lists the programs on the Next's card that were tried and what each does.
- The manual names the DMA among what a Next lacks and points at the new documents.

### Known state

- Of eight of the card's programs started for the screenshots, Night-Knight and Spectron 2084 draw right and The Lords of Midnight draws its title right. Warhawk, Nextoid, AngryBloaters and ScrollNutter draw wrongly. Those four all send transfers to the Next's DMA, which is not emulated (T-58, new, the first thing to do for the games); ScrollNutter also uses LoRes, which is not drawn (T-51). TX-1696 gave a black screen and was not started from its own folder, which it loads from.

## [0.18.0] - 2026-11-16

Joysticks and the mouse, gone over.

### Fixed

- **The Sinclair joysticks did nothing in the GUI.** The joystick's keys were pressed in the keyboard matrix and the keyboard handler, which runs after it, cleared the matrix every frame. That is every Sinclair mode, the +2, +2A and +3's own two ports included, which are what those models get by default. A joystick's keys are now kept apart from the host keyboard's and joined when the keyboard port is read (`joyports.go`), so neither releases the other's.
- **The Kempston mouse's Y counter ran the wrong way**: it rose as the mouse moved down the screen. It rises as the mouse moves up, as the Next's ports.txt says and as CSpect does.
- `-mouse amx`, there since 0.4.17, was missing from the flag's help and from the manual.

### Added

- **A Next's joystick ports are what the machine chooses** (`-joystick auto`, NextREG 0x05): Kempston 1 or 2, an MD pad on either port, Sinclair on either set of keys, or Cursor, for each port separately. NextZXOS sets them at start. A Next's Kempston port has a second fire button in bit 5 and an MD pad has A and START in bits 6 and 7. Before, a Next had no joystick unless `-joystick` named one.
- **A Next has its mouse** (`-mouse auto`): a Kempston mouse with the wheel in bits 7-4 of port 0xFADF, the three buttons as CSpect returns them (7 with none pressed), and NextREG 0x0A bit 3 to swap left and right. `-mouse amx` is refused on a Next, which has no such interface.
- **The Cursor joystick** (`-joystick cursor`): keys 5 6 7 8 0 (T-13).
- The gamepad's right and left face buttons and its right middle button, for the second fire button, A and START where the machine reads them.

### Known state

- The AMX mouse was already there (0.4.17, T-14): interrupts through IM 2 as AMX Art's own driver takes them. A Next does not have one.
- Checked against CSpect from NextBASIC: the mouse's three ports with the pointer moved and each button held. The buttons and the direction of both counters agree. How far the counters move for a given movement of the host's pointer does not (T-57).
- None of this has been tried with a real mouse, or with a gamepad in any mode but Kempston on a 48K. T-57 (new) lists what has not been tried and what a Next has that is not modelled: user defined keys, the ports' I/O mode, the extra buttons of a six button pad, the mouse's sensitivity.

## [0.17.0] - 2026-11-16

The Next's line interrupt, and a frame of 311 lines.

### Added

- **The line interrupt** (`nextlineint.go`, T-54): NextREG 0x22 bit 1 with a line in 0x22 bit 0 and 0x23 raises a second interrupt in the frame, on the line before the one asked for as the beam leaves the pixel area, for as long as the frame interrupt lasts. The line is counted as NextREG 0x1E/0x1F counts it, NextREG 0x64 included. Setting the line again further down the frame, as a handler does to split the screen more than once, gives another interrupt in the same frame. Bit 2 of NextREG 0x22 turns the frame interrupt off, and bit 7 reads back while the ULA asserts it.

### Fixed

- **A Next's frame is 311 lines of 228 T-states** (70908), where it was the 48K's 69888 with 228 T-state lines: 306 lines. A copper WAIT or a line interrupt for lines 306 to 310, the last five of the top border, was never answered, and the top border's lines were numbered five too low. The 128K machines still have the short frame (T-55, new).

### Known state

- Checked against CSpect 3.3.1.0 by counting interrupts from NextBASIC during a loop: with the line interrupt on the count doubles in both (CSpect 94 to 191, ZenZX 89 to 181), and with the frame interrupt off as well it is back to the single count in both (93 and 89). The loop takes 5% fewer frames here than in CSpect; the cause is not known (T-56, new).
- The copper list and show512 are still the same as CSpect's pictures on every pixel with the longer frame.
- Not modelled: the Next's own interrupt mode with its vectors, enables and status (NextREG 0xC0 to 0xCF).

## [0.16.1] - 2026-11-16

Copper bars are the full width of the frame, as in CSpect.

### Changed

- **What changes part of the way down the frame changes by whole lines.** CSpect is the reference for a Next's picture, and it draws each line whole, from the state near the end of the line's right border. ZenZX 0.15.0 changed at the pixel the beam was at, and the copper's horizontal 0 is the left edge of the pixel area, so a list that waits for horizontal 0 of each line left the 32 pixels of left border, and one pixel more, in the colour of the line before: a step at the start of every bar. Now a change made on a line before the beam leaves its right border shows on all of that line, and a later one from the next line. The copper itself still runs to a quarter of a pixel.

### Known state

- The copper list of 0.15.0's comparison (WAIT line,0 on each of 192 lines) is now the same as CSpect's picture on every pixel, and show512 still is.
- A second list with the WAIT's horizontal position going from 0 to 47 is the same as CSpect for positions 0 to 33. From 34 on CSpect alternates between a line's own colour and the line before's in a pattern that does not depend on the position; ZenZX shows the line's own colour for 34 and 35 and the line before's from 36. 33 of the 192 lines differ.

## [0.16.0] - 2026-11-16

The Next's CPU runs at the speed it is asked for.

### Added

- **NextREG 0x07 changes the CPU's speed**: 3.5, 7, 14 or 28 MHz (`nextspeed.go`, T-53). It was stored and nothing more. The frame, the lines, the copper, the sound and the tape stay in 3.5 MHz T-states; an instruction takes a half, a quarter or an eighth of them. The register reads the speed back in bits 5-4 as well as bits 1-0, and a reset puts it back to 3.5 MHz.

### Changed

- **NextZXOS is as quick as on the machine**, since it turns the speed up for its own work: the firmware hands over after about 450 frames where it was 2250, and the main menu is there after about 760. Scripts that wait a number of frames for NextZXOS wait longer than they need to; none waits too little.
- While the speed is turned up the emulator runs up to eight times the instructions in a frame. Here that is about 2.5 ms a frame at 28 MHz, and the GUI holds 50 frames a second.

### Known state

- show512 is now the same as CSpect's picture on every pixel, all 512 colours: at 28 MHz the demo writes its second palette in the gap between the halves of its picture, as it was written to.
- Memory contention is charged at any speed and divided with the rest; the machine has it only at 3.5 MHz.

## [0.15.0] - 2026-11-16

The copper, and a frame that can change part of the way down.

### Added

- **The copper** (`nextcopper.go`): NextREG 0x60 and 0x63 load its program, 0x61 and 0x62 hold the write address and the start control (stop, start from the top, start from where it was, start and go back to the top at the beam's (0,0)), and 0x64 moves the line count, for NextREG 0x1E/0x1F as well. WAIT and MOVE follow the beam at the copper's own 28 MHz, so a MOVE is half a pixel. A reset stops it and keeps its program.
- **The frame is drawn in pieces** (`nextraster.go`). Just before the copper or the CPU changes something the picture depends on, the part of the frame the beam has passed is composed from the state as it still is, to the pixel. What the front ends show is those pieces and, for the rest of the frame, the state at that moment. A palette rewritten half way down the screen now shows both palettes; a sprite moved after the beam has drawn it shows twice.
- **The border is striped again on a Next** when a program or a tape loader changes its colour during the frame: the change is one more thing the frame is drawn up to.

### Changed

- A frame in which nothing changes is composed once, when it is shown, as before. A frame that does change is composed once in all, in pieces; in the headless build that is 3 to 7 ms of work for each such frame that was not done before, shown or not. The lean frame used while a tape loads in turbo keeps no picture.

### Known state

- Compared with CSpect 3.3.1.0 on a copper list typed in NextBASIC, 192 lines each with its own paper colour: the same picture except the left border and first pixel of each line. CSpect changes whole lines; here the change is where the copper's horizontal 0 is, the left edge of the pixel area, as the Next's documentation has it.
- show512 differs from CSpect on twelve lines now, where it was the whole top half: the demo asks for 28 MHz to write its second palette in the gap between the halves, and the CPU runs at 3.5 MHz whatever NextREG 0x07 holds (T-53, new).
- The line interrupt of NextREG 0x22 and 0x23 is not raised (T-54, new). A write by the CPU is placed at the start of its instruction, up to 40 pixels early.

## [0.14.1] - 2026-11-16

Colours compared with CSpect, and two faults found by it.

### Fixed

- **NextREG 0x44 can be read back.** It returns the second byte of the pair, the lowest bit of the entry's blue. NextZXOS reads its colours through NextREG 0x41 and 0x44 and writes them again through 0x44; the read gave 0, so every colour lost that bit and white came out as the warm grey 0.14.0 described as what the registers hold. The palette NextZXOS ends up with is now the starting one: white 5,5,5 of 7, bright white 7,7,7.
- **Three bits of colour become eight by repeating them** (0, 36, 73, 109, 146, 182, 219, 255), as CSpect shows them; `v*255/7` rounded down gave 72, 145 and 218 for three of the eight levels.

### Changed

- **A Next is always drawn by the frame compositor**, in both builds, with or without NextZXOS and whether or not a layer is on. Its colours are its palettes' (white is #B6B6B6, where the classic machines' drawing has #C8C8C8), so the picture no longer changes colour when a layer comes on. A headless screenshot of a Next is always the 320x256 frame (640x256 in Timex hi-res or with a double resolution layer).

### Known state

- Compared with CSpect 3.3.1.0 run under Mono with the same ROMs and distribution: the NextZXOS welcome screen has the same seven colours, and the card's show512 demo is identical pixel for pixel except the top half of its colour grid. The demo rewrites the Layer 2 palette part of the way down the screen to show 512 colours at once; ZenZX composes the frame once, at its end, and shows the second half's colours in both halves (T-51).
- On a Next the GUI no longer draws the border stripes of a tape loading: the frame's border is one colour (T-51).

## [0.14.0] - 2026-11-16

Sprites show, and palette changes show. Programs load their sprites through three ports that were not there, and the classic picture ignored the Next's ULA palette.

### Added

- **The sprite ports** (`spriteports.go`): 0x303B chooses the sprite and the pattern address, 0x57 takes a sprite's four or five attribute bytes and moves on to the next sprite, 0x5B takes pattern data. Until now sprites could only be set up through the NextREG mirrors, and patterns not at all outside the tests, so no program's sprites appeared. Also NextREG 0x75-0x79 (attributes with the sprite number going up after each write) and lockstep (NextREG 0x09 bit 4).
- **ULANext** (NextREG 0x42 and 0x43 bit 0): the attribute byte split into ink and paper by a mask, inks from palette entry 0 and papers and the border from 128.
- **`-nextreg-log=LIST`** (both builds): prints every write to the listed NextREG registers to stderr, hex numbers separated by commas, or `all`. Off by default.
- `-next-dump` prints palette entries with all nine bits, as three hex digits.

### Changed

- **The classic picture goes through the Next's ULA palette** whenever that palette is not at its starting values, or ULANext is on: the frame compositor then draws the picture, border included, as it does when a layer is on. NextZXOS sets the ULA palette itself, so under NextZXOS the picture is now always the 320x256 frame, in both builds, and `LAYER PALETTE` and other palette changes show at once.
- **Under NextZXOS white is a warm grey**, RGB 5,5,4 of 7, and bright white 7,7,6. NextZXOS writes its colours through NextREG 0x44 with the blue's lowest bit 0; the fixed Spectrum colours used before had it 1. This is what the registers hold and has not been compared with a real machine (T-51).

### Fixed

- **In the GUI the frame's border was always black**: it was given the border colour of the start of the frame from the display manager, which nothing sets; it now takes the machine's.

### Known state

- Run from the card in the headless build: `spriteTest.bas` (64 sprites from `dksprite.spr`) and `spriteFeatures.bas` (a composite sprite, a sprite in the border); a three line palette program with `LAYER PALETTE`. In the GUI build: `spriteTest.bas`, at 50 frames a second under software OpenGL.
- A snapshot started from NextBASIC (`SPECTRUM "file.snx"`) ends at address 0 with a black screen, which is how the card's palette cycling demo starts (T-52).
- 4-bit sprite patterns, sprite collision and reversed sprite priority are not modelled (T-51).

## [0.13.0] - 2026-11-16

The Next's picture is drawn whole. One compositor draws the 320x256 frame, border included, for both builds; the firmware's loading screen, which showed only its left half, is the first thing it puts right.

### Added

- **The Next's frame compositor** (`nextframe.go`, `ComposeNextFrame`). A Next's display is 320x256, with the classic picture 32 pixels in and 32 down. The compositor draws all of it: Layer 2 in its three modes (256x192 on the classic picture, 320x256 and 640x256 over the whole frame) with its clip window and its scroll registers (NextREG 0x16, 0x17, 0x71); the sprites, placed in the frame's coordinates, with their clip window (NextREG 0x19), "sprites over border" and the clipping of it (NextREG 0x15 bits 1 and 5); the tilemap over the whole frame in 40 and 80 columns; the ULA with its Timex modes, its clip window (NextREG 0x1A) and the border, in the Next's ULA palette; the layer order of NextREG 0x15; the global transparency colour for Layer 2 and the ULA; and the fallback colour where nothing is opaque. The frame is 640 pixels across when a layer with double horizontal resolution is on.
- **NextREG 0x19 and 0x1A**, the sprites' and the ULA's clip windows, and NextREG 0x1C in full: every bit restarts its window's sequence, and a read gives the four write positions.
- `nextframe_test.go`: where each layer sits, clip windows, scrolling, the six layer orders, transparency, the fallback colour; and a benchmark (3.6 ms a frame at 320 across, 6 to 8 ms at 640, on the sandbox).

### Changed

- **Both builds show the frame whenever a Next has Layer 2, the sprites or the tilemap on.** The GUI shows it as one texture over the whole picture area, in place of its cell drawing, its border, and the earlier drawing of Layer 2 and the sprites a pixel at a time on the GPU; with the border hidden it shows the classic picture's part. The firmware's loading screen went from about 13 frames a second to 50 under software OpenGL. A headless screenshot of a Next with a layer on is the frame: 320x256 or 640x256, where it was 256x192 or 512x192 with the layers cut to the classic picture.
- **Sprites are placed as on the hardware**: their coordinates are the frame's, so (32,32) is the classic picture's corner. They were drawn 32 pixels up and to the left of that.
- **The classic picture is 32 pixels down in the GUI's window, on every machine**; it was 28. The window's 256 lines were split 28 above and 36 below; they are now 32 and 32, which is where a Next has the picture in its frame, so nothing moves when a Next turns a layer on.

### Fixed

- **The firmware's loading screen showed only its left half** (T-48), in both builds. It is Layer 2 at 320x256. In that mode the clip window counts X in units of two pixels, so the firmware's 0 to 159 is the whole width and was taken as the left 160 pixels; and the picture starts at the frame's corner, in the border, where it was drawn from the classic picture's corner and cut to 256x192.
- **Layer 2 did not scroll**: NextREG 0x16, 0x17 and 0x71 were stored and not used.
- **A layer under the ULA never showed in the GUI**, which drew only the layers above it; and with the border hidden Layer 2 and the sprites were displaced.

### Known state

- Run from the card in the headless build and looked at: the firmware's loading screen, Daybreak (Layer 2), ScrollNutter (Layer 2 scrolling), show512 (Layer 2 with a palette), and the tilemap demo, which now fills the frame. In the GUI build: the loading screen, and the same compositor through `TestGLNextPicture`.
- The frame is drawn once, from the state at the end of each frame, so what a program changes part of the way down the screen shows its last value everywhere; LoRes, the blending layer orders, the ULA's scrolling and the Layer 2 priority bit are not drawn (T-51).
- The drawing code the compositor replaced is still in the tree, unused by the front ends, for one release (T-50).

## [0.12.0] - 2026-11-16

The Next sets itself up. Started for the first time, it offers to download its SD card and ROMs, builds and verifies the card, and boots NextZXOS; from then on it finds the files and starts without asking.

### Added

- **The Next gets its boot files by itself, once.** Started without them (`-model=next`, or ZX Spectrum Next in the Machine menu), the GUI shows a dialog that explains what is needed, where it will be kept and where it comes from, with both addresses editable, and downloads nothing until it is confirmed. A progress bar follows the download and the building of the card; the downloads are checked against their SHA-256 and the finished files verified before the Next starts from its boot ROM. The files go to `zenzx/next` in the user's configuration folder and are found there from then on. `zenzx-headless -next-setup` does the same without a window. `-next-files=DIR` names another folder, `-next-classic` keeps the +3 ROMs.
- **`pkg/nextcard`**: the download, a FAT32 reader and builder, and the verification, in Go, so that preparing the card needs neither Python nor pyfatfs. The card it builds has the 4K clusters, `timing=0`, short names made again from the long ones, and a free space record written from the FAT.
- **The Machine menu switches to and from a Next with its boot chain** (`nextchain.go`): choosing the Next attaches boot ROM, DivMMC and card and starts from the boot ROM; choosing another model detaches them.
- **`-next-files=DIR`**, **`-next-classic`** (both builds) and **`-next-setup`**, **`-next-distribution=URL`**, **`-next-bootrom-url=URL`** (headless): where the boot files are, a Next without them, and the download without a window.
- **The files are looked for beside the program too** (`thirdparty/` next to the binary), after `-next-files`, `NEXT_FILES` and `./thirdparty` and before the per-user folder, so a start from a desktop or another folder finds what `make next-card` made.

### Changed

- **The first-start window is drawn as the Help and About windows are**: the selected theme's backdrop and panel, slightly see-through over the running machine, the same text size, a close box, and a body that scrolls (wheel, arrow keys, Page Up and Down, scrollbar) when the window is too small for it. The address fields, the progress bar and the buttons stay fixed below the body. The address fields take typing, Backspace, Tab and a paste (Ctrl+V, Cmd+V), and show the end of a long address.
- **No window when the files are there and usable.** At every start the three files are checked (`nextcard.Usable`): both ROMs 8K, the card a FAT32 volume the firmware accepts with `TBBLUE.FW` on it. Usable files start the Next at once. Files that are there and are not usable are passed over, and the window then says which folder and why.
- **`run_next_divmmc.sh` no longer logs by default.** It passed `-divmmc-debug` on every start, which writes a line to the terminal for each byte exchanged with the SD card and slowed the Next down a great deal. `NEXT_DEBUG=1 sh run_next_divmmc.sh` (or `-divmmc-debug` on either binary) brings the logging back.
- **The audio path's warnings are off unless `ZENZX_AUDIO_DEBUG` is set**: speaker history truncated, CPU cycle counter going backwards or jumping. The Next resets its CPU when the firmware hands over, which printed there while the audio lock was held.

### Known state

- Every DivMMC, SD card and NextREG message in the emulation is behind `-divmmc-debug`, and the FDC's behind `-debugfdc`; nothing else prints from the machine's inner loops. Without logging the headless build runs the whole boot to the main menu, 3,010 frames, in about 1.5 s.
- What is slow in the GUI on a Next is drawing, not logging: Layer 2 and the sprites are drawn a pixel at a time on the GPU, so the firmware's loading screen runs well below 50 frames a second under software OpenGL (T-48).

## [0.11.0] - 2026-11-16

One ZenZX again. Two lines had grown apart from 0.9.0: 0.9.1 and 0.9.2 with the Next boot work, and 0.10.0 with `.szx` loading, the lazy Next page store and AY gating. This release is their merge; everything in the 0.10.0, 0.9.2 and 0.9.1 entries below is in it.

### Added

- **DivMMC and SD card emulation for the Next boot chain (T-36, in progress).** `divmmc.go` models the DivMMC interface (port 0xE3 control, automap entry and exit, CONMEM, MAPRAM, 8K ROM and RAM pages); automap is gated on NextREG 0x0A bit 4 and is off at power-on, as on the hardware. `sdcard.go` models an SD card over SPI (ports 0xE7 chip select and 0xEB data) from a raw image: CMD0, CMD8, CMD9, CMD12, CMD17, CMD18, ACMD41, CMD55 and CMD58, with CRC-16 on data blocks. `pkg/nextdisk` reads named files out of a FAT16 or FAT32 image through blipper's `fatfs`.
- **Boot ROM overlay.** `-bootrom` maps an 8K first-stage boot ROM (`nextboot.rom`) over the bottom of memory until the firmware turns it off through NextREG 0x03.
- **New flags** in both binaries: `-bootrom`, `-divmmc-rom`, `-sdcard` and `-divmmc-debug`. `-sdcard` needs `-divmmc-rom`. `run_next_divmmc.sh` starts the whole chain from `thirdparty/` (`nextboot.rom`, `enNxtmmc.rom`, `next-card.img`; `NEXT_FILES` names another folder); `ztrace_next.sh` runs the trace harness over the same files.
- NextREG power-on values for 0x01 (core version 3.02), 0x0A, 0x0E, 0x0F and 0x10, taken from jnext's VHDL-cited defaults.
- **Config mode: the firmware can now load the machine's ROMs (`nextconfig.go`).** A Next with a boot ROM powers on in config mode, where NextREG 0x04 maps a 16K SRAM bank at 0x0000-0x3FFF: the four ROM banks, the DivMMC ROM and RAM, the Multiface and alternate ROM banks (stored, not yet used) and every RAM bank. NextREG 0x03 chooses the machine and leaves config mode (111 enters it again; the machine type is accepted only in config mode). NextREG 0x02 resets: bit 0 restarts the CPU and the paging and keeps memory, so the machine starts from the ROM the firmware loaded; bit 1 also brings the boot ROM and config mode back.
- **NextREG 0x8E** (`nextreg8e.go`): the paging ports 0x7FFD, 0x1FFD and 0xDFFD in one register, effective at once. NextZXOS changes ROM through it in its inter-ROM call.
- **NextREG 0x1E and 0x1F** read the line the display is drawing (`nextvideoline.go`); the firmware paces its screens with it.
- **NextREG 0x50-0x57 read back the live mapping**, including what ports 0x7FFD and 0xDFFD and NextREG 0x8E did to slots 6 and 7. NextZXOS saves and restores a slot by reading it.
- **The DivMMC's programmable entry points** (`divmmc_entries.go`): NextREG 0xB8-0xBB choose which opcode fetches map the interface in, whether only with ROM 3 at 0x0000, and whether at once or after the opcode; 0x1FF8-0x1FFF unmaps after its opcode. The acknowledge cycle of an interrupt is no longer taken for a fetch at 0x0038. NextREG 0x09 bit 3 clears MAPRAM, and port 0xE3 reads back on the Next.
- **The alternate ROM and ROM lock, NextREG 0x8C** (`nextaltrom.go`): two more ROM images in SRAM banks 6 and 7, written through the ROM's addresses and then run in its place; the lock bits hold a ROM whatever the paging ports say; a soft reset takes bits 3-0 into bits 7-4. NextREG 0x02 reads back the kind of the last reset.
- **`TestNextZXOSBoots`** (`next_boot_test.go`), the boot chain against the real firmware, and unit tests for config mode, the paging register, the entry points, the alternate ROM and the screen cache that need no third-party files.
- **The SD card can be written** (`sdcard_write.go`): CMD24, CMD25 and CMD13, with CMD16 and ACMD23 accepted. Both front ends open the image for writing, so the firmware and NextZXOS change it as they do a real card; `-sdcard-readonly` write-protects it, and an image that cannot be opened for writing is write-protected by itself. A program saved from NextBASIC is on the image afterwards (T-43, closed).
- **The tilemap** (`nexttilemap.go`) and **the Timex screen modes** (`nexttimex.go`), in the headless build's picture: the tilemap in 40 and 80 columns with text mode, clip window, scrolling and its combination with the ULA, and screen 1, hi-colour and hi-res through port 0xFF or NextREG 0x69. In 80 column and hi-res modes the picture, and so a screenshot, is 512x192. NextBASIC's editor displays in 64 and 85 columns and the card's tilemap demo runs. Both front ends take this picture from one function (`nextsoftula.go`): the GUI, whose standard picture is drawn on the GPU cell by cell, shows it as a texture whenever a Next is in a Timex mode or has the tilemap on.
- **GL tests** (`gl_gui_test.go`, `TestGLNextPicture`): they open a window and read back what was drawn, for the Layer 2 palette textures, a Layer 2 picture in bank 8, the standard screen, Timex hi-res and the tilemap. They run when `ZENZX_GL_TESTS` is set; a virtual display with software OpenGL is enough (dormant guards G-03 and G-05).
- **`-next-dump`** (headless) prints the Next's registers, memory mapping, DivMMC state and the start of each palette at the end of a run.
- **`scripts/next_card.py`** makes the SD card from the official emulator distribution (`sn-emulator-26.11.zip`): it formats the card again as a valid FAT32 volume with 4K clusters, copies every file, and sets `timing=0` in `config.ini` so the firmware does not stop at its video-mode test. It also writes out the distribution's `enNxtmmc.rom`. Needs `pip install pyfatfs`.
- **`make next-card`** (`scripts/get_next_card.sh`) downloads the distribution, checks its SHA-256 and runs `scripts/next_card.py`; `make run-next` starts the machine; `make next-card-repair` corrects an existing card in place. `thirdparty/` is ignored by git.
- **`TestNextZXOSSavesToCard`** (next_save_test.go, with guard G-04): boots NextZXOS, saves a program from NextBASIC and reads the directory entry and the file back from the card image.

### Changed

- **Go 1.26.5 or later is now required**: `pkg/nextdisk` depends on blipper v0.9.25, which requires it. `golang.org/x/sys` moves to v0.47.0 with it.
- **`manifest.sh`** writes and checks `MANIFEST.sha256` (every tracked file, sha256, sorted). The manifests of 0.9.0 to 0.10.0 had been kept by hand and were stale; CI now asks for Go 1.26.
- **Register IDs.** Both lines had used T-35 and T-36. The 0.9.x numbers stand; the 0.10.0 items T-35, T-36 and T-37 are now T-38, T-39 and T-40 (`docs/TRACKING.md` carries the note).

### Fixed

- **Next: the renderer's screen cache no longer overwrites RAM.** When the displayed bank changed (port 0x7FFD bit 3), the cache was copied back into the bank, as on the classic models where the cache is the displayed bank. On the Next writes go to RAM first, so this overwrote bank 7 with bank 5's picture. The cache now follows RAM on the Next, and only writes to the displayed bank reach it.
- **Next: Layer 2 is drawn only when it is on.** It was composited over the ULA picture on every Next, from a reading in T-29 that no enable bit was documented. The Next's `ports.txt` gives it: port 0x123B bit 1, alias NextREG 0x69 bit 7, off after a reset. A program that drew Layer 2 without turning it on, which the hardware would not show, is no longer shown.
- **Next: NextREG 0x12 and 0x13 are 16K bank numbers** with reset values 8 and 11, as the Next's `registers.txt` gives them; they were read as 8K page numbers with reset value 0 (T-42).
- T-41 closed: the boot chain has a repeatable check (see `docs/RESOLVED.md`).

- **A script's keys reach the machine in the GUI** (T-45). The GUI copies the host keyboard into the key matrix every frame, after the script had put its keys there, so `key`, `type` and `press` did nothing in the GUI build on any model. While a script has keys down the host keyboard is now left out (`Scheduler.OwnsKeyboard`).
- The start-up line for `-model=next` no longer says there is no NextZXOS boot chain.
- **The card's free space record** (T-49). `scripts/next_card.py` left the FSInfo sector as the formatting library wrote it for the empty volume, so NextZXOS reported the whole card as free (1022M on a card with 902M free) and was told the first free cluster was 2. The script now writes both values from the finished FAT.
- **`run_next_divmmc.sh` rebuilds a stale `./zenzx`.** It built the binary only when there was none, so a binary from an earlier version, without card writes, kept being used. Both front ends now say at start-up whether the card is write-protected, and warn when the image could not be opened for writing.
- **Next: NextREG 0x15 resets to 0x00**, the layer order S L U, as `registers.txt` gives it; the table held 0x08 (S U L), which kept Layer 2 under the ULA until a program wrote the register (T-47).
- **The GUI-tagged tests run and pass** (T-46). They had only ever been compiled. One started a real browser through `rl.OpenURL` and kept `go test` from ending; three expected the 640x256 Layer 2 mode to count with Layer 2 off; one read the machine menu wrongly. `go test .` without the `headless` tag now passes on a machine with a display.

### Known state

- **NextZXOS boots** from the 26.11 card made by `scripts/next_card.py`: the boot ROM loads TBBLUE.FW, the firmware loads the ROMs into SRAM, chooses the machine and resets into it, and NextZXOS starts from the card and shows its welcome screen, its main menu (v2.10, 1792K) and the Browser listing the card. Checked in the headless build; the GUI build has not been run with it. `TestNextZXOSBoots` repeats the boot against the real files (dormant guard G-04, `ZENZX_NEXT_FILES`).
- The same in the GUI build, run on a virtual display with software OpenGL: the welcome screen, the main menu, NextBASIC's editor in 64 columns and the card's tilemap demo.
- Not there yet: the Multiface and the NMI buttons are not modelled.
- With the boot ROM and no card the firmware reports "Error initializing SD card!", and with an empty card "Error opening TBBLUE.FW file", as it should.

## [0.10.0] - 2026-11-12

### Added

- **`.szx` snapshots load.** `-snapshot`, the `snapshot` script verb and dropping a file on the window now accept a `.szx`. Before, an `.szx` was taken for a `.z80` and failed with `decompressed RAM is 11632 bytes, want 49152`. `DetectSnapshotFormat` knows the `.szx` extension and the `ZXST` magic, and loading goes through `zentools/pkg/szx`. Saving to `.szx` is not supported and says so; without that case in `SaveSnapshotFormat`, a name ending `.szx` would have called the function again, for ever, once detection returned `szx`. `-format` lists `szx` (load only).

### Changed

- **The Next page store is allocated on its first write instead of inline in every `SpectrumMemory`.** `nextExtraRAM` (208 pages of 8 KiB) made every machine, a plain 48K included, carry about 1.7 MB; it is now a pointer allocated by `nextPageWrite`, and reading an unwritten page still returns 0. `SpectrumMemory` drops from 1,908,792 to 204,864 bytes, guarded by a size test. No behaviour change.

- **The zentools requirement moves from v0.6.0 to v0.8.6**, which provides `pkg/szx`. The in-tree `pkg/bdf` and the rest of `go.mod` are unchanged. The headless suite, the `./pkg/...` suite and a cgo-free windows type-check of every GUI file and test pass with it.

### Fixed

- **The AY-3-8912 ports no longer answer on a 48K (T-17).** 0xFFFD (register select, data read) and 0xBFFD (data write) were decoded on every model. They are now gated by a new `hasAY()`, true for the 128K family and Next. On a 48K a write is ignored and a read of 0xFFFD falls through to the floating bus, like any other unattached port. The TS2068's own AY ports are unaffected. Closes T-17.

- A personal name removed from comments in `display.go`, `display_constants.go`, `layer2.go` and `layer2_viewport_test.go`, and from `docs/TRACKING.md`, which the release gate's forbidden-string scan was already failing on.
## [0.9.2] - 2026-09-18

### Fixed

- **T-35: `-model=next` booted to a blank screen despite correct ROM/CPU behaviour.** Root-caused by direct comparison against a working `-model=plus3` boot: CPU execution and raw memory content were confirmed byte-for-byte identical between the two, isolating the bug to rendering rather than execution. `writeNext` (the memory-write path used whenever `isNext=true`) wrote screen data correctly into the raw RAM array but never mirrored it into `zx.screen.bitmap`/`.attributes`, the separate cached buffer the video renderer actually reads from -- a mirror step the classic (non-Next) write path has always done for bank 5. Fixed by adding the same mirror to `writeNext`, scoped to bank 5 (page 10) to match the classic path's own address range exactly. `-model=next` now renders a genuine, correct boot screen. This also resolves Wave 3 (`ti0.r0`)'s placeholder scope in full -- auditing Z80N wiring as a real machine mode and confirming (by finding and fixing it) the one gap that existed beyond opcode semantics.

## [0.9.1] - 2026-09-18

### Added

- **`next` is now a selectable machine model** (`-model=next` in both the headless and GUI binaries, plus the GUI's live model-switch menu and a new "Specnext Ltd" section in `machines.json`). Selecting it enables the Z80N CPU and the Next extended MMU together. This is a hardware-level milestone, not a boot-compatible Next yet: booting a classic ROM under `-model=next` still produces a blank screen (root cause not yet found -- classic-ROM boot has a much larger failure surface than the Next hardware model itself and was deliberately not chased this release, see docs/NEXT_BOOT_REFERENCE.md). What *is* now confirmed, end to end through the real CLI binary rather than only at the unit-test level, is that the underlying Next hardware model works correctly when driven directly by a Z80N program: NEXTREG dispatch, the 8-slot MMU (paging a fresh Next-extra-RAM page in via a live NEXTREG write), the Layer 2 framebuffer, the palette system (including the NR 0x43 target-select register, whose ULA-vs-Layer2 default is easy to miss), and the ULA/Layer2/sprite compositor all produced a correct, verified pixel through `DecodeDisplay()` and the final PNG.

- **docs/NEXT_BOOT_REFERENCE.md**, a reference doc on the real ZX Spectrum Next boot sequence (the three-stage boot-ROM to firmware to NextZXOS handoff, with the exact NextREG values at each transition) and where Next firmware is legally sourced from. Notably: real hardware's classic-compatible boot presents itself as NextREG machine-type "+3" (`0xB3`), not "128K" -- a likely lead for the still-open blank-screen issue above, since the current `next` model case loads the 128K ROM set.

### Fixed

- **T-33: NEXTREG-opcode writes bypassing the port-0x243B select latch.** Closed -- `zen80 v0.5.7` is confirmed published and fetchable as a real, hashed module dependency (no `replace` directive), and the full test suite (`./pkg/...` and the headless build, including the two dedicated regression tests) re-runs clean against it.
- **T-34: NextREG 0x00 (machine ID) default was 0x00; real hardware/VHDL value is 0x0A.** Closed -- already shipped in v0.9.0's tree.
- Removed a handful of incidental personal-name references from source comments and `docs/TRACKING.md` (no functional change).

## [0.9.0] - 2026-09-15

### Added

- **Layer 2 wider resolution modes (320x256x8bpp, 640x256x4bpp) and the Layer 2 Access Port, completing T-29's remaining Tier 0 scope.** layer2.go's layer2CurrentMode/layer2FramebufferDimensions/layer2Pixel now decode and address both wider modes, with the wider modes' column-major (X-then-Y) addressing confirmed against wiki.specnext.dev/Layer_2 and cross-checked numerically against the page's own worked examples. New layer2accessport.go implements IO port 0x123B (confirmed via the wiki to be a pure CPU memory-paging feature, independent of which bank is displayed), requiring a new closure-hook pair on SpectrumMemory (layer2AccessReadFn/WriteFn) since readNext/writeNext previously had no route back to SpectrumIO's own NR 0x12/0x13 state.

- **Sprite attribute 4, scaling, rotation, mirroring, composite/unified relative sprites, and a configurable transparency index, completing most of T-30's remaining Tier 0 scope.** sprite.go was rewritten to store raw attribute bytes (attr0-attr4) rather than a pre-decoded subset, since several bits carry different meanings for anchor versus relative sprites. Scaling (x1/x2/x4/x8 per axis) and rotation/mirroring render as GPU draw-call geometry via rl.DrawTexturePro's source/dest rectangles rather than baked texture variants, once raylib's real capabilities were confirmed against vendored source; both composite and unified ("big sprite") relative sprite modes are implemented, with sequential anchor-dependency tracking matching real hardware's own index-order semantics. NR 0x4B (transparency index) is now a real, configurable register rather than a fixed stand-in.

- **The GUI window now correctly displays every Layer 2 resolution mode.** DisplayManager's window sizing previously assumed the classic 256x192 canvas unconditionally, so the wider Layer 2 modes above -- though correctly addressed in memory -- had no reserved viewport space and were not visible end-to-end. The window's client area is now a fixed reference canvas (320x256) that never resizes on its own; Layer 2's own viewport centres within it, with 640x256x4bpp mode's pixels drawn at half the normal screen width (confirmed against an exact wiki.specnext.dev/Layer_2_Control_Register quote: "every byte is displayed as two half-width paired pixels") so that mode occupies the identical footprint as 320x256 mode at any given zoom, matching real hardware's own double-density (not double-width) behaviour.

### Fixed

- **Sprite transparency check compared the palette-offset-resolved colour index instead of the raw pattern byte.** Found while wiring NR 0x4B into a real register; the real hardware (and this emulator's own resolveNextPixelIndex convention elsewhere) compares the raw byte. Caught by a dedicated regression test before shipping.

- **A GPU rendering regression introduced, then caught and fixed, during the Layer 2 GUI-wiring work above.** renderLayer2GPU's per-pixel draw technique deliberately splats each pixel wider than its own column spacing (relying on left-to-right draw order to overwrite the overlap), previously contained by a scissor rect sized to the classic 256x192 screen. Removing that scissor -- necessary so wider Layer 2 modes could draw outside the classic screen's own narrower bounds -- would have let each row's rightmost pixel bleed into the border uncontained; fixed by scissoring to each mode's own real displayed footprint instead.## [0.8.0] - 2026-09-15

### Added

- **Hardware sprites: 128 sprites, 16x16 pixels, 8-bit colour, unscaled (T-30, wave 7).** New sprite.go implements the base sprite system per NEXT_SUPPORT_DEVELOPMENT_PLAN.md's ti0.r4 minimum scope: an attribute table (position, palette offset, visibility, pattern index) and 64-slot pattern memory, both reachable via NextREG 0x34 (sprite select) and 0x35-0x38 (attribute writes), with sprite pixel colour resolved through the same resolveNextPixel seam Layer 2 uses (T-29's own compositing-seam extraction) rather than a second, independent colour mapping. Compositing replaces the previous two-layer ULA/Layer2 boolean with a genuine three-layer decode of NR 0x15's bit-4:2 priority field (nextLayerOrder/layersAboveULA, layer2.go), covering all 6 real hardware orderings. In the live GUI, sprites render through a capped, LRU-evicted GPU texture cache (spriteGPUCache, videorender_gpu.go) keyed by (pattern, palette offset) rather than pattern alone, since a palette offset is per-sprite; a stale or missing entry is rebaked on demand, so a program's actual working set of sprite appearances becomes GPU-resident after its first few frames. Scaling, rotation, mirroring, 4-bit colour patterns, composite (anchor+relative) sprites, and several NextREG-adjacent registers (NR 0x19 clip window, 0x43 palette selection, 0x4B transparency index) are explicitly deferred, per the plan's own "Deferred within Tier 0" list -- see docs/TRACKING.md's T-30 entry for the full breakdown of what shipped versus what remains.

- **The real 9-bit/512-colour Next palette, replacing the 16-colour interim mapping (T-31, wave 8).** New nextpalette.go and pkg/zxpalette/nextpalette.go implement all 8 real hardware palettes (ULA/Layer2/Sprites/Tilemap x first/second, 256 entries each) and the full NextREG palette protocol: NR 0x40 (index select), NR 0x41 (8-bit RRRGGGBB write/read, auto-incrementing), NR 0x43 (read/write-target select plus independent per-layer display-palette select), and NR 0x44 (two-write 9-bit colour, the true third blue bit no longer synthesised). Layer 2 and Sprites now resolve pixels through their real, runtime-writable palettes instead of collapsing down to the classic 16-colour zxPalette; the GPU fast path gained a dedicated 256-entry texture cache (layer2ColourGPU) that rebakes whenever the palette actually changes, since Layer 2's old cache-reuse trick relied on the palette being immutable. ULA and Tilemap palettes are stored and correctly readable back but not yet wired into any renderer (both are separate, larger features outside this wave). The palette's power-on default is documented as an open gap (all-black, not an approximation of the classic 16 colours) rather than an invented accuracy claim -- see docs/TRACKING.md's T-31 entry for the full breakdown.

### Fixed

- **Layer 2/ULA priority: NR 0x15's %010 ordering (S U L) was composited as Layer2-on-top instead of ULA-on-top.** layer2ULAOnTop's switch statement grouped %010 into the wrong branch, contradicting its own doc comment's stated rule that ULA is on top whenever U precedes L in the ordering -- and U does precede L in "S U L". Since NR 0x15's construction/power-on default is 0x08 (bits 4-2 = %010), every Next-mode session that never explicitly wrote NR 0x15 was affected. Found while building T-30's three-layer priority decode; fixed alongside a related test bug (TestT29NextModeLayer2EndToEnd wrongly assumed a 0x00 construction default for NR 0x15 rather than the real 0x08).

## [0.7.1] - 2026-09-15

### Added

- **Next-mode Layer 2 graphics: 256x192 8-bit-colour framebuffer,
  composited against the ULA layer (T-29, wave 6, partial).** New
  layer2.go implements the base Layer 2 mode per
  NEXT_SUPPORT_DEVELOPMENT_PLAN.md's ti0.r3 scope: NextREG 0x12/0x13
  select the visible/shadow framebuffer base bank (a corrected reading
  of the SpecNext wiki -- these are base-bank registers, not clip-window
  registers as this repository's own earlier T-29 note had it), the
  already-implemented NR 0x18 clip window (T-27) is now genuinely
  consumed rather than just stored, and NR 0x70 controls a palette
  offset. Compositing decodes NR 0x15's real bit-4:2 six-way priority
  table and reduces it correctly to a two-layer ULA/Layer2 ordering
  question in the absence of a sprite layer. `DecodeDisplay` -- the one
  call site both the GUI and headless front ends already use -- now
  composites Layer 2 in automatically when Next mode has it active, and
  is provably byte-identical to its pre-T-29 form on every classic
  48K/128K/+3/TS2068 machine and on a Next machine that hasn't enabled
  Layer 2. Deferred, and not part of this change: the 320x256/640x256
  colour modes, a real 256-entry 9-bit Next palette (T-31 -- Layer 2
  pixels currently map down to the existing 16-colour palette as a
  documented interim measure), and IO port 0x123B (CPU-address-space
  Layer 2 paging and the shadow/visible select bit).
- **Layer 2 GPU fast path for the GUI's live rendering.** `renderLayer2GPU`
  (videorender_gpu.go) draws Layer 2 directly via raylib texture blits,
  restricted to the NR 0x18 clip rectangle, reusing the existing
  16-entry `paperColourGPU` texture cache rather than baking a new one
  keyed on Layer 2's raw pixel bytes -- the palette-offset register (NR
  0x70) can change which colour a raw byte resolves to at runtime, so a
  bake-once cache keyed on the raw byte would go stale, while keying on
  the already-resolved colour index doesn't. This is an additional pass
  after whichever path already drew the ULA layer, not a replacement for
  it, and it skips drawing entirely (zero extra GPU calls) whenever
  Layer 2 is disabled or NR 0x15 puts the ULA layer on top. The portable
  `DecodeLayer2`/`CompositeLayer2` CPU path is unchanged and still used
  by headless screenshot capture and as the GUI's own fallback before
  its texture cache is ready.
- **Shared colour-resolution seam and a palette-version counter, ahead of
  T-30 sprite work.** Per docs/proposals/next-gpu-compositing-seam.md's
  Phase 1: `layer2Pixel`'s inline raw-byte-to-colour arithmetic is now a
  standalone `resolveNextPixel(raw, paletteOffset)` function so T-30's
  sprite pixel path can call the same seam instead of inventing its own
  copy, and a new `nextPaletteVersion` counter on `SpectrumIO` increments
  on every NR 0x70 write (both the live-write path and the
  snapshot-restore path), giving future GPU texture caches keyed on
  resolved colour a cheap staleness check once one actually needs it. No
  behaviour changes from this refactor alone -- verified by a dedicated
  test confirming only NR 0x70 writes bump the counter, and by the full
  existing Layer 2 test suite passing unchanged.

### Fixed

- **A comment in nextreg.go describing NR 0x15's stored default value
  incorrectly.** The line documenting register 0x15's power-on default
  (0x08) claimed it decoded to the priority ordering "S L U"; direct bit
  arithmetic (0x08's bits 4-2 = %010) actually decodes to "S U L" per
  the wiki's own priority table. The stored default value itself is
  unchanged -- only the comment describing what it means was wrong, now
  corrected, and the discrepancy between this stored default and the
  wiki's documented real-hardware reset value (0x00) is flagged rather
  than silently resolved.

## [0.7.0] - 2026-09-15

### Added

- **Next-mode Extended MMU: NextREG- and legacy-port-driven 8K-page
  paging now actually pages memory (T-28).** NextREG registers
  0x50-0x57 (via both a raw port write and the NEXTREG opcode) now
  genuinely drive which of the 224 available 8K pages each of the 8
  Next MMU slots serves, rather than only updating a readback mirror
  with no effect on memory access. On a genuine Next-mode machine, the
  classic 128K/+3 paging ports (0x7FFD, plus the new 0xDFFD extended
  bank port) also recompose the Next MMU's slots 6/7 to match, mirroring
  real hardware's own "the legacy ports have no map of their own, they
  recompose the same eight slots" behaviour. Slots 0/1 additionally
  reproduce a real-hardware special case: a reserved-range page number
  there re-engages the classic ROM selection instead of going inactive,
  matching how slots 2-7 behave differently from slots 0/1 on real
  Next hardware. All of this is new, additive code with zero effect on
  any classic 48K/128K/+3/TS2068 machine -- Next mode is not yet
  reachable from any `-model` option, so nothing user-facing changes
  yet; this closes out the memory-model groundwork the eventual Next
  machine mode will build on.

### Fixed

- **A latent port-decode collision between the new 0xDFFD support and
  the existing AY-3-8912 register-select port.** 0xDFFD's coarse
  address-line decode happens to match the AY register-select mask
  (normally targeting 0xFFFD). Harmless before this release since
  nothing gave 0xDFFD any meaning, but would have made a Next-mode
  0xDFFD write also silently corrupt the currently-selected AY
  register. Fixed with a narrow, address-exact exclusion that is a
  provable no-op on every classic machine.

## [0.6.15] - 2026-09-15

This minor version (0.6.0 - 0.6.15, 2026-08-25 to 2026-09-15) made tape
loading accurate against real hardware and laid the first groundwork for the
Next. Its per-patch detail is in
[`docs/CHANGELOG-ARCHIVE.md`](docs/CHANGELOG-ARCHIVE.md); in summary:

### Added

- Tape/ULA accuracy: precise 48K ULA contention, a floating bus, EAR idle
  feedback, and a coherent frame timebase, with permanent cross-emulator
  trace instrumentation and a tape-corpus harness (`contrib/tape-corpus-harness/`)
  to measure real-world loading coverage.
- ZX Spectrum Next: the NextREG port handler and the Tier 0 support planning
  that the later Next work built on.

### Fixed

- TZX pause blocks now sit LOW per the spec; ULA port reads return bits 5 and 7
  high; the GUI `-tapemode` accepts `turbo`; and a sandbox-only absolute path in
  `fdc_read_test.go` was removed.

## [0.5.1] - 2026-08-24

### Fixed

- **T-19: Turbo tape mode failed on 128K+ titles** (see RESOLVED.md for
  full detail). Root cause: the flat, block-boundary-synced 64K memory
  snapshot could not represent a game that actively pages RAM/ROM banks
  via port 0x7FFD *during ongoing code execution*, not just between
  tape blocks -- confirmed with Cybernoid 2 (47 paging writes in the
  first 4000 turbo frames alone). Replaced with a page-based model
  matching SpectrumMemory's own real layout (12 independently-addressable
  16K pages: 4 ROM banks + 8 RAM banks), synced immediately on every
  paging port write via a new zen80 hook (`FastPortWriteOut`, mirroring
  the existing `FastPortReadIn`) rather than only at block boundaries.
  Verified: Cybernoid 2 now reaches real game code and produces a
  screenshot pixel-identical to accurate mode's own ground truth, via
  the real CLI end to end; Chase HQ (48K) re-verified correct after the
  rewrite. Scope: standard 128K/+2/+3 paging only, not +3's special
  all-RAM mode or TS2068's separate Extension-ROM scheme (both rare for
  tape-based titles).

## [0.5.0] - 2026-08-24

### Added

- **Full TZX block coverage** via zentools' now-complete 25-block-type
  `pkg/tzx`/`pkg/tap` (see zentools 0.6.0): `loadTAP`/`loadTZX` migrated
  to delegate all structural decoding to `tap.Decode()`/`tzx.Decode()`
  rather than zenzx's own, previously partial parsing. `TapeBlock`
  extended with `HeaderType`/`HeaderName`/`DataLength`, populated at
  load time for both `.tap` files and TZX 0x10 blocks via
  `tap.DecodeBlock()`.
- **0x11/0x12/0x13/0x14 wiring**, both fast-load (`Blocks[]`, for
  trapLoad's ROM-address interception) and accurate-mode playback
  (`Pulses[]`, including `genCustomPulses` for 0x11/0x14's per-block
  custom pilot/sync/data timing). **0x2A Stop-The-Tape-If-In-48K**
  wired (`StopPointsIf48K`, checked conditionally on `!is128K`).
- **`cmd/tapereport`**: an HTML tape-compatibility report generator
  (ZSP design system, embedded ZX-Spectrum-style font) -- run against
  the newdiv corpus (303 games, 413 TZX files): 100% fully playable
  (accurate mode) and 100% fully fast-loadable, both figures confirmed
  non-vacuous by deliberately removing 0x2A from the wiring map and
  observing the playable figure correctly drop to 96.9%.
- **`wait-tape-done` zenscript verb**: blocks a script until
  `!zx.tape.st.Playing` or a timeout, falling through either way.
- **Turbo tape mode** (`-tapemode=turbo`): identical correctness to
  accurate mode's real pulse-level EAR-pin emulation (unlike fast
  mode's ROM-trap, which only accelerates while the ROM's own
  `LD-BYTES` is running -- ineffective the moment a game's own custom
  loader takes over, which is common). Two independent layers:
  - **`loadingFrame`**: a leaner `RunFrame` used only while a tape is
    actively playing in Turbo mode -- identical CPU/tape stepping, with
    per-instruction AMX-mouse/audio bookkeeping and per-frame
    border-history rendering skipped (irrelevant while nothing is
    rendering intermediate frames anyway). ~20-27% faster on its own,
    zero correctness risk since nothing is semantically shortened, only
    bookkeeping that has no effect on CPU or tape state is skipped.
  - **zen80's `FastMem`/`FastPort`** (see zen80 0.3.0): a flat 64K
    scratch memory/port array zenzx snapshots real `SpectrumMemory`
    into (`turboSnapshotIn`), runs the CPU against directly (avoiding
    interface-dispatch overhead -- ~3.6x on memory access alone), and
    reconciles back per-block (`turboSyncBlock`, keyed off `loadTZX`'s
    new `BlockBoundaries` tracking) rather than only at the very end --
    correct for the overwhelmingly common case of a 128K loader
    switching RAM banks between blocks, not mid-block. A `FastPortReadIn`
    hook supplies the ULA's live keyboard+EAR-bit port value (which a
    static array can't cheaply represent, given how often the EAR bit
    toggles) by reading `Tape.EarLevel` directly on the rare event (an
    actual `IN`), not the frequent one (an EAR transition).
    `Scheduler.BlockingOnRealMemory()` pauses the fast path whenever a
    barrier that reads real screen/memory content (`wait-boot`,
    `wait-screen`, `wait-attr`, `wait-mem`) is currently active, since
    such a barrier has no way to observe progress in the fast path's
    own unsynced scratch memory.
  - **Confirmed correct for 48K** (`Chase HQ`, an Ocean/"Paul Owens
    loader core" title -- a documented, one-byte-different variant of
    the standard ROM's own bit-sampling routine, confirmed via
    <https://sinclair.wiki.zxnet.co.uk/wiki/Loading_routine_%22cores%22>):
    byte-identical outcome to accurate mode via both direct screenshot
    comparison and a full memory diff at the same checkpoint, at a
    measured ~17-27% wall-clock speedup over accurate mode across
    several runs.
  - **128K known broken -- filed as T-19**, not yet root-caused. Do not
    ship `-tapemode=turbo` for 128K+ titles until resolved; 48K is safe.

### Fixed

- **TAP inter-block pause**: was a flat `pause × 3500` T-states between
  blocks; corrected to the proper 3-pulse `addTZXPause()` pattern
  (`{945, 3500, 3500×ms}`), matching both SpecIDE's and libspectrum's
  independent implementations, which both insert this exact pattern
  after every TAP block.
- **`EnqueueText`'s K-mode keyword typing**: was typing keywords
  letter-by-letter even in 48K K-mode, where a keyword should be a
  single keypress (`LOAD` -> J). `keywordKeys` map added, currently
  covering `LOAD`; extensible for others as needed.
- Five zentools `pkg/tzx` bounds-checking gaps (see zentools 0.6.0)
  found via corpus/fixture testing during this work; fixed upstream.

### Removed

- Seven leaked Amstrad CPC ROM files from `rom/` (unrelated to any
  Spectrum model this project targets).

## [0.4.65] - 2026-08-21

This minor version (0.4.0 - 0.4.65, 2026-06-29 to 2026-08-21) built the GUI's
menu bar and theming, the scripting assertions, and the reference manual. Its
per-patch detail is in
[`docs/CHANGELOG-ARCHIVE.md`](docs/CHANGELOG-ARCHIVE.md); in summary:

### Added

- The GUI menu bar: Machine (models grouped by manufacturer, switched live),
  Custom ROM, Tape, Floppy Disk (with "Open DSK Image..." and a live controller
  state), Snapshot, View, and Theme, with checkmarks, checkboxes, submenus,
  group headings, and the logo/rainbow dropdown (Help, About, homepage, fixed
  bar). The Machine menu's layout became configurable via `machines.json`.
- Switchable themes (Dark, Light, Spectrum) with a bundled-BDF-font submenu and
  X1/X2/X3 font zoom, plus a `-theme` flag.
- `docs/MANUAL.md`, the organised reference with full CLI tables; zenscript
  memory assertions (`wait-mem`, `expect-mem`, `sym`) and `shot zoom=N`; and the
  `go-sanity` release gate.

### Fixed

- Hi-colour FLASH honoured on real hardware (T-18); menu options could fire
  before the bar finished unrolling; and numerous theme, menu-layout and
  menu-bar reveal refinements against direct feedback.

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
