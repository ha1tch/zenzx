package main

// ============================================================================
// Turbo mode: fast memory/IO synchronisation, now page-aware
//
// zen80's Z80.FastMem is a flat 64K array -- correct for a machine with no
// banking, but incapable on its own of representing one that pages
// physical memory in and out of the logical address space. Confirmed
// necessary, not speculative: Cybernoid 2 (128K) actively switches RAM and
// ROM banks via port 0x7FFD *during ongoing code execution*, not just
// between tape blocks -- a flat snapshot synced only at block boundaries
// was never going to be enough for this class of game (T-19).
//
// turbo_pages.go models the physical side properly: a fixed pool of
// MemoryPageCount 16K pages (matching SpectrumMemory's own real rom[4]/
// ram[8] layout), each independently addressable. This file is the fast
// path's own use of that pool: turboRemap fires immediately from zen80's
// FastPortWriteOut hook the moment a paging port write actually happens,
// swapping FastMem's affected slot to the newly-selected page there and
// then -- not waiting for a block boundary, which per T-19 is provably
// too late for a loader that pages mid-execution.
// ============================================================================

// turboSnapshotIn copies each of the 12 physical pages' current content
// from real SpectrumMemory into the page pool, establishes the current
// slot mapping from SpectrumMemory's own paging state, populates FastMem
// from that mapping, and activates the fast path. Called once, before a
// Turbo-mode loading run begins.
func (zx *ZenZX) turboSnapshotIn() {
	zx.turboPageFlags = initTurboPageFlags()
	for i := 0; i < 4; i++ {
		zx.turboPages[i] = zx.memory.rom[i]
	}
	for i := 0; i < 8; i++ {
		zx.turboPages[4+i] = zx.memory.ram[i]
	}

	zx.turboPort7FFD = zx.memory.port7FFD

	var mem [65536]byte
	for slot := 0; slot < 4; slot++ {
		page := turboPageIndexFor128K(slot, zx.turboPort7FFD, zx.memory.isPlus3, false)
		zx.turboSlotPage[slot] = page
		copy(mem[slot*16384:(slot+1)*16384], zx.turboPages[page][:])
	}
	zx.cpu.FastMem = &mem

	var port [65536]byte
	port[0x7FFD] = zx.turboPort7FFD
	zx.cpu.FastPort = &port

	// The ULA's port (any even address -- keyboard + tape EAR, combined
	// dynamically via the port's high byte) cannot be correctly modelled
	// as a static FastPort entry: Tape.Tick toggles the EAR level far too
	// often (every pulse) to rewrite FastPort across every matching
	// address each time without erasing the fast path's own benefit. This
	// hook computes the correct value on the much rarer event instead (an
	// actual IN instruction), reading the tape's own live EarLevel plus
	// the real keyboard state -- identical logic to SpectrumIO.In's own
	// ULA-port handling, just evaluated here instead of through the
	// interface. Keyboard state itself is read live too (not snapshotted)
	// since nothing else in this fast-path window can change it.
	zx.cpu.FastPortReadIn = func(port uint16) (uint8, bool) {
		if port&0x01 != 0 {
			return 0, false // not a ULA-decoded address
		}
		result := uint8(0x1F)
		for row := 0; row < 8; row++ {
			if port&(1<<uint(row+8)) == 0 {
				result &= zx.io.keyboard[row]
			}
		}
		if zx.tape != nil && zx.tape.st != nil && zx.tape.st.EarLevel {
			result |= 0x40
		}
		result |= 0x80
		return result, true
	}

	// The actual fix for T-19: fires immediately on every OUT, not just at
	// block boundaries. Only port 0x7FFD's standard decode (port&0xC002==
	// 0x4000) is handled -- +3 special paging (0x1FFD) and TS2068's
	// Extension ROM scheme are out of scope for now (see turbo_pages.go's
	// own doc comment).
	zx.cpu.FastPortWriteOut = func(port uint16, val uint8) {
		if port&0xC002 != 0x4000 {
			return
		}
		if zx.memory.pagingLocked {
			return
		}
		zx.turboPort7FFD = val
		if val&0x20 != 0 {
			zx.memory.pagingLocked = true
		}
		zx.turboRemap()
	}
}

// turboRemap recomputes the desired physical page for each of the four
// logical slots from the fast path's own tracked port7FFD, and for any
// slot whose mapping actually changed: writes the slot's current FastMem
// content back into the page it's about to stop representing (preserving
// whatever the loader just wrote there), then loads the newly-selected
// page's content into that slot.
func (zx *ZenZX) turboRemap() {
	if zx.cpu.FastMem == nil {
		return
	}
	for slot := 0; slot < 4; slot++ {
		newPage := turboPageIndexFor128K(slot, zx.turboPort7FFD, zx.memory.isPlus3, false)
		if newPage == zx.turboSlotPage[slot] {
			continue
		}
		oldPage := zx.turboSlotPage[slot]
		start := slot * 16384
		copy(zx.turboPages[oldPage][:], zx.cpu.FastMem[start:start+16384])
		copy(zx.cpu.FastMem[start:start+16384], zx.turboPages[newPage][:])
		zx.turboSlotPage[slot] = newPage
	}
}

// turboSyncToReal folds FastMem's currently-mapped slots back into the
// page pool, writes every one of the 12 pages back into real
// SpectrumMemory's own rom[]/ram[] storage (not just the four presently
// mapped -- a bank the loader paged out earlier still needs its data to
// land), then applies the tracked paging state via SetPaging (correctly
// updating romBank/ramBankLow/High/Top and, only when it actually
// changed, the screen-bank shadow copy) and force-refreshes the screen
// shadow directly regardless, since SetPaging's own updateScreenBank
// early-returns when the bank is unchanged and would otherwise leave a
// stale shadow after this sync's own bank-4-11 write-back.
func (zx *ZenZX) turboSyncToReal() {
	if zx.cpu.FastMem == nil {
		return
	}
	for slot := 0; slot < 4; slot++ {
		page := zx.turboSlotPage[slot]
		start := slot * 16384
		copy(zx.turboPages[page][:], zx.cpu.FastMem[start:start+16384])
	}
	for i := 0; i < 4; i++ {
		zx.memory.rom[i] = zx.turboPages[i]
	}
	for i := 0; i < 8; i++ {
		zx.memory.ram[i] = zx.turboPages[4+i]
	}

	zx.memory.SetPaging(zx.turboPort7FFD)

	sb := zx.memory.screenBank
	copy(zx.screen.bitmap[:], zx.memory.ram[sb][0:6144])
	copy(zx.screen.attributes[:], zx.memory.ram[sb][6144:6912])
}

// turboSnapshotOut performs a final turboSyncToReal, then deactivates the
// fast path -- zx.cpu.FastMem/FastPort go back to nil, so every subsequent
// RunFrame call uses the normal, real SpectrumMemory/SpectrumIO exactly as
// before Turbo mode ever ran. Called once, when the tape finishes.
func (zx *ZenZX) turboSnapshotOut() {
	zx.turboSyncToReal()
	zx.cpu.FastMem = nil
	zx.cpu.FastPort = nil
	zx.cpu.FastPortReadIn = nil
	zx.cpu.FastPortWriteOut = nil
}

// RunTurboAwareFrame is the single entry point a driver loop should call
// once per frame: it owns the whole Turbo-mode lifecycle (snapshot-in the
// first frame the tape is playing in TapeTurbo mode, periodic real-memory
// sync as Position crosses each BlockBoundaries entry -- now purely for
// screen-shadow/script-visibility freshness, since turboRemap already
// handles paging correctness immediately as it happens -- snapshot-out the
// moment the tape stops), falling through to plain RunFrame in every other
// case (tape not loaded, not in Turbo mode, already finished, or a
// script-level barrier -- wait-boot, wait-screen, wait-attr, wait-mem --
// is currently blocking on real screen/memory content the fast path's own
// unsynced scratch memory has no way to reflect). A caller never needs to
// reason about the lifecycle itself, only call this once per frame,
// passing whether such a barrier is currently active (nil scheduler, or
// no barrier: pass false).
//
// zx.turboNextBoundary is deliberately NOT reset when pausing for a
// barrier -- only turboActive toggles. Block-boundary progress is a
// property of how far the tape has actually loaded, not of whether the
// fast path happens to be active on any given frame; pausing and
// resuming must not replay or skip a sync.
func (zx *ZenZX) RunTurboAwareFrame(blockedOnRealMemory bool) {
	turbo := !blockedOnRealMemory && zx.tape != nil && zx.tape.st != nil &&
		zx.tape.st.Mode == TapeTurbo && zx.tape.st.Playing

	tapeGenuinelyDone := zx.tape == nil || zx.tape.st == nil || !zx.tape.st.Playing

	if !turbo {
		if zx.turboActive {
			zx.turboSnapshotOut()
			zx.turboActive = false
		}
		if tapeGenuinelyDone {
			// Reset only here, not for a temporary barrier pause (where
			// Playing is still true) -- a later resume must continue
			// from wherever block-boundary tracking actually was, not
			// restart it.
			zx.turboNextBoundary = 0
		}
		zx.RunFrame()
		return
	}

	if !zx.turboActive {
		zx.turboSnapshotIn()
		zx.turboActive = true
		if zx.turboNextBoundary == 0 {
			// Start at index 1, not 0: BlockBoundaries[0] marks where the
			// first block's own pulses begin (typically Position 0,
			// matching where playback starts) -- a transition worth
			// syncing at is BlockBoundaries[1] onward, where one block
			// ends and the next begins. Syncing "at" index 0 would just
			// be a wasted, no-op full-memory round trip before any data
			// has even arrived. Only set on the very first activation
			// (turboNextBoundary == 0 is otherwise not a valid index to
			// resume from); a later re-activation after a barrier pause
			// must resume from wherever it actually was.
			zx.turboNextBoundary = 1
		}
	}

	zx.loadingFrame()

	for zx.turboNextBoundary < len(zx.tape.st.BlockBoundaries) &&
		zx.tape.st.Position >= zx.tape.st.BlockBoundaries[zx.turboNextBoundary] {
		zx.turboSyncToReal()
		zx.turboNextBoundary++
	}
}
