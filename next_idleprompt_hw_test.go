//go:build headless

package zenzx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextAtIdlePromptDetectsMenu checks that nextAtIdlePrompt reports true when
// a Next is sitting at its NextZXOS menu or Browser (both spin on the same
// key-wait idle loop) and false for a non-Next machine. It is the detector the
// drop-choice dialog gates on -- a tape/.scr/snapshot dropped while the Next is at
// an idle prompt offers copy-to-SD vs load-and-run (dormant guard G-11).
func TestNextAtIdlePromptDetectsMenu(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-11)")
	}
	f := nextcard.In(dir)
	boot, err := newEmptyCard(filepath.Join(t.TempDir(), "boot"), smallestCard)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyBootFiles(f.Card, boot); err != nil {
		t.Fatal(err)
	}
	zx, err := newMachine("next", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootNextFrom(zx, f, boot, false, false); err != nil {
		t.Fatal(err)
	}
	defer zx.detachNextBootChain()
	zx.running = true

	// Reach the menu (an idle prompt) and confirm the detector fires there.
	for i := 0; i < 4000; i++ {
		zx.RunFrame()
	}
	atPrompt := false
	for i := 0; i < 60; i++ {
		zx.RunFrame()
		if zx.nextAtIdlePrompt() {
			atPrompt = true
			break
		}
	}
	if !atPrompt {
		t.Error("a Next at its menu should report nextAtIdlePrompt=true")
	}

	// A non-Next machine is never at the Next prompt.
	other, err := newMachine("48k", "./rom")
	if err != nil {
		t.Fatal(err)
	}
	if other.nextAtIdlePrompt() {
		t.Error("a 48K machine should never report nextAtIdlePrompt=true")
	}
}
