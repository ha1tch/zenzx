// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"strings"
)

// The memory the hex editor of Snapshot > Load Binary > Hot Memory can show. It is a list of
// spaces to choose from: the 64K the CPU sees as it is paged now, then each RAM page the model has
// (pagedmem.go: banks 0 to 7 on a 128K, +2, +2A or +3, 8K pages 0 to 223 on a Next, none on a 48K
// or a TS2068). The editor reads the space again every frame, so what the running program does to
// it shows, and it writes a byte the moment it is typed.

// hotSpace is one thing the editor can show.
type hotSpace struct {
	// Page is the RAM page, or -1 for the memory as the CPU sees it.
	Page int
}

// hotCPU is the space of the memory as the CPU sees it.
var hotCPU = hotSpace{Page: -1}

// hotSpaces are the spaces this memory has, in the order they are listed: the CPU's first.
func (m *SpectrumMemory) hotSpaces() []hotSpace {
	out := []hotSpace{hotCPU}
	for p := 0; p < m.PageLayout().Count; p++ {
		out = append(out, hotSpace{Page: p})
	}
	return out
}

// hotSize is how many bytes a space has: what an address in it runs up to.
func (m *SpectrumMemory) hotSize(sp hotSpace) int {
	if sp.Page < 0 {
		return 0x10000
	}
	return m.PageLayout().Size
}

// pageMappedAt lists the addresses a RAM page is paged in at now, for a page that is one.
func (m *SpectrumMemory) pageMappedAt(page int) []uint16 {
	var at []uint16
	switch m.PageLayout().Kind {
	case pagesNext:
		for s := 0; s < len(m.nextSlots); s++ {
			pg := m.nextSlots[s]
			if int(pg) == page && !nextSlotDefersToLegacyROM(uint16(s), pg) && !nextPageIsReserved(pg) {
				at = append(at, uint16(s)<<13)
			}
		}
	case pagesBanks:
		if m.isPlus3 && m.specialPaging {
			// the all-RAM configurations have a RAM bank at 0 too
			if int([4]uint8{0, 4, 4, 4}[m.specialMode&3]) == page {
				at = append(at, 0)
			}
		}
		for i, b := range []uint8{m.ramBankLow, m.ramBankHigh, m.ramBankTop} {
			if int(b) == page {
				at = append(at, uint16(i+1)*0x4000)
			}
		}
	}
	return at
}

// hotName is what a space is called in the list, and says where it is paged in now.
func (m *SpectrumMemory) hotName(sp hotSpace) string {
	if sp.Page < 0 {
		return "CPU address space (64K, as paged in now)"
	}
	l := m.PageLayout()
	s := fmt.Sprintf("%s %d (%dK)", strings.ToUpper(l.pageWord()[:1])+l.pageWord()[1:], sp.Page, l.Size/1024)
	var notes []string
	if at := m.pageMappedAt(sp.Page); len(at) > 0 {
		var a []string
		for _, x := range at {
			a = append(a, fmt.Sprintf("0x%04X", x))
		}
		notes = append(notes, "at "+strings.Join(a, ", "))
	}
	// the bank that is on screen: 5 or 7, the first 6912 bytes of it (a Next's pages 2N and 2N+1)
	if l.Kind == pagesBanks && sp.Page == int(m.screenBank) || l.Kind == pagesNext && sp.Page/2 == int(m.screenBank) {
		notes = append(notes, "screen")
	}
	if len(notes) > 0 {
		s += ": " + strings.Join(notes, ", ")
	}
	return s
}

// hotRead reads up to n bytes of a space from off, as far as the space goes.
func (m *SpectrumMemory) hotRead(sp hotSpace, off, n int) []byte {
	n = min(n, m.hotSize(sp)-off)
	if n <= 0 || off < 0 {
		return nil
	}
	if sp.Page < 0 {
		return m.PeekMap(uint16(off), n)
	}
	b, err := m.PeekPage(sp.Page, off, n)
	if err != nil {
		return nil
	}
	return b
}

// hotWritable says whether a byte of a space can be written: a page always can, and the CPU's
// memory only where there is RAM paged in.
func (m *SpectrumMemory) hotWritable(sp hotSpace, off int) bool {
	if sp.Page >= 0 {
		return true
	}
	return m.writableAt(uint16(off))
}

// hotWrite writes a byte of a space.
func (m *SpectrumMemory) hotWrite(sp hotSpace, off int, v byte) error {
	if off < 0 || off >= m.hotSize(sp) {
		return fmt.Errorf("0x%X is outside the space", off)
	}
	if sp.Page < 0 {
		return m.PokeMap(uint16(off), []byte{v})
	}
	return m.PokePage(sp.Page, off, []byte{v})
}
