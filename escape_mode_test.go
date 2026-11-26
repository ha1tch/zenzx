package zenzx

import (
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// The Escape key is disabled unless chosen otherwise: the zero mode is off, a word that is
// no mode is off (never on), and the words agree with the settings package's.
func TestEscapeModeDefaultsToOffAndParsesItsWords(t *testing.T) {
	var zero escapeMode
	if zero != escapeOff {
		t.Error("the zero escapeMode is not off")
	}
	if m, ok := parseEscapeMode(""); m != escapeOff || !ok {
		t.Errorf(`parseEscapeMode("") = %v, %v; want off, true`, m, ok)
	}
	for i, w := range []string{"off", "close", "query", "on"} {
		m, ok := parseEscapeMode(w)
		if !ok || int(m) != i || m.String() != w {
			t.Errorf("parseEscapeMode(%q) = %v, %v", w, m, ok)
		}
	}
	for _, w := range []string{"quit", "ON", "yes", "1"} {
		if m, ok := parseEscapeMode(w); m != escapeOff || ok {
			t.Errorf("parseEscapeMode(%q) = %v, %v; want off, false", w, m, ok)
		}
	}
	if got := strings.Join(settingsconfig.EscapeWords, ","); got != strings.Join(escapeModeWords[:], ",") {
		t.Errorf("settingsconfig.EscapeWords %s differ from the modes %v", got, escapeModeWords)
	}
	if ZenZX := (&ZenZX{}); ZenZX.escape != escapeOff {
		t.Error("a new ZenZX does not start with Escape off")
	}
}
