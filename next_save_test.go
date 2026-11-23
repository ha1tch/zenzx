//go:build headless

package zenzx

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// cardOverlay is a card image whose writes are kept in memory, block by block, so that a
// test can let NextZXOS write to the real card without changing the file.
type cardOverlay struct {
	base   *os.File
	blocks map[int64][]byte
}

func (c *cardOverlay) ReadAt(p []byte, off int64) (int, error) {
	if b, ok := c.blocks[off/512]; ok && off%512 == 0 && len(p) <= 512 {
		return copy(p, b), nil
	}
	return c.base.ReadAt(p, off)
}

func (c *cardOverlay) WriteAt(p []byte, off int64) (int, error) {
	if off%512 != 0 || len(p) != 512 {
		panic("cardOverlay: the SD card writes whole blocks")
	}
	c.blocks[off/512] = append([]byte(nil), p...)
	return len(p), nil
}

// TestNextZXOSSavesToCard boots NextZXOS, enters NextBASIC, types a one-line program and
// saves it, and then reads the card the way another computer would: the directory entry is
// there, with a length, and the file's first block is the +3DOS header NextBASIC writes.
//
// Part of dormant guard G-04; needs the same files as TestNextZXOSBoots:
//
//	ZENZX_NEXT_FILES=thirdparty go test -tags headless -run TestNextZXOS -v .
//
// The card file itself is not changed.
func TestNextZXOSSavesToCard(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-04)")
	}
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return data
	}
	file, err := os.Open(filepath.Join(dir, "next-card.img"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	card := &cardOverlay{base: file, blocks: map[int64][]byte{}}

	zx := NewZenZX(AudioBackendOto)
	if err := loadModelROM(zx, "next"); err != nil {
		t.Fatal(err)
	}
	zx.memory.SetBootROM(read("nextboot.rom"))
	zx.EnableDivMMC(card, info.Size(), read("enNxtmmc.rom"))
	zx.cpu.Reset()
	zx.running = true

	run := func(frames int) {
		for i := 0; i < frames; i++ {
			zx.RunFrame()
		}
	}
	// keys plays a chord or a text through the script's key queue, one frame at a time.
	keys := func(fill func(q *KeyQueue)) {
		q := NewKeyQueue(defaultKeyHoldFrames, defaultKeyGapFrames)
		fill(q)
		for q.Step(zx) {
			zx.RunFrame()
		}
		zx.io.ResetKeyboard()
	}
	chord := func(name string) { keys(func(q *KeyQueue) { q.EnqueueChord(namedKeys[name]) }) }
	text := func(s string) { keys(func(q *KeyQueue) { q.EnqueueText(s) }) }
	cyanCells := func() int {
		n := 0
		for cell := 4 * 32; cell < 16*32; cell++ {
			if zx.memory.screen.attributes[cell]&0x38 == 0x28 {
				n++
			}
		}
		return n
	}

	run(3300) // to the welcome screen
	chord("space")
	for f := 0; cyanCells() < 8; f += 50 {
		if f > 2000 {
			t.Fatal("the main menu did not appear")
		}
		run(50)
	}
	run(100)
	chord("down")
	run(20)
	chord("down")
	run(20)
	chord("enter") // NextBASIC
	run(150)
	before := card.blocksWritten()

	text("10 rem saved by the test")
	chord("enter")
	run(50)
	text(`save "zztest.bas"`)
	chord("enter")
	// SAVE makes the directory entry at once, with a length of 0, and fills in the clusters and
	// the length when it has found free ones: 600 to 700 frames on a card that has been in use,
	// where the free space is far into the FAT. So wait for the length, not for a fixed time; if
	// it never comes, the checks below say what is missing.
	savedLength := func() uint32 {
		var n uint32
		for _, b := range card.blocks {
			if i := bytes.Index(b, []byte("ZZTEST  BAS")); i >= 0 && i%32 == 0 {
				n = binary.LittleEndian.Uint32(b[i+28 : i+32])
			}
		}
		return n
	}
	saveFrames := 0
	for ; savedLength() < 128 && saveFrames <= 2000; saveFrames += 50 {
		run(50)
	}
	t.Logf("the save was complete after %d frames", saveFrames)
	expectFrames(t, "the save", saveFrames, 0, 1500)
	run(50)

	if n := card.blocksWritten() - before; n == 0 {
		t.Fatal("SAVE wrote nothing to the card")
	}
	if zx.io.divMMC.SD().wr.status != 0 {
		t.Fatalf("the card reports a write error: status 0x%02X", zx.io.divMMC.SD().wr.status)
	}

	// Read the card back from outside. The geometry comes from the partition's boot sector.
	sector := func(lba int64) []byte {
		b := make([]byte, 512)
		if _, err := card.ReadAt(b, lba*512); err != nil {
			t.Fatal(err)
		}
		return b
	}
	start := int64(binary.LittleEndian.Uint32(sector(0)[454:458]))
	boot := sector(start)
	perCluster := int64(boot[13])
	reserved := int64(binary.LittleEndian.Uint16(boot[14:16]))
	fats := int64(boot[16])
	fatSize := int64(binary.LittleEndian.Uint32(boot[36:40]))
	clusterLBA := func(cluster int64) int64 {
		return start + reserved + fats*fatSize + (cluster-2)*perCluster
	}
	// The directory entry is in one of the blocks the save wrote.
	var entry []byte
	for _, b := range card.blocks {
		if i := bytes.Index(b, []byte("ZZTEST  BAS")); i >= 0 && i%32 == 0 {
			entry = b[i : i+32]
		}
	}
	if entry == nil {
		t.Fatal("no directory entry for ZZTEST.BAS in the blocks written to the card")
	}
	size := binary.LittleEndian.Uint32(entry[28:32])
	cluster := int64(binary.LittleEndian.Uint16(entry[20:22]))<<16 | int64(binary.LittleEndian.Uint16(entry[26:28]))
	if size < 128 || cluster < 2 {
		t.Fatalf("ZZTEST.BAS has length %d and first cluster %d", size, cluster)
	}
	data := sector(clusterLBA(cluster))
	if !bytes.HasPrefix(data, []byte("PLUS3DOS\x1a")) {
		t.Fatalf("ZZTEST.BAS does not start with a +3DOS header: % X", data[:16])
	}
	if !bytes.Contains(data, []byte("saved by the test")) {
		t.Fatal("ZZTEST.BAS does not hold the program line that was typed")
	}
	whole := make([]byte, 0, int(size)+512)
	for i := int64(0); i*512 < int64(size); i++ {
		whole = append(whole, sector(clusterLBA(cluster)+i)...)
	}
	checkSavedProgram(t, whole[:size], size, "saved by the test")
	checkCardChangedOnlyWhereASaveChanges(t, card, "zztest.bas", size, boot, start)
	t.Logf("ZZTEST.BAS: %d bytes at cluster %d, %d blocks written", size, cluster, card.blocksWritten()-before)
}

func (c *cardOverlay) blocksWritten() int { return len(c.blocks) }
