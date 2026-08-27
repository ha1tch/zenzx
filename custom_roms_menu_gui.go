//go:build !headless

package main

import (
	"fmt"
	"image/color"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/zenui"
	"github.com/ha1tch/zenzx/pkg/zenuiraylib"
)

// toNRGBA converts a zenui.Colour to the image/color.NRGBA
// zenuiraylib.NewBDFText's ink parameter needs -- glyphs are rasterised
// into image.RGBA cells (pkg/bdf) before anything raylib-specific enters
// the picture.
func toNRGBA(c zenui.Colour) color.NRGBA { return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A} }

// runMenuLoop drives one zenui.Menu to a decision, drawing and polling
// input once per frame until the person picks an item, cancels, or closes
// the window outright. Returns (-1, false) for cancel or window-close,
// (index, true) for a genuine pick.
func runMenuLoop(menu *zenui.Menu, renderer zenui.Renderer, theme zenui.Theme) (int, bool) {
	for !rl.WindowShouldClose() {
		screenW, screenH := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		menu.Draw(renderer, screenW, screenH, theme)
		rl.EndDrawing()

		switch menu.Update(zenuiraylib.Input()) {
		case zenui.Accepted:
			return menu.Result(), true
		case zenui.Cancelled:
			return -1, false
		}
	}
	return -1, false
}

// runGraphicalCustomROMMenu is the GUI build's counterpart to
// runCustomROMMenu (custom_roms_menu.go, used by the headless build, which
// has no window to draw a menu into). Same two-step shape -- pick a ROM,
// then (for multi-bank models) pick which bank -- driven by zenui.Menu and
// drawn through the Sinclair bitmap face. Silently does nothing if dir has
// no .rom files, matching runCustomROMMenu's own "nothing to select isn't
// a failure" stance.
func runGraphicalCustomROMMenu(zx *ZenZX, dir string) {
	names := listCustomROMs(dir)
	if len(names) == 0 {
		fmt.Printf("No .rom files found in %s -- nothing to select, keeping the standard ROM set.\n", dir)
		return
	}

	face, err := fonts.Sinclair()
	if err != nil {
		fmt.Printf("Warning: could not load the Sinclair font for the ROM menu (%v) -- skipping custom ROM selection.\n", err)
		return
	}
	theme := zenui.DefaultTheme()
	text := zenuiraylib.NewBDFText(face)
	defer text.Unload()
	renderer := zenuiraylib.Renderer{Text: text}

	items := make([]zenui.Item, 0, len(names)+1)
	for _, n := range names {
		items = append(items, zenui.Item{Label: n})
	}
	items = append(items, zenui.Item{Label: "Skip -- keep the standard ROM set"})

	romMenu := zenui.NewMenu(zenui.MenuConfig{Items: items})
	choice, ok := runMenuLoop(romMenu, renderer, theme)
	if !ok || choice == len(names) { // cancelled, or the trailing "Skip" row
		fmt.Println("Skipped -- keeping the standard ROM set.")
		return
	}

	selected := names[choice]
	fullPath := filepath.Join(dir, selected)

	bank := 0
	if maxBank := zx.maxROMBank(); maxBank > 0 {
		bankItems := make([]zenui.Item, maxBank+1)
		for i := range bankItems {
			bankItems[i] = zenui.Item{Label: fmt.Sprintf("Bank %d", i)}
		}
		bankMenu := zenui.NewMenu(zenui.MenuConfig{Items: bankItems})
		b, ok := runMenuLoop(bankMenu, renderer, theme)
		if !ok {
			fmt.Println("Bank selection cancelled -- keeping the standard ROM set.")
			return
		}
		bank = b
	}

	if err := zx.OverrideROMBank(bank, fullPath); err != nil {
		fmt.Printf("Warning: could not apply %s to bank %d: %v\n", selected, bank, err)
		return
	}
	fmt.Printf("Loaded %s into ROM bank %d.\n", selected, bank)

	showConfirmationOSD(fmt.Sprintf("Loaded %s -> bank %d", selected, bank), text)
}

// showConfirmationOSD plays the rising, fading caption to confirm a ROM
// selection landed, reusing the same font/renderer the menu itself just
// drew with rather than loading a second copy. Runs its own short frame
// loop until the animation finishes on its own (OSD.Active()) or the
// window closes -- there's nothing else competing for the frame here, the
// menu loop has already returned.
func showConfirmationOSD(message string, text *zenuiraylib.BDFText) {
	caption := zenuiraylib.NewOSD()
	caption.Start(message)

	tw := text.Measure(message, 2)
	th := text.CellH() * 2

	for caption.Active() && !rl.WindowShouldClose() {
		screenW, screenH := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
		dt := rl.GetFrameTime()

		caption.Update(dt, 0, message, tw, th)

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		caption.Draw(text, screenW-10, screenH-10, tw, th)
		rl.EndDrawing()
	}
}
