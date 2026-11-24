//go:build headless

package zenzx

import "testing"

// A Next in its own interrupt mode, with a handler for each of three sources that counts
// how often it runs: the frame (ULA), the line interrupt and the first CTC channel.
func TestNextHardwareInterruptMode(t *testing.T) {
	zx := lineIntMachine(t)
	poke := func(addr uint16, bytes ...uint8) {
		for i, b := range bytes {
			zx.memory.Write(addr+uint16(i), b)
		}
	}
	counter := func(at uint16, count uint16) { // push af; ld a,(count); inc a; ld (count),a; pop af; ei; ret
		poke(at, 0xF5, 0x3A, uint8(count), uint8(count>>8), 0x3C, 0x32, uint8(count), uint8(count>>8), 0xF1, 0xFB, 0xC9)
	}
	counter(0xA000, 0x9010) // the frame
	counter(0xA020, 0x9011) // the line
	counter(0xA040, 0x9012) // CTC channel 0
	// The vectors with NextREG 0xC0 = 0xA1: 0xA0 for the line, 0xA6 for CTC 0, 0xB6 for the ULA.
	poke(0xBEA0, 0x20, 0xA0)
	poke(0xBEA6, 0x40, 0xA0)
	poke(0xBEB6, 0x00, 0xA0)
	poke(0x9010, 0, 0, 0)
	lineIntLog(zx, 2) // the program sets itself up: IM 2, I = 0xBE

	zx.io.nextRegWriteDirect(0xC0, 0xA1)
	zx.io.nextRegWriteDirect(0x23, 100)
	zx.io.nextRegWriteDirect(0x22, 0x02)
	// CTC 0: a timer, prescaler 256, time constant 256: every 65536 clocks, 8192 T-states.
	zx.io.WritePort(0x183B, 0x25)
	zx.io.WritePort(0x183B, 0x00)
	zx.io.nextRegWriteDirect(0xC5, 0x01)
	for i := 0; i < 10; i++ {
		zx.RunFrame()
	}
	frame, line, ctc := zx.memory.Read(0x9010), zx.memory.Read(0x9011), zx.memory.Read(0x9012)
	if frame != 10 || line != 10 {
		t.Errorf("in ten frames the frame's handler ran %d times and the line's %d, want 10 and 10", frame, line)
	}
	if want := 10 * NextFrameCycles / 8192; int(ctc) < want-1 || int(ctc) > want+1 {
		t.Errorf("in ten frames the CTC's handler ran %d times, want about %d: one every 8192 T-states", ctc, want)
	}
	// The status registers have the sources that interrupted; a 1 clears a bit.
	zx.io.nextRegs.selected = 0xC9
	if got := zx.io.nextRegReadSelected(); got&0x01 == 0 {
		t.Errorf("NextREG 0xC9 reads 0x%02X after the CTC has interrupted, want bit 0", got)
	}
	zx.io.nextRegWriteDirect(0xC9, 0x01)
	if zx.io.nextRegs.regs[0xC9]&0x01 != 0 {
		t.Error("writing 1 to NextREG 0xC9 bit 0 did not clear it")
	}

	// With the CTC's interrupt disabled its handler stops running; the count goes on.
	zx.io.nextRegWriteDirect(0xC5, 0x00)
	before := zx.memory.Read(0x9012)
	for i := 0; i < 3; i++ {
		zx.RunFrame()
	}
	if got := zx.memory.Read(0x9012); got != before {
		t.Errorf("with NextREG 0xC5 clear the CTC's handler still ran %d times", got-before)
	}
	// NextREG 0xC4 and 0x22 are the same two enables.
	zx.io.nextRegWriteDirect(0xC4, 0x80) // neither the ULA nor the line
	if zx.io.nextRegs.regs[0x22]&0x06 != 0x04 {
		t.Errorf("after NextREG 0xC4 = 0x80, NextREG 0x22 is 0x%02X: want the ULA's interrupt off and the line's off", zx.io.nextRegs.regs[0x22])
	}
	zx.io.nextRegs.selected = 0xC4
	zx.io.nextRegWriteDirect(0x22, 0x02)
	if got := zx.io.nextRegReadSelected(); got != 0x83 {
		t.Errorf("NextREG 0xC4 reads 0x%02X after NextREG 0x22 = 0x02, want 0x83", got)
	}
	// Back in the ordinary mode nothing is left waiting.
	zx.io.nextRegWriteDirect(0xC0, 0x00)
	if zx.io.nextInt.pending != 0 || zx.io.nextInt.hw {
		t.Error("leaving the hardware mode left interrupts waiting")
	}
}

func TestCTCCounterCountsTheChannelBefore(t *testing.T) {
	io := newNextIO()
	io.WritePort(0x183B, 0x05) // channel 0: timer, prescaler 16, time constant follows
	io.WritePort(0x183B, 10)   // every 160 clocks
	io.WritePort(0x193B, 0x45) // channel 1: counter
	io.WritePort(0x193B, 3)    // every third pulse of channel 0
	io.ctcTick(160 * 2)
	if got := io.ReadPort(0x193B); got != 1 {
		t.Errorf("after two pulses of channel 0, channel 1's count is %d, want 1", got)
	}
	io.ctcTick(160)
	if got := io.ReadPort(0x193B); got != 3 || io.nextRegs.regs[0xC9]&0x02 == 0 {
		t.Errorf("after the third, channel 1's count is %d and status 0x%02X; want 3 again and its status bit", got, io.nextRegs.regs[0xC9])
	}
	if got := io.ReadPort(0x183B); got != 10 {
		t.Errorf("channel 0's count reads %d at the start of a period, want 10", got)
	}
	// A reset stops a channel until it is given a time constant.
	io.WritePort(0x183B, 0x03)
	io.ctcTick(1600)
	if got := io.ReadPort(0x183B); got != 10 {
		t.Errorf("a reset channel counted: %d", got)
	}
}
