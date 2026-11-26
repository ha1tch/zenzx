package fonts

import (
	"bytes"
	"strings"
	"testing"
)

// bdfBytes is the embedded source of each face.
func bdfBytes(n Name) []byte {
	switch n {
	case NameSinclair:
		return sinclairBDF
	case NameSinclairNarrow:
		return sinclairNarrowBDF
	case NameCozette:
		return cozetteBDF
	case NameTomThumb:
		return tomThumbBDF
	case NameSpleen:
		return spleenBDF
	case NameCreep:
		return creepBDF
	case NameHaxorMedium:
		return haxorMediumBDF
	}
	return nil
}

// declaredProportional is whether a BDF says SPACING "P".
func declaredProportional(src []byte) bool {
	for _, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "SPACING") {
			return strings.Contains(line, `"P"`)
		}
	}
	return false
}

// TestMonospacedMatchesTheFaces: the table says a face is monospaced exactly when its BDF does not
// declare itself proportional, and every printable ASCII character of one has the same advance.
func TestMonospacedMatchesTheFaces(t *testing.T) {
	for _, n := range All {
		src := bdfBytes(n)
		if len(src) == 0 {
			t.Fatalf("%s: no source", n)
		}
		if got, want := Monospaced(n), !declaredProportional(src); got != want {
			t.Errorf("Monospaced(%s) = %v, but the face's SPACING says it is %v", n, got, want)
		}
		if !Monospaced(n) {
			continue
		}
		widths := map[string]bool{}
		enc := -1
		for _, line := range bytes.Split(src, []byte("\n")) {
			f := strings.Fields(string(line))
			if len(f) == 2 && f[0] == "ENCODING" {
				enc = 0
				for _, c := range f[1] {
					if c < '0' || c > '9' {
						enc = -1
						break
					}
					enc = enc*10 + int(c-'0')
				}
			}
			if len(f) >= 2 && f[0] == "DWIDTH" && enc >= 32 && enc < 127 {
				widths[f[1]] = true
			}
		}
		if len(widths) != 1 {
			t.Errorf("%s is called monospaced but its ASCII advances are %v", n, widths)
		}
	}
	if Monospaced("NoSuchFace") {
		t.Error("an unknown name is monospaced")
	}
}

// TestNearestMonospacedKeepsAMonospacedFace: the face the person chose is used when it will do.
func TestNearestMonospacedKeepsAMonospacedFace(t *testing.T) {
	for _, n := range All {
		if Monospaced(n) && NearestMonospaced(n) != n {
			t.Errorf("NearestMonospaced(%s) = %s: a monospaced face should be kept", n, NearestMonospaced(n))
		}
	}
}

// TestNearestMonospacedIsTheClosestInSize: for a face that is not monospaced, the answer is
// monospaced, and no monospaced face has a cell closer in height, nor equally close in height and
// closer in width.
func TestNearestMonospacedIsTheClosestInSize(t *testing.T) {
	for _, n := range All {
		if Monospaced(n) {
			continue
		}
		got := NearestMonospaced(n)
		if !Monospaced(got) {
			t.Errorf("NearestMonospaced(%s) = %s, which is not monospaced", n, got)
			continue
		}
		ref, _ := Load(n)
		pick, _ := Load(got)
		for _, m := range All {
			if !Monospaced(m) || m == got {
				continue
			}
			f, _ := Load(m)
			dh, dw := abs(f.CellHeight-ref.CellHeight), abs(f.CellWidth-ref.CellWidth)
			ph, pw := abs(pick.CellHeight-ref.CellHeight), abs(pick.CellWidth-ref.CellWidth)
			if dh < ph || dh == ph && dw < pw {
				t.Errorf("for %s the answer is %s (%dx%d) but %s (%dx%d) is closer to %dx%d", n, got, pick.CellWidth, pick.CellHeight, m, f.CellWidth, f.CellHeight, ref.CellWidth, ref.CellHeight)
			}
		}
	}
	if got := NearestMonospaced("NoSuchFace"); !Monospaced(got) {
		t.Errorf("an unknown name gave %s", got)
	}
}

// TestTomThumbGetsSpleen pins the one case there is: the 4x6 face is not monospaced, and the
// closest monospaced cell is Spleen's 5x8 (same height as the Sinclair faces, but narrower).
func TestTomThumbGetsSpleen(t *testing.T) {
	if got := NearestMonospaced(NameTomThumb); got != NameSpleen {
		t.Errorf("NearestMonospaced(TomThumb) = %s, want Spleen", got)
	}
}
