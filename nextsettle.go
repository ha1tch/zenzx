// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import "image"

// Knowing that a Next has booted and is at rest (docs/proposals/NEXT_BOOT_CACHE.md, section
// 1). A state cached half-way is worse than none, so three things have to hold together,
// for a run of frames, and the watch gives up after a limit rather than cache a machine
// that never settles:
//
//   - NextZXOS is waiting for a key: the CPU is in the key-wait loop its menu and Browser
//     spin in (nextAtIdlePrompt);
//   - the picture has stopped changing: the display is the same, bar its cursor, a
//     second of frames later;
//   - nothing is in flight: no tape, no DMA, nothing half-way between the machine and a
//     card, no program being typed in or run from the host.
//
// The watch is the host's, not the machine's: it is not part of the saved state.
const (
	// nextSettleIdleFrames is how long the machine must have been idle before the picture
	// is first taken, and nextSettleSteadyFrames how long after that it is compared.
	nextSettleIdleFrames   = 10
	nextSettleSteadyFrames = 40
	// nextSettleSame is the share of the picture that must be the same.
	nextSettleSame = 0.98
	// nextSettleLimit is how many frames after the hand-over the watch waits: 60 s at the
	// real speed, four times what NextZXOS takes to reach its menu.
	nextSettleLimit = 3000
)

type nextSettleWatch struct {
	zx        *ZenZX
	frames    int // since the watch began
	idle      int // consecutive frames at the prompt and quiet
	ref       *image.RGBA
	idleAddr  uint16
	haveAddr  bool
	nextScan  int
	onSettled func(frames int) // called once, when the machine is at rest
	onGaveUp  func()
}

// watchNextSettle starts a watch on a Next that has just been handed over; onSettled is
// called, between two frames, the first time the machine is at rest.
func (zx *ZenZX) watchNextSettle(onSettled func(frames int), onGaveUp func()) {
	zx.nextSettle = &nextSettleWatch{zx: zx, onSettled: onSettled, onGaveUp: onGaveUp}
}

// step is called at the start of every frame, before the frame runs.
func (w *nextSettleWatch) step() {
	zx := w.zx
	w.frames++
	if w.frames > nextSettleLimit {
		zx.nextSettle = nil
		if w.onGaveUp != nil {
			w.onGaveUp()
		}
		return
	}
	if !w.atRest() {
		w.idle, w.ref = 0, nil
		return
	}
	w.idle++
	switch w.idle {
	case nextSettleIdleFrames:
		w.ref = zx.DecodeDisplay()
	case nextSettleIdleFrames + nextSettleSteadyFrames:
		same := w.ref != nil && sameShare(w.ref, zx.DecodeDisplay()) >= nextSettleSame
		w.ref = nil
		if !same {
			w.idle = 0
			return
		}
		zx.nextSettle = nil
		if w.onSettled != nil {
			w.onSettled(zx.frameCount)
		}
	}
}

// atRest is the first and third of the conditions: at the key-wait prompt, and nothing in
// flight. The picture is the second, and is judged over time in step.
func (w *nextSettleWatch) atRest() bool {
	zx := w.zx
	if zx.next == nil || zx.memory.nextCfg.active || zx.memory.bootROMEnabled {
		return false
	}
	// The key-wait loop is found by its signature, which costs a scan of the ROM: it is
	// looked for every 25 frames until it is found, and kept after.
	if !w.haveAddr {
		if w.frames < w.nextScan {
			return false
		}
		w.nextScan = w.frames + 25
		addr, ok := zx.locateBrowserIdleLoop()
		if !ok {
			return false
		}
		w.idleAddr, w.haveAddr = addr, true
	}
	if pc := zx.cpu.PC; pc < w.idleAddr || pc > w.idleAddr+8 {
		return false
	}
	return zx.nextQuiet()
}

// nextQuiet reports that nothing is in flight in the machine or the host's side of it.
func (zx *ZenZX) nextQuiet() bool {
	if zx.turboActive || zx.auto != nil || zx.nexRun != nil || zx.io.nextResetRequest != 0 {
		return false
	}
	if zx.tape != nil && zx.tape.st != nil && zx.tape.st.Playing {
		return false
	}
	d := zx.io.dma
	if d.enabled || d.busy || d.stolen != 0 {
		return false
	}
	if dm := zx.io.divMMC; dm != nil {
		if dm.pending != 0 {
			return false
		}
		for _, c := range []*SDCard{dm.sd, dm.sd2} {
			if c != nil && !c.idle() {
				return false
			}
		}
	}
	return true
}

// idle reports that no command, response, block or write is half-way on this card.
func (c *SDCard) idle() bool {
	return len(c.cmdBuf) == 0 && len(c.pending) == 0 && !c.multiBlock &&
		c.wr.state == 0 && len(c.wr.buf) == 0 && !c.wr.multi
}

// sameShare is the share of pixels two pictures of the same size agree on.
func sameShare(a, b *image.RGBA) float64 {
	if a.Bounds() != b.Bounds() {
		return 0
	}
	n, same := 0, 0
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			n++
			if a.RGBAAt(x, y) == b.RGBAAt(x, y) {
				same++
			}
		}
	}
	if n == 0 {
		return 0
	}
	return float64(same) / float64(n)
}
