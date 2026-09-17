package main

// ============================================================================
// ZX Spectrum Next: NextREG port handler (T-27, wave ti0.r1)
//
// Two fully-decoded I/O ports drive every Next-only register:
//
//	0x243B  select   write: choose which register subsequent 0x253B
//	                  accesses; read: returns the currently selected
//	                  register number (standard NextREG protocol
//	                  behaviour -- not separately documented as
//	                  unsupported by any source consulted, and this is
//	                  the behaviour every existing Next emulator/CSpect
//	                  implements; flagged here as the one point in this
//	                  file inferred from common practice rather than
//	                  read directly off a register-by-register table).
//	0x253B  data      read/write the currently selected register,
//	                  subject to that register's own access rules
//	                  below.
//
// Unlike the ULA's partial port decode (io.go matches on a handful of
// address bits and lets aliases fall through), Next ports are fully
// 16-bit decoded on real hardware, so both ports here are matched
// exactly rather than via a bitmask -- confirmed against the SpecNext
// wiki's I/O port documentation (specnext.com/tbblue-io-port-system,
// the tbblue project's own registers.txt) and cross-checked against
// explorer.specnext.dev's register summary; both sources agree on
// which of the registers below are read-only, write-only, or R/W.
//
// zen80's NEXTREG instruction (ED 91/92, z80n.go) already implements
// itself as exactly the two OUT calls this file's WritePort path
// expects (z.ioOut(0x243B, reg) then z.ioOut(0x253B, value)) -- so no
// CPU-side change is needed; z80nNextreg_test.go below is the
// regression confirming that prediction rather than assuming it.
//
// Register coverage here is deliberately the minimal real set needed
// to make wave ti0.r1 "done" per NEXT_SUPPORT_DEVELOPMENT_PLAN.md:
// machine identification (0x00/0x01), reset (0x02), machine type
// (0x03), CPU speed (0x07), two peripheral-configuration registers
// (0x08/0x09), the sprite/layer priority register (0x15), the Layer 2
// clip window (0x18), and the eight MMU slot registers (0x50-0x57).
// The MMU slots are stored and read back correctly here but not yet
// wired to actual paging -- that wiring is ti0.r2 (T-28); this file
// only has to prove the register file itself is correct. Registers
// outside this set read back as 0x00 and accept writes silently
// (store-and-ignore), which is distinguishable from "not implemented
// erroring" and matches how a real, growing register file behaves as
// coverage is added incrementally wave by wave.
// ============================================================================

const (
	nextRegSelectPort = 0x243B
	nextRegDataPort   = 0x253B
)

// nextRegAccess records whether a given register number accepts reads,
// writes, or both -- most Next registers are R/W, but a handful are
// read-only (identification) or have asymmetric behaviour.
type nextRegAccess uint8

const (
	nextRegRW nextRegAccess = iota
	nextRegRO
	nextRegWO
)

// NextRegisterFile is the ZX Spectrum Next's register-bus state:
// which register is currently selected, and the stored value of every
// register this emulator implements.
type NextRegisterFile struct {
	selected uint8
	regs     [256]uint8
	access   [256]nextRegAccess

	// mmuSlots mirrors regs[0x50:0x58] for callers that want the MMU
	// page-select values directly without recomputing the offset --
	// ti0.r2 (T-28) reads this once real paging lands. Kept in sync by
	// writeSelected; never written independently.
	mmuSlots [8]uint8
}

// nextRegDefault pairs a register number with its power-on reset value
// and access mode, matching the SpecNext wiki's per-register defaults.
type nextRegDefault struct {
	reg     uint8
	value   uint8
	access  nextRegAccess
	comment string
}

// nextRegDefaults is the full set of registers this file implements.
// Values and access modes are taken from specnext.com's I/O port
// documentation and cross-checked against explorer.specnext.dev; see
// the file-level comment above for the one inferred (not directly
// documented) behaviour, register-select-port readback.
var nextRegDefaults = []nextRegDefault{
	{0x00, 0x0A, nextRegRO, "machine ID -- 0x0A is the VHDL g_machine_id board-generic value (zxnext_top_issue{2,4,5}.vhd); confirmed against jnext (github.com/jorgegv/jnext), whose own history records an 0x08 default breaking NextZXOS boot (wrong ROM1 machine-ID branch, fixed 2026-07-09) -- 0x00 has no basis in any real hardware or firmware and was this file's own pre-T-34 placeholder"},
	{0x01, 0x00, nextRegRO, "core version: bits 7-4 major, bits 3-0 minor (sub-minor is a separate register, not implemented here)"},
	{0x02, 0x00, nextRegRW, "reset: bit1 hard reset, bit0 soft reset (write-triggered; read reports last power-on-reset cause, not modelled beyond 0x00 here)"},
	{0x03, 0x00, nextRegRW, "machine type / timing select; real hardware restricts bits 2-0 to config mode only -- not modelled, this emulator has no separate config-mode gate"},
	{0x07, 0x00, nextRegRW, "CPU speed: bits 1-0, 00=3.5MHz 01=7MHz 10=14MHz (11 reserved); stored only -- ti0.r1 does not yet change actual emulated CPU speed"},
	{0x08, 0x00, nextRegRW, "peripheral setting 3: 128K paging/contention/stereo/speaker/Specdrum/Timex/TurboSound bits; stored only, no behavioural wiring yet"},
	{0x09, 0x00, nextRegRW, "peripheral setting 4: AY mono bits, sprite lockstep, Kempston/divMMC disable, scanline mode; stored only"},
	{0x15, 0x08, nextRegRW, "sprite and layer system: priority/clip/visibility bits, bits 4-2; stored default here is 0x08, which decodes to \"S U L\" (sprites over ULA over Layer 2) per the wiki's own bits-4:2 table -- NOT \"S L U\" as this line's comment previously and incorrectly claimed (confirmed by direct bit arithmetic: 0x08 = 00001000, bits[4:2] = 010 = S U L, not 000 = S L U). The wiki documents the real hardware reset default for the WHOLE register as 0x00 (= S L U), so this stored default is itself questionable and worth reconciling against real hardware separately -- flagged in docs/TRACKING.md T-29, not changed here since it is unrelated, already-shipped T-27 behaviour"},
	{0x18, 0x00, nextRegRW, "Layer 2 clip window: sequential writes set X1,X2,Y1,Y2 in turn (see nextRegClip below); reads return the byte at the current index without advancing it"},
	{0x12, 0x00, nextRegRW, "Layer 2 visible screen base bank: 8K-page-pair index into Next extended RAM where the active 256x192x8bpp framebuffer starts (T-29, wave ti0.r3); reset default 0x00 per the wiki (no stated non-zero default), stored only until DecodeLayer2 consumes it"},
	{0x13, 0x00, nextRegRW, "Layer 2 shadow screen base bank: same encoding as 0x12, selects the alternate buffer once IO port 0x123B's shadow bit is implemented; that port is out of scope for T-29 (see nextreg.go file header) so this register is stored-and-ignored for now, exactly like every other not-yet-wired register"},
	{0x70, 0x00, nextRegRW, "Layer 2 control: bits 5-4 resolution (00=256x192x8bpp, 01=320x256x8bpp, 10=640x256x4bpp, 11=reserved/treated as 00; soft-reset default 00), bits 3-0 palette offset added to every pixel value before palette lookup (soft-reset default 0); all three real modes implemented (layer2.go's layer2CurrentMode/layer2Pixel) as of the T-29 completion pass -- see layer2.go's own header for the wider modes' column-major addressing and the 4bpp mode's inferred, unconfirmed palette-offset treatment"},
	{0x34, 0x00, nextRegRW, "sprite number select (T-30, wave ti0.r4): selects which of the 128 sprites (0-127) subsequent writes to 0x35-0x38 address; this emulator implements only the NextREG-mirror access path the wiki also documents (0x34 select + 0x35-0x38 attribute registers), not the port-0x303B/0x57/0x5B path -- NEXT_SUPPORT_DEVELOPMENT_PLAN.md requires only that sprites be reachable via ti0.r1's NextREG mechanism, not a specific one of the two real hardware paths"},
	{0x35, 0x00, nextRegRW, "sprite attribute 0 for the currently selected sprite (0x34): X position bits 7-0, anchor (low byte; bit 8 lives in attribute 2, register 0x37) -- or a signed 8-bit X offset from the anchor's position, if this sprite is a relative (NR 0x39's discriminator bits)"},
	{0x36, 0x00, nextRegRW, "sprite attribute 1 for the currently selected sprite (0x34): Y position bits 7-0, anchor (low byte; bit 8 lives in attribute 4, register 0x39 bit 0 -- see that register's own entry) -- or a signed 8-bit Y offset from the anchor's position, if this sprite is a relative"},
	{0x37, 0x00, nextRegRW, "sprite attribute 2 for the currently selected sprite (0x34): bits 7-4 palette offset (added to the pixel's raw index before palette lookup, same convention as Layer 2's NR 0x70 -- see resolveNextPixelIndex); bit 3 X mirror, bit 2 Y mirror, bit 1 rotate (90 degrees clockwise, applied before mirroring); bit 0 X position MSB for an anchor (9th bit, combined with 0x35 for 0-511) / 'PR' enable-relative-palette-offset for a relative sprite (adds the anchor's own palette offset to this sprite's, per NR 0x39's discriminator) -- see sprite.go's file header for the full cross-checked bit table, added in the T-29/T-30 completion pass"},
	{0x38, 0x00, nextRegRW, "sprite attribute 3 for the currently selected sprite (0x34): bit 7 visibility (1 = displayed), bit 6 'E' -- enables attribute 4 (NR 0x39) for this sprite; when clear, the sprite is always its own unscaled/unrotated/non-relative anchor regardless of what 0x39 holds, bits 5-0 pattern index (0-63, or a relative-sprite pattern-index OFFSET added to the anchor's own pattern index if attr4's PO bit is set)"},
	{0x39, 0x00, nextRegRW, "sprite attribute 4 for the currently selected sprite (T-29/T-30 completion pass): only consulted when NR 0x38 bit 6 ('E') is set. Dual format discriminated by bits 7-6 alone: %01 marks a RELATIVE sprite (bit 5 selects composite-vs-unified TYPE for the anchor that owns this bit; for a relative sprite itself, bit 5 is decoded but not consumed -- the wiki documents relative type as an anchor-level choice, not per-relative), bits 4-3/2-1 X/Y scale (relative's own, composite mode only -- unified mode uses the anchor's), bit 0 'PO' (add anchor's pattern index to this sprite's own). Any other bits-7-6 value marks an ANCHOR: bit 5 sets the relative TYPE this anchor's own following relatives use, bits 4-3/2-1 this anchor's X/Y scale (%00=1x %01=2x %10=4x %11=8x), bit 0 Y position MSB (9th bit, combined with NR 0x36). H/N6 (bits 7-6 as %1x on an anchor) flag 4-bit sprite patterns -- not implemented (no second pattern-memory format exists), decoded as if H were clear rather than silently misread as a relative sprite; see sprite.go's file header for the full cross-checked bit table"},
	{0x40, 0x00, nextRegRW, "palette index (T-31, wave ti0.r5): selects index 0-255 within whichever palette NR 0x43 bits 6-4 currently target for 0x41/0x44 read/write; writing this register also resets 0x44's own two-write sequence (nextpalette.go)"},
	{0x41, 0x00, nextRegRW, "palette value, 8-bit write (T-31): RRRGGGBB, missing third blue bit synthesised as (bit1 OR bit0) of the 2-bit blue field; auto-increments the 0x40 index afterward unless NR 0x43 bit 7 disables it; a read returns the CURRENT entry re-encoded to this 8-bit view, not the last byte written -- see nextpalette.go"},
	{0x43, 0x00, nextRegRW, "palette control (T-31): bit 7 disables index auto-increment, bits 6-4 select which of the 8 palettes 0x40/0x41/0x44 target, bits 3-2-1 independently select the currently-displayed (first/second) Sprites/Layer2/ULA palette, bit 0 enables ULANext mode (stored, not consumed -- a separate ULA feature); see nextpalette.go for the full table and the inferred-not-confirmed note on this register also resetting 0x44's sequence"},
	{0x4B, spriteTransparentIndexDefault, nextRegRW, "sprite transparency index (T-29/T-30 completion pass): the raw pattern-byte value treated as transparent for a sprite pixel, compared BEFORE the palette offset is applied (wiki.specnext.dev/Sprite_Transparency_Index_Register's own explicit wording); default 0xE3 per that page. For 4-bit sprites (not implemented in this emulator) the wiki documents only the low nibble as significant -- see sprite.go's spritePixelIndex for why the full byte is used here instead"},
	{0x44, 0x00, nextRegRW, "palette value, 9-bit write (T-31): two consecutive writes -- first RRRGGGBB as 0x41, second byte bit 0 the true third blue bit and bit 7 a Layer-2-only priority flag (accepted, not stored -- per-pixel priority compositing is out of this wave's scope); auto-increments and resets the sequence after the second write; no confirmed read behaviour, see nextpalette.go"},
}

// newNextRegisterFile builds a register file with every implemented
// register at its documented power-on default and every other register
// number defaulting to 0x00/R-W (see file header: unimplemented
// registers store-and-ignore rather than refusing).
func newNextRegisterFile() *NextRegisterFile {
	nr := &NextRegisterFile{}
	for i := range nr.access {
		nr.access[i] = nextRegRW
	}
	for _, d := range nextRegDefaults {
		nr.regs[d.reg] = d.value
		nr.access[d.reg] = d.access
	}
	// MMU slots 0x50-0x57: default mapping per the wiki -- slots 0/1
	// map to ROM (0xFF sentinel), slots 2-7 mirror the classic 128K
	// layout (banks 10,11,4,5,0,1) so a next-mode machine that hasn't
	// touched the MMU yet still boots into something address-sane.
	// Read/write; ti0.r2 (T-28) is what makes these values affect
	// actual memory access.
	mmuBootDefaults := [8]uint8{0xFF, 0xFF, 10, 11, 4, 5, 0, 1}
	for i, v := range mmuBootDefaults {
		reg := uint8(0x50 + i)
		nr.regs[reg] = v
		nr.access[reg] = nextRegRW
		nr.mmuSlots[i] = v
	}
	return nr
}

// clipIndex tracks the Layer 2 clip window's own internal write
// sequence (X1,X2,Y1,Y2), separate from the general register array
// since register 0x18 aliases four logical values behind one port --
// confirmed by the wiki's own description ("sequential writes set
// X1, X2, Y1, Y2 positions... reads do not advance the position
// counter"), so read and write intentionally do not share one cursor.
type nextRegClip struct {
	writeIndex uint8    // 0..3, wraps: next write goes to X1,X2,Y1,Y2 in turn
	values     [4]uint8 // X1, X2, Y1, Y2
}

func newNextRegClip() *nextRegClip {
	// Reset default 0,255,0,191 per the wiki (full-screen clip window).
	return &nextRegClip{values: [4]uint8{0, 255, 0, 191}}
}

func (c *nextRegClip) write(v uint8) {
	c.values[c.writeIndex] = v
	c.writeIndex = (c.writeIndex + 1) % 4
}

// read returns the byte at the *current* write index without advancing
// it, matching "reads do not advance the position counter" -- a read
// mid-sequence sees whatever the next write would overwrite, not a
// separate read cursor.
func (c *nextRegClip) read() uint8 {
	return c.values[c.writeIndex]
}

// selectRegister implements a write to 0x243B.
func (io *SpectrumIO) nextRegSelect(value uint8) {
	io.nextRegs.selected = value
}

// selectedRegister implements a read from 0x243B: returns the
// currently selected register number. See the file-level comment for
// why this behaviour is inferred rather than directly documented.
func (io *SpectrumIO) nextRegSelected() uint8 {
	return io.nextRegs.selected
}

// writeSelected implements a write to 0x253B: stores into whichever
// register nextRegSelect last chose, honouring write-only vs
// read-only, and updating the clip-window/MMU-slot mirrors where the
// selected register is one of those.
func (io *SpectrumIO) nextRegWriteSelected(value uint8) {
	io.nextRegWriteDirect(io.nextRegs.selected, value)
}

// nextRegWriteDirect writes value into register reg, independent of
// whatever nextRegSelect last chose -- the shared body behind BOTH the
// port-0x253B path (nextRegWriteSelected, which reads reg from
// io.nextRegs.selected first) and the Z80N NEXTREG-opcode path
// (nextRegOpcodeWrite, wired as the CPUs NextregWrite hook), which must
// NOT read or touch io.nextRegs.selected at all -- on real hardware
// (and per jnext, github.com/jorgegv/jnext, GH #54/RevivalSurvival.nex)
// the opcode writes its own register argument directly into the
// register file and leaves whatever register was previously selected
// via port 0x243B completely untouched. See EnableZ80N in zenzx.go for
// where NextregWrite is wired, and nextreg_test.go for the regression
// proving the two paths genuinely diverge on `selected` state.
func (io *SpectrumIO) nextRegWriteDirect(reg uint8, value uint8) {
	if io.nextRegs.access[reg] == nextRegRO {
		// Read-only registers silently ignore writes -- matches real
		// hardware behaviour for identification registers (there is no
		// documented write-triggers-an-error semantics anywhere in the
		// sources consulted).
		return
	}
	if reg == 0x18 {
		io.nextRegClipWin.write(value)
		io.nextRegs.regs[reg] = io.nextRegClipWin.read()
		return
	}
	io.nextRegs.regs[reg] = value
	if reg == 0x70 {
		// Layer 2 palette offset changed -- see nextPaletteVersion's own
		// doc comment (io.go) and docs/proposals/next-gpu-compositing-seam.md
		// for why this is the one live trigger for now.
		io.nextPaletteVersion++
	}
	if reg == 0x34 {
		// Sprite number select (T-30) -- io.sprites is constructed by
		// NewSpectrumIO the same way nextRegClipWin is, so it is never nil
		// on a real SpectrumIO; the guard matches this file's established
		// io.memory defensiveness convention for the rare test/zero-value
		// construction path that skips NewSpectrumIO entirely.
		if io.sprites != nil {
			io.sprites.selected = value
		}
	}
	if reg >= 0x35 && reg <= 0x39 {
		// Sprite attribute write for whichever sprite NR 0x34 selected --
		// mirrors register 0x18's own delegation to nextRegClipWin.write
		// above in spirit (a NextREG write fanning out to richer state than
		// a flat byte), but sprites keep their per-register readback in
		// io.nextRegs.regs[reg] too (unlike 0x18's clip window, which
		// overwrites regs[reg] with read()'s own cursor-relative value) --
		// see sprite.go's spriteSystem.writeAttribute for the actual
		// per-sprite state this fans out to.
		if io.sprites != nil {
			io.sprites.writeAttribute(reg-0x35, value)
		}
	}
	if reg == 0x40 {
		// Palette index select (T-31) -- io.nextPalette is constructed by
		// NewSpectrumIO the same way sprites is, so it is never nil on a
		// real SpectrumIO; the guard matches this file's established
		// io.memory defensiveness convention.
		if io.nextPalette != nil {
			io.nextPalette.writeIndexReg(value)
		}
	}
	if reg == 0x41 {
		if io.nextPalette != nil {
			io.nextPalette.writeValue8(value)
			io.nextPaletteVersion++
		}
	}
	if reg == 0x43 {
		if io.nextPalette != nil {
			io.nextPalette.writeControlReg(value)
			io.nextPaletteVersion++
		}
	}
	if reg == 0x44 {
		if io.nextPalette != nil {
			before := io.nextPalette.awaitingSecondByte
			io.nextPalette.writeValue9(value)
			// Only the write that COMPLETES a colour (the second byte of
			// the pair, or the second write when a sequence was already
			// pending) actually changes a palette entry -- the first byte
			// of a pair is buffered, not yet resolved to a colour, so
			// bumping the version on it would invalidate GPU caches for a
			// write that changed nothing observable yet.
			if before {
				io.nextPaletteVersion++
			}
		}
	}
	if reg >= 0x50 && reg <= 0x57 {
		io.nextRegs.mmuSlots[reg-0x50] = value
		// T-28 item #26: drive the actual paging model, not just the
		// mmuSlots readback mirror above. This applies the register
		// value uniformly across all 8 slots via SpectrumMemory's
		// existing nextPageRead/nextPageWrite reserved-range gating
		// (nextPageIsReserved, 0xE0-0xFF) -- it does NOT yet implement
		// the slots-0/1-defer-to-legacy-ROM special case for the
		// reserved range that the jnext-comparison addendum in
		// docs/TRACKING.md T-28 describes; that upgrade is item #27,
		// a separate, later change layered on top of this wiring, not
		// a re-do of it. The nil guard matches this file's existing
		// io.memory defensiveness convention (see e.g. the isTS2068
		// check above and floatingbus.go).
		if io.memory != nil {
			io.memory.SetNextSlot(int(reg-0x50), value)
		}
	}
}

// nextRegOpcodeWrite is the Z80N NEXTREG-opcode write path (ED 91/92),
// wired as zx.cpu.NextregWrite by EnableZ80N. It must go straight to
// nextRegWriteDirect and must NEVER read or write io.nextRegs.selected
// -- that is the entire point of this function existing separately from
// nextRegWriteSelected (see nextRegWriteDirects doc comment).
func (io *SpectrumIO) nextRegOpcodeWrite(reg uint8, value uint8) {
	io.nextRegWriteDirect(reg, value)
}

// readSelected implements a read from 0x253B.
func (io *SpectrumIO) nextRegReadSelected() uint8 {
	reg := io.nextRegs.selected
	if io.nextRegs.access[reg] == nextRegWO {
		return 0xFF
	}
	if reg == 0x18 {
		return io.nextRegClipWin.read()
	}
	if reg == 0x41 && io.nextPalette != nil {
		return io.nextPalette.readValue8()
	}
	return io.nextRegs.regs[reg]
}

// GetNextRegister returns register reg's current value regardless of
// access mode -- for snapshot/debug/test use, not the guest-facing
// port path (which honours write-only/read-only above).
func (io *SpectrumIO) GetNextRegister(reg uint8) uint8 {
	if reg == 0x18 {
		return io.nextRegClipWin.read()
	}
	if reg == 0x41 && io.nextPalette != nil {
		return io.nextPalette.readValue8()
	}
	return io.nextRegs.regs[reg]
}

// SetNextRegister writes register reg's value directly -- for
// snapshot restore, bypassing access-mode checks the same way
// GetNextRegister bypasses them for reads.
func (io *SpectrumIO) SetNextRegister(reg uint8, value uint8) {
	io.nextRegs.regs[reg] = value
	if reg == 0x70 {
		// Layer 2 palette offset changed on snapshot restore too -- mirrors
		// nextRegWriteDirect's own handling of 0x70 above, for the same
		// reason the mmuSlots sync below mirrors that function's 0x50-0x57
		// handling: a restored snapshot must produce the same live state a
		// sequence of direct writes would have.
		io.nextPaletteVersion++
	}
	if reg == 0x34 && io.sprites != nil {
		io.sprites.selected = value
	}
	if reg >= 0x35 && reg <= 0x39 && io.sprites != nil {
		io.sprites.writeAttribute(reg-0x35, value)
	}
	if reg == 0x40 && io.nextPalette != nil {
		io.nextPalette.writeIndexReg(value)
	}
	if reg == 0x41 && io.nextPalette != nil {
		io.nextPalette.writeValue8(value)
		io.nextPaletteVersion++
	}
	if reg == 0x43 && io.nextPalette != nil {
		io.nextPalette.writeControlReg(value)
		io.nextPaletteVersion++
	}
	if reg == 0x44 && io.nextPalette != nil {
		// Snapshot restore always bumps the version on 0x44, unlike the
		// live-write path's before/after check in nextRegWriteDirect --
		// a restored snapshot's own saved awaitingSecondByte state isn't
		// captured by this single-register call, so this conservatively
		// treats every restored 0x44 write as cache-invalidating rather
		// than risk under-invalidating.
		io.nextPalette.writeValue9(value)
		io.nextPaletteVersion++
	}
	if reg >= 0x50 && reg <= 0x57 {
		io.nextRegs.mmuSlots[reg-0x50] = value
		// T-28 item #26: keep the paging model in sync on snapshot
		// restore too, the same as a live register write does in
		// nextRegWriteDirect -- otherwise a restored snapshot would show
		// the correct mmuSlots readback but stale/wrong actual paging.
		if io.memory != nil {
			io.memory.SetNextSlot(int(reg-0x50), value)
		}
	}
}

// GetMMUSlots returns the eight NextREG MMU slot values (registers
// 0x50-0x57), for ti0.r2's paging model to read once it exists.
func (io *SpectrumIO) GetMMUSlots() [8]uint8 {
	return io.nextRegs.mmuSlots
}
