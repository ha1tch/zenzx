# custom-roms/

Alternate ROM variants for international users -- language translations
and similar regional variants of the standard ROM set, distinct from
`rom/` (the standard, embedded-in-the-binary set covered by
`rom/SINCLAIR.txt` and `rom/TIMEX.txt`).

Drop any `.rom` file in here and it becomes available to the interactive
selector. Folders work too, two levels down (`custom-roms/mods/x.rom`, `custom-roms/mods/more/y.rom`; a
folder whose name begins with a dot is left out): in the window's Custom ROM menu each is a
submenu, and what is too much for the window's height goes under **More...**; in the selector a
ROM in a folder is listed as `mods/x.rom`. The selector:

```
./zenzx -model=48k -custom-roms-menu
```

This lists every `.rom` file in this directory, lets you pick one, and
(for multi-bank models) asks which ROM bank it should replace, applying
it on top of `-model`'s standard set via the same mechanism `-rom0`
through `-rom3` use (`zenzx.OverrideROMBank`).

`-custom-roms-dir <path>` points the selector at a different directory
if you'd rather keep alternates somewhere else.

### rom-archives.json

The window's Custom ROM > **Download more ROMs** submenu, which opens in the browser the pages where
ROMs are published, is made from `rom-archives.json` in this directory when it is there: a JSON
array of objects, each with the `text_label` of the row and the `url` of the page (http or https):

```json
[
  {"text_label": "Rodolfo Guerra ROM mods", "url": "https://sites.google.com/view/rodolfoguerra/"},
  {"text_label": "Speccy4ever ROM archive", "url": "https://speccy4ever.speccy.org/"}
]
```

The file shipped here is that list, the standard distribution's; edit it to add, change or remove
pages, and the menu follows without restarting. Without the file the menu has those two pages. An
entry with no `text_label`, or whose `url` is not an http or https address, is left out (a warning
goes to the console); a file that is not a list, or none of whose entries is usable, gives the two
standard pages; `[]` removes the row. More pages than fit under the row for the window's height are
cut to the ones that do, so keep the list short. It is read with `-custom-roms-dir`'s directory.

## Shipped here

In the folder `Microdigital/`, the Microdigital TK90X and TK95 (Brazilian ZX Spectrum clones) ROMs,
each a 16K image derived from the Spectrum ROM, so the same Sinclair/Amstrad copyright
as `rom/` applies (the folder is a submenu of the window's Custom ROM menu, and the selector lists
them as `Microdigital/TK90X.rom` and so on):

| File | Machine | What it is | Rodolfo's original file |
|---|---|---|---|
| `TK90X.rom` | TK90X | Portuguese, version 2: always Portuguese | `TK90X_v2BR.ROM` |
| `TK90X-pt-BR.rom` | TK90X | Version 1, the one that reads the language switch: Spanish, or Portuguese when the switch is turned (see below) | `TK90X_v1ES.ROM` |
| `TK90X-es-UY.rom` | TK90X | Spanish, version 2: always Spanish | `TK90X_v2ES.ROM` |
| `TK95-pt.rom` | TK95 | Portuguese: always Portuguese | `TK95BR.ROM` |
| `TK95-es.rom` | TK95 | Spanish: always Spanish | `TK95ES.ROM` |

A ROM's language, and its country where that is known, are in its name as a language tag
(BCP 47): the language in lower case, then the country in capitals, as in `pt-BR` and `es-UY`.
The ROM with no tag is the machine's version 2 Portuguese one.

The last column is the name in the package of the historical Microdigital ROMs that Rodolfo Guerra
supplied (`TK90X - todas las ROMs originales`): the five files in it are byte for byte the five
here, so nothing was replaced and no second copy is shipped. His names say the version and then
`BR` or `ES` where ours say a language and a country, and they differ from ours for `TK90X.rom`
(his `BR`, ours untagged) and for `TK90X-pt-BR.rom` (his `v1ES`: a Spanish ROM, which is what it
shows with the switch at its default). The names above stay as the ones ZenZX has used so far.

### The language switch

Microdigital's machines had a diode that chose between Spanish and Portuguese: it set bit 7 of the
ULA port (`IN A,(0xFE)`), and some of their ROMs read that bit before they show a message. ZenZX
has no Microdigital model, so the switch is a setting of the program: File > Preferences >
Machines > **Language switch** (**Spanish**, the default, or **Portuguese**), or `-language-switch
spanish|portuguese` for one run. Portuguese reads bit 7 low, Spanish leaves it high as on every
other Spectrum. It applies to the 48K models only (the Microdigital clones were 48K machines): the
Amstrad models (128K, +2, +2A, +3), the Timex and the Next read the port as they always did.

Of the ROMs in `Microdigital/` only `TK90X-pt-BR.rom` reads it (`IN A,(0xFE)`, `RLA`, `RET NC`: Portuguese
when the bit is low). The others do not follow it: `TK90X.rom` and `TK95-pt.rom` always answer in
Portuguese, `TK90X-es-UY.rom` and `TK95-es.rom` always in Spanish.

They go on a 48K model: `./zenzx -model=48k -custom-roms-menu`, then pick one.

In the folder `Zenpoint Project/`, our own [zenpoint](https://github.com/ha1tch/zenpoint) swap ROMs -- a 48K ROM
with zenpoint's Sinclair-format floating point in place of the original -- in
Spanish and English. These are interim v0.1.3 builds, quad-licensed (Apache-2.0
OR GPL-2.0-only OR GPL-3.0-only OR MPL-2.0):

| File | What |
|---|---|
| `zenpoint-0.1.3-es.rom` | zenpoint 48K swap ROM, Spanish |
| `zenpoint-0.1.3-en.rom` | zenpoint 48K swap ROM, English |
