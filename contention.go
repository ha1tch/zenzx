package main

// ============================================================================
// ULA Memory and I/O Contention
// ============================================================================
//
// On real 48K/128K hardware, the ULA and the Z80 share the lower 16K of
// RAM's bus. While the ULA is actively reading screen+attribute bytes to
// drive the video signal, it holds the CPU's bus access if the CPU tries
// to touch contended memory or I/O at the same moment -- the electron
// beam can't be interrupted, so the ULA wins. This costs the CPU extra
// T-states, following a documented, deterministic 6,5,4,3,2,1,0,0 pattern
// repeating every 8 T-states, active only during the 128 T-states of each
// visible screen line that the ULA is genuinely fetching pixel data (not
// during border, retrace, or non-visible lines).
//
// Reference: the authoritative timing breakdown (exact starting cycle,
// per-cycle delay table, and I/O contention combination table) comes
// from the comp.sys.sinclair FAQ's 48K technical reference:
// https://worldofspectrum.org/faq/reference/48kreference.htm
//
// Precision note: zen80's ContendedMemDelay/ContendedIODelay hooks
// supply each access's estimated T-state position: the cycle count at
// the start of the current instruction plus a within-instruction
// offset built from per-access base costs (opcode-aware first fetch
// via firstMCycleCost, prefixed second fetch 4, other memory accesses
// 3, I/O 4) and contention delays already applied earlier in the same
// instruction. Measured against real execution (Speedlock loader
// workload, 4.2M instructions), the residual position error -- from
// mid-instruction internal cycles the base costs cannot see -- bounds
// at roughly 0.33% of total memory contention delay and ~0% for I/O
// (IN A,(n)/OUT (n),A have no internal cycles, so their estimated IO
// position, +7, is exact). This supersedes the earlier
// instruction-start-position scheme, whose measured bounds were ~6%
// for memory (flat base costs) and ~20% for I/O (un-offset phase
// error against the 8-cycle pattern).

// contentionPattern48 is the 8-T-state repeating delay pattern, indexed
// by position within the current 8-cycle group. Confirmed directly
// against the FAQ's own cycle-by-cycle table (14335->6, 14336->5, ...,
// 14341->0, 14342->0, 14343->6 [repeat]).
var contentionPattern48 = [8]int{6, 5, 4, 3, 2, 1, 0, 0}

// 48K contention geometry, confirmed against both libspectrum's own
// timing constants and the FAQ's textual description (224 T-states/line,
// 312 lines/frame, 128 T-states of genuinely contended screen-drawing
// per visible line, 192 visible lines, contention beginning at T-state
// 14335 relative to the start of the interrupt).
const (
	contentionStart48  = 14335
	tstatesPerLine48   = 224
	contendedPerLine48 = 128
	contendedLines48   = 192
	ulaFrameLength48   = 69888 // real interrupt-to-interrupt period; zx.cyclesPerFrame now matches this, and positions arrive frame-origin-relative (see setupContention48), so the modulo below is purely defensive against frame-boundary overshoot
)

// contentionDelay48 returns the extra T-states the ULA would hold the CPU
// for if it accesses contended memory or I/O at the given frame-relative
// cycle. frameCycle should already be reduced modulo ulaFrameLength48 (or
// a multiple of it) by the caller; this function reduces it again
// defensively.
func contentionDelay48(frameCycle int) int {
	frameCycle %= ulaFrameLength48
	if frameCycle < 0 {
		frameCycle += ulaFrameLength48
	}
	if frameCycle < contentionStart48 {
		return 0
	}
	rel := frameCycle - contentionStart48
	line := rel / tstatesPerLine48
	if line >= contendedLines48 {
		return 0
	}
	posInLine := rel % tstatesPerLine48
	if posInLine >= contendedPerLine48 {
		return 0
	}
	return contentionPattern48[posInLine%8]
}

// contendedMemDelay48 implements Z80.ContendedMemDelay for 48K: memory
// in 0x4000-0x7FFF is always contended (single fixed 16K bank, no
// paging to account for).
func contendedMemDelay48(addr uint16, framePos uint64) int {
	if addr < 0x4000 || addr >= 0x8000 {
		return 0
	}
	return contentionDelay48(int(framePos))
}

// contendedIODelay48 implements Z80.ContendedIODelay for 48K, following
// the FAQ's own combination table exactly:
//
//	High byte in 0x40-0x7F? | Low bit | Pattern
//	------------------------+---------+-----------------
//	No                      | Reset   | N:1, C:3
//	No                      | Set     | N:4
//	Yes                     | Reset   | C:1, C:1, C:1, C:1
//	Yes                     | Set     | C:1, C:1, C:1, C:1
//
// ("C:n" = the ULA may delay for the pattern value at the current cycle,
// then the Z80 continues for n more T-states; "N:n" = no delay, continue
// for n T-states.) The tape-loading sampling loop's own IN A,(0xFE) --
// high byte 0x7F, low bit reset -- is the "Yes | Reset" case, which is
// why I/O contention dominates during tape loading far more than memory
// contention does (confirmed empirically this session: ~13x larger
// contribution over the same execution window).
//
// Precision note: the "Yes" row's C:1,C:1,C:1,C:1 is applied as four
// CASCADED rounds -- each round applies the pattern delay at its own
// position, then advances one T-state, so a round's delay shifts every
// later round's position -- exactly the FAQ's C:1 semantics. Two
// earlier versions were measurably wrong: a two-check approximation
// undercounted I/O contention by 77%, and four fixed-position checks
// (ft..ft+3, ignoring the shift each real delay causes) still
// mis-phased against the 8-cycle pattern: a 6-cycle delay at phase 0
// puts the next true check at phase 7 (value 0), not phase 1 (value
// 5). Combined with zen80's within-instruction access offset, the
// measured position-error bound for I/O contention is ~0%.
func contendedIODelay48(port uint16, framePos uint64) int {
	highByte := port >> 8
	lowBitSet := port&0x0001 != 0
	inRange := highByte >= 0x40 && highByte <= 0x7F

	if !inRange && lowBitSet {
		return 0 // N:4 -- no ULA involvement at all
	}

	ft := int(framePos)
	if !inRange {
		// N:1, C:3 -- only the second part is contended.
		return contentionDelay48(ft + 1)
	}
	// Yes|Reset and Yes|Set both reduce to C:1,C:1,C:1,C:1, cascaded.
	total := 0
	p := ft
	for k := 0; k < 4; k++ {
		d := contentionDelay48(p)
		total += d
		p += d + 1
	}
	return total
}

// setupContention48 wires the 48K contention model into the CPU. Called
// once ROM loading has confirmed this is genuinely a 48K machine (not
// 128K/+2/+3, which need their own bank-aware model -- see
// setupContention128's doc comment -- and not TS2068, which has no ULA
// contention at all and correctly gets neither hook set).
//
// The closures convert zen80's absolute cycle positions into
// frame-relative ones by subtracting zx.frameOrigin (set by RunFrame at
// each frame's start, coincident with the /INT assertion) -- the same
// single timebase the interrupt and, later, the floating bus use.
// The previous wiring reduced the cumulative counter modulo 69888,
// which precessed against the actual frames.
func (zx *ZenZX) setupContention48() {
	zx.cpu.ContendedMemDelay = func(addr uint16, cyclesBefore uint64) int {
		return contendedMemDelay48(addr, cyclesBefore-zx.frameOrigin)
	}
	zx.cpu.ContendedIODelay = func(port uint16, cyclesBefore uint64) int {
		return contendedIODelay48(port, cyclesBefore-zx.frameOrigin)
	}
}

// setupContention128 wires a 128K/+2 contention approximation into the
// CPU.
//
// KNOWN LIMITATION, deliberately scoped out for now: real 128K
// contention is bank-aware, not address-range-aware -- RAM banks
// 1, 3, 5, and 7 are contended wherever they happen to be paged (any of
// the four 16K slots, not just 0x4000-0x7FFF), and which bank sits where
// changes at runtime via port 0x7FFD. Correctly modeling this needs the
// memory system's current paging state consulted on every access, not
// just the raw address -- not implemented here. This function instead
// reuses the 48K address-range check (0x4000-0x7FFF only) as a
// placeholder that is directionally correct (contention still fires,
// still roughly the right average magnitude for typical bank layouts)
// but will be wrong for programs that page a contended bank outside
// 0x4000-0x7FFF, or an uncontended bank into it.
//
// The timing geometry itself (228 T-states/line, 311 lines/frame,
// contention starting at cycle 14361) is confirmed from the FAQ text
// ("for the 128k/+2 models the contention sequence starts at cycle
// 14361") and the reference timing table (libspectrum's own 128K
// constants), but the FAQ does not give a full 128K cycle-by-cycle
// table the way it does for 48K -- the 6,5,4,3,2,1,0,0 pattern shape is
// assumed to carry over unchanged (same 8-T-state repeat, same 128
// T-states of contended window per line), which is a reasonable but
// unverified extrapolation, not a confirmed fact the way the 48K table
// is.
func (zx *ZenZX) setupContention128() {
	zx.cpu.ContendedMemDelay = func(addr uint16, cyclesBefore uint64) int {
		return contendedMemDelay128(addr, cyclesBefore-zx.frameOrigin)
	}
	zx.cpu.ContendedIODelay = func(port uint16, cyclesBefore uint64) int {
		return contendedIODelay128(port, cyclesBefore-zx.frameOrigin)
	}
}

const (
	contentionStart128  = 14361
	tstatesPerLine128   = 228
	contendedPerLine128 = 128
	contendedLines128   = 192
	ulaFrameLength128   = 70908
)

func contentionDelay128(frameCycle int) int {
	frameCycle %= ulaFrameLength128
	if frameCycle < 0 {
		frameCycle += ulaFrameLength128
	}
	if frameCycle < contentionStart128 {
		return 0
	}
	rel := frameCycle - contentionStart128
	line := rel / tstatesPerLine128
	if line >= contendedLines128 {
		return 0
	}
	posInLine := rel % tstatesPerLine128
	if posInLine >= contendedPerLine128 {
		return 0
	}
	return contentionPattern48[posInLine%8]
}

func contendedMemDelay128(addr uint16, framePos uint64) int {
	// See setupContention128's doc comment: address-range check only,
	// not the real bank-aware model.
	if addr < 0x4000 || addr >= 0x8000 {
		return 0
	}
	return contentionDelay128(int(framePos))
}

// See contendedIODelay48's doc comment: same semantics, same
// reasoning -- four cascaded C:1 rounds for the "Yes" row.
func contendedIODelay128(port uint16, framePos uint64) int {
	highByte := port >> 8
	lowBitSet := port&0x0001 != 0
	inRange := highByte >= 0x40 && highByte <= 0x7F
	if !inRange && lowBitSet {
		return 0
	}
	ft := int(framePos)
	if !inRange {
		return contentionDelay128(ft + 1)
	}
	total := 0
	p := ft
	for k := 0; k < 4; k++ {
		d := contentionDelay128(p)
		total += d
		p += d + 1
	}
	return total
}
