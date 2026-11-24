package fonts

import "testing"

func TestSinclairDecodes(t *testing.T) {
	f, err := Sinclair()
	if err != nil {
		t.Fatalf("Sinclair(): %v", err)
	}
	if f == nil {
		t.Fatal("Sinclair() returned a nil font with no error")
	}
}

func TestSinclairBytesIsDefensiveCopy(t *testing.T) {
	b := SinclairBytes()
	if len(b) == 0 {
		t.Fatal("SinclairBytes() returned empty data")
	}
	original := b[0]
	b[0] ^= 0xFF // mutate the copy

	b2 := SinclairBytes()
	if b2[0] != original {
		t.Error("mutating a previous SinclairBytes() result affected a later call -- not a real copy")
	}
}

func TestAllFontsDecode(t *testing.T) {
	for _, name := range All {
		f, err := Load(name)
		if err != nil {
			t.Errorf("Load(%s): %v", name, err)
			continue
		}
		if f == nil {
			t.Errorf("Load(%s) returned a nil font with no error", name)
		}
	}
}

func TestLoadUnknownName(t *testing.T) {
	_, err := Load(Name("NotARealFont"))
	if err == nil {
		t.Fatal("Load with an unrecognised name should return an error")
	}
	var unknownErr *UnknownFontError
	if _, ok := err.(*UnknownFontError); !ok {
		t.Errorf("error type = %T, want *UnknownFontError", err)
	}
	_ = unknownErr
}

func TestAllHasNoDuplicates(t *testing.T) {
	seen := make(map[Name]bool)
	for _, name := range All {
		if seen[name] {
			t.Errorf("All contains %s more than once", name)
		}
		seen[name] = true
	}
}
