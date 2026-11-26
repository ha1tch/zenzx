package zenzx

import "testing"

// Whether a card has been mounted, as far as the machine can tell (SDCard.served): the firmware
// reads a card's partition table to mount it, and a card it has not read from was inserted and
// nothing more.

// guestMounts does to c what the firmware does to mount a card that it has started: reads its
// partition table.
func guestMounts(t *testing.T, c *SDCard) {
	t.Helper()
	c.initialised = true
	c.WriteCS(0xFE)
	if r := sdCommand(t, c, 17, 0); r != 0x00 {
		t.Fatalf("CMD17: R1 = %02X", r)
	}
	c.WriteCS(0xFF)
	if !c.served {
		t.Fatal("a block was read and the card is not served")
	}
}

func TestACardIsServedOnceABlockIsReadFromIt(t *testing.T) {
	img := make(memCard, 1<<20)
	c := NewSDCard(img, int64(len(img)))
	if c.served {
		t.Fatal("a card is served before anything was done to it")
	}
	// started, and selected, and not read from: inserted and not mounted
	c.initialised = true
	c.WriteCS(0xFE)
	if c.served {
		t.Error("a card that was started is taken for one that was read")
	}
	c.WriteCS(0xFF)

	// a read before the card was started is answered 'idle' and reads nothing
	idle := NewSDCard(img, int64(len(img)))
	idle.WriteCS(0xFE)
	if r := sdCommand(t, idle, 17, 0); r != 0x01 || idle.served {
		t.Errorf("a read of a card not started: R1 %02X, served %v", r, idle.served)
	}

	guestMounts(t, c)

	// a stream of blocks counts as well
	m := NewSDCard(img, int64(len(img)))
	m.initialised = true
	m.WriteCS(0xFE)
	if r := sdCommand(t, m, 18, 0); r != 0x00 || !m.served {
		t.Errorf("CMD18: R1 %02X, served %v", r, m.served)
	}
}
