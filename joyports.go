package zenzx

// The two joystick ports, and what is plugged into each.
//
// Every joystick mode comes down to a kind of interface for each of two ports. The
// classic modes fix them (joystick.go). On a Next the machine chooses, in NextREG 0x05,
// and the firmware sets it from the card's config.ini (Kempston 1 on the left port and
// Sinclair on the right, as shipped), and a program can change it:
//
//	bits 7-6 and 3   left port's mode: bit 3 is the top bit of three
//	bits 5-4 and 1   right port's
//	  000  Sinclair, keys 1 2 3 4 5        100  Kempston 2, port 0x37
//	  001  Kempston 1, port 0x1F           101  MD pad on port 0x1F
//	  010  Cursor, keys 5 6 7 8 0          110  MD pad on port 0x37
//	  011  Sinclair, keys 6 7 8 9 0        111  user defined keys (not modelled)
//
// A Kempston port on a Next has a second fire button in bit 5; an MD pad adds A in bit 6
// and START in bit 7, with B and C as the two fire buttons (the Next's ports.txt). On the
// classic machines bits 5 to 7 stay 0.
//
// The keyboard kinds (Sinclair, Cursor) press keys. They are kept apart from the keys
// the host keyboard presses (joyPressed, joined in when the keyboard port is read), so
// neither can release the other's keys: before 0.18.0 the GUI's keyboard handler cleared
// the matrix after the joystick had pressed its keys, and the Sinclair modes did nothing.

type joyKind int

const (
	joyNone joyKind = iota
	joyKempston1
	joyKempston2
	joyMD1
	joyMD2
	joySinclair67890
	joySinclair12345
	joyCursor
)

// joyKeys is the matrix position of each direction and fire for a keyboard kind.
type joyKeys struct{ Left, Right, Down, Up, Fire [2]uint8 }

// cursorKeyMatrix is the Cursor (Protek, AGF) joystick: 5 left, 6 down, 7 up, 8 right,
// 0 fire, the keys the Spectrum's own cursor arrows are on.
var cursorKeyMatrix = joyKeys{
	Left:  [2]uint8{3, 4}, // key 5
	Down:  [2]uint8{4, 4}, // key 6
	Up:    [2]uint8{4, 3}, // key 7
	Right: [2]uint8{4, 2}, // key 8
	Fire:  [2]uint8{4, 0}, // key 0
}

// nextJoyKind decodes one port's three mode bits of NextREG 0x05.
func nextJoyKind(mode uint8) joyKind {
	switch mode & 0x07 {
	case 0:
		return joySinclair12345
	case 1:
		return joyKempston1
	case 2:
		return joyCursor
	case 3:
		return joySinclair67890
	case 4:
		return joyKempston2
	case 5:
		return joyMD1
	case 6:
		return joyMD2
	}
	return joyNone
}

// joystickKinds is what is on each port now.
func (io *SpectrumIO) joystickKinds() (left, right joyKind) {
	switch io.joystickMode {
	case JoystickKempston:
		return joyKempston1, joyNone
	case JoystickKempston2:
		return joyKempston2, joyNone
	case JoystickKempstonBoth:
		return joyKempston1, joyKempston2
	case JoystickSinclair1:
		return joySinclair67890, joyNone
	case JoystickSinclair2:
		return joySinclair12345, joyNone
	case JoystickSinclairBoth:
		return joySinclair67890, joySinclair12345
	case JoystickCursor:
		return joyCursor, joyNone
	case JoystickNext:
		r := io.nextRegs.regs[0x05]
		return nextJoyKind(r>>6 | r>>1&0x04), nextJoyKind(r>>4&0x03 | r<<1&0x04)
	}
	return joyNone, joyNone
}

// joystickTwoPorts reports whether the mode takes a state for each port.
func (io *SpectrumIO) joystickTwoPorts() bool {
	switch io.joystickMode {
	case JoystickSinclairBoth, JoystickKempstonBoth, JoystickNext:
		return true
	}
	return false
}

// applyJoystick puts the two ports' states on whatever is plugged into them.
func (io *SpectrumIO) applyJoystick(left, right JoystickState) {
	next := io.memory != nil && io.memory.isNext
	io.joyPressed = [8]uint8{}
	var k1, k2 uint8
	press := func(m joyKeys, s JoystickState) {
		for _, k := range []struct {
			pos  [2]uint8
			down bool
		}{{m.Left, s.Left}, {m.Right, s.Right}, {m.Down, s.Down}, {m.Up, s.Up}, {m.Fire, s.Fire}} {
			if k.down {
				io.joyPressed[k.pos[0]] |= 1 << k.pos[1]
			}
		}
	}
	kempston := func(s JoystickState, md bool) uint8 {
		b := kempstonByte(s)
		if next && s.Fire2 {
			b |= 0x20
		}
		if md && s.A {
			b |= 0x40
		}
		if md && s.Start {
			b |= 0x80
		}
		return b
	}
	kinds := [2]joyKind{}
	kinds[0], kinds[1] = io.joystickKinds()
	for i, s := range [2]JoystickState{left, right} {
		switch kinds[i] {
		case joyKempston1:
			k1 |= kempston(s, false)
		case joyKempston2:
			k2 |= kempston(s, false)
		case joyMD1:
			k1 |= kempston(s, true)
		case joyMD2:
			k2 |= kempston(s, true)
		case joySinclair67890:
			press(sinclairKeyMatrix1, s)
		case joySinclair12345:
			press(sinclairKeyMatrix2, s)
		case joyCursor:
			press(cursorKeyMatrix, s)
		}
	}
	io.kempstonState, io.kempston2State = k1, k2
}
