//go:build headless

package zenzx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcache"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The start of a Next from a cache (nextcacheuse.go), on a chain whose firmware is made up:
// what the real firmware does is G-04's, in nextcache_hw_test.go.

// fakeChain makes the three files of a boot chain, the card a real FAT32 volume that holds
// every firmware file, each with the content content(name).
func fakeChain(t *testing.T, content func(name string) string) nextcard.Files {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{nextcard.BootROMName, nextcard.DivMMCROMName} {
		if err := os.WriteFile(filepath.Join(dir, n), bytes.Repeat([]byte(n[:1]), 8192), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	card := filepath.Join(dir, nextcard.CardName)
	if _, err := newEmptyCard(card, smallestCard); err != nil {
		t.Fatal(err)
	}
	var files []nextcard.File
	for _, n := range nextcard.BootFiles {
		files = append(files, nextcard.File{Name: n, Data: []byte(content(n))})
	}
	if err := nextcard.AddTree(context.Background(), card, files, nil); err != nil {
		t.Fatal(err)
	}
	return nextcard.In(dir)
}

func sameContent(name string) string { return "firmware " + name }

// chainNext is a Next with the boot chain attached, as a front end leaves it.
func chainNext(t *testing.T, f nextcard.Files) *ZenZX {
	t.Helper()
	zx := bareNext(t)
	if _, err := zx.attachNextBootChain(f, true, false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(zx.detachNextBootChain)
	return zx
}

// withCache sets the front end's choice for a test and returns the lines it said.
func withCache(t *testing.T, mode nextCacheMode, dir string) *[]string {
	t.Helper()
	saved := nextBootCache
	var said []string
	nextBootCache = nextBootCacheSettings{mode: mode, dir: dir, logf: func(f string, a ...any) {
		said = append(said, strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(fmt.Sprintf(f, a...)), "\n", " ")))
	}}
	t.Cleanup(func() { nextBootCache = saved })
	return &said
}

func sayHas(said *[]string, part string) bool {
	for _, s := range *said {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

func TestParseNextCacheMode(t *testing.T) {
	for s, want := range map[string]nextCacheMode{"off": nextCacheOff, "auto": nextCacheAuto, "refresh": nextCacheRefresh} {
		if got, err := parseNextCacheMode(s); err != nil || got != want || got.String() != s {
			t.Errorf("%q: %v %v", s, got, err)
		}
	}
	for _, s := range []string{"", "on", "AUTO", "yes"} {
		if _, err := parseNextCacheMode(s); err == nil {
			t.Errorf("%q was accepted", s)
		}
	}
}

func TestNextCacheKeyFollowsTheFirmwareOnTheCard(t *testing.T) {
	a := fakeChain(t, sameContent)
	b := fakeChain(t, sameContent)
	c := fakeChain(t, func(n string) string {
		if strings.EqualFold(n, "TBBLUE.FW") {
			return "firmware changed"
		}
		return sameContent(n)
	})
	zx := bareNext(t)
	ka, err := zx.nextCacheKeyFor(a)
	if err != nil {
		t.Fatal(err)
	}
	kb, _ := zx.nextCacheKeyFor(b)
	kc, _ := zx.nextCacheKeyFor(c)
	if d := ka.Diff(kb); len(d) != 0 {
		t.Fatalf("two cards with the same firmware differ in %v", d)
	}
	if d := ka.Diff(kc); len(d) != 1 || d[0] != "firmware file TBBLUE.FW" {
		t.Fatalf("a changed firmware file: %v", d)
	}
	if ka.Firmware.Digest == kc.Firmware.Digest || len(ka.Firmware.Files) != len(nextcard.BootFiles) {
		t.Fatalf("digest %s / %s with %d files", ka.Firmware.Digest, kc.Firmware.Digest, len(ka.Firmware.Files))
	}

	// Files on the card that are not firmware do not reach the key.
	if err := nextcard.AddTree(context.Background(), a.Card, []nextcard.File{{Name: "games/new.nex", Data: []byte("x")}}, nil); err != nil {
		t.Fatal(err)
	}
	kd, _ := zx.nextCacheKeyFor(a)
	if d := ka.Diff(kd); len(d) != 0 {
		t.Fatalf("a file added to the card changed the key: %v", d)
	}
	// A changed boot ROM does.
	os.WriteFile(b.BootROM, bytes.Repeat([]byte{9}, 8192), 0o644)
	ke, _ := zx.nextCacheKeyFor(b)
	if d := ka.Diff(ke); len(d) != 1 || d[0] != "the boot ROM" {
		t.Fatalf("a changed boot ROM: %v", d)
	}
	// A card without a firmware file has no key at all.
	os.Remove(c.Card)
	os.Rename(a.Card, c.Card)
	if _, err := zx.nextCacheKeyFor(nextcard.Files{BootROM: a.BootROM, DivMMCROM: a.DivMMCROM, Card: filepath.Join(t.TempDir(), "none")}); err == nil {
		t.Fatal("a key was made for a card that is not there")
	}
}

func TestNextBootCacheOffDoesNothing(t *testing.T) {
	said := withCache(t, nextCacheOff, t.TempDir())
	zx := chainNext(t, fakeChain(t, sameContent))
	if zx.next.cachePending || zx.onNextHandOver != nil {
		t.Fatal("an off cache is armed")
	}
	zx.RunFrame()
	if len(*said) != 0 {
		t.Fatalf("said %v", *said)
	}
}

// cacheFrom saves what pokeNext makes of a machine on a chain, as a cache of that chain.
func cacheFrom(t *testing.T, f nextcard.Files, dir string, stage nextcache.Stage, edit func(*nextcache.Key)) (string, *ZenZX) {
	t.Helper()
	zx := chainNext(t, f)
	pokeNext(zx)
	key, err := zx.nextCacheKeyFor(f)
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(&key)
	}
	state, err := zx.NextStateBytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := nextcache.Write(dir, nextcache.Entry{Key: key, Stage: stage, Frames: 428, State: state}); err != nil {
		t.Fatal(err)
	}
	return dumpOf(t, zx), zx
}

func TestNextStartsFromACacheForItsFirmware(t *testing.T) {
	cacheDir := t.TempDir()
	f := fakeChain(t, sameContent)
	want, _ := cacheFrom(t, f, cacheDir, nextcache.HandOver, nil)

	said := withCache(t, nextCacheAuto, cacheDir)
	zx := chainNext(t, f)
	if !zx.next.cachePending {
		t.Fatal("the cache is not armed")
	}
	fresh := dumpOf(t, zx)
	zx.serviceNextReset() // what the first frame does first
	if zx.next.cachePending {
		t.Fatal("still pending after the first frame boundary")
	}
	if !sayHas(said, "Next started from the hand-over cache: 428 frames of boot skipped") {
		t.Fatalf("said %v", *said)
	}
	got := dumpOf(t, zx)
	if got == fresh {
		t.Fatal("nothing was put back")
	}
	if got != want {
		t.Fatalf("not the machine that was saved: %s", firstDifference(want, got))
	}
	if zx.onNextHandOver != nil {
		t.Fatal("a hand-over cache was armed on a machine that started from one")
	}
	if zx.nextSettle == nil {
		t.Fatal("no cache of a settled machine was asked for, though there is none")
	}
}

func TestNextPrefersTheSettledCacheAndFallsBackToTheHandOverOne(t *testing.T) {
	cacheDir := t.TempDir()
	f := fakeChain(t, sameContent)
	cacheFrom(t, f, cacheDir, nextcache.HandOver, nil)
	// The settled one is a different machine, so that the test can tell which was used.
	settledZX := chainNext(t, f)
	pokeNext(settledZX)
	settledZX.cpu.PC = 0x4321
	key, _ := settledZX.nextCacheKeyFor(f)
	state, _ := settledZX.NextStateBytes()
	if err := nextcache.Write(cacheDir, nextcache.Entry{Key: key, Stage: nextcache.Settled, Frames: 555, State: state}); err != nil {
		t.Fatal(err)
	}

	said := withCache(t, nextCacheAuto, cacheDir)
	zx := chainNext(t, f)
	zx.serviceNextReset()
	if zx.cpu.PC != 0x4321 || !sayHas(said, "settled cache: 555 frames") {
		t.Fatalf("PC %04X, said %v", zx.cpu.PC, *said)
	}
	if zx.nextSettle != nil {
		t.Fatal("a watch for settling was armed on a machine that started settled")
	}

	// Without the settled one, the hand-over one is used.
	os.Remove(nextcache.Path(cacheDir, key.Firmware, nextcache.Settled))
	*said = nil
	zx = chainNext(t, f)
	zx.serviceNextReset()
	if zx.cpu.PC == 0x4321 || !sayHas(said, "hand-over cache") {
		t.Fatalf("PC %04X, said %v", zx.cpu.PC, *said)
	}
}

func TestNextScratchCardsNeverUseOrMakeASettledCache(t *testing.T) {
	cacheDir := t.TempDir()
	f := fakeChain(t, sameContent)
	cacheFrom(t, f, cacheDir, nextcache.Settled, func(k *nextcache.Key) {})
	said := withCache(t, nextCacheAuto, cacheDir)
	zx := chainNext(t, f)
	zx.next.scratchDir = t.TempDir() // a card made to run one program
	defer func() { zx.next.scratchDir = "" }()
	zx.serviceNextReset()
	if sayHas(said, "settled cache") {
		t.Fatalf("a scratch card used a settled cache: %v", *said)
	}
	if zx.onNextHandOver == nil {
		t.Fatal("no hand-over cache is to be made")
	}
	zx.onNextHandOver()
	if zx.nextSettle != nil {
		t.Fatal("a scratch card is to make a settled cache")
	}
}

func TestEachOutOfDateCacheIsRefusedAndNamed(t *testing.T) {
	f := fakeChain(t, sameContent)
	for name, c := range map[string]struct {
		edit func(*nextcache.Key)
		want string
	}{
		"version": {func(k *nextcache.Key) { k.Version = "0.1.0" }, "the ZenZX version"},
		"layout":  {func(k *nextcache.Key) { k.Layout = "00" }, "the layout of the machine's state"},
		"issue":   {func(k *nextcache.Key) { k.Issue = "2" }, "the board issue"},
		"patches": {func(k *nextcache.Key) { k.Patches = []string{"x"} }, "the ROM patches"},
		"boot":    {func(k *nextcache.Key) { k.BootROM = "00" }, "the boot ROM"},
	} {
		cacheDir := t.TempDir()
		cacheFrom(t, f, cacheDir, nextcache.HandOver, c.edit)
		said := withCache(t, nextCacheAuto, cacheDir)
		zx := chainNext(t, f)
		before := dumpOf(t, zx)
		zx.serviceNextReset()
		if !sayHas(said, "out of date") || !sayHas(said, c.want) {
			t.Errorf("%s: said %v", name, *said)
		}
		if dumpOf(t, zx) != before {
			t.Errorf("%s: a stale cache changed the machine", name)
		}
		if zx.onNextHandOver == nil {
			t.Errorf("%s: no new cache is to be made", name)
		}
	}
}

func TestADamagedCacheIsNotUsed(t *testing.T) {
	cacheDir := t.TempDir()
	f := fakeChain(t, sameContent)
	_, a := cacheFrom(t, f, cacheDir, nextcache.HandOver, nil)
	key, _ := a.nextCacheKeyFor(f)
	path := nextcache.Path(cacheDir, key.Firmware, nextcache.HandOver)
	data, _ := os.ReadFile(path)
	data[len(data)-5] ^= 0xFF
	os.WriteFile(path, data, 0o644)

	said := withCache(t, nextCacheAuto, cacheDir)
	zx := chainNext(t, f)
	before := dumpOf(t, zx)
	zx.serviceNextReset()
	if !sayHas(said, "is not used") || dumpOf(t, zx) != before {
		t.Fatalf("said %v", *said)
	}
}

func TestACacheThatCannotBePutBackStartsTheMachineFromTheFirmware(t *testing.T) {
	cacheDir := t.TempDir()
	f := fakeChain(t, sameContent)
	zx0 := chainNext(t, f)
	key, _ := zx0.nextCacheKeyFor(f)
	// A file that reads back as what was written, and is not a state.
	if err := nextcache.Write(cacheDir, nextcache.Entry{Key: key, Stage: nextcache.HandOver, Frames: 1, State: []byte("not a state, but intact")}); err != nil {
		t.Fatal(err)
	}
	said := withCache(t, nextCacheAuto, cacheDir)
	zx := chainNext(t, f)
	zx.serviceNextReset()
	if !sayHas(said, "could not be put back") || !sayHas(said, "starting from the firmware") {
		t.Fatalf("said %v", *said)
	}
	if _, err := os.Stat(nextcache.Path(cacheDir, key.Firmware, nextcache.HandOver)); err == nil {
		t.Fatal("the cache that failed was kept")
	}
	// The hard reset was asked for and done in the same frame boundary: config mode is back,
	// with the boot ROM mapped, and a new cache is to be made when the firmware is done.
	if zx.io.nextResetRequest != 0 || !zx.memory.nextCfg.active || !zx.memory.bootROMEnabled || zx.onNextHandOver == nil {
		t.Fatalf("the machine was not set to boot from the firmware and make a new cache: request %d, config %v, boot ROM %v",
			zx.io.nextResetRequest, zx.memory.nextCfg.active, zx.memory.bootROMEnabled)
	}
}

func TestRefreshBootsFromTheFirmwareWhateverThereIs(t *testing.T) {
	cacheDir := t.TempDir()
	f := fakeChain(t, sameContent)
	cacheFrom(t, f, cacheDir, nextcache.HandOver, nil)
	said := withCache(t, nextCacheRefresh, cacheDir)
	zx := chainNext(t, f)
	before := dumpOf(t, zx)
	zx.serviceNextReset()
	if sayHas(said, "started from") || dumpOf(t, zx) != before {
		t.Fatalf("a refresh used the cache: %v", *said)
	}
	if zx.onNextHandOver == nil {
		t.Fatal("a refresh makes no new cache")
	}
}

func TestTheCachesAreMadeAtTheHandOverAndWhenSettled(t *testing.T) {
	cacheDir := t.TempDir()
	f := fakeChain(t, sameContent)
	said := withCache(t, nextCacheAuto, cacheDir)
	zx := chainNext(t, f)
	zx.serviceNextReset()
	if zx.onNextHandOver == nil {
		t.Fatal("nothing is to be made at the hand-over")
	}
	pokeNext(zx)
	key, _ := zx.nextCacheKeyFor(f)
	zx.frameCount = 428
	zx.onNextHandOver() // what serviceNextReset does when the firmware is done
	e, err := nextcache.Read(cacheDir, key, nextcache.HandOver)
	if err != nil || e.Frames != 428 || len(e.State) == 0 {
		t.Fatalf("hand-over cache: %v %+v", err, e)
	}
	if zx.nextSettle == nil {
		t.Fatal("no watch for settling after the hand-over")
	}
	zx.frameCount = 555
	zx.nextSettle.onSettled(555)
	if e, err := nextcache.Read(cacheDir, key, nextcache.Settled); err != nil || e.Frames != 555 {
		t.Fatalf("settled cache: %v", err)
	}
	if !sayHas(said, "(hand-over) made at frame 428") || !sayHas(said, "(settled) made at frame 555") {
		t.Fatalf("said %v", *said)
	}
	// Giving up is said, and leaves the hand-over cache.
	zx.nextSettle.onGaveUp()
	if !sayHas(said, "did not come to rest") {
		t.Fatalf("said %v", *said)
	}
}
