package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// every model that ZenZX runs, and how its memory is paged.
var pagingByModel = []struct {
	model string
	kind  pageKind
	size  int
	count int
}{
	{"48k", pagesNone, 0, 0},
	{"spanish48k", pagesNone, 0, 0},
	{"ts2068", pagesNone, 0, 0},
	{"128k", pagesBanks, 0x4000, 8},
	{"spanish128k", pagesBanks, 0x4000, 8},
	{"plus2", pagesBanks, 0x4000, 8},
	{"spanishplus2", pagesBanks, 0x4000, 8},
	{"plus2a", pagesBanks, 0x4000, 8},
	{"plus3", pagesBanks, 0x4000, 8},
	{"spanishplus3", pagesBanks, 0x4000, 8},
	{"next", pagesNext, 0x2000, 224},
}

func machineOf(t *testing.T, model string) *ZenZX {
	t.Helper()
	zx, err := newMachine(model, "./rom")
	if err != nil {
		t.Fatalf("%s: %v", model, err)
	}
	return zx
}

// pagedIn makes the CPU see page (a bank on a 128K, a page on a Next) at the top of memory, 0xC000
// or 0xE000, and says where.
func pagedIn(zx *ZenZX, page int) (base uint16) {
	m := zx.memory
	if m.isNext {
		m.SetNextSlot(7, uint8(page))
		return 0xE000
	}
	m.SetPaging(uint8(page) & 7)
	return 0xC000
}

func TestPageLayoutOfEveryModel(t *testing.T) {
	for _, c := range pagingByModel {
		l := machineOf(t, c.model).memory.PageLayout()
		if l.Kind != c.kind || l.Size != c.size || l.Count != c.count {
			t.Errorf("%s: %+v, want kind %d, %d pages of %d", c.model, l, c.kind, c.count, c.size)
		}
	}
}

// A byte poked into a page is the byte the CPU reads when that page is paged in, for every page of
// every model that has them; and nothing else moved.
func TestPokePageIsWhatThePagedInMemoryReads(t *testing.T) {
	for _, c := range pagingByModel {
		if c.kind == pagesNone {
			continue
		}
		zx := machineOf(t, c.model)
		pages := make([]int, 0, c.count)
		if c.kind == pagesBanks {
			for p := 0; p < c.count; p++ {
				pages = append(pages, p)
			}
		} else {
			pages = []int{0, 1, 2, 7, 10, 11, 14, 15, 16, 17, 100, 222, 223}
		}
		for _, p := range pages {
			data := []byte{byte(p), 0xA5, byte(p) ^ 0xFF}
			// at a place that is not the screen's: past 6912
			off := 0x1D00
			if err := zx.memory.PokePage(p, off, data); err != nil {
				t.Fatalf("%s page %d: %v", c.model, p, err)
			}
			base := pagedIn(zx, p)
			for i, want := range data {
				if got := zx.memory.Read(base + uint16(off+i)); got != want {
					t.Errorf("%s page %d: the CPU reads 0x%02X at 0x%04X, want 0x%02X", c.model, p, got, base+uint16(off+i), want)
				}
			}
			got, err := zx.memory.PeekPage(p, off, 3)
			if err != nil || !bytes.Equal(got, data) {
				t.Errorf("%s page %d: peeked %v (%v), want %v", c.model, p, got, err, data)
			}
		}
		// the pages poked first were not disturbed by those after
		for _, p := range pages {
			got, _ := zx.memory.PeekPage(p, 0x1D00, 1)
			if got[0] != byte(p) {
				t.Errorf("%s page %d holds 0x%02X: another page's poke reached it", c.model, p, got[0])
			}
		}
	}
}

// A Next's page 2N and 2N+1 are the 128K's bank N.
func TestNextPagesAreTheBanksHalves(t *testing.T) {
	zx := machineOf(t, "next")
	if err := zx.memory.PokePage(6, 0x0010, []byte{0x11}); err != nil { // bank 3 low half
		t.Fatal(err)
	}
	if err := zx.memory.PokePage(7, 0x0010, []byte{0x22}); err != nil { // bank 3 high half
		t.Fatal(err)
	}
	if zx.memory.ram[3][0x10] != 0x11 || zx.memory.ram[3][0x2010] != 0x22 {
		t.Errorf("bank 3 holds 0x%02X and 0x%02X", zx.memory.ram[3][0x10], zx.memory.ram[3][0x2010])
	}
}

// The picture the renderer draws is a copy of the displayed bank; a poke into that bank reaches it,
// a poke into the other one does not, and a peek of the displayed bank reads what is on show.
func TestPokePageKeepsTheScreenCopy(t *testing.T) {
	for _, model := range []string{"128k", "plus2", "plus3", "next"} {
		zx := machineOf(t, model)
		m := zx.memory
		l := m.PageLayout()
		bank := func(b int) int { // the page that holds a bank's first byte
			if l.Kind == pagesNext {
				return b * 2
			}
			return b
		}
		pixel := func(bn int, v byte) { // pixel byte 7 and attribute byte 3 of bank bn
			if err := m.PokePage(bank(bn), 7, []byte{v}); err != nil {
				t.Fatal(err)
			}
			if err := m.PokePage(bank(bn), 6144+3, []byte{v ^ 0xFF}); err != nil {
				t.Fatal(err)
			}
		}
		// normal screen, in bank 5
		m.SetPaging(0x00)
		pixel(5, 0x11)
		if zx.screen.bitmap[7] != 0x11 || zx.screen.attributes[3] != 0xEE {
			t.Errorf("%s: the screen shows 0x%02X, 0x%02X after a poke into bank 5", model, zx.screen.bitmap[7], zx.screen.attributes[3])
		}
		pixel(7, 0x22) // the shadow screen: not on show
		if zx.screen.bitmap[7] != 0x11 || zx.screen.attributes[3] != 0xEE {
			t.Errorf("%s: a poke into bank 7 reached the screen on show: 0x%02X, 0x%02X", model, zx.screen.bitmap[7], zx.screen.attributes[3])
		}
		// the shadow screen on show: it is bank 7's now, and bank 5's pokes stay off it
		m.SetPaging(0x08)
		if zx.screen.bitmap[7] != 0x22 {
			t.Fatalf("%s: showing bank 7, the screen has 0x%02X, want 0x22", model, zx.screen.bitmap[7])
		}
		pixel(5, 0x33)
		if zx.screen.bitmap[7] != 0x22 {
			t.Errorf("%s: a poke into bank 5 reached the shadow screen on show", model)
		}
		pixel(7, 0x44)
		if zx.screen.bitmap[7] != 0x44 || zx.screen.attributes[3] != 0xBB {
			t.Errorf("%s: the shadow screen shows 0x%02X, 0x%02X after a poke into bank 7", model, zx.screen.bitmap[7], zx.screen.attributes[3])
		}
		if got, _ := m.PeekPage(bank(7), 7, 1); got[0] != 0x44 {
			t.Errorf("%s: peeking bank 7 gives 0x%02X, want 0x44", model, got[0])
		}
		// a peek of the displayed bank is what is on show, whoever wrote it
		zx.screen.bitmap[9] = 0x5A
		if got, _ := m.PeekPage(bank(7), 9, 1); l.Kind == pagesBanks && got[0] != 0x5A {
			t.Errorf("%s: peeking the displayed bank gives 0x%02X, want what is on show, 0x5A", model, got[0])
		}
	}
}

// A run that goes past the end of a page goes on into the next; one that would go past the last
// page is refused and writes nothing.
func TestPokePageContinuesAndStopsAtTheLastPage(t *testing.T) {
	for _, c := range pagingByModel {
		if c.kind == pagesNone {
			continue
		}
		zx := machineOf(t, c.model)
		m := zx.memory
		page := 3
		if c.kind == pagesNext {
			page = 40
		}
		data := []byte{1, 2, 3, 4, 5, 6}
		if err := m.PokePage(page, c.size-2, data); err != nil {
			t.Fatalf("%s: %v", c.model, err)
		}
		tail, _ := m.PeekPage(page, c.size-2, 2)
		head, _ := m.PeekPage(page+1, 0, 4)
		if !bytes.Equal(tail, data[:2]) || !bytes.Equal(head, data[2:]) {
			t.Errorf("%s: the page ends with %v and the next begins with %v", c.model, tail, head)
		}
		// the end of the pages
		last := c.count - 1
		before, _ := m.PeekPage(last, c.size-3, 3)
		err := m.PokePage(last, c.size-3, []byte{9, 9, 9, 9})
		if err == nil || !strings.Contains(err.Error(), "pass the last") {
			t.Errorf("%s: a run past the last page: %v", c.model, err)
		}
		after, _ := m.PeekPage(last, c.size-3, 3)
		if !bytes.Equal(before, after) {
			t.Errorf("%s: a refused run changed the last page: %v, was %v", c.model, after, before)
		}
		if err := m.PokePage(last, c.size-3, []byte{7, 8, 9}); err != nil {
			t.Errorf("%s: a run that ends at the last byte: %v", c.model, err)
		}
		if got, _ := m.PeekPage(last, c.size-3, 10); len(got) != 3 {
			t.Errorf("%s: a peek past the end gave %d bytes, want the 3 there are", c.model, len(got))
		}
	}
}

// Bad pages and offsets are refused with a reason; a model with no paging has no pages.
func TestPageErrors(t *testing.T) {
	for _, c := range pagingByModel {
		m := machineOf(t, c.model).memory
		if c.kind == pagesNone {
			for _, f := range []func() error{
				func() error { return m.PokePage(0, 0, []byte{1}) },
				func() error { _, err := m.PeekPage(0, 0, 1); return err },
			} {
				if err := f(); err == nil || !strings.Contains(err.Error(), "no paged memory") {
					t.Errorf("%s: %v", c.model, err)
				}
			}
			continue
		}
		for name, f := range map[string]func() error{
			"a page past the last": func() error { return m.PokePage(c.count, 0, []byte{1}) },
			"a negative page":      func() error { return m.PokePage(-1, 0, []byte{1}) },
			"an offset past size":  func() error { return m.PokePage(0, c.size, []byte{1}) },
			"a negative offset":    func() error { return m.PokePage(0, -1, []byte{1}) },
			"a peek past the last": func() error { _, err := m.PeekPage(c.count, 0, 1); return err },
		} {
			if err := f(); err == nil {
				t.Errorf("%s: %s is accepted", c.model, name)
			}
		}
	}
}

// With the memory as the CPU sees it, ROM is refused whole, never half written, on every model;
// RAM is written where the CPU would write it.
func TestPokeMapRefusesROMOnEveryModel(t *testing.T) {
	for _, c := range pagingByModel {
		zx := machineOf(t, c.model)
		m := zx.memory
		for _, addr := range []uint16{0x0000, 0x2000, 0x3FFF} {
			if err := m.PokeMap(addr, []byte{0xAA}); err == nil || !strings.Contains(err.Error(), "ROM") {
				t.Errorf("%s: a poke at 0x%04X: %v", c.model, addr, err)
			}
		}
		rom := m.PeekMap(0x3FFE, 2)
		if err := m.PokeMap(0x3FFE, []byte{0xAA, 0xBB, 0xCC, 0xDD}); err == nil {
			t.Errorf("%s: a run from ROM into RAM is accepted", c.model)
		}
		if got := m.PeekMap(0x3FFE, 4); !bytes.Equal(got[:2], rom) || got[2] == 0xCC {
			t.Errorf("%s: a refused run wrote something: % X", c.model, got)
		}
		if err := m.PokeMap(0x8000, []byte{0xAA, 0xBB}); err != nil {
			t.Errorf("%s: a poke at 0x8000: %v", c.model, err)
		}
		if got := m.PeekMap(0x8000, 2); got[0] != 0xAA || got[1] != 0xBB {
			t.Errorf("%s: 0x8000 holds % X", c.model, got)
		}
		if err := m.PokeMap(0xFFFF, []byte{1, 2}); err == nil || !strings.Contains(err.Error(), "top") {
			t.Errorf("%s: a run past the top: %v", c.model, err)
		}
	}
}

// The memory as the CPU sees it follows the paging: a poke at the top of memory lands in whatever
// bank is paged in there, which is the difference a page makes.
func TestPokeMapFollowsThePaging(t *testing.T) {
	for _, model := range []string{"128k", "plus2", "plus3"} {
		zx := machineOf(t, model)
		m := zx.memory
		m.SetPaging(3)
		if err := m.PokeMap(0xC000, []byte{0x33}); err != nil {
			t.Fatal(err)
		}
		m.SetPaging(6)
		if err := m.PokeMap(0xC000, []byte{0x66}); err != nil {
			t.Fatal(err)
		}
		a, _ := m.PeekPage(3, 0, 1)
		b, _ := m.PeekPage(6, 0, 1)
		if a[0] != 0x33 || b[0] != 0x66 {
			t.Errorf("%s: bank 3 holds 0x%02X and bank 6 0x%02X, want 0x33 and 0x66", model, a[0], b[0])
		}
	}
	// a +3's special paging puts RAM at 0: the ROM is not there to refuse
	zx := machineOf(t, "plus3")
	zx.memory.SetPlus3Paging(0x01)
	if err := zx.memory.PokeMap(0x0000, []byte{0x77}); err != nil {
		t.Errorf("a +3 with RAM at 0: %v", err)
	}
	if got := zx.memory.Read(0); got != 0x77 {
		t.Errorf("a +3 with RAM at 0 reads 0x%02X", got)
	}
	// a Next: a slot with no page is ROM, and with a page it is RAM
	nx := machineOf(t, "next")
	if err := nx.memory.PokeMap(0x0000, []byte{1}); err == nil {
		t.Error("a Next's ROM slot takes a poke")
	}
	nx.memory.SetNextSlot(0, 40)
	if err := nx.memory.PokeMap(0x0000, []byte{0x88}); err != nil {
		t.Errorf("a Next with RAM at 0: %v", err)
	}
	if got, _ := nx.memory.PeekPage(40, 0, 1); got[0] != 0x88 {
		t.Errorf("page 40 holds 0x%02X", got[0])
	}
	// the slot beside it is still ROM: a run across the boundary is refused
	if err := nx.memory.PokeMap(0x1FFE, []byte{1, 2, 3, 4}); err == nil {
		t.Error("a run from RAM into a Next's ROM slot is accepted")
	}
}

// LoadBINAt, as the dialogue and -binpage call it: a file into the memory as it is, or into a page;
// PC when it is to start; and what it says.
func TestLoadBINAtEveryModel(t *testing.T) {
	file := filepath.Join(t.TempDir(), "code.bin")
	code := []byte{0x3E, 0x07, 0xD3, 0xFE, 0xC9}
	if err := os.WriteFile(file, code, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range pagingByModel {
		zx := machineOf(t, c.model)
		msg, err := zx.LoadBINAt(file, -1, 0x8000, 0x8000)
		if err != nil || !strings.Contains(msg, "5 bytes loaded at 0x8000") || !strings.Contains(msg, "started at 0x8000") {
			t.Errorf("%s: %q, %v", c.model, msg, err)
		}
		if zx.cpu.PC != 0x8000 || !bytes.Equal(zx.memory.PeekMap(0x8000, 5), code) {
			t.Errorf("%s: PC 0x%04X, memory % X", c.model, zx.cpu.PC, zx.memory.PeekMap(0x8000, 5))
		}
		// no start: PC stays
		zx.cpu.PC = 0x1234
		if _, err := zx.LoadBINAt(file, -1, 0x9000, -1); err != nil || zx.cpu.PC != 0x1234 {
			t.Errorf("%s: PC 0x%04X after a load that does not start (%v)", c.model, zx.cpu.PC, err)
		}
		// into ROM: refused, and PC stays
		if _, err := zx.LoadBINAt(file, -1, 0x0000, 0x0000); err == nil || zx.cpu.PC != 0x1234 {
			t.Errorf("%s: a load into ROM: %v, PC 0x%04X", c.model, err, zx.cpu.PC)
		}
		if _, err := zx.LoadBINAt(file, -1, 0x8000, 0x10000); err == nil {
			t.Errorf("%s: a start past 64K is accepted", c.model)
		}
		if _, err := zx.LoadBINAt(filepath.Join(t.TempDir(), "gone.bin"), -1, 0x8000, -1); err == nil {
			t.Errorf("%s: a file that is not there is accepted", c.model)
		}
		// a page
		if c.kind == pagesNone {
			if _, err := zx.LoadBINAt(file, 1, 0, -1); err == nil || !strings.Contains(err.Error(), "no paged memory") {
				t.Errorf("%s: a page on a model with none: %v", c.model, err)
			}
			continue
		}
		page := 4
		word := "bank 4"
		if c.kind == pagesNext {
			page, word = 33, "page 33"
		}
		msg, err = zx.LoadBINAt(file, page, 0x0100, -1)
		if err != nil || !strings.Contains(msg, "in "+word+" at 0x0100") {
			t.Errorf("%s: %q, %v", c.model, msg, err)
		}
		if got, _ := zx.memory.PeekPage(page, 0x0100, 5); !bytes.Equal(got, code) {
			t.Errorf("%s: %s holds % X", c.model, word, got)
		}
		if c.kind == pagesNext {
			msg, _ = zx.LoadBINAt(file, 33, uint16(c.size-2), -1)
			if !strings.Contains(msg, "pages 33 to 34") {
				t.Errorf("%s: a run over a page's end said %q", c.model, msg)
			}
		}
	}
}

func TestParsePage(t *testing.T) {
	for s, want := range map[string]int{"": -1, " ": -1, "current": -1, "CPU": -1, "-1": -1, "0": 0, "7": 7, "0x1F": 31, "$10": 16, "223": 223} {
		if got, err := ParsePage(s); err != nil || got != want {
			t.Errorf("ParsePage(%q) = %d, %v; want %d", s, got, err, want)
		}
	}
	for _, s := range []string{"bank", "-2", "0x", "1.5", "99999999999"} {
		if _, err := ParsePage(s); err == nil {
			t.Errorf("ParsePage(%q) is accepted", s)
		}
	}
}

func TestParseByteList(t *testing.T) {
	got, err := ParseByteList("3E 01, C9\t0xFF $0a #62 7")
	if err != nil || !bytes.Equal(got, []byte{0x3E, 0x01, 0xC9, 0xFF, 0x0A, 62, 7}) {
		t.Errorf("% X, %v", got, err)
	}
	for _, s := range []string{"", "  ", "100", "GG", "#256", "0x1FF", "3E,,xx"} {
		if _, err := ParseByteList(s); err == nil {
			t.Errorf("ParseByteList(%q) is accepted", s)
		}
	}
}
