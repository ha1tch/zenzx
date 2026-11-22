package zenzx

import (
	"io"
	"os"
)

// Writing to the SD card (T-43).
//
// The firmware and NextZXOS write to the card in ordinary use: the firmware saves the video
// mode it was told to use, NextZXOS saves its settings, and anything the user saves goes
// there. In SPI mode a write goes like this (SD Physical Layer Simplified Specification,
// section 7.2.4 and 7.3):
//
//	CMD24 (WRITE_BLOCK), answer R1
//	  host: any number of 0xFF, then the token 0xFE, 512 data bytes, 2 CRC bytes
//	  card: the data response token (0x05 accepted, 0x0D write error), then 0x00 while
//	        it is busy, then 0xFF
//	CMD25 (WRITE_MULTIPLE_BLOCK), answer R1
//	  the same for each block with the token 0xFC; the token 0xFD ends the sequence and
//	  is followed by a busy period
//	CMD13 (SEND_STATUS), answer R2: R1 and a second byte of error bits, which a careful
//	  driver reads after a write
//	CMD16 (SET_BLOCKLEN) and ACMD23 (SET_WR_BLK_ERASE_COUNT) are accepted and change
//	  nothing: the block is always 512 bytes
//
// The data CRC is not checked, as the command CRC is not. The image is written when it can
// be (an image that is also an io.WriterAt, such as a file opened for writing) and the card
// is not write-protected; otherwise the block is refused with a write error and CMD13
// reports a write-protect violation, which is how a card with its lock switch set behaves
// to the software.

const (
	sdWriteNone  = iota // no write in progress
	sdWriteToken        // R1 sent for CMD24 or CMD25; waiting for a data token
	sdWriteData         // collecting 512 data bytes and 2 CRC bytes

	sdTokenSingle = 0xFE
	sdTokenMulti  = 0xFC
	sdTokenStop   = 0xFD

	sdDataAccepted   = 0x05
	sdDataWriteError = 0x0D

	sdStatusWPViolation = 0x20 // R2, second byte
	sdStatusError       = 0x04
)

// sdWriteState is the part of an SDCard that exists for writes.
type sdWriteState struct {
	state  int
	multi  bool
	sector uint32
	buf    []byte
	status uint8 // R2's second byte, cleared when CMD13 reads it
	// WriteProtected refuses every write, as the card's lock switch does.
	WriteProtected bool
	// blocksWritten counts blocks that reached the image, for tests and the debug log.
	blocksWritten int
}

// writable reports whether a block can be stored.
func (c *SDCard) writable() (io.WriterAt, bool) {
	if c.wr.WriteProtected || c.img == nil {
		return nil, false
	}
	w, ok := c.img.(io.WriterAt)
	return w, ok
}

func (c *SDCard) cmdWriteBlock(sector uint32, multi bool) {
	if !c.initialised {
		c.queueR1(0x01)
		return
	}
	if int64(sector)*512+512 > c.sizeBytes {
		c.queueR1(0x40) // parameter error: no data phase follows
		return
	}
	c.queueR1(0x00)
	c.wr.state, c.wr.multi, c.wr.sector = sdWriteToken, multi, sector
	c.wr.buf = c.wr.buf[:0]
}

func (c *SDCard) cmd13SendStatus() {
	c.pending = []byte{sdIdle, sdIdle, c.r1(), c.wr.status}
	c.wr.status = 0
}

// writeDataByte takes a byte the host sends while a write is in progress. It reports
// whether the byte belonged to the write; false means it is the start of a new command.
func (c *SDCard) writeDataByte(value uint8) bool {
	switch c.wr.state {
	case sdWriteToken:
		switch {
		case value == sdTokenSingle && !c.wr.multi, value == sdTokenMulti && c.wr.multi:
			c.wr.state = sdWriteData
			c.wr.buf = c.wr.buf[:0]
			c.pending = c.pending[:0]
		case value == sdTokenStop && c.wr.multi:
			c.wr.state = sdWriteNone
			c.pending = []byte{sdIdle, 0x00, sdIdle} // one byte to turn round, busy, ready
			c.persistentByte = sdIdle
			if c.Debug {
				tracef("[sdcard] multiple-block write ended\n")
			}
		case value&0xC0 == 0x40:
			c.wr.state = sdWriteNone // the host gave up and sent a command
			return false
		}
		return true
	case sdWriteData:
		c.wr.buf = append(c.wr.buf, value)
		c.lastByte = sdIdle
		if len(c.wr.buf) == 512+2 {
			c.finishWriteBlock()
		}
		return true
	}
	return false
}

func (c *SDCard) finishWriteBlock() {
	token := uint8(sdDataAccepted)
	if w, ok := c.writable(); !ok {
		token = sdDataWriteError
		c.wr.status |= sdStatusWPViolation
	} else if _, err := w.WriteAt(c.wr.buf[:512], int64(c.wr.sector)*512); err != nil {
		token = sdDataWriteError
		c.wr.status |= sdStatusError
	} else {
		c.wr.blocksWritten++
	}
	if c.Debug {
		tracef("[sdcard] write block %d -> data response 0x%02X\n", c.wr.sector, token)
	}
	// The token comes straight after the CRC, then one busy byte, then the card is ready.
	c.pending = []byte{token, 0x00, sdIdle}
	c.persistentByte = sdIdle
	if c.wr.multi && token == sdDataAccepted {
		c.wr.sector++
		c.wr.state = sdWriteToken
		if int64(c.wr.sector)*512+512 > c.sizeBytes {
			c.wr.state = sdWriteNone
		}
	} else {
		c.wr.state = sdWriteNone
	}
}

// openSDCardImage opens a card image for the front ends: for reading and writing unless
// readOnly is asked for, falling back to read-only when the file cannot be written (the
// card is then write-protected). It reports whether the card ends up write-protected.
func openSDCardImage(path string, readOnly bool) (*os.File, bool, error) {
	if !readOnly {
		if f, err := os.OpenFile(path, os.O_RDWR, 0); err == nil {
			return f, false, nil
		}
	}
	f, err := os.Open(path)
	return f, true, err
}
