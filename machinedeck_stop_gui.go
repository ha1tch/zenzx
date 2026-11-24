//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Stopping a machine (Machine > Stop, or Delete on its picture in the dock): the stomp plays
// over it (stomp_gui.go), the machine held where it was, and when it is over the machine is
// gone from the deck and the dock. When it was the last one the window is left with nothing
// but its background: the deck keeps the machine as idle, which runs nothing and draws nothing,
// only so that the window has a front machine to hand to the menus until a machine is started
// (the dock's New, or a model in the Machine menu), which takes its place.

// Stopping says a machine is being stopped, which the window waits for: it takes no keys.
func (d *machineDeck) Stopping() bool { return d != nil && d.stomp != nil }

// Empty says no machine is running: the window shows only its background.
func (d *machineDeck) Empty() bool { return d != nil && len(d.machines) == 0 }

// stopFront stops the machine the window shows.
func (d *machineDeck) stopFront() {
	if len(d.machines) > 0 {
		d.stop(d.front)
	}
}

// stop stops machine i: with the animation when it can be seen (it is in front, or the layout
// shows every machine), without it when it is not on screen.
func (d *machineDeck) stop(i int) {
	if d.stomp != nil || i < 0 || i >= len(d.machines) {
		return
	}
	m := d.machines[i]
	d.hide()
	if (i != d.front && !d.tiled()) || !d.stopAnimates() {
		d.destroy(m)
		return
	}
	m.zx.paused, m.zx.stopping = true, true
	d.stomp = &stompAnim{machine: m, start: time.Now(), target: d.stompTarget(i)}
}

// stopAnimates is whether stopping a machine plays the animation: the Preference, on by default.
func (d *machineDeck) stopAnimates() bool { return d.stopAnimation == nil || d.stopAnimation() }

// stompTarget is where the animation of machine i plays: its tile when the machines are
// tiled, the whole window when it has the window.
func (d *machineDeck) stompTarget(i int) rl.Rectangle {
	if d.tiled() {
		return d.tileRects()[i]
	}
	return rl.Rectangle{Width: float32(rl.GetScreenWidth()), Height: float32(rl.GetScreenHeight())}
}

// tickStomp ends the animation when its time is up, and the machine with it.
func (d *machineDeck) tickStomp(now time.Time) {
	if d.stomp == nil {
		return
	}
	if _, done := stompAt(d.stomp.elapsed(now)); done {
		m := d.stomp.machine
		d.stomp = nil
		d.destroy(m)
	}
}

// drawStomp draws the animation, if there is one, over whatever else the deck has drawn.
func (d *machineDeck) drawStomp() {
	if d.stomp == nil {
		return
	}
	if d.textures == nil {
		d.textures = newStompTextures(embeddedStompImages())
	}
	d.stomp.draw(d.textures, time.Now())
}

// destroy ends machine m: out of the deck when others remain, into the idle state when it was
// the last.
func (d *machineDeck) destroy(m *deckMachine) {
	m.zx.stopping = false
	for i, x := range d.machines {
		if x != m {
			continue
		}
		if len(d.machines) > 1 {
			d.remove(i)
		} else {
			d.makeIdle()
		}
		return
	}
}

// makeIdle ends the only machine there is: its disc is committed, its sound stops, and it stays
// as the window's front machine, stopped.
func (d *machineDeck) makeIdle() {
	m := d.machines[0]
	zx := m.zx
	saveDiskOnClose(zx)
	zx.detachNextBootChain()
	if m.thumbW != 0 {
		rl.UnloadTexture(m.thumb)
	}
	zx.io.ResetKeyboard()
	zx.audio.Close()
	zx.audio.SetDevice(nil)
	zx.paused, zx.stopped, zx.stopping = true, true, false

	d.idle, d.idleModel = zx, m.model
	d.machines, d.front, d.sel = nil, 0, 0
	d.pressOn, d.dragging, d.edit, d.editing, d.flying = -1, false, nil, -1, false
	d.applyLayout()
	sayf("Machines: closed a %s (none running)\n", modelLabel(m.model))
}

// revive starts a machine of model in the place of the stopped one: it is given the window's
// display and the sound device as a machine brought to the front is (flip).
func (d *machineDeck) revive(model string) int {
	old := d.idle
	nz := d.make(model)
	d.machines = []*deckMachine{{zx: nz, model: model, mag: 1}}
	d.front, d.sel, d.idle = 0, 0, nil
	if old != nil {
		old.SetDisplay(nil)
	}
	d.dm.showMachine(nz)
	nz.SetDisplay(d.dm)
	d.dm.SetVideoRenderer(nz.videoRenderer)
	nz.io.ResetKeyboard()
	if nz.audio.IsEnabled() {
		nz.audio.SetDevice(d.dev)
		if err := nz.audio.Initialize(); err != nil {
			sayf("Warning: Audio initialization failed: %v\n", err)
		}
	}
	d.applyLayout()
	sayf("Machines: started a %s (1 running)\n", modelLabel(model))
	return 0
}

// start starts a machine of model when none is running, and returns it (nil when one is): for
// the Machine menu's model rows, which on a running machine switch its model instead.
func (d *machineDeck) start(model string) *ZenZX {
	if !d.Empty() {
		return nil
	}
	d.revive(model)
	d.hide()
	return d.Front()
}
