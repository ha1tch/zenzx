package zenzx

import (
	"io"
)

// sdIdle is the universal SPI bus filler / NCR / Nac-gap byte value.
// Real cards drive MISO high (0xFF) whenever they aren't actively
// shifting out a meaningful byte -- confirmed against jnext
// (src/peripheral/sd_card.cpp's own kIdle constant and its citations
// of SD Physical Layer Simplified Spec § 7.5.4).
const sdIdle = 0xFF

// SDCardImage is the minimal block-read interface an SDCard needs from
// whatever backs it -- the same raw bytes pkg/nextdisk's fatfs.Volume
// interprets as a filesystem one layer up, read here instead at the
// plain 512-byte-sector level real SD/MMC hardware actually speaks.
// *os.File and bytes.NewReader-wrapped images both already satisfy
// this via io.ReaderAt.
type SDCardImage interface {
	io.ReaderAt
}

// SDCard emulates an SD card's SPI-mode command protocol closely
// enough for real Next firmware's own documented boot sequence
// (docs/NEXT_BOOT_REFERENCE.md stage 1: "CMD12 -> CMD0 -> CMD8 ->
// ACMD41 -> CMD58") to succeed, plus CMD9 and CMD17 for the register
// read and the actual sector reads that follow.
//
// This file was rewritten this session (T-36) after direct comparison
// against jorgegv/jnext's own sd_card.cpp -- a reference
// implementation with an extensive, real-firmware-verified history of
// exactly these command formats (their own comments cite specific
// TBBLUE.FW/enNxtmmc.rom memory addresses their fixes were driven by).
// Every non-obvious byte value and timing detail below is ported from
// there, cited individually, rather than re-derived -- this stub's own
// earlier version got CMD0's persistent-response behaviour and CMD8's
// R7 format wrong in exactly the ways jnext's own history describes
// hitting and fixing.
//
// Deliberate remaining simplifications, not ported from jnext:
//
//   - No CRC *checking* on incoming command frames (SPI-mode CRC is
//     normally disabled after CMD8, and real firmware issuing
//     well-formed commands doesn't need this stub to validate them).
//   - CMD55/ACMD41 gating: the boot path only ever does CMD55 then
//     ACMD41, so command index 41 is treated as ACMD41 whenever it
//     follows CMD55, without jnext's fuller "fall through to the
//     regular CMD switch for any other post-CMD55 command" handling.
//   - CMD9 (SEND_CSD) and CMD18 (READ_MULTIPLE_BLOCK) are implemented
//     -- both confirmed required by real firmware this session
//     (enNxtmmc.rom's own SD-init flow for CMD9; TBBLUE.FW's FatFs for
//     CMD18) -- but CMD10/16/23/24 (CID, set-blocklen, writes) are
//     not; they fall to the illegal-command default below.
type SDCard struct {
	img       SDCardImage
	sizeBytes int64

	// programAt are the places in the image where blocks that begin a BASIC
	// program were read, oldest first, until the machine asks which files
	// they are (program_name.go). A few are kept: the first may be the
	// system's own start-up file.
	programAt []int64

	csAsserted bool
	cmdBuf     []byte // bytes of the SD command frame collected so far (0-6)
	pending    []byte // response bytes still to be shifted out
	lastByte   uint8  // most recent byte the "card" shifted back

	// persistentByte is what ReadData returns once pending drains to
	// empty, until a new command starts or CS deasserts -- confirmed
	// against jnext, which itself cites ZEsarUX's real-hardware-derived
	// mmc_read() (storage/mmc.c:854-857): SD Physical Layer Simplified
	// Spec § 7.3.2 says the bus goes to 0xFF (idle) after R1, but
	// TBBlue-family firmware's MMC_Init polls for the first non-0xFF
	// byte and checks (R1&0xFE)==0 -- a strictly spec-faithful 0xFF
	// tail reads as 0xFE once masked, which the firmware treats as a
	// CRC error, and it never proceeds. This was directly observed
	// this session: without persistent-byte behaviour, real
	// nextboot.rom firmware retried CMD0 in an unbounded loop.
	persistentByte uint8

	initialised bool
	appCmd      bool // set true by CMD55, consumed by the next command
	hostSDHC    bool // latched from ACMD41's HCS argument bit (30)

	// multiBlock/multiBlockSector track an open CMD18
	// (READ_MULTIPLE_BLOCK) stream: true between CMD18 and whatever
	// ends it (CMD12, or the image running out -- NOT CS deassert, see
	// WriteCS's own comment). multiBlockSector is the next sector to
	// load once pending's current block finishes draining -- see
	// nextResponseByte/loadNextMultiBlock.
	multiBlock       bool
	multiBlockSector uint32

	// freshByte is true immediately after WriteData produces a new
	// lastByte, and false once a ReadData call has consumed it.
	// Confirmed necessary by direct instruction-trace evidence this
	// session: real firmware sometimes polls with a tight bare
	// IN A,(0xEB) loop -- no paired OUT at all -- waiting for the
	// first non-0xFF byte, which nothing could ever satisfy while
	// only WriteData advanced the response queue. But the earlier,
	// working part of the same boot sequence pairs each OUT with an
	// immediate IN to read that same clock's response, and that IN
	// must NOT ALSO advance the queue, or every multi-byte response
	// (CMD8's R7, CMD9's CSD, CMD17's data block) would drain twice as
	// fast as bytes actually arrive and misalign, the same class of
	// bug the start-bit resync elsewhere in this file already fixed
	// once. freshByte distinguishes the two: true means "the next
	// ReadData is this write's own paired read, don't clock again";
	// false means "this ReadData has no fresh write behind it, clock
	// it exactly like a WriteData(0xFF) would."
	freshByte bool

	// wr is the state of a write in progress, and the write-protect switch: sdcard_write.go.
	wr sdWriteState

	// Debug, when true, logs every CS transition and command/response
	// frame to stderr -- off by default (zero value), no behaviour
	// change unless a caller sets it (zenzx_headless.go/zenzx_gui.go's
	// -divmmc-debug flag).
	Debug bool
}

// NewSDCard constructs an SDCard backed by img, which holds exactly
// sizeBytes of raw sector data (typically a whole SD-card image file,
// FAT32 filesystem and all -- SDCard itself knows nothing about
// filesystems, only sectors; pkg/nextdisk reads the FAT32 structure
// this card exposes, one layer up).
func NewSDCard(img SDCardImage, sizeBytes int64) *SDCard {
	return &SDCard{img: img, sizeBytes: sizeBytes, lastByte: 0xFF, persistentByte: 0xFF}
}

// WriteCS handles port 0xE7 ("SPI CS (sd card, flash, rpi)", confirmed
// against github.com/avlixa/zx_spectrum_next_03.01.09's own ports.txt
// -- the official FPGA core repository's port table). Bit 0 clear
// asserts CS (selects the card): the universal SD/MMC-over-SPI
// convention, not independently confirmed against real Next firmware
// this session, since ports.txt names the port but not its bit
// polarity. Deselecting mid-command discards whatever partial command/
// response was in flight, matching real SPI behaviour -- EXCEPT an
// open CMD18 stream, which survives CS deassert. Confirmed against
// jnext, which cites this as a specific, previously-hit NextZXOS-boot
// bug (its own commit dated 2026-07-09): in SPI mode, deasserting CS
// pauses the card; a multi-block read is ended by CMD12 (or a new
// command), not by CS. esxDOS's own driver deselects between blocks
// and later reselects, polling port 0xEB for the next block WITHOUT
// sending any command -- aborting the stream on CS deassert makes that
// poll see endless 0xFF and time out, exactly the failure jnext
// documents hitting and fixing.
func (c *SDCard) WriteCS(value uint8) {
	asserted := value&0x01 == 0
	if c.Debug {
		tracef("[sdcard] CS write 0x%02X -> %s (was %s)\n", value,
			map[bool]string{true: "asserted", false: "deasserted"}[asserted],
			map[bool]string{true: "asserted", false: "deasserted"}[c.csAsserted])
	}
	if asserted != c.csAsserted {
		c.cmdBuf = c.cmdBuf[:0]
		c.appCmd = false
		c.wr.state = sdWriteNone // a write does not survive the card being deselected
		c.freshByte = false
		c.persistentByte = 0xFF
		if !c.multiBlock {
			c.pending = c.pending[:0]
		}
	}
	c.csAsserted = asserted
}

// WriteData handles port 0xEB ("SPI DATA" per the same ports.txt): one
// full-duplex byte shift. Real Next hardware provides a byte-at-a-time
// SPI shifter here, not a bit-banged GPIO pin, so one port access moves
// a whole byte each way, matching how real Next firmware drives it.
// value is whatever the host is currently driving out; the byte the
// "card" simultaneously shifts back becomes readable via ReadData
// until the next call.
//
// The read side (draining a queued response, or returning
// persistentByte once that queue is empty) and the write side
// (collecting a new command) are independent, not mutually exclusive:
// real SPI shifts a bit each way on every clock, so the master's
// outgoing byte is a real byte on the wire even on a clock that's also
// draining a preceding command's last response byte. Treating them as
// either/or was a real, observed bug this session: real firmware
// sometimes starts clocking a new command's first byte on the very
// same transfer that drains the previous command's last response
// byte, and discarding that byte instead of capturing it shifted every
// subsequent command frame by one byte, permanently.
//
// The write side resyncs on the SD command start-bit pattern (bits
// 7:6 == 0b01, i.e. 0x40-0x7F) rather than blindly counting every byte
// that goes by -- confirmed against jnext's own receive(), which uses
// the identical `(tx & 0xC0) == 0x40` check. This matters once the
// read side can be draining a large response (a whole 512-byte CMD17
// data block): without resyncing, the 0xFF filler bytes real firmware
// sends while reading that block would themselves get miscounted as
// command bytes every 6 bytes. A start-bit byte arriving while a
// response is still draining also aborts that response, matching
// jnext's confirmed behaviour (its receive() does the same in its
// RESPONDING/SENDING_DATA/WRITE_RESP states), not just when idle.
func (c *SDCard) WriteData(value uint8) {
	if !c.csAsserted {
		if c.Debug {
			tracef("[sdcard] data write 0x%02X ignored (CS deasserted)\n", value)
		}
		c.lastByte = 0xFF
		return
	}

	if len(c.pending) > 0 || c.multiBlock {
		b := c.nextResponseByte()
		if c.Debug {
			tracef("[sdcard] data write 0x%02X -> clocked out 0x%02X (%d bytes left)\n", value, b, len(c.pending))
		}
		c.lastByte = b
	} else {
		c.lastByte = c.persistentByte
	}
	c.freshByte = true

	// A write in progress takes the byte: a data token, the data, or filler (sdcard_write.go).
	if len(c.cmdBuf) == 0 && c.writeDataByte(value) {
		return
	}

	if len(c.cmdBuf) == 0 {
		if value&0xC0 != 0x40 {
			if c.Debug {
				tracef("[sdcard] data write 0x%02X ignored (not a command start-bit byte, not mid-frame)\n", value)
			}
			return
		}
		// New command start aborts any still-draining response and
		// resets the persistent byte to the IDLE-state default --
		// confirmed against jnext (its receive() does the same on
		// every fresh RECEIVING_CMD transition, in or out of IDLE).
		// Also terminates an open CMD18 stream even though this isn't
		// CMD12: confirmed against jnext's own defensive handling --
		// the SD spec only legalises CMD12 or CS-deassert as abort
		// mechanisms (and WriteCS's own comment covers why CS deassert
		// alone must NOT abort it), but a new, different command
		// arriving mid-stream must not leave multiBlock set and
		// silently resume after this command's own response drains.
		c.pending = c.pending[:0]
		c.persistentByte = 0xFF
		c.multiBlock = false
		c.multiBlockSector = 0
	}
	c.cmdBuf = append(c.cmdBuf, value)
	if c.Debug {
		tracef("[sdcard] data write 0x%02X -> cmdBuf now % 02X\n", value, c.cmdBuf)
	}
	if len(c.cmdBuf) == 6 {
		if c.Debug {
			tracef("[sdcard] cmd frame % 02X\n", c.cmdBuf)
		}
		c.execCommand(c.cmdBuf)
		if c.Debug {
			n := len(c.pending)
			if n > 16 {
				tracef("[sdcard]   -> resp % 02X ... (%d bytes total)\n", c.pending[:16], n)
			} else {
				tracef("[sdcard]   -> resp % 02X\n", c.pending)
			}
		}
		c.cmdBuf = c.cmdBuf[:0]
	}
}

// ReadData returns the byte most recently shifted back from the card
// (port 0xEB read).
//
// Confirmed necessary by direct instruction-trace evidence this
// session (not inferred): real firmware sometimes polls with a tight
// bare `IN A,(0xEB)` loop -- no paired `OUT` at all -- waiting for the
// first non-0xFF byte. If ReadData only ever returned whatever
// WriteData last produced, such a loop could never see anything past
// the first stale byte and would spin forever, which is exactly what
// was observed: thousands of identical polling reads, all 0xFF,
// against a queue that genuinely had a non-0xFF byte still in it a
// few positions deep.
//
// freshByte (see its own field comment) distinguishes this from the
// earlier, working part of the same boot sequence, which pairs each
// OUT with an immediate IN to read that specific clock's response --
// that paired IN must NOT also clock, or multi-byte responses would
// drain twice as fast as bytes actually arrive.
func (c *SDCard) ReadData() uint8 {
	if c.freshByte {
		c.freshByte = false
		return c.lastByte
	}
	if !c.csAsserted {
		return 0xFF
	}
	if len(c.pending) > 0 || c.multiBlock {
		b := c.nextResponseByte()
		if c.Debug {
			tracef("[sdcard] bare data read -> clocked out 0x%02X (%d bytes left)\n", b, len(c.pending))
		}
		c.lastByte = b
	} else {
		c.lastByte = c.persistentByte
	}
	return c.lastByte
}

// nextResponseByte returns the next byte the card shifts out. If
// pending is non-empty, drains it as before. If pending has just run
// dry and a CMD18 stream is open (multiBlock), transparently loads the
// next sector and continues the stream (see loadNextMultiBlock) rather
// than falling back to persistentByte -- ported from jnext's own
// "block complete -> load next sector, re-prime" continuation logic in
// its SENDING_DATA state (src/peripheral/sd_card.cpp).
func (c *SDCard) nextResponseByte() uint8 {
	if len(c.pending) == 0 && c.multiBlock {
		c.loadNextMultiBlock()
	}
	if len(c.pending) > 0 {
		b := c.pending[0]
		c.pending = c.pending[1:]
		return b
	}
	return c.persistentByte
}

// noteBlock remembers where a block that begins a BASIC program was read.
func (c *SDCard) noteBlock(offset int64, block []byte) {
	if len(c.programAt) < 8 && plus3DOSProgram(block) {
		c.programAt = append(c.programAt, offset)
	}
}

// loadNextMultiBlock loads multiBlockSector into pending as the next
// CMD18 block: one filler byte (0xFF) + the 0xFE data token + 512
// sector bytes + a real CRC-16 -- confirmed against jnext, which
// specifically inserts that one filler byte "so the host's 'skip until
// non-0xFF then expect 0xFE' token-poll... finds the new token on a
// subsequent read rather than the same byte that terminated the
// previous block's CRC" (its own comment cites tbblue's diskio.c
// rcvr_datablock). A sector past the end of the image ends the stream
// with a single data-error token (0x08, OUT_OF_RANGE) instead of
// silently going idle -- also confirmed against jnext, whose own
// history describes the silent-idle version as a real, spec-diverging
// bug it had to fix (a real card emits a discriminative error token,
// not a timeout).
func (c *SDCard) loadNextMultiBlock() {
	const sectorSize = 512
	sector := c.multiBlockSector
	offset := int64(sector) * sectorSize
	if c.img == nil || offset < 0 || offset+sectorSize > c.sizeBytes {
		c.pending = []byte{0x08}
		c.multiBlock = false
		c.multiBlockSector = 0
		return
	}
	buf := make([]byte, sectorSize)
	_, _ = c.img.ReadAt(buf, offset)
	c.noteBlock(offset, buf)
	crc := sdCRC16(buf)
	resp := make([]byte, 0, 2+sectorSize+2)
	resp = append(resp, sdIdle, 0xFE)
	resp = append(resp, buf...)
	resp = append(resp, byte(crc>>8), byte(crc))
	c.pending = resp
	c.multiBlockSector = sector + 1
}

// r1 is the plain R1 response byte: 0x01 (idle state bit set) before
// ACMD41 has completed, 0x00 (no error/idle bits set) after.
func (c *SDCard) r1() uint8 {
	if c.initialised {
		return 0x00
	}
	return 0x01
}

// queueR1 queues exactly 2 NCR idle bytes then r1 -- the universal
// response shape for any command that's "just R1", confirmed against
// jnext's own queue_r1 (which cites SD Physical Layer Simplified Spec
// § 7.5.4, Ncr = 1..8): firmware readers that hardcode a fixed Ncr=2
// gap need exactly this many idle bytes before R1, and poll-based
// readers that just skip leading 0xFF bytes work fine with it too.
func (c *SDCard) queueR1(r1 uint8) {
	c.pending = []byte{sdIdle, sdIdle, r1}
}

// execCommand decodes one complete 6-byte SD command frame
// ([0x40|index, arg0, arg1, arg2, arg3, crc]) and sets c.pending (and
// c.persistentByte, for the commands that need one) accordingly. CRC
// (frame[5]) is never checked -- see the type's own doc comment.
func (c *SDCard) execCommand(frame []byte) {
	index := frame[0] & 0x3F
	arg := uint32(frame[1])<<24 | uint32(frame[2])<<16 | uint32(frame[3])<<8 | uint32(frame[4])

	if c.appCmd {
		c.appCmd = false
		if index == 41 {
			c.cmdACMD41(arg)
			return
		}
		// Falls through to the regular switch below -- see this
		// type's own doc comment on why the fuller jnext behaviour
		// (spec-correct for any other post-CMD55 command) isn't
		// replicated.
	}

	switch index {
	case 0:
		c.cmd0GoIdle()
	case 8:
		c.cmd8SendIfCond(frame)
	case 9:
		c.cmd9SendCSD()
	case 12:
		c.cmd12StopTransmission()
	case 13:
		c.cmd13SendStatus()
	case 16, 23:
		// SET_BLOCKLEN and (after CMD55) SET_WR_BLK_ERASE_COUNT: accepted, nothing to do.
		c.queueR1(c.r1())
	case 17:
		c.cmd17ReadSingleBlock(arg)
	case 18:
		c.cmd18ReadMultipleBlock(arg)
	case 24:
		c.cmdWriteBlock(arg, false)
	case 25:
		c.cmdWriteBlock(arg, true)
	case 55:
		c.cmd55AppCmd()
	case 58:
		c.cmd58ReadOCR()
	default:
		// Illegal-command bit (R1 bit 2) set, confirmed against jnext
		// (its own comment cites SD Physical Layer Simplified Spec
		// § 7.3.2.1): CMD10 and anything else this stub doesn't
		// implement lands here. (CMD13, 16, 23, 24 and 25 came with
		// writing, sdcard_write.go.)
		c.queueR1(c.r1() | 0x04)
	}
}

// cmd0GoIdle handles CMD0 (GO_IDLE_STATE). See persistentByte's own
// doc comment for why 0x01 must persist after the transient queue
// drains, not fall back to 0xFF.
func (c *SDCard) cmd0GoIdle() {
	c.initialised = false
	c.queueR1(0x01)
	c.persistentByte = 0x01
}

// cmd8SendIfCond handles CMD8 (SEND_IF_COND). R7 = R1 + a 4-byte
// register whose byte 0 is a FIXED command-version field (0001 per SD
// Physical Layer Simplified Spec § 7.3.2.6 Table 7-9, i.e. 0x10), not
// an echo of whatever the host sent in that argument position --
// confirmed against jnext, which explicitly fixed exactly this
// (echoing the raw argument bytes back, as this stub's own earlier
// version did too, produces an illegal command-version field). Only
// the trailing check-pattern byte is an echo of the host's argument.
func (c *SDCard) cmd8SendIfCond(frame []byte) {
	check := frame[4]
	c.pending = []byte{sdIdle, sdIdle, c.r1(), 0x10, 0x00, 0x01, check}
}

// cmd9SendCSD handles CMD9 (SEND_CSD): confirmed required specifically
// by enNxtmmc.rom's own SD-init flow (jnext's own comment cites
// firmware addresses $1925/$1F3D) -- without a real CSD register, the
// firmware reads 0xFF for all 16 bytes and aborts the boot. Register
// content is a generic-but-valid CSD v2.0 (SDHC) layout, ported
// directly from jnext rather than re-derived; sizeBytes feeds C_SIZE
// so the reported capacity matches whatever image is actually
// attached.
func (c *SDCard) cmd9SendCSD() {
	if !c.initialised {
		c.queueR1(0x01)
		return
	}
	sizeIn512K := uint64(c.sizeBytes) / (512 * 1024)
	var cSize uint64
	if sizeIn512K > 0 {
		cSize = sizeIn512K - 1
	}
	csd := [16]byte{
		0x40, 0x0E, 0x00, 0x32,
		0x5B, 0x59, 0x00, uint8((cSize >> 16) & 0x3F),
		uint8((cSize >> 8) & 0xFF), uint8(cSize & 0xFF), 0x7F, 0x80,
		0x0A, 0x40, 0x00, 0x01,
	}
	crc := sdCRC16(csd[:])
	resp := make([]byte, 0, 5+len(csd)+2)
	resp = append(resp, sdIdle, sdIdle, 0x00, sdIdle, 0xFE)
	resp = append(resp, csd[:]...)
	resp = append(resp, byte(crc>>8), byte(crc))
	c.pending = resp
}

// cmd12StopTransmission handles CMD12 (STOP_TRANSMISSION), which also
// terminates an open CMD18 stream if one is active -- confirmed
// against jnext. The 8 leading "stuff bytes" and the persistent 0xFF
// tail (not R1's own 0x00/0x01, unlike CMD0) are both confirmed
// against jnext, which cites the exact NextZXOS supervisor code doing
// the polling (bank-2 $196D-$1978): "IN A,($EB); AND A; JR Z,$1972"
// polls until a NON-ZERO byte, treating 0x00 as "still busy" -- so the
// persistent byte here must be 0xFF (idle/non-zero), or the firmware
// would read a persistent R1=0x00 as permanently busy.
func (c *SDCard) cmd12StopTransmission() {
	c.multiBlock = false
	c.multiBlockSector = 0
	c.pending = []byte{sdIdle, sdIdle, sdIdle, sdIdle, sdIdle, sdIdle, sdIdle, sdIdle, sdIdle, c.r1()}
	c.persistentByte = 0xFF
}

func (c *SDCard) cmd55AppCmd() {
	c.appCmd = true
	c.queueR1(c.r1())
}

// cmdACMD41 handles ACMD41 (SD_SEND_OP_COND), reached via CMD55.
// Latches the host's HCS (Host Capacity Support, argument bit 30) for
// CMD58 to reflect in OCR's CCS bit -- confirmed against jnext, which
// fixed an earlier unconditional-CCS=1 divergence from spec here.
func (c *SDCard) cmdACMD41(arg uint32) {
	c.hostSDHC = arg&0x40000000 != 0
	c.initialised = true
	c.queueR1(0x00)
}

// cmd58ReadOCR handles CMD58 (READ_OCR). Two details confirmed against
// jnext, both non-obvious: CCS (bit 30, SDHC) is only set if the most
// recent ACMD41 requested it (HCS bit), not unconditionally; and the
// voltage-window byte is 0x7F, not the spec-plausible 0xFF, because
// tbblue.fw's OCR reader treats any 0xFF response byte as still-idle
// filler and skips it, misaligning every OCR byte after it by one.
func (c *SDCard) cmd58ReadOCR() {
	var ocr0 uint8
	if c.initialised {
		ocr0 = 0x80
		if c.hostSDHC {
			ocr0 |= 0x40
		}
	}
	c.pending = []byte{sdIdle, sdIdle, c.r1(), ocr0, 0x7F, 0x80, 0x00}
}

// cmd17ReadSingleBlock handles CMD17 (READ_SINGLE_BLOCK). sector is a
// block number, not a byte address (SDHC/CCS=1 addressing, matching
// the CMD58 answer above). The leading 2 NCR + 1 Nac gap byte before
// the 0xFE data token, the real CRC-16 trailer, and the out-of-range
// R1 PARAMETER_ERROR (bit 6) + data-error-token shape are all
// confirmed against jnext: a zero-gap response caused a real, observed
// bug in another emulator's command sender (a flush byte sent right
// after latching R1 swallowed the token), and firmware that validates
// the trailing CRC-16 needs a real one, not a dummy value.
func (c *SDCard) cmd17ReadSingleBlock(sector uint32) {
	if !c.initialised {
		c.queueR1(0x01)
		return
	}
	const sectorSize = 512
	offset := int64(sector) * sectorSize
	if c.img == nil || offset < 0 || offset+sectorSize > c.sizeBytes {
		c.pending = []byte{sdIdle, sdIdle, 0x40, 0x08}
		return
	}
	buf := make([]byte, sectorSize)
	_, _ = c.img.ReadAt(buf, offset)
	c.noteBlock(offset, buf)
	crc := sdCRC16(buf)
	resp := make([]byte, 0, 5+sectorSize+2)
	resp = append(resp, sdIdle, sdIdle, 0x00, sdIdle, 0xFE)
	resp = append(resp, buf...)
	resp = append(resp, byte(crc>>8), byte(crc))
	c.pending = resp
}

// cmd18ReadMultipleBlock handles CMD18 (READ_MULTIPLE_BLOCK):
// confirmed required by TBBLUE.FW's own FatFs -- jnext's own history
// documents "Error reading TBBLUE.FW data!" without it, and this
// session hit that exact message on real firmware before this was
// implemented. Shaped like CMD17 for the first block; the stream then
// continues automatically (see nextResponseByte/loadNextMultiBlock)
// until the host sends CMD12, or (see WriteCS's own comment) CS
// deassert alone does NOT end it.
func (c *SDCard) cmd18ReadMultipleBlock(sector uint32) {
	if !c.initialised {
		c.queueR1(0x01)
		return
	}
	const sectorSize = 512
	offset := int64(sector) * sectorSize
	if c.img == nil || offset < 0 || offset+sectorSize > c.sizeBytes {
		c.pending = []byte{sdIdle, sdIdle, 0x40, 0x08}
		return
	}
	buf := make([]byte, sectorSize)
	_, _ = c.img.ReadAt(buf, offset)
	c.noteBlock(offset, buf)
	crc := sdCRC16(buf)
	resp := make([]byte, 0, 5+sectorSize+2)
	resp = append(resp, sdIdle, sdIdle, 0x00, sdIdle, 0xFE)
	resp = append(resp, buf...)
	resp = append(resp, byte(crc>>8), byte(crc))
	c.pending = resp
	c.multiBlock = true
	c.multiBlockSector = sector + 1
}

// sdCRC16 is the CRC-16/XMODEM variant (poly 0x1021, init 0x0000, no
// reflection, no final XOR) the SD Physical Layer Simplified Spec
// requires for every SPI-mode data block's trailing CRC (§ 4.5/7.2.4).
// Ported from jnext's own sd_crc16; independently re-verified this
// session against the standard check value CRC16("123456789")=0x31C3
// (via a standalone Python port, run to confirm before this Go version
// was written -- this environment has no Go toolchain to check the Go
// code itself against that vector directly).
func sdCRC16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
