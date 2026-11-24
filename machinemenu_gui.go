//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "github.com/ha1tch/zenui"

// The head of the Machine menu: Reset, then Pause or Play, then Stop, and after them the model
// rows (machines.json). The second row says what it will do, not what it is: Pause while the
// machine runs, Play while it is paused, each with the transport sprite the debugger and the
// paused machine's button use. Stop ends the machine (the deck's stompStop, machinedeck_gui.go);
// with no machine to act on, all three are disabled.

// machineHeadRows is how many rows come before the model rows.
const machineHeadRows = 3

// transportIcons are the transport sprites, parsed once.
var transportIcons = parseTransportSprites()

// transportIcon is the Item.Icon that draws a transport sprite ("play", "pause", "stop") in the
// cell a menu row gives it: whole pixels, as large as fit, centred, in the row's own colour.
func transportIcon(name string) func(r zenui.Renderer, cell zenui.Rect, col zenui.Colour) {
	icon, ok := transportIcons[name]
	return func(r zenui.Renderer, cell zenui.Rect, col zenui.Colour) {
		if !ok {
			return
		}
		s := cell.W / 16
		if s < 1 {
			s = 1
		}
		x0, y0, x1, y1 := iconBounds(icon)
		offX := cell.X + (cell.W-(x1-x0)*s)/2
		offY := cell.Y + (cell.H-(y1-y0)*s)/2
		for _, run := range iconRuns(icon) {
			if !run.ink {
				continue
			}
			r.FillRect(zenui.Rect{X: offX + (run.x-x0)*s, Y: offY + (run.y-y0)*s, W: run.n * s, H: s}, col)
		}
	}
}

// machineHead is the first rows of the Machine menu as zx is now.
func (b *appMenuBar) machineHead(zx *ZenZX) []zenui.Item {
	label, sprite := "Pause", "pause"
	if zx.paused {
		label, sprite = "Play", "play"
	}
	off := zx.stopped || zx.stopping
	return []zenui.Item{
		{Label: "Reset", Disabled: off},
		{Label: label, Icon: transportIcon(sprite), Disabled: off},
		{Label: "Stop", Icon: transportIcon("stop"), Disabled: off},
	}
}

// refreshMachineItems puts the Machine menu's head rows right for the machine as it is now,
// each frame the bar is shown (the way the View menu is: Item.Disabled and a label have no
// pointer to follow, and F2 or the Play button changes the pause from outside the menu).
func (b *appMenuBar) refreshMachineItems(zx *ZenZX) {
	items := append(b.machineHead(zx), b.machineGroupItems...)
	b.widget.SetItems(barMachine, items)
}

// needsMachine says the menu row chosen acts on the machine, so that with none running it has
// nothing to act on: the File menu's Open and Export, and every row of the menus that are about
// the machine's tape, discs, ROMs and snapshots. The Machine menu's own head rows are disabled
// then, and its model rows start a machine.
func (b *appMenuBar) needsMachine(sel zenui.MenuBarSelection) bool {
	switch sel.BarIndex {
	case barFile:
		return sel.ItemIndex <= 1 // Open, Export
	case barMachine:
		return sel.ItemIndex < machineHeadRows
	case barCustomROM, barTape, barFloppyDisk, barSnapshot:
		return true
	}
	return false
}

// pickMachineModel carries out a choice among the Machine menu's model rows: the running
// machine becomes that model, or, when none is running, a machine of it is started.
func (b *appMenuBar) pickMachineModel(zx *ZenZX, sel zenui.MenuBarSelection) {
	var flag string
	if sub, ok := b.machineSubmenuFlags[sel.ItemIndex]; ok {
		if sel.SubIndex >= 0 && sel.SubIndex < len(sub) {
			flag = sub[sel.SubIndex]
		}
	} else if sel.ItemIndex < len(b.machineModelFlags) {
		flag = b.machineModelFlags[sel.ItemIndex]
	}
	if flag == "next" && b.needsOwnCard(zx) {
		return // the SD Cards window is open: the machine becomes a Next when a card is chosen
	}
	if flag != "" && zx.stopped && b.startMachine != nil {
		// None is running: the model is the machine that is started.
		if nz := b.startMachine(flag); nz != nil {
			if flag == "next" {
				b.startNext(nz)
			}
			return
		}
	}
	if flag != "" {
		b.switchModel(zx, flag)
		b.setCurrentModel(flag)
	}
}
