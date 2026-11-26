//go:build !headless

// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import rl "github.com/gen2brain/raylib-go/raylib"

// afterNextRestore is the window's part of a Next's state being put back (snapshot_next.go): what it
// drew from the machine before is of another state. The pictures made from the palettes and the
// sprites are made again from the ones that are there now.
func (dm *DisplayManager) afterNextRestore() {
	if dm.spriteGPUCache != nil {
		dm.spriteGPUCache.clear()
	}
	if dm.layer2ColourGPUReady {
		for i := range dm.layer2ColourGPU {
			rl.UnloadTexture(dm.layer2ColourGPU[i])
		}
		dm.layer2ColourGPUReady = false
	}
}
