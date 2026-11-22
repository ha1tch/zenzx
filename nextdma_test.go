package zenzx

import "testing"

// dmaSend writes a program to the DMA's port.
func dmaSend(io *SpectrumIO, port uint16, bytes ...uint8) {
	for _, b := range bytes {
		io.WritePort(port, b)
	}
}

func TestDMAFillsMemory(t *testing.T) {
	// What Warhawk sends to clear the screen: one byte, the address fixed, to 0x4000 on.
	io := newNextIO()
	io.memory.Write(0xBC94, 0x00)
	for a := uint16(0x4000); a < 0x4010; a++ {
		io.memory.Write(a, 0xAA)
	}
	io.memory.Write(0x5800, 0xAA)
	dmaSend(io, 0x6B, 0x83, 0x7D, 0x94, 0xBC, 0x00, 0x18, 0x24, 0x10, 0xAD, 0x00, 0x40, 0xCF, 0x87)
	if a, b, c := io.memory.Read(0x4000), io.memory.Read(0x57FF), io.memory.Read(0x5800); a != 0 || b != 0 || c != 0xAA {
		t.Fatalf("after a fill of 0x1800 bytes at 0x4000: first %02X, last %02X, the byte after %02X; want 00, 00, AA", a, b, c)
	}
	if io.dma.stolen != 0x1800*6 {
		t.Errorf("the transfer is charged %d CPU cycles, want %d: 3 for each port, each byte", io.dma.stolen, 0x1800*6)
	}
	if io.dma.enabled {
		t.Error("the DMA is still enabled after its block")
	}
}

func TestDMACopiesMemoryAndFillsByOverlap(t *testing.T) {
	io := newNextIO()
	for i := uint16(0); i < 8; i++ {
		io.memory.Write(0x8000+i, uint8(i+1))
	}
	// 0x8000 to 0x9000, both counting up, 8 bytes, with timing bytes as AngryBloaters sends.
	dmaSend(io, 0x6B, 0xC3, 0xC7, 0xCB, 0x7D, 0x00, 0x80, 0x08, 0x00, 0x54, 0x01, 0x50, 0x01, 0xAD, 0x00, 0x90, 0x82, 0xCF, 0xB3, 0x87)
	for i := uint16(0); i < 8; i++ {
		if got := io.memory.Read(0x9000 + i); got != uint8(i+1) {
			t.Fatalf("copied byte %d is %02X, want %02X", i, got, i+1)
		}
	}
	// The overlapping copy that fills: 0xA000 to 0xA001.
	io.memory.Write(0xA000, 0x5A)
	dmaSend(io, 0x6B, 0x7D, 0x00, 0xA0, 0xFF, 0x00, 0xAD, 0x01, 0xA0, 0xCF, 0x87)
	if a := io.memory.Read(0xA0FF); a != 0x5A {
		t.Fatalf("after copying 0xA000 onto 0xA001 for 255 bytes, 0xA0FF is %02X, want 5A", a)
	}
	// Counting down, B to A.
	io.memory.Write(0xB010, 0x11)
	io.memory.Write(0xB00F, 0x22)
	dmaSend(io, 0x6B, 0x79, 0x21, 0xC0, 0x02, 0x00, 0x04, 0x00, 0xAD, 0x10, 0xB0, 0xCF, 0x87) // WR0 B to A; WR1 A counts down; WR2 B counts down
	if a, b := io.memory.Read(0xC021), io.memory.Read(0xC020); a != 0x11 || b != 0x22 {
		t.Fatalf("B to A, both counting down: %02X %02X, want 11 22", a, b)
	}
}

func TestDMASendsSpritePatterns(t *testing.T) {
	// What Nextoid sends: 256 bytes from memory to port 0x5B.
	io := newNextIO()
	for i := 0; i < 256; i++ {
		io.memory.Write(0xC000+uint16(i), uint8(i))
	}
	io.WritePort(0x303B, 3) // pattern 3
	dmaSend(io, 0x6B, 0xC3, 0xC7, 0xCB, 0x7D, 0x00, 0xC0, 0x00, 0x01, 0x54, 0x02, 0x68, 0x02, 0xAD, 0x5B, 0x00, 0x82, 0xCF, 0xB3, 0x87)
	if io.sprites.patterns[3][0] != 0 || io.sprites.patterns[3][255] != 255 || io.sprites.patterns[3][100] != 100 {
		t.Fatalf("pattern 3 after the transfer starts %d, has %d at 100 and ends %d; want 0, 100, 255",
			io.sprites.patterns[3][0], io.sprites.patterns[3][100], io.sprites.patterns[3][255])
	}
	if io.dma.stolen != 256*4 {
		t.Errorf("charged %d cycles, want %d: 2 for each port", io.dma.stolen, 256*4)
	}
}

func TestDMAZ80ModeMovesOneMore(t *testing.T) {
	io := newNextIO()
	dmaSend(io, 0x0B, 0xC3, 0x7D, 0x00, 0x80, 0x04, 0x00, 0x24, 0x10, 0xAD, 0x00, 0x90, 0xCF, 0x87)
	if io.dma.count != 5 {
		t.Errorf("a length of 4 through port 0x0B moved %d bytes, want 5", io.dma.count)
	}
	dmaSend(io, 0x6B, 0xC3, 0x7D, 0x00, 0x80, 0x04, 0x00, 0x24, 0x10, 0xAD, 0x00, 0x90, 0xCF, 0x87)
	if io.dma.count != 4 {
		t.Errorf("a length of 4 through port 0x6B moved %d bytes, want 4", io.dma.count)
	}
}

func TestDMAContinueAndReadBack(t *testing.T) {
	io := newNextIO()
	dmaSend(io, 0x6B, 0xC3, 0x7D, 0x00, 0x80, 0x10, 0x00, 0x14, 0x10, 0xAD, 0x00, 0x90, 0xCF, 0x87)
	// Continue: the count starts again and the addresses go on from where they were.
	dmaSend(io, 0x6B, 0xD3, 0x87)
	if io.dma.aAddr != 0x8020 || io.dma.bAddr != 0x9020 {
		t.Fatalf("after a block of 16 and a continue: A at %04X, B at %04X; want 8020, 9020", io.dma.aAddr, io.dma.bAddr)
	}
	// Read mask 0x7F: status, count, A, B.
	dmaSend(io, 0x6B, 0xBB, 0x7F)
	want := []uint8{0x1B, 0x10, 0x00, 0x20, 0x80, 0x20, 0x90, 0x1B}
	for i, w := range want {
		if got := io.ReadPort(0x006B); got != w {
			t.Errorf("read %d of the sequence is %02X, want %02X", i, got, w)
		}
	}
	// Status alone, before anything has moved.
	dmaSend(io, 0x6B, 0xC3, 0xBF)
	if got := io.ReadPort(0x006B); got != 0x3A {
		t.Errorf("status after a reset is %02X, want 3A", got)
	}
}

func TestDMATimedTransferGoesByTheClock(t *testing.T) {
	// A sample player: bytes to a port at 875 kHz / 55, burst mode. The CPU keeps running.
	io := newNextIO()
	for i := 0; i < 8; i++ {
		io.memory.Write(0x8000+uint16(i), uint8(0x10+i))
	}
	// WR2's timing byte has bit 5: the prescaler follows. Port B is port 0xFE (the border, so the test can see it).
	dmaSend(io, 0x6B, 0xC3, 0x7D, 0x00, 0x80, 0x08, 0x00, 0x14, 0x68, 0x22, 55, 0xCD, 0xFE, 0x00, 0xCF, 0x87)
	if io.dma.count != 0 || io.dma.stolen != 0 {
		t.Fatalf("a timed transfer moved %d bytes and took %d cycles at once, want none", io.dma.count, io.dma.stolen)
	}
	io.dmaTick(1) // the first byte goes at once
	if io.dma.count != 1 {
		t.Fatalf("after the first tick %d bytes have gone, want 1", io.dma.count)
	}
	io.dmaTick(4*55 - 2) // 219 T-states after it was enabled
	if io.dma.count != 1 {
		t.Fatalf("one T-state short of the period %d bytes have gone, want 1", io.dma.count)
	}
	io.dmaTick(1)
	if io.dma.count != 2 || io.borderColor != 0x11&7 {
		t.Fatalf("at the period %d bytes have gone and the border is %d, want 2 and %d", io.dma.count, io.borderColor, 0x11&7)
	}
	io.dmaTick(4 * 55 * 10)
	if io.dma.count != 8 || io.dma.enabled {
		t.Fatalf("after ten more periods %d bytes have gone, enabled %v; want 8 and stopped", io.dma.count, io.dma.enabled)
	}
}

func TestDMAIsOnlyOnANext(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	mem.Write(0x8000, 0x77)
	dmaSend(io, 0x6B, 0x7D, 0x00, 0x80, 0x01, 0x00, 0x14, 0x10, 0xAD, 0x00, 0x90, 0xCF, 0x87)
	if mem.Read(0x9000) == 0x77 {
		t.Error("a 48K machine has a DMA on port 0x6B")
	}
}
