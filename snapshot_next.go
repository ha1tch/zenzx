// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ha1tch/zenzx/pkg/machinestate"
)

// Snapshots of a Next. The .zxs of the 48K, the 128K and the +3 holds the state of a Spectrum, and
// none of a Next's SRAM, NextREG file, DMA, copper, sprites, palettes, tilemap, Layer 2, DivMMC or SD
// controller: a Next saved that way came back as a +2A with its game in pieces. A Next's .zxs is
// the machine's whole state instead (nextstate.go): the header says it is of a Next (Model
// snapshotModelNext, FlagNext) and one chunk, ChunkNext, holds the state with the fingerprint of the
// layout it was taken in.
//
// What it does not hold is the card: the state names where the machine was on the card the Next
// has, and goes back over the card the Next has now. A snapshot is for the card it was taken on, and
// for the build of ZenZX that took it (the layout of the state is the program's own structures: a
// build that changed them says so, and refuses the snapshot).

const (
	snapshotModelNext = 5      // SnapshotHeader.Model of a Next
	FlagNext          = 1 << 6 // the snapshot is of a Next: ChunkNext holds the machine
	ChunkNext         = 0x4E58 // "NX" - the whole state of a Next
)

// errNextSnapshotOtherBuild is the refusal of a Next's snapshot that another build of ZenZX made.
var errNextSnapshotOtherBuild = errors.New("the snapshot was made by another build of ZenZX, whose Next was laid out differently")

// saveNextSnapshot writes the snapshot of a Next to filename (which ends in .zxs), with the
// metadata SaveSnapshot has made for it.
func (zx *ZenZX) saveNextSnapshot(filename string, metadata *SnapshotMetadata) error {
	blob, err := zx.NextStateBytes()
	if err != nil {
		return fmt.Errorf("saving the Next: %w", err)
	}

	header := SnapshotHeader{
		Version:   CurrentSnapshotVersion,
		Flags:     FlagNext | FlagHasMetadata,
		Timestamp: uint64(time.Now().Unix()),
		Model:     snapshotModelNext,
	}
	copy(header.Magic[:], SnapshotMagic)

	var image bytes.Buffer
	if err := binary.Write(&image, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	// the cards come first, so that a load that refuses them has not touched the machine's state
	cards, notes := zx.cardChunk()
	if cards != nil {
		if err := binary.Write(&image, binary.LittleEndian, ChunkHeader{ID: ChunkCards, Size: uint32(len(cards))}); err != nil {
			return fmt.Errorf("writing the cards: %w", err)
		}
		image.Write(cards)
	}
	if err := binary.Write(&image, binary.LittleEndian, ChunkHeader{ID: ChunkNext, Size: uint32(len(blob))}); err != nil {
		return fmt.Errorf("writing the Next's chunk: %w", err)
	}
	image.Write(blob)
	if err := binary.Write(&image, binary.LittleEndian, ChunkHeader{ID: ChunkEnd}); err != nil {
		return fmt.Errorf("writing end marker: %w", err)
	}

	metadata.Checksums["next"] = calculateChecksum(blob)
	metadata.ChunkSizes["next"] = len(blob)

	if err := os.WriteFile(filename, image.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing to file: %w", err)
	}
	metaFilename := strings.TrimSuffix(filename, ".zxs") + ".json"
	if metaData, err := json.MarshalIndent(metadata, "", "  "); err == nil {
		if err := os.WriteFile(metaFilename, metaData, 0o644); err != nil {
			sayf("Warning: Could not save metadata file: %v\n", err) // the snapshot is valid without it
		}
	}
	sayf("Snapshot saved: %s (a Next, PC=%04X, %d bytes of state)\n", filename, zx.cpu.PC, len(blob))
	for _, n := range notes {
		if zx.OnNotice != nil {
			zx.OnNotice(n) // the window shows it as a toast
		} else {
			sayf("%s\n", n)
		}
	}
	return nil
}

// loadNextChunk puts the state in a ChunkNext over this machine, which is a Next, and makes what
// was in progress on it before the snapshot stop being: a boot cache that was to be put over it at the
// next frame or made from it, and the wait for NextZXOS to come to rest, would each put the boot's
// state where the snapshot's is.
func (zx *ZenZX) loadNextChunk(data []byte) error {
	if err := zx.RestoreNextState(bytes.NewReader(data)); err != nil {
		if errors.Is(err, machinestate.ErrLayout) {
			return errNextSnapshotOtherBuild
		}
		return err
	}
	if zx.next != nil {
		zx.next.cachePending = false
	}
	zx.onNextHandOver = nil
	zx.nextSettle = nil
	if d, ok := zx.display.(interface{ afterNextRestore() }); ok {
		d.afterNextRestore()
	}
	return nil
}
