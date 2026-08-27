# Pluggable video rendering

How `-ns-graphics` modes plug into ZenZX's display, in both the live (GUI)
and headless builds. Landed 2026-08-17, replacing a design where the
standard bitmap+attribute renderer was hardcoded twice: once as a
GPU-texture fast path in the GUI (`display.go`), once as a CPU decoder
duplicated near-identically in both the GUI and headless builds.

## The seam: `VideoRenderer`

```go
type VideoRenderer interface {
    Name() string
    Decode(mem *SpectrumMemory, screen *SpectrumScreen) *image.RGBA
    Dimensions() (width, height int)
    BorderMargins() (left, right, top, bottom int)
}
```

`videorender.go` (raylib-free, compiled into both builds). A renderer
registers itself in an `init()`:

```go
func init() { RegisterVideoRenderer(myRenderer{}) }
```

keyed by `Name()` -- `""` for the standard renderer, or the exact
`-ns-graphics` value it implements (e.g. `NSGraphicsZenZX01`). Both mains
resolve the active renderer once at startup, from the validated
`-ns-graphics` value (`nonstandard.go`), via `zx.SelectVideoRenderer`.
Selecting an unregistered-but-valid mode is a startup error, not a silent
fallback to standard -- a not-yet-implemented mode must say so, not quietly
show the wrong picture. `zx.DecodeDisplay()` calls the active renderer;
every caller (the GUI's per-frame loop, headless screenshot capture, the
zenscript `shot` command) goes through this one method and never touches a
`VideoRenderer` directly.

Implementing a new mode is: write a `VideoRenderer`, register it. Nothing
in `display.go`, `display_headless.go`, or either main needs to change.

## Three design decisions (2026-08-17)

**FLASH is honoured by every renderer that has it in its real attribute
format.** Standard mode always did; hi-colour mode now does too (fixed
2026-08-18, corroborated by the ZX-Uno manual's own attribute
description for this mode -- "paper/ink/bright/flash attribute per each
8x1 pixels block" -- and this project's own `docs/timex-modes.md`). An
earlier version of this document claimed FLASH was standard-only "by
design," which was a mistaken description of a scope decision as if it
reflected real hardware behaviour -- it didn't; nothing found suggests
real hi-colour hardware disables or ignores this bit. FLASH state
(`flashEnabled`, `flashTickTock`, `lastFlashTime`) lives on
`SpectrumScreen` because that's the per-instance parameter already
threaded into every `Decode` call, not because it's part of the generic
contract; a renderer whose real format has no FLASH bit at all (should
one ever be added) simply never reads those fields.
`screen.updateFlash()` is called unconditionally once per frame from the
GUI's live path, regardless of which renderer is active, and preserves
this codebase's pre-existing asymmetry where headless screenshot capture
never advances FLASH on its own.

**Border is optional, shared infrastructure.** `BorderMargins()` lets a
renderer opt out entirely (all-zero margins); `DisplayManager` skips every
border code path when that's the case. Border rendering itself (colour
history, the stripe visualisation) is entirely orthogonal to which
renderer is active -- it composites around whatever the renderer decoded,
driven by the emulated machine's own port `0xFE` writes, not by video-mode
logic. Headless screenshots never include the border, in any mode, matching
existing behaviour (the border isn't part of any mode's display file).

**Magnification is shared; not every zoom level fits every mode.**
`SpectrumScreen.multiplier` is one scale factor for all modes. Because a
higher-resolution mode's window at a given multiplier can exceed the
monitor (e.g. a hypothetical 512x384 mode at 5x, with a proportional
border, would be far larger than most displays), `DisplayManager` computes
window and texture sizes from the active renderer's own `Dimensions()`/
`BorderMargins()` rather than assuming the standard 256x192/32-32-24-32,
and `maxMultiplierThatFits()` clamps `ScaleUp()` (and the initial
`-scale`, in `InitDisplay`) to what the current monitor can actually show,
falling back to no clamping if the monitor size can't be determined.
**Not yet exercised against an actual non-256x192 renderer** -- there
isn't one yet (T-09) -- so treat the clamping logic as reasoned-through
but unverified until a higher-resolution mode exists to test it against.

## What moved where

| Before | After |
|---|---|
| `SpectrumScreen` (display.go, GUI: bitmap/attributes/flash + GPU textures) and `SpectrumScreen` (display_headless.go, headless: bitmap/attributes/flash only) -- two independent definitions | `SpectrumScreen` (`screen.go`, one definition, raylib-free): bitmap, attributes, multiplier, flash state. Storage only. |
| `DecodeRGBA` (display.go) and `DecodeRGBA` (display_headless.go) -- near-identical, independently maintained | `standardVideoRenderer.Decode` (`videorender.go`), one implementation. Also fixed a real divergence: the GUI's version checked `flashEnabled` before swapping ink/paper on FLASH; headless's didn't. Now both call the GUI's (correct) version. |
| GUI's per-cell fast path: 256 pre-baked 1x8 bit-pattern textures x 16 paper-colour textures, blitted per character cell (`generateTextures`, `render()`) | One `image.RGBA` from `Decode`, uploaded to one `rl.Texture2D` per frame (`rl.UpdateTexture`), drawn scaled. Removed entirely rather than kept as a special case, since it only knew how to draw the standard bitmap+attribute layout. |
| `writeScreenPNG` (scheduler.go) and `writePNG` (zenzx_headless.go) -- two near-identical PNG-encode-a-screenshot functions | One `writeScreenPNG(path string, zx *ZenZX)`, calling `zx.DecodeDisplay()`. |
| GPU texture fields (`bitPatternTextures`, `paperColorTextures`, `borderTexture`) and `borderStripesEnabled` on `SpectrumScreen` | `borderTexture`, `screenTexture`, `borderStripesEnabled` on `DisplayManager` (GUI-only) -- these are rendering-backend resources, not display-file storage, so they don't belong on the type `memory.go` reads and writes directly. |

## Regression evidence

Before touching any rendering code, the pre-refactor headless build was
built fresh from the last git commit (not the working tree) and run
alongside the post-refactor build with identical flags. Screenshot output
was byte-for-byte identical (SHA-256 match) across two models (48K, 128K)
and multiple frame checkpoints. The standard mode's picture did not change;
only the path that produces it did.
