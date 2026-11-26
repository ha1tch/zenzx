//go:build !headless

package zenzx

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/ha1tch/zenui"
)

// minRowsFor50ROMs is the fewest rows a dropdown has for the ROMs of TestGLCustomROMMenuFitsTheWindow
// (fifty and two folders) to fit it, found by running it: at ten rows they do not.
const minRowsFor50ROMs = 13

// TestGLCustomROMMenuFitsTheWindow lays out the Custom ROM dropdown, and the submenu of each of
// its rows, in windows of several sizes and at each text size, with more ROMs and folders than any
// of them has room for, and checks that every row of every one of them is inside the window:
// the cutting of buildROMRows (pages of a folder, "More...") is meant to keep them there, and
// romRowsFit's row height is the one zenui gives a row.
//
// Part of dormant guard G-26 (docs/KNOWN_ISSUES.md): it opens a window.
//
//	DISPLAY=:99 LIBGL_ALWAYS_SOFTWARE=1 ZENZX_GL_TESTS=1 go test -run TestGLCustomROMMenuFitsTheWindow -v .
func TestGLCustomROMMenuFitsTheWindow(t *testing.T) {
	if os.Getenv("ZENZX_GL_TESTS") == "" {
		t.Skip("ZENZX_GL_TESTS is not set: this test opens a window (dormant guard G-26)")
	}
	runtime.LockOSThread() // not unlocked: the window's cleanup below runs after this function
	rl.SetTraceLogLevel(rl.LogNone)
	rl.InitWindow(1024, 768, "ZenZX custom ROM menu")
	t.Cleanup(rl.CloseWindow) // registered first, so it runs after the bar's text is unloaded

	b, zx := newTestBar(t, nil, "")
	b.romDir = t.TempDir()
	for i := 0; i < 20; i++ {
		touchROMs(t, b.romDir, fmt.Sprintf("n%02d.rom", i))
	}
	for f := 0; f < 2; f++ {
		for i := 0; i < 10+f*8; i++ {
			touchROMs(t, b.romDir, fmt.Sprintf("folder%d/r%02d.rom", f, i))
		}
		touchROMs(t, b.romDir, fmt.Sprintf("folder%d/inner/in%d.rom", f, f))
	}
	want := len(listCustomROMs(b.romDir))

	for _, zoom := range []int{1, 2, 3} {
		b.ApplyZoom(zoom)
		for _, sz := range [][2]int{{1024, 768}, {800, 500}, {640, 400}, {320, 240}} {
			w, h := sz[0], sz[1]
			rt := rl.LoadRenderTexture(int32(w), int32(h))
			rl.BeginTextureMode(rt)
			rl.ClearBackground(rl.Gray)
			// the window the program has is the one the test has: its own size is what romRowsFit reads
			rl.SetWindowSize(w, h)
			rl.EndTextureMode()
			b.refreshCustomROMItems(zx)
			items := b.widget.ItemsFor(barCustomROM)

			name := fmt.Sprintf("%dx%d at text size %d (fit %d rows)", w, h, zoom, b.romRowsFit())
			if b.romRowsFit() < minRowsFor50ROMs {
				// one level of submenu is all zenui has: fifty ROMs and two folders do not fit
				// a dropdown of fewer rows than this, and the rest would run below the window
				t.Logf("%s: fewer than %d rows, not checked", name, minRowsFor50ROMs)
				rl.UnloadRenderTexture(rt)
				continue
			}
			anchor := zenui.Rect{X: 0, Y: 0, W: 100, H: b.stripH()}
			top := zenui.NewMenu(zenui.MenuConfig{Items: items, Anchor: anchor, Scale: b.scale})
			rl.BeginTextureMode(rt)
			top.Draw(b.renderer, w, h, b.theme)
			rl.EndTextureMode()
			if r := top.ItemRect(len(items) - 1); r.Y+r.H > h {
				t.Errorf("%s: the dropdown ends at %d, below the window", name, r.Y+r.H)
			}

			reached := 0
			for k, it := range items {
				if len(it.SubItems) == 0 {
					if !it.Separator && it.Label != "Download more ROMs" {
						reached++
					}
					continue
				}
				if it.Label == "Download more ROMs" {
					continue
				}
				reached += len(it.SubItems)
				row := top.ItemRect(k)
				sub := zenui.NewMenu(zenui.MenuConfig{Items: it.SubItems,
					Anchor: zenui.Rect{X: row.X + row.W - 4, Y: row.Y}, Scale: b.scale})
				rl.BeginTextureMode(rt)
				sub.Draw(b.renderer, w, h, b.theme)
				rl.EndTextureMode()
				if r := sub.ItemRect(len(it.SubItems) - 1); r.Y+r.H > h {
					t.Errorf("%s: the submenu of %q (row %d) ends at %d, below the window", name, it.Label, k, r.Y+r.H)
				}
			}
			if reached != want {
				t.Errorf("%s: %d ROMs reachable, want %d", name, reached, want)
			}
			rl.UnloadRenderTexture(rt)
		}
	}
}
