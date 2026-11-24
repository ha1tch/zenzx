package zenzx

import "testing"

// These follow the guide on the distribution's card, /docs/zxnDDMA/zxnddma.gde.

// dmaPhys reads and writes the Next's memory by physical address, for setting up and
// checking what the DMA did.
func dmaPhys(io *SpectrumIO, addr uint32) uint8 { return io.memory.nextPhysRead(addr) }

func TestDMAPlusIsFoundTheGuidesWay(t *testing.T) {
	// The guide's detection (3.4): DMA+ bytes after 0xC3, then a one-byte copy. With DMA+
	// the byte goes to physical 0x0B0000 + the address; a plain DMA writes the address.
	io := newNextIO()
	const from, to = 0x8100, 0x8200
	io.memory.Write(from, 0x5A)
	io.memory.Write(to, 0x00)
	program := []uint8{0x83, 0xC3, 0xD7, 0x04, 0xDB, 0x0B, 0xDF, dmaPhysical,
		0x83, 0x83, 0x83, 0x83, 0x83, 0x83, 0x83, 0x83,
		0x7D, from & 0xFF, from >> 8, 0x01, 0x00, 0x14, 0x10, 0xAD, to & 0xFF, to >> 8, 0x82, 0xCF, 0x87}
	dmaSend(io, 0x6B, program...)
	if got := dmaPhys(io, 0x0B0000+to); got != 0x5A {
		t.Errorf("physical 0B%04X holds %02X, want the marker 5A", to, got)
	}
	if got := io.memory.Read(to); got != 0x00 {
		t.Errorf("the byte at %04X was written (%02X): DMA+ was not used", to, got)
	}
	// Bank 2 is the CPU's 0x8000: physical 0x048000. The marker was read from there.
	if dmaPhys(io, 0x040000+from) != 0x5A {
		t.Error("physical 0x048100 is not the CPU's 0x8100")
	}

	// Port 0x0B has no DMA+: the command bytes are dropped and the copy is an ordinary one.
	io = newNextIO()
	io.memory.Write(from, 0x5A)
	dmaSend(io, 0x0B, program...)
	if got := io.memory.Read(to); got != 0x5A {
		t.Errorf("through port 0x0B the byte at %04X is %02X, want the marker", to, got)
	}
}

func TestDMAPlusReachesFourMegabytesOnAnIssue5(t *testing.T) {
	fill := func(io *SpectrumIO, ext uint8, value uint8) {
		// Fill 16 bytes from physical ext:0000 with value.
		dmaSend(io, 0x6B, 0x83, 0xC3, 0xDB, ext, 0xDF, dmaPhysical|dmaFillMode, 0xF7, value,
			0x7D, 0x00, 0x00, 0x10, 0x00, 0x14, 0x10, 0xAD, 0x00, 0x00, 0x82, 0xCF, 0x87)
	}
	io := newNextIO()
	fill(io, 0x30, 0xA5) // 0x300000: in the second two megabytes
	fill(io, 0x10, 0x11) // 0x100000: 8K page 96
	if a, b := dmaPhys(io, 0x300000), dmaPhys(io, 0x30000F); a != 0xA5 || b != 0xA5 {
		t.Fatalf("at 0x300000: %02X ... %02X, want A5", a, b)
	}
	if got := dmaPhys(io, 0x300010); got != 0 {
		t.Errorf("the byte after the block is %02X", got)
	}
	if got := io.memory.nextPageRead(96, 0); got != 0x11 {
		t.Errorf("page 96 holds %02X, want 11: 0x300000 and 0x100000 are different memory on an issue 5", got)
	}
	if io.dma.stolen != 2*16*3 {
		t.Errorf("two fills of 16 bytes took %d cycles, want %d: a fill does not read", io.dma.stolen, 2*16*3)
	}
	// The top six bits of the extended byte are all there are: 0x70 is 0x30.
	if got := dmaPhys(io, 0x700000); got != 0xA5 {
		t.Errorf("0x700000 reads %02X, want what is at 0x300000", got)
	}

	// An issue 4 has two megabytes and drops bit 21: 0x300000 is 0x100000.
	io = newNextIO()
	io.memory.SetNextIssue(4)
	fill(io, 0x30, 0xA5)
	if got := io.memory.nextPageRead(96, 0); got != 0xA5 {
		t.Errorf("on an issue 4, page 96 holds %02X after a fill at 0x300000, want A5", got)
	}
}

func TestDMAPlusCopiesARectangleLeavingTheTransparentColour(t *testing.T) {
	io := newNextIO()
	// A 4x3 shape in a sheet 8 bytes wide, at physical 0x090000 (8K page 40), with holes.
	shape := [3][4]uint8{{1, 0xE3, 2, 3}, {0xE3, 4, 5, 0xE3}, {6, 7, 0xE3, 8}}
	for r, row := range shape {
		for c, v := range row {
			io.memory.nextPhysWrite(0x090000+uint32(r*8+c), v)
		}
	}
	// To Layer 2 256x192 (bank 9) at X 10, Y 20, over a background of 0x77.
	io.nextRegWriteDirect(0x12, 9)
	io.nextRegWriteDirect(0x70, 0x00)
	base := uint32(0x040000 + 9*0x4000)
	for i := uint32(0); i < 256*30; i++ {
		io.memory.nextPhysWrite(base+i, 0x77)
	}
	dmaSend(io, 0x6B, 0x83, 0xC3, 0xD7, 0x09, 0xE3, 4, 0, 0xE7, 3, 0, 0xEB, 8, 0, 0xF3, 0xE3,
		0x7D, 0x00, 0x00, 0x00, 0x00, 0x14, 0x10, 0xA1,
		0xDF, dmaPhysical|dma2D|dmaTransp|dmaLayer2XY, 10, 0, 20, 0x82, 0xCF, 0x87)
	for r, row := range shape {
		for c, v := range row {
			want := v
			if v == 0xE3 {
				want = 0x77
			}
			if got := dmaPhys(io, base+uint32((20+r)*256+10+c)); got != want {
				t.Errorf("row %d column %d on the screen is %02X, want %02X", r, c, got, want)
			}
		}
	}
	if got := dmaPhys(io, base+20*256+14); got != 0x77 {
		t.Errorf("the pixel after the first row was written: %02X", got)
	}
	if io.dma.bStride != 256 {
		t.Errorf("port B's stride after a Layer 2 X,Y is %d, want 256", io.dma.bStride)
	}
	if io.dma.count != 12 {
		t.Errorf("%d bytes moved, want 12: the block length is not used in 2D", io.dma.count)
	}
}

func TestDMAPlusLayer2PositionsInEachMode(t *testing.T) {
	io := newNextIO()
	io.nextRegWriteDirect(0x12, 9)
	io.nextRegWriteDirect(0x13, 14)
	where := func(mode uint8, flags uint8, x uint16, y uint8) uint32 {
		io.nextRegWriteDirect(0x70, mode<<4)
		dmaSend(io, 0x6B, 0xC3, 0xDF, flags, uint8(x), uint8(x>>8), y)
		return uint32(io.dma.bExt)<<16 | uint32(io.dma.bStart)
	}
	xy := uint8(dmaPhysical | dmaLayer2XY)
	for _, c := range []struct {
		name  string
		mode  uint8
		flags uint8
		x     uint16
		y     uint8
		want  uint32
	}{
		{"256x192, row by row", 0, xy, 100, 50, 0x064000 + 50*256 + 100},
		{"256x192 takes no high byte of X", 0, xy, 0x100 + 7, 1, 0x064000 + 256 + 7},
		{"320x256, column by column", 1, xy, 300, 200, 0x064000 + 300*256 + 200},
		{"640x256, X is the byte column", 2, xy, 319, 255, 0x064000 + 319*256 + 255},
		{"the hidden screen", 1, xy | dmaHiddenLayer, 0, 0, 0x040000 + 14*0x4000},
	} {
		if got := where(c.mode, c.flags, c.x, c.y); got != c.want {
			t.Errorf("%s: port B at %06X, want %06X", c.name, got, c.want)
		}
	}
}

func TestDMAGuideCorrections(t *testing.T) {
	// A length of 0 moves one byte on port 0x6B.
	io := newNextIO()
	io.memory.Write(0x8000, 0x12)
	io.memory.Write(0x8001, 0x34)
	dmaSend(io, 0x6B, 0x83, 0xC3, 0x7D, 0x00, 0x80, 0x00, 0x00, 0x14, 0x10, 0xAD, 0x00, 0x90, 0x82, 0xCF, 0x87)
	if a, b := io.memory.Read(0x9000), io.memory.Read(0x9001); a != 0x12 || b != 0x00 {
		t.Errorf("with a length of 0: %02X %02X, want one byte moved", a, b)
	}
	// 0xC3 keeps the start addresses, the direction and the length: after it a load and
	// an enable run the same transfer again.
	io.memory.Write(0x8000, 0x56)
	dmaSend(io, 0x6B, 0xC3, 0xCF, 0x87)
	if got := io.memory.Read(0x9000); got != 0x56 {
		t.Errorf("after 0xC3 the transfer set up before it gave %02X, want 56", got)
	}
	// But it puts back the timings and DMA+'s settings.
	dmaSend(io, 0x6B, 0x54, 0x02, 0xDF, dmaPhysical, 0xD7, 0x12, 0xE3, 5, 0, 0xC3)
	if d := &io.dma; d.aCycles != 3 || d.flags != 0 || d.aExt != 0 || d.rowLen != 0 {
		t.Errorf("after 0xC3: A's cycle length %d, flags %02X, extended byte %02X, row length %d", d.aCycles, d.flags, d.aExt, d.rowLen)
	}
	// Cycle lengths: 00 and 11 are 4, 01 is 3, 10 is 2.
	for bits, want := range map[uint8]int{0: 4, 1: 3, 2: 2, 3: 4} {
		if got := dmaCycles(bits); got != want {
			t.Errorf("cycle length %02b is %d, want %d", bits, got, want)
		}
	}
	// WR4 asking for an interrupt control byte and no address stops the decoder for good.
	dmaSend(io, 0x6B, 0x91, 0xC3, 0x7D, 0x00, 0x70, 0x01, 0x00)
	if !io.dma.hung || io.dma.aStart != 0x8000 {
		t.Errorf("after WR4 0x91: hung %v, port A start %04X (want it unchanged at 8000)", io.dma.hung, io.dma.aStart)
	}
}

func TestPhysicalWritesUnderTheULAsBanksDoNotReachThem(t *testing.T) {
	// Banks 5 and 7 are in block RAM on the machine; the SRAM at their addresses is other
	// memory, which reads back what was written to it.
	io := newNextIO()
	io.memory.Write(0x4000, 0x3C) // bank 5, through the MMU
	io.memory.nextPhysWrite(0x040000+5*0x4000, 0xC3)
	if got := io.memory.Read(0x4000); got != 0x3C {
		t.Errorf("a physical write changed the ULA's bank: %02X", got)
	}
	if got := io.memory.nextPhysRead(0x040000 + 5*0x4000); got != 0xC3 {
		t.Errorf("the SRAM under bank 5 reads %02X, want what was written", got)
	}
}
