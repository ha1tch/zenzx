//go:build headless

package zenzx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcache"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextBootCacheOnTheRealFirmware is the test that says a cached boot is the boot: the
// real firmware (dormant guard G-04's files) is booted cold, and the caches it makes are
// put back into another machine; the two are run side by side and every field of the
// machine, and the picture, must agree. A field the state missed would show here.
//
//	ZENZX_NEXT_FILES=thirdparty go test -tags headless -run TestNextBootCacheOnTheRealFirmware -v .
func TestNextBootCacheOnTheRealFirmware(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-04)")
	}
	f := nextcard.In(dir)
	cacheDir := t.TempDir()

	start := func(f nextcard.Files, mode nextCacheMode) (*ZenZX, *[]string) {
		said := withCache(t, mode, cacheDir)
		zx, err := newMachine("next", "./rom")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bootNextFrom(zx, f, f.Card, true, false); err != nil {
			t.Fatal(err)
		}
		zx.running = true
		t.Cleanup(zx.detachNextBootChain)
		return zx, said
	}
	run := func(zx *ZenZX, n int) {
		for i := 0; i < n; i++ {
			zx.RunFrame()
		}
	}

	// 1. A cold boot makes both caches, one at the firmware's hand-over and one when NextZXOS is at rest.
	cold, said := start(f, nextCacheAuto)
	for i := 0; i < 2000 && len(nextcache.List(cacheDir)) < 2; i++ {
		cold.RunFrame()
	}
	infos := nextcache.List(cacheDir)
	if len(infos) != 2 {
		t.Fatalf("caches made: %d (%v)", len(infos), *said)
	}
	frames := map[nextcache.Stage]int{}
	for _, in := range infos {
		if in.Err != nil {
			t.Fatalf("%s: %v", in.Path, in.Err)
		}
		frames[in.Stage] = in.Frames
		t.Logf("%s cache: frame %d, %d bytes", in.Stage, in.Frames, in.Size)
	}
	expectFrames(t, "the hand-over cache", frames[nextcache.HandOver], 150, 1500)
	if frames[nextcache.Settled] <= frames[nextcache.HandOver] {
		t.Fatalf("settled at %d, before the hand-over at %d", frames[nextcache.Settled], frames[nextcache.HandOver])
	}
	settledAt := frames[nextcache.Settled]

	// 2. A reference that has never seen a cache, run to the same frame.
	ref, _ := start(f, nextCacheOff)
	for ref.frameCount < settledAt {
		ref.RunFrame()
	}

	// 3. From the settled cache, side by side with the reference.
	fast, said := start(f, nextCacheAuto)
	fast.RunFrame()
	if !sayHas(said, "started from the settled cache") || fast.frameCount != settledAt+1 {
		t.Fatalf("frame %d, said %v", fast.frameCount, *said)
	}
	ref.RunFrame() // the frame the restored machine has already run
	compare := func(what string) {
		t.Helper()
		if da, db := dumpOf(t, ref), dumpOf(t, fast); da != db {
			t.Fatalf("%s: the restored machine differs from the booted one: %s", what, firstDifference(da, db))
		}
		if !reflect.DeepEqual(ref.DecodeDisplay(), fast.DecodeDisplay()) {
			t.Fatalf("%s: the pictures differ", what)
		}
	}
	compare("one frame after the settled cache")
	for _, n := range []int{60, 120, 240} {
		run(ref, n)
		run(fast, n)
		compare("after more frames")
	}
	// SPACE leaves the welcome guide; both reach the same menu in the same frames.
	for _, z := range []*ZenZX{ref, fast} {
		z.io.PressKey(7, 0)
	}
	run(ref, 10)
	run(fast, 10)
	for _, z := range []*ZenZX{ref, fast} {
		z.io.ReleaseKey(7, 0)
	}
	run(ref, 200)
	run(fast, 200)
	compare("at the main menu")
	cyan := 0
	for cell := 4 * 32; cell < 16*32; cell++ {
		if fast.memory.screen.attributes[cell]&0x38 == 0x28 {
			cyan++
		}
	}
	if cyan < 8 {
		t.Fatalf("no menu highlight on the machine that started from the cache (%d cells)", cyan)
	}

	// 4. From the hand-over cache alone: the settled one is made again, and it is the same one.
	settledPath := nextcache.Path(cacheDir, mustKey(t, f).Firmware, nextcache.Settled)
	original, err := os.ReadFile(settledPath)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(settledPath)
	again, said := start(f, nextCacheAuto)
	again.RunFrame()
	if !sayHas(said, "started from the hand-over cache") {
		t.Fatalf("said %v", *said)
	}
	for i := 0; i < 2000; i++ {
		if _, err := os.Stat(settledPath); err == nil {
			break
		}
		again.RunFrame()
	}
	remade, err := os.ReadFile(settledPath)
	if err != nil {
		t.Fatalf("the settled cache was not made again: %v", *said)
	}
	a, _ := nextcache.Read(cacheDir, mustKey(t, f), nextcache.Settled)
	if a == nil {
		t.Fatal("the new settled cache does not read back")
	}
	if !bytes.Contains(original, a.State) {
		t.Fatalf("the settled cache made from the hand-over one (%d bytes, frame %d) is not the one made by a cold boot (%d bytes)",
			len(remade), a.Frames, len(original))
	}

	// 5. A different DivMMC ROM is another machine: the cache is refused, and named.
	other := t.TempDir()
	rom, _ := os.ReadFile(f.DivMMCROM)
	rom[100] ^= 0xFF
	f2 := f
	f2.DivMMCROM = filepath.Join(other, nextcard.DivMMCROMName)
	os.WriteFile(f2.DivMMCROM, rom, 0o644)
	third, said := start(f2, nextCacheAuto)
	third.RunFrame()
	if !sayHas(said, "out of date") || !sayHas(said, "the DivMMC ROM") || third.frameCount > 2 {
		t.Fatalf("frame %d, said %v", third.frameCount, *said)
	}
}

func mustKey(t *testing.T, f nextcard.Files) nextcache.Key {
	t.Helper()
	k, err := bareNext(t).nextCacheKeyFor(f)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// TestNextSettledCacheOverAChangedCard asks the question the study left open (section 3 of
// NEXT_BOOT_CACHE.md): NextZXOS may hold the card's directories in RAM at rest, and a
// state taken on one card then shows the wrong card when put back over another. The
// cache is made on a card; files and a folder are then added to the card, and a machine
// that starts from the cache is run beside one that booted from the firmware, on the
// changed card, into the Browser and into the folder. They must show the same listings.
func TestNextSettledCacheOverAChangedCard(t *testing.T) {
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
	changed := filepath.Join(tmp, "changed.img")
	data, _ := os.ReadFile(base)
	os.WriteFile(changed, data, 0o644)
	var added []nextcard.File
	for i, name := range []string{"games/alpha.nex", "games/beta.nex", "games/gamma.tap", "ONE.NEX", "TWO.NEX", "demos/deep/three.nex"} {
		added = append(added, nextcard.File{Name: name, Data: bytes.Repeat([]byte{byte(i + 1)}, 300+i*700)})
	}
	if err := nextcard.AddTree(context.Background(), changed, added, nil); err != nil {
		t.Fatal(err)
	}

	start := func(card string, mode nextCacheMode) (*ZenZX, *[]string) {
		said := withCache(t, mode, cacheDir)
		zx, err := newMachine("next", "./rom")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := bootNextFrom(zx, f, card, true, false); err != nil {
			t.Fatal(err)
		}
		zx.running = true
		t.Cleanup(zx.detachNextBootChain)
		return zx, said
	}
	run := func(zs []*ZenZX, n int) {
		for i := 0; i < n; i++ {
			for _, z := range zs {
				z.RunFrame()
			}
		}
	}
	press := func(zs []*ZenZX, name string) {
		var chord []matrixPos
		if c, ok := namedKeys[name]; ok {
			chord = c
		} else {
			chord = []matrixPos{baseKeys[[]rune(name)[0]]}
		}
		for _, z := range zs {
			for _, k := range chord {
				z.io.PressKey(k.row, k.col)
			}
		}
		run(zs, 8)
		for _, z := range zs {
			for _, k := range chord {
				z.io.ReleaseKey(k.row, k.col)
			}
		}
		run(zs, 12)
	}

	// The cache is made on the base card.
	maker, said := start(base, nextCacheAuto)
	for i := 0; i < 2500 && len(nextcache.List(cacheDir)) < 2; i++ {
		maker.RunFrame()
	}
	if len(nextcache.List(cacheDir)) != 2 {
		t.Fatalf("caches made: %v", *said)
	}
	settledAt := 0
	for _, in := range nextcache.List(cacheDir) {
		if in.Stage == nextcache.Settled {
			settledAt = in.Frames
		}
	}

	// On the changed card: one machine boots, one starts from the cache.
	ref, _ := start(changed, nextCacheOff)
	for ref.frameCount < settledAt {
		ref.RunFrame()
	}
	fast, said := start(changed, nextCacheAuto)
	fast.RunFrame()
	if !sayHas(said, "started from the settled cache") {
		t.Fatalf("said %v", *said)
	}
	ref.RunFrame()
	both := []*ZenZX{ref, fast}
	same := func(what string) {
		t.Helper()
		if !reflect.DeepEqual(ref.DecodeDisplay(), fast.DecodeDisplay()) {
			t.Errorf("%s: the pictures differ (%.1f%% alike)", what, 100*sameShare(ref.DecodeDisplay(), fast.DecodeDisplay()))
		} else {
			t.Logf("%s: the pictures agree", what)
		}
		da, db := dumpOf(t, ref), dumpOf(t, fast)
		la, lb := strings.Split(da, "\n"), strings.Split(db, "\n")
		n := 0
		for i := 0; i < len(la) && i < len(lb); i++ {
			if la[i] != lb[i] {
				if n < 12 {
					t.Logf("    %s: %s   |   %s", what, la[i], lb[i])
				}
				n++
			}
		}
		t.Logf("    %s: %d fields of the machine differ", what, n)
	}
	run(both, 100)
	same("at rest, before any key")
	press(both, "space")
	run(both, 150)
	same("at the menu")
	press(both, "enter") // the Browser, which lists the card
	run(both, 250)
	same("in the Browser, listing the changed card")
	if ink := inkInFileArea(fast); ink == 0 {
		t.Fatal("the Browser shows no files on the machine that started from the cache")
	}
	press(both, "enter") // into the first folder
	run(both, 250)
	same("inside a folder of the changed card")
	if da, db := dumpOf(t, ref), dumpOf(t, fast); da != db {
		t.Logf("the machines differ in memory now: %s", firstDifference(da, db))
	}
}
