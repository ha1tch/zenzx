package zenzx

import (
	"bytes"
	"testing"
)

// memCard is a card image in memory that can be written.
type memCard []byte

func (m memCard) ReadAt(p []byte, off int64) (int, error)  { return copy(p, m[off:]), nil }
func (m memCard) WriteAt(p []byte, off int64) (int, error) { return copy(m[off:], p), nil }

func newWriteTestCard(img SDCardImage, size int) *SDCard {
	c := NewSDCard(img, int64(size))
	c.initialised = true
	c.WriteCS(0xFE) // select
	return c
}

// sdExchange clocks one byte out and returns the byte the card shifts back.
func sdExchange(c *SDCard, out uint8) uint8 {
	c.WriteData(out)
	return c.ReadData()
}

// sdCommand sends a command frame and returns R1, skipping the idle bytes before it.
func sdCommand(t *testing.T, c *SDCard, index uint8, arg uint32) uint8 {
	t.Helper()
	for _, b := range []uint8{0x40 | index, uint8(arg >> 24), uint8(arg >> 16), uint8(arg >> 8), uint8(arg), 0xFF} {
		c.WriteData(b)
	}
	for i := 0; i < 8; i++ {
		if r := sdExchange(c, 0xFF); r != 0xFF {
			return r
		}
	}
	t.Fatalf("CMD%d: no R1 within 8 bytes", index)
	return 0xFF
}

func sdSendBlock(c *SDCard, token uint8, fill uint8) uint8 {
	c.WriteData(token)
	for i := 0; i < 512; i++ {
		c.WriteData(fill)
	}
	c.WriteData(0x12) // CRC, not checked
	c.WriteData(0x34)
	return sdExchange(c, 0xFF) & 0x1F
}

func sdWaitReady(t *testing.T, c *SDCard) {
	t.Helper()
	sawBusy := false
	for i := 0; i < 8; i++ {
		switch sdExchange(c, 0xFF) {
		case 0x00:
			sawBusy = true
		case 0xFF:
			if !sawBusy {
				t.Fatal("the card never went busy after the data response")
			}
			return
		}
	}
	t.Fatal("the card stayed busy")
}

func TestSDCardWritesASingleBlock(t *testing.T) {
	img := make(memCard, 16*512)
	c := newWriteTestCard(img, len(img))
	if r1 := sdCommand(t, c, 24, 3); r1 != 0x00 {
		t.Fatalf("CMD24 R1 = 0x%02X, want 0x00", r1)
	}
	sdExchange(c, 0xFF) // a gap byte before the token, as drivers send
	if resp := sdSendBlock(c, sdTokenSingle, 0xA5); resp != sdDataAccepted {
		t.Fatalf("data response 0x%02X, want 0x05 (accepted)", resp)
	}
	sdWaitReady(t, c)
	if !bytes.Equal(img[3*512:4*512], bytes.Repeat([]byte{0xA5}, 512)) {
		t.Fatal("block 3 of the image does not hold the data written")
	}
	if img[2*512+511] != 0 || img[4*512] != 0 {
		t.Fatal("the write touched a neighbouring block")
	}
	// The card takes commands again.
	if r1 := sdCommand(t, c, 13, 0); r1 != 0x00 {
		t.Fatalf("CMD13 R1 after a good write = 0x%02X", r1)
	}
	if status := sdExchange(c, 0xFF); status != 0x00 {
		t.Fatalf("CMD13 status after a good write = 0x%02X, want 0x00", status)
	}
}

func TestSDCardWritesMultipleBlocks(t *testing.T) {
	img := make(memCard, 16*512)
	c := newWriteTestCard(img, len(img))
	if r1 := sdCommand(t, c, 25, 5); r1 != 0x00 {
		t.Fatalf("CMD25 R1 = 0x%02X", r1)
	}
	for i, fill := range []uint8{0x11, 0x22, 0x33} {
		if resp := sdSendBlock(c, sdTokenMulti, fill); resp != sdDataAccepted {
			t.Fatalf("block %d: data response 0x%02X", i, resp)
		}
		sdWaitReady(t, c)
	}
	c.WriteData(sdTokenStop)
	sdExchange(c, 0xFF) // the byte the card takes to turn round after the stop token
	sdWaitReady(t, c)
	for i, fill := range []uint8{0x11, 0x22, 0x33} {
		if img[(5+i)*512] != fill || img[(5+i)*512+511] != fill {
			t.Fatalf("block %d holds 0x%02X, want 0x%02X", 5+i, img[(5+i)*512], fill)
		}
	}
	if c.wr.blocksWritten != 3 {
		t.Fatalf("%d blocks written, want 3", c.wr.blocksWritten)
	}
	// After the stop token a read works as before.
	if r1 := sdCommand(t, c, 17, 6); r1 != 0x00 {
		t.Fatalf("CMD17 after a multiple-block write: R1 0x%02X", r1)
	}
}

func TestSDCardRefusesWritesWhenProtectedOrReadOnly(t *testing.T) {
	for name, c := range map[string]*SDCard{
		"write-protect switch": func() *SDCard {
			img := make(memCard, 8*512)
			c := newWriteTestCard(img, len(img))
			c.wr.WriteProtected = true
			return c
		}(),
		"read-only image": newWriteTestCard(bytes.NewReader(make([]byte, 8*512)), 8*512),
	} {
		if r1 := sdCommand(t, c, 24, 1); r1 != 0x00 {
			t.Fatalf("%s: CMD24 R1 = 0x%02X", name, r1)
		}
		if resp := sdSendBlock(c, sdTokenSingle, 0x77); resp != sdDataWriteError {
			t.Fatalf("%s: data response 0x%02X, want 0x0D (write error)", name, resp)
		}
		sdWaitReady(t, c)
		if r1 := sdCommand(t, c, 13, 0); r1 != 0x00 {
			t.Fatalf("%s: CMD13 R1 = 0x%02X", name, r1)
		}
		if status := sdExchange(c, 0xFF); status&sdStatusWPViolation == 0 {
			t.Fatalf("%s: CMD13 status 0x%02X does not report the write-protect violation", name, status)
		}
		sdCommand(t, c, 13, 0)
		if status := sdExchange(c, 0xFF); status != 0 {
			t.Fatalf("%s: the status was not cleared by reading it: 0x%02X", name, status)
		}
		if c.wr.blocksWritten != 0 {
			t.Fatalf("%s: a block was written", name)
		}
	}
}

func TestSDCardWriteOutOfRangeAndAbandoned(t *testing.T) {
	img := make(memCard, 4*512)
	c := newWriteTestCard(img, len(img))
	if r1 := sdCommand(t, c, 24, 4); r1&0x40 == 0 {
		t.Fatalf("CMD24 past the end: R1 0x%02X, want the parameter error bit", r1)
	}
	// A write the host abandons by sending another command.
	if r1 := sdCommand(t, c, 24, 1); r1 != 0x00 {
		t.Fatalf("CMD24 R1 = 0x%02X", r1)
	}
	if r1 := sdCommand(t, c, 13, 0); r1 != 0x00 {
		t.Fatalf("CMD13 in place of the data: R1 0x%02X", r1)
	}
	if c.wr.state != sdWriteNone {
		t.Fatal("the abandoned write is still pending")
	}
	// Deselecting the card ends a write too.
	sdCommand(t, c, 24, 1)
	c.WriteCS(0xFF)
	if c.wr.state != sdWriteNone {
		t.Fatal("a write survived the card being deselected")
	}
}
