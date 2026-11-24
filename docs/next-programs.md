# Programs from the Next's card

What the programs on the distribution's card do in ZenZX, as of 0.26.0. Each was started from the
NextZXOS command line with `.nexload` and looked at on its first screens; none has been played
through. CSpect is the reference for what a Next's picture should be; the version of it that runs
here does not run the games, so they were compared by eye with jnext 1.3.0 instead, run headless on
the same card. The last column says what that showed.

jnext itself was checked against CSpect where both run: the NextZXOS welcome screen and show512 are
the same on every pixel, and a copper list gives the same bars. Since 0.20.0 the programs are started the same way in jnext and in ZenZX, with `.nexload` from the
NextZXOS command line and the same keys after it; jnext's own `--load` gave show512 a different
ground colour.

| Program | On the card | What it does here | In jnext |
|---|---|---|---|
| Warhawk | `/games/Next/Warhawk` | Title, high scores and the first level draw right (since 0.19.0, the DMA) | same screens, into the game |
| AngryBloaters | `/games/Next/AngryBloaters` | Title, high score table and the first stage draw right (since 0.19.0) | same screens, into the game |
| Nextoid | `/games/Next/Nextoid` | Menu and the first level draw right (since 0.19.0) | same screens, into the game |
| The Lords of Midnight | `/games/Next/LordsOfMidnight` | Title and the first view of the game draw cleanly (since 0.19.0) | same title |
| Santa's Chimney Challenge | `/games/Next/Santa's Pressie` | Title and the game draw right | same screens, into the game |
| Revival Survival | `/games/Next/Revival Survival` | Title, menu and the story draw right | same screens |
| Halls of the Things | `/games/Next/Halls of The Things` | Menu, hall of fame and the game draw right | same screens, into the game |
| Daybreak | `/demos/WidescreenImageDemo` | The 320x256 picture draws right | same picture |
| Tilemap test | `/demos/tilemap` | Draws the same test screen | same screen |
| Night-Knight (demo) | `/games/Next/Night-Knight` | Title, and the first stage plays itself: draws right | same screen |
| Spectron 2084 | `/demos/Spectron 2084` | Title, high scores, the story and the first room: draws right | same screens |
| show512 | `/demos/show512` | The same as CSpect's picture on every pixel | the same as CSpect's on every pixel, started with `.nexload` |
| ScrollNutter | `/demos/scrollnutter` | Draws right through its first two minutes (since 0.20.0, LoRes) | same stages; CSpect shows them too |
| TX-1696 | `/games/Next/TX-1696` | Started from its own folder: title, menu, weapon screen and the first level draw right (since 0.20.0, 4-bit sprites) | same screens |
| PaletteCycle | `/demos/PaletteCycle` | A snapshot, started from its own folder with `SPECTRUM "!RunMe.snx"` or from the Browser: the text, then each picture with its colours cycling (since 0.22.0, the Multiface) | does not run: back at the welcome screen. CSpect 3.3.1.0 starts it and stays at a black screen |
| Pogie | `/games/Next/Pogie` | A snapshot: the first screen of the game draws (since 0.22.0) | not tried |
| The Hollow Earth Hypothesis | `/games/Next/THEH` | A snapshot: the title picture draws (since 0.22.0) | not tried |
| OnlyYou6ch | `/demos/OnlyYou6ch` | A snapshot: its level meters draw (since 0.22.0); the music was not heard | not tried |

## ZenZX's own examples

Three programs made for ZenZX are in [`examples/next`](../examples/next/README.md) and
inside the program: `zenzx -next-examples` puts them on ZenZX's own card for the second
slot, drive D in NextZXOS.

| Example | What it does here | In jnext |
|---|---|---|
| `gallop.nex` | Layer 2, scaled 16-colour sprites streamed by the DMA, copper bands in the border, text-mode tilemap: draws as meant | same picture |
| `fuji.nex` | Layer 2 at 320x256 scrolled in four bands by the copper, a 512-tile tilemap over it, sprites over the border, palette fades: draws as meant | same picture |
| `orbit.nex` | Layer 2 with transparency, the layer order changed as it runs, scaled and turned sprites, the Next's interrupt mode with a CTC timer: draws as meant | same picture |

## The DMA+ guide's examples

The nine programs of `/docs/zxnDDMA/examples` on the card, assembled from their sources with
sjasmplus 1.24.1 and started with `.nexload` from the second card (since 0.26.0):

| Example | What it does here |
|---|---|
| `zxnddma_examples` | Each of its eight screens draws as its text says |
| `zxnddma_compare` | "ZXNDDMA FOUND"; 7.2, 15.6, 8.1, 2.1 and 2.1 ms for its five ways of drawing 32 balls, where the guide gives 7.2, 15.6, 8.2, 2.1 and 2.1 ms measured on an issue 5 |
| `zxnddma_sprites`, `zxnddma_clip`, `zxnddma_lemmings` | The balls and the walkers move over the picture, cut at the edges where the example cuts them |
| `zxnddma_modes` | The 256x192, 320x256 and 640x256 screens each draw their row of balls |
| `zxnddma_scroll`, `zxnddma_scroller`, `zxnddma_windows` | The landscape, the scrolling text and the window draw and move |

They were not compared with a machine or with CSpect 4.5, which has DMA+ and does not run here.

## What is missing

- **The Multiface's menu from the keyboard**: the Multiface is there for NextZXOS's own use (starting snapshots); the front ends have no NMI key yet (T-61).
- **Sound.** Nothing has been listened to: the DMA's timed transfers (T-58), the CTC's music (T-59).
- **A comparison of the games with CSpect**: the version that runs here does not run most of them.

The register, [`TRACKING.md`](TRACKING.md), has each of these and the rest.
