//go:build !headless

package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"

	"github.com/ha1tch/zenzx/pkg/fonts"
	"github.com/ha1tch/zenzx/pkg/zenui"
	"github.com/ha1tch/zenzx/pkg/zenuiraylib"
)

// demoOverlayKind identifies which widget (if any) demoOverlay currently
// owns the frame's input. Exactly one can be active at a time -- this is a
// demo harness, not a stacked window manager.
type demoOverlayKind int

const (
	demoOverlayNone demoOverlayKind = iota
	demoOverlayMenu
	demoOverlayDialog
	demoOverlayNotification
)

// demoOverlay owns whichever bogus GUI element is currently shown over the
// running emulator, for visually confirming the widget system genuinely
// works layered on top of live emulation rather than only at startup
// (custom_roms_menu_gui.go's menus, by contrast, block the whole frame
// loop -- CPU stepping stops while they're open, which is fine for a
// startup-only picker but not what's wanted here).
//
// The emulator keeps running every frame regardless of what demoOverlay is
// doing -- RunFrame/Render are never gated on it. Only keyboard input to
// the emulated machine is gated: the main loop skips zx.HandleInput
// entirely while Active() is true, so the Spectrum receives no keystrokes
// while a demo widget owns the frame.
type demoOverlay struct {
	kind demoOverlayKind

	text     *zenuiraylib.BDFText
	renderer zenuiraylib.Renderer
	theme    zenui.Theme

	menu   *zenui.Menu
	dialog *zenui.Dialog
	toast  *zenuiraylib.OSD
}

// newDemoOverlay builds an idle overlay, loading the Sinclair face once so
// every demo widget shares the same glyph texture cache rather than each
// paying its own upload cost.
func newDemoOverlay() (*demoOverlay, error) {
	face, err := fonts.Sinclair()
	if err != nil {
		return nil, fmt.Errorf("loading Sinclair font for demo overlay: %w", err)
	}
	theme := zenui.DefaultTheme()
	text := zenuiraylib.NewBDFText(face)
	return &demoOverlay{
		text:     text,
		renderer: zenuiraylib.Renderer{Text: text},
		theme:    theme,
		toast:    zenuiraylib.NewOSD(),
	}, nil
}

// Active reports whether the overlay currently owns input -- the main loop
// checks this to decide whether to call zx.HandleInput this frame.
func (d *demoOverlay) Active() bool { return d.kind != demoOverlayNone }

// TriggerMenu opens a bogus dropdown menu, anchored near the top-left, to
// demonstrate zenui.Menu drawn live over the running emulator.
func (d *demoOverlay) TriggerMenu() {
	d.menu = zenui.NewMenu(zenui.MenuConfig{
		Anchor: zenui.Rect{X: 40, Y: 40, W: 120, H: 20},
		Items: []zenui.Item{
			{Label: "Bogus Item One"},
			{Label: "Bogus Item Two"},
			{Label: "Disabled Item", Disabled: true},
			{Label: "Close Menu"},
		},
	})
	d.kind = demoOverlayMenu
}

// TriggerDialog opens a bogus file-open dialog rooted at the current
// working directory, to demonstrate zenui.Dialog drawn live over the
// running emulator.
func (d *demoOverlay) TriggerDialog() {
	d.dialog = zenui.NewDialog(zenui.DialogConfig{
		Mode:     zenui.ModeOpen,
		Title:    "Bogus File Dialog",
		StartDir: ".",
		FS:       zenui.OSFS{},
	})
	d.kind = demoOverlayDialog
}

// TriggerNotification plays a bogus animated status caption -- unlike the
// menu and dialog, this one dismisses itself automatically once its
// rise-and-fade animation finishes; there's nothing to accept or cancel.
func (d *demoOverlay) TriggerNotification() {
	d.toast.Start("Bogus notification!")
	d.kind = demoOverlayNotification
}

// Update advances whichever widget is active and clears the overlay once
// it finishes (menu/dialog: Accepted or Cancelled; notification: its
// animation runs out on its own). Must be called exactly once per frame
// regardless of Active(), so the toast's own dt-based animation timer
// stays correct even between triggers.
func (d *demoOverlay) Update(screenW, screenH int) {
	switch d.kind {
	case demoOverlayMenu:
		switch d.menu.Update(zenuiraylib.Input()) {
		case zenui.Accepted, zenui.Cancelled:
			d.kind = demoOverlayNone
			d.menu = nil
		}
	case demoOverlayDialog:
		switch d.dialog.Update(zenuiraylib.Input()) {
		case zenui.Accepted, zenui.Cancelled:
			d.kind = demoOverlayNone
			d.dialog = nil
		}
	case demoOverlayNotification:
		const msg = "Bogus notification!"
		tw := d.text.Measure(msg, 2)
		th := d.text.CellH() * 2
		d.toast.Update(rl.GetFrameTime(), 1, msg, tw, th)
		if !d.toast.Active() {
			d.kind = demoOverlayNone
		}
	}
}

// Draw darkens the emulator screen already rendered this frame, then draws
// whichever widget is active on top of that dimmed backdrop -- zenui's own
// Theme.Backdrop colour, the same dim layer Dialog/Menu already draw
// themselves against when used normally, applied here once so the demo
// reads as one coherent darkened frame rather than the widget having its
// own separate dimming.
func (d *demoOverlay) Draw(screenW, screenH int) {
	if d.kind == demoOverlayNone {
		return
	}
	d.renderer.FillRect(zenui.Rect{X: 0, Y: 0, W: screenW, H: screenH}, d.theme.Backdrop)

	switch d.kind {
	case demoOverlayMenu:
		d.menu.Draw(d.renderer, screenW, screenH, d.theme)
	case demoOverlayDialog:
		d.dialog.Draw(d.renderer, screenW, screenH, d.theme)
	case demoOverlayNotification:
		const msg = "Bogus notification!"
		tw := d.text.Measure(msg, 2)
		th := d.text.CellH() * 2
		d.toast.Draw(d.text, screenW-10, screenH-10, tw, th)
	}
}

// Unload frees the overlay's glyph textures. Call once, before
// rl.CloseWindow.
func (d *demoOverlay) Unload() {
	d.text.Unload()
}
