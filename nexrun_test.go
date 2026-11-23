package zenzx

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// screen paints a 320x256 picture: a grey ground and the rectangles given, each filled with a
// pattern of black and white so that every row of it has ink.
func testNextScreen(rects ...image.Rectangle) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 320, 256))
	grey := color.RGBA{182, 182, 182, 255}
	for y := 0; y < 256; y++ {
		for x := 0; x < 320; x++ {
			img.SetRGBA(x, y, grey)
		}
	}
	for _, r := range rects {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				c := color.RGBA{0, 0, 0, 255}
				if (x/3+y/2)%2 == 0 {
					c = color.RGBA{255, 255, 255, 255}
				}
				img.SetRGBA(x, y, c)
			}
		}
	}
	return img
}

func TestClassifyNextScreen(t *testing.T) {
	cases := []struct {
		name string
		img  *image.RGBA
		want nexScreen
	}{
		{"the menu's first page", testNextScreen(image.Rect(96, 72, 224, 144), image.Rect(32, 200, 290, 220)), nexScreenMenu1},
		{"the menu's second page", testNextScreen(image.Rect(96, 72, 224, 176), image.Rect(32, 200, 290, 220)), nexScreenMenu2},
		{"the welcome screen or the Browser", testNextScreen(image.Rect(32, 32, 288, 220)), nexScreenWide},
		{"a command line with an error at the foot", testNextScreen(image.Rect(32, 215, 200, 223)), nexScreenOther},
		{"a blank screen", testNextScreen(), nexScreenOther},
		{"a picture of something else", testNextScreen(image.Rect(150, 120, 190, 150)), nexScreenOther},
	}
	for _, c := range cases {
		if got := classifyNextScreen(c.img); got != c.want {
			t.Errorf("%s: classified %d, want %d", c.name, got, c.want)
		}
	}
	if classifyNextScreen(nil) != nexScreenOther || classifyNextScreen(image.NewRGBA(image.Rect(0, 0, 10, 10))) != nexScreenOther {
		t.Error("a missing or tiny picture is not a screen the keys are known for")
	}
}

func TestNexVersionOK(t *testing.T) {
	good := append([]byte("NextV1.2"), make([]byte, 600)...)
	if err := nexVersionOK(good); err != nil {
		t.Errorf("a V1.2 file: %v", err)
	}
	v13 := append([]byte("NextV1.3"), make([]byte, 600)...)
	if err := nexVersionOK(v13); err == nil || !strings.Contains(err.Error(), "1.3") {
		t.Errorf("a V1.3 file: %v, want it named as one .nexload does not load", err)
	}
	if err := nexVersionOK([]byte("hello")); err == nil {
		t.Error("a file that is not a NEX was accepted")
	}
}

// The autoexec.bas of a scratch card is a +3DOS BASIC program of one line, with the header's
// length and checksum right, or NextZXOS ignores it.
func TestNexAutoexecIsABasicFile(t *testing.T) {
	f := nexAutoexec("run.nex")
	if string(f[:8]) != "PLUS3DOS" || f[8] != 0x1A {
		t.Fatalf("header starts %q", f[:9])
	}
	if got := int(f[11]) | int(f[12])<<8 | int(f[13])<<16 | int(f[14])<<24; got != len(f) {
		t.Errorf("header says %d bytes, the file has %d", got, len(f))
	}
	var sum byte
	for _, b := range f[:127] {
		sum += b
	}
	if f[127] != sum {
		t.Errorf("checksum %#x, want %#x", f[127], sum)
	}
	prog := f[128:]
	if int(f[16])|int(f[17])<<8 != len(prog) || f[15] != 0 || f[18] != 10 {
		t.Errorf("header's program fields are wrong: type %d length %d start %d", f[15], int(f[16])|int(f[17])<<8, f[18])
	}
	if prog[0] != 0 || prog[1] != 10 {
		t.Errorf("the line is numbered %d, want 10", int(prog[0])<<8|int(prog[1]))
	}
	text := prog[4:]
	if int(prog[2])|int(prog[3])<<8 != len(text) || !bytes.Equal(text, []byte(".nexload run.nex\r")) {
		t.Errorf("the line is %q", text)
	}
}

func TestRunNEXOnAMachineThatIsNotANext(t *testing.T) {
	zx, err := newMachine("48k", "./rom")
	if err != nil {
		t.Skipf("no ROMs: %v", err)
	}
	msg, reset := zx.RunNEX(filepath.Join(t.TempDir(), "x.nex"), false)
	if reset || !strings.Contains(msg, "can't run") {
		t.Errorf("RunNEX on a 48K: %q, reset %v", msg, reset)
	}
}

// stageNEX puts the file where the Next can reach it without touching a card it should not.
func TestStageNEX(t *testing.T) {
	tmp := t.TempDir()
	nex := append([]byte("NextV1.2"), make([]byte, 600)...)
	has := func(card, name string) bool {
		f, err := os.Open(card)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		vol, err := nextcard.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		return vol.Root.Find(name) != nil
	}
	made := 0
	machine := func() *ZenZX {
		made++
		main, err := newEmptyCard(filepath.Join(tmp, "main"+string(rune('0'+made))), smallestCard)
		if err != nil {
			t.Fatal(err)
		}
		fh, err := os.Open(main)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { fh.Close() })
		zx := NewZenZX(AudioBackendOto)
		zx.EnableDivMMC(fh, smallestCard, make([]byte, 8192))
		t.Cleanup(func() { zx.dropNEXScratch(); zx.RemoveSecondCard() })
		return zx
	}

	t.Run("an empty slot gets a scratch card", func(t *testing.T) {
		zx := machine()
		where, err := zx.stageNEX("g.nex", nex)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(where, "scratch") || zx.SecondCard() == "" || zx.SecondCard() != zx.nexScratch {
			t.Errorf("staged %q, slot 2 %q, scratch %q", where, zx.SecondCard(), zx.nexScratch)
		}
		if !has(zx.SecondCard(), "g.nex") {
			t.Error("the file is not on the scratch card")
		}
		dir := filepath.Dir(zx.nexScratch)
		zx.dropNEXScratch()
		if _, err := os.Stat(dir); err == nil || zx.SecondCard() != "" {
			t.Error("dropping the scratch card left its folder or its slot")
		}
	})

	t.Run("a card that can be written is added to", func(t *testing.T) {
		zx := machine()
		mine, err := newEmptyCard(filepath.Join(tmp, "mine"), smallestCard)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := zx.InsertSecondCard(mine, false); err != nil {
			t.Fatal(err)
		}
		where, err := zx.stageNEX("g.nex", nex)
		if err != nil {
			t.Fatal(err)
		}
		if zx.SecondCard() != mine || zx.nexScratch != "" || !has(mine, "g.nex") || !strings.Contains(where, "mine.img") {
			t.Errorf("staged %q; slot 2 %q, scratch %q", where, zx.SecondCard(), zx.nexScratch)
		}
	})

	t.Run("a card that is read-only is left alone", func(t *testing.T) {
		zx := machine()
		ro, err := newEmptyCard(filepath.Join(tmp, "ro"), smallestCard)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := zx.InsertSecondCard(ro, true); err != nil {
			t.Fatal(err)
		}
		where, err := zx.stageNEX("g.nex", nex)
		if err != nil {
			t.Fatal(err)
		}
		if has(ro, "g.nex") {
			t.Error("a file was written to a read-only card")
		}
		if !strings.Contains(where, "read-only") || zx.SecondCard() != zx.nexScratch {
			t.Errorf("staged %q; slot 2 %q, scratch %q", where, zx.SecondCard(), zx.nexScratch)
		}
	})
}
