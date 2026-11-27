# Zenbench

Updated: 2026-11-17

Zenbench is the name for what ZenZX and zenplay have in common: the
machine, the tools and the user interface parts that both programs are
assembled from. It is a name for a set of libraries, most of which
exist. It is not a program, and it is not a module of its own; whether
part of it becomes one is among the questions at the end.

ZenZX is an emulator that is growing a workbench
([`proposals/workbench.md`](proposals/workbench.md)). zenplay is a game
maker with a page that runs the game. Seen from Zenbench they are two
front ends over one base: ZenZX points it at a running machine, zenplay
at a project file.

This document is kept in ZenZX's repository until Zenbench has a home of
its own.

## The parts

| Part | What it is | Where it lives | State |
|---|---|---|---|
| CPU | The Z80 and Z80N core | zen80 `z80` | Used as it is by both |
| Machine | The Spectrums, the TS2068 and the Next: memory, video, sound, tape, disk, SD card | ZenZX's root package | Importable since 0.28.0 as `github.com/ha1tch/zenzx`, built with `-tags headless,purego`; two machines run side by side in its tests. What an embedder can reach is still small: the facade over models, snapshots and input is the rest of ZenZX T-39. zenplay carries a frozen copy from the 0.9 line, `internal/zxspike`, which that ends |
| Formats | Tapes, snapshots, NEX files, screens, the Next's graphics, ZX0, symbol files, BDF fonts | zentools `pkg/*` | Shared |
| Assembler | Source to bytes in memory; build tags; symbol files; packaging as a tape, a snapshot or a NEX file | zenas `assembler`, `pkg/artifact` | Shared since zenas 0.16.0 |
| Disassembler | Bytes to source, with names and data kinds; NEX input | zendis `pkg/api`, `pkg/disasm`, `pkg/render`, `pkg/load` | Shared since zendis 0.8.0. ZenZX does not use it yet; the workbench will |
| Widgets | Menus, dialogs, lists, tabs, text fields, canvases, the text reader, the progress bar; the raylib adapter; the palettes | zenui, `zenui/zenuiraylib`, `zenui/zxpalette` | Shared: ZenZX since 0.27.0, zenplay and zenimate before it |
| Fonts | The BDF faces and their loading | ZenZX `pkg/fonts`, read with zentools' `pkg/bdf`, drawn by `zenuiraylib` | Shared in fact: zenplay imports it from ZenZX |
| User settings | Theme, face and zoom in a `settings.json` | ZenZX `pkg/settingsconfig` | Shared in fact, in the same way |
| Art model | Sprites and their frames as data | zenimate `pkg/model`; zentools' `.cut` collections | Two forms; zenplay's register has settling on one (its T-04) |
| Editors | Sprites and animation, tiles, characters, fonts, rooms, the map | zenplay `internal/shell` | zenplay's only, for its project model. To be shared and bound to memory as well (workbench idea 10) |
| Application frame | The top bar, navigation, panes, focus and layout of a window with pages | zenplay `internal/shell`; ZenZX has its menu bar and no pages | zenplay's only. The workbench needs a frame of this kind |
| Workbench services | Regions of memory and their types, the patch step and its journal, a symbol table that changes, saving from a machine's state, diffing machines | Nowhere yet | Proposed ([`proposals/workbench.md`](proposals/workbench.md)) |

## How they stand on one another

```
            ZenZX (emulator, workbench)        zenplay (game maker)
                        \                          /
        ----------------------- Zenbench -----------------------
        editors, application frame, workbench services (to come)
        widgets: zenui            fonts and settings (in ZenZX)
        assembler: zenas          disassembler: zendis
        machine: ZenZX's, as a library (T-39)
        formats: zentools         CPU: zen80
```

Each line uses the ones below it and none above. The two programs keep
what is theirs alone:

| Program | Its own |
|---|---|
| ZenZX | Its window and menu bar, the command line, the headless program and ZenScript, the Next's first-start dialog and card handling, the workbench's own windows |
| zenplay | The game model (`pkg/game`), the project file, the engine in Z80 assembly, the exporter, its pages |

## The rules it already follows

These are decisions the projects have each taken; Zenbench is where they
meet.

- **One copy of each thing.** A part is extracted once and imported, not
  copied: zenui was the first, the machine is next.
- **Libraries, not processes.** The emulator runs inside the program
  that uses it, and the assembler and disassembler are called, not run.
- **A machine is a value.** No state shared between machines, the
  display and the sound as sinks the host supplies, so that a program
  can hold several (T-39's acceptance).
- **No cgo for an application that embeds it.** zenplay builds with
  `CGO_ENABLED=0` and `-tags purego`; what Zenbench offers has to build
  that way. ZenZX's own window may use cgo.
- **Widgets draw nothing themselves.** A `Renderer` and an `Input` each
  frame, so that everything is testable without a display.
- **Text is the BDF font system.** The faces of `pkg/fonts`, zentools'
  reader, zenui's renderer; raylib's own text is not used.
- **Round trips are checked.** What is written is read back and
  compared: assembler to disassembler, encoder to decoder, a picture
  before and after a change.

## A second pass: what else belongs here

The table of parts above is what was already seen as common. This
section is from reading both trees for what is in one place and wanted
in the other, or in both places at once. ZenZX's root package is 91
files and about 27,200 lines (without tests); zenplay's own code is
`internal/shell`, `internal/app`, `pkg/game`, `pkg/projectfile` and
`internal/oakexport`. File names are given so that each claim can be
checked.

### The same thing in two places

| What | In ZenZX | In zenplay | Where one copy would live |
|---|---|---|---|
| The machine | The root package | `internal/zxspike`, a copy from the 0.9 line | The root package itself, `github.com/ha1tch/zenzx` (T-39) |
| Appearance: the Theme and Font menus, zoom, applying and saving them | `menubar_gui.go` (`ApplyTheme`, `ApplyFont`, `ApplyZoom`, saving to `settings.json`) | `internal/shell/shell.go` (the same menus, "laid out as zenzx lays out its own"), `settingsview.go`, `internal/app/settings.go`, which embeds a copy of ZenZX's shipped `settings.json` | With the widgets, beside fonts and settings |
| The classic 16 colours | `display.go` (`ZXPaletteRGBA`, raylib colours) and `videorender.go` (`zxPalette`) | Uses `zenui/zxpalette` | `zenui/zxpalette`; ZenZX's two tables go |
| The display's geometry: 256 by 192, the border's widths | `display_constants.go` | `internal/shell/runview.go`, which says it repeats them "until the machine library exports them" | The machine library |
| The frame loop of a hosted machine: 50 frames a second whatever the window's rate, catching up after a stall | The loop in `zenzx_gui.go` | `internal/app/machine.go` (`pacer`) | A machine host beside the machine library |
| Symbol tables | `symtab.go`: a pasmo `.sym` reader for ZenScript's `sym` verb | Not used yet | zentools' `symfile.Table`, which reads five formats; `symtab.go` goes |
| Reading an address written as text | `loadbin.go` (`ParseAddr`) | — (zenas has `artifact.ParseAddress`, zendis its own) | zentools, once, for all four |
| Wrapping text | `zenui.WrapText`, by characters, which came from here | `internal/shell/wrap.go`, by pixel width | zenui, with both measures |
| The Help and About window | `zenui.TextModal` since 0.27.0 | `internal/shell/aboutmodal.go` | zenui (its T-08) |
| Reading the text on a screen against the ROM's font | `screen_read.go`, from a machine's memory | — (zentools `pkg/scr` has `ocr.go`, from a screen file) | zentools' recogniser, given the machine's screen bytes |

zenimate is a third program on the same base and has its own copies of
some of these (`helpmodal.go`, `osd.go` beside `zenuiraylib.OSD`,
`bdftext.go`, `filltool.go`); it is not surveyed here.

### In ZenZX, wanted by zenplay or by the workbench

| What | Where | Why it is Zenbench's |
|---|---|---|
| ZenScript: scripts that drive a machine, wait on memory and on the screen, take screenshots | `script.go`, `scheduler.go`, `keyboard_script.go`, `bootdetect.go`, `bootbaselines.go` (about 2,200 lines with the two above) | zenplay's engine tests with golden frames (its T-13) are this; so are the workbench's tests against a live machine (idea 18). It goes with the machine library, as a package beside it |
| Machine state to and from zentools' neutral snapshot | `snapshot_adapter.go` (`toMachineState`, `fromMachineState`) | This is the path from a running machine to zentools' writers that saving from the hot state needs (idea 3). The workbench proposal lists it as missing; for the 48K and 128K families it exists. A Next's state has no such form yet |
| Writing bytes into a running machine and starting them | `loadbin.go` (`LoadBIN`, `LoadBINWithSafeReturn`, `LoadSCR`) | The beginning of the patch step (idea 2) |
| Looking inside: the trace harness, the NextREG write log, the Next's state dump, the snapshot debugger | `zzz_trace_harness_test.go`, `zzz_trace_callstack_test.go`, `zzz_trace_symbols_test.go`, `nextreglog.go`, `nextdump.go`, `snapshot_debug.go`; `proposals/zen80-tracing-hooks.md` | The workbench's services start from these: what was executed, read and written (idea 11), who wrote a byte (12). The harness is test code today |
| The host's keyboard, gamepad and mouse mapped to the machine's | `input.go` (845 lines, tied to raylib), `joyports.go`, `joystick.go`, `mouse.go` | zenplay's Run page needs the same mapping. The mapping itself has no need of raylib and would be split from the reading of the keys |
| The picture as pixels, without a window | `videorender.go`, `nextframe.go`, `display_core.go` (the `Display` interface) | The display sink of the machine library; what a host draws, and what a diff of two machines' screens compares |
| Sound: the AY, the mixer and resampler, the choice of backend | `ay8912.go`, `audio_common.go`, `audio_sinc_filter.go`, `audio_engine.go`, `audio_selector.go`; `audio_oto.go` and `audio_device_oto.go` are the output, behind the `AudioDevice` interface | The audio sink; an output that builds without cgo is zenplay's T-06 |
| The catalogue of machines | `pkg/machineconfig`, `machines.json` | zenplay's top bar chooses a target machine from the same list |
| Fonts and user settings | `pkg/fonts`, `pkg/settingsconfig` | Already imported by zenplay from the emulator |
| The Next's SD card as a file system: building a FAT32 card, adding files | `pkg/nextcard` (`fat32.go`, `fat32build.go`, `add.go`, about 780 lines for the file system alone), `pkg/nextdisk` | Format code, of zentools' kind. An exporter that puts a game on a Next's card, and the workbench saving to one, need it without the emulator |
| The +3's disk images | `dsk.go` | Format code likewise. The plus3 project has disk images of its own; the two were not compared |
| ZenZX's own snapshot format | `snapshot.go`, `snapshot_chunks.go`, `snapshot_formats.go` (about 1,800 lines) | A format, and so zentools', if any other program is to read a `.zxs`; otherwise the emulator's |
| A menu bar that slides away when it is not used | `menubar_gui.go` (the easing and the timings) | A behaviour of `zenui.MenuBar` any host could switch on; zenplay's bar is fixed |

### In zenplay, wanted by ZenZX's workbench

| What | Where | Why it is Zenbench's |
|---|---|---|
| The application frame | `internal/shell`: `shell.go`, `panes.go`, `layout.go`, `focus.go`, `pages.go`, `view.go`, `toolbar.go`, `navicons.go` | The workbench is a window of pages and panes |
| The editors | `spritesview.go`, `spritesanim.go`, `tilesview.go`, `tilesprops.go`, `charactersview.go`, `fontsview.go`, `roomsview.go`, `roomspaint.go`, `mapview.go` | Workbench idea 10 |
| What a pixel editor does to a picture: the picture model, flood fill | `pkg/game/picture.go`, `pkg/game/fill.go` | The shared editors need them apart from the game model. zenimate has drawing tools of its own to reconcile with |
| An 8 by 8 character set as data, with its presets | `pkg/game/fonts.go`, `fontpresets.go` | The font editor bound to memory edits a Spectrum character set or its UDGs in this form |
| The window that hosts a shell and a machine | `internal/app`: `app.go`, `machine.go`, `icons.go` | The machine host named above |
| The map of ZenZX's root package for its extraction, and the tool that measures it | `docs/MACHINE_MAP.md`, `docs/machine_clusters.json`, `tools/depmap` | They are about ZenZX and are the measured plan for T-39. They were taken at ZenZX 0.9.0, when the root package was 57 files; it is 91 now, so the map has to be run again. They belong with ZenZX, or with the tooling |

### What this pass leaves where it is

The Next's hardware (`next*.go`, `layer2.go`, `sprite.go`, the DMA, the
copper, DivMMC, the SD card's protocol) is the machine and moves with it
as a whole. The first-start dialog and the card menus are ZenZX's.
zenplay's game model, its project file, its engine and `oakexport` are
zenplay's. The release checks both carry (`check_changelog.sh`,
`check_archive.sh`) are tooling, not Zenbench.

## Versions today

| Part | Version | Published |
|---|---|---|
| zen80 | 0.5.7 | yes |
| zentools | 0.16.1 | to 0.8.6; later versions in the workspace |
| zenas | 0.16.1 | to 0.11.2; later in the workspace |
| zendis | 0.8.2 | in the workspace |
| zenui | 0.2.0 | yes |
| raycanvas | 0.3.1 | to 0.2.4; 0.3.1 delivered, to be pushed |
| ZenZX | 0.28.0 | in the workspace |
| zenplay | 0.0.3 | in the workspace |
| zenimate | 0.8.0 | in the workspace |

The zenplay workspace (`go.work`) is where the parts are built together
until they are tagged; its `make build vet test check-gui` is the check
that Zenbench holds together.

## What is to be done

In the order things depend on one another.

| Step | What | Where it is tracked |
|---|---|---|
| 1 | The machine as a library, with two machines run side by side as its test. Importable, silent and tested with two machines since ZenZX 0.28.0; the `Machine` facade for embedders remains | ZenZX T-39; zenplay T-05, T-20 |
| 2 | Sound that builds without cgo for an embedded machine | zenplay T-06 |
| 3 | zenplay's Run page on that library; `internal/zxspike` removed | zenplay T-07 |
| 4 | The Help and About windows of zenplay and zenimate on `zenui.TextModal` | zenui T-08 |
| 5 | The memory map and regions; a block as its source; assembling back | ZenZX T-65 |
| 6 | The editors shared, bound to a file or to memory, extended to the Next's formats | ZenZX T-65, idea 10 |

From the second pass, none of them tracked anywhere yet except where a
number is given:

| Step | What | Depends on |
|---|---|---|
| 7 | `docs/MACHINE_MAP.md` and `tools/depmap` brought to ZenZX and run on the present tree. Done in ZenZX 0.28.0 | Nothing; it is the first thing T-39 needs |
| 8 | ZenZX's two palette tables and `symtab.go` replaced by `zenui/zxpalette` and `symfile.Table`; one address parser in zentools | Nothing |
| 9 | ZenScript, the input mapping, the display and audio sinks and the state adapter extracted with the machine | Part of step 1 |
| 10 | Appearance (theme, face, zoom and their menus), fonts and settings in one place beside the widgets | The open question of that place |
| 11 | The FAT32 card code and the disk image code as zentools packages | Nothing |
| 12 | The frame, the pixel operations and the character set model out of zenplay | The open question of their home |

## Open questions

- **A home for what has none.** The editors, the application frame and
  the workbench services belong to neither program. They can go into
  zenui, which would then be more than widgets, or into a module of
  their own, which would be Zenbench in name too. Not decided.
- **Fonts and settings.** zenplay takes them from ZenZX, so the emulator
  is a dependency of a program that only wants its faces and its
  settings file. They may move beside the widgets.
- **One art model.** zenimate's model or zentools' collections (zenplay
  T-04) decides what the shared editors edit.
- **The frame.** Whether ZenZX's workbench takes zenplay's frame of
  pages and panes as it is, or the two are reconciled as the widgets
  were.
