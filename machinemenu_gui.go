//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "github.com/ha1tch/zenui"

// The head of the Machine menu: a title, then Reset, Play, Pause and Stop, and after them the model
// rows (machines.json). The rows are the same ones whatever the machine is doing, as the Tape
// menu's Play, Stop and Rewind are, and each is enabled only when it can act: Play while the machine
// is paused or stopped (a stopped machine starts again, as the model it was), Pause while it runs,
// Stop and Reset while there is a machine. Reset has the recycle sprite, the others the transport
// sprites the debugger and the paused machine's button use. Stop ends the machine (the deck's
// stompStop, machinedeck_gui.go).

// machineHeadRows is how many rows come before the model rows, and machinePlayRow is the Play
// row among them: the one that acts on a stopped machine.
const (
	machineHeadRows = 5
	machinePlayRow  = 2
)

// transportIcons are the transport sprites, parsed once.
var transportIcons = parseTransportSprites()

// transportIcon is the Item.Icon that draws a transport sprite ("play", "pause", "stop") in the
// cell a menu row gives it: whole pixels, as large as fit, centred, in the row's own colour.
func transportIcon(name string) func(r zenui.Renderer, cell zenui.Rect, col zenui.Colour) {
	icon, ok := transportIcons[name]
	if !ok {
		icon, ok = toolbarIcons[name]
	}
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
	off := zx.stopped || zx.stopping
	return []zenui.Item{
		{Label: "Machine", Title: true},
		{Label: "Reset", Icon: transportIcon("recycle"), Disabled: off},
		{Label: "Play", Icon: transportIcon("play"), Disabled: zx.stopping || !(zx.paused || zx.stopped)},
		{Label: "Pause", Icon: transportIcon("pause"), Disabled: off || zx.paused},
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
// nothing to act on: the File menu's Export, and every row of the menus that are about
// the machine's tape, disks and ROMs, and the rows of the Snapshot menu that save or look into one. The Machine menu's own head rows are disabled
// then, and its model rows start a machine.
func (b *appMenuBar) needsMachine(sel zenui.MenuBarSelection) bool {
	switch sel.BarIndex {
	case barFile:
		// Export acts on the machine. Open does not have to: a snapshot opened makes the machine
		// it is of (the other files say there is none: loadDroppedFile)
		return sel.ItemIndex == 1
	case barMachine:
		// Play acts on a stopped machine too: it starts one
		return sel.ItemIndex < machineHeadRows && sel.ItemIndex != machinePlayRow
	case barCustomROM:
		return !b.isRomSites(sel) // the row of links to ROM pages opens a page, whatever is running
	case barSnapshot:
		// Loading a snapshot makes the machine it is of; the rows that act on a machine need one
		if i := sel.ItemIndex; i >= 0 && i < len(b.snapSpec) {
			return b.snapSpec[i].needsMachine
		}
		return true
	case barTape:
		// the rows that act on the tape in a machine's deck need one: Load Tape, Recent Tapes and the
		// transport and mode rows; Manage Tapes and Autoload Tape do not
		if i := sel.ItemIndex; i >= 0 && i < len(b.tapeMenu) {
			return b.tapeMenu[i].needsMachine
		}
		return true
	case barFloppyDisk:
		return true
	}
	return false
}

// playMachine is the Machine menu's Play row: a paused machine goes on, and with none running
// one is started, of the model the menu has ticked (the one that was stopped), the 48K when none is.
func (b *appMenuBar) playMachine(zx *ZenZX) {
	if !zx.stopped {
		if zx.paused {
			zx.TogglePause()
		}
		return
	}
	flag := b.currentModel()
	if flag == "" {
		flag = "48k"
	}
	if flag == "next" && b.needsOwnCard(zx) {
		return // the SD Cards window is open: the machine becomes a Next when a card is chosen
	}
	if b.startMachine == nil {
		return
	}
	if nz := b.startMachine(flag); nz != nil && flag == "next" {
		b.startNext(nz)
	}
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
