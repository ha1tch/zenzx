//go:build headless

package zenzx

import "testing"

// lineIntMachine is a Next running a program that does nothing but take interrupts: its
// handler (IM 2) appends the low byte of the video line (NextREG 0x1F) to a list at
// 0x9100, so the test can read off where in the frame each interrupt came. The handler
// reads the line some 115 T-states after the interrupt is raised: the line interrupt for
// line y comes 100 T-states before the end of line y-1, so what it reads is y.
func lineIntMachine(t *testing.T) *ZenZX {
	t.Helper()
	zx := NewZenZX(AudioBackendOto)
	if err := loadModelROM(zx, "next"); err != nil {
		t.Fatal(err)
	}
	zx.cpu.Reset()
	zx.running = true
	poke := func(addr uint16, bytes ...uint8) {
		for i, b := range bytes {
			zx.memory.Write(addr+uint16(i), b)
		}
	}
	for i := 0; i <= 256; i++ { // the IM 2 table: every vector is 0xBFBF
		poke(0xBE00+uint16(i), 0xBF)
	}
	poke(0x9000, 0x00, 0x91) // where the next entry goes
	poke(0xBFBF,
		0xF5,             // push af
		0xE5,             // push hl
		0xC5,             // push bc
		0x2A, 0x00, 0x90, // ld hl,(0x9000)
		0x3E, 0x1F, // ld a,0x1F
		0x01, 0x3B, 0x24, // ld bc,0x243B
		0xED, 0x79, // out (c),a
		0x04,       // inc b
		0xED, 0x78, // in a,(c)
		0x77,             // ld (hl),a
		0x23,             // inc hl
		0x22, 0x00, 0x90, // ld (0x9000),hl
		0xC1, // pop bc
		0xE1, // pop hl
		0xF1, // pop af
		0xFB, // ei
		0xC9, // ret
	)
	poke(0x8000,
		0xF3,             // di
		0x31, 0x00, 0xBE, // ld sp,0xBE00
		0x3E, 0xBE, // ld a,0xBE
		0xED, 0x47, // ld i,a
		0xED, 0x5E, // im 2
		0xFB,       // ei
		0x76,       // halt
		0x18, 0xFD, // jr to the halt
	)
	zx.cpu.PC = 0x8000
	return zx
}

// lineIntLog runs frames and returns the lines the handler recorded in them.
func lineIntLog(zx *ZenZX, frames int) []uint8 {
	start := uint16(zx.memory.Read(0x9000)) | uint16(zx.memory.Read(0x9001))<<8
	for i := 0; i < frames; i++ {
		zx.RunFrame()
	}
	end := uint16(zx.memory.Read(0x9000)) | uint16(zx.memory.Read(0x9001))<<8
	var log []uint8
	for a := start; a < end; a++ {
		log = append(log, zx.memory.Read(a))
	}
	return log
}

func TestNextLineInterrupt(t *testing.T) {
	zx := lineIntMachine(t)
	lineIntLog(zx, 2) // the program sets itself up
	if tm := zx.rasterTiming(); tm.lines != 311 || tm.tPerLine != 228 {
		t.Fatalf("a Next's frame is %d lines of %d T-states, want 311 of 228", tm.lines, tm.tPerLine)
	}
	frameLine := uint8(zx.rasterTiming().lines - zx.rasterTiming().firstPixelLine)

	// Without the line interrupt: one interrupt a frame, at the frame's start.
	if log := lineIntLog(zx, 3); len(log) != 3 || log[0] != frameLine || log[2] != frameLine {
		t.Fatalf("frame interrupt only: the handler ran on lines %v, want three times on line %d", log, frameLine)
	}

	// With it, for line 100: a second interrupt, raised on line 99 after the pixel area,
	// whose handler is reading the line as line 100 begins.
	zx.io.nextRegWriteDirect(0x23, 100)
	zx.io.nextRegWriteDirect(0x22, 0x02)
	log := lineIntLog(zx, 3)
	if len(log) != 6 {
		t.Fatalf("line interrupt for line 100: the handler ran on lines %v, want two interrupts a frame for three frames", log)
	}
	for i := 0; i < 6; i += 2 {
		if log[i] != frameLine || log[i+1] != 100 {
			t.Fatalf("line interrupt for line 100: the handler read lines %v, want line %d then line 100 in each frame", log, frameLine)
		}
	}
	tm := zx.rasterTiming()
	if want := (tm.firstPixelLine+99)*tm.tPerLine + 128; zx.io.nextLineInt.at != want {
		t.Errorf("the interrupt for line 100 is raised at T-state %d of the frame, want %d: line 99, 256 pixels across", zx.io.nextLineInt.at, want)
	}

	// NextREG 0x64 moves the count: line 100 is then 32 raster lines earlier.
	zx.io.nextRegWriteDirect(0x64, 32)
	if want := (tm.firstPixelLine+99-32)*tm.tPerLine + 128; zx.io.nextLineInt.at != want {
		t.Errorf("with NextREG 0x64 = 32 the interrupt is raised at T-state %d, want %d", zx.io.nextLineInt.at, want)
	}
	zx.io.nextRegWriteDirect(0x64, 0)

	// Bit 2 turns the frame interrupt off: the line interrupt is the only one.
	zx.io.nextRegWriteDirect(0x22, 0x06)
	if log := lineIntLog(zx, 3); len(log) != 3 || log[0] != 100 || log[1] != 100 || log[2] != 100 {
		t.Fatalf("frame interrupt off: the handler read lines %v, want once a frame, line 100", log)
	}

	// Line 0's interrupt is raised on the last line before the pixel area.
	zx.io.nextRegWriteDirect(0x23, 0)
	if log := lineIntLog(zx, 2); len(log) != 2 || log[0] != 0 {
		t.Fatalf("line interrupt for line 0: the handler read lines %v, want line 0", log)
	}
	if want := (tm.firstPixelLine-1)*tm.tPerLine + 128; zx.io.nextLineInt.at != want {
		t.Errorf("the interrupt for line 0 is raised at T-state %d of the frame, want %d: the line before the pixel area", zx.io.nextLineInt.at, want)
	}

	// A line the frame does not have gives no interrupt, and off is off.
	zx.io.nextRegWriteDirect(0x23, 0xFF)
	zx.io.nextRegWriteDirect(0x22, 0x07) // line 511
	if log := lineIntLog(zx, 2); len(log) != 0 {
		t.Fatalf("line 511 with the frame interrupt off: the handler ran on lines %v, want no interrupt", log)
	}
	zx.io.nextRegWriteDirect(0x22, 0x00)
	if log := lineIntLog(zx, 2); len(log) != 2 {
		t.Fatalf("everything back as it was: the handler ran on lines %v, want the frame interrupt alone", log)
	}
}

func TestNextLineInterruptSetAgainInTheSameFrame(t *testing.T) {
	// The handler's own trick, done here by hand: once the interrupt for one line has
	// come, set the line further down and get another in the same frame.
	zx := lineIntMachine(t)
	lineIntLog(zx, 2)
	zx.io.nextRegWriteDirect(0x23, 50)
	zx.io.nextRegWriteDirect(0x22, 0x06)
	zx.io.StartFrame()
	zx.frameOrigin = zx.cpu.Cycles
	li := &zx.io.nextLineInt
	if li.raised {
		t.Fatal("a new frame starts with the line interrupt already raised")
	}
	zx.cpu.Cycles += uint64(li.at + 10) // the beam is past line 49
	zx.lineInterrupt(li.at + 10)
	if !li.raised || !zx.cpu.INT {
		t.Fatal("the interrupt was not raised when the frame reached its place")
	}
	zx.io.nextRegWriteDirect(0x23, 150)
	if li.raised {
		t.Error("setting the line further down the frame did not arm the interrupt again")
	}
	zx.io.nextRegWriteDirect(0x23, 20)
	if !li.raised {
		t.Error("setting a line the beam has passed armed the interrupt for this frame")
	}
}

func TestNextLineInterruptRegisterReadsBack(t *testing.T) {
	zx := lineIntMachine(t)
	zx.io.nextRegWriteDirect(0x22, 0x03)
	zx.io.nextRegs.selected = 0x22
	zx.frameOrigin = zx.cpu.Cycles // the frame has just started: the ULA's interrupt is up
	if got := zx.io.nextRegReadSelected(); got != 0x83 {
		t.Errorf("NextREG 0x22 reads 0x%02X at the frame's start, want 0x83: bit 7 while the ULA asserts its interrupt", got)
	}
	zx.cpu.Cycles += 1000
	if got := zx.io.nextRegReadSelected(); got != 0x03 {
		t.Errorf("NextREG 0x22 reads 0x%02X later in the frame, want 0x03", got)
	}
}
