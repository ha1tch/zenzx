# Next GPU compositing seam — proposal

Updated: 2026-09-15

Design intent for how T-30 (hardware sprites, wave 7) and T-31 (Next
palette, wave 8) should be built so neither duplicates or has to unwind
work the other already did, and so both keep faith with the GPU-traffic
optimisation intent behind the original, hand-written standard-mode fast
path (`videorender_gpu.go`'s `bitPatternGPU`/`paperColourGPU`: pre-bake a
small texture set once, then only issue draw calls per frame — no
CPU-composited image built or re-uploaded every frame). Written before
either wave starts, per this project's own tracking-document practice of
recording deviations/refinements to a frozen plan as a proposal rather
than editing `NEXT_SUPPORT_DEVELOPMENT_PLAN.md` in place (that plan froze
once `ti0.r1` began executing).

Nothing here has been implemented. This is a design note to read before
starting T-30, not a record of what exists.

## The problem this is heading off

T-29 (Layer 2, done at wave level, T-29 itself still open at ◐) built its
own interim colour resolution: `layer2Pixel` (`layer2.go`) reads a raw
Next-mode byte, adds NR 0x70's palette offset, and maps the result down
to one of the 16 existing classic `zxPalette` entries — a documented,
explicitly temporary stand-in for the real 9-bit/512-colour Next palette
T-31 will eventually provide. Its GPU fast path (`renderLayer2GPU`)
reuses the classic renderer's own `paperColourGPU` cache (16 pre-baked
1x1 textures) rather than baking a new one, specifically because a
256-entry cache keyed on the *raw* pixel byte would go stale the moment
NR 0x70 changes at runtime — keying on the *already-resolved* 16-colour
index sidesteps that, since `paperColourGPU`'s 16 entries never change
for as long as "resolved colour" means "one of the 16 classic colours."

If T-30 (sprites) is built next and independently invents its own
version of that same raw-byte-to-colour mapping — which it will need,
since sprite pixels are Next-mode 8-bit colour bytes exactly like Layer
2's — two things go wrong when T-31 lands:

1. T-31 has to find and rewrite colour resolution in two unrelated
   places (`layer2.go` and wherever sprites put their own copy) instead
   of one, with no shared seam to swap the body of.
2. Worse, once a *real* Next palette exists, "resolved colour" stops
   being a runtime-invariant fact. A palette write becomes a live event
   that must invalidate whichever GPU textures were baked from the old
   colour set. Two independently-built compositors means two
   independently-invented (and probably differently-shaped)
   cache-invalidation stories, discovered and debugged twice, instead of
   one shared mechanism designed once.

The fix is cheap now and expensive later: factor the "raw byte + palette
state -> display colour" step into one small seam both Layer 2 and
sprites call, and give the *cache* side a hook for "the palette changed"
that costs nothing today (it never fires, because nothing can change the
resolved colour of a fixed 16-entry table) but is already wired to the
right place once T-31 actually needs it to fire.

## Proposed seam

A function (exact name/package TBD at implementation time — candidate:
`ResolveNextPixel(raw uint8, paletteOffset uint8, paletteVersion
*uint32) color2bpp` or similar, package `main` alongside `layer2.go`,
since neither sprites nor Layer 2 warrant their own package split for
this alone) that both `layer2Pixel` and sprites' own pixel-resolution
path call, replacing `layer2Pixel`'s current inline top-nibble
arithmetic. Its body stays exactly what `layer2Pixel` does today (top
nibble of `raw + paletteOffset<<4`, mapped into the 16-entry
`zxPalette`) until T-31 replaces the body with a real 256-entry, 9-bit
RGB lookup across the wiki's 8 palettes (ULA/Layer2/Sprite/Tilemap x
first/second). The point of extracting it now, before it has anything
real to abstract over, is purely so T-31 has one call site to change
instead of two independently-drifted ones.

Layer 2's existing `layer2PaletteOffset()` accessor (`layer2.go`,
NR 0x70 bits 3-0) stays as the interim per-Layer2 configuration this
seam consumes; sprites will need their own analogous per-sprite palette
offset (attribute-table field, per NEXT_SUPPORT_DEVELOPMENT_PLAN.md's
own sprite attribute list: "position, pattern index, palette offset,
visibility") passed through the same seam rather than a second, parallel
offset-application path.

## Palette-version counter (the cache-invalidation hook)

Add a single monotonically-increasing counter -- call it
`nextPaletteVersion uint32`, living on `SpectrumIO` alongside
`nextRegs`/`nextRegClipWin` since it is Next-register-adjacent state, not
a display/GUI concept -- incremented on every write that could change a
resolved colour. Under the current interim mapping that is only NR 0x70
(the palette offset), so the counter only increments on `nextRegWriteDirect`
handling register 0x70; T-31 widens the trigger set to every real
palette-entry write once those ports exist, in exactly the one place
this increment already lives, not a new mechanism.

GPU texture caches that are keyed on *resolved colour* -- today that's
`paperColourGPU` (already effectively version 0 forever, since nothing
increments the counter yet) and, once sprites exist, whatever pattern
texture cache they bake -- store the `nextPaletteVersion` value they were
baked against, and cheaply compare it each frame (or on each cache
lookup) against the live value: a mismatch means "rebake the affected
entries," not "rebuild everything from scratch." This is the mechanism
T-31 needs to exist already, wired to the one real trigger it will add
writes to, rather than something sprites (or a later Layer 2 revision)
has to retrofit under time pressure once palette writes actually start
happening.

`paperColourGPU` itself does not need to start reacting to this counter
as part of T-30 -- it is still genuinely runtime-invariant until T-31
lands, since nothing yet writes anything that changes what a "resolved
16-colour index" means. The counter only needs to *exist and increment
correctly* by the time T-30 ships, verified with a unit test that NR
0x70 writes bump it and nothing else does, so T-31 inherits a working,
already-tested trigger rather than having to build and prove one from
scratch under its own scope.

## Sprite pattern-texture cache shape (T-30's own GPU design)

Unlike Layer 2 (a free-form 256x192 byte grid with no small catalogue of
reusable patterns), sprites are the closer match to the original
technique: 128 sprites, each a fixed 16x16x8-bit pattern. The natural
cache is one GPU texture per sprite's *currently loaded pattern*, baked
only when that sprite's pattern data actually changes (a write to its
pattern memory, or its pattern-index attribute pointing it at a
different pattern slot) -- not rebaked every frame regardless of
whether anything changed, and not one shared 256-entry cache the way
Layer 2's per-pixel-value reuse works, since a sprite's *pattern*
(256 pixels arranged in a specific 16x16 shape) is the reusable unit
here, not a single pixel value.

Concretely: a `spritePatternGPU` cache keyed by pattern slot (not by
sprite instance -- multiple sprites can share one pattern), each entry
storing both the baked `rl.Texture2D` and the `nextPaletteVersion` it
was baked against (see above) plus a dirty flag set whenever that
pattern slot's raw bytes are written. `RenderGPU`-equivalent per-frame
work becomes, per active sprite: is this pattern's texture dirty or
stale against the current palette version? If so, rebake (256 pixel
reads through the shared colour-resolution seam, same shape as
`generateGPUTextures`' existing per-entry bake loop) and clear the flag;
either way, one positioned `DrawTextureEx` call using the sprite's
current x/y attribute fields. This keeps the "bake once (or on real
change), draw many" property even though sprite patterns -- unlike the
classic ULA's fixed 256 bit-patterns -- can change at runtime: the
_reactive_ rebake-on-dirty design is what preserves the intent, not a
naive one-time bake that would silently go stale the first time a
program updates a sprite's graphic.

Composite ordering (NR 0x15's full six-way S/L/U table, already decoded
correctly by `layer2ULAOnTop` in `layer2.go` since T-29 -- see that
function's own doc comment on why every ordering was decoded rather than
just the two the wave plan called out) needs a genuine three-layer
decode once sprites exist, since `layer2ULAOnTop`'s current boolean
collapse ("is ULA on top of Layer2") was only ever correct in the
sprite-absent world T-29 shipped into. T-30 will need to either widen
that function's return type to express all three relative orderings, or
add a sibling function sprites consult separately -- worth deciding at
T-30 implementation time, not here, since it depends on how the sprite
draw pass actually gets threaded into `DisplayManager.Render` alongside
the existing ULA and Layer 2 passes.

## What changes where, once T-31 actually lands

So this is traceable later without re-deriving it: T-31 should expect to
touch exactly these three places, and no others, if this proposal was
followed --

1. The shared colour-resolution seam's body (`ResolveNextPixel` or
   whatever it ends up named): replace the 16-colour top-nibble mapping
   with a real 8-palettes x 256-entries x 9-bit-RGB lookup.
2. `nextPaletteVersion`'s trigger set (currently just NR 0x70): widen to
   every real palette-index/palette-value NextREG port T-31 adds.
3. Each GPU cache keyed on resolved colour (`paperColourGPU`'s Layer 2
   consumer, `spritePatternGPU`): start actually comparing against a
   *changing* `nextPaletteVersion` instead of a value that never moved --
   no new invalidation logic to write, since the comparison and rebake
   path already exists and was already exercised (against a version
   counter that just happened to never increment) by T-30's own tests.

If T-30 is built without this seam and cache-version hook, none of the
above is free -- T-31 would instead need to locate and refactor two
independently-evolved colour paths and invent cache invalidation from
scratch under its own scope, which is the exact outcome this proposal
exists to avoid.
