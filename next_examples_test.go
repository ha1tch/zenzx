//go:build headless

package zenzx

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/ha1tch/zenzx/pkg/nextcard"
)

// The examples are in the program, and each is a NEX file.
func TestNextExamplesAreEmbedded(t *testing.T) {
	files, err := nextExamples()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("%d examples, want 3", len(files))
	}
	for _, f := range files {
		if len(f.Data) < 512 || string(f.Data[:8]) != "NextV1.2" {
			t.Errorf("%s: %d bytes, starts %q; want a NEX V1.2 file (the card's .nexload refuses V1.3)", f.Name, len(f.Data), f.Data[:8])
		}
	}
}

// TestNextZXOSRunsTheExamples makes ZenZX's own card with the examples on it, as
// -next-examples does, boots NextZXOS with it in the second slot, and starts each example
// from the command line. An example passes when it has set the Next up as it means to and
// its picture is rich and moving.
//
// Dormant guard G-06 (docs/KNOWN_ISSUES.md): gated as G-04 is, on ZENZX_NEXT_FILES. The
// files there are not changed: the boot card is opened read-only and the second card is
// made in a temporary folder.
func TestNextZXOSRunsTheExamples(t *testing.T) {
	dir := os.Getenv("ZENZX_NEXT_FILES")
	if dir == "" {
		t.Skip("ZENZX_NEXT_FILES not set (dormant guard G-06)")
	}
	examples, err := nextExamples()
	if err != nil {
		t.Fatal(err)
	}
	ownCard := filepath.Join(t.TempDir(), ownCardName)
	if err := addToOwnCard(ownCard, examples); err != nil {
		t.Fatal(err)
	}
	cardPath := filepath.Join(dir, nextcard.CardName)
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return data
	}

	checks := map[string]func(zx *ZenZX) string{
		"gallop.nex": func(zx *ZenZX) string {
			r := &zx.io.nextRegs.regs
			if zx.io.copper.mode != 3 || r[0x6B] != 0xA9 || r[0x15] != 0x01 {
				return "the copper, the tilemap's text mode or the sprites are not as gallop sets them"
			}
			return ""
		},
		"fuji.nex": func(zx *ZenZX) string {
			r := &zx.io.nextRegs.regs
			if zx.io.copper.mode != 3 || r[0x70] != 0x10 || r[0x6B] != 0x83 || r[0x15] != 0x0B {
				return "the copper, Layer 2 at 320x256, the tilemap or the layer order are not as fuji sets them"
			}
			return ""
		},
		"orbit.nex": func(zx *ZenZX) string {
			if !zx.io.nextInt.hw || zx.cpu.IM != 2 {
				return "the Next's interrupt mode is not on"
			}
			return ""
		},
	}

	for _, ex := range examples {
		t.Run(ex.Name, func(t *testing.T) {
			zx := NewZenZX(AudioBackendOto)
			if err := loadModelROM(zx, "next"); err != nil {
				t.Fatal(err)
			}
			zx.memory.SetBootROM(read(nextcard.BootROMName))
			card, err := os.Open(cardPath)
			if err != nil {
				t.Fatal(err)
			}
			defer card.Close()
			info, err := card.Stat()
			if err != nil {
				t.Fatal(err)
			}
			zx.EnableDivMMC(card, info.Size(), read(nextcard.DivMMCROMName))
			zx.io.divMMC.SD().wr.WriteProtected = true
			if _, err := zx.InsertSecondCard(ownCard, true); err != nil {
				t.Fatal(err)
			}
			defer zx.RemoveSecondCard()
			zx.cpu.Reset()
			zx.running = true
			run := func(frames int) {
				for i := 0; i < frames; i++ {
					zx.RunFrame()
				}
			}
			key := func(names ...string) {
				var chord []matrixPos
				for _, n := range names {
					if c, ok := namedKeys[n]; ok {
						chord = append(chord, c...)
					} else if pos, ok := baseKeys[[]rune(n)[0]]; ok {
						chord = append(chord, pos)
					} else {
						t.Fatalf("no key %q", n)
					}
				}
				for _, k := range chord {
					zx.io.PressKey(k.row, k.col)
				}
				run(6)
				for _, k := range chord {
					zx.io.ReleaseKey(k.row, k.col)
				}
				run(8)
			}
			symbols := map[rune]string{'"': "p", '.': "m", '/': "v", ':': "z"}
			typeLine := func(s string) {
				for _, c := range s {
					switch {
					case symbols[c] != "":
						key("sym", symbols[c])
					case c == ' ':
						key("space")
					default:
						key(string(c))
					}
				}
				key("enter")
			}
			run(3500) // to the welcome screen
			key("space")
			run(150) // the menu
			key("down")
			run(40)
			key("enter")
			run(150)            // the command line
			typeLine(`cd "d:"`) // the second card's drive
			run(100)
			typeLine(".nexload " + ex.Name)
			run(300)

			if why := checks[ex.Name](zx); why != "" {
				t.Error(why)
			}
			first := zx.DecodeDisplay()
			run(40)
			second := zx.DecodeDisplay()
			if n := countColours(first); n < 40 {
				t.Errorf("the picture has %d colours; an example's has many more", n)
			}
			if string(first.Pix) == string(second.Pix) {
				t.Error("the picture did not change in 40 frames")
			}
		})
	}
}

func countColours(img *image.RGBA) int {
	seen := map[[3]uint8]bool{}
	for i := 0; i+3 < len(img.Pix); i += 4 {
		seen[[3]uint8{img.Pix[i], img.Pix[i+1], img.Pix[i+2]}] = true
	}
	return len(seen)
}
