//go:build !headless

package main

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// T-30's sprite GPU fast path (renderSpritesGPU, renderNextLayersGPU,
// videorender_gpu.go) is only reachable from the GUI build, the same way
// T-29's renderLayer2GPU is (layer2_gui_test.go) -- and for the same
// reason, this suite tests its early-return gates rather than actual
// draw output: a real bake-and-blit needs a live GL context
// (rl.InitWindow), which this suite does not set up.
//
// One thing was checked empirically this session before writing these
// tests, not assumed: calling any raylib GPU function -- including
// rl.UnloadTexture on a harmless zero-value or fake Texture2D -- without
// a prior rl.InitWindow segfaults (a nil GL function pointer, confirmed
// via a throwaway scratch test, removed after the check). That rules out
// unit-testing spriteGPUCache's eviction (evictLRU), key-replacement
// (put on an already-present key), and clear paths directly in this
// container, since all three call rl.UnloadTexture -- exactly the same
// category of limitation as renderLayer2GPU's own draw call being
// untestable here, just one layer further down. What IS exercised below,
// without ever reaching rl.UnloadTexture, is: construction, a miss on an
// empty cache, a hit after inserting under capacity (no eviction, no
// replacement), and growing/shrinking capacity without crossing the
// current entry count (so setCapacity's own eviction loop never fires).
// The eviction/replace/clear paths remain covered only by reading, not
// by a running test -- recorded as a known gap in docs/TRACKING.md's
// T-30 entry rather than left silent.

func TestRenderSpritesGPUNoOpWhenSpritesDisabled(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	dm := NewDisplayManager(screen)
	// NR 0x15 bit 0 clear (construction default) -- spriteEnabled() must
	// be false, and renderSpritesGPU must return before touching
	// dm.spriteGPUCache or any sprite attribute at all.
	renderSpritesGPU(dm, io, 0, 0, 1)
}

func TestRenderSpritesGPUNoOpWhenNoVisibleSprites(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	dm := NewDisplayManager(screen)
	io.SetNextRegister(0x15, 0x01) // sprites enabled, bit 0 set
	// Every sprite starts invisible (attrs' zero value has visible ==
	// false) -- the loop must run all 128 iterations and skip every one
	// via the `!a.visible` continue, never reaching the cache lookup or
	// a draw call.
	renderSpritesGPU(dm, io, 0, 0, 1)
}

func TestRenderNextLayersGPUNoOpWhenNothingEnabled(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	dm := NewDisplayManager(screen)
	// Construction default: NR 0x15 = 0x08 (bits 4-2 = %010, S U L) --
	// layersAboveULA(0x08) is [Sprites] (layer2.go's nextLayerOrder), so
	// this exercises the dispatch loop's Sprites case, which must itself
	// gate on spriteEnabled() (false here) and return without drawing.
	renderNextLayersGPU(dm, io, mem, 0, 0, 1)
}

func TestRenderNextLayersGPUNoOpWhenLayer2AboveULAButDisabled(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	dm := NewDisplayManager(screen)
	// bits 4-2 = %100 = U S L -- layersAboveULA puts both Sprites and
	// Layer2 above ULA (order: U, S, L bottom-to-top -> above ULA is
	// [Sprites, Layer2]), so this exercises the dispatch loop reaching
	// BOTH cases in one call, with Layer 2 disabled (layer2Enabled()
	// false, mem.isNext true but no layer2 NextREG ever written to
	// enable it) and sprites disabled (bit 0 clear) -- both must return
	// via their own gates without drawing.
	io.SetNextRegister(0x15, 0b00010000)
	renderNextLayersGPU(dm, io, mem, 0, 0, 1)
}

// --- spriteGPUCache: GL-context-free logic only (see file header) ---

func TestSpriteGPUCacheNewFloorsCapacityAtOne(t *testing.T) {
	c := newSpriteGPUCache(0)
	if c.capacity != 1 {
		t.Errorf("capacity = %d, want 1 (floored)", c.capacity)
	}
	c2 := newSpriteGPUCache(-5)
	if c2.capacity != 1 {
		t.Errorf("capacity = %d, want 1 (floored from negative)", c2.capacity)
	}
	c3 := newSpriteGPUCache(64)
	if c3.capacity != 64 {
		t.Errorf("capacity = %d, want 64", c3.capacity)
	}
}

func TestSpriteGPUCacheGetMissOnEmptyCache(t *testing.T) {
	c := newSpriteGPUCache(4)
	_, hit := c.get(spriteGPUCacheKey{pattern: 0, paletteOffset: 0})
	if hit {
		t.Error("get on empty cache returned a hit")
	}
}

func TestSpriteGPUCachePutThenGetHit(t *testing.T) {
	c := newSpriteGPUCache(4)
	key := spriteGPUCacheKey{pattern: 3, paletteOffset: 5}
	fake := rl.Texture2D{ID: 42, Width: spriteWidth, Height: spriteHeight}
	// Single insert under capacity: the key is not already present and
	// len(entries) (0) < capacity (4), so put's own logic never reaches
	// rl.UnloadTexture -- safe without a GL context.
	c.put(key, fake, 7, 3)
	entry, hit := c.get(key)
	if !hit {
		t.Fatal("get after put missed")
	}
	if entry.texture.ID != 42 || entry.generation != 7 || entry.paletteVer != 3 {
		t.Errorf("entry = %+v, want texture.ID=42 generation=7 paletteVer=3", entry)
	}
}

func TestSpriteGPUCacheGetMissOnDifferentKey(t *testing.T) {
	c := newSpriteGPUCache(4)
	c.put(spriteGPUCacheKey{pattern: 1, paletteOffset: 0}, rl.Texture2D{ID: 1}, 1, 1)
	// Same pattern, different palette offset -- must be a distinct key
	// (this is precisely the bug the pattern-only-keyed design, drafted
	// and rejected earlier this session, would have gotten wrong).
	_, hit := c.get(spriteGPUCacheKey{pattern: 1, paletteOffset: 1})
	if hit {
		t.Error("get hit on a key differing only in paletteOffset")
	}
}

func TestSpriteGPUCacheSetCapacityGrowDoesNotEvict(t *testing.T) {
	c := newSpriteGPUCache(2)
	c.put(spriteGPUCacheKey{pattern: 0}, rl.Texture2D{ID: 1}, 0, 0)
	c.put(spriteGPUCacheKey{pattern: 1}, rl.Texture2D{ID: 2}, 0, 0)
	// Growing capacity from 2 (at capacity, 2 entries) to 8: len(entries)
	// (2) never exceeds the new capacity (8) at any point, so
	// setCapacity's own eviction loop condition is never true -- safe.
	c.setCapacity(8)
	if c.capacity != 8 {
		t.Errorf("capacity = %d, want 8", c.capacity)
	}
	if len(c.entries) != 2 {
		t.Errorf("len(entries) = %d, want 2 (grow must not evict)", len(c.entries))
	}
	if _, hit := c.get(spriteGPUCacheKey{pattern: 0}); !hit {
		t.Error("pattern 0 entry lost after growing capacity")
	}
	if _, hit := c.get(spriteGPUCacheKey{pattern: 1}); !hit {
		t.Error("pattern 1 entry lost after growing capacity")
	}
}

func TestSpriteGPUCacheSetCapacityFloorsAtOne(t *testing.T) {
	c := newSpriteGPUCache(4)
	c.put(spriteGPUCacheKey{pattern: 0}, rl.Texture2D{ID: 1}, 0, 0)
	// Shrinking to 1 with only 1 entry present must not need to evict
	// (len == new capacity, not >), so this stays GL-context-free; it
	// only exercises the floor-at-1 clamp on a below-one request.
	c.setCapacity(0)
	if c.capacity != 1 {
		t.Errorf("capacity = %d, want 1 (floored)", c.capacity)
	}
	if len(c.entries) != 1 {
		t.Errorf("len(entries) = %d, want 1 (no eviction needed)", len(c.entries))
	}
}
