package zenzx

// The sprite ports.
//
// Programs do not set sprites up through the NextREG mirrors alone: the patterns can only
// be loaded through a port, and NextBASIC and most machine code send the attributes that
// way too. From the Next's ports.txt:
//
//	0x303B (W)  bits 6-0 choose the current sprite, 0 to 127, for port 0x57; and bits 5-0
//	            with bit 7 choose the pattern address for port 0x5B: pattern 0 to 63 of 256
//	            bytes each, bit 7 adding 128 for the second of two 4-bit patterns. The two
//	            are separate quantities inside.
//	0x303B (R)  bit 0 two sprites collided, bit 1 too many sprites on a line; reading
//	            clears both. Neither is modelled: it reads 0.
//	0x57   (W)  the current sprite's attributes, one byte after another: four, or five
//	            when the fourth has bit 6 set. After the last one the current sprite
//	            moves on to the next, 127 wrapping to 0.
//	0x5B   (W)  a byte of pattern data at the current address, which then moves on by one
//	            (through all 16K of pattern memory, wrapping at the end).
//
// and from registers.txt:
//
//	NextREG 0x09 bit 4  lockstep: NextREG 0x34 and port 0x303B then set the same sprite
//	NextREG 0x34        the sprite NextREG 0x35-0x39 and 0x75-0x79 write to (in lockstep,
//	                    also the one port 0x57 writes to)
//	NextREG 0x75-0x79   as 0x35-0x39, and the sprite number then goes up by one
//
// The low byte alone decodes ports 0x57 and 0x5B.

const spritePatternBytes = spritePatternSlots * spriteWidth * spriteHeight

// spritePorts is the state the three ports keep, apart from the sprites themselves.
type spritePorts struct {
	sprite      uint8 // the sprite port 0x57 writes to
	attrIndex   uint8 // which of its attributes comes next, 0 to 4
	patternAddr int   // where port 0x5B writes, 0 to spritePatternBytes-1
}

// spriteLockstep reports NextREG 0x09 bit 4.
func (io *SpectrumIO) spriteLockstep() bool {
	return io.nextRegs.regs[0x09]&0x10 != 0
}

// spriteSlotPortWrite is a write to port 0x303B.
func (io *SpectrumIO) spriteSlotPortWrite(value uint8) {
	p := &io.sprites.ports
	p.sprite, p.attrIndex = value&0x7F, 0
	p.patternAddr = int(value&0x3F)*spriteWidth*spriteHeight + int(value>>7)*128
	if io.spriteLockstep() {
		io.sprites.selected = p.sprite
		io.nextRegs.regs[0x34] = p.sprite
	}
}

// spriteAttributePortWrite is a write to port 0x57.
func (io *SpectrumIO) spriteAttributePortWrite(value uint8) {
	p := &io.sprites.ports
	a := &io.sprites.attrs[p.sprite&0x7F]
	last := false
	switch p.attrIndex {
	case 0:
		a.attr0 = value
	case 1:
		a.attr1 = value
	case 2:
		a.attr2 = value
	case 3:
		a.attr3 = value
		if value&0x40 == 0 { // no fifth byte: the sprite is a plain anchor
			a.attr4 = 0
			last = true
		}
	default:
		a.attr4 = value
		last = true
	}
	p.attrIndex++
	if last {
		p.attrIndex = 0
		p.sprite = (p.sprite + 1) & 0x7F
		if io.spriteLockstep() {
			io.sprites.selected = p.sprite
			io.nextRegs.regs[0x34] = p.sprite
		}
	}
}

// spritePatternPortWrite is a write to port 0x5B.
func (io *SpectrumIO) spritePatternPortWrite(value uint8) {
	p := &io.sprites.ports
	slot, offset := p.patternAddr/(spriteWidth*spriteHeight), p.patternAddr%(spriteWidth*spriteHeight)
	io.sprites.patterns[slot][offset] = value
	io.sprites.patternGeneration[slot]++
	p.patternAddr = (p.patternAddr + 1) % spritePatternBytes
}

// spriteSelectRegWrite is a write to NextREG 0x34: the sprite the attribute registers write
// to, and in lockstep the one the ports write to as well.
func (io *SpectrumIO) spriteSelectRegWrite(value uint8) {
	io.sprites.selected = value & 0x7F
	if io.spriteLockstep() {
		p := &io.sprites.ports
		p.sprite, p.attrIndex = value&0x7F, 0
		p.patternAddr = int(value&0x3F)*spriteWidth*spriteHeight + int(value>>7)*128
	}
}

// spriteAttributeRegIncrement is what follows a write to NextREG 0x75-0x79: the selected
// sprite goes up by one.
func (io *SpectrumIO) spriteAttributeRegIncrement() {
	io.spriteSelectRegWrite((io.sprites.selected + 1) & 0x7F)
	io.nextRegs.regs[0x34] = io.sprites.selected
}
