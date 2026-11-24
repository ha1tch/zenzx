# The workbench — proposal

Updated: 2026-11-17

Design intent for a development workbench inside ZenZX: the running
machine as the thing being edited. Written to keep the ideas of the
discussion that produced it in one place, before any of it is designed in
detail. Nothing here has been implemented, and nothing here is a
commitment to an order beyond the foundations in the last section.

## The idea

The usual loop is source, build, load, debug. The workbench turns it
round: the machine is running, any part of its memory can be looked at as
source, changed, and assembled straight back, and it goes on running.
Files (a snapshot, a tape, a `.nex`) are exports of the state the machine
is in. It starts from what CSpect's debugger gives a developer
(breakpoints, stepping, watches, symbols from a map file) and is not only
a debugger.

## What was asked for

| # | Idea | What it means |
|---|---|---|
| 1 | A debugger of CSpect's kind | Breakpoints, stepping, watches, registers and memory, names from symbol files |
| 2 | Assemble to hot memory | Source assembled into the running machine's memory, with no file and no reload |
| 3 | Save from the hot state | Snapshots, tapes and `.nex` files written from the machine as it is at that moment |
| 4 | A visual memory map | The machine's memory drawn as blocks, by what each holds |
| 5 | A block is its source | Clicking a block shows the source for that region; changing the source and assembling puts it back |
| 6 | Dragging a block relocates it | A block of code moved to another address is relocated, and the symbol tables are updated |
| 7 | Block types, found and forced | Code blocks are detected automatically where they can be; for the rest the type of a block and its symbols can be set by hand |
| 8 | Several machines at once | More than one machine running in the same program. In the GUI since 0.29.0: the dock at the bottom edge adds machines and flips between them |
| 9 | Diffing machines | Two machines, or one machine at two moments, compared |
| 10 | Sprites and images edited in memory | A power user edits the sprites, tiles, fonts and screens of the running program where they are, with editors brought over from zenplay and extended |

## What was added

| # | Idea | What it means |
|---|---|---|
| 11 | The emulator classifies memory | Every byte that was executed, read or written is recorded as such. The map colours itself as code, data or untouched, and disassembly stops guessing: this is the automatic half of 7, from the side that knows |
| 12 | Who wrote this byte | The last instruction to write each address is kept: from a variable to the code that set it in one step |
| 13 | Rewind | Periodic snapshots and the recorded input between them, so that a machine can be stepped back to before the fault |
| 14 | A data block opens as what it is | A screen, sprites, tiles, a palette, text, a table of words: shown as that. Editing them is 10 |
| 15 | State diff to find things | Two moments compared to find what changed (the lives counter), and the address named there and then. A use of 9 |
| 16 | A patch journal | Every hot edit recorded, undoable, and exportable as a patch |
| 17 | Export as a project | The whole machine as zenas sources and a build script that rebuild to the identical snapshot, checked as round trips are checked now |
| 18 | Tests against the live machine | zenas's `.EXPECT` and `.MATCH` assertions run on the hot state |
| 19 | A hot `.nex` with a stub | A NEX file has no registers and no hardware state, so one saved from a running machine gets a small stub that restores registers, paging and the Next's registers and jumps back in |
| 20 | A free-space finder | Where a block that has grown can go, or a jump out to a trampoline when it cannot stay |

## Diffing machines

Asked for last and worth its own section, since several of the others
are cases of it.

- **Two machines, live.** Their memory side by side, the bytes that
  differ marked, by block. Registers, paging and the Next's registers the
  same way.
- **One machine, two moments.** A snapshot kept as a reference and
  compared with the machine now: what a routine changed, what a frame
  changed.
- **Divergence.** Two copies started from the same snapshot, one of them
  patched, run in step until their state first differs: the instruction
  and the address where the patch first made a difference.
- **Finding a variable.** Successive diffs narrowed by what is known
  ("it went down by one"), the search cheat tools do, ending in a name
  in the symbol table (15).
- **Across models.** The same program on a 48K and on a Next, or on two
  board issues, to see where they part.
- **As a test.** A machine compared with a stored state as a pass or a
  fail, for a headless run.

What a diff compares has to be chosen: memory by physical address,
since two machines can page the same bank differently, and state that is
only timing (the frame counter, the floating bus) left out unless asked
for.

## Editing sprites and images in memory

zenplay has the editors: sprites with their animation frames, tiles and
their properties, characters, fonts, rooms painted from tiles and a map
of rooms (`internal/shell`: `spritesview.go`, `spritesanim.go`,
`tilesview.go`, `charactersview.go`, `fontsview.go`, `roomspaint.go`,
`mapview.go`). They are built from zenui's `PixelCanvas`, `CellGrid`,
`ToolPalette`, `Swatches` and `ZXClassicPaletteChooser`, which ZenZX has
too since it moved onto zenui, so what is brought over is the editors'
own logic and not a second toolkit.

They edit zenplay's project model. For the workbench they would be
extended and improved:

- **Bound to memory.** An editor opens on a block of the map (4, 14) and
  what it changes is written to the machine, through the same patch step
  as code (2) and into the same journal (16), so the running program
  shows the change in its next frame and the edit can be undone.
- **Told the layout.** Memory does not say that a block is a sprite
  sheet: width, height, how many, bits per pixel, whether a mask is
  interleaved, row order. The user says so once for a block and it is
  kept with the block's type (7), as its symbols are.
- **The Next's formats.** zenplay's editors are for the classic
  Spectrum's pixels and attributes. The Next needs 4 and 8 bits per
  pixel sprites and tiles, 9-bit palettes edited as palettes, Layer 2 in
  its three sizes, tilemaps, and sprite attributes; zentools' `pkg/zxn`
  already reads and writes those.
- **In and out.** A block exported as a PNG or a `.cut` collection and
  imported from one, with zentools, so that art made elsewhere can be
  dropped into a running program.

The same editors, bound to a file and not to a machine, are zenplay's
own: the extension should leave one set that both programs use, which
points at zenui or a package beside it as their home, not at a copy in
each.

## What is hard

- **Relocation (6).** Moving code is sound only where its references are
  symbolic. A block the workbench assembled is re-assembled at its new
  origin and whatever points at it is resolved again. A block that was
  only disassembled has `LD HL,0x8000`, which is an address or a number:
  relocate what can be proved, and show a review list of the rest. What
  the emulator recorded (11) narrows the list: a value that was used as
  an address is one.
- **Hot patching (2, 5).** A write lands at a frame boundary. It is
  refused, or the addresses are remapped, when the program counter or a
  return address on the stack is inside the bytes being replaced. Code
  that grows needs room (20).
- **Saving from the hot state (3).** A snapshot is the machine's state
  and is direct. A tape needs a choice of blocks and a loader. A `.nex`
  is a cold-start format (19). The `.szx` and `.z80` formats do not hold
  a Next's state.
- **Banked memory.** The map is of physical memory (4 MB on an issue 5
  board, in 8 KB pages) with the MMU's view on top, and a symbol needs
  its physical address to be put in the right bank.
- **Several machines (8).** The emulation holds its state in the
  machine; what is shared is the host's:

| Shared | To decide |
|---|---|
| Sound | One audio device: the machines mixed, or only the focused one heard |
| Keyboard, joystick, mouse | Input goes to the focused machine |
| Display | Each machine draws into its own frame; the window places them |
| Clock | Each runs its own frames; a paused or stepped machine does not stall the others |
| Files | Two Nexts must not write one SD card image |

## What there is to build on

[`../ZENBENCH.md`](../ZENBENCH.md) names the whole base that ZenZX and
zenplay share, of which these are the parts the workbench needs.

| Need | What exists | What is missing |
|---|---|---|
| A machine that can be held, copied and run beside another | The emulation, importable as `package zenzx` since 0.28.0 and tested with two machines side by side | The `Machine` facade a host builds, loads and drives a machine through: the rest of T-39 |
| Memory to source | zendis: `api.Disassemble`, data kinds, symbol tables, NEX input | Names on data operands (zendis T-16), physical addresses (zendis T-17), following the program's flow (zendis T-06, which 11 does from the emulator's side) |
| Source to memory | zenas's `assembler` package, in memory; `loadbin.go`, which writes bytes into a running machine | The patch step with its checks |
| Saving | zentools' snapshot, tape and NEX writers; zenas's `pkg/artifact`; `snapshot_adapter.go`, which turns a 48K or 128K machine's state into zentools' neutral snapshot and back | The same for a Next's state, and the paths to a tape and a NEX file |
| Names that follow edits | zentools' `symfile.Table` | A table that can be changed and that keeps its history |
| Windows, lists, panes, text fields | zenui 0.2.0, which ZenZX is on since 0.27.0 | A source editor, and the memory map itself |
| Sprite, tile, font and screen editors | zenplay's, for its own project model; zentools' `pkg/zxn` and `pkg/scr` for the formats | The editors bound to memory, told a block's layout, and extended to the Next's formats (10) |
| zenplay's Run page | A frozen copy of this machine in zenplay | The same library (T-39), which ends that copy |

## Order

T-39 first: the workbench, several machines, diffing them and zenplay's
Run page all stand on a machine that is a value. Then the memory map
with what the emulator recorded (4, 11), a block as its source (5) and
assembling back an edit that does not change a block's size (2): the
model of regions that everything else hangs from. The rest is not
ordered yet.
