package main

import (
	"fmt"
	"strings"
)

// ============================================================================
// Mouse emulation: Kempston Mouse and AMX Mouse
//
// Kempston Mouse: three ports, confirmed against four independent sources
// (gamedev.net, zxpress.ru, 8bit.yarek.pl, and the ZX_BUS_Mouse hardware
// clone project on GitHub, all agreeing exactly) --
//
//	0xFADF  buttons  bit0=right, bit1=left, bit2=middle
//	0xFBDF  X        8-bit wrapping counter
//	0xFFDF  Y        8-bit wrapping counter
//
// Critically, X/Y are RELATIVE, not absolute screen coordinates -- the
// interface accumulates movement with two 74LS191 up/down counters and
// simply wraps at the byte boundary; software tracks a cursor position
// itself and re-centres its own accounting when it hits a screen edge
// (k1.spdns.de's Kempston Mouse Interface page states this outright).
//
// Button polarity (active low, unused bits reporting high) is confirmed
// for the CPC-bus variant of the same Kempston Mouse interface family
// (cpcwiki.eu's Kempston Mouse page gives the bit table explicitly) --
// not separately re-confirmed against a ZX-specific source for polarity
// specifically, though the port/bit *assignment* is independently
// confirmed for the ZX interface by the four sources above.
//
// AMX Mouse: confirmed directly from a genuine AMX Art tape (session
// work, docs/TRACKING.md T-14) -- disassembled with the z80dis package,
// cross-checked between the dedicated AMX driver block and ART 1.1's own
// inline copy of the same routines, which agree exactly.
//
// X/Y are interrupt-driven, one CPU interrupt per step of movement, NOT
// software-side quadrature phase-decoding -- the Z80 PIO hardware does
// the edge-detection and debounce, and delivers a ready-made direction
// bit per interrupt:
//
//	IM 2; I=0xE9; vector 0xD0 -> table entry 0xE9D0 -> Y handler,
//	reads port 0x3F once, bit 0 = direction, +/-1 to a position counter.
//	vector 0xD2 -> table entry 0xE9D2 -> X handler, port 0x1F, same
//	pattern.
//
// Buttons are polled, not interrupt-driven, from port 0xDF -- bits 5, 6,
// 7 (a different layout from Kempston Mouse's bits 0-2 on a different
// port; a genuinely different device), read directly whenever the guest
// asks, no debounce needed on the emulation side even though AMX Art's
// own code retries a few times waiting for the bit (real hardware
// debounce that a synthetic level read doesn't need to replicate).
//
// Port 0x1F collides with Kempston Joystick on real hardware (both
// hardware families use it) -- ZenZX wiring rejects selecting AMX mouse
// and Kempston joystick (or kempston-both, which also uses 0x1F for its
// first sub-port) simultaneously at startup.
//
// Kempston Mouse itself is a genuinely different question, investigated
// directly rather than assumed: it does NOT collide with either
// Kempston joystick port. It's a separate, dedicated physical interface
// with its own ports (0xFADF/0xFBDF/0xFFDF -- confirmed by a first-hand
// account on Spectrum Computing's forums from someone who owned one:
// "a custom interface" recording X/Y as 8-bit values "accessed via a
// port, and another that shows the buttons," not something that plugs
// into a joystick port at all), and none of those addresses' low bytes
// (0xDF) collide with 0x1F (Kempston 1) or 0x37 (Kempston 2). The ZX-
// VGA-JOY interface's own hardware design notes name this precisely:
// some real, incompletely-decoded 1980s Kempston joystick interfaces
// (checking only a few address bits, not the full byte) could
// accidentally collide with a Kempston Mouse plugged in alongside one --
// "Best is use full 8-bit address decoding 00011111" to avoid it. zenzx
// already does exactly that for every port here (io.go), so two
// Kempston joysticks plus a Kempston mouse coexist correctly and
// independently -- proved, not just assumed, by
// TestKempstonMouseCoexistsWithDualKempstonJoysticks in joystick_test.go.
// ============================================================================

// MouseMode selects which hardware mouse interface, if any, is emulated.
type MouseMode int

const (
	MouseNone MouseMode = iota
	MouseKempston
	MouseAMX
)

// ParseMouseMode validates a -mouse flag value.
func ParseMouseMode(s string) (MouseMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none":
		return MouseNone, nil
	case "kempston":
		return MouseKempston, nil
	case "amx":
		return MouseAMX, nil
	default:
		return MouseNone, fmt.Errorf("unknown -mouse mode %q: valid values are none, kempston, amx", s)
	}
}

func (m MouseMode) String() string {
	switch m {
	case MouseKempston:
		return "kempston"
	case MouseAMX:
		return "amx"
	default:
		return "none"
	}
}

// AMX interrupt vectors, confirmed directly from disassembly (see package
// doc above) -- these are the low bytes AMX-aware software's own PIO
// control-word writes program into the IM2 vector table, not something
// zenzx invents or negotiates.
const (
	amxVectorY uint8 = 0xD0
	amxVectorX uint8 = 0xD2
)

// MouseState is the abstract, hardware-independent per-frame mouse state.
// DeltaX/DeltaY are movement since the last frame in native Spectrum
// pixels -- the caller (input.go, GUI-only) is responsible for converting
// from host window-client pixels by dividing by the current display
// magnification before constructing this; mouse.go has no notion of
// window scale at all, deliberately, the same separation joystick.go
// keeps from its own host input source.
type MouseState struct {
	DeltaX, DeltaY      float32
	Left, Right, Middle bool
}

// kempstonMouseButtonByte renders a MouseState's buttons as the byte port
// 0xFADF would return: active low, unused bits high.
func kempstonMouseButtonByte(s MouseState) uint8 {
	b := uint8(0xFF)
	if s.Right {
		b &^= 0x01
	}
	if s.Left {
		b &^= 0x02
	}
	if s.Middle {
		b &^= 0x04
	}
	return b
}

// amxMouseButtonByte renders a MouseState's buttons as port 0xDF would
// return under AMX Mouse: bits 5/6/7, active high (per the disassembly:
// AMX Art polls each bit waiting for it to become 1). Which physical
// button is bit 5 vs 6 vs 7 is not distinguished by the disassembly
// beyond "three separate bits" -- mapped here as Left=7, Right=6,
// Middle=5, an arbitrary but internally consistent choice pending a
// source that pins down which is which.
func amxMouseButtonByte(s MouseState) uint8 {
	var b uint8
	if s.Left {
		b |= 0x80
	}
	if s.Right {
		b |= 0x40
	}
	if s.Middle {
		b |= 0x20
	}
	return b
}

// SetMouseMode configures which hardware interface, if any, future
// SetMouseState calls are translated into.
func (io *SpectrumIO) SetMouseMode(mode MouseMode) {
	io.mouseMode = mode
}

// SetMouseState updates the mouse's abstract state and applies it to
// whichever hardware mechanism is configured. Safe to call with
// MouseNone configured -- a no-op past the mode switch.
//
// Fractional deltas are accumulated across frames (mouseRemX/Y) rather
// than truncated per-frame: at high magnification a single frame's
// magnification-corrected delta is often well under 1.0, and truncating
// it to zero every frame would silently discard slow mouse movement
// entirely rather than merely making it coarse.
func (io *SpectrumIO) SetMouseState(s MouseState) {
	switch io.mouseMode {
	case MouseKempston:
		io.mouseRemX += s.DeltaX
		io.mouseRemY += s.DeltaY
		dx := int32(io.mouseRemX) // truncates toward zero
		dy := int32(io.mouseRemY)
		io.mouseRemX -= float32(dx)
		io.mouseRemY -= float32(dy)
		// uint8(negative int32) wraps via two's-complement reinterpretation
		// per Go's defined conversion semantics -- exactly the up/down
		// counter wraparound the real 74LS191 hardware performs.
		io.kempstonMouseX += uint8(dx)
		io.kempstonMouseY += uint8(dy)
		io.kempstonMouseButtons = kempstonMouseButtonByte(s)

	case MouseAMX:
		io.amxMouseButtons = amxMouseButtonByte(s)
		io.mouseRemX += s.DeltaX
		io.mouseRemY += s.DeltaY
		// AMX is step-interrupt-driven, not a position register: queue
		// one interrupt per whole step of accumulated movement, in
		// whichever direction, rather than updating a readable counter.
		// The guest's own ISR (real code, running unmodified -- see
		// package doc) does the +/-1 accounting on its side once each
		// interrupt is delivered.
		for io.mouseRemX >= 1 {
			io.mouseRemX -= 1
			io.queueAMXInterrupt(amxVectorX, 0) // bit0=0: one direction
		}
		for io.mouseRemX <= -1 {
			io.mouseRemX += 1
			io.queueAMXInterrupt(amxVectorX, 1) // bit0=1: the other
		}
		for io.mouseRemY >= 1 {
			io.mouseRemY -= 1
			io.queueAMXInterrupt(amxVectorY, 0)
		}
		for io.mouseRemY <= -1 {
			io.mouseRemY += 1
			io.queueAMXInterrupt(amxVectorY, 1)
		}
	}
}

// amxInterruptRequest is one queued AMX step: which vector to deliver
// (X or Y's handler) and what byte the port read inside that handler
// should return (bit0 = direction, matching the disassembled handlers'
// own `AND 1` test).
type amxInterruptRequest struct {
	vector    uint8
	portValue uint8
}

// amxQueueCap bounds the pending-step queue so a stalled guest
// (interrupts disabled for a long stretch, or a very fast mouse flick)
// can't grow it without limit -- real hardware would simply lose events
// past what a human could physically generate in one frame; dropping the
// oldest is a reasonable approximation of that.
const amxQueueCap = 64

func (io *SpectrumIO) queueAMXInterrupt(vector, portValue uint8) {
	if len(io.amxIntQueue) >= amxQueueCap {
		io.amxIntQueue = io.amxIntQueue[1:]
	}
	io.amxIntQueue = append(io.amxIntQueue, amxInterruptRequest{vector: vector, portValue: portValue})
}

// PopAMXInterrupt is called from ZenZX's per-instruction run loop (only
// when AMX mouse mode is active) to check for and consume the next
// queued step. Returns ok=false when nothing is pending. The popped
// request's port value becomes what port 0x1F/0x3F returns until the
// next request is popped (io.go's port dispatch reads amxPortValue).
func (io *SpectrumIO) PopAMXInterrupt() (vector uint8, ok bool) {
	if len(io.amxIntQueue) == 0 {
		return 0, false
	}
	req := io.amxIntQueue[0]
	io.amxIntQueue = io.amxIntQueue[1:]
	io.amxPortValue = req.portValue
	io.amxPendingVector = req.vector
	io.amxVectorPending = true
	return req.vector, true
}

// GetInterruptVector implements zen80's optional InterruptController
// interface (z80.InterruptController), consulted for every IM2 interrupt
// acceptance, not just AMX's. Only AMX mouse ever needs anything but the
// standard Spectrum ULA's fixed 0xFF (zen80's own default when this
// interface isn't implemented at all) -- an AMX vector, when one is
// pending, takes priority; otherwise this returns exactly what zen80
// would have defaulted to on its own, so implementing this interface at
// all changes nothing for any software that isn't AMX-aware.
func (io *SpectrumIO) GetInterruptVector() uint8 {
	if io.amxVectorPending {
		io.amxVectorPending = false
		return io.amxPendingVector
	}
	return 0xFF
}

// GetMode0Instruction implements the other half of the same optional
// interface. Nothing in this project uses IM0; a NOP is a harmless,
// universal default so the interface can be satisfied at all.
func (io *SpectrumIO) GetMode0Instruction() []uint8 {
	return []uint8{0x00}
}
