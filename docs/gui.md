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
with seven menus. Once the bar is shown, hovering any label opens its
dropdown immediately -- no click needed -- and moving to a different label
switches directly to it, the same as a traditional desktop menu bar once
any menu is already engaged.

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
  open for toggling the other without reopening it. **Show Status** is
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
right of the bar has its own dropdown -- click it directly, no hover
needed, always right-aligned to the window's own right edge regardless
of the hot zone's own width:

- **Fixed menu bar** -- a checkbox. Pins the bar permanently shown,
  bypassing the idle-at-edge auto-hide entirely, until unchecked --
  the window grows taller to accommodate it (animated, the same way
  toggling the border already animates a resize) rather than
  overlapping the Spectrum display, and shrinks back when unfixed.
- **ZenZX website** -- opens the project's own page
  (`https://ha1tch.github.io/zsp/projects/zenzx`) in the default
  browser.
- **Help** -- a scrollable reference covering every menu, keyboard
  shortcut, and command-line flag.
- **About...** -- version, project link, and licence/attribution.

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

Theme, font, font zoom, display scale, and whether the menu bar is
pinned shown are saved to `settings.json` (`pkg/settingsconfig`, same
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
| `F2` | Pause / resume |
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
| `Alt+F` | Toggle the FPS counter |
| `Alt+B` | Toggle the border |
| `Alt+P` | Play / stop the tape |
| `Alt+R` | Rewind the tape |
| `Alt+T` | Toggle tape mode (accurate / fast) |
| `Alt+I` | Print tape status and block list |
| `Alt+M` | Open or close the machine dock |
| `Alt+1`..`Alt+9` | With several machines (`-multi`), bring that machine to the front: the display, sound and keyboard pass to it, the same as clicking its tile. A digit past the number of machines does nothing. |

## Drag and drop

In the GUI build, dropping a file onto the window loads it by extension:

| Extension | Loads as |
|-----------|----------|
| `.scr` | A raw 6912-byte screen dump, written straight onto the framebuffer (6144 bytes of bitmap + 768 bytes of attributes). This only paints the screen — it does not change CPU state or memory, so it is a still image, not a running program. |
| `.zxs` | A ZenZX snapshot (the proprietary chunk format). |
| `.tap` / `.tzx` | A tape image (loaded, then driven with the `Alt+P/R/T/I` controls). |
| `.dsk` | A +3 disk image (requires +3 mode with the FDC enabled). |
| `.sna` / `.z80` | Standard snapshots (48K and 128K), via the shared `zentools` library. |
| `.szx` | ZX-State snapshots, load only, via the shared `zentools` library. |

A `.scr` can also be loaded headlessly with `-scr` (see above).

Dropping a file **onto a storage slot** copies it into that slot's media rather
than loading it into the machine. While a file is dragged over the window a slot
drawer slides down from the top edge, with a rounded tile per writable slot (the
same look as the machine dock); dropping onto a tile adds the file there:

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
