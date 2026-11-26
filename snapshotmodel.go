// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ha1tch/zentools/pkg/snapshot"
	"github.com/ha1tch/zentools/pkg/szx"
)

// What machine a snapshot is of. A snapshot holds the state of a 48K, a 128K or a +3, and loaded into
// a machine of another kind it is a picture of one machine in the memory of another: a 128K's pages
// where a 48K has none, a +3's disc controller where a 128K has no port for it. So a snapshot is
// loaded into a machine of its own kind, and the machine is made when there is not one (the deck,
// snapshotopen_gui.go): the kinds below are how that is told.

// machineClass is the kind of machine a snapshot can be of, or a running machine is.
type machineClass int

const (
	// classOther is a machine that no snapshot is of: the TS2068. A snapshot loaded into one is not
	// refused, as it never was; the deck does not load into one, it makes the machine the snapshot
	// is of.
	classOther machineClass = iota
	class48K
	class128K
	classPlus3
	// classNext is a Next, whose snapshot is its whole state (snapshot_next.go).
	classNext
)

func (c machineClass) String() string {
	switch c {
	case class48K:
		return "48K"
	case class128K:
		return "128K"
	case classPlus3:
		return "+3"
	case classNext:
		return "Next"
	}
	return "machine"
}

// classOfModel is the kind of a model, as -model names it. The +2 is a 128K, and the +2A a +3 with
// no disc drive.
func classOfModel(model string) machineClass {
	switch model {
	case "48k", "spanish48k":
		return class48K
	case "128k", "plus2", "spanish128k", "spanishplus2":
		return class128K
	case "plus2a", "plus3", "spanishplus3":
		return classPlus3
	case "next":
		return classNext
	}
	return classOther
}

// machineClass is the kind of machine this one is, from what it is and not from what it was
// called: the model of a machine that was switched is the one it was switched to, and that is
// what this tells.
func (zx *ZenZX) machineClass() machineClass {
	switch {
	case zx.memory.isNext:
		return classNext
	case zx.memory.isTS2068:
		return classOther
	case zx.isPlus3:
		return classPlus3
	case zx.is128K:
		return class128K
	}
	return class48K
}

// snapshotMachine is the machine a snapshot file is of: its kind, and the model that makes one
// (a snapshot does not say which ROM its machine had, so it is the Sinclair one).
type snapshotMachine struct {
	Class machineClass
	Model string
}

// snapshotMachineOfHeader is the machine a .zxs is of, from its header: a Next by its flag, a +3 with
// its disc controller or the +2A that is a +3 in all but that has none, a 128K, or a 48K.
func snapshotMachineOfHeader(h SnapshotHeader) snapshotMachine {
	switch {
	case h.Flags&FlagNext != 0 || h.Model == snapshotModelNext:
		return snapshotMachine{classNext, "next"}
	case h.Flags&FlagPlus3 != 0 || h.Model >= 3:
		if h.Flags&FlagFDC != 0 {
			return snapshotMachine{classPlus3, "plus3"}
		}
		return snapshotMachine{classPlus3, "plus2a"}
	case h.Flags&Flag128K != 0 || h.Model == 1 || h.Model == 2:
		return snapshotMachine{class128K, "128k"}
	}
	return snapshotMachine{class48K, "48k"}
}

// snapshotModelOf is the machine a model of a decoded snapshot (.z80, .szx) is of.
func snapshotModelOf(m snapshot.Model) snapshotMachine {
	switch m {
	case snapshot.Model128K:
		return snapshotMachine{class128K, "128k"}
	case snapshot.ModelPlus2:
		return snapshotMachine{class128K, "plus2"}
	case snapshot.ModelPlus2A:
		return snapshotMachine{classPlus3, "plus2a"}
	case snapshot.ModelPlus3:
		return snapshotMachine{classPlus3, "plus3"}
	}
	return snapshotMachine{class48K, "48k"}
}

// readSnapshotMachine tells which machine the snapshot at path is of, from its header and no more
// than it takes to find it. format is "" to find it out from the file, or what DetectSnapshotFormat
// says.
func readSnapshotMachine(path, format string) (snapshotMachine, error) {
	if format == "" {
		format = DetectSnapshotFormat(path)
	}
	switch format {
	case "zxs":
		f, err := os.Open(path)
		if err != nil {
			return snapshotMachine{}, err
		}
		defer f.Close()
		var h SnapshotHeader
		if err := binary.Read(f, binary.LittleEndian, &h); err != nil {
			return snapshotMachine{}, fmt.Errorf("reading header: %w", err)
		}
		if string(h.Magic[:]) != SnapshotMagic {
			return snapshotMachine{}, fmt.Errorf("invalid snapshot magic: %s", h.Magic)
		}
		return snapshotMachineOfHeader(h), nil

	case "sna":
		st, err := os.Stat(path)
		if err != nil {
			return snapshotMachine{}, err
		}
		if st.Size() > 49179 { // as LoadSNA picks the decoder
			return snapshotMachine{class128K, "128k"}, nil
		}
		return snapshotMachine{class48K, "48k"}, nil

	case "z80":
		image, err := os.ReadFile(path)
		if err != nil {
			return snapshotMachine{}, err
		}
		s, err := snapshot.DecodeZ80(image)
		if err != nil {
			return snapshotMachine{}, fmt.Errorf("decode .z80: %w", err)
		}
		return snapshotModelOf(s.Model), nil

	case "szx":
		image, err := os.ReadFile(path)
		if err != nil {
			return snapshotMachine{}, err
		}
		s, err := szx.Decode(image)
		if err != nil {
			return snapshotMachine{}, fmt.Errorf("decode .szx: %w", err)
		}
		return snapshotModelOf(s.Model), nil
	}
	return snapshotMachine{}, fmt.Errorf("%s is not a snapshot", path)
}

// snapshotMismatch is the refusal to load a snapshot into a machine that is not of its kind.
type snapshotMismatch struct {
	Snapshot, Machine machineClass
}

func (e *snapshotMismatch) Error() string {
	return fmt.Sprintf("the snapshot is of a %s and this machine is a %s", e.Snapshot, e.Machine)
}

// checkSnapshotFits is nil when the snapshot at path can be loaded into zx: it is of the kind zx is,
// or zx is a machine that has no kind (classOther).
func (zx *ZenZX) checkSnapshotFits(path, format string) error {
	have := zx.machineClass()
	if have == classOther {
		return nil
	}
	want, err := readSnapshotMachine(path, format)
	if err != nil {
		return nil // the load says what is wrong with the file, in its own words
	}
	if want.Class != have {
		return &snapshotMismatch{Snapshot: want.Class, Machine: have}
	}
	return nil
}

// snapshotMismatchOf is the refusal in err, if err is one.
func snapshotMismatchOf(err error) (*snapshotMismatch, bool) {
	var m *snapshotMismatch
	if errors.As(err, &m) {
		return m, true
	}
	return nil, false
}

// snapshotRefusal is what is said of a snapshot that was not loaded: that it is of another kind of
// machine, when that is why.
func snapshotRefusal(base string, err error) string {
	if m, ok := snapshotMismatchOf(err); ok {
		return fmt.Sprintf("%s is a %s snapshot and this machine is a %s", base, m.Snapshot, m.Machine)
	}
	if errors.Is(err, errNextSnapshotOtherBuild) {
		return base + " was saved by another build of ZenZX and cannot be loaded"
	}
	var mm *cardMismatch
	if errors.As(err, &mm) {
		return base + " was made with cards that this Next does not have: " + mm.Error()
	}
	var nf *cardsNotFound
	if errors.As(err, &nf) {
		return base + " cannot be loaded: " + nf.Error()
	}
	var cu *cardsUnusable
	if errors.As(err, &cu) {
		return base + " cannot be loaded: " + cu.Error()
	}
	return base + " could not be loaded as a snapshot"
}

// isSnapshotExt says the file is named as a snapshot is: the extensions that load a machine's
// state, and not .rzx, which plays a recording into the machine that is.
func isSnapshotExt(file string) bool {
	if len(file) <= 4 {
		return false
	}
	switch strings.ToLower(file[len(file)-4:]) {
	case ".zxs", ".sna", ".z80", ".szx":
		return true
	}
	return false
}

// startModelForSnapshot is the model a run that opens a snapshot starts as, when the person did not
// choose one: the snapshot's own, so that it is loaded into the machine it is of. It returns model
// as it was when there is no snapshot, when it is not one that is read, or when the file is not
// readable (the load says why).
func startModelForSnapshot(model, path, format string) string {
	if path == "" || path == embeddedSnapshot {
		return model
	}
	if format == "auto" {
		format = ""
	}
	if format == "rzx" {
		return model
	}
	sm, err := readSnapshotMachine(path, format)
	if err != nil {
		return model
	}
	if classOfModel(model) == sm.Class {
		return model // a Spanish 128K stays one for a 128K snapshot
	}
	return sm.Model
}

// startModelFromSnapshot makes -model the machine of the snapshot that -snapshot opens, when -model
// was not given (and -multi, which has its own list, was not either): a 128K snapshot opened with
// no more said starts a 128K, as the window makes one for a snapshot dropped on it.
func startModelFromSnapshot(fs *flag.FlagSet, model *string, multi, snapshot, format string) {
	given := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "model" {
			given = true
		}
	})
	if given || multi != "" {
		return
	}
	*model = startModelForSnapshot(*model, snapshot, format)
}
