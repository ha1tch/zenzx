// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ha1tch/zenzx/pkg/machinestate"
)

// The state of a Next, saved and put back whole (docs/proposals/NEXT_BOOT_CACHE.md).
//
// The snapshots (.zxs, .sna, .z80) hold the 48K and 128K machines and none of the Next's
// SRAM, NextREG file, DMA, copper, sprites, palettes, DivMMC or SD controller. This is a
// state of its own, taken by walking the machine's structs (pkg/machinestate) so that a
// field is kept because it exists, and not because someone remembered to list it. What is
// left out is said here, in one place, and a test (nextstate_test.go) pins the list: a new
// callback, file or device added to the machine shows there and has to be decided.
//
// A state is put back into a machine that was made the way a cold boot makes it (the
// model's ROMs, the boot ROM, the DivMMC and the card attached), so that what was wired at
// construction stays wired.

// nextStatePolicy says what is not the machine's state.
func nextStatePolicy() *machinestate.Policy {
	return &machinestate.Policy{
		SkipFields: map[string]bool{
			// The host's side of the machine: the window, the sound device, the tape
			// deck, a program being typed in, a file being run, and the boot chain's files.
			"zenzx.ZenZX.audio":            true,
			"zenzx.ZenZX.tape":             true,
			"zenzx.ZenZX.auto":             true,
			"zenzx.ZenZX.nexRun":           true,
			"zenzx.ZenZX.nexScratch":       true,
			"zenzx.ZenZX.next":             true,
			"zenzx.ZenZX.nextSettle":       true,
			"zenzx.ZenZX.romPatches":       true,
			"zenzx.ZenZX.card2":            true,
			"zenzx.ZenZX.cardsApproved":    true,
			"zenzx.ZenZX.running":          true,
			"zenzx.ZenZX.paused":           true,
			"zenzx.ZenZX.escape":           true,
			"zenzx.ZenZX.uiOwnsEscape":     true,
			"zenzx.ZenZX.scriptKeys":       true,
			"zenzx.ZenZX.snapshotFormat":   true,
			"zenzx.ZenZX.nonStandard":      true,
			"zenzx.ZenZX.videoRenderer":    true,
			"zenzx.ZenZX.rendererSelected": true,
			"zenzx.ZenZX.firstProgram":     true,
			// The tape loader's fast path: not in use on a Next at rest.
			"zenzx.ZenZX.turboActive":       true,
			"zenzx.ZenZX.turboNextBoundary": true,
			"zenzx.ZenZX.turboPageFlags":    true,
			"zenzx.ZenZX.turboPages":        true,
			"zenzx.ZenZX.turboSlotPage":     true,
			"zenzx.ZenZX.turboPort7FFD":     true,
			// Pointers that are wired to storage at construction: the DivMMC's ROM and RAM,
			// which are kept through the DivMMC itself.
			"zenzx.nextConfigState.divROM": true,
			"zenzx.nextConfigState.divRAM": true,
			// The wall clock and the host's debug settings are not the machine's.
			"zenzx.SpectrumScreen.lastFlashTime": true,
			"zenzx.SpectrumIO.nextRegLog":        true,
			"zenzx.DivMMC.Debug":                 true,
			"zenzx.SDCard.Debug":                 true,
			"zenzx.SDCard.wr":                    true, // the card's write protection is the chain's, not the state's
		},
		SkipTypes: []reflect.Type{
			reflect.TypeOf(AudioWrapper{}),
			reflect.TypeOf(Link{}),
			reflect.TypeOf(FDC765{}),
			reflect.TypeOf(image.RGBA{}),
			reflect.TypeOf(time.Time{}),
			reflect.TypeOf(sync.Mutex{}),
			reflect.TypeOf(sync.RWMutex{}),
			reflect.TypeOf(atomic.Int32{}),
		},
	}
}

// nextStateRoots are what is saved: the machine, whole. The CPU, memory, I/O and screen are
// reached through it.
func (zx *ZenZX) nextStateRoots() []any { return []any{zx} }

// SaveNextState writes the machine's state to w. A machine that is playing a tape or runs
// the tape loader's fast path is not at rest and is refused.
func (zx *ZenZX) SaveNextState(w io.Writer) error {
	if !zx.memory.isNext {
		return fmt.Errorf("the state is the Next's; this machine is not one")
	}
	if zx.turboActive || (zx.tape != nil && zx.tape.st != nil && zx.tape.st.Playing) {
		return fmt.Errorf("the machine is not at rest (a tape is loading)")
	}
	return nextStatePolicy().Encode(w, zx.nextStateRoots()...)
}

// RestoreNextState puts a saved state over this machine, which must be a Next made as a
// cold boot makes it. What belongs to the boot chain rather than to the state, the card's
// write protection, is kept; the sound chip is given its registers again.
func (zx *ZenZX) RestoreNextState(r io.Reader) error {
	if !zx.memory.isNext {
		return fmt.Errorf("the state is the Next's; this machine is not one")
	}
	if err := nextStatePolicy().Decode(r, zx.nextStateRoots()...); err != nil {
		return err
	}
	zx.io.SetAYRegisters(zx.io.ayRegisters)
	return nil
}

// NextStateBytes is SaveNextState into memory.
func (zx *ZenZX) NextStateBytes() ([]byte, error) {
	var b bytes.Buffer
	if err := zx.SaveNextState(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
