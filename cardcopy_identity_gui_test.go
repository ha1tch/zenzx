//go:build !headless

package zenzx

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// A card copied in the SD Cards window is a card of its own: it has another signature than the
// original, so a snapshot made over one is not taken to be made over the other.
func TestACopiedCardHasAnIdentityOfItsOwn(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "next-card.img")
	if _, err := nextcard.NewCard(src, nextcard.SmallestCard, nextcard.NewDir("")); err != nil {
		t.Fatal(err)
	}
	was, err := nextcard.IdentityOfFile(src)
	if err != nil || !was.Keyed() {
		t.Fatalf("the card made has no signature: %+v, %v", was, err)
	}

	w := &cardsWindow{dir: dir}
	w.startCopy(cardEntry{path: src, size: was.Size})
	deadline := time.Now().Add(30 * time.Second)
	for {
		w.mu.Lock()
		copied, err := w.copied, w.copyErr
		dst := w.copyDst
		w.mu.Unlock()
		if copied {
			if err != nil {
				t.Fatal(err)
			}
			got, err := nextcard.IdentityOfFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Keyed() || got.Sig == was.Sig {
				t.Errorf("the copy has signature %08x, the original %08x", got.Sig, was.Sig)
			}
			if got.Size != was.Size || got.Table != was.Table {
				t.Errorf("the copy is laid out differently: %+v, %+v", got, was)
			}
			if after, _ := nextcard.IdentityOfFile(src); after != was {
				t.Errorf("the original was changed: %+v", after)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the copy did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
