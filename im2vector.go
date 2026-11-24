package zenzx

// The whole of the IM 2 vector.
//
// In interrupt mode 2 the Z80 takes the address of its handler from a table: the table is
// at I*256 plus the byte the interrupting device puts on the data bus, all eight bits of
// it. Zilog's own peripherals always give an even byte, and the CPU core (zen80 0.5.7)
// takes that as the CPU's rule and clears bit 0 itself. A Spectrum's ULA gives no byte at
// all: the bus floats and the CPU reads 0xFF, so the handler's address is at I*256+0xFF and
// the byte after it, in the next page. A program with a table of 257 equal bytes cannot
// tell the difference, and most have one. One that sets only the two bytes that are read
// (the palette cycling demo on the Next's card sets 0xFEFF and 0xFF00, for a handler in
// page 0xBF) gets its low byte from the wrong place and goes to the wrong address.
//
// Until the core uses the whole byte, the handler's address is read again here when the
// vector was odd. With a core that does it right this reads the same address a second
// time, and can then be removed.

// im2Accepted is called when the CPU has just accepted an interrupt in mode 2.
func (zx *ZenZX) im2Accepted() {
	v := zx.io.intVector
	if v&1 == 0 {
		return
	}
	addr := uint16(zx.cpu.I)<<8 | uint16(v)
	zx.cpu.PC = uint16(zx.memory.Read(addr)) | uint16(zx.memory.Read(addr+1))<<8
	zx.cpu.WZ = zx.cpu.PC
}
