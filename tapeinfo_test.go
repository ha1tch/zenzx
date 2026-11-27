package zenzx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zentools/pkg/tzx"
)

// rawTAPBlock is one block of a .tap file: its length, the flag, the payload and the checksum.
func rawTAPBlock(flag byte, payload []byte) []byte {
	body := append([]byte{flag}, payload...)
	sum := byte(0)
	for _, b := range body {
		sum ^= b
	}
	body = append(body, sum)
	return append([]byte{byte(len(body)), byte(len(body) >> 8)}, body...)
}

// aScreen is a screen's worth of bytes that is easy to tell from another's.
func aScreen(fill byte) []byte { return bytes.Repeat([]byte{fill}, tapeScreenLen) }

func concat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func TestSummariseTapeListsTheBlocksAndFindsTheScreenAfterItsHeader(t *testing.T) {
	prog := tap.EncodeProgram("LOADER", []byte{1, 2, 3, 4}, 10)
	code := tap.EncodeCode("SCREEN$", aScreen(0x55), 16384)
	s := summariseTape(concat(prog, code), false)

	kinds := []string{"Program", "Data", "Bytes", "Data"}
	if s.Format != "TAP" || len(s.Lines) != len(kinds) {
		t.Fatalf("format %q, %d blocks: %+v", s.Format, len(s.Lines), s.Lines)
	}
	for i, k := range kinds {
		if s.Lines[i].Kind != k {
			t.Errorf("block %d is %q, want %q", i+1, s.Lines[i].Kind, k)
		}
	}
	if l := s.Lines[0]; l.Name != "LOADER" || l.Detail != "4 bytes, runs from line 10" {
		t.Errorf("the program header reads %+v", l)
	}
	if l := s.Lines[2]; l.Name != "SCREEN$" || l.Detail != "6912 bytes at 16384" {
		t.Errorf("the bytes header reads %+v", l)
	}
	if s.ScreenAt != 4 || !bytes.Equal(s.Screen, aScreen(0x55)) {
		t.Errorf("the screen is at block %d (%d bytes), want block 4 of 6912", s.ScreenAt, len(s.Screen))
	}
	if !strings.Contains(s.Lines[3].Detail, "the screen shown") {
		t.Errorf("the screen's data block says %q", s.Lines[3].Detail)
	}
	if s.Partial != "" {
		t.Errorf("a whole tape is partial: %q", s.Partial)
	}
}

func TestSummariseTapeFindsAScreenThatHasNoHeader(t *testing.T) {
	s := summariseTape(concat(tap.EncodeProgram("L", []byte{1}, 1), rawTAPBlock(0xFF, aScreen(7))), false)
	if s.ScreenAt != 3 || len(s.Screen) != tapeScreenLen {
		t.Fatalf("a headerless screen is at block %d (%d bytes)", s.ScreenAt, len(s.Screen))
	}
	if d := s.Lines[2].Detail; !strings.Contains(d, "no header") || !strings.Contains(d, "the screen shown") {
		t.Errorf("the headerless screen reads %q", d)
	}
}

func TestSummariseTapeDoesNotTakeOtherBytesForAScreen(t *testing.T) {
	for name, img := range map[string][]byte{
		"6912 bytes loaded somewhere else": tap.EncodeCode("DATA", aScreen(1), 32768),
		"a screen's address, wrong length": tap.EncodeCode("DATA", make([]byte, 6000), 16384),
		"a header's data of another flag":  concat(tap.EncodeProgram("P", []byte{1}, 1), rawTAPBlock(0x80, aScreen(2))),
	} {
		if s := summariseTape(img, false); s.Screen != nil || s.ScreenAt != 0 {
			t.Errorf("%s: a screen was found at block %d", name, s.ScreenAt)
		}
	}
	// by its name a header's 6912 bytes are a screen wherever they load
	if s := summariseTape(tap.EncodeCode("title.scr", aScreen(3), 32768), false); s.ScreenAt != 2 {
		t.Errorf("a header that names a screen: found at %d, want 2", s.ScreenAt)
	}
}

func TestSummariseTapeShowsTheFirstOfTwoScreens(t *testing.T) {
	s := summariseTape(concat(tap.EncodeCode("ONE", aScreen(1), 16384), tap.EncodeCode("TWO", aScreen(2), 16384)), false)
	if s.ScreenAt != 2 || s.Screen[0] != 1 {
		t.Errorf("the screen is from block %d, first byte %d, want block 2 and 1", s.ScreenAt, s.Screen[0])
	}
}

func TestSummariseTapeKeepsTheBlocksBeforeAFileThatEndsEarly(t *testing.T) {
	img := concat(tap.EncodeProgram("P", []byte{1, 2}, 1), tap.EncodeCode("C", []byte{9, 9, 9}, 40000))
	s := summariseTape(img[:len(img)-3], false)
	if len(s.Lines) != 3 || s.Partial == "" {
		t.Fatalf("%d blocks kept, partial %q: want the 3 whole ones and a reason", len(s.Lines), s.Partial)
	}
}

func TestSummariseTapeReadsATZXAsItReadsATAP(t *testing.T) {
	img := concat(tap.EncodeProgram("LOADER", []byte{1, 2, 3}, 5), tap.EncodeCode("SCREEN$", aScreen(0x42), 16384))
	z, err := tzx.EncodeFromTAP(img, tzx.EncodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := summariseTape(z, true)
	want := summariseTape(img, false)
	if s.Format != "TZX" || len(s.Lines) != len(want.Lines) || s.ScreenAt != want.ScreenAt || !bytes.Equal(s.Screen, want.Screen) {
		t.Errorf("TZX: %d blocks, screen at %d; TAP: %d blocks, screen at %d", len(s.Lines), s.ScreenAt, len(want.Lines), want.ScreenAt)
	}
}

func TestATapeLineIsPlainText(t *testing.T) {
	l := tapeLine{Kind: "Bytes", Name: "A\xA9B", Detail: "6912 bytes at 16384"}
	if got := l.text(7); got != "  7  Bytes     A?B        6912 bytes at 16384" {
		t.Errorf("the line reads %q", got)
	}
}
