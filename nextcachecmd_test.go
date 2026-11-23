//go:build headless

package zenzx

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcache"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

func makeCacheFor(t *testing.T, dir string, zx *ZenZX, chainDir string, stage nextcache.Stage) nextcache.Key {
	t.Helper()
	key, err := zx.nextCacheKeyFor(nextcard.In(chainDir))
	if err != nil {
		t.Fatal(err)
	}
	if err := nextcache.Write(dir, nextcache.Entry{Key: key, Stage: stage, Frames: 428, State: []byte("state")}); err != nil {
		t.Fatal(err)
	}
	return key
}

// -next-firmware check says, for each stage, whether the cache is good, and what differs when
// it is not.
func TestNextFirmwareCheckReportsTheCaches(t *testing.T) {
	cache := t.TempDir()
	withCache(t, nextCacheAuto, cache)
	chain := fakeChain(t, sameContent)

	var none bytes.Buffer
	reportNextCaches(&none, chain.Dir)
	if !strings.Contains(none.String(), "none yet") {
		t.Fatalf("no caches:\n%s", none.String())
	}

	zx := bareNext(t)
	makeCacheFor(t, cache, zx, chain.Dir, nextcache.HandOver)
	makeCacheFor(t, cache, zx, chain.Dir, nextcache.Settled)
	var ok bytes.Buffer
	reportNextCaches(&ok, chain.Dir)
	for _, want := range []string{"settled   valid: 428 frames", "hand-over valid: 428 frames"} {
		if !strings.Contains(ok.String(), want) {
			t.Fatalf("want %q in:\n%s", want, ok.String())
		}
	}

	// The DivMMC ROM changes: the same firmware, another key.
	if err := os.WriteFile(chain.DivMMCROM, bytes.Repeat([]byte{7}, 8192), 0o644); err != nil {
		t.Fatal(err)
	}
	var stale bytes.Buffer
	reportNextCaches(&stale, chain.Dir)
	if strings.Count(stale.String(), "out of date, differs in: the DivMMC ROM") != 2 {
		t.Fatalf("a changed DivMMC ROM:\n%s", stale.String())
	}
}

// cleanup, and setup, remove the caches of firmware that is no longer on this computer, and
// leave those a start can use.
func TestNextFirmwarePrunesCachesNothingCanUse(t *testing.T) {
	cache := t.TempDir()
	withCache(t, nextCacheAuto, cache)
	saved := nextFilesUsable
	nextFilesUsable = func(string) error { return nil }
	t.Cleanup(func() { nextFilesUsable = saved })

	here := fakeChain(t, sameContent)
	gone := fakeChain(t, func(n string) string { return "old " + n })
	zx := bareNext(t)
	makeCacheFor(t, cache, zx, here.Dir, nextcache.HandOver)
	makeCacheFor(t, cache, zx, here.Dir, nextcache.Settled)
	makeCacheFor(t, cache, zx, gone.Dir, nextcache.HandOver)
	os.WriteFile(cache+"/damaged"+nextcache.Ext, []byte("junk"), 0o644)

	var out bytes.Buffer
	if err := pruneNextCaches(here.Dir, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Deleted 2 boot cache file(s)") {
		t.Fatalf("got %q", out.String())
	}
	left := nextcache.List(cache)
	if len(left) != 2 {
		t.Fatalf("%d caches left", len(left))
	}
	var again bytes.Buffer
	if err := pruneNextCaches(here.Dir, &again); err != nil || again.Len() != 0 {
		t.Fatalf("a second prune: %q %v", again.String(), err)
	}
}
