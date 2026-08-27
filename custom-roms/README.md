# custom-roms/

Alternate ROM variants for international users -- language translations
and similar regional variants of the standard ROM set, distinct from
`rom/` (the standard, embedded-in-the-binary set covered by
`rom/SINCLAIR.txt` and `rom/TIMEX.txt`).

Empty by default. Drop any `.rom` file in here and it becomes available
to the interactive selector:

```
./zenzx -model=48k -custom-roms-menu
```

This lists every `.rom` file in this directory, lets you pick one, and
(for multi-bank models) asks which ROM bank it should replace, applying
it on top of `-model`'s standard set via the same mechanism `-rom0`
through `-rom3` use (`zenzx.OverrideROMBank`).

`-custom-roms-dir <path>` points the selector at a different directory
if you'd rather keep alternates somewhere else.
