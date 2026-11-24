//go:build headless

package zenzx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestNextZXOSBoots runs the whole Next boot chain against the real firmware: the boot ROM
// loads TBBLUE.FW from the card, the firmware loads the ROMs into SRAM and resets into the
// machine, and NextZXOS starts from the card and shows its welcome screen, then its menu.
//
// Dormant guard G-04 (docs/KNOWN_ISSUES.md): it needs three third-party files that are not
// in the tree and cannot be. ZENZX_NEXT_FILES names the folder that holds them:
//
//	nextboot.rom    the boot ROM
//	enNxtmmc.rom    the DivMMC ROM of the distribution the card was made from
//	next-card.img   the card, made by scripts/next_card.py
//
//	ZENZX_NEXT_FILES=thirdparty go test -tags headless -run TestNextZXOSBoots -v .
//
// Skipped when the variable is not set. The card is opened read-only.
func TestNextZXOSBoots(t *testing.T) {
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
	zx := NewZenZX(AudioBackendOto)
	if err := loadModelROM(zx, "next"); err != nil {
		t.Fatal(err)
	}
	zx.memory.SetBootROM(read("nextboot.rom"))
	card, err := os.Open(filepath.Join(dir, "next-card.img"))
	if err != nil {
		t.Fatal(err)
	}
	defer card.Close()
	info, err := card.Stat()
	if err != nil {
		t.Fatal(err)
	}
	zx.EnableDivMMC(card, info.Size(), read("enNxtmmc.rom"))
	zx.cpu.Reset()
	zx.running = true

	run := func(frames int) {
		for i := 0; i < frames; i++ {
			zx.RunFrame()
		}
	}
	waitFor := func(what string, limit int, found func() bool) {
		for f := 0; f < limit; f += 50 {
			run(50)
			if found() {
				t.Logf("%s after %d frames", what, zx.frameCount)
				return
			}
		}
		t.Fatalf("%s not reached within %d frames", what, limit)
	}
	// The screen is told apart by what the display RAM holds, not by reading glyphs:
	// NextZXOS draws with its own fonts, which the screen reader does not know.
	ramHas := func(text string) func() bool {
		needle := []byte(text)
		return func() bool {
			for page := 0; page < nextTotalPages; page++ {
				var buf [8192]byte
				for o := range buf {
					buf[o] = zx.memory.nextPageRead(uint8(page), uint16(o))
				}
				if bytes.Contains(buf[:], needle) {
					return true
				}
			}
			return false
		}
	}
	// cyanCells counts attribute cells with cyan paper in character rows 4 to 15:
	// the highlighted line of the main menu, and nothing on the welcome screen.
	cyanCells := func() int {
		n := 0
		for cell := 4 * 32; cell < 16*32; cell++ {
			if zx.memory.screen.attributes[cell]&0x38 == 0x28 {
				n++
			}
		}
		return n
	}
	waitFor("firmware chose the machine and left config mode", 4000, func() bool {
		return zx.memory.nextCfg.machineType == 3 && !zx.memory.nextCfg.active
	})
	expectFrames(t, "the firmware leaving config mode", zx.frameCount, 150, 1500)
	if zx.memory.bootROMEnabled {
		t.Fatal("the boot ROM is still mapped after the hand-over")
	}
	waitFor("NextZXOS loaded its welcome guide from the card", 3000, ramHas("NextZXOS is an evolution of the system software"))
	expectFrames(t, "NextZXOS loading its welcome guide", zx.frameCount, 150, 1700)
	run(200)
	if n := cyanCells(); n != 0 {
		t.Fatalf("%d cyan cells on what should be the welcome screen", n)
	}
	checkScreenLooksLikeAMenu(t, zx.DecodeDisplay(), "the welcome screen")

	// SPACE leaves the welcome screen for the main menu.
	zx.io.PressKey(7, 0)
	run(10)
	zx.io.ReleaseKey(7, 0)
	waitFor("NextZXOS main menu", 2000, func() bool { return cyanCells() >= 8 })
	expectFrames(t, "the NextZXOS main menu", zx.frameCount, 300, 2500)
	run(50)
	checkScreenLooksLikeAMenu(t, zx.DecodeDisplay(), "the main menu")
	checkScreenIsSteady(t, zx, "the main menu")
}
