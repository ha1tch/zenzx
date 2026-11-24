//go:build headless

package zenzx

import (
	"os"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// TestNextZXOSStartUpDoesNotNameTheMachine boots NextZXOS from the real card
// (dormant guard G-04's files): the system reads BASIC of its own while it
// starts, and none of that is a program the machine was asked to load.
func TestNextZXOSStartUpDoesNotNameTheMachine(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-04)")
	}
	zx := NewZenZX(AudioBackendOto)
	if err := loadModelROM(zx, "next"); err != nil {
		t.Fatal(err)
	}
	if _, err := zx.attachNextBootChain(nextcard.In(dir), true, false); err != nil {
		t.Fatal(err)
	}
	defer zx.detachNextBootChain()
	zx.cpu.Reset()
	zx.running = true
	seen := 0
	for i := 0; i < 1500; i++ {
		zx.RunFrame()
		seen += len(zx.io.divMMC.SD().programAt)
		if got := zx.FirstProgram(); got != "" {
			t.Fatalf("frame %d: starting NextZXOS named the machine %q", i, got)
		}
	}
	t.Logf("BASIC programs the system read from the card while starting: %d", seen)
}
