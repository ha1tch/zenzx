# The GUI

The window, its menu bar, the files that configure it, its keys and what can be dropped on it. The flags are in [`MANUAL.md`](MANUAL.md).

## Top menu bar

`-theme=Dark|Light|Spectrum` (default `Dark`, case-insensitive) sets
the theme active from the very first frame, without needing to open
the Theme menu first -- when `-theme` isn't given at all, the theme
last saved to `settings.json`
is used instead of this flag's own default (see **Configuring the
Machine menu and persisted preferences** below).

Hover the mouse pointer within 10px of the top window edge and hold it
perfectly still for 100ms -- a menu bar slides in (eased in, not linear)
with eight menus. Once the bar is shown, hovering any label opens its
dropdown immediately -- no click needed -- and moving to a different label
switches directly to it, the same as a traditional desktop menu bar once
any menu is already engaged.

- **File** -- the menu a person coming from another program looks for
  first, and the one a system with no application menu (Windows, most
  Linux desktops) has no other place to quit from.
  - **Open...** -- a file browser for anything the program loads: tapes
    (`.tap`, `.tzx`), snapshots (`.sna`, `.z80`, `.szx`, `.zxs`), screens
    (`.scr`), discs (`.dsk`) and `.nex` files (which a Next runs, see "Running a
    .nex", below). What is chosen goes exactly
    where the same file dropped on the window would go (see "What a drop
    does", below), through the same code, so the two cannot differ; a file
    chosen here is a plain drop, with no Shift.
  - **Preferences...** -- a window with every choice the program keeps
    between sessions, in one place (below).
  - **Quit** -- ends the program the way closing its window does.
- **Machine** -- Reset, Pause/Resume, and every supported model,
  grouped by manufacturer under a small heading and separator each:
  **Sinclair Research Ltd** (the original 48k/128k), **Amstrad plc**
  (+2/+2A/+3), **En Español** (the Spanish-market variants -- which,
  unlike the other three groups, span three different manufacturers
  across their own history rather than one, so each model gets its
  own indented sub-heading naming its actual maker: **Sinclair**
  (48k, Sinclair's own design, distributed and locally assembled in
  Spain), **Investrónica** (128k, genuinely designed by Investrónica
  itself with Sinclair's permission, and launched in Spain months
  before the UK got it), and **Amstrad** (+2 and +3, the +2
  specifically the first computer launched under Amstrad's own
  ownership after its 1986 acquisition of Sinclair)), and **Timex**
  (the Timex Sinclair 2068) -- with a checkmark on whichever model is
  currently running. Picking a model switches the running machine
  live, without restarting: reloads the target's standard ROM set,
  resets, and resets the joystick to that
  model's own default.

  This grouping is entirely configurable, not hardcoded: it's loaded
  from `machines.json` at startup -- a file in the current directory
  if one exists and validates, otherwise the copy built into the
  binary (the layout described above). A hand-edited `machines.json`
  is validated against a schema (using
  [queryfy](https://github.com/ha1tch/queryfy)) before it's used; an
  invalid one is reported and skipped in favour of the built-in
  default, never treated as fatal. The file is a flat list of nodes:

  ```json
  {"type": "separator"}
  {"type": "title", "label": "Sinclair Research Ltd"}
  {"type": "model", "id": "48k", "label": "ZX Spectrum 48k", "indent": 1}
  {"type": "submenu", "label": "More", "items": [ ... ]}
  ```

  `separator` (a divider line), `title` (a small, non-selectable
  heading, `indent` optional), `model` (a selectable entry -- `id`
  must be one of the fixed `-model` identifiers the emulator actually
  supports, `indent` optional), and `submenu` (a hover-opened nested
  menu, capped at one level of nesting) can appear in any order and
  combination. `indent` adds two spaces of leading whitespace per
  level, purely visual. What can't change is *which* models exist:
  every one of the emulator's supported `-model` identifiers must
  appear exactly once across the whole file -- rearrange, retitle,
  and regroup freely, but nothing can go missing or get duplicated.
- **Custom ROM** -- pick a file from `-custom-roms-dir` and apply it
  live (same two-step ROM-then-bank flow as `-custom-roms-menu`, for
  models with more than one bank).
- **Tape** -- Play/Stop, Rewind, toggle Accurate/Fast mode, and **Show
  Info**, which opens a window with the tape's file, mode, state and
  blocks -- the first three the same actions as the `Alt+P/R/T`
  shortcuts (`Alt+I` still prints the tape's blocks to the terminal).
- **Floppy Disk** -- **Open DSK Image...** (a live file browser, +3
  and Spanish +3 only -- prints a message and does nothing on any
  other model rather than failing silently), Save, Insert Blank,
  Eject, Save As, and **Disk Info**, which opens a window with the disc's
  file, its format, whether it has unsaved changes and whether the file
  has changed on disk since it was loaded. The rest are the same actions
  as `F4`-`F8`.
  **Manage Disks...** opens the Floppy Disks window: the disc in memory
  against its file, and the folder's disc images. See "The +3's discs" in
  the manual.
- **SD Card** -- in the Floppy Disk menu's place while a Next that boots
  NextZXOS is running, which has no use for floppies. It shows the cards in
  the Next's two slots and changes the second: **Insert Card in Slot
  2...**, **Insert ZenZX's Card (Examples)**, **Remove Card from Slot 2**,
  **New Empty Card** (280 MB or 1 GB), **Card Info** (a window with each slot's file, size, whether it is write-protected, how much it holds and whether it boots a Next), and **Manage Cards...**, the SD Cards
  window: every card image, who has it, booting from another, copying. See "The second card" in the manual.
- **Snapshot** -- Quick Save/Load, Save Timestamped, **Snapshot Info** (a
  window that says how snapshots are loaded and saved), run diagnostics --
  the same actions as `F9`-`F12`.
- **View** -- **FPS Counter** and **Border Display** are checkboxes,
  showing current on/off state directly (wired to the display
  manager's own fields, so it's always correct whether last changed
  from here or via `Alt+F`/`Alt+B`), and clicking one keeps the menu
  open for toggling the other without reopening it. The border is the
  user's to choose only where nothing else decides it: in a tiled layout
  every machine is shown with its border, in a Layer 2 mode that fills the
  whole 320x256 frame (the 320x256 and 640x256 modes) the border is part of
  the picture, and in a mode with no border there is nothing to show. There
  **Border Display** is greyed and shows what is on the screen, `Alt+B`
  says why it does nothing, and the choice already made is kept for when
  the machine is back where it applies. The window's size follows the
  border however it came to change. **Show Status** is
  unchanged, the same action as `F3`. Below a separator, **X 1**/**X
  2**/**X 3** set the emulated display's own scale directly (the same
  scale `PgUp`/`PgDn` step through one at a time), with a checkmark on
  the current one and any scale too large for the current monitor
  disabled -- refreshed every frame the bar is shown, since the scale
  can change outside the menu entirely. Below another separator, the
  layouts for several machines: **One Machine**, **Side by Side**,
  **Stacked**, **Grid**, **Stack Left, Main Right**, **Main Left, Stack
  Right**; the scale items are greyed in a tiled one. **Rotate
  Machines** moves each machine to the next place. See "Several
  machines" in the manual.
- **Theme** -- switch the bar's (and every dropdown's) colour scheme
  live, with a checkmark on the current selection: **Dark** (the
  default) and **Light** (a macOS Aqua-inspired scheme) both show the
  Zen Spectrum Project's own logo on the right side of the bar, space
  permitting -- two colour-halves of ascending three-block "teeth",
  drawn as filled rectangles (not a bitmap), rotating through three
  colour arrangements every third of a second. **Spectrum** (the
  real Sinclair 128K/+2/+3 boot menu's own palette -- white panel,
  black text, bright cyan selection in the dropdown, grey separators,
  a darker standard-brightness green checkmark, and a fixed blue for
  checkboxes; a vertical gradient bar background (40% grey at the top
  to black at the bottom) with a second, horizontal gradient layered
  on top in multiply blend mode over the rainbow's own region --
  white at that region's left edge fading to black at the window's
  right edge, darkening the background there without affecting the
  rainbow itself, drawn on top of it; a 2px border with no top edge on
  the dropdown, since it always opens directly under the bar; tight
  item spacing plus extra bottom margin below the last row, matching
  the real menu's own compact rows) shows that title strip's own
  four-band diagonal rainbow instead, in the same bar slot -- red/
  yellow/green/cyan, measured pixel-by-pixel from an actual boot
  screenshot and independently confirmed against a purpose-built
  reference asset, drawn as adjacent 1px scanline segments (not a
  bitmap, and not lines -- filled rects, since raylib's own line
  drawing left a 1px gap between bands), whenever there's enough room
  for it. Below a separator, **Font** opens as its own submenu:
  switch the bar's (and every dropdown's) typeface live, with a
  checkmark on the current selection, among the bundled BDF faces --
  **Sinclair** (the default, the ZX Spectrum 48K ROM's own 8x8
  character set), **TomThumb**, **Spleen**, **Cozette**, **Creep**,
  and **HaxorMedium** -- then, below its own separator, **Zoom X1**/
  **X2**/**X3** (likewise checkmarked; unlike the font choice itself,
  this magnification factor applies only to dropdown text and layout,
  never the bar strip, which always draws at X1 regardless). X2 is
  the default, matching the size dropdowns have always drawn at.

The rainbow (Spectrum theme) or logo (Dark/Light themes) on the
right of the bar is the Help menu's button, and behaves as one, after the
Start button of the systems the menu's users come from: it is raised under the
pointer and pressed in while the menu is open, and only the logo (and the corner
of the bar beyond it) opens the menu, not the empty bar beside the labels. The
pointer is a hand over the logo. Click
it directly, no hover needed; the menu is right-aligned to the window's own right edge (moved a little
left where its submenu needs the room). It is in titled sections:

- **Help**
  - **Help** -- a scrollable reference covering every menu, keyboard
    shortcut, and command-line flag.
  - **About...** -- version, project link, and licence/attribution, and a
    pointer to the acknowledgements.
  - **Acknowledgements...** -- the people the project owes thanks to. It
    shows `ACKNOWLEDGEMENTS.md`, the file in the source tree, so there is
    one text of it; `https://github.com/ha1tch/zenzx/blob/main/ACKNOWLEDGEMENTS.md`
    is where to read it on GitHub (the reader cannot follow links).
  - **ZenZX website** -- opens the project's own page
    (`https://ha1tch.github.io/zsp/projects/zenzx`) in the default
    browser.
- **Tools**
  - **Developer** ▸ **Debugger** -- opens the debugger (`Alt+D`) on the
    front machine; in a window of several machines it hides the dock, which
    the debugger cannot share the window with.

### File > Preferences...

One window for the choices the program keeps between sessions, in titled
sections. Each row reads its value afresh every frame and changes it through
the same code the menu that sets it uses, so the window and the menus cannot
disagree, and a change is written to `settings.json` as it is made.

| Section | Row | What it does |
|---------|-----|--------------|
| Appearance | **Theme** | Dark, Light or Spectrum (the Theme menu) |
| Appearance | **Font** | The bundled faces (Theme > Font) |
| Appearance | **Font zoom** | X1-X3, the size of the menus' text |
| Window | **Display zoom** | X1-X3 (the View menu's X 1-X 3); greyed while machines are tiled, and a size that does not fit the monitor or the video mode is refused as the View menu disables it |
| Window | **Fixed menu bar** | Pins the bar permanently shown, bypassing the idle-at-edge auto-hide until unchecked -- the window grows taller to make room (animated, as toggling the border animates a resize) rather than overlapping the display, and shrinks back when unfixed. The space is reserved exactly while the bar is fixed, checked every frame, so a bar saved as fixed has its space from the first frame |
| Machines | **Machine dock** | Whether the dock may slide up from the bottom edge (and with `Alt+M`). Off, it stays away, and one that is open closes. On by default |
| Machines | **A loaded snapshot** | What a `.zxs`, `.sna`, `.z80` or `.szx` does once it is loaded by File > Open or a drop: **Waits paused**, for Play or `F2`, or **Plays at once**. Quick Load and `-snapshot` are not affected. Waits paused by default |
| Machines | **Several machines** | Whether the dock's **New** tile may add machines. Off, the tile is greyed and says so when used. Machines that `-multi` starts at launch are not affected. On by default |
| Files dropped on a Next at its menu | **A plain drop** | What a tape, screen or snapshot dropped on a Next sitting at its NextZXOS menu or Browser is done with: **Ask**, **Copy to SD card** or **Load and run**. Ask by default |
| Files dropped on a Next at its menu | **With Shift held** | The same, when Shift is down as the file is let go. Load and run by default |

Border display and the FPS counter are deliberately not here: they are session
diagnostics, kept in the View menu, and (as the settings file's own rule says) not
saved. Sound and the Escape-key mode are command-line choices (`-no-audio`,
`-escape`) and are not in the window.

The bar shows one thing at a time: a window or dialog has the frame to
itself, and opening the Help menu closes any dropdown that was open. That
rule is kept in one place, applied every frame, and not left to each way of
opening a menu to remember. Likewise each menu is written once, as a table
of rows that carries each row's label, state and action, and the items the
bar draws and the actions it runs are both made from it, so a row cannot be
given another row's action.

The emulator keeps running the whole time -- CPU stepping and the
Spectrum's own screen never pause while the bar or a dropdown is open.
Only keyboard input to the emulated machine is withheld while a
dropdown, dialog, the logo menu, or a modal is actually open -- the
bar being merely shown (or fixed) never withholds it by itself. Move
the mouse away from the bar (with nothing open, and not fixed) and it
slides back out the same way it came in.

The bar itself (`zenui.MenuBar`) and every dropdown it opens
(`zenui.Menu`) are general, renderer-agnostic components of
[zenui](https://github.com/ha1tch/zenui), the toolkit ZenZX shares with
zenimate and zenplay -- reusable by any host, not specific to this menu's
contents or to raylib. The Help and About window (`zenui.TextModal`) and
the progress bar of the Next's first-start dialog are zenui's too.

## Configuring the Machine menu (`machines.json`) and persisted preferences (`settings.json`)

The Machine menu's own layout -- which models exist, how they're
grouped, and under what headings -- comes from `machines.json`, not a
hardcoded list. A copy is bundled into the binary as the default; a
valid `machines.json` next to the executable overrides it entirely (an
invalid one is reported and the built-in default is used instead,
rather than treated as fatal). The file is a flat list of nodes:

```json
{"type": "separator"}
{"type": "title", "label": "Sinclair Research Ltd"}
{"type": "model", "id": "48k", "label": "ZX Spectrum 48k", "indent": 1}
{"type": "submenu", "label": "Amstrad plc", "items": [ ... ]}
```

- **`separator`** -- a thin divider line.
- **`title`** -- a small, non-selectable heading; `indent` (default 0)
  adds two spaces per level, for a sub-heading within a group.
- **`model`** -- a selectable entry. `id` must be one of the fixed
  `-model` identifiers (`48k`, `128k`, `plus2`, `plus2a`, `plus3`,
  `spanish48k`, `spanish128k`, `spanishplus2`, `spanishplus3`,
  `ts2068`) -- the file controls how models are presented and grouped,
  never which models exist, and every one of them must appear exactly
  once across the whole file.
- **`submenu`** -- a hover-opened nested menu, holding its own list of
  `separator`/`title`/`model` nodes (not a further `submenu` -- one
  level of nesting is the limit the menu bar's own selection reporting
  can express).

Groups, titles, and separators can be arranged in any order and
combined freely; `machines.json` at the project root is the actual
default configuration and the clearest example. Validated against a
[queryfy](https://github.com/ha1tch/queryfy) schema (`pkg/machineconfig`)
before use.

Theme, font, font zoom, display scale, whether the menu bar is
pinned shown, whether the machine dock and adding machines are allowed
(`"dock"`, `"multi"`: booleans, on when absent), and what a plain drop and a
Shift drop do on a Next at its menu (`"drop"`, `"dropShift"`: `"ask"`,
`"copy"` or `"run"`; absent they are `"ask"` and `"run"`) are saved to
`settings.json` (`pkg/settingsconfig`, same
embedded-default-with-disk-override pattern, also queryfy-validated)
every time one of them changes via a menu, so the next launch resumes
where the last session left off. `-settings=PATH` points at a
different file instead of the default `settings.json`. An explicitly
passed `-theme` or `-scale` flag always wins over whatever's
persisted; leave either unset and the persisted value is used instead
of that flag's own hardcoded default. The running `-model` and FPS/
border visibility are deliberately not persisted -- an explicit model
should never be silently overridden by a stale save, and FPS/border
are session-level diagnostic toggles, not the kind of preference most
applications carry across restarts.

## Runtime keyboard shortcuts (GUI)

While the emulator is running, these host keys control it. They are intercepted
by ZenZX and are not passed to the guest.

| Key | Action |
|-----|--------|
| `Esc` | Close the open menu or window; with none open, quit (unless started with `-noesc`) |
| `F1` | Reset the machine |
| `F2` | Pause / resume (a paused machine shows a Play button in the middle of its picture; clicking it resumes) |
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
| `Shift+F1` | Show a demo dropdown menu, live over the running emulator |
| `Shift+F2` | Play a demo animated notification, live over the running emulator |
| `Shift+F3` | Show a demo file-open dialog, live over the running emulator |
| `Page Up` | Increase window scale |
| `Page Down` | Decrease window scale |
| `Alt+F` | Toggle the FPS counter (off at start; drawn in the interface font) |
| `Alt+B` | Toggle the border (where it is the user's to choose: not in a tiled layout or a full-frame Layer 2 mode) |
| `Alt+P` | Play / stop the tape |
| `Alt+R` | Rewind the tape |
| `Alt+T` | Toggle tape mode (accurate / fast) |
| `Alt+I` | Print tape status and block list |
| `Alt+M` | Open or close the machine dock |
| `Alt+1`..`Alt+9` | With several machines (`-multi`), bring that machine to the front: the display, sound and keyboard pass to it, the same as clicking its tile. A digit past the number of machines does nothing. |

## Drag and drop

In the GUI build, dropping a file onto the window loads it into the machine by
its extension:

| Extension | Loads as |
|-----------|----------|
| `.scr` | A raw 6912-byte screen dump, written straight onto the framebuffer (6144 bytes of bitmap + 768 bytes of attributes). This only paints the screen — it does not change CPU state or memory, so it is a still image, not a running program. |
| `.zxs` | A ZenZX snapshot (the proprietary chunk format). |
| `.tap` / `.tzx` | A tape image (loaded, then driven with the `Alt+P/R/T/I` controls). |
| `.dsk` | A +3 disk image. On a +3 it is mounted in Drive A (requires the FDC). On a Next, real NextZXOS can mount disc images as its own drives, but ZenZX does not emulate that yet (T-72), so a dropped `.dsk` is copied to the SD card instead (see the per-machine matrix below). |
| `.sna` / `.z80` | Standard snapshots (48K and 128K), via the shared `zentools` library. |
| `.szx` | ZX-State snapshots, load only, via the shared `zentools` library. |
| `.nex` | A Next executable. It is not applicable to the classic models — only a Next can run it. On a Next that boots NextZXOS it is run by NextZXOS's own `.nexload`, started for the person (see "Running a .nex", below); it can also be copied to the SD card. |

A `.scr` can also be loaded headlessly with `-scr` (see above).

### What a drop does, per machine and per file type

The same drop is routed differently depending on the machine and, for a Next,
whether it is sitting at its NextZXOS menu or Browser (an *idle prompt*). The
default — dropping on the machine's screen — behaves as follows:

| File | 48K / 128K / +2 | +3 (with drive) | Next (running a program) | Next (at menu / Browser) |
|------|-----------------|-----------------|--------------------------|--------------------------|
| `.tap` / `.tzx` | load tape | load tape | load tape | **ask**: copy to SD card, or load and run |
| `.scr` | paint screen | paint screen | paint screen | **ask**: copy to SD card, or load and run |
| `.sna` / `.z80` / `.szx` / `.zxs` | restore snapshot | restore snapshot | restore snapshot | **ask**: copy to SD card, or load and run |
| `.dsk` | needs a +3 (toast) | mount & boot (RUN) | needs a +3 (toast) | **copy to SD card** (no prompt — ZenZX does not mount Next disc images yet, T-72) |
| `.nex` | not applicable (toast) | not applicable (toast) | **run** — after a question: the Next is busy, reset it and run it? | **ask**: copy to SD card, or run it |
| anything else | copied to a card/disc if one is present (see below), else ignored | as 48K | as 48K | as 48K |

The **ask** cases open a small dialog over the running machine with two buttons,
*Copy to SD Card* and *Load and Run*. Holding **Shift** at the drop is, by default,
the same as pressing *Load and Run*: it skips the dialog and runs the file at once
(so a quick "just run it" needs no click). Both halves are preferences
(File > Preferences..., "Files dropped on a Next at its menu"): what a plain drop does
-- **Ask** (the default), **Copy to SD card** or **Load and run** -- and what a drop
with Shift held does (**Load and run** by default; **Ask** and **Copy** are the other
two), so the modifier can be made to mean whichever is wanted. The choice applies to
tapes, screens, snapshots and `.nex` files, the files that have both outcomes; a `.dsk`
has only one on a Next, so it is copied whatever is chosen. The dialog's *Copy to SD Card*
goes to the slot the files were dropped on, not to whichever is under the button.
All of it is decided in one place, `planDrop`, for a drop on the window and for
File > Open alike. For a disc, "run" means mount and boot it: on a +3
that is Drive A, and real NextZXOS mounts disc images as its own drives, but ZenZX
does not emulate that for the Next yet (T-72) — so for now a `.dsk` dropped on a
Next has no run outcome and is copied to the card with no dialog, Shift or not. A
drop that mixes one with a tape copies it and asks about the tape.

### Running a .nex

A NEX is loaded by NextZXOS's `.nexload` and nothing else in the machine does it, so
"run" is what a person does at the keyboard, done for them, by the least invasive way
that works (`nexrun.go`; the guard is G-12):

1. **At the welcome screen, the menu or the Browser** — nothing is reset. The file goes
   on the second SD card, which NextZXOS calls D: (an empty slot gets a scratch card
   made in a temporary folder; a card that can be written is added to; a read-only one
   is replaced by the scratch card, which is said). The Browser is made to look at the
   cards again (its M then Y), and the keys that go from the menu to the command line,
   `cd "d:"` and `.nexload NAME` are pressed one at a time, frame by frame, the screen
   being looked at between them to know which screen the keys are meeting. It takes
   about twenty-five seconds, most of it the Browser reading the cards.
2. **While a program is running, or at the command line** — the Next may be in the
   middle of a game, and the menu cannot be reached from there by a key that is known
   to work, so a dialog asks whether the Next may be reset. When it is, the file is put
   on the card first, so NextZXOS sees the card as it starts, the machine is reset and
   the keys of 1 are pressed from the welcome screen. *Cancel* changes nothing.
3. **When neither works** (a menu the keys do not recognise, the command failing) — the
   Next is started again from a scratch boot card: the system's boot files, `.nexload`,
   the NEX and an `autoexec.bas` of one line that runs it. It takes the place of the
   card the Next started from until the card is changed back in the **SD Card** menu.

A version 1.3 NEX is refused, as NextZXOS's `.nexload` does not load it. How a run
ended is said by a toast.

### Dropping onto a storage slot

Dropping a file **onto a storage slot** copies it into that slot's media rather
than loading it into the machine, whatever the file type. While a file is dragged
over the window a slot drawer slides down from the top edge, with a rounded tile
per writable slot (the same look as the machine dock); dropping onto a tile adds
the file there:

| Slot | A dropped file is |
|------|-------------------|
| SD slot 2 (a Next's second card) | copied onto the card's FAT filesystem, and the Browser is refreshed so it shows at once, with no keypress |
| +3 Drive A (a disc in the drive) | written into the disc's +3DOS filesystem; it is seen on the next directory access (a `CAT` or a loader) |

With several machines the drop goes to the focused one's slot. A slot that cannot
take a file (read-only, or an empty drive) is shown dimmed with the reason, and a
drop onto it is refused with a toast.

## Spectrum keyboard mapping

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
