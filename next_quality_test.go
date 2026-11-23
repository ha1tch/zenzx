//go:build headless

package zenzx

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"sort"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The G-04 tests run the real firmware, whose screens cannot be compared with a golden image:
// the card it starts from, the clock and the flashing cursor all change what is drawn. So
// passing is not the whole of the check. These are the ones that need no golden, each of
// them something a wrong result breaks: the screen has the structure of a menu and not of a
// blank, a crash or noise; a milestone comes within the time it should; what a save writes
// is a +3DOS program that holds the line that was typed, on a card whose two FATs still
// agree and which is changed nowhere else; and a card made by MakeBootable shows the menu
// the full card shows.

// tester is what the checks need of a *testing.T, so that a test can hand them a recorder and
// see that they do fail for what they are there to catch.
type tester interface {
	Helper()
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
}

// expectFrames fails when a milestone came outside the frames it is expected in. The bounds
// are wide (about a third to three times what was measured on the 26.11 firmware): they are
// there to catch a boot that is suspiciously quick, which has skipped a stage, and one that
// has come to take far longer, which has started to wait for something.
func expectFrames(t tester, what string, got, lo, hi int) {
	t.Helper()
	if got < lo || got > hi {
		t.Errorf("%s came after %d frames; expected between %d and %d", what, got, lo, hi)
	}
}

// screenFacts is what can be said of a picture without knowing what it should be.
type screenFacts struct {
	W, H       int
	Colours    int     // how many different colours are used
	Background float64 // the share of the picture that is its commonest colour
	Runs       float64 // the share of pixels that equal the one to their left
	Rows       int     // how many of the 8-pixel rows hold anything but the commonest colour
	RowCount   int     // how many 8-pixel rows there are
}

func measureScreen(img *image.RGBA) screenFacts {
	b := img.Bounds()
	f := screenFacts{W: b.Dx(), H: b.Dy()}
	count := map[[4]uint8]int{}
	px := func(x, y int) [4]uint8 {
		c := img.RGBAAt(b.Min.X+x, b.Min.Y+y)
		return [4]uint8{c.R, c.G, c.B, c.A}
	}
	same := 0
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			c := px(x, y)
			count[c]++
			if x > 0 && px(x-1, y) == c {
				same++
			}
		}
	}
	var common [4]uint8
	best := 0
	for c, n := range count {
		if n > best || (n == best && lessColour(c, common)) {
			best, common = n, c
		}
	}
	total := f.W * f.H
	f.Colours = len(count)
	f.Background = float64(best) / float64(total)
	f.Runs = float64(same) / float64(total-f.H)
	f.RowCount = f.H / 8
	for r := 0; r < f.RowCount; r++ {
		lit := false
		for y := r * 8; y < r*8+8 && !lit; y++ {
			for x := 0; x < f.W; x++ {
				if px(x, y) != common {
					lit = true
					break
				}
			}
		}
		if lit {
			f.Rows++
		}
	}
	return f
}

func lessColour(a, b [4]uint8) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func (f screenFacts) String() string {
	return fmt.Sprintf("%dx%d, %d colours, background %.0f%%, runs %.0f%%, %d of %d rows used", f.W, f.H, f.Colours, f.Background*100, f.Runs*100, f.Rows, f.RowCount)
}

// checkScreenLooksLikeAMenu fails for a picture that is blank, is noise, is one smear, or has
// the colours of a loader. A menu or a text screen is made of runs of one colour (text on a
// plain paper), has a plain background, uses a handful of colours and has something on
// most of its rows; a screen of random bytes has runs of about a sixth of its pixels, and
// a crashed machine a blank or a barred screen.
func checkScreenLooksLikeAMenu(t tester, img *image.RGBA, what string) screenFacts {
	t.Helper()
	f := measureScreen(img)
	t.Logf("%s: %s", what, f)
	switch {
	case f.Colours < 3:
		t.Errorf("%s: %d colours; a blank screen has one or two, a menu has at least three", what, f.Colours)
	case f.Colours > 40:
		t.Errorf("%s: %d colours; that is a picture or noise, not a menu", what, f.Colours)
	}
	if f.Background < 0.30 || f.Background > 0.97 {
		t.Errorf("%s: the commonest colour is %.0f%% of the screen; a menu has a plain background of 30 to 97%%", what, f.Background*100)
	}
	if f.Runs < 0.75 {
		t.Errorf("%s: only %.0f%% of the pixels continue a run of their colour; text on a plain paper gives over 75%%, noise a sixth", what, f.Runs*100)
	}
	if f.Rows*4 < f.RowCount {
		t.Errorf("%s: %d of %d rows hold anything; a menu fills at least a quarter", what, f.Rows, f.RowCount)
	}
	return f
}

// checkScreenIsSteady runs the machine on for a few frames and holds the picture to be nearly
// the same: a menu waiting for a key changes only where its cursor flashes, and a machine that
// is still loading, or has crashed into a loop that draws, changes a good deal more.
func checkScreenIsSteady(t *testing.T, zx *ZenZX, what string) {
	t.Helper()
	before := zx.DecodeDisplay()
	for i := 0; i < 25; i++ {
		zx.RunFrame()
	}
	same := screenSimilarity(before, zx.DecodeDisplay())
	t.Logf("%s: %.1f%% of the picture is the same 25 frames later", what, same*100)
	if same < 0.98 {
		t.Errorf("%s: only %.1f%% of the picture is the same 25 frames later; a menu that is waiting for a key changes only at its cursor", what, same*100)
	}
}

// screenSimilarity is the share of pixels two pictures of the same size agree on.
func screenSimilarity(a, b *image.RGBA) float64 {
	if a.Bounds() != b.Bounds() {
		return 0
	}
	n, same := 0, 0
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			n++
			if a.RGBAAt(x, y) == b.RGBAAt(x, y) {
				same++
			}
		}
	}
	return float64(same) / float64(n)
}

// mergedCard reads a cardOverlay as one whole image, whatever the size and the alignment of
// the read: the overlay answers only the reads of one block it holds.
type mergedCard struct{ c *cardOverlay }

func (m mergedCard) ReadAt(p []byte, off int64) (int, error) {
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		lba, within := pos/512, int(pos%512)
		var block [512]byte
		if b, ok := m.c.blocks[lba]; ok {
			copy(block[:], b)
		} else if _, err := m.c.base.ReadAt(block[:], lba*512); err != nil {
			return n, err
		}
		n += copy(p[n:], block[within:])
	}
	return n, nil
}

// checkSavedProgram holds a file NextBASIC saved to the rules of a +3DOS BASIC program that
// has the one line typed in it: the header's signature, its length, its type and its
// checksum, and a tokenised line 10 that is a REM of the text typed, as long as it says.
func checkSavedProgram(t tester, data []byte, size uint32, rem string) {
	t.Helper()
	if uint32(len(data)) != size {
		t.Errorf("the file read back is %d bytes; the directory says %d", len(data), size)
		return
	}
	if len(data) < 128+6 {
		t.Errorf("the file is %d bytes, shorter than a header and a line", len(data))
		return
	}
	h := data[:128]
	if !bytes.HasPrefix(h, []byte("PLUS3DOS\x1a")) {
		t.Errorf("no +3DOS signature: % X", h[:9])
	}
	if got := binary.LittleEndian.Uint32(h[11:15]); got != size {
		t.Errorf("the header says the file is %d bytes; it is %d", got, size)
	}
	if h[15] != 0 {
		t.Errorf("the header's type is %d; a BASIC program is 0", h[15])
	}
	var sum byte
	for _, b := range h[:127] {
		sum += b
	}
	if sum != h[127] {
		t.Errorf("the header's checksum is %02X; its bytes add up to %02X", h[127], sum)
	}
	body := data[128:]
	if progLen := binary.LittleEndian.Uint16(h[16:18]); int(progLen) != len(body) {
		t.Errorf("the header says the program is %d bytes; %d follow it", progLen, len(body))
	}
	if line := int(body[0])<<8 | int(body[1]); line != 10 {
		t.Errorf("the first line is numbered %d, not 10", line)
	}
	n := int(binary.LittleEndian.Uint16(body[2:4]))
	if 4+n != len(body) {
		t.Errorf("line 10 says it is %d bytes; the program has %d after its number and length", n, len(body)-4)
		return
	}
	text := body[4:]
	if len(text) == 0 || text[len(text)-1] != 0x0D {
		t.Errorf("line 10 does not end with ENTER")
		return
	}
	if text[0] != 0xEA {
		t.Errorf("line 10 starts with token %02X, not REM (EA)", text[0])
	}
	if got := string(text[1 : len(text)-1]); got != rem {
		t.Errorf("line 10 is a REM of %q, not %q", got, rem)
	}
}

// checkCardChangedOnlyWhereASaveChanges holds every block a save wrote to the places a save
// writes: the FSInfo sector, the two FATs, the root directory and the new file's first
// cluster; a block in the boot sector or in another file's data is a stray write. It also
// holds the two FATs to be equal after the save, and the volume to read as a volume with the
// file in it, by a reader that is not NextZXOS. The file is taken to be in one cluster, as
// a one-line program is.
func checkCardChangedOnlyWhereASaveChanges(t tester, card *cardOverlay, name string, size uint32, boot []byte, start int64) {
	t.Helper()
	perCluster := int64(boot[13])
	reserved := int64(binary.LittleEndian.Uint16(boot[14:16]))
	fats := int64(boot[16])
	fatSize := int64(binary.LittleEndian.Uint32(boot[36:40]))
	rootCluster := int64(binary.LittleEndian.Uint32(boot[44:48]))
	fsInfo := int64(binary.LittleEndian.Uint16(boot[48:50]))
	dataStart := start + reserved + fats*fatSize
	if int64(size) > perCluster*512 {
		t.Errorf("%s is %d bytes, more than the %d of a cluster", name, size, perCluster*512)
		return
	}

	merged := mergedCard{card}
	vol, err := nextcard.Open(merged)
	if err != nil {
		t.Errorf("the card does not read as a FAT32 volume after the save: %v", err)
		return
	}
	node := vol.Root.Find(name)
	if node == nil {
		t.Errorf("the volume has no %s after the save", name)
		return
	}
	if node.Size != size {
		t.Errorf("the volume says %s is %d bytes; the directory entry says %d", name, node.Size, size)
	}
	if _, err := vol.ReadFile(node); err != nil {
		t.Errorf("%s cannot be read through its cluster chain: %v", name, err)
	}

	rootLo := dataStart + (rootCluster-2)*perCluster
	var stray []int64
	for lba := range card.blocks {
		switch {
		case lba == start+fsInfo:
		case lba >= start+reserved && lba < dataStart:
		case lba >= rootLo && lba < rootLo+perCluster:
		default:
			if path, ok := vol.FileAt(lba * 512); !ok || !strings.EqualFold(path, "/"+name) {
				stray = append(stray, lba)
			}
		}
	}
	sort.Slice(stray, func(i, j int) bool { return stray[i] < stray[j] })
	if len(stray) > 0 {
		t.Errorf("a save wrote blocks outside the FSInfo sector, the FATs, the root directory and the file's first cluster: %v", stray)
	}
	if n := len(card.blocks); n < 3 || n > 40 {
		t.Errorf("a one-line save wrote %d blocks; expected between 3 and 40", n)
	}

	a := make([]byte, fatSize*512)
	b := make([]byte, fatSize*512)
	if _, err := merged.ReadAt(a, (start+reserved)*512); err != nil {
		t.Errorf("%v", err)
		return
	}
	if _, err := merged.ReadAt(b, (start+reserved+fatSize)*512); err != nil {
		t.Errorf("%v", err)
		return
	}
	if !bytes.Equal(a, b) {
		t.Errorf("the card's two FAT copies differ after the save")
	}
}

// nextMenuScreen starts a Next from the boot files f with the card at card, read-only, and
// brings it to the NextZXOS main menu, pressing SPACE at the welcome screen if there is one.
// It returns the picture of the menu once it has settled.
func nextMenuScreen(t *testing.T, f nextcard.Files, card string) *image.RGBA {
	t.Helper()
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootNextFrom(zx, f, card, true, false); err != nil {
		t.Fatal(err)
	}
	defer zx.detachNextBootChain()
	cyan := func() int {
		n := 0
		for cell := 4 * 32; cell < 16*32; cell++ {
			if zx.memory.screen.attributes[cell]&0x38 == 0x28 {
				n++
			}
		}
		return n
	}
	for frames := 0; frames < 8000; frames += 50 {
		for i := 0; i < 50; i++ {
			zx.RunFrame()
		}
		if frames > 600 && frames%300 == 0 && cyan() < 8 {
			zx.io.PressKey(7, 0)
			for i := 0; i < 10; i++ {
				zx.RunFrame()
			}
			zx.io.ReleaseKey(7, 0)
		}
		if zx.frameCount > 600 && cyan() >= 8 {
			for i := 0; i < 50; i++ {
				zx.RunFrame()
			}
			return zx.DecodeDisplay()
		}
	}
	t.Fatalf("no NextZXOS menu from %s in 8000 frames", card)
	return nil
}
