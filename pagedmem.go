// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Reaching RAM that is not where the CPU can see it. -bin and -binaddr put a file in the memory
// as the CPU sees it now, so on a 128K, a +2, a +3 or a Next what the file lands in depends on
// which banks are paged in; Snapshot > Load Binary and -binpage name the page instead, and so does
// the Peek and Poke tab. The memory has the pages in one of two shapes (PageLayout): the 128K family
// has eight 16K RAM banks, 0 to 7, and a Next has 224 8K pages, 0 to 223, of which the first 16
// are those same banks (page 2N and 2N+1 are bank N) and the rest are the Next's extra RAM. A 48K
// and a TS2068 have no paging: their RAM is only where the CPU sees it.
//
// The writes go through the same screen mirror as a CPU write does: the picture the renderer
// draws is a copy of the displayed bank's first 6912 bytes (and on a Next of the displayed
// bank's lower half), so a page that is the screen's is written in both places and read from the
// copy, which is what the CPU reads.

// pageKind says how a memory is paged.
type pageKind int

const (
	pagesNone  pageKind = iota // no paging: 48K, TS2068
	pagesBanks                 // eight 16K RAM banks: 128K, +2, +2A, +3
	pagesNext                  // 224 8K pages: Next
)

// pageLayout is a memory's pages: how many, and how big each is.
type pageLayout struct {
	Kind        pageKind
	Size, Count int
}

// PageLayout says how the memory is paged.
func (m *SpectrumMemory) PageLayout() pageLayout {
	switch {
	case m.isNext:
		return pageLayout{Kind: pagesNext, Size: 0x2000, Count: nextTotalPages}
	case m.is128K:
		return pageLayout{Kind: pagesBanks, Size: 0x4000, Count: len(m.ram)}
	}
	return pageLayout{}
}

// pageWord is what a page is called on the layout: a bank, or a page.
func (l pageLayout) pageWord() string {
	if l.Kind == pagesBanks {
		return "bank"
	}
	return "page"
}

// readPageByte reads the byte at linear position idx of the pages, as the CPU would read it if the
// page were paged in: the screen's own copy for the displayed bank.
func (m *SpectrumMemory) readPageByte(l pageLayout, idx int) uint8 {
	p, o := idx/l.Size, idx%l.Size
	if l.Kind == pagesNext {
		return m.nextPageRead(uint8(p), uint16(o))
	}
	if p == int(m.screenBank) {
		switch {
		case o < 6144:
			return m.screen.bitmap[o]
		case o < 6912:
			return m.screen.attributes[o-6144]
		}
	}
	return m.ram[p][o]
}

// writePageByte writes the byte at linear position idx of the pages, and the screen's copy when
// the page is the displayed one.
func (m *SpectrumMemory) writePageByte(l pageLayout, idx int, v uint8) {
	p, o := idx/l.Size, idx%l.Size
	if l.Kind == pagesNext {
		m.nextPageWrite(uint8(p), uint16(o), v)
		m.nextMirrorScreenWrite(uint8(p), uint16(o), v)
		return
	}
	m.ram[p][o] = v
	if p == int(m.screenBank) {
		switch {
		case o < 6144:
			m.screen.bitmap[o] = v
		case o < 6912:
			m.screen.attributes[o-6144] = v
		}
	}
}

// pageSpan checks a page and an offset in it, and returns where in the pages they are: the linear
// position of the first byte and how many bytes there are from it to the last page's end.
func (m *SpectrumMemory) pageSpan(page, offset int) (l pageLayout, idx, room int, err error) {
	l = m.PageLayout()
	switch {
	case l.Kind == pagesNone:
		return l, 0, 0, fmt.Errorf("this model has no paged memory: its RAM is where the CPU sees it")
	case page < 0 || page >= l.Count:
		return l, 0, 0, fmt.Errorf("there is no %s %d: they are 0 to %d", l.pageWord(), page, l.Count-1)
	case offset < 0 || offset >= l.Size:
		return l, 0, 0, fmt.Errorf("an address in a %s is 0 to 0x%X, not 0x%X", l.pageWord(), l.Size-1, offset)
	}
	idx = page*l.Size + offset
	return l, idx, l.Count*l.Size - idx, nil
}

// PeekPage reads up to n bytes from offset in a page, continuing into the pages that follow when n
// goes past its end, and stopping at the last page.
func (m *SpectrumMemory) PeekPage(page, offset, n int) ([]byte, error) {
	l, idx, room, err := m.pageSpan(page, offset)
	if err != nil {
		return nil, err
	}
	n = min(n, room)
	out := make([]byte, n)
	for i := range out {
		out[i] = m.readPageByte(l, idx+i)
	}
	return out, nil
}

// checkPokePage is PokePage without the write: nil when n bytes at offset in a page can be written.
func (m *SpectrumMemory) checkPokePage(page, offset, n int) error {
	l, _, room, err := m.pageSpan(page, offset)
	if err != nil {
		return err
	}
	if n > room {
		by := fmt.Sprintf("%d bytes", n-room)
		if n-room == 1 {
			by = "1 byte"
		}
		return fmt.Errorf("%d bytes from there would pass the last %s, %d, by %s", n, l.pageWord(), l.Count-1, by)
	}
	return nil
}

// PokePage writes data at offset in a page, continuing into the pages that follow when it goes
// past the page's end. It writes nothing, and says so, when the data would pass the last page.
func (m *SpectrumMemory) PokePage(page, offset int, data []byte) error {
	if err := m.checkPokePage(page, offset, len(data)); err != nil {
		return err
	}
	l, idx, _, _ := m.pageSpan(page, offset)
	for i, b := range data {
		m.writePageByte(l, idx+i, b)
	}
	return nil
}

// pageSpanWords says where a run of n bytes at offset in page ends, for a message: the page it
// begins in, and the one it ends in when that is another.
func (m *SpectrumMemory) pageSpanWords(page, offset, n int) string {
	l := m.PageLayout()
	last := (page*l.Size + offset + max(n, 1) - 1) / l.Size
	if last == page {
		return fmt.Sprintf("%s %d", l.pageWord(), page)
	}
	return fmt.Sprintf("%ss %d to %d", l.pageWord(), page, last)
}

// writableAt reports whether a write at addr lands in RAM with the memory as it is paged now. The
// ROM is not written to, and nor is a Next's slot that has no page in it.
func (m *SpectrumMemory) writableAt(addr uint16) bool {
	if m.bootROMEnabled && addr < 0x4000 {
		return false
	}
	if m.mfPaged && addr < 0x4000 {
		return addr >= 0x2000
	}
	switch {
	case m.isNext:
		slot := addr >> 13
		page := m.nextSlots[slot]
		return !nextSlotDefersToLegacyROM(uint16(slot), page) && !nextPageIsReserved(page)
	case !m.is128K:
		return addr >= ROMSize
	case m.isPlus3 && m.specialPaging:
		return true
	}
	return addr >= ROMSize
}

// firstUnwritable is the first address of the n bytes from addr that a write would not reach, and
// whether there is one. The memory is paged in 8K, so the bytes are looked at at the start and at
// each 8K boundary after it.
func (m *SpectrumMemory) firstUnwritable(addr, n int) (uint16, bool) {
	for a := addr; a < addr+n && a <= 0xFFFF; {
		if !m.writableAt(uint16(a)) {
			return uint16(a), true
		}
		a = (a | 0x1FFF) + 1
	}
	return 0, false
}

// PeekMap reads up to n bytes from addr in the memory as the CPU sees it now.
func (m *SpectrumMemory) PeekMap(addr uint16, n int) []byte {
	n = min(n, 0x10000-int(addr))
	out := make([]byte, max(n, 0))
	for i := range out {
		out[i] = m.Read(addr + uint16(i))
	}
	return out
}

// PokeMap writes data at addr in the memory as the CPU sees it now. It writes nothing, and says
// why, when a byte of it would be in ROM, or past the top.
func (m *SpectrumMemory) PokeMap(addr uint16, data []byte) error {
	if err := m.checkPokeMap(addr, len(data)); err != nil {
		return err
	}
	m.Load(addr, data)
	return nil
}

// checkPokeMap is PokeMap without the write: nil when n bytes at addr can be written.
func (m *SpectrumMemory) checkPokeMap(addr uint16, n int) error {
	if int(addr)+n > 0x10000 {
		return fmt.Errorf("%d bytes at 0x%04X would pass the top of the 64K address space by %d bytes", n, addr, int(addr)+n-0x10000)
	}
	if bad, ok := m.firstUnwritable(int(addr), n); ok {
		return fmt.Errorf("0x%04X is ROM, or no RAM is paged in there, with the memory as it is now: nothing can be written to it (name a RAM page instead)", bad)
	}
	return nil
}

// ParsePage reads a page number: empty, "current", "cpu" or "-1" for no page (the memory as the
// CPU sees it), otherwise a number in decimal, 0x.. or $.. .
func ParsePage(s string) (int, error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "", "current", "cpu", "-1":
		return -1, nil
	}
	v, err := parseNumber(s)
	if err != nil || v > 0xFFFF {
		return 0, fmt.Errorf("%q is not a page number", s)
	}
	return int(v), nil
}

// parseNumber reads a decimal, 0x.. or $.. number.
func parseNumber(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0X"):
		return strconv.ParseUint(s[2:], 16, 32)
	case strings.HasPrefix(s, "$"):
		return strconv.ParseUint(s[1:], 16, 32)
	}
	return strconv.ParseUint(s, 10, 32)
}

// ParseByteList reads bytes to poke: numbers apart by spaces or commas. A number is hex ("3E",
// "0x3E", "$3E") and a decimal one begins with #: "#62".
func ParseByteList(s string) ([]byte, error) {
	var out []byte
	for _, tok := range strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' }) {
		var v uint64
		var err error
		switch {
		case strings.HasPrefix(tok, "#"):
			v, err = strconv.ParseUint(tok[1:], 10, 32)
		case strings.HasPrefix(tok, "0x"), strings.HasPrefix(tok, "0X"), strings.HasPrefix(tok, "$"):
			v, err = parseNumber(tok)
		default:
			v, err = strconv.ParseUint(tok, 16, 32)
		}
		if err != nil || v > 0xFF {
			return nil, fmt.Errorf("%q is not a byte: hex 0 to FF, or #0 to #255", tok)
		}
		out = append(out, byte(v))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("there are no bytes to write")
	}
	return out, nil
}

// LoadBINAt puts the file in memory. With page < 0 it is the memory as the CPU sees it now, at
// the address (as -bin does); with a page it is
// that RAM page, at the address in it, whatever is paged in. startAddr >= 0 sets PC to it, and
// startAddr < 0 leaves PC. It says what it did, in words.
func (zx *ZenZX) LoadBINAt(filename string, page int, addr uint16, startAddr int) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", filename, err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("%s is empty", filename)
	}
	if startAddr > 0xFFFF {
		return "", fmt.Errorf("start address 0x%X is outside the 64K address space", startAddr)
	}
	var where string
	if page < 0 {
		if err := zx.memory.PokeMap(addr, data); err != nil {
			return "", err
		}
		where = fmt.Sprintf("at 0x%04X", addr)
	} else {
		if err := zx.memory.PokePage(page, int(addr), data); err != nil {
			return "", err
		}
		where = fmt.Sprintf("in %s at 0x%04X", zx.memory.pageSpanWords(page, int(addr), len(data)), addr)
	}
	msg := fmt.Sprintf("%d bytes loaded %s", len(data), where)
	if startAddr >= 0 {
		zx.cpu.PC = uint16(startAddr)
		msg += fmt.Sprintf(", and started at 0x%04X", startAddr)
	}
	return msg, nil
}
