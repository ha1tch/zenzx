//go:build headless

package zenzx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// recorder is a tester that keeps what it is told, for the checks of next_quality_test.go to
// be run on pictures and files that are wrong.
type recorder struct{ errs []string }

func (r *recorder) Helper()                 {}
func (r *recorder) Logf(f string, a ...any) {}
func (r *recorder) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

// menuLike is a picture of 320x256 with the structure of a text menu: a plain paper, rows of
// text-like runs of a second colour, and a highlighted band.
func menuLike() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 320, 256))
	paper := color.RGBA{0, 0, 0xD7, 0xFF}
	ink := color.RGBA{0xD7, 0xD7, 0xD7, 0xFF}
	band := color.RGBA{0, 0xD7, 0xD7, 0xFF}
	for y := 0; y < 256; y++ {
		for x := 0; x < 320; x++ {
			img.SetRGBA(x, y, paper)
		}
	}
	for row := 4; row < 16; row++ {
		for y := row * 8; y < row*8+8; y++ {
			for x := 32; x < 200; x++ {
				switch {
				case row == 6:
					img.SetRGBA(x, y, band)
				case (x/4+y/2)%3 != 0 && y%8 > 1 && y%8 < 7:
					img.SetRGBA(x, y, ink)
				}
			}
		}
	}
	return img
}

func TestScreenChecksTellAMenuFromWhatIsNot(t *testing.T) {
	ok := &recorder{}
	checkScreenLooksLikeAMenu(ok, menuLike(), "menu")
	if len(ok.errs) != 0 {
		t.Fatalf("the menu-like picture was turned away: %v", ok.errs)
	}

	blank := image.NewRGBA(image.Rect(0, 0, 320, 256))
	noise := image.NewRGBA(image.Rect(0, 0, 320, 256))
	rng := rand.New(rand.NewSource(1))
	palette := []color.RGBA{{0, 0, 0, 255}, {0, 0, 0xD7, 255}, {0xD7, 0, 0, 255}, {0xD7, 0, 0xD7, 255}, {0, 0xD7, 0, 255}, {0, 0xD7, 0xD7, 255}, {0xD7, 0xD7, 0, 255}, {0xD7, 0xD7, 0xD7, 255}}
	for y := 0; y < 256; y++ {
		for x := 0; x < 320; x++ {
			noise.SetRGBA(x, y, palette[rng.Intn(len(palette))])
		}
	}
	bars := image.NewRGBA(image.Rect(0, 0, 320, 256)) // a loader's stripes: whole rows, few colours
	for y := 0; y < 256; y++ {
		for x := 0; x < 320; x++ {
			bars.SetRGBA(x, y, palette[(y/2)%2*5])
		}
	}
	smear := menuLike() // a menu whose text is gone but for a few rows
	for y := 80; y < 256; y++ {
		for x := 0; x < 320; x++ {
			smear.SetRGBA(x, y, color.RGBA{0, 0, 0xD7, 0xFF})
		}
	}
	for name, img := range map[string]*image.RGBA{"blank": blank, "noise": noise, "stripes": bars, "a smear": smear} {
		r := &recorder{}
		checkScreenLooksLikeAMenu(r, img, name)
		if len(r.errs) == 0 {
			t.Errorf("%s passed as a menu", name)
		}
	}
}

func TestFrameBounds(t *testing.T) {
	r := &recorder{}
	expectFrames(r, "x", 450, 150, 1500)
	if len(r.errs) != 0 {
		t.Fatalf("450 frames was out of 150 to 1500: %v", r.errs)
	}
	for _, got := range []int{10, 149, 1501, 90000} {
		r := &recorder{}
		expectFrames(r, "x", got, 150, 1500)
		if len(r.errs) == 0 {
			t.Errorf("%d frames passed as within 150 to 1500", got)
		}
	}
}

// program makes the file NextBASIC saves for 10 REM text: a +3DOS header and one line.
func program(rem string) []byte {
	line := append([]byte{0xEA}, []byte(rem)...)
	line = append(line, 0x0D)
	body := []byte{0, 10, byte(len(line)), byte(len(line) >> 8)}
	body = append(body, line...)
	h := make([]byte, 128)
	copy(h, "PLUS3DOS\x1a")
	h[9], h[10] = 1, 0 // issue and version
	binary.LittleEndian.PutUint32(h[11:15], uint32(128+len(body)))
	h[15] = 0
	binary.LittleEndian.PutUint16(h[16:18], uint16(len(body)))
	var sum byte
	for _, b := range h[:127] {
		sum += b
	}
	h[127] = sum
	return append(h, body...)
}

func TestSavedProgramChecks(t *testing.T) {
	good := program("saved by the test")
	r := &recorder{}
	checkSavedProgram(r, good, uint32(len(good)), "saved by the test")
	if len(r.errs) != 0 {
		t.Fatalf("a well-formed program was turned away: %v", r.errs)
	}
	mutate := func(f func(b []byte) []byte) []byte { return f(append([]byte(nil), good...)) }
	bad := map[string][]byte{
		"a flipped header byte":   mutate(func(b []byte) []byte { b[20] ^= 1; return b }),
		"a wrong signature":       mutate(func(b []byte) []byte { b[0] = 'X'; return b }),
		"a wrong length field":    mutate(func(b []byte) []byte { b[11]++; return b }),
		"a wrong type":            mutate(func(b []byte) []byte { b[15] = 3; return b }),
		"another line number":     mutate(func(b []byte) []byte { b[129] = 20; return b }),
		"no REM token":            mutate(func(b []byte) []byte { b[132] = 0xF5; return b }),
		"other text":              mutate(func(b []byte) []byte { b[140] = 'X'; return b }),
		"no ENTER at the end":     mutate(func(b []byte) []byte { b[len(b)-1] = 0; return b }),
		"a wrong line length":     mutate(func(b []byte) []byte { b[130]++; return b }),
		"a truncated file":        good[:140],
		"a file longer than said": append(append([]byte(nil), good...), 0),
	}
	for name, data := range bad {
		r := &recorder{}
		checkSavedProgram(r, data, uint32(len(good)), "saved by the test")
		if len(r.errs) == 0 {
			t.Errorf("%s passed as a saved program", name)
		}
	}
	// The messages say what is wrong.
	r = &recorder{}
	checkSavedProgram(r, bad["a flipped header byte"], uint32(len(good)), "saved by the test")
	if !strings.Contains(strings.Join(r.errs, "\n"), "checksum") {
		t.Errorf("a flipped header byte was not reported as a checksum: %v", r.errs)
	}
}

// A card with another file on it and the one a save would make, for the card check to be run
// on writes that a save makes and on ones it should not.
func TestCardChangeCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "card.img")
	root := nextcard.NewDir("")
	root.Add("other.bin", bytes.Repeat([]byte{0xAA}, 3000))
	data := program("saved by the test")
	root.Add("zztest.bas", data)
	if _, err := nextcard.NewCard(path, nextcard.SmallestCard, root); err != nil {
		t.Fatal(err)
	}
	base, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	sector := func(lba int64) []byte {
		b := make([]byte, 512)
		if _, err := base.ReadAt(b, lba*512); err != nil {
			t.Fatal(err)
		}
		return b
	}
	start := int64(binary.LittleEndian.Uint32(sector(0)[454:458]))
	boot := sector(start)
	reserved := int64(binary.LittleEndian.Uint16(boot[14:16]))
	fatSize := int64(binary.LittleEndian.Uint32(boot[36:40]))
	dataStart := start + reserved + 2*fatSize
	vol, err := nextcard.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	clusterOf := func(name string) int64 {
		for lba := dataStart; lba < dataStart+int64(vol.SectorsPerCluster())*8; lba++ {
			if p, ok := vol.FileAt(lba * 512); ok && strings.EqualFold(p, "/"+name) {
				return lba
			}
		}
		t.Fatalf("no %s in the first clusters", name)
		return 0
	}
	// What a save writes: the FSInfo sector, a block of each FAT, the directory's first block
	// and the file's first block.
	save := func() *cardOverlay {
		c := &cardOverlay{base: base, blocks: map[int64][]byte{}}
		for _, lba := range []int64{start + int64(binary.LittleEndian.Uint16(boot[48:50])), start + reserved, start + reserved + fatSize, dataStart + (int64(binary.LittleEndian.Uint32(boot[44:48]))-2)*int64(boot[13]), clusterOf("zztest.bas")} {
			c.blocks[lba] = sector(lba)
		}
		return c
	}

	r := &recorder{}
	checkCardChangedOnlyWhereASaveChanges(r, save(), "zztest.bas", uint32(len(data)), boot, start)
	if len(r.errs) != 0 {
		t.Fatalf("the writes of a save were turned away: %v", r.errs)
	}

	stray := save()
	other := clusterOf("other.bin")
	stray.blocks[other] = sector(other)
	r = &recorder{}
	checkCardChangedOnlyWhereASaveChanges(r, stray, "zztest.bas", uint32(len(data)), boot, start)
	if len(r.errs) == 0 || !strings.Contains(strings.Join(r.errs, "\n"), "outside") {
		t.Errorf("a write into another file's data was not reported: %v", r.errs)
	}

	bootSector := save()
	bootSector.blocks[start] = sector(start)
	r = &recorder{}
	checkCardChangedOnlyWhereASaveChanges(r, bootSector, "zztest.bas", uint32(len(data)), boot, start)
	if len(r.errs) == 0 {
		t.Errorf("a write into the boot sector was not reported")
	}

	split := save()
	b := sector(start + reserved)
	b[8] ^= 0xFF // one FAT now says something the other does not
	split.blocks[start+reserved] = b
	r = &recorder{}
	checkCardChangedOnlyWhereASaveChanges(r, split, "zztest.bas", uint32(len(data)), boot, start)
	if len(r.errs) == 0 || !strings.Contains(strings.Join(r.errs, "\n"), "FAT") {
		t.Errorf("FAT copies that differ were not reported: %v", r.errs)
	}

	nothing := &cardOverlay{base: base, blocks: map[int64][]byte{}}
	r = &recorder{}
	checkCardChangedOnlyWhereASaveChanges(r, nothing, "zztest.bas", uint32(len(data)), boot, start)
	if len(r.errs) == 0 {
		t.Error("a save that wrote nothing was not reported")
	}
}
