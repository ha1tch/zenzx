//go:build !headless

package main

import "testing"

// T-29's Layer 2 GPU fast path (renderLayer2GPU, videorender_gpu.go) is
// only reachable from the GUI build -- it calls raylib draw functions
// directly, so it cannot be exercised from the headless test suite the
// way layer2_test.go's SpectrumIO-level tests are. A real draw call needs
// a live GL context (rl.BeginDrawing/InitWindow), which this test suite
// does not set up, so what's tested here is deliberately narrower: that
// renderLayer2GPU's early-return gates (Layer 2 disabled, or NR 0x15
// putting ULA on top) are taken correctly and the function returns
// without ever reaching a draw call. A DisplayManager built via
// NewDisplayManager has gpuTexturesReady false and every paperColourGPU
// entry at its zero value (no live GL context, no window) -- if either
// gate were broken, this would attempt a raylib draw call against an
// uninitialised texture with no drawing in progress, which is exactly
// the kind of failure this test is positioned to catch (a panic or
// raylib-internal error) rather than a silent false pass.

func TestRenderLayer2GPUNoOpWhenLayer2Disabled(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	dm := NewDisplayManager(screen)
	// mem.isNext is false (newTestMemoryAndScreen never calls
	// EnableNext) -- layer2Enabled() must be false, and renderLayer2GPU
	// must return before touching dm.paperColourGPU at all.
	renderLayer2GPU(dm, io, mem, 0, 0, 1)
}

func TestRenderLayer2GPUNoOpWhenULAOnTop(t *testing.T) {
	mem, screen := newTestMemoryAndScreen()
	mem.EnableNext()
	io := NewSpectrumIO(mem, nil)
	dm := NewDisplayManager(screen)
	// NR 0x15 = 0b00010000 -> bits 4-2 = %100 = U S L -- ULA on top,
	// matching layer2ULAOnTopDecodesRegister0x15's own table in
	// layer2_test.go. renderLayer2GPU must return before drawing.
	io.SetNextRegister(0x15, 0b00010000)
	renderLayer2GPU(dm, io, mem, 0, 0, 1)
}

// T-31 (wave ti0.r5) added dm.layer2ColourGPU/bakeLayer2ColourGPU
// (videorender_gpu.go) behind the same two gates the two tests above
// already exercise -- both early returns happen before renderLayer2GPU
// ever reaches the staleness check or a bake, so no new gate test is
// needed for them specifically. bakeLayer2ColourGPU itself is untestable
// here for the identical reason recorded in sprite_gui_test.go's own file
// header: it calls rl.LoadTextureFromImage, which (like rl.UnloadTexture)
// segfaults without a prior rl.InitWindow. What IS exercised below,
// without reaching either raylib call, is CleanupTextures' own gate: a
// freshly constructed DisplayManager has layer2ColourGPUReady false (the
// zero value), so CleanupTextures must return without ever touching
// dm.layer2ColourGPU -- the same class of "gate reachable without a GL
// context" check TestRenderLayer2GPUNoOpWhenLayer2Disabled above performs
// for renderLayer2GPU.
func TestCleanupTexturesNoOpOnLayer2ColourCacheWhenNeverBaked(t *testing.T) {
	_, screen := newTestMemoryAndScreen()
	dm := NewDisplayManager(screen)
	if dm.layer2ColourGPUReady {
		t.Fatalf("layer2ColourGPUReady on a freshly constructed DisplayManager = true, want false (zero value)")
	}
	dm.CleanupTextures()
}
