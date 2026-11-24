// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ha1tch/zenzx/pkg/settingsconfig"
)

// ============================================================================
// Standalone programs: File > Export
//
// A standalone program is ZenZX with a machine inside it. It is one file: the program that
// exported it, followed by the state of the machine at the moment it was exported. Started, it
// powers up that machine, in that state, and nothing else is needed: no snapshot, no ROM files,
// settings of its own to find. (The ROMs are in the program, as they are in ZenZX itself, and so
// are the preferences and the border it was exported with: ProgramView.)
//
// What follows the program is a payload and a trailer:
//
//	program | payload | payload length (8 bytes, little-endian) | trailer magic (16 bytes)
//	payload = "ZXPROGRM" | meta length (4) | meta (JSON) | snapshot length (8) | snapshot | SHA-256 of all before it (32)
//
// The snapshot is a .zxs one (snapshot.go). The operating system loads a program from its own
// headers and ignores what follows the last section, so the program runs as it did, and the
// program reads its own tail when it starts. Exporting from a standalone program takes the
// payload off first, so a program does not grow as it is exported again.
//
// Machines exported are the Spectrums: the 48K, the 128K, the +2, +2A and +3 and their Spanish
// versions. A +3's disc is in its snapshot, so it goes with it.
// ============================================================================

const (
	programMagic = "ZXPROGRM"
	// trailerMagic ends a standalone program. The last four bytes are the format's version.
	trailerMagic = "ZENZXPROGRAM\x00\x00\x00\x01"
	trailerSize  = 8 + len(trailerMagic)

	// embeddedSnapshot is the name a standalone program gives its machine to the snapshot
	// loader: not a file, which is the point.
	embeddedSnapshot = "(this program)"

	// maxProgramPayload is the most a payload may be: a 128K snapshot is 130 KB, and a +3 with
	// a disc a few hundred. This is far above, and keeps a corrupt length from being believed.
	maxProgramPayload = 256 << 20
)

// programModels are the models a program can be exported from.
var programModels = map[string]bool{
	"48k": true, "128k": true, "plus2": true, "plus2a": true, "plus3": true,
	"spanish48k": true, "spanish128k": true, "spanishplus2": true, "spanishplus3": true,
}

// Program is the machine a standalone program carries.
type Program struct {
	Model    string `json:"model"`             // the -model the machine is
	Title    string `json:"title,omitempty"`   // what the person called it
	Version  string `json:"version,omitempty"` // the ZenZX that exported it
	Snapshot []byte `json:"-"`                 // the machine, as a .zxs snapshot
	// View is how the window stood when the program was exported, nil from the headless program
	// (it has no window): a program starts the way it was exported and not as the settings of
	// whoever runs it say.
	View *ProgramView `json:"view,omitempty"`
}

// ProgramView is how the window stood at the export: the menus' own preferences (theme, font,
// zoom, scale, the menu bar fixed or left to hide at the edge, the dock, what a drop does, the
// tape deck) and what the View menu has: the border, and the FPS counter.
type ProgramView struct {
	Settings *settingsconfig.Settings `json:"settings,omitempty"`
	Border   bool                     `json:"border"`
	FPS      bool                     `json:"fps,omitempty"`
}

// checked is the view's settings as the ZenZX that starts the program will have them: validated
// as a settings.json is, and nil when they are not (a program from a ZenZX with a theme or a
// font this one has not), so that the settings of the person who runs it stand instead.
func (v *ProgramView) checked(themes, fonts []string) *settingsconfig.Settings {
	if v == nil || v.Settings == nil {
		return nil
	}
	data, err := json.Marshal(v.Settings)
	if err != nil {
		return nil
	}
	s, err := settingsconfig.Parse(data, themes, fonts)
	if err != nil {
		return nil
	}
	return s
}

// errNoProgram is what readProgram says of a file that carries no machine.
var errNoProgram = errors.New("the file carries no program")

// readProgram reads the machine at the end of the file r of size bytes, and says where the
// program that carries it ends. A file with no trailer gives errNoProgram and its whole size.
func readProgram(r io.ReaderAt, size int64) (p *Program, programLen int64, err error) {
	if size < int64(trailerSize) {
		return nil, size, errNoProgram
	}
	tail := make([]byte, trailerSize)
	if _, err := r.ReadAt(tail, size-int64(trailerSize)); err != nil {
		return nil, size, err
	}
	if string(tail[8:]) != trailerMagic {
		return nil, size, errNoProgram
	}
	n := binary.LittleEndian.Uint64(tail[:8])
	if n < uint64(len(programMagic)+4+8+sha256.Size) || n > maxProgramPayload || int64(n) > size-int64(trailerSize) {
		return nil, size, fmt.Errorf("the program's machine is damaged (its length is %d)", n)
	}
	programLen = size - int64(trailerSize) - int64(n)
	payload := make([]byte, n)
	if _, err := r.ReadAt(payload, programLen); err != nil {
		return nil, size, err
	}
	body, sum := payload[:len(payload)-sha256.Size], payload[len(payload)-sha256.Size:]
	if got := sha256.Sum256(body); !bytes.Equal(got[:], sum) {
		return nil, size, errors.New("the program's machine is damaged (its checksum is wrong)")
	}
	if string(body[:len(programMagic)]) != programMagic {
		return nil, size, errors.New("the program's machine is damaged (it does not begin as a machine does)")
	}
	rest := body[len(programMagic):]
	if len(rest) < 4 {
		return nil, size, errors.New("the program's machine is damaged (it is short)")
	}
	metaLen := binary.LittleEndian.Uint32(rest)
	rest = rest[4:]
	if uint64(metaLen) > uint64(len(rest)) || len(rest)-int(metaLen) < 8 {
		return nil, size, errors.New("the program's machine is damaged (its description is short)")
	}
	p = &Program{}
	if err := json.Unmarshal(rest[:metaLen], p); err != nil {
		return nil, size, fmt.Errorf("the program's machine is damaged (%v)", err)
	}
	rest = rest[metaLen:]
	snapLen := binary.LittleEndian.Uint64(rest)
	rest = rest[8:]
	if snapLen != uint64(len(rest)) {
		return nil, size, errors.New("the program's machine is damaged (its snapshot is the wrong length)")
	}
	p.Snapshot = rest
	if !programModels[p.Model] {
		return nil, size, fmt.Errorf("the program's machine is a %q, which this ZenZX does not know", p.Model)
	}
	return p, programLen, nil
}

// payload is the bytes that follow the program: the machine, then the trailer.
func (p *Program) payload() ([]byte, error) {
	meta, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(programMagic)
	binary.Write(&b, binary.LittleEndian, uint32(len(meta)))
	b.Write(meta)
	binary.Write(&b, binary.LittleEndian, uint64(len(p.Snapshot)))
	b.Write(p.Snapshot)
	sum := sha256.Sum256(b.Bytes())
	b.Write(sum[:])
	n := uint64(b.Len())
	binary.Write(&b, binary.LittleEndian, n)
	b.WriteString(trailerMagic)
	return b.Bytes(), nil
}

// ExportProgram writes the standalone program out: the program in stub, with p after it. stub
// is the program that is running when the person exports, or another system's ZenZX when a
// program is made for that system. A stub that is itself a standalone program is taken
// without its machine.
func ExportProgram(stub, out string, p *Program) error {
	if !programModels[p.Model] {
		return fmt.Errorf("a program cannot be exported from this machine (%s)", p.Model)
	}
	if len(p.Snapshot) == 0 {
		return errors.New("there is no machine to export")
	}
	if sameProgramFile(stub, out) {
		return errors.New("the program cannot be written over the one it is made from")
	}
	prog, err := os.ReadFile(stub)
	if err != nil {
		return err
	}
	if _, programLen, err := readProgram(bytes.NewReader(prog), int64(len(prog))); err == nil {
		prog = prog[:programLen]
	} else if !errors.Is(err, errNoProgram) {
		return err
	}
	tail, err := p.payload()
	if err != nil {
		return err
	}

	// Written beside the target and renamed over it, so that a failure leaves no half a program.
	tmp, err := os.CreateTemp(filepath.Dir(out), ".export-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(prog); err == nil {
		_, err = tmp.Write(tail)
	}
	if err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), out)
}

// sameProgramFile reports whether a and b are the same file on disk.
func sameProgramFile(a, b string) bool {
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(sa, sb)
}

// ProgramFileName is the name a program gets from the one the person typed: the system's own
// extension for a program (".exe" on Windows) is added when it is not there.
func ProgramFileName(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	return name
}

// programState is the machine as a program carries it: the model it is, and a .zxs snapshot.
func (zx *ZenZX) programState(model string) (*Program, error) {
	if !programModels[model] {
		return nil, fmt.Errorf("a program cannot be exported from this machine (%s)", model)
	}
	dir, err := os.MkdirTemp("", "zenzx-export-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "machine.zxs")
	if err := zx.SaveSnapshot(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Program{Model: model, Version: version, Snapshot: data}, nil
}

// ExportProgramFrom exports the machine zx, a machine of model, as a program at out, made from
// the program that is running or, when stub is not empty, from the one at stub.
func (zx *ZenZX) ExportProgramFrom(model, out, stub string) error {
	p, err := zx.programState(model)
	if err != nil {
		return err
	}
	p.Title = strings.TrimSuffix(filepath.Base(out), filepath.Ext(out))
	if stub == "" {
		if stub, err = ownPath(); err != nil {
			return err
		}
	}
	return ExportProgram(stub, out, p)
}

// ownPath is the file this program was started from.
func ownPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if q, err := filepath.EvalSymlinks(p); err == nil {
		p = q
	}
	return p, nil
}

// ownProgram is the machine this program carries, nil when it carries none (ZenZX as it is built).
func ownProgram() (*Program, error) {
	path, err := ownPath()
	if err != nil {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, nil
	}
	p, _, err := readProgram(f, st.Size())
	if errors.Is(err, errNoProgram) {
		return nil, nil
	}
	return p, err
}

// startProgram makes a standalone program start as its machine: the model is the machine's, and
// the snapshot to load is the machine, which LoadSnapshotFormat takes from here and not from a
// file. A program that carries no machine is left as it was.
func startProgram(model, snapshot, format *string) error {
	p, err := ownProgram()
	if err != nil {
		return err
	}
	if p == nil {
		return nil
	}
	startupProgram = p
	*model, *snapshot, *format = p.Model, embeddedSnapshot, "zxs"
	return nil
}

// startupProgram is the machine this program started as, nil if it carries none.
var startupProgram *Program
