//go:build !headless

package zenzx

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenui"
)

// Page Up and Page Down page the dump of Peek and Poke by what it shows, as Back and Forward do,
// across the end of a page, and stop where they stop.
func TestPageKeysPageThePeekDump(t *testing.T) {
	w, _, r := binWindow(t, "next", binTabPeek)
	n := w.dumpBytes()
	pick(t, w.peekMem, 6)
	w.pAddr.Set(0x1FF8)
	binFrame(w, r, zenui.Input{})
	binKey(w, r, zenui.KeyPageDown)
	if w.peekMem.page() != 7 || w.pAddr.Value() != n-8 {
		t.Errorf("Page Down from page 6, 0x1FF8: page %d, 0x%X; want page 7, 0x%X", w.peekMem.page(), w.pAddr.Value(), n-8)
	}
	binKey(w, r, zenui.KeyPageUp)
	if w.peekMem.page() != 6 || w.pAddr.Value() != 0x1FF8 {
		t.Errorf("Page Up came to page %d, 0x%X; want page 6, 0x1FF8", w.peekMem.page(), w.pAddr.Value())
	}
	// the dump shows the new place the same frame
	if want := "1FF8"; w.dump[0][:4] != want {
		t.Errorf("the dump starts at %s, want %s", w.dump[0][:4], want)
	}
	// at the ends nothing moves
	pick(t, w.peekMem, -1)
	w.pAddr.Set(0)
	binFrame(w, r, zenui.Input{})
	binKey(w, r, zenui.KeyPageUp)
	if w.pAddr.Value() != 0 {
		t.Errorf("Page Up at 0: 0x%X", w.pAddr.Value())
	}
	w.pAddr.Set(0xFFFF - n + 1)
	binFrame(w, r, zenui.Input{})
	binKey(w, r, zenui.KeyPageDown)
	if w.pAddr.Value() != 0xFFFF-n+1 {
		t.Errorf("Page Down at the end: 0x%X, want it where it was", w.pAddr.Value())
	}
	// with the Bytes box being edited, and with the list open, as the arrows are
	w.pAddr.Set(0x8000)
	binClick(w, r, w.pBytesA)
	binKey(w, r, zenui.KeyPageDown)
	if w.pAddr.Value() != 0x8000+n {
		t.Errorf("Page Down in the Bytes box: 0x%X, want 0x%X", w.pAddr.Value(), 0x8000+n)
	}
	w.pBytes.Blur()
	binClick(w, r, w.peekMem.selRect)
	binKey(w, r, zenui.KeyPageDown)
	if w.pAddr.Value() != 0x8000+n {
		t.Errorf("Page Down with the memory list open moved the dump: 0x%X", w.pAddr.Value())
	}
}

// Page Up and Page Down in Hot Memory go a page of the dump at a time, the dump follows, in the
// CPU's memory and in a page, and a held key goes on.
func TestPageKeysPageTheHotEditor(t *testing.T) {
	for _, c := range []struct {
		name  string
		space int
	}{{"the CPU's memory", 0}, {"a bank", 3}} {
		w, _, r := binWindow(t, "128k", binTabHot)
		h := w.hot
		h.selectSpace(c.space)
		h.goTo(0x0800)
		page := h.rows * h.perRow
		binKey(w, r, zenui.KeyPageDown)
		if h.pos != 0x0800+page {
			t.Errorf("%s: Page Down: 0x%X, want 0x%X", c.name, h.pos, 0x0800+page)
		}
		if h.pos < h.top || h.pos >= h.top+page {
			t.Errorf("%s: the cursor 0x%X is not in the dump at 0x%X", c.name, h.pos, h.top)
		}
		binKey(w, r, zenui.KeyPageUp)
		if h.pos != 0x0800 {
			t.Errorf("%s: Page Up: 0x%X, want 0x800", c.name, h.pos)
		}
		// held: a page a frame, and then it stops at the end of the space
		for i := 0; i < 1000; i++ {
			binKey(w, r, zenui.KeyPageDown)
		}
		if h.pos < h.size()-page || h.pos > h.size()-1 {
			t.Errorf("%s: held Page Down ended at 0x%X of %d", c.name, h.pos, h.size())
		}
		if h.pos < h.top || h.pos >= h.top+page {
			t.Errorf("%s: the cursor 0x%X is not in the dump at 0x%X", c.name, h.pos, h.top)
		}
		for i := 0; i < 1000; i++ {
			binKey(w, r, zenui.KeyPageUp)
		}
		if h.pos >= page || h.top != 0 {
			t.Errorf("%s: held Page Up ended at 0x%X, top 0x%X", c.name, h.pos, h.top)
		}
	}
}

// Keys that are repeating on a frame are added to it, once, and only the ones that move or delete.
func TestKeyRepeatAddsTheHeldKeys(t *testing.T) {
	held := map[int32]bool{rl.KeyPageDown: true, rl.KeyUp: true, rl.KeyEnter: true}
	repeating := func(k int32) bool { return held[k] }
	got := withKeyRepeat(zenui.Input{Keys: []zenui.Key{zenui.KeyUp}}, repeating)
	count := map[zenui.Key]int{}
	for _, k := range got.Keys {
		count[k]++
	}
	if count[zenui.KeyPageDown] != 1 || count[zenui.KeyUp] != 1 {
		t.Errorf("keys %v: want Page Down added and Up not doubled", got.Keys)
	}
	if count[zenui.KeyEnter] != 0 || len(got.Keys) != 2 {
		t.Errorf("keys %v: Enter must not repeat", got.Keys)
	}
	if none := withKeyRepeat(zenui.Input{}, func(int32) bool { return false }); len(none.Keys) != 0 {
		t.Errorf("nothing was held and keys %v were added", none.Keys)
	}
}
