package zenzx

import (
	"strings"
	"testing"
)

// The spaces of the hex editor, on every model.

func TestHotSpacesOfEveryModel(t *testing.T) {
	for _, c := range pagingByModel {
		m := machineOf(t, c.model).memory
		sp := m.hotSpaces()
		if len(sp) != 1+c.count {
			t.Errorf("%s: %d spaces, want the CPU's and %d pages", c.model, len(sp), c.count)
			continue
		}
		if sp[0] != hotCPU || m.hotSize(sp[0]) != 0x10000 {
			t.Errorf("%s: the first space is %+v of %d bytes, want the CPU's 64K", c.model, sp[0], m.hotSize(sp[0]))
		}
		for i, s := range sp[1:] {
			if s.Page != i || m.hotSize(s) != c.size {
				t.Errorf("%s: space %d is page %d of %d bytes, want page %d of %d", c.model, i+1, s.Page, m.hotSize(s), i, c.size)
			}
		}
	}
}

// What is written to a space is what is read from it, in every page of every model, and the CPU's
// 64K takes writes in RAM and refuses them in ROM.
func TestHotWriteReadsBack(t *testing.T) {
	for _, c := range pagingByModel {
		zx := machineOf(t, c.model)
		m := zx.memory
		for _, sp := range m.hotSpaces() {
			off := 0x1D00 // not in the screen, in any page
			if sp.Page < 0 {
				off = 0x9000
			}
			for _, v := range []byte{0x00, 0xA5, 0xFF} {
				if err := m.hotWrite(sp, off, v); err != nil {
					t.Fatalf("%s %+v: write 0x%02X: %v", c.model, sp, v, err)
				}
				if got := m.hotRead(sp, off, 1); len(got) != 1 || got[0] != v {
					t.Errorf("%s %+v: wrote 0x%02X, read %v", c.model, sp, v, got)
				}
			}
			if !m.hotWritable(sp, off) {
				t.Errorf("%s %+v: 0x%X is not writable", c.model, sp, off)
			}
		}
		// the ROM is not written to, in the CPU's space, and says so
		if m.hotWritable(hotCPU, 0x0100) {
			t.Errorf("%s: 0x0100 is writable in the CPU's space", c.model)
		}
		before := m.hotRead(hotCPU, 0x0100, 1)
		if err := m.hotWrite(hotCPU, 0x0100, ^before[0]); err == nil {
			t.Errorf("%s: a write to the ROM was not refused", c.model)
		}
		if after := m.hotRead(hotCPU, 0x0100, 1); after[0] != before[0] {
			t.Errorf("%s: the ROM changed", c.model)
		}
	}
}

// Reads stop at the end of the space and do not go on into the next page, and writes outside it are
// refused.
func TestHotReadStopsAtTheEnd(t *testing.T) {
	for _, c := range pagingByModel {
		m := machineOf(t, c.model).memory
		if got := m.hotRead(hotCPU, 0xFFF0, 100); len(got) != 16 {
			t.Errorf("%s: %d bytes read from 0xFFF0 of the CPU's space, want 16", c.model, len(got))
		}
		if c.kind == pagesNone {
			continue
		}
		last := hotSpace{Page: c.count - 1}
		if got := m.hotRead(last, c.size-4, 100); len(got) != 4 {
			t.Errorf("%s: %d bytes read from the end of the last page, want 4", c.model, len(got))
		}
		first := hotSpace{Page: 0}
		if got := m.hotRead(first, c.size-4, 100); len(got) != 4 {
			t.Errorf("%s: %d bytes read from the end of page 0, want 4 (not on into page 1)", c.model, len(got))
		}
		if err := m.hotWrite(first, c.size, 1); err == nil {
			t.Errorf("%s: a write past the end of a page was not refused", c.model)
		}
		if err := m.hotWrite(first, -1, 1); err == nil {
			t.Errorf("%s: a write before the start of a page was not refused", c.model)
		}
		if got := m.hotRead(first, -1, 4); got != nil {
			t.Errorf("%s: a read before the start gave %v", c.model, got)
		}
	}
}

// The list says where each page is paged in now, and the screen's.
func TestHotNamesSayWhereAPageIs(t *testing.T) {
	zx := machineOf(t, "128k")
	m := zx.memory
	has := func(sp hotSpace, want ...string) {
		t.Helper()
		got := m.hotName(sp)
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("%q does not say %q", got, w)
			}
		}
	}
	has(hotCPU, "CPU", "64K")
	has(hotSpace{Page: 5}, "Bank 5", "16K", "at 0x4000", "screen")
	has(hotSpace{Page: 2}, "Bank 2", "at 0x8000")
	has(hotSpace{Page: 0}, "Bank 0", "at 0xC000")
	if n := m.hotName(hotSpace{Page: 1}); strings.Contains(n, " at ") || strings.Contains(n, "screen") {
		t.Errorf("bank 1 is not paged in nor shown: %q", n)
	}
	// page bank 1 in at the top, and shown screen 7
	m.SetPaging(0x01 | 0x08)
	has(hotSpace{Page: 1}, "at 0xC000")
	has(hotSpace{Page: 7}, "screen")
	if n := m.hotName(hotSpace{Page: 5}); strings.Contains(n, "screen") {
		t.Errorf("bank 5 is no longer the screen: %q", n)
	}
}

// A +3 in special paging has RAM at 0, and the list says which.
func TestHotNamesInPlus3SpecialPaging(t *testing.T) {
	m := machineOf(t, "plus3").memory
	m.SetPlus3Paging(0x01) // special, configuration 0: banks 0, 1, 2, 3
	want := map[int]string{0: "at 0x0000", 1: "at 0x4000", 2: "at 0x8000", 3: "at 0xC000"}
	for p, w := range want {
		if n := m.hotName(hotSpace{Page: p}); !strings.Contains(n, w) {
			t.Errorf("special 0: bank %d is %q, want %q", p, n, w)
		}
	}
	// and the CPU's space is then all RAM: nothing is read-only
	for _, a := range []int{0, 0x1FFF, 0x2000, 0x3FFF} {
		if !m.hotWritable(hotCPU, a) {
			t.Errorf("special 0: 0x%04X is not writable", a)
		}
	}
}

// On a Next the pages are the 8K ones, in the slots; a page in two slots is listed at both.
func TestHotNamesOnANext(t *testing.T) {
	m := machineOf(t, "next").memory
	m.SetNextSlot(2, 40)
	m.SetNextSlot(6, 40)
	n := m.hotName(hotSpace{Page: 40})
	if !strings.Contains(n, "Page 40") || !strings.Contains(n, "8K") || !strings.Contains(n, "0x4000, 0xC000") {
		t.Errorf("page 40 in slots 2 and 6: %q", n)
	}
	// a page in no slot says nothing about where
	if n := m.hotName(hotSpace{Page: 201}); strings.Contains(n, " at ") {
		t.Errorf("page 201 is in no slot: %q", n)
	}
	// the screen is the pages of bank 5, and bank 5 is pages 10 and 11
	if n := m.hotName(hotSpace{Page: 10}); !strings.Contains(n, "screen") {
		t.Errorf("page 10 is the screen's: %q", n)
	}
	if n := m.hotName(hotSpace{Page: 12}); strings.Contains(n, "screen") {
		t.Errorf("page 12 is not the screen's: %q", n)
	}
}

// Writing the displayed bank through the editor moves the picture too, as a CPU write does.
func TestHotWriteKeepsTheScreenCopy(t *testing.T) {
	zx := machineOf(t, "128k")
	m := zx.memory
	if err := m.hotWrite(hotSpace{Page: 5}, 0x10, 0xC3); err != nil {
		t.Fatal(err)
	}
	if got := m.Read(0x4010); got != 0xC3 {
		t.Errorf("the CPU reads 0x%02X at 0x4010 after the editor wrote bank 5, want 0xC3", got)
	}
	if got := m.screen.bitmap[0x10]; got != 0xC3 {
		t.Errorf("the screen's copy is 0x%02X, want 0xC3", got)
	}
}
