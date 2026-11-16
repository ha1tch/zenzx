package main

import (
	"testing"
	"unsafe"
)

// TestNextExtraRAMAllocatedOnFirstWrite: the 208 extra Next pages (about
// 1.7 MB) must not be carried by every machine. Unwritten pages read as zero,
// as the old zero-initialised array did; the first write allocates.
func TestNextExtraRAMAllocatedOnFirstWrite(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	if mem.nextExtraRAM != nil {
		t.Fatal("a fresh memory must not allocate the Next page store")
	}
	if got := mem.nextPageRead(20, 5); got != 0 {
		t.Errorf("unwritten extra page read = 0x%02X, want 0", got)
	}
	if mem.nextExtraRAM != nil {
		t.Fatal("reading an unwritten page must not allocate")
	}
	if got := mem.nextPageRead(0xE0, 0); got != 0xFF {
		t.Errorf("reserved page read = 0x%02X, want 0xFF (unchanged)", got)
	}
	mem.nextPageWrite(20, 5, 0x9A)
	if mem.nextExtraRAM == nil {
		t.Fatal("the first write must allocate the store")
	}
	if got := mem.nextPageRead(20, 5); got != 0x9A {
		t.Errorf("read back = 0x%02X, want 0x9A", got)
	}
	if got := mem.nextPageRead(21, 5); got != 0 {
		t.Errorf("a different page must stay independent, got 0x%02X", got)
	}
}

// TestSpectrumMemoryStaysSmall guards the 1.7 MB array coming back inline.
func TestSpectrumMemoryStaysSmall(t *testing.T) {
	const limit = 512 * 1024
	if sz := unsafe.Sizeof(SpectrumMemory{}); sz > limit {
		t.Errorf("SpectrumMemory is %d bytes, over the %d-byte guard", sz, limit)
	}
}
