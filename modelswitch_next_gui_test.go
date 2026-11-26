//go:build !headless

package zenzx

import "testing"

// Switching live from the Next to another model leaves nothing of the Next behind: not the 8K pages
// the memory editors offer, not the Z80N, and the other way round works too.
func TestSwitchingAwayFromTheNextLeavesNothingOfIt(t *testing.T) {
	for _, to := range []string{"48k", "128k", "plus2", "plus3", "spanish128k", "ts2068"} {
		zx := machineOf(t, "next")
		if !zx.memory.isNext || !zx.cpu.Z80N {
			t.Fatalf("a Next is not a Next: isNext %v, Z80N %v", zx.memory.isNext, zx.cpu.Z80N)
		}
		switchModelLive(zx, to, "./rom")
		want := pagingOf(t, to)
		got := zx.memory.PageLayout()
		if zx.memory.isNext || zx.cpu.Z80N || zx.cpu.NextregWrite != nil {
			t.Errorf("%s: isNext %v, Z80N %v, after the switch", to, zx.memory.isNext, zx.cpu.Z80N)
		}
		if got.Kind != want.Kind || got.Size != want.Size || got.Count != want.Count {
			t.Errorf("%s after the Next: pages %+v, want %+v", to, got, want)
		}
		if n := len(zx.memory.hotSpaces()); n != 1+want.Count {
			t.Errorf("%s after the Next: the editor offers %d memories, want %d", to, n, 1+want.Count)
		}
		w := newBinaryWindow(zx, binTabHot)
		if len(w.hot.spaces) != 1+want.Count || w.paged() != (want.Kind != pagesNone) {
			t.Errorf("%s after the Next: the window offers %d memories", to, len(w.hot.spaces))
		}
	}
	// and a Next switched to from another model is one
	zx := machineOf(t, "128k")
	switchModelLive(zx, "next", "./rom")
	if !zx.memory.isNext || !zx.cpu.Z80N || zx.memory.PageLayout().Kind != pagesNext {
		t.Errorf("128K to Next: isNext %v, Z80N %v, layout %+v", zx.memory.isNext, zx.cpu.Z80N, zx.memory.PageLayout())
	}
}

// pagingOf is what a model's table says its pages are.
func pagingOf(t *testing.T, model string) pageLayout {
	t.Helper()
	for _, c := range pagingByModel {
		if c.model == model {
			return pageLayout{Kind: c.kind, Size: c.size, Count: c.count}
		}
	}
	t.Fatalf("no model %q in the paging table", model)
	return pageLayout{}
}
