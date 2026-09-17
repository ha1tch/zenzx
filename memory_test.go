package main

import "testing"

// TestNextMMUDisabledByDefault confirms EnableNext() is required before
// the Next-mode addressing path is reachable at all -- a plain
// NewSpectrumMemory (as every existing 48K/128K/+3/TS2068 code path
// constructs one) must behave exactly as it did before T-28 item #19.
func TestNextMMUDisabledByDefault(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	if mem.isNext {
		t.Fatalf("isNext must default false -- a plain NewSpectrumMemory must not enable the Next MMU")
	}
}

// TestNextMMU48KUnaffected is T-28 item #19's own compatibility
// requirement in miniature: a 48K-mode memory (is128K=false, the default)
// reads/writes exactly as before when isNext is left false, regardless of
// whatever nextSlots holds.
func TestNextMMU48KUnaffected(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.ramBankLow = 5
	mem.screenBank = 5

	mem.Write(0x8000, 0xAB) // bank 2, existing 48K RAM path
	if got := mem.Read(0x8000); got != 0xAB {
		t.Fatalf("48K Read/Write at 0x8000 = 0x%02X, want 0xAB (isNext=false must not change 48K behaviour)", got)
	}
}

// TestNextMMU128KUnaffected is the same proof for 128K mode: paging via
// SetPaging (port 0x7FFD) must work exactly as before with isNext=false.
func TestNextMMU128KUnaffected(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.SetPaging(0x03) // page RAM bank 3 into 0xC000-0xFFFF

	mem.Write(0xC100, 0x42)
	if got := mem.Read(0xC100); got != 0x42 {
		t.Fatalf("128K Read/Write at 0xC100 = 0x%02X, want 0x42 (isNext=false must not change 128K behaviour)", got)
	}
	if mem.ramBankTop != 3 {
		t.Fatalf("ramBankTop = %d, want 3 -- SetPaging must be untouched by the Next MMU addition", mem.ramBankTop)
	}
}

// TestNextMMUAliasesClassicBanks proves the load-bearing compatibility
// property: a Next-mode slot mapped to logical page 2*b or 2*b+1 must
// read/write the SAME storage as the classic ramBankLow/High/Top path for
// bank b -- not a copy. This is what makes nextreg.go's mmuBootDefaults
// (banks 10/11=bank5, 4/5=bank2, 0/1=bank0) meaningful once wired (T-28
// item #26): the Next MMU and 128K paging must never disagree about what
// bank 5, for instance, actually contains.
func TestNextMMUAliasesClassicBanks(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()

	// Write via the classic path direct to bank 5's storage.
	mem.ram[5][0x0100] = 0x11 // low 8K half of bank 5 -> page 10
	mem.ram[5][0x2100] = 0x22 // high 8K half of bank 5 -> page 11

	mem.EnableNext()
	mem.nextSlots[0] = 10 // slot 0 (0x0000-0x1FFF) -> page 10 (bank 5 low half)
	mem.nextSlots[1] = 11 // slot 1 (0x2000-0x3FFF) -> page 11 (bank 5 high half)

	if got := mem.Read(0x0100); got != 0x11 {
		t.Fatalf("Next-mode slot 0 (page 10) Read(0x0100) = 0x%02X, want 0x11 (must alias ram[5] low half)", got)
	}
	if got := mem.Read(0x2100); got != 0x22 {
		t.Fatalf("Next-mode slot 1 (page 11) Read(0x2100) = 0x%02X, want 0x22 (must alias ram[5] high half)", got)
	}

	// Write via the Next-mode path, confirm visible through the classic
	// array directly -- proving it's the SAME backing store, not a copy.
	mem.Write(0x0105, 0x99)
	if got := mem.ram[5][0x0105]; got != 0x99 {
		t.Fatalf("ram[5][0x0105] = 0x%02X after Next-mode write, want 0x99 (Next MMU must alias, not copy, classic bank storage)", got)
	}
}

// TestNextMMUExtraStore confirms pages at and beyond nextExtraPageBase
// (16) are backed by the new nextExtraRAM array, independent per page,
// and round-trip correctly across a slot boundary.
func TestNextMMUExtraStore(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()

	mem.nextSlots[2] = nextExtraPageBase     // slot 2: 0x4000-0x5FFF -> first extra page
	mem.nextSlots[3] = nextExtraPageBase + 1 // slot 3: 0x6000-0x7FFF -> second extra page

	mem.Write(0x4000, 0xAA)
	mem.Write(0x6000, 0xBB)

	if got := mem.Read(0x4000); got != 0xAA {
		t.Fatalf("extra-store page %d Read(0x4000) = 0x%02X, want 0xAA", nextExtraPageBase, got)
	}
	if got := mem.Read(0x6000); got != 0xBB {
		t.Fatalf("extra-store page %d Read(0x6000) = 0x%02X, want 0xBB", nextExtraPageBase+1, got)
	}
	// Confirm the two pages are genuinely independent storage, not
	// aliased to each other or to page 0, by reading the backing array
	// directly.
	if mem.nextExtraRAM[0][0] != 0xAA {
		t.Fatalf("nextExtraRAM[0][0] = 0x%02X, want 0xAA", mem.nextExtraRAM[0][0])
	}
	if mem.nextExtraRAM[1][0] != 0xBB {
		t.Fatalf("nextExtraRAM[1][0] = 0x%02X, want 0xBB", mem.nextExtraRAM[1][0])
	}
}

// TestNextMMUOutOfRangePage confirms a reserved page number -- the real
// hardware NR 0x50-0x57 sentinel range 0xE0-0xFF (nextReservedPageBase
// and above) -- reads back 0xFF (floating-bus convention) and silently
// drops writes, rather than panicking or wrapping into valid storage.
// RAM-capacity correction 2026-09-15: nextTotalPages is now 224 (1792K),
// not a magic ceiling itself -- the reserved range is what actually gates
// 0xE0-0xFF, checked explicitly via nextPageIsReserved rather than as a
// side effect of nextTotalPages/array bounds.
//
// Uses slot 2, deliberately NOT slots 0/1: T-28 item #27 gave slots 0/1
// their own, different reserved-range behaviour (defer to the classic
// legacy-ROM-paging machinery instead of floating-bus) -- see
// TestNextReadWriteDefersToLegacyROMSlots0And1 and
// TestNextReadWriteReservedSlots2To7StillFloat below for that split. This
// test is scoped to the plain floating-bus case slots 2-7 still get.
func TestNextMMUOutOfRangePage(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()

	mem.nextSlots[2] = 0xFF // reserved: nextReservedPageBase (0xE0) and above

	if got := mem.Read(0x4000); got != 0xFF {
		t.Fatalf("reserved page 0xFF Read(0x4000) = 0x%02X, want 0xFF (floating-bus convention)", got)
	}

	// Write must not panic and must not corrupt any real storage.
	mem.Write(0x4000, 0x55)
	if got := mem.Read(0x4000); got != 0xFF {
		t.Fatalf("reserved page 0xFF Read(0x4000) after write = 0x%02X, want 0xFF (write must be dropped, not stored)", got)
	}

	mem.nextSlots[7] = nextReservedPageBase // first reserved page (0xE0)
	if got := mem.Read(0xE000); got != 0xFF {
		t.Fatalf("page nextReservedPageBase (0x%02X) Read(0xE000) = 0x%02X, want 0xFF", nextReservedPageBase, got)
	}

	mem.nextSlots[6] = nextTotalPages - 1 // last ORDINARY page (0xDF) must NOT be reserved
	mem.Write(0xC000, 0x77)
	if got := mem.Read(0xC000); got != 0x77 {
		t.Fatalf("last ordinary page (nextTotalPages-1 = 0x%02X) Read(0xC000) = 0x%02X, want 0x77 -- must be real storage, not reserved", nextTotalPages-1, got)
	}
}

// TestNextMMUEightIndependentSlots exercises all 8 slots at once with
// distinct pages, confirming each slot's address window maps to its own
// page independently -- the core claim of an "8 independent 8K MMU
// slots" model, not just a couple of hand-picked cases.
func TestNextMMUEightIndependentSlots(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()

	for slot := 0; slot < 8; slot++ {
		mem.nextSlots[slot] = uint8(nextExtraPageBase + slot)
	}
	for slot := 0; slot < 8; slot++ {
		addr := uint16(slot) * 0x2000
		mem.Write(addr, byte(0x10+slot))
	}
	for slot := 0; slot < 8; slot++ {
		addr := uint16(slot) * 0x2000
		want := byte(0x10 + slot)
		if got := mem.Read(addr); got != want {
			t.Fatalf("slot %d (addr 0x%04X, page %d) Read = 0x%02X, want 0x%02X",
				slot, addr, nextExtraPageBase+slot, got, want)
		}
	}
}

// TestNextSlotDefersToLegacyROMPure exercises the bare predicate function
// directly, independent of Read/Write dispatch: T-28 item #27's special
// case applies only to slots 0/1 holding a reserved-range page, matching
// jnext's Mmu::rebuild_ptr divergence between slots 0/1 and slots 2-7.
func TestNextSlotDefersToLegacyROMPure(t *testing.T) {
	cases := []struct {
		slot uint16
		page uint8
		want bool
	}{
		{0, 0xFF, true},                 // slot 0, reserved sentinel -> defer
		{1, nextReservedPageBase, true}, // slot 1, first reserved page -> defer
		{2, 0xFF, false},                // slot 2, reserved -> NOT deferred (stays floating-bus)
		{7, 0xFF, false},                // slot 7, reserved -> NOT deferred
		{0, nextTotalPages - 1, false},  // slot 0, last ORDINARY page -> not reserved, no defer
		{1, 0, false},                   // slot 1, page 0 -> not reserved, no defer
	}
	for _, c := range cases {
		if got := nextSlotDefersToLegacyROM(c.slot, c.page); got != c.want {
			t.Errorf("nextSlotDefersToLegacyROM(slot=%d, page=0x%02X) = %v, want %v", c.slot, c.page, got, c.want)
		}
	}
}

// TestNextReadWriteDefersToLegacyROMSlots0And1 proves the dispatch-level
// behaviour: with a genuine Next machine, slots 0/1 holding a
// reserved-range page must read from the EXISTING classic rom/romBank
// storage (not a plain 0xFF floating-bus value), and writes to that range
// must be silently dropped rather than stored anywhere.
func TestNextReadWriteDefersToLegacyROMSlots0And1(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.rom[0][0x0000] = 0x11 // slot 0 -> rom offset 0x0000 (low half)
	mem.rom[0][0x1FFF] = 0x22 // slot 0 -> rom offset 0x1FFF (end of low half)
	mem.rom[0][0x2000] = 0x33 // slot 1 -> rom offset 0x2000 (high half start)

	mem.EnableNext()
	mem.nextSlots[0] = 0xFF // reserved sentinel -> defers to legacy ROM
	mem.nextSlots[1] = 0xFF

	if got := mem.Read(0x0000); got != 0x11 {
		t.Fatalf("Read(0x0000) with slot 0 deferring to ROM = 0x%02X, want 0x11 (m.rom[0][0x0000])", got)
	}
	if got := mem.Read(0x1FFF); got != 0x22 {
		t.Fatalf("Read(0x1FFF) with slot 0 deferring to ROM = 0x%02X, want 0x22 (m.rom[0][0x1FFF])", got)
	}
	if got := mem.Read(0x2000); got != 0x33 {
		t.Fatalf("Read(0x2000) with slot 1 deferring to ROM = 0x%02X, want 0x33 (m.rom[0][0x2000])", got)
	}

	// Writes to the deferred range must be dropped, not stored anywhere --
	// including not corrupting the ROM array itself.
	mem.Write(0x0000, 0x99)
	if got := mem.Read(0x0000); got != 0x11 {
		t.Fatalf("Read(0x0000) after Write while deferring to ROM = 0x%02X, want unchanged 0x11 (ROM writes must be dropped)", got)
	}
	if mem.rom[0][0x0000] != 0x11 {
		t.Fatalf("mem.rom[0][0x0000] = 0x%02X after deferred write, want unchanged 0x11", mem.rom[0][0x0000])
	}
}

// TestNextReadWriteReservedSlots2To7StillFloat is the regression proof
// that item #27's slot 0/1 special case did NOT change behaviour for
// slots 2-7: a reserved-range page there must still read 0xFF and drop
// writes via the existing nextPageIsReserved/nextPageRead/nextPageWrite
// path, exactly as TestNextMMUOutOfRangePage already established before
// this change.
func TestNextReadWriteReservedSlots2To7StillFloat(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.rom[0][0x0000] = 0x11 // must NOT leak into a slot-2-7 defer
	mem.EnableNext()
	mem.nextSlots[2] = 0xFF

	if got := mem.Read(0x4000); got != 0xFF {
		t.Fatalf("Read(0x4000) with slot 2 reserved = 0x%02X, want 0xFF (slots 2-7 must not defer to ROM)", got)
	}
	mem.Write(0x4000, 0x55)
	if got := mem.Read(0x4000); got != 0xFF {
		t.Fatalf("Read(0x4000) after write with slot 2 reserved = 0x%02X, want 0xFF still (dropped write)", got)
	}
}

// TestNextReadDefersToLIVERomBankNotCached confirms the ROM-defer path
// tracks m.romBank LIVE (via the existing classic SetPaging/EnablePlus3
// machinery, completely unmodified by T-28) rather than capturing it once
// -- switching romBank after enabling Next mode must change what a
// deferred slot 0/1 read returns.
func TestNextReadDefersToLIVERomBankNotCached(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.rom[0][0x0000] = 0xAA
	mem.rom[1][0x0000] = 0xBB

	mem.EnableNext()
	mem.nextSlots[0] = 0xFF

	if got := mem.Read(0x0000); got != 0xAA {
		t.Fatalf("Read(0x0000) with romBank=0 = 0x%02X, want 0xAA", got)
	}

	mem.SetPaging(0x10) // 128K: bit 4 selects ROM 1
	if mem.romBank != 1 {
		t.Fatalf("romBank after SetPaging(0x10) = %d, want 1 (classic paging unaffected by Next mode)", mem.romBank)
	}
	if got := mem.Read(0x0000); got != 0xBB {
		t.Fatalf("Read(0x0000) with romBank=1 (post SetPaging) = 0x%02X, want 0xBB -- deferred read must track romBank live", got)
	}
}

// TestNextLegacyPagingNoOpUnlessNext confirms SetNextLegacyBank7FFD and
// SetNextLegacyPortDFFD are complete no-ops when isNext is false -- the
// provable-no-op contract their own doc comments claim for every classic
// 48K/128K/+3/TS2068 machine.
func TestNextLegacyPagingNoOpUnlessNext(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	// isNext is false (default) -- these must do nothing at all.
	mem.SetNextLegacyBank7FFD(0x07)
	mem.SetNextLegacyPortDFFD(0x0F)

	wantZeroSlots := [8]uint8{}
	if mem.nextSlots != wantZeroSlots {
		t.Fatalf("nextSlots = %v after legacy-paging calls with isNext=false, want all-zero (must be a no-op)", mem.nextSlots)
	}
	if mem.nextLegacyBank7FFD != 0 || mem.nextLegacyPortDFFD != 0 {
		t.Fatalf("nextLegacyBank7FFD=%d nextLegacyPortDFFD=%d after isNext=false calls, want both 0 (must not even record the value)", mem.nextLegacyBank7FFD, mem.nextLegacyPortDFFD)
	}
}

// TestNextLegacyPagingBankComposition proves recomposeNextLegacySlots's
// bit mapping bit-for-bit against jnext's compose_bank_: bank(2:0) from
// the last 0x7FFD write, bank(4:3)/bank(5)/bank(6) from 0xDFFD bits
// (1:0)/2/3 respectively, and the resulting 16K bank N lands its two 8K
// halves in Next MMU slots 6/7 as page 2N and 2N+1.
func TestNextLegacyPagingBankComposition(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()

	cases := []struct {
		name     string
		bank7FFD uint8
		portDFFD uint8
		wantBank uint8
	}{
		{"bank 0, all zero", 0x00, 0x00, 0},
		{"7FFD bits only (bank 5)", 0x05, 0x00, 5},
		{"7FFD bits only (bank 7, max 3-bit)", 0x07, 0x00, 7},
		{"DFFD bits(1:0) -> bank(4:3)", 0x00, 0x03, 24}, // 3<<3 = 24
		{"DFFD bit 2 -> bank(5)", 0x00, 0x04, 32},       // 1<<5 = 32
		{"DFFD bit 3 -> bank(6)", 0x00, 0x08, 64},       // 1<<6 = 64
		{"all bits combined -> max bank 127", 0x07, 0x0F, 127},
		{"7FFD extra bits (3-7) ignored", 0xF8 | 0x02, 0x00, 2}, // only low 3 bits of 7FFD matter
	}
	for _, c := range cases {
		mem.SetNextLegacyBank7FFD(c.bank7FFD)
		mem.SetNextLegacyPortDFFD(c.portDFFD)

		wantPage6 := c.wantBank * 2
		wantPage7 := wantPage6 + 1
		if got := mem.nextSlots[6]; got != wantPage6 {
			t.Errorf("%s: nextSlots[6] = %d, want %d (bank %d * 2)", c.name, got, wantPage6, c.wantBank)
		}
		if got := mem.nextSlots[7]; got != wantPage7 {
			t.Errorf("%s: nextSlots[7] = %d, want %d (bank %d * 2 + 1)", c.name, got, wantPage7, c.wantBank)
		}
	}
}

// TestNextLegacyPagingActuallyPagesMemory confirms the composed bank
// genuinely drives Read/Write through the Next MMU (via the existing
// SetNextSlot/nextPageRead/nextPageWrite machinery), not merely the
// nextSlots bookkeeping array -- the same "must actually page" bar
// TestNextRegMMUWriteDrivesPaging held item #26 to.
func TestNextLegacyPagingActuallyPagesMemory(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()

	// Bank 5 (7FFD bits=5, DFFD=0) -> page 10/11, both in the extra-store
	// range (>= nextExtraPageBase=16)? No: page 10/11 alias classic bank
	// 5 (ram[5]) since they are below nextExtraPageBase. Use a higher
	// bank instead so this proves the extra-store path specifically.
	// bank 9 -> page 18/19 (extra-store, since nextExtraPageBase=16).
	mem.SetNextLegacyBank7FFD(0x01) // bits(2:0) = 1
	mem.SetNextLegacyPortDFFD(0x01) // bits(4:3) = 1 -> bank(4:3) = 01 -> bank = 1 | 8 = 9

	if mem.nextSlots[6] != 18 || mem.nextSlots[7] != 19 {
		t.Fatalf("nextSlots[6:8] = [%d %d], want [18 19] (bank 9)", mem.nextSlots[6], mem.nextSlots[7])
	}

	mem.Write(0xC000, 0x77) // slot 6 (0xC000-0xDFFF) -> page 18
	if got := mem.Read(0xC000); got != 0x77 {
		t.Fatalf("Read(0xC000) after legacy-paging-driven Write = 0x%02X, want 0x77", got)
	}
	if got := mem.nextExtraRAM[18-nextExtraPageBase][0]; got != 0x77 {
		t.Fatalf("nextExtraRAM[%d][0] = 0x%02X, want 0x77 -- write must land in the composed page's real backing store", 18-nextExtraPageBase, got)
	}
}

// TestNextLegacyPagingOverflowIsReservedNoExtraBoundsCheck proves the
// doc comment's claim on recomposeNextLegacySlots: a composed bank at or
// beyond the honest 224-page ceiling (bank 112+, since 112*2=224=
// nextReservedPageBase) needs no dedicated bounds check -- the existing
// nextPageIsReserved gate in nextPageRead/nextPageWrite already gives
// correct floating-bus/dropped-write behaviour automatically.
func TestNextLegacyPagingOverflowIsReservedNoExtraBoundsCheck(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.EnableNext()

	// Max composable bank: 7FFD bits(2:0)=7, DFFD bits(3:0)=0xF -> bank 127.
	mem.SetNextLegacyBank7FFD(0x07)
	mem.SetNextLegacyPortDFFD(0x0F)

	if mem.nextSlots[6] != 254 || mem.nextSlots[7] != 255 {
		t.Fatalf("nextSlots[6:8] = [%d %d], want [254 255] (max composed bank 127)", mem.nextSlots[6], mem.nextSlots[7])
	}

	if got := mem.Read(0xC000); got != 0xFF {
		t.Fatalf("Read(0xC000) with overflowed composed bank = 0x%02X, want 0xFF (floating bus, via existing nextPageIsReserved gate)", got)
	}
	mem.Write(0xC000, 0x42)
	if got := mem.Read(0xC000); got != 0xFF {
		t.Fatalf("Read(0xC000) after write with overflowed composed bank = 0x%02X, want 0xFF still (dropped write)", got)
	}
}

// TestNextLegacyPagingDoesNotTouchClassicFields is the "build on the side"
// regression proof: driving the new legacy-paging setters must leave
// EVERY classic paging field exactly as it was -- port7FFD, ramBankTop,
// romBank, pagingLocked, and the ram array itself -- because these
// setters have deliberately separate storage (nextLegacyBank7FFD/
// nextLegacyPortDFFD) and never call SetPaging/SetPlus3Paging.
func TestNextLegacyPagingDoesNotTouchClassicFields(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.ram[3][0] = 0xEE // sentinel to prove no accidental aliasing/corruption
	mem.EnableNext()

	mem.SetNextLegacyBank7FFD(0x07)
	mem.SetNextLegacyPortDFFD(0x0F)

	if mem.port7FFD != 0 {
		t.Fatalf("port7FFD = 0x%02X after legacy-paging calls, want 0 (unchanged -- these never call SetPaging)", mem.port7FFD)
	}
	if mem.ramBankTop != 0 {
		t.Fatalf("ramBankTop = %d after legacy-paging calls, want 0 (unchanged)", mem.ramBankTop)
	}
	if mem.romBank != 0 {
		t.Fatalf("romBank = %d after legacy-paging calls, want 0 (unchanged)", mem.romBank)
	}
	if mem.pagingLocked {
		t.Fatalf("pagingLocked = true after legacy-paging calls, want false (unchanged)")
	}
	if mem.ram[3][0] != 0xEE {
		t.Fatalf("ram[3][0] = 0x%02X after legacy-paging calls, want unchanged 0xEE", mem.ram[3][0])
	}
}
