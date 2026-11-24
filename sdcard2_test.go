package zenzx

import "testing"

// Port 0xE7 chooses between two cards: bit 0 low the first, bit 1 low the second.
func TestSecondCardIsSelectedByItsOwnBit(t *testing.T) {
	first, second := make(memCard, 1<<20), make(memCard, 1<<20)
	first[0], second[0] = 0x11, 0x22
	d := NewDivMMC(nil, newWriteTestCard(first, len(first)))
	exchange := func(out uint8) uint8 {
		d.WriteData(out)
		return d.ReadData()
	}
	// readFirstByte reads block 0 from whichever card is selected and returns its first byte.
	readFirstByte := func() (uint8, bool) {
		for _, b := range []uint8{0x40 | 17, 0, 0, 0, 0, 0xFF} { // CMD17, block 0
			exchange(b)
		}
		for i := 0; i < 64; i++ { // the response, then the data token, then the block
			if exchange(0xFF) == 0xFE {
				return exchange(0xFF), true
			}
		}
		return 0, false
	}

	// An empty second slot reads as nothing there.
	d.WriteCS(0xFD)
	if v, ok := readFirstByte(); ok {
		t.Fatalf("an empty slot answered a read with %02X", v)
	}
	if got := exchange(0xFF); got != 0xFF {
		t.Fatalf("an empty slot read %02X, want FF", got)
	}
	d.WriteCS(0xFF)

	d.sd2 = newWriteTestCard(second, len(second))
	d.WriteCS(0xFE) // the first card
	if v, ok := readFirstByte(); !ok || v != 0x11 {
		t.Fatalf("with bit 0 low: %02X (answered %v), want the first card's 11", v, ok)
	}
	d.WriteCS(0xFF)
	d.WriteCS(0xFD) // the second
	if v, ok := readFirstByte(); !ok || v != 0x22 {
		t.Fatalf("with bit 1 low: %02X (answered %v), want the second card's 22", v, ok)
	}
	d.WriteCS(0xFF)
	if d.sd.csAsserted || d.sd2.csAsserted {
		t.Fatal("a card is still selected with both bits high")
	}
}
