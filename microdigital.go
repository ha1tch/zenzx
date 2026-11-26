package zenzx

import (
	"strings"
	"sync/atomic"
)

// The language switch of the Microdigital clones (the TK90X and the TK95). On those machines a
// diode, fitted or not, set the level of bit 7 of the ULA port (0xFE) when it was read, and some
// of their ROMs read it before they show a message: a ROM that carries a Portuguese and a Spanish
// table of messages (as Microdigital's do) picks between them on that bit -- low, Portuguese;
// high, Spanish. A ROM that does not read it (a standard Spectrum's, or a Microdigital ROM from
// which the read was patched out) is not moved by it.
//
// ZenZX has no Microdigital model, so the switch is a setting of the program, not of a machine:
// it is the same for every machine that runs, but only the 48K models (the 48K and the Spanish
// 48K) read it: the Amstrad and Timex models and the Next are left as they were. It is on the
// Preferences window and in settings.json. Spanish, the default, leaves bit 7 high, as it reads on every other Spectrum;
// Portuguese reads it low.

// isPlain48K is whether the memory is a 48K machine's: not a 128K, +2, +2A or +3, not a Timex
// TS2068, not a Next. The Microdigital clones were 48K machines, and the language switch is only
// theirs: on the others the ULA port reads as it always did. No memory at all counts as a 48K.
func (m *SpectrumMemory) isPlain48K() bool {
	return m == nil || !(m.is128K || m.isPlus3 || m.isTS2068 || m.isNext)
}

// microdigitalPortuguese is the position of the switch: set, and on a 48K machine bit 7 of the
// ULA port reads low.
var microdigitalPortuguese atomic.Bool

// SetMicrodigitalLanguage turns the language switch to Portuguese (true) or Spanish (false).
func SetMicrodigitalLanguage(portuguese bool) { microdigitalPortuguese.Store(portuguese) }

// MicrodigitalPortuguese is whether the language switch is at Portuguese.
func MicrodigitalPortuguese() bool { return microdigitalPortuguese.Load() }

// parseMicrodigitalLanguage reads a position of the switch, from the -language-switch switch or
// from settings.json: portuguese (or pt) is true, spanish (or es) is false. ok is false for any
// other word, so that a mistyped switch leaves the position it found.
func parseMicrodigitalLanguage(s string) (portuguese, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "portuguese", "pt":
		return true, true
	case "spanish", "es":
		return false, true
	}
	return false, false
}

// microdigitalWord is the position as the switch and settings.json spell it.
func microdigitalWord(portuguese bool) string {
	if portuguese {
		return "portuguese"
	}
	return "spanish"
}

// languageSwitchHelp is the -language-switch switch's help, in the window and the headless program.
const languageSwitchHelp = "Position of the Microdigital (TK90X, TK95) language switch, for this session: spanish or portuguese. The ROMs of those machines that read the diode behind it show Spanish or Portuguese messages; it reads as bit 7 of the ULA port, high at spanish and low at portuguese, on the 48K models only. Without it, the program with a window uses the setting of its Preferences window (File > Preferences), and that is spanish until it is changed; this program has no Preferences"
