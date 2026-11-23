//go:build headless

package zenzx

import (
	"context"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcache"
	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The key's Startup part follows the files NextZXOS reads at start, those the card has of
// nextcard.BootExtras, and nothing else on the card.
func TestNextCacheKeyStartupFollowsTheExtrasOnTheCard(t *testing.T) {
	a := fakeChain(t, sameContent)
	zx := bareNext(t)
	k0, err := zx.nextCacheKeyFor(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(k0.Startup.Files) != len(nextcard.BootFiles) {
		t.Fatalf("a card with no extras: %d start-up files", len(k0.Startup.Files))
	}
	add := func(name, data string) {
		t.Helper()
		if err := nextcard.AddTree(context.Background(), a.Card, []nextcard.File{{Name: name, Data: []byte(data)}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	add("games/new.nex", "x")
	k1, _ := zx.nextCacheKeyFor(a)
	if d := k0.DiffFor(k1, nextcache.Settled); len(d) != 0 {
		t.Fatalf("a game on the card reached the key: %v", d)
	}
	add("nextzxos/autoexec.1st", "welcome")
	k2, _ := zx.nextCacheKeyFor(a)
	if d := k1.DiffFor(k2, nextcache.HandOver); len(d) != 0 {
		t.Fatalf("an extra reached the hand-over's key: %v", d)
	}
	if d := k1.DiffFor(k2, nextcache.Settled); len(d) != 1 || d[0] != "start-up file NEXTZXOS/AUTOEXEC.1ST (not in the cache)" {
		t.Fatalf("an extra added: %v", d)
	}
}
