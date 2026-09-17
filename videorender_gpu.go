//go:build !headless

package main

import (
	"image"
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Compile-time checks that both fast-path renderers genuinely satisfy
// FastGUIRenderer -- catches an accidental signature drift immediately,
// rather than only discovering it via the runtime type assertion in
// Render silently falling back to Decode().
var (
	_ FastGUIRenderer = standardVideoRenderer{}
	_ FastGUIRenderer = hicolourVideoRenderer{}
)

// FastGUIRenderer is an optional interface a VideoRenderer can also
// implement for GPU-accelerated live rendering via raylib texture
// blitting, used by DisplayManager.Render instead of Decode()+texture-
// upload when available. A renderer without this interface -- or the
// headless build, which has no GL context to blit into -- uses Decode()
// exactly as before; nothing about that portable path changes.
//
// RenderGPU draws directly to whatever render target is currently bound
// (the main window, already cleared and with the border texture drawn if
// applicable by the time Render calls this) -- no intermediate texture,
// matching how this renderer worked before the VideoRenderer abstraction
// existed. offsetX/offsetY is where the screen area begins (0,0 if
// borderless, or just past the border if not); multiplier is the fixed
// integer pixel scale (dm.screen.multiplier) -- both pkg-textures
// (bitPatternGPU, paperColourGPU) are baked once and never re-uploaded;
// only draw calls happen per frame.
type FastGUIRenderer interface {
	RenderGPU(dm *DisplayManager, mem *SpectrumMemory, screen *SpectrumScreen, offsetX, offsetY float32, multiplier int)
}

// generateGPUTextures bakes the two shared texture caches every
// FastGUIRenderer draws from: bitPatternGPU (256 textures, one per
// possible bitmap byte value -- an 8x1 mask of which pixels are "on",
// tinted per-draw to whatever colour a given cell/row needs) and
// paperColourGPU (16 textures, one solid-colour block per palette entry).
// Both are universal concepts, not renderer-specific, so every
// FastGUIRenderer implementation shares this one bake rather than each
// generating its own. Call once, after the window exists (raylib texture
// upload needs a live GL context) -- InitializeAfterWindow does this.
func (dm *DisplayManager) generateGPUTextures() {
	for bitPattern := 0; bitPattern < 256; bitPattern++ {
		img := image.NewRGBA(image.Rect(0, 0, 8, 1))
		for bit := 0; bit < 8; bit++ {
			on := (bitPattern>>(7-bit))&1 == 1
			var c color.RGBA
			if on {
				c = color.RGBA{0xff, 0xff, 0xff, 0xff} // opaque: tinted by draw colour
			} else {
				c = color.RGBA{0, 0, 0, 0} // transparent: paper shows through
			}
			img.Set(bit, 0, c)
		}
		dm.bitPatternGPU[bitPattern] = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
	}

	for colorIndex := 0; colorIndex < 16; colorIndex++ {
		c := zxPalette[colorIndex]
		img := image.NewRGBA(image.Rect(0, 0, 8, 1))
		for x := 0; x < 8; x++ {
			img.Set(x, 0, color.RGBA{c.R, c.G, c.B, c.A})
		}
		dm.paperColourGPU[colorIndex] = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
	}

	dm.gpuTexturesReady = true
}

// RenderGPU is standardVideoRenderer's fast path: two passes over the
// 24x32 attribute grid, one texture-blit per cell per pass, all pixel
// compositing done by the GPU via tinted texture draws rather than a
// CPU-side image.RGBA built pixel by pixel. Restores the technique this
// renderer used before the VideoRenderer abstraction (needed for
// hi-colour and headless screenshot support) replaced it with the
// portable Decode() path -- Decode() is unchanged and still used by
// headless and by any renderer without a fast path; this is purely an
// additional, faster path for the GUI's live rendering of the specific
// case this renderer covers.
func (standardVideoRenderer) RenderGPU(dm *DisplayManager, mem *SpectrumMemory, screen *SpectrumScreen, offsetX, offsetY float32, multiplier int) {
	mult := float32(multiplier)

	// Paper pass: one 8x8 solid-colour block per attribute cell.
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			attr := screen.attributes[row*32+col]
			paper := (attr >> 3) & 0x07
			bright := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			colourIndex := paper | (bright << 3)
			if screen.flashEnabled && flash == 1 && screen.flashTickTock {
				ink := attr & 0x07
				colourIndex = ink | (bright << 3)
			}

			pos := rl.NewVector2(offsetX+float32(col*8)*mult, offsetY+float32(row*8)*mult)
			rl.DrawTextureEx(dm.paperColourGPU[colourIndex], pos, 0, mult*8, rl.White)
		}
	}

	// Ink pass: one 8x1 bit-pattern blit per bitmap byte (8 per cell),
	// tinted to that cell's ink (or paper, mid-flash) colour.
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			attr := screen.attributes[row*32+col]
			ink := attr & 0x07
			bright := (attr >> 6) & 0x01
			flash := (attr >> 7) & 0x01

			var tint rl.Color
			if screen.flashEnabled && flash == 1 && screen.flashTickTock {
				paper := (attr >> 3) & 0x07
				tint = zxPalette[paper|(bright<<3)]
			} else {
				tint = zxPalette[ink|(bright<<3)]
			}

			for y := 0; y < 8; y++ {
				off := screen.calcByteOffset(col*8, row*8+y)
				byteValue := screen.bitmap[off]
				pos := rl.NewVector2(offsetX+float32(col*8)*mult, offsetY+float32(row*8+y)*mult)
				rl.DrawTextureEx(dm.bitPatternGPU[byteValue], pos, 0, mult, tint)
			}
		}
	}
}

// RenderGPU is hicolourVideoRenderer's fast path -- same shared textures
// and the same two-pass shape as the standard renderer, but at the
// per-byte (8x1) attribute granularity hi-colour actually has, rather
// than standard's per-cell (8x8) granularity. FLASH is deliberately
// unread here too, matching Decode()'s own documented design (see
// videorender_hicolour.go's package doc) -- this is a faster path to the
// exact same pixels Decode() already produces, not a behaviour change.
func (hicolourVideoRenderer) RenderGPU(dm *DisplayManager, mem *SpectrumMemory, screen *SpectrumScreen, offsetX, offsetY float32, multiplier int) {
	mult := float32(multiplier)

	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			for y := 0; y < 8; y++ {
				off := screen.calcByteOffset(col*8, row*8+y)
				attr := mem.Read(uint16(hicolourAttrBase + off))
				ink := attr & 0x07
				paper := (attr >> 3) & 0x07
				bright := (attr >> 6) & 0x01
				flash := (attr >> 7) & 0x01

				if screen.flashEnabled && flash == 1 && screen.flashTickTock {
					ink, paper = paper, ink
				}

				px := offsetX + float32(col*8)*mult
				py := offsetY + float32(row*8+y)*mult

				rl.DrawTextureEx(dm.paperColourGPU[paper|(bright<<3)],
					rl.NewVector2(px, py), 0, mult*8, rl.White)

				byteValue := screen.bitmap[off]
				tint := zxPalette[ink|(bright<<3)]
				rl.DrawTextureEx(dm.bitPatternGPU[byteValue],
					rl.NewVector2(px, py), 0, mult, tint)
			}
		}
	}
}

// renderLayer2GPU is T-29's Layer 2 fast path: draws directly into
// whatever render target is currently bound, one 1x1-texture blit per
// visible pixel, using the shared paperColourGPU cache (see this file's
// own struct-field comment on why Layer 2 needs no cache of its own).
// Called from DisplayManager.Render as an ADDITIONAL pass after the ULA
// layer has already drawn (via either RenderGPU or the CPU texture path)
// -- unlike FastGUIRenderer, Layer 2 is not a VideoRenderer and never
// replaces the ULA draw, only composites on top of or is skipped in
// favour of it. T-30 (wave ti0.r4) widened the gate from the original
// two-layer layer2ULAOnTop check to layerIsAboveULA(nextLayerLayer2, ...)
// (layer2.go), the real three-layer decode -- necessary once sprites
// exist, since a wrong ordering could show Layer 2 covering a sprite
// that should be on top of it, or vice versa, something the old boolean
// couldn't express. Returns immediately (draws nothing) when Layer 2 is
// not enabled or when Layer 2 is not positioned above ULA in the current
// NR 0x15 ordering, so the only case ULA is left showing already-drawn on
// the render target is also the cheapest case here: zero extra draw
// calls, not a composite-then-discard.
//
// Only the clip window's rectangle is blitted -- pixels outside it never
// draw at all, rather than being drawn transparent and discarded, since
// there is no separate "erase" step needed: the ULA layer is already the
// correct picture everywhere Layer 2 doesn't draw, exactly matching
// DecodeLayer2/CompositeLayer2's transparency semantics on the portable
// path but without ever constructing the transparent pixels in the first
// place. (NR 0x18's clip coordinates are 8-bit, 0-255 per axis, so a
// program that never widens the clip window past its own power-on
// default leaves 320/640-mode's columns beyond 255 undrawn regardless of
// this function's own correctness -- a known, separately-tracked gap,
// see docs/TRACKING.md's T-29 entry, not something this pass changes.)
//
// GEOMETRY (2026-09-15, T-29 GUI-wiring pass): each pixel draws via
// DrawTexturePro rather than the uniform-scale DrawTextureEx this
// function used before, because 640x256x4bpp mode's pixels are HALF the
// screen-width of every other mode's (layer2PixelWidthScale, layer2.go --
// confirmed against an exact wiki.specnext.dev quote: "every byte is
// displayed as two half-width paired pixels") while their HEIGHT is
// unaffected -- a per-axis scale DrawTextureEx cannot express. pxW is
// both the horizontal spacing between consecutive pixels' draw positions
// AND (scaled up by the cached texture's own 8px width) the width of each
// individual splat -- preserving the pre-existing "each pixel draws
// wider than its own spacing, so the painter's-algorithm left-to-right
// draw order overwrites the overlap and only the rightmost column's own
// splat overreaches" technique bakeLayer2ColourGPU's 8x1 texture exists
// for, just parametrised by mode instead of hardcoded to multiplier
// alone. That rightmost-column overreach is why this function scissors
// itself to its own mode's real displayed footprint (layer2DisplayedSize)
// before drawing -- without it, the last pixel in each row bleeds up to
// 7 pixel-widths of its own colour into whatever sits beyond the Layer 2
// picture's own right edge (the border, or -- since this pass stopped
// wrapping the Layer2/sprite draw in the classic screen's own narrower
// scissor rect, precisely so wide-mode content wouldn't be wrongly
// cropped by it -- potentially further still). Confirmed as a real,
// previously-latent requirement: the pre-T-29-GUI-wiring-pass code relied
// on Render's OWN classic-screen scissor to contain this same overreach
// as a side effect, which is exactly the clipping this pass had to stop
// doing to let wide Layer 2 content draw at all -- so the containment
// has to happen here instead, scoped to Layer 2's own footprint, not
// reinstated at the old, now-too-narrow classic-screen granularity.
func renderLayer2GPU(dm *DisplayManager, io *SpectrumIO, mem *SpectrumMemory, offsetX, offsetY float32, multiplier int) {
	if !io.layer2Enabled() || !layerIsAboveULA(nextLayerLayer2, io.GetNextRegister(0x15)) {
		return
	}
	if !dm.layer2ColourGPUReady || dm.layer2ColourGPUVersion != io.nextPaletteVersion {
		dm.bakeLayer2ColourGPU(io)
	}
	mode := io.layer2CurrentMode()
	mult := float32(multiplier)
	pxW := layer2PixelWidthScale(mode, multiplier)

	dispW, dispH := layer2DisplayedSize(mode, multiplier)
	rl.BeginScissorMode(int32(offsetX), int32(offsetY), int32(dispW), int32(dispH))
	defer rl.EndScissorMode()

	x1, x2, y1, y2 := io.layer2ClipWindow()
	for y := int(y1); y <= int(y2); y++ {
		for x := int(x1); x <= int(x2); x++ {
			idx := mem.layer2Pixel(io, x, y)
			tex := dm.layer2ColourGPU[idx]
			source := rl.NewRectangle(0, 0, float32(tex.Width), float32(tex.Height))
			dest := rl.NewRectangle(offsetX+float32(x)*pxW, offsetY+float32(y)*mult, float32(tex.Width)*pxW, mult)
			rl.DrawTexturePro(tex, source, dest, rl.Vector2{}, 0, rl.White)
		}
	}
}

// bakeLayer2ColourGPU (re)builds every entry of dm.layer2ColourGPU against
// Layer 2's currently-displayed palette (T-31), unloading any previously
// baked textures first so a rebake never leaks GPU memory. Called by
// renderLayer2GPU whenever the cache is unbaked or stale against
// io.nextPaletteVersion -- see layer2ColourGPUVersion's own doc comment
// (display.go) for why a full rebake, rather than per-entry dirty
// tracking, is the right cost/complexity trade here.
func (dm *DisplayManager) bakeLayer2ColourGPU(io *SpectrumIO) {
	if dm.layer2ColourGPUReady {
		for i := range dm.layer2ColourGPU {
			rl.UnloadTexture(dm.layer2ColourGPU[i])
		}
	}
	for idx := 0; idx < 256; idx++ {
		c := io.nextColourFor(nextLayerLayer2, nextColourIndex(idx))
		img := image.NewRGBA(image.Rect(0, 0, 8, 1))
		for x := 0; x < 8; x++ {
			img.Set(x, 0, color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A})
		}
		dm.layer2ColourGPU[idx] = rl.LoadTextureFromImage(rl.NewImageFromImage(img))
	}
	dm.layer2ColourGPUReady = true
	dm.layer2ColourGPUVersion = io.nextPaletteVersion
}

// spriteGPUCacheKey identifies one distinct resolved-sprite-pixel-data
// combination for the sprite GPU texture cache: a pattern slot's raw
// bytes AND the palette offset applied to them (sprite attribute 2,
// per-SPRITE not per-pattern) both determine what resolveNextPixel
// actually outputs -- keying on pattern alone (drafted and rejected, see
// display.go's own struct-field comment on spriteGPUCache) would
// incorrectly share one baked texture across two sprites using the same
// pattern with different palette offsets.
type spriteGPUCacheKey struct {
	pattern       uint8
	paletteOffset uint8
}

// spriteGPUCacheEntry is one cached, baked sprite texture plus the
// generation/palette-version it was baked against (same staleness check
// as Layer 2's own GPU path would use, extended with an LRU recency
// counter for eviction).
type spriteGPUCacheEntry struct {
	texture    rl.Texture2D
	generation uint32
	paletteVer uint32
	lastUsed   uint64
}

// spriteGPUCache is a capped, LRU-evicted cache of baked sprite textures
// keyed by spriteGPUCacheKey -- per explicit design direction
// (2026-09-15): rather than reserve space for every possible
// (pattern, paletteOffset) combination (up to 64 patterns x 16 offsets =
// 1024) or fall back to a plain CPU-decode-and-reupload-every-frame path,
// cap the cache at a configurable size and evict the least-recently-used
// entry when full. A running program that reuses a small, stable set of
// sprite appearances (the common case) sees that working set become
// resident and render at full GPU-texture-blit speed after its first
// few frames; a program that constantly invents brand-new
// pattern/offset combinations degrades gracefully to more frequent
// rebakes rather than growing memory unboundedly.
type spriteGPUCache struct {
	capacity int
	entries  map[spriteGPUCacheKey]*spriteGPUCacheEntry
	clock    uint64
}

// defaultSpriteGPUCacheCapacity is the initial capacity NewDisplayManager
// configures -- generous enough that a typical program's actual working
// set of distinct sprite appearances fits without eviction churn (64
// cached 16x16 textures is trivial GPU memory), while still bounding
// worst-case memory for a program that varies patterns/offsets constantly.
// Configurable at runtime via DisplayManager.SetSpriteGPUCacheCapacity.
const defaultSpriteGPUCacheCapacity = 64

func newSpriteGPUCache(capacity int) *spriteGPUCache {
	if capacity < 1 {
		capacity = 1
	}
	return &spriteGPUCache{
		capacity: capacity,
		entries:  make(map[spriteGPUCacheKey]*spriteGPUCacheEntry),
	}
}

// get looks up key, bumping its recency on a hit. Returns (entry, true)
// on a hit -- the caller still must check the returned entry's
// generation/paletteVer against the current live values, since a hit
// only means "this key has been baked before," not "the bake is still
// current."
func (c *spriteGPUCache) get(key spriteGPUCacheKey) (*spriteGPUCacheEntry, bool) {
	e, ok := c.entries[key]
	if ok {
		c.clock++
		e.lastUsed = c.clock
	}
	return e, ok
}

// put inserts or replaces the entry for key. If key is not already
// present and the cache is at capacity, the least-recently-used existing
// entry is unloaded and evicted first.
func (c *spriteGPUCache) put(key spriteGPUCacheKey, texture rl.Texture2D, generation, paletteVer uint32) {
	if existing, ok := c.entries[key]; ok {
		rl.UnloadTexture(existing.texture)
	} else if len(c.entries) >= c.capacity {
		c.evictLRU()
	}
	c.clock++
	c.entries[key] = &spriteGPUCacheEntry{
		texture:    texture,
		generation: generation,
		paletteVer: paletteVer,
		lastUsed:   c.clock,
	}
}

// evictLRU unloads and removes the single least-recently-used entry.
// No-op on an empty cache (nothing to evict) -- callers only reach this
// when len(c.entries) >= c.capacity, so an empty cache with capacity 0
// would be the only way to hit that with nothing to evict, and
// newSpriteGPUCache's own capacity floor of 1 rules that out.
func (c *spriteGPUCache) evictLRU() {
	var oldestKey spriteGPUCacheKey
	var oldestUsed uint64 = ^uint64(0)
	found := false
	for k, e := range c.entries {
		if !found || e.lastUsed < oldestUsed {
			oldestKey = k
			oldestUsed = e.lastUsed
			found = true
		}
	}
	if !found {
		return
	}
	rl.UnloadTexture(c.entries[oldestKey].texture)
	delete(c.entries, oldestKey)
}

// setCapacity changes the cache's capacity, evicting least-recently-used
// entries immediately if the new capacity is smaller than the current
// entry count -- called by DisplayManager.SetSpriteGPUCacheCapacity.
func (c *spriteGPUCache) setCapacity(capacity int) {
	if capacity < 1 {
		capacity = 1
	}
	c.capacity = capacity
	for len(c.entries) > c.capacity {
		c.evictLRU()
	}
}

// clear unloads every cached texture and empties the cache -- called by
// DisplayManager.CleanupTextures.
func (c *spriteGPUCache) clear() {
	for _, e := range c.entries {
		rl.UnloadTexture(e.texture)
	}
	c.entries = make(map[spriteGPUCacheKey]*spriteGPUCacheEntry)
}

// renderSpritesGPU is T-30's sprite fast path (extended in the T-29/T-30
// completion pass for scale/rotate/mirror/composite+unified relative
// sprites): draws each visible RESOLVED sprite (spriteSystem.
// effectiveAttrs -- sprite.go) via one positioned, transformed texture
// blit, baking (or rebaking) a texture only when the (pattern,
// paletteOffset) combination is not already cached, or the cached bake
// has gone stale -- its pattern slot's raw pixel data changed
// (spriteSystem.patternGeneration) or the palette version it was
// resolved against is no longer current (SpectrumIO.nextPaletteVersion).
//
// Design note on scale/rotate/mirror: the cache key deliberately stays
// (pattern, paletteOffset) ONLY, with no transform axis, even though a
// transform-inclusive key was the recommended design going into this
// pass. Once raylib's actual DrawTexturePro signature was confirmed
// against the vendored raylib-go source (source Rectangle, dest
// Rectangle, origin, rotation, tint), it became clear the GPU can apply
// mirror (negative source-rect width/height), rotation (the rotation
// parameter, real hardware's fixed 90-degree case), and scale (the dest
// rect's size) as pure per-draw-call geometry against ONE baked,
// untransformed 16x16 texture -- no separate baked variant per
// transform combination is needed at all. This is strictly better than
// the transform-inclusive key: fewer cache entries, zero extra bake
// cost for a sprite that merely rotates or rescales, and the same
// underlying "bake resolved pixel data once" principle this cache
// already used, just recognising that geometry belongs in the draw call
// rather than the bake.
//
// The cache (DisplayManager.spriteGPUCache) is a capped LRU, not one
// entry per possible combination -- see spriteGPUCache's own doc comment
// for why. Called from renderNextLayersGPU (layer2.go's layersAboveULA
// determines whether/when this runs relative to Layer 2), not directly
// from DisplayManager.Render, so ordering against Layer 2 is always
// correct for the current NR 0x15 priority rather than a fixed sequence.
func renderSpritesGPU(dm *DisplayManager, io *SpectrumIO, offsetX, offsetY float32, multiplier int) {
	if !io.spriteEnabled() {
		return
	}
	mult := float32(multiplier)
	resolved := io.sprites.effectiveAttrs()
	for i := 0; i < spriteCount; i++ {
		r := resolved[i]
		if !r.visible {
			continue
		}
		key := spriteGPUCacheKey{pattern: r.pattern, paletteOffset: r.paletteOffset}
		gen := io.sprites.patternGeneration[r.pattern]
		entry, hit := dm.spriteGPUCache.get(key)
		stale := !hit || entry.generation != gen || entry.paletteVer != io.nextPaletteVersion
		if stale {
			img := image.NewRGBA(image.Rect(0, 0, spriteWidth, spriteHeight))
			for ly := 0; ly < spriteHeight; ly++ {
				for lx := 0; lx < spriteWidth; lx++ {
					idx, opaque := io.spritePixelIndexAt(r, lx, ly)
					if !opaque {
						continue // leaves img.Pix at zero value: alpha 0, transparent
					}
					c := io.nextColourFor(nextLayerSprites, idx)
					img.SetRGBA(lx, ly, color.RGBA{R: c.R, G: c.G, B: c.B, A: c.A})
				}
			}
			texture := rl.LoadTextureFromImage(rl.NewImageFromImage(img))
			dm.spriteGPUCache.put(key, texture, gen, io.nextPaletteVersion)
			entry, _ = dm.spriteGPUCache.get(key)
		}

		// Source rect: the full baked 16x16 texture, with a negative
		// width/height flipping that axis -- raylib's own documented
		// convention for DrawTexturePro (a negative source dimension
		// mirrors the sampled region), applied here instead of baking a
		// separate mirrored texture.
		srcW, srcH := float32(spriteWidth), float32(spriteHeight)
		if r.xMirror {
			srcW = -srcW
		}
		if r.yMirror {
			srcH = -srcH
		}
		source := rl.NewRectangle(0, 0, srcW, srcH)

		// Dest rect: final on-screen size and position, post-scale and
		// post-rotation footprint swap (spriteFootprint, sprite.go) --
		// origin (below) is set to the pre-rotation texture's own
		// center so the rotation this draw call performs turns the
		// sprite in place around its own centre, matching "rotating the
		// anchor causes all the relatives to rotate around the anchor"
		// (wiki, sprite.go's header) for the unified-relative case, and
		// simply "the sprite rotates in place" for every other case.
		destW := float32(spriteWidth) * float32(r.scaleX.factor()) * mult
		destH := float32(spriteHeight) * float32(r.scaleY.factor()) * mult
		// dest's (x,y) is where ORIGIN lands -- with origin at the
		// texture's own centre (destW/2, destH/2 in DEST space, which
		// DrawTexturePro measures in dest-rect units), so the dest
		// rect's top-left must be offset by +origin to keep the
		// sprite's final top-left corner at (r.x, r.y) after rotation
		// re-centres it.
		footW, footH := spriteFootprint(r)
		destX := offsetX + float32(r.x)*mult + float32(footW)*mult/2
		destY := offsetY + float32(r.y)*mult + float32(footH)*mult/2
		dest := rl.NewRectangle(destX, destY, destW, destH)
		origin := rl.NewVector2(destW/2, destH/2)
		var rotation float32
		if r.rotate {
			rotation = 90
		}
		rl.DrawTexturePro(entry.texture, source, dest, origin, rotation, rl.White)
	}
}

// renderNextLayersGPU draws Layer 2 and/or the sprite layer as additional
// GPU passes after the ULA layer, in the correct relative order per NR
// 0x15 (layersAboveULA, layer2.go) -- computed once per frame here rather
// than each layer's own draw function independently re-deriving "should I
// draw at all" from a two-layer question that can't express "am I above
// or below the OTHER non-ULA layer." Each layer's own draw function
// (renderLayer2GPU/renderSpritesGPU) still gates on its own enabled check
// and, in renderLayer2GPU's case, its own layerIsAboveULA re-check too --
// harmless redundancy that keeps both functions safe to call directly
// (layer2_gui_test.go's existing tests call renderLayer2GPU on its own,
// bypassing this orchestrator entirely).
func renderNextLayersGPU(dm *DisplayManager, io *SpectrumIO, mem *SpectrumMemory, offsetX, offsetY float32, multiplier int) {
	nr15 := io.GetNextRegister(0x15)
	for _, layer := range layersAboveULA(nr15) {
		switch layer {
		case nextLayerLayer2:
			renderLayer2GPU(dm, io, mem, offsetX, offsetY, multiplier)
		case nextLayerSprites:
			renderSpritesGPU(dm, io, offsetX, offsetY, multiplier)
		}
	}
}
