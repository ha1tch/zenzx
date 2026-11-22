# The Next's files

ZenZX looks here first for what a ZX Spectrum Next needs, and keeps what it makes here.
None of it is in the repository: the files are other people's, or large, or yours.

| File | What it is | Where it comes from |
|---|---|---|
| `nextboot.rom` | The Next's boot ROM, 8K | Downloaded by ZenZX the first time the Next is chosen (or `zenzx-headless -next-setup`) |
| `enNxtmmc.rom` | The DivMMC ROM of the distribution, 8K | The same download |
| `next-card.img` | The SD card NextZXOS boots from, 1 GB | Made by ZenZX from the same download |
| `zenzx-card.img` | ZenZX's own card for the second slot: the example programs and anything added with `-next-add` | `zenzx -next-examples`, or the SD Card menu |
| any other `*.img` | Cards of your own, for the second slot | The SD Card menu's New Empty Card, `zenzx -next-new-card=PATH`, or a card image from elsewhere; the menu's browser opens here |

A download goes to this folder when ZenZX is started from the folder that holds it, and to
`zenzx/next` under your configuration folder otherwise. `-next-files=DIR` or the
`NEXT_FILES` variable names another folder. `thirdparty/`, where these files used to go, is
still looked in after this one.

The manual, `docs/MANUAL.md`, has the rest under "ZX Spectrum Next".
