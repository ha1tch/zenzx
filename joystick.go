package main

import (
	"fmt"
	"strings"
)

// ============================================================================
// Joystick emulation: Kempston and Sinclair Interface 2
//
// Two entirely different hardware mechanisms, both real, both available on
// the canonical Spectrum family -- independent of TS2068's own, unrelated
// AY-register-14 mechanism (ts2068.go).
//
// Kempston: a dedicated port, 0x1F (IN 31), active HIGH. Bit layout
// confirmed against the original Kempston Joystick Interface instruction
// sheet: bit0=right, bit1=left, bit2=down, bit3=up, bit4=fire, bits 5-7
// unused (always 0). Not a keyboard mechanism at all -- reading it never
// touches the keyboard matrix. The classic-era Kempston Interface (Kempston
// Micro Electronics' own product, what the vast majority of 1980s
// "Kempston-compatible" software checks for) was always genuinely
// single-port -- confirmed via its own Wikipedia article: "a single
// joystick port." Never built into any stock Sinclair/Amstrad model --
// always a third-party add-on interface, on every canonical model
// including the ones that came with their own built-in Sinclair ports
// (see below); still a legitimate explicit -joystick choice for any of
// them.
//
// A genuine second, independent Kempston-protocol port (JoystickKempston2/
// JoystickKempstonBoth) exists on modern "neo-Spectrum" platforms -- ZX-Uno,
// ZX-Tres, the Omni, and the ZX Spectrum Next (originally codenamed
// TBBlue) -- not on any classic-era hardware. Confirmed directly against
// the Next's own official I/O port register documentation
// (specnext.com/tbblue-io-port-system): register 0x05 selects "Kempston 1
// (port 0x1F)" and "Kempston 2 (port 0x37)" independently per port, and
// 0x37 cross-confirms exactly against a separate, independent source --
// the modern "KEMPSTON_MAX 2" hobbyist interface's own documented
// "Kempston Port 55" (55 decimal = 0x37 hex). Deliberately not implied by
// any -model's auto default, unlike JoystickSinclairBoth: no stock
// Sinclair/Amstrad/Timex machine ever had this built in. Represents an
// explicit choice to emulate a modern FPGA-reimplementation platform's
// own enhancement -- the same category of thing -ns-graphics/-ns-storage
// exist for.
//
// Sinclair (Interface 2): not a port at all. Each direction and fire
// simply shorts the same keyboard matrix contacts as a physical key
// would. Confirmed against multiple independent sources (Wikipedia's ZX
// Interface 2 article, the Interface 2 circuitry reference, libretro's
// Fuse core docs, and sharedmemorydump.net's own +2/+3 port testing):
// Joystick 1 mirrors keys 6-0 as 6=left, 7=right, 8=down, 9=up, 0=fire;
// Joystick 2 mirrors keys 1-5 as 1=left, 2=right, 3=down, 4=up, 5=fire.
// Worth flagging rather than silently resolving: the ZX Spectrum Next's
// own register documentation labels this the other way round ("Sinclair 1
// (12345)", "Sinclair 2 (67890)") -- the numbering used here follows
// Wikipedia's explicit statement plus the Interface 2's own physical
// hardware detail (its two sockets are literally labelled, with
// Joystick 1's identified by a dimple -- ZX Interface 2 circuitry
// reference), which is independent of any one board's own internal
// firmware-port-labelling choice.
//
// Built-in vs add-on, confirmed against real hardware history (searched,
// not assumed) rather than treated as interchangeable -- see
// defaultJoystickModeForModel below for how -model uses this:
//   - 48K, and the *original* Sinclair 128K ("Toastrack", pre-Amstrad):
//     no built-in joystick port of any kind. Confirmed via
//     spectrumforeveryone.com's own +2 history: Sinclair-compatible
//     joystick ports were "a feature that really should have been part
//     of the 128 specification from the start" -- i.e. explicitly
//     absent from it. Kempston, Sinclair, and Cursor interfaces were all
//     equally third-party add-ons for these models.
//   - Amstrad's +2 (grey case) and +3 (and +2A, which shares the +3
//     board): two built-in Sinclair-protocol ports, badged "SJS" with a
//     proprietary DB9 pinout (Amstrad's own IC, rebadged from the
//     Interface 2's own MT62001) -- genuinely Sinclair Joystick 1 and 2
//     electrically, confirmed by spectrumforeveryone.com and
//     sharedmemorydump.net's own port testing ("Where a Kempston
//     joystick can be read from I/O-port 31... the Sinclair joystick
//     just emulates keyboard keys"). Connecting a real Kempston
//     interface to the +2A/+3's expansion bus at the same time risks
//     damaging the machine (ZX Interface 2 circuitry reference: the
//     +2A/+3's own port-0xFE read isn't open-collector) -- not a
//     concern in emulation, but the reason Kempston was never a
//     practical simultaneous option on these two models specifically.
//   - TS2068's own built-in ports (ts2068.go) are read via the
//     AY-3-8912's I/O port, not a dedicated Kempston-style port address
//     -- genuinely not protocol-compatible with either Kempston port
//     modelled here, confirmed against timexsinclair.com. "Kempston-
//     style" as a broad category label (a dedicated digital port read,
//     as opposed to Sinclair/Cursor's keyboard-remapping mechanism) is
//     a fair description of what TS2068 does architecturally, even
//     though the specific protocol differs from both ports here.
// ============================================================================

// JoystickMode selects which hardware joystick mechanism, if any, abstract
// JoystickState updates are translated into. The *Both modes are distinct
// from their single-port counterparts -- they drive two real ports
// simultaneously from two separate host inputs, where the single-port
// modes each drive one port from one.
type JoystickMode int

const (
	JoystickNone JoystickMode = iota
	JoystickKempston
	JoystickKempston2
	JoystickKempstonBoth
	JoystickSinclair1
	JoystickSinclair2
	JoystickSinclairBoth
)

// ParseJoystickMode validates a -joystick flag value. "sinclair" is kept
// as a backward-compatible alias for "sinclair1" (this flag's only value
// for Sinclair before this session's dual-port work). "kempston" remains
// the alias-free name for the classic single port (0x1F), matching every
// existing script/habit built around it. "auto" is handled by the caller
// (zenzx_headless.go/zenzx_gui.go), which resolves it against -model
// before ever reaching this function -- ParseJoystickMode itself has no
// notion of which model is running.
//
// Unlike the older -tapemode flag's silent-fallback style, an
// unrecognised value is a hard error -- consistent with the
// -ns-graphics/-non-standard validation established in nonstandard.go.
func ParseJoystickMode(s string) (JoystickMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none":
		return JoystickNone, nil
	case "kempston":
		return JoystickKempston, nil
	case "kempston2":
		return JoystickKempston2, nil
	case "kempston-both":
		return JoystickKempstonBoth, nil
	case "sinclair", "sinclair1":
		return JoystickSinclair1, nil
	case "sinclair2":
		return JoystickSinclair2, nil
	case "sinclair-both":
		return JoystickSinclairBoth, nil
	default:
		return JoystickNone, fmt.Errorf("unknown -joystick mode %q: valid values are none, kempston, kempston2, kempston-both, sinclair (alias for sinclair1), sinclair1, sinclair2, sinclair-both, auto", s)
	}
}

func (m JoystickMode) String() string {
	switch m {
	case JoystickKempston:
		return "kempston"
	case JoystickKempston2:
		return "kempston2"
	case JoystickKempstonBoth:
		return "kempston-both"
	case JoystickSinclair1:
		return "sinclair1"
	case JoystickSinclair2:
		return "sinclair2"
	case JoystickSinclairBoth:
		return "sinclair-both"
	default:
		return "none"
	}
}

// JoystickState is the abstract, hardware-independent state of one
// joystick for one frame. Diagonals and fire-while-moving are both valid
// (multiple fields true at once) -- real joysticks and all emulated
// mechanisms support this natively.
type JoystickState struct {
	Up, Down, Left, Right, Fire bool
}

// kempstonByte renders a JoystickState as the byte a Kempston-protocol
// port would return (0x1F for the first port, 0x37 for the second --
// see the package doc above). Bit layout confirmed against the original
// interface instruction sheet: bit0=right, bit1=left, bit2=down,
// bit3=up, bit4=fire, bits 5-7 unused. Both ports share the same byte
// encoding; only the port address differs.
func kempstonByte(s JoystickState) uint8 {
	var b uint8
	if s.Right {
		b |= 0x01
	}
	if s.Left {
		b |= 0x02
	}
	if s.Down {
		b |= 0x04
	}
	if s.Up {
		b |= 0x08
	}
	if s.Fire {
		b |= 0x10
	}
	return b
}

// sinclairKeyMatrix1 gives the row/col matrix position (SpectrumIO.
// PressKey/ReleaseKey convention) for each Sinclair Joystick 1 direction,
// matching input.go's own key-6..0 mapping exactly (row 4: 6,7,8,9,0 at
// columns 4,3,2,1,0 respectively).
var sinclairKeyMatrix1 = struct {
	Left, Right, Down, Up, Fire [2]uint8
}{
	Left:  [2]uint8{4, 4}, // key 6
	Right: [2]uint8{4, 3}, // key 7
	Down:  [2]uint8{4, 2}, // key 8
	Up:    [2]uint8{4, 1}, // key 9
	Fire:  [2]uint8{4, 0}, // key 0
}

// sinclairKeyMatrix2 gives the same for Sinclair Joystick 2, matching
// input.go's key-1..5 mapping (row 3: 1,2,3,4,5 at columns 0,1,2,3,4).
var sinclairKeyMatrix2 = struct {
	Left, Right, Down, Up, Fire [2]uint8
}{
	Left:  [2]uint8{3, 0}, // key 1
	Right: [2]uint8{3, 1}, // key 2
	Down:  [2]uint8{3, 2}, // key 3
	Up:    [2]uint8{3, 3}, // key 4
	Fire:  [2]uint8{3, 4}, // key 5
}

// applySinclairState presses/releases exactly the five matrix cells the
// given Sinclair port uses, per the current abstract state. Safe to call
// every frame regardless of what else is happening on the keyboard: these
// are ordinary PressKey/ReleaseKey calls on specific bits, ordinary
// keyboard use of the same physical keys composes correctly, exactly as
// it would on real hardware sharing the same contacts.
func applySinclairState(io *SpectrumIO, matrix struct{ Left, Right, Down, Up, Fire [2]uint8 }, s JoystickState) {
	setKey := func(pos [2]uint8, pressed bool) {
		if pressed {
			io.PressKey(pos[0], pos[1])
		} else {
			io.ReleaseKey(pos[0], pos[1])
		}
	}
	setKey(matrix.Left, s.Left)
	setKey(matrix.Right, s.Right)
	setKey(matrix.Down, s.Down)
	setKey(matrix.Up, s.Up)
	setKey(matrix.Fire, s.Fire)
}

// defaultJoystickModeForModel returns the joystick configuration -model
// implies when -joystick is left at "auto" -- the machine's own real,
// built-in hardware, not a guess. Verified against real hardware history
// (see the package doc above for citations), not assumed:
//   - 48K and the original Sinclair 128K ("Toastrack", pre-Amstrad): no
//     built-in joystick port of any kind.
//   - +2 (grey), +2A, +3 (and their Spanish variants): two built-in
//     Sinclair-protocol ports.
//   - TS2068: has its own, separate, always-on built-in joystick
//     mechanism (ts2068.go's handleTS2068JoystickInput, independent of
//     -joystick entirely) -- the canonical Spectrum Kempston/Sinclair
//     mechanism this function configures genuinely does not apply, so
//     the correct default here is None, not a guess at which one.
//
// Neither Kempston2 nor KempstonBoth is ever a -model default: no stock
// model ever had a second Kempston port built in (see the package doc)
// -- they remain purely explicit choices, for anyone specifically
// emulating a modern neo-Spectrum platform's own enhancement.
//
// Third-party interfaces (Kempston being the most common) remain a
// legitimate explicit -joystick override for any model, including ones
// with their own built-in ports -- exactly matching how real add-on
// interfaces worked historically (with the caveat, not relevant to
// emulation, that connecting one to a real +2A/+3 alongside its own
// built-in ports risked hardware damage; see the package doc).
func defaultJoystickModeForModel(model string) JoystickMode {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "plus2", "plus2a", "plus3", "spanishplus2", "spanishplus3":
		return JoystickSinclairBoth
	default:
		return JoystickNone
	}
}

// resolveJoystickMode handles the -joystick flag's "auto" value (model's
// own built-in configuration) or an explicit user override, against the
// given -model value. Shared by both zenzx_headless.go and zenzx_gui.go.
func resolveJoystickMode(joystickFlag, model string) (JoystickMode, error) {
	if strings.EqualFold(strings.TrimSpace(joystickFlag), "auto") {
		return defaultJoystickModeForModel(model), nil
	}
	return ParseJoystickMode(joystickFlag)
}

// SetJoystickMode configures which hardware mechanism, if any, future
// SetJoystickState/SetJoystickStateBoth calls are translated into.
func (io *SpectrumIO) SetJoystickMode(mode JoystickMode) {
	io.joystickMode = mode
}

// SetJoystickState updates the joystick's abstract state for
// single-target modes (None, Kempston, Kempston2, Sinclair1, Sinclair2)
// and applies it to whichever hardware mechanism is configured. A no-op
// for JoystickNone and for the two *Both modes (which need two states --
// see SetJoystickStateBoth).
func (io *SpectrumIO) SetJoystickState(s JoystickState) {
	switch io.joystickMode {
	case JoystickKempston:
		io.kempstonState = kempstonByte(s)
	case JoystickKempston2:
		io.kempston2State = kempstonByte(s)
	case JoystickSinclair1:
		applySinclairState(io, sinclairKeyMatrix1, s)
	case JoystickSinclair2:
		applySinclairState(io, sinclairKeyMatrix2, s)
	}
}

// SetJoystickStateBoth updates both ports at once for either dual-port
// mode -- the +2/+2A/+3's real built-in Sinclair ports, or a neo-Spectrum
// platform's configured dual Kempston ports -- from two independent
// states in the same call, matching the machine's two simultaneous
// physical ports. A no-op unless one of the two *Both modes is
// configured.
func (io *SpectrumIO) SetJoystickStateBoth(port1, port2 JoystickState) {
	switch io.joystickMode {
	case JoystickSinclairBoth:
		applySinclairState(io, sinclairKeyMatrix1, port1)
		applySinclairState(io, sinclairKeyMatrix2, port2)
	case JoystickKempstonBoth:
		io.kempstonState = kempstonByte(port1)
		io.kempston2State = kempstonByte(port2)
	}
}
