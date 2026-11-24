package zenzx

// DivMMC models the ZX Spectrum Next's built-in DivMMC interface: the
// automap/CONMEM memory-paging logic behind IO port 0xE3, layered on
// top of the SD-over-SPI transport in sdcard.go (ports 0xE7/0xEB).
// Part of T-36's Tier 2 boot chain (docs/TRACKING.md,
// docs/NEXT_SUPPORT_DEVELOPMENT_PLAN.md).
//
// What this does NOT model: the FPGA's own separate "config mode" boot
// ROM (docs/NEXT_BOOT_REFERENCE.md stage 1's "nextboot.rom", gated by
// NextREG 0x03's boot-rom-enable bit) is a distinct mechanism from
// DivMMC's own automap ROM below -- real hardware runs the
// config-mode boot ROM BEFORE any classic-compatible personality (and
// DivMMC's own automap addressing) is even active. That remains a
// separate, unstarted gap (NEXT_SUPPORT_DEVELOPMENT_PLAN.md Tier 2
// item 6, "NR 0x03 as a real register"), not folded into this file.
// sdcard.go's SD/SPI transport is shared physical hardware either way,
// so it's already usable by both once that gap closes.
//
// Sourcing for the memory-mapping behaviour below (0xE3's bit
// meanings, the automap trigger/exit addresses): confirmed against two
// independent sources, cross-checked against each other rather than
// taken from either alone --
//
//   - the ZX-Uno core's own Spectrum-Next-compatible DivMMC
//     implementation (cores/Spectrum/new_memory.v, a real, shipped
//     hardware core), for the 0xE3 bit layout (CONMEM/MAPRAM/SRAM
//     page) and the low-8K-vs-high-8K addressing rules, and
//   - a ZX-Uno developer's own account of implementing this same logic
//     (board.esxdos.org, "ESXDOS in the ZX-Uno hardware: NMI not
//     working"), which independently states the same six automap
//     entry addresses and the 0x1FF8-0x1FFF exit range, plus that
//     DivMMC "mandates a minimum of 128K RAM" (vs DivIDE's 32K) --
//     this file's own SRAM sizing below matches that figure.
//
// Neither source is the ZX Spectrum Next's own FPGA core repository
// directly; one direct confirmation against
// gitlab.com/SpectrumNext/ZX_Spectrum_Next_FPGA is worth doing before
// this is trusted as authoritative rather than well-corroborated.
//
// One confirmed real-hardware subtlety is deliberately NOT modelled:
// Garry Lancaster (NextZXOS's own author) reported against the real
// Next FPGA core (gitlab.com/SpectrumNext/ZX_Spectrum_Next_FPGA issue
// #5) that automap entry at 0x0038 takes effect one byte late (the
// byte AT 0x0038 itself still executes from whatever was mapped
// before), specifically so esxDOS's own RST-hook interrupt-reentrancy
// works. This stub's own target -- a cold-boot SD read, not
// esxDOS-hosted RST-hook reentrancy -- doesn't exercise that edge
// case, but any future esxDOS-hosting work should revisit this before
// relying on automap timing being exact.
type DivMMC struct {
	ctrl uint8 // last value written to port 0xE3

	// paged is the automap's own internal enable latch, set/cleared by
	// M1-cycle PC matches (OnM1) independently of ctrl's CONMEM bit --
	// mirrors real hardware's separate divmmc_is_paged register
	// (confirmed against the ZX-Uno source above): automap state is
	// genuinely distinct from what the control register holds, not
	// derived from it.
	paged bool

	// bootROM is paged in at 0x0000-0x1FFF whenever DivMMC memory is
	// active and not "MAPRAM without CONMEM" (see MemRead). Despite
	// the name this is DivMMC's own ESXDOS-hosting ROM, not the
	// FPGA's separate config-mode boot ROM -- see this type's header
	// comment.
	bootROM [8192]byte

	// sram is 16 pages x 8K = 128K, matching the "minimum 128K RAM"
	// figure the board.esxdos.org source above states for real DivMMC
	// hardware (as distinct from the older, smaller DivIDE standard).
	sram [16][8192]byte

	sd *SDCard
	// sd2 is the card in the second slot, nil when it is empty; second is set while port
	// 0xE7 has it alone selected (sdcard2.go).
	sd2    *SDCard
	second bool

	// automapEnabled gates whether automap TRIGGERS (OnM1's PC checks)
	// are even considered -- NOT whether the overlay is visible once
	// paged. Confirmed against jnext (src/peripheral/divmmc.h/.cpp):
	// real hardware has two independent enable levers, "port I/O
	// enable" (always on; DivMMC-hardware-present, not modelled
	// separately here since this emulator has no scenario where it'd
	// ever be off) and NextREG 0x0A bit 4 (automap enable specifically,
	// confirmed OFF at power-on). CONMEM is gated by neither -- it
	// "sails through" regardless, per jnext's own comment.
	//
	// jnext's own commit history (its set_enabled's doc comment)
	// describes hitting exactly the bug this field fixes: before they
	// added this gate, "cold-boot automap fire[d] at PC=0x0000 ... and
	// overlay[ed] enNxtmmc.rom on slot 0 throughout the boot ROM
	// phase" -- unconditional automap hijacking the interrupt vectors
	// (0x0008/0x0038/0x0066) during a phase of boot where real hardware
	// hasn't enabled it yet, which is exactly the shape of a real,
	// observed stall this session hit (a permanent JR $ self-loop after
	// TBBLUE.FW's own core-selection UI, confirmed via instruction
	// trace) once boot got far enough for the distinction to matter.
	// Defaults false (zero value), matching real hardware's own
	// power-on default.
	automapEnabled bool

	// The programmable entry points and the delayed mapping they need: divmmc_entries.go.
	entry0, valid0, timing0, entry1 uint8
	rom3Fn                          func() bool   // nil: ROM 3 taken as present
	pending                         int           // a delayed map or unmap not yet applied
	fetchPC                         uint16        // the opcode fetch that caused it
	pcFn                            func() uint16 // the CPU's program counter, for instant entry points
	controlReadable                 bool          // the Next's port 0xE3 reads back

	// Debug, when true, logs automap trigger/exit transitions and
	// every 0xE3 control-register write to stderr -- off by default,
	// no behaviour change unless a caller sets it (zenzx_headless.go/
	// zenzx_gui.go's -divmmc-debug flag, which also sets sd.Debug).
	Debug bool
}

// NewDivMMC constructs a DivMMC with the given ESXDOS/boot ROM image
// (copied into an internal 8192-byte array; a shorter image is
// zero-padded, a longer one silently truncated -- callers should treat
// either as their own error rather than relying on this coping
// quietly) and the SD card it talks to.
func NewDivMMC(romImage []byte, sd *SDCard) *DivMMC {
	d := &DivMMC{sd: sd}
	copy(d.bootROM[:], romImage)
	d.resetEntryPoints()
	return d
}

// SD returns the attached SD card, for io.go's port dispatch.
func (d *DivMMC) SD() *SDCard { return d.sd }

// SetAutomapEnabled is called from nextreg.go on every write to
// NextREG 0x0A, passing bit 4. On an enabled-to-disabled transition,
// also clears paged immediately -- confirmed against jnext (DA-08,
// mirroring VHDL's i_automap_reset): disabling automap drops any
// in-progress overlay right away rather than waiting for the normal
// exit-address path.
func (d *DivMMC) SetAutomapEnabled(enabled bool) {
	if d.automapEnabled && !enabled && d.paged {
		if d.Debug {
			tracef("[divmmc] automap disabled (NR 0x0A bit 4 cleared) -- dropping active overlay\n")
		}
		d.paged = false
	}
	d.automapEnabled = enabled
}

// conmem/mapram/sramPage decode ctrl's three fields -- bit7=CONMEM,
// bit6=MAPRAM mode, bits3:0=SRAM page (0-15) -- confirmed against the
// ZX-Uno source cited in this file's header comment.
func (d *DivMMC) conmem() bool    { return d.ctrl&0x80 != 0 }
func (d *DivMMC) mapram() bool    { return d.ctrl&0x40 != 0 }
func (d *DivMMC) sramPage() uint8 { return d.ctrl & 0x0F }

// active reports whether DivMMC memory currently substitutes for the
// CPU's lower 16K, via either the forced (CONMEM) or automatic
// (automap) path.
func (d *DivMMC) active() bool {
	return d.conmem() || d.paged
}

// MemRead/MemWrite are memory.go's divMMCReadFn/WriteFn targets.
// ok/handled=false means "DivMMC isn't intercepting this address, fall
// through to the normal dispatch unchanged" -- the same contract
// layer2AccessReadFn/WriteFn already established for the Layer 2
// Access Port (T-29 completion pass).
func (d *DivMMC) MemRead(address uint16) (uint8, bool) {
	d.settle(address)
	if !d.active() || address >= 0x4000 {
		return 0, false
	}
	if address < 0x2000 {
		if d.conmem() || !d.mapram() {
			return d.bootROM[address], true
		}
		// MAPRAM without CONMEM: the low 8K is forced to SRAM page 3,
		// read-only (confirmed against both sources in the header
		// comment).
		return d.sram[3][address], true
	}
	return d.sram[d.sramPage()][address-0x2000], true
}

func (d *DivMMC) MemWrite(address uint16, value uint8) bool {
	d.settle(address)
	if !d.active() || address >= 0x4000 {
		return false
	}
	if address < 0x2000 {
		return true // ROM, or the MAPRAM page-3 alias -- both read-only, write dropped
	}
	page := d.sramPage()
	if d.mapram() && page == 3 {
		return true // page 3 is write-protected in MAPRAM mode
	}
	d.sram[page][address-0x2000] = value
	return true
}

// WriteControl/ReadControl handle port 0xE3 ("divMMC Control",
// confirmed against github.com/avlixa/zx_spectrum_next_03.01.09's own
// ports.txt).
//
// MAPRAM (bit 6) is a one-way latch on real DivIDE/DivMMC hardware:
// confirmed across several independent DivIDE programming-model docs
// (velesoft.speccy.cz/zx/divide, k1.spdns.de's DivIDE references) --
// "once set to '1', MAPRAM can only be reset again with a power-on";
// a plain RESET leaves it unchanged too. Firmware relies on this being
// true (observed directly: real ESXDOS/DivMMC ROM code loops writing
// alternating CONMEM|page and CONMEM|MAPRAM|page values, apparently
// self-checking that MAPRAM actually stuck, and never proceeds if it
// didn't). This emulator has no separate power-on-vs-soft-reset
// distinction yet, so the latch simply never clears for one DivMMC
// instance's whole lifetime, which is only ever constructed once per
// process run -- matching the power-on-default-is-0 case fine, but
// worth revisiting if F1 soft-reset and true power-cycle ever need to
// behave differently here.
func (d *DivMMC) WriteControl(value uint8) {
	if d.ctrl&0x40 != 0 {
		value |= 0x40
	}
	if d.Debug && value != d.ctrl {
		tracef("[divmmc] ctrl <- 0x%02X (CONMEM=%v MAPRAM=%v page=%d)\n",
			value, value&0x80 != 0, value&0x40 != 0, value&0x0F)
	}
	d.ctrl = value
}

// ReadControl returns 0xFF (floating bus) rather than the stored
// value: gitlab.com/SpectrumNext/ZX_Spectrum_Next_FPGA issue #5 states
// outright that "on an original divmmc interface, the port containing
// CONMEM is not readable" -- that source discusses it as a limitation
// the Next's own core might improve on, so whether the Next's port is
// actually readable is genuinely unconfirmed either way; this keeps
// the more conservative, source-stated default rather than assuming
// an improvement that wasn't directly confirmed.
//
// On the Next the port is documented as read/write (ports.txt, "0xE3 Divmmc control (R/W)"),
// so there it reads back; controlReadable is set when the interface is wired to a Next.
func (d *DivMMC) ReadControl() uint8 {
	if d.controlReadable {
		return d.ctrl
	}
	return 0xFF
}
