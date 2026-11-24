package zenzx

import "testing"

func newNextIO() *SpectrumIO {
	mem, _ := newTestMemoryAndScreen()
	mem.Enable128K()
	mem.EnablePlus3()
	mem.EnableNext()
	return NewSpectrumIO(mem, nil)
}

// keyRow reads a half row of the keyboard through the port, as a program does.
func keyRow(io *SpectrumIO, row uint) uint8 {
	return io.ReadPort(uint16(0xFFFE)&^(1<<(row+8))) & 0x1F
}

func TestJoystickKeysAndHostKeysDoNotReleaseEachOther(t *testing.T) {
	// The GUI's keyboard handler clears the matrix every frame and presses what the host
	// has down. A Sinclair joystick's keys must still be there afterwards, and a host key
	// must not be released by a joystick that is not pressing it.
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	io.SetJoystickMode(JoystickSinclair1)
	io.SetJoystickState(JoystickState{Fire: true}) // key 0: row 4, bit 0
	for i := range io.keyboard {
		io.keyboard[i] = 0x1F // the keyboard handler's clear
	}
	io.PressKey(4, 4) // the host holds 6
	if got := keyRow(io, 4); got != 0x1F&^0x11 {
		t.Fatalf("row 6-0 reads %05b, want keys 6 (host) and 0 (joystick fire) both down", got)
	}
	io.SetJoystickState(JoystickState{}) // the joystick lets go
	if got := keyRow(io, 4); got != 0x1F&^0x10 {
		t.Fatalf("row 6-0 reads %05b after the joystick let go, want the host's 6 still down", got)
	}
}

func TestCursorJoystick(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	io.SetJoystickMode(JoystickCursor)
	for _, c := range []struct {
		s        JoystickState
		row      uint
		bit      uint8
		name     string
		otherRow uint
	}{
		{JoystickState{Left: true}, 3, 4, "left is 5", 4},
		{JoystickState{Down: true}, 4, 4, "down is 6", 3},
		{JoystickState{Up: true}, 4, 3, "up is 7", 3},
		{JoystickState{Right: true}, 4, 2, "right is 8", 3},
		{JoystickState{Fire: true}, 4, 0, "fire is 0", 3},
	} {
		io.SetJoystickState(c.s)
		if got := keyRow(io, c.row); got != 0x1F&^(1<<c.bit) {
			t.Errorf("%s: its row reads %05b, want only bit %d down", c.name, got, c.bit)
		}
		if got := keyRow(io, c.otherRow); got != 0x1F {
			t.Errorf("%s: the other number row reads %05b, want nothing down", c.name, got)
		}
	}
}

func TestNextJoystickFollowsTheRegister(t *testing.T) {
	io := newNextIO()
	io.SetJoystickMode(JoystickNext)
	left := JoystickState{Right: true, Fire: true, Fire2: true, A: true, Start: true}
	right := JoystickState{Left: true, Fire: true}
	reg := func(leftMode, rightMode uint8) uint8 {
		return leftMode&3<<6 | leftMode&4<<1 | rightMode&3<<4 | rightMode&4>>1
	}

	// As the firmware sets it from config.ini: Kempston 1 on the left, Sinclair 6-0 on
	// the right.
	io.nextRegWriteDirect(0x05, reg(1, 3))
	io.SetJoystickStateBoth(left, right)
	if got := io.ReadPort(0x001F); got != 0x31 {
		t.Errorf("Kempston 1 on the left port: port 0x1F reads 0x%02X, want 0x31 (right, fire, second fire; no A or START on a plain Kempston)", got)
	}
	if got := keyRow(io, 4); got != 0x1F&^0x11 {
		t.Errorf("Sinclair 6-0 on the right port: the row reads %05b, want 6 (left) and 0 (fire) down", got)
	}
	if got := io.ReadPort(0x0037); got != 0 {
		t.Errorf("port 0x37 reads 0x%02X with nothing on it", got)
	}

	// A program asks for an MD pad on the left and Kempston 2 on the right.
	io.nextRegWriteDirect(0x05, reg(5, 4))
	io.SetJoystickStateBoth(left, right)
	if got := io.ReadPort(0x001F); got != 0xF1 {
		t.Errorf("MD pad on the left port: port 0x1F reads 0x%02X, want 0xF1 (right, B, C, A, START)", got)
	}
	if got := io.ReadPort(0x0037); got != 0x12 {
		t.Errorf("Kempston 2 on the right port: port 0x37 reads 0x%02X, want 0x12", got)
	}
	if got := keyRow(io, 4); got != 0x1F {
		t.Errorf("no keyboard joystick is chosen and the row reads %05b", got)
	}

	// Cursor on the left, Sinclair 1-5 on the right.
	io.nextRegWriteDirect(0x05, reg(2, 0))
	io.SetJoystickStateBoth(left, right)
	if got := keyRow(io, 4); got != 0x1F&^0x05 {
		t.Errorf("Cursor on the left: row 6-0 reads %05b, want 8 (right) and 0 (fire) down", got)
	}
	if got := keyRow(io, 3); got != 0x1F&^0x11 {
		t.Errorf("Sinclair 1-5 on the right: row 1-5 reads %05b, want 1 (left) and 5 (fire) down", got)
	}
	if io.ReadPort(0x001F) != 0 {
		t.Error("port 0x1F still reads buttons after the left port stopped being a Kempston one")
	}

	// The other bits of the register are not the joysticks'.
	io.nextRegWriteDirect(0x05, reg(1, 1)|0x05)
	io.SetJoystickStateBoth(JoystickState{Up: true}, JoystickState{Down: true})
	if got := io.ReadPort(0x001F); got != 0x0C {
		t.Errorf("both ports Kempston 1: port 0x1F reads 0x%02X, want both pads joined, 0x0C", got)
	}
}

func TestClassicKempstonHasNoExtraButtons(t *testing.T) {
	mem, _ := newTestMemoryAndScreen()
	io := NewSpectrumIO(mem, nil)
	io.SetJoystickMode(JoystickKempston)
	io.SetJoystickState(JoystickState{Fire: true, Fire2: true, A: true, Start: true})
	if got := io.ReadPort(0x001F); got != 0x10 {
		t.Errorf("a classic Kempston port reads 0x%02X, want 0x10: bits 5 to 7 stay 0", got)
	}
}

func TestDefaultsForTheNext(t *testing.T) {
	if got := defaultJoystickModeForModel("next"); got != JoystickNext {
		t.Errorf("a Next's joystick by default is %v, want the machine's own choice", got)
	}
	for _, c := range []struct {
		flag, model string
		want        MouseMode
		wantErr     bool
	}{
		{"auto", "next", MouseKempston, false},
		{"auto", "48k", MouseNone, false},
		{"auto", "plus3", MouseNone, false},
		{"none", "next", MouseNone, false},
		{"kempston", "48k", MouseKempston, false},
		{"amx", "48k", MouseAMX, false},
		{"amx", "next", MouseNone, true},
	} {
		got, err := resolveMouseMode(c.flag, c.model)
		if (err != nil) != c.wantErr || (!c.wantErr && got != c.want) {
			t.Errorf("-mouse %s -model %s: %v, error %v; want %v, error %v", c.flag, c.model, got, err, c.want, c.wantErr)
		}
	}
}

func TestNextMousePort(t *testing.T) {
	io := newNextIO()
	io.SetMouseMode(MouseKempston)
	if got := io.ReadPort(0xFADF); got != 0x07 {
		t.Errorf("buttons before the mouse is touched: 0x%02X, want 0x07 as in CSpect", got)
	}
	io.SetMouseState(MouseState{Left: true})
	if got := io.ReadPort(0xFADF); got != 0x05 {
		t.Errorf("left button: 0x%02X, want 0x05", got)
	}
	io.SetMouseState(MouseState{Right: true})
	if got := io.ReadPort(0xFADF); got != 0x06 {
		t.Errorf("right button: 0x%02X, want 0x06", got)
	}
	io.SetMouseState(MouseState{Middle: true, Wheel: 3})
	if got := io.ReadPort(0xFADF); got != 0x33 {
		t.Errorf("middle button and three notches of the wheel: 0x%02X, want 0x33", got)
	}
	io.SetMouseState(MouseState{Wheel: -4})
	if got := io.ReadPort(0xFADF); got != 0xF7 {
		t.Errorf("four notches back: 0x%02X, want 0xF7: the wheel wraps in its four bits", got)
	}
	io.nextRegWriteDirect(0x0A, io.nextRegs.regs[0x0A]|0x08) // buttons reversed
	io.SetMouseState(MouseState{Left: true})
	if got := io.ReadPort(0xFADF) & 0x07; got != 0x06 {
		t.Errorf("left button with NextREG 0x0A bit 3 set: 0x%02X, want the right button's bit", got)
	}
	// 40 right and 30 down: X goes up, Y goes down, as in CSpect.
	io.SetMouseState(MouseState{DeltaX: 40, DeltaY: 30})
	if x, y := io.ReadPort(0xFBDF), io.ReadPort(0xFFDF); x != 40 || y != 226 {
		t.Errorf("after 40 right and 30 down the counters read %d, %d; want 40, 226", x, y)
	}
}
