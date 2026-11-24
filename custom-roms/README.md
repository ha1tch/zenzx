# custom-roms/

Alternate ROM variants for international users -- language translations
and similar regional variants of the standard ROM set, distinct from
`rom/` (the standard, embedded-in-the-binary set covered by
`rom/SINCLAIR.txt` and `rom/TIMEX.txt`).

Drop any `.rom` file in here and it becomes available to the interactive
selector:

```
./zenzx -model=48k -custom-roms-menu
```

This lists every `.rom` file in this directory, lets you pick one, and
(for multi-bank models) asks which ROM bank it should replace, applying
it on top of `-model`'s standard set via the same mechanism `-rom0`
through `-rom3` use (`zenzx.OverrideROMBank`).

`-custom-roms-dir <path>` points the selector at a different directory
if you'd rather keep alternates somewhere else.

## Shipped here

The Microdigital TK90X and TK95 (Brazilian ZX Spectrum clones) ROMs, each a
16K image derived from the Spectrum ROM, so the same Sinclair/Amstrad copyright
as `rom/` applies:

| File | Machine | Variant |
|---|---|---|
| `TK90X.rom` | TK90X | standard |
| `TK90X-BR.rom` | TK90X | Brazilian |
| `TK90X-UY.rom` | TK90X | Uruguayan |
| `TK95-PT.rom` | TK95 | Portuguese |
| `TK95-ES.rom` | TK95 | Spanish |

They go on a 48K model: `./zenzx -model=48k -custom-roms-menu`, then pick one.

Our own [zenpoint](https://github.com/ha1tch/zenpoint) swap ROMs -- a 48K ROM
with zenpoint's Sinclair-format floating point in place of the original -- in
Spanish and English. These are interim v0.1.3 builds, quad-licensed (Apache-2.0
OR GPL-2.0-only OR GPL-3.0-only OR MPL-2.0):

| File | What |
|---|---|
| `zenpoint-0.1.3-es.rom` | zenpoint 48K swap ROM, Spanish |
| `zenpoint-0.1.3-en.rom` | zenpoint 48K swap ROM, English |
