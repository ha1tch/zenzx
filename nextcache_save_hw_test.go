//go:build headless

package zenzx

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcache"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// fsInfoHint reads the free-space record of a card's FAT32 volume: the count of free
// clusters and the number of the one a writer should try first.
func fsInfoHint(t *testing.T, card string) (free, next uint32) {
	t.Helper()
	img, err := os.Open(card)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Close()
	sector := func(lba int64) []byte {
		b := make([]byte, 512)
		if _, err := img.ReadAt(b, lba*512); err != nil {
			t.Fatal(err)
		}
		return b
	}
	start := int64(binary.LittleEndian.Uint32(sector(0)[454:458]))
	info := sector(start + int64(binary.LittleEndian.Uint16(sector(start)[48:50])))
	return binary.LittleEndian.Uint32(info[488:492]), binary.LittleEndian.Uint32(info[492:496])
}

// TestNextSettledCacheSavesSafelyOverAChangedCard is the other half of the question
// TestNextSettledCacheOverAChangedCard asks. That one shows the same listings; this one
// writes. A settled cache was taken on one card, files were then added to it, and NextZXOS,
// started from the cache, SAVEs a program. If it had kept where the free space was on the
// card it was made on, the save would land on the clusters of the files that were added. So:
// after a save from the cache and after one from a cold boot, every file that was on the
// card is still what it was, and the saved program is there and reads back.
//
// Dormant guard G-04 (docs/KNOWN_ISSUES.md), same files as TestNextZXOSBoots. The cards it
// writes to are copies, in a temporary folder.
func TestNextSettledCacheSavesSafelyOverAChangedCard(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-04)")
	}
	f := nextcard.In(dir)
	cacheDir := t.TempDir()
	tmp := t.TempDir()
	base := filepath.Join(tmp, "base.img")
	if _, err := newEmptyCard(base, smallestCard); err != nil {
		t.Fatal(err)
	}
	if err := copyBootFiles(f.Card, base); err != nil {
		t.Fatal(err)
	}
	baseData, _ := os.ReadFile(base)
	var added []nextcard.File
	for i, name := range []string{"games/alpha.nex", "games/beta.nex", "games/gamma.tap", "ONE.NEX", "TWO.NEX", "demos/deep/three.nex"} {
		added = append(added, nextcard.File{Name: name, Data: bytes.Repeat([]byte{byte(i + 1)}, 300+i*700)})
	}

	start := func(card string, mode nextCacheMode) (*ZenZX, *[]string) {
		said := withCache(t, mode, cacheDir)
		zx, err := newMachine("next", "./rom")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bootNextFrom(zx, f, card, false, false); err != nil {
			t.Fatal(err)
		}
		zx.running = true
		return zx, said
	}

	// The cache is made on the base card, which is not written to again.
	maker, said := start(base, nextCacheAuto)
	for i := 0; i < 2500 && len(nextcache.List(cacheDir)) < 2; i++ {
		maker.RunFrame()
	}
	maker.detachNextBootChain()
	if len(nextcache.List(cacheDir)) != 2 {
		t.Fatalf("caches made: %v", *said)
	}

	save := func(label string, mode nextCacheMode) {
		card := filepath.Join(tmp, label+".img")
		os.WriteFile(card, baseData, 0o644)
		if err := nextcard.AddTree(context.Background(), card, added, nil); err != nil {
			t.Fatal(err)
		}
		// The test means something only if the card is not the one the cache was made on in
		// the very thing a writer would go by.
		bf, bn := fsInfoHint(t, base)
		cf, cn := fsInfoHint(t, card)
		if bn == cn {
			t.Fatalf("the free-space hint is the same on both cards (next free cluster %d): the test cannot tell", bn)
		}
		t.Logf("%s: next free cluster %d on the card the cache was made on, %d here (%d and %d free)", label, bn, cn, bf, cf)
		zx, said := start(card, mode)
		run := func(n int) {
			for i := 0; i < n; i++ {
				zx.RunFrame()
			}
		}
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
		if mode == nextCacheAuto {
			zx.RunFrame()
			if !sayHas(said, "started from the settled cache") {
				t.Fatalf("%s: said %v", label, *said)
			}
			run(150)
		} else {
			run(3300)
		}
		chord("space")
		for fr := 0; cyanCells() < 8; fr += 50 {
			if fr > 2000 {
				t.Fatalf("%s: the main menu did not appear", label)
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
		text("10 rem saved by the test")
		chord("enter")
		run(50)
		text(`save "zztest.bas"`)
		chord("enter")
		run(1800)
		zx.detachNextBootChain()

		img, err := os.Open(card)
		if err != nil {
			t.Fatal(err)
		}
		defer img.Close()
		vol, err := nextcard.Open(img)
		if err != nil {
			t.Fatalf("%s: the card no longer reads as a FAT32 volume: %v", label, err)
		}
		for _, a := range added {
			n := vol.Root.Find(a.Name)
			if n == nil {
				t.Errorf("%s: %s is gone from the card", label, a.Name)
				continue
			}
			got, err := vol.ReadFile(n)
			if err != nil || !bytes.Equal(got, a.Data) {
				t.Errorf("%s: %s was overwritten by the save (read: %v)", label, a.Name, err)
			}
		}
		n := vol.Root.Find("zztest.bas")
		if n == nil || n.Size < 128 {
			t.Fatalf("%s: the saved program is not on the card", label)
		}
		data, err := vol.ReadFile(n)
		if err != nil || !bytes.HasPrefix(data, []byte("PLUS3DOS\x1a")) || !strings.Contains(string(data), "saved by the test") {
			t.Errorf("%s: the saved program does not read back (%v)", label, err)
		}
		t.Logf("%s: ZZTEST.BAS is %d bytes and all %d files that were there are intact", label, n.Size, len(added))
	}
	save("cold boot", nextCacheOff)
	save("from the settled cache", nextCacheAuto)
}
